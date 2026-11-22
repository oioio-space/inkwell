# PU1b — The inbound order of a peer's sync streams: two places, removal with wake, capacity refusal

Date: 2026-11-16. Block PU1b, rigour **complete** (wire and state between peers; octets from a peer). Step 1 only:
rules, spec amendment, model, vectors, gate; no product code. Origin: Codex's code reviews of PU1 (rounds 1 and 2,
`scratchpad/codex-pu1code-report.md`, `codex-pu1code2-report.md`) and the design debate
(`codex-pu1design-report.md`, decision « CHANGE accepted »): the third finding in the same area (the order of a
peer's inbound streams across the move, rule P-6) calls for a design decision, not a patch (method v2).

## 1. The defect, stated

PU1 made the responder write the ACK before it waits for its own PeerUp (P-3). Round 1: the stream took its place in
the peer's order (`ps.inTail`) only after that wait, so the opener could move its session to the next stream while
the handler of the old one had not registered: the new frames overtook the old ones (red test
`move_keeps_order_across_late_handler`). The fix reserved the place before the ACK. Round 2: a place is a chain link
(`prev`, `mine`); a stream that ends before its turn hands the order on by waiting, without bound, for the stream
ahead of it. A Byzantine peer that holds a first stream open and resets every following one during its ACK makes
each failure keep a goroutine and a place for ever. `reorderWait` (60 s) bounds the wait of the *next reader*, not the
life of those links. Two facts are missing from the design: a place must be removable, and places must be bounded.

## 2. How others bound per-peer stream state (research)

| Stack | What bounds it | Source |
|---|---|---|
| QUIC (RFC 9000 §4.6) | `MAX_STREAMS`: a peer may open only as many streams as the receiver granted; past it, `STREAMS_BLOCKED`, the opener waits — the connection is not closed | [RFC 9000 §4.6](https://www.rfc-editor.org/rfc/rfc9000#section-4.6) |
| HTTP/2 (RFC 9113 §5.1.2) | `SETTINGS_MAX_CONCURRENT_STREAMS`; a stream over the limit is refused with `REFUSED_STREAM`, which tells the client the request was not processed and **may be retried** — a stream error, not a connection error | [RFC 9113 §5.1.2, §8.7](https://www.rfc-editor.org/rfc/rfc9113#section-5.1.2) |
| go-libp2p resource manager | per protocol × peer stream limits; a stream over them is reset with `StreamResourceLimitExceeded` (0x1002) | `core/network/mux.go`, `p2p/host/basic/basic_host.go` (`SetProtocol` failure) |
| libp2p error codes | stream reset codes carried by QUIC (RESET_STREAM) and go-yamux v5; the spec is open (libp2p/specs #479, PRs #622, #623) | [specs#479](https://github.com/libp2p/specs/issues/479) |
| Hypercore Protomux | a channel is paired by id; an unpaired open is dropped on close, nothing waits for it | [protomux](https://github.com/holepunchto/protomux) |

Every robust design (QUIC, HTTP/2) separates **capacity** (« not now, retry; nothing was processed ») from
**refusal** (« you are not welcome »): REFUSED_STREAM is explicitly retryable and leaves the connection up. PU1's
§8.7.4 closes the peer on any refusal, which is right for an admission refusal and wrong for capacity: refusing a third
place by a reset that the opener reads as a refusal would destroy a healthy session (Codex). Beyond the existing: the
signal need not be a new byte (PU1's corpus fixes every non-`0x01` byte as a violation); the reset's error code,
which QUIC and go-yamux already carry, is enough, and go-libp2p already uses `StreamResourceLimitExceeded` for its own
per-peer stream limit — so both limits read the same, retryable way.

## 3. Rules

| Rule | Statement | Proof (step 2) | Mutant |
|---|---|---|---|
| P-8 | **Places.** Per peer, at most **two** inbound sync streams hold a place in its order. A stream takes its place on arrival, before its ACK (P-6), and gives it back when it leaves the order (P-9); the count is per peer identity, shared by all its connections and by every generation of its state (PeerDown/PeerUp). A further stream is refused **for capacity**: reset with code `0x1002` (`StreamResourceLimitExceeded`), no byte, no `ClosePeer`, no event. The **opener** reads a reset with that code before the ACK as a capacity refusal: it resets its stream, keeps its session and connections, and opens again after a backoff (50 ms, doubled, at most 1 s), within the opening's single 15 s budget counted from the first open (P-5); any other reset before the ACK or an end is a refusal (P-4). Scope of the closes: a refusal, a violation or the end of the budget (capacity refusals included) close the peer (`ClosePeer`) for the opening of the session's stream or of a reopened one; for a move the same causes only abandon the move (session unchanged, no event, P-6); a capacity refusal never closes anything by itself. | `TestSyncOrder` arms `at_bound`, `bound_plus_one`, `recovery`, `busy_forever`, `resource_manager_refusal` (the remote's libp2p resource manager refusing the stream during its negotiation, same code), `drain_replace_reconnect`, `generation_change`, `byzantine_reset_loop`; vectors `syncorder.json` | TLA Q3 ClosePeerOnBusy (`SessionKept`), Q4 Bound3 (`Places`); Go MQ3, MQ4 |
| P-9 | **Removable order.** The streams holding places form a list in arrival order. A stream delivers frames only when no stream ahead of it holds a place, or after it waited 60 s (`reorderWait`: the order is given up, as today). A stream **leaves the list at once**, atomically and wherever it stands, when it ends without having delivered a frame (refused, its ACK failed, reset, closed for writing by its opener with nothing to read, or its connection gone before its turn); its successors are woken and wait for the stream now ahead of them, even if they were already waiting. A stream that delivered frames leaves once read to its end. Nothing waits on behalf of a stream that has left. What bounds the head's read is not time: it ends with its stream (the opener's end of writing, a reset, the connection's loss, the node's close); a Byzantine head held open costs its own peer one place and, after 60 s, the order — never a goroutine per failure. | arms `removal_while_waiting` (reset and empty end of writing), `byzantine_reset_loop` (places ≤ 2, goroutines bounded, counted), `generation_change` (PeerDown then PeerUp while an old handler holds a place), PU1's `move_keeps_order_across_late_handler` | TLA Q1 NoWake (`AllDelivered`), Q2 KeepEmpty, Q5 EofKeeps (`EmptyLeaves`); Go MQ1, MQ2, MQ5 |

**Why two places.** One is the stream being read; the second is the move's (or a reopened) stream, which must hold its
place before its ACK (P-6). A third is legitimately needed only when a drained stream, its replacement and a reconnect
overlap (Codex): the capacity refusal plus the opener's retry serialises them — the third stream gets its place as soon
as the drained one is read to its end or the lost one leaves (P-9), with no event. Crossed opens are unaffected: each
direction has its own order.

**Time in the model (Codex round 1).** `SyncOrder` gives each opening a clock (`spent`, ticks since its first open,
retries included, reset only by a new opening); a retry after a capacity refusal is possible only after a tick (the
backoff, `wait` → `due`), and at `spent = D` the opening ends as the spec says (session opening: `ClosePeer`; move:
abandoned). The exact waits (50 ms doubled to 1 s) are not in the model: the order and place properties do not depend
on their values, only on « at least one tick between a refusal and its retry » and « a finite budget », which the
model has; the values are fixed by the vectors (`sync_refusal`) and the Go arms. Liveness is checked under
`HonestTiming` (time passes only while neither the responder nor a due retry can act: the budget outlasts an honest
drain), safety also under any timing (`SyncOrderAnyTiming`, where the budget does run out:
`SyncOrderWitnessBudgetOut`).

**What the opener knows without the code.** A conforming responder never refuses a first stream of a fresh peer for
capacity unless the peer's earlier streams still hold places (a previous generation); the code is what distinguishes
the two refusals on the wire. A peer that fakes the code (Byzantine) gains nothing: retries stop at the 15 s budget,
then `ClosePeer`, as a silent peer (P-5).

## 4. Specification artefacts (step 1)

- Spec §8.7.3 (step 0: the place; capacity), §8.7.4 (two kinds of refusal), §8.7.5 (rows), §8.7.6, §8.7.9; §16.1 row
  « flux sync entrants par pair (§8.7) » with its constants in `protocol/limits.go` (`SyncInboundPlaces`,
  `SyncBusyCode`, `SyncBusyRetryMin`, `SyncBusyRetryMax`, `SyncReorderWait`); CHANGES.md entry.
- Model `docs/protocol/tla/SyncOrder.tla` (README « SyncOrder »): honest (move, drop, reopen: capacity reached),
  honest under any timing, and Byzantine (opens, writes, ends, resets, holds) openers; the opener's backoff (a tick
  before a retry) and its 15 s budget (`spent`, `D`; at its end a session opening closes the peer, a move is
  abandoned); `Places`, `Order`, `SessionKept`, `Budget`, `EmptyLeaves` (reset or empty end of writing),
  `AllDelivered`, `SessionResumes`; mutants Q1-Q5 and four witnesses. Its runs are named `SyncOrder*`, outside PU1's
  frozen `^PeerUpAck` set (19 ✓ unchanged); `PeerUpAck.tla` is untouched.
- Vectors `protocol/vectors/syncorder.json` (generator `protocol/cmd/vectors/syncorder.go`): `sync_order` (the
  responder's list: arrive, ack, deliver, end, expire; places, turns, refusals) and `sync_refusal` (the opener's
  reading of a reset: code → capacity or refusal). Status `proposed`, no `enforce` (step 2 adds the reference in
  `protocol/syncack` and sets `enforce`).
- Gate `gates/PU1b.toml` (≤ 45 min), red witnesses `gates/PU1b/WITNESSES.md`.

## 5. Variant handled by PU1c

[done] The bulk lane (`/inkwell/bulk/1.0.0`, `internal/transport/p2p/bulk.go`) orders its inbound streams with the same
« wait for the stream ahead » chain (`bulkTail`) that P-9 removed from the sync lane: a bulk stream that ends while
it waits keeps its handler and its link until the stream ahead ends or `reorderWait` (60 s) passes (Codex, code
review of PU1b). The bulk lane is outside PU1b's frozen spec (it opens only after PeerUp and holds no place under
P-8): the variant is **handled by block PU1c** (same removable order for the bulk lane: rule, tests, gate), specified
next, not by a residual here.

## 6. Open questions for the integrator

1. PU1b lands with PU1 (same candidate, PU1's code kept in the worktree) or right after it; `gates/PU1n.toml`
   (netsim) is then run once on the candidate that carries both (the order of the move is in scenarios 1, 2).
2. The code `0x1002` is go-libp2p's own; interoperability with other libp2p stacks is unspecified until
   libp2p/specs #622/#623 land (inkwell speaks only go-libp2p today).
3. Exigence and plan placement (`docs/exigences.md`, `coherence.md` §5, `PROGRESS.md`) are the integrator's.
