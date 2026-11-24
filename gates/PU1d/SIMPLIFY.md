# /simplify — PU1d

Diff : 789127f..the worktree `inkwell-pu1d` (main at the frozen gate, then step 1's uncommitted specification and step
2's code).
- Step 1: spec §8.7, `CHANGES.md`, the TLA README, `CloseScope.tla` and its configurations and mutants,
  `scripts/tla-check.sh`, the vector generator and `syncack.json` / `syncorder.json`, the plan.
- Step 2, production code: `internal/transport/p2p/stream.go` and `protocol/syncack/{syncack,order}.go`.
- Step 2, tests: `internal/transport/p2p/close_scope_test.go` (new: the seven named tests and, after the code review,
  `TestPreferredCircuitStalledKeepsNewDirect`),
  `internal/transport/p2p/stream_internal_test.go`, `hp1_lifecycle_test.go`, `protocol/syncack/{syncack,order}_test.go`.
- Step 2, mutants: `scripts/netsim/mutants.toml` (MD1-MD3).

Reviewer: the step-2 session (Claude Opus), one pass over the whole diff for reuse, simplification, efficiency and
altitude. A second-family review of the code is still to come (method v2). Every check was run on the final tree.

## Root cause and the change

**Root cause.** The opener closed every connection to the peer (`ClosePeer`, `stream.go` `openOut` and `outLost`) on a
refusal, a violation or the deadline of one opening. It could do nothing narrower, because the opening went through
`BasicHost.NewStream`, which chooses the connection inside the swarm. On a failure before the return it resets the
stream and returns nil, so the node did not know which connection had failed (spec §8.7.4, plan §1-§2).

**The change.**
- The session opening names its connection before it starts. `sessionConn` picks the circuit when `preferCircuit`
  asks for it, else the connection whose arrival causes the opening (`onConn`'s `c`), else the one the swarm would
  take. `betterConn` is go-libp2p's unexported `isBetterConn`.
- The opening runs on that connection itself: `openOn` = `network.Conn.NewStream`, then the host's identify wait,
  then the optimistic path (`lazyStream`, the host's unexported `streamWrapper`) or negotiation.
- A failure closes `on` alone (P-16). `outLost` closes `s.Conn()` alone after a refusal or a violation after the ACK.
- `reopenLocked` opens the session again on any connection still open (P-17). It is shared by the lost and refused
  paths of `openOut` and `outLost`.
- The reference `protocol/syncack` answers `close_conn` (`CloseConn`, `CloseConnAction`).

## Findings

| # | Finding | Decision |
|---|---|---|
| 1 | `openOn` first reset the stream and opened a second one, through `newStreamOn`, when the peer's protocol was unknown | **applied**: `negotiate` was taken out of `newStreamOn` and runs on the same stream; `newStreamOn` (H-15 circuits, DCUtR rounds, P-15 pings) now calls it too, unchanged |
| 2 | staticcheck QF1008, `s.Stream.SetReadDeadline` in `lazyStream.Close` | **applied** (`s.SetReadDeadline`, the same method through embedding); `CloseWrite` keeps `s.Stream.CloseWrite`, which it overrides |
| 3 | gofumpt on two test files | **applied** |
| 4 | Always negotiate on the named connection (plain `newStreamOn`) instead of copying the host's lazy wrapper | **rejected**: it would change two things at once (the connection and the negotiation path) and drop the identify wait that `remember` relies on at PeerUp. `lazyStream` is 25 lines, and the go-libp2p type it copies is unexported. PU1's `TestPeerUpLatency` and `TestPeerUpAfterAck` (optimistic arms) still pass in `regression` |
| 5 | `errNoConn`, a sentinel only to make `err` non-nil when no connection is open | **kept**: the lost path then runs as for any other loss (no close, `reopenLocked`, `settleLocked`), with no second branch |
| 6 | `preferCircuit` used to re-read `circuitTo` at each busy retry ("when the replacement actually opens") | **accepted change**: the circuit is read once, when the opening starts, and busy retries stay on it. A retry on another connection would no longer be the named opening (P-16), and the opening starts at the same moment as before |
| 7 | A refusal or violation after the ACK reopens without rule L's resync | **as specified**: §8.7.5 row « Ouvert … octet reçu ; fin ou réinitialisation par le pair, connexion vivante » says the peer stays announced, like the existing « flux perdu, connexion vivante » row and the connection-loss path (`watchOut` → `outLost`, resync false) |
| 8 | Drop, Hangup, Reset and the admission sweep (`node.go:797,933,955,985`) keep `ClosePeer` | **as specified**: peer-wide by definition (§8.7.4: only admission withdrawal and the node's close close every connection) |

## Cross-family code review (`gates/PU1d/CODE-REVIEW.md`, DeepSeek V4 Pro, « accepter avec corrections »)

**Finding 1 (major): `preferCircuit` holds a reopening on a dead circuit while a live direct connection waits.**

What HP1 requires:
- `preferCircuit` comes from HP1 (`a0e40de`; plan `2026-11-16-hp1-direct-upgrade.md`, "connection-selection race",
  and post-port finding 2).
- In PU1b's `drain_replace_reconnect`, two direct connections were closing at once. The reopening picked one of them,
  and its pre-ACK failure closed the peer's surviving circuit (`ClosePeer`).
- HP1 therefore prefers the surviving circuit after a confirmed direct loss: the siblings of a lost direct connection
  share its path. H-20 and P-15 (plan `2026-11-23-hp1-reformalize.md` §2) bind work to the connection that causes it
  and close a superseded silent connection. They say nothing about which connection a reopening takes.

On main before PU1d, the trace was real and worse:
- The old `openOut` read `circuitTo` per attempt, but `openAcked` repeats an attempt only on a capacity refusal
  (`isBusy`). A blackholed circuit gives none, so the single attempt held `ps.opening` for the whole 15 s, exactly as
  now.
- Then the deadline, not a loss (`openingLost`: the circuit was still open), reached `ClosePeer`, which closed the
  new direct connection too.
- Measured: `TestPreferredCircuitStalledKeepsNewDirect` (new, `close_scope_test.go`) is red against main's
  `stream.go` (scratch copy of the tree, main's `stream.go` and its two internal test files) with
  « P-16 other connection closed: … closed the new direct connection » and « 1 PeerUp, 1 PeerDown ». The fixture
  confirms that the reopening went to the circuit (2 sync streams on it). The same holds for
  `TestDeadlineOnStalledCircuitKeepsDirect`.

On PU1d:
- The same test is green (-race, 1.2 s): the circuit alone closes at the deadline, the new direct connection carries
  the session, and the peer stays announced.
- **Verdict: an inherited trace, which PU1d removes (no PeerDown, the live direct connection kept).** The defect was
  `ClosePeer`, fixed here at its root.
- The 15 s wait itself is P-5's designed bound on one opening (S4 `Budget`; L2 `OpenResolves`). P-15 shortens it when
  the peer's own session lands on the newer connection: the silent circuit is closed once a newer connection carries
  an acknowledged stream in either direction, so this opening ends as lost.

Reviewer's fix, rejected:
- The fix was « the circuit only if no live direct connection ». It reverses HP1's decision without its evidence: the
  siblings of the lost direct connection can be open and dying.
- It does not cover the trace either: the new direct connection arrives *after* the opening has named its connection.

**Residual → PU1e** (named in `PROGRESS.md`):
- A connection that arrives while an opening holds on an older one cannot pre-empt it. → PU1e
- Pre-emption is a new rule. It needs the spec and a model (`CloseScope`/`Supersede`: a newer connection starting an
  opening while the older one's opening is unanswered). → PU1e
- Its Go mutant would need a row in `gates/PU1d.toml` `mutants`, whose table is frozen at MD1-MD3. The new test runs
  under `regression` (the whole p2p package), not under `go`, which names the seven frozen tests. → PU1e

**Finding 3 (minor): an opening could start with no named connection (`named == nil` → `errNoConn`).**
- **Applied.** `startOpenLocked` names the connection as it sets `ps.opening` (`sessionConn`, under `n.mu`) and starts
  nothing when none is open. `openOut` receives it, which is S7 `OpeningNamed` exactly: no opening is in flight
  without its connection.
- `errNoConn` and `liveConn` are removed. `reopenLocked` is now `startOpenLocked(ps, preferCircuit, nil)`, and MD3's
  anchor moved with it (step 2 adjusts anchors only).

Findings 2, 4, 5 and 6 need no change: 5 is covered by the new test, and its mutant is the residual above. → PU1e

## Tests adapted, not weakened

The session opening no longer calls the host's `NewStream`, so the fakes that stood for it moved to the connection.
Each test's scene and assertions stay the same:
- `stream_internal_test.go`: `TestOutLostReleasesStreamBeforeReopen`, `TestOpenOutConnectionLossPreservesReplacement`
  (it now also checks that the replacement connection is not closed), `TestOpenOutLostWithoutStreamSparesReplacement`
  and `TestNewConnectionOpensWithoutConnectednessEvent`. `closableConn` gained an `open` hook, and
  `lostAndReplacement` holds the shared pair.
- `TestOutLostReleasesStreamBeforeReopen`: its `outLost(…, true, …)` meant `reopen`; the parameter is now `refused`,
  so the call passes `false`, the lost case it always tested.
- `hp1_lifecycle_test.go` `TestReopenChoosesLiveConnectionAfterReset`: the fresh connection's `open` hook replaces the
  host's. The assertions are unchanged: the dead circuit is not opened, no ClosePeer, the live connection opens.
- `openRecordingHost` lost its `open` field and `NewStream`. A session opening that went through the host would now
  panic on the embedded nil host. `openRecordingConn` was unused and is removed.
- `protocol/syncack` tests: `ClosePeer` → `CloseConn`, `ClosePeerAction` → `CloseConnAction`, names only.

## mutation protocol/syncack

gremlins (criterion `mutation`, the `block_done` settings): Killed 114, Lived 4, Not covered 15, Timed out 0; **test
efficacy 96.61 %** (≥ 95 %), mutator coverage 88.72 %, 14 s. The four survivors are in files PU1d does not touch:
- `lifecycle.go:261:75` ARITHMETIC_BASE (PU1c);
- `supersede.go:62:12` CONDITIONALS_BOUNDARY (HP1c);
- `supersede.go:135:50` CONDITIONALS_BOUNDARY and CONDITIONALS_NEGATION (HP1c).

The « not covered » cases in `syncack.go` (lines 204, 206, 209, 251) and `order.go` (65, 67) are `switch`/`case`
conditions. gremlins reports those as not covered (CLAUDE.md, « Pièges connus »), and `TestCorpus` with its 13
`close_conn` vectors executes them.

## Contract change to record (method v2 rule 9), draft for the integrator

> **PU1d supersedes the `vectors` criteria of `gates/PU1.toml` and `gates/PU1b.toml`** (2026-11-24; decision:
> `docs/plans/2026-11-24-pu1d-close-scope.md` §2; reviews `gates/PU1d/SPEC-REVIEW.md`, `GATE-REVIEW.md`).
> - **What changes.** PU1 `vectors` pins `syncack.json` at (34 vectors, `5307797671903594…`). PU1b `vectors` pins
>   `syncorder.json` and checks `syncack.json`'s same digest. Rule P-16 changes the expectation of 13 public vectors
>   from `close_peer` to `close_conn`: 9 opener events of `syncack.json` (`sync-ack-opener-{eof,reset,bad-byte,
>   byte-after,deadline,slow-negotiation,deadline-before-return,silent,reopen-refused}`) and 4 refusal actions of
>   `syncorder.json` (`sync-refusal-{busy-out-of-budget,other-code,negotiation-code,eof}`). No other field of any
>   vector changes.
> - **New pin.** `gates/PU1d.toml` `vectors`: `syncack.json` (34, `dc6ced8357d63d4b4877aecf9cb8d98c044352e6b62809eaa4d76966221f6c74`),
>   `syncorder.json` (20, `b91c14f1f2e4d0f7c5a4c317effc8a893b75df8aa36d2c2bd077c798257936a8`), and no `close_peer` in either.
> - **Old/new compared.** Under the old corpus the reference returned `close_peer` on those 13 inputs. Under the new
>   corpus it returns `close_conn`, and every other expectation is identical (diff of the two JSON files: only
>   `expect` / `action` and the description of those 13 vectors).
> - **Negative control.** The frozen corpus and the reference still catch a regression. With the reference back to
>   `close_peer` (step 1's tree), PU1d `vectors` was red on exactly those 13
>   (`gates/PU1d/WITNESSES.md`, row `vectors`). MD1 (ClosePeer back in `openOut`) is caught by
>   `TestFailedOpeningClosesOnlyItsConnection`.
> - **Untouched.** Every other criterion of PU1 and PU1b stays as frozen: `PeerUpAck.tla` is byte-identical to
>   `c7c90c7`, and PU1's 19 `^PeerUpAck` runs are unchanged.

## Pre-existing, outside PU1d (for the integrator to route)

`scripts/netsim/mutants.py --check` fails on anchors that are already absent at `789127f`, before this block (checked
with `git show HEAD:<file>`):
- NI0 M5c (`relay.go`, `libp2p.EnableHolePunching()`);
- PU1 MP1 (`stream.go`, `if err != nil || (n.opts.skipAck && …)`);
- RL1 RL1a (`stream.go`, `n.outLost(ps, s, true, !canceled, nil)`);
- SB1b-06, -13, -30, -33 (`internal/sidecar`).

Every block whose anchors PU1d's edit touches still applies (PU1 MP2-MP5, PU1b, PU1c, HP1, HP1r, HP1c), and so does
PU1d's own MD1-MD3.

## Checks on the final tree (each gate criterion's own command, one by one)

| Criterion | Result |
|---|---|
| `docs` | green, 0 s |
| `tla` | green: 10 ✓, 135 s |
| `vectors` | green, 1 s |
| `go` | green: the seven tests, -race, 5 s |
| `mutants` | green: MD1, MD2, MD3 caught by their tests and texts, 5 s |
| `mutation` | green: 96.61 %, 14 s |
| `regression` | green: p2p 71.2 s, app 29.5 s |
| `netsim` | green: scenario 9 crash, seed 20771, strict; C1d 6.5 s, C2d 7.0 s, C3k pass, on binaries rebuilt from this tree, 20 s |
| `simplify-report` | this file |

Also green: `go build ./cmd/... ./internal/...`, golangci-lint (p2p and `protocol/syncack`: 0 issues), `mise run test:py`
(201 tests).
