//go:build !linux

package typst

import "testing"

func landlockTCPConfinement(t *testing.T) bool {
	t.Helper()
	return false
}
