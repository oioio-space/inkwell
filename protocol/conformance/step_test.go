// SPDX-License-Identifier: MPL-2.0

package conformance_test

import (
	"testing"

	"github.com/oioio-space/inkwell/protocol/conformance"
)

// ADR 0034 "Migration": plain `go test` (the CI) must enforce every vector up to the current
// step, so a regression of a finished step fails the CI and not only that step's gate.
func TestStepFromEnvDefaultsToCurrentStep(t *testing.T) {
	t.Setenv("INKWELL_PROTOCOL_STEP", "")
	if got := conformance.StepFromEnv(); got != conformance.CurrentStep {
		t.Fatalf("StepFromEnv() = %d with no override, want CurrentStep %d", got, conformance.CurrentStep)
	}
}

// A gate may pin another step explicitly (gates/P2.toml runs step 2).
func TestStepFromEnvHonoursAnExplicitStep(t *testing.T) {
	t.Setenv("INKWELL_PROTOCOL_STEP", "2")
	if got := conformance.StepFromEnv(); got != 2 {
		t.Fatalf("StepFromEnv() = %d, want 2", got)
	}
}

// A malformed override never lowers enforcement silently: it falls back to the current step.
func TestStepFromEnvIgnoresAMalformedValue(t *testing.T) {
	t.Setenv("INKWELL_PROTOCOL_STEP", "three")
	if got := conformance.StepFromEnv(); got != conformance.CurrentStep {
		t.Fatalf("StepFromEnv() = %d for a malformed value, want CurrentStep %d", got, conformance.CurrentStep)
	}
}
