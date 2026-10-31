package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDirectoryBoundRejectsUnenforceableSpec(t *testing.T) {
	for _, names := range [][]string{nil, {"../escape"}, {".."}, {"a", "a"}, {"."}, {"sub/file"}} {
		s := spec("echo")
		s.WatchDir, s.MaxDirBytes, s.WriteFiles = t.TempDir(), 1024, names
		if _, err := runWithoutMount(t.Context(), s); err == nil {
			t.Fatalf("accepted invalid output list %q", names)
		}
	}
}

func TestDirectoryBoundExactBytes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("hard directory output unsupported on this OS")
	}
	for _, limit := range []uint64{0, 1001} {
		dir := t.TempDir()
		s := spec("writefile:" + filepath.Join(dir, "a") + ",1")
		s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 3001, []string{"a", "b"}
		s.FileSizeLimit = limit
		out, err := runWithoutMount(t.Context(), s)
		if err != nil || !out.OutputLimit || out.ExitCode == 0 {
			t.Fatalf("Run = %v, %+v; want failed capped write", err, out)
		}
		info, err := os.Stat(filepath.Join(dir, "a"))
		want := int64(1500)
		if limit != 0 {
			want = 1001
		}
		if err != nil || info.Size() != want {
			t.Fatalf("file = %v, %v; want exactly %d bytes", info, err, want)
		}
	}
}

func runWithoutMount(ctx context.Context, s Spec) (Output, error) {
	return run(ctx, s, func(Spec) (*directoryMount, error) { return nil, nil })
}

func TestDirectoryFallbackWhenNamespacesUnavailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Landlock fallback is Linux-only")
	}
	dir := t.TempDir()
	s := spec("writefile:" + filepath.Join(dir, "a") + ",3")
	s.WatchDir, s.MaxDirBytes, s.WriteFiles = dir, 4<<20, []string{"a", "b"}
	out, err := runWithoutMount(t.Context(), s)
	if err != nil || out.ExitCode == 0 || !out.OutputLimit {
		t.Fatalf("fallback = %+v, %v", out, err)
	}
	if got := dirBytes(dir); got != 2<<20 {
		t.Fatalf("fixed share = %d, want 2 MiB", got)
	}
}
