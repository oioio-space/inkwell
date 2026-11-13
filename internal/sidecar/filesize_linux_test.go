package sidecar

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// fileCap is the file-size cap of the capwrite runs, below their 1 MiB
// directory bound, so RLIMIT_FSIZE is what stops the writer.
const fileCap = 64 << 10

// launchPaths are the two bounded launch paths: a private tmpfs, and Landlock
// with per-file RLIMIT_FSIZE shares when namespaces cannot mount one.
var launchPaths = []struct {
	name string
	run  func(context.Context, Spec) (Output, error)
}{{name: "tmpfs", run: Run}, {name: "landlock", run: runWithoutMount}}

// capRun runs the capwrite helper, ending as end says, on one declared output.
func capRun(ctx context.Context, t *testing.T, end string, launch func(context.Context, Spec) (Output, error)) (out Output, path string, err error) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "out")
	s := deathSpec(t, "capwrite:"+path+","+end)
	s.WatchDir, s.MaxDirBytes, s.WriteFiles, s.FileSizeLimit = dir, 1<<20, []string{"out"}, fileCap
	out, err = launch(ctx, s)
	return out, path, err
}

// requireNoCoreHelper fails unless the kernel reported, and reported no core dump.
func requireNoCoreHelper(t *testing.T, out Output) {
	t.Helper()
	switch out.CoreDump {
	case CoreDumpUnknown:
		t.Skip("the kernel does not report PIDFD_INFO_COREDUMP (Linux 6.16): no core helper cannot be shown")
	case CoreDumpStarted:
		t.Errorf("the kernel started a core helper for a signal %d death", out.Signal)
	}
}

// X1 (ADR 0042 A6): the write that reaches the cap is shortened, the next one
// fails with EFBIG, no signal ends the target, and the run is an output limit.
func TestFileSizeCapShortWriteThenEFBIG(t *testing.T) {
	for _, p := range launchPaths {
		t.Run(p.name, func(t *testing.T) {
			out, path, err := capRun(t.Context(), t, "short", p.run)
			want := fmt.Sprintf("short=%d next=%v", fileCap-capChunk, unix.EFBIG)
			if err != nil || out.ExitCode != 1 || out.Signal != 0 || strings.TrimSpace(string(out.Stdout)) != want {
				t.Fatalf("Run = %v, exit %d signal %d stdout %q stderr %q; want exit 1, no signal, %q", err, out.ExitCode, out.Signal, out.Stdout, out.Stderr, want)
			}
			if !out.OutputLimit {
				t.Error("OutputLimit = false for a run stopped at its cap")
			}
			if got := fileSize(path); got != fileCap {
				t.Errorf("output holds %d bytes, want exactly the cap %d", got, fileCap)
			}
		})
	}
}

// X2: a target that aborts on EFBIG is an output limit, its file at the cap.
func TestFileSizeCapAbortOnEFBIGIsOutputLimit(t *testing.T) {
	for _, p := range launchPaths {
		t.Run(p.name, func(t *testing.T) {
			out, path, err := capRun(t.Context(), t, "abort", p.run)
			if err != nil || out.Signal != int(unix.SIGABRT) {
				t.Fatalf("Run = %v, exit %d signal %d stderr %q; want death by SIGABRT", err, out.ExitCode, out.Signal, out.Stderr)
			}
			if !out.OutputLimit {
				t.Error("OutputLimit = false for an abort at the cap")
			}
			if got := fileSize(path); got != fileCap {
				t.Errorf("output holds %d bytes, want exactly the cap %d", got, fileCap)
			}
			requireNoCoreHelper(t, out)
		})
	}
}

// X3: a target that restores SIGXFSZ's default action dies of it at the cap;
// that is still an output limit, with the cap kept and no core helper.
func TestFileSizeCapDispositionResetIsOutputLimit(t *testing.T) {
	requireGuaranteedCorePattern(t)
	for _, p := range launchPaths {
		t.Run(p.name, func(t *testing.T) {
			out, path, err := capRun(t.Context(), t, "reset", p.run)
			if err != nil || out.Signal != int(unix.SIGXFSZ) {
				t.Fatalf("Run = %v, exit %d signal %d stderr %q; want death by SIGXFSZ", err, out.ExitCode, out.Signal, out.Stderr)
			}
			if !out.OutputLimit {
				t.Error("OutputLimit = false for SIGXFSZ at the cap")
			}
			if got := fileSize(path); got != fileCap {
				t.Errorf("output holds %d bytes, want exactly the cap %d", got, fileCap)
			}
			requireNoCoreHelper(t, out)
		})
	}
}

// X4: a target that loops on EFBIG is ended by the timeout, as an output limit, the cap kept.
func TestFileSizeCapEFBIGLoopEndsAtTimeout(t *testing.T) {
	for _, p := range launchPaths {
		t.Run(p.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			out, path, err := capRun(ctx, t, "loop", p.run)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Run = %v, exit %d signal %d; want the timeout", err, out.ExitCode, out.Signal)
			}
			if !out.OutputLimit {
				t.Error("OutputLimit = false for a writer looping at its cap")
			}
			if got := fileSize(path); got != fileCap {
				t.Errorf("output holds %d bytes, want exactly the cap %d", got, fileCap)
			}
		})
	}
}
