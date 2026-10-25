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

// TestSecondNodeDoesNotShareThePort starts two nodes on the same port, as two
// Inkwell on one machine do: the second must listen elsewhere on TCP as well as
// UDP. With SO_REUSEPORT the kernel let both bind the TCP port and spread
// incoming connections between them, so handshakes reached the wrong process.
func TestSecondNodeDoesNotShareThePort(t *testing.T) {
	free, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("no TCP socket: %v", err)
	}
	port := free.Addr().(*net.TCPAddr).Port
	if err := free.Close(); err != nil {
		t.Fatal(err)
	}
	start := func() []string {
		t.Setenv("INKWELL_HOME", t.TempDir())
		home, err := app.OpenHome()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = home.Close() })
		node, err := app.StartNode(t.Context(), home, port)
		if err != nil {
			t.Fatalf("StartNode(%d) = %v", port, err)
		}
		t.Cleanup(func() { _ = node.Close() })
		return node.Addrs()
	}
	first := start()
	if !slices.ContainsFunc(first, func(a string) bool { return strings.Contains(a, "/tcp/"+strconv.Itoa(port)) }) {
		t.Skipf("first node did not get port %d (taken meanwhile): %v", port, first)
	}
	for _, a := range start() {
		if strings.Contains(a, "/tcp/"+strconv.Itoa(port)) || strings.Contains(a, "/udp/"+strconv.Itoa(port)+"/") {
			t.Errorf("second node listens on the first one's port: %s", a)
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
