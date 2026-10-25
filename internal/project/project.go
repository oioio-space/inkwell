// Package project is a project as a folder the user can see: a Typst
// projection of the document next to a private .inkwell/ directory holding
// everything Inkwell needs.
//
//	<Title>/
//	  main.typ            the projection, owned by Inkwell
//	  .inkwell/           0o700
//	    project.json      id, title, root document, owned files and their hashes
//	    repo/ ...         automerge-repo chunks (package store)
//	    backups/ quarantine/
//
// Ownership rule: Inkwell owns the files listed in project.json and no other.
// It records the SHA-256 of what it last wrote; a file whose hash differs was
// changed from outside. Inkwell reports it ([Project.Watch]) and refuses to
// overwrite it ([ErrExternalEdit]) until the change is acknowledged
// ([Project.Acknowledge]). Files it does not own are never touched.
//
// All disk access goes through [os.Root], so nothing resolves outside the
// project folder. Document text is never logged.
package project

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/oioio-space/inkwell/internal/store"
)

const (
	// MainFile is the projection of the document, owned by every project.
	MainFile = "main.typ"
	// DefaultTitle names a project whose title is empty.
	DefaultTitle = "Sans titre"

	inkwellDir      = ".inkwell"
	manifestFile    = "project.json"
	maxManifest     = 1 << 20
	maxFolderRunes  = 80
	maxCollisions   = 1000
	unsafeFolderRun = `/\:*?"<>|`
)

var (
	// ErrNoProject is returned by [Latest] when the base holds no project.
	ErrNoProject = errors.New("project: no project")
	// ErrNotOwned is returned for a file that project.json does not list.
	ErrNotOwned = errors.New("project: file not owned by Inkwell")
	// ErrExternalEdit is returned when writing a file that was changed from
	// outside since Inkwell last wrote it, until [Project.Acknowledge].
	ErrExternalEdit = errors.New("project: file edited outside Inkwell")
)

// Meta identifies a project.
type Meta struct {
	// ID is random and never changes.
	ID string `json:"id"`
	// Title is shown to the user; the folder name derives from it.
	Title string `json:"title"`
	// RootURL is the automerge URL of the project's root document; empty
	// until the webview has created it.
	RootURL string `json:"rootUrl"`
}

// manifest is the content of project.json.
type manifest struct {
	// Files maps each owned file to the hex SHA-256 of what Inkwell last
	// wrote there; empty if it wrote nothing yet.
	Files map[string]string `json:"files"`
	Meta
}

// Project is an open project folder. It embeds the [store.KV] of its
// .inkwell/ directory. Create or open one with [Create] or [Open]; methods
// are safe for concurrent use.
type Project struct {
	*store.KV
	dir      string
	root     *os.Root // the project folder
	inkwell  *os.Root // its .inkwell directory
	mu       sync.Mutex
	m        manifest
	notified map[string]string // file -> hash already reported as external
}

// Create makes a new project folder named after title (made unique and safe
// for a file system) in base, with the private directory and project.json,
// and returns it open. An empty title becomes [DefaultTitle]. rootURL may be
// empty and set later with [Project.SetRootURL].
func Create(base *os.Root, title, rootURL string) (*Project, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = DefaultTitle
	}
	dir, err := makeFolder(base, folderName(title))
	if err != nil {
		return nil, err
	}
	root, err := base.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("project: open %s: %w", dir, err)
	}
	p := &Project{dir: dir, root: root, notified: map[string]string{}}
	p.m = manifest{
		Meta:  Meta{ID: rand.Text(), Title: title, RootURL: rootURL},
		Files: map[string]string{MainFile: ""},
	}
	if err := p.initInkwell(true); err != nil {
		_ = root.Close()
		return nil, err
	}
	return p, nil
}

// Open opens the existing project in folder dir of base.
func Open(base *os.Root, dir string) (*Project, error) {
	root, err := base.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("project: open %s: %w", dir, err)
	}
	p := &Project{dir: dir, root: root, notified: map[string]string{}}
	if err := p.initInkwell(false); err != nil {
		_ = root.Close()
		return nil, err
	}
	return p, nil
}

// initInkwell opens .inkwell/ (creating it, and project.json from p.m, if
// create) and loads or writes the manifest and the store.
func (p *Project) initInkwell(create bool) error {
	if create {
		if err := p.root.Mkdir(inkwellDir, 0o700); err != nil {
			return fmt.Errorf("project: create %s: %w", inkwellDir, err)
		}
	}
	ink, err := p.root.OpenRoot(inkwellDir)
	if err != nil {
		return fmt.Errorf("project: open %s: %w", inkwellDir, err)
	}
	p.inkwell = ink
	if create {
		err = p.saveLocked()
	} else {
		err = p.load()
	}
	if err == nil {
		p.KV, err = store.Open(ink)
	}
	if err != nil {
		_ = ink.Close()
		return err
	}
	return nil
}

func (p *Project) load() error {
	data, err := readLimited(p.inkwell, manifestFile, maxManifest)
	if err != nil {
		return fmt.Errorf("project: read %s: %w", manifestFile, err)
	}
	if err := json.Unmarshal(data, &p.m); err != nil {
		return fmt.Errorf("project: decode %s: %w", manifestFile, err)
	}
	if p.m.ID == "" || p.m.Files == nil {
		return fmt.Errorf("project: %s is incomplete", manifestFile)
	}
	return nil
}

// saveLocked writes project.json; the caller holds p.mu or owns p exclusively.
func (p *Project) saveLocked() error {
	data, err := json.MarshalIndent(p.m, "", "  ")
	if err != nil {
		return fmt.Errorf("project: encode %s: %w", manifestFile, err)
	}
	if err := store.WriteAtomic(p.inkwell, manifestFile, data, 0o600); err != nil {
		return fmt.Errorf("project: %w", err)
	}
	return nil
}

// Latest returns the folder name of the most recently changed project in
// base, or an error wrapping [ErrNoProject] if there is none. Folders that
// are not projects are ignored.
func Latest(base *os.Root) (string, error) {
	d, err := base.Open(".")
	if err != nil {
		return "", fmt.Errorf("project: list: %w", err)
	}
	entries, err := d.ReadDir(-1)
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("project: list: %w", err)
	}
	var best string
	var bestTime int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := base.Stat(e.Name() + "/" + inkwellDir + "/" + manifestFile)
		if err != nil {
			continue
		}
		if t := info.ModTime().UnixNano(); best == "" || t > bestTime {
			best, bestTime = e.Name(), t
		}
	}
	if best == "" {
		return "", ErrNoProject
	}
	return best, nil
}

// Dir returns the project's folder name in its base.
func (p *Project) Dir() string { return p.dir }

// Meta returns the project's identity.
func (p *Project) Meta() Meta {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m.Meta
}

// SetRootURL records the automerge URL of the project's root document.
func (p *Project) SetRootURL(url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	old := p.m.RootURL
	p.m.RootURL = url
	if err := p.saveLocked(); err != nil {
		p.m.RootURL = old
		return err
	}
	return nil
}

// OpenDir opens the directory name of .inkwell/, creating it (private to the
// user) when missing, for code that keeps its own files there, such as the
// member list in "trust". The caller closes the result before [Project.Close].
func (p *Project) OpenDir(name string) (*os.Root, error) {
	if err := p.inkwell.MkdirAll(name, 0o700); err != nil {
		return nil, fmt.Errorf("project: create %s: %w", name, err)
	}
	r, err := p.inkwell.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("project: open %s: %w", name, err)
	}
	return r, nil
}

// Close releases the project folder. Stop [Project.Watch] first.
func (p *Project) Close() error {
	return errors.Join(p.inkwell.Close(), p.root.Close())
}

// owned returns the recorded hash of rel, or ErrNotOwned.
func (p *Project) owned(rel string) (string, error) {
	h, ok := p.m.Files[rel]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrNotOwned, rel)
	}
	return h, nil
}

// ReadProjection returns the content of the owned file rel as it is on
// disk, which may include edits made outside Inkwell.
func (p *Project) ReadProjection(rel string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.owned(rel); err != nil {
		return nil, err
	}
	return readLimited(p.root, rel, store.MaxValue)
}

// WriteProjection atomically replaces the owned file rel with data and records
// its hash. It returns [ErrNotOwned] for a file project.json does not list
// and [ErrExternalEdit], leaving the file as is, if it was changed since the
// last write and the change was not acknowledged.
func (p *Project) WriteProjection(rel string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	recorded, err := p.owned(rel)
	if err != nil {
		return err
	}
	current, exists, err := p.diskHash(rel)
	if err != nil {
		return err
	}
	if exists && current != recorded {
		return fmt.Errorf("%w: %q", ErrExternalEdit, rel)
	}
	if err := store.WriteAtomic(p.root, rel, data, 0o600); err != nil {
		return fmt.Errorf("project: write %s: %w", rel, err)
	}
	return p.record(rel, hashOf(data))
}

// Acknowledge records the current content of the owned file rel as seen, so
// the next [Project.WriteProjection] may replace it. Use it once the user has
// imported or ignored an external edit.
func (p *Project) Acknowledge(rel string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.owned(rel); err != nil {
		return err
	}
	current, _, err := p.diskHash(rel)
	if err != nil {
		return err
	}
	return p.record(rel, current)
}

// record stores hash as the last seen content of rel and persists it. The
// caller holds p.mu. A failed save is returned: the file is already
// written, so the next start reports it as an external edit, which is safe.
func (p *Project) record(rel, hash string) error {
	p.m.Files[rel] = hash
	delete(p.notified, rel)
	return p.saveLocked()
}

// diskHash returns the hex SHA-256 of rel on disk; exists is false (and the
// hash empty) when the file is absent. The caller holds p.mu.
func (p *Project) diskHash(rel string) (hash string, exists bool, err error) {
	data, err := readLimited(p.root, rel, store.MaxValue)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("project: read %s: %w", rel, err)
	}
	return hashOf(data), true, nil
}

func hashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readLimited reads the file name of r, refusing one larger than limit bytes.
func readLimited(r *os.Root, name string, limit int64) ([]byte, error) {
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only handle
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is %d bytes, over the limit of %d", name, info.Size(), limit)
	}
	return io.ReadAll(io.LimitReader(f, limit+1))
}

// makeFolder creates the directory name in base, or "name (2)", "name (3)"…
// if it exists, and returns the name it created.
func makeFolder(base *os.Root, name string) (string, error) {
	for n := 1; n <= maxCollisions; n++ {
		candidate := name
		if n > 1 {
			candidate = fmt.Sprintf("%s (%d)", name, n)
		}
		err := base.Mkdir(candidate, 0o700)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("project: create folder %q: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("project: no free folder name for %q", name)
}

// folderName turns a title into a name that is safe on every file system:
// path separators, reserved and control characters become "-", leading and
// trailing dots and spaces go, and the length is bounded.
func folderName(title string) string {
	var b strings.Builder
	for _, r := range title {
		if unicode.IsControl(r) || strings.ContainsRune(unsafeFolderRun, r) {
			r = '-'
		}
		b.WriteRune(r)
	}
	name := strings.Trim(b.String(), ". ")
	if runes := []rune(name); len(runes) > maxFolderRunes {
		name = strings.TrimRight(string(runes[:maxFolderRunes]), ". ")
	}
	if name == "" {
		return DefaultTitle
	}
	return name
}
