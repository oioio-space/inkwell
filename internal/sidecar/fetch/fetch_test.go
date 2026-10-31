package fetch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const (
	target = "linux_amd64"
	member = "tool-1.0/tool"
)

var binary = []byte("#!/bin/sh\necho tool-ok\n")

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// tarGz builds a .tar.gz holding the given regular files and, for names in
// links, symbolic links.
func tarGz(t *testing.T, files map[string][]byte, links map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for name, dest := range links {
		if err := tw.WriteHeader(&tar.Header{Name: name, Linkname: dest, Typeflag: tar.TypeSymlink}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type fixture struct {
	srv      *httptest.Server
	hits     *atomic.Int32
	cfg      Config
	archive  []byte
	manifest *sidecar.Manifest
}

// newFixture serves archive and pins it in a manifest; art lets a test break
// one pin.
func newFixture(t *testing.T, archive []byte, art func(*sidecar.Artifact)) *fixture {
	t.Helper()
	hits := &atomic.Int32{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	a := sidecar.Artifact{
		URL: srv.URL + "/tool.tar.gz", ArchiveSHA256: sum(archive), ArchiveSize: int64(len(archive)),
		Member: member, SHA256: sum(binary), Size: int64(len(binary)),
	}
	if art != nil {
		art(&a)
	}
	m := &sidecar.Manifest{Schema: 1, Sidecars: map[string]sidecar.Sidecar{
		"tool": {Version: "1.0", License: "MIT", Targets: map[string]sidecar.Artifact{target: a}},
	}}
	u, _ := url.Parse(srv.URL)
	return &fixture{
		srv: srv, hits: hits, archive: archive, manifest: m,
		cfg: Config{
			Manifest: m, Targets: []string{target}, OutDir: t.TempDir(), CacheDir: t.TempDir(),
			Client: srv.Client(), AllowHost: func(h string) bool { return h == u.Host },
		},
	}
}

func goodArchive(t *testing.T) []byte {
	return tarGz(t, map[string][]byte{member: binary, "tool-1.0/README": []byte("hi")}, nil)
}

func outFile(f *fixture) string { return filepath.Join(f.cfg.OutDir, target, "tool.gz") }

func requireTar(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tar"); err != nil {
		t.Fatalf("fetch needs tar: %v", err)
	}
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			t.Errorf("leftover file %s", path)
		}
		return err
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestFetch(t *testing.T) {
	requireTar(t)

	t.Run("a pinned archive becomes a compressed, verified binary", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatalf("Fetch = %v", err)
		}
		got := readGz(t, outFile(f))
		if !bytes.Equal(got, binary) {
			t.Errorf("fetched binary = %q, want %q", got, binary)
		}
		if f.hits.Load() != 1 {
			t.Errorf("%d downloads, want 1", f.hits.Load())
		}
	})

	t.Run("the compressed output is reproducible", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		first, _ := os.ReadFile(outFile(f))
		if err := os.Remove(outFile(f)); err != nil {
			t.Fatal(err)
		}
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		second, _ := os.ReadFile(outFile(f))
		if !bytes.Equal(first, second) {
			t.Error("two fetches produced different gzip bytes")
		}
	})

	t.Run("SIDE-12 an archive with the wrong SHA-256 is refused", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), func(a *sidecar.Artifact) { a.ArchiveSHA256 = sum([]byte("other")) })
		err := Fetch(t.Context(), f.cfg)
		if !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.OutDir)
		assertEmpty(t, f.cfg.CacheDir)
	})

	t.Run("SIDE-13 a binary with the wrong SHA-256 is refused although the archive is right", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), func(a *sidecar.Artifact) { a.SHA256 = sum([]byte("other")) })
		err := Fetch(t.Context(), f.cfg)
		if !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("SIDE-14 a second run needs no network", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		f.srv.Close() // offline from here on

		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatalf("second Fetch (output present) = %v", err)
		}
		if err := os.Remove(outFile(f)); err != nil {
			t.Fatal(err)
		}
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatalf("third Fetch (download cache only) = %v", err)
		}
		if got := readGz(t, outFile(f)); !bytes.Equal(got, binary) {
			t.Errorf("rebuilt binary = %q", got)
		}
		if f.hits.Load() != 1 {
			t.Errorf("%d downloads over three runs, want 1", f.hits.Load())
		}
	})

	t.Run("a damaged output is rebuilt", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(outFile(f), []byte("junk"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		if got := readGz(t, outFile(f)); !bytes.Equal(got, binary) {
			t.Errorf("rebuilt binary = %q", got)
		}
	})

	t.Run("a damaged download cache entry is fetched again", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(f.cfg.CacheDir)
		if len(entries) != 1 {
			t.Fatalf("download cache holds %d entries, want 1", len(entries))
		}
		if err := os.WriteFile(filepath.Join(f.cfg.CacheDir, entries[0].Name()), []byte("junk"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(outFile(f)); err != nil {
			t.Fatal(err)
		}
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		if f.hits.Load() != 2 {
			t.Errorf("%d downloads, want 2 (the cache entry was damaged)", f.hits.Load())
		}
	})
}

func TestFetchRefuses(t *testing.T) {
	requireTar(t)

	t.Run("SIDE-15 member absent from the archive", func(t *testing.T) {
		archive := tarGz(t, map[string][]byte{"tool-1.0/other": binary}, nil)
		f := newFixture(t, archive, nil)
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil, want an error")
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("SIDE-15 member that is a symbolic link", func(t *testing.T) {
		archive := tarGz(t, nil, map[string]string{member: "/etc/passwd"})
		f := newFixture(t, archive, nil)
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil, want an error")
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("SIDE-15 server sends more than the pinned size", func(t *testing.T) {
		archive := goodArchive(t)
		f := newFixture(t, archive, func(a *sidecar.Artifact) { a.ArchiveSize = int64(len(archive)) - 5 })
		err := Fetch(t.Context(), f.cfg)
		if !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.CacheDir)
	})

	t.Run("SIDE-15 redirect to another host", func(t *testing.T) {
		evil := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(goodArchive(t))
		}))
		t.Cleanup(evil.Close)
		f := newFixture(t, goodArchive(t), nil)
		f.srv.Config.Handler = http.RedirectHandler(evil.URL, http.StatusFound)
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch followed a redirect to a host that is not allowed")
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("a server error", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		f.srv.Config.Handler = http.NotFoundHandler()
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil on a 404")
		}
	})

	t.Run("a URL that is not https", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		sc := f.manifest.Sidecars["tool"]
		a := sc.Targets[target]
		a.URL = "http://github.com/x"
		sc.Targets[target] = a
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil for an http URL")
		}
	})

	t.Run("a target the manifest does not pin", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		f.cfg.Targets = []string{"plan9_mips"}
		if err := Fetch(t.Context(), f.cfg); !errors.Is(err, sidecar.ErrUnsupported) {
			t.Fatalf("Fetch = %v, want ErrUnsupported", err)
		}
	})

	t.Run("a cancelled context", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := Fetch(ctx, f.cfg); err == nil {
			t.Fatal("Fetch = nil on a cancelled context")
		}
	})
}

func TestDefaultAllowHost(t *testing.T) {
	for host, want := range map[string]bool{
		"github.com":                           true,
		"objects.githubusercontent.com":        true,
		"release-assets.githubusercontent.com": true,
		"evil.example":                         false,
		"githubusercontent.com.evil.example":   false,
		"notgithub.com":                        false,
		"github.com.evil.example":              false,
	} {
		if got := DefaultAllowHost(host); got != want {
			t.Errorf("DefaultAllowHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func readGz(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(zr); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestDefaultAllowHostWithPort(t *testing.T) {
	for host, want := range map[string]bool{
		"github.com:443":                    true,
		"objects.githubusercontent.com:443": true,
		"evil.example:443":                  false,
		"github.com.evil.example:443":       false,
	} {
		if got := DefaultAllowHost(host); got != want {
			t.Errorf("DefaultAllowHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestFetchDefaultsRefuseForeignHost(t *testing.T) {
	f := newFixture(t, goodArchive(t), nil)
	f.cfg.AllowHost = nil // the default allows GitHub only, not the test server
	err := Fetch(t.Context(), f.cfg)
	if err == nil || !strings.Contains(err.Error(), "refusing to download") {
		t.Fatalf("Fetch = %v, want a refusal to download", err)
	}
	if got := f.hits.Load(); got != 0 {
		t.Errorf("server was contacted %d times, want 0", got)
	}
}

func TestFetchNeedsManifest(t *testing.T) {
	if err := Fetch(t.Context(), Config{}); err == nil {
		t.Fatal("Fetch without a manifest = nil, want an error")
	}
}

func TestFetchLogsEachStep(t *testing.T) {
	requireTar(t)
	f := newFixture(t, goodArchive(t), nil)
	var log bytes.Buffer
	f.cfg.Log = &log
	for range 2 {
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
	}
	got := log.String()
	for _, want := range []string{"tool linux_amd64: downloaded and verified", "tool linux_amd64: up to date"} {
		if !strings.Contains(got, want) {
			t.Errorf("log lacks %q:\n%s", want, got)
		}
	}
	if err := os.Remove(outFile(f)); err != nil {
		t.Fatal(err)
	}
	log.Reset()
	if err := Fetch(t.Context(), f.cfg); err != nil {
		t.Fatal(err)
	}
	if want := "rebuilt from the download cache"; !strings.Contains(log.String(), want) {
		t.Errorf("log lacks %q:\n%s", want, &log)
	}
	if got := f.hits.Load(); got != 1 {
		t.Errorf("server hit %d times, want 1 (cache reused)", got)
	}
}

func TestFetchRejectsHostileDownloads(t *testing.T) {
	requireTar(t)

	t.Run("archive with the right size but another checksum", func(t *testing.T) {
		archive := goodArchive(t)
		f := newFixture(t, archive, func(a *sidecar.Artifact) { a.ArchiveSHA256 = sum([]byte("other")) })
		if err := Fetch(t.Context(), f.cfg); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.CacheDir)
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("archive shorter than pinned", func(t *testing.T) {
		archive := goodArchive(t)
		f := newFixture(t, archive, func(a *sidecar.Artifact) { a.ArchiveSize = int64(len(archive)) + 5 })
		if err := Fetch(t.Context(), f.cfg); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.CacheDir)
	})

	t.Run("redirect loop", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		f.srv.Config.Handler = http.RedirectHandler(f.srv.URL+"/again", http.StatusFound)
		err := Fetch(t.Context(), f.cfg)
		if err == nil || !strings.Contains(err.Error(), "too many redirects") {
			t.Fatalf("Fetch = %v, want too many redirects", err)
		}
	})

	t.Run("redirect to plain http", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		f.srv.Config.Handler = http.RedirectHandler("http://"+strings.TrimPrefix(f.srv.URL, "https://"), http.StatusFound)
		err := Fetch(t.Context(), f.cfg)
		if err == nil || !strings.Contains(err.Error(), "refusing redirect") {
			t.Fatalf("Fetch = %v, want a refused redirect", err)
		}
	})

	t.Run("unparsable URL", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), func(a *sidecar.Artifact) { a.URL = "https://bad host/\x7f" })
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil, want an error")
		}
	})
}

func TestFetchRejectsBadBinaries(t *testing.T) {
	requireTar(t)

	t.Run("member larger than pinned", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), func(a *sidecar.Artifact) { a.Size = int64(len(binary)) - 3 })
		if err := Fetch(t.Context(), f.cfg); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("member smaller than pinned", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), func(a *sidecar.Artifact) { a.Size = int64(len(binary)) + 3 })
		if err := Fetch(t.Context(), f.cfg); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("member with another checksum", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), func(a *sidecar.Artifact) { a.SHA256 = sum([]byte("other")) })
		if err := Fetch(t.Context(), f.cfg); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Fetch = %v, want ErrMismatch", err)
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("corrupt archive reports tar's complaint", func(t *testing.T) {
		garbage := bytes.Repeat([]byte("not a tar archive "), 100)
		f := newFixture(t, garbage, nil)
		err := Fetch(t.Context(), f.cfg)
		if err == nil || !strings.Contains(err.Error(), "extract "+member) {
			t.Fatalf("Fetch = %v, want an extract error", err)
		}
		assertEmpty(t, f.cfg.OutDir)
	})

	t.Run("damaged output is rebuilt", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(outFile(f), []byte("garbage"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Fetch(t.Context(), f.cfg); err != nil {
			t.Fatal(err)
		}
		if got := readGz(t, outFile(f)); !bytes.Equal(got, binary) {
			t.Errorf("rebuilt binary = %q, want %q", got, binary)
		}
	})
}

func TestFetchUnwritableDirectories(t *testing.T) {
	requireTar(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("cache directory under a file", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		f.cfg.CacheDir = filepath.Join(blocker, "cache")
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil, want an error")
		}
		if got := f.hits.Load(); got != 0 {
			t.Errorf("server hit %d times, want 0", got)
		}
	})

	t.Run("output directory under a file", func(t *testing.T) {
		f := newFixture(t, goodArchive(t), nil)
		f.cfg.OutDir = filepath.Join(blocker, "bin")
		if err := Fetch(t.Context(), f.cfg); err == nil {
			t.Fatal("Fetch = nil, want an error")
		}
	})
}

func TestBoundedBufferKeepsTheStart(t *testing.T) {
	b := &boundedBuffer{max: 5}
	for _, s := range []string{"abc", "defgh", "ij"} {
		if n, err := b.Write([]byte(s)); n != len(s) || err != nil {
			t.Fatalf("Write(%q) = %d, %v, want %d, nil", s, n, err, len(s))
		}
	}
	if got, want := b.String(), "abcde"; got != want {
		t.Errorf("kept %q, want %q", got, want)
	}
}
