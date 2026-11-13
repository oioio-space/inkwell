//go:build linux

package sidecar

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func ignoreFileSizeSignal() { signal.Ignore(syscall.SIGXFSZ) }

// capChunk is the size of each raw write of the "capwrite" helper.
const capChunk = 48 << 10

// capWriteHelper writes past the file-size cap of the file arg names ("path,end")
// with raw writes, so a short count is seen, and ends as end says (plan, Gate
// SB1b, rows X1 to X4): "short" reports the short write and the error after it
// and exits 1; "abort" aborts on EFBIG; "reset" restores SIGXFSZ's default
// action first; "loop" retries forever.
func capWriteHelper(arg string) int {
	path, end, _ := strings.Cut(arg, ",")
	if end == "reset" {
		if err := defaultAction(unix.SIGXFSZ); err != nil {
			return 2
		}
	}
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return 2
	}
	chunk := make([]byte, capChunk)
	short := -1
	for {
		n, err := unix.Write(fd, chunk)
		switch {
		case err == nil && n < len(chunk) && short < 0:
			short = n
		case errors.Is(err, unix.EFBIG) && end == "short":
			fmt.Printf("short=%d next=%v\n", short, err)
			return 1
		case errors.Is(err, unix.EFBIG) && end == "abort":
			_ = defaultAction(unix.SIGABRT)
			_ = unix.Tgkill(unix.Getpid(), unix.Gettid(), unix.SIGABRT)
			time.Sleep(time.Hour)
		case errors.Is(err, unix.EFBIG) && end == "loop":
			continue
		case err != nil:
			fmt.Printf("unexpected %v\n", err)
			return 3
		}
	}
}

// defaultAction installs the kernel default action of sig, bypassing Go's handler.
func defaultAction(sig syscall.Signal) error {
	action := [4]uint64{}
	if _, _, errno := unix.RawSyscall6(unix.SYS_RT_SIGACTION, uintptr(sig), uintptr(unsafe.Pointer(&action)), 0, 8, 0, 0); errno != 0 { // #nosec G103 -- the kernel copies the action.
		return errno
	}
	return nil
}

// fileSize is the size of path, -1 if it cannot be read.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

// Try both native limit-setting interfaces; neither may alter the sentinel.
func lowerCoreHelper(api string) int {
	limit := unix.Rlimit{Cur: 0, Max: 1}
	var err error
	if api == "setrlimit" {
		_, _, errno := unix.RawSyscall(uintptr(setrlimitSyscall), unix.RLIMIT_CORE, uintptr(unsafe.Pointer(&limit)), 0)
		if errno != 0 {
			err = errno
		}
	} else {
		err = unix.Prlimit(0, unix.RLIMIT_CORE, &limit, nil)
	}
	if !errors.Is(err, unix.EPERM) {
		fmt.Printf("lower core: %v\n", err)
		return 1
	}
	if err := unix.Prlimit(0, unix.RLIMIT_CORE, nil, &limit); err != nil || limit.Cur != 1 || limit.Max != 1 {
		return 2
	}
	fmt.Println("core=1/1 protected")
	return 0
}

func scopeAttackHelper(mode, arg string) int {
	switch mode {
	case "resident-tree":
		var children []*exec.Cmd
		for range 6 {
			child := exec.Command(os.Args[0], "--sidecar-test=scope-resident-chunk")
			child.Env = os.Environ()
			child.Stderr = os.Stderr
			ready, err := child.StdoutPipe()
			if err != nil {
				return 2
			}
			if err := child.Start(); err != nil {
				return 2
			}
			line, err := bufio.NewReader(ready).ReadString('\n')
			if err != nil {
				return 2
			}
			fmt.Print(line)
			children = append(children, child)
		}
		for _, child := range children {
			_ = child.Wait()
		}
		return 1
	case "resident-chunk":
		b, err := unix.Mmap(-1, 0, 32<<20, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
		if err != nil {
			return 2
		}
		for i := 0; i < len(b); i += 4096 {
			b[i] = 1
		}
		fmt.Println("resident-child-ready")
		time.Sleep(time.Hour)
		runtime.KeepAlive(b)
		return 0
	case "budget":
		var limit unix.Rlimit
		if err := unix.Prlimit(0, unix.RLIMIT_AS, nil, &limit); err != nil {
			return 2
		}
		fmt.Printf("as=%d/%d\n", limit.Cur, limit.Max)
		return 0
	case "resident-wait":
		fmt.Println("memory-ready")
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return 2
		}
		return scopeAttackHelper("resident", "")
	case "resident":
		for {
			b, err := unix.Mmap(-1, 0, 8<<20, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
			if err != nil {
				fmt.Println("allocation-failed")
				return 1
			}
			for i := 0; i < len(b); i += 4096 {
				b[i] = 1
			}
		}
	case "address":
		_, err := unix.Mmap(-1, 0, 2<<30, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
		if err != nil {
			fmt.Println("allocation-failed")
			return 1
		}
		return 0
	case "tree":
		cmd := exec.Command(os.Args[0], "--sidecar-test=sleep:1h")
		cmd.Env = os.Environ()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Println(err)
			return 2
		}
		fmt.Printf("tree=%d,%d\n", os.Getpid(), cmd.Process.Pid)
		var b [1]byte
		_, _ = os.Stdin.Read(b[:])
		return 0
	case "fds-tree":
		if scopeAttackHelper("fds", "") != 0 {
			return 2
		}
		cmd := exec.Command(os.Args[0], "--sidecar-test=scope-fds")
		cmd.Env = os.Environ()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "descriptor descendant:", err)
			return 2
		}
		return 0
	case "fds":
		// Landlock's /proc/self grant names the original target's proc inode;
		// a forked child cannot enumerate its own proc directory. Query every
		// possible descriptor directly, preserving the same strict oracle.
		var limit unix.Rlimit
		if err := unix.Prlimit(0, unix.RLIMIT_NOFILE, nil, &limit); err != nil {
			fmt.Fprintln(os.Stderr, "descriptor limit:", err)
			return 2
		}
		// Fail closed if the environment exceeds the bounded exhaustive scan.
		const maxDescriptorScan = 1 << 20
		if limit.Cur > maxDescriptorScan {
			fmt.Fprintln(os.Stderr, "descriptor limit exceeds scan cap:", limit.Cur)
			return 2
		}
		for fd := uint64(0); fd < limit.Cur; fd++ {
			flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
			if errors.Is(err, unix.EBADF) {
				continue
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "descriptor query:", err)
				return 2
			}
			target, err := os.Readlink("/proc/self/fd/" + strconv.FormatUint(fd, 10))
			if err != nil {
				fmt.Fprintln(os.Stderr, "descriptor link:", err)
				return 2
			}
			fmt.Printf("fd=%d cloexec=%d target=%s\n", fd, flags&unix.FD_CLOEXEC, target)
		}
		return 0
	case "scope-app":
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return 2
		}
		var held []byte
		if arg == "aggregate" {
			var err error
			held, err = unix.Mmap(-1, 0, 96<<20, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
			if err != nil {
				return 2
			}
			for i := 0; i < len(held); i += 4096 {
				held[i] = 1
			}
		}
		manager := &SandboxScope{}
		manager.beforeStart = func(r *cgroupRun) { fmt.Printf("scope=%s holder=%d\n", r.session.path, r.session.holder.Process.Pid) }
		mode := "tree"
		if arg == "aggregate" || arg == "leaf" {
			mode = "resident-wait"
		}
		s := Spec{
			Path:          os.Args[0],
			Dir:           os.Getenv("INKWELL_TEST_PROJECT"),
			Args:          []string{"--sidecar-test=scope-" + mode},
			Env:           []string{},
			MemoryLimit:   2 << 30,
			ResidentLimit: 256 << 20,
			Stdin:         os.Stdin,
			Stdout:        os.Stdout,
			scope:         manager,
		}
		out, err := Run(context.Background(), s)
		fmt.Printf("out=%s cause=%s signal=%d err=%v\n", out.Containment, out.MemoryCause, out.Signal, err)
		runtime.KeepAlive(held)
		manager.mu.Lock()
		if manager.session != nil {
			closeSession(manager.session)
			manager.session = nil
		}
		manager.mu.Unlock()
		if err != nil {
			return 2
		}
		return 0
	}
	return 2
}
