package sidecar

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runpathHelper builds a dynamically linked C program whose one library is
// found only through its DT_RUNPATH, as the libraries of the real application
// are (WebKitGTK needs libpulse, whose libpulsecommon lives in a RUNPATH
// folder).
func runpathHelper(t *testing.T) string {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatalf("UX-SB2a1-1 probe target: a C compiler is required (cgo is, for the application): %v", err)
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "private")
	if err := os.Mkdir(lib, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, src string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	build := func(args ...string) {
		if out, err := exec.CommandContext(t.Context(), cc, args...).CombinedOutput(); err != nil {
			t.Fatalf("UX-SB2a1-1 probe target: cc %v: %v\n%s", args, err, out)
		}
	}
	build("-shared", "-fPIC", "-o", filepath.Join(lib, "libinkwellprobedep.so"), write("dep.c", "int inkwell_probe_dep(void) { return 0; }\n"))
	helper := filepath.Join(dir, "helper")
	build("-o", helper, write("main.c", "int inkwell_probe_dep(void);\nint main(void) { return inkwell_probe_dep(); }\n"),
		"-L"+lib, "-linkwellprobedep", "-Wl,--enable-new-dtags,-rpath,"+lib)
	return helper
}

// UX-SB2a1-1: the probe's target was this executable, which in the application
// is linked to libraries found through a DT_RUNPATH; the launcher refused it
// (« missing dynamic dependency ») before its tier report, and every probe was
// inconclusive. The target is now the pinned typst (TestProbeTargetIsTheVerifiedTypst);
// a program with a RUNPATH library must still launch in both tiers.
func TestProbeTargetWithRunpathLibrary(t *testing.T) {
	if !requireLaunchFor(t, "UX-SB2a1-1 probe target") {
		return
	}
	helper := runpathHelper(t)
	for _, tr := range []tier{tierN, tierF} {
		if tr == tierN && machineTier(t) != tierN {
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*probeTimeout)
		err := runProbe(ctx, helper, tr)
		cancel()
		if err != nil {
			t.Errorf("UX-SB2a1-1 probe target: %s: %v", tr, err)
		}
	}
}

// UX-SB2a1-2: a launcher that stops before its tier report says why on its
// standard error; the launch error carries that cause.
func TestLaunchFailureCarriesLauncherCause(t *testing.T) {
	if !requireLaunchFor(t, "UX-SB2a1-2 launch cause") {
		return
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "not-a-program")
	if err := os.WriteFile(target, []byte("not an ELF file\n"), 0o700); err != nil { // #nosec G306 -- an executable test file.
		t.Fatal(err)
	}
	spec := Spec{Path: target, Dir: dir, Env: []string{}}
	if err := validateLaunch(t.Context(), &spec); err != nil {
		t.Fatal(err)
	}
	_, err := run(t.Context(), spec, testPlan(tierF), newDirectoryMount)
	if err == nil || !strings.Contains(err.Error(), "bad magic number") {
		t.Fatalf("UX-SB2a1-2 launch cause: got %v, want the launcher's own cause (bad magic number)", err)
	}
}

// UX-SB2a1-2: an inconclusive decision names why the probe launch was
// inconclusive, so that the log line can be diagnosed.
func TestInconclusiveDecisionNamesItsCause(t *testing.T) {
	cause := errors.New("no tier report: missing dynamic dependency libx.so")
	for _, f := range []tierFacts{
		{Arch: "amd64", Landlock: 3, Seccomp: true, NS: launchInconclusive, NSErr: cause},
		{Arch: "amd64", Landlock: 3, Seccomp: true, NS: launchRefused, F: launchInconclusive, FErr: cause},
	} {
		_, err := decideTier(program{name: "probe", proven: []string{"amd64"}}, f)
		if !errors.Is(err, ErrProbeInconclusive) || !strings.Contains(err.Error(), cause.Error()) {
			t.Errorf("UX-SB2a1-2 probe cause: %+v: got %v, want ErrProbeInconclusive naming %q", f, err, cause)
		}
	}
}
