#!/usr/bin/env python3
"""Definition of done for one block (lot, protocol step, tooling batch), in the order that makes sense.

Usage: mise run done -- <block> [--base <ref>] [--skip-gate]
Order (each step runs on the code the previous steps produced; the slowest comes last):
  1. the frozen gate gates/<block>.toml exists and was committed before the block's code;
  2. the /simplify report gates/<block>/SIMPLIFY.md exists and is newer than every code change;
  3. fast mechanical checks: lint, frontend check, lint:env, lint:claude-md;
  4. formal models: TLA+ and Alloy, mutants included (mise run formal);
  5. tests: Go, frontend, Python;
  6. mutation testing on the touched protocol/* packages (efficacy >= 95% or survivors justified
     in SIMPLIFY.md);
  7. the full gate, regressions and native measurement included;
  8. documentation mentions the block (docs/objectif-en-cours.md).
Only the Claude session commits afterwards (CLAUDE.md). Stops at the first failure.
"""
import argparse
import os
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
MIN_EFFICACY = 95.0


def sh(cmd, **kw):
    print(f"$ {cmd}", flush=True)
    return subprocess.run(cmd, shell=True, cwd=kw.pop("cwd", ROOT), **kw)


def stream(cmd, cwd, env):
    """Runs cmd, echoing every output line as it comes (a long step stays observable: rule
    « Tâches longues ») and returning (exit code, full output)."""
    print(f"$ {cmd}", flush=True)
    lines = []
    with subprocess.Popen(cmd, shell=True, cwd=cwd, env=env, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as proc:
        for line in proc.stdout:
            print(line, end="", flush=True)
            lines.append(line)
    return proc.returncode, "".join(lines)


def must(cmd, step, **kw):
    if sh(cmd, **kw).returncode != 0:
        sys.exit(f"done: step {step} failed: {cmd}")


def git(*args):
    return subprocess.run(["git", *args], cwd=ROOT, capture_output=True, text=True).stdout.strip()


def changed_files(base):
    committed = git("diff", "--name-only", f"{base}...HEAD").splitlines()
    worktree = git("diff", "--name-only", "HEAD").splitlines()
    untracked = git("ls-files", "--others", "--exclude-standard").splitlines()
    return sorted({f for f in committed + worktree + untracked if f})


def code(files):
    return [f for f in files if not f.startswith(("docs/", "gates/")) and not f.endswith(".md")]


def step1_gate_first(block, base):
    gate = ROOT / f"gates/{block}.toml"
    if not gate.exists():
        sys.exit(f"done: step 1: gates/{block}.toml is missing (write the gate before the code)")
    gate_time = git("log", "--diff-filter=A", "--format=%ct", "-1", "--", f"gates/{block}.toml")
    if not gate_time:
        sys.exit(f"done: step 1: gates/{block}.toml is not committed (commit the gate first)")
    later = [f for f in code(git("diff", "--name-only", f"{base}...HEAD").splitlines())
             if (t := git("log", "--format=%ct", "-1", f"{base}..HEAD", "--", f))
             and int(t) < int(gate_time)]
    if later:
        sys.exit(f"done: step 1: code committed before the gate: {', '.join(later[:5])}")
    print("done: 1 gate written before the code: ok")


def step2_simplify(block, files):
    report = ROOT / f"gates/{block}/SIMPLIFY.md"
    if not report.exists() or report.stat().st_size == 0:
        sys.exit(f"done: step 2: gates/{block}/SIMPLIFY.md is missing (run /simplify on the block's diff)")
    newest = max((ROOT / f).stat().st_mtime for f in code(files) if (ROOT / f).exists()) if code(files) else 0
    if report.stat().st_mtime < newest:
        sys.exit("done: step 2: code changed after the /simplify report; rerun /simplify and update it")
    print("done: 2 /simplify report present and current: ok")


# Same environment as `mise run mutate:go`: -count=1 (the timeout is a multiple of a real coverage
# run, not a cached one) and a per-test-binary cap (4 workers x 1.8 GB < the 8 GB of heavy.sh), so a
# mutant that allocates without bound fails alone (2026-11-08: fold.go bitset mutants, run OOM-killed).
MUTATION_ENV = {**os.environ,
                "GOFLAGS": f"{os.environ.get('GOFLAGS', '-mod=readonly')} -count=1 -exec={ROOT}/scripts/memcap.sh",
                "INKWELL_AS_LIMIT": os.environ.get("INKWELL_AS_LIMIT", "1800000000")}
GREMLINS = "github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0"


def gremlins():
    """Installs the pinned gremlins once and returns its path. Not `go run`: GOFLAGS' -exec would
    then cap gremlins itself (pthread_create fails), while the cap is meant for the test binaries."""
    gobin = pathlib.Path(os.environ.get("XDG_CACHE_HOME", pathlib.Path.home() / ".cache")) / "inkwell/gremlins-v0.6.0"
    if not (gobin / "gremlins").exists():
        must(f"GOBIN={gobin} GOFLAGS=-mod=mod go install {GREMLINS}", 6)
    return gobin / "gremlins"


MAX_TIMED_OUT = 0.10  # share of mutants that may time out before a measurement is void


def evaluated(output):
    """Mutants gremlins actually judged (killed + lived)."""
    k = re.search(r"Killed: (\d+), Lived: (\d+)", output)
    return int(k.group(1)) + int(k.group(2)) if k else 0


def void_measurement(output):
    """Why a gremlins run proves nothing, or "" if it is valid. Timed-out mutants are left out of
    the efficacy, so many of them can hide survivors: no evaluated mutant, or more than
    MAX_TIMED_OUT of them timed out (2026-11-08: 278 of 278), is an error whatever SIMPLIFY.md
    says, never a low score that a justification could excuse."""
    judged = evaluated(output)
    t = re.search(r"Timed out: (\d+)", output)
    timed_out = int(t.group(1)) if t else 0
    if judged == 0:
        return "no mutant was evaluated"
    if timed_out > MAX_TIMED_OUT * (judged + timed_out):
        return f"{timed_out} of {judged + timed_out} mutants timed out"
    return ""


def parse_efficacy(output):
    """Gremlins' "Test efficacy: N%"; a missing line counts as 0 (a run that reports nothing fails)."""
    m = re.search(r"Test efficacy: ([0-9.]+)%", output)
    return float(m.group(1)) if m else 0.0


def step6_mutation(block, files):
    packages = sorted({str(pathlib.Path(f).parent) for f in code(files)
                       if f.startswith("protocol/") and f.endswith(".go") and not f.endswith("_test.go")})
    report = (ROOT / f"gates/{block}/SIMPLIFY.md").read_text()
    for pkg in packages:
        rel = os.path.relpath(pkg, "protocol")
        rc, out = stream(f"../scripts/heavy.sh {gremlins()} unleash --workers 4 --timeout-coefficient 10 ./{rel}",
                         ROOT / "protocol", MUTATION_ENV)
        if rc != 0:
            hint = " (137: killed by the heavy.sh memory cap)" if rc == 137 else ""
            sys.exit(f"done: step 6: {pkg}: gremlins exited {rc}{hint}; the measurement is void")
        if why := void_measurement(out):
            sys.exit(f"done: step 6: {pkg}: {why}; the measurement is void — rerun on an idle machine")
        efficacy = parse_efficacy(out)
        justified = f"mutation {pkg}" in report
        print(f"done: 6 mutation {pkg}: {efficacy:.1f}%{' (survivors justified)' if justified else ''}")
        if efficacy < MIN_EFFICACY and not justified:
            sys.exit(f"done: step 6: {pkg} efficacy {efficacy:.1f}% < {MIN_EFFICACY}% and no "
                     f"'mutation {pkg}' justification in gates/{block}/SIMPLIFY.md")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("block")
    ap.add_argument("--base", default="origin/main")
    ap.add_argument("--skip-gate", action="store_true", help="steps 1-6 and 8 only (fast loop)")
    a = ap.parse_args()
    files = changed_files(a.base)
    step1_gate_first(a.block, a.base)
    step2_simplify(a.block, files)
    must("mise run lint && mise run fe:check && mise run lint:env && mise run lint:claude-md", 3)
    must("mise run formal", 4)
    must("mise run test && mise run test:py", 5)
    step6_mutation(a.block, files)
    if not a.skip_gate:
        must(f"INKWELL_NATIVE_PERF_RUN=1 mise run gate -- {a.block}", 7)
    if a.block not in (ROOT / "docs/objectif-en-cours.md").read_text():
        sys.exit(f"done: step 8: docs/objectif-en-cours.md does not mention {a.block}")
    print(f"done: block {a.block} is done; commit it (Claude only), then push (CI reruns)")


if __name__ == "__main__":
    main()
