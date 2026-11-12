---
status: accepted
date: 2026-11-11
amends: docs/engineering-principles.md section 3 (performance proofs); gate L0 criterion `baseline` (PERF-START)
decision-makers: [Mathieu]
---

# Performance criteria are paired and interleaved, and judged as non-inferiority bounds

Plan: `docs/plans/2026-11-11-pb-paired-perf.md`. Formal model: `scripts/perf/pbmodel.py` (rule, environments and mutant
rules). Rule implementation: `scripts/perf/paired.py` (step 2). Gate: `gates/PB.toml`.

## Context and problem

PERF-START compared a fresh p95 of 30 starts with an absolute limit archived on 2026-11-02 (baseline p95 328.2 ms,
limit 361.0 ms). On 2026-11-11 the unchanged HEAD measured p95 395 and 667 ms: the level of the machine moved by 29 %
between sessions (nine archived sample sets, medians 293 to 377 ms,
`docs/perf/2026-11-11-start-drift/`), and one outlier moves a p95 of 30 samples (the 29th order statistic) by 75 %.
The criterion judged the machine, not the change: it fails unrelated work and would equally pass a real regression on
a quiet day. In the model, the incumbent rule gives 55 % false regressions on the fitted noise.

## Decision drivers

- Error rates must be known and bounded at the budget itself, not only "no significant difference".
- No assumption on the noise: drift, steps, bursts, correlation, carry-over and heavy tails all occur on this host.
- An environmental fault must never become a pass or a regression.
- Cost: a few minutes per gate, one reference build per merge-base.

## Considered options

- Absolute archived baseline (the incumbent).
- Significance tests without a margin (Pinpoint KS and Mann-Whitney, Perfherder Mann-Whitney with Cliff's delta,
  benchstat).
- Ratio of p95, mean ratio with a t or bootstrap bound (Kalibera and Jones), point median, a 95 % bound.
- Sequential blocks (all A then all B), fixed order (A then B), duet benchmarking (A and B concurrently).
- Randomised interleaved pairs with an exact signed-rank bound (chosen).

## Decision

Option retained: randomised interleaved pairs judged by an exact one-sided signed-rank bound, because it is the only
option whose error rates are exact under any drift (derivation: plan section 3.3).

1. A regression criterion compares the change with its committed reference (merge-base with main), both built on this
   machine by the same build function and measured in the same session, pair by pair, in an order drawn at random
   for each pair (randomised multiple interleaved trials, Abedi and Brecht 2017). The reference build is cached by
   commit and toolchain.
2. The budget is an effect size, tested as non-inferiority (Schuirmann 1987): the change passes iff the exact
   one-sided 99.9 % Wilcoxon signed-rank upper bound (Hodges-Lehmann estimate, Walsh averages) of the median ratio
   change/reference is below 1.10. Randomisation makes the bound exact under any drift. A run whose interval is wider
   than 4 % on each side is invalid; with that limit a regression verdict always means a slowdown significant at
   1e-3. Sizes: 80 judged pairs after 2 warm-up pairs.
3. An absolute budget on a quantile is certified by its distribution-free order-statistic upper bound at 95 %
   confidence (David and Nagaraja, section 7.1), never by a point quantile of few samples. Native typing therefore
   takes 100 warm samples (bound = second largest) and 59 cold fresh processes (bound = the maximum), user decision
   2026-11-11.
4. Absolute startup budget (the Chromium proxy, to a usable editor), user decision 2026-11-11: median <= 350 ms and
   p95 <= 400 ms, each judged on an order-statistic bound of the 80 judged starts of the change. Block ST1
   (`docs/plans/2026-11-11-startup-speed.md`) builds three gains, after which the budget is tightened to what ST1
   measures (target about 300 ms median, 350 ms p95); that tightening is a new gate, not a margin. No native startup
   budget exists until a native measurement does.
5. An environmental fault (port busy, focus lost, input delivery lost, activation failed, imprecise run) makes a run
   invalid; an invalid run is retried up to three times after cooling, then the criterion fails as invalid. A valid
   verdict is never retried. A server that does not become ready is a failure, not an invalid run.

   **Amendment (2026-11-12): the invalid-run rule applies per start.** The absolute p95 budget is judged on the 80th of
   80 starts, so a single start that coincides with a burst of another process (the gortex daemon: all CPUs for ~0.12 s
   every ~7.7 s) fails a run whose paired verdict and median pass. The driver therefore records, for each start, the
   background CPU of its window: the busy CPU of the machine (`/proc/stat`) minus the CPU of the run's cgroup v2
   subtree (`cpu.stat` `usage_usec`), which holds the driver, the browser and, in a fresh cgroup per start joined
   before the binary executes, the application. A process cannot leave its cgroup by forking or calling `setsid`, and
   the cgroup keeps the CPU of reaped children, so a detached helper's CPU is the application's; whatever is left in a
   start's cgroup is killed with it (`cgroup.kill`). A descendant of the driver moved out by a cgroup writer (Chromium puts
   its browser process in its own systemd scope) is moved back before every start. Without a writable cgroup v2, or a
   descendant that cannot be moved back, the attempt is invalid, never measured.
   A start whose background CPU over its window exceeds 1.5 CPUs is invalid and is re-measured in its own slot, so the
   randomised order is kept; the bound reads background CPU, never the duration. A run in which more than 20 % of the
   plan's starts were invalid is invalid, never a pass, and the retry above applies. The discards must not depend on
   the side: Fisher's exact test (two-sided) on the discarded and kept starts of A and B at level 1e-3 makes a
   significant imbalance a failure, a suspected regression whose slowness lands outside the cgroup (a service the
   application wakes), even when the 20 % cap stopped the run. The 350 ms and 400 ms limits and every other threshold
   are unchanged. The bound is the quiet edge of 640 archived starts (none of the 535 at or below it exceeds 370 ms; all
   49 over 400 ms are above it). At the bound, the model shows a false failure of the absolute check of at most 1e-3
   and a false alarm of the side test of at most 1e-3 (7 in 30000 runs), while a truly slow application fails: +20 % on
   every start and 10 % of slow starts with probability >= 0.99, 8 % and 12 % of slow starts in a detached helper with
   probability >= 0.99 (judged exactly as the same slowness in the application's own process); 30 % of slow starts in
   a service outside the cgroup pass in at most 1 % of the criteria. Derivation and limits: plan section 18.

   **Native typing (2026-11-12).** An environment fault explains only what is missing: a native run whose other
   reasons are all missing samples, controls or replicates is invalid; any genuine failure beside the fault (a budget
   exceeded on the samples that were measured, an integrity check, an uncoded failure) makes it fail, with the fault
   among its reasons. Focus taken by the product is not a fault: a window of the measured process or of one of its
   descendants that takes the focus, or a focus moved inside the page, is a failure.

### Consequences

- Good: machine drift and outliers no longer fail unrelated work: 0 false regressions in 3000 simulated runs in each of
  seven environments fitted to and beyond this host's samples; a true +10 % fails with probability >= 0.995.
- Bad: changes up to +5 % pass and +10 % fail; between them the verdict depends on the noise (indifference zone).
- Bad: PERF-START costs about 2 more minutes per L0 run, plus one reference build per merge-base; the native typing
  run takes about 1.5 more minutes.
- Bad: a change that only adds rare slow starts moves the median little; only the absolute budget bounds the tail
  (plan risk R3).
- The archived absolute limit of 2026-11-02 is kept as history and no longer gates; its 10 % multiplier is read.
- Out of scope: the Go hot-path guard `regression.py --bench` (follow-up F1) and a paired native comparison (F2).

### Confirmation

`scripts/perf/pbmodel_test.py` (false regression and power per environment, every mutant rule rejected),
`paired_test.py` (attempt machine, order statistics), `refbuild_test.py`, `paired_e2e_test.py` (real driver against a
fake server and the real server), and the frozen gate `gates/PB.toml`.

## More information

Rejected alternatives with their measured violations: plan section 5 (absolute p95 55 % false regressions; p95 ratio
22 %; mean with a t bound 58 % under heavy tails; point median power 0.49; 95 % bound power 0.95; blocked order never
decides under drift; fixed order power 0.01 with a carry-over). Sources: plan section 17. Revisit if the host proves
noisier than the `heavy` environment (plan risk R2: add a sequential extension through a new spec and gate, never a
wider margin).
