// SPDX-License-Identifier: MPL-2.0

package linkprobe

import (
	"slices"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// Level is the quality of a link a person is shown (§8.5.5 step 6), the `quality` of a Links
// notification (Annex A.1).
type Level uint8

// The levels, in the order of Annex A.1.
const (
	LevelUnknown Level = iota // nothing measured: the figure is unknown
	LevelGood
	LevelFair
	LevelPoor
)

// String returns the level's name, as the corpus writes it.
func (l Level) String() string {
	switch l {
	case LevelGood:
		return "good"
	case LevelFair:
		return "fair"
	case LevelPoor:
		return "poor"
	default:
		return "unknown"
	}
}

// band is one boundary between two levels: a lower level rises at up or above, an upper level
// falls back below down, and the first level after unknown is placed against mid.
type band struct{ down, mid, up int64 }

var (
	lossBands = [2]band{
		{transport.LinkProbeLossFairDown, transport.LinkProbeLossFair, transport.LinkProbeLossFairUp},
		{transport.LinkProbeLossPoorDown, transport.LinkProbeLossPoor, transport.LinkProbeLossPoorUp},
	}
	rttBands = [2]band{
		{int64(transport.LinkProbeRTTFairDown), int64(transport.LinkProbeRTTFair), int64(transport.LinkProbeRTTFairUp)},
		{int64(transport.LinkProbeRTTPoorDown), int64(transport.LinkProbeRTTPoor), int64(transport.LinkProbeRTTPoorUp)},
	}
)

// step is the level after prev for a value x (§8.5.5 step 6). It is idempotent: a second step
// with the same x changes nothing, so how often it runs does not matter.
func step(prev Level, x int64, b [2]band) Level {
	switch {
	case prev == LevelUnknown && x >= b[1].mid:
		return LevelPoor
	case prev == LevelUnknown && x >= b[0].mid:
		return LevelFair
	case prev == LevelUnknown:
		return LevelGood
	case prev != LevelPoor && x >= b[1].up:
		return LevelPoor
	case prev == LevelGood && x >= b[0].up:
		return LevelFair
	case prev != LevelGood && x < b[0].down:
		return LevelGood
	case prev == LevelPoor && x < b[1].down:
		return LevelFair
	default:
		return prev
	}
}

// advance steps the levels with the figure at now (§8.5.5 step 6). It runs at every accepted
// sample, so the hysteresis follows each new figure whatever the caller's cadence. After the figure
// has been unknown for more than Window since the last known one, the hysteresis starts afresh.
func (p *Prober) advance(now time.Duration) {
	ws, ok := p.window(now)
	if !ok {
		return
	}
	if p.lossLevel != LevelUnknown && now-p.knownAt > Window {
		p.lossLevel, p.rttLevel = LevelUnknown, LevelUnknown
	}
	p.knownAt = now
	p.lossLevel = step(p.lossLevel, int64(p.Loss(now)), lossBands)
	p.rttLevel = step(p.rttLevel, int64(medianRTT(ws)), rttBands)
}

// Level returns the level at now (§8.5.5 step 6): the worse of the loss level and the round-trip
// level of the window, as the last new figure set them, or unknown while the figure is unknown.
func (p *Prober) Level(now time.Duration) Level {
	if _, ok := p.window(now); !ok {
		return LevelUnknown
	}
	return max(p.lossLevel, p.rttLevel)
}

// medianRTT returns the lower median of the samples' round trips.
func medianRTT(ws []sample) time.Duration {
	var buf [MaxSamples]time.Duration
	rs := buf[:0]
	for _, s := range ws {
		rs = append(rs, s.rtt)
	}
	return lowMedian(rs)
}

// lowMedian returns the element of rank ⌊(n−1)/2⌋ of ds, sorted ascending in place; ds is not empty.
func lowMedian(ds []time.Duration) time.Duration {
	slices.Sort(ds)
	return ds[(len(ds)-1)/2]
}

// RTT returns the round trip of the link at now (Annex A.1 rttMs, rttVarMs, rttAgeMs): the lower
// median of the window's samples' rtt (the value that drives the round-trip level), the lower
// median of their absolute deviations from it, and the time since the newest ECHO. ok false means
// the figure is unknown (§8.5.5 step 5), and the caller keeps another source.
func (p *Prober) RTT(now time.Duration) (rtt, dev, age time.Duration, ok bool) {
	ws, ok := p.window(now)
	if !ok {
		return 0, 0, 0, false
	}
	rtt = medianRTT(ws)
	var buf [MaxSamples]time.Duration
	ds := buf[:0]
	for _, s := range ws {
		ds = append(ds, max(s.rtt-rtt, rtt-s.rtt))
	}
	last := ws[len(ws)-1]
	return rtt, lowMedian(ds), max(now-(last.at+last.rtt), 0), true
}

// StepLoss is one step of the loss level (§8.5.5 step 6): the level after from for a figure in
// permille.
func StepLoss(from Level, permille int64) Level { return step(from, permille, lossBands) }

// StepRTT is one step of the round-trip level (§8.5.5 step 6).
func StepRTT(from Level, rtt time.Duration) Level { return step(from, int64(rtt), rttBands) }
