# RF1 - red witnesses (draft gate, revision 2)

Recorded on 2026-11-24, at step 1, before any RF1 code. The gate draft had been revised after the round-1 review
(`SPEC-REVIEW.md`, DeepSeek V4 Pro, « accepter avec corrections »).

- **Tree.** Worktree `inkwell-rf1`, branch `rf1` from `main` `df31531`.
- **Runner.** A small runner reads `gates/RF1.toml` and runs each criterion's `command` once, with bash, from the
  worktree (`MISE_TRUSTED_CONFIG_PATHS` set to it).
- **Machine.** A push CI was running at the same time, and every heavy command waited for its `heavy.sh` slot.
- **Logs.** Raw logs are in the session scratchpad (`crit2/*.log`); TLC logs in `~/.cache/inkwell/tla/logs/RF1*.log`.

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| `docs` | 30 s | 1 | 0 s | The ten §16.1 anchors are missing (share, 16/8/48, bucket, 184). The old « 128 entrantes, 128 sortantes au total » is still there. `CHANGES.md` has no RF1 entry. The TLA README anchors are present | **yes**: the rows are in plan §3 and enter the spec after review round 2 |
| `tla` | 390 s | 0 | 68 s | 26 ✓, no ✗. Flood: 1,657,395 states, 163,866 distinct, depth 25, 35 s. Inbound: 9,331 states, 2,108 distinct, depth 16. Mixed: 406,549 states, 44,100 distinct, depth 20, 11 s. Liveness was checked in all three. MF1-MF13 are each caught by their named property, MF12 (blind refund) and MF13 (round 1's fixed window) by `RateBound`. The seven witnesses are reached, `WitnessResidual` among them → RLY1. The sound operators are in place | green as expected: the model is step 1's |
| `admit` | 270 s | 1 | 0 s | `protocol/admit: directory not found`, `go list exit 1`, the ten named tests missing, `protocol/vectors/admit.json` missing | **yes**: step 2 writes the package, its vectors and `FuzzLedger` |
| `go` | 300 s | 1 | 2 s | « the harness still lifts the per-IP limit » (`export_test.go`, `upgrade_sim_test.go`); the seven named tests are missing (`TestMemberShareKept` among them) | **yes**: step 2. The defect itself is reproduced below |
| `bounds` | 420 s | 1 | 1 s | the three named tests are missing | **yes**: step 2 |
| `mutants` | 660 s | 1 | 0 s | `RF1 mutant entries differ from the frozen table: ['MG1', 'MG2', 'MG3', 'MG4', 'MG5', 'MG6']` | **yes**: their anchors are step 2's code |
| `mutation` | 300 s | 1 | 0 s | `protocol/admit is missing` | **yes**: it measures once the package exists |
| `regression` | 420 s | 1 | 84 s | `internal/transport` ok in 1.7 s, `internal/transport/p2p` ok in 73.3 s, `internal/app` ok in 31.9 s, `cmd/inkwell-relay` ok. In the protocol module, `TestLimitsMatchSpecification` fails (« §16 rows changed: review and update testdata/limits.json and limits.go together »); every other package passes | **red for a reason outside RF1, already on `main`** (below). This is the widening asked by review item 4 doing its job. Once that defect is fixed, this criterion is RF1's guard: step 2 updates `limits.json` and `limits.go` with the new rows |
| `netsim` | 840 s | 0 | 17 s | scenario 9 crash, seed 20771, strict: C1d 6.5 s, C2d 7.0 s, C3k pass (1 in the join window, 0 elsewhere), CJ and CJd pass (direct 1.0 s after the join restart) | green as expected: a regression guard. Two peers do not reach the per-IP limit |
| `simplify-report` | — | 2 | 0 s | `gates/RF1/SIMPLIFY.md` absent | **yes**: step 3 |

Caps (G-1): 3630 s ≤ `max_minutes` 61 (3660 s), checked with `gate.deadline_problem` (None). The widened
`regression` (+120 s) and the fuzzing in `admit` (+60 s) took it past 60 min. No cap was lowered to fit.

## The defect, reproduced on today's code

- **What ran.** `TestUpgradeSlotsFIFO` (HP1, 10 relayed members) through `go test -overlay`. The overlay replaces
  `upgrade_sim_test.go` with a copy whose only change is line 276: `p2p.LiftRelayIPLimit(&oa, relayIP, 64)` becomes
  `_ = relayIP`. No file of the tree was edited.
- **Result.** FAIL in 0.20 s: « H-5 setup: 7 of 10 peers up after a minute without an event ». The three missing
  peers have « circuits in 0 out 0 ». The relay connection plus 7 circuits make 8 under the relay's IPv4, libp2p's
  limit.
- **Contrast.** With the lift, the same test passes (in `regression`).

## Defect found on `main` (outside RF1, for the integrator)

`TestLimitsMatchSpecification` (`protocol/limits_test.go`) has been red since `df31531` (PU1d). PU1d rewrote the §16.1
row « accusé de session sync (§8.7) »:
- the spec now says « refus, violation ou délai : flux réinitialisé, sa connexion fermée, elle seule (P-16) »;
- `protocol/testdata/limits.json` still has the old text, « refus ou violation : flux réinitialis… ».

PU1d's gate `regression` did not run the protocol module, so the drift went unseen. Shown by comparing the 66 parsed
§16 rows with `limits.json`: exactly one row differs.

## The computations of plan §4 (review items 2 and 3)

`docs/plans/2026-11-24-rf1/rf1_wave.py` (join wave) and `docs/plans/2026-11-24-rf1/rf1_recover.py` (recovery after a flood of every member) were run
with `heavy.sh`, 5 seeds each; their output is in plan §4.
- **Mesh wave, 64 members behind one relay:** with 8 per relay, 36.9 s at h = 2 s and 74.0 s at h = 4 s; with 16,
  10.4 s and 21.7 s. So the relay origin was raised to 16 (and the total to 48).
- **Recovery after the flood:** 76.9 s at h = 2 s and 124.3 s at h = 4 s with 1 token per 5 s; 78.9 s at h = 4 s
  with 1 per 1 s. The 60 s cap of the dial loop's backoff dominates, and the rate numbers are kept.

## Changes since revision 1 (recorded 2026-11-24, same day)

- **Model.**
  - The fixed window became a token bucket kept as debts: any payment order, and a refund of the member's own unpaid
    debt only.
  - `RateBound` is now « B + k in any k periods ».
  - Added: a Byzantine member and the share rule (`SharesKept`, S9).
  - `OutJustified` and `AuthJustified` became `OutNeverRefused` and `AuthNeverRefused`: the share makes honest
    refusals impossible.
  - New mutants: MF10 NoShare, MF12 RefundUnheld (the bucket version's own), MF13 WindowRefill (round 1's abstraction,
    caught).
  - Witnesses: `WitnessOutCap` dropped (no honest dial is refused any more); added `WitnessRateRefused` (the
    reviewer's case: strangers drain the bucket, and L1 still holds through the node's dials), `WitnessSurplus`,
    `WitnessByzRefused`, `WitnessPaidPending` and `WitnessResidual` → RLY1.
  - Runs went from 19 to 26.
- **A mutant tried and dropped.** `NoShareStarved` (MF10 caught by `OutNeverRefused`) was vacuous with one Byzantine
  member at `PerCap` 4: three honest members at 2 and the Byzantine one at 4 make 8 < 9, so the honest dial is never
  refused. It was removed rather than given its own constants. `SharesKept` catches MF10.
- **Gate.**
  - `admit` gained the vectors, 30 s of `FuzzLedger` and the import check.
  - `go` gained `TestMemberShareKept`, and `mutants` gained MG6.
  - `regression` was widened (review item 4).
  - `max_minutes` went from 60 to 61.

## Revision 1 notes (kept)

- **The `go` criterion's structure.** It first chained `! grep LiftRelayIPLimit && go test …`, and the checker
  crashed when the grep matched. The check now runs inside the checker.
- **TLC parse failures.** The first runs failed at parse time because a config cannot write `-1`
  (`MaxAtt <- Unbounded`) and unary minus needs `Integers`. They were not property violations.
- **A fresh worktree.** `internal/app` needs `mise run sidecar:fetch` (pinned, download cache) in a new worktree, as
  in PU1d.

## Code review, round 1 (recorded 2026-11-24, step 2)

`gates/RF1/CODE-REVIEW.md` (« refuser »). Each finding has a named test (`gates/RF1/SIMPLIFY.md`, « Cross-family
code review »), red on the code before its fix for the confirmed ones:
- **Roster growth** (finding 1): `TestRosterGrowthShedsSurplus` red (« sheds 0 surplus connections, want 2 »). The
  model gained the case: constant `Late`, variable `admitted`, action `AdmitLate` with `ShedStep`. The runs are kept
  apart from the 26 the gate froze, under `INKWELL_TLA_ONLY='^RFR'`: `RFRGrowth` exhaustive, no error, liveness
  included (25,453 states, 3,010 distinct, depth 16, 1 s); mutant `RFRNoShed` (MFR1, no shed) violates `SharesKept`
  (trace of 5, `AdmitLate(3)` last). The 26 `^RF1` runs pass unchanged with `Late = {}`.
- **Unlisted peer at an Allow-listed relay** (finding 2): `TestRelayUnlistedHoldsNoPlace` red before the gater change,
  with the stranger holding 9 of its address's places. After the change: at most its one handshake's place.
- **Open relay** (no `Allow`, the RLY1 residual, plan §4): the same test with the gater off measures 9 places at the
  peak, each connection ending after about 45 s (one dial a second for six minutes, kept alive). → RLY1.
- **Sweep mutants** (finding 3): `TestBucketsBoundedUnderChurn` kills the negation and the inversion of the sweep
  condition; two variants stay equivalent.
