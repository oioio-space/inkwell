// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/protocol/blob"
)

// corpus returns every embedded vector of the given decoder and verdict.
func corpus(t *testing.T, decoder, verdict string) []Vector {
	t.Helper()
	results, err := Run(CurrentStep, func(Vector) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var out []Vector
	for _, r := range results {
		if r.Vector.Decoder == decoder && r.Vector.Verdict == verdict {
			out = append(out, r.Vector)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s/%s vector in the corpus", decoder, verdict)
	}
	return out
}

// corpusVector returns the vector with the given ID.
func corpusVector(t *testing.T, id string) Vector {
	t.Helper()
	results, err := Run(CurrentStep, func(Vector) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Vector.ID == id {
			return r.Vector
		}
	}
	t.Fatalf("vector %q not in the corpus", id)
	return Vector{}
}

// with returns a copy of v whose verdict, decoder and fields are overridden. A nil field
// value removes the field; every other value is JSON-encoded.
func with(t *testing.T, v Vector, verdict string, fields map[string]any) Vector {
	t.Helper()
	out := v
	out.Verdict = verdict
	out.Fields = maps.Clone(v.Fields)
	for k, val := range fields {
		if val == nil {
			delete(out.Fields, k)
			continue
		}
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		out.Fields[k] = raw
	}
	return out
}

// Spec ADR 0034 "Verdict": a vector whose verdict disagrees with the decoder fails, and the
// error says which side was wrong (a rejected decode is wrapped, an accepted one is named).
func TestReferenceVerdictMismatchMessages(t *testing.T) {
	missing := corpusVector(t, "blob-missing")

	err := Reference(with(t, missing, "reject", nil))
	if err == nil || !strings.Contains(err.Error(), "expected reject, decoder accepted input") {
		t.Errorf("Reference(accepted input, verdict reject) = %v, want the accepted-input error", err)
	}

	err = Reference(with(t, missing, "accept", map[string]any{"input_hex": ""}))
	if !errors.Is(err, blob.ErrMalformed) || !strings.HasPrefix(err.Error(), "verdict accept: ") {
		t.Errorf("Reference(rejected input, verdict accept) = %v, want a wrapped blob.ErrMalformed", err)
	}
}

// Spec ADR 0034 "Verdict": the round trip Encode(Decode(b)) == b is part of acceptance, for
// any decoder. The shipped decoders are canonical, so a stand-in decoder proves the rule.
func TestReferenceRejectsBinaryRoundTripMismatch(t *testing.T) {
	const name = "x-noncanonical"
	t.Cleanup(func() { delete(decoderTable, name) })
	v := Vector{Decoder: name, Verdict: "accept", Fields: map[string]json.RawMessage{"input_hex": json.RawMessage(`"0102"`)}}
	for _, tc := range []struct {
		desc    string
		encoded func(in []byte) []byte
	}{
		{"re-encodes to other bytes", func(in []byte) []byte { return append(bytes.Clone(in), 0) }},
		// A missing re-encoding must not skip the check (it did, silently, before 2026-11-08).
		{"never re-encodes", func([]byte) []byte { return nil }},
	} {
		decoderTable[name] = decoderSpec{hasInput: true, check: func(c *referenceCase) error {
			c.roundTrip = true
			c.encoded = tc.encoded(c.input)
			return nil
		}}
		if err := Reference(v); err == nil || !strings.Contains(err.Error(), "round trip") {
			t.Errorf("Reference(accept, decoder %s) = %v, want a round trip error", tc.desc, err)
		}
	}
}

// Spec bulk.json "blob-*": blake3_hex is the hash at input[1:33]. A 33-byte frame (Missing)
// is the smallest one that carries it; a rejected frame is not held to the expectation.
func TestReferenceBlobHashExpectation(t *testing.T) {
	missing := corpusVector(t, "blob-missing")
	var in string
	if err := missing.Decode("input_hex", &in); err != nil {
		t.Fatal(err)
	}
	hash := in[2:66]

	if err := Reference(with(t, missing, "accept", map[string]any{"expect": map[string]string{"blake3_hex": hash}})); err != nil {
		t.Errorf("Reference(33-byte frame, matching hash) = %v", err)
	}
	wrong := strings.Repeat("00", 32)
	if err := Reference(with(t, missing, "accept", map[string]any{"expect": map[string]string{"blake3_hex": wrong}})); err == nil {
		t.Error("Reference(33-byte frame, wrong hash) = nil, want a hash mismatch")
	}
	if err := Reference(with(t, missing, "reject", map[string]any{"input_hex": "", "expect": map[string]string{"blake3_hex": hash}})); err != nil {
		t.Errorf("Reference(rejected frame with hash expectation) = %v, want nil", err)
	}
	// A malformed blake3_hex (not a string) is a broken fixture, never a skipped check.
	if err := Reference(with(t, missing, "accept", map[string]any{"expect": map[string]any{"blake3_hex": 5}})); err == nil {
		t.Error("Reference(33-byte frame, blake3_hex = 5) = nil, want a broken-fixture error")
	}
}

// Spec conformance schema: an expectation, or the input, that is present but malformed is a
// broken fixture and fails the vector even when the decoder rejects as the verdict says.
func TestReferenceBrokenFixtureFieldsFail(t *testing.T) {
	missing := corpusVector(t, "blob-missing")
	for name, v := range map[string]Vector{
		"string expectation of the wrong type": {Decoder: "signature", Verdict: "reject", Fields: map[string]json.RawMessage{
			"input_hex": json.RawMessage(`"00"`),
			"expect":    json.RawMessage(`{"public_hex":42,"signed_bytes_hex":"00"}`),
		}},
		"keys_hex on a non-key decoder without input": with(t, missing, "reject", map[string]any{
			"input_hex": nil, "expect": map[string]any{"keys_hex": []string{"00"}},
		}),
	} {
		if err := Reference(v); err == nil {
			t.Errorf("%s: Reference = nil, want an error", name)
		}
	}

	ticket := Vector{Decoder: "ticket_text", Verdict: "reject", Fields: map[string]json.RawMessage{
		"expect": json.RawMessage(`{"text":42}`),
	}}
	if err := Reference(ticket); err == nil {
		t.Error("ticket_text with a non-string text: Reference = nil, want an error")
	}
}

// Spec lobby.json "ticket_text": a text that does not parse is rejected, and the round-trip
// rule applies only to a ticket that parsed.
func TestReferenceTicketRejectsGarbage(t *testing.T) {
	v := Vector{Decoder: "ticket_text", Verdict: "reject", Fields: map[string]json.RawMessage{
		"expect": json.RawMessage(`{"text":"not a ticket"}`),
	}}
	if err := Reference(v); err != nil {
		t.Fatalf("Reference(garbage ticket, reject) = %v, want nil", err)
	}
}

// Spec crypto.json "sig-domain-1": for the domain-separated form (no decoder) the signed bytes
// are rebuilt as "inkwell/<domain>/v1\x00" || message and signed_bytes_hex is not trusted;
// for the "signature" form the signed bytes are exactly signed_bytes_hex.
func TestReferenceSignedBytesConstruction(t *testing.T) {
	domain := corpusVector(t, "sig-domain-1")
	if err := Reference(with(t, domain, "accept", nil)); err != nil {
		t.Fatalf("sig-domain-1 = %v", err)
	}
	var expect map[string]string
	if err := domain.Decode("expect", &expect); err != nil {
		t.Fatal(err)
	}

	bogusSigned := maps.Clone(expect)
	bogusSigned["signed_bytes_hex"] = "00"
	if err := Reference(with(t, domain, "accept", map[string]any{"expect": bogusSigned})); err != nil {
		t.Errorf("domain form must ignore signed_bytes_hex, got %v", err)
	}

	plain := Vector{Decoder: "signature", Verdict: "accept", Fields: map[string]json.RawMessage{}}
	plain = with(t, plain, "accept", map[string]any{
		"input_hex": expect["signature_hex"],
		"expect":    map[string]string{"public_hex": expect["public_hex"], "signed_bytes_hex": expect["signed_bytes_hex"]},
	})
	if err := Reference(plain); err != nil {
		t.Errorf("signature form must verify signed_bytes_hex as given, got %v", err)
	}
}

// Spec repo.json: an accepted message is checked against every expectation but "reason".
// Property: for each accepted repo vector, a contradicting or unknown expectation fails it,
// while a "reason" (documentation of a rejection) is ignored.
func TestReferenceRepoExpectations(t *testing.T) {
	for _, v := range corpus(t, "repo_message", "accept") {
		t.Run(v.ID, func(t *testing.T) {
			if err := Reference(v); err != nil {
				t.Fatalf("corpus vector: %v", err)
			}
			var expect map[string]json.RawMessage
			if err := v.Decode("expect", &expect); err != nil {
				t.Fatal(err)
			}
			add := func(k string, val any) Vector {
				e := make(map[string]any, len(expect)+1)
				for key, raw := range expect {
					e[key] = raw
				}
				e[k] = val
				return with(t, v, "accept", map[string]any{"expect": e})
			}
			if err := Reference(add("reason", "documentation only")); err != nil {
				t.Errorf("reason must be ignored: %v", err)
			}
			for _, k := range slices.Sorted(maps.Keys(expect)) {
				if k == "reason" {
					continue
				}
				if err := Reference(add(k, "contradiction")); err == nil {
					t.Errorf("contradicting %q was accepted", k)
				}
			}
			if err := Reference(add("noSuchField", 1)); err == nil {
				t.Error("an expectation on an absent field was accepted")
			}
		})
	}
}

// Spec fold.json: the fold is checked against the expected members, in both application
// orders, and an op that does not decode fails the vector.
func TestReferenceFoldExpectations(t *testing.T) {
	v := corpusVector(t, "fold-seniority")
	if err := Reference(v); err != nil {
		t.Fatalf("corpus vector: %v", err)
	}
	var expect struct {
		Members []map[string]any `json:"members"`
	}
	if err := v.Decode("expect", &expect); err != nil {
		t.Fatal(err)
	}
	for field, wrong := range map[string]any{"role": 1, "suspended": true, "public_hex": strings.Repeat("00", 32)} {
		members := slices.Clone(expect.Members)
		members[0] = maps.Clone(members[0])
		members[0][field] = wrong
		if err := Reference(with(t, v, "accept", map[string]any{"expect": map[string]any{"members": members}})); err == nil {
			t.Errorf("wrong member %s accepted", field)
		}
	}
	if err := Reference(with(t, v, "accept", map[string]any{"inputs_hex": []string{"00"}})); err == nil {
		t.Error("an undecodable op was accepted")
	}
}
