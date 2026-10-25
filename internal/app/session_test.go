package app_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func newSession(t *testing.T) (*app.Session, *p2p.Node) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	node, err := p2p.New(t.Context(), key, p2p.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Close() })
	return app.NewSession(node), node
}

func TestLocalPeerStable(t *testing.T) {
	s, node := newSession(t)
	if got, want := s.LocalPeer(), string(node.Local()); got != want || got != s.LocalPeer() {
		t.Errorf("LocalPeer() = %q, want stable %q", got, want)
	}
}

func TestJoinRejectsBadInvitation(t *testing.T) {
	s, _ := newSession(t)
	for _, text := range []string{"", "host", "192.168.1.20:7419", "/ip4/1.2.3.4/tcp/1", "a b"} {
		if _, err := s.Join(t.Context(), text); err == nil {
			t.Errorf("Join(%q) = nil, want an error", text)
		}
	}
}

func TestJoinRejectsOwnInvitation(t *testing.T) {
	s, _ := newSession(t)
	text, err := s.Host()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Join(t.Context(), text); err == nil {
		t.Error("Join(own invitation) = nil, want an error")
	}
}

func TestHostReturnsInvitation(t *testing.T) {
	s, node := newSession(t)
	text, err := s.Host()
	if err != nil {
		t.Fatalf("Host() = %v", err)
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		t.Fatal("Host() returned an empty invitation")
	}
	for _, a := range fields {
		if !strings.HasSuffix(a, "/p2p/"+string(node.Local())) {
			t.Errorf("invitation address %q does not end with the host identity", a)
		}
	}
	if again, err := s.Host(); err != nil || again != text {
		t.Errorf("second Host() = %q, %v; want the same invitation", again, err)
	}
}

func TestJoinConnectsToHost(t *testing.T) {
	host, _ := newSession(t)
	guest, _ := newSession(t)
	text, err := host.Host()
	if err != nil {
		t.Fatal(err)
	}
	peer, err := guest.Join(t.Context(), text)
	if err != nil {
		t.Fatalf("Join() = %v", err)
	}
	if peer != host.LocalPeer() {
		t.Errorf("Join() = %q, want the host's identity %q", peer, host.LocalPeer())
	}
	again, err := guest.Join(t.Context(), text)
	if err != nil || again != peer {
		t.Errorf("re-Join() = %q, %v; want %q, nil (already connected is fine)", again, err, peer)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !slices.Equal(guest.Peers(), []string{host.LocalPeer()}) {
		if time.Now().After(deadline) {
			t.Fatalf("guest.Peers() = %v, want [%s]", guest.Peers(), host.LocalPeer())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartNodeFallsBackWhenPortBusy(t *testing.T) {
	t.Setenv("INKWELL_HOME", t.TempDir())
	home, err := app.OpenHome()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })

	busy, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Skipf("no UDP socket: %v", err)
	}
	t.Cleanup(func() { _ = busy.Close() })
	port := busy.LocalAddr().(*net.UDPAddr).Port

	node, err := app.StartNode(t.Context(), home, port)
	if err != nil {
		t.Fatalf("StartNode(busy port %d) = %v, want a fallback", port, err)
	}
	t.Cleanup(func() { _ = node.Close() })
	for _, a := range node.Addrs() {
		if strings.Contains(a, "/udp/"+strconv.Itoa(port)+"/") {
			t.Errorf("node listens on the busy port: %s", a)
		}
	}
}

func TestStartNodeKeepsIdentity(t *testing.T) {
	t.Setenv("INKWELL_HOME", t.TempDir())
	var ids []string
	for range 2 {
		home, err := app.OpenHome()
		if err != nil {
			t.Fatal(err)
		}
		node, err := app.StartNode(t.Context(), home, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, string(node.Local()))
		_ = node.Close()
		_ = home.Close()
	}
	if ids[0] != ids[1] {
		t.Errorf("identity changed between runs: %q then %q", ids[0], ids[1])
	}
}

// wantRecorder is a Network that records the last Want.
type wantRecorder struct {
	app.Network
	local transport.PeerID
	want  map[transport.PeerID][]string
}

func (w *wantRecorder) Local() transport.PeerID { return w.local }

func (w *wantRecorder) Connect(context.Context, transport.PeerID, []string) error { return nil }

func (w *wantRecorder) Want(p map[transport.PeerID][]string) { w.want = p }

func TestJoinKeepsThePeerConnected(t *testing.T) {
	host, node := newSession(t)
	text, err := host.Host()
	if err != nil {
		t.Fatal(err)
	}
	rec := &wantRecorder{local: "12D3KooWGuest"}
	peer, err := app.NewSession(rec).Join(t.Context(), text)
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.want[transport.PeerID(peer)]; len(got) == 0 || peer != string(node.Local()) {
		t.Errorf("Want() after Join = %v, want the addresses of %s", rec.want, node.Local())
	}
}

func TestStartNodeRelayOnlyNeedsARelay(t *testing.T) {
	t.Setenv("INKWELL_HOME", t.TempDir())
	t.Setenv("INKWELL_NET", "relay-only")
	t.Setenv("INKWELL_RELAYS", "")
	home, err := app.OpenHome()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	for _, port := range []int{0, app.DefaultPort} {
		node, err := app.StartNode(t.Context(), home, port)
		if err == nil {
			_ = node.Close()
			t.Errorf("StartNode(port %d) with relay-only and no relay = nil, want an error (no silent direct mode)", port)
		}
	}
}

func TestStartNodeRejectsBadRelay(t *testing.T) {
	t.Setenv("INKWELL_HOME", t.TempDir())
	t.Setenv("INKWELL_RELAYS", "not-an-address")
	home, err := app.OpenHome()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	if node, err := app.StartNode(t.Context(), home, 0); err == nil {
		_ = node.Close()
		t.Error("StartNode(bad relay) = nil, want an error")
	}
}
