// Package transporttest is the behavioural contract every
// [transport.Transport] implementation must satisfy.
//
// An implementation's tests describe how to build two unconnected peers with
// a [Pair] and call [Run]:
//
//	func TestContract(t *testing.T) {
//		transporttest.Run(t, newPair, transporttest.Options{Ordered: true})
//	}
package transporttest

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	gocmp "github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/transport"
)

const (
	// wait bounds how long the suite waits for an event that must happen.
	wait = 10 * time.Second
	// quiet is how long the suite watches for an event that must not happen.
	quiet = 200 * time.Millisecond
	// defaultMaxFrame is the largest payload sent when Options.MaxFrame is zero.
	defaultMaxFrame = 1 << 20
)

// Pair is two unconnected transports and the means to link and break them.
type Pair struct {
	// A and B are the two peers, with distinct valid [transport.PeerID]s.
	A, B transport.Transport
	// Connect links A and B. It is idempotent, may be called from several
	// goroutines at once, and so must report failure with t.Error, never
	// t.Fatal.
	Connect func(t testing.TB)
	// Cut abruptly loses the link between A and B, without closing either.
	Cut func(t testing.TB)
}

// Options describes what an implementation guarantees beyond the contract.
type Options struct {
	// MaxFrame is the size of the largest payload the suite sends. Zero
	// means 1 MiB.
	MaxFrame int
	// Ordered says that messages from one sender arrive in sending order.
	Ordered bool
	// Synctest runs every subtest inside a [testing/synctest] bubble, for
	// implementations whose network and clock are virtual. newPair then
	// receives the bubble's t.
	Synctest bool
}

// Run runs the contract against pairs built by newPair, one fresh pair per
// subtest. Both transports of a pair are closed when its subtest ends.
func Run(t *testing.T, newPair func(t *testing.T) Pair, o Options) {
	o.MaxFrame = cmp.Or(o.MaxFrame, defaultMaxFrame)
	type subtest struct {
		name string
		fn   func(*testing.T, Pair, Options)
	}
	tests := []subtest{
		{"PeerUpBothSides", peerUpBothSides},
		{"DeliversIntactToTargetOnly", deliversIntactToTargetOnly},
		{"SendErrors", sendErrors},
		{"CloseNotifiesPeerOnce", closeNotifiesPeerOnce},
		{"CutEmitsSinglePeerDown", cutEmitsSinglePeerDown},
		{"ReconnectAfterCut", reconnectAfterCut},
		{"ConcurrentConnectNoFlap", concurrentConnectNoFlap},
		{"BackpressureNoLoss", backpressureNoLoss},
		{"RecvHonorsContext", recvHonorsContext},
		{"RecvBlockedThenClose", recvBlockedThenClose},
		{"PeersSorted", peersSorted},
	}
	if o.Ordered {
		tests = append(tests, subtest{"OrderedPerSender", orderedPerSender})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := func(t *testing.T) {
				p := newPair(t)
				t.Cleanup(func() {
					_ = p.A.Close()
					_ = p.B.Close()
				})
				tc.fn(t, p, o)
			}
			if o.Synctest {
				synctest.Test(t, body)
				return
			}
			body(t)
		})
	}
}

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

func expectEvent(t *testing.T, n transport.Transport, kind transport.Kind, peer transport.PeerID) {
	t.Helper()
	got, want := recv(t, n), transport.Event{Kind: kind, Peer: peer}
	if !gocmp.Equal(got, want) {
		t.Errorf("Recv(%s) event: %v", n.Local(), gocmp.Diff(want, got))
	}
}

// connect links the pair and consumes the PeerUp each side must see.
func connect(t *testing.T, p Pair) {
	t.Helper()
	p.Connect(t)
	expectEvent(t, p.A, transport.PeerUp, p.B.Local())
	expectEvent(t, p.B, transport.PeerUp, p.A.Local())
}

func peerUpBothSides(t *testing.T, p Pair, _ Options) {
	if a, b := p.A.Local(), p.B.Local(); a == b || !transport.ValidPeerID(a) || !transport.ValidPeerID(b) {
		t.Fatalf("Local() = %q and %q, want distinct valid IDs", a, b)
	}
	if got := p.A.Peers(); len(got) != 0 {
		t.Errorf("A.Peers() before Connect = %v, want none", got)
	}
	connect(t, p)
	if got, want := p.A.Peers(), []transport.PeerID{p.B.Local()}; !slices.Equal(got, want) {
		t.Errorf("A.Peers() = %v, want %v", got, want)
	}
	if got, want := p.B.Peers(), []transport.PeerID{p.A.Local()}; !slices.Equal(got, want) {
		t.Errorf("B.Peers() = %v, want %v", got, want)
	}
	expectQuiet(t, p.A)
	expectQuiet(t, p.B)
}

// payload returns n bytes derived from seed, so corruption and mix-ups show.
func payload(n, seed int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + seed)
	}
	return b
}

func deliversIntactToTargetOnly(t *testing.T, p Pair, o Options) {
	connect(t, p)
	for i, n := range []int{5, 0, o.MaxFrame} {
		msg := payload(n, i)
		if err := p.A.Send(t.Context(), p.B.Local(), msg); err != nil {
			t.Fatalf("A.Send(%d bytes): %v", n, err)
		}
		clear(msg) // Send must not retain the caller's buffer.
		got := recv(t, p.B)
		if got.Kind != transport.Message || got.Peer != p.A.Local() || !bytes.Equal(got.Payload, payload(n, i)) {
			t.Errorf("B got %v from %s with %d bytes, want an intact Message from A with %d bytes",
				got.Kind, got.Peer, len(got.Payload), n)
		}
	}
	if err := p.B.Send(t.Context(), p.A.Local(), []byte("back")); err != nil {
		t.Fatalf("B.Send: %v", err)
	}
	if got := recv(t, p.A); got.Kind != transport.Message || got.Peer != p.B.Local() || string(got.Payload) != "back" {
		t.Errorf("A got %v %q from %s, want back from B", got.Kind, got.Payload, got.Peer)
	}
	// A's messages go to B only: nothing is echoed to A.
	expectQuiet(t, p.A)
}

func orderedPerSender(t *testing.T, p Pair, _ Options) {
	connect(t, p)
	const total = 200
	sendAll(t, p.A, p.B.Local(), total, 16)
	for i := range total {
		got := recv(t, p.B)
		if want := payload(16, i); !bytes.Equal(got.Payload, want) {
			t.Fatalf("message %d = %v, want %v", i, got.Payload, want)
		}
	}
}

// sendAll sends total messages of size bytes, message i being payload(size, i),
// from a goroutine that is awaited when the test ends.
func sendAll(t *testing.T, from transport.Transport, to transport.PeerID, total, size int) {
	t.Helper()
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait) // runs after t.Context() is canceled, which unblocks Send.
	wg.Go(func() {
		for i := range total {
			if err := from.Send(t.Context(), to, payload(size, i)); err != nil {
				if t.Context().Err() == nil {
					t.Errorf("Send #%d: %v", i, err)
				}
				return
			}
		}
	})
}

func sendErrors(t *testing.T, p Pair, _ Options) {
	if err := p.A.Send(t.Context(), "nobody", nil); !errors.Is(err, transport.ErrUnknownPeer) {
		t.Errorf("Send(unknown peer) = %v, want ErrUnknownPeer", err)
	}
	connect(t, p)
	if err := p.A.Send(t.Context(), p.B.Local(), make([]byte, transport.MaxFrame+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Send(MaxFrame+1) = %v, want ErrFrameTooLarge", err)
	}
	if err := p.A.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.A.Send(t.Context(), p.B.Local(), nil); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("Send after Close = %v, want ErrClosed", err)
	}
	if _, err := p.A.Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("Recv after Close = %v, want ErrClosed", err)
	}
}

func closeNotifiesPeerOnce(t *testing.T, p Pair, _ Options) {
	connect(t, p)
	for range 2 { // Close is idempotent
		if err := p.A.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
	expectEvent(t, p.B, transport.PeerDown, p.A.Local())
	expectQuiet(t, p.B)
	if got := p.B.Peers(); len(got) != 0 {
		t.Errorf("B.Peers() after A.Close = %v, want none", got)
	}
}

func cutEmitsSinglePeerDown(t *testing.T, p Pair, _ Options) {
	connect(t, p)
	p.Cut(t)
	expectEvent(t, p.A, transport.PeerDown, p.B.Local())
	expectEvent(t, p.B, transport.PeerDown, p.A.Local())
	expectQuiet(t, p.A)
	expectQuiet(t, p.B)
	if a, b := p.A.Peers(), p.B.Peers(); len(a) != 0 || len(b) != 0 {
		t.Errorf("Peers() after Cut = %v and %v, want none", a, b)
	}
	if err := p.A.Send(t.Context(), p.B.Local(), nil); !errors.Is(err, transport.ErrUnknownPeer) {
		t.Errorf("Send after Cut = %v, want ErrUnknownPeer", err)
	}
}

func reconnectAfterCut(t *testing.T, p Pair, _ Options) {
	connect(t, p)
	p.Cut(t)
	expectEvent(t, p.A, transport.PeerDown, p.B.Local())
	expectEvent(t, p.B, transport.PeerDown, p.A.Local())
	connect(t, p)
	expectQuiet(t, p.A)
	expectQuiet(t, p.B)
	if err := p.A.Send(t.Context(), p.B.Local(), []byte("again")); err != nil {
		t.Fatalf("Send after reconnect: %v", err)
	}
	if got := recv(t, p.B); string(got.Payload) != "again" {
		t.Errorf("B got %q after reconnect, want again", got.Payload)
	}
}

func concurrentConnectNoFlap(t *testing.T, p Pair, _ Options) {
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { p.Connect(t) })
	}
	wg.Wait()
	expectEvent(t, p.A, transport.PeerUp, p.B.Local())
	expectEvent(t, p.B, transport.PeerUp, p.A.Local())
	// No PeerDown, no second PeerUp: the link does not flap.
	expectQuiet(t, p.A)
	expectQuiet(t, p.B)
	if err := p.A.Send(t.Context(), p.B.Local(), []byte("x")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := recv(t, p.B); string(got.Payload) != "x" {
		t.Errorf("B got %v %q, want x", got.Kind, got.Payload)
	}
}

func backpressureNoLoss(t *testing.T, p Pair, o Options) {
	connect(t, p)
	// Several MiB, far more than a queue holds, sent before B reads any.
	// Message i is payload(size, i), whose first byte is i: total <= 256.
	const size, total = 64 << 10, 256
	sendAll(t, p.A, p.B.Local(), total, size)
	time.Sleep(100 * time.Millisecond) // a slow receiver
	var seen [total]bool
	for n := range total {
		got := recv(t, p.B)
		if got.Kind != transport.Message || len(got.Payload) != size {
			t.Fatalf("event %d = %v with %d bytes, want a %d-byte Message", n, got.Kind, len(got.Payload), size)
		}
		i := int(got.Payload[0])
		switch {
		case !bytes.Equal(got.Payload, payload(size, i)):
			t.Fatalf("event %d is corrupt", n)
		case o.Ordered && i != n:
			t.Fatalf("event %d is message %d, want in order", n, i)
		case seen[i]:
			t.Fatalf("message %d delivered twice", i)
		}
		seen[i] = true
	}
}

func recvHonorsContext(t *testing.T, p Pair, _ Options) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.A.Recv(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Recv(idle, timeout) = %v, want DeadlineExceeded", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	cancel()
	if _, err := p.A.Recv(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Recv(idle, canceled) = %v, want Canceled", err)
	}
}

func recvBlockedThenClose(t *testing.T, p Pair, _ Options) {
	res := make(chan error, 1)
	go func() {
		_, err := p.A.Recv(t.Context())
		res <- err
	}()
	time.Sleep(50 * time.Millisecond)
	_ = p.A.Close()
	select {
	case err := <-res:
		if !errors.Is(err, transport.ErrClosed) {
			t.Errorf("blocked Recv = %v, want ErrClosed", err)
		}
	case <-time.After(wait):
		t.Fatal("Recv still blocked after Close")
	}
}

// peersSorted checks the Peers lifecycle. A Pair has a single remote peer
// each, so the order itself is only checked on that list.
func peersSorted(t *testing.T, p Pair, _ Options) {
	connect(t, p)
	if got := p.A.Peers(); !slices.IsSorted(got) || !slices.Contains(got, p.B.Local()) {
		t.Errorf("A.Peers() = %v, want sorted and containing B", got)
	}
	p.Cut(t)
	expectEvent(t, p.A, transport.PeerDown, p.B.Local())
	if got := p.A.Peers(); len(got) != 0 {
		t.Errorf("A.Peers() after Cut = %v, want none", got)
	}
}
