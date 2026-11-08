package nativeperf

import (
	"fmt"
	"testing"
)

func frameFixture(kind string, id int, runID, phase string) Sample {
	s := rawSample()
	s.ID = id
	s.RunID = runID
	s.Phase = phase
	s.RevisionBefore = fmt.Sprint("before", id)
	s.RevisionAfter = fmt.Sprint("after", id)
	s.Baseline = []float64{0, 16, 32, 48, 64, 80}
	s.EventTiming = nil
	s.Control.Kind = kind
	s.Frames = []WorkFrame{{Start: 110, End: 110.5, Timestamp: 110}, {Start: 126, End: 126.5, Timestamp: 126}}
	if kind == "handler" || kind == "document" {
		s.Control = Control{Kind: kind, Start: 102, End: 182, Iterations: 10}
		s.Handlers[0].End = 185
		s.Handlers[1].Start = 182
		s.Handlers[1].End = 185
		s.DocChangedAt = 184
		s.RAF = []float64{190, 206}
		s.Heartbeat = []float64{98, 190, 206}
		s.Frames = []WorkFrame{{Start: 190, End: 190.5, Timestamp: 190}, {Start: 206, End: 206.5, Timestamp: 206}}
		s.EventTiming = []Timing{{Name: "input", StartTime: 101, Duration: 96, ProcessingStart: 102, ProcessingEnd: 185}}
	}
	if kind == "layout" || kind == "measure" {
		s.Control = Control{Kind: kind, Start: 110, End: 190, Iterations: 10}
		s.Handlers[0].End = 109
		s.Handlers[1].End = 108
		s.DocChangedAt = 108
		s.RAF = []float64{110, 206}
		s.Heartbeat = []float64{98, 110, 206}
		s.Frames = []WorkFrame{{Start: 110, End: 192, Timestamp: 110}, {Start: 206, End: 206.5, Timestamp: 206}}
		s.EventTiming = []Timing{{Name: "input", StartTime: 101, Duration: 104, ProcessingStart: 102, ProcessingEnd: 109}}
	}
	if kind == "early-raf" {
		s.Control = Control{Kind: kind, Start: 109, End: 189, Iterations: 10}
		s.RAF = []float64{190, 206}
		s.Heartbeat = []float64{98, 190, 206}
		s.Frames = []WorkFrame{{Start: 190, End: 190.5, Timestamp: 190}, {Start: 206, End: 206.5, Timestamp: 206}}
		s.EventTiming = []Timing{{Name: "input", StartTime: 101, Duration: 104, ProcessingStart: 102, ProcessingEnd: 105}}
	}
	if kind == "task" {
		s.Control = Control{Kind: kind, Start: 106, End: 136, Iterations: 10}
		s.RAF = []float64{142, 158}
		s.Heartbeat = []float64{98, 142, 158}
		s.Frames = []WorkFrame{{Start: 142, End: 142.5, Timestamp: 142}, {Start: 158, End: 158.5, Timestamp: 158}}
	}
	if kind == "deferred" {
		s.Control = Control{Kind: kind, Start: 400, End: 430, Iterations: 10}
	}
	for at := s.Heartbeat[len(s.Heartbeat)-1] + 16; at <= 1410; at += 16 {
		if kind == "deferred" && at > 400 && at < 430 {
			continue
		}
		s.Heartbeat = append(s.Heartbeat, at)
		s.Frames = append(s.Frames, WorkFrame{Start: at, End: at + .5, Timestamp: at})
	}
	s.SettledAt = s.Frames[len(s.Frames)-1].End + .5
	for at := 98.0; at < s.SettledAt; {
		end := at + 4
		if kind != "none" && at < s.Control.Start && end > s.Control.Start {
			end = s.Control.Start
		}
		if kind != "none" && at == s.Control.Start {
			end = s.Control.End + 1
		}
		if end > s.SettledAt {
			end = s.SettledAt
		}
		s.Busy = append(s.Busy, FineSpan{Start: at, End: end})
		at = end
	}
	for i := range s.Handlers {
		h := &s.Handlers[i]
		h.Fine = &FineSpan{Start: h.Start + 1000, End: h.End + 1000}
	}
	first, last := s.Handlers[0].Start, s.Handlers[0].End
	for _, h := range s.Handlers {
		if h.Start < first {
			first = h.Start
		}
		if h.End > last {
			last = h.End
		}
	}
	s.Handlers = append(s.Handlers, Handler{Event: "keystroke-task", Start: first, End: last, Fine: &FineSpan{Start: first + 1000, End: last + 1000}, Trusted: true})
	s.Tasks = []FineSpan{{Start: first, End: last}}
	return s
}

func completeFrameFixture() []Run {
	runs := []Run{}
	for n := 0; n <= 30; n++ {
		id := fmt.Sprint("run", n)
		run := Run{
			Protocol: 1, RunID: id, PID: 100 + n, StartedNS: int64(1000 + n), Phase: "cold", ReplicateIndex: n, ReplicateCount: 30,
			Host:  Host{Engine: "WebKitGTK", Native: true, GOOS: "linux", WebKitVersion: "2.54.1", GTKVersion: "4.22.5", Backend: "x11", XID: uint64(100 + n), RefreshHz: 60},
			Ready: Ready{Type: "ready", Visible: true, Focused: true, PaintOracle: "event-timing", EarlyCapture: true, InputVerified: true, Clock: Clock{RefreshHz: 60, ResolutionMS: .1, DeltasMS: []float64{.1, .1, .1}, Rule: "upper:+quantum-per-disjoint-interval"}},
		}
		run.Ready.Clock.Precise = &PreciseClock{Source: "native-monotonic", ResolutionMS: .000001, ReadSpansMS: []float64{0, 1, 0, 1}}
		run.Host.InputPermissionVerified = true
		if n == 0 {
			run.Phase = "warm"
			for k := 1; k <= 30+len(ControlKinds); k++ {
				kind := "none"
				if k <= len(ControlKinds) {
					kind = ControlKinds[k-1]
				}
				run.Samples = append(run.Samples, frameFixture(kind, k, id, "warm"))
			}
			for k := 1; k <= 2*NullCount; k++ {
				if nullTrialKind(k) == NullOff {
					run.NullOffSamples = append(run.NullOffSamples, nullOffFixture(len(run.Samples)+k, id))
				} else {
					run.NullSamples = append(run.NullSamples, nullFixture(len(run.Samples)+k, id))
				}
			}
		} else {
			run.Samples = []Sample{frameFixture("none", 1, id, "cold")}
		}
		runs = append(runs, run)
	}
	return runs
}

func nullFixture(id int, runID string) Sample {
	s := frameFixture("none", id, runID, "warm")
	s.Control.Kind = "null"
	s.NullInserted, s.Changed = true, false
	s.LengthAfter, s.RevisionAfter = s.LengthBefore, s.RevisionBefore
	s.KeyDownAt = 102
	s.DocChangedAt = 102.05
	s.Tasks = nil
	s.Handlers = nil
	for _, event := range []string{"keydown", "beforeinput", "input", "null-input", "keystroke-task"} {
		s.Handlers = append(s.Handlers, Handler{Event: event, Start: 102, End: 102.1, Trusted: true, Fine: &FineSpan{Start: 1102, End: 1102.1}})
	}
	return s
}

// nullOffFixture: the same NULL key with every recording hook off; the outer
// keystroke bracket is its only record.
func nullOffFixture(id int, runID string) Sample {
	s := nullFixture(id, runID)
	s.Control.Kind = NullOff
	s.DocChangedAt, s.RAF = 0, nil
	s.Handlers = []Handler{{Event: "keystroke-task", Start: 102, End: 102.1, Trusted: true, Fine: &FineSpan{Start: 1102, End: 1102.1}}}
	return s
}

func TestCompleteFrameVerdictAcceptsCensoredFastEventsOnlyWithBothValidatedControls(t *testing.T) {
	runs := completeFrameFixture()
	got := Validate(runs)
	if got.Status != "pass" {
		t.Fatal(got.Reasons)
	}
	if got.Paint.P95MS != nil || got.FrameWorkUpper.P95MS == nil || len(got.ColdHandler.SamplesMS) != 30 {
		t.Fatal("censored paint invented or cold/frame evidence lost")
	}
	runs[0].Samples[1].Frames[0].End = 112
	if Validate(runs).Status != "fail" {
		t.Fatal("missed layout oracle passed")
	}
}

func TestQuantizedHandlerNeverGetsAnExtraMillisecondOfBudget(t *testing.T) {
	runs := completeFrameFixture()
	for i := range runs[0].Samples {
		if i >= 2 {
			s := &runs[0].Samples[i]
			s.Handlers[0].End = 106
			s.Handlers[1].End = 106
		}
	}
	got := Validate(runs)
	if got.Status != "fail" || got.HandlerUpper.P95MS == nil || *got.HandlerUpper.P95MS <= 4 {
		t.Fatal("coarsening loosened the 4 ms budget", got.Reasons)
	}
}

func TestVsyncWaitIsNotAWorkBudgetAndDroppedFirstFrameStillFails(t *testing.T) {
	runs := completeFrameFixture()
	for n := range runs {
		for i := range runs[n].Samples {
			s := &runs[n].Samples[i]
			if s.Control.Kind == "none" {
				s.KeyDownAt = 80
				s.Heartbeat = []float64{79, 95, 110, 126}
				s.Baseline = []float64{0, 15, 31, 47, 63, 79}
				prefix := []FineSpan{}
				for at := 78.0; at < 98; at += 4 {
					prefix = append(prefix, FineSpan{Start: at, End: at + 4})
				}
				s.Busy = append(prefix, s.Busy...)
				s.Frames = append([]WorkFrame{{Start: 95, End: 95.5, Timestamp: 95}}, s.Frames...)
				// The settle tail is still required when testing a 30ms vsync wait.
				for at := 142.0; at < s.SettledAt; at += 16 {
					s.Heartbeat = append(s.Heartbeat, at)
				}
			}
		}
	}
	got := Validate(runs)
	if got.Status != "pass" || *got.FrameWait.P95MS != 30 {
		t.Fatal("idle frame waiting was charged as work", got.Reasons)
	}
	runs[1].Samples[0].Heartbeat = []float64{78, 110, 126}
	if Validate(runs).Status != "fail" {
		t.Fatal("missing frame before first input rendering was hidden")
	}
}
