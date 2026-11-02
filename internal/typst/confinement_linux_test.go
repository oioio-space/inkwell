package typst

import (
	"testing"

	"golang.org/x/sys/unix"
)

// Match the launcher's restrictOutput ABI probe; ABI 4 adds TCP confinement.
func landlockTCPConfinement(t *testing.T) bool {
	t.Helper()
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		t.Logf("Landlock ABI probe: %v; no TCP confinement", errno)
		return false
	}
	return abi >= 4
}
