package sidecar

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// These tests prove the death-evidence rules of ADR 0042 (rows J1, H6 and R6
// of docs/plans/2026-11-11-sandbox-crash-reports.md): who died is read from
// the host procfs before the reap, and is never guessed.

func writeProc(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// J1: the target is the child whose innermost namespace PID (last NSpid field)
// is the PID the namespace init started; its host PID and command name come
// from the host procfs. A child whose status cannot be read is passed over.
func TestHostIdentityMatchesInnermostNamespacePID(t *testing.T) {
	dir := t.TempDir()
	writeProc(t, dir, "thread-self/children", "4100 4200 4300 4400 ")
	writeProc(t, dir, "4200/status", "Name:\tholder\nPid:\t4200\nNSpid:\t4200\t9\n")
	writeProc(t, dir, "4300/status", "Name:\ttypst\nUmask:\t0022\nPid:\t4300\nPPid:\t4000\nNSpid:\t4300\t2\n")
	writeProc(t, dir, "4400/status", "Name:\tsame-host-pid\nPid:\t2\nNSpid:\t2\t77\n")
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()

	d, err := targetHostIdentity(root, 2, true)
	if err != nil || d == nil || d.PID != 4300 || d.Comm != "typst" || len(d.NSPID) != 2 || d.NSPID[0] != 4300 || d.NSPID[1] != 2 {
		t.Fatalf("identity of namespace PID 2 = %+v, %v; want host PID 4300 (typst)", d, err)
	}
	if d, err := targetHostIdentity(root, 5, true); err == nil {
		t.Fatalf("required identity of a PID that is no child: %+v", d)
	}
	if d, err := targetHostIdentity(root, 5, false); err != nil || d != nil {
		t.Fatalf("optional identity of a PID that is no child: %+v, %v", d, err)
	}
	if d, err := targetHostIdentity(nil, 2, true); err == nil {
		t.Fatalf("required identity without a host procfs: %+v", d)
	}
}

// J1: the identity is read again after the death and before the reap, so the
// record carries the name the target had when it died, not the launcher's or
// the executable's; without a readable status there is no identity at all.
func TestIdentityRefreshedFromStatusAtDeath(t *testing.T) {
	dir := t.TempDir()
	writeProc(t, dir, "4300/status", "Name:\trenamed-itself\nPid:\t4300\nNSpid:\t4300\t2\n")
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()

	atStart := &SandboxDeath{PID: 4300, NSPID: []int{4300, 2}, Comm: "exe"}
	d, err := refreshTargetIdentity(root, atStart, true)
	if err != nil || d == nil || d.Comm != "renamed-itself" || d.PID != 4300 || atStart.Comm != "exe" {
		t.Fatalf("refreshed identity = %+v, %v", d, err)
	}
	gone := &SandboxDeath{PID: 4999, NSPID: []int{4999, 2}, Comm: "exe"}
	if d, err := refreshTargetIdentity(root, gone, true); err == nil {
		t.Fatalf("required identity kept from before the death: %+v", d)
	}
	if d, err := refreshTargetIdentity(root, gone, false); err != nil || d != nil {
		t.Fatalf("optional identity of an unreadable status = %+v, %v; want none", d, err)
	}
	if d, err := refreshTargetIdentity(root, nil, true); err != nil || d != nil {
		t.Fatalf("identity invented for a target that had none: %+v, %v", d, err)
	}
}

// J1: the manifest gets one line per deliberate death and nothing else; a
// line that cannot be written fails the run instead of losing the evidence.
func TestDeathManifestOnlyRecordsSignalDeaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deaths.jsonl")
	t.Setenv("INKWELL_SANDBOX_DEATHS", path)
	d := &SandboxDeath{PID: 4300, NSPID: []int{4300, 2}, Comm: "typst"}

	if err := recordRunManifest(t.Name(), Output{Death: d, ExitCode: 1}); err != nil {
		t.Fatal(err)
	}
	if err := recordRunManifest("", Output{Death: d, Signal: 31}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a run that is no labelled signal death wrote the manifest: %v", err)
	}

	for _, out := range []Output{
		{Death: d, Signal: 31, CoreDump: CoreDumpNone},
		{Death: d, Signal: 11, CoreDump: CoreDumpStarted},
	} {
		if err := recordRunManifest(t.Name(), out); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(b, []byte("\n")), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("%d manifest lines, want 2 appended records:\n%s", len(lines), b)
	}
	for i, want := range []struct {
		signal     float64
		coredumped bool
	}{{31, false}, {11, true}} {
		var got map[string]any
		if err := json.Unmarshal(lines[i], &got); err != nil {
			t.Fatal(err)
		}
		if got["test"] != t.Name() || got["signal"] != want.signal || got["coredumped"] != want.coredumped || got["pid"] != float64(4300) {
			t.Errorf("line %d = %v, want signal %v coredumped %v", i, got, want.signal, want.coredumped)
		}
	}

	for name, incomplete := range map[string]*SandboxDeath{
		"no host PID":      {NSPID: []int{2}, Comm: "typst"},
		"no namespace PID": {PID: 4300, Comm: "typst"},
		"no command name":  {PID: 4300, NSPID: []int{4300, 2}},
	} {
		if err := recordRunManifest(t.Name(), Output{Death: incomplete, Signal: 31}); err == nil {
			t.Errorf("death with %s recorded", name)
		}
	}

	t.Setenv("INKWELL_SANDBOX_DEATHS", filepath.Join(t.TempDir(), "missing", "deaths.jsonl"))
	if err := recordRunManifest(t.Name(), Output{Death: d, Signal: 31}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unwritable manifest: %v, want the open error", err)
	}
}

// J1: identity is captured before the reap from the child's own pidfd; with
// no pidfd, or one that names no child, there is no identity and no guess.
func TestDeathIdentityCapturedBeforeReap(t *testing.T) {
	if d := captureDeath(exec.Command("unused")); d != nil {
		t.Fatalf("identity without a pidfd: %+v", d)
	}
	closed := -1
	if d := captureDeath(&exec.Cmd{SysProcAttr: &syscall.SysProcAttr{PidFD: &closed}}); d != nil {
		t.Fatalf("identity from an invalid pidfd: %+v", d)
	}

	pidfd := -1
	cmd := helperCmd("exit:7")
	cmd.SysProcAttr = &syscall.SysProcAttr{PidFD: &pidfd}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unix.Close(pidfd) }()
	d := captureDeath(cmd)
	// The child is still a zombie here: Wait must find its real status.
	err := cmd.Wait()
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() != 7 {
		t.Fatalf("status after the capture: %v, want exit 7 (the capture must not reap)", err)
	}
	comm := filepath.Base(os.Args[0])
	comm = comm[:min(len(comm), 15)]
	if d == nil || d.PID != cmd.Process.Pid || len(d.NSPID) == 0 || d.NSPID[len(d.NSPID)-1] != cmd.Process.Pid || d.Comm != comm {
		t.Fatalf("identity = %+v, want PID %d and command %q", d, cmd.Process.Pid, comm)
	}
}

// ADR 0042, CoreDump: "none" is a kernel statement about a pidfd. Anything
// that is not a pidfd the kernel answers for gives "unknown", never "none".
func TestCoreDumpUnknownWithoutKernelEvidence(t *testing.T) {
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	for name, fd := range map[string]int{"no descriptor": -1, "regular file": int(file.Fd())} {
		if got := coreDumpOf(fd); got != CoreDumpUnknown {
			t.Errorf("%s: core dump = %d, want unknown", name, got)
		}
	}
}

// The signal report travels on an inherited descriptor of its own (3 or
// above), apart from the target's stdio; any other value is refused before
// the first confinement step, and the number is not passed on to the target.
func TestSignalReportDescriptorNeverStdio(t *testing.T) {
	for _, value := range []string{"0", "1", "2", "-1", "three", "3 "} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(signalReportEnv, value)
			err := sandboxExec(sandboxConfig{Probe: true}, nil)
			if err == nil || !strings.Contains(err.Error(), "signal report descriptor") {
				t.Fatalf("report descriptor %q: %v", value, err)
			}
		})
	}
	t.Setenv(signalReportEnv, "7")
	c := sandboxConfig{}
	if err := readReportFD(&c); err != nil || c.ReportFD != 7 || os.Getenv(signalReportEnv) != "" {
		t.Fatalf("report descriptor 7: fd %d, %v, still exported %q", c.ReportFD, err, os.Getenv(signalReportEnv))
	}
}

// R6: the bootstrap goes on only when /proc/self/cgroup is the expected run
// cgroup, and the expectation is removed from the target's environment.
func TestBootstrapAcceptsOnlyExpectedCgroup(t *testing.T) {
	own, err := selfCgroup()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(expectedCgroupEnv, own)
	if err := sandboxExec(sandboxConfig{Probe: true}, nil); err != nil {
		t.Fatalf("membership in the expected cgroup refused: %v", err)
	}
	if v, ok := os.LookupEnv(expectedCgroupEnv); ok {
		t.Fatalf("expected cgroup still exported to the target: %q", v)
	}
	if own == "/" {
		t.Fatal("test process in the root cgroup: no other cgroup path to compare with")
	}
	for _, other := range []string{own + "/run-1", filepath.Dir(own), own + "/"} {
		t.Setenv(expectedCgroupEnv, other)
		if err := sandboxExec(sandboxConfig{Probe: true}, nil); err == nil {
			t.Fatalf("membership accepted for %q while in %q", other, own)
		}
	}
}

// H6: the namespace init arms PR_SET_PDEATHSIG(SIGKILL) first, then compares
// its real parent with the app that launched it, and refuses to go on when
// that app is already gone (the race the signal alone cannot close).
func TestNamespaceInitChecksLaunchingParent(t *testing.T) {
	const name = "TestNamespaceInitChecksLaunchingParent"
	if os.Getenv("INKWELL_PARENT_DEATH") != "1" {
		// The parent-death signal stays armed on the thread: use a child process.
		runChild(t, childTest(name, "INKWELL_PARENT_DEATH=1"), name)
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	armed := func() int {
		var sig int32
		if err := unix.Prctl(unix.PR_GET_PDEATHSIG, uintptr(unsafe.Pointer(&sig)), 0, 0, 0); err != nil {
			t.Fatal(err)
		}
		return int(sig)
	}
	if got := armed(); got != 0 {
		t.Fatalf("parent-death signal %d armed before the bootstrap", got)
	}

	t.Setenv(expectedParentEnv, strconv.Itoa(os.Getpid())) // any process but our parent
	err := sandboxExec(sandboxConfig{Probe: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "launching parent died") {
		t.Fatalf("bootstrap under another parent: %v", err)
	}
	if got := armed(); got != int(unix.SIGKILL) {
		t.Fatalf("parent-death signal = %d after the check, want SIGKILL armed before it", got)
	}

	t.Setenv(expectedParentEnv, strconv.Itoa(os.Getppid()))
	if err := sandboxExec(sandboxConfig{Probe: true}, nil); err != nil {
		t.Fatalf("bootstrap under the launching parent: %v", err)
	}
	if v, ok := os.LookupEnv(expectedParentEnv); ok {
		t.Fatalf("launching parent still exported to the target: %q", v)
	}
	if got := armed(); got != int(unix.SIGKILL) {
		t.Fatalf("parent-death signal = %d, want SIGKILL", got)
	}
}

// J1 and ADR 0042 (no core helper): the namespace init waits for the target
// without reaping it, reads its host identity, then reports the signal, the
// kernel's core-dump mark and that identity on the report descriptor; a
// target that exits by itself is reported by its status alone.
func TestInitReportsTargetDeathWithIdentity(t *testing.T) {
	const name = "TestInitReportsTargetDeathWithIdentity"
	if os.Getenv("INKWELL_INIT_REPORT") != "1" {
		// The init reaps whatever child dies (it is PID 1 in production): it
		// must run in a process that has no other child.
		runChild(t, childTest(name, "INKWELL_INIT_REPORT=1"), name)
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	hostProc, err := os.OpenRoot("/proc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hostProc.Close() }()
	comm := filepath.Base(os.Args[0])
	comm = comm[:min(len(comm), 15)]

	for _, tc := range []struct {
		attack string // what the target does once confined (see confinementAttack)
		signal unix.Signal
	}{{"xfsz", unix.SIGXFSZ}, {"fds", 0}} {
		t.Run(tc.attack, func(t *testing.T) {
			// thread-self/children lists the children of this very thread.
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close(); _ = w.Close() }()
			t.Setenv("INKWELL_ATTACK", tc.attack)
			pidfd := -1
			c := sandboxConfig{WorkingDir: cwd, ReportFD: int(w.Fd()), RequireDeath: true}
			cmd, err := sandboxInitCommand(c, []string{os.Args[0]}, &pidfd)
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = unix.Close(pidfd); _ = cmd.Process.Release() }()
			atStart, err := targetHostIdentity(hostProc, cmd.Process.Pid, c.RequireDeath)
			if err != nil {
				_ = cmd.Process.Kill()
				t.Fatal(err)
			}
			if atStart.PID != cmd.Process.Pid {
				t.Fatalf("identity at start = %+v, want PID %d", atStart, cmd.Process.Pid)
			}

			status, err := reapSandboxTarget(c, cmd, hostProc, atStart, pidfd)
			if err != nil {
				t.Fatal(err)
			}
			if err := unix.Kill(cmd.Process.Pid, 0); !errors.Is(err, unix.ESRCH) {
				t.Fatalf("target not reaped: kill(0) = %v", err)
			}
			_ = w.Close()
			report := make([]byte, 8192)
			n, _ := r.Read(report)
			report = report[:n]

			if tc.signal == 0 {
				if status != 0 || len(report) != 0 {
					t.Fatalf("target that exited 0: status %d, report %q", status, report)
				}
				return
			}
			if status != 128+int(tc.signal) {
				t.Fatalf("status = %d, want 128+%d", status, tc.signal)
			}
			if len(report) < 3 || report[0] != byte(tc.signal) {
				t.Fatalf("report = %q, want signal %d, core-dump mark and identity", report, tc.signal)
			}
			if CoreDump(report[1]) == CoreDumpStarted || CoreDump(report[1]) > CoreDumpStarted {
				t.Fatalf("core-dump mark = %d: a core-class death under RLIMIT_CORE 1 must start no helper", report[1])
			}
			var d SandboxDeath
			if err := json.Unmarshal(report[2:], &d); err != nil {
				t.Fatalf("identity %q: %v", report[2:], err)
			}
			if d.PID != cmd.Process.Pid || len(d.NSPID) == 0 || d.NSPID[len(d.NSPID)-1] != cmd.Process.Pid || d.Comm != comm {
				t.Fatalf("identity = %+v, want PID %d and the target's own name %q", d, cmd.Process.Pid, comm)
			}
		})
	}
}
