package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TestSandboxProbeReport is a measurement, not a test: it prints what this
// machine lets the sandbox do, one "Qn: ..." line per fact, and never fails on
// an answer. It is inert unless INKWELL_SANDBOX_PROBE=1
// (scripts/sandbox-probe.sh, .github/workflows/sandbox-probe.yml; block SB2).
//
// It calls, in this order: the two cached probes the launcher decides with
// (probeOutputMount, probeIsolation); the same probe child through
// runNamespaceProbe with each namespace set, to read the raw error; Run on the
// test helper in "echo" mode, plain and with NoNetwork; and the Landlock
// system call for its ABI version and for a ruleset with TCP rules.
func TestSandboxProbeReport(t *testing.T) {
	if os.Getenv("INKWELL_SANDBOX_PROBE") != "1" {
		return // a measurement, not a test: it asserts nothing, so there is nothing to skip
	}
	say := func(q, format string, args ...any) { fmt.Printf(q+": "+format+"\n", args...) }

	say("Q3", "process label=%s", procLabel())
	for _, child := range labelChildren(t) {
		out, err := exec.CommandContext(t.Context(), child, "/proc/self/attr/current").CombinedOutput() // #nosec G204 -- cat, or a copy of it.
		say("Q3", "child %s label=%s err=%v", child, strings.Trim(string(out), "\x00\n "), err)
	}

	ok, err := probeOutputMount()
	say("Q3", "probeOutputMount supported=%v err=%v errno=%s", ok, err, errnoName(err))
	ok, err = probeIsolation()
	say("Q3", "probeIsolation supported=%v err=%v errno=%s", ok, err, errnoName(err))

	path, args, err := sandboxPrefix(sandboxConfig{Probe: true})
	if err != nil {
		say("Q3", "sandboxPrefix err=%v", err)
	} else {
		for _, ns := range []struct {
			name string
			set  func(*exec.Cmd)
		}{{"mount namespaces", mountNamespace}, {"user+net+pid+ipc+mount namespaces", isolate}} {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			ok, err := runNamespaceProbe(ctx, ns.set, []string{}, path, args...)
			cancel()
			say("Q3", "raw probe %s: supported=%v err=%v errno=%s definitive=%v", ns.name, ok, err, errnoName(err), err != nil && definitiveRefusal(err))
		}
	}

	for _, noNetwork := range []bool{false, true} {
		s := deathSpec(t, "echo")
		s.NoNetwork = noNetwork
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		out, err := Run(ctx, s)
		cancel()
		say("Q3", "Run echo NoNetwork=%v: err=%v errno=%s exit=%d signal=%d isolated=%v containment=%q stderr=%q",
			noNetwork, err, errnoName(err), out.ExitCode, out.Signal, out.Isolated, out.Containment, strings.TrimSpace(string(out.Stderr)))
	}

	const versionFlag = 1 // LANDLOCK_CREATE_RULESET_VERSION
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, versionFlag)
	if errno != 0 {
		say("Q4", "landlock ABI: unavailable errno=%s", unix.ErrnoName(errno))
	} else {
		say("Q4", "landlock ABI=%d", abi)
	}
	// A ruleset that handles TCP bind and connect: accepted from ABI 4 on.
	attr := struct{ fs, net uint64 }{fs: 1, net: 3}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0) // #nosec G103 -- the kernel reads a fixed-size struct.
	if errno != 0 {
		say("Q4", "landlock ruleset with TCP rules: refused errno=%s", unix.ErrnoName(errno))
	} else {
		say("Q4", "landlock ruleset with TCP rules: accepted")
		_ = unix.Close(int(fd))
	}
}

// procLabel is the AppArmor (or other LSM) label of this process.
func procLabel() string {
	b, err := os.ReadFile("/proc/self/attr/current")
	if err != nil {
		return "unreadable: " + err.Error()
	}
	return strings.Trim(string(b), "\x00\n ")
}

// labelChildren returns cat and a copy of it in a cache-like folder, as a
// sidecar extracted at run time would be: each prints the label it runs under.
func labelChildren(t *testing.T) []string {
	cat, err := exec.LookPath("cat")
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(cat) // #nosec G304 -- the system's cat.
	if err != nil {
		return []string{cat}
	}
	extracted := filepath.Join(t.TempDir(), "extracted-cat")
	if err := os.WriteFile(extracted, b, 0o700); err != nil { // #nosec G306 -- an executable copy, private to the user.
		return []string{cat}
	}
	return []string{cat, extracted}
}

// errnoName names the errno inside err, or "-" when there is none.
func errnoName(err error) string {
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		return unix.ErrnoName(errno)
	}
	return "-"
}
