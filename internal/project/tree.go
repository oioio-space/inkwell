package project

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"

	"lukechampine.com/blake3"

	"github.com/oioio-space/inkwell/internal/store"
)

// MaxNodes is the most entries a project tree may have (ADR 0023). The
// webview drops the ones beyond it, and Go refuses a longer list.
const MaxNodes = 10000

// NodeType says what an entry of the project tree is.
type NodeType uint8

// The node types. The numeric values are part of the store stream protocol.
const (
	// NodeDir is a folder.
	NodeDir NodeType = iota
	// NodeText is a text file, a document of its own.
	NodeText
	// NodeBlob is a binary file, named by its hash.
	NodeBlob
)

// String returns "dir", "text" or "blob".
func (t NodeType) String() string {
	switch t {
	case NodeDir:
		return "dir"
	case NodeText:
		return "text"
	case NodeBlob:
		return "blob"
	}
	return "NodeType(?)"
}

// TreeEntry is one node of the project tree, as the webview materialized it
// from the project index (ADR 0023). Go does not read the index: it takes this
// list as a claim of a peer, and checks every path before it touches the disk.
type TreeEntry struct {
	// Path is relative to the project folder, with forward slashes.
	Path string
	// URL is the automerge URL of the document of a text file.
	URL string
	// Size is the length in bytes the index gives a blob.
	Size int64
	// Hash is the BLAKE3 hash of a blob.
	Hash [32]byte
	// Type says which of URL and Hash is set.
	Type NodeType
	// Main marks the project's main document.
	Main bool
}

// ref is what identifies the content behind a file path, so that a renamed
// file is recognized: the document for a text, the hash for a blob.
func (e TreeEntry) ref() string {
	switch e.Type {
	case NodeText:
		return "text:" + e.URL
	case NodeBlob:
		return "blob:" + hex.EncodeToString(e.Hash[:])
	}
	return ""
}

// Rejection says why an entry of a tree was not applied.
type Rejection struct {
	// Path is the path as given, which may be hostile: log it only quoted.
	Path string
	// Err explains the refusal; it wraps [ErrInvalidName] for a bad name and
	// [ErrTreeConflict] for a collision.
	Err error
}

// ErrTreeConflict is wrapped by the [Rejection] of an entry that collides with
// another: the same name once case and normalization are ignored, or a file
// where a folder is needed.
var ErrTreeConflict = errors.New("project: tree entries collide")

// ErrBlobMoved is returned by [Project.WriteBlob] when the tree no longer
// lists the blob at that path: the write is stale.
var ErrBlobMoved = errors.New("project: blob is not at that path of the tree")

// TreeResult is what [Project.ApplyTree] did.
type TreeResult struct {
	// Rejected lists the entries that were refused, in path order. A hostile
	// or colliding entry never stops the others.
	Rejected []Rejection
	// Blobs lists the accepted blob entries, in path order: the files whose
	// visible copy [Project.WriteBlob] writes once the blob is in the store.
	Blobs []TreeEntry
}

// checkTree validates entries and returns the accepted ones, ordered by path,
// and the refusals. A path is refused when a name is not portable (see
// [ValidatePath]), when the tree is deeper than [MaxDepth], when it collides
// with an earlier path once case and Unicode normalization are ignored (the
// path that sorts first wins), or when a file stands where a folder is needed.
func checkTree(entries []TreeEntry) (accepted []TreeEntry, rejected []Rejection) {
	sorted := slices.SortedFunc(slices.Values(entries), func(a, b TreeEntry) int { return cmp.Compare(a.Path, b.Path) })
	seats := map[string]seat{}
	for _, e := range sorted {
		if err := checkEntry(e); err != nil {
			rejected = append(rejected, Rejection{Path: e.Path, Err: err})
			continue
		}
		if err := claim(seats, e); err != nil {
			rejected = append(rejected, Rejection{Path: e.Path, Err: err})
			continue
		}
		accepted = append(accepted, e)
	}
	return accepted, rejected
}

// checkEntry validates one entry on its own.
func checkEntry(e TreeEntry) error {
	if err := ValidatePath(e.Path); err != nil {
		return err
	}
	if strings.Count(e.Path, "/")+1 > MaxDepth {
		return fmt.Errorf("%w: %d levels deep", ErrTreeConflict, strings.Count(e.Path, "/")+1)
	}
	switch e.Type {
	case NodeDir:
	case NodeText:
		if e.URL == "" {
			return errors.New("project: text entry without a document")
		}
	case NodeBlob:
		if e.Size < 0 {
			return errors.New("project: blob entry with a negative size")
		}
	default:
		return fmt.Errorf("project: entry of type %d", e.Type)
	}
	return nil
}

// seat is what the tree holds at one folded path.
type seat struct {
	spelled string // the path as the first entry that needed it wrote it
	typ     NodeType
}

// claim records e in seats, which maps each folded path the tree holds so far
// to what is there, or returns the conflict. The folders above a file count as
// held, spelled as the file spells them.
func claim(seats map[string]seat, e TreeEntry) error {
	parts := strings.Split(e.Path, "/")
	folded := make([]string, len(parts))
	for i, c := range parts {
		folded[i] = CollisionKey(c)
	}
	for i := range parts {
		key, spelled := strings.Join(folded[:i+1], "/"), strings.Join(parts[:i+1], "/")
		held, ok := seats[key]
		switch {
		case !ok:
		case held.spelled != spelled:
			return fmt.Errorf("%w: same name as another entry", ErrTreeConflict)
		case i < len(parts)-1 && held.typ != NodeDir:
			return fmt.Errorf("%w: a file stands where a folder is needed", ErrTreeConflict)
		case i == len(parts)-1 && (held.typ != NodeDir || e.Type != NodeDir):
			return fmt.Errorf("%w: same name as another entry", ErrTreeConflict)
		}
	}
	for i := range parts[:len(parts)-1] {
		key := strings.Join(folded[:i+1], "/")
		if _, ok := seats[key]; !ok {
			seats[key] = seat{spelled: strings.Join(parts[:i+1], "/"), typ: NodeDir}
		}
	}
	seats[strings.Join(folded, "/")] = seat{spelled: e.Path, typ: e.Type}
	return nil
}

// ApplyTree takes the project tree from the webview and projects it on disk.
// Every entry is validated first (see [checkTree]); the ones refused are
// reported and skipped. For the others:
//
//   - folders are created, with the folders above every file;
//   - a text file becomes owned by Inkwell, so that [Project.WriteProjection]
//     writes it, as main.typ always was;
//   - a blob is listed in the result: its visible copy is a plain file that
//     [Project.WriteBlob] writes once the blob is in the store;
//   - a file that moved in the tree (same document or same hash, new path) is
//     moved on disk, not written again, if Inkwell wrote it and it is
//     unchanged.
//
// A tree is a claim of a peer, so it never gets a path that exists on disk
// unless Inkwell wrote the file there: such an entry is refused with
// [ErrTreeConflict], as is one under a symbolic link. Files and folders of the
// user are never moved. A file that left the tree is not deleted, but its
// ownership is dropped if Inkwell never wrote it. It returns an error only for
// a failure of the disk, after doing as much as it could.
func (p *Project) ApplyTree(entries []TreeEntry) (TreeResult, error) {
	var res TreeResult
	if len(entries) > MaxNodes {
		return res, fmt.Errorf("project: tree of %d entries, over the limit of %d", len(entries), MaxNodes)
	}
	accepted, rejected := checkTree(entries)
	res.Rejected = rejected

	p.mu.Lock()
	defer p.mu.Unlock()
	before := p.snapshotOwned()
	next := map[string]string{} // owned file path -> ref
	for _, e := range accepted {
		if e.Type != NodeDir {
			next[e.Path] = e.ref()
		}
	}
	errs := p.moveRenamed(next)
	for _, e := range accepted {
		if err := p.apply(e); err != nil {
			res.Rejected = append(res.Rejected, Rejection{Path: e.Path, Err: err})
			delete(next, e.Path)
			continue
		}
		if e.Type == NodeBlob {
			res.Blobs = append(res.Blobs, e)
		}
	}
	for rel, recorded := range p.m.Files { // what left the tree and was never written
		if _, listed := next[rel]; !listed && recorded == "" && rel != MainFile {
			delete(p.m.Files, rel)
			delete(p.claimed, rel)
			delete(p.notified, rel)
		}
	}
	p.m.Tree = next
	errs = append(errs, p.syncOwned(before))
	select {
	case p.treeChanged <- struct{}{}: // the watcher has new files to look at
	default:
	}
	return res, errors.Join(errs...)
}

// apply makes the folder or the owned file of an accepted entry. The caller
// holds p.mu.
func (p *Project) apply(e TreeEntry) error {
	if e.Type == NodeDir {
		return p.mkdir(e.Path)
	}
	if _, owned := p.m.Files[e.Path]; !owned {
		if _, err := p.lstatPath(e.Path); err == nil {
			return fmt.Errorf("%w: a file is there already, not written by Inkwell", ErrTreeConflict)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := p.mkdir(path.Dir(e.Path)); err != nil {
		return err
	}
	if _, owned := p.m.Files[e.Path]; !owned {
		p.m.Files[e.Path] = ""
	}
	return nil
}

// moveRenamed moves on disk the owned files that the new tree lists at another
// path: those whose content ref appears once in the old tree and once in the
// new one, at different paths, and that are safe to move: a regular file with
// one name, written by Inkwell and unchanged since (its hash is the recorded
// one), going to a path that is free and not owned. Folders are never moved.
// The caller holds p.mu.
func (p *Project) moveRenamed(next map[string]string) []error {
	oldByRef, newByRef := map[string][]string{}, map[string][]string{}
	for pth, ref := range p.m.Tree {
		oldByRef[ref] = append(oldByRef[ref], pth)
	}
	for pth, ref := range next {
		newByRef[ref] = append(newByRef[ref], pth)
	}
	var errs []error
	for _, ref := range slices.Sorted(maps.Keys(oldByRef)) {
		from, to := oldByRef[ref], newByRef[ref]
		if len(from) != 1 || len(to) != 1 || from[0] == to[0] {
			continue
		}
		src, dst := from[0], to[0]
		if _, stillListed := next[src]; stillListed {
			continue
		}
		if _, taken := p.m.Files[dst]; taken || p.m.Files[src] == "" {
			continue // dst is owned, or Inkwell never wrote src
		}
		if !p.unchanged(src) {
			continue
		}
		if _, err := p.lstatPath(dst); err == nil || !errors.Is(err, fs.ErrNotExist) {
			continue // something is there already, or the path is not safe: leave both
		}
		if err := p.mkdir(path.Dir(dst)); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := p.renameNoReplace(src, dst); err != nil {
			if !errors.Is(err, fs.ErrExist) {
				errs = append(errs, fmt.Errorf("project: move %q: %w", src, err))
			}
			continue
		}
		p.m.Files[dst] = p.m.Files[src]
		p.claimed[dst] = p.claimed[src]
		delete(p.m.Files, src)
		delete(p.claimed, src)
		delete(p.notified, src)
	}
	return errs
}

// unchanged reports whether rel is a regular file with one name whose content
// is what Inkwell last wrote there. The caller holds p.mu.
func (p *Project) unchanged(rel string) bool {
	recorded := p.m.Files[rel]
	if recorded == "" {
		return false
	}
	sum, exists, err := p.diskHash(rel)
	return err == nil && exists && sum == recorded
}

// ErrBlobMismatch is returned by [Project.WriteBlob] when the content it was
// given does not hash to the blob's hash.
var ErrBlobMismatch = errors.New("project: content does not match the blob hash")

// WriteBlob writes the visible copy of a blob: the file rel of the project
// folder, which the current tree must list as the blob with this hash. The
// copy is streamed from src with [store.WriteAtomicFrom] and checked against
// hash on the way, so a 100 MB file is never held in memory and a wrong
// content never appears in the folder. It is never linked to the blob store, so
// that an edit made outside Inkwell cannot damage the store.
//
// A file that is already there with this content is left alone. One that was
// changed outside Inkwell since the last write is not overwritten
// ([ErrExternalEdit]) until [Project.Acknowledge]. It returns [ErrBlobMoved] if
// the tree no longer lists the blob there, [ErrNotOwned] or an
// [ErrInvalidName] for a path that was never listed, [ErrBlobMismatch] for a
// wrong content, and [ErrLink] or [ErrHardLink] for a path that is not a plain
// file.
func (p *Project) WriteBlob(rel string, hash [32]byte, src io.Reader) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	recorded, err := p.owned(rel)
	if err != nil {
		return err
	}
	if p.m.Tree[rel] != (TreeEntry{Type: NodeBlob, Hash: hash}).ref() {
		return fmt.Errorf("%w: %q", ErrBlobMoved, rel)
	}
	current, onDisk, exists, err := p.hashDisk(rel)
	switch {
	case err != nil:
		return err
	case exists && onDisk == hash:
		// The file is that version already: from here on it follows the tree.
		if current != recorded || p.claimed[rel] {
			delete(p.claimed, rel)
			return p.record(rel, current)
		}
		return nil
	case exists && (current != recorded || p.claimed[rel]):
		// Changed outside Inkwell, or brought in by the user and not the version
		// the tree lists: the user decides, a peer's file never replaces it.
		return fmt.Errorf("%w: %q", ErrExternalEdit, rel)
	}
	if err := p.mkdir(path.Dir(rel)); err != nil {
		return err
	}
	b3, sha := blake3.New(32, nil), sha256.New()
	check := func() error {
		var got [32]byte
		b3.Sum(got[:0])
		if got != hash {
			return ErrBlobMismatch
		}
		return nil
	}
	if err := store.WriteAtomicFrom(p.root, rel, io.TeeReader(src, io.MultiWriter(b3, sha)), 0o600, check); err != nil {
		return fmt.Errorf("project: write %s: %w", rel, err)
	}
	return p.record(rel, hex.EncodeToString(sha.Sum(nil)))
}

// Claim records the file rel as written by Inkwell, with the hex SHA-256 sum
// that the caller has just computed from reading it: the user asked for this
// file to join the project (see [Project.OpenFile]). A tree that lists the path
// afterwards finds it owned, where a path the user already had is otherwise
// refused. It fails if the file is not a plain file with that content, so that
// a file changed since the read is not taken over.
func (p *Project) Claim(rel, sum string) error {
	if err := ValidatePath(rel); err != nil {
		return fmt.Errorf("%w: %w", ErrNotOwned, err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	current, exists, err := p.diskHash(rel)
	switch {
	case err != nil:
		return err
	case !exists || current != sum:
		return fmt.Errorf("project: %q changed while it was read", rel)
	}
	p.claimed[rel] = true
	return p.record(rel, sum)
}

// BlobPaths returns the paths where the current tree lists the blob hash, in
// order.
func (p *Project) BlobPaths(hash [32]byte) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	ref := (TreeEntry{Type: NodeBlob, Hash: hash}).ref()
	var out []string
	for pth, r := range p.m.Tree {
		if r == ref {
			out = append(out, pth)
		}
	}
	slices.Sort(out)
	return out
}

// OpenFile opens the file rel of the project folder for reading, after
// checking the path with [ValidatePath]. The file must be a regular file with
// one name, reached through no symbolic link; the caller closes it. It is how
// a file added from outside is read to be stored as a blob.
func (p *Project) OpenFile(rel string) (*os.File, fs.FileInfo, error) {
	if err := ValidatePath(rel); err != nil {
		return nil, nil, err
	}
	if exists, err := p.regular(rel); err != nil {
		return nil, nil, err
	} else if !exists {
		return nil, nil, fmt.Errorf("project: %q: %w", rel, fs.ErrNotExist)
	}
	f, err := p.root.Open(rel)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || hardLinks(info) > 1 { // swapped since the check
		_ = f.Close()
		return nil, nil, fmt.Errorf("%w: %q", ErrNotRegular, rel)
	}
	return f, info, nil
}

// diskHash returns the hex SHA-256 of rel on disk without reading it into
// memory, whatever its size; exists is false (and the hash empty) when the file
// is absent. The path must be a regular file with one name, through no
// symbolic link. The caller holds p.mu.
func (p *Project) diskHash(rel string) (hash string, exists bool, err error) {
	hash, _, exists, err = p.hashDisk(rel)
	return hash, exists, err
}

// hashDisk is [Project.diskHash] that also returns the BLAKE3 hash of the file,
// the name of a blob, from the same pass.
func (p *Project) hashDisk(rel string) (sum string, b3 [32]byte, exists bool, err error) {
	if exists, err = p.regular(rel); err != nil || !exists {
		return "", b3, false, err
	}
	f, err := p.root.Open(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return "", b3, false, nil
	}
	if err != nil {
		return "", b3, false, fmt.Errorf("project: read %s: %w", rel, err)
	}
	defer func() { _ = f.Close() }() // read-only handle
	sha, hasher := sha256.New(), blake3.New(32, nil)
	if _, err := io.Copy(io.MultiWriter(sha, hasher), f); err != nil {
		return "", b3, false, fmt.Errorf("project: read %s: %w", rel, err)
	}
	hasher.Sum(b3[:0])
	return hex.EncodeToString(sha.Sum(nil)), b3, true, nil
}
