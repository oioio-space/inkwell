package typst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

const (
	// maxAnchors bounds the anchors kept; maxAnchorText the characters of one text.
	maxAnchors    = 20_000
	maxAnchorText = 120
	maxAnchorOut  = 8 << 20
)

// Anchor is a heading or a paragraph of the laid-out document, with where it
// landed: what the preview and the source use to find each other. The page is
// 1-based; X and Y are in points from the top left of the page, the unit of
// the SVG pages.
type Anchor struct {
	// Heading is true for a heading, false for a paragraph.
	Heading bool `json:"heading,omitzero"`
	// Page is the 1-based page.
	Page int     `json:"page"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	// Text is the plain text of the element, cut after maxAnchorText characters.
	Text string `json:"text"`
}

// anchorsCode is the Typst code run by `typst eval` in the context of the
// document. It is constant: nothing of the project is spliced into it.
const anchorsCode = `{
let flat(c) = {
  if type(c) == str { c }
  else if c.func() == text { c.text }
  else if repr(c.func()) == "space" { " " }
  else if c.has("children") { c.children.map(flat).join("", default: "") }
  else if c.has("body") { flat(c.body) }
  else if c.has("child") { flat(c.child) }
  else { "" }
}
query(selector(heading).or(par)).map(e => (
  k: if e.func() == heading { "h" } else { "p" },
  page: e.location().page(),
  x: e.location().position().x.pt(),
  y: e.location().position().y.pt(),
  text: flat(e.body),
))
}`

// Anchors lays the document out and lists its headings and paragraphs in
// order, each with its page and position. Matching their text with the source
// gives the jump between the source and the preview.
//
// It has the same limits and the same errors as [Engine.Compile].
func (e *Engine) Anchors(ctx context.Context, p Project, main string, opts ...Option) ([]Anchor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o := applyOptions(opts)
	release, err := e.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	j, err := e.prepare(p, main)
	if err != nil {
		return nil, err
	}
	defer j.cleanup()

	runCtx, cancel := e.deadline(ctx)
	defer cancel()
	out, err := e.run(runCtx, j, e.args("eval", j, o, "--in", filepath.ToSlash(j.main), "--format", "json", "--", anchorsCode), maxAnchorOut, "")
	if err != nil {
		if runCtx.Err() != nil {
			return nil, ended(runCtx, e.cfg.Timeout)
		}
		return nil, err
	}
	if out.ExitCode != 0 {
		res := &Result{}
		if err := e.settle(res, j, []sidecar.Output{out}); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("typst: eval: the compiler exited with status %d", out.ExitCode)
	}
	if out.Truncated {
		return nil, fmt.Errorf("%w: the anchors exceed %d bytes", ErrOutputTooLarge, maxAnchorOut)
	}
	return decodeAnchors(out.Stdout)
}

// decodeAnchors reads the JSON of anchorsCode, bounded, dropping what makes no
// sense (a page below 1, a position that is not a finite non-negative number).
func decodeAnchors(b []byte) ([]Anchor, error) {
	var raw []struct {
		K    string  `json:"k"`
		Page int     `json:"page"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Text string  `json:"text"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, errors.New("typst: eval: the result is not the expected JSON")
	}
	out := make([]Anchor, 0, min(len(raw), maxAnchors))
	for _, r := range raw {
		if len(out) == maxAnchors {
			break
		}
		if r.Page < 1 || !finite(r.X) || !finite(r.Y) || r.X < 0 || r.Y < 0 {
			continue
		}
		out = append(out, Anchor{Heading: r.K == "h", Page: r.Page, X: r.X, Y: r.Y, Text: clip(r.Text, maxAnchorText)})
	}
	return out, nil
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// clip cuts s after n characters.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
