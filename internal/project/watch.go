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

	state := &watcher{
		p: p, w: w, onExternal: onExternal,
		root:  p.root.Name(),
		dirty: map[string]bool{}, held: map[string]bool{},
		dirs: map[string]bool{}, known: map[string]bool{},
		timer: time.NewTimer(watchDebounce),
	}
	state.timer.Stop()
	defer state.timer.Stop()
	if err := state.refresh(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.treeChanged:
			if err := state.refresh(); err != nil {
				return err
			}
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			state.onEvent(ev)
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("project: watch %s: %w", p.dir, err)
		case <-state.timer.C:
			state.flush()
		}
	}
}

// watcher keeps directory subscriptions and pending edits on the Watch goroutine.
type watcher struct {
	p                        *Project
	w                        *fsnotify.Watcher
	onExternal               func(string)
	root                     string
	dirty, held, dirs, known map[string]bool
	timer                    *time.Timer
}

// refresh picks up newly owned files, retrying folders that were previously absent.
func (s *watcher) refresh() error {
	s.p.mu.Lock()
	owned := slices.Sorted(maps.Keys(s.p.m.Files))
	s.p.mu.Unlock()
	var fresh []string
	newDirs := map[string]bool{}
	if err := s.watchDirs(owned, newDirs); err != nil {
		return err
	}
	for _, rel := range owned {
		dir := filepath.Dir(filepath.Join(s.root, filepath.FromSlash(rel)))
		if (!s.known[rel] && s.dirs[dir]) || newDirs[dir] {
			fresh = append(fresh, rel)
		}
		s.known[rel] = s.known[rel] || s.dirs[dir]
	}
	for _, rel := range fresh {
		if s.p.emptied(rel) {
			s.dirty[rel], s.held[rel] = true, true
			s.timer.Reset(watchDebounce)
			continue
		}
		s.p.check(rel, s.onExternal)
	}
	return nil
}

// watchDirs subscribes before refresh checks content, so edits cannot fall between them.
func (s *watcher) watchDirs(owned []string, newDirs map[string]bool) error {
	for _, rel := range owned {
		dir := filepath.Dir(filepath.Join(s.root, filepath.FromSlash(rel)))
		if !s.dirs[dir] {
			switch err := s.w.Add(dir); {
			case err == nil:
				s.dirs[dir], newDirs[dir] = true, true
			case !errors.Is(err, fs.ErrNotExist):
				return fmt.Errorf("project: watch %s: %w", s.p.dir, err)
			}
		}
	}
	return nil
}

func (s *watcher) onEvent(ev fsnotify.Event) {
	if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Rename) && !ev.Has(fsnotify.Remove) {
		return
	}
	rel, err := filepath.Rel(s.root, ev.Name)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	s.p.mu.Lock()
	_, isOwned := s.p.m.Files[rel]
	s.p.mu.Unlock()
	if isOwned {
		s.dirty[rel] = true
		s.timer.Reset(watchDebounce)
	}
}

// flush holds an empty edit for one extra debounce: writers may truncate before writing.
func (s *watcher) flush() {
	for _, rel := range slices.Sorted(maps.Keys(s.dirty)) {
		if s.p.emptied(rel) && !s.held[rel] {
			s.held[rel] = true
			continue
		}
		delete(s.held, rel)
		delete(s.dirty, rel)
		s.p.check(rel, s.onExternal)
	}
	if len(s.dirty) > 0 {
		s.timer.Reset(watchDebounce)
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

// goneMark stands in p.notified for "reported as deleted": no content has this
// hash form.
const goneMark = "gone"

// check calls onExternal(rel) if the file on disk differs from what Inkwell
// last wrote and from what was already reported, or if it is gone: a file
// Inkwell wrote that is no longer there is reported once (the reader finds it
// absent). A file Inkwell never wrote is not missed.
func (p *Project) check(rel string, onExternal func(rel string)) {
	p.mu.Lock()
	current, exists, err := p.diskHash(rel)
	recorded := p.m.Files[rel]
	var report bool
	switch {
	case err != nil:
	case exists:
		report = current != recorded && current != p.notified[rel]
		if !report && p.notified[rel] == goneMark {
			delete(p.notified, rel) // back: a later deletion is news again
		}
	default:
		report = recorded != "" && p.notified[rel] != goneMark
		current = goneMark
	}
	if report {
		p.notified[rel] = current
	}
	p.mu.Unlock()
	if report {
		onExternal(rel)
	}
}
