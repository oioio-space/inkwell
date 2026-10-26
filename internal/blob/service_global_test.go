package blob_test

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/transport"
)

// answers returns the replies waiting on c.
func answers(t *testing.T, c transport.Transport) []blob.Message {
	t.Helper()
	var out []blob.Message
	for {
		ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		ev, err := c.Recv(ctx)
		cancel()
		if err != nil {
			return out
		}
		if ev.Kind != transport.Message {
			continue
		}
		m, err := blob.Decode(ev.Payload)
		if err != nil {
			t.Fatalf("a reply that does not decode: %v", err)
		}
		out = append(out, m)
	}
}

// TestServerLimitsConcurrentRequests: a server serves MaxServing requests at
// once, whoever asks, answers Busy to the others, and serves them as soon as
// a slot is free. The limit of one peer is counted apart.
func TestServerLimitsConcurrentRequests(t *testing.T) {
	want := func(h blob.Hash) []byte {
		b, _ := blob.Encode(blob.Want{Hash: h, Length: blob.GroupSize})
		return b
	}
	for _, tc := range []struct {
		name             string
		maxServing       int
		clients          int // each sends perClient requests
		perClient        int
		wantServing      int
		wantBusyPerFirst int // Busy replies for the whole run
	}{
		{"global limit across peers", 2, 5, 1, 2, 3},
		{"per-peer limit of one peer", 8, 1, 5, 2, 3},
		{"per-peer limits do not add up to more than the global one", 3, 4, 2, 3, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				n := newNet(t)
				a := n.join("a", blob.Options{}, blob.ServiceOptions{MaxServing: tc.maxServing})
				h := a.put(t, testData(blob.GroupSize, 130))
				release := make(chan struct{})
				a.mu.Lock()
				a.gate = func(blob.Hash) { <-release }
				a.mu.Unlock()
				var clients []transport.Transport
				for i := range tc.clients {
					c := n.net.JoinBulk(transport.PeerID(fmt.Sprintf("c%d", i)))
					n.link("a", transport.PeerID(fmt.Sprintf("c%d", i)))
					clients = append(clients, c)
				}
				for _, c := range clients {
					for range tc.perClient {
						if err := c.Send(t.Context(), "a", want(h)); err != nil {
							t.Fatal(err)
						}
					}
				}
				synctest.Wait()
				busy := 0
				for _, c := range clients {
					for _, m := range answers(t, c) {
						if _, ok := m.(blob.Busy); !ok {
							t.Fatalf("a %T arrived while every slot was held", m)
						}
						busy++
					}
				}
				if busy != tc.wantBusyPerFirst {
					t.Errorf("%d Busy replies while %d requests were being served, want %d", busy, tc.wantServing, tc.wantBusyPerFirst)
				}
				close(release)
				synctest.Wait()
				served := 0
				for _, c := range clients {
					for _, m := range answers(t, c) {
						if _, ok := m.(blob.Slice); ok {
							served++
						}
					}
				}
				if served != tc.wantServing {
					t.Errorf("%d slices once the slots were free, want %d", served, tc.wantServing)
				}
			})
		})
	}
}
