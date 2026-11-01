// SPDX-License-Identifier: MPL-2.0

package conformance_test

import (
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

func TestCorpus(t *testing.T) {
	results, err := conformance.Run(1, nil)
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
