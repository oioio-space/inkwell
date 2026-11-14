package instance

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

// vectors is testdata/handoff-v1.json, written by an encoder independent of this package: the
// bytes two versions of Inkwell must keep agreeing on.
type vectors struct {
	Requests []struct {
		Name    string `json:"name"`
		Hex     string `json:"hex"`
		Request struct {
			ID         string   `json:"id"`
			Cwd        string   `json:"cwd"`
			Activation string   `json:"activation"`
			Dropped    uint16   `json:"dropped"`
			Args       []string `json:"args"`
		} `json:"request"`
	} `json:"requests"`
	Rejects []struct {
		Name   string `json:"name"`
		Hex    string `json:"hex"`
		Status string `json:"status"`
	} `json:"rejects"`
	Sockets []struct {
		Profile    string   `json:"profile"`
		Runtime    string   `json:"runtime"`
		Candidates []string `json:"candidates"`
	} `json:"sockets"`
}

func loadVectors(t *testing.T) vectors {
	t.Helper()
	b, err := os.ReadFile("testdata/handoff-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Requests) == 0 || len(v.Rejects) == 0 || len(v.Sockets) == 0 {
		t.Fatal("empty vector file")
	}
	return v
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestRequestVectors pins wire format v1 (plan section 4.1): each accepted frame decodes to its
// request and encodes back to the same bytes (one encoding per request), and each rejected frame
// gets the status the primary answers with: « unsupported » for another version (R17, F14),
// « malformed » for anything else.
func TestRequestVectors(t *testing.T) {
	v := loadVectors(t)
	for _, tc := range v.Requests {
		frame := mustHex(t, tc.Hex)
		var id [16]byte
		copy(id[:], mustHex(t, tc.Request.ID))
		want := Request{ID: id, Cwd: tc.Request.Cwd, Activation: tc.Request.Activation, Dropped: tc.Request.Dropped, Args: tc.Request.Args}
		got, err := decodeRequest(frame)
		if err != nil {
			t.Errorf("%s: decodeRequest: %v", tc.Name, err)
			continue
		}
		if !equalRequest(got, want) {
			t.Errorf("%s: decodeRequest = %+v, want %+v", tc.Name, got, want)
		}
		enc, err := want.Encode()
		if err != nil {
			t.Errorf("%s: Encode: %v", tc.Name, err)
			continue
		}
		if !bytes.Equal(enc, frame) {
			t.Errorf("%s: Encode = %x, want %x", tc.Name, enc, frame)
		}
	}
	for _, tc := range v.Rejects {
		_, err := decodeRequest(mustHex(t, tc.Hex))
		if got := replyTo(err); got.String() != tc.Status {
			t.Errorf("%s: status %v (err %v), want %s", tc.Name, got, err, tc.Status)
		}
	}
}

func equalRequest(a, b Request) bool {
	return a.ID == b.ID && a.Cwd == b.Cwd && a.Activation == b.Activation && a.Dropped == b.Dropped &&
		slices.Equal(a.Args, b.Args)
}

// TestRequestEncodeBounds checks that a request outside the bounds is refused before any byte is
// written, and that the largest request fills a frame of exactly MaxFrame bytes.
func TestRequestEncodeBounds(t *testing.T) {
	largest := Request{Cwd: "/" + strings.Repeat("c", MaxCwd-1), Activation: strings.Repeat("!", MaxActivation), Args: slices.Repeat([]string{strings.Repeat("a", MaxArg)}, MaxArgs)}
	b, err := largest.Encode()
	if err != nil {
		t.Fatalf("Encode(largest): %v", err)
	}
	if len(b) != 4+MaxFrame || MaxFrame != 136280 {
		t.Errorf("largest frame = %d bytes, MaxFrame = %d; want 4+136280", len(b), MaxFrame)
	}
	if got, err := decodeRequest(b); err != nil || !equalRequest(got, largest) {
		t.Errorf("decodeRequest(largest) = %v", err)
	}
	ok := Request{Cwd: "/"}
	bad := map[string]Request{
		"empty cwd":           {},
		"cwd over 4096":       {Cwd: strings.Repeat("/", MaxCwd+1)},
		"cwd with NUL":        {Cwd: "/\x00"},
		"token over 1024":     {Cwd: "/", Activation: strings.Repeat("t", MaxActivation+1)},
		"token with space":    {Cwd: "/", Activation: "a b"},
		"token with 0x7f":     {Cwd: "/", Activation: "\x7f"},
		"token with 0x20":     {Cwd: "/", Activation: " "},
		"33 arguments":        {Cwd: "/", Args: slices.Repeat([]string{"a"}, MaxArgs+1)},
		"empty argument":      {Cwd: "/", Args: []string{""}},
		"argument over 4096":  {Cwd: "/", Args: []string{strings.Repeat("a", MaxArg+1)}},
		"argument with NUL":   {Cwd: "/", Args: []string{"a\x00"}},
		"argument NUL first":  {Cwd: "/", Args: []string{"\x00a"}},
		"cwd NUL first":       {Cwd: "\x00/"},
		"second argument bad": {Cwd: "/", Args: []string{"a", ""}},
	}
	for name, r := range bad {
		if b, err := r.Encode(); !errors.Is(err, errInvalid) || b != nil {
			t.Errorf("%s: Encode = %d bytes, %v; want errInvalid", name, len(b), err)
		}
	}
	edge := []Request{
		ok,
		{Cwd: strings.Repeat("/", MaxCwd)},
		{Cwd: "/", Activation: "!~"},
		{Cwd: "/", Args: []string{strings.Repeat("a", MaxArg)}},
		{Cwd: "/", Args: slices.Repeat([]string{"a"}, MaxArgs)},
	}
	for _, r := range edge {
		if _, err := r.Encode(); err != nil {
			t.Errorf("Encode(%.40q…): %v", r.Cwd, err)
		}
	}
}

// TestRequestNew: whatever a command line holds, the request of a launch encodes: empty arguments
// left out, a longer one cut to MaxArg bytes (so that the primary still calls it too long), those
// beyond MaxArgs counted, an out-of-bounds directory or token replaced; a fresh ID each time.
func TestRequestNew(t *testing.T) {
	long := strings.Repeat("a", MaxArg+10)
	args := append([]string{"", "a.typ", long, "b\x00c"}, slices.Repeat([]string{"x"}, 40)...)
	r := NewRequest(args, "/home/ana", "tok-1")
	if _, err := r.Encode(); err != nil {
		t.Fatalf("Encode(NewRequest) = %v", err)
	}
	if len(r.Args) != MaxArgs || r.Args[0] != "a.typ" || r.Args[1] != long[:MaxArg] || r.Args[2] != "b" {
		t.Errorf("Args = %.3q… (%d)", r.Args, len(r.Args))
	}
	if r.Dropped != 40+3-MaxArgs || r.Cwd != "/home/ana" || r.Activation != "tok-1" {
		t.Errorf("Dropped %d, Cwd %q, Activation %q", r.Dropped, r.Cwd, r.Activation)
	}
	if other := NewRequest(nil, "/", ""); other.ID == r.ID || other.ID == ([16]byte{}) {
		t.Error("IDs not random")
	}
	for cwd, want := range map[string]string{"": ".", strings.Repeat("/", MaxCwd+1): ".", "/a\x00": ".", strings.Repeat("/", MaxCwd): strings.Repeat("/", MaxCwd)} {
		if got := NewRequest(nil, cwd, "").Cwd; got != want {
			t.Errorf("NewRequest(cwd %.10q…).Cwd = %.10q…", cwd, got)
		}
	}
	for tok, want := range map[string]string{"a b": "", strings.Repeat("t", MaxActivation+1): "", strings.Repeat("t", MaxActivation): strings.Repeat("t", MaxActivation), "!~": "!~"} {
		if got := NewRequest(nil, "/", tok).Activation; got != want {
			t.Errorf("NewRequest(token %.10q…).Activation = %.10q…", tok, got)
		}
	}
	many := NewRequest(slices.Repeat([]string{"x"}, MaxArgs+70000), "/", "")
	if many.Dropped != 65535 {
		t.Errorf("Dropped = %d, want saturated at 65535", many.Dropped)
	}
	if exact := NewRequest(slices.Repeat([]string{"x"}, MaxArgs), "/", ""); exact.Dropped != 0 || len(exact.Args) != MaxArgs {
		t.Errorf("32 arguments: %d kept, %d dropped", len(exact.Args), exact.Dropped)
	}
	if edge := NewRequest([]string{strings.Repeat("a", MaxArg)}, "/", ""); len(edge.Args[0]) != MaxArg {
		t.Errorf("argument of MaxArg bytes cut to %d", len(edge.Args[0]))
	}
}

// TestRequestStatusNames pins the reply byte of each status (plan section 4.1) and its name.
func TestRequestStatusNames(t *testing.T) {
	want := map[Status]string{0: "accepted", 1: "closing", 2: "busy", 3: "malformed", 4: "unsupported", 5: "status(5)"}
	for s, name := range want {
		if s.String() != name {
			t.Errorf("Status(%d).String() = %q, want %q", byte(s), s.String(), name)
		}
	}
	if StatusAccepted != 0 || StatusClosing != 1 || StatusBusy != 2 || StatusMalformed != 3 || StatusUnsupported != 4 {
		t.Error("status bytes moved")
	}
}

// FuzzDecodeRequest: no panic on any input; an accepted frame respects every bound and encodes
// back to the same bytes (the format is canonical).
func FuzzDecodeRequest(f *testing.F) {
	v := vectors{}
	if b, err := os.ReadFile("testdata/handoff-v1.json"); err == nil {
		_ = json.Unmarshal(b, &v)
	}
	for _, tc := range v.Requests {
		if b, err := hex.DecodeString(tc.Hex); err == nil {
			f.Add(b)
		}
	}
	for _, tc := range v.Rejects {
		if b, err := hex.DecodeString(tc.Hex); err == nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		r, err := decodeRequest(b)
		if err != nil {
			return
		}
		if n := len(r.Cwd); n < 1 || n > MaxCwd || strings.IndexByte(r.Cwd, 0) >= 0 {
			t.Fatalf("cwd of %d bytes accepted", n)
		}
		if len(r.Activation) > MaxActivation || !printable(r.Activation) {
			t.Fatalf("activation %q accepted", r.Activation)
		}
		if len(r.Args) > MaxArgs {
			t.Fatalf("%d arguments accepted", len(r.Args))
		}
		for _, a := range r.Args {
			if len(a) < 1 || len(a) > MaxArg || strings.IndexByte(a, 0) >= 0 {
				t.Fatalf("argument of %d bytes accepted", len(a))
			}
		}
		enc, err := r.Encode()
		if err != nil {
			t.Fatalf("decoded request does not encode: %v", err)
		}
		if !bytes.Equal(enc, b) {
			t.Fatalf("Encode(Decode(b)) != b\n got %x\nwant %x", enc, b)
		}
	})
}
