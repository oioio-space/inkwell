// SPDX-License-Identifier: MPL-2.0

package admit_test

import (
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

// TestAdmitVectors: the corpus vectors/admit.json (spec §16.1, block RF1: a bound, the bound + 1 and the
// recovery per rule) is this package's normative test, run here as well as in protocol/conformance, so that
// mutation testing of the ledger sees it.
func TestAdmitVectors(t *testing.T) {
	results, err := conformance.Run(conformance.StepFromEnv(), func(v conformance.Vector) error {
		if v.File != "admit.json" {
			return nil
		}
		return conformance.Reference(v)
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range results {
		if r.Vector.File != "admit.json" {
			continue
		}
		n++
		if !r.Passed {
			t.Errorf("%s: %v", r.Vector.ID, r.Err)
		}
	}
	if n == 0 {
		t.Fatal("no admit vector")
	}
}
