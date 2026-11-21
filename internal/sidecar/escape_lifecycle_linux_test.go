package sidecar

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func escapeExecSlot(t *testing.T, tr tier, abi int) {
	if !requireTier(t, tr) {
		return
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("S-4 escape: executable fixture: %v", err)
	}
	s := spec("")
	s.Dir = t.TempDir()
	s.tier = tr
	s.landlockABI = abi
	s.WatchDir = t.TempDir()
	s.WriteFiles = []string{"payload"}
	s.MaxDirBytes = int64(len(data)) + 1<<20
	s.Env = helperEnv("sb2-attack:exec-slot|" + filepath.Join(s.WatchDir, "payload") + "|0")
	var mount *directoryMount
	var preparationErr error
	s.afterReport = func() {
		var f *os.File
		var err error
		if mount != nil {
			f, err = mount.root.OpenFile("payload", os.O_WRONLY|os.O_TRUNC, 0)
		} else {
			f, err = os.OpenFile(filepath.Join(s.WatchDir, "payload"), os.O_WRONLY|os.O_TRUNC, 0)
		}
		if err != nil {
			preparationErr = err
			return
		}
		defer func() { _ = f.Close() }()
		if _, err = f.Write(data); err != nil {
			preparationErr = err
			return
		}
		// Supply both a valid ELF and ordinary execute permission. Only the
		// sandbox's no-execute rule can explain the target's EACCES.
		preparationErr = f.Chmod(0o700)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := validateLaunch(ctx, &s); err != nil {
		t.Fatalf("S-4 escape: validate slot: %v", err)
	}
	out, err := run(ctx, s, testPlan(tr), func(s Spec) (*directoryMount, error) { var e error; mount, e = newDirectoryMount(s); return mount, e })
	if preparationErr != nil || err != nil || out.ExitCode != 0 || string(out.Stdout) != "refused\n" {
		t.Fatalf("S-4 escape: exec-slot: prep=%v out=%+v err=%v", preparationErr, out, err)
	}
}

func escapeAllocation(t *testing.T, tr tier, abi int) {
	if !requireTier(t, tr) {
		return
	}
	// The helper maps the whole limit at once, which the limit can never hold.
	// The limit is 2 GiB, not 1: under 1 GiB the Go runtime itself failed to
	// start once in a hundred runs (measured 2026-11-15: 4 of 400 at 1 GiB,
	// 0 of 400 at 2 GiB), and that is not the allocation refused.
	limit := 2<<30 + raceAddressReserve(t)
	s := spec("scope-address:" + strconv.FormatUint(limit, 10))
	s.Dir = t.TempDir()
	s.tier = tr
	s.landlockABI = abi
	s.disableScope = true
	s.MemoryLimit = limit
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	out, err := Run(ctx, s)
	if err != nil || out.ExitCode != 1 || string(out.Stdout) != "allocation-failed\n" || out.Containment != "rlimit" {
		t.Fatalf("S-11 no oom-kill: allocation: %+v %v", out, err)
	}
}

func escapeParentDeath(t *testing.T, tr tier, abi int) {
	if !requireTier(t, tr) {
		return
	}
	dir := t.TempDir()
	cmd := helperCmd("sb2-parent:running|" + dir + "|" + tr.String() + "|" + strconv.Itoa(abi))
	p, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("S-4 parent check: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("S-4 parent check: %v", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	reader := bufio.NewReader(p)
	text, _ := readLine(t, reader)
	pid := phasePID(text, "running")
	if pid == 0 {
		t.Fatalf("S-4 parent check: bad phase %q", text)
	}
	defer func() { _ = unix.Kill(pid, unix.SIGKILL) }()
	namespace := 0
	if tr == tierN {
		namespace = pidNamespace(pid)
	}
	readTargetStarted(t, reader)
	_ = cmd.Process.Kill()
	if !waitGone(pid, time.Second) {
		t.Fatalf("S-4 parent check: launcher %d survived", pid)
	}
	if tr == tierN && (namespace == 0 || !waitGone(-namespace, time.Second)) {
		t.Fatalf("S-4 parent check: PID namespace %d survived", namespace)
	}
}

func escapeNoProcess(t *testing.T, abi int) {
	if !requireLaunch(t) {
		return
	}
	s := spec("sb2-fork")
	s.tier = tierF
	s.landlockABI = abi
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	out, err := Run(ctx, s)
	if err != nil || out.ExitCode != 0 || string(out.Stdout) != "fork refused\n" {
		t.Fatalf("S-2 no process: F fork attempt: %+v %v", out, err)
	}
}

func forkAttemptHelper() int {
	cmd := exec.Command(os.Args[0], "--sidecar-test=exit:0")
	cmd.Env = []string{}
	err := cmd.Run()
	if !errors.Is(err, unix.EPERM) {
		fmt.Println("fork result:", err)
		return 3
	}
	fmt.Println("fork refused")
	return 0
}

// A bounded reproduction of a fork bomb: eight live descendants are enough
// to prove that cancellation destroys their PID namespace, without creating
// an unbounded number of processes on a broken candidate.
func escapeProcessTree(t *testing.T) {
	if !requireN(t) {
		return
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := spec("sb2-tree")
	s.Dir = t.TempDir()
	s.tier = tierN
	s.disableScope = true
	// The package's 2 GiB address bound (scopeSpec): the Go runtime sizes its
	// reservations to RLIMIT_AS, and under 1 GiB a cgo, coverage-built helper
	// keeps about 10 MiB for its threads' stacks — one stack size above 8 MiB
	// and it cannot start (measured 2026-11-16; the `ci` runner failed so).
	s.MemoryLimit = 2<<30 + raceAddressReserve(t)
	ready := make(chan struct{}, 1)
	s.Stdout = &readyWriter{ready: ready, seen: func(text string) bool { return strings.Contains(text, "children=8\n") }}
	var cmd *exec.Cmd
	namespace := 0
	s.rewriteLauncher = func(c *exec.Cmd) { cmd = c }
	s.afterReport = func() { namespace = pidNamespace(cmd.Process.Pid) }
	done := make(chan struct{})
	var out Output
	var err error
	go func() { out, err = Run(ctx, s); close(done) }()
	select {
	case <-ready:
	case <-done:
		t.Fatalf("S-2 no process: N tree did not start: %+v %v", out, err)
	case <-time.After(10 * time.Second):
		cancel()
		<-done
		t.Fatal("S-2 no process: N tree start deadline")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = signalGroup(cmd.Process.Pid, unix.SIGKILL)
		<-done
		t.Fatal("S-2 no process: N tree cancellation deadline")
	}
	if !errors.Is(err, context.Canceled) || !out.Isolated || namespace == 0 || !waitGone(-namespace, time.Second) {
		t.Fatalf("S-2 no process: N descendants survived: namespace=%d out=%+v err=%v", namespace, out, err)
	}
}

func processTreeHelper() int {
	var children []*exec.Cmd
	defer func() {
		for _, c := range children {
			_ = c.Process.Kill()
			_ = c.Wait()
		}
	}()
	for range 8 {
		c := exec.Command(os.Args[0], "--sidecar-test=sleep")
		c.Env = []string{}
		if err := c.Start(); err != nil {
			fmt.Println("spawn:", err)
			return 3
		}
		children = append(children, c)
	}
	fmt.Println("children=8")
	time.Sleep(time.Hour)
	return 0
}
