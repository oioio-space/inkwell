package blob

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/oioio-space/inkwell/internal/store"
)

// gcFile holds, for each blob that nothing references, since when.
const gcFile = "gc.json"

// maxGCFile bounds the state file we read back.
const maxGCFile = 16 << 20

// gcState is the content of gcFile: the Unix time, in seconds, at which a pass
// first saw each blob unreferenced.
type gcState struct {
	Unreferenced map[string]int64 `json:"unreferenced"`
}

// GCResult is what a pass of [Store.GC] did.
type GCResult struct {
	// Removed is the number of blobs and abandoned downloads deleted.
	Removed int
	// Freed is the number of bytes they held.
	Freed int64
}

// GC deletes the blobs that nothing has referenced for maxAge, 30 days in the
// ADR ([UnreferencedTTL]), and the downloads of such blobs that stopped as
// long ago. referenced says which hashes the project refers to now, in the
// index, the trash and the packages; a blob that is referenced again loses its
// mark, so a file restored from the trash after a week is kept.
//
// The first pass that sees a blob unreferenced only marks it, so the age is
// the time between passes, which survives a restart. Do not run a pass before
// the project has said what it references: everything would be marked.
func (s *Store) GC(referenced func(Hash) bool, maxAge time.Duration) (GCResult, error) {
	var res GCResult
	state := s.loadGC()
	now := s.opt.Now()
	hashes, err := s.List()
	if err != nil {
		return res, err
	}
	next := make(map[string]int64, len(state.Unreferenced))
	var errs []error
	for _, h := range hashes {
		if referenced(h) {
			continue
		}
		since, marked := state.Unreferenced[h.Hex()]
		if !marked {
			next[h.Hex()] = now.Unix()
			continue
		}
		if now.Sub(time.Unix(since, 0)) < maxAge {
			next[h.Hex()] = since
			continue
		}
		size, _ := s.Size(h)
		if err := s.Remove(h); err != nil {
			errs = append(errs, err)
			next[h.Hex()] = since
			continue
		}
		res.Removed++
		res.Freed += size
	}
	freed, removed, perr := s.gcPartials(referenced, maxAge, now)
	res.Freed += freed
	res.Removed += removed
	errs = append(errs, perr)
	if err := s.saveGC(gcState{Unreferenced: next}); err != nil {
		errs = append(errs, err)
	}
	return res, errors.Join(errs...)
}

// gcPartials deletes the downloads of unreferenced blobs that were last
// written maxAge ago or more.
func (s *Store) gcPartials(referenced func(Hash) bool, maxAge time.Duration, now time.Time) (freed int64, removed int, err error) {
	entries, err := fs.ReadDir(s.root.FS(), partialDir)
	if err != nil {
		return 0, 0, fmt.Errorf("blob: list %s: %w", partialDir, err)
	}
	var errs []error
	for _, e := range entries {
		h, ok := hashFromName(e.Name())
		if !ok || !e.Type().IsRegular() || referenced(h) {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < maxAge {
			continue
		}
		name := partialDir + "/" + e.Name()
		if err := s.root.Remove(name); err != nil {
			errs = append(errs, fmt.Errorf("blob: remove %s: %w", name, err))
			continue
		}
		_ = s.root.Remove(name + ".map") // may be absent
		s.unhold(h.Hex())
		freed += info.Size()
		removed++
	}
	return freed, removed, errors.Join(errs...)
}

// loadGC reads the state file; a missing or damaged one is an empty state, at
// the price of restarting the 30 days.
func (s *Store) loadGC() gcState {
	state := gcState{Unreferenced: map[string]int64{}}
	raw, err := s.root.ReadFile(gcFile)
	if err != nil || len(raw) > maxGCFile {
		return state
	}
	var read gcState
	if json.Unmarshal(raw, &read) != nil {
		return state
	}
	for k, v := range read.Unreferenced {
		if isHashName(k) && !strings.ContainsAny(k, "/\\") {
			state.Unreferenced[k] = v
		}
	}
	return state
}

func (s *Store) saveGC(state gcState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("blob: encode %s: %w", gcFile, err)
	}
	if err := store.WriteAtomic(s.root, gcFile, raw, 0o600); err != nil {
		return fmt.Errorf("blob: %w", err)
	}
	return nil
}
