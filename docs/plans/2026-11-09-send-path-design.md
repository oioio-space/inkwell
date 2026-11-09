# Send path between peers: one rule set, specified before code (W38)

Date: 2026-11-09. Status: **proposed**. Requirement W38: « remettre à plat, de façon très formelle, tout le chemin
d'envoi ». Author: architect. Inputs: spec §5.1, §5.5, §7.2, §10, §16.1, Annex A.1; ADR 0034 steps 4-6;
`docs/protocol/tla/{RepoGen,ConnCredit,WriteBudget}.tla`; the send-path inventory (21 items, scenario 7);
`docs/plans/2026-11-08-netsim-impaired-plan.md` (NI-1..NI-4, block WB). Nothing here is code. The target rules
amend ADR 0034 and must pass their model (§4) before any implementation.

## 1. Library facts (pinned versions, read from the module cache)

| Fact | Source |
|---|---|
| yamux stream window is 256 KiB at stream start. The protocol forbids a smaller one (`VerifyConfig`: "must be larger or equal 256 kB"). | go-yamux v5.1.0 `const.go:181`, `mux.go:99` |
| A yamux receiver sends a window update only once it has consumed **half** the window (`delta < max/2` → no update) | go-yamux `util.go:105` (`GrowTo`), `stream.go:215` |
| The yamux window doubles if an update comes within 4 × RTT of the previous one. RTT is measured by pings that queue behind data. | `stream.go:226` |
| yamux data frames are ≤ 64 KiB. Keepalive is 30 s. `ConnectionWriteTimeout` is 10 s. | `mux.go:69-84` |
| quic-go sends MAX_STREAM_DATA after **25 %** of a 512 KiB window is consumed, i.e. 128 KiB as well | quic-go v0.64.1 `internal/protocol/params.go:31`, `flow_controller_base.go:38` |
| go-libp2p builds `yamux.Transport` from a `yamux.Config` (it can be replaced, but the window stays ≥ 256 KiB). Direct connections use QUIC. Relayed and TCP connections use yamux. | go-libp2p v0.51.0 `p2p/muxer/yamux/transport.go`; `internal/transport/p2p/node.go:347-351` |
| Circuit v2: a relay may set per-circuit `data` and `duration` limits; past them it "should reset the stream" on both hops. The spec says nothing about buffering between the hops. | [circuit-v2] |

So on both transports a sender's `Write` only sees progress in **128 KiB quanta** (the update quantum `U`), each
one delayed by the reverse path.

## 2. Root causes: a model of today's path

Path in scenario 7: Alice ─QUIC─ relay ─QUIC─ Bob, then Noise and yamux end to end over the circuit. netem sits on
the relay's single `eth0` egress: 8 KiB/s, 24 packets ≈ 4.1 s of queue, for **both** directions. So the data going
to Bob and the window updates and ACKs going to Alice share one FIFO (assumption taken from the plan's topology:
"relay `eth0`").

| # | Root cause | Evidence | Status |
|---|---|---|---|
| RC1 | **Progress is measured at "the local Write returned", not at what the peer consumed.** The first hop is fast, so Alice fills the 256 KiB yamux window in under 1 s. The next `Write` is then blocked until Bob has consumed `U` = 128 KiB and his update has come back. Stall ≈ `U/R + d_fwd + d_rev` ≈ 131 072 / 6 656 + 4.1 + 4.1 ≈ **28 s > WriteIdle (20 s)**. Predicted threshold: every honest rate under `U / (20 − 8.2 s)` ≈ 10.8 KiB/s is cut, at every attempt. | Every failure is `len=32782`, `elapsed=20.000 s`, on the single-`Write` path. The rate measured at Bob (6.5 KiB/s) is above `FloorRate`. Four attempts out of four. | arithmetic consistent; **to prove (E1)** |
| RC2 | The deadline is armed once per `Write` call (one 32 KiB fragment, inventory #13). Re-arming per chunk would not help: while the window is closed nothing is accepted. | `frame/progress.go` (`len ≤ smallFrame` → one `Write`); yamux `write()` waits for the window | proven by reading |
| RC3 | **Downstream bufferbloat.** The full window (≈ 32 s of data at 8 KiB/s) waits in the relay's hop buffer. (1) Every later frame (typing, and the in-band yamux RST) queues behind it, whatever the stream. (2) The RST reaches Bob only after the window's content: that explains why the bytes read at a reset are a constant (≈ 233 KB, the window's 7 whole fragments) and why Bob kept receiving after Alice's cut. | 4 resets at 232 913 / 232 830 / 232 794 / 232 749 B; 4 fragments received after the stall | **to prove (E1, E3)** |
| RC4 | **Resend from byte 0.** Rule L resets the automerge sync state, the same bytes go again on the same path, and they fail the same way: a deterministic loop. 930 KB were sent and nothing was delivered. | inventory #21, logs | proven |
| RC5 | **Head-of-line blocking in v1.** `Router.sendTo` sends one message at a time and `ps.wsem` admits one writer per stream: a typed message waits behind the blocked fragment `Write` (≤ 20 s), then behind RC3's queue, then behind the resync. | typing waits 44-49 s | consistent |
| RC6 | **The WB model is unsound for this failure.** `WriteBudget.tla` lets the sender see progress at the reader's rate on every tick. With no window and no reverse path, `Delivered` holds in the model and fails in reality. WB-2 (the total) was never the trigger, so WB's own NI-3 criterion (0 resyncs) is expected to stay red. | `WriteBudget.tla` header; RC1 | expected; measured by E1 |
| RC7 | **v2 as written inherits RC1 and RC3.** §7.2.5 « aucun progrès pendant 60 s » does not say *whose* progress. Measured at the sender's `Write`, it cuts honest links under `U/(60 − 2·D)` ≈ 2.6 KiB/s, which is above the 2 KiB/s floor. §5.6.3 (« un gros transfert ne retarde pas la frappe ») is false on a relayed circuit: the REPO and LARGE streams are separate, but they share the relay FIFO, and LARGE can fill it with 256 KiB. | §1, RC3 | design finding |

Lesson for the model (§4): it must contain the transport window with its update quantum, a FIFO bottleneck that
both directions share, and the reverse path.

## 3. Target protocol (v2, amends ADR 0034 rev. 10; v1 interim in §5)

### 3.1 State of the art, and what we keep from it

| Prior art | What it gives | What we keep, or what it misses |
|---|---|---|
| RFC 9218 extensible priorities [9218] | urgency plus incremental, used by the *sender's* scheduler | Keep: one lane per urgency. Missing: a priority cannot reorder bytes already queued downstream (relay, bottleneck). quic-go round-robins its streams, and yamux sends in FIFO order. |
| HTTP/2 prioritisation with `TCP_NOTSENT_LOWAT`, BBR [cf-h2][nginx-1763][oku] | Large buffers commit data in order. Fix: keep the unsent and in-flight data small, plus pacing; network-level bloat needs rate-based pacing. | **Keep, at our layer**: the yamux window cannot go below 256 KiB (§1), so the only lever is a cap above it. |
| QUIC/HTTP2 credit [RFC 9000 §4][RFC 9113 §5.2] | absolute limits that never go back, idempotent | Keep the vocabulary (`consumed`, `window`) already used by the bridge (Annex A.1). |
| Aeron status messages [aeron] | The receiver reports its consumption position and window, at 25 % of the window or on a timer. The sender's liveness is measured on those messages. | Keep: progress = the consumer's reported position. |
| BBR [bbr] | inflight ≈ BtlBw × RTprop, with windowed max and min filters | Keep the rate × time sizing, but run it **at the receiver**, which sees the arrivals; no new congestion control. |
| LEDBAT [RFC 6817] | a scavenger class aiming at a target queueing delay | Missing: we own both lanes, so a fixed latency budget for bulk is simpler than a delay-based controller. |
| Resumable uploads [resumable] (`Upload-Offset`) | resume at an offset the server confirms | Keep: resume at the offset the *receiver* states, never one the sender guesses. |
| Automerge sync, Beelay/sedimentree [am-sync][sediment] | Sync resends every missing change after a reset. Sedimentree cuts history into content-addressed strata, aiming at pause and resume. | Keep: content addressing (the LARGE `root`) as the identity to resume on. |

**Beyond them.** Most systems stack windows (transport, mux, application) and let whichever is smallest win by
accident. Here **one window binds per hop by construction**: our LARGE window is sized under the transport's
update quantum (rule SP-3). The transport windows then never block an honest peer; they only stop a Byzantine one.
Progress, pacing and resumption all read **one** number, `consumed`, reported by the receiver.

### 3.2 Lanes

| Lane | Carries | Who paces | Write deadline |
|---|---|---|---|
| CONTROL | HELLO, GOAWAY, SET, ATTEST, LARGE_REPLY, **LARGE_CREDIT** | always read (§10.2 a) | none |
| REPO (interactive) | REPO ≤ 64 KiB, ATTEST, AUTHOR, LARGE announce | the receiver's inbound queue, then the transport window (backpressure, never a failure) | none |
| LARGE (bulk) | one object of 64 KiB to `r` per peer, after LARGE_REPLY | **the receiver's LARGE credit** (SP-2) | none: SP-4 cuts it by a stream reset |
| bulk `/inkwell/bulk/1.0.0` | blobs and indivisible changes, Bao ranges | the requester's outstanding ranges, sized by the same rule as SP-2 (step 5) | none: SP-4 |
| PRESENCE | ephemeral | always read, lossy | none |

### 3.3 Rules

- **SP-1, progress.** The progress of a transfer is `consumed`: the byte offset the receiver has taken into its
  reservation, stated by the receiver. "The `Write` returned" never counts as progress, on any lane.
- **SP-2, one credit.** The receiver grants `LARGE_CREDIT{gen u32; id u32; consumed u32; window u32}` (new critical
  type 0x0C, CONTROL; LARGE_REPLY gains the same two fields).
  - Like A.1, the sender keeps `sent ≤ consumed + window`. Credit is monotone in `consumed`; a regressing one is
    ignored.
  - The receiver sends a credit when `consumed` has advanced by `C` = 8 KiB, and at most `T_ack` = 1 s after any
    unreported byte (Aeron's "25 % or timer"; QUIC `max_ack_delay`).
  - Sizing: `window = clamp(max(2C, rate × max(1 s, 2·rtt)), 2C, Wmax)`. `rate` is the receiver's windowed maximum
    arrival rate. `rtt` comes from the W33 link metrics (`rttMs`).
- **SP-3, the transport never binds an honest peer.** `Wmax = (1 − θ)·W_init`: 128 KiB on yamux (relayed, TCP) and
  384 KiB on QUIC.
  - Proof: the receiver reads LARGE at once, so the transport window available to the sender is at least
    `W_init − (sent − consumed) − θ·W_init`. With `sent − consumed ≤ window ≤ (1 − θ)·W_init`, it is never negative.
  - On yamux the window update and our credit share one ordered session, and the update is written first, so the
    update never arrives late.
- **SP-4, cut on a floor.** A one-shot timer `T_p` = 40 s is re-armed each time `consumed` (at the sender) or the
  received count (at the receiver) has advanced by `P` = 32 KiB since the last arming, and only while bytes are
  outstanding. When it fires, the stream is reset, which unblocks any `Write`.
  - This is one idle timer whose quantum is `P` instead of one byte. A stopped reader is cut within `T_p`, and a
    drip within `S / (P/T_p) + T_p`. No total timer depends on the size.
  - Derivation: an honest link of rate ≥ `F` = 2 KiB/s with one-way delay ≤ `D` = 5 s must never be cut:
    `P/F + 2D + T_ack` = 16 + 10 + 1 = 27 s, and 27 × 1.2 = 32.4 ≤ 40.
  - It replaces LARGE's « 60 s sans progrès » and bulk's « 30 s sans octet » (one constant pair, `protocol` §16).
- **SP-5, resume at the receiver's offset.** The receiver keeps the prefix it received for `(peer, root)`, in that
  peer's fixed 4 MiB share (§10.6), for 10 min or until that peer's next LARGE with another root.
  - LARGE_REPLY carries `consumed = offset`. The sender opens `/inkwell/large/2.0.0` with the preamble
    `gen; id; size; offset` and writes from `offset`.
  - At the end, BLAKE3 over the whole object must give `root`. If it does not, the violation is charged to that peer
    and the prefix is dropped.
  - An `offset` beyond what the receiver holds is impossible for an honest pair: `#large-resume-beyond`.
- **SP-6, rule L, unchanged in shape.** A LARGE stream of the current epoch that is cut (SP-4) or reset by the peer
  is a loss, and goes through rule L as today.
  - Only the *cost* changes: SP-5 makes a retry resend at most what was outstanding (≤ `window`), not `S`, whenever
    the next epoch announces the same `root` (decision 5).
  - SP-4's derivation means an honest link inside the contract is never cut. So no resend loop can start inside the
    contract (RC4 is gone).
- **SP-7, interactive bound.** The bulk bytes ahead of a typed frame are at most `window`, plus one `C`-sized write
  in the mux.
  - So `L_int ≤ D_q + (window + I)/R + rtt`. `D_q` is queueing caused by *other* traffic, outside our control.
  - NI-3 at 8 KiB/s: window = 2C = 16 KiB, so ≈ 2 s + 0.1 s, against the plan's 10 s bound.
  - The LARGE writer writes in `C`-sized calls, so no yamux frame of more than `C` bulk bytes precedes a typed frame
    locally.
- **SP-8, Byzantine bounds.**
  - The sender rejects a credit whose `consumed > sent` (`#large-credit-beyond-sent`, a violation).
  - Credits for another epoch or an unknown `id` are ignored (rule T).
  - More than `⌈size/C⌉ + 1` credits per second per object is OVERLOADED.
  - A sender that writes beyond `consumed + window` is in violation (`#large-over-credit`).
  - A reader that drips holds only its own LARGE lane and its own share, at most 4 MiB / 0.8 KiB/s ≈ 85 min. Typing
    to it, and every other peer, are untouched.
  - A writer that drips into our reservation is cut by SP-4 on the receiver's side.
  - A huge window only hurts the latency of the peer that grants it, so it is the receiver's own trade-off: the
    receiver is the one waiting for the sender's typing.
  - A relay can only delay or drop (the end-to-end Noise/TLS protects the content). That looks like a slow path or a
    lost session.

### 3.4 Timers: one meaning each

| Timer | Kind | Value | Fires → |
|---|---|---|---|
| `T_p` progress floor (SP-4), LARGE and bulk, both ends | idle per quantum `P` | 40 s / 32 KiB | stream reset; current epoch: rule L; bulk: another holder |
| `T_ack` credit delay (SP-2) | maximum delay of a report | 1 s | the receiver sends LARGE_CREDIT |
| LARGE_REPLY wait (§7.2) | total, waiting for a reservation | 10 min | forgotten, sync state reset |
| RESET_STREAM after STOP_SENDING, HELLO (§5.1, §5.3) | total, protocol deadline | 10 s | GOAWAY (G-1 stays open) |
| QUIC idle 30 s, keepalive 15 s; yamux keepalive 30 s, write valve 10 s | connection liveness, owned by the transport | pinned | session lost |
| rule I/R cadence | rate limit | 2 s doubling to 60 s | delayed |

Removed by v2: `WriteIdle`, `FloorRate` per frame and per message, `partialTTL`, the reset on `outboxDepth`
(inventory #1, #7, #8, #11-#13). `SlowSend` stays as a metric only.

### 3.5 Mapping onto ADR 0034

| Step | Content of this design |
|---|---|
| spec | **ADR 0035** amends §5.1 (preamble `offset`), §5.2 (0x0B +8 B, 0x0C), §5.6.3 (SP-7 bound), §7.2.5 (SP-4), §10.2 (SP-1, SP-3), §16.1 (`C`, `P`, `T_p`, `T_ack`, `Wmax`, `F`, `D` stated as the public contract); new vectors in `large.json` |
| 4 `protocol/sync2` | codecs for 0x0B, 0x0C and the preamble; **pure** rules SP-2/3/4/8 (time injected, no I/O); rapid state-machine tests against the model's rules |
| 5 bulk | ranges and outstanding requests sized by SP-2; the server's `WriteProgress` replaced by SP-4 |
| 6 session | LARGE lane with credit, SP-5 prefix store (Go, opaque bytes), and the webview replay if decision 5 asks for it (TypeScript, ADR 0011) |

## 4. What the TLA+ model must check

New module `docs/protocol/tla/SendPath.tla`, with configurations `SendPathHonest.cfg` and `SendPathAny.cfg`, and
mutants in `mutants/SendPathMutants.tla`.

**Model content:**
- one peer pair, one LARGE object of `S` units, and interactive units arriving at any time;
- a bottleneck FIFO of rate `R` that both directions share, with delay `D`;
- a transport window `Wt` with update quantum `θ·Wt` (yamux: `Wt/2`);
- receiver credit (SP-2), the floor timer (SP-4), and epochs with the SP-5 prefix;
- Lamport upper-bound timers, as in `WriteBudget`.

**Readers and sizes:**
- Honest: rate ≥ `Floor`. Any: drips, stops for good (no fairness), forges credit.
- Small sizes: `S` = 6, `Wt` = 4, window ≤ 2, `D` ≤ 1.

| Property | Kind | Meaning | Mutant that must break it |
|---|---|---|---|
| `NoSilentLoss` | safety | every object is delivered or its epoch ended and the webview was told (rule L) | M-SilentAbandon: the cut does not change the epoch |
| `NoFalseCut` | safety | a cut ⇒ fewer than `P` units consumed during the last `T_p` | **M-WriteReturn**: progress = transport credit at the sender, `Idle` < `U/R + 2D` (reproduces scenario 7) |
| `DeliveredHonest` | liveness | honest link ⇒ ◇ delivered, in exactly one attempt (progress ⇒ delivery, no resend loop) | M-WriteReturn, M-FixedFloor |
| `WindowNeverBinds` | safety, honest | the sender is never blocked on the transport window | M-CapAboveWindow: `Wmax` > `(1 − θ)·Wt` |
| `InteractiveBound` | safety | the age of an interactive unit is ≤ `Lmax` = `(window + 1)/R + 2D` | M-NoCap (window = `Wt`); M-SharedWriter (interactive waits for the bulk `Write`, v1) |
| `StopCut` | safety (timer) | a stopped reader is cut within `T_p` | M-NoCut |
| `HoldBound` | safety | any reader holds the lane ≤ `S·T_p/P + T_p` | M-ZeroOnly (quantum of 1 unit: a drip holds for ever) |
| `BoundedWaste` | safety | units sent ≤ `S` + (number of epoch changes) × `Wmax` | M-FromZero (offset ignored) |
| `CreditSane` | safety | `consumed` ≤ `sent` at the sender | M-TrustCredit |
| `ControlLive` | liveness | CONTROL credits always get through | reuse `ConnCredit`: a credit frame cannot wait behind data |

Acceptance: both configurations exhaustive, liveness included, and every mutant caught by `mise run tla`.
`WriteBudget.tla` stays as the record of block WB. Its README gains « superseded by SendPath: no window, no reverse
path ».

## 5. Experiments that turn the « to prove » items into evidence

| Id | Proves | Method (netsim at rest, `heavy.sh`, 3 seeds) | Pass if |
|---|---|---|---|
| E1 | RC1, RC3 | Probe over the relay: Alice pushes 1 MiB in 4 KiB writes on one libp2p stream; both sides log time and bytes per call. Sweep `R` ∈ {6, 8, 12, 16, 32} KiB/s. **Intervention**: Bob's `InitialStreamWindowSize` = 512 KiB. | Every stall = `U/R + d_fwd + d_rev` ± 20 %. The cut threshold is ≈ 10.8 KiB/s. With 512 KiB, the bytes before the first stall and the bytes Bob holds at a reset move from ≈ 256 KiB to ≈ 512 KiB. |
| E2 | RC1 on direct QUIC | the same probe without the relay, netem on the NAT | quantum ≈ 128 KiB (25 % of 512 KiB) |
| E3 | RC3 | v1: a typed frame during the paste; latency against the bytes queued (`tc -s qdisc` plus the probe's counters) | latency ≈ bytes ahead / `R` |
| E4 | SP-2, SP-3, SP-7 | the probe with a reverse-stream credit following SP-2 | no `Write` blocked > `C/R + 2D`; typed frame ≤ SP-7 bound |
| E5 | value of SP-5 | TypeScript: 879 KiB paste; reset the sync state (rule L); regenerate, with and without concurrent typing; compare BLAKE3 | gives the share of identical roots, which feeds decision 5 |
| E6 | yamux auto-tuning under bloat | E1, read across two epochs on one session | growth (doubling within 4 × RTT) observed or not |

## 6. Block WB, and the decisions for the user

**Reading of WB:**
- WB fixed the right *loop* with the wrong *trigger*: RC6.
- Its frozen gate expects NI-3 with 0 resyncs. By RC1 that criterion stays red as long as `WriteIdle` is 20 s.
- `gates/WB.toml` is frozen (the `gate-freeze` hook), so it is closed, not edited.

Decisions:

1. **WB:** close it as *superseded, not passed*, and record that in the netsim plan. Fold its WB-2 code (the
   2 KiB/s floor, which is harmless and already in the working tree) into block SP1. *Recommended.* The alternative,
   running WB's gate as it is, is expected to stay red (RC1).
2. **v1 interim, block SP1:** set `frame.WriteIdle` (v1 only) to 90 s, derived from `U/F + 2D` = 64 + 10 = 74 s,
   × 1.2.
   - Cost: a reader that stops is cut at 90 s instead of 20 s, and it still holds only its own sender.
   - Gain: NI-3 C1 and C3 pass on v1 (no resend loop), and a blackout of NI-2 during a write no longer resyncs.
   - NI-3 C2 stays a declared known failure (RC3, RC5) until step 6.
   - Gate SP1: E1 proves RC1 first; then NI-3 C1 and C3 pass on 3 seeds with 0 resyncs; NI-1, NI-2 and NI-4 do not
     regress. *Recommended.*
3. **v2 credit:** the receiver grants it, `LARGE_CREDIT{consumed, window}` (SP-2), rather than sender ACKs plus a
   BBR-style estimator at the sender. The receiver sees the arrivals, the vocabulary is that of A.1 and RFC 9000,
   and the choice is incentive-compatible. *Recommended.*
4. **Contract constants:** `F` = 2 KiB/s, `D` = 5 s, `P` = 32 KiB, `T_p` = 40 s, `C` = 8 KiB, `T_ack` = 1 s. One
   floor for LARGE and bulk, replacing 60 s and 30 s, and published in §16 as the links inkwell promises to finish
   over. *Recommended.*
5. **Resumption:** SP-5, the prefix by `(peer, root)` and the offset in LARGE_REPLY. *Recommended now.* Whether the
   webview replays the unfinished object first in the next epoch, so that the root stays stable, is decided after
   E5. Note that a third-party relay with go-libp2p's default limits (128 KiB, 2 min per circuit) can only carry a
   paste through SP-5.
6. **Form:** a new ADR 0035 that amends ADR 0034 (the protocol is still a draft and steps 4-6 are not implemented,
   so the change is cheap now), with a review by Codex and by `security-auditor` of SP-8. *Recommended.*
7. **Order:** E1 → SP1 (v1) → SendPath model and mutants → ADR 0035 → steps 4-6 as in §3.5. *Recommended.*

## References

- [circuit-v2] libp2p, Circuit Relay v2, https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md
- [9218] RFC 9218, Extensible Prioritization Scheme for HTTP, https://www.rfc-editor.org/rfc/rfc9218
- [RFC 9000 §4] QUIC flow control, https://www.rfc-editor.org/rfc/rfc9000#section-4 ; [RFC 9113 §5.2]
  https://www.rfc-editor.org/rfc/rfc9113#section-5.2
- [cf-h2] P. Meenan, Optimizing HTTP/2 prioritization with BBR and tcp_notsent_lowat,
  https://blog.cloudflare.com/http-2-prioritization-with-nginx/ ; [nginx-1763] https://trac.nginx.org/nginx/ticket/1763 ;
  [oku] K. Oku, httpbis list, https://lists.w3.org/Archives/Public/ietf-http-wg/2016JulSep/0556.html
- [aeron] Aeron, Flow and congestion control, https://github.com/real-logic/aeron/wiki/Flow-And-Congestion-Control
- [bbr] N. Cardwell, I. Swett, draft-ietf-ccwg-bbr, https://datatracker.ietf.org/doc/html/draft-ietf-ccwg-bbr
- [RFC 6817] LEDBAT, https://www.rfc-editor.org/rfc/rfc6817
- [resumable] draft-ietf-httpbis-resumable-upload-15,
  https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-15
- [am-sync] Automerge, Merry Commitmas (sync of large documents), https://automerge.org/blog/merry-commitmas/ ;
  [sediment] Beelay sedimentree, https://github.com/automerge/beelay/blob/main/docs/sedimentree.md
- L. Lamport, Real Time is Really Simple, MSR-TR-2005-30 (timers in TLA+, as in `WriteBudget.tla`)
- go-yamux v5.1.0, quic-go v0.64.1, go-libp2p v0.51.0 sources (module cache, lines cited in §1)

## Decisions accepted (user, 2026-11-09)

The direction above is accepted as recommended: block WB is closed as superseded (its frozen gate stays as
history; its 2 KiB/s floor moves into block SP1); SP1 first (v1 sync/1: floor 2 KiB/s, WriteIdle 20 → 90 s
until credits exist), specified and modelled before code; then SendPath.tla with every property and mutant;
then ADR 0035 amending ADR 0034; then ADR 0034 steps 4-6. Measurement E1 runs first and may change SP1's
numbers. Working rule (CLAUDE.md, 2026-11-09): when a problem gets hard, stop, reformalize, then do it well.

## E1 results (2026-11-09)

Method. A scratch probe (`e1probe`, not committed; scratchpad `e1/`, built in a throw-away worktree) puts two libp2p hosts
in the netsim topology (symmetric NATs, so the path stays relayed: Alice, QUIC, relay, QUIC, Bob; Noise and yamux end to
end; no hole punching; `inkwell-relay` unchanged). netem sits on `relay:eth0` like profile `slowrelay`
(`delay 20ms rate R limit 24 seed 1`). Alice pushes 1 MiB on one stream in 4 KiB writes, Bob reads at once; both log time
and bytes per call. A « stall » is a `Write` that blocks for more than 1 s. One run per row, **one seed**, not the 3 seeds
of the plan. Differences from production: the libp2p resource manager is the default one, there is no `frame` layer, and
there is no sync layer, so « WriteIdle » is read as « a single `Write` blocked ≥ 20 s » and « resyncs » is not measurable
here (every such stall would be a cut and a resync on v1).

| R (KiB/s) | first stall: Alice had accepted, Bob had consumed | stall durations (s), in order | mean excl. first | `U/R` (U = 128 KiB) | interval between unblocks | blocked ≥ 20 s | outcome |
|---|---|---|---|---|---|---|---|
| 32 | 252 KiB, 0 | 5.4 4.4 4.5 4.4 4.4 4.1 | 4.3 | 4.0 | 4.1-4.5 s | 0 | done, 27 s |
| 16 | 252 KiB, 0 | 10.8 8.6 9.0 8.6 9.2 8.3 | 8.7 | 8.0 | 8.3-9.2 s | 0 | done, 55 s |
| 12 | 252 KiB, 0 | 14.3 11.9 11.8 11.3 11.9 11.4 | 11.7 | 10.7 | 11.3-11.9 s | 0 | done, 73 s |
| 8 | 252 KiB, 0 | 21.8 16.7 18.7 17.0 19.9 15.4 | 17.5 | 16.0 | 15.4-19.9 s | **1** (the first) | done, 110 s |
| 8, 32 KiB writes | 224 KiB, 0 | 21.5 19.4 15.5 17.5 23.3 10.8 | 17.3 | 16.0 | 10.8-23.3 s | **2** | done, 108 s |
| 6 | 252 KiB, 0 | 28.4 23.6 10.0 | 16.8 | 21.3 | 10-28 s | **2** | **session closed at 62 s** (yamux `connection write timeout`), 512 KiB sent |
| 4 | 252 KiB, 0 | 46.9 31.2 40.0 31.6 33.3 43.4 | 35.9 | 32.0 | 31-47 s | **6 of 6** | done, 226 s |

Bob had consumed 148-160 KiB when the first stall ended, then about 270-280, 404-416, 528-532, 652-676, 784-812 KiB.
Each unblock comes one quantum (128 KiB) of Bob's consumption after the previous one, and Alice's accepted total
advances 252, 384, 512, 640, 768, 896 KiB: the sender sees progress in 128 KiB steps, as RC1 says. Alice filled the
256 KiB window in under 1 s (first write ends at 252 KiB) while Bob had consumed nothing.

Intervention: Bob's `InitialStreamWindowSize` = 512 KiB, R = 8 KiB/s (same probe).

| Run | accepted at the first long stall | Bob consumed at that stall | stall | notes |
|---|---|---|---|---|
| 256 KiB (default) | 252 KiB | 0 → 152 KiB | 21.8 s | quantum 128 KiB |
| 512 KiB | **508 KiB** | 20 → **264 KiB** | **31.5 s** | quantum 244-256 KiB = W/2; 244 KiB / 8 KiB/s = 30.5 s; a short first stall (7.9 s at 252 KiB) is the window-grow update; the session was closed at 49 s by `connection write timeout` (764 KiB sent) |

**Verdict on RC1: confirmed, with one number corrected.**
- Confirmed: progress is seen at the sender only in `U` steps; the stall is `U/R` plus a delay; the stall point moves
  from ≈ 256 KiB to ≈ 512 KiB and the quantum doubles with the window (`alice.csv`/`bob.csv` of `out_r8_w0_c4096` and
  `out_r8_w512_c4096`). Stall means are within +9 % to +12 % of `U/R` for R = 4, 8, 12, 16 (+8 % at 32).
- Corrected: the delay term `d_fwd + d_rev` is 1-4 s here, not the 8.2 s of §2. The first stall of a run is the longest
  (+5.8 s at 8 KiB/s) because the relay queue holds the burst. So the 20 s limit is crossed by the first stall at
  ≤ 8 KiB/s (steady state at ≲ 6.4 KiB/s), not at 10.8 KiB/s. The cut at 8 KiB/s, the scenario-7 rate, still happens, but
  on one or two stalls out of six, which fits scenario 7's « four attempts out of four » only with 32 KiB writes
  being larger than the probe's and the extra `frame` layer; not proven here.
- RC3 (partly): 252 KiB are in flight with Bob at 0 consumed, so a window's worth sits in the path buffers. Not
  tested here: the latency of a typed frame (E3) and the content of the relay's buffer.

**Unexpected.**
1. At 6 KiB/s and in the 512 KiB run, yamux closed the whole session: `stream reset: connection closed: connection
   write timeout` (`ConnectionWriteTimeout` = 10 s), just after a `Write` blocked for 10.0 s. At 4 KiB/s, with longer
   stalls, it did not happen in this run, so it depends on what the session writer is blocked on: 2 runs of 8, not
   reproducible by rate. This is a second cut, independent of `WriteIdle` and of any `frame` constant, and it kills every
   stream of the pair (R4 of the netsim plan); raising `WriteIdle` alone (decision 2) does not remove it. It needs its own
   experiment (several seeds) before decision 2 is fixed at 90 s.
2. netem tail-drops 250-520 packets in every run (`limit 24` against bursts from the QUIC hop): QUIC loss recovery
   hides them, but they add to the first-stall excess.
3. Not done: E1's 3 seeds, the rate 12 sweep repeated, and the `tc -s` counters during a stall (only the final counters are kept).
