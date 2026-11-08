// SPDX-License-Identifier: MPL-2.0

// Package testutil holds helpers shared by the protocol module's tests.
package testutil

import (
	"encoding/hex"
	"testing"
)

// Unhex decodes a hex string, failing the test on error.
func Unhex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
