package sidecar

import (
	"context"
	"fmt"
	"math/rand/v2"
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

// TestBoundSparesPageCache: memory.high counts the page cache, so a run that
// reads a file larger than its bound is a legitimate run, not a bound
// violation (ADR 0044 point 1g). The file lives on a disk filesystem: tmpfs
// pages are shmem, which is unreclaimable without swap. On slow storage the
// pages waiting for their read cannot be reclaimed yet, so memory.current
// stays above memory.high for longer than the watchdog's grace (160 ms to
// 200 ms at 50 MB/s, measured 2026-11-16): the "slow storage" case caps the
// scope's read bandwidth there with io.max, where the io controller is
// delegated.
func TestBoundSparesPageCache(t *testing.T) {
	for _, c := range []struct {
		name string
		rbps uint64
	}{{"local storage", 0}, {"slow storage", 50 << 20}} {
		t.Run(c.name, func(t *testing.T) {
			if !requireNScope(t) {
				return
			}
			dir := pageCacheDir(t)
			const size = 512 << 20 // four times the 128 MiB bound
			writeEvicted(t, filepath.Join(dir, "data"), size)
			device := ""
			if c.rbps > 0 {
				device = blockDevice(t, dir)
			}
			m := scopeManager(t)
			s := residentSpec(t, m, "pagecache:data")
			s.Dir = dir
			var run map[string]uint64
			var throttle error
			m.beforeStart = func(r *cgroupRun) {
				r.watch.beforeRead = func() { run, _ = readCounters(filepath.Join(r.path, "memory.events.local")) }
				if device != "" {
					throttle = throttleReads(r.session.path, device, c.rbps)
				}
			}
			out, err := Run(t.Context(), s)
			if err != nil {
				t.Fatal(err)
			}
			if throttle != nil {
				t.Skipf("io controller not delegated to the scope (%v): slow storage cannot be shown", throttle)
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
		})
	}
}

// TestBoundCountsDirtyFileCache: dirty and writeback pages count against the
// bound (ADR 0044 point 1g), and a run that writes four times its bound into a
// regular file is still an ordinary run: the kernel throttles a cgroup's
// dirtying to a share of its memory (dirty plus writeback peaked at 69 to
// 102 MiB under a 128 MiB memory.high, the watchdog's measure at 53 to
// 69 MiB, measured 2026-11-16), so it completes, with no bound outcome and
// no OOM kill. The output is on disk, not the tier-N tmpfs (whose pages are
// shmem and count anyway).
func TestBoundCountsDirtyFileCache(t *testing.T) {
	if !requireNScope(t) {
		return
	}
	dir := pageCacheDir(t)
	m := scopeManager(t)
	path := filepath.Join(dir, "out")
	s := residentSpec(t, m, "dirty:"+path)
	s.Dir = dir
	s.WatchDir, s.MaxDirBytes, s.WriteFiles, s.FileSizeLimit = dir, 1<<30, []string{"out"}, 1<<30
	var run map[string]uint64
	m.beforeStart = func(r *cgroupRun) {
		r.watch.beforeRead = func() { run, _ = readCounters(filepath.Join(r.path, "memory.events.local")) }
	}
	out, err := runWithoutMount(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Containment != "cgroup" {
		t.Fatal("no delegated user scope or PID namespaces available")
	}
	if out.Signal != 0 || out.ExitCode != 0 || out.MemoryCause != MemoryNone || string(out.Stdout) != fmt.Sprintf("wrote=%d\n", 512<<20) {
		t.Fatalf("S-11 page cache: dirty run: signal=%d exit=%d cause=%s stdout=%q stderr=%q, want a complete write and no bound", out.Signal, out.ExitCode, out.MemoryCause, out.Stdout, out.Stderr)
	}
	if run["high"] == 0 || run["oom_kill"] != 0 {
		t.Fatalf("run counters %v: want the dirty cache to have reached memory.high, and no OOM kill", run)
	}
}

// pageCacheDir is a fresh directory on a disk filesystem, removed at cleanup.
func pageCacheDir(t *testing.T) string {
	t.Helper()
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
	return dir
}

// blockDevice is the "major:minor" of the whole disk under dir's mount, as
// io.max names it: the mount's source device, or its parent if a partition.
func blockDevice(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		t.Fatal(err)
	}
	source, longest := "", -1
	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		pre, post, ok := strings.Cut(line, " - ")
		f, g := strings.Fields(pre), strings.Fields(post)
		if !ok || len(f) < 5 || len(g) < 2 {
			continue
		}
		mount := f[4]
		if (dir == mount || strings.HasPrefix(dir, strings.TrimSuffix(mount, "/")+"/")) && len(mount) > longest {
			source, longest = g[1], len(mount)
		}
	}
	var st unix.Stat_t
	if err := unix.Stat(source, &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFBLK {
		t.Skipf("%s is not on a block device (%q: %v)", dir, source, err)
	}
	sys, err := filepath.EvalSymlinks(fmt.Sprintf("/sys/dev/block/%d:%d", unix.Major(st.Rdev), unix.Minor(st.Rdev)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sys, "partition")); err == nil {
		sys = filepath.Dir(sys)
	}
	dev, err := os.ReadFile(filepath.Join(sys, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(dev))
}

// throttleReads caps the read bandwidth of every process of the scope on
// device, through the scope's io.max.
func throttleReads(scope, device string, rbps uint64) error {
	controllers, err := readValue(filepath.Join(scope, "cgroup.controllers"))
	if err != nil {
		return err
	}
	if !containsWord(controllers, "io") {
		return fmt.Errorf("controllers %q", controllers)
	}
	return writeValue(filepath.Join(scope, "io.max"), fmt.Sprintf("%s rbps=%d", device, rbps))
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
	// Pseudo-random, not a pattern: a filesystem could store zeros as holes,
	// and a compressing one (btrfs compress=zstd) would read a pattern from
	// storage in a few megabytes.
	chunk := make([]byte, 1<<20)
	_, _ = rand.NewChaCha8([32]byte{1}).Read(chunk)
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
