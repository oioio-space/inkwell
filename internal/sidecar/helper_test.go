package sidecar

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// modeEnv selects what the re-executed test binary does when it plays a
// sidecar: the tests launch os.Args[0] as the "program" to supervise.
const modeEnv = "INKWELL_SIDECAR_TEST_MODE"

// sharedCache is the cache directory of the tests that run the real sidecars,
// so that the 77 MB tinymist is extracted once, not once per test.
var sharedCache string

func TestMain(m *testing.M) {
	if mode := os.Getenv(modeEnv); mode != "" {
		os.Exit(runHelper(mode))
	}
	dir, err := os.MkdirTemp("", "inkwell-sidecar-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sharedCache = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	if code == 0 {
		if err := goleak.Find(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

// helperEnv is the whole environment of a re-executed test binary. A binary
// built with -cover warns on stderr at exit unless GOCOVERDIR is set, which
// would pollute the output the tests assert on.
func helperEnv(mode string) []string {
	return []string{modeEnv + "=" + mode, "GOCOVERDIR=" + sharedCache}
}

// sink keeps the helper's allocation alive.
var sink []byte

// runHelper is the body of a fake sidecar; it returns the exit code.
func runHelper(mode string) int {
	switch name, arg, _ := strings.Cut(mode, ":"); name {
	case "echo":
		_, _ = fmt.Fprint(os.Stdout, "out\n")
		_, _ = fmt.Fprint(os.Stderr, "err\n")
		return 0
	case "exit":
		code, _ := strconv.Atoi(arg)
		return code
	case "sleep":
		time.Sleep(time.Hour)
	case "ignoreterm": // only SIGKILL stops it
		signal.Ignore(syscall.SIGTERM)
		time.Sleep(time.Hour)
	case "sleepms": // run for arg milliseconds, then crash
		ms, _ := strconv.Atoi(arg)
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return 1
	case "flood":
		chunk := make([]byte, 1<<16)
		for range 64 {
			_, _ = os.Stdout.Write(chunk)
			_, _ = os.Stderr.Write(chunk)
		}
		return 0
	case "cat":
		_, _ = io.Copy(os.Stdout, os.Stdin)
		return 0
	case "env":
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, modeEnv+"=") && !strings.HasPrefix(kv, "GOCOVERDIR=") {
				fmt.Println(kv)
			}
		}
		return 0
	case "pwd":
		wd, _ := os.Getwd()
		fmt.Println(wd)
		return 0
	case "grandchild": // start a child in the same process group, tell its pid, wait
		c := exec.Command(os.Args[0]) // #nosec G204 -- re-executes the test binary
		c.Env = append(os.Environ(), modeEnv+"=sleep")
		if err := c.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		fmt.Printf("gc=%d\n", c.Process.Pid)
		time.Sleep(time.Hour)
	case "alloc": // reserve arg MiB of address space (the pages stay untouched)
		mib, _ := strconv.Atoi(arg)
		sink = make([]byte, mib<<20)
		fmt.Println("allocated", len(sink))
		return 0
	case "rlimits": // report the limits in force at startup, before any of our code ran
		return reportLimits()
	case "writefile": // arg is "path,MiB": write that much, report if it got through
		path, mib, _ := strings.Cut(arg, ",")
		n, _ := strconv.Atoi(mib)
		f, err := os.Create(path)
		if err != nil {
			return 2
		}
		chunk := make([]byte, 1<<20)
		for range n {
			if _, err := f.Write(chunk); err != nil {
				fmt.Println("write-failed")
				return 3
			}
		}
		fmt.Println("wrote-all")
		return 0
	case "flooddir": // write 1 MiB files into arg, forever
		chunk := make([]byte, 1<<20)
		for i := 0; ; i++ {
			if err := os.WriteFile(fmt.Sprintf("%s/f%06d", arg, i), chunk, 0o600); err != nil {
				return 2
			}
		}
	case "dial": // try to reach the TCP address arg
		conn, err := net.DialTimeout("tcp", arg, 2*time.Second)
		if err != nil {
			fmt.Println("unreachable")
			return 0
		}
		_ = conn.Close()
		fmt.Println("connected")
		return 0
	case "supervise-forever": // the parent that gets killed in the orphan test
		return superviseForever()
	}
	return 0
}

// helperCmd returns a command that runs the test binary in the given mode.
func helperCmd(mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0]) // #nosec G204 -- re-executes the test binary
	cmd.Env = helperEnv(mode)
	return cmd
}

// alive reports whether pid is a running process (a zombie awaiting its
// reaper counts as dead).
func alive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true // no /proc: kill(0) is all there is
	}
	// "pid (comm) S ...": the state follows the last parenthesis.
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] != 'Z'
	}
	return true
}

func syscallKill(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }

// waitGone waits up to d for pid to die.
func waitGone(pid int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !alive(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !alive(pid)
}
