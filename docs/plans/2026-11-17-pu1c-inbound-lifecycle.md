# PU1c — One lifecycle for every inbound stream of the two ordered lanes (sync and bulk)

Status: step 1 (specify first, executable), 2026-11-17. Rigour: **complete** (bytes from a peer, state between peers,
the receive budget shared by every peer). Inputs: spec §8.1, §8.7.3–§8.7.6, §16.1; plans
[PU1](2026-11-16-pu1-peerup-after-acceptance.md) and [PU1b](2026-11-16-pu1b-inbound-order.md); the architect's draft
and Codex's verdict on it (CHANGE, every point applied, table in §7). Model:
[InboundLifecycle.tla](../protocol/tla/InboundLifecycle.tla). Vectors: `protocol/vectors/inbound.json`. Gate:
`gates/PU1c.toml`. PU1, PU1b, PU1c and PU1n land as one candidate; each gate runs once on it, none runs another.

## 1. The defect, stated once

Five findings of the PU1/PU1b code reviews have one cause: **a handler waits somewhere that does not observe its own
stream's end.** (1) the place taken after the up-wait (the late handler loses its rank); (2) a failed stream waiting
for its predecessor before leaving; (3) a `Peek(1)` watcher blind after its one byte; (4) `IsClosed` masking a refusal
(opener side, fixed in PU1b as a pure function, `TestOutEnded`); (5) a waiting reader blocked in `budget.Acquire`
(a `sync.Cond` with no cancel, `internal/transport/frame/budget.go`), which also lets waiting streams fill the budget
the head needs. The bulk lane has (1) and (2) again (`bulkTail`, `internal/transport/p2p/bulk.go`). Codex's review of
the draft added: the next probe must come **at most** T after the previous one; a read that returns bytes and an
error keeps the bytes; an end after a valid prefix must not drop whole frames; no late interrupt may hit a stream
that has started delivering; and on QUIC the waiting streams of **both** lanes can exhaust the connection's credit and
stop the head.

## 2. Facts the design starts from (read in the pinned sources)

- libp2p's `network.MuxedStream` has Read, Write, Close, CloseRead, CloseWrite, Reset, ResetWithError, Set*Deadline:
  **no Done, no context**. An end (reset, FIN, connection loss) is learnt only by a Read (or a Write).
- yamux v5 (`stream.go` Read): a reset is returned **even with data buffered**; a FIN only after the buffered data;
  with nothing buffered Read blocks on a channel that RST, FIN and data all signal; `SetReadDeadline` wakes a pending
  Read (`deadline.go`).
- quic-go v0.64.1 (`receive_stream.go`): a plain RESET_STREAM wins over buffered data; RESET_STREAM_AT (enabled by
  go-libp2p, `quicreuse/config.go: EnableStreamResetPartialDelivery`) delivers its reliable bytes first; a
  `SetReadDeadline` wakes a pending Read; a zero-length Read returns (0, nil) without looking at the FIN (no portable
  zero-length probe; no `Peek` on yamux nor on libp2p's stream).
- Flow control holds an unread sender: yamux's window is pinned at 256 KiB (`muxer.go`, SP1-5). QUIC: initial stream
  window 512 KiB, auto-tuned **only as the application reads** up to 10 MiB; connection window initial 768 KiB
  (1.5 × the stream window), auto-tuned up to 15 MiB (`quicreuse/config.go`).
- quic-go's connection credit (`flow_controller_base.go`, `flow_controller_connection.go`, `flow_controller_stream.go`):
  bytes **received and not read** on any stream hold connection credit; MAX_DATA is sent only once a quarter of the
  window was read (`WindowUpdateThreshold = 0.25`); a stream window grows without asking anyone, the connection
  window only through `AllowConnectionWindowIncrease` (go-libp2p: the resource manager, `transport/quic/conn.go`);
  the **initial** windows are never asked for (`connection.go`, newConnectionFlowController). Upstream go-libp2p
  exposes no option to set the quic.Config (`quicreuse/options.go`, master, read 2026-11-17).
- Honest timing: `frame.WriteIdle` = 90 s > `reorderWait` = 60 s: an honest opener held by flow control while its
  stream waits never fails its write before the stream's turn.

Prior art: Go's HTTP/2 server cancels the handler's context on RST_STREAM
(`net/http/internal/http2/server.go`, `processResetStream`), so the transport observes the reset for the handler;
POSIX offers `poll(POLLRDHUP)` and `MSG_PEEK`. libp2p offers neither, so the observer is built from Read itself, by
the stream's only reader.

**Impossibility (it fixes the contract).** On this API, « every end seen at once in every waiting state » and
« bounded bytes taken per waiting stream » cannot both hold: a Read returns buffered data before a FIN, and returns
data rather than blocking when data is buffered. What can hold, and is the contract: **at once** when no byte is
unread; **within T** for a reset (it wins over data on both muxers); **at its turn** for an end behind unread bytes
(FIN after bytes, RESET_STREAM_AT's reliable bytes).

## 3. Options compared

| | A. read everything into a per-stream buffer while waiting | B. budget wait cancelled by a reset observer | **C. not read before the turn, flow control holds the sender, a 1-byte probe at most every T (chosen)** |
|---|---|---|---|
| End seen while waiting | until the buffer is full, then blind (or unbounded memory) | circular: the only observer is a Read, and nothing reads while blocked | at once with nothing unread; ≤ T for a reset; at its turn behind bytes |
| Bytes held per waiting stream | W, unbounded under a slow consumer | the shared 256 MiB | ≤ 256 B probed, plus the transport window (accounted by the transport) |
| Shared receive budget while waiting | yes: waiters fill what the head needs | yes | **never** |
| Goroutines per placed stream | 2 (today) | 2 | **1** (the libp2p handler's) |

Wake-up variants for C (Codex: prefer the simpler if the contract holds): (C1) a waker sets the reader's deadline to
« now » on a turn or a PeerUp, with a flag and a re-check against lost wake-ups; (C2) **the reader alone sets its
deadline, at most T ahead, and re-evaluates turn, PeerUp and expiries when it fires**. C2 is chosen: nobody else
touches a deadline, so no interrupt can arrive late, during the transition to Delivering or after it (the class
Codex named disappears instead of being serialised). Its cost: a turn and a PeerUp are seen within T (250 ms), not
at once; the first frames of a stream placed before PeerUp wait at most 250 ms more. The head that is already first
and up when it arrives never waits. Rejected: C1 (a second writer of the deadline, a race to serialise).

## 4. Rules

T = `InboundProbeEvery` = 250 ms; `InboundProbeMax` = 256 B (≥ 1 + 60 s / T = 241); P = places per peer and per
lane = 2; L = lanes = 2; w0 = QUIC initial stream window = 128 KiB (set by the node); W = QUIC initial connection window = 1 MiB (set by
the node); θ = 1/4.

| Rule | Text | Proof (tests, vectors) | Mutants |
|---|---|---|---|
| P-10 | **One reader, explicit states.** Each inbound stream of the sync and bulk lanes has **one** reader for its whole life, the libp2p handler's goroutine (no watcher, no helper, no Peek). States: Arrived → Refused (no place: reset `0x1002`, no byte, nothing kept) or Accepting (place taken **on arrival**, before any wait, P-6; the ACK `0x01` written in ≤ 15 s; the end is seen by the write's error) → Waiting → Kept (ended, non-empty probed prefix) → Delivering ⇄ Blocked (the lane's receive budget is full) → Gone (leaves the order, wakes the successor, P-9; reset). Waiting ends at the turn (no stream ahead, or 60 s since arrival) once the peer is up; not up within 30 s of the ACK: Gone. The reader alone sets its read deadline: at most T ahead while Waiting, **cleared before Delivering** by the reader itself. | arms `matrix/*` (every state × every end, both lanes), `one_goroutine_per_place`, `interrupt_during_transition`, `wake_within_bound`, `immediate_when_first_and_up`; vectors `inbound-*-turn-needs-up`, `-up-wait-expires`, `-refused`, `-ack-fails` | TLA QC1 PlaceSyncLate (`Order`); Go MC10 |
| P-11 | **Probe, ends, prefix.** Waiting, the reader reads at most one byte per attempt; the next attempt starts **at most T** after the previous one started; it takes at most one byte per period and keeps at most 256 B. A read returning n > 0 bytes is handled **before** its error, every probed byte kept in order and given to the decoder first at the turn. A deadline error is « re-evaluate », never an end. A reset is seen within T (it wins over buffered data on QUIC and yamux); an end with **no unread byte** is seen at once: with an **empty** prefix the stream leaves at once (P-9, « sans trame à lire » means « sans octet non lu »); with a **non-empty** prefix it is Kept: its place is kept to its turn, then the decoder delivers every whole frame and drops a truncated last one (nothing more is delivered, no violation). An end behind unread bytes (FIN after bytes, RESET_STREAM_AT's reliable bytes) is seen at the turn. | arms `matrix/*/waiting_bytes/*`, `matrix/*/kept/*`, `probe_bound`, `probe_period`, `probe_bytes_before_error`, `valid_prefix_then_truncated`; vectors `inbound-*-waiting-*`, `-kept-prefix-at-turn`, `inbound-probe-*` | TLA QC2 FailedWaits (`EmptyLeaves`), QC3 ProbeOnce (`NoBlindWait`), QC5 KeepWholeOnly (`FinFramesDelivered`); Go MC2, MC3, MC5, MC6; PU1b MQ2, MQ5 re-anchored |
| P-12 | **Budget only from the turn; blocked readers.** No stream reserves or waits for the receive budget before its turn (sync: the 256 MiB shared by all peers; bulk: one frame). Delivering, each frame's cost is reserved before its payload is allocated (as today). Blocked on the budget, the reader is freed by room (its consumer), by the loss of **its own** connection (the reservation wait takes a context cancelled by that connection's `Disconnected`, never by another connection of the peer) or by the node's close; every reservation is returned on every exit. **Explicit resource limit:** its own reset is seen only once room is freed: while the local consumer is stalled there is no time bound, and the cost is that stream's place only (no goroutine, place or byte accumulates per failure). | arms `matrix/*/blocked/*`, `blocked_exact_connection`, `byzantine_floods` (budget back to its baseline); vectors `inbound-*-blocked-*` | TLA QC4 WaitCharges (`WaitNoBudget`), QC6 NoCancel (`BlockedFreed`); Go MC4, MC7 |
| P-13 | **QUIC connection credit for the head, accounted.** On QUIC every stream of a connection shares its credit, and bytes received and not read hold it. The streams not read (Accepting, Waiting, Kept, Blocked) of the two lanes on one connection are at most P × L − 1 besides a head, each holding at most w0 (never read past 256 B < θ × w0 = 32 KiB, so their stream window never grows nor updates). **Arriving and refused streams hold nothing on an honest peer's connection:** its opener writes only after the ACK (P-4, P-14), the model's invariant `ArrivingHoldsNothing`; a refused stream's bytes, if any, are released only when its reset is processed (a step of its own in the model, `Release`); a connection carries one peer, so a Byzantine peer that fills refused or unaccepted streams starves only its own heads — **without any time bound**: the head's progress is guaranteed for a **conforming** peer only, and a Byzantine lane can stop the heads of its own connection for as long as it holds that connection's credit (its own sessions, never another peer's). The node sets **w0 = 128 KiB and W = 1 MiB** with **(P × L + 1) × w0 ≤ (1 − θ) × W**: 640 KiB ≤ 768 KiB (the « + 1 » is a margin of one stream window). A head then always finds connection credit, however full the waiting streams of both lanes are. **Accounting:** the fork's QUIC transport reserves W in the connection's resource scope when the scope is opened, next to its existing `allowWindowIncrease` reservations; a refused reservation refuses the connection; the scope's `Done` releases it; at 128 connections that is 128 MiB of the node's 256 MiB, and the auto-tuned increases stay under the resource manager (quic-go's auto-tuning kept). **Cost:** 128 KiB per RTT per stream until its window grows, ≈ 1.25 MiB/s at 100 ms RTT (5 MiB/s with quic-go's 512 KiB), connection 10 MiB/s initially; auto-tuning doubles a window while it is read fast. **Explicit back-pressure limit across lanes:** a head whose own consumer is stalled holds its own stream window (≤ 10 MiB once auto-tuned); quic-go raises the connection window to 1.5 × that window only if the resource manager grants it, so the other lane on that connection can wait for that consumer — ended by room, the connection's loss or the node's close; guaranteed independence would need the lanes isolated in the transport (separate application credits would not isolate QUIC's credit). Configured in the product: the go-libp2p fork gets one `quicreuse` option (initial stream and connection receive windows) and the reservation, proposed upstream with the same patch (fork policy of block HP1); `New` passes `QUICInitialStreamWindow` and `QUICInitialConnectionWindow`; a compile-time assertion in `protocol/limits.go` fails the build if the inequality breaks. | arms `credit_exhausted_before_head` (an honest peer's opener fills one waiting sync stream, one waiting and one blocked bulk stream to 128 KiB each **before** the head writes; the head then delivers 4 MiB), `credit_witness_default_window` (the same with quic-go's 512 KiB / 768 KiB through a seam: the head stalls), `quic_window_reserved` (each QUIC connection's scope holds W from its start; with less than W left in the resource manager the connection is refused; the reservation is returned at close), `refused_stream_bytes_released` (a raw opener writes into a stream refused for capacity: the credit comes back once its reset is processed); vectors `inbound-quic-window-*` | TLA QC8 SmallWindow (`HeadProgresses`), invariant `ArrivingHoldsNothing`; Go MC8, MC11 |
| P-14 | **The bulk lane has the same lifecycle.** `/inkwell/bulk/2.0.0` (incompatible, clean switch: `1.0.0` no longer served nor opened): the same places (2 per peer, all connections and generations), the same ACK `0x01` (P-4) and order (P-6, P-9), the same refusal for capacity `0x1002`, the same states (P-10 to P-12). Without the ACK an honest reopen cannot be ordered: two handlers may start in either order (the model's first run found `dlv[bulk] = <<2, 1>>`). The bulk opener reads the ACK before its first frame, within 15 s of the open; a capacity refusal is retried as sync's (50 ms doubled to 1 s, within the 15 s); any other refusal, a violation or the budget's end fails that `Send` (stream reset, **no** `ClosePeer`, no event); the next `Send` opens a new stream. | arms `bulk_ack`, `bulk_order_across_late_handler`, `bulk_capacity`, `bulk_removal_while_waiting`, every `matrix/bulk/*`; vectors `inbound-bulk-*` | TLA QC7 BulkTail (`Order`); Go MC1, MC9 |

**Where the times are proved.** The model's temporal properties prove « eventually », not 250 ms: every time bound
(250 ms for a reset, a turn, a PeerUp and between probe attempts; at once when nothing is unread; the head already first
and announced reading at once) is proved by Go arms on controlled time (`testing/synctest`, simnet): `probe_period`,
`wake_within_bound`, `immediate_when_first_and_up`, `interrupt_during_transition`, `matrix/*/waiting_*/*`, and by the
vectors `inbound-probe-*`.

Frozen rules P-1 to P-9 are **unchanged in substance**; P-9's « fermé en écriture sans trame à lire » is made precise
by P-11 (no unread byte). PU1b's arms still hold: a waiting stream reset or closed without a byte leaves within 1 s
(T = 250 ms).

## 5. What a Byzantine peer can do, by state

| State | Attack | Bound |
|---|---|---|
| Arrived | open-reset loops | each leaves at once; ≤ 2 places per lane; no goroutine kept, no `ClosePeer` |
| Accepting | never read the ACK | 1 byte fits any window; ≤ 15 s |
| Waiting | hold silent / push data / reset / partial frame then FIN / RESET_STREAM_AT | its own place ≤ 60 s; ≤ 256 B probed, the rest held by its own flow control and its own connection's credit (P-13); reset seen ≤ T; ends behind bytes kept to its turn. It cannot touch the budget, other peers or its own head. |
| Delivering | flood valid frames | fills the shared receive budget (no per-peer share of the 256 MiB: pre-existing, flagged for a later block → TX1) |
| Blocked | reset unseen | P-12's limit: its own place only; its connection's loss or the node's close free it at once |
| Refused | many streams with data | each reset at once; on QUIC their bytes stay counted until its RESET_STREAM: only its own connection's credit |

## 6. Specification artefacts (step 1)

- Spec: §8.1 (bulk/2.0.0, ACK, places, lifecycle, refusal), §8.7.3 (lifecycle, probe, prefix, budget, the QUIC
  credit), §8.7.8, §8.7.9, §16.1 (new row « cycle de vie d'un flux entrant », QUIC windows row amended), §20.6;
  constants `InboundProbeEvery`, `InboundProbeMax`, `InboundLanes`, `BulkInboundPlaces`, `QUICInitialStreamWindow`
  = 128 KiB and `QUICInitialConnectionWindow` = 1 MiB in `protocol/limits.go` and `protocol/testdata/limits.json` (`TestLimitsMatchSpecification` green).
- Vectors `protocol/vectors/inbound.json` (33, proposed, not enforced: decoders `inbound_lifecycle`, `inbound_probe`,
  `quic_window` come at step 2 with the pure core in `protocol/syncack`: `Lifecycle`, `ProbeSchedule`,
  `QUICWindowOK`; `syncack.Order` stays where it is, MQ4's anchor in the frozen PU1b gate).
- Model `InboundLifecycle.tla`: both lanes **at once**, one shared connection credit; configurations Honest (both
  lanes, late handlers, reopens), Byzantine (a head and a waiting stream of one lane, every end kind, a stalled consumer,
  a connection loss; the rules are lane-parametric and the lanes' interaction is Honest's and Credit's),
  ByzantineSafety (three streams, a third place refused, a probe keeping a whole and a truncated frame), Credit (greedy streams of both lanes fill their windows before the head writes, the
  bulk head blocked); mutants QC1–QC8 and four witnesses (README « InboundLifecycle »).
- **Composition of the two lanes (over-approximation, Codex round 1).** A direct model of two Byzantine lanes on one
  credit does not finish (≥ 700 000 states and growing with the smallest constants: one unit of window, one frame per
  lane, no `Stall`, no loss), and an explicit environment process taking any credit (tried: `Env`, any amount up to
  `ConnWindow` at any time) adds states but no behaviour. The argument, checked on the model's text:
  (1) the lanes share **only** the connection's credit — each lane has its own order (`list[l]`), budget (`used[l]`),
  consumer (`Consume(l)`) and frame numbers; `up` and `alive` are global inputs every configuration leaves free; no
  action of a lane reads the other lane's variables except through `SumQ(S)` in `ConnRoom`;
  and no action branches on a lane's name (the names appear only in `Lanes`, `Lane` and `LaneIds`, which pick the
  indices; both lanes start in `accepting` since bulk has the ACK), so the one-lane check of the sync lane is the check
  of either lane;
  (2) `ConnRoom` guards **only writes** (`HWrite`, `GWrite`, `BWrite`); no reader action (arrive, ACK, probe, turn,
  read, block, unblock, release, remove) needs credit;
  (3) in the Byzantine configurations every write is an opener's choice **without fairness**, so for any behaviour of
  the composition, its projection on one lane — the same steps of that lane, with the other lane's steps removed and
  some writes not taken — is a behaviour of the one-lane model with that lane's credit unconstrained by the other: the
  other lane holding credit can only disable writes, which the one-lane model may already decline;
  (4) every property of the Byzantine configurations (`Places`, `WaitNoBudget`, `ReservationsReturned`,
  `FinFramesDelivered`, `EmptyLeaves`, `NoBlindWait`, `BlockedFreed`) is about one lane's own streams, and the liveness
  ones rest only on weakly fair reader actions, which (2) makes independent of credit; so they hold of the composition.
  What does depend on credit, `HeadProgresses`, concerns an honest peer (a connection carries one peer) and is checked
  with both lanes modelled concretely (Credit, and Honest for order and liveness).
- Gate `gates/PU1c.toml` and red witnesses `gates/PU1c/WITNESSES.md`.
- Go mutants (`scripts/netsim/mutants.toml`, block PU1c, written at step 2 with their anchors): MC1 bulk place after
  the up-wait, MC2 probe stops after a byte, MC3 probe period 2 T, MC4 a waiting stream charges the budget, MC5 a
  prefix kept only without a truncated tail, MC6 bytes dropped when a read also returns an error, MC7 the budget wait
  not cancelled by the connection's loss, MC8 the QUIC option dropped, MC9 a third bulk place, MC10 the probe deadline
  left armed into Delivering, MC11 the initial window not reserved with the resource manager. PU1b's MQ1, MQ2, MQ5 are re-anchored on the new code (same ids, tests, clauses, texts).

## 7. Codex's verdict on the draft, point by point

| Point | Applied |
|---|---|
| Next probe **at most** T after the previous one | P-11; vectors `inbound-probe-period`, `-late`; arm `probe_period`; Go MC3 |
| n > 0 before the error, every byte re-injected in order | P-11; vector `inbound-*-waiting-n-with-error`; arm `probe_bytes_before_error`; Go MC6 |
| Serialise interrupt, disarm and deadline changes; or the simpler periodic deadline | C2 (§3): only the reader sets deadlines, cleared before Delivering; arm `interrupt_during_transition`; Go MC10 |
| Any EOF with a non-empty prefix waits for its turn, then the decoder | P-11 Kept; TLA QC5 (`FinFramesDelivered`); arm `valid_prefix_then_truncated`; Go MC5 |
| QUIC: prove a credit reserve for the head, with exhaustion **before** the head sends | P-13 (window derived and configured); TLA Credit + QC8; arms `credit_exhausted_before_head`, `credit_witness_default_window`; Go MC8 |
| Both lanes share the credit: one model with both lanes at once | `InboundLifecycle.tla` has both lanes and one `ConnWindow`; Honest and Credit instantiate both concretely; the Byzantine configurations take one lane and the other as `Env`, an over-approximation of any lane through the shared credit (§6, « Composition of the two lanes ») |
| Round 1 (DO NOT FREEZE): memory, Byzantine composition, the arriving bound, time bounds, the tla cap | P-13 rewritten (128 KiB / 1 MiB, reserved, cost stated); the composition by over-approximation (`Env`, §6); `ArrivingHoldsNothing` and the `Release` step; « Where the times are proved » (§4) with `immediate_when_first_and_up`; the tla criterion's cap = TLC time with margin + the slot wait bound |
| Own reset seen after room: explicit limit; cancellation tied to the exact connection; every reservation returned | P-12; TLA `BlockedFreed`, `ReservationsReturned`, QC6; arm `blocked_exact_connection`; Go MC7 |
| Scenarios: credit exhausted, valid prefix then truncated, interrupt during the transition | arms of the same names |

## 8. Decisions taken here (for `docs/autonomous-decisions.md` at step 7)

1. **C2 wake-ups (≤ 250 ms) instead of interrupts.** Alternative C1 rejected (a second writer of the deadline).
2. **Bulk gets the ACK (`/inkwell/bulk/2.0.0`).** Found by the model: without it the order across a reopen is not
   provable. Alternative « bulk order best effort » rejected: the integrator asked for the P-6 race fixed.
3. **QUIC initial windows 128 KiB per stream and 1 MiB per connection, reserved with the resource manager** (Codex,
   round 1: the first draft's 4 MiB per connection outside any accounting, 512 MiB at 128 connections, was refused).
   Alternatives: 4 MiB unaccounted (refused); initial = maximum = 16 MiB (covers an auto-tuned head of the other lane,
   16 MiB per connection); pinning the stream window like yamux (no auto-tuning). Cost stated: ≈ 1.25 MiB/s per stream
   at 100 ms until auto-tuning grows the window. The cross-lane stalled consumer is an explicit back-pressure limit.

## 9. Sources

- go-libp2p fork `oioio-space/go-libp2p v0.51.1-0.20261109121346-cae49c803966`: `core/network/mux.go`,
  `p2p/transport/quicreuse/{config,connmgr,options}.go`, `p2p/transport/quic/{transport,conn}.go`,
  `p2p/host/resource-manager/limit_defaults.go` (connection 32 MiB, system scaled to inkwell's 256 MiB).
- quic-go v0.64.1: `receive_stream.go`, `flow_controller_base.go`, `flow_controller_connection.go`,
  `flow_controller_stream.go`, `connection.go`, `internal/protocol/params.go` (`WindowUpdateThreshold = 0.25`,
  `ConnectionFlowControlMultiplier = 1.5`).
- go-yamux v5: `stream.go` (Read), `deadline.go`.
- RFC 9000 §4 (flow control), §4.1 (MAX_DATA), §19.4 (RESET_STREAM); draft-ietf-quic-reliable-stream-reset
  (RESET_STREAM_AT).
- Go `net/http/internal/http2/server.go` (`processResetStream`); POSIX `poll` (`POLLRDHUP`), `recv` (`MSG_PEEK`).
- upstream go-libp2p master `p2p/transport/quicreuse/options.go` (no quic.Config option), read 2026-11-17.
