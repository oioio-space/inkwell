package p2p

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"

	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

// TestLinkProbeRelayClientRole: the relay answers the peers that hold a reservation or have an active
// circuit (spec §8.5.1), as its own connection manager tags them, and the role ends with the tag. A
// peer whose request was refused was never tagged; and a peer minting identities costs the relay
// nothing: the roles keep no state.
func TestLinkProbeRelayClientRole(t *testing.T) {
	cm, err := connmgr.NewConnManager(10, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cm.Close() }()
	roles := func(p peer.ID) linkprobe.Roles { return relayRolesOf(cm, p) }
	refused, reserved, circuit := peer.ID("refused"), peer.ID("reserved"), peer.ID("circuit")

	cm.TagPeer(reserved, relayReservationTag, 1)
	cm.TagPeer(circuit, relayHopTag, 1)
	if got := roles(refused); got != 0 {
		t.Errorf("a refused requester has roles %v, want none", got)
	}
	if got := roles(reserved); got != linkprobe.RelayClient {
		t.Errorf("a peer with a reservation has roles %v, want RelayClient", got)
	}
	if got := roles(circuit); got != linkprobe.RelayClient {
		t.Errorf("a peer with an active circuit has roles %v, want RelayClient", got)
	}
	cm.UntagPeer(reserved, relayReservationTag) // the reservation expired
	if got := roles(reserved); got != 0 {
		t.Errorf("after the reservation expired the peer has roles %v, want none", got)
	}
	cm.UntagPeer(circuit, relayHopTag) // the circuit closed
	if got := roles(circuit); got != 0 {
		t.Errorf("after the circuit closed the peer has roles %v, want none", got)
	}
	// O(1) per request, no growth: the answer for an identity never seen creates nothing.
	if allocs := testing.AllocsPerRun(100, func() { roles(peer.ID("minted")) }); allocs > 4 {
		t.Errorf("a role lookup for an unknown identity allocates %.0f times, want a constant few", allocs)
	}
	for i := range 10000 {
		roles(peer.ID(string(rune('a'+i%26)) + string(rune(i))))
	}
	if ti := cm.GetTagInfo(peer.ID("minted")); ti != nil && len(ti.Tags) != 0 {
		t.Errorf("minted identities left %d tags", len(ti.Tags))
	}
}
