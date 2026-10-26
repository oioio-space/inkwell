package bridge_test

import (
	"context"
	"slices"
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
	release chan struct{}
}

func (g gated) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	if to == "slow" {
		select {
		case <-g.release:
		case <-ctx.Done():
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
		g := gated{Transport: a, release: make(chan struct{})}
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
