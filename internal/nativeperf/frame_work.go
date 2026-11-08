package nativeperf

import (
	"errors"
	"math"
	"slices"
)

// Clock reports observations, not an assumption about a browser's precision.
type Clock struct {
	RefreshHz    float64          `json:"refreshHz,omitempty"`
	Precise      *PreciseClock    `json:"precise,omitempty"`
	ResolutionMS float64          `json:"resolutionMs"`
	DeltasMS     []float64        `json:"deltasMs"`
	Reads        int              `json:"reads"`
	Candidates   []ClockCandidate `json:"candidates,omitempty"`
	Rule         string           `json:"roundingRule"`
}

// ClockCandidate is one browser timing source probed during clock
// calibration: its observed resolution and read spans, or the error that ruled
// it out.
type ClockCandidate struct {
	Source       string    `json:"source"`
	ResolutionMS float64   `json:"resolutionMs,omitempty"`
	ReadSpansMS  []float64 `json:"readSpansMs,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// PreciseClock is the native elapsed-time source; browser clocks remain raw
// diagnostics and define the uncertainty of cadence/Event Timing correlation.
type PreciseClock struct {
	BatchSize    int       `json:"batchSize,omitempty"`
	Source       string    `json:"source"`
	ResolutionMS float64   `json:"resolutionMs"`
	ReadSpansMS  []float64 `json:"readSpansMs"`
}

// FineSpan is a start/end interval in milliseconds on the sample timeline.
type FineSpan struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func validPreciseClock(c *PreciseClock) bool {
	if c != nil && c.Source == "browser-bounded" {
		if !finite(c.ResolutionMS) || c.ResolutionMS <= 0 || c.ResolutionMS > 1 || c.BatchSize < 1000 || len(c.ReadSpansMS) != 4 {
			return false
		}
		for _, d := range c.ReadSpansMS {
			if !finite(d) || d <= 0 || d > 1 {
				return false
			}
		}
		return true
	}
	if c == nil || c.Source != "native-monotonic" || !finite(c.ResolutionMS) || c.ResolutionMS <= 0 || c.ResolutionMS > .01 || len(c.ReadSpansMS) < 4 {
		return false
	}
	for _, d := range c.ReadSpansMS {
		if !finite(d) || d > 4 {
			return false
		}
	}
	return true
}

// measuredHandlerWork returns the raw and upper-bound handler work under the
// clock's rules: the bounded browser clock, the legacy raw diagnostics, or the
// native monotonic clock.
func measuredHandlerWork(handlers []Handler, c Clock) (raw, upper float64, err error) {
	if c.Precise != nil && c.Precise.Source == "browser-bounded" {
		return boundedBrowserWork(handlers, c)
	}
	if c.Precise == nil {
		// Retain legacy raw diagnostics; addRun refuses to certify this clock.
		raw, upper = handlerWork(handlers, c.ResolutionMS)
		return raw, upper, nil
	}
	return nativeWork(handlers, c)
}

// bracketCoversHandlers reports whether every handler lies inside the
// keystroke bracket h, give or take one browser quantum.
func bracketCoversHandlers(handlers []Handler, h Handler, quantum float64) bool {
	for _, inner := range handlers {
		if inner.Start < h.Start-quantum || inner.End > h.End+quantum {
			return false
		}
	}
	return true
}

// boundedBrowserWork measures handler work from the single keystroke-task
// bracket under a bounded browser clock.
func boundedBrowserWork(handlers []Handler, c Clock) (raw, upper float64, err error) {
	if !validPreciseClock(c.Precise) || c.Precise.ResolutionMS != c.ResolutionMS {
		return 0, 0, errors.New("invalid bounded browser handler clock")
	}
	outer := []Handler{}
	for _, h := range handlers {
		if h.Event != "keystroke-task" {
			continue
		}
		if !finite(h.Start) || !finite(h.End) || h.End < h.Start || h.Fine == nil || !finite(h.Fine.Start) || !finite(h.Fine.End) || h.Fine.End < h.Fine.Start || h.Fine.Start < h.Start-c.ResolutionMS || h.Fine.End > h.End+c.ResolutionMS {
			return 0, 0, errors.New("missing or invalid browser handler boundaries")
		}
		if !bracketCoversHandlers(handlers, h, c.ResolutionMS) {
			return 0, 0, errors.New("keystroke bracket does not cover input dispatch")
		}
		outer = append(outer, h)
	}
	if len(outer) != 1 {
		return 0, 0, errors.New("missing unique bounded browser keystroke bracket")
	}
	// Keep the raw coarse interval AND its full uncertainty. Never subtract
	// NULL overhead, transport, queue wait, or round away a real 5ms handler.
	raw, upper = handlerWork(outer, c.ResolutionMS)
	return raw, upper, nil
}

// hasKeystrokeTask reports whether any handler is a keystroke-task bracket.
func hasKeystrokeTask(handlers []Handler) bool {
	return slices.ContainsFunc(handlers, func(h Handler) bool { return h.Event == "keystroke-task" })
}

// nativeWork measures handler work from the native clock's fine boundaries,
// cross-checked against the browser's coarse ones.
func nativeWork(handlers []Handler, c Clock) (raw, upper float64, err error) {
	if !validPreciseClock(c.Precise) {
		return 0, 0, errors.New("invalid native handler clock")
	}
	precise := make([]Handler, 0, len(handlers))
	outer := hasKeystrokeTask(handlers)
	transport := 0.0
	for _, d := range c.Precise.ReadSpansMS {
		transport = math.Max(transport, d)
	}
	if transport+c.ResolutionMS > 4 {
		return 0, 0, errors.New("native clock preflight transport upper bound exceeds 4ms")
	}
	for _, h := range handlers {
		if outer && h.Event != "keystroke-task" {
			continue
		}
		if h.Fine == nil || !finite(h.Fine.Start) || !finite(h.Fine.End) || h.Fine.End < h.Fine.Start {
			return 0, 0, errors.New("missing native handler boundaries")
		}
		// Two views of the same interval must agree within the browser quantum.
		// A long clock round trip or stale fine timestamp cannot certify a pass.
		browser, fine := h.End-h.Start, h.Fine.End-h.Fine.Start
		if fine > browser+c.ResolutionMS+c.Precise.ResolutionMS || browser-fine > c.ResolutionMS+transport+c.Precise.ResolutionMS {
			return 0, 0, errors.New("native handler clock disagrees with browser boundaries")
		}
		precise = append(precise, Handler{Start: h.Fine.Start, End: h.Fine.End})
		if outer && !bracketCoversHandlers(handlers, h, c.ResolutionMS) {
			return 0, 0, errors.New("keystroke bracket does not cover input dispatch")
		}
	}
	raw, upper = handlerWork(precise, c.Precise.ResolutionMS)
	return raw, upper, nil
}

// WorkFrame brackets a rendering opportunity with an rAF and a subsequent task.
// It measures a conservative work window, never physical presentation latency.
type WorkFrame struct {
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Timestamp float64 `json:"timestamp"`
}

func validClock(c Clock) bool {
	if !finite(c.ResolutionMS) || c.ResolutionMS <= 0 || len(c.DeltasMS) < 3 || c.Rule != "upper:+quantum-per-disjoint-interval" {
		return false
	}
	minimum := math.Inf(1)
	for _, d := range c.DeltasMS {
		if !finite(d) || d <= 0 {
			return false
		}
		minimum = math.Min(minimum, d)
	}
	return math.Abs(minimum-c.ResolutionMS) < .000001
}

func handlerWork(handlers []Handler, quantum float64) (raw, upper float64) {
	ordered := slices.Clone(handlers)
	slices.SortFunc(ordered, func(a, b Handler) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	if len(ordered) == 0 {
		return 0, 0
	}
	start, end := ordered[0].Start, ordered[0].End
	for _, h := range ordered[1:] {
		if h.Start <= end {
			end = math.Max(end, h.End)
		} else {
			raw += end - start
			upper += end - start + quantum
			start, end = h.Start, h.End
		}
	}
	return raw + end - start, upper + end - start + quantum
}

func frameWork(s Sample, c Clock) (raw, upper, interval float64, dropped bool, err error) {
	raw, upper, interval, missed, err := frameEvidence(s, c)
	return raw, upper, interval, len(missed) > 0, err
}

// cadencePeriod returns the median interval of the six-point cadence
// baseline, or an error when the baseline is not strictly increasing.
func cadencePeriod(baseline []float64) (float64, error) {
	periods := []float64{}
	for i, t := range baseline {
		if !finite(t) || (i > 0 && t <= baseline[i-1]) {
			return 0, errors.New("invalid native cadence calibration")
		}
		if i > 0 {
			periods = append(periods, t-baseline[i-1])
		}
	}
	slices.Sort(periods)
	return periods[len(periods)/2], nil
}

// checkCadence rejects a baseline that is stale, post-input, or slower than
// the nominal monitor refresh.
func checkCadence(s Sample, c Clock, period float64) error {
	last := s.Baseline[len(s.Baseline)-1]
	provenPreInput := s.PreInputHeartbeatCount > 0 && s.PreInputHeartbeatCount <= len(s.Heartbeat) && s.Heartbeat[s.PreInputHeartbeatCount-1] <= s.KeyDownAt+c.ResolutionMS
	if (last >= s.KeyDownAt && !(provenPreInput && last <= s.KeyDownAt+c.ResolutionMS)) || s.KeyDownAt-last > 2*period {
		return errors.New("stale or post-input cadence baseline")
	}
	if c.RefreshHz > 0 && period > 1000/c.RefreshHz+c.ResolutionMS {
		return errors.New("observed cadence slower than nominal GdkMonitor refresh")
	}
	return nil
}

// frameSpans computes the per-frame work maxima (the first frame carries the
// handler work h and its upper bound hu), the longest frame interval, and the
// frame intervals that exceed the cadence period.
func frameSpans(s Sample, c Clock, period, h, hu float64) (raw, upper, interval float64, missed []FineSpan, err error) {
	for i, f := range s.Frames {
		if !finite(f.Start) || !finite(f.End) || !finite(f.Timestamp) || f.End < f.Start || f.Start < s.KeyDownAt || len(s.RAF) != 2 {
			return 0, 0, 0, nil, errors.New("invalid per-frame work boundaries")
		}
		w, u := f.End-f.Start, f.End-f.Start+c.ResolutionMS
		if i == 0 {
			w += h
			u += hu
		}
		raw = math.Max(raw, w)
		upper = math.Max(upper, u)
		if i > 0 {
			delta := f.Timestamp - s.Frames[i-1].Timestamp
			if delta <= 0 {
				return 0, 0, 0, nil, errors.New("invalid frame interval")
			}
			interval = math.Max(interval, delta)
			if delta > period+2*c.ResolutionMS {
				missed = append(missed, FineSpan{Start: s.Frames[i-1].Timestamp, End: f.Timestamp})
			}
		}
	}
	return raw, upper, interval, missed, nil
}

// heartbeatSpans extends interval and missed with the heartbeat intervals that
// begin before the end of the measured window.
func heartbeatSpans(s Sample, c Clock, period, interval float64, missed []FineSpan) (float64, []FineSpan) {
	for i := 1; i < len(s.Heartbeat); i++ {
		before, after := s.Heartbeat[i-1], s.Heartbeat[i]
		end := s.Frames[len(s.Frames)-1].End
		if s.SettledAt > 0 {
			end = s.SettledAt
		}
		if before <= end {
			interval = math.Max(interval, after-before)
			if after-before > period+2*c.ResolutionMS {
				missed = append(missed, FineSpan{Start: before, End: after})
			}
		}
	}
	return interval, missed
}

// busyWork returns the longest task-probe span, measured from the key's start.
func busyWork(s Sample, c Clock) (raw, upper float64) {
	// A task-probe span runs from one probe task to the next: the probe's
	// 8 ms timer yield plus whatever kept the main thread before the next
	// one. KeyDownAt is the start of the keydown listener, the earliest
	// instant of the key's work, so the part of a span before it is the
	// probe's idle wait or an earlier, unrelated task. Everything after the
	// key stays whole, idle or not, with its +quantum.
	for _, span := range s.Busy {
		if span.End >= s.KeyDownAt {
			from := math.Max(span.Start, s.KeyDownAt)
			raw = math.Max(raw, span.End-from)
			upper = math.Max(upper, span.End-from+c.ResolutionMS)
		}
	}
	return raw, upper
}

// workSpans collects every task, frame and keystroke-bracket interval.
func workSpans(s Sample) []FineSpan {
	all := append([]FineSpan{}, s.Tasks...)
	for _, f := range s.Frames {
		all = append(all, FineSpan{Start: f.Start, End: f.End})
	}
	for _, h := range s.Handlers {
		if h.Event == "keystroke-task" {
			all = append(all, FineSpan{Start: h.Start, End: h.End})
		}
	}
	return all
}

// fencedWork sums disjoint actual callbacks/rendering/dispatch intervals within
// each frame, rather than only checking the longest individual task. Empty
// space between callbacks is idle time and is never added to this sum.
func fencedWork(s Sample, c Clock) (raw, upper float64, err error) {
	all := workSpans(s)
	from := s.KeyDownAt
	ends := []float64{}
	for _, f := range s.Frames {
		ends = append(ends, f.End)
	}
	ends = append(ends, s.SettledAt)
	for _, to := range ends {
		if to < from {
			return 0, 0, errors.New("nonmonotonic render/task fence")
		}
		parts := []Handler{}
		for _, span := range all {
			a, b := math.Max(from, span.Start), math.Min(to, span.End)
			if b >= a && span.End >= from && span.Start <= to {
				parts = append(parts, Handler{Start: a, End: b})
			}
		}
		work, bound := handlerWork(parts, c.ResolutionMS)
		raw = math.Max(raw, work)
		upper = math.Max(upper, bound)
		from = to
	}
	return raw, upper, nil
}

// frameEvidence is frameWork with each missed refresh interval kept as a span:
// a frame or heartbeat interval, or the settle tail, longer than the cadence
// period plus two clock quanta. One pause may show as several overlapping spans.
func frameEvidence(s Sample, c Clock) (raw, upper, interval float64, missed []FineSpan, err error) {
	if !validClock(c) || len(s.Frames) < 2 || len(s.Baseline) != 6 || len(s.Heartbeat) < 2 {
		return 0, 0, 0, nil, errors.New("missing frame work or calibrated clock/cadence evidence")
	}
	period, err := cadencePeriod(s.Baseline)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	if err := checkCadence(s, c, period); err != nil {
		return 0, 0, 0, nil, err
	}
	h, hu, handlerErr := measuredHandlerWork(s.Handlers, c)
	if handlerErr != nil {
		return 0, 0, 0, nil, handlerErr
	}
	raw, upper, interval, missed, err = frameSpans(s, c, period, h, hu)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	interval, missed = heartbeatSpans(s, c, period, interval, missed)
	if s.SettledAt > 0 {
		last := s.Heartbeat[len(s.Heartbeat)-1]
		if s.SettledAt-last > period+2*c.ResolutionMS {
			missed = append(missed, FineSpan{Start: last, End: s.SettledAt})
		}
		busyRaw, busyUpper := busyWork(s, c)
		raw, upper = math.Max(raw, busyRaw), math.Max(upper, busyUpper)
		fenceRaw, fenceUpper, err := fencedWork(s, c)
		if err != nil {
			return 0, 0, 0, nil, err
		}
		raw, upper = math.Max(raw, fenceRaw), math.Max(upper, fenceUpper)
	}
	return raw, upper, interval, missed, nil
}

// validTasks checks that every App callback interval is finite and falls
// inside the settle window.
func validTasks(s Sample, c Clock) bool {
	for _, span := range s.Tasks {
		if !finite(span.Start) || !finite(span.End) || span.End < span.Start || span.Start > s.SettledAt || span.End > s.SettledAt+c.ResolutionMS {
			return false
		}
	}
	return true
}

// hasOuterBracket reports whether a keystroke-task handler carries fine
// boundaries.
func hasOuterBracket(handlers []Handler) bool {
	return slices.ContainsFunc(handlers, func(h Handler) bool { return h.Event == "keystroke-task" && h.Fine != nil })
}

// heartbeatsCovered reports whether every post-input heartbeat inside the
// settle window matches the start of some frame.
func heartbeatsCovered(s Sample, c Clock) bool {
	for index, arrival := range s.Heartbeat {
		if index < s.PreInputHeartbeatCount || arrival < s.KeyDownAt || arrival > s.SettledAt {
			continue
		}
		found := slices.ContainsFunc(s.Frames, func(f WorkFrame) bool {
			return math.Abs(f.Start-arrival) <= c.ResolutionMS
		})
		if !found {
			return false
		}
	}
	return true
}

// probeContinuous reports whether the task-probe spans are finite and
// contiguous within one quantum, returning the end of the last span.
func probeContinuous(s Sample, c Clock) (last float64, ok bool) {
	last = s.Busy[0].Start
	for _, span := range s.Busy {
		if !finite(span.Start) || !finite(span.End) || span.End < span.Start || math.Abs(span.Start-last) > c.ResolutionMS {
			return 0, false
		}
		last = span.End
	}
	return last, true
}

func coverage(s Sample, c Clock) error {
	if !finite(s.SettledAt) || s.SettledAt-s.KeyDownAt < 1200 || len(s.Busy) < 2 {
		return errors.New("uncovered deferred work: missing >=1200ms settle/task-probe window")
	}
	if len(s.Tasks) == 0 && s.Control.Kind != "null" {
		return errors.New("uncovered App callbacks: task recorder did not observe input")
	}
	if !validTasks(s, c) {
		return errors.New("invalid App callback work interval")
	}
	if !hasOuterBracket(s.Handlers) {
		return errors.New("uncovered window listeners/microtasks: missing outer keystroke bracket")
	}
	if s.PreInputHeartbeatCount < 0 || s.PreInputHeartbeatCount > len(s.Heartbeat) || (s.PreInputHeartbeatCount > 0 && s.Heartbeat[s.PreInputHeartbeatCount-1] > s.KeyDownAt+c.ResolutionMS) {
		return errors.New("invalid pre-input heartbeat boundary")
	}
	if !heartbeatsCovered(s, c) {
		return errors.New("uncovered rendering opportunity in settle window")
	}
	if s.Busy[0].Start > s.KeyDownAt {
		return errors.New("task probe starts after input")
	}
	last, ok := probeContinuous(s, c)
	if !ok {
		return errors.New("uncovered main-thread work: discontinuous task probe")
	}
	if last < s.SettledAt {
		return errors.New("task probe ends before settle boundary")
	}
	return nil
}

func frameDetectsControl(s Sample) bool {
	for _, f := range s.Frames {
		if f.Start <= s.Control.Start && f.End >= s.Control.End && f.End-f.Start >= 50 {
			return true
		}
	}
	return false
}
