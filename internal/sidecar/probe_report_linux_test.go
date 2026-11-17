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
// It prints, in this order: the tier facts of the real N and F probe launches
// (ADR 0044 point 2) and the tier decided for each sidecar; Run on the test
// helper in "echo" mode, plain and with NoNetwork; and the Landlock system
// call for its ABI version and for a ruleset with TCP rules.
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

	target, err := probeTarget(nil)
	if err != nil {
		say("Q3", "no probe target: %v", err)
		return
	}
	facts := probeFacts(target)
	say("Q3", "tier facts: landlock=%d seccomp=%v ns=%v (err=%v errno=%s) f=%v (err=%v errno=%s)",
		facts.Landlock, facts.Seccomp, facts.NS, facts.NSErr, errnoName(facts.NSErr), facts.F, facts.FErr, errnoName(facts.FErr))
	for _, p := range sidecarPrograms() {
		decided, err := decideTier(p, facts)
		say("Q3", "decided tier for %s: %v err=%v", p.name, decided, err)
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

// TestSandboxProbeTiers is a measurement, not a test (block SB2, questions Q10
// and Q11 of scripts/sandbox-probe.sh): it runs the pinned typst and tinymist
// through Run, first in the tier this machine decides (Q10), then with the
// namespace-free tier forced (Q11: both probes answer « no »), and prints the
// outcome of each: a version, a compile of a small document to standard
// output, and tinymist's version. Inert unless INKWELL_SANDBOX_PROBE=1.
func TestSandboxProbeTiers(t *testing.T) {
	if os.Getenv("INKWELL_SANDBOX_PROBE") != "1" {
		return // a measurement, not a test: it asserts nothing, so there is nothing to skip
	}
	say := func(q, format string, args ...any) { fmt.Printf(q+": "+format+"\n", args...) }
	store := &Store{Dir: sharedCache}
	typst, terr := store.Resolve(Typst)
	tinymist, merr := store.Resolve(Tinymist)
	if terr != nil || merr != nil {
		say("Q10", "no embedded sidecar (typst: %v, tinymist: %v): build after `go run ./cmd/sidecar-fetch`", terr, merr)
		return
	}
	project := t.TempDir()
	doc := "#set page(width: 8cm, height: 6cm)\n= Probe\nText $x^2$ and a table:\n#table(columns: 2, [a], [b])\n"
	if err := os.WriteFile(filepath.Join(project, "main.typ"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	runs := func(q string, forceF bool) {
		for _, c := range []struct {
			label string
			spec  Spec
		}{
			{"typst --version", Spec{Path: typst, Dir: t.TempDir(), Args: []string{"--version"}}},
			{"typst compile to stdout", Spec{Path: typst, Dir: project, Args: []string{"compile", "--root", project, "--ignore-system-fonts", "main.typ", "-"}, MaxOutput: 1 << 20, NoNetwork: true}},
			{"tinymist --version", Spec{Path: tinymist, Dir: t.TempDir(), Args: []string{"--version"}, Env: []string{"HOME=" + t.TempDir()}, NoNetwork: true}},
		} {
			if forceF {
				c.spec.tier = tierF
			}
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			out, err := Run(ctx, c.spec)
			cancel()
			head := strings.TrimSpace(string(out.Stdout))
			if len(head) > 40 {
				head = head[:40]
			}
			say(q, "%s: err=%v errno=%s exit=%d signal=%d isolated=%v containment=%q stdout=%d bytes %q stderr=%q",
				c.label, err, errnoName(err), out.ExitCode, out.Signal, out.Isolated, out.Containment, len(out.Stdout), head,
				strings.TrimSpace(strings.ReplaceAll(string(out.Stderr), "\n", "|")))
		}
	}
	say("Q10", "process label=%s; the tier this machine decides:", procLabel())
	runs("Q10", false)

	say("Q11", "the namespace-free tier forced:")
	runs("Q11", true)
}
