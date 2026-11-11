#!/usr/bin/env python3
"""Statistical specification of the paired performance criteria of block PB: a change is compared with
its committed reference, both built and measured on this machine, interleaved, and judged by its effect
size. docs/plans/2026-11-11-pb-paired-perf.md, ADR 0038.

Standard library only. Every number of the plan comes from here:

* `paired_verdict` is the regression rule of PERF-START (and of any later paired criterion): pairs of
  starts (reference A, change B), each pair in a fresh random order, warm-up pairs dropped; the
  per-pair log ratio d = ln B - ln A; the Hodges-Lehmann estimate of its centre and the exact one-sided
  Wilcoxon signed-rank bounds (Walsh averages; Hollander, Wolfe & Chicken, Nonparametric Statistical
  Methods, 3rd ed., section 3.3). The change passes iff the upper bound of the median ratio is below
  1.10 (the 10 % budget of docs/perf/baseline/2026-11-02/thresholds.json, `e2e_multiplier`): a
  non-inferiority test (Schuirmann 1987), not a test of "no difference". Two label-blind validity
  checks (contamination of the pair minima, precision of the bound) make a run invalid, and an invalid
  run is retried, never judged.
* `quantile_upper_index` and `budget_verdict` are the absolute-budget rule: the distribution-free
  upper confidence bound of a quantile is an order statistic (David & Nagaraja, Order Statistics,
  section 7.1), so a p95 budget is certified only by enough samples, never by the p95 of 30.
* The rule section moves verbatim to scripts/perf/paired.py; this model then imports it from there,
  so the gate and the specification share one implementation.
* `Env` and `session` are the measurement noise of this host, fitted to the nine startup sample sets
  on disk (`fit`): a lognormal bulk, independent or bursty contamination with the observed excesses or
  a Pareto tail, warm-up, linear drift, level steps, a carry-over between the two slots of a pair and
  AR(1) correlation. `MUTANTS` are the rules the plan rejects, among them the incumbent one.

Commands: `fit` (the noise parameters of sample files), `report` (false failures and power of the
rule and of every mutant), `calibrate` (the sample size sweep), `budget` (order-statistic table and
operating characteristic of the absolute rule).
"""

from __future__ import annotations

import argparse
import functools
import glob
import json
import math
import os
import random
import statistics
import sys
from dataclasses import dataclass, replace

# ── The rule (moves verbatim to scripts/perf/paired.py) ──────────────────────────────────────────

MARGIN = math.log(1.10)  # thresholds.json e2e_multiplier: the 10 % budget, now an effect size
ALPHA = 1e-3  # one-sided level of each bound: P(pass | true ratio 1.10) <= ALPHA per valid run
PAIRS = 80  # judged pairs per run: the smallest N of `calibrate` whose "worst" environment ends invalid in <= 1e-3 of null runs
WARMUP_PAIRS = 2  # dropped: first-sample excess +0.32 and +0.38 (log) in two of the nine sets
ATTEMPTS = 3  # runs per criterion; only an invalid run is retried
HALF_WIDTH_MAX = math.log(1.04)  # half the two-sided interval wider than 4 %: too imprecise to judge (plan 3.4)
RISE = math.log(1.15)  # diagnostic only: a pair whose faster start is 15 % above the median minimum


@functools.cache
def signed_rank_cdf(n: int) -> tuple[float, ...]:
    """Exact null distribution of the Wilcoxon signed-rank statistic T+ for n pairs: cdf[w] = P(T+ <= w).
    Counting subsets of {1..n} by sum (integers, exact), then one division."""
    top = n * (n + 1) // 2
    counts = [1] + [0] * top
    for k in range(1, n + 1):
        for w in range(top, k - 1, -1):
            counts[w] += counts[w - k]
    total, acc, cdf = 2**n, 0, []
    for c in counts:
        acc += c
        cdf.append(acc / total)
    return tuple(cdf)


def critical(n: int, alpha: float) -> int:
    """The largest c with P(T+ <= c) <= alpha; -1 if even T+ = 0 is more likely than alpha."""
    cdf = signed_rank_cdf(n)
    c = -1
    while c + 1 < len(cdf) and cdf[c + 1] <= alpha:
        c += 1
    return c


def hl_bounds(d: list[float], alpha: float = ALPHA) -> tuple[float, float, float]:
    """(lower, Hodges-Lehmann estimate, upper) of the centre of d. With W the M = n(n+1)/2 sorted Walsh
    averages, U = W[M-1-c] and L = W[c] (0-based) satisfy P(U < theta) = P(L > theta) <= alpha when d
    is symmetric about theta (Tukey: T+(theta) counts the Walsh averages above theta)."""
    n = len(d)
    c = critical(n, alpha)
    if c < 0:
        raise ValueError(f"{n} pairs cannot give a bound at alpha {alpha}")
    w = sorted(x + y for i, x in enumerate(d) for y in d[i:])
    m = len(w)
    centre = (w[(m - 1) // 2] + w[m // 2]) / 4
    return w[c] / 2, centre, w[m - 1 - c] / 2


def paired_verdict(pairs: list[tuple[float, float]], alpha: float = ALPHA, margin: float = MARGIN) -> dict:
    """pairs = [(reference ms, change ms)] after the warm-up pairs. Status pass, regression or invalid.

    Validity comes first and depends only on precision: the half-width is unchanged by any shift of d,
    so a slowdown never makes a run invalid. The bound's coverage does not depend on the check; the check
    keeps an imprecise run from being called a regression (plan 3.4). `contaminated`, the share of pairs
    whose faster start is disturbed, is symmetric in the labels and reported for diagnosis only."""
    if not pairs or any(not (a > 0 and b > 0) for a, b in pairs):
        raise ValueError("every start needs a positive duration")
    minima = [min(a, b) for a, b in pairs]
    floor = statistics.median(minima) * math.exp(RISE)
    contaminated = sum(x > floor for x in minima) / len(pairs)
    lower, centre, upper = hl_bounds([math.log(b / a) for a, b in pairs], alpha)
    half = (upper - lower) / 2
    if half > HALF_WIDTH_MAX:
        status, reason = "invalid", "imprecise: the interval is too wide to judge the budget"
    elif upper < margin:
        status, reason = "pass", "upper bound of the median ratio below the budget"
    else:
        status, reason = "regression", "the change is not shown to stay within the budget"
    return dict(status=status, reason=reason, pairs=len(pairs), alpha=alpha, budget=math.exp(margin),
                ratio=math.exp(centre), lower=math.exp(lower), upper=math.exp(upper),
                half_width=math.exp(half) - 1, contaminated=contaminated)


def binomial_pmf(n: int, j: int, p: float) -> float:
    """P(Binomial(n, p) = j), in log space so that no term overflows (n in the thousands)."""
    if p in (0.0, 1.0):
        return float(j == (0 if p == 0.0 else n))
    return math.exp(math.lgamma(n + 1) - math.lgamma(j + 1) - math.lgamma(n - j + 1)
                    + j * math.log(p) + (n - j) * math.log1p(-p))


def quantile_upper_index(n: int, p: float = 0.95, confidence: float = 0.95) -> int | None:
    """Smallest 1-based k with P(X_(k) >= x_p) = P(Binomial(n, p) <= k-1) >= confidence, for any
    continuous distribution; None when no order statistic of n samples reaches the confidence."""
    acc = 0.0
    for j in range(n):
        acc += binomial_pmf(n, j, p)
        if acc >= confidence:
            return j + 1
    return None


def budget_verdict(samples: list[float], budget: float, p: float = 0.95, confidence: float = 0.95) -> dict:
    """An absolute budget on the p-quantile passes iff its distribution-free upper bound is within it."""
    k = quantile_upper_index(len(samples), p, confidence)
    if k is None:
        raise ValueError(f"{len(samples)} samples cannot bound the {p} quantile at {confidence}")
    bound = sorted(samples)[k - 1]
    return dict(status="pass" if bound <= budget else "fail", bound=bound, order=k, samples=len(samples),
                quantile=p, confidence=confidence, budget=budget)


# ── Noise of this host (fitted to the startup samples on disk, 2026-11-11) ────────────────────────

HERE = os.path.dirname(os.path.abspath(__file__))
# The nine archived startup sample sets (SHA256SUMS next to them) that the noise model is fitted to.
ARCHIVE = os.path.join(HERE, "..", "..", "docs", "perf", "2026-11-11-start-drift")
MEDIAN_MS = 370.0  # session medians range 293-377 ms; only the absolute mutant and budgets need a scale


def fit(paths: list[str]) -> dict:
    """Per set: robust log scale (1.4826 MAD), excesses above 4 MAD, Theil-Sen slope over the set, lag-1
    correlation, first-sample excess; plus the spread of the set medians (drift between sessions)."""
    sets = []
    for path in paths:
        with open(path) as f:
            sets.append(json.load(f)["samples_ms"])
    sigma, excess, slopes, lag1, first, medians = [], [], [], [], [], []
    for s in sets:
        x = [math.log(v) for v in s]
        mid = statistics.median(x)
        mad = 1.4826 * statistics.median(abs(v - mid) for v in x)
        e = [v - mid for v in x]
        sigma.append(mad)
        excess += [v for v in e if v > 4 * mad]
        n = len(x)
        slopes.append(statistics.median((x[j] - x[i]) / (j - i) for i in range(n) for j in range(i + 1, n)) * n)
        lag1.append(statistics.correlation(e[:-1], e[1:]))
        first.append(e[0])
        medians.append(math.exp(mid))
    total = sum(len(s) for s in sets)
    return dict(sets=len(sets), samples=total, sigma=sorted(sigma), eps=len(excess) / total,
                excess=sorted(excess), slope_per_set=(min(slopes), max(slopes)), lag1=(min(lag1), max(lag1)),
                first=sorted(first), medians_ms=sorted(medians))


# The fit of the nine sets (`python3 pbmodel.py fit`), frozen here so the guarantees do not move when
# a sample file is added; Fit.test_constants_match_disk refits and compares.
FIT_SIGMA = (0.0177, 0.0183, 0.0212, 0.0216, 0.0240, 0.0250, 0.0256, 0.0330, 0.0387)
FIT_EPS = 19 / 270
FIT_EXCESS = (0.082, 0.093, 0.104, 0.110, 0.121, 0.129, 0.131, 0.138, 0.317, 0.380, 0.548, 0.553,
              0.561, 0.562, 0.577, 0.595, 0.601, 0.998, 1.025)


@dataclass(frozen=True)
class Env:
    """One measurement environment. Offsets are natural logs of a duration ratio."""

    name: str
    sigma_scale: float = 1.0  # times a fitted per-session bulk sigma
    eps: float = FIT_EPS  # stationary fraction of disturbed slots
    tail: str = "empirical"  # excess of a disturbed slot: the observed ones, or Pareto (x_m 0.08, a 1.1)
    burst: float = 1.0  # mean length, in slots, of a disturbance (both starts of a pair hit when > 1)
    trend: float = 0.0  # total linear drift across the run, random sign
    wave: float = 0.0  # amplitude of a slow oscillation of the level (period WAVE_PERIOD slots, random phase)
    step: float = 0.0  # level step of random sign at a random slot
    carry: float = 0.0  # offset of the second start of every pair (cache left warm by the first)
    rho: float = 0.0  # AR(1) correlation of the bulk
    warmup: float = 0.35  # excess of the first slot of the run, in 2 of 9 runs as on disk


WAVE_PERIOD = 60  # slots: with amplitude 0.15 the level moves up to 0.0157 per slot, 5.6 x the steepest set
# on disk (Theil-Sen -0.084 over 30 samples, gates/L0/start.json of 2026-11-11)

ENVS = {e.name: e for e in (
    Env("fitted"),
    Env("drift", trend=0.30, wave=0.15, step=0.25),
    Env("bursty", eps=0.15, burst=4.0),
    Env("heavy", sigma_scale=1.6, eps=0.20, tail="pareto"),
    Env("carry", carry=-0.04),
    Env("ar", rho=0.6),
    Env("worst", sigma_scale=1.5, eps=0.15, tail="pareto", burst=3.0, trend=0.30, wave=0.15, step=0.25, carry=-0.04,
        rho=0.5),
)}


def _excess(rng: random.Random, env: Env) -> float:
    if env.tail == "pareto":
        return min(0.08 * rng.paretovariate(1.1), 2.0)
    return rng.choice(FIT_EXCESS) * rng.uniform(0.8, 1.25)


def environment(rng: random.Random, env: Env, slots: int) -> list[float]:
    """Log offset of every slot of one run, before any product effect."""
    sigma = rng.choice(FIT_SIGMA) * env.sigma_scale
    trend = env.trend * rng.choice((-1, 1))
    step_at, step = rng.randrange(slots), env.step * rng.choice((-1, 1))
    phase = rng.uniform(0, 2 * math.pi)
    enter = env.eps / (env.burst * (1 - env.eps))
    stay = 1 - 1 / env.burst
    out, ar, on = [], 0.0, False
    for t in range(slots):
        ar = env.rho * ar + math.sqrt(1 - env.rho**2) * rng.gauss(0, sigma)
        on = rng.random() < (stay if on else enter)
        level = trend * t / slots + env.wave * math.sin(2 * math.pi * t / WAVE_PERIOD + phase)
        v = ar + level + (step if t >= step_at else 0.0) + (_excess(rng, env) if on else 0.0)
        out.append(v + (env.carry if t % 2 else 0.0))
    if rng.random() < 2 / 9:
        out[0] += env.warmup
    return out


def effect(kind: str, theta: float):
    """The change, as f(slot offset, run median offset): 'mult' scales every start by 1 + theta; 'add'
    adds a fixed cost of theta x the run's median start; 'none' is the null."""
    if kind == "mult":
        return lambda v, base: v + math.log1p(theta)
    if kind == "add":
        return lambda v, base: math.log(math.exp(v) + theta * math.exp(base))
    return lambda v, base: v


def session(rng: random.Random, env: Env, change, pairs: int = PAIRS, design: str = "random") -> list[tuple[float, float]]:
    """Judged (A ms, B ms) pairs of one run. design: 'random' (fresh coin per pair, the rule), 'fixed'
    (always A then B), 'blocked' (every A, then every B: no interleaving)."""
    total = pairs + WARMUP_PAIRS
    s = environment(rng, env, 2 * total)
    base = statistics.median(s)
    out = []
    for i in range(total):
        if design == "blocked":
            ia, ib = i, total + i
        elif design == "fixed" or rng.random() < 0.5:
            ia, ib = 2 * i, 2 * i + 1
        else:
            ia, ib = 2 * i + 1, 2 * i
        out.append((MEDIAN_MS * math.exp(s[ia]), MEDIAN_MS * math.exp(change(s[ib], base))))
    return out[WARMUP_PAIRS:]


def criterion(rng: random.Random, env: Env, change, judge=paired_verdict, design: str = "random",
              pairs: int = PAIRS) -> tuple[str, int]:
    """Up to ATTEMPTS runs; the first valid verdict is final. Returns (status, attempts used)."""
    for attempt in range(1, ATTEMPTS + 1):
        status = judge(session(rng, env, change, pairs, design))["status"]
        if status != "invalid":
            return status, attempt
    return "invalid", ATTEMPTS


# ── Mutants: rules the plan rejects ─────────────────────────────────────────────────────────────


def _p95(x):
    return sorted(x)[math.ceil(0.95 * len(x)) - 1]


def _point(status_pass: bool) -> dict:
    return dict(status="pass" if status_pass else "regression")


def mutant_absolute(rng: random.Random, env: Env, change) -> tuple[str, int]:
    """The incumbent (baseline.py start_verdict): 30 sequential starts of the change, p95 against 1.10 x
    the p95 of 30 archived starts of another day; the day moves the level by U(0, +0.25) as on disk
    (293 ms on 2026-11-02, 356-377 ms on 2026-11-11)."""
    ref = environment(rng, env, 30)
    now = environment(rng, env, 30)
    day = rng.uniform(0.0, 0.25)
    base = statistics.median(now) + day
    ok = _p95([math.exp(change(v + day, base)) for v in now]) <= 1.10 * _p95([math.exp(v) for v in ref])
    return ("pass" if ok else "regression"), 1


def judge_p95_ratio(pairs):
    return _point(_p95([b for _, b in pairs]) <= 1.10 * _p95([a for a, _ in pairs]))


def judge_median_point(pairs):
    return _point(statistics.median(math.log(b / a) for a, b in pairs) < MARGIN)


def judge_alpha_05(pairs):
    return paired_verdict(pairs, alpha=0.05)


T_999_79 = 3.1953  # Student t quantile 0.999, 79 degrees of freedom (PAIRS - 1)


def judge_mean_t(pairs):
    """Kalibera-Jones style mean ratio with a t bound, on the same pairs (outliers enter the mean)."""
    d = [math.log(b / a) for a, b in pairs]
    return _point(statistics.fmean(d) + T_999_79 * statistics.stdev(d) / math.sqrt(len(d)) < MARGIN)


MUTANTS = {
    # name: (criterion runner, the environment that rejects it, what it violates)
    "absolute-p95": (lambda rng, env, ch: mutant_absolute(rng, env, ch), "fitted", "false"),
    "p95-ratio": (lambda rng, env, ch: criterion(rng, env, ch, judge_p95_ratio), "fitted", "false"),
    "median-point": (lambda rng, env, ch: criterion(rng, env, ch, judge_median_point), "fitted", "power"),
    "alpha-0.05": (lambda rng, env, ch: criterion(rng, env, ch, judge_alpha_05), "fitted", "power"),
    "mean-t": (lambda rng, env, ch: criterion(rng, env, ch, judge_mean_t), "heavy", "false"),
    "blocked": (lambda rng, env, ch: criterion(rng, env, ch, design="blocked"), "drift", "false"),
    "fixed-order": (lambda rng, env, ch: criterion(rng, env, ch, design="fixed"), "carry", "power"),
}

# ── Simulation ──────────────────────────────────────────────────────────────────────────────────


def clopper_pearson(k: int, n: int, level: float = 0.95) -> tuple[float, float]:
    """Exact one-sided bounds (lower, upper) of a binomial proportion at `level`, by bisection."""
    def tail_ge(p):  # P(X >= k)
        return sum(binomial_pmf(n, j, p) for j in range(k, n + 1))

    def tail_le(p):  # P(X <= k)
        return sum(binomial_pmf(n, j, p) for j in range(k + 1))

    def solve(f, target, increasing):
        lo, hi = 0.0, 1.0
        for _ in range(60):
            mid = (lo + hi) / 2
            if (f(mid) < target) == increasing:
                lo = mid
            else:
                hi = mid
        return (lo + hi) / 2

    lower = 0.0 if k == 0 else solve(tail_ge, 1 - level, True)
    upper = 1.0 if k == n else solve(tail_le, 1 - level, False)
    return lower, upper


def rates(runner, env: Env, change, runs: int, seed: str) -> dict:
    counts = {"pass": 0, "regression": 0, "invalid": 0}
    attempts = 0
    for r in range(runs):
        status, used = runner(random.Random(f"{seed}:{r}"), env, change)
        counts[status] += 1
        attempts += used
    return dict(runs=runs, mean_attempts=attempts / runs, **counts)


def report(runs: int) -> dict:
    out = {"rule": {}, "mutants": {}}
    for env in ENVS.values():
        row = {}
        for label, change in (("null", effect("none", 0)), ("mult+10%", effect("mult", 0.10)),
                              ("add+10%", effect("add", 0.10))):
            row[label] = rates(criterion, env, change, runs, f"rule/{env.name}/{label}")
        out["rule"][env.name] = row
    for name, (runner, env_name, kind) in MUTANTS.items():
        change = effect("none", 0) if kind == "false" else effect("mult", 0.10)
        out["mutants"][name] = dict(env=env_name, violates=kind,
                                    **rates(runner, ENVS[env_name], change, runs, f"mutant/{name}"))
    return out


def calibrate(runs: int) -> dict:
    out = {}
    for n in (20, 30, 40, 60, 80):
        row = {}
        for env_name in ("fitted", "heavy", "worst"):
            env = ENVS[env_name]
            null = rates(lambda rng, e, ch: criterion(rng, e, ch, pairs=n), env, effect("none", 0), runs, f"cal/{n}/{env_name}/0")
            slow = rates(lambda rng, e, ch: criterion(rng, e, ch, pairs=n), env, effect("mult", 0.10), runs, f"cal/{n}/{env_name}/10")
            row[env_name] = dict(false_regression=null["regression"], final_invalid_null=null["invalid"],
                                 pass_at_10=slow["pass"], mean_attempts_null=round(null["mean_attempts"], 3))
        out[n] = row
    return out


def operating_curve(runs: int, env_name: str = "fitted") -> dict:
    """P(pass) of the rule against a true multiplicative slowdown, for the plan's table."""
    return {f"{t:+.0%}": rates(criterion, ENVS[env_name], effect("mult", t), runs, f"oc/{t}")["pass"] / runs
            for t in (0.0, 0.02, 0.04, 0.05, 0.06, 0.07, 0.08, 0.09, 0.10, 0.12)}


def budget(runs: int) -> dict:
    """Order statistic per sample size, and the absolute rule against 1.0 s and 1.5 s on the change's
    judged starts (PAIRS of them), fitted and heavy environments."""
    table = {n: {c: quantile_upper_index(n, 0.95, c) for c in (0.90, 0.95)} for n in (30, 45, 59, 60, 100, 150, 300)}
    oc = {}
    for env_name in ("fitted", "heavy"):
        for limit in (1000.0, 1500.0):
            fails = 0
            for r in range(runs):
                b = [y for _, y in session(random.Random(f"budget/{env_name}/{r}"), ENVS[env_name], effect("none", 0))]
                fails += budget_verdict(b, limit)["status"] == "fail"
            oc[f"{env_name}<={limit:.0f}ms"] = fails / runs
    return dict(order_statistic=table, false_failure=oc)


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    sub = parser.add_subparsers(dest="cmd", required=True)
    f = sub.add_parser("fit")
    f.add_argument("paths", nargs="*")
    for name in ("report", "calibrate", "budget", "curve"):
        sub.add_parser(name).add_argument("--runs", type=int, default=3000)
    args = parser.parse_args(argv)
    if args.cmd == "fit":
        result = fit(args.paths or sorted(glob.glob(os.path.join(ARCHIVE, "*.json"))))
    else:
        result = {"report": report, "calibrate": calibrate, "budget": budget, "curve": operating_curve}[args.cmd](args.runs)
    json.dump(result, sys.stdout, indent=1, default=str)
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
