// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"encoding/json"
	"maps"
	"testing"
)

// The link-probe runner (spec §21) must reject a case that does not hold: every expectation it
// reads is compared, every malformed field is refused. Each test perturbs one thing of a corpus
// vector that passes and requires the reference to report it.

func linkprobeScripts(t *testing.T) []Vector {
	t.Helper()
	var out []Vector
	for _, d := range []string{"linkprobe_prober", "linkprobe_responder", "linkprobe_host"} {
		out = append(out, corpus(t, d, "accept")...)
	}
	return out
}

// wrongExpect returns a JSON expectation that no event can produce: a string, a number or an
// action list of the same JSON kind as the original.
func wrongExpect(raw json.RawMessage) any {
	switch {
	case len(raw) > 0 && raw[0] == '[':
		return []string{"no_such_action"}
	case len(raw) > 0 && raw[0] == '"':
		return "no_such_outcome"
	default:
		return 12345
	}
}

// Spec §21, scripts: changing the expectation of any one event makes the case fail.
func TestLinkprobeEveryExpectationIsCompared(t *testing.T) {
	for _, v := range linkprobeScripts(t) {
		var es []map[string]json.RawMessage
		if err := v.Decode("events", &es); err != nil {
			t.Fatal(err)
		}
		for i := range es {
			if _, ok := es[i]["expect"]; !ok {
				continue
			}
			bad := make([]map[string]json.RawMessage, len(es))
			copy(bad, es)
			ev := maps.Clone(es[i])
			w, _ := json.Marshal(wrongExpect(es[i]["expect"]))
			ev["expect"] = w
			bad[i] = ev
			if err := Reference(with(t, v, v.Verdict, map[string]any{"events": bad})); err == nil {
				t.Errorf("%s: event %d with a wrong expectation passes", v.ID, i)
			}
		}
	}
}

// Spec §21: events go forward in time; a script that goes back is refused.
func TestLinkprobeEventsGoForward(t *testing.T) {
	for _, v := range linkprobeScripts(t) {
		var es []map[string]any
		if err := v.Decode("events", &es); err != nil || len(es) < 2 {
			continue
		}
		es[1]["at_ms"] = -1
		if err := Reference(with(t, v, v.Verdict, map[string]any{"events": es})); err == nil {
			t.Errorf("%s: a script going back in time passes", v.ID)
		}
	}
}

// Spec §21: a script needs events, a message field must be exactly one well-formed message, a
// data field must be hexadecimal, an op and a host context must be known.
func TestLinkprobeMalformedFieldsAreRefused(t *testing.T) {
	for _, v := range linkprobeScripts(t) {
		if err := Reference(with(t, v, v.Verdict, map[string]any{"events": []any{}})); err == nil {
			t.Errorf("%s: no events passes", v.ID)
		}
		if err := Reference(with(t, v, v.Verdict, map[string]any{"events": "x"})); err == nil {
			t.Errorf("%s: undecodable events pass", v.ID)
		}
		var es []map[string]any
		if err := v.Decode("events", &es); err != nil {
			t.Fatal(err)
		}
		for i, e := range es {
			for field, bad := range map[string]any{"message_hex": "05010000000100", "data_hex": "zz", "op": "no_such_op"} {
				if _, ok := e[field]; !ok && field != "op" {
					continue
				}
				mut := make([]map[string]any, len(es))
				for j := range es {
					mut[j] = maps.Clone(es[j])
				}
				mut[i][field] = bad
				if err := Reference(with(t, v, v.Verdict, map[string]any{"events": mut})); err == nil {
					t.Errorf("%s: event %d with %s = %v passes", v.ID, i, field, bad)
				}
			}
		}
	}
	for _, v := range corpus(t, "linkprobe_host", "accept") {
		for _, ctx := range []any{"x", map[string]any{"transport": "carrier-pigeon"}, map[string]any{"transport": "quic", "roles": []string{"king"}}} {
			if err := Reference(with(t, v, v.Verdict, map[string]any{"context": ctx})); err == nil {
				t.Errorf("%s: context %v passes", v.ID, ctx)
			}
		}
	}
}

// Spec §21: message, PTO and level vectors compare their verdict and every expected field.
func TestLinkprobeTablesCompareTheirExpectations(t *testing.T) {
	for _, v := range corpus(t, "linkprobe_message", "accept") {
		var exp map[string]json.RawMessage
		if err := v.Decode("expect", &exp); err != nil {
			t.Fatal(err)
		}
		for k := range exp {
			if err := Reference(with(t, v, v.Verdict, map[string]any{"expect": map[string]any{k: 12345678}})); err == nil {
				t.Errorf("%s: a wrong %s passes", v.ID, k)
			}
		}
		if err := Reference(with(t, v, "reject", nil)); err == nil {
			t.Errorf("%s: a wrong verdict passes", v.ID)
		}
		if err := Reference(with(t, v, v.Verdict, map[string]any{"expect": map[string]any{"no_such_field": 1}})); err == nil {
			t.Errorf("%s: an unknown expected field passes", v.ID)
		}
	}
	for _, v := range corpus(t, "linkprobe_pto", "accept") {
		if err := Reference(with(t, v, v.Verdict, map[string]any{"expect": map[string]any{"pto_ms": -1}})); err == nil {
			t.Errorf("%s: a wrong PTO passes", v.ID)
		}
		if err := Reference(with(t, v, v.Verdict, map[string]any{"context": "x"})); err == nil {
			t.Errorf("%s: an undecodable context passes", v.ID)
		}
	}
	for _, v := range corpus(t, "linkprobe_level", "accept") {
		var exp struct{ Level string }
		if err := v.Decode("expect", &exp); err != nil {
			t.Fatal(err)
		}
		other := map[string]string{"good": "poor", "fair": "good", "poor": "fair", "unknown": "good"}[exp.Level]
		if err := Reference(with(t, v, v.Verdict, map[string]any{"expect": map[string]any{"level": other}})); err == nil {
			t.Errorf("%s: level %s instead of %s passes", v.ID, other, exp.Level)
		}
		var ctx map[string]any
		if err := v.Decode("context", &ctx); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []any{"x", map[string]any{"kind": "jitter", "from": ctx["from"], "value": ctx["value"]}} {
			if err := Reference(with(t, v, v.Verdict, map[string]any{"context": bad})); err == nil {
				t.Errorf("%s: context %v passes", v.ID, bad)
			}
		}
	}
}

// Spec §21: a message vector with trailing bytes is refused (one message per vector).
func TestLinkprobeTrailingBytesAreRefused(t *testing.T) {
	v := corpusVector(t, "linkprobe-probe")
	if err := Reference(with(t, v, "accept", map[string]any{"input_hex": "05010000000100"})); err == nil {
		t.Error("a PROBE with a trailing byte is accepted")
	}
	if err := Reference(with(t, v, "reject", map[string]any{"input_hex": "05010000000100"})); err != nil {
		t.Errorf("a PROBE with a trailing byte, expected rejected: %v", err)
	}
}
