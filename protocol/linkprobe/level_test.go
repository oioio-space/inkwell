// SPDX-License-Identifier: MPL-2.0

package linkprobe

import (
	"testing"
	"time"
)

// Spec §8.5.5 step 6: from every level, the level of a value is non-decreasing in the value, and
// a second step with the same value changes nothing (so the sampler's cadence does not matter).
func TestStepMonotoneAndIdempotent(t *testing.T) {
	for _, b := range [][2]band{lossBands, rttBands} {
		hi := b[1].up * 2
		for prev := LevelUnknown; prev <= LevelPoor; prev++ {
			last := LevelUnknown
			for x := int64(0); x <= hi; x += max(1, hi/4000) {
				got := step(prev, x, b)
				if got < last {
					t.Fatalf("step(%v, %d) = %v after %v: not monotone", prev, x, got, last)
				}
				if again := step(got, x, b); again != got {
					t.Fatalf("step(step(%v, %d)) = %v, want %v", prev, x, again, got)
				}
				last = got
			}
		}
	}
}

// Spec §8.5.5 step 6: each boundary is exact (rise at up, fall strictly below down, start at mid).
func TestStepBoundaries(t *testing.T) {
	b := lossBands
	cases := []struct {
		prev Level
		x    int64
		want Level
	}{
		{LevelUnknown, 9, LevelGood},
		{LevelUnknown, 10, LevelFair},
		{LevelUnknown, 99, LevelFair},
		{LevelUnknown, 100, LevelPoor},
		{LevelGood, 24, LevelGood},
		{LevelGood, 25, LevelFair},
		{LevelGood, 149, LevelFair},
		{LevelGood, 150, LevelPoor},
		{LevelFair, 5, LevelFair},
		{LevelFair, 4, LevelGood},
		{LevelFair, 149, LevelFair},
		{LevelFair, 150, LevelPoor},
		{LevelPoor, 50, LevelPoor},
		{LevelPoor, 49, LevelFair},
		{LevelPoor, 4, LevelGood},
	}
	for _, c := range cases {
		if got := step(c.prev, c.x, b); got != c.want {
			t.Errorf("step(%v, %d) = %v, want %v", c.prev, c.x, got, c.want)
		}
	}
}

// Spec §8.5.6 step 3 (review 3, F6): the elapsed time of the rate bound is clamped at 0, so an
// instant before the opening (a clock read out of order) cannot wrap the bound open.
func TestSeqBoundClampsBeforeOpening(t *testing.T) {
	r := NewResponder(0)
	if !r.StreamOpened(10 * time.Second) {
		t.Fatal("stream refused")
	}
	if _, o := r.Receive(5*time.Second, Message{Type: TypeProbe, Seq: 1}, 1, true); o != Echoed {
		t.Fatalf("seq 1: %v", o)
	}
	if _, o := r.Receive(5*time.Second, Message{Type: TypeProbe, Seq: 2 + SeqSlack + SeqDropMax}, 2, true); o != Violation {
		t.Fatalf("seq above the drop band before the opening: %v, want violation", o)
	}
}

// windowProber returns a prober whose window holds one sample per rtt, evenly spread over twice
// MinSpan and ending at the returned instant (the newest ECHO), with enough packets sent for the
// figure to be known.
func windowProber(rtts ...time.Duration) (*Prober, time.Duration) {
	p := NewProber()
	step := 2 * MinSpan / time.Duration(len(rtts)-1)
	var at time.Duration
	for i, r := range rtts {
		at = time.Duration(i) * step
		p.samples = append(p.samples, sample{at: at, rtt: r, sent: uint64(i) * 2 * MinPackets, received: uint64(i) * 2 * MinPackets})
	}
	return p, at + rtts[len(rtts)-1]
}

// Spec Annex A.1 « Sens de rttMs et rttVarMs »: the link's round trip is the lower median of the
// window's samples' rtt (element of rank ⌊(n−1)/2⌋), its variance the lower median of the
// absolute deviations from it, its age the age of the newest sample; odd n, even n, ties.
func TestProberRTTLowMedian(t *testing.T) {
	ms := time.Millisecond
	cases := []struct {
		name     string
		rtts     []time.Duration
		rtt, dev time.Duration
	}{
		{"odd", []time.Duration{50 * ms, 90 * ms, 60 * ms}, 60 * ms, 10 * ms},
		{"even takes the lower of the middle two", []time.Duration{40 * ms, 80 * ms, 60 * ms, 100 * ms}, 60 * ms, 20 * ms},
		{"ties", []time.Duration{60 * ms, 60 * ms, 60 * ms, 60 * ms}, 60 * ms, 0},
		{"two samples", []time.Duration{70 * ms, 50 * ms}, 50 * ms, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, end := windowProber(tc.rtts...)
			now := end + 3*time.Second
			rtt, dev, age, ok := p.RTT(now)
			if !ok {
				t.Fatalf("RTT(%v) unknown with a known figure (loss %d)", now, p.Loss(now))
			}
			if rtt != tc.rtt || dev != tc.dev {
				t.Errorf("RTT = %v ± %v, want %v ± %v", rtt, dev, tc.rtt, tc.dev)
			}
			if age != 3*time.Second {
				t.Errorf("age = %v, want 3s (since the newest ECHO)", age)
			}
			if got := medianRTT(p.samples); got != rtt {
				t.Errorf("RTT %v differs from the median that drives the level, %v", rtt, got)
			}
		})
	}
}

// Spec Annex A.1 « Sens de rttMs et rttVarMs »: while the figure is unknown there is no probe
// round trip (the link keeps the QUIC stack's).
func TestProberRTTUnknownFigure(t *testing.T) {
	p, end := windowProber(50*time.Millisecond, 60*time.Millisecond)
	if _, _, _, ok := p.RTT(end + 2*Window); ok {
		t.Error("RTT known after the window went by")
	}
	if _, _, _, ok := NewProber().RTT(0); ok {
		t.Error("RTT known with no sample")
	}
	h := NewHost(QUIC, Member, 0)
	if _, _, _, ok := h.RTT(0); ok {
		t.Error("Host.RTT known with no sample")
	}
}
