---
status: proposed
date: 2026-11-09
amends: 0034 (revision 10; steps 4-6)
decision-makers: [Mathieu]
---

# One send path: receiver-consumed progress, one credit per hop, one cut timer, resume by hash, a typing lane

## Context and problem

Requirement W38 asks to re-specify the whole peer send path formally. Design:
[2026-11-09-send-path-design.md](../plans/2026-11-09-send-path-design.md) (RC1-RC10, SP-1..SP-9, E1/E1b/E3).
The evidence chain, on scenario 7 (Alice, QUIC, relay at 8 KiB/s, QUIC, Bob; Noise and yamux end to end):
- **RC1, E1:** a sender's `Write` sees progress only in update quanta `U` = 128 KiB (yamux sends an update at half
  of its 256 KiB window; quic-go at 25 % of 512 KiB). Stalls measured at `U/R` + 9-12 %, up to 21.8 s at 8 KiB/s,
  so sync/1's 20 s `WriteIdle` cuts an honest link; with a 512 KiB window the stall point and quantum double.
- **RC3, E3:** a whole window sits in the relay path (252 KiB in flight while Bob had consumed 0); typed frames
  queue behind it whatever the stream: NI-3 C2 worst 44.8 / 53.3 / 49.0 s on seeds 1-3, with 423 qdisc drops.
- **RC8, RC9, E1b:** the muxer's own timers (`ConnectionWriteTimeout` 10 s: write valve and ping timeouts) close
  the *whole session*: 18 of 36 runs dead (3/18 with a 256 KiB window, 15/18 with 512 KiB); 6/6 alive at 120 s.
- **E3:** go-yamux rejects any window under 256 KiB (`VerifyConfig`), as the yamux spec fixes it at 256 KiB.
ADR 0034 §10.2 says « credit = the transport's, per stream » and « 60 s without progress », without saying *whose*
progress (RC7): as written, v2 inherits RC1 and RC3. Steps 4-6 are not implemented, so amending now is cheap.

## Decision drivers

Full Byzantine threat model (N7); ADR 0028 (a slow peer finishes; typing is not delayed by a large transfer);
one mechanism, one meaning per timer; every rule checked by a model with mutants before code; reuse (N15).

## Decision

Rules SP-1..SP-9 are the **normative send path of sync/2** (spec 2.0.0-brouillon.11). Constants are public (§16.1):
`C` 8 KiB, `T_ack` 1 s, `P` 32 KiB, `T_p` 40 s, `F` 2 KiB/s, `D` 5 s, `Wmax` 128 KiB (yamux) / 384 KiB (QUIC).

- **SP-1 Progress = what the receiver consumed.** `consumed` is the offset the receiver took into its reservation,
  stated by the receiver. « The `Write` returned » never counts, on any lane (Aeron status messages).
- **SP-2 One receiver-granted credit.** New critical frame `LARGE_CREDIT{gen u32; id u32; consumed u32; window
  u32}` (0x0C, CONTROL); `LARGE_REPLY` gains the same two fields (body 9 → 17 B). The sender keeps
  `sent ≤ consumed + window` (A.1 and RFC 9000 §4 vocabulary: absolute, monotone; a regressing credit is ignored).
  The receiver reports every `C` consumed and at most `T_ack` after any unreported byte. Sizing:
  `window = clamp(rate × max(1 s, 2·RTprop), 2C, Wmax)`; `rate` = windowed maximum arrival rate. **Correction to
  the design:** `RTprop` is the 10 s windowed *minimum* RTT (BBR's filter), never a smoothed RTT: a queue-inclusive
  RTT grows with the window it sizes (self-inflicted bloat, a positive feedback loop).
- **SP-3 The transport never binds an honest peer.** `Wmax = (1 − θ)·W_init`: one window binds per hop, by
  construction; the transport windows only stop a Byzantine peer. The cap is **per session**: the windows granted
  on LARGE and bulk (and REPO if Q1 is accepted) sum to at most `Wmax`.
- **SP-4 One cut timer.** One-shot `T_p`, re-armed each time `consumed` (sender) or the received count (receiver)
  advanced by `P` since the last arming, only while bytes are outstanding; firing resets the stream. Derivation:
  `P/F + 2D + T_ack` = 27 s, × 1.2 ≤ 40 s. Replaces LARGE's « 60 s without progress » and bulk's « 30 s without a
  byte ». No size-dependent total timer.
- **SP-5 Resume from the receiver's offset, keyed by hash.** The receiver keeps the prefix of `(peer, root)` in that
  peer's fixed 4 MiB share (§10.6) for 10 min or until that peer's next LARGE with another root; never shared
  across peers. `LARGE_REPLY.consumed` is the offset; the preamble becomes `gen; id; size; offset`. BLAKE3 over the
  whole object must give `root`, else a violation charged to that peer and the prefix dropped.
- **SP-6 Rule L unchanged in shape.** A current-epoch LARGE cut or reset is a loss (rule L); with SP-5 a retry
  resends at most what was outstanding (≤ `window`), not `S`. Inside the contract no cut occurs, so no resend loop.
- **SP-7 Typing lane.** Interactive REPO frames have priority in the sender's per-peer scheduler (§7.4) over every
  bulk byte not yet written; the LARGE writer writes in `C`-sized calls. Bulk bytes ahead of a typed frame anywhere
  on the path are then ≤ the session's granted windows + one `C` write, so
  `L_int ≤ D_q + (Σwindow + C + I)/R + rtt` (`D_q`: queueing from traffic we do not send). Sender-side priority
  alone (RFC 9218) cannot reorder bytes already in a relay; the credit is what keeps that queue short.
- **SP-8 Byzantine bounds.** `consumed > sent` is a violation (`#large-credit-beyond-sent`); writing beyond
  `consumed + window` is one (`#large-over-credit`, checked by the receiver against what it granted); an offset
  beyond what the receiver holds is one (`#large-resume-beyond`); credits of another epoch or an unknown `id` are
  ignored (rule T). **Tightened:** an honest receiver sends a credit only on a `C` advance or a `T_ack` firing, so
  more than `⌈size/C⌉ + ⌈elapsed/T_ack⌉ + 2` credits for one object is OVERLOADED (exact, no per-second guess).
- **SP-9 The session's own timers never cut an honest link.** The muxer's write valve and ping timeouts are a stated
  constant `T_mux` ≥ 2 × the longest honest blocking of a frame write and ≥ `L_int + D` for a ping (the valve fires
  as early as `T_mux/2`); proposal `T_mux` = 40 s on sync/2 (Q2); keepalive 30 s; window pinned at 256 KiB.
  The hop margin (`Win ≤ H − Uh + D`) is a margin, not a proof: a relay's buffer is not ours to know.

### What SP1b already ships on sync/1, and what it leaves

Ships (gate `gates/SP1b.toml`): `WriteIdle` 90 s per fragment `Write`; the WB-2 floor `WriteIdle + S/2 KiB/s`;
`partialTTL` 120 s; yamux `ConnectionWriteTimeout` 120 s, keepalive 30 s; stream window pinned at 256 KiB.
Proved (SP1* configurations): no false cut and one attempt at 8 KiB/s and at the floor; no session loss at 8 and
6 KiB/s and at the floor through a 128 KiB hop; a 50 s silence loses nothing.
Leaves (residual witnesses that must keep violating): typing on the relay, 36 s in the model and 45-53 s measured
(NI-3 C2 a declared known failure); the keepalive at the floor through a relay buffering a whole window (needs
140 s); a lost session resends the whole message; a 60 s silence with a blocked send loop closes the session.
These sync/1 constants stay until the switch (ADR 0034 step 10) and are then deleted with `frame` and `mux`.

### Changes to ADR 0034 steps 4-6

Order unchanged (4 → 5 → 6, then 8-10). New prerequisite **3b**: this ADR accepted, spec amended (list below),
model extended (Confirmation), E4 measured (credit over a reverse stream on the netsim) before `C`/`Wmax` freeze.
- **Step 4** gains: codecs for the extended 0x0B, 0x0C and the LARGE preamble with `offset`; a pure send-path core
  in `protocol/sync2` (SP-2/3/4/8, time injected, no I/O); rapid `StateMachine` tests against SendPath's rules;
  `protocol/libp2p` states the muxer config (`T_mux`, keepalive, pinned window) as exported constants, moved
  from `internal/transport/p2p/muxer.go`.
- **Step 5** scope: the bulk requester's outstanding ranges are its credit, drawn from the same session budget;
  bulk's « 30 s without a byte » and the server's `WriteProgress` are replaced by SP-4.
- **Step 6** scope: LARGE lane with credit, the SP-5 prefix store (Go, opaque bytes, ADR 0011), the typed-first
  scheduler (SP-7); webview replay of the unfinished object only if E5 and Q3 ask for it; NI-3 C2 becomes strict.

## Consequences

- **Security (full Byzantine, N7).** A reader that drips holds only its own LARGE lane and its own share, at most
  `(S/P + 1)·T_p` ≈ 86 min for 4 MiB; typing to it and every other peer are untouched. A writer that drips into
  our reservation is cut by the receiver-side SP-4. A huge granted window only costs the granting receiver's own
  latency. A forged prefix fails BLAKE3 at the end and costs only its sender's share; per-peer keying rules out
  cross-peer poisoning. A relay can only delay or drop (Noise end to end): it looks like a slow path or a lost
  session; a relay that keeps both QUIC hops alive but black-holes the circuit is noticed by SP-4 or the keepalive
  within `T_mux`, not by QUIC's 30 s idle timeout. New decoders (0x0B', 0x0C, preamble) get fuzz targets (P-FUZZ);
  `security-auditor` reviews SP-5 and SP-8 (P-SEC).
- **Performance.** One LARGE moves at most `Wmax`/RTT: 1.25 MiB/s relayed at 100 ms RTT, 3.75 MiB/s direct; a
  4 MiB object then takes ≤ 3.2 s, accepted for ≤ `r` objects. Credit overhead is one 17 B frame per 8 KiB
  (0.2 %), but 12 800 frames/s at 100 MiB/s: if benchstat shows CONTROL cost, the report step becomes
  `max(C, window/4)` (Aeron's 25 %), which changes no bound. No total timer: a slow honest link finishes.
- **To measure, not assumed:** E2 (the QUIC quantum on a direct path), E4 (no `Write` blocked > `C/R + 2D`; typed
  frame within SP-7; qdisc drops vanish once ≤ `Σwindow` is in the path), E5 (root stability across epochs, feeds
  Q3), benchstat for `protocol/sync2` and `internal/transport/session` (P-PERF); `tc` backlog at 1 Hz in NI-3.
- **Less good:** one more critical frame, a larger LARGE_REPLY, a receiver-side rate estimator.

## Alternatives rejected

- **Stacked windows** (transport, mux and application windows, the smallest wins): which one binds is an accident
  of the path; the hop below the stream window binds and stalls the send loop (mutant `SendPathStackedWindows`
  breaks `NoSessionLoss`; E1b's write-timeout deaths).
- **Estimate-based budgets at the sender (block WB):** the sender sees progress only through `Write` returns, in
  `U` quanta; `WriteBudget.tla` had no window and no reverse path, so `Delivered` held in the model and failed on
  the netsim (RC6). A sender-side BBR estimator has the same blind spot; the receiver sees the arrivals.
- **Forking go-yamux for a window < 256 KiB:** the yamux spec fixes 256 KiB at stream start, so other libp2p
  implementations would mis-account; it still leaves the relay buffer, the QUIC hop and the muxer timers (E3).
- **QUIC only:** a circuit-v2 relayed connection is a stream upgraded with Noise and yamux; there is no QUIC over a
  circuit, so dropping yamux drops relays (ADR 0028, symmetric NATs). QUIC's 512 KiB window has the same `U`.
- **Delay-based scavenger (LEDBAT) for bulk:** needs one-way delay estimates and suffers latecomer unfairness; we
  own both lanes, so a fixed latency budget (SP-7) is simpler and provable.

## Confirmation (executable acceptance)

- **Model** `docs/protocol/tla/SendPath.tla`, `SendPathHonest`, `SendPathHop`, `SendPathAny`, exhaustive with
  liveness: `NoFalseCut`, `NoSessionLoss`, `WindowNeverBinds`, `InteractiveBound`, `StopCut`, `HoldBound`,
  `BoundedWaste`, `CreditSane`, `NoSilentLoss`, `DeliveredHonest`, `CutEventually`; `ControlLive` via ConnCredit.
- **Mutants**, each caught by `mise run tla`: WriteReturn, FixedFloor, NoCap, CapAboveWindow, SharedWriter,
  FromZero, StackedWindows, MuxWriteTimeout, KeepaliveBehindData, SilentAbandon, NoCut, NoCutLive, ZeroOnly,
  TrustCredit. **New before step 4:** `SendPathSmoothedRtt` (window sized on a queue-inclusive RTT) breaks
  `InteractiveBound`; `SendPathPerObjectCap` (two lanes each at `Wmax`) breaks `NoSessionLoss`;
  `SendPathTrustOffset` breaks `CreditSane`.
- **Model extensions before step 4** (README « Not covered »): loss and retransmission on the hop (E3's gap
  between 36 s and 45-53 s), two lanes on one session budget, the receiver-side SP-4 (writer drip), Byzantine
  window updates.
- **Netsim, seeds 1-3:** NI-3 C1 pass, C3 « resyncs 0, 0 », C4 pass, and **C2 strict: every typed token seen within
  10 s** (model `Lmax` at 8 KiB/s with `Σwindow` = 16 KiB is ≈ 2 s + 2D; the 10 s is the plan's bound, Q4);
  NI-2 blackout delivered without resync; NI-1, NI-4 and scenarios 1-8 do not regress. **New NI-5:** an 879 KiB
  paste over a relay with go-libp2p's default circuit limits (128 KiB, 2 min) completes through SP-5, units sent
  ≤ `S` + epochs × `Wmax` (`BoundedWaste`).
- **Unit and vectors:** one test per SP rule named after it (P-BORNES rows for `C`, `P`, `T_p`, `T_ack`, `Wmax`,
  the credit bound); vectors for 0x0B', 0x0C and the preamble in Go and TypeScript (P-VEC); gremlins ≥ 95 % on
  `protocol/sync2`.

## spec.md sections to amend (not edited by this ADR)

§5.1 (LARGE preamble `offset`; the muxer config: pinned window, `T_mux`, keepalive) · §5.2 (0x0B body 17 B; new
0x0C) · §5.5 rule L (« no progress » defined by SP-4; cost via SP-5) · §5.6 point 3 (SP-7 bound replaces « does not
delay typing ») · §7.2 points 2, 3, 5 (credit, offset, BLAKE3 over the whole object, `T_p`) · §7.4 (typed-first
scheduler, waiting for credit) · §8.1 (bulk: ranges as credit, SP-4) · §10.2 (SP-1, SP-3; « credit = the
transport's » no longer for LARGE and bulk) · §10.6 (prefix store in the fixed share, 10 min) · §10.7 (credits on
CONTROL, always read) · §11.2 (LARGE states with offset and credit) · §12 (order of the credit checks) · §13 (new
violation codes) · §16.1 (rows « progress of a LARGE stream » and « bulk 30 s » replaced; new rows `C`, `T_ack`,
`P`, `T_p`, `F`, `D`, `Wmax`, credit bound, `T_mux`, prefix) · §17 (drip and relay bounds) · §20.1 (0x0C) · §21
(SendPath, vectors) · §22 (Aeron, RFC 9000 credit, resumable upload) · CHANGES.md and the draft revision.

## Open questions for the user

1. **Q1, REPO under the credit.** REPO is paced only by the transport window: an initial sync burst puts up to
   256 KiB ahead of typing (32 s at 8 KiB/s). Recommended: a lane field in the credit (`REPO`, `LARGE`) so REPO
   draws on the same session budget (backpressure only, never a cut); otherwise SP-7 holds behind LARGE and bulk only.
2. **Q2, `T_mux` on sync/2:** 40 s (faster detection of a black-holing relay) or SP1b's 120 s; after E4.
3. **Q3, webview replay** of the unfinished object so its root stays stable (decision 5), after E5.
4. **Q4, the strict C2 bound:** 10 s (plan) or a tighter one once E4 measures the credit path.

## More information

RFC 9000 §4; RFC 9218; RFC 6817; Aeron (https://github.com/real-logic/aeron/wiki/Flow-And-Congestion-Control);
BBR: draft-ietf-ccwg-bbr, Cardwell et al., ACM Queue 14(5) 2016 (https://queue.acm.org/detail.cfm?id=3022184) and Linux
`tcp_bbr.c` (min-RTT window 10 s); yamux spec (https://github.com/hashicorp/yamux/blob/master/spec.md); circuit v2
(https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md); draft-ietf-httpbis-resumable-upload-15; go-yamux
v5.1.0, quic-go v0.64.1, go-libp2p v0.51.0 (design §1). Reviews before acceptance: Codex and `security-auditor`.
