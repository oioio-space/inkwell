package typst

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchDebounce groups the bursts of events one save produces.
const watchDebounce = 150 * time.Millisecond

// skippedDirs are not watched: Inkwell's own data, version control and
// dependency folders change without the document changing.
var skippedDirs = map[string]bool{".inkwell": true, ".git": true, "node_modules": true}

// Watch compiles main now, then again after each change in the project folder,
// and calls fn with every result until ctx ends; it then returns nil. A burst
// of changes gives one compilation, and a change during a compilation cancels
// it and starts a new one, so fn only sees results of the latest sources. fn
// is called from the watching goroutine: a slow fn delays the next
// compilation, never loses a change.
//
// It recompiles the whole document each time. The incremental preview through
// the language server (C2) will replace it for the live view; Watch stays the
// simple, always-correct way to follow a project.
func (e *Engine) Watch(ctx context.Context, p Project, main string, fn func(*Result, error), opts ...Option) error {
	root, err := realRoot(p.Root)
	if err != nil {
		return err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("typst: watch: %w", err)
	}
	defer func() { _ = w.Close() }()
	if err := addTree(w, root); err != nil {
		return fmt.Errorf("typst: watch %s: %w", p.Root, err)
	}

	changed := make(chan struct{}, 1)
	var failure error
	loopDone := make(chan struct{})
	go func() { // turns raw events into debounced "something changed" signals
		defer close(loopDone)
		timer := time.NewTimer(watchDebounce)
		timer.Stop()
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Has(fsnotify.Create) {
					if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
						_ = addTree(w, ev.Name) // a folder created later is watched too
					}
				}
				timer.Reset(watchDebounce)
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				failure = err
				return
			case <-timer.C:
				select {
				case changed <- struct{}{}:
				default:
				}
			}
		}
	}()
	defer func() { <-loopDone }()

	for {
		if err := e.compileOnce(ctx, p, main, changed, fn, opts); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-loopDone:
			return failureOr(failure)
		case <-changed:
		}
	}
}

func failureOr(err error) error {
	if err != nil {
		return fmt.Errorf("typst: watch: %w", err)
	}
	return nil
}

// compileOnce compiles and delivers the result, unless a change or the end of
// ctx cancels it first; in that case it returns nil and the caller loops.
func (e *Engine) compileOnce(ctx context.Context, p Project, main string, changed chan struct{}, fn func(*Result, error), opts []Option) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type outcome struct {
		res *Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := e.Compile(runCtx, p, main, opts...)
		done <- outcome{res, err}
	}()
	select {
	case out := <-done:
		if ctx.Err() != nil {
			return nil
		}
		fn(out.res, out.err)
		return nil
	case <-changed:
		// Newer sources: abandon this compilation and start over.
		cancel()
		<-done
		select { // the signal was consumed: compile again right away
		case changed <- struct{}{}:
		default:
		}
		return nil
	case <-ctx.Done():
		cancel()
		<-done
		return nil
	}
}

// addTree watches dir and every folder below it that is not skipped.
func addTree(w *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // vanished while walking
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != dir && skippedDirs[d.Name()] {
			return fs.SkipDir
		}
		return w.Add(path)
	})
}
