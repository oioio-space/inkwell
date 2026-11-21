package sidecar

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// probeTypst is the probe's real target: the pinned typst, verified in the
// test cache.
func probeTypst(t *testing.T) string {
	t.Helper()
	target, err := probeTarget(nil)
	if err != nil {
		t.Fatalf("T-3 probe target: %v", err)
	}
	return target
}

// ADR 0044 point 2 (amended 2026-11-15): the probe's target is the pinned
// typst of the caller's store, verified before the probe as before every
// launch: a modified copy is replaced, never run.
func TestProbeTargetIsTheVerifiedTypst(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	want, err := store.Resolve(Typst)
	if err != nil {
		t.Fatalf("T-3 probe target: %v", err)
	}
	if err := os.Chmod(want, 0o700); err != nil { // #nosec G302 -- to tamper with the test's own copy.
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("not typst\n"), 0o700); err != nil { // #nosec G306 -- an executable test file.
		t.Fatal(err)
	}
	got, err := probeTarget(store)
	if err != nil || got != want {
		t.Fatalf("T-3 probe target: probeTarget = %q, %v; want the store's typst %q", got, err, want)
	}
	m, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	_, art, err := m.Artifact(Typst, HostTarget())
	if err != nil {
		t.Fatal(err)
	}
	if problem := verify(got, art); problem != "" {
		t.Errorf("T-3 probe target: the target was not verified before the probe: %s", problem)
	}
}

// A target that cannot be verified (a build without its sidecars) is that
// error: no probe is launched, and it is neither « inconclusive, retry » nor
// remembered.
func TestUnverifiableProbeTargetIsNotInconclusive(t *testing.T) {
	saved := probeTarget
	t.Cleanup(func() { probeTarget = saved })
	fail := true
	probeTarget = func(*Store) (string, error) {
		if fail {
			return "", ErrNotEmbedded
		}
		return "/verified/typst", nil
	}
	var probed []string
	facts := cachedFacts(func(target string) tierFacts {
		probed = append(probed, target)
		return tierFacts{Arch: hostArch, Landlock: minLandlockABI, Seccomp: true, NS: launchOK}
	}, time.Now)
	if _, err := facts(nil); !errors.Is(err, ErrNotEmbedded) || errors.Is(err, ErrProbeInconclusive) || len(probed) != 0 {
		t.Fatalf("T-3 probe target: got %v after %d probes, want ErrNotEmbedded and no probe", err, len(probed))
	}
	fail = false
	if f, err := facts(nil); err != nil || f.NS != launchOK || len(probed) != 1 || probed[0] != "/verified/typst" {
		t.Fatalf("T-3 probe target: after the target is back: facts %+v, %v, probes %v", f, err, probed)
	}
}

// T-3, ADR 0044 point 2 (amended 2026-11-15): the probe launches its target as
// `--version`, under typst's own descriptor, in both tiers.
func TestProbeRunsVersionUnderTypstDescriptor(t *testing.T) {
	if !requireLaunchFor(t, "T-3 probe target") {
		return
	}
	helper := probeFixture(t.Context(), t, "version")
	for _, tr := range []tier{tierN, tierF} {
		if tr == tierN && machineTier(t) != tierN {
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*probeTimeout)
		err := runProbe(ctx, helper, tr)
		cancel()
		if err != nil {
			t.Errorf("T-3 probe target: %s: %v", tr, err)
		}
	}
}
