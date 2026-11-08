package nativeperf

import (
	"strings"
	"testing"
)

func TestCadenceRejectsStalePostInputAndSlowStartupCalibration(t *testing.T) {
	c := completeFrameFixture()[1].Ready.Clock
	for _, baseline := range [][]float64{{0, 10, 20, 30, 40, 50}, {20, 36, 52, 68, 84, 100}} {
		s := frameFixture("none", 1, "test", "cold")
		s.Baseline = baseline
		if _, _, _, _, err := frameWork(s, c); err == nil {
			t.Fatal("stale/post-input cadence accepted", baseline)
		}
	}
	s := frameFixture("none", 1, "test", "cold")
	s.KeyDownAt = 200
	s.Baseline = []float64{0, 33, 66, 99, 132, 165}
	if _, _, _, _, err := frameWork(s, c); err == nil || !strings.Contains(err.Error(), "nominal GdkMonitor") {
		t.Fatal("33ms startup hid a 60Hz missed refresh", err)
	}
}

func TestDeferredThirtyMillisecondWorkFailsAfterSecondFrame(t *testing.T) {
	runs := completeFrameFixture()
	// A warm keystroke: the per-frame 16 ms budget is the warm one (cold has p95 <= 50).
	s := &runs[0].Samples[len(ControlKinds)]
	busy := []FineSpan{}
	for at := 98.0; at < s.SettledAt; {
		end := at + 4
		if at < 400 && end > 400 {
			end = 400
		}
		if at == 400 {
			end = 430
		}
		if end > s.SettledAt {
			end = s.SettledAt
		}
		busy = append(busy, FineSpan{Start: at, End: end})
		at = end
	}
	s.Busy = busy
	heartbeat := []float64{}
	frames := []WorkFrame{}
	for _, p := range s.Heartbeat {
		if p <= 400 || p >= 430 {
			heartbeat = append(heartbeat, p)
		}
	}
	for _, f := range s.Frames {
		if f.Start <= 400 || f.Start >= 430 {
			frames = append(frames, f)
		}
	}
	s.Heartbeat = heartbeat
	s.Frames = frames
	got := Validate(runs)
	if got.Status != "fail" || got.FrameWorkUpper.P95MS == nil || got.DroppedFrames == 0 || !strings.Contains(strings.Join(got.Reasons, ";"), "per-frame work exceeds16ms") {
		t.Fatal("late task escaped", got.Reasons)
	}
}

func TestTwoShortCallbacksCannotHideAnOverBudgetFrame(t *testing.T) {
	runs := completeFrameFixture()
	runs[0].Ready.Clock.ResolutionMS = 1
	runs[0].Ready.Clock.DeltasMS = []float64{1, 1, 1}
	// A warm keystroke: the per-frame 16 ms budget is the warm one (cold has p95 <= 50).
	s := &runs[0].Samples[len(ControlKinds)]
	s.Tasks = append(s.Tasks, FineSpan{Start: 110.6, End: 119.1}, FineSpan{Start: 119.1, End: 127.6})
	s.Frames[1] = WorkFrame{Start: 127.7, End: 128.2, Timestamp: 127.7}
	s.RAF[1] = 127.7
	s.Heartbeat[2] = 127.7
	s.Busy = nil
	for at := 98.0; at < s.SettledAt; {
		end := at + 4
		if at < 110.6 && end > 110.6 {
			end = 110.6
		}
		switch at {
		case 110.6:
			end = 119.1
		case 119.1:
			end = 127.6
		}
		if end > s.SettledAt {
			end = s.SettledAt
		}
		s.Busy = append(s.Busy, FineSpan{Start: at, End: end})
		at = end
	}
	got := Validate(runs)
	if got.Status != "fail" || got.DroppedFrames != 0 || !strings.Contains(strings.Join(got.Reasons, ";"), "per-frame work exceeds16ms") {
		t.Fatal("8.5ms + 8.5ms callbacks were checked individually", got.Reasons, got.DroppedFrames)
	}
}

func TestEveryIndependentControlPlacementMustBeCovered(t *testing.T) {
	for index, kind := range ControlKinds {
		runs := completeFrameFixture()
		s := &runs[0].Samples[index]
		if kind == "handler" {
			for i := range s.Handlers {
				if s.Handlers[i].Event == "keystroke-task" {
					s.Handlers[i].End = s.Handlers[i].Start + 1
					s.Handlers[i].Fine.End = s.Handlers[i].Fine.Start + 1
				}
			}
		}
		for i := range s.Frames {
			if s.Frames[i].Start <= s.Control.Start && s.Frames[i].End >= s.Control.End {
				s.Frames[i].End = s.Frames[i].Start + .5
			}
		}
		spans := []FineSpan{}
		for _, f := range s.Busy {
			if f.Start <= s.Control.Start && f.End >= s.Control.End {
				for at := f.Start; at < f.End; {
					end := at + 1
					if end > f.End {
						end = f.End
					}
					spans = append(spans, FineSpan{Start: at, End: end})
					at = end
				}
			} else {
				spans = append(spans, f)
			}
		}
		s.Busy = spans
		tasks := []FineSpan{}
		for _, span := range s.Tasks {
			if !(span.Start <= s.Control.Start && span.End >= s.Control.End) {
				tasks = append(tasks, span)
			}
		}
		s.Tasks = tasks
		got := Validate(runs)
		if got.Status != "fail" {
			t.Fatal("uncovered placement passed", kind)
		}
		found := false
		for _, control := range got.Controls {
			if control.Kind == kind {
				found = true
				if control.Validated {
					t.Fatal("uncovered control marked validated", kind)
				}
			}
		}
		if !found {
			t.Fatal("missing control record", kind)
		}
	}
}

func TestPauseUsesUpperBoundAndControlUsesLowerBound(t *testing.T) {
	runs := completeFrameFixture()
	runs[1].Ready.Clock.ResolutionMS = 1
	runs[1].Ready.Clock.DeltasMS = []float64{1, 1, 1}
	// Cold (first keystroke, first preview presentation): raw 70 ms at q=1 is
	// 71 ms upper, over the cold 70 ms pause budget.
	pause(&runs[1].Samples[0], 70)
	got := Validate(runs)
	if got.ColdMaxGapMS == nil || *got.ColdMaxGapMS != 70 || got.ColdMaxGapUpper == nil || *got.ColdMaxGapUpper != 71 || !strings.Contains(strings.Join(got.Reasons, ";"), "native cold frame gap exceeds70ms") {
		t.Fatal("coarse 70ms cold pause passed", got.Reasons)
	}
	// Warm keeps "no pause > 50 ms": raw 50 ms is 50.1 ms upper at q=0.1.
	runs = completeFrameFixture()
	pause(&runs[0].Samples[len(ControlKinds)], 50)
	got = Validate(runs)
	if got.WarmMaxGapMS == nil || *got.WarmMaxGapMS != 50 || got.WarmMaxGapUpper == nil || *got.WarmMaxGapUpper != 50.1 || !strings.Contains(strings.Join(got.Reasons, ";"), "native frame gap exceeds50ms") {
		t.Fatal("50ms warm pause passed", got.Reasons)
	}
	c := Control{Start: 110, End: 140}
	if controlGapLower([]float64{100, 150}, c, 1, 50) {
		t.Fatal("raw 50ms gap cannot prove >=50ms at q=1")
	}
}

func TestClockAgreementIncludesOnlyBoundedTransportAndNeverRaisesBudget(t *testing.T) {
	c := Clock{ResolutionMS: 1, Precise: &PreciseClock{Source: "native-monotonic", ResolutionMS: .000001, ReadSpansMS: []float64{2, 1, 2, 1}}}
	h := []Handler{{Event: "keystroke-task", Start: 10, End: 15, Fine: &FineSpan{Start: 100, End: 103}}}
	raw, upper, err := measuredHandlerWork(h, c)
	if err != nil || raw != 3 || upper > 4 {
		t.Fatal("systematic XHR offset was treated as clock drift", raw, upper, err)
	}
	h[0].End = 17
	h[0].Fine.End = 105
	if raw, upper, err = measuredHandlerWork(h, c); err != nil || raw != 5 || upper <= 4 {
		t.Fatal("transport allowance hid a real 5ms handler", raw, upper, err)
	}
	c.Precise.ReadSpansMS = []float64{4, 4, 4, 4}
	if _, _, err = measuredHandlerWork(h, c); err == nil {
		t.Fatal("unbounded preflight transport passed")
	}
}

// pause puts one heartbeat gap of d ms after the keystroke, then a 16 ms cadence.
func pause(s *Sample, d float64) {
	s.Heartbeat = []float64{98, 110, 110 + d}
	for at := 110 + d + 16; at < s.SettledAt; at += 16 {
		s.Heartbeat = append(s.Heartbeat, at)
	}
}
