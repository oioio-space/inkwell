package sidecar

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReviewBootstrapReport(t *testing.T) {
	for _, refusal := range []bool{false, true} {
		t.Run(fmt.Sprint(refusal), func(t *testing.T) {
			cmd := exec.Command("unused")
			finish, closeReport, err := signalReport(cmd)
			if err != nil {
				t.Fatal(err)
			}
			defer closeReport()
			out := Output{ExitCode: sandboxExitStatus(unix.WaitStatus(124<<8), -1, CoreDumpNone)}
			if refusal {
				t.Setenv(signalReportEnv, fmt.Sprint(cmd.ExtraFiles[0].Fd()))
				t.Setenv(expectedCgroupEnv, "/definitely-not-the-current-cgroup")
				if err := sandboxExec(sandboxConfig{}, nil); err == nil {
					t.Fatal("membership accepted")
				}
			}
			finish(&out)
			if out.bootstrapRefused != refusal || out.ExitCode != 124 || out.Signal != 0 {
				t.Fatalf("target status/refusal confused: %+v", out)
			}
		})
	}
}

func TestReviewPlacementErrors(t *testing.T) {
	path := t.TempDir()
	r := &cgroupRun{path: path}
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{unix.EACCES, true},
		{unix.EBUSY, true},
		{unix.EOPNOTSUPP, true},
		{unix.ENODEV, true},
		{unix.ENOENT, false},
		{unix.ENOEXEC, false},
		{unix.EAGAIN, false},
	} {
		if got := r.placementError(fmt.Errorf("start: %w", tc.err)); got != tc.want {
			t.Errorf("%v: %v", tc.err, got)
		}
	}
	r.path = filepath.Join(path, "removed")
	if !r.placementError(unix.ENOENT) {
		t.Fatal("removed cgroup not recognized")
	}
}

func TestReviewOptionalHostIdentity(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if d, err := targetHostIdentity(root, 42, false); err != nil || d != nil {
		t.Fatalf("optional identity: %v %v", d, err)
	}
	if _, err := targetHostIdentity(root, 42, true); err == nil {
		t.Fatal("required evidence accepted missing children")
	}
}

func TestReviewKillFallback(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "cgroup.procs"), []byte("123\n456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var pids []int
	kill := func(pid int) error { pids = append(pids, pid); return nil }
	if err := checkCgroupKill(path); err != nil {
		t.Fatal(err)
	}
	if err := killRunCgroup(path, kill); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(pids) != "[123 456]" {
		t.Fatal(pids)
	}
	if _, err := os.Stat(filepath.Join(path, "cgroup.kill")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created fake cgroup.kill")
	}
	if err := os.Remove(filepath.Join(path, "cgroup.procs")); err != nil {
		t.Fatal(err)
	}
	if err := checkCgroupKill(path); err == nil {
		t.Fatal("missing both cleanup interfaces accepted")
	}
}
