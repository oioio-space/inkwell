package project

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// outsideEdit replaces main.typ as another program would and acknowledges it.
func outsideEdit(t *testing.T, p *Project, raw []byte) {
	t.Helper()
	if err := p.root.WriteFile(MainFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.Acknowledge(MainFile); err != nil {
		t.Fatal(err)
	}
}

func onDisk(t *testing.T, p *Project) []byte {
	t.Helper()
	b, err := p.root.ReadFile(MainFile)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProjectionKeepsTheFileFormat(t *testing.T) {
	for _, c := range []struct {
		name string
		raw  []byte
		body string
		want []byte
	}{
		{"UTF-8 BOM and CRLF", append([]byte{0xEF, 0xBB, 0xBF}, "a\r\nb\r\n"...), "a\nb\nc\n", append([]byte{0xEF, 0xBB, 0xBF}, "a\r\nb\r\nc\r\n"...)},
		{"Windows-1252", []byte("r\xe9p\n"), "répertoire €\n", []byte("r\xe9pertoire \x80\n")},
		{"UTF-16 LE", utf16Bytes("a\nb", false, true), "a\nb\nc", utf16Bytes("a\nb\nc", false, true)},
		{"CR", []byte("a\rb"), "x\ny", []byte("x\ry")},
		{"mixed becomes the majority", []byte("a\r\nb\r\nc\n"), "1\n2\n", []byte("1\r\n2\r\n")},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := create(t, newBase(t), "p")
			outsideEdit(t, p, c.raw)

			read, err := p.ReadProjection(MainFile)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(read), mustDecode(t, c.raw).Body; got != want {
				t.Errorf("ReadProjection = %q, want the LF UTF-8 body %q", got, want)
			}
			if err := p.WriteProjection(MainFile, []byte(c.body)); err != nil {
				t.Fatal(err)
			}
			if got := onDisk(t, p); string(got) != string(c.want) {
				t.Errorf("on disk = %q, want %q", got, c.want)
			}
			// The recorded hash is that of the encoded file: no false external edit.
			if err := p.WriteProjection(MainFile, []byte(c.body)); err != nil {
				t.Errorf("second WriteProjection: %v", err)
			}
		})
	}
}

func TestReadTextAndNonText(t *testing.T) {
	p := create(t, newBase(t), "p")
	outsideEdit(t, p, []byte("a\r\nb"))
	got, err := p.ReadText(MainFile)
	if err != nil || got.Body != "a\nb" || got.EOL != CRLF || got.Encoding != EncodingUTF8 {
		t.Errorf("ReadText = %+v, %v", got, err)
	}

	outsideEdit(t, p, []byte("\x89PNG\x00"))
	if _, err := p.ReadProjection(MainFile); !errors.Is(err, ErrBinary) {
		t.Errorf("ReadProjection(binary) = %v, want ErrBinary", err)
	}
	outsideEdit(t, p, []byte("é ok\nbad \xff\n"))
	if _, err := p.ReadProjection(MainFile); !errors.Is(err, ErrInvalidUTF8) {
		t.Errorf("ReadProjection(invalid UTF-8) = %v, want ErrInvalidUTF8", err)
	}
	// A file Inkwell cannot decode is replaced as plain UTF-8 once acknowledged.
	if err := p.WriteProjection(MainFile, []byte("é\n")); err != nil {
		t.Fatal(err)
	}
	if got := onDisk(t, p); string(got) != "é\n" {
		t.Errorf("on disk = %q", got)
	}
}

// A write of what the file already holds leaves the file alone: nothing to replace, so nothing
// to clobber if another program is saving it at that very moment.
func TestWriteProjectionSkipsAnUnchangedFile(t *testing.T) {
	p := create(t, newBase(t), "p")
	if err := p.WriteProjection(MainFile, []byte("= a")); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := p.root.Chtimes(MainFile, old, old); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection(MainFile, []byte("= a")); err != nil {
		t.Fatal(err)
	}
	info, err := p.root.Stat(MainFile)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("an identical write replaced the file (mtime %v)", info.ModTime())
	}
	if err := p.WriteProjection(MainFile, []byte("= b")); err != nil {
		t.Fatal(err)
	}
	if got := string(onDisk(t, p)); got != "= b" {
		t.Errorf("on disk = %q, want the new text", got)
	}
}

// The webview tells a refused write from an outside edit (the notice asks the person) from any
// other failure (a banner) by this wording of the error it receives: it is a contract.
func TestErrExternalEditWording(t *testing.T) {
	if got := ErrExternalEdit.Error(); !strings.Contains(got, "edited outside Inkwell") {
		t.Errorf("ErrExternalEdit = %q, frontend/src/workspace isExternalEdit matches %q", got, "edited outside Inkwell")
	}
}

func TestNewFileIsUTF8(t *testing.T) {
	p := create(t, newBase(t), "p")
	if err := p.WriteProjection(MainFile, []byte("é\nx\n")); err != nil {
		t.Fatal(err)
	}
	want := string(Encode(Text{Body: "é\nx\n", EOL: SystemEOL()}))
	if got := onDisk(t, p); string(got) != want {
		t.Errorf("on disk = %q, want %q", got, want)
	}
}
