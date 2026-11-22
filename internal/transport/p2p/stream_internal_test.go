package p2p

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

// A reconnect can need the resource manager slot of its lost outbound stream.
// Force any premature opener to observe the slot before Reset releases it.
func TestOutLostReleasesStreamBeforeReopen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		s := &retiringStream{}
		opened := make(chan bool, 1)
		n := &Node{ctx: ctx, chg: make(chan struct{}), h: openRecordingHost{
			net: &recordingNetwork{},
			open: func(ctx context.Context) (network.Stream, error) {
				opened <- s.released.Load()
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}}
		defer func() {
			n.mu.Lock()
			n.closed = true
			n.mu.Unlock()
			cancel()
			n.wg.Wait()
		}()
		ps := &peerState{connected: true, admitted: true, out: s}
		n.outLost(ps, s, true, false)
		synctest.Wait()
		select {
		case released := <-opened:
			if !released {
				t.Error("replacement opened before the lost stream released its resource manager slot")
			}
		default:
			t.Error("lost stream was not reopened")
		}
	})
}

type retiringStream struct {
	network.Stream
	released atomic.Bool
}

func (s *retiringStream) Reset() error {
	synctest.Wait()
	s.released.Store(true)
	return nil
}

func TestOpenOutConnectionLossPreservesReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		firstOpen, loseFirst := make(chan struct{}), make(chan struct{})
		replacementOpen := make(chan struct{})
		net := &recordingNetwork{}
		n := &Node{ctx: ctx, chg: make(chan struct{}), h: openRecordingHost{
			net: net,
			open: func(ctx context.Context) (network.Stream, error) {
				select {
				case <-firstOpen:
					close(replacementOpen)
					<-ctx.Done()
					return nil, ctx.Err()
				default:
					close(firstOpen)
					<-loseFirst
					return nil, &network.ConnError{Remote: true, ErrorCode: network.ConnGated}
				}
			},
		}}
		defer func() {
			n.mu.Lock()
			n.closed = true
			n.mu.Unlock()
			cancel()
			n.wg.Wait()
		}()

		ps := &peerState{connected: true, admitted: true}
		n.startOpenLocked(ps)
		<-firstOpen
		// The original connection is gone. The replacement has already
		// arrived while openOut still owns the opening.
		net.conns = []network.Conn{openRecordingConn{}}
		close(loseFirst)
		synctest.Wait()
		if net.closed {
			t.Fatal("the failed opening closed the replacement connection")
		}
		select {
		case <-replacementOpen:
		default:
			t.Fatal("the replacement connection did not get its own opening")
		}
	})
}

// TestOpenOutLostWithoutStreamSparesReplacement: an opening whose connection is
// lost before NewStream returns a stream fails with no connection error at all:
// BasicHost.NewStream turns the swarm's network.ErrNoConn (every connection it
// tried was closed) into errors.New("connection failed"). That is a lost
// connection, not a refusal (spec §8.7.4, PeerUpAck.tla StreamLost): no
// connection is closed, and a replacement that arrived meanwhile gets its own
// opening. Seen 123 times in 125 failed openings of TestLanAskAndAnswer.
func TestOpenOutLostWithoutStreamSparesReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		firstOpen, loseFirst := make(chan struct{}), make(chan struct{})
		replacementOpen := make(chan struct{})
		orig, repl := &closableConn{}, &closableConn{}
		net := &recordingNetwork{conns: []network.Conn{orig}}
		n := &Node{ctx: ctx, chg: make(chan struct{}), h: openRecordingHost{
			net: net,
			open: func(ctx context.Context) (network.Stream, error) {
				select {
				case <-firstOpen:
					close(replacementOpen)
					<-ctx.Done()
					return nil, ctx.Err()
				default:
					close(firstOpen)
					<-loseFirst
					return nil, errors.New("connection failed")
				}
			},
		}}
		defer stopNode(n, cancel)

		ps := &peerState{connected: true, admitted: true}
		n.startOpenLocked(ps)
		<-firstOpen
		orig.closed.Store(true)
		net.conns = []network.Conn{repl}
		close(loseFirst)
		synctest.Wait()
		if net.closed || repl.closed.Load() {
			t.Fatal("an opening lost with its connection closed the replacement connection")
		}
		select {
		case <-replacementOpen:
		default:
			t.Fatal("the replacement connection did not get its own opening")
		}
	})
}

// TestNewConnectionOpensWithoutConnectednessEvent: libp2p reports a peer's
// connectedness, not its connections, and reads it when it emits: the loss of a
// connection followed by a replacement emits no event at all (go-libp2p
// p2p/net/swarm/connection_events_emitter.go, notifyPeer). An opening lost with
// the old connection before the replacement exists cannot reopen on it, so the
// replacement itself must start the opening (PeerUpAck.tla StartOpen: any
// usable connection). Otherwise the peer stays connected with no stream and no
// opening, and every Send to it waits for ever: the host's wiring did, and the
// joiner's lobby answer never came (TestLanAskAndAnswer, GitHub run
// 45076934841, closed after 120 s by the joiner's own window).
func TestNewConnectionOpensWithoutConnectednessEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		firstOpen, loseFirst := make(chan struct{}), make(chan struct{})
		replacementOpen := make(chan struct{})
		orig, repl := &closableConn{}, &closableConn{}
		net := &recordingNetwork{conns: []network.Conn{orig}}
		n := &Node{ctx: ctx, chg: make(chan struct{}), peers: map[peer.ID]*peerState{}, h: openRecordingHost{
			net: net,
			open: func(ctx context.Context) (network.Stream, error) {
				select {
				case <-firstOpen:
					close(replacementOpen)
					<-ctx.Done()
					return nil, ctx.Err()
				default:
					close(firstOpen)
					<-loseFirst
					return nil, errors.New("connection failed")
				}
			},
		}}
		defer stopNode(n, cancel)

		ps := &peerState{connected: true, admitted: true}
		n.peers[ps.id] = ps
		n.startOpenLocked(ps)
		<-firstOpen
		orig.closed.Store(true)
		net.conns = nil // lost, and no replacement yet
		close(loseFirst)
		synctest.Wait()
		net.conns = []network.Conn{repl} // the replacement arrives, with no connectedness event
		n.onConn(repl)
		synctest.Wait()
		select {
		case <-replacementOpen:
		default:
			t.Fatal("a new connection to a connected peer with no stream did not start an opening")
		}
	})
}

func stopNode(n *Node, cancel context.CancelFunc) {
	n.mu.Lock()
	n.closed = true
	n.mu.Unlock()
	cancel()
	n.wg.Wait()
}

// closableConn is a connection that records its close.
type closableConn struct {
	network.Conn
	closed atomic.Bool
}

func (c *closableConn) IsClosed() bool { return c.closed.Load() }

func (c *closableConn) RemotePeer() peer.ID { return "" }

func (c *closableConn) RemoteMultiaddr() ma.Multiaddr {
	return ma.StringCast("/ip4/127.0.0.1/udp/1/quic-v1")
}

func (c *closableConn) Close() error {
	c.closed.Store(true)
	return nil
}

type recordingNetwork struct {
	network.Network
	conns  []network.Conn
	closed bool
}

func (n *recordingNetwork) ConnsToPeer(peer.ID) []network.Conn { return n.conns }

func (n *recordingNetwork) ClosePeer(peer.ID) error {
	n.closed = true
	return nil
}

type openRecordingConn struct{ network.Conn }

func (openRecordingConn) IsClosed() bool { return false }

type openRecordingHost struct {
	host.Host
	net  network.Network
	open func(context.Context) (network.Stream, error)
}

func (h openRecordingHost) Network() network.Network { return h.net }

func (h openRecordingHost) NewStream(ctx context.Context, _ peer.ID, _ ...protocol.ID) (network.Stream, error) {
	return h.open(ctx)
}
