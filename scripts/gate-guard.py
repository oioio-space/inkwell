#!/usr/bin/env python3
"""Fail closed on weakening relative to main, including unstaged/untracked work.

Overrides: gates/guard-overrides.json, {"overrides":[{"id": SHA256,
"reason": "...", "decision": "docs/..."}]}. IDs bind the exact finding.
This file and its decision documents must be listed in the lot raw_results.
Review overrides before merge; never generate them just to turn a gate green.
"""
import argparse
import collections
import difflib
import hashlib
import json
import pathlib
import re
import subprocess
import sys
import tomllib

ROOT = pathlib.Path(__file__).resolve().parent.parent
SKIP = re.compile(r'\b(?:t\s*\.\s*Skip(?:f|Now)?|(?:it|test|describe)\s*\.\s*skip)\s*\(')
TIMEOUT = re.compile(r'timeout|WithDeadline|Set(?:Read|Write)?Deadline|time\.(?:After|AfterFunc|NewTimer)', re.I)


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


def inspect(name, old, new):
    issues = []
    if name.startswith("gates/") and name.endswith(".toml"):
        a, b = tomllib.loads(old), tomllib.loads(new)
        previous = {c["id"]: c for c in a.get("criteria", [])}
        current = {c["id"]: c for c in b.get("criteria", [])}
        for ident, criterion in previous.items():
            if ident not in current:
                issues.append("removed criterion " + ident)
            elif criterion.get("status") != "deferred" and current[ident].get("status") == "deferred":
                issues.append("deferred criterion " + ident)
        # Thresholds may be nested anywhere in a gate, including criteria.
        for key in a:
            if key != "criteria":
                issues += numeric_changes(a[key], b.get(key), key)
        for ident in previous.keys() & current.keys():
            for key in previous[ident]:
                if key not in ("id", "command", "status", "reason", "target_lot"):
                    issues += numeric_changes(previous[ident][key], current[ident].get(key), ident + "." + key)
    if name.startswith("docs/perf/") and name.endswith("/thresholds.json"):
        issues += numeric_changes(json.loads(old), json.loads(new or "null")) if old else []
    if name.endswith((".go", ".ts", ".tsx", ".js", ".mjs")):
        if any(SKIP.search(line[2:]) for line in difflib.ndiff(old.splitlines(), new.splitlines()) if line.startswith("+ ")):
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


def run(base):
    git("rev-parse", "--verify", base)
    names = set(git("diff", "--name-only", base, "--").decode().splitlines())
    names.update(git("ls-files", "--others", "--exclude-standard").decode().splitlines())
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
    failed = []
    for name in sorted(names):
        if name.startswith(("frontend/node_modules", "frontend/bindings", "gates/L0/", "gates/L1/")):
            continue
        try:
            old = git("show", f"{base}:{name}").decode()
        except subprocess.CalledProcessError:
            old = ""
        except UnicodeDecodeError:
            continue
        path = ROOT / name
        if path.is_symlink() or path.is_dir():
            continue
        try:
            new = path.read_text() if path.exists() else ""
        except UnicodeDecodeError:
            continue
        for issue in inspect(name, old, new):
            finding = f"{name}: {issue}"
            ident = hashlib.sha256((finding + "\n" + old + "\n" + new).encode()).hexdigest()
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
    print("gate guard self-tests: pass")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default="main")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
    else:
        sys.exit(run(args.base))
