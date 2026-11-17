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
	"strings"
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
	// tier forces the launch's tier in tests (tierNone: decided by the probe).
	tier tier
	// armDelay delays the launcher's parent-death arming in tests (plan S-4).
	armDelay time.Duration
	// landlockABI forces a supported ruleset ABI in tests (zero uses the kernel ABI).
	landlockABI int
	// slotsReady says the output slots exist: the placement fallback reuses
	// them instead of creating them again (plan T-6).
	slotsReady bool
	// handshake seams, in tests: afterReport runs between the verified report
	// and « proceed »; skipProceed never sends it.
	afterReport func()
	skipProceed bool
	// rewriteLauncher replaces the launcher command in tests (a faulty
	// launcher, plan T-4); it runs after every descriptor is set.
	rewriteLauncher func(*exec.Cmd)
	// Protected marks a launch for a protected project: refused where only
	// the namespace-free tier is available ([ErrProtectedRefused]).
	Protected bool
	// ReadPaths lists verified package and font directories needed by the workload.
	ReadPaths []string
	// Stdin is the child's standard input; nil means none.
	Stdin io.Reader
	// Stdout receives live output, in addition to the bounded retained copy.
	// A protocol consumer must bound and validate each message it accepts.
	Stdout io.Writer
	// Path is the executable, as returned by [Store.Resolve].
	Path string
	// Store verifies the pinned typst the first launches of a process run to
	// decide the sandbox tier (Linux); nil means the zero [Store].
	Store *Store
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
	// NoNetwork is kept for callers: every Linux launch is without network,
	// in a private network namespace in tier N, through seccomp (no socket)
	// and Landlock in tier F. Output.Isolated reports the namespace.
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
	// Isolated is true when the child ran in a private network namespace
	// (the namespaces tier, ADR 0044).
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
	if err := validateLaunch(ctx, &spec); err != nil {
		return Output{}, err
	}
	plan, err := planLaunch(spec)
	if err != nil {
		return Output{}, err
	}
	return run(ctx, spec, plan, newDirectoryMount)
}

// run launches a validated spec in plan's tier.
func run(ctx context.Context, spec Spec, plan launchPlan, prepareMount func(Spec) (*directoryMount, error)) (Output, error) {
	maxOut := spec.MaxOutput
	if maxOut <= 0 {
		maxOut = DefaultMaxOutput
	}
	stdout, stderr := &capped{max: maxOut}, &capped{max: maxOut}

	originalSpec := spec
	var mount *directoryMount
	var err error
	if plan.mounted() {
		if mount, err = prepareMount(spec); err != nil {
			return Output{}, err
		}
	}
	var prefix []string
	if mount == nil {
		spec, prefix, err = directoryLimits(spec, plan)
		originalSpec.slotsReady = true
	} else {
		defer mount.close()
		spec.FileSizeLimit, spec.MemoryLimit = 0, 0 // applied by the mount launcher
	}
	if err != nil {
		return Output{}, err
	}
	path, args, err := launchCommand(spec, plan, mount != nil, prefix)
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
	tierNamespaces(cmd, plan)
	if mount != nil {
		if err := mount.configure(cmd, plan); err != nil {
			return Output{}, err
		}
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
	hs, err := newHandshake(cmd, plan)
	if err != nil {
		return Output{}, err
	}
	defer hs.close()
	hs.afterReport, hs.skipProceed = spec.afterReport, spec.skipProceed
	if spec.rewriteLauncher != nil {
		spec.rewriteLauncher(cmd)
	}
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
			return run(ctx, originalSpec, plan, prepareMount)
		}
		return Output{}, fmt.Errorf("sidecar: start %s: %w", spec.Path, err)
	}
	if scope != nil {
		scope.afterStart()
	}
	if err := openRunMount(mount, cmd); err != nil {
		return Output{}, err
	}
	if err := hs.await(ctx); err != nil {
		err = stopLaunch(cmd, fmt.Errorf("sidecar: launch %s: %w", spec.Path, err))
		err = withLauncherCause(err, stderr.buf)
		return launchRefusal(cmd, scope != nil, finishReport, err)
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
		Isolated:  plan.isolated(),

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
	return runResult(ctx, out, waitErr, mount, originalSpec, spec.Path)
}

// runResult is the outcome of a run whose target was waited for.
func runResult(ctx context.Context, out Output, waitErr error, mount *directoryMount, originalSpec Spec, path string) (Output, error) {
	out, err := finishRunOutputs(out, mount, originalSpec)
	if err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil && out.ExitCode != 0 {
		return out, err
	}
	if waitErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](waitErr); !exited {
			return out, fmt.Errorf("sidecar: run %s: %w", path, waitErr)
		}
	}
	return out, nil
}

// launcherCauseMax bounds the launcher's own words kept in a launch error.
const launcherCauseMax = 512

// withLauncherCause adds to err what the launcher wrote on its standard error
// before it stopped: the target never ran (it starts after the handshake), so
// these are the launcher's own words, the cause of a missing tier report.
func withLauncherCause(err error, stderr []byte) error {
	cause := strings.TrimSpace(string(stderr[:min(len(stderr), launcherCauseMax)]))
	if cause == "" {
		return err
	}
	return fmt.Errorf("%w (%s)", err, cause)
}

// launchRefusal is the result of a launcher that stopped before its target:
// a bootstrap refusal it reported (it is not in the expected cgroup) stays
// told apart from any other launch failure, with the run's containment.
func launchRefusal(cmd *exec.Cmd, scoped bool, finishReport func(*Output), err error) (Output, error) {
	out := Output{Containment: "rlimit"}
	if scoped {
		out.Containment = "cgroup"
	}
	if cmd.ProcessState != nil {
		out.ExitCode = cmd.ProcessState.ExitCode()
	}
	finishReport(&out)
	if !out.bootstrapRefused {
		return Output{}, err
	}
	return out, fmt.Errorf("sidecar: bootstrap cgroup membership refused: %w", err)
}

func openRunMount(mount *directoryMount, cmd *exec.Cmd) error {
	if mount != nil {
		if err := mount.open(cmd.Process.Pid); err != nil {
			return stopLaunch(cmd, err)
		}
	}
	return nil
}

// stopLaunch kills the launcher's group and reaps it: the target never
// started. A launcher that had already exited on its own keeps its status.
func stopLaunch(cmd *exec.Cmd, err error) error {
	_ = signalGroup(cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill() // a command outside its own group
	_ = cmd.Wait()
	if code := cmd.ProcessState.ExitCode(); code > 0 {
		return errors.Join(err, launcherExit(code))
	}
	return err
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

// NetworkIsolation reports the sandbox tier of this machine (ADR 0044): true
// for the namespace tier, false for the namespace-free one, an error wrapping
// [ErrSandboxRefused] when the sidecars cannot run here, or
// [ErrProbeInconclusive] until the probe is conclusive (retry later). The tier
// is decided by launching the pinned typst, verified in store (nil: the zero
// [Store]), as `typst --version`.
func NetworkIsolation(store *Store) (bool, error) {
	return networkIsolation(store)
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
func launchCommand(spec Spec, plan launchPlan, mounted bool, prefix []string) (string, []string, error) {
	path, args := spec.Path, spec.Args
	var err error
	if !mounted && len(prefix) == 0 {
		path, args, err = withLimits(spec, plan)
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
