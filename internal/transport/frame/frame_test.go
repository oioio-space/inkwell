package frame_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestReadRejectsOversize(t *testing.T) {
	// Announces 1 GiB; Read must refuse from the header alone.
	r := bytes.NewReader([]byte{0x40, 0, 0, 0})
	got, err := frame.Read(r, 1024)
	if !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Read(1GiB header, limit 1024) = %v, %v; want ErrFrameTooLarge", got, err)
	}
}

func TestReadTruncated(t *testing.T) {
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
			if _, err := frame.Read(bytes.NewReader(tc.in), 100); !errors.Is(err, tc.want) {
				t.Errorf("Read(%x) error = %v, want %v", tc.in, err, tc.want)
			}
		})
	}
}

func TestRoundTripSizes(t *testing.T) {
	for _, size := range []int{0, 1, 1000, 64 << 10, 64<<10 + 1, 300 << 10} {
		payload := bytes.Repeat([]byte{0xAB}, size)
		var buf bytes.Buffer
		if err := frame.Write(&buf, payload); err != nil {
			t.Fatalf("Write(%d bytes): %v", size, err)
		}
		got, err := frame.Read(&buf, size)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("Read(%d bytes) = %d bytes, %v", size, len(got), err)
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

func TestWriteErrors(t *testing.T) {
	if err := frame.Write(io.Discard, make([]byte, transport.MaxFrame+1)); !errors.Is(err, transport.ErrFrameTooLarge) {
		t.Errorf("Write(MaxFrame+1) = %v, want ErrFrameTooLarge", err)
	}
	for _, tc := range []struct {
		name  string
		size  int
		after int
	}{{"small", 10, 0}, {"large header", 100 << 10, 0}, {"large payload", 100 << 10, 1}} {
		if err := frame.Write(&failWriter{after: tc.after}, make([]byte, tc.size)); !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("%s: Write() = %v, want ErrClosedPipe", tc.name, err)
		}
	}
}

func FuzzRead(f *testing.F) {
	f.Add([]byte{0, 0, 0, 3, 'a', 'b', 'c'}, 10)
	f.Add([]byte{0xff, 0xff, 0xff, 0xff}, 1<<20)
	f.Add([]byte{0, 0, 0, 0}, 0)
	f.Add([]byte{0, 0, 0, 9, 1}, 100)
	f.Fuzz(func(t *testing.T, in []byte, limit int) {
		limit %= 1 << 20 // keep the fuzzer from asking for huge buffers
		got, err := frame.Read(bytes.NewReader(in), limit)
		if err != nil {
			if got != nil {
				t.Fatalf("Read returned data with error %v", err)
			}
			return
		}
		if len(got) > limit || cap(got) > limit {
			t.Fatalf("Read(limit %d) returned len %d cap %d", limit, len(got), cap(got))
		}
		var buf bytes.Buffer
		if err := frame.Write(&buf, got); err != nil {
			t.Fatalf("Write: %v", err)
		}
		back, err := frame.Read(&buf, limit)
		if err != nil || !bytes.Equal(back, got) {
			t.Fatalf("round trip = %x, %v; want %x", back, err, got)
		}
	})
}
