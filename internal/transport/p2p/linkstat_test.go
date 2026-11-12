package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

var t0 = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// Plan §7, rttMs/rttVarMs/rttAgeMs: our own EWMA with alpha = 1/4, so that a
// change of delay shows within a few samples (the peerstore's 0.1 is too slow).
func TestLinkRTTEstimatorFollowsChange(t *testing.T) {
	var e rttEstimator
	e.sample(t0, 40*time.Millisecond)
	if got, _, _ := e.get(t0); got != 40*time.Millisecond {
		t.Fatalf("first sample: rtt = %v, want 40ms", got)
	}
	for i := range 8 {
		e.sample(t0.Add(time.Duration(i+1)*10*time.Second), 200*time.Millisecond)
	}
	rtt, dev, age := e.get(t0.Add(100 * time.Second))
	if rtt < 180*time.Millisecond || rtt > 200*time.Millisecond {
		t.Errorf("rtt after 8 samples of 200ms = %v, want within 20ms of 200ms", rtt)
	}
	if dev <= 0 {
		t.Errorf("deviation = %v, want > 0 after a jump", dev)
	}
	if want := 20 * time.Second; age != want {
		t.Errorf("age = %v, want %v", age, want)
	}
}

// Plan §7, rtt*: a path with no sample reports zero, not a made-up figure.
func TestLinkRTTEstimatorEmpty(t *testing.T) {
	var e rttEstimator
	if rtt, dev, age := e.get(t0); rtt != 0 || dev != 0 || age != 0 {
		t.Errorf("empty estimator = %v %v %v, want zeros", rtt, dev, age)
	}
}

// Plan §7, upSince/downs10m/downsTotal/lastDownMs: stability from PeerUp/PeerDown.
func TestLinkStability(t *testing.T) {
	var s stability
	s.up(t0)
	s.down(t0.Add(time.Minute))
	s.up(t0.Add(time.Minute + 5*time.Second))
	s.down(t0.Add(5 * time.Minute))
	s.up(t0.Add(5*time.Minute + 8*time.Second))

	at := t0.Add(6 * time.Minute)
	if got, want := s.upSince, t0.Add(5*time.Minute+8*time.Second); !got.Equal(want) {
		t.Errorf("upSince = %v, want %v", got, want)
	}
	if got := s.downsWithin(at); got != 2 {
		t.Errorf("downs10m = %d, want 2", got)
	}
	if s.total != 2 {
		t.Errorf("downsTotal = %d, want 2", s.total)
	}
	if want := 8 * time.Second; s.last != want {
		t.Errorf("lastDown = %v, want %v", s.last, want)
	}
	if got := s.downsWithin(t0.Add(20 * time.Minute)); got != 0 {
		t.Errorf("downs10m after 20 min = %d, want 0 (aged out) ", got)
	}
}

// Plan §7, path/pathSince/upgrades/fallbacks: relayed to direct and back.
func TestLinkPathTracking(t *testing.T) {
	var p pathTrack
	p.observe(t0, transport.LinkRelayed)
	p.observe(t0.Add(time.Second), transport.LinkRelayed)
	if !p.since.Equal(t0) {
		t.Errorf("since = %v, want unchanged %v", p.since, t0)
	}
	p.observe(t0.Add(5*time.Second), transport.LinkDirect)
	p.observe(t0.Add(9*time.Second), transport.LinkLAN) // direct to direct: neither
	p.observe(t0.Add(10*time.Second), transport.LinkRelayed)
	if p.upgrades != 1 || p.fallbacks != 1 {
		t.Errorf("upgrades/fallbacks = %d/%d, want 1/1", p.upgrades, p.fallbacks)
	}
	if want := t0.Add(10 * time.Second); !p.since.Equal(want) {
		t.Errorf("since = %v, want %v", p.since, want)
	}
}

// Spec rule L: a resync restarts the session but is no outage.
func TestLinkResyncIsNotAnOutage(t *testing.T) {
	var s stability
	s.up(t0)
	s.resync(t0.Add(time.Minute))
	if !s.upSince.IsZero() {
		t.Errorf("upSince after a resync = %v, want zero until the peer is up again", s.upSince)
	}
	s.up(t0.Add(time.Minute + time.Second))

	at := t0.Add(2 * time.Minute)
	if got := s.downsWithin(at); got != 0 || s.total != 0 || s.last != 0 {
		t.Errorf("downs10m/total/last after a resync = %d/%d/%v, want 0/0/0", got, s.total, s.last)
	}
	if got := s.resyncsWithin(at); got != 1 || s.resyncTotal != 1 {
		t.Errorf("resyncs10m/total = %d/%d, want 1/1", got, s.resyncTotal)
	}
	if got := s.resyncsWithin(t0.Add(20 * time.Minute)); got != 0 {
		t.Errorf("resyncs10m after 20 min = %d, want 0 (aged out)", got)
	}
}

// Spec rule L: only a cancellation that came before the write returned makes
// a failure the caller's doing.
func TestRuleLWatchCancel(t *testing.T) {
	t.Run("canceled during the write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		done := watchCancel(ctx)
		cancel()
		if !done() {
			t.Error("cancel before the write returned: canceled = false, want true")
		}
	})
	t.Run("canceled after the write failed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		done := watchCancel(ctx)
		failed := done() // the write returned with a timeout
		cancel()
		if failed {
			t.Error("write failed before the cancel: canceled = true, want false")
		}
	})
	t.Run("never canceled", func(t *testing.T) {
		if watchCancel(t.Context())() {
			t.Error("canceled = true without any cancel")
		}
	})
}
