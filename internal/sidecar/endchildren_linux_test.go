package sidecar

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// endChildrenEnv selects the case TestEndChildren plays in a process of its own: ending the
// children of a process reaps every child it has.
const endChildrenEnv = "INKWELL_END_CHILDREN"

// TestEndChildren: a process ends the children it still has before it executes itself again
// (rule N-14 of docs/plans/2026-11-14-pc1-project-change-by-restart.md).
func TestEndChildren(t *testing.T) {
	const name = "TestEndChildren"
	if mode := os.Getenv(endChildrenEnv); mode != "" {
		endChildrenCase(t, mode)
		return
	}
	for _, c := range []string{"ignores-term", "grandchild", "deadline"} {
		t.Run(c, func(t *testing.T) {
			runChild(t, childTest(name, endChildrenEnv+"="+c), name)
		})
	}
}

// foreignChild starts a shell that runs script, as a process this package does not supervise, and
// returns the lines it printed up to « ready ».
func foreignChild(t *testing.T, script string, env ...string) (pid int, lines []string) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", script) // #nosec G204 -- a fixed test script
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if sc.Text() == "ready" {
			return cmd.Process.Pid, lines
		}
		lines = append(lines, sc.Text())
	}
	t.Fatalf("the child never said ready: %v %q", sc.Err(), lines)
	return 0, nil
}

// endChildrenCase plays the case mode in this process, which must have no other child.
func endChildrenCase(t *testing.T, mode string) {
	t.Helper()
	const grace = 150 * time.Millisecond
	switch mode {
	case "ignores-term":
		// The child ignores the polite request: it is killed once the grace has passed, not before.
		pid, _ := foreignChild(t, `trap "" TERM; echo ready; exec sleep 600`)
		start := time.Now()
		if err := EndChildren(5*time.Second, grace); err != nil {
			t.Fatalf("N-14 remaining children are ended: %v", err)
		}
		if took := time.Since(start); took < grace {
			t.Errorf("N-14 a child gets the grace before it is killed: ended after %v, grace %v", took, grace)
		}
		assertNoChild(t, pid)
	case "grandchild":
		// The child ends on the request and leaves a process behind, like a sandbox holder: that
		// process comes to this one (child subreaper), not to init, and is ended too.
		asked := filepath.Join(t.TempDir(), "asked")
		pid, lines := foreignChild(t, `sleep 600 & echo $!; trap 'echo term > "$ASKED"; exit 0' TERM; echo ready; wait`, "ASKED="+asked)
		if len(lines) != 1 {
			t.Fatalf("the child printed %q, want the pid of its own child", lines)
		}
		grand, err := strconv.Atoi(lines[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := EndChildren(5*time.Second, grace); err != nil {
			t.Fatalf("N-14 remaining children are ended: %v", err)
		}
		if b, _ := os.ReadFile(asked); strings.TrimSpace(string(b)) != "term" { // #nosec G304 -- the test's own file
			t.Errorf("N-14 a child is asked to end before it is killed: the child got no SIGTERM (%q)", b)
		}
		assertNoChild(t, pid, grand)
		// Once no child is left the process is no reaper for anyone: the attribute would be kept
		// by the image it executes.
		var on int32
		if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&on)), 0, 0, 0); err != nil || on != 0 { // #nosec G103 -- the kernel writes an int there
			t.Errorf("N-14 the process is still a child subreaper once its children ended (%d, %v)", on, err)
		}
	case "deadline":
		// The deadline passes before the grace: the failure names the child that still runs.
		pid, _ := foreignChild(t, `trap "" TERM; echo ready; exec sleep 600`)
		err := EndChildren(100*time.Millisecond, time.Minute)
		_ = unix.Kill(pid, unix.SIGKILL)
		want := fmt.Sprintf("1 child processes still running: [%d (sleep)]", pid)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("N-14 a child that still runs fails the barrier by name: err = %v, want %q", err, want)
		}
	default:
		t.Fatalf("unknown case %q", mode)
	}
}

// assertNoChild fails unless this process has no child left and none of pids runs.
func assertNoChild(t *testing.T, pids ...int) {
	t.Helper()
	if kids, err := Children(); err != nil || len(kids) != 0 {
		t.Errorf("N-14 remaining children are ended: children = %v, %v; want none", kids, err)
	}
	for _, pid := range pids {
		if alive(pid) {
			t.Errorf("N-14 remaining children are ended: %d still runs", pid)
			_ = unix.Kill(pid, unix.SIGKILL)
		}
	}
}

// TestChildrenUnreadableIsAnError: the barrier counts children from the kernel's per-thread
// lists; when none can be read (a kernel without them), it does not know, and says so instead of
// reporting no child. A thread that ended between the listing and the read is skipped (rule N-14).
func TestChildrenUnreadableIsAnError(t *testing.T) {
	tasks := t.TempDir()
	for _, tid := range []string{"1", "2"} {
		if err := os.Mkdir(filepath.Join(tasks, tid), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if kids, err := childrenOf(tasks); err == nil {
		t.Errorf("N-14 no child survives: no readable list = %v, nil; want an error", kids)
	}
	if err := os.WriteFile(filepath.Join(tasks, "2", "children"), []byte("41 42 "), 0o600); err != nil {
		t.Fatal(err)
	}
	if kids, err := childrenOf(tasks); err != nil || len(kids) != 2 {
		t.Errorf("children with one thread ended = %v, %v; want the two of the other", kids, err)
	}
	if _, err := Children(); err != nil {
		t.Errorf("children of this process: %v", err)
	}
}
