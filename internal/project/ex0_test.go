package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEX0ListReadsNoContent: listing reads no file content (ADR 0027 rule 10).
// A file of an unknown extension that cannot even be opened is listed with its
// size, and no content open happens during List (the open counter is the
// proof: as root, mode 000 would not stop an open, but none is made).
func TestEX0ListReadsNoContent(t *testing.T) {
	p := create(t, newBase(t), "p")
	put(t, p, "notes.unknownext", "some bytes")
	if err := os.Chmod(filepath.Join(p.root.Name(), "notes.unknownext"), 0); err != nil {
		t.Fatal(err)
	}

	var opened []string
	hook := func(name string) { opened = append(opened, name) }
	openHook.Store(&hook)
	t.Cleanup(func() { openHook.Store(nil) })

	files, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(opened) != 0 {
		t.Errorf("List opened files %q, want none", opened)
	}
	var got *File
	for i := range files {
		if files[i].Path == "notes.unknownext" {
			got = &files[i]
		}
	}
	if got == nil || got.Size != int64(len("some bytes")) {
		t.Errorf("List = %+v, want notes.unknownext of %d bytes", files, len("some bytes"))
	}
}

// TestEX0SniffKeepsACutRune: the 4 KiB prefix that tells text from binary may
// cut a UTF-8 rune; the cut bytes are not taken for binary ones (EX0 code
// review, finding 1). U+2041 is E2 81 81, and 0x81 has no Windows-1252 meaning.
func TestEX0SniffKeepsACutRune(t *testing.T) {
	p := create(t, newBase(t), "p")
	body := strings.Repeat("a", sniffLen-2) + "⁁" + strings.Repeat("b", 100)
	put(t, p, "cut.txt", body)
	got, err := p.ReadProjectText("cut.txt")
	if err != nil || got.Body != body {
		t.Fatalf("ReadProjectText = %d bytes, %v; want the %d-byte text", len(got.Body), err, len(body))
	}
}
