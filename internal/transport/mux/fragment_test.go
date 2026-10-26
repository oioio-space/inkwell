package mux_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// fragChannel is the reserved wire byte of a fragment, see the package doc.
const fragChannel = 0xFF

// content returns n deterministic bytes that differ from one message to the
// next, so that a mixed-up reassembly is noticed.
func content(n int, salt byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7+i>>8) ^ salt
	}
	return b
}

// slow wraps a transport with a link of rate bytes per second: Send takes
// len/rate and sends are serialised, as the writes of a libp2p stream are.
type slow struct {
	transport.Transport
	lane chan struct{}
	rate int
}

func newSlow(t transport.Transport, rate int) *slow {
	return &slow{Transport: t, rate: rate, lane: make(chan struct{}, 1)}
}

func (s *slow) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	select {
	case s.lane <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.lane }()
	select {
	case <-time.After(time.Duration(len(p)) * time.Second / time.Duration(s.rate)):
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.Transport.Send(ctx, to, p)
}

// linked returns the repo channel of a, wrapped by wrap, and that of b.
func linked(t *testing.T, f memnet.Faults, wrap func(transport.Transport) transport.Transport) (a, b transport.Transport) {
	t.Helper()
	n := memnet.New(1, f)
	a = mux.Split(wrap(n.Join("a")), chRepo)[chRepo]
	b = mux.Split(n.Join("b"), chRepo)[chRepo]
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	n.Link("a", "b")
	for _, c := range []transport.Transport{a, b} {
		if ev, err := c.Recv(t.Context()); err != nil || ev.Kind != transport.PeerUp {
			t.Fatalf("first event = %v, %v; want PeerUp", ev, err)
		}
	}
	return a, b
}

func identity(t transport.Transport) transport.Transport { return t }

// recv returns the next Message of c.
func recv(t *testing.T, c transport.Transport) transport.Event {
	t.Helper()
	ev, err := c.Recv(t.Context())
	if err != nil {
		t.Fatalf("Recv = %v", err)
	}
	return ev
}

func TestFragmentRoundTrip(t *testing.T) {
	for _, size := range []int{64 << 10, 64<<10 + 1, 1<<20 + 17, 5 << 20} {
		t.Run("", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				a, b := linked(t, memnet.Faults{}, identity)
				want := content(size, 1)
				if err := a.Send(t.Context(), "b", want); err != nil {
					t.Fatalf("Send(%d bytes) = %v", size, err)
				}
				if ev := recv(t, b); ev.Kind != transport.Message || ev.Peer != "a" || !bytes.Equal(ev.Payload, want) {
					t.Fatalf("got kind %v from %q with %d bytes, want the %d sent", ev.Kind, ev.Peer, len(ev.Payload), size)
				}
			})
		})
	}
}

// TestSmallMessageStaysOneFrame: up to 64 KiB, the wire format is unchanged.
func TestSmallMessageStaysOneFrame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		raw := n.Join("a")
		sub := mux.Split(n.Join("b"), chRepo)[chRepo]
		defer func() { _ = raw.Close(); _ = sub.Close() }()
		n.Link("a", "b")
		if err := sub.Send(t.Context(), "a", content(64<<10-mux.Overhead, 2)); err != nil {
			t.Fatal(err)
		}
		for {
			ev := recv(t, raw)
			if ev.Kind == transport.Message {
				if ev.Payload[0] != chRepo || len(ev.Payload) != 64<<10 {
					t.Errorf("wire frame = %#x…, %d bytes; want one frame on the channel byte", ev.Payload[0], len(ev.Payload))
				}
				return
			}
		}
	})
}

func TestSendTooLarge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, _ := linked(t, memnet.Faults{}, identity)
		if err := a.Send(t.Context(), "b", make([]byte, transport.MaxFrame-mux.Overhead+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
			t.Errorf("Send(MaxFrame) = %v, want ErrFrameTooLarge", err)
		}
	})
}

// TestTypingDuringBigMessage is PERF-05: a 6 MB message on a 1 Mbit/s link
// (125 kB/s, 48 s) while the user types. Typing arrives within 500 ms and the
// big message arrives intact.
func TestTypingDuringBigMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const rate = 125_000
		a, b := linked(t, memnet.Faults{}, func(t transport.Transport) transport.Transport { return newSlow(t, rate) })
		big := content(6<<20, 3)
		bigDone := make(chan error, 1)
		go func() { bigDone <- a.Send(t.Context(), "b", big) }()
		time.Sleep(time.Second) // the big message is under way

		var worst time.Duration
		var gotBig []byte
		for i := range 20 {
			typed := []byte{'k', byte(i)}
			sent := time.Now()
			if err := a.Send(t.Context(), "b", typed); err != nil {
				t.Fatalf("typing Send = %v", err)
			}
			for {
				ev := recv(t, b)
				if len(ev.Payload) == len(big) {
					gotBig = ev.Payload
					continue
				}
				if !bytes.Equal(ev.Payload, typed) {
					t.Fatalf("typing %d arrived as %q", i, ev.Payload)
				}
				break
			}
			worst = max(worst, time.Since(sent))
			time.Sleep(300 * time.Millisecond)
		}
		if worst > 500*time.Millisecond {
			t.Errorf("worst typing latency = %v, want <= 500ms", worst)
		}
		if err := <-bigDone; err != nil {
			t.Fatalf("big Send = %v, want nil (no abort on a slow link)", err)
		}
		if gotBig == nil {
			gotBig = recv(t, b).Payload
		}
		if !bytes.Equal(gotBig, big) {
			t.Errorf("big message corrupted: %d bytes", len(gotBig))
		}
	})
}

// TestBackgroundNotStarved: a flood of interactive messages leaves the big one
// a share of the link, so it still finishes.
func TestBackgroundNotStarved(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const rate = 100_000
		a, b := linked(t, memnet.Faults{}, func(t transport.Transport) transport.Transport { return newSlow(t, rate) })
		big := content(300<<10, 4)
		done := make(chan error, 1)
		go func() { done <- a.Send(t.Context(), "b", big) }()
		go func() { // drain
			for {
				if _, err := b.Recv(t.Context()); err != nil {
					return
				}
			}
		}()
		start := time.Now()
		for {
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("big Send = %v", err)
				}
				// At 25%% of 100 kB/s the 300 KiB take 12 s; without a floor, never.
				if d := time.Since(start); d > 40*time.Second {
					t.Errorf("big message took %v under a flood, want <= 40s", d)
				}
				return
			default:
			}
			if err := a.Send(t.Context(), "b", content(8<<10, 5)); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// TestFragmentsSurviveFaults: with loss and duplication a message
// either arrives intact or not at all.
func TestFragmentsSurviveFaults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b := linked(t, memnet.Faults{Drop: 0.02, Duplicate: 0.05}, identity)
		go func() {
			for i := range 30 {
				_ = a.Send(t.Context(), "b", content(300<<10+i, byte(i)))
			}
		}()
		var got atomic.Int32
		go func() {
			for {
				ev, err := b.Recv(t.Context())
				if err != nil {
					return
				}
				if ev.Kind != transport.Message {
					continue
				}
				salt := byte(0)
				for s := range 30 {
					if len(ev.Payload) == 300<<10+s {
						salt = byte(s)
					}
				}
				if !bytes.Equal(ev.Payload, content(len(ev.Payload), salt)) {
					t.Errorf("corrupted message of %d bytes delivered", len(ev.Payload))
				}
				got.Add(1)
			}
		}()
		time.Sleep(time.Minute)
		if got.Load() == 0 {
			t.Error("no message survived 2% loss")
		}
	})
}

func TestCanceledSendThenNext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b := linked(t, memnet.Faults{}, func(t transport.Transport) transport.Transport { return newSlow(t, 200_000) })
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := a.Send(ctx, "b", content(2<<20, 6)); err == nil {
			t.Fatal("canceled big Send = nil, want an error")
		}
		want := content(500<<10, 7)
		if err := a.Send(t.Context(), "b", want); err != nil {
			t.Fatal(err)
		}
		if ev := recv(t, b); !bytes.Equal(ev.Payload, want) {
			t.Errorf("message after a canceled one: %d bytes, want %d intact", len(ev.Payload), len(want))
		}
	})
}

// frag builds a fragment frame as the wire carries it.
func frag(ch byte, id, total, off uint32, data []byte) []byte {
	h := []byte{fragChannel, ch, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(h[2:], id)
	binary.BigEndian.PutUint32(h[6:], total)
	binary.BigEndian.PutUint32(h[10:], off)
	return append(h, data...)
}

// TestHostileFragments: malformed or inconsistent fragments never deliver
// anything and never block the channel.
func TestHostileFragments(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		repo := subs[chRepo]
		_ = recv(t, repo) // PeerUp
		_ = recv(t, subs[chTrust])
		hostile := [][]byte{
			{fragChannel},
			{fragChannel, chRepo, 1, 2, 3},
			frag(chRepo, 1, 0, 0, nil), // empty message
			frag(chRepo, 1, uint32(transport.MaxFrame), 0, []byte("x")), // over the limit
			frag(0x09, 1, 10, 0, []byte("abc")),                         // nobody's channel
			frag(chRepo, 2, 10, 5, []byte("abc")),                       // continuation of nothing
			frag(chRepo, 3, 10, 0, []byte("0123456789ABC")),             // more than total
			frag(chRepo, 4, 10, 0, []byte("0123")),                      // start…
			frag(chRepo, 4, 10, 5, []byte("567")),                       // …gap
			frag(chRepo, 4, 10, 4, []byte("456789")),                    // …after abort
			frag(chRepo, 5, 10, 0, []byte("0123")),                      // start…
			frag(chRepo, 6, 10, 4, []byte("456789")),                    // …other id
			frag(chRepo, 7, 10, 0, []byte("0123")),                      // start…
			frag(chRepo, 7, 11, 4, []byte("456789")),                    // …other total
		}
		for _, m := range hostile {
			if err := raw.Send(t.Context(), "b", m); err != nil {
				t.Fatal(err)
			}
		}
		// A valid two-fragment message still works afterwards.
		for _, m := range [][]byte{frag(chRepo, 8, 6, 0, []byte("abc")), frag(chRepo, 8, 6, 3, []byte("def"))} {
			if err := raw.Send(t.Context(), "b", m); err != nil {
				t.Fatal(err)
			}
		}
		if ev := recv(t, repo); string(ev.Payload) != "abcdef" {
			t.Errorf("after hostile fragments got %q, want %q", ev.Payload, "abcdef")
		}
		synctest.Wait()
		if ev, ok := poll(repo); ok {
			t.Errorf("extra event %v %q", ev.Kind, ev.Payload)
		}
	})
}

// TestPeerDownDropsPartial: a partial message does not survive its peer.
func TestPeerDownDropsPartial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		raw := n.Join("a")
		repo := mux.Split(n.Join("b"), chRepo)[chRepo]
		defer func() { _ = raw.Close(); _ = repo.Close() }()
		n.Link("a", "b")
		_ = recv(t, repo)
		if err := raw.Send(t.Context(), "b", frag(chRepo, 1, 6, 0, []byte("abc"))); err != nil {
			t.Fatal(err)
		}
		n.Partition("a", "b")
		n.Link("a", "b")
		if err := raw.Send(t.Context(), "b", frag(chRepo, 1, 6, 3, []byte("def"))); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		for {
			ev, ok := poll(repo)
			if !ok {
				return
			}
			if ev.Kind == transport.Message {
				t.Fatalf("delivered %q across a reconnect", ev.Payload)
			}
		}
	})
}

// FuzzFragment feeds arbitrary fragment frames: nothing may panic, hang or
// deliver more than MaxFrame, and a sentinel sent after must still arrive.
func FuzzFragment(f *testing.F) {
	f.Add(frag(chRepo, 1, 6, 0, []byte("abc")), frag(chRepo, 1, 6, 3, []byte("def")))
	f.Add(frag(chRepo, 1, 1<<30, 0, []byte("a")), []byte{fragChannel})
	f.Add([]byte{fragChannel, chRepo}, frag(chTrust, 9, 3, 0, []byte("xyz")))
	f.Fuzz(func(t *testing.T, x, y []byte) {
		synctest.Test(t, func(t *testing.T) {
			raw, subs := pair(t, chRepo, chTrust)
			for _, c := range subs {
				_ = recv(t, c)
			}
			for _, m := range [][]byte{append([]byte{fragChannel}, x...), append([]byte{fragChannel}, y...), {chRepo, 's'}} {
				if err := raw.Send(t.Context(), "b", m); err != nil {
					t.Fatal(err)
				}
			}
			synctest.Wait()
			sentinel := false
			for _, c := range subs {
				for {
					ev, ok := poll(c)
					if !ok {
						break
					}
					if len(ev.Payload) > transport.MaxFrame {
						t.Fatalf("delivered %d bytes", len(ev.Payload))
					}
					if bytes.Equal(ev.Payload, []byte{'s'}) {
						sentinel = true
					}
				}
			}
			if !sentinel {
				t.Error("sentinel lost")
			}
		})
	})
}
