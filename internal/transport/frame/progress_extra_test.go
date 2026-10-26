package frame_test

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// recorder is a DeadlineWriter that keeps the bytes, the size of every Write
// and the deadline in force at each Write.
type recorder struct {
	buf       bytes.Buffer
	sizes     []int
	deadlines []time.Time
	armed     time.Time
	armErr    error
	armCalls  int
}

func (r *recorder) Write(p []byte) (int, error) {
	r.sizes = append(r.sizes, len(p))
	r.deadlines = append(r.deadlines, r.armed)
	return r.buf.Write(p)
}

func (r *recorder) SetWriteDeadline(t time.Time) error {
	r.armCalls++
	r.armed = t
	return r.armErr
}

func TestWriteProgressTooLarge(t *testing.T) {
	var ok recorder
	if err := frame.WriteProgress(t.Context(), &ok, time.Second, frame.MinRate, make([]byte, transport.MaxFrame)); err != nil {
		t.Errorf("WriteProgress(MaxFrame) = %v, want nil", err)
	}
	if ok.buf.Len() != 4+transport.MaxFrame {
		t.Errorf("WriteProgress(MaxFrame) wrote %d bytes, want %d", ok.buf.Len(), 4+transport.MaxFrame)
	}
	var r recorder
	err := frame.WriteProgress(t.Context(), &r, time.Second, frame.MinRate, make([]byte, transport.MaxFrame+1))
	if !errors.Is(err, transport.ErrFrameTooLarge) || r.armCalls != 0 || r.buf.Len() != 0 {
		t.Errorf("WriteProgress(MaxFrame+1) = %v, armed %d times, wrote %d bytes; want ErrFrameTooLarge and no side effect", err, r.armCalls, r.buf.Len())
	}
}

// TestWriteProgressWire: the bytes are those of frame.Write, for the small
// (one write) and large (header then 32 KiB chunks) paths, and read back.
func TestWriteProgressWire(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for _, tc := range []struct {
			size  int
			sizes []int
		}{
			{0, []int{4}},
			{64 << 10, []int{4 + 64<<10}},
			{64<<10 + 1, []int{4, 32 << 10, 32 << 10, 1}},
			{100 << 10, []int{4, 32 << 10, 32 << 10, 32 << 10, 4 << 10}},
		} {
			payload := make([]byte, tc.size)
			for i := range payload {
				payload[i] = byte(i * 7)
			}
			var r recorder
			if err := frame.WriteProgress(t.Context(), &r, time.Second, frame.MinRate, payload); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(r.sizes, tc.sizes) {
				t.Errorf("size %d: write sizes = %v, want %v", tc.size, r.sizes, tc.sizes)
			}
			var want bytes.Buffer
			if err := frame.Write(&want, payload); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(r.buf.Bytes(), want.Bytes()) {
				t.Errorf("size %d: wire differs from frame.Write", tc.size)
			}
		}
	})
}

// TestWriteProgressDeadlines: each write runs under now+idle, capped by the
// overall limit idle + size/minRate.
func TestWriteProgressDeadlines(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const idle, minRate = 10 * time.Second, 1 << 10
		start := time.Now()
		var r recorder
		if err := frame.WriteProgress(t.Context(), &r, idle, minRate, make([]byte, 100<<10)); err != nil {
			t.Fatal(err)
		}
		// 100 KiB at 1 KiB/s allows 110 s overall; nothing sleeps, so every
		// write is bound by now+idle.
		for i, d := range r.deadlines {
			if want := start.Add(idle); !d.Equal(want) {
				t.Errorf("write %d deadline = %v after start, want %v", i, d.Sub(start), idle)
			}
		}

		// A frame just over the one-write limit, with a generous idle: still
		// bounded by now+idle, and never beyond the overall limit.
		var r2 recorder
		if err := frame.WriteProgress(t.Context(), &r2, time.Hour, 1<<20, make([]byte, 70<<10)); err != nil {
			t.Fatal(err)
		}
		wantCap := start.Add(time.Hour + time.Duration(70<<10)*time.Second/(1<<20))
		wantIdle := start.Add(time.Hour)
		for i, d := range r2.deadlines {
			if !d.Equal(wantIdle) {
				t.Errorf("write %d deadline = %v, want now+idle %v", i, d, wantIdle)
			}
			if d.After(wantCap) {
				t.Errorf("write %d deadline %v beyond the overall cap %v", i, d, wantCap)
			}
		}
	})
}

// TestWriteProgressOverallCapBinds: when time advances between chunks, the
// deadline stops following now+idle and sticks to the overall limit.
func TestWriteProgressOverallCapBinds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		w := &sleepyRecorder{}
		const idle, minRate = 10 * time.Second, 32 << 10 // 100 KiB: cap = start + 10s + ~3.1s
		if err := frame.WriteProgress(t.Context(), w, idle, minRate, make([]byte, 100<<10)); err != nil {
			t.Fatal(err)
		}
		limit := start.Add(idle + time.Duration(100<<10)*time.Second/minRate)
		var capped int
		for i, d := range w.deadlines {
			if d.After(limit) {
				t.Errorf("write %d deadline %v after the overall limit %v", i, d.Sub(start), limit.Sub(start))
			}
			if d.Equal(limit) {
				capped++
			}
		}
		if capped == 0 {
			t.Errorf("deadlines %v never reached the overall limit %v; the cap is not applied", w.deadlines, limit)
		}
	})
}

// sleepyRecorder advances the (fake) clock by 4 s per write, as a slow peer.
type sleepyRecorder struct{ recorder }

func (w *sleepyRecorder) Write(p []byte) (int, error) {
	time.Sleep(4 * time.Second)
	return w.recorder.Write(p)
}

func TestWriteProgressMinRateZero(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var r recorder
		// A zero rate must not divide by zero; it means one byte per second.
		if err := frame.WriteProgress(t.Context(), &r, time.Second, 0, []byte("hi")); err != nil {
			t.Fatalf("WriteProgress(minRate 0) = %v", err)
		}
	})
}

// TestWriteProgressArmError: a failing SetWriteDeadline stops the write before
// any byte goes out, and the cause is kept.
func TestWriteProgressArmError(t *testing.T) {
	r := recorder{armErr: io.ErrClosedPipe}
	err := frame.WriteProgress(t.Context(), &r, time.Second, frame.MinRate, []byte("hi"))
	if !errors.Is(err, io.ErrClosedPipe) || r.buf.Len() != 0 {
		t.Errorf("WriteProgress(arm fails) = %v, wrote %d bytes; want ErrClosedPipe and nothing written", err, r.buf.Len())
	}
}

// failAt fails the nth Write.
type failAt struct {
	recorder
	n int
}

func (w *failAt) Write(p []byte) (int, error) {
	if len(w.sizes) == w.n {
		return 0, io.ErrClosedPipe
	}
	return w.recorder.Write(p)
}

func TestWriteProgressWriteErrors(t *testing.T) {
	for n, stage := range []string{"header", "first chunk", "second chunk"} {
		w := &failAt{n: n}
		err := frame.WriteProgress(t.Context(), w, time.Second, frame.MinRate, make([]byte, 100<<10))
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("fail at %s: WriteProgress = %v, want ErrClosedPipe", stage, err)
		}
	}
	w := &failAt{n: 0}
	if err := frame.WriteProgress(t.Context(), w, time.Second, frame.MinRate, []byte("hi")); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("small frame: WriteProgress = %v, want ErrClosedPipe", err)
	}
}

func TestBudgetTryAcquire(t *testing.T) {
	b := frame.NewBudget(10)
	if !b.TryAcquire(0) {
		t.Error("TryAcquire(0) = false")
	}
	if !b.TryAcquire(10) {
		t.Error("TryAcquire(10) of 10 = false, want true")
	}
	if b.TryAcquire(1) {
		t.Error("TryAcquire(1) on an exhausted budget = true")
	}
	b.Release(3)
	if b.TryAcquire(4) {
		t.Error("TryAcquire(4) with 3 free = true")
	}
	if !b.TryAcquire(3) {
		t.Error("TryAcquire(3) with 3 free = false")
	}
	b.Release(5)
	b.Close()
	if b.TryAcquire(1) {
		t.Error("TryAcquire on a closed budget with free bytes = true")
	}
}
