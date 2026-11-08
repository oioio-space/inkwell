package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	snapshotFileLimit  = 64 << 20
	snapshotTotalLimit = 512 << 20
	snapshotFilesLimit = 10000
)

// ProjectionSnapshot describes the exact immutable bytes copied for one compile.
// TextHashes use decoded UTF-8/LF to match webview documents; RawHashes include
// every copied dependency, including binary assets and vendored packages.
type ProjectionSnapshot struct{ TextHashes, RawHashes map[string]string }

// SnapshotTo copies regular project files into a private empty destination.
// It never follows links, shares writable inodes, or includes Inkwell internals.
func (p *Project) SnapshotTo(ctx context.Context, target string) (ProjectionSnapshot, error) {
	p.snapshotMu.Lock()
	defer p.snapshotMu.Unlock()
	if p.snapshotEntries == nil {
		p.snapshotEntries = map[string]snapshotEntry{}
	}
	if p.snapshotObjects == "" {
		identity := sha256.Sum256([]byte(p.root.Name()))
		p.snapshotObjects = filepath.Join(filepath.Dir(target), ".objects-"+hex.EncodeToString(identity[:8]))
		if err := os.MkdirAll(p.snapshotObjects, 0o700); err != nil {
			return ProjectionSnapshot{}, err
		}
	}
	dst, err := os.OpenRoot(target)
	if err != nil {
		return ProjectionSnapshot{}, err
	}
	defer func() { _ = dst.Close() }()
	result, err := p.snapshotFiles(ctx, dst)
	// A failed walk may already have ingested changed sources. Keep only
	// objects reached by this attempt; active roots retain their own hardlinks.
	p.pruneSnapshotObjects(result)
	return result, err
}

// MatchesSnapshot checks all dependencies, including additions and removals.
func (p *Project) MatchesSnapshot(ctx context.Context, want ProjectionSnapshot) bool {
	p.snapshotMu.Lock()
	defer p.snapshotMu.Unlock()
	got, err := p.snapshotFiles(ctx, nil)
	return err == nil && maps.Equal(got.RawHashes, want.RawHashes)
}

func snapshotVisible(name string, entry fs.DirEntry) bool {
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") && part != ".typstdeps" {
			return false
		}
	}
	return name == "." || !strings.HasSuffix(entry.Name(), tmpSuffix)
}

func (p *Project) snapshotFiles(ctx context.Context, dst *os.Root) (ProjectionSnapshot, error) {
	w := snapshotWalk{ctx: ctx, dst: dst, project: p, snapshot: ProjectionSnapshot{TextHashes: map[string]string{}, RawHashes: map[string]string{}}}
	err := fs.WalkDir(p.root.FS(), ".", w.visit)
	return w.snapshot, err
}

type snapshotWalk struct {
	ctx      context.Context
	dst      *os.Root
	project  *Project
	snapshot ProjectionSnapshot
	entries  int
	total    int64
}

func (w *snapshotWalk) visit(name string, entry fs.DirEntry, walkErr error) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if walkErr != nil {
		return walkErr
	}
	if name == "." {
		return nil
	}
	if !snapshotVisible(name, entry) {
		if entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	w.entries++
	if w.entries > snapshotFilesLimit {
		return errors.New("project snapshot: too many files")
	}
	if entry.Type()&fs.ModeSymlink != 0 {
		return errors.New("project snapshot: symbolic link refused")
	}
	if entry.IsDir() {
		if w.dst != nil {
			return w.dst.MkdirAll(name, 0o700)
		}
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || hardLinks(info) != 1 {
		return errors.New("project snapshot: nonregular file or hardlink refused")
	}
	if info.Size() > snapshotFileLimit || info.Size() > snapshotTotalLimit-w.total {
		return errors.New("project snapshot: byte bound exceeded")
	}

	content, err := w.project.snapshotContent(name, info, w.dst != nil)
	if err != nil {
		return err
	}
	w.total += content.size
	if w.total > snapshotTotalLimit {
		return errors.New("project snapshot: byte bound exceeded")
	}
	w.snapshot.RawHashes[name] = content.raw
	if content.text != "" {
		w.snapshot.TextHashes[name] = content.text
	}
	if w.dst != nil {
		if err := w.dst.MkdirAll(path.Dir(name), 0o700); err != nil {
			return err
		}
		// Only our private immutable object is linked. Untrusted source inodes
		// are always read and copied; source hardlinks remain forbidden.
		return os.Link(content.object, filepath.Join(w.dst.Name(), filepath.FromSlash(name)))
	}

	return nil
}

func (p *Project) snapshotRead(name string, info fs.FileInfo) ([]byte, error) {
	file, err := p.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) || !opened.Mode().IsRegular() || hardLinks(opened) != 1 {
		return nil, errors.New("project snapshot: file replaced")
	}
	raw, err := io.ReadAll(io.LimitReader(file, snapshotFileLimit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > snapshotFileLimit {
		return nil, errors.New("project snapshot: file grew past bound")
	}
	return raw, nil
}

type snapshotEntry struct {
	version, raw, text, object string
	size                       int64
}

func (p *Project) snapshotContent(name string, info fs.FileInfo, materialize bool) (snapshotEntry, error) {
	version := snapshotFileVersion(info)
	if entry, ok := p.snapshotEntries[name]; ok && version != "" && entry.version == version {
		object, err := os.Stat(entry.object)
		if err == nil && object.Mode().IsRegular() && object.Size() == entry.size && object.Mode().Perm() == 0o400 {
			return entry, nil
		}
	}
	raw, err := p.snapshotRead(name, info)
	if err != nil {
		return snapshotEntry{}, err
	}
	current, err := p.root.Stat(name)
	if err != nil {
		return snapshotEntry{}, err
	}
	if version != "" && version != snapshotFileVersion(current) {
		return snapshotEntry{}, errors.New("project snapshot: source changed during read")
	}
	digest := sha256.Sum256(raw)
	entry := snapshotEntry{version: version, raw: hex.EncodeToString(digest[:]), size: int64(len(raw))}
	if text, err := Decode(raw); err == nil {
		hash := sha256.Sum256([]byte(text.Body))
		entry.text = hex.EncodeToString(hash[:])
	}
	if materialize {
		entry.object = filepath.Join(p.snapshotObjects, entry.raw)
		if err := writeSnapshotObject(entry.object, raw); err != nil {
			return snapshotEntry{}, err
		}
		p.snapshotEntries[name] = entry
	}
	return entry, nil
}

func writeSnapshotObject(name string, raw []byte) error {
	if info, err := os.Lstat(name); err == nil && info.Mode().IsRegular() && info.Size() == int64(len(raw)) && info.Mode().Perm() == 0o400 {
		return nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(name), "object-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary.Name()) }()
	_, writeErr := temporary.Write(raw)
	if writeErr == nil {
		writeErr = temporary.Chmod(0o400)
	}
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), name)
}

func (p *Project) pruneSnapshotObjects(current ProjectionSnapshot) {
	used := map[string]bool{}
	for _, hash := range current.RawHashes {
		used[hash] = true
	}
	for name := range p.snapshotEntries {
		if _, ok := current.RawHashes[name]; !ok {
			delete(p.snapshotEntries, name)
		}
	}
	entries, err := os.ReadDir(p.snapshotObjects)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && !used[entry.Name()] {
			_ = os.Remove(filepath.Join(p.snapshotObjects, entry.Name()))
		}
	}
}
