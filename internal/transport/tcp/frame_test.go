package tcp_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/tcp"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestReadFrameRejectsOversize(t *testing.T) {
	// Announces 1 GiB; ReadFrame must refuse from the header alone.
	r := bytes.NewReader([]byte{0x40, 0, 0, 0})
	got, err := tcp.ReadFrame(r, 1024)
	if !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("ReadFrame(1GiB header, limit 1024) = %v, %v; want ErrFrameTooLarge", got, err)
	}
}

func TestReadFrameTruncated(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"empty", nil, io.EOF},
		{"short header", []byte{0, 0}, io.ErrUnexpectedEOF},
		{"short payload", []byte{0, 0, 0, 5, 'a', 'b'}, io.ErrUnexpectedEOF},
		{"no payload", []byte{0, 0, 0, 5}, io.ErrUnexpectedEOF},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tcp.ReadFrame(bytes.NewReader(tc.in), 100); !errors.Is(err, tc.want) {
				t.Errorf("ReadFrame(%x) error = %v, want %v", tc.in, err, tc.want)
			}
		})
	}
}

func TestFrameRoundTripSizes(t *testing.T) {
	for _, size := range []int{0, 1, 1000, 64 << 10, 64<<10 + 1, 300 << 10} {
		payload := bytes.Repeat([]byte{0xAB}, size)
		var buf bytes.Buffer
		if err := tcp.WriteFrame(&buf, payload); err != nil {
			t.Fatalf("WriteFrame(%d bytes): %v", size, err)
		}
		got, err := tcp.ReadFrame(&buf, size)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("ReadFrame(%d bytes) = %d bytes, %v", size, len(got), err)
		}
	}
}

type failWriter struct{ after int }

func (w *failWriter) Write(p []byte) (int, error) {
	if w.after == 0 {
		return 0, io.ErrClosedPipe
	}
	w.after--
	return len(p), nil
}

func TestWriteFrameErrors(t *testing.T) {
	if err := tcp.WriteFrame(io.Discard, make([]byte, transport.MaxFrame+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("WriteFrame(MaxFrame+1) = %v, want ErrFrameTooLarge", err)
	}
	for _, tc := range []struct {
		name  string
		size  int
		after int
	}{{"small", 10, 0}, {"large header", 100 << 10, 0}, {"large payload", 100 << 10, 1}} {
		if err := tcp.WriteFrame(&failWriter{after: tc.after}, make([]byte, tc.size)); !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("%s: WriteFrame() = %v, want ErrClosedPipe", tc.name, err)
		}
	}
}

func FuzzReadFrame(f *testing.F) {
	f.Add([]byte{0, 0, 0, 3, 'a', 'b', 'c'}, 10)
	f.Add([]byte{0xff, 0xff, 0xff, 0xff}, 1<<20)
	f.Add([]byte{0, 0, 0, 0}, 0)
	f.Add([]byte{0, 0, 0, 9, 1}, 100)
	f.Fuzz(func(t *testing.T, in []byte, limit int) {
		limit %= 1 << 20 // keep the fuzzer from asking for huge buffers
		got, err := tcp.ReadFrame(bytes.NewReader(in), limit)
		if err != nil {
			if got != nil {
				t.Fatalf("ReadFrame returned data with error %v", err)
			}
			return
		}
		if len(got) > limit || cap(got) > limit {
			t.Fatalf("ReadFrame(limit %d) returned len %d cap %d", limit, len(got), cap(got))
		}
		var buf bytes.Buffer
		if err := tcp.WriteFrame(&buf, got); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
		back, err := tcp.ReadFrame(&buf, limit)
		if err != nil || !bytes.Equal(back, got) {
			t.Fatalf("round trip = %x, %v; want %x", back, err, got)
		}
	})
}
