package sidecar

import (
	"bytes"
	_ "embed" // manifest.json
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strings"
)

// The names of the sidecars.
const (
	// Typst compiles and queries documents.
	Typst = "typst"
	// Tinymist is the language server and incremental preview.
	Tinymist = "tinymist"
)

// manifestSchema is the only manifest layout this code reads.
const manifestSchema = 1

// releaseHost is the only host a sidecar archive may be downloaded from.
const releaseHost = "github.com"

//go:embed manifest.json
var manifestJSON []byte

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Manifest pins every sidecar binary: where it comes from, the SHA-256 of the
// archive and of the binary inside it. It is checked in; the build fetches
// against it and the application verifies against it.
type Manifest struct {
	// Sidecars maps a sidecar name ([Typst], [Tinymist]) to its pins.
	Sidecars map[string]Sidecar `json:"sidecars"`
	// Schema is the manifest layout version; the only one known is 1.
	Schema int `json:"schema"`
}

// Sidecar is one program pinned at one version, for every supported target.
type Sidecar struct {
	// Targets maps "<goos>_<goarch>" to the artifact for that platform.
	Targets map[string]Artifact `json:"targets"`
	// Version is the upstream release, without the leading "v".
	Version string `json:"version"`
	// License is the SPDX identifier of the upstream license.
	License string `json:"license"`
}

// Artifact is the release archive of one sidecar for one platform and the
// binary inside it.
type Artifact struct {
	// URL is the official release archive (https, [releaseHost]).
	URL string `json:"url"`
	// ArchiveSHA256 is the lower-case hex SHA-256 of the archive.
	ArchiveSHA256 string `json:"archive_sha256"`
	// Member is the path of the binary inside the archive.
	Member string `json:"member"`
	// SHA256 is the lower-case hex SHA-256 of the binary itself.
	SHA256 string `json:"sha256"`
	// ArchiveSize is the exact size of the archive in bytes.
	ArchiveSize int64 `json:"archive_size"`
	// Size is the exact size of the binary in bytes.
	Size int64 `json:"size"`
}

// LoadManifest returns the manifest embedded in the executable.
func LoadManifest() (*Manifest, error) { return ParseManifest(manifestJSON) }

// ParseManifest decodes and validates a manifest. Unknown fields, a foreign
// host, a malformed hash or a member path that leaves the archive are errors.
func ParseManifest(data []byte) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("sidecar: manifest: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("sidecar: manifest: trailing data")
	}
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("sidecar: manifest: %w", err)
	}
	return &m, nil
}

func (m *Manifest) validate() error {
	if m.Schema != manifestSchema {
		return fmt.Errorf("schema %d, want %d", m.Schema, manifestSchema)
	}
	if len(m.Sidecars) == 0 {
		return errors.New("no sidecars")
	}
	for name, sc := range m.Sidecars {
		if !nameRE.MatchString(name) {
			return fmt.Errorf("sidecar name %q", name)
		}
		if sc.Version == "" || len(sc.Targets) == 0 {
			return fmt.Errorf("%s: version and targets are required", name)
		}
		for target, a := range sc.Targets {
			if err := a.validate(); err != nil {
				return fmt.Errorf("%s/%s: %w", name, target, err)
			}
		}
	}
	return nil
}

func (a Artifact) validate() error {
	u, err := url.Parse(a.URL)
	if err != nil || u.Scheme != "https" || u.Host != releaseHost {
		return fmt.Errorf("url %q: want https on %s", a.URL, releaseHost)
	}
	for _, h := range []string{a.ArchiveSHA256, a.SHA256} {
		if !isSHA256Hex(h) {
			return fmt.Errorf("%q is not a lower-case hex SHA-256", h)
		}
	}
	if a.ArchiveSize <= 0 || a.Size <= 0 {
		return errors.New("sizes must be positive")
	}
	if a.Member == "" || !isLocalSlashPath(a.Member) {
		return fmt.Errorf("member %q must stay inside the archive", a.Member)
	}
	return nil
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// isLocalSlashPath reports whether p is a clean, relative, slash-separated
// path that does not climb out of its root.
func isLocalSlashPath(p string) bool {
	return p == path.Clean(p) && !path.IsAbs(p) && p != ".." && !strings.HasPrefix(p, "../") && !strings.Contains(p, `\`)
}

// Artifact returns the sidecar pins and the artifact of name for target
// ("<goos>_<goarch>"). The error wraps [ErrUnsupported] when the manifest has
// no such sidecar or target.
func (m *Manifest) Artifact(name, target string) (Sidecar, Artifact, error) {
	sc, ok := m.Sidecars[name]
	if !ok {
		return Sidecar{}, Artifact{}, fmt.Errorf("%w: no sidecar %q in the manifest", ErrUnsupported, name)
	}
	art, ok := sc.Targets[target]
	if !ok {
		return Sidecar{}, Artifact{}, fmt.Errorf("%w: %s has no build for %s", ErrUnsupported, name, target)
	}
	return sc, art, nil
}

// HostTarget is the "<goos>_<goarch>" this executable runs on.
func HostTarget() string { return runtime.GOOS + "_" + runtime.GOARCH }
