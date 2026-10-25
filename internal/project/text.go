package project

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// The encodings [Decode] recognizes, as reported in [Text.Encoding].
const (
	EncodingUTF8        = "utf-8"
	EncodingUTF16LE     = "utf-16le"
	EncodingUTF16BE     = "utf-16be"
	EncodingWindows1252 = "windows-1252"
)

var (
	// ErrBinary is returned by [Decode] for content that is not text: it has
	// NUL bytes, control bytes no text encoding uses, or broken UTF-16.
	ErrBinary = errors.New("project: binary content")
	// ErrInvalidUTF8 is returned by [Decode] for a file that is UTF-8 apart
	// from some invalid bytes. The error is an [*InvalidUTF8Error] that says
	// where; Inkwell never rewrites such a file silently.
	ErrInvalidUTF8 = errors.New("project: invalid UTF-8")
)

// InvalidUTF8Error is the concrete error behind [ErrInvalidUTF8].
type InvalidUTF8Error struct {
	// Line is the 1-based line of the first invalid byte, counting LF, CRLF
	// and lone CR as line ends.
	Line int
}

func (e *InvalidUTF8Error) Error() string {
	return fmt.Sprintf("%v at line %d", ErrInvalidUTF8, e.Line)
}

// Is reports whether target is [ErrInvalidUTF8].
func (e *InvalidUTF8Error) Is(target error) bool { return target == ErrInvalidUTF8 }

// EOL is a line ending convention.
type EOL uint8

// The line endings. The zero value is LF.
const (
	// LF is "\n".
	LF EOL = iota
	// CRLF is "\r\n".
	CRLF
	// CR is "\r" alone (classic Mac).
	CR
	// Mixed means the file uses several; see [Text.Dominant].
	Mixed
)

// String returns "LF", "CRLF", "CR" or "Mixed".
func (e EOL) String() string {
	switch e {
	case LF:
		return "LF"
	case CRLF:
		return "CRLF"
	case CR:
		return "CR"
	case Mixed:
		return "Mixed"
	}
	return fmt.Sprintf("EOL(%d)", uint8(e))
}

func (e EOL) sequence() string {
	switch e {
	case CRLF:
		return "\r\n"
	case CR:
		return "\r"
	}
	return "\n"
}

// SystemEOL is the line ending of a file Inkwell creates: CRLF on Windows,
// LF elsewhere.
func SystemEOL() EOL {
	if runtime.GOOS == "windows" {
		return CRLF
	}
	return LF
}

// Text is the content of a text file as the editor sees it, with what is
// needed to write it back as it was found. The zero value is an empty UTF-8
// text with LF line ends.
type Text struct {
	// Body is the text with LF line ends and without BOM. Its Unicode
	// normalization is never changed (NFD from macOS stays NFD).
	Body string
	// Encoding is one of the Encoding constants; empty means UTF-8.
	Encoding string
	// EOL is the file's line ending, or [Mixed].
	EOL EOL
	// Dominant is the most frequent line ending of a [Mixed] file (LF on a
	// tie, then CRLF), the one [Encode] writes. Unused otherwise.
	Dominant EOL
	// BOM tells that the file started with a byte order mark.
	BOM bool
}

// MixedEOL reports that the file mixed line endings, which [Encode] unifies:
// the interface warns about it.
func (t Text) MixedEOL() bool { return t.EOL == Mixed }

// LineEnding returns the line ending [Encode] writes.
func (t Text) LineEnding() EOL {
	if t.EOL == Mixed {
		return t.Dominant
	}
	return t.EOL
}

// Convert returns t as UTF-8 without BOM, keeping the text and line endings:
// the offer made for a file in another encoding.
func Convert(t Text) Text {
	t.Encoding = EncodingUTF8
	t.BOM = false
	return t
}

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
)

// Decode reads the content of a text file. The encoding comes from the BOM
// (UTF-8, UTF-16 LE or BE), else from the bytes: valid UTF-8, or
// Windows-1252 when the bytes are not UTF-8 at all. It returns [ErrBinary]
// for binary content and an [*InvalidUTF8Error] (matching [ErrInvalidUTF8])
// for a UTF-8 file with isolated invalid bytes.
func Decode(raw []byte) (Text, error) {
	switch {
	case len(raw) == 0:
		return Text{Body: "", Encoding: EncodingUTF8, EOL: SystemEOL()}, nil
	case bytes.HasPrefix(raw, bomUTF8):
		return decodeUTF8(raw[len(bomUTF8):], true)
	case bytes.HasPrefix(raw, bomUTF32LE):
		return Text{}, fmt.Errorf("%w: UTF-32", ErrBinary)
	case bytes.HasPrefix(raw, bomUTF16LE):
		return decodeUTF16(raw[2:], EncodingUTF16LE)
	case bytes.HasPrefix(raw, bomUTF16BE):
		return decodeUTF16(raw[2:], EncodingUTF16BE)
	}
	return decodeUTF8(raw, false)
}

func decodeUTF8(b []byte, bom bool) (Text, error) {
	if bytes.IndexByte(b, 0) >= 0 {
		return Text{}, fmt.Errorf("%w: NUL byte", ErrBinary)
	}
	if utf8.Valid(b) {
		return newText(string(b), EncodingUTF8, bom), nil
	}
	bad, multibyte := scanUTF8(b)
	if bom || multibyte {
		return Text{}, &InvalidUTF8Error{Line: lineAt(b, bad)}
	}
	body, ok := decodeWindows1252(b)
	if !ok {
		return Text{}, fmt.Errorf("%w: control bytes", ErrBinary)
	}
	return newText(body, EncodingWindows1252, false), nil
}

// scanUTF8 returns the offset of the first invalid byte of b and whether b
// holds any valid multi-byte sequence (which a Windows-1252 text almost never
// does).
func scanUTF8(b []byte) (bad int, multibyte bool) {
	bad = -1
	for i := 0; i < len(b); {
		r, n := utf8.DecodeRune(b[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			if bad < 0 {
				bad = i
			}
		case n > 1:
			multibyte = true
		}
		i += n
	}
	return bad, multibyte
}

// lineAt returns the 1-based line of offset off in b.
func lineAt(b []byte, off int) int {
	line := 1
	for i := 0; i < off && i < len(b); i++ {
		switch b[i] {
		case '\n':
			line++
		case '\r':
			if i+1 >= len(b) || b[i+1] != '\n' {
				line++
			}
		}
	}
	return line
}

func decodeUTF16(b []byte, encoding string) (Text, error) {
	if len(b)%2 != 0 {
		return Text{}, fmt.Errorf("%w: odd UTF-16 length", ErrBinary)
	}
	units := make([]uint16, len(b)/2)
	for i := range units {
		if encoding == EncodingUTF16BE {
			units[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			units[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	runes := utf16.Decode(units)
	for _, r := range runes {
		if r == utf8.RuneError || r == 0 {
			// utf16.Decode maps an unpaired surrogate to U+FFFD; a real
			// U+FFFD in a text is rare enough to treat the file as damaged.
			return Text{}, fmt.Errorf("%w: invalid UTF-16", ErrBinary)
		}
	}
	return newText(string(runes), encoding, true), nil
}

// newText normalizes the line ends of s to LF and records which they were.
func newText(s, encoding string, bom bool) Text {
	t := Text{Encoding: encoding, BOM: bom}
	if !strings.ContainsRune(s, '\r') {
		t.Body = s
		t.EOL = SystemEOL()
		if strings.ContainsRune(s, '\n') {
			t.EOL = LF
		}
		return t
	}
	var lf, crlf, cr int
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\r' && i+1 < len(s) && s[i+1] == '\n':
			crlf++
			i++
			sb.WriteByte('\n')
		case c == '\r':
			cr++
			sb.WriteByte('\n')
		case c == '\n':
			lf++
			sb.WriteByte('\n')
		default:
			sb.WriteByte(c)
		}
	}
	t.Body = sb.String()
	kinds := 0
	for _, n := range []int{lf, crlf, cr} {
		if n > 0 {
			kinds++
		}
	}
	switch {
	case kinds > 1:
		t.EOL = Mixed
		t.Dominant = LF
		best := lf
		if crlf > best {
			t.Dominant, best = CRLF, crlf
		}
		if cr > best {
			t.Dominant = CR
		}
	case crlf > 0:
		t.EOL = CRLF
	case cr > 0:
		t.EOL = CR
	}
	return t
}

// Encode returns the bytes of t in its own encoding, with its BOM and its
// line ending (the dominant one for [Mixed]). A Body that Windows-1252 cannot
// hold is written as UTF-8 instead, so nothing is lost.
func Encode(t Text) []byte {
	body := t.Body
	if eol := t.LineEnding().sequence(); eol != "\n" {
		body = strings.ReplaceAll(body, "\n", eol)
	}
	switch t.Encoding {
	case EncodingUTF16LE, EncodingUTF16BE:
		return encodeUTF16(body, t.Encoding == EncodingUTF16BE, t.BOM)
	case EncodingWindows1252:
		if b, ok := encodeWindows1252(body); ok {
			return b
		}
		t.BOM = false
	}
	out := make([]byte, 0, len(bomUTF8)+len(body))
	if t.BOM {
		out = append(out, bomUTF8...)
	}
	return append(out, body...)
}

func encodeUTF16(body string, bigEndian, bom bool) []byte {
	units := utf16.Encode([]rune(body))
	out := make([]byte, 0, 2*(len(units)+1))
	put := func(u uint16) {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	if bom {
		put(0xFEFF)
	}
	for _, u := range units {
		put(u)
	}
	return out
}

// cp1252High maps the bytes 0x80-0x9F of Windows-1252 to their characters;
// 0 marks the five bytes the code page leaves undefined.
var cp1252High = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

// decodeWindows1252 converts b; ok is false if b holds a byte no text uses:
// a control byte other than tab, line feed, form feed and carriage return,
// DEL, or one of the five undefined bytes.
func decodeWindows1252(b []byte) (string, bool) {
	var sb strings.Builder
	sb.Grow(len(b) + len(b)/8)
	for _, c := range b {
		switch {
		case c < 0x20 && c != '\t' && c != '\n' && c != '\r' && c != '\f', c == 0x7F:
			return "", false
		case c >= 0x80 && c < 0xA0:
			r := cp1252High[c-0x80]
			if r == 0 {
				return "", false
			}
			sb.WriteRune(r)
		default:
			sb.WriteRune(rune(c))
		}
	}
	return sb.String(), true
}

// encodeWindows1252 converts s; ok is false if s has a character the code
// page cannot hold.
func encodeWindows1252(s string) ([]byte, bool) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80, r >= 0xA0 && r <= 0xFF:
			out = append(out, byte(r))
		default:
			i := slices.Index(cp1252High[:], r)
			if i < 0 {
				return nil, false
			}
			out = append(out, byte(0x80+i))
		}
	}
	return out, true
}
