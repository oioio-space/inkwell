package nativeperf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The committed verdict stays small: raw runs are referenced by path and
// SHA-256, and the full diagnostics live in a separate detail file.
func TestMergeWritesCompactSummaryAndReferencesRawRuns(t *testing.T) {
	raw := t.TempDir()
	run := []byte(`{"protocol":1,"runId":"r","phase":"warm"}`)
	if err := os.WriteFile(filepath.Join(raw, "warm-01.json"), run, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "PERF-TYPE.json")
	if got := Merge(raw, out); got.Status != "fail" {
		t.Fatalf("incomplete evidence status %q, want fail", got.Status)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"raw_runs", "attribution_table", "attribution_diagnostic", "clocks"} {
		if _, ok := summary[key]; ok {
			t.Errorf("summary embeds %s; it belongs in the detail file", key)
		}
	}
	var s Summary
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(run)
	want := Evidence{Path: filepath.ToSlash(filepath.Join(raw, "warm-01.json")), SHA256: hex.EncodeToString(sum[:])}
	if len(s.RawEvidence) != 1 || s.RawEvidence[0] != want {
		t.Errorf("raw evidence %+v, want [%+v]", s.RawEvidence, want)
	}
	if s.Status != "fail" || len(s.Reasons) == 0 {
		t.Errorf("summary lost the verdict: status %q, reasons %v", s.Status, s.Reasons)
	}
	if s.Detail != DetailPath(out) {
		t.Errorf("detail %q, want %q", s.Detail, DetailPath(out))
	}
	var detail map[string]json.RawMessage
	if b, err := os.ReadFile(s.Detail); err != nil || json.Unmarshal(b, &detail) != nil {
		t.Fatalf("detail file unreadable: %v", err)
	}
	if _, ok := detail["attribution_diagnostic"]; !ok {
		t.Error("detail file lacks the attribution diagnostics")
	}
}

// Every p95 metric and every maximum gap of the report reaches the summary.
func TestSummaryKeepsEveryP95AndGap(t *testing.T) {
	r := Validate(nil)
	p95, gap := 7.5, 42.0
	r.WarmHandler.P95MS, r.ColdFrameWork.P95MS, r.ColdMaxGapMS = &p95, &p95, &gap
	s := r.Summary("detail.json")
	for _, name := range []string{"handler", "warm_handler", "cold_handler", "frame_work", "cold_frame_work_upper_bound", "warm_paint_upper_bound", "null_handler", "frame_interval"} {
		if _, ok := s.P95MS[name]; !ok {
			t.Errorf("summary lacks p95 %s", name)
		}
	}
	if got := s.P95MS["warm_handler"]; got == nil || *got != p95 {
		t.Errorf("warm_handler p95 %v, want %v", got, p95)
	}
	if len(s.MaxGapsMS) != 6 {
		t.Errorf("summary has %d maximum gaps, want 6: %v", len(s.MaxGapsMS), s.MaxGapsMS)
	}
	if got := s.MaxGapsMS["cold_max_gap_ms"]; got == nil || *got != gap {
		t.Errorf("cold_max_gap_ms %v, want %v", got, gap)
	}
}
