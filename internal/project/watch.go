package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchDebounce groups the bursts of events one save produces, including a
// formatter that rewrites the file many times in a row.
const watchDebounce = 200 * time.Millisecond

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

	dirty := map[string]bool{}
	// A file found empty is looked at once more before it is reported: a writer
	// that truncates then writes (os.WriteFile) shows an empty file in between,
	// and a slow writer can leave it empty for longer than watchDebounce.
	held := map[string]bool{}
	timer := time.NewTimer(watchDebounce)
	timer.Stop()
	defer timer.Stop()

	// Watch the directories, not the files: editors replace a file by renaming
	// another over it, which ends a watch on the old inode.
	root := p.root.Name()
	dirs := map[string]bool{}  // watched
	known := map[string]bool{} // owned files looked at
	// refresh watches the folders of the owned files it did not know, then looks
	// at them, in that order so that no edit falls between the two. The owned
	// files grow when a tree is applied (see ApplyTree). A folder that is gone
	// is left for the next refresh.
	refresh := func() error {
		p.mu.Lock()
		owned := slices.Sorted(maps.Keys(p.m.Files))
		p.mu.Unlock()
		var fresh []string
		newDirs := map[string]bool{}
		for _, rel := range owned {
			dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
			if !dirs[dir] {
				switch err := w.Add(dir); {
				case err == nil:
					dirs[dir], newDirs[dir] = true, true
				case !errors.Is(err, fs.ErrNotExist):
					return fmt.Errorf("project: watch %s: %w", p.dir, err)
				}
			}
		}
		for _, rel := range owned {
			dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
			if (!known[rel] && dirs[dir]) || newDirs[dir] {
				fresh = append(fresh, rel)
			}
			known[rel] = known[rel] || dirs[dir]
		}
		for _, rel := range fresh {
			if p.emptied(rel) {
				dirty[rel], held[rel] = true, true
				timer.Reset(watchDebounce)
				continue
			}
			p.check(rel, onExternal)
		}
		return nil
	}
	if err := refresh(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.treeChanged:
			if err := refresh(); err != nil {
				return err
			}
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
				if p.emptied(rel) && !held[rel] {
					held[rel] = true
					continue
				}
				delete(held, rel)
				delete(dirty, rel)
				p.check(rel, onExternal)
			}
			if len(dirty) > 0 {
				timer.Reset(watchDebounce)
			}
		}
	}
}

// emptied reports whether rel exists, is empty, and would be reported as an
// external edit.
func (p *Project) emptied(rel string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	current, exists, err := p.diskHash(rel)
	return err == nil && exists && current == emptyHash &&
		current != p.m.Files[rel] && current != p.notified[rel]
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
