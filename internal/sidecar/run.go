package sidecar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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
	disableScope      bool
	scope             *SandboxScope
	bootstrapExpected string
	// DeathTest labels opt-in test evidence; empty disables manifest writes.
	DeathTest string
	// ResidentLimit bounds aggregate charged memory; zero uses MemoryLimit.
	ResidentLimit uint64
	// isolationProbe replaces the platform probe for this run in tests.
	isolationProbe func() (bool, error)
	// ReadPaths lists verified package and font directories needed by the workload.
	ReadPaths []string
	// Stdin is the child's standard input; nil means none.
	Stdin io.Reader
	// Stdout receives live output, in addition to the bounded retained copy.
	// A protocol consumer must bound and validate each message it accepts.
	Stdout io.Writer
	// Path is the executable, as returned by [Store.Resolve].
	Path string
	// Dir is the mandatory project directory; home and its ancestors are refused.
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
	// Linux (see withLimits for the rounding); zero means no limit. The write
	// that reaches it is shortened and the next one fails with EFBIG: the
	// target starts with SIGXFSZ ignored (a target that restores the default
	// action dies of it instead; the cap holds either way).
	FileSizeLimit uint64
	// NoNetwork adds a private network namespace when supported. Every Linux
	// invocation independently requires seccomp (no socket creation or external
	// signals) and Landlock, including when namespaces are unavailable.
	// Output.Isolated reports the additional network namespace only.
	NoNetwork bool
	// MaxOutput is how many bytes of each of stdout and stderr are kept; the
	// rest is read and dropped so the child never blocks. Zero means
	// [DefaultMaxOutput].
	MaxOutput int
}

// MemoryCause records evidence, never an inference from SIGKILL alone.
type MemoryCause string

// Memory causes describe the observed memory limit evidence.
const (
	// MemoryNone means no memory limit evidence was observed.
	MemoryNone       MemoryCause = "none"
	MemoryLocalLimit MemoryCause = "local-limit"
	MemoryVictim     MemoryCause = "victim"
	MemoryUnknown    MemoryCause = "unknown"
)

// Output is what a finished child wrote and how it ended.
type Output struct {
	bootstrapRefused bool
	Death            *SandboxDeath
	Containment      string
	MemoryCause      MemoryCause
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
	// CoreDump says whether the kernel started a core dump, and so a crash
	// helper such as systemd-coredump, for the program that ended (the target
	// behind the sandbox's namespace init included). It is a test and
	// diagnostics fact: no decision depends on it.
	CoreDump CoreDump
}

// CoreDump is what the kernel did about a core dump of a finished program.
type CoreDump uint8

const (
	// CoreDumpUnknown means the system cannot tell (Linux before 6.16, other systems).
	CoreDumpUnknown CoreDump = iota
	// CoreDumpNone means no core dump was started.
	CoreDumpNone
	// CoreDumpStarted means the kernel started one (PIDFD_COREDUMPED).
	CoreDumpStarted
)

// Run starts the program, waits for it and returns what it wrote. A non-zero
// exit status is reported in [Output.ExitCode], not as an error. When ctx ends
// first, the child's whole process group is killed and Run returns ctx.Err()
// with what was captured so far.
func Run(ctx context.Context, spec Spec) (Output, error) {
	if err := confinementAvailable(); err != nil {
		return Output{}, err
	}
	return run(ctx, spec, newDirectoryMount)
}

func run(ctx context.Context, spec Spec, prepareMount func(Spec) (*directoryMount, error)) (Output, error) {
	if err := validateLaunch(ctx, &spec); err != nil {
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
		spec.FileSizeLimit, spec.MemoryLimit = 0, 0 // applied by the mount launcher
	}
	if err != nil {
		return Output{}, err
	}
	path, args, err := launchCommand(spec, mount != nil, prefix)
	if err != nil {
		return Output{}, err
	}
	cmd := exec.CommandContext(ctx, path, args...) // #nosec G204 -- Path comes from Store.Resolve, never from user input
	cmd.Dir = spec.Dir
	cmd.Env = append([]string{}, spec.Env...) // non-nil: an empty environment, not the caller's
	cmd.Stdin = spec.Stdin
	cmd.Stdout, cmd.Stderr = streamOutput(stdout, spec.Stdout), stderr
	cmd.WaitDelay = waitDelay
	setProcAttr(cmd)
	isolated, err := configureRunCommand(cmd, spec, mount)
	if err != nil {
		return Output{}, err
	}
	manager := &appSandbox
	if originalSpec.scope != nil {
		manager = originalSpec.scope
	}
	scope := manager.prepare(ctx, originalSpec, cmd)
	if scope != nil {
		defer scope.finish(0)
	}
	finishReport, closeReport, err := signalReport(cmd)
	if err != nil {
		return Output{}, err
	}
	defer closeReport()
	cmd.Cancel = func() error { return signalGroup(cmd.Process.Pid, syscall.SIGKILL) }

	// The parent-death signal follows the thread that forks the child.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	start := cmd.Start
	if scope != nil {
		start = func() error { return scope.start(cmd) }
	}
	if err := start(); err != nil {
		if scope != nil && scope.placementError(err) {
			scope.placementFailed()
			originalSpec.disableScope = true
			return run(ctx, originalSpec, prepareMount)
		}
		return Output{}, fmt.Errorf("sidecar: start %s: %w", spec.Path, err)
	}
	if scope != nil {
		scope.afterStart()
	}
	if err := openRunMount(mount, cmd); err != nil {
		return Output{}, err
	}
	var overflow atomic.Bool
	stopWatch := watchDir(spec, cmd.Process.Pid, &overflow)
	death := captureDeath(cmd)
	waitErr := cmd.Wait()
	stopWatch()
	memoryCause := MemoryUnknown
	if scope != nil {
		memoryCause = scope.finish(exitSignal(cmd.ProcessState))
	}
	// Whatever the child left behind in its group goes with it.
	_ = signalGroup(cmd.Process.Pid, syscall.SIGKILL)

	out := Output{
		Death:     death,
		Stdout:    stdout.buf,
		Stderr:    stderr.buf,
		ExitCode:  cmd.ProcessState.ExitCode(),
		Truncated: stdout.dropped || stderr.dropped,
		Signal:    exitSignal(cmd.ProcessState),
		Isolated:  isolated,

		OutputLimit: overflow.Load(),
	}
	out.Containment, out.MemoryCause = "rlimit", memoryCause
	if scope != nil {
		out.Containment = "cgroup"
	}
	finishReport(&out)
	if err := recordRunManifest(originalSpec.DeathTest, out); err != nil {
		return out, err
	}
	out, err = finishRunOutputs(out, mount, originalSpec)
	if err != nil {
		return out, err
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

func openRunMount(mount *directoryMount, cmd *exec.Cmd) error {
	if mount != nil {
		if err := mount.open(cmd.Process.Pid); err != nil {
			_ = signalGroup(cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
			return err
		}
	}
	return nil
}

func configureRunCommand(cmd *exec.Cmd, spec Spec, mount *directoryMount) (bool, error) {
	if err := configureConfinement(cmd); err != nil {
		return false, err
	}
	isolated, err := spec.isolated()
	if err != nil {
		return false, err
	}
	if isolated {
		isolate(cmd)
	}
	if mount != nil {
		if err := mount.configure(cmd); err != nil {
			return false, err
		}
	}
	return isolated, nil
}

func finishRunOutputs(out Output, mount *directoryMount, originalSpec Spec) (Output, error) {
	if out.bootstrapRefused {
		return out, errors.New("sidecar: bootstrap cgroup membership refused")
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

// isolated reports whether this launch runs in a private network namespace.
func (s Spec) isolated() (bool, error) {
	if !s.NoNetwork {
		return false, nil
	}
	return s.networkIsolation()
}

func (s Spec) networkIsolation() (bool, error) {
	if s.isolationProbe != nil {
		return s.isolationProbe()
	}
	return NetworkIsolation()
}

// NetworkIsolation reports whether [Spec.NoNetwork] can be honoured on this
// machine. The answer is probed once it is conclusive; until then it returns
// an error wrapping [ErrProbeInconclusive], and the launch must be refused.
func NetworkIsolation() (bool, error) {
	return probeIsolation()
}

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

// launchCommand selects exactly one confinement wrapper.
func launchCommand(spec Spec, mounted bool, prefix []string) (string, []string, error) {
	path, args := spec.Path, spec.Args
	var err error
	if !mounted && len(prefix) == 0 {
		path, args, err = withLimits(spec)
	}
	if len(prefix) > 0 {
		args = slices.Concat(prefix[1:], []string{path}, args)
		path = prefix[0]
	}
	return path, args, err
}

func validateSpecDirectory(spec *Spec) error {
	dir, err := workingDirectory(spec.Dir)
	if err == nil {
		spec.Dir = dir
	}
	return err
}

func validateLaunch(ctx context.Context, spec *Spec) error {
	if spec.Path == "" {
		return errors.New("sidecar: run: no executable")
	}
	if _, err := os.Stat(spec.Path); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateSpecDirectory(spec)
}

// streamOutput preserves the bounded transcript while feeding a live protocol reader.
func streamOutput(retained, live io.Writer) io.Writer {
	if live == nil {
		return retained
	}
	return io.MultiWriter(retained, live)
}
