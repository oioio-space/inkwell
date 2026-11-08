package nativeperf

import (
	"math"
	"strings"
)

// NULL uses the same trusted native key, window brackets, clock and settle
// recorder, but edits a separate DOM target. It must never edit CodeMirror or
// enter either production-sample population. Its cost is NEVER subtracted.
func (v *verifier) addNull(s Sample, c Clock) {
	if s.Control.Kind != "null" || !s.NullInserted || s.Changed || !s.Trusted || !s.Visible || !s.Focused || s.RevisionBefore == "" || !nullDocumentIsolated(s) || s.LengthAfter != s.LengthBefore || s.KeyDownAt <= 0 || s.DocChangedAt < s.KeyDownAt || len(s.RAF) != 2 {
		v.fail("invalid NULL keystroke/document isolation proof")
		return
	}
	if err := keystrokeStart(s); err != nil {
		v.fail(err.Error())
		return
	}
	events := map[string]int{}
	for _, h := range s.Handlers {
		if !h.Trusted || h.Event == "codemirror-input" {
			v.fail("NULL path executed editor work or untrusted input")
			return
		}
		events[h.Event]++
	}
	for _, event := range []string{"keydown", "beforeinput", "input", "null-input", "keystroke-task"} {
		if events[event] != 1 {
			v.fail("incomplete NULL event boundaries")
			return
		}
	}
	if err := coverage(s, c); err != nil {
		v.fail("NULL overhead uncovered: " + err.Error())
		return
	}
	raw, upper, err := measuredHandlerWork(s.Handlers, c)
	if err != nil {
		v.fail("NULL clock invalid: " + err.Error())
		return
	}
	work, _, _, dropped, err := frameWork(s, c)
	// NULL missed refreshes no longer block: they are the warm baseline (missed.go).
	if err != nil || work > 16 {
		v.fail("NULL path frame evidence invalid or exceeded frame work budget: instrument cannot certify typing")
	} else {
		v.nullTrials++
		if dropped {
			v.nullMissed++
		}
	}
	for i := 1; i < len(s.Heartbeat); i++ {
		if s.Heartbeat[i]-s.Heartbeat[i-1]+c.ResolutionMS > 50 {
			v.fail("NULL path pause exceeds50ms")
		}
	}
	v.table.add("null", s)
	v.nulls = append(v.nulls, raw)
	v.nullUppers = append(v.nullUppers, upper)
}

// addNullOff accepts a NULL trial run with every recording hook off: the same
// trusted key, target and isolation proof, and the outer keystroke bracket as the
// only record. Its duration is compared with the "null" arm's, never subtracted.
func (v *verifier) addNullOff(s Sample, c Clock) {
	if s.Control.Kind != NullOff || !s.NullInserted || s.Changed || !s.Trusted || !s.Visible || !s.Focused || s.RevisionBefore == "" || !nullDocumentIsolated(s) || s.LengthAfter != s.LengthBefore || s.KeyDownAt <= 0 {
		v.fail("invalid NULL keystroke/document isolation proof")
		return
	}
	if len(s.Handlers) != 1 || s.Handlers[0].Event != "keystroke-task" || !s.Handlers[0].Trusted || s.Handlers[0].Start != s.KeyDownAt || len(s.Tasks)+len(s.Listeners)+len(s.Stages) != 0 {
		v.fail("NULL-off trial recorded more than its outer keystroke bracket: recording hooks were on")
		return
	}
	raw, _, err := measuredHandlerWork(s.Handlers, c)
	if err != nil {
		v.fail("NULL-off clock invalid: " + err.Error())
		return
	}
	v.nullOffs = append(v.nullOffs, raw)
}

// Overhead is the instrument's cost: the outer keystroke bracket of interleaved
// NULL trials with the recording hooks on minus off (the same two outer clock
// reads in both arms). The criterion is the one-sided 95 % upper confidence
// bound of the difference of means (Welch).
type Overhead struct {
	Method             string    `json:"method"`
	ProbesOffSamplesMS []float64 `json:"probes_off_samples_ms"`
	ProbesOnMeanMS     float64   `json:"probes_on_mean_ms"`
	ProbesOffMeanMS    float64   `json:"probes_off_mean_ms"`
	DifferenceMS       float64   `json:"difference_ms"`
	UpperBound95MS     float64   `json:"upper_bound_95_ms"`
	DegreesOfFreedom   float64   `json:"degrees_of_freedom"`
	NOn                int       `json:"n_on"`
	NOff               int       `json:"n_off"`
}

const overheadMethod = "mean(on)-mean(off) + t(0.95, Welch df) * sqrt(var_on/n_on + var_off/n_off); on = null_handler samples, unbiased under random 1 ms clock phase"

func overheadBound(on, off []float64) (Overhead, bool) {
	o := Overhead{Method: overheadMethod, ProbesOffSamplesMS: off, NOn: len(on), NOff: len(off)}
	if len(on) < 2 || len(off) < 2 {
		return o, false
	}
	n1, n2 := float64(len(on)), float64(len(off))
	o.ProbesOnMeanMS, o.ProbesOffMeanMS = mean(on), mean(off)
	o.DifferenceMS = o.ProbesOnMeanMS - o.ProbesOffMeanMS
	a, b := variance(on, o.ProbesOnMeanMS)/n1, variance(off, o.ProbesOffMeanMS)/n2
	if a+b == 0 {
		o.DegreesOfFreedom, o.UpperBound95MS = n1+n2-2, o.DifferenceMS
		return o, true
	}
	o.DegreesOfFreedom = (a + b) * (a + b) / (a*a/(n1-1) + b*b/(n2-1))
	o.UpperBound95MS = o.DifferenceMS + studentT95(o.DegreesOfFreedom)*math.Sqrt(a+b)
	return o, true
}

// variance is the unbiased sample variance around m.
func variance(values []float64, m float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += (v - m) * (v - m)
	}
	return sum / float64(len(values)-1)
}

// studentT95 is the one-sided 95 % Student t quantile: the Cornish-Fisher
// expansion around z(0.95), within 1e-4 of the exact value for df >= 5
// (Abramowitz & Stegun 26.7.5). Welch df here is at least 29.
func studentT95(df float64) float64 {
	const z = 1.6448536269514722
	z2 := z * z
	z3 := z2 * z
	z5 := z3 * z2
	z7 := z5 * z2
	z9 := z7 * z2
	g1 := (z3 + z) / 4
	g2 := (5*z5 + 16*z3 + 3*z) / 96
	g3 := (3*z7 + 19*z5 + 17*z3 - 15*z) / 384
	g4 := (79*z9 + 776*z7 + 1482*z5 - 1920*z3 - 945*z) / 92160
	return z + g1/df + g2/(df*df) + g3/(df*df*df) + g4/(df*df*df*df)
}

// nullDocumentIsolated: the NULL key reached no document text. An unchanged revision
// proves it; a changed one is accepted only with recorded paths, none in "text"
// (App work such as the device claim after a 2 s pause). Its cost stays in the
// NULL sample, so it can only make the NULL budget harder to meet.
func nullDocumentIsolated(s Sample) bool {
	if s.RevisionAfter == s.RevisionBefore {
		return len(s.NullForeignPaths) == 0
	}
	if len(s.NullForeignPaths) == 0 {
		return false
	}
	for _, path := range s.NullForeignPaths {
		if path == "" || path == "text" || strings.HasPrefix(path, "text/") {
			return false
		}
	}
	return true
}

func clippedWork(spans []FineSpan, start, end float64) float64 {
	parts := []Handler{}
	for _, s := range spans {
		if !finite(s.Start) || !finite(s.End) || s.End < s.Start {
			continue
		}
		a, b := math.Max(start, s.Start), math.Min(end, s.End)
		if b > a {
			parts = append(parts, Handler{Start: a, End: b})
		}
	}
	raw, _ := handlerWork(parts, 0)
	return raw
}

// Nested stages are unioned, never added as if independent. The remainder is
// explicitly unattributed: it can include browser work, harness, clock rounding
// and task-queue delay. It is not claimed to be application or pure instrument
// time, and it does not replace any budgeted raw/conservative measurement.
func (v *verifier) attribute(s Sample, raw float64) {
	if v.attributed == nil {
		v.attributed = map[string][]float64{}
	}
	for _, outer := range s.Handlers {
		if outer.Event != "keystroke-task" {
			continue
		}
		callbacks := clippedWork(s.Tasks, outer.Start, outer.End)
		v.attributed["outer_browser_bracket"] = append(v.attributed["outer_browser_bracket"], outer.End-outer.Start)
		core := []FineSpan{}
		for _, h := range s.Handlers {
			if h.Event == "codemirror-input" {
				core = append(core, FineSpan{Start: h.Start, End: h.End})
			}
		}
		v.attributed["raw_handler_including_harness_and_queue"] = append(v.attributed["raw_handler_including_harness_and_queue"], raw)
		v.attributed["recorded_app_callbacks_union"] = append(v.attributed["recorded_app_callbacks_union"], callbacks)
		v.attributed["codemirror_dispatch_inclusive"] = append(v.attributed["codemirror_dispatch_inclusive"], clippedWork(core, outer.Start, outer.End))
		for _, name := range []string{"automerge-ui", "language-didChange"} {
			spans := []FineSpan{}
			for _, h := range s.Stages {
				if h.Event == name {
					spans = append(spans, FineSpan{Start: h.Start, End: h.End})
				}
			}
			v.attributed[name+"_during_input_inclusive"] = append(v.attributed[name+"_during_input_inclusive"], clippedWork(spans, outer.Start, outer.End))
			v.attributed[name+"_whole_settle_inclusive"] = append(v.attributed[name+"_whole_settle_inclusive"], clippedWork(spans, s.KeyDownAt, s.SettledAt))
		}
		v.attributed["unattributed_harness_browser_or_queue"] = append(v.attributed["unattributed_harness_browser_or_queue"], math.Max(0, outer.End-outer.Start-callbacks))
		for _, h := range s.Handlers {
			if h.Event == "input" {
				v.attributed["post_input_task_tail"] = append(v.attributed["post_input_task_tail"], math.Max(0, outer.End-h.End))
			}
		}
	}
}
