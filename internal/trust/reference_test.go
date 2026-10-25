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
	memo    map[int]cutInfo
	creator string
	nodes   []*node // in ≺ order.
}

func newRefModel(l *List) *refModel {
	m := &refModel{index: map[Hash]int{}, memo: map[int]cutInfo{}}
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

// isRole reports whether op i is a role op (not a suspension or a resumption).
func (m *refModel) isRole(i int) bool {
	k := m.nodes[i].op.Kind
	return k != OpSuspend && k != OpResume
}

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
	exc     map[int]bool // standing cuts by a junior on a senior.
	rank    map[string]int
	base    map[string][]int
	members []Member
}

// othersOn returns the standing ops on x by other keys, in ≺ order.
func (m *refModel) othersOn(set []int, res refResult, x string) (out []int) {
	for _, i := range set {
		if m.isRole(i) && m.subject(i) == x && m.author(i) != x && res.stands[i] && !res.exc[i] {
			out = append(out, i)
		}
	}
	return out
}

// suspOn returns the standing suspensions and resumptions of x by other keys
// that the keys decided so far made, in ≺ order. With juniors, the cuts of a
// junior key on a senior x count too.
func (m *refModel) suspOn(set []int, res refResult, x string, juniors bool) (out []int) {
	for _, i := range set {
		if !m.isRole(i) && m.subject(i) == x && res.stands[i] && (juniors || !res.exc[i]) {
			out = append(out, i)
		}
	}
	return out
}

// liveSuspension applies S2 to q, ascending ≺ positions of suspensions and
// resumptions of one key: a resumption concurrent to a suspension of q is
// dropped, and the key is suspended if the ≺-last op left is a suspension. It
// returns that op, or -1.
func (m *refModel) liveSuspension(q []int) int {
	last := -1
	for _, i := range q {
		beaten := false
		for _, j := range q {
			beaten = beaten || (m.nodes[i].op.Kind == OpResume && m.nodes[j].op.Kind == OpSuspend && m.concurrent(i, j))
		}
		if !beaten {
			last = i
		}
	}
	if last >= 0 && m.nodes[last].op.Kind == OpSuspend {
		return last
	}
	return -1
}

// suspendedAt reports whether the key with surviving role ops roles and
// standing suspension ops susp is suspended as op i sees it: S2 on what i
// sees, after the last Remove it sees.
func (m *refModel) suspendedAt(i int, roles, susp []int) bool {
	rm := -1
	for _, j := range roles {
		if m.nodes[j].op.Kind == OpRemove && m.sees(i, j) {
			rm = max(rm, j)
		}
	}
	var q []int
	for _, j := range susp {
		if j > rm && m.sees(i, j) {
			q = append(q, j)
		}
	}
	return m.liveSuspension(q) >= 0
}

// spared reports whether some op of set by a key other than y sees i but not
// the exit k of y.
func (m *refModel) spared(set []int, i, k int, y string) bool {
	for _, j := range set {
		if m.author(j) != y && m.sees(j, i) && !m.sees(j, k) {
			return true
		}
	}
	return false
}

func (m *refModel) selfCut(i int) bool { return m.subject(i) == m.author(i) && !m.grant(i) }

// lastIs reports whether the ≺-latest op of l that i sees is a grant.
func (m *refModel) lastIs(i int, l []int) bool {
	last := -1
	for _, j := range l {
		if m.sees(i, j) && j > last {
			last = j
		}
	}
	return last >= 0 && m.grant(last)
}

// fold evaluates the rules on set, a downward-closed list of positions.
func (m *refModel) fold(set []int) refResult {
	res := refResult{stands: map[int]bool{}, exc: map[int]bool{}, rank: map[string]int{}, base: map[string][]int{}}
	decide := func(y string, r int) {
		res.rank[y] = r
		e := m.othersOn(set, res, y)
		surv := m.survivors(e)
		susp := m.suspOn(set, res, y, false)
		// dead is S3: y signs i while suspended, or without having seen a
		// standing suspension of y by another key.
		dead := func(i int) bool {
			for _, k := range susp {
				if m.nodes[k].op.Kind == OpSuspend && m.concurrent(i, k) {
					return true
				}
			}
			return m.suspendedAt(i, surv, susp)
		}
		var exits, self []int
		for _, k := range set { // an exit stands only if no standing cut on y is concurrent to it.
			if m.author(k) != y || !m.selfCut(k) || !m.cutVerdict(k).exit || dead(k) {
				continue
			}
			cut := false
			for _, c := range e {
				cut = cut || (m.level(c) < int(RoleAdmin) && m.concurrent(k, c))
			}
			if !cut {
				exits = append(exits, k)
			}
		}
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
			for _, k := range exits { // 3b: an exit of y concurrent to i.
				if k != i && !slices.Contains(exits, i) && m.concurrent(i, k) && !m.spared(set, i, k, y) {
					killed = true
				}
			}
			if killed || dead(i) || !m.lastIs(i, slices.Concat(surv, self)) { // 3a, S3.
				continue
			}
			if x := m.subject(i); x == y {
				if m.selfCut(i) && !m.cutVerdict(i).honoured {
					continue
				}
				self = append(self, i)
			} else if _, senior := res.rank[x]; senior { // 2.
				if !m.excCut(i) || m.lastIs(i, res.base[x]) {
					continue
				}
				res.exc[i] = true
			}
			res.stands[i] = true
		}
		res.base[y] = slices.Concat(surv, self)
		slices.Sort(res.base[y])
	}
	decide(m.creator, 0)
	for { // 1: the next rank is the smallest pos of a surviving standing grant.
		best, bestPos := "", -1
		for _, i := range set {
			y := m.subject(i)
			if _, done := res.rank[y]; done {
				continue
			}
			for _, g := range m.survivors(m.othersOn(set, res, y)) {
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
		var selfs, juniors []int
		for _, j := range set {
			if m.isRole(j) && m.subject(j) == x && res.stands[j] {
				switch {
				case m.author(j) == x:
					selfs = append(selfs, j)
				case res.exc[j]:
					juniors = append(juniors, j)
				}
			}
		}
		surv := m.survivors(m.othersOn(set, res, x))
		l := slices.Concat(surv, selfs)
		for _, c := range juniors {
			beaten := false
			for _, g := range surv {
				beaten = beaten || (m.grant(g) && m.concurrent(c, g))
			}
			if !beaten {
				l = append(l, c)
			}
		}
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
			rm := -1 // the last surviving Remove of x by another key.
			for _, j := range l {
				if m.nodes[j].op.Kind == OpRemove && m.author(j) != x {
					rm = j
				}
			}
			var q []int
			for _, j := range m.suspOn(set, res, x, true) {
				if j > rm {
					q = append(q, j)
				}
			}
			if k := m.liveSuspension(q); k >= 0 {
				mem.Suspended, mem.SuspendedBy = true, m.nodes[k].op.Author
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

func (m *refModel) excCut(i int) bool { return m.nodes[i].op.excCut() }

// cutVerdict judges the self-cut i on the fold of its ancestors alone.
func (m *refModel) cutVerdict(i int) cutInfo {
	if c, ok := m.memo[i]; ok {
		return c
	}
	var past []int
	for j := range i {
		if m.sees(i, j) {
			past = append(past, j)
		}
	}
	var c cutInfo
	mine := false
	for _, x := range m.fold(past).members {
		if x.Role == RoleAdmin {
			if string(x.Pub) == m.author(i) {
				mine = true
			} else {
				c.honoured = true
			}
		}
	}
	c.exit = c.honoured && mine
	m.memo[i] = c
	return c
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

		// regular returns the ops of y's role history that its seniors and itself made.
		regular := func(y string) (others, selfs []int) {
			for j, ok := range v.stands {
				switch {
				case !ok || m.subject(j) != y || !m.isRole(j):
				case m.author(j) == y:
					selfs = append(selfs, j)
				case !v.exc[j]:
					others = append(others, j)
				}
			}
			return others, selfs
		}
		res := refResult{stands: map[int]bool{}, exc: v.exc}
		for j, ok := range v.stands {
			res.stands[j] = ok
		}
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
		creatorLeft := false
		for i, ok := range v.stands {
			if !ok {
				continue
			}
			y, x := m.author(i), m.subject(i)
			others, selfs := regular(y)
			if x != y {
				ry, ranked := v.rank[y]
				if !ranked {
					rt.Fatalf("op %d by the unranked key %x stands", i, y)
				}
				if rx, senior := v.rank[x]; senior && rx < ry {
					// Acting on a senior is a cut of a key that is no longer admin.
					xo, xs := regular(x)
					if !m.excCut(i) || m.lastIs(i, slices.Concat(m.survivors(xo), xs)) {
						rt.Fatalf("op %d by rank %d acts on admin or by grant on senior rank %d", i, ry, rx)
					}
				}
			}
			if m.nodes[i].op.Kind == OpGenesis {
				continue
			}
			for _, j := range others {
				if m.level(j) < int(RoleAdmin) && m.concurrent(i, j) {
					rt.Fatalf("op %d stands though concurrent to the standing cut %d on its author", i, j)
				}
			}
			iExits := m.selfCut(i) && m.cutVerdict(i).exit
			for j, ok := range v.stands {
				if ok && j != i && m.author(j) == y && m.selfCut(j) && m.cutVerdict(j).exit && !iExits && m.concurrent(i, j) && !m.spared(m.all(), i, j, y) {
					rt.Fatalf("op %d stands though concurrent to its author's exit %d", i, j)
				}
			}
			if !m.lastIs(i, slices.Concat(m.survivors(others), selfs)) {
				rt.Fatalf("op %d stands though its author is not admin there", i)
			}
			susp := m.suspOn(m.all(), res, y, false)
			for _, k := range susp {
				if m.nodes[k].op.Kind == OpSuspend && m.concurrent(i, k) {
					rt.Fatalf("op %d stands though concurrent to the standing suspension %d of its author", i, k)
				}
			}
			if m.suspendedAt(i, m.survivors(others), susp) {
				rt.Fatalf("op %d stands though its author is suspended there", i)
			}
			if m.selfCut(i) {
				if !m.cutVerdict(i).honoured {
					rt.Fatalf("op %d left without having seen another admin", i)
				}
				if y == string(creator) {
					creatorLeft = true
				}
			}
		}
		// While the creator has not stepped down it is an admin.
		if !creatorLeft && l.Role(creator) != RoleAdmin {
			rt.Fatalf("creator is %v though it never stepped down", l.Role(creator))
		}
	})
}
