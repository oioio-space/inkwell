package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/oioio-space/inkwell/internal/bridge"
)

// A project change (a take-over or a join) is never applied to the running session: it is
// recorded, the bridge is drained, the application quits, [Runtime.Close] writes it once nothing
// else can write the member list, and the process restarts on the same folder (plan
// docs/plans/2026-11-14-pc1-project-change-by-restart.md, rules N-11..N-16).

// errLeaving refuses a request made once a change was requested: one change per process image
// (rule N-13).
var errLeaving = errors.New("restarting: nothing else can change until then")

// errNoRestart refuses a take-over where this build cannot restart Inkwell (residual R-8).
var errNoRestart = errors.New("take-over: Inkwell cannot restart on this platform")

// errClosing refuses a change request that was still draining when the runtime closed.
var errClosing = errors.New("closing: the change was not recorded")

// startupID is the random 128-bit identity of this process image (rule N-12): only it tells an
// image from the next one, since an execution in place keeps the pid.
var startupID = func() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // never fails (crypto/rand)
	return hex.EncodeToString(b[:])
}()

// changeWrite is the durable write of a change, run by [Runtime.Close] on the member-list store
// while it is still open and nothing else can write it.
type changeWrite func(store *opStore) error

// change is the one project change a process image may record.
type change struct {
	drain *bridge.Drain
	// canRestart is set where the process can restart itself (see [Options.CanRestart]).
	canRestart bool
	// after is set when this image was started by the restart of a change: its notice is shown
	// once (see [Session.StartupNotice]).
	after atomic.Bool

	mu        sync.Mutex
	requested bool        // a request started: every other one is refused
	write     changeWrite // recorded once the bridge is drained; nil before
	quit      func()      // asks the application to quit, once a change is recorded
	confirm   func() bool // consumes the desktop approval before the irreversible drain
	reject    func()      // releases the desktop hold if recording fails
	finished  bool        // the runtime closed and ran the write, if any (once): nothing is recorded after
}

func newChange(afterChange, canRestart bool) *change {
	c := &change{drain: bridge.NewDrain(), canRestart: canRestart}
	c.after.Store(afterChange)
	return c
}

// leaving reports errLeaving once a change was requested.
func (c *change) leaving() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.requested {
		return errLeaving
	}
	return nil
}

// request records the change write: it refuses a second request, drains the bridge (at most
// [bridge.DrainTimeout]), and only then records write and asks the application to quit. If the
// drain fails, or the runtime closed meanwhile, nothing is recorded: the request fails, nothing is
// written, and the streams stay closed until the person quits (rule N-16).
func (c *change) request(ctx context.Context, write changeWrite) (err error) {
	c.mu.Lock()
	if c.requested {
		c.mu.Unlock()
		return errLeaving
	}
	if c.confirm != nil && !c.confirm() {
		c.mu.Unlock()
		return errors.New("project change has no confirmed desktop close")
	}
	reject := c.reject
	c.requested = true
	c.mu.Unlock()
	defer func() {
		if err != nil && reject != nil {
			reject()
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, bridge.DrainTimeout)
	defer cancel()
	if err := c.drain.Drain(ctx); err != nil {
		return fmt.Errorf("closing the bridge before the change: %w", err)
	}
	c.mu.Lock()
	if c.finished { // the runtime closed while the bridge drained: a restart would follow no write
		c.mu.Unlock()
		return errClosing
	}
	c.write = write
	quit := c.quit
	c.mu.Unlock()
	if quit != nil {
		// Not inside this call: its answer leaves first, and a shutdown that waits for the calls
		// in flight (the server build's) never waits for the one that asked for it.
		go quit()
	}
	return nil
}

// recorded reports whether a change was recorded: the process then restarts once it closed.
func (c *change) recorded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write != nil
}

// finish runs the recorded write once, on store.
func (c *change) finish(store *opStore) error {
	c.mu.Lock()
	write := c.write
	if c.finished {
		write = nil
	}
	c.finished = true
	c.mu.Unlock()
	if write == nil {
		return nil
	}
	return write(store)
}

// StartupID returns the random identity of this process image: it differs after a restart,
// which keeps the pid.
func (s *Session) StartupID() string { return startupID }

// StartupNotice returns, once, the catalogue key of the sentence that follows a take-over, chosen
// from the standing this image actually loaded: "foreign.takeOver.done" for a member,
// "foreign.takeOver.failed" otherwise. It is empty when this image was not started by such a
// restart, and on every later call.
func (s *Session) StartupNotice() string {
	if s.chg == nil || !s.chg.after.Swap(false) {
		return ""
	}
	if s.Standing() == StandingMember {
		return "foreign.takeOver.done"
	}
	return "foreign.takeOver.failed"
}

// leaving reports errLeaving once a change was requested, errNoProject without a project.
func (s *Session) leaving() error {
	if s.chg == nil {
		return errNoProject
	}
	return s.chg.leaving()
}

// requestChange is the entry point of a project change (take-over, N-5; join, J-1): see
// [change.request].
func (s *Session) requestChange(ctx context.Context, write changeWrite) error {
	if s.chg == nil {
		return errNoProject
	}
	return s.chg.request(ctx, write)
}

// SetQuit sets how a recorded change asks the application to quit; it is called before the
// window runs.
func (r *Runtime) SetQuit(quit func()) {
	r.chg.mu.Lock()
	defer r.chg.mu.Unlock()
	r.chg.quit = quit
}

// SetProjectClose makes project changes consume a desktop save approval before
// draining the bridge and reports a failed transition to the coordinator.
func (r *Runtime) SetProjectClose(confirm func() bool, reject func()) {
	r.chg.mu.Lock()
	defer r.chg.mu.Unlock()
	r.chg.confirm, r.chg.reject = confirm, reject
}

// RestartRequested reports whether this image recorded a project change: once the runtime is
// closed, the process restarts on the same folder (rule N-12). It is false for a nil runtime.
func (r *Runtime) RestartRequested() bool { return r != nil && r.chg.recorded() }

// CanTakeOver reports whether a take-over can be offered: only where Inkwell can restart itself
// (residual R-8 of the PC1 plan).
func (s *Session) CanTakeOver() bool { return s.chg != nil && s.chg.canRestart }
