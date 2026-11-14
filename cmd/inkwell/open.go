package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/instance"
	"github.com/oioio-space/inkwell/internal/launch"
)

// openStreamName is the Wails stream that carries the open requests to the window (ADR 0009: the
// bridge is streams).
const openStreamName = "open"

const (
	// openQueueSize bounds the requests the primary holds, those sent to the window and not yet
	// handled included: four times a burst of 16 launches; a launch told « busy » tries again.
	openQueueSize = 64
	// maxInFlight bounds the requests sent to the window and not yet handled (the webview's own
	// bound, MAX_PENDING).
	maxInFlight = 8
	// drainTimeout bounds how long a closing window may take to report the requests it was given.
	drainTimeout = 2 * time.Second
	// maxHandledFrame bounds a frame of the webview on the open stream ({"handled":n}).
	maxHandledFrame = 64
)

// openEntry is a queued request, its number on the stream, and the generation of the stream it was
// delivered to (0: not delivered, or given back).
type openEntry struct {
	req instance.Request
	seq uint64
	gen uint64
}

// openQueue holds the open requests, the primary's own first, until the window reports each
// handled (its file shown, or its toast up): a request keeps its slot until then, so the bound
// counts what is in flight too. It refuses launches (« busy ») until it is opened, once the
// identity, the project and the network are up: a launch is acknowledged only by a primary able to
// show its file (rule D-ACK). It is read for as long as the window lives (rule D-DRAIN).
//
// One owner at a time (rule D-OWNER): each connection of the window to the open stream is a
// generation; a new one supersedes the previous, whose undelivered work goes back to the queue; a
// report frees a request only from the generation it was delivered to.
type openQueue struct {
	mu     sync.Mutex
	items  []*openEntry
	seq    uint64
	gen    uint64 // the current connection's generation, 0 before the first
	opened bool
	wake   chan struct{} // closed and replaced when the queue changes
}

func newOpenQueue() *openQueue { return &openQueue{wake: make(chan struct{})} }

func (q *openQueue) changedLocked() {
	close(q.wake)
	q.wake = make(chan struct{})
}

// changed is closed at the next change of the queue.
func (q *openQueue) changed() <-chan struct{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.wake
}

func (q *openQueue) entryLocked(r instance.Request) *openEntry {
	q.seq++
	return &openEntry{req: r, seq: q.seq}
}

// own queues the primary's own request, first, whatever the state of the queue.
func (q *openQueue) own(r instance.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append([]*openEntry{q.entryLocked(r)}, q.items...)
	q.changedLocked()
}

// open lets launches in: from now on push queues them.
func (q *openQueue) open() {
	q.mu.Lock()
	q.opened = true
	q.mu.Unlock()
}

// push queues a launch's request and reports whether it did: false before open, and when
// openQueueSize requests are held (instance.Primary.Serve then answers « busy »). It never blocks.
func (q *openQueue) push(r instance.Request) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.opened || len(q.items) >= openQueueSize {
		return false
	}
	q.items = append(q.items, q.entryLocked(r))
	q.changedLocked()
	return true
}

// connect starts a new generation, the current one from now on: what the previous one was given
// and had not reported handled goes back to the queue, to be sent on the new one, first, in order.
func (q *openQueue) connect() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.giveBackLocked(q.gen)
	q.gen++
	return q.gen
}

// release is the end of the stream of generation g: what it was given and had not reported handled
// goes back to the queue. Nothing delivered to another generation moves.
func (q *openQueue) release(g uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.giveBackLocked(g)
}

func (q *openQueue) giveBackLocked(g uint64) {
	for _, e := range q.items {
		if e.gen == g {
			e.gen = 0
		}
	}
	q.changedLocked()
}

// take returns the oldest request not delivered, marked delivered to generation g, when g is the
// current generation and fewer than maxInFlight are in flight on it.
func (q *openQueue) take(g uint64) (*openEntry, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if g != q.gen {
		return nil, false
	}
	inFlight := 0
	for _, e := range q.items {
		if e.gen != 0 {
			inFlight++
			continue
		}
		if inFlight >= maxInFlight {
			return nil, false
		}
		e.gen = g
		return e, true
	}
	return nil, false
}

// handled removes the request numbered seq, which the window of generation g reported handled; false,
// and nothing moves, when it is not held or was delivered to another generation (a superseded
// stream's report).
func (q *openQueue) handled(g, seq uint64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, e := range q.items {
		if e.seq == seq && e.gen == g && g != 0 {
			q.items = append(q.items[:i], q.items[i+1:]...)
			q.changedLocked()
			return true
		}
	}
	return false
}

// pending is the number of requests held.
func (q *openQueue) pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// waitHandled waits until the window handled every request held, or d has passed, and returns how
// many are left.
func (q *openQueue) waitHandled(d time.Duration) int {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		q.mu.Lock()
		left, wake := len(q.items), q.wake
		q.mu.Unlock()
		if left == 0 {
			return 0
		}
		select {
		case <-wake:
		case <-timer.C:
			return left
		}
	}
}

// drainOnClose is the closing of the window: stop acknowledging launches (closing), then let the
// window report handled what was acknowledged, within d (rule D-DRAIN at close). What is still not
// handled at the bound is logged: the residual of a window that cannot answer while it closes.
func drainOnClose(q *openQueue, closing func(), d time.Duration, log *slog.Logger) {
	closing()
	if left := q.waitHandled(d); left > 0 {
		log.Warn("window closed before showing every open request", "requests", left)
	}
}

// opener sends the open requests to the window: it resolves each against the working directory of
// its launch and the open project, raises the window, and sends the outcome to the webview with its
// number; the webview shows the file or says in a toast why it cannot, then reports it handled.
type opener struct {
	queue *openQueue
	env   launch.Env // without Cwd, which comes with each request
	raise func(activation string)
	log   *slog.Logger
	mu    sync.Mutex
	cur   openConn // the current connection, closed when a new one supersedes it
}

// openConn is the open stream (a Wails stream): Go closes a superseded one.
type openConn interface {
	bridge.Conn
	Close() error
}

// openFrame is what the webview gets for one request: its number, which the webview reports back
// once it handled it ({"handled":id}), and the outcome.
type openFrame struct {
	ID uint64 `json:"id"`
	launch.Outcome
}

// Serve sends the queued requests on c, at most maxInFlight not yet handled, until c closes or a
// newer connection supersedes it (rule D-OWNER); what the window had not reported handled then is
// sent again on the next connection. Reports are taken only for what c holds. Their sender is the
// application's own window: the stream is a Wails stream, with no local port (ADR 0009).
func (o *opener) Serve(c openConn) {
	g := o.attach(c)
	defer o.detach(c, g)
	ctx := c.Context()
	frames := receive(ctx, c)
	for {
		changed := o.queue.changed()
		if err := o.sendQueued(c, g); err != nil {
			return
		}
		select {
		case b, ok := <-frames:
			if !ok {
				return
			}
			o.report(g, b)
		case <-changed:
		case <-ctx.Done():
			return
		}
	}
}

// attach makes c the current connection, a new generation, and closes the one it supersedes.
func (o *opener) attach(c openConn) uint64 {
	o.mu.Lock()
	g := o.queue.connect()
	prev := o.cur
	o.cur = c
	o.mu.Unlock()
	if prev != nil {
		if err := prev.Close(); err != nil {
			o.log.Debug("open stream: closing the superseded connection", "err", err)
		}
	}
	return g
}

// detach is the end of c, of generation g: what it held goes back to the queue.
func (o *opener) detach(c openConn, g uint64) {
	o.mu.Lock()
	if o.cur == c {
		o.cur = nil
	}
	o.mu.Unlock()
	o.queue.release(g)
}

// receive reads the frames of c until it closes.
func receive(ctx context.Context, c openConn) <-chan []byte {
	frames := make(chan []byte)
	go func() {
		defer close(frames)
		for {
			b, err := c.Receive()
			if err != nil {
				return
			}
			select {
			case frames <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	return frames
}

// sendQueued sends on c, of generation g, the requests it may take now; an error is the end of c.
func (o *opener) sendQueued(c openConn, g uint64) error {
	for {
		e, ok := o.queue.take(g)
		if !ok {
			return nil
		}
		env := o.env
		env.Cwd = e.req.Cwd
		b, err := json.Marshal(openFrame{ID: e.seq, Outcome: launch.Plan(launch.Resolve(e.req.Args, int(e.req.Dropped), env))})
		if err != nil {
			o.log.Error("open request", "err", err)
			o.queue.handled(g, e.seq)
			continue
		}
		o.raise(e.req.Activation)
		if err := c.Send(b); err != nil {
			return err
		}
	}
}

// report takes a frame of the window of generation g: {"handled":id} frees that request if g holds it.
func (o *opener) report(g uint64, b []byte) {
	var a struct {
		Handled uint64 `json:"handled"`
	}
	if len(b) > maxHandledFrame || json.Unmarshal(b, &a) != nil || a.Handled == 0 {
		return
	}
	if !o.queue.handled(g, a.Handled) {
		o.log.Debug("open stream: report of a request this connection does not hold", "id", a.Handled)
	}
}
