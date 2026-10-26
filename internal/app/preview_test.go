package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/sidecar"
	"github.com/oioio-space/inkwell/internal/typst"
)

// previewConn plays the webview of the preview stream.
type previewConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     chan []byte
	sent   chan []byte
	once   sync.Once
}

var _ bridge.Conn = (*previewConn)(nil)

func newPreviewConn(t *testing.T) *previewConn {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	c := &previewConn{ctx: ctx, cancel: cancel, in: make(chan []byte, 16), sent: make(chan []byte, 64)}
	t.Cleanup(c.close)
	return c
}

func (c *previewConn) Send(b []byte) error {
	select {
	case c.sent <- b:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *previewConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.ctx.Done():
		return nil, io.EOF
	}
}

func (c *previewConn) Context() context.Context { return c.ctx }
func (c *previewConn) close()                   { c.once.Do(c.cancel) }

// ask sends a request as the webview would.
func (c *previewConn) ask(t *testing.T, req string) {
	t.Helper()
	select {
	case c.in <- []byte(req):
	case <-time.After(5 * time.Second):
		t.Fatalf("request %s not taken", req)
	}
}

// message waits for the next message of type typ, skipping the others.
func (c *previewConn) message(t *testing.T, typ string) app.PreviewMessage {
	t.Helper()
	timeout := time.After(60 * time.Second)
	for {
		select {
		case b := <-c.sent:
			var m app.PreviewMessage
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatalf("message %q is not JSON: %v", b, err)
			}
			if m.Type == typ {
				return m
			}
		case <-timeout:
			t.Fatalf("no %q message", typ)
		}
	}
}

// settle consumes the messages of type typ until none arrives for d: what is
// left of the work the requests so far started.
func (c *previewConn) settle(t *testing.T, typ string, d time.Duration) {
	t.Helper()
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case b := <-c.sent:
			var m app.PreviewMessage
			if err := json.Unmarshal(b, &m); err == nil && m.Type == typ {
				timer.Reset(d)
			}
		case <-timer.C:
			return
		}
	}
}

// quiet asserts that nothing of type typ arrives within d.
func (c *previewConn) quiet(t *testing.T, typ string, d time.Duration) {
	t.Helper()
	timeout := time.After(d)
	for {
		select {
		case b := <-c.sent:
			var m app.PreviewMessage
			if err := json.Unmarshal(b, &m); err == nil && m.Type == typ {
				t.Fatalf("unexpected %q message: %.200s", typ, b)
			}
		case <-timeout:
			return
		}
	}
}

func attach(t *testing.T, p *app.Preview) *previewConn {
	t.Helper()
	c := newPreviewConn(t)
	done := make(chan struct{})
	go func() { defer close(done); p.Serve(c) }()
	t.Cleanup(func() { c.close(); <-done })
	return c
}

func writePreviewProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

var (
	engineOnce sync.Once
	engine     *typst.Engine
	engineErr  error
	engineDir  string
)

// realEngine returns an engine shared by the tests of the file (one extraction
// of the binary, the packages and the fonts).
func realEngine(t *testing.T) *typst.Engine {
	t.Helper()
	engineOnce.Do(func() {
		engineDir, engineErr = os.MkdirTemp("", "inkwell-preview-test-")
		if engineErr != nil {
			return
		}
		engine, engineErr = typst.New(typst.Config{CacheDir: engineDir, Store: &sidecar.Store{Dir: engineDir}})
	})
	if engineErr != nil {
		t.Fatalf("typst.New = %v (did you run `mise run sidecar:fetch`?)", engineErr)
	}
	return engine
}

const threePages = "= Titre un\nBonjour *monde*.\n#pagebreak()\n== Deux\nSuite du texte.\n#pagebreak()\n= Trois\nFin.\n"

func TestPreviewCompilesTheProject(t *testing.T) {
	eng := realEngine(t)

	t.Run("PRV-10 the first result holds the pages in view, the page count and no failure", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": threePages})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root})
		t.Cleanup(p.Close)
		c := attach(t, p)
		c.ask(t, `{"op":"view","first":2,"last":3}`)

		var m app.PreviewMessage
		for {
			m = c.message(t, "result")
			if m.First == 2 {
				break // the first compilation (pages 1 to 4) may arrive before the window is known
			}
		}
		if m.PageCount != 3 || len(m.Pages) != 2 || m.Failure != nil {
			t.Fatalf("result = pages %d, %d svg, failure %v; want 3 pages, 2 svg, none", m.PageCount, len(m.Pages), m.Failure)
		}
		for i, svg := range m.Pages {
			if !strings.HasPrefix(svg, "<svg") {
				t.Errorf("page %d is not an SVG: %.60q", i, svg)
			}
		}
	})

	t.Run("PRV-11 a file written is compiled again after the debounce, once for a burst", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "= Un\n"})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Debounce: 80 * time.Millisecond})
		t.Cleanup(p.Close)
		c := attach(t, p)
		first := c.message(t, "result")
		if first.PageCount != 1 {
			t.Fatalf("first result has %d pages, want 1", first.PageCount)
		}

		for range 5 { // a burst of writes
			if err := os.WriteFile(filepath.Join(root, "main.typ"), []byte(threePages), 0o600); err != nil {
				t.Fatal(err)
			}
			p.Changed("main.typ")
			time.Sleep(10 * time.Millisecond)
		}
		m := c.message(t, "result")
		if m.PageCount != 3 {
			t.Fatalf("result after the burst has %d pages, want 3", m.PageCount)
		}
		if m.Seq != first.Seq+5 {
			t.Errorf("seq = %d after 5 changes, want %d", m.Seq, first.Seq+5)
		}
		c.quiet(t, "result", 600*time.Millisecond) // one result, not five
	})

	t.Run("PRV-12 an error shows in the diagnostics with its file and line, and the fix clears it", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{
			"main.typ": "= Titre\n#include \"part.typ\"\n",
			"part.typ": "ok\n\n#unknown()\n",
		})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Debounce: 20 * time.Millisecond})
		t.Cleanup(p.Close)
		c := attach(t, p)
		m := c.message(t, "result")
		if m.Failure == nil || m.Failure.Kind != app.FailErrors {
			t.Fatalf("failure = %+v, want %q", m.Failure, app.FailErrors)
		}
		if len(m.Diagnostics) == 0 || m.Diagnostics[0].File != "part.typ" || m.Diagnostics[0].Line != 3 || m.Diagnostics[0].Severity != typst.Error {
			t.Fatalf("diagnostics = %+v, want an error in part.typ line 3", m.Diagnostics)
		}
		if err := os.WriteFile(filepath.Join(root, "part.typ"), []byte("ok\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		p.Changed("part.typ")
		m = c.message(t, "result")
		if m.Failure != nil || len(m.Diagnostics) != 0 || m.PageCount != 1 {
			t.Errorf("after the fix: failure %v, %d diagnostics, %d pages", m.Failure, len(m.Diagnostics), m.PageCount)
		}
	})

	t.Run("PRV-13 a warning keeps the document and is reported", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "#set text(font: \"Police Inconnue\")\nBonjour\n"})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root})
		t.Cleanup(p.Close)
		m := attach(t, p).message(t, "result")
		if m.Failure != nil || m.PageCount != 1 || len(m.Pages) != 1 {
			t.Fatalf("failure %v, %d pages: a warning must not hide the document", m.Failure, m.PageCount)
		}
		if len(m.Diagnostics) == 0 || m.Diagnostics[0].Severity != typst.Warning {
			t.Errorf("diagnostics = %+v, want a warning", m.Diagnostics)
		}
	})

	t.Run("PRV-14 a missing package says which, and that Inkwell never downloads", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "#import \"@preview/absent-pkg:1.2.3\": *\n"})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root})
		t.Cleanup(p.Close)
		m := attach(t, p).message(t, "result")
		if m.Failure == nil || m.Failure.Kind != app.FailPackage || len(m.Failure.Packages) != 1 || m.Failure.Packages[0] != "@preview/absent-pkg:1.2.3" {
			t.Fatalf("failure = %+v, want the package named", m.Failure)
		}
	})

	t.Run("PRV-15 a project without main document is a state, not a crash", func(t *testing.T) {
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: t.TempDir()})
		t.Cleanup(p.Close)
		m := attach(t, p).message(t, "result")
		if m.Failure == nil || m.Failure.Kind != app.FailNoMain {
			t.Errorf("failure = %+v, want %q", m.Failure, app.FailNoMain)
		}
	})

	t.Run("PRV-16 the webview reloads: the last result comes back without compiling", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": threePages})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root})
		t.Cleanup(p.Close)
		c1 := attach(t, p)
		want := c1.message(t, "result")
		c1.close()

		c2 := attach(t, p)
		got := c2.message(t, "result")
		if got.Seq != want.Seq || got.PageCount != want.PageCount || len(got.Pages) != len(want.Pages) {
			t.Errorf("after reload: seq %d, %d pages; before: seq %d, %d pages", got.Seq, len(got.Pages), want.Seq, len(want.Pages))
		}
	})
}

var creationDate = regexp.MustCompile(`/CreationDate\s*\(D:(\d{14})`)

func TestPreviewExport(t *testing.T) {
	eng := realEngine(t)
	exportTo := func(t *testing.T, dir string) func(string) (string, error) {
		return func(name string) (string, error) { return filepath.Join(dir, name), nil }
	}

	t.Run("PRV-20 the PDF is written where the person chose, valid and with its attachments", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{
			"main.typ": "= Rapport\n#pdf.attach(\"notes.txt\", bytes(\"bonjour\"), description: \"mes notes\")\nTexte.\n",
		})
		out := t.TempDir()
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Title: func() string { return "Mon: rapport/2" }, Choose: exportTo(t, out)})
		t.Cleanup(p.Close)

		res, err := app.NewPreviewService(p).ExportPDF()
		if err != nil || res.Reason != "" || res.Cancelled {
			t.Fatalf("ExportPDF = %+v, %v", res, err)
		}
		if want := filepath.Join(out, "Mon rapport2.pdf"); res.Path != want {
			t.Errorf("Path = %q, want %q (characters a file name cannot hold are dropped)", res.Path, want)
		}
		pdf, err := os.ReadFile(res.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
			t.Fatalf("not a PDF: %.20q", pdf)
		}
		atts, err := typst.Attachments(pdf)
		if err != nil || len(atts) != 1 || atts[0].Name != "notes.txt" || string(atts[0].Data) != "bonjour" || atts[0].Description != "mes notes" {
			t.Errorf("Attachments = %+v, %v; want notes.txt with its content", atts, err)
		}
		if entries, _ := os.ReadDir(out); len(entries) != 1 {
			t.Errorf("%d files in the folder, want only the PDF (no temporary file left)", len(entries))
		}
	})

	t.Run("PRV-21 the person cancels: nothing is written", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "x\n"})
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Choose: func(string) (string, error) { return "", nil }})
		t.Cleanup(p.Close)
		res, err := app.NewPreviewService(p).ExportPDF()
		if err != nil || !res.Cancelled || res.Path != "" {
			t.Errorf("ExportPDF = %+v, %v; want cancelled", res, err)
		}
	})

	t.Run("PRV-22 a document with errors is not exported, and the reason says so", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "#unknown()\n"})
		out := t.TempDir()
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Choose: exportTo(t, out)})
		t.Cleanup(p.Close)
		res, err := app.NewPreviewService(p).ExportPDF()
		if err != nil || res.Reason != app.FailErrors || res.Path != "" {
			t.Errorf("ExportPDF = %+v, %v; want reason %q", res, err, app.FailErrors)
		}
		if entries, _ := os.ReadDir(out); len(entries) != 0 {
			t.Errorf("%d files written for a document with errors", len(entries))
		}
	})

	t.Run("PRV-23 the export has the extension, needs an absolute path and a dialog", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "x\n"})
		out := t.TempDir()
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Choose: func(string) (string, error) { return filepath.Join(out, "sans-extension"), nil }})
		t.Cleanup(p.Close)
		res, err := app.NewPreviewService(p).ExportPDF()
		if err != nil || !strings.HasSuffix(res.Path, "sans-extension.pdf") {
			t.Errorf("ExportPDF = %+v, %v; want the .pdf extension added", res, err)
		}

		rel := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Choose: func(string) (string, error) { return "relative.pdf", nil }})
		t.Cleanup(rel.Close)
		if _, err := app.NewPreviewService(rel).ExportPDF(); err == nil {
			t.Error("a relative path was accepted")
		}

		none := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root})
		t.Cleanup(none.Close)
		if _, err := app.NewPreviewService(none).ExportPDF(); !errors.Is(err, app.ErrNoDialog) {
			t.Errorf("without a dialog: %v, want ErrNoDialog", err)
		}
	})

	t.Run("PRV-24 the document date is the time of the last change the webview reported", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "x\n"})
		out := t.TempDir()
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Choose: exportTo(t, out), Debounce: 10 * time.Millisecond})
		t.Cleanup(p.Close)
		c := attach(t, p)
		c.message(t, "result")
		c.ask(t, `{"op":"edit","time":1700000000}`) // 2023-11-14 22:13:20 UTC
		c.ask(t, `{"op":"refresh"}`)
		c.message(t, "result")

		res, err := app.NewPreviewService(p).ExportPDF()
		if err != nil || res.Path == "" {
			t.Fatalf("ExportPDF = %+v, %v", res, err)
		}
		pdf, _ := os.ReadFile(res.Path)
		m := creationDate.FindSubmatch(pdf)
		if m == nil || string(m[1]) != "20231114221320" {
			t.Errorf("creation date = %q, want 20231114221320", m)
		}
	})

	t.Run("PRV-25 a change after the last compilation is in the export", func(t *testing.T) {
		root := writePreviewProject(t, map[string]string{"main.typ": "= Un\n"})
		out := t.TempDir()
		p := app.NewPreview(app.PreviewOptions{Engine: eng, Root: root, Choose: exportTo(t, out), Debounce: time.Hour})
		t.Cleanup(p.Close)
		if err := os.WriteFile(filepath.Join(root, "main.typ"), []byte(threePages), 0o600); err != nil {
			t.Fatal(err)
		}
		p.Changed("main.typ") // the debounce has not elapsed: nothing compiled yet
		res, err := app.NewPreviewService(p).ExportPDF()
		if err != nil || res.Path == "" {
			t.Fatalf("ExportPDF = %+v, %v", res, err)
		}
		pdf, _ := os.ReadFile(res.Path)
		if n, err := typst.PageCount(pdf); err != nil || n != 3 {
			t.Errorf("exported PDF has %d pages (%v), want the 3 of the latest text", n, err)
		}
	})
}

// fakeCompiler lets the tests drive timing without the real Typst.
type fakeCompiler struct {
	compile     func(ctx context.Context, call int) (*typst.Result, error)
	calls       atomic.Int32
	anchorCalls atomic.Int32
	deckCalls   atomic.Int32
	anchorGate  chan struct{} // when set, Anchors waits for it to be closed
}

func (f *fakeCompiler) Compile(ctx context.Context, _ typst.Project, _ string, _ ...typst.Option) (*typst.Result, error) {
	return f.compile(ctx, int(f.calls.Add(1)))
}

func (f *fakeCompiler) Anchors(context.Context, typst.Project, string, ...typst.Option) ([]typst.Anchor, error) {
	f.anchorCalls.Add(1)
	if f.anchorGate != nil {
		<-f.anchorGate
	}
	return []typst.Anchor{{Heading: true, Page: 1, X: 70, Y: 70, Text: "Titre"}}, nil
}

func (f *fakeCompiler) Deck(context.Context, typst.Project, string, ...typst.Option) (*typst.Deck, error) {
	f.deckCalls.Add(1)
	return &typst.Deck{Pages: []typst.DeckPage{{Slide: 1}, {Slide: 2}, {Slide: 2, Step: 1, Note: "dire"}}, Minutes: 20}, nil
}

func (f *fakeCompiler) NetworkMode() string { return "fake" }

// okResult is the result of a document of n pages.
func okResult(n int) *typst.Result {
	pdf := fmt.Sprintf("%%PDF-1.7\n1 0 obj\n<< /Type /Pages /Count %d /Kids [] >>\nendobj\n%%%%EOF\n", n)
	pages := make([]typst.SVG, n)
	for i := range pages {
		pages[i] = typst.SVG("<svg/>")
	}
	return &typst.Result{PDF: []byte(pdf), Pages: pages}
}

func TestPreviewScheduling(t *testing.T) {
	t.Run("PRV-30 a compilation superseded by a change is cancelled and never delivered", func(t *testing.T) {
		started := make(chan context.Context, 2)
		fc := &fakeCompiler{compile: func(ctx context.Context, call int) (*typst.Result, error) {
			started <- ctx
			if call == 1 {
				<-ctx.Done() // a long compilation, ended only by its cancellation
				return nil, ctx.Err()
			}
			return okResult(2), nil
		}}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir(), Debounce: 10 * time.Millisecond})
		t.Cleanup(p.Close)
		c := attach(t, p)
		first := <-started
		p.Changed("main.typ")

		select {
		case <-first.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("the superseded compilation was not cancelled")
		}
		m := c.message(t, "result")
		if m.PageCount != 2 {
			t.Errorf("delivered result has %d pages, want the second compilation's 2", m.PageCount)
		}
		if got := fc.calls.Load(); got != 2 {
			t.Errorf("%d compilations, want 2", got)
		}
	})

	t.Run("PRV-31 many changes within the debounce are one compilation", func(t *testing.T) {
		fc := &fakeCompiler{compile: func(context.Context, int) (*typst.Result, error) { return okResult(1), nil }}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir(), Debounce: 100 * time.Millisecond})
		t.Cleanup(p.Close)
		c := attach(t, p)
		c.message(t, "result") // the first compilation, started by the attach
		before := fc.calls.Load()
		for range 20 {
			p.Changed("main.typ")
			time.Sleep(2 * time.Millisecond)
		}
		c.message(t, "result")
		c.quiet(t, "result", 400*time.Millisecond)
		if got := fc.calls.Load() - before; got != 1 {
			t.Errorf("%d compilations for a burst of 20 changes, want 1", got)
		}
	})

	t.Run("PRV-32 the pages in view are known without compiling again", func(t *testing.T) {
		fc := &fakeCompiler{compile: func(context.Context, int) (*typst.Result, error) { return okResult(3), nil }}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir()})
		t.Cleanup(p.Close)
		c := attach(t, p)
		c.ask(t, `{"op":"view","first":1,"last":4}`)
		c.message(t, "result")
		// The compilation the attachment started and the one the window asked
		// for may both answer: take the count once they have.
		c.settle(t, "result", 150*time.Millisecond)
		before := fc.calls.Load()
		c.ask(t, `{"op":"view","first":2,"last":3}`) // held already
		c.ask(t, `{"op":"view","first":5,"last":9}`) // past the end of a 3-page document
		c.quiet(t, "result", 300*time.Millisecond)
		if got := fc.calls.Load(); got != before {
			t.Errorf("%d compilations for windows the result already covers, want none", got-before)
		}
	})

	t.Run("PRV-33 Close stops a compilation in flight and leaves no goroutine", func(t *testing.T) {
		running := make(chan struct{})
		fc := &fakeCompiler{compile: func(ctx context.Context, _ int) (*typst.Result, error) {
			close(running)
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir()})
		attach(t, p)
		<-running
		done := make(chan struct{})
		go func() { p.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Close did not return")
		}
		p.Changed("x") // after Close: ignored
	})

	t.Run("PRV-34 anchors are answered for the jump", func(t *testing.T) {
		fc := &fakeCompiler{compile: func(context.Context, int) (*typst.Result, error) { return okResult(1), nil }}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir()})
		t.Cleanup(p.Close)
		c := attach(t, p)
		c.ask(t, `{"op":"anchors"}`)
		m := c.message(t, "anchors")
		if len(m.Anchors) != 1 || m.Anchors[0].Text != "Titre" || !m.Anchors[0].Heading {
			t.Errorf("anchors = %+v", m.Anchors)
		}
	})

	t.Run("PRV-36 a flood of anchors requests is one computation", func(t *testing.T) {
		fc := &fakeCompiler{
			compile:    func(context.Context, int) (*typst.Result, error) { return okResult(1), nil },
			anchorGate: make(chan struct{}),
		}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir()})
		t.Cleanup(p.Close)
		c := attach(t, p)
		for range 5 {
			c.ask(t, `{"op":"anchors"}`)
		}
		time.Sleep(200 * time.Millisecond) // all five are read while the first computation waits
		close(fc.anchorGate)
		c.message(t, "anchors")
		c.quiet(t, "anchors", 300*time.Millisecond)
		if got := fc.anchorCalls.Load(); got != 1 {
			t.Errorf("%d computations for 5 requests, want 1", got)
		}
	})

	t.Run("PRV-37 the slides and steps are answered, and computed once per version", func(t *testing.T) {
		fc := &fakeCompiler{compile: func(context.Context, int) (*typst.Result, error) { return okResult(3), nil }}
		p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: t.TempDir()})
		t.Cleanup(p.Close)
		c := attach(t, p)
		c.ask(t, `{"op":"deck"}`)
		m := c.message(t, "deck")
		if m.Deck == nil || len(m.Deck.Pages) != 3 || m.Deck.Pages[2].Note != "dire" || m.Deck.Minutes != 20 {
			t.Fatalf("deck = %+v", m.Deck)
		}
		c.ask(t, `{"op":"deck"}`)
		c.message(t, "deck")
		if got := fc.deckCalls.Load(); got != 1 {
			t.Errorf("%d computations for the same sources, want 1", got)
		}
		p.Changed("main.typ")
		c.ask(t, `{"op":"deck"}`)
		c.message(t, "deck")
		if got := fc.deckCalls.Load(); got != 2 {
			t.Errorf("%d computations after a change, want 2", got)
		}
	})

	t.Run("PRV-35 the network mode is the engine's", func(t *testing.T) {
		p := app.NewPreview(app.PreviewOptions{Engine: &fakeCompiler{}, Root: t.TempDir()})
		t.Cleanup(p.Close)
		if got := app.NewPreviewService(p).NetworkMode(); got != "fake" {
			t.Errorf("NetworkMode = %q", got)
		}
	})
}
