package mux_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// hostileRig is a mux over a transport with a memory budget, fed by raw peers.
type hostileRig struct {
	n    *memnet.Network
	res  *reserving
	repo transport.Transport
	// trust is a second channel, to check that a message never changes channel.
	trust transport.Transport
}

func newHostileRig(t *testing.T, peers ...transport.PeerID) (*hostileRig, map[transport.PeerID]*memnet.Node) {
	t.Helper()
	n := memnet.New(1, memnet.Faults{})
	raws := map[transport.PeerID]*memnet.Node{}
	res := &reserving{Transport: n.Join("b"), limit: 1 << 30}
	subs := mux.Split(res, chRepo, chTrust)
	repo := subs[chRepo]
	t.Cleanup(func() {
		for _, r := range raws {
			_ = r.Close()
		}
		_ = repo.Close()
	})
	for _, p := range peers {
		raws[p] = n.Join(p)
		n.Link(p, "b")
	}
	return &hostileRig{n: n, res: res, repo: repo, trust: subs[chTrust]}, raws
}

func sendFrag(t *testing.T, from *memnet.Node, p []byte) {
	t.Helper()
	if err := from.Send(t.Context(), "b", p); err != nil {
		t.Fatal(err)
	}
}

// messages returns the message payloads queued on c.
func messages(c transport.Transport) [][]byte {
	var out [][]byte
	for _, ev := range drain(c) {
		if ev.Kind == transport.Message {
			out = append(out, ev.Payload)
		}
	}
	return out
}

// TestHeaderOnlyFragmentAbortsMessage: a fragment with a full header and no
// data is invalid, and ends the message in progress like any other invalid one.
func TestHeaderOnlyFragmentAbortsMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x")
		x := raws["x"]
		sendFrag(t, x, frag(chRepo, 1, 10, 0, []byte("abc")))
		sendFrag(t, x, frag(chRepo, 1, 10, 3, nil))
		sendFrag(t, x, frag(chRepo, 1, 10, 3, []byte("defghij")))
		synctest.Wait()
		if got := messages(rig.repo); len(got) != 0 {
			t.Errorf("delivered %q after a header-only fragment aborted the message", got)
		}
		if got := rig.res.inUse(); got != 0 {
			t.Errorf("%d bytes still reserved after the abort", got)
		}
	})
}

// TestFragmentFieldBoundaries: the limits of one fragment's fields are exact.
func TestFragmentFieldBoundaries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x")
		x := raws["x"]

		// A whole message in one fragment: total == len(data), for tiny and
		// fragment-sized data.
		for i, size := range []int{1, 2, 32 << 10} {
			sendFrag(t, x, frag(chRepo, uint32(i+1), uint32(size), 0, content(size, byte(i))))
		}
		synctest.Wait()
		got := messages(rig.repo)
		if len(got) != 3 || len(got[0]) != 1 || len(got[1]) != 2 || !bytes.Equal(got[2], content(32<<10, 2)) {
			t.Fatalf("one-fragment messages = %d delivered, want 3 of sizes 1, 2, 32768", len(got))
		}

		// One byte more than the total announces: refused, nothing kept.
		sendFrag(t, x, frag(chRepo, 10, 4, 0, []byte("abcde")))
		synctest.Wait()
		if got := messages(rig.repo); len(got) != 0 || rig.res.inUse() != 0 {
			t.Errorf("fragment longer than its total: delivered %d, reserved %d; want 0, 0", len(got), rig.res.inUse())
		}

		// The largest total is accepted (and kept), one more is refused.
		sendFrag(t, x, frag(chRepo, 11, uint32(transport.MaxFrame-mux.Overhead), 0, []byte("a")))
		synctest.Wait()
		if rig.res.inUse() == 0 {
			t.Error("a message of the largest allowed total was refused")
		}
		sendFrag(t, x, frag(chRepo, 12, uint32(transport.MaxFrame-mux.Overhead+1), 0, []byte("a")))
		synctest.Wait()
		if got := rig.res.inUse(); got != 0 {
			t.Errorf("a total one past the limit left %d bytes reserved (and dropped the previous message)", got)
		}

		// Total of 0 and of 1 with a continuation at the edge.
		sendFrag(t, x, frag(chRepo, 13, 0, 0, []byte("a")))
		synctest.Wait()
		if got := messages(rig.repo); len(got) != 0 {
			t.Errorf("a zero-total fragment delivered %q", got)
		}
	})
}

// TestReassemblyChargesByCapacity pins the memory accounting: capacity grows
// to one fragment first, then doubles, never past the announced total, and is
// all given back on completion.
func TestReassemblyChargesByCapacity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const kib = 1 << 10
		rig, raws := newHostileRig(t, "x")
		x := raws["x"]
		total := 100 * kib
		data := content(total, 5)
		off := 0
		send := func(n int) {
			sendFrag(t, x, frag(chRepo, 1, uint32(total), uint32(off), data[off:off+n]))
			off += n
			synctest.Wait()
		}
		check := func(step string, want int) {
			t.Helper()
			if got := rig.res.inUse(); got != want*kib {
				t.Errorf("%s: reserved %d KiB, want %d KiB", step, got/kib, want)
			}
		}
		send(1)
		check("after 1 byte", 32)
		send(32*kib - 1) // exactly fills the capacity: no growth
		check("after filling 32 KiB exactly", 32)
		send(1) // one byte more: doubles
		check("after 32 KiB + 1", 64)
		send(32*kib - 1) // exactly 64 KiB
		check("after filling 64 KiB exactly", 64)
		send(1)
		check("after 64 KiB + 1", 100) // doubling would be 128: capped by the total
		send(total - off)
		check("after completion", 0)
		got := messages(rig.repo)
		if len(got) != 1 || !bytes.Equal(got[0], data) {
			t.Fatalf("delivered %d messages; want the one intact message", len(got))
		}
	})
}

// TestFragmentJumpGrowsToFit: a fragment bigger than the doubled capacity
// still fits at once.
func TestFragmentJumpGrowsToFit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x")
		x := raws["x"]
		sendFrag(t, x, frag(chRepo, 1, 1<<20, 0, make([]byte, 100<<10)))
		synctest.Wait()
		if got := rig.res.inUse(); got != 100<<10 {
			t.Errorf("reserved %d bytes for a 100 KiB first fragment, want exactly 100 KiB", got)
		}
	})
}

// TestStaleExactlyAtTTL: a partial message is dropped when it has gone
// exactly 30 s without a fragment, not one tick later.
func TestStaleExactlyAtTTL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x")
		sendFrag(t, raws["x"], frag(chRepo, 1, 1<<20, 0, make([]byte, 1024)))
		synctest.Wait()
		time.Sleep(30*time.Second - time.Nanosecond)
		synctest.Wait()
		if rig.res.inUse() == 0 {
			t.Fatal("dropped before the TTL")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if got := rig.res.inUse(); got != 0 {
			t.Errorf("%d bytes still reserved at exactly the TTL", got)
		}
	})
}

// TestReaderSurvivesExpiry: the wake-up that drops stale partial messages must
// not stop the reader: traffic after it is delivered.
func TestReaderSurvivesExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x", "y")
		sendFrag(t, raws["x"], frag(chRepo, 1, 1<<20, 0, make([]byte, 1024)))
		synctest.Wait()
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		sendFrag(t, raws["y"], []byte{chRepo, 'o', 'k'})
		synctest.Wait()
		var got []string
		for _, m := range messages(rig.repo) {
			got = append(got, string(m))
		}
		if len(got) != 1 || got[0] != "ok" {
			t.Errorf("after an expiry, delivered %q, want [ok]", got)
		}
	})
}

// TestPeerDownReleasesPartial: when a peer goes away, its partial message and
// the memory it holds are released at once, not at the TTL.
func TestPeerDownReleasesPartial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x", "y")
		sendFrag(t, raws["x"], frag(chRepo, 1, 1<<20, 0, make([]byte, 1024)))
		sendFrag(t, raws["y"], frag(chRepo, 1, 1<<20, 0, make([]byte, 1024)))
		synctest.Wait()
		both := rig.res.inUse()
		rig.n.Partition("x", "b")
		synctest.Wait()
		if got := rig.res.inUse(); got != both/2 {
			t.Errorf("after x went down: reserved %d, want %d (only y's message)", got, both/2)
		}
	})
}

// TestDuplicateFragmentIgnored: a fragment the link repeats changes nothing.
func TestDuplicateFragmentIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x")
		x := raws["x"]
		first := frag(chRepo, 1, 6, 0, []byte("abc"))
		sendFrag(t, x, first)
		sendFrag(t, x, first)
		sendFrag(t, x, frag(chRepo, 1, 6, 3, []byte("def")))
		sendFrag(t, x, frag(chRepo, 1, 6, 3, []byte("def"))) // after completion: a new, orphan continuation
		synctest.Wait()
		got := messages(rig.repo)
		if len(got) != 1 || string(got[0]) != "abcdef" {
			t.Errorf("delivered %q, want exactly one abcdef", got)
		}
	})
}

// TestInterleavedPeersAndChannels: two peers send large messages to two
// channels at once; each message arrives whole on its own channel.
func TestInterleavedPeersAndChannels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		xs := mux.Split(n.Join("x"), chRepo, chTrust)
		ys := mux.Split(n.Join("y"), chRepo, chTrust)
		bs := mux.Split(n.Join("b"), chRepo, chTrust)
		t.Cleanup(func() { _ = xs[chRepo].Close(); _ = ys[chRepo].Close(); _ = bs[chRepo].Close() })
		n.Link("x", "b")
		n.Link("y", "b")
		skipPeerUp(t, bs[chRepo], bs[chTrust])
		skipPeerUp(t, bs[chRepo], bs[chTrust])
		msgs := map[string]struct {
			from transport.Transport
			ch   byte
			data []byte
		}{
			"x/repo":  {xs[chRepo], chRepo, content(200<<10, 1)},
			"x/trust": {xs[chTrust], chTrust, content(100<<10, 2)},
			"y/repo":  {ys[chRepo], chRepo, content(300<<10, 3)},
			"y/trust": {ys[chTrust], chTrust, content(65<<10, 4)},
		}
		for _, m := range msgs {
			go func() { _ = m.from.Send(t.Context(), "b", m.data) }()
		}
		synctest.Wait()
		for ch, c := range map[byte]transport.Transport{chRepo: bs[chRepo], chTrust: bs[chTrust]} {
			got := messages(c)
			if len(got) != 2 {
				t.Fatalf("channel %#x got %d messages, want 2", ch, len(got))
			}
			for _, g := range got {
				ok := false
				for _, m := range msgs {
					ok = ok || (m.ch == ch && bytes.Equal(m.data, g))
				}
				if !ok {
					t.Errorf("channel %#x got a message of %d bytes that nobody sent there", ch, len(g))
				}
			}
		}
	})
}

// TestWireBoundaries: a payload up to 64 KiB is one frame on the channel byte;
// one byte more is cut into 32 KiB fragments, exactly as many as needed, with
// consistent headers.
func TestWireBoundaries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		raw := n.Join("a")
		sub := mux.Split(n.Join("b"), chRepo)[chRepo]
		defer func() { _ = raw.Close(); _ = sub.Close() }()
		n.Link("a", "b")
		recvKind(t, raw, transport.PeerUp)

		for _, tc := range []struct {
			size  int
			frags []int // payload size of each fragment; nil: a single plain frame
		}{
			{64<<10 - 1, nil},
			{64 << 10, nil},
			{64<<10 + 1, []int{32 << 10, 32 << 10, 1}},
			{96 << 10, []int{32 << 10, 32 << 10, 32 << 10}},
			{96<<10 + 1, []int{32 << 10, 32 << 10, 32 << 10, 1}},
		} {
			payload := content(tc.size, 9)
			if err := sub.Send(t.Context(), "a", payload); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			var frames [][]byte
			for _, ev := range drain(raw) {
				frames = append(frames, ev.Payload)
			}
			if tc.frags == nil {
				if len(frames) != 1 || frames[0][0] != chRepo || !bytes.Equal(frames[0][1:], payload) {
					t.Errorf("size %d: want one plain frame, got %d frames", tc.size, len(frames))
				}
				continue
			}
			if len(frames) != len(tc.frags) {
				t.Errorf("size %d: %d frames, want %d fragments", tc.size, len(frames), len(tc.frags))
				continue
			}
			var id uint32
			off := 0
			for i, f := range frames {
				if f[0] != fragChannel || f[1] != chRepo || len(f)-14 != tc.frags[i] {
					t.Errorf("size %d fragment %d: kind %#x channel %#x data %d, want %#x %#x %d", tc.size, i, f[0], f[1], len(f)-14, fragChannel, chRepo, tc.frags[i])
					continue
				}
				if i == 0 {
					id = binary.BigEndian.Uint32(f[2:])
				}
				if got := binary.BigEndian.Uint32(f[2:]); got != id {
					t.Errorf("size %d fragment %d: id %d, want %d", tc.size, i, got, id)
				}
				if got := binary.BigEndian.Uint32(f[6:]); int(got) != tc.size {
					t.Errorf("size %d fragment %d: total %d", tc.size, i, got)
				}
				if got := binary.BigEndian.Uint32(f[10:]); int(got) != off {
					t.Errorf("size %d fragment %d: offset %d, want %d", tc.size, i, got, off)
				}
				if !bytes.Equal(f[14:], payload[off:off+tc.frags[i]]) {
					t.Errorf("size %d fragment %d: wrong data", tc.size, i)
				}
				off += tc.frags[i]
			}
		}
	})
}

func recvKind(t *testing.T, c transport.Transport, kind transport.Kind) {
	t.Helper()
	ev, err := c.Recv(t.Context())
	if err != nil || ev.Kind != kind {
		t.Fatalf("Recv = %v, %v; want kind %v", ev.Kind, err, kind)
	}
}

// TestGateBookkeeping: the per-peer gate of a large message exists exactly
// while someone sends to that peer, and is gone, with its counter, after.
func TestGateBookkeeping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b := linked(t, memnet.Faults{}, func(x transport.Transport) transport.Transport { return newSlow(x, 256<<10) })
		gates := func(step string, want int) {
			t.Helper()
			if g, c := mux.GateCount(a); g != want || int(c) != want {
				t.Errorf("%s: %d gates (counter %d), want %d", step, g, c, want)
			}
		}
		gates("idle", 0)
		if err := a.Send(t.Context(), "b", []byte("small")); err != nil {
			t.Fatal(err)
		}
		gates("after a small send", 0)

		done := make(chan error, 3)
		big := content(256<<10, 1)
		go func() { done <- a.Send(t.Context(), "b", big) }()
		synctest.Wait()
		gates("during a large send", 1)
		go func() { done <- a.Send(t.Context(), "b", []byte("typing")) }() // leases the gate
		go func() { done <- a.Send(t.Context(), "b", content(70<<10, 2)) }()
		synctest.Wait()
		gates("with a small and a second large send waiting", 1)
		for range 3 {
			if err := <-done; err != nil {
				t.Fatalf("Send = %v", err)
			}
		}
		gates("after every send", 0)
		synctest.Wait()
		if got := len(messages(b)); got != 4 {
			t.Errorf("b received %d messages, want 4", got)
		}
	})
}

// TestGateReleasedOnCancel: a canceled large send leaves no gate behind.
func TestGateReleasedOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, _ := linked(t, memnet.Faults{}, func(x transport.Transport) transport.Transport { return newSlow(x, 64<<10) })
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- a.Send(ctx, "b", content(512<<10, 1)) }()
		synctest.Wait()
		if g, _ := mux.GateCount(a); g != 1 {
			t.Fatalf("%d gates during the send, want 1", g)
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("canceled Send = %v, want context.Canceled", err)
		}
		if g, c := mux.GateCount(a); g != 0 || c != 0 {
			t.Errorf("%d gates (counter %d) after cancel, want 0", g, c)
		}
	})
}
