# Block RG — a Go benchmark regression guard robust to code layout (2026-11-12)

Status: step 1 (specification), draft for review. Rigour: **complete** (docs/engineering-principles.md § 0): the
block changes the machinery that judges every later block's Go hot paths (gate L0 criterion `regression-guard`,
chained into every later gate), and a wrong rule either blocks unrelated work or passes real regressions. Proofs: the
statistical model with mutant rules (`scripts/perf/rgmodel.py`, section 6), the rule and layout tests (section 9), the
self-test with a real layout mutant (section 8), the frozen gate `gates/RG.toml`, and one review by another model
family before the gate is frozen (Codex; its first review is folded in, section 4.6). ADR: amendment "RG" of ADR 0038
(closes its follow-up F1; no new number).

## 1. Need and evidence

The guard (`scripts/perf/regression.py --bench`) builds one test binary per side (merge-base, change), runs 20
alternated processes per side, and fails when benchstat reports a worsening > 5 % with p < 0.05 that a second
session reproduces. Measured on 2026-11-12 (archive `docs/perf/2026-11-12-layout/`, section 10):

- `internal/transport/mux` BenchmarkSend/256: +6.3 % (p = 0.000, n = 20) between HEAD and block C5S. The machine code
  of mux is byte-identical (`base.Send.asm` = `c5s.Send.asm`). C5S removed one method of `internal/transport`, which
  is linked just before mux: mux's hot functions moved from offset 32 to 0 of a 64-byte block. 224 bytes of unused
  code restore the base speed (+7 % padded against unpadded).
- Reproduced with this block's own tool: one unused 32-byte function placed in front of mux's code (same source
  otherwise) changes Send/256 by -5.4 % (12 processes per variant, random order).
- With 64-byte function alignment on both sides (`-ldflags=-funcalign=64`) the C5S difference falls to +0.95 %
  (inside the noise; n = 12), and the faster of the two phases is obtained.
- Under 64-byte alignment, 12 random page offsets of mux (64-byte steps) show no detectable between-layout variance
  (spread of layout means 1.9-2.0 % against 2.0-2.7 % expected from process noise alone; upper estimate 2.2 %).
  Under the default 32-byte alignment, 12 random offsets show 1.7-1.8 % between-layout spread: the phase effect.
- Process noise of Send/256 (one sample per process, 1 s benchtime): 4.2 % sd pooled over 300 processes. Shorter
  benchtimes are much noisier (100 ms: 22.7 %, 250 ms: 19.0 %): the cost cannot be cut by shortening runs.
- The last L0 guard session gives a per-benchmark process sd of 1.2 % to 14 % (30 % for mux Send/1048576).

- The incumbent guard, rerun on the current tree the same day (`--bench`, scratch output): mux Send/256 flagged
  again in the first session (+5.3 %, p = 0.000) and not confirmed by the second, so the run passed. The verdict on
  this layout case is decided by noise around the 5 % point threshold, not by the code.

A layout effect is deterministic for a pair of binaries, so the confirmation session reproduces it by construction:
the incumbent flags this pure layout case in 73 % of simulated gate runs (model, section 6), while its power against
a true +5 % slowdown is 0.26 (it requires the point estimate above 5 %). It judges the link layout, not the change.
Such shifts recur on any unrelated edit of a package linked before a hot one.

## 2. State of the art

- Mytkowicz, Diwan, Hauswirth, Sweeney, "Producing wrong data without doing anything obviously wrong!" (ASPLOS 2009,
  https://sape.inf.usi.ch/publications/asplos09): measurement bias from innocuous setup (link order, environment
  size) is significant and commonplace; remedies are setup randomization (avoid) and causal analysis (detect).
- Curtsinger, Berger, "STABILIZER: statistically sound performance evaluation" (ASPLOS 2013,
  https://people.cs.umass.edu/~emery/pubs/stabilizer-asplos13.pdf): re-randomize code, stack and heap placement
  during the run so that executions sample many layouts and the results support ordinary tests; the effect of -O3
  over -O2 on SPEC CPU2006 becomes indistinguishable from noise. The tool is an LLVM pass, unmaintained, and only
  works on old LLVM (https://github.com/parsa/stabilizer): not usable for Go.
- Kalibera, Jones, "Rigorous benchmarking in reasonable time" (ISMM 2013, https://kar.kent.ac.uk/id/document/3127886):
  repeat at the level where the variance is (builds, executions, iterations), sized by a cost model. Here the build
  level (layout) and the process level carry the variance; the iteration level is averaged by `testing.B`.
- Go: `cmd/link -funcalign N` sets the function alignment (introduced 2025-04, in Go 1.27,
  https://pkg.go.dev/cmd/link@go1.27.2); the default is `thearch.Funcalign` (amd64 32, arm64 16:
  `src/cmd/link/internal/amd64/l.go`, `arm64/l.go` of go1.27.4). The x86 assembler pads jumps so they do not cross
  32-byte boundaries but does not align loops (`maxLoopPad = 0`, `src/cmd/internal/obj/x86/asm6.go`); hot-block
  alignment exists only under PGO (`-d=alignhot`, `ssagen/ssa.go`). No linker flag shuffles the link order; `-T`
  moves text by whole pages only (a sub-page `-T` produces an unloadable binary, tried). The Go team treats such
  regressions as layout noise case by case (golang/go#47074, #59319, #37429, #67857), without tooling.
- LLVM offers `-align-all-functions`, `-align-all-nofallthru-blocks`; Bakhvalov (easyperf.net, 2018) reports a 10 %
  slowdown from removing an unrelated function with identical assembly, the C5S phenomenon.
- rustc-perf judges instruction counts (`instructions:u`) rather than wall time; CodSpeed simulates with Valgrind:
  both remove most layout and noise effects, and with them the regressions that matter here (cache, memory,
  contention); and no `perf` binary is installed on this host.
- ADR 0038 (Abedi and Brecht 2017 randomized interleaving; exact signed-rank bound) for the pairing and the bound.

Critique: canonical alignment alone removes the measured class (function phase within a cache line) but leaves every
coarser effect (page offset, inter-function aliasing) deterministic; randomization alone keeps the phase effect as
variance (bimodal, ±3 %), which costs power; instruction counts are blind to the regressions that matter. None of the
sources sizes the sample against a stated power.

## 3. Method (what improves on the sources)

Two arms, both measured in one session, pairs in a random order (ADR 0038), one sample per process:

1. **Robust arm (layout-randomized).** Canonicalize, then randomize what is left:
   - every binary is built with `-ldflags=-funcalign=64` (one cache line): the phase class is removed
     deterministically, at zero run cost, for all code including dependencies and the runtime;
   - every process runs its own binary: a fresh random layout is drawn per process, independently for the two sides,
     through a `go test -c -overlay` that adds padding files to the benchmarked package (none written into the
     tree): one pad file sorting before the first Go file and one after each Go file, each holding k empty
     `//go:noinline` functions (k uniform in 0..63; one function = exactly one 64-byte slot under `-funcalign=64`),
     so the package's code moves over the 4 KiB page and its files move relative to each other. Only the benchmarked
     package recompiles per layout.
   - Minimal mechanism (review round 2): the pad files have no import, no `init`, no variable, no table. They are kept
     reachable by one internal test file of the overlay, `TestInkwellLayoutPadKeep`, which calls each pad and is
     never run (`-test.run=^$`); it is present in every robust binary, k = 0 included, so all robust layouts share the
     same data and test table and differ only in text. A first design (a function table called from an `init` behind
     `os.Getenv`) was dropped: it added data, an init task and an import that vary with k.
   - Measured on mux (k = 0, 1, 37, 63 against no overlay, `padcheck2.sh`): `-gcflags=-m=2` inlining and escape
     decisions of every mux function identical (the only lines that differ are `_testmain.go` line numbers, from the
     one extra test entry); normalized assembly of every mux function (addresses and encoded displacements removed)
     identical, 3851 lines; mux data symbols identical; `(*sub).Send` moved by exactly 64 k bytes (0x653340 +0x40,
     +0x940, +0xfc0). No type is declared by the pads, so no type layout can change. These checks run on every build
     (`verify_layout`, section 9) against the first layout of the session, so a toolchain that changes a decision
     fails closed.
   - Under the null hypothesis "same code up to layout", the two sides' layouts are independent draws from one
     distribution, so each pair difference is symmetric about 0: the signed-rank bound is exact (STABILIZER's
     premise, made exact by independence and random order instead of normality). Section 4.0 states what holds
     beyond that null.
2. **Production arm (unperturbed).** The test binaries built with the shipping build vector (section 4.7: default
   alignment, no pads, the release flags), so a loss that users would actually get cannot be averaged away. Measured
   on P pairs; beyond the layout envelope (section 4.4) it is an `investigate` verdict, not a code regression.

Verdict per benchmark and time unit: **regression** if the robust arm flags; **investigate** if only the production arm
flags (fails the gate; section 4.4); **certified** if the robust arm's certification bound is below the budget and
the production arm does not flag; otherwise **inconclusive**, which fails the gate (never a pass). allocs/op and B/op do not depend on layout: they keep the incumbent rule (benchstat, > 5 % and
p < 0.05 on all processes of the robust arm). Throughput units (B/s) are the inverse of sec/op on the same
processes: judged by the same bound with the sign reversed.

## 4. Decision rule and predeclared constants

All constants live in `regression.py`, are equal to the model's (`test_constants_match_code`), and are declared
before any measurement; the comparison count is fixed from `-test.list` before the first process.

4.0 **Assumptions and what each guarantee rests on.** Pairs are independent (one process per sample, fresh binaries,
random order per pair: ADR 0038). The estimand of the regression test is the Hodges-Lehmann pseudo-median of the pair
log-ratio (the median of the Walsh averages), not a location shift of a common distribution.
- Regression flag (signed-rank lower bound): **exact** level when the two sides have the same distribution up to
  layout (the pure-layout null, the case that must not flag), whatever that distribution is. When the change alters
  the shape of its distribution (noise, layout sensitivity, tails) the signed-rank bound is exact only if the pair
  difference stays symmetric (true when both sides are symmetric about one centre, for any scales); otherwise it is a
  consistent test of the pseudo-median, with a level measured by the model, not proved: environments `b-noisy`
  (change side noise x1.5), `b-layout` (change side twice as layout-sensitive), `b-quiet` (x0.7) and `b-stalls`
  (10 % of the change's processes +15 %, a skewed loss whose pair median is about 0 and whose pseudo-median is
  positive: a real loss in the tail, which the guard is meant to flag). The prototype `rgsim2.py` is not the
  common-residual, additive model of the first round: each side draws its own scale, layout sensitivity and tail.
- Certification (the only passing outcome): an **exact sign-test** upper bound of the median pair log-ratio (an order
  statistic), valid for any continuous distribution of independent pairs, symmetric or not, and under drift (each
  pair's own median); so "certified" never rests on symmetry.

4.1 **Effect threshold.** Budget B = 5 % (the archived `delta_percent`, unchanged). Margin m0 = 1 %: a true slowdown of
1 % or less is flagged with probability at most alpha. Between 1 % and 5 % the verdict depends on the noise
(indifference zone); at 5 % and above the robust arm flags with power >= 0.95 (model).

4.2 **Sample budget.** Two looks, both predeclared:
- stage 1: N1 = 45 robust pairs plus P = 20 production pairs, all benchmarks, randomly interleaved;
- a benchmark is certified at stage 1 if its sign-test upper bound at level alpha_c / 2 is below log(1.05) and the
  production arm does not flag. Certification can happen at either look, so its level is spent over the two looks
  (Bonferroni, alpha_c / 2 each): a true >= 5 % slowdown is certified with probability <= alpha_c in total (the
  union of the two looks), never 2 alpha_c; certification only stops sampling, so it can only lower the regression
  rate;
- stage 2: 45 more robust pairs, fresh layouts, running only the uncertified benchmarks (`-test.bench` regex built
  from their names); final robust bounds on all 90 pairs. Regression is judged only at 90 pairs: one look, so the
  regression level stays alpha unconditionally (the event "uncertified at 45 and flagged at 90" is contained in
  "flagged at 90").
- never extended further, never rerun (a rerun until pass is optional stopping).

4.3 **Level and multiple comparisons.** Per comparison, one-sided alpha_c = min(1e-3, 0.02 / C), C = the number of
enforced time comparisons of the run (one per benchmark: B/s is sec/op inverted on the same pairs, not a second test).
C_max = 40 (today 16); a run with C > C_max fails closed before measuring ("comparison budget exceeded": a new spec,
not a silent smaller alpha). Power and the per-benchmark budgets are sized at C_max (alpha_c = 5e-4), the worst
supported case. Family-wise: regression <= C alpha_c <= 0.02, certification of a true >= budget loss <= 0.02,
production `investigate` <= C alpha_c <= 0.02, each at its worst case; at no change far lower (model). Every model
rate is reported with its 95 % Clopper-Pearson interval: 0 events in 20000 runs means "<= 1.5e-4 at 95 %", not 0. The confirmation session is removed: with a randomized
layout the second session no longer protects against anything a single larger session does not.

4.4 **Production envelope.** The production arm flags iff its lower bound exceeds log(1 + E). 7 % is one observed
maximum (mux Send/256), not an envelope. E is therefore measured in step 2a for every hot benchmark: 16 default-
alignment layouts (pads in the benchmarked package and, separately, in a dependency linked before it) x 4 processes;
E = the largest layout range over all benchmarks, its 95 % upper bound, rounded up to the percent, never below 8 %;
written to the step 2a budgets file under the same guard. The trade-off, explicitly: a larger E means fewer false
`investigate` verdicts on unrelated edits, and lets a layout-specific production loss up to E pass; a smaller E
the reverse. A production flag is never attributed to the source by itself: the robust arm decides whether the code
regressed. Production-only = `investigate`: the gate fails with both arms' estimates, the realized hot-function
offsets and the layout ranges; it is cleared only by a reviewed entry naming the benchmark and the evidence (same
mechanism as `gates/guard-overrides.json`), or by a change that removes the loss.

4.5 **Precision.** Inconclusive = the robust 90-pair interval straddles both the margin and the budget. It fails the
gate with the benchmark's achieved minimum detectable effect, never passes. A benchmark too noisy for 5 % (section 1:
up to 30 % process sd) is a defect of the benchmark, fixed at its root (step 2a) or, if inherent, given a predeclared
per-benchmark budget in `docs/perf/<date>-rg-budgets/thresholds.json` (step 2a, with the A/A run it derives from,
sealed by its own SHA256SUMS) by the predeclared rule: budget = the smallest
of 5, 10, 15, 20 % that its archived A/A precision (step 2a, 90 pairs) detects with power >= 0.95; beyond 20 %, no
budget: the benchmark must be fixed. The file's numbers are guarded by `gate-guard.py` (`docs/perf/*/thresholds.json`:
numeric changes need an override); `test_budgets_follow_predeclared_rule` recomputes them from the archived A/A run.

4.7 **Build vectors and toolchain.** The production arm uses the shipping vector, one function shared with the
release build (`scripts/perf/refbuild.py`'s Go vector: tags, `-trimpath`, `-buildvcs=false`, GOAMD64 and ldflags as
`.github/workflows/release.yml`), asserted equal by a test; the robust arm is that vector plus exactly
`-ldflags=-funcalign=64` and the overlay, asserted as the only difference. Releases keep the default alignment
(decision: `-funcalign=64` is a measurement device, not a product change; changing it is a separate, measured block).
Every build runs the pinned toolchain through `refbuild.go_env` (commit f92d6c6), and `go version` is checked
against the pin before the first build (section 9).

4.6 **Codex review (folded in).** Inference rests on independent layout samples (N fresh builds per side, never
repeated timings of one layout: the model's mutant M2 shows pseudo-replication inflates false regressions);
threshold, sample budget and multiple comparisons are predeclared (4.1-4.3); insufficient precision is inconclusive,
never a pass (4.5); the unperturbed production binary keeps its own bound (4.4).

### Review round 2: the rule under unequal sides, two-look spending, alpha at C_max (`rgsim2.py`)

Each side draws its own noise scale, layout sensitivity and tail; certification by the exact sign test at
alpha_c / 2 per look; rates with 95 % Clopper-Pearson intervals (20000 runs for null rows, 4000-8000 otherwise;
`sim3.log`, `sim4.log`). Columns: regression, certified, inconclusive.

| Environment (stages 40 + 40 unless stated) | regression | certified | inconclusive |
|---|---|---|---|
| A/A, alpha 1e-3 | 0 [0, 2e-4] | 0.9949 | 0.0052 |
| A/A, alpha 5e-4 (C_max) | 0 [0, 2e-4] | 0.9940 | 0.0060 |
| change noisier x1.5, no shift | 0 [0, 2e-4] | 0.9272 | 0.0728 |
| change twice as layout-sensitive (sigma_l 2.2 %) | 0 [0, 2e-4] | 0.8410 | 0.1590 |
| change: 10 % of processes +15 % (skewed) | 0.0015 [6e-4, 3.3e-3] | 0.8195 | 0.1790 |
| true +1 % (margin) | 8e-4 [5e-4, 1.4e-3] | 0.9254 | 0.0738 |
| true +1 %, change noisier x1.5 | 1.2e-3 [8e-4, 1.8e-3] | 0.7118 | 0.2869 |
| true +5 % | 0.9962 | 0 [0, 9e-4] | 0.0037 |
| true +5 %, change noisier x1.5 | 0.9385 | 0 [0, 9e-4] | 0.0615 |
| true +5 %, alpha 5e-4, sigma_l 2.2 % (40 + 40) | 0.9560 [0.949, 0.962] | 3e-4 [0, 1.4e-3] | 0.0437 |
| same, **45 + 45 (chosen)** | 0.9818 [0.979, 0.985] | 5e-4 [1e-4, 1.3e-3] | 0.0177 |
| A/A, alpha 5e-4, 45 + 45 | 0 [0, 5e-4] | 0.9976 | 0.0024 |

Readings, and the qualified guarantees:
- No false regression when the change only alters noise or layout sensitivity (0 in 20000, <= 2e-4).
- At the margin with unequal scales the signed-rank level is not exact: 1.2e-3 measured (95 % upper 1.8e-3) for a
  nominal 1e-3. The guarantee is stated as measured, and the family-wise budget of section 4.3 is read with this
  factor: <= 0.036 in the worst case (C_max comparisons all exactly at the margin with a change in noise).
- A skewed loss (10 % of runs 15 % slower, mean +1.4 %) is flagged in 0.15 % of runs and certified in 82 %: the
  estimand is the pseudo-median and the certification bound the median; a tail loss below the budget in mean is not
  a 5 % regression. Tail budgets stay with ADR 0038's order statistics, not this guard.
- A true +5 % is certified (the only pass) in <= 1.4e-3 (95 % upper) of runs in every environment; sign-test
  certification costs 0.5 % inconclusive at no change (signed-rank certification: 0.01 %, but it rests on symmetry).
- Sizing at C_max with the largest measured layout variance: 40 + 40 gives power 0.956 [0.949, 0.962], too close;
  45 + 45 gives 0.982 [0.979, 0.985]: chosen.

## 5. Choice of N, alpha, m0 (model, `scripts/perf/rgmodel.py`)

Noise: per-process residuals of Send/256 (log ns minus the variant median), 300 processes, sd 4.2 %, pair difference
sd 5.9 %, bootstrapped; layout: canonical arm with between-layout sd 0 (fit) or 2.2 % (measured upper bound); stress:
noise x1.5. Single-look power at a true +5 % (4000 runs each):

| N pairs | alpha 1e-3, m0 1 % | same, sigma_l 2.2 % | alpha 1e-3, m0 0.5 % | alpha 5e-3, m0 1 % |
|---|---|---|---|---|
| 40 | 0.803 | 0.646 | 0.904 | 0.922 |
| 60 | 0.965 | 0.893 | 0.989 | 0.990 |
| 64 | 0.976 | 0.911 | 0.996 | 0.992 |
| 70 | 0.985 | 0.944 | 0.997 | 0.998 |
| 80 | 0.996 | 0.974 | 1.000 | 1.000 |

N = 80, alpha 1e-3, m0 = 1 % was the first-round choice; round 2 (above: alpha spent over looks, sign-test
certification, sizing at C_max) moves it to 45 + 45 pairs; incumbent: power 0.26 at +5 %, 0.999 at +10 %, false regression 0.73 on the C5S layout case.

Full rule (two looks, both arms, 20000 runs for the null rows, 4000 otherwise; columns: regression, certified,
inconclusive; `rgsim.py`, RG_STAGED=1):

| Environment | regression | certified | inconclusive |
|---|---|---|---|
| A/A, fit | 0.0000 | 1.0000 | 0.0000 |
| C5S layout case (production +6.3 %) | 0.0000 | 1.0000 | 0.0001 |
| C5S, sigma_l 2.2 % | 0.0001 | 0.9995 | 0.0004 |
| C5S, noise x1.5 | 0.0001 | 0.9618 | 0.0382 |
| true +1 % (the margin) | 0.0005 | 0.9958 | 0.0037 |
| true +5 % | 0.9942 | 0.0018 | 0.0040 |
| true +5 %, sigma_l 2.2 % | 0.9790 | 0.0013 | 0.0198 |
| true +5 %, noise x1.5 | 0.7853 | 0.0013 | 0.2135 |
| true +5 % on a favourable production layout | 0.9948 | 0.0015 | 0.0037 |
| production-only +15 % (layout-specific) | 0.8263 | (robust arm certifies) | 0 |
| A/A, noise x2.3 (pin2, load 6) | 0.0000 | 0.5360 | 0.4640 |
| true +5 %, noise x2.3 | 0.2938 | 0.0013 | 0.7050 |

The pure layout case never flags (<= 1e-4 in 20000 runs); a true +5 % is flagged with power 0.994 (0.979 with layout
variance) and, because inconclusive fails, is passed in at most 0.2 % of runs in every environment, the noisiest
included. On a loaded host (noise x2.3, measured at load 6 in `pin2.jsonl`) a no-change run is inconclusive in 46 %
of runs: it fails as imprecise, never passes, and the report shows the measured noise. P = 20 production pairs flag a
production-only +15 % in 83 % of runs (about all from +20 %); the arm exists for large layout-specific losses.

## 6. Formal model and mutants (`scripts/perf/rgmodel.py`, `rgmodel_test.py`)

The model draws sessions from the archived residuals (refit from `docs/perf/2026-11-12-layout/`, never typed in) and
the layout environments: `fit` (canonical, sigma_l 0), `page` (sigma_l 2.2 %), `phase` (default alignment
randomized: two-point ±3.15 %), `c5s` (production arm: fixed layouts, change side in the slow phase, +6.3 %),
`envelope` (production +7 %), `drift` (linear drift 5 % over the session, random order), `stress` (noise x1.5),
`loaded` (noise x2.3, as measured at load 6). It
evaluates the rule exactly as regression.py does (it imports the rule, as pbmodel imports paired).

Every rate is asserted on its 95 % Clopper-Pearson bound, never on the point estimate (a requirement "<= x" holds only
if the upper bound is <= x; ">= y" only if the lower bound is >= y), at alpha_c of C_max = 40 unless stated. Added
environments: `b-noisy`, `b-quiet`, `b-layout`, `b-stalls` (section 4.0). Requirements (each a test): false
regression <= alpha_c per comparison in every null environment including the unequal-shape ones, <= 2 alpha_c at the
margin with unequal shapes (measured 1.2e-3 for 1e-3) and <= 1e-3 at no
change in `c5s` and `envelope` (20000 runs); flag rate at the margin <= alpha_c; power >= 0.95 at +5 % in `fit`
and `page`; a true +5 % certified (the only passing outcome) in <= 0.005 of runs in every environment, `stress` and
`loaded` (noise x2.3) included; a loaded host is inconclusive, never certified, at no change more often than it
flags; the production arm flags a production-only +20 % with probability >= 0.95; power never below the incumbent's
at +5 %, +10 %, +20 %; the incumbent flags `c5s` (> 0.5).

Mutant rules, each rejected by at least one requirement:
- M1 fixed layout per side (the incumbent design with the new bound): false regression on `c5s`;
- M2 pseudo-replication: K = 4 layouts per side reused, pairs treated as independent: false regression > alpha under
  `page`;
- M3 point estimate above 5 % instead of a bound: power about 0.5 at +5 %;
- M4 fixed order (reference first) under drift and carry-over: false regression under `drift`;
- M5 default alignment, randomized (no canonicalization): power < 0.95 under `phase` at N = 80;
- M6 regression judged at both looks without alpha spending: level above alpha_c;
- M7 no production arm: a production-only +15 % (`envelope` + 8 %) passes;
- M8 inconclusive counted as pass: a benchmark with noise x2 passes a true +5 %;
- M9 certification at alpha_c on both looks (no spending): a true +5 % certified above alpha_c;
- M10 signed-rank certification (symmetry assumed) under `b-stalls`: the skewed loss certified above alpha_c.

End-to-end falsification of the harness (section 9, `test_falsify_*`, real builds, short benchmarks): each injected
fault must make the session fail closed, never produce a verdict:
- F1 one binary reused for several processes (identity check: every process records its binary's SHA-256, and the
  verifier requires one distinct binary per process and side);
- F2 overlay ignored (pads absent: realized offsets equal across binaries, or `verify_layout` finds no pad);
- F3 production flags altered (vector differs from the release vector);
- F4 production verdict dropped from the judgement (a production-only +20 % fixture must give `investigate`);
- F5 inconclusive treated as pass (a noise x3 fixture must fail);
- F6 robust flags missing `-funcalign=64` (alignment check);
- F7 a side's pad vectors equal to the other's (independence check on the recorded plan).
The session report records, per process: side, arm, pair, order, binary path and SHA-256, pad vector, realized
offsets of the benchmarked package's functions; the verdict is computed only after the verifier recomputes all of
them from the files.

## 7. Cost (this host, the four HOT_PATHS packages)

Per process (1 s benchtime, all benchmarks of the package): blob 3.7 s, frame 10 s, mux 4.0 s, app 2.4 s (L0 log of
2026-11-12 05:05-05:18). Self-test (run first by L0's `regression-guard`): fixtures about 0.2 s per process (x1.2,
gain, code +5 %: 3 x 120 processes, about 1.5 min with builds); mux layout mutants (package and dependency): control
2 x 40 Send/256 processes, then the guard on Send/256 only (120 + 80 processes), about 1.5 s each, twice: about 12
min; total self-test about 14 min (the incumbent's: 5 min, measured 302 s). Stage 1: 130 processes per package, 43 min for the four; stage 2: 90 processes of the
uncertified benchmarks only (about 1.3 s per benchmark per process). Builds: 182 test binaries per package (only the
benchmarked package and the test main recompile; about 0.7 s each for mux), built before the first measurement.
Incumbent: about 13 min without a flag, 26 min with confirmations. Two savings, both without weakening anything:
packages whose dependency closure is unchanged are informational today (they cannot fail the gate) and are no longer
measured (listed as skipped); builds run in parallel before measuring. A block that touches one hot package pays one
package. With the self-test (about 14 min), an L0 `regression-guard` run on a block that touches the four hot packages
takes about 65-70 min, against about 18 min today (5 min self-test + 13 min); one hot package: about 25 min. The price
buys power 0.98 at +5 % (at C_max) instead of 0.26, and no pass of a true +5 % beyond 0.14 %.

## 8. Self-test (`regression.py --self-test`, real processes)

Kept: the environment fixture (spin 10 ms, slow side 12 ms: x1.2) is flagged in sec/op and B/s; its reverse (a gain)
is certified. Added:
- code slowdown: two fixture sources, the change spinning 5 % longer (10.5 ms): flagged;
- dependency layout mutant: one unused 32-byte function in `internal/transport` (linked before mux, the exact C5S
  mechanism), mux unchanged: must not give `regression`;
- pure layout mutant: internal/transport/mux at default alignment, the change side with one unused 32-byte function
  in front of mux (overlay, the C5S mechanism). Control first: under one fixed layout per side, the incumbent-style
  comparison of Send/256 must show |delta| >= 3 % at p < 1e-3, or the self-test fails as "layout mutant ineffective on
  this CPU" (a self-test that cannot fail proves nothing); then the guard must not flag it.
- success line: `regression-guard self-test passed: layout-randomized (x1.2 flagged, code +5 % flagged, gain passed,
  layout mutants (package, dependency) effective and passed)`.

## 9. Blueprint for go-dev (step 2)

regression.py (rule shared with the model; bound imported from `paired.py`: `hl_bounds`, `critical`, `orders`,
`fresh_seed`; no copy). Every `go` call runs with the pinned toolchain through `refbuild.go_env` / `tree_go_env` (commit f92d6c6: the
tree's mise pin, its GOROOT, its bin first on PATH, `GOTOOLCHAIN=local`; reused, not copied). Measured 2026-11-12:
`GOROOT=/home/mathieu/go/go1.27.3` exported by the session made mise's go1.27.4 call a go1.27.3 compiler (`compile:
version "go1.27.3" does not match go tool version "go1.27.4"`); `go version` is checked against the pin before the
first build:
1. `bench_list(binary)`: `-test.list` names; C and alpha_c fixed before measuring.
2. `pad_source(package, k, tag)`, `keep_source(package, pads)` and `layout_overlay(package_dir, go_files, ks)`: the
   overlay JSON (pad file before the first Go file and after each, plus the keep test file); names that sort in
   place, checked.
3. `build_layout(tree, package, ks, canonical)`: `go test -c -overlay … [-ldflags=-funcalign=64]`; then
   `verify_layout(binary, package, ks)` reads `go tool nm -n -size` and fails closed unless every pad occupies one
   64-byte slot, pad 0 precedes every function of the package, every function of the module is 64-byte aligned, and
   the normalized assembly, `-m=2` report (without `_testmain.go` lines) and data symbols of the package equal the
   session's first robust layout.
4. `plan(seed, n1, p)`: per pair, side order (fair coin), arm (robust or production, shuffled), and the pad vectors of
   both sides drawn independently; replayable from the recorded seed.
5. `run_pairs(...)`: one process per sample, `-test.count=1 -test.benchmem`, raw lines kept; `parse_bench(text)`.
6. `judge(...)`: the verdicts of section 4 per benchmark and unit (regression, investigate, certified,
   inconclusive); allocs/op by the incumbent rule; computed only after `verify_session` recomputes every recorded
   binary SHA-256, pad vector and realized offset from the files (falsifications F1-F7).
7. `measure_session`: skip informational packages; stage 1, stage 2, `session.json` with seed, pad vectors, realized
   hot-function offsets, bounds, verdicts, achieved MDE, timings.
8. Self-test of section 8. Keep `thresholds.json` assertions (history); the old `significant()` path stays only for
   allocs/op.
Tests: `scripts/perf/regression_test.py` (rule, no process), `scripts/perf/regression_layout_test.py` (real builds of
mux, no measurement), `scripts/perf/regression_falsify_test.py` (F1-F7 on short fixtures), `scripts/perf/rgmodel_test.py` (excluded from `test:py` like `pbmodel_test.py`, run by the
gate). Overrides in `gates/guard-overrides.json` for `regression.py` (gate dependency of L0) and `mise.toml`.

Freeze and re-inclusion (decision 2026-11-12): the four test modules are committed with the frozen gate as failing
skeletons ("not implemented"), so every criterion is red for the right reason. Until RG's implementation they are
gate-only: `mise run test:py` excludes them (mise.toml, the mechanism of `pbmodel_test.py`), so CI stays green. In the
implementation commit, `regression_test.py`, `regression_layout_test.py` and `regression_falsify_test.py` return to
`test:py`; `rgmodel_test.py` stays gate-only, like `pbmodel_test.py` (minutes). Gate criterion `test-py-inclusion`
checks it.

Step 2a (before the A/A-derived budgets): one A/A run of the new guard on every hot package; diagnose every
benchmark whose 80-pair interval is wider than the budget (root cause first: allocation and GC in the 1 MiB paths,
frequency ramps), then write the predeclared budgets.

## 10. Evidence archive (`docs/perf/2026-11-12-layout/`, SHA256SUMS)

mux-diag (r1, r2 runs, `base.Send.asm`, `c5s.Send.asm`, `base.nm`, `c5s.nm`, `ab.sh`), the layout experiment
(`exp.jsonl`: 5 fixed variants x 12 processes, 36 random layouts x 4), the pinning experiments (`pin.jsonl`,
`pin2.jsonl`), the benchtime experiment (`bt.jsonl`), and the scripts that produced them (`genpad.py`,
`exp_build.sh`, `exp_run.py`, `exp_pin.py`, `exp_pin2.py`, `exp_bt.py`, `analyze.py`, `rgsim.py` and `rgsim2.py`:
the model's prototypes, `sim1.log` to `sim4.log`), and the pad side-effect check (`genpad2.py`, `padcheck2.sh`,
`padcheck2.txt`).

## 11. Risks and residuals

- R1 only the benchmarked package is randomized; its dependencies are canonicalized (64-byte alignment) but keep a
  deterministic page-level layout. Measured page-level effect on mux: not detectable (<= 2.2 %). Extending the pads
  to every module package costs a recompilation of the closure per layout (follow-up, measured first).
- R2 data layout (heap, stacks) is not randomized; Go heap addresses are deterministic per binary. No evidence of an
  effect yet; a heap jitter at init is the follow-up if one is measured.
- R3 hybrid CPU (6 performance cores, 8 efficiency cores): pinning to the performance cores (`taskset -c 0-11`) was
  measured and rejected: 36 against 36 processes, variance ratios 0.73 (Send/256), 0.52 (16384), 1.63 (1048576), no
  consistent gain (`pin2.jsonl`; the 16-process run `pin.jsonl` gave 0.51, not significant either).
- R4 the pad placement relies on cmd/compile emitting functions in file order and cmd/link in package order; checked
  on every build (fail closed), so a toolchain change cannot silently disable the randomization.
- R5 the production arm's 8 % envelope is fitted to one CPU; a new host re-measures it (new spec and gate).
- R6 cost (section 7); a sequential design with more looks would cut it further, at the price of an alpha spending
  function (follow-up).
