#!/usr/bin/env python3
"""Mutation check of the impaired-network tests. Development tool, never shipped.

Usage: mutants.py [--check] BLOCK [MUTANT_ID...]   (mise run netsim:mutants -- NI0)
       mutants.py --check                          (the whole catalogue)
--check only verifies that every anchor is found exactly once and no edit is a no-op. Without a block it checks the
whole catalogue: no id twice in one block, every anchor found unless the entry is retired, and every entry a gate
names (gates/*.toml) present in its block.

Ids are scoped by their block: the runner and the gates select `block == B` first, then ids within B, so the same id
in two blocks is harmless; the same id twice in one block is refused. An entry whose code is gone is kept with
`retired = "reason"`: it is never run nor anchored, and a gate that names it is reported as not replayable.

A `vitest` entry may name the exact title of the test it targets (`name`, block EX0): it is then caught only when
that test fails, never by another test of the same file. Entries without `name` keep the file-wide rule (SI1).

A `playwright` entry (block EX0c2) mutates the frontend and proves an end-to-end test sees it: `spec` (a file of
frontend/e2e/), `grep` (the start of the test's title), `project` (the browser, chromium or webkit) and optionally
`error` (a text each failure must say). The copy's frontend and server are built, the spec runs there alone, and it
is caught only when every run of the tests whose title starts with `grep` failed (with `error`, when given): a build
failure, a timeout, no such test or a passing one is not a catch.

Modelled on scripts/alloy_check.py: each mutant of scripts/netsim/mutants.toml is a text
substitution applied to a copy of the tree (never to the working tree); its scenario must turn
the named clause red. The anchor must be found exactly once. Mutants run one after the other
(each e2e run is a heavy job); the copy is removed at the end, whatever happens.
"""
import json
import os
import pathlib
import re
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


# The process groups started by `sh` and not yet finished: a signal kills them (`on_signal`).
CHILDREN = set()


def on_signal(signum, _frame):
    """SIGTERM or SIGINT (the gate's timeout, Ctrl+C): kill every running child's process group, then unwind, so each
    `with tempfile.TemporaryDirectory()` removes its copy of the tree."""
    for pid in list(CHILDREN):
        try:
            os.killpg(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        CHILDREN.discard(pid)
    raise SystemExit(128 + signum)


def install_signals():
    for sig in (signal.SIGTERM, signal.SIGINT):
        signal.signal(sig, on_signal)


def sh(cmd, cwd, env=None, log=None, timeout=1800):
    """Run cmd (own process group, killed at the timeout); return (exit code, output). 124 = timeout."""
    full = {**os.environ, **(env or {})}
    proc = subprocess.Popen(cmd, cwd=cwd, env=full, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                            text=True, start_new_session=True)
    CHILDREN.add(proc.pid)
    try:
        out, _ = proc.communicate(timeout=timeout)
        rc = proc.returncode
    except subprocess.TimeoutExpired:
        os.killpg(proc.pid, signal.SIGKILL)
        out, _ = proc.communicate()
        rc = 124
    finally:
        CHILDREN.discard(proc.pid)
    if log is not None:
        log.append(out[-3000:])
    return rc, out


def caught_by(out, test, expect):
    """Whether the `go test -json` output `out` shows `test` failing with `expect` in its own output.

    Both events must belong to the named test (or one of its subtests for the text): a build failure
    (events without a test), another failing test, or the text printed by another test is not a catch.
    """
    failed, said = False, False
    for line in out.splitlines():
        try:
            ev = json.loads(line)
        except ValueError:
            continue  # not an event: the output of go build, heavy.sh or timeout
        if not isinstance(ev, dict):
            continue
        name = ev.get("Test", "")
        if ev.get("Action") == "fail" and name == test:
            failed = True
        elif ev.get("Action") == "output" and (name == test or name.startswith(test + "/")) and expect in ev.get("Output", ""):
            said = True
    return failed and said


def run_go(m, tree, run=None):
    """Run the mutant's Go test in the copy. `run` is the command runner (default `sh`; tests inject one)."""
    run = run or sh
    log = []
    expect = m.get("expect")
    name = m["test"].strip("^$")  # the entry may write the test as an anchored pattern
    cmd = ["./scripts/heavy.sh", "timeout", "900", "go", "test", m["pkg"], "-run", f"^{name}$" if expect else m["test"], "-count=1"]
    if expect:
        cmd.append("-json")
    rc, out = run(cmd, tree, log=log, timeout=1000)
    if expect:
        # Caught only if the named test failed with its assertion text: not by a build error, a timeout,
        # a kill or another failing test.
        caught = rc not in (0, 124, 137) and caught_by(out, name, expect)
        detail = f"go test exit {rc}" + ("" if caught else f"; {name} did not fail with {expect!r}")
        return caught, detail, log
    # Caught only by a real test failure: not by a build error, a timeout or a kill.
    caught = rc not in (0, 124, 137) and "--- FAIL" in out and "[build failed]" not in out
    return caught, f"go test exit {rc}" + ("" if caught or rc == 0 else " without a failing test (build error or timeout)"), log


def run_vitest(m, tree, run=None):
    """One Vitest file of the frontend, run in the copy. Caught only by a real failing assertion:
    a build or type error, an import error, a failed suite or a timeout is not a measured miss.
    With `name`, only a failure of the test with that exact title is a catch: another rule's
    test failing in the same file does not stand for it. `run` is the command runner (default
    `sh`; tests inject one)."""
    run = run or sh
    log = []
    report = tree / "frontend/.mutant-vitest.json"
    test = m["test"].removeprefix("frontend/")
    rc, out = run(["../scripts/heavy.sh", "timeout", "900", "pnpm", "--config.verifyDepsBeforeRun=false", "exec",
                   "vitest", "run", "--configLoader", "runner", "--reporter=json", f"--outputFile={report}", test],
                  tree / "frontend", log=log, timeout=1000)
    if rc == 124 or not report.exists():
        return False, f"vitest exit {rc} without a report (timeout or crash)", log
    r = json.loads(report.read_text())
    failed = [a for f in r["testResults"] for a in f["assertionResults"] if a["status"] == "failed"]
    name = m.get("name")
    if name:
        caught = any(a["title"] == name for a in failed)
        detail = f"{name!r} failed" if caught else (
            f"{name!r} did not fail; " + (f"another test failed: {failed[0]['title'][:80]}" if failed else
                                         f"no failing assertion (vitest exit {rc})"))
        return caught, detail, log
    caught = bool(failed)
    detail = f"{len(failed)} failing test(s): {failed[0]['title'][:80]}" if caught else (
        f"no failing assertion (vitest exit {rc}, {r.get('numFailedTestSuites', 0)} failed suite(s): build, type or import error?)"
        if rc else "all tests pass")
    return caught, detail, log


def build_tree(tree, run, log, frontend=True, caps=None):
    """Build what a browser test of the copy runs: its frontend (when asked) and its two Go binaries, into tree/bin.
    Return (bin directory, None), or (None, the reason the build failed). `caps` gives each step's limit in seconds
    (the playwright kind's, under its gate's); the e2e kind keeps 900 s each."""
    caps = caps or {"frontend": 900, "inkwell-server": 900, "inkwell-relay": 900}
    if frontend:
        rc, _ = run(["pnpm", "--config.verifyDepsBeforeRun=false", "run", "build"], tree / "frontend",
                    {"VITE_E2E_OBSERVE": "1"}, log, timeout=caps["frontend"])
        if rc:
            return None, f"frontend build failed (exit {rc})"
    bins = tree / "bin"
    bins.mkdir(exist_ok=True)
    for out_name, target, env, tags in (("inkwell-server", "./cmd/inkwell", {}, ["-tags", "server"]),
                                        ("inkwell-relay", "./cmd/inkwell-relay", {"CGO_ENABLED": "0"}, [])):
        # The mutant tree is an rsync copy without .git: -buildvcs=false keeps the build from stamping, or failing on,
        # whatever repository a parent directory holds (an empty /tmp/.git failed MN1 and MN2 with "error obtaining
        # VCS status", 2026-11-23; the same class as scripts/pkg/selftest.sh, f5d2b55).
        rc, _ = run(["./scripts/heavy.sh", "go", "build", "-buildvcs=false", *tags, "-o", str(bins / out_name), target],
                    tree, env, log, timeout=caps[out_name])
        if rc:
            return None, f"go build of {out_name} failed (the mutant must compile)"
    return bins, None


def run_e2e(m, tree):
    log = []
    bins, err = build_tree(tree, sh, log, frontend=any(e["file"].startswith("frontend/") for e in m["edit"]))
    if err:
        return False, err, log
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


PLAYWRIGHT_FIELDS = ("spec", "grep", "project")
# The playwright kind runs under its gate's 240 s (gates/EX0c2.toml, `e2e-mutant`): each step's limit, all four
# together under it, so a slow step fails here with its reason rather than by the gate's kill.
PLAYWRIGHT_BUDGET = 240
PLAYWRIGHT_CAPS = {"frontend": 50, "inkwell-server": 50, "inkwell-relay": 50, "test": 80}


def validate(mutants):
    """Static checks of the mutant list; return the problems found."""
    bad = []
    for m in mutants:
        if "retired" in m and (not isinstance(m["retired"], str) or not m["retired"].strip()):
            bad.append(f"mutant {m['id']}: retired needs its reason (a non-empty text)")
        if "expect" in m and (m["kind"] != "go" or not m["expect"]):
            bad.append(f"mutant {m['id']}: expect is for a go mutant, and not empty")
        if "name" in m and (m["kind"] != "vitest" or not isinstance(m["name"], str) or not m["name"].strip()):
            bad.append(f"mutant {m['id']}: name is the title of a vitest test, and not empty")
        if m["kind"] not in ("go", "vitest", "e2e", "playwright"):
            bad.append(f"mutant {m['id']}: unknown kind {m['kind']!r}")
        if m["kind"] == "playwright":
            for field in PLAYWRIGHT_FIELDS:
                if not isinstance(m.get(field), str) or not m[field].strip():
                    bad.append(f"mutant {m['id']}: a playwright entry needs {field} (a non-empty text)")
            if isinstance(m.get("spec"), str) and not m["spec"].startswith("frontend/e2e/"):
                bad.append(f"mutant {m['id']}: spec is a file of frontend/e2e/")
            if m.get("project") not in (None, "chromium", "webkit"):
                bad.append(f"mutant {m['id']}: project is chromium or webkit")
        elif any(field in m for field in ("spec", "grep", "project", "error")):
            bad.append(f"mutant {m['id']}: spec, grep, project and error are for a playwright entry")
        if m["kind"] == "e2e" and m["scenario"] == 8 and not m.get("variant"):
            bad.append(f"mutant {m['id']}: scenario 8 needs a variant (editor or reader)")
    return bad


def playwright_caught(report, grep, error=None):
    """(caught, detail) from a Playwright JSON report: every run of the tests whose title starts with `grep` failed,
    and, with `error`, each failure says it. No such test, one passing, or a report without results is not a catch."""
    suites, runs = list(report.get("suites", [])), []
    while suites:
        s = suites.pop()
        suites.extend(s.get("suites", []))
        for spec in s.get("specs", []):
            if spec.get("title", "").startswith(grep):
                for t in spec.get("tests", []):
                    runs += [(r.get("status"), json.dumps(r.get("errors", []), ensure_ascii=False))
                             for r in t.get("results", [])]
    if not runs:
        return False, f"no test titled {grep!r} ran"
    if not all(st in ("failed", "timedOut") for st, _ in runs):
        return False, f"{grep!r} passed ({[st for st, _ in runs]})"
    if error and not all(error in e for _, e in runs):
        return False, f"{grep!r} failed without saying {error!r}"
    return True, f"{grep!r} failed" + (f" with {error!r}" if error else "")


def run_playwright(m, tree, run=None):
    """Build the copy's frontend and server, run the entry's spec there in its browser; caught as `playwright_caught`.
    `run` is the command runner (default `sh`; tests inject one)."""
    run = run or sh
    log = []
    _, err = build_tree(tree, run, log, caps=PLAYWRIGHT_CAPS)
    if err:
        return False, err, log
    report = tree / ".task/mutant-playwright.json"
    report.parent.mkdir(parents=True, exist_ok=True)
    report.unlink(missing_ok=True)
    rc, _ = run(["../scripts/heavy.sh", "timeout", str(PLAYWRIGHT_CAPS["test"] - 5), "./node_modules/.bin/playwright", "test", m["spec"].removeprefix("frontend/"), "--grep", m["grep"],
                 "--retries=0", "--reporter=json"], tree / "frontend",
                {"INKWELL_E2E_BROWSER": m["project"], "PLAYWRIGHT_JSON_OUTPUT_NAME": str(report)}, log=log,
                timeout=PLAYWRIGHT_CAPS["test"])
    if rc == 124:
        return False, "timeout (not a failing test)", log
    if not report.exists():
        return False, f"no Playwright report (exit {rc})", log
    caught, detail = playwright_caught(json.loads(report.read_text()), m["grep"], m.get("error"))
    return caught, detail, log


def duplicate_ids(mutants):
    """The (block, id) pairs written more than once: an id is unique within its block (the selection's scope)."""
    seen, bad = set(), []
    for m in mutants:
        key = (m["block"], m["id"])
        if key in seen:
            bad.append(f"block {key[0]}: mutant id {key[1]} written twice")
        seen.add(key)
    return bad


def anchor_problem(m, root=ROOT):
    """Why the entry's edits do not apply to the code under root (None if they do, or if the entry is retired)."""
    if m.get("retired"):
        return None
    with tempfile.TemporaryDirectory(prefix="netsim-check-") as d:
        tree = pathlib.Path(d)
        for e in m["edit"]:
            if not e.get("create"):
                src = root / e["file"]
                if not src.exists():
                    return f"{e['file']} does not exist"
                dest = tree / e["file"]
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src, dest)
        return apply(m["edit"], tree)


def select(mutants, block, only):
    """The entries of block to run (all of it, or the ids in only), and the refusals: a retired entry named."""
    chosen, refused = [], []
    for m in mutants:
        if m["block"] != block or (only and m["id"] not in only):
            continue
        if m.get("retired"):
            if only:
                refused.append(f"mutant {m['id']} is retired: {m['retired']}")
            continue
        chosen.append(m)
    return chosen, refused


# What a gate's criterion names: the runner with a block and its ids, `m.get("block") == B` in an inline check, id
# literals, `[f'MS{i}' for i in range(a, b)]` and `for i in $(seq -w a b) ... "id = \"P$i\""`.
_RUNNER = re.compile(r"(?:mutants\.py(?:[ \t]+--check)?|netsim:mutants[ \t]+--)[ \t]+([A-Za-z0-9_-]+)((?:[ \t]+[A-Za-z0-9_-]+)*)")
_BLOCK = re.compile(r"""get\(\s*["']block["']\s*\)\s*==\s*["']([A-Za-z0-9_-]+)["']""")
_RANGE = re.compile(r"""f["']([A-Za-z0-9-]*)\{i\}["']\s+for\s+i\s+in\s+range\((\d+),\s*(\d+)\)""")
_SEQ = re.compile(r'seq -w (\d+) (\d+)\).*?id = \\"([A-Za-z0-9-]*)\$i\\"', re.S)


def criterion_refs(command):
    """(blocks, ids) a criterion's command names, or None when it does not run nor read the catalogue."""
    blocks, ids = set(), set()
    for b, rest in _RUNNER.findall(command):
        blocks.add(b)
        ids.update(rest.split())
    blocks.update(_BLOCK.findall(command))
    if not blocks:
        return None
    for prefix, a, b in _RANGE.findall(command):
        ids.update(f"{prefix}{i}" for i in range(int(a), int(b)))
    for a, b, prefix in _SEQ.findall(command):
        ids.update(f"{prefix}{i:0{len(b)}d}" for i in range(int(a), int(b) + 1))
    ids.update(re.findall(r"\bM[A-Z]{0,2}\d+[a-z]?\b", command))
    return blocks, ids


def gate_problems(mutants, root=ROOT):
    """Entries named by a gate and missing from the catalogue (refusals), and retired ones a gate names (notes).

    A gate is checked once it has a result (gates/<lot>.json) or once its block has an entry: step 1 freezes a gate
    before step 2 writes its entries.
    """
    by_block = {}
    for m in mutants:
        by_block.setdefault(m["block"], {})[m["id"]] = m
    bad, notes = [], []
    for gate in sorted((root / "gates").glob("*.toml")):
        rel = gate.relative_to(root)
        ran = (root / "gates" / f"{gate.stem}.json").exists()
        for c in tomllib.loads(gate.read_text()).get("criteria", []):
            refs = criterion_refs(c.get("command", ""))
            if refs is None:
                continue
            blocks, ids = refs
            if not ran and not any(b in by_block for b in blocks):
                continue
            where = f"{rel}: criterion {c.get('id')}"
            for b in sorted(blocks):
                if not any(not m.get("retired") for m in by_block.get(b, {}).values()):
                    (notes if b in by_block else bad).append(
                        f"{where} runs block {b}, " + ("all of whose entries are retired" if b in by_block else "which has no entry"))
            for i in sorted(ids):
                found = [by_block[b][i] for b in blocks if i in by_block.get(b, {})]
                if not found:
                    bad.append(f"{where} names mutant {i}, missing from block(s) {', '.join(sorted(blocks))}")
                elif all(m.get("retired") for m in found):
                    notes.append(f"{where} names {i}, retired (not replayable as frozen): {found[0]['retired']}")
    return bad, notes


def check_catalogue(mutants):
    """`--check` without a block: every refusal of the whole catalogue, printed; return the exit code."""
    problems = validate(mutants) + duplicate_ids(mutants)
    for m in mutants:
        err = anchor_problem(m)
        if err:
            problems.append(f"block {m['block']}: mutant {m['id']}: {err}")
    bad, notes = gate_problems(mutants)
    for n in notes:
        print(f"note: {n}")
    for p in problems + bad:
        print(p)
    retired = sum(1 for m in mutants if m.get("retired"))
    print(f"netsim mutants: {len(mutants)} entries ({retired} retired), {len(problems) + len(bad)} problem(s)")
    return 1 if problems or bad else 0


def main():
    install_signals()
    args = sys.argv[1:]
    check_only = "--check" in args
    args = [a for a in args if a != "--check"]
    catalogue = tomllib.loads((ROOT / "scripts/netsim/mutants.toml").read_text())["mutant"]
    if not args:
        if check_only:
            sys.exit(check_catalogue(catalogue))
        sys.exit(__doc__)
    block, only = args[0], set(args[1:])
    mutants, refused = select(catalogue, block, only)
    for m in catalogue:
        if m["block"] == block and m.get("retired") and not only:
            print(f"mutant {m['id']}: retired, not run ({m['retired']})")
    if refused:
        sys.exit("netsim mutants: " + "; ".join(refused))
    if not mutants:
        sys.exit(f"netsim mutants: no mutant for block {block}")
    problems = validate(mutants) + duplicate_ids([m for m in catalogue if m["block"] == block])
    if problems:
        sys.exit("netsim mutants: " + "; ".join(problems))
    if check_only:  # anchors only, on a throwaway copy of each file: nothing is run
        for m in mutants:
            err = anchor_problem(m)
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
            caught, detail, log = {"go": run_go, "vitest": run_vitest, "e2e": run_e2e,
                                   "playwright": run_playwright}[m["kind"]](m, tree)
            # A Go mutant names its failure text, a netsim one its clause, a Playwright one its test.
            target = m.get("clause") or m.get("expect") or m.get("grep")
            print(f"mutant {m['id']} -> {target}: {'caught' if caught else 'NOT CAUGHT'} ({detail})", flush=True)
            if not caught:
                failures.append(m["id"])
                print("\n".join(log)[-1500:])
    if failures:
        sys.exit("netsim mutants: not caught: " + ", ".join(failures))
    print(f"netsim mutants: all {len(mutants)} caught")


if __name__ == "__main__":
    main()
