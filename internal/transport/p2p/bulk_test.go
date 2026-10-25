package p2p_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

const (
	// bulkFrame is the size of the bulk messages of the latency tests, the
	// size of a blob slice.
	bulkFrame = 1 << 20
	// bulkTotal is the transfer that runs while the latency is measured: the
	// 100 MB of PRJ-19.
	bulkTotal = 100 << 20
)

// stamp returns a ping payload carrying the time it was sent.
func stamp(now time.Time) []byte {
	b, _ := now.MarshalBinary()
	return b
}

// latency returns the time elapsed since the ping payload was stamped.
func latency(t *testing.T, payload []byte) time.Duration {
	t.Helper()
	var sent time.Time
	if err := sent.UnmarshalBinary(payload); err != nil {
		t.Fatalf("ping payload %x: %v", payload, err)
	}
	return time.Since(sent)
}

// flood sends total bytes to peer in frames of bulkFrame, until done or ctx ends.
func flood(ctx context.Context, from transport.Transport, to transport.PeerID, total int) error {
	frame := bytes.Repeat([]byte{0xAB}, bulkFrame)
	for sent := 0; sent < total; sent += bulkFrame {
		if err := from.Send(ctx, to, frame); err != nil {
			return err
		}
	}
	return nil
}

// drain reads the lane until ctx ends, counting the bytes of the Messages.
func drain(ctx context.Context, lane transport.Transport, got *int, mu *sync.Mutex) {
	for {
		ev, err := lane.Recv(ctx)
		if err != nil {
			return
		}
		if ev.Kind == transport.Message {
			mu.Lock()
			*got += len(ev.Payload)
			mu.Unlock()
		}
	}
}

// percentile returns the p-th percentile (0 to 1) of the sorted durations.
func percentile(sorted []time.Duration, p float64) time.Duration {
	return sorted[min(len(sorted)-1, int(float64(len(sorted))*p))]
}

// latencies runs the PRJ-19 scenario in the current synctest bubble and
// returns the one-way latency of pings sent every 50 ms on the sync lane,
// before and during a transfer of bulkTotal bytes on the bulk lane, and how
// long the transfer took. The network is simulated with a 100 Mbit/s link and a
// virtual clock, so the 100 MB take 8 s of virtual time and the numbers are
// exact.
func latencies(t *testing.T) (idle, loaded []time.Duration, took time.Duration) {
	t.Helper()
	s := newSimNet(t)
	a := s.node(t, 1, "1.0.0.1", p2p.Options{})
	b := s.node(t, 2, "1.0.0.2", p2p.Options{})
	bulkA, bulkB := a.Bulk(), b.Bulk()
	s.connect(t, a, b)
	expectUp(t, a, b)
	expectEvent(t, bulkA, transport.PeerUp, b.Local())
	expectEvent(t, bulkB, transport.PeerUp, a.Local())

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	var (
		mu       sync.Mutex
		lats     []time.Duration
		received int
	)
	wg.Go(func() {
		for {
			ev, err := b.Recv(ctx)
			if err != nil {
				return
			}
			if ev.Kind == transport.Message {
				mu.Lock()
				lats = append(lats, latency(t, ev.Payload))
				mu.Unlock()
			}
		}
	})
	wg.Go(func() { drain(ctx, bulkB, &received, &mu) })
	wg.Go(func() {
		for ctx.Err() == nil {
			if err := a.Send(ctx, b.Local(), stamp(time.Now())); err != nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	time.Sleep(time.Second)
	mu.Lock()
	idle, lats = slices.Clone(lats), nil
	mu.Unlock()

	start := time.Now()
	if err := flood(ctx, bulkA, b.Local(), bulkTotal); err != nil {
		t.Fatalf("flood: %v", err)
	}
	for {
		mu.Lock()
		done := received >= bulkTotal
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	took = time.Since(start)
	mu.Lock()
	loaded = slices.Clone(lats)
	mu.Unlock()
	slices.Sort(idle)
	slices.Sort(loaded)
	return idle, loaded, took
}

// PRJ-19: a message of the sync lane (the repo stream) is not held back by
// 100 MB moving on the bulk lane.
func TestRepoLatencyDuringBulk(t *testing.T) {
	t.Run("PRJ-19 latency of the repo stream during 100 MB of bulk", runRepoLatencyDuringBulk)
}

func runRepoLatencyDuringBulk(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		idle, loaded, took := latencies(t)
		if len(idle) < 10 || len(loaded) < 20 {
			t.Fatalf("pings: %d idle, %d during the transfer; want at least 10 and 20", len(idle), len(loaded))
		}
		t.Logf("100 MB in %v; sync latency idle p50 %v max %v; during bulk p50 %v p99 %v max %v",
			took, percentile(idle, 0.5), idle[len(idle)-1],
			percentile(loaded, 0.5), percentile(loaded, 0.99), loaded[len(loaded)-1])
		if took < 5*time.Second {
			t.Fatalf("the transfer took %v: the link is too fast to prove anything", took)
		}
		if worst := loaded[len(loaded)-1]; worst > 100*time.Millisecond {
			t.Errorf("sync latency during the bulk transfer: max %v, want at most 100ms (idle max %v)", worst, idle[len(idle)-1])
		}
	})
}

// The bulk lane works over the real loopback interface, on both transports,
// in both directions, and keeps its frames apart from the sync lane's.
func TestBulkLoopback(t *testing.T) {
	for _, tc := range []struct{ name, sub string }{{"QUIC", "/quic-v1"}, {"TCP and Noise", "/tcp/"}} {
		t.Run(tc.name, func(t *testing.T) {
			a := loopbackNode(t, 1, p2p.Options{})
			b := loopbackNode(t, 2, p2p.Options{})
			bulkA, bulkB := a.Bulk(), b.Bulk()
			if a.Bulk() != bulkA {
				t.Error("Bulk() returned a second lane")
			}
			if err := b.Connect(t.Context(), a.Local(), loopbackAddrs(a, tc.sub)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			expectUp(t, a, b)
			expectUp(t, bulkA, bulkB)

			big := bytes.Repeat([]byte("0123456789abcdef"), 3<<16) // 3 MiB
			if err := bulkA.Send(t.Context(), b.Local(), big); err != nil {
				t.Fatalf("bulk Send: %v", err)
			}
			if err := a.Send(t.Context(), b.Local(), []byte("sync")); err != nil {
				t.Fatalf("sync Send: %v", err)
			}
			if got := recvWithin(t, bulkB, wait); got.Kind != transport.Message || got.Peer != a.Local() || !bytes.Equal(got.Payload, big) {
				t.Errorf("bulk lane got %v from %s with %d bytes, want the 3 MiB frame from a", got.Kind, got.Peer, len(got.Payload))
			}
			if got := recvWithin(t, b, wait); string(got.Payload) != "sync" {
				t.Errorf("sync lane got %q, want sync", got.Payload)
			}
			if err := bulkB.Send(t.Context(), a.Local(), []byte("back")); err != nil {
				t.Fatalf("bulk Send back: %v", err)
			}
			if got := recvWithin(t, bulkA, wait); string(got.Payload) != "back" {
				t.Errorf("bulk lane of a got %q, want back", got.Payload)
			}
			expectQuietFor(t, bulkA, quiet)
			expectQuietFor(t, bulkB, quiet)
		})
	}
}

// A lane created after the peers connected is told about them.
func TestBulkLateLaneSeesPeers(t *testing.T) {
	a := loopbackNode(t, 1, p2p.Options{})
	b := loopbackNode(t, 2, p2p.Options{})
	if err := b.Connect(t.Context(), a.Local(), loopbackAddrs(a, "/quic-v1")); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	expectUp(t, a, b)
	bulkA, bulkB := a.Bulk(), b.Bulk()
	expectEvent(t, bulkA, transport.PeerUp, b.Local())
	expectEvent(t, bulkB, transport.PeerUp, a.Local())
	if err := bulkA.Send(t.Context(), b.Local(), []byte("late")); err != nil {
		t.Fatalf("bulk Send: %v", err)
	}
	if got := recvWithin(t, bulkB, wait); string(got.Payload) != "late" {
		t.Errorf("got %q, want late", got.Payload)
	}
}

// Closing the lane closes the node, and a closed lane refuses everything.
func TestBulkCloseClosesNode(t *testing.T) {
	a := loopbackNode(t, 1, p2p.Options{})
	bulk := a.Bulk()
	if err := bulk.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := bulk.Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("Recv after Close = %v, want ErrClosed", err)
	}
	if err := bulk.Send(t.Context(), "x", nil); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("Send after Close = %v, want ErrClosed", err)
	}
	if _, err := a.Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
		t.Errorf("node Recv after the lane's Close = %v, want ErrClosed", err)
	}
}

// BenchmarkRepoLatencyDuringBulk measures, over the real loopback interface,
// the latency of a sync-lane message while 100 MB move on the bulk lane
// (PRJ-19). Loopback is fast, so the figures are a floor; the simulated link
// of TestRepoLatencyDuringBulk is the one that shows queueing.
func BenchmarkRepoLatencyDuringBulk(b *testing.B) {
	for _, withBulk := range []bool{false, true} {
		name := "idle"
		if withBulk {
			name = "bulk100MB"
		}
		b.Run(name, func(b *testing.B) {
			ctx := b.Context()
			a, err := p2p.New(ctx, key(1), p2p.Options{})
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = a.Close() }()
			c, err := p2p.New(ctx, key(2), p2p.Options{})
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = c.Close() }()
			bulkA, bulkC := a.Bulk(), c.Bulk()
			if err := c.Connect(ctx, a.Local(), loopbackAddrs(a, "/quic-v1")); err != nil {
				b.Fatal(err)
			}
			for _, n := range []transport.Transport{a, c, bulkA, bulkC} {
				if ev, err := n.Recv(ctx); err != nil || ev.Kind != transport.PeerUp {
					b.Fatalf("Recv = %v, %v; want a PeerUp", ev, err)
				}
			}
			var wg sync.WaitGroup
			floodCtx, stop := context.WithCancel(ctx)
			defer func() {
				stop()
				wg.Wait()
			}()
			if withBulk {
				var mu sync.Mutex
				var got int
				wg.Go(func() { drain(floodCtx, bulkC, &got, &mu) })
				wg.Go(func() {
					for floodCtx.Err() == nil {
						if flood(floodCtx, bulkA, c.Local(), bulkTotal) != nil {
							return
						}
					}
				})
			}
			var lats []time.Duration
			for b.Loop() {
				if err := a.Send(ctx, c.Local(), stamp(time.Now())); err != nil {
					b.Fatal(err)
				}
				ev, err := c.Recv(ctx)
				if err != nil {
					b.Fatal(err)
				}
				lats = append(lats, time.Since(mustStamp(b, ev.Payload)))
				time.Sleep(time.Millisecond)
			}
			b.StopTimer()
			slices.Sort(lats)
			b.ReportMetric(float64(percentile(lats, 0.5).Microseconds()), "p50-µs")
			b.ReportMetric(float64(percentile(lats, 0.99).Microseconds()), "p99-µs")
		})
	}
}

func mustStamp(b *testing.B, payload []byte) time.Time {
	b.Helper()
	var sent time.Time
	if err := sent.UnmarshalBinary(payload); err != nil {
		b.Fatalf("ping payload %x: %v", payload, err)
	}
	return sent
}
