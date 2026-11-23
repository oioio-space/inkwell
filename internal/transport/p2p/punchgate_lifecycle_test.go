package p2p

import (
	"context"
	"errors"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch/pb"
	"github.com/libp2p/go-msgio/pbio"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multistream"
)

const punchTestPeer peer.ID = "member"

var punchTestAddr = ma.StringCast("/ip4/1.1.1.1/udp/1234/quic-v1")

func TestPunchCancelledRoundReturnsGrant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n, h := newPunchLifecycleFixture(t)
		s, remote := newPunchTestStream(t, network.DirInbound)
		h.stream = s
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		result := make(chan error, 1)
		// The round opens on the circuit it is given (H-20), and negotiates /libp2p/dcutr on it.
		via := punchTestConn{direction: network.DirInbound, stream: s}
		go func() { result <- n.punch(ctx, punchTestPeer, via) }()
		answerNegotiation(t, remote)
		readPunchTestMessage(t, remote, pb.HolePunch_CONNECT)
		time.Sleep(time.Second) // give the completed exchange a nonzero SYNC wait
		if err := pbio.NewDelimitedWriter(remote).WriteMsg(&pb.HolePunch{
			Type: pb.HolePunch_CONNECT.Enum(), ObsAddrs: [][]byte{punchTestAddr.Bytes()},
		}); err != nil {
			t.Fatal(err)
		}
		readPunchTestMessage(t, remote, pb.HolePunch_SYNC)
		synctest.Wait()
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("punch result = %v, want cancellation during SYNC wait", err)
		}
		if got := heldPunchSlots(n.pgate); got != 0 {
			t.Fatalf("cancelled round still holds %d slot(s)", got)
		}
	})
}

func TestPunchRefusedAnswerDoesNotReleaseAnotherRound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n, h := newPunchLifecycleFixture(t)
		g := n.pgate
		g.mu.Lock()
		now := time.Now()
		bucket := &g.peerLocked(punchTestPeer, now).budgets.Rounds
		for bucket.Tokens(now) > 1 {
			bucket.Take(now)
		}
		g.mu.Unlock()
		admitted, remote := newPunchTestStream(t, network.DirOutbound)
		answered := make(chan struct{})
		go func() {
			h.handler(admitted)
			close(answered)
		}()
		synctest.Wait() // first handler is waiting for its CONNECT
		if got := heldPunchSlots(g); got != 1 {
			t.Fatalf("first handler holds %d slots, want one", got)
		}
		refused, _ := newPunchTestStream(t, network.DirOutbound)
		h.handler(refused)
		if got := heldPunchSlots(g); got != 1 {
			t.Errorf("refused handler changed the admitted round's slot count to %d", got)
		}
		_ = remote.Close()
		<-answered
		if got := heldPunchSlots(g); got != 0 {
			t.Errorf("admitted handler did not return its slot: %d held", got)
		}
	})
}

func heldPunchSlots(g *punchGate) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.slots.Held(time.Now())
}

func newPunchLifecycleFixture(t *testing.T) (*Node, *punchTestHost) {
	t.Helper()
	h := &punchTestHost{}
	g := newPunchGate(func(peer.ID) bool { return true }, time.Now())
	s := newPunchService(t.Context(), h, nil, g, func() []ma.Multiaddr {
		return []ma.Multiaddr{punchTestAddr}
	})
	n := &Node{ctx: t.Context(), proofs: newProofSet(), h: h, pgate: g, connWaits: make(map[network.Conn]map[*context.CancelFunc]struct{})}
	n.hps.Store(s)
	s.serve()
	return n, h
}

func newPunchTestStream(t *testing.T, direction network.Direction) (*punchTestStream, net.Conn) {
	t.Helper()
	local, remote := net.Pipe()
	t.Cleanup(func() {
		_ = local.Close()
		_ = remote.Close()
	})
	return &punchTestStream{ConnIO: local, conn: punchTestConn{direction: direction}}, remote
}

// answerNegotiation plays the peer's side of the multistream negotiation of /libp2p/dcutr on remote.
func answerNegotiation(t *testing.T, remote net.Conn) {
	t.Helper()
	mux := multistream.NewMultistreamMuxer[protocol.ID]()
	mux.AddHandler(holepunch.Protocol, nil)
	if _, _, err := mux.Negotiate(remote); err != nil {
		t.Fatal(err)
	}
}

func readPunchTestMessage(t *testing.T, remote net.Conn, want pb.HolePunch_Type) {
	t.Helper()
	var msg pb.HolePunch
	if err := pbio.NewDelimitedReader(remote, 4096).ReadMsg(&msg); err != nil {
		t.Fatal(err)
	}
	if msg.GetType() != want {
		t.Fatalf("received %s, want %s", msg.GetType(), want)
	}
}

type punchTestHost struct {
	host.Host
	handler network.StreamHandler
	stream  network.Stream
}

func (*punchTestHost) Network() network.Network       { return punchTestNet{} }
func (*punchTestHost) Peerstore() peerstore.Peerstore { return punchTestStore{} }
func (h *punchTestHost) SetStreamHandler(_ protocol.ID, handler network.StreamHandler) {
	h.handler = handler
}

func (h *punchTestHost) NewStream(context.Context, peer.ID, ...protocol.ID) (network.Stream, error) {
	return h.stream, nil
}

type punchTestNet struct{ network.Network }

func (punchTestNet) ConnsToPeer(peer.ID) []network.Conn {
	return []network.Conn{punchTestConn{direction: network.DirInbound}}
}

type punchTestStore struct{ peerstore.Peerstore }

func (punchTestStore) Addrs(peer.ID) []ma.Multiaddr { return nil }

type punchTestConn struct {
	network.Conn
	direction network.Direction
	stream    network.Stream // what NewStream opens
}

func (c punchTestConn) NewStream(context.Context) (network.Stream, error) { return c.stream, nil }

func (c punchTestConn) Stat() network.ConnStats {
	var stats network.ConnStats
	stats.Direction = c.direction
	return stats
}
func (punchTestConn) IsClosed() bool                { return false }
func (punchTestConn) RemotePeer() peer.ID           { return punchTestPeer }
func (punchTestConn) RemoteMultiaddr() ma.Multiaddr { return ma.StringCast("/p2p-circuit") }

type punchTestStream struct {
	network.Stream
	ConnIO net.Conn
	conn   punchTestConn
}

func (s *punchTestStream) Read(b []byte) (int, error)    { return s.ConnIO.Read(b) }
func (s *punchTestStream) Write(b []byte) (int, error)   { return s.ConnIO.Write(b) }
func (s *punchTestStream) SetDeadline(d time.Time) error { return s.ConnIO.SetDeadline(d) }
func (s *punchTestStream) Reset() error                  { return s.ConnIO.Close() }
func (s *punchTestStream) Close() error                  { return s.ConnIO.Close() }
func (s *punchTestStream) Conn() network.Conn            { return s.conn }
func (*punchTestStream) SetProtocol(protocol.ID) error   { return nil }
func (*punchTestStream) Scope() network.StreamScope      { return &network.NullScope{} }
