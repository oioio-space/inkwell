// SPDX-License-Identifier: MPL-2.0

package linkprobe_test

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

// The shared corpus (vectors/linkprobe.json, run by protocol/conformance) carries the normative
// cases of spec §8.5; these tests cover what a script cannot reach cheaply: the bounds of §8.5.7
// at and past their value, counters near 2^64, and decoding a stream message by message.

const ms = time.Millisecond

// Spec §8.5.4: a stream is a sequence of messages; Decode reads one and says how many bytes it
// took, or ErrShort while the head is incomplete (the caller reads more, up to 64 bytes).
func TestDecodeStream(t *testing.T) {
	stream := []byte{0x05, 0x01, 0, 0, 0, 1, 0x01, 0x4a, 0x0d, 0x02, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 9}
	var got []linkprobe.Message
	for len(stream) > 0 {
		m, n, err := linkprobe.Decode(stream)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, m)
		stream = stream[n:]
	}
	if len(got) != 3 || got[0].Seq != 1 || !got[1].Ignorable() || got[2].Received != 9 {
		t.Fatalf("got %+v", got)
	}
	for _, head := range [][]byte{{}, {0x05}, {0x05, 0x01, 0, 0}, {0x0d, 0x02, 0, 0, 0, 1}} {
		if _, _, err := linkprobe.Decode(head); !errors.Is(err, linkprobe.ErrShort) {
			t.Errorf("Decode(% x) = %v, want ErrShort", head, err)
		}
	}
	// A length above the bound is refused from the first byte, without waiting for the body.
	if _, _, err := linkprobe.Decode([]byte{0x41}); !errors.Is(err, linkprobe.ErrMalformed) {
		t.Errorf("len 65: %v, want ErrMalformed", err)
	}
	if _, _, err := linkprobe.Decode([]byte{0x00}); !errors.Is(err, linkprobe.ErrMalformed) {
		t.Errorf("len 0: %v, want ErrMalformed", err)
	}
}

// Spec §8.5.4: Encode refuses what Decode refuses, so nothing malformed is ever written.
func TestEncodeRefusesInvalid(t *testing.T) {
	for _, m := range []linkprobe.Message{
		{Type: linkprobe.TypeProbe},
		{Type: linkprobe.TypeEcho, Received: 3},
		{Type: 0x03},
		{Type: 0x00},
		{Type: 0x80},
		{Type: 0x4a, Body: make([]byte, linkprobe.MaxLen)},
	} {
		if _, err := linkprobe.Encode(m); err == nil {
			t.Errorf("Encode(%+v) accepted", m)
		}
	}
}

// FuzzDecode: no input panics; an accepted message re-encodes to exactly the bytes it took
// (unique encoding, spec §1.3), and never takes more than MaxLen + 1 bytes.
func FuzzDecode(f *testing.F) {
	for _, s := range [][]byte{
		{0x05, 0x01, 0, 0, 0, 1},
		{0x0d, 0x02, 0, 0, 0, 7, 0, 0, 0, 0, 0, 0, 3, 0xe8},
		{0x03, 0x4a, 0xaa, 0xbb},
		{0x41, 0x4a},
		{0x00},
		{0x01, 0x80},
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		m, n, err := linkprobe.Decode(b)
		if err != nil {
			if !errors.Is(err, linkprobe.ErrShort) && !errors.Is(err, linkprobe.ErrMalformed) {
				t.Fatalf("unclassified error %v", err)
			}
			return
		}
		if n < 2 || n > linkprobe.MaxLen+1 || n > len(b) {
			t.Fatalf("took %d of %d bytes", n, len(b))
		}
		enc, err := linkprobe.Encode(m)
		if err != nil {
			t.Fatalf("accepted %x but cannot encode it: %v", b[:n], err)
		}
		if !bytes.Equal(enc, b[:n]) {
			t.Fatalf("round trip % x -> % x", b[:n], enc)
		}
	})
}

func open(t *testing.T) *linkprobe.Prober {
	t.Helper()
	p := linkprobe.NewProber()
	if err := p.Open(0); err != nil {
		t.Fatal(err)
	}
	return p
}

func echo(seq uint32, r uint64) linkprobe.Message {
	return linkprobe.Message{Type: linkprobe.TypeEcho, Seq: seq, Received: r}
}

// Spec §8.5.7: at most MaxOutstanding probes wait for their echo; the oldest is forgotten.
func TestOutstandingBound(t *testing.T) {
	p := open(t)
	n := linkprobe.MaxOutstanding + 1
	for i := range n {
		if _, err := p.Probe(time.Duration(i)*time.Second, uint64(i), true); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Duration(n) * time.Second
	if got := p.Receive(now, echo(1, 0), 100, time.Hour); got != linkprobe.Stale {
		t.Fatalf("echo of the oldest of %d probes: %v, want stale", n, got)
	}
	if got := p.Receive(now, echo(2, 0), 100, time.Hour); got != linkprobe.Accepted {
		t.Fatalf("echo of the oldest kept: %v, want accepted", got)
	}
}

// Spec §8.5.7: at most MaxSamples samples are kept, and the figure only reads the window.
func TestSampleBound(t *testing.T) {
	p := open(t)
	var sent uint64
	for i := range linkprobe.MaxSamples + 10 {
		now := time.Duration(i) * linkprobe.MinSpacing // twice the rate of the interval: the window would hold > M samples
		if i > 0 && now-time.Duration(i-1)*linkprobe.MinSpacing < linkprobe.MinSpacing {
			t.Fatal("test spacing below MinSpacing")
		}
		if _, err := p.Probe(now, sent, true); err != nil {
			t.Fatal(err)
		}
		if got := p.Receive(now+ms, echo(uint32(i+1), sent), sent+1, time.Second); got != linkprobe.Accepted {
			t.Fatalf("probe %d: %v", i, got)
		}
		sent += 100
	}
	if got := p.Samples(); got != linkprobe.MaxSamples {
		t.Fatalf("%d samples kept, want %d", got, linkprobe.MaxSamples)
	}
	if got := p.Loss(time.Duration(linkprobe.MaxSamples+9) * linkprobe.MinSpacing); got != 0 {
		t.Fatalf("lossless window: %d permille", got)
	}
}

// Spec §8.5.5: the figure is exact for any u64 counters (a 128-bit product, no overflow).
func TestLossExactNearMaxUint64(t *testing.T) {
	p := open(t)
	base := uint64(math.MaxUint64 - 4_000_000_000_000_000_000)
	if _, err := p.Probe(0, base, true); err != nil {
		t.Fatal(err)
	}
	p.Receive(ms, echo(1, base-10), base, time.Second)
	at := linkprobe.MinSpan // the two samples are MinSpan apart
	if _, err := p.Probe(at, base+3_000_000_000_000_000_000, true); err != nil {
		t.Fatal(err)
	}
	// 3e18 sent, 1e18 lost: 333.33... permille, rounds to 333.
	got := p.Receive(at+ms, echo(2, base-10+2_000_000_000_000_000_000), math.MaxUint64, time.Second)
	if got != linkprobe.Accepted {
		t.Fatalf("receive: %v", got)
	}
	if l := p.Loss(at + ms); l != 333 {
		t.Fatalf("loss %d permille, want 333", l)
	}
}

// Spec §8.5.5 step 5, computed exactly: 1000 × L + ⌊ΔS / 2⌋ overflows 64 bits (the carry of the
// 128-bit sum) for these counters; 17 446 744 073 709 551 lost of 3e18 is 5.8 permille, rounds to 6.
func TestLossRoundingCarry(t *testing.T) {
	p := open(t)
	const dS, lost = 3_000_000_000_000_000_000, 17_446_744_073_709_551
	if _, err := p.Probe(0, 10, true); err != nil {
		t.Fatal(err)
	}
	p.Receive(ms, echo(1, 0), 10, time.Second)
	if _, err := p.Probe(linkprobe.MinSpan, 10+dS, true); err != nil {
		t.Fatal(err)
	}
	if got := p.Receive(linkprobe.MinSpan+ms, echo(2, dS-lost), math.MaxUint64, time.Second); got != linkprobe.Accepted {
		t.Fatalf("receive: %v", got)
	}
	if l := p.Loss(linkprobe.MinSpan + ms); l != 6 {
		t.Fatalf("loss %d permille, want 6", l)
	}
}

// Spec §8.5.6: the credit is per connection; the elapsed time is capped before it is added,
// so an instant far in the future cannot overflow it.
func TestResponderCreditCapped(t *testing.T) {
	r := linkprobe.NewResponder(0)
	if !r.StreamOpened(0) {
		t.Fatal("first stream refused")
	}
	probe := func(seq uint32) linkprobe.Message { return linkprobe.Message{Type: linkprobe.TypeProbe, Seq: seq} }
	far := time.Duration(math.MaxInt64)
	for i := range uint32(3) {
		if _, out := r.Receive(far, probe(i+1), 0, true); out != linkprobe.Echoed {
			t.Fatalf("probe %d at the far instant: %v", i+1, out)
		}
	}
	if _, out := r.Receive(far, probe(4), 0, true); out != linkprobe.Dropped {
		t.Fatalf("fourth probe at the same instant: %v, want dropped (credit at most 3 echoes)", out)
	}
}

// Spec §8.5.5 step 2 (review 2 F15): exhaustion is per connection: after the end of the
// stream, Open refuses for good.
func TestProberExhaustedForGood(t *testing.T) {
	p := open(t)
	p.SetNextSeqForTest(math.MaxUint32)
	if _, err := p.Probe(0, 0, true); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Probe(time.Second, 1, true); !errors.Is(err, linkprobe.ErrExhausted) {
		t.Fatalf("after 2^32 - 1: %v", err)
	}
	p.Ended(time.Second)
	if err := p.Open(time.Hour); !errors.Is(err, linkprobe.ErrExhausted) {
		t.Fatalf("reopen after exhaustion: %v, want ErrExhausted", err)
	}
}

// Spec §8.5.5 step 3.6, review 2 F7: PTO = srtt + max(4 rttvar, 1 ms) + max_ack_delay, rounded
// up to a whole millisecond; 25 ms when max_ack_delay is not exposed.
func TestPTO(t *testing.T) {
	for _, c := range []struct{ srtt, rttvar, mad, want time.Duration }{
		{50 * ms, 2 * ms, 25 * ms, 83 * ms},
		{50 * ms, 100 * time.Microsecond, 25 * ms, 76 * ms},
		{50 * ms, 2 * ms, -1, 84 * ms}, // max_ack_delay not exposed: 26 ms, quic-go's
		{50*ms + 300*time.Microsecond, 2 * ms, 25 * ms, 84 * ms},
		{10 * ms, ms, 0, 14 * ms},
	} {
		if got := linkprobe.PTO(c.srtt, c.rttvar, c.mad); got != c.want {
			t.Errorf("PTO(%v, %v, %v) = %v, want %v", c.srtt, c.rttvar, c.mad, got, c.want)
		}
	}
}

// FuzzHost (review 2 F16): any chunking of the bytes read gives the same actions and figure as
// one chunk, and no more than 65 bytes ever stay buffered.
func FuzzHost(f *testing.F) {
	f.Add([]byte{0x0d, 0x02, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0x03, 0x4a, 1, 2}, []byte{1, 5, 9}, true)
	f.Add([]byte{0x05, 0x01, 0, 0, 0, 1, 0x05, 0x01, 0, 0, 0, 2}, []byte{3, 4}, false)
	f.Add([]byte{0x41, 0x00}, []byte{1}, true)
	f.Fuzz(func(t *testing.T, data, cuts []byte, outbound bool) {
		run := func(chunks [][]byte) ([]linkprobe.Action, uint16) {
			h := linkprobe.NewHost(linkprobe.QUIC, linkprobe.Member, 0)
			var acts []linkprobe.Action
			if outbound {
				h.Tick(0, 0, true)
				acts = append(acts, h.Negotiated(0, true, 0, true)...)
			} else {
				h.InboundOpened(0, 7)
			}
			for _, c := range chunks {
				if outbound {
					acts = append(acts, h.OutboundData(ms, c, 1<<40, time.Second)...)
				} else {
					acts = append(acts, h.InboundData(10*time.Second, 7, c, 1<<20, true)...)
				}
				if o, i := h.Buffered(); o > 65 || i > 65 {
					t.Fatalf("buffered %d, %d bytes", o, i)
				}
			}
			return acts, h.Loss(ms)
		}
		var chunks [][]byte
		rest := data
		for _, c := range cuts {
			n := min(int(c)%8, len(rest))
			chunks, rest = append(chunks, rest[:n]), rest[n:]
		}
		chunks = append(chunks, rest)
		a1, l1 := run([][]byte{data})
		a2, l2 := run(chunks)
		if !sameActions(a1, a2) || l1 != l2 {
			t.Fatalf("one chunk %v / %d, chunks %v / %d", a1, l1, a2, l2)
		}
	})
}

// sameActions compares actions, a WriteIn of the same read counting once whatever the chunking:
// one read may hold several PROBEs but answers one, so only the sequence of kinds and bytes up to
// the first divergence of reads is compared through the writes' order and the final reset.
func sameActions(a, b []linkprobe.Action) bool {
	key := func(as []linkprobe.Action) (int, []string) {
		resets, writes := 0, []string(nil)
		for _, x := range as {
			if x.Kind == linkprobe.ResetOut || x.Kind == linkprobe.ResetIn {
				resets++
			}
			if x.Kind == linkprobe.WriteOut {
				writes = append(writes, string(x.Data))
			}
		}
		return resets, writes
	}
	ra, wa := key(a)
	rb, wb := key(b)
	if ra != rb || len(wa) != len(wb) {
		return false
	}
	for i := range wa {
		if wa[i] != wb[i] {
			return false
		}
	}
	return true
}

// Spec §8.5.5 step 2: after seq 2^32 - 1 the prober closes its stream (FIN) and never probes
// the connection again; what it reads afterwards is discarded.
func TestHostExhausted(t *testing.T) {
	h := linkprobe.NewHost(linkprobe.QUIC, linkprobe.Member, 0)
	if a := h.Tick(0, 0, true); len(a) != 1 || a[0].Kind != linkprobe.OpenOut {
		t.Fatalf("first tick: %+v", a)
	}
	if a := h.Negotiated(0, true, 0, true); len(a) != 1 || a[0].Kind != linkprobe.WriteOut {
		t.Fatalf("negotiated: %+v", a)
	}
	h.ProberForTest().SetNextSeqForTest(math.MaxUint32)
	if a := h.Tick(time.Second, 1, true); len(a) != 1 || a[0].Kind != linkprobe.WriteOut {
		t.Fatalf("last seq: %+v", a)
	}
	if a := h.Tick(2*time.Second, 2, true); len(a) != 1 || a[0].Kind != linkprobe.FinOut {
		t.Fatalf("after the last seq: %+v, want FIN", a)
	}
	if a := h.OutboundData(2*time.Second, []byte{0x0d, 0x02, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}, 3, time.Second); a != nil {
		t.Fatalf("read after FIN: %+v, want nothing", a)
	}
	h.OutboundEnded(3 * time.Second)
	for _, at := range []time.Duration{4 * time.Second, time.Hour} {
		if a := h.Tick(at, 4, true); a != nil {
			t.Fatalf("tick at %v after exhaustion: %+v, want nothing ever", at, a)
		}
	}
}
