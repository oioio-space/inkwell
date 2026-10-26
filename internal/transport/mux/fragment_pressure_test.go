package mux_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// reserving is a transport with a byte budget, as the p2p node is.
type reserving struct {
	transport.Transport
	limit, used int
	mu          sync.Mutex
}

func (r *reserving) Reserve(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.used+n > r.limit {
		return false
	}
	r.used += n
	return true
}

func (r *reserving) Unreserve(n int) {
	r.mu.Lock()
	r.used -= n
	r.mu.Unlock()
}

func (r *reserving) inUse() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.used
}

var _ mux.Reserver = (*reserving)(nil)

// TestStalePartialExpires: an unfinished message that stops making progress
// gives its memory back.
func TestStalePartialExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		x := n.Join("x")
		res := &reserving{Transport: n.Join("b"), limit: 1 << 30}
		repo := mux.Split(res, chRepo)[chRepo]
		defer func() { _ = x.Close(); _ = repo.Close() }()
		n.Link("x", "b")
		if err := x.Send(t.Context(), "b", frag(chRepo, 1, 1<<20, 0, make([]byte, 32<<10))); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if res.inUse() == 0 {
			t.Fatal("a partial message holds no reserved memory")
		}
		time.Sleep(time.Minute)
		if got := res.inUse(); got != 0 {
			t.Errorf("%d bytes still reserved a minute after the last fragment", got)
		}
	})
}

// TestPressureEvictsTheStalest: parked messages of hostile peers fill the
// budget; an honest message still gets through, at their expense.
func TestPressureEvictsTheStalest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		peers := map[transport.PeerID]*memnet.Node{}
		for _, id := range []transport.PeerID{"x", "y", "z", "h"} {
			peers[id] = n.Join(id)
		}
		res := &reserving{Transport: n.Join("b"), limit: 100 << 10}
		repo := mux.Split(res, chRepo)[chRepo]
		defer func() {
			for _, p := range peers {
				_ = p.Close()
			}
			_ = repo.Close()
		}()
		for id := range peers {
			n.Link(id, "b")
		}
		for i, id := range []transport.PeerID{"x", "y", "z"} { // parked, x the stalest
			if err := peers[id].Send(t.Context(), "b", frag(chRepo, 1, 10<<20, 0, make([]byte, 32<<10))); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Second * time.Duration(i+1))
		}
		want := content(80<<10, 9)
		for _, f := range [][]byte{
			frag(chRepo, 2, 80<<10, 0, want[:32<<10]),
			frag(chRepo, 2, 80<<10, 32<<10, want[32<<10:64<<10]),
			frag(chRepo, 2, 80<<10, 64<<10, want[64<<10:]),
		} {
			if err := peers["h"].Send(t.Context(), "b", f); err != nil {
				t.Fatal(err)
			}
		}
		for {
			ev := recv(t, repo)
			if ev.Kind != transport.Message {
				continue
			}
			if ev.Peer != "h" || !bytes.Equal(ev.Payload, want) {
				t.Fatalf("got %d bytes from %q, want the honest message", len(ev.Payload), ev.Peer)
			}
			break
		}
		if got := res.inUse(); got > 100<<10 {
			t.Errorf("%d bytes reserved, over the %d budget", got, 100<<10)
		}
	})
}

// drip is a link that takes 19 s per frame: always inside an idle limit of
// 20 s, far below any useful rate.
type drip struct{ transport.Transport }

func (d drip) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	select {
	case <-time.After(19 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	return d.Transport.Send(ctx, to, p)
}

// TestDripFeedIsCutOff: a large message to a receiver that takes a fragment
// every 19 s fails after idle + len/minRate, not after hours.
func TestDripFeedIsCutOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, _ := linked(t, memnet.Faults{}, func(t transport.Transport) transport.Transport { return drip{t} })
		start := time.Now()
		err := a.Send(t.Context(), "b", make([]byte, 1<<20))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Send(1 MiB, 19 s per fragment) = %v, want DeadlineExceeded", err)
		}
		if d := time.Since(start); d > 2*time.Minute {
			t.Errorf("gave up after %v, want about 20 s + 1 MiB / 16 KiB/s = 84 s", d)
		}
	})
}
