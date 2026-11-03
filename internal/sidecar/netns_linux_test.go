package sidecar

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// swapIsolationProbe replaces the package network-isolation probe for one test.
func swapIsolationProbe(t *testing.T, probe func() (bool, error)) {
	t.Helper()
	saved := probeIsolation
	probeIsolation = probe
	t.Cleanup(func() { probeIsolation = saved })
}

func TestIsolationProbeHungChildRefusesLaunch(t *testing.T) {
	// A hung isolation probe refuses the NoNetwork launch (never runs it
	// proxy-only), caches nothing, and probes again after the backoff.
	const timeout = 200 * time.Millisecond
	clock := time.Unix(0, 0)
	calls := 0
	swapIsolationProbe(t, cachedProbe(func(ctx context.Context) (bool, error) {
		calls++
		if calls == 1 {
			return runNamespaceProbe(ctx, isolate, helperEnv("sleep"), os.Args[0])
		}
		return true, nil
	}, timeout, func() time.Time { return clock }))
	s := spec("echo")
	s.NoNetwork = true
	begin := time.Now()
	if out, err := Run(t.Context(), s); !errors.Is(err, ErrProbeInconclusive) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run with a hung isolation probe = %+v, %v; want ErrProbeInconclusive after the deadline", out, err)
	}
	if took := time.Since(begin); took > timeout+5*time.Second {
		t.Fatalf("refused after %v, want about %v", took, timeout)
	}
	if _, err := NetworkIsolation(); !errors.Is(err, ErrProbeInconclusive) {
		t.Fatalf("NetworkIsolation during the backoff: %v, want the same refusal", err)
	}
	clock = clock.Add(probeBackoffMax)
	if isolated, err := NetworkIsolation(); err != nil || !isolated || calls != 2 {
		t.Fatalf("after the backoff: %v, %v, %d probes; want a fresh, successful probe", isolated, err, calls)
	}
}

func TestIsolationProbeDefinitiveRefusalSelectsFallback(t *testing.T) {
	// A uid map the kernel rejects (EINVAL, for any caller) makes Start fail
	// the way a sysctl or AppArmor refusal does: that is conclusive.
	invalidMap := func(cmd *exec.Cmd) {
		isolate(cmd)
		cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 1<<32 - 1, HostID: os.Getuid(), Size: 1}}
	}
	calls := 0
	swapIsolationProbe(t, cachedProbe(func(ctx context.Context) (bool, error) {
		calls++
		return runNamespaceProbe(ctx, invalidMap, helperEnv("exit:0"), os.Args[0])
	}, time.Second, time.Now))
	s := spec("echo")
	s.NoNetwork = true
	for range 2 {
		out, err := Run(t.Context(), s)
		if err != nil || out.ExitCode != 0 || out.Isolated {
			t.Fatalf("Run after a definitive refusal = %+v, %v; want the proxy-only fallback", out, err)
		}
	}
	if calls != 1 {
		t.Fatalf("definitive refusal probed %d times, want it cached", calls)
	}
}
