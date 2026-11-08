// SPDX-License-Identifier: MPL-2.0

package conformance_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

func TestCorpus(t *testing.T) {
	step := 1
	if raw := os.Getenv("INKWELL_PROTOCOL_STEP"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("INKWELL_PROTOCOL_STEP=%q: %v", raw, err)
		}
		step = n
	}
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
