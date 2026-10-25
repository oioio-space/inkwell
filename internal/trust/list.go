package trust

import (
	"bytes"
	"cmp"
	"crypto/ed25519"
	"errors"
	"maps"
	"math"
	"slices"
	"sync"
)

// Resource bounds. A member list is tiny; these keep a hostile peer from
// growing it, and keep the O(n²) fold cheap.
const (
	maxOps     = 1024
	maxPending = 256
)

// Member is one entry of the folded member list.
type Member struct {
	// Name is the display name chosen by the admin who added the member.
	Name string
	// Pub is the member's key.
	Pub ed25519.PublicKey
	// Person groups the devices of one person.
	Person Hash
	// Role is the member's current role.
	Role Role
}

type node struct {
	op   Op
	hash Hash
}

type pendingOp struct {
	op   Op
	hash Hash
}

func cmpNodes(a, b *node) int {
	return cmp.Or(
		cmp.Compare(a.op.Lamport, b.op.Lamport),
		bytes.Compare(a.op.Author, b.op.Author),
		bytes.Compare(a.hash[:], b.hash[:]),
	)
}

// List is the member list of one project: a hash-DAG of signed ops folded
// into a deterministic membership. The package documentation is the spec of
// the fold. A List is safe for concurrent use.
type List struct {
	ops     map[Hash]*node
	heads   map[Hash]struct{}
	view    *view
	pending []pendingOp
	id      ProjectID
	mu      sync.Mutex
}

// New returns an empty list for the project id. It stays empty until the
// genesis op, whose hash is id, is applied.
func New(id ProjectID) *List {
	return &List{id: id, ops: make(map[Hash]*node), heads: make(map[Hash]struct{})}
}

// Project returns the project the list belongs to.
func (l *List) Project() ProjectID { return l.id }

// Apply adds op to the list. It checks the structure and the signature
// (bound to the project), then the op's parents: if some are unknown the op
// waits in a bounded buffer and is integrated when they arrive. Applying a
// known op is a no-op. Whether an integrated op takes effect is decided by
// the fold, never by the order of arrival.
func (l *List) Apply(op Op) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.apply(op)
	return err
}

// ApplyAll applies ops in turn and returns those that became part of the
// list (including buffered ops they released), plus the joined errors of the
// rejected ones.
func (l *List) ApplyAll(ops []Op) ([]Op, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var added []Op
	var errs []error
	for _, op := range ops {
		a, err := l.apply(op)
		added = append(added, a...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return added, errors.Join(errs...)
}

func (l *List) apply(op Op) ([]Op, error) {
	if err := op.validate(); err != nil {
		return nil, err
	}
	h := op.Hash()
	if l.known(h) {
		return nil, nil
	}
	if op.Kind == OpGenesis && ProjectID(h) != l.id {
		return nil, ErrWrongProject
	}
	if err := op.verify(l.id); err != nil {
		return nil, err
	}
	if len(l.ops) >= maxOps {
		return nil, ErrTooManyOps
	}
	if !l.hasAll(op.Prev) {
		if len(l.pending) >= maxPending {
			l.pending = slices.Delete(l.pending, 0, 1) // evict the oldest: liveness over memory.
		}
		l.pending = append(l.pending, pendingOp{op, h})
		return nil, nil
	}
	if err := l.integrate(op, h); err != nil {
		return nil, err
	}
	added := []Op{op}
	for progress := true; progress; {
		progress = false
		for i := 0; i < len(l.pending); i++ {
			p := l.pending[i]
			if !l.hasAll(p.op.Prev) {
				continue
			}
			l.pending = slices.Delete(l.pending, i, i+1)
			i--
			progress = true
			if len(l.ops) < maxOps && l.integrate(p.op, p.hash) == nil {
				added = append(added, p.op)
			}
		}
	}
	return added, nil
}

func (l *List) known(h Hash) bool {
	if _, ok := l.ops[h]; ok {
		return true
	}
	return slices.ContainsFunc(l.pending, func(p pendingOp) bool { return p.hash == h })
}

func (l *List) hasAll(hs []Hash) bool {
	for _, h := range hs {
		if _, ok := l.ops[h]; !ok {
			return false
		}
	}
	return true
}

// integrate stores op, whose parents are all known. The checks depend only on
// the op's causal past, so every replica reaches the same verdict.
func (l *List) integrate(op Op, h Hash) error {
	if op.Kind != OpGenesis {
		var top uint64
		for _, p := range op.Prev {
			top = max(top, l.ops[p].op.Lamport)
		}
		if top == math.MaxUint64 || op.Lamport != top+1 {
			return ErrBadLamport
		}
		if !l.everAdded(op.Author, op.Prev) {
			return ErrUnknownAuthor
		}
	}
	l.ops[h] = &node{op: op, hash: h}
	for _, p := range op.Prev {
		delete(l.heads, p)
	}
	l.heads[h] = struct{}{}
	l.view = nil
	return nil
}

// everAdded reports whether key created the genesis or is the subject of an
// Add in the causal past formed by from, whether or not that Add took effect.
// It is a cheap, order-independent filter: only keys once invited can make
// the list store their ops.
func (l *List) everAdded(key ed25519.PublicKey, from []Hash) bool {
	seen := make(map[Hash]struct{})
	stack := slices.Clone(from)
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		o := l.ops[h].op
		if (o.Kind == OpGenesis || o.Kind == OpAdd) && bytes.Equal(o.Subject, key) {
			return true
		}
		stack = append(stack, o.Prev...)
	}
	return false
}

// Issue creates, signs and applies an op of the given kind on behalf of s,
// which must currently be an admin. See [List.IssueWith] for the Person field.
func (l *List) Issue(s Signer, k OpKind, subject ed25519.PublicKey, r Role, name string) (Op, error) {
	return l.IssueWith(s, Op{Kind: k, Subject: subject, Role: r, Name: name})
}

// IssueWith is like [List.Issue] with the op's Kind, Subject, Role, Name and
// Person taken from tmpl (Person lets an Add admit another device of an
// existing person). The other fields are filled in. It refuses a Remove or
// demotion of the last admin with [ErrLastAdmin].
func (l *List) IssueWith(s Signer, tmpl Op) (Op, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.ops) == 0 {
		return Op{}, ErrNoGenesis
	}
	if tmpl.Kind == OpGenesis {
		return Op{}, ErrMalformed
	}
	v := l.current()
	me := s.Public()
	if i := v.state.find(me); i < 0 || v.state.members[i].Role != RoleAdmin {
		return Op{}, ErrNotAdmin
	}
	if v.state.wouldEmptyAdmins(tmpl) {
		return Op{}, ErrLastAdmin
	}
	if len(l.heads) > MaxPrev {
		return Op{}, ErrTooManyHeads
	}
	op := tmpl
	op.Author = bytes.Clone(me)
	op.Subject = bytes.Clone(tmpl.Subject)
	op.Prev = l.frontier()
	op.Lamport = 0
	for _, p := range op.Prev {
		op.Lamport = max(op.Lamport, l.ops[p].op.Lamport+1)
	}
	if tmpl.Kind == OpRemove {
		op.Role, op.Person, op.Name = 0, Hash{}, ""
	}
	if err := op.validate(); err != nil {
		return Op{}, err
	}
	op.sign(s, l.id)
	if _, err := l.apply(op); err != nil {
		return Op{}, err
	}
	return op, nil
}

// Role returns the current role of pub, or 0 for a non-member.
func (l *List) Role(pub ed25519.PublicKey) Role {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := &l.current().state
	if i := st.find(pub); i >= 0 {
		return st.members[i].Role
	}
	return 0
}

// Members returns a copy of the current members, in order of admission.
func (l *List) Members() []Member {
	l.mu.Lock()
	defer l.mu.Unlock()
	ms := l.current().state.members
	out := make([]Member, len(ms))
	for i, m := range ms {
		out[i] = m
		out[i].Pub = bytes.Clone(m.Pub)
	}
	return out
}

// Has reports whether the op with hash h is integrated.
func (l *List) Has(h Hash) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.ops[h]
	return ok
}

// Len returns the number of integrated ops.
func (l *List) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.ops)
}

// Pending returns the number of ops waiting for unknown parents.
func (l *List) Pending() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.pending)
}

// Frontier returns the hashes of the ops no other op follows, ascending.
func (l *List) Frontier() []Hash {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.frontier()
}

func (l *List) frontier() []Hash {
	hs := slices.Collect(maps.Keys(l.heads))
	slices.SortFunc(hs, func(a, b Hash) int { return bytes.Compare(a[:], b[:]) })
	return hs
}

// Since returns, in a valid application order, the ops that a replica whose
// frontier is have lacks. Hashes in have that this list does not know are
// ignored, so the answer may then include ops the replica already holds.
func (l *List) Since(have []Hash) []Op {
	l.mu.Lock()
	defer l.mu.Unlock()
	covered := make(map[Hash]struct{})
	stack := slices.Clone(have)
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n, ok := l.ops[h]
		if !ok {
			continue
		}
		if _, dup := covered[h]; dup {
			continue
		}
		covered[h] = struct{}{}
		stack = append(stack, n.op.Prev...)
	}
	var missing []*node
	for h, n := range l.ops {
		if _, ok := covered[h]; !ok {
			missing = append(missing, n)
		}
	}
	slices.SortFunc(missing, cmpNodes)
	out := make([]Op, len(missing))
	for i, n := range missing {
		out[i] = n.op
	}
	return out
}

func (l *List) current() *view {
	if l.view == nil {
		l.view = fold(l.ops)
	}
	return l.view
}
