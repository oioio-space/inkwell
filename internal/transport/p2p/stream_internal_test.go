package p2p

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// A reconnect can need the resource manager slot of its lost outbound stream.
// Force any premature opener to observe the slot before Reset releases it.
func TestOutLostReleasesStreamBeforeReopen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		s := &retiringStream{}
		opened := make(chan bool, 1)
		n := &Node{ctx: ctx, chg: make(chan struct{}), h: openRecordingHost{
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
