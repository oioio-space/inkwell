package sidecar

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// On our supported 64-bit Linux ABIs, siginfo_t's union begins at byte 16;
// _sigchld.si_pid is its first member (asm-generic/siginfo.h). WNOWAIT lets
// the init inspect the selected target before reaping it, while continuing
// to reap orphaned descendants as soon as they die.
func siginfoChildPID(info *unix.Siginfo) int {
	type childInfo struct{ signo, errno, code, padding, pid int32 }
	return int((*childInfo)(unsafe.Pointer(info)).pid) // #nosec G103 -- kernel-filled siginfo_t; childInfo matches the supported 64-bit Linux SIGCHLD ABI prefix.
}
