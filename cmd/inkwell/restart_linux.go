package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

// canRestart: this build restarts itself in place (residual R-8 of the PC1 plan).
const canRestart = true

// barrierTimeout bounds the ending of the children of the image before the execution;
// barrierGrace is what a child gets to end on request before it is killed. It is short because
// every restart of the desktop build pays it: the inner process of a web view sandbox is the init
// of a pid namespace, which takes no SIGTERM from outside (measured: it lives the whole grace,
// gates/PC1a/UX.md, « Fix of UX-PC1a-1 »), and the helpers have nothing left to save (N-15).
const (
	barrierTimeout = 5 * time.Second
	barrierGrace   = 100 * time.Millisecond
)

// restartImage executes the running image again in place ([sidecar.ExecSelf]: the running file
// itself, whatever became of its path) on the project folder dir (rule N-12),
// once the termination barrier holds (rule N-14). It returns only when it failed.
func restartImage(dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("restart: project folder: %w", err)
	}
	if err := barrier(barrierTimeout); err != nil {
		return err
	}
	argv, env := restartCommand(dir, os.Environ())
	return fmt.Errorf("restart: %w", sidecar.ExecSelf(argv, env))
}

// barrier is the termination barrier before the execution (rule N-14). The shutdown already ended
// what the session started; what is left are helpers it does not control (the web view's processes
// and their sandbox holders), whose work is over: the page flushed and left its context (N-15).
// The barrier ends them ([sidecar.EndChildren]) and fails, naming them, when one still runs after
// timeout; it then makes every descriptor beyond the standard three close-on-exec, so that the
// new image inherits neither a process nor a descriptor.
func barrier(timeout time.Duration) error {
	if err := sidecar.EndChildren(timeout, barrierGrace); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return fmt.Errorf("restart: listing descriptors: %w", err)
	}
	for _, e := range fds {
		if fd, err := strconv.Atoi(e.Name()); err == nil && fd > 2 {
			syscall.CloseOnExec(fd) // the listing's own descriptor is closed already: harmless
		}
	}
	return nil
}
