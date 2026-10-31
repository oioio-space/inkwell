package sidecar

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func requireOutputMount(t *testing.T) {
	t.Helper()
	if !probeOutputMount() {
		t.Skip("host denies unprivileged user/mount namespace or tmpfs (AppArmor/sysctl/container policy)")
	}
}

func TestOutputMountENOSPC(t *testing.T) {
	requireOutputMount(t)
	dir := t.TempDir()
	s := spec("fillmount:" + dir)
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 1<<20, []string{"a", "b"}
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || !strings.Contains(string(out.Stdout), "ENOSPC") {
		t.Fatalf("Run = %+v, %v", out, err)
	}
	if got := dirBytes(dir); got != s.MaxDirBytes {
		t.Fatalf("directory bytes = %d, want %d", got, s.MaxDirBytes)
	}
}

func TestOutputMountRejectsSparseLogicalOverflow(t *testing.T) {
	requireOutputMount(t)
	dir := t.TempDir()
	s := spec("sparsemount:" + dir)
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 8<<20, []string{"a", "b"}
	out, err := Run(t.Context(), s)
	if err == nil || !out.OutputLimit {
		t.Fatalf("sparse outputs must be rejected: %+v, %v", out, err)
	}
	if got := dirBytes(dir); got > s.MaxDirBytes {
		t.Fatalf("exported %d bytes", got)
	}
}

// An unused output must not reserve a fixed share of the directory budget.
// This deterministically distinguishes remaining-budget enforcement from shares.
func TestDirectoryBoundAllowsUnevenOutputs(t *testing.T) {
	requireOutputMount(t)
	dir := t.TempDir()
	s := spec("writefile:" + filepath.Join(dir, "a") + ",3")
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 4<<20, []string{"a", "b"}
	out, err := Run(t.Context(), s)
	if err != nil || out.ExitCode != 0 || out.OutputLimit {
		t.Fatalf("3 MiB in a 4 MiB budget with an unused output: Run = %+v, %v", out, err)
	}
	if got := dirBytes(dir); got != 3<<20 {
		t.Fatalf("directory holds %d bytes, want 3 MiB", got)
	}
}

func TestOutputMountProbeRetriesFailure(t *testing.T) {
	calls := 0
	probe := cachedMountProbe(func() bool { calls++; return calls > 1 })
	if probe() {
		t.Fatal("first probe succeeded")
	}
	retried, cached := probe(), probe()
	if !retried || !cached || calls != 2 {
		t.Fatalf("probe calls = %d, want transient failure retried and success cached", calls)
	}
}

func TestOutputMountExportFailureRemovesCreatedFiles(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("output"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, "b"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	m := directoryMount{spec: Spec{WatchDir: dest, MaxDirBytes: 100}, root: root}
	if err := m.export(); err == nil {
		t.Fatal("expected destination collision")
	}
	if _, err := os.Stat(filepath.Join(dest, "a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partly exported a remains: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "b"))
	if err != nil || string(data) != "existing" {
		t.Fatalf("existing output changed: %q, %v", data, err)
	}
}

func TestExportFileRejectsSizeChange(t *testing.T) {
	for _, size := range []int64{2, 8} {
		t.Run(strconv.FormatInt(size, 10), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "out")
			if err := exportFile(path, strings.NewReader("data"), size, 100); err == nil {
				t.Fatal("accepted changed size")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("partial file remains: %v", err)
			}
		})
	}
}
