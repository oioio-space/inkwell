package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/typst"
)

func TestFailureOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
		pkgs []string
	}{
		{"PRV-40 no error", nil, "", nil},
		{"PRV-40 errors in the document", &typst.CompileError{Diagnostics: []typst.Diagnostic{{Severity: typst.Error, File: "a.typ", Line: 2}}}, FailErrors, nil},
		{"PRV-40 a missing package", fmt.Errorf("x: %w", &typst.CompileError{MissingPackages: []string{"@preview/p:1.0.0"}}), FailPackage, []string{"@preview/p:1.0.0"}},
		{"PRV-40 a missing package without the list", typst.ErrMissingPackage, FailPackage, nil},
		{"PRV-40 no main document", fmt.Errorf("%w: main.typ", typst.ErrInvalidMain), FailNoMain, nil},
		{"PRV-40 too slow", fmt.Errorf("%w (limit 1m)", typst.ErrTimeout), FailTimeout, nil},
		{"PRV-40 too much memory", typst.ErrMemoryLimit, FailMemory, nil},
		{"PRV-40 too large", typst.ErrOutputTooLarge, FailTooLarge, nil},
		{"PRV-40 a link out of the folder", typst.ErrUnsafeProject, FailUnsafe, nil},
		{"PRV-40 typst cannot start", fmt.Errorf("%w: %w", errUnavailable, errors.New("not embedded")), FailUnavailable, nil},
		{"PRV-40 a corrupted binary", typst.ErrIntegrity, FailUnavailable, nil},
		{"PRV-40 anything else", errors.New("boom /secret/path"), FailInternal, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := failureOf(tc.err)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("failureOf(nil) = %+v", got)
				}
				return
			}
			if got == nil || got.Kind != tc.want || strings.Join(got.Packages, ",") != strings.Join(tc.pkgs, ",") {
				t.Errorf("failureOf(%v) = %+v, want kind %q packages %v", tc.err, got, tc.want, tc.pkgs)
			}
		})
	}
}

func TestDecodePreviewRequest(t *testing.T) {
	ok := []struct {
		name, in string
		want     previewRequest
	}{
		{"PRV-41 view", `{"op":"view","first":2,"last":5}`, previewRequest{Op: "view", First: 2, Last: 5}},
		{"PRV-41 edit", `{"op":"edit","time":1700000000}`, previewRequest{Op: "edit", Time: 1700000000}},
		{"PRV-41 refresh", `{"op":"refresh"}`, previewRequest{Op: "refresh"}},
		{"PRV-41 anchors", `{"op":"anchors"}`, previewRequest{Op: "anchors"}},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodePreviewRequest([]byte(tc.in))
			if err != nil || got != tc.want {
				t.Errorf("decodePreviewRequest(%s) = %+v, %v; want %+v", tc.in, got, err, tc.want)
			}
		})
	}
	bad := []string{
		``, `null`, `[]`, `{}`, `{"op":""}`, `{"op":"nope"}`, `{"op":"view"}`,
		`{"op":"view","first":0,"last":3}`, `{"op":"view","first":4,"last":3}`,
		`{"op":"view","first":1,"last":41}`, `{"op":"view","first":1999,"last":2001}`,
		`{"op":"edit","time":-1}`, `{"op":"edit","time":99999999999999999}`,
		`{"op":"refresh","extra":1}`, `{"op":"refresh"} {"op":"refresh"}`,
		`{"op":"view","first":"1","last":3}`, strings.Repeat(" ", maxPreviewRequest+1) + `{"op":"refresh"}`,
	}
	for _, in := range bad {
		t.Run(fmt.Sprintf("PRV-42 refuses %.30q", in), func(t *testing.T) {
			if got, err := decodePreviewRequest([]byte(in)); err == nil {
				t.Errorf("decodePreviewRequest accepted %.60q as %+v", in, got)
			}
		})
	}
}

func FuzzDecodePreviewRequest(f *testing.F) {
	for _, s := range []string{`{"op":"view","first":1,"last":4}`, `{"op":"edit","time":5}`, `{"op":"refresh"}`, `{`, ``} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		r, err := decodePreviewRequest(in)
		if err != nil {
			return
		}
		if r.Op == "view" && (r.First < 1 || r.Last < r.First || r.Last-r.First >= maxPreviewWindow) {
			t.Fatalf("accepted a window %+v", r)
		}
	})
}

func TestEncodeMessageBoundsTheFrame(t *testing.T) {
	m := &PreviewMessage{Type: "result", Pages: []string{strings.Repeat("x", maxPreviewFrame+1)}}
	b := encodeMessage(m)
	if len(b) > 1<<10 || m.Failure == nil || m.Failure.Kind != FailTooLarge || m.Pages != nil {
		t.Errorf("a message over the frame limit was kept: %d bytes, failure %+v", len(b), m.Failure)
	}
}

func TestExportName(t *testing.T) {
	for in, want := range map[string]string{
		"":                   "Document.pdf",
		"  Rapport  ":        "Rapport.pdf",
		`a/b\c:d*e?"f<g>h|i`: "abcdefghi.pdf",
		"tab\tet\nnewline":   "tabetnewline.pdf",
		"Café — résumé 日本語":  "Café — résumé 日本語.pdf",
	} {
		p := NewPreview(PreviewOptions{Root: "/x", Title: func() string { return in }})
		if got := p.exportName(); got != want {
			t.Errorf("exportName(%q) = %q, want %q", in, got, want)
		}
		p.Close()
	}
}
