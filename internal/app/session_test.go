package app_test

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/transport/tcp"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func newSession(t *testing.T) (*app.Session, *tcp.Node) {
	t.Helper()
	node := tcp.New(tcp.NewPeerID(), tcp.Options{})
	t.Cleanup(func() { _ = node.Close() })
	return app.NewSession(node), node
}

func TestLocalPeerStable(t *testing.T) {
	s, node := newSession(t)
	if got, want := s.LocalPeer(), string(node.Local()); got != want || got != s.LocalPeer() {
		t.Errorf("LocalPeer() = %q, want stable %q", got, want)
	}
}

func TestJoinRejectsBadAddress(t *testing.T) {
	s, _ := newSession(t)
	for _, addr := range []string{"", "host", "a:99999", ":8080", "host:0", "host:http", "a b:80"} {
		if _, err := s.Join(t.Context(), addr); err == nil {
			t.Errorf("Join(%q) = nil, want an error", addr)
		}
	}
}

func TestHostReturnsReachableAddrs(t *testing.T) {
	s, _ := newSession(t)
	addrs, err := s.Host(t.Context(), 0)
	if err != nil {
		t.Fatalf("Host(0) = %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("Host(0) returned no address")
	}
	for _, a := range addrs {
		host, port, err := net.SplitHostPort(a)
		if err != nil || port == "0" || host == "" {
			t.Errorf("Host(0) address %q is not a usable host:port", a)
		}
	}
	again, err := s.Host(t.Context(), 0)
	if err != nil || !slices.Equal(again, addrs) {
		t.Errorf("second Host(0) = %v, %v; want the same addresses %v", again, err, addrs)
	}
}

func TestJoinConnectsToHost(t *testing.T) {
	host, _ := newSession(t)
	guest, _ := newSession(t)
	addrs, err := host.Host(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(addrs[0])
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	peer, err := guest.Join(ctx, net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("Join() = %v", err)
	}
	if peer != host.LocalPeer() {
		t.Errorf("Join() = %q, want the host's identity %q", peer, host.LocalPeer())
	}
	again, err := guest.Join(ctx, net.JoinHostPort("127.0.0.1", port))
	if err != nil || again != peer {
		t.Errorf("re-Join() = %q, %v; want %q, nil (already connected is fine)", again, err, peer)
	}
	if got := guest.Peers(); !slices.Equal(got, []string{host.LocalPeer()}) {
		t.Errorf("guest.Peers() = %v, want [%s]", got, host.LocalPeer())
	}
}
