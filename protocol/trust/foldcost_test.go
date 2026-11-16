// SPDX-License-Identifier: MPL-2.0

package trust

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// ancestorCount counts the ops that o sees, o excluded, by walking Prev: the
// oracle of rule K-2, independent of the fold's ancestor bitsets.
func ancestorCount(l *List, o Hash) int {
	seen := make(map[Hash]bool)
	for stack := []Hash{o}; len(stack) > 0; {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, p := range l.ops[x].op.Prev {
			if !seen[p] {
				seen[p] = true
				stack = append(stack, p)
			}
		}
	}
	return len(seen)
}

// wantFoldCost folds l and requires the cost of TC1: exactly sweeps sweeps
// (K-1: one, plus one per self-cut of cuts) over exactly swept ops (K-2: the
// n ops, plus the ancestors of each self-cut). The closed form the caller
// gives is first checked against the walk of Prev.
func wantFoldCost(t testing.TB, l *List, cuts []Hash, sweeps, swept int) {
	t.Helper()
	n := l.Len()
	sum := n
	for _, c := range cuts {
		sum += ancestorCount(l, c)
	}
	if sweeps != 1+len(cuts) || swept != sum {
		t.Fatalf("closed form (%d sweeps, %d ops swept) differs from the list: 1 + %d self-cuts, n + Σ|anc| = %d", sweeps, swept, len(cuts), sum)
	}
	gotSweeps, gotSwept := foldCost(t, fold(l.ops))
	if gotSweeps != sweeps {
		t.Errorf("K-1 sweeps = %d, want %d (one, plus one per self-cut: %d) for %d ops", gotSweeps, sweeps, len(cuts), n)
	}
	if gotSwept != swept {
		t.Errorf("K-2 ops swept = %d, want %d (n = %d, plus the ancestors of each self-cut)", gotSwept, swept, n)
	}
}

// TC1 K-1 and K-2 at the seal: the two shapes of self-cuts at their worst.
// Chained cuts are the worst of all lists: each op but the genesis is a
// self-cut that sees every op before it.
func TestSelfCutsCostAtSeal(t *testing.T) {
	a := newSigner(1)
	selfCut := func(t *testing.T, l *List, i int, prev []Hash) Op {
		t.Helper()
		o := craft(t, l, a, OpSetRole, a, RoleWriter, fmt.Sprint(i), prev)
		mustApply(t, l, o)
		return o
	}
	// A short chain first: without the memo a chain of s cuts costs 2^s sweeps,
	// so the lists at the seal would never end instead of failing.
	if !t.Run("chain of 12", func(t *testing.T) {
		l := project(t, a)
		prev := l.Frontier()
		var cuts []Hash
		for i := range 12 {
			o := selfCut(t, l, i, prev)
			cuts, prev = append(cuts, o.Hash()), h(o)
		}
		wantFoldCost(t, l, cuts, 13, 91) // 1 + s ; n(n+1)/2 = 13·14/2.
	}) {
		return
	}
	t.Run("sibling", func(t *testing.T) {
		const m, s = maxOps / 2, maxOps / 2 // m ops, then s cuts that all see exactly those.
		l := project(t, a)
		if err := chain(t, l, a, m-1); err != nil {
			t.Fatalf("chain() = %v", err)
		}
		prev := l.Frontier()
		var cuts []Hash
		for i := range s {
			cuts = append(cuts, selfCut(t, l, i, prev).Hash())
		}
		wantFoldCost(t, l, cuts, 513, 263_168) // 1 + s ; n + s·m = 1024 + 512·512.
	})
	t.Run("chained", func(t *testing.T) {
		l := project(t, a)
		prev := l.Frontier()
		var cuts []Hash
		for i := range maxOps - 1 {
			o := selfCut(t, l, i, prev)
			cuts, prev = append(cuts, o.Hash()), h(o)
		}
		wantFoldCost(t, l, cuts, 1024, 524_800) // 1 + s ; n + s·m + s(s−1)/2 = 1024 + 1023 + 1023·1022/2.
	})
}

// TC1 K-1 and K-2 on any list: a self-cut whose author never holds a rank is
// not judged, so the exact cost of the creator's lists is the upper bound of
// every list, and one sweep over the n ops is the lower one.
func TestFoldCostWithinSelfCutBound(t *testing.T) {
	keys := testKeys()
	withCuts := 0 // the draws that hold a self-cut: without one the bound says nothing.
	defer func() {
		if withCuts == 0 && !t.Failed() {
			t.Errorf("K-1 bound: no drawn list holds a self-cut")
		}
	}()
	rapid.Check(t, func(rt *rapid.T) {
		l, _ := genList(rt, keys)
		m := newRefModel(l)
		n, s := l.Len(), 0
		bound := n
		for i, nd := range m.nodes {
			if m.selfCut(i) {
				s++
				bound += ancestorCount(l, nd.hash)
			}
		}
		if s > 0 {
			withCuts++
		}
		sweeps, swept := foldCost(rt, fold(l.ops))
		// One text for both bounds: rapid shrinks to any failure, so a mutant must find the same words either way.
		if sweeps < 1 || sweeps > 1+s || swept < n || swept > bound {
			rt.Fatalf("K-1 bound and K-2 bound: %d sweeps over %d ops for %d ops and %d self-cuts, want 1 to %d sweeps over %d to %d ops", sweeps, swept, n, s, 1+s, n, bound)
		}
	})
}
