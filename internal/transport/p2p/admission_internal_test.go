package p2p

import (
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	wire "github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/admit"
)

// recordingRM is a resource manager that records the methods called on it.
type recordingRM struct {
	network.NullResourceManager
	calls []string
}

func (r *recordingRM) call(m string) { r.calls = append(r.calls, m) }

func (r *recordingRM) ViewSystem(f func(network.ResourceScope) error) error {
	r.call("ViewSystem")
	return r.NullResourceManager.ViewSystem(f)
}

func (r *recordingRM) ViewTransient(f func(network.ResourceScope) error) error {
	r.call("ViewTransient")
	return r.NullResourceManager.ViewTransient(f)
}

func (r *recordingRM) ViewService(s string, f func(network.ServiceScope) error) error {
	r.call("ViewService")
	return r.NullResourceManager.ViewService(s, f)
}

func (r *recordingRM) ViewProtocol(p protocol.ID, f func(network.ProtocolScope) error) error {
	r.call("ViewProtocol")
	return r.NullResourceManager.ViewProtocol(p, f)
}

func (r *recordingRM) ViewPeer(p peer.ID, f func(network.PeerScope) error) error {
	r.call("ViewPeer")
	return r.NullResourceManager.ViewPeer(p, f)
}

func (r *recordingRM) OpenConnection(d network.Direction, fd bool, a ma.Multiaddr) (network.ConnManagementScope, error) {
	r.call("OpenConnection")
	return r.NullResourceManager.OpenConnection(d, fd, a)
}

func (r *recordingRM) VerifySourceAddress(a net.Addr) bool {
	r.call("VerifySourceAddress")
	return false
}

func (r *recordingRM) OpenStream(p peer.ID, d network.Direction) (network.StreamManagementScope, error) {
	r.call("OpenStream")
	return r.NullResourceManager.OpenStream(p, d)
}

func (r *recordingRM) Close() error { r.call("Close"); return nil }

// TestAdmissionForwardsResourceManager: the admission layer forwards every method of network.ResourceManager
// to libp2p's (the footgun of go-libp2p #2805, where a wrapper silently dropped one), and answers
// VerifySourceAddress with libp2p's own rule on its own limiter (RF-6).
func TestAdmissionForwardsResourceManager(t *testing.T) {
	const rule = "RF-6 wrapper drops a method"
	inner := &recordingRM{}
	a := newAdmission(inner, nodeAdmitConfig())
	var rm network.ResourceManager = a
	nop := func(network.ResourceScope) error { return nil }
	_ = rm.ViewSystem(nop)
	_ = rm.ViewTransient(nop)
	_ = rm.ViewService("s", func(network.ServiceScope) error { return nil })
	_ = rm.ViewProtocol("/p", func(network.ProtocolScope) error { return nil })
	_ = rm.ViewPeer("p", func(network.PeerScope) error { return nil })
	if s, err := rm.OpenConnection(network.DirInbound, false, ma.StringCast("/ip4/2.2.9.1/udp/1/quic-v1")); err != nil {
		t.Fatalf("%s: OpenConnection: %v", rule, err)
	} else {
		s.Done()
	}
	_, _ = rm.OpenStream("p", network.DirOutbound)
	_ = rm.Close()
	want := []string{}
	ty := reflect.TypeFor[network.ResourceManager]()
	for method := range ty.Methods() {
		if m := method.Name; m != "VerifySourceAddress" {
			want = append(want, m)
		}
	}
	slices.Sort(want)
	got := slices.Sorted(slices.Values(inner.calls))
	if !slices.Equal(got, want) {
		t.Fatalf("%s: libp2p's resource manager saw %v, want every method but VerifySourceAddress: %v", rule, got, want)
	}
	// VerifySourceAddress: libp2p's rule, a burst of 4 per IPv4 address then verification, never on loopback.
	addr := &net.UDPAddr{IP: net.ParseIP("2.2.9.1"), Port: 4000}
	for i := range wire.MaxPendingPerIP / 2 {
		if rm.VerifySourceAddress(addr) {
			t.Fatalf("%s: address verified at handshake %d, inside the burst", rule, i)
		}
	}
	if !rm.VerifySourceAddress(addr) {
		t.Fatalf("%s: VerifySourceAddress never verifies beyond the burst", rule)
	}
	if rm.VerifySourceAddress(&net.UDPAddr{IP: net.ParseIP("2.2.9.2"), Port: 4000}) {
		t.Fatalf("%s: another address shares the burst", rule)
	}
	for range 10 {
		if rm.VerifySourceAddress(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4000}) {
			t.Fatalf("%s: a loopback address verified", rule)
		}
	}
	if slices.Contains(inner.calls, "VerifySourceAddress") {
		t.Fatalf("%s: VerifySourceAddress reached libp2p's limiter, which admission switches off", rule)
	}
}

const testRelay = "12D3KooWLBnqnHG3pqmuGD6GPGEkPAosAuBA4PCfgxFtqJ2hTYA6"

// TestCircuitOriginIsRelayPeer: an inbound circuit is charged to its relay's authenticated PeerID, never to
// the relay's IP address (RF-1); a direct connection to its /32 or /56; an outbound one to no origin.
func TestCircuitOriginIsRelayPeer(t *testing.T) {
	const rule = "RF-1 origin"
	dest := "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN"
	for _, tc := range []struct{ endpoint, want string }{
		// Inbound: the relay connection's address, then /p2p/<relay>/p2p-circuit (circuitv2/client/conn.go).
		{"/ip4/1.2.0.1/udp/8000/quic-v1/p2p/" + testRelay + "/p2p-circuit", "relay:" + testRelay},
		// Outbound: the dialled circuit address, ending with the destination.
		{"/ip4/1.2.0.1/udp/8000/quic-v1/p2p/" + testRelay + "/p2p-circuit/p2p/" + dest, "relay:" + testRelay},
		{"/ip4/2.2.9.1/udp/8000/quic-v1", "ip:2.2.9.1/32"},
		{"/ip6/2001:db8:1:2345::1/udp/8000/quic-v1", "ip:2001:db8:1:2300::/56"},
		{"/ip4/2.2.9.1/tcp/8000", "ip:2.2.9.1/32"},
		{"/p2p-circuit", "none"}, // no relay named: one shared origin, never an exemption
	} {
		if got := originOf(ma.StringCast(tc.endpoint)).String(); got != tc.want {
			t.Errorf("%s: origin of %s = %s, want %s", rule, tc.endpoint, got, tc.want)
		}
	}
	// Through the layer, with libp2p's real limits: two inbound circuits through one relay and an outbound one.
	a, err := newNodeAdmission()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	circuit := ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1/p2p/" + testRelay + "/p2p-circuit")
	for range 2 {
		if _, err := a.OpenConnection(network.DirInbound, false, circuit); err != nil {
			t.Fatalf("%s: inbound circuit refused: %v", rule, err)
		}
	}
	if _, err := a.OpenConnection(network.DirOutbound, false, ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1/p2p/"+testRelay+"/p2p-circuit/p2p/"+dest)); err != nil {
		t.Fatalf("%s: outbound circuit refused: %v", rule, err)
	}
	s := a.ledger.Stats(a.now())
	if got := s.PerOrigin[admit.RelayOrigin(testRelay)]; got != 2 || s.Pending != 2 {
		t.Fatalf("%s: %d pending at the relay origin, %d in all (%v), want 2 and 2", rule, got, s.Pending, s.PerOrigin)
	}
}

// connsAt opens n connections in direction dir from peer p (inbound from ip), each reserving the 1 MiB initial
// QUIC window as a QUIC connection does, and returns their scopes; it fails with rule at the first refusal.
func connsAt(t *testing.T, a *admission, dir network.Direction, ip string, p peer.ID, n int, rule string) []network.ConnManagementScope {
	t.Helper()
	var out []network.ConnManagementScope
	for i := range n {
		s, err := openAs(a, dir, ip, p)
		if err != nil {
			t.Fatalf("%s: connection %d of %s (%v) refused: %v", rule, i, p, dir, err)
		}
		out = append(out, s)
	}
	return out
}

// openAs opens one connection, reserves its initial window and sets its peer (none if p is empty).
func openAs(a *admission, dir network.Direction, ip string, p peer.ID) (network.ConnManagementScope, error) {
	s, err := a.OpenConnection(dir, false, ma.StringCast("/ip4/"+ip+"/udp/8000/quic-v1"))
	if err != nil {
		return nil, err
	}
	if err := s.ReserveMemory(int(wire.QUICInitialConnectionWindow), network.ReservationPriorityAlways); err != nil {
		s.Done()
		return nil, err
	}
	if p != "" {
		if err := s.SetPeer(p); err != nil {
			s.Done()
			return nil, err
		}
	}
	return s, nil
}

func memberIDs(n int) []peer.ID {
	ids := make([]peer.ID, n)
	for i := range ids {
		ids[i] = peer.ID(fmt.Sprintf("member-%02d", i))
	}
	return ids
}

func classByName(p peer.ID) admit.Class {
	switch {
	case strings.HasPrefix(string(p), "member-"):
		return admit.Member
	case strings.HasPrefix(string(p), "relay-"):
		return admit.Relay
	}
	return admit.Stranger
}

// TestNodeConnsBound: 128 member connections, all inbound, then all outbound (§16.1 « connexions », RF-6),
// alongside the 48 pending and the relays' 8, each reserving its 1 MiB QUIC window: 184 connections and
// 184 MiB within libp2p's limits and the node's 256 MiB. The 129th is refused, one closed lets the next in.
func TestNodeConnsBound(t *testing.T) {
	const rule = "RF-6 node connections"
	for _, dir := range []network.Direction{network.DirInbound, network.DirOutbound} {
		t.Run(dir.String(), func(t *testing.T) {
			a, err := newNodeAdmission()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = a.Close() }()
			a.classify = classByName
			ids := memberIDs(wire.MaxConnectedMembers)
			a.setRoster(ids)
			for r := range wire.MaxRelays {
				connsAt(t, a, network.DirOutbound, "1.2.0.1", peer.ID(fmt.Sprintf("relay-%d", r)), wire.ConnectionsPerRelay, rule)
			}
			var held []network.ConnManagementScope
			for i, id := range ids {
				held = append(held, connsAt(t, a, dir, fmt.Sprintf("4.4.%d.1", i), id, wire.MemberShareConnections, rule)...)
			}
			for _, p := range []peer.ID{ids[0], "member-outsider"} {
				if s, err := openAs(a, dir, "4.4.200.1", p); err == nil {
					s.Done()
					t.Fatalf("%s: the 129th member connection (%s) admitted", rule, p)
				} else if !strings.Contains(err.Error(), "members") {
					t.Fatalf("%s: the 129th refused for another reason than the member bound: %v", rule, err)
				}
			}
			held[len(held)-1].Done() // the last member is one short of its share: its next is admitted
			s, err := openAs(a, dir, "4.4.201.1", ids[len(ids)-1])
			if err != nil {
				t.Fatalf("%s: after a close, the member's next connection refused: %v", rule, err)
			}
			held[len(held)-1] = s
			// The 48 pending fit beside them: 184 connections, 184 MiB of windows.
			for i := range wire.MaxPending {
				connsAt(t, a, network.DirInbound, fmt.Sprintf("3.3.%d.1", i), "", 1, rule)
			}
			if got := a.ledger.Stats(a.now()); got.Members != wire.MaxMemberConnections || got.Pending != wire.MaxPending || got.Relays != wire.MaxRelays*wire.ConnectionsPerRelay {
				t.Fatalf("%s: %d members, %d pending, %d relays, want %d, %d, %d", rule, got.Members, got.Pending, got.Relays, wire.MaxMemberConnections, wire.MaxPending, wire.MaxRelays*wire.ConnectionsPerRelay)
			}
			_ = a.ViewSystem(func(s network.ResourceScope) error {
				if st := s.Stat(); st.NumConnsInbound+st.NumConnsOutbound != wire.MaxNodeConnections || st.Memory != wire.MaxNodeConnections*wire.QUICInitialConnectionWindow {
					t.Errorf("%s: libp2p counts %d connections and %d bytes, want %d and %d MiB", rule, st.NumConnsInbound+st.NumConnsOutbound, st.Memory, wire.MaxNodeConnections, wire.MaxNodeConnections)
				}
				return nil
			})
			if s, err := openAs(a, network.DirInbound, "3.3.200.1", ""); err == nil {
				s.Done()
				t.Fatalf("RF-3: a 49th pending connection admitted")
			}
		})
	}
}

// TestMemberShareKept: a Byzantine member opening connections as fast as the layer lets it holds 4 at most and
// never one of another member's 2 reserved; every honest member then gets its two (RF-6, model SharesKept).
func TestMemberShareKept(t *testing.T) {
	const rule = "RF-6 member share"
	for _, roster := range []int{wire.MaxConnectedMembers, 8} {
		t.Run(fmt.Sprintf("roster%d", roster), func(t *testing.T) {
			a, err := newNodeAdmission()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = a.Close() }()
			a.classify = classByName
			ids := memberIDs(roster)
			a.setRoster(ids)
			byz, honest := ids[0], ids[1:]
			held := 0
			for range 2 * wire.MaxConnectionsPerPeer {
				if _, err := openAs(a, network.DirInbound, "6.6.6.6", byz); err == nil {
					held++
				}
			}
			// Every other share owed: the Byzantine member's surplus is what the roster leaves.
			surplus := wire.MaxMemberConnections - wire.MemberShareConnections*roster
			want := min(wire.MaxConnectionsPerPeer, wire.MemberShareConnections+surplus)
			if held > wire.MaxConnectionsPerPeer || held != want {
				t.Fatalf("%s: the Byzantine member holds %d connections, want %d", rule, held, want)
			}
			// Fill the rest of the capacity with outsiders' (non-roster members') connections.
			for i := 0; ; i++ {
				if _, err := openAs(a, network.DirInbound, fmt.Sprintf("7.7.%d.1", i), peer.ID(fmt.Sprintf("member-x%03d", i))); err != nil {
					break
				}
			}
			for i, id := range honest {
				for k := range wire.MemberShareConnections {
					if _, err := openAs(a, network.DirOutbound, fmt.Sprintf("5.5.%d.1", i), id); err != nil {
						t.Fatalf("%s: honest member %s refused its reserved connection %d: %v", rule, id, k+1, err)
					}
				}
			}
			if got := a.ledger.Stats(a.now()).Members; got > wire.MaxMemberConnections {
				t.Fatalf("RF-6: %d member connections > %d", got, wire.MaxMemberConnections)
			}
		})
	}
}

// Review finding 1, at the layer: a member admitted while the surplus is taken makes the layer name, one per
// connection, the peers whose surplus connections must close for its share (RF-6, SharesKept).
func TestRosterGrowthShedsAtLayer(t *testing.T) {
	const rule = "RF-6 member share"
	a, err := newNodeAdmission()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	a.classify = classByName
	ids := memberIDs(32)
	a.setRoster(ids)
	for i, id := range ids {
		connsAt(t, a, network.DirOutbound, fmt.Sprintf("4.4.%d.1", i), id, wire.MaxConnectionsPerPeer, rule)
	}
	shed := a.setRoster(append(ids, "member-late"))
	if len(shed) != wire.MemberShareConnections {
		t.Fatalf("%s: %d connections named to shed (%v), want %d", rule, len(shed), shed, wire.MemberShareConnections)
	}
	for _, p := range shed {
		if !strings.HasPrefix(string(p), "member-") || p == "member-late" {
			t.Fatalf("%s: shed names %q, want a member holding surplus", rule, p)
		}
	}
}
