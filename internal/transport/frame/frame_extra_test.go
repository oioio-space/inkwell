package frame_test

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"slices"
	"testing"
	"testing/iotest"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

// TestWireFormat pins the on-the-wire layout: a 4-byte big-endian length then
// the payload, for both the one-write and the two-write paths.
func TestWireFormat(t *testing.T) {
	for _, size := range []int{0, 3, 64 << 10, 64<<10 + 1, 0x01_02_03} {
		payload := bytes.Repeat([]byte{7}, size)
		var buf bytes.Buffer
		if err := frame.Write(&buf, payload); err != nil {
			t.Fatalf("Write(%d bytes) = %v", size, err)
		}
		want := append([]byte{byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)}, payload...)
		if !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("Write(%d bytes) wire = %x..., want %x...", size, buf.Bytes()[:4], want[:4])
		}
	}
}

// callRecorder records the size of each Write call.
type callRecorder struct{ calls []int }

func (w *callRecorder) Write(p []byte) (int, error) {
	w.calls = append(w.calls, len(p))
	return len(p), nil
}

// TestWriteCallCount: small frames go out in one Write (so concurrent users of
// a shared stream cannot split header from payload), large ones in two (no copy).
func TestWriteCallCount(t *testing.T) {
	for _, tc := range []struct {
		size int
		want []int
	}{
		{0, []int{4}},
		{64 << 10, []int{4 + 64<<10}},
		{64<<10 + 1, []int{4, 64<<10 + 1}},
	} {
		var w callRecorder
		if err := frame.Write(&w, make([]byte, tc.size)); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(w.calls, tc.want) {
			t.Errorf("Write(%d bytes) calls = %v, want %v", tc.size, w.calls, tc.want)
		}
	}
}

func TestWriteMaxFrameBoundary(t *testing.T) {
	if err := frame.Write(io.Discard, make([]byte, transport.MaxFrame)); err != nil {
		t.Errorf("Write(MaxFrame) = %v, want nil", err)
	}
	if err := frame.Write(io.Discard, make([]byte, transport.MaxFrame+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Write(MaxFrame+1) = %v, want ErrFrameTooLarge", err)
	}
}

// TestWriteErrorWraps: a failed Write adds context and keeps the cause.
func TestWriteErrorWraps(t *testing.T) {
	for _, after := range []int{0, 1} {
		err := frame.Write(&failWriter{after: after}, make([]byte, 100<<10))
		if !errors.Is(err, io.ErrClosedPipe) || err.Error() == io.ErrClosedPipe.Error() {
			t.Errorf("Write(after %d) = %v, want a wrapped ErrClosedPipe", after, err)
		}
	}
}

// TestReadLimitBoundary: a frame of exactly limit bytes is accepted, limit+1
// is refused having consumed only the header.
func TestReadLimitBoundary(t *testing.T) {
	for _, limit := range []int{0, 1, 10, 64 << 10, 64<<10 + 1} {
		var ok bytes.Buffer
		if err := frame.Write(&ok, make([]byte, limit)); err != nil {
			t.Fatal(err)
		}
		got, err := frame.Read(&ok, limit)
		if err != nil || len(got) != limit {
			t.Errorf("Read(size == limit == %d) = %d bytes, %v; want accepted", limit, len(got), err)
		}

		var over bytes.Buffer
		if err := frame.Write(&over, make([]byte, limit+1)); err != nil {
			t.Fatal(err)
		}
		before := over.Len()
		got, err = frame.Read(&over, limit)
		if !errors.Is(err, transport.ErrFrameTooLarge) || got != nil {
			t.Errorf("Read(size %d, limit %d) = %d bytes, %v; want ErrFrameTooLarge", limit+1, limit, len(got), err)
		}
		if consumed := before - over.Len(); consumed != 4 {
			t.Errorf("Read(size %d, limit %d) consumed %d bytes, want only the 4-byte header", limit+1, limit, consumed)
		}
	}
}

func TestReadNegativeLimit(t *testing.T) {
	// Even an empty frame is refused: a negative limit allows nothing.
	if _, err := frame.Read(bytes.NewReader([]byte{0, 0, 0, 0}), -1); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Read(limit -1) = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadMaxUint32Length(t *testing.T) {
	_, err := frame.Read(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff}), transport.MaxFrame)
	if !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Read(len 0xFFFFFFFF) = %v, want ErrFrameTooLarge", err)
	}
}

// TestReadEmptyFrame: a zero-length frame is a valid, non-nil empty payload.
func TestReadEmptyFrame(t *testing.T) {
	got, err := frame.Read(bytes.NewReader([]byte{0, 0, 0, 0}), 10)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("Read(empty frame) = %v (nil=%t), %v; want empty non-nil, nil", got, got == nil, err)
	}
}

// TestReadDoesNotOverread: only one frame is consumed per call.
func TestReadDoesNotOverread(t *testing.T) {
	var buf bytes.Buffer
	for _, p := range []string{"one", "", "three"} {
		if err := frame.Write(&buf, []byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"one", "", "three"} {
		got, err := frame.Read(&buf, 100)
		if err != nil || string(got) != want {
			t.Fatalf("Read = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := frame.Read(&buf, 100); !errors.Is(err, io.EOF) {
		t.Errorf("Read at end = %v, want io.EOF", err)
	}
}

// TestReadSlowStream: short reads across the growth chunk boundaries still
// give the exact payload.
func TestReadSlowStream(t *testing.T) {
	for _, size := range []int{1, 64<<10 - 1, 64 << 10, 64<<10 + 1, 3*64<<10 + 5} {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i * 31)
		}
		var buf bytes.Buffer
		if err := frame.Write(&buf, payload); err != nil {
			t.Fatal(err)
		}
		got, err := frame.Read(iotest.HalfReader(&buf), size)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("Read(%d bytes, half reader) = %d bytes, %v", size, len(got), err)
		}
	}
}

// TestReadTruncatedAcrossChunks: a stream that dies inside a large payload,
// after the first chunk, is ErrUnexpectedEOF and returns nothing.
func TestReadTruncatedAcrossChunks(t *testing.T) {
	var buf bytes.Buffer
	if err := frame.Write(&buf, make([]byte, 200<<10)); err != nil {
		t.Fatal(err)
	}
	cut := buf.Bytes()[:4+100<<10]
	got, err := frame.Read(bytes.NewReader(cut), 200<<10)
	if !errors.Is(err, io.ErrUnexpectedEOF) || got != nil {
		t.Errorf("Read(truncated) = %d bytes, %v; want nil, ErrUnexpectedEOF", len(got), err)
	}
}

// TestReadErrorsKeepCause: a non-EOF failure, in the header or the payload,
// is wrapped, never turned into EOF.
func TestReadErrorsKeepCause(t *testing.T) {
	boom := errors.New("boom")
	_, err := frame.Read(iotest.ErrReader(boom), 10)
	if !errors.Is(err, boom) || errors.Is(err, io.EOF) {
		t.Errorf("Read(header error) = %v, want boom", err)
	}
	r := io.MultiReader(bytes.NewReader([]byte{0, 0, 0, 5, 'a'}), iotest.ErrReader(boom))
	_, err = frame.Read(r, 10)
	if !errors.Is(err, boom) || errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("Read(payload error) = %v, want boom", err)
	}
}

// TestReadDoesNotPreallocate: a header claiming a huge size on a stream that
// carries nothing must not cost the claimed size.
func TestReadDoesNotPreallocate(t *testing.T) {
	hdr := []byte{0x03, 0xff, 0xff, 0xff} // ~64 MiB claimed, nothing sent
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 20 {
		_, _ = frame.Read(bytes.NewReader(hdr), 64<<20)
	}
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > 20<<20 {
		t.Errorf("20 reads of a lying header allocated %d bytes, want well under the claimed 1.3 GiB", got)
	}
}

func TestReadReserve(t *testing.T) {
	var buf bytes.Buffer
	if err := frame.Write(&buf, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	var sizes []int
	got, err := frame.ReadReserve(&buf, 10, func(n int) error { sizes = append(sizes, n); return nil })
	if err != nil || string(got) != "hello" {
		t.Fatalf("ReadReserve = %q, %v", got, err)
	}
	if !slices.Equal(sizes, []int{5}) {
		t.Errorf("reserve calls = %v, want [5]", sizes)
	}
}

func TestReadReserveRefusal(t *testing.T) {
	denied := errors.New("budget closed")
	var buf bytes.Buffer
	if err := frame.Write(&buf, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, err := frame.ReadReserve(&buf, 10, func(int) error { return denied })
	if !errors.Is(err, denied) || got != nil {
		t.Errorf("ReadReserve = %q, %v; want nil, the reserve error", got, err)
	}
	if buf.Len() != 5 {
		t.Errorf("payload consumed despite refused reservation: %d bytes left, want 5", buf.Len())
	}
}

func TestReadReserveNotCalled(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []byte
		want error
	}{
		"oversize": {[]byte{0, 0, 0, 9}, transport.ErrFrameTooLarge},
		"eof":      {nil, io.EOF},
		"short":    {[]byte{0, 0}, io.ErrUnexpectedEOF},
	} {
		called := false
		_, err := frame.ReadReserve(bytes.NewReader(tc.in), 8, func(int) error { called = true; return nil })
		if !errors.Is(err, tc.want) || called {
			t.Errorf("%s: ReadReserve = %v, reserve called = %t; want %v, false", name, err, called, tc.want)
		}
	}
}
