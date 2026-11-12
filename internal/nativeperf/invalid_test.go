package nativeperf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// An environment fault (focus lost, input delivery lost, activation failed) makes a native run invalid, to be
// retried with a fresh raw directory; it is neither a pass nor a performance failure (ADR 0038, plan section 8).

const focusLoss = "native frontend failure: [fault:focus-lost] native editor must be visible and focused before arming"

func TestVerdictInvalidOnCodedFocusLoss(t *testing.T) {
	runs := completeFrameFixture()
	runs[5].Failures = []string{focusLoss}
	runs[5].Samples = nil // the run stopped where focus was lost
	got := Validate(runs)
	if got.Status != "invalid" {
		t.Fatalf("status = %q, reasons %v", got.Status, got.Reasons)
	}
	if len(got.Reasons) != 1 || !strings.Contains(got.Reasons[0], "focus-lost") {
		t.Fatalf("reasons = %v", got.Reasons)
	}
	if code, ok := FaultCode(focusLoss); !ok || code != "focus-lost" {
		t.Fatalf("FaultCode = %q %v", code, ok)
	}
	if _, ok := FaultCode("native frontend failure: frame heartbeat absent"); ok {
		t.Fatal("an uncoded failure is a genuine failure")
	}
	if _, ok := FaultCode("[fault:made-up] x"); ok {
		t.Fatal("unknown codes are not environment faults")
	}
	// The same truncated run without the code is a genuine failure.
	runs[5].Failures = []string{"native frontend failure: frame heartbeat absent"}
	if got := Validate(runs); got.Status != "fail" {
		t.Fatalf("uncoded failure status = %q", got.Status)
	}
	// The driver's own record of a fault, found by the merge.
	dir := t.TempDir()
	driver := `{"status":"fault","fault":{"kind":"activation-failed","message":"native X11 window activation did not establish focus"}}`
	if err := os.WriteFile(filepath.Join(dir, "driver.json"), []byte(driver), 0o600); err != nil {
		t.Fatal(err)
	}
	merged := Merge(dir, filepath.Join(dir, "out", "PERF-TYPE.json"))
	if merged.Status != "invalid" || len(merged.Reasons) != 1 || !strings.Contains(merged.Reasons[0], "activation-failed") {
		t.Fatalf("merge of a faulted driver: %q %v", merged.Status, merged.Reasons)
	}
}

func TestVerdictInvalidFlag(t *testing.T) {
	reasons := InvalidReasons("focus-lost: window lost focus | delivery-lost: key not delivered")
	if !slices.Equal(reasons, []string{"focus-lost: window lost focus", "delivery-lost: key not delivered"}) {
		t.Fatalf("reasons = %q", reasons)
	}
	if got := InvalidReasons(""); len(got) != 1 || !strings.Contains(got[0], "environment") {
		t.Fatalf("an empty flag still says why: %q", got)
	}
	report := Invalid(reasons)
	if report.Status != "invalid" || report.ID != "PERF-TYPE-native" || !slices.Equal(report.Reasons, reasons) {
		t.Fatalf("report = %+v", report)
	}
	out := filepath.Join(t.TempDir(), "PERF-TYPE.json")
	if err := WriteReport(out, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"status": "invalid"`) {
		t.Fatalf("written report: %s %v", data, err)
	}
}

func TestVerdictInvalidAddsNoPerformanceReason(t *testing.T) {
	report := Invalid([]string{"focus-lost: window lost focus"})
	for _, reason := range report.Reasons {
		if strings.Contains(reason, "missing") || strings.Contains(reason, "p95") || strings.Contains(reason, "samples") {
			t.Fatalf("performance reason in an invalid verdict: %q", reason)
		}
	}
	if len(report.Reasons) != 1 {
		t.Fatalf("reasons = %v", report.Reasons)
	}
	// Validate(nil) would have said "missing >=59 ... samples": an invalid run is not judged.
	if got := Validate(nil); got.Status != "fail" || len(got.Reasons) < 2 {
		t.Fatalf("anti-vacuity: Validate(nil) = %q %v", got.Status, got.Reasons)
	}
}

// coldGap removes the heartbeats of a cold sample between 500 and 600 ms: a measured 100 ms frame gap, a genuine
// performance failure ("native cold frame gap exceeds70ms") that no fault explains.
func coldGap(runs []Run, index int) {
	s := &runs[index].Samples[0]
	kept := s.Heartbeat[:0:0]
	for _, at := range s.Heartbeat {
		if at < 500 || at > 600 {
			kept = append(kept, at)
		}
	}
	s.Heartbeat = kept
}

func faulted() []Run {
	runs := completeFrameFixture()
	runs[5].Failures = []string{focusLoss}
	runs[5].Samples = nil
	return runs
}

func hasReason(reasons []string, part string) bool {
	return slices.ContainsFunc(reasons, func(r string) bool { return strings.Contains(r, part) })
}

// A fault explains only what is missing; a failure measured on the samples that exist, or any other genuine
// failure, makes the run fail, and the verdict keeps both the failure and the fault (review 2026-11-12).
func TestVerdictFaultKeepsPerformanceFailure(t *testing.T) {
	runs := faulted()
	coldGap(runs, 3)
	got := Validate(runs)
	if got.Status != "fail" || !hasReason(got.Reasons, "native cold frame gap exceeds70ms") || !hasReason(got.Reasons, "focus-lost") {
		t.Fatalf("status %q, reasons %v", got.Status, got.Reasons)
	}
	runs = faulted()
	runs[7].Failures = []string{"native frontend failure: frame heartbeat absent"}
	if got := Validate(runs); got.Status != "fail" || !hasReason(got.Reasons, "frame heartbeat absent") {
		t.Fatalf("an uncoded failure beside a fault: %q %v", got.Status, got.Reasons)
	}
	// The gap alone fails, the fault alone is invalid: the combination is the failure.
	alone := completeFrameFixture()
	coldGap(alone, 3)
	if got := Validate(alone); got.Status != "fail" {
		t.Fatalf("anti-vacuity: the gap alone = %q", got.Status)
	}
	if got := Validate(faulted()); got.Status != "invalid" {
		t.Fatalf("anti-vacuity: the fault alone = %q %v", got.Status, got.Reasons)
	}
}

// The merge judges the driver's own fault record like a fault in a run: the measured runs keep their failures.
func TestMergeFaultKeepsRunFailures(t *testing.T) {
	dir := t.TempDir()
	runs := completeFrameFixture()
	coldGap(runs, 3)
	for i, run := range runs[:len(runs)-1] { // the last replicate never ran: the driver lost the focus
		data, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("run-%02d.json", i)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	driver := `{"status":"fault","fault":{"kind":"focus-lost","message":"native window lost focus"}}`
	if err := os.WriteFile(filepath.Join(dir, "driver.json"), []byte(driver), 0o600); err != nil {
		t.Fatal(err)
	}
	got := Merge(dir, filepath.Join(dir, "out", "PERF-TYPE.json"))
	if got.Status != "fail" || !hasReason(got.Reasons, "native cold frame gap exceeds70ms") || !hasReason(got.Reasons, "focus-lost") {
		t.Fatalf("status %q, reasons %v", got.Status, got.Reasons)
	}
	if err := os.Remove(filepath.Join(dir, "run-03.json")); err != nil {
		t.Fatal(err)
	}
	if got := Merge(dir, filepath.Join(dir, "out", "PERF-TYPE.json")); got.Status != "invalid" {
		t.Fatalf("anti-vacuity: without the gapped run the fault makes it invalid, got %q %v", got.Status, got.Reasons)
	}
}
