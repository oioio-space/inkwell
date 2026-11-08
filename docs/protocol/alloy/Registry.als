-- Decision registry D2 of ADR 0030 section 2, for the `node/<id>` keys: among the standing
-- decisions that write a key, take the frontier (those no other such decision has seen), then the
-- most prudent value, kept > trashed > purged ("le refus gagne", FM-08). A frontier holding more
-- than one value is `contested`. Standing itself (D1) is checked in Decisions.als and taken as
-- given here. Run: mise run alloy.
module Registry

abstract sig Value {}
one sig Kept, Trashed, Purged extends Value {}

-- Prudence order: Kept is the most prudent.
fun rank[v: Value]: Int { v = Kept => 2 else (v = Trashed => 1 else 0) }

sig Node {}

sig Decision {
  node: one Node,
  value: one Value,
  sees: set Decision      -- causal past (transitive, acyclic)
}

fact Causal {
  all d: Decision | d not in d.^sees and d.sees = d.^sees
}

fun writers[n: Node]: set Decision { node.n }
fun frontier[n: Node]: set Decision { writers[n] - writers[n].sees }

-- The registry value: the most prudent value present in the frontier.
fun registry[n: Node]: lone Value {
  { v: frontier[n].value | all w: frontier[n].value | rank[v] >= rank[w] }
}

pred contested[n: Node] { #(frontier[n].value) > 1 }

-- ── Properties ──────────────────────────────────────────────────────────────────────────────────

-- Refusal wins: if a frontier decision keeps the node, the node is kept.
assert RefusalWins {
  all n: Node | Kept in frontier[n].value implies registry[n] = Kept
}

-- A value can only be outranked by a decision that did not see it, never silently dropped:
-- the registry value always comes from the frontier.
assert FromFrontier {
  all n: Node | some writers[n] implies registry[n] in frontier[n].value
}

-- Superseding works: a later decision that saw every other writer of the node decides alone.
assert LaterDecisionWins {
  all n: Node, d: writers[n] | (writers[n] - d) in d.sees implies registry[n] = d.value
}

-- A purge never wins over a concurrent keep or trash.
assert PurgeNeverBeatsConcurrent {
  all n: Node | (Purged in frontier[n].value and some (frontier[n].value - Purged))
    implies registry[n] != Purged
}

-- A contested key is exactly one whose frontier disagrees.
assert ContestedMeansDisagreement {
  all n: Node | contested[n] iff (some disj a, b: frontier[n] | a.value != b.value)
}

check RefusalWins for 6 but 4 Int
check FromFrontier for 6 but 4 Int
check LaterDecisionWins for 6 but 4 Int
check PurgeNeverBeatsConcurrent for 6 but 4 Int
check ContestedMeansDisagreement for 6 but 4 Int

run ConcurrentKeepAndPurge {
  some n: Node | Kept in frontier[n].value and Purged in frontier[n].value
} for 6 but 4 Int
