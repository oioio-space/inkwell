package sidecar

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// testProgram is the test binary, whose helper modes are the package's fake
// sidecars: the sidecars' table, and PR_SET_SECCOMP so that a helper can stack
// a filter of its own (the audit architecture tests); no ioctl.
var testProgram = program{
	name:     filepath.Base(os.Args[0]),
	syscalls: sidecarSyscalls,
	proven:   []string{"amd64"},
	prctl:    []uint32{unix.PR_SET_SECCOMP},
}

func init() { programs = append(programs, testProgram) }

// sidecarPrograms are the registered descriptors without the test binary's.
func sidecarPrograms() []program {
	var out []program
	for _, p := range programs {
		if p.name != testProgram.name {
			out = append(out, p)
		}
	}
	return out
}

// testPlan launches the test binary in tier t.
func testPlan(t tier) launchPlan { return launchPlan{prog: testProgram, tier: t} }

// hostTestFilter is the test program's filter on this host.
func hostTestFilter(t tier) ([]unix.SockFilter, error) {
	a, err := hostSysArch()
	if err != nil {
		return nil, err
	}
	return seccompFilter(testProgram, a, uint32(unix.Getpid()), t) // #nosec G115 -- positive pid.
}

// machineTier is this machine's decided tier for the test program; it logs
// the tier report line every sidecar test prints (scripts/gojson_check.py):
// `tier-report: tier=<N|F|refused> n=<available|unavailable> [errno=X]`.
func machineTier(t *testing.T) tier {
	t.Helper()
	facts, err := sandboxFacts(nil)
	if err != nil {
		t.Fatalf("tier probe: %v", err)
	}
	decided, err := decideTier(testProgram, facts)
	name := "refused"
	if err == nil {
		name = decided.String()
	}
	n := "unavailable"
	if facts.NS == launchOK {
		n = "available"
	}
	line := fmt.Sprintf("tier-report: tier=%s n=%s", name, n)
	if e, ok := errors.AsType[unix.Errno](facts.NSErr); ok {
		line += " errno=" + unix.ErrnoName(e)
	}
	t.Log(line)
	if err != nil {
		if !errors.Is(err, ErrSandboxRefused) {
			t.Fatalf("tier decision: %v", err)
		}
		t.Logf("refusal: preview.fail.sandbox: %v", err)
	}
	return decided
}

// requireN skips, with the tier report, where tier N is unavailable; on a
// refused machine it returns false and the test passes with the refusal.
func requireN(t *testing.T) bool {
	t.Helper()
	switch machineTier(t) {
	case tierNone:
		return false
	case tierF:
		t.Skip("tier N unavailable on this machine (tier-report n=unavailable)")
	}
	return true
}

// forksInSandbox reports whether a target may create a process on this
// machine: tier F creates none (S-2).
func forksInSandbox(t *testing.T) bool {
	t.Helper()
	return machineTier(t) == tierN
}

// requireLaunch returns false on a refused machine: the test then passes with
// the refusal sentence in its output.
func requireLaunch(t *testing.T) bool {
	t.Helper()
	return machineTier(t) != tierNone
}

// requireTier is requireN for tier N and requireLaunch for tier F.
func requireTier(t *testing.T, tr tier) bool {
	t.Helper()
	if tr == tierN {
		return requireN(t)
	}
	return requireLaunch(t)
}

// requireLaunchFor is requireLaunch for a test whose rule is about the launch
// itself: a probe that cannot conclude is that rule's failure, reported under
// its tag, not a setup error.
func requireLaunchFor(t *testing.T, rule string) bool {
	t.Helper()
	if _, err := sandboxFacts(nil); err != nil {
		t.Fatalf("%s: the launch did not start: %v", rule, err)
	}
	return requireLaunch(t)
}

// launcherConfig completes c, for a launcher a test starts by hand, with the
// test program's lists in tier F (such a command gets no namespace).
func launcherConfig(c sandboxConfig) sandboxConfig {
	c.Tier, c.Program = tierF.String(), testProgram.name
	c.Syscalls, c.Prctl, c.Ioctl = testProgram.syscalls[runtime.GOARCH], testProgram.prctl, testProgram.ioctl
	return c
}

// startLauncher starts a hand-built launcher command with the launch
// handshake, as run does.
func startLauncher(t *testing.T, cmd *exec.Cmd) error {
	t.Helper()
	hs, err := newHandshake(cmd, testPlan(tierF))
	if err != nil {
		return err
	}
	t.Cleanup(hs.close)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := hs.await(t.Context()); err != nil {
		return stopLaunch(cmd, err)
	}
	return nil
}

// combinedLaunch is cmd.CombinedOutput through the launch handshake.
func combinedLaunch(t *testing.T, cmd *exec.Cmd) ([]byte, error) {
	t.Helper()
	var b bytes.Buffer
	cmd.Stdout, cmd.Stderr = &b, &b
	if err := startLauncher(t, cmd); err != nil {
		return b.Bytes(), err
	}
	err := cmd.Wait()
	return b.Bytes(), err
}

// pidNamespace is the inode of pid's PID namespace, 0 when it cannot be read.
func pidNamespace(pid int) int {
	link, _ := os.Readlink(fmt.Sprintf("/proc/%d/ns/pid", pid))
	inode, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(link, "pid:["), "]"))
	return inode
}

// phasePID is the pid of a « <word> <pid> » barrier line, 0 unless the line is
// exactly that with word.
func phasePID(line, word string) int {
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[0] != word {
		return 0
	}
	pid, _ := strconv.Atoi(fields[1])
	return max(pid, 0)
}

// readyWriter signals ready once the output so far satisfies seen.
type readyWriter struct {
	ready chan struct{}
	seen  func(string) bool
	text  string
}

func (w *readyWriter) Write(p []byte) (int, error) {
	w.text += string(p)
	if w.seen(w.text) {
		select {
		case w.ready <- struct{}{}:
		default:
		}
	}
	return len(p), nil
}

// targetStarted: the first output line is the « target <pid> » of target-sleep.
func targetStarted(text string) bool {
	line, _, complete := strings.Cut(text, "\n")
	return complete && strings.HasPrefix(line, "target ")
}
