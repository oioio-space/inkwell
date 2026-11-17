package sidecar

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// pidfdInfo is struct pidfd_info of include/uapi/linux/pidfd.h at its fourth
// published size (PIDFD_INFO_SIZE_VER3, 88 bytes). The kernel copies the
// fields that fit the size encoded in the ioctl command, zero-fills what its
// own struct does not reach, and clears the mask bits of the fields it does
// not have (fs/pidfs.c, pidfd_info). tail holds the two words after
// coredump_signal, whose meaning depends on the kernel's own struct size:
// VER2 (80 bytes, Linux 7.0) puts supported_mask in the first, VER3 puts
// coredump_code and padding there and supported_mask in the second.
type pidfdInfo struct {
	mask                                             uint64
	cgroupID                                         uint64
	pid, tgid, ppid                                  uint32
	ruid, rgid, euid, egid, suid, sgid, fsuid, fsgid uint32
	exitCode                                         int32
	coredumpMask, coredumpSignal                     uint32
	tail                                             [2]uint64
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
	return coreDumpFrom(info)
}

// coreDumpFrom reads the kernel's answer. supported_mask always lists its own
// flag: a zero second word is a kernel whose struct ends before it (VER2),
// which wrote the mask in the first.
func coreDumpFrom(info pidfdInfo) CoreDump {
	supported := info.tail[1]
	if supported == 0 {
		supported = info.tail[0]
	}
	switch {
	case info.mask&pidfdInfoCoredump != 0 && info.coredumpMask&pidfdCoredumped != 0:
		return CoreDumpStarted
	case info.mask&pidfdInfoCoredump != 0:
		return CoreDumpNone
	case info.mask&pidfdInfoExit != 0 && info.mask&pidfdInfoSupportedMask != 0 && supported&pidfdInfoCoredump != 0:
		return CoreDumpNone
	}
	return CoreDumpUnknown
}
