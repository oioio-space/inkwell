#!/usr/bin/env python3
"""Rule G-1 of block PC1a: a gate has a deadline (docs/plans/2026-11-15-pc1a-take-over.md, « Tooling rule »).

A gate file carries a top-level max_minutes; scripts/gate.py refuses to start when the `timeout` caps of its criteria
sum past it, stops the run at start + max_minutes (SIGTERM to the running criterion's process group, SIGKILL after
10 s), records `deadline` for that criterion and every one not run, and fails. A gate file without max_minutes is
refused unless gates/legacy-no-deadline.txt lists it; that list is closed. Each test runs the working-tree runner on a
temporary git repository, as scripts/gate_nested_test.py does; time is controlled through gate.clock.
"""
import contextlib
import hashlib
import io
import importlib.util
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import time
import unittest
from unittest import mock

SCRIPTS = pathlib.Path(__file__).resolve().parent

# The gates committed before rule G-1 without a max_minutes (computed from HEAD at c42f6d4). Closed: a gate written
# after the rule carries max_minutes, so a name added to the file fails this set.
LEGACY = {"C5S", "CI1", "CI1b", "CI1c", "DU", "GR", "L0", "L1", "L2", "L3", "LF0", "LF0a", "LF0a2", "LF0a3", "NI0",
          "OI", "P2", "P3", "PB", "PR1", "RC1", "RG", "RL1", "SB1", "SB1b", "SB2a1", "SI1", "SI1b", "SP1", "SP1b",
          "W58", "W58a", "WB"}


def git(root, *args):
    subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args], cwd=root, check=True,
                   capture_output=True)


def criterion(cid, command):
    return f'[[criteria]]\nid = "{cid}"\ncommand = {json.dumps(command)}\n'


def load():
    spec = importlib.util.spec_from_file_location("gate_deadline_under_test", SCRIPTS / "gate.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.version = lambda command: "fake " + command
    return module


def group_alive(pgid):
    try:
        os.killpg(pgid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    # A killed member may still be a zombie until its parent (or the subreaper) reaps it.
    status = subprocess.run(["ps", "-o", "stat=", "-g", str(pgid)], capture_output=True, text=True).stdout.split()
    return any(not s.startswith("Z") for s in status)


def alive(pid):
    status = subprocess.run(["ps", "-o", "stat=", "-p", str(pid)], capture_output=True, text=True).stdout.strip()
    return bool(status) and not status.startswith("Z")


class Deadline(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="gate-deadline-"))
        self.addCleanup(shutil.rmtree, self.tmp)
        self.root = self.tmp / "repo"
        archive = self.root / "docs/perf/baseline/2026-11-02"
        archive.mkdir(parents=True)
        (archive / "measurements.json").write_bytes(b'{"archived":true}\n')
        digest = hashlib.sha256((archive / "measurements.json").read_bytes()).hexdigest()
        (archive / "SHA256SUMS").write_text(digest + "  measurements.json\n")
        self.pins = {"2026-11-02": hashlib.sha256((archive / "SHA256SUMS").read_bytes()).hexdigest()}
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

    def write(self, head, *criteria):
        (self.root / "gates/T.toml").write_text(head + "".join(criterion(cid, c) for cid, c in criteria))

    def run_gate(self):
        # The runner's progress lines would split unittest's « name ... ok » line, which the gate criterion reads.
        with contextlib.redirect_stdout(io.StringIO()):
            return self.gate.run("T", self.root, pins=self.pins, guard=False)

    def manifest(self):
        return json.loads((self.root / "gates/T.json").read_text())

    def test_caps_over_max_minutes_refused_before_any_run(self):
        self.assertEqual(self.gate.criterion_caps("timeout -k 10 300 a && timeout 60 b; timeout -k 5 2m c"), 480)
        self.write("max_minutes = 1\n", ("a", "timeout -k 10 50 touch sentinel/a"), ("b", "timeout 20 touch sentinel/b"))
        with self.assertRaisesRegex(ValueError, r"70 s.*max_minutes = 1 \(60 s\)"):
            self.run_gate()
        self.assertEqual(list((self.root / "sentinel").iterdir()), [], "no criterion may run")
        self.assertFalse((self.root / "gates/T").exists(), "refused before the evidence is touched")
        # At the limit, it runs.
        self.write("max_minutes = 1\n", ("a", "timeout -k 10 40 touch sentinel/a"), ("b", "timeout 20 touch sentinel/b"))
        self.assertEqual(self.run_gate(), 0)
        # The frozen gates of block PC1a hold their own caps.
        for lot in ("PC1a", "PC1b"):
            config = self.gate.tomllib.loads((SCRIPTS.parent / "gates" / (lot + ".toml")).read_text())
            self.assertIsNone(self.gate.deadline_problem(lot, config, SCRIPTS.parent), lot)

    def test_run_stopped_at_the_deadline(self):
        self.assertEqual(self.gate.KILL_GRACE_SECONDS, 10)
        self.gate.KILL_GRACE_SECONDS = 1
        # The gate starts, then 58 s of its 60 s pass at once: the sleeping criterion has 2 s left.
        real, started = time.monotonic, []
        self.gate.clock = lambda: real() + (58 if started or started.append(True) else 0)
        # The criterion ignores SIGTERM (so SIGKILL is needed), leaves a child in its group and, as every capped
        # criterion does, a `timeout` in a group of its own (coreutils timeout calls setpgid unless --foreground).
        inner = "timeout 50 sh -c 'echo $$ > sentinel/inner; sleep 50'"
        self.write("max_minutes = 1\n", ("first", "true"),
                   ("sleeper", f"trap '' TERM; echo $$ > sentinel/pgid; sleep 50 & {inner} & wait; wait"),
                   ("after", "touch sentinel/after"))
        begin = real()
        self.assertEqual(self.run_gate(), 1)
        self.assertLess(real() - begin, 20, "the deadline stops the run, not the criterion's own end")
        pgid = int((self.root / "sentinel/pgid").read_text())
        inner_pid = int((self.root / "sentinel/inner").read_text())
        for _ in range(50):
            if not group_alive(pgid) and not alive(inner_pid):
                break
            time.sleep(0.1)
        self.assertFalse(group_alive(pgid), "the criterion's process group is gone")
        self.assertFalse(alive(inner_pid), "so is the group its `timeout` made")
        self.assertFalse((self.root / "sentinel/after").exists(), "no criterion runs past the deadline")
        statuses = {e["id"]: e["status"] for e in self.manifest()["criteria"]}
        self.assertEqual(statuses, {"first": "pass", "sleeper": "deadline", "after": "deadline"})
        self.assertEqual(self.manifest()["status"], "fail")

    def test_interrupted_runner_stops_the_criterion(self):
        # In its own session the criterion does not get the terminal's Ctrl-C: the runner must stop its tree itself,
        # or a heavy command would outlive an interrupted gate.
        import signal
        self.gate.KILL_GRACE_SECONDS = 1
        sentinel = self.root / "sentinel"
        command = "echo $$ > sentinel/pgid; timeout 50 sh -c 'echo $$ > sentinel/inner; sleep 50'"

        def interrupt(*_):
            raise KeyboardInterrupt
        previous = signal.signal(signal.SIGALRM, interrupt)
        signal.alarm(1)
        try:
            with self.assertRaises(KeyboardInterrupt):
                self.gate.run_until(command, self.gate.clock() + 600, cwd=self.root)
        finally:
            signal.alarm(0)
            signal.signal(signal.SIGALRM, previous)
        pgid = int((sentinel / "pgid").read_text())
        inner_pid = int((sentinel / "inner").read_text())
        self.assertFalse(group_alive(pgid), "the criterion's group is gone after an interruption")
        self.assertFalse(alive(inner_pid), "so is the group its `timeout` made")

    def test_gate_without_max_minutes_refused_unless_legacy(self):
        self.write("", ("a", "touch sentinel/a"))
        with self.assertRaisesRegex(ValueError, r"max_minutes.*legacy-no-deadline\.txt"):
            self.run_gate()
        self.assertFalse((self.root / "sentinel/a").exists())
        (self.root / "gates/legacy-no-deadline.txt").write_text("# gates before rule G-1\nT\n")
        self.assertEqual(self.run_gate(), 0)
        self.assertTrue((self.root / "sentinel/a").exists())

    def test_legacy_list_is_closed(self):
        self.assertEqual(self.gate.legacy_gates(SCRIPTS.parent), LEGACY)
        for lot in LEGACY:
            config = self.gate.tomllib.loads((SCRIPTS.parent / "gates" / (lot + ".toml")).read_text())
            self.assertNotIn("max_minutes", config, lot)


if __name__ == "__main__":
    unittest.main()
