package nativeperf

import (
	"math"
	"testing"
)

// One keystroke: a contentDOM listener dispatches CodeMirror, which runs the
// Automerge stage, which schedules a microtask. Self times must add up to the
// outer bracket exactly, with the rest attributed to "unrecorded".
func TestAttributionSelfTimeIsAdditive(t *testing.T) {
	s := Sample{
		KeyDownAt: 10, SettledAt: 30,
		Handlers: []Handler{
			{Event: "keystroke-task", Start: 10, End: 20, Trusted: true},
			{Event: "codemirror-input", Start: 11, End: 15, Call: 2, Parent: 1, Trusted: true},
		},
		Stages: []Handler{{Event: "automerge-ui", Start: 12, End: 14, Call: 3, Parent: 2}},
		Listeners: []ListenerSpan{
			{Start: 10, End: 16, Call: 1, ID: 7},
			{Start: 16, End: 18, Call: 4, ID: 8},
			{Start: 25, End: 26, Call: 5, ID: 9}, // After the bracket: settle window only.
		},
		Callbacks: []CallbackIdentity{
			{ID: 7, Target: "view.contentDOM", Event: "beforeinput", Registration: "f@a.js:1:2"},
			{ID: 8, Target: "scheduler", Event: "queueMicrotask", Name: "flush", ScheduledBy: 3},
			{ID: 9, Target: "scheduler", Event: "setTimeout", Name: "save"},
		},
	}
	var table AttributionTable
	table.add("warm", s)
	table.finish(60)
	p := table.Phases["warm"]
	want := map[string][2]float64{ // label: keystroke self, settle self
		"listener view.contentDOM beforeinput":                    {2, 2},
		"stage codemirror-input":                                  {2, 2},
		"stage automerge-ui":                                      {2, 2},
		"queueMicrotask flush ← stage automerge-ui":               {2, 2},
		"setTimeout save ← (no recorded caller: promise/browser)": {0, 1},
		unrecordedLabel:                                           {2, 0},
	}
	if len(p.Rows) != len(want) {
		t.Fatalf("rows = %+v", p.Rows)
	}
	sum := 0.0
	for _, row := range p.Rows {
		w, ok := want[row.Label]
		if !ok || row.KeystrokeSelfMean != w[0] || row.SettleSelfMean != w[1] {
			t.Errorf("row %q = %v/%v, want %v", row.Label, row.KeystrokeSelfMean, row.SettleSelfMean, w)
		}
		sum += row.KeystrokeSelfMean
	}
	if math.Abs(sum-p.KeystrokeMean) > 1e-9 || p.KeystrokeMean != 10 || p.UnrecordedMean != 2 {
		t.Fatalf("self-time sum %v, bracket %v, unrecorded %v", sum, p.KeystrokeMean, p.UnrecordedMean)
	}
	for _, row := range p.Rows {
		if row.Label == "listener view.contentDOM beforeinput" && row.Registration != "f@a.js:1:2" {
			t.Fatalf("listener registration frames = %q", row.Registration)
		}
	}
}

// A sample without the outer bracket has no keystroke window: it is skipped, not invented.
func TestAttributionSkipsSampleWithoutBracket(t *testing.T) {
	var table AttributionTable
	table.add("cold", Sample{Listeners: []ListenerSpan{{Start: 1, End: 2, Call: 1, ID: 1}}})
	table.finish(60)
	if len(table.Phases) != 0 {
		t.Fatalf("phases = %+v", table.Phases)
	}
}

func TestNullDocumentIsolation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before, after string
		paths         []string
		want          bool
	}{
		{"unchanged", "h1", "h1", nil, true},
		{"unchanged but paths claimed", "h1", "h1", []string{"meta/claimedAt"}, false},
		{"claim outside text", "h1", "h2", []string{"meta/claimedAt", "meta"}, true},
		{"text touched", "h1", "h2", []string{"meta", "text/4"}, false},
		{"whole text", "h1", "h2", []string{"text"}, false},
		{"changed without proof", "h1", "h2", nil, false},
		{"empty path", "h1", "h2", []string{""}, false},
	} {
		s := Sample{RevisionBefore: tc.before, RevisionAfter: tc.after, NullForeignPaths: tc.paths}
		if got := nullDocumentIsolated(s); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
