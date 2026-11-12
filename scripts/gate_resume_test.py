#!/usr/bin/env python3
"""Block GR: `gate.py --resume` re-uses a finished criterion only on the same candidate.

Rule: docs/plans/2026-11-12-gr-gate-resume.md. Each test runs the working-tree runner (a copy, so a test can change
it) on a temporary git repository; a criterion counts its runs by appending to counts/<id>, an ignored directory.
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
REPO = SCRIPTS.parent


def git(root, *args):
    subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args], cwd=root, check=True,
                   capture_output=True)


def criterion(cid, command, extra=""):
    return f'[[criteria]]\nid = "{cid}"\ncommand = {json.dumps(command)}\n{extra}'


def counted(cid, then="true"):
    return f"echo run >> counts/{cid}; {then}"


class Resume(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="gate-resume-"))
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
        (self.root / "counts").mkdir()
        (self.root / ".gitignore").write_text("/counts/\n/gates/*/*\n!/gates/*/*.md\n")
        (self.root / "src.txt").write_text("v1\n")
        self.write_gate(criterion("a", counted("a")) + criterion("b", counted("b")))
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
        # Host tools are not inputs of these tests; test_miss_on_tool_version_change changes this answer.
        module.version = lambda command: "fake " + command
        return module

    def write_gate(self, text):
        (self.root / "gates/T.toml").write_text(text)

    def run_gate(self, resume=False, gate=None, guard=False):
        gate = gate or self.gate
        if resume:
            return gate.run("T", self.root, pins=self.pins, guard=guard, resume=True)
        return gate.run("T", self.root, pins=self.pins, guard=guard)

    def counts(self):
        return {p.name: len(p.read_text().splitlines()) for p in (self.root / "counts").iterdir()}

    def manifest(self):
        return json.loads((self.root / "gates/T.json").read_text())

    def assert_rerun(self, cid, reason=None):
        resume = self.manifest()["resume"]
        self.assertIn(cid, resume["rerun"], resume)
        self.assertNotIn(cid, resume["reused"])
        if reason:
            self.assertRegex(resume["rerun"][cid], reason)

    # Hit.
    def test_hit_on_identical_candidate(self):
        self.assertEqual(self.run_gate(), 0)
        first = {e["id"]: e for e in self.manifest()["criteria"]}
        self.assertEqual(self.run_gate(resume=True), 0)
        self.assertEqual(self.counts(), {"a": 1, "b": 1}, "a re-used criterion ran again")
        m = self.manifest()
        self.assertEqual(m["resume"], {"reused": ["a", "b"], "rerun": {}})
        for e in m["criteria"]:
            self.assertTrue(e["resumed"])
            self.assertEqual((e["status"], e["exit_code"]), (first[e["id"]]["status"], first[e["id"]]["exit_code"]))
        for path, digest in m["sha256"].items():
            self.assertEqual(hashlib.sha256((self.root / path).read_bytes()).hexdigest(), digest)
        self.assertTrue((self.root / "gates/T/resume/a.json").is_file())

    # Miss, one test per input class.
    def test_miss_on_tracked_change(self):
        self.run_gate()
        (self.root / "src.txt").write_text("v2\n")
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assert_rerun("a", "digest|candidate")
        (self.root / "src.txt").unlink()
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 3, "b": 3}, "a deleted tracked file is a change")

    def test_miss_on_untracked_change(self):
        self.run_gate()
        (self.root / "new.txt").write_text("untracked, not ignored\n")
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assert_rerun("b", "digest|candidate")

    def test_miss_on_mode_change(self):
        self.run_gate()
        (self.root / "src.txt").chmod(0o755)
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})

    def test_miss_on_report_md_change(self):
        # gates/<lot>/*.md are versioned inputs (simplify-report reads SIMPLIFY.md), not the gate's own outputs.
        (self.root / "gates/T").mkdir()
        (self.root / "gates/T/SIMPLIFY.md").write_text("review\n")
        self.run_gate()
        (self.root / "gates/T/SIMPLIFY.md").write_text("")
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})

    def test_non_inputs_do_not_miss(self):
        self.run_gate()
        (self.root / "counts/ignored.txt").write_text("ignored\n")
        manifest = self.root / "gates/T.json"
        manifest.write_text(manifest.read_text() + "\n")
        os.environ["INKWELL_GATE_RUNNER"] = "base 0123"
        self.run_gate(resume=True)
        self.assertEqual({k: v for k, v in self.counts().items() if k in "ab"}, {"a": 1, "b": 1})

    def test_miss_on_command_change(self):
        # The gate file is made ignored, so only the criterion check (not the digest) can see the change.
        (self.root / ".gitignore").write_text((self.root / ".gitignore").read_text() + "/gates/T.toml\n")
        git(self.root, "rm", "-q", "--cached", "gates/T.toml")
        self.run_gate()
        self.write_gate(criterion("a", counted("a", "echo changed")) + criterion("b", counted("b")))
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 1})
        self.assert_rerun("a", "command|criterion")

    def test_miss_on_runner_change(self):
        self.run_gate()
        changed = self.load((self.runner / "gate.py").read_text() + "\n# another runner\n")
        self.run_gate(resume=True, gate=changed)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assert_rerun("a", "runner")

    def test_miss_on_tool_version_change(self):
        self.run_gate()
        self.gate.version = lambda command: "fake " + command + (" 2" if command.startswith("go ") else "")
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assert_rerun("a", "tool|version")

    def test_miss_on_environment_change(self):
        self.run_gate()
        os.environ["INKWELL_NATIVE_PERF_RUN"] = "1"
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assert_rerun("a", "environment|INKWELL")

    def test_miss_on_evidence_changed_or_missing(self):
        self.write_gate(criterion("a", counted("a", 'echo a > "$INKWELL_PERF_OUT/a.out"')) +
                        criterion("b", counted("b")))
        self.run_gate()
        (self.root / "gates/T/a.out").write_text("tampered\n")
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 1})
        self.assert_rerun("a", "evidence")
        (self.root / "gates/T/b.log").unlink()
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assert_rerun("b", "evidence")

    def test_corrupted_record_reruns(self):
        self.write_gate(criterion("a", counted("a")) + criterion("b", counted("b")) + criterion("c", counted("c")))
        self.run_gate()
        records = self.root / "gates/T/resume"
        intact_b = (records / "b.json").read_text()
        # a truncated; b's verdict edited without its checksum; c replaced by b's intact record (wrong id).
        (records / "a.json").write_text((records / "a.json").read_text()[:20])
        edited = re.sub(r'("exit_code":\s*)0', r"\g<1>1", intact_b, count=1)
        self.assertNotEqual(edited, intact_b, "no exit_code field to corrupt")
        (records / "b.json").write_text(edited)
        (records / "c.json").write_text(intact_b)
        self.assertEqual(self.run_gate(resume=True), 0)
        self.assertEqual(self.counts(), {"a": 2, "b": 2, "c": 2})
        for cid in "abc":
            self.assert_rerun(cid)

    def test_recorded_failure_stays_blocking(self):
        self.write_gate(criterion("f", counted("f", "exit 3")) + criterion("b", counted("b")))
        self.assertEqual(self.run_gate(), 1)
        self.assertEqual(self.run_gate(resume=True), 1)
        self.assertEqual(self.counts(), {"f": 1, "b": 1}, "resume retried a valid failure")
        f = self.manifest()["criteria"][0]
        self.assertEqual((f["id"], f["status"], f["exit_code"], f["resumed"]), ("f", "fail", 3, True))
        self.assertEqual(self.manifest()["status"], "fail")

    def test_interrupted_criterion_reruns(self):
        kill = "test -e counts/b.armed || { touch counts/b.armed; kill -9 $PPID; exit 1; }"
        self.write_gate(criterion("a", counted("a")) + criterion("b", kill + "; " + counted("b")) +
                        criterion("c", counted("c")))
        script = ("import importlib.util, json, pathlib, sys\n"
                  "spec = importlib.util.spec_from_file_location('g', sys.argv[1])\n"
                  "g = importlib.util.module_from_spec(spec); spec.loader.exec_module(g)\n"
                  "g.version = lambda command: 'fake ' + command\n"
                  "g.run('T', pathlib.Path(sys.argv[2]), pins=json.loads(sys.argv[3]), guard=False)\n")
        p = subprocess.run([sys.executable, "-I", "-c", script, str(self.runner / "gate.py"), str(self.root),
                            json.dumps(self.pins)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.assertEqual(p.returncode, -9, "the runner was not interrupted")
        self.assertTrue((self.root / "gates/T/resume/a.json").is_file())
        self.assertFalse((self.root / "gates/T/resume/b.json").exists(), "an interrupted criterion left a record")
        self.assertEqual(self.run_gate(resume=True), 0)
        self.assertEqual(self.counts(), {"a": 1, "b": 1, "c": 1, "b.armed": 0})
        self.assertEqual(self.manifest()["resume"]["reused"], ["a"])

    def test_exclusive_criterion_always_reruns(self):
        step = ('echo \'{"criterion":"%s","exclusive":true}\' >> "$INKWELL_PERF_OUT/exclusive.jsonl"')
        self.write_gate(criterion("x", counted("x", step % "x"), "exclusive = true\n") +
                        criterion("y", counted("y", step % "y")) + criterion("z", counted("z")))
        self.assertEqual(self.run_gate(), 0)
        self.assertEqual(self.run_gate(resume=True), 0)
        self.assertEqual(self.counts(), {"x": 2, "y": 2, "z": 1})
        self.assert_rerun("x", "exclusive|measure")
        self.assert_rerun("y", "exclusive|measure")
        steps = {e["id"]: e.get("exclusive_steps") for e in self.manifest()["criteria"]}
        self.assertEqual(len(steps["x"]), 1, "the previous run's measurement steps were kept")
        self.assertEqual(len(steps["y"]), 1)

    def test_resume_removes_stale_evidence(self):
        self.write_gate(criterion("a", counted("a", 'echo a > "$INKWELL_PERF_OUT/a.out"')) +
                        criterion("b", counted("b")))
        self.run_gate()
        (self.root / "gates/T/stray.txt").write_text("not evidence of any record\n")
        (self.root / "gates/T/NOTES.md").write_text("report\n")
        self.run_gate(resume=True)
        m = self.manifest()
        self.assertFalse((self.root / "gates/T/stray.txt").exists())
        self.assertNotIn("gates/T/stray.txt", m["sha256"])
        self.assertIn("gates/T/a.out", m["sha256"])
        self.assertTrue((self.root / "gates/T/NOTES.md").exists())

    def test_validity_checked_when_reached(self):
        # a and b both write shared.out; after a full run it holds b's content, so a's record is stale and a
        # re-runs, which in turn makes b's record stale: b must be judged after a ran, not at start.
        self.write_gate(criterion("a", counted("a", 'echo a > "$INKWELL_PERF_OUT/shared.out"')) +
                        criterion("b", counted("b", 'echo b > "$INKWELL_PERF_OUT/shared.out"')))
        self.run_gate()
        self.run_gate(resume=True)
        self.assertEqual(self.counts(), {"a": 2, "b": 2})
        self.assertEqual((self.root / "gates/T/shared.out").read_text(), "b\n")

    def test_without_flag_behaviour_unchanged(self):
        self.run_gate()
        self.assertTrue((self.root / "gates/T/resume/a.json").is_file(), "a plain run must write records")
        (self.root / "gates/T/stale.txt").write_text("old evidence\n")
        self.assertEqual(self.run_gate(), 0)
        self.assertEqual(self.counts(), {"a": 2, "b": 2}, "a plain run re-used a record")
        m = self.manifest()
        self.assertNotIn("resume", m)
        self.assertFalse(any(e.get("resumed") for e in m["criteria"]))
        self.assertFalse((self.root / "gates/T/stale.txt").exists())

    def test_base_guard_always_runs_on_resume(self):
        calls = []

        def guard(root, stream, ref="main"):
            calls.append(ref)
            return {"status": "pass" if len(calls) < 3 else "fail"}

        self.gate.base_guard = guard
        self.assertEqual(self.run_gate(guard=True), 0)
        self.assertEqual(self.run_gate(resume=True, guard=True), 0)
        self.assertEqual(len(calls), 2)
        self.assertEqual(self.run_gate(resume=True, guard=True), 1)
        self.assertEqual(self.manifest()["status"], "fail")

    def test_cli_accepts_resume_flag(self):
        shutil.rmtree(self.root / "docs/perf/baseline")
        shutil.copytree(REPO / "docs/perf/baseline", self.root / "docs/perf/baseline")
        env = dict(os.environ, INKWELL_GATE_ROOT=str(self.root))
        for args in (["--resume", "T"], ["T", "--resume"]):
            subprocess.run([sys.executable, "-I", str(self.runner / "gate.py"), *args], env=env,
                           cwd=self.tmp, capture_output=True)
            self.assertIn("resume", self.manifest(), args)
        subprocess.run([sys.executable, "-I", str(self.runner / "gate.py"), "T"], env=env, cwd=self.tmp,
                       capture_output=True)
        self.assertNotIn("resume", self.manifest())
        for args in (["--resume"], ["--resume", "T", "U"], ["--resum", "T"], ["--base-guard", "--resume"]):
            p = subprocess.run([sys.executable, "-I", str(self.runner / "gate.py"), *args], env=env,
                               cwd=self.tmp, capture_output=True)
            self.assertEqual(p.returncode, 2, args)

    def test_mutant_skipping_digest_check_is_caught(self):
        # A runner whose candidate digest is constant skips without checking the candidate: the tracked-change
        # scenario must tell it from the real runner.
        source = (self.runner / "gate.py").read_text()
        mutated, n = re.subn(r"^def candidate_digest\(([^)]*)\):\n", 'def candidate_digest(\\1):\n    return "0" * 64\n',
                             source, count=1, flags=re.M)
        self.assertEqual(n, 1, "anchor `def candidate_digest(` missing from gate.py")
        mutant = self.load(mutated)
        self.run_gate(gate=mutant)
        (self.root / "src.txt").write_text("v2\n")
        self.run_gate(resume=True, gate=mutant)
        self.assertEqual(self.counts(), {"a": 1, "b": 1}, "the scenario does not distinguish the mutant")
        real = self.load(source)
        self.run_gate(gate=real)
        (self.root / "src.txt").write_text("v3\n")
        self.run_gate(resume=True, gate=real)
        self.assertEqual(self.counts(), {"a": 3, "b": 3})


if __name__ == "__main__":
    unittest.main()
