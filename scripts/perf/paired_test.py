"""Tests of paired.py, the rule of the paired performance criteria as the scripts run it (block PB,
docs/plans/2026-11-11-pb-paired-perf.md sections 3, 6 and 8, ADR 0038). Fast, no process is started.
Run from the repository root: ./scripts/heavy.sh python3 -m unittest -v scripts/perf/paired_test.py
"""

import importlib.util
import inspect
import itertools
import json
import math
import os
import random
import subprocess
import sys
import unittest
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import paired  # noqa: E402

VECTOR = os.path.join(HERE, "testdata", "quantile-index.json")


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def tight(shift, n=paired.PAIRS, seed="tight"):
    rng = random.Random(seed)
    return [(370.0, 370.0 * math.exp(shift + rng.gauss(0, 0.01))) for _ in range(n)]


class Shared(unittest.TestCase):
    """One implementation of the rule: the model imports it."""

    def test_model_shares_the_rule(self):
        model = load("pbmodel")
        for name in ("signed_rank_cdf", "critical", "hl_bounds", "paired_verdict", "binomial_pmf",
                     "quantile_upper_index", "budget_verdict", "MARGIN", "ALPHA", "PAIRS", "WARMUP_PAIRS",
                     "ATTEMPTS", "HALF_WIDTH_MAX"):
            self.assertIs(getattr(model, name), getattr(paired, name), name)

    def test_verdict_matches_model_vectors(self):
        """Values fixed by the plan: c = 983 at N = 80 (section 3.2); A/A passes, x1.2 regresses, a wide
        run is invalid; the half-width does not move with a shift (section 3.3)."""
        self.assertEqual(paired.critical(paired.PAIRS, paired.ALPHA), 983)
        aa = paired.paired_verdict(tight(0.0))
        self.assertEqual(aa["status"], "pass")
        self.assertAlmostEqual(aa["ratio"], 1.0, places=2)
        self.assertEqual(aa["pairs"], 80)
        self.assertEqual(aa["alpha"], 0.001)
        slow = paired.paired_verdict(tight(math.log(1.2)))
        self.assertEqual(slow["status"], "regression")
        self.assertAlmostEqual(slow["half_width"], aa["half_width"], places=9)
        rng = random.Random("wide")
        wide = [(370.0, 370.0 * math.exp(rng.gauss(0, 0.5))) for _ in range(paired.PAIRS)]
        self.assertEqual(paired.paired_verdict(wide)["status"], "invalid")
        self.assertLess(paired.paired_verdict(tight(math.log(1.05)))["upper"], 1.10)


class Machine(unittest.TestCase):
    """Plan section 8: the attempt machine."""

    SIDES = (None, "a", "b")

    def observation(self, busy, not_ready, shift):
        return dict(port_busy=busy, not_ready=not_ready, pairs=tight(shift))

    def test_attempt_machine_exhaustive(self):
        """Every combination of the three observations decides the outcome of the decision table."""
        shifts = {"invalid": None, "pass": 0.0, "regression": math.log(1.3)}
        wide = [(370.0, 370.0 * math.exp(random.Random(i).gauss(0, 0.5))) for i in range(paired.PAIRS)]
        for busy, not_ready, kind in itertools.product((False, True), self.SIDES, shifts):
            obs = dict(port_busy=busy, not_ready=not_ready,
                       pairs=wide if kind == "invalid" else tight(shifts[kind]))
            if busy:
                want = "invalid"
            elif not_ready:
                want = "fail"
            else:
                want = kind
            got = paired.classify(obs)
            self.assertEqual(got["outcome"], want, (busy, not_ready, kind))
            self.assertTrue(got["reasons"], (busy, not_ready, kind))
            if want == "fail":
                self.assertIn(f"not ready: {not_ready}", got["reasons"][0])
            if want in ("pass", "regression", "invalid") and not busy:
                self.assertIn("verdict", got)

    def run_sequence(self, outcomes):
        calls = []
        coolings = []

        def measure(seed, n):
            calls.append((seed, n))
            return outcomes[n - 1]

        result = paired.attempt(measure, classify=lambda o: dict(outcome=o, reasons=[o]),
                                cool=lambda: coolings.append(1), seeds=iter(range(10, 20)).__next__)
        return result, calls, coolings

    def test_valid_verdict_is_never_retried(self):
        for first in ("pass", "regression", "fail"):
            result, calls, coolings = self.run_sequence([first, "pass", "pass"])
            self.assertEqual(len(calls), 1, first)
            self.assertEqual(result["outcome"], first)
            self.assertEqual(coolings, [])
        result, calls, coolings = self.run_sequence(["invalid", "regression", "pass"])
        self.assertEqual((result["outcome"], len(calls), len(coolings)), ("regression", 2, 1))
        result, calls, coolings = self.run_sequence(["invalid", "invalid", "pass"])
        self.assertEqual((result["outcome"], len(calls), len(coolings)), ("pass", 3, 2))

    def test_invalid_retried_at_most_three_times(self):
        result, calls, coolings = self.run_sequence(["invalid"] * 5)
        self.assertEqual(result["outcome"], "invalid")
        self.assertEqual(len(calls), 3)
        self.assertEqual(len(result["attempts"]), 3)
        self.assertEqual(len(coolings), 2)
        self.assertEqual([s for s, _ in calls], [10, 11, 12])  # a fresh seed per attempt
        self.assertTrue(result["reasons"])

    def test_cooling_waits_for_load(self):
        loads = iter([9.0, 8.0, 6.0, 3.0])
        slept = []
        paired.cool(load=lambda: next(loads), sleep=slept.append, cpus=20, idle_s=30)
        self.assertEqual(slept, [paired.COOL_POLL_S] * 3 + [30])  # until load <= 20 / 4, then idle
        slept.clear()
        paired.cool(load=lambda: 99.0, sleep=slept.append, cpus=20, idle_s=30, max_wait_s=3 * paired.COOL_POLL_S)
        self.assertEqual(slept[-1], 30)  # bounded: it measures anyway, the next verdict judges precision
        self.assertLessEqual(len(slept), 5)


class Orders(unittest.TestCase):
    def test_orders_replay_from_seed(self):
        self.assertEqual(paired.orders(123, 82), paired.orders(123, 82))
        self.assertNotEqual(paired.orders(123, 82), paired.orders(124, 82))
        self.assertEqual(len(paired.orders(1, 82)), 82)
        plan = paired.plan(77, port=1, bins=("a", "b"), homes=("ha", "hb"), docs=("da", "db"))
        self.assertEqual(plan, paired.plan(77, port=1, bins=("a", "b"), homes=("ha", "hb"), docs=("da", "db")))

    def test_orders_are_independent_fair_coins(self):
        """Over 200 seeds of 82 coins: the share of A-first is 1/2, and no pair of positions is
        correlated (an alternating or blocked design would fail)."""
        runs = [paired.orders(s, 82) for s in range(200)]
        total = sum(sum(r) for r in runs) / (200 * 82)
        self.assertAlmostEqual(total, 0.5, delta=0.015)
        same = sum(r[i] == r[i + 1] for r in runs for i in range(81)) / (200 * 81)
        self.assertAlmostEqual(same, 0.5, delta=0.02)
        first = sum(r[0] for r in runs) / 200
        self.assertAlmostEqual(first, 0.5, delta=0.12)
        self.assertGreater(len({tuple(r) for r in runs}), 195)

    def test_warmup_pairs_dropped(self):
        plan = paired.plan(5, port=1, bins=("a", "b"), homes=("ha", "hb"), docs=("da", "db"))
        slots = plan["slots"]
        self.assertEqual(len(slots), 2 * (paired.WARMUP_PAIRS + paired.PAIRS))
        self.assertEqual(sum(s["side"] == "a" for s in slots), 82)
        self.assertEqual([s["warmup"] for s in slots[:4]], [True] * 4)
        self.assertFalse(any(s["warmup"] for s in slots[4:]))
        for i in range(0, len(slots), 2):  # the two starts of a pair, one per side, in a coin's order
            self.assertEqual({slots[i]["side"], slots[i + 1]["side"]}, {"a", "b"})
            self.assertEqual(slots[i]["pair"], slots[i + 1]["pair"])
        done = [dict(s, ms=100.0 + s["pair"] + (7 if s["side"] == "b" else 0)) for s in slots]
        judged = paired.judged_pairs(done)
        self.assertEqual(len(judged), 80)
        self.assertEqual(judged[0], (102.0, 109.0))
        # A and B get different homes and binaries of equal length.
        self.assertEqual(len(slots[0]["bin"]), len(slots[1]["bin"]))


class Budgets(unittest.TestCase):
    def test_budget_needs_enough_samples(self):
        with self.assertRaises(ValueError):
            paired.budget_verdict([1.0] * 30, 10.0)  # 30 samples never certify a p95 at 95 %
        self.assertEqual(paired.budget_verdict([1.0] * 59, 10.0)["order"], 59)

    def test_budget_uses_order_statistic(self):
        samples = [float(i) for i in range(1, 101)]
        v = paired.budget_verdict(samples, 99.0)
        self.assertEqual((v["order"], v["bound"], v["status"]), (99, 99.0, "pass"))
        self.assertEqual(paired.budget_verdict(samples, 98.5)["status"], "fail")
        # The point p95 (95.0) would have passed 96: the bound is the second largest, not the p95.
        self.assertEqual(paired.budget_verdict(samples, 96.0)["status"], "fail")
        with open(VECTOR) as f:
            vector = json.load(f)
        self.assertEqual({str(n): paired.quantile_upper_index(n) for n in range(1, 301)},
                         {k: v for k, v in vector["p95"].items()})
        self.assertEqual(vector["p95"]["100"], 99)

    def test_startup_budget_median_and_p95_orders(self):
        self.assertEqual(paired.quantile_upper_index(80, 0.5, 0.95), 48)
        self.assertEqual(paired.quantile_upper_index(80, 0.95, 0.95), 80)
        samples = [300.0 + i for i in range(80)]  # 300 ... 379
        v = paired.startup_budget(samples)
        self.assertEqual(v["status"], "pass")
        self.assertEqual((v["samples"], v["median"]["order"], v["median"]["limit_ms"]), (80, 48, 350))
        self.assertEqual((v["p95"]["order"], v["p95"]["limit_ms"], v["p95"]["bound_ms"]), (80, 400, 379.0))
        self.assertEqual(v["median"]["bound_ms"], 347.0)
        high = paired.startup_budget([351.0] * 33 + [300.0] * 47)  # 48th smallest is 351
        self.assertEqual((high["status"], high["median"]["status"], high["p95"]["status"]),
                         ("fail", "fail", "pass"))
        spike = paired.startup_budget([300.0] * 79 + [401.0])  # one start over 400 ms fails the p95 bound
        self.assertEqual((spike["status"], spike["p95"]["status"]), ("fail", "fail"))


class Interference(unittest.TestCase):
    """Plan « Per-sample interference »: the rule as the scripts apply it."""

    def test_a_start_is_invalid_by_background_cpu_only(self):
        self.assertFalse(paired.invalid_start(paired.BG_SHARE_MAX))  # at the bound: valid
        self.assertTrue(paired.invalid_start(paired.BG_SHARE_MAX + 0.01))
        self.assertEqual(paired.BG_SHARE_MAX, 1.5)
        self.assertEqual(inspect.signature(paired.invalid_start).parameters.keys(), {"bg_share", "limit"})  # no duration

    def test_run_is_interfered_above_a_fifth_of_the_plan(self):
        self.assertEqual(paired.INVALID_MAX, 0.20)
        self.assertFalse(paired.run_interfered(32, 164))  # 32.8 allowed
        self.assertTrue(paired.run_interfered(33, 164))
        self.assertFalse(paired.run_interfered(0, 164))

    def test_interference_is_an_invalid_attempt_never_a_pass(self):
        obs = dict(port_busy=False, interference=True, not_ready=None, pairs=tight(0.0))
        got = paired.classify(obs)
        self.assertEqual(got["outcome"], "invalid")
        self.assertIn("background", got["reasons"][0])
        self.assertNotIn("verdict", got)
        self.assertEqual(paired.classify(dict(obs, port_busy=True))["reasons"], ["environment: the port is busy"])
        doc = dict(fault=dict(kind="interference", side="a", index=3, detail="d"), slots=[])
        self.assertTrue(paired.observation(doc)["interference"])
        self.assertFalse(paired.observation(dict(fault=None, slots=[]))["interference"])

    def test_plan_carries_the_rule_to_the_driver(self):
        spec = paired.plan(1, port=1, bins=("a", "b"), homes=("a", "b"), docs=("a", "b"))
        self.assertEqual((spec["bg_share_max"], spec["invalid_max"], spec["max_tries"], spec["quiet_wait_ms"]),
                         (paired.BG_SHARE_MAX, paired.INVALID_MAX, paired.MAX_TRIES, paired.QUIET_WAIT_MS))
        off = paired.plan(1, port=1, bins=("a", "b"), homes=("a", "b"), docs=("a", "b"), bg_share_max=None)
        self.assertIsNone(off["bg_share_max"])

    def test_report_keeps_every_raw_share(self):
        """The audit names the plan's bound and size (written by the driver), every kept and discarded share, the
        invalid rate over the planned slots, and B's discarded durations."""
        slot = lambda i, side, sh: dict(index=i, side=side, ms=300.0 + i, bg_share=sh, warmup=False)  # noqa: E731
        doc = dict(bg_share_max=1.5, invalid_max=0.2, planned=4, slots=[slot(0, "a", 1.2), slot(1, "b", 1.3)],
                   discarded=[dict(slot(0, "a", 6.0), **{"try": 1}), dict(slot(1, "b", 2.0), **{"try": 1})])
        r = paired.interference_report(doc)
        self.assertEqual((r["limit"], r["invalid_max"], r["planned"]), (1.5, 0.2, 4))
        self.assertEqual(r["kept_bg_share"], [1.2, 1.3])
        self.assertEqual([(d["index"], d["bg_share"], d["tries"]) for d in r["discarded"]], [(0, 6.0, 1), (1, 2.0, 1)])
        self.assertEqual((r["invalid_rate"], r["invalid_by_side"]), (0.5, {"a": 1, "b": 1}))
        self.assertEqual(r["discarded_ms"], {"a": [300.0], "b": [301.0]})
        self.assertEqual(r["imbalance"]["p"], 1.0)

    def test_driver_and_model_share_the_slot_rule(self):
        """One per-start rule: paired.slot_step (which the model runs) and start.cjs's slotStep give the decision
        of every row of the shared vector, and the driver holds no bound of its own (the plan carries it)."""
        with open(os.path.join(HERE, "testdata", "slot-rule.json")) as f:
            vector = json.load(f)
        cases = [(vector["rule"], r) for r in vector["rows"]] + [(vector["off"]["rule"], r) for r in vector["off"]["rows"]]
        for rule, r in cases:
            self.assertEqual(paired.slot_step(r["bg_share"], r["tries"], r["discarded"], r["planned"], rule), r["want"], r)
        self.assertEqual({w for _, r in cases for w in [r["want"]]}, {"keep", "retry", "stop-cap", "stop-tries"})
        driver = os.path.join(HERE, "start.cjs")
        code = (f"const {{slotStep}} = require({json.dumps(driver)}); const cases = {json.dumps(cases)};"
                "console.log(JSON.stringify(cases.map(([rule, r]) => slotStep(r.bg_share, r.tries, r.discarded, r.planned, rule))))")
        got = json.loads(subprocess.run(["node", "-e", code], capture_output=True, text=True, check=True).stdout)
        self.assertEqual(got, [r["want"] for _, r in cases])
        self.assertIs(load("pbmodel").slot_step, paired.slot_step)
        with open(driver) as f:
            self.assertNotRegex(f.read(), r"\b1\.5\b|\b0\.2\b")


class Accounting(unittest.TestCase):
    """Plan section 18.1: the measured set is a cgroup; no cgroup accounting, no verdict."""

    def test_accounting_unavailable_is_an_invalid_attempt(self):
        doc = dict(fault=dict(kind="accounting", side="a", index=0, detail="cgroup v2 not writable"), slots=[], discarded=[])
        obs = paired.observation(doc)
        self.assertTrue(obs["accounting"])
        got = paired.classify(obs)
        self.assertEqual(got["outcome"], "invalid")
        self.assertIn("cgroup", got["reasons"][0])
        self.assertFalse(paired.observation(dict(fault=None, slots=[], discarded=[]))["accounting"])


class Discards(unittest.TestCase):
    """Plan section 18.2: the discarded starts must not depend on the side. A label-blind bound discards both
    sides alike; an application whose slowness lands in CPU outside its cgroup (a service it wakes) is
    discarded on its own side: Fisher's exact test on the two sides' discard counts makes that a failure."""

    def test_fisher_matches_published_values(self):
        # R: fisher.test(matrix(c(1, 11, 9, 3), 2))$p.value = 0.002759456 (two-sided, the classic tea table).
        self.assertAlmostEqual(paired.fisher_two_sided(1, 9, 11, 3), 0.002759456, places=8)
        self.assertEqual(paired.fisher_two_sided(5, 82, 5, 82), 1.0)
        self.assertAlmostEqual(paired.fisher_two_sided(3, 1, 1, 3), 0.4857143, places=6)  # fisher.test(matrix(c(3,1,1,3),2))
        self.assertEqual(paired.fisher_two_sided(9, 1, 11, 3), paired.fisher_two_sided(11, 3, 9, 1))  # symmetric in the sides
        self.assertEqual(paired.IMBALANCE_ALPHA, 1e-3)

    def doc(self, a_bad, b_bad, fault=None, kept=82):
        row = lambda i, side, sh: dict(index=i, pair=i // 2, side=side, warmup=i < 4, ms=330.0, bg_share=sh, order="first")  # noqa: E731
        slots = [row(i, "ab"[i % 2], 1.2) for i in range(2 * kept)]
        discarded = [row(0, "a", 3.0)] * a_bad + [row(1, "b", 3.0)] * b_bad
        return dict(fault=fault, slots=slots, discarded=discarded, planned=164, bg_share_max=1.5, invalid_max=0.2)

    def test_side_dependent_discards_are_a_suspected_regression(self):
        obs = paired.observation(self.doc(4, 26))
        self.assertLessEqual(obs["imbalance"]["p"], paired.IMBALANCE_ALPHA)
        got = paired.classify(obs)
        self.assertEqual(got["outcome"], "regression")
        self.assertIn("suspected regression", got["reasons"][0])
        self.assertEqual((got["imbalance"]["discarded"], got["imbalance"]["kept"]), ({"a": 4, "b": 26}, {"a": 82, "b": 82}))
        # Even when the cap stopped the run: an interference stop with side-dependent discards is a failure, not invalid.
        cap = paired.observation(self.doc(5, 28, fault=dict(kind="interference", side="b", index=90, detail="cap"), kept=45))
        self.assertEqual(paired.classify(cap)["outcome"], "regression")

    def test_balanced_discards_change_nothing(self):
        obs = paired.observation(self.doc(13, 15))
        self.assertGreater(obs["imbalance"]["p"], 0.5)
        self.assertNotEqual(paired.classify(obs)["outcome"], "regression")
        stopped = paired.observation(self.doc(17, 16, fault=dict(kind="interference", side="b", index=90, detail="cap"), kept=45))
        self.assertEqual(paired.classify(stopped)["outcome"], "invalid")

    def test_incomplete_run_is_a_failure(self):
        """No fault, but fewer judged pairs than the plan: never judged on what is left."""
        got = paired.classify(dict(port_busy=False, not_ready=None, pairs=tight(0.0, n=paired.PAIRS - 1)))
        self.assertEqual(got["outcome"], "fail")
        self.assertIn(f"{paired.PAIRS - 1} judged pairs", got["reasons"][0])
        self.assertEqual(paired.classify(dict(port_busy=False, not_ready=None, pairs=tight(0.0)))["outcome"], "pass")


class SelfTest(unittest.TestCase):
    def test_self_test_mode_runs_no_process(self):
        boom = AssertionError("a process was started")
        with mock.patch.object(subprocess, "Popen", side_effect=boom), \
                mock.patch.object(subprocess, "run", side_effect=boom), \
                mock.patch.object(os, "system", side_effect=boom):
            line = paired.self_test()
        self.assertEqual(line, "paired-randomized (A/A pass, x1.2 regression, wide run invalid)")
        env = dict(os.environ, INKWELL_BASELINE_SELF_TEST="1")
        out = subprocess.run([sys.executable, os.path.join(HERE, "baseline.py")], env=env, check=True,
                             capture_output=True, text=True).stdout
        self.assertIn("baseline self-test passed: paired-randomized (A/A pass, x1.2 regression, wide run invalid)", out)


if __name__ == "__main__":
    unittest.main()
