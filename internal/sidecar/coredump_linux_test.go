package sidecar

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// corePattern is how kernel.core_pattern hands a core dump on (core(5)).
type corePattern string

const (
	corePipe          corePattern = "pipe"           // "|helper": RLIMIT_CORE 1 is the kernel's "start no helper" sentinel
	coreFile          corePattern = "file"           // a path: 1 byte is below min_coredump, so no file
	coreSocket        corePattern = "socket"         // "@socket" (Linux 6.16): the kernel ignores RLIMIT_CORE
	coreSocketRequest corePattern = "socket-request" // "@@socket" (Linux 6.17): likewise
)

// corePatternMode classifies a kernel.core_pattern value. ADR 0042 A5: the
// no-core-helper guarantee holds for pipe and file only (fs/coredump.c,
// coredump_pipe versus coredump_sock_connect); socket mode is residual RS1.
func corePatternMode(pattern string) corePattern {
	switch {
	case strings.HasPrefix(pattern, "@@"):
		return coreSocketRequest
	case strings.HasPrefix(pattern, "@"):
		return coreSocket
	case strings.HasPrefix(pattern, "|"):
		return corePipe
	}
	return coreFile
}

// guaranteed reports whether RLIMIT_CORE {1,1} keeps every crash helper from starting in this mode.
func (m corePattern) guaranteed() bool { return m == corePipe || m == coreFile }

// requireGuaranteedCorePattern fails the test on a machine where the guarantee does not hold (RS1).
func requireGuaranteedCorePattern(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile("/proc/sys/kernel/core_pattern")
	if err != nil {
		t.Fatalf("cannot read kernel.core_pattern: %v", err)
	}
	pattern := strings.TrimSpace(string(data))
	if mode := corePatternMode(pattern); !mode.guaranteed() {
		t.Fatalf("RS1: kernel.core_pattern %q is in %s mode, where the kernel ignores RLIMIT_CORE: a crash helper may start (ADR 0042 A5)", pattern, mode)
	}
}

// ADR 0042 A5: only pipe and file core_pattern modes are covered by the sentinel.
func TestCorePatternGuaranteeScope(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		want          corePattern
		guaranteed    bool
	}{
		{name: "pipe", pattern: "|/usr/lib/systemd/systemd-coredump %P %u %g %s %t %c %h %d %F", want: corePipe, guaranteed: true},
		{name: "file", pattern: "core.%p", want: coreFile, guaranteed: true},
		{name: "socket", pattern: "@/run/systemd/coredump", want: coreSocket},
		{name: "socket-request", pattern: "@@/run/systemd/io.systemd.Coredump", want: coreSocketRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := corePatternMode(tc.pattern)
			if got != tc.want || got.guaranteed() != tc.guaranteed {
				t.Errorf("corePatternMode(%q) = %s (guaranteed %v), want %s (guaranteed %v)", tc.pattern, got, got.guaranteed(), tc.want, tc.guaranteed)
			}
		})
	}
}

// W49 T1, ADR 0042 A5: the target starts with RLIMIT_CORE exactly {1,1}, read by the target itself, on every launch path.
func TestTargetCoreLimitIsSentinel(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "bounded"}[bounded], func(t *testing.T) {
			s := spec("rlimits")
			if bounded {
				s.WatchDir, s.MaxDirBytes, s.WriteFiles = t.TempDir(), 8192, []string{"out"}
			}
			out, err := Run(t.Context(), s)
			if err != nil || out.ExitCode != 0 {
				t.Fatalf("Run = %v exit %d stderr %q", err, out.ExitCode, out.Stderr)
			}
			_, limits, _ := strings.Cut(string(out.Stdout), "core=")
			if got := strings.TrimSpace(limits); got != "1/1" {
				t.Errorf("target RLIMIT_CORE = %q (from %q), want 1/1: the kernel's no-helper sentinel, not raisable", got, out.Stdout)
			}
		})
	}
}

// W49 T2, ADR 0042 A5, D1: each attack still dies of the intended signal, and the
// kernel starts no core helper for it (PIDFD_INFO_COREDUMP, Linux 6.16).
func TestSandboxDeathStartsNoCoreHelper(t *testing.T) {
	requireGuaranteedCorePattern(t)
	for _, tc := range []struct {
		attack string
		signal syscall.Signal
	}{
		{attack: "arch", signal: unix.SIGSYS},
		{attack: "x32", signal: unix.SIGSYS},
		{attack: "xfsz", signal: unix.SIGXFSZ},
		{attack: "segv", signal: unix.SIGSEGV},
	} {
		t.Run(tc.attack, func(t *testing.T) {
			path, args, err := sandboxPrefix(sandboxConfig{WorkingDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(path, append(args, os.Args[0])...) // #nosec G204 -- our own launcher and test binary.
			cmd.Env = []string{"INKWELL_ATTACK=" + tc.attack}
			pidfd := -1
			cmd.SysProcAttr = &syscall.SysProcAttr{PidFD: &pidfd}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = unix.Close(pidfd) }()
			// The dead process is examined before it is reaped: its namespace PIDs and name, and the kernel's record.
			if err := waitDead(pidfd); err != nil {
				_ = cmd.Wait()
				t.Fatal(err)
			}
			nspid, comm := processIdentity(cmd.Process.Pid)
			before := coreDumpOf(pidfd)
			_ = cmd.Wait()
			after := coreDumpOf(pidfd)
			got := exitSignal(cmd.ProcessState)
			recordDeath(t, cmd.Process.Pid, nspid, comm, got, before)
			if got != int(tc.signal) {
				t.Fatalf("%s attack died of signal %d, want %d (%v): the kill is the security guarantee", tc.attack, got, tc.signal, tc.signal)
			}
			switch before {
			case CoreDumpUnknown:
				t.Skip("the kernel does not report PIDFD_INFO_COREDUMP (Linux 6.16): no core helper cannot be shown")
			case CoreDumpStarted:
				t.Fatalf("%s attack: the kernel started a core helper (PIDFD_COREDUMPED)", tc.attack)
			}
			if after == CoreDumpStarted {
				t.Errorf("after the reap the kernel reports a core helper, before it none")
			}
		})
	}
}

// waitDead blocks until the process of pidfd has ended, leaving it unreaped.
func waitDead(pidfd int) error {
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PIDFD, pidfd, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

// processIdentity reads the NSpid list (outermost first) and the command name of a live or unreaped process.
func processIdentity(pid int) (nspid []int, comm string) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		key, value, _ := strings.Cut(line, ":")
		switch key {
		case "Name":
			comm = strings.TrimSpace(value)
		case "NSpid":
			for field := range strings.FieldsSeq(value) {
				if n, err := strconv.Atoi(field); err == nil {
					nspid = append(nspid, n)
				}
			}
		}
	}
	return nspid, comm
}

// recordDeath appends a deliberate sandbox death to the manifest named by
// INKWELL_SANDBOX_DEATHS, which scripts/sandbox_no_coredump.py matches against
// the journal (plan, Gate SB1b, row J1). Without the variable it does nothing.
func recordDeath(t *testing.T, pid int, nspid []int, comm string, signal int, core CoreDump) {
	t.Helper()
	path := os.Getenv("INKWELL_SANDBOX_DEATHS")
	if path == "" || signal == 0 {
		return
	}
	line, err := json.Marshal(map[string]any{
		"test": t.Name(), "pid": pid, "nspid": nspid, "comm": comm, "signal": signal, "coredumped": core != CoreDumpNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 G703 -- test manifest named by the gate script.
	if err != nil {
		t.Fatalf("death manifest: %v", err)
	}
	_, err = f.Write(append(line, '\n'))
	if err = errors.Join(err, f.Close()); err != nil {
		t.Fatalf("death manifest: %v", err)
	}
}

func TestTargetCannotLowerCoreLimit(t *testing.T) {
	for _, api := range []string{"prlimit64", "setrlimit"} {
		t.Run(api, func(t *testing.T) {
			s := spec("lower-core:" + api)
			out, err := Run(t.Context(), s)
			if err != nil || out.ExitCode != 0 || !strings.Contains(string(out.Stdout), "core=1/1 protected") {
				t.Fatalf("target lowered core limit: err=%v exit=%d stdout=%q stderr=%q", err, out.ExitCode, out.Stdout, out.Stderr)
			}
		})
	}
}
