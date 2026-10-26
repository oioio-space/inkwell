package p2p_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// lobbyPair returns two connected loopback nodes; admit decides who a admits.
func lobbyPair(t *testing.T, admit func(transport.PeerID) bool) (a, b *p2p.Node) {
	t.Helper()
	a = loopbackNode(t, 1, p2p.Options{Admit: admit})
	b = loopbackNode(t, 2, p2p.Options{})
	return a, b
}

func connect(t *testing.T, from, to *p2p.Node) error {
	t.Helper()
	return from.Connect(t.Context(), to.Local(), loopbackAddrs(to, "/quic-v1"))
}

func connected(a, b *p2p.Node) bool {
	id, ok := p2p.LibP2PID(b.Local())
	return ok && a.Host().Network().Connectedness(id) == network.Connected
}

func TestLobbyRoundTrip(t *testing.T) {
	a, b := lobbyPair(t, nil)
	a.HandleLobby(func(_ context.Context, from transport.PeerID, req []byte) ([]byte, bool) {
		if from != b.Local() {
			return nil, false
		}
		return append([]byte("re:"), req...), true
	})
	if err := connect(t, b, a); err != nil {
		t.Fatal(err)
	}
	got, err := b.AskLobby(t.Context(), a.Local(), []byte("hello"))
	if err != nil || string(got) != "re:hello" {
		t.Errorf("AskLobby() = %q, %v; want re:hello", got, err)
	}
	// Several requests in a row, each on its own stream.
	for range 3 {
		if _, err := b.AskLobby(t.Context(), a.Local(), []byte("again")); err != nil {
			t.Fatalf("AskLobby() = %v", err)
		}
	}
	// A big answer, up to the limit.
	big := bytes.Repeat([]byte{7}, p2p.MaxLobbyResponse)
	a.HandleLobby(func(context.Context, transport.PeerID, []byte) ([]byte, bool) { return big, true })
	if got, err := b.AskLobby(t.Context(), a.Local(), nil); err != nil || !bytes.Equal(got, big) {
		t.Errorf("AskLobby(big answer) = %d bytes, %v; want %d", len(got), err, len(big))
	}
}

func TestLobbyGivesNoAnswer(t *testing.T) {
	a, b := lobbyPair(t, nil)
	if err := connect(t, b, a); err != nil {
		t.Fatal(err)
	}
	ask := func(req []byte) error {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		_, err := b.AskLobby(ctx, a.Local(), req)
		return err
	}
	if err := ask([]byte("x")); err == nil {
		t.Error("AskLobby() without a handler = nil, want an error")
	}
	called := make(chan struct{}, 4)
	a.HandleLobby(func(context.Context, transport.PeerID, []byte) ([]byte, bool) {
		called <- struct{}{}
		return nil, false
	})
	if err := ask([]byte("x")); err == nil {
		t.Error("AskLobby() with a handler that does not answer = nil, want an error")
	}
	if len(called) != 1 {
		t.Errorf("handler called %d times, want 1", len(called))
	}
	if _, err := b.AskLobby(t.Context(), a.Local(), make([]byte, p2p.MaxLobbyRequest+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("AskLobby(oversized request) = %v, want ErrFrameTooLarge", err)
	}
	a.HandleLobby(func(context.Context, transport.PeerID, []byte) ([]byte, bool) {
		return make([]byte, p2p.MaxLobbyResponse+1), true
	})
	if err := ask(nil); err == nil {
		t.Error("AskLobby() with an oversized answer = nil, want an error")
	}
	a.HandleLobby(nil)
	if err := ask(nil); err == nil {
		t.Error("AskLobby() after HandleLobby(nil) = nil, want an error")
	}
	if _, err := b.AskLobby(t.Context(), "not-a-peer", nil); err == nil {
		t.Error("AskLobby(bad peer) = nil, want an error")
	}
}

// TestOpenWindow: a stranger gets in, and can use the lobby, only while the
// window is open; closing it closes the stranger's connection.
func TestOpenWindow(t *testing.T) {
	a, b := lobbyPair(t, func(transport.PeerID) bool { return false })
	a.HandleLobby(func(context.Context, transport.PeerID, []byte) ([]byte, bool) { return []byte("ok"), true })
	ask := func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		if err := connect(t, b, a); err != nil {
			return err
		}
		_, err := b.AskLobby(ctx, a.Local(), nil)
		return err
	}

	if err := ask(); err == nil {
		t.Fatal("a stranger used the lobby while the window was closed")
	}

	a.SetOpen(time.Now().Add(time.Hour))
	if err := ask(); err != nil {
		t.Fatalf("stranger during the window: %v", err)
	}
	a.SetOpen(time.Time{})
	waitFor(t, "the stranger's connection to close", func() bool { return !connected(a, b) })
	if err := ask(); err == nil {
		t.Error("a stranger used the lobby after the window was closed")
	}

	// A window that ends by itself closes connections too.
	a.SetOpen(time.Now().Add(300 * time.Millisecond))
	if err := ask(); err != nil {
		t.Fatalf("stranger during a short window: %v", err)
	}
	waitFor(t, "the window to expire and close the connection", func() bool { return !connected(a, b) })
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNearby(t *testing.T) {
	plain := loopbackNode(t, 1, p2p.Options{})
	if got := plain.Nearby(); len(got) != 0 || plain.LANError() != nil {
		t.Errorf("a node without LAN: Nearby() = %v, LANError() = %v; want nothing", got, plain.LANError())
	}
	svc := testService()
	a := loopbackNode(t, 2, p2p.Options{LAN: true, LANService: svc})
	b := loopbackNode(t, 3, p2p.Options{LAN: true, LANService: svc})
	if err := errors.Join(a.LANError(), b.LANError()); err != nil {
		t.Skipf("mDNS is not available here: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		// Other nodes of the machine (other tests) may be seen too.
		found := a.Nearby()
		if i := slices.IndexFunc(found, func(n p2p.Nearby) bool { return n.ID == b.Local() }); i >= 0 && len(found[i].Addrs) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Skipf("mDNS multicast does not work on this machine: a sees %v", found)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
