"""Tests of cpu.cjs, the CPU accounting of the paired startup driver (block PB, plan section 18.1). The
arithmetic runs on a fake /proc and a fake cgroup directory, so every case is exact; the cgroup semantics (a
detached helper cannot leave the start's cgroup, a reaped child's CPU stays in it) run against the real cgroup
v2 of this process, which scripts/heavy.sh makes a user-owned systemd scope. Each test names the rule it proves.
Run from the repository root: ./scripts/heavy.sh python3 -m unittest scripts/perf/cpu_test.py"""

import json
import os
import stat
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

CPU = Path(__file__).with_name("cpu.cjs")


def js(expression: str):
    """The JSON value of a JavaScript expression evaluated with cpu.cjs loaded as `c`."""
    done = subprocess.run(["node", "-e", f"const c = require({json.dumps(str(CPU))}); console.log(JSON.stringify({expression}))"],
                          capture_output=True, text=True, check=True)
    return json.loads(done.stdout)


def node(body: str, timeout=60) -> dict:
    """Run an async JavaScript body with cpu.cjs as `c` and child_process.spawn as `spawn`; it prints one JSON value."""
    code = (f"const c = require({json.dumps(str(CPU))}); const {{spawn}} = require('node:child_process');"
            f"const sleep = (ms) => new Promise((r) => setTimeout(r, ms)); (async () => {{ {body} }})()"
            ".catch((e) => { console.error(e); process.exit(1); });")
    done = subprocess.run(["node", "-e", code], capture_output=True, text=True, timeout=timeout)
    if done.returncode:
        raise AssertionError(done.stderr)
    return json.loads(done.stdout)


def writable_cgroup() -> bool:
    """This process's cgroup v2 directory accepts a child cgroup (true under scripts/heavy.sh)."""
    try:
        own = next(line[3:] for line in Path("/proc/self/cgroup").read_text().splitlines() if line.startswith("0::"))
        base = Path("/sys/fs/cgroup" + own)
        return os.access(base / "cgroup.procs", os.W_OK) and os.access(base, os.W_OK)
    except (OSError, StopIteration):
        return False


class Arithmetic(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        self.root = Path(self.dir.name)
        (self.root / "proc").mkdir()
        (self.root / "run").mkdir()

    def state(self, busy, usage_usec):
        # user nice system idle iowait irq softirq steal guest guest_nice: busy = user here.
        (self.root / "proc" / "stat").write_text(f"cpu  {busy} 0 0 1000 7 0 0 0 0 0\ncpu0 1 0 0 1 0 0 0 0 0 0\n")
        (self.root / "run" / "cpu.stat").write_text(f"usage_usec {usage_usec}\nuser_usec 1\nsystem_usec 1\n")

    def snap(self, opening, at):
        run = json.dumps({"dir": str(self.root / "run")})
        return js(f"c.snapshot({run}, {{proc: {json.dumps(str(self.root / 'proc'))}, now: () => {at}, opening: {json.dumps(opening)}}})")

    def test_busy_is_user_nice_system_irq_softirq_steal(self):
        """idle and iowait are not busy; guest is already inside user."""
        text = "cpu  10 20 30 400 50 60 70 80 900 1000\ncpu0 1 1 1 1 1 1 1 1 1 1\n"
        self.assertEqual(js(f"c.busyJiffies({json.dumps(text)})"), 10 + 20 + 30 + 60 + 70 + 80)

    def test_own_cgroup_is_the_v2_line(self):
        self.assertEqual(js("c.ownCgroup('1:name=systemd:/x\\n0::/user.slice/app.scope\\n')"), "/user.slice/app.scope")
        with self.assertRaises(subprocess.CalledProcessError):
            js("c.ownCgroup('1:cpu:/x\\n')")  # cgroup v1 only: no accounting

    def test_background_is_the_machine_minus_the_run_cgroup(self):
        """200 busy jiffies on the machine (2 s at 100 Hz), 300 ms of them in the run's cgroup: 1.7 s are
        background, in a 500 ms window: 3.4 CPUs."""
        self.state(1000, 5_000_000)
        opened = self.snap(True, 1000)
        self.state(1200, 5_300_000)
        got = js(f"c.background({json.dumps(opened)}, {json.dumps(self.snap(False, 1500))}, 100)")
        self.assertEqual((got["window_ms"], got["bg_cpu_ms"]), (500, 1700))
        self.assertAlmostEqual(got["bg_share"], 3.4)

    def test_negative_difference_is_clamped_to_zero(self):
        """/proc/stat is tick sampled, usage_usec is exact: the cgroup may exceed the machine by a tick."""
        self.state(0, 0)
        opened = self.snap(True, 0)
        self.state(2, 30_000)
        got = js(f"c.background({json.dumps(opened)}, {json.dumps(self.snap(False, 400))}, 100)")
        self.assertEqual((got["bg_cpu_ms"], got["bg_share"]), (0, 0))

    def test_no_writable_cgroup_is_an_accounting_error(self):
        """No cgroup v2 line, or a directory that is no cgroup (no cpu.stat): AccountingError, which the driver
        writes as the fault `accounting` (an invalid attempt)."""
        (self.root / "self-v1").write_text("1:cpu:/x\n")
        (self.root / "self-v2").write_text("0::/scope\n")
        for self_file in ("self-v1", "self-v2"):
            got = js(f"(() => {{ try {{ c.openRun({{fsRoot: {json.dumps(str(self.root / 'fs'))}, self: {json.dumps(str(self.root / self_file))}}});"
                     " return 'opened'; } catch (e) { return e instanceof c.AccountingError ? 'accounting' : String(e); } })()")
            self.assertEqual(got, "accounting", self_file)


@unittest.skipUnless(writable_cgroup(), "needs a writable cgroup v2 (run under scripts/heavy.sh)")
class Cgroup(unittest.TestCase):
    """The real kernel: what the process-tree accounting got wrong (review of 2026-11-12, findings 1 and 5)."""

    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        self.root = Path(self.dir.name)

    def script(self, name: str, text: str) -> str:
        path = self.root / name
        path.write_text(text)
        path.chmod(path.stat().st_mode | stat.S_IXUSR)
        return str(path)

    def app(self, body: str) -> str:
        return self.script("app", "#!/bin/sh\n" + body + "\n")

    def burn(self, seconds: float) -> str:
        """A command that keeps one CPU busy for `seconds`."""
        return self.script("burn", "#!/usr/bin/env python3\nimport sys, time\nt = time.time() + float(sys.argv[1])\n"
                           "while time.time() < t:\n    pass\n") + f" {seconds}"

    def start(self, app: str, wait_ms: int) -> dict:
        """Launch `app` in a start cgroup, wait for it to exit and `wait_ms` more, then close the start. Returns
        closeStart's record, the run cgroup's CPU over the window and the pid and argv0 of the launched process."""
        return node(f"""
            const run = c.openRun({{name: 'pbtest-' + process.pid}});
            try {{
              const dir = c.startCgroup(run, 7, 1);
              const opened = c.snapshot(run);
              const [cmd, args] = c.launcher(dir, {json.dumps(app)});
              const child = spawn(cmd, args, {{stdio: 'ignore'}});
              const code = await new Promise((r) => child.once('close', r));
              await sleep({wait_ms});
              const closed = c.snapshot(run, {{opening: false}});
              const done = await c.closeStart(dir);
              console.log(JSON.stringify({{...done, code, pid: child.pid, measured_ms: (closed.measured - opened.measured) / 1000,
                removed: !require('node:fs').existsSync(dir)}}));
            }} finally {{ c.closeRun(run); }}""")

    def test_a_detached_helper_stays_in_the_measured_cgroup(self):
        """A helper that double-forks and calls setsid leaves the process tree, not the cgroup: its CPU is the
        application's (measured, not background), it is a leftover, and cgroup.kill ends it."""
        pidfile = self.root / "helper.pid"
        app = self.app(f"setsid sh -c 'echo $$ > {pidfile}; exec {self.burn(5)}' </dev/null >/dev/null 2>&1 &\nexit 0")
        got = self.start(app, 800)
        self.assertEqual(got["code"], 0)
        self.assertGreaterEqual(got["leftover"], 1)
        self.assertGreater(got["app_cpu_ms"], 500)
        self.assertGreater(got["measured_ms"], 500)  # in the measured set: not background
        self.assertTrue(got["removed"])
        helper = int(pidfile.read_text())
        deadline = time.monotonic() + 2
        while Path(f"/proc/{helper}").exists() and time.monotonic() < deadline:
            time.sleep(0.05)
        self.assertFalse(Path(f"/proc/{helper}").exists(), "the detached helper outlived its start")

    def test_a_reaped_child_counts(self):
        """A child that ran and was reaped before the window closed keeps its CPU in the cgroup."""
        got = self.start(self.app(self.burn(0.5) + "\nexit 0"), 0)
        self.assertEqual(got["leftover"], 0)
        self.assertGreater(got["app_cpu_ms"], 350)
        self.assertGreater(got["measured_ms"], 350)

    def test_launcher_executes_the_binary_in_its_own_process(self):
        """The launcher joins the cgroup and executes the binary: same pid, the binary's own argv0."""
        out = self.root / "who"
        got = self.start(self.app(f'echo "$$ $0" > {out}'), 0)
        pid, argv0 = out.read_text().split()
        self.assertEqual((int(pid), argv0), (got["pid"], str(self.root / "app")))

    def test_an_escaped_descendant_is_brought_back(self):
        """Chromium moves its browser process into its own systemd scope (app-org.chromium.Chromium-<pid>.scope):
        a descendant of the driver that left the run's cgroup is moved back into <run>/driver, and reported."""
        got = node("""
            const run = c.openRun({name: 'pbtest-' + process.pid});
            try {
              const child = spawn('/bin/sh', ['-c', `echo $$ > "${run.base}/cgroup.procs"; exec sleep 5`], {stdio: 'ignore'});
              await sleep(300);
              const where = () => require('node:fs').readFileSync(`/proc/${child.pid}/cgroup`, 'utf8');
              const before = where();
              const moved = c.adopt(run);
              const after = where();
              const again = c.adopt(run);
              child.kill();
              console.log(JSON.stringify({pid: child.pid, before, after, moved, again, run: run.path}));
            } finally { c.closeRun(run); }""")
        self.assertNotIn(got["run"] + "/", got["before"])
        self.assertEqual([m["pid"] for m in got["moved"]], [got["pid"]])
        self.assertIn(got["run"] + "/driver", got["after"])
        self.assertEqual(got["again"], [])

    def test_launch_outside_a_cgroup_fails_with_its_own_status(self):
        cmd, args = js(f"c.launcher({json.dumps(str(self.root / 'none'))}, '/bin/true')")
        self.assertEqual(subprocess.run([cmd, *args]).returncode, js("c.LAUNCH_FAILED"))


if __name__ == "__main__":
    unittest.main()
