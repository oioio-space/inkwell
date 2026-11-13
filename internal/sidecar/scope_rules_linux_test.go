package sidecar

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

// These tests prove rules of ADR 0042 and of the tables of
// docs/plans/2026-11-11-sandbox-crash-reports.md (S, R, F, H, A rows) on
// ordinary directories and channels: no user manager, no cgroupfs. The rows
// that need the real kernel and systemd are in scope_linux_test.go.

// R6 and S5: membership is the canonical cgroup v2 path of the "0::" record,
// so a legacy hierarchy or a non-canonical path never equals an expected path.
func TestCgroupMembershipIsCanonicalUnifiedPath(t *testing.T) {
	for _, tc := range []struct{ name, file, want string }{
		{"unified after legacy records", "12:memory:/legacy\n0::/user.slice/app.scope\n", "/user.slice/app.scope"},
		{"legacy hierarchies only", "1:name=systemd:/user.slice/app.scope\n", ""},
		{"relative path", "0::user.slice/app.scope\n", ""},
		{"path with dot-dot", "0::/user.slice/../system.slice\n", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unifiedCgroup([]byte(tc.file), nil)
			if got != tc.want || (err == nil) != (tc.want != "") {
				t.Fatalf("unifiedCgroup = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := unifiedCgroup([]byte("0::/ignored\n"), os.ErrPermission); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("an unreadable /proc/<pid>/cgroup gave %v, want the read error", err)
	}
}

// S5 and F5: the memory controller must itself be delegated; a name that only
// contains "memory" is not it.
func TestMemoryControllerMustBeListedExactly(t *testing.T) {
	for list, want := range map[string]bool{
		"cpuset cpu io memory pids": true,
		"memory":                    true,
		"cpu io pids":               false,
		"cpu hugetlb_memory pids":   false,
		"":                          false,
	} {
		if got := containsWord(list, "memory"); got != want {
			t.Errorf("containsWord(%q) = %v, want %v", list, got, want)
		}
	}
	if containsWord("cpuset io memory", "cpu") || !containsWord("cpuset cpu io", "cpu") {
		t.Error("a controller name was matched as a prefix of another one")
	}
}

func runLimits(t *testing.T, dir string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, name := range []string{"memory.max", "memory.swap.max", "memory.oom.group"} {
		if v, err := readValue(filepath.Join(dir, name)); err == nil {
			got[name] = v
		}
	}
	return got
}

// R2 and R3: memory.max is the resident budget rounded down to the page size
// (the address budget when no resident budget is given), swap is 0 and the
// whole run is one OOM group.
func TestRunLimitsRoundedDownToPageSize(t *testing.T) {
	page := uint64(os.Getpagesize())
	for _, tc := range []struct {
		name             string
		resident, memory uint64
		want             uint64
	}{
		{"resident budget between two pages", 3*page + 123, 1 << 30, 3 * page},
		{"address budget when there is no resident budget", 0, 2*page + 1, 2 * page},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &cgroupRun{path: filepath.Join(t.TempDir(), "run-1")}
			if err := (&SandboxScope{}).configureRunLimits(r, Spec{ResidentLimit: tc.resident, MemoryLimit: tc.memory}); err != nil {
				t.Fatal(err)
			}
			got := runLimits(t, r.path)
			if got["memory.max"] != strconv.FormatUint(tc.want, 10) || got["memory.swap.max"] != "0" || got["memory.oom.group"] != "1" {
				t.Fatalf("limits = %v, want memory.max %d, swap 0, oom.group 1", got, tc.want)
			}
		})
	}
}

// R2, F8 and F9: a run whose limits cannot be set and read back is refused
// before Start, never launched with a missing or zero limit.
func TestRunLimitsRefusedUnlessVerified(t *testing.T) {
	page := uint64(os.Getpagesize())
	t.Run("budget below one page writes no limit", func(t *testing.T) {
		r := &cgroupRun{path: filepath.Join(t.TempDir(), "run-1")}
		if err := (&SandboxScope{}).configureRunLimits(r, Spec{ResidentLimit: page - 1}); err == nil {
			t.Fatal("a budget that rounds down to zero was accepted")
		}
		if got := runLimits(t, r.path); len(got) != 0 {
			t.Fatalf("limits written for a refused budget: %v", got)
		}
	})
	t.Run("F8 run directory cannot be made", func(t *testing.T) {
		r := &cgroupRun{path: filepath.Join(t.TempDir(), "missing-scope", "run-1")}
		if err := (&SandboxScope{}).configureRunLimits(r, Spec{ResidentLimit: page}); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("configureRunLimits = %v, want the mkdir error", err)
		}
	})
	t.Run("F8 limit write refused", func(t *testing.T) {
		m := &SandboxScope{write: func(path, value string) error {
			if filepath.Base(path) == "memory.swap.max" {
				return unix.EACCES
			}
			return writeValue(path, value)
		}}
		r := &cgroupRun{path: filepath.Join(t.TempDir(), "run-1")}
		if err := m.configureRunLimits(r, Spec{ResidentLimit: page}); !errors.Is(err, unix.EACCES) {
			t.Fatalf("configureRunLimits = %v, want EACCES", err)
		}
	})
	for _, ignored := range []string{"memory.max", "memory.swap.max", "memory.oom.group"} {
		t.Run("F9 write of "+ignored+" silently ignored", func(t *testing.T) {
			m := &SandboxScope{write: func(path, value string) error {
				if filepath.Base(path) == ignored {
					return writeValue(path, "max")
				}
				return writeValue(path, value)
			}}
			r := &cgroupRun{path: filepath.Join(t.TempDir(), "run-1")}
			if err := m.configureRunLimits(r, Spec{ResidentLimit: page}); err == nil {
				t.Fatalf("a %s that reads back as max was accepted", ignored)
			}
		})
	}
	t.Run("F9 fault before the read-back", func(t *testing.T) {
		m := &SandboxScope{fault: func(step string) error {
			if step == "F9" {
				return unix.EIO
			}
			return nil
		}}
		r := &cgroupRun{path: filepath.Join(t.TempDir(), "run-1")}
		if err := m.configureRunLimits(r, Spec{ResidentLimit: page}); !errors.Is(err, unix.EIO) {
			t.Fatalf("configureRunLimits = %v, want EIO", err)
		}
	})
}

// countingHandler counts log records and keeps the attributes of the last one.
type countingHandler struct {
	records int
	attrs   map[string]string
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *countingHandler) WithGroup(string) slog.Handler            { return h }
func (h *countingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records++
	h.attrs = map[string]string{}
	r.Attrs(func(a slog.Attr) bool { h.attrs[a.Key] = a.Value.String(); return true })
	return nil
}

// Plan § "Fallback": a setup failure is retried after a backoff of 1 s
// doubling to 30 s, not at every launch, and the app logs the reason once,
// as a fixed word (never the error text, which can hold a path).
func TestScopeFallbackBackoffAndSingleLog(t *testing.T) {
	logs := &countingHandler{}
	previous := slog.Default()
	slog.SetDefault(slog.New(logs))
	t.Cleanup(func() { slog.SetDefault(previous) })

	m := &SandboxScope{}
	for i, want := range []time.Duration{1, 2, 4, 8, 16, 30, 30} {
		want *= time.Second
		before := time.Now()
		m.fallback(&os.PathError{Op: "mkdir", Path: "/sys/fs/cgroup/private-name", Err: unix.EACCES})
		if m.backoff != want {
			t.Fatalf("failure %d: backoff %v, want %v", i+1, m.backoff, want)
		}
		if m.retryAt.Before(before.Add(want)) || m.retryAt.After(time.Now().Add(want)) {
			t.Fatalf("failure %d: retry at %v, want %v after the failure", i+1, m.retryAt, want)
		}
	}
	if logs.records != 1 || logs.attrs["reason"] != "permission" {
		t.Fatalf("%d log records, attributes %v; want one record with reason=permission", logs.records, logs.attrs)
	}
	for key, value := range logs.attrs {
		if strings.Contains(value, "private-name") {
			t.Fatalf("log attribute %s leaks the path: %s", key, value)
		}
	}

	// During the backoff a launch goes straight to the fallback: no setup step runs.
	steps := 0
	m.fault = func(string) error { steps++; return nil }
	cmd := exec.Command("unused")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWPID}
	if r := m.prepare(t.Context(), Spec{MemoryLimit: 1 << 30}, cmd); r != nil || steps != 0 || m.session != nil {
		t.Fatalf("launch during the backoff: run %v, %d setup steps, session %v", r, steps, m.session)
	}
	if cmd.SysProcAttr.UseCgroupFD {
		t.Fatal("fallback launch still asks for a cgroup placement")
	}
}

// H7: a launch path without its own PID namespace uses the fallback; and a run
// without a memory budget, or with the scope disabled, asks for no scope at all.
func TestScopeRequiresPIDNamespace(t *testing.T) {
	for _, attr := range []*syscall.SysProcAttr{nil, {}, {Cloneflags: unix.CLONE_NEWUSER | unix.CLONE_NEWNS}} {
		// One manager per case: a recorded failure would hide the next one behind its backoff.
		steps := 0
		m := scopeManager(t)
		m.fault = func(string) error { steps++; return nil }
		cmd := exec.Command("unused")
		cmd.SysProcAttr = attr
		if r := m.prepare(t.Context(), Spec{MemoryLimit: 1 << 30}, cmd); r != nil {
			r.finish(0)
			t.Fatalf("cgroup run prepared without a PID namespace: %+v", attr)
		}
		if steps != 0 || m.session != nil || m.backoff == 0 {
			t.Fatalf("no PID namespace (%+v): %d setup steps, session %v, backoff %v; want the recorded fallback only", attr, steps, m.session, m.backoff)
		}
	}

	steps := 0
	m := scopeManager(t)
	m.fault = func(string) error { steps++; return nil }
	cmd := exec.Command("unused")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWPID}
	for _, spec := range []Spec{{}, {MemoryLimit: 1 << 30, disableScope: true}} {
		if r := m.prepare(t.Context(), spec, cmd); r != nil || steps != 0 || m.backoff != 0 {
			t.Fatalf("spec %+v: run %v, %d setup steps, backoff %v; want no scope and no failure recorded", spec, r, steps, m.backoff)
		}
	}
}

func jobRemoved(job dbus.ObjectPath, result string) *dbus.Signal {
	return &dbus.Signal{
		Name: "org.freedesktop.systemd1.Manager.JobRemoved",
		Body: []any{uint32(7), job, "inkwell-sandbox-1-00.scope", result},
	}
}

// S4 and F3: the scope exists only once JobRemoved reports "done" for our own
// job; any other result, a lost manager or the 5 s bound is a setup failure.
func TestScopeJobMustBeDone(t *testing.T) {
	const job = dbus.ObjectPath("/org/freedesktop/systemd1/job/42")
	unrelated := []*dbus.Signal{
		nil,
		jobRemoved("/org/freedesktop/systemd1/job/41", "done"),
		{Name: "org.freedesktop.systemd1.Manager.UnitNew", Body: []any{uint32(7), job, "x", "done"}},
		{Name: "org.freedesktop.systemd1.Manager.JobRemoved", Body: []any{uint32(7), job}},
	}
	for _, tc := range []struct {
		result string
		ok     bool
	}{{"done", true}, {"failed", false}, {"timeout", false}, {"canceled", false}, {"dependency", false}} {
		t.Run(tc.result, func(t *testing.T) {
			ch := make(chan *dbus.Signal, len(unrelated)+1)
			for _, s := range unrelated {
				ch <- s
			}
			ch <- jobRemoved(job, tc.result)
			if err := waitJob(t.Context(), ch, job); (err == nil) != tc.ok {
				t.Fatalf("waitJob after result %q = %v", tc.result, err)
			}
		})
	}
	t.Run("manager disconnected", func(t *testing.T) {
		ch := make(chan *dbus.Signal)
		close(ch)
		if err := waitJob(t.Context(), ch, job); err == nil {
			t.Fatal("a closed signal stream was taken for a finished job")
		}
	})
	t.Run("no JobRemoved within the bound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		ch := make(chan *dbus.Signal, 1)
		ch <- jobRemoved("/org/freedesktop/systemd1/job/41", "done")
		if err := waitJob(ctx, ch, job); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waitJob = %v, want the deadline", err)
		}
	})
}

// A2 and A3: a finite memory bound of the app's own cgroup is copied to the
// session scope; "max" adds nothing; an unreadable or unparsable bound refuses
// the scope instead of starting it unbounded.
func TestScopeCopiesFiniteAppBounds(t *testing.T) {
	appDir := func(t *testing.T, memMax, swap string) string {
		t.Helper()
		dir := t.TempDir()
		for name, value := range map[string]string{"memory.max": memMax, "memory.swap.max": swap} {
			if value == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		// inheritedScopeProperties takes a path relative to the cgroup root.
		rel, err := filepath.Rel("/sys/fs/cgroup", dir)
		if err != nil {
			t.Fatal(err)
		}
		return rel
	}
	base := []scopeProperty{scopeProp("Delegate", true)}
	for _, tc := range []struct {
		name, max, swap string
		want            map[string]uint64
		refused         bool
	}{
		{"both finite", "1073741824", "0", map[string]uint64{"MemoryMax": 1 << 30, "MemorySwapMax": 0}, false},
		{"only memory.max finite", "4096", "max", map[string]uint64{"MemoryMax": 4096}, false},
		{"unbounded", "max", "max", map[string]uint64{}, false},
		{"unparsable bound", "12 MiB", "max", nil, true},
		{"negative bound", "max", "-1", nil, true},
		{"unreadable swap bound", "max", "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			props, err := inheritedScopeProperties(appDir(t, tc.max, tc.swap), base)
			if tc.refused {
				if err == nil {
					t.Fatalf("scope properties built from an unusable bound: %v", props)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]uint64{}
			for _, p := range props[1:] {
				got[p.Name] = p.Value.Value().(uint64)
			}
			if props[0].Name != "Delegate" || len(got) != len(tc.want) {
				t.Fatalf("properties %v, want Delegate and %v", props, tc.want)
			}
			for name, want := range tc.want {
				if v, ok := got[name]; !ok || v != want {
					t.Fatalf("properties %v, want %s=%d", props, name, want)
				}
			}
		})
	}
}

// unusedPID is a PID no process can have: PIDs stay below kernel.pid_max.
func unusedPID(t *testing.T) int {
	t.Helper()
	v, err := readValue("/proc/sys/kernel/pid_max")
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Kill(pid, 0); !errors.Is(err, unix.ESRCH) {
		t.Fatalf("kill(pid_max, 0) = %v, want ESRCH", err)
	}
	return pid
}

// fakeRun makes a run directory with the files removeRunCgroup uses and
// returns the path of its cgroup.kill.
func fakeRun(t *testing.T, dir, populated string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"cgroup.kill": "", "cgroup.events": "populated " + populated + "\nfrozen 0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "cgroup.kill")
}

// H9: at the next start, the run cgroups of a session whose app and holder
// are both gone are killed; a live app's session, a session whose holder is
// still there and directories that are not ours are left alone.
func TestStaleRunsSweptOnlyForDeadSessions(t *testing.T) {
	slice := t.TempDir()
	dead := strconv.Itoa(unusedPID(t))
	holder := func(scope, procs string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(slice, scope, "holder"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(slice, scope, "holder/cgroup.procs"), []byte(procs), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stale := "inkwell-sandbox-" + dead + "-0a0b.scope"
	holder(stale, "")
	swept := []string{fakeRun(t, filepath.Join(slice, stale, "run-1"), "0"), fakeRun(t, filepath.Join(slice, stale, "run-2"), "0")}
	kept := []string{fakeRun(t, filepath.Join(slice, stale, "holder"), "0")}
	if err := os.WriteFile(filepath.Join(slice, stale, "run-3"), nil, 0o600); err != nil { // a file, not a run cgroup
		t.Fatal(err)
	}

	live := "inkwell-sandbox-" + strconv.Itoa(os.Getpid()) + "-0a0b.scope"
	holder(live, "")
	heldByHolder := "inkwell-sandbox-" + dead + "-0c0d.scope"
	holder(heldByHolder, "4242\n")
	noHolder := "inkwell-sandbox-" + dead + "-0e0f.scope"
	otherName := "inkwell-sandbox-" + dead + "-0a0b-extra.scope"
	holder(otherName, "")
	notAPID := "inkwell-sandbox-self-0a0b.scope"
	holder(notAPID, "")
	foreign := "app-" + dead + "-0a0b.scope"
	holder(foreign, "")
	for _, scope := range []string{live, heldByHolder, noHolder, otherName, notAPID, foreign} {
		kept = append(kept, fakeRun(t, filepath.Join(slice, scope, "run-1"), "0"))
	}
	if err := os.WriteFile(filepath.Join(slice, "inkwell-sandbox-"+dead+"-file.scope"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	sweepStaleScopes(slice)
	sweepStaleScopes(filepath.Join(slice, "missing.slice")) // nothing to sweep is not an error

	for _, path := range swept {
		if b, err := os.ReadFile(path); err != nil || string(b) != "1" {
			t.Errorf("stale run not killed: %s = %q, %v", path, b, err)
		}
	}
	for _, path := range kept {
		if b, err := os.ReadFile(path); err != nil || len(b) != 0 {
			t.Errorf("swept a cgroup that is not a stale run: %s = %q, %v", path, b, err)
		}
	}
}

// R8: cleanup kills first, then waits for "populated 0" for a bounded time
// only, and reports a cgroup it could not empty or read instead of hanging.
func TestRunCgroupCleanupIsBounded(t *testing.T) {
	t.Run("still populated at the deadline", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "run-1")
		kill := fakeRun(t, dir, "1")
		start := time.Now()
		err := removeRunCgroup(dir, start.Add(60*time.Millisecond))
		if waited := time.Since(start); err == nil || errors.Is(err, unix.ENOTEMPTY) || waited < 60*time.Millisecond || waited > 5*time.Second {
			t.Fatalf("removeRunCgroup = %v after %v, want the populated error at the deadline, without an rmdir", err, waited)
		}
		if b, _ := os.ReadFile(kill); string(b) != "1" {
			t.Fatalf("cgroup.kill = %q: the wait started without a kill", b)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("a populated cgroup was removed: %v", err)
		}
	})
	t.Run("deadline already passed", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "run-1")
		kill := fakeRun(t, dir, "0")
		if err := removeRunCgroup(dir, time.Now().Add(-time.Second)); err == nil {
			t.Fatal("cleanup past its deadline reported success")
		}
		if b, _ := os.ReadFile(kill); len(b) != 0 {
			t.Fatalf("cgroup.kill = %q after the deadline", b)
		}
	})
	t.Run("unreadable cgroup.events", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "run-1")
		fakeRun(t, dir, "0")
		if err := os.Remove(filepath.Join(dir, "cgroup.events")); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		if err := removeRunCgroup(dir, start.Add(5*time.Second)); !errors.Is(err, os.ErrNotExist) || time.Since(start) > 2*time.Second {
			t.Fatalf("removeRunCgroup = %v after %v, want the read error at once", err, time.Since(start))
		}
	})
	t.Run("run directory already gone", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "run-1")
		if err := removeRunCgroup(dir, time.Now().Add(time.Second)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removeRunCgroup = %v, want not-exist", err)
		}
		if err := checkCgroupKill(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("checkCgroupKill = %v, want not-exist", err)
		}
	})
}

// R8 on a kernel without cgroup.kill (before 5.14): the procs sweep signals
// every listed PID, tolerates one that already exited, and stops on a record
// that is not a PID or on a kill it is not allowed to make. With cgroup.kill,
// no PID is signalled one by one.
func TestProcsSweepWithoutCgroupKill(t *testing.T) {
	procs := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	t.Run("exited PID tolerated", func(t *testing.T) {
		var killed []int
		err := killRunCgroup(procs(t, "11\n12\n13\n"), func(pid int) error {
			killed = append(killed, pid)
			if pid == 12 {
				return unix.ESRCH
			}
			return nil
		})
		if err != nil || len(killed) != 3 {
			t.Fatalf("killRunCgroup = %v after %v, want all three PIDs signalled", err, killed)
		}
	})
	for _, bad := range []string{"abc", "0", "-7", "1.5"} {
		t.Run("record "+bad+" refused", func(t *testing.T) {
			var killed []int
			err := killRunCgroup(procs(t, "11\n"+bad+"\n13\n"), func(pid int) error { killed = append(killed, pid); return nil })
			if err == nil || len(killed) != 1 || killed[0] != 11 {
				t.Fatalf("killRunCgroup = %v after %v, want an error after PID 11 only", err, killed)
			}
		})
	}
	t.Run("refused kill reported", func(t *testing.T) {
		err := killRunCgroup(procs(t, "11\n12\n"), func(int) error { return unix.EPERM })
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("killRunCgroup = %v, want EPERM", err)
		}
	})
	t.Run("cgroup.kill preferred", func(t *testing.T) {
		dir := procs(t, "11\n12\n")
		if err := os.WriteFile(filepath.Join(dir, "cgroup.kill"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		err := killRunCgroup(dir, func(pid int) error { t.Errorf("PID %d signalled although cgroup.kill exists", pid); return nil })
		if b, _ := os.ReadFile(filepath.Join(dir, "cgroup.kill")); err != nil || string(b) != "1" {
			t.Fatalf("killRunCgroup = %v, cgroup.kill = %q", err, b)
		}
	})
}

// R8 and H5: the procs sweep signals a PID only while it is a member of the
// run cgroup being removed (identity pinned by a pidfd), so a PID that left
// the cgroup, or was recycled, is never killed in its place.
func TestProcsSweepKillsOnlyRunMembers(t *testing.T) {
	cmd := helperCmd("sleep")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	relative, err := pidCgroup(pid)
	if err != nil {
		t.Fatal(err)
	}
	own := filepath.Join("/sys/fs/cgroup", relative)

	if err := killCgroupPID(filepath.Join(own, "run-1"), pid); err != nil {
		t.Fatalf("PID outside the run cgroup: %v", err)
	}
	if err := killCgroupPID(filepath.Dir(own), pid); err != nil {
		t.Fatalf("PID below the given cgroup: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if !alive(pid) {
		t.Fatal("a process outside the run cgroup was killed")
	}

	if err := killCgroupPID(own, pid); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	reaped = true
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitSignal(exit.ProcessState) != int(unix.SIGKILL) {
		t.Fatalf("member of the run cgroup ended with %v, want SIGKILL", err)
	}
	if err := killCgroupPID(own, pid); !errors.Is(err, unix.ESRCH) {
		t.Fatalf("reaped PID: %v, want ESRCH (which the sweep tolerates)", err)
	}
}

// H1: the session lives exactly as long as its holder; a readable pidfd means
// the holder is dead, and a session without a valid pidfd is never alive.
func TestHolderLivenessFollowsPidfd(t *testing.T) {
	for _, pidfd := range []int{-1, 1 << 40} {
		if holderAlive(&scopeSession{pidfd: pidfd}) {
			t.Fatalf("session with pidfd %d reported alive", pidfd)
		}
	}
	pidfd := -1
	cmd := helperCmd("sleep")
	cmd.SysProcAttr = &syscall.SysProcAttr{PidFD: &pidfd}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unix.Close(pidfd) }()
	s := &scopeSession{pidfd: pidfd}
	if !holderAlive(s) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("running holder reported dead")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for holderAlive(s) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	dead := !holderAlive(s) // read before the reap: the pidfd alone is the evidence
	_ = cmd.Wait()
	if !dead {
		t.Fatal("killed holder still reported alive after 5 s")
	}
}
