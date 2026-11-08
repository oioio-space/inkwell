// SPDX-License-Identifier: MPL-2.0

package sig_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/oioio-space/inkwell/protocol/sig"
	"github.com/oioio-space/inkwell/protocol/vectors"
)

type cryptoVector struct {
	ID       string `json:"id"`
	Decoder  string `json:"decoder"`
	Input    string `json:"input_hex"`
	Verdict  string `json:"verdict"`
	Enforce  string `json:"enforce"`
	Status   string `json:"status"`
	Expected struct {
		Domain      string   `json:"domain"`
		Message     string   `json:"message_hex"`
		SignedBytes string   `json:"signed_bytes_hex"`
		Public      string   `json:"public_hex"`
		Signature   string   `json:"signature_hex"`
		Keys        []string `json:"keys_hex"`
	} `json:"expect"`
}

func cryptoVectors(t testing.TB) []cryptoVector {
	t.Helper()
	data, err := vectors.FS.ReadFile("crypto.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Vectors []cryptoVector `json:"vectors"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.Vectors
}

func unhex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVerifyVectors(t *testing.T) {
	for _, v := range cryptoVectors(t) {
		switch v.Decoder {
		case "public_key":
			keys := v.Expected.Keys
			if len(keys) == 0 {
				keys = []string{v.Input}
			}
			for _, k := range keys {
				err := sig.ValidatePublicKey(unhex(t, k))
				if (err == nil) != (v.Verdict == "accept") {
					t.Errorf("%s: ValidatePublicKey verdict = %v, want %s", v.ID, err, v.Verdict)
				}
			}
		case "signature", "signature+key", "":
			key := unhex(t, v.Expected.Public)
			signature := unhex(t, v.Expected.Signature)
			if v.Expected.Signature == "" {
				signature = unhex(t, v.Input)
			}
			msg := unhex(t, v.Expected.SignedBytes)
			if v.Decoder == "" {
				msg = append([]byte("inkwell/"+v.Expected.Domain+"/v1\x00"), unhex(t, v.Expected.Message)...)
			}
			err := sig.Verify(key, msg, signature)
			if (err == nil) != (v.Verdict == "accept") {
				t.Errorf("%s: Verify verdict = %v, want %s", v.ID, err, v.Verdict)
			}
		}
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	valid := cryptoVectors(t)
	var key, msg, signature []byte
	for _, v := range valid {
		if v.ID == "sig-domain-1" {
			key = unhex(t, v.Expected.Public)
			msg = unhex(t, v.Expected.SignedBytes)
			signature = unhex(t, v.Expected.Signature)
		}
	}
	if sig.Verify(key, msg, signature) != nil {
		t.Fatal("fixture signature rejected")
	}
	for name, tc := range map[string]struct{ key, msg, signature []byte }{
		"short key":       {key[:31], msg, signature},
		"long key":        {append(append([]byte{}, key...), 0), msg, signature},
		"short signature": {key, msg, signature[:63]},
		"long signature":  {key, msg, append(append([]byte{}, signature...), 0)},
		"wrong message":   {key, append([]byte("x"), msg...), signature},
	} {
		if err := sig.Verify(tc.key, tc.msg, tc.signature); err == nil {
			t.Errorf("%s: Verify accepted malformed input", name)
		}
	}
}

func FuzzVerify(f *testing.F) {
	for _, v := range cryptoVectors(f) {
		if v.Expected.Public == "" {
			continue
		}
		key := unhex(f, v.Expected.Public)
		signature := unhex(f, v.Expected.Signature)
		if v.Expected.Signature == "" {
			signature = unhex(f, v.Input)
		}
		msg := unhex(f, v.Expected.SignedBytes)
		if v.Decoder == "" {
			msg = append([]byte("inkwell/"+v.Expected.Domain+"/v1\x00"), unhex(f, v.Expected.Message)...)
		}
		f.Add(key, msg, signature)
	}
	f.Fuzz(func(t *testing.T, key, msg, signature []byte) {
		// Hostile keys and signatures must never panic.
		_ = sig.Verify(key, msg, signature)
	})
}
