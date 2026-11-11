# Block PB — paired, interleaved performance criteria (2026-11-11)

Status: step 1 (specification) — draft for review. Rigour: **complete** (docs/engineering-principles.md § 0): the
block changes the machinery that judges every later block's performance, it has a retry machine of six states and a
shared cache, and a wrong rule silently passes regressions or blocks unrelated work. Proofs: the statistical model
with mutant rules (`scripts/perf/pbmodel.py`, the formal model of this block, section 5), the attempt machine as an
exhaustive decision table test (section 8), the frozen gate `gates/PB.toml`, one independent review by another model
family before the gate is frozen (Codex from 2026-11-12, or an OpenRouter non-Claude model via `scripts/llm.sh`).
ADR: 0038 (section 15; 0037 is reserved for OI).

## 1. Need

User decision 2026-11-11: fix the performance method at the root. Evidence collected the same day:

- PERF-START (`scripts/perf/baseline.py`, gate L0 criterion `baseline`, chained into every later gate through the
  regression criteria) compares the p95 of 30 sequential starts with an absolute limit archived on 2026-11-02
  (`docs/perf/baseline/2026-11-02/thresholds.json`: baseline p95 328.2 ms, limit 361.0 ms).
- The committed HEAD (f78949b) measured twice on 2026-11-11 gives medians 360 and 366 ms, p95 395 and 667 ms; the
  L0 gate run of 15:51 gives median 377 ms, p95 518 ms. The level of the machine moved (gortex 7 GB RSS, GoLand,
  load ~3 on 20 CPUs): the nine sample sets on disk have medians from 293 to 377 ms (+29 %) with no relevant change.
- The p95 of 30 samples is the 29th order statistic: one outlier moves it from 380 to 667 ms inside one set.
- The uncommitted blocks C5S and LF0a3 measured 365-372 ms median, inside HEAD's own spread.

The criterion fails for reasons unrelated to the change under test, and it would equally pass a real regression on
a quiet day. Two defects, two fixes: (a) the reference is absolute and stale — compare with the committed reference
built and measured now, interleaved; (b) the statistic is a single high order statistic — judge a robust effect size
with a bound whose error rates are known.

The Go hot-path guard (`scripts/perf/regression.py --bench`) already measures the merge-base and the branch in one
interleaved session; it is out of this block's scope (section 14, follow-up F1).

## 2. Noise of this host (fitted, `python3 scripts/perf/pbmodel.py fit`)

Nine distinct startup sample sets, 270 starts (archived by step 2 in `docs/perf/2026-11-11-start-drift/`, with
SHA256SUMS): the 2026-11-02 reference and baseline, the 2026-11-03 merge-base, the 2026-11-11 L0 gate run, and the
five 2026-11-11 diagnostic runs (HEAD twice, LF0a3, C5S, both).

| Quantity (log of a duration ratio) | Fitted | Model stress environments |
|---|---|---|
| bulk scale, 1.4826 MAD per set | 1.8-3.9 % | ×1.5 (`worst`), ×1.6 (`heavy`) |
| outliers (> 4 MAD) | 19/270 = 7.0 %, excess ×1.09 to ×2.79, in 2-3 per set | 15 % in bursts of 3-4 starts (`bursty`, `worst`); 20 % with a Pareto tail (x_m 0.08, a = 1.1, cap ×7.4) (`heavy`) |
| first start of a session | +0.32 and +0.38 in 2 of 9 sets | same, always modelled |
| drift inside a set (Theil-Sen) | -8.4 % to +1.3 % per 30 starts | linear 30 % per run, ±15 % oscillation of period 60 starts (5.6 × the steepest slope), a ±25 % step (`drift`, `worst`) |
| lag-1 correlation | -0.16 to +0.16 | AR(1) 0.6 (`ar`), 0.5 (`worst`) |
| carry-over between the two starts of a pair | not measured | second start 4 % faster (`carry`, `worst`) |
| level between sessions | 293-377 ms | U(0, +25 %) between the archived and the fresh session (incumbent mutant only) |

`Fit.test_stress_environments_dominate_the_fit` checks every stress environment against the refit of the archive.

## 3. Decision rule (regression criteria)

### 3.1 Design

A run measures **N = 80 judged pairs** after **2 warm-up pairs**. Each pair is one start of the reference build A
and one of the change B, in an order drawn by an independent fair coin per pair from a recorded 64-bit seed
(randomised multiple interleaved trials, Abedi & Brecht 2017). Both sides use the same browser instance, port,
command line, environment variables (same names, values of equal length), binary paths of equal length and homes of
equal length (`…/start-home-a`, `…/start-home-b`): environment size and path length bias measurements by several
percent (Mytkowicz et al. 2009). A fixed idle gap of `COOL_MS = 200` follows every start, identical on both sides.

### 3.2 Statistic

For pair i, `d_i = ln B_i - ln A_i`. The estimate is the Hodges-Lehmann centre of `d` (median of the
`M = N(N+1)/2 = 3240` Walsh averages); the bounds are the exact one-sided Wilcoxon signed-rank bounds at
`ALPHA = 1e-3`: with `c` the largest integer such that `P(T+ <= c) <= ALPHA` under the exact null distribution
(`c = 983` for N = 80; the normal approximation gives 976), `L = W_(c+1)` and `U = W_(M-c)`. Reported as ratios: `exp(HL)` (the median ratio),
`exp(L)`, `exp(U)`.

**Verdict:** `half = (U - L) / 2`; if `half > ln 1.04` the run is **invalid** (imprecise); else **pass** iff
`U < ln 1.10`, else **regression**. The 10 % is `thresholds.json` `e2e_multiplier`, read from the immutable archive.

### 3.3 Why it is exact under drift (derivation)

Write the slot offsets of the environment `e_t` (anything: drift, steps, bursts, correlation, carry-over). With
the coin `s_i` choosing which of slots `2i, 2i+1` is B, `d_i - theta = s_i (e_{2i+1} - e_{2i})` for a multiplicative
change `theta`. Conditionally on the whole environment realisation, the `s_i` are independent fair signs, so
`d_i - theta` is symmetric about 0 and independent across pairs. The signed-rank statistic of `d - theta` then has
exactly its null distribution (Fisher's randomisation argument), and by Tukey's identity (`T+(theta)` counts the
Walsh averages above `theta`) `P(U < theta) <= ALPHA` and `P(L > theta) <= ALPHA` — **for every environment**,
without any assumption on the noise. The environment's level never enters `d` except through the difference of two
adjacent slots; randomisation turns any systematic slot effect into symmetric noise.

Consequences, per valid attempt:

- **Power at the budget.** At a true ratio 1.10, `P(pass) = P(U < ln 1.10) <= ALPHA`. Over at most 3 attempts,
  `P(pass) <= 3 ALPHA = 3e-3`: power >= 0.997, distribution-free.
- **A regression is always a significant slowdown.** On a valid run `L = U - 2 half >= ln 1.10 - 2 ln 1.04 = +1.7 %`.
  So `regression` implies the change is slower than the reference at the 1e-3 level by at least 1.7 %:
  `P(false regression | no change) <= P(L > +1.7 %) < ALPHA` per attempt, and far smaller in practice (at
  `half = 4 %`, `L > 1.7 %` needs the estimate 4.7 standard errors above 0; the model sees 0 in 21 000 null runs).
  This is why `HALF_WIDTH_MAX` must stay below half the budget (4.77 %).
- **Validity is decided by precision only.** The half-width is invariant under any shift of `d`, so a slowdown,
  however large, never makes a run invalid; the coverage of the bounds does not depend on the check (the model
  measures the conditional miss rate at 0.0008-0.0011 for `ALPHA = 1e-3`).

The estimand is the median ratio. A change that only fattens the tail is caught by the absolute budget (section 6),
not by this rule (risk R3).

### 3.4 Non-inferiority, not significance

Pinpoint (min of KS and Mann-Whitney p-values, low threshold 0.01, `Unknown` asks for more data), Perfherder
(Mann-Whitney p < 0.05 with Cliff's delta) and benchstat (Mann-Whitney, alpha 0.05) test "is there a difference".
That question has no budget: with many samples a +1 % change is "significant", with few a +30 % one is not, and the
error at the budget is uncontrolled. The PB rule tests the question the gate asks — "is the change shown to stay
within +10 %?" — as a one-sided non-inferiority test (Schuirmann 1987), with both error rates bounded at the budget.

## 4. Sample size and operating characteristic

Sample-size sweep (`calibrate`, 1000 runs per cell; final invalid = all three attempts invalid; at a true +10 %):

| N pairs | fitted: final invalid at null / mean attempts | worst: final invalid at null / mean attempts | heavy: final invalid at null | passes at +10 % (all) |
|---|---|---|---|---|
| 20 | 386 / 2.29 | 925 / 2.93 | 996 | <= 1 |
| 30 | 73 / 1.59 | 744 / 2.72 | 995 | <= 1 |
| 40 | 1 / 1.17 | 336 / 2.15 | 967 | <= 1 |
| 60 | 0 / 1.01 | 14 / 1.28 | 783 | 0 |
| 80 | 0 / 1.00 | 0 / 1.04 | 352 | <= 2 |

N = 80 is the smallest tested size at which every environment of the envelope decides (0 final invalid in 1000).
`heavy` (three times the observed contamination with an infinite-variance tail) stays invalid in a third of the
criteria: the correct answer there is "the machine is too noisy", and it is never a regression.

Guarantees (`report`, 3000 runs per cell; 0/3000 bounds a rate by 1e-3, exact one-sided 95 % Clopper-Pearson):

| Environment | null: regression | null: final invalid | mean attempts | +10 % ×: passes | +10 % fixed cost: passes |
|---|---|---|---|---|---|
| fitted | 0 | 0 | 1.000 | 1 | 2 |
| drift | 0 | 0 | 1.001 | 3 | 1 |
| bursty | 0 | 0 | 1.020 | 8 | 5 |
| carry | 0 | 0 | 1.009 | 2 | 2 |
| ar | 0 | 0 | 1.000 | 3 | 3 |
| worst | 0 | 0 | 1.032 | 5 | 0 |
| heavy | 0 | 994 | 2.184 | 0 | 0 |

False regression <= 1e-3 in every environment; the worst power is 2992/3000, lower bound 0.9953 >= 0.99. Half-width
of a run: median 1.7 % (fitted), 2.9 % (worst), 4.5 % (heavy).

Operating characteristic (fitted, 3000 runs, P(pass) against a true slowdown):

| +0-4 % | +5 % | +6 % | +7 % | +8 % | +9 % | +10 % | +12 % |
|---|---|---|---|---|---|---|---|
| 1.000 | 0.998 | 0.974 | 0.884 | 0.579 | 0.111 | 0.0013 | 0 |

A change up to +5 % passes, one of +10 % or more fails; the indifference zone +6-9 % is the price of controlling the
error at the budget itself.

## 5. Rejected rules (mutants of the model, `Mutants.test_every_mutant_is_rejected`)

| Mutant | Environment | Violation (3000 runs) |
|---|---|---|
| `absolute-p95`: the incumbent, p95 of 30 against 1.10 × an archived p95 | fitted | 1660 false regressions (55 %) |
| `p95-ratio`: interleaved, p95(B) / p95(A) <= 1.10 | fitted | 667 false regressions (22 %) |
| `median-point`: median ratio <= 1.10, no bound | fitted | power 0.49 |
| `alpha-0.05`: the same rule with a 95 % bound | fitted | power 0.953 |
| `mean-t`: mean log ratio with a t bound (Kalibera-Jones style mean) | heavy | 1733 false regressions (58 %) |
| `blocked`: all A then all B (no interleaving) | drift | 3000 never decide (invalid) |
| `fixed-order`: always A then B | carry | power 0.011 (B always gets the warm slot) |

Code mutants of the rule itself, each run against `Rule` and `Guarantees` (2026-11-11, draft): `ALPHA = 0.05`
(power and critical-value tests red), no precision check (`HALF_WIDTH_MAX = 10`: false regressions in `heavy`, and
the imprecise-run tests red), the coin removed from `session` (always A first: power red in `carry`). Step 5 repeats
them on `scripts/perf/paired.py` (manual mutants, `SIMPLIFY.md` « mutation paired »; gremlins covers Go only).

Considered and not chosen: Kalibera & Jones' bootstrap of the mean ratio (means are outlier-driven on this host, the
verdict would depend on resampling randomness, and the percentile bootstrap of a median is poorly calibrated at
small n); the sign test (valid, wider: 64 % efficiency under normal noise); Mann-Whitney / two-sample
Hodges-Lehmann on the same interleaved data (valid, but discards the pairing that cancels drift); duet benchmarking
(Bulej et al. 2020: A and B concurrently) — two cold starts in parallel contend for the same browser and CPU,
change the quantity measured, and give no absolute figure; tango-style paired in-process benchmarking does not
apply to process start-up. A sequential extension (add pairs when imprecise, as Pinpoint's `Unknown`) would reduce
invalid runs in `heavy`; rejected for now as unneeded inside the envelope (risk R2).

## 6. Absolute budgets: order statistics

A budget on a quantile stays an absolute requirement. Its distribution-free upper confidence bound is an order
statistic: `X_(k)` with the smallest `k` such that `P(Binomial(n, p) <= k - 1) >= confidence` (David & Nagaraja,
Order Statistics, § 7.1). The budget passes iff `X_(k) <= budget`. For p = 0.95, confidence 0.95:

| n | 30 | 45 | 59 | 60 | 80 | 100 | 150 | 300 |
|---|---|---|---|---|---|---|---|---|
| k | none | none (45 at 90 %) | 59 (max) | 60 (max) | 80 (max) | 99 | 148 | 292 |

Thirty samples can never certify a p95; the point p95 of 30 passes a product whose true p95 is 5 % over budget in a
large share of runs (`Budget.test_p95_of_30_accepts_a_noncompliant_product`), the bound in <= 5 %.

**PERF-START.** The 80 judged starts of B give `k = 80` (their maximum). Against the spec budgets
(`docs/architecture/spec.md`, table "Budget p95": window painted <= 1 s, last project editable <= 1.5 s), the model
gives false failures of 29.4 % at 1 s and 0/3000 at 1.5 s (fitted): two of the 270 starts on disk took 992 and
1018 ms. The proxy measures "usable editor", the scope of "last project editable"; the plan proposes 1.5 s (open
question Q1). The budget verdict is reported next to the regression verdict and both must pass.

**PERF-TYPE-native.** The verifier (`internal/nativeperf/verdict.go`, `metric`) takes the point p95 of >= 30
samples; at 30 the bound does not exist. Proposed (open question Q2): warm samples per run 30 -> 100 (k = 99, the
second largest; keystrokes are cheap), cold fresh processes 30 -> 59 (k = 59, the maximum); the budgets (warm handler
<= 8 ms, cold <= 50 ms, frame work <= 16 ms) apply to the bound of the same conservative per-sample upper values the
verifier already computes; the point p95 stays in the report as a diagnostic. Native runs are single-arm absolute
measurements, not regression comparisons; a paired native comparison is follow-up F2.

## 7. Reference build

- **Reference commit**: `git merge-base HEAD main`, as `regression.py` (on main with uncommitted work: HEAD). The
  change under test is the working tree, uncommitted files included. If the working tree's build is byte-identical
  to the reference (same binary SHA-256), the run is an A/A comparison, reported `identity: true`; it must pass.
- **Build**: one function `refbuild.build_server(tree, out)` builds both sides with the same command vector (the
  vite production build into `frontend/dist`, then `go build -tags server -trimpath -o <out> ./cmd/inkwell`; the
  `-trimpath` keeps the two binaries free of their checkout paths). The branch build still also produces
  `bin/inkwell-server` for the other L0 steps.
- **Worktree**: a private `git clone --bare --shared` in the cache, then `git worktree add --detach` at the reference
  (the source checkout's `.git` is never written, as in `regression.py`). Frontend dependencies:
  `pnpm install --frozen-lockfile --prefer-offline` in the worktree (the global store makes it a link farm).
  Sidecars (`internal/sidecar/bin/<goos>_<goarch>`, ignored by git, embedded by `go:embed`): copied from the source
  checkout when the reference's `internal/sidecar/manifest.json` equals the source's and every copied file matches
  its pinned SHA-256; otherwise `mise run sidecar:fetch` in the worktree. Never a download at run time (ADR 0024).
- **Cache**: `~/.cache/inkwell/perf-ref/<key>/` holding `inkwell-server` and `build.json` (commit, key inputs,
  binary SHA-256, build time). `key = SHA-256` of: reference commit, `go version`, `node --version`,
  `pnpm --version`, the build command vector, `GOFLAGS`/`CGO_ENABLED`/`GOAMD64`/`GOARCH`, the sidecar manifest.
  Built in a temporary sibling directory, published by `os.rename` (atomic), under `flock` on
  `~/.cache/inkwell/perf-ref.lock`; a hit re-hashes the binary against `build.json` and rebuilds on mismatch; the 6
  most recent entries are kept. The worktree is removed after the build; only the binary is cached.
- A reference that does not build fails the criterion with "reference build failed" (fail closed, never invalid).

## 8. Interleaving, cooling, invalid runs, retries

Measurement (`scripts/perf/start.cjs --plan <json>`): for each slot of the plan, check that nothing listens on the
port, open a page, spawn the side's binary with its home and documents directories, poll `/health` every 20 ms (at
most 6 s), load the page, wait for `.cm-content`; record the duration, the exit status, the last 2 KiB of stderr
(per-side log), `/proc/loadavg` and the `/proc/pressure/cpu` and `/proc/pressure/io` `some total` deltas over the
slot (diagnostics, never a verdict input); kill, await exit, close the page, sleep `COOL_MS`.

Classification of one attempt (exhaustive decision table, `paired_test.test_attempt_machine_exhaustive`):

| Observation | Outcome | Retried |
|---|---|---|
| something listens on the port before a slot, or a server exits reporting the address in use | invalid (environment: port busy) | yes |
| a server exits or times out before ready, any other cause, either side | fail "not ready: <side>" | no |
| all slots ready, half-width > 4 % | invalid (imprecise) | yes |
| all slots ready, precise, U < ln 1.10 | pass | no |
| all slots ready, precise, U >= ln 1.10 | regression | no |

Retry: an invalid attempt is followed, after cooling (wait until the 1-minute load is <= `nproc / 4`, the
`heavy.sh --exclusive` threshold, then 30 s idle), by a fresh attempt with a fresh seed; at most 3 attempts. After
3 invalid attempts the criterion fails with status `invalid` and the reasons — never `pass`, never `regression`.
A valid verdict is never retried (retrying until a pass is the multiple-testing error benchstat warns against).
The budget verdict is computed on the final valid attempt.

Native typing: the driver already detects focus loss, lost X11 delivery and failed activation. Step 2 makes them
an explicit `EnvironmentFault(kind)` (`native_input.py`; kinds `focus-lost`, `delivery-lost`, `activation-failed`,
`permission-dialog`), written to `driver.json` and signalled by exit status 75 (EX_TEMPFAIL); the frontend's focus
loss (`frontend/src/perf/native.ts`) carries the code `focus-lost` next to its message; the Go verifier turns a coded
environment fault into status `invalid` instead of adding performance reasons; `type_native.py` retries with a fresh
raw directory under the same cooling and attempt limit, and after 3 writes the verdict through a new
`--native-perf-invalid=<reasons>` flag. A setup error (wrong engine, missing binary) is not an environment fault.

## 9. What changes where (blueprint for go-dev)

New (all under `scripts/perf/`, Python standard library, `node` for the driver):

1. `paired.py` — the rule, moved verbatim from the model's rule section (`signed_rank_cdf`, `critical`,
   `hl_bounds`, `paired_verdict`, `binomial_pmf`, `quantile_upper_index`, `budget_verdict`, constants), plus
   `orders(seed, n)` (independent coins), `attempt(classify, measure)` (the decision table) and `cool()`.
   `pbmodel.py` then imports the rule from `paired` (`paired_test.test_model_shares_the_rule`).
2. `paired_test.py` — unit tests, fast, part of `test:py`.
3. `pbmodel.py`, `pbmodel_test.py` — the model (draft in the scratchpad), `ARCHIVE` set to
   `docs/perf/2026-11-11-start-drift`; run by the gate, not by `test:py` (one minute).
4. `refbuild.py`, `refbuild_test.py` — section 7; tests use a temporary git repository and a stub build vector.
5. `paired_e2e_test.py` and `testdata/fakeserver.py` — the real driver with Playwright Chromium against a fake
   server (`/health`, a page with `.cm-content`, a start delay from argv), and against the real `bin/inkwell-server`
   with a delay wrapper on B.

Changed:

6. `start.cjs` — the plan-driven paired driver of section 8 (its only caller is `baseline.py`); writes
   `start-pairs-<attempt>.json` with every slot, and `start.json` (B's samples and point p95, for continuity of the
   L0 report).
7. `baseline.py` — `start_verdict` (absolute) replaced: build B (as today) and A (`refbuild`), copy both to
   `<out>/bin/a|b/inkwell-server`, run attempts, write `measurements.json` `PERF-START` = {design
   `paired-randomized`, reference {commit, key, sha256, cache hit}, branch {sha256}, identity, attempts [{seed,
   outcome, reasons, verdict}], verdict, budget}. The self-test (`INKWELL_BASELINE_SELF_TEST=1`, run by
   `gate-selftest.py`) checks the verdict on synthetic pairs: A/A passes, ×1.2 regresses, a wide run is invalid.
8. `type_native.py`, `native_input.py` (+ their tests), `frontend/src/perf/native.ts` (+ test),
   `internal/nativeperf/verdict.go` and `main`/flag code (+ tests): section 8, native part; section 6 if Q2 is
   accepted (`QuantileUpperIndex` in Go, the twin of `quantile_upper_index`, with the same table as a test vector).
9. `scripts/gate-selftest.py` — the one comment above the baseline self-test call, which describes the old rule.

Unchanged: every `gates/*.toml` (the L0 command stays `./scripts/heavy.sh --exclusive python3
scripts/perf/baseline.py`), the immutable archive `docs/perf/baseline/2026-11-02/` (its multiplier is read, its
absolute limit is no longer used), `regression.py`, `heavy.sh`.

Documents: `docs/adr/0038-paired-performance.md`; `docs/perf/2026-11-11-start-drift/` (the nine sets, README with
provenance, SHA256SUMS); `docs/exigences.md` (the 2026-11-11 request), its place in
`docs/architecture/coherence.md` § 5 and `docs/objectif-en-cours.md`; `PROGRESS.md`; a pointer from
`docs/engineering-principles.md` § 3 to ADR 0038 (one home for the rule: the ADR and `paired.py`).

## 10. Guard overrides

`scripts/gate-guard.py` flags any change under `scripts/perf/**` and to `scripts/gate*.py`. Each changed or added
file gets one entry in `gates/guard-overrides.json` (the ID is the one the guard prints for the exact finding and
contents), `decision: docs/plans/2026-11-11-pb-paired-perf.md`, and a reason naming what changed and which test
proves it (example: "Block PB: baseline.py replaces the absolute PERF-START limit by the paired rule of ADR 0038;
proven by pbmodel_test (false regression 0/3000 per environment, power >= 0.995) and paired_e2e_test"). No
threshold is weakened: the 10 % budget is kept, and the absolute limit is replaced by a stronger, two-sided
guarantee. Overrides are written after the code, from the guard's own findings, and reviewed with the diff.

## 11. Cost in wall time (estimates; step 2 measures and updates this table)

| Step | Today | PB |
|---|---|---|
| reference build | none | cache miss ~2-3 min (clone 2 s, pnpm link 20-40 s, vite 30-60 s, go 40-90 s); hit < 1 s |
| start measurement | 30 starts, ~15 s | 164 starts × ~0.65 s ≈ 1.8 min per attempt; 1.00-1.03 attempts in the envelope |
| L0 `baseline` criterion | builds + ~15 s | builds + ~2 min (hit), + ~4.5 min (miss); worst case 3 attempts + 2 coolings ≈ +7 min |
| native typing | ~2.7 min per run | + Q2: ~+1.5 min per run for 29 more cold processes (estimate), warm +70 keystrokes negligible |
| `gates/PB.toml` | — | ~60-90 min, dominated by the L0 regression |

Every chained `mise run gate -- L0` of later gates hits the cache (same merge-base).

## 12. Gate

Draft: `gates/PB.toml` (scratchpad until frozen). Criteria: `spec`, `model`, `rule`, `reference-build`, `driver`
(exclusive), `perf-start` (exclusive), `real-server` (exclusive), `native-invalid`, `native-budget` (only if Q2 is
accepted), `selftest`, `regression` (`test:py`, `internal/nativeperf`, `mise run gate -- L0`), `simplify-report`.
Every Python criterion requires each named test to be listed by `unittest -v`, nothing skipped, exit 0; every Go
criterion requires `--- PASS: <name>`; the Vitest criterion reads the JSON reporter.

## 13. Risks

- R1 — a systematic difference between the two sides that is not the change (binary size, path, page cache of the
  freshly built B). Mitigated by equal-length paths and environments, `-trimpath` on both, copying both binaries
  into the same directory tree, warm-up pairs; detected by the A/A identity runs, which must pass.
- R2 — a noisier machine than `heavy`: criteria end invalid (fail closed, labelled). If it happens in practice,
  add the sequential extension of section 5 (new spec, new gate), never a wider margin.
- R3 — a change that only adds rare slow starts moves the median little; the absolute budget bounds the tail only
  coarsely (the maximum of 80). A tail-ratio rule needs several hundred pairs; follow-up if a real case appears.
- R4 — the reference commit's build breaks with today's toolchain (old lockfile, removed sidecar): fail closed with
  the build log; the next merge moves the merge-base.
- R5 — clean tree on main: reference equals change, the criterion is an A/A check and cannot see a regression
  already committed. Gates run before the commit (definition of done, step 8), so the change is uncommitted.
- R6 — the carry-over environment is modelled, not measured. The `start-pairs-*.json` slots record order, so step 2
  can estimate it from the first real runs and the model's `carry` environment is checked against it.

## 14. Follow-ups (not in PB)

- F1 — `regression.py --bench`: align its rule (5 %, p < 0.05, confirmation) with the same non-inferiority bound
  per benchmark, with a family-wise level over the benchmarks.
- F2 — a paired native typing comparison (reference and change windows interleaved run by run).

## 15. Open questions for the user (product choices)

- Q1 — budget for the startup proxy (server build, usable editor in Chromium): 1.5 s ("last project editable",
  proposed; 0 false failures in the model) or 1 s ("window painted"; 29 % false failures on this host with 80
  samples, because 2 of 270 starts already exceed 990 ms).
- Q2 — native typing: certify the p95 budgets with 95 % confidence (warm 30 -> 100 samples, cold 30 -> 59 fresh
  processes, about +1.5 min per run), or keep 30 and 30 and report the confidence actually reached (about 78 %).

## 16. ADR 0038 (draft text)

```markdown
# 0038 — Paired, interleaved performance criteria

- Status: proposed (2026-11-11)
- Deciders: the user (2026-11-11: fix the performance method at the root), architect
- Plan: docs/plans/2026-11-11-pb-paired-perf.md; model: scripts/perf/pbmodel.py

## Context

PERF-START compared a fresh p95 of 30 starts with an absolute limit archived on 2026-11-02. On 2026-11-11 the
unchanged HEAD measured p95 395-667 ms against a 361 ms limit: the machine's level moved by 29 % between sessions
and one outlier moves a p95 of 30 by 75 %. The criterion judged the machine, not the change.

## Decision

1. A regression criterion compares the change with its committed reference (merge-base with main), both built on
   this machine by the same build function and measured in the same session, pair by pair, in an order drawn at
   random for each pair (randomised interleaved trials). The reference build is cached by commit and toolchain.
2. The budget is an effect size: the change passes iff the exact one-sided 99.9 % Wilcoxon signed-rank upper bound
   (Hodges-Lehmann, Walsh averages) of the median ratio change/reference is below 1.10. Randomisation makes the
   bound exact under any drift. A run whose interval is wider than ±4 % is invalid; with that limit a regression
   verdict always means a slowdown significant at 1e-3. Sizes: 80 judged pairs after 2 warm-up pairs.
3. An absolute budget on a quantile is certified by its distribution-free order-statistic upper bound at 95 %
   confidence, never by a point quantile of few samples.
4. An environmental fault (port busy, focus lost, input delivery lost, imprecise run) makes a run invalid; an
   invalid run is retried up to three times after cooling, then the criterion fails as invalid. A valid verdict is
   never retried.

## Consequences

- Machine drift and outliers no longer fail unrelated work: 0 false regressions in 3000 simulated runs in each of
  seven environments fitted to and beyond this host's samples; a true +10 % fails with probability >= 0.995.
- Changes up to +5 % pass, +10 % fail; between them the verdict depends on the noise (indifference zone).
- PERF-START costs ~2 more minutes per L0 run, plus one reference build per merge-base.
- The archived absolute limit of 2026-11-02 is kept as history and no longer gates.

## Alternatives considered

Absolute archived baseline (incumbent; 55 % false regressions in the model); significance tests without a margin
(Pinpoint KS/Mann-Whitney, Perfherder Mann-Whitney with Cliff's delta, benchstat): they answer "is there a
difference", not "is it within budget"; p95 ratio (22 % false regressions); mean ratio with a t or bootstrap bound
(Kalibera & Jones; outlier-driven, 58 % false regressions under heavy tails); point median (power 0.49); 95 % bound
(power 0.95); sequential blocks (no decision under drift); fixed order (power 0.01 with a carry-over); duet
benchmarking (parallel starts contend and change the quantity).
```

## 17. Sources

- Abedi & Brecht, "Conducting Repeatable Experiments in Highly Variable Cloud Computing Environments", ICPE 2017 —
  https://research.spec.org/icpe_proceedings/2017/proceedings/p287.pdf
- Kalibera & Jones, "Rigorous Benchmarking in Reasonable Time", ISMM 2013 — https://kar.kent.ac.uk/id/document/3127886 ;
  "Quantifying Performance Changes with Effect Size Confidence Intervals" — https://arxiv.org/abs/2007.10899
- Chromium Pinpoint comparison (KS and Mann-Whitney, low/high thresholds, `Unknown`) —
  https://skia.googlesource.com/buildbot/+/main/pinpoint/go/compare/compare.go
- Mozilla PerfCompare (Mann-Whitney default, Cliff's delta) —
  https://raw.githubusercontent.com/mozilla-firefox/firefox/main/testing/perfdocs/generated/perfcompare.md
- benchstat (Mann-Whitney U, interleave, at least 10 runs, multiple testing) —
  https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
- Bulej et al., "Duet Benchmarking: Improving Measurement Accuracy in the Cloud", ICPE 2020 — https://arxiv.org/abs/2001.05811
- tango-bench (paired benchmarking) — https://docs.rs/crate/tango-bench/0.7.2/source/README.md
- Mytkowicz, Diwan, Hauswirth & Sweeney, "Producing Wrong Data Without Doing Anything Obviously Wrong!", ASPLOS 2009.
- Hollander, Wolfe & Chicken, Nonparametric Statistical Methods, 3rd ed. (signed-rank bounds, Hodges-Lehmann, § 3.3);
  David & Nagaraja, Order Statistics, 3rd ed. (distribution-free quantile bounds, § 7.1); Schuirmann, "A comparison
  of the two one-sided tests procedure…", J. Pharmacokinet. Biopharm. 1987 (non-inferiority / TOST).

## Decisions of the user (2026-11-11)

- Q2 (native typing): yes, certify the p95 at 95 % (100 warm samples, 59 cold fresh processes).
- Q1 (absolute startup budget), after the study `docs/plans/2026-11-11-startup-speed.md`: « accelerate then tighten ». Now: median ≤ 350 ms and p95 ≤ 400 ms (the Chromium proxy, to a usable editor), judged on an order-statistic bound; block ST1 builds the three gains (preload hints, Go handshake overlapped with Automerge init, non-editor code deferred), then the budget is tightened to what ST1 measures (target ≈ 300 ms median, 350 ms p95). No native budget until a native measurement exists.
