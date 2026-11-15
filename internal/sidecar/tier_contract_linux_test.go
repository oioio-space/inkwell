package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestIsolationReportsTier(t *testing.T) {
	savedFacts, savedArch := sandboxFacts, hostArch
	t.Cleanup(func() { sandboxFacts = savedFacts; hostArch = savedArch })
	hostArch = "amd64"
	for _, tr := range []tier{tierN, tierF, tierNone} {
		f := tierFacts{Arch: "amd64", Landlock: 3, Seccomp: true, NS: launchOK}
		if tr != tierN {
			f.NS = launchRefused
			f.F = launchOK
		}
		if tr == tierNone {
			f.Seccomp = false
		}
		sandboxFacts = func(*Store) (tierFacts, error) { return f, nil }
		isolated, err := networkIsolation(nil)
		if isolated != (tr == tierN) || (tr == tierNone) != errors.Is(err, ErrSandboxRefused) {
			t.Fatalf("U-1 isolation: %s: isolated=%t err=%v", tr, isolated, err)
		}
	}
}

func TestHostileCgroupMemoryIsMemoryLimit(t *testing.T) {
	tr := machineTier(t)
	if tr == tierNone {
		return
	}
	s := deathSpec(t, "scope-resident")
	s.Dir = t.TempDir()
	s.tier = tr
	s.MemoryLimit = 2<<30 + raceAddressReserve(t)
	if tr == tierN {
		s.scope = scopeManager(t)
		s.ResidentLimit = 128 << 20
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	out, err := Run(ctx, s)
	if err != nil {
		t.Fatalf("S-11 no oom-kill: %v", err)
	}
	failedAllocation := out.ExitCode == 1 && strings.Contains(string(out.Stdout), "allocation-failed")
	switch {
	case tr == tierF:
		if out.Containment != "rlimit" || !failedAllocation {
			t.Fatalf("S-11 no oom-kill: F allocation outcome %+v", out)
		}
	case out.Containment == "cgroup":
		if out.MemoryCause != MemoryLocalLimit || out.Signal != 9 {
			t.Fatalf("S-11 no oom-kill: N bound outcome %+v", out)
		}
	case !failedAllocation:
		t.Fatalf("S-11 no oom-kill: N rlimit outcome %+v", out)
	}
}

func TestHostileMemoryDeathStartsNoCoreHelper(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	s := deathSpec(t, "scope-resident")
	s.Dir = t.TempDir()
	s.tier = machineTier(t)
	// 2 GiB, not 1: the helper's own start must not be what fails (escapeAllocation).
	s.MemoryLimit = 2<<30 + raceAddressReserve(t)
	s.ResidentLimit = 128 << 20
	s.scope = scopeManager(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	out, err := Run(ctx, s)
	if err != nil || out.ExitCode == 0 {
		t.Fatalf("S-11 no oom-kill: memory death/core outcome %+v %v", out, err)
	}
	assertNoCoreHelper(t, out.CoreDump)
}

func TestThirdProgramDescriptor(t *testing.T) {
	for _, tr := range []tier{tierN, tierF} {
		t.Run(tr.String(), func(t *testing.T) {
			if !requireTier(t, tr) {
				return
			}
			dir := t.TempDir()
			binary := filepath.Join(dir, "third")
			data, err := os.ReadFile(os.Args[0])
			if err != nil {
				t.Fatalf("S-10 descriptor: binary: %v", err)
			}
			if err := os.WriteFile(binary, data, 0o700); err != nil {
				t.Fatalf("S-10 descriptor: copy: %v", err)
			}
			// A Go helper requires the runtime calls, but gets its own prctl list:
			// PR_GET_NAME is allowed here and denied by both product descriptors.
			third := program{name: "third", proven: []string{"amd64"}, syscalls: map[string][]uint32{runtime.GOARCH: slices.Clone(sidecarSyscalls[runtime.GOARCH])}, prctl: []uint32{unix.PR_GET_NAME}}
			withProgram(t, third)
			s := Spec{Path: binary, Dir: t.TempDir(), Env: helperEnv("sb2-third"), tier: tr}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			out, err := Run(ctx, s)
			if err != nil || out.ExitCode != 0 || string(out.Stdout) != "own prctl allowed; other prctl refused\n" {
				t.Fatalf("S-10 descriptor: %s: %+v %v", tr, out, err)
			}
		})
	}
}

func thirdProgramHelper() int {
	var name [16]byte
	if err := unix.Prctl(unix.PR_SET_NAME, uintptr(unsafe.Pointer(&name[0])), 0, 0, 0); !errors.Is(err, unix.EPERM) {
		return 2
	}
	if err := unix.Prctl(unix.PR_GET_NAME, uintptr(unsafe.Pointer(&name[0])), 0, 0, 0); err != nil {
		return 3
	}
	fmt.Println("own prctl allowed; other prctl refused")
	return 0
}
