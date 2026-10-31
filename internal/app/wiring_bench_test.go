package app

import (
	"context"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// relayInput feeds real mux queues without network or trust setup in the measurement.
type relayInput struct{ events chan transport.Event }

func (r *relayInput) Local() transport.PeerID                              { return "local" }
func (r *relayInput) Peers() []transport.PeerID                            { return nil }
func (r *relayInput) Send(context.Context, transport.PeerID, []byte) error { return nil }
func (r *relayInput) Close() error                                         { return nil }
func (r *relayInput) Recv(ctx context.Context) (transport.Event, error) {
	select {
	case ev := <-r.events:
		return ev, nil
	case <-ctx.Done():
		return transport.Event{}, ctx.Err()
	}
}

func BenchmarkRelay(b *testing.B) {
	for _, queued := range []bool{false, true} {
		name := "Idle"
		if queued {
			name = "Message"
		}
		b.Run(name, func(b *testing.B) {
			input := &relayInput{events: make(chan transport.Event)}
			subs := mux.Split(input, chanRepo, chanTrust, chanAttest)
			b.Cleanup(func() { _ = subs[chanRepo].Close() })
			ctx, cancel := context.WithCancel(b.Context())
			defer cancel()
			pollCtx, cancelPoll := context.WithCancel(context.Background())
			cancelPoll()
			w := &Wiring{
				pollCtx: pollCtx, ctx: ctx, cancel: cancel,
				repoCh: subs[chanRepo], trustCh: subs[chanTrust], attestCh: subs[chanAttest],
				out:  make(chan bridge.Envelope, 1),
				full: map[transport.PeerID]struct{}{"peer": {}},
			}
			w.publishFull()
			wake := mux.Wake(w.repoCh)
			ev := transport.Event{Kind: transport.Message, Peer: "peer", Payload: []byte{chanRepo, 'x'}}
			b.ReportAllocs()
			for b.Loop() {
				if queued {
					input.events <- ev
					<-wake
				}
				if got := w.step(); got != queued {
					b.Fatalf("step = %v, want %v", got, queued)
				}
				if queued {
					out, err := (*repoPort)(w).RecvEnvelope(ctx)
					if err != nil {
						b.Fatal(err)
					}
					if out.Kind != bridge.Message || string(out.Payload) != "x" {
						b.Fatal("incorrect relay output")
					}
				}
			}
		})
	}
}
