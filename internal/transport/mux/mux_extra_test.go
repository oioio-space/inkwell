package mux_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// drain returns every event currently queued on t.
func drain(t transport.Transport) []transport.Event {
	var evs []transport.Event
	for {
		ev, ok := poll(t)
		if !ok {
			return evs
		}
		evs = append(evs, ev)
	}
}

// mustSend sends one message from raw to b.
func mustSend(t *testing.T, raw transport.Transport, payload []byte) {
	t.Helper()
	if err := raw.Send(t.Context(), "b", payload); err != nil {
		t.Fatalf("Send(%d bytes) = %v", len(payload), err)
	}
}

// skipPeerUp consumes the PeerUp every channel starts with.
func skipPeerUp(t *testing.T, cs ...transport.Transport) {
	t.Helper()
	for _, c := range cs {
		if ev, err := c.Recv(t.Context()); err != nil || ev.Kind != transport.PeerUp {
			t.Fatalf("first event = %v, %v; want PeerUp", ev, err)
		}
	}
}

// TestSendBufferReuse: frames of every size around the pooled-buffer limit,
// sent back to back on two channels, arrive intact and in order: reusing a
// send buffer must never alias an earlier frame.
func TestSendBufferReuse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		as := mux.Split(n.Join("a"), chRepo, chTrust)
		bs := mux.Split(n.Join("b"), chRepo, chTrust)
		t.Cleanup(func() { _ = as[chRepo].Close(); _ = bs[chRepo].Close() })
		n.Link("a", "b")
		skipPeerUp(t, bs[chRepo], bs[chTrust])
		want := map[byte][][]byte{}
		for i, size := range []int{0, 1, 100, 64<<10 - 1, 64 << 10, 64<<10 + 1, 100, 3, 200 << 10, 5} {
			ch := byte(chRepo)
			if i%2 == 1 {
				ch = chTrust
			}
			payload := bytes.Repeat([]byte{byte('a' + i)}, size)
			if err := as[ch].Send(t.Context(), "b", payload); err != nil {
				t.Fatal(err)
			}
			want[ch] = append(want[ch], payload)
		}
		synctest.Wait()
		for ch, c := range map[byte]transport.Transport{chRepo: bs[chRepo], chTrust: bs[chTrust]} {
			got := messages(c)
			if len(got) != len(want[ch]) {
				t.Fatalf("channel %#x: %d messages, want %d", ch, len(got), len(want[ch]))
			}
			for i := range got {
				if !bytes.Equal(got[i], want[ch][i]) {
					t.Errorf("channel %#x message %d: len %d first %v, want len %d first %v", ch, i, len(got[i]), head(got[i]), len(want[ch][i]), head(want[ch][i]))
				}
			}
		}
	})
}

func head(b []byte) []byte { return b[:min(len(b), 3)] }

// TestPeerEventsOnEveryChannelInOrder: PeerUp/PeerDown reach every channel,
// ordered with the messages of that channel.
func TestPeerEventsOnEveryChannelInOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		mustSend(t, raw, []byte{chRepo, '1'})
		mustSend(t, raw, []byte{chTrust, '2'})
		mustSend(t, raw, []byte{chRepo, '3'})
		if err := raw.Close(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		for ch, want := range map[byte][]string{
			chRepo:  {"up", "1", "3", "down"},
			chTrust: {"up", "2", "down"},
		} {
			var got []string
			for _, ev := range drain(subs[ch]) {
				switch ev.Kind {
				case transport.PeerUp:
					got = append(got, "up")
				case transport.PeerDown:
					got = append(got, "down")
				default:
					got = append(got, string(ev.Payload))
				}
				if ev.Peer != "a" {
					t.Errorf("channel %#x event %v from %q, want a", ch, ev.Kind, ev.Peer)
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("channel %#x events = %v, want %v", ch, got, want)
			}
		}
	})
}

// TestPerChannelOrder: interleaved traffic keeps each channel's order.
func TestPerChannelOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		skipPeerUp(t, subs[chRepo], subs[chTrust])
		const n = 100
		for i := range n {
			mustSend(t, raw, []byte{chRepo, byte(i)})
			mustSend(t, raw, []byte{chTrust, byte(n - i)})
		}
		synctest.Wait()
		for ch, f := range map[byte]func(int) byte{chRepo: func(i int) byte { return byte(i) }, chTrust: func(i int) byte { return byte(n - i) }} {
			evs := drain(subs[ch])
			if len(evs) != n {
				t.Fatalf("channel %#x got %d events, want %d", ch, len(evs), n)
			}
			for i, ev := range evs {
				if len(ev.Payload) != 1 || ev.Payload[0] != f(i) {
					t.Fatalf("channel %#x event %d = %v, want %d", ch, i, ev.Payload, f(i))
				}
			}
		}
	})
}

func TestSplitDuplicate(t *testing.T) {
	n := memnet.New(1, memnet.Faults{})
	subs := mux.Split(n.Join("b"), chRepo, chRepo, chTrust)
	if len(subs) != 2 || subs[chRepo] == nil || subs[chTrust] == nil {
		t.Errorf("Split(1, 1, 2) = %d subs, want channels 1 and 2 once", len(subs))
	}
	if err := subs[chRepo].Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}

// TestCloseClosesEveryChannel: closing one channel closes the transport and so
// all of them; Close is idempotent; Send fails after.
func TestCloseClosesEveryChannel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, subs := pair(t, chRepo, chTrust)
		repo, trust := subs[chRepo], subs[chTrust]
		skipPeerUp(t, repo, trust)
		blocked := make(chan error, 1)
		go func() { _, err := trust.Recv(context.Background()); blocked <- err }()
		synctest.Wait()

		if err := repo.Close(); err != nil {
			t.Fatalf("Close = %v", err)
		}
		if err := <-blocked; !errors.Is(err, transport.ErrClosed) {
			t.Errorf("blocked Recv on the other channel = %v, want ErrClosed", err)
		}
		for ch, c := range subs {
			if _, err := c.Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
				t.Errorf("Recv(%#x) after Close = %v, want ErrClosed", ch, err)
			}
			if err := c.Send(t.Context(), "a", []byte("x")); err == nil {
				t.Errorf("Send(%#x) after Close = nil, want an error", ch)
			}
		}
		if err := trust.Close(); err != nil {
			t.Errorf("second Close = %v, want the first result (nil)", err)
		}
	})
}

// TestRecvCanceledEmpty: with nothing queued, Recv honours its context.
func TestRecvCanceledEmpty(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, subs := pair(t, chRepo)
		skipPeerUp(t, subs[chRepo])
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := subs[chRepo].Recv(ctx); !errors.Is(err, context.Canceled) {
			t.Errorf("Recv(canceled, empty) = %v, want context.Canceled", err)
		}
	})
}

func TestLocalAndPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, subs := pair(t, chRepo)
		repo := subs[chRepo]
		skipPeerUp(t, repo)
		if got := repo.Local(); got != "b" {
			t.Errorf("Local() = %q, want b", got)
		}
		if got := repo.Peers(); len(got) != 1 || got[0] != "a" {
			t.Errorf("Peers() = %v, want [a]", got)
		}
	})
}

// TestWakeOnClose: closing wakes a consumer parked on Wake.
func TestWakeOnClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, subs := pair(t, chRepo, chTrust)
		skipPeerUp(t, subs[chRepo], subs[chTrust])
		wake := mux.Wake(subs[chTrust])
		select {
		case <-wake:
		default:
		}
		if err := subs[chRepo].Close(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-wake:
		default:
			t.Error("no wakeup after Close")
		}
	})
}

// TestBackpressureCount: a channel holds 256 events; the 257th stops the
// reader, and with it every other channel, without losing anything; popping
// one event lets traffic flow again.
func TestBackpressureCount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		repo, trust := subs[chRepo], subs[chTrust]
		skipPeerUp(t, repo, trust)
		const queued = 256
		for i := range queued {
			mustSend(t, raw, []byte{chTrust, byte(i)})
		}
		mustSend(t, raw, []byte{chRepo, 'r'})
		synctest.Wait()
		if ev, ok := poll(repo); !ok || string(ev.Payload) != "r" {
			t.Fatalf("with a queue exactly full, repo got %q, %v; want r", ev.Payload, ok)
		}

		mustSend(t, raw, []byte{chTrust, 'x'}) // the 257th: the reader parks on it
		mustSend(t, raw, []byte{chRepo, 's'})
		synctest.Wait()
		if ev, ok := poll(repo); ok {
			t.Fatalf("repo received %q while trust is full; the reader should be blocked", ev.Payload)
		}

		first, err := trust.Recv(t.Context())
		if err != nil || len(first.Payload) != 1 || first.Payload[0] != 0 {
			t.Fatalf("first trust event = %v, %v", first.Payload, err)
		}
		synctest.Wait()
		if ev, ok := poll(repo); !ok || string(ev.Payload) != "s" {
			t.Errorf("repo after a pop = %q, %v; want s", ev.Payload, ok)
		}
		rest := drain(trust)
		if len(rest) != queued {
			t.Fatalf("trust has %d events left, want %d (nothing lost)", len(rest), queued)
		}
		if last := rest[len(rest)-1].Payload; string(last) != "x" {
			t.Errorf("last trust event = %q, want x", last)
		}
	})
}

// TestBackpressureBytes: a channel holds up to 32 MiB of payload; one byte
// more parks the reader; a single frame bigger than the cap is still taken
// when the queue is empty.
func TestBackpressureBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		repo, trust := subs[chRepo], subs[chTrust]
		skipPeerUp(t, repo, trust)
		const half = 16 << 20
		big := func(n int) []byte { return append([]byte{chTrust}, make([]byte, n)...) }

		mustSend(t, raw, big(half))
		mustSend(t, raw, big(half)) // exactly 32 MiB queued
		mustSend(t, raw, []byte{chRepo, 'r'})
		synctest.Wait()
		if ev, ok := poll(repo); !ok || string(ev.Payload) != "r" {
			t.Fatalf("with exactly 32 MiB queued, repo got %q, %v; want r", ev.Payload, ok)
		}

		mustSend(t, raw, big(1)) // 32 MiB + 1: parks the reader
		mustSend(t, raw, []byte{chRepo, 's'})
		synctest.Wait()
		if _, ok := poll(repo); ok {
			t.Fatal("repo received a message while trust is over its byte budget")
		}
		if _, err := trust.Recv(t.Context()); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if ev, ok := poll(repo); !ok || string(ev.Payload) != "s" {
			t.Errorf("repo after a pop = %q, %v; want s", ev.Payload, ok)
		}
		if got := drain(trust); len(got) != 2 {
			t.Errorf("trust has %d events left, want 2", len(got))
		}

		// A frame above the cap alone: accepted into the empty queue.
		mustSend(t, raw, big(33<<20))
		synctest.Wait()
		if got := drain(trust); len(got) != 1 || len(got[0].Payload) != 33<<20 {
			t.Errorf("33 MiB frame into an empty queue: got %d events", len(got))
		}
	})
}

// TestCloseWhileBlocked: closing the transport while the reader is parked on a
// full queue stops it, with no goroutine left behind (goleak in TestMain).
func TestCloseWhileBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, subs := pair(t, chRepo, chTrust)
		skipPeerUp(t, subs[chRepo], subs[chTrust])
		for range 300 {
			mustSend(t, raw, []byte{chTrust, 1})
		}
		synctest.Wait()
		if err := subs[chRepo].Close(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if _, err := subs[chTrust].Recv(t.Context()); !errors.Is(err, transport.ErrClosed) {
			t.Errorf("Recv after Close with a full queue = %v, want ErrClosed", err)
		}
	})
}
