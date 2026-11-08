package nativeperf

import (
	"strings"
	"testing"
)

// setColdBracket gives every cold first keystroke an outer bracket of d ms.
func setColdBracket(runs []Run, d float64) {
	for n := range runs {
		if runs[n].Phase != "cold" {
			continue
		}
		for j := range runs[n].Samples[0].Handlers {
			if h := &runs[n].Samples[0].Handlers[j]; h.Event == "keystroke-task" {
				h.End, h.Fine.End = h.Start+d, h.Fine.Start+d
			}
		}
	}
}

func TestColdFirstKeystrokeHasItsOwnFiftyMillisecondBudgets(t *testing.T) {
	// 30 ms: over the warm 8 ms and 16 ms budgets, within the cold 50 ms ones.
	runs := completeFrameFixture()
	setColdBracket(runs, 30)
	got := Validate(runs)
	if got.Status != "pass" || *got.ColdHandler.P95MS != 30 || *got.ColdFrameWork.P95MS <= 16 || *got.ColdFrameWork.P95MS > 50 {
		t.Fatal(got.Reasons, got.ColdHandler.P95MS, got.ColdFrameWork.P95MS)
	}
	if got.Thresholds["cold_handler_p95_ms"] != 50 || got.Thresholds["cold_frame_work_p95_ms"] != 50 || got.Thresholds["handler_p95_ms"] != 8 || got.Thresholds["frame_work_ms"] != 16 || got.Thresholds["frame_gap_ms"] != 50 || got.Thresholds["cold_frame_gap_ms"] != 70 {
		t.Fatal(got.Thresholds)
	}
	runs = completeFrameFixture()
	setColdBracket(runs, 60)
	reasons := strings.Join(Validate(runs).Reasons, ";")
	for _, want := range []string{"cold handler p95 exceeds50ms", "cold frame work p95 exceeds50ms"} {
		if !strings.Contains(reasons, want) {
			t.Fatal(want, reasons)
		}
	}
	if strings.Contains(reasons, "warm") || strings.Contains(reasons, "exceeds16ms") {
		t.Fatal("cold samples judged by warm budgets", reasons)
	}
}

func TestWarmKeystrokesKeepEightAndSixteenMillisecondBudgets(t *testing.T) {
	runs := completeFrameFixture()
	for i := len(ControlKinds); i < len(runs[0].Samples); i++ {
		for j := range runs[0].Samples[i].Handlers {
			if h := &runs[0].Samples[i].Handlers[j]; h.Event == "keystroke-task" {
				h.End, h.Fine.End = h.Start+20, h.Fine.Start+20
			}
		}
	}
	reasons := strings.Join(Validate(runs).Reasons, ";")
	for _, want := range []string{"warm handler p95 exceeds8ms", "native per-frame work exceeds16ms", "warm frame work p95 exceeds16ms"} {
		if !strings.Contains(reasons, want) {
			t.Fatal(want, reasons)
		}
	}
}

func TestKeystrokeStartIsTheKeydownCaptureRead(t *testing.T) {
	s := rawSample()
	stamp := 95.0 // event.timeStamp 5 ms before performance.now(): a diagnostic only
	s.KeyEventTimeStamp = &stamp
	s.Handlers = append(s.Handlers, Handler{Event: "keydown", Start: s.KeyDownAt, End: 101, Trusted: true})
	if _, _, _, _, err := sampleMetrics(s); err != nil {
		t.Fatal(err)
	}
	s.Handlers[len(s.Handlers)-1].Start = s.KeyDownAt + 1
	if _, _, _, _, err := sampleMetrics(s); err == nil || !strings.Contains(err.Error(), "first clock read") {
		t.Fatal("keystroke start detached from the keydown capture read accepted", err)
	}
	// Host run 2: the keydown's timeStamp ran +22 ms ahead while the input entry
	// stayed on the performance.now() base. Paint stays bounded from KeyDownAt.
	s = rawSample()
	stamp = s.KeyDownAt + 22
	s.KeyEventTimeStamp = &stamp
	if paint, err := paintBound(s); err != nil || paint != 21 {
		t.Fatal("keydown event clock used for paint", paint, err)
	}
}

// setWarmBracket gives every warm non-control keystroke an outer bracket of d ms.
func setWarmBracket(runs []Run, d float64) {
	for i := len(ControlKinds); i < len(runs[0].Samples); i++ {
		for j := range runs[0].Samples[i].Handlers {
			if h := &runs[0].Samples[i].Handlers[j]; h.Event == "keystroke-task" {
				h.End, h.Fine.End = h.Start+d, h.Fine.Start+d
			}
		}
	}
}

func TestWarmHandlerGateIsEightMillisecondsAndFourIsANonGatingObjective(t *testing.T) {
	for _, tc := range []struct {
		d          float64
		pass, met  bool
		wantReason string
	}{
		{d: 3, pass: true, met: true},
		{d: 6, pass: true, met: false},
		{d: 9, pass: false, met: false, wantReason: "warm handler p95 exceeds8ms"},
	} {
		runs := completeFrameFixture()
		setWarmBracket(runs, tc.d)
		got := Validate(runs)
		if (got.Status == "pass") != tc.pass || got.ObjectiveMet != tc.met || got.ObjectiveWarmHandlerMS != 4 {
			t.Fatal(tc.d, got.Status, got.ObjectiveMet, got.ObjectiveWarmHandlerMS, got.Reasons)
		}
		if tc.wantReason != "" && !strings.Contains(strings.Join(got.Reasons, ";"), tc.wantReason) {
			t.Fatal(tc.d, got.Reasons)
		}
	}
}

func TestColdPauseBudgetIsSeventyWarmStaysFifty(t *testing.T) {
	// A 60 ms pause on the first keystroke (first preview presentation) passes.
	runs := completeFrameFixture()
	pause(&runs[1].Samples[0], 60)
	got := Validate(runs)
	for _, r := range got.Reasons {
		if strings.Contains(r, "gap") {
			t.Fatal("60ms cold pause failed", got.Reasons)
		}
	}
	if got.ColdMaxGapMS == nil || *got.ColdMaxGapMS != 60 || got.WarmMaxGapMS == nil || *got.WarmMaxGapMS >= 50 {
		t.Fatal(got.ColdMaxGapMS, got.WarmMaxGapMS)
	}
	// The same 60 ms pause on a warm keystroke fails.
	runs = completeFrameFixture()
	pause(&runs[0].Samples[len(ControlKinds)], 60)
	if reasons := strings.Join(Validate(runs).Reasons, ";"); !strings.Contains(reasons, "native frame gap exceeds50ms") || strings.Contains(reasons, "cold frame gap") {
		t.Fatal(reasons)
	}
}
