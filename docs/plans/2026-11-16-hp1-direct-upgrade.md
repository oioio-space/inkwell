# HP1 - a relayed link to a wanted peer keeps trying to become direct

Status: **draft, step 1** (2026-11-16). Rigour: **complete** (state between peers, bytes and traffic driven by a
peer). Worktree `../inkwell-hp1`, branch `hp1`, from `main` (`c560c9a`). Gate: `gates/HP1.toml` (to write from
section 9). Model: `docs/protocol/tla/DirectUpgrade.tla` (to write from section 6). Integrator's decision
(PROGRESS.md, « Reprise », point 1): retry the upgrade in `internal/transport/p2p` with a bounded backoff; rejected:
making the bench wait, widening the go-libp2p fork.

## 1. Problem and evidence

Since PC1b, a device that joins restarts its process and dials its peers at once. After that restart a relayed link
is never upgraded to a direct one. Reproduced on an idle machine (`e2e:netsim:impaired`, seed 20772, logs of the
2026-11-15 session, `scratchpad/pc1b-netsim/repro{5,8}-evidence/`, libp2p debug lines):

- **Scenario 5** (alice invites, bob restarts): 21:29:56.520 alice starts a punch towards bob's *first* process;
  56.618 bob's side logs `failed to dial: swarm closed` (the old process ends); 56.690 bob's second process arrives
  over the relay and libp2p's automatic attempt fails with `another hole punching attempt to this peer is active`;
  30:01.521 round 1 of the old call ends after the QUIC hole-punch wait (`hole punching attempted; no active dial`);
  its round 2 happens to open its stream on the *new* circuit and succeeds at 01.526. C1 passes 5 s late, by luck
  (a call already in its third round would not have been rescued). C3 fails every run: alice counts the end of bob's
  first process (`downsTotal 1, 0`, `results/5-s20772.txt`: `C3 fail`).
- **Scenario 8, reader variant** (alice hosts; carol then bob join, each restarts; alice-bob direct path cut by the
  topology): 38:04.689 carol's automatic attempt towards bob's new process ends with `failed to open hole-punching
  stream: failed to negotiate protocol: protocols not supported: [/libp2p/dcutr]`; nothing retries; bob-carol stays
  relayed (C1 red), and once the relay goes down bob has no link (C2, C3 red). alice-bob ends with `all retries for
  hole punch with peer … failed` (38:19.276): correct, that pair is meant to stay relayed.

Causes, read in the pinned source (fork `oioio-space/go-libp2p v0.51.1-0.20261109121346-cae49c803966` = v0.51.0 +
one commit for issue #3575; upstream v0.52.0 of 2026-11-14 changes nothing in `p2p/protocol/holepunch`):

| Fact | Where |
|---|---|
| The side that **receives** the relayed connection (DCUtR's B) initiates, once identify has run on it; the side that dialled answers. A stream that arrives on the answerer's *inbound* connection is reset. | `holepuncher.go:276-297`, `svc.go:236-245`; DCUtR spec |
| One `DirectConnect` = a direct dial if the peerstore holds a public address of the peer, then **at most 3 rounds** (CONNECT, SYNC, punch), roles switched on the last; a stream or protocol error ends the call at once. No retry after that, ever. | `holepuncher.go:23, 103-183` |
| `/libp2p/dcutr` is registered only once the host has a public hole-punch address, polled at 250 ms, 500 ms, 1 s… up to 5 s; the hole puncher (and its connection notifiee) is created at the same moment. A relayed connection that arrives before that gets **no attempt at all**. In the netsim the address is known at listen time, so the window is the first 250 ms tick; in production it waits for the relay's identify (observed address), up to several seconds. | `svc.go:113-153` |
| `ErrHolePunchActive`: one call per peer per host; a second one fails at once, without traffic. | `holepuncher.go:21, 68-83` |
| QUIC punch (B side) sends 64-byte random packets every 10 ms + U[0, min(10(i+1)², 200)) ms for `HolePunchTimeout = 5 s`. | `p2p/transport/quic/transport.go:35, 195-260` |
| The hole-punch address list includes observed addresses seen by **one** observer (`observedAddrsManager.Addrs(1)`), which the public `Host.Addrs/AllAddrs` do not expose. | `p2p/host/basic/addrs_manager.go:488-499` |
| The service is a private field of the host (`hps`), but `holepunch.Option` is `func(*Service) error`, applied to the host's own service; `WithAddrFilter` filters the addresses sent (`FilterLocal`) and received (`FilterRemote`) in **both** roles, before any punch packet. | `basic_host.go:258-268`, `svc.go:41, 99-104, 175-214`, `holepuncher.go:222-256`, `filter.go` |
| `DirectConnect` waits on `hasPublicAddrsChan`, which is never closed if the service closes first. | `svc.go:145-153, 282-288` |
| Identify pushes a change of the local protocols to every peer; the receiver emits `EvtPeerProtocolsUpdated{Added}`. | `identify/id.go:255-305, 740-744` |

Upstream: the race (a) is go-libp2p issue #3024 (closed without a fix: « you'll need a public address »), the restart
case is #3420 (open; a maintainer: « very hard to support holepunching during a node restart », the reporter asks for
a retry). Nobody fixes it upstream; the remedy belongs to the caller.

Before PC1b the guest dialled seconds after its start, after its own handler was registered: the races were hidden.
They also hit real users outside PC1b: any restart, crash or network change of a peer behind a NAT (R1 acceptance:
« tuer et relancer les deux pairs »).

## 2. Prior art, and what is borrowed

| System | What it does about a stuck relayed path | Borrowed |
|---|---|---|
| DCUtR spec / go-libp2p | B tries 3 times, then « the peers keep using the relay connection »; no later attempt | nothing to fix there; the role rule (B initiates) |
| iroh 1.4 (`remote_state.rs`) | hole punching is **triggered by events** (new connection, local or remote candidate addresses changed); without new candidates, not more often than `HOLEPUNCH_ATTEMPTS_INTERVAL = 5 s`; `UPGRADE_INTERVAL = 60 s`; « both endpoints may initiate if both have a client connection » | event triggers; « new candidates » resets; both sides eligible when both hold a connection of the initiating kind |
| Tailscale (`magicsock`, 2026-11-12) | while the best path is not direct, full discovery pings on each 3 s heartbeat, only while the session is active (45 s); `upgradeUDPDirectInterval = 1 min`; path state reset when the peer's disco key changes (`invalidateDiscoPathLocked`) | 1 min as the base of the slow retry; reset on peer identity epoch |
| WebRTC ICE | ICE restart on network change (RFC 8445 §2.4, `restartIce()`), consent freshness every 5 s ±20 %, 30 s expiry (RFC 7675) | local network change as a reset event; jittered periods |
| Measurement (Trautwein et al., arXiv 2510.27500 and IMC'26 version 2604.12484) | DCUtR succeeds ≈ 70 % ± 7 %; 97.6 % of the successes on the first attempt; failures mostly endpoint-dependent NAT mappings | blind fast retries after a real punch failure have low yield: back off slowly, retry fast only when nothing was punched |

Improvement over all of them: the attempt budget is enforced at the **one choke point every punch goes through**
(libp2p's `AddrFilter`, both roles, libp2p's own attempts included), so a peer cannot make us punch without bound by
initiating, reconnecting or toggling its protocols, and the outcome of a call is classed by what was observed (rounds
punched, connection made), not by parsing error strings.

## 3. Design

### 3.1 Data flow and packages

```
libp2p host ── EvtPeerIdentificationCompleted, EvtPeerProtocolsUpdated, EvtLocalProtocolsUpdated,
   │           EvtLocalAddressesUpdated, Notifiee.Connected(circuit)           (node.go subscriptions)
   ▼
internal/transport/p2p  upgrade.go: per-wanted actor = the existing dialLoop goroutine, in its « connected »
   │                                branch; builds an Observation, asks the policy, runs the action
   │                     punchgate.go: holepunch.AddrFilter (wanted check, round budget, address cap, FIFO
   │                                   admission to a node-wide slot with a lease: every round, every origin)
   │                     gate.go: ConnectionGater.InterceptAddrDial (wanted check, destination bucket: every
   │                              non-relay dial, every origin, libp2p's own direct dials included)
   ▼                     relay.go: EnableHolePunching(WithAddrFilter(gate), capture(&n.hps))
protocol/directup (pure, sans I/O, time passed in, like protocol/linkprobe): Policy, Bucket, Classify
```

- `n.hps` is the host's own `*holepunch.Service`, captured by a `holepunch.Option` closure at construction; no
  reflection; it keeps libp2p's one-observer hole-punch addresses (an own `Service` would lose them).
- **The only fork change (D1):** `(*holepunch.Service).DirectConnectContext(ctx, p)`, the existing `DirectConnect`
  with a context: the call returns when `ctx` ends (its rounds, its first direct dial and the wait on
  `hasPublicAddrsChan`, `svc.go:145-153, 282-288`, all select on it) and releases libp2p's per-peer lock. A few lines
  in `holepuncher.go` and `svc.go`; upstream issue and PR filed with it (it also fixes R2). It keeps H-6's « ≤ 3 s »
  (a new epoch cancels the old call instead of waiting up to 3 rounds), H-4 (one call in flight, a cancelled one
  until it returns, which is prompt) and Close (cancels every call, waits for none).
- `protocol/directup` (new, pure): `Policy` per peer (state, ladders, epoch), `Bucket` (token bucket on a given
  clock), `Classify(rounds, directAfter, err kind) Class`. Vectors in `protocol/vectors/directup/`. Q1 discusses the
  placement.
- Session relayed = `ps.out` runs on a circuit (as in `maybeUpgrade`), not `bestKind`: a stale direct connection to a
  dead process (#3420) must not count as direct.

### 3.2 State machine per (local node, wanted peer p)

Observation (read at each step, no lock held across libp2p calls): `wanted`, `relayed` (session on a circuit),
`direct` (session on a non-circuit connection), `in` / `out` (we hold an inbound / outbound circuit to p), `epoch`,
`localReady` (`/libp2p/dcutr` in `h.Mux().Protocols()`), `remoteReady` (identify done on the epoch's circuit and its
protocols, or a later push, list `/libp2p/dcutr`), `remotePublic` (p announced at least one public non-circuit
address through identify), `lower` (our PeerID < p's), `slot` (a global slot is free).

States: **Off** (not wanted, no session, RelayOnly or no relay), **Direct**, **NotReady** (relayed, an attempt is not
possible: `localReady` or role missing; `remoteReady` is a trigger, not a guard, see H-2), **Scheduled(due)**, **InFlight(epoch, action, rounds0)**,
**Settling(until)** (a call succeeded, the session has not moved yet).

Actions: **Punch** = `n.hps.DirectConnect(p)` (only with `in`); **Dial** = `h.Connect(WithForceDirectDial)` to p's
public announced addresses, peerstore not cleared (only with `out` and not `in`, and `remotePublic`).

| From | Event / condition | To, effect |
|---|---|---|
| Off | session up on a circuit, wanted (E1) | NotReady or Scheduled(now) |
| any relayed | new circuit connection to p (E7, peer restarted or moved) | `epoch++`, ladders reset, Scheduled(max(now, lastStart + T_min)); an InFlight call is cancelled (D1) and its result is stale (H-6) |
| NotReady | E2 identify done, E3 push adds `/libp2p/dcutr`, E4 our handler registered, role gained | Scheduled(max(now, lastStart + T_min)) |
| Scheduled (`localReady`, role, not `remoteReady`) | poll timer of the fast ladder | a real call: a missing handler costs one stream, no gate round, class Unspent, one attempt token (covers a lost push or identify, which libp2p never resends) |
| Scheduled | `now ≥ due`, ready, role, attempt token, and the app active unless the due time came from an event (H-17) | waits for a node-wide slot in the FIFO queue (H-5) |
| Scheduled, due from a ladder | the app goes idle or the machine sleeps | suspended; on activity, due at once (H-17) |
| Scheduled, queued | at the head of the queue and a slot is free | InFlight; token taken (not for Busy) |
| Scheduled, queued | an event moves it out of « due » (E3, E7, unwant…) | leaves the queue |
| Scheduled | no token | Scheduled(token refill time) |
| InFlight | result of the current epoch, class Success | Settling(now + T_settle) |
| InFlight | Busy (`ErrHolePunchActive`; the slot is given back at once) | Scheduled(now + T_busy); no token, ladders unchanged |
| InFlight | Spent (≥ 1 round passed the gate during the call, or Dial reached the network; no direct connection) | Scheduled(now + slow(k)); `k++` |
| InFlight | Unspent (nothing punched, no direct connection) | Scheduled(now + fast(j)); `j++` |
| InFlight | result of an older epoch | slot freed only; Scheduled per E7 |
| Settling | session on a direct connection | Direct |
| Settling | `until` passed, still relayed | `maybeUpgrade` nudged once; class Unspent |
| any relayed | E8: our non-circuit addresses changed (iroh « new candidates ») | slow ladder reset, Scheduled(max(now, lastStart + T_min)) |
| Direct | session falls back to a circuit (direct lost) | NotReady/Scheduled, ladders reset, **budgets kept** |
| any | unwanted, no session left, node closing | Off; nothing new starts; an InFlight call is cancelled, its result discarded |
| InFlight | the slot's lease ends | the call is cancelled (D1), classed by what it observed |

Only one of our calls per peer is ever in flight (H-4). Close never waits for one: the call runs in a goroutine whose
only effect is a send on a channel of capacity 1; Close cancels its context (D1) and returns.

### 3.3 Timers, backoff and budgets

| Name | Value | Source |
|---|---|---|
| T_min, least spacing of two calls to p | 1 s | `minBackoff` of the dial loop (`dial.go`); event-driven calls coalesce (iroh) |
| T_busy | 2 s ± 25 % | Busy costs no traffic (`beginDirectConnect` fails before any I/O); the blocking call is ≤ 5 s per round (`HolePunchTimeout`) |
| fast ladder (Unspent) | 1 s · 2^j, cap 30 s | cap = quic-go's default `MaxIdleTimeout` 30 s: a stale connection is gone by then |
| slow ladder (Spent) | 60 s · 2^k, cap 16 min (k ≤ 4) | base: Tailscale `upgradeUDPDirectInterval`, iroh `UPGRADE_INTERVAL` (both 60 s); exponential because 97.6 % of successes are first attempts (Trautwein); cap chosen (Q3): a hopeless pair costs ≈ 4 calls an hour |
| jitter | ± 25 % on every due time | `jitter()` of `dial.go` |
| T_offset | +3 s on the due times of the higher PeerID while both sides hold an inbound circuit | > T_busy; a DCUtR coordination is about one RTT, far below |
| T_settle | 15 s | `openTimeout`, the bound of `upgrade()`'s stream open |
| attempt budget, per peer | bucket of 4, refill 1 per 7.5 min: **≤ 4 + 8 = 12 non-Busy calls in any hour** (burst included), 8 an hour sustained | matches the slow ladder's first hour (t = 0, 1, 3, 7, 15, 31, 47 min…); events cannot exceed it |
| round budget, per peer, both roles (gate) | bucket of 9, refill 1 per 100 s: **≤ 9 + 36 = 45 rounds in any hour**; **0 for a peer that is not wanted** | 8 calls × 3 rounds + libp2p's own 3 per new circuit + slack |
| destination budget, per peer, every origin (gater) | bucket of 30, refill 1 per 25 s: **≤ 30 + 144 = 174 non-relay destinations in any hour**; **none for a peer that is not wanted** | 45 rounds × 3 addresses + 12 calls × 3 for DirectConnect's first direct dial |
| node-wide admissions (H-16) | bucket of 8, refill 1 per 120 s: ≤ 8 + 30 = 38 slot grants in any hour, all peers and origins, granted in FIFO order | the energy budget (B3) |
| destination allowance per grant (H-16) | 12 non-relay destinations, all the peer's origins together, while it holds the slot (several rounds may share one grant, never its allowance); **no non-relay destination outside a grant**, libp2p's first direct dial of its one-shot included | one call: 3 for the first direct dial + 3 rounds × 3 addresses; node ≤ 38 × 12 = 456 destinations in any hour |
| slot lease | 75 s from the grant; early release when the round or call ends; a grant to a ticket whose round gave up is held for its peer for the lease | ≥ `StreamTimeout` 60 s (`svc.go:30`, the deadline set on the exchange's stream) + `directDialTimeout` 10 s (`svc.go:22`): libp2p's own deadlines end the holder's work first |
| activity (H-17) | ladders run only while the app is active (input within 5 min, not asleep); events call at once | Tailscale stops its endpoint heartbeat once the session is idle (`sessionActiveTimeout`, `endpoint.go`) and restarts it on traffic |
| addresses accepted from a CONNECT | ≤ 3 public, non-circuit: first QUIC/IPv4, first QUIC/IPv6, first TCP/IPv4 | spec's 4 KiB message bound allows ~100; we bound the fan-out |
| global concurrency | ≤ 4 node-wide slots, **every origin** (our calls, libp2p's automatic attempts, the answerer side: a round is admitted in the address filter, `FilterLocal`, called first in both roles, `holepuncher.go:224`, `svc.go:177`), granted **FIFO**, one ticket per peer (a peer that keeps asking never takes a slot while another waits), none for a peer that is not wanted (B4); the filter blocks a waiting round up to 20 s (< `StreamTimeout`), then the round gives up but **the ticket keeps its place** (B6) until its peer is no longer wanted or relayed, or 20 min without any round; when it reaches the head its grant is held for the peer for the lease (75 s), which an honest peer's fast ladder (≤ 30 s · 1.25) reaches | `wakeBurst = 4` (`dial.go`), memory rule of the project; FIFO as `golang.org/x/sync/semaphore.Weighted` serves its waiters in order |

A bucket is dropped once full (no information is lost), so budgets survive reconnections, epochs and un-want/re-want.

Traffic bound, per destination: as B (QUIC) ≈ 48 packets × 64 B ≈ 3 KB expected, ≤ 500 packets (32 KB) in the
worst draw; as A one QUIC dial ≤ ~12 KB; a round plays one role, so ≤ 32 KB per destination; DCUtR messages ≤ 4 KiB
each over the relay. **Worst case per Byzantine member and hour** (any window, bursts included): 174 destinations ×
32 KB + 45 exchanges × 2 × 4 KiB ≈ 5.9 MB (≈ 13 kbit/s), all of it to addresses the member chose (its share of the
node's grants, 12 destinations each, cannot exceed its own destination bucket). **Per node**, every origin counted
(our calls, libp2p's one-shot and its first direct dial, the answerer side, rounds sharing a grant), whatever the number
of members (B3): ≤ 38 grants × 12 destinations = 456 destinations × 32 KB ≈ 14.6 MB, plus ≤ 38 × 3 exchanges × 8 KiB
≈ 0.9 MB of DCUtR messages over relays: **≤ 15.5 MB/h (≈ 34 kbit/s)**; nothing while idle except event-driven
calls. The earlier « 76 × 3 ⇒ 7.3 MB/h » did not count libp2p's first direct dial (outside any grant) nor several
rounds sharing one grant.
Expected for a hopeless honest pair: ≈ 4 calls × 3 rounds × 3 × 15 KB ≈ 0.5 MB/h, only while the app is active.

### 3.3a Two choke points (D2, D3)

The address filter alone bounds what a CONNECT carries, not what is dialled: `DirectConnect` first dials **every
address of the peerstore** once one public address is known (`holepuncher.go:113`), and every round dials through
`host.Connect`, which adds the CONNECT addresses to the peerstore and dials all of them (`util.go:51`,
`basic_host.go:547-558`; our `dial.go:312` clears the peerstore for that reason). The bound on destinations is
therefore enforced where every outbound dial passes, whatever its origin: `ConnectionGater.InterceptAddrDial`
(`swarm_dial.go:555`, our `gate.go`). For a peer that is not wanted, no non-relay address is ever dialled: a stranger
admitted for an invitation has no automatic punching path, libp2p's one-shot included. For a wanted peer reachable
only through a relay, a non-relay destination is allowed only if it is one of the ≤ 3 addresses the filter admitted
for the current round, or one of its announced public addresses for our Dial, and only while the peer holds a
node-wide slot: it costs one token of the peer's destination bucket and one of the grant's allowance (12). libp2p's
one-shot dials first, before any filter (`holepuncher.go:113`), and holds no slot then: the gater refuses that dial;
the one-shot goes on with its rounds, which take a slot in `FilterLocal` like any other. The answerer side (`svc.go:254`, `holePunchConnect` from the stream handler, outside libp2p's
per-peer lock and outside our queue) is admitted like any round: its `FilterLocal` takes a FIFO slot with a lease, its
dials pass the gater.

### 3.4 Who initiates (tie-break)

DCUtR's own rule, made explicit: **Punch only over an inbound circuit**; a side that only dialled does Dial (if p is
public) and otherwise waits for the other side. With two circuits (both dialled at once, likely during a restart),
both sides are eligible (iroh's rule). `NewStream` picks the circuit with the most streams, then the newest in the
local list (`swarm.isBetterConn`). With equal stream counts the two ends never both pick their own outbound circuit
(Alloy `SomeEligibleAccepted`), but they may **durably** both pick their own inbound one (Alloy `BothAccepted`): two
exchanges then run at once, and libp2p's per-peer lock does not stop them (an answerer takes no lock,
`holepuncher.go:68`). T_offset (+3 s) is not mutual exclusion: a call lasts up to three rounds. Whether two
overlapping punch phases cancel each other (both ends' dials meet in one swarm dial per peer) is **not known**: the
model's `CrossCancel` switch keeps both answers. With cancellation, both-eligible livelocks (mutant M24). **H-15 is
adopted** (B1), safe whether or not real libp2p cancels crossed punches: with two circuits nobody initiates, and the
circuit the higher PeerID dialled is closed once the session is off it (the spec §5.1 duplicate rule applied to
circuits); one circuit gives one initiator. `TestUpgradeCrossedExchanges` records the overlap figures for information
and checks that the close happens. « Lower PeerID only » would stall forever whenever the chosen circuit is the lower side's outbound one
(model mutant M6).

### 3.5 Hopeless pairs

There is no reliable NAT-type test here (AutoNAT v2 needs public servers the network lacks; reachability is forced
private, `relay.go`). « Hopeless » is therefore not a state: after Spent outcomes the slow ladder reaches its cap and
the upgrader keeps one call per 16 min ± 25 % while the peer is wanted and relayed. A silent change (a firewall rule
lifted, a NAT rebinding, `TestRelayThenDirect` with a long block) produces no event on either side; stopping would
lose liveness for ≈ 0.5 MB an hour saved (mutant M10). Events with new candidates (E7, E8) reset the ladder.

### 3.6 Threat model (full Byzantine member; strangers strictly bounded)

| Adversary | Bound |
|---|---|
| Stranger (not wanted: invitation window, non-member) | never a target of the scheduler; the gate gives no address either way and takes no ticket: **zero punch rounds**, our public addresses never disclosed in a CONNECT; the gater refuses every non-relay dial to it, libp2p's one-shot first direct dial included: **no direct path**; libp2p's automatic attempt costs one stream per inbound circuit |
| Member that refuses or stalls as answerer | one call in flight, cancelled at its lease (75 s); ≤ 12 calls in any hour |
| Member (or members) that keep asking for slots | FIFO: no peer takes a slot while another waits; leases end at 75 s whatever the holder does; ≤ 76 admissions an hour node-wide. Members are PeerIDs of the signed member list, so per-PeerID order is per-member order; strangers get no ticket |
| Member that initiates in a loop | rounds beyond its bucket get no address: the stream is reset before any punch packet |
| Member that reconnects, restarts or toggles `/libp2p/dcutr` | resets ladders and advances due times (≥ T_min), never refills a bucket |
| Member that points CONNECT or its identify addresses at a victim | every destination passes the gater: ≤ 3 per round, ≤ 174 in any hour: ≤ 5.9 MB/h, a bounded, member-only reflection; the dial loop already accepts the same order with book addresses |

## 4. Rules

| Id | Rule | Oracle (layer) | Mutant → caught by |
|---|---|---|---|
| H-1 | The upgrader acts only for a **wanted** peer whose session is on a circuit, with relays configured and RelayOnly off | pure (`directup`: no decision when unwanted, direct or disabled); Go sim: RelayOnly node makes no call | drop the wanted check → pure `H-1`; TLA M4 |
| H-2 | Punch starts only if `localReady`; `remoteReady` (E2, E3) is a trigger that advances the due time, not a guard: without it the fast-ladder poll still calls `DirectConnect` (a missing handler: one stream, no round, Unspent, one attempt token), since a lost push or identify is never resent | pure; Go sim `TestUpgradeAfterRestartBeforeHandler` (b restarts and dials within the 250 ms window: direct ≤ 5 s simulated after b's handler; **red today**) | no `localReady` guard → Go fake service that blocks until ready: call observed before ready; TLA M9; the old `remoteReady` guard → TLA M17 (L1, LostPush) |
| H-3 | Punch only with an inbound circuit; Dial only with outbound only and a public announced address; both inbound → H-15 | pure (role table); Alloy/TLA two-circuit config; Go sim `TestUpgradeTwoCircuits` | outbound side punches → TLA S4; lower-only → TLA L1 (M6) |
| H-4 | At most one of our calls in flight per peer; the next starts after the result; a new epoch, an unwant, Close or the slot's lease end cancels the call (D1, `DirectConnectContext`), which then returns at once | pure; Go sim with a fake puncher counting concurrency under `-race` | launch while in flight → TLA S1 (M13), pure |
| H-5 | At most 4 node-wide slots for **every** punch round: our calls, libp2p's automatic attempts and the answerer side all take one in the address filter (`FilterLocal`, first in both roles), granted FIFO, one ticket per peer and none for a peer that is not wanted, the filter blocking a waiting round ≤ 20 s; a round that gives up keeps its ticket's place (dropped only when the peer is no longer wanted or relayed, or after 20 min without a round), and a grant to it is held for its peer for the lease; a slot is a lease of 75 s, ended at its deadline whatever its holder does (early when the round or call ends) | pure (10 peers); Go fake puncher blocking, 10 relayed wanted peers: max 4; a peer asking in a loop never delays a waiting one by more than one lease; a holder that never finishes loses its slot at 75 s; `TestUpgradeAnswererBehindQueue` (R the only initiator, four members holding the slots in turn: R's rounds give up, come back on R's fast ladder, R goes direct) | no cap → TLA M11 (S9); grant in any order → TLA M18 (L2); no deadline → TLA M15 (L2); a stranger's ticket → TLA M28 (S17); a give-up loses the place → TLA M29 (L1, Answerer) |
| H-6 | A new circuit to p is a new epoch: ladders reset, due at max(now, lastStart + T_min), the call in flight cancelled (D1); a result applies only to its epoch | pure; Go sim `TestUpgradeAfterRestartDuringAttempt` (call in flight towards the old process: direct ≤ 3 s after the new circuit, not 5 s) | apply stale results / no epoch → TLA M2, Go sim |
| H-7 | Classes by observation: Success = direct connection after the call; Busy = `ErrHolePunchActive` (no token); Spent = ≥ 1 gated round or a network Dial; Unspent = otherwise; `ErrClosed` = stop | pure classification table (vectors) | Spent classed Unspent → pure `H-7`, Go hopeless-calm count; Busy charged → TLA M3 (S11) |
| H-8 | Ladders: fast 1 s·2^j cap 30 s; slow 60 s·2^k cap 16 min; ± 25 % jitter; Success clears them | pure on controlled time, jitter injected | cap removed or base halved → pure `H-8` |
| H-9 | E3, E4, E7, E8 advance the due time to max(now, lastStart + T_min); E7 and E8 (non-circuit address set changed) reset the slow ladder; no event refills a budget; relay-address-only changes are ignored | pure | event refills the bucket → pure `H-11`, TLA M7 |
| H-10 | No terminal hopeless state: at the cap, one call per 16 min ± 25 % while wanted and relayed | pure; Go sim `TestUpgradeAfterLongBlock` (direct blocked 20 simulated min, then open: direct ≤ 16 min · 1.25 + 30 s after) | stop after N failures → Go sim, TLA M10 |
| H-11 | Attempt budget: ≤ 4 + 8 = 12 non-Busy calls per peer in any hour (8 an hour sustained), kept across epochs, reconnections and un-want | pure; Go sim `TestUpgradeByzantineReconnect` (reconnect every 10 s for 1 h: calls ≤ 12) | reset on epoch → Go sim, TLA M16 (S10) |
| H-12 | Two choke points (3.3a). Address filter: no address to or from a peer that is not wanted; beyond 9 rounds (refill 1/100 s) none; else ≤ 3 public non-circuit addresses. Connection gater: no non-relay dial to a peer that is not wanted (no automatic punching path for a stranger); for a wanted peer reachable only through a relay, only an address the filter admitted for the round (or an announced public one for our Dial), each costing one token of a bucket of 30, refill 1/25 s (≤ 174 in any hour), DirectConnect's first direct dial included | pure (filter and gater functions, vectors); Go sim `TestUpgradeGateRefusesSpam` (raw libp2p member opens DCUtR in a loop: rounds ≤ 9 + t/100 s; stranger: 0); Go sim `TestUpgradeDialsOnlyAdmitted` (a member's identify lists 20 public addresses: ≤ 3 dialled per round, all counted; a stranger with a public address: none dialled) | gate only in the scheduler → TLA M8 (S5); the gate ignores wanted → TLA M14 (S6); first direct dial unbudgeted → TLA M19 (S12); gater dials a stranger → TLA M20 (S13), Go sim |
| H-13 | After Success, if the session is not direct within T_settle, nudge `maybeUpgrade` once and class Unspent | pure; Go sim with a stale direct connection to a dead process (#3420) | no settle check → Go sim stuck |
| H-14 | Direct stops the upgrader; a fallback to a circuit restarts it with budgets kept; unwant or Close: nothing new starts, the call is cancelled, Close never waits for it | pure; Go sim `TestUpgradeCloseDuringAttempt` (Close ≤ 1 s with a call in flight, goleak clean) | Close waits for the call → Go sim times out; call after unwant → TLA S2 |
| H-15 | Adopted (B1): with two circuits to p nobody initiates; the circuit the higher PeerID dialled is closed once the session is off it, so one circuit gives one initiator and the pair's two exchanges never overlap, whether or not crossed punches cancel | TLA `DirectUpgradeTwoCircuits` (`CrossCancel` on, L1, L3); Go sim `TestUpgradeCrossedExchanges`: oracle — with two circuits no DCUtR stream is opened by our upgrader and the higher PeerID's circuit is closed within 5 s of the session leaving it; measurement for information — overlap and both-fail rates at offsets 0 to 3 s, 20 seeds, written to the gate report | H-15 off (`CrossCancel` on) → TLA M24 (L1); no close → TLA M25 (L3), Go sim |
| H-16 | Node-wide budget, every origin (B3): every slot grant takes one token of a node bucket of 8, refill 1/120 s (≤ 38 in any hour), granted in FIFO order, and allows 12 non-relay destinations for the peer while held; no non-relay destination to a peer reachable only through a relay outside a grant | pure; Go sim `TestUpgradeNodeTraffic`: 10 Byzantine members (raw libp2p: DCUtR in a loop, reconnects, identify with 20 public addresses each) for 1 simulated hour; counts **destinations actually dialled** (every `InterceptAddrDial` admission and every simnet packet to a non-relay address): ≤ 456 node-wide, ≤ 174 per member, bytes ≤ 15.5 MB; and grants ≤ 8 + t/120 s | grant without a token → TLA M23 (S15); rounds sharing a grant beyond its allowance → TLA M26 (S16); a destination outside a grant (libp2p's first direct dial) → TLA M27 (S16) |
| H-17 | Energy: ladder-driven calls (fast and slow) run only while the app is active (input in its window within 5 min, machine not asleep); when idle they are suspended, and resume at once on activity; event-driven calls (E1-E8) run at once whatever the activity | pure (activity passed in); Go sim: idle 30 simulated min, no ladder call; activity: a call ≤ 1 s | ladders while idle → TLA M21 (S14); no resume → TLA M22 (L1) |

Go integration tests run in `sim_test.go`'s simnet inside `testing/synctest` (controlled time, real libp2p hole
punching, the NAT firewall that go-libp2p's own tests use); a restart helper keeps the node's address and changes its
port. `TestUpgradeCrossedExchanges` is a measurement, not an assertion first: both ends hold an inbound circuit and
initiate at offsets 0, 0.5, 1, 2 and 3 s, 20 seeds each; it records the overlap of the punch phases and whether both
fail, for information; its oracle is H-15's close (rule H-15). The netsim is for environment assumptions only (section 8). Every test names its rule (`H-n`) in its failure
text, so Go mutants use `expect`.

## 5. Spec text (ten points of `docs/protocol/README.md`)

New §8.6 « Montée en direct » (normative, no new message or identifier: DCUtR is `/libp2p/dcutr`, registry §20.6
gains a line): roles (3.4), the attempt rate a conformant node MUST NOT exceed (8 calls per peer per hour, one in
flight, 12 in any hour), the rounds a node MAY refuse (beyond 45 in any hour, or from a non-member), the destinations
it MAY refuse to dial (beyond 3 per round or 174 in any hour, or any for a non-member), the timers (3.3);
errors and floods (3.6); §17 and §18 lines (strangers get no address); vectors `protocol/vectors/directup/`; model
section 6; §16 bounds table rows.

## 6. Formal model `DirectUpgrade.tla` (for `formal-modeler`)

Written and checked (`docs/protocol/tla/DirectUpgrade.tla`, README « DirectUpgrade », Alloy
`DirectUpgradeRoles.als`). Two nodes; L in full: the policy, its cancellable calls (D1) with libp2p's first direct
dial, libp2p's one-shot, the answerer side of R's exchanges; every round passes the address filter (wanted, round
bucket, FIFO admission to a node-wide slot with a lease) and every non-relay destination the gater (wanted,
destination bucket, a grant held with an allowance); other peers ask for slots, hold them as long as their lease
allows and dial within their grant's allowance (no fairness on them); waiting rounds give up and their tickets stay;
the app may go idle (D4). R honest (its upgrader under the same rule, paced by time, and its one-shot; its exchanges
are concurrent with L's, D5) or Byzantine. Picks: any pair of circuits, durable; only « both outbound » is left by a
fair step, since it needs unequal stream counts (Alloy). `CrossCancel`: overlapping punch phases may both fail.

Invariants: **S1** one call per peer (H-4); **S2** no live call for a peer not wanted; **S3** none while direct;
**S4** Punch only with an inbound circuit, Dial only with outbound only; **S5** rounds within the round bucket, every
origin; **S6** zero rounds with a peer not wanted; **S7** no call before `localReady`; **S8** a stale result changes
nothing; **S9** ≤ K slots node-wide, every origin; **S10** non-Busy calls within the attempt bucket; **S11** Busy never
charged; **S12** non-relay destinations within the destination bucket, every origin; **S13** no non-relay dial to a
peer not wanted; **S14** no ladder-driven call while idle; **S15** node-wide admissions within their bucket; **S16**
every non-relay destination, every origin, drawn from a held grant's allowance (with S15: node destinations ≤
allowance × admissions); **S17** a peer not wanted takes no ticket. S1-S17 are invariants of every configuration
(B2). Budgets are checked against reference buckets. **L1** `wanted ∧ relayed ~> direct ∨ ¬wanted ∨ no circuit`; **L2** waiting for a
slot or Busy does not repeat forever, other peers asking without end and never releasing (leases end them); **L3**
two circuits do not last (H-15).

Configs: `Honest`, `Silent` (EnvFlip only, one idle period), `TwoCircuits` (durable asymmetric picks, both PeerID
orders, `CrossCancel` on, H-15), `TwoPeers` (one slot, another peer asking for ever and never releasing), `Answerer`
(R the only initiator, its rounds give up waiting and come back, one slot shared with a peer asking for ever),
`LostPush`, `Byzantine` (invariants). One-peer configs use K = 2.

Mutants: M1 no retry → L1 · M2 stale results applied → S8, and L1 · M3 Busy takes a token → S11 · M4 no wanted check
→ S2 · M5 Punch from the outbound side → S4 · M6 lower PeerID only → L1 (TwoCircuits) · M7 an epoch refills the
buckets → S5 · M8 round budget in the scheduler only → S5 · M9 call before `localReady` → S7 · M10 stop after the
cap → L1 · M11 no node-wide cap → S9 · M12 call while direct → S3 · M13 call while a cancelled call is in flight →
S1 · M14 the gate ignores « wanted » → S6 · M15 no lease deadline → L2 · M16 an epoch refills the attempt bucket →
S10 · M17 the old `remoteReady` guard → L1 (LostPush) · M18 slots granted in any order → L2 · M19 DirectConnect's
first direct dial unbudgeted → S12 · M20 the gater dials a stranger → S13 · M21 ladders call while idle → S14 · M22 no
resume after idle → L1 · M23 a grant without a node token → S15 · M24 H-15 off (`CrossCancel` on) → L1 · M25 the
duplicate never closed → L3 · M26 rounds sharing a grant beyond its allowance → S16 · M27 a destination outside a
grant → S16 · M28 a stranger takes a ticket → S17 · M29 a give-up loses the ticket's place → L1 (Answerer). Witnesses: direct reached, a stale result, Busy, a slow step, the gate refusing a flood, waiting
behind another peer, two punch phases overlapping.

## 7. Scenario 5, C3

Decision: **count `downsTotal` from a baseline**, taken when C1 has resolved (`await c1`, which already gates the
phases; by then the first process is gone, ≤ 30 s QUIC idle), and judge `notTornDown(baseline, end)` as scenarios 6
and 7 do. Add **C3a** (D6): a link-down is excused only inside the **identified restart window**, from the joiner's
« restarting » log line (`EvtRestart`, PC1b) to its new process's first PeerUp at the inviter, and at most one there;
every other link-down before the baseline fails C3a, as C3 did before (`netsim.spec.ts:751`), restart or not. The
netsim mutant « an extra link-down before the baseline » (a relay drop injected outside the window, kind `netsim` in
`scripts/netsim/mutants.toml`) must make C3a red. Reason: C3 states « stay up under bad Wi-Fi », which is the
phases, not the pairing; PC1b's restart is a real end of a process at the transport level, and `downsTotal` is a
transport fact. With overlapping connections the restart may count 0 or 1 (`outLost` reopens without a PeerDown),
hence `≤ 1`.

Rejected: a « going away, back soon » notice. The wire today is `/inkwell/sync/1.0.0`, with no control channel;
GOAWAY exists only in the v2 spec (§20.3, vectors) and is not implemented. A new code would need spec, registry,
vectors and both ends; it would let a Byzantine peer hide real outages from the figures; it couples application intent
into a transport counter. Its only gain is a cosmetic count. Revisit with v2 if the interface ever needs to hide the
departure (Q6).

## 8. Alternatives

| Option | Fixes (a) | Fixes (b) | Silent change | Cost / risk | Verdict |
|---|---|---|---|---|---|
| A. Event-driven retry with slow safety ladder in `internal/transport/p2p`, gate in `AddrFilter`, service captured by an Option | yes, at the push (≈ ms) | yes, at the new epoch | yes (16 min cap) | ~400 lines + pure package; public APIs only | **recommended** |
| B. Retry inside the fork (`holepuncher.go`) | yes | yes | if written so | widens the fork; rebased on each release (v0.52.0 already out); upstream unlikely to take it (#3024, #3420); rejected by the integrator | no; only the cancellable `DirectConnectContext` (D1) goes into the fork, with its upstream PR |
| B'. Register `/libp2p/dcutr` at once in the fork | no: the answerer refuses without public addresses (`svc.go:181`) | no | no | fork | no |
| C. Events only, no timer | yes | yes | **no** | lost push or silent change = relayed forever | no |
| D. Make the bench wait after restart | hides it | hides it | no | masks a real defect (restarts and crashes of real users) | no (integrator) |
| E. Close the circuit and redial to re-trigger libp2p's one-shot | yes | yes | no | PeerDown churn, session resync, counts downs | no |
| F. Own `holepunch.Service` instead of the host's | yes | yes | yes | loses the one-observer observed addresses (private API): fewer successes in a small network | no |
| G. Own punching (QUIC multipath / QNT like iroh) | — | — | — | replaces DCUtR, not available in go-libp2p | out of scope |

## 9. Gate sketch (`gates/HP1.toml`, to freeze after Codex review)

`tla` (7 configs green with S1-S17 and L1-L3, 29 mutants and 7 witnesses red with the named property) · `go-pure` (`protocol/directup` tests and vectors) ·
`go-sim` (the named H-tests, red first on `main` for H-2, H-6, H-10, H-11, H-12) · `mutation` (gremlins ≥ 95 % on
`protocol/directup`) · `go-mutants` (one text mutant per H-rule in `scripts/netsim/mutants.toml`, kind `go` with
`expect`) · `netsim` (scenarios 5 and 8, both variants, seeds 20772 and two more, on the exact tree: no regression;
plus a new scenario 9 if Q4 is accepted) · `netsim-mutant` (HP1 disabled → scenario 9 C1 red; an extra
link-down before the baseline → scenario 5 C3a red) · `go-measure` (`TestUpgradeCrossedExchanges`: H-15's close
asserted, overlap figures recorded; `TestUpgradeNodeTraffic`: destinations and bytes dialled, not grants) · `regression`
(packages touched and their consumers) · `docs`.

## 10. PC1b's return to `main`

After HP1 is merged: `git cherry-pick 73d0f4b` onto `main` in a fresh worktree; scenario 5's C3/C3a change lands with
HP1 (it holds on `main` too: no restart, baseline 0). PC1b's gate gains two criteria: `netsim-5` and `netsim-8`
(`e2e:netsim:impaired`, both scenario 8 variants, seeds 20772 + 2, `heavy.sh --exclusive`, machine idle), both green
on the exact candidate; then its full gate once, and its `SIMPLIFY.md` notes the rebase. The rule « any gate that
touches the arrival of a device includes the netsim scenarios » (CLAUDE.md) is thereby satisfied.

## 11. Risks

- **R1** `ErrHolePunchActive` and responder rounds can overlap our call: the « rounds during the call » class may
  attribute a peer-initiated round to our call (two-circuit case only). Effect: a slow step instead of a fast one;
  budgets stay exact. Accepted.
- **R2** `DirectConnect` blocks forever if the service closes between registering its handler and closing
  `hasPublicAddrsChan` (`svc.go:145-153`): closed by D1 (the wait selects on the call's context).
- **R3** Error-free success with a stale direct connection (#3420): handled by H-13.
- **R4** The captured `*Service` is libp2p's; `DirectConnect` is commented « only exposed for testing purposes »:
  a future release may remove it. `DirectConnectContext` is ours in the fork, proposed upstream with the same PR;
  pinned version, `go-sim` tests fail loudly.
- **R6** H-5's hold assumes an honest peer comes back within 75 s of giving up: true on its fast ladder (≤ 37.5 s),
  not once its attempt bucket is empty (next token ≤ 7.5 min); it then re-waits, its place kept for 20 min.
- **R5** Netsim timing under load: the measure needs the machine alone (CLAUDE.md « Pièges connus »).

## 12. Open questions for the integrator (recommended answer first)

1. **Pure core in `protocol/directup`** (public, sans I/O, vectors, gremlins) **or a pure file in
   `internal/transport/p2p`?** Recommended: `protocol/directup`, as ADR 0036 did for the link probe: the rates are
   peer-observable bounds stated in the spec.
2. **R2 and R4: file the upstream issue and PR now, keep the fork as it is?** Recommended: yes; do not add the line to
   the fork unless a hang is ever observed.
3. **Slow-ladder cap 16 min** (≈ 0.5 MB/h for a hopeless pair) **or 8 min**? Recommended: 16 min; events cover network
   changes, the cap only serves silent ones.
4. **Add netsim scenario 9 « a device restarts while linked »** (two NATed devices and a relay; bob's process
   restarts; C1 direct ≤ 30 s, C2 converge, C3a), so that HP1's gate shows the defect without PC1b, and R1's
   acceptance « tuer et relancer » becomes executable? Recommended: yes (≈ 3 min of netsim).
5. **Alloy model of the role rule** in addition to TLA+? Recommended: yes, small and exhaustive.
6. **« Going away, back soon » notice**: recommended no (section 7); reconsider with the v2 session.
7. **Language of the new spec section §8.6**: recommended French, like §8.5 (the doc stays one language), with the
   plan and any ADR in English.
8. **An ADR** (« relayed links retry the direct upgrade, budgets at the hole-punch address filter »)? Recommended:
   yes, short, pointing to this plan, since it adds peer-observable bounds.

## 13. Sources

- DCUtR spec: https://github.com/libp2p/specs/blob/master/relay/DCUtR.md
- go-libp2p v0.51.0 (fork `oioio-space/go-libp2p@cae49c803966`, module cache), files cited above; v0.52.0 release
  (2026-11-14): https://github.com/libp2p/go-libp2p/releases/tag/v0.52.0 ; holepunch history:
  https://github.com/libp2p/go-libp2p/commits/master/p2p/protocol/holepunch
- go-libp2p #3024: https://github.com/libp2p/go-libp2p/issues/3024 ; #3420:
  https://github.com/libp2p/go-libp2p/issues/3420 ; fork's #3575/#3576:
  https://github.com/libp2p/go-libp2p/issues/3575
- iroh 1.4.1, `iroh/src/socket/remote_map/remote_state.rs`: https://github.com/n0-computer/iroh ; iroh on QUIC
  multipath: https://iroh.computer/blog/iroh-on-QUIC-multipath
- Tailscale `wgengine/magicsock/{magicsock,endpoint}.go` (commit `ddb3fd6b43`, 2026-11-12):
  https://github.com/tailscale/tailscale/tree/main/wgengine/magicsock
- RFC 8445 (ICE), §2.4 restarts: https://www.rfc-editor.org/rfc/rfc8445 ; RFC 7675 (consent freshness):
  https://www.rfc-editor.org/rfc/rfc7675
- Trautwein et al., « Challenging Tribal Knowledge »: https://arxiv.org/abs/2510.27500 ; IMC'26 version:
  https://arxiv.org/abs/2604.12484 ; ProbeLab: https://probelab.io/blog/can-libp2p-punch-through-nats/

## Integrator decisions (2026-11-16)

The eight open questions are settled as recommended: (1) the pure policy core goes in `protocol/directup`, no I/O,
like `protocol/linkprobe`; (2) the shutdown-window `DirectConnect` leak is filed upstream (issue and PR) and the fork
is left alone; the residual is one goroutine, Close still returns; (3) the slow-retry cap is 16 min; (4) netsim
scenario 9, « a device restarts while linked », is added so that HP1's gate shows the defect without PC1b; (5) a
small Alloy model of who initiates sits beside the TLA+ model; (6) no « going away, back soon » notice; (7) the new
spec section §8.6 is written in French, like §8.5; (8) a short ADR records the bounds other peers can observe. → HP1

Decisions on the formal model's findings (2026-11-16, step 1):

1. **Lost push** (the model showed scenario 8 again: R dials before L's handler exists, identify runs before R's
   handler exists, R's single push is lost, L is the only initiator and stays NotReady for ever): in NotReady with
   `localReady` and a role, the fast-ladder poll really calls `DirectConnect`; a missing handler costs one stream, no
   gate round, class Unspent, one attempt token. `remoteReady` (H-2) becomes a trigger, not a guard. Config
   `DirectUpgradeLostPush` checks L1; mutant M17 restores the old guard and fails L1.
2. **M3** (Busy takes a token) is caught by S11 « Busy is never charged », not by L1.
3. **K ≥ 2** in the one-peer configurations, so that the node-wide cap does not hide H-4's mutant (M13).
4. **FIFO grant of the node-wide slots** (part of H-5): a peer waits for a slot in a FIFO queue; no peer takes a
   slot while an earlier one waits, so a Byzantine member that keeps calling cannot starve the others. L2 holds with
   the other peers calling without end (`DirectUpgradeTwoPeers`); mutant M18 grants in any order and fails L2.

### Codex debate (2026-11-16)

Codex reviewed the design (six decisions D1-D6); the integrator accepted his changes. Each was checked in the pinned
fork before being applied.

| Decision | Objection | Disposition |
|---|---|---|
| D1 policy in the product, fork untouched | H-6's « ≤ 3 s » cannot hold while the next call waits for the old one; `DirectConnect` takes no context and can block at close (`svc.go:282`) | accepted: the only fork change is `DirectConnectContext(ctx, p)`, with its upstream PR (3.1); a new epoch, an unwant, Close or a lease end cancels the call; H-4, H-6, H-14 reconciled; R2 closed |
| D2 the probe costs one stream; the address filter bounds what is punched; 8 calls and 36 rounds an hour | `DirectConnect` dials every peerstore address first (`holepuncher.go:113`); rounds dial through `Connect`, which dials beyond the admitted addresses (`util.go:51`, `dial.go:312`); strangers keep an automatic path; with bursts the buckets allow 4 + 8 calls and 9 + 36 rounds | accepted, confirmed in the code (`basic_host.go:547-558`): the connection gater is the second choke point (3.3a), every non-relay destination from every origin, none for a peer not wanted; figures restated as any-hour maxima: 12 calls, 45 rounds, 174 destinations, ≤ 5.9 MB/h per Byzantine member (H-11, H-12) |
| D3 FIFO slots for our calls | the four slots cover only our calls; the answerer side dials outside the queue (`svc.go:254`); the model assumed every holder finishes | accepted: every round takes its slot in `FilterLocal` (first in both roles, `holepuncher.go:224`, `svc.go:177`), FIFO, one ticket per peer, a 75 s lease grounded on libp2p's deadlines (`StreamTimeout` 60 s, `directDialTimeout` 10 s), node-wide admissions (H-5, H-16); the model gives other holders no fairness, leases end them (M15) |
| D4 slow ladder for ever, 16 min cap | no node-wide energy budget: the cost multiplies by the peers and wakes the machine; Tailscale stops its heartbeat when idle | accepted: H-16 (node bucket: ≤ 76 admissions in any hour, ≤ 7.3 MB/h per node) and H-17 (ladders only while active, events at once, resume at once); confirmed in Tailscale's `endpoint.go` (`heartbeat` ends « for idle session », `noteTxActivityExtTriggerLocked` restarts it) |
| D5 both eligible with two circuits, T_offset | a remote initiation takes no lock (`holepuncher.go:68`); the model's R call was atomic and `Align` fair, while Alloy allows two durably different picks with equal counts | accepted: R's exchanges now run concurrently with L's, no `Align`; only « both outbound » is left by a fair step (it needs unequal counts). Finding: if overlapping punches cancel each other, both-eligible can livelock (residual `DirectUpgradeResidualCrossCancel`); proposed H-15 (one circuit, one initiator) removes it (`DirectUpgradeTwoCircuitsResolve`); `TestUpgradeCrossedExchanges` measures the overlap on real libp2p and decides H-15 → HP1 |
| D6 C3a: inviter `downsTotal ≤ 1` at the baseline | it forgives a real outage before the baseline, with or without a restart, where C3 required zero | accepted: the excuse holds only inside the identified restart window, zero elsewhere; netsim mutant « an extra link-down before the baseline » (section 7) |

One point not taken: « no per-person fairness follows from per-PeerID fairness ». For HP1 the unit is the member, and a
member is one PeerID of the signed member list (CLAUDE.md invariants; ADR 0017): a Byzantine member cannot mint PeerIDs
that the gate wants, and a stranger takes no ticket at all (H-1, H-12). Per-PeerID FIFO is therefore per-member FIFO
here; the node-wide admission bucket (H-16) bounds what a coalition of members can cost. His points E1-E3 concern
other blocks (history attribution, the join's root, PC1b's order) and are not part of HP1.

Second review (freeze, 2026-11-16): Codex rejected the gate on five points of the model and the plan; the integrator
decided B1, the others were applied as asked.

| Point | Objection | Disposition |
|---|---|---|
| B1 H-15 « proposed » | a measurement alone proves no `CrossCancel = FALSE`; the behaviour and its oracle must be fixed before the freeze | H-15 adopted unconditionally (integrator); `TwoCircuits` checks L1 and the new L3 with `CrossCancel` on; the old residual is mutant M24 (H-15 off), M25 (no close) fails L3; `TestUpgradeCrossedExchanges` asserts the close and records the overlap figures for information → HP1 |
| B2 S12-S15 only through mutants | killing a mutant does not prove the sound model | S1-S17 are invariants of all seven configurations |
| B3 node bound | libp2p's first direct dial spends destinations without admission; several rounds can share one lease; « 76 × 3 ⇒ 7.3 MB/h » does not follow | every non-relay destination needs a held grant and draws on its allowance (12); node-wide ≤ 38 grants × 12 = 456 destinations, ≤ 15.5 MB/h; per Byzantine member ≤ 174, ≤ 5.9 MB/h; S16 over all origins, M26 and M27; `TestUpgradeNodeTraffic` counts destinations dialled and bytes, not grants |
| B4 strangers' tickets | the one-shot and the answerer reached the queue without the wanted check | a peer not wanted takes no ticket, its rounds are refused at once (S17, M28) |
| B6 FIFO with a 20 s wait | tickets did not expire in the model; starvation not excluded | a round that gives up keeps its ticket's place; a grant to it is held for its peer for the lease; `Answerer` checks L1 and L2 with give-ups and returns; M29 (the place lost) fails L1 |
