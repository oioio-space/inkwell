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
// written by a key that was granted admin in that view, whether or not the
// grant took effect or was later revoked (so revoked admins backdate), and
// targets any key, the creator included.
func genDAG(rt *rapid.T, id ProjectID, genesis Op, keys []*testSigner) []Op {
	all := []Op{genesis}
	hashes := []Hash{genesis.Hash()}
	byHash := map[Hash]Op{hashes[0]: genesis}
	n := rapid.IntRange(8, 40).Draw(rt, "ops")
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
		granted := map[string]bool{}
		for h := range view {
			for _, p := range byHash[h].Prev {
				referenced[p] = true
			}
			if o := byHash[h]; o.Role == RoleAdmin {
				granted[string(o.Subject)] = true
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
			if granted[string(k.Public())] {
				authors = append(authors, k)
			}
		}
		author := authors[rapid.IntRange(0, len(authors)-1).Draw(rt, "author")]
		op := Op{
			Kind:    rapid.SampledFrom([]OpKind{OpAdd, OpAdd, OpSetRole, OpRemove, OpRemove}).Draw(rt, "kind"),
			Subject: keys[rapid.IntRange(0, len(keys)-1).Draw(rt, "subject")].Public(),
			Author:  author.Public(),
			Role:    rapid.SampledFrom([]Role{RoleAdmin, RoleAdmin, RoleWriter, RoleReader}).Draw(rt, "role"),
			Name:    rapid.SampledFrom([]string{"", "", "n1", "n2"}).Draw(rt, "name"),
			Prev:    prev,
		}
		if op.Kind == OpAdd {
			op.Person = rapid.SampledFrom([]Hash{{}, {}, {7}}).Draw(rt, "person")
		}
		// Lamport is 1 + the max over the parents, as a real author computes it.
		for _, p := range prev {
			op.Lamport = max(op.Lamport, byHash[p].Lamport+1)
		}
		if op.Kind == OpRemove {
			op.Role, op.Name = 0, ""
		}
		op.sign(author, id)
		h := op.Hash()
		all, hashes, byHash[h] = append(all, op), append(hashes, h), op
	}
	return all
}

func testKeys() []*testSigner {
	return []*testSigner{newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(5)}
}

// genList draws a DAG and applies it to a fresh list.
func genList(rt *rapid.T, keys []*testSigner) (*List, []Op) {
	genesis, id := Genesis(keys[0], "p", "u")
	ops := genDAG(rt, id, genesis, keys)
	l := New(id)
	slices.SortFunc(ops, func(a, b Op) int { return int(a.Lamport) - int(b.Lamport) })
	for _, o := range ops {
		if err := l.Apply(o); err != nil {
			rt.Fatalf("Apply(generated op) = %v", err)
		}
	}
	return l, ops
}

func TestFoldOrderIndependent(t *testing.T) {
	keys := testKeys()
	rapid.Check(t, func(rt *rapid.T) {
		ref, ops := genList(rt, keys)
		for range 4 {
			rng := rand.New(rand.NewPCG(rapid.Uint64().Draw(rt, "seed"), 1)) // #nosec G404 -- shuffling a test input.
			perm := slices.Clone(ops)
			rng.Shuffle(len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
			l := New(ref.Project())
			for _, o := range perm {
				_ = l.Apply(o)
			}
			if d := cmp.Diff(ref.Members(), l.Members()); d != "" {
				rt.Fatalf("members differ (-ref +perm):\n%s", d)
			}
			if d := cmp.Diff(ref.Frontier(), l.Frontier()); d != "" {
				rt.Fatalf("frontier differs:\n%s", d)
			}
			if ref.Len() != l.Len() || ref.Pending() != l.Pending() || ref.Sealed() != l.Sealed() {
				rt.Fatalf("len/pending/sealed differ: %d/%d/%v vs %d/%d/%v", ref.Len(), ref.Pending(), ref.Sealed(), l.Len(), l.Pending(), l.Sealed())
			}
		}
	})
}
