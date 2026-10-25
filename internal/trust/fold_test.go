package trust

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

// genDAG draws a random DAG of ops over a few keys: each op sees a random
// downward-closed view (so concurrency, merges and antedating all occur), is
// written by a key that was added in that view, and targets any key.
func genDAG(rt *rapid.T, id ProjectID, genesis Op, keys []*testSigner) []Op {
	all := []Op{genesis}
	hashes := []Hash{genesis.Hash()}
	byHash := map[Hash]Op{hashes[0]: genesis}
	n := rapid.IntRange(10, 30).Draw(rt, "ops")
	for range n {
		view := map[Hash]bool{}
		var visit func(h Hash)
		visit = func(h Hash) {
			if view[h] {
				return
			}
			view[h] = true
			for _, p := range byHash[h].Prev {
				visit(p)
			}
		}
		for range rapid.IntRange(1, 2).Draw(rt, "views") {
			visit(hashes[rapid.IntRange(0, len(hashes)-1).Draw(rt, "base")])
		}
		referenced := map[Hash]bool{}
		known := map[string]bool{string(genesis.Author): true}
		for h := range view {
			for _, p := range byHash[h].Prev {
				referenced[p] = true
			}
			if o := byHash[h]; o.Kind == OpAdd {
				known[string(o.Subject)] = true
			}
		}
		var prev []Hash
		for h := range view {
			if !referenced[h] {
				prev = append(prev, h)
			}
		}
		slices.SortFunc(prev, func(a, b Hash) int { return slices.Compare(a[:], b[:]) })
		var authors []*testSigner
		for _, k := range keys {
			if known[string(k.Public())] {
				authors = append(authors, k)
			}
		}
		author := authors[rapid.IntRange(0, len(authors)-1).Draw(rt, "author")]
		op := Op{
			Kind:    rapid.SampledFrom([]OpKind{OpAdd, OpAdd, OpSetRole, OpRemove, OpRemove}).Draw(rt, "kind"),
			Subject: keys[rapid.IntRange(0, len(keys)-1).Draw(rt, "subject")].Public(),
			Author:  author.Public(),
			Role:    Role(rapid.IntRange(1, 3).Draw(rt, "role")),
			Prev:    prev,
		}
		// Lamport is 1 + the max over the parents, as a real author computes it.
		for _, p := range prev {
			op.Lamport = max(op.Lamport, byHash[p].Lamport+1)
		}
		if op.Kind == OpRemove {
			op.Role = 0
		}
		op.sign(author, id)
		h := op.Hash()
		all, hashes, byHash[h] = append(all, op), append(hashes, h), op
	}
	return all
}

func TestFoldOrderIndependent(t *testing.T) {
	keys := []*testSigner{newSigner(1), newSigner(2), newSigner(3), newSigner(4)}
	rapid.Check(t, func(rt *rapid.T) {
		genesis, id := Genesis(keys[0], "p", "u")
		ops := genDAG(rt, id, genesis, keys)

		ref := New(id)
		slices.SortFunc(ops, func(a, b Op) int { return int(a.Lamport) - int(b.Lamport) })
		for _, o := range ops {
			_ = ref.Apply(o) // rejected ops are rejected in every order.
		}
		if ref.Len() > 0 && !slices.ContainsFunc(ref.Members(), func(m Member) bool { return m.Role == RoleAdmin }) {
			rt.Fatalf("no admin left: %+v", ref.Members())
		}

		for range 4 {
			rng := rand.New(rand.NewPCG(rapid.Uint64().Draw(rt, "seed"), 1)) // #nosec G404 -- shuffling a test input.
			perm := slices.Clone(ops)
			rng.Shuffle(len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
			l := New(id)
			for _, o := range perm {
				_ = l.Apply(o)
			}
			if d := cmp.Diff(ref.Members(), l.Members()); d != "" {
				rt.Fatalf("members differ (-ref +perm):\n%s", d)
			}
			if d := cmp.Diff(ref.Frontier(), l.Frontier()); d != "" {
				rt.Fatalf("frontier differs:\n%s", d)
			}
			if ref.Len() != l.Len() || ref.Pending() != l.Pending() {
				rt.Fatalf("len/pending differ: %d/%d vs %d/%d", ref.Len(), ref.Pending(), l.Len(), l.Pending())
			}
		}
	})
}
