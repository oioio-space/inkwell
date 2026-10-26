package app

import (
	"encoding/json"
	"testing"
)

func FuzzCheckPresentFrame(f *testing.F) {
	f.Add([]byte(`{"type":"state","slide":3}`))
	f.Add([]byte(`{"type":"roster","windows":2}`))
	f.Add([]byte(`{"type":"a b"}`))
	f.Add([]byte(`[1,2]`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if checkPresentFrame(b) != nil {
			return
		}
		// Whatever passes is a JSON object with a short lowercase type.
		var head struct{ Type string }
		if err := json.Unmarshal(b, &head); err != nil {
			t.Fatalf("accepted %q, which does not decode: %v", b, err)
		}
		if n := len(head.Type); n == 0 || n > maxPresentType {
			t.Fatalf("accepted %q with a type of %d bytes", b, n)
		}
		for _, c := range []byte(head.Type) {
			if c < 'a' || c > 'z' {
				t.Fatalf("accepted %q with type %q", b, head.Type)
			}
		}
	})
}
