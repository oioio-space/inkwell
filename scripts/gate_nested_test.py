#!/usr/bin/env python3
"""No gate runs another gate (method v2, rule 6; user decision 2026-11-12).

A `gate -- X` inside a criterion of another gate does not run X: it is recorded as deferred, in the criterion's entry
and at the top of the parent manifest, and X runs on its own when needed. Each test runs the working-tree runner (a
copy, so a test can mutate it) on a temporary git repository, as scripts/gate_resume_test.py does.
"""
import hashlib
import importlib.util
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

SCRIPTS = pathlib.Path(__file__).resolve().parent


def git(root, *args):
    subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args], cwd=root, check=True,
                   capture_output=True)


def criterion(cid, command):
    return f'[[criteria]]\nid = "{cid}"\ncommand = {json.dumps(command)}\n'


class Nested(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="gate-nested-"))
        self.addCleanup(shutil.rmtree, self.tmp)
        self.root = self.tmp / "repo"
        self.runner = self.tmp / "runner"
        (self.runner / "perf").mkdir(parents=True)
        shutil.copy(SCRIPTS / "gate.py", self.runner / "gate.py")
        shutil.copy(SCRIPTS / "perf/archive.py", self.runner / "perf/archive.py")
        archive = self.root / "docs/perf/baseline/2026-11-02"
        archive.mkdir(parents=True)
        (archive / "measurements.json").write_bytes(b'{"archived":true}\n')
        digest = hashlib.sha256((archive / "measurements.json").read_bytes()).hexdigest()
        (archive / "SHA256SUMS").write_text(digest + "  measurements.json\n")
        self.pins = {"2026-11-02": hashlib.sha256((archive / "SHA256SUMS").read_bytes()).hexdigest()}
        (self.root / "gates").mkdir()
        (self.root / "sentinel").mkdir()
        (self.root / ".gitignore").write_text("/sentinel/\n/gates/*/*\n!/gates/*/*.md\n/gates/*.json\n")
        # The nested lot N leaves a sentinel when (and only when) its criterion runs.
        (self.root / "gates/N.toml").write_text(criterion("n", "touch sentinel/N"))
        git(self.root, "init", "-q", "-b", "main")
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", "base")
        self.gate = self.load()
        self.env = mock.patch.dict(os.environ, {k: v for k, v in os.environ.items() if not k.startswith("INKWELL_")},
                                   clear=True)
        self.env.start()
        self.addCleanup(self.env.stop)

    def load(self, source=None):
        path = self.runner / "gate.py"
        if source is not None:
            path.write_text(source)
        spec = importlib.util.spec_from_file_location("gate_under_test", path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        module.version = lambda command: "fake " + command
        return module

    def nested_call(self, lot="N"):
        # As gate.sh calls it: isolated, from outside the checkout, the checkout named by INKWELL_GATE_ROOT.
        return f"INKWELL_GATE_ROOT={self.root} {sys.executable} -I {self.runner / 'gate.py'} {lot}"

    def parent(self, *commands):
        (self.root / "gates/P.toml").write_text("".join(criterion(f"p{i}", c) for i, c in enumerate(commands)))

    def run_gate(self, lot="P", gate=None, **kwargs):
        return (gate or self.gate).run(lot, self.root, pins=self.pins, guard=False, **kwargs)

    def manifest(self, lot="P"):
        return json.loads((self.root / f"gates/{lot}.json").read_text())

    def assert_deferred_listed(self, lots, entry=0):
        # Recorded as an obligation, distinct from the criterion's own status, in the entry and at the top.
        m = self.manifest()
        e = m["criteria"][entry]
        self.assertEqual([o["lot"] for o in e.get("deferred_obligations", [])], lots, e)
        for o in e["deferred_obligations"]:
            self.assertRegex(o["reason"], "rule 6")
        self.assertEqual([(o["lot"], o["criterion"]) for o in m.get("deferred_obligations", [])],
                         [(lot, e["id"]) for lot in lots], "top-level deferred_obligations")

    def ledger(self):
        path = self.root / "gates/deferred.jsonl"
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def open_ledger(self):
        return [r for r in self.ledger() if "closed_commit" not in r]

    # (1) The nested lot's criteria do not run; the parent passes on its own commands.
    def test_nested_gate_does_not_run(self):
        self.parent(self.nested_call() + " && echo own > sentinel/own")
        self.assertEqual(self.run_gate(), 0)
        self.assertFalse((self.root / "sentinel/N").exists(), "the nested gate ran its criteria")
        self.assertTrue((self.root / "sentinel/own").exists(), "the criterion's own command did not run")
        self.assertFalse((self.root / "gates/N.json").exists(), "the nested gate wrote a manifest")
        log = (self.root / "gates/P/p0.log").read_text()
        self.assertIn("nested gate N deferred: no nested gates (method v2, user decision 2026-11-12)", log)

    # (2) The deferral is in the criterion's entry and at the top of the manifest, never hidden.
    def test_manifest_lists_deferred_lot(self):
        self.parent(self.nested_call(), "true")
        self.assertEqual(self.run_gate(), 0)
        self.assert_deferred_listed(["N"])
        self.assertNotIn("deferred_obligations", self.manifest()["criteria"][1], "a plain criterion gained the field")
        self.assertFalse((self.root / "gates/P/nested-deferred.txt").exists(), "the hand-off file was kept")

    def test_resumed_entry_keeps_deferral(self):
        self.parent(self.nested_call())
        self.run_gate()
        self.assertEqual(self.run_gate(resume=True), 0)
        self.assertEqual(self.manifest()["resume"]["reused"], ["p0"])
        self.assert_deferred_listed(["N"])

    def test_through_gate_sh(self):
        # gate.sh runs the merge-base runner; the environment reaches it, so the nested call is deferred.
        scripts = self.root / "scripts"
        (scripts / "perf").mkdir(parents=True)
        for name in ("gate.py", "gate.sh", "perf/archive.py"):
            shutil.copy(SCRIPTS / name, scripts / name)
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", "runner")
        self.parent("bash scripts/gate.sh N")
        self.assertEqual(self.run_gate(), 0, (self.root / "gates/P/p0.log").read_text())
        self.assertFalse((self.root / "sentinel/N").exists())
        self.assert_deferred_listed(["N"])

    # (3) A failing own command still fails the parent, deferral recorded all the same.
    def test_failing_own_command_fails_parent(self):
        self.parent(self.nested_call() + "; exit 4")
        self.assertEqual(self.run_gate(), 1)
        m = self.manifest()
        self.assertEqual((m["status"], m["criteria"][0]["exit_code"]), ("fail", 4))
        self.assert_deferred_listed(["N"])

    # (4) A top-level run is unchanged: the lot runs.
    def test_top_level_run_runs_lot(self):
        self.assertEqual(self.run_gate("N"), 0)
        self.assertTrue((self.root / "sentinel/N").exists())
        m = self.manifest("N")
        self.assertEqual((m["status"], m["deferred_obligations"]), ("pass", []))

    # Validation: a deferral that cannot be recorded or names no gate is never dropped silently.
    def test_parent_without_output_dir_refuses(self):
        os.environ["INKWELL_GATE_PARENT"] = "P"
        self.assertNotEqual(self.run_gate("N"), 0)
        self.assertFalse((self.root / "sentinel/N").exists())

    def test_parent_output_dir_must_be_gates_parent_of_root(self):
        elsewhere = self.tmp / "elsewhere"
        elsewhere.mkdir()
        (self.root / "gates/P").mkdir()
        for parent, out in (("P", elsewhere), ("P", self.root / "gates"), ("Q", self.root / "gates/P"),
                            ("../P", self.root / "gates/P")):
            os.environ.update(INKWELL_GATE_PARENT=parent, INKWELL_GATE_PARENT_OUT=str(out))
            self.assertNotEqual(self.run_gate("N"), 0, (parent, out))
            self.assertFalse((elsewhere / "nested-deferred.txt").exists())
        self.assertFalse((self.root / "sentinel/N").exists())

    def test_unknown_nested_lot_fails_parent_criterion(self):
        # `; true` masks the nested exit code: the parent's own validation must still fail the criterion.
        self.parent(self.nested_call("Missing") + "; true", "true")
        self.assertEqual(self.run_gate(), 1)
        e = self.manifest()["criteria"][0]
        self.assertEqual(e["status"], "fail")
        self.assertRegex(e["problem"], "invalid nested deferral.*Missing")
        self.assertEqual(self.open_ledger(), [])

    def test_invalid_handoff_line_fails_parent_criterion(self):
        self.parent('echo "../x" >> "$INKWELL_GATE_PARENT_OUT/nested-deferred.txt"', "true")
        self.assertEqual(self.run_gate(), 1)
        self.assertRegex(self.manifest()["criteria"][0]["problem"], "invalid nested deferral")

    def test_self_deferral_fails(self):
        self.parent(self.nested_call("P") + "; true")
        self.assertEqual(self.run_gate(), 1)
        self.assertRegex(self.manifest()["criteria"][0]["problem"], "invalid nested deferral")

    # Ledger: every deferred obligation is durable until a top-level run of its lot passes.
    def test_ledger_records_and_top_level_pass_closes(self):
        self.parent(self.nested_call())
        self.run_gate()
        self.run_gate()
        rows = self.open_ledger()
        self.assertEqual(len(rows), 1, "a re-run duplicated an open obligation")
        head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=self.root, capture_output=True, text=True).stdout.strip()
        self.assertEqual({k: rows[0][k] for k in ("lot", "from_gate", "criterion", "commit")},
                         {"lot": "N", "from_gate": "P", "criterion": "p0", "commit": head})
        self.assertIn("date", rows[0])
        (self.root / "gates/N.toml").write_text(criterion("n", "touch sentinel/N; exit 1"))
        self.assertEqual(self.run_gate("N"), 1)
        self.assertEqual(len(self.open_ledger()), 1, "a failing run closed the obligation")
        (self.root / "gates/N.toml").write_text(criterion("n", "touch sentinel/N"))
        self.assertEqual(self.run_gate("N"), 0)
        self.assertEqual(self.open_ledger(), [])
        closed = self.ledger()[0]
        self.assertEqual(closed["closed_commit"], head)
        self.assertIn("closed_date", closed)

    def test_ledger_kept_across_resets(self):
        self.parent(self.nested_call())
        self.run_gate()
        self.run_gate(resume=True)
        self.run_gate()
        self.assertEqual(len(self.open_ledger()), 1)

    def test_deferred_cli_reports_overdue(self):
        env = dict(os.environ, INKWELL_GATE_ROOT=str(self.root))
        cli = [sys.executable, "-I", str(self.runner / "gate.py"), "--deferred"]

        def deferred():
            return subprocess.run(cli, env=env, cwd=self.tmp, capture_output=True, text=True)

        self.assertEqual(deferred().returncode, 0, "an empty ledger is clean")
        self.parent(self.nested_call())
        self.run_gate()
        p = deferred()
        self.assertEqual(p.returncode, 0, "a fresh obligation is not overdue")
        self.assertIn("N", p.stdout)
        path = self.root / "gates/deferred.jsonl"
        rows = self.ledger()
        rows[0]["date"] = "2026-11-01T00:00:00+00:00"
        path.write_text("".join(json.dumps(r) + "\n" for r in rows))
        p = deferred()
        self.assertEqual(p.returncode, 1, p.stdout)
        self.assertIn("overdue", p.stdout)
        self.run_gate("N")
        self.assertEqual(deferred().returncode, 0, "a closed obligation still blocks")
        path.write_text("not json\n")
        self.assertNotEqual(deferred().returncode, 0, "a corrupted ledger reads as clean")

    def test_tools_calling_gate_run_clear_parent_env(self):
        # gate-selftest.py asserts on gate.run() results; run inside a gate's criterion, an inherited
        # INKWELL_GATE_PARENT would turn each into a deferral returning 0.
        for path in sorted(SCRIPTS.rglob("*.py")):
            source = path.read_text()
            if path.name.endswith("_test.py") or path.name == "gate.py" or "gate.run(" not in source:
                continue
            for name in ("INKWELL_GATE_PARENT", "INKWELL_GATE_PARENT_OUT"):
                self.assertRegex(source, rf'os\.environ\.pop\("{name}"', f"{path.name} does not clear {name}")

    # (5) A runner that skips the nested gate without recording it is caught by test (2)'s check.
    def test_mutant_skipping_without_record_is_caught(self):
        source = (self.runner / "gate.py").read_text()
        mutated, n = re.subn(r"^def defer_nested\(([^)]*)\):\n", "def defer_nested(\\1):\n    return 0\n", source,
                             count=1, flags=re.M)
        self.assertEqual(n, 1, "anchor `def defer_nested(` missing from gate.py")
        mutant = self.load(mutated)
        self.parent(self.nested_call(), "true")
        self.assertEqual(self.run_gate(gate=mutant), 0)
        self.assertFalse((self.root / "sentinel/N").exists(), "the mutant must still skip the nested gate")
        with self.assertRaises(AssertionError):
            self.assert_deferred_listed(["N"])


if __name__ == "__main__":
    unittest.main()
