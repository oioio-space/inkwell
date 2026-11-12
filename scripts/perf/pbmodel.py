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
* The rule lives in scripts/perf/paired.py; this model imports it from there, so the gate and the
  specification share one implementation.
* `Bursts`, `bg_session` and `BG_MUTANTS` are the per-sample interference rule of the plan's section 18: a
  background process that bursts every ~7.7 s, a start invalid above BG_SHARE_MAX CPUs of background and
  re-measured (`slot_step`, the driver's rule), a run invalid above 20 % invalid starts, side-dependent
  discards a failure (`discard_imbalance`), and the cgroup accounting of the measured set (a detached helper's
  CPU is the application's).
* `Env` and `session` are the measurement noise of this host, fitted to the nine startup sample sets
  on disk (`fit`): a lognormal bulk, independent or bursty contamination with the observed excesses or
  a Pareto tail, warm-up, linear drift, level steps, a carry-over between the two slots of a pair and
  AR(1) correlation. `MUTANTS` are the rules the plan rejects, among them the incumbent one.

Commands: `fit` (the noise parameters of sample files), `bgfit` and `threshold` (the burst process and the
bound on background CPU: plan section 18), `report` (false failures and power of the
rule and of every mutant), `calibrate` (the sample size sweep), `budget` (order-statistic table and
operating characteristic of the absolute rule).
"""

from __future__ import annotations

import argparse
import glob
import json
import math
import os
import random
import statistics
import sys
from dataclasses import dataclass, replace

# ── The rule: one implementation, shared with the gate (scripts/perf/paired.py) ───────────────────

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from paired import (ALPHA, ATTEMPTS, BG_SHARE_MAX, HALF_WIDTH_MAX, IMBALANCE_ALPHA, INVALID_MAX, MARGIN,  # noqa: E402,F401
                    MAX_TRIES, PAIRS, STARTUP_P95_MS, WARMUP_PAIRS, binomial_pmf, budget_verdict, critical,
                    fisher_two_sided, hl_bounds, invalid_start, paired_verdict, quantile_upper_index,
                    run_interfered, signed_rank_cdf, slot_step, startup_budget)

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

# ── Background bursts and per-sample interference (plan « Per-sample interference ») ───────────────
#
# The host runs a daemon that bursts every ~7.7 s: for about 0.14 s all 20 CPUs are busy, with a lower
# shoulder of 1-4 CPUs around the peak. A start that meets it gets slower in proportion to the CPU the
# burst took during its window. The driver measures that CPU (`bg_share`, in CPUs), and the rule calls a
# start invalid, and re-measures it in its slot, when it exceeds BG_SHARE_MAX. The constants below are
# fitted to the four archived A/A sets of docs/perf/2026-11-12-start-bg (`python3 pbmodel.py bgfit`);
# BgFit.test_constants_match_disk refits them.

BG_ARCHIVE = os.path.join(HERE, "..", "..", "docs", "perf", "2026-11-12-start-bg")
SLOT_S = 0.63  # one slot: 164 starts in 104 s, cooling and tear-down included
BG_BASE, BG_SD = 1.27, 0.096  # CPUs used by everything else in a quiet window: median and robust sd
BG_HIT = BG_SHARE_MAX  # a share above this is a burst hit (535 of 640 starts are at or below it, and none above 370 ms)
BG_SLOPE_MS = 41.4  # wall-clock cost of one CPU of background over the window: median of (ms - 329) / (share - BASE) over the 105 hits
# (quartiles about 33 and 62: the stressed environment uses 1.5 x). Residual sd 23 ms for shares 1.5-3 (65 hits), 39 ms for 3-6, 120 ms above (the peaks).
BG_PEAK_ABOVE = 6.0  # share of a start that holds the whole peak: 31 starts at 6.3-10.2 CPUs (mode 9.4)


def fit_bg(paths: list[str]) -> dict:
    """The statistics the burst model is fitted to, from the archived A/A sets (judged starts only)."""
    rows = []
    for path in paths:
        with open(path) as f:
            rows += [(x["bg_share"], x["ms"]) for x in json.load(f)["slots"] if not x["warmup"]]
    quiet = [(sh, ms) for sh, ms in rows if sh <= BG_HIT]
    base = statistics.median(sh for sh, _ in quiet)
    level = statistics.median(ms for _, ms in quiet)
    hits = [(sh, ms) for sh, ms in rows if sh > BG_HIT]
    shares = sorted(sh for sh, _ in rows)
    return dict(starts=len(rows), base=base, sd=1.4826 * statistics.median(abs(sh - base) for sh, _ in quiet),
                level_ms=level, quiet_max_ms=max(ms for _, ms in quiet),
                hit_rate=len(hits) / len(rows), peak_rate=sum(sh > BG_PEAK_ABOVE for sh, _ in rows) / len(rows),
                shoulder_rate=sum(BG_HIT < sh <= 2.5 for sh, _ in rows) / len(rows),
                slope_ms=statistics.median((ms - level) / (sh - base) for sh, ms in hits),
                p99=shares[int(0.99 * len(shares))])


def simulated_bg(runs: int = 60, bursts: "Bursts | None" = None) -> dict:
    """The same statistics of the model, without invalidation, on `runs` runs."""
    rng, trace = random.Random("bgfit"), []
    for _ in range(runs):
        bg_session(rng, BG_ENV, bursts or Bursts(), limit=math.inf, trace=trace)
    shares = sorted(sh for sh, _ in trace)
    n = len(shares)
    return dict(starts=n, hit_rate=sum(x > BG_HIT for x in shares) / n, peak_rate=sum(x > BG_PEAK_ABOVE for x in shares) / n,
                shoulder_rate=sum(BG_HIT < x <= 2.5 for x in shares) / n, p99=shares[int(0.99 * n)])


@dataclass(frozen=True)
class Bursts:
    """The background process: a cycle of `period_s` (12.2 slots between peaks in the sets), firing with
    probability `fire`; a fired cycle is a peak (`core_cpus` for `core_s`, 2.7 CPU-s: the 9.4 CPUs of a
    start that holds it) with a shoulder of `shoulder_cpus` for `shoulder_s` on each side."""

    period_s: float = 7.7
    fire: float = 1.0
    core_s: float = 0.12
    core_cpus: float = 20.0
    shoulder_s: float = 0.4
    shoulder_cpus: float = 1.25
    jitter_s: float = 0.15


def overlap(a0: float, a1: float, b0: float, b1: float) -> float:
    return max(0.0, min(a1, b1) - max(a0, b0))


def burst_cycles(rng: random.Random, b: Bursts, span_s: float) -> list[tuple[float, bool]]:
    """(peak start time, fired) of every cycle across span_s, with a random phase."""
    t = rng.uniform(0, b.period_s)
    out = []
    while t < span_s + b.period_s:
        out.append((t + rng.gauss(0, b.jitter_s), rng.random() < b.fire))
        t += b.period_s
    return out


def burst_cpu(cycles, b: Bursts, t0: float, t1: float) -> float:
    """CPU-seconds the bursts took in [t0, t1]."""
    total = 0.0
    for c, fired in cycles:
        if fired and c - b.shoulder_s < t1 and t0 < c + b.core_s + b.shoulder_s:
            total += b.core_cpus * overlap(t0, t1, c, c + b.core_s)
            total += b.shoulder_cpus * (overlap(t0, t1, c - b.shoulder_s, c) + overlap(t0, t1, c + b.core_s, c + b.core_s + b.shoulder_s))
    return total


def burst_end(cycles, b: Bursts, t: float) -> float:
    """The time at which no burst covers t any more (t itself when none does)."""
    for c, fired in cycles:
        if fired and c - b.shoulder_s <= t < c + b.core_s + b.shoulder_s:
            return c + b.core_s + b.shoulder_s
    return t


def hit_slowdown_ms(rng: random.Random, extra_share: float, stress: float = 1.0) -> float:
    """Wall-clock cost of a start that shared its window with `extra_share` CPUs of background burst: the
    fitted slope, scaled by `stress`, with the fitted residual (ramped in over the first 0.4 CPUs)."""
    if extra_share <= 0:
        return 0.0
    sd = 23.0 if extra_share < 1.7 else 39.0 if extra_share < 4.7 else 120.0
    return max(0.0, stress * BG_SLOPE_MS * extra_share + rng.gauss(0, sd * min(1.0, extra_share / 0.4)))


HELPER_CPUS = 8.0  # background share added to a slow start whose CPU the accounting does not see


def driver_step(share: float, ms: float, tries: int, discarded: int, planned: int, rule: dict) -> str:
    """The driver's per-start decision: paired.slot_step, which reads the share only (start.cjs runs its twin)."""
    return slot_step(share, tries, discarded, planned, rule)


def bg_session(rng: random.Random, env: Env, bursts: Bursts | None, *, limit: float | None = BG_SHARE_MAX,
               cap: float = INVALID_MAX, step=driver_step, side_test: bool = True, accounting: str = "cgroup",
               slow: float = 0.0, slow_factor: float = 1.35, where: str = "app", level: float = 1.0,
               median_ms: float = 330.0, stress: float = 1.0, pairs: int = PAIRS, trace: list | None = None) -> dict:
    """One run of the per-sample rule as start.cjs applies it: warm-up + judged pairs in random order, each slot
    measured until `step` keeps its start (paired.slot_step: at most MAX_TRIES, with a wait for quiet in between,
    the run interfered when more than `cap` of the plan's slots were invalid), then the side test of the discards.
    `slow` is the share of B starts that are `slow_factor` slower; `where` is where that slowness spends its CPU:
    "app" (the application's own process), "helper" (a detached helper: inside the start's cgroup under the
    "cgroup" accounting, outside the measured set under the old process-tree accounting, "tree"), or "outside"
    (a service outside the cgroup that the application wakes, background under any accounting). CPU the
    accounting does not see adds HELPER_CPUS to the start's background share. `level` multiplies every B start
    (a uniformly slow application). `trace` collects (share, ms) of every measurement. Returns the judged B
    starts, the discards and kept starts per side, the side test and whether the run is interfered."""
    rule = dict(bg_share_max=limit, invalid_max=cap, max_tries=MAX_TRIES)
    unseen = where == "outside" or (where == "helper" and accounting == "tree")
    total = pairs + WARMUP_PAIRS
    planned = 2 * total
    off = environment(rng, env, planned * MAX_TRIES)
    cycles = burst_cycles(rng, bursts, planned * (SLOT_S + 1) * MAX_TRIES) if bursts else []
    t, k = rng.uniform(0, 5.0), 0
    kept_b, interfered = [], False
    dropped, kept = {"a": 0, "b": 0}, {"a": 0, "b": 0}
    for pair in range(total):
        order = ("a", "b") if rng.random() < 0.5 else ("b", "a")
        for side in order:
            for tries in range(1, MAX_TRIES + 1):
                v = off[k]
                k += 1
                ms = median_ms * math.exp(v)
                is_slow = side == "b" and rng.random() < slow
                if side == "b":
                    ms *= level * (slow_factor if is_slow else 1.0)
                w = ms / 1000
                extra = burst_cpu(cycles, bursts, t, t + w) / w if bursts else 0.0
                ms += hit_slowdown_ms(rng, extra, stress)
                share = max(0.8, rng.gauss(BG_BASE, BG_SD)) + extra + (HELPER_CPUS if unseen and is_slow else 0.0)
                decision = step(share, ms, tries, dropped["a"] + dropped["b"], planned, rule)
                if trace is not None:
                    trace.append((share, ms))
                t += SLOT_S
                if decision == "keep":
                    break
                dropped[side] += 1
                if decision != "retry":
                    interfered = True
                    break
                t = burst_end(cycles, bursts, t) + 0.4 if bursts else t + 0.4
            if interfered:
                break
            kept[side] += 1
            if side == "b" and pair >= WARMUP_PAIRS:
                kept_b.append(ms)
        if interfered:
            break
    p = fisher_two_sided(dropped["a"], kept["a"], dropped["b"], kept["b"])
    return dict(b=kept_b, invalid=dropped["a"] + dropped["b"], discarded=dropped, kept=kept, p=p,
                imbalanced=side_test and p <= IMBALANCE_ALPHA, interfered=interfered, planned=planned)


def bg_outcome(session: dict, check=None) -> str:
    """'fail' when the discards depend on the side (a suspected regression, even if the cap stopped the run),
    'invalid' for an interfered run, else the absolute startup budget: pass or fail."""
    if session["imbalanced"]:
        return "fail"
    if session["interfered"]:
        return "invalid"
    return "pass" if (check or startup_budget)(session["b"])["status"] == "pass" else "fail"


def bg_criterion(rng: random.Random, env: Env, bursts, *, check=None, **kw) -> tuple[str, int]:
    """Up to ATTEMPTS runs; an invalid run is retried, a valid outcome is final."""
    for attempt in range(1, ATTEMPTS + 1):
        status = bg_outcome(bg_session(rng, env, bursts, **kw), check)
        if status != "invalid":
            return status, attempt
    return "invalid", ATTEMPTS


BG_ENV = Env("bg", eps=0.0, warmup=0.0)  # the bulk of the fitted environment; its contamination is the bursts


def p95_check(b: list[float]) -> dict:
    """The absolute p95 budget alone: the 80th of 80 starts within STARTUP_P95_MS."""
    return budget_verdict(b, STARTUP_P95_MS, p=0.95)


def bg_rates(runs: int, seed: str, *, bursts: "Bursts | None" = None, check=None, env: Env = BG_ENV,
             limit: float | None = BG_SHARE_MAX, **kw) -> dict:
    """pass / fail / invalid of `runs` criteria (up to ATTEMPTS runs each) under the per-sample rule, at the
    gate's bound unless `limit` says otherwise."""
    counts = {"pass": 0, "fail": 0, "invalid": 0}
    for r in range(runs):
        counts[bg_criterion(random.Random(f"{seed}:{r}"), env, bursts or Bursts(), check=check, limit=limit, **kw)[0]] += 1
    return dict(runs=runs, **counts)


# What a truly slow application looks like: every start 20 % slower; one B start in ten 35 % slower in its own
# process (not correlated with any burst); 4, 8 or 12 % of the B starts 35 % slower in a detached helper (setsid,
# double fork), which the cgroup keeps in the measured set; three in ten slower in a service outside the cgroup
# that the application wakes, which the driver can only see as background CPU.
BG_SCENARIOS = {
    "slow-all+20%": dict(level=1.20),
    "slow-10%": dict(slow=0.10),
    **{f"{where}-{pct}%": dict(slow=pct / 100, where=where) for where in ("app", "helper") for pct in (4, 8, 12)},
    "outside-30%": dict(slow=0.30, where="outside"),
}
BG_MUTANTS = {
    # name: (rule override, the scenario that rejects it or "false" for the null, the outcome that shows the
    # violation, what it violates). The rule gives that outcome clearly less often than the mutant.
    "no-invalidation": (dict(limit=None), "false", "fail", "false failures"),
    "wall-time": (dict(step=lambda share, ms, *a: slot_step(math.inf if ms > STARTUP_P95_MS else 0.0, *a)), "slow-10%",
                  "pass", "a slow application passes"),
    "tree-accounting": (dict(accounting="tree"), "helper-12%", "pass", "a detached helper hides the slowness"),
    "no-cap": (dict(cap=math.inf), "outside-30%", "pass", "a slowness outside the cgroup passes"),
    "no-side-test": (dict(side_test=False), "outside-30%", "invalid", "side-dependent discards end invalid, not failed"),
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


def threshold(runs: int) -> dict:
    """The sweep behind BG_SHARE_MAX: per bound, the false failures of the absolute p95 check among the runs
    that are not interfered, and the runs interfered, on the fitted bursts and on stressed ones (a longer,
    stronger shoulder and a 1.5 x slope)."""
    cases = {"fitted": (Bursts(), 1.0), "stressed": (Bursts(shoulder_s=0.6, shoulder_cpus=1.5), 1.5)}
    out = {}
    for name, (bursts, stress) in cases.items():
        for limit in (1.4, 1.45, 1.5, 1.55, 1.6, 1.75, 2.0, 2.5, math.inf):
            rng = random.Random(f"threshold/{name}/{limit}")
            fail = interfered = 0
            for _ in range(runs):
                s = bg_session(rng, BG_ENV, bursts, limit=limit, stress=stress)
                interfered += s["interfered"]
                fail += not s["interfered"] and p95_check(s["b"])["status"] != "pass"
            out[f"{name}/{limit}"] = dict(false_failure=fail, interfered=interfered, runs=runs)
    return out


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    sub = parser.add_subparsers(dest="cmd", required=True)
    f = sub.add_parser("fit")
    f.add_argument("paths", nargs="*")
    g = sub.add_parser("bgfit")
    g.add_argument("paths", nargs="*")
    sub.add_parser("threshold").add_argument("--runs", type=int, default=3000)
    for name in ("report", "calibrate", "budget", "curve"):
        sub.add_parser(name).add_argument("--runs", type=int, default=3000)
    args = parser.parse_args(argv)
    if args.cmd == "fit":
        result = fit(args.paths or sorted(glob.glob(os.path.join(ARCHIVE, "*.json"))))
    elif args.cmd == "bgfit":
        result = dict(archive=fit_bg(args.paths or sorted(glob.glob(os.path.join(BG_ARCHIVE, "set-*.json")))), model=simulated_bg())
    elif args.cmd == "threshold":
        result = threshold(args.runs)
    else:
        result = {"report": report, "calibrate": calibrate, "budget": budget, "curve": operating_curve}[args.cmd](args.runs)
    json.dump(result, sys.stdout, indent=1, default=str)
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
