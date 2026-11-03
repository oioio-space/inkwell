package sidecar

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// seccompFilter uses seccomp_data offsets from linux/seccomp.h. Deny every
// socket family (including UNIX, INET, INET6, NETLINK and PACKET).
// Tinymist Tokio signal handling needs an unnamed UNIX STREAM socketpair.
// connect/bind/sendto/sendmsg remain denied, including on inherited sockets.
func seccompFilter(arch uint32, pid uint32) []unix.SockFilter {
	stmt := func(code uint16, k uint32) unix.SockFilter { return unix.SockFilter{Code: code, K: k} }
	eq := func(k uint32, yes, no uint8) unix.SockFilter {
		return unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: k, Jt: yes, Jf: no}
	}
	deny := stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ERRNO|uint32(unix.EPERM))
	f := []unix.SockFilter{
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 4),
		eq(arch, 1, 0),
		stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_KILL_PROCESS),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0),
		{Code: unix.BPF_JMP | unix.BPF_JSET | unix.BPF_K, K: 0x40000000, Jf: 1},
		stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_KILL_PROCESS),
	}
	for _, nr := range []uint32{
		unix.SYS_SOCKET, unix.SYS_CONNECT, unix.SYS_BIND, unix.SYS_SENDTO, unix.SYS_SENDMSG, unix.SYS_SENDMMSG, unix.SYS_PTRACE,
		unix.SYS_PROCESS_VM_READV, unix.SYS_PROCESS_VM_WRITEV,
		unix.SYS_KEXEC_LOAD, unix.SYS_KEXEC_FILE_LOAD, unix.SYS_BPF,
		unix.SYS_KEYCTL, unix.SYS_ADD_KEY, unix.SYS_REQUEST_KEY,
		unix.SYS_MOUNT, unix.SYS_UMOUNT2, unix.SYS_UNSHARE, unix.SYS_SETNS,
		unix.SYS_FSOPEN, unix.SYS_FSCONFIG, unix.SYS_FSMOUNT, unix.SYS_FSPICK,
		unix.SYS_OPEN_TREE, unix.SYS_MOVE_MOUNT, unix.SYS_MOUNT_SETATTR, unix.SYS_PIDFD_GETFD,
		unix.SYS_PERSONALITY, unix.SYS_KILL, unix.SYS_TKILL,
		unix.SYS_RT_SIGQUEUEINFO, unix.SYS_RT_TGSIGQUEUEINFO, unix.SYS_PIDFD_SEND_SIGNAL,
		unix.SYS_IO_URING_SETUP, unix.SYS_USERFAULTFD, unix.SYS_PERF_EVENT_OPEN,
		unix.SYS_SHMGET, unix.SYS_SHMAT, unix.SYS_SHMCTL, unix.SYS_SHMDT,
		unix.SYS_MSGGET, unix.SYS_MSGSND, unix.SYS_MSGRCV, unix.SYS_MSGCTL,
		unix.SYS_SEMGET, unix.SYS_SEMOP, unix.SYS_SEMTIMEDOP, unix.SYS_SEMCTL,
		unix.SYS_MQ_OPEN, unix.SYS_MQ_UNLINK, unix.SYS_MQ_TIMEDSEND, unix.SYS_MQ_TIMEDRECEIVE, unix.SYS_MQ_NOTIFY, unix.SYS_MQ_GETSETATTR,
	} {
		f = append(f, eq(nr, 0, 1), deny)
	}
	// Signal ownership controls must never target processes outside this job.
	for _, control := range []struct {
		nr       uint32
		commands []uint32
	}{
		{unix.SYS_FCNTL, []uint32{unix.F_SETOWN, unix.F_SETOWN_EX, unix.F_SETSIG}},
		{unix.SYS_IOCTL, []uint32{0x8901, 0x8902, unix.TIOCSTI, unix.TIOCLINUX}}, // FIOSETOWN, SIOCSPGRP
	} {
		f = append(f, eq(control.nr, 0, uint8(2*len(control.commands)+2)), stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 24)) // #nosec G115 -- two fixed command arrays, maximum jump is ten.
		for _, command := range control.commands {
			f = append(f, eq(command, 0, 1), deny)
		}
		f = append(f, stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0))
	}
	// F_SETFL may enable SIGIO even on an inherited descriptor with an owner.
	f = append(f, eq(unix.SYS_FCNTL, 0, 7),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 24), eq(unix.F_SETFL, 0, 4),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 32),
		unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JSET | unix.BPF_K, K: unix.O_ASYNC, Jf: 1},
		deny, stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0))
	// clone3 carries flags behind a pointer. ENOSYS makes libc fall back to
	// clone, whose namespace flags can be inspected without reading memory.
	f = append(f, eq(unix.SYS_CLONE3, 0, 1),
		stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ERRNO|uint32(unix.ENOSYS)))
	const namespaces = unix.CLONE_NEWUSER | unix.CLONE_NEWNS | unix.CLONE_NEWPID |
		unix.CLONE_NEWNET | unix.CLONE_NEWIPC | unix.CLONE_NEWUTS | unix.CLONE_NEWCGROUP
	f = append(f, eq(unix.SYS_CLONE, 0, 4),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 16),
		unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JSET | unix.BPF_K, K: namespaces | unix.CLONE_PARENT, Jf: 1},
		deny, stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0))
	// Permit only unnamed UNIX stream pairs for Tokio's signal self-pipe.
	f = append(f, eq(unix.SYS_SOCKETPAIR, 0, 7),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 16),
		eq(unix.AF_UNIX, 1, 0), deny,
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 24),
		stmt(unix.BPF_ALU|unix.BPF_AND|unix.BPF_K, 0xf),
		eq(unix.SOCK_STREAM, 1, 0), deny,
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0))
	// Go and Rust runtimes need self-directed tgkill (e.g. async preemption).
	// Outside PID namespaces this also prevents signalling a sibling process.
	f = append(f, eq(unix.SYS_TGKILL, 0, 4),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 16),
		eq(pid, 1, 0), deny,
		stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ALLOW),
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0))
	// Unknown native syscalls fail with EPERM. Only ABI confusion kills:
	// emulating an unexpected syscall return is safer for legitimate workloads.
	for _, nr := range allowedSyscalls {
		f = append(f, eq(nr, 0, 1), stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ALLOW))
	}
	f = append(f, deny)
	return f
}

func installSeccomp() error {
	var arch uint32
	switch runtime.GOARCH {
	case "amd64":
		arch = unix.AUDIT_ARCH_X86_64
	case "arm64":
		arch = unix.AUDIT_ARCH_AARCH64
	default:
		return errors.New("unsupported seccomp audit architecture")
	}
	f := seccompFilter(arch, uint32(unix.Getpid()))                                                        // #nosec G115 -- positive Linux PID fits uint32.
	prog := unix.SockFprog{Len: uint16(len(f)), Filter: &f[0]}                                             // #nosec G115 -- fixed small filter.
	return unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(&prog)), 0, 0) // #nosec G103 -- synchronous kernel copy of BPF program.
}
