// Package nativeperf validates raw native input evidence. It is imported by the
// desktop entry point only in a dedicated perf_native, non-production build.
package nativeperf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

// StreamName exists only in the explicitly tagged native performance harness.
const StreamName = "native-perf"

// ControlKinds lists the positive-control kinds a run is expected to cover,
// one sample per kind.
var ControlKinds = []string{"handler", "layout", "early-raf", "measure", "document", "task", "deferred"}

// NullCount is the number of NULL trials per arm: recording hooks on ("null") and off (NullOff).
const NullCount = 30

// NullOff is the NULL trial run with every in-page recording hook off; only the
// outer keystroke bracket's clock reads remain, as in the "null" arm.
const NullOff = "null-off"

// Phase budgets (user decisions 2026-11-07). The first keystroke of a fresh
// process (cold, including the first preview presentation) has its own
// thresholds: no pause > 70 ms cold, > 50 ms warm. The warm handler gate is
// p95 <= 8 ms; 4 ms stays the objective, reported but not gating. Cold missed
// refreshes are published but do not gate ("nuit" decision).
const (
	warmHandlerP95MS       = 8
	objectiveWarmHandlerMS = 4
	warmFrameWorkMS        = 16
	warmGapMS              = 50
	coldHandlerP95MS       = 50
	coldFrameWorkP95MS     = 50
	coldGapMS              = 70
	overheadBoundMS        = 1
)

// nullTrialKind is the arm of the k-th NULL trial (1-based): ABBA blocks
// (on, off, off, on) interleave the arms so a linear host drift weighs on both
// equally. scripts/perf/native_input.py:null_schedule mirrors it.
func nullTrialKind(k int) string {
	if r := (k - 1) % 4; r == 0 || r == 3 {
		return "null"
	}
	return NullOff
}

type (
	// Host identifies the linked native libraries and build mode.
	Host struct {
		InputPermissionVerified bool    `json:"inputPermissionVerified"`
		Backend                 string  `json:"backend"`
		XID                     uint64  `json:"xid"`
		RefreshHz               float64 `json:"refreshHz"`
		WebKitVersion           string  `json:"webkitVersion"`
		GTKVersion              string  `json:"gtkVersion"`
		Engine                  string  `json:"engine"`
		Native                  bool    `json:"native"`
		GOOS                    string  `json:"goos"`
		Server                  bool    `json:"server"`
	}
	// Ready is the renderer readiness record before any native trial.
	Ready struct {
		EarlyCapture  bool   `json:"earlyCapture"`
		InputVerified bool   `json:"inputVerified"`
		Clock         Clock  `json:"clock"`
		Type          string `json:"type"`
		Visible       bool   `json:"visible"`
		Focused       bool   `json:"focused"`
		UserAgent     string `json:"userAgent"`
		PaintOracle   string `json:"paintOracle"`
		// PluginSources maps cm-plugin stage labels to a minified source prefix (diagnostic only).
		PluginSources map[string]string `json:"pluginSources,omitempty"`
	}
	// Handler records one actual input dispatch segment.
	Handler struct {
		Fine           *FineSpan `json:"fine,omitempty"`
		EventTimeStamp *float64  `json:"eventTimeStamp,omitempty"`
		Call           int64     `json:"call,omitempty"`
		Parent         int64     `json:"parent,omitempty"`
		Event          string    `json:"event"`
		Start          float64   `json:"start"`
		End            float64   `json:"end"`
		Trusted        bool      `json:"trusted"`
	}
	// Timing preserves an unmodified native PerformanceEventTiming entry.
	Timing struct {
		Name            string   `json:"name"`
		StartTime       float64  `json:"startTime"`
		Duration        float64  `json:"duration"`
		ProcessingStart float64  `json:"processingStart"`
		ProcessingEnd   float64  `json:"processingEnd"`
		InteractionID   *float64 `json:"interactionId,omitempty"`
	}
	// Control records the bounded positive-control workload interval.
	Control struct {
		Kind       string  `json:"kind"`
		Start      float64 `json:"start"`
		End        float64 `json:"end"`
		Iterations int     `json:"iterations"`
	}
	// Sample contains raw timestamps and actual document change identities.
	// KeyDownAt, the keystroke start, is the first performance.now() read in the
	// window capture keydown listener; KeyEventTimeStamp, the keydown's
	// event.timeStamp, is a diagnostic only.
	Sample struct {
		PreInputHeartbeatCount int                `json:"preInputHeartbeatCount,omitempty"`
		KeyEventTimeStamp      *float64           `json:"keyEventTimeStamp,omitempty"`
		NullInserted           bool               `json:"nullInserted,omitempty"`
		NullForeignPaths       []string           `json:"nullForeignPaths,omitempty"`
		Tasks                  []FineSpan         `json:"tasks"`
		SettledAt              float64            `json:"settledAt"`
		Busy                   []FineSpan         `json:"busy"`
		Frames                 []WorkFrame        `json:"frames"`
		Baseline               []float64          `json:"baseline"`
		Stages                 []Handler          `json:"stages,omitempty"`
		Listeners              []ListenerSpan     `json:"listeners,omitempty"`
		Callbacks              []CallbackIdentity `json:"callbacks,omitempty"`
		Type                   string             `json:"type"`
		ID                     int                `json:"id"`
		RunID                  string             `json:"runId"`
		Phase                  string             `json:"phase"`
		Handlers               []Handler          `json:"handlers"`
		KeyDownAt              float64            `json:"keyDownAt"`
		DocChangedAt           float64            `json:"docChangedAt"`
		RevisionBefore         string             `json:"revisionBefore"`
		RevisionAfter          string             `json:"revisionAfter"`
		LengthBefore           int                `json:"lengthBefore"`
		LengthAfter            int                `json:"lengthAfter"`
		RAF                    []float64          `json:"raf"`
		Heartbeat              []float64          `json:"heartbeat"`
		EventTiming            []Timing           `json:"eventTiming"`
		Control                Control            `json:"control"`
		Visible                bool               `json:"visible"`
		Focused                bool               `json:"focused"`
		Trusted                bool               `json:"trusted"`
		Changed                bool               `json:"changed"`
	}
	// Run is one fresh native process or one warm native session.
	Run struct {
		NullSamples    []Sample `json:"nullSamples,omitempty"`
		NullOffSamples []Sample `json:"nullOffSamples,omitempty"`
		Protocol       int      `json:"protocol"`
		RunID          string   `json:"runId"`
		PID            int      `json:"pid"`
		StartedNS      int64    `json:"startedNS"`
		Phase          string   `json:"phase"`
		ReplicateIndex int      `json:"replicateIndex"`
		ReplicateCount int      `json:"replicateCount"`
		Host           Host     `json:"host"`
		Ready          Ready    `json:"ready"`
		Samples        []Sample `json:"samples"`
		Failures       []string `json:"failures"`
	}
	// Metric contains raw values and a recomputed nearest-rank p95.
	Metric struct {
		SamplesMS []float64 `json:"samples_ms"`
		P95MS     *float64  `json:"p95_ms"`
	}
	// Report is the full fail-closed verdict with its diagnostics; the raw runs
	// stay in their files, referenced by RawEvidence.
	Report struct {
		NullHandler      Metric              `json:"null_handler"`
		NullHandlerUpper Metric              `json:"null_handler_upper_bound"`
		Overhead         *Overhead           `json:"instrument_overhead,omitempty"`
		Attribution      map[string]Metric   `json:"attribution_diagnostic"`
		AttributionTable *AttributionTable   `json:"attribution_table,omitempty"`
		MaxGapUpperMS    *float64            `json:"max_gap_upper_bound_ms"`
		Controls         []ControlValidation `json:"controls"`
		CoarseHandler    Metric              `json:"coarse_handler_diagnostic"`
		Profile          map[string]Metric   `json:"input_profile"`
		Measurement      string              `json:"measurement"`
		HandlerUpper     Metric              `json:"handler_upper_bound"`
		FrameWork        Metric              `json:"frame_work"`
		FrameWorkUpper   Metric              `json:"frame_work_upper_bound"`
		ColdFrameWork    Metric              `json:"cold_frame_work_upper_bound"`
		WarmFrameWork    Metric              `json:"warm_frame_work_upper_bound"`
		FrameInterval    Metric              `json:"frame_interval"`
		DroppedFrames    int                 `json:"dropped_frames"`
		MissedRefresh    *MissedRefresh      `json:"missed_refresh,omitempty"`
		Clocks           []Clock             `json:"clocks"`
		ColdHandler      Metric              `json:"cold_handler"`
		WarmHandler      Metric              `json:"warm_handler"`
		ColdFrameWait    Metric              `json:"cold_frame_wait"`
		WarmFrameWait    Metric              `json:"warm_frame_wait"`
		ID               string              `json:"id"`
		Status           string              `json:"status"`
		// ObjectiveWarmHandlerMS is the non-gating warm handler objective, shown
		// next to the verdict; ObjectiveMet reports whether the warm p95 meets it.
		ObjectiveWarmHandlerMS float64            `json:"objective_warm_handler_ms"`
		ObjectiveMet           bool               `json:"objective_met"`
		Reasons                []string           `json:"reasons"`
		Handler                Metric             `json:"handler"`
		FrameWait              Metric             `json:"frame_wait"`
		Paint                  Metric             `json:"paint_upper_bound"`
		ColdPaint              Metric             `json:"cold_paint_upper_bound"`
		WarmPaint              Metric             `json:"warm_paint_upper_bound"`
		MaxGapMS               *float64           `json:"max_gap_ms"`
		WarmMaxGapMS           *float64           `json:"warm_max_gap_ms"`
		ColdMaxGapMS           *float64           `json:"cold_max_gap_ms"`
		WarmMaxGapUpper        *float64           `json:"warm_max_gap_upper_bound_ms"`
		ColdMaxGapUpper        *float64           `json:"cold_max_gap_upper_bound_ms"`
		RawEvidence            []Evidence         `json:"raw_evidence"`
		Thresholds             map[string]float64 `json:"thresholds"`
	}
	// ControlValidation exposes the independent frame and Event Timing witnesses.
	ControlValidation struct {
		Kind               string  `json:"kind"`
		Validated          bool    `json:"validated"`
		FrameWorkMS        float64 `json:"frame_work_ms"`
		DroppedFrame       bool    `json:"dropped_frame"`
		EventTimingUpperMS float64 `json:"event_timing_upper_ms"`
	}
)

func strict(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("native perf: trailing data")
	}
	return nil
}

// DecodeRun rejects aggregates and unknown evidence fields.
func DecodeRun(data []byte) (Run, error) { var run Run; err := strict(data, &run); return run, err }
func finite(v float64) bool              { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
func metric(values []float64) Metric {
	m := Metric{SamplesMS: values}
	if len(values) > 0 {
		sorted := append([]float64{}, values...)
		sort.Float64s(sorted)
		p := sorted[int(math.Ceil(.95*float64(len(sorted))))-1]
		m.P95MS = &p
	}
	return m
}

func gap(values []float64) (float64, error) {
	if len(values) < 2 {
		return 0, errors.New("missing frame heartbeat")
	}
	maxGap := 0.0
	for i, v := range values {
		if !finite(v) || (i > 0 && v <= values[i-1]) {
			return 0, errors.New("invalid frame timestamps")
		}
		if i > 0 {
			maxGap = math.Max(maxGap, v-values[i-1])
		}
	}
	return maxGap, nil
}

func sampleMetrics(s Sample) (handler, wait, paint, maxGap float64, err error) {
	if !actualInput(s) {
		return 0, 0, 0, 0, errors.New("input was hidden, unfocused, untrusted or did not change the actual document")
	}
	if !finite(s.KeyDownAt) || !finite(s.DocChangedAt) || s.DocChangedAt < s.KeyDownAt || len(s.Handlers) == 0 || len(s.RAF) < 2 {
		return 0, 0, 0, 0, errors.New("missing input/frame boundaries")
	}
	if err := keystrokeStart(s); err != nil {
		return 0, 0, 0, 0, err
	}
	if err := inputSegments(s); err != nil {
		return 0, 0, 0, 0, err
	}
	handler, _ = handlerWork(s.Handlers, 0)
	if _, err = gap(s.RAF); err != nil || s.RAF[0] < s.DocChangedAt {
		return 0, 0, 0, 0, errors.New("invalid frame callback timestamps")
	}
	wait = s.RAF[0] - s.KeyDownAt
	if len(s.Heartbeat) < 2 || s.Heartbeat[0] > s.KeyDownAt || s.Heartbeat[len(s.Heartbeat)-1] < s.RAF[len(s.RAF)-1] {
		return 0, 0, 0, 0, errors.New("heartbeat does not cover input through final frame callback")
	}
	maxGap, err = gap(s.Heartbeat)
	if err != nil {
		return handler, wait, paint, maxGap, err
	}
	if finite(s.SettledAt) && s.SettledAt > 0 {
		maxGap = math.Max(maxGap, s.SettledAt-s.Heartbeat[len(s.Heartbeat)-1])
	}
	paint, err = censoredPaintBound(s)
	return handler, wait, paint, maxGap, err
}

// inputSegments checks that every handler is trusted and ordered after the
// keydown, and that both a CodeMirror segment covering the document change and
// a DOM input segment are present.
func inputSegments(s Sample) error {
	cm, dom := false, false
	for _, h := range s.Handlers {
		if !h.Trusted || !finite(h.Start) || !finite(h.End) || h.Start < s.KeyDownAt || h.End < h.Start {
			return errors.New("invalid handler timestamps")
		}
		if h.Event == "codemirror-input" && s.DocChangedAt >= h.Start && s.DocChangedAt <= h.End {
			cm = true
		}
		if h.Event == "input" || h.Event == "beforeinput" {
			dom = true
		}
	}
	if !cm || !dom {
		return errors.New("missing actual CodeMirror and DOM input handler segments")
	}
	return nil
}

// censoredPaintBound is paintBound, except that for the none, task and
// deferred controls a missing correlated event is not an error.
func censoredPaintBound(s Sample) (float64, error) {
	paint, err := paintBound(s)
	if (s.Control.Kind == "none" || s.Control.Kind == "task" || s.Control.Kind == "deferred") && errors.Is(err, errNoCorrelatedTiming) {
		// Event Timing censors fast events. Its presence is required only for controls.
		return 0, nil
	}
	return paint, err
}

// keystrokeStart: the keystroke starts at the keydown capture listener's first
// clock read, so every later read is at or after it by construction.
func keystrokeStart(s Sample) error {
	for _, h := range s.Handlers {
		if h.Event == "keydown" && h.Start != s.KeyDownAt {
			return errors.New("keystroke start is not the keydown capture listener's first clock read")
		}
	}
	return nil
}

var (
	errPaintOracle        = errors.New("E-2: native paint oracle unavailable or unvalidated")
	errNoCorrelatedTiming = fmt.Errorf("%w: no event correlated with actual input; frame callbacks are not paint", errPaintOracle)
)

func paintBound(s Sample) (float64, error) {
	// Paint is bounded from the keystroke start (KeyDownAt). The keydown's own
	// timeStamp is not used: on the host it ran up to +24 ms ahead of
	// performance.now(), while beforeinput/input entries share its base.
	matched := false
	upper := 0.0
	for _, e := range s.EventTiming {
		if !finite(e.StartTime) || !finite(e.Duration) || !finite(e.ProcessingStart) || !finite(e.ProcessingEnd) || e.ProcessingStart < e.StartTime || e.ProcessingEnd < e.ProcessingStart {
			return 0, fmt.Errorf("%w: invalid EventTiming timestamps", errPaintOracle)
		}
		if e.Name != "input" && e.Name != "beforeinput" {
			continue
		}
		associated := false
		for _, h := range s.Handlers {
			// WebKit independently coarsens event/observer and performance.now
			// boundaries to 1 ms. Require event identity and containment within
			// that quantum, never a nearby unrelated event or interactionId=0.
			if h.Event == e.Name && h.EventTimeStamp != nil && finite(*h.EventTimeStamp) && math.Abs(*h.EventTimeStamp-e.StartTime) <= 1 && h.Start >= e.ProcessingStart-1 && h.End <= e.ProcessingEnd+1 {
				associated = true
			}
		}
		if !associated || e.StartTime < s.KeyDownAt-1 {
			continue
		}
		if e.Duration < 16 || math.Mod(e.Duration, 8) != 0 {
			return 0, fmt.Errorf("%w: invalid native EventTiming quantization", errPaintOracle)
		}
		// Presentation cannot precede completion of this correlated event's
		// processing. Intersect the 8ms duration bucket with that independent
		// lower bound, retaining WebKit's 1ms timestamp uncertainty. Do not
		// mistake a duration bucket overlapping processing for post-paint work.
		lower := math.Max(e.StartTime+e.Duration-4, e.ProcessingEnd-1)
		if s.DocChangedAt > lower {
			return 0, fmt.Errorf("%w: document change overlaps uncertain paint interval", errPaintOracle)
		}
		if e.ProcessingEnd > e.StartTime+e.Duration+4 {
			return 0, fmt.Errorf("%w: paint precedes event processing", errPaintOracle)
		}
		upper = math.Max(upper, e.StartTime+e.Duration+4-s.KeyDownAt)
		matched = true
	}
	if !matched {
		return 0, errNoCorrelatedTiming
	}
	return upper, nil
}

// Validate recomputes the verdict from raw, authenticated native-session records.
func Validate(runs []Run) Report {
	v := verifier{report: Report{ID: "PERF-TYPE-native", Measurement: "native-work-v6-overhead-bound-phases", Status: "fail", ObjectiveWarmHandlerMS: objectiveWarmHandlerMS, Thresholds: map[string]float64{"frame_work_ms": warmFrameWorkMS, "frame_work_p95_ms": warmFrameWorkMS, "handler_p95_ms": warmHandlerP95MS, "cold_handler_p95_ms": coldHandlerP95MS, "cold_frame_work_p95_ms": coldFrameWorkP95MS, "frame_gap_ms": warmGapMS, "cold_frame_gap_ms": coldGapMS, "instrument_overhead_upper95_ms": overheadBoundMS}}, coldIndices: map[int]bool{}, ids: map[string]bool{}, processes: map[string]bool{}}
	for _, run := range runs {
		v.addRun(run)
	}
	return v.finish()
}

type verifier struct {
	nulls, nullUppers, nullOffs                                                    []float64
	attributed                                                                     map[string][]float64
	table                                                                          AttributionTable
	stages                                                                         map[string][]float64
	uppers, works, workUppers, intervals, coldWork, warmWork, coldUpper, warmUpper []float64
	report                                                                         Report
	handlers, waits, paints, cold, warm                                            []float64
	coldHandlers, warmHandlers, coldWaits, warmWaits                               []float64
	coldIndices                                                                    map[int]bool
	ids, processes                                                                 map[string]bool
	replicates, warmCount                                                          int
	warmMissed, warmTrials, nullMissed, nullTrials, coldExempt, coldMissed         int
	handlerControl, layoutControl                                                  bool
	maxGap                                                                         float64
	maxGapUpper                                                                    float64
	warmGap, coldGap, warmGapUpper, coldGapUpper                                   float64
	controls                                                                       map[string]bool
}

func (v *verifier) fail(reason string) {
	if slices.Contains(v.report.Reasons, reason) {
		return
	}
	v.report.Reasons = append(v.report.Reasons, reason)
}

func validHost(h Host) bool {
	return h.Native && !h.Server && h.Engine == "WebKitGTK" && h.GOOS == "linux" && h.WebKitVersion != "" && h.GTKVersion != ""
}

func (v *verifier) addRun(run Run) {
	if run.Protocol != 1 || run.RunID == "" || v.ids[run.RunID] || run.PID <= 0 || run.StartedNS <= 0 || !validHost(run.Host) {
		v.fail("invalid native host/run identity")
		return
	}
	v.ids[run.RunID] = true
	process := fmt.Sprintf("%d:%d", run.PID, run.StartedNS)
	if v.processes[process] {
		v.fail("cold evidence reused a process")
	}
	v.processes[process] = true
	if !run.Ready.Visible || !run.Ready.Focused || run.Ready.Type != "ready" || run.Ready.PaintOracle != "event-timing" {
		v.fail("E-2: native window or validated paint oracle unavailable")
	}
	if !validClock(run.Ready.Clock) {
		v.fail("missing observed timer resolution and rounding rule")
	}
	if !validPreciseClock(run.Ready.Clock.Precise) {
		v.fail("missing adequate native handler clock; coarse p95 cannot certify the 4ms budget")
	}
	if !run.Ready.EarlyCapture {
		v.fail("uncovered window listeners/early rAF: recorder not installed before App")
	}
	if !run.Ready.InputVerified || !run.Host.InputPermissionVerified || run.Host.Backend != "x11" || run.Host.XID == 0 {
		v.fail("native X11 target/permission not attested by trusted input")
	}
	if !finite(run.Host.RefreshHz) || run.Host.RefreshHz <= 0 || run.Ready.Clock.RefreshHz != run.Host.RefreshHz {
		v.fail("missing or mismatched GdkMonitor nominal refresh")
	}
	v.report.Clocks = append(v.report.Clocks, run.Ready.Clock)
	for label, source := range run.Ready.PluginSources {
		if v.table.PluginSources == nil {
			v.table.PluginSources = map[string]string{}
		}
		v.table.PluginSources[label] = source
	}
	for _, reason := range run.Failures {
		v.fail(reason)
	}
	validateRunShape(run, v.fail)
	if run.Phase == "cold" {
		v.addCold(run)
	}
	for _, sample := range run.Samples {
		v.addSample(run.Phase, sample, run.Ready.Clock)
	}
	v.addNullArms(run)
}

// addNullArms checks each NULL sample against its interleaved slot, then its arm.
func (v *verifier) addNullArms(run Run) {
	for _, arm := range []struct {
		kind    string
		samples []Sample
		add     func(Sample, Clock)
	}{{"null", run.NullSamples, v.addNull}, {NullOff, run.NullOffSamples, v.addNullOff}} {
		previous := 0
		for _, sample := range arm.samples {
			k := sample.ID - len(run.Samples)
			if run.Phase != "warm" || sample.Type != "sample" || k <= previous || k > 2*NullCount || nullTrialKind(k) != arm.kind || sample.RunID != run.RunID || sample.Phase != run.Phase {
				v.fail("invalid NULL trial identity")
				continue
			}
			previous = k
			arm.add(sample, run.Ready.Clock)
		}
	}
}

func (v *verifier) addCold(run Run) {
	if v.coldIndices[run.ReplicateIndex] {
		v.fail("duplicate cold replicate")
	}
	v.coldIndices[run.ReplicateIndex] = true
	if v.replicates != 0 && v.replicates != run.ReplicateCount {
		v.fail("inconsistent cold replicate count")
	}
	v.replicates = run.ReplicateCount
}

func (v *verifier) addSample(phase string, s Sample, clock Clock) {
	h, w, p, g, err := sampleMetrics(s)
	if err != nil {
		v.fail(err.Error())
		if !errors.Is(err, errPaintOracle) {
			return
		}
	}
	coverageErr := coverage(s, clock)
	if coverageErr != nil {
		v.fail(coverageErr.Error())
	}
	if s.Control.Kind != "none" {
		v.addControl(s, clock, h, p, g, coverageErr == nil && err == nil)
		return
	}
	v.report.CoarseHandler.SamplesMS = append(v.report.CoarseHandler.SamplesMS, h)
	measured, upper, handlerErr := measuredHandlerWork(s.Handlers, clock)
	if handlerErr != nil {
		v.fail(handlerErr.Error())
		// Preserve coarse evidence for diagnosis, but this run can never pass.
		measured, upper = handlerWork(s.Handlers, clock.ResolutionMS)
	}
	h = measured
	v.attribute(s, h)
	v.table.add(phase, s)
	v.recordStages(s)
	v.uppers = append(v.uppers, upper)
	v.recordFrames(phase, s, clock)
	v.recordTimings(phase, clock, h, w, g, upper)
	if err != nil {
		return
	}
	if p == 0 {
		return
	} // No censored/missing Event Timing value is invented.
	v.paints = append(v.paints, p)
	if phase == "cold" {
		v.cold = append(v.cold, p)
	} else {
		v.warm = append(v.warm, p)
	}
}

// addControl validates one positive-control sample and records the outcome.
// clean is true when neither the metrics nor the coverage check failed.
func (v *verifier) addControl(s Sample, clock Clock, h, p, g float64, clean bool) {
	a, b := validateControl(s, h, p, g-clock.ResolutionMS)
	if !controlGapLower(s.Heartbeat, s.Control, clock.ResolutionMS, 50) {
		a, b = false, false
	}
	work, _, _, dropped, frameErr := frameWork(s, clock)
	if frameErr != nil {
		v.fail(frameErr.Error())
		a, b = false, false
	} else if work <= 16 || !dropped {
		a, b = false, false
	}
	passed := a || b
	if s.Control.Kind != "handler" && s.Control.Kind != "layout" {
		passed = placementControl(s, h, p, clock.ResolutionMS) && frameErr == nil && work > 16 && dropped
	}
	passed = passed && clean
	if v.controls == nil {
		v.controls = map[string]bool{}
	}
	v.controls[s.Control.Kind] = passed
	v.report.Controls = append(v.report.Controls, ControlValidation{Kind: s.Control.Kind, Validated: passed, FrameWorkMS: work, DroppedFrame: dropped, EventTimingUpperMS: p})
	v.handlerControl = v.handlerControl || a
	v.layoutControl = v.layoutControl || b
	if !passed {
		v.fail("uncovered or missed positive-control placement: " + s.Control.Kind)
		v.fail("positive slow-layout/control oracle was not validated")
	}
}

// recordStages appends the duration of each profiled input stage of s.
func (v *verifier) recordStages(s Sample) {
	if v.stages == nil {
		v.stages = map[string][]float64{}
	}
	for _, stage := range append(append([]Handler{}, s.Stages...), s.Handlers...) {
		if stage.Event != "codemirror-input" && stage.Event != "automerge-ui" && stage.Event != "language-didChange" {
			continue
		}
		if !finite(stage.Start) || !finite(stage.End) || stage.End < stage.Start {
			v.fail("invalid input profiling segment")
			continue
		}
		v.stages[stage.Event] = append(v.stages[stage.Event], stage.End-stage.Start)
	}
}

// recordFrames records the per-frame work, intervals and missed refresh
// intervals of s for its phase.
func (v *verifier) recordFrames(phase string, s Sample, clock Clock) {
	work, workUpper, interval, missed, frameErr := frameEvidence(s, clock)
	if frameErr != nil {
		v.fail(frameErr.Error())
		return
	}
	v.works = append(v.works, work)
	v.workUppers = append(v.workUppers, workUpper)
	if phase == "warm" && workUpper > warmFrameWorkMS {
		v.fail("native per-frame work exceeds16ms")
	}
	v.intervals = append(v.intervals, interval)
	if phase == "cold" {
		// Cold misses are a published diagnostic, not a gate (decision
		// 2026-11-07, nuit): the startup thresholds judge the cold phase.
		counted := coldCountedMisses(missed, clock.ResolutionMS)
		if len(counted) < len(missed) {
			v.coldExempt++
		}
		if len(counted) > 0 {
			v.report.DroppedFrames++
			v.coldMissed++
		}
	} else {
		// Warm misses are judged relative to NULL typing in finish.
		v.warmTrials++
		if len(missed) > 0 {
			v.report.DroppedFrames++
			v.warmMissed++
		}
	}
	if phase == "cold" {
		v.coldWork = append(v.coldWork, workUpper)
	} else {
		v.warmWork = append(v.warmWork, workUpper)
	}
}

// recordTimings records the handler, wait and heartbeat-gap measurements of
// one sample, overall and for its phase.
func (v *verifier) recordTimings(phase string, clock Clock, h, w, g, upper float64) {
	v.handlers = append(v.handlers, h)
	v.waits = append(v.waits, w)
	v.maxGap = math.Max(v.maxGap, g)
	v.maxGapUpper = math.Max(v.maxGapUpper, g+clock.ResolutionMS)
	if phase == "cold" {
		v.coldGap = math.Max(v.coldGap, g)
		v.coldGapUpper = math.Max(v.coldGapUpper, g+clock.ResolutionMS)
		v.coldUpper = append(v.coldUpper, upper)
		v.coldHandlers = append(v.coldHandlers, h)
		v.coldWaits = append(v.coldWaits, w)
	} else {
		v.warmGap = math.Max(v.warmGap, g)
		v.warmGapUpper = math.Max(v.warmGapUpper, g+clock.ResolutionMS)
		v.warmUpper = append(v.warmUpper, upper)
		v.warmCount++
		v.warmHandlers = append(v.warmHandlers, h)
		v.warmWaits = append(v.warmWaits, w)
	}
}

func (v *verifier) finish() Report {
	if len(v.nulls) < NullCount || len(v.nullOffs) < NullCount {
		v.fail("missing >=30 NULL keystrokes per arm (recording hooks on and off): instrument overhead cannot be bounded")
	}
	if o, ok := overheadBound(v.nulls, v.nullOffs); ok {
		v.report.Overhead = &o
		if o.UpperBound95MS > overheadBoundMS {
			v.fail("instrument overhead (recording hooks on minus off) one-sided 95% upper bound exceeds1ms")
		}
	}
	if v.replicates < 30 || len(v.coldIndices) != v.replicates || len(v.coldHandlers) < 30 || v.warmCount < 30 || len(v.coldWork) < 30 || len(v.warmWork) < 30 {
		v.fail("missing >=30 fresh-process cold and >=30 warm samples")
	}
	if !v.handlerControl || !v.layoutControl {
		v.fail("missing independently validated handler and post-handler layout controls")
	}
	for _, kind := range ControlKinds {
		if !v.controls[kind] {
			v.fail("missing validated positive control: " + kind)
		}
	}
	r := &v.report
	r.NullHandler = metric(v.nulls)
	r.NullHandlerUpper = metric(v.nullUppers)
	if len(v.table.Phases) > 0 {
		v.table.Method = attributionMethod
		v.table.finish(60)
		r.AttributionTable = &v.table
	}
	r.Attribution = map[string]Metric{}
	for name, values := range v.attributed {
		r.Attribution[name] = metric(values)
	}
	r.CoarseHandler = metric(r.CoarseHandler.SamplesMS)
	r.Profile = map[string]Metric{}
	for name, values := range v.stages {
		r.Profile[name] = metric(values)
	}
	r.HandlerUpper = metric(v.uppers)
	r.FrameWork = metric(v.works)
	r.FrameWorkUpper = metric(v.workUppers)
	r.ColdFrameWork = metric(v.coldWork)
	r.WarmFrameWork = metric(v.warmWork)
	r.FrameInterval = metric(v.intervals)
	r.ColdHandler = metric(v.coldHandlers)
	r.WarmHandler = metric(v.warmHandlers)
	r.ColdFrameWait = metric(v.coldWaits)
	r.WarmFrameWait = metric(v.warmWaits)
	r.Handler = metric(v.handlers)
	r.FrameWait = metric(v.waits)
	r.Paint = metric(v.paints)
	r.ColdPaint = metric(v.cold)
	r.WarmPaint = metric(v.warm)
	if len(v.handlers) > 0 {
		r.MaxGapMS = &v.maxGap
		r.MaxGapUpperMS = &v.maxGapUpper
		r.WarmMaxGapMS, r.WarmMaxGapUpper = &v.warmGap, &v.warmGapUpper
		r.ColdMaxGapMS, r.ColdMaxGapUpper = &v.coldGap, &v.coldGapUpper
	}
	r.ObjectiveMet = r.WarmHandler.P95MS != nil && *r.WarmHandler.P95MS <= objectiveWarmHandlerMS
	v.checkBudget("warm handler", r.WarmHandler, warmHandlerP95MS)
	v.checkBudget("cold handler", r.ColdHandler, coldHandlerP95MS)
	v.checkBudget("cold handler conservative upper bound", metric(v.coldUpper), coldHandlerP95MS)
	v.checkBudget("warm handler conservative upper bound", metric(v.warmUpper), warmHandlerP95MS)
	v.checkBudget("cold frame work", r.ColdFrameWork, coldFrameWorkP95MS)
	v.checkBudget("warm frame work", r.WarmFrameWork, warmFrameWorkMS)
	if v.warmGapUpper > warmGapMS {
		v.fail("native frame gap exceeds50ms")
	}
	if v.coldGapUpper > coldGapMS {
		v.fail("native cold frame gap exceeds70ms")
	}
	r.MissedRefresh = v.missedRefresh()
	if len(r.Reasons) == 0 {
		r.Status = "pass"
	}
	return *r
}

func validateRunShape(run Run, fail func(string)) {
	if run.Phase != "cold" && run.Phase != "warm" {
		fail("invalid phase")
	}
	if run.Phase == "cold" && (len(run.Samples) != 1 || run.ReplicateCount < 30 || run.ReplicateIndex < 1 || run.ReplicateIndex > run.ReplicateCount) {
		fail("cold run is not one first-input fresh-process replicate")
	}
	if run.Phase == "warm" && len(run.Samples) < 30+len(ControlKinds) {
		fail("warm controls and >=30 subsequent inputs missing")
	}
	changes := map[string]bool{}
	for i, s := range run.Samples {
		change := s.RevisionBefore + ":" + s.RevisionAfter
		if changes[change] {
			fail("duplicate document revision transition")
		}
		changes[change] = true
		kind := "none"
		if run.Phase == "warm" && i < len(ControlKinds) {
			kind = ControlKinds[i]
		}
		if s.Type != "sample" || s.ID != i+1 || s.RunID != run.RunID || s.Phase != run.Phase || s.Control.Kind != kind {
			fail("duplicate, out-of-order or mismatched trial identity")
		}
	}
}

func validateControl(s Sample, h, p, g float64) (bool, bool) {
	c := s.Control
	if !finite(c.Start) || !finite(c.End) || c.End-c.Start < 80 || c.Iterations < 1 || g < 50 || !controlHeartbeatGap(s.Heartbeat, c) || p-8 < 50 || p+s.KeyDownAt < c.End {
		return false, false
	}
	if c.Kind == "handler" {
		return h >= 50, false
	}
	if c.Kind == "layout" && c.Start >= s.RAF[0] && c.End <= s.RAF[1] && frameDetectsControl(s) {
		return false, true
	}
	return false, false
}

// WriteJSON writes an evidence artifact after creating its private directory.
func WriteJSON(name string, value any) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(data, '\n'), 0o600)
}

// Merge recomputes a complete verdict from raw run files and writes it with
// WriteReport; driver metadata only adds failures.
func Merge(directory, out string) Report {
	files, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return Report{ID: "PERF-TYPE-native", Status: "fail", Reasons: []string{err.Error()}}
	}
	runs := []Run{}
	failures := []string{}
	raw := []Evidence{}
	for _, name := range files {
		data, err := os.ReadFile(name) // #nosec G304 -- perf_native harness: evidence files listed by the operator's own run, never peer input
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		sum := sha256.Sum256(data)
		raw = append(raw, Evidence{Path: filepath.ToSlash(name), SHA256: hex.EncodeToString(sum[:])})
		if filepath.Base(name) == "driver.json" {
			var driver struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}
			switch {
			case json.Unmarshal(data, &driver) != nil:
				failures = append(failures, "invalid native driver metadata")
			case driver.Status != "collected" && driver.Error != "":
				failures = append(failures, "native driver: "+driver.Error)
			}
			continue
		}
		run, err := DecodeRun(data)
		if err != nil {
			failures = append(failures, name+": "+err.Error())
			continue
		}
		runs = append(runs, run)
	}
	report := Validate(runs)
	report.RawEvidence = raw
	report.Reasons = uniqueReasons(append(report.Reasons, failures...))
	if len(failures) > 0 {
		report.Status = "fail"
	}
	if err := WriteReport(out, report); err != nil {
		report.Status = "fail"
		report.Reasons = append(report.Reasons, err.Error())
	}
	return report
}

func actualInput(s Sample) bool {
	return s.Visible && s.Focused && s.Trusted && s.Changed && s.RevisionBefore != "" && s.RevisionAfter != "" && s.RevisionBefore != s.RevisionAfter && s.LengthBefore >= 0 && s.LengthAfter == s.LengthBefore+1
}

func (v *verifier) checkBudget(name string, m Metric, budget float64) {
	if m.P95MS != nil && *m.P95MS > budget {
		v.fail(fmt.Sprintf("%s p95 exceeds%.0fms", name, budget))
	}
}

// collectedFailures validates one process without claiming that it establishes
// the required thirty fresh-process cold samples of a complete verdict.
func collectedFailures(run Run) []string {
	reasons := []string{}
	validateRunShape(run, func(reason string) { reasons = append(reasons, reason) })
	for _, s := range run.Samples {
		// Collection is distinct from acceptance: even failed controls or budgets
		// must not suppress the thirty first-key cold replicates.
		if !actualInput(s) || len(s.Handlers) == 0 || len(s.RAF) != 2 {
			reasons = append(reasons, "incomplete native raw input evidence")
		}
	}
	return uniqueReasons(reasons)
}

func uniqueReasons(reasons []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, reason := range reasons {
		if !seen[reason] {
			result = append(result, reason)
			seen[reason] = true
		}
	}
	return result
}

// The blocking control must itself interrupt adjacent heartbeat callbacks;
// a pause elsewhere in the measurement window is not a positive witness.
func controlHeartbeatGap(points []float64, c Control) bool {
	return controlGapLower(points, c, 0, 50)
}

func controlGapLower(points []float64, c Control, q, minimum float64) bool {
	for i := 1; i < len(points); i++ {
		if points[i-1] <= c.Start && points[i] >= c.End && points[i]-points[i-1]-q >= minimum {
			return true
		}
	}
	return false
}

func placementControl(s Sample, h, p, q float64) bool {
	c := s.Control
	minimum := 80.0
	if c.Kind == "task" || c.Kind == "deferred" {
		minimum = 30
	}
	if !finite(c.Start) || !finite(c.End) || c.End-c.Start < minimum || c.Iterations < 1 {
		return false
	}
	if c.Kind == "deferred" && c.Start < s.KeyDownAt+300-q {
		return false
	}
	covered := false
	for _, f := range s.Frames {
		if f.Start <= c.Start && f.End >= c.End && f.End-f.Start-q > 16 {
			covered = true
		}
	}
	for _, f := range s.Busy {
		if f.Start <= c.Start && f.End >= c.End && f.End-f.Start-q > 16 {
			covered = true
		}
	}
	for _, f := range s.Tasks {
		if f.Start <= c.Start && f.End >= c.End && f.End-f.Start-q > 16 {
			covered = true
		}
	}
	if !covered || !controlGapLower(s.Heartbeat, c, q, 16) {
		return false
	}
	if c.Kind == "document" {
		return h >= 50 && p-8 >= 50 && s.KeyDownAt+p >= c.End
	}
	if c.Kind == "early-raf" || c.Kind == "measure" {
		return p-8 >= 50 && s.KeyDownAt+p >= c.End
	}
	return c.Kind == "task" || c.Kind == "deferred"
}
