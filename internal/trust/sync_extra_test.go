package trust

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/transport"
)

// opsMessage is an Ops sync message carrying ops, as a peer would send it.
func opsMessage(t testing.TB, ops ...Op) []byte {
	t.Helper()
	out := chunk("x", ops)
	if len(out) != 1 {
		t.Fatalf("chunk() = %d messages, want 1", len(out))
	}
	return out[0].Data
}

// destinations returns the peers of out, in order.
func destinations(out []Outbound) []transport.PeerID {
	var to []transport.PeerID
	for _, o := range out {
		to = append(to, o.To)
	}
	return to
}

// decodedOps returns the ops carried by the Ops message m.
func decodedOps(t testing.TB, m []byte) []Op {
	t.Helper()
	kind, _, ops, err := decodeMessage(m)
	if err != nil || kind != msgOps {
		t.Fatalf("decodeMessage() = kind %d, %v; want an Ops message", kind, err)
	}
	return ops
}

// syncRig is a Sync over a project of a, with peers p and q up.
type syncRig struct {
	sync *Sync
	list *List
	src  *List // a replica that issues the ops the rig then receives
	a, b Signer
}

func newSyncRig(t *testing.T) *syncRig {
	t.Helper()
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	r := &syncRig{sync: NewSync(l), list: l, src: fork(t, l), a: a, b: b}
	r.sync.OnPeerUp("p")
	r.sync.OnPeerUp("q")
	return r
}

// TestSyncAnnouncesFrontier: a peer coming up gets exactly one Have with our
// frontier, and none when the list is sealed.
func TestSyncAnnouncesFrontier(t *testing.T) {
	a := newSigner(1)
	l := project(t, a)
	s := NewSync(l)
	out := s.OnPeerUp("p")
	if len(out) != 1 || out[0].To != "p" {
		t.Fatalf("OnPeerUp = %v, want one message to p", destinations(out))
	}
	kind, have, _, err := decodeMessage(out[0].Data)
	if err != nil || kind != msgHave || !slices.Equal(have, l.Frontier()) {
		t.Errorf("announcement = kind %d, %d hashes, %v; want a Have with the frontier", kind, len(have), err)
	}

	sealed := project(t, a)
	if err := chain(t, sealed, a, maxOps+1); !errors.Is(err, ErrTooManyOps) {
		t.Fatalf("sealing the list: %v", err)
	}
	s = NewSync(sealed)
	if out := s.OnPeerUp("p"); len(out) != 0 {
		t.Errorf("a sealed list announced itself: %d messages", len(out))
	}
	if out := s.Tick(); len(out) != 0 {
		t.Errorf("a sealed list announced itself on Tick: %d messages", len(out))
	}
}

// TestSyncRelaysToEveryoneButTheSender: an op that arrives is forwarded to the
// other peers, never echoed back, and only once.
func TestSyncRelaysToEveryoneButTheSender(t *testing.T) {
	r := newSyncRig(t)
	r.sync.OnPeerUp("s")
	op := mustIssue(t, r.src, r.a, OpAdd, r.b, RoleWriter)

	out, err := r.sync.OnMessage("q", opsMessage(t, op))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := destinations(out), []transport.PeerID{"p", "s"}; !slices.Equal(got, want) {
		t.Fatalf("relayed to %v, want %v (sorted, without the sender q)", got, want)
	}
	for _, o := range out {
		if ops := decodedOps(t, o.Data); len(ops) != 1 || ops[0].Hash() != op.Hash() {
			t.Errorf("relay to %s carried %d ops, want exactly the op", o.To, len(ops))
		}
	}
	if !r.list.Has(op.Hash()) {
		t.Error("the relayed op was not applied")
	}

	// The same op again, from another peer: already known, nothing to relay.
	out, err = r.sync.OnMessage("p", opsMessage(t, op))
	if err != nil || len(out) != 0 {
		t.Errorf("duplicate op = %d messages, %v; want none", len(out), err)
	}
}

func TestSyncBroadcastAndPeerDown(t *testing.T) {
	r := newSyncRig(t)
	op := mustIssue(t, r.src, r.a, OpAdd, r.b, RoleWriter)
	if got := destinations(r.sync.Broadcast(op)); !slices.Equal(got, []transport.PeerID{"p", "q"}) {
		t.Errorf("Broadcast to %v, want p, q", got)
	}
	r.sync.OnPeerDown("q")
	if got := destinations(r.sync.Broadcast(op)); !slices.Equal(got, []transport.PeerID{"p"}) {
		t.Errorf("Broadcast after q went down to %v, want p", got)
	}
	if out := r.sync.Broadcast(); len(out) != 0 {
		t.Errorf("Broadcast() of nothing = %d messages", len(out))
	}
	if got := destinations(r.sync.Tick()); !slices.Equal(got, []transport.PeerID{"p"}) {
		t.Errorf("Tick announced to %v, want only p", got)
	}
}

// TestSyncAnswersHaveWithinBudget: a peer asking again and again is answered
// twice per Tick, with what it lacks, and the budget is renewed by Tick.
func TestSyncAnswersHaveWithinBudget(t *testing.T) {
	r := newSyncRig(t)
	op := mustIssue(t, r.src, r.a, OpAdd, r.b, RoleWriter)
	mustApplyAll(t, r.list, []Op{op})
	empty := []byte{msgHave, 0, 0}

	answers := func() int {
		t.Helper()
		out, err := r.sync.OnMessage("p", empty)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) == 0 {
			return 0
		}
		if out[0].To != "p" {
			t.Errorf("answered %s, want the asker p", out[0].To)
		}
		if n := len(decodedOps(t, out[0].Data)); n != 2 {
			t.Errorf("answer carried %d ops, want the 2 the peer lacks", n)
		}
		return len(out)
	}
	first, second := answers(), answers()
	if first == 0 || second == 0 {
		t.Fatal("the first two Haves were not answered")
	}
	if answers() != 0 {
		t.Error("the third Have in the same Tick was answered")
	}
	// Another peer has its own budget.
	if out, _ := r.sync.OnMessage("q", empty); len(out) == 0 {
		t.Error("q's Have was refused because of p's budget")
	}
	r.sync.Tick()
	if answers() == 0 {
		t.Error("the budget was not renewed by Tick")
	}

	// A peer that has everything gets nothing back.
	have := []byte{msgHave, 0, 1}
	have = append(have, op.Hash().bytes()...)
	r.sync.Tick()
	if out, err := r.sync.OnMessage("p", have); err != nil || len(out) != 0 {
		t.Errorf("Have of the whole frontier = %d messages, %v; want none", len(out), err)
	}
}

func (h Hash) bytes() []byte { return h[:] }

// TestSyncOnMessageErrors: only malformed input is an error; the honest
// failures of out-of-order delivery are not.
func TestSyncOnMessageErrors(t *testing.T) {
	r := newSyncRig(t)
	x := mustIssue(t, r.src, r.a, OpAdd, r.b, RoleWriter)
	y := mustIssue(t, r.src, r.a, OpSetRole, r.b, RoleReader)

	// y without x: buffered, no error, nothing relayed yet.
	if out, err := r.sync.OnMessage("p", opsMessage(t, y)); err != nil || len(out) != 0 {
		t.Errorf("orphan = %d messages, %v; want none, nil", len(out), err)
	}
	// x releases y: both are relayed to q, in causal order.
	out, err := r.sync.OnMessage("p", opsMessage(t, x))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].To != "q" {
		t.Fatalf("relay = %v, want one message to q", destinations(out))
	}
	if ops := decodedOps(t, out[0].Data); len(ops) != 2 || ops[0].Hash() != x.Hash() || ops[1].Hash() != y.Hash() {
		t.Errorf("relay carried %d ops, want x then y", len(ops))
	}

	// A stranger's op: refused without being an error, not stored, not relayed.
	stranger := newSigner(9)
	s := Op{Kind: OpAdd, Role: RoleAdmin, Subject: stranger.Public(), Author: stranger.Public(), Prev: r.list.Frontier(), Lamport: 9}
	s.sign(stranger, r.list.Project())
	if out, err := r.sync.OnMessage("p", opsMessage(t, s)); err != nil || len(out) != 0 || r.list.Has(s.Hash()) {
		t.Errorf("stranger op = %d messages, %v, stored %t; want none, nil, false", len(out), err, r.list.Has(s.Hash()))
	}

	// Garbage is an error.
	for name, msg := range map[string][]byte{
		"empty":        nil,
		"unknown kind": {9, 0, 0},
		"trailing":     {msgHave, 0, 0, 1},
		"short hashes": {msgHave, 0, 2, 1, 2, 3},
		"short ops":    {msgOps, 0, 1},
	} {
		if _, err := r.sync.OnMessage("p", msg); !errors.Is(err, ErrMalformed) {
			t.Errorf("OnMessage(%s) = %v, want ErrMalformed", name, err)
		}
	}
}

func TestDecodeMessageLimits(t *testing.T) {
	have := func(n int) []byte {
		b := binary.BigEndian.AppendUint16([]byte{msgHave}, uint16(n))
		return append(b, bytes.Repeat([]byte{7}, n*len(Hash{}))...)
	}
	if _, h, _, err := decodeMessage(have(maxHave)); err != nil || len(h) != maxHave {
		t.Errorf("Have of %d hashes = %d, %v; want accepted", maxHave, len(h), err)
	}
	if _, _, _, err := decodeMessage(have(maxHave + 1)); !errors.Is(err, ErrMalformed) {
		t.Errorf("Have of %d hashes = %v, want ErrMalformed", maxHave+1, err)
	}
	if _, _, _, err := decodeMessage(append(have(1), make([]byte, MaxMessage)...)); !errors.Is(err, ErrMalformed) {
		t.Errorf("message over MaxMessage = %v, want ErrMalformed", err)
	}
	if maxHave*len(Hash{})+3 > MaxMessage {
		t.Error("a maximal Have does not fit MaxMessage")
	}
}

// sizedOp returns a valid op, unsigned, whose encoding is size bytes long,
// made of parents (32 bytes each) and a name for the remainder.
func sizedOp(t *testing.T, size int) Op {
	t.Helper()
	o := baseOp()
	extra := size - len(Encode(o))
	if extra < 0 || extra/len(Hash{}) > MaxPrev-1 {
		t.Fatalf("cannot size an op to %d bytes", size)
	}
	o.Prev = hashes(1 + extra/len(Hash{}))
	o.Name = strings.Repeat("n", extra%len(Hash{}))
	if got := len(Encode(o)); got != size {
		t.Fatalf("sizedOp(%d) encodes to %d bytes", size, got)
	}
	if err := o.validate(); err != nil {
		t.Fatalf("sizedOp(%d) is invalid: %v", size, err)
	}
	return o
}

// TestChunkBoundaries: ops are packed into messages of chunkBytes of content
// exactly; one byte more starts a new message. Order and content survive.
func TestChunkBoundaries(t *testing.T) {
	const opSize = 400
	perChunk := chunkBytes / (opSize + 2)
	fill := chunkBytes - perChunk*(opSize+2) // bytes left to give to the last op of the chunk

	var ops []Op
	for i := range perChunk - 1 {
		o := sizedOp(t, opSize)
		o.Lamport = uint64(i + 1) // tells the ops apart
		ops = append(ops, o)
	}
	exact := sizedOp(t, opSize+fill)
	ops = append(ops, exact)

	out := chunk("p", ops)
	if len(out) != 1 {
		t.Fatalf("%d ops filling exactly %d bytes gave %d messages, want 1", len(ops), chunkBytes, len(out))
	}
	if got := decodedOps(t, out[0].Data); len(got) != len(ops) {
		t.Errorf("decoded %d ops, want %d", len(got), len(ops))
	}
	if len(out[0].Data) > MaxMessage {
		t.Errorf("message of %d bytes exceeds MaxMessage", len(out[0].Data))
	}

	ops[len(ops)-1] = sizedOp(t, opSize+fill+1)
	out = chunk("p", ops)
	if len(out) != 2 {
		t.Fatalf("one byte over gave %d messages, want 2", len(out))
	}
	first, second := decodedOps(t, out[0].Data), decodedOps(t, out[1].Data)
	if len(first) != len(ops)-1 || len(second) != 1 {
		t.Errorf("split %d + %d ops, want %d + 1", len(first), len(second), len(ops)-1)
	}
	for i, o := range append(first, second...) {
		if o.Name != ops[i].Name || o.Lamport != ops[i].Lamport {
			t.Errorf("op %d changed or reordered by chunking", i)
		}
	}
	for _, o := range out {
		if o.To != "p" {
			t.Errorf("chunk addressed to %s, want p", o.To)
		}
	}
	if got := chunk("p", nil); len(got) != 0 {
		t.Errorf("chunk(nil) = %d messages, want none", len(got))
	}
}
