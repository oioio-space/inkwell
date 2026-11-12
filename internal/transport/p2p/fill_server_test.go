//go:build server

package p2p_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

const serverBuild = true

// TestNetsimFillRate: the filler sends one message every 1/120 s on the session's connection in each
// direction, three times as many during the first 40 s, and the peer discards it.
func TestNetsimFillRate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nn := newNatNet(1)
		a, _ := directPair(t, nn, p2p.Options{}, p2p.Options{})
		rate := func(from, to int) float64 {
			time.Sleep(time.Duration(from) * time.Second)
			s0, _ := nn.counts(a.end)
			time.Sleep(time.Duration(to-from) * time.Second)
			s1, _ := nn.counts(a.end)
			return float64(s1-s0) / float64(to-from)
		}
		time.Sleep(5 * time.Second)
		boosted := rate(0, 20)
		time.Sleep(20 * time.Second)
		steady := rate(5, 25)
		// The packets this end sends include the ACKs of the peer's filler: more than the 120 a second
		// of messages.
		if steady < 120 || steady > 260 {
			t.Errorf("rate afterwards = %.0f packets/s, want 120 to 260", steady)
		}
		if boosted < 2.5*steady {
			t.Errorf("rate during the first 40 s = %.0f packets/s, want at least 2.5 times %.0f", boosted, steady)
		}
	})
}
