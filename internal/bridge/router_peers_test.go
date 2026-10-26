package bridge_test

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// gated blocks Send to the peer "slow" until release is closed, as a slow
// link does.
type gated struct {
	transport.Transport
	release  chan struct{}
	canceled *atomic.Bool // set when a send to "slow" is interrupted
	dropped  *atomic.Value
}

// Reset records the peer the router reset.
func (g gated) Reset(who transport.PeerID) { g.dropped.Store(who) }

func (g gated) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	if to == "slow" {
		select {
		case <-g.release:
		case <-ctx.Done():
			g.canceled.Store(true)
			return ctx.Err()
		}
	}
	return g.Transport.Send(ctx, to, p)
}

// TestSlowPeerDoesNotBlockOthers: a send stuck on one peer holds back neither
// the other peers nor the order of that peer's own messages (the priority gate
// of the mux is useless if the router sends one message at a time).
func TestSlowPeerDoesNotBlockOthers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, slow, fast := n.Join("a"), n.Join("slow"), n.Join("fast")
		n.Link("a", "slow")
		n.Link("a", "fast")
		g := newGated(a)
		r := bridge.NewRouter(g, quietLogger())
		c := newFakeConn(t)
		serveDone := wait(func() { r.Serve(c) })
		c.next(t)
		c.next(t) // the two PeerUp

		for _, m := range []bridge.Envelope{
			{Kind: bridge.Message, Peer: "slow", Payload: []byte("s1")},
			{Kind: bridge.Message, Peer: "slow", Payload: []byte("s2")},
			{Kind: bridge.Message, Peer: "fast", Payload: []byte("f1")},
		} {
			c.push(mustEncode(t, m))
		}
		synctest.Wait()
		if got := messages(t, fast); !slices.Equal(got, []string{"f1"}) {
			t.Fatalf("fast received %q while slow is stuck, want [f1]", got)
		}
		close(g.release)
		synctest.Wait()
		if got := messages(t, slow); !slices.Equal(got, []string{"s1", "s2"}) {
			t.Errorf("slow received %q, want [s1 s2] in order", got)
		}
		c.close()
		<-serveDone
	})
}

// messages drains the payloads queued at nd.
func messages(t *testing.T, nd transport.Transport) []string {
	t.Helper()
	var out []string
	for {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		ev, err := nd.Recv(ctx)
		if err != nil {
			return out
		}
		if ev.Kind == transport.Message {
			out = append(out, string(ev.Payload))
		}
	}
}

func newGated(t transport.Transport) gated {
	return gated{Transport: t, release: make(chan struct{}), canceled: new(atomic.Bool), dropped: new(atomic.Value)}
}

// TestFullQueueResetsThePeer: a peer that does not read fills its queue; the
// router then drops the message and resets that peer instead of waiting,
// which would stall every other peer.
func TestFullQueueResetsThePeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, _, fast := n.Join("a"), n.Join("slow"), n.Join("fast")
		n.Link("a", "slow")
		n.Link("a", "fast")
		g := newGated(a)
		r := bridge.NewRouter(g, quietLogger())
		c := newFakeConn(t)
		serveDone := wait(func() { r.Serve(c) })
		c.next(t)
		c.next(t)

		for i := range 200 { // far more than the queue holds
			c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "slow", Payload: []byte(fmt.Sprint(i))}))
		}
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "fast", Payload: []byte("f1")}))
		synctest.Wait()
		if got := messages(t, fast); !slices.Equal(got, []string{"f1"}) {
			t.Fatalf("fast received %q behind a full queue, want [f1]", got)
		}
		if who, _ := g.dropped.Load().(transport.PeerID); who != "slow" {
			t.Errorf("Reset(%q), want the slow peer reset", who)
		}
		c.close()
		<-serveDone
	})
}

// TestPeerDownStopsItsSender: when a peer goes down, the send stuck on it is
// interrupted.
func TestPeerDownStopsItsSender(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("slow")
		n.Link("a", "slow")
		g := newGated(a)
		r := bridge.NewRouter(g, quietLogger())
		c := newFakeConn(t)
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })
		serveDone := wait(func() { r.Serve(c) })
		c.next(t) // PeerUp
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "slow", Payload: []byte("x")}))
		synctest.Wait()
		n.Partition("a", "slow")
		synctest.Wait()
		if !g.canceled.Load() {
			t.Error("the send to the peer that went down was not interrupted")
		}
		cancel()
		<-runDone
		c.close()
		<-serveDone
	})
}
