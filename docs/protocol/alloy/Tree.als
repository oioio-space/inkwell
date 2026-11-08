-- Materialisation of the project tree from the Automerge index (ADR 0023, "Matérialisation").
-- The index may be hostile or concurrent: parents may form cycles, point to a missing node, and
-- siblings may share a name. The visible tree is a pure function of the index:
--   * a node whose parent is missing is attached to the root;
--   * in a cycle, the node with the smallest id is attached to the root;
--   * among homonym siblings the smallest id keeps the name, the others are renamed (suffix).
-- Run: mise run alloy.
module Tree
open util/ordering[Id]

sig Id {}
sig Name {}

one sig Root {}

sig Node {
  id: one Id,
  name: one Name,
  parent: lone Node        -- the index's `parent`; none = missing (unknown or deleted id)
}

fact UniqueIds { all disj a, b: Node | a.id != b.id }

-- Nodes on a parent cycle of the raw index.
fun onCycle: set Node { { n: Node | n in n.^parent } }

-- The cycle breaker of each cycle: the smallest id among the nodes of that cycle.
fun breakers: set Node {
  { n: onCycle | all m: onCycle | (m in n.^parent and n in m.^parent) implies lte[n.id, m.id] }
}

-- Materialised parent: Root for a missing parent or a cycle breaker, else the index parent.
fun mparent: Node -> (Node + Root) {
  { n: Node, p: Node + Root |
      (no n.parent or n in breakers) => p = Root else p = n.parent }
}

-- Homonyms keep the name only for the smallest id among siblings that share it.
fun keepsName: set Node {
  { n: Node | all m: Node - n |
      (n.mparent = m.mparent and n.name = m.name) implies lt[n.id, m.id] }
}

-- ── Properties ──────────────────────────────────────────────────────────────────────────────────

-- Every node has exactly one materialised parent.
assert OneParent { all n: Node | one n.mparent }

-- The materialised tree has no cycle.
assert Acyclic { no n: Node | n in n.^mparent }

-- Every node is reachable from the root.
assert Rooted { all n: Node | Root in n.^mparent }

-- After renaming, no two siblings show the same name.
assert UniqueVisibleNames {
  all disj a, b: keepsName | a.mparent = b.mparent implies a.name != b.name
}

-- A node outside any cycle with a present parent keeps its index parent (no needless moves).
assert NoNeedlessMove {
  all n: Node | (some n.parent and n not in onCycle) implies n.mparent = n.parent
}

check OneParent for 6
check Acyclic for 6
check Rooted for 6
check UniqueVisibleNames for 6
check NoNeedlessMove for 6

run TwoCyclesAndHomonyms {
  some disj a, b: onCycle | a not in b.^parent
  some disj x, y: Node | x.mparent = y.mparent and x.name = y.name
} for 7
