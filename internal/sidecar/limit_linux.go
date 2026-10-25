package sidecar

import (
	"errors"

	"golang.org/x/sys/unix"
)

// limitMemory caps the address space of the process pid. A hostile document
// that asks for more makes the allocation fail inside the child.
func limitMemory(pid int, bytes uint64) error {
	lim := unix.Rlimit{Cur: bytes, Max: bytes}
	err := unix.Prlimit(pid, unix.RLIMIT_AS, &lim, nil)
	if errors.Is(err, unix.ESRCH) { // already gone: nothing left to limit
		return nil
	}
	return err
}
