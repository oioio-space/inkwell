package p2p_test

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch/pb"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-msgio/pbio"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

// Helpers of the block HP1 tests (upgrade_sim_test.go): a restart of a node on its own network, the
// time a node's /libp2p/dcutr handler appears, a counting hole-punch hook, and a raw libp2p member that
// plays DCUtR by hand (a flood, a stall, twenty announced addresses).

// restart closes n and starts its identity again on the same network (the NAT keeps its public address;
// what it let in is forgotten), as a device whose process restarts. The new node wants the same peers.
func (nn *natNet) restart(t *testing.T, n *simNode, seed byte, o p2p.Options, want ...*simNode) *simNode {
	t.Helper()
	_ = n.Close()
	end := nn.newEnd(seed, n.end.public)
	m := nn.nodeOn(t, seed, end, o)
	for _, w := range want {
		m.Want(map[transport.PeerID][]string{w.Local(): nil})
	}
	return m
}

// dcutrReady returns a channel closed once n's /libp2p/dcutr handler is registered.
func dcutrReady(t *testing.T, h host.Host) <-chan time.Time {
	t.Helper()
	ch := make(chan time.Time, 1)
	if slices.Contains(h.Mux().Protocols(), holepunch.Protocol) {
		ch <- time.Now()
		return ch
	}
	sub, err := h.EventBus().Subscribe(new(event.EvtLocalProtocolsUpdated))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer func() { _ = sub.Close() }()
		for {
			select {
			case ev, ok := <-sub.Out():
				if !ok {
					return
				}
				if slices.Contains(ev.(event.EvtLocalProtocolsUpdated).Added, holepunch.Protocol) {
					ch <- time.Now()
					return
				}
			case <-t.Context().Done():
				return
			}
		}
	}()
	return ch
}

// callLog counts the upgrader's calls through SetPuncher: their start times, how many run at once at
// most, and whether each was made with an inbound circuit.
type callLog struct {
	mu        sync.Mutex
	starts    []time.Time
	busy      int // Busy results: they are not charged
	inFlight  int
	maxFlight int
	noInbound int // calls made without an inbound circuit to the peer
}

func (l *callLog) begin(in bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.starts = append(l.starts, time.Now())
	l.inFlight++
	l.maxFlight = max(l.maxFlight, l.inFlight)
	if !in {
		l.noInbound++
	}
}

func (l *callLog) end(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inFlight--
	if err != nil && strings.Contains(err.Error(), holepunch.ErrHolePunchActive.Error()) {
		l.busy++
	}
}

func (l *callLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.starts)
}

// maxInWindow returns the most calls that started within any window of length w.
func (l *callLog) maxInWindow(w time.Duration) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	best := 0
	for i := range l.starts {
		n := 0
		for j := i; j < len(l.starts) && l.starts[j].Sub(l.starts[i]) <= w; j++ {
			n++
		}
		best = max(best, n)
	}
	return best
}

// realPuncher records each call of *node's upgrader in l and makes it with the host's own service.
func realPuncher(l *callLog, node **simNode) func(ctx context.Context, to transport.PeerID) error {
	return func(ctx context.Context, to transport.PeerID) error {
		in, _ := (*node).Circuits(to)
		l.begin(in > 0)
		err := (*node).RealPunch(ctx, to)
		l.end(err)
		return err
	}
}

// rawMember is a libp2p host that is a member of the project to the node it meets (or a stranger, as
// the node's Admit says) but plays DCUtR by hand: it never runs inkwell's upgrader.
type rawMember struct {
	h    host.Host
	id   transport.PeerID
	end  *natEnd
	addr string // its address through the relay, for Want and Connect
	// connects counts the CONNECTs of the node that carried at least one address: its replies to exchange,
	// and, once answer is set, those of the rounds the node initiates.
	connects atomic.Int64
}

// rawMember starts a raw member of seed behind a NAT with a reservation on relay. announce, if not nil,
// replaces the addresses its identify announces.
func (nn *natNet) rawMember(t *testing.T, seed byte, relays []string, announce []ma.Multiaddr) *rawMember {
	t.Helper()
	end := nn.newEnd(seed, false)
	priv, err := crypto.UnmarshalEd25519PrivateKey(key(seed))
	if err != nil {
		t.Fatal(err)
	}
	opts := []libp2p.Option{
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
		libp2p.AddrsFactory(func(addrs []ma.Multiaddr) []ma.Multiaddr {
			if announce != nil {
				return append(slices.DeleteFunc(slices.Clone(addrs), func(a ma.Multiaddr) bool { _, err := a.ValueForProtocol(ma.P_CIRCUIT); return err != nil }), announce...)
			}
			return []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", end.external, simPort))}
		}),
	}
	h, err := libp2p.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	// Acknowledge the node's sync stream before reading its frames (PU1, P-3).
	h.SetStreamHandler(p2p.ProtocolSync, func(s network.Stream) {
		defer func() { _ = s.Reset() }()
		if syncack.WriteAck(s) == nil {
			_, _ = io.Copy(io.Discard, s)
		}
	})
	ri, err := peer.AddrInfoFromString(relays[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Connect(t.Context(), *ri); err != nil {
		t.Fatalf("raw member %d: connect to the relay: %v", seed, err)
	}
	if _, err := client.Reserve(t.Context(), h, *ri); err != nil {
		t.Fatalf("raw member %d: reserve: %v", seed, err)
	}
	return &rawMember{
		h: h, id: idOf(key(seed)), end: end,
		addr: fmt.Sprintf("%s/p2p-circuit/p2p/%s", relays[0], h.ID()),
	}
}

// dial connects the member to a node at addr (through the relay): the node then holds an inbound circuit.
func (m *rawMember) dial(ctx context.Context, addr string) error {
	ai, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return err
	}
	// Through the relay only, and at once: forget other addresses and the swarm's backoff of a failure.
	m.h.Peerstore().ClearAddrs(ai.ID)
	if s, ok := m.h.Network().(interface{ Backoff() *swarm.DialBackoff }); ok {
		s.Backoff().Clear(ai.ID)
	}
	return m.h.Connect(network.WithAllowLimitedConn(ctx, "relayed"), *ai)
}

// answer makes the member take the node's own DCUtR rounds (the node holds an inbound circuit from it and
// initiates): it reads the node's CONNECT, counts it in connects if it carried an address, and resets the
// stream, giving no address of its own.
func (m *rawMember) answer() {
	m.h.SetStreamHandler(holepunch.Protocol, func(s network.Stream) {
		defer func() { _ = s.Reset() }()
		var msg pb.HolePunch
		if err := pbio.NewDelimitedReader(s, 4096).ReadMsg(&msg); err != nil || msg.GetType() != pb.HolePunch_CONNECT {
			return
		}
		if slices.ContainsFunc(msg.ObsAddrs, func(b []byte) bool { _, err := ma.NewMultiaddrBytes(b); return err == nil }) {
			m.connects.Add(1)
		}
	})
}

// exchange opens a DCUtR stream to the node and sends a CONNECT with addrs; it reads the node's CONNECT
// and counts it if it carried an address. With stall it then waits, sending no SYNC, until the node
// gives up (its stream deadline); otherwise it sends SYNC and returns.
func (m *rawMember) exchange(ctx context.Context, to transport.PeerID, addrs []ma.Multiaddr, stall bool) error {
	_, _, err := m.round(ctx, to, addrs, stall)
	return err
}

// round is exchange that also returns the node's addresses and the time from CONNECT to its reply.
func (m *rawMember) round(ctx context.Context, to transport.PeerID, addrs []ma.Multiaddr, stall bool) ([]ma.Multiaddr, time.Duration, error) {
	p, ok := p2p.LibP2PID(to)
	if !ok {
		return nil, 0, fmt.Errorf("bad peer %s", to)
	}
	sctx := network.WithAllowLimitedConn(network.WithNoDial(ctx, "relayed"), "dcutr")
	s, err := m.h.NewStream(sctx, p, holepunch.Protocol)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = s.Reset() }()
	stop := context.AfterFunc(ctx, func() { _ = s.Reset() })
	defer stop()
	w := pbio.NewDelimitedWriter(s)
	r := pbio.NewDelimitedReader(s, 4096)
	bs := make([][]byte, len(addrs))
	for i, a := range addrs {
		bs[i] = a.Bytes()
	}
	sent := time.Now()
	if err := w.WriteMsg(&pb.HolePunch{Type: pb.HolePunch_CONNECT.Enum(), ObsAddrs: bs}); err != nil {
		return nil, 0, err
	}
	var msg pb.HolePunch
	if err := r.ReadMsg(&msg); err != nil {
		return nil, 0, err
	}
	rtt := time.Since(sent)
	var theirs []ma.Multiaddr
	for _, b := range msg.ObsAddrs {
		if a, err := ma.NewMultiaddrBytes(b); err == nil {
			theirs = append(theirs, a)
		}
	}
	if msg.GetType() == pb.HolePunch_CONNECT && len(theirs) > 0 {
		m.connects.Add(1)
	}
	if stall {
		_ = r.ReadMsg(&msg) // until the node's deadline resets the stream
		return theirs, rtt, nil
	}
	if err := w.WriteMsg(&pb.HolePunch{Type: pb.HolePunch_SYNC.Enum()}); err != nil {
		return nil, 0, err
	}
	// Close, not reset, and wait for the node's close: a reset would discard the SYNC before the node reads
	// it, and the node would never dial (the round would end in a protocol error).
	_ = s.CloseWrite()
	_ = r.ReadMsg(&msg)
	return theirs, rtt, nil
}

// punch is one DCUtR round as libp2p's initiator plays it (holepuncher.go): CONNECT with the member's own
// external address, SYNC, then, half an RTT later, a simultaneous-connect dial of the node's addresses as the
// server side. It reports whether a direct connection to the node exists afterwards.
func (m *rawMember) punch(ctx context.Context, to transport.PeerID) bool {
	own := []ma.Multiaddr{ma.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", m.end.external, simPort))}
	theirs, rtt, err := m.round(ctx, to, own, false)
	if err != nil || len(theirs) == 0 {
		return false
	}
	p, _ := p2p.LibP2PID(to)
	time.Sleep(rtt / 2)
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	dctx = network.WithForceDirectDial(network.WithSimultaneousConnect(dctx, false, "hole-punching"), "hole-punching")
	_ = m.h.Connect(dctx, peer.AddrInfo{ID: p, Addrs: theirs})
	for _, c := range m.h.Network().ConnsToPeer(p) {
		if !isCircuitAddr(c.RemoteMultiaddr()) {
			return true
		}
	}
	return false
}

func isCircuitAddr(a ma.Multiaddr) bool {
	_, err := a.ValueForProtocol(ma.P_CIRCUIT)
	return err == nil
}

// publicAddrs returns n distinct public QUIC addresses over IPv4 and IPv6 nobody listens on, from base. No TCP:
// the simnet carries only QUIC, and the nodes' TCP transport would dial a real host of the Internet (and block the
// bubble on real I/O).
func publicAddrs(base, n int) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, n)
	for i := range n {
		if i%2 == 0 {
			out = append(out, ma.StringCast(fmt.Sprintf("/ip4/5.%d.%d.1/udp/9000/quic-v1", base, i)))
		} else {
			out = append(out, ma.StringCast(fmt.Sprintf("/ip6/2001:db9:%x::%x/udp/9000/quic-v1", base, i)))
		}
	}
	return out
}
