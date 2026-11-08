// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"bytes"
	"testing"
)

// recorder is a testing.TB that records Fatal instead of stopping the test.
type recorder struct {
	testing.TB
	fataled bool
}

func (r *recorder) Helper()      {}
func (r *recorder) Fatal(...any) { r.fataled = true }

// Unhex is how every protocol test reads input_hex from the vectors: valid hex decodes exactly,
// and invalid hex fails the test instead of silently yielding fewer bytes.
func TestUnhex(t *testing.T) {
	r := &recorder{TB: t}
	if got := Unhex(r, "00ff10"); !bytes.Equal(got, []byte{0x00, 0xff, 0x10}) || r.fataled {
		t.Errorf("Unhex(%q) = %x, fataled %v; want 00ff10, no failure", "00ff10", got, r.fataled)
	}
	for _, bad := range []string{"0", "zz", "0g"} {
		r := &recorder{TB: t}
		Unhex(r, bad)
		if !r.fataled {
			t.Errorf("Unhex(%q) did not fail the test", bad)
		}
	}
}
