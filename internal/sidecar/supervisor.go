package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ErrGaveUp is returned by [Supervisor.Run] after too many consecutive
// failures to run the program.
var ErrGaveUp = errors.New("sidecar: gave up restarting")

// Supervisor defaults.
const (
	defaultInitialBackoff = 250 * time.Millisecond
	defaultMaxBackoff     = 15 * time.Second
	defaultStableAfter    = 30 * time.Second
	defaultMaxFailures    = 10
	defaultStopGrace      = 3 * time.Second
)

// Backoff is the pause between a failure and the next launch: Initial, then
// multiplied by Factor after each consecutive failure, up to Max.
type Backoff struct {
	// Initial is the first pause. The default is 250 ms.
	Initial time.Duration
	// Max caps the pause. The default is 15 s.
	Max time.Duration
	// Factor multiplies the pause after each failure. The default is 2.
	Factor float64
}

// SupervisorConfig configures a [Supervisor].
type SupervisorConfig struct {
	// Command builds the command of one launch. It is called before every
	// launch, the first included, so it is the place to verify the binary
	// ([Store.Resolve]) and to wire fresh pipes. It must use [exec.Command],
	// not [exec.CommandContext]: the supervisor stops the process itself. An
	// error counts as a failed launch.
	Command func(ctx context.Context) (*exec.Cmd, error)
	// OnEvent, if set, is called from the supervising goroutine for each
	// [Event]; it must not block.
	OnEvent func(Event)
	// Name labels the events and errors.
	Name string
	// Backoff paces the restarts; zero fields take their defaults.
	Backoff Backoff
	// StableAfter is how long a launch must run for the failure count and the
	// pause to start over. The default is 30 s.
	StableAfter time.Duration
	// StopGrace is how long a child gets between SIGTERM and SIGKILL when the
	// supervisor stops. The default is 3 s.
	StopGrace time.Duration
	// MaxFailures is the number of consecutive failed launches after which
	// [Supervisor.Run] returns [ErrGaveUp]. Zero means 10; a negative value
	// means never give up.
	MaxFailures int
}

func (c SupervisorConfig) withDefaults() SupervisorConfig {
	b := &c.Backoff
	if b.Initial <= 0 {
		b.Initial = defaultInitialBackoff
	}
	if b.Max <= 0 {
		b.Max = defaultMaxBackoff
	}
	b.Max = max(b.Max, b.Initial)
	if b.Factor < 1 {
		b.Factor = 2
	}
	if c.StableAfter <= 0 {
		c.StableAfter = defaultStableAfter
	}
	if c.StopGrace <= 0 {
		c.StopGrace = defaultStopGrace
	}
	if c.MaxFailures == 0 {
		c.MaxFailures = defaultMaxFailures
	}
	return c
}

// EventKind says what an [Event] reports.
type EventKind int

// The kinds of [Event], in the order a launch goes through them.
const (
	// EventPrepareFailed: Command returned an error; nothing was started.
	EventPrepareFailed EventKind = iota
	// EventStartFailed: the program could not be started.
	EventStartFailed
	// EventStart: the program is running (PID).
	EventStart
	// EventExit: the program ended (ExitCode, Err, Uptime).
	EventExit
	// EventBackoff: the supervisor waits Delay before the next launch.
	EventBackoff
	// EventGiveUp: too many consecutive failures; Run is about to return.
	EventGiveUp
)

var kindNames = [...]string{"prepare-failed", "start-failed", "start", "exit", "backoff", "give-up"}

func (k EventKind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return fmt.Sprintf("EventKind(%d)", int(k))
	}
	return kindNames[k]
}

// Event is one step in the life of a supervised program, for logs and status.
type Event struct {
	// Err is the failure, for the failed and exit kinds.
	Err error
	// Sidecar is the supervisor's Name.
	Sidecar string
	// Uptime is how long the program ran (EventExit).
	Uptime time.Duration
	// Delay is the pause that follows (EventBackoff).
	Delay time.Duration
	// Kind says what happened.
	Kind EventKind
	// PID is the process id (EventStart, EventExit).
	PID int
	// ExitCode is the exit status, -1 if a signal ended it (EventExit).
	ExitCode int
}

// Supervisor keeps one program running: it relaunches it with a growing pause
// when it ends, verifies it before every launch (through Command), and stops it
// and its whole process group when its context ends.
type Supervisor struct {
	cfg      SupervisorConfig
	restarts atomic.Int64
}

// NewSupervisor returns a supervisor; call [Supervisor.Run] to start it.
func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	return &Supervisor{cfg: cfg.withDefaults()}
}

// Restarts is the number of launches after the first.
func (s *Supervisor) Restarts() int { return int(s.restarts.Load()) }

// Run launches and supervises the program until ctx ends, then stops it and
// returns nil. It returns an error wrapping [ErrGaveUp] and the last failure
// after MaxFailures consecutive failed launches. It blocks: run it in its own
// goroutine.
func (s *Supervisor) Run(ctx context.Context) error {
	if s.cfg.Command == nil {
		return errors.New("sidecar: supervisor: Command is required")
	}
	// The parent-death signal follows the thread that forks the child: this
	// goroutine keeps it for the whole supervision.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cfg := s.cfg
	failures := 0
	delay := cfg.Backoff.Initial
	for first := true; ; first = false {
		if ctx.Err() != nil {
			return nil
		}
		if !first {
			s.restarts.Add(1)
		}
		uptime, err := s.runOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if uptime >= cfg.StableAfter {
			failures, delay = 0, cfg.Backoff.Initial
		}
		failures++
		if cfg.MaxFailures > 0 && failures >= cfg.MaxFailures {
			s.emit(Event{Kind: EventGiveUp, Err: err})
			return fmt.Errorf("%w: %s: %d consecutive failures, last: %w", ErrGaveUp, cfg.Name, failures, err)
		}
		s.emit(Event{Kind: EventBackoff, Delay: delay})
		if !sleep(ctx, delay) {
			return nil
		}
		delay = min(time.Duration(float64(delay)*cfg.Backoff.Factor), cfg.Backoff.Max)
	}
}

// runOnce performs one launch and returns how long the program ran and why it
// ended; the error is never nil.
func (s *Supervisor) runOnce(ctx context.Context) (time.Duration, error) {
	cmd, err := s.cfg.Command(ctx)
	if err != nil {
		s.emit(Event{Kind: EventPrepareFailed, Err: err})
		return 0, fmt.Errorf("prepare: %w", err)
	}
	setProcAttr(cmd)
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = waitDelay
	}
	began := time.Now()
	if err := cmd.Start(); err != nil {
		s.emit(Event{Kind: EventStartFailed, Err: err})
		return 0, fmt.Errorf("start: %w", err)
	}
	pid := cmd.Process.Pid
	s.emit(Event{Kind: EventStart, PID: pid})

	stopped := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			_ = signalGroup(pid, syscall.SIGTERM)
			grace := time.NewTimer(s.cfg.StopGrace)
			defer grace.Stop()
			select {
			case <-stopped:
			case <-grace.C:
				_ = signalGroup(pid, syscall.SIGKILL)
			}
		case <-stopped:
		}
	})
	waitErr := cmd.Wait()
	close(stopped)
	wg.Wait()
	// Whatever the program left behind in its group goes with it.
	_ = signalGroup(pid, syscall.SIGKILL)

	uptime := time.Since(began)
	code := cmd.ProcessState.ExitCode()
	s.emit(Event{Kind: EventExit, PID: pid, ExitCode: code, Err: waitErr, Uptime: uptime})
	if waitErr == nil {
		waitErr = errors.New("exit status 0")
	}
	return uptime, fmt.Errorf("exited after %v: %w", uptime.Round(time.Millisecond), waitErr)
}

func (s *Supervisor) emit(e Event) {
	if s.cfg.OnEvent == nil {
		return
	}
	e.Sidecar = s.cfg.Name
	s.cfg.OnEvent(e)
}

// sleep waits d or until ctx ends; it reports whether the full time elapsed.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
