package project

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchDebounce groups the bursts of events one save produces.
const watchDebounce = 50 * time.Millisecond

// Watch reports the owned files that are edited outside Inkwell until ctx is
// done, then returns nil. onExternal(rel) is called from the watching
// goroutine, so it must not block for long, once per distinct new content: an
// edit that Inkwell wrote itself, or one already reported, is not reported.
//
// Edits made before Watch started (Inkwell was closed) are reported first.
// Only the owned files are looked at; the rest of the folder is not touched.
func (p *Project) Watch(ctx context.Context, onExternal func(rel string)) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("project: watch: %w", err)
	}
	defer func() { _ = w.Close() }()

	p.mu.Lock()
	owned := make([]string, 0, len(p.m.Files))
	for rel := range p.m.Files {
		owned = append(owned, rel)
	}
	p.mu.Unlock()
	slices.Sort(owned)

	// Watch the directories, not the files: editors replace a file by renaming
	// another over it, which ends a watch on the old inode.
	root := p.root.Name()
	dirs := map[string]bool{}
	for _, rel := range owned {
		dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
		if dirs[dir] {
			continue
		}
		dirs[dir] = true
		if err := w.Add(dir); err != nil {
			return fmt.Errorf("project: watch %s: %w", p.dir, err)
		}
	}
	for _, rel := range owned { // after Add, so no edit falls between the two
		p.check(rel, onExternal)
	}

	dirty := map[string]bool{}
	timer := time.NewTimer(watchDebounce)
	timer.Stop()
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Rename) {
				continue
			}
			rel, err := filepath.Rel(root, ev.Name)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			p.mu.Lock()
			_, isOwned := p.m.Files[rel]
			p.mu.Unlock()
			if isOwned {
				dirty[rel] = true
				timer.Reset(watchDebounce)
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("project: watch %s: %w", p.dir, err)
		case <-timer.C:
			for _, rel := range slices.Sorted(maps.Keys(dirty)) {
				p.check(rel, onExternal)
			}
			clear(dirty)
		}
	}
}

// check calls onExternal(rel) if the file on disk differs from what Inkwell
// last wrote and from what was already reported.
func (p *Project) check(rel string, onExternal func(rel string)) {
	p.mu.Lock()
	current, exists, err := p.diskHash(rel)
	report := err == nil && exists && current != p.m.Files[rel] && current != p.notified[rel]
	if report {
		p.notified[rel] = current
	}
	p.mu.Unlock()
	if report {
		onExternal(rel)
	}
}
