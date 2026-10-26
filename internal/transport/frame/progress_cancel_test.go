package frame_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// countingWriter ignores deadlines, like a writer that cannot be interrupted.
type countingWriter struct {
	writes  int
	onWrite func()
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.onWrite != nil {
		w.onWrite()
	}
	return len(p), nil
}

func (*countingWriter) SetWriteDeadline(time.Time) error { return nil }

// TestWriteProgressCanceledWritesNothing: a context that is already canceled
// stops the write before any byte, even if the writer ignores deadlines.
func TestWriteProgressCanceledWritesNothing(t *testing.T) {
	for _, size := range []int{10, 1 << 20} {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var w countingWriter
		err := frame.WriteProgress(ctx, &w, time.Minute, frame.MinRate, make([]byte, size))
		if !errors.Is(err, context.Canceled) || w.writes != 0 {
			t.Errorf("WriteProgress(canceled, %d bytes) = %v after %d writes, want context.Canceled and 0", size, err, w.writes)
		}
	}
}

// TestWriteProgressCancelStopsBetweenChunks: a cancellation during a chunk
// stops the next one, even if the writer ignores deadlines.
func TestWriteProgressCancelStopsBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	w := countingWriter{onWrite: cancel}
	err := frame.WriteProgress(ctx, &w, time.Minute, frame.MinRate, make([]byte, 1<<20))
	if !errors.Is(err, context.Canceled) || w.writes != 1 {
		t.Errorf("WriteProgress(canceled while writing) = %v after %d writes, want context.Canceled after the header", err, w.writes)
	}
}
