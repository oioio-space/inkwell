package sidecar

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// pidfdInfo is struct pidfd_info of include/uapi/linux/pidfd.h at its fourth
// published size (PIDFD_INFO_SIZE_VER3, 88 bytes). The kernel copies the
// fields that fit the size encoded in the ioctl command and clears the mask
// bits of those that do not (fs/pidfs.c, pidfd_info).
type pidfdInfo struct {
	mask                                             uint64
	cgroupID                                         uint64
	pid, tgid, ppid                                  uint32
	ruid, rgid, euid, egid, suid, sgid, fsuid, fsgid uint32
	exitCode                                         int32
	coredumpMask, coredumpSignal, coredumpCode, _    uint32
	supportedMask                                    uint64
}

const (
	pidfdInfoExit          = 1 << 3
	pidfdInfoCoredump      = 1 << 4
	pidfdInfoSupportedMask = 1 << 5
	pidfdCoredumped        = 1 << 0
	// pidfdGetInfo is PIDFD_GET_INFO, _IOWR(0xFF, 11, struct pidfd_info).
	pidfdGetInfo = 3<<30 | unsafe.Sizeof(pidfdInfo{})<<16 | 0xFF<<8 | 11
)

// coreDumpOf asks the kernel whether it started a core dump for the process of
// pidfd. The kernel marks the pidfd when it starts the pipe helper or hands
// the dump to a socket (fs/coredump.c, pidfs_coredump), and keeps that mark
// after the reap; for a reaped process without the mark, a kernel that lists
// PIDFD_INFO_COREDUMP among its supported fields started none.
func coreDumpOf(pidfd int) CoreDump {
	if pidfd < 0 {
		return CoreDumpUnknown
	}
	info := pidfdInfo{mask: pidfdInfoExit | pidfdInfoCoredump | pidfdInfoSupportedMask}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(pidfd), pidfdGetInfo, uintptr(unsafe.Pointer(&info))); errno != 0 { // #nosec G103 -- the kernel fills this ABI-sized struct synchronously.
		return CoreDumpUnknown
	}
	switch {
	case info.mask&pidfdInfoCoredump != 0 && info.coredumpMask&pidfdCoredumped != 0:
		return CoreDumpStarted
	case info.mask&pidfdInfoCoredump != 0:
		return CoreDumpNone
	case info.mask&pidfdInfoExit != 0 && info.mask&pidfdInfoSupportedMask != 0 && info.supportedMask&pidfdInfoCoredump != 0:
		return CoreDumpNone
	}
	return CoreDumpUnknown
}
