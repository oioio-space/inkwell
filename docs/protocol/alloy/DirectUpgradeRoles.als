-- Block HP1 (docs/plans/2026-11-16-hp1-direct-upgrade.md §3.4, rule H-3): who initiates the DCUtR exchange.
-- Two peers and any set of live relayed circuits between them, each dialled by one side. Each side's
-- NewStream picks one circuit (go-libp2p swarm.isBetterConn: more open streams, then the later one in the
-- side's own connection list). The answerer resets a stream that arrives on its inbound circuit
-- (holepunch svc.go handleNewStream), so a side's stream is accepted only on a circuit it sees as inbound.
-- A circuit enters the dialler's list when its handshake completes there, and the receiver's list one
-- one-way delay later (the last handshake message travels to the receiver).
-- Run: mise run alloy.
module DirectUpgradeRoles

abstract sig Side {}
one sig L, R extends Side {}
one sig Lower in Side {}          -- the side with the lower PeerID

sig Circuit {
  dialler: one Side,
  tDial: one Int,                 -- when it entered the dialler's connection list
  delay: one Int,                 -- one-way delay: it enters the receiver's list that much later
  streams: Side -> one Int        -- open streams on it, as each end counts them
}

fact Bounds {
  all c: Circuit | c.tDial >= 0 and c.tDial <= 3 and c.delay <= 3
  all c: Circuit, s: Side | c.streams[s] >= 0 and c.streams[s] <= 2
  all c: Circuit | c.delay >= 1   -- a relayed handshake ends at the receiver after it ends at the dialler
}

fun receiver[c: Circuit]: Side { Side - c.dialler }
fun localTime[c: Circuit, s: Side]: Int { s = c.dialler => c.tDial else plus[c.tDial, c.delay] }

-- A1: both ends count the same open streams on each circuit (no stream opening in flight).
pred sameCounts { all c: Circuit | c.streams[L] = c.streams[R] }

-- NewStream's pick at side s: a circuit no other beats; on a full tie any of them.
pred better[a, b: Circuit, s: Side] {
  a.streams[s] > b.streams[s] or (a.streams[s] = b.streams[s] and localTime[a, s] > localTime[b, s])
}
fun picks[s: Side]: set Circuit { { c: Circuit | no d: Circuit | better[d, c, s] } }

one sig Pick { at: Side -> one Circuit }
fun pick[s: Side]: Circuit { s.(Pick.at) }
fact Picks { some Circuit and (all s: Side | pick[s] in picks[s]) }

pred holdsInbound[s: Side] { some c: Circuit | receiver[c] = s }
-- H-3: a side may Punch iff it holds an inbound circuit; with circuits both ways, both sides may.
pred eligible[s: Side] { holdsInbound[s] }
-- A side's DCUtR stream is accepted iff it rides a circuit the other side dialled.
pred accepted[s: Side] { receiver[pick[s]] = s }

-- ── Properties ──────────────────────────────────────────────────────────────────────────────────

-- For every circuit set and every pick (A1 holding), some eligible side's stream is accepted:
-- the role rule never stalls (« lower PeerID only » does: mutant).
assert SomeEligibleAccepted { sameCounts implies some s: Side | eligible[s] and accepted[s] }

-- No side punches without an inbound circuit (the outbound side's stream is reset before any punch).
assert PunchNeedsInbound { all s: Side | eligible[s] implies holdsInbound[s] }

-- The rule never forbids a side whose stream would be accepted.
assert AcceptedIsEligible { all s: Side | accepted[s] implies eligible[s] }

check SomeEligibleAccepted for 3 but 4 Int
check PunchNeedsInbound for 3 but 4 Int
check AcceptedIsEligible for 3 but 4 Int

-- Exploration: without A1 (an opening in flight counted at one end only) both ends can pick their own
-- outbound circuit; both streams are then reset. Transient: only while an opening is in flight (the TLA+
-- model's Settle step is fair for this case only; every other pick may last).
run CrossedWithoutSameCounts { not sameCounts and no s: Side | accepted[s] } for 3 but 4 Int
-- Exploration: with circuits both ways both streams can be accepted at once, durably with equal counts:
-- two exchanges run at once (plan §3.4, rule H-15).
run BothAccepted { sameCounts and all s: Side | accepted[s] } for 3 but 4 Int
