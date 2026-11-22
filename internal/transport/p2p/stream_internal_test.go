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

type openRecordingHost struct {
	host.Host
	open func(context.Context) (network.Stream, error)
}

func (h openRecordingHost) NewStream(ctx context.Context, _ peer.ID, _ ...protocol.ID) (network.Stream, error) {
	return h.open(ctx)
}
