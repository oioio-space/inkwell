package sidecar

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// noSyscall marks a system call an architecture does not have (arm64 has no
// fork or vfork): a rule on it never matches, and the default refusal applies.
const noSyscall = ^uint32(0)

// sysArch holds, for one architecture, the audit value seccomp checks and the
// number of every system call the filter names. The filter is built from this
// table, never from the host's unix.SYS_* constants, so that the rules of both
// architectures are interpreted on either (plan S-1, S-2, S-4). Numbers are
// those of golang.org/x/sys/unix zsysnum_linux_{amd64,arm64}.go;
// TestSysArchMatchesHost checks the host's table against them on each runner.
type sysArch struct {
	name  string // GOARCH
	audit uint32

	prlimit64, setrlimit, fcntl, ioctl, prctl uint32
	clone, clone3, fork, vfork, socketpair    uint32
	tgkill                                    uint32
	// poll is the launcher's wait: poll on amd64, ppoll on arm64.
	poll uint32
	// denied are refused before any other rule, whatever their arguments.
	denied []uint32
}

// The order of denied is the one of TestSysArchMatchesHost's expected list:
// socket, connect, bind, sendto, sendmsg, sendmmsg, ptrace, process_vm_readv,
// process_vm_writev, kexec_load, kexec_file_load, bpf, keyctl, add_key,
// request_key, mount, umount2, unshare, setns, fsopen, fsconfig, fsmount,
// fspick, open_tree, move_mount, mount_setattr, pidfd_getfd, personality, kill,
// tkill, rt_sigqueueinfo, rt_tgsigqueueinfo, pidfd_send_signal, io_uring_setup,
// userfaultfd, perf_event_open, shmget, shmat, shmctl, shmdt, msgget, msgsnd,
// msgrcv, msgctl, semget, semop, semtimedop, semctl, mq_open, mq_unlink,
// mq_timedsend, mq_timedreceive, mq_notify, mq_getsetattr.
var (
	archAMD64 = sysArch{
		name: "amd64", audit: unix.AUDIT_ARCH_X86_64,
		prlimit64: 302, setrlimit: 160, fcntl: 72, ioctl: 16, prctl: 157,
		clone: 56, clone3: 435, fork: 57, vfork: 58, socketpair: 53, tgkill: 234, poll: 7,
		denied: []uint32{
			41, 42, 49, 44, 46, 307, 101, 310, 311, 246, 320, 321, 250, 248, 249,
			165, 166, 272, 308, 430, 431, 432, 433, 428, 429, 442, 438, 135, 62, 200,
			129, 297, 424, 425, 323, 298, 29, 30, 31, 67, 68, 69, 70, 71, 64, 65, 220, 66,
			240, 241, 242, 243, 244, 245,
		},
	}
	// archARM64's setrlimit stays noSyscall until block SB2a2 (plan A-2) sets
	// 164 with its raw-call test; corelimit_linux_arm64.go holds the same value.
	archARM64 = sysArch{
		name: "arm64", audit: unix.AUDIT_ARCH_AARCH64,
		prlimit64: 261, setrlimit: noSyscall, fcntl: 25, ioctl: 29, prctl: 167,
		clone: 220, clone3: 435, fork: noSyscall, vfork: noSyscall, socketpair: 199, tgkill: 131, poll: 73,
		denied: []uint32{
			198, 203, 200, 206, 211, 269, 117, 270, 271, 104, 294, 280, 219, 217, 218,
			40, 39, 97, 268, 430, 431, 432, 433, 428, 429, 442, 438, 92, 129, 130,
			138, 240, 424, 425, 282, 241, 194, 196, 195, 197, 186, 189, 188, 187, 190, 193, 192, 191,
			180, 181, 182, 183, 184, 185,
		},
	}
	sysArches = []sysArch{archAMD64, archARM64}
)

// hostSysArch is the table of the architecture this process runs on.
func hostSysArch() (sysArch, error) {
	for _, a := range sysArches {
		if a.name == runtime.GOARCH {
			return a, nil
		}
	}
	return sysArch{}, fmt.Errorf("sidecar: no seccomp table for %s", runtime.GOARCH)
}
