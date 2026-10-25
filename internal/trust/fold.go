package trust

import (
	"container/heap"
	"maps"
	"math/bits"
	"slices"
)

// bitset is a fixed-size set of op positions.
type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }

func (b bitset) has(i int) bool { return b[i/64]>>(i%64)&1 == 1 }

func (b bitset) set(i int) { b[i/64] |= 1 << (i % 64) }

func (b bitset) unset(i int) { b[i/64] &^= 1 << (i % 64) }

func (b bitset) or(o bitset) {
	for i := range o {
		b[i] |= o[i]
	}
}

func (b bitset) andNot(o bitset) {
	for i := range o {
		b[i] &^= o[i]
	}
}

// positions returns the members of b, ascending.
func (b bitset) positions() []int {
	n := 0
	for _, w := range b {
		n += bits.OnesCount64(w)
	}
	out := make([]int, 0, n)
	for i, w := range b {
		for ; w != 0; w &= w - 1 {
			out = append(out, i*64+bits.TrailingZeros64(w))
		}
	}
	return out
}

// level orders the effects of a role op: Remove < reader < writer < admin.
func (n *node) level() int {
	if n.op.Kind == OpRemove {
		return 0
	}
	return int(n.op.Role)
}

// grant reports whether the op makes its subject an admin.
func (n *node) grant() bool { return n.level() == int(RoleAdmin) }

// view is the result of folding the stored ops. Positions are indexes into
// nodes, which is sorted by ≺.
type view struct {
	index    map[Hash]int
	memo     map[int]cutInfo // self-cut position → verdict on its own ancestors.
	rank     map[string]int
	last     map[string]int // member key → position of its ≺-latest surviving role op.
	memberAt map[string]int // member key → index in members.
	creator  string
	auth     []string // author key of each node.
	subj     []string // subject key of each node.
	nodes    []*node
	anc      []bitset
	stands   []bool
	exc      map[int]bool // standing junior cuts on seniors.
	members  []Member
}

// rankEntry is a key waiting in the sweep's priority queue.
type rankEntry struct {
	key  string
	rank int
}

type rankHeap []rankEntry

func (h rankHeap) Len() int           { return len(h) }
func (h rankHeap) Less(i, j int) bool { return h[i].rank < h[j].rank }
func (h rankHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *rankHeap) Push(x any)        { *h = append(*h, x.(rankEntry)) }
func (h *rankHeap) Pop() any {
	old := *h
	e := old[len(old)-1]
	*h = old[:len(old)-1]
	return e
}

// fold computes the deterministic membership of ops. See the package
// documentation for the rules.
func fold(ops map[Hash]*node) *view {
	nodes := slices.Collect(maps.Values(ops))
	slices.SortFunc(nodes, cmpNodes)
	n := len(nodes)
	v := &view{
		nodes: nodes, index: make(map[Hash]int, n), anc: make([]bitset, n), memo: make(map[int]cutInfo),
		auth: make([]string, n), subj: make([]string, n),
		rank: make(map[string]int), last: make(map[string]int), memberAt: make(map[string]int),
	}
	if n == 0 {
		return v
	}
	for i, nd := range nodes {
		v.index[nd.hash] = i
		v.auth[i], v.subj[i] = string(nd.op.Author), string(nd.op.Subject)
	}
	for i, nd := range nodes {
		v.anc[i] = newBitset(n)
		for _, p := range nd.op.Prev {
			j := v.index[p] // parents have a smaller Lamport, hence j < i.
			v.anc[i].or(v.anc[j])
			v.anc[i].set(j)
		}
	}
	v.creator = v.auth[0] // the genesis is the only op of Lamport 0.
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	s := v.sweep(all)
	v.stands, v.rank, v.exc = s.stands, s.rank, s.exc
	recs := s.members()
	slices.SortFunc(recs, func(a, b memberRec) int { return a.pos - b.pos })
	for _, r := range recs {
		v.memberAt[string(r.m.Pub)] = len(v.members)
		v.last[string(r.m.Pub)] = r.last
		v.members = append(v.members, r.m)
	}
	return v
}

func (v *view) conc(i, j int) bool { return i != j && !v.anc[i].has(j) && !v.anc[j].has(i) }

// survivors drops from e (standing role ops on one key by other keys, in ≺
// order) those with a concurrent op of a lower level: a revocation beats a
// concurrent grant.
func (v *view) survivors(e []int) []int {
	var out []int
	for _, i := range e {
		if !slices.ContainsFunc(e, func(j int) bool { return v.conc(i, j) && v.nodes[j].level() < v.nodes[i].level() }) {
			out = append(out, i)
		}
	}
	return out
}

// sweep is one evaluation of the fold rules over a downward-closed set of ops.
type sweep struct {
	v      *view
	on     map[string][]int // subject → role ops on it, in ≺ order.
	by     map[string][]int // author → ops by it, in ≺ order.
	rank   map[string]int
	base   map[string][]int // decided key → its surviving role ops, junior cuts excluded.
	stands []bool
	exc    map[int]bool // standing cuts by a junior on a senior (rule 2's exception).
	inSet  bitset       // the positions of the ops being swept.
}

// sweep decides which ops of in (positions, in ≺ order, closed under
// ancestors) stand, visiting keys in rank order.
func (v *view) sweep(in []int) *sweep {
	s := &sweep{
		v: v, on: make(map[string][]int), by: make(map[string][]int), rank: make(map[string]int),
		base: make(map[string][]int), stands: make([]bool, len(v.nodes)), exc: make(map[int]bool),
	}
	s.inSet = newBitset(len(v.nodes))
	for _, i := range in {
		s.inSet.set(i)
		s.on[v.subj[i]] = append(s.on[v.subj[i]], i)
		s.by[v.auth[i]] = append(s.by[v.auth[i]], i)
	}
	h := &rankHeap{{key: v.creator}}
	for h.Len() > 0 {
		e := heap.Pop(h).(rankEntry)
		y := e.key
		if _, done := s.rank[y]; done {
			continue
		}
		var cand []int // standing ops on y by other keys, all by senior keys.
		for _, i := range s.on[y] {
			if v.auth[i] != y && s.stands[i] {
				cand = append(cand, i)
			}
		}
		surv := v.survivors(cand)
		if y != v.creator {
			k := slices.IndexFunc(surv, func(i int) bool { return v.nodes[i].grant() })
			if k < 0 {
				continue // never ranked, for now.
			}
			if g := surv[k]; g > e.rank {
				heap.Push(h, rankEntry{key: y, rank: g}) // its earliest grant was dropped.
				continue
			}
		}
		s.rank[y] = e.rank
		s.decide(h, y, cand, surv)
	}
	return s
}

// isSelfCut reports whether op i removes or demotes its own author.
func (v *view) isSelfCut(i int) bool {
	return v.subj[i] == v.auth[i] && !v.nodes[i].grant()
}

// cutInfo is the verdict on a self-cut, a function of its ancestors only.
type cutInfo struct {
	honoured bool // some admin other than the author exists in the ancestors' membership.
	exit     bool // honoured, and the author is an admin there too.
}

// selfCut judges the self-cut at position k on the membership of its
// ancestors alone, a strictly smaller set.
func (v *view) selfCut(k int) cutInfo {
	if c, ok := v.memo[k]; ok {
		return c
	}
	c := v.verdict(v.sweep(v.anc[k].positions()), v.auth[k])
	v.memo[k] = c
	return c
}

// verdict reads the admins of s as seen by the key y.
func (v *view) verdict(s *sweep, y string) (c cutInfo) {
	mine := false
	for _, r := range s.members() {
		if r.m.Role != RoleAdmin {
			continue
		}
		if string(r.m.Pub) == y {
			mine = true
		} else {
			c.honoured = true
		}
	}
	c.exit = c.honoured && mine
	return c
}

// excCut reports whether a role op may target a senior key that is no longer
// admin: a Remove, or a SetRole or Add to a lower role that renames nobody
// (and, for an Add, regroups no person). A grant never may.
func (o Op) excCut() bool {
	switch o.Kind {
	case OpRemove:
		return true
	case OpSetRole:
		return o.Role < RoleAdmin && o.Name == ""
	case OpAdd:
		return o.Role < RoleAdmin && o.Name == "" && o.Person == (Hash{})
	}
	return false
}

// decide judges the ops of y, now that every key senior to it is decided.
func (s *sweep) decide(h *rankHeap, y string, cand, surv []int) {
	v := s.v
	cuts := slices.DeleteFunc(slices.Clone(cand), func(i int) bool { return v.nodes[i].grant() })
	var exits []int // y's self-cuts that stand: they leave another admin in place and no cut by another key is concurrent to them.
	for _, k := range s.by[y] {
		if v.isSelfCut(k) && v.selfCut(k).exit && !slices.ContainsFunc(cuts, func(c int) bool { return v.conc(k, c) }) {
			exits = append(exits, k)
		}
	}
	var self []int
	for _, i := range s.by[y] {
		nd := v.nodes[i]
		if nd.op.Kind == OpGenesis {
			s.stands[i] = true
			self = append(self, i)
			continue
		}
		if slices.ContainsFunc(cuts, func(k int) bool { return v.conc(i, k) }) {
			continue // rule 3b, cut by another key.
		}
		if !slices.Contains(exits, i) && slices.ContainsFunc(exits, func(k int) bool { return v.conc(i, k) && !s.spared(i, k, y) }) {
			continue // rule 3b, y's own exit.
		}
		if !v.adminAt(i, surv, self) {
			continue // rule 3a.
		}
		x := v.subj[i]
		if x == y {
			if v.isSelfCut(i) && !v.selfCut(i).honoured {
				continue
			}
			self = append(self, i)
		} else if _, senior := s.rank[x]; senior {
			if !nd.op.excCut() || v.adminAt(i, s.base[x], nil) {
				continue // rule 2.
			}
			s.exc[i] = true
		}
		s.stands[i] = true
		if x != y && nd.grant() {
			heap.Push(h, rankEntry{key: x, rank: i})
		}
	}
	s.base[y] = slices.Concat(surv, self)
	slices.Sort(s.base[y])
}

// adminAt reports whether the ≺-latest of the ops of surv and self that op i
// sees is a grant.
func (v *view) adminAt(i int, surv, self []int) bool {
	last := -1
	for _, l := range [][]int{surv, self} {
		for _, j := range l {
			if v.anc[i].has(j) {
				last = max(last, j)
			}
		}
	}
	return last >= 0 && v.nodes[last].grant()
}

// otherAdminSeenBy reports whether an admin other than y exists in the
// membership of the ops reachable from heads.
func (v *view) otherAdminSeenBy(heads []Hash, y string) bool {
	seen := newBitset(len(v.nodes))
	for _, h := range heads {
		j := v.index[h]
		seen.or(v.anc[j])
		seen.set(j)
	}
	return v.verdict(v.sweep(seen.positions()), y).honoured
}

// memberRec is a member with the data needed to order and reason about it.
type memberRec struct {
	m    Member
	pos  int // ≺ position that orders the member list.
	last int // position of the ≺-latest surviving role op.
}

// members applies rule 5.
func (s *sweep) members() []memberRec {
	v := s.v
	var out []memberRec
	for x, on := range s.on {
		var others, selfs, juniors []int
		for _, i := range on {
			switch {
			case !s.stands[i]:
			case v.auth[i] == x:
				selfs = append(selfs, i)
			case s.exc[i]:
				juniors = append(juniors, i)
			default:
				others = append(others, i)
			}
		}
		surv := v.survivors(others)
		l := slices.Concat(surv, selfs)
		for _, c := range juniors { // a senior's concurrent grant beats a junior's cut.
			if !slices.ContainsFunc(surv, func(g int) bool { return v.nodes[g].grant() && v.conc(c, g) }) {
				l = append(l, c)
			}
		}
		slices.Sort(l)
		if len(l) == 0 || v.nodes[l[len(l)-1]].op.Kind == OpRemove {
			continue
		}
		// The run is what follows the latest surviving Remove.
		run := l
		for k := len(l) - 1; k >= 0; k-- {
			if v.nodes[l[k]].op.Kind == OpRemove {
				run = l[k+1:]
				break
			}
		}
		last := v.nodes[run[len(run)-1]]
		m := Member{Pub: last.op.Subject, Role: last.op.Role}
		pos := run[0]
		person := v.nodes[run[0]].hash
		for _, i := range run {
			nd := v.nodes[i]
			if nd.op.Kind != OpGenesis && nd.op.Kind != OpRemove && nd.op.Name != "" {
				m.Name = nd.op.Name
			}
			if nd.op.Kind == OpAdd {
				pos, person = i, nd.op.Person
				if person == (Hash{}) {
					person = nd.hash
				}
			}
		}
		m.Person = person
		if x == v.creator {
			pos = 0
		}
		out = append(out, memberRec{m: m, pos: pos, last: run[len(run)-1]})
	}
	return out
}

// spared reports whether y's exit k spares its concurrent op i: some op by
// another key in the swept set sees i but not k, so the others had accepted i
// before y's exit was in play, and a backdated exit cannot take it back.
func (s *sweep) spared(i, k int, y string) bool {
	v := s.v
	for j := i + 1; j < len(v.nodes); j++ {
		if s.inSet.has(j) && v.auth[j] != y && v.anc[j].has(i) && !v.anc[j].has(k) {
			return true
		}
	}
	return false
}
