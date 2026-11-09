# Impaired-network test plan (W32) and shared link metrics (W33)

Date: 2026-11-08. Status: **proposed**, to review before the gate below is frozen.
Requirements: W32 (tests on a disturbed network, each aimed at a precise risk, measurable, with a
proof that it fails when the mechanism is broken) and W33 (see how each person is connected and the
link quality, with the same measurements as W32). Lot: **« Collaborer sans perdre »**
(`docs/architecture/coherence.md` §5: « inviter, retirer, hors ligne puis converger », with the
ADR 0034 migration). Nothing here is a new subsystem: the plan extends `scripts/netsim`,
`frontend/e2e/netsim.spec.ts`, the `natNet` simulator of `internal/transport/p2p/sim_test.go` and the
existing `p2p.Node.Links()`. Method: `docs/plans/2026-11-08-spec-executable.md` (executable spec and
mutants first, frozen gate before code).

## 1. State of the art, and what we take from it

| Source | What it says | What we take, or refuse |
|---|---|---|
| `tc-netem(8)` [1] | Reordering needs delay ("for any method of reordering to work, some delay is necessary"). Jitter reorders through the internal `tfifo`, unless it is replaced by `pfifo`. `limit` is the number of packets held while delaying. `loss gemodel p r 1-h 1-k` is Gilbert-Elliott (p = enter bad state, r = leave it, 1-h = loss when bad, 1-k = loss when good). `seed` replays the loss and corruption draws. `rate` emulates a link speed, plus a per-packet overhead. | Jitter gives the reordering, and we keep `tfifo` on purpose. GE stands for bursty Wi-Fi loss. `seed` is always set. `limit` is always sized explicitly (pitfall below). |
| netem `limit` pitfall [2][3] | The default of 1000 packets, combined with `rate`, makes a huge hidden queue. At 64 kbit/s it is 1000 × 1400 B / 8 KB/s ≈ 170 s of queue, which measures bufferbloat, not the protocol. Hemminger: size the limit to the bandwidth-delay product. | `limit = ceil(rate × target queue delay / 1400 B)`, written next to every profile. |
| netem acts on egress | The libp2p hole-punch router [4] puts half the delay on each of the router's two interfaces, "tc can only apply delays on egress traffic". | Same placement: netem goes on the **egress of both interfaces of a forwarding namespace** (NAT `eth0` upstream, `eth1` downstream; relay `eth0`). No `ifb`. No netem on an endpoint either: a qdisc that drops on the sender's own interface feeds back into its socket (ENOBUFS, TSQ), which a real network never does. |
| QUIC interop runner [5] | The QUIC community's own impairment matrix: `transferloss` 2 % with bursts of 3, `handshakeloss` 30 %, `blackhole` 5 s on and 2 s off, `longrtt` 750 ms, `transfercorruption`, `rebind-port` every 5 s. All of it at 10 Mbit/s with a **25-packet** queue. | quic-go already passes this matrix [5], so we do **not** retest QUIC loss recovery. Our profiles reuse its orders of magnitude (2 % bursty loss, small queues), but each one targets **our** timers. |
| quic-go's own tests [6] | `integrationtests/tools/proxy`: a UDP proxy with `DropPacket` and `DelayPacket` callbacks; plus `testutils/simnet`. | Our Go layer already does this with `natNet` (`sim_test.go`, seeded, under `synctest`). We extend it rather than add a proxy. |
| libp2p test-plans [4] | Moved from Testground to docker compose; NAT routers with nftables and netem, one RTT per test, RTT printed as JSON. | Same shape as our netsim (nftables NATs, netem on the routers), without Docker, in unprivileged namespaces. RTT reported as a measurement is the precedent for our metrics oracle. |
| Toxiproxy [7] | "A TCP proxy": latency, bandwidth, slicer, reset_peer… | **Refused**: our traffic is QUIC over UDP. |
| Pumba [8] | Wraps netem for containers (Docker, containerd, Podman). | **Refused**: we have no containers, and netem already works in our unprivileged namespaces. |
| Jepsen [9] | Faults (nemesis) plus a checker over the recorded history; the property is checked, not eyeballed. | Kept as the oracle style. Every scenario records what each side typed (unique tokens) and what it ended with. The checker asserts strong eventual consistency (same text), no lost write (every token present), no duplicate (every token once), and bounds derived from our timers. Elle-style anomaly search does not apply: Automerge has no transactions. |

**Beyond the state of the art.** Four choices that the usual approaches miss:

1. **Two layers, each test at the cheapest layer that can observe its risk.**
   - Transport timing properties go in the seeded, virtual-time `natNet` (`synctest`): 20 seeds in seconds, deterministic replays. Its seeded reachability tests already show the method works (`TestPartitionThenHeal`, `TestRoamToNewAddress`).
   - Only what needs the real stack (webview, Automerge, admission, real UDP, NAT and conntrack) goes into netns with netem.
2. **Clause-level verdicts and characterization.**
   - Several risks are known, open defects of today's `sync/1` (R-Evict, N-15, R-Reset in `docs/protocol/inventaire.md`).
   - A scenario that targets one of them runs now as a **characterization**: expected red, result archived. It is the *pre-fix mutant* that proves the scenario bites before the fix exists. It becomes a pass/fail criterion of the ADR 0034 step that closes the defect.
3. **Anti-vacuity clauses.** Each scenario asserts that its impairment really happened: netem qdisc counters, relay bytes, the A–B link really down. A netem rule that silently failed to install must turn the test red, not green.
4. **One metrics contract for tests and UI** (§7). The impaired scenarios *validate the metrics against injected ground truth* (netem delay and loss are known), so W33's numbers are tested before any UI shows them.

## 2. Risks, derived from our code and spec

Timers and limits as they are today (`sync/1`), with sources:

- **Write and stream timeouts.**
  - `frame.WriteIdle` = 20 s without progress, plus a total budget of 20 s + len / `MinRate` (16 KiB/s) (`internal/transport/frame/progress.go`).
  - `openTimeout` 15 s, `upTimeout` 30 s, `reorderWait` 1 min (`p2p/node.go`).
  - mux fragments of 32 KiB, `partialTTL` 30 s; fragments get 1 turn in 4 (`mux/fragment.go`).
- **Dialing and holds.**
  - Dial backoff 1 s → 60 s with ±25 % jitter, `stableAfter` 30 s; dial timeouts 10 s (known addresses) and 20 s (circuit) (`p2p/dial.go`).
  - Reset hold 2 s → 60 s, remembered for 10 min (`node.go`).
- **Relay.** Relay keeper backoff ≤ 10 s, `reserveTimeout` 15 s, and a kick of every dial loop when a reservation comes back (`p2p/relay.go`). Circuits are unlimited (`relay.go`, against the libp2p default of 2 min / 128 KiB).
- **QUIC and yamux.**
  - QUIC: keepalive 15 s, idle timeout 30 s (default), handshake idle 5 s (go-libp2p `quicreuse/config.go`, quic-go `internal/protocol/params.go`).
  - quic-go PTO doubles up to `maxPTODuration` 60 s (`internal/ackhandler/sent_packet_handler.go`, RFC 9002 §6.2.1).
  - yamux, on relayed links only: keepalive 30 s; a ping fails after 10 s to flush plus 10 s to answer (`ConnectionWriteTimeout`) (go-yamux v5.1.0 `session.go`).
- **Bridge.**
  - On a Send error, the bridge only **logs** (`internal/bridge/router.go`, `sendTo`).
  - When a peer's 64-message outbox is full, the bridge drops the message and calls `Reset` (`enqueue`).

| # | Failure mode a disturbed network can trigger | Code / spec element | Disposition |
|---|---|---|---|
| R1 | **Silent loss after a stalled write.** <br>• A blackout of 20–30 s (under QUIC's 30 s idle timeout, so no PeerDown) while a large message is being written: `WriteProgress` fails at 20 s, `Node.Send` replaces the stream and the payload is gone. <br>• The peer stays up, and `Router.sendTo` only logs. <br>• Automerge's `sentHashes` never resends, so the two sides diverge for good. | `stream.go` `Send`/`sendFailed`; `router.go` `sendTo`; inventory R-Evict; spec §5.5 rule L | **NI-2** |
| R2 | **Slowness treated as failure.** <br>• Below 128 kbit/s, a large message misses its total budget (20 s + len / 16 KiB/s) even while bytes are flowing, so it is lost as in R1. <br>• This contradicts spec §10.2 and ADR 0028 ("a slow relay must finish"). | `frame/progress.go`, `mux/fragment.go:189`; N-15; P-BYZ "relais à 8 Kio/s qui finit" | **NI-3** |
| R3 | **Typing starved behind a big transfer** on a slow path (one 32 KiB fragment takes 4 s at 8 KiB/s). | `mux/fragment.go` gate; spec §5.6.3, §7.2; ADR 0028 | **NI-3** |
| R4 | **Relayed link flapping under queueing delay.** A yamux ping blocked for more than 10 s, or a write blocked for more than 10 s, closes the relayed session; then PeerDown, redial and resync, over and over. | go-yamux `Ping`, `ConnectionWriteTimeout` | **NI-3** (stability clause) |
| R5 | **Loss or reordering across the relay → direct stream swap.** The old stream is drained under loss while writers resume on the new one; `reorderWait` orders the two. | `stream.go` `upgrade`, `handleStream` `inTail` | **G-0** (deterministic). NI-1 runs through the swap too. |
| R6 | **Hole punching under jitter.** DCUtR times its simultaneous open from a measured RTT; jitter makes that sample wrong. | go-libp2p `holepunch` (3 retries, 10 s direct-dial timeout) | **NI-1** (clause "direct within 90 s") |
| R7 | **Instability and wrong metrics under steady loss.** A spurious PeerDown under bursty loss; link-quality numbers that do not follow the real conditions (today `Links().RTT` is a single ping at stream open, via `LatencyEWMA`). | `links.go` `measureRTT`; W33 | **NI-1** |
| R8 | **Mesh: a change does not reach B when A–B is down.** <br>• Transitive sync through C must work. <br>• B's coverage needs A's attestation, which a third member (a reader) must relay. <br>• In v2 it also needs the AUTHOR tuples. | `app/attest.go` `offerAttest`; spec §6.2 (relays, readers included), §9.4, §4.7 | **NI-4** |
| R9 | **Mesh amplification or sync storm.** Duplicate delivery over two paths; ping-pong of sync messages; no quiescence once converged. | automerge-repo sync, `WailsNetworkAdapter.ts` | **NI-4** |
| R10 | **Deadlines that run during a blackout punish an honest peer.** <br>• The HELLO deadline (10 s) and the RESET_STREAM deadline after STOP_SENDING (10 s) can both fire during a 10–30 s blackout that QUIC itself survives. <br>• The result is GOAWAY (HELLO_TIMEOUT or PROTOCOL_VIOLATION), then Tenu with backoff up to 60 s: an honest peer is treated as a violator. <br>• `ConnCredit` lets `Teardown` fire even with `Honest` (an untimed model), so it does not see this cost. | spec §5.1 (c), §5.3, §11.1, §13, §16.1; `ConnCredit.tla` `Teardown` | **G-1** (needs a spec decision, Q1) |
| R11 | **v2 session tie-break disagrees under jitter.** Two simultaneous DCUtR connections reach the two sides in different orders; if the sides keep different sessions, both close and the link flaps. | spec §5.1 deterministic tie-break | **G-2** |
| R12 | **Repeated long outages, backoff never converging.** | `dial.go`, `relay.go` keeper | Already covered (§6) |
| R13 | Corruption reaching the app; packet reordering or duplication breaking our framing. | AEAD (QUIC TLS 1.3, Noise); ordered QUIC and yamux streams | Impossible by construction; folded into NI-1 at no cost (§6) |
| R14 | Credit deadlock under loss (`ConnCredit`). | spec §5.1, §10.7 | Loss only delays the reliable `MAX_DATA` frames; deadlock freedom is TLC's job (P-TLA). The loss-specific residual is R10. |
| R15 | Duplicate or reordered *sync messages* breaking admission or the generation rules (rule A). | spec §7.3; `admission.ts` | One peer pair rides a single ordered stream (`sync/1`) or a single REPO stream per generation (v2), so the network cannot reorder or duplicate them. The only real source of duplicates is a second path: NI-4. |

## 3. Netem conventions

- Names refer to `scripts/netsim/topology.sh`.
- A profile is applied by a new subcommand, `topology.sh impair NS DEV ARGS…` (`tc qdisc replace dev DEV root netem ARGS`), and removed by `topology.sh clear NS DEV`.
- Always set: `limit` (sized as in §1), and `seed $NETSIM_SEED`.
- Both directions of a host's access link = the egress of the NAT's `eth0` **and** of its `eth1`.
- **Anti-vacuity.** `topology.sh qstat NS DEV` prints the qdisc's sent/dropped/overlimits. Each scenario asserts that they moved.

| Profile | Where | netem arguments (each listed interface) | Meaning |
|---|---|---|---|
| `wifi` | nat-a `eth0`, `eth1`; nat-b `eth0`, `eth1` | `delay 15ms 5ms distribution normal loss gemodel 1% 30% 50% 0.1% duplicate 0.5% corrupt 0.1% limit 10000 seed S` | One way A→B crosses 2 qdiscs: 30 ms, so RTT ≈ 60 ms with σ ≈ 7 ms. Jitter reorders packets less than ~10 ms apart. GE: bad state 3.2 % of the time, mean burst 3.3 packets, about 1.7 % loss per qdisc, about 3.4 % per direction (the order of quic-interop's `transferloss`). |
| `wifi-far` | same | as `wifi` with `delay 75ms 10ms` | RTT ≈ 300 ms (a far relay or a satellite-like link). |
| `blackout` | nat-a `eth0`, `eth1` | `loss 100% limit 1000 seed S` | Alice's network silently stops; the **interface stays up** and the addresses stay. Scenario 3 only covers *detach*, which triggers an address-change kick; this profile does not. |
| `slowrelay` | relay `eth0` | `delay 20ms rate 65536bit limit 24 seed S` | 8 KiB/s (P-BYZ), shared by both directions of every circuit. Queue ≤ 24 × 1400 B / 8192 B/s ≈ 4.1 s, home-uplink bufferbloat, not the default limit's 170 s. |

## 4. The tests

Layer A = Go, `natNet` under `synctest`, 20 seeds (`forSeeds`). Layer B = netns with netem,
`netsim.spec.ts`, real `inkwell-server`, Chromium.

Each test lists its oracle clauses (C…) and its mutants (M…). A mutant is a text substitution
(anchor → replacement) applied to a copy of the tree. It must turn the named clause red: the same
mechanism as `scripts/alloy_check.py` and `docs/protocol/tla/mutants/`, and an anchor that is not
found is itself a failure.

### G-0 — Relay → direct swap under bursty loss keeps every frame, in order (`sync/1`)

- **Risk:** R5. It guards the `sync/1` contract "the sync stream follows it without any event or lost frame" (`stream.go` `upgrade`, `handleStream`).
- **Where:** Layer A. Extend `TestRelayThenDirect`; `natNet` gains GE loss and duplication from its seeded `rng`.
- **Profile:** jitter 0–20 ms (as today), GE (p 1 %, r 30 %, 50 % / 0.1 %), duplicate 0.5 %, applied to every packet including the relay's.
- **Steps:**
  1. Direct paths are blocked by the firewall until t = 5 s.
  2. From t = 0, a sends frames with sequence numbers 1…N, one every 50 ms of virtual time, until 30 s after `LinkDirect`.
- **Oracle:** C1, b receives exactly 1…N, in order, with no gap and no duplicate, in each of the 20 seeds. C2, exactly one PeerUp each side and no PeerDown (as today).
- **Mutants:**
  - M0a: in `upgrade`, `_ = old.CloseWrite()` → `_ = old.Reset()` (C1: gap).
  - M0b: `reorderWait = time.Minute` → `reorderWait = 0` (C1: reorder).
- **Life:** deleted at ADR 0034 step 10 along with `sync/1` and `mux` (ADR 0034, list of redundant tests).

### G-1 — A blackout does not make an honest peer a violator (v2 session, step 4)

- **Risk:** R10 (spec §5.1 (c), §5.3, §13).
- **Where:** Layer A, `internal/transport/session` behind the `sync2` flag.
- **Profile:** a per-end blackout added to `natNet` (today's `cut` is global), lasting 15 s: longer than both 10 s deadlines, shorter than QUIC's 30 s idle timeout.
- **Steps:**
  - Case H: the blackout starts 100 ms after the QUIC handshake, before the HELLOs.
  - Case R: in Synchro, a webview demand (rule L) makes A apply rule I, which sends STOP_SENDING to B; the blackout starts 100 ms later.
- **Oracle (with the fix proposed in Q1):**
  - C1: no HELLO_TIMEOUT or PROTOCOL_VIOLATION is recorded against the honest peer, and it is never Tenu.
  - C2: Active again in one generation (`GenGap`, `SyncedAgree` of `RepoGen`) within 15 s + 5 s after the blackout ends.
  - Both clauses over 20 seeds.
- **Mutant:** M1, the deadline counted on the wall clock instead of on the peer's activity (C1 red in both cases).
- **If Q1 keeps today's text:** C1 becomes "the Tenu after a blackout-induced teardown stays at 2 s (not escalated)".

### G-2 — Simultaneous direct connections under jitter resolve to the same session (v2, step 4)

- **Risk:** R11 (spec §5.1, rules 1–3 of the tie-break).
- **Where:** Layer A, cone `natNet`, jitter 0–20 ms, 20 seeds, **plus one constructed case**: A sees connection X first, B sees Y first.
- **Oracle:**
  - C1: within 10 s after either side's first direct session becomes Active, each side holds exactly one session to the other; it is direct, and it has the same key (nonce pair) on both sides.
  - C2: at most 3 sessions opened (the relayed one plus ≤ 2 direct), and at most 2 webview epochs.
- **Mutant:** M2, rule (3) "oldest Active" applied before rule (2) "larger key" (C1 red in the constructed case, deterministically).

### NI-1 — Bad Wi-Fi that changes: converge, stay up, report the truth (scenario 5)

- **Risks:** R6, R7, plus a realistic end-to-end baseline (R5 runs through it, R13 is folded in).
- **Topology:** `cone`; profile `wifi`, then `wifi-far`, then `wifi` with loss removed.
- **Steps:**
  1. `twoPeople`.
  2. Apply `wifi`.
  3. Both type seeded unique tokens (`a001 `…, `b001 `…), one every 500 ms, concurrently, over three phases of 40 s:
     - P1: `wifi`;
     - P2: `wifi-far`;
     - P3: `wifi` without `loss`, `duplicate` or `corrupt`.
- **Oracle:**
  - C1: path direct within 90 s of pairing (the `direct` NAT counter > 0 and `path = direct` in the metrics; same bound as scenario 1).
  - C2: both texts equal and every token present exactly once, within 15 s after the last keystroke. Derivation: at 300 ms RTT a PTO is ≈ 3 × (srtt + 4·rttvar + 25 ms) ≈ 1.3 s; three consecutive lost probes ≈ 9 s, which is < 15 s.
  - C3: zero PeerDown on either side. No blackout lasts 30 s (the idle timeout), and a GE burst averages 3.3 packets.
  - C4, metric truth: at the end of each phase, `rttMs` ∈ [0.8 × R, 1.5 × R + 20 ms] with R = 60 / 300 / 60 ms, and it is reached ≤ 20 s after the phase change.
  - C5: `lossPct` ∈ [1 %, 10 %] at the end of P1 and P2 (3.4 % injected per direction, plus spurious losses from reordering, quic-go `packetThreshold` = 3); ≤ 0.5 % at the end of P3 (30 s window).
  - C6, anti-vacuity: every netem qdisc dropped and duplicated > 0 packets in P1.
- **Mutants:**
  - M5a: RTT taken from the one-shot ping at stream open, as `measureRTT` does today (C4 red in P2).
  - M5b: `lossPct` computed since the connection started instead of over a 30 s window (C5 red at the end of P3).
  - M5c: holepunching disabled in `reachabilityOptions` (C1 red).

### NI-2 — A blackout shorter than QUIC's idle timeout, during a large paste (scenario 6)

- **Risk:** R1 (rule L, R-Evict, N-15).
- **Topology:** `cone`, direct path.
- **Steps:**
  1. `twoPeople`.
  2. Bob types tokens every 1 s throughout.
  3. At t0, apply `blackout` to Alice.
  4. At t0 + 1 s, Alice pastes 100 000 seeded characters drawn from U+4E00–U+5DFF (12 bits each, so ≥ 150 KB whatever the compression). This is more than QUIC can buffer without acknowledgements, so the write blocks.
  5. At t0 + 25 s, `clear`.
- **Why 25 s:** the margins are 5 s on both sides. Today's write gives up at 20 s (`WriteIdle`); the connection survives because no packet arrived for less than 30 s (QUIC idle).
- **Oracle:**
  - C1: zero PeerDown on either side.
  - C2: Bob has the whole paste and both texts are equal, within 35 s after restore. Derivation: after 25 s without ACKs the next PTO can be as long as the blackout itself (RFC 9002 §6.2.1, capped at 60 s by quic-go), so 25 s plus 10 s for transfer and application.
  - C3: every token Bob typed during the blackout reaches Alice within the same bound.
  - C4, anti-vacuity: the blackout qdiscs dropped > 0 packets.
  - C5, v2 only: no GOAWAY and no violation counter.
- **Expected today:** C1 green; C2 **red**. That is the characterization of R-Evict and N-15: it proves the scenario bites.
- **Mutant (v2):** M6, the session's REPO/LARGE writer gets back a 20 s progress deadline whose failure resets the stream *without* rule I, i.e. today's behaviour (C2 red).

### NI-3 — Slow relay (8 KiB/s): the big transfer finishes, typing elsewhere is not held back, the link stays up (scenario 7)

- **Risks:** R2, R3, R4 (spec §10.2, §5.6.3, §7.2; ADR 0028 D8; P-BYZ).
- **Topology:** `symmetric` (relayed only); profile `slowrelay`.
- **Steps:**
  1. `twoPeople`.
  2. Alice creates a second file `notes.typ` in the same project (two files, because tokens typed after the paste in the same document would causally depend on it and could not arrive first).
  3. Alice pastes 300 000 seeded CJK characters into `big.typ`: ≈ 439 KiB to 879 KiB on the wire, compressed or not. That is below `SLOW_CHARS` (512 Ki), so the editable tier does not change.
  4. During the transfer, Alice types a token in `notes.typ` every 2 s.
- **Oracle:**
  - C1: Bob's `big.typ` equals Alice's within 175 s. Derivation: 879 KiB / 8 KiB/s ≈ 110 s, × 1.3 for overhead, + 30 s.
  - C2: each token typed in `notes.typ` is visible at Bob within 10 s (p100 over ~50 tokens). Derivation: 4.1 s of netem queue, + 4 s for one 32 KiB fragment on `sync/1` (or QUIC stream interleaving on v2), + RTT.
  - C3: zero PeerDown, and the path stays relayed (`direct` counter = 0).
  - C4, anti-vacuity: the relay `eth0` qdisc sent ≥ 0.6 × 8192 B/s × the transfer time, and was overlimit > 0 times.
- **Expected today:** C2, C3, C4 green. C1 **red**, because on `sync/1` the message's budget is 20 + S / 16 KiB s < S / 8 KiB s for every S > ≈ 270 KiB (the N-15 characterization).
- **Mutants:**
  - M7a (v2): a rate-based total deadline, `frame.WriteProgress` semantics, on the LARGE writer (C1 red).
  - M7b (v2): the LARGE writer holds the REPO writer's lock (C2 red).
  - M7c (now and in v2): `libp2pyamux.DefaultTransport.ConnectionWriteTimeout = time.Second` in `New` (C3 red: the relayed yamux session closes when the queue holds it).
- **Variant for step 5:** the same profile with an image attached (the bulk lane) instead of the text paste: "a slow relay finishes" for blobs. The same scenario with another payload, not another test.

### NI-4 — Three-device mesh: converge through a third member, without amplification (scenario 8)

- **Risks:** R8, R9.
- **Topology:** `mesh`. A third host `carol` on the WAN with a public address 11.0.0.20 and no NAT (a desk machine); `nat-a` and `nat-b` symmetric; relay present.
- **Setup:**
  - Carol hosts the project and invites Alice, then Bob, so the tickets carry her public address.
  - Carol joins as a **reader** (Q3): a reader cannot attest, so B's coverage of A's changes depends only on the attestations C relays (spec §6.2).
  - Expected links: A–C and B–C direct, A–B relayed.
- **Phase 1, full mesh:** each of the three types 30 tokens, interleaved; then 30 s idle.
- **Phase 2, A–B really cut:**
  1. `topology.sh relay-down` (`ip -n relay link set eth0 down`). The A–B circuit dies; A–C and B–C do not use the relay.
  2. Wait until A and B each report the other gone (≤ 60 s: QUIC idle 30 s on the relay hop, then the yamux session).
  3. Alice and Bob type 20 tokens each.
- **Oracle:**
  - C1: the phase-1 links are as expected (metrics `path`).
  - C2, anti-vacuity: in phase 2, A has no link to B and B none to A, while both keep their link to C.
  - C3: all three texts equal, every token present exactly once, within 15 s after the last keystroke, in both phases.
  - C4, amplification: in phase 1, the `sync` messages sent by all three ≤ 2·n(n−1) = 12 per local change. Per change and per ordered pair, at most one message carrying it and one acknowledging the new heads. This is the only clause where the n(n−1) mesh cost appears, and it is bounded on purpose.
  - C5, quiescence: in the 30 s of idleness after convergence, ≤ 2 `sync` messages per ordered pair. A ping-pong never stops.
- **Mutants:**
  - M8a: `offerAttest` stores the attestation but no longer relays it (the loop over `activePeers`) (C3 red in phase 2: B's coverage refuses A's changes relayed by C).
  - M8b: the adapter emits `peer-disconnected` then `peer-candidate` for a peer on each message received from it, which resets the sync state (C4 and C5 red).
  - M8c (step 8): C does not forward AUTHOR tuples for the changes it relays (C3 red, G12).

## 5. Flakiness control and runtime

- **Seeds.**
  - `NETSIM_SEED` drives netem's `seed`, the typing schedule and the payloads.
  - It is printed in `results/<n>.json` with every clause's verdict and measurement.
  - The CI seed is derived from the date, so it rotates and is recorded.
- **No retries** in the gate (`retries: 0`, as today). A red run is replayed twice with the same seed, only to classify it:
  - still red: a defect;
  - green on replay: a flaky test, to investigate.
  - A bound is never relaxed (CLAUDE.md).
- **Bounds and load.** Every bound is derived from a cited timer with a margin of ≥ 20 %, so none is tuned by observation. Runs go through `scripts/heavy.sh` at rest (native-bench pitfall); NI-4 runs three Chromium pages.
- **Burn-in before freezing a clause as pass/fail:**
  - 10 consecutive green runs on distinct seeds;
  - a characterization clause must be red in 3 of 3 runs;
  - each mutant must turn its clause red in 3 of 3 runs (Layer A: in at least one of the 20 seeds, and always in the constructed case).
- **Runtime per seed:**
  - NI-1 ≈ 150 s, NI-2 ≈ 80 s, NI-3 ≤ 200 s, NI-4 ≈ 170 s: ≈ 10 min, plus the existing 155 s.
  - Layer A: seconds, in virtual time.
  - e2e mutants: 7 × ≈ 3 min ≈ 20 min, run at gate freeze, in `mise run done`, and nightly. Never more than 2 heavy jobs at once.
- **Targets:** CI per push ≤ 12 min (one seed); gate with 3 seeds ≈ 30 min, plus mutants ≈ 50 min; nightly 5 seeds.

## 6. Considered and rejected

| Candidate | Why not |
|---|---|
| Repeated long outages, backoff convergence (R12), end to end | `TestPartitionThenHeal` and `TestRoamToNewAddress` already prove the 30 s reconnect bound over 20 seeds, in virtual time, with a cut that keeps addresses; scenario 3 proves it end to end. A GE outage train would only measure it again, slowly. |
| Corruption-only scenario (R13) | AEAD (QUIC TLS 1.3, Noise) drops every corrupted packet before our code sees it. `corrupt 0.1%` rides along in NI-1 for free. |
| Reorder-only or duplicate-only scenario (R13, R15) | QUIC and yamux streams are ordered and deduplicated; packet reordering and duplication are inside NI-1. Duplicate *messages* only come from a second path: NI-4. |
| Separate hole-punching-under-jitter test (R6) | One clause of NI-1 (C1) at no extra cost. |
| NAT mapping timeout and rebinding (quic-interop `rebind-port`) | QUIC keepalive is 15 s, below common UDP NAT timers (RFC 4787 REQ-5 asks for ≥ 2 min). It also needs `nf_conntrack_udp_timeout` to be writable in a user namespace, which is unverified. Deferred (Q5). |
| Credit deadlock under loss (R14) | Exhaustively checked by TLC (`ConnCredit`) and carried by P-QUIC end to end; loss adds nothing but delay. The loss-specific residual is G-1. |
| Toxiproxy, Pumba, Testground, ns-3 (quic-network-simulator) | TCP only; containers only; deprecated; heavy, and redundant with quic-go's own interop matrix. |

## 7. Shared observability: the per-peer link metrics contract (W33)

Designed once. The impaired scenarios read **exactly** what the people popover and the status bar
will read. The UI itself (wording, quality grade, layout) is a later block with its own UX design,
inside the **existing** people popover (« directement » / « par un relais », « Envoi en cours (n) »,
« Lien lent », `docs/design/spec.md` §4.15 and the WAN row of the performance table) and the status
bar. No new panel.

**Ownership and flow (spec §15):**

1. `p2p.Node` samples its connections once per second. It publishes an immutable `[]Link` snapshot through `atomic.Pointer`: this extends today's `Links()` with the same type, not a parallel API. `memnet` implements the same optional interface for unit tests, through a type assertion, like `bridge.Resetter`.
2. `bridge.Router` merges its own per-peer queue figures into the snapshot.
3. The router pushes the result to the webview as a **coalesced state notification** (Annex A.1: last value per key), at ≤ 1 Hz and only when something changed.
4. The webview keeps it in a store. Under `VITE_E2E_OBSERVE`, `window.__inkwellLinks()` returns that store, following the pattern of `__inkwellVariantHeads`. The tests therefore read the UI's own data.

**Privacy (§18):** no IP address and no multiaddr crosses the bridge, only the kind of path. Nothing is ever logged with a payload.

| Field | Meaning | Source |
|---|---|---|
| `peer` | PeerID | — |
| `path` | `lan` / `direct` / `relayed` | `bestKind(ConnsToPeer)` (exists) |
| `pathSince`, `upgrades`, `fallbacks` | when the path last changed; relayed → direct and direct → relayed counts | sampler, from `onConn`/`upgrade` |
| `rttMs`, `rttVarMs`, `rttAgeMs` | smoothed RTT and its deviation; age of the last sample | direct: `network.Conn.As(**quic.Conn)` → `ConnectionStats().SmoothedRTT`, `MeanDeviation` (quic-go v0.64.1 `connection.go:796`; go-libp2p `p2p/transport/quic/conn.go:28`). Relayed: end-to-end libp2p ping every 10 s with our own EWMA (α = 1/4) instead of the peerstore's (its α of 0.1 is too slow for C4). |
| `lossPct`, `lossScope` | lost / sent packets over the last 30 s; `null` under 50 packets. `path`, or `firstHop` when relayed (only our hop to the relay is visible) | `ConnectionStats().PacketsLost`, `PacketsSent`; the counters are not monotonic, so deltas are clamped at 0 |
| `upSince`, `downs10m`, `downsTotal`, `lastDownMs` | stability: current session age, outages in the last 10 min, total, duration of the last outage | PeerUp/PeerDown in `stream.go` (`openOut`, `settleLocked`) |
| `pendingMsgs`, `pendingBytes`, `oldestPendingMs` | what waits to be sent to this person (« Envoi en cours (n) »; « Lien lent » when `oldestPendingMs` > 2000, ADR 0028 D8) | router outbox (needs an enqueue timestamp). v2: the bridge credits and the per-(peer, document) message held at the source (§7.4). |
| `sendFailures`, `resets` | writes given up (the R1 detector on `sync/1`); Reset on overflow (R-Reset) | `Router.sendTo` error branch, `enqueue` overflow branch |
| `genChanges`, `violations`, `held` (v2) | rule I count; GOAWAYs received or sent by code; Tenu state | `internal/transport/session` |
| `sentBytes`, `recvBytes` (by class in v2) | application payload bytes | atomic counters in `Send` and `handleStream`, then in the session. Hot path: proved with benchstat (`mux`, `app` are in `HOT_PATHS`). |
| webview: `syncSent`, `syncRecv` per peer | count of automerge-repo `sync` messages (NI-4 C4, C5) | `WailsNetworkAdapter.ts` |

**How the tests use it:**

| Test | Fields read |
|---|---|
| NI-1 | `path`, `rttMs`, `lossPct`, `downsTotal` (C1, C3–C5) |
| NI-2 | `downsTotal`; v2: `violations` |
| NI-3 | `path`, `downsTotal` |
| NI-4 | `path`, the link list (C2), `syncSent` (C4, C5) |
| G-0 | the transport events directly |
| G-1, G-2 | the session's `violations` and `held`, and the number of sessions |

NI-1's C4 and C5 are the metrics' own acceptance test: injected truth against reported value.

## 8. Integration

### Code and tooling (inside the existing structure)

- **`scripts/netsim/topology.sh`:**
  - `impair`, `clear`, `qstat`;
  - `up mesh` (adds `carol` to `NAMESPACES`, `WAN_HOST[carol]=20`, no NAT, symmetric `nat-a`/`nat-b`);
  - `relay-down` and `relay-up`.
  - Note: the NATs' `direct` counter also counts packets from Carol; scenarios 1 and 2 never start her.
- **`scripts/netsim/run.sh`:**
  - scenarios 5–8 in `MODE` and `TITLE`;
  - `PORT[carol]=18083`;
  - a third instance started only for `mesh`;
  - `NETSIM_SEED`;
  - per-clause `results/<n>.json`;
  - a preflight probe `unshare -Urn tc qdisc add dev lo root netem delay 1ms`, with the fix in the skip reason (`sudo modprobe sch_netem`). A user namespace cannot load the module, so the CI job loads it.
  - `NETSIM_SCENARIOS` keeps working as is.
- **`frontend/e2e/netsim.spec.ts`:**
  - four tests, `scenario 5:` … `scenario 8:`, sharing `converge`, `note`, `packets`;
  - a `checker` helper (the token history and its verdict, as in Jepsen).
- **`frontend/e2e/helpers.ts`:**
  - `pair(host, guest, people = 2)` (today it asserts "2 personnes");
  - `pasteAtEnd`, via `keyboard.insertText`;
  - `openFile`.
- **`internal/transport/p2p/sim_test.go`:** GE loss, duplication and per-end blackout in `natNet.RecvPacket`.
- **Mutants:** `scripts/netsim/mutants.toml`, entries of the form `{id, file, before, after, scenario, clause}`.
  - The runner applies each substitution in a temporary `git worktree` and builds into a separate `bin/`. Through `NETSIM_BIN_DIR`, the scenario must turn the named clause red; an anchor that is not found fails.
  - Layer A mutants are run the same way with `go test`.
- **mise:**
  - `e2e:netsim:impaired` (= `NETSIM_SCENARIOS="5 6 7 8"`);
  - `netsim:mutants`;
  - both through `scripts/heavy.sh`.
  - The new variables (`NETSIM_SEED`, `NETSIM_BIN_DIR`, `NETSIM_EXPECT`) are declared in `.env.reference` (`lint:env`).

### Which ADR 0034 step carries what

Each later step's gate (frozen before its code, as P2 and P3 were) adds a `netsim-impaired-regression`
criterion, in the same pattern as `P2-regression`.

| Step | Gate carries | Closes |
|---|---|---|
| now: block **NI0** (tooling + metrics v1 on `sync/1`) | G-0; NI-1 and NI-4 pass/fail; NI-2 and NI-3 run as characterization (C2/C1 red, the other clauses pass/fail); every NI0 mutant caught | W33 contract v1; the transitive mesh is tested for the first time |
| 4 (`sync2` session behind the flag) | G-1, G-2; NI-1 with `sync2` on | R10, R11 |
| 5 (non-blocking loops, bulk) | NI-3 bulk variant: C1 pass/fail | N-15 for the bulk lane |
| 6 (bridge credits, rules L and H, LARGE) | NI-2 C2 and NI-3 C1 become pass/fail under `sync2`; M6, M7a, M7b caught | R-Evict, N-15 (sync), R-Reset |
| 8 (SET, AUTHOR, ADR 0031 objects) | NI-4 with M8c | AUTHOR relayed by a third member |
| 9 (all P-* gates) | NI-3 is the end-to-end part of P-BYZ "relais à 8 Kio/s qui finit" | — |
| 10 (switch-over) | everything on `sync2` by default; G-0 deleted with `mux` | — |

### Proposed `gates/NI0.toml` (to freeze before the code)

```toml
# Frozen acceptance for block NI0 (W32 tooling + W33 metrics v1, lot « Collaborer sans perdre »).
# Plan: docs/plans/2026-11-08-netsim-impaired-plan.md.
raw_results = ["docs/plans/2026-11-08-netsim-impaired-plan.md", ".task/netsim/results/*.json"]
[[criteria]]
id = "netsim-regression"            # scenarios 1-4 unchanged
command = "NETSIM_SCENARIOS='1 2 3 4' mise run e2e:netsim"
[[criteria]]
id = "G-0"                          # 20 seeds, virtual time
command = "scripts/heavy.sh go test ./internal/transport/p2p -run 'TestRelayThenDirect' -count=1"
[[criteria]]
id = "links-contract"               # unit tests of the snapshot, the windows, the clamping, the privacy (no address)
command = "scripts/heavy.sh go test ./internal/transport/p2p ./internal/bridge -run 'Link' -count=1"
[[criteria]]
id = "NI-1-NI-4"                    # pass/fail, 3 seeds
command = "NETSIM_SCENARIOS='5 8' NETSIM_SEEDS='1 2 3' mise run e2e:netsim:impaired"
[[criteria]]
id = "NI-2-NI-3-characterized"      # C1/C3/C4 of NI-2 and C2-C4 of NI-3 green; NI-2 C2 and NI-3 C1 red (expected)
command = "NETSIM_SCENARIOS='6 7' NETSIM_EXPECT=characterize mise run e2e:netsim:impaired"
[[criteria]]
id = "mutants"                      # M0a, M0b, M5a-c, M7c, M8a, M8b each turn their clause red
command = "mise run netsim:mutants -- NI0"
[[criteria]]
id = "P3-regression"
command = "mise run gate -- P3"
```

## 9. What the code lacks today

1. **Metrics.**
   - `Links()` has a one-shot RTT and no loss, stability or pending figures (§7).
   - The bridge has no per-peer counters and no enqueue time.
   - The webview has no link store and no `__inkwellLinks`.
2. **`natNet` (`sim_test.go`):** loss, duplication and per-end blackout.
3. **netsim:** netem subcommands, `carol` and the `mesh` mode, `relay-down`, qdisc statistics, per-clause results, the preflight probe.
4. **e2e helpers:** a people count in `pair`, paste, a second file.
5. **A mutant runner for e2e and Go**, modelled on `alloy_check.py`.
6. **CI:** load `sch_netem` (and check `nf_tables`) before the netsim job.
7. **Spec:** a decision on R10 (Q1); the metrics contract added to Annex A.1 as a coalesced notification kind.

## 10. Open questions

- **Q1 (spec, R10):**
  - Should the 10 s deadlines (HELLO, RESET_STREAM after STOP_SENDING) count only while the peer's packets arrive (a QUIC-activity clock)?
  - Or should their expiry close the session with NO_ERROR and no Tenu?
  - Today a blackout of 10–30 s, which QUIC survives, turns an honest peer into a violator, with a backoff that doubles up to 60 s on a flaky link.
- **Q2 (CI cost):** run all four scenarios on every push (≈ 12 min), or NI-1 and NI-4 per push and NI-2 and NI-3 nightly until step 6?
- **Q3 (NI-4):** can the invitation flow (or an admin decision in the UI) make Carol a **reader**? Otherwise M8a cannot be caught: a writer's own attestation of its heads covers the changes it relays.
- **Q4 (W33 grading):** the UX block decides the friendly grades; is a first proposal wanted in the contract (e.g. "poor" if `rttMs` > 400, `lossPct` > 5 % or `downs10m` ≥ 3)?
- **Q5:** add a NAT-rebinding scenario later (needs `nf_conntrack_udp_timeout` writable in a user namespace), if field reports show short NAT timers?

## Sources

1. tc-netem(8): https://man7.org/linux/man-pages/man8/tc-netem.8.html
2. netem limit and bufferbloat (Hemminger, quoted): https://kiwix.lema.org/stackoverflow.com_en_all_2023-11/a/38277940 ; https://dev.to/dtaht/comment/1n51c ; ns-3 bug 2388: https://www.nsnam.org/bugzilla/show_bug.cgi?id=2388
3. netem chained-qdisc backlog patch: https://lists.openwall.net/netdev/2012/07/04/44 ; LWN: https://lwn.net/Articles/617070/
4. libp2p test-plans, hole-punch router: https://github.com/libp2p/test-plans/blob/master/hole-punch-interop/router/run.sh ; https://libp2p.io/blog/new-test-plans
5. QUIC interop runner test cases: https://github.com/quic-interop/quic-interop-runner/blob/master/testcases_quic.py
6. quic-go `integrationtests/tools/proxy` (v0.64.1, module cache); go-libp2p v0.51.0 `p2p/transport/quicreuse/config.go`, `p2p/protocol/holepunch`, `p2p/protocol/circuitv2/relay/resources.go`; go-yamux v5.1.0 `session.go`, `mux.go`
7. Toxiproxy: https://github.com/Shopify/toxiproxy
8. Pumba: https://alexei-led.github.io/post/pumba_docker_netem ; https://oneuptime.com/blog/post/2026-02-08-how-to-use-docker-for-chaos-engineering-with-pumba/view
9. Jepsen patterns: https://tessl.io/registry/testland/jepsen-patterns ; Garage Jepsen: https://git.deuxfleurs.fr/Deuxfleurs/garage/src/tag/v2.4.0/script/jepsen.garage
10. RFC 9002 §6.2.1 (PTO backoff): https://www.rfc-editor.org/rfc/rfc9002#section-6.2.1 ; RFC 4787 REQ-5: https://www.rfc-editor.org/rfc/rfc4787

## Decisions (user, 2026-11-08)

- **Q1 — protocol deadlines under an outage:** the 10 s HELLO and RESET_STREAM deadlines count only while
  the peer's packets are arriving; a network outage is never counted as a fault. This is a spec change
  (docs/protocol/spec.md, with its TLA+ model where the deadline is modelled), made in the ADR 0034 step
  that owns those deadlines, specified before the code like every protocol rule. G-1 guards it.
- **Q2 — when the impaired scenarios run:** on every push, all four NI scenarios (~12 min added to the
  pre-push CI). Scenarios expected to fail until their step (NI-2 until step 6, NI-3 until the LARGE
  writer fix) run as declared known failures with their target step, so they are executed and reported
  on every push without blocking it, and they turn into hard failures once their step lands.
- **Q3 — third device in NI-4:** both passes — Carol as an editor, then Carol as a reader — so the
  « change relayed by a member without write access » case is covered.
- **Q4 — link-quality grades:** the metrics contract includes a first proposal of plain grades
  (e.g. good / fair / poor) and their thresholds, derived from the shared metrics; the UX design of the
  people popover and status bar refines them later.
- **Q5 — NAT rebinding:** deferred; listed with the rejected-for-now candidates, to revisit after NI-4.

## Addition (user, 2026-11-08): sleep and resume

A laptop that sleeps is not a network outage: the app's process is frozen (no timers, no keepalives, no
reads), then resumes with a clock jump, possibly on another network. Planned scenario **NI-5 — sleep then
resume**, inside the existing netsim structure: freeze Alice's app process with SIGSTOP (her namespace's
network stays up, as on a real suspend the NIC goes down too — so both variants: with and without
`alice-detach`), while Bob keeps typing; then SIGCONT and, in the variant, `alice-attach`.
- Durations: 20 s (QUIC connection may survive: the same risk as NI-2 C3, frames sent around the freeze)
  and 90 s (beyond QUIC's idle timeout and the 15 s keepalive: the connection must be torn down and redialed).
- Oracles: both texts converge with every token typed by both sides; reconnection within the dial backoff
  bound after resume (`maxBackoff` = 1 min, `stableAfter` rule in internal/transport/p2p/dial.go); no
  honest peer counted as a violator (ties to Q1).
- Mutation proof: disable the redial on resume (or keep a stale session after the clock jump) — NI-5 must fail.

## Addition (user, 2026-11-08): an honest sync indicator (part of W33)

Today `statusOf` (frontend/src/workspace/workspace.svelte.ts) returns « synced » as soon as one peer is
connected: the header says « Synchronisé » even while changes are pending or lost. The metrics contract (§7)
therefore also feeds a truthful two-way sync state, in the existing header status (no new component):
- outgoing: pending messages and the age of the oldest (bridge `PendingMsgs`, `OldestPending`) →
  « Envoi en cours (n) »; stuck beyond a bound → a calm explicit message;
- incoming: per peer, whether the peer's heads are known to be ahead of ours (automerge-repo sync state:
  their heads vs. our heads) → « Pas encore à jour »;
- « Synchronisé » only when both are empty for every connected member.
Test oracle: in NI-1 and NI-2 the indicator must never read « Synchronisé » while a token typed on the
other side is missing; mutation proof: the current peer-count rule (`peerCount > 0`) must fail it.

## Formal specification first (definition of done, step 1)

Tests are not the spec. Every behaviour change in this plan is modelled and checked before its code, with
mutants the model must reject (method: docs/plans/2026-11-08-spec-executable.md; tools: `mise run formal`):
- **Q1 — deadlines count only while the peer's packets arrive:** extend the TLA+ model that owns the HELLO
  and RESET_STREAM deadlines (docs/protocol/tla) with an outage action; property: no honest peer is ever
  counted as a violator because of an outage; liveness: the session recovers after the outage. Mutant: the
  wall-clock deadline (today's rule) — the model must find the violation. Spec text updated with the model.
- **Honest sync indicator (W33):** model the indicator as a state machine (TLA+, or Alloy for the static
  rule) over pending-out, missing-in and connected members; safety: « Synchronisé » ⇒ nothing pending
  either way for any connected member; liveness: once the network is back and quiet, it reaches
  « Synchronisé ». Mutant: `peerCount > 0` ⇒ « Synchronisé » — the model must find the counterexample.
- **NI-5 sleep and resume:** model the redial after a frozen period with a clock jump (liveness: reconnected
  within the backoff bound); mutant: no redial on resume.
Each model, its properties and its mutants are frozen in the gate of the block that changes the behaviour
(agent `formal-modeler`), before that block's code — as P3's models were.

## Clarification after RL1 (2026-11-08): NI-2 C1 and rule L

RL1 applies spec rule L on sync/1: a send that fails while the QUIC connection survives now raises PeerDown then
PeerUp, so automerge-repo resets its sync state and re-sends the lost change. NI-2 C1 (« zero PeerDown on either
side ») therefore contradicts rule L as worded. Its intent is that the **connection** survives a blackout shorter
than QUIC's idle timeout: C1 is re-stated as « the transport connection is not torn down » (link metrics: no new
`DownsTotal` from a QUIC close; a rule-L resync is counted separately and expected). The metrics contract (§7)
distinguishes the two causes so the people popover can say « reconnexion » only for a real one.

## As built (2026-11-08): deviations from §4, justified

- **NI-3:** only `main.typ` is editable in this build, so Bob types the tokens into the shared file instead of
  Alice typing into a second file; the oracle (typing not held back beyond the bound, read from the document
  state) is unchanged. Revisit when G-0b (one document per file) lands.
- **NI-4 reader pass:** Alice hosts and makes Carol a reader, because the host cannot be demoted after a
  hand-over; the risk tested (a change relayed through a member without write access) is the same.
- **NI-1 C6:** `tc -s qdisc` does not report duplicated packets, so C6 checks drops only; duplication is still
  injected and its effect is covered by the convergence clauses.
- **Tokens:** each token is one atomic edit (concurrent keystrokes at the same place interleave and split them).
- **Oracles read the editor's document state, never the rendered page** (CodeMirror does not render parts of
  lines over 20 000 characters — this misread produced the false « lost tokens » of the first NI-2 run).

## Addition (user, 2026-11-08): editing mix — big pastes and deletions, undo/redo, single characters

Two buffers already exist: the editor coalesces keystrokes before writing the Automerge document
(frontend/src/editor/amSync.ts: deferred write, flushed on blur, visibilitychange, pagehide and by
`flushTyped`), and the bridge keeps a bounded per-peer outbox (internal/bridge/router.go; large messages are
fragmented by internal/transport/mux). The risks sit at their edges: undo/redo while typed text is still
buffered, a multi-MiB paste immediately undone, a large deletion during an outage, redo after the other side
edited the same range, outbox overflow under a burst.
- **EM-1 — model-based property test, no network (fast-check, already a dependency):** random sequences of
  {type one char, paste 1 B…4 MiB, delete a large range, undo, redo, flush, sync} on two editors joined by an
  in-memory automerge-repo; reference model: per editor a plain string plus its own undo/redo stack.
  Invariants: each editor always shows exactly its Automerge document; sequential runs match the reference
  model; after a sync both texts are equal; undo/redo never resurrects or loses the other side's text.
  Shrinking reports the shortest failing sequence. Mutation proof: undo without flushing the typed buffer,
  and a redo stack not cleared by a new edit — EM-1 must fail.
- **EM-2 — the same mix under impairment (netsim):** the bad-Wi-Fi and blackout profiles of NI-1/NI-2 with
  two and three devices; oracle: convergence, no lost token, document-state reads only.
EM-1 runs in the frontend unit tests (seconds); EM-2 joins the impaired scenarios on every push.

## Honest sync indicator: executable specification (W33, step 1, 2026-11-08)

Step 1 of the definition of done for the « Honest sync indicator » addition above: the rule, its TLA+
model and mutants, and the test the code must pass. The model is checked by `mise run tla`
(`docs/protocol/tla/SyncIndicator.tla`, README section "SyncIndicator").

### What the webview can observe

- **Presence of a peer:** `peer-candidate` / `peer-disconnected` from `WailsNetworkAdapter` (Go PeerUp/PeerDown,
  a re-`arrive`, and the PeerDown/PeerUp pair of a rule-L resync, spec §5.5). On `peer-disconnected`
  automerge-repo drops its sync state with that peer.
- **The peer's heads, per document:** the adapter already decodes every outgoing sync message
  (`A.decodeSyncMessage(data).heads` in `#attest`); it decodes incoming ones the same way, in
  `#receiveMessage` *before* `#enforce`, so that changes stripped by the coverage filter still count as
  known. This is the only acknowledgement there is: the peer's reply after applying my changes carries its
  new heads. automerge-repo's own `getSyncInfo` is not usable: it is kept per `storageId`, and our peers are
  ephemeral.
- **My heads:** `A.getHeads(handle.doc())`.
- **Links notification (§7):** `pendingMsgs`, `oldestPendingMs`. It is a ≤ 1 Hz coalesced snapshot that
  also counts presence messages. It never decides « Synchronisé ».

### The rule, per peer p and document d

`theirHeads[p][d]` is the heads of the last sync message received from p for d since p was announced.
It is cleared on `peer-disconnected`.

| State | Condition (first that holds) | Header wording |
|---|---|---|
| offline | p not announced | (p not counted) |
| checking | p announced, and no sync message from p yet for a document we sync with p | « Vérification… » (UX to word) |
| behind | some d with `!A.hasHeads(myDoc[d], theirHeads[p][d])`: I know p has changes I lack | « Pas encore à jour » |
| sending | some d with `!sameHeads(A.getHeads(myDoc[d]), theirHeads[p][d])`: p has not acknowledged a change of mine | « Envoi en cours (n) », n from `pendingMsgs` when > 0 |
| synced | otherwise | « Synchronisé » |

**Globally:**

- With no announced peer, the header keeps today's `alone` / `connecting`. It is never « Synchronisé »,
  because an empty `every` is true (mutant M-EveryVacuous).
- Otherwise it shows the worst state among the announced peers, in the order behind > sending > checking >
  synced.
- « Synchronisé » requires every announced peer to be synced.
- `oldestPendingMs` > 2000 keeps adding « Lien lent » (ADR 0028 D8).

**Two limits that hold by construction, and the wording must not hide them:**

- A change the peer made but has not sent yet is not known. « Synchronisé » means "everything I know of".
- The rule speaks only of connected members. A member who is offline and lacks my changes does not prevent
  « Synchronisé » (Q-UX3).

### Model and properties (`SyncIndicator.tla`)

**What the model contains:**

- **Devices:** Me plus 1 or 2 peers.
- **Sync protocol:** one ordered stream per direction, Automerge `sentHashes`, and causal application of changes.
- **Environment:**
  - bounded edits, online or offline;
  - transport losses, each one owing a rule-L resync;
  - connection drops and returns;
  - Links samples.
- **Properties:** they are checked against the ground truth (the peer's real document), which the indicator
  never reads.

| Property | Proves |
|---|---|
| `PeerHonest` | per peer: « Synchronisé » ⇒ the peer is connected and has been heard since the (re)connection, it really holds every change of mine, and I hold every change it reported |
| `GlobalHonest` | header « Synchronisé » ⇒ at least one connected member, and `PeerHonest`'s condition for each connected member |
| `EventuallySynced` | under weak fairness on sending, receiving, rule-L resync, reconnection and Links samples, with bounded edits, losses and outages (the network becomes quiet and connected), the header reaches « Synchronisé » and stays there |

**Results (TLC 1.8.0, 2 workers):**

- One peer (MaxEdits 2, MaxLoss 1, MaxDown 1, QMax 2): exhaustive, no error, 118 301 distinct states, 11 s.
- Two peers (MaxEdits 1, MaxLoss 0, MaxDown 1, QMax 1): exhaustive, no error, 530 696 distinct states, 60 s.
- Every mutant below is caught; the whole `mise run tla` run takes about 1 min 33 s.

**Mutants, each caught:**

| Mutant | What it breaks | Property that catches it |
|---|---|---|
| M-PeerCount | today's `statusOf` | `PeerHonest` |
| M-IgnoreRemote | the peer's heads are ignored. Counterexample: p's first change is lost, its second arrives without its dependency, Me still shows « Synchronisé ». | `PeerHonest` |
| M-SentIsAcked | `lastSentHeads` counts as acknowledged | `PeerHonest` |
| M-QueueOnly | `pendingMsgs = 0` | `PeerHonest` |
| M-EveryVacuous | `every` on an empty list | `GlobalHonest` |
| M-AnyPeer | `some` instead of `every` | `GlobalHonest` |
| M-NoRuleL | no PeerDown/PeerUp after a loss | `EventuallySynced` |

### The executable test the code must pass

- **Code shape:** `frontend/src/net/syncIndicator.ts` exports two pure functions, `peerSyncState(observed)` and
  `globalSyncState(states)`.
  - `observed` holds exactly the observables above: announced, the heads per document, my docs, and Links.
  - The adapter stores the heads; `Workspace.status` derives from `globalSyncState` instead of `statusOf(peers.length)`.
- **Test:** Vitest, `frontend/src/net/syncIndicator.model.test.ts`, a fast-check `fc.commands` model-based test.
  - **Name:** `W33 sync indicator follows SyncIndicator.tla: PeerHonest, GlobalHonest, EventuallySynced`.
  - **System under test:** three real automerge-repo `Repo`s (me, p1, p2), each behind a real
    `WailsNetworkAdapter`, joined by a step-by-step variant of `SimNet` (frames are held per directed link until a
    command delivers them).
  - **Commands:**
    - `Edit(node)`;
    - `Deliver(link)`;
    - `Lose(link)`: drops the head frame and owes a resync;
    - `Resync(p)`: PeerDown then PeerUp on both sides;
    - `Down(p)` and `Up(p)`;
    - `Links(p)`: sends `Kind.Links` with `pendingMsgs` set to the held frame count.
- **Oracle, after every command (safety)**, for each peer p whose state is `synced`, with peer docs read from the
  peer repos:
  - p is announced at me;
  - `A.hasHeads(peerDoc, myHeads)` for every document;
  - `A.hasHeads(myDoc, theirHeads)`.
- **Global oracle:** `synced` ⇒ at least one announced peer, and every announced peer satisfies the above.
- **Liveness oracle:** after the sequence comes the quiet phase:
  - every peer is brought up and every owed resync is done;
  - frames are delivered until none is held, at most 50 rounds;
  - then `globalSyncState` must be `synced`.
- **Runs and seeds:** 200 runs, with a seed printed on failure and shrinking on.
- **Mutation proof:**
  - the seven mutants above, as anchor → replacement substitutions on `syncIndicator.ts`, are listed in the
    block's frozen gate;
  - each must turn the test red;
  - M-PeerCount is the current rule, `peerCount > 0` ⇒ `synced`.
- **End to end:** the NI-1/NI-2 clause « never Synchronisé while a token of the other side is missing » reads
  `globalSyncState` through `__inkwellLinks`' store.

### Open questions for the UX design

- **Q-UX1, the meaning of n:** what does « (n) » count?
  - bridge `pendingMsgs` (messages, presence included), as planned;
  - or my unacknowledged changes (`A.getChanges(doc, theirHeads).length`, exact but large while typing);
  - or the people still waiting?
- **Q-UX2, behind and sending at once:** the model shows « Pas encore à jour ». Should both be shown?
- **Q-UX3, offline members:** « Synchronisé » covers connected members only. Should a known member who is
  offline and lacks my changes be named (e.g. « Bob n'a pas encore vos modifications »)?
- **Q-UX4, the checking state:** what wording, and after what delay should it be shown, to avoid a flicker
  right after each (re)connection or rule-L resync?
- **Q-UX5, hysteresis while typing:** every keystroke batch makes the state « sending » until the reply comes
  (one RTT). Should « Envoi en cours » show only after, say, 1 s?
- **Q-UX6, changes stripped by coverage:** changes the coverage filter strips (a reader's forgery, or an
  attestation that never arrives, NI-4 M8a) leave the peer « Pas encore à jour » for good.
  - Count only covered heads?
  - Or say why?
- **Q-UX7, stuck bound:** after how long in behind or sending does the calm explicit message replace the
  state? ADR 0028 D8 already gives « Lien lent » at 2 s of `oldestPendingMs`.

## Decision 2026-11-09: NI-3 C2 declared known, NI-2 C2 strict, block WB opened

- **Measured** (scenario 7, 8 KiB/s relay, two seeds per tree): worst delay of a typed token 48.9 s and
  44.0 s with rule L (block RL1, 1-2 resyncs), 21.0 s and 18.7 s before it (809efec). Cause: a write
  may take at most `frame.WriteIdle` + size / `frame.MinRate` (20 s + size / 16 KiB/s); a 300 KB paste
  on a link under 16 KiB/s always exceeds it, the failed write resyncs (rule L) and the paste is sent
  again from the start, ahead of the typing. Before RL1 the paste was silently lost instead.
- **Decision (user, 2026-11-09):** NI-3 C2 is a declared known failure until ADR 0034 step 6 (a
  separate lane for large objects, so typing is never queued behind them), next to NI-3 C1. NI-2 C2 is
  flipped to strict: rule L closed the silent loss and it passed on every run since.
- **Block WB (opened, spec first):** the sender's write budget follows the link's measured throughput
  (link metrics v1) instead of the fixed 16 KiB/s floor, with a lower bound that keeps the slow-reader
  protection, so a large message on a slow but progressing link is not failed and resent in a loop.
  Executable spec first: a TLA+ or model-based test showing no resend loop while bytes progress, the
  mutant « fixed floor » caught, and NI-3 C2 measured again (expected back under the RL1 baseline).
