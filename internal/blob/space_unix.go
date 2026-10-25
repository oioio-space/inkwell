//go:build unix

package blob

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// freeSpace returns the bytes a user may still write on the file system that
// holds dir.
func freeSpace(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, fmt.Errorf("blob: statfs %s: %w", dir, err)
	}
	blocks := uint64(st.Bavail) //nolint:unconvert // uint64 on Linux, int64 on the BSDs.
	size := uint64(st.Bsize)    // #nosec G115 -- a block size is positive.
	return blocks * size, nil
}
