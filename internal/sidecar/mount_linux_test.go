package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// mountSupported is the probe's conclusive answer; an inconclusive probe fails the test.
func mountSupported(t *testing.T) bool {
	t.Helper()
	supported, err := probeOutputMount()
	if err != nil {
		t.Fatal(err)
	}
	return supported
}

func requireOutputMount(t *testing.T) {
	t.Helper()
	if !mountSupported(t) {
		t.Skip("host denies unprivileged user/mount namespace or tmpfs (AppArmor/sysctl/container policy)")
	}
}

func TestOutputMountENOSPC(t *testing.T) {
	requireOutputMount(t)
	dir := t.TempDir()
	s := spec("fillmount:" + dir)
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 1<<20, []string{"a", "b"}
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || !strings.Contains(string(out.Stdout), "ENOSPC") {
		t.Fatalf("Run = %+v, %v", out, err)
	}
	if got := dirBytes(dir); got != s.MaxDirBytes {
		t.Fatalf("directory bytes = %d, want %d", got, s.MaxDirBytes)
	}
}

func TestOutputMountRejectsSparseLogicalOverflow(t *testing.T) {
	requireOutputMount(t)
	dir := t.TempDir()
	s := spec("sparsemount:" + dir)
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 8<<20, []string{"a", "b"}
	out, err := Run(t.Context(), s)
	if err == nil || !out.OutputLimit {
		t.Fatalf("sparse outputs must be rejected: %+v, %v", out, err)
	}
	if got := dirBytes(dir); got > s.MaxDirBytes {
		t.Fatalf("exported %d bytes", got)
	}
}

// An unused output must not reserve a fixed share of the directory budget.
// This deterministically distinguishes remaining-budget enforcement from shares.
func TestDirectoryBoundAllowsUnevenOutputs(t *testing.T) {
	requireOutputMount(t)
	dir := t.TempDir()
	s := spec("writefile:" + filepath.Join(dir, "a") + ",3")
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 4<<20, []string{"a", "b"}
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || out.OutputLimit {
		t.Fatalf("3 MiB in a 4 MiB budget with an unused output: Run = %+v, %v", out, err)
	}
	if got := dirBytes(dir); got != 3<<20 {
		t.Fatalf("directory holds %d bytes, want 3 MiB", got)
	}
}

func TestOutputMountProbeCachesConclusiveAnswers(t *testing.T) {
	for _, want := range []bool{true, false} {
		calls := 0
		probe := cachedProbe(func(context.Context) (bool, error) { calls++; return want, nil }, time.Second, time.Now)
		for range 3 {
			if got, err := probe(); got != want || err != nil {
				t.Fatalf("probe = %v, %v, want %v", got, err, want)
			}
		}
		if calls != 1 {
			t.Fatalf("conclusive %v probed %d times, want once", want, calls)
		}
	}
}

func TestOutputMountProbeRetriesInconclusive(t *testing.T) {
	calls := 0
	clock := time.Unix(0, 0)
	probe := cachedProbe(func(context.Context) (bool, error) {
		calls++
		if calls <= 2 {
			return false, unix.EAGAIN
		}
		return true, nil
	}, time.Second, func() time.Time { return clock })
	refused := func() {
		t.Helper()
		if got, err := probe(); got || !errors.Is(err, ErrProbeInconclusive) || !errors.Is(err, unix.EAGAIN) {
			t.Fatalf("probe = %v, %v, want a refusal wrapping ErrProbeInconclusive", got, err)
		}
	}
	refused()
	refused()
	if calls != 1 {
		t.Fatalf("probe calls = %d, want the refusal repeated without probing during the backoff", calls)
	}
	clock = clock.Add(probeBackoffMin)
	refused()
	clock = clock.Add(2*probeBackoffMin - time.Nanosecond)
	refused()
	if calls != 2 {
		t.Fatalf("probe calls = %d, want one retry per elapsed, doubled backoff", calls)
	}
	clock = clock.Add(time.Nanosecond)
	if got, err := probe(); !got || err != nil || calls != 3 {
		t.Fatalf("probe = %v, %v after %d calls, want the inconclusive result never cached", got, err, calls)
	}
}

func TestNamespaceENOSPCDefinitiveOnlyWithZeroLimit(t *testing.T) {
	saved := userNamespaceLimit
	t.Cleanup(func() { userNamespaceLimit = saved })
	clone := &fs.PathError{Op: "fork/exec", Path: "/proc/self/exe", Err: unix.ENOSPC}
	for _, c := range []struct {
		limit      int
		readErr    error
		definitive bool
	}{
		{limit: 0, definitive: true}, // namespaces disabled by sysctl
		{limit: 63452},               // the limit is only reached for now
		{readErr: os.ErrNotExist},    // unknown: never assume disabled
		{limit: 1},
	} {
		userNamespaceLimit = func() (int, error) { return c.limit, c.readErr }
		if got := definitiveRefusal(clone); got != c.definitive {
			t.Errorf("ENOSPC with limit %d (%v): definitive %v, want %v", c.limit, c.readErr, got, c.definitive)
		}
	}
	// Through the probe cache: a transient ENOSPC refuses and is not cached.
	userNamespaceLimit = func() (int, error) { return 63452, nil }
	calls := 0
	clock := time.Unix(0, 0)
	probe := cachedProbe(func(context.Context) (bool, error) {
		calls++
		if definitiveRefusal(clone) {
			return false, nil
		}
		return false, clone
	}, time.Second, func() time.Time { return clock })
	if _, err := probe(); !errors.Is(err, ErrProbeInconclusive) {
		t.Fatalf("transient ENOSPC: %v, want ErrProbeInconclusive", err)
	}
	clock = clock.Add(probeBackoffMax)
	userNamespaceLimit = func() (int, error) { return 0, nil }
	if got, err := probe(); got || err != nil || calls != 2 {
		t.Fatalf("ENOSPC with namespaces disabled: %v, %v after %d probes, want a cached refusal", got, err, calls)
	}
}

func TestOutputMountProbeBackoffIsCapped(t *testing.T) {
	// Without the cap, the doubling would outgrow probeBackoffMax by the sixth failure.
	clock := time.Unix(0, 0)
	calls := 0
	probe := cachedProbe(func(context.Context) (bool, error) { calls++; return false, unix.EAGAIN }, time.Second, func() time.Time { return clock })
	for range 20 {
		_, _ = probe()
		clock = clock.Add(probeBackoffMax)
	}
	if calls != 20 {
		t.Fatalf("%d probes in 20 maximal backoffs, want 20", calls)
	}
}

// swapProbe replaces the package probe for one test.
func swapProbe(t *testing.T, probe func() (bool, error)) {
	t.Helper()
	saved := probeOutputMount
	probeOutputMount = probe
	t.Cleanup(func() { probeOutputMount = saved })
}

// launchOverlay reports the Overlay policy withLimits encodes for a launch.
func launchOverlay(t *testing.T) (bool, error) {
	t.Helper()
	_, args, err := withLimits(Spec{Path: "/bin/true", Dir: t.TempDir()})
	if err != nil {
		return false, err
	}
	var c sandboxConfig
	if err := json.Unmarshal([]byte(args[1]), &c); err != nil {
		t.Fatal(err)
	}
	return c.Overlay, nil
}

func TestOutputMountProbeHungChildRefusesLaunch(t *testing.T) {
	// A probe child that never exits is killed at the deadline. The launch is
	// refused, never downgraded, and the next probe after the backoff runs again.
	const timeout = 200 * time.Millisecond
	clock := time.Unix(0, 0)
	calls := 0
	swapProbe(t, cachedProbe(func(ctx context.Context) (bool, error) {
		calls++
		if calls == 1 {
			return runMountProbe(ctx, helperEnv("sleep"), os.Args[0])
		}
		return true, nil
	}, timeout, func() time.Time { return clock }))
	begin := time.Now()
	if _, err := launchOverlay(t); !errors.Is(err, ErrProbeInconclusive) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("launch with a hung probe: %v, want ErrProbeInconclusive after the deadline", err)
	}
	if took := time.Since(begin); took > timeout+5*time.Second {
		t.Fatalf("hung probe refused after %v, want about %v", took, timeout)
	}
	if err := configureConfinement(exec.Command("/bin/true")); !errors.Is(err, ErrProbeInconclusive) {
		t.Fatalf("confinement during the backoff: %v, want the same refusal", err)
	}
	clock = clock.Add(probeBackoffMax)
	if overlay, err := launchOverlay(t); err != nil || !overlay || calls != 2 {
		t.Fatalf("after the backoff: overlay %v, %v, %d probes; want a fresh, successful probe", overlay, err, calls)
	}
}

func TestOutputMountProbeDefinitiveRefusalSelectsFallback(t *testing.T) {
	// The real probe child, in a user namespace that does not own the mount
	// namespace, gets EPERM on its first mount and reports it as definitive.
	path, args, err := sandboxPrefix(sandboxConfig{Directory: t.TempDir(), MountBytes: 4096, Probe: true, Overlay: true})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, args...) // #nosec G204 -- re-exec our own binary.
	cmd.Env = []string{}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	out, err := cmd.CombinedOutput()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != probeUnsupportedExit || !strings.Contains(string(out), "operation not permitted") {
		t.Fatalf("probe child refused by EPERM: %v %s, want exit %d", err, out, probeUnsupportedExit)
	}
	// That status, or a refused clone, is a conclusive "unsupported"; others are not.
	if got, err := runMountProbe(t.Context(), helperEnv("exit:"+strconv.Itoa(probeUnsupportedExit)), os.Args[0]); got || err != nil {
		t.Fatalf("definitive exit: %v, %v, want unsupported", got, err)
	}
	if got, err := runMountProbe(t.Context(), helperEnv("exit:125"), os.Args[0]); got || err == nil {
		t.Fatalf("generic failure: %v, %v, want inconclusive", got, err)
	}
	for _, refusal := range []error{unix.EPERM, unix.EINVAL} {
		if !definitiveRefusal(&fs.PathError{Op: "fork/exec", Path: path, Err: refusal}) {
			t.Errorf("%v from clone is not definitive", refusal)
		}
	}
	if definitiveRefusal(&fs.PathError{Op: "fork/exec", Path: path, Err: unix.EAGAIN}) {
		t.Error("EAGAIN from clone treated as definitive")
	}
	// The cached refusal selects the fallback: no error, no overlay, one probe.
	calls := 0
	swapProbe(t, cachedProbe(func(context.Context) (bool, error) { calls++; return false, nil }, time.Second, time.Now))
	for range 2 {
		if overlay, err := launchOverlay(t); err != nil || overlay {
			t.Fatalf("definitive refusal: overlay %v, %v, want the namespace-free fallback", overlay, err)
		}
	}
	if calls != 1 {
		t.Fatalf("definitive refusal probed %d times, want it cached", calls)
	}
}

func TestOutputMountExportFailureRemovesCreatedFiles(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("output"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, "b"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	m := directoryMount{spec: Spec{WatchDir: dest, MaxDirBytes: 100}, root: root}
	if err := m.export(); err == nil {
		t.Fatal("expected destination collision")
	}
	if _, err := os.Stat(filepath.Join(dest, "a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partly exported a remains: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "b"))
	if err != nil || string(data) != "existing" {
		t.Fatalf("existing output changed: %q, %v", data, err)
	}
}

func TestExportFileRejectsSizeChange(t *testing.T) {
	for _, size := range []int64{2, 8} {
		t.Run(strconv.FormatInt(size, 10), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "out")
			if err := exportFile(path, strings.NewReader("data"), size, 100); err == nil {
				t.Fatal("accepted changed size")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("partial file remains: %v", err)
			}
		})
	}
}
