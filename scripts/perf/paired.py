"""The rule of the paired performance criteria, as the scripts run it (block PB, ADR 0038,
docs/plans/2026-11-11-pb-paired-perf.md). Standard library only; `pbmodel.py` (the statistical
specification, with its environments and mutant rules) imports the rule from here, so the gate and
the model share one implementation.

* `paired_verdict`: pairs (reference A, change B) measured interleaved in a random order; the
  Hodges-Lehmann centre of the per-pair log ratios and its exact one-sided Wilcoxon signed-rank
  bounds (Hollander, Wolfe & Chicken, section 3.3). The change passes iff the upper bound of the
  median ratio is below the 10 % budget: non-inferiority (Schuirmann 1987). A run whose interval is
  too wide is invalid, never judged.
* `quantile_upper_index`, `budget_verdict`, `startup_budget`: an absolute budget on a quantile is
  certified by a distribution-free order statistic (David & Nagaraja, section 7.1).
* `invalid_start`, `run_interfered`, `slot_step`, `discard_imbalance`, `interference_report`: per-sample
  interference (plan section 18): a start whose background CPU (the machine minus the run's cgroup) exceeds
  a bound is invalid and re-measured in its slot, a run with too many invalid starts is invalid, and
  discards that depend on the side are a suspected regression. `slot_step` is the per-start decision; the
  driver's `slotStep` (start.cjs) is its twin, both checked against testdata/slot-rule.json.
* `orders`, `plan`, `classify`, `attempt`, `cool`, `run_criterion`: the randomised design, the
  decision table of an attempt (plan section 8) and the driver glue (`start.cjs --plan`).
"""

from __future__ import annotations

import functools
import json
import math
import os
import secrets
import random
import statistics
import subprocess
import time
from pathlib import Path

# ── The rule (specified by pbmodel.py) ───────────────────────────────────────────────────────────

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



def startup_budget(samples: list[float]) -> dict:
    """The absolute startup budget (ADR 0038 decision 4, user decision Q1 2026-11-11): the median at
    most STARTUP_MEDIAN_MS and the p95 at most STARTUP_P95_MS, each on its order-statistic bound
    (order 48 and 80 of 80 samples)."""
    median = budget_verdict(samples, STARTUP_MEDIAN_MS, p=0.5)
    p95 = budget_verdict(samples, STARTUP_P95_MS, p=0.95)
    parts = {}
    for name, v, limit in (("median", median, STARTUP_MEDIAN_MS), ("p95", p95, STARTUP_P95_MS)):
        parts[name] = dict(status=v["status"], order=v["order"], bound_ms=v["bound"], limit_ms=limit,
                           quantile=v["quantile"], confidence=v["confidence"])
    status = "pass" if median["status"] == p95["status"] == "pass" else "fail"
    return dict(status=status, samples=len(samples), **parts)


# ── Design, decision table, retries ──────────────────────────────────────────────────────────────

STARTUP_MEDIAN_MS = 350.0  # user decision Q1: block ST1 tightens it later, by a new gate
STARTUP_P95_MS = 400.0
COOL_MS = 200  # idle gap after every start, identical on both sides
COOL_POLL_S = 5.0  # between load checks while cooling
COOL_IDLE_S = 30.0  # idle after the load fell, before a retry
COOL_MAX_WAIT_S = 600.0  # cooling gives up waiting (the retry's own precision check judges)
READY_TIMEOUT_MS = 6000
# Per-sample interference (plan « Per-sample interference », ADR 0038 amendment 2026-11-12). BG_SHARE_MAX is
# derived from the raw background shares of 4 x 80 starts (plan section 18: the quiet edge, where no start exceeds 370 ms).
BG_SHARE_MAX = 1.5  # CPUs: background CPU time / window above this makes a start invalid
INVALID_MAX = 0.20  # the run is invalid when more than this share of the plan's starts was invalid
MAX_TRIES = 5  # measurements of one slot before the run is called interfered
QUIET_WAIT_MS = 10000  # at most, before re-measuring an invalid start: until the machine is quiet again
# Two-sided level of Fisher's exact test on the discarded starts per side (plan 18.2): the model's false alarm on an
# unchanged application under the fitted bursts is measured by pbmodel_test.Interference.test_no_false_failure_under_bursts.
IMBALANCE_ALPHA = 1e-3
SIDES = ("a", "b")  # a: the reference, b: the change


def fresh_seed() -> int:
    return secrets.randbits(64)


def orders(seed: int, n: int) -> list[bool]:
    """One independent fair coin per pair from a recorded seed: True when the reference starts first."""
    rng = random.Random(seed)
    return [bool(rng.getrandbits(1)) for _ in range(n)]


def plan(seed: int, *, port: int, bins: tuple[str, str], homes: tuple[str, str], docs: tuple[str, str],
         pairs: int = PAIRS, warmup: int = WARMUP_PAIRS, cool_ms: int = COOL_MS,
         bg_share_max: float | None = BG_SHARE_MAX) -> dict:
    """The slots of one attempt for start.cjs: the warm-up pairs, then the judged ones, each pair in
    the order of its coin. Both sides get paths and environment values of equal length (the caller
    passes sides that differ by one letter): their size biases a start by several percent."""
    side = dict(zip(SIDES, zip(bins, homes, docs)))
    slots = []
    for pair, a_first in enumerate(orders(seed, warmup + pairs)):
        for s in (SIDES if a_first else SIDES[::-1]):
            binary, home, doc = side[s]
            slots.append(dict(index=len(slots), pair=pair, side=s, warmup=pair < warmup, bin=binary,
                              home=home, docs=doc))
    return dict(seed=seed, port=port, cool_ms=cool_ms, ready_timeout_ms=READY_TIMEOUT_MS, slots=slots,
                bg_share_max=bg_share_max, invalid_max=INVALID_MAX, max_tries=MAX_TRIES, quiet_wait_ms=QUIET_WAIT_MS)


def judged_pairs(slots: list[dict]) -> list[tuple[float, float]]:
    """(A ms, B ms) of every judged pair, in pair order; the warm-up pairs are dropped."""
    by_pair: dict[int, dict[str, float]] = {}
    for s in slots:
        if not s["warmup"]:
            by_pair.setdefault(s["pair"], {})[s["side"]] = s["ms"]
    return [(p["a"], p["b"]) for _, p in sorted(by_pair.items()) if len(p) == 2]


def invalid_start(bg_share: float, limit: float = BG_SHARE_MAX) -> bool:
    """A start is invalid when the CPU of processes outside the measured tree, over its window, exceeds
    `limit` CPUs. It reads background CPU only, never the duration: a slow application is not invalid."""
    return bg_share > limit


def run_interfered(invalid: int, planned: int, cap: float = INVALID_MAX) -> bool:
    """More than `cap` of the plan's starts invalid: the run is interfered (invalid, never a pass)."""
    return invalid > cap * planned


def slot_step(bg_share: float, tries: int, discarded: int, planned: int, rule: dict) -> str:
    """The decision after one measured start of a slot (plan 18.2): "keep" it, "retry" the slot, or stop the run
    as interfered, by the cap ("stop-cap") or by the slot's tries ("stop-tries"). `tries` counts this start,
    `discarded` the invalid starts of the run before it; `rule` holds bg_share_max (None: no bound),
    invalid_max and max_tries, as the plan carries them. start.cjs's slotStep is the twin of this function."""
    limit = rule["bg_share_max"]
    if limit is None or not invalid_start(bg_share, limit):
        return "keep"
    if run_interfered(discarded + 1, planned, rule["invalid_max"]):
        return "stop-cap"
    return "stop-tries" if tries >= rule["max_tries"] else "retry"


def fisher_two_sided(a_bad: int, a_good: int, b_bad: int, b_good: int) -> float:
    """Fisher's exact test of the 2 x 2 table (side x discarded), two-sided: the probability, under the
    hypergeometric law of the table with its margins, of every table at most as likely as the observed one.
    Integer counts, so the comparison is exact (no tolerance)."""
    ra, rb, bad = a_bad + a_good, b_bad + b_good, a_bad + b_bad
    weight = {x: math.comb(ra, x) * math.comb(rb, bad - x) for x in range(max(0, bad - rb), min(ra, bad) + 1)}
    seen = weight[a_bad]
    return min(1.0, sum(w for w in weight.values() if w <= seen) / math.comb(ra + rb, bad))


def discard_imbalance(doc: dict, alpha: float = IMBALANCE_ALPHA) -> dict:
    """Discards that depend on the side. The bound reads background CPU only, so on an unchanged application
    both sides are discarded alike; a side discarded significantly more often (Fisher, two-sided, `alpha`) points
    at CPU the application causes outside its cgroup. Counts every measured start: kept and discarded."""
    count = lambda rows: {x: sum(s["side"] == x for s in rows) for x in SIDES}  # noqa: E731
    bad, kept = count(doc.get("discarded", [])), count(doc["slots"])
    p = fisher_two_sided(bad["a"], kept["a"], bad["b"], kept["b"])
    return dict(discarded=bad, kept=kept, p=p, alpha=alpha, imbalanced=p <= alpha)


def interference_report(doc: dict) -> dict:
    """The audit of a driver record: the bound and the plan's size (the driver writes them), the raw background
    share of every kept start and every discarded one, the invalid rate over the planned slots, the discarded
    durations per side and the side test."""
    kept, dropped = doc["slots"], doc.get("discarded", [])
    row = lambda s: dict(index=s["index"], side=s["side"], ms=s["ms"], bg_share=s["bg_share"], tries=s.get("try", 1))  # noqa: E731
    planned = doc.get("planned") or len(kept)
    return dict(limit=doc.get("bg_share_max"), invalid_max=doc.get("invalid_max"), planned=planned,
                kept_bg_share=[s["bg_share"] for s in kept], discarded=[row(s) for s in dropped],
                invalid_rate=len(dropped) / max(1, planned),
                invalid_by_side={x: sum(s["side"] == x for s in dropped) for x in SIDES},
                discarded_ms={x: [s["ms"] for s in dropped if s["side"] == x] for x in SIDES},
                imbalance=discard_imbalance(doc))


def classify(obs: dict, expected_pairs: int = PAIRS) -> dict:
    """The decision table of one attempt (plan sections 8 and 18). obs: port_busy (something listened on the
    port, or a server reported the address in use), accounting (no cgroup CPU accounting), imbalance (the
    side test of the discarded starts), interference (the driver stopped on the cap or the tries), not_ready
    (the side whose server did not become ready, else None), pairs (the judged pairs when every slot was
    ready). Side-dependent discards are a failure even when the cap stopped the run; a complete run has
    `expected_pairs` judged pairs, and fewer is a failure, never judged on what is left."""
    if obs["port_busy"]:
        return dict(outcome="invalid", reasons=["environment: the port is busy"])
    if obs.get("accounting"):
        return dict(outcome="invalid", reasons=["environment: cgroup CPU accounting unavailable"])
    imbalance = obs.get("imbalance")
    if imbalance and imbalance["imbalanced"]:
        d = imbalance["discarded"]
        return dict(outcome="regression", imbalance=imbalance, reasons=[
            f"suspected regression: the discarded starts depend on the side (A {d['a']}, B {d['b']}; "
            f"Fisher p = {imbalance['p']:.2g} <= {imbalance['alpha']})"])
    if obs.get("interference"):
        return dict(outcome="invalid", reasons=["environment: background CPU interference"])
    if obs["not_ready"]:
        return dict(outcome="fail", reasons=[f"not ready: {obs['not_ready']}"])
    if len(obs["pairs"]) != expected_pairs:
        return dict(outcome="fail", reasons=[f"incomplete: {len(obs['pairs'])} judged pairs, the plan has {expected_pairs}"])
    v = paired_verdict(obs["pairs"])
    return dict(outcome=v["status"], reasons=[v["reason"]], verdict=v)


def cool(load=lambda: os.getloadavg()[0], sleep=time.sleep, cpus: int | None = None,
         idle_s: float = COOL_IDLE_S, max_wait_s: float = COOL_MAX_WAIT_S) -> None:
    """Wait until the 1-minute load is at most cpus / 4 (the heavy.sh --exclusive threshold), at most
    max_wait_s, then stay idle."""
    limit = (cpus or os.cpu_count() or 1) / 4
    waited = 0.0
    while load() > limit and waited < max_wait_s:
        sleep(COOL_POLL_S)
        waited += COOL_POLL_S
    sleep(idle_s)


def attempt(measure, classify=classify, cool=cool, attempts: int = ATTEMPTS, seeds=fresh_seed) -> dict:
    """Up to `attempts` runs, each with a fresh seed after cooling; only an invalid run is retried, so a
    valid verdict (pass, regression) or a failure is final. After the last invalid run the criterion is
    invalid: never a pass, never a regression."""
    history = []
    for n in range(1, attempts + 1):
        if n > 1:
            cool()
        seed = seeds()
        result = dict(classify(measure(seed, n)), seed=seed)
        history.append(result)
        if result["outcome"] != "invalid":
            break
    final = history[-1]
    return dict(outcome=final["outcome"], reasons=final["reasons"], attempts=history)


# ── Driver glue: start.cjs --plan ────────────────────────────────────────────────────────────────

DRIVER = Path(__file__).with_name("start.cjs")


def observation(doc: dict) -> dict:
    """The driver's record of an attempt as the decision table's observation."""
    fault = doc.get("fault") or {}
    return dict(port_busy=fault.get("kind") == "port-busy", accounting=fault.get("kind") == "accounting",
                interference=fault.get("kind") == "interference", imbalance=discard_imbalance(doc),
                not_ready=fault.get("side") if fault.get("kind") == "not-ready" else None,
                pairs=judged_pairs(doc["slots"]) if not fault else [])


def run_criterion(out: Path, bin_a: str, bin_b: str, *, port: int, node: str = "node", work: Path | None = None,
                  cool=cool, seeds=fresh_seed, cool_ms: int = COOL_MS, driver: Path = DRIVER,
                  bg_share_max: float | None = BG_SHARE_MAX) -> dict:
    """The paired measurement of a startup criterion: `attempt` over the driver, one raw file per attempt
    (`start-pairs-<n>.json`, every slot with its order, duration, load, pressure and background CPU, and the
    discarded starts), the verdict of
    the final attempt and the absolute startup budget on B's judged starts."""
    out.mkdir(parents=True, exist_ok=True)
    work = work or out  # where the two sides keep their home and documents: equal-length paths
    raws = {}

    def measure(seed, n):
        raw = out / f"start-pairs-{n}.json"
        spec = plan(seed, port=port, bins=(bin_a, bin_b), homes=(str(work / "start-home-a"), str(work / "start-home-b")),
                    docs=(str(work / "start-docs-a"), str(work / "start-docs-b")), cool_ms=cool_ms, bg_share_max=bg_share_max)
        spec["out"] = str(raw)
        spec_path = out / f"start-plan-{n}.json"
        spec_path.write_text(json.dumps(spec))
        done = subprocess.run([node, str(driver), "--plan", str(spec_path)], capture_output=True, text=True)
        if not raw.exists():
            raise RuntimeError(f"start driver failed ({done.returncode}): {done.stderr[-2000:]}")
        raws[seed] = str(raw)
        return observation(json.loads(raw.read_text()))

    result = attempt(measure, cool=cool, seeds=seeds)
    for a in result["attempts"]:
        a["raw"] = raws[a["seed"]]
    final = result["attempts"][-1]
    result["verdict"] = final.get("verdict")
    result["interference"] = interference_report(json.loads(Path(final["raw"]).read_text()))
    if result["verdict"] is not None:  # a complete run: the budget is judged on its 80 judged starts of B
        judged = json.loads(Path(final["raw"]).read_text())["slots"]
        result["budget"] = startup_budget([b for _, b in judged_pairs(judged)])
    return result


# ── Self-test, no process ────────────────────────────────────────────────────────────────────────


def self_test() -> str:
    """Synthetic pairs: an unchanged product passes, a x1.2 regression regresses, a wide run is
    invalid. Returns the line printed by baseline.py and required by gate PB."""
    rng = random.Random("self-test")

    def pairs(shift, sd):
        return [(370.0, 370.0 * math.exp(shift + rng.gauss(0, sd))) for _ in range(PAIRS)]

    assert paired_verdict(pairs(0.0, 0.01))["status"] == "pass"
    assert paired_verdict(pairs(math.log(1.2), 0.01))["status"] == "regression"
    assert paired_verdict(pairs(0.0, 0.5))["status"] == "invalid"
    assert quantile_upper_index(30) is None and quantile_upper_index(80, 0.5) == 48
    return "paired-randomized (A/A pass, x1.2 regression, wide run invalid)"
