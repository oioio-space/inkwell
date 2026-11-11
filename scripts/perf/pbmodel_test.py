"""Tests of pbmodel.py, the statistical specification of the paired performance criteria of block PB
(docs/plans/2026-11-11-pb-paired-perf.md, ADR 0038). Each test names the rule it proves.

The simulations use fixed seeds, so they are deterministic. A bound on a probability is the exact
one-sided 95 % Clopper-Pearson bound: 0 events in 3000 runs bound a rate by 1e-3, and k passes in 3000
runs against a true +10 % slowdown bound the power from below by clopper_pearson(3000 - k, 3000)[0].
Run from the repository root: ./scripts/heavy.sh timeout 900 python3 -m unittest scripts/perf/pbmodel_test.py
"""

import glob
import importlib.util
import itertools
import json
import math
import os
import random
import statistics
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
# Loaded by path so that `python3 -m unittest scripts/perf/pbmodel_test.py` works from the repository root.
_spec = importlib.util.spec_from_file_location("pbmodel", os.path.join(HERE, "pbmodel.py"))
m = importlib.util.module_from_spec(_spec)
sys.modules["pbmodel"] = m
_spec.loader.exec_module(m)

RUNS = 3000  # the fewest runs for which 0 / RUNS bounds a rate by 1e-3 (95 %)
THRESHOLDS = os.path.join(HERE, "..", "..", "docs", "perf", "baseline", "2026-11-02", "thresholds.json")
ENVELOPE = ("fitted", "drift", "bursty", "carry", "ar", "worst")  # every environment but "heavy"
NULL, SLOW, FIXED_COST = m.effect("none", 0), m.effect("mult", 0.10), m.effect("add", 0.10)


def power_lower(passes: int, runs: int = RUNS) -> float:
    return m.clopper_pearson(runs - passes, runs)[0]


class Rule(unittest.TestCase):
    """Plan section 3: the decision rule and its exactness."""

    def test_signed_rank_null_matches_enumeration(self):
        """signed_rank_cdf is the distribution of T+ over the 2^n equally likely sign vectors."""
        for n in range(1, 11):
            counts = [0] * (n * (n + 1) // 2 + 1)
            for signs in itertools.product((0, 1), repeat=n):
                counts[sum(r for r, s in zip(range(1, n + 1), signs) if s)] += 1
            acc, cdf = 0, []
            for c in counts:
                acc += c
                cdf.append(acc / 2**n)
            self.assertEqual(len(cdf), len(m.signed_rank_cdf(n)))
            for a, b in zip(cdf, m.signed_rank_cdf(n)):
                self.assertAlmostEqual(a, b, places=15)

    def test_critical_value_is_the_alpha_quantile(self):
        """critical(n, alpha) is the largest c with P(T+ <= c) <= alpha, near the normal approximation."""
        n = m.PAIRS
        c = m.critical(n, m.ALPHA)
        cdf = m.signed_rank_cdf(n)
        self.assertLessEqual(cdf[c], m.ALPHA)
        self.assertGreater(cdf[c + 1], m.ALPHA)
        mean, sd = n * (n + 1) / 4, math.sqrt(n * (n + 1) * (2 * n + 1) / 24)
        self.assertLess(abs(c - (mean - 3.0902 * sd)), 0.05 * sd)  # 207 at n = 80; the exact tail is lighter
        self.assertEqual(m.critical(8, m.ALPHA), -1)  # 2^-8 > 1e-3: no bound from 8 pairs
        with self.assertRaises(ValueError):
            m.hl_bounds([0.1] * 8)

    def test_bounds_cover_a_symmetric_centre(self):
        """For d symmetric about theta (the randomised order guarantees it), P(upper < theta) <= alpha and
        P(lower > theta) <= alpha, whatever the tails: Laplace, and 20 % of Pareto outliers of random sign.
        Checked at alpha = 0.01 so that 3000 runs can see it."""
        alpha, theta = 0.01, 0.07
        for tail in ("laplace", "outliers"):
            low_miss = high_miss = 0
            for r in range(RUNS):
                rng = random.Random(f"cover/{tail}/{r}")
                d = []
                for _ in range(m.PAIRS):
                    e = rng.expovariate(1 / 0.03) * rng.choice((-1, 1))
                    if tail == "outliers" and rng.random() < 0.2:
                        e += 0.1 * rng.paretovariate(1.1) * rng.choice((-1, 1))
                    d.append(theta + e)
                lower, _, upper = m.hl_bounds(d, alpha)
                high_miss += upper < theta
                low_miss += lower > theta
            for miss in (high_miss, low_miss):
                self.assertLessEqual(m.clopper_pearson(miss, RUNS)[0], alpha, (tail, miss))

    def test_validity_depends_on_precision_only(self):
        """A slowdown never makes a run invalid (the half-width is shift-invariant), and the diagnostic
        `contaminated` is the same whichever side is called A."""
        rng = random.Random("shift")
        pairs = m.session(rng, m.ENVS["fitted"], NULL)
        base = m.paired_verdict(pairs)
        for factor in (0.5, 1.1, 1.5, 3.0):
            v = m.paired_verdict([(a, b * factor) for a, b in pairs])
            self.assertAlmostEqual(v["half_width"], base["half_width"], places=9)
            self.assertAlmostEqual(v["ratio"], base["ratio"] * factor, places=9)
        swapped = m.paired_verdict([(b, a) if i % 3 == 0 else (a, b) for i, (a, b) in enumerate(pairs)])
        self.assertEqual(swapped["contaminated"], base["contaminated"])

    def test_imprecise_run_is_invalid_not_judged(self):
        """A run whose half-width exceeds HALF_WIDTH_MAX is invalid even when its bound would pass, and
        would regress; a precise one is judged on the bound alone."""
        rng = random.Random("wide")
        wide = [(370.0, 370.0 * math.exp(rng.gauss(0, 0.5))) for _ in range(m.PAIRS)]
        v = m.paired_verdict(wide)
        self.assertEqual(v["status"], "invalid")
        self.assertGreater(v["half_width"], math.exp(m.HALF_WIDTH_MAX) - 1)
        tight = [(370.0, 370.0 * math.exp(0.05 + rng.gauss(0, 0.01))) for _ in range(m.PAIRS)]
        self.assertEqual(m.paired_verdict(tight)["status"], "pass")
        self.assertEqual(m.paired_verdict([(a, b * 1.06) for a, b in tight])["status"], "regression")

    def test_budget_is_the_archived_multiplier(self):
        """The 10 % budget is kept: MARGIN is the log of thresholds.json e2e_multiplier (immutable archive)."""
        with open(THRESHOLDS) as f:
            self.assertAlmostEqual(m.MARGIN, math.log(json.load(f)["e2e_multiplier"]), places=12)


class Fit(unittest.TestCase):
    """Plan section 2: the noise model is fitted to the samples on disk, and the stress environments
    dominate the fit."""

    @classmethod
    def setUpClass(cls):
        cls.paths = sorted(glob.glob(os.path.join(m.ARCHIVE, "*.json")))
        cls.fit = m.fit(cls.paths)

    def test_constants_match_disk(self):
        self.assertEqual((self.fit["sets"], self.fit["samples"]), (9, 270))
        self.assertEqual(len(self.fit["sigma"]), len(m.FIT_SIGMA))
        for a, b in zip(self.fit["sigma"], m.FIT_SIGMA):
            self.assertAlmostEqual(a, b, places=4)
        self.assertAlmostEqual(self.fit["eps"], m.FIT_EPS, places=12)
        for a, b in zip(self.fit["excess"], m.FIT_EXCESS):
            self.assertAlmostEqual(a, b, places=3)

    def test_incumbent_fails_on_unchanged_code(self):
        """Anti-vacuity: the archived absolute limit (p95 <= 1.10 x 328 ms) fails the committed HEAD of
        2026-11-11 measured twice, and the day-to-day level moved by more than the budget."""
        with open(THRESHOLDS) as f:
            limit = json.load(f)["e2e"]["PERF-START"]["limit_p95_ms"]
        for name in ("2026-11-11-a-head-start.json", "2026-11-11-a2-head-start.json"):
            with open(os.path.join(m.ARCHIVE, name)) as f:
                s = json.load(f)["samples_ms"]
            self.assertGreater(sorted(s)[math.ceil(0.95 * len(s)) - 1], limit, name)
        lo, hi = self.fit["medians_ms"][0], self.fit["medians_ms"][-1]
        self.assertGreater(hi / lo, 1.10)

    def test_stress_environments_dominate_the_fit(self):
        env = m.ENVS
        self.assertGreaterEqual(env["bursty"].eps, 2 * self.fit["eps"])
        self.assertGreaterEqual(env["heavy"].eps, 2.5 * self.fit["eps"])
        self.assertGreaterEqual(env["heavy"].sigma_scale * min(m.FIT_SIGMA), 1.3 * min(self.fit["sigma"]))
        steepest = max(abs(s) for s in self.fit["slope_per_set"]) / 30  # per sample
        self.assertGreaterEqual(2 * math.pi * env["drift"].wave / m.WAVE_PERIOD, 3 * steepest)
        self.assertGreaterEqual(env["ar"].rho, 3 * max(self.fit["lag1"]))
        rng = random.Random("tail")
        self.assertGreater(sum(m._excess(rng, env["heavy"]) > max(self.fit["excess"]) for _ in range(20000)), 0)


class Guarantees(unittest.TestCase):
    """Plan section 4: false failures <= 1e-3, power >= 0.99 at +10 %, in every environment."""

    def test_no_false_regression(self):
        """No environment, heavy included, turns unchanged code into a regression."""
        for name, env in m.ENVS.items():
            r = m.rates(m.criterion, env, NULL, RUNS, f"rule/{name}/null")
            self.assertEqual(r["regression"], 0, (name, r))
            if name in ENVELOPE:
                self.assertEqual(r["invalid"], 0, (name, r))  # inside the envelope a run always decides

    def test_power_against_ten_percent(self):
        """A true +10 % (every start x 1.10, or a fixed cost of 10 % of the median) passes with
        probability <= 0.01, the retries of invalid runs included."""
        for name, env in m.ENVS.items():
            for label, change in (("mult+10%", SLOW), ("add+10%", FIXED_COST)):
                r = m.rates(m.criterion, env, change, RUNS, f"rule/{name}/{label}")
                self.assertGreaterEqual(power_lower(r["pass"]), 0.99, (name, label, r))

    def test_heavy_environment_is_invalid_not_judged(self):
        """Anti-vacuity of the precision check: 3 x the observed contamination with a Pareto tail makes
        runs invalid; none of them is called a regression (test_no_false_regression)."""
        r = m.rates(m.criterion, m.ENVS["heavy"], NULL, RUNS, "rule/heavy/null")
        self.assertGreater(r["invalid"], RUNS // 10, r)

    def test_changes_well_inside_the_budget_pass(self):
        """The rule is not stricter than the budget needs: +5 % passes in >= 99 % of fitted runs."""
        r = m.rates(m.criterion, m.ENVS["fitted"], m.effect("mult", 0.05), RUNS, "oc/0.05")
        self.assertGreaterEqual(m.clopper_pearson(r["pass"], RUNS)[0], 0.99, r)


class Mutants(unittest.TestCase):
    """Plan section 5: every alternative the plan rejects violates a guarantee."""

    def test_every_mutant_is_rejected(self):
        for name, (runner, env_name, kind) in m.MUTANTS.items():
            env = m.ENVS[env_name]
            if kind == "false":
                r = m.rates(runner, env, NULL, RUNS, f"mutant/{name}")
                bad = r["regression"] + r["invalid"]
                self.assertGreater(m.clopper_pearson(bad, RUNS)[0], 1e-3, (name, r))
            else:
                r = m.rates(runner, env, SLOW, RUNS, f"mutant/{name}")
                self.assertLess(m.clopper_pearson(RUNS - r["pass"], RUNS)[1], 0.99, (name, r))


class Budget(unittest.TestCase):
    """Plan section 6: absolute budgets are certified by an order statistic."""

    def test_quantile_index_matches_binomial(self):
        for n in range(1, 201):
            k = m.quantile_upper_index(n)
            cover = lambda j: sum(math.comb(n, i) * 0.95**i * 0.05 ** (n - i) for i in range(j))
            if k is None:
                self.assertLess(cover(n), 0.95, n)
            else:
                self.assertGreaterEqual(cover(k), 0.95, n)
                self.assertTrue(k == 1 or cover(k - 1) < 0.95, n)
        self.assertIsNone(m.quantile_upper_index(30))  # 30 samples never certify a p95 at 95 %
        self.assertEqual(m.quantile_upper_index(59), 59)
        self.assertEqual(m.quantile_upper_index(100), 99)
        self.assertEqual(m.quantile_upper_index(300), 292)

    def test_bound_covers_the_true_p95(self):
        """On the fitted noise (heavy-tailed), P(bound < true p95) <= 0.05."""
        rng = random.Random("truth")
        big = sorted(math.exp(v) for v in m.environment(rng, m.Env("iid", warmup=0.0), 400_000))
        truth = big[int(0.95 * len(big))]
        misses = 0
        for r in range(RUNS):
            x = [math.exp(v) for v in m.environment(random.Random(f"bound/{r}"), m.Env("iid", warmup=0.0), m.PAIRS)]
            misses += m.budget_verdict(x, budget=truth)["status"] == "pass" and sorted(x)[m.quantile_upper_index(m.PAIRS) - 1] < truth
        self.assertLessEqual(m.clopper_pearson(misses, RUNS)[0], 0.05, misses)

    def test_p95_of_30_accepts_a_noncompliant_product(self):
        """Mutant: the point p95 of 30 starts passes a product whose true p95 is 5 % over budget far more
        often than 5 %; the order-statistic bound does not."""
        rng = random.Random("truth2")
        env = m.Env("iid", warmup=0.0)
        big = sorted(math.exp(v) for v in m.environment(rng, env, 400_000))
        limit = big[int(0.95 * len(big))] / 1.05
        point = bound = 0
        for r in range(RUNS):
            x = [math.exp(v) for v in m.environment(random.Random(f"nc/{r}"), env, m.PAIRS)]
            point += sorted(x[:30])[math.ceil(0.95 * 30) - 1] <= limit
            bound += m.budget_verdict(x, limit)["status"] == "pass"
        self.assertGreater(m.clopper_pearson(point, RUNS)[0], 0.05, point)
        self.assertLessEqual(m.clopper_pearson(bound, RUNS)[0], 0.05, bound)


if __name__ == "__main__":
    unittest.main()
