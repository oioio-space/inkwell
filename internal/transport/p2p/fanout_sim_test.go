package p2p_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	ma "github.com/multiformats/go-multiaddr"

	inktransport "github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	wire "github.com/oioio-space/inkwell/protocol"
)

// admitLog collects what a node's admission layer decided (block RF1).
type admitLog struct {
	mu   sync.Mutex
	evs  []p2p.AdmitEvent
	peak map[string]int // the most pending connections seen per origin
}

func (l *admitLog) observe(o *p2p.Options) {
	l.peak = map[string]int{}
	p2p.SetAdmitObserve(o, func(ev p2p.AdmitEvent) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.evs = append(l.evs, ev)
		if ev.Peer == "" {
			l.peak[ev.Origin] = max(l.peak[ev.Origin], ev.Pending)
		}
	})
}

// refused returns the refusals of direction dir (any if 0), as "origin: reason".
func (l *admitLog) refused(dir network.Direction) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, ev := range l.evs {
		if ev.Refused != "" && (dir == 0 || ev.Dir == dir) {
			out = append(out, fmt.Sprintf("%v %s%s: %s", ev.Dir, ev.Origin, ev.Peer, ev.Refused))
		}
	}
	return out
}

// origins returns the origins the inbound openings were charged to.
func (l *admitLog) origins() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[string]int{}
	for _, ev := range l.evs {
		if ev.Dir == network.DirInbound && ev.Peer == "" {
			out[ev.Origin]++
		}
	}
	return out
}

// waitPeers waits until n has announced every peer of want, for at most d; it returns the ones missing.
func waitPeers(t *testing.T, n *simNode, want []inktransport.PeerID, d time.Duration) []inktransport.PeerID {
	t.Helper()
	up := map[inktransport.PeerID]bool{}
	deadline := time.Now().Add(d)
	for len(up) < len(want) {
		ctx, cancel := context.WithDeadline(t.Context(), deadline)
		ev, err := n.Recv(ctx)
		cancel()
		if err != nil {
			break
		}
		if ev.Kind == inktransport.PeerUp {
			up[ev.Peer] = true
		}
	}
	var missing []inktransport.PeerID
	for _, p := range want {
		if !up[p] {
			missing = append(missing, p)
		}
	}
	return missing
}

// fanOut runs n members reachable only through one relay: half dial the node, the node dials the other half.
// All must reach the node within 60 s, every inbound circuit charged to the relay's PeerID, and the node must
// refuse nothing. upRule names the rule a missing member breaks.
func fanOut(t *testing.T, n int, upRule string) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(uint64(n))
		relays := nn.relay(t, p2p.RelayOptions{})
		nn.blockDirect(true)
		var log admitLog
		oa := p2p.Options{Relays: relays}
		log.observe(&oa)
		p2p.SetNoAutoPunch(&oa)
		a := nn.node(t, 1, oa)
		var all []inktransport.PeerID
		dialled := map[inktransport.PeerID][]string{}
		for i := range n {
			o := p2p.Options{Relays: relays}
			p2p.SetNoAutoPunch(&o)
			m := nn.node(t, byte(10+i), o)
			all = append(all, m.Local())
			if i%2 == 0 {
				m.Want(map[inktransport.PeerID][]string{a.Local(): nil}) // the member dials: an inbound circuit at a
			} else {
				dialled[m.Local()] = nil
			}
		}
		a.Want(dialled) // a dials: outbound circuits
		missing := waitPeers(t, a, all, time.Minute)
		// The causes first, then the verdict: an opening the node refused, or charged to the relay's address.
		relay := "relay:" + string(idOf(key(0)))
		for o, k := range log.origins() {
			if o != relay {
				t.Errorf("RF-1 circuit charged to relay IP: %d inbound openings charged to %s, not to the relay's PeerID", k, o)
			}
		}
		if r := log.refused(0); len(r) > 0 {
			t.Errorf("RF-5 rate shared by the relay's members: the node refused %d openings: %v", len(r), r[:min(len(r), 8)])
		}
		if len(missing) > 0 {
			t.Fatalf("%s: %d of %d members behind one relay not up within 60 s: %v", upRule, len(missing), n, missing)
		}
		_ = a.Close()
	})
}

// TestRelayFanOut10: ten members reachable only through one relay all reach the node, half of them dialling
// it, half dialled by it (spec §16.1 « connexions par IP (membres) », RF-1): more than the 7 libp2p's limit of
// 8 connections per IPv4 address let through when every circuit counted under the relay's address.
func TestRelayFanOut10(t *testing.T) { fanOut(t, 10, "RF-1 circuit charged to relay IP") }

// TestRelayFanOut17: seventeen members through one relay: above the relay's old 16 circuits per peer (RF-7)
// and libp2p's burst of 16 connections per IPv4 address, which a relay's members shared (RF-5).
func TestRelayFanOut17(t *testing.T) { fanOut(t, 17, "RF-7 relay circuits") }

// homeEnd is the end of the node behind the home router at ip, seen at port.
func homeEnd(seed byte, ip string, port int) *natEnd {
	return &natEnd{sent: map[string]struct{}{}, internal: fmt.Sprintf("10.0.%d.1", seed), external: ip, port: port}
}

// TestHomeRouterNine: nine members behind one IPv4 address connect directly to the node within 10 s (§16.1
// « connexions par IP (membres) », RF-2): each leaves its address's pending count at its handshake, so the
// eighth's place is free for the ninth well before the 15 s deadline would free it.
func TestHomeRouterNine(t *testing.T) {
	const rule = "RF-2 member still counted"
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(9)
		var log admitLog
		var oa p2p.Options
		log.observe(&oa)
		a := nn.nodeAt(t, 1, true, oa)
		addr := fmt.Sprintf("/ip4/%s/udp/%d/quic-v1/p2p/%s", a.end.external, simPort, a.Local())
		var all []inktransport.PeerID
		for i := range 9 {
			m := nn.nodeOn(t, byte(10+i), homeEnd(byte(10+i), "2.2.77.1", 9000+i), p2p.Options{})
			m.Want(map[inktransport.PeerID][]string{a.Local(): {addr}})
			all = append(all, m.Local())
		}
		if missing := waitPeers(t, a, all, 10*time.Second); len(missing) > 0 {
			t.Fatalf("%s: %d of 9 members behind one address not up within 10 s: %v; refusals %v", rule, len(missing), missing, log.refused(0))
		}
		if got := log.origins()["ip:2.2.77.1/32"]; got < 9 {
			t.Fatalf("%s: %d openings charged to the router's address, want 9 at least", rule, got)
		}
		_ = a.Close()
	})
}

// relayClient is a plain libp2p host on end, with a relay client and no resource limits of its own, connected to
// the relay at relayAddr.
func (nn *natNet) relayClient(t *testing.T, seed byte, end *natEnd, relayAddr string) (host.Host, peer.AddrInfo) {
	t.Helper()
	h := nn.relayClientNoDial(t, seed, end)
	ri, err := peer.AddrInfoFromString(relayAddr)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Connect(t.Context(), *ri); err != nil {
		t.Fatalf("host %d: connect to the relay: %v", seed, err)
	}
	return h, *ri
}

// relayClientNoDial is relayClient without its connection to the relay.
func (nn *natNet) relayClientNoDial(t *testing.T, seed byte, end *natEnd) host.Host {
	t.Helper()
	priv, err := crypto.UnmarshalEd25519PrivateKey(key(seed))
	if err != nil {
		t.Fatal(err)
	}
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.NoListenAddrs,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", end.internal, simPort)),
		libp2p.DisableIdentifyAddressDiscovery(),
		libp2p.ResourceManager(&network.NullResourceManager{}),
		libp2p.EnableRelay(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.DisableMetrics(),
		nn.quic(end),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// circuitDial opens one circuit from h to dest through the relay at relayAddr, outside h's swarm (no
// connection reuse), and returns the connection.
func circuitDial(ctx context.Context, h host.Host, relayAddr string, dest peer.ID) (transport.CapableConn, error) {
	a := ma.StringCast(relayAddr + "/p2p-circuit/p2p/" + dest.String())
	tpt := h.Network().(*swarm.Swarm).TransportForDialing(a)
	if tpt == nil {
		return nil, fmt.Errorf("no transport for %s", a)
	}
	return tpt.Dial(ctx, a, dest)
}

// TestStrangerFloodThroughRelay: strangers open circuits to the node through its members' relay as fast as
// it lets them. They never hold more than 8 pending places at the node (RF-3); the node's own dials to the
// members through that relay are never refused, since an outbound connection is charged to no origin (RF-2);
// and a member behind another origin still connects inbound (RF-1, RF-3).
func TestStrangerFloodThroughRelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(21)
		relays := nn.relay(t, p2p.RelayOptions{})
		nn.blockDirect(true)
		members := map[inktransport.PeerID]bool{}
		for i := range 4 {
			members[idOf(key(byte(10+i)))] = true
		}
		var log admitLog
		oa := p2p.Options{Relays: relays, Admit: func(p inktransport.PeerID) bool { return members[p] }}
		log.observe(&oa)
		p2p.SetNoAutoPunch(&oa)
		a := nn.nodeAt(t, 1, true, oa)
		self := func(p inktransport.PeerID) bool { return p == a.Local() }
		var relayed []inktransport.PeerID
		for i := range 3 {
			o := p2p.Options{Relays: relays, Admit: self}
			p2p.SetNoAutoPunch(&o)
			relayed = append(relayed, nn.node(t, byte(10+i), o).Local())
		}
		// The flood: four strangers' loops, each opening circuits to a one after the other.
		s, _ := nn.relayClient(t, 200, nn.newEnd(200, false), relays[0])
		aID, _ := p2p.LibP2PID(a.Local())
		ctx, stop := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for ctx.Err() == nil {
					if c, err := circuitDial(ctx, s, relays[0], aID); err == nil {
						_ = c.Close()
					}
					time.Sleep(100 * time.Millisecond)
				}
			})
		}
		time.Sleep(10 * time.Second) // the bucket is drained
		want := map[inktransport.PeerID][]string{}
		for _, p := range relayed {
			want[p] = nil
		}
		a.Want(want)
		direct := nn.node(t, 13, p2p.Options{Admit: self})
		direct.Want(map[inktransport.PeerID][]string{a.Local(): {fmt.Sprintf("/ip4/%s/udp/%d/quic-v1/p2p/%s", a.end.external, simPort, a.Local())}})
		missing := waitPeers(t, a, append(relayed, direct.Local()), time.Minute)
		stop()
		wg.Wait()
		if r := log.refused(network.DirOutbound); len(r) > 0 {
			t.Errorf("RF-2 outbound starved: the node's dials to its members were refused %d times: %v", len(r), r)
		}
		if len(missing) > 0 {
			t.Errorf("RF-2 outbound starved: members not up within 60 s of the flood: %v", missing)
		}
		relay := "relay:" + string(idOf(key(0)))
		log.mu.Lock()
		peak := log.peak[relay]
		log.mu.Unlock()
		if peak > wire.MaxPendingPerIP {
			t.Errorf("RF-3 origin quota: strangers held %d pending places at the node through the relay, want %d at most", peak, wire.MaxPendingPerIP)
		}
		rate := 0
		for _, r := range log.refused(network.DirInbound) {
			if strings.HasSuffix(r, ": rate") {
				rate++
			}
		}
		if rate == 0 {
			t.Errorf("RF-5: the flood was never refused for the rate (%d openings charged): the test floods nothing", log.origins()[relay])
		}
		_ = a.Close()
	})
}

// TestRelayCircuitsPerPeerBound: an inkwell relay carries 128 circuits to one peer (§16.1 « circuits par
// relais », RF-7); the 129th is refused with RESOURCE_LIMIT_EXCEEDED, and one closed lets the next through.
func TestRelayCircuitsPerPeerBound(t *testing.T) {
	const rule = "RF-7 relay circuits"
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(31)
		relays := nn.relay(t, p2p.RelayOptions{})
		nn.blockDirect(true)
		d, ri := nn.relayClient(t, 31, nn.newEnd(31, false), relays[0])
		if _, err := client.Reserve(t.Context(), d, ri); err != nil {
			t.Fatalf("reserve: %v", err)
		}
		s, _ := nn.relayClient(t, 32, nn.newEnd(32, false), relays[0])
		var conns []transport.CapableConn
		defer func() {
			for _, c := range conns {
				_ = c.Close()
			}
		}()
		for i := range wire.RelayMaxCircuitsPerPeer {
			c, err := circuitDial(t.Context(), s, relays[0], d.ID())
			if err != nil {
				t.Fatalf("%s: circuit %d refused: %v", rule, i+1, err)
			}
			conns = append(conns, c)
		}
		if c, err := circuitDial(t.Context(), s, relays[0], d.ID()); err == nil {
			_ = c.Close()
			t.Fatalf("%s: circuit %d admitted", rule, wire.RelayMaxCircuitsPerPeer+1)
		} else if !strings.Contains(err.Error(), "RESOURCE_LIMIT_EXCEEDED") {
			t.Fatalf("%s: circuit %d refused with %v, want RESOURCE_LIMIT_EXCEEDED", rule, wire.RelayMaxCircuitsPerPeer+1, err)
		}
		_ = conns[0].Close()
		conns = conns[1:]
		time.Sleep(time.Second) // the relay sees the circuit end
		c, err := circuitDial(t.Context(), s, relays[0], d.ID())
		if err != nil {
			t.Fatalf("%s: after a close, the next circuit refused: %v", rule, err)
		}
		conns = append(conns, c)
	})
}

// TestRelayReservationsPerIPBound: an inkwell relay keeps 64 reservations from one IPv4 address (§16.1
// « circuits par relais », RF-7); the 65th is refused, one released lets the next in. The clients are the
// relay's Allow list, so their connections leave the address's count at their handshake (RF-2).
func TestRelayReservationsPerIPBound(t *testing.T) {
	const rule = "RF-7 relay reservations per IP"
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(64)
		n := wire.RelayReservationsPerIP + 1
		var allow []inktransport.PeerID
		for i := range n {
			allow = append(allow, idOf(key(byte(100+i))))
		}
		relays := nn.relay(t, p2p.RelayOptions{Allow: allow})
		var hosts []host.Host
		var ri peer.AddrInfo
		for i := range n {
			h, r := nn.relayClient(t, byte(100+i), homeEnd(byte(100+i), "2.2.88.1", 9000+i), relays[0])
			hosts, ri = append(hosts, h), r
		}
		for i, h := range hosts[:n-1] {
			if _, err := client.Reserve(t.Context(), h, ri); err != nil {
				t.Fatalf("%s: reservation %d refused: %v", rule, i+1, err)
			}
		}
		last := hosts[n-1]
		if _, err := client.Reserve(t.Context(), last, ri); err == nil {
			t.Fatalf("%s: reservation %d from one address admitted", rule, n)
		}
		_ = hosts[0].Close() // its connection ends: the relay drops its reservation
		time.Sleep(time.Second)
		if _, err := client.Reserve(t.Context(), last, ri); err != nil {
			t.Fatalf("%s: after a release, the next reservation refused: %v", rule, err)
		}
	})
}

// Review finding 2 (gates/RF1/CODE-REVIEW.md): at a relay with an Allow list, a peer that is not listed holds no
// connection place: it may neither reserve nor connect (the circuit-v2 ACL), so its authenticated connection is
// closed at once, and it cannot fill the 64 places of an address its members share (RF-7, RF-2).
func TestRelayUnlistedHoldsNoPlace(t *testing.T) {
	const rule = "RF-7 relay per-IP places"
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(65)
		member := idOf(key(41))
		var log admitLog
		ro := p2p.RelayOptions{Allow: []inktransport.PeerID{member}}
		log.peak = map[string]int{}
		p2p.SetRelayAdmitObserve(&ro, func(ev p2p.AdmitEvent) {
			log.mu.Lock()
			defer log.mu.Unlock()
			log.evs = append(log.evs, ev)
			if ev.Peer == "" {
				log.peak[ev.Origin] = max(log.peak[ev.Origin], ev.Pending)
			}
		})
		relays := nn.relay(t, ro)
		ri, err := peer.AddrInfoFromString(relays[0])
		if err != nil {
			t.Fatal(err)
		}
		// The stranger opens a connection every second for six minutes, from the members' address: the bucket
		// lets one through per 5 s, more than the 64 places in that time.
		s := nn.relayClientNoDial(t, 40, homeEnd(40, "2.2.99.1", 9000))
		tpt := s.Network().(*swarm.Swarm).TransportForDialing(ri.Addrs[0])
		var held []transport.CapableConn
		ctx, stop := context.WithCancel(t.Context())
		var wg sync.WaitGroup
		for range 360 {
			if c, err := tpt.Dial(ctx, ri.Addrs[0], ri.ID); err == nil {
				held = append(held, c)
				wg.Go(func() { // an attacker keeps its connections alive
					for ctx.Err() == nil && !c.IsClosed() {
						if s, err := c.OpenStream(ctx); err == nil {
							_ = s.Reset()
						}
						time.Sleep(10 * time.Second)
					}
				})
			}
			time.Sleep(time.Second)
		}
		time.Sleep(time.Second)
		log.mu.Lock()
		peak := log.peak["ip:2.2.99.1/32"]
		log.mu.Unlock()
		if peak > 1 { // it dials one connection at a time: only that one's handshake may hold a place
			t.Errorf("%s: the stranger held %d of its address's places at the relay, want its handshake's only", rule, peak)
		}
		// Then, the stranger still connected, the listed member behind the same address reserves, retrying each
		// second: the stranger drained the address's bucket (RF-5), which gives a token within 5 s; a place the
		// stranger held would not free while it stays connected.
		h := nn.relayClientNoDial(t, 41, homeEnd(41, "2.2.99.1", 9001))
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
			if err = h.Connect(t.Context(), *ri); err == nil {
				if _, err = client.Reserve(t.Context(), h, *ri); err == nil {
					break
				}
			}
			h.Network().(*swarm.Swarm).Backoff().Clear(ri.ID)
		}
		if err != nil {
			t.Fatalf("%s: the listed member's reservation still refused for 30 s while the stranger keeps its connections from the address (%d places held): %v", rule, peak, err)
		}
		stop()
		for _, c := range held {
			_ = c.Close()
		}
		wg.Wait()
	})
}
