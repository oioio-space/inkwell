package sidecar

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

func scopeManager(t *testing.T) *SandboxScope {
	t.Helper()
	m := &SandboxScope{}
	t.Cleanup(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.session != nil {
			s := m.session
			m.session = nil
			s.stale = true
			if s.refs == 0 {
				closeSession(s)
			}
		}
	})
	return m
}

func scopeSpec(t *testing.T, m *SandboxScope, mode string) Spec {
	s := deathSpec(t, "scope-"+mode)
	s.Dir = t.TempDir()
	s.scope = m
	s.MemoryLimit = 2 << 30
	s.ResidentLimit = 128 << 20
	return s
}

func requireScopedRun(t *testing.T, m *SandboxScope, mode string) Output {
	t.Helper()
	out, err := Run(t.Context(), scopeSpec(t, m, mode))
	if err != nil {
		t.Fatal(err)
	}
	if out.Containment != "cgroup" {
		t.Fatal("no delegated user scope or PID namespaces available")
	}
	if mode == "fds" || mode == "fds-tree" {
		if out.ExitCode != 0 || out.Signal != 0 {
			t.Fatalf("descriptor helper failed: exit=%d signal=%d stderr=%s", out.ExitCode, out.Signal, out.Stderr)
		}
	}
	return out
}

// requireScope is requireN for a tier-aware cgroup test (plan « Tier-aware
// suites »). On a machine in tier F it asserts that tier's promise instead: a
// real run given a scope manager creates no session and places no run cgroup
// (T-6: no cgroup without a PID namespace) and is bounded by its rlimits; it
// then returns false, as it does on a refused machine.
func requireScope(t *testing.T) bool {
	t.Helper()
	switch machineTier(t) {
	case tierNone:
		return false
	case tierN:
		return true
	}
	m := scopeManager(t)
	m.beforeStart = func(*cgroupRun) { t.Error("T-6 fallback: a run cgroup was placed without a PID namespace") }
	out, err := Run(t.Context(), scopeSpec(t, m, "fds"))
	if err != nil || out.ExitCode != 0 || out.Containment != "rlimit" || m.session != nil {
		t.Fatalf("T-6 fallback: tier F run with a scope manager: %+v %v, session %v; want a clean rlimit run and no session", out, err, m.session)
	}
	return false
}

func TestCgroupSessionScopeDelegated(t *testing.T) {
	if !requireScope(t) {
		return
	}
	m := scopeManager(t)
	requireScopedRun(t, m, "fds")
	s := m.session
	if s == nil || !holderAlive(s) {
		t.Fatal("no live holder")
	}
	v, err := readValue(filepath.Join(s.path, "holder/cgroup.procs"))
	if err != nil || v != strconv.Itoa(s.holder.Process.Pid) {
		t.Fatalf("holder membership: %q %v", v, err)
	}
	v, err = readValue(filepath.Join(s.path, "cgroup.procs"))
	if err != nil || v != "" {
		t.Fatalf("internal processes: %q %v", v, err)
	}
	v, err = readValue(filepath.Join(s.path, "cgroup.subtree_control"))
	if err != nil || !containsWord(v, "memory") {
		t.Fatalf("controller: %q %v", v, err)
	}
}

func TestCgroupRunLimitsVerifiedBeforeStart(t *testing.T) {
	if !requireScope(t) {
		return
	}
	m := scopeManager(t)
	var writes []string
	m.write = func(path, value string) error {
		writes = append(writes, filepath.Base(path))
		return writeValue(path, value)
	}
	called := false
	m.beforeStart = func(r *cgroupRun) {
		called = true
		if strings.Join(writes, ",") != "memory.max,memory.high,memory.swap.max,memory.oom.group" {
			t.Errorf("limit write ordering: %v", writes)
		}
		for file, want := range map[string]string{"memory.max": "201326592", "memory.high": "134217728", "memory.swap.max": "0", "memory.oom.group": "1"} {
			got, err := readValue(filepath.Join(r.path, file))
			if err != nil || got != want {
				t.Errorf("%s = %s %v want %s", file, got, err, want)
			}
		}
	}
	requireScopedRun(t, m, "fds")
	if !called {
		t.Fatal("no before-start verification")
	}
}

// lineSink forwards completed lines without blocking the os/exec copier.
type lineSink struct {
	mu    sync.Mutex
	b     bytes.Buffer
	lines chan string
}

func newLineSink() *lineSink { return &lineSink{lines: make(chan string, 64)} }
func (w *lineSink) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.b.Write(b)
	for {
		line, err := w.b.ReadString('\n')
		if err != nil {
			w.b.WriteString(line)
			break
		}
		select {
		case w.lines <- strings.TrimSpace(line):
		default:
		}
	}
	return len(b), nil
}

func waitLine(t *testing.T, w *lineSink, prefix string) string {
	t.Helper()
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case line := <-w.lines:
			if suffix, ok := strings.CutPrefix(line, prefix); ok {
				return suffix
			}
		case <-timer.C:
			t.Fatal("target did not report " + prefix)
			return ""
		}
	}
}

func TestCgroupWorkloadAndDescendantsInside(t *testing.T) {
	if !requireScope(t) {
		return
	}
	m := scopeManager(t)
	s := scopeSpec(t, m, "tree")
	input, writer := io.Pipe()
	defer func() { _ = input.Close() }()
	defer func() { _ = writer.Close() }()
	s.Stdin = input
	lines := newLineSink()
	s.Stdout = lines
	runPath := make(chan string, 1)
	m.beforeStart = func(r *cgroupRun) { runPath <- r.path }
	done := make(chan Output, 1)
	go func() {
		out, err := Run(t.Context(), s)
		if err != nil {
			t.Error(err)
		}
		done <- out
	}()
	var path string
	select {
	case path = <-runPath:
	case out := <-done:
		if out.Containment != "cgroup" {
			t.Fatal("scope unavailable")
		}
		t.Fatal("target ended before membership check")
	case line := <-lines.lines:
		// The target starts after beforeStart: a line with no run cgroup
		// announced is a run outside the scope. Its stdin is released first,
		// or Run would wait for it until the package's timeout.
		select {
		case path = <-runPath:
			lines.lines <- line
		default:
			_ = writer.Close()
			<-done
			t.Fatalf("scope unavailable: the target runs (%s) outside any run cgroup", line)
		}
	}
	pids := strings.Split(waitLine(t, lines, "tree="), ",")
	if len(pids) != 2 {
		t.Fatal(pids)
	}
	members, err := readValue(filepath.Join(path, "cgroup.procs"))
	if err != nil {
		t.Fatal(err)
	}
	app, _ := selfCgroup()
	for _, nspid := range pids {
		found := false
		for host := range strings.FieldsSeq(members) {
			b, err := os.ReadFile("/proc/" + host + "/status")
			if err != nil {
				continue
			}
			d := identityFromStatus(b)
			if len(d.NSPID) > 0 && strconv.Itoa(d.NSPID[len(d.NSPID)-1]) == nspid {
				b, err = os.ReadFile("/proc/" + host + "/cgroup")
				cg, err := unifiedCgroup(b, err)
				if err != nil || cg == app || filepath.Join("/sys/fs/cgroup", cg) != path {
					t.Errorf("workload outside run: %s %v", cg, err)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("namespace PID %s absent from %s", nspid, members)
		}
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	// This input belongs to the test: sending the release byte does not
	// terminate os/exec's copier. Close it before waiting for Run.
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if out := <-done; out.Containment != "cgroup" {
		t.Fatal(out.Containment)
	}
}

func TestBootstrapRefusesUnexpectedCgroup(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	called := false
	m.beforeStart = func(r *cgroupRun) { called = true }
	// Change the encoded expectation, not the actual placement.
	s := scopeSpec(t, m, "fds")
	s.bootstrapExpected = "/unexpected-cgroup"
	out, err := Run(t.Context(), s)
	if !called || out.Containment != "cgroup" {
		t.Fatal("scope unavailable")
	}
	if err == nil || !out.bootstrapRefused {
		t.Fatalf("unexpected membership accepted: %+v %v", out, err)
	}
}

func TestCgroupRemovedAfterRun(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	var path string
	m.beforeStart = func(r *cgroupRun) { path = r.path }
	requireScopedRun(t, m, "fds")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("run cgroup remains: %v", err)
	}
}

func TestCgroupFallbackAtEveryStep(t *testing.T) {
	if !requireN(t) {
		return
	}
	for i := 1; i <= 11; i++ {
		row := fmt.Sprintf("F%d", i)
		t.Run(row, func(t *testing.T) {
			m := scopeManager(t)
			hit := false
			m.fault = func(step string) error {
				if step == row && row != "F4" && row != "F9" {
					hit = true
					return unix.EACCES
				}
				return nil
			}
			if row == "F4" {
				m.afterJob = func(s *scopeSession) { hit = true; s.relative += "-not-the-holder" }
			}
			if row == "F9" {
				m.write = func(path, value string) error {
					if filepath.Base(path) == "memory.max" {
						hit = true
						return nil
					}
					return writeValue(path, value)
				}
			}
			s := scopeSpec(t, m, "fds")
			out, err := Run(t.Context(), s)
			if !hit {
				t.Fatal("scope failed before injected step")
			}
			if err != nil || out.ExitCode != 0 || out.Containment != "rlimit" {
				t.Fatalf("fallback: %+v %v", out, err)
			}
		})
	}
}

func TestFallbackReportsRlimitOnly(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	s := scopeSpec(t, scopeManager(t), "fds")
	s.disableScope = true
	out, err := Run(t.Context(), s)
	if err != nil || out.Containment != "rlimit" || out.MemoryCause != MemoryUnknown {
		t.Fatalf("fallback: %+v %v", out, err)
	}
}

func TestBothMemoryBudgetsHold(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	s := scopeSpec(t, m, "budget")
	m.beforeStart = func(r *cgroupRun) {
		v, err := readValue(filepath.Join(r.path, "memory.high"))
		if err != nil || v != "134217728" {
			t.Errorf("resident budget = %s %v", v, err)
		}
		if v, err = readValue(filepath.Join(r.path, "memory.max")); err != nil || v != "201326592" {
			t.Errorf("resident backstop = %s %v", v, err)
		}
	}
	out, err := Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Containment != "cgroup" {
		t.Fatal("scope unavailable")
	}
	if !strings.Contains(string(out.Stdout), "as=2147483648/2147483648") {
		t.Fatalf("address bound raised or removed: %q", out.Stdout)
	}
}

func TestResidentBudgetKillsWithSIGKILL(t *testing.T) {
	if !requireN(t) {
		return
	}
	for _, mode := range []string{"resident", "resident-tree"} {
		t.Run(mode, func(t *testing.T) {
			out, err := Run(t.Context(), residentSpec(t, scopeManager(t), mode))
			if err != nil {
				t.Fatal(err)
			}
			if out.Containment != "cgroup" {
				t.Fatal("no delegated user scope or PID namespaces available")
			}
			if out.Signal != 9 || out.MemoryCause != MemoryLocalLimit || out.CoreDump != CoreDumpNone {
				t.Fatalf("resident outcome: %+v", out)
			}
			if mode == "resident-tree" && strings.Count(string(out.Stdout), "resident-child-ready") < 2 {
				t.Fatalf("aggregate workload did not include two bounded descendants: %s", out.Stdout)
			}
		})
	}
}

func TestAddressBudgetFailsAllocationFirst(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	s := scopeSpec(t, m, "address")
	s.ResidentLimit = 3 << 30
	out, err := Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Containment != "cgroup" {
		t.Fatal("scope unavailable")
	}
	if out.Signal != 0 || out.ExitCode != 1 || !strings.Contains(string(out.Stdout), "allocation-failed") || out.MemoryCause != MemoryNone || out.CoreDump != CoreDumpNone {
		t.Fatalf("address outcome: %+v", out)
	}
}

func TestCgroupCountersReadBeforeCleanup(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	var events []string
	m.trace = func(s string) { events = append(events, s) }
	m.beforeStart = func(r *cgroupRun) {
		r.watch.beforeRead = func() {
			if _, err := readMemorySample(r.path); err != nil {
				t.Errorf("local counters unavailable at final read: %v", err)
			}
		}
	}
	requireScopedRun(t, m, "resident")
	if strings.Join(events, ",") != "counters,cleanup" {
		t.Fatal(events)
	}
}

func TestHolderDeathBetweenVerifyAndStart(t *testing.T) {
	if !requireN(t) {
		return
	}
	for _, row := range []string{"before-run", "after-verify"} {
		t.Run(row, func(t *testing.T) {
			m := scopeManager(t)
			requireScopedRun(t, m, "fds")
			old := m.session
			if row == "before-run" {
				_ = old.holder.Process.Kill()
				for holderAlive(old) {
					time.Sleep(time.Millisecond)
				}
				requireScopedRun(t, m, "fds")
				if m.session == old {
					t.Fatal("dead session reused")
				}
				return
			}
			m.beforeStart = func(r *cgroupRun) {
				_ = r.session.holder.Process.Kill()
				for holderAlive(r.session) {
					time.Sleep(time.Millisecond)
				}
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					if _, err := os.Stat(r.path); errors.Is(err, os.ErrNotExist) {
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
			out, err := Run(t.Context(), scopeSpec(t, m, "fds"))
			if err != nil || out.ExitCode != 0 || out.Containment != "rlimit" {
				t.Fatalf("placement not retried in fallback: %+v %v", out, err)
			}
		})
	}
}

func TestConcurrentRunsDuringHolderDeath(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	requireScopedRun(t, m, "fds")
	old := m.session
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			out, err := Run(t.Context(), scopeSpec(t, m, "fds"))
			if err != nil || out.ExitCode != 0 || (out.Containment != "rlimit" && out.Containment != "cgroup") {
				t.Errorf("unconfined run: %+v %v", out, err)
			}
		})
	}
	_ = old.holder.Process.Kill()
	wg.Wait()
}

func TestSandboxInheritsNoSessionDescriptor(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	m.beforeStart = func(r *cgroupRun) {
		for _, f := range r.command.ExtraFiles {
			if f == r.session.pipe {
				t.Error("holder write end passed to a launch")
			}
		}
	}
	out := requireScopedRun(t, m, "fds-tree")
	if strings.Count(string(out.Stdout), "fd=0 ") != 2 {
		t.Fatalf("target and descendant did not both report descriptors: %s", out.Stdout)
	}
	pipe, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", m.session.pipe.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(out.Stdout), "\n") {
		if strings.Contains(line, pipe) || strings.Contains(line, "/sys/fs/cgroup") {
			t.Fatal("session descriptor inherited: " + line)
		}
		var fd, clo int
		if _, err := fmt.Sscanf(line, "fd=%d cloexec=%d", &fd, &clo); err == nil && fd > 2 && clo == 0 {
			t.Fatal("extra inherited descriptor: " + line)
		}
	}
}

// Ignore only the directory fd that ReadDir itself has already closed.
func descriptorSnapshot(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	fds := make(map[string]string)
	for _, entry := range entries {
		target, err := os.Readlink("/proc/self/fd/" + entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		fds[entry.Name()] = target
	}
	return fds
}

func TestCgroupDescriptorClosedOnEveryPath(t *testing.T) {
	if !requireN(t) {
		return
	}
	for _, row := range []string{"success", "placement-error", "refused-bootstrap"} {
		t.Run(row, func(t *testing.T) {
			// Each path starts with its own live session. A placement failure
			// invalidates it; the next row must not count a replacement session
			// (holder pipe, holder pidfd and Cmd pidfd) as leaked run descriptors.
			m := scopeManager(t)
			requireScopedRun(t, m, "fds")
			before := descriptorSnapshot(t)
			var fd int
			m.beforeStart = func(r *cgroupRun) { fd = r.fd }
			m.afterStarted = func(r *cgroupRun) {
				if r.fd >= 0 {
					t.Error("directory descriptor not closed immediately after Start")
				}
			}
			s := scopeSpec(t, m, "fds")
			if row == "placement-error" {
				m.fault = func(row string) error {
					if row == "F10" {
						return unix.ENODEV
					}
					return nil
				}
			}
			if row == "refused-bootstrap" {
				s.bootstrapExpected = "/wrong"
			}
			_, _ = Run(t.Context(), s)
			m.fault = nil
			if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
				t.Fatalf("cgroup descriptor remains open: %d %v", fd, err)
			}
			after := descriptorSnapshot(t)
			if len(after) > len(before) {
				t.Fatalf("descriptors grew: %d -> %d", len(before), len(after))
			}
			for fd, target := range after {
				if before[fd] != target {
					t.Errorf("unexpected descriptor %s: %q (before %q)", fd, target, before[fd])
				}
			}
		})
	}
}

// appExit is the end of an app helper: err is its Wait error once done is closed.
type appExit struct {
	done chan struct{}
	err  error
}

func appHelper(t *testing.T, arg string) (*exec.Cmd, *lineSink, io.WriteCloser, *appExit) {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "--sidecar-test=scope-scope-app:"+arg)
	cmd.Env = append(os.Environ(), "INKWELL_TEST_PROJECT="+dir, "INKWELL_TEST_ADDRESS_RESERVE="+strconv.FormatUint(raceAddressReserve(t), 10))
	w := newLineSink()
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exit := &appExit{done: make(chan struct{})}
	go func() {
		exit.err = cmd.Wait()
		close(exit.done)
	}()
	t.Cleanup(func() { _ = input.Close(); _ = cmd.Process.Kill(); <-exit.done })
	return cmd, w, input, exit
}

func TestAppSIGKILLLeavesNoWorkload(t *testing.T) {
	if !requireN(t) {
		return
	}
	for _, holderDead := range []bool{false, true} {
		t.Run(fmt.Sprintf("holder-dead-%v", holderDead), func(t *testing.T) {
			cmd, lines, input, _ := appHelper(t, "")
			_, _ = input.Write([]byte("x"))
			info := waitLine(t, lines, "scope=")
			scope, holder, _ := strings.Cut(info, " holder=")
			holderPID, err := strconv.Atoi(holder)
			if err != nil {
				t.Fatal(err)
			}
			pidfd, err := unix.PidfdOpen(holderPID, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = unix.Close(pidfd) }()
			defer func() { _ = unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0) }()
			_ = waitLine(t, lines, "tree=")
			if holderDead {
				if err = unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil {
					t.Fatal(err)
				}
			}
			_ = cmd.Process.Kill()
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(scope); errors.Is(err, os.ErrNotExist) {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("scope survived app SIGKILL: " + scope)
		})
	}
}

// testUnit uses the real manager, and owns every uniquely named unit it creates.
func testUnit(t *testing.T, name string, props []scopeProperty) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1").Call("org.freedesktop.systemd1.Manager.StopUnit", 0, name, "replace").Err
		_ = conn.Close()
	})
	manager := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	if err = conn.AddMatchSignalContext(ctx, dbus.WithMatchInterface("org.freedesktop.systemd1.Manager"), dbus.WithMatchMember("JobRemoved")); err != nil {
		t.Fatal(err)
	}
	ch := make(chan *dbus.Signal, 32)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	if err = manager.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.Subscribe", 0).Err; err != nil {
		t.Fatal(err)
	}
	var job dbus.ObjectPath
	if err = manager.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.StartTransientUnit", 0, name, "fail", props, []struct {
		Name       string
		Properties []scopeProperty
	}{}).Store(&job); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case s := <-ch:
			if s != nil && len(s.Body) == 4 && s.Body[1] == job {
				if s.Body[3] != "done" {
					t.Fatal(s.Body)
				}
				return
			}
		}
	}
}

// boundApp plays the scenario until its app helper reports. The helper shares
// the test's bounded slice, so the kernel can OOM-kill it too, right after the
// run and before the report (residual R2c of gates/SB2a1/SIMPLIFY.md): such an
// attempt says nothing about the rule and is played again, three times at most.
// Every other outcome is judged by the attempt itself.
func boundApp(t *testing.T, leaf bool) {
	t.Helper()
	const attempts = 3
	for attempt := 1; !boundAttempt(t, leaf); attempt++ {
		if attempt == attempts {
			t.Fatalf("the kernel OOM-killed the app helper before its report in each of %d attempts: the scenario never ran to its end", attempts)
		}
		t.Logf("attempt %d: the kernel OOM-killed the app helper before its report; scenario played again", attempt)
	}
}

// boundAttempt reports false only when the kernel OOM-killed the app helper
// itself before it reported the run.
func boundAttempt(t *testing.T, leaf bool) bool {
	t.Helper()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(nonce[:])
	slice := "app-inkwelltest-" + suffix + ".slice"
	props := []scopeProperty{scopeProp("MemoryMax", uint64(192<<20)), scopeProp("MemorySwapMax", uint64(0))}
	if leaf {
		props[0] = scopeProp("MemoryMax", uint64(1<<30))
	}
	testUnit(t, slice, props)
	arg := "aggregate"
	if leaf {
		arg = "leaf"
	}
	cmd, lines, input, exit := appHelper(t, arg)
	// An idle process keeps the app's scope, and so its OOM counters, readable
	// after the app helper's death; OOMPolicy=continue keeps the manager from
	// stopping that scope on an OOM kill (its default for a scope without Delegate).
	keeper := exec.Command("sleep", "infinity")
	if err := keeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = keeper.Process.Kill(); _ = keeper.Wait() })
	appScope := "inkwell-test-app-" + suffix + ".scope"
	appProps := []scopeProperty{scopeProp("PIDs", []uint32{uint32(cmd.Process.Pid), uint32(keeper.Process.Pid)}), scopeProp("Slice", slice), scopeProp("CollectMode", "inactive-or-failed"), scopeProp("OOMPolicy", "continue"), scopeProp("MemorySwapMax", uint64(0))}
	if leaf {
		appProps = append(appProps, scopeProp("MemoryMax", uint64(128<<20)))
	}
	testUnit(t, appScope, appProps)
	_, _ = input.Write([]byte("x"))
	info := waitLine(t, lines, "scope=")
	scope, _, _ := strings.Cut(info, " holder=")
	ownApp, _ := selfCgroup()
	if filepath.Base(filepath.Dir(scope)) != slice || strings.Contains(ownApp, slice) {
		t.Fatal("workload did not share the child app's limited slice")
	}
	if leaf {
		for file, want := range map[string]string{"memory.max": "134217728", "memory.swap.max": "0"} {
			got, err := readValue(filepath.Join(scope, file))
			if err != nil || got != want {
				t.Fatalf("copied leaf %s=%s %v", file, got, err)
			}
		}
	}
	_ = waitLine(t, lines, "memory-ready")
	// Select the growing workload as the ancestor OOM victim, preserving the
	// harness's existing allocation while the kernel enforces their sum.
	runs, err := os.ReadDir(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if !strings.HasPrefix(run.Name(), "run-") {
			continue
		}
		members, err := readValue(filepath.Join(scope, run.Name(), "cgroup.procs"))
		if err != nil {
			t.Fatal(err)
		}
		for pid := range strings.FieldsSeq(members) {
			if err = writeValue("/proc/"+pid+"/oom_score_adj", "1000"); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, _ = input.Write([]byte("x"))
	result, reported := waitReport(t, lines, exit)
	if !reported {
		events, err := readCounters(filepath.Join(filepath.Dir(scope), appScope, "memory.events.local"))
		killed := false
		if ended, ok := errors.AsType[*exec.ExitError](exit.err); ok {
			status, ok := ended.Sys().(syscall.WaitStatus)
			killed = ok && status.Signaled() && status.Signal() == syscall.SIGKILL
		}
		if !killed || err != nil || events["oom_kill"] == 0 {
			t.Fatalf("app helper ended without a report and not by the kernel's OOM kill: %v, app scope counters %v %v", exit.err, events, err)
		}
		return false
	}
	// The shared slice bounds the app too, and the kernel charges each inotify
	// event to the watcher's memory cgroup: at that bound the allocation of the
	// kill's event can fail, which inotify reports as a queue overflow (kernel
	// source fs/notify/inotify/inotify_fsnotify.c, cited from memory; measured
	// here as IN_Q_OVERFLOW after 3 or 4 samples, R2b of gates/SB2a1/SIMPLIFY.md).
	// The leaf bound leaves the app outside: no event is lost.
	// ADR 0042 rule M9: a lost watch answers "unknown", never a guessed cause.
	lostEvent := !leaf && result == "cgroup cause=unknown signal=9 err=<nil> watch-lost=true"
	if result != "cgroup cause=victim signal=9 err=<nil> watch-lost=false" && !lostEvent {
		t.Fatal("ancestor bound did not kill the sandbox: " + result)
	}
	ancestor := filepath.Dir(scope)
	if leaf {
		ancestor = scope
	}
	// The session scope may be collected after its holder exits (RS6); the
	// shared slice is kept alive by this test and its local OOM must be recorded.
	if !leaf {
		events, err := readCounters(filepath.Join(ancestor, "memory.events.local"))
		if err != nil || events["oom"] == 0 {
			t.Fatalf("shared aggregate bound never fired: %v %v", events, err)
		}
	}
	return true
}

// waitReport returns the app helper's "out=" line, or false when the helper
// ended without writing one.
func waitReport(t *testing.T, w *lineSink, exit *appExit) (string, bool) {
	t.Helper()
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case line := <-w.lines:
			if report, ok := strings.CutPrefix(line, "out="); ok {
				return report, true
			}
		case <-exit.done:
			// Wait has returned: every line the helper wrote is already queued.
			for {
				select {
				case line := <-w.lines:
					if report, ok := strings.CutPrefix(line, "out="); ok {
						return report, true
					}
				default:
					return "", false
				}
			}
		case <-timer.C:
			t.Fatal("target did not report out=")
		}
	}
}

func TestCgroupSessionSharesSliceBound(t *testing.T) {
	if requireN(t) {
		boundApp(t, false)
	}
}

func TestCgroupSessionCopiesLeafBound(t *testing.T) {
	if requireN(t) {
		boundApp(t, true)
	}
}

func TestCgroupTargetExit124(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	s := scopeSpec(t, m, "fds")
	s.Args = []string{"--sidecar-test=exit:124"}
	out, err := Run(t.Context(), s)
	if err != nil || out.Containment != "cgroup" || out.ExitCode != 124 || out.bootstrapRefused {
		t.Fatalf("target exit confused with refusal: %+v %v", out, err)
	}
}

func TestCgroupStartErrorPreservesSession(t *testing.T) {
	if !requireN(t) {
		return
	}
	for _, startErr := range []error{unix.ENOENT, unix.ENOEXEC, unix.EAGAIN} {
		t.Run(startErr.Error(), func(t *testing.T) {
			m := scopeManager(t)
			requireScopedRun(t, m, "fds")
			session := m.session
			m.fault = func(step string) error {
				if step == "F10" {
					return startErr
				}
				return nil
			}
			_, err := Run(t.Context(), scopeSpec(t, m, "fds"))
			if !errors.Is(err, startErr) || m.session != session || session.stale {
				t.Fatalf("healthy session invalidated by executable/start error: %v", err)
			}
		})
	}
}
