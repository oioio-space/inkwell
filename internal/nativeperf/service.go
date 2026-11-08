package nativeperf

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type (
	// Conn is the authenticated Wails byte-stream boundary.
	Conn interface {
		Send([]byte) error
		Receive() ([]byte, error)
		Context() context.Context
	}
	// Config contains native-host attestation and external driver callbacks.
	Config struct {
		Phase                    string
		Count, Index, Replicates int
		RunID, Output            string
		Host                     func() (Host, bool, bool)
		Prepare                  func() error
		Marker                   func(any)
		Done                     func(bool)
	}
	// Service schedules native trials and preserves raw samples.
	Service struct {
		dataMu        sync.Mutex
		finished      atomic.Bool
		cfg           Config
		run           Run
		mu            sync.Mutex
		once          sync.Once
		next          int
		armed         bool
		ready         bool
		probeArmed    bool
		inputVerified bool
		inputSeen     bool
	}
	hello struct {
		Type      string `json:"type"`
		Protocol  int    `json:"protocol"`
		Challenge string `json:"challenge"`
	}
	armed struct {
		Type    string `json:"type"`
		ID      int    `json:"id"`
		RunID   string `json:"runId"`
		Visible bool   `json:"visible"`
		Focused bool   `json:"focused"`
	}
	failure struct {
		Type   string `json:"type"`
		ID     *int   `json:"id,omitempty"`
		RunID  string `json:"runId"`
		Reason string `json:"reason"`
	}
)

// New refuses non-native hosts and invalid trial configurations.
func New(cfg Config) (*Service, error) {
	if cfg.Phase != "cold" && cfg.Phase != "warm" {
		return nil, errors.New("native phase must be cold or warm")
	}
	if cfg.Phase == "cold" && (cfg.Count != 1 || cfg.Replicates < 30 || cfg.Index < 1 || cfg.Index > cfg.Replicates) {
		return nil, errors.New("cold requires one sample and a valid >=30-process replicate")
	}
	if cfg.Phase == "warm" && cfg.Count < 30 {
		return nil, errors.New("warm requires >=30 samples after controls")
	}
	if cfg.Host == nil || cfg.Output == "" {
		return nil, errors.New("native host and raw output required")
	}
	if cfg.RunID == "" {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		cfg.RunID = hex.EncodeToString(nonce[:])
	}
	host, _, _ := cfg.Host()
	if !validHost(host) {
		return nil, errors.New("native WebKitGTK host required")
	}
	return &Service{cfg: cfg, run: Run{Protocol: 1, RunID: cfg.RunID, PID: os.Getpid(), StartedNS: processStartedNS, Phase: cfg.Phase, ReplicateIndex: cfg.Index, ReplicateCount: cfg.Replicates, Host: host}}, nil
}

func (s *Service) emit(c Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Send(b)
}

func (s *Service) marker(v any) {
	if s.cfg.Marker != nil {
		s.cfg.Marker(v)
	}
}

func (s *Service) kind(id int) string {
	if s.cfg.Phase == "warm" && id <= len(ControlKinds) {
		return ControlKinds[id-1]
	}
	if s.cfg.Phase == "warm" && id > len(ControlKinds)+s.cfg.Count {
		return nullTrialKind(id - len(ControlKinds) - s.cfg.Count)
	}
	return "none"
}

func (s *Service) total() int {
	if s.cfg.Phase == "warm" {
		return s.cfg.Count + len(ControlKinds) + 2*NullCount
	}
	return 1
}

func (s *Service) trial(c Conn) error {
	s.next++
	s.armed = false
	if s.next > s.total() {
		s.finish("")
		return nil
	}
	return s.emit(c, map[string]any{"type": "trial", "id": s.next, "control": s.kind(s.next)})
}

func (s *Service) focused() bool {
	host, visible, focused := s.cfg.Host()
	return host.Native && !host.Server && host.Engine == "WebKitGTK" && visible && focused
}

func readyEnvironment(r Ready, host Host, visible, focused bool) bool {
	return r.Visible && r.Focused && validHost(host) && visible && focused
}

// Serve authorizes one renderer before scheduling trusted native-key trials.
func (s *Service) Serve(c Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if closer, ok := c.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}
	// About 1.4 s per trial (1.3 s settle); 3 s each keeps the per-trial watchdog's margin.
	timer := time.AfterFunc(max(90*time.Second, time.Duration(s.total())*3*time.Second), func() { s.finish("native trial deadline exceeded") })
	defer timer.Stop()
	data, err := c.Receive()
	if err != nil {
		s.finish("missing native capability handshake")
		return
	}
	var h hello
	if strict(data, &h) != nil || h.Type != "hello" || h.Protocol != 1 || len(h.Challenge) != 32 {
		s.finish("invalid native capability challenge")
		return
	}
	if _, err := hex.DecodeString(h.Challenge); err != nil {
		s.finish("invalid native capability challenge")
		return
	}
	if s.cfg.Prepare != nil {
		if err := s.cfg.Prepare(); err != nil {
			s.finish("native activation failed: " + err.Error())
			return
		}
	}
	host, _, _ := s.cfg.Host()
	s.dataMu.Lock()
	s.run.Host = host
	s.dataMu.Unlock()
	if s.cfg.Phase == "cold" {
		s.dataMu.Lock()
		s.inputVerified = host.InputPermissionVerified
		s.dataMu.Unlock()
	}
	if err := s.emit(c, map[string]any{"type": "capability", "protocol": 1, "challenge": h.Challenge, "engine": "WebKitGTK", "native": true, "runId": s.cfg.RunID, "phase": s.cfg.Phase, "sampleCount": s.cfg.Count, "replicateIndex": s.cfg.Index, "replicateCount": s.cfg.Replicates, "refreshHz": host.RefreshHz, "inputPermissionVerified": host.InputPermissionVerified}); err != nil {
		s.finish(err.Error())
		return
	}
	fmt.Fprintln(os.Stderr, "native collector: capability sent")
	for {
		data, err := c.Receive()
		if err != nil {
			s.finish("native evidence stream disconnected")
			return
		}
		if len(data) > 4<<20 {
			s.finish("native evidence frame too large")
			return
		}
		var typ struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &typ) != nil {
			s.finish("malformed native evidence")
			return
		}
		fmt.Fprintln(os.Stderr, "native collector: received", typ.Type)
		complete, err := s.receive(c, typ.Type, data)
		fmt.Fprintln(os.Stderr, "native collector: handled", typ.Type, "error=", err)
		if err != nil {
			s.finish(err.Error())
			return
		}
		if complete {
			return
		}
	}
}

func (s *Service) receive(c Conn, typ string, data []byte) (bool, error) {
	if s.finished.Load() {
		return true, nil
	}
	switch typ {
	case "probe-armed", "input-proof":
		return false, s.receiveProbe(typ, data)
	case "ready":
		return false, s.receiveReady(c, data)
	case "armed":
		return false, s.receiveArmed(data)
	case "sample":
		return s.receiveSample(c, data)
	case "failure":
		var f failure
		if err := strict(data, &f); err != nil {
			return false, err
		}
		return false, fmt.Errorf("native frontend failure: %s", f.Reason)
	default:
		return false, errors.New("unexpected native evidence message")
	}
}

// receiveProbe handles the XTest "probe-armed" and "input-proof" messages.
func (s *Service) receiveProbe(typ string, data []byte) error {
	var p struct {
		Type    string `json:"type"`
		RunID   string `json:"runId"`
		Visible bool   `json:"visible,omitempty"`
		Focused bool   `json:"focused,omitempty"`
		Trusted bool   `json:"trusted,omitempty"`
		Key     string `json:"key,omitempty"`
	}
	if err := strict(data, &p); err != nil {
		return err
	}
	if p.RunID != s.cfg.RunID || s.cfg.Phase != "warm" || s.ready || !s.focused() {
		return errors.New("XTest probe identity/focus invalid")
	}
	if typ == "probe-armed" {
		if s.probeArmed || !p.Visible || !p.Focused {
			return errors.New("duplicate or hidden XTest probe")
		}
		s.probeArmed = true
		s.marker(map[string]any{"type": "probe", "runId": s.cfg.RunID, "pid": os.Getpid()})
		return nil
	}
	if !s.probeArmed || s.inputVerified || !p.Trusted || p.Key != "Shift" {
		return errors.New("XTest permission/delivery proof rejected")
	}
	s.dataMu.Lock()
	s.inputVerified = true
	s.run.Host.InputPermissionVerified = true
	s.dataMu.Unlock()
	s.marker(map[string]any{"type": "probe-ok", "runId": s.cfg.RunID, "pid": os.Getpid()})
	return nil
}

// receiveReady handles the renderer's "ready" message and starts the first trial.
func (s *Service) receiveReady(c Conn, data []byte) error {
	var r Ready
	if err := strict(data, &r); err != nil {
		return err
	}
	nativeHost, visible, focused := s.cfg.Host()
	s.dataMu.Lock()
	s.run.Ready = r
	s.dataMu.Unlock()
	if s.ready || !readyEnvironment(r, nativeHost, visible, focused) {
		return fmt.Errorf("native readiness rejected: duplicate=%t renderer visible=%t focused=%t native visible=%t focused=%t", s.ready, r.Visible, r.Focused, visible, focused)
	}
	if !s.inputVerified || !r.InputVerified {
		return errors.New("XTest input delivery/permission not validated")
	}
	s.ready = true
	return s.trial(c)
}

// receiveArmed handles the "armed" message announcing the next native input.
func (s *Service) receiveArmed(data []byte) error {
	var a armed
	if err := strict(data, &a); err != nil {
		return err
	}
	if !s.ready || s.armed || a.ID != s.next || a.RunID != s.cfg.RunID || !a.Visible || !a.Focused || !s.focused() {
		return errors.New("unfocused, duplicate or mismatched native arm; focus interrompu : dialogue GNOME d’autorisation d’interaction distante/XTest possible, autoriser puis relancer")
	}
	s.armed = true
	s.marker(map[string]any{"type": "armed", "runId": s.cfg.RunID, "id": a.ID, "phase": s.cfg.Phase, "control": s.kind(a.ID), "visible": true, "focused": true, "engine": "WebKitGTK", "expected": "x"})
	return nil
}

// receiveSample stores a "sample" message and either finishes the run or
// starts the next trial. The boolean reports that the run is complete.
func (s *Service) receiveSample(c Conn, data []byte) (bool, error) {
	var sample Sample
	if err := strict(data, &sample); err != nil {
		return false, err
	}
	if !s.armed || sample.ID != s.next || sample.RunID != s.cfg.RunID || sample.Phase != s.cfg.Phase || sample.Control.Kind != s.kind(s.next) || !s.focused() {
		return false, errors.New("sample does not match armed native input")
	}
	s.dataMu.Lock()
	if s.finished.Load() {
		s.dataMu.Unlock()
		return true, nil
	}
	switch sample.Control.Kind {
	case "null":
		s.run.NullSamples = append(s.run.NullSamples, sample)
	case NullOff:
		s.run.NullOffSamples = append(s.run.NullOffSamples, sample)
	default:
		s.run.Samples = append(s.run.Samples, sample)
	}
	s.inputSeen = s.inputSeen || actualInput(sample)
	s.dataMu.Unlock()
	s.armed = false
	s.marker(map[string]any{"type": "sample", "runId": s.cfg.RunID, "id": sample.ID, "phase": s.cfg.Phase, "control": s.kind(sample.ID)})
	if s.next == s.total() {
		s.finish("")
		return true, nil
	}
	return false, s.trial(c)
}

func (s *Service) finish(reason string) {
	s.once.Do(func() {
		s.dataMu.Lock()
		s.finished.Store(true)
		run := s.run
		run.Samples = append([]Sample{}, s.run.Samples...)
		run.NullSamples = append([]Sample{}, s.run.NullSamples...)
		run.NullOffSamples = append([]Sample{}, s.run.NullOffSamples...)
		run.Failures = append([]string{}, s.run.Failures...)
		inputUnavailable := !s.inputVerified || !s.inputSeen
		s.dataMu.Unlock()
		run.Failures = append(run.Failures, collectedFailures(run)...)
		if run.Ready.Type != "ready" {
			run.Failures = append(run.Failures, "native renderer readiness not collected; paint oracle not probed")
		}
		if reason != "" {
			run.Failures = append(run.Failures, reason)
		}
		run.Failures = uniqueReasons(run.Failures)
		if err := WriteJSON(s.cfg.Output, run); err != nil {
			run.Failures = append(run.Failures, err.Error())
		}
		ok := len(run.Failures) == 0
		status := "fail"
		if ok {
			status = "collected"
		}
		s.marker(map[string]any{"type": "complete", "runId": s.cfg.RunID, "rawPath": s.cfg.Output, "status": status, "inputUnavailable": inputUnavailable})
		if s.cfg.Done != nil {
			s.cfg.Done(ok)
		}
	})
}

// Expire records a failed run if the tagged window never establishes its stream.
func (s *Service) Expire(reason string) { s.finish(reason) }

// Constant across service generations, so merge detects reuse of one process.
var processStartedNS = time.Now().UnixNano()
