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
