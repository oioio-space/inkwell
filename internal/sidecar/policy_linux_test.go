package sidecar

import (
	"encoding/binary"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

// interpretSeccomp executes the emitted classic BPF on native-endian seccomp_data.
// Keeping this small interpreter strict makes unsupported instructions fail closed.
func interpretSeccomp(t *testing.T, program []unix.SockFilter, data []byte) uint32 {
	t.Helper()
	var a uint32
	for pc := 0; pc < len(program); pc++ {
		i := program[pc]
		switch i.Code {
		case unix.BPF_LD | unix.BPF_W | unix.BPF_ABS:
			a = binary.NativeEndian.Uint32(data[i.K : i.K+4])
		case unix.BPF_ALU | unix.BPF_AND | unix.BPF_K:
			a &= i.K
		case unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K:
			if a == i.K {
				pc += int(i.Jt)
			} else {
				pc += int(i.Jf)
			}
		case unix.BPF_JMP | unix.BPF_JSET | unix.BPF_K:
			if a&i.K != 0 {
				pc += int(i.Jt)
			} else {
				pc += int(i.Jf)
			}
		case unix.BPF_RET | unix.BPF_K:
			return i.K
		default:
			t.Fatalf("unsupported BPF instruction: %+v", i)
		}
	}
	t.Fatal("BPF fell through")
	return 0
}

func TestSeccompPolicyVM(t *testing.T) {
	const arch = unix.AUDIT_ARCH_X86_64
	const pid = 1234
	f := seccompFilter(arch, pid)
	check := func(nr uint32, audit uint32, want uint32, args ...uint64) {
		t.Helper()
		data := make([]byte, 64)
		binary.NativeEndian.PutUint32(data, nr)
		binary.NativeEndian.PutUint32(data[4:], audit)
		for i, a := range args {
			binary.NativeEndian.PutUint64(data[16+8*i:], a)
		}
		if got := interpretSeccomp(t, f, data); got != want {
			t.Fatalf("nr=%d arch=%x args=%x: got %x want %x", nr, audit, args, got, want)
		}
	}
	allow := uint32(unix.SECCOMP_RET_ALLOW)
	deny := uint32(unix.SECCOMP_RET_ERRNO | unix.EPERM)
	// Hand-written, independent of the generated allowedSyscalls table: what a
	// Go or Rust sidecar cannot run without, and what it must never get.
	for _, nr := range []uint32{
		unix.SYS_READ, unix.SYS_WRITE, unix.SYS_CLOSE, unix.SYS_OPENAT, unix.SYS_MMAP, unix.SYS_MUNMAP,
		unix.SYS_MPROTECT, unix.SYS_BRK, unix.SYS_FUTEX, unix.SYS_EXIT, unix.SYS_EXIT_GROUP, unix.SYS_EXECVE,
		unix.SYS_RT_SIGACTION, unix.SYS_RT_SIGPROCMASK, unix.SYS_RT_SIGRETURN, unix.SYS_SIGALTSTACK,
		unix.SYS_GETPID, unix.SYS_GETTID, unix.SYS_SCHED_YIELD, unix.SYS_NANOSLEEP, unix.SYS_GETRANDOM,
		unix.SYS_EPOLL_PWAIT, unix.SYS_PIPE2, unix.SYS_WAIT4,
	} {
		check(nr, arch, allow)
		check(nr, 0, unix.SECCOMP_RET_KILL_PROCESS)
		check(nr|0x40000000, arch, unix.SECCOMP_RET_KILL_PROCESS)
	}
	for _, nr := range []uint32{
		unix.SYS_SOCKET, unix.SYS_CONNECT, unix.SYS_BIND, unix.SYS_LISTEN, unix.SYS_ACCEPT4, unix.SYS_SENDTO,
		unix.SYS_SENDMSG, unix.SYS_PTRACE, unix.SYS_PROCESS_VM_READV, unix.SYS_PROCESS_VM_WRITEV,
		unix.SYS_MOUNT, unix.SYS_UMOUNT2, unix.SYS_UNSHARE, unix.SYS_SETNS, unix.SYS_PIVOT_ROOT, unix.SYS_CHROOT,
		unix.SYS_BPF, unix.SYS_KEXEC_LOAD, unix.SYS_INIT_MODULE, unix.SYS_FINIT_MODULE, unix.SYS_DELETE_MODULE,
		unix.SYS_REBOOT, unix.SYS_SWAPON, unix.SYS_KEYCTL, unix.SYS_ADD_KEY, unix.SYS_PERF_EVENT_OPEN,
		unix.SYS_USERFAULTFD, unix.SYS_OPEN_BY_HANDLE_AT, unix.SYS_KILL, unix.SYS_TKILL, unix.SYS_SHMAT,
		unix.SYS_IO_URING_SETUP, unix.SYS_IO_URING_ENTER, unix.SYS_FSOPEN, unix.SYS_MOVE_MOUNT, unix.SYS_PIDFD_GETFD,
		unix.SYS_PERSONALITY, unix.SYS_SETUID, unix.SYS_CAPSET, 10000,
	} {
		check(nr, arch, deny)
	}
	for domain := range uint64(64) {
		check(unix.SYS_SOCKET, arch, deny, domain, unix.SOCK_STREAM)
	}
	// Exhaustive sweep with zero arguments: a jump-offset slip in the emitted
	// program cannot open anything beyond the table, and every refusal is a
	// plain EPERM (ENOSYS for clone3), never a kill or another verdict.
	conditional := map[uint32]uint32{unix.SYS_SOCKETPAIR: deny, unix.SYS_TGKILL: deny, unix.SYS_CLONE3: unix.SECCOMP_RET_ERRNO | uint32(unix.ENOSYS)}
	for nr := range uint32(1024) {
		want, ok := conditional[nr]
		if !ok {
			want = deny
			if slices.Contains(allowedSyscalls, nr) {
				want = allow
			}
		}
		check(nr, arch, want)
	}
	check(unix.SYS_CLONE3, arch, unix.SECCOMP_RET_ERRNO|uint32(unix.ENOSYS))
	for _, flags := range []uint64{0, unix.CLONE_VM | unix.CLONE_THREAD | unix.CLONE_SIGHAND} {
		check(unix.SYS_CLONE, arch, allow, flags)
		for _, bad := range []uint64{unix.CLONE_NEWUSER, unix.CLONE_NEWNS, unix.CLONE_NEWPID, unix.CLONE_NEWNET, unix.CLONE_NEWIPC, unix.CLONE_NEWUTS, unix.CLONE_NEWCGROUP, unix.CLONE_PARENT} {
			check(unix.SYS_CLONE, arch, deny, flags|bad)
		}
	}
	for _, high := range []uint64{0, 1 << 32, 0xffffffff00000000} {
		for _, cmd := range []uint64{unix.F_SETOWN, unix.F_SETOWN_EX, unix.F_SETSIG} {
			check(unix.SYS_FCNTL, arch, deny, 0, high|cmd)
		}
		for _, cmd := range []uint64{unix.TIOCSTI, unix.TIOCLINUX, 0x8901, 0x8902} {
			check(unix.SYS_IOCTL, arch, deny, 0, high|cmd)
		}
		check(unix.SYS_IOCTL, arch, allow, 0, high|unix.TIOCGWINSZ)
	}
	check(unix.SYS_FCNTL, arch, allow, 0, unix.F_GETFL)
	check(unix.SYS_FCNTL, arch, allow, 0, unix.F_SETFL, unix.O_NONBLOCK)
	check(unix.SYS_FCNTL, arch, deny, 0, unix.F_SETFL, unix.O_ASYNC)
	for _, domain := range []uint64{unix.AF_UNIX, unix.AF_INET, unix.AF_INET6, unix.AF_NETLINK, unix.AF_PACKET} {
		for _, kind := range []uint64{unix.SOCK_STREAM, unix.SOCK_STREAM | unix.SOCK_CLOEXEC | unix.SOCK_NONBLOCK, unix.SOCK_DGRAM} {
			want := deny
			if domain == unix.AF_UNIX && kind&15 == unix.SOCK_STREAM {
				want = allow
			}
			check(unix.SYS_SOCKETPAIR, arch, want, domain, kind)
		}
	}
	check(unix.SYS_TGKILL, arch, allow, pid)
	check(unix.SYS_TGKILL, arch, deny, pid+1)
}

func TestLandlockExactRights(t *testing.T) {
	// ABI transitions must neither omit a supported restriction nor submit a
	// future field or bit to an older kernel.
	for abi := uintptr(3); abi <= 12; abi++ {
		attr, size := landlockPolicy(abi)
		fs := uint64(unix.LANDLOCK_ACCESS_FS_EXECUTE | unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE | unix.LANDLOCK_ACCESS_FS_MAKE_CHAR | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_MAKE_REG | unix.LANDLOCK_ACCESS_FS_MAKE_SOCK | unix.LANDLOCK_ACCESS_FS_MAKE_FIFO | unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK | unix.LANDLOCK_ACCESS_FS_MAKE_SYM | unix.LANDLOCK_ACCESS_FS_REFER | unix.LANDLOCK_ACCESS_FS_TRUNCATE)
		net, scope := uint64(0), uint64(0)
		wantSize := uintptr(8)
		if abi >= 4 {
			net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
			wantSize = 16
		}
		if abi >= 5 {
			fs |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
		}
		if abi >= 6 {
			scope = unix.LANDLOCK_SCOPE_SIGNAL | unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET
			wantSize = 24
		}
		if abi >= 9 {
			fs |= unix.LANDLOCK_ACCESS_FS_RESOLVE_UNIX
		}
		if abi >= 10 {
			net |= 1<<2 | 1<<3
		}
		if attr.Handled_access_fs != fs || attr.Handled_access_net != net || attr.Scoped != scope || size != wantSize {
			t.Fatalf("ABI %d: %+v size=%d", abi, attr, size)
		}
	}
}
