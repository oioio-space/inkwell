package frame_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// drainSlowly reads r one chunk per tick into out, like a slow link, until r
// ends.
func drainSlowly(r io.Reader, chunk int, tick time.Duration, out *bytes.Buffer) {
	buf := make([]byte, chunk)
	for {
		time.Sleep(tick)
		n, err := r.Read(buf)
		out.Write(buf[:n])
		if err != nil {
			return
		}
	}
}

// TestWriteProgressSlowLinkSucceeds: a write far longer than idle completes as
// long as bytes keep flowing (PERF-05: the 6 MB message on a 1 Mbit/s link).
func TestWriteProgressSlowLinkSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c1, c2 := net.Pipe()
		var got bytes.Buffer
		done := make(chan struct{})
		go func() { defer close(done); drainSlowly(c2, 32<<10, time.Second, &got) }()

		payload := bytes.Repeat([]byte("x"), 1<<20) // about 32 s at 32 KiB/s; idle is 5 s
		if err := frame.WriteProgress(t.Context(), c1, 5*time.Second, payload); err != nil {
			t.Fatalf("WriteProgress(slow link) = %v, want nil", err)
		}
		_ = c1.Close()
		<-done
		back, err := frame.Read(&got, len(payload))
		if err != nil || !bytes.Equal(back, payload) {
			t.Fatalf("Read back = %d bytes, %v; want the payload", len(back), err)
		}
	})
}

// TestWriteProgressStall: no progress for idle fails, after exactly idle.
func TestWriteProgressStall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer func() { _ = c2.Close() }()
		start := time.Now()
		err := frame.WriteProgress(t.Context(), c1, 5*time.Second, make([]byte, 1<<20))
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("WriteProgress(stalled) = %v, want a deadline error", err)
		}
		if d := time.Since(start); d != 5*time.Second {
			t.Errorf("gave up after %v, want 5s", d)
		}
		_ = c1.Close()
	})
}

// TestWriteProgressCancel: canceling ctx interrupts a stalled write at once,
// and the cancellation is not lost by re-arming the deadline.
func TestWriteProgressCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer func() { _ = c2.Close() }()
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		start := time.Now()
		err := frame.WriteProgress(ctx, c1, time.Minute, make([]byte, 1<<20))
		if err == nil {
			t.Fatal("WriteProgress(canceled) = nil, want an error")
		}
		if d := time.Since(start); d != time.Second {
			t.Errorf("returned after %v, want 1s", d)
		}
		_ = c1.Close()
	})
}

// deadlineBuffer is a writer that accepts deadlines and keeps the bytes.
type deadlineBuffer struct{ bytes.Buffer }

func (*deadlineBuffer) SetWriteDeadline(time.Time) error { return nil }

func TestWriteProgressSmall(t *testing.T) {
	var b deadlineBuffer
	if err := frame.WriteProgress(t.Context(), &b, time.Second, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if want := []byte{0, 0, 0, 2, 'h', 'i'}; !bytes.Equal(b.Bytes(), want) {
		t.Errorf("wire = %v, want %v", b.Bytes(), want)
	}
}
