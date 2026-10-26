package blob_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/oioio-space/inkwell/internal/blob"
)

func hashOfByte(b byte) blob.Hash {
	var h blob.Hash
	for i := range h {
		h[i] = b
	}
	return h
}

func TestWireGolden(t *testing.T) {
	h := hashOfByte(0xAB)
	hh := hex.EncodeToString(h[:])
	tests := []struct {
		name string
		msg  blob.Message
		wire string
	}{
		{
			"want",
			blob.Want{Hash: h, Offset: 32768, Length: 1 << 20},
			"01" + hh + "0000000000008000" + "00100000",
		},
		{
			"slice",
			blob.Slice{Hash: h, Offset: 0, Length: 1, Bao: []byte{1, 0, 0, 0, 0, 0, 0, 0, 'x'}},
			"02" + hh + "0000000000000000" + "00000001" + "010000000000000078",
		},
		{"missing", blob.Missing{Hash: h}, "03" + hh},
		{"busy", blob.Busy{Hash: h, Retry: 1500 * time.Millisecond}, "04" + hh + "000005dc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := blob.Encode(tc.msg)
			if err != nil || hex.EncodeToString(got) != tc.wire {
				t.Fatalf("Encode(%+v) = %x, %v; want %s", tc.msg, got, err, tc.wire)
			}
			back, err := blob.Decode(got)
			if err != nil || !cmp.Equal(back, tc.msg) {
				t.Errorf("Decode(Encode(%+v)) = %+v, %v", tc.msg, back, err)
			}
		})
	}
}

func TestWireRejects(t *testing.T) {
	h := hashOfByte(1)
	frame := func(m blob.Message) []byte {
		b, err := blob.Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	want := frame(blob.Want{Hash: h, Offset: 0, Length: blob.GroupSize})
	tests := []struct {
		name string
		wire []byte
	}{
		{"empty", nil},
		{"unknown kind", append([]byte{9}, h[:]...)},
		{"truncated want", want[:len(want)-1]},
		{"want with trailing bytes", append(bytes.Clone(want), 0)},
		{"want of zero length", func() []byte { b := bytes.Clone(want); copy(b[len(b)-4:], []byte{0, 0, 0, 0}); return b }()},
		{"want over the slice limit", func() []byte { b := bytes.Clone(want); copy(b[len(b)-4:], []byte{0, 0x20, 0, 0}); return b }()},
		{"want not a multiple of the group", func() []byte { b := bytes.Clone(want); copy(b[len(b)-4:], []byte{0, 0, 0x40, 1}); return b }()},
		{"want at a misaligned offset", func() []byte { b := bytes.Clone(want); b[hashEndForTest+7] = 1; return b }()},
		{"want at an absurd offset", func() []byte { b := bytes.Clone(want); b[hashEndForTest] = 0xFF; return b }()},
		{"truncated missing", frame(blob.Missing{Hash: h})[:10]},
		{"busy over a minute", func() []byte { b := frame(blob.Busy{Hash: h}); copy(b[len(b)-4:], []byte{0, 1, 0, 0}); return b }()},
		{"slice shorter than its data", func() []byte {
			b := frame(blob.Slice{Hash: h, Length: 4, Bao: make([]byte, 12)})
			return b[:len(b)-1]
		}()},
		{"slice without a body", append([]byte{2}, make([]byte, 44)...)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if m, err := blob.Decode(tc.wire); !errors.Is(err, blob.ErrMalformed) {
				t.Errorf("Decode(%x) = %+v, %v; want ErrMalformed", tc.wire, m, err)
			}
		})
	}
}

// hashEndForTest is the offset of the first field after the hash.
const hashEndForTest = 1 + blob.HashSize

func TestEncodeRejects(t *testing.T) {
	h := hashOfByte(1)
	for name, m := range map[string]blob.Message{
		"want out of the group": blob.Want{Hash: h, Offset: 1, Length: blob.GroupSize},
		"want too long":         blob.Want{Hash: h, Length: blob.MaxSlice + blob.GroupSize},
		"slice without data":    blob.Slice{Hash: h, Length: 0, Bao: make([]byte, 8)},
		"slice too short":       blob.Slice{Hash: h, Length: 10, Bao: make([]byte, 12)},
		"busy forever":          blob.Busy{Hash: h, Retry: time.Hour},
		"busy backwards":        blob.Busy{Hash: h, Retry: -time.Second},
		"nil":                   nil,
	} {
		if b, err := blob.Encode(m); !errors.Is(err, blob.ErrMalformed) {
			t.Errorf("%s: Encode = %x, %v; want ErrMalformed", name, b, err)
		}
	}
}

// FuzzBlobWire: Decode never panics on a peer's bytes, and what it accepts
// encodes back to the same bytes.
func FuzzBlobWire(f *testing.F) {
	h := hashOfByte(7)
	for _, m := range []blob.Message{
		blob.Want{Hash: h, Offset: 0, Length: blob.GroupSize},
		blob.Want{Hash: h, Offset: 5 * blob.GroupSize, Length: blob.MaxSlice},
		blob.Slice{Hash: h, Offset: blob.GroupSize, Length: 3, Bao: append(make([]byte, 8), 1, 2, 3)},
		blob.Missing{Hash: h},
		blob.Busy{Hash: h, Retry: time.Second},
	} {
		b, err := blob.Encode(m)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Add([]byte{})
	f.Add([]byte{kindUnknown})
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := blob.Decode(b)
		if err != nil {
			if !errors.Is(err, blob.ErrMalformed) {
				t.Fatalf("Decode(%x) error %v does not wrap ErrMalformed", b, err)
			}
			return
		}
		again, err := blob.Encode(m)
		if err != nil || !bytes.Equal(again, b) {
			t.Fatalf("Encode(Decode(%x)) = %x, %v", b, again, err)
		}
	})
}

const kindUnknown = 0xEE
