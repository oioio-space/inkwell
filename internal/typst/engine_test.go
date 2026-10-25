package typst

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

const mainFixture = `#import "@preview/cmarker:0.1.10"
#import "@preview/lilaq:0.6.0" as lq
#set document(title: "Fixture", author: "Inkwell")
#set page(width: 12cm, height: 16cm, margin: 1cm)

= Fixture <top>

#include "chapter.typ"

#figure(image("logo.png", width: 2cm), caption: [Logo])

Cited: @knuth84.

#let rows = csv("data.csv")
#metadata(rows) <csvdata>
#table(columns: rows.at(0).len(), ..rows.flatten())

#figure(
  lq.diagram(width: 6cm, height: 4cm, lq.plot((0, 1, 2, 3), (0, 1, 4, 9))),
  caption: [Chart],
) <chart>

#cmarker.render(read("notes.md"))

#bibliography("refs.bib")
`

const slidesFixture = `#import "@preview/touying:0.7.4": *
#import themes.simple: *
#show: simple-theme.with(aspect-ratio: "16-9")

= Title slide

== First

Hello #pause world

#speaker-note[Remember to smile]

== Second

Bye
`

func fixtureFiles(t *testing.T) map[string]string {
	return map[string]string{
		"main.typ":    mainFixture,
		"chapter.typ": "== Chapter\nText of the chapter.\n",
		"logo.png":    pngBytes(t),
		"data.csv":    "name,qty\nalpha,1\nbeta,2\n",
		"notes.md":    "# Notes title\n\nSome *emphasis* and `code`.\n\n- one\n- two\n",
		"refs.bib":    "@book{knuth84,\n  author = {Donald E. Knuth},\n  title = {The TeXbook},\n  publisher = {Addison-Wesley},\n  year = {1984},\n}\n",
		"slides.typ":  slidesFixture,
	}
}

func TestCompileFixtureOffline(t *testing.T) {
	e := newEngine(t, nil)
	p := writeProject(t, fixtureFiles(t))

	t.Run("TYP-01 a full project compiles offline to PDF and SVG", func(t *testing.T) {
		res, err := e.Compile(t.Context(), p, "main.typ")
		if err != nil {
			t.Fatalf("Compile(main.typ) = %v\ndiagnostics: %+v", err, res)
		}
		assertPDF(t, res.PDF)
		assertSVGs(t, res.Pages)
		if got, want := len(res.Pages), pdfPages(res.PDF); got != want {
			t.Errorf("%d SVG pages for a PDF of %d pages", got, want)
		}
		for _, d := range res.Diagnostics {
			if d.Severity == Error {
				t.Errorf("unexpected error: %+v", d)
			}
		}
	})

	t.Run("TYP-01 the Touying slides compile, one page per step", func(t *testing.T) {
		res, err := e.Compile(t.Context(), p, "slides.typ")
		if err != nil {
			t.Fatalf("Compile(slides.typ) = %v", err)
		}
		assertPDF(t, res.PDF)
		// Title section, "First" in two steps (#pause), "Second".
		if got, want := len(res.Pages), 4; got != want {
			t.Errorf("slides give %d pages, want %d", got, want)
		}
		if got, want := pdfPages(res.PDF), len(res.Pages); got != want {
			t.Errorf("PDF has %d pages, SVG %d", got, want)
		}
	})

	t.Run("TYP-02 every brick produced its content", func(t *testing.T) {
		contains := func(selector string, want ...string) {
			t.Helper()
			raw, err := e.Query(t.Context(), p, "main.typ", selector)
			if err != nil {
				t.Fatalf("Query(%s) = %v", selector, err)
			}
			for _, w := range want {
				if !bytes.Contains(raw, []byte(w)) {
					t.Errorf("Query(%s) does not contain %q: %.300s", selector, w, raw)
				}
			}
		}
		contains("heading", "Notes title", "Chapter", "Fixture") // cmarker turned # into a heading
		contains("<csvdata>", "alpha", "beta")                   // the CSV was read
		contains("<chart>", "figure")                            // the lilaq diagram is in a figure
		contains("cite", "knuth84")                              // the bibliography key resolved
	})

	t.Run("TYP-28 a Markdown note compiles on a fresh cache, with nothing to download", func(t *testing.T) {
		fresh := newEngine(t, func(c *Config) { c.CacheDir = t.TempDir() })
		md := writeProject(t, map[string]string{
			"main.typ": `#import "@preview/cmarker:0.1.10"
#cmarker.render(read("note.md"))
`,
			"note.md": "# Hello\n\nA *note*.\n",
		})
		res, err := fresh.Compile(t.Context(), md, "main.typ")
		if err != nil {
			t.Fatalf("Compile = %v", err)
		}
		assertPDF(t, res.PDF)
	})
}

func TestCompileAttachments(t *testing.T) {
	e := newEngine(t, nil)
	csv := "a,b\n1,2\n3,4\n"
	p := writeProject(t, map[string]string{
		"data.csv": csv,
		"main.typ": `= Doc
#pdf.attach("data.csv", relationship: "data", mime-type: "text/csv", description: "the data")
#pdf.attach("inline.txt", bytes("hello attach"), description: "inline")
`,
	})
	res, err := e.Compile(t.Context(), p, "main.typ")
	if err != nil {
		t.Fatalf("Compile = %v", err)
	}
	t.Run("TYP-03 pdf.attach files are inside the produced PDF", func(t *testing.T) {
		got, err := Attachments(res.PDF)
		if err != nil {
			t.Fatalf("Attachments = %v", err)
		}
		want := []Attachment{
			{Name: "data.csv", Description: "the data", MIMEType: "text/csv", Data: []byte(csv)},
			{Name: "inline.txt", Description: "inline", Data: []byte("hello attach")},
		}
		if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b Attachment) bool { return a.Name < b.Name })); diff != "" {
			t.Errorf("Attachments mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestCompileDeterministic(t *testing.T) {
	e := newEngine(t, nil)
	files := fixtureFiles(t)
	files["main.typ"] += "\n#pdf.attach(\"data.csv\")\n#datetime.today().display()\n"
	p := writeProject(t, files)

	first, err := e.Compile(t.Context(), p, "main.typ")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond) // a clock-dependent output would differ now
	second, err := e.Compile(t.Context(), p, "main.typ")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("TYP-04 two compilations a second apart are identical", func(t *testing.T) {
		if !bytes.Equal(first.PDF, second.PDF) {
			t.Error("the PDFs differ")
		}
		if len(first.Pages) != len(second.Pages) {
			t.Fatalf("%d and %d pages", len(first.Pages), len(second.Pages))
		}
		for i := range first.Pages {
			if !bytes.Equal(first.Pages[i], second.Pages[i]) {
				t.Errorf("SVG page %d differs", i+1)
			}
		}
	})

	t.Run("TYP-04 a copy elsewhere, with another cache, is identical", func(t *testing.T) {
		other := newEngine(t, func(c *Config) { c.CacheDir = t.TempDir() })
		copyDir := writeProject(t, files)
		res, err := other.Compile(t.Context(), copyDir, "main.typ")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first.PDF, res.PDF) {
			t.Error("the PDF of the copy differs")
		}
	})

	t.Run("TYP-04 the document date is the one given", func(t *testing.T) {
		date := time.Date(2026, 3, 14, 9, 26, 53, 0, time.UTC)
		a, err := e.Compile(t.Context(), p, "main.typ", WithTimestamp(date), WithFormats(FormatPDF))
		if err != nil {
			t.Fatal(err)
		}
		b, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(a.PDF, b.PDF) {
			t.Error("a different document date gave the same PDF")
		}
		if a.Pages != nil {
			t.Error("SVG pages were produced although only the PDF was asked for")
		}
		if !bytes.Contains(a.PDF, []byte("20260314092653")) {
			t.Error("the PDF does not carry the given creation date")
		}
	})
}

func TestCompileDiagnostics(t *testing.T) {
	e := newEngine(t, nil)

	t.Run("TYP-06 an error in an included file names that file, line and column", func(t *testing.T) {
		p := writeProject(t, map[string]string{
			"main.typ":  "= Title\n#import \"sub/x.typ\": y\n",
			"sub/x.typ": "#let y = 1 +\n",
		})
		res, err := e.Compile(t.Context(), p, "main.typ")
		if !errors.Is(err, ErrCompile) {
			t.Fatalf("Compile = %v, want ErrCompile", err)
		}
		ce, ok := errors.AsType[*CompileError](err)
		if !ok {
			t.Fatalf("error %T is not a *CompileError", err)
		}
		want := Diagnostic{Severity: Error, File: "sub/x.typ", Line: 1, Column: 13, Message: "expected expression"}
		if len(ce.Diagnostics) == 0 || ce.Diagnostics[0] != want {
			t.Errorf("diagnostics = %+v, want first %+v", ce.Diagnostics, want)
		}
		if res == nil || len(res.PDF) != 0 || len(res.Diagnostics) == 0 {
			t.Errorf("Result = %+v, want diagnostics and no PDF", res)
		}
		if !strings.Contains(err.Error(), "sub/x.typ:1:13") {
			t.Errorf("Error() = %q, want the position", err)
		}
	})

	t.Run("TYP-07 a warning does not fail the compilation", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "#set text(font: \"Nonexistent Font\")\nHello\n"})
		res, err := e.Compile(t.Context(), p, "main.typ")
		if err != nil {
			t.Fatalf("Compile = %v", err)
		}
		assertPDF(t, res.PDF)
		if len(res.Diagnostics) != 1 || res.Diagnostics[0].Severity != Warning ||
			!strings.Contains(res.Diagnostics[0].Message, "unknown font family") {
			t.Errorf("diagnostics = %+v, want the unknown-font warning", res.Diagnostics)
		}
	})

	t.Run("TYP-05 system fonts are never used, so an installed font is still unknown", func(t *testing.T) {
		// "DejaVu Sans" is on most Linux machines; with --ignore-system-fonts the
		// result must not depend on that.
		p := writeProject(t, map[string]string{"main.typ": "#set text(font: \"Arial\")\nHello\n"})
		res, err := e.Compile(t.Context(), p, "main.typ")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.Diagnostics[0].Message, "unknown font family: arial") {
			t.Errorf("diagnostics = %+v, want Arial unknown on every machine", res.Diagnostics)
		}
	})

	t.Run("TYP-05 the bundled Inter is available", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "#set text(font: \"Inter\")\nHello\n"})
		res, err := e.Compile(t.Context(), p, "main.typ")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Diagnostics) != 0 {
			t.Errorf("diagnostics = %+v, want Inter found", res.Diagnostics)
		}
		if !bytes.Contains(res.PDF, []byte("Inter")) {
			t.Error("the PDF does not embed Inter")
		}
	})

	t.Run("TYP-05 the project's fonts folder is searched after the bundled fonts", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "Hi\n", "fonts/README": "x"})
		j, err := e.prepare(p, "main.typ")
		if err != nil {
			t.Fatal(err)
		}
		defer j.cleanup()
		realRoot, _ := filepath.EvalSymlinks(p.Root)
		want := []string{e.fonts.root, filepath.Join(realRoot, "fonts")}
		if !cmp.Equal(j.fonts, want) {
			t.Errorf("font paths = %v, want %v", j.fonts, want)
		}
		args := e.args("compile", j, applyOptions(nil))
		if !slices.Contains(args, "--ignore-system-fonts") {
			t.Errorf("arguments %v lack --ignore-system-fonts", args)
		}
	})

	t.Run("TYP-08 the column counts characters, after accents, CJK and emoji", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "héllo wörld 😀 日本 #unknownfn()\n"})
		_, err := e.Compile(t.Context(), p, "main.typ")
		ce, ok := errors.AsType[*CompileError](err)
		if !ok {
			t.Fatalf("Compile = %v, want a CompileError", err)
		}
		d := ce.Diagnostics[0]
		if d.Line != 1 || d.Column != 19 || d.Message != "unknown variable: unknownfn" {
			t.Errorf("diagnostic = %+v, want 1:19 unknown variable: unknownfn", d)
		}
	})

	t.Run("TYP-20 thousands of warnings are bounded", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "#for i in range(1500) [#text(font: \"Nope\" + str(i))[x]]\n"})
		res, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF))
		if err != nil {
			t.Fatalf("Compile = %v", err)
		}
		if len(res.Diagnostics) != DefaultMaxDiagnostics || !res.DiagnosticsTruncated {
			t.Errorf("kept %d diagnostics, truncated=%v; want %d and true", len(res.Diagnostics), res.DiagnosticsTruncated, DefaultMaxDiagnostics)
		}
	})
}

func TestCompilePackages(t *testing.T) {
	e := newEngine(t, nil)
	ghost := `#import "@preview/ghost:1.2.3": *` + "\n"

	t.Run("TYP-09 a package that is not available is named, never downloaded", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": ghost})
		begin := time.Now()
		res, err := e.Compile(t.Context(), p, "main.typ")
		if !errors.Is(err, ErrMissingPackage) || !errors.Is(err, ErrCompile) {
			t.Fatalf("Compile = %v, want ErrMissingPackage and ErrCompile", err)
		}
		ce, _ := errors.AsType[*CompileError](err)
		if len(ce.MissingPackages) != 1 || ce.MissingPackages[0] != "@preview/ghost:1.2.3" {
			t.Errorf("MissingPackages = %v, want [@preview/ghost:1.2.3]", ce.MissingPackages)
		}
		d := res.Diagnostics[0]
		if d.Package != "@preview/ghost:1.2.3" || d.File != "main.typ" || d.Line != 1 ||
			!strings.Contains(d.Message, "never downloads") || !strings.Contains(d.Message, ".typstdeps") {
			t.Errorf("diagnostic = %+v, want a clear, positioned message", d)
		}
		if took := time.Since(begin); took > 20*time.Second {
			t.Errorf("took %v: Typst waited on the network", took)
		}
	})

	t.Run("TYP-10 a download attempt reaches the refused proxy and nothing else", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		requests := make(chan string, 8)
		var wg sync.WaitGroup
		wg.Go(func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				buf := make([]byte, 512)
				_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				n, _ := conn.Read(buf)
				requests <- string(buf[:n])
				_ = conn.Close() // the proxy refuses
			}
		})
		t.Cleanup(func() {
			_ = ln.Close()
			wg.Wait()
		})

		spy := newEngine(t, func(c *Config) { c.egressProxy = "http://" + ln.Addr().String() })
		p := writeProject(t, map[string]string{"main.typ": ghost})
		if _, err := spy.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); !errors.Is(err, ErrMissingPackage) {
			t.Fatalf("Compile = %v, want ErrMissingPackage", err)
		}
		select {
		case req := <-requests:
			if !strings.HasPrefix(req, "CONNECT packages.typst.org:443") {
				t.Errorf("the proxy received %q, want a CONNECT to packages.typst.org", req)
			}
		case <-time.After(5 * time.Second):
			t.Error("Typst did not try to download through the configured proxy: the environment is not what the engine set")
		}
	})

	t.Run("the environment of Inkwell does not leak into the compilation", func(t *testing.T) {
		t.Setenv("TYPST_PACKAGE_CACHE_PATH", t.TempDir())
		t.Setenv("TYPST_ROOT", "/")
		t.Setenv("SOURCE_DATE_EPOCH", "1")
		p := writeProject(t, map[string]string{"main.typ": "#import \"@preview/cmarker:0.1.10\"\nHi\n"})
		if _, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); err != nil {
			t.Fatalf("Compile = %v", err)
		}
	})

	t.Run("TYP-11 a package vendored in .typstdeps works offline", func(t *testing.T) {
		p := writeProject(t, map[string]string{
			".typstdeps/preview/mylib/0.1.0/typst.toml": "[package]\nname = \"mylib\"\nversion = \"0.1.0\"\nentrypoint = \"lib.typ\"\n",
			".typstdeps/preview/mylib/0.1.0/lib.typ":    "#let tag = \"mylib-0.1.0-ok\"\n",
			"main.typ":                                  "#import \"@preview/mylib:0.1.0\": tag\n#metadata(tag) <t>\nHi\n",
		})
		raw, err := e.Query(t.Context(), p, "main.typ", "<t>")
		if err != nil {
			t.Fatalf("Query = %v", err)
		}
		if !bytes.Contains(raw, []byte("mylib-0.1.0-ok")) {
			t.Errorf("Query = %s, want the vendored package's value", raw)
		}
		if _, err := e.Compile(t.Context(), p, "main.typ"); err != nil {
			t.Errorf("Compile = %v", err)
		}
	})

	t.Run("TYP-11 a bundled package wins over a vendored one of the same version", func(t *testing.T) {
		p := writeProject(t, map[string]string{
			".typstdeps/preview/touying/0.7.4/typst.toml": "[package]\nname = \"touying\"\nversion = \"0.7.4\"\nentrypoint = \"lib.typ\"\n",
			".typstdeps/preview/touying/0.7.4/lib.typ":    "#let evil = 1\n",
			"main.typ": "#import \"@preview/touying:0.7.4\": *\n#import themes.simple: *\n#show: simple-theme.with(aspect-ratio: \"16-9\")\n== Slide\nHi\n",
		})
		if _, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); err != nil {
			t.Fatalf("Compile = %v: the vendored copy replaced the pinned package", err)
		}
	})

	t.Run("TYP-11 a vendored package shows its files as the package it is", func(t *testing.T) {
		p := writeProject(t, map[string]string{
			".typstdeps/preview/broken/1.0.0/typst.toml": "[package]\nname = \"broken\"\nversion = \"1.0.0\"\nentrypoint = \"lib.typ\"\n",
			".typstdeps/preview/broken/1.0.0/lib.typ":    "#let x = 1 +\n",
			"main.typ": "#import \"@preview/broken:1.0.0\": x\n",
		})
		_, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF))
		ce, ok := errors.AsType[*CompileError](err)
		if !ok || ce.Diagnostics[0].File != "@preview/broken:1.0.0/lib.typ" {
			t.Fatalf("Compile = %v, want an error in @preview/broken:1.0.0/lib.typ", err)
		}
	})
}

func TestCompileHostile(t *testing.T) {
	t.Run("TYP-14 an endless loop stops at the timeout, leaving no process", func(t *testing.T) {
		e := newEngine(t, func(c *Config) { c.Timeout = time.Second })
		p := writeProject(t, map[string]string{"main.typ": "#for i in range(3000000000) { let x = i * 2 }\n"})
		begin := time.Now()
		_, err := e.Compile(t.Context(), p, "main.typ")
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("Compile = %v, want ErrTimeout", err)
		}
		if took := time.Since(begin); took > 6*time.Second {
			t.Errorf("took %v for a 1s timeout", took)
		}
		bin, _ := sharedStore.Resolve(sidecar.Typst)
		deadline := time.Now().Add(3 * time.Second)
		for len(typstProcesses(t, bin)) > 0 && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if left := typstProcesses(t, bin); len(left) > 0 {
			t.Errorf("typst processes left running: %v", left)
		}
	})

	t.Run("TYP-15 cancelling the context stops the compilation", func(t *testing.T) {
		e := newEngine(t, nil)
		p := writeProject(t, map[string]string{"main.typ": "#for i in range(3000000000) { let x = i * 2 }\n"})
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(400*time.Millisecond, cancel)
		begin := time.Now()
		_, err := e.Compile(ctx, p, "main.typ")
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimeout) {
			t.Fatalf("Compile = %v, want Canceled and not a timeout", err)
		}
		if took := time.Since(begin); took > 4*time.Second {
			t.Errorf("took %v after cancellation", took)
		}
		bin, _ := sharedStore.Resolve(sidecar.Typst)
		deadline := time.Now().Add(3 * time.Second)
		for len(typstProcesses(t, bin)) > 0 && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if left := typstProcesses(t, bin); len(left) > 0 {
			t.Errorf("typst processes left running: %v", left)
		}
	})

	t.Run("an already cancelled context compiles nothing", func(t *testing.T) {
		e := newEngine(t, nil)
		p := writeProject(t, map[string]string{"main.typ": "Hi\n"})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := e.Compile(ctx, p, "main.typ"); !errors.Is(err, context.Canceled) {
			t.Fatalf("Compile = %v, want Canceled", err)
		}
	})

	t.Run("TYP-16 a huge allocation hits the memory limit, not the machine", func(t *testing.T) {
		e := newEngine(t, func(c *Config) { c.MemoryLimit = 512 << 20 })
		p := writeProject(t, map[string]string{"main.typ": "#let a = range(2000000000)\n#a.len()\n"})
		_, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF))
		if !errors.Is(err, ErrMemoryLimit) {
			t.Fatalf("Compile = %v, want ErrMemoryLimit", err)
		}
	})

	t.Run("TYP-17 paths that leave the project are refused by Typst", func(t *testing.T) {
		e := newEngine(t, nil)
		for name, src := range map[string]string{
			"parent directory": `#read("../secret.txt")`,
			"include":          `#include "../secret.txt"`,
			"image":            `#image("../secret.txt")`,
			"absolute path":    `#read("%ABS%")`,
		} {
			t.Run(name, func(t *testing.T) {
				base := t.TempDir()
				secret := filepath.Join(base, "secret.txt")
				if err := os.WriteFile(secret, []byte("TOP-SECRET-CONTENT"), 0o600); err != nil {
					t.Fatal(err)
				}
				root := filepath.Join(base, "proj")
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				src = strings.ReplaceAll(src, "%ABS%", secret)
				if err := os.WriteFile(filepath.Join(root, "main.typ"), []byte(src), 0o600); err != nil {
					t.Fatal(err)
				}
				res, err := e.Compile(t.Context(), Project{Root: root}, "main.typ")
				if !errors.Is(err, ErrCompile) {
					t.Fatalf("Compile = %v, want ErrCompile", err)
				}
				if bytes.Contains(res.PDF, []byte("TOP-SECRET")) || strings.Contains(err.Error(), "TOP-SECRET") {
					t.Error("the file outside the project was read")
				}
			})
		}
	})

	t.Run("TYP-17 a main file outside the project is refused before anything runs", func(t *testing.T) {
		e := newEngine(t, nil)
		p := writeProject(t, map[string]string{"main.typ": "Hi\n", "dir/x.typ": "x"})
		for _, main := range []string{"", "../main.typ", "/etc/passwd", "nope.typ", "dir", "dir/../../x.typ"} {
			if _, err := e.Compile(t.Context(), p, main); !errors.Is(err, ErrInvalidMain) {
				t.Errorf("Compile(main=%q) = %v, want ErrInvalidMain", main, err)
			}
		}
	})

	t.Run("TYP-18 a symbolic link that leaves the project is refused", func(t *testing.T) {
		e := newEngine(t, nil)
		secret := filepath.Join(t.TempDir(), "secret.txt")
		if err := os.WriteFile(secret, []byte("TOP-SECRET-CONTENT"), 0o600); err != nil {
			t.Fatal(err)
		}
		p := writeProject(t, map[string]string{"main.typ": "#read(\"leak.txt\")\n"})
		if err := os.Symlink(secret, filepath.Join(p.Root, "leak.txt")); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Compile(t.Context(), p, "main.typ"); !errors.Is(err, ErrUnsafeProject) {
			t.Fatalf("Compile = %v, want ErrUnsafeProject", err)
		}

		dirLink := writeProject(t, map[string]string{"main.typ": "Hi\n"})
		if err := os.Symlink(filepath.Dir(secret), filepath.Join(dirLink.Root, "up")); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Compile(t.Context(), dirLink, "main.typ"); !errors.Is(err, ErrUnsafeProject) {
			t.Fatalf("Compile (folder link) = %v, want ErrUnsafeProject", err)
		}
	})

	t.Run("TYP-18 links that stay inside, or lead nowhere, are fine", func(t *testing.T) {
		e := newEngine(t, nil)
		p := writeProject(t, map[string]string{"main.typ": "#include \"alias.typ\"\n", "real.typ": "Real\n"})
		if err := os.Symlink("real.typ", filepath.Join(p.Root, "alias.typ")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/nonexistent/target", filepath.Join(p.Root, "dangling")); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); err != nil {
			t.Fatalf("Compile = %v", err)
		}
	})

	t.Run("TYP-19 too many pages, or too many bytes", func(t *testing.T) {
		src := "#for i in range(5) [Page #i #pagebreak()]\n"
		p := writeProject(t, map[string]string{"main.typ": src})
		pages := newEngine(t, func(c *Config) { c.MaxPages = 3 })
		if _, err := pages.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG)); !errors.Is(err, ErrOutputTooLarge) {
			t.Errorf("Compile(5 pages, max 3) = %v, want ErrOutputTooLarge", err)
		}
		small := newEngine(t, func(c *Config) { c.MaxOutput = 200 })
		if _, err := small.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); !errors.Is(err, ErrOutputTooLarge) {
			t.Errorf("Compile(PDF > 200 bytes) = %v, want ErrOutputTooLarge", err)
		}
		if _, err := small.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG)); !errors.Is(err, ErrOutputTooLarge) {
			t.Errorf("Compile(SVG > 200 bytes) = %v, want ErrOutputTooLarge", err)
		}
	})

	t.Run("a compilation leaves nothing in the project or the engine's temporary folder", func(t *testing.T) {
		e := newEngine(t, nil)
		p := writeProject(t, map[string]string{
			"main.typ":                               "Hi\n",
			".typstdeps/preview/mylib/0.1.0/lib.typ": "x",
		})
		before := readDirNames(t, p.Root)
		if _, err := e.Compile(t.Context(), p, "main.typ"); err != nil {
			t.Fatal(err)
		}
		if after := readDirNames(t, p.Root); !cmp.Equal(before, after) {
			t.Errorf("project folder changed: %v -> %v", before, after)
		}
		if left := readDirNames(t, e.tmp); len(left) != 0 {
			t.Errorf("temporary folder holds %v", left)
		}
	})
}

func TestCompileConcurrent(t *testing.T) {
	e := newEngine(t, nil)
	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	got := make([]string, n)
	for i := range n {
		wg.Go(func() {
			p := writeProject(t, map[string]string{"main.typ": "= Project number " + string(rune('A'+i)) + "\n#metadata(\"" + string(rune('A'+i)) + "\") <id>\n"})
			if _, errs[i] = e.Compile(t.Context(), p, "main.typ"); errs[i] != nil {
				return
			}
			raw, err := e.Query(t.Context(), p, "main.typ", "<id>")
			errs[i], got[i] = err, string(raw)
		})
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Errorf("TYP-25 project %d: %v", i, errs[i])
			continue
		}
		if want := string(rune('A' + i)); !strings.Contains(got[i], want) {
			t.Errorf("TYP-25 project %d answered %s, want %s", i, got[i], want)
		}
	}
}

func TestCompileInputs(t *testing.T) {
	e := newEngine(t, nil)

	t.Run("TYP-26 a bad project folder", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		for name, root := range map[string]string{"empty": "", "missing": filepath.Join(t.TempDir(), "nope"), "a file": file} {
			if _, err := e.Compile(t.Context(), Project{Root: root}, "main.typ"); err == nil {
				t.Errorf("Compile(root %s) = nil, want an error", name)
			}
		}
	})

	t.Run("TYP-26 a relative project folder and a nested main file", func(t *testing.T) {
		p := writeProject(t, map[string]string{"deep/er/main.typ": "Hi\n"})
		t.Chdir(filepath.Dir(p.Root))
		rel := Project{Root: filepath.Base(p.Root)}
		if _, err := e.Compile(t.Context(), rel, "deep/er/main.typ", WithFormats(FormatPDF)); err != nil {
			t.Fatalf("Compile = %v", err)
		}
	})

	t.Run("no output format requested", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "Hi\n"})
		if _, err := e.Compile(t.Context(), p, "main.typ", WithFormats(0)); err == nil {
			t.Error("Compile with no format = nil, want an error")
		}
	})

	t.Run("TYP-27 Query returns the headings and rejects a bad selector", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "= One\n== Two\n"})
		raw, err := e.Query(t.Context(), p, "main.typ", "heading")
		if err != nil || !bytes.Contains(raw, []byte("One")) || !bytes.Contains(raw, []byte("Two")) {
			t.Fatalf("Query(heading) = %s, %v", raw, err)
		}
		if _, err := e.Query(t.Context(), p, "main.typ", ""); err == nil {
			t.Error("Query with an empty selector = nil, want an error")
		}
		if _, err := e.Query(t.Context(), p, "main.typ", strings.Repeat("x", 5000)); err == nil {
			t.Error("Query with a 5000-byte selector = nil, want an error")
		}
		if _, err := e.Query(t.Context(), p, "main.typ", "not a valid selector ("); !errors.Is(err, ErrCompile) {
			t.Errorf("Query(bad selector) = %v, want a compile error", err)
		}
		if _, err := e.Query(t.Context(), p, "../main.typ", "heading"); !errors.Is(err, ErrInvalidMain) {
			t.Errorf("Query(main outside) = %v, want ErrInvalidMain", err)
		}
	})

	t.Run("TYP-27 Query honours cancellation and the timeout", func(t *testing.T) {
		slow := writeProject(t, map[string]string{"main.typ": "#for i in range(3000000000) { let x = i * 2 }\n"})
		short := newEngine(t, func(c *Config) { c.Timeout = time.Second })
		if _, err := short.Query(t.Context(), slow, "main.typ", "heading"); !errors.Is(err, ErrTimeout) {
			t.Errorf("Query(slow) = %v, want ErrTimeout", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := e.Query(ctx, slow, "main.typ", "heading"); !errors.Is(err, context.Canceled) {
			t.Errorf("Query(cancelled) = %v, want Canceled", err)
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("defaults come from the user cache directory", func(t *testing.T) {
		cache := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", cache)
		t.Setenv("HOME", cache)
		e, err := New(Config{Store: sharedStore})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(e.pkgs.root, cache) {
			t.Errorf("packages in %q, want under %q", e.pkgs.root, cache)
		}
	})

	t.Run("a cache directory that cannot be created is an error", func(t *testing.T) {
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := New(Config{CacheDir: filepath.Join(blocker, "x"), Store: sharedStore}); err == nil {
			t.Error("New under a regular file = nil, want an error")
		}
	})

	t.Run("a damaged bundled package is repaired at the next compilation", func(t *testing.T) {
		e := newEngine(t, func(c *Config) { c.CacheDir = t.TempDir() })
		target := filepath.Join(e.pkgs.root, "preview", "cmarker", "0.1.10", "typst.toml")
		if err := os.WriteFile(target, []byte("garbage"), 0o600); err != nil {
			t.Fatal(err)
		}
		p := writeProject(t, map[string]string{"main.typ": "#import \"@preview/cmarker:0.1.10\"\nHi\n"})
		if _, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); err != nil {
			t.Fatalf("Compile = %v", err)
		}
		if data, _ := os.ReadFile(target); !bytes.Contains(data, []byte(`name = "cmarker"`)) {
			t.Errorf("typst.toml = %q, want it restored", data)
		}
	})
}

func TestCompileErrorIs(t *testing.T) {
	ce := &CompileError{}
	if !errors.Is(ce, ErrCompile) || errors.Is(ce, ErrMissingPackage) {
		t.Error("an empty CompileError must be ErrCompile only")
	}
	if got := ce.Error(); got != ErrCompile.Error() {
		t.Errorf("Error() = %q", got)
	}
	ce = &CompileError{Diagnostics: []Diagnostic{{Severity: Warning, Message: "w"}, {Severity: Error, Message: "boom"}}}
	if got := ce.Error(); got != "typst: boom" {
		t.Errorf("Error() = %q, want the first error without position", got)
	}
}
