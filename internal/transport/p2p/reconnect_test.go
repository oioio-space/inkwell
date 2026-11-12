package p2p_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// The bounds of docs/plans/2026-11-12-rc1-reconnect.md §6.
const (
	// reconnectBound is R = knownDialTimeout + 5 s: the restarted device holds a valid address.
	reconnectBound = 15 * time.Second
	// sleepingBound: only the surviving device holds one, and its dial loop sleeps (75 s + 10 s
	// + 5 s).
	sleepingBound = 90 * time.Second
	// awayFor is long enough for the survivor's dial loop to reach its longest pause.
	awayFor = 3 * time.Minute
)

// meet connects a and b once, as an invitation does (each dials the other, so that both NATs let
// the packets in), then makes each want the other with no address, as the app does
// (Admission.dialTargets).
func meet(t *testing.T, a, b *simNode) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	var errA, errB error
	wg.Go(func() { errA = a.Connect(ctx, b.Local(), b.Addrs()) })
	wg.Go(func() { errB = b.Connect(ctx, a.Local(), a.Addrs()) })
	wg.Wait()
	if errA != nil && errB != nil {
		t.Fatalf("neither side could connect: %v; %v", errA, errB)
	}
	wantEachOther(a, b)
	expectUp(t, a, b)
	// The book records a peer when identify's event is handled, a few milliseconds after PeerUp:
	// wait for it, so that a node closed right after holds the other's address, as R assumes.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		ab, _ := a.BookAddrs(b.Local())
		ba, _ := b.BookAddrs(a.Local())
		if len(ab) > 0 && len(ba) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("books after 5 s: a has %v for b, b has %v for a", ab, ba)
		}
	}
}

// restartAfter closes b, waits for away, and starts b's identity again on the endpoint of
// endSeed with book, wanting a with no address as the app does. It returns the new node and T0,
// the instant it listens with its Want installed.
func restartAfter(t *testing.T, nn *natNet, a, b *simNode, away time.Duration, endSeed byte, public bool, book p2p.BookStore) (*simNode, time.Time) {
	t.Helper()
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	expectEventWithin(t, a, transport.PeerDown, b.Local(), time.Minute)
	time.Sleep(away)
	again := nn.nodeKeyAt(t, 2, endSeed, public, p2p.Options{Book: book})
	again.Want(map[transport.PeerID][]string{a.Local(): nil})
	return again, time.Now()
}

// expectBackWithin requires a and b to see each other up again within d of t0.
func expectBackWithin(t *testing.T, a, b *simNode, t0 time.Time, d time.Duration) {
	t.Helper()
	expectEventWithin(t, b, transport.PeerUp, a.Local(), d)
	expectEventWithin(t, a, transport.PeerUp, b.Local(), d)
	if got := time.Since(t0); got > d {
		t.Errorf("link back %v after the restart, want at most %v", got, d)
	}
}

// TestReconnectAfterRestartNoRelay is the repro of the block made permanent: two members, no
// relay, one restarts. The bound R holds when the restarted device holds a valid address of the
// other (same address; or it moved and the survivor did not), and the longer bound when only the
// sleeping survivor holds one.
func TestReconnectAfterRestartNoRelay(t *testing.T) {
	t.Run("samePort", func(t *testing.T) {
		forSeeds(t, func(t *testing.T, nn *natNet) {
			bBook := &p2p.MemBook{}
			a := nn.nodeKeyAt(t, 1, 1, false, p2p.Options{Book: &p2p.MemBook{}})
			b := nn.nodeKeyAt(t, 2, 2, false, p2p.Options{Book: bBook})
			meet(t, a, b)
			b, t0 := restartAfter(t, nn, a, b, awayFor, 2, false, bBook)
			expectBackWithin(t, a, b, t0, reconnectBound)
		})
	})
	t.Run("restartedMovedPort", func(t *testing.T) {
		forSeeds(t, func(t *testing.T, nn *natNet) {
			bBook := &p2p.MemBook{}
			a := nn.nodeKeyAt(t, 1, 1, true, p2p.Options{Book: &p2p.MemBook{}})
			b := nn.nodeKeyAt(t, 2, 2, false, p2p.Options{Book: bBook})
			meet(t, a, b)
			b, t0 := restartAfter(t, nn, a, b, awayFor, 3, false, bBook) // another network
			expectBackWithin(t, a, b, t0, reconnectBound)
		})
	})
	t.Run("survivorSleeping", func(t *testing.T) {
		forSeeds(t, func(t *testing.T, nn *natNet) {
			a := nn.nodeKeyAt(t, 1, 1, false, p2p.Options{Book: &p2p.MemBook{}})
			b := nn.nodeKeyAt(t, 2, 2, true, p2p.Options{Book: &p2p.MemBook{}})
			meet(t, a, b)
			b, t0 := restartAfter(t, nn, a, b, awayFor, 2, true, &p2p.MemBook{}) // its book is lost
			expectBackWithin(t, a, b, t0, sleepingBound)
		})
	})
}

// TestReconnectAfterLongOutage: a member away for an hour, past libp2p's 15-minute expiry of the
// addresses of a disconnected peer, still comes back within R: the dial reads the book.
func TestReconnectAfterLongOutage(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		bBook := &p2p.MemBook{}
		a := nn.nodeKeyAt(t, 1, 1, false, p2p.Options{Book: &p2p.MemBook{}})
		b := nn.nodeKeyAt(t, 2, 2, false, p2p.Options{Book: bBook})
		meet(t, a, b)
		b, t0 := restartAfter(t, nn, a, b, time.Hour, 2, false, bBook)
		expectBackWithin(t, a, b, t0, reconnectBound)
	})
}

// TestReconnectStaleAddressWrongPeer: §5, the recorded address of a member now answers as
// another device: the handshake fails, no link and no event for the member.
func TestReconnectStaleAddressWrongPeer(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		a := nn.nodeKeyAt(t, 1, 1, false, p2p.Options{Book: &p2p.MemBook{}})
		b := nn.nodeKeyAt(t, 2, 2, true, p2p.Options{})
		meet(t, a, b)
		if err := b.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		expectEventWithin(t, a, transport.PeerDown, b.Local(), time.Minute)
		other := nn.nodeKeyAt(t, 3, 2, true, p2p.Options{}) // another device at b's address
		expectQuietFor(t, a, awayFor)
		expectQuietFor(t, other, quiet)
		if got := a.Peers(); len(got) != 0 {
			t.Errorf("a.Peers() = %v, want none", got)
		}
		if got := linkTo(a, b.Local()); got != 0 {
			t.Errorf("link a->b = %v, want none", got)
		}
	})
}

// bookFor is a stored book holding addr for peer, seen now.
func bookFor(peer transport.PeerID, addr string) *p2p.MemBook {
	m := &p2p.MemBook{}
	m.Set(p2p.BookFile(time.Now(), map[transport.PeerID][]string{peer: {addr}}))
	return m
}

// directAddr is the address others dial n at.
func directAddr(n *simNode) string {
	return fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", n.end.external, simPort)
}

// TestDialFallbackUsesBook: §2.4, a peer wanted with no address is dialed at the book's; without
// an entry (nor relay) it is not dialed at all, the defect the block fixes.
func TestDialFallbackUsesBook(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		a := nn.node(t, 1, p2p.Options{Book: bookFor(b.Local(), directAddr(b))})
		without := nn.node(t, 3, p2p.Options{})
		without.Want(map[transport.PeerID][]string{b.Local(): nil})
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), reconnectBound)
		expectQuietFor(t, without, time.Minute)
	})
}

// TestDialFallbackRelayOnly: §2.4, a RelayOnly node dials no address of its book.
func TestDialFallbackRelayOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		var made atomic.Int32
		b.Host().Network().Notify(&network.NotifyBundle{ConnectedF: func(_ network.Network, c network.Conn) {
			if c.RemotePeer().String() == string(idOf(key(1))) {
				made.Add(1)
			}
		}})
		a := nn.node(t, 1, p2p.Options{Relays: relays, RelayOnly: true, Book: bookFor(b.Local(), directAddr(b))})
		a.Want(map[transport.PeerID][]string{b.Local(): nil})
		expectQuietFor(t, a, time.Minute)
		if got := made.Load(); got != 0 {
			t.Errorf("the RelayOnly node made %d direct connections from its book, want none", got)
		}
	})
}

// TestHangupKeepsWant: §2.6, after Hangup a wanted member comes back and stays in the book; a
// stranger's peerstore entry is cleared.
func TestHangupKeepsWant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		a := nn.node(t, 1, p2p.Options{})
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		stranger := nn.nodeAt(t, 3, true, p2p.Options{})
		a.Want(map[transport.PeerID][]string{b.Local(): b.Addrs()})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), wait)

		a.Hangup(b.Local())
		expectEventWithin(t, a, transport.PeerDown, b.Local(), wait)
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 30*time.Second)
		if got, _ := a.BookAddrs(b.Local()); len(got) == 0 {
			t.Error("Hangup removed the member from the address book")
		}

		if err := a.Connect(t.Context(), stranger.Local(), stranger.Addrs()); err != nil {
			t.Fatalf("Connect(stranger): %v", err)
		}
		expectEventWithin(t, a, transport.PeerUp, stranger.Local(), wait)
		a.Hangup(stranger.Local())
		expectEventWithin(t, a, transport.PeerDown, stranger.Local(), wait)
		id, _ := p2p.LibP2PID(stranger.Local())
		if got := a.Host().Peerstore().Addrs(id); len(got) != 0 {
			t.Errorf("after Hangup the peerstore holds %v for a stranger, want nothing", got)
		}
		expectQuietFor(t, a, time.Minute)
	})
}

// TestDropUnwantsAndForgetsBook: §2.6 and §4, Drop un-wants the member, deletes its entry and
// saves the book at once.
func TestDropUnwantsAndForgetsBook(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		bID := idOf(key(2))
		var revoked atomic.Bool
		book := &p2p.MemBook{}
		a := nn.node(t, 1, p2p.Options{Book: book, Admit: func(id transport.PeerID) bool { return !revoked.Load() || id != bID }})
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		a.Want(map[transport.PeerID][]string{b.Local(): b.Addrs()})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), wait)
		if got, _ := a.BookAddrs(bID); len(got) == 0 {
			t.Fatal("the member is not in the address book once up")
		}

		revoked.Store(true)
		saves := book.Saves()
		a.Drop(bID)
		if got := book.Saves(); got <= saves {
			t.Errorf("Drop saved the book %d times, want at once", got-saves)
		}
		if strings.Contains(string(book.Data()), string(bID)) {
			t.Errorf("after Drop the stored book still holds the member: %s", book.Data())
		}
		if got, _ := a.BookAddrs(bID); len(got) != 0 {
			t.Errorf("after Drop the book holds %v for the member, want nothing", got)
		}
		expectEventWithin(t, a, transport.PeerDown, bID, wait)
		revoked.Store(false)
		expectQuietFor(t, a, awayFor) // un-wanted: not dialed again
	})
}

// TestBookRecordsAdmittedOnly: §2.3, the addresses of a peer Admit refuses (a stranger let in by
// the window of SetOpen) are never recorded, on PeerUp or on Want.
func TestBookRecordsAdmittedOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		bID := idOf(key(2))
		a := nn.nodeAt(t, 1, true, p2p.Options{Admit: func(id transport.PeerID) bool { return id == bID }})
		a.SetOpen(time.Now().Add(time.Hour))
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		stranger := nn.nodeAt(t, 3, true, p2p.Options{})
		for _, n := range []*simNode{b, stranger} {
			if err := n.Connect(t.Context(), a.Local(), a.Addrs()); err != nil {
				t.Fatalf("Connect: %v", err)
			}
		}
		for len(a.Peers()) < 2 {
			time.Sleep(100 * time.Millisecond)
		}
		a.Want(map[transport.PeerID][]string{stranger.Local(): nil})
		if got, _ := a.BookAddrs(bID); !slices.Equal(got, []string{directAddr(b)}) {
			t.Errorf("book of the member = %v, want [%s]", got, directAddr(b))
		}
		if got, _ := a.BookAddrs(stranger.Local()); len(got) != 0 {
			t.Errorf("book of the stranger = %v, want nothing", got)
		}
	})
}

// TestBookRecordsMDNSSighting: §2.3, a sighting of a member on the local network updates its
// addresses and when it was seen; a stranger's is not recorded.
func TestBookRecordsMDNSSighting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		bID, sID := idOf(key(2)), idOf(key(3))
		a := nn.nodeAt(t, 1, true, p2p.Options{Admit: func(id transport.PeerID) bool { return id == bID }})
		sight := func(who transport.PeerID, addr string) {
			id, _ := p2p.LibP2PID(who)
			a.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast(addr)}})
		}
		sight(bID, "/ip4/192.168.1.20/udp/7419/quic-v1")
		first := time.Now()
		time.Sleep(time.Minute)
		sight(bID, "/ip4/192.168.1.21/udp/7419/quic-v1")
		got, seen := a.BookAddrs(bID)
		if want := []string{"/ip4/192.168.1.21/udp/7419/quic-v1", "/ip4/192.168.1.20/udp/7419/quic-v1"}; !slices.Equal(got, want) {
			t.Errorf("book after two sightings = %v, want %v", got, want)
		}
		if !seen.After(first) {
			t.Errorf("seen = %v after a sighting at %v, want the later one", seen, time.Now())
		}
		sight(sID, "/ip4/192.168.1.30/udp/7419/quic-v1")
		if got, _ := a.BookAddrs(sID); len(got) != 0 {
			t.Errorf("book of a stranger seen on the network = %v, want nothing", got)
		}
	})
}

// TestBookLANSpoofKeepsAuthenticated: a device on the local network floods a member's entry with
// forged mDNS announcements; the member's authenticated addresses survive, on disk too, and the
// member, back with its book lost, is reconnected through them.
func TestBookLANSpoofKeepsAuthenticated(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		aBook := &p2p.MemBook{}
		a := nn.nodeKeyAt(t, 1, 1, false, p2p.Options{Book: aBook})
		b := nn.nodeKeyAt(t, 2, 2, true, p2p.Options{})
		meet(t, a, b)
		id, _ := p2p.LibP2PID(b.Local())
		for i := range 50 {
			a.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/192.168.9.%d/udp/8000/quic-v1", i+1))}})
		}
		if got, _ := a.BookAddrs(b.Local()); len(got) == 0 || got[0] != directAddr(b) {
			t.Fatalf("after the flood the book dials %v first, want %s", got, directAddr(b))
		}
		b, t0 := restartAfter(t, nn, a, b, awayFor, 2, true, &p2p.MemBook{})
		expectBackWithin(t, a, b, t0, sleepingBound)
		if data := string(aBook.Data()); strings.Contains(data, "192.168.9.") {
			t.Errorf("the stored book holds forged addresses: %s", data)
		}
	})
}

// lanNode is the node of seed on the local network of the others: a private address, no NAT
// between them.
func (nn *natNet) lanNode(t *testing.T, seed byte, o p2p.Options) *simNode {
	t.Helper()
	ip := fmt.Sprintf("192.168.0.%d", seed)
	return nn.nodeOn(t, seed, &natEnd{sent: make(map[string]struct{}), public: true, internal: ip, external: ip}, o)
}

// TestReconnectOnLANSighting: a sighting of a wanted member on the local network wakes its dial
// loop, which does not wait for the end of its backoff (up to 75 s).
func TestReconnectOnLANSighting(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		b := nn.lanNode(t, 2, p2p.Options{})
		a := nn.node(t, 1, p2p.Options{})
		a.Want(map[transport.PeerID][]string{b.Local(): nil}) // no address: the loop backs off
		time.Sleep(awayFor)
		id, _ := p2p.LibP2PID(b.Local())
		a.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast(directAddr(b))}})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 5*time.Second)
	})
}

// TestBookPerDeviceAcrossProjects: security review 2, the book belongs to the device. A member of
// two projects keeps its entry when it is revoked in one of them, and a member dropped for a fault
// (still admitted) keeps it too.
func TestBookPerDeviceAcrossProjects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		bID := idOf(key(2))
		var scope atomic.Value
		scope.Store("A")
		var revoked atomic.Bool
		book := &p2p.MemBook{}
		a := nn.node(t, 1, p2p.Options{
			Book:  book,
			Scope: func() string { return scope.Load().(string) },
			Admit: func(id transport.PeerID) bool { return !revoked.Load() || id != bID },
		})
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		a.Want(map[transport.PeerID][]string{b.Local(): b.Addrs()})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), wait)
		scope.Store("B") // another project opened, b a member there too
		a.Want(map[transport.PeerID][]string{b.Local(): nil})

		a.Drop(bID) // a fault: b is still admitted
		expectEventWithin(t, a, transport.PeerDown, bID, wait)
		if got, _ := a.BookAddrs(bID); len(got) == 0 {
			t.Fatal("a Drop for a fault removed the member from the book")
		}
		revoked.Store(true) // revoked in project B only
		a.Drop(bID)
		if got, _ := a.BookAddrs(bID); len(got) == 0 || !strings.Contains(string(book.Data()), string(bID)) {
			t.Errorf("revoked in project B, the member of project A left the book (stored: %s)", book.Data())
		}
		scope.Store("A")
		a.Drop(bID) // revoked in A too
		if got, _ := a.BookAddrs(bID); len(got) != 0 || strings.Contains(string(book.Data()), string(bID)) {
			t.Errorf("revoked in every project, the member is still in the book: %v", got)
		}
	})
}

// TestDialFallbackOrder: security review 9, a round dials the book's authenticated addresses
// first, then, in an attempt of their own, those mDNS gave.
func TestDialFallbackOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		bID := idOf(key(2))
		auth, lan := "/ip4/1.2.9.1/udp/8000/quic-v1", "/ip4/192.168.0.9/udp/8000/quic-v1" // nobody there
		var mu sync.Mutex
		var dials [][]string
		o := p2p.Options{Book: bookFor(bID, auth)}
		p2p.SetOnDial(&o, func(to transport.PeerID, addrs []string) {
			mu.Lock()
			defer mu.Unlock()
			if to == bID {
				dials = append(dials, addrs)
			}
		})
		a := nn.node(t, 1, o)
		id, _ := p2p.LibP2PID(bID)
		a.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast(lan)}}) // not wanted yet: no wake
		a.Want(map[transport.PeerID][]string{bID: nil})
		time.Sleep(30 * time.Second)
		mu.Lock()
		defer mu.Unlock()
		if len(dials) < 2 || !slices.Equal(dials[0], []string{auth}) || !slices.Equal(dials[1], []string{lan}) {
			t.Errorf("first dial attempts = %v, want [%s] then [%s]", dials, auth, lan)
		}
	})
}

// TestReconnectUnderSightingFlood: second review 1, forged mDNS sightings of a member, however
// many, never postpone the normal attempts of its dial loop: once the member is reachable at its
// authenticated address it is reconnected within the sleeping bound, plus one LAN attempt that may
// be under way.
func TestReconnectUnderSightingFlood(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		bID := idOf(key(2))
		a := nn.node(t, 1, p2p.Options{Book: bookFor(bID, fmt.Sprintf("/ip4/1.2.2.1/udp/%d/quic-v1", simPort))})
		a.Want(map[transport.PeerID][]string{bID: nil})
		id, _ := p2p.LibP2PID(bID)
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				case <-time.After(time.Second):
					a.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast("/ip4/192.168.0.9/udp/8000/quic-v1")}}) // nobody there
				}
			}
		})
		time.Sleep(awayFor) // the loop's backoff is at its longest
		b := nn.nodeAt(t, 2, true, p2p.Options{})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), sleepingBound+10*time.Second)
		close(stop)
		wg.Wait()
	})
}

// TestReconnectRoamedSaturated: second review 2, a member whose 8 authenticated addresses all went
// stale (it moved) is still found by a sighting on the local network: mDNS addresses have places of
// their own.
func TestReconnectRoamedSaturated(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		bID := idOf(key(2))
		var stale []string
		for i := range 8 {
			stale = append(stale, fmt.Sprintf("/ip4/1.2.9.%d/udp/8000/quic-v1", i+1)) // nobody there
		}
		book := &p2p.MemBook{}
		book.Set(p2p.BookFile(time.Now(), map[transport.PeerID][]string{bID: stale}))
		a := nn.node(t, 1, p2p.Options{Book: book})
		a.Want(map[transport.PeerID][]string{bID: nil})
		b := nn.lanNode(t, 2, p2p.Options{})
		time.Sleep(awayFor)
		id, _ := p2p.LibP2PID(bID)
		a.SightLAN(peer.AddrInfo{ID: id, Addrs: []ma.Multiaddr{ma.StringCast(directAddr(b))}})
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 5*time.Second)
	})
}
