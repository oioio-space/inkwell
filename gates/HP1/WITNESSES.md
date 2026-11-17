# HP1 - red witnesses

Recorded 2026-11-16 at step 1, before any production code (`internal/transport/p2p` and the fork are those of
`main`; `protocol/directup` holds only the named tests, pending). Each command was run exactly as written in
`gates/HP1.toml`, from the worktree root, by a runner that reads the command from the TOML (scratchpad
`hp1/runcrit2.py`, logs `hp1/witness-HP1-<id>.log`). The network simulation of plan §9 is in the three other gates of
the block (`gates/HP1n.toml`, `HP1n2`, `HP1n3`; their witnesses in their own folders).

The named tests are red stubs: each body calls `pending` / `hp1Pending` (test-only files
`protocol/directup/pending_test.go`, `internal/transport/p2p/upgrade_pending_test.go`, deleted at step 2), which fails
with `HP1 step 1: H-n is specified … but not implemented yet`. That text never contains a rule's failure text, so no
mutant can be caught by a stub. The behaviour tests that the plan wants red on `main` first (H-2, H-6, H-10, H-11,
H-12) get their bodies at step 2, in TDD: each is run red on the step-1 tree before its code is written.

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| docs | — | 1 | 0.0 s | `HP1 docs: ['spec §8.6 missing', 'spec §16 does not state the HP1 bounds …', 'spec §20.6 does not register /libp2p/dcutr', 'protocol/vectors/directup.json: … No such file', 'ADR 0045 missing …']`; the 17 plan rows, H-15 « Adopted » and the corrected worst case (456 destinations, 15.5 MB/h; 174, 5.9 MB/h) are found | yes |
| tla | 900 s, `--exclusive` | 0 | 274 s | 44 ✓, no ✗, no TwoCircuitsResolve or Residual run; the 7 configurations hold, the 30 mutant runs and the 7 witnesses each caught by their named property; the structure check passes (17 invariants in each configuration, Upgrades, BusyEnds, DupResolves in the six non-Byzantine ones, CrossCancel = TRUE in TwoCircuits) | passes by design: the model is the specification written first (definition of done, step 1) → HP1 |
| alloy | 60 s | 0 | 9 s | the 3 checks hold, the 2 runs are satisfiable, the 5 mutants caught (and every other Alloy model) | passes by design, as `tla` |
| go | 90 + 300 s | 1 | 1.7 s (builds cached) | `HP1 go: ['exit codes pure 1, sim 1', 'missing or not passing: …']`; the 17 pure and 18 sim stubs (TestUpgradeNodeTraffic and TestUpgradeAnswererBehindQueue among them) fail with `HP1 step 1: H-n is specified … but not implemented yet`; both packages build | yes |
| mutant-tool | 30 + 30 s | 1 | 0.2 s | runner tests `OK`; the frozen table (MH1-MH22, MN1, MN2) matches the 24 entries; `--check`: `mutant MN2: edits apply`, `MN1 … dial.go found 0 times`, `MH12 … gate.go found 0 times`, then `FileNotFoundError: … protocol/directup/policy.go` | yes: the anchors name step 2's lines |
| mutation | 300 s | 1 | 0.2 s | gremlins: `failed to gather coverage … exit status 1`, `HP1 mutation: gremlins exited 1` | yes |
| regression | 300 + 180 + 150 s | 1 | 62 s | `FAIL …/internal/transport/p2p 61.4s`: only the 18 `TestUpgrade*` stubs fail; `ok cmd/inkwell-relay`, `ok internal/app 24.0s`; the chain stops there (`&&`) | yes |
| simplify-report | — | 2 | 0.0 s | `grep: gates/HP1/SIMPLIFY.md: no such file` | yes |

Sum of the caps 2 340 s ≤ 2 700 s (max_minutes 45). The Go mutants moved to `gates/HP1m.toml` (22 mutants would
not fit beside tla's 900 s cap).

Third round (after the model's B1-B6 findings): docs, tla, go, mutant-tool and regression changed and were run again;
alloy, mutation and simplify-report too, unchanged. The table above is that round; the earlier rounds are superseded.

## Sizing and the regression choice

- The whole of `internal/transport/p2p` under -race took 628 s (shared machine): that alone would take a quarter of
  the gate. No test of the package reads `testing.Short`, so `-short` changes nothing there. The gate runs the
  package (and relay, app) without -race, as CI runs the root module, 92 s; and keeps -race on the 17 HP1 tests
  (`go`), the simnet tests of the dial loop, relay, gate and RelayOnly plus the five race-targeted tests (measured
  together under -race: 78 s, `ok … 76.4s`) and `protocol/conformance` (70 s). Left out: -race on the other existing
  p2p tests (probe, bulk, address book), which HP1 does not change and which no CI job runs under -race.
- The race subset and conformance parts did not run in the witness (the chain stops at the red p2p run); measured
  apart on the same tree, as above.
- `mutation` (300 s) cannot be measured before the code (no file to mutate); the Go mutants are in `gates/HP1m.toml`.
- After the first round, only caps changed, plus `mkdir -p .task` in `go` (its first run failed on the missing folder,
  superseded) and the rewritten `regression`; the changed criteria were run again, and this table is that round.
