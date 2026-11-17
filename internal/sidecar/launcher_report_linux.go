package sidecar

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// readHandshakeFD takes the handshake descriptor from the environment, so
// that the target never sees it.
func readHandshakeFD(c *sandboxConfig) error {
	value := os.Getenv(handshakeEnv)
	if value == "" {
		return errors.New("sidecar: no launch handshake")
	}
	fd, err := strconv.Atoi(value)
	if err != nil || fd < 3 {
		return errors.New("sidecar: invalid launch handshake descriptor")
	}
	c.HandshakeFD = fd
	return os.Unsetenv(handshakeEnv)
}

// launcherFilter is the seccomp filter of the program and tier the parent
// resolved (ADR 0044 point 1f).
func launcherFilter(c sandboxConfig) ([]unix.SockFilter, error) {
	a, err := hostSysArch()
	if err != nil {
		return nil, err
	}
	t := tierN
	if c.Tier == tierF.String() {
		t = tierF
	} else if c.Tier != tierN.String() {
		return nil, fmt.Errorf("sidecar: unknown tier %q", c.Tier)
	}
	p := program{name: c.Program, syscalls: map[string][]uint32{runtime.GOARCH: c.Syscalls}, prctl: c.Prctl, ioctl: c.Ioctl}
	return seccompFilter(p, a, uint32(unix.Getpid()), t) // #nosec G115 -- positive Linux PID fits uint32.
}

// reportPrefix is the tier report up to its last field: the seccomp mode is
// read after the filter is installed. Built before RLIMIT_AS, with room for
// that field, so that completing it does not allocate.
func reportPrefix(c sandboxConfig) ([]byte, error) {
	ns, err := ownNamespaces()
	if err != nil {
		return nil, err
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return nil, err
	}
	d := identityFromStatus(status)
	line := fmt.Sprintf("tier=%s ns=%s proc=%t landlock=%d seccomp=", c.Tier, strings.Join(ns[:], ","), d.PID == unix.Getpid(), c.landlock)
	return append(make([]byte, 0, len(line)+4), line...), nil
}

// reportAndAwait completes and writes the tier report, then waits, with a
// deadline, for the parent's « proceed ». Nothing here allocates: RLIMIT_AS
// and seccomp are in force.
func reportAndAwait(fd int, report, status []byte, poll []unix.PollFd, proceed []byte) error {
	mode, err := seccompMode(status)
	if err != nil {
		return err
	}
	report = append(report, mode, '\n')
	if n, err := unix.Write(fd, report); err != nil || n != len(report) {
		return errors.New("sidecar: tier report not written")
	}
	if err := pollFds(poll, handshakeTimeout); err != nil {
		return err
	}
	if n, err := unix.Read(fd+1, proceed); err != nil || n != 1 || proceed[0] != 'p' {
		return errors.New("sidecar: no « proceed » from the parent")
	}
	return nil
}

// seccompMode reads the Seccomp: digit of /proc/self/status into status.
func seccompMode(status []byte) (byte, error) {
	fd, err := unix.Open("/proc/self/status", unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	n, err := unix.Read(fd, status)
	_ = unix.Close(fd)
	if err != nil {
		return 0, err
	}
	const key = "\nSeccomp:\t"
	i := bytes.Index(status[:n], []byte(key))
	if i < 0 || i+len(key) >= n {
		return 0, errors.New("sidecar: seccomp mode unreadable")
	}
	return status[i+len(key)], nil
}

// pollReadable waits, at most timeout, for fd to be readable.
func pollReadable(fd int, timeout time.Duration) error {
	return pollFds([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, timeout) // #nosec G115 -- a small descriptor.
}

// pollFds waits, at most timeout, for the first descriptor of poll to be
// readable or closed; it does not allocate. x/sys's Poll is ppoll, which the
// amd64 allow-list does not hold: this calls the architecture's own (poll on
// amd64, ppoll on arm64, where the sidecars are refused until SB2a2).
func pollFds(poll []unix.PollFd, timeout time.Duration) error {
	a, err := hostSysArch()
	if err != nil {
		return err
	}
	ts := unix.NsecToTimespec(timeout.Nanoseconds())
	// poll takes an int of milliseconds: a negative value would wait for ever, a large one would wrap.
	ms := uintptr(min(max(timeout.Milliseconds(), 0), math.MaxInt32)) // #nosec G115 -- clamped to [0, MaxInt32] just before.
	for {
		var n uintptr
		var errno unix.Errno
		if a.name == "amd64" {
			n, _, errno = unix.Syscall(uintptr(a.poll), uintptr(unsafe.Pointer(&poll[0])), uintptr(len(poll)), ms) // #nosec G103 -- the kernel reads the array synchronously.
		} else {
			n, _, errno = unix.Syscall6(uintptr(a.poll), uintptr(unsafe.Pointer(&poll[0])), uintptr(len(poll)), uintptr(unsafe.Pointer(&ts)), 0, 0, 0) // #nosec G103 -- the kernel reads the array and the timeout synchronously.
		}
		if errno == unix.EINTR {
			continue
		}
		if errno != 0 || n == 0 {
			return errors.New("sidecar: no « proceed » from the parent")
		}
		return nil
	}
}
