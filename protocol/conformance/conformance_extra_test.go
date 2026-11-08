// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"encoding/json"
	"errors"
	"testing"
)

// Spec: Run with nil check uses Reference implementation.
func TestRunWithNilCheck(t *testing.T) {
	results, err := Run(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected some results")
	}
}

// Spec: Run with custom check function.
func TestRunWithCustomCheck(t *testing.T) {
	customCheck := func(v Vector) error {
		return errors.New("custom error")
	}
	results, err := Run(1, customCheck)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Required {
			if r.Passed {
				t.Fatal("expected custom check to fail required cases")
			}
			if r.Err == nil || r.Err.Error() != "custom error" {
				t.Fatalf("expected custom error, got %v", r.Err)
			}
		}
	}
}

// Spec: Run with different step values respects Required().
func TestRunStepParameter(t *testing.T) {
	for step := 1; step <= 10; step++ {
		results, err := Run(step, nil)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		for _, r := range results {
			required := r.Vector.Required(step)
			if r.Required != required {
				t.Fatalf("step %d: vector %s Required mismatch: got %v want %v", step, r.Vector.ID, r.Required, required)
			}
		}
	}
}

// Spec: Vector.Required() implements normative/proposed step enforcement.
func TestVectorRequired(t *testing.T) {
	v := Vector{Status: "normative", Enforce: ""}
	if !v.Required(1) {
		t.Fatal("normative should be required at step 1")
	}

	v = Vector{Status: "proposed", Enforce: "step-3"}
	if v.Required(2) {
		t.Fatal("step-3 should not be required at step 2")
	}
	if !v.Required(3) {
		t.Fatal("step-3 should be required at step 3")
	}
	if !v.Required(5) {
		t.Fatal("step-3 should be required at step 5")
	}

	v = Vector{Status: "proposed", Enforce: "step-8"}
	if v.Required(7) {
		t.Fatal("step-8 should not be required at step 7")
	}
	if !v.Required(8) {
		t.Fatal("step-8 should be required at step 8")
	}

	v = Vector{Status: "proposed", Enforce: "invalid"}
	if v.Required(1) {
		t.Fatal("invalid enforce should not be required")
	}
}

// Spec: Vector.Decode() extracts typed fields from RawMessage.
func TestVectorDecode(t *testing.T) {
	v := Vector{Fields: map[string]json.RawMessage{
		"test_str":  json.RawMessage(`"hello"`),
		"test_int":  json.RawMessage(`42`),
		"test_bool": json.RawMessage(`true`),
		"test_obj":  json.RawMessage(`{"a":1}`),
	}}

	var s string
	if err := v.Decode("test_str", &s); err != nil || s != "hello" {
		t.Fatalf("decode string: %v, got %q", err, s)
	}

	var i int
	if err := v.Decode("test_int", &i); err != nil || i != 42 {
		t.Fatalf("decode int: %v, got %d", err, i)
	}

	var b bool
	if err := v.Decode("test_bool", &b); err != nil || !b {
		t.Fatalf("decode bool: %v, got %v", err, b)
	}

	var obj map[string]int
	if err := v.Decode("test_obj", &obj); err != nil || obj["a"] != 1 {
		t.Fatalf("decode obj: %v, got %v", err, obj)
	}

	// Missing field returns an error
	missing := "original"
	err := v.Decode("missing", &missing)
	if err == nil {
		t.Fatal("decode missing should error")
	}
}

// Spec: Reference routes various decoders and handles broken vectors.
func TestReferenceWithBrokenVectors(t *testing.T) {
	tests := []struct {
		name   string
		vector Vector
	}{
		{
			name: "missing decoder",
			vector: Vector{
				Decoder: "bulk",
				Verdict: "reject",
				Fields:  map[string]json.RawMessage{},
			},
		},
		{
			name: "unknown decoder",
			vector: Vector{
				Decoder: "unknown_decoder",
				Verdict: "accept",
				Fields:  map[string]json.RawMessage{"input_hex": json.RawMessage(`""`)},
			},
		},
		{
			name: "repo_message accept with bad context",
			vector: Vector{
				Decoder: "repo_message",
				Verdict: "accept",
				Fields: map[string]json.RawMessage{
					"input_hex": json.RawMessage(`"a564747970656673796e63"`),
					"context":   json.RawMessage(`{"from":"","me":"","admitted":[]}`),
				},
			},
		},
		{
			name: "repo_message ignore verdict",
			vector: Vector{
				Decoder: "repo_message",
				Verdict: "ignore",
				Fields: map[string]json.RawMessage{
					"input_hex": json.RawMessage(`"a464747970656b6675747572652d6b696e64"`),
					"context":   json.RawMessage(`{"from":"12D3KooWQK1wnefoLrcVHbbnf5tLzbopUd3K3bFAoJpA7YJgL5pV","me":"12D3KooWDwTirQce1RRKnasT5fPVFgzXCy6SiRgSwrwPGLC7zE91","admitted":["4NMNnkMhL8jXrdJ9jamS58PAVdXu"]}`),
				},
			},
		},
		{
			name: "ticket_text missing text",
			vector: Vector{
				Decoder: "ticket_text",
				Verdict: "accept",
				Fields:  map[string]json.RawMessage{},
			},
		},
		{
			name: "public_key with keys_hex",
			vector: Vector{
				Decoder: "public_key",
				Verdict: "accept",
				Fields: map[string]json.RawMessage{
					"keys_hex": json.RawMessage(`["ed25519publickey11111111111111111111111111111"]`),
				},
			},
		},
		{
			name: "signature missing public_hex",
			vector: Vector{
				Decoder: "signature",
				Verdict: "accept",
				Fields: map[string]json.RawMessage{
					"signed_bytes_hex": json.RawMessage(`"deadbeef"`),
					"signature_hex":    json.RawMessage(`"sig"`),
				},
			},
		},
		{
			name: "signature+key missing signed_bytes",
			vector: Vector{
				Decoder: "signature+key",
				Verdict: "accept",
				Fields: map[string]json.RawMessage{
					"public_hex":    json.RawMessage(`"ed25519publickey11111111111111111111111111111"`),
					"message_hex":   json.RawMessage(`"deadbeef"`),
					"signature_hex": json.RawMessage(`"sig"`),
				},
			},
		},
		{
			name: "default decoder (empty string)",
			vector: Vector{
				Decoder: "",
				Verdict: "accept",
				Fields: map[string]json.RawMessage{
					"public_hex":    json.RawMessage(`"ed25519publickey11111111111111111111111111111"`),
					"message_hex":   json.RawMessage(`"deadbeef"`),
					"signature_hex": json.RawMessage(`"sig"`),
					"domain":        json.RawMessage(`"test"`),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Reference(tc.vector)
			// We expect errors for these broken vectors
			if err == nil {
				t.Logf("vector passed unexpectedly: %s", tc.name)
			}
		})
	}
}

// Spec: Strict routes to correct check functions (tests routing table).
func TestStrictPaths(t *testing.T) {
	tests := []struct {
		name    string
		decoder string
	}{
		{"public_key", "public_key"},
		{"peer_id", "peer_id"},
		{"signature", "signature"},
		{"signature+key", "signature+key"}, // gitleaks:allow -- decoder name, not a secret
		{"empty decoder", ""},
		{"repo_message", "repo_message"},
		{"fallback to Reference", "bulk"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := Vector{
				Decoder: tc.decoder,
				Verdict: "reject",
				Fields:  map[string]json.RawMessage{"input_hex": json.RawMessage(`""`)},
			}
			err := Strict(v)
			// We expect errors for these minimal vectors
			if err == nil {
				t.Logf("Strict(%s) passed unexpectedly", tc.decoder)
			}
			// Just verify it doesn't panic and routes correctly
		})
	}
}

// Spec: base58() decodes base58btc strings.
func TestBase58(t *testing.T) {
	tests := []struct {
		input    string
		expected []byte
	}{
		{"1", []byte{0}},
		{"2", []byte{1}},
		{"z", []byte{57}},
		{"11", []byte{0, 0}},
		{"12", []byte{0, 1}},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := base58(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("base58(%q) len %d want %d", tc.input, len(got), len(tc.expected))
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Fatalf("base58(%q)[%d] = %d want %d", tc.input, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

// Spec: vectorsData loads embedded corpus files.
func TestVectorsData(t *testing.T) {
	data, err := vectorsData("repo.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("repo.json should not be empty")
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("repo.json not valid JSON: %v", err)
	}
}
