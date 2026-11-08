package nativeperf

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
)

type (
	// ListenerSpan is one recorded App callback execution after the trusted keydown.
	ListenerSpan struct {
		Start  float64 `json:"start"`
		End    float64 `json:"end"`
		Call   int64   `json:"call"`
		Parent int64   `json:"parent"`
		ID     int64   `json:"id"`
	}
	// CallbackIdentity names a recorded callback once per sample.
	CallbackIdentity struct {
		ID           int64  `json:"id"`
		Target       string `json:"target"`
		Event        string `json:"event"`
		Capture      bool   `json:"capture"`
		Name         string `json:"name"`
		Registration string `json:"registration"`
		ScheduledBy  int64  `json:"scheduledBy,omitempty"`
	}
	// AttributionRow is one callback/stage label's per-sample self time.
	AttributionRow struct {
		Label        string `json:"label"`
		Kind         string `json:"kind"`
		Registration string `json:"registration,omitempty"`
		// Source is filled offline from the perf build's sourcemaps (scripts/perf/sourcemap.py).
		Source            string  `json:"source,omitempty"`
		Present           int     `json:"present"`
		KeystrokeSelfMean float64 `json:"keystroke_self_mean_ms"`
		KeystrokeSelfP95  float64 `json:"keystroke_self_p95_ms"`
		SettleSelfMean    float64 `json:"settle_self_mean_ms"`
		SettleSelfP95     float64 `json:"settle_self_p95_ms"`
	}
	// AttributionPhase aggregates one sample population (warm, cold or null).
	AttributionPhase struct {
		Samples           int              `json:"samples"`
		KeystrokeMean     float64          `json:"keystroke_bracket_mean_ms"`
		KeystrokeP95      float64          `json:"keystroke_bracket_p95_ms"`
		RecordedSelfMean  float64          `json:"keystroke_recorded_self_mean_ms"`
		UnrecordedMean    float64          `json:"keystroke_unrecorded_mean_ms"`
		Rows              []AttributionRow `json:"rows"`
		perSample         []map[string]rowSample
		identity          map[string]AttributionRow
		brackets, records []float64
	}
	// AttributionTable is a diagnostic: it never changes a budget, a sample or a verdict.
	AttributionTable struct {
		Method        string                       `json:"method"`
		Phases        map[string]*AttributionPhase `json:"phases"`
		PluginSources map[string]string            `json:"plugin_sources,omitempty"`
	}
	rowSample struct{ keystroke, settle float64 }
	traceNode struct {
		call, parent, by   int64
		start, end         float64
		label, kind, frame string
	}
)

const attributionMethod = "Self time per sample = span clipped to the window minus its directly nested recorded spans " +
	"(one synchronous trace for listeners, observers, timers, microtasks, rAF, messages and app stages). " +
	"keystroke window = outer keystroke-task bracket; settle window = keydown..settle. " +
	"unrecorded = keystroke bracket minus the union of top-level recorded spans: browser event dispatch and default " +
	"action, style/layout/paint, GC/JIT, promise continuations and harness brackets. Clock quantum 1 ms: means are " +
	"unbiased and additive (row means + unrecorded = bracket mean); per-row p95 is quantized. Never subtracted from a gate."

const unrecordedLabel = "unrecorded: browser dispatch/default action, style/layout, GC/JIT, promise continuations, harness"

// traceNodes builds the recorded call tree of one sample.
func traceNodes(s Sample) []traceNode {
	identities := map[int64]CallbackIdentity{}
	for _, c := range s.Callbacks {
		identities[c.ID] = c
	}
	nodes := []traceNode{}
	base := map[int64]string{}
	for _, l := range s.Listeners {
		c := identities[l.ID]
		kind, label := "listener", fmt.Sprintf("listener %s %s", c.Target, c.Event)
		switch {
		case c.Event == "observer":
			kind, label = "observer", "observer "+c.Target
		case c.Event == "message":
			kind, label = "message", "message "+c.Target
		case c.Target == "scheduler":
			kind, label = "scheduled", fmt.Sprintf("%s %s", c.Event, c.Name)
		case c.Capture:
			label += " (capture)"
		}
		base[l.Call] = label
		nodes = append(nodes, traceNode{call: l.Call, parent: l.Parent, by: c.ScheduledBy, start: l.Start, end: l.End, label: label, kind: kind, frame: c.Registration})
	}
	for _, h := range append(append([]Handler{}, s.Stages...), s.Handlers...) {
		if h.Call == 0 {
			continue
		}
		label := "stage " + h.Event
		base[h.Call] = label
		nodes = append(nodes, traceNode{call: h.Call, parent: h.Parent, start: h.Start, end: h.End, label: label, kind: "stage"})
	}
	// A timer/microtask/rAF row names what scheduled it (one level, no recursion).
	for i, n := range nodes {
		if n.kind != "scheduled" {
			continue
		}
		switch scheduler, ok := base[n.by]; {
		case ok:
			nodes[i].label += " ← " + scheduler
		case n.by == 0:
			nodes[i].label += " ← (no recorded caller: promise/browser)"
		default:
			nodes[i].label += " ← (call before this keystroke)"
		}
	}
	return nodes
}

func clip(start, end, from, to float64) float64 {
	return math.Max(0, math.Min(end, to)-math.Max(start, from))
}

// selfTimes returns per-label self time within [from,to] and the union of top-level spans.
func selfTimes(nodes []traceNode, from, to float64) (map[string]float64, float64) {
	present := map[int64]bool{}
	for _, n := range nodes {
		present[n.call] = true
	}
	childTime := map[int64]float64{}
	top := []FineSpan{}
	for _, n := range nodes {
		if present[n.parent] && n.parent != n.call {
			childTime[n.parent] += clip(n.start, n.end, from, to)
		} else {
			top = append(top, FineSpan{Start: n.start, End: n.end})
		}
	}
	out := map[string]float64{}
	for _, n := range nodes {
		self := math.Max(0, clip(n.start, n.end, from, to)-childTime[n.call])
		out[n.label+"\x00"+n.kind+"\x00"+n.frame] += self
	}
	return out, clippedWork(top, from, to)
}

func (t *AttributionTable) phase(name string) *AttributionPhase {
	if t.Phases == nil {
		t.Phases = map[string]*AttributionPhase{}
	}
	if t.Phases[name] == nil {
		t.Phases[name] = &AttributionPhase{identity: map[string]AttributionRow{}}
	}
	return t.Phases[name]
}

// add records one sample; samples without an outer bracket carry no keystroke window.
func (t *AttributionTable) add(phase string, s Sample) {
	var outer *Handler
	for i := range s.Handlers {
		if s.Handlers[i].Event == "keystroke-task" {
			outer = &s.Handlers[i]
		}
	}
	if outer == nil || !finite(outer.Start) || !finite(outer.End) || outer.End < outer.Start {
		return
	}
	nodes := traceNodes(s)
	keystroke, recorded := selfTimes(nodes, outer.Start, outer.End)
	settle, _ := selfTimes(nodes, s.KeyDownAt, s.SettledAt)
	p := t.phase(phase)
	values := map[string]rowSample{}
	for key, v := range keystroke {
		r := values[key]
		r.keystroke = v
		values[key] = r
	}
	for key, v := range settle {
		r := values[key]
		r.settle = v
		values[key] = r
	}
	bracket := outer.End - outer.Start
	unrecorded := unrecordedLabel + "\x00unrecorded\x00"
	values[unrecorded] = rowSample{keystroke: math.Max(0, bracket-recorded)}
	for key := range values {
		if _, ok := p.identity[key]; !ok {
			parts := strings.SplitN(key, "\x00", 3)
			p.identity[key] = AttributionRow{Label: parts[0], Kind: parts[1], Registration: parts[2]}
		}
	}
	p.perSample = append(p.perSample, values)
	p.brackets = append(p.brackets, bracket)
	p.records = append(p.records, recorded)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func p95(values []float64) float64 {
	if m := metric(values); m.P95MS != nil {
		return *m.P95MS
	}
	return 0
}

// finish aggregates every phase: zero is a real per-sample value for an absent row.
func (t *AttributionTable) finish(maxRows int) {
	for _, p := range t.Phases {
		p.Samples = len(p.perSample)
		p.KeystrokeMean = mean(p.brackets)
		p.KeystrokeP95 = p95(p.brackets)
		p.RecordedSelfMean = mean(p.records)
		p.Rows = p.Rows[:0]
		for key, row := range p.identity {
			ks, ss := make([]float64, p.Samples), make([]float64, p.Samples)
			for i, sample := range p.perSample {
				ks[i], ss[i] = sample[key].keystroke, sample[key].settle
				if ks[i] > 0 || ss[i] > 0 {
					row.Present++
				}
			}
			row.KeystrokeSelfMean = mean(ks)
			row.KeystrokeSelfP95 = p95(ks)
			row.SettleSelfMean, row.SettleSelfP95 = mean(ss), p95(ss)
			if row.Kind == "unrecorded" {
				p.UnrecordedMean = row.KeystrokeSelfMean
			}
			if row.KeystrokeSelfMean > 0 || row.SettleSelfMean > 0 {
				p.Rows = append(p.Rows, row)
			}
		}
		slices.SortFunc(p.Rows, func(a, b AttributionRow) int {
			return cmp.Or(cmp.Compare(b.KeystrokeSelfMean, a.KeystrokeSelfMean),
				cmp.Compare(b.SettleSelfMean, a.SettleSelfMean), strings.Compare(a.Label, b.Label))
		})
		if len(p.Rows) > maxRows {
			p.Rows = p.Rows[:maxRows]
		}
	}
}
