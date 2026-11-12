#!/usr/bin/env python3
"""Fail closed on weakening relative to main, including unstaged/untracked work.

Threat model: this guard protects against an agent weakening a gate by
CONVENIENCE or by MISTAKE (a skipped test, a raised timeout, a relaxed
command, an edited runner), not against a deliberately malicious agent, which
could always hide a weakening somewhere the guard does not look. It stays
proportionate on purpose: explicit, reviewable rules and a reviewed override
file, no attempt to sandbox the repository.

What is flagged:
- a removed criterion, and ANY change to a criterion's command, status or
  target_lot in gates/*.toml; a new criterion that starts out deferred;
- any change to the gate machinery (scripts/gate*.py, scripts/gate*.sh,
  scripts/perf/**, scripts/heavy.sh whose --exclusive mode isolates
  measurements) and to the immutable baseline archive (docs/perf/baseline/**);
- any change to what gate criteria call: the mise tasks named by a criterion
  command (closed over `depends` and nested `mise run`), every scripts/* file
  those commands and tasks name (closed over scripts naming scripts), and the
  tool configurations they read (CONFIGS), all as of the merge-base;
- a test file deleted or renamed away;
- weakened numeric thresholds, removed or raised timeouts;
- added test skips on any receiver (Go `.Skip(`, `.Skipf(`, `.SkipNow(`,
  `testing.Short()`, build constraints added to existing tests; JS/TS `it|test|describe….skip|
  only|todo|fixme|skipIf|runIf`, `xit(`, `fit(`…), including ones split across
  lines (`t.\nSkip(`): the number of skips in a file may not grow.

The gate runs this guard from the merge-base's version (scripts/gate.py
base_guard), so a lot cannot rewrite the guard that judges it.

Overrides: gates/guard-overrides.json, {"overrides":[{"id": SHA256,
"reason": "...", "decision": "docs/..."}]}. IDs bind the exact finding and the
exact old and new file contents. This file and its decision documents must be
listed in the lot raw_results; the gate copies the file into its manifest.
Review overrides before merge; never generate them just to turn a gate green.
"""
import argparse
import collections
import contextlib
import difflib
import fnmatch
import hashlib
import io
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import tomllib

ROOT = pathlib.Path(__file__).resolve().parent.parent
SKIP_GO = re.compile(r'\.\s*Skip(?:f|Now)?\b|\btesting\s*\.\s*Short\s*\(')
BUILD_GO = re.compile(r'^\s*//\s*(?:go:build|\+build)\b')
SKIP_JS = re.compile(r'\b(?:it|test|describe|suite|bench|context)(?:\s*\.\s*\w+)*?\s*\.\s*'
                     r'(?:skip|skipIf|runIf|only|todo|fixme)\b|\b(?:xit|xtest|xdescribe|fit|fdescribe)\s*\(')
JS = (".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".svelte")
TIMEOUT = re.compile(r'timeout|WithDeadline|Set(?:Read|Write)?Deadline|time\.(?:After|AfterFunc|NewTimer)', re.I)
MACHINERY = ("scripts/gate*.py", "scripts/gate*.sh", "scripts/perf/*", "scripts/heavy.sh")
BASELINE = "docs/perf/baseline/*"
# Fields compared as a whole: any change at all is a finding.
CRITERION_FIELDS = ("command", "status", "target_lot")
# Configurations the criteria's tools read implicitly (golangci-lint, Vite and
# Vitest, Playwright, Svelte): no command names them, so they are listed.
CONFIGS = (".golangci.yml", "frontend/vite.config.ts", "frontend/playwright.config.ts",
           "frontend/playwright.netsim.config.ts", "frontend/svelte.config.js")
SCRIPT_REF = re.compile(r'(?<![\w/])(?:\.{1,2}/)*(scripts/[\w./-]+\.(?:sh|py|cjs|mjs|js))')
TASK_REF = re.compile(r'\bmise\s+run\s+(?:--?[\w-]+\s+)*([\w:.][\w:.-]*)')  # skip flags such as --skip-deps
TEST_FILE = re.compile(r'(?:_test\.go|\.(?:test|spec)\.[cm]?[jt]sx?)$')
Dependencies = collections.namedtuple("Dependencies", "tasks files")
# Local mise configuration that mise merges over mise.toml (often gitignored),
# so it can silently replace a gate task. A local TOML file is a finding only when
# it touches tasks (an env-only file, e.g. holding a personal API key, is not);
# file tasks under a tasks directory always are.
MISE_OVERRIDES = ("mise.local.toml", ".mise.toml", ".mise.local.toml", "mise.*.toml", ".mise.*.toml",
                  ".config/mise*.toml", ".config/mise/**/*", "mise/**/*", ".mise/**/*",
                  "mise-tasks/**/*", ".mise-tasks/**/*")
TASK_KEYS = {"tasks", "task_config"}


def overrides_tasks(path):
    if path.suffix != ".toml":
        return True
    try:
        return bool(TASK_KEYS & tomllib.loads(path.read_text()).keys())
    except (OSError, tomllib.TOMLDecodeError, UnicodeDecodeError):
        return True  # unreadable: assume it may replace tasks


def mise_overrides(root):
    found = {p for pattern in MISE_OVERRIDES for p in root.glob(pattern) if p.is_file() or p.is_symlink()}
    return sorted(str(p.relative_to(root)) for p in found if overrides_tasks(p))
IGNORED = re.compile(r'^(?:frontend/node_modules|frontend/bindings)/|^gates/[^/]+/')


def numeric_changes(old, new, path=""):
    """Unknown changes to a protected threshold fail closed."""
    if isinstance(old, dict) and isinstance(new, dict):
        return [issue for key in old for issue in
                numeric_changes(old[key], new.get(key), path + "." + key)]
    if old == new:
        return []
    if isinstance(old, (int, float)) and not isinstance(old, bool) and isinstance(new, (int, float)):
        # Lower bounds (coverage, minimum throughput) have the inverse direction.
        minimum = bool(re.search(r'(^|[._-])(min|minimum|floor|coverage|p_less_than|confidence|samples?|throughput|fps)([._-]|$)', path, re.I))
        maximum = bool(re.search(r'(^|[._-])(max|maximum|limit|ceiling|delta|multiplier|p95|latency|duration|timeout|budget|ms|bytes)([._-]|$)', path, re.I))
        if minimum and new >= old or not minimum and maximum and new <= old:
            return []
    return [f"threshold {path}: {old!r} -> {new!r}"]


def timeout_values(text):
    result = collections.defaultdict(list)
    for line in text.splitlines():
        if not TIMEOUT.search(line):
            continue
        # Keep the expression structure: a change of units or expression cannot
        # silently pass. Numeric increases fail; unknown rewrites fail closed.
        key = re.sub(r'\d+(?:\.\d+)?', '#', line.strip())
        result[key].append(tuple(float(n) for n in re.findall(r'\d+(?:\.\d+)?', line)))
    return result


def gate_toml(old, new):
    issues = []
    a, b = tomllib.loads(old) if old else {}, tomllib.loads(new) if new else {}
    previous = {c["id"]: c for c in a.get("criteria", [])}
    current = {c["id"]: c for c in b.get("criteria", [])}
    for ident, criterion in previous.items():
        if ident not in current:
            issues.append("removed criterion " + ident)
            continue
        for key in CRITERION_FIELDS:
            if criterion.get(key) != current[ident].get(key):
                issues.append(f"criterion {ident} {key} changed: {criterion.get(key)!r} -> {current[ident].get(key)!r}")
        for key in criterion:
            if key not in ("id", "reason", "target_lot_name", *CRITERION_FIELDS):
                issues += numeric_changes(criterion[key], current[ident].get(key), ident + "." + key)
    for ident, criterion in current.items():
        if ident not in previous and criterion.get("status") == "deferred":
            issues.append("new criterion starts deferred " + ident)
    # Thresholds may be nested anywhere in a gate, including criteria.
    for key in a:
        if key not in ("criteria", "measurement_reports"):
            # Report locations are evidence paths, not acceptance thresholds.
            issues += numeric_changes(a[key], b.get(key), key)
    return issues


def dependencies(read, gate_files):
    """The mise tasks and files gate criteria run, read through read(name)."""
    tasks = tomllib.loads(read("mise.toml") or "").get("tasks", {})
    seen, files = set(), set(CONFIGS)
    pending = ["mise run gate"] + [c.get("command", "") for name in gate_files
                                   for c in tomllib.loads(read(name) or "").get("criteria", [])]
    direct = set(pending)
    while pending:
        command = pending.pop()
        for task in TASK_REF.findall(command):
            if task in seen or task not in tasks:
                continue
            seen.add(task)
            run = tasks[task].get("run", [])
            pending += [run] if isinstance(run, str) else list(run)
            pending += ["mise run " + d for d in tasks[task].get("depends", [])]
        for script in SCRIPT_REF.findall(command):
            # A path only named in a script's text (a test fixture, a message) that is no file cannot
            # change what a gate runs; a criterion naming a missing script is caught by the self-test.
            if script not in files and (command in direct or (ROOT / script).is_file()):
                files.add(script)
                # Comments name tasks and scripts that the script does not run.
                pending.append("\n".join(line for line in (read(script) or "").splitlines()
                                          if not line.lstrip().startswith(("#", "//"))))
    return Dependencies(frozenset(seen), frozenset(files))


def skip_count(name, content):
    if name.endswith(".go"):
        return len(SKIP_GO.findall(content))
    return len(SKIP_JS.findall(content)) if name.endswith(JS) else 0


def added_lines(old, new):
    return [line[2:] for line in difflib.ndiff(old.splitlines(), new.splitlines()) if line.startswith("+ ")]


def inspect(name, old, new, deps=Dependencies(frozenset(), frozenset())):
    issues = []
    if name in deps.files and old != new:
        issues.append("gate dependency changed")
    if TEST_FILE.search(name) and old and not new:
        issues.append("test file deleted or renamed away")
    if fnmatch.fnmatchcase(name, BASELINE) and old != new:
        issues.append("immutable baseline archive changed")
    if any(fnmatch.fnmatchcase(name, p) for p in MACHINERY) and old != new:
        issues.append("gate machinery changed")
    if name == "mise.toml":
        tasks = lambda text: tomllib.loads(text).get("tasks", {}) if text else {}
        for task in sorted(deps.tasks | {"gate"}):
            if tasks(old).get(task) != tasks(new).get(task):
                issues.append("gate dependency changed: mise task " + task)
    if name.startswith("gates/") and name.endswith(".toml") and "/" not in name[len("gates/"):]:
        issues += gate_toml(old, new)
    if name.startswith("docs/perf/") and name.endswith("/thresholds.json"):
        issues += numeric_changes(json.loads(old), json.loads(new or "null")) if old else []
    if name.endswith(".go"):
        added = added_lines(old, new)
        if any(SKIP_GO.search(line) for line in added):
            issues.append("added test skip")
        # A constraint added to an existing test file can exclude its tests.
        if name.endswith("_test.go") and old and any(BUILD_GO.search(line) for line in added):
            issues.append("added build constraint to a test file")
    if name.endswith(JS) and any(SKIP_JS.search(line) for line in added_lines(old, new)):
        issues.append("added test skip")
    # A skip split across lines escapes the per-line check, not the count.
    if skip_count(name, new) > skip_count(name, old) and "added test skip" not in issues:
        issues.append("added test skip")
    # Check timeout settings in tests, configs and commands alike.
    before, after = timeout_values(old), timeout_values(new)
    for key, values in before.items():
        if key not in after:
            if any(TIMEOUT.search(line) for line in new.splitlines()):
                issues.append("timeout expression removed or rewritten: " + key)
            else:
                issues.append("timeout removed: " + key)
        elif len(values) != len(after[key]) or any(
                len(a) != len(b) or any(y > x for x, y in zip(a, b))
                for a, b in zip(values, after[key])):
            issues.append("timeout increased: " + key)
    return issues


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, stderr=subprocess.PIPE)


def text(data):
    # Binary files are compared too: surrogateescape keeps every byte.
    return data.decode("utf-8", "surrogateescape")


def load_overrides():
    overrides_path = ROOT / "gates/guard-overrides.json"
    overrides = {}
    listed = set()
    for manifest in (ROOT / "gates").glob("*.toml"):
        listed.update(tomllib.loads(manifest.read_text()).get("raw_results", []))
    if overrides_path.exists():
        if "gates/guard-overrides.json" not in listed:
            raise ValueError("override file must be listed in manifest raw_results")
        for item in json.loads(overrides_path.read_text())["overrides"]:
            decision = pathlib.PurePosixPath(item.get("decision", ""))
            if not decision.parts or decision.parts[0] != "docs" or ".." in decision.parts:
                raise ValueError("override decision must be a repository docs path")
            if not item.get("reason", "").strip() or not (ROOT / decision).is_file():
                raise ValueError("override requires reason and existing decision document")
            if item["decision"] not in listed:
                raise ValueError("override decision must be listed in manifest raw_results")
            overrides[item["id"]] = item
    return overrides


def run(base):
    git("rev-parse", "--verify", base)
    # --no-renames: a renamed file is a deletion plus an addition, so
    # `git mv x_test.go x.go` still shows the test file removed.
    names = set(git("diff", "--no-renames", "--name-only", base, "--").decode().splitlines())
    names.update(git("ls-files", "--others", "--exclude-standard").decode().splitlines())
    overrides = load_overrides()

    def read(name):
        try:
            return text(git("show", f"{base}:{name}"))
        except subprocess.CalledProcessError:
            return ""
    gate_files = [n for n in git("ls-tree", "--name-only", base, "gates/").decode().splitlines() if n.endswith(".toml")]
    deps = dependencies(read, gate_files)
    failed = []
    for name in mise_overrides(ROOT):
        failed.append(f"{name}: mise override present; it can replace gate tasks (remove or rename it, then rerun)")
    for name in sorted(names):
        if IGNORED.match(name):
            continue
        try:
            old = text(git("show", f"{base}:{name}"))
        except subprocess.CalledProcessError:
            old = ""
        path = ROOT / name
        if path.is_symlink():
            new = "symlink -> " + os.readlink(path)
        elif path.is_dir():
            continue
        else:
            new = text(path.read_bytes()) if path.exists() else ""
        for issue in inspect(name, old, new, deps):
            finding = f"{name}: {issue}"
            ident = hashlib.sha256((finding + "\n" + old + "\n" + new).encode("utf-8", "surrogateescape")).hexdigest()
            if ident not in overrides:
                failed.append(f"{ident}: {finding}")
            else:
                print("OVERRIDE", ident, overrides[ident]["reason"])
    print("\n".join(failed) if failed else "gate guard: pass")
    return bool(failed)


def self_test():
    assert inspect("gates/X.toml", '[[criteria]]\nid="a"\n', "") == ["removed criterion a"]
    assert inspect("docs/perf/x/thresholds.json", '{"max":10}', '{"max":11}')
    assert not inspect("docs/perf/x/thresholds.json", '{"max":10}', '{"max":9}')
    assert inspect("docs/perf/x/thresholds.json", '{"min_coverage":85}', '{"min_coverage":84}')
    assert inspect("x_test.go", "", 't.Skip("oops")')
    assert inspect("x.spec.ts", "", 'test.skip("oops", () => {})')
    assert inspect("x_test.go", 'WithTimeout(ctx, 2*time.Second)', 'WithTimeout(ctx, 3*time.Second)')
    assert inspect("x.ts", 'timeout: 100', 'timeout: 200')
    assert not inspect("x.ts", 'timeout: 200', 'timeout: 100')
    assert inspect("x.ts", 'timeout: 100', '')
    assert inspect("x_test.go", 'time.After(2*time.Second)', 'time.After(3*time.Second)')
    assert inspect("x.ts", 'timeout: 100', 'timeout: limit')
    assert inspect("x.go", 'WithTimeout(ctx, time.Second)', 'WithTimeout(ctx, time.Minute)')
    assert not inspect("x.go", "ordinary()", "ordinary()")
    assert inspect("x_test.go", 't.Skip("old")', 't.Skip("new")')
    assert inspect("docs/perf/x/thresholds.json", '{"max":10}', '{}')
    assert inspect("docs/perf/x/thresholds.json", '{"p_less_than":0.05}', '{"p_less_than":0.01}')
    assert inspect("docs/perf/x/thresholds.json", '{"samples":30}', '{"samples":20}')
    assert inspect("docs/perf/x/thresholds.json", '{"unknown":30}', '{"unknown":20}')
    assert inspect("gates/X.toml", '[[criteria]]\nid="a"\ncommand="true"\n', '[[criteria]]\nid="a"\nstatus="deferred"\n')

    # Review finding 1: every bypass named by the review is flagged.
    crit = '[[criteria]]\nid="a"\ncommand="go test ./..."\n'
    assert inspect("gates/X.toml", crit, crit.replace("go test ./...", "true")), "command replaced by true"
    assert inspect("gates/X.toml", crit, crit + 'status="fail"\n'), "status change"
    moved = '[[criteria]]\nid="a"\nstatus="deferred"\ntarget_lot="%s"\nreason="r"\n'
    assert inspect("gates/X.toml", moved % "L2", moved % "L9"), "deferral moved to another lot"
    assert not inspect("gates/X.toml", moved % "L2", (moved % "L2").replace('reason="r"', 'reason="clearer"'))
    assert inspect("gates/X.toml", crit, crit + moved.replace('"a"', '"b"') % "L2") == ["new criterion starts deferred b"]
    assert not inspect("gates/X.toml", crit, crit + '[[criteria]]\nid="b"\ncommand="true"\n'), "new enforced criterion"
    for line in ('tb.Skip("x")', 's.T().Skip("x")', 'b.SkipNow()', 'f.Skipf("%d", 1)', 'skip := t.Skip',
                 'if testing.Short() {'):
        assert inspect("x_test.go", "", line), line
    assert inspect("x_test.go", "package x\n", "//go:build ignore\npackage x\n")
    assert not inspect("race_test.go", "", "//go:build race\npackage x\n"), "new constrained test file"
    for line in ('it.skip("x", f)', 'describe.only("x", f)', 'test.describe.skip("x", f)', 'test.fixme()',
                 'it.todo("x")', 'test.skipIf(cond)("x", f)', 'xit("x", f)', 'fdescribe("x", f)'):
        assert inspect("x.test.ts", "", line), line
    assert not inspect("x.ts", "", "if (opts.skip?.(range)) return"), "ordinary skip option"
    assert inspect("scripts/perf/regression.py", "return delta > 5", "return delta > 50"), "threshold in code"
    for name in ("scripts/gate-guard.py", "scripts/gate.py", "scripts/gate.sh", "scripts/gate-selftest.py",
                 "scripts/perf/bench.sh", "scripts/perf/archive.py", "scripts/heavy.sh"):
        assert "gate machinery changed" in inspect(name, "a", "b"), name
        assert "gate machinery changed" in inspect(name, "a", ""), name + " deleted"
    assert inspect("mise.toml", '[tasks.gate]\nrun = "./scripts/gate.sh"\n', '[tasks.gate]\nrun = "true"\n')
    assert not inspect("mise.toml", '[tasks.gate]\nrun = "x"\n', '[tasks.gate]\nrun = "x"\n[tasks.y]\nrun = "y"\n')
    # Review finding 2: the archive changes only with an override, binary files included.
    assert inspect("docs/perf/baseline/2026-11-02/README.md", "a", "b")
    assert inspect("docs/perf/baseline/2026-11-02/browser.png", text(b"\x89PNG\xff"), text(b"\x89PNG\xfe"))
    assert inspect("docs/perf/baseline/2026-12-01/new.json", "", "{}"), "new archive"
    # Review 2: what criteria call is protected; every criterion resolves to it.
    def read(name):
        path = ROOT / name
        return path.read_text() if path.is_file() else ""
    gate_files = sorted(str(p.relative_to(ROOT)) for p in (ROOT / "gates").glob("*.toml"))
    deps = dependencies(read, gate_files)
    tasks = tomllib.loads(read("mise.toml"))["tasks"]
    for name in gate_files:
        for criterion in tomllib.loads(read(name)).get("criteria", []):
            command = criterion.get("command", "")
            for task in TASK_REF.findall(command):
                assert task in tasks and task in deps.tasks, (name, criterion["id"], task)
            for script in SCRIPT_REF.findall(command):
                assert (ROOT / script).is_file() and script in deps.files, (name, criterion["id"], script)
    assert {"arch", "test", "lint", "gate", "sidecar:fetch"} <= deps.tasks, deps.tasks
    assert {".golangci.yml", "frontend/vite.config.ts", "scripts/lint.sh", "scripts/arch-check.sh",
            "scripts/arch-exec-check.sh", "scripts/heavy.sh", "scripts/rtkx.sh", "scripts/gate.sh",
            "scripts/sidecar-no-tools.sh", "scripts/perf/baseline.py", "scripts/perf/automerge.cjs"} <= deps.files, deps.files
    assert all((ROOT / f).is_file() for f in deps.files), [f for f in deps.files if not (ROOT / f).is_file()]
    # A fixture path inside a dependency's text is not a dependency (scripts/netsim/affected_test.py names
    # scripts/perf/p.py as test data, 2026-11-11).
    assert "scripts/perf/p.py" not in deps.files
    assert inspect(".golangci.yml", "a", "b", deps) == ["gate dependency changed"]
    assert inspect("scripts/lint.sh", "a", "b", deps) == ["gate dependency changed"]
    lint = '[tasks.lint]\nrun = "./scripts/lint.sh"\n'
    assert inspect("mise.toml", lint, lint.replace("./scripts/lint.sh", "true"), deps) == ["gate dependency changed: mise task lint"]
    assert not inspect("mise.toml", lint, lint + '[tasks.unrelated]\nrun = "x"\n', deps)
    # Review 7: skips split across lines, and test files deleted or renamed away.
    assert inspect("x_test.go", "package x\n", 'package x\nfunc T(t *testing.T) { t.\nSkip("x") }\n')
    assert inspect("x.test.ts", "", "it\n  .skip('x', f)\n")
    assert inspect("x_test.go", "package x\n", "") == ["test file deleted or renamed away"]
    assert inspect("e2e/files.spec.ts", "test('a', f)\n", "") == ["test file deleted or renamed away"]
    assert not inspect("x.go", "package x\n", "")
    end_to_end_test()
    print("gate guard self-tests: pass")


def end_to_end_test():
    """run() in a temporary repository: a `git mv` of a test file, a mise override."""
    global ROOT
    with tempfile.TemporaryDirectory() as tmp:
        root = pathlib.Path(tmp)
        sh = lambda *args: subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args],
                                          cwd=root, check=True, capture_output=True)
        sh("init", "-q", "-b", "main")
        (root / "gates").mkdir()
        (root / "x_test.go").write_text("package x\n\nfunc TestX(t *testing.T) {}\n")
        (root / "mise.toml").write_text('[tasks.gate]\nrun = "./scripts/gate.sh"\n')
        sh("add", ".")
        sh("commit", "-q", "-m", "base")
        sh("checkout", "-q", "-b", "lot")
        saved, ROOT = ROOT, root
        try:
            def guard():
                out = io.StringIO()
                with contextlib.redirect_stdout(out):
                    failed = run("main")
                return failed, out.getvalue()
            assert guard() == (False, "gate guard: pass\n")
            sh("mv", "x_test.go", "x.go")
            sh("commit", "-q", "-m", "rename the test away")
            failed, log = guard()
            assert failed and "x_test.go: test file deleted or renamed away" in log, log
            sh("mv", "x.go", "x_test.go")
            sh("commit", "-q", "-m", "restore")
            assert guard() == (False, "gate guard: pass\n")
            for name in ("mise.local.toml", ".config/mise/conf.d/x.toml", "mise-tasks/gate"):
                (root / name).parent.mkdir(parents=True, exist_ok=True)
                (root / name).write_text('[tasks.gate]\nrun = "true"\n')
                failed, log = guard()
                assert failed and name + ": mise override present" in log, (name, log)
                (root / name).unlink()
            # An env-only local file (personal settings, API keys) replaces no task.
            (root / "mise.local.toml").write_text('[env]\nSOME_KEY = "x"\n')
            assert guard() == (False, "gate guard: pass\n"), "env-only mise.local.toml must pass"
            (root / "mise.local.toml").unlink()
        finally:
            ROOT = saved


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--base", default="main")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
    else:
        sys.exit(run(args.base))
