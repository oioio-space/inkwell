package bridge_test

import (
	"bytes"
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

type presenceHandoff struct {
	transport.Transport
	entered chan struct{}
	release chan struct{}
}

func (p presenceHandoff) Send(ctx context.Context, to transport.PeerID, data []byte) error {
	close(p.entered)
	select {
	case <-p.release:
		return p.Transport.Send(ctx, to, data)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestWC1W3PresenceBarrier(t *testing.T) {
	for _, name := range []string{"HealthyHandoff", "AbortedHandoff"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := memnet.New(1, memnet.Faults{})
				a, b := n.Join("a"), n.Join("b")
				n.Link("a", "b")
				if _, err := b.Recv(t.Context()); err != nil {
					t.Fatal(err)
				}
				p := presenceHandoff{Transport: a, entered: make(chan struct{}), release: make(chan struct{})}
				r := newRouter(p, quietLogger())
				c := newFakeConn(t)
				done := wait(func() { r.Serve(c) })
				if up := c.next(t); up.Kind != bridge.PeerUp || up.Peer != "b" {
					t.Fatalf("expected a healthy link, got %+v", up)
				}
				goodbye := []byte("opaque goodbye")
				frame, err := bridge.Encode(bridge.Envelope{Kind: bridge.Message, Peer: "b", Payload: goodbye})
				if err != nil {
					t.Fatal(err)
				}
				c.push(frame)
				<-p.entered
				barrier := []byte{0, 0, 0, 0, 1}
				c.push(barrier)
				synctest.Wait()
				if len(c.sent) != 0 {
					t.Error("W-3: acknowledged before the goodbye was handed off")
				}
				if name == "AbortedHandoff" {
					c.close()
					<-done
					if len(c.sent) != 0 {
						t.Error("W-3: acknowledged an aborted handoff")
					}
					return
				}
				close(p.release)
				select {
				case ack := <-c.sent:
					if !bytes.Equal(ack, barrier) {
						t.Errorf("W-3: wrong barrier reply %v", ack)
					}
				case <-time.After(time.Second):
					t.Error("W-3: no barrier reply after healthy handoff")
				}
				if event, err := b.Recv(t.Context()); err != nil || event.Kind != transport.Message || !bytes.Equal(event.Payload, goodbye) {
					t.Errorf("W-3: the healthy peer did not receive the goodbye: %+v, %v", event, err)
				}
				c.close()
				<-done
			})
		})
	}
}
