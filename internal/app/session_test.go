package app_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/app"
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
