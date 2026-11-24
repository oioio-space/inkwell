#!/usr/bin/env python3
"""Rule G-2 of block CI3a: a gate whose block changes docs/protocol/spec.md runs the protocol module's tests.

PU1d's gate ran only the touched packages while protocol's TestLimitsMatchSpecification reads spec.md §16, which the
block changed; the defect was seen after the merge (fix 64a16dc). scripts/gate.py now refuses to start such a gate
unless a criterion runs `cd protocol && … go test ./...` (or `go -C protocol test ./...`) or names
TestLimitsMatchSpecification. The gates written before the rule are listed in gates/legacy-no-spec-tests.txt, a closed
list. Each test runs the working-tree runner on a temporary git repository, as scripts/gate_deadline_test.py does.
"""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import tomllib
import unittest
from unittest import mock

SCRIPTS = pathlib.Path(__file__).resolve().parent

# The gates committed before rule G-2 with no criterion running the protocol tests (computed at fbe3c8f). Closed: a
# name added to the file fails this set.
LEGACY = {"CI1", "CI1b", "CI1c", "CL1", "DU", "GR", "HP1", "HP1b", "HP1c", "HP1c2", "HP1m", "HP1n", "HP1n2", "HP1n3",
          "HP1n4", "HP1n5", "HP1r", "HP1r2", "L0", "L1", "L2", "L3", "LF0", "LF0a", "LF0a2", "LF0a3", "NI0", "OI", "P2",
          "P3", "PB", "PC1a", "PC1b", "PC1b2", "PC1n", "PC1n2", "PR1", "PU1", "PU1d", "PU1n", "PU1n2", "RC1", "RG", "RL1",
          "SB1", "SB1b", "SB2a1", "SI1", "SI1b", "SP1", "SP1b", "TC1", "W58", "W58a", "WB", "WC1"}

UNIT_ONLY = "timeout -k 10 60 touch sentinel/ran"
PROTOCOL_RUN = "cd protocol && timeout -k 10 60 ../scripts/heavy.sh go test ./... && cd .. && touch sentinel/ran"


def git(root, *args):
    subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args], cwd=root, check=True,
                   capture_output=True)


def load():
    spec = importlib.util.spec_from_file_location("gate_spec_tests_under_test", SCRIPTS / "gate.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.version = lambda command: "fake " + command
    return module


class SpecTests(unittest.TestCase):
    def setUp(self):
        tmp = pathlib.Path(tempfile.mkdtemp(prefix="gate-spec-tests-"))
        self.addCleanup(shutil.rmtree, tmp)
        self.root = tmp / "repo"
        archive = self.root / "docs/perf/baseline/2026-11-02"
        archive.mkdir(parents=True)
        (archive / "measurements.json").write_bytes(b'{"archived":true}\n')
        digest = hashlib.sha256((archive / "measurements.json").read_bytes()).hexdigest()
        (archive / "SHA256SUMS").write_text(digest + "  measurements.json\n")
        self.pins = {"2026-11-02": hashlib.sha256((archive / "SHA256SUMS").read_bytes()).hexdigest()}
        (self.root / "docs/protocol").mkdir(parents=True)
        (self.root / "docs/protocol/spec.md").write_text("## 16. Limits\n")
        (self.root / "docs/protocol/other.md").write_text("notes\n")
        (self.root / "gates").mkdir()
        (self.root / "sentinel").mkdir()
        (self.root / ".gitignore").write_text("/sentinel/\n/gates/*/*\n!/gates/*/*.md\n/gates/*.json\n")
        git(self.root, "init", "-q", "-b", "main")
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", "base")
        self.gate = load()
        env = mock.patch.dict(os.environ, {k: v for k, v in os.environ.items() if not k.startswith("INKWELL_")},
                              clear=True)
        env.start()
        self.addCleanup(env.stop)

    def write(self, command):
        # The fixture's protocol "module" is only what the command needs: an empty folder it can cd into.
        (self.root / "protocol").mkdir(exist_ok=True)
        (self.root / "gates/T.toml").write_text(
            f'max_minutes = 5\n[[criteria]]\nid = "a"\ncommand = {json.dumps(command)}\n')

    def run_gate(self):
        with contextlib.redirect_stdout(io.StringIO()):
            return self.gate.run("T", self.root, pins=self.pins, guard=False)

    def change_spec(self):
        with (self.root / "docs/protocol/spec.md").open("a") as f:
            f.write("| new limit | 64 |\n")

    def test_spec_change_without_protocol_tests_refused_before_any_run(self):
        self.change_spec()
        self.write(UNIT_ONLY)
        with self.assertRaisesRegex(ValueError, r"changes docs/protocol/spec\.md.*protocol module's tests"):
            self.run_gate()
        self.assertFalse((self.root / "sentinel/ran").exists(), "no criterion may run")
        self.assertFalse((self.root / "gates/T").exists(), "refused before the evidence is touched")

    def test_spec_change_committed_on_the_block_branch_is_seen(self):
        git(self.root, "switch", "-q", "-c", "T")
        self.change_spec()
        git(self.root, "commit", "-q", "-am", "block")
        self.write(UNIT_ONLY)
        with self.assertRaisesRegex(ValueError, r"changes docs/protocol/spec\.md"):
            self.run_gate()

    def test_spec_change_with_protocol_tests_runs(self):
        self.change_spec()
        # The real command would run the module's tests; the fixture stubs heavy.sh so only the shape is judged.
        (self.root / "scripts").mkdir()
        (self.root / "scripts/heavy.sh").write_text("#!/bin/sh\nexit 0\n")
        (self.root / "scripts/heavy.sh").chmod(0o755)
        self.write(PROTOCOL_RUN)
        self.assertEqual(self.run_gate(), 0)
        self.assertTrue((self.root / "sentinel/ran").exists())

    def test_accepted_command_shapes(self):
        accepted = ["cd protocol && ../scripts/heavy.sh go test ./...",
                    "(cd ./protocol && timeout 300 go test -count=1 ./...)",
                    "go -C protocol test ./...",
                    "go test -run 'TestLimitsMatchSpecification' ./trust"]
        refused = ["go test ./protocol/...", "cd protocol && go vet ./...", "go test ./internal/app/...",
                   "cd protocol\ngo test ./..."]
        for command in accepted:
            self.assertRegex(command, self.gate.PROTOCOL_TESTS, command)
        for command in refused:
            self.assertNotRegex(command, self.gate.PROTOCOL_TESTS, command)

    def test_other_changes_need_no_protocol_tests(self):
        (self.root / "docs/protocol/other.md").write_text("changed\n")
        self.write(UNIT_ONLY)
        self.assertEqual(self.run_gate(), 0)

    def test_legacy_gate_runs_without_protocol_tests(self):
        self.change_spec()
        self.write(UNIT_ONLY)
        (self.root / "gates/legacy-no-spec-tests.txt").write_text("# gates before rule G-2\nT\n")
        self.assertEqual(self.run_gate(), 0)

    def test_unknown_base_refuses(self):
        git(self.root, "branch", "-q", "-m", "main", "other")
        self.write(UNIT_ONLY)
        with self.assertRaisesRegex(ValueError, r"cannot tell.*merge-base"):
            self.run_gate()

    def test_legacy_list_is_closed(self):
        self.assertEqual(self.gate.listed_gates(SCRIPTS.parent, self.gate.LEGACY_SPEC_TESTS), LEGACY)
        for lot in LEGACY:
            config = tomllib.loads((SCRIPTS.parent / "gates" / (lot + ".toml")).read_text())
            self.assertFalse(any(self.gate.PROTOCOL_TESTS.search(c.get("command", "")) for c in config["criteria"]),
                             f"{lot} runs the protocol tests: it needs no listing")


if __name__ == "__main__":
    unittest.main()
