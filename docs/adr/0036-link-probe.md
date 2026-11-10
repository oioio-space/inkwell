---
status: proposed
date: 2026-11-09
amends: 0034 (spec revision 11); W33 link metrics contract v1 (Annex A.1, quality and lossPermille)
decision-makers: [Mathieu]
---

# Link loss is counted by the receiver: protocol /inkwell/linkprobe/1.0.0

## Context and problem

W33 shows a person how good each link is; one of its figures is the share of packets the network loses.
Today that figure is quic-go's `PacketsLost / PacketsSent` over 30 s (`internal/transport/p2p/links.go:152`,
`linkstat.go:84`). Block C5S (docs/plans/2026-11-09-c5-sound.md, sections 1-2) showed it is not a loss rate:

- `PacketsLost` counts **declarations** (RFC 9002 §6.1). With quic-go v0.64.1, a declaration later proven
  spurious by an ACK is never subtracted (`congestion/cubic_sender.go:222` only adds), and ECN-CE marks count as
  losses (quic-go #6167). Under the 5 ms jitter of the netsim `wifi-clean` profile, the initial 9/8 RTT
  threshold declares about 2 reordered packets lost per 30 s window: the « P3 at 0.7 % » failures of NI-1 C5.
- `PacketsSent` counts every packet, ACK-only ones included, whose loss the sender can never detect (RFC 9002
  §2): the figure reads about 2/3 of the real loss in two-way typing.
- quic-go merged the subtraction of spurious losses on 2026-11-08 (PR #6169, unreleased), but no version counts
  the ack-eliciting packets sent, the only denominator that matches `PacketsLost`.

The figure must mean: **the share of the packets this endpoint sent to the peer on the connection, over the last
30 s, that the peer did not receive intact.** Only the receiver knows that.

**Revised on 2026-11-10 (user decision, after three independent reviews).** Revisions 1 and 2 of this ADR promised a
figure accurate to a few permille and added MUSTs to make it hold (the PROBE at the highest send priority, the
counter read in the step that delivers the PROBE). Review 3 showed that the first cannot be implemented on our
stack and that the model assumed its own result, and that the permille numbers rested on a single seed (see
« Levels »). Following the project rule (when it gets hard, redefine the problem), the user changed what is
promised: **the person sees the quality of a link in three levels, good, fair or poor, plus unknown**, as W33 asks
(simple, no jargon); the permille precision promise is dropped. The figure stays as a diagnostic.

## Decision drivers

- What the person needs (W33): a simple and honest level (good, fair, poor, unknown) that follows the network's
  loss, not a sender's guess, and does not flicker.
- No dependency on an unreleased or patched quic-go, and nothing that go-libp2p v0.51 does not expose
  (`quicreuse` sets `quic.Config.Tracer` itself: no in-process qlog).
- A protocol others can implement from the spec alone, with conformance vectors and a model (ADR 0034's bar).
- Full Byzantine writers (spec §17): a lying peer must not hurt anyone but its own figure.
- Statistical power of the end-to-end check (C5): fewer packets needed for the same rule.
- No new information leaks; no per-packet cost on the hot path.

## Considered options

- **D1, sender side.** `PacketsLost` with #6169, over an ack-eliciting sent counter to be added upstream.
  It needs a quic-go release that does not exist yet and a new upstream API; it still estimates from the
  ack-eliciting packets only (a sample of 2/3 of them), so its residual against the network truth is 1.85
  times wider (σ_z 1.14 against 0.62; `scripts/netsim/c5model_test.py`, `test_d1_needs_several_times_the_traffic_of_d2`) and the same test needs about 3.4
  times the traffic.
- **D2, receiver count (RTCP receiver report style, RFC 3550 §6.4.1).** The sender sends a small PROBE once a
  second and notes how many packets it had sent; the receiver answers with how many of the sender's packets it
  has received. Over two answers 30 s apart, loss = 1 − Δreceived / Δsent. Works with quic-go v0.64.1's own
  counters (`ConnectionStats().PacketsSent`, `.PacketsReceived`, which skips duplicates and undecryptable
  packets), is immune to spurious declarations, ECN-CE and ACK-only dilution, and measures what netem's
  counters measure.
- **Keep the declared figure and widen C5's interval.** Rejected: it hides a wrong number; the person keeps
  seeing ≈ 2/3 of the loss and phantom loss on clean links.

## Decision

Chosen option: **D2**, as the standalone protocol **`/inkwell/linkprobe/1.0.0`**, specified normatively in spec
§8.5, because it is the only option that measures the network's own loss without a dependency we do not
control, and it needs the fewest packets to be checked. Its normative output is a **level** (spec §8.5.5 step 6;
« Levels » below), computed from the loss figure and the round trip of the window.

### Place in ADR 0034's layering: its own protocol id, not a frame kind

The probe is a libp2p protocol of its own, next to `/inkwell/bulk/1.0.0` and `/inkwell/lobby/1.0.0` (spec §2,
§20.6), and not a frame type on the CONTROL stream:

- it measures **one QUIC connection**, so its stream must be opened on that connection (`Conn.NewStream`); a
  CONTROL frame rides whatever connection the session uses, and the session moves between connections;
- it is independent of the session: no HELLO, no capability, no generation; it works on today's 1.x stack
  (`sync/1`) and on 2.0 alike, and toward a relay, which speaks no inkwell session;
- its failures stay local: a violation resets the probe stream only, never the session (spec §8.5.8);
- its versioning is multistream-select's, so a peer without it answers `na` and the figure is simply unknown
  (graceful absence, spec §8.5.9). It is not in `core/2`: it carries no security guarantee to negotiate.

### Wire format, rate, window, late echoes (normative text: spec §8.5.3 to §8.5.7)

- One stream per direction and connection, opened by the measuring side (the prober) on the connection it
  measures; the negotiation finishes before the first PROBE, which goes out at once; `na` means never again on
  that connection. Messages: `len u8 (1..64) ; type u8 ; body`. PROBE `0x01`: `seq u32` (≥ 1), len 5. ECHO
  `0x02`: `seq u32 ; received u64`, len 13. Types `0x40`-`0x7F` are ignorable (skipped, no state, counted against a budget of one per critical message read plus one),
  `0x03`-`0x3F` critical (unassigned: a violation), `0x00` and `0x80`-`0xFF` reserved. Binary, not CBOR: two
  fixed-size messages need no schema, and the layout follows the frame convention of §5.2.
- One-message writer: a message is handed to the transport whole; while it is not fully accepted, the next
  PROBE is skipped and the next ECHO dropped (no credit spent): at most one ECHO per read. Nothing is ever queued.
  Probe streams are always read, writer busy or not (F2). A higher send priority is allowed (MAY), never required
  (review 3, F1: go-libp2p v0.51 does not expose it, and quic-go v0.64.1 sends retransmissions and ACK-only packets
  before it anyway).
- Timeouts (F6, F14): the negotiation must finish within 10 s (a reset at 10 s exactly); after its FIN the prober
  resets a stream the responder never closes within 60 s, or that sends more than 1024 bytes after it (review 3,
  F4: a junk pipe); closing during the negotiation sends the FIN when it succeeds. Closing is final for the
  connection: the prober never reopens there (review 3, F5), and still answers the peer.
- Rate: one PROBE per second (±10 %), never two within 500 ms on a connection; the responder answers from a
  credit **per connection** of 1500 ms, 500 ms per echo (3 at once, then 2 per second: at most `3 + 2·t` echoes
  in `t` seconds). A dropped PROBE is **never a fault**: PROBEs held by a stalled network and delivered at once
  cost nothing but drops.
- Flood bounds that no stall can trip (review 2, F1; review 3, F6): with `bound = ⌊max(0, now − accepted) /
  500 ms⌋ + 2`, `accepted` being when the responder's handler receives the stream, a PROBE above the bound is
  dropped without fault up to 20 more (PROBEs written before a slow accept, at most one per 500 ms of the 10 s
  negotiation timeout), and a violation beyond; at most one ignorable per
  PROBE or ECHO read, plus one, per stream; a new inbound stream is refused for 30 s after the previous one ended,
  whatever ended it (60 s after a violation).
- Window: samples `(t, sent, received, rtt)` of accepted echoes, at most 64 (a window never holds more than 61);
  the figure uses the newest sample and the oldest one at most 30 s older: `⌊(1000·L + ⌊ΔS/2⌋) / ΔS⌋` permille
  with `L = ΔS − min(ΔR, ΔS)`, unknown under 200 packets, with fewer than two samples, when they are less than
  15 s apart, or when the newest is more than 30 s old. The level follows from it (« Levels »).
- Late echo: an echo arriving more than one PTO (RFC 9002 §6.2.1, no backoff, `max_ack_delay` 25 ms when the QUIC
  stack does not expose it) after its PROBE is dropped without fault. An echo of a PROBE no longer outstanding
  (8 kept) is stale and dropped without fault.
- End of a stream: whatever is read afterwards has no effect; a truncated message at the end is an ordinary end.
  The prober waits 60 s before reopening; its samples stay unless it detected a violation. After a violation it
  detected, the responder refuses every new probe stream of that connection for 60 s.
- Restart: all state is per connection; a new connection starts empty, refusals and credit included (F17:
  bounded by 2 connections per peer and the cost of an authenticated handshake).
- Roles (F11): the union of the peer's roles, evaluated at each stream's opening; a stream the new roles exclude
  is reset; raw QUIC v1 or v2 only (no TCP, circuit or WebTransport).
- Both sides probing at once: two independent streams; the responder's credit is shared by nothing else.

### Levels: what the person sees, and why these numbers (review 3; user decision of 2026-11-10)

**Why the promise changed.** Three independent reviews (17, 18 and 13 findings) kept finding the same weak point:
the permille figure. Review 3 settled it:

- the « highest priority » MUST cannot be implemented on go-libp2p v0.51 and quic-go v0.64.1. `network.Stream`
  has no `SetPriority`. `Conn.As` would bypass multistream-select and the resource manager. And quic-go sends
  retransmissions of any urgency before new stream data (`framer.go:141-175`), and ACK-only packets when the
  congestion window is full (`sent_packet_handler.go:1128-1133`). Those packets cross the PROBE whatever its
  priority, and the model's priority case counted none of them: it assumed its own result;
- the 99.9th percentiles came from one seed: with 1000 windows that percentile is the maximum, and seed 2 broke
  the spec's « ≤ 6 ‰ ».

A person does not need permille, they need to know whether a link is fine. So the protocol keeps measuring the
counts and the figure, and its normative output becomes a level with bands wide enough to absorb an error that we
can bound honestly. Gone: the priority MUST (now MAY), the « read in the delivery step » MUST (now « right after
the read call that returned the PROBE », with its ±1 race stated), and every precision claim in permille.
Kept: the 200-packet minimum and the 15 s minimum span; the error bound below is computed with them.

**The error, modelled conservatively** (`scripts/netsim/c5model.py errors`: 10 seeds × 500 windows per cell). The
model has no priority. Every packet the prober sends between its read of `sent(C)` and the PROBE's packet
crosses it: retransmissions, ACK-only packets, other streams. The responder's count is read up to 20 ms after
delivery, with the PROBE's own packet counted or not (±1 at each end). A PROBE whose own packet is lost is resent
after 9/8 RTT, and one behind a lost PROBE waits for it (head-of-line blocking); either is still accepted when it
comes back within one PTO. Each cell is the 95 % upper bound of the 99.9th percentile of `|figure − loss of its
own window|` over the 5000 windows, with the per-seed 99.9th percentiles in brackets:

| Traffic | P1 `wifi` | P2 `wifi-far` | P4 `wifi-reorder` | largest net crossing (P1, P2, P4) |
|---|---|---|---|---|
| idle, 4 packets/s | unknown: under 200 packets | unknown | unknown | — |
| typing, 17/s | 9.6 ‰ (7.2-9.6) | 9.9 ‰ (6.6-9.9) | **14.0 ‰** (8.0-14.0) | 5, 5, 9 packets |
| design, 60/s | 4.5 ‰ (3.1-4.5) | 5.6 ‰ (3.7-5.6) | 8.0 ‰ (5.0-8.0) | 8, 10, 16 |
| saturated, 300/s, the PROBE waiting up to 300 ms | per seed 3.5-5.5 ‰, 25 % unknown | 7.9 ‰ (5.1-7.9) | 11.0 ‰ (7.0-11.0) | 28, 44, 82 |
| 60/s, the count read 50 ms late | 6.3 ‰ | 6.3 ‰ | 9.0 ‰ | 11, 12, 19 |

So **the figure is within 15 ‰ of its window's loss at the 99.9th percentile in every modelled condition**. The
worst case is typing traffic under heavy reordering, a few crossings over few packets. Structurally, an accepted
echo bounds the sender-side crossings by the packets sent in one PTO after the PROBE was handed over; the spec
states the conservative bound `(p₀ + p₁ + 2) / ΔS` (§8.5.5, « Exactitude »). The clamp `min(ΔR, ΔS)` cuts
negative errors, so near zero loss the figure leans upward (review 3, F9; P4's positive mean comes from it, not
from stragglers). Under load (review 2, F8), a paced sender has 0.05 late probes per accepted one; with the PROBE
waiting up to 300 ms, 5.8 are late, so the figure goes unknown (25 % of the windows) rather than wrong. Widening
the late bound would let resent PROBEs through, with their crossings: it stays one PTO (`test_paced_sender_keeps_its_echoes`).

**The thresholds, for a person** (loss over the 30 s window; round trip: the lower median of its samples):

| Boundary | Fall below | Midpoint | Rise at | Why |
|---|---|---|---|---|
| loss, good \| fair | 0.5 % | **1 %** | 2.5 % | a few tenths of a percent is common and does not affect perceived quality (FCC, *Measuring Broadband America* 2023); Microsoft's target for a client is under 1 % loss in any 15 s; WebRTC's congestion control still raises its rate below 2 % (GCC, draft-ietf-rmcat-gcc §6) |
| loss, fair \| poor | 5 % | **10 %** | 15 % | Microsoft Teams classifies a stream as poor above 10 % loss; GCC cuts its rate above 10 %. For an editor over QUIC nothing is lost, but every lost packet delays an edit by about one PTO: at 10 % one edit in ten is late, and transfers slow down as 1/√p |
| round trip, good \| fair | 400 ms | **500 ms** | 625 ms | Teams classifies a stream as poor above 500 ms. An editor is less demanding than a call, so for us that is fair |
| round trip, fair \| poor | 1.6 s | **2 s** | 2.5 s | edits then arrive more than 1 s late, the limit of a user's flow of thought (Nielsen, *Response Times: The 3 Important Limits*) |

The level is the worse of the two. It steps at every new figure (each accepted echo whose figure is known). After
« unknown » the first figure is placed against the midpoint, then a level rises at its threshold and falls back
strictly below the lower one (spec §8.5.5 step 6). The hysteresis survives an unknown spell of at most W and is
forgotten after a longer one (review 4, F4: a short spell must not re-place the level). The step is monotone and
idempotent (`TestStepMonotoneAndIdempotent`), so the sampler's cadence does not matter.

**The bands, from the model.** Each band must do two jobs. It absorbs the figure's error: a window whose true
loss is clearly outside a band is shown on its side. And it keeps a link sitting exactly on a boundary from
flickering. `c5model.py flicker` gives the level changes per hour of a steady bursty link (netsim's `wifi` bursts)
whose loss is exactly at the boundary, over 4 hours:

| Bands | 1 % at 17/s | 1 % at 60/s | 10 % at 17/s | 10 % at 60/s |
|---|---|---|---|---|
| none | 463 | 452 | 281 | 297 |
| ×1.25 either side | 119 | 77 | 39 | 12.5 |
| ×1.5 | 89 | 45 | 10.8 | 0.25 |
| ×2 | 50 | 15 | 0 | 0 |
| **chosen: 0.5 / 1 / 2.5 %, 5 / 10 / 15 %** | 27.8 | **1.75** | 7.25 | **0** |
| ×3 | 15.8 | 0.25 | 0 | 0 |

The chosen bands keep a boundary link to about two changes an hour at the design traffic (7 changes in 4 hours;
Poisson 95 % upper bound 3.3 an hour). The rise thresholds stay on the sources' side (2.5 % against GCC's 2 %, 15 %
against Teams' 10 %), and so do the fall thresholds (0.5 % against the FCC's « a few tenths »). At typing traffic
a boundary link still changes about every two minutes: there are few packets, and that is a fact about the link,
not a fault of the bands. The round-trip bands (×1.25 / ÷1.25) face a low-noise measure: the median of up to 61
samples.

**Where the netsim profiles fall** (`c5model.py profiles`, 200 runs at the design traffic; path loss across the
two netem hops; the truth of the 30 s windows, 0.1 %-99.9 %):

| Profile | Path loss | Window truth | Level | Shown in the model |
|---|---|---|---|---|
| P0 `wifi-entry` (proposed; independent loss, entered from « unknown ») | 1.75 % | 1.4 %-2.3 % | fair | fair 100 % |
| P1 `wifi`, bursty | 3.57 % (1.8 % per hop) | 2.1 %-5.3 % | fair | fair 100 % |
| P2 `wifi-far` (RTT ≈ 300 ms) | 3.57 % | 2.3 %-5.9 % | fair | fair 100 % |
| P3 `wifi-clean` | 0 | 0-0.03 % | good | good 100 % |
| P4 `wifi-reorder` | 0 | 0 | good (the figure reads up to ≈ 1 % under heavy reordering, below the 2.5 % rise) | good 100 % |
| P5 `wifi-edge` (proposed) | 0.99 % | 0.4 %-1.8 % | on the boundary: the hysteresis decides | good 100 % |

**C5 becomes a classification test** (plan section 4; `c5model.py report`, 3000 runs of the six phases, both
directions, 120 packets/s, 240 during P0, 10 % foreign traffic on the qdiscs). Review 4 showed that the first
version, which judged each read against the truth plus a tolerance, could not see a level that never falls back:
its crossing term (about 6.7 ‰ on P3) was above the 5 ‰ fall threshold. The rule is now built from what the
prober itself reports:

- **R1, exact.** The test reads every sampler step the prober logs, its level and its figure. The level must be
  exactly the one spec §8.5.5 step 6 gives for the figures shown so far, with the round-trip level good (every
  profile is below 400 ms). A level that never falls back, enters at the wrong threshold or rises at the wrong
  place differs at the first figure where it should not.
- **R2, the figure follows the truth.** From 33 s on, the figure must lie within `1.5·√(T*/n) + c/n` of the range
  of qdisc truths its window may cover, with at least 3000 packets. `c` is the crossings in packets: 22 on P0, P1
  and P5, 32 on P2, 24 on P3, 52 on P4, about 1.5 times the largest needed over 4000 traces. **R3:** P0, P1 and P2
  were impaired (mean `T* ≥ 1 %`).
- **0 false failures in 3000 runs: ≤ 1.0e-3 per run (95 %).** Each mutant is rejected in 3000 of 3000 runs, a
  power **≥ 0.999**: always good, always poor, always fair, swapped, no hysteresis, sticky (never falls back), the
  entry against `up`, the thresholds doubled, the round-trip thresholds divided by 10 (R1); the old figure from
  quic-go's `PacketsLost / PacketsSent`, M5b (a window since the connection started) and a 60 s window (R2).
  Halved thresholds are caught in 2997 of 3000 runs (power ≥ 0.997, reported, not required). The entry is judged
  on P0, whose independent loss puts the first figure between the midpoint and the rise threshold in 99.8 % of
  the directions, hence the doubled traffic there.

**What happens to `lossPermille`** (Annex A.1). Removing it was considered. It would change the Links contract v1
and its vectors, and lose the one number a diagnostic needs; and the level is computed from it, so the two stay
consistent. **Recommended and specified: keep it as a raw diagnostic** (logs, a diagnostic panel, support), with
its error bound stated, never shown to the person as a quality and used by nothing else. `quality` carries the level.
On a relayed link the probe measures the first hop only: `lossPermille` keeps it as a diagnostic (`lossScope = 2`),
but **`quality = 0`** (review 4, F8). A level labelled « first hop » was the alternative; it is jargon, and showing
the first hop as the link's quality would mislead.

**Review 4** (the C5 rule could not see a level that never falls back: its crossing term, about 6.7 ‰ on P3, was
above the 5 ‰ fall threshold). The rule now replays the level from the figures, exactly; see the C5 paragraph above.
The other findings: a vector for every observable MUST and a `linkprobe_level` table at each threshold and one below
(the equivalent variants are named in spec §8.5.11); the role-change backoff during the negotiation; the seq bound
from the end of the responder's negotiation with a 1/64 clock margin (F5, F6); the go-libp2p recipe, a 26 ms
default `max_ack_delay` (what quic-go advertises) and the PMTUD bias (F7); the round trip's lower median in the model
and a round-trip mutant in C5 (F9).

**Review 3, how each accuracy finding is resolved.** F1: priority is MAY and the model has none. F2: 10 seeds,
confidence bounds, the tests check every seed. F3: the read instant is reworded, the ±1 race is modelled, and an
integration test pins the counter on every quic-go upgrade (gate C5S). F9: the clamp's bias is stated. F10: one
generator for every table (`errors`, `flicker`, `profiles`, `report`), each run with its seeds.

### A sans-I/O reference

`protocol/linkprobe` is pure: `Prober` and `Responder` (the rules) and `Host` (one connection, both roles:
transport and role checks, negotiation, bytes read in any chunks, ends of streams, the writer; events in, actions
out). The corpus drives all three, so a third party can check an implementation against data files alone, and
the I/O layer in inkwell (after the gate) only maps libp2p events to `Host` calls.

### Threat model (full Byzantine writers, spec §17)

- A peer can forge echoes. That only affects **its own link's figure** at the prober, and the figure is
  bounded: `received` must never decrease and never exceed the prober's own `sent(C)` at the echo (else a
  violation), so the figure stays in 0..1000 ‰ and cannot claim more packets arrived than were sent; the peer
  can make its own link look good or poor, as it could by dropping packets. The figure and the level are
  display-only: they must not enter any protocol decision (path choice, admission, flow control, sanction).
- A relay that answers sets the first-hop figure (`lossScope = 2`) of every peer relayed through it, without
  being a member of any project. It already carries and could delay or drop those packets; the figure is marked
  first hop and only shown. An endpoint answers only members and, if it is a relay, the peers using it; a lobby
  guest gets no answer (spec §8.5.1).
- A prober can make the responder echo at most `3 + 2·t` times per connection (14 bytes for 6: bounded
  amplification inside an authenticated QUIC connection, no reflection to a third party). Reopening the stream
  does not refill the credit; after a violation its new streams are refused for 60 s. A prober that turns its
  closing stream into a junk pipe is reset after 1024 bytes (review 3, F4). Refusals are not counted (F11): a
  peer that opens and gets refused in a loop is bounded only by QUIC's `MAX_STREAMS` and the resource manager, a
  residual it shares with every libp2p protocol.
- An honest prober on a stalling network or behind a slow accept never trips the responder: drops are free,
  PROBEs written before the accept are dropped within the 20 margin, and `seq` stays increasing on an ordered
  stream. TLA+ `HonestNoViolation` checks it with a stall and a burst, and with an acceptance delay
  (`LinkProbeAccept`, mutant `LinkProbeNoDropBand`).
- Replay and reordering by a third party are impossible (TLS 1.3); by a peer they are violations (`seq` strictly
  increasing on each stream).
- Decoder bounds: `len` checked against 1..64 from its own byte, before the body is read (at most 65 bytes kept
  per message); a known type has exactly its length; `seq ≥ 1`; `received` any u64, judged by the prober's rule.
  Prober state is bounded by 8 outstanding probes and 64 samples; responder state is constant. Every byte from
  the peer goes through `protocol/linkprobe.Decode`, which is fuzzed (`FuzzDecode`: no panic, unique encoding,
  never more than 65 bytes taken; `FuzzHost`: any chunking gives the same result, at most 65 bytes kept). The
  probe streams have their own resource-manager limit, per peer since libp2p's resource manager has no connection
  scope (spec §16.1: 4 inbound and 2 outbound per peer, 2 connections × (1 + 1 closing) and 2 × 1), and `Host`
  keeps the per-connection rule, so they never take the stream slots of the session.

### Privacy

The probe carries no content, address, identifier or timestamp. `received` tells the prober how many of its
packets arrived, which QUIC ACK frames largely tell it already (RFC 9000 §19.3); a relay answering on its first
hop already sees every packet. The application-level delay of an echo, beyond the RTT, hints at the responder's
load (its scheduler, its machine): a residual accepted, reduced by writing the echo at once and bounded by the
PTO (a slower echo is dropped). The figure stays local (Annex A.1 is never sent to a peer).

### Consequences

- Good: the person sees a level that follows the network's own loss, with bands derived from a stated error
  bound; NI-1 C5 checks it as a classification against netem's own counters, with a false-failure rate ≤ 1e-3 per
  run and power ≥ 0.999 (« Levels »); no quic-go change; no priority; no per-packet cost; any QUIC
  implementation that exposes the two counters of §8.5.2 can speak it.
- Good: a peer without the probe is not broken: its level is unknown (`quality = 0`, `lossPermille = 0xFFFF`),
  as an idle link's is today.
- Bad: the level says less than a figure: a link at 1.2 % and one at 9 % are both fair. That is the point
  (W33: simple, no jargon); the figure stays in the diagnostics.
- Bad: at typing traffic a link sitting on a boundary changes level every few minutes; under 200 packets in
  30 s (an idle link) the level is unknown.
- Bad: one more protocol, stream and state machine per connection, and about 20 bytes a second each way.
- Bad: the figure needs the peer's cooperation; toward an old peer or a relay that does not answer, it is unknown
  where today a (wrong) number was shown.
- Neutral: each endpoint still reports its outgoing direction only, as today.

### Confirmation

- Corpus `protocol/vectors/linkprobe.json` (147 vectors: messages; PTO; the `linkprobe_level` table; prober scripts with `level`, responder
  and host scripts; scope `transport`), run by `protocol/conformance` at the current step; spec §8.5.11 maps
  every MUST to its vector or to the test that covers what a script cannot reach (`TestStepBoundaries`,
  `TestStepMonotoneAndIdempotent`, `TestSeqBoundClampsBeforeOpening`, ...); `FuzzDecode`, `FuzzHost`.
- TLA+ `docs/protocol/tla/LinkProbe.tla`: honest (stall and burst, two streams, a transport end, the
  responder's credit, refusals and rate bound), Byzantine (forged echoes, late echoes, a restart, a hostile
  figure with data packets), a flooding prober, and an acceptance delay (`LinkProbeAccept`); 16 properties,
  `Exact` among them, and 19 mutants (`scripts/tla-check.sh`).
- Levels and accuracy: `scripts/netsim/c5model.py errors`, `flicker`, `profiles` and `report`, and
  `scripts/netsim/c5model_test.py` (`Levels`, `Rule`, `Statistics`: the 3000-run false-failure and power bounds;
  `test_error_bound_without_priority`; the bands are read from `protocol/limits.go`, one set of numbers).
- The C5S gate (plan section 6): netsim scenario 5 judged by `scripts/netsim/c5model.py verdict`, and the code
  mutants of `scripts/netsim/mutants.toml`.

## More information

- Spec §8.5, §16.1, §17, §18, §20.6, §20.8, §21, Annex A.1; CHANGES.md revision 11.
- RFC 9000 §12.3, §19.3; RFC 9001 §5; RFC 9002 §2, §5, §6.1, §6.2.1; RFC 9218; RFC 3550 §6.4.1; W3C WebRTC
  statistics (`remote-inbound-rtp.fractionLost`).
- Levels: FCC, *Measuring Broadband America, Thirteenth Report* (2023), packet loss section,
  https://data.fcc.gov/download/measuring-broadband-america/2023/2023-Fixed-Measuring-Broadband-America-Report.pdf;
  Microsoft, *Stream classification in Call Quality Dashboard*,
  https://learn.microsoft.com/microsoftteams/stream-classification-in-call-quality-dashboard, and the *Quality of
  Experience Review Guide*; Holmer et al., *A Google Congestion Control Algorithm for Real-Time Communication*,
  draft-ietf-rmcat-gcc-02 §6, https://datatracker.ietf.org/doc/html/draft-ietf-rmcat-gcc-02; J. Nielsen,
  *Response Times: The 3 Important Limits*, https://www.nngroup.com/articles/response-times-3-important-limits/.
- Review 3 (2026-11-10): go-libp2p v0.51 `swarm_stream.go:18-30`; quic-go v0.64.1 `framer.go:141-175`,
  `sent_packet_handler.go:1128-1133`, `connection.go:1739-1765`.
- quic-go issues #6167, #6168 and PR #6169. Revisit D1 if quic-go exposes an ack-eliciting sent counter and the
  probe proves too costly.
