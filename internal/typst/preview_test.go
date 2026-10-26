package typst

import (
	"strings"
	"testing"
)

const threePages = "= Titre un <a>\nBonjour *monde*.\n#pagebreak()\n== Deux\nSuite du texte.\n#pagebreak()\n= Trois\nFin.\n"

func TestPageCount(t *testing.T) {
	e := newEngine(t, nil)
	p := writeProject(t, map[string]string{"main.typ": threePages})

	t.Run("PRV-01 the page count of the PDF is the document's, whatever window was rendered", func(t *testing.T) {
		res, err := e.Compile(t.Context(), p, "main.typ", WithPages(2, 2))
		if err != nil {
			t.Fatalf("Compile = %v", err)
		}
		n, err := PageCount(res.PDF)
		if err != nil {
			t.Fatalf("PageCount = %v", err)
		}
		if n != 3 || len(res.Pages) != 1 {
			t.Errorf("PageCount = %d with %d SVG pages, want 3 and 1", n, len(res.Pages))
		}
	})

	t.Run("PRV-02 a window past the end of a shorter document is empty, not an error", func(t *testing.T) {
		res, err := e.Compile(t.Context(), p, "main.typ", WithPages(7, 9))
		if err != nil {
			t.Fatalf("Compile = %v", err)
		}
		if len(res.Pages) != 0 || len(res.PDF) == 0 {
			t.Errorf("got %d pages and %d PDF bytes, want no page and a PDF", len(res.Pages), len(res.PDF))
		}
	})

	t.Run("PRV-03 not a PDF, and a PDF without a page tree, are refused", func(t *testing.T) {
		for _, in := range []string{"", "hello", "%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\n"} {
			if n, err := PageCount([]byte(in)); err == nil {
				t.Errorf("PageCount(%q) = %d, want an error", in, n)
			}
		}
	})
}

func FuzzPageCount(f *testing.F) {
	f.Add([]byte("%PDF-1.7\n1 0 obj\n<< /Type /Pages /Count 2 /Kids [] >>\nendobj\n"))
	f.Add([]byte("%PDF-"))
	f.Fuzz(func(t *testing.T, in []byte) {
		n, err := PageCount(in)
		if err == nil && n < 0 {
			t.Fatalf("PageCount = %d", n)
		}
	})
}

func TestAnchors(t *testing.T) {
	e := newEngine(t, nil)
	p := writeProject(t, map[string]string{"main.typ": threePages})

	t.Run("PRV-04 headings and paragraphs come with their page and position, in order", func(t *testing.T) {
		got, err := e.Anchors(t.Context(), p, "main.typ")
		if err != nil {
			t.Fatalf("Anchors = %v", err)
		}
		want := []struct {
			heading bool
			page    int
			text    string
		}{
			{true, 1, "Titre un"},
			{false, 1, "Bonjour monde."},
			{true, 2, "Deux"},
			{false, 2, "Suite du texte."},
			{true, 3, "Trois"},
			{false, 3, "Fin."},
		}
		if len(got) != len(want) {
			t.Fatalf("%d anchors, want %d: %+v", len(got), len(want), got)
		}
		for i, w := range want {
			a := got[i]
			if a.Heading != w.heading || a.Page != w.page || a.Text != w.text || a.X <= 0 || a.Y <= 0 {
				t.Errorf("anchor %d = %+v, want heading=%v page=%d text=%q at a positive position", i, a, w.heading, w.page, w.text)
			}
		}
		if got[1].Y <= got[0].Y {
			t.Errorf("the paragraph (y=%v) is not below its heading (y=%v)", got[1].Y, got[0].Y)
		}
	})

	t.Run("PRV-05 a document with errors reports them", func(t *testing.T) {
		bad := writeProject(t, map[string]string{"main.typ": "= Titre\n#unknown()\n"})
		_, err := e.Anchors(t.Context(), bad, "main.typ")
		if err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Errorf("Anchors = %v, want a compile error naming the unknown function", err)
		}
	})

	t.Run("PRV-06 nonsense positions and long texts are cleaned", func(t *testing.T) {
		long := strings.Repeat("é", 500)
		got, err := decodeAnchors([]byte(`[{"k":"p","page":0,"x":1,"y":1,"text":"a"},{"k":"p","page":1,"x":-1,"y":1,"text":"b"},{"k":"h","page":2,"x":3,"y":4,"text":"` + long + `"}]`))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !got[0].Heading || got[0].Page != 2 || len([]rune(got[0].Text)) != maxAnchorText {
			t.Errorf("decodeAnchors = %+v, want the one valid anchor with its text cut", got)
		}
		if _, err := decodeAnchors([]byte(`{"not":"an array"}`)); err == nil {
			t.Error("decodeAnchors accepted an object")
		}
	})
}
