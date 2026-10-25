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

var kinds = map[string]transport.Kind{
	"PeerUp": transport.PeerUp, "PeerDown": transport.PeerDown, "Message": transport.Message,
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
	if len(vectors) < 6 {
		t.Fatalf("got %d vectors, want at least 6", len(vectors))
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			payload, err := hex.DecodeString(v.PayloadHex)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := hex.DecodeString(v.WireHex)
			if err != nil {
				t.Fatal(err)
			}
			env := bridge.Envelope{Kind: kinds[v.Kind], Peer: transport.PeerID(v.Peer), Payload: payload}

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
		{name: "kind 4", wire: []byte{1, 4, 1, 'a'}},
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
		{name: "bad peer", env: bridge.Envelope{Kind: transport.Message, Peer: "a b"}},
		{name: "payload on PeerDown", env: bridge.Envelope{Kind: transport.PeerDown, Peer: "a", Payload: []byte{1}}},
		{name: "oversized", env: bridge.Envelope{Kind: transport.Message, Peer: "a", Payload: make([]byte, transport.MaxFrame+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := bridge.Encode(tt.env); !errors.Is(err, bridge.ErrMalformed) {
				t.Errorf("Encode() error = %v, want ErrMalformed", err)
			}
		})
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{1, 3, 1, 'a', 'x'})
	f.Add([]byte{1, 1, 4, 'k', '7', 'f', '3'})
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
	b, err := bridge.Encode(bridge.Envelope{Kind: transport.Message, Peer: peer, Payload: make([]byte, transport.MaxFrame)})
	if err != nil {
		t.Fatalf("Encode(MaxFrame payload): %v", err)
	}
	if len(b) > wailsLimit {
		t.Errorf("encoded %d bytes, want at most %d", len(b), wailsLimit)
	}
	_, err = bridge.Encode(bridge.Envelope{Kind: transport.Message, Peer: peer, Payload: make([]byte, transport.MaxFrame+1)})
	if !errors.Is(err, bridge.ErrMalformed) {
		t.Errorf("Encode(MaxFrame+1 payload) = %v, want ErrMalformed", err)
	}
}
