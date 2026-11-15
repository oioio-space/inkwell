package sidecar

import "golang.org/x/sys/unix"

// sysForkNumbers are x/sys's fork and vfork numbers on this architecture.
func sysForkNumbers() (fork, vfork uint32) { return unix.SYS_FORK, unix.SYS_VFORK }
