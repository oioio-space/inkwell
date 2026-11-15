package sidecar

import (
	"bytes"
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

	"golang.org/x/sys/unix"
)

func TestProbeIsTheLaunch(t *testing.T) {
	if !requireLaunchFor(t, "T-3 probe is the launch") {
		return
	}
	target := probeTypst(t)
	for _, tr := range []tier{tierN, tierF} {
		if tr == tierN && machineTier(t) != tierN {
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*probeTimeout)
		err := runProbe(ctx, target, tr)
		cancel()
		if err != nil {
			t.Fatalf("T-3 probe is the launch: %s: %v", tr, err)
		}
	}
}

func TestNoCapabilityNamespaceGivesF(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	saved := probeLaunch
	t.Cleanup(func() { probeLaunch = saved })
	var steps []tier
	probeLaunch = func(_ string, tr tier) error {
		steps = append(steps, tr)
		if tr == tierN {
			return fmt.Errorf("id-map write: %w", unix.EACCES)
		}
		return nil
	}
	if landlockABI() < 3 {
		t.Fatal("T-3 probe is the launch: Landlock ABI 3 required")
	}
	f := probeFacts("")
	got, err := decideTier(testProgram, f)
	if err != nil || got != tierF || len(steps) != 2 || steps[0] != tierN || steps[1] != tierF {
		t.Fatalf("T-3 probe is the launch: EACCES facts %+v steps %v tier %v: %v", f, steps, got, err)
	}
}

func TestOneConfigurationPerTier(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	for _, tr := range []tier{tierN, tierF} {
		s := spec("echo")
		p := testPlan(tr)
		c := baseConfig(s, p)
		cmd := exec.Command(os.Args[0])
		setProcAttr(cmd)
		tierNamespaces(cmd, p)
		want := uintptr(0)
		if tr == tierN {
			want = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNET
		}
		if cmd.SysProcAttr.Cloneflags != want || c.Overlay != (tr == tierN) || c.Tier != tr.String() || p.mounted() != (tr == tierN) || p.isolated() != (tr == tierN) {
			t.Fatalf("T-4 verified: %s configuration %+v attrs %+v", tr, c, cmd.SysProcAttr)
		}
		if tr == tierF && c.ParentPID != os.Getpid() {
			t.Fatalf("T-4 verified: F parent %d", c.ParentPID)
		}
	}
}

func TestLaunchVerifiesTier(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	s := spec("sentinel:" + filepath.Join(t.TempDir(), "started"))
	s.tier = tierF
	s.rewriteLauncher = func(cmd *exec.Cmd) { cmd.Args = []string{os.Args[0], "--sidecar-test=sb2-fake:wrong-tier"} }
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	out, err := Run(ctx, s)
	if err == nil || !strings.Contains(err.Error(), "decided tier F") || !strings.Contains(err.Error(), "tier=N") || bytes.Contains(out.Stdout, []byte("proceed received")) {
		t.Fatalf("T-4 verified: wrong tier accepted or wrong error: %+v %v", out, err)
	}
}

func TestHandshakeFailsClosed(t *testing.T) {
	for _, fault := range []string{"no-report", "truncated", "invalid", "eof", "no-proceed", "cancel-before-report", "cancel-after-report", "cancel-after-proceed"} {
		t.Run(fault, func(t *testing.T) {
			if !requireLaunch(t) {
				return
			}
			s := spec("target-sleep")
			ready := make(chan struct{}, 1)
			s.Stdout = &readyWriter{ready: ready, seen: targetStarted}
			s.tier = tierF
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var cmd *exec.Cmd
			genuine := fault == "no-proceed" || fault == "cancel-after-report" || fault == "cancel-after-proceed"
			s.rewriteLauncher = func(c *exec.Cmd) {
				cmd = c
				if !genuine {
					c.Args = []string{os.Args[0], "--sidecar-test=sb2-fake:" + fault}
				}
			}
			reported := false
			if genuine {
				s.afterReport = func() {
					reported = true
					if fault == "cancel-after-report" {
						cancel()
					}
					if fault == "cancel-after-proceed" {
						go func() {
							select {
							case <-ready:
							case <-time.After(time.Second):
							}
							cancel()
						}()
					}
				}
			}
			s.skipProceed = fault == "no-proceed"
			// A launcher which never reports is cancelled explicitly; one whose
			// report is cut or invalid ends the handshake by itself, and a timer
			// there would race the launcher's start on a loaded machine. Real
			// launchers are cancelled only at their requested phase barrier.
			var timer *time.Timer
			switch fault {
			case "no-proceed":
				s.afterReport = func() { reported = true; timer = time.AfterFunc(100*time.Millisecond, cancel) }
			case "no-report", "cancel-before-report":
				timer = time.AfterFunc(300*time.Millisecond, cancel)
			}
			began := time.Now()
			out, err := Run(ctx, s)
			if timer != nil {
				timer.Stop()
			}
			if err == nil || time.Since(began) > 2*time.Second {
				t.Fatalf("T-4 handshake: %s returned %+v %v after %v", fault, out, err, time.Since(began))
			}
			if cmd == nil || cmd.Process == nil || cmd.ProcessState == nil || alive(cmd.Process.Pid) {
				t.Fatalf("T-4 handshake: %s launcher not reaped", fault)
			}
			if genuine && !reported {
				t.Fatalf("T-4 handshake: %s never reached the report barrier", fault)
			}
			started := strings.HasPrefix(string(out.Stdout), "target ")
			if fault == "cancel-after-proceed" {
				if !started {
					t.Fatal("T-4 handshake: target never reached run phase")
				}
			} else if started {
				t.Fatalf("T-4 handshake: %s target started", fault)
			}

			if (fault == "eof" || fault == "truncated") && !strings.Contains(err.Error(), "no tier report") {
				t.Fatalf("T-4 handshake: %s ignored report read failure: %v", fault, err)
			}
			if bytes.Contains(out.Stdout, []byte("proceed received")) {
				t.Fatalf("T-4 handshake: %s got proceed", fault)
			}
		})
	}
}

func TestProbeDeadline(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	target := probeTypst(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	began := time.Now()
	err := runProbe(ctx, target, tierF)
	if !errors.Is(err, context.Canceled) || time.Since(began) > 2*time.Second {
		t.Fatalf("T-5 deadline: cancelled probe returned %v after %v", err, time.Since(began))
	}
}

func TestRunReturnsAfterCancel(t *testing.T) {
	for _, tr := range []tier{tierN, tierF} {
		t.Run(tr.String(), func(t *testing.T) {
			for _, containment := range []string{"cgroup", "rlimit"} {
				if tr == tierF && containment == "cgroup" {
					continue
				}
				t.Run(containment, func(t *testing.T) {
					if !requireTier(t, tr) {
						return
					}
					s := spec("sleep")
					s.tier = tr
					s.disableScope = containment == "rlimit"
					s.MemoryLimit = 4 << 30
					if containment == "cgroup" {
						m := scopeManager(t)
						requireScopedRun(t, m, "fds")
						s.scope = m
					}
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					started := make(chan int, 1)
					var launched *exec.Cmd
					s.rewriteLauncher = func(c *exec.Cmd) { launched = c }
					// A verified report is the deterministic cancellation point. Defer it
					// briefly so that proceed reaches the target and exercises cmd.Cancel.
					s.afterReport = func() { started <- 1; time.AfterFunc(50*time.Millisecond, cancel) }
					done := make(chan struct{})
					var out Output
					var err error
					go func() { out, err = Run(ctx, s); close(done) }()
					select {
					case <-started:
					case <-done:
						t.Fatalf("T-5 deadline: launch failed before report: %+v %v", out, err)
					case <-time.After(5 * time.Second):
						cancel()
						t.Fatal("T-5 deadline: no verified report")
					}
					began := time.Now()
					select {
					case <-done:
						if !errors.Is(err, context.Canceled) || out.Containment != containment {
							t.Fatalf("T-5 deadline: %+v %v", out, err)
						}
					case <-time.After(2 * time.Second):
						_ = signalGroup(launched.Process.Pid, syscall.SIGKILL)
						<-done
						t.Fatal("T-5 deadline: Run did not return within 2 s")
					}
					if launched.ProcessState == nil || alive(launched.Process.Pid) {
						t.Fatal("T-5 deadline: launcher not reaped")
					}
					if time.Since(began) > 2*time.Second {
						t.Fatal("T-5 deadline: termination exceeded 2 s")
					}
				})
			}
		})
	}
}

func TestNoCgroupWithoutPIDNamespace(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	m := scopeManager(t)
	steps := 0
	m.fault = func(string) error { steps++; return nil }
	c := exec.Command(os.Args[0])
	setProcAttr(c)
	tierNamespaces(c, testPlan(tierF))
	r := m.prepare(t.Context(), Spec{MemoryLimit: 1 << 30}, c)
	if r != nil {
		r.finish(0)
	}
	if r != nil || steps != 0 || c.SysProcAttr.UseCgroupFD {
		t.Fatalf("T-6 fallback: F prepared cgroup: %v steps=%d attrs=%+v", r, steps, c.SysProcAttr)
	}
}

func TestPlacementRefusedFallsBack(t *testing.T) {
	if !requireN(t) {
		return
	}
	m := scopeManager(t)
	requireScopedRun(t, m, "fds")
	m.beforeStart = func(r *cgroupRun) {
		// A cgroup directory which has disappeared is a recognized placement refusal.
		_ = r.session.holder.Process.Kill()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(r.path); errors.Is(err, os.ErrNotExist) {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("T-6 fallback: run cgroup did not disappear after holder death")
	}
	s := scopeSpec(t, m, "echo")
	s.Env = helperEnv("echo")
	s.tier = tierN
	s.WatchDir = t.TempDir()
	s.WriteFiles = []string{"a", "b"}
	s.MaxDirBytes = 8192
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// Force slot mode so the retry must reuse the exact slots of attempt one.
	out, err := run(ctx, s, testPlan(tierN), func(Spec) (*directoryMount, error) { return nil, nil })
	if err != nil || out.ExitCode != 0 || out.Containment != "rlimit" || string(out.Stdout) != "out\n" {
		t.Fatalf("T-6 fallback: %+v %v", out, err)
	}
}

func TestDescriptorsClosedBeforeExec(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	s := spec("sb2-fds")
	s.tier = tierF
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatalf("T-4 descriptors: %v", err)
	}
	defer func() { _ = f.Close() }()
	s.rewriteLauncher = func(c *exec.Cmd) { c.ExtraFiles = append(c.ExtraFiles, f) }
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || string(out.Stdout) != "descriptors closed\n" {
		t.Fatalf("T-4 descriptors: %+v %v", out, err)
	}
}

func TestProtectedRefusedInF(t *testing.T) {
	f := tierFacts{Arch: "amd64", Landlock: 3, Seccomp: true, NS: launchRefused, F: launchOK, Protected: true}
	if tr, err := decideTier(testProgram, f); tr != tierNone || !errors.Is(err, ErrProtectedRefused) {
		t.Fatalf("S-9 protected: decision %v %v", tr, err)
	}
	if !requireLaunch(t) {
		return
	}
	marker := filepath.Join(t.TempDir(), "started")
	s := spec("sentinel:" + marker)
	s.tier = tierF
	s.Protected = true
	_, err := Run(t.Context(), s)
	if !errors.Is(err, ErrProtectedRefused) {
		t.Fatalf("S-9 protected: Run %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("S-9 protected: target started: %v", err)
	}
}

func TestProtectedRunsInN(t *testing.T) {
	if !requireN(t) {
		return
	}
	s := spec("echo")
	s.tier = tierN
	s.Protected = true
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || !out.Isolated || string(out.Stdout) != "out\n" {
		t.Fatalf("S-9 protected: N: %+v %v", out, err)
	}
}
