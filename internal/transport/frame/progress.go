package frame

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// progressChunk is how many payload bytes are written between two re-arms of
// the deadline: the granularity at which progress is noticed.
const progressChunk = 32 << 10

const (
	// MaxOneWayDelay is the one-way network delay the sync/1 timers are sized
	// for (send-path design, SP1-1): 5 s, a relayed path on a poor link.
	MaxOneWayDelay = 5 * time.Second
	// WriteIdle is how long one Write (a 32 KiB fragment) may go without
	// progress on sync/1 (SP1-1 of docs/plans/2026-11-09-send-path-design.md):
	// the window update quantum U = 128 KiB read at the floor takes U/F = 64 s,
	// plus 2 * MaxOneWayDelay is 74 s, times 1.2.
	WriteIdle = 90 * time.Second
	// FloorRate is the slowest sustained rate, in bytes per second, at which a
	// write is still worth waiting for (SP1-2, folded from WB-2): 2 KiB/s.
	FloorRate = 2 << 10
)

// DeadlineWriter is a stream with write deadlines, such as a libp2p stream or
// a [net.Conn].
type DeadlineWriter interface {
	io.Writer
	SetWriteDeadline(t time.Time) error
}

// WriteProgress writes payload to w as one frame, like [Write], but bounds the
// write by progress rather than by size: it fails only if no chunk was
// written for idle, so a large frame on a slow link still completes as long
// as bytes keep flowing. A receiver that drips just inside idle is still cut
// off: the whole frame gets idle plus len(payload)/floorRate (bytes per
// second; callers pass [FloorRate]). Canceling ctx interrupts a write in progress. It
// returns [transport.ErrFrameTooLarge] if payload exceeds [transport.MaxFrame].
// Like [Write], it must not run concurrently on the same w.
func WriteProgress(ctx context.Context, w DeadlineWriter, idle time.Duration, floorRate int, payload []byte) error {
	if len(payload) > transport.MaxFrame {
		return transport.ErrFrameTooLarge
	}
	// The deadline is updated under mu, so that re-arming it can never undo
	// the interruption a cancellation asked for.
	last := time.Now().Add(idle + time.Duration(len(payload))*time.Second/time.Duration(max(floorRate, 1)))
	var mu sync.Mutex
	arm := func() error {
		mu.Lock()
		defer mu.Unlock()
		// ctx.Err covers a cancellation whose callback has not run yet, and a
		// writer that ignores deadlines: the context is checked before each chunk.
		if err := ctx.Err(); err != nil {
			return err
		}
		return w.SetWriteDeadline(minTime(time.Now().Add(idle), last))
	}
	fin := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(fin)
		mu.Lock()
		defer mu.Unlock()
		_ = w.SetWriteDeadline(time.Now())
	})
	defer func() {
		if !stop() { // the callback started: let it finish, it must not outlive us
			<-fin
		}
	}()

	if err := arm(); err != nil {
		return fmt.Errorf("frame: arm write deadline: %w", err)
	}
	if len(payload) <= smallFrame {
		return Write(w, payload)
	}
	var hdr [headerLen]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload))) // #nosec G115 -- bounded by MaxFrame above.
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("frame: write frame header: %w", err)
	}
	for rest := payload; len(rest) > 0; {
		if err := arm(); err != nil {
			return fmt.Errorf("frame: arm write deadline: %w", err)
		}
		n := min(len(rest), progressChunk)
		if _, err := w.Write(rest[:n]); err != nil {
			return fmt.Errorf("frame: write frame payload: %w", err)
		}
		rest = rest[n:]
	}
	return nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
