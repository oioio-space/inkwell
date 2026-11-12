package p2p

import (
	"slices"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// The per-link estimators of the metrics contract (plan W33, §7). They hold no
// lock and take the time as an argument: the caller serialises them, and the
// tests drive them with a fake clock.

// rttEstimator smooths round-trip samples with an EWMA of weight 1/4, quick
// enough to show a change of delay within a few samples.
type rttEstimator struct {
	srtt, dev time.Duration
	at        time.Time
	ok        bool
}

// sample folds in a measured round trip.
func (e *rttEstimator) sample(now time.Time, r time.Duration) {
	if !e.ok {
		e.srtt, e.dev, e.at, e.ok = r, r/2, now, true
		return
	}
	e.dev += (abs(e.srtt-r) - e.dev) / 4
	e.srtt += (r - e.srtt) / 4
	e.at = now
}

// set records the smoothed figures the transport already keeps (QUIC).
func (e *rttEstimator) set(now time.Time, srtt, dev time.Duration) {
	e.srtt, e.dev, e.at, e.ok = srtt, dev, now, true
}

// get returns the smoothed RTT, its deviation and the age of the last sample;
// all zero before the first sample.
func (e *rttEstimator) get(now time.Time) (rtt, dev, age time.Duration) {
	if !e.ok {
		return 0, 0, 0
	}
	return e.srtt, e.dev, now.Sub(e.at)
}

func abs(d time.Duration) time.Duration { return max(d, -d) }

// stability counts the outages of a peer from its PeerUp and PeerDown, and
// apart from them the resyncs: a PeerDown the transport announced itself, over
// a connection that survived (spec rule L).
type stability struct {
	upSince     time.Time
	downAt      time.Time   // when the current outage began; zero while up
	seen        time.Time   // the last up, down or resync
	downs       []time.Time // the starts of outages, oldest first
	total       int
	last        time.Duration
	resyncs     []time.Time // the resyncs, oldest first
	resyncTotal int
}

// up records a PeerUp: the session begins and the outage, if any, ends.
func (s *stability) up(now time.Time) {
	if !s.downAt.IsZero() {
		s.last = now.Sub(s.downAt)
		s.downAt = time.Time{}
	}
	s.upSince, s.seen = now, now
}

// down records a PeerDown caused by a lost connection.
func (s *stability) down(now time.Time) {
	s.downAt, s.seen = now, now
	s.downs = pushWithin(s.downs, now, transport.DownsWindow)
	s.total++
	s.upSince = time.Time{}
}

// resync records a PeerDown that restarts the session: no outage.
func (s *stability) resync(now time.Time) {
	s.seen = now
	s.resyncs = pushWithin(s.resyncs, now, transport.DownsWindow)
	s.resyncTotal++
	s.upSince = time.Time{}
}

// pushWithin appends now to the times in at, after dropping those older than
// window.
func pushWithin(at []time.Time, now time.Time, window time.Duration) []time.Time {
	i := slices.IndexFunc(at, func(t time.Time) bool { return now.Sub(t) <= window })
	if i < 0 {
		i = len(at)
	}
	return append(slices.Delete(at, 0, i), now)
}

// within counts the times in at that are in the last [transport.DownsWindow].
func within(at []time.Time, now time.Time) int {
	n := 0
	for _, t := range at {
		if now.Sub(t) <= transport.DownsWindow {
			n++
		}
	}
	return n
}

// downsWithin counts the outages that began in the last [transport.DownsWindow].
func (s *stability) downsWithin(now time.Time) int {
	return within(s.downs, now)
}

// resyncsWithin counts the resyncs of the last [transport.DownsWindow].
func (s *stability) resyncsWithin(now time.Time) int {
	return within(s.resyncs, now)
}

// pathTrack follows the kind of path, counting relayed to direct changes and
// the opposite.
type pathTrack struct {
	kind                transport.LinkKind
	since               time.Time
	upgrades, fallbacks int
}

// observe records the kind of path seen at now.
func (p *pathTrack) observe(now time.Time, kind transport.LinkKind) {
	switch {
	case p.kind == 0:
	case p.kind == kind:
		return
	case p.kind == transport.LinkRelayed:
		p.upgrades++
	case kind == transport.LinkRelayed:
		p.fallbacks++
	}
	p.kind, p.since = kind, now
}
