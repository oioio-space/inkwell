package sidecar

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

const (
	testTarget = "test_arch"
	testName   = "tool"
)

// testScript is a tiny program every Unix machine can run.
var testScript = []byte("#!/bin/sh\necho tool-ok\n")

func gz(t testing.TB, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// countFS counts how many embedded files are opened.
type countFS struct {
	fs.FS
	opens atomic.Int32
}

func (c *countFS) Open(name string) (fs.File, error) {
	c.opens.Add(1)
	return c.FS.Open(name)
}

type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// newTestStore builds a Store whose single sidecar "tool" holds content.
func newTestStore(t *testing.T, content []byte) (*Store, *countFS, *logs) {
	t.Helper()
	files := &countFS{FS: fstest.MapFS{testName + ".gz": {Data: gz(t, content)}}}
	out := &logs{}
	return &Store{
		Dir: t.TempDir(),
		FS:  files,
		Manifest: &Manifest{Schema: 1, Sidecars: map[string]Sidecar{
			testName: {Version: "1.2.3", License: "MIT", Targets: map[string]Artifact{
				testTarget: {
					URL: "https://github.com/x/y", ArchiveSHA256: sum([]byte("a")), ArchiveSize: 1,
					Member: testName, SHA256: sum(content), Size: int64(len(content)),
				},
			}},
		}},
		Target: testTarget,
		Logger: slog.New(slog.NewTextHandler(out, nil)),
	}, files, out
}

func mustResolve(t *testing.T, s *Store) string {
	t.Helper()
	path, err := s.Resolve(testName)
	if err != nil {
		t.Fatalf("Resolve(%q) = %v", testName, err)
	}
	return path
}

func fileSum(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum(data)
}

func TestResolve(t *testing.T) {
	t.Run("SIDE-02 first launch extracts a runnable, private, intact binary", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)

		if !strings.HasPrefix(path, s.Dir) {
			t.Errorf("path %q is outside the cache dir %q", path, s.Dir)
		}
		if got, want := fileSum(t, path), sum(testScript); got != want {
			t.Errorf("sha256 of %s = %s, want %s", path, got, want)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o500 {
			t.Errorf("binary mode = %v, want 0500", got)
		}
		dir, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if got := dir.Mode().Perm(); got != 0o700 {
			t.Errorf("directory mode = %v, want 0700", got)
		}
		out, err := exec.CommandContext(t.Context(), path).Output() // #nosec G204 -- the file this test just extracted
		if err != nil {
			t.Fatalf("running %s: %v", path, err)
		}
		if got := strings.TrimSpace(string(out)); got != "tool-ok" {
			t.Errorf("output = %q, want tool-ok", got)
		}
	})

	t.Run("SIDE-03 second launch does not rewrite the file", func(t *testing.T) {
		s, files, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)
		before, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		opened := files.opens.Load()

		if again := mustResolve(t, s); again != path {
			t.Errorf("second Resolve = %q, want %q", again, path)
		}
		after, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) {
			t.Error("the binary was replaced although it was intact")
		}
		if got := files.opens.Load(); got != opened {
			t.Errorf("embedded files opened %d times on the second Resolve, want 0", got-opened)
		}
	})

	t.Run("SIDE-04 modified byte is detected, logged and re-extracted", func(t *testing.T) {
		s, _, out := newTestStore(t, testScript)
		path := mustResolve(t, s)
		tampered := bytes.Clone(testScript)
		tampered[len(tampered)-2] ^= 0xff
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, tampered, 0o600); err != nil {
			t.Fatal(err)
		}

		mustResolve(t, s)
		if got, want := fileSum(t, path), sum(testScript); got != want {
			t.Errorf("sha256 after re-extraction = %s, want %s", got, want)
		}
		if log := out.String(); !strings.Contains(log, "re-extract") || !strings.Contains(log, testName) {
			t.Errorf("log = %q, want a warning naming %s and the re-extraction", log, testName)
		}
	})

	t.Run("SIDE-05 missing file is extracted again", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		mustResolve(t, s)
		if got, want := fileSum(t, path), sum(testScript); got != want {
			t.Errorf("sha256 = %s, want %s", got, want)
		}
	})

	t.Run("SIDE-06 truncated file is extracted again", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(path, int64(len(testScript)/2)); err != nil {
			t.Fatal(err)
		}
		mustResolve(t, s)
		if got, want := fileSum(t, path), sum(testScript); got != want {
			t.Errorf("sha256 = %s, want %s", got, want)
		}
	})

	t.Run("SIDE-07 a symlink in place of the binary is replaced, not followed", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)
		decoy := filepath.Join(t.TempDir(), "decoy")
		// The decoy has the right content: a verifier that follows links would accept it.
		if err := os.WriteFile(decoy, testScript, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(decoy, path); err != nil {
			t.Fatal(err)
		}

		mustResolve(t, s)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Errorf("mode after Resolve = %v, want a regular file", info.Mode())
		}
		if got := fileSum(t, decoy); got != sum(testScript) {
			t.Error("the link target was modified")
		}
	})

	t.Run("a directory in place of the binary is replaced", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(path, "x"), 0o700); err != nil {
			t.Fatal(err)
		}
		mustResolve(t, s)
		if got, want := fileSum(t, path), sum(testScript); got != want {
			t.Errorf("sha256 = %s, want %s", got, want)
		}
	})

	t.Run("SIDE-08 corrupted embedded file is refused and leaves nothing behind", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		s.FS = fstest.MapFS{testName + ".gz": {Data: gz(t, []byte("#!/bin/sh\necho evil\n"))}}
		_, err := s.Resolve(testName)
		if !errors.Is(err, ErrIntegrity) {
			t.Fatalf("Resolve = %v, want ErrIntegrity", err)
		}
		assertNoFiles(t, s.Dir)
	})

	t.Run("SIDE-08 an embedded file that inflates past its pinned size is refused", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		s.FS = fstest.MapFS{testName + ".gz": {Data: gz(t, bytes.Repeat([]byte("a"), 1<<20))}}
		_, err := s.Resolve(testName)
		if !errors.Is(err, ErrIntegrity) {
			t.Fatalf("Resolve = %v, want ErrIntegrity", err)
		}
		assertNoFiles(t, s.Dir)
	})

	t.Run("SIDE-08 an embedded file that is not gzip is refused", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		s.FS = fstest.MapFS{testName + ".gz": {Data: []byte("not gzip")}}
		if _, err := s.Resolve(testName); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("Resolve = %v, want ErrIntegrity", err)
		}
		assertNoFiles(t, s.Dir)
	})

	t.Run("SIDE-09 nothing embedded says how to fix it", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		s.FS = fstest.MapFS{".gitkeep": {}}
		_, err := s.Resolve(testName)
		if !errors.Is(err, ErrNotEmbedded) {
			t.Fatalf("Resolve = %v, want ErrNotEmbedded", err)
		}
		if !strings.Contains(err.Error(), "mise run sidecar:fetch") {
			t.Errorf("error %q does not name the fix", err)
		}
	})

	t.Run("SIDE-10 concurrent Resolve on an empty cache", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		const n = 8
		paths := make([]string, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() { paths[i], errs[i] = s.Resolve(testName) })
		}
		wg.Wait()
		for i := range n {
			if errs[i] != nil {
				t.Fatalf("Resolve #%d = %v", i, errs[i])
			}
			if paths[i] != paths[0] {
				t.Errorf("Resolve #%d = %q, want %q", i, paths[i], paths[0])
			}
		}
		if got, want := fileSum(t, paths[0]), sum(testScript); got != want {
			t.Errorf("sha256 = %s, want %s", got, want)
		}
		assertOnlyFile(t, filepath.Dir(paths[0]), testName)
	})

	t.Run("SIDE-11 unsupported platform names the target", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		s.Target = "plan9_mips"
		_, err := s.Resolve(testName)
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "plan9_mips") {
			t.Fatalf("Resolve = %v, want ErrUnsupported naming plan9_mips", err)
		}
	})

	t.Run("an unknown sidecar is unsupported", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		if _, err := s.Resolve("nope"); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("Resolve(nope) = %v, want ErrUnsupported", err)
		}
	})

	t.Run("a cache directory that is not private is tightened", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		path := mustResolve(t, s)
		if err := os.Chmod(filepath.Dir(path), 0o755); err != nil { // #nosec G302 -- the test loosens it on purpose
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		mustResolve(t, s)
		dir, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if got := dir.Mode().Perm(); got != 0o700 {
			t.Errorf("directory mode = %v, want 0700", got)
		}
	})

	t.Run("the cache directory cannot be created", func(t *testing.T) {
		s, _, _ := newTestStore(t, testScript)
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		s.Dir = filepath.Join(blocker, "cache")
		if _, err := s.Resolve(testName); err == nil {
			t.Fatal("Resolve under a regular file = nil, want an error")
		}
	})

	t.Run("a default store uses the user cache directory", func(t *testing.T) {
		cache := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", cache)
		t.Setenv("HOME", cache)
		dir, err := DefaultDir()
		if err != nil {
			t.Fatalf("DefaultDir() = %v", err)
		}
		if filepath.Base(dir) != "inkwell" || !strings.HasPrefix(dir, cache) {
			t.Errorf("DefaultDir() = %q, want .../inkwell under %s", dir, cache)
		}
	})
}

func assertNoFiles(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			t.Errorf("leftover file %s", path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("%s holds %v, want only %s", dir, names, name)
	}
}
