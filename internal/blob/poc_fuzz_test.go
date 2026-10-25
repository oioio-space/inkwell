package blob

import "testing"

func FuzzCheckSliceForgery(f *testing.F) {
	data := make([]byte, 3*GroupSize+7)
	for i := range data {
		data[i] = byte(i * 7)
	}
	h := HashOf(data)
	f.Add(uint64(0), uint32(GroupSize), []byte{1, 2, 3, 4, 5, 6, 7, 8})
	f.Fuzz(func(t *testing.T, off uint64, n uint32, b []byte) {
		m := Slice{Hash: h, Offset: off, Length: n, Bao: b}
		enc, err := Encode(m)
		if err != nil {
			return
		}
		dm, err := Decode(enc)
		if err != nil {
			return
		}
		got, size, final, err := checkSlice(dm.(Slice), h)
		if err == nil {
			t.Fatalf("forged slice accepted: %d bytes size %d final %v", len(got), size, final)
		}
	})
}
