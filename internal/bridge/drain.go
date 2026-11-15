package bridge

import (
	"context"
	"io"
	"maps"
	"slices"
	"sync"
	"time"
)

// ReasonLeaving is the payload of the [StatusError] answer to a store request read once the
// session is leaving for a project change: the request was refused, nothing was written.
const ReasonLeaving = "leaving"

// DrainTimeout bounds the wait of [Drain.Drain] for the requests in flight (plan
// docs/plans/2026-11-14-pc1-project-change-by-restart.md, rule N-16).
const DrainTimeout = 5 * time.Second

// Drain ends the bridge before a project change (rule N-16): shared by the store and net servers
// (see [StoreServer.UseDrain] and [Router.UseDrain]), it marks the session « leaving », waits
// until every request in flight has been handled and answered, then closes every open stream for
// good. From then on a request read from the page is refused and a new stream is closed at once,
// until the process ends. Create one with [NewDrain]; a nil Drain never drains.
type Drain struct {
	mu       sync.Mutex
	leaving  bool
	conns    map[Conn]struct{}
	handlers sync.WaitGroup // the requests taken by a handler, until answered
	streams  sync.WaitGroup // the streams being served
}

// NewDrain returns a drain of a session that is not leaving.
func NewDrain() *Drain { return &Drain{conns: map[Conn]struct{}{}} }

// Begin registers a request taken by a handler, which calls [Drain.End] once it is answered. It
// reports false once the session is leaving: the request must then be refused, not handled, and
// End is not called.
func (d *Drain) Begin() bool {
	if d == nil {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.leaving {
		return false
	}
	d.handlers.Add(1)
	return true
}

// End ends the request a successful [Drain.Begin] registered: once per Begin.
func (d *Drain) End() {
	if d != nil {
		d.handlers.Done()
	}
}

// attach registers a stream being served; detach is called when its Serve returns. It reports
// false, and closes c, once the session is leaving.
func (d *Drain) attach(c Conn) (detach func(), ok bool) {
	if d == nil {
		return func() {}, true
	}
	d.mu.Lock()
	if d.leaving {
		d.mu.Unlock()
		closeConn(c)
		return nil, false
	}
	d.conns[c] = struct{}{}
	d.streams.Add(1)
	d.mu.Unlock()
	return func() {
		d.mu.Lock()
		delete(d.conns, c)
		d.mu.Unlock()
		d.streams.Done()
	}, true
}

// Drain marks the session leaving, waits for the requests in flight, closes every open stream and
// waits until their Serve returned. It fails with ctx's error when ctx ends first: the session
// stays leaving and its streams closed all the same. A stream is closed through its Close method
// (the Wails stream has one); one without it ends only with its context.
func (d *Drain) Drain(ctx context.Context) error {
	d.mu.Lock()
	d.leaving = true
	d.mu.Unlock()
	err := waitFor(ctx, &d.handlers)
	d.mu.Lock()
	conns := slices.Collect(maps.Keys(d.conns))
	d.mu.Unlock()
	for _, c := range conns {
		closeConn(c)
	}
	if err != nil {
		return err
	}
	return waitFor(ctx, &d.streams)
}

// waitFor waits for wg or for the end of ctx, whichever comes first.
func waitFor(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// closeConn closes c when it can be closed.
func closeConn(c Conn) {
	if cl, ok := c.(io.Closer); ok {
		_ = cl.Close()
	}
}
