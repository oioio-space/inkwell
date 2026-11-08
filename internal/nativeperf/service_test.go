package nativeperf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fakeConn struct {
	ctx     context.Context
	in, out chan []byte
}

func (c *fakeConn) Context() context.Context { return c.ctx }
func (c *fakeConn) Send(b []byte) error      { c.out <- b; return nil }
func (c *fakeConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}
func sendFrame(c *fakeConn, value any) { b, _ := json.Marshal(value); c.in <- b }
func TestReceiverRecordsOnlyArmedNativeRawInput(t *testing.T) {
	c := &fakeConn{ctx: t.Context(), in: make(chan []byte, 8), out: make(chan []byte, 8)}
	markers := make(chan map[string]any, 8)
	out := filepath.Join(t.TempDir(), "cold.json")
	host := func() (Host, bool, bool) {
		return Host{Engine: "WebKitGTK", Native: true, GOOS: "linux", WebKitVersion: "2.52.0", GTKVersion: "4.22.0", InputPermissionVerified: true}, true, true
	}
	service, err := New(Config{Phase: "cold", Count: 1, Index: 1, Replicates: 30, RunID: "native", Output: out, Host: host, Marker: func(v any) { markers <- v.(map[string]any) }})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); service.Serve(c) }()
	sendFrame(c, map[string]any{"type": "hello", "protocol": 1, "challenge": "0123456789abcdef0123456789abcdef"})
	var capability map[string]any
	if err := json.Unmarshal(<-c.out, &capability); err != nil {
		t.Fatal(err)
	}
	if capability["challenge"] != "0123456789abcdef0123456789abcdef" || capability["native"] != true {
		t.Fatal("native capability mismatch")
	}
	sendFrame(c, Ready{Type: "ready", Visible: true, Focused: true, UserAgent: "WebKit", PaintOracle: "unavailable", InputVerified: true})
	var trial map[string]any
	_ = json.Unmarshal(<-c.out, &trial)
	if trial["id"] != float64(1) || trial["control"] != "none" {
		t.Fatal("cold was not first input before controls")
	}
	sendFrame(c, armed{Type: "armed", ID: 1, RunID: "native", Visible: true, Focused: true})
	if marker := <-markers; marker["type"] != "armed" || marker["visible"] != true {
		t.Fatal("driver armed before native focus verification")
	}
	sendFrame(c, rawSample())
	if marker := <-markers; marker["type"] != "sample" || marker["phase"] != "cold" || marker["control"] != "none" {
		t.Fatal("driver sample acknowledgement missing identity")
	}
	<-markers
	<-done
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	run, err := DecodeRun(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Samples) != 1 || run.Host.Engine != "WebKitGTK" {
		t.Fatal("raw native record missing")
	}
	if Validate([]Run{run}).Status != "fail" {
		t.Fatal("unsupported paint oracle passed")
	}
}

func TestWarmTrialOrderingAndNonNativeRejection(t *testing.T) {
	if _, err := New(Config{Phase: "warm", Count: 30, Output: "unused", Host: func() (Host, bool, bool) { return Host{Engine: "Chromium"}, true, true }}); err == nil {
		t.Fatal("server engine accepted")
	}
	s := &Service{cfg: Config{Phase: "warm", Count: 30}}
	for i, want := range append(append([]string{}, ControlKinds...), "none") {
		if got := s.kind(i + 1); got != want {
			t.Fatalf("trial%d=%s", i+1, got)
		}
	}
	if s.total() != 30+len(ControlKinds)+2*NullCount {
		t.Fatal("missing warm raw inputs after controls")
	}
	arms := map[string]int{}
	for id := 30 + len(ControlKinds) + 1; id <= s.total(); id++ {
		arms[s.kind(id)]++
	}
	first := 30 + len(ControlKinds) + 1
	if arms["null"] != NullCount || arms[NullOff] != NullCount || s.kind(first) != "null" || s.kind(first+1) != NullOff || s.kind(first+2) != NullOff || s.kind(first+3) != "null" {
		t.Fatal("NULL arms missing, unbalanced or not interleaved", arms)
	}
}

func TestWarmPermissionProofIsTrustedAndColdNeverReceivesWarmupKeys(t *testing.T) {
	host := func() (Host, bool, bool) {
		return Host{Engine: "WebKitGTK", Native: true, GOOS: "linux", WebKitVersion: "2.54.1", GTKVersion: "4.22.5"}, true, true
	}
	s := &Service{cfg: Config{Phase: "warm", Count: 30, RunID: "proof", Host: host}, run: Run{Host: Host{}}}
	call := func(typ string, value any) error {
		data, _ := json.Marshal(value)
		_, err := s.receive(nil, typ, data)
		return err
	}
	if err := call("probe-armed", map[string]any{"type": "probe-armed", "runId": "proof", "visible": true, "focused": true}); err != nil {
		t.Fatal(err)
	}
	if err := call("input-proof", map[string]any{"type": "input-proof", "runId": "proof", "trusted": false, "key": "Shift"}); err == nil {
		t.Fatal("synthetic proof accepted")
	}
	if s.inputVerified {
		t.Fatal("failed proof granted permission")
	}
	if err := call("input-proof", map[string]any{"type": "input-proof", "runId": "proof", "trusted": true, "key": "Shift"}); err != nil {
		t.Fatal(err)
	}
	if !s.inputVerified || !s.run.Host.InputPermissionVerified {
		t.Fatal("trusted proof not recorded")
	}
	s.cfg.Phase = "cold"
	s.probeArmed = false
	if err := call("probe-armed", map[string]any{"type": "probe-armed", "runId": "proof", "visible": true, "focused": true}); err == nil {
		t.Fatal("cold first input was warmed by a probe")
	}
}

func TestNativeHostVisibilityOverridesRendererClaim(t *testing.T) {
	host := func() (Host, bool, bool) {
		return Host{Engine: "WebKitGTK", Native: true, GOOS: "linux", WebKitVersion: "2.52", GTKVersion: "4.22"}, false, true
	}
	s, err := New(Config{Phase: "warm", Count: 30, Output: filepath.Join(t.TempDir(), "raw.json"), Host: host})
	if err != nil {
		t.Fatal(err)
	}
	frame, _ := json.Marshal(Ready{Type: "ready", Visible: true, Focused: true, UserAgent: "WebKit", PaintOracle: "event-timing"})
	c := &fakeConn{ctx: t.Context(), out: make(chan []byte, 1)}
	if _, err := s.receive(c, "ready", frame); err == nil {
		t.Fatal("renderer visible flag overrode actual hidden native window")
	}
	if s.run.Ready.Type != "ready" || s.run.Ready.PaintOracle != "event-timing" {
		t.Fatal("rejected readiness evidence was discarded")
	}
}

func TestReceiverRejectsSubmittedAggregates(t *testing.T) {
	s := &Service{}
	if _, err := s.receive(nil, "ready", []byte(`{"type":"ready","visible":true,"focused":true,"userAgent":"WebKit","paintOracle":"event-timing","p95_ms":0}`)); err == nil {
		t.Fatal("submitted aggregate was accepted instead of raw boundaries")
	}
}

func TestNativePreparationPrecedesRendererCapability(t *testing.T) {
	prepared := false
	c := &fakeConn{ctx: t.Context(), in: make(chan []byte, 2), out: make(chan []byte, 2)}
	s, err := New(Config{
		Phase: "warm", Count: 30, Output: filepath.Join(t.TempDir(), "raw.json"),
		Host: func() (Host, bool, bool) {
			return Host{Engine: "WebKitGTK", Native: true, GOOS: "linux", WebKitVersion: "2.54.1", GTKVersion: "4.22.5"}, true, true
		},
		Prepare: func() error { prepared = true; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); s.Serve(c) }()
	sendFrame(c, hello{Type: "hello", Protocol: 1, Challenge: "0123456789abcdef0123456789abcdef"})
	<-c.out
	if !prepared {
		t.Fatal("renderer capability sent before native focus preparation")
	}
	sendFrame(c, failure{Type: "failure", Reason: "test end"})
	<-done
}
