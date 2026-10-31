package typst

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	pkgNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	versionRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// Bounds on a package archive: a hostile one cannot fill the disk.
const (
	maxPackageFiles = 2000
	maxPackageFile  = 32 << 20
	maxPackageTotal = 128 << 20
)

//go:embed packages/*.tar.gz
var packagesFS embed.FS

//go:embed packages.json
var packagesJSON []byte

// embeddedPackages holds <name>-<version>.tar.gz for each bundled package.
var embeddedPackages = func() fs.FS {
	sub, err := fs.Sub(packagesFS, "packages")
	if err != nil { // "packages" is a constant that exists.
		panic(err)
	}
	return sub
}()

// bundledPackage is one @preview package shipped inside the executable,
// pinned by the SHA-256 of its archive from packages.typst.org.
type bundledPackage struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	License   string `json:"license"`
	SHA256    string `json:"sha256"`
}

func (p bundledPackage) archive() string { return p.Name + "-" + p.Version + ".tar.gz" }

func (p bundledPackage) spec() string { return "@" + p.Namespace + "/" + p.Name + ":" + p.Version }

// loadBundled decodes packages.json strictly.
func loadBundled() ([]bundledPackage, error) {
	dec := json.NewDecoder(bytes.NewReader(packagesJSON))
	dec.DisallowUnknownFields()
	var doc struct {
		Packages []bundledPackage `json:"packages"`
		Schema   int              `json:"schema"`
	}
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("typst: packages.json: %w", err)
	}
	if doc.Schema != 1 || len(doc.Packages) == 0 {
		return nil, errors.New("typst: packages.json: unknown schema or no packages")
	}
	for _, p := range doc.Packages {
		if p.Namespace != "preview" || !pkgNameRE.MatchString(p.Name) || !versionRE.MatchString(p.Version) || len(p.SHA256) != 64 {
			return nil, fmt.Errorf("typst: packages.json: bad entry %+v", p)
		}
	}
	return doc.Packages, nil
}

// fileSum is the expected content of one file of a package.
type fileSum struct {
	sum  [sha256.Size]byte
	size int64
}

// packageCache keeps the bundled packages extracted under root as Typst's
// package cache expects them: root/preview/<name>/<version>/. Every call to
// [packageCache.Ensure] checks each file against the archive it came from.
type packageCache struct {
	fsys  fs.FS
	trees map[string]map[string]fileSum
	root  string
	mu    sync.Mutex
}

func newPackageCache(root string) *packageCache {
	return &packageCache{root: root, fsys: embeddedPackages}
}

func (c *packageCache) dir(p bundledPackage) string {
	return filepath.Join(c.root, p.Namespace, p.Name, p.Version)
}

// Ensure makes sure every bundled package is on disk exactly as pinned,
// extracting the ones that are missing or differ in any way (a changed,
// truncated, added or replaced file, a link). It returns the packages it had to
// extract.
func (c *packageCache) Ensure() ([]string, error) {
	pkgs, err := loadBundled()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var fixed []string
	for _, p := range pkgs {
		want, err := c.expected(p)
		if err != nil {
			return fixed, err
		}
		if verifyTree(c.dir(p), want) == nil {
			continue
		}
		if err := c.extract(p, want); err != nil {
			return fixed, err
		}
		fixed = append(fixed, p.spec())
	}
	return fixed, nil
}

// expected returns the files the package must hold, checking the archive
// against its pinned hash the first time.
func (c *packageCache) expected(p bundledPackage) (map[string]fileSum, error) {
	if want, ok := c.trees[p.spec()]; ok {
		return want, nil
	}
	data, err := c.read(p)
	if err != nil {
		return nil, err
	}
	want, err := treeOf(data)
	if err != nil {
		return nil, fmt.Errorf("typst: bundled %s: %w", p.spec(), err)
	}
	if c.trees == nil {
		c.trees = map[string]map[string]fileSum{}
	}
	c.trees[p.spec()] = want
	return want, nil
}

// read returns the archive of p after checking its pinned SHA-256.
func (c *packageCache) read(p bundledPackage) ([]byte, error) {
	data, err := fs.ReadFile(c.fsys, p.archive())
	if err != nil {
		return nil, fmt.Errorf("typst: bundled %s: %w", p.spec(), err)
	}
	if got := sumHex(data); got != p.SHA256 {
		return nil, fmt.Errorf("%w: bundled %s has sha256 %s, pinned %s", ErrIntegrity, p.spec(), got, p.SHA256)
	}
	return data, nil
}

func (c *packageCache) extract(p bundledPackage, want map[string]fileSum) error {
	data, err := c.read(p)
	if err != nil {
		return err
	}
	err = installTree(c.dir(p), want, func(tmp string) error { return extractArchive(data, tmp) })
	if err != nil {
		return fmt.Errorf("typst: install %s: %w", p.spec(), err)
	}
	return nil
}

// installTree builds a directory with fill in a temporary sibling of final and
// moves it into place, replacing whatever was there. Other processes (a second
// Inkwell window) may be doing the same: whoever finds final already equal to
// want stops, and a lost race is retried.
func installTree(final string, want map[string]fileSum, fill func(tmp string) error) error {
	parent := filepath.Dir(final)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(final)+".tmp-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := fill(tmp); err != nil {
		return err
	}
	var lastErr error
	for range 5 {
		if verifyTree(final, want) == nil {
			return nil
		}
		if err := os.RemoveAll(final); err != nil {
			return err
		}
		if lastErr = os.Rename(tmp, final); lastErr == nil {
			return nil
		}
	}
	return lastErr
}

// verifyTree reports an error unless dir is a real directory holding exactly
// the files of want, with the same content, and nothing else: a link, an extra
// file or any difference is an error.
func verifyTree(dir string, want map[string]fileSum) error {
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return errors.New("not a directory")
	}
	seen := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		exp, ok := want[filepath.ToSlash(rel)]
		if !ok || !d.Type().IsRegular() {
			return fmt.Errorf("unexpected %s", rel)
		}
		got, err := hashFile(p)
		if err != nil || got != exp {
			return fmt.Errorf("%s differs", rel)
		}
		seen++
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(want) {
		return errors.New("files are missing")
	}
	return nil
}

func hashFile(p string) (fileSum, error) {
	f, err := os.Open(p) // #nosec G304 -- inside the cache directory the engine owns
	if err != nil {
		return fileSum{}, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxPackageFile+1))
	if err != nil || n > maxPackageFile {
		return fileSum{}, errors.New("unreadable or too large")
	}
	var s fileSum
	copy(s.sum[:], h.Sum(nil))
	s.size = n
	return s, nil
}

func sumHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// treeOf lists the regular files of a package archive with their size and
// SHA-256, refusing an unsafe archive.
func treeOf(archive []byte) (map[string]fileSum, error) {
	tree := map[string]fileSum{}
	err := readArchive(archive, func(name string, dir bool, r io.Reader) error {
		if dir {
			return nil
		}
		h := sha256.New()
		n, err := io.Copy(h, r)
		if err != nil {
			return err
		}
		s := fileSum{size: n}
		copy(s.sum[:], h.Sum(nil))
		tree[name] = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// extractArchive writes a package archive under dest, which must exist and be
// empty. Nothing outside dest can be written: the writes go through an
// [os.Root], and the archive is checked first.
func extractArchive(archive []byte, dest string) error {
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return readArchive(archive, func(name string, dir bool, r io.Reader) error {
		if dir {
			return root.MkdirAll(name, 0o700)
		}
		if d := path.Dir(name); d != "." {
			if err := root.MkdirAll(d, 0o700); err != nil {
				return err
			}
		}
		f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, r); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	})
}

// readArchive walks a .tar.gz of a package, calling visit for each directory
// and regular file with a cleaned, slash-separated, relative name. Links,
// devices, names that leave the package, duplicates, and archives past the
// bounds are refused with [ErrUnsafeArchive]; visit never sees them.
func readArchive(archive []byte, visit func(name string, dir bool, r io.Reader) error) error {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnsafeArchive, err)
	}
	defer func() { _ = zr.Close() }()
	tr := tar.NewReader(zr)
	entries := archiveEntries{dirs: map[string]bool{}, seen: map[string]bool{}}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrUnsafeArchive, err)
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name, err := entries.validate(h)
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeDir && name == "" {
			continue // the package folder itself
		}
		var r io.Reader
		if h.Typeflag == tar.TypeReg {
			r = io.LimitReader(tr, h.Size)
		}
		if err := visit(name, h.Typeflag == tar.TypeDir, r); err != nil {
			return err
		}
	}
}

// archiveEntries accounts for file limits and names already used by an archive.
type archiveEntries struct {
	files      int
	total      int64
	dirs, seen map[string]bool
}

// validate checks an entry before the visitor can observe it.
func (e *archiveEntries) validate(h *tar.Header) (string, error) {
	if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
		return "", fmt.Errorf("%w: %q is not a file or a directory", ErrUnsafeArchive, h.Name)
	}
	name, ok := cleanMember(h.Name)
	if !ok {
		return "", fmt.Errorf("%w: bad name %q", ErrUnsafeArchive, h.Name)
	}
	if h.Typeflag == tar.TypeDir {
		if name == "" {
			return "", nil // the package folder itself
		}
		if e.seen[name] {
			return "", fmt.Errorf("%w: %q is both a file and a directory", ErrUnsafeArchive, name)
		}
		markDirs(e.dirs, name)
		return name, nil
	}
	e.files++
	e.total += h.Size
	switch {
	case name == "":
		return "", fmt.Errorf("%w: a file without a name", ErrUnsafeArchive)
	case e.files > maxPackageFiles:
		return "", fmt.Errorf("%w: more than %d files", ErrUnsafeArchive, maxPackageFiles)
	case h.Size < 0 || h.Size > maxPackageFile:
		return "", fmt.Errorf("%w: %q is larger than %d bytes", ErrUnsafeArchive, name, maxPackageFile)
	case e.total > maxPackageTotal:
		return "", fmt.Errorf("%w: more than %d bytes in all", ErrUnsafeArchive, maxPackageTotal)
	case e.seen[name] || e.dirs[name]:
		return "", fmt.Errorf("%w: %q appears twice", ErrUnsafeArchive, name)
	}
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		if e.seen[parent] {
			return "", fmt.Errorf("%w: %q is a file and a directory", ErrUnsafeArchive, parent)
		}
	}
	e.seen[name] = true
	markDirs(e.dirs, path.Dir(name))

	return name, nil
}

// markDirs records name and its parents as directories.
func markDirs(dirs map[string]bool, name string) {
	for ; name != "." && name != ""; name = path.Dir(name) {
		dirs[name] = true
	}
}

// cleanMember normalises an archive member name. The empty name means the
// package folder itself.
func cleanMember(name string) (string, bool) {
	if strings.ContainsAny(name, "\\\x00") {
		return "", false
	}
	for strings.HasPrefix(name, "./") {
		name = name[2:]
	}
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "." {
		return "", true
	}
	if !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}
