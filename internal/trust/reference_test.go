package trust

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"pgregory.net/rapid"
)

// refModel is a deliberately naive implementation of the fold rules: no
// bitsets, no heap, ancestors by graph search, ranks by repeated scans. It is
// O(n³) or worse and exists only to check the real fold.
type refModel struct {
	index   map[Hash]int
	memo    map[int]bool
	creator string
	nodes   []*node // in ≺ order.
}

func newRefModel(l *List) *refModel {
	m := &refModel{index: map[Hash]int{}, memo: map[int]bool{}}
	for _, n := range l.ops {
		m.nodes = append(m.nodes, n)
	}
	slices.SortFunc(m.nodes, cmpNodes)
	for i, n := range m.nodes {
		m.index[n.hash] = i
	}
	if len(m.nodes) > 0 {
		m.creator = string(m.nodes[0].op.Author)
	}
	return m
}

func (m *refModel) author(i int) string  { return string(m.nodes[i].op.Author) }
func (m *refModel) subject(i int) string { return string(m.nodes[i].op.Subject) }
func (m *refModel) level(i int) int      { return m.nodes[i].level() }
func (m *refModel) grant(i int) bool     { return m.level(i) == int(RoleAdmin) }

// sees reports whether j is a strict ancestor of i.
func (m *refModel) sees(i, j int) bool {
	for _, p := range m.nodes[i].op.Prev {
		if k := m.index[p]; k == j || m.sees(k, j) {
			return true
		}
	}
	return false
}

func (m *refModel) concurrent(i, j int) bool { return i != j && !m.sees(i, j) && !m.sees(j, i) }

// survivors drops the ops of e that have a concurrent op of e at a lower level.
func (m *refModel) survivors(e []int) (out []int) {
	for _, i := range e {
		dropped := false
		for _, j := range e {
			if m.concurrent(i, j) && m.level(j) < m.level(i) {
				dropped = true
			}
		}
		if !dropped {
			out = append(out, i)
		}
	}
	return out
}

type refResult struct {
	stands  map[int]bool
	rank    map[string]int
	members []Member
}

// othersOn returns the standing ops on x by other keys, in ≺ order.
func (m *refModel) othersOn(set []int, stands map[int]bool, x string) (out []int) {
	for _, i := range set {
		if m.subject(i) == x && m.author(i) != x && stands[i] {
			out = append(out, i)
		}
	}
	return out
}

// fold evaluates the rules on set, a downward-closed list of positions.
func (m *refModel) fold(set []int) refResult {
	res := refResult{stands: map[int]bool{}, rank: map[string]int{}}
	decide := func(y string, r int) {
		res.rank[y] = r
		e := m.othersOn(set, res.stands, y)
		surv := m.survivors(e)
		var self []int
		for _, i := range set {
			if m.author(i) != y {
				continue
			}
			if m.nodes[i].op.Kind == OpGenesis {
				res.stands[i] = true
				self = append(self, i)
				continue
			}
			killed := false
			for _, k := range e { // 3b: a standing cut on y concurrent to i.
				if m.level(k) < int(RoleAdmin) && m.concurrent(i, k) {
					killed = true
				}
			}
			last := -1 // 3a: the ≺-latest surviving role op on y that i sees.
			for _, j := range slices.Concat(surv, self) {
				if m.sees(i, j) && j > last {
					last = j
				}
			}
			if killed || last < 0 || !m.grant(last) {
				continue
			}
			if x := m.subject(i); x == y {
				if y == m.creator && !m.grant(i) && !m.leaves(i) {
					continue
				}
				self = append(self, i)
			} else if _, senior := res.rank[x]; senior {
				continue // 2: x is senior.
			}
			res.stands[i] = true
		}
	}
	decide(m.creator, 0)
	for { // 1: the next rank is the smallest pos of a surviving standing grant.
		best, bestPos := "", -1
		for _, i := range set {
			y := m.subject(i)
			if _, done := res.rank[y]; done {
				continue
			}
			for _, g := range m.survivors(m.othersOn(set, res.stands, y)) {
				if m.grant(g) && (bestPos < 0 || g < bestPos) {
					best, bestPos = y, g
				}
			}
		}
		if bestPos < 0 {
			break
		}
		decide(best, bestPos)
	}

	type entry struct {
		m   Member
		pos int
	}
	var out []entry
	done := map[string]bool{}
	for _, i := range set {
		x := m.subject(i)
		if done[x] {
			continue
		}
		done[x] = true
		var selfs []int
		for _, j := range set {
			if m.subject(j) == x && m.author(j) == x && res.stands[j] {
				selfs = append(selfs, j)
			}
		}
		l := slices.Concat(m.survivors(m.othersOn(set, res.stands, x)), selfs)
		slices.Sort(l)
		var mem Member
		present, pos := false, 0
		for _, j := range l {
			o := m.nodes[j].op
			if o.Kind == OpRemove {
				present = false
				continue
			}
			if !present {
				present, pos, mem = true, j, Member{Person: m.nodes[j].hash}
			}
			mem.Pub, mem.Role = o.Subject, o.Role
			if o.Kind != OpGenesis && o.Name != "" {
				mem.Name = o.Name
			}
			if o.Kind == OpAdd {
				pos, mem.Person = j, o.Person
				if o.Person == (Hash{}) {
					mem.Person = m.nodes[j].hash
				}
			}
		}
		if present {
			if x == m.creator {
				pos = 0
			}
			out = append(out, entry{mem, pos})
		}
	}
	slices.SortFunc(out, func(a, b entry) int { return a.pos - b.pos })
	for _, e := range out {
		res.members = append(res.members, e.m)
	}
	return res
}

// leaves reports whether the creator's cut of itself at i is honoured: another
// admin exists in the fold of i's ancestors.
func (m *refModel) leaves(i int) bool {
	if d, ok := m.memo[i]; ok {
		return d
	}
	var past []int
	for j := range i {
		if m.sees(i, j) {
			past = append(past, j)
		}
	}
	d := slices.ContainsFunc(m.fold(past).members, func(x Member) bool {
		return x.Role == RoleAdmin && string(x.Pub) != m.creator
	})
	m.memo[i] = d
	return d
}

func (m *refModel) all() []int {
	s := make([]int, len(m.nodes))
	for i := range s {
		s[i] = i
	}
	return s
}

func TestFoldMatchesReference(t *testing.T) {
	keys := testKeys()
	rapid.Check(t, func(rt *rapid.T) {
		l, _ := genList(rt, keys)
		ref := newRefModel(l)
		want := ref.fold(ref.all())
		v := l.current()
		if d := cmp.Diff(want.members, l.Members(), cmpopts.EquateEmpty()); d != "" {
			rt.Fatalf("members differ (-reference +fold):\n%s", d)
		}
		for i := range v.nodes {
			if v.stands[i] != want.stands[i] {
				rt.Fatalf("op %d stands = %v, reference says %v", i, v.stands[i], want.stands[i])
			}
		}
		if d := cmp.Diff(want.rank, v.rank); d != "" {
			rt.Fatalf("ranks differ (-reference +fold):\n%s", d)
		}
	})
}

func TestFoldInvariants(t *testing.T) {
	keys := testKeys()
	rapid.Check(t, func(rt *rapid.T) {
		l, _ := genList(rt, keys)
		m := newRefModel(l)
		v := l.current()
		creator := keys[0].Public()

		standingLeave := false
		seenRank := map[int]string{}
		for y, r := range v.rank {
			if other, dup := seenRank[r]; dup {
				rt.Fatalf("keys %x and %x share rank %d", other, y, r)
			}
			seenRank[r] = y
		}
		if v.rank[string(creator)] != 0 {
			rt.Fatalf("creator rank = %d, want 0", v.rank[string(creator)])
		}
		for i, ok := range v.stands {
			if !ok {
				continue
			}
			y, x := m.author(i), m.subject(i)
			if x == string(creator) && y != x {
				rt.Fatalf("op %d by another key on the creator stands", i)
			}
			if x != y { // every standing op on another key goes from a senior to a junior.
				if ry, ok := v.rank[y]; !ok {
					rt.Fatalf("op %d by the unranked key %x stands", i, y)
				} else if rx, ranked := v.rank[x]; ranked && rx < ry {
					rt.Fatalf("op %d by rank %d acts on senior rank %d", i, ry, rx)
				}
			}
			var others, selfs []int
			for j, jok := range v.stands {
				switch {
				case !jok || m.subject(j) != y:
				case m.author(j) == y:
					selfs = append(selfs, j)
				default:
					others = append(others, j)
					if m.level(j) < int(RoleAdmin) && m.concurrent(i, j) {
						rt.Fatalf("op %d stands though concurrent to the standing cut %d on its author", i, j)
					}
				}
			}
			if m.nodes[i].op.Kind == OpGenesis {
				continue
			}
			last := -1
			for _, j := range slices.Concat(m.survivors(others), selfs) {
				if m.sees(i, j) && j > last {
					last = j
				}
			}
			if last < 0 || !m.grant(last) {
				rt.Fatalf("op %d stands though its author is not admin there", i)
			}
			if x == y && y == string(creator) && !m.grant(i) {
				standingLeave = true
				if !m.leaves(i) {
					rt.Fatalf("creator left at op %d without having seen another admin", i)
				}
			}
		}
		// The creator leaves only voluntarily, and while it stays there is an admin.
		if !standingLeave && l.Role(creator) != RoleAdmin {
			rt.Fatalf("creator is %v though it never stepped down", l.Role(creator))
		}
	})
}
