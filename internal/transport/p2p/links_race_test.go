package p2p

import (
	"context"
	"crypto/ed25519"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

// quicHost returns a bare libp2p host on the loopback QUIC transport whose identity is the one
// loopbackNode derives from seed: it runs none of the node's protocols.
func quicHost(t *testing.T, seed byte) host.Host {
	t.Helper()
	priv, err := crypto.UnmarshalEd25519PrivateKey(ed25519.NewKeyFromSeed(append(make([]byte, 31), seed)))
	if err != nil {
		t.Fatalf("UnmarshalEd25519PrivateKey: %v", err)
	}
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"),
		libp2p.DisableMetrics(),
	)
	if err != nil {
		t.Fatalf("libp2p.New: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// waitEvent receives from n until an event of kind, or fails the test.
func waitEvent(t *testing.T, n *Node, kind transport.Kind) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		ev, err := n.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv, waiting for event kind %v: %v", kind, err)
		}
		if ev.Kind == kind {
			return
		}
	}
}

// waitFor polls cond until it holds, or fails the test with what.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !cond(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// syncOut returns the stream n's session with p runs on, or nil.
func syncOut(n *Node, p peer.ID) network.Stream {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ps := n.peers[p]; ps != nil {
		return ps.out
	}
	return nil
}

// sampleOf returns the sample of p among samples.
func sampleOf(t *testing.T, samples []linkSample, p peer.ID) linkSample {
	t.Helper()
	for _, s := range samples {
		if s.id == p {
			return s
		}
	}
	t.Fatalf("no sample of %s among %d", p, len(samples))
	return linkSample{}
}

// TestLinkProbeEndsWithItsConnection: a connection that ends, and whose detach runs, while attach
// is between its look at the connection and the insertion of the probe, must not keep a probe
// (and its timer) until shutdown: the probe ends with its connection and unregisters (spec §8.5.1,
// one probe per live QUIC connection).
func TestLinkProbeEndsWithItsConnection(t *testing.T) {
	ha, hb := quicHost(t, 11), quicHost(t, 12)
	if err := ha.Connect(t.Context(), peer.AddrInfo{ID: hb.ID(), Addrs: hb.Addrs()}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	c := ha.Network().ConnsToPeer(hb.ID())[0]
	var ps *probeSet
	var looked atomic.Bool
	ps = newProbeSet(t.Context(), ha, func(c network.Conn) linkprobe.Roles {
		if looked.CompareAndSwap(false, true) { // the connection ends and its notification is handled before the insertion
			_ = c.Close()
			ps.detach(c)
		}
		return 0
	})
	t.Cleanup(ps.close)
	ps.attach(c)
	if pc := ps.of(c.ID()); pc != nil {
		select {
		case <-pc.ctx.Done():
		case <-time.After(10 * time.Second):
			t.Fatal("the probe of a closed connection is still running")
		}
	}
	if pc := ps.of(c.ID()); pc != nil {
		t.Fatal("the probe of a closed connection is still registered")
	}
}

// TestLinkLevelIsTheSyncConnections: with two connections to a peer, the level shown is the one
// of the connection that carries the session (Annex A.1, « Sens de quality »), not that of the
// first QUIC connection to the peer. Here the session runs on TCP, which the probe never runs on,
// so the level is unknown whatever the QUIC connection's probe shows.
func TestLinkLevelIsTheSyncConnections(t *testing.T) {
	a, b := loopbackNode(t, 13, Options{}), loopbackNode(t, 14, Options{})
	var tcp []string
	for _, addr := range b.Addrs() {
		if strings.Contains(addr, "/tcp/") {
			tcp = append(tcp, addr)
		}
	}
	if err := a.Connect(t.Context(), b.Local(), tcp); err != nil {
		t.Fatalf("Connect over TCP: %v", err)
	}
	waitEvent(t, b, transport.PeerUp) // b's session with a runs on the TCP connection
	a2 := quicHost(t, 13)             // a second connection from a's identity, over QUIC (its only transport)
	if a2.ID() != a.h.ID() {
		t.Fatalf("a2 is %s, want a's identity %s", a2.ID(), a.h.ID())
	}
	if err := a2.Connect(t.Context(), peer.AddrInfo{ID: b.h.ID(), Addrs: b.h.Addrs()}); err != nil {
		t.Fatalf("Connect over QUIC: %v", err)
	}
	var qc network.Conn
	waitFor(t, "b's probe on the QUIC connection", func() bool {
		for _, c := range b.h.Network().ConnsToPeer(a.h.ID()) {
			if _, ok := quicStats(c); ok && b.probes.of(c.ID()) != nil {
				qc = c
				return true
			}
		}
		return false
	})
	out := syncOut(b, a.h.ID())
	if out == nil || out.Conn().ID() == qc.ID() {
		t.Fatalf("b's session is on %v, want the TCP connection", out)
	}
	views := stepViews{qc.ID(): {level: transport.QualityPoor, loss: 400, hasLoss: true}}
	s := sampleOf(t, b.readLinks(views), a.h.ID())
	if s.probe.level != transport.QualityUnknown || s.probe.hasLoss {
		t.Errorf("level %v, loss %d (known %t), want unknown: the session's connection has no probe", s.probe.level, s.probe.loss, s.probe.hasLoss)
	}

	// The level of the connection that carries the session is shown.
	c, d := loopbackNode(t, 15, Options{}), loopbackNode(t, 16, Options{})
	if err := c.Connect(t.Context(), d.Local(), quicAddrs(d)); err != nil {
		t.Fatalf("Connect over QUIC: %v", err)
	}
	waitEvent(t, c, transport.PeerUp)
	views = stepViews{syncOut(c, d.h.ID()).Conn().ID(): {level: transport.QualityFair, loss: 30, hasLoss: true}}
	if s := sampleOf(t, c.readLinks(views), d.h.ID()); s.probe.level != transport.QualityFair {
		t.Errorf("level %v, want the session connection's, fair", s.probe.level)
	}
}

// quicAddrs returns n's QUIC addresses.
func quicAddrs(n *Node) []string {
	var out []string
	for _, a := range n.Addrs() {
		if strings.Contains(a, "/quic-v1") {
			out = append(out, a)
		}
	}
	return out
}

// TestLinkLevelDropsSampleOfEndedSession: a sample read before the peer went down and came back
// up describes the connection of the ended session; applied after the PeerUp of the new one, it
// would show the old connection's level as the new link's (Annex A.1: after a change of
// connection the figure and the level start again from the new one, unknown first).
func TestLinkLevelDropsSampleOfEndedSession(t *testing.T) {
	a, b := loopbackNode(t, 17, Options{}), loopbackNode(t, 18, Options{})
	if err := a.Connect(t.Context(), b.Local(), quicAddrs(b)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitEvent(t, a, transport.PeerUp)
	stale := sampleOf(t, a.readLinks(make(stepViews)), b.h.ID())
	stale.probe = probeView{level: transport.QualityPoor, loss: 999, hasLoss: true} // recognisable

	// Between the read and the apply, the peer goes down and comes back.
	_ = a.h.Network().ClosePeer(b.h.ID())
	waitEvent(t, a, transport.PeerDown)
	if err := a.Connect(t.Context(), b.Local(), quicAddrs(b)); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	waitEvent(t, a, transport.PeerUp)

	for _, st := range a.applyLinks([]linkSample{stale}) {
		if st.Peer == b.Local() {
			t.Errorf("a sample of the ended session was applied to the new one: level %v, loss %d", st.Level, st.LossPermille)
		}
	}
	steps := a.applyLinks(a.readLinks(make(stepViews)))
	if len(steps) != 1 || steps[0].Peer != b.Local() {
		t.Errorf("a fresh sample gave the steps %v, want one of %s", steps, b.Local())
	}
}
