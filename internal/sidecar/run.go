package sidecar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"syscall"
	"time"
)

// DefaultMaxOutput is the number of bytes of each of stdout and stderr that
// [Run] keeps when [Spec.MaxOutput] is zero.
const DefaultMaxOutput = 4 << 20

// waitDelay bounds how long Run waits for the pipes of a child that exited
// but left a descendant holding them.
const waitDelay = 2 * time.Second

// Spec describes one launch of a verified sidecar.
type Spec struct {
	// Stdin is the child's standard input; nil means none.
	Stdin io.Reader
	// Path is the executable, as returned by [Store.Resolve].
	Path string
	// Dir is the working directory; empty means the caller's.
	Dir string
	// Args are the arguments after the program name.
	Args []string
	// Env is the child's complete environment. The caller's environment is
	// never inherited: a stray TYPST_* or proxy variable must not change a
	// compilation.
	Env []string
	// WatchDir is an empty, private output directory. Linux prefers a private
	// user/mount namespace with a size-limited tmpfs and verifies logical sizes
	// before copying outputs out. If user namespaces/mounts are unavailable,
	// Landlock and RLIMIT_FSIZE assign fixed per-file shares instead. The caller
	// must not mutate the directory concurrently. Other OSes reject bounded runs.
	// See docs/adr/0033-sidecar-directory-bound.md for sparse-file limitations.
	WatchDir string
	// WriteFiles lists unique simple output names to precreate in WatchDir.
	// File creation, removal, linking and renaming by the child are denied.
	WriteFiles []string
	// MaxDirBytes bounds exported logical bytes and private tmpfs allocation.
	// Zero disables the bound. Sparse logical sizes inside tmpfs are verified
	// after exit, not bounded in aggregate during execution.
	MaxDirBytes int64
	// MemoryLimit caps the child's address space, in bytes, on Linux; zero
	// means no limit. Other systems ignore it. It is in force from the child's
	// first instruction.
	MemoryLimit uint64
	// FileSizeLimit caps the size of any file the child writes, in bytes, on
	// Linux (see withLimits for the rounding); zero means no limit. A write past
	// it ends the child with SIGXFSZ.
	FileSizeLimit uint64
	// NoNetwork runs the child in a private network namespace, where not even
	// the loopback answers, when the system allows it (Linux with unprivileged
	// user namespaces). Otherwise it runs normally and [Output.Isolated] is false:
	// the caller must keep its other barriers. See [NetworkIsolation].
	NoNetwork bool
	// MaxOutput is how many bytes of each of stdout and stderr are kept; the
	// rest is read and dropped so the child never blocks. Zero means
	// [DefaultMaxOutput].
	MaxOutput int
}

// Output is what a finished child wrote and how it ended.
type Output struct {
	// Stdout and Stderr hold up to [Spec.MaxOutput] bytes each.
	Stdout, Stderr []byte
	// ExitCode is the exit status, or -1 when a signal ended the child.
	ExitCode int
	// Signal is the signal that ended the child, 0 if it exited by itself.
	Signal int
	// Truncated is true when stdout or stderr was longer than MaxOutput.
	Truncated bool
	// OutputLimit reports a directory backstop kill or a failed bounded run
	// with an output file at its kernel-enforced size cap.
	OutputLimit bool
	// Isolated is true when the child ran without network ([Spec.NoNetwork]).
	Isolated bool
}

// Run starts the program, waits for it and returns what it wrote. A non-zero
// exit status is reported in [Output.ExitCode], not as an error. When ctx ends
// first, the child's whole process group is killed and Run returns ctx.Err()
// with what was captured so far.
func Run(ctx context.Context, spec Spec) (Output, error) {
	return run(ctx, spec, newDirectoryMount)
}

func run(ctx context.Context, spec Spec, prepareMount func(Spec) (*directoryMount, error)) (Output, error) {
	if spec.Path == "" {
		return Output{}, errors.New("sidecar: run: no executable")
	}
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	maxOut := spec.MaxOutput
	if maxOut <= 0 {
		maxOut = DefaultMaxOutput
	}
	stdout, stderr := &capped{max: maxOut}, &capped{max: maxOut}

	originalSpec := spec
	mount, err := prepareMount(spec)
	if err != nil {
		return Output{}, err
	}
	var prefix []string
	if mount == nil {
		spec, prefix, err = directoryLimits(spec)
	} else {
		defer mount.close()
		spec.FileSizeLimit = 0 // exact byte cap is applied inside the mount wrapper
	}
	if err != nil {
		return Output{}, err
	}
	path, args, err := withLimits(spec)
	if err != nil {
		return Output{}, err
	}
	if len(prefix) > 0 {
		args = slices.Concat(prefix[1:], []string{path}, args)
		path = prefix[0]
	}
	cmd := exec.CommandContext(ctx, path, args...) // #nosec G204 -- Path comes from Store.Resolve, never from user input
	cmd.Dir = spec.Dir
	cmd.Env = append([]string{}, spec.Env...) // non-nil: an empty environment, not the caller's
	cmd.Stdin = spec.Stdin
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = waitDelay
	setProcAttr(cmd)
	isolated := spec.NoNetwork && NetworkIsolation()
	if isolated {
		isolate(cmd)
	}
	if mount != nil {
		if err := mount.configure(cmd); err != nil {
			return Output{}, err
		}
	}
	cmd.Cancel = func() error { return signalGroup(cmd.Process.Pid, syscall.SIGKILL) }

	// The parent-death signal follows the thread that forks the child.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := cmd.Start(); err != nil {
		return Output{}, fmt.Errorf("sidecar: start %s: %w", spec.Path, err)
	}
	if mount != nil {
		if err := mount.open(cmd.Process.Pid); err != nil {
			_ = signalGroup(cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
			return Output{}, err
		}
	}
	var overflow atomic.Bool
	stopWatch := watchDir(spec, cmd.Process.Pid, &overflow)
	waitErr := cmd.Wait()
	stopWatch()
	// Whatever the child left behind in its group goes with it.
	_ = signalGroup(cmd.Process.Pid, syscall.SIGKILL)

	out := Output{
		Stdout:    stdout.buf,
		Stderr:    stderr.buf,
		ExitCode:  cmd.ProcessState.ExitCode(),
		Truncated: stdout.dropped || stderr.dropped,
		Signal:    exitSignal(cmd.ProcessState),
		Isolated:  isolated,

		OutputLimit: overflow.Load(),
	}
	if mount != nil {
		out.OutputLimit = out.OutputLimit || mount.limitHit(out)
		if err := mount.export(); err != nil {
			out.OutputLimit = true
			return out, err
		}
	} else {
		out.OutputLimit = out.OutputLimit || directoryLimitHit(originalSpec, out)
		cleanupEmptyOutputs(originalSpec)
	}
	if err := ctx.Err(); err != nil && out.ExitCode != 0 {
		return out, err
	}
	if waitErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](waitErr); !exited {
			return out, fmt.Errorf("sidecar: run %s: %w", spec.Path, waitErr)
		}
	}
	return out, nil
}

// watchDir kills the group of pid when the files under spec.WatchDir exceed
// spec.MaxDirBytes; the returned function stops the watch and waits for it.
func watchDir(spec Spec, pid int, overflow *atomic.Bool) (stop func()) {
	if spec.WatchDir == "" || spec.MaxDirBytes <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if dirBytes(spec.WatchDir) > spec.MaxDirBytes {
					overflow.Store(true)
					_ = signalGroup(pid, syscall.SIGKILL)
					return
				}
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}

// dirBytes adds up the sizes of the regular files under dir.
func dirBytes(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// NetworkIsolation reports whether [Spec.NoNetwork] can be honoured on this
// machine, probing once.
func NetworkIsolation() bool {
	if isolationOverride != nil {
		return isolationOverride()
	}
	return probeIsolation()
}

// isolationOverride replaces the probe in tests.
var isolationOverride func() bool

// capped keeps the first max bytes written to it and counts the rest as
// dropped, always reporting a full write so the child never blocks.
type capped struct {
	buf     []byte
	max     int
	dropped bool
}

func (c *capped) Write(p []byte) (int, error) {
	room := c.max - len(c.buf)
	if room > 0 {
		c.buf = append(c.buf, p[:min(room, len(p))]...)
	}
	if len(p) > room {
		c.dropped = true
	}
	return len(p), nil
}
