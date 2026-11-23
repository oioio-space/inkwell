package sidecar

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

// sharedCache is the Store directory of the tests that run the real sidecars:
// a test-only folder of the user cache, kept across runs, so that the 130 MB of
// pinned binaries are extracted once per pin, not once per run. Store names each
// binary by version and SHA-256 and hashes it before every use, as the product
// does, so a stale or damaged copy is replaced, never run. It is not per run on
// purpose: a run killed before its cleanup (a timeout, heavy.sh's memory cap,
// a mutant) left its 130 MB copy in /tmp, a tmpfs, for good (119 of them,
// 2026-11-23).
var sharedCache string

// helperDir is the working directory and GOCOVERDIR (helperEnv) of the
// re-executed test binary. It outlives the run as well and stays empty: a
// -cover run of the package wrote nothing in it (2026-11-23).
var helperDir string

// testCacheDirs returns sharedCache and helperDir under the user cache, apart
// from the product's own folder (DefaultDir): the tier probe verifies its typst
// there, never in the user's cache.
func testCacheDirs() (store, helper string, err error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", "", err
	}
	root := filepath.Join(base, "inkwell-test", "sidecar")
	return filepath.Join(root, "store"), filepath.Join(root, "helper"), nil
}

func TestMain(m *testing.M) {
	mode := os.Getenv(modeEnv)
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--sidecar-test=") {
		mode = strings.TrimPrefix(os.Args[1], "--sidecar-test=")
	}
	if mode != "" {
		if os.Getenv("INKWELL_REPORT_HOST_PID") == "1" {
			pid := os.Getpid()
			if data, err := os.ReadFile("/proc/self/status"); err == nil {
				for line := range strings.SplitSeq(string(data), "\n") {
					if strings.HasPrefix(line, "Pid:") {
						_, _ = fmt.Sscanf(line, "Pid: %d", &pid)
					}
				}
			}
			// A fresh procfs intentionally hides outer PIDs. Report the PID
			// namespace inode instead so the parent can verify its destruction.
			if ns, err := os.Readlink("/proc/self/ns/pid"); err == nil && os.Getenv("INKWELL_PRIVATE_PID") == "1" {
				inode, parseErr := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(ns, "pid:["), "]"))
				if parseErr == nil {
					pid = -inode
				}
			}
			fmt.Printf("gc=%d\n", pid)
		}
		helperExit(runHelper(mode))
	}
	// Without a user cache (a confined child gets no HOME) DefaultDir fails too,
	// so the zero sharedCache can never fall back to the user's cache.
	if store, helper, err := testCacheDirs(); err == nil {
		if err := os.MkdirAll(helper, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sharedCache, helperDir = store, helper
		defaultStore = &Store{Dir: store}
	}
	code := m.Run()
	if code == 0 {
		if err := goleak.Find(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

// helperExit ends a helper process. Under -cover it uses syscall.Exit, which
// skips the coverage exit hook: a confined helper cannot write its coverage
// files, and the resulting error would pollute the stderr the tests assert on.
// Otherwise it uses os.Exit, whose exit hooks include the race detector's: a
// helper that raced must exit 66, not the code it returned.
func helperExit(code int) {
	if testing.CoverMode() != "" {
		syscall.Exit(code)
	}
	os.Exit(code)
}

func TestHelperExitKeepsRaceVerdict(t *testing.T) {
	err := helperCmd("race").Run()
	got := 0
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		got = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	// The race detector's exit hook turns a race into status 66. Under -cover the
	// helper skips every exit hook (helperExit), so it keeps the code it returned.
	want := 0
	if raceEnabled && testing.CoverMode() == "" {
		want = 66
	}
	if got != want {
		t.Fatalf("racy helper exited %d, want %d (race %v, cover %q)", got, want, raceEnabled, testing.CoverMode())
	}
}

// childTest returns a command that re-executes this test binary to run only
// the test called name, with env added, so that process-wide effects (mounts,
// resource limits) never reach this process.
func childTest(name string, env ...string) *exec.Cmd {
	args := []string{"-test.run=^" + name + "$", "-test.v"}
	if f := flag.Lookup("test.gocoverdir"); f != nil && f.Value.String() != "" {
		args = append(args, "-test.gocoverdir="+f.Value.String())
	}
	cmd := exec.Command(os.Args[0], args...) // #nosec G204 -- re-executes the test binary
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// runChild runs cmd and fails unless the child really ran and passed name.
func runChild(t *testing.T, cmd *exec.Cmd, name string) {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("--- PASS: "+name)) {
		t.Fatalf("child %s: %v\n%s", name, err, out)
	}
}

// helperEnv is the whole environment of a re-executed test binary. A -cover
// test binary prints "GOCOVERDIR not set" from its os.Exit hook, at exit: the
// helper itself skips that hook, but the sandbox launcher, which is this same
// binary, exits through os.Exit and needs GOCOVERDIR to keep stderr clean.
func helperEnv(mode string) []string {
	return []string{modeEnv + "=" + mode, "GOCOVERDIR=" + helperDir}
}

// sink keeps the helper's allocation alive.
var sink []byte

// runHelper is the body of a fake sidecar; it returns the exit code.
func runHelper(mode string) int {
	switch name, arg, _ := strings.Cut(mode, ":"); name {
	case "build-probe-fixtures":
		return buildProbeFixtures(arg)
	case "sandboxstatus":
		data, err := os.ReadFile("/proc/self/status")
		if err != nil {
			return 1
		}
		_, _ = os.Stdout.Write(data)
		return 0
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
		c.Env = append(os.Environ(), modeEnv+"=sleep", "INKWELL_REPORT_HOST_PID=1")
		if os.Getppid() == 1 {
			c.Env = append(c.Env, "INKWELL_PRIVATE_PID=1")
		}
		c.Stdout = os.Stdout
		if err := c.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		time.Sleep(time.Hour)
	case "alloc": // reserve arg MiB of address space (the pages stay untouched)
		mib, _ := strconv.Atoi(arg)
		sink = make([]byte, mib<<20)
		fmt.Println("allocated", len(sink))
		return 0
	case "scope-resident-tree", "scope-resident-chunk", "scope-budget", "scope-resident", "scope-resident-wait", "scope-address", "scope-tree", "scope-fds", "scope-fds-tree", "scope-scope-app", "scope-pagecache", "scope-dirty":
		return scopeAttackHelper(strings.TrimPrefix(name, "scope-"), arg)
	case "lower-core":
		return lowerCoreHelper(arg)
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
	case "boundattack", "boundattack-child":
		if !hardFileLimit() {
			return 4
		}
		// Ignore SIGXFSZ so that both files reach their caps and every attempted
		// escape runs even after a failed write. Also exercise a forked writer.
		ignoreFileSizeSignal()
		for range 20 {
			for _, name := range []string{"a", "b", "undeclared"} {
				f, err := os.OpenFile(arg+"/"+name, os.O_CREATE|os.O_WRONLY, 0o600)
				if err == nil {
					_ = f.Truncate(64 << 20)
					_, _ = f.Write(make([]byte, 8<<20))
					_ = f.Close()
				}
			}
			_ = os.Mkdir(arg+"/sub", 0o700)
			_ = os.Rename(arg+"/a", arg+"/renamed")
			_ = os.Link(arg+"/a", arg+"/linked")
			_ = os.Remove(arg + "/a")
		}
		if name == "boundattack" {
			child := helperCmd("boundattack-child:" + arg)
			child.Dir = "" // Already in the sandbox project; no chdir syscall is needed.
			if out, err := child.CombinedOutput(); err != nil {
				fmt.Fprintf(os.Stderr, "descendant: %v: %s", err, out)
				return 3
			}
		}
		return 0
	case "capwrite":
		return capWriteHelper(arg)
	case "sparsemount":
		for _, name := range []string{"a", "b"} {
			f, err := os.OpenFile(arg+"/"+name, os.O_WRONLY, 0o600)
			if err != nil {
				return 2
			}
			err = f.Truncate(8 << 20)
			_ = f.Close()
			if err != nil {
				return 3
			}
		}
		return 0
	case "fillmount":
		chunk := make([]byte, 4096)
		for _, name := range []string{"a", "b"} {
			f, err := os.OpenFile(arg+"/"+name, os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				return 2
			}
			for range 192 {
				_, err = f.Write(chunk)
				if dirBytes(arg) > 1<<20 {
					return 3
				}
				if errors.Is(err, syscall.ENOSPC) {
					_ = f.Close()
					fmt.Println("ENOSPC")
					return 0
				}
				if err != nil {
					return 4
				}
			}
			_ = f.Close()
		}
		return 5
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
	case "race": // two unsynchronised writes; the race detector must flag them
		var shared int
		done := make(chan struct{})
		go func() {
			shared = 1
			close(done)
		}()
		shared = 2
		<-done
		sink = make([]byte, shared) // uses the value, so neither write is dead
		return 0
	default:
		return tierHelper(name, arg)
	}
	return 0
}

// helperCmd returns a command that runs the test binary in the given mode.
func helperCmd(mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "--sidecar-test="+mode) // #nosec G204 -- re-executes the test binary
	cmd.Env = helperEnv(mode)
	cmd.Dir = helperDir
	if cmd.Dir == "" {
		cmd.Dir, _ = os.Getwd()
	}
	return cmd
}

// alive reports whether pid is a running process (a zombie awaiting its
// reaper counts as dead).
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
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

// waitGone waits up to d for pid to die.
func waitGone(pid int, d time.Duration) bool {
	stillAlive := func() bool {
		if pid >= 0 {
			return alive(pid)
		}
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return true
		}
		want := fmt.Sprintf("pid:[%d]", -pid)
		for _, entry := range entries {
			if _, err := strconv.Atoi(entry.Name()); err != nil {
				continue
			}
			ns, _ := os.Readlink(filepath.Join("/proc", entry.Name(), "ns/pid"))
			if ns == want {
				return true
			}
		}
		return false
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !stillAlive() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !stillAlive()
}

func deathSpec(t *testing.T, mode string) Spec { s := spec(mode); s.DeathTest = t.Name(); return s }
