package typst

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

// W49 T3, ADR 0042 A6: the real typst runs with SIGXFSZ ignored and no other
// signal ignored by us, whatever ignored signals started the app.
func TestTargetIgnoresOnlyFileSizeSignal(t *testing.T) {
	e := newEngine(t, func(c *Config) { c.Timeout = time.Minute })
	p := writeProject(t, map[string]string{"main.typ": "#for i in range(10000) { for j in range(10000) { let x = i * j } }\n"})
	bin, err := sharedStore.Resolve(sidecar.Typst)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := e.Compile(ctx, p, "main.typ", WithFormats(FormatPDF)); done <- err }()
	defer func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Compile after the cancel = %v, want Canceled", err)
		}
	}()
	var pids []int
	for deadline := time.Now().Add(20 * time.Second); len(pids) == 0 && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
		pids = typstProcesses(t, bin)
	}
	if len(pids) == 0 {
		t.Fatal("typst never started")
	}
	got, err := ignoredSignals(pids[0])
	if err != nil {
		t.Fatal(err)
	}
	// SIGPIPE is the target's own choice (Rust's runtime may ignore it; the
	// pinned typst, measured on 2026-11-13, does not): only the rest is ours.
	want, own := uint64(1)<<(syscall.SIGXFSZ-1), uint64(1)<<(syscall.SIGPIPE-1)
	if got&^own != want {
		t.Errorf("typst ignores signals %#x, want SIGXFSZ (%#x) and at most SIGPIPE", got, want)
	}
}

// ignoredSignals reads the SigIgn mask of a process.
func ignoredSignals(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "SigIgn:"); ok {
			return strconv.ParseUint(strings.TrimSpace(value), 16, 64)
		}
	}
	return 0, errors.New("no SigIgn line")
}

// X5 (ADR 0042 A6): over its output cap, the pinned typst exits 1 with no
// signal and Rust's "os error 27" (EFBIG), and the compilation is still
// ErrOutputTooLarge, for the PDF and the SVG pages (TYP-19).
func TestPinnedTypstOutputCapExitsOne(t *testing.T) {
	e := newEngine(t, func(c *Config) { c.MaxOutput = 200 })
	p := writeProject(t, map[string]string{"main.typ": "#for i in range(5) [Page #i #pagebreak()]\n"})
	for _, format := range []string{"pdf", "svg"} {
		t.Run(format, func(t *testing.T) {
			j, err := e.prepare(p, "main.typ")
			if err != nil {
				t.Fatal(err)
			}
			defer j.cleanup()
			dir := filepath.Join(t.TempDir(), format)
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			o := options{timestamp: time.Unix(0, 0)}
			var out sidecar.Output
			if format == "pdf" {
				out, err = e.run(t.Context(), j, e.args("compile", j, o, "--format", "pdf", "--", filepath.ToSlash(j.main), filepath.Join(dir, "out.pdf")), maxStderr, dir, 0, 0)
			} else {
				out, err = e.run(t.Context(), j, e.args("compile", j, o, "--format", "svg", "--pages", "1-4", "--", filepath.ToSlash(j.main), filepath.Join(dir, "page-{p}.svg")), maxStderr, dir, 1, 4)
			}
			if err != nil || out.Signal != 0 || out.ExitCode != 1 || !strings.Contains(string(out.Stderr), "os error 27") {
				t.Fatalf("run = %v, exit %d signal %d stderr %q; want exit 1, no signal, os error 27", err, out.ExitCode, out.Signal, out.Stderr)
			}
			if !out.OutputLimit {
				t.Error("OutputLimit = false at the cap")
			}
			formats := map[string]Format{"pdf": FormatPDF, "svg": FormatSVG}[format]
			if _, err := e.Compile(t.Context(), p, "main.typ", WithFormats(formats)); !errors.Is(err, ErrOutputTooLarge) {
				t.Errorf("Compile = %v, want ErrOutputTooLarge", err)
			}
		})
	}
}

func TestFailureMemoryCause(t *testing.T) {
	for _, tc := range []struct {
		cause  sidecar.MemoryCause
		memory bool
	}{{sidecar.MemoryLocalLimit, true}, {sidecar.MemoryVictim, false}, {sidecar.MemoryUnknown, false}, {sidecar.MemoryNone, false}} {
		err := failure(&Result{}, &sidecar.Output{ExitCode: -1, Signal: 9, MemoryCause: tc.cause, Stderr: []byte("memory allocation of CANARY bytes failed")}, DefaultMemoryLimit)
		if errors.Is(err, ErrMemoryLimit) != tc.memory {
			t.Errorf("cause %s: %v", tc.cause, err)
		}
	}
}

func hostileMemoryRun(t *testing.T, resident, address uint64) (sidecar.Output, error) {
	t.Helper()
	e := newEngine(t, func(c *Config) { c.MemoryLimit = address; c.ResidentLimit = resident; c.Timeout = time.Minute })
	p := writeProject(t, map[string]string{"main.typ": "#let a = range(20000000).map(x => str(x))\n#a.len()\n"})
	j, err := e.prepare(p, "main.typ")
	if err != nil {
		t.Fatal(err)
	}
	defer j.cleanup()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	out, err := e.run(ctx, j, e.args("compile", j, options{timestamp: DefaultTimestamp()}, "--format", "pdf", "--", filepath.ToSlash(j.main), filepath.Join(t.TempDir(), "out.pdf")), maxStderr, "", 0, 0)
	return out, err
}

// hostileContainment is what bounds a hostile document's memory in the tier
// this machine gives: the run cgroup in N with a delegated user scope, the
// address limit alone in F (no cgroup without a PID namespace) and in N
// without one (the per-process fallback, ADR 0042, ADR 0044, T-6).
func hostileContainment(t *testing.T, isolated bool) string {
	t.Helper()
	if !isolated {
		return "rlimit"
	}
	if reason := delegatedScopeUnreachable(); reason != "" {
		t.Log("scope-report: scope=unavailable reason=" + reason)
		return "rlimit"
	}
	t.Log("scope-report: scope=available")
	return "cgroup"
}

// delegatedScopeUnreachable names what keeps this process from a delegated
// user scope, "" when nothing does: the facts of the environment that the
// sidecar's scope needs (its test of the same name in internal/sidecar). The
// process runs under its user's systemd manager, and the session bus answers.
func delegatedScopeUnreachable() string {
	b, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "no-cgroup"
	}
	if !strings.Contains(string(b), fmt.Sprintf("/user@%d.service/", os.Getuid())) {
		return "outside-user-manager"
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "no-session-bus"
	}
	_ = conn.Close()
	return ""
}

// requireCoreReport skips where the kernel cannot say whether it started a
// core dump (PIDFD_INFO_COREDUMP, Linux 6.16), as the sidecar's tests do.
func requireCoreReport(t *testing.T, out sidecar.Output) {
	t.Helper()
	if out.CoreDump == sidecar.CoreDumpUnknown {
		t.Skip("the kernel does not report PIDFD_INFO_COREDUMP (Linux 6.16): no core helper cannot be shown")
	}
}

func TestHostileCgroupMemoryIsMemoryLimit(t *testing.T) {
	isolated, ok := launchTier(t)
	if !ok {
		return
	}
	out, err := hostileMemoryRun(t, 128<<20, 4<<30)
	if err != nil {
		t.Fatal(err)
	}
	want := hostileContainment(t, isolated)
	if out.Containment != want {
		t.Fatalf("containment %q, want %q: %+v", out.Containment, want, out)
	}
	// With a run cgroup the resident bound is Inkwell's own kill; without one
	// the allocation fails at the address limit (S-11). Both are the memory
	// limit to the person.
	if want == "cgroup" && (out.MemoryCause != sidecar.MemoryLocalLimit || out.Signal != 9) {
		t.Fatalf("not a local resident death: %+v", out)
	}
	if err := failure(&Result{}, &out, 4<<30); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal(err)
	}
}

func TestHostileMemoryDeathStartsNoCoreHelper(t *testing.T) {
	isolated, ok := launchTier(t)
	if !ok {
		return
	}
	for _, kind := range []string{"local-limit", "address-limit"} {
		t.Run(kind, func(t *testing.T) {
			resident, address := uint64(128<<20), uint64(4<<30)
			if kind == "address-limit" {
				resident, address = 2<<30, 512<<20
			}
			out, err := hostileMemoryRun(t, resident, address)
			if err != nil {
				t.Fatal(err)
			}
			if want := hostileContainment(t, isolated); out.Containment != want {
				t.Fatalf("containment %q, want %q: %+v", out.Containment, want, out)
			}
			if err := failure(&Result{}, &out, address); !errors.Is(err, ErrMemoryLimit) {
				t.Fatalf("not memory: %v (%+v)", err, out)
			}
			requireCoreReport(t, out)
			if out.CoreDump != sidecar.CoreDumpNone {
				t.Fatalf("the kernel started a core helper: %+v", out)
			}
		})
	}
}
