package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

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

// Plan §7, lossPct: lost / sent packets over the last 30 s, null under 50 packets.
func TestLinkLossWindowNullUnderMinimum(t *testing.T) {
	var w lossWindow
	w.observe(t0, 0, 0)
	w.observe(t0.Add(time.Second), 49, 10)
	if _, ok := w.pct(t0.Add(time.Second)); ok {
		t.Error("loss reported under 50 packets, want none")
	}
	w.observe(t0.Add(2*time.Second), 100, 10)
	got, ok := w.pct(t0.Add(2 * time.Second))
	if !ok || got != 10 {
		t.Errorf("loss = %v (ok %v), want 10%%", got, ok)
	}
}

// Plan §7, lossPct: only the last 30 s count; older loss ages out.
func TestLinkLossWindowAgesOut(t *testing.T) {
	var w lossWindow
	w.observe(t0, 0, 0)
	w.observe(t0.Add(time.Second), 100, 50) // a bad second
	for i := 2; i <= 40; i++ {
		w.observe(t0.Add(time.Duration(i)*time.Second), uint64(100+i*10), 50)
	}
	got, ok := w.pct(t0.Add(40 * time.Second))
	if !ok || got != 0 {
		t.Errorf("loss 39 s after the bad second = %v (ok %v), want 0", got, ok)
	}
}

// Plan §7, lossPct: the counters are not monotonic (lost packets may be
// received later; a new connection restarts them), so deltas clamp at 0.
func TestLinkLossWindowClampsDecreases(t *testing.T) {
	var w lossWindow
	w.observe(t0, 0, 0)
	w.observe(t0.Add(time.Second), 100, 20)
	w.observe(t0.Add(2*time.Second), 150, 15) // lost went down: late arrivals
	w.observe(t0.Add(3*time.Second), 10, 0)   // new connection: counters restart
	w.observe(t0.Add(4*time.Second), 60, 0)
	got, ok := w.pct(t0.Add(4 * time.Second))
	if !ok {
		t.Fatal("no loss figure with 200 packets")
	}
	if got < 0 || got > 100 {
		t.Errorf("loss = %v, want within [0, 100]", got)
	}
	if want := 20.0 / 200 * 100; got != want {
		t.Errorf("loss = %v, want %v (steps clamped at 0)", got, want)
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

// Decision Q4: plain grades and their thresholds.
func TestLinkQualityGrades(t *testing.T) {
	pct := func(v float64) *float64 { return &v }
	ms := time.Millisecond
	tests := []struct {
		name string
		l    transport.Link
		want transport.Quality
	}{
		{"nothing measured", transport.Link{}, transport.QualityUnknown},
		{"fast and clean", transport.Link{RTT: 30 * ms, LossPct: pct(0)}, transport.QualityGood},
		{"rtt at fair", transport.Link{RTT: transport.FairRTT}, transport.QualityFair},
		{"rtt just under fair", transport.Link{RTT: transport.FairRTT - ms}, transport.QualityGood},
		{"rtt at poor", transport.Link{RTT: transport.PoorRTT}, transport.QualityPoor},
		{"loss at fair", transport.Link{RTT: 20 * ms, LossPct: pct(transport.FairLoss)}, transport.QualityFair},
		{"loss at poor", transport.Link{RTT: 20 * ms, LossPct: pct(transport.PoorLoss)}, transport.QualityPoor},
		{"loss alone grades", transport.Link{LossPct: pct(12)}, transport.QualityPoor},
		{"one recent outage", transport.Link{RTT: 20 * ms, Downs10m: 1}, transport.QualityFair},
		{"flapping", transport.Link{RTT: 20 * ms, Downs10m: 3}, transport.QualityPoor},
		{"worst figure wins", transport.Link{RTT: 500 * ms, LossPct: pct(0)}, transport.QualityPoor},
	}
	for _, tt := range tests {
		if got := tt.l.Quality(); got != tt.want {
			t.Errorf("%s: Quality() = %v, want %v", tt.name, got, tt.want)
		}
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

// A resync does not lower the grade of a link.
func TestLinkQualityIgnoresResyncs(t *testing.T) {
	l := transport.Link{RTT: time.Millisecond, Resyncs10m: 50, Resyncs: 50}
	if got := l.Quality(); got != transport.QualityGood {
		t.Errorf("Quality with 50 resyncs = %v, want %v", got, transport.QualityGood)
	}
}

// The loss window starts again when the connection or the scope it reads
// changes: counters of another connection do not continue the old ones.
func TestLinkLossWindowResetsOnNewConnection(t *testing.T) {
	n := &Node{stats: make(map[peer.ID]*peerLink)}
	pl := new(peerLink)
	sample := func(conn string, scope transport.LossScope, sent uint64) linkSample {
		return linkSample{kind: transport.LinkDirect, quic: true, connID: conn, scope: scope, sent: sent}
	}
	step := func(i int, s linkSample) {
		n.applyLinkLocked("p", pl, t0.Add(time.Duration(i)*time.Second), s)
	}
	step(0, sample("c1", transport.LossPath, 100))
	step(1, sample("c1", transport.LossPath, 200))
	if got := len(pl.loss.steps); got != 1 {
		t.Fatalf("steps on one connection = %d, want 1", got)
	}
	step(2, sample("c2", transport.LossPath, 10)) // a new connection
	if got := len(pl.loss.steps); got != 0 {
		t.Errorf("steps after a new connection = %d, want 0", got)
	}
	step(3, sample("c2", transport.LossPath, 20))
	step(4, sample("c2", transport.LossFirstHop, 30)) // same connection, new scope
	if got := len(pl.loss.steps); got != 0 {
		t.Errorf("steps after a new scope = %d, want 0", got)
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
