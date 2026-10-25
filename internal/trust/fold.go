package trust

import (
	"container/heap"
	"maps"
	"slices"
)

// bitset is a fixed-size set of op positions.
type bitset []uint64

func newBitset(n int) bitset { return make(bitset, (n+63)/64) }

func (b bitset) has(i int) bool { return b[i/64]>>(i%64)&1 == 1 }

func (b bitset) set(i int) { b[i/64] |= 1 << (i % 64) }

func (b bitset) or(o bitset) {
	for i := range o {
		b[i] |= o[i]
	}
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
	memo     map[int]bool // creator self-cut position → honoured.
	rank     map[string]int
	last     map[string]int // member key → position of its ≺-latest surviving role op.
	memberAt map[string]int // member key → index in members.
	creator  string
	auth     []string // author key of each node.
	subj     []string // subject key of each node.
	nodes    []*node
	anc      []bitset
	stands   []bool
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
		nodes: nodes, index: make(map[Hash]int, n), anc: make([]bitset, n), memo: make(map[int]bool),
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
	v.stands, v.rank = s.stands, s.rank
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
	stands []bool
}

// sweep decides which ops of in (positions, in ≺ order, closed under
// ancestors) stand, visiting keys in rank order.
func (v *view) sweep(in []int) *sweep {
	s := &sweep{v: v, on: make(map[string][]int), by: make(map[string][]int), rank: make(map[string]int), stands: make([]bool, len(v.nodes))}
	for _, i := range in {
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

// decide judges the ops of y, now that every key senior to it is decided.
func (s *sweep) decide(h *rankHeap, y string, cand, surv []int) {
	v := s.v
	cuts := slices.DeleteFunc(slices.Clone(cand), func(i int) bool { return v.nodes[i].grant() })
	var self []int
	for _, i := range s.by[y] {
		nd := v.nodes[i]
		if nd.op.Kind == OpGenesis {
			s.stands[i] = true
			self = append(self, i)
			continue
		}
		if slices.ContainsFunc(cuts, func(k int) bool { return v.conc(i, k) }) {
			continue // rule 3b.
		}
		if !v.adminAt(i, surv, self) {
			continue // rule 3a.
		}
		x := v.subj[i]
		if x == y {
			if y == v.creator && !nd.grant() && !v.creatorMayLeave(i) {
				continue
			}
			self = append(self, i)
		} else if _, senior := s.rank[x]; senior {
			continue // rule 2.
		}
		s.stands[i] = true
		if x != y && nd.grant() {
			heap.Push(h, rankEntry{key: x, rank: i})
		}
	}
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

// creatorMayLeave reports whether the creator's cut of itself at position i is
// honoured: some other admin exists in the membership of i's ancestors alone.
// The answer depends only on those ancestors, a strictly smaller set.
func (v *view) creatorMayLeave(i int) bool {
	if d, ok := v.memo[i]; ok {
		return d
	}
	past := make([]int, 0, i)
	for j := range i {
		if v.anc[i].has(j) {
			past = append(past, j)
		}
	}
	d := v.sweep(past).otherAdmin()
	v.memo[i] = d
	return d
}

// otherAdminSeenBy reports whether an admin other than the creator exists in
// the membership of the ops reachable from heads.
func (v *view) otherAdminSeenBy(heads []Hash) bool {
	seen := newBitset(len(v.nodes))
	for _, h := range heads {
		j := v.index[h]
		seen.or(v.anc[j])
		seen.set(j)
	}
	var past []int
	for j := range v.nodes {
		if seen.has(j) {
			past = append(past, j)
		}
	}
	return v.sweep(past).otherAdmin()
}

// memberRec is a member with the data needed to order and reason about it.
type memberRec struct {
	m    Member
	pos  int // ≺ position that orders the member list.
	last int // position of the ≺-latest surviving role op.
}

func (s *sweep) otherAdmin() bool {
	return slices.ContainsFunc(s.members(), func(r memberRec) bool {
		return r.m.Role == RoleAdmin && string(r.m.Pub) != s.v.creator
	})
}

// members applies rule 5.
func (s *sweep) members() []memberRec {
	v := s.v
	var out []memberRec
	for x, on := range s.on {
		var others, selfs []int
		for _, i := range on {
			switch {
			case !s.stands[i]:
			case v.auth[i] == x:
				selfs = append(selfs, i)
			default:
				others = append(others, i)
			}
		}
		l := slices.Concat(v.survivors(others), selfs)
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
