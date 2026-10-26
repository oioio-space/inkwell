package p2p_test

import (
	"crypto/rand"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// testService returns an mDNS service name no other run shares, so that tests
// never see (or answer) the nodes of the real network.
func testService() string { return "_ink" + rand.Text()[:12] + "._udp" }

// TestLANServiceIsolates: nodes that announce under another service name do
// not see each other, those under the same one do.
func TestLANServiceIsolates(t *testing.T) {
	svc := testService()
	a := loopbackNode(t, 4, p2p.Options{LAN: true, LANService: svc})
	b := loopbackNode(t, 5, p2p.Options{LAN: true, LANService: svc})
	other := loopbackNode(t, 6, p2p.Options{LAN: true, LANService: testService()})
	if err := errors.Join(a.LANError(), b.LANError(), other.LANError()); err != nil {
		t.Skipf("mDNS is not available here: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !slices.ContainsFunc(a.Nearby(), func(n p2p.Nearby) bool { return n.ID == b.Local() }) {
		if time.Now().After(deadline) {
			t.Skip("mDNS multicast does not work on this machine")
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, n := range a.Nearby() {
		if n.ID == other.Local() {
			t.Errorf("a.Nearby() lists %v, announced under another service", n.ID)
		}
	}
}
