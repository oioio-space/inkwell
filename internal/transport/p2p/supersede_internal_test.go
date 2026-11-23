package p2p

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/multiformats/go-multistream"
)

// P-15: any byte of the peer is an answer — its refusal of /ipfs/ping/1.0.0, or its reset of the stream; a local
// failure, the end of the window or the close of the connection is none.
func TestPeerAnswered(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want bool
	}{
		{"refused", fmt.Errorf("negotiate: %w", multistream.ErrNotSupported[protocol.ID]{Protos: []protocol.ID{"/ipfs/ping/1.0.0"}}), true},
		{"reset by the peer", &network.StreamError{ErrorCode: 0x1002, Remote: true}, true},
		{"reset here", &network.StreamError{ErrorCode: 0, Remote: false}, false},
		{"window over", context.DeadlineExceeded, false},
		{"connection closed", &network.ConnError{Remote: true}, false},
		{"refused by the resource manager", errors.New("cannot reserve stream: resource limit exceeded"), false},
	} {
		if got := peerAnswered(c.err); got != c.want {
			t.Errorf("P-15 peerAnswered(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

// P-15: the node may hold up to maxPeerConns connections to a peer (libp2p's per-peer limit), so a proof on each
// older one and the RTT ping of a relayed link always fit the ping streams the resource manager allows per peer.
func TestMaxPeerConnsIsLibp2pLimit(t *testing.T) {
	scaling := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&scaling)
	if got := scaling.Scale(memoryLimit, fdLimit).ToPartialLimitConfig().PeerDefault.Conns; got != maxPeerConns {
		t.Fatalf("P-15 libp2p allows %v connections per peer, maxPeerConns is %d: the ping streams per peer no longer cover the proofs", got, maxPeerConns)
	}
}
