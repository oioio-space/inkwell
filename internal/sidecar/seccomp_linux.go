package sidecar

import (
	"fmt"
	"slices"
	"unsafe"

	"golang.org/x/sys/unix"
)

// seccomp_data offsets (linux/seccomp.h): nr, arch, then six 64-bit
// arguments. Both supported architectures are little-endian, so an
// argument's low word comes first.
const (
	offNR   = 0
	offArch = 4
)

func argLow(i uint32) uint32  { return 16 + 8*i }
func argHigh(i uint32) uint32 { return 20 + 8*i }

func bpfStmt(code uint16, k uint32) unix.SockFilter { return unix.SockFilter{Code: code, K: k} }
func bpfLoad(off uint32) unix.SockFilter            { return bpfStmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, off) }
func bpfRet(k uint32) unix.SockFilter               { return bpfStmt(unix.BPF_RET|unix.BPF_K, k) }
func bpfEq(k uint32, yes, no uint8) unix.SockFilter {
	return unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: k, Jt: yes, Jf: no}
}

func bpfSet(k uint32, yes, no uint8) unix.SockFilter {
	return unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JSET | unix.BPF_K, K: k, Jt: yes, Jf: no}
}

// maxListed bounds a descriptor's prctl or ioctl list: each value is one
// forward jump, and a classic BPF jump offset is a byte.
const maxListed = 32

// seccompFilter builds the filter of program p on architecture a, in tier t;
// pid is the target's own pid. Every argument rule only refuses or falls
// through: what is allowed is p's allow-list alone (ADR 0044 point 1f).
//
// Always: every socket family is denied (tinymist's Tokio signal handling
// needs only an unnamed UNIX STREAM socketpair); prlimit64 only with pid 0
// (S-1); prctl and ioctl only with p's options and requests (S-4); clone3
// answers ENOSYS so libc falls back to clone, whose flags are inspected
// without reading memory. Tier F adds: clone only with CLONE_THREAD, fork and
// vfork denied, so no process is created (S-2).
func seccompFilter(p program, a sysArch, pid uint32, t tier) ([]unix.SockFilter, error) {
	allowed, ok := p.syscalls[a.name]
	if !ok {
		return nil, fmt.Errorf("sidecar: %s has no seccomp allow-list for %s", p.name, a.name)
	}
	if slices.Contains(p.prctl, unix.PR_SET_PDEATHSIG) {
		return nil, fmt.Errorf("sidecar: %s: PR_SET_PDEATHSIG would let the target disarm its parent-death signal", p.name)
	}
	if len(p.prctl) > maxListed || len(p.ioctl) > maxListed {
		return nil, fmt.Errorf("sidecar: %s: more than %d prctl options or ioctl requests", p.name, maxListed)
	}
	if t != tierN && t != tierF {
		return nil, fmt.Errorf("sidecar: no seccomp filter for tier %v", t)
	}
	deny := bpfRet(unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM))
	reload := bpfLoad(offNR)
	f := []unix.SockFilter{
		bpfLoad(offArch),
		bpfEq(a.audit, 1, 0),
		bpfRet(unix.SECCOMP_RET_KILL_PROCESS),
		reload,
		bpfSet(0x40000000, 0, 1), // x32 numbers on amd64
		bpfRet(unix.SECCOMP_RET_KILL_PROCESS),
	}
	for _, nr := range a.denied {
		f = append(f, bpfEq(nr, 0, 1), deny)
	}
	// RLIMIT_CORE's soft limit must stay at the pipe-core sentinel after exec.
	// The resource is the kernel's unsigned int; pointers are 64-bit, so both
	// words are checked: a high-only pointer is not a query.
	f = append(f, bpfEq(a.setrlimit, 0, 3), bpfLoad(argLow(0)), bpfEq(unix.RLIMIT_CORE, 0, 1), deny, reload)
	// S-1: prlimit64 never targets another process. pid_t is truncated by the
	// kernel; requiring both words to be 0 refuses 2^32+pid too.
	f = append(f, bpfEq(a.prlimit64, 0, 14),
		bpfLoad(argLow(0)), bpfEq(0, 1, 0), deny,
		bpfLoad(argHigh(0)), bpfEq(0, 1, 0), deny,
		bpfLoad(argLow(1)), bpfEq(unix.RLIMIT_CORE, 0, 5),
		bpfLoad(argLow(2)), bpfEq(0, 0, 2), bpfLoad(argHigh(2)), bpfEq(0, 1, 0), deny,
		reload)
	// Signal ownership must never target a process outside this job.
	owner := []uint32{unix.F_SETOWN, unix.F_SETOWN_EX, unix.F_SETSIG}
	f = append(f, bpfEq(a.fcntl, 0, uint8(2*len(owner)+2)), bpfLoad(argLow(1))) // #nosec G115 -- three commands.
	for _, command := range owner {
		f = append(f, bpfEq(command, 0, 1), deny)
	}
	f = append(f, reload)
	// F_SETFL may enable SIGIO even on an inherited descriptor with an owner.
	f = append(f, bpfEq(a.fcntl, 0, 6),
		bpfLoad(argLow(1)), bpfEq(unix.F_SETFL, 0, 3),
		bpfLoad(argLow(2)), bpfSet(unix.O_ASYNC, 0, 1),
		deny, reload)
	// S-4: the request and the option are the kernel's unsigned int and int.
	f = onlyValues(f, a.ioctl, argLow(1), p.ioctl, deny)
	f = onlyValues(f, a.prctl, argLow(0), p.prctl, deny)
	f = append(f, bpfEq(a.clone3, 0, 1), bpfRet(unix.SECCOMP_RET_ERRNO|uint32(unix.ENOSYS)))
	// clone's flags are taken from their low word by the kernel.
	const forbidden = unix.CLONE_NEWUSER | unix.CLONE_NEWNS | unix.CLONE_NEWPID |
		unix.CLONE_NEWNET | unix.CLONE_NEWIPC | unix.CLONE_NEWUTS | unix.CLONE_NEWCGROUP | unix.CLONE_PARENT
	if t == tierF {
		f = append(f, bpfEq(a.clone, 0, 6),
			bpfLoad(argLow(0)), bpfSet(forbidden, 0, 1), deny,
			bpfSet(unix.CLONE_THREAD, 1, 0), deny,
			reload)
		for _, nr := range []uint32{a.fork, a.vfork} {
			if nr != noSyscall {
				f = append(f, bpfEq(nr, 0, 1), deny)
			}
		}
	} else {
		f = append(f, bpfEq(a.clone, 0, 4),
			bpfLoad(argLow(0)), bpfSet(forbidden, 0, 1), deny,
			reload)
	}
	// Only unnamed UNIX stream pairs, for Tokio's signal self-pipe.
	f = append(f, bpfEq(a.socketpair, 0, 7),
		bpfLoad(argLow(0)), bpfEq(unix.AF_UNIX, 1, 0), deny,
		bpfLoad(argLow(1)), bpfStmt(unix.BPF_ALU|unix.BPF_AND|unix.BPF_K, 0xf),
		bpfEq(unix.SOCK_STREAM, 1, 0), deny,
		reload)
	// Go and Rust runtimes need self-directed tgkill (async preemption, abort).
	f = append(f, bpfEq(a.tgkill, 0, 4), bpfLoad(argLow(0)), bpfEq(pid, 1, 0), deny, reload)
	// Unknown native calls fail with EPERM: only ABI confusion kills, since an
	// unexpected error return is safer for a legitimate workload.
	for _, nr := range allowed {
		f = append(f, bpfEq(nr, 0, 1), bpfRet(unix.SECCOMP_RET_ALLOW))
	}
	return append(f, deny), nil
}

// onlyValues refuses system call nr unless the 32-bit word at off is one of
// values; a listed value falls through to the allow-list.
func onlyValues(f []unix.SockFilter, nr, off uint32, values []uint32, deny unix.SockFilter) []unix.SockFilter {
	n := len(values)                                      // at most maxListed
	f = append(f, bpfEq(nr, 0, uint8(n+3)), bpfLoad(off)) // #nosec G115 -- n <= maxListed.
	for i, v := range values {
		f = append(f, bpfEq(v, uint8(n-i), 0)) // #nosec G115 -- n <= maxListed.
	}
	return append(f, deny, bpfLoad(offNR))
}

// installSeccompProgram does not allocate: it only passes f to the kernel.
func installSeccompProgram(f []unix.SockFilter) error {
	prog := unix.SockFprog{Len: uint16(len(f)), Filter: &f[0]}                                             // #nosec G115 -- fixed small filter.
	return unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(&prog)), 0, 0) // #nosec G103 -- synchronous kernel copy of BPF program.
}
