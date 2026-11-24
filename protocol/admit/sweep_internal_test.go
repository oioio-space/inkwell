// SPDX-License-Identifier: MPL-2.0

package admit

import (
	"net/netip"
	"testing"
)

// Review finding 3 (gates/RF1/CODE-REVIEW.md): under a long churn of distinct origins, one admission each, the
// buckets kept stay bounded by the origins still owed a token (those admitted within about two refill
// periods), quiet gaps included: the sweep drops the full ones.
func TestBucketsBoundedUnderChurn(t *testing.T) {
	const second = Nanos(1_000_000_000)
	cfg := Config{Burst: 16, Refill: 5 * second, PerIP: 8}
	l := New(cfg)
	now := Nanos(0)
	peak := 0
	for i := range 20000 {
		a := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		c, err := l.Open(now, Inbound, IPOrigin(a))
		if err != nil {
			t.Fatalf("opening %d refused: %v", i, err)
		}
		l.Close(now, c)
		peak = max(peak, len(l.buckets))
		now += second
		if i%1000 == 999 {
			now += 60 * second // a quiet minute
		}
	}
	// One admission a second; a debt lives one refill (5 s), a full bucket at most one sweep period more.
	if want := int(2*cfg.Refill/second) + 1; peak > want {
		t.Fatalf("RF-5: %d buckets kept under churn, want %d at most", peak, want)
	}
}
