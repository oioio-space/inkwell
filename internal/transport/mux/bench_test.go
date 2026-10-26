package mux_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// sink is a transport whose Send discards the payload and whose Recv blocks
// until it is closed: it isolates the cost of a sub-transport's Send.
type sink struct{ closed chan struct{} }

func (s *sink) Local() transport.PeerID                              { return "sink" }
func (s *sink) Peers() []transport.PeerID                            { return nil }
func (s *sink) Send(context.Context, transport.PeerID, []byte) error { return nil }
func (s *sink) Close() error                                         { close(s.closed); return nil }

func (s *sink) Recv(ctx context.Context) (transport.Event, error) {
	select {
	case <-s.closed:
		return transport.Event{}, transport.ErrClosed
	case <-ctx.Done():
		return transport.Event{}, ctx.Err()
	}
}

func BenchmarkSend(b *testing.B) {
	for _, size := range []int{256, 16 << 10, 1 << 20} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			sub := mux.Split(&sink{closed: make(chan struct{})}, chRepo)[chRepo]
			b.Cleanup(func() { _ = sub.Close() })
			payload := make([]byte, size)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				if err := sub.Send(b.Context(), "peer", payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
