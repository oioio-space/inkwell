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
