#!/usr/bin/env python3
"""Checks a `go test -json` run against the mandatory inventory (block SB2a1, decision D7).

  gojson_check.py RUN.json [--exit RC] --inventory gates/inventories/SB2a1.toml --columns local-N,local-F

Fails (exit 1, one problem per line on stderr) on: a non-zero exit status; any failed or unfinished test or package,
in or out of the inventory (a package finishes with its terminal `pass`, or `skip` when it ran no test); a required cell whose test is missing, failed or skipped; a test of tier F (a name segment
`F` or `F-...`) that skipped; `n-unavailable` accepted only in a cell that allows it, outside an F column, for a test
that skipped, and only when that test's output (or its parent's or a subtest's) holds the tier report line
`tier-report: tier=<N|F|refused> n=<available|unavailable> [errno=...]` saying n=unavailable; a `refused` cell whose
test did not pass with the refusal sentence (`preview.fail.sandbox` or its French text) in its output.

Cells (see gates/inventories/SB2a1.toml): "pass", "pass|n-unavailable", "refused", "-" (required nowhere; if the test
runs it must pass). Standard library only; `tomllib` needs Python 3.11."""
import argparse
import json
import re
import sys
import tomllib
from pathlib import Path

REPORT = re.compile(r"tier-report:(?P<kv>(?:\s+\w+=\S+)+)")
REFUSAL_MARKERS = ("preview.fail.sandbox", "ne fournit pas la protection")
CELLS = {"pass", "pass|n-unavailable", "refused", "-"}
# A subtest path segment naming tier F: `F`, `F-abi3`, `F-provided-font`.
F_SEGMENT = re.compile(r"^F($|-)")


class CheckError(Exception):
    """The check could not be made (bad inventory, unknown column); not a verdict on the run."""


def load_inventory(path):
    inv = tomllib.loads(Path(path).read_text())
    return {"columns": inv.get("columns", []), "tests": inv.get("test", [])}


class Run:
    """The events of one `go test -json` stream, by (package, test)."""

    def __init__(self):
        self.status = {}  # (package, test) -> run | pass | fail | skip
        self.output = {}  # (package, test) -> [line]
        self.package_failed = []
        self.package_state = {}  # package -> run | pass | fail | skip (its terminal event, if any)
        self.package_tested = set()  # packages with at least one test event
        self.bad_lines = []

    def add(self, line, number):
        line = line.strip()
        if not line:
            return
        try:
            e = json.loads(line)
            action = e["Action"]
        except (ValueError, KeyError, TypeError):
            self.bad_lines.append(number)
            return
        key = (e.get("Package", ""), e.get("Test", ""))
        self.package_state.setdefault(key[0], "run")
        if not key[1]:
            if action == "fail":
                self.package_failed.append(key[0])
            if action in ("pass", "fail", "skip"):
                self.package_state[key[0]] = action
            return
        self.package_tested.add(key[0])
        if action == "output":
            self.output.setdefault(key, []).append(e.get("Output", ""))
        elif action in ("run", "pass", "fail", "skip"):
            self.status[key] = action

    def names(self):
        return {t for _, t in self.status}

    def outcome(self, name):
        """The worst status of the test across packages, or None when it never ran."""
        seen = {s for (_, t), s in self.status.items() if t == name}
        for s in ("fail", "run", "skip", "pass"):
            if s in seen:
                return s
        return None

    def related_output(self, name):
        """The test's own output, its ancestors' and its subtests'."""
        lines = []
        for (_, t), out in self.output.items():
            if t == name or name.startswith(t + "/") or t.startswith(name + "/"):
                lines += out
        return lines


def is_f(name):
    return any(F_SEGMENT.match(seg) for seg in name.split("/"))


def n_unavailable_reported(lines):
    for line in lines:
        m = REPORT.search(line)
        if m and "n=unavailable" in m.group("kv").split():
            return True
    return False


def parse(lines):
    run = Run()
    for number, line in enumerate(lines, 1):
        run.add(line, number)
    return run


def check_stream(lines, inventory, columns, rc=None):
    run = parse(lines)
    return check_run(run, inventory, columns, rc)


def check_run(run, inventory, columns, rc=None):
    problems = []
    for c in columns:
        if c not in inventory["columns"]:
            raise CheckError(f"column {c!r} is not in the inventory ({inventory['columns']})")
    if rc:
        problems.append(f"go test exit status {rc}")
    problems += [f"line {n} is not JSON from go test -json" for n in run.bad_lines]
    problems += [f"package {p} failed" for p in run.package_failed]
    # A stream cut after the last required test but before its package's terminal event is not a pass: every
    # package ends in pass, or in skip when it ran no test (no test files).
    for p, s in sorted(run.package_state.items()):
        if s == "run":
            problems.append(f"package {p} did not finish")
        elif s == "skip" and p in run.package_tested:
            problems.append(f"package {p} ended in skip after running tests")
    if not run.status:
        problems.append("the run holds no test event")
    for name in sorted(run.names()):
        s = run.outcome(name)
        if s == "fail":
            problems.append(f"{name}: fail")
        elif s == "run":
            problems.append(f"{name}: did not finish")
        elif s == "skip" and is_f(name):
            problems.append(f"{name}: skipped in tier F (no skip is accepted there)")
    reported = set(problems)
    for column in columns:
        f_column = column.endswith("-F")
        for row in inventory["tests"]:
            name, cell = row["name"], row.get(column, "-")
            if cell not in CELLS:
                problems.append(f"{name} [{column}]: unknown status {cell!r}")
                continue
            if cell == "-":
                continue
            if cell == "pass|n-unavailable" and f_column:
                problems.append(f"{name} [{column}]: n-unavailable in an F column")
                continue
            s = run.outcome(name)
            if s is None:
                problems.append(f"{name} [{column}]: missing from the run")
            elif s == "pass":
                if cell == "refused" and not any(
                    m in line for line in run.related_output(name) for m in REFUSAL_MARKERS
                ):
                    problems.append(f"{name} [{column}]: refused expected, no refusal sentence in its output")
            elif s == "skip":
                if cell == "pass|n-unavailable" and not is_f(name):
                    if not n_unavailable_reported(run.related_output(name)):
                        problems.append(f"{name} [{column}]: skipped without a tier report saying n=unavailable")
                else:
                    problems.append(f"{name} [{column}]: skipped where {cell} is required")
            else:
                msg = f"{name} [{column}]: {s}"
                if f"{name}: " + ("fail" if s == "fail" else "did not finish") not in reported:
                    problems.append(msg)
    return problems


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("run", help="file holding the `go test -json` output")
    ap.add_argument("--exit", type=int, default=None, dest="rc", help="the exit status of go test")
    ap.add_argument("--inventory", required=True)
    ap.add_argument("--columns", required=True, help="comma-separated inventory columns")
    a = ap.parse_args(argv)
    try:
        inventory = load_inventory(a.inventory)
        with open(a.run, encoding="utf-8", errors="replace") as f:
            problems = check_stream(f, inventory, a.columns.split(","), a.rc)
    except (OSError, CheckError, tomllib.TOMLDecodeError) as err:
        print(f"gojson_check: {err}", file=sys.stderr)
        return 2
    for p in problems:
        print(f"gojson_check: {p}", file=sys.stderr)
    if problems:
        return 1
    print(f"gojson_check: {a.run}: columns {a.columns} satisfied")
    return 0


if __name__ == "__main__":
    sys.exit(main())
