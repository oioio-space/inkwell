package bridge_test

import (
	"bytes"
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
)

// FuzzSmallCodecs feeds bytes to the small fixed-layout decoders no other fuzz
// target reaches. None may panic, and what one accepts it writes back byte for
// byte: a decoder that accepts more than its encoder writes is a second
// spelling of the same value.
func FuzzSmallCodecs(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 1, 0})
	f.Add([]byte{0, 0, 0, 1, byte(bridge.StatusTooLarge), 'x'})
	f.Add([]byte{0, 3, 'a', 'b', 'c', 't', 'e', 'x', 't'})
	f.Add(bridge.EncodeBlobInfo([32]byte{1}, 9))
	f.Add(bridge.EncodeTreeResult(3))
	f.Add(bridge.EncodeFileInfo(bridge.FileInfo{Encoding: "utf-8", BOM: true}))
	f.Fuzz(func(t *testing.T, b []byte) {
		if r, err := bridge.DecodeStoreResponse(b); err == nil {
			if back, err := bridge.EncodeStoreResponse(r); err != nil || !bytes.Equal(back, b) {
				t.Errorf("store response %x re-encodes to %x, %v", b, back, err)
			}
		}
		if point, text, err := bridge.DecodeWriteAt(b); err == nil {
			if back, err := bridge.EncodeWriteAt(point, text); err != nil || !bytes.Equal(back, b) {
				t.Errorf("write-at %x re-encodes to %x, %v", b, back, err)
			}
		}
		if h, size, err := bridge.DecodeBlobInfo(b); err == nil {
			if back := bridge.EncodeBlobInfo(h, size); !bytes.Equal(back, b) {
				t.Errorf("blob info %x re-encodes to %x", b, back)
			}
		}
		if n, err := bridge.DecodeTreeResult(b); err == nil {
			if back := bridge.EncodeTreeResult(n); !bytes.Equal(back, b) {
				t.Errorf("tree result %x re-encodes to %x", b, back)
			}
		}
		if info, err := bridge.DecodeFileInfo(b); err == nil {
			if back := bridge.EncodeFileInfo(info); !bytes.Equal(back, b) {
				t.Errorf("file info %x re-encodes to %x", b, back)
			}
		}
	})
}
