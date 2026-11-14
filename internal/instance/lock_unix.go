//go:build unix

package instance

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock takes the exclusive lock of f without waiting: flock(LOCK_EX|LOCK_NB). The kernel
// releases it when the last descriptor of f closes, a crash included.
func tryLock(f *os.File) error {
	raw, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf("instance: lock: %w", err)
	}
	var lerr error
	if err := raw.Control(func(fd uintptr) { lerr = unix.Flock(int(fd), unix.LOCK_EX|unix.LOCK_NB) }); err != nil {
		return fmt.Errorf("instance: lock: %w", err)
	}
	return lockError(lerr)
}

// lockError maps the error of flock: held by another process, impossible on this filesystem
// (fail closed, R18), or another failure.
func lockError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EWOULDBLOCK):
		return errHeld
	case errors.Is(err, unix.ENOLCK), errors.Is(err, unix.EOPNOTSUPP):
		return fmt.Errorf("%w: %w", ErrNoLock, err)
	}
	return fmt.Errorf("instance: lock: %w", err)
}
