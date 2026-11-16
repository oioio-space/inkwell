# TC1 - red witnesses

Recorded 2026-11-16 at step 1, before any production change (`protocol/trust/fold.go` is the one of `main`).

Each command was run exactly as written in `gates/TC1.toml`, from the repository root, one at a time, after the
blind review's fixes F1..F6 (the commands of the first round are superseded). Logs: scratchpad
`witness-TC1-<id>.log`. On the first pass of this round the machine was loaded by other blocks: `mutant-tool`
(exit 124 at 60 s) and `regression` (exit 124 at 120 s) expired waiting for a heavy slot, and `mutants` took 85 s;
the three were run again on a free machine, which is what the table shows.

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| docs | 1 | < 1 s | `TC1 docs: [...]`: the six French key texts missing from the cost paragraph of §4.9 (« dont le sujet est l'auteur », « au plus 1 + s balayages », « exactement 1 + s quand », « au coût d'un balayage », « 524 800 ops balayées », « pas une borne de temps »), the six English ones missing from `doc.go`, and `doc.go still says 'one more fold per self-cut'`; the four plan rows and the single DOIT are found | yes (K-4) |
| go | 1 | 28 s | the four named tests fail, each with `TC1 step 1: the fold does not count its sweeps yet (view.sweeps, view.swept; N ops folded)` (`TestSelfCutsCostAtSeal` stops at `chain of 12`, N = 13); the package builds; the clock check is not reached | yes (K-1..K-3: the counters do not exist) |
| mutant-tool | 0 | 1 s | `OK` (unit tests of the runner), `mutant MK1..MK6: edits apply`, the six entries carry test and expect | passes by design: the entries are the specification written first, and their anchors are in today's `fold.go` |
| mutants | 1 | 7 s | `mutant MKn -> … : NOT CAUGHT (go test exit 1; <test> did not fail with '<expect>')` for the six; `netsim mutants: not caught: MK1, MK2, MK3, MK4, MK5, MK6` | yes: each named test fails on the step-1 accessor, never with a rule's text, so nothing is caught by accident |
| regression | 1 | 9 s | the same four tests and no other fail in `./trust/...` (`FAIL … 9.229s`) | yes (the rest of the package is green) |
| simplify-report | 1 | < 1 s | `gates/TC1/SIMPLIFY.md` is the skeleton: its « Diff : » line names no commit | yes |

Red for the right reason: 5 of 6; `mutant-tool` passes by design. The clock check of `go`, run alone on the four
files: grep exit 1 (nothing found), so that part already holds.

## Capable of passing (checked on a copy, not in the tree)

A copy of `protocol/` with the two counters of the plan (`v.sweeps++`, `v.swept += len(in)` in `sweep`) and
`foldCost` returning them: the `go` command's test run passes in 17.3 s under `-race` with `-timeout=90s`
(`ok … 16.054s`); each mutant applied by hand turns its named test red with its text — MK1 `K-1 sweeps = 841, want
421`, MK2 `K-2 ops swept = 342694, want 254704`, MK3 `K-1 sweeps = 25, want 13` (chain of 12), MK4 `K-3 one sweep:
fold of 1001 ops made 2 sweeps over 2002 ops`, MK5 `K-1 sweeps = 531441, want 13` in 0.7 s (the seal lists are not
built), MK6 `K-1 bound and K-2 bound: 3 sweeps over 6 ops for 4 ops and 1 self-cuts`. The import-path form the
mutants use runs from the root: `go test github.com/oioio-space/inkwell/protocol/trust -run
'^TestCascadeSingleSweep$'` → `ok`. A red run of `TestFoldCostWithinSelfCutBound` leaves a rapid failure file under
`protocol/trust/testdata/rapid/`: it was removed after each run and is not part of the block.
