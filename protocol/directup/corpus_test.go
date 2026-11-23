// SPDX-License-Identifier: MPL-2.0

package directup_test

import (
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

// Spec §8.6: the corpus vectors/directup.json (classification table, roles, ladders, budgets, address
// selection) is this package's normative test, run here as well as in protocol/conformance, so that
// mutation testing of this package sees it.
func TestCorpus(t *testing.T) {
	results, err := conformance.Run(conformance.StepFromEnv(), func(v conformance.Vector) error {
		if v.File != "directup.json" {
			return nil
		}
		return conformance.Reference(v)
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range results {
		if r.Vector.File != "directup.json" {
			continue
		}
		n++
		if !r.Required || !r.Passed {
			t.Errorf("%s: required %v, passed %v: %v", r.Vector.ID, r.Required, r.Passed, r.Err)
		}
	}
	if n == 0 {
		t.Fatal("no directup vector")
	}
}
