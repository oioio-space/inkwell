package mux_test

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// TestOBS1Z5LinkCountersSummedOnRead proves rule Z-5's counters (plan
// docs/plans/2026-11-24-obs1-observability.md §2): each peer's bytes are
// counted on its own link, received and opened exactly, sent at most one held
// back buffer late per sender, large messages included, and only summed when
// read.
func TestOBS1Z5LinkCountersSummedOnRead(t *testing.T) {
	n := memnet.New(1, memnet.Faults{})
	hub := mux.Split(n.Join("hub"), chRepo, chTrust)
	a, b := n.Join("a"), n.Join("b")
	t.Cleanup(func() { _ = hub[chRepo].Close(); _ = a.Close(); _ = b.Close() })
	n.Link("hub", "a")
	n.Link("hub", "b")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	const senders, frames = 4, 1000
	sizes := map[transport.PeerID]int{"a": 4096, "b": 2048}
	want := map[transport.PeerID]uint64{}
	var wg sync.WaitGroup
	for i := range senders {
		ch := []byte{chRepo, chTrust}[i%2]
		for peer, size := range sizes {
			want[peer] += uint64(frames * (size + mux.Overhead))
			wg.Go(func() {
				payload := make([]byte, size)
				for range frames {
					if err := hub[ch].Send(ctx, peer, payload); err != nil {
						t.Errorf("Send to %s: %v", peer, err)
						return
					}
				}
			})
		}
	}
	// A large message crosses as fragments: every fragment is counted, headers included.
	large := make([]byte, 200<<10)
	if err := hub[chRepo].Send(ctx, "a", large); err != nil {
		t.Fatalf("large Send: %v", err)
	}
	wireLarge := drainBytes(ctx, t, a, func(n uint64) bool { return n > uint64(len(large)) })
	want["a"] += wireLarge
	wg.Wait()

	// What the peers send in: counted exactly, on the right link.
	got := map[transport.PeerID]uint64{}
	for peer, nd := range map[transport.PeerID]*memnet.Node{"a": a, "b": b} {
		for i := range 10 * len(peer) * 3 {
			frame := append([]byte{chTrust}, make([]byte, 100+i)...)
			if err := nd.Send(ctx, "hub", frame); err != nil {
				t.Fatal(err)
			}
			got[peer] += uint64(len(frame))
		}
	}
	for peer := range got {
		for range 10 * len(peer) * 3 {
			if _, err := recvMessage(ctx, hub[chTrust]); err != nil {
				t.Fatal(err)
			}
		}
	}

	// A send buffer the pool let go (at a collection, or at random under the race detector) adds
	// what it held back once it is collected: then only the buffers still pooled lag behind.
	slack := uint64(2*senders) * (mux.FlushAt + 64<<10)
	byPeer := map[transport.PeerID]mux.LinkTraffic{}
	waitFor(t, func() bool {
		runtime.GC()
		for _, l := range mux.Traffic(hub[chTrust]) {
			byPeer[l.Peer] = l
		}
		return byPeer["a"].Sent+slack >= want["a"] && byPeer["b"].Sent+slack >= want["b"]
	})
	for peer, w := range want {
		l := byPeer[peer]
		if l.Sent > w || l.Sent+slack < w {
			t.Errorf("%s: Sent = %d, want %d (at most %d held back)", peer, l.Sent, w, slack)
		}
		if l.Received != got[peer] {
			t.Errorf("%s: Received = %d, want exactly %d", peer, l.Received, got[peer])
		}
		if l.Opened != 1 {
			t.Errorf("%s: Opened = %d, want 1", peer, l.Opened)
		}
	}
	if len(byPeer) != 2 {
		t.Errorf("Traffic has %d links, want 2 (a, b): %+v", len(byPeer), byPeer)
	}

	// A connection opened again is counted again; a reader of another Split sees its own.
	n.Partition("hub", "b")
	n.Link("hub", "b")
	waitFor(t, func() bool {
		for _, l := range mux.Traffic(hub[chRepo]) {
			if l.Peer == "b" {
				return l.Opened == 2
			}
		}
		return false
	})
	if mux.Traffic(a) != nil {
		t.Error("Traffic of a transport that is not a sub-transport is not nil")
	}
}

// drainBytes reads nd's messages until done says their total is enough, and
// returns it.
func drainBytes(ctx context.Context, t *testing.T, nd transport.Transport, done func(uint64) bool) uint64 {
	t.Helper()
	var n uint64
	for !done(n) {
		ev, err := nd.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		if ev.Kind == transport.Message && ev.Payload[0] == 0xFF {
			n += uint64(len(ev.Payload))
		}
	}
	return n
}

// recvMessage returns the next message of t, skipping peer events.
func recvMessage(ctx context.Context, t transport.Transport) (transport.Event, error) {
	for {
		ev, err := t.Recv(ctx)
		if err != nil || ev.Kind == transport.Message {
			return ev, err
		}
	}
}

// waitFor polls ok until it holds, for at most 10 s.
func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached in 10 s")
		}
	}
}

// TestCountedLaneCountsPerLink proves that a lane without channels, such as
// the bulk lane of the blobs, has its bytes counted per link like a Split's:
// each frame sent or received, and each connection.
func TestCountedLaneCountsPerLink(t *testing.T) {
	n := memnet.New(1, memnet.Faults{})
	lane := mux.Counted(n.JoinBulk("hub"))
	peer := n.JoinBulk("a")
	t.Cleanup(func() { _ = lane.Close(); _ = peer.Close() })
	n.Link("hub", "a")
	ctx := t.Context()
	var sent, received uint64
	for i := range 50 {
		chunk := make([]byte, 1000+i)
		if err := lane.Send(ctx, "a", chunk); err != nil {
			t.Fatal(err)
		}
		sent += uint64(len(chunk))
		if err := peer.Send(ctx, "hub", chunk[:500]); err != nil {
			t.Fatal(err)
		}
		received += 500
	}
	for got := 0; got < 50; {
		ev, err := lane.Recv(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind == transport.Message {
			got++
		}
	}
	tr := mux.Traffic(lane)
	if len(tr) != 1 || tr[0] != (mux.LinkTraffic{Peer: "a", Sent: sent, Received: received, Opened: 1}) {
		t.Errorf("Traffic = %+v, want a: sent %d, received %d, opened 1", tr, sent, received)
	}
}
