---
status: accepted
date: 2026-11-16
decision-makers: [Mathieu]
---

# Relayed links retry the direct upgrade, with budgets at the hole-punch choke points

## Context and problem

libp2p calls DCUtR once per new relayed connection and never again. A peer that restarts, crashes or changes
network while linked (PC1b's restart, R1's « tuer et relancer les deux pairs »), or whose identify push of
`/libp2p/dcutr` is lost, stays relayed for ever. Upstream leaves the remedy to the caller (go-libp2p #3024,
#3420). Evidence, prior art and alternatives: `docs/plans/2026-11-16-hp1-direct-upgrade.md` §1-§2, §8.

## Decision drivers

- liveness: a relayed link to a member becomes direct as soon as the network allows, after any restart;
- full Byzantine members (project threat model): a peer must not make the node punch or dial without bound,
  by initiating, reconnecting, toggling its protocols or announcing addresses;
- strangers (the invitation window) get no automatic direct path and none of our public addresses;
- energy: nothing on a ladder while the app is idle;
- the fork stays one small, upstreamable change.

## Considered options

- A. Event-driven retry with a slow safety ladder in `internal/transport/p2p`, the policy pure in
  `protocol/directup`, the budgets at libp2p's hole-punch address filter and the connection gater, the
  host's own hole-punch service captured by a public option.
- B. Retry inside the go-libp2p fork.
- C. Events only, no timer.
- D. Make the netsim bench wait after a restart.
- E. Close the circuit and redial to re-trigger libp2p's one-shot.

## Decision

Option A (plan §3), because it fixes both races at their event, keeps a slow ladder for silent changes (a
firewall lifted: 16 min cap), and enforces every bound at the two points every punch passes, libp2p's own
attempts and the answerer side included. The only fork change is `(*holepunch.Service).DirectConnectContext`,
the existing call with a context, so that a new epoch, an unwant, a lease end or Close cancels a call at once
(patch and upstream text: `docs/plans/2026-11-16-hp1-libp2p-directconnectcontext.*`).

Peer-observable bounds (spec §8.6, §16.1): one call in flight per peer, ≤ 12 calls per peer in any hour;
≤ 45 DCUtR rounds and ≤ 174 non-relay destinations per member in any hour, ≤ 3 addresses per round; 4
node-wide slots granted FIFO with a 75 s lease, 12 destinations per grant, ≤ 38 grants per node in any hour;
nothing for a non-member. With two circuits nobody initiates and the one the higher PeerID dialled is closed
once the session is off it (H-15).

No black hole detector (H-18, step 2, 2026-11-16, the integrator's decision after a debate with Codex): the node
builds its host with `libp2p.UDPBlackHoleSuccessCounter(nil)` and `libp2p.IPv6BlackHoleSuccessCounter(nil)`.
libp2p's detector refuses every new public UDP (IPv6) dial once fewer than 5 of the last 100 results succeeded, with
no time expiry and without knowing who chose the addresses (`p2p/net/swarm/black_hole_detector.go`): a member
pointing the node at unreachable addresses would shut the honest direct upgrades of every peer, and the budgets
do not prevent it (in `TestUpgradeNodeTraffic`, 37 grants gave 215 distinct destinations; the detector blocked the
node's UDP dials after about 16 simulated minutes). Keeping it with a per-member cap does not hold against a
coalition, and excluding only hole-punch results would still leave the first direct dial and the filtering itself.

Retention of a direct connection (H-19, block HP1r, 2026-11-16, the integrator's decision after Codex's review): a node keeps an outbound non-relay connection only if, once registered, it holds a circuit to the peer, wants it, or is connecting to it on explicit request (a `connecting` counter decremented on every exit); any other is closed in the node's existing `ConnectedF`, before the node accepts a stream on it, so the peer cannot complete a session over it. No clean-up runs when the last circuit closes: a connection registered while a circuit was held is that
circuit's upgrade, and closing it then would undo every successful one-sided upgrade (Codex's freeze review). Cancelling our call (H-4) is not enough: the late connection observed was libp2p's one-shot, started on `context.Background`, and a swarm dial outlives any one caller's context; a connection cannot be attributed to its dial (`dial_sync.go`), so the rule is on the node's reasons, not on circuit incarnations. Rejected: binding the one-shot and the answerer to their circuit in the fork (more fork hunks, the dial worker's race remains), disabling libp2p's one-shot (redesign of the first direct dial, the answerer and the race remain), a gater-only check (`InterceptSecured` runs before registration: a window between check and use), a clean-up at the last circuit's loss (it closes the legitimate upgrade). Limits: stated in the plan's H-19 row: the connection is published for a moment before `ConnectedF`, so a stream another consumer opens in that moment is not excluded; a guarantee of « no stream at all » would need a barrier inside the swarm; a late connection registered
between the peer's Drop and this node's removal of the circuit passes the check and only `Admit` refuses it.

### Consequences

- Good: a restart is followed by a direct link within seconds; a hopeless pair costs ≈ 4 calls an hour, only
  while the app is in use; a Byzantine member costs ≤ 5.9 MB/h, a whole node ≤ 15.5 MB/h.
- Bad (objection kept): on a network that really blocks UDP, the node keeps making UDP attempts the detector would
  have saved; they are bounded by the per-member (174) and node-wide (456) destination budgets.
- Neutral: libp2p's first direct dial of its one-shot gets its grant on demand, once, through the same FIFO
  (no bypass), and that grant serves the whole one-shot, its rounds included (one node token): refused with the
  ticket kept when the FIFO grants none, so under contention the NAT mapping it used to leave as a side effect
  comes only with a later round (plan §3.3a); the session's
  stream moves wait for the old stream's negotiation, and the order of inbound session streams is taken as
  they arrive, per peer.

### Confirmation

Gates `gates/HP1.toml` (spec, TLA+ S1-S17 L1-L3 and 29 mutants, Alloy roles, Go tests H-1..H-18, gremlins on
`protocol/directup`, corpus `protocol/vectors/directup.json`), `HP1m` (Go mutants), `HP1n`, `HP1n2`, `HP1n3`
(netsim scenarios 1-9, the netsim mutants), `HP1r` (H-19: TLA+ S18 and M30, Go `TestUpgradeLateLinkClosed`, mutant MR1).

## More information

Plan `docs/plans/2026-11-16-hp1-direct-upgrade.md` (rules H-1..H-17, Codex debate D1-D6, B1-B6); DCUtR
specification https://github.com/libp2p/specs/blob/master/relay/DCUtR.md. Reconsider when go-libp2p retries
DCUtR itself or exposes a cancellable call upstream.
