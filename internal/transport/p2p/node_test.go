package p2p_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// TestMain fails the run if a test leaves a goroutine behind. Nothing is
// ignored: Node.Close must stop everything libp2p started.
func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const (
	wait  = 10 * time.Second
	quiet = 200 * time.Millisecond
)

func recvWithin(t *testing.T, n transport.Transport, d time.Duration) transport.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), d)
	defer cancel()
	ev, err := n.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv(%s): %v", n.Local(), err)
	}
	return ev
}

func expectEvent(t *testing.T, n transport.Transport, kind transport.Kind, peer transport.PeerID) {
	t.Helper()
	got, want := recvWithin(t, n, wait), transport.Event{Kind: kind, Peer: peer}
	if !cmp.Equal(got, want) {
		t.Errorf("Recv(%s) event: %v", n.Local(), cmp.Diff(want, got))
	}
}

func expectQuietFor(t *testing.T, n transport.Transport, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), d)
	defer cancel()
	if ev, err := n.Recv(ctx); err == nil {
		t.Errorf("Recv(%s) unexpected event %v from %s", n.Local(), ev.Kind, ev.Peer)
	}
}

func expectUp(t *testing.T, a, b transport.Transport) {
	t.Helper()
	expectEvent(t, a, transport.PeerUp, b.Local())
	expectEvent(t, b, transport.PeerUp, a.Local())
}

func TestKeyIDRoundTrip(t *testing.T) {
	for seed := range byte(5) {
		k := key(seed)
		pub, _ := k.Public().(ed25519.PublicKey)
		id := p2p.IDFromKey(pub)
		if !transport.ValidPeerID(id) {
			t.Fatalf("IDFromKey(seed %d) = %q, want a valid PeerID", seed, id)
		}
		got, err := p2p.KeyFromID(id)
		if err != nil || !got.Equal(pub) {
			t.Errorf("KeyFromID(%q) = %x, %v; want %x", id, got, err, []byte(pub))
		}
	}
}

func TestKeyFromIDRejects(t *testing.T) {
	pub, _ := key(1).Public().(ed25519.PublicKey)
	canonical := p2p.IDFromKey(pub)
	pid, err := peer.Decode(string(canonical))
	if err != nil {
		t.Fatal(err)
	}
	_, secp, err := crypto.GenerateSecp256k1Key(bytes.NewReader(bytes.Repeat([]byte{7}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	secpID, err := peer.IDFromPublicKey(secp)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		id   transport.PeerID
	}{
		{"empty", ""},
		{"garbage", "nobody"},
		{"other key type", transport.PeerID(secpID.String())},
		{"non canonical encoding", transport.PeerID(peer.ToCid(pid).String())},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := p2p.KeyFromID(tc.id); err == nil {
				t.Errorf("KeyFromID(%q) = %x, want an error", tc.id, got)
			}
		})
	}
}

func TestIDFromKeyInvalid(t *testing.T) {
	if got := p2p.IDFromKey(ed25519.PublicKey("short")); got != "" {
		t.Errorf("IDFromKey(short key) = %q, want empty", got)
	}
}

func TestNewRejectsBadInput(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	tests := []struct {
		name string
		ctx  context.Context
		key  ed25519.PrivateKey
		opts p2p.Options
	}{
		{"short key", t.Context(), ed25519.PrivateKey("short"), p2p.Options{}},
		{"canceled context", canceled, key(1), p2p.Options{}},
		{"negative port", t.Context(), key(1), p2p.Options{Port: -1}},
		{"port too large", t.Context(), key(1), p2p.Options{Port: 65536}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := p2p.New(tc.ctx, tc.key, tc.opts)
			if err == nil {
				_ = n.Close()
				t.Fatal("New succeeded, want an error")
			}
		})
	}
}

// loopbackNode returns a node listening on the real loopback interface.
func loopbackNode(t *testing.T, seed byte, o p2p.Options) *p2p.Node {
	t.Helper()
	n, err := p2p.New(t.Context(), key(seed), o)
	if err != nil {
		t.Fatalf("New(seed %d): %v", seed, err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}

// loopbackAddrs returns the 127.0.0.1 addresses of n that contain sub.
func loopbackAddrs(n *p2p.Node, sub string) []string {
	var out []string
	for _, a := range n.Addrs() {
		if strings.HasPrefix(a, "/ip4/127.0.0.1/") && strings.Contains(a, sub) {
			out = append(out, a)
		}
	}
	return out
}

// TestLoopbackSmoke runs two nodes over the real loopback interface, once per
// transport, outside any synctest bubble.
func TestLoopbackSmoke(t *testing.T) {
	tests := []struct {
		name string
		sub  string // selects the dialed address
	}{
		{"QUIC", "/quic-v1"},
		{"TCP and Noise", "/tcp/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := loopbackNode(t, 1, p2p.Options{})
			b := loopbackNode(t, 2, p2p.Options{})
			addrs := loopbackAddrs(a, tc.sub)
			if len(addrs) == 0 {
				t.Fatalf("a.Addrs() = %v, want a loopback address containing %q", a.Addrs(), tc.sub)
			}
			for _, addr := range a.Addrs() {
				if !strings.HasSuffix(addr, "/p2p/"+string(a.Local())) {
					t.Errorf("Addrs() has %q, want the /p2p/<id> suffix", addr)
				}
			}
			if err := b.Connect(t.Context(), a.Local(), addrs); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			expectUp(t, a, b)
			assertConnTransport(t, b, a, tc.sub)

			if err := b.Send(t.Context(), a.Local(), []byte("hello")); err != nil {
				t.Fatalf("b.Send: %v", err)
			}
			if got := recvWithin(t, a, wait); got.Kind != transport.Message || got.Peer != b.Local() || string(got.Payload) != "hello" {
				t.Errorf("a got %v %q from %s, want hello from b", got.Kind, got.Payload, got.Peer)
			}
			if err := a.Send(t.Context(), b.Local(), []byte("back")); err != nil {
				t.Fatalf("a.Send: %v", err)
			}
			if got := recvWithin(t, b, wait); string(got.Payload) != "back" {
				t.Errorf("b got %q, want back", got.Payload)
			}
		})
	}
}

func assertConnTransport(t *testing.T, from, to *p2p.Node, sub string) {
	t.Helper()
	id, ok := p2p.LibP2PID(to.Local())
	if !ok {
		t.Fatalf("LibP2PID(%s) failed", to.Local())
	}
	conns := from.Host().Network().ConnsToPeer(id)
	if len(conns) == 0 {
		t.Fatalf("no connection to %s", to.Local())
	}
	if got := conns[0].RemoteMultiaddr().String(); !strings.Contains(got, sub) {
		t.Errorf("connection to %s uses %s, want a %q address", to.Local(), got, sub)
	}
}

func TestPortOption(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, _ := l.Addr().(*net.TCPAddr)
	port := addr.Port
	_ = l.Close()

	n := loopbackNode(t, 1, p2p.Options{Port: port})
	for _, want := range []string{
		fmt.Sprintf("/udp/%d/quic-v1/", port),
		fmt.Sprintf("/tcp/%d/", port),
	} {
		if len(loopbackAddrs(n, want)) == 0 {
			t.Errorf("Addrs() = %v, want an address containing %q", n.Addrs(), want)
		}
	}
}

func TestAdmitRefusesStranger(t *testing.T) {
	t.Run("GateRefusesConnection", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := newSimNet(t)
			friend := idOf(key(3))
			a := s.node(t, 1, "1.0.0.1", p2p.Options{Admit: func(id transport.PeerID) bool { return id == friend }})
			stranger := s.node(t, 2, "1.0.0.2", p2p.Options{})
			c := s.node(t, 3, "1.0.0.3", p2p.Options{})

			// The gate drops the connection once it is authenticated, so a never
			// even registers it; without the gate the connection would exist
			// for a moment, until the stranger finds its stream refused.
			var seen atomic.Int32
			a.Host().Network().Notify(&network.NotifyBundle{ConnectedF: func(network.Network, network.Conn) { seen.Add(1) }})
			_ = stranger.Connect(t.Context(), a.Local(), a.Addrs()) // may succeed on its side
			expectQuietFor(t, a, 5*time.Second)
			if got := seen.Load(); got != 0 {
				t.Errorf("a registered %d connections from a stranger, want none", got)
			}
			if got := a.Peers(); len(got) != 0 {
				t.Errorf("a.Peers() = %v after a stranger dialed, want none", got)
			}
			if got := stranger.Peers(); len(got) != 0 {
				t.Errorf("stranger.Peers() = %v, want none", got)
			}
			if err := stranger.Send(t.Context(), a.Local(), []byte("hi")); !errors.Is(err, transport.ErrUnknownPeer) {
				t.Errorf("stranger.Send = %v, want ErrUnknownPeer", err)
			}

			// A member is let in, so the refusal above is the gate's doing.
			s.connect(t, c, a)
			expectUp(t, a, c)
		})
	})

	t.Run("StreamResetAfterRevocation", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			s := newSimNet(t)
			var revoked atomic.Bool
			a := s.node(t, 1, "1.0.0.1", p2p.Options{Admit: func(transport.PeerID) bool { return !revoked.Load() }})
			b := s.node(t, 2, "1.0.0.2", p2p.Options{})
			s.connect(t, b, a)
			expectUp(t, a, b)
			// Once a has served b's own stream, which it admitted, revoke b.
			if err := b.Send(t.Context(), a.Local(), []byte("before")); err != nil {
				t.Fatalf("b.Send: %v", err)
			}
			if got := recvWithin(t, a, wait); got.Kind != transport.Message {
				t.Fatalf("a got %v, want the Message before", got.Kind)
			}

			revoked.Store(true)
			// The connection is still up; b opens one more sync stream by hand.
			aID, _ := p2p.LibP2PID(a.Local())
			st, err := b.Host().NewStream(t.Context(), aID, p2p.ProtocolSync)
			if err != nil {
				t.Fatalf("NewStream: %v", err)
			}
			if err := frame.Write(st, []byte("forged")); err != nil {
				t.Fatalf("frame.Write: %v", err)
			}
			if err := st.SetReadDeadline(time.Now().Add(wait)); err != nil {
				t.Fatalf("SetReadDeadline: %v", err)
			}
			if n, err := st.Read(make([]byte, 1)); err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Read on a refused stream = %d, %v; want a reset", n, err)
			}
			expectQuietFor(t, a, quiet)
		})
	})
}

func TestDropRemovesPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newSimNet(t)
		a := s.node(t, 1, "1.0.0.1", p2p.Options{})
		b := s.node(t, 2, "1.0.0.2", p2p.Options{})
		s.connect(t, a, b)
		expectUp(t, a, b)

		a.Drop("nobody") // not a libp2p identity: ignored
		a.Drop(a.Local())
		expectQuietFor(t, a, quiet)

		a.Drop(b.Local())
		expectEvent(t, a, transport.PeerDown, b.Local())
		expectEvent(t, b, transport.PeerDown, a.Local())
		if got := a.Peers(); len(got) != 0 {
			t.Errorf("a.Peers() after Drop = %v, want none", got)
		}
		if err := a.Send(t.Context(), b.Local(), nil); !errors.Is(err, transport.ErrUnknownPeer) {
			t.Errorf("Send after Drop = %v, want ErrUnknownPeer", err)
		}
	})
}

// TestIdleTimeoutPeerDown loses every packet without closing anything, so only
// QUIC's idle timeout can tell the nodes the link is gone.
func TestIdleTimeoutPeerDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newSimNet(t)
		a := s.node(t, 1, "1.0.0.1", p2p.Options{})
		b := s.node(t, 2, "1.0.0.2", p2p.Options{})
		s.connect(t, a, b)
		expectUp(t, a, b)

		s.cut.Store(true)
		for _, n := range []*p2p.Node{a, b} {
			other := b.Local()
			if n == b {
				other = a.Local()
			}
			if got, want := recvWithin(t, n, 2*time.Minute), (transport.Event{Kind: transport.PeerDown, Peer: other}); !cmp.Equal(got, want) {
				t.Errorf("Recv(%s) = %v, want %v", n.Local(), got, want)
			}
		}
		s.connect(t, a, b)
		expectUp(t, a, b)
		expectQuietFor(t, a, quiet)
	})
}

// TestSendCanceledMidFrame abandons a large frame half way: the stream is then
// in an unknown state, so it is replaced without any PeerDown or PeerUp, and
// the receiver never sees the truncated frame.
func TestSendCanceledMidFrame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newSimNet(t)
		a := s.node(t, 1, "1.0.0.1", p2p.Options{})
		b := s.node(t, 2, "1.0.0.2", p2p.Options{})
		s.connect(t, a, b)
		expectUp(t, a, b)

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		if err := a.Send(ctx, b.Local(), make([]byte, 32<<20)); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Send(32 MiB, 100 ms) = %v, want DeadlineExceeded", err)
		}
		if err := a.Send(t.Context(), b.Local(), []byte("after")); err != nil {
			t.Fatalf("Send after cancel: %v", err)
		}
		if got := recvWithin(t, b, wait); got.Kind != transport.Message || string(got.Payload) != "after" {
			t.Errorf("b got %v with %d bytes, want the Message after", got.Kind, len(got.Payload))
		}
		expectQuietFor(t, a, quiet)
		expectQuietFor(t, b, quiet)
	})
}

// TestRefusedOutboundStreamClosesPeer covers a remote that resets the stream
// we opened while the connection is healthy: the link is useless, so it is
// closed rather than retried in a loop.
func TestRefusedOutboundStreamClosesPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newSimNet(t)
		a := s.node(t, 1, "1.0.0.1", p2p.Options{})
		b := s.node(t, 2, "1.0.0.2", p2p.Options{})
		s.connect(t, a, b)
		expectUp(t, a, b)

		if !a.ResetOut(b.Local()) {
			t.Fatal("ResetOut: no outbound stream")
		}
		expectEvent(t, a, transport.PeerDown, b.Local())
		expectEvent(t, b, transport.PeerDown, a.Local())
	})
}

func TestConnectRejectsBadInput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newSimNet(t)
		a := s.node(t, 1, "1.0.0.1", p2p.Options{})
		b := s.node(t, 2, "1.0.0.2", p2p.Options{})
		tests := []struct {
			name  string
			peer  transport.PeerID
			addrs []string
		}{
			{"invalid peer", "nobody", b.Addrs()},
			{"ourselves", a.Local(), a.Addrs()},
			{"malformed address", b.Local(), []string{"not a multiaddr"}},
			{"address of another peer", b.Local(), []string{"/ip4/1.0.0.9/udp/8000/quic-v1/p2p/" + string(idOf(key(9)))}},
			{"no address", b.Local(), nil},
		}
		for _, tc := range tests {
			if err := a.Connect(t.Context(), tc.peer, tc.addrs); err == nil {
				t.Errorf("Connect(%s) succeeded, want an error", tc.name)
			}
		}
		// A bare address, without /p2p/, works: the peer is the one asked for.
		bare := strings.SplitN(b.Addrs()[0], "/p2p/", 2)[0]
		if err := a.Connect(t.Context(), b.Local(), []string{bare}); err != nil {
			t.Errorf("Connect(bare %s) = %v", bare, err)
		}
	})
}

func TestContextCancelClosesNode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := newSimNet(t)
		ctx, cancel := context.WithCancel(t.Context())
		n := s.nodeCtx(ctx, t, 1, "1.0.0.1", p2p.Options{})
		cancel()
		synctest.Wait()
		if _, err := n.Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
			t.Errorf("Recv after context cancel = %v, want ErrClosed", err)
		}
	})
}
