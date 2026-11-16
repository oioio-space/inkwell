package sidecar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

// S-11 (ADR 0044 point 1g, W73): a target that allocates without end is
// stopped by inkwell at its bound and reported as a memory bound, never by
// the kernel's OOM killer, whose `oom-kill` unit result makes the desktop
// announce that the device's memory is almost full.
func TestBoundViolationIsNotAnOOMKill(t *testing.T) {
	t.Run("cgroup", func(t *testing.T) {
		if !requireNScope(t) {
			return
		}
		m := scopeManager(t)
		s := residentSpec(t, m, "resident")
		var scope string
		var scopeBefore, run map[string]uint64
		var scopeErr, runErr error
		m.beforeStart = func(r *cgroupRun) {
			scope = r.session.path
			scopeBefore, scopeErr = readCounters(filepath.Join(scope, "memory.events"))
			r.watch.beforeRead = func() { run, runErr = readCounters(filepath.Join(r.path, "memory.events.local")) }
		}
		// A watchdog that does not act leaves the target throttled at
		// memory.high for ever: the run must end by the bound, not the test's timeout.
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		out, err := Run(ctx, s)
		if err != nil {
			t.Fatalf("S-11 no oom-kill: the run did not end at its bound: %v", err)
		}
		if out.Containment != "cgroup" {
			t.Fatal("no delegated user scope or PID namespaces available")
		}
		if out.Signal != 9 || out.MemoryCause != MemoryLocalLimit || out.CoreDump != CoreDumpNone {
			t.Fatalf("bound outcome: signal=%d cause=%s core=%d, want inkwell's kill reported as the local bound", out.Signal, out.MemoryCause, out.CoreDump)
		}
		if scopeErr != nil || runErr != nil {
			t.Fatalf("counters unreadable: scope %v, run %v", scopeErr, runErr)
		}
		if run["oom_kill"] != 0 || run["high"] == 0 {
			t.Fatalf("run counters %v: want the bound reached at memory.high and no OOM kill", run)
		}
		after, err := readCounters(filepath.Join(scope, "memory.events"))
		if err != nil || after["oom_kill"] != scopeBefore["oom_kill"] {
			t.Fatalf("scope oom_kill %d -> %d (%v): the kernel's OOM killer fired", scopeBefore["oom_kill"], after["oom_kill"], err)
		}
		requireUnitNotOOMKilled(t, filepath.Base(scope))
	})
	t.Run("rlimit", func(t *testing.T) {
		if !requireLaunch(t) {
			return
		}
		s := deathSpec(t, "scope-resident")
		s.Dir = t.TempDir()
		s.scope = scopeManager(t)
		s.tier = tierF
		// 2 GiB, not 1: the helper's own start must not be what fails (escapeAllocation).
		s.MemoryLimit = 2<<30 + raceAddressReserve(t)
		own, err := selfCgroup()
		if err != nil {
			t.Fatal(err)
		}
		events := filepath.Join("/sys/fs/cgroup", own, "memory.events")
		before, beforeErr := readCounters(events)
		out, err := Run(t.Context(), s)
		if err != nil {
			t.Fatal(err)
		}
		if out.Containment != "rlimit" || out.Signal != 0 || out.ExitCode != 1 || !strings.Contains(string(out.Stdout), "allocation-failed") {
			t.Fatalf("tier F outcome: containment=%s signal=%d exit=%d stdout=%q, want the allocation failed inside the target", out.Containment, out.Signal, out.ExitCode, out.Stdout)
		}
		if beforeErr == nil {
			after, err := readCounters(events)
			if err != nil || after["oom_kill"] != before["oom_kill"] {
				t.Fatalf("own cgroup oom_kill %d -> %d (%v): the kernel's OOM killer fired", before["oom_kill"], after["oom_kill"], err)
			}
		}
	})
}

// TestMemoryMaxBackstop is the residual R-7: with the watchdog disabled the
// target reaches memory.max, set above the bound, and the kernel OOM-kills it.
// It is the one test that may raise the desktop's memory notification, hence
// opt-in.
func TestMemoryMaxBackstop(t *testing.T) {
	if os.Getenv("INKWELL_OOM_BACKSTOP") != "1" {
		t.Skip("set INKWELL_OOM_BACKSTOP=1: this test makes the kernel OOM-kill a target (desktop notification)")
	}
	if !requireNScope(t) {
		return
	}
	m := scopeManager(t)
	s := residentSpec(t, m, "resident")
	var run map[string]uint64
	var limits map[string]string
	m.beforeStart = func(r *cgroupRun) {
		r.watch.disarmed.Store(true)
		limits = runLimits(t, r.path)
		r.watch.beforeRead = func() { run, _ = readCounters(filepath.Join(r.path, "memory.events.local")) }
	}
	out, err := Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Containment != "cgroup" {
		t.Fatal("no delegated user scope or PID namespaces available")
	}
	if limits["memory.max"] != "201326592" || limits["memory.high"] != "134217728" {
		t.Fatalf("limits %v: want memory.high at the bound and memory.max 64 MiB above it", limits)
	}
	if out.Signal != 9 || out.MemoryCause != MemoryLocalLimit || run["oom_kill"] == 0 {
		t.Fatalf("backstop outcome: signal=%d cause=%s counters=%v, want an OOM kill at memory.max", out.Signal, out.MemoryCause, run)
	}
}

// TestBoundSparesPageCache: memory.high counts the page cache, which reclaim
// takes back at once, so a run that reads a file larger than its bound is a
// legitimate run, not a bound violation (ADR 0044 point 1g). The file lives on
// a disk filesystem: tmpfs pages are shmem, which is unreclaimable without swap.
func TestBoundSparesPageCache(t *testing.T) {
	if !requireNScope(t) {
		return
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(cache, "inkwell-pagecache-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	var fs unix.Statfs_t
	if err := unix.Statfs(dir, &fs); err != nil || fs.Type == unix.TMPFS_MAGIC {
		t.Fatalf("%s is not on a disk filesystem (statfs %v, type %#x)", dir, err, fs.Type)
	}
	const size = 512 << 20 // four times the 128 MiB bound
	writeEvicted(t, filepath.Join(dir, "data"), size)
	m := scopeManager(t)
	s := residentSpec(t, m, "pagecache:data")
	s.Dir = dir
	var run map[string]uint64
	m.beforeStart = func(r *cgroupRun) {
		r.watch.beforeRead = func() { run, _ = readCounters(filepath.Join(r.path, "memory.events.local")) }
	}
	out, err := Run(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Containment != "cgroup" {
		t.Fatal("no delegated user scope or PID namespaces available")
	}
	if out.Signal != 0 || out.ExitCode != 0 || out.MemoryCause != MemoryNone || string(out.Stdout) != fmt.Sprintf("read=%d\n", size) {
		t.Fatalf("page-cache run: signal=%d exit=%d cause=%s stdout=%q stderr=%q, want a complete read and no bound", out.Signal, out.ExitCode, out.MemoryCause, out.Stdout, out.Stderr)
	}
	if run["high"] == 0 || run["oom_kill"] != 0 {
		t.Fatalf("run counters %v: want the page cache to have reached memory.high, and no OOM kill", run)
	}
}

// writeEvicted writes size bytes to path, flushes them and drops them from
// the page cache, so that a later read charges the reader's cgroup.
func writeEvicted(t *testing.T, path string, size int) {
	t.Helper()
	f, err := os.Create(path) // #nosec G304 -- test file in the test's own directory
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = byte(i*7 + 1) // not zeros: a filesystem could store zeros as holes
	}
	for range size / len(chunk) {
		if _, err := f.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED); err != nil {
		t.Fatal(err)
	}
}

// residentSpec is scopeSpec for a test whose bound is the cgroup's: its address
// limit stays above what the race runtime reserves (raceAddressReserve).
func residentSpec(t *testing.T, m *SandboxScope, mode string) Spec {
	s := scopeSpec(t, m, mode)
	s.MemoryLimit += raceAddressReserve(t)
	return s
}

// raceAddressReserve is the address space an idle helper reserves under the
// race detector, zero without it. ThreadSanitizer maps its shadow at startup
// whatever RLIMIT_AS allows (measured 2026-11-15, amd64: VmSize 1.92 GiB idle,
// leaving 110 MiB under a 2 GiB limit, and no start at all under 1 GiB),
// whereas Go's own runtime fits its reservations to the limit. The tests add
// it to their address limit so the target keeps the same room to allocate.
func raceAddressReserve(t *testing.T) uint64 {
	t.Helper()
	if !raceEnabled {
		return 0
	}
	status, err := helperCmd("sandboxstatus").Output()
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(status), "\n") {
		if v, ok := strings.CutPrefix(line, "VmPeak:"); ok {
			kb, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			return kb << 10
		}
	}
	t.Fatalf("no VmPeak in the helper's status: %q", status)
	return 0
}

// requireUnitNotOOMKilled watches the session scope's unit for a while after
// the run: systemd reads memory.events asynchronously, and an OOM kill would
// turn its Result into `oom-kill` (and, by its default OOM policy, stop it).
// Without a reachable user manager there is nothing to ask.
func requireUnitNotOOMKilled(t *testing.T, name string) {
	t.Helper()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Logf("no session bus, unit result not checked: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()
	var unit dbus.ObjectPath
	manager := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	deadline := time.Now().Add(time.Second)
	for {
		if err := manager.Call("org.freedesktop.systemd1.Manager.GetUnit", 0, name).Store(&unit); err != nil {
			t.Fatalf("scope unit %s gone after the run: %v", name, err)
		}
		obj := conn.Object("org.freedesktop.systemd1", unit)
		result, err := obj.GetProperty("org.freedesktop.systemd1.Scope.Result")
		if err != nil {
			t.Fatal(err)
		}
		state, err := obj.GetProperty("org.freedesktop.systemd1.Unit.ActiveState")
		if err != nil {
			t.Fatal(err)
		}
		if result.Value() != "success" || state.Value() != "active" {
			t.Fatalf("scope unit %s: Result=%v ActiveState=%v, want success and active", name, result.Value(), state.Value())
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
