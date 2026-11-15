package sidecar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// want is what ADR 0044 point 2 decides for one row of facts.
type want uint8

const (
	wantN want = iota + 1
	wantF
	wantRefuse
	wantRetry
)

func (w want) String() string {
	return [...]string{"?", "N", "F", "refuse", "retry"}[w]
}

// tierRow is one row of ADR 0044 point 2's table, written by hand from the
// ADR, with arm64 refused (plan S-7). A zero field matches anything.
type tierRow struct {
	landlockBelow3, landlockAtLeast3 bool
	seccomp                          *bool
	ns, f                            launchOutcome
	arch                             string
	out                              want
}

var (
	yes = new(true)
	no  = new(false)
)

// tierTable is checked first row first. arm64 comes first: it is refused per
// program whatever the facts until block SB2a2 proves its inventory.
var tierTable = []tierRow{
	{arch: "arm64", out: wantRefuse},
	{landlockBelow3: true, out: wantRefuse},
	{landlockAtLeast3: true, seccomp: no, out: wantRefuse},
	{landlockAtLeast3: true, seccomp: yes, ns: launchOK, out: wantN},
	{landlockAtLeast3: true, seccomp: yes, ns: launchRefused, f: launchOK, out: wantF},
	{landlockAtLeast3: true, seccomp: yes, ns: launchRefused, f: launchRefused, out: wantRefuse},
	{landlockAtLeast3: true, seccomp: yes, ns: launchInconclusive, out: wantRetry},
	{landlockAtLeast3: true, seccomp: yes, ns: launchRefused, f: launchInconclusive, out: wantRetry},
}

func oracle(f tierFacts) want {
	for _, r := range tierTable {
		switch {
		case r.arch != "" && r.arch != f.Arch,
			r.landlockBelow3 && f.Landlock >= 3,
			r.landlockAtLeast3 && f.Landlock < 3,
			r.seccomp != nil && *r.seccomp != f.Seccomp,
			r.ns != 0 && r.ns != f.NS,
			r.f != 0 && r.f != f.F:
			continue
		}
		return r.out
	}
	return 0
}

// T-1: decideTier is total over the facts of ADR 0044 point 2 and agrees with
// the hand table on all 180 combinations, for every registered program.
func TestDecideTierExhaustive(t *testing.T) {
	outcomes := []launchOutcome{launchOK, launchRefused, launchInconclusive}
	fErr := &fs.PathError{Op: "fork/exec", Path: "/proc/self/exe", Err: unix.EACCES}
	count := 0
	for _, p := range programs {
		count = 0
		for _, landlock := range []int{0, 2, 3, 6, 8} {
			for _, seccomp := range []bool{true, false} {
				for _, ns := range outcomes {
					for _, f := range outcomes {
						for _, arch := range []string{"amd64", "arm64"} {
							count++
							facts := tierFacts{Arch: arch, Landlock: landlock, Seccomp: seccomp, NS: ns, F: f, FErr: fErr}
							checkDecision(t, p, facts, oracle(facts))
						}
					}
				}
			}
		}
	}
	if count != 180 {
		t.Fatalf("T-1 tier: %d combinations, want 180", count)
	}
}

func checkDecision(t *testing.T, p program, facts tierFacts, w want) {
	t.Helper()
	got, err := decideTier(p, facts)
	label := fmt.Sprintf("%s %+v", p.name, facts)
	switch w {
	case wantN, wantF:
		wantTier := map[want]tier{wantN: tierN, wantF: tierF}[w]
		if err != nil || got != wantTier {
			t.Errorf("T-1 tier: %s: got %v, %v; want %v", label, got, err, w)
		}
	case wantRefuse:
		if got != tierNone || !errors.Is(err, ErrSandboxRefused) || errors.Is(err, ErrProbeInconclusive) {
			t.Errorf("T-1 tier: %s: got %v, %v; want a definitive refusal", label, got, err)
		}
		if facts.Arch == "amd64" && facts.Landlock >= 3 && facts.Seccomp && facts.F == launchRefused && !errors.Is(err, unix.EACCES) {
			t.Errorf("T-1 tier: %s: refusal %v does not name f's errno", label, err)
		}
	case wantRetry:
		if got != tierNone || !errors.Is(err, ErrProbeInconclusive) || errors.Is(err, ErrSandboxRefused) {
			t.Errorf("T-1 tier: %s: got %v, %v; want a retry (ErrProbeInconclusive)", label, got, err)
		}
	default:
		t.Fatalf("T-1 tier: %s: the hand table has no row", label)
	}
}

// S-7: arm64 is refused for typst and tinymist until block SB2a2 proves their
// inventory, even when every fact would give tier N; the refusal is the
// sandbox one (preview.fail.sandbox), not a retry.
func TestArm64Refused(t *testing.T) {
	for _, p := range programs {
		for _, ns := range []launchOutcome{launchOK, launchRefused} {
			got, err := decideTier(p, tierFacts{Arch: "arm64", Landlock: 8, Seccomp: true, NS: ns, F: launchOK})
			if got != tierNone || !errors.Is(err, ErrSandboxRefused) || errors.Is(err, ErrProbeInconclusive) {
				t.Errorf("S-7 arm64: %s with ns %v: got %v, %v; want ErrSandboxRefused", p.name, ns, got, err)
			}
			// The inventory reads a refusal from this key in the test output.
			t.Logf("refusal: preview.fail.sandbox: %v", err)
		}
	}
	// An architecture with no table at all is refused the same way.
	if _, err := decideTier(programs[0], tierFacts{Arch: "riscv64", Landlock: 8, Seccomp: true, NS: launchOK}); !errors.Is(err, ErrSandboxRefused) {
		t.Errorf("S-7 arm64: riscv64: %v, want ErrSandboxRefused", err)
	}
}

// S-7: an unproven architecture is refused before any probe launch. On arm64
// the probe's own target is not yet inventoried (its ppoll is refused), so a
// probe there comes back inconclusive and would hide the definitive refusal
// and its sentence; the architecture is injected so this runs on amd64.
func TestUnprovenArchitectureRefusedBeforeProbe(t *testing.T) {
	arch, facts := hostArch, sandboxFacts
	t.Cleanup(func() { hostArch, sandboxFacts = arch, facts })
	hostArch = "arm64"
	probed := 0
	sandboxFacts = func(*Store) (tierFacts, error) {
		probed++
		return tierFacts{}, fmt.Errorf("%w: the probe's ppoll was refused", ErrProbeInconclusive)
	}
	for _, p := range sidecarPrograms() {
		_, err := planLaunch(Spec{Path: "/opt/inkwell/" + p.name})
		if !errors.Is(err, ErrSandboxRefused) || errors.Is(err, ErrProbeInconclusive) {
			t.Errorf("S-7 arm64: planLaunch(%s) = %v, want ErrSandboxRefused", p.name, err)
		}
		t.Logf("refusal: preview.fail.sandbox: %v", err)
	}
	if _, err := networkIsolation(nil); !errors.Is(err, ErrSandboxRefused) || errors.Is(err, ErrProbeInconclusive) {
		t.Errorf("S-7 arm64: networkIsolation = %v, want ErrSandboxRefused", err)
	}
	if probed != 0 {
		t.Errorf("S-7 arm64: %d probe launches before the refusal, want none", probed)
	}
}

// errnoRow is one errno a namespace launch can return, from the ERRORS of
// clone(2), unshare(2), mount(2), user_namespaces(7) (the id-map writes) and
// execve(2) (the launcher's /proc/self/exe re-execution).
type errnoRow struct {
	errno unix.Errno
	from  string
	class launchOutcome
}

// T-2: every errno of the namespace launch has its class, written by hand from
// ADR 0044 point 2: EPERM, EACCES, EINVAL and ENOSPC with
// user.max_user_namespaces = 0 are a refusal; everything else is inconclusive.
func TestRefusalErrnoTable(t *testing.T) {
	r, i := launchRefused, launchInconclusive
	rows := []errnoRow{
		// clone(2)
		{unix.EAGAIN, "clone", i},
		{unix.EBUSY, "clone", i},
		{unix.EEXIST, "clone", i},
		{unix.EINVAL, "clone", r},
		{unix.ENOMEM, "clone", i},
		{unix.EOPNOTSUPP, "clone", i},
		{unix.EPERM, "clone", r},
		{unix.EUSERS, "clone", i},
		{unix.EFAULT, "clone", i},
		// unshare(2)
		{unix.EINVAL, "unshare", r},
		{unix.ENOMEM, "unshare", i},
		{unix.EPERM, "unshare", r},
		{unix.EUSERS, "unshare", i},
		// mount(2)
		{unix.EACCES, "mount", r},
		{unix.EBUSY, "mount", i},
		{unix.EFAULT, "mount", i},
		{unix.EINVAL, "mount", r},
		{unix.ELOOP, "mount", i},
		{unix.EMFILE, "mount", i},
		{unix.ENAMETOOLONG, "mount", i},
		{unix.ENODEV, "mount", i},
		{unix.ENOENT, "mount", i},
		{unix.ENOMEM, "mount", i},
		{unix.ENOTBLK, "mount", i},
		{unix.ENOTDIR, "mount", i},
		{unix.ENXIO, "mount", i},
		{unix.EPERM, "mount", r},
		{unix.EROFS, "mount", i},
		// id-map writes, user_namespaces(7)
		{unix.EPERM, "uid_map", r},
		{unix.EINVAL, "uid_map", r},
		{unix.EFAULT, "uid_map", i},
		// execve(2) of the launcher: EACCES measured on stock Ubuntu 24.04 (Q3)
		{unix.EACCES, "execve", r},
		{unix.ENOENT, "execve", i},
		{unix.ENOEXEC, "execve", i},
		{unix.ETXTBSY, "execve", i},
		{unix.E2BIG, "execve", i},
		{unix.EIO, "execve", i},
		{unix.EISDIR, "execve", i},
		{unix.ELIBBAD, "execve", i},
		{unix.ENFILE, "execve", i},
		{unix.EMFILE, "execve", i},
		{unix.ENAMETOOLONG, "execve", i},
		{unix.ENOTDIR, "execve", i},
		{unix.ELOOP, "execve", i},
		{unix.ENOMEM, "execve", i},
		{unix.EFAULT, "execve", i},
	}
	for _, row := range rows {
		if got := namespaceErrnoClass(row.errno, false); got != row.class {
			t.Errorf("T-2 errno: %s %v: class %v, want %v", row.from, row.errno, got, row.class)
		}
		// The sysctl matters for ENOSPC only.
		if got := namespaceErrnoClass(row.errno, true); got != row.class {
			t.Errorf("T-2 errno: %s %v with namespaces disabled: class %v, want %v", row.from, row.errno, got, row.class)
		}
	}
	for _, from := range []string{"clone", "unshare"} {
		if got := namespaceErrnoClass(unix.ENOSPC, true); got != r {
			t.Errorf("T-2 errno: %s ENOSPC with user.max_user_namespaces = 0: %v, want refused", from, got)
		}
		if got := namespaceErrnoClass(unix.ENOSPC, false); got != i {
			t.Errorf("T-2 errno: %s ENOSPC below a non-zero limit: %v, want inconclusive", from, got)
		}
	}
	// Total: every errno number has a class, and only the four above refuse.
	for e := unix.Errno(1); e < 4096; e++ {
		got := namespaceErrnoClass(e, false)
		wantClass := i
		if e == unix.EPERM || e == unix.EACCES || e == unix.EINVAL {
			wantClass = r
		}
		if got != wantClass {
			t.Errorf("T-2 errno: %v (%d): class %v, want %v", e, e, got, wantClass)
		}
	}
}

// T-2, through the errors a launch returns: the errno is found inside the
// wrapping; a timeout, an exit status or any non-errno error is inconclusive.
func TestNamespaceOutcome(t *testing.T) {
	saved := userNamespaceLimit
	t.Cleanup(func() { userNamespaceLimit = saved })
	limit := 0
	userNamespaceLimit = func() (int, error) { return limit, nil }
	path := "/proc/self/exe"
	for _, c := range []struct {
		err  error
		want launchOutcome
	}{
		{nil, launchOK},
		{&fs.PathError{Op: "fork/exec", Path: path, Err: unix.EACCES}, launchRefused},
		{fmt.Errorf("probe: %w", &fs.PathError{Op: "fork/exec", Path: path, Err: unix.EPERM}), launchRefused},
		{&fs.PathError{Op: "fork/exec", Path: path, Err: unix.ENOSPC}, launchRefused},
		{&fs.PathError{Op: "fork/exec", Path: path, Err: unix.EAGAIN}, launchInconclusive},
		{context.DeadlineExceeded, launchInconclusive},
		{fmt.Errorf("probe: %w", context.Canceled), launchInconclusive},
		{&exec.ExitError{}, launchInconclusive},
		{errors.New("crash"), launchInconclusive},
	} {
		if got := namespaceOutcome(c.err); got != c.want {
			t.Errorf("T-2 errno: %v: %v, want %v", c.err, got, c.want)
		}
	}
	limit = 63452
	if got := namespaceOutcome(&fs.PathError{Op: "fork/exec", Path: path, Err: unix.ENOSPC}); got != launchInconclusive {
		t.Errorf("T-2 errno: ENOSPC below a non-zero limit: %v, want inconclusive", got)
	}
	userNamespaceLimit = func() (int, error) { return 0, fs.ErrNotExist }
	if got := namespaceOutcome(&fs.PathError{Op: "fork/exec", Path: path, Err: unix.ENOSPC}); got != launchInconclusive {
		t.Errorf("T-2 errno: ENOSPC with an unreadable limit: %v, want inconclusive", got)
	}
}

// T-4: the tier report is the launcher's word on its confinement, so a line
// that is not exactly what the launcher writes is refused before « proceed »:
// each namespace is its kind's identity (`kind:[inode]`, in the order user,
// mnt, pid, ipc, net), seccomp is a mode of /proc/self/status (0 to 2) and the
// Landlock ABI a small canonical number.
func TestTierReportRefusesMalformedFields(t *testing.T) {
	own, err := ownNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	valid := "tier=F ns=" + strings.Join(own[:], ",") + " proc=true landlock=3 seccomp=2\n"
	if r, err := parseTierReport(valid); err != nil || r.namespaces != own || r.seccomp != 2 || r.landlock != 3 {
		t.Fatalf("parseTierReport(%q) = %+v, %v", valid, r, err)
	}
	ns := func(ids ...string) string { return strings.Join(ids, ",") }
	good := ns("user:[1]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]")
	line := func(ns, landlock, seccomp string) string {
		return "tier=N ns=" + ns + " proc=true landlock=" + landlock + " seccomp=" + seccomp + "\n"
	}
	if _, err := parseTierReport(line(good, "3", "2")); err != nil {
		t.Fatalf("well-formed report refused: %v", err)
	}
	for name, report := range map[string]string{
		"bare names":            line("a,b,c,d,e", "3", "2"),
		"kinds out of order":    line(ns("mnt:[2]", "user:[1]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"a kind repeated":       line(ns("user:[1]", "user:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"no brackets":           line(ns("user:1", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"inode not a number":    line(ns("user:[x]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"inode zero":            line(ns("user:[0]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"inode signed":          line(ns("user:[+1]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"inode leading zero":    line(ns("user:[01]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"inode beyond 64 bits":  line(ns("user:[18446744073709551616]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"trailing text":         line(ns("user:[1]x", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"cgroup namespace":      line(ns("cgroup:[1]", "mnt:[2]", "pid:[3]", "ipc:[4]", "net:[5]"), "3", "2"),
		"seccomp beyond 2":      line(good, "3", "3"),
		"seccomp negative":      line(good, "3", "-1"),
		"seccomp signed":        line(good, "3", "+2"),
		"landlock negative":     line(good, "-1", "2"),
		"landlock signed":       line(good, "+3", "2"),
		"landlock leading zero": line(good, "03", "2"),
		"landlock out of range": line(good, "4294967296", "2"),
	} {
		t.Run(name, func(t *testing.T) {
			if r, err := parseTierReport(report); err == nil {
				t.Fatalf("parseTierReport(%q) accepted %+v", report, r)
			}
		})
	}

	t.Run("refused before proceed", func(t *testing.T) {
		hs, err := newHandshake(exec.Command("true"), launchPlan{prog: testProgram, tier: tierN})
		if err != nil {
			t.Fatal(err)
		}
		defer hs.close()
		child := hs.proceedChild
		hs.proceedChild = nil // await closes the child ends; this test reads « proceed » from it
		if _, err := hs.reportChild.WriteString(line("a,b,c,d,e", "3", "2")); err != nil {
			t.Fatal(err)
		}
		if err := hs.await(t.Context()); err == nil {
			t.Fatal("await accepted a malformed report")
		}
		_ = hs.proceed.Close()
		got, _ := io.ReadAll(child)
		_ = child.Close()
		if len(got) != 0 {
			t.Fatalf("« proceed » sent after a malformed report: %q", got)
		}
	})
}
