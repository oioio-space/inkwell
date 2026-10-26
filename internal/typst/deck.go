package typst

import (
	"context"
	"encoding/json"
	"errors"
	"math"
)

const (
	// maxDeckNote bounds the speaker notes kept for one page, in bytes.
	maxDeckNote = 16 << 10
	// maxDeckMinutes bounds the planned duration (a day).
	maxDeckMinutes = 24 * 60
	// deckSelector is the label Touying puts on the metadata that describes the
	// slides (the pdfpc file): `typst query` reads it back.
	deckSelector = "<pdfpc-file>"
)

// DeckPage is one compiled page seen as part of a presentation.
type DeckPage struct {
	// Slide is the 1-based number of the slide the page belongs to: the pages
	// that Touying makes of one slide (`#pause`, `#uncover`...) share it.
	Slide int `json:"slide"`
	// Step is the 0-based step of the slide that the page shows.
	Step int `json:"step"`
	// Hidden is true for a page that a presentation skips.
	Hidden bool `json:"hidden,omitzero"`
	// Note is the speaker note written on the page, if any.
	Note string `json:"note,omitzero"`
}

// Deck describes the pages of a document as slides and steps, from the
// metadata that Touying writes for pdfpc. Pages[i] is page i+1.
type Deck struct {
	// Pages has one entry per page of the document, in order. It is empty when
	// the document is not a Touying presentation.
	Pages []DeckPage `json:"pages,omitzero"`
	// Minutes is the planned duration of the talk (`pdfpc.config(duration-minutes:)`); 0 if unset.
	Minutes int `json:"minutes,omitzero"`
}

// Deck reads how the document divides into slides and steps. A document that
// is not a Touying presentation gives an empty deck, not an error.
//
// It has the same limits and the same errors as [Engine.Compile].
func (e *Engine) Deck(ctx context.Context, p Project, main string, opts ...Option) (*Deck, error) {
	raw, err := e.Query(ctx, p, main, deckSelector, opts...)
	if err != nil {
		return nil, err
	}
	return DecodeDeck(raw)
}

// DecodeDeck reads the JSON that `typst query <pdfpc-file>` prints. It is
// bounded: a page index outside [0, [DefaultMaxPages]) is dropped, notes are
// cut, and anything unexpected gives an error rather than a guess.
func DecodeDeck(b []byte) (*Deck, error) {
	var elems []struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(b, &elems); err != nil {
		return nil, errors.New("typst: deck: the result is not the expected JSON")
	}
	if len(elems) == 0 {
		return &Deck{}, nil
	}
	var v struct {
		Pages []struct {
			Idx     int    `json:"idx"`
			Label   string `json:"label"`
			Overlay int    `json:"overlay"`
			Hidden  bool   `json:"hidden"`
			Note    string `json:"note"`
		} `json:"pages"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(elems[0].Value, &v); err != nil {
		return nil, errors.New("typst: deck: the metadata is not the expected JSON")
	}
	d := &Deck{}
	if m := v.Duration; m >= 1 && m <= maxDeckMinutes && !math.IsNaN(m) {
		d.Minutes = int(m)
	}
	if len(v.Pages) == 0 {
		return d, nil
	}
	n := 0
	for _, p := range v.Pages {
		if p.Idx >= 0 && p.Idx < DefaultMaxPages {
			n = max(n, p.Idx+1)
		}
	}
	d.Pages = make([]DeckPage, n)
	labels := make([]string, n)
	known := make([]bool, n)
	for _, p := range v.Pages {
		if p.Idx < 0 || p.Idx >= n {
			continue
		}
		known[p.Idx] = true
		labels[p.Idx] = p.Label
		d.Pages[p.Idx] = DeckPage{
			Step:   min(max(p.Overlay, 0), DefaultMaxPages),
			Hidden: p.Hidden,
			Note:   clipBytes(p.Note, maxDeckNote),
		}
	}
	// A new slide starts at step 0 or when the label changes; a page the
	// metadata does not mention (it should not happen) is a slide of its own.
	slide := 0
	for i := range d.Pages {
		if i == 0 || !known[i] || d.Pages[i].Step == 0 || labels[i] != labels[i-1] {
			slide++
		}
		d.Pages[i].Slide = slide
	}
	return d, nil
}

// clipBytes cuts s to at most n bytes, on a character boundary.
func clipBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 { // inside a multi-byte character
		n--
	}
	return s[:n]
}
