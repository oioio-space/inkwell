package nativeperf

import (
	"math"
	"strings"
	"testing"
)

func boundedClock() Clock {
	return Clock{RefreshHz: 60, ResolutionMS: 1, DeltasMS: []float64{1, 1, 1}, Rule: "upper:+quantum-per-disjoint-interval", Precise: &PreciseClock{Source: "browser-bounded", ResolutionMS: 1, BatchSize: 1000, ReadSpansMS: []float64{.001, .001, .001, .001}}}
}

func TestBoundedBrowserClockNeverHidesFiveOrSilentlyDeductsNullOverhead(t *testing.T) {
	c := boundedClock()
	h := []Handler{{Event: "keystroke-task", Start: 10, End: 13, Fine: &FineSpan{Start: 10, End: 13}}}
	if raw, upper, err := measuredHandlerWork(h, c); err != nil || raw != 3 || upper != 4 {
		t.Fatal(raw, upper, err)
	}
	// 4ms in a 1ms-quantized clock could really be 5ms: it MUST fail 4ms.
	h[0].End, h[0].Fine.End = 14, 14
	if raw, upper, err := measuredHandlerWork(h, c); err != nil || raw != 4 || upper != 5 {
		t.Fatal(raw, upper, err)
	}
	h[0].End, h[0].Fine.End = 15, 15
	if raw, upper, err := measuredHandlerWork(h, c); err != nil || raw != 5 || upper != 6 {
		t.Fatal(raw, upper, err)
	}
	c.Precise.BatchSize = 0
	if _, _, err := measuredHandlerWork(h, c); err == nil {
		t.Fatal("unmeasured clock read cost accepted")
	}
}

// setNullArm gives the n-th sample of a NULL arm an outer bracket of durations[n % len].
func setNullArm(samples []Sample, durations ...float64) {
	for i := range samples {
		for j := range samples[i].Handlers {
			h := &samples[i].Handlers[j]
			d := durations[i%len(durations)]
			h.End, h.Fine.End = h.Start+d, h.Fine.Start+d
		}
	}
}

func TestInstrumentOverheadIsMandatoryAndBoundedByTheDifferenceOfArms(t *testing.T) {
	runs := completeFrameFixture()
	runs[0].NullOffSamples = nil
	if got := Validate(runs); got.Status != "fail" || !strings.Contains(strings.Join(got.Reasons, ";"), "missing >=30 NULL keystrokes per arm") {
		t.Fatal(got.Reasons)
	}
	// Intrinsic browser work (~1.3 ms, raw p95 2 ms) in both arms is not overhead:
	// the old "NULL upper p95 <= 1 ms" rule failed this, the overhead rule passes it.
	runs = completeFrameFixture()
	setNullArm(runs[0].NullSamples, 1, 2, 1, 1, 2)
	setNullArm(runs[0].NullOffSamples, 1, 1, 2, 1, 2)
	got := Validate(runs)
	if got.Status != "pass" || got.Overhead == nil || got.Overhead.NOn != NullCount || got.Overhead.NOff != NullCount || *got.NullHandler.P95MS != 2 {
		t.Fatal(got.Reasons, got.Overhead)
	}
	if o := got.Overhead; o.DifferenceMS != 0 || o.UpperBound95MS <= 0 || o.UpperBound95MS > 1 {
		t.Fatal("overhead bound", *o)
	}
	// Hooks costing 2 ms more per key: bound above 1 ms, never subtracted from the budgets.
	runs = completeFrameFixture()
	setNullArm(runs[0].NullSamples, 2.1, 2.3)
	setNullArm(runs[0].NullOffSamples, 0.1, 0.3)
	got = Validate(runs)
	if got.Status != "fail" || got.Overhead == nil || got.Overhead.DifferenceMS < 1.99 || !strings.Contains(strings.Join(got.Reasons, ";"), "one-sided 95% upper bound exceeds1ms") {
		t.Fatal(got.Reasons, got.Overhead)
	}
	if got.WarmHandler.P95MS == nil || *got.WarmHandler.P95MS != 3 {
		t.Fatal("NULL was subtracted from raw handlers", got.WarmHandler)
	}
}

func TestOverheadBoundIsWelchOneSided95(t *testing.T) {
	// Exact one-sided 95 % Student quantiles.
	for df, want := range map[float64]float64{5: 2.015048, 10: 1.812461, 29: 1.699127, 58: 1.671553} {
		if got := studentT95(df); math.Abs(got-want) > 1e-4 {
			t.Fatal(df, got, want)
		}
	}
	on := []float64{1, 2, 1, 2}
	off := []float64{1, 1, 1, 2}
	o, ok := overheadBound(on, off)
	// means 1.5, 1.25; var 1/3, 1/4; se^2 = 1/12 + 1/16; Welch df = 5.9...
	a, b := 1.0/12, 1.0/16
	df := (a + b) * (a + b) / (a*a/3 + b*b/3)
	want := .25 + studentT95(df)*math.Sqrt(a+b)
	if !ok || o.DifferenceMS != .25 || math.Abs(o.DegreesOfFreedom-df) > 1e-12 || math.Abs(o.UpperBound95MS-want) > 1e-12 || o.NOn != 4 || o.NOff != 4 {
		t.Fatal(o, want)
	}
	if _, ok := overheadBound([]float64{1}, off); ok {
		t.Fatal("bound from one sample")
	}
}

func TestNullOffArmMustRecordOnlyItsOuterBracketInItsInterleavedSlot(t *testing.T) {
	for _, defect := range []string{"task", "handler", "start", "slot", "proof"} {
		runs := completeFrameFixture()
		s := &runs[0].NullOffSamples[0]
		switch defect {
		case "task":
			s.Tasks = []FineSpan{{Start: 102, End: 102.05}}
		case "handler":
			s.Handlers = append(s.Handlers, Handler{Event: "input", Start: 102, End: 102.1, Trusted: true})
		case "start":
			s.KeyDownAt = 101
		case "slot":
			s.ID = runs[0].NullSamples[0].ID // an on-arm slot
		case "proof":
			s.NullInserted = false
		}
		if got := Validate(runs); got.Status != "fail" {
			t.Fatal("invalid NULL-off accepted", defect)
		}
	}
	for k, want := range []string{"null", NullOff, NullOff, "null", "null", NullOff} {
		if got := nullTrialKind(k + 1); got != want {
			t.Fatal(k+1, got, want)
		}
	}
}

func TestNullControlCannotEditCodeMirrorOrMasqueradeAsNormalSamples(t *testing.T) {
	for _, defect := range []string{"changed", "cm", "length", "proof", "id"} {
		runs := completeFrameFixture()
		s := &runs[0].NullSamples[0]
		switch defect {
		case "changed":
			s.Changed = true
		case "cm":
			s.Handlers[3].Event = "codemirror-input"
		case "length":
			s.LengthAfter++
		case "proof":
			s.NullInserted = false
		case "id":
			s.ID = 1
		}
		if got := Validate(runs); got.Status != "fail" {
			t.Fatal("invalid NULL accepted", defect)
		}
	}
}

func TestPreInputMillisecondTieDoesNotHidePostInputMissingFrame(t *testing.T) {
	s := frameFixture("deferred", 1, "test", "warm")
	s.Heartbeat[0] = s.KeyDownAt
	c := boundedClock()
	if err := coverage(s, c); err == nil {
		t.Fatal("unproven missing frame accepted")
	}
	s.PreInputHeartbeatCount = 1
	if err := coverage(s, c); err != nil {
		t.Fatal("proven pre-input heartbeat treated as post-input", err)
	}
	s.Frames = s.Frames[1:]
	if err := coverage(s, c); err == nil {
		t.Fatal("post-input missing frame hidden")
	}
	s.PreInputHeartbeatCount = 2
	if err := coverage(s, c); err == nil {
		t.Fatal("post-input frame falsely labelled pre-input")
	}
}

func TestHandlerAndDeferredControlsValidateWithNonblockingBoundedClock(t *testing.T) {
	v := verifier{}
	for i, kind := range ControlKinds {
		s := frameFixture(kind, i+1, "test", "warm")
		for j := range s.Handlers {
			h := &s.Handlers[j]
			h.Fine = &FineSpan{Start: h.Start, End: h.End}
		}
		if kind == "deferred" {
			s.Heartbeat[0] = s.KeyDownAt
			s.PreInputHeartbeatCount = 1
		}
		v.addSample("warm", s, boundedClock())
	}
	for _, kind := range ControlKinds {
		if !v.controls[kind] {
			t.Fatal("control unvalidated", kind, v.report.Reasons)
		}
	}
}

func TestAttributionUnionsNestedCallbacksAndKeepsUnknownTimeExplicit(t *testing.T) {
	s := frameFixture("none", 1, "test", "warm")
	s.Tasks = []FineSpan{{Start: 102, End: 104}, {Start: 103, End: 104}}
	v := verifier{}
	v.attribute(s, 3)
	if v.attributed["recorded_app_callbacks_union"][0] != 2 || v.attributed["unattributed_harness_browser_or_queue"][0] != 1 || v.attributed["raw_handler_including_harness_and_queue"][0] != 3 {
		t.Fatal(v.attributed)
	}
}

func TestHostPaintBucketCannotPrecedeItsCorrelatedProcessingEnd(t *testing.T) {
	// Exact timestamps from Mutter host cold replicate 26. The duration-only
	// lower bound is 487, but processingEnd-1 proves no presentation before 490.
	s := rawSample()
	s.KeyDownAt = 469
	s.DocChangedAt = 490
	stamp := 475.0
	s.Handlers = []Handler{{Event: "input", Start: 476, End: 491, EventTimeStamp: &stamp, Trusted: true}}
	s.EventTiming = []Timing{{Name: "input", StartTime: 475, Duration: 16, ProcessingStart: 476, ProcessingEnd: 491}}
	if upper, err := paintBound(s); err != nil || upper != 26 {
		t.Fatal(upper, err)
	}
	s.DocChangedAt = 491
	if _, err := paintBound(s); err == nil {
		t.Fatal("unproven post-processing change accepted")
	}
}
