package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"
)

func utf16Bytes(s string, bigEndian, bom bool) []byte {
	var u []uint16
	if bom {
		u = append(u, 0xFEFF)
	}
	u = append(u, utf16.Encode([]rune(s))...)
	out := make([]byte, 0, 2*len(u))
	for _, c := range u {
		if bigEndian {
			out = append(out, byte(c>>8), byte(c))
		} else {
			out = append(out, byte(c), byte(c>>8))
		}
	}
	return out
}

func mustDecode(t *testing.T, raw []byte) Text {
	t.Helper()
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode(%q) error: %v", raw, err)
	}
	return got
}

func TestDecodeEncode(t *testing.T) {
	t.Run("TXT-01 UTF-8 without BOM, LF round-trips byte for byte", func(t *testing.T) {
		raw := []byte("= Titre\nSuite é 漢 🙂\n")
		got := mustDecode(t, raw)
		if got.Body != string(raw) || got.Encoding != EncodingUTF8 || got.BOM || got.EOL != LF {
			t.Errorf("Decode = %+v", got)
		}
		if enc := Encode(got); !bytes.Equal(enc, raw) {
			t.Errorf("Encode = %q, want %q", enc, raw)
		}
	})

	t.Run("TXT-02 UTF-8 with BOM keeps the BOM out of the body and re-emits it", func(t *testing.T) {
		raw := append([]byte{0xEF, 0xBB, 0xBF}, "a\nb\n"...)
		got := mustDecode(t, raw)
		if !got.BOM || got.Body != "a\nb\n" || got.Encoding != EncodingUTF8 {
			t.Errorf("Decode = %+v", got)
		}
		if enc := Encode(got); !bytes.Equal(enc, raw) {
			t.Errorf("Encode = %q, want %q", enc, raw)
		}
	})

	t.Run("TXT-03 CRLF gives an LF body and is re-emitted", func(t *testing.T) {
		raw := []byte("a\r\nb\r\n")
		got := mustDecode(t, raw)
		if got.EOL != CRLF || got.Body != "a\nb\n" {
			t.Errorf("Decode = %+v", got)
		}
		if enc := Encode(got); !bytes.Equal(enc, raw) {
			t.Errorf("Encode = %q, want %q", enc, raw)
		}
		cr := mustDecode(t, []byte("a\rb\r"))
		if cr.EOL != CR || cr.Body != "a\nb\n" || string(Encode(cr)) != "a\rb\r" {
			t.Errorf("CR: Decode = %+v", cr)
		}
	})

	t.Run("TXT-04 mixed line endings: Mixed, LF body, majority on encode", func(t *testing.T) {
		got := mustDecode(t, []byte("a\r\nb\r\nc\nd\r\n"))
		if got.EOL != Mixed || got.Body != "a\nb\nc\nd\n" || got.Dominant != CRLF {
			t.Fatalf("Decode = %+v", got)
		}
		if enc := Encode(got); string(enc) != "a\r\nb\r\nc\r\nd\r\n" {
			t.Errorf("Encode = %q", enc)
		}
		if !got.MixedEOL() {
			t.Error("MixedEOL() = false, want the warning flag")
		}
		lf := mustDecode(t, []byte("a\nb\nc\r\n"))
		if lf.Dominant != LF || string(Encode(lf)) != "a\nb\nc\n" {
			t.Errorf("LF majority: %+v -> %q", lf, Encode(lf))
		}
	})

	t.Run("TXT-05 Windows-1252", func(t *testing.T) {
		raw := []byte("r\xe9pertoire \x80 \x93cit\xe9\x94\n")
		got := mustDecode(t, raw)
		if got.Encoding != EncodingWindows1252 || got.Body != "répertoire € “cité”\n" || got.BOM {
			t.Errorf("Decode = %+v", got)
		}
		if enc := Encode(got); !bytes.Equal(enc, raw) {
			t.Errorf("Encode = %q, want %q", enc, raw)
		}
		got.Body = "漢\n"
		if enc := Encode(got); string(enc) != "漢\n" {
			t.Errorf("text the encoding cannot hold: Encode = %q, want UTF-8", enc)
		}
	})

	t.Run("TXT-06 UTF-16 with BOM", func(t *testing.T) {
		for _, c := range []struct {
			name string
			raw  []byte
			enc  string
		}{
			{"LE", utf16Bytes("é𝄞\r\nx", false, true), EncodingUTF16LE},
			{"BE", utf16Bytes("é𝄞\r\nx", true, true), EncodingUTF16BE},
		} {
			got := mustDecode(t, c.raw)
			if got.Encoding != c.enc || !got.BOM || got.Body != "é𝄞\nx" || got.EOL != CRLF {
				t.Errorf("%s: Decode = %+v", c.name, got)
			}
			if enc := Encode(got); !bytes.Equal(enc, c.raw) {
				t.Errorf("%s: Encode = %x, want %x", c.name, enc, c.raw)
			}
			conv := Convert(got)
			if conv.Encoding != EncodingUTF8 || conv.BOM || conv.Body != got.Body || conv.EOL != CRLF {
				t.Errorf("%s: Convert = %+v", c.name, conv)
			}
			if enc := Encode(conv); string(enc) != "é𝄞\r\nx" {
				t.Errorf("%s: Encode(Convert) = %q", c.name, enc)
			}
		}
		if _, err := Decode([]byte{0xFF, 0xFE, 0x41}); !errors.Is(err, ErrBinary) {
			t.Errorf("odd-length UTF-16 error = %v, want ErrBinary", err)
		}
		if _, err := Decode([]byte{0xFF, 0xFE, 0x00, 0xD8, 0x41, 0x00}); !errors.Is(err, ErrBinary) {
			t.Errorf("lone surrogate error = %v, want ErrBinary", err)
		}
	})

	t.Run("TXT-07 binary", func(t *testing.T) {
		for name, raw := range map[string][]byte{
			"NUL":          []byte("ab\x00cd"),
			"PNG":          {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0x0d},
			"undefined C1": []byte("a\x81b"),
			"C0 control":   []byte("a\x01b\xe9"),
			"UTF-32 LE":    {0xFF, 0xFE, 0, 0, 'a', 0, 0, 0},
		} {
			if _, err := Decode(raw); !errors.Is(err, ErrBinary) {
				t.Errorf("%s: Decode error = %v, want ErrBinary", name, err)
			}
		}
	})

	t.Run("TXT-08 isolated invalid UTF-8 reports its line", func(t *testing.T) {
		_, err := Decode([]byte("= Titre\nbonjour é\nmauvais \xff ici\nfin\n"))
		if !errors.Is(err, ErrInvalidUTF8) {
			t.Fatalf("error = %v, want ErrInvalidUTF8", err)
		}
		bad, ok := errors.AsType[*InvalidUTF8Error](err)
		if !ok || bad.Line != 3 {
			t.Errorf("InvalidUTF8Error = %+v, %v; want line 3", bad, ok)
		}
		_, err = Decode([]byte("é\r\n\xfe"))
		bad, ok = errors.AsType[*InvalidUTF8Error](err)
		if !ok || bad.Line != 2 {
			t.Errorf("CRLF case: %+v, %v; want line 2", bad, ok)
		}
	})

	t.Run("TXT-09 NFD is kept as is", func(t *testing.T) {
		raw := []byte("e\u0301te\u0301 \U0001F468\u200D\U0001F469\n")
		got := mustDecode(t, raw)
		if got.Body != string(raw) {
			t.Errorf("Body = %q, want it unchanged", got.Body)
		}
		if enc := Encode(got); !bytes.Equal(enc, raw) {
			t.Errorf("Encode = %q", enc)
		}
	})

	t.Run("TXT-10 empty file", func(t *testing.T) {
		got := mustDecode(t, nil)
		if got.Body != "" || got.Encoding != EncodingUTF8 || got.BOM || got.EOL != SystemEOL() {
			t.Errorf("Decode(nil) = %+v", got)
		}
		if enc := Encode(got); len(enc) != 0 {
			t.Errorf("Encode = %q, want empty", enc)
		}
		if enc := Encode(Text{Body: "a\nb"}); string(enc) != "a\nb" {
			t.Errorf("zero-value Text encodes %q, want UTF-8 LF", enc)
		}
	})

	t.Run("5 MB CRLF file round-trips", func(t *testing.T) {
		raw := []byte(strings.Repeat("ligne de texte é\r\n", 5<<20/19))
		if enc := Encode(mustDecode(t, raw)); !bytes.Equal(enc, raw) {
			t.Error("5 MB CRLF file does not round-trip")
		}
	})
}

// TXT-11: Decode never panics, and Encode(Decode(x)) == x for UTF-8 input with
// consistent line endings.
func FuzzDecode(f *testing.F) {
	for _, s := range []string{
		"", "a", "a\nb", "a\r\nb\r\n", "a\rb", "\r\r\n\n", "é", "\xef\xbb\xbfx", "\xef\xbb\xbf\xef\xbb\xbf",
		"r\xe9", "\xff\xfea\x00", "\xfe\xff\x00a", "\x00", "é\xff",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		got, err := Decode(raw)
		if err != nil {
			if !errors.Is(err, ErrBinary) && !errors.Is(err, ErrInvalidUTF8) {
				t.Fatalf("Decode(%q) unexpected error %v", raw, err)
			}
			return
		}
		enc := Encode(got)
		if got.Encoding == EncodingUTF8 && got.EOL != Mixed && !bytes.Equal(enc, raw) {
			t.Fatalf("Encode(Decode(%q)) = %q", raw, enc)
		}
		again, err := Decode(enc)
		if err != nil || again.Body != got.Body {
			t.Fatalf("re-decode of %q = %+v, %v; want body %q", enc, again, err, got.Body)
		}
	})
}
