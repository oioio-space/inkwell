# /simplify — PU1

Diff : c7c90c7.. (rebased onto main 3ac908e, which carries the limits constants; 10 tracked files changed, 5 new: `protocol/syncack/{syncack,syncack_test,corpus_test}.go`,
`protocol/conformance/syncack.go`, `internal/transport/p2p/peerup_ack_test.go`). Reviewers: four /simplify angles
(reuse, simplification, efficiency, altitude), Sonnet / medium each, read-only; findings triaged and applied by the
step-2 session (Opus). The review that counts for the merge is the integrator's (Opus or Codex), not this one.

## Appliqué

- One budget: `openTimeout = syncack.Budget`, and `syncack.Budget = protocol.SyncAckTimeout`; the test-only
  `OpenTimeout` export and its equality check removed (TestSyncAckReference keeps only `ProtocolSync ==
  syncack.ProtocolID`, since the frozen arch criterion wants the literal in p2p). `closedAtBudget` reads
  `syncack.Budget`, not a fourth 15 s.
- §16.1 row « accusé de session sync » had no constants (`TestLimitsMatchSpecification` red on c7c90c7): now on
  main (3ac908e, `SyncAckLen`, `SyncAckTimeout`); `syncack.Budget = SyncAckTimeout`, `ReadAck` reads `[SyncAckLen]byte`.
- `Opener.Step`: the ErrOrder message built once (`order`), not twice.
- `protocol/conformance/syncack.go`: the opener script reuses `lpEvent` (one `byte_hex` field added), `c.events()`
  (adds the monotonic-time check the private decoder lacked) and `checkString`; the three verdict tails fold into
  `verdictIs`.
- `TestResponderAccept`: the want table is literal (it recomputed `Accept`'s own expression).
- Tests: `natNet.simOptions` shared by `nodeOn` and `rawHost` (the simulated-network option list was in two places);
  the recorder documents that a recorded node is never `Recv`ed elsewhere.
- `Opener` documented as the reference model of §8.7.5; the node follows it with its own control flow, its arms
  proven by `TestPeerUpAfterAck` (finding « the machine is not driven by the node »).

## Codex code review (2026-11-16), MAJOR, fixed at the root

The responder wrote the ACK before the stream took its place in the peer's order (`ps.inTail`), which it did only
after `waitUp`: a handler held between its ACK and that registration let the next (direct) stream register and be
read first, against P-6. Red first: `TestPeerUpMoveAfterAck/move_keeps_order_across_late_handler` holds b's handler of
the relayed stream right after its ACK (test seam `SetAfterAck`) while a moves and sends on the direct stream; on the
previous code b delivered frame 511 first (red-order.log). Fix (`stream.go`): the stream takes its place (`inOrder`,
`takeOrderLocked`) before its ACK, without waiting for PeerUp; it takes a new place only if the peer's state changed
by the time it is announced; a stream that leaves before its turn (refused, never announced) hands the order on — its
place ends with the one ahead of it (`leave`, under `n.wg`), so the streams behind it still wait for that one.

## Superseded by block PU1b (2026-11-16)

Codex's second code round found that the round-1 fix's chain (`inOrder`: a stream that left before its turn handed
the order on by waiting, without bound, for the stream ahead of it) let a Byzantine reset loop keep a goroutine and a
place per failure. Third finding in the same area: a design decision, block PU1b (`gates/PU1b.toml`, frozen fc458e8),
replaced the chain by the removable order list of `protocol/syncack` (`Order`), two places per peer identity, and the
capacity refusal (`0x1002`) the opener retries. `ps.inTail`, `inOrder`, `takeOrderLocked` and `leave` are gone; PU1's
`move_keeps_order_across_late_handler` still passes on the new code. Its /simplify review: `gates/PU1b/SIMPLIFY.md`.

## Écarté (avec la raison)

- Drive `openSync`/`upgrade` from `syncack.Opener.Step`: the node's events come from libp2p calls and a context, not
  from a script; wrapping them in Step events adds ceremony without a rule the arms do not already prove. Documented
  instead (above).
- `skipAck`/`beforeAck` as an injectable `readAck` func or an `ackHooks` struct: the frozen gate names « a test-only
  seam … skips the ACK read » and mutant MP1's anchor is `if err != nil || n.opts.skipAck {`; two unexported fields
  in the existing seam style (`extra`, `rewrite`, `onDial`) are the smallest form.
- `context.AfterFunc` → `SetReadDeadline`: the AfterFunc also ends the read on `Close` (n.ctx), which a deadline does
  not; the double `Reset` on a timeout is harmless.
- `acceptSync` takes `n.mu` once more (`isClosed`) and evaluates the three facts eagerly: one lock per inbound stream,
  off any hot path; keeping the full `Responder` value keeps the decision the reference's.
- 1-byte allocations in `ReadAck`/`WriteAck`, double `fmt.Errorf` on a refusal: once per stream, not per frame.
- `Role`/`announced` folded into one bool: the vectors carry both fields (`role`, `peer_announced`).
- `one_sided_admission`'s `Admit` call counter (« brittle »): the flag flipped in `ConnectedF` was the flaky form
  (reg1.log: a's own admission check could run before the flip). The counter states the scene exactly (the gate
  admits, every later check refuses) and is commented.
- `TestRoamToNewAddress`'s own `Notify` bundle versus `watchConn`: different facts (first remote address versus
  times); one small bundle in place.
- Share `waitFor`/an `isCircuit` test helper across files, move the raw-host fakes to a shared file: outside the
  block's surgical scope.

## Changement d'un test existant (à relire par l'intégrateur)

`TestRoamToNewAddress` asserted `linkTo(a, b) == LinkRelayed` « right after the return » (both PeerUp). With PU1,
PeerUp waits one round trip for the ACK, and the hole punch to b's new address lands in it on 10 of 20 seeds
(reg1.log). The link kind at PeerUp was a timing proxy for the rule « the old address is useless, the way back is the
relay »; the test now asserts that fact directly: a's first connection to b after the return is a relay circuit.
The later checks (quiet 10 s, direct within a minute) are unchanged.

## mutation protocol/syncack

gremlins v0.6.0, 4 workers, coefficient 10: killed 21, lived 0, efficacy 100 % (one survivor at step 2, `int(o) <
len` → `<=` in `Outcome.String`, killed by `Outcome(6)`). « Not covered » are the `case` conditions of `Step` and
`accepted` (gremlins' known blind spot) and the `Budget` constant: each `case` negated by hand (199, 201 ×2, 204,
241) fails `go test ./syncack/`.

## Vérifié

After the Codex fix: go, regression, mutant, arch, docs, vectors rerun (final3.log; go and vectors again in final4.log after a slot wait hit their caps), PU1 tests × 10. Before it,
gate criteria run once each on the tree: vectors, fuzz (60 s), tla (19 ✓, exclusive), go, regression, mutant
(MP1–MP3 caught), arch, docs — exit 0. Also: `TestPeerUpAfterAck|MoveAfterAck|Latency` × 20 without -race; the packages
touched and their consumers (`./internal/transport/...`, `./internal/app/...`, `./internal/bridge/...`,
`protocol/...` with -race); golangci-lint on `internal/transport/p2p` and `protocol` (0 issues). Not run here:
`gates/PU1n.toml` (netsim, exclusive, later). No frontend code names the sync protocol ID.

## PU1c (2026-11-17)

`acceptSync` became `acceptInbound` (the same Responder check, on both lanes) and `openSync` wraps `openAcked`
(protocol and role as arguments, the bulk opener reads the ACK too); PU1's arms (`TestPeerUpAfterAck`,
`TestPeerUpMoveAfterAck`, `TestPeerUpLatency`) pass unchanged (PU1c's inherited criterion). Review:
`gates/PU1c/SIMPLIFY.md`.

## Review follow-up (Codex, 2026-11-21)

The replacement-stream regression closes the retired stream before starting its replacement. This lets libp2p
return the old stream's protocol scope before opening the new one, so a local capacity error does not close the
surviving peer connection. The old ordering fails the deterministic witness. The repaired scenario passed 200
iterations under the race detector. See `gates/PU1c/STATE.local.md` for the run evidence.

The eight frozen criteria passed in the diagnostic replay on `f5d2b55`, including fuzzing, the named race tests,
and MP1–MP3. The base guard still blocks acceptance. The diagnostic manifest is in the evidence directory named
in `gates/PU1c/STATE.local.md`.
