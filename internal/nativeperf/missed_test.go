package nativeperf

import (
	"math"
	"slices"
	"strings"
	"testing"
)

const relativeMissReason = "warm editor missed significantly more refreshes than NULL typing (one-sided Fisher p < 0.05)"

// skipRefreshes removes n consecutive rendering opportunities (heartbeat and
// frame at the same instant) after at: one pause of (n+1) cadence periods.
func skipRefreshes(s *Sample, at float64, n int) {
	i := slices.IndexFunc(s.Heartbeat, func(h float64) bool { return h > at })
	gone := s.Heartbeat[i : i+n]
	s.Frames = slices.DeleteFunc(s.Frames, func(f WorkFrame) bool { return slices.Contains(gone, f.Timestamp) })
	s.Heartbeat = slices.Delete(s.Heartbeat, i, i+n)
}

// skipInWarm makes the first editor and NULL trials of the warm run each miss one refresh.
func skipInWarm(t *testing.T, runs []Run, editor, null int) {
	t.Helper()
	missed := 0
	for i := range runs[0].Samples {
		if s := &runs[0].Samples[i]; s.Control.Kind == "none" && missed < editor {
			skipRefreshes(s, 600, 1)
			missed++
		}
	}
	for i := range null {
		skipRefreshes(&runs[0].NullSamples[i], 600, 1)
	}
}

func TestWarmEditorMissingMoreRefreshesThanNullFails(t *testing.T) {
	runs := completeFrameFixture()
	skipInWarm(t, runs, 40, 1)
	got := Validate(runs)
	m := got.MissedRefresh
	if got.Status != "fail" || !slices.Contains(got.Reasons, relativeMissReason) || m == nil || m.PValue >= missedRefreshAlpha {
		t.Fatal("editor missing 40/100 vs NULL 1/30 passed", got.Reasons, m)
	}
	if m.EditorMissed != 40 || m.EditorTrials != WarmSamples || m.NullMissed != 1 || m.NullTrials != 30 || m.EditorRate != .4 || math.Abs(m.NullRate-1.0/30) > 1e-12 {
		t.Fatalf("rates not published: %+v", m)
	}
}

func TestWarmEditorMissingAsOftenAsNullPasses(t *testing.T) {
	runs := completeFrameFixture()
	skipInWarm(t, runs, 3, 3)
	got := Validate(runs)
	if got.Status != "pass" || got.MissedRefresh == nil || got.MissedRefresh.PValue < .5 {
		t.Fatal("equal missed-refresh rates failed", got.Reasons, got.MissedRefresh)
	}
}

func TestNullMissedRefreshesNoLongerBlock(t *testing.T) {
	runs := completeFrameFixture()
	skipInWarm(t, runs, 0, 3)
	got := Validate(runs)
	if got.Status != "pass" || got.MissedRefresh.NullMissed != 3 || got.MissedRefresh.PValue < 1-1e-9 {
		t.Fatal("NULL missed frames blocked certification", got.Reasons, got.MissedRefresh)
	}
}

func TestColdFirstPreviewPauseIsNotAMissedRefresh(t *testing.T) {
	runs := completeFrameFixture()
	for n := 1; n < len(runs); n++ {
		skipRefreshes(&runs[n].Samples[0], 300, 3) // one 64 ms pause: the first preview presentation
	}
	got := Validate(runs)
	if got.Status != "pass" || got.DroppedFrames != 0 || got.MissedRefresh.ColdExempt != ColdReplicates || got.MissedRefresh.ColdMissed != 0 {
		t.Fatal("cold first-preview pause counted as a missed refresh", got.Reasons, got.MissedRefresh)
	}
}

// Decision 2026-11-07 (nuit): a cold missed refresh outside the startup pause
// is published as a diagnostic and does not gate; the startup thresholds do.
func TestColdMissedRefreshIsDiagnosticOnly(t *testing.T) {
	runs := completeFrameFixture()
	skipRefreshes(&runs[1].Samples[0], 300, 3) // the exempt startup pause
	skipRefreshes(&runs[1].Samples[0], 800, 1) // a second pause in the same launch: counted
	skipRefreshes(&runs[2].Samples[0], 300, 3)
	skipRefreshes(&runs[2].Samples[0], 800, 1)
	got := Validate(runs)
	if got.Status != "pass" || got.MissedRefresh.ColdMissed != 2 || got.MissedRefresh.ColdExempt != 2 || got.DroppedFrames != 2 {
		t.Fatal("cold missed refreshes gated or were not published", got.Reasons, got.MissedRefresh, got.DroppedFrames)
	}
	if got.MissedRefresh.EditorMissed != 0 {
		t.Fatal("cold misses leaked into the warm editor count", got.MissedRefresh)
	}
}

func TestColdPauseOverBudgetFailsByTheStartupPauseRule(t *testing.T) {
	runs := completeFrameFixture()
	skipRefreshes(&runs[1].Samples[0], 300, 4) // 80 ms
	got := Validate(runs)
	if got.Status != "fail" || !slices.Contains(got.Reasons, "native cold frame gap exceeds70ms") || got.MissedRefresh.ColdMissed != 1 {
		t.Fatal("an 80 ms cold pause passed", got.Reasons, got.MissedRefresh)
	}
	if slices.ContainsFunc(got.Reasons, func(r string) bool { return strings.Contains(r, "missed a refresh") }) {
		t.Fatal("cold missed refresh still gates", got.Reasons)
	}
}

func TestFisherGreaterMatchesHypergeometric(t *testing.T) {
	for _, c := range []struct {
		a, n1, b, n2 int
		want         float64
	}{
		{3, 10, 0, 10, 120.0 / 1140},    // C(10,3)/C(20,3)
		{5, 5, 0, 5, 1.0 / 252},         // 1/C(10,5)
		{0, 30, 3, 30, 1},               // editor at zero: no evidence against it
		{3, 30, 3, 30, 0.6646257400049}, // equal rates
	} {
		if got := fisherGreater(c.a, c.n1, c.b, c.n2); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("fisherGreater(%d/%d vs %d/%d) = %v, want %v", c.a, c.n1, c.b, c.n2, got, c.want)
		}
	}
}
