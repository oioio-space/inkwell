package p2p_test

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// TestWantLateStart: a peer that is not there yet is found once it shows up,
// however long the node waited for it.
func TestWantLateStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := idOf(key(2))
		a.Want(map[transport.PeerID][]string{b: {"/ip4/2.2.2.1/udp/8000/quic-v1"}})
		time.Sleep(40 * time.Second)
		expectQuietFor(t, a, quiet)

		late := nn.node(t, 2, p2p.Options{Relays: relays})
		expectEventWithin(t, a, transport.PeerUp, b, 90*time.Second)
		expectEventWithin(t, late, transport.PeerUp, a.Local(), wait)
	})
}

// TestWantStopsAndDrop: a peer that is no longer wanted, or was dropped, is not
// dialed again.
func TestWantStopsAndDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		bID := idOf(key(2))
		var revoked atomic.Bool
		a := nn.node(t, 1, p2p.Options{Relays: relays, Admit: func(id transport.PeerID) bool { return !revoked.Load() || id != bID }})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		a.Want(map[transport.PeerID][]string{b.Local(): nil, "nobody": nil, a.Local(): nil})
		expectUp(t, a, b)

		// Dropping a revoked peer closes the link and cancels the want: it
		// stays closed, even once the peer is admitted again.
		revoked.Store(true)
		a.Drop(b.Local())
		expectEvent(t, a, transport.PeerDown, b.Local())
		expectEvent(t, b, transport.PeerDown, a.Local())
		revoked.Store(false)
		expectQuietFor(t, a, 3*time.Minute)

		// Wanted again, it is dialed; no longer wanted, it is left alone.
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), wait)
		expectEventWithin(t, b, transport.PeerUp, a.Local(), wait)
		a.Want(nil)
		expectQuietFor(t, a, time.Minute)
		if got := a.Peers(); len(got) != 1 {
			t.Errorf("a.Peers() = %v after Want(nil), want the connection left alone", got)
		}
	})
}

// TestAddressChangeRetriesNow: after a long wait the retries are a minute
// apart, but a change of the node's own address (it moved to another network)
// retries at once.
func TestAddressChangeRetriesNow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		a.Want(map[transport.PeerID][]string{idOf(key(2)): nil})
		time.Sleep(150 * time.Second) // the backoff is at its longest

		b := nn.node(t, 2, p2p.Options{Relays: relays})
		time.Sleep(time.Second) // b reserves a slot on the relay
		nn.roam(a.end)
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 20*time.Second)
	})
}

// TestRefusedPeerIsNotHammered: a peer that drops our connection as soon as it
// is made is retried with a growing delay, not in a loop.
func TestRefusedPeerIsNotHammered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		friend := idOf(key(3))
		a := nn.nodeAt(t, 1, true, p2p.Options{Admit: func(id transport.PeerID) bool { return id == friend }})
		stranger := nn.nodeAt(t, 2, true, p2p.Options{})
		var made atomic.Int32
		stranger.Host().Network().Notify(&network.NotifyBundle{ConnectedF: func(network.Network, network.Conn) { made.Add(1) }})

		stranger.Want(map[transport.PeerID][]string{a.Local(): a.Addrs()})
		time.Sleep(3 * time.Minute)
		// 1 s, 2 s, 4 s… up to a minute: about 8 attempts in 3 minutes.
		if got := made.Load(); got < 2 || got > 12 {
			t.Errorf("the stranger made %d connections in 3 minutes, want a few with growing delays", got)
		}
	})
}

// TestResetReconnectsAfterBackoff: a peer that was reset (it did not read) is
// let back in after a short backoff that doubles on repeated resets, as long
// as it is wanted; a dropped peer, in contrast, stays away.
func TestResetReconnectsAfterBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		bID := idOf(key(2))
		var revoked atomic.Bool
		a := nn.node(t, 1, p2p.Options{Relays: relays, Admit: func(id transport.PeerID) bool { return !revoked.Load() || id != bID }})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		expectUp(t, a, b)

		a.Reset(b.Local())
		expectEvent(t, a, transport.PeerDown, b.Local())
		expectEvent(t, b, transport.PeerDown, a.Local())
		expectQuietFor(t, a, time.Second) // held back: no instant redial
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 20*time.Second)
		expectEventWithin(t, b, transport.PeerUp, a.Local(), wait)

		// A second reset right after waits longer.
		a.Reset(b.Local())
		expectEvent(t, a, transport.PeerDown, b.Local())
		expectEvent(t, b, transport.PeerDown, a.Local())
		expectQuietFor(t, a, 3*time.Second)
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 30*time.Second)
		expectEventWithin(t, b, transport.PeerUp, a.Local(), wait)

		// Dropped (suspended, removed), it does not come back.
		revoked.Store(true) // as the app does, first
		a.Drop(b.Local())
		expectEvent(t, a, transport.PeerDown, b.Local())
		expectQuietFor(t, a, 3*time.Minute)
	})
}
