# /simplify — TC1

Diff : c560c9a..the block's tree (main at the frozen gate, then this block's changes): `protocol/trust/fold.go` (two counters), the two rewritten
tests, `foldcost_test.go`, `foldcost_counts_test.go`, six entries of `scripts/netsim/mutants.toml`, spec §4.9 and
`doc.go` (one informative sentence each). Reviewer: the integrating session (Opus 5.5), after a blind review of the
specification (Fable, F1–F6 applied before the freeze).

## Applied

- The production change is the smallest the plan allows: two unexported `int` fields on `view`, incremented in
  `sweep()`; no hook, no API, no exported symbol. Their cost: two integer additions per sweep, at most 1024 sweeps
  per fold (K-2, proven maximum 1 + s ≤ 1024), so it cannot be measured against one sweep's O(n²/64 + n·e); no
  benchmark run, by reasoning, as the plan states.
- The two tests that asserted wall-clock time now assert exact counts; no `time` import is left in them (the gate
  greps it).
- The accessor `foldCost` lives in one test file (`foldcost_counts_test.go`) so that step 2 changed only its body.

## Skipped

- Memoising self-cuts by (author, ancestor set): an algorithmic change of the fold, trust at « complete » rigour,
  residual R-TC1 for TR1.

## mutation protocol/trust

gremlins v0.6.0, `--workers 4 --timeout-coefficient 10 ./trust`, from `protocol/`, 2026-11-16: killed 256, lived 21,
not covered 6, efficacy 92.42 % (a first run gave 255 / 22 / 6, 92.06 %: one mutant at the timing edge). The 21
survivors are exactly those block P3 left in `protocol/trust` at 92.4 % by the user's decision of 2026-11-08
(`gates/P3/SIMPLIFY.md`, « mutation protocol/trust »): `attest.go:109`, `fold.go` 90, 375, 391, 489, 506 (88, 371,
387, 485, 502 before the two counter lines), `list.go` 175, 391, 399, 458, 463, 465, 469, 500, 516, 518, `op.go` 320,
364. None is on a line of this block; the six mutants of TC1 (MK1–MK6) are all caught by their named tests. The
package stays owned by TR1 and the protocol audit for those survivors.

## Residuals

- R-TC1: the fold is Θ(k·m) in self-cuts (at most n(n+1)/2 = 524 800 ops swept at the seal); the cost inside one
  sweep (`spared` per pair, `survivors` in e²) is bounded by no gate (BenchmarkFoldAtCap is indicative). TR1.
