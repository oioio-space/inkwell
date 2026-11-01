// SPDX-License-Identifier: MPL-2.0

package conformance_test

import (
	"fmt"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

func ExampleRun() {
	results, err := conformance.Run(1, nil)
	if err != nil {
		panic(err)
	}
	passed := true
	for _, r := range results {
		if r.Required && !r.Passed {
			passed = false
		}
	}
	fmt.Println(passed)
	// Output: true
}
