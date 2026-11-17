# HP1r - red witnesses

Recorded 2026-11-16 at step 1, before any production code of rule H-19, each criterion run exactly as written in
`gates/HP1r.toml` on the worktree `inkwell-hp1` (HP1's step-2 code, uncommitted). Second round, after Codex's freeze
review (REJECT): the general clean-up at the last circuit's loss is removed (H-19 is the `ConnectedF` check alone),
the arm `cleanup_circuit_gone_after_check` is replaced by `kept_upgrade_outbound_circuit_closed`, the witness
`HP1rWitnessSurvives` is added, and the rigour is complete. Raw logs in the session scratchpad (`hp1r-w2-*.log`; first
round `hp1r-w-*.log`, `hp1r-tla44.log`).

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| tla, before the runs were registered (first round: `scripts/tla-check.sh` without its `run HP1r…` lines) | 600 s | 1 | < 1 s | `want 2 ✓, got 0` (now 3 are required) | yes: the runs do not exist |
| tla, structure on the model before the H-19 edit (`DirectUpgrade.tla` and the seven configurations as they were) | (same) | 1 | < 1 s | `LandOK == circuits # {} missing`, `LinkBound (S18) missing`, `landing branches: 6, guarded by LandOK: 0`, `invariant LinkBound missing` ×7, `DirectUpgradeLinkLost.cfg missing` | yes |
| tla, current tree (model written at step 1) | 600 s | 0 | 83 s | `✓ HP1rLinkLost: no error (exhaustive, liveness checked)`, `✓ HP1rLateLand: violation detected (LinkBound)`, `✓ HP1rWitnessSurvives: violation detected (NoDirectAfterRelayLost)` | passes with the real tools |
| go | 300 + 600 s | 1 | 4 s | `missing or not passing: TestUpgradeLateLinkClosed` and its 12 arms (`[no tests to run]`); `TestWantStopsAndDrop: 28 passed, 12 failed of 40` (first round 20 of 40), each failure `dial_test.go:53: … unexpected event PeerUp` (the late link accepted after re-admission, the trace's bug) | yes |
| mutant | 30 + 30 + 600 s | 1 | < 1 s | entry table matches; `mutant MR1: anchor in internal/transport/p2p/node.go found 0 times (need exactly 1): 'if n.lateLink(c) {'`: the check is not written yet | yes (not a catch, not a pass) |
| docs, on the texts without HP1r (H-19 row removed, H-4's open note back, the ADR's H-19 paragraph removed, spec and TLA README at HEAD) | 30 s | 1 | < 1 s | `plan row H-19 lacks …` ×17, `plan row H-4 still has its open note`, `spec §8.6 lacks …` ×6, `ADR 0045 decision lacks …` ×5, `tla README lacks …` ×5 | yes |
| docs, current tree | 30 s | 0 | < 1 s | the documents are step 1's specification | passes |

## Can pass, with the real tools

- **tla**: `scripts/tla-check.sh` writes TLC's logs to `~/.cache/inkwell/tla/logs/HP1rLinkLost.log`,
  `HP1rLateLand.log`, `HP1rWitnessSurvives.log`. HP1rLinkLost: 1,200,294 states, 263,386 distinct, depth 34,
  1 min 17 s, complete state space. HP1rLateLand's counterexample is the observed bug: `autoL = -2` (libp2p's
  one-shot, its first direct dial pending), the relay drops the circuit (`circuits = {}`), `AutoPre` lands
  (`direct = TRUE`, `lateLand = TRUE`) at depth 5. HP1rWitnessSurvives reaches a direct session with no circuit after
  `RelayLost` (1,582 distinct states).
- **Model scope** (plan row H-19, TLA README): S18 covers a landing without a circuit. No property checks that an
  upgraded session survives its circuit's closure: no action closes a direct connection on a circuit's loss
  (`RelayLost`, `Resolve` leave `direct` unchanged; only `DirectLost`, `RRestart` set it FALSE), so it holds by
  construction and an invariant would be vacuous; the witness shows the state is reached, the Go arm
  `kept_upgrade_outbound_circuit_closed` checks the product. No invariant was added to the seven configurations.
- **HP1's frozen count**: after the model edit (first round), `INKWELL_TLA_ONLY='^DirectUpgrade'` gave 44 ✓, 0 ✗
  (exit 0). The second round adds only a definition (`NoDirectAfterRelayLost`) to `DirectUpgradeMutants.tla` and runs
  named outside the pattern; to be confirmed by HP1's `tla` criterion on the candidate.
- **go**: `go test -json` writes `.task/hp1r-go.json` and `.task/hp1r-drop.json`; the parser read the real events
  (28 pass and 12 fail events of `TestWantStopsAndDrop`, the package `[no tests to run]` for the missing test).
  Subtests report as `TestUpgradeLateLinkClosed/<arm>`, the names the criterion lists. 40 runs of
  `TestWantStopsAndDrop` take about 4 s (synctest, no -race); the 12 arms under -race are estimated well under 300 s
  (HP1's 18 `TestUpgrade*` tests take about 100 s under -race).
- **Statistical acceptance**: measured failure rate before the fix 0.30 to 0.50 (19/40, 20/40, 12/40); the false pass
  P(40/40 | p) = (1 - p)^40 is 6e-12 at 0.475, 6e-7 at 0.30; a residual rate ≥ 7.2 % is rejected at 95 %. → HP1
- **mutant**: `scripts/netsim/mutants.py` copies the tree, applies the edit and runs
  `go test ./internal/transport/p2p/ -run ^TestUpgradeLateLinkClosed$ -count=1 -json`; a catch needs a fail event
  of that test and an output line of it or its subtests holding « H-19 late link kept » (the same path catches HP1's
  MH* mutants, gates/HP1m). `mutants.py --check HP1` still passes (MR1 is in block HP1r).
- **docs**: plain reads of the five files.

Sum of the caps: 2 190 s ≤ 2 700 s (`gate.deadline_problem` returns none).
