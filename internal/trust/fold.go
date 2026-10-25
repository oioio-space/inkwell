package trust

import (
	"bytes"
	"maps"
	"math"
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

// state is the membership produced by a sequence of ops.
type state struct {
	members []Member
}

func (s *state) find(pub []byte) int {
	return slices.IndexFunc(s.members, func(m Member) bool { return bytes.Equal(m.Pub, pub) })
}

func (s *state) admins() (n int) {
	for _, m := range s.members {
		if m.Role == RoleAdmin {
			n++
		}
	}
	return n
}

func (s *state) isAdmin(pub []byte) bool {
	i := s.find(pub)
	return i >= 0 && s.members[i].Role == RoleAdmin
}

// wouldEmptyAdmins reports whether applying tmpl (a Remove or SetRole) would
// take the only admin away.
func (s *state) wouldEmptyAdmins(tmpl Op) bool {
	if tmpl.Kind != OpRemove && (tmpl.Kind != OpSetRole || tmpl.Role == RoleAdmin) {
		return false
	}
	return s.isAdmin(tmpl.Subject) && s.admins() == 1
}

// step applies the membership effect of n. It returns false, leaving the
// state unchanged, only when the op would leave the project without admin;
// ops that do not apply (adding a member, removing a stranger) are no-ops.
func (s *state) step(n *node) bool {
	o := n.op
	i := s.find(o.Subject)
	switch o.Kind {
	case OpGenesis:
		s.members = append(s.members, Member{Pub: o.Subject, Role: RoleAdmin, Person: n.hash})
	case OpAdd:
		if i < 0 {
			person := o.Person
			if person == (Hash{}) {
				person = n.hash
			}
			s.members = append(s.members, Member{Pub: o.Subject, Role: o.Role, Person: person, Name: o.Name})
		}
	case OpSetRole:
		if i < 0 {
			break
		}
		if s.wouldEmptyAdmins(o) {
			return false
		}
		s.members[i].Role = o.Role
		if o.Name != "" {
			s.members[i].Name = o.Name
		}
	case OpRemove:
		if i < 0 {
			break
		}
		if s.wouldEmptyAdmins(o) {
			return false
		}
		s.members = slices.Delete(s.members, i, i+1)
	}
	return true
}

// view is the result of folding a set of ops.
type view struct {
	index map[Hash]int
	nodes []*node // total order
	anc   []bitset
	state state
}

// passResult is the outcome of one sequential replay.
type passResult struct {
	first map[string]int // key → position of the op that first admitted it
	live  []bool         // authorized in its causal past and not voided
	eff   []bool         // live and applied (not blocked by the last-admin rule)
	state state
}

// fold computes the deterministic membership of ops. See the package
// documentation for the rules.
func fold(ops map[Hash]*node) *view {
	nodes := slices.Collect(maps.Values(ops))
	slices.SortFunc(nodes, cmpNodes)
	v := &view{nodes: nodes, index: make(map[Hash]int, len(nodes)), anc: make([]bitset, len(nodes))}
	for i, n := range nodes {
		v.index[n.hash] = i
	}
	for i, n := range nodes {
		v.anc[i] = newBitset(len(nodes))
		for _, p := range n.op.Prev {
			j := v.index[p] // parents have a smaller Lamport, hence j < i.
			v.anc[i].or(v.anc[j])
			v.anc[i].set(j)
		}
	}
	void := newBitset(len(nodes))
	for {
		res := v.pass(void)
		if !v.addVoids(res, void) {
			v.state = res.state
			return v
		}
	}
}

func (v *view) concurrent(i, j int) bool {
	return i != j && !v.anc[i].has(j) && !v.anc[j].has(i)
}

// pass replays every op once in total order, judging each against its causal
// past, and skipping the ops already known to be voided.
func (v *view) pass(void bitset) passResult {
	n := len(v.nodes)
	res := passResult{first: make(map[string]int), live: make([]bool, n), eff: make([]bool, n)}
	for i, nd := range v.nodes {
		if void.has(i) || !v.authorized(i, res.eff) {
			continue
		}
		res.live[i] = true
		known := res.state.find(nd.op.Subject) >= 0
		if !res.state.step(nd) {
			continue
		}
		res.eff[i] = true
		if !known && res.state.find(nd.op.Subject) >= 0 {
			res.first[string(nd.op.Subject)] = i
		}
	}
	return res
}

// authorized reports whether op i's author is an admin in the fold of the
// effective ops of i's causal past.
func (v *view) authorized(i int, eff []bool) bool {
	nd := v.nodes[i]
	if nd.op.Kind == OpGenesis {
		return true
	}
	var past state
	for j := range i {
		if v.anc[i].has(j) && eff[j] {
			past.step(v.nodes[j])
		}
	}
	return past.isAdmin(nd.op.Author)
}

// addVoids adds to void the ops voided by the live Removes of res and reports
// whether void grew.
func (v *view) addVoids(res passResult, void bitset) bool {
	rank := func(key []byte) int {
		if r, ok := res.first[string(key)]; ok {
			return r
		}
		return math.MaxInt
	}
	grew := false
	mark := func(j int) {
		if !void.has(j) {
			void.set(j)
			grew = true
		}
	}
	for i, ri := range v.nodes {
		if !res.live[i] || ri.op.Kind != OpRemove {
			continue
		}
		target := ri.op.Subject
		for j, oj := range v.nodes {
			if !v.concurrent(i, j) {
				continue
			}
			switch {
			case bytes.Equal(oj.op.Author, target):
				crossed := oj.op.Kind == OpRemove && bytes.Equal(oj.op.Subject, ri.op.Author)
				if crossed && rank(target) < rank(ri.op.Author) {
					continue // the older admin's Remove survives.
				}
				mark(j)
			case (oj.op.Kind == OpAdd || oj.op.Kind == OpSetRole) && bytes.Equal(oj.op.Subject, target):
				mark(j)
			}
		}
	}
	return grew
}
