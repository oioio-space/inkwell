# /simplify — PU1b

Diff : 3ac908e.. (main fc458e8 carries the frozen gate; the block's own files: `protocol/syncack/order.go`,
`order_test.go`, `protocol/conformance/syncack.go` (`sync_order`, `sync_refusal`), `protocol/cmd/vectors/syncorder.go`
and `vectors/syncorder.json`, `protocol/limits.go` (five §16.1 constants), `internal/transport/p2p/stream.go`,
`node.go`, `export_test.go`, `syncorder_test.go`, the model `docs/protocol/tla/SyncOrder*`, spec §8.7, plan, CHANGES,
TLA README, `scripts/tla-check.sh`, `scripts/netsim/mutants.toml`; PU1's step-2 code is in the same diff, reviewed in
`gates/PU1/SIMPLIFY.md`). Reviewers: two /simplify passes (reuse + simplification; efficiency + altitude), Sonnet /
medium each, read-only; findings triaged and applied by the step-2 session (Opus). The review that counts for the
merge is the integrator's (Opus or Codex).

## Appliqué

- `Order.Turn` removed: no product caller; `Ready` already says « its turn came » (the tests use `Ready`).
- `protocol/conformance/syncack.go`: the `map[bool]string{…}[b]` idiom (three copies, a map per call) folded into
  `pick`.
- `OnRefusal`: the wait computed by doubling up to `RetryMax` (`wait = min(2*wait, RetryMax)` per attempt) instead of
  a guarded shift: the guard's boundary was an equivalent mutant (survivor at the first gremlins pass).
- `Order.Ready` as a switch (`i < 0`, `i == 0`, the wait): the `i > 0` guard after `i == 0` was an equivalent mutant.
- Tests: `defer s.Reset()` → checked form (errcheck); `rawHost` wraps `rawHostWith` (the resource-manager arm passes
  `libp2p.ResourceManager`).
- Found by the frozen arm `drain_replace_reconnect` (2 of 6 runs red, `DEBUGCLOSE` trace): `watchOut` took a stream
  ended by its connection's close for a refusal when the swarm had not yet marked the connection closed (the remote's
  close reaches every stream first), and closed the peer (`PeerDown` on both sides). Root fix: a `network.ConnError`
  is a connection gone, like `IsClosed()`; 12 of 12 runs green after it.

## Codex code review of PU1+PU1b (2026-11-16, NOT MERGEABLE), fixed at the root, red first

- `Peek(1)` stopped watching after one byte: behind a head held open, a stream that sent one byte then was reset or
  ended kept its place until the 60 s with no frame delivered (P-9 says « delivered a frame »). Red:
  `TestSyncOrder/byte_then_end_while_waiting` (`pu1b-r3red.log`). Fix: the handler reads its stream from the start
  (frames charged to the receive budget as before) and keeps what it reads until the release (its peer announced,
  its turn come), which a helper goroutine waits for only while the stream waits; an end before any frame was
  delivered leaves the order at once, unless whole frames were read and the stream ended cleanly (`stay`: they are
  delivered at its turn, which the same arm checks). A first version read ahead in a second goroutine that lived as
  long as the stream: `generation_change`'s goroutine bound (+8) failed 5 runs of 10 (one more long-lived goroutine
  per stream, counted from a stack diff), so the reader is the handler itself. MQ2 and MQ5 now mutate `stay`.
- `IsClosed()` masked an explicit refusal: a remote that resets the stream then closes that connection while another
  survives was taken for a lost connection (no `ClosePeer`). Red: `TestOutEnded` (the decision as a pure function,
  `outEndedByConn`, the cheapest layer that keeps the defect: a two-connection scene cannot order the remote's reset
  and close deterministically). Fix: a byte after the ACK and a reset by the remote come first; only the connection's
  own close error (`network.ConnError`) or an end on a connection marked closed is the connection lost.
  `drain_replace_reconnect` and `generation_change`: 12 of 12 green with -race.

## Écarté (avec la raison)

- `Order.Wait` (two sources for the 60 s wait): node.go keeps `reorderWait = time.Minute`, the anchor of NI0's
  mutant M0b (`reorderWait = 0`); with `Wait` the sync lane still obeys that constant, so M0b keeps meaning something
  for `TestRelayThenDirect`. `export_test.go` exports `ReorderWait` for a test to compare.
- A single-scan `Ready`+`WaitLeft`, a per-`inbound` mutex, one reused timer in `waitTurn`, `Leave` returning a bool:
  at most two places per peer, one waiter; the costs are a few nanoseconds and one small allocation per stream end,
  not per frame.
- The release helper: one goroutine per stream only while it waits (waitUp, waitTurn), bounded by the places; the
  stream's reading stays in its handler.
- `<-ctx.Done()` when no retry fits: the opening ends with its budget (spec §8.7.4 « à son terme », the frozen arms
  `busy_forever` and `resource_manager_refusal` measure 15 s ± 10 ms), as a silent peer's does.
- One `dialCtx` helper for sync, bulk and lobby: outside the block. The bulk lane's own `bulkTail` chain (the same
  « wait for a stream that left » variant, Codex's code review): handled by block PU1c (plan §5), not here.
- Grouping `skipAck`/`beforeAck`/`afterAck` behind one hooks pointer: three unexported fields in the existing seam
  style; the frozen gates name them.

## mutation protocol/syncack

gremlins v0.6.0, 4 workers, coefficient 10: killed 49, lived 0, efficacy 100 % (`gremlins5.log`). Survivors at the
first passes (`int(a) < len` in `Action.String`, `i <= 0` in `WaitLeft`, `>= 0` in `Holds`) killed by tests
(`Action(3)`, `WaitLeft(head)`, `Holds(head)`); the two equivalent ones removed by rewriting (above). « Not covered »
are `case` conditions (gremlins' blind spot): `Ready`'s three negated by hand fail `go test ./syncack/`, as do the
Opener's (PU1).

## Vérifié

Red first: `TestSyncOrder`'s arms on PU1's round-1 code (`pu1b-gored.log`: 8 of 9 red for their reason, `at_bound`
green by design). Then: `TestSyncOrder`, `TestPeerUpAfterAck`, `TestPeerUpMoveAfterAck`, `TestPeerUpLatency` × 10
with -race; `./internal/transport/...`, `./internal/bridge`, `./internal/app/...`, `protocol/...` with -race
(`pu1b-stress.log`); golangci-lint on `internal/transport/p2p` and `protocol` (0 issues); MQ1–MQ5 caught; every
criterion of `gates/PU1.toml` and `gates/PU1b.toml` once (`gates-final.log`); after Codex's code review, again on
the final tree (`gates-final3.log`: all exit 0 but PU1b mutant, whose MQ2 edit removed the last use of `io` and failed
to build; the edit keeps `io` now, and the mutant criterion re-run alone catches MQ1-MQ5, `pu1b-mutant-r3.log`). Not run: `gates/PU1n.toml` (netsim,
scheduled by the integrator on an exclusive machine).

## PU1c (2026-11-17): the code above is replaced

The handler that read ahead and its release helper, `arriveLocked`/`leaveOrder`/`waitTurn` in `stream.go` and the
`stay` rule are gone: both lanes share `internal/transport/p2p/inbound.go` (one reader, a 1-byte probe, the budget
from the turn; review in `gates/PU1c/SIMPLIFY.md`). MQ1, MQ2 and MQ5 keep their ids, tests, clauses and texts,
re-anchored on `inbound.go` (MQ1: a successor that saw a stream ahead waits `reorderWait`; MQ2: a reset ignored while
waiting) and `protocol/syncack/lifecycle.go` (MQ5: an end with nothing probed keeps the place); MQ1–MQ5 caught again
(`pu1/c-mut2.log`). `TestSyncOrder/byte_then_end_while_waiting` (an arm added after the freeze, not in the gate's
list) follows P-11: one byte then an end of writing is kept to its turn, then leaves without delivering (a reset
after one byte still leaves at once). For the integrator: one red run of `drain_replace_reconnect` (« Send frame 105:
unknown peer ») in the first full pass of the criteria, not reproduced in 15 runs since (10 alone with -race, 5 as the
PU1b go criterion).

## Review follow-up (Codex, 2026-11-21)

The frozen `removal_while_waiting` arm still checks that both reset and empty FIN release the place within the
original bound and wake the successor. The separate `byte_then_end_while_waiting` arm checks a reset after a byte,
a truncated FIN prefix, and a full frame before FIN. Only the FIN cases keep a place until their turn.

The ten frozen criteria passed in the diagnostic replay on `f5d2b55` after the replacement-stream fix, including
the full order regression and MQ1–MQ5. The base guard still blocks acceptance. The diagnostic manifest is in the
evidence directory named in `gates/PU1c/STATE.local.md`.
