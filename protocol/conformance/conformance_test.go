// SPDX-License-Identifier: MPL-2.0

package conformance_test

import (
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

// Spec: TestCorpus runs the corpus at CurrentStep (default) or INKWELL_PROTOCOL_STEP.
func TestCorpus(t *testing.T) {
	step := conformance.StepFromEnv()
	results, err := conformance.Run(step, conformance.Strict)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		t.Run(r.Vector.File+"/"+r.Vector.ID, func(t *testing.T) {
			if !r.Required {
				t.Logf("future: %v", r.Err)
				return
			}
			if !r.Passed {
				t.Fatal(r.Err)
			}
		})
	}
}
