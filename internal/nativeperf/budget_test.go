package nativeperf

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The p95 budgets are certified at 95 % confidence by an order statistic (ADR 0038, user decision Q2 2026-11-11):
// warm 100 samples (bound = second largest), cold 59 fresh processes (bound = maximum).

func TestQuantileUpperIndexMatchesPythonVector(t *testing.T) {
	data, err := os.ReadFile("../../scripts/perf/testdata/quantile-index.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		P95 map[string]*int `json:"p95"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	if len(vector.P95) != 300 {
		t.Fatalf("vector has %d sizes, want 300", len(vector.P95))
	}
	for key, want := range vector.P95 {
		n, _ := strconv.Atoi(key)
		expected := 0 // the vector's null: no order statistic reaches the confidence
		if want != nil {
			expected = *want
		}
		if got := quantileUpperIndex(n); got != expected {
			t.Errorf("quantileUpperIndex(%d) = %d, Python vector %d", n, got, expected)
		}
	}
	if quantileUpperIndex(ColdReplicates) != ColdReplicates || quantileUpperIndex(WarmSamples) != WarmSamples-1 {
		t.Error("59 cold samples must be judged on their maximum, 100 warm ones on the second largest")
	}
	if quantileUpperIndex(30) != 0 {
		t.Error("30 samples can never certify a p95")
	}
}

func ramp(n int) []float64 {
	values := make([]float64, n)
	for i := range values {
		values[i] = float64(i + 1)
	}
	return values
}

func TestBudgetUsesOrderStatisticBound(t *testing.T) {
	m := metric(ramp(100)) // point p95 = 95, bound = 99 (the second largest)
	if m.P95BoundOrder != 99 || m.P95UpperBoundMS == nil || *m.P95UpperBoundMS != 99 {
		t.Fatalf("bound = %+v", m)
	}
	var v verifier
	v.checkBudget("warm handler", m, 98) // the point p95 (95) would have passed
	if len(v.report.Reasons) != 1 || !strings.Contains(v.report.Reasons[0], "warm handler p95 exceeds98ms") {
		t.Fatalf("a bound over the budget must fail: %v", v.report.Reasons)
	}
	v = verifier{}
	v.checkBudget("warm handler", m, 99)
	if len(v.report.Reasons) != 0 {
		t.Fatalf("a bound within the budget must pass: %v", v.report.Reasons)
	}
	cold := metric(ramp(ColdReplicates))
	if *cold.P95UpperBoundMS != ColdReplicates {
		t.Fatalf("cold bound = %v, want the maximum", *cold.P95UpperBoundMS)
	}
}

func TestMetricMinimumSamples(t *testing.T) {
	if m := metric(ramp(ColdReplicates - 1)); m.P95UpperBoundMS != nil || m.P95BoundOrder != 0 {
		t.Fatalf("58 samples cannot bound a p95: %+v", m)
	}
	var v verifier
	v.checkBudget("cold handler", metric(ramp(30)), 1000)
	if len(v.report.Reasons) != 1 || !strings.Contains(v.report.Reasons[0], "30 samples cannot certify a p95") {
		t.Fatalf("too few samples must fail closed, never pass: %v", v.report.Reasons)
	}
	if m := metric(nil); m.P95MS != nil || m.P95UpperBoundMS != nil {
		t.Fatalf("no samples, no figure: %+v", m)
	}
}

func TestMetricKeepsPointP95AsDiagnostic(t *testing.T) {
	if p := metric(ramp(100)).P95MS; p == nil || *p != 95 {
		t.Fatalf("point p95 of 1..100 = %v, want 95", p)
	}
	if p := metric(ramp(10)).P95MS; p == nil || *p != 10 {
		t.Fatalf("the point p95 stays reported below the minimum: %v", p)
	}
	data, err := json.Marshal(metric(ramp(100)))
	if err != nil || !strings.Contains(string(data), `"p95_ms":95`) || !strings.Contains(string(data), `"p95_upper_bound_ms":99`) {
		t.Fatalf("report fields: %s %v", data, err)
	}
}
