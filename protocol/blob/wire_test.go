// SPDX-License-Identifier: MPL-2.0

package blob

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/protocol/vectors"
)

func frame(kind byte, offset uint64, length uint32, payload int) []byte {
	b := make([]byte, sliceHead+payload)
	b[0] = kind
	binary.BigEndian.PutUint64(b[hashEnd:], offset)
	binary.BigEndian.PutUint32(b[hashEnd+8:], length)
	return b
}

func TestDecodeRejects(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"wrong type", []byte{255}},
		{"want truncated", make([]byte, wantLen-1)},
		{"want trailing", frame(kindWant, 0, GroupSize, 1)},
		{"want zero length", frame(kindWant, 0, 0, 0)},
		{"want oversized length", frame(kindWant, 0, MaxSlice+1, 0)},
		{"want unaligned length", frame(kindWant, 0, 1, 0)},
		{"want unaligned offset", frame(kindWant, 1, GroupSize, 0)},
		{"want oversized offset", frame(kindWant, maxOffset+GroupSize, GroupSize, 0)},
		{"slice truncated", []byte{kindSlice}},
		{"slice zero length", frame(kindSlice, 0, 0, 8)},
		{"slice oversized length", frame(kindSlice, 0, MaxSlice+1, 8)},
		{"slice unaligned offset", frame(kindSlice, 1, 1, 9)},
		{"slice oversized offset", frame(kindSlice, maxOffset+GroupSize, 1, 9)},
		{"slice short bao", frame(kindSlice, 0, 1, 8)},
		{"slice oversized bao", frame(kindSlice, 0, 1, maxBao+1)},
		{"missing truncated", []byte{kindMissing}},
		{"missing trailing", append([]byte{kindMissing}, make([]byte, HashSize+1)...)},
		{"busy truncated", []byte{kindBusy}},
		{"busy trailing", append([]byte{kindBusy}, make([]byte, HashSize+5)...)},
		{"busy excessive retry", append(append([]byte{kindBusy}, make([]byte, HashSize)...), 0, 0, 234, 97)},
	}
	cases[2].data[0] = kindWant
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(tc.data); !errors.Is(err, ErrMalformed) {
				t.Fatalf("Decode error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestCodecRoundTrip(t *testing.T) {
	for _, m := range []Message{Want{Offset: maxOffset, Length: MaxSlice}, Slice{Length: 1, Bao: make([]byte, 9)}, Missing{}, Busy{Retry: time.Minute}} {
		b, err := Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(b)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := Encode(got)
		if err != nil || !bytes.Equal(encoded, b) {
			t.Fatalf("round trip: %v", err)
		}
	}
	for _, m := range []Message{nil, Want{}, Slice{}, Slice{Length: 1}, Busy{Retry: -1}, Busy{Retry: time.Nanosecond}, Busy{Retry: time.Hour}} {
		if _, err := Encode(m); !errors.Is(err, ErrMalformed) {
			t.Fatalf("Encode(%#v) = %v", m, err)
		}
	}
}

func TestHashText(t *testing.T) {
	h := HashOf([]byte("hello"))
	parsed, err := ParseHash(h.String())
	if err != nil || parsed != h {
		t.Fatalf("hash round trip: %v", err)
	}
	if h.String() != HashPrefix+h.Hex() || h.Short() != h.Hex()[:8] {
		t.Fatal("inconsistent hash formatting")
	}
	for _, s := range []string{"", h.Hex(), HashPrefix + strings.Repeat("A", 64), HashPrefix + strings.Repeat("z", 64)} {
		if _, err := ParseHash(s); !errors.Is(err, ErrBadHash) {
			t.Fatalf("ParseHash(%q) = %v", s, err)
		}
	}
}

func FuzzDecode(f *testing.F) {
	// docs/protocol/vectors points to this canonical embedded bulk corpus.
	data, err := vectors.FS.ReadFile("bulk.json")
	if err != nil {
		f.Fatal(err)
	}
	var corpus struct {
		Vectors []struct {
			Decoder string `json:"decoder"`
			Input   string `json:"input_hex"`
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		f.Fatal(err)
	}
	for _, v := range corpus.Vectors {
		if v.Decoder == "bulk" && v.Input != "" {
			b, err := hex.DecodeString(v.Input)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(b)
		}
	}
	f.Add([]byte{})
	f.Add([]byte{255})
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := Decode(b) // A panic fails the fuzz test automatically.
		if err != nil {
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		encoded, err := Encode(m)
		if err != nil || !bytes.Equal(b, encoded) {
			t.Fatalf("accepted frame does not round trip: %v", err)
		}
	})
}
