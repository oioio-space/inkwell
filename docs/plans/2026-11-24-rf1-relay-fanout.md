# RF1: relay fan-out and per-origin admission (complete tier: trust/admission, peer resources)

Step 1 of block RF1 (`PROGRESS.md`, « Cap à long terme »). Spec §16.1 promises 64 members; libp2p's connection
limiter refuses the 8th member behind one relay. Decision taken 2026-11-16 (`docs/autonomous-decisions.md`, debated
with Codex), re-checked against go-libp2p v0.52.0 and master (2026-11-24). Revision 2 applies the round-1 review,
`gates/RF1/SPEC-REVIEW.md` (DeepSeek V4 Pro, « accepter avec corrections »), and the coordinator's decisions (§8).

## 1. Defect, with evidence

- **Per-IP limit.** rcmgr's `connLimiter` allows 8 concurrent connections per IPv4 /32 (8 per /56 and 64 per /48 for
  IPv6). The node leaves it unconfigured. Pinned fork `oioio-space/go-libp2p@v0.51.1-0.20261122194938-8ebee42bc17a`:
  `p2p/host/resource-manager/conn_limiter.go:33-48`. It is checked at `rcmgr.go:390-394`, before any peer is known.
- **A circuit carries the relay's IP.** `OpenConnection` takes the endpoint's first IP (`rcmgr.go:371-383`,
  `go-multiaddr@v0.16.1/net/convert.go:138-152`). Outbound circuits pass `/ip4/<relay>/…/p2p/<relay>/p2p-circuit/…`
  (`circuitv2/client/transport.go:64`). Inbound circuits pass `conn.RemoteMultiaddr()` (`net/upgrader/listener.go:193`),
  which is the relay connection's address plus `/p2p/<relay>/p2p-circuit` (`circuitv2/client/conn.go:79-89`). So the
  relay connection and every circuit through it share one count of 8, in both directions, and one rate bucket:
  0.2/s with a burst of 16 (`conn_rate_limiter.go:10-15`, `rcmgr.go:386`).
- **Upstream.** go-libp2p#2805 (May 2024) dropped `OpenConnectionNoIP`, whose comment wanted circuits limited « by
  everything but the IP ». Since then the relay's IP comes first. The code is the same on master and in v0.52.0, and
  no issue reports it (#3431 and #3453 only document the limiter).
- **Reproduction, 2026-11-24, `df31531`.** `TestUpgradeSlotsFIFO` (10 relayed members) was run through a
  `go test -overlay` that removes only `LiftRelayIPLimit`. It fails with « H-5 setup: 7 of 10 peers up after a
  minute ». It passes with the lift (`gates/RF1/WITNESSES.md`).
- **Elsewhere.** The relay keeps libp2p's defaults: 8 connections and 8 reservations per IP, 16 circuits per peer
  (`relay.go:39-40`, `circuitv2/relay/resources.go:46-59`), and 64 relay-service streams per peer
  (`go-libp2p/limits.go:84-88`). The node allows 128 connections in all and 64 per direction (`node.go:690-696`),
  while §16.1 and `protocol/limits.go` say 128 inbound and 128 outbound. The node keeps its own copy (`maxPeers`) of
  `protocol.MaxConnectedMembers`.

## 2. Decision and sources

Kept: fix (a) of 2026-11-16. The rejected options stay rejected:
- a /32 exception per relay: fragile, and it exempts every stranger the relay carries;
- a global raise of the per-IP limit: one stranger IP would take the inbound slots.

go-libp2p's `NetworkPrefixLimit` and its allowlist are keyed by IP, so neither can express « this relay's members ».
rust-libp2p bounds pending and established connections separately (`libp2p-connection-limits` 0.7,
`with_max_pending_incoming`); that is the shape kept here.

Two traps were found while checking:

1. **An endpoint with no IP is not unlimited.** `SubnetLimiter.Allow` puts every zero `netip.Addr` in one shared
   /56 bucket (`x/rate/limiter.go:176-216`; `netip.Addr.Prefix` returns a zero prefix with no error). So libp2p's
   rate and per-IP limiters must be switched off explicitly where admission owns them (§8):
   `WithNetworkPrefixLimit` with 0.0.0.0/0 and ::/0 unlimited, and an empty `rate.Limiter`.
2. **`golang.org/x/time/rate` cannot give a token back after it was used.** `Reservation.CancelAt` returns without
   restoring anything once the reservation's time to act has passed (`x/time@v0.16.0/rate/rate.go:177`). So the
   ledger keeps its own bucket, as debts.

## 3. Rules

The node wraps its resource manager in an **admission layer**. Its accounting core is the pure package
`protocol/admit` (§8). The libp2p glue is in `internal/transport/p2p`. The bound constants live in
`protocol/limits.go`, pinned to §16 by `protocol/testdata/limits.json` (`TestLimitsMatchSpecification`).

- **RF-1 (origin).**
  - Before its handshake, an inbound connection is charged to its origin.
  - For a circuit, the origin is its relay, named by the relay's authenticated PeerID (the `/p2p/<id>` before
    `/p2p-circuit`), never by the relay's IP.
  - Otherwise, the origin is the IPv4 /32 or the IPv6 /56 (/48: 64).
  - An outbound connection is charged to no origin.
- **RF-2 (members leave the counts).** At `SetPeer`, a connection of a member (the membership list, not the
  invitation window) or of a configured relay leaves every pending count, and its token is returned if still owed.
  Inbound connections reach `SetPeer` after their TLS or Noise handshake; outbound ones with the dialled peer, which
  the handshake confirms.
- **RF-3 (pending quota).** Unverified connections are those before the handshake, or identified as non-members and
  not yet closed by the gate. Limits: 16 per relay origin, 8 per IP origin, 48 in all. Above them the opening is
  refused.
- **RF-4 (deadline).** A pending connection holds its place for 15 s at most: libp2p's accept timeout, which is
  above QUIC's 2 × 5 s.
- **RF-5 (rate).**
  - Each origin has a token bucket: 16 tokens, refilled 1 per 5 s. An unverified admission takes a token.
  - A member's or a configured relay's handshake removes its own debt if a period has not paid it yet, and nothing
    else.
  - It binds only connections that stay unverified (§4).
  - The burst must be ≥ the per-origin quota, so a wave of members never waits for a token.
- **RF-6 (capacity).**
  - Member connections: 128 in either direction, of which 2 are reserved for each of the 63 other members. A
    member's third or fourth connection (PerCap 4, §8) comes only from the rest, never from another member's unused
    share.
  - Roster growth (code review, round 1): a member admitted to the roster while the rest is taken makes the node
    close, just enough for every share to fit again, connections held above their own share (from the member most
    above its own, outside the roster everything is above, newest first): `admit.Ledger.SetRoster`,
    `Node.shedSurplus`; model `AdmitLate`, runs `RFRGrowth` and mutant `RFRNoShed` (MFR1).
  - Configured relays: 2 each, 4 relays at most.
  - Plus the 48 pending. The libp2p system and per-direction limits are set to the sum, 184, so they never bind
    first.
  - `VerifySourceAddress` keeps libp2p's rule (QUIC address validation beyond a burst of 4 per IP, refilled 1 per
    10 s, `conn_limiter.go:290-340`) on the layer's own limiter.
- **RF-7 (relay).**
  - 128 circuits per peer.
  - 64 reservations and 64 connections per IP.
  - Relay-service and hop/stop streams: 130 per peer and per direction, and 8 192 relay-service streams in all (two
    for each of 64 × 63 circuits).
  - With `Allow`, listed peers leave the relay's per-IP counts at their handshake (RF-2), and an unlisted peer is
    refused at its handshake (code review, round 1): the circuit-v2 ACL is checked per HOP message only, so its
    connection would otherwise hold one of its address's places while it stays open
    (`TestRelayUnlistedHoldsNoPlace`).

New and changed §16.1 rows (French, normative unless marked « impl. »):

| Objet | Borne | Part réservée | À la borne | Reprise |
|---|---|---|---|---|
| connexions | 2 par pair en régime, 4 au plus pendant un passage (circuit, direct, connexion supplantée en preuve P-15, appel croisé) ; membres : 128 en tout, dans un sens ou l'autre (impl.), dont 2 réservées à chacun des 63 autres membres, une troisième ou une quatrième seulement sur le reste ; un membre ajouté à la liste quand le reste est pris : le nœud ferme, juste assez pour que chaque part tienne, des connexions au-delà de la part propre (celles du membre le plus au-dessus de la sienne, la plus récente d'abord) ; relais configurés : 2 chacun, 4 relais au plus ; plus les connexions non vérifiées ; limites système et par sens de libp2p : 184 | 2 par membre | refusée | à la fermeture d'une autre |
| connexions non vérifiées (avant la poignée de main, ou non-membre identifié) | 16 par relais d'un circuit, désigné par son PeerID authentifié, jamais par son adresse IP ; 8 par adresse IPv4 (/32) ou préfixe IPv6 /56 (64 par /48) ; 48 en tout ; 15 s au plus ; admissions : seau de 16 jetons par origine, 1 de plus par 5 s ; le jeton d'un membre ou d'un relais configuré lui est rendu à sa poignée de main s'il est encore dû | par origine | refusée à l'ouverture | à la fermeture d'une connexion non vérifiée ; jeton suivant |
| connexions par IP (membres) | aucune borne par adresse ni par relais pour une connexion authentifiée d'un membre ou d'un relais configuré, ni pour une connexion sortante (8 membres derrière une même box, 63 derrière un même relais) | — | — | — |
| circuits par relais (relais inkwell) | 128 circuits par pair (64 × 2) ; réservations 256, 64 par IP ; connexions 64 par IP ; flux du service par pair et par sens 130, 8 192 en tout ; avec une liste Allow, un pair absent de la liste est refusé à sa poignée de main (il ne peut ni réserver ni relier) | par pair | `RESOURCE_LIMIT_EXCEEDED` | à la fermeture d'un circuit |

**Memory (PU1c).** The fork reserves the 1 MiB initial window of every QUIC connection in the resource manager
when the connection opens, pre-handshake ones included (`quicreuse/connmgr.go:461-471` inbound, `quic/transport.go:360-364` outbound). A circuit reserves none.
The worst case is 128 direct member connections, 8 relay connections and 48 pending QUIC connections: 184 MiB of the
256 MiB (`node.go:102`), against 128 MiB of 256 MiB before RF1. That leaves 72 MiB for window growth, instead of 128.
A refused increase keeps the window smaller and costs throughput, never the connection (PU1c P-13). The receive budget
is a separate 256 MiB (`maxBuffered`, `node.go:135`) and does not change. The QUIC-window row becomes
« 184 connexions : 184 Mio des 256 Mio ».

## 4. Threat model and numbers

The adversary has any number of PeerIDs, any number of circuits through a public relay the node reserves on, and
a few IPs. It may complete handshakes or stall them. It cannot sign as a member, but members may be Byzantine.

**G1. Members keep what they need.** Strangers and Byzantine members cannot take:
- the member shares;
- the node's outbound dials, which are never charged at the node;
- members arriving through origins that are not flooded.

**G2. One origin's share is bounded.** A relay origin holds at most 16 of the 48 places and an IP origin 8, so
filling the quota takes 3 relays or 6 IPs. Under the rate, an origin keeps only 0.2 × 15 = 3 places for as long
as the flood lasts.

**G3 (review items 2 and 3, computed): join and recovery times.** The scripts are `docs/plans/2026-11-24-rf1/rf1_wave.py` and
`docs/plans/2026-11-24-rf1/rf1_recover.py`, discrete events with 5 seeds each. They use the real dial loop (`dial.go`: 1 s backoff,
doubled up to 60 s, jitter −25 % to +25 %) and 64 members behind one relay; `h` is a handshake's duration.

- **Wave, no flood.** Every member dials every other one, for example after the relay restarts. The rate never binds
  here, which is what the reviewer missed. RF-5 rule text: « A member's or a configured relay's handshake removes its
  own debt », so at most P tokens are out at once, and the burst (16) ≥ P. The per-origin quota binds instead:

  | per-relay P | h = 0.5 s | 1 s | 2 s | 4 s |
  |---|---|---|---|---|
  | 8 (round 1) | 8.6 s | 17.6 s | 36.9 s | 74.0 s |
  | **16 (kept)** | 4.2 s | 4.7 s | 10.4 s | 21.7 s |
  | 32 | 1.8 s | 4.4 s | 5.6 s | 12.3 s |

  Worst-case join times. A star (63 members and one node) takes h alone, because the node's dials are not charged at
  the node. With 8, a bad relay (h = 4 s, round trip 2.5 s, §8.5.7) takes over a minute, against the 60 s of H-5's
  setup. So the relay quota is raised to 16, and 48 in all keeps room for 3 flooded relays. IP origins stay at 8, as
  in libp2p.
- **Recovery after a flood of every member (§4 last paragraph).** At the flood's end, every bucket is empty, every place is held for up to
  15 s more, and every dial loop sits at the 60 s cap. All 2 016 pairs are back within:

  | refill | h = 1 s | 2 s | 4 s |
  |---|---|---|---|
  | **1 per 5 s (kept)** | 75.9 s | 76.9 s | 124.3 s |
  | 1 per 1 s | 75.9 s | 76.9 s | 78.9 s |

  Worst cases. The dial loop's backoff cap (45-75 s) dominates. The rate costs one more backoff cycle only when a
  handshake takes 4 s. §16.1 makes no promise about recovering from such a flood. Refilling 1 per 1 s would let
  one origin keep 15 places, so the rate would no longer bound a flood. The numbers stay.

**Residual → RLY1 (accepted for RF1, owned by RLY1 after the merge).** An adversary that floods, through one public relay, the
inbound quota of every member denies relayed links through that relay for as long as the flood lasts. Recovery
takes the times above. Direct paths and other relays are unaffected, and `Allow` closes this hole at the relay. The
model's run `RF1WitnessResidual` shows the denial. Second part (code review, round 1): on an open relay (no
`Allow`), every client is unverified while connected, so a stranger behind the members' address holds places of
its 64 for as long as its connections live: measured in the simulated network (`TestRelayUnlistedHoldsNoPlace`
with the gate off, one dial a second for six minutes, connections kept alive), **9 places held at the peak**, each
connection ending after **about 45 s**, bounded by the bucket (one admission per 5 s) times that lifetime. Kept for
RF1; → RLY1.

## 5. Model (TLA+, `docs/protocol/tla/RelayFanout.tla`)

Why TLA+ rather than Alloy: the properties concern concurrent openings, handshakes, deadlines and refills, plus
liveness with fair members and an adversary that is not fair. Alloy fits static structure.

**The token bucket is modelled, not abstracted (review item 1).** The bucket is kept as debts: unpaid stranger
admissions, the tokens of pending members, and member tokens never given back. One period (`Refill`, weakly fair)
pays any one debt, so every payment order of a real bucket is explored. A member's handshake removes its own unpaid
debt. Ghost `g` is the strangers' admissions minus the periods, measured from its lowest point. `RateBound`
(g ≤ B) is thus exactly « at most B + k stranger admissions in any k periods ». Round 1's fixed window is now mutant
MF13, and `RateBound` catches it: a refill of the whole bucket lets strangers take 2B across a boundary. So the
round-1 abstraction did not over-approximate the rate.

**Configurations.**
- Flood: three honest members behind one origin, a Byzantine member, a stranger flooding two origins forever, the
  node dialling.
- Inbound: inbound only, with a stranger that stops.
- Mixed: members behind a relay and behind a home router.

Model constants are scaled down: P = 2, Q = 3 or 4, B = 2, MemberCap 6 or 9, PerCap 4.

**Properties.**
- RF-3: S1 `OriginBound`, S2 `GlobalBound`.
- RF-5: S3 `RateBound`, S4 `MembersSpendNoRate`.
- RF-6: S5 `MemberCapBound`, S9 `SharesKept`, S8 `AuthNeverRefused`.
- RF-1 and RF-2: S6 `InJustified`.
- RF-2 and RF-6: S7 `OutNeverRefused`.
- Liveness: L1 `MembersEventuallyUp` (RF-2, RF-4).

**Mutants.** MF1-MF13, including MF12 (a blind refund, `tokens = min(B, tokens + 1)`, which the bucket version must
catch) and MF13. There are seven witnesses, one of them for §4's denial → RLY1. All are listed in the TLA README.

## 6. Code expected (step 2)

- **`protocol/admit`:** a pure ledger (open, identify, close, refill; time injected) with:
  - vectors `protocol/vectors/admit.json`, one bound, bound + 1 and recovery case per rule;
  - `FuzzLedger`, which checks the invariants;
  - a `rapid` model test;
  - no import of inkwell's internals, libp2p or I/O.
- **`protocol/limits.go` and `testdata/limits.json`:** `limits.json` is written at step 1 (the four rows and the
  QUIC-window row pinned). `limits.go` must add, with these exact values: `MaxConnectionsPerPeer` 4,
  `MemberShareConnections` 2, `MaxMemberConnections` 128, `MaxConnectedMembers` 64, `ConnectionsPerRelay` 2,
  `MaxRelays` 4, `MaxNodeConnections` 184, `MaxPendingPerRelay` 16, `MaxPendingPerIP` 8, `PendingIPv4PrefixBits` 32,
  `PendingIPv6PrefixBits` 56, `PendingIPv6WidePrefixBits` 48, `MaxPendingPerIPv6WidePrefix` 64, `MaxPending` 48,
  `PendingDeadline` 15 s, `AdmissionBurst` 16, `AdmissionRefill` 5 s, `RelayMaxCircuitsPerPeer` 128,
  `RelayMaxReservations` 256, `RelayReservationsPerIP` 64, `RelayConnectionsPerIP` 64, `RelayServiceStreamsPerPeer`
  130, `RelayServiceStreams` 8 192. It must drop `MaxInboundConnections` and `MaxOutboundConnections`
  (`TestLimitsMatchSpecification` is red until then). The node stops copying `maxPeers`.
- **`internal/transport/p2p/admission.go`:** a `network.ResourceManager` wrapper that forwards every method (the
  footgun of go-libp2p#2805), derives the origin, and wraps the scope's `SetPeer` and `Done`.
- **`node.go`:** the 184 limits, with libp2p's per-IP and rate limiters off.
- **`relay.go`:** RF-7, and the admission layer with `Allow` as the members.
- `LiftRelayIPLimit` and its three uses are removed.
- Spec §16.1 and `CHANGES.md` are updated.

## 7. Gate (`gates/RF1.toml`)

| Criterion | What it checks |
|---|---|
| `docs` | the spec rows and the TLA README section |
| `tla` | 26 runs |
| `admit` | the named bound tests, the vectors, 30 s of fuzzing, the import check |
| `go` | N = 10 and N = 17 behind one relay; nine members behind one IPv4; a stranger flood; a Byzantine member's share; origin derivation; the wrapper's forwarding |
| `bounds` | node 128/129, relay circuits 128/129, relay reservations 64/65, each with its recovery |
| `mutants` | Go mutants MG1-MG6 |
| `mutation` | `protocol/admit` ≥ 95 % |
| `regression` | review item 4: every test that reads what the block changes, namely `internal/transport`, `internal/transport/p2p`, `internal/app`, `cmd/inkwell-relay`, and the whole `protocol` module (`TestLimitsMatchSpecification` reads §16) |
| `netsim` | scenario 9, crash |
| `simplify-report` | the `/simplify` report |

`TestLimitsMatchSpecification` already fails on `main` (`df31531`). PU1d changed the §16.1 sync-ACK row
(« refus, violation ou délai … elle seule (P-16) ») without updating `limits.json`, and PU1d's gate did not run the
protocol module. This is reported to the integrator, and RF1's `regression` stays red on today's tree for that
reason until it is fixed.

## 8. Decisions (coordinator, 2026-11-24) and upstream text

- **Core.** The pure accounting core is a new `protocol/admit`: pure, no I/O, vectors and fuzzing, nothing of inkwell
  imported.
- **libp2p's limiters.** Its rate limiter and per-IP limiter are switched off explicitly where admission owns them.
- **Connections per member: 4, not 2.** Two is not enough at all times. HP1c2 (review 3b) has three connections
  coexist: a dead acknowledged middle one awaiting its P-15 proof (≤ 5 s), the new circuit, and the direct upgrade
  (`SupersedeThree.cfg`, `DirectUpgradeCrash.cfg`). A crossed simultaneous dial adds a fourth. The 2-per-member share
  plus the surplus rule keeps a Byzantine member from taking another member's room (`SharesKept`, MF10). At 64
  members the surplus is 2 connections, so a third connection may wait for P-15 to close the dead one.
- **Relays.** At most 4 relays, and 64 per IP on the relay: both stand.
- **QUIC memory.** 184 MiB against PU1c's 256 MiB, with 72 MiB of headroom (§3).
- **Upstream issue.** The draft below is to file on go-libp2p; filing waits for the user and is tracked → LP1.

> **rcmgr: relayed connections are counted under the relay's IP address.** Since #2805 the circuit transport calls
> `ResourceManager.OpenConnection` with the full circuit address (`/ip4/<relay>/…/p2p/<relay>/p2p-circuit/…`). The
> inbound path does the same through `upgrader/listener.go` (`conn.RemoteMultiaddr()`). `manet.ToIP` takes the first
> IP, so the relay connection and every circuit through it share the connection limiter's 8 per /32 and one rate
> bucket. A host reachable only through one relay can then hold at most 7 relayed peers, in either direction.
> #2805's comment wanted circuits limited « by everything but the IP ». Proposal: charge a circuit to its relay's
> PeerID (or not to the IP limiter at all), and document that an endpoint with no IP shares a single /56 rate bucket
> (`x/rate` `SubnetLimiter.Allow` with a zero `netip.Addr`). Reproduction: a host with 8 relayed peers through one
> relay, the default resource manager, v0.52.0.

Sources (read 2026-11-24): go-libp2p [#2805](https://github.com/libp2p/go-libp2p/pull/2805),
[#3431](https://github.com/libp2p/go-libp2p/issues/3431), [#3453](https://github.com/libp2p/go-libp2p/pull/3453),
[releases](https://github.com/libp2p/go-libp2p/releases) (v0.52.0, 2026-11-14); master `circuitv2/client/transport.go`
and the `resource-manager` history (unchanged since 2026-02-26);
[libp2p-connection-limits 0.7](https://docs.rs/libp2p-connection-limits/latest/libp2p_connection_limits/struct.ConnectionLimits.html);
[circuit relay v2 spec](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md) (limits are local policy);
[x/time/rate](https://pkg.go.dev/golang.org/x/time/rate#Reservation.CancelAt) v0.16.0 source;
[`netip.Addr.Prefix`](https://pkg.go.dev/net/netip#Addr.Prefix); libp2p forum
[« connections per ip limit exceeded »](https://discuss.libp2p.io/t/how-to-overcome-connections-per-ip-limit-exceeded/2456).
