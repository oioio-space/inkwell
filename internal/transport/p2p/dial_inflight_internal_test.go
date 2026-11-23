package p2p

import (
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// TestLoopDialSparesTheAddressesOfAConnectInFlight: the peerstore is shared by every dialler of a node, so the
// dial loop's attempt, which clears a peer's addresses to dial its own set only, must not run while a call of
// Connect to that peer is in flight: the swarm would read the cleared set for that call and fail it with « no
// addresses » (a join into the running project failed so, 12 times in 400 runs of TestJoinSameProjectNoRestart).
func TestLoopDialSparesTheAddressesOfAConnectInFlight(t *testing.T) {
	h, err := libp2p.New(libp2p.NoListenAddrs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	other, err := libp2p.New(libp2p.NoListenAddrs)
	if err != nil {
		t.Fatal(err)
	}
	id := other.ID()
	_ = other.Close()
	given := ma.StringCast("/ip4/127.0.0.1/udp/9/quic-v1")
	h.Peerstore().AddAddr(id, given, peerstore.TempAddrTTL) // what the call of Connect in flight added
	n := &Node{h: h, connecting: map[peer.ID]int{id: 1}}

	err = n.dial(t.Context(), id, []ma.Multiaddr{ma.StringCast("/ip4/127.0.0.1/udp/10/quic-v1")}, 100*time.Millisecond)

	if !errors.Is(err, errConnectInFlight) {
		t.Errorf("the dial loop's attempt while a Connect is in flight = %v, want errConnectInFlight", err)
	}
	if got := h.Peerstore().Addrs(id); len(got) != 1 || !got[0].Equal(given) {
		t.Errorf("a Connect in flight lost its addresses to the dial loop: peerstore holds %v, want [%v]", got, given)
	}
}
