package sidecar

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fixedProbeTarget makes the probe's target a path no probe of these tests runs.
func fixedProbeTarget(t *testing.T) {
	t.Helper()
	saved := probeTarget
	t.Cleanup(func() { probeTarget = saved })
	probeTarget = func(*Store) (string, error) { return "/verified/typst", nil }
}

// ADR 0044 point 2: conclusive facts are gathered once per process, whatever
// they decide (N, F or a refusal).
func TestTierFactsConclusiveProbedOnce(t *testing.T) {
	fixedProbeTarget(t)
	for name, want := range map[string]tierFacts{
		"N":       {Arch: hostArch, Landlock: minLandlockABI, Seccomp: true, NS: launchOK},
		"F":       {Arch: hostArch, Landlock: minLandlockABI, Seccomp: true, NS: launchRefused, F: launchOK},
		"refused": {Arch: hostArch, Landlock: minLandlockABI, Seccomp: true, NS: launchRefused, F: launchRefused},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			facts := cachedFacts(func(string) tierFacts { calls++; return want }, time.Now)
			for range 3 {
				if got, err := facts(nil); err != nil || got != want {
					t.Fatalf("ADR 0044 point 2 facts kept: facts = %+v, %v; want %+v", got, err, want)
				}
			}
			if calls != 1 {
				t.Fatalf("ADR 0044 point 2 facts kept: conclusive facts probed %d times, want once", calls)
			}
		})
	}
}

// ADR 0044 point 2: inconclusive facts are never kept; the refusal is repeated
// without a probe during the backoff, which doubles, and the next probe after
// it decides.
func TestTierFactsInconclusiveRetriedAfterBackoff(t *testing.T) {
	fixedProbeTarget(t)
	calls := 0
	clock := time.Unix(0, 0)
	conclusive := tierFacts{Arch: hostArch, Landlock: minLandlockABI, Seccomp: true, NS: launchOK}
	facts := cachedFacts(func(string) tierFacts {
		calls++
		if calls <= 2 {
			f := conclusive
			f.NS, f.NSErr = launchInconclusive, context.DeadlineExceeded
			return f
		}
		return conclusive
	}, func() time.Time { return clock })
	refused := func() {
		t.Helper()
		if got, err := facts(nil); !errors.Is(err, ErrProbeInconclusive) || got != (tierFacts{}) {
			t.Fatalf("ADR 0044 point 2 retry: facts = %+v, %v; want a refusal wrapping ErrProbeInconclusive", got, err)
		}
	}
	refused()
	refused()
	if calls != 1 {
		t.Fatalf("ADR 0044 point 2 retry: %d probes, want the refusal repeated without probing during the backoff", calls)
	}
	clock = clock.Add(probeBackoffMin)
	refused()
	clock = clock.Add(2*probeBackoffMin - time.Nanosecond)
	refused()
	if calls != 2 {
		t.Fatalf("ADR 0044 point 2 retry: %d probes, want one retry per elapsed, doubled backoff", calls)
	}
	clock = clock.Add(time.Nanosecond)
	if got, err := facts(nil); err != nil || got != conclusive || calls != 3 {
		t.Fatalf("ADR 0044 point 2 retry: facts = %+v, %v after %d probes; want the inconclusive answer never kept", got, err, calls)
	}
	if _, err := facts(nil); err != nil || calls != 3 {
		t.Fatalf("ADR 0044 point 2 retry: %v after %d probes; want the conclusive answer kept", err, calls)
	}
}

// ADR 0044 point 2: the backoff is capped; without the cap the doubling would
// outgrow probeBackoffMax by the sixth failure.
func TestTierFactsBackoffIsCapped(t *testing.T) {
	fixedProbeTarget(t)
	clock := time.Unix(0, 0)
	calls := 0
	facts := cachedFacts(func(string) tierFacts {
		calls++
		return tierFacts{Arch: hostArch, Landlock: minLandlockABI, Seccomp: true, NS: launchInconclusive}
	}, func() time.Time { return clock })
	for range 20 {
		_, _ = facts(nil)
		clock = clock.Add(probeBackoffMax)
	}
	if calls != 20 {
		t.Fatalf("ADR 0044 point 2 retry: %d probes in 20 maximal backoffs, want 20", calls)
	}
}

// ADR 0044 point 2: a probe launch whose target hangs ends at its deadline and
// is inconclusive in both tiers, never a refusal nor a decided tier.
func TestHungProbeLaunchIsInconclusive(t *testing.T) {
	if !requireLaunchFor(t, "ADR 0044 point 2 hung probe") {
		return
	}
	helper := probeFixture(t, t.Context(), "spin")
	const deadline = 300 * time.Millisecond
	for _, tr := range []tier{tierN, tierF} {
		if tr == tierN && machineTier(t) != tierN {
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), deadline)
		begin := time.Now()
		err := runProbe(ctx, helper, tr)
		took := time.Since(begin)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) || probeOutcome(tr, err) != launchInconclusive {
			t.Errorf("ADR 0044 point 2 hung probe: %s: %v (outcome %v); want the deadline, inconclusive", tr, err, probeOutcome(tr, err))
		}
		if took > deadline+probeTimeout {
			t.Errorf("ADR 0044 point 2 hung probe: %s: ended after %v, want about %v", tr, took, deadline)
		}
	}
}
