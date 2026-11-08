package bridge_test

import (
	"context"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// benchConn is a [bridge.Conn] that hands the router one frame at a time and
// swallows what the router sends.
type benchConn struct {
	ctx context.Context
	in  chan []byte
}

func (c *benchConn) Send([]byte) error        { return nil }
func (c *benchConn) Context() context.Context { return c.ctx }
func (c *benchConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.ctx.Done():
		return nil, io.EOF
	}
}

// BenchmarkServeMessage measures the path of a Message from the webview to the
// peer: decode, enqueue, per-peer sender, transport. It is the hot path of the
// bridge (CLAUDE.md, HOT_PATHS).
func BenchmarkServeMessage(b *testing.B) {
	n := memnet.New(1, memnet.Faults{})
	a, peer := n.Join("a"), n.Join("b")
	n.Link("a", "b")
	ctx, cancel := context.WithCancel(b.Context())
	r := bridge.NewRouter(a, quietLogger())
	if s, ok := any(r).(interface{ SetLinksPeriod(time.Duration) }); ok {
		s.SetLinksPeriod(0) // measure the messages only
	}
	c := &benchConn{ctx: ctx, in: make(chan []byte)}
	var delivered atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			ev, err := peer.Recv(ctx)
			if err != nil {
				return
			}
			if ev.Kind == transport.Message {
				delivered.Add(1)
			}
		}
	}()
	go func() { _ = r.Run(ctx) }()
	served := make(chan struct{})
	go func() { r.Serve(c); close(served) }()

	frame, err := bridge.Encode(bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: make([]byte, 1024)})
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		for int64(i)-delivered.Load() >= 32 { // stay under the outbox depth
			runtime.Gosched()
		}
		c.in <- frame
	}
	for delivered.Load() < int64(b.N) {
		runtime.Gosched()
	}
	b.StopTimer()
	cancel()
	<-served
	<-done
}
