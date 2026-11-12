"""End-to-end tests of the paired startup driver (start.cjs, Playwright Chromium) of block PB, plan section 8.
`FakeServer` runs against testdata/fakeserver.py, `RealServer` against bin/inkwell-server (built by the
`perf-start` criterion). Wall-clock measurements: run exclusively, one at a time, never in test:py.
  ./scripts/heavy.sh --exclusive timeout 2400 python3 -m unittest -v -k FakeServer scripts/perf/paired_e2e_test.py
"""

import json
import os
import shutil
import socket
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(os.path.abspath(__file__)).parent
sys.path.insert(0, str(HERE))
import paired  # noqa: E402

ROOT = HERE.parent.parent
FAKE = HERE / "testdata" / "fakeserver.py"
COOL_MS = 20  # the idle gap after a start: shorter than the production 200 ms, same on both sides
BASE_MS = 300  # a fake start's own delay: large against the jitter of the process and the page load


def free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def script(path: Path, body: str) -> str:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("#!/bin/sh\n" + body + "\n")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)
    return str(path)


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def run_pair(self, bin_a, bin_b, **kw):
        kw.setdefault("cool", lambda: None)
        kw.setdefault("cool_ms", COOL_MS)
        kw.setdefault("bg_share_max", None)  # these tests judge timing, not the machine's ambient load
        self.coolings = []
        cool = kw.pop("cool")
        return paired.run_criterion(self.root / "out", bin_a, bin_b, port=kw.pop("port", free_port()),
                                    cool=lambda: (self.coolings.append(1), cool()), **kw)

    def raw(self, result, n=-1):
        return json.loads(Path(result["attempts"][n]["raw"]).read_text())


class FakeServer(Base):
    def fake(self, side, delay=BASE_MS, noise=0, crash=False, record=None):
        flags = f"--delay-ms {delay} --noise-ms {noise}" + (" --crash" if crash else "")
        flags += f" --record {record}" if record else ""
        return script(self.root / "bin" / side / "inkwell-server", f'exec python3 {FAKE} {flags} --self "$0"')

    def test_aa_passes(self):
        result = self.run_pair(self.fake("a"), self.fake("b"))
        self.assertEqual(result["outcome"], "pass", result["attempts"])
        self.assertEqual(len(result["attempts"]), 1)
        self.assertEqual(result["verdict"]["pairs"], 80)
        self.assertAlmostEqual(result["verdict"]["ratio"], 1.0, delta=0.03)
        self.assertEqual(len(self.raw(result)["slots"]), 164)

    def test_injected_slowdown_is_a_regression(self):
        result = self.run_pair(self.fake("a"), self.fake("b", delay=BASE_MS + 300))
        self.assertEqual(result["outcome"], "regression", result["attempts"])
        self.assertGreater(result["verdict"]["lower"], 1.10)
        self.assertEqual(len(self.coolings), 0)  # a valid verdict is never retried

    def test_tiny_change_passes(self):
        result = self.run_pair(self.fake("a"), self.fake("b", delay=BASE_MS + 2))
        self.assertEqual(result["outcome"], "pass", result["attempts"])
        self.assertLess(result["verdict"]["upper"], 1.10)

    def test_noise_is_invalid_and_retried(self):
        result = self.run_pair(self.fake("a", noise=900), self.fake("b", noise=900))
        self.assertEqual(result["outcome"], "invalid", result["attempts"])
        self.assertEqual([a["outcome"] for a in result["attempts"]], ["invalid"] * 3)
        self.assertEqual(len(self.coolings), 2)
        self.assertEqual(len({a["seed"] for a in result["attempts"]}), 3)  # a fresh seed per attempt
        self.assertEqual(len({a["raw"] for a in result["attempts"]}), 3)  # a fresh raw file per attempt

    def test_port_busy_is_invalid(self):
        with socket.socket() as busy:
            busy.bind(("127.0.0.1", 0))
            busy.listen()
            result = self.run_pair(self.fake("a"), self.fake("b"), port=busy.getsockname()[1])
        self.assertEqual(result["outcome"], "invalid", result["attempts"])
        self.assertEqual(len(result["attempts"]), 3)
        self.assertIn("port", result["reasons"][0])
        self.assertEqual(self.raw(result)["fault"]["kind"], "port-busy")

    def test_crash_is_not_ready_failure(self):
        result = self.run_pair(self.fake("a"), self.fake("b", crash=True))
        self.assertEqual(result["outcome"], "fail", result["attempts"])
        self.assertEqual(result["reasons"], ["not ready: b"])
        self.assertEqual(len(result["attempts"]), 1)  # a failure is not retried
        self.assertEqual(len(self.coolings), 0)

    def test_slots_record_order_load_and_pressure(self):
        result = self.run_pair(self.fake("a"), self.fake("b"))
        slots = self.raw(result)["slots"]
        self.assertEqual(len(slots), 164)
        self.assertEqual(sum(s["side"] == "a" for s in slots), 82)
        self.assertEqual([s["order"] for s in slots[:2]], ["first", "second"])
        self.assertTrue(all(s["ready"] and s["ms"] > BASE_MS for s in slots))
        self.assertTrue(all(isinstance(s["load"], float) for s in slots))
        if Path("/proc/pressure/cpu").exists():
            self.assertTrue(all(isinstance(s["cpu_pressure_us"], int) for s in slots))
        coin = {(s["pair"], s["side"]): s["order"] for s in slots}
        self.assertEqual(len({s["pair"] for s in slots}), 82)
        self.assertEqual(sum(coin[(p, "a")] == "first" for p in range(82)) + sum(coin[(p, "b")] == "first" for p in range(82)), 82)
        self.assertTrue(all(isinstance(s["stderr"], str) for s in slots))

    def test_sides_get_equal_length_paths_and_env(self):
        record = self.root / "record.jsonl"
        self.run_pair(self.fake("a", record=record), self.fake("b", record=record))
        starts = [json.loads(line) for line in record.read_text().splitlines()]
        self.assertEqual(len(starts), 164)
        by_side = {"a": [], "b": []}
        for s in starts:
            by_side["a" if "/bin/a/" in s["self"] else "b"].append(s)
        self.assertEqual((len(by_side["a"]), len(by_side["b"])), (82, 82))
        for key in ("self",):
            self.assertEqual({len(s[key]) for s in starts}, {len(by_side["a"][0][key])})
        for var in ("INKWELL_SERVER_PORT", "INKWELL_RENDEZVOUS", "INKWELL_HOME", "INKWELL_DOCS"):
            self.assertEqual({len(s["env"][var]) for s in starts}, {len(starts[0]["env"][var])}, var)
        self.assertNotEqual(by_side["a"][0]["env"]["INKWELL_HOME"], by_side["b"][0]["env"]["INKWELL_HOME"])

    # ── Per-sample interference (plan « Per-sample interference ») ────────────────────────────────

    def drive(self, bin_a, bin_b, *, pairs=4, **overrides):
        """The driver alone on a short plan; `overrides` replace plan fields. Returns the raw record."""
        spec = paired.plan(7, port=free_port(), bins=(bin_a, bin_b), homes=(str(self.root / "ha"), str(self.root / "hb")),
                           docs=(str(self.root / "da"), str(self.root / "db")), pairs=pairs, warmup=0, cool_ms=COOL_MS)
        spec.update(out=str(self.root / "raw.json"), quiet_wait_ms=300, **overrides)
        (self.root / "plan.json").write_text(json.dumps(spec))
        done = subprocess.run(["node", str(paired.DRIVER), "--plan", str(self.root / "plan.json")], capture_output=True, text=True)
        self.assertTrue((self.root / "raw.json").exists(), done.stderr)
        return json.loads((self.root / "raw.json").read_text()), spec

    def ambient(self):
        """The background CPUs of this machine right now: the driver's own measurement of an idle window."""
        cpu = HERE / "cpu.cjs"
        out = subprocess.run(["node", "-e", f"const c=require({json.dumps(str(cpu))});(async()=>{{const r=c.openRun();try{{const o=c.snapshot(r);"
                             "await new Promise(r=>setTimeout(r,1000));console.log(c.background(o,c.snapshot(r,{opening:false}),c.clockTicks()).bg_share)}"
                             "finally{c.closeRun(r)}})()"],
                             capture_output=True, text=True, check=True).stdout
        return float(out)

    def burn(self, cpus, seconds):
        """`cpus` busy processes outside the driver for `seconds`: the daemon's burst."""
        code = f"import time\nt=time.time()+{seconds}\nwhile time.time()<t: pass"
        procs = [subprocess.Popen([sys.executable, "-c", code]) for _ in range(cpus)]
        self.addCleanup(lambda: [p.kill() for p in procs])
        return procs

    def test_every_slot_records_its_background_cpu(self):
        raw, _ = self.drive(self.fake("a"), self.fake("b"), bg_share_max=None)
        self.assertEqual(len(raw["slots"]), 8)
        for s in raw["slots"]:
            self.assertGreater(s["window_ms"], s["ms"] - 1)
            self.assertLess(s["window_ms"], s["ms"] + 200)
            self.assertGreaterEqual(s["bg_share"], 0)
            self.assertAlmostEqual(s["bg_share"], s["bg_cpu_ms"] / s["window_ms"], places=6)
            self.assertTrue(s["valid"])
        self.assertEqual(raw["discarded"], [])

    def test_burst_start_is_discarded_and_remeasured_in_its_slot(self):
        limit = self.ambient() + 2.0
        burner = self.burn(8, 2.5)  # 8 CPUs, far above the bound, for the first slots
        raw, spec = self.drive(self.fake("a"), self.fake("b"), bg_share_max=limit, invalid_max=1.0)
        self.assertIsNone(raw["fault"], raw["fault"])
        self.assertGreaterEqual(len(raw["discarded"]), 1)
        self.assertTrue(all(d["bg_share"] > limit and not d["valid"] for d in raw["discarded"]))
        self.assertTrue(all(s["bg_share"] <= limit and s["valid"] for s in raw["slots"]))
        self.assertEqual([(s["index"], s["pair"], s["side"]) for s in raw["slots"]],
                         [(s["index"], s["pair"], s["side"]) for s in spec["slots"]])  # same slots, same order
        for d in raw["discarded"]:
            kept = raw["slots"][d["index"]]
            self.assertEqual((kept["side"], kept["order"]), (d["side"], d["order"]))
            self.assertGreater(kept["try"], d["try"])
        burner[0].wait()

    def test_a_slow_start_is_not_invalid_by_its_duration(self):
        """The bound reads background CPU, never wall time: starts at 3 x the delay are all kept under a bound no
        burst reaches (the machine has 20 CPUs), and the same bound set to 0 discards even the fast ones
        (test_more_than_a_fifth_invalid_is_the_interference_fault)."""
        raw, _ = self.drive(self.fake("a", delay=BASE_MS * 3), self.fake("b", delay=BASE_MS * 3), bg_share_max=1000.0)
        self.assertEqual((len(raw["slots"]), raw["discarded"]), (8, []))
        self.assertTrue(all(s["ms"] > BASE_MS * 3 for s in raw["slots"]))

    def test_more_than_a_fifth_invalid_is_the_interference_fault(self):
        raw, _ = self.drive(self.fake("a"), self.fake("b"), bg_share_max=0.0)  # every start is above 0 CPUs
        self.assertEqual(raw["fault"]["kind"], "interference")
        self.assertEqual(len(raw["discarded"]), 2)  # 8 slots x 0.2 = 1.6: the second invalid start stops the run
        self.assertEqual(raw["slots"], [])  # an interference stop keeps its start in `discarded` only
        self.assertEqual(paired.classify(paired.observation(raw))["outcome"], "invalid")

    def test_a_detached_helper_is_measured_not_background(self):
        """Every start runs in its own cgroup: a helper that B starts with setsid (outside the process tree) is
        the application's CPU, recorded in app_cpu_ms, and is killed with its start (leftover)."""
        burn = script(self.root / "burn", f"exec {sys.executable} -c 'import time\nt = time.time() + 3\nwhile time.time() < t: pass'")
        b = script(self.root / "bin" / "b" / "inkwell-server",
                   f'setsid {burn} </dev/null >/dev/null 2>&1 &\nexec python3 {FAKE} --delay-ms {BASE_MS} --noise-ms 0 --self "$0"')
        raw, _ = self.drive(self.fake("a"), b, pairs=2, bg_share_max=None)
        self.assertIsNone(raw["fault"], raw["fault"])
        side = {x: [s for s in raw["slots"] if s["side"] == x] for x in "ab"}
        self.assertTrue(all(s["leftover"] >= 1 for s in side["b"]), side["b"])
        self.assertTrue(all(s["leftover"] == 0 for s in side["a"]), side["a"])
        for a, b in zip(side["a"], side["b"]):
            self.assertGreater(b["app_cpu_ms"] - a["app_cpu_ms"], 0.5 * b["window_ms"], (a, b))

    def test_a_slot_invalid_max_tries_times_is_the_interference_fault(self):
        raw, _ = self.drive(self.fake("a"), self.fake("b"), bg_share_max=0.0, invalid_max=1.0, max_tries=3)
        self.assertEqual(raw["fault"]["kind"], "interference")
        self.assertEqual([d["try"] for d in raw["discarded"]], [1, 2, 3])
        self.assertIn("3 tries", raw["fault"]["detail"])


class RealServer(Base):
    """The same real server on both sides is an A/A pass; a 120 ms start delay wrapped around B (about
    +30 %) is a regression. Needs bin/inkwell-server (built by the perf-start criterion)."""

    def binaries(self, delay_ms=0):
        real = ROOT / "bin" / "inkwell-server"
        self.assertTrue(real.is_file(), "bin/inkwell-server is missing: run the perf-start criterion first")
        a = self.root / "bin" / "a" / "inkwell-server"
        a.parent.mkdir(parents=True)
        shutil.copy2(real, a)
        if delay_ms:
            inner = self.root / "bin" / "b" / "real-server"  # equal-length path to the reference's is kept below
            inner.parent.mkdir(parents=True)
            shutil.copy2(real, inner)
            b = script(self.root / "bin" / "b" / "inkwell-server", f'sleep {delay_ms / 1000}\nexec "{inner}"')
        else:
            b = self.root / "bin" / "b" / "inkwell-server"
            b.parent.mkdir(parents=True)
            shutil.copy2(real, b)
        return str(a), str(b)

    def test_real_server_aa_passes(self):
        result = self.run_pair(*self.binaries())
        self.assertEqual(result["outcome"], "pass", result["attempts"])
        self.assertLess(result["verdict"]["upper"], 1.10)

    def test_real_server_delay_is_a_regression(self):
        result = self.run_pair(*self.binaries(delay_ms=120))
        self.assertEqual(result["outcome"], "regression", result["attempts"])
        self.assertGreater(result["verdict"]["ratio"], 1.10)


if __name__ == "__main__":
    unittest.main()
