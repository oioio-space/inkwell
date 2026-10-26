package blob_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/transport"
)

// TestFetchRequestsAreBounded: a peer that only ever answers Busy is asked a
// bounded number of times: five waits that do not count, then eight attempts.
func TestFetchRequestsAreBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		store, _ := openStore(t, "", blob.Options{})
		data := testData(blob.GroupSize*3, 120)
		h, _ := store.PutBytes(data)
		sp := newScripted(t, n, "s", store)
		sp.handle = func(_ transport.PeerID, w blob.Want, _ func() blob.Message) []blob.Message {
			return []blob.Message{blob.Busy{Hash: w.Hash, Retry: time.Millisecond}}
		}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("s", "b")
		err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))})
		if !errors.Is(err, blob.ErrNoPeer) {
			t.Fatalf("Fetch from a peer that is always busy = %v, want ErrNoPeer", err)
		}
		const maxBusy, maxAttempts = 5, 8
		if got := len(sp.requested()); got != maxBusy+maxAttempts {
			t.Errorf("the peer was asked %d times, want %d (%d waited-out Busy answers, then %d attempts)", got, maxBusy+maxAttempts, maxBusy, maxAttempts)
		}
		if b.log.security() != 0 {
			t.Error("Busy answers were counted as faults")
		}
	})
}

// TestBusyWaitIsCapped: a server asking to wait a minute is waited for 5 s.
func TestBusyWaitIsCapped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := newNet(t)
		store, _ := openStore(t, "", blob.Options{})
		data := testData(blob.GroupSize, 121)
		h, _ := store.PutBytes(data)
		sp := newScripted(t, n, "s", store)
		var calls atomic.Int32
		sp.handle = func(_ transport.PeerID, w blob.Want, def func() blob.Message) []blob.Message {
			if calls.Add(1) == 1 {
				return []blob.Message{blob.Busy{Hash: w.Hash, Retry: time.Minute}}
			}
			return []blob.Message{def()}
		}
		b := n.join("b", blob.Options{}, blob.ServiceOptions{})
		n.link("s", "b")
		start := time.Now()
		if err := b.svc.Fetch(t.Context(), blob.Ref{Hash: h, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if took := time.Since(start); took != 5*time.Second {
			t.Errorf("Fetch waited %v for a Busy of one minute, want the 5 s cap", took)
		}
	})
}

// TestServingSlotsAreReleased: the slots a server counts requests in are
// given back, so a client that asks one request after the other is never told
// Busy, however many it asks, even with a single slot.
func TestServingSlotsAreReleased(t *testing.T) {
	for name, opt := range map[string]blob.ServiceOptions{
		"one slot":     {MaxServing: 1},
		"default slot": {},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := newNet(t)
				a := n.join("a", blob.Options{}, opt)
				data := testData(blob.GroupSize*4, 122)
				h := a.put(t, data)
				client := n.net.JoinBulk("client")
				n.link("a", "client")
				for i := range 12 {
					b, _ := blob.Encode(blob.Want{Hash: h, Offset: 0, Length: blob.MaxSlice})
					if err := client.Send(t.Context(), "a", b); err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
					var reply blob.Message
					for {
						ev, err := client.Recv(ctx)
						if err != nil {
							cancel()
							t.Fatalf("request %d: no answer: %v", i, err)
						}
						if ev.Kind == transport.Message {
							reply, err = blob.Decode(ev.Payload)
							if err != nil {
								cancel()
								t.Fatal(err)
							}
							break
						}
					}
					cancel()
					if _, ok := reply.(blob.Slice); !ok {
						t.Fatalf("request %d answered with %T, want a Slice: a slot was not released", i, reply)
					}
				}
			})
		})
	}
}
