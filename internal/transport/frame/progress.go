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

// DeadlineWriter is a stream with write deadlines, such as a libp2p stream or
// a [net.Conn].
type DeadlineWriter interface {
	io.Writer
	SetWriteDeadline(t time.Time) error
}

// WriteProgress writes payload to w as one frame, like [Write], but bounds the
// write by progress rather than by size: it fails only if no chunk was
// written for idle, so a large frame on a slow link still completes as long
// as bytes keep flowing. Canceling ctx interrupts a write in progress. It
// returns [transport.ErrFrameTooLarge] if payload exceeds [transport.MaxFrame].
// Like [Write], it must not run concurrently on the same w.
func WriteProgress(ctx context.Context, w DeadlineWriter, idle time.Duration, payload []byte) error {
	if len(payload) > transport.MaxFrame {
		return transport.ErrFrameTooLarge
	}
	// canceled and the deadline are updated under mu, so that re-arming the
	// deadline can never undo the interruption a cancellation asked for.
	var (
		mu       sync.Mutex
		canceled bool
	)
	arm := func() error {
		mu.Lock()
		defer mu.Unlock()
		if canceled {
			return ctx.Err()
		}
		return w.SetWriteDeadline(time.Now().Add(idle))
	}
	stop := context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		canceled = true
		_ = w.SetWriteDeadline(time.Now())
	})
	defer stop()

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
