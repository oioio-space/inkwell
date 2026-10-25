package p2p_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// startRelay runs a relay on the real loopback interface until the test ends
// and returns its addresses.
func startRelay(t *testing.T, o p2p.RelayOptions) []string {
	t.Helper()
	ready := make(chan []string, 1)
	o.Listen = []string{"/ip4/127.0.0.1/udp/0/quic-v1", "/ip4/127.0.0.1/tcp/0"}
	o.OnReady = func(addrs []string) { ready <- addrs }
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- p2p.RunRelay(ctx, key(100), o) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("RunRelay returned %v after cancel, want nil", err)
		}
	})
	select {
	case addrs := <-ready:
		return addrs
	case err := <-done:
		t.Fatalf("RunRelay: %v", err)
	case <-time.After(wait):
		t.Fatal("RunRelay not ready")
	}
	return nil
}

// TestRelayOnlyLoopback: two nodes that accept nothing but circuits, through a
// relay run by RunRelay, all on the real loopback interface.
func TestRelayOnlyLoopback(t *testing.T) {
	addrs := startRelay(t, p2p.RelayOptions{})
	var quic []string
	for _, a := range addrs {
		if strings.Contains(a, "/quic-v1/") {
			quic = append(quic, a)
		}
	}
	if len(quic) == 0 {
		t.Fatalf("relay addresses %v, want a QUIC one", addrs)
	}
	a := loopbackNode(t, 1, p2p.Options{Relays: quic, RelayOnly: true})
	b := loopbackNode(t, 2, p2p.Options{Relays: quic, RelayOnly: true})
	a.Want(map[transport.PeerID][]string{b.Local(): nil})
	b.Want(map[transport.PeerID][]string{a.Local(): nil})
	expectUp(t, a, b)

	if err := a.Send(t.Context(), b.Local(), []byte("through the relay")); err != nil {
		t.Fatalf("a.Send: %v", err)
	}
	if got := recvWithin(t, b, wait); got.Kind != transport.Message || string(got.Payload) != "through the relay" {
		t.Errorf("b got %v %q, want the message", got.Kind, got.Payload)
	}
	if err := b.Send(t.Context(), a.Local(), []byte("and back")); err != nil {
		t.Fatalf("b.Send: %v", err)
	}
	if got := recvWithin(t, a, wait); string(got.Payload) != "and back" {
		t.Errorf("a got %q, want and back", got.Payload)
	}
	for _, n := range []*p2p.Node{a, b} {
		links := n.Links()
		if len(links) != 1 || links[0].Kind != p2p.LinkRelayed {
			t.Errorf("%s links = %v, want one relayed link", n.Local(), links)
		}
	}
}

// TestLinksLAN: a direct connection on the loopback interface is a LAN link,
// and its round trip is measured.
func TestLinksLAN(t *testing.T) {
	a := loopbackNode(t, 1, p2p.Options{})
	b := loopbackNode(t, 2, p2p.Options{})
	if got := a.Links(); len(got) != 0 {
		t.Errorf("Links() before any connection = %v, want none", got)
	}
	if err := b.Connect(t.Context(), a.Local(), loopbackAddrs(a, "/quic-v1")); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	expectUp(t, a, b)
	deadline := time.Now().Add(wait)
	for {
		links := b.Links()
		if len(links) == 1 && links[0].Peer == a.Local() && links[0].Kind == p2p.LinkLAN && links[0].RTT > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("b.Links() = %v, want one LAN link with a round trip time", links)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLinkKindString(t *testing.T) {
	tests := []struct {
		kind p2p.LinkKind
		want string
	}{
		{p2p.LinkLAN, "LinkLAN"},
		{p2p.LinkDirect, "LinkDirect"},
		{p2p.LinkRelayed, "LinkRelayed"},
		{0, "LinkKind(?)"},
	}
	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("LinkKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestRunRelayRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		opts p2p.RelayOptions
		bad  bool // key
	}{
		{"short key", p2p.RelayOptions{}, true},
		{"invalid allowed peer", p2p.RelayOptions{Allow: []transport.PeerID{"nobody"}}, false},
		{"invalid listen address", p2p.RelayOptions{Listen: []string{"not an address"}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k := key(1)
			if tc.bad {
				k = k[:10]
			}
			if err := p2p.RunRelay(t.Context(), k, tc.opts); err == nil {
				t.Error("RunRelay succeeded, want an error")
			}
		})
	}
}

// TestAddrsThroughALoopbackRelay: a node whose relay is not on a public address
// (here the loopback interface) is still given an address through it once it
// holds a slot, and another node dials that address alone.
func TestAddrsThroughALoopbackRelay(t *testing.T) {
	var quic []string
	for _, a := range startRelay(t, p2p.RelayOptions{}) {
		if strings.Contains(a, "/quic-v1/") {
			quic = append(quic, a)
		}
	}
	a := loopbackNode(t, 1, p2p.Options{Relays: quic, RelayOnly: true})
	b := loopbackNode(t, 2, p2p.Options{Relays: quic, RelayOnly: true})
	var circuit []string
	deadline := time.Now().Add(wait)
	for len(circuit) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		circuit = a.Addrs()
	}
	if len(circuit) == 0 || !strings.Contains(circuit[0], "/p2p-circuit/p2p/"+string(a.Local())) {
		t.Fatalf("a.Addrs() = %v, want an address through the relay ending with a's identity", circuit)
	}
	if err := b.Connect(t.Context(), a.Local(), circuit); err != nil {
		t.Fatalf("b.Connect(a, %v) = %v", circuit, err)
	}
}
