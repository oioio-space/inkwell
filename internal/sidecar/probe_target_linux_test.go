package sidecar

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// UX-SB2a1-1: the probe's target was this executable, which in the application
// is linked to libraries found through a DT_RUNPATH; the launcher refused it
// (« missing dynamic dependency ») before its tier report, and every probe was
// inconclusive. The target is now the pinned typst (TestProbeTargetIsTheVerifiedTypst);
// a program with a RUNPATH library must still launch in both tiers.
func TestProbeTargetWithRunpathLibrary(t *testing.T) {
	if !requireLaunchFor(t, "UX-SB2a1-1 probe target") {
		return
	}
	helper := probeFixture(t, t.Context(), "runpath")
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
