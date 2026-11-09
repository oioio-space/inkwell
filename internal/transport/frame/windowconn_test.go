package frame_test

import (
	"errors"
	"os"
	"time"
)

var errNoDeadline = errors.New("windowConn: reader stopped and no write deadline is armed")

// windowConn is a [frame.DeadlineWriter] that emulates one yamux stream in
// virtual time (block SP1, send-path model): the writer may have at most
// window bytes beyond what the reader has released. The reader consumes at
// rate bytes per second what reaches it delay after the writer handed it
// over, and once it has consumed window/2 since its last update it sends one,
// which reaches the writer delay later. Write blocks while the window is
// closed and honours SetWriteDeadline. It runs without goroutines: the
// consumption schedule is computed when bytes are accepted, so it is exact
// under testing/synctest.
type windowConn struct {
	window    int
	rate      int           // reader's bytes per second
	delay     time.Duration // one-way delay
	stopAfter int           // the reader consumes nothing beyond this many bytes; 0 means no limit

	deadline time.Time
	sent     int       // bytes accepted so far
	granted  int       // bytes released by the updates that have reached the writer
	pending  []grant   // updates in flight, in time order
	free     time.Time // when the reader finishes what it was handed
	consumed int       // bytes the reader will have consumed once it caught up
	// blocked is the duration of every Write call that had to wait.
	blocked []time.Duration
}

// grant is a window update that reaches the writer at at.
type grant struct {
	at time.Time
	n  int
}

func newWindowConn(window, rate int, delay time.Duration) *windowConn {
	return &windowConn{window: window, rate: rate, delay: delay, granted: window}
}

func (c *windowConn) SetWriteDeadline(t time.Time) error { c.deadline = t; return nil }

// settle credits the updates that have reached the writer by now.
func (c *windowConn) settle() {
	for len(c.pending) > 0 && !c.pending[0].at.After(time.Now()) {
		c.granted += c.pending[0].n
		c.pending = c.pending[1:]
	}
}

func (c *windowConn) Write(p []byte) (int, error) {
	start, n := time.Now(), 0
	defer func() {
		if d := time.Since(start); d > 0 {
			c.blocked = append(c.blocked, d)
		}
	}()
	for n < len(p) {
		c.settle()
		avail := c.granted - c.sent
		if avail == 0 {
			if len(c.pending) == 0 {
				// The reader has stopped: only the deadline ends the wait.
				if c.deadline.IsZero() {
					return n, errNoDeadline
				}
				time.Sleep(time.Until(c.deadline))
				return n, os.ErrDeadlineExceeded
			}
			wake := c.pending[0].at
			if !c.deadline.IsZero() && c.deadline.Before(wake) {
				time.Sleep(time.Until(c.deadline))
				return n, os.ErrDeadlineExceeded
			}
			time.Sleep(time.Until(wake))
			continue
		}
		if !c.deadline.IsZero() && !time.Now().Before(c.deadline) {
			return n, os.ErrDeadlineExceeded
		}
		k := min(avail, len(p)-n)
		c.accept(k)
		n += k
	}
	return n, nil
}

// accept hands k bytes to the reader and schedules its consumption and the
// window updates it triggers.
func (c *windowConn) accept(k int) {
	c.sent += k
	begin := time.Now().Add(c.delay)
	if c.free.After(begin) {
		begin = c.free
	}
	got := k
	if c.stopAfter > 0 {
		got = max(0, min(k, c.stopAfter-c.consumed))
	}
	half := c.window / 2
	for t := (c.consumed/half + 1) * half; t <= c.consumed+got; t += half {
		off := time.Duration(t-c.consumed) * time.Second / time.Duration(c.rate)
		c.pending = append(c.pending, grant{at: begin.Add(off + c.delay), n: half})
	}
	c.free = begin.Add(time.Duration(got) * time.Second / time.Duration(c.rate))
	c.consumed += got
}
