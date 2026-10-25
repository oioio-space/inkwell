//go:build windows

package blob

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// freeSpace returns the bytes a user may still write on the volume that holds
// dir.
func freeSpace(dir string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, fmt.Errorf("blob: free space of %s: %w", dir, err)
	}
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, nil, nil); err != nil {
		return 0, fmt.Errorf("blob: free space of %s: %w", dir, err)
	}
	return free, nil
}
