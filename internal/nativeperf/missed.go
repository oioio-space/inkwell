package nativeperf

import (
	"math"
	"slices"
)

// MissedRefresh publishes the missed-refresh rule decided on 2026-11-07
// (docs/objectif-en-cours.md, "soir, 2"): warm typing fails only when the
// editor misses refreshes significantly more often than NULL typing in the
// same evidence. Cold misses (decision "nuit") no longer gate: the startup
// thresholds judge the cold phase and ColdMissed publishes them as a diagnostic.
type MissedRefresh struct {
	Method       string  `json:"method"`
	EditorMissed int     `json:"editor_missed"`
	EditorTrials int     `json:"editor_trials"`
	EditorRate   float64 `json:"editor_rate"`
	NullMissed   int     `json:"null_missed"`
	NullTrials   int     `json:"null_trials"`
	NullRate     float64 `json:"null_rate"`
	PValue       float64 `json:"p_value"`
	ColdExempt   int     `json:"cold_startup_exempt"`
	ColdMissed   int     `json:"cold_missed_diagnostic"`
}

const (
	missedRefreshAlpha  = 0.05
	missedRefreshMethod = "warm: trials with >=1 missed refresh, editor vs NULL (recording hooks on); one-sided Fisher exact test, H1 editor proportion > NULL proportion, fail if p < 0.05; cold (diagnostic, not gating): fresh processes with a missed refresh besides one pause <=70 ms (first preview presentation)"
)

// coldCountedMisses removes the cold startup pause from missed: the longest
// missed span whose upper bound fits the cold pause budget, together with
// every span overlapping it (the frame and heartbeat views of the same pause).
func coldCountedMisses(missed []FineSpan, resolution float64) []FineSpan {
	var pause FineSpan
	found := false
	for _, m := range missed {
		if d := m.End - m.Start; d+resolution <= coldGapMS && (!found || d > pause.End-pause.Start) {
			pause, found = m, true
		}
	}
	if !found {
		return missed
	}
	return slices.DeleteFunc(slices.Clone(missed), func(m FineSpan) bool {
		return m.Start < pause.End && m.End > pause.Start
	})
}

// fisherGreater is the one-sided Fisher exact test p-value of H1 "the first
// group's proportion a/n1 exceeds the second group's b/n2": the
// hypergeometric probability of a or more successes in the first group given
// both margins.
func fisherGreater(a, n1, b, n2 int) float64 {
	k, n := a+b, n1+n2
	logChoose := func(n, k int) float64 {
		x, _ := math.Lgamma(float64(n + 1))
		y, _ := math.Lgamma(float64(k + 1))
		z, _ := math.Lgamma(float64(n - k + 1))
		return x - y - z
	}
	p := 0.0
	for x := a; x <= min(n1, k); x++ {
		p += math.Exp(logChoose(k, x) + logChoose(n-k, n1-x) - logChoose(n, n1))
	}
	return min(p, 1)
}

// missedRefresh applies the warm relative rule and returns its publication.
func (v *verifier) missedRefresh() *MissedRefresh {
	if v.warmTrials == 0 || v.nullTrials == 0 {
		return nil
	}
	m := MissedRefresh{Method: missedRefreshMethod, EditorMissed: v.warmMissed, EditorTrials: v.warmTrials, NullMissed: v.nullMissed, NullTrials: v.nullTrials, ColdExempt: v.coldExempt, ColdMissed: v.coldMissed}
	m.EditorRate = float64(m.EditorMissed) / float64(m.EditorTrials)
	m.NullRate = float64(m.NullMissed) / float64(m.NullTrials)
	m.PValue = fisherGreater(m.EditorMissed, m.EditorTrials, m.NullMissed, m.NullTrials)
	if m.PValue < missedRefreshAlpha {
		v.fail("warm editor missed significantly more refreshes than NULL typing (one-sided Fisher p < 0.05)")
	}
	return &m
}
