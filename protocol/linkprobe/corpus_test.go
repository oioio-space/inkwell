// SPDX-License-Identifier: MPL-2.0

package linkprobe_test

import (
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

// Spec §8.5.11: the corpus vectors/linkprobe.json is this package's normative test, run here as
// well as in protocol/conformance, so that a change to the state machines is judged against every
// rule the corpus encodes (and mutation testing of this package sees it).
func TestCorpus(t *testing.T) {
	results, err := conformance.Run(conformance.StepFromEnv(), func(v conformance.Vector) error {
		if v.File != "linkprobe.json" {
			return nil
		}
		return conformance.Reference(v)
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range results {
		if r.Vector.File != "linkprobe.json" {
			continue
		}
		n++
		if !r.Passed {
			t.Errorf("%s: %v", r.Vector.ID, r.Err)
		}
	}
	if n == 0 {
		t.Fatal("no linkprobe vector")
	}
}
