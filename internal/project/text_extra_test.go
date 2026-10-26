package project

import (
	"bytes"
	"errors"
	"testing"
)

// TestMixedLineEndingsDominant: the dominant ending of a mixed file is the
// most frequent one, a tie going to LF then CRLF, and Encode writes it back
// everywhere.
func TestMixedLineEndingsDominant(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		eol  EOL
		dom  EOL
		body string
	}{
		{"lf and crlf, tie", "a\nb\r\nc", Mixed, LF, "a\nb\nc"},
		{"crlf wins", "a\nb\r\nc\r\nd", Mixed, CRLF, "a\nb\nc\nd"},
		{"lf wins", "a\nb\nc\r\nd", Mixed, LF, "a\nb\nc\nd"},
		{"cr wins", "a\rb\rc\nd", Mixed, CR, "a\nb\nc\nd"},
		{"cr ties crlf: crlf stays", "a\rb\r\nc", Mixed, CRLF, "a\nb\nc"},
		{"cr ties lf: lf stays", "a\rb\nc", Mixed, LF, "a\nb\nc"},
		{"cr beats both", "a\rb\rc\rd\r\ne\nf", Mixed, CR, "a\nb\nc\nd\ne\nf"},
		{"only crlf", "a\r\nb\r\n", CRLF, 0, "a\nb\n"},
		{"only cr", "a\rb\r", CR, 0, "a\nb\n"},
		{"only lf", "a\nb\n", LF, 0, "a\nb\n"},
		{"trailing cr then lf is one crlf", "a\r\n", CRLF, 0, "a\n"},
		{"cr at the very end", "a\nb\r", Mixed, LF, "a\nb\n"},
	} {
		got, err := Decode([]byte(tc.in))
		if err != nil {
			t.Fatalf("%s: Decode = %v", tc.name, err)
		}
		if got.EOL != tc.eol || got.Dominant != tc.dom || got.Body != tc.body {
			t.Errorf("%s: EOL %v dominant %v body %q; want %v, %v, %q", tc.name, got.EOL, got.Dominant, got.Body, tc.eol, tc.dom, tc.body)
		}
		if tc.eol != Mixed {
			if back := Encode(got); !bytes.Equal(back, []byte(tc.in)) {
				t.Errorf("%s: Encode(Decode(x)) = %q, want the input back", tc.name, back)
			}
		}
	}
	got, _ := Decode([]byte("a\rb\rc\nd"))
	if want := []byte("a\rb\rc\rd"); !bytes.Equal(Encode(got), want) {
		t.Errorf("a mixed file is written back with its dominant ending: %q, want %q", Encode(got), want)
	}
}

// TestInvalidUTF8Line: the line of the first isolated bad byte, whatever the
// line endings before it; with a BOM or any valid multi-byte character the
// file is UTF-8 with an error (a Windows-1252 file would not have either).
func TestInvalidUTF8Line(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	for _, tc := range []struct {
		name string
		in   []byte
		line int
	}{
		{"first line", append(bytes.Clone(bom), "ab\xff"...), 1},
		{"after lf", append(bytes.Clone(bom), "a\nb\xff"...), 2},
		{"after crlf", append(bytes.Clone(bom), "a\r\nb\r\nc\xff"...), 3},
		{"after cr", append(bytes.Clone(bom), "a\rb\rc\xff"...), 3},
		{"mixed endings", append(bytes.Clone(bom), "l1\r\nl2\rl3\nl4\xff"...), 4},
		{"first of several", append(bytes.Clone(bom), "a\nb\xff\nc\xff"...), 2},
		{"bad byte right after the BOM, another later", append(bytes.Clone(bom), "\xff\nok\n\xff"...), 1},
		{"multibyte makes it utf-8", []byte("é\nok\n\xff"), 3},
	} {
		_, err := Decode(tc.in)
		bad, ok := errors.AsType[*InvalidUTF8Error](err)
		if !ok || !errors.Is(err, ErrInvalidUTF8) {
			t.Errorf("%s: Decode = %v, want an InvalidUTF8Error", tc.name, err)
			continue
		}
		if bad.Line != tc.line {
			t.Errorf("%s: line %d, want %d", tc.name, bad.Line, tc.line)
		}
	}
}

// TestWindows1252Boundaries: every edge of the byte classes of the code page.
func TestWindows1252Boundaries(t *testing.T) {
	// A high byte after an ASCII one is invalid UTF-8, so the table is consulted.
	decode := func(b byte) (string, error) {
		got, err := Decode([]byte{'a', b})
		if err != nil {
			return "", err
		}
		if got.Encoding != EncodingWindows1252 {
			t.Errorf("byte %#x: encoding %q, want Windows-1252", b, got.Encoding)
		}
		return got.Body, nil
	}
	accepted := map[byte]string{
		0x80: "a\u20ac", 0x82: "a\u201a", 0x8E: "a\u017d", 0x91: "a\u2018", 0x9E: "a\u017e", 0x9F: "a\u0178",
		0xA0: "a\u00a0", 0xA9: "a\u00a9", 0xE9: "a\u00e9", 0xFF: "a\u00ff",
	}
	for b, want := range accepted {
		if got, err := decode(b); err != nil || got != want {
			t.Errorf("byte %#x = %q, %v; want %q", b, got, err, want)
		}
	}
	for _, b := range []byte{0x01, 0x08, 0x0B, 0x0E, 0x1F, 0x7F, 0x81, 0x8D, 0x8F, 0x90, 0x9D} {
		// The 0xE9 first makes the file invalid UTF-8 so that the table is consulted.
		if _, err := Decode([]byte{0xE9, b}); !errors.Is(err, ErrBinary) {
			t.Errorf("Decode(E9, %#x) = %v, want ErrBinary", b, err)
		}
	}
	for _, b := range []byte{0x09, 0x0A, 0x0C, 0x0D, 0x20, 0x7E, 0x80, 0x9F, 0xA0} {
		if _, err := Decode([]byte{0xE9, b}); err != nil {
			t.Errorf("Decode(E9, %#x) = %v, want accepted", b, err)
		}
	}
}

// TestWindows1252RoundTrip: what Decode maps, Encode maps back, byte for byte,
// for every byte the code page defines; and a character it cannot hold makes
// the text fall back to UTF-8 without loss.
func TestWindows1252RoundTrip(t *testing.T) {
	var all []byte
	for b := 0x20; b <= 0xFF; b++ {
		switch b {
		case 0x7F, 0x81, 0x8D, 0x8F, 0x90, 0x9D:
			continue
		}
		all = append(all, byte(b))
	}
	all = append(all, '\t', '\f')
	got, err := Decode(all)
	if err != nil || got.Encoding != EncodingWindows1252 {
		t.Fatalf("Decode(all bytes) = %q, %v; want Windows-1252", got.Encoding, err)
	}
	if back := Encode(got); !bytes.Equal(back, all) {
		t.Errorf("Encode(Decode(x)) differs from x:\n%x\n%x", back, all)
	}

	for _, r := range []string{"\u0100", "\u20ad", "\U0001F600", "\u0080", "\u0081", "\u009f"} {
		txt := Text{Body: "a" + r, Encoding: EncodingWindows1252, EOL: LF, BOM: true}
		out := Encode(txt)
		if want := []byte("a" + r); !bytes.Equal(out, want) {
			t.Errorf("Encode(Windows-1252 text with %U) = %x, want UTF-8 %x (no BOM)", []rune(r)[0], out, want)
		}
	}
	// The edges of the representable ranges.
	for r, want := range map[rune]byte{0x7F: 0x7F, 0xA0: 0xA0, 0xFF: 0xFF, 0x20AC: 0x80, 0x0178: 0x9F, 0x017E: 0x9E} {
		out := Encode(Text{Body: string(r), Encoding: EncodingWindows1252, EOL: LF})
		if !bytes.Equal(out, []byte{want}) {
			t.Errorf("Encode(%U) = %x, want %#x", r, out, want)
		}
	}
}

func TestUTF16RoundTripAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"LE with BOM", utf16Bytes("héllo\n😀", false, true)},
		{"BE with BOM", utf16Bytes("héllo\n😀", true, true)},
		{"LE empty body", utf16Bytes("", false, true)},
	} {
		got, err := Decode(tc.in)
		if err != nil {
			t.Fatalf("%s: Decode = %v", tc.name, err)
		}
		if back := Encode(got); !bytes.Equal(back, tc.in) {
			t.Errorf("%s: Encode(Decode(x)) = %x, want %x", tc.name, back, tc.in)
		}
	}
	le, _ := Decode(utf16Bytes("ab", false, true))
	be, _ := Decode(utf16Bytes("ab", true, true))
	if le.Body != "ab" || be.Body != "ab" || le.Encoding != EncodingUTF16LE || be.Encoding != EncodingUTF16BE {
		t.Errorf("byte order: LE %q/%s, BE %q/%s", le.Body, le.Encoding, be.Body, be.Encoding)
	}
	for name, in := range map[string][]byte{
		"odd length":        {0xFF, 0xFE, 'a', 0, 'b'},
		"lone high":         {0xFF, 0xFE, 0x00, 0xD8, 'a', 0},
		"lone low":          {0xFE, 0xFF, 0xDC, 0x00},
		"NUL character":     {0xFF, 0xFE, 'a', 0, 0, 0},
		"U+FFFD in a text":  {0xFF, 0xFE, 0xFD, 0xFF},
		"UTF-32 LE":         {0xFF, 0xFE, 0, 0, 'a', 0, 0, 0},
		"NUL in UTF-8 text": []byte("a\x00b"),
	} {
		if _, err := Decode(in); !errors.Is(err, ErrBinary) {
			t.Errorf("Decode(%s) = %v, want ErrBinary", name, err)
		}
	}
}
