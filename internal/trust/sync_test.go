package trust

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// replica is one peer: a list, its Sync and its memnet node. The mutex stands
// in for the single event loop a real caller has.
type replica struct {
	node *memnet.Node
	list *List
	sync *Sync
	mu   sync.Mutex
}

func (r *replica) send(ctx context.Context, out []Outbound) {
	for _, o := range out {
		_ = r.node.Send(ctx, o.To, o.Data) // loss is part of the test.
	}
}

func (r *replica) run(ctx context.Context) {
	for {
		ev, err := r.node.Recv(ctx)
		if err != nil {
			return
		}
		r.mu.Lock()
		var out []Outbound
		switch ev.Kind {
		case transport.PeerUp:
			out = r.sync.OnPeerUp(ev.Peer)
		case transport.PeerDown:
			r.sync.OnPeerDown(ev.Peer)
		case transport.Message:
			out, _ = r.sync.OnMessage(ev.Peer, ev.Payload)
		}
		r.mu.Unlock()
		r.send(ctx, out)
	}
}

func (r *replica) tick(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
		r.mu.Lock()
		out := r.sync.Tick()
		r.mu.Unlock()
		r.send(ctx, out)
	}
}

func (r *replica) issue(ctx context.Context, t *testing.T, by Signer, k OpKind, subject Signer, role Role) {
	t.Helper()
	r.mu.Lock()
	op, err := r.list.Issue(by, k, subject.Public(), role, "")
	var out []Outbound
	if err == nil {
		out = r.sync.Broadcast(op)
	}
	r.mu.Unlock()
	if err != nil {
		t.Errorf("Issue(kind %d) = %v", k, err)
	}
	r.send(ctx, out)
}

func TestSyncConverges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b, c, d, e := newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(5)
		root := project(t, a)
		net := memnet.New(42, memnet.Faults{Drop: 0.3, Duplicate: 0.2, MaxDelay: 200 * time.Millisecond})

		reps := map[transport.PeerID]*replica{}
		for id, l := range map[transport.PeerID]*List{"a": root, "b": New(root.Project()), "c": New(root.Project())} {
			reps[id] = &replica{node: net.Join(id), list: l, sync: NewSync(l)}
		}
		ra, rb := reps["a"], reps["b"]

		ctx, cancel := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		for _, r := range reps {
			wg.Go(func() { r.run(ctx) })
			wg.Go(func() { r.tick(ctx) })
		}

		net.Link("a", "b") // a line: a and c never talk directly.
		net.Link("b", "c")
		ra.issue(ctx, t, a, OpAdd, b, RoleAdmin)
		ra.issue(ctx, t, a, OpAdd, c, RoleWriter)
		time.Sleep(20 * time.Second)

		// Partition a from b; both sides edit concurrently, including a
		// revocation by a that races b's invitation.
		net.Partition("a", "b")
		time.Sleep(time.Second)
		ra.issue(ctx, t, a, OpRemove, c, 0)
		ra.issue(ctx, t, a, OpAdd, d, RoleReader)
		rb.issue(ctx, t, b, OpAdd, e, RoleWriter)
		rb.issue(ctx, t, b, OpSetRole, c, RoleAdmin)
		time.Sleep(10 * time.Second)
		net.Link("a", "b")
		time.Sleep(40 * time.Second)

		cancel()
		wg.Wait()

		want := ra.list.Members()
		for id, r := range reps {
			if got := r.list.Members(); cmp.Diff(want, got) != "" {
				t.Errorf("replica %s members differ (-a +%s):\n%s", id, id, cmp.Diff(want, got))
			}
			if diff := cmp.Diff(ra.list.Frontier(), r.list.Frontier()); diff != "" {
				t.Errorf("replica %s frontier differs:\n%s", id, diff)
			}
			if r.list.Pending() != 0 {
				t.Errorf("replica %s has %d pending ops", id, r.list.Pending())
			}
		}
		if ra.list.Len() != 7 {
			t.Errorf("Len() = %d, want 7 ops", ra.list.Len())
		}
		wantRole(t, ra.list, c, 0) // a's revocation beats b's concurrent SetRole.
		wantRole(t, ra.list, d, RoleReader)
		wantRole(t, ra.list, e, RoleWriter)
	})
}

func TestSyncRejectsGarbage(t *testing.T) {
	a := newSigner(1)
	s := NewSync(project(t, a))
	for _, msg := range [][]byte{nil, {9, 0, 0}, {msgHave, 0xff, 0xff}, {msgOps, 0, 1, 0xff, 0xff}, append([]byte{msgHave, 0, 0}, 0)} {
		if _, err := s.OnMessage("p", msg); !errors.Is(err, ErrMalformed) {
			t.Errorf("OnMessage(%x) = %v, want ErrMalformed", msg, err)
		}
	}
}

// Out-of-order delivery is not the peer's fault: OnMessage reports only
// malformed ops, never ErrUnknownAuthor.
func TestSyncOutOfOrderIsNotAPeerError(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	src := project(t, a)
	mustIssue(t, src, a, OpAdd, b, RoleAdmin)
	ops := src.Since(nil) // genesis, then the Add of b.
	mustIssue(t, src, b, OpAdd, newSigner(3), RoleWriter)
	last := src.Since([]Hash{ops[1].Hash()})
	s := NewSync(New(src.Project()))
	for _, m := range chunk("p", last) { // b's op before b's grant: unknown author.
		if _, err := s.OnMessage("p", m.Data); err != nil {
			t.Errorf("OnMessage(op of an unknown author) = %v, want nil", err)
		}
	}
	if s.list.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.list.Len())
	}
}

func TestSyncChunksLargeBatches(t *testing.T) {
	a := newSigner(1)
	l := project(t, a)
	if err := chain(t, l, a, 600); err != nil {
		t.Fatal(err)
	}
	msgs := chunk("p", l.Since(nil))
	if len(msgs) < 2 {
		t.Fatalf("chunk produced %d messages, want several", len(msgs))
	}
	peer := NewSync(New(l.Project()))
	for _, m := range msgs {
		if len(m.Data) > MaxMessage {
			t.Fatalf("message of %d bytes exceeds MaxMessage", len(m.Data))
		}
		if _, err := peer.OnMessage("q", m.Data); err != nil {
			t.Fatal(err)
		}
	}
	if peer.list.Len() != l.Len() {
		t.Errorf("Len() = %d, want %d", peer.list.Len(), l.Len())
	}
}

// haveMsg encodes a Have message announcing hs.
func haveMsg(hs ...Hash) []byte {
	b := []byte{msgHave, byte(len(hs) >> 8), byte(len(hs))}
	for _, h := range hs {
		b = append(b, h[:]...)
	}
	return b
}

func TestSyncNoAnnounceStorm(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	la := project(t, a)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin)
	lb := fork(t, la)
	for i := range 3 {
		if _, err := la.Issue(a, OpAdd, key(50, i).Public(), RoleReader, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := lb.Issue(b, OpAdd, key(51, i).Public(), RoleWriter, ""); err != nil {
			t.Fatal(err)
		}
	}
	sa, sb := NewSync(la), NewSync(lb)
	const A, B = transport.PeerID("A"), transport.PeerID("B")
	queue := append(sa.OnPeerUp(B), sb.OnPeerUp(A)...) // one announcement each, no Tick.
	msgs := 0
	for ; len(queue) > 0 && msgs < 100; msgs++ {
		m := queue[0]
		queue = queue[1:]
		var out []Outbound
		if m.To == B {
			out, _ = sb.OnMessage(A, m.Data)
		} else {
			out, _ = sa.OnMessage(B, m.Data)
		}
		queue = append(queue, out...)
	}
	if len(queue) > 0 || msgs > 6 {
		t.Errorf("exchange took %d messages (%d left), want at most 6", msgs, len(queue))
	}
	if d := cmp.Diff(la.Members(), lb.Members()); d != "" {
		t.Errorf("members differ:\n%s", d)
	}
	if d := cmp.Diff(la.Frontier(), lb.Frontier()); d != "" {
		t.Errorf("frontiers differ:\n%s", d)
	}
}

func TestSyncHaveNeverAnsweredWithHave(t *testing.T) {
	s := NewSync(project(t, newSigner(1)))
	out, err := s.OnMessage("p", haveMsg(Hash{9})) // a hash we do not know.
	if err != nil || len(out) == 0 {
		t.Fatalf("OnMessage(Have) = %d messages, %v; want the ops", len(out), err)
	}
	for _, m := range out {
		if m.Data[0] != msgOps {
			t.Errorf("answer has kind %d, want Ops (%d)", m.Data[0], msgOps)
		}
	}
}

func TestSyncHaveBudgetPerTick(t *testing.T) {
	s := NewSync(project(t, newSigner(1)))
	answered := func(p transport.PeerID) (n int) {
		for range 10 {
			if out, _ := s.OnMessage(p, haveMsg()); len(out) > 0 {
				n++
			}
		}
		return n
	}
	if n := answered("p"); n != 2 {
		t.Errorf("answers to 10 Haves = %d, want 2", n)
	}
	if n := answered("q"); n != 2 {
		t.Errorf("answers to a second peer = %d, want its own budget of 2", n)
	}
	s.Tick()
	if n := answered("p"); n != 2 {
		t.Errorf("answers after Tick = %d, want 2", n)
	}
	s.Tick()
	answered("p")
	s.OnPeerDown("p") // the budget is the peer's, not the connection's.
	s.OnPeerUp("p")
	if n := answered("p"); n != 0 {
		t.Errorf("answers after a reconnection = %d, want 0", n)
	}
}

func TestSyncSealedDoesNotAnnounce(t *testing.T) {
	a := newSigner(1)
	l := project(t, a)
	if err := chain(t, l, a, maxOps-1); err != nil || !l.Sealed() {
		t.Fatalf("chain = %v, Sealed() = %v", err, l.Sealed())
	}
	s := NewSync(l)
	if out := s.OnPeerUp("p"); len(out) != 0 {
		t.Errorf("OnPeerUp announced %d messages while sealed", len(out))
	}
	if out := s.Tick(); len(out) != 0 {
		t.Errorf("Tick announced %d messages while sealed", len(out))
	}
	if out, _ := s.OnMessage("p", haveMsg()); len(out) == 0 {
		t.Error("a sealed list must still answer a Have")
	}
}
