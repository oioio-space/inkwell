// Package fetch downloads the pinned sidecar archives at BUILD time and turns
// them into the files the executable embeds: bin/<os>_<arch>/<name>.gz.
//
// It is used by cmd/sidecar-fetch only (mise run sidecar:fetch); the
// application never imports it, so the shipped executable holds no download
// code (checked by scripts/arch-check.sh).
//
// Every step is pinned and verified: the archive against its SHA-256 and exact
// size, then the binary inside it against its own SHA-256 and size. Anything
// that does not match is refused and leaves nothing behind. A second run with
// everything in place makes no request.
package fetch

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

// ErrMismatch is returned when downloaded or extracted bytes do not match
// their pinned SHA-256 or size.
var ErrMismatch = errors.New("sidecar fetch: does not match the pinned checksum")

const (
	downloadTimeout = 15 * time.Minute
	maxRedirects    = 5
	stderrKept      = 4 << 10
)

// Config says what to fetch and where to put it.
type Config struct {
	// Manifest pins the sidecars.
	Manifest *sidecar.Manifest
	// Client downloads the archives. The default has a 15 minute timeout.
	Client *http.Client
	// AllowHost reports whether a "host[:port]" may be contacted, including by
	// a redirect. The default is [DefaultAllowHost].
	AllowHost func(host string) bool
	// Log receives one line per sidecar and target; the default discards.
	Log io.Writer
	// OutDir is internal/sidecar/bin: the files go to OutDir/<target>/<name>.gz.
	OutDir string
	// CacheDir keeps the downloaded archives so that other checkouts and later
	// runs need no network.
	CacheDir string
	// Targets lists the "<goos>_<goarch>" to fetch.
	Targets []string
	// Names lists the sidecars to fetch; the default is all of them.
	Names []string
}

// DefaultAllowHost accepts GitHub and the hosts GitHub serves release assets
// from, and nothing else.
func DefaultAllowHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// Fetch makes OutDir/<target>/<name>.gz hold the pinned binary of each
// requested sidecar and target.
func Fetch(ctx context.Context, cfg Config) error {
	if cfg.Manifest == nil {
		return errors.New("sidecar fetch: no manifest")
	}
	names := cfg.Names
	if len(names) == 0 {
		names = slices.Sorted(maps.Keys(cfg.Manifest.Sidecars))
	}
	for _, name := range names {
		for _, target := range cfg.Targets {
			if err := cfg.fetchOne(ctx, name, target); err != nil {
				return fmt.Errorf("%s %s: %w", name, target, err)
			}
		}
	}
	return nil
}

func (c *Config) logf(format string, args ...any) {
	if c.Log != nil {
		_, _ = fmt.Fprintf(c.Log, format+"\n", args...)
	}
}

func (c *Config) fetchOne(ctx context.Context, name, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, art, err := c.Manifest.Artifact(name, target)
	if err != nil {
		return err
	}
	out := filepath.Join(c.OutDir, target, name+".gz")
	if upToDate(out, art) {
		c.logf("%s %s: up to date", name, target)
		return nil
	}
	archive, downloaded, err := c.archive(ctx, art)
	if err != nil {
		return err
	}
	if err := extract(ctx, archive, out, art); err != nil {
		return err
	}
	if downloaded {
		c.logf("%s %s: downloaded and verified", name, target)
	} else {
		c.logf("%s %s: rebuilt from the download cache", name, target)
	}
	return nil
}

// upToDate reports whether out is a gzip of exactly the pinned binary.
func upToDate(out string, art sidecar.Artifact) bool {
	f, err := os.Open(out) // #nosec G304 -- OutDir is the build's own directory
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return false
	}
	return sha256Exact(zr, art.Size) == art.SHA256
}

// archive returns the path of the verified archive in the download cache,
// downloading it when absent or damaged.
func (c *Config) archive(ctx context.Context, art sidecar.Artifact) (path string, downloaded bool, err error) {
	if err := os.MkdirAll(c.CacheDir, 0o750); err != nil {
		return "", false, err
	}
	path = filepath.Join(c.CacheDir, art.ArchiveSHA256+".archive")
	if hashOf(path, art.ArchiveSize) == art.ArchiveSHA256 {
		return path, false, nil
	}
	if err := c.download(ctx, art, path); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// hashOf returns the SHA-256 of the file when it is exactly size bytes, else "".
func hashOf(path string, size int64) string {
	f, err := os.Open(path) // #nosec G304 -- CacheDir is the build's own directory
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	return sha256Exact(f, size)
}

// sha256Exact returns the SHA-256 of r when it holds exactly size bytes, else
// "". It reads one byte more than size at most.
func sha256Exact(r io.Reader, size int64) string {
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(r, size+1))
	if err != nil || n != size {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Config) allow(host string) bool {
	if c.AllowHost != nil {
		return c.AllowHost(host)
	}
	return DefaultAllowHost(host)
}

func (c *Config) download(ctx context.Context, art sidecar.Artifact, dest string) (err error) {
	u, err := url.Parse(art.URL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || !c.allow(u.Host) {
		return fmt.Errorf("refusing to download %s: want https on an allowed host", art.URL)
	}
	client := http.Client{Timeout: downloadTimeout}
	if c.Client != nil {
		client = *c.Client
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" || !c.allow(req.URL.Host) {
			return fmt.Errorf("refusing redirect to %s", req.URL.Redacted())
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, art.URL, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req) // #nosec G107 G704 -- scheme and host are checked above and on every redirect
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", art.URL, resp.Status)
	}

	tmp, err := os.CreateTemp(c.CacheDir, "download-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	h := sha256.New()
	// One byte more than pinned: a server that sends too much is caught
	// without filling the disk.
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, art.ArchiveSize+1))
	if err != nil {
		return err
	}
	if n != art.ArchiveSize {
		return fmt.Errorf("%w: archive is %d bytes, pinned %d", ErrMismatch, n, art.ArchiveSize)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != art.ArchiveSHA256 {
		return fmt.Errorf("%w: archive sha256 %s, pinned %s", ErrMismatch, got, art.ArchiveSHA256)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// extract streams the archive member out of tar (which reads xz and gzip),
// verifies it against the pins and writes it gzip-compressed to out. tar only
// writes to its standard output: no path from the archive ever touches the
// file system.
func extract(ctx context.Context, archive, out string, art sidecar.Artifact) (err error) {
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(out), filepath.Base(out)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	cmd := exec.CommandContext(ctx, "tar", "-xOf", archive, art.Member) // #nosec G204 -- fixed program; archive is our cache file, member is pinned
	stderr := &boundedBuffer{max: stderrKept}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start tar (needed to read the archives): %w", err)
	}

	// Level and header are fixed so the same input gives the same bytes.
	zw, err := gzip.NewWriterLevel(tmp, gzip.BestCompression)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(zw, h), io.LimitReader(stdout, art.Size+1))
	if n > art.Size || copyErr != nil {
		_ = cmd.Process.Kill() // tar may be blocked writing more than we will read
	}
	waitErr := cmd.Wait()
	switch {
	case copyErr != nil:
		return copyErr
	case n > art.Size:
		return fmt.Errorf("%w: %s is larger than the pinned %d bytes", ErrMismatch, art.Member, art.Size)
	case waitErr != nil:
		return fmt.Errorf("extract %s: %w: %s", art.Member, waitErr, strings.TrimSpace(stderr.String()))
	case n != art.Size:
		return fmt.Errorf("%w: %s is %d bytes, pinned %d (not a regular file?)", ErrMismatch, art.Member, n, art.Size)
	case hex.EncodeToString(h.Sum(nil)) != art.SHA256:
		return fmt.Errorf("%w: %s does not match its pinned SHA-256", ErrMismatch, art.Member)
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), out)
}

// boundedBuffer keeps the first max bytes written to it.
type boundedBuffer struct {
	sb  strings.Builder
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.sb.Len(); room > 0 {
		b.sb.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string { return b.sb.String() }
