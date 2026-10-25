package bridge_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// vector is one golden case of testdata/envelopes.json, shared with the
// TypeScript implementation.
type vector struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Peer       string `json:"peer"`
	PayloadHex string `json:"payloadHex"`
	WireHex    string `json:"wireHex"`
}

var kinds = map[string]bridge.Kind{
	"PeerUp": bridge.PeerUp, "PeerDown": bridge.PeerDown, "Message": bridge.Message,
	"Attest": bridge.Attest, "Attested": bridge.Attested, "Revoked": bridge.Revoked, "Sealed": bridge.Sealed,
}

func TestEnvelopeVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/envelopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []vector
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) < 13 {
		t.Fatalf("got %d vectors, want at least 13", len(vectors))
	}

	seen := map[bridge.Kind]bool{}
	for _, v := range vectors {
		k, ok := kinds[v.Kind]
		if !ok {
			t.Fatalf("vector %q has unknown kind %q", v.Name, v.Kind)
		}
		seen[k] = true
		t.Run(v.Name, func(t *testing.T) {
			payload, err := hex.DecodeString(v.PayloadHex)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := hex.DecodeString(v.WireHex)
			if err != nil {
				t.Fatal(err)
			}
			env := bridge.Envelope{Kind: k, Peer: transport.PeerID(v.Peer), Payload: payload}

			got, err := bridge.Encode(env)
			if err != nil {
				t.Fatalf("Encode(%+v): %v", env, err)
			}
			if !bytes.Equal(got, wire) {
				t.Errorf("Encode() = %x, want %x", got, wire)
			}

			dec, err := bridge.Decode(wire)
			if err != nil {
				t.Fatalf("Decode(%x): %v", wire, err)
			}
			if dec.Kind != env.Kind || dec.Peer != env.Peer || !bytes.Equal(dec.Payload, env.Payload) {
				t.Errorf("Decode(%x) = %+v, want %+v", wire, dec, env)
			}
		})
	}
	for name, k := range kinds {
		if !seen[k] {
			t.Errorf("no golden vector for kind %s", name)
		}
	}
}

func TestHeadsVector(t *testing.T) {
	// The payload of the "attest two heads" vector.
	want, err := hex.DecodeString("0004646f6331" + strings.Repeat("01", 32) + strings.Repeat("fe", 32))
	if err != nil {
		t.Fatal(err)
	}
	h := bridge.Heads{Doc: "doc1", Heads: [][bridge.HashSize]byte{filled(1), filled(0xfe)}}
	got, err := bridge.EncodeHeads(h)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("EncodeHeads() = %x, %v; want %x", got, err, want)
	}
	back, err := bridge.DecodeHeads(want)
	if err != nil || back.Doc != h.Doc || len(back.Heads) != 2 || back.Heads[1] != h.Heads[1] {
		t.Errorf("DecodeHeads(%x) = %+v, %v; want %+v", want, back, err, h)
	}
}

func filled(b byte) (h [bridge.HashSize]byte) {
	for i := range h {
		h[i] = b
	}
	return h
}

func TestKindString(t *testing.T) {
	for k, want := range map[bridge.Kind]string{
		bridge.PeerUp: "PeerUp", bridge.Attest: "Attest", bridge.Attested: "Attested",
		bridge.Revoked: "Revoked", bridge.Sealed: "Sealed", 0: "Kind(0)",
	} {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", uint8(k), got, want)
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name string
		wire []byte
	}{
		{name: "empty", wire: nil},
		{name: "header only", wire: []byte{1, 3}},
		{name: "version 2", wire: []byte{2, 3, 1, 'a'}},
		{name: "version 0", wire: []byte{0, 3, 1, 'a'}},
		{name: "kind 0", wire: []byte{1, 0, 1, 'a'}},
		{name: "kind 8", wire: []byte{1, 8, 1, 'a'}},
		{name: "attest without payload", wire: []byte{1, 4, 1, 'a'}},
		{name: "attest empty document", wire: []byte{1, 4, 1, 'a', 0, 0}},
		{name: "attest document too long", wire: []byte{1, 4, 1, 'a', 0, 5, 'd'}},
		{name: "attest short head", wire: append([]byte{1, 4, 1, 'a', 0, 1, 'd'}, make([]byte, 31)...)},
		{name: "attest invalid UTF-8 document", wire: []byte{1, 4, 1, 'a', 0, 1, 0xff}},
		{name: "attest 65 heads", wire: append([]byte{1, 4, 1, 'a', 0, 1, 'd'}, make([]byte, 65*32)...)},
		{name: "attested without payload", wire: []byte{1, 5, 1, 'a'}},
		{name: "payload on Revoked", wire: []byte{1, 6, 1, 'a', 9}},
		{name: "payload on Sealed", wire: []byte{1, 7, 1, 'a', 9}},
		{name: "truncated peer", wire: []byte{1, 3, 4, 'a', 'b'}},
		{name: "empty peer", wire: []byte{1, 3, 0, 'x'}},
		{name: "peer with space", wire: []byte{1, 3, 1, ' '}},
		{name: "peer non ascii", wire: []byte{1, 3, 2, 0xc3, 0xa9}},
		{name: "peer too long", wire: append([]byte{1, 3, 65}, strings.Repeat("a", 65)...)},
		{name: "payload on PeerUp", wire: []byte{1, 1, 1, 'a', 9}},
		{name: "oversized", wire: append([]byte{1, 3, 1, 'a'}, make([]byte, transport.MaxFrame+1)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := bridge.Decode(tt.wire); !errors.Is(err, bridge.ErrMalformed) {
				t.Errorf("Decode() error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestEncodeRejects(t *testing.T) {
	tests := []struct {
		name string
		env  bridge.Envelope
	}{
		{name: "bad kind", env: bridge.Envelope{Kind: 9, Peer: "a"}},
		{name: "attest without payload", env: bridge.Envelope{Kind: bridge.Attest, Peer: "a"}},
		{name: "payload on Revoked", env: bridge.Envelope{Kind: bridge.Revoked, Peer: "a", Payload: []byte{1}}},
		{name: "bad peer", env: bridge.Envelope{Kind: bridge.Message, Peer: "a b"}},
		{name: "payload on PeerDown", env: bridge.Envelope{Kind: bridge.PeerDown, Peer: "a", Payload: []byte{1}}},
		{name: "oversized", env: bridge.Envelope{Kind: bridge.Message, Peer: "a", Payload: make([]byte, transport.MaxFrame+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := bridge.Encode(tt.env); !errors.Is(err, bridge.ErrMalformed) {
				t.Errorf("Encode() error = %v, want ErrMalformed", err)
			}
		})
	}
}

func FuzzDecodeHeads(f *testing.F) {
	f.Add([]byte{0, 1, 'd'})
	f.Add(append([]byte{0, 1, 'd'}, make([]byte, 32)...))
	f.Add([]byte{0, 9})
	f.Fuzz(func(t *testing.T, b []byte) {
		h, err := bridge.DecodeHeads(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("DecodeHeads(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		again, err := bridge.EncodeHeads(h)
		if err != nil || !bytes.Equal(again, b) {
			t.Fatalf("round trip of %x = %x, %v", b, again, err)
		}
	})
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{1, 3, 1, 'a', 'x'})
	f.Add([]byte{1, 1, 4, 'k', '7', 'f', '3'})
	f.Add(append([]byte{1, 4, 1, 'a', 0, 1, 'd'}, make([]byte, 64)...))
	f.Add(append([]byte{1, 5, 1, 'a', 0, 1, 'd'}, make([]byte, 32)...))
	f.Add([]byte{1, 6, 1, 'a'})
	f.Add([]byte{1, 7, 1, 'a'})
	f.Add([]byte{2, 3, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		env, err := bridge.Decode(b)
		if err != nil {
			if !errors.Is(err, bridge.ErrMalformed) {
				t.Fatalf("Decode(%x) error %v is not ErrMalformed", b, err)
			}
			return
		}
		again, err := bridge.Encode(env)
		if err != nil {
			t.Fatalf("Encode(Decode(%x)): %v", b, err)
		}
		if !bytes.Equal(again, b) {
			t.Fatalf("round trip of %x = %x", b, again)
		}
	})
}

func TestMaxFrameFitsWailsLimit(t *testing.T) {
	const wailsLimit = 64 << 20
	peer := transport.PeerID(strings.Repeat("p", 64))
	b, err := bridge.Encode(bridge.Envelope{Kind: bridge.Message, Peer: peer, Payload: make([]byte, transport.MaxFrame)})
	if err != nil {
		t.Fatalf("Encode(MaxFrame payload): %v", err)
	}
	if len(b) > wailsLimit {
		t.Errorf("encoded %d bytes, want at most %d", len(b), wailsLimit)
	}
	_, err = bridge.Encode(bridge.Envelope{Kind: bridge.Message, Peer: peer, Payload: make([]byte, transport.MaxFrame+1)})
	if !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("Encode(MaxFrame+1 payload) = %v, want ErrMalformed", err)
	}
}
