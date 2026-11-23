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

// errNoWelcome refuses [Session.FinishJoin] when no verified Welcome is held (rule J-1).
var errNoWelcome = errors.New("join: no accepted invitation to finish")

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

// The kinds of project change, as the restart line of rule J-7 names them.
const (
	ChangeTakeOver = "takeOver" // the take-over of a foreign folder (rule N-5)
	ChangeJoin     = "join"     // an accepted invitation into another project (rule J-1)
	ChangeReturn   = "return"   // the return to the document open before a join (rule J-2)
)

// change is the one project change a process image may record.
type change struct {
	drain *bridge.Drain
	// canRestart is set where the process can restart itself (see [Options.CanRestart]).
	canRestart bool
	// after is set when this image was started by the restart of a change: its notice is shown
	// once (see [Session.StartupNotice]).
	after atomic.Bool
	// joinFailed is set when this image found, and removed, what an unfinished join left beside
	// its folder: it says so once (rule J-4).
	joinFailed atomic.Bool

	mu        sync.Mutex
	requested bool        // a request started: every other one is refused
	held      changeWrite // the write of a verified Welcome, until [Session.FinishJoin] (rule J-1)
	joined    string      // the folder a join installed: the restart opens it (rule J-6)
	write     changeWrite // recorded once the bridge is drained; nil before
	kind      string      // what write records (ChangeTakeOver, ChangeJoin, ChangeReturn), set with it
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
func (c *change) request(ctx context.Context, kind string, write changeWrite) error {
	c.mu.Lock()
	if c.requested {
		c.mu.Unlock()
		return errLeaving
	}
	reject, err := c.approveLocked()
	if err != nil {
		c.mu.Unlock()
		return err
	}
	c.requested = true
	c.mu.Unlock()
	return c.record(ctx, kind, write, reject)
}

// approveLocked consumes the desktop approval of the irreversible drain, when the desktop close
// coordinator asks for one (see [Runtime.SetProjectClose]), and returns how to release that hold
// if recording fails. c.mu is held.
func (c *change) approveLocked() (reject func(), err error) {
	if c.confirm != nil && !c.confirm() {
		return nil, errors.New("project change has no confirmed desktop close")
	}
	return c.reject, nil
}

// hold keeps the write of a verified Welcome until [change.finishHeld]: nothing is drained,
// written or asked yet, but the request has started, so every other one is refused (rules J-1
// and N-13).
func (c *change) hold(write changeWrite) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.requested {
		return errLeaving
	}
	c.held = write
	c.requested = true
	return nil
}

// finishHeld records the held write as [change.request] records one; it is refused when nothing
// is held, as a second call is. The desktop approval is consumed here, at the irreversible step;
// without it the Welcome is dropped as [change.cancelHeld] drops it: the restart page comes back
// with « nothing changed », so the session must be as before the request (rule N-15).
func (c *change) finishHeld(ctx context.Context) error {
	c.mu.Lock()
	write := c.held
	if write == nil {
		c.mu.Unlock()
		return errNoWelcome
	}
	reject, err := c.approveLocked()
	c.held = nil
	if err != nil {
		c.requested = false
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()
	return c.record(ctx, ChangeJoin, write, reject)
}

// cancelHeld drops a held Welcome: the session is as before the request (rule N-15). It does
// nothing once the write is being recorded, or when nothing is held.
func (c *change) cancelHeld() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.held != nil {
		c.held, c.requested = nil, false
	}
}

// record drains the bridge, then records write, a change of kind kind, and asks the application to
// quit: the second half of a request. When it fails, reject (if any) releases the desktop hold.
func (c *change) record(ctx context.Context, kind string, write changeWrite, reject func()) (err error) {
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
	c.write, c.kind = write, kind
	quit := c.quit
	c.mu.Unlock()
	if quit != nil {
		// Not inside this call: its answer leaves first, and a shutdown that waits for the calls
		// in flight (the server build's) never waits for the one that asked for it.
		go quit()
	}
	return nil
}

// recorded returns the kind of the recorded change, "" when none was: the process restarts once
// it closed when one was.
func (c *change) recorded() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kind
}

// setJoined notes the folder the write of a join installed.
func (c *change) setJoined(folder string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.joined = folder
}

// joinedFolder returns the folder a join installed, or "".
func (c *change) joinedFolder() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.joined
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

// StartupNotice returns, once, the catalogue key of the sentence that follows a project change,
// chosen from what this image actually loaded: "join.failed" when it found and removed what an
// unfinished join left (rule J-4); nothing while the folder waits for the document of a join
// (the page says it waits, then that the document was joined, rule J-2); after a take-over
// "foreign.takeOver.done" for a member, "foreign.takeOver.failed" otherwise. It is empty when
// this image was not started by such a restart, and on every later call.
func (s *Session) StartupNotice() string {
	if s.chg == nil {
		return ""
	}
	after := s.chg.after.Swap(false)
	if s.chg.joinFailed.Swap(false) {
		return "join.failed"
	}
	if !after || s.PendingJoin().Waiting {
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

// requestChange is the entry point of a project change of kind kind (take-over, N-5; return,
// J-2): see [change.request].
func (s *Session) requestChange(ctx context.Context, kind string, write changeWrite) error {
	if s.chg == nil {
		return errNoProject
	}
	return s.chg.request(ctx, kind, write)
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

// RestartChange returns the kind of the project change this image recorded (ChangeTakeOver,
// ChangeJoin or ChangeReturn), which the restart line names (rule J-7): once the runtime is
// closed, the process restarts on the same folder (rule N-12); "" when it recorded none,
// and for a nil runtime.
func (r *Runtime) RestartChange() string {
	if r == nil {
		return ""
	}
	return r.chg.recorded()
}

// RestartFolder returns the folder the restart of a recorded change opens when it is not the
// open project's: the folder a join installed (rule J-6). It is empty otherwise, and for a nil
// runtime.
func (r *Runtime) RestartFolder() string {
	if r == nil {
		return ""
	}
	return r.chg.joinedFolder()
}

// CanTakeOver reports whether a take-over can be offered: only where Inkwell can restart itself
// (residual R-8 of the PC1 plan).
func (s *Session) CanTakeOver() bool { return s.chg != nil && s.chg.canRestart }
