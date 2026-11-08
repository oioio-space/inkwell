#!/usr/bin/env python3
"""Mutation check of the impaired-network tests. Development tool, never shipped.

Usage: mutants.py [--check] BLOCK [MUTANT_ID...]   (mise run netsim:mutants -- NI0)
--check only verifies that every anchor is found exactly once and no edit is a no-op.

Modelled on scripts/alloy_check.py: each mutant of scripts/netsim/mutants.toml is a text
substitution applied to a copy of the tree (never to the working tree); its scenario must turn
the named clause red. The anchor must be found exactly once. Mutants run one after the other
(each e2e run is a heavy job); the copy is removed at the end, whatever happens.
"""
import json
import os
import pathlib
import shutil
import signal
import subprocess
import sys
import tempfile
import tomllib

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
SEED = os.environ.get("NETSIM_MUTANT_SEED", "1")


def copy_tree(dest: pathlib.Path):
    """A copy of the working tree (uncommitted changes and generated files included)."""
    subprocess.run(
        ["rsync", "-a", "--exclude=/.git", "--exclude=node_modules", "--exclude=/.claude",
         "--exclude=/.task", "--exclude=/bin", "--exclude=/frontend/test-results",
         f"{ROOT}/", f"{dest}/"], check=True)
    (dest / "frontend/node_modules").symlink_to(ROOT / "frontend/node_modules")
    for pkg in (ROOT / "frontend/packages").glob("*/node_modules"):
        link = dest / pkg.relative_to(ROOT)
        if not link.exists():
            link.symlink_to(pkg)


def apply(edits, tree: pathlib.Path):
    """Apply the edits; return an error message if an anchor is not found exactly once, or an edit changes nothing."""
    for e in edits:
        path = tree / e["file"]
        if e.get("create"):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(e["after"])
            continue
        if e["before"] == e["after"]:
            return f"no-op edit in {e['file']} (before == after): {e['before'][:60]!r}"
        text = path.read_text()
        n = text.count(e["before"])
        if n != 1:
            return f"anchor in {e['file']} found {n} times (need exactly 1): {e['before'][:60]!r}"
        path.write_text(text.replace(e["before"], e["after"], 1))
    return None


def sh(cmd, cwd, env=None, log=None, timeout=1800):
    """Run cmd (own process group, killed at the timeout); return (exit code, output). 124 = timeout."""
    full = {**os.environ, **(env or {})}
    proc = subprocess.Popen(cmd, cwd=cwd, env=full, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                            text=True, start_new_session=True)
    try:
        out, _ = proc.communicate(timeout=timeout)
        rc = proc.returncode
    except subprocess.TimeoutExpired:
        os.killpg(proc.pid, signal.SIGKILL)
        out, _ = proc.communicate()
        rc = 124
    if log is not None:
        log.append(out[-3000:])
    return rc, out


def run_go(m, tree):
    log = []
    rc, out = sh(["./scripts/heavy.sh", "timeout", "900", "go", "test", m["pkg"], "-run", m["test"], "-count=1"],
                 tree, log=log, timeout=1000)
    # Caught only by a real test failure: not by a build error, a timeout or a kill.
    caught = rc not in (0, 124, 137) and "--- FAIL" in out and "[build failed]" not in out
    return caught, f"go test exit {rc}" + ("" if caught or rc == 0 else " without a failing test (build error or timeout)"), log


def run_e2e(m, tree):
    log = []
    if any(e["file"].startswith("frontend/") for e in m["edit"]):
        rc, _ = sh(["pnpm", "--config.verifyDepsBeforeRun=false", "run", "build"], tree / "frontend",
                   {"VITE_E2E_OBSERVE": "1"}, log, timeout=900)
        if rc:
            return False, f"frontend build failed (exit {rc})", log
    bins = tree / "bin"
    bins.mkdir(exist_ok=True)
    for out_name, target, env in (("inkwell-server", "./cmd/inkwell", {}), ("inkwell-relay", "./cmd/inkwell-relay", {"CGO_ENABLED": "0"})):
        tags = ["-tags", "server"] if out_name == "inkwell-server" else []
        rc, _ = sh(["./scripts/heavy.sh", "go", "build", *tags, "-o", str(bins / out_name), target], tree, env, log, timeout=900)
        if rc:
            return False, f"go build of {out_name} failed (the mutant must compile)", log
    n = str(m["scenario"])
    variant = m.get("variant", "")
    env = {"NETSIM_BIN_DIR": str(bins), "NETSIM_SCENARIOS": n, "NETSIM_SEEDS": SEED,
           "NETSIM_VARIANTS": variant, "INKWELL_E2E_BROWSER": "chromium", "NETSIM_EXPECT": "characterize"}
    run = n + (f"-{variant}" if variant else "") + f"-s{SEED}"
    result = ROOT / ".task/netsim/results" / f"{run}.json"
    result.unlink(missing_ok=True)  # a stale file from an earlier run must never be read as this one's
    # run.sh of the real tree drives the real frontend/e2e specs; only the binaries are mutated.
    rc, _ = sh(["./scripts/heavy.sh", "./scripts/netsim/run.sh"], ROOT, env, log, timeout=1500)
    if rc == 124:
        return False, "timeout (not a clause failure)", log
    if not result.exists():
        return False, f"no results file {result.name} (skipped or crashed, exit {rc})", log
    clauses = {c["id"]: c for c in json.loads(result.read_text())["clauses"]}
    c = clauses.get(m["clause"])
    if c is None:
        return False, f"clause {m['clause']} not recorded", log
    # "fail" with an error is a harness exception, not a measured miss.
    caught = c["status"] == "fail" and not c["measured"].startswith("error:")
    return caught, f"{m['clause']} is {c['status']}: {c['measured'][:160]}", log


def validate(mutants):
    """Static checks of the mutant list; return the problems found."""
    bad = []
    for m in mutants:
        if m["kind"] == "e2e" and m["scenario"] == 8 and not m.get("variant"):
            bad.append(f"mutant {m['id']}: scenario 8 needs a variant (editor or reader)")
    return bad


def main():
    args = sys.argv[1:]
    check_only = "--check" in args
    args = [a for a in args if a != "--check"]
    if not args:
        sys.exit(__doc__)
    block, only = args[0], set(args[1:])
    mutants = [m for m in tomllib.loads((ROOT / "scripts/netsim/mutants.toml").read_text())["mutant"]
               if m["block"] == block and (not only or m["id"] in only)]
    if not mutants:
        sys.exit(f"netsim mutants: no mutant for block {block}")
    problems = validate(mutants)
    if problems:
        sys.exit("netsim mutants: " + "; ".join(problems))
    if check_only:  # anchors only, on a throwaway copy of each file: nothing is run
        for m in mutants:
            with tempfile.TemporaryDirectory(prefix="netsim-check-") as d:
                tree = pathlib.Path(d)
                for e in m["edit"]:
                    if not e.get("create"):
                        dest = tree / e["file"]
                        dest.parent.mkdir(parents=True, exist_ok=True)
                        shutil.copy2(ROOT / e["file"], dest)
                err = apply(m["edit"], tree)
                print(f"mutant {m['id']}: {err or 'edits apply'}")
                if err:
                    problems.append(m["id"])
        sys.exit(1 if problems else 0)
    if any(m["kind"] == "e2e" for m in mutants):
        # Only the e2e mutants need the frontend bundle and the browser (go-only ones do not).
        rc, out = sh(["mise", "run", "e2e:build"], ROOT, timeout=1800)
        if rc:
            sys.exit(f"netsim mutants: e2e:build failed (exit {rc})\n{out[-1500:]}")
    failures = []
    for m in mutants:
        with tempfile.TemporaryDirectory(prefix=f"netsim-{m['id']}-") as d:
            tree = pathlib.Path(d) / "tree"
            copy_tree(tree)
            err = apply(m["edit"], tree)
            if err:
                print(f"mutant {m['id']}: {err}")
                failures.append(m["id"])
                continue
            caught, detail, log = (run_go if m["kind"] == "go" else run_e2e)(m, tree)
            print(f"mutant {m['id']} -> {m['clause']}: {'caught' if caught else 'NOT CAUGHT'} ({detail})", flush=True)
            if not caught:
                failures.append(m["id"])
                print("\n".join(log)[-1500:])
    if failures:
        sys.exit("netsim mutants: not caught: " + ", ".join(failures))
    print(f"netsim mutants: all {len(mutants)} caught")


if __name__ == "__main__":
    main()
