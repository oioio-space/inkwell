//go:build unix

package launch

import (
	"os"
	"syscall"
)

// device returns the device number of the filesystem holding path; false when it cannot tell.
func device(path string) (uint64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true //nolint:unconvert // #nosec G115 -- Dev is uint64 on Linux, int32 on macOS; never negative
}
