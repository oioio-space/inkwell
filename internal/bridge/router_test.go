package bridge_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// fakeConn is an in-memory [bridge.Conn]: the test plays the webview by
// calling push and reading sent.
type fakeConn struct {
	ctx    context.Context
	in     chan []byte
	sent   chan []byte
	cancel context.CancelFunc
	once   sync.Once
}

var _ bridge.Conn = (*fakeConn)(nil)

func newFakeConn(t *testing.T) *fakeConn {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	c := &fakeConn{ctx: ctx, cancel: cancel, in: make(chan []byte, 16), sent: make(chan []byte, 16)}
	t.Cleanup(c.close)
	return c
}

func (c *fakeConn) Send(b []byte) error {
	select {
	case c.sent <- b:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *fakeConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.ctx.Done():
		return nil, io.EOF
	}
}

func (c *fakeConn) Context() context.Context { return c.ctx }

func (c *fakeConn) push(b []byte) { c.in <- b }

func (c *fakeConn) close() { c.once.Do(c.cancel) }

// next returns the next envelope sent to the webview.
func (c *fakeConn) next(t *testing.T) bridge.Envelope {
	t.Helper()
	env, err := bridge.Decode(<-c.sent)
	if err != nil {
		t.Fatalf("Decode(sent frame): %v", err)
	}
	return env
}

func mustEncode(t *testing.T, e bridge.Envelope) []byte {
	t.Helper()
	b, err := bridge.Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// wait runs fn in a goroutine and returns a channel closed when it ends.
func wait(fn func()) <-chan struct{} {
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	return done
}

func TestServeAnnouncesExistingPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("b")
		n.Join("c")
		n.Link("a", "b")
		n.Link("a", "c")

		r := bridge.NewRouter(a, quietLogger())
		c := newFakeConn(t)
		done := wait(func() { r.Serve(c) })

		for _, peer := range []transport.PeerID{"b", "c"} {
			if got := c.next(t); got.Kind != bridge.PeerUp || got.Peer != peer {
				t.Errorf("announce = %+v, want PeerUp %s", got, peer)
			}
		}
		c.close()
		<-done
	})
}

func TestRouterForwardsBothWays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		r := bridge.NewRouter(a, quietLogger())
		c := newFakeConn(t)
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })
		serveDone := wait(func() { r.Serve(c) })
		synctest.Wait()

		n.Link("a", "b")
		if got := c.next(t); got.Kind != bridge.PeerUp || got.Peer != "b" {
			t.Fatalf("got %+v, want PeerUp b", got)
		}

		// webview -> peer
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: []byte("to b")}))
		ev, err := b.Recv(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind == transport.PeerUp { // b's own PeerUp for a
			if ev, err = b.Recv(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		if ev.Kind != transport.Message || ev.Peer != "a" || !bytes.Equal(ev.Payload, []byte("to b")) {
			t.Errorf("b received %+v", ev)
		}

		// peer -> webview
		if err := b.Send(t.Context(), "a", []byte("to js")); err != nil {
			t.Fatal(err)
		}
		got := c.next(t)
		if got.Kind != bridge.Message || got.Peer != "b" || !bytes.Equal(got.Payload, []byte("to js")) {
			t.Errorf("webview received %+v", got)
		}

		n.Partition("a", "b")
		if got := c.next(t); got.Kind != bridge.PeerDown || got.Peer != "b" {
			t.Errorf("got %+v, want PeerDown b", got)
		}

		cancel()
		<-runDone
		c.close()
		<-serveDone
	})
}

func TestRunDropsWithoutConn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("b")
		r := bridge.NewRouter(a, quietLogger())
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })

		n.Link("a", "b") // PeerUp with nobody attached: dropped
		synctest.Wait()

		cancel()
		<-runDone
	})
}

func TestRunReturnsOnTransportClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		r := bridge.NewRouter(a, quietLogger())
		var err error
		runDone := wait(func() { err = r.Run(t.Context()) })
		synctest.Wait()
		_ = a.Close()
		<-runDone
		if !errors.Is(err, transport.ErrClosed) {
			t.Errorf("Run() = %v, want ErrClosed", err)
		}
	})
}

func TestMalformedFrameIsDroppedAndConnStaysOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")
		r := bridge.NewRouter(a, quietLogger())
		c := newFakeConn(t)
		done := wait(func() { r.Serve(c) })
		if got := c.next(t); got.Kind != bridge.PeerUp {
			t.Fatalf("got %+v, want PeerUp", got)
		}
		if _, err := b.Recv(t.Context()); err != nil { // b's PeerUp
			t.Fatal(err)
		}

		c.push([]byte{1, 3})                                                                   // truncated
		c.push([]byte{2, 3, 1, 'b'})                                                           // unknown version
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.PeerUp, Peer: "b"}))                 // wrong direction
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "zz", Payload: nil})) // unknown peer
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: []byte("ok")}))

		ev, err := b.Recv(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ev.Kind != transport.Message || !bytes.Equal(ev.Payload, []byte("ok")) {
			t.Errorf("b received %+v, want the valid message only", ev)
		}
		select {
		case <-done:
			t.Error("Serve returned after malformed frames")
		default:
		}
		c.close()
		<-done
	})
}

func TestReattachReplacesConnAndReannounces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		n.Link("a", "b")
		r := bridge.NewRouter(a, quietLogger())
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })

		old := newFakeConn(t)
		oldDone := wait(func() { r.Serve(old) })
		if got := old.next(t); got.Kind != bridge.PeerUp {
			t.Fatalf("old: got %+v, want PeerUp", got)
		}
		synctest.Wait() // Run has drained a's own PeerUp (sent to old)
		for len(old.sent) > 0 {
			<-old.sent
		}

		fresh := newFakeConn(t)
		freshDone := wait(func() { r.Serve(fresh) })
		if got := fresh.next(t); got.Kind != bridge.PeerUp || got.Peer != "b" {
			t.Fatalf("fresh: got %+v, want PeerUp b", got)
		}

		if err := b.Send(t.Context(), "a", []byte("after reload")); err != nil {
			t.Fatal(err)
		}
		var msg bridge.Envelope
		for msg.Kind != bridge.Message {
			msg = fresh.next(t)
		}
		if !bytes.Equal(msg.Payload, []byte("after reload")) {
			t.Errorf("fresh received %+v", msg)
		}
		synctest.Wait()
		if len(old.sent) != 0 {
			t.Error("old conn still receives traffic after being replaced")
		}

		// The old stream closing must not detach the new conn.
		old.close()
		<-oldDone
		if err := b.Send(t.Context(), "a", []byte("still attached")); err != nil {
			t.Fatal(err)
		}
		if got := fresh.next(t); !bytes.Equal(got.Payload, []byte("still attached")) {
			t.Errorf("fresh received %+v after old closed", got)
		}

		cancel()
		<-runDone
		fresh.close()
		<-freshDone
	})
}

func TestServeDetachesOnExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a, b := n.Join("a"), n.Join("b")
		r := bridge.NewRouter(a, quietLogger())
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })
		c := newFakeConn(t)
		serveDone := wait(func() { r.Serve(c) })
		synctest.Wait()
		c.close()
		<-serveDone

		n.Link("a", "b")
		if err := b.Send(t.Context(), "a", []byte("x")); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if len(c.sent) != 0 {
			t.Errorf("detached conn received %d frames", len(c.sent))
		}
		cancel()
		<-runDone
	})
}

// peersHook wraps a Transport and runs after on every Peers call, once the
// snapshot is taken.
type peersHook struct {
	transport.Transport
	after func()
}

func (p peersHook) Peers() []transport.PeerID {
	ps := p.Transport.Peers()
	p.after()
	return ps
}

// A PeerDown that Run handles between Serve's snapshot and its PeerUp must
// not overtake that PeerUp. Real time: a goroutine blocked on a mutex is not
// durably blocked, so synctest cannot be used.
func TestServeAnnouncesBeforeRunForwards(t *testing.T) {
	n := memnet.New(1, memnet.Faults{})
	a := n.Join("a")
	n.Join("b")
	n.Link("a", "b")
	if _, err := a.Recv(t.Context()); err != nil { // drop a's own PeerUp so only the Partition event is queued
		t.Fatal(err)
	}

	hook := peersHook{Transport: a, after: func() {
		n.Partition("a", "b")             // PeerDown queued after the snapshot holds b
		time.Sleep(50 * time.Millisecond) // let Run reach the session
	}}
	r := bridge.NewRouter(hook, quietLogger())
	ctx, cancel := context.WithCancel(t.Context())
	runDone := wait(func() { _ = r.Run(ctx) })
	c := newFakeConn(t)
	serveDone := wait(func() { r.Serve(c) })

	if got := c.next(t); got.Kind != bridge.PeerUp {
		t.Errorf("first frame = %+v, want PeerUp", got)
	}
	if got := c.next(t); got.Kind != bridge.PeerDown {
		t.Errorf("second frame = %+v, want PeerDown", got)
	}
	cancel()
	<-runDone
	c.close()
	<-serveDone
}

func TestMalformedFrameLogsAreRateLimited(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var buf syncBuffer
		log := slog.New(slog.NewTextHandler(&buf, nil))
		n := memnet.New(1, memnet.Faults{})
		r := bridge.NewRouter(n.Join("a"), log)
		c := newFakeConn(t)
		done := wait(func() { r.Serve(c) })
		for range 5 {
			c.push([]byte{1, 3})
		}
		synctest.Wait()
		if got := bytes.Count(buf.bytes(), []byte("dropped frame")); got != 1 {
			t.Errorf("logged %d times, want 1:\n%s", got, buf.bytes())
		}
		time.Sleep(time.Second)
		c.push([]byte{1, 3})
		synctest.Wait()
		if !bytes.Contains(buf.bytes(), []byte("suppressed=4")) {
			t.Errorf("missing suppressed=4:\n%s", buf.bytes())
		}
		c.close()
		<-done
	})
}

type syncBuffer struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.b.Bytes())
}

// enforcer is a memnet node that, like the app's wiring, signs attestations
// and reports envelopes ahead of messages. It records, in order, what the
// router asked of it.
type enforcer struct {
	*memnet.Node
	mu      sync.Mutex
	log     []string
	refuse  error
	notices []bridge.Envelope
	feed    chan bridge.Envelope
}

var (
	_ bridge.Attester = (*enforcer)(nil)
	_ bridge.Source   = (*enforcer)(nil)
)

func (e *enforcer) Attest(_ context.Context, doc string, heads [][bridge.HashSize]byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.log = append(e.log, "attest "+doc+" "+string(rune('0'+len(heads))))
	return e.refuse
}

func (e *enforcer) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	e.mu.Lock()
	e.log = append(e.log, "send "+string(p))
	e.mu.Unlock()
	return e.Node.Send(ctx, to, p)
}

func (e *enforcer) Notices() []bridge.Envelope { return e.notices }

func (e *enforcer) RecvEnvelope(ctx context.Context) (bridge.Envelope, error) {
	select {
	case env := <-e.feed:
		return env, nil
	case <-ctx.Done():
		return bridge.Envelope{}, ctx.Err()
	}
}

func (e *enforcer) calls() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.log...)
}

func heads1(doc string) []byte {
	b, err := bridge.EncodeHeads(bridge.Heads{Doc: doc, Heads: [][bridge.HashSize]byte{{1}}})
	if err != nil {
		panic(err)
	}
	return b
}

func TestAttestRunsBeforeTheNextFrame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		enf := &enforcer{Node: n.Join("a"), feed: make(chan bridge.Envelope, 4)}
		n.Join("b")
		n.Link("a", "b")
		r := bridge.NewRouter(enf, quietLogger())
		c := newFakeConn(t)
		done := wait(func() { r.Serve(c) })
		c.next(t) // PeerUp b

		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Attest, Peer: "a", Payload: heads1("d1")}))
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: []byte("sync")}))
		synctest.Wait()

		want := []string{"attest d1 1", "send sync"}
		if got := enf.calls(); !slices.Equal(got, want) {
			t.Errorf("calls = %q, want %q", got, want)
		}
		c.close()
		<-done
	})
}

func TestRefusedAttestKeepsServing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		enf := &enforcer{Node: n.Join("a"), refuse: errors.New("reader"), feed: make(chan bridge.Envelope)}
		n.Join("b")
		n.Link("a", "b")
		r := bridge.NewRouter(enf, quietLogger())
		c := newFakeConn(t)
		done := wait(func() { r.Serve(c) })
		c.next(t)

		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Attest, Peer: "a", Payload: heads1("d1")}))
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: []byte("ephemeral")}))
		synctest.Wait()

		if got := enf.calls(); !slices.Equal(got, []string{"attest d1 1", "send ephemeral"}) {
			t.Errorf("calls = %q, want the refused attest then the message", got)
		}
		c.close()
		<-done
	})
}

func TestAttestWithoutEnforcementAndWrongDirectionAreDropped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("b")
		n.Link("a", "b")
		r := bridge.NewRouter(a, quietLogger())
		c := newFakeConn(t)
		done := wait(func() { r.Serve(c) })
		c.next(t)

		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Attest, Peer: "a", Payload: heads1("d1")}))
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Attested, Peer: "b", Payload: heads1("d1")}))
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Revoked, Peer: "b"}))
		c.push(mustEncode(t, bridge.Envelope{Kind: bridge.Sealed, Peer: "b"}))
		synctest.Wait()
		c.close()
		<-done
	})
}

func TestNoticesAndEnvelopesReachTheWebview(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		enf := &enforcer{Node: n.Join("a"), feed: make(chan bridge.Envelope, 4)}
		n.Join("b")
		n.Link("a", "b")
		past := bridge.Envelope{Kind: bridge.Attested, Peer: "b", Payload: heads1("d1")}
		enf.notices = []bridge.Envelope{past, {Kind: bridge.Sealed, Peer: "a"}}

		r := bridge.NewRouter(enf, quietLogger())
		c := newFakeConn(t)
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })
		serveDone := wait(func() { r.Serve(c) })

		// On attach: the peers, then the state.
		for _, want := range []bridge.Kind{bridge.PeerUp, bridge.Attested, bridge.Sealed} {
			if got := c.next(t); got.Kind != want {
				t.Fatalf("on attach got %v, want %v", got.Kind, want)
			}
		}
		// Live envelopes keep their order.
		enf.feed <- bridge.Envelope{Kind: bridge.Attested, Peer: "b", Payload: heads1("d2")}
		enf.feed <- bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: []byte("m")}
		enf.feed <- bridge.Envelope{Kind: bridge.Revoked, Peer: "b"}
		for _, want := range []bridge.Kind{bridge.Attested, bridge.Message, bridge.Revoked} {
			if got := c.next(t); got.Kind != want {
				t.Fatalf("live got %v, want %v", got.Kind, want)
			}
		}
		cancel()
		<-runDone
		c.close()
		<-serveDone
	})
}
