-- Admin decisions outside the CRDT: admission A4 and the "standing" rule D1 of ADR 0030 section 2.
-- The member-list fold itself (ADR 0017) is abstracted: a grant or a cut is an op whose effect is
-- read at a frontier. What this model checks is how decisions interact with concurrent revocations.
-- Run: mise run alloy (every `check` must find no counterexample; every mutant must find one).
module Decisions

sig Key {}
one sig Founder in Key {}

abstract sig Op {
  author: one Key,
  past: set Op            -- strict causal past (transitive, acyclic)
}

abstract sig Kind {}
one sig Grant, Cut extends Kind {}

sig MemberOp extends Op {
  kind: one Kind,
  target: one Key,
  seen: set DecisionOp     -- `Seen`: decisions the author had seen (ADR 0030, amendment of 0017)
}

sig DecisionOp extends Op {
  members: set MemberOp    -- `Members`: member-list frontier seen by the author
}

fact CausalOrder {
  all o: Op | o not in o.^past
  all o: Op | o.past = o.^past
  -- a frontier is part of the author's causal past
  all d: DecisionOp | d.members in d.past
  all m: MemberOp | m.seen in m.past
}

fact NoSelfOps {
  all m: MemberOp | m.target != m.author
}

-- Member ops known at a decision's frontier: the frontier and everything before it.
fun knownMembers[d: DecisionOp]: set MemberOp { (d.members + d.members.past) & MemberOp }

-- Admin at a frontier: the founder, or granted there and not cut there (abstract fold).
pred adminAt[k: Key, ms: set MemberOp] {
  k = Founder or {
    some g: ms | g.kind = Grant and g.target = k
    no c: ms | c.kind = Cut and c.target = k
  }
}

-- A member op is concurrent to a decision when neither saw the other (ADR 0030 D1(b)).
pred concurrent[m: MemberOp, d: DecisionOp] {
  m not in knownMembers[d]
  d not in (m.seen + m.seen.past)
}

-- D1: a decision stands.
pred standing[d: DecisionOp] {
  adminAt[d.author, knownMembers[d]]                          -- (a)
  no c: MemberOp | c.kind = Cut and c.target = d.author and
     c.author != d.author and concurrent[c, d]               -- (b)
}

-- ── Properties ──────────────────────────────────────────────────────────────────────────────────

-- A4 + D1(a): a key never granted (a reader or a writer) cannot make any decision stand.
assert NoEscalation {
  all d: DecisionOp | standing[d] implies
    (d.author = Founder or some g: MemberOp | g.kind = Grant and g.target = d.author)
}

-- An admin cut by another key, writing with a frontier that predates the cut, stays dead when
-- the cut did not see that decision ("un admin retiré qui antidate reste mort").
assert AntedatedStaysDead {
  all d: DecisionOp, c: MemberOp |
    (c.kind = Cut and c.target = d.author and c.author != d.author and
     c not in knownMembers[d] and d not in (c.seen + c.seen.past))
    implies not standing[d]
}

-- What an admin decided and the revoker had seen stands
-- ("ce qu'un admin a décidé et que son révocateur avait vu tient").
assert SeenByRevokerStands {
  all d: DecisionOp |
    (adminAt[d.author, knownMembers[d]] and
     all c: MemberOp | (c.kind = Cut and c.target = d.author and c.author != d.author)
        implies (c in knownMembers[d] or d in (c.seen + c.seen.past)))
    implies standing[d]
}

-- Once every later decision's frontier contains the cut, the cut key makes nothing new stand.
assert RevocationEffective {
  all c: MemberOp, d: DecisionOp |
    (c.kind = Cut and d.author = c.target and d.author != Founder and c in knownMembers[d])
    implies not standing[d]
}

check NoEscalation for 6
check AntedatedStaysDead for 6
check SeenByRevokerStands for 6
check RevocationEffective for 6

-- Exploration (must be satisfiable): two admins who cut each other concurrently.
run MutualCut {
  some disj a, b: Key, disj ca, cb: MemberOp |
    ca.kind = Cut and ca.author = a and ca.target = b and
    cb.kind = Cut and cb.author = b and cb.target = a and
    ca not in cb.past and cb not in ca.past
} for 6
