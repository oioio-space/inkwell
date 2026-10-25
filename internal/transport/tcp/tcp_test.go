package tcp_test

import (
	"context"
	"errors"
	"io"
	"net"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	"github.com/oioio-space/inkwell/internal/transport/tcp"
	"github.com/oioio-space/inkwell/internal/transport/transporttest"
)

const (
	loopback = "127.0.0.1:0"
	wait     = 10 * time.Second
	quiet    = 200 * time.Millisecond
)

// newNode returns a listening node closed at the end of the test, and its address.
func newNode(t *testing.T, id transport.PeerID, o tcp.Options) (*tcp.Node, string) {
	t.Helper()
	n := tcp.New(id, o)
	t.Cleanup(func() { _ = n.Close() })
	addr, err := n.Listen(t.Context(), loopback)
	if err != nil {
		t.Fatalf("Listen(%s): %v", id, err)
	}
	return n, addr.String()
}

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func recv(t *testing.T, n transport.Transport) transport.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), wait)
	defer cancel()
	ev, err := n.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv(%s): %v", n.Local(), err)
	}
	return ev
}

// expectQuiet fails if an event shows up within the quiet period.
func expectQuiet(t *testing.T, n transport.Transport) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), quiet)
	defer cancel()
	if ev, err := n.Recv(ctx); err == nil {
		t.Errorf("Recv(%s) unexpected event %v from %s", n.Local(), ev.Kind, ev.Peer)
	}
}

func hello(id string) []byte { return append([]byte("INKW\x01"), id...) }

func TestOperationsAfterClose(t *testing.T) {
	a, _ := newNode(t, "a", tcp.Options{})
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := a.Dial(t.Context(), "127.0.0.1:1"); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("Dial after Close = %v, want ErrClosed", err)
	}
	if _, err := a.Listen(t.Context(), loopback); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("Listen after Close = %v, want ErrClosed", err)
	}
}

func TestSendCanceledContext(t *testing.T) {
	a, _ := newNode(t, "a", tcp.Options{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := a.Send(ctx, "nobody", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Send(canceled) = %v, want context.Canceled", err)
	}
}

func TestHandshakeRejectsSelf(t *testing.T) {
	a, addrA := newNode(t, "same", tcp.Options{})
	b, _ := newNode(t, "same", tcp.Options{})
	if peer, err := b.Dial(t.Context(), addrA); err == nil {
		t.Fatalf("Dial(self id) = %q, nil; want error", peer)
	}
	expectQuiet(t, a)
	expectQuiet(t, b)
}

func TestHandshakeRejectsBadHello(t *testing.T) {
	for name, hs := range map[string][]byte{
		"bad magic":     append([]byte("XXXX\x01"), "peer"...),
		"bad version":   append([]byte("INKW\x02"), "peer"...),
		"empty peer":    []byte("INKW\x01"),
		"space in peer": hello("a b"),
		"too short":     []byte("IN"),
		"empty":         {},
	} {
		t.Run(name, func(t *testing.T) {
			a, addrA := newNode(t, "a", tcp.Options{})
			nc, err := net.Dial("tcp", addrA)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = nc.Close() }()
			if err := frame.Write(nc, hs); err != nil {
				t.Fatal(err)
			}
			_ = nc.SetReadDeadline(time.Now().Add(wait))
			// a answers with its own hello then hangs up; either way the read ends in an error.
			for {
				if _, err := frame.Read(nc, 1<<10); err != nil {
					break
				}
			}
			expectQuiet(t, a)
		})
	}
}

func TestHandshakeTimeout(t *testing.T) {
	a, addrA := newNode(t, "a", tcp.Options{HandshakeTimeout: 100 * time.Millisecond})
	nc, err := net.Dial("tcp", addrA) // mute peer: never sends its hello
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	_ = nc.SetReadDeadline(time.Now().Add(wait))
	start := time.Now()
	for {
		if _, err := frame.Read(nc, 1<<10); err != nil {
			if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
				t.Fatalf("a did not hang up on a mute peer: %v", err)
			}
			break
		}
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("hang up took %v, want about the 100ms timeout", d)
	}
	expectQuiet(t, a)
}

func TestDialHandshakeTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", loopback) // accepts but never answers
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		nc, err := ln.Accept()
		if err == nil {
			defer func() { _ = nc.Close() }()
			time.Sleep(2 * time.Second)
		}
	}()
	a := tcp.New("a", tcp.Options{HandshakeTimeout: 100 * time.Millisecond})
	defer func() { _ = a.Close() }()
	if peer, err := a.Dial(t.Context(), ln.Addr().String()); err == nil {
		t.Errorf("Dial(mute server) = %q, nil; want timeout error", peer)
	}
}

func TestDialContextCanceledDuringHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		nc, err := ln.Accept()
		if err == nil {
			defer func() { _ = nc.Close() }()
			time.Sleep(time.Second)
		}
	}()
	a := tcp.New("a", tcp.Options{})
	defer func() { _ = a.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if peer, err := a.Dial(ctx, ln.Addr().String()); err == nil {
		t.Errorf("Dial(canceled) = %q, nil; want error", peer)
	}
}

func TestDialRefused(t *testing.T) {
	ln, err := net.Listen("tcp", loopback)
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	a := tcp.New("a", tcp.Options{})
	defer func() { _ = a.Close() }()
	if _, err := a.Dial(t.Context(), addr); err == nil {
		t.Error("Dial(closed port) = nil error")
	}
}

func TestSimultaneousDialKeepsOneLink(t *testing.T) {
	for round := range 20 {
		a, addrA := newNode(t, "a", tcp.Options{})
		b, addrB := newNode(t, "b", tcp.Options{})
		errs := make(chan error, 2)
		go func() { _, err := a.Dial(t.Context(), addrB); errs <- err }()
		go func() { _, err := b.Dial(t.Context(), addrA); errs <- err }()
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatalf("round %d: Dial: %v", round, err)
			}
		}
		for _, n := range []*tcp.Node{a, b} {
			other := transport.PeerID("b")
			if n == b {
				other = "a"
			}
			if got, want := recv(t, n), (transport.Event{Kind: transport.PeerUp, Peer: other}); !cmp.Equal(got, want) {
				t.Fatalf("round %d: %s first event: %v", round, n.Local(), cmp.Diff(want, got))
			}
		}
		// No PeerDown, no second PeerUp: the link does not flap.
		expectQuiet(t, a)
		expectQuiet(t, b)
		if err := a.Send(t.Context(), "b", []byte("x")); err != nil {
			t.Fatalf("round %d: a.Send: %v", round, err)
		}
		if err := b.Send(t.Context(), "a", []byte("y")); err != nil {
			t.Fatalf("round %d: b.Send: %v", round, err)
		}
		if got := recv(t, b); string(got.Payload) != "x" {
			t.Errorf("round %d: b got %v %q, want x", round, got.Kind, got.Payload)
		}
		if got := recv(t, a); string(got.Payload) != "y" {
			t.Errorf("round %d: a got %v %q, want y", round, got.Kind, got.Payload)
		}
		_ = a.Close()
		_ = b.Close()
	}
}

func TestRedialReplacesLink(t *testing.T) {
	a, _ := newNode(t, "a", tcp.Options{})
	b, addrB := newNode(t, "b", tcp.Options{})
	for range 2 {
		if _, err := a.Dial(t.Context(), addrB); err != nil {
			t.Fatal(err)
		}
	}
	recv(t, a)
	recv(t, b)
	expectQuiet(t, a)
	expectQuiet(t, b)
	if err := a.Send(t.Context(), "b", []byte("z")); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, b); string(got.Payload) != "z" {
		t.Errorf("b got %q, want z", got.Payload)
	}
}

func TestPeerDropMidFrameEmitsSinglePeerDown(t *testing.T) {
	a, addrA := newNode(t, "a", tcp.Options{})
	nc, err := net.Dial("tcp", addrA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	if err := frame.Write(nc, hello("raw")); err != nil {
		t.Fatal(err)
	}
	if _, err := frame.Read(nc, 1<<10); err != nil {
		t.Fatal(err)
	}
	if got, want := recv(t, a), (transport.Event{Kind: transport.PeerUp, Peer: "raw"}); !cmp.Equal(got, want) {
		t.Fatalf("first event: %v", cmp.Diff(want, got))
	}
	// A header announcing 100 bytes, then only 10, then the connection drops.
	if _, err := nc.Write(append([]byte{0, 0, 0, 100}, make([]byte, 10)...)); err != nil {
		t.Fatal(err)
	}
	_ = nc.Close()
	if got, want := recv(t, a), (transport.Event{Kind: transport.PeerDown, Peer: "raw"}); !cmp.Equal(got, want) {
		t.Errorf("second event: %v", cmp.Diff(want, got))
	}
	expectQuiet(t, a)
	if got := a.Peers(); len(got) != 0 {
		t.Errorf("Peers() = %v, want none", got)
	}
}

func TestOversizeFrameDropsPeer(t *testing.T) {
	a, addrA := newNode(t, "a", tcp.Options{})
	nc, err := net.Dial("tcp", addrA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	_ = frame.Write(nc, hello("raw"))
	_, _ = frame.Read(nc, 1<<10)
	recv(t, a) // PeerUp
	if _, err := nc.Write([]byte{0xff, 0xff, 0xff, 0xff}); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, a); got.Kind != transport.PeerDown {
		t.Errorf("event after oversize header = %v, want PeerDown", got.Kind)
	}
}

func TestSendToDeadPeerFailsAndDropsLink(t *testing.T) {
	a, _ := newNode(t, "a", tcp.Options{WriteTimeout: 200 * time.Millisecond})
	b, addrB := newNode(t, "b", tcp.Options{})
	if _, err := a.Dial(t.Context(), addrB); err != nil {
		t.Fatal(err)
	}
	recv(t, a)
	recv(t, b)
	_ = b.Close()
	// Writes to a closed peer eventually fail (RST); the link then goes down once.
	payload := make([]byte, 1<<20)
	for range 50 {
		if err := a.Send(t.Context(), "b", payload); err != nil {
			break
		}
	}
	if got := recv(t, a); got.Kind != transport.PeerDown || got.Peer != "b" {
		t.Errorf("a event = %v %s, want PeerDown b", got.Kind, got.Peer)
	}
	expectQuiet(t, a)
}

func TestListenContextStopsAccepting(t *testing.T) {
	a := tcp.New("a", tcp.Options{})
	defer func() { _ = a.Close() }()
	ctx, cancel := context.WithCancel(t.Context())
	addr, err := a.Listen(ctx, loopback)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	b := tcp.New("b", tcp.Options{HandshakeTimeout: time.Second})
	defer func() { _ = b.Close() }()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if _, err := b.Dial(t.Context(), addr.String()); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("listener kept accepting after its context was canceled")
}

func TestListenBadAddress(t *testing.T) {
	a := tcp.New("a", tcp.Options{})
	defer func() { _ = a.Close() }()
	if _, err := a.Listen(t.Context(), "not an address"); err == nil {
		t.Error("Listen(bad addr) = nil error")
	}
}

func TestNewPanicsOnInvalidID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New(\"\") did not panic")
		}
	}()
	tcp.New("", tcp.Options{})
}

func TestNewPeerID(t *testing.T) {
	re := regexp.MustCompile(`^[a-z2-7]{26}$`)
	seen := map[transport.PeerID]bool{}
	for range 100 {
		id := tcp.NewPeerID()
		if !re.MatchString(string(id)) || !transport.ValidPeerID(id) {
			t.Fatalf("NewPeerID() = %q, want 26 lowercase base32 chars", id)
		}
		if seen[id] {
			t.Fatalf("NewPeerID() repeated %q", id)
		}
		seen[id] = true
	}
}

// rawPeer connects to addr and completes the handshake claiming id.
func rawPeer(t *testing.T, addr, id string) net.Conn {
	t.Helper()
	nc, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = nc.Close() })
	if err := frame.Write(nc, hello(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := frame.Read(nc, 1<<10); err != nil {
		t.Fatalf("raw handshake as %s: %v", id, err)
	}
	return nc
}

// eofWithin reports whether nc is closed by the remote within d.
func eofWithin(nc net.Conn, d time.Duration) bool {
	_ = nc.SetReadDeadline(time.Now().Add(d))
	_, err := io.Copy(io.Discard, nc) // drain the node's hello, if any
	ne, ok := errors.AsType[net.Error](err)
	return err == nil || !ok || !ne.Timeout()
}

func TestPendingHandshakesAreCapped(t *testing.T) {
	_, addr := newNode(t, "a", tcp.Options{MaxPendingHandshakes: 2, HandshakeTimeout: time.Minute})
	var mute []net.Conn
	for range 2 {
		nc, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = nc.Close() })
		mute = append(mute, nc)
	}
	time.Sleep(100 * time.Millisecond) // let the node account for both
	third, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = third.Close() }()
	if !eofWithin(third, 5*time.Second) {
		t.Error("third pending connection was not closed immediately")
	}
	if eofWithin(mute[0], 100*time.Millisecond) {
		t.Error("an admitted pending connection was closed")
	}
}

func TestMaxPeers(t *testing.T) {
	a, addr := newNode(t, "a", tcp.Options{MaxPeers: 1})
	rawPeer(t, addr, "p1")
	if got, want := recv(t, a), (transport.Event{Kind: transport.PeerUp, Peer: "p1"}); !cmp.Equal(got, want) {
		t.Fatalf("first event: %v", cmp.Diff(want, got))
	}
	nc, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	_ = frame.Write(nc, hello("p2"))
	if !eofWithin(nc, 5*time.Second) {
		t.Error("connection beyond MaxPeers was kept")
	}
	if got, want := a.Peers(), []transport.PeerID{"p1"}; !slices.Equal(got, want) {
		t.Errorf("Peers() = %v, want %v", got, want)
	}
}

func TestByteBudgetBackpressure(t *testing.T) {
	// A budget far smaller than the traffic, and than one frame: messages
	// still all arrive, in order, one buffered at a time.
	a, _ := newNode(t, "a", tcp.Options{})
	b, addrB := newNode(t, "b", tcp.Options{MaxBufferedBytes: 1000})
	if _, err := a.Dial(t.Context(), addrB); err != nil {
		t.Fatal(err)
	}
	recv(t, a)
	recv(t, b)
	sizes := []int{600, 600, 5000, 10, 600}
	go func() {
		for i, size := range sizes {
			p := make([]byte, size)
			p[0] = byte(i)
			if err := a.Send(t.Context(), "b", p); err != nil {
				return
			}
		}
	}()
	for i, size := range sizes {
		got := recv(t, b)
		if len(got.Payload) != size || got.Payload[0] != byte(i) {
			t.Fatalf("message %d: %d bytes first=%d, want %d bytes first=%d", i, len(got.Payload), got.Payload[0], size, i)
		}
	}
}

func TestLosingConnMessagesAreDroppedAndDisconnectKeepsLink(t *testing.T) {
	// a < z: the link a dialed (initiator a) beats any connection claiming z.
	a, addrA := newNode(t, "a", tcp.Options{})
	z, addrZ := newNode(t, "z", tcp.Options{})
	if _, err := a.Dial(t.Context(), addrZ); err != nil {
		t.Fatal(err)
	}
	recv(t, a)
	recv(t, z)
	evil := rawPeer(t, addrA, "z") // attacker claims to be z
	if err := frame.Write(evil, []byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := z.Send(t.Context(), "a", []byte("good")); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, a); string(got.Payload) != "good" || got.Kind != transport.Message {
		t.Fatalf("a got %v %q, want the real peer's message first", got.Kind, got.Payload)
	}
	_ = evil.Close()
	expectQuiet(t, a)
	if got, want := a.Peers(), []transport.PeerID{"z"}; !slices.Equal(got, want) {
		t.Errorf("Peers() = %v, want %v", got, want)
	}
}

func TestSpoofedLinkDisconnectDoesNotCutRealLink(t *testing.T) {
	// z dialed m, so the link is z-initiated; the attacker claiming z ties
	// and stays a spare. When it leaves, the real link is untouched and no
	// PeerDown is reported.
	m, addrM := newNode(t, "m", tcp.Options{})
	z, _ := newNode(t, "z", tcp.Options{})
	if _, err := z.Dial(t.Context(), addrM); err != nil {
		t.Fatal(err)
	}
	recv(t, m)
	recv(t, z)
	evil := rawPeer(t, addrM, "z")
	time.Sleep(100 * time.Millisecond)
	_ = evil.Close()
	expectQuiet(t, m)
	if got, want := m.Peers(), []transport.PeerID{"z"}; !slices.Equal(got, want) {
		t.Fatalf("Peers() = %v, want %v", got, want)
	}
	if err := m.Send(t.Context(), "z", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, z); string(got.Payload) != "hi" {
		t.Errorf("z got %q, want hi", got.Payload)
	}
	if s := m.Stats(); s.Replaced != 0 {
		t.Errorf("Stats().Replaced = %d, want 0: a tie must not take over the link", s.Replaced)
	}
}

func TestStatsCountsFailedHandshakes(t *testing.T) {
	a, addr := newNode(t, "a", tcp.Options{})
	nc, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	_ = frame.Write(nc, []byte("garbage"))
	eofWithin(nc, 5*time.Second)
	if s := a.Stats(); s.FailedHandshakes != 1 {
		t.Errorf("Stats().FailedHandshakes = %d, want 1", s.FailedHandshakes)
	}
}

func TestSendCanceledInterruptsWrite(t *testing.T) {
	a, _ := newNode(t, "a", tcp.Options{})
	ln, err := net.Listen("tcp", loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() { // a peer that handshakes then never reads
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		_ = frame.Write(nc, hello("slow"))
		_, _ = frame.Read(nc, 1<<10)
		accepted <- nc
	}()
	if _, err := a.Dial(t.Context(), ln.Addr().String()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = (<-accepted).Close() }()
	recv(t, a)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = a.Send(ctx, "slow", make([]byte, 48<<20))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Send() = %v, want DeadlineExceeded", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Send took %v after cancellation", d)
	}
}

func TestRedialAlreadyLinkedIsIdempotent(t *testing.T) {
	a, addrA := newNode(t, "a", tcp.Options{})
	b, _ := newNode(t, "b", tcp.Options{})
	if _, err := b.Dial(t.Context(), addrA); err != nil {
		t.Fatal(err)
	}
	if got, want := recv(t, a), (transport.Event{Kind: transport.PeerUp, Peer: "b"}); !cmp.Equal(got, want) {
		t.Errorf("a first event: %v", cmp.Diff(want, got))
	}
	if got, want := recv(t, b), (transport.Event{Kind: transport.PeerUp, Peer: "a"}); !cmp.Equal(got, want) {
		t.Errorf("b first event: %v", cmp.Diff(want, got))
	}
	for i := range 10 {
		peer, err := b.Dial(t.Context(), addrA)
		if err != nil || peer != "a" {
			t.Fatalf("Dial #%d = %q, %v; want a, nil", i, peer, err)
		}
		if got, want := a.Peers(), []transport.PeerID{"b"}; !slices.Equal(got, want) {
			t.Fatalf("Dial #%d: a.Peers() = %v, want %v", i, got, want)
		}
		if got, want := b.Peers(), []transport.PeerID{"a"}; !slices.Equal(got, want) {
			t.Fatalf("Dial #%d: b.Peers() = %v, want %v", i, got, want)
		}
	}
	expectQuiet(t, a)
	expectQuiet(t, b)
	// The redialed connections end on both sides; one conn per peer remains.
	deadline := time.Now().Add(wait)
	for a.ConnCount("b") != 1 || b.ConnCount("a") != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("conns: a has %d for b, b has %d for a; want 1 each", a.ConnCount("b"), b.ConnCount("a"))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := a.Send(t.Context(), "b", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if got := recv(t, b); string(got.Payload) != "ok" {
		t.Errorf("b got %q, want ok", got.Payload)
	}
	if s := a.Stats(); s.Replaced != 0 {
		t.Errorf("a.Stats().Replaced = %d, want 0", s.Replaced)
	}
}

// cutProxy forwards TCP connections to target until Cut closes them all,
// which loses a link the way a dead network would, without closing a node.
type cutProxy struct {
	ln    net.Listener
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
	mu    sync.Mutex
}

func newCutProxy(t *testing.T, target string) *cutProxy {
	t.Helper()
	ln, err := net.Listen("tcp", loopback)
	if err != nil {
		t.Fatal(err)
	}
	p := &cutProxy{ln: ln, conns: make(map[net.Conn]struct{})}
	p.wg.Go(func() {
		for {
			down, err := ln.Accept()
			if err != nil {
				return
			}
			up, err := net.Dial("tcp", target)
			if err != nil {
				_ = down.Close()
				continue
			}
			p.mu.Lock()
			p.conns[down], p.conns[up] = struct{}{}, struct{}{}
			p.mu.Unlock()
			p.wg.Go(func() { pipe(down, up) })
			p.wg.Go(func() { pipe(up, down) })
		}
	})
	t.Cleanup(func() {
		_ = ln.Close()
		p.cut()
		p.wg.Wait()
	})
	return p
}

// pipe copies src to dst, then closes both.
func pipe(dst, src net.Conn) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	_ = src.Close()
}

func (p *cutProxy) cut() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.conns {
		_ = c.Close()
	}
	clear(p.conns)
}

func TestContract(t *testing.T) {
	transporttest.Run(t, func(t *testing.T) transporttest.Pair {
		a, _ := newNode(t, "a", tcp.Options{})
		b, addrB := newNode(t, "b", tcp.Options{})
		proxy := newCutProxy(t, addrB)
		return transporttest.Pair{
			A: a,
			B: b,
			Connect: func(t testing.TB) {
				if _, err := a.Dial(t.Context(), proxy.ln.Addr().String()); err != nil {
					t.Errorf("Dial: %v", err)
				}
			},
			Cut: func(testing.TB) { proxy.cut() },
		}
	}, transporttest.Options{Ordered: true})
}
