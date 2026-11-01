// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"encoding/json"
	"testing"
)

func TestReferenceRejectsBrokenFields(t *testing.T) {
	for name, fields := range map[string]map[string]json.RawMessage{
		"missing input":      {},
		"wrong input type":   {"input_hex": json.RawMessage(`42`)},
		"bad hex":            {"input_hex": json.RawMessage(`"zz"`)},
		"wrong expectations": {"input_hex": json.RawMessage(`""`), "expect": json.RawMessage(`[]`)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := Reference(Vector{Decoder: "bulk", Verdict: "reject", Fields: fields}); err == nil {
				t.Fatal("broken fixture passed as expected rejection")
			}
		})
	}
	for _, members := range []string{`[]`, `null`, `{}`} {
		if err := Reference(Vector{Decoder: "trust_fold", Verdict: "accept", Fields: map[string]json.RawMessage{"inputs_hex": json.RawMessage(`[]`), "expect": json.RawMessage(`{"members":` + members + `}`)}}); err == nil {
			t.Fatal("invalid fold fixture accepted")
		}
	}
}
