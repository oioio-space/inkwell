//go:build windows

package instance

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes the exclusive lock of the first byte of f without waiting:
// LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY). The system releases it when the
// process ends. Built and tested at N35.
func tryLock(f *os.File) error {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION):
		return errHeld
	case errors.Is(err, windows.ERROR_NOT_SUPPORTED):
		return fmt.Errorf("%w: %w", ErrNoLock, err)
	}
	return fmt.Errorf("instance: lock: %w", err)
}
