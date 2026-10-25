package typst

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func TestBundledPackages(t *testing.T) {
	pkgs, err := loadBundled()
	if err != nil {
		t.Fatalf("loadBundled() = %v", err)
	}

	t.Run("TYP-12 every archive matches its pinned SHA-256 and names itself", func(t *testing.T) {
		var names []string
		for _, p := range pkgs {
			names = append(names, p.Name+":"+p.Version)
			if p.License == "" {
				t.Errorf("%s has no license in packages.json", p.Name)
			}
			data, err := fs.ReadFile(embeddedPackages, p.archive())
			if err != nil {
				t.Errorf("%s: %v", p.Name, err)
				continue
			}
			if got := sumHex(data); got != p.SHA256 {
				t.Errorf("%s archive sha256 = %s, pinned %s", p.Name, got, p.SHA256)
			}
			toml := readMember(t, data, "typst.toml")
			if !regexp.MustCompile(`(?m)^name = "`+p.Name+`"$`).Match(toml) || !regexp.MustCompile(`(?m)^version = "`+regexp.QuoteMeta(p.Version)+`"$`).Match(toml) {
				t.Errorf("%s: typst.toml does not say %s %s", p.Name, p.Name, p.Version)
			}
		}
		want := []string{"cmarker:0.1.10", "elembic:1.1.1", "lilaq:0.6.0", "tiptoe:0.4.0", "touying:0.7.4", "uniwarn:0.1.1", "zero:0.6.1"}
		slices.Sort(names)
		if !slices.Equal(names, want) {
			t.Errorf("bundled = %v, want %v", names, want)
		}
	})

	t.Run("the archives hold what a package needs and nothing else", func(t *testing.T) {
		for _, p := range pkgs {
			tree, err := treeOf(mustRead(t, p.archive()))
			if err != nil {
				t.Errorf("%s: %v", p.Name, err)
				continue
			}
			if _, ok := tree["typst.toml"]; !ok {
				t.Errorf("%s has no typst.toml", p.Name)
			}
		}
	})
}

func newCache(t *testing.T) *packageCache {
	t.Helper()
	return &packageCache{root: filepath.Join(t.TempDir(), "packages"), fsys: embeddedPackages}
}

func TestPackageCache(t *testing.T) {
	t.Run("first use extracts every package", func(t *testing.T) {
		c := newCache(t)
		fixed, err := c.Ensure()
		if err != nil {
			t.Fatalf("Ensure() = %v", err)
		}
		if len(fixed) != 7 {
			t.Errorf("Ensure() extracted %v, want all 7", fixed)
		}
		for _, name := range []string{"cmarker/0.1.10", "touying/0.7.4", "lilaq/0.6.0"} {
			if _, err := os.Stat(filepath.Join(c.root, "preview", filepath.FromSlash(name), "typst.toml")); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	})

	t.Run("an intact cache is left alone", func(t *testing.T) {
		c := newCache(t)
		if _, err := c.Ensure(); err != nil {
			t.Fatal(err)
		}
		before, _ := os.Stat(filepath.Join(c.root, "preview", "cmarker", "0.1.10", "typst.toml"))
		fixed, err := c.Ensure()
		if err != nil || len(fixed) != 0 {
			t.Fatalf("second Ensure() = %v, %v; want nothing to fix", fixed, err)
		}
		after, _ := os.Stat(filepath.Join(c.root, "preview", "cmarker", "0.1.10", "typst.toml"))
		if !os.SameFile(before, after) {
			t.Error("an intact package was rewritten")
		}
	})

	damage := map[string]func(t *testing.T, dir string){
		"a file with one byte changed": func(t *testing.T, dir string) {
			f := filepath.Join(dir, "typst.toml")
			data := mustReadFile(t, f)
			data[0] ^= 0xff
			mustWrite(t, f, data)
		},
		"a file that is missing": func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "typst.toml")); err != nil {
				t.Fatal(err)
			}
		},
		"an extra file that would be picked up": func(t *testing.T, dir string) {
			mustWrite(t, filepath.Join(dir, "evil.typ"), []byte("#panic()"))
		},
		"a truncated file": func(t *testing.T, dir string) {
			if err := os.Truncate(filepath.Join(dir, "typst.toml"), 3); err != nil {
				t.Fatal(err)
			}
		},
		"a symlink in place of a file": func(t *testing.T, dir string) {
			f := filepath.Join(dir, "typst.toml")
			data := mustReadFile(t, f)
			decoy := filepath.Join(t.TempDir(), "decoy")
			mustWrite(t, decoy, data)
			if err := os.Remove(f); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(decoy, f); err != nil {
				t.Fatal(err)
			}
		},
		"a symlink in place of the package": func(t *testing.T, dir string) {
			target := t.TempDir()
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, dir); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, hurt := range damage {
		t.Run("TYP-12 "+name+" is extracted again", func(t *testing.T) {
			c := newCache(t)
			if _, err := c.Ensure(); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(c.root, "preview", "cmarker", "0.1.10")
			hurt(t, dir)

			fixed, err := c.Ensure()
			if err != nil {
				t.Fatalf("Ensure() = %v", err)
			}
			if !slices.Equal(fixed, []string{"@preview/cmarker:0.1.10"}) {
				t.Errorf("Ensure() fixed %v, want only cmarker", fixed)
			}
			if again, err := c.Ensure(); err != nil || len(again) != 0 {
				t.Errorf("Ensure() after the fix = %v, %v; want it clean", again, err)
			}
		})
	}

	t.Run("concurrent first use", func(t *testing.T) {
		c := newCache(t)
		const n = 6
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				// Separate values over one directory: as separate processes would be.
				other := &packageCache{root: c.root, fsys: c.fsys}
				_, errs[i] = other.Ensure()
			})
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("Ensure #%d = %v", i, err)
			}
		}
		if fixed, err := c.Ensure(); err != nil || len(fixed) != 0 {
			t.Errorf("Ensure() afterwards = %v, %v; want a clean cache", fixed, err)
		}
	})

	t.Run("a missing archive is an error", func(t *testing.T) {
		c := newCache(t)
		c.fsys = fstest.MapFS{}
		if _, err := c.Ensure(); err == nil {
			t.Fatal("Ensure() with no archives = nil, want an error")
		}
	})

	t.Run("an archive that is not the pinned one is refused", func(t *testing.T) {
		c := newCache(t)
		c.fsys = fstest.MapFS{"cmarker-0.1.10.tar.gz": {Data: tarGz(t, map[string]string{"typst.toml": "evil"})}}
		if _, err := c.Ensure(); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("Ensure() = %v, want ErrIntegrity", err)
		}
	})
}

func TestReadArchiveRefuses(t *testing.T) {
	big := strings.Repeat("a", maxPackageFile+1)
	many := map[string]string{}
	for i := range maxPackageFiles + 1 {
		many[fmt.Sprintf("d/f%d", i)] = ""
	}
	tests := map[string]struct {
		archive []byte
	}{
		"TYP-13 parent directory":          {tarGz(t, map[string]string{"../evil.typ": "x"})},
		"TYP-13 buried parent directory":   {tarGz(t, map[string]string{"a/../../evil.typ": "x"})},
		"TYP-13 absolute path":             {tarGz(t, map[string]string{"/etc/evil": "x"})},
		"TYP-13 backslash":                 {tarGz(t, map[string]string{`a\..\evil`: "x"})},
		"TYP-13 symbolic link":             {tarGzEntries(t, tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})},
		"TYP-13 hard link":                 {tarGzEntries(t, tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "typst.toml"})},
		"TYP-13 device":                    {tarGzEntries(t, tar.Header{Name: "dev", Typeflag: tar.TypeChar})},
		"TYP-13 file past the size bound":  {tarGz(t, map[string]string{"big.bin": big})},
		"TYP-13 too many files":            {tarGz(t, many)},
		"TYP-13 not gzip":                  {[]byte("plain text")},
		"TYP-13 truncated tar":             {tarGz(t, map[string]string{"a": "hello"})[:20]},
		"TYP-13 empty name":                {tarGz(t, map[string]string{"": "x"})},
		"TYP-13 same file twice":           {tarGzEntries(t, tar.Header{Name: "a", Typeflag: tar.TypeReg}, tar.Header{Name: "a", Typeflag: tar.TypeReg})},
		"TYP-13 a file where a dir stands": {tarGz(t, map[string]string{"a": "x", "a/b": "y"})},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := treeOf(tt.archive)
			if err == nil {
				t.Fatal("treeOf accepted the archive")
			}
			dest := t.TempDir()
			if err := extractArchive(tt.archive, dest); err == nil {
				t.Fatal("extractArchive accepted the archive")
			}
			if entries, _ := os.ReadDir(filepath.Dir(dest)); len(entries) != 1 {
				t.Errorf("extraction touched the parent directory: %v", entries)
			}
		})
	}
}

func TestReadArchiveAccepts(t *testing.T) {
	archive := tarGzEntries(
		t,
		tar.Header{Name: "./", Typeflag: tar.TypeDir},
		tar.Header{Name: "./sub/", Typeflag: tar.TypeDir},
		tar.Header{Name: "./sub/a.typ", Typeflag: tar.TypeReg, Size: 3},
		tar.Header{Name: "typst.toml", Typeflag: tar.TypeReg},
		tar.Header{Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "x"}},
	)
	tree, err := treeOf(archive)
	if err != nil {
		t.Fatalf("treeOf = %v", err)
	}
	var names []string
	for n := range tree {
		names = append(names, n)
	}
	slices.Sort(names)
	if want := []string{"sub/a.typ", "typst.toml"}; !slices.Equal(names, want) {
		t.Errorf("files = %v, want %v", names, want)
	}
	dest := filepath.Join(t.TempDir(), "out")
	if err := extractArchive(archive, dest); err != nil {
		t.Fatalf("extractArchive = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "sub", "a.typ")); err != nil {
		t.Error(err)
	}
}

// helpers

func tarGz(t testing.TB, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		data := files[n]
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(data)); err != nil {
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

// tarGzEntries builds an archive from raw headers; regular entries get Size
// bytes of 'x'.
func tarGzEntries(t testing.TB, headers ...tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, h := range headers {
		if h.Typeflag != tar.TypeXGlobalHeader {
			h.Mode = 0o644
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(bytes.Repeat([]byte("x"), int(h.Size))); err != nil {
				t.Fatal(err)
			}
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

func mustRead(t testing.TB, name string) []byte {
	t.Helper()
	data, err := fs.ReadFile(embeddedPackages, name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustReadFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustWrite(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readMember(t testing.TB, archive []byte, name string) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			t.Fatalf("member %s: %v", name, err)
		}
		if strings.TrimPrefix(h.Name, "./") == name {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
	}
}
