package p2p_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/x/simlibp2p"
	"github.com/marcopolo/simnet"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/transport/transporttest"
)

const (
	// blackhole is a latency so large that a packet is never delivered.
	blackhole = 1000 * time.Hour
	// simLatency is the one-way latency of the simulated network.
	simLatency = 5 * time.Millisecond
)

// key returns a deterministic identity for seed.
func key(seed byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
}

// idOf returns the PeerID of the identity key.
func idOf(k ed25519.PrivateKey) transport.PeerID {
	pub, _ := k.Public().(ed25519.PublicKey)
	return p2p.IDFromKey(pub)
}

// simNet is a simulated UDP network: real QUIC, TLS and yamux over virtual
// packets and a virtual clock, so it must be used inside a synctest bubble.
type simNet struct {
	net *simnet.Simnet
	cut atomic.Bool // when set, every packet is lost
}

// newSimNet starts a network that is closed when the test ends.
func newSimNet(t *testing.T) *simNet {
	t.Helper()
	s := new(simNet)
	s.net = &simnet.Simnet{LatencyFunc: func(*simnet.Packet) time.Duration {
		if s.cut.Load() {
			return blackhole
		}
		return simLatency
	}}
	s.net.Start()
	t.Cleanup(s.net.Close)
	return s
}

// node returns a node listening on QUIC at ip:8000 of the simulated network.
func (s *simNet) node(t *testing.T, seed byte, ip string, o p2p.Options) *p2p.Node {
	t.Helper()
	return s.nodeCtx(t.Context(), t, seed, ip, o)
}

// nodeCtx is node with an explicit context for New.
func (s *simNet) nodeCtx(ctx context.Context, t *testing.T, seed byte, ip string, o p2p.Options) *p2p.Node {
	t.Helper()
	link := simnet.NodeBiDiLinkSettings{
		Downlink: simnet.LinkSettings{BitsPerSecond: 100 * simlibp2p.OneMbps},
		Uplink:   simnet.LinkSettings{BitsPerSecond: 100 * simlibp2p.OneMbps},
	}
	p2p.SetExtra(
		&o,
		libp2p.NoListenAddrs,
		libp2p.ListenAddrStrings("/ip4/"+ip+"/udp/8000/quic-v1"),
		libp2p.DisableIdentifyAddressDiscovery(), // it stalls synctest
		simlibp2p.QUICSimnet(s.net, link),
	)
	n, err := p2p.New(ctx, key(seed), o)
	if err != nil {
		t.Fatalf("New(seed %d): %v", seed, err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}

// connect dials to from from; it reports failure with t.Error so that it can
// run in several goroutines.
func (s *simNet) connect(t testing.TB, from, to *p2p.Node) {
	t.Helper()
	s.cut.Store(false)
	if err := from.Connect(t.Context(), to.Local(), to.Addrs()); err != nil {
		t.Errorf("Connect(%s -> %s): %v", from.Local(), to.Local(), err)
	}
}

// sever loses every packet and closes both ends' connections locally, as the
// idle timeout would after 30 s; TestIdleTimeoutPeerDown covers the timeout.
func (s *simNet) sever(t testing.TB, a, b *p2p.Node) {
	t.Helper()
	s.cut.Store(true)
	closePeer(t, a, b)
	closePeer(t, b, a)
}

func closePeer(t testing.TB, n, other *p2p.Node) {
	t.Helper()
	id, ok := p2p.LibP2PID(other.Local())
	if !ok {
		t.Fatalf("LibP2PID(%s) failed", other.Local())
	}
	if err := n.Host().Network().ClosePeer(id); err != nil {
		t.Errorf("ClosePeer(%s): %v", other.Local(), err)
	}
}

func newSimPair(t *testing.T) transporttest.Pair {
	s := newSimNet(t)
	a := s.node(t, 1, "1.0.0.1", p2p.Options{})
	b := s.node(t, 2, "1.0.0.2", p2p.Options{})
	return transporttest.Pair{
		A: a, B: b,
		Connect: func(t testing.TB) { s.connect(t, a, b) },
		Cut:     func(t testing.TB) { s.sever(t, a, b) },
	}
}

func TestContract(t *testing.T) {
	transporttest.Run(t, newSimPair, transporttest.Options{Ordered: true, Synctest: true})
}
