// SPDX-License-Identifier: MPL-2.0

package trust_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/oioio-space/inkwell/protocol/trust"
	"github.com/oioio-space/inkwell/protocol/vectors"
)

// Every proper prefix of a valid signed object must be rejected, regardless
// of whether truncation falls in a fixed field, variable field or signature.
func TestCorpusTruncation(t *testing.T) {
	for _, file := range []string{"trust.json", "attest.json"} {
		data, err := vectors.FS.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var corpus struct {
			Vectors []struct {
				ID      string `json:"id"`
				Decoder string `json:"decoder"`
				Verdict string `json:"verdict"`
				Input   string `json:"input_hex"`
			} `json:"vectors"`
		}
		if err := json.Unmarshal(data, &corpus); err != nil {
			t.Fatal(err)
		}
		for _, v := range corpus.Vectors {
			if v.Verdict != "accept" || (v.Decoder != "trust_op" && v.Decoder != "attestation") {
				continue
			}
			t.Run(v.ID, func(t *testing.T) {
				b, err := hex.DecodeString(v.Input)
				if err != nil {
					t.Fatal(err)
				}
				for n := 0; n < len(b); n++ {
					if v.Decoder == "trust_op" {
						_, err = trust.Decode(b[:n])
					} else {
						_, err = trust.DecodeAttestation(b[:n])
					}
					if err == nil {
						t.Fatalf("accepted prefix of %d/%d bytes", n, len(b))
					}
				}
			})
		}
	}
}
