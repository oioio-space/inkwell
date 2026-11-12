package p2p

import (
	"context"
	"crypto/ed25519"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/oioio-space/inkwell/internal/transport"
)

func loopbackNode(t *testing.T, seed byte, o Options) *Node {
	t.Helper()
	k := ed25519.NewKeyFromSeed(append(make([]byte, 31), seed))
	n, err := New(t.Context(), k, o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}

func outboundProbeStreams(n *Node, other *Node) int {
	out := 0
	for _, c := range n.h.Network().ConnsToPeer(other.h.ID()) {
		for _, s := range c.GetStreams() {
			if s.Protocol() == ProtocolProbe && s.Stat().Direction == network.DirOutbound {
				out++
			}
		}
	}
	return out
}

// TestLinkProbeStaleOpenAttempt: a role removed while the outbound stream is still opening ends
// that attempt (the Host reset it), and the attempt, finding itself stale, resets the stream it
// opened instead of leaving a probe stream the Host knows nothing about.
func TestLinkProbeStaleOpenAttempt(t *testing.T) {
	var member atomic.Bool
	member.Store(true)
	a := loopbackNode(t, 1, Options{Admit: func(transport.PeerID) bool { return member.Load() }})
	b := loopbackNode(t, 2, Options{})
	reached, release := make(chan struct{}, 1), make(chan struct{})
	var once atomic.Bool
	a.probes.afterNewStream = func() {
		if once.CompareAndSwap(false, true) {
			reached <- struct{}{}
			<-release
		}
	}
	if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	select {
	case <-reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the prober never opened its stream")
	}
	pc := a.probes.of(a.h.Network().ConnsToPeer(b.h.ID())[0].ID())
	pc.mu.Lock()
	before := pc.attempt
	pc.mu.Unlock()
	member.Store(false) // b is no longer a member: the next tick resets the attempt
	deadline := time.Now().Add(5 * time.Second)
	for {
		pc.mu.Lock()
		now := pc.attempt
		pc.mu.Unlock()
		if now != before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the role change did not end the attempt")
		}
		time.Sleep(50 * time.Millisecond)
	}
	member.Store(true)
	close(release) // the stale attempt goes on to negotiate
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if n := outboundProbeStreams(a, b); n != 0 {
			t.Fatalf("%d outbound probe streams after the attempt was cancelled, want none", n)
		}
	}
}

// TestSendHoldsTheWriterLockOfTheStreamItWritesOn: a Send that waited for the writer lock of a peer
// state that has been replaced meanwhile (the peer went down and came back) must take the new
// state's lock before it writes on the new stream, or two frames interleave on it.
func TestSendHoldsTheWriterLockOfTheStreamItWritesOn(t *testing.T) {
	a, b := loopbackNode(t, 3, Options{}), loopbackNode(t, 4, Options{})
	if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	for _, n := range []*Node{a, b} {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		ev, err := n.Recv(ctx)
		cancel()
		if err != nil || ev.Kind != transport.PeerUp {
			t.Fatalf("Recv = %v, %v, want PeerUp", ev.Kind, err)
		}
	}
	id := b.h.ID()
	a.mu.Lock()
	ps1 := a.peers[id]
	a.mu.Unlock()
	ps1.wsem <- struct{}{} // another writer holds the old state's lock

	done := make(chan error, 1)
	go func() { done <- a.Send(t.Context(), b.Local(), []byte("late frame")) }()
	time.Sleep(300 * time.Millisecond) // it waits for ps1's lock

	a.mu.Lock() // the peer went down and came back: a new state, another lock, a stream in use
	ps2 := &peerState{
		id: ps1.id, name: ps1.name, wsem: make(chan struct{}, 1), bulkSem: make(chan struct{}, 1),
		connected: true, admitted: true, up: true, out: ps1.out,
	}
	a.peers[id] = ps2
	a.mu.Unlock()
	ps2.wsem <- struct{}{} // and a writer is writing on it

	<-ps1.wsem // the old writer ends
	ctx, cancel := context.WithTimeout(t.Context(), 700*time.Millisecond)
	ev, err := b.Recv(ctx)
	cancel()
	if err == nil {
		t.Fatalf("the frame %q was written while another writer held the new stream", ev.Payload)
	}
	<-ps2.wsem // that writer ends: now the frame may go
	ctx, cancel = context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if ev, err := b.Recv(ctx); err != nil || string(ev.Payload) != "late frame" {
		t.Fatalf("Recv = %q, %v, want the late frame", ev.Payload, err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Send: %v", err)
	}
}
