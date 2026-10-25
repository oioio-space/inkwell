package p2p_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	"github.com/marcopolo/simnet"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/quic-go/quic-go"
	"go.uber.org/fx"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// latencySeeds is how many seeds of network latency each reachability test
// runs on: packets are delayed by a few milliseconds that differ per packet,
// so the order in which they cross changes from seed to seed.
const latencySeeds = 20

// The simulated internet. The relay is on a public address; a node is behind
// a NAT that keeps its port, translates its address, and lets a packet in only
// from an address it has already sent one to (the firewall of
// simnet.SimpleFirewallRouter, which the hole punching tests of go-libp2p use,
// plus latency, a way to cut the network and a way to change a node's address).
const (
	relayIP   = "1.2.0.1"
	simPort   = 8000
	baseDelay = 3 * time.Millisecond
	jitterMax = 20 * time.Millisecond
)

// natEnd is one endpoint of the simulated network.
type natEnd struct {
	conn     *simnet.SimConn
	sent     map[string]struct{} // external addresses this end has sent to
	internal string              // IP the node listens on
	external string              // IP the others see
	public   bool                // no firewall
}

// natNet is a simulated UDP network; use it inside a synctest bubble.
type natNet struct {
	rng    *rand.Rand
	byInt  map[string]*natEnd
	byExt  map[string]*natEnd
	mu     sync.Mutex
	nextIP int
	cut    bool
}

// newNatNet returns a network whose latencies come from seed.
func newNatNet(seed uint64) *natNet {
	return &natNet{
		rng:   rand.New(rand.NewPCG(seed, 1)), // #nosec G404 -- latencies must replay from a seed.
		byInt: make(map[string]*natEnd),
		byExt: make(map[string]*natEnd),
	}
}

// forSeeds runs f on a network per latency seed, each in its own synctest bubble.
func forSeeds(t *testing.T, f func(t *testing.T, nn *natNet)) {
	t.Helper()
	for seed := range latencySeeds {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { f(t, newNatNet(uint64(seed))) })
		})
	}
}

// RecvPacket routes a packet sent by a node: translate its source, apply the
// firewall of the destination, delay it.
func (nn *natNet) RecvPacket(p *simnet.Packet) {
	nn.mu.Lock()
	src := nn.byInt[p.From.String()]
	if src == nil || nn.cut {
		nn.mu.Unlock()
		return
	}
	dstExt := p.To.String()
	p.From = udpAddr(fmt.Sprintf("%s:%d", src.external, simPort))
	src.sent[dstExt] = struct{}{}
	dst := nn.byExt[dstExt]
	replied := dst.sentTo(p.From.String())
	delay := baseDelay + time.Duration(nn.rng.Int64N(int64(jitterMax)))
	nn.mu.Unlock()

	if dst == nil || (!dst.public && !replied) {
		return
	}
	time.AfterFunc(delay, func() { dst.conn.RecvPacket(p) })
}

// sentTo reports whether the end has sent a packet to addr (nil-safe).
func (e *natEnd) sentTo(addr string) bool {
	if e == nil {
		return false
	}
	_, ok := e.sent[addr]
	return ok
}

func udpAddr(s string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		panic(err)
	}
	return a
}

// setCut loses every packet, or stops doing so.
func (nn *natNet) setCut(cut bool) {
	nn.mu.Lock()
	nn.cut = cut
	nn.mu.Unlock()
}

// roam gives the node behind end a new public address and forgets what its
// NAT let in, as moving to another network does. The node is not told.
func (nn *natNet) roam(end *natEnd) {
	nn.mu.Lock()
	defer nn.mu.Unlock()
	nn.nextIP++
	delete(nn.byExt, fmt.Sprintf("%s:%d", end.external, simPort))
	end.external = fmt.Sprintf("3.3.%d.1", nn.nextIP)
	end.sent = make(map[string]struct{})
	nn.byExt[fmt.Sprintf("%s:%d", end.external, simPort)] = end
}

// quic returns the libp2p option that puts QUIC on the end: the listening
// socket is a simulated one wired to nn.
func (nn *natNet) quic(end *natEnd) libp2p.Option {
	selector := &mockSource{}
	return libp2p.QUICReuse(
		func(l fx.Lifecycle, reset quic.StatelessResetKey, token quic.TokenGeneratorKey, opts ...quicreuse.Option) (*quicreuse.ConnManager, error) {
			cm, err := quicreuse.NewConnManager(reset, token, opts...)
			if err != nil {
				return nil, err
			}
			l.Append(fx.StopHook(cm.Close))
			return cm, nil
		},
		quicreuse.OverrideSourceIPSelector(func() (quicreuse.SourceIPSelector, error) { return selector, nil }),
		quicreuse.OverrideListenUDP(func(_ string, address *net.UDPAddr) (net.PacketConn, error) {
			selector.ip.Store(&address.IP)
			c := simnet.NewBlockingSimConn(address)
			c.SetUpPacketReceiver(nn)
			nn.mu.Lock()
			defer nn.mu.Unlock()
			end.conn = c
			nn.byInt[address.String()] = end
			nn.byExt[fmt.Sprintf("%s:%d", end.external, simPort)] = end
			return c, nil
		}),
	)
}

// mockSource tells QUIC which address to send from.
type mockSource struct{ ip atomic.Pointer[net.IP] }

func (m *mockSource) PreferredSourceIPForDestination(*net.UDPAddr) (net.IP, error) {
	return *m.ip.Load(), nil
}

// newEnd returns an endpoint for the node of seed. A private one has a
// private address behind a NAT; a public one has the address ip.
func (nn *natNet) newEnd(seed byte, public bool) *natEnd {
	end := &natEnd{sent: make(map[string]struct{}), public: public}
	if public {
		end.internal = fmt.Sprintf("1.2.%d.1", seed)
		end.external = end.internal
		return end
	}
	end.internal = fmt.Sprintf("10.0.%d.1", seed)
	end.external = fmt.Sprintf("2.2.%d.1", seed)
	return end
}

// simNode is a node of the simulated network, with its endpoint.
type simNode struct {
	*p2p.Node
	end *natEnd
}

// node returns the node of seed behind a NAT, advertising its public address.
// o.Relays and o.RelayOnly are the caller's. The seeds of the nodes of a test
// differ, and 0 is the relay.
func (nn *natNet) node(t *testing.T, seed byte, o p2p.Options) *simNode {
	t.Helper()
	return nn.nodeAt(t, seed, false, o)
}

// nodeAt is node, behind a NAT unless public: then the node is reachable by
// anyone, at 1.2.<seed>.1.
func (nn *natNet) nodeAt(t *testing.T, seed byte, public bool, o p2p.Options) *simNode {
	t.Helper()
	end := nn.newEnd(seed, public)
	p2p.SetRewrite(&o, func(addrs []ma.Multiaddr) []ma.Multiaddr {
		out := make([]ma.Multiaddr, 0, len(addrs))
		for _, a := range addrs {
			if _, err := a.ValueForProtocol(ma.P_CIRCUIT); err == nil {
				out = append(out, a)
			}
		}
		nn.mu.Lock()
		ext := end.external
		nn.mu.Unlock()
		return append(out, ma.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", ext, simPort)))
	})
	p2p.SetExtra(
		&o,
		libp2p.NoListenAddrs,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", end.internal, simPort)),
		libp2p.DisableIdentifyAddressDiscovery(), // it stalls synctest
		nn.quic(end),
	)
	n, err := p2p.New(t.Context(), key(seed), o)
	if err != nil {
		t.Fatalf("New(seed %d): %v", seed, err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return &simNode{Node: n, end: end}
}

// relay runs a relay on the public address, and returns the address to pass as
// Options.Relays. The relay stops with the test.
func (nn *natNet) relay(t *testing.T, o p2p.RelayOptions) []string {
	t.Helper()
	end := nn.newEnd(0, true)
	o.Listen = []string{fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", relayIP, simPort)}
	ready := make(chan []string, 1)
	o.OnReady = func(addrs []string) { ready <- addrs }
	p2p.SetRelayExtra(
		&o,
		libp2p.NoListenAddrs,
		libp2p.ListenAddrStrings(o.Listen...),
		libp2p.DisableIdentifyAddressDiscovery(),
		nn.quic(end),
	)
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() {
		if err := p2p.RunRelay(ctx, key(0), o); err != nil {
			t.Errorf("RunRelay: %v", err)
		}
	})
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	addrs := <-ready
	if want := fmt.Sprintf("/ip4/%s/udp/%d/quic-v1/p2p/%s", relayIP, simPort, idOf(key(0))); len(addrs) != 1 || addrs[0] != want {
		t.Fatalf("RunRelay addresses = %v, want [%s]", addrs, want)
	}
	return addrs
}

// linkTo returns the kind of the link from n to peer, or 0 if there is none.
func linkTo(n *simNode, peer transport.PeerID) p2p.LinkKind {
	for _, l := range n.Links() {
		if l.Peer == peer {
			return l.Kind
		}
	}
	return 0
}

// waitLink waits until n reaches peer with the kind of link.
func waitLink(t *testing.T, n *simNode, peer transport.PeerID, kind p2p.LinkKind, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for linkTo(n, peer) != kind {
		if time.Now().After(deadline) {
			t.Fatalf("%s: link to %s is %v after %v, want %v", n.Local(), peer, linkTo(n, peer), d, kind)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// expectEventWithin is expectEvent with its own bound on the wait.
func expectEventWithin(t *testing.T, n transport.Transport, kind transport.Kind, peer transport.PeerID, d time.Duration) {
	t.Helper()
	got, want := recvWithin(t, n, d), transport.Event{Kind: kind, Peer: peer}
	if got.Kind != want.Kind || got.Peer != want.Peer {
		t.Errorf("Recv(%s) = %v from %s, want %v from %s", n.Local(), got.Kind, got.Peer, want.Kind, want.Peer)
	}
}

// wantEachOther makes each node keep the other connected, as the app does.
func wantEachOther(a, b *simNode) {
	a.Want(map[transport.PeerID][]string{b.Local(): nil})
	b.Want(map[transport.PeerID][]string{a.Local(): nil})
}

func TestRelayThenDirect(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		a.Want(map[transport.PeerID][]string{b.Local(): nil})

		// The two cannot reach each other: the first link is through the relay.
		expectUp(t, a, b)
		if got := linkTo(a, b.Local()); got != p2p.LinkRelayed {
			t.Fatalf("first link a->b = %v, want %v", got, p2p.LinkRelayed)
		}

		// Frames flow while the link is upgraded: none is lost or reordered,
		// and nobody goes down.
		var wg sync.WaitGroup
		var sent, received atomic.Uint64
		stop := make(chan struct{})
		wg.Go(func() { // a: nothing to receive, and no event
			for {
				ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
				ev, err := a.Recv(ctx)
				cancel()
				if err != nil {
					return
				}
				t.Errorf("a got %v from %s during the upgrade", ev.Kind, ev.Peer)
			}
		})
		wg.Go(func() {
			for next := uint64(0); ; next++ {
				ev, err := b.Recv(t.Context())
				if err != nil {
					return
				}
				if ev.Kind != transport.Message || len(ev.Payload) != 8 {
					t.Errorf("b got %v with %d bytes during the upgrade, want a Message of 8", ev.Kind, len(ev.Payload))
					return
				}
				if got := binary.BigEndian.Uint64(ev.Payload); got != next {
					t.Errorf("b got frame %d, want %d", got, next)
					return
				}
				received.Add(1)
				select {
				case <-stop:
					if received.Load() == sent.Load() {
						return
					}
				default:
				}
			}
		})

		var buf [8]byte
		directSince := time.Time{}
		deadline := time.Now().Add(time.Minute)
		for directSince.IsZero() || time.Since(directSince) < 3*time.Second {
			if time.Now().After(deadline) {
				t.Fatalf("links after a minute: a->b %v, b->a %v, want %v", linkTo(a, b.Local()), linkTo(b, a.Local()), p2p.LinkDirect)
			}
			binary.BigEndian.PutUint64(buf[:], sent.Load())
			if err := a.Send(t.Context(), b.Local(), buf[:]); err != nil {
				t.Fatalf("Send frame %d: %v", sent.Load(), err)
			}
			sent.Add(1)
			if directSince.IsZero() && linkTo(a, b.Local()) == p2p.LinkDirect && linkTo(b, a.Local()) == p2p.LinkDirect {
				directSince = time.Now()
			}
			time.Sleep(20 * time.Millisecond)
		}
		close(stop)
		time.Sleep(time.Second)
		if got, want := received.Load(), sent.Load(); got != want {
			t.Errorf("b received %d frames, a sent %d", got, want)
		}
		for _, n := range []*simNode{a, b} {
			if direct, ok := n.OutDirect(map[*simNode]transport.PeerID{a: b.Local(), b: a.Local()}[n]); !ok || !direct {
				t.Errorf("%s: outbound stream direct = %v (open %v), want a stream on the direct connection", n.Local(), direct, ok)
			}
		}
		_ = a.Close()
		_ = b.Close()
		wg.Wait()
	})
}

func TestPartitionThenHeal(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		wantEachOther(a, b)
		expectUp(t, a, b)
		waitLink(t, a, b.Local(), p2p.LinkDirect, time.Minute)

		// Longer than QUIC's idle timeout (30 s): each side concludes that the
		// other is gone.
		nn.setCut(true)
		expectEventWithin(t, a, transport.PeerDown, b.Local(), time.Minute)
		expectEventWithin(t, b, transport.PeerDown, a.Local(), time.Minute)
		time.Sleep(20 * time.Second)

		healed := time.Now()
		nn.setCut(false)
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 30*time.Second)
		expectEventWithin(t, b, transport.PeerUp, a.Local(), 30*time.Second)
		if d := time.Since(healed); d > 30*time.Second {
			t.Errorf("PeerUp %v after the network healed, want at most 30s", d)
		}

		// One PeerUp, not several; and the data flows.
		expectQuietFor(t, a, 20*time.Second)
		expectQuietFor(t, b, 20*time.Second)
		if err := a.Send(t.Context(), b.Local(), []byte("again")); err != nil {
			t.Fatalf("Send after heal: %v", err)
		}
		if got := recvWithin(t, b, wait); string(got.Payload) != "again" {
			t.Errorf("b got %q, want again", got.Payload)
		}
	})
}

func TestRoamToNewAddress(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		// The app persists the last addresses it saw; a has b's old ones.
		a.Want(map[transport.PeerID][]string{b.Local(): b.Addrs()})
		b.Want(map[transport.PeerID][]string{a.Local(): a.Addrs()})
		expectUp(t, a, b)
		waitLink(t, a, b.Local(), p2p.LinkDirect, time.Minute)

		// b is offline for two minutes and comes back on another network.
		nn.setCut(true)
		expectEventWithin(t, a, transport.PeerDown, b.Local(), time.Minute)
		expectEventWithin(t, b, transport.PeerDown, a.Local(), time.Minute)
		time.Sleep(2 * time.Minute)
		nn.roam(b.end)

		healed := time.Now()
		nn.setCut(false)
		expectEventWithin(t, a, transport.PeerUp, b.Local(), 30*time.Second)
		expectEventWithin(t, b, transport.PeerUp, a.Local(), 30*time.Second)
		if d := time.Since(healed); d > 30*time.Second {
			t.Errorf("PeerUp %v after b came back, want at most 30s", d)
		}
		// Its old address is useless: the way back is through the relay.
		if got := linkTo(a, b.Local()); got != p2p.LinkRelayed {
			t.Errorf("link a->b right after the return = %v, want %v", got, p2p.LinkRelayed)
		}
		expectQuietFor(t, a, 10*time.Second)
		expectQuietFor(t, b, 10*time.Second)
		waitLink(t, a, b.Local(), p2p.LinkDirect, time.Minute) // and the new address is punched
	})
}

func TestRelayOnlyRefusesDirect(t *testing.T) {
	forSeeds(t, func(t *testing.T, nn *natNet) {
		relays := nn.relay(t, p2p.RelayOptions{})
		a := nn.node(t, 1, p2p.Options{Relays: relays, RelayOnly: true})
		b := nn.node(t, 2, p2p.Options{Relays: relays, RelayOnly: true})
		wantEachOther(a, b)
		expectUp(t, a, b)
		if len(a.Addrs()) == 0 || len(b.Addrs()) == 0 {
			t.Errorf("RelayOnly nodes advertise %v and %v once reserved, want circuit addresses", a.Addrs(), b.Addrs())
		}
		for _, addr := range append(a.Addrs(), b.Addrs()...) {
			if _, err := ma.StringCast(addr).ValueForProtocol(ma.P_CIRCUIT); err != nil {
				t.Errorf("RelayOnly node advertises %s, want circuit addresses only", addr)
			}
		}

		// No direct connection is ever made: no hole punching, and a direct
		// dial is refused by the gate.
		bDirect := fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", b.end.external, simPort)
		if err := a.Connect(t.Context(), b.Local(), []string{bDirect}); err != nil {
			// Already connected through the relay: the dial is not even tried.
			t.Logf("Connect(direct) = %v", err)
		}
		time.Sleep(30 * time.Second)
		for _, n := range []*simNode{a, b} {
			other := b.Local()
			if n == b {
				other = a.Local()
			}
			if got := linkTo(n, other); got != p2p.LinkRelayed {
				t.Errorf("%s: link = %v, want %v", n.Local(), got, p2p.LinkRelayed)
			}
			for _, c := range n.Host().Network().Conns() {
				if _, err := c.RemoteMultiaddr().ValueForProtocol(ma.P_CIRCUIT); err != nil && c.RemotePeer().String() != string(idOf(key(0))) {
					t.Errorf("%s has a direct connection to %s", n.Local(), c.RemotePeer())
				}
			}
		}

		// A circuit is not limited to 2 minutes or 128 KiB (libp2p's default).
		big := make([]byte, 1<<20)
		if err := a.Send(t.Context(), b.Local(), big); err != nil {
			t.Fatalf("Send 1 MiB: %v", err)
		}
		if got := recvWithin(t, b, wait); len(got.Payload) != len(big) {
			t.Errorf("b got %d bytes, want %d", len(got.Payload), len(big))
		}
		time.Sleep(3 * time.Minute)
		if err := b.Send(t.Context(), a.Local(), []byte("late")); err != nil {
			t.Fatalf("Send after 3 minutes: %v", err)
		}
		if got := recvWithin(t, a, wait); string(got.Payload) != "late" {
			t.Errorf("a got %q, want late", got.Payload)
		}
		expectQuietFor(t, a, quiet)
		expectQuietFor(t, b, quiet)
	})
}

// TestRelayOnlyGate: a RelayOnly node is reachable only through its relays.
func TestRelayOnlyGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{})
		hidden := nn.nodeAt(t, 2, true, p2p.Options{Relays: relays, RelayOnly: true})
		open := nn.nodeAt(t, 3, true, p2p.Options{Relays: relays})
		stranger := nn.nodeAt(t, 4, true, p2p.Options{})
		direct := func(n *simNode) []string {
			return []string{fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", n.end.external, simPort)}
		}

		// Control: an ordinary node that is reachable is connected to directly.
		if err := stranger.Connect(t.Context(), open.Local(), direct(open)); err != nil {
			t.Fatalf("Connect(open node) = %v", err)
		}
		expectUp(t, stranger, open)

		// The hidden node refuses a direct connection, though it could take it.
		_ = stranger.Connect(t.Context(), hidden.Local(), direct(hidden)) // may seem to succeed on its side
		expectQuietFor(t, hidden, 5*time.Second)
		expectQuietFor(t, stranger, quiet)
		if conns := hidden.Host().Network().Conns(); len(conns) != 1 || conns[0].RemotePeer().String() != string(idOf(key(0))) {
			t.Errorf("hidden node has connections %v, want only the one to its relay", conns)
		}

		// And it does not dial a direct address either.
		if err := hidden.Connect(t.Context(), open.Local(), direct(open)); err == nil {
			t.Error("RelayOnly node dialed a direct address")
		}
		if got := open.Peers(); len(got) != 1 || got[0] != stranger.Local() {
			t.Errorf("open.Peers() = %v after the hidden node dialed it, want only the stranger", got)
		}

		// Through the relay it is reachable, and nothing else changes.
		stranger.Want(map[transport.PeerID][]string{hidden.Local(): direct(hidden)})
		hidden.Want(map[transport.PeerID][]string{open.Local(): direct(open)})
		expectUp(t, open, hidden)
		for _, n := range []*simNode{hidden, open} {
			if got := n.Links(); len(got) == 0 {
				t.Errorf("%s has no link", n.Local())
			}
		}
		waitLink(t, open, hidden.Local(), p2p.LinkRelayed, time.Second)
		time.Sleep(30 * time.Second) // time to hole punch, if it were to
		if got := linkTo(open, hidden.Local()); got != p2p.LinkRelayed {
			t.Errorf("link open->hidden after 30s = %v, want %v", got, p2p.LinkRelayed)
		}
	})
}

// TestRelayAllowlist: a relay with an allowlist serves the listed peers only.
func TestRelayAllowlist(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		relays := nn.relay(t, p2p.RelayOptions{Allow: []transport.PeerID{idOf(key(1)), idOf(key(2))}})
		a := nn.node(t, 1, p2p.Options{Relays: relays})
		b := nn.node(t, 2, p2p.Options{Relays: relays})
		c := nn.node(t, 3, p2p.Options{Relays: relays})
		wantEachOther(a, b)
		c.Want(map[transport.PeerID][]string{a.Local(): nil})
		expectUp(t, a, b)
		expectQuietFor(t, c, time.Minute)
		expectQuietFor(t, a, quiet)
		if got := c.Peers(); len(got) != 0 {
			t.Errorf("c.Peers() = %v, want none: the relay does not serve it", got)
		}
	})
}
