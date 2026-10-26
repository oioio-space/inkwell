// Package sidecar delivers and runs the programs Inkwell ships next to itself:
// typst (compile, query) and tinymist (language server, preview).
//
// The binaries are pinned in manifest.json, fetched at build time by
// `mise run sidecar:fetch` (cmd/sidecar-fetch), gzip-compressed and embedded
// with go:embed for the platform being built. Nothing is downloaded when the
// application runs.
//
// At run time [Store.Resolve] extracts a binary once into the user cache
// directory and, on every call, hashes the extracted file against the pinned
// SHA-256: a file that is missing, truncated, replaced by a link or modified is
// removed and extracted again from the embedded copy, which is itself checked
// while it is decompressed. [Run] and [Supervisor] then launch the verified
// path in its own process group, with a bounded lifetime and no orphan left
// behind when Inkwell dies (Linux).
//
//	store := &sidecar.Store{}
//	path, err := store.Resolve(sidecar.Typst) // verified, ready to execute
//	out, err := sidecar.Run(ctx, sidecar.Spec{Path: path, Args: []string{"--version"}})
//
// The check protects against corruption, an interrupted extraction and a
// replacement made by someone who cannot write as the owner. It does not
// defend against a process running as the same user, which can change
// Inkwell itself.
package sidecar

import (
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	// ErrUnsupported is returned for a platform or a sidecar the manifest does
	// not pin.
	ErrUnsupported = errors.New("sidecar: unsupported")
	// ErrNotEmbedded is returned when the executable was built without the
	// sidecar binaries (`mise run sidecar:fetch` was not run).
	ErrNotEmbedded = errors.New("sidecar: binary not embedded")
	// ErrIntegrity is returned when an embedded binary does not match its
	// pinned SHA-256 or size. It is never executed.
	ErrIntegrity = errors.New("sidecar: integrity check failed")
)

// Store extracts and verifies the embedded sidecars. The zero value is ready
// for use on the machine it runs on; the fields exist for tests.
type Store struct {
	// Logger receives a warning each time a cached binary fails verification.
	// The default is [slog.Default].
	Logger *slog.Logger
	// FS holds <name>.gz for each sidecar. The default is [Embedded].
	FS fs.FS
	// Manifest holds the pins. The default is [LoadManifest].
	Manifest *Manifest
	// Dir is the Inkwell cache directory; sidecars live in Dir/sidecar. The
	// default is [DefaultDir].
	Dir string
	// Target is "<goos>_<goarch>". The default is [HostTarget].
	Target string

	mu sync.Mutex
}

// Embedded returns the sidecars embedded for the platform being built:
// <name>.gz files. It holds no sidecar when `mise run sidecar:fetch` has not
// run before the build.
func Embedded() fs.FS {
	sub, err := fs.Sub(embeddedFiles, embeddedDir)
	if err != nil { // embeddedDir is a constant that exists.
		panic(err)
	}
	return sub
}

// DefaultDir is the Inkwell directory in the user's cache.
func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("sidecar: user cache dir: %w", err)
	}
	return filepath.Join(base, "inkwell"), nil
}

// Resolve returns the path of the verified binary of the named sidecar,
// extracting it first when it is absent or does not match its pinned SHA-256.
// The file is hashed on every call, so call it right before each launch.
func (s *Store) Resolve(name string) (string, error) {
	manifest := s.Manifest
	if manifest == nil {
		var err error
		if manifest, err = LoadManifest(); err != nil {
			return "", err
		}
	}
	sc, art, err := manifest.Artifact(name, cmp.Or(s.Target, HostTarget()))
	if err != nil {
		return "", err
	}
	root := s.Dir
	if root == "" {
		if root, err = DefaultDir(); err != nil {
			return "", err
		}
	}
	dir := filepath.Join(root, "sidecar", name+"-"+sc.Version+"-"+art.SHA256[:12])
	path := filepath.Join(dir, name)

	s.mu.Lock()
	defer s.mu.Unlock()
	problem := verify(path, art)
	if problem == "" {
		return path, nil
	}
	if problem != problemMissing {
		s.logger().Warn("sidecar: cached binary failed verification, re-extracting",
			"sidecar", name, "path", path, "problem", problem)
	}
	if err := s.extract(name, dir, path, art); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *Store) files() fs.FS {
	if s.FS != nil {
		return s.FS
	}
	return Embedded()
}

const problemMissing = "missing"

// verify returns "" when path is a regular file holding exactly art, else the
// reason it is not. A symbolic link is a problem even when it leads to the
// right bytes: it can be repointed after the check.
func verify(path string, art Artifact) string {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return problemMissing
	case err != nil:
		return err.Error()
	case !info.Mode().IsRegular():
		return "not a regular file"
	case info.Size() != art.Size:
		return "size differs from the pin"
	}
	f, err := os.Open(path) // #nosec G304 -- the path is built from the cache dir and the pinned name
	if err != nil {
		return err.Error()
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err.Error()
	}
	if hex.EncodeToString(h.Sum(nil)) != art.SHA256 {
		return "sha256 differs from the pin"
	}
	return ""
}

// extract writes the embedded binary to path through a temporary file in the
// same directory, so that a concurrent or interrupted extraction never leaves
// a partial file under the final name.
func (s *Store) extract(name, dir, path string, art Artifact) (err error) {
	if err := ensurePrivateDir(dir); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("sidecar: remove %s: %w", path, err)
	}
	src, err := s.files().Open(name + ".gz")
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s.gz is not in this build, run `mise run sidecar:fetch` and rebuild", ErrNotEmbedded, name)
	}
	if err != nil {
		return fmt.Errorf("sidecar: open embedded %s: %w", name, err)
	}
	defer func() { _ = src.Close() }()
	zr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("%w: embedded %s: %w", ErrIntegrity, name, err)
	}
	defer func() { _ = zr.Close() }()

	tmp, err := os.CreateTemp(dir, name+".tmp-*")
	if err != nil {
		return fmt.Errorf("sidecar: extract %s: %w", name, err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	h := sha256.New()
	// One byte more than pinned: an oversized stream is detected without
	// writing an unbounded file.
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(zr, art.Size+1))
	switch {
	case err != nil:
		return fmt.Errorf("%w: embedded %s: %w", ErrIntegrity, name, err)
	case n != art.Size:
		return fmt.Errorf("%w: embedded %s is %d bytes, pinned %d", ErrIntegrity, name, n, art.Size)
	case hex.EncodeToString(h.Sum(nil)) != art.SHA256:
		return fmt.Errorf("%w: embedded %s does not match its pinned SHA-256", ErrIntegrity, name)
	}
	if err := tmp.Chmod(0o500); err != nil {
		return fmt.Errorf("sidecar: extract %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("sidecar: extract %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("sidecar: extract %s: %w", name, err)
	}
	return nil
}

// ensurePrivateDir makes dir an owner-only directory (not a link).
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("sidecar: create %s: %w", dir, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("sidecar: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("sidecar: %s is not a directory", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- a directory, owner only
			return fmt.Errorf("sidecar: make %s private: %w", dir, err)
		}
	}
	return nil
}
