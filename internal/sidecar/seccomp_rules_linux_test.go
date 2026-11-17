package sidecar

import (
	"encoding/binary"
	"runtime"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

const (
	retAllow  = uint32(unix.SECCOMP_RET_ALLOW)
	retEPERM  = unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)
	retENOSYS = unix.SECCOMP_RET_ERRNO | uint32(unix.ENOSYS)
	testPID   = 1234
)

// verdict interprets f on the seccomp_data of system call nr of architecture a.
func verdict(t *testing.T, f []unix.SockFilter, a sysArch, nr uint32, args ...uint64) uint32 {
	t.Helper()
	data := make([]byte, 64)
	binary.NativeEndian.PutUint32(data, nr)
	binary.NativeEndian.PutUint32(data[4:], a.audit)
	for i, arg := range args {
		binary.NativeEndian.PutUint64(data[16+8*i:], arg)
	}
	return interpretSeccomp(t, f, data)
}

// eachFilter runs check on the filter of every registered program, on both
// architectures' tables and in both tiers.
func eachFilter(t *testing.T, check func(p program, a sysArch, tr tier, f []unix.SockFilter)) {
	t.Helper()
	for _, p := range programs {
		for _, a := range sysArches {
			for _, tr := range []tier{tierN, tierF} {
				f, err := seccompFilter(p, a, testPID, tr)
				if err != nil {
					t.Fatalf("%s %s %v: %v", p.name, a.name, tr, err)
				}
				check(p, a, tr, f)
			}
		}
	}
}

// S-1: prlimit64 only with pid 0, the 64-bit argument compared in both halves,
// both tiers and both architectures; the RLIMIT_CORE sentinel still holds.
func TestSeccompPrlimitPidZero(t *testing.T) {
	const self = testPID
	pids := []uint64{0, self, self - 1, 1, ^uint64(0), 1 << 31, 1<<32 + self, 1 << 32}
	resources := []uint64{unix.RLIMIT_AS, unix.RLIMIT_CORE, unix.RLIMIT_NOFILE, unix.RLIMIT_FSIZE, unix.RLIMIT_CPU}
	pointers := []uint64{0, 0x7fff0000, 1 << 32}
	eachFilter(t, func(p program, a sysArch, tr tier, f []unix.SockFilter) {
		for _, pid := range pids {
			for _, resource := range resources {
				for _, newLimit := range pointers {
					want := retEPERM
					if pid == 0 && (resource != unix.RLIMIT_CORE || newLimit == 0) {
						want = retAllow
					}
					if got := verdict(t, f, a, a.prlimit64, pid, resource, newLimit, 0x7fff1000); got != want {
						t.Errorf("S-1 prlimit64: %s %s %v pid %#x resource %d new %#x: verdict %#x, want %#x", p.name, a.name, tr, pid, resource, newLimit, got, want)
					}
				}
			}
		}
	})
}

// S-2: in tier F no process is created: clone only with CLONE_THREAD (and none
// of the namespace or CLONE_PARENT flags), clone3 answered ENOSYS, fork and
// vfork denied. Tier N keeps its rule: no namespace flag, no CLONE_PARENT.
func TestSeccompNoProcessInF(t *testing.T) {
	bits := []uint64{unix.CLONE_THREAD, unix.CLONE_VM, unix.CLONE_VFORK, unix.CLONE_PARENT, unix.CLONE_NEWUSER}
	eachFilter(t, func(p program, a sysArch, tr tier, f []unix.SockFilter) {
		for mask := range 1 << len(bits) {
			var flags uint64
			for i, b := range bits {
				if mask&(1<<i) != 0 {
					flags |= b
				}
			}
			bad := flags&(unix.CLONE_PARENT|unix.CLONE_NEWUSER) != 0
			for _, base := range []uint64{0, unix.CLONE_SIGHAND | uint64(unix.SIGCHLD), 0xffffffff00000000} {
				want := retAllow
				if bad || (tr == tierF && flags&unix.CLONE_THREAD == 0) {
					want = retEPERM
				}
				if got := verdict(t, f, a, a.clone, base|flags); got != want {
					t.Errorf("S-2 no process: %s %s %v clone %#x: verdict %#x, want %#x", p.name, a.name, tr, base|flags, got, want)
				}
			}
		}
		for _, ns := range []uint64{unix.CLONE_NEWNS, unix.CLONE_NEWPID, unix.CLONE_NEWNET, unix.CLONE_NEWIPC, unix.CLONE_NEWUTS, unix.CLONE_NEWCGROUP} {
			if got := verdict(t, f, a, a.clone, unix.CLONE_THREAD|unix.CLONE_VM|unix.CLONE_SIGHAND|ns); got != retEPERM {
				t.Errorf("S-2 no process: %s %s %v clone with namespace %#x: verdict %#x", p.name, a.name, tr, ns, got)
			}
		}
		if got := verdict(t, f, a, a.clone3, 0x7fff0000, 88); got != retENOSYS {
			t.Errorf("S-2 no process: %s %s %v clone3: verdict %#x, want ENOSYS", p.name, a.name, tr, got)
		}
		for _, nr := range []uint32{a.fork, a.vfork} {
			if nr == noSyscall {
				continue
			}
			if got := verdict(t, f, a, nr); got != retEPERM {
				t.Errorf("S-2 no process: %s %s %v fork-like %d: verdict %#x, want EPERM", p.name, a.name, tr, nr, got)
			}
		}
	})
}

// S-4: prctl only with the options the descriptor lists (tinymist:
// PR_SET_NAME; typst: none), so PR_SET_PDEATHSIG can never disarm the
// parent-death signal; the option is the kernel's int, so the high word is
// ignored as the kernel ignores it.
func TestSeccompPrctlOptions(t *testing.T) {
	options := []uint64{unix.PR_SET_PDEATHSIG, unix.PR_GET_PDEATHSIG, unix.PR_SET_NO_NEW_PRIVS, unix.PR_SET_SECCOMP, unix.PR_SET_DUMPABLE, unix.PR_SET_PTRACER, unix.PR_CAP_AMBIENT, unix.PR_SET_MM, unix.PR_SET_CHILD_SUBREAPER, unix.PR_GET_NAME, unix.PR_SET_NAME}
	for o := range uint64(100) {
		options = append(options, o)
	}
	eachFilter(t, func(p program, a sysArch, tr tier, f []unix.SockFilter) {
		for _, o := range options {
			for _, high := range []uint64{0, 1 << 32, 0xffffffff00000000} {
				want := retEPERM
				if slices.Contains(p.prctl, uint32(o)) && o != unix.PR_SET_PDEATHSIG { // #nosec G115 -- test options are below 2^32.
					want = retAllow
				}
				if got := verdict(t, f, a, a.prctl, high|o, uint64(unix.SIGKILL)); got != want {
					t.Errorf("S-4 prctl: %s %s %v option %#x: verdict %#x, want %#x", p.name, a.name, tr, high|o, got, want)
				}
			}
		}
		if got := verdict(t, f, a, a.prctl, unix.PR_SET_PDEATHSIG, 0); got != retEPERM {
			t.Errorf("S-4 prctl: %s %s %v PR_SET_PDEATHSIG 0 allowed", p.name, a.name, tr)
		}
	})
	if p, _ := lookupProgram(Tinymist); !slices.Equal(p.prctl, []uint32{unix.PR_SET_NAME}) {
		t.Errorf("S-4 prctl: tinymist options %v, want [PR_SET_NAME]", p.prctl)
	}
}

// S-4: ioctl only with the requests the descriptor lists (typst: TIOCGWINSZ;
// tinymist: FIONREAD, TCGETS), every other request refused, both tiers and
// architectures; the request is the kernel's unsigned int.
func TestSeccompIoctlRequests(t *testing.T) {
	requests := []uint64{unix.TIOCSTI, unix.TIOCLINUX, 0x8901, 0x8902, unix.TIOCSCTTY, unix.TIOCSWINSZ, 0x5421, 0x5451, unix.TCSETS, unix.TIOCSPGRP, 0xc0185879}
	for r := uint64(0x5400); r < 0x5500; r++ {
		requests = append(requests, r)
	}
	eachFilter(t, func(p program, a sysArch, tr tier, f []unix.SockFilter) {
		for _, r := range requests {
			for _, high := range []uint64{0, 1 << 32} {
				want := retEPERM
				if slices.Contains(p.ioctl, uint32(r)) { // #nosec G115 -- test requests are below 2^32.
					want = retAllow
				}
				if got := verdict(t, f, a, a.ioctl, 1, high|r); got != want {
					t.Errorf("S-4 ioctl: %s %s %v request %#x: verdict %#x, want %#x", p.name, a.name, tr, high|r, got, want)
				}
			}
		}
	})
	for name, want := range map[string][]uint32{Typst: {unix.TIOCGWINSZ}, Tinymist: {unix.TIOCINQ, unix.TCGETS}} {
		if p, _ := lookupProgram(name); !slices.Equal(p.ioctl, want) {
			t.Errorf("S-4 ioctl: %s requests %#x, want %#x", name, p.ioctl, want)
		}
	}
}

// The host's table holds the host's x/sys numbers, so each runner checks the
// table of its own architecture (amd64 on ci, arm64 on sidecars-arm64).
func TestSysArchMatchesHost(t *testing.T) {
	a, err := hostSysArch()
	if err != nil {
		t.Fatal(err)
	}
	fork, vfork := sysForkNumbers()
	got := []uint32{a.prlimit64, a.setrlimit, a.fcntl, a.ioctl, a.prctl, a.clone, a.clone3, fork, vfork, a.socketpair, a.tgkill}
	want := []uint32{unix.SYS_PRLIMIT64, setrlimitSyscall, unix.SYS_FCNTL, unix.SYS_IOCTL, unix.SYS_PRCTL, unix.SYS_CLONE, unix.SYS_CLONE3, a.fork, a.vfork, unix.SYS_SOCKETPAIR, unix.SYS_TGKILL}
	if !slices.Equal(got, want) {
		t.Errorf("%s: named numbers %v, want %v", runtime.GOARCH, got, want)
	}
	denied := []uint32{
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
	}
	if !slices.Equal(a.denied, denied) {
		t.Errorf("%s: denied %v, want %v", runtime.GOARCH, a.denied, denied)
	}
}
