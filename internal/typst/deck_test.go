package typst

import (
	"slices"
	"strings"
	"testing"
)

func TestDeckOfTouyingFixture(t *testing.T) {
	e := newEngine(t, nil)
	p := writeProject(t, map[string]string{"main.typ": slidesFixture})
	got, err := e.Deck(t.Context(), p, "main.typ")
	if err != nil {
		t.Fatalf("Deck() = %v", err)
	}
	want := []DeckPage{
		{Slide: 1, Step: 0},
		{Slide: 2, Step: 0},
		{Slide: 2, Step: 1, Note: "Remember to smile"},
		{Slide: 3, Step: 0},
	}
	if !slices.Equal(got.Pages, want) {
		t.Errorf("Deck().Pages = %+v, want %+v", got.Pages, want)
	}
}

func TestDeckOfPlainDocumentIsEmpty(t *testing.T) {
	e := newEngine(t, nil)
	p := writeProject(t, map[string]string{"main.typ": "= Just a page\n"})
	got, err := e.Deck(t.Context(), p, "main.typ")
	if err != nil {
		t.Fatalf("Deck() = %v", err)
	}
	if len(got.Pages) != 0 || got.Minutes != 0 {
		t.Errorf("Deck() = %+v, want empty", got)
	}
}

func TestDeckDuration(t *testing.T) {
	e := newEngine(t, nil)
	src := strings.Replace(slidesFixture, "= Title slide", "#pdfpc.config(duration-minutes: 25)\n\n= Title slide", 1)
	p := writeProject(t, map[string]string{"main.typ": src})
	got, err := e.Deck(t.Context(), p, "main.typ")
	if err != nil {
		t.Fatalf("Deck() = %v", err)
	}
	if got.Minutes != 25 {
		t.Errorf("Deck().Minutes = %d, want 25", got.Minutes)
	}
}

func TestDecodeDeck(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []DeckPage
		mins int
		bad  bool
	}{
		{name: "no metadata", in: `[]`},
		{name: "not json", in: `nope`, bad: true},
		{name: "wrong shape", in: `[{"value":3}]`, bad: true},
		{
			name: "slides and steps",
			in:   `[{"value":{"duration":30,"pages":[{"idx":0,"label":"1","overlay":0},{"idx":1,"label":"2","overlay":0},{"idx":2,"label":"2","overlay":1,"note":"n"}]}}]`,
			want: []DeckPage{{Slide: 1}, {Slide: 2}, {Slide: 2, Step: 1, Note: "n"}},
			mins: 30,
		},
		{
			name: "hidden slide",
			in:   `[{"value":{"pages":[{"idx":0,"label":"1","overlay":0,"hidden":true}]}}]`,
			want: []DeckPage{{Slide: 1, Hidden: true}},
		},
		{
			name: "starts on a later step",
			in:   `[{"value":{"pages":[{"idx":0,"label":"1","overlay":2}]}}]`,
			want: []DeckPage{{Slide: 1, Step: 2}},
		},
		{
			name: "label change without step 0",
			in:   `[{"value":{"pages":[{"idx":0,"label":"1","overlay":0},{"idx":1,"label":"2","overlay":1}]}}]`,
			want: []DeckPage{{Slide: 1}, {Slide: 2, Step: 1}},
		},
		{
			name: "page index out of bounds is dropped",
			in:   `[{"value":{"pages":[{"idx":-1,"label":"1"},{"idx":99999999,"label":"1"},{"idx":0,"label":"1"}]}}]`,
			want: []DeckPage{{Slide: 1}},
		},
		{
			name: "gap becomes a slide of its own",
			in:   `[{"value":{"pages":[{"idx":0,"label":"1"},{"idx":2,"label":"1","overlay":1}]}}]`,
			want: []DeckPage{{Slide: 1}, {Slide: 2}, {Slide: 3, Step: 1}},
		},
		{
			name: "duration out of range is ignored",
			in:   `[{"value":{"duration":99999,"pages":[{"idx":0,"label":"1"}]}}]`,
			want: []DeckPage{{Slide: 1}},
		},
		{
			name: "negative step is zero",
			in:   `[{"value":{"pages":[{"idx":0,"label":"1","overlay":-4}]}}]`,
			want: []DeckPage{{Slide: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeDeck([]byte(tt.in))
			if (err != nil) != tt.bad {
				t.Fatalf("DecodeDeck(%s) error = %v, want error: %v", tt.in, err, tt.bad)
			}
			if err != nil {
				return
			}
			if !slices.Equal(got.Pages, tt.want) || got.Minutes != tt.mins {
				t.Errorf("DecodeDeck(%s) = %+v minutes %d, want %+v minutes %d", tt.in, got.Pages, got.Minutes, tt.want, tt.mins)
			}
		})
	}
}

func TestDecodeDeckCutsLongNotes(t *testing.T) {
	note := strings.Repeat("é", maxDeckNote) // two bytes each
	in := `[{"value":{"pages":[{"idx":0,"label":"1","note":"` + note + `"}]}}]`
	got, err := DecodeDeck([]byte(in))
	if err != nil {
		t.Fatalf("DecodeDeck() = %v", err)
	}
	if n := len(got.Pages[0].Note); n > maxDeckNote || n < maxDeckNote-1 {
		t.Errorf("note length = %d, want about %d", n, maxDeckNote)
	}
	if !strings.HasSuffix(got.Pages[0].Note, "é") {
		t.Error("the note was cut inside a character")
	}
}

func FuzzDecodeDeck(f *testing.F) {
	f.Add([]byte(`[]`))
	f.Add([]byte(`[{"value":{"duration":30,"pages":[{"idx":0,"label":"1","overlay":0,"note":"n"}]}}]`))
	f.Add([]byte(`[{"value":{"pages":[{"idx":5,"label":"","overlay":9,"hidden":true}]}}]`))
	f.Fuzz(func(t *testing.T, b []byte) {
		d, err := DecodeDeck(b)
		if err != nil {
			return
		}
		if len(d.Pages) > DefaultMaxPages {
			t.Fatalf("%d pages, over the bound", len(d.Pages))
		}
		prev := 0
		for i, p := range d.Pages {
			if p.Slide < 1 || p.Slide < prev || p.Slide > prev+1 || p.Step < 0 || len(p.Note) > maxDeckNote {
				t.Fatalf("page %d = %+v after slide %d", i+1, p, prev)
			}
			prev = p.Slide
		}
	})
}
