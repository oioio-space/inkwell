package main

import (
	"context"
	"sync"
	"time"
)

const closeDeadline = 5 * time.Second

// closeFrame shares the open stream with the existing open/handled messages.
type closeFrame struct {
	Kind    string   `json:"close"`
	Attempt uint32   `json:"attempt"`
	Reason  string   `json:"reason,omitempty"`
	Files   []string `json:"files,omitempty"`
}

type closeEditor struct {
	close      func()
	send       func(closeFrame) error // must not block
	abort      func(closeFrame)       // queues a reliable abort without blocking
	generation uint64
	started    bool // a page has been allowed to start editing, even if it disconnected
	consumed   bool
	crashed    bool
}

type closeCut struct {
	generation uint64
	sent       bool
	saved      bool
	presence   bool
	complete   bool
}

type closeAttempt struct {
	deadline  time.Time
	timer     *time.Timer
	cuts      map[uint]*closeCut
	id        uint32
	window    uint // zero is application quit
	failure   closeFrame
	prepared  bool
	approved  bool
	forced    bool
	project   bool
	leaving   bool
	committed bool
}

func (a *closeAttempt) stopTimer() {
	if a.timer != nil {
		a.timer.Stop()
	}
}

// closeCoordinator owns both the attempt and its authorization. Neither a
// stream operation nor a Wails main-thread operation runs synchronously in a hook.
type closeCoordinator struct {
	mu      sync.Mutex
	editors map[uint]*closeEditor
	current *closeAttempt
	next    uint32
	changed chan struct{}
	pause   func(bool)
	drain   func(time.Time)
	observe func(string)
}

func newCloseCoordinator() *closeCoordinator {
	return &closeCoordinator{editors: make(map[uint]*closeEditor), changed: make(chan struct{})}
}

func (c *closeCoordinator) notifyLocked() {
	close(c.changed)
	c.changed = make(chan struct{})
}

func (c *closeCoordinator) register(id uint, closeWindow func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.editors[id] = &closeEditor{close: closeWindow}
}

// shouldQuit is Wails' application-level veto. The native window close hooks
// must first consume the authorization of every editor in the exit scope.
func (c *closeCoordinator) shouldQuit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, editor := range c.editors {
		if !editor.consumed {
			c.requestAllLocked()
			return false
		}
	}
	return true
}

func (c *closeCoordinator) forceExit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current != nil && c.current.approved && c.current.forced && c.lastEditorLocked()
}

func (c *closeCoordinator) editorClosed(id uint) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.editors, id)
	if len(c.editors) == 0 {
		return true
	}
	if a := c.current; a != nil && a.approved && a.window == id {
		c.clearLocked()
	}
	return false
}

func (c *closeCoordinator) attach(id uint, send func(closeFrame) error, abort func(closeFrame)) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	editor := c.editors[id]
	if editor == nil || editor.consumed {
		return 0
	}
	if a := c.current; a != nil && a.leaving {
		return 0 // the bridge is draining for the recorded project transition
	}
	if a := c.current; a != nil && a.cuts[id] != nil && a.failure.Kind == "" {
		if !editor.started {
			return 0 // the attempt already counted this window as empty
		}
		if a.approved && !a.leaving {
			c.abortLocked(a, closeFrame{Kind: "failed", Reason: "replaced"})
			c.clearLocked()
		} else if a.prepared || editor.send != nil {
			c.abortLocked(a, closeFrame{Kind: "failed", Reason: "replaced"})
		}
	}
	editor.generation++
	editor.send = send
	editor.abort = abort
	editor.crashed = false
	return editor.generation
}

// ready allows workspace startup. Serialize it with prepare so an empty cut
// cannot gain an editable page after the coordinator has counted it.
func (c *closeCoordinator) ready(id uint, generation uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	editor := c.editors[id]
	if editor == nil || editor.consumed || editor.generation != generation || editor.send == nil {
		return false
	}
	if a := c.current; a != nil && a.cuts[id] != nil && a.failure.Kind == "" && !editor.started {
		return false
	}
	if err := editor.send(closeFrame{Kind: "ready"}); err != nil {
		return false
	}
	editor.started = true
	return true
}

func (c *closeCoordinator) rendererCrashed(id uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	editor := c.editors[id]
	if editor == nil || editor.consumed {
		return
	}
	editor.crashed = true
	editor.send = nil
	c.recordLocked("renderer-crashed")
	if a := c.current; a != nil && a.cuts[id] != nil && !a.approved && a.failure.Kind == "" {
		c.abortLocked(a, closeFrame{Kind: "failed", Reason: "crashed"})
	}
}

func (c *closeCoordinator) detach(id uint, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	editor := c.editors[id]
	if editor == nil || generation != editor.generation || editor.consumed {
		return
	}
	editor.send = nil
	if a := c.current; a != nil && editor.started && a.cuts[id] != nil && !a.approved && a.failure.Kind == "" {
		c.abortLocked(a, closeFrame{Kind: "failed", Reason: "renderer"})
	}
}

func (c *closeCoordinator) requestWindow(id uint) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	editor := c.editors[id]
	if editor == nil {
		return true // independent presentation window
	}
	if editor.consumed {
		return false
	}
	if a := c.current; a != nil && a.approved && (!a.project || a.committed || a.forced) {
		if cut := a.cuts[id]; cut != nil && cut.generation == editor.generation {
			editor.consumed = true
			c.notifyLocked()
			return true
		}
	}
	if c.current == nil {
		c.startLocked(id, time.Now().Add(closeDeadline))
	}
	return false
}

func (c *closeCoordinator) requestAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestAllLocked()
}

func (c *closeCoordinator) requestAllLocked() {
	when := time.Now().Add(closeDeadline)
	if a := c.current; a != nil {
		if a.window == 0 || a.approved || a.failure.Kind != "" {
			return
		}
		when = a.deadline
		c.abortLocked(a, closeFrame{Kind: "failed"})
	}
	c.startLocked(0, when)
}

func (c *closeCoordinator) beginProjectChange() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.current
	if a == nil || !a.project || !a.approved || a.forced || a.leaving {
		return false
	}
	a.leaving = true
	return true
}

func (c *closeCoordinator) finishProjectChange() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a := c.current; a != nil && a.project && a.leaving {
		a.committed = true
		c.releaseLocked(a)
	}
}

func (c *closeCoordinator) rejectProjectChange() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a := c.current; a != nil && a.project && a.leaving && !a.committed {
		c.abortLocked(a, closeFrame{Kind: "failed", Reason: "leaving"})
		c.clearLocked()
	}
}

// waitExit waits for a session manager's attempt to resolve. The initial
// query and the actual end-session message have separate OS deadlines.
func (c *closeCoordinator) waitExit(ctx context.Context) bool {
	c.requestAll()
	for {
		c.mu.Lock()
		a, changed := c.current, c.changed
		c.mu.Unlock()
		if a == nil {
			return false
		}
		c.mu.Lock()
		ok, failed := a.approved, a.failure.Kind != ""
		c.mu.Unlock()
		if ok || failed {
			return ok
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return false
		}
	}
}

func (c *closeCoordinator) startLocked(window uint, deadline time.Time) {
	c.next++
	if c.next == 0 {
		c.next++
	}
	a := &closeAttempt{id: c.next, window: window, deadline: deadline, cuts: make(map[uint]*closeCut)}
	var hasStarted, hasCrashed bool
	for id, editor := range c.editors {
		if !editor.consumed && (window == 0 || window == id) {
			a.cuts[id] = &closeCut{generation: editor.generation, complete: !editor.started}
			hasStarted = hasStarted || editor.started
			hasCrashed = hasCrashed || editor.crashed
		}
	}
	c.current = a
	if hasCrashed {
		c.abortLocked(a, closeFrame{Kind: "failed", Reason: "crashed"})
		return
	}
	c.notifyLocked()
	var drainUntil time.Time
	if window == 0 || c.lastEditorLocked() {
		if c.pause != nil {
			c.pause(true)
		}
		drainUntil = deadline
		if !hasStarted {
			// No page could have received an open request. Log any pending
			// opens through the usual drain path without waiting for a reply.
			drainUntil = time.Now()
		}
	}
	a.timer = time.AfterFunc(time.Until(deadline), func() { c.expire(a) })
	go c.prepare(a, drainUntil)
}

func (c *closeCoordinator) expire(a *closeAttempt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current != a || a.approved || a.failure.Kind != "" {
		return
	}
	c.abortLocked(a, closeFrame{Kind: "failed", Reason: "timeout"})
}

func (c *closeCoordinator) prepare(a *closeAttempt, drainUntil time.Time) {
	if !drainUntil.IsZero() && c.drain != nil {
		c.drain(drainUntil)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current != a || a.failure.Kind != "" || a.approved {
		return
	}
	a.prepared = true
	for id, cut := range a.cuts {
		if cut.complete {
			continue
		}
		editor := c.editors[id]
		if editor == nil {
			cut.complete = true
			continue
		}
		cut.generation = editor.generation
		if editor.send == nil {
			continue
		}
		if err := editor.send(closeFrame{Kind: "prepare", Attempt: a.id}); err != nil {
			c.abortLocked(a, closeFrame{Kind: "failed", Reason: "renderer"})
			return
		}
		cut.sent = true
	}
	c.approveCompletedLocked(a)
}

func (c *closeCoordinator) lastEditorLocked() bool {
	live := 0
	for _, editor := range c.editors {
		if !editor.consumed {
			live++
		}
	}
	return live <= 1
}

func (c *closeCoordinator) presence(window uint, attempt uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.current
	if a != nil && a.id == attempt && a.prepared && !a.approved && a.failure.Kind == "" {
		if cut := a.cuts[window]; cut != nil && cut.sent {
			cut.presence = true
			c.recordLocked("goodbye-handed-off")
		}
	}
}

func (c *closeCoordinator) report(window uint, generation uint64, frame closeFrame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.current
	if frame.Kind == "project" {
		if editor := c.editors[window]; editor != nil && editor.generation == generation && editor.send != nil {
			if a != nil {
				_ = editor.send(closeFrame{Kind: "project-denied", Attempt: a.id})
				return
			}
			c.startLocked(0, time.Now().Add(closeDeadline))
			c.current.project = true
		}
		return
	}
	if a == nil || a.id != frame.Attempt || !a.prepared || a.approved || a.failure.Kind != "" {
		return
	}
	if !time.Now().Before(a.deadline) {
		c.abortLocked(a, closeFrame{Kind: "failed", Reason: "timeout"})
		return
	}
	cut := a.cuts[window]
	if cut == nil || !cut.sent || cut.generation != generation {
		return
	}
	switch frame.Kind {
	case "saved":
		cut.saved = true
		c.recordLocked("save-ack-durable")
	case "failed":
		c.abortLocked(a, frame)
	case "empty":
		cut.complete = true
		c.approveCompletedLocked(a)
	case "complete":
		if !cut.saved || !cut.presence {
			return
		}
		cut.complete = true
		c.approveCompletedLocked(a)
	}
}

func (c *closeCoordinator) approveCompletedLocked(a *closeAttempt) {
	for _, cut := range a.cuts {
		if !cut.complete {
			return
		}
	}
	c.approveLocked(a)
}

func (c *closeCoordinator) approveLocked(a *closeAttempt) {
	a.stopTimer()
	a.approved = true
	c.recordLocked("close-authorized")
	c.notifyLocked()
	if a.project && !a.forced {
		for id, cut := range a.cuts {
			if editor := c.editors[id]; cut.sent && editor != nil && editor.send != nil {
				if err := editor.send(closeFrame{Kind: "project-approved", Attempt: a.id}); err != nil {
					c.abortLocked(a, closeFrame{Kind: "failed", Reason: "renderer"})
					return
				}
			}
		}
		return
	}
	c.releaseLocked(a)
}

func (c *closeCoordinator) recordLocked(event string) {
	if c.observe != nil {
		c.observe(event)
	}
}

func (c *closeCoordinator) releaseLocked(a *closeAttempt) {
	for id := range a.cuts {
		if editor := c.editors[id]; editor != nil && !editor.consumed {
			go editor.close()
		}
	}
}

func (c *closeCoordinator) abortLocked(a *closeAttempt, failure closeFrame) {
	a.stopTimer()
	a.approved = false
	failure.Attempt = a.id
	a.failure = failure
	for id, cut := range a.cuts {
		if editor := c.editors[id]; editor != nil && !editor.consumed && editor.send != nil &&
			cut.sent && editor.generation == cut.generation {
			editor.abort(closeFrame{Kind: "abort", Attempt: a.id})
		}
	}
	if c.pause != nil {
		c.pause(false)
	}
	c.recordLocked("close-failed")
	c.notifyLocked()
}

func (c *closeCoordinator) clearLocked() {
	c.current = nil
	c.notifyLocked()
}

func (c *closeCoordinator) failedLocked(id uint32) *closeAttempt {
	if a := c.current; a != nil && a.id == id && a.failure.Kind != "" && !a.approved {
		return a
	}
	return nil
}

func (c *closeCoordinator) dismiss(id uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a := c.failedLocked(id); a != nil {
		if a.project {
			for id := range a.cuts {
				if editor := c.editors[id]; editor != nil && editor.send != nil {
					_ = editor.send(closeFrame{Kind: "project-denied", Attempt: a.id})
				}
			}
		}
		c.clearLocked()
	}
}

func (c *closeCoordinator) retry(id uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a := c.failedLocked(id); a != nil {
		c.startLocked(a.window, time.Now().Add(closeDeadline))
		c.current.project = a.project
	}
}

func (c *closeCoordinator) force(id uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.failedLocked(id)
	if a == nil {
		return
	}
	a.forced = true
	for window, cut := range a.cuts {
		if editor := c.editors[window]; editor != nil {
			cut.generation = editor.generation
		}
	}
	if c.pause != nil && (a.window == 0 || c.lastEditorLocked()) {
		c.pause(true)
	}
	c.approveLocked(a)
}
