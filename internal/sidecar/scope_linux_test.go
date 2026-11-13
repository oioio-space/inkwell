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

func TestCgroupSessionScopeDelegated(t *testing.T) {
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
	m := scopeManager(t)
	var writes []string
	m.write = func(path, value string) error {
		writes = append(writes, filepath.Base(path))
		return writeValue(path, value)
	}
	called := false
	m.beforeStart = func(r *cgroupRun) {
		called = true
		if strings.Join(writes, ",") != "memory.max,memory.swap.max,memory.oom.group" {
			t.Errorf("limit write ordering: %v", writes)
		}
		for file, want := range map[string]string{"memory.max": "134217728", "memory.swap.max": "0", "memory.oom.group": "1"} {
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
	m := scopeManager(t)
	var path string
	m.beforeStart = func(r *cgroupRun) { path = r.path }
	requireScopedRun(t, m, "fds")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("run cgroup remains: %v", err)
	}
}

func TestCgroupFallbackAtEveryStep(t *testing.T) {
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
	s := scopeSpec(t, scopeManager(t), "fds")
	s.disableScope = true
	out, err := Run(t.Context(), s)
	if err != nil || out.Containment != "rlimit" || out.MemoryCause != MemoryUnknown {
		t.Fatalf("fallback: %+v %v", out, err)
	}
}

func TestBothMemoryBudgetsHold(t *testing.T) {
	m := scopeManager(t)
	s := scopeSpec(t, m, "budget")
	m.beforeStart = func(r *cgroupRun) {
		v, err := readValue(filepath.Join(r.path, "memory.max"))
		if err != nil || v != "134217728" {
			t.Errorf("resident budget = %s %v", v, err)
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
	for _, mode := range []string{"resident", "resident-tree"} {
		t.Run(mode, func(t *testing.T) {
			out := requireScopedRun(t, scopeManager(t), mode)
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

func appHelper(t *testing.T, arg string) (*exec.Cmd, *lineSink, io.WriteCloser) {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "--sidecar-test=scope-scope-app:"+arg)
	cmd.Env = append(os.Environ(), "INKWELL_TEST_PROJECT="+dir)
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
	t.Cleanup(func() { _ = input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd, w, input
}

func TestAppSIGKILLLeavesNoWorkload(t *testing.T) {
	for _, holderDead := range []bool{false, true} {
		t.Run(fmt.Sprintf("holder-dead-%v", holderDead), func(t *testing.T) {
			cmd, lines, input := appHelper(t, "")
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

func boundApp(t *testing.T, leaf bool) {
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
	cmd, lines, input := appHelper(t, arg)
	appProps := []scopeProperty{scopeProp("PIDs", []uint32{uint32(cmd.Process.Pid)}), scopeProp("Slice", slice), scopeProp("CollectMode", "inactive-or-failed"), scopeProp("MemorySwapMax", uint64(0))}
	if leaf {
		appProps = append(appProps, scopeProp("MemoryMax", uint64(128<<20)))
	}
	testUnit(t, "inkwell-test-app-"+suffix+".scope", appProps)
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
	result := waitLine(t, lines, "out=")
	if !strings.Contains(result, "cgroup cause=victim signal=9 err=<nil>") {
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
}
func TestCgroupSessionSharesSliceBound(t *testing.T) { boundApp(t, false) }
func TestCgroupSessionCopiesLeafBound(t *testing.T)  { boundApp(t, true) }

func TestCgroupTargetExit124(t *testing.T) {
	m := scopeManager(t)
	s := scopeSpec(t, m, "fds")
	s.Args = []string{"--sidecar-test=exit:124"}
	out, err := Run(t.Context(), s)
	if err != nil || out.Containment != "cgroup" || out.ExitCode != 124 || out.bootstrapRefused {
		t.Fatalf("target exit confused with refusal: %+v %v", out, err)
	}
}

func TestCgroupStartErrorPreservesSession(t *testing.T) {
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
