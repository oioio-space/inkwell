package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/oioio-space/inkwell/internal/store"
)

// ownedDir is the folder of .inkwell/ that holds the record of every file
// Inkwell owns, in a store of its own. One record per file, each written
// atomically, so that writing a file costs one small write however large the
// project is; the project.json it replaced was rewritten whole every time.
const ownedDir = "owned"

// ownedPrefix is the first component of the key of every record.
const ownedPrefix = "f"

// ownedRec is the record of one owned file.
type ownedRec struct {
	// Path is the file, relative to the project folder.
	Path string `json:"path"`
	// Hash is the hex SHA-256 of what Inkwell last wrote or was told to take
	// as seen; empty if it wrote nothing yet.
	Hash string `json:"hash,omitempty"`
	// Ref is what identifies the content the last tree gave the path (see
	// [TreeEntry]), to recognize a file that moved; empty if the tree no
	// longer lists it.
	Ref string `json:"ref,omitempty"`
	// Claimed is true when the user brought the file in ([Project.Claim]) and
	// no blob of the tree has been written there since: Inkwell does not
	// replace it with a version a peer announces.
	Claimed bool `json:"claimed,omitempty"`
}

// ownedKey returns the key of the record of rel: the path is hashed, as a path
// is not a valid key component.
func ownedKey(rel string) []string {
	sum := sha256.Sum256([]byte(rel))
	return []string{ownedPrefix, hex.EncodeToString(sum[:])}
}

// openOwned opens the store of the records under .inkwell/owned.
func (p *Project) openOwned() error {
	if err := p.inkwell.MkdirAll(ownedDir, 0o700); err != nil {
		return fmt.Errorf("project: create %s: %w", ownedDir, err)
	}
	root, err := p.inkwell.OpenRoot(ownedDir)
	if err != nil {
		return fmt.Errorf("project: open %s: %w", ownedDir, err)
	}
	kv, err := store.Open(root)
	if err != nil {
		_ = root.Close()
		return err
	}
	p.ownedRoot, p.records = root, kv
	return nil
}

// loadOwned reads every record into memory. A record that cannot be read is
// left out: its file is then not owned, which is the safe side.
func (p *Project) loadOwned() error {
	entries, err := p.records.LoadRange([]string{ownedPrefix})
	if err != nil && !errors.Is(err, store.ErrCorrupt) {
		return fmt.Errorf("project: read the owned files: %w", err)
	}
	for _, e := range entries {
		var r ownedRec
		if json.Unmarshal(e.Data, &r) != nil || ValidatePath(r.Path) != nil {
			continue
		}
		p.m.Files[r.Path] = r.Hash
		if r.Ref != "" {
			p.m.Tree[r.Path] = r.Ref
		}
		if r.Claimed {
			p.claimed[r.Path] = true
		}
	}
	return nil
}

// saveOwned writes the record of rel from the state in memory. The caller holds
// p.mu or owns p exclusively.
func (p *Project) saveOwned(rel string) error {
	data, err := json.Marshal(ownedRec{Path: rel, Hash: p.m.Files[rel], Ref: p.m.Tree[rel], Claimed: p.claimed[rel]})
	if err != nil {
		return fmt.Errorf("project: encode a record: %w", err)
	}
	if err := p.records.Save(ownedKey(rel), data); err != nil {
		return fmt.Errorf("project: save a record: %w", err)
	}
	return nil
}

// forgetOwned deletes the record of rel.
func (p *Project) forgetOwned(rel string) error {
	if err := p.records.Remove(ownedKey(rel)); err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("project: remove a record: %w", err)
	}
	return nil
}

// ownedState is a copy of what the records say, to find what changed.
type ownedState struct {
	files, tree map[string]string
	claimed     map[string]bool
}

func (p *Project) snapshotOwned() ownedState {
	return ownedState{files: maps.Clone(p.m.Files), tree: maps.Clone(p.m.Tree), claimed: maps.Clone(p.claimed)}
}

// syncOwned writes the records that differ from the state before, and deletes
// those of the files that are no longer owned. The caller holds p.mu.
func (p *Project) syncOwned(before ownedState) error {
	var errs []error
	for rel, hash := range p.m.Files {
		old, was := before.files[rel]
		if was && old == hash && before.tree[rel] == p.m.Tree[rel] && before.claimed[rel] == p.claimed[rel] {
			continue
		}
		errs = append(errs, p.saveOwned(rel))
	}
	for rel := range before.files {
		if _, still := p.m.Files[rel]; !still {
			errs = append(errs, p.forgetOwned(rel))
		}
	}
	return errors.Join(errs...)
}

// migrateLegacy moves the owned files that project.json used to hold into
// their own records, and rewrites project.json without them.
func (p *Project) migrateLegacy(files, tree map[string]string) error {
	if len(files) == 0 && len(tree) == 0 {
		return nil
	}
	for rel, hash := range files {
		if ValidatePath(rel) != nil {
			continue
		}
		if _, known := p.m.Files[rel]; known {
			continue // a record is newer than the manifest
		}
		p.m.Files[rel] = hash
		if ref, ok := tree[rel]; ok {
			p.m.Tree[rel] = ref
		}
		if err := p.saveOwned(rel); err != nil {
			return err
		}
	}
	return p.saveLocked()
}
