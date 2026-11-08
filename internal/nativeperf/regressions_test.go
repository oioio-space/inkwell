package nativeperf

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestFineClockResolvesTheEightMillisecondBoundaryWithoutRoundingAwayNine(t *testing.T) {
	for _, elapsed := range []float64{7.8, 8.2, 9} {
		runs := completeFrameFixture()
		for n := range runs {
			runs[n].Ready.Clock.ResolutionMS = 1
			runs[n].Ready.Clock.DeltasMS = []float64{1, 1, 1}
			for i := range runs[n].Samples {
				s := &runs[n].Samples[i]
				if s.Control.Kind != "none" {
					continue
				}
				s.Handlers[0].End = 110 // Same coarse sample for 7.8/8.2/9 ms.
				s.Handlers[0].Fine.End = s.Handlers[0].Fine.Start + elapsed
				for j := range s.Handlers {
					if s.Handlers[j].Event == "keystroke-task" {
						s.Handlers[j].End = 110
						s.Handlers[j].Fine.End = s.Handlers[j].Fine.Start + elapsed
					}
				}
			}
		}
		got := Validate(runs)
		if (got.Status == "pass") != (elapsed < 8) || got.Handler.P95MS == nil || math.Abs(*got.Handler.P95MS-elapsed) > .000001 {
			t.Fatal("fine boundary was replaced by coarse rounding", elapsed, got.Reasons)
		}
	}
}

func TestRealHostControlsCorrelateWithActualInput(t *testing.T) {
	// Replay the unmodified failed host artifact, not a hand-written oracle.
	data, err := os.ReadFile("testdata/webkit-2.54.1-controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var samples []Sample
	if err = json.Unmarshal(data, &samples); err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 {
		t.Fatal("host controls missing")
	}
	for _, sample := range samples {
		bound, err := paintBound(sample)
		if err != nil || bound-8 < 50 || sample.KeyDownAt+bound < sample.Control.End {
			t.Fatal("actual slow control Event Timing not corroborated", sample.Control.Kind, bound, err)
		}
	}
}

func TestEventTimingCorrelationAllowsOnlyOneBrowserQuantum(t *testing.T) {
	s := rawSample()
	s.EventTiming[0].ProcessingStart = 103
	s.EventTiming[0].ProcessingEnd = 104
	if _, err := paintBound(s); err != nil {
		t.Fatal("1 ms coarsening rejected", err)
	}
	s.EventTiming[0].StartTime += 3
	if _, err := paintBound(s); err == nil {
		t.Fatal("unrelated event accepted")
	}
}

func TestFineClockDoesNotHideNineMillisecondHandler(t *testing.T) {
	runs := completeFrameFixture()
	for n := range runs {
		for i := range runs[n].Samples {
			s := &runs[n].Samples[i]
			if s.Control.Kind != "none" {
				continue
			}
			for j := range s.Handlers {
				s.Handlers[j].End = s.Handlers[j].Start + 9
				s.Handlers[j].Fine.End = s.Handlers[j].Fine.Start + 9
			}
		}
	}
	got := Validate(runs)
	if got.Status != "fail" || got.Handler.P95MS == nil || *got.Handler.P95MS < 9 {
		t.Fatal("real 9ms handler hidden", got.Reasons)
	}
}

func TestSingleOverBudgetFrameFailsEvenWhenP95Passes(t *testing.T) {
	runs := completeFrameFixture()
	runs[0].Samples[len(ControlKinds)].Frames[0].End = 124 // handler 3 + frame 14 > 16
	got := Validate(runs)
	if got.Status != "fail" || got.FrameWorkUpper.P95MS == nil || *got.FrameWorkUpper.P95MS > 16 || !strings.Contains(strings.Join(got.Reasons, ";"), "per-frame work exceeds16ms") {
		t.Fatal("one slow frame hidden by p95", got.Reasons)
	}
}

func TestReasonsAreDeduplicatedAcrossRawRunsAndRevalidation(t *testing.T) {
	runs := completeFrameFixture()
	for i := range runs {
		runs[i].Failures = []string{"same failure", "same failure"}
	}
	got := Validate(runs)
	if got.Status != "fail" {
		t.Fatal("raw rejection disappeared")
	}
	seen := map[string]bool{}
	for _, reason := range got.Reasons {
		if seen[reason] {
			t.Fatal("duplicate reason", reason)
		}
		seen[reason] = true
	}
}

func TestMissingClockOrSlowControlNeverPasses(t *testing.T) {
	for _, defect := range []string{"clock", "layout-timing", "layout-frame", "layout-cadence"} {
		runs := completeFrameFixture()
		switch defect {
		case "clock":
			runs[0].Ready.Clock.Precise = nil
		case "layout-timing":
			runs[0].Samples[1].EventTiming = nil
		case "layout-frame":
			runs[0].Samples[1].Frames[0].End = 112
		case "layout-cadence":
			runs[0].Samples[1].Baseline = nil
		}
		if got := Validate(runs); got.Status != "fail" {
			t.Fatal(defect, "passed")
		}
	}
}
