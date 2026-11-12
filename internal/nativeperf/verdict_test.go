package nativeperf

import "testing"

func TestUnsupportedOrFabricatedNativeEvidenceFails(t *testing.T) {
	if got := Validate(nil); got.Status != "fail" {
		t.Fatal("missing evidence passed")
	}
	if _, err := DecodeRun([]byte(`{"protocol":1,"p95_ms":0}`)); err == nil {
		t.Fatal("fabricated aggregate accepted")
	}
	run := Run{Protocol: 1, RunID: "test", Phase: "warm", Host: Host{Engine: "Chromium", Native: false}}
	if got := Validate([]Run{run}); got.Status != "fail" {
		t.Fatal("non-native evidence passed")
	}
}

func rawSample() Sample {
	return Sample{Type: "sample", ID: 1, RunID: "native", Phase: "cold", Handlers: []Handler{{Event: "input", Start: 102, End: 105, Trusted: true, EventTimeStamp: func() *float64 { v := 101.0; return &v }()}, {Event: "codemirror-input", Start: 102, End: 105, Trusted: true}}, KeyDownAt: 100, DocChangedAt: 104, RevisionBefore: "a", RevisionAfter: "b", LengthBefore: 1, LengthAfter: 2, RAF: []float64{110, 126}, Heartbeat: []float64{98, 110, 126}, EventTiming: []Timing{{Name: "input", StartTime: 101, Duration: 16, ProcessingStart: 102, ProcessingEnd: 105}}, Control: Control{Kind: "none"}, Visible: true, Focused: true, Trusted: true, Changed: true}
}

func TestPaintBoundsUseInputOriginAndQuantization(t *testing.T) {
	s := rawSample()
	paint, err := paintBound(s)
	if err != nil {
		t.Fatal(err)
	}
	if paint != 21 {
		t.Fatalf("paint upper bound %.1f, expected start101+duration16+rounding4-key100=21", paint)
	}
	s.EventTiming = nil
	if _, err := paintBound(s); err == nil {
		t.Fatal("rAF was accepted as paint")
	}
	s = rawSample()
	s.EventTiming[0].Duration = 8
	if _, err := paintBound(s); err == nil {
		t.Fatal("uncensored fabricated low duration accepted")
	}
}

func TestVisibilityAndControlOraclesAreIndependent(t *testing.T) {
	s := rawSample()
	s.Visible = false
	if _, _, _, _, err := sampleMetrics(s); err == nil {
		t.Fatal("hidden sample accepted")
	}
	s = rawSample()
	s.Control = Control{Kind: "layout", Start: 110, End: 190, Iterations: 100}
	s.RAF = []float64{110, 200}
	s.Heartbeat = []float64{98, 110, 200}
	s.EventTiming[0].Duration = 104
	s.Frames = []WorkFrame{{Start: 110, End: 192, Timestamp: 110}, {Start: 200, End: 201, Timestamp: 200}}
	h, _, p, g, err := sampleMetrics(s)
	if err != nil {
		t.Fatal(err)
	}
	handler, layout := validateControl(s, h, p, g)
	if handler || !layout {
		t.Fatal("post-handler layout control oracle failed")
	}
	s.EventTiming = nil
	if _, _, _, _, err := sampleMetrics(s); err == nil {
		t.Fatal("slow layout without paint oracle passed")
	}
}

func TestMutationObserverWorkCanFollowDOMProcessingBeforePaint(t *testing.T) {
	s := rawSample()
	s.DocChangedAt = 109
	s.Handlers[1].Start = 108
	s.Handlers[1].End = 110
	if _, err := paintBound(s); err != nil {
		t.Fatal("verified CM change before earliest paint wrongly rejected", err)
	}
	s.DocChangedAt = 114
	s.Handlers[1].Start = 113
	s.Handlers[1].End = 115
	s.RAF = []float64{120, 136}
	s.Heartbeat = []float64{98, 110, 126, 142}
	if _, err := paintBound(s); err == nil {
		t.Fatal("uncertain post-paint CM change accepted")
	}
}

func TestIndependentRawMetricsRemainWhenPaintUnknown(t *testing.T) {
	s := rawSample()
	s.EventTiming = nil
	run := Run{Protocol: 1, RunID: "native", PID: 1, StartedNS: 1, Phase: "cold", ReplicateIndex: 1, ReplicateCount: ColdReplicates, Host: Host{Native: true, Engine: "WebKitGTK", GOOS: "linux", WebKitVersion: "2.52", GTKVersion: "4.22"}, Ready: Ready{Type: "ready", Visible: true, Focused: true, PaintOracle: "unavailable"}, Samples: []Sample{s}}
	report := Validate([]Run{run})
	if report.Status != "fail" || report.Paint.P95MS != nil || report.Handler.P95MS == nil || report.FrameWait.P95MS == nil {
		t.Fatal("missing paint was fabricated or independent raw metrics discarded")
	}
	if *report.Handler.P95MS != 3 || *report.FrameWait.P95MS != 10 {
		t.Fatal("raw input/frame boundaries were not recomputed")
	}
}

func TestDuplicateTrialsAndRevisionsFail(t *testing.T) {
	s := rawSample()
	failures := []string{}
	run := Run{Phase: "warm", Samples: []Sample{s, s}}
	collect := func(reason string) { failures = append(failures, reason) }
	validateRunShape(run, collect, collect)
	if len(failures) == 0 {
		t.Fatal("duplicate raw trial accepted")
	}
}

func TestControlRejectsUnrelatedHeartbeatGap(t *testing.T) {
	s := rawSample()
	s.Control = Control{Kind: "layout", Start: 110, End: 190, Iterations: 100}
	s.RAF = []float64{110, 200}
	s.EventTiming[0].Duration = 104
	// The only >=50ms gap occurs before the control. Regular callbacks inside
	// the claimed blocking interval cannot validate the control's gap oracle.
	s.Heartbeat = []float64{0, 60, 100, 110, 125, 140, 155, 170, 185, 200}
	h, _, p, g, err := sampleMetrics(s)
	if err != nil {
		t.Fatal(err)
	}
	if handler, layout := validateControl(s, h, p, g); handler || layout {
		t.Fatal("unrelated heartbeat gap validated blocking layout control")
	}
}
