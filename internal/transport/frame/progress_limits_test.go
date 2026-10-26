package frame_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// TestWriteProgressOverallDeadline: a receiver that takes a chunk just inside
// the idle limit, forever, is cut off by the deadline for the whole frame.
func TestWriteProgressOverallDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c1, c2 := net.Pipe()
		var got bytes.Buffer
		done := make(chan struct{})
		go func() { defer close(done); drainSlowly(c2, 32<<10, 4*time.Second, &got) }() // idle is 5 s: always "progress"
		defer func() { _ = c1.Close(); _ = c2.Close(); <-done }()
		const size, minRate = 1 << 20, 256 << 10 // 4 s allowed, plus idle
		start := time.Now()
		err := frame.WriteProgress(t.Context(), c1, 5*time.Second, minRate, make([]byte, size))
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("WriteProgress(drip) = %v, want a deadline error", err)
		}
		if d := time.Since(start); d > 10*time.Second {
			t.Errorf("gave up after %v, want <= idle + size/minRate (9s)", d)
		}
	})
}

// lateWriter cancels the context during its first Write and fails it, while
// the cancellation callback (SetWriteDeadline) is slow.
type lateWriter struct {
	cancel   context.CancelFunc
	returned *atomic.Bool
	late     *atomic.Bool
	calls    atomic.Int32
}

func (w *lateWriter) Write([]byte) (int, error) { w.cancel(); return 0, io.ErrClosedPipe }

func (w *lateWriter) SetWriteDeadline(time.Time) error {
	if w.calls.Add(1) > 1 { // the cancellation callback, not the arming
		time.Sleep(time.Second)
		if w.returned.Load() {
			w.late.Store(true)
		}
	}
	return nil
}

// TestWriteProgressWaitsForCancelCallback: when WriteProgress returns, its
// cancellation callback is over, so it cannot expire the caller's next write.
func TestWriteProgressWaitsForCancelCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var returned, late atomic.Bool
		w := &lateWriter{cancel: cancel, returned: &returned, late: &late}
		_ = frame.WriteProgress(ctx, w, time.Minute, frame.MinRate, make([]byte, 1<<20))
		returned.Store(true)
		time.Sleep(2 * time.Second)
		if late.Load() {
			t.Error("the cancellation callback ran after WriteProgress returned")
		}
	})
}
