package nativeperf

import (
	"math"
	"strings"
	"testing"
)

func TestHandlerWorkUnionsNestedSegmentsAndKeepsCoarseUpperBound(t *testing.T) {
	s := rawSample()
	s.Handlers = []Handler{{Event: "keydown", Start: 100, End: 101}, {Event: "input", Start: 102, End: 105}, {Event: "codemirror-input", Start: 102, End: 104}}
	work, upper := handlerWork(s.Handlers, 1)
	if work != 4 || upper != 6 {
		t.Fatalf("raw=%v upper=%v; nested CM must not be counted twice", work, upper)
	}
}

func TestFrameWorkExcludesVsyncWaitAndDetectsMissedFrame(t *testing.T) {
	s := rawSample()
	s.Frames = []WorkFrame{{Start: 110, End: 111, Timestamp: 110}, {Start: 126, End: 127, Timestamp: 126}}
	s.RAF = []float64{110, 126}
	clock := Clock{ResolutionMS: 1, DeltasMS: []float64{1, 1, 1}, Rule: "upper:+quantum-per-disjoint-interval"}
	s.Baseline = []float64{0, 16, 33, 50, 66, 83}
	w, u, interval, dropped, err := frameWork(s, clock)
	if err != nil || w != 4 || u != 6 || interval != 16 || dropped {
		t.Fatalf("work=%v upper=%v interval=%v dropped=%v err=%v", w, u, interval, dropped, err)
	}
	s.Frames[1].Timestamp = 143
	if _, _, _, dropped, err = frameWork(s, clock); err != nil || !dropped {
		t.Fatal("one missed refresh went undetected", err)
	}
}

func TestFastNormalInputDoesNotRequireEventTimingEntry(t *testing.T) {
	s := rawSample()
	s.EventTiming = nil
	if _, _, _, _, err := sampleMetrics(s); err != nil {
		t.Fatal("censored normal event is not an oracle failure", err)
	}
	s.Control.Kind = "layout"
	if _, _, _, _, err := sampleMetrics(s); err == nil {
		t.Fatal("missing slow-control Event Timing must fail")
	}
}

func TestRawCollectionContinuesDespitePerformanceBudgetFailure(t *testing.T) {
	s := rawSample()
	s.Handlers[0].End = 109
	s.Phase = "cold"
	run := Run{Phase: "cold", ReplicateCount: ColdReplicates, ReplicateIndex: 1, RunID: s.RunID, Samples: []Sample{s}}
	if reasons := collectedFailures(run); len(reasons) != 0 {
		t.Fatal("raw collection must not abort cold replication because a budget failed", reasons)
	}
}

// probeSpanAcrossKey gives the warm keystroke s one task-probe span from
// key+from to key+to; the spans before it end where it starts.
func probeSpanAcrossKey(s *Sample, from, to float64) {
	start, end := s.KeyDownAt+from, s.KeyDownAt+to
	busy := []FineSpan{}
	for _, span := range s.Busy {
		switch {
		case span.End <= start:
			busy = append(busy, span)
		case span.Start < start:
			busy = append(busy, FineSpan{Start: span.Start, End: start})
		}
	}
	busy = append(busy, FineSpan{Start: start, End: end})
	for at := end; at < s.SettledAt; at += 8 {
		busy = append(busy, FineSpan{Start: at, End: min(at+8, s.SettledAt)})
	}
	s.Busy = busy
}

func TestProbeIdleBeforeTheKeyIsNotFrameWork(t *testing.T) {
	runs := completeFrameFixture()
	s := &runs[0].Samples[len(ControlKinds)]
	// The 2026-11-08 final-run-1 trial 12 shape: the probe's 8 ms timer was
	// armed 7 ms before the key; the key task, the key frame and the amSync
	// write kept the main thread until +11 ms.
	probeSpanAcrossKey(s, -7, 11)
	c := runs[0].Ready.Clock
	work, upper, _, _, err := frameEvidence(*s, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := 11.0; math.Abs(work-want) > 1e-9 || math.Abs(upper-(want+c.ResolutionMS)) > 1e-9 {
		t.Fatalf("work=%v upper=%v; want %v and %v + quantum: the 7 ms before the key are the probe's idle wait", work, upper, want, want)
	}
	if got := Validate(runs); got.Status != "pass" {
		t.Fatal("an 11 ms post-key span failed the 16 ms frame budget", got.Reasons)
	}
}

func TestProbeSpanAcrossTheKeyStillCountsWorkAfterIt(t *testing.T) {
	runs := completeFrameFixture()
	s := &runs[0].Samples[len(ControlKinds)]
	// Same pre-key idle, but the main thread stays busy until +17 ms.
	probeSpanAcrossKey(s, -7, 17)
	got := Validate(runs)
	if got.Status != "fail" || !strings.Contains(strings.Join(got.Reasons, ";"), "per-frame work exceeds16ms") {
		t.Fatal("17 ms of post-key main-thread work escaped the frame budget", got.Reasons)
	}
}

// Real probe spans rarely start at the control: the probe's timer is armed
// before the key. Clipping at the key must keep every control placement.
func TestEveryControlIsDetectedWhenItsProbeSpanStartsBeforeTheKey(t *testing.T) {
	runs := completeFrameFixture()
	for index, kind := range ControlKinds {
		s := &runs[0].Samples[index]
		covering := FineSpan{}
		for _, span := range s.Busy {
			if span.Start <= s.Control.Start && span.End >= s.Control.End {
				covering = span
			}
		}
		if covering.End == 0 {
			t.Fatal("fixture has no probe span over the control", kind)
		}
		probeSpanAcrossKey(s, -7, covering.End-s.KeyDownAt)
	}
	got := Validate(runs)
	if got.Status != "pass" {
		t.Fatal(got.Reasons)
	}
	if len(got.Controls) != len(ControlKinds) {
		t.Fatal("missing control records", got.Controls)
	}
	for _, control := range got.Controls {
		if !control.Validated || control.FrameWorkMS <= 16 {
			t.Fatalf("%s control lost: validated=%v frame work %v", control.Kind, control.Validated, control.FrameWorkMS)
		}
	}
}
