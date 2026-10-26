package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/typst"
)

// PreviewStreamName is the name of the Wails stream of the preview: the
// webview says which pages it shows, Go answers with the pages, the page
// count and the diagnostics (no local port, ADR 0009).
const PreviewStreamName = "preview"

const (
	// DefaultPreviewDebounce is how long the sources must stay quiet before
	// they are compiled again: the webview already waits for a pause in typing
	// before it writes the file, so this only groups bursts of writes.
	DefaultPreviewDebounce = 150 * time.Millisecond
	// maxPreviewRequest bounds a request of the webview.
	maxPreviewRequest = 1 << 10
	// maxPreviewWindow bounds the pages rendered at once.
	maxPreviewWindow = 40
	// maxPreviewFrame keeps a message under the stream frame limit.
	maxPreviewFrame = 48 << 20
	// anchorsTimeout bounds one `typst eval` for the jumps.
	anchorsTimeout = 45 * time.Second
)

// Failure kinds of a preview message: what the webview words for the person.
const (
	// FailErrors: the document has errors; they are in the diagnostics.
	FailErrors = "errors"
	// FailPackage: a package is neither bundled nor in .typstdeps (PRJ-14).
	FailPackage = "package"
	// FailNoMain: the project has no main document yet.
	FailNoMain = "noMain"
	// FailTimeout: the compilation took too long.
	FailTimeout = "timeout"
	// FailMemory: the compilation wanted too much memory.
	FailMemory = "memory"
	// FailTooLarge: the result is too large.
	FailTooLarge = "tooLarge"
	// FailUnsafe: the project holds a link that leaves its folder.
	FailUnsafe = "unsafe"
	// FailUnavailable: Typst could not be started.
	FailUnavailable = "unavailable"
	// FailInternal: anything else.
	FailInternal = "internal"
)

// Compiler is what [Preview] needs from the Typst engine, so that tests can
// replace it. *typst.Engine is one.
type Compiler interface {
	Compile(ctx context.Context, p typst.Project, main string, opts ...typst.Option) (*typst.Result, error)
	Anchors(ctx context.Context, p typst.Project, main string, opts ...typst.Option) ([]typst.Anchor, error)
	NetworkMode() string
}

var _ Compiler = (*typst.Engine)(nil)

// ErrNoDialog is returned by [PreviewService.ExportPDF] when nothing can ask the
// person where to save.
var ErrNoDialog = errors.New("no way to choose where to save")

// PreviewOptions tunes [NewPreview]. Root is required.
type PreviewOptions struct {
	// Engine compiles. Nil starts the embedded Typst on first use.
	Engine Compiler
	// Root is the project folder on disk: what is compiled.
	Root string
	// Main is the file to compile, relative to Root; default [project.MainFile].
	Main string
	// Title names the exported file; nil or empty gives "Document".
	Title func() string
	// Debounce waits for quiet sources before compiling; default
	// [DefaultPreviewDebounce].
	Debounce time.Duration
	// Now is the clock; default time.Now. It dates a document whose webview
	// did not say when it was last changed.
	Now func() time.Time
	// Choose asks the person where to save the export, given a suggested file
	// name, and returns the path; "" means they cancelled. Nil: no export.
	Choose func(name string) (string, error)
	// Log receives diagnostics, never document contents. Nil discards them.
	Log *slog.Logger
}

// Preview compiles the open project each time its files change and streams the
// pages in view, the page count and the diagnostics to the webview. It is also
// the Wails service behind "Exporter en PDF". Create one with [NewPreview] and
// close it with [Preview.Close].
type Preview struct {
	opt      PreviewOptions
	log      *slog.Logger
	debounce time.Duration
	now      func() time.Time
	engine   func() (Compiler, error)

	ctx    context.Context
	stop   context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
	timer  *time.Timer

	srcGen    uint64             // counts source changes
	seq       uint64             // counts compilations asked for; only the latest is delivered
	cancel    context.CancelFunc // of the compilation running now
	win       window             // the pages the webview shows
	editTime  time.Time          // when the webview says the document last changed
	wroteAt   time.Time          // when the sources last changed on disk
	cur       *previewConn
	last      []byte // the last result message, for a page that reloads
	lastFirst int    // first page of last
	lastPages int    // pages in last
	lastCount int    // page count of last
	cache     snapshot
	anchors   anchorCache
	asking    bool // an "anchors" request is being answered: a second one waits for that answer
}

// window is a range of pages, 1-based and inclusive.
type window struct{ first, last int }

// snapshot is what the last full compilation produced, valid while the sources
// have not changed.
type snapshot struct {
	srcGen    uint64
	ok        bool // compiled without errors
	pdf       []byte
	pageCount int
}

type anchorCache struct {
	srcGen uint64
	list   []typst.Anchor
	ok     bool
}

// NewPreview returns a preview of the project in o.Root. Nothing runs until the
// sources change or a webview attaches ([Preview.Serve]).
func NewPreview(o PreviewOptions) *Preview {
	p := &Preview{opt: o, log: o.Log, debounce: o.Debounce, now: o.Now, win: window{1, 4}}
	if p.log == nil {
		p.log = slog.New(slog.DiscardHandler)
	}
	if p.debounce <= 0 {
		p.debounce = DefaultPreviewDebounce
	}
	if p.now == nil {
		p.now = time.Now
	}
	if p.opt.Main == "" {
		p.opt.Main = project.MainFile
	}
	p.ctx, p.stop = context.WithCancel(context.Background())
	p.engine = sync.OnceValues(func() (Compiler, error) {
		if o.Engine != nil {
			return o.Engine, nil
		}
		return typst.New(typst.Config{Logger: p.log})
	})
	return p
}

// Close stops the compilations and waits for them. It is idempotent.
func (p *Preview) Close() {
	p.mu.Lock()
	p.closed = true
	if p.timer != nil {
		p.timer.Stop()
	}
	p.mu.Unlock()
	p.stop()
	p.wg.Wait()
}

// networkMode says how Typst is kept offline ("namespace" or "proxy only"; see
// [typst.Engine.NetworkMode]). It is empty when Typst cannot start.
func (p *Preview) networkMode() string {
	eng, err := p.engine()
	if err != nil {
		return ""
	}
	return eng.NetworkMode()
}

// Changed tells the preview that a file of the project changed on disk. The
// running compilation is abandoned and a new one starts once the sources have
// been quiet for the debounce delay.
func (p *Preview) Changed(string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.srcGen++
	p.wroteAt = p.now()
	p.armLocked(p.debounce)
}

// armLocked abandons the compilation in flight and schedules the next one.
func (p *Preview) armLocked(after time.Duration) {
	p.seq++
	if p.cancel != nil {
		p.cancel()
	}
	if p.timer == nil {
		p.timer = time.AfterFunc(after, p.fire)
		return
	}
	p.timer.Reset(after)
}

// docTimeLocked is the date of the document: when it last changed.
func (p *Preview) docTimeLocked() time.Time {
	if !p.editTime.IsZero() {
		return p.editTime
	}
	if !p.wroteAt.IsZero() {
		return p.wroteAt
	}
	return typst.DefaultTimestamp
}

// fire starts the compilation the timer was waiting for.
func (p *Preview) fire() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	run := compilation{
		seq: p.seq, srcGen: p.srcGen, win: p.win, date: p.docTimeLocked(),
		reuse: p.cache.ok && p.cache.srcGen == p.srcGen, pageCount: p.cache.pageCount,
	}
	ctx, cancel := context.WithCancel(p.ctx)
	p.cancel = cancel
	p.wg.Go(func() {
		defer cancel()
		p.compile(ctx, run)
	})
	p.mu.Unlock()
}

// compilation is one run of the compiler, as it was asked for.
type compilation struct {
	seq, srcGen uint64
	win         window
	date        time.Time
	reuse       bool // the PDF of these sources is known: only the pages are needed
	pageCount   int  // of the PDF that is reused
}

// compile runs one compilation and delivers its result unless it was
// superseded meanwhile.
func (p *Preview) compile(ctx context.Context, run compilation) {
	p.deliver(run.seq, mustJSON(PreviewMessage{Type: "compiling", Seq: run.seq}))
	opts := []typst.Option{typst.WithPages(run.win.first, run.win.last), typst.WithTimestamp(run.date)}
	if run.reuse {
		opts = append(opts, typst.WithFormats(typst.FormatSVG))
	}
	var res *typst.Result
	eng, err := p.engine()
	if err == nil {
		res, err = eng.Compile(ctx, typst.Project{Root: p.opt.Root}, p.opt.Main, opts...)
	} else {
		err = fmt.Errorf("%w: %w", errUnavailable, err)
	}
	if ctx.Err() != nil {
		return // superseded or closing: a newer compilation will answer
	}
	msg := PreviewMessage{Type: "result", Seq: run.seq, Rev: run.srcGen, First: run.win.first, PageCount: run.pageCount}
	snap := snapshot{srcGen: run.srcGen, ok: err == nil}
	if res != nil {
		msg.Diagnostics = res.Diagnostics
		msg.Truncated = res.DiagnosticsTruncated
		if res.PDF != nil {
			snap.pdf = res.PDF
			if n, perr := typst.PageCount(res.PDF); perr == nil {
				msg.PageCount = n
			} else {
				p.log.Warn("preview: counting the pages failed", "err", perr)
			}
			snap.pageCount = msg.PageCount
		}
		for _, page := range res.Pages {
			msg.Pages = append(msg.Pages, string(page))
		}
	}
	msg.Failure = failureOf(err)
	if msg.Failure != nil && msg.Failure.Kind == FailInternal {
		p.log.Warn("preview: compilation failed", "err", err)
	}
	b := encodeMessage(&msg)

	p.mu.Lock()
	if p.seq != run.seq || p.closed {
		p.mu.Unlock()
		return
	}
	if !run.reuse || err != nil { // else the PDF of these sources is still the one cached
		p.cache = snap
	}
	p.last, p.lastFirst, p.lastPages, p.lastCount = b, msg.First, len(msg.Pages), msg.PageCount
	cur := p.cur
	p.mu.Unlock()
	cur.send(p.log, b)
}

// deliver sends b to the webview if seq is still the latest compilation.
func (p *Preview) deliver(seq uint64, b []byte) {
	p.mu.Lock()
	if p.seq != seq || p.closed {
		p.mu.Unlock()
		return
	}
	cur := p.cur
	p.mu.Unlock()
	cur.send(p.log, b)
}

var errUnavailable = errors.New("typst could not start")

// PreviewMessage is a frame from Go on the preview stream, as JSON.
type PreviewMessage struct {
	// Type is "compiling" (a compilation started), "result" or "anchors".
	Type string `json:"type"`
	// Seq numbers the compilation; a higher Seq supersedes a lower one.
	Seq uint64 `json:"seq,omitzero"`
	// Rev numbers the version of the sources the result was made from: pages of
	// the same Rev belong together, however many compilations delivered them.
	Rev uint64 `json:"rev,omitzero"`
	// PageCount is the number of pages of the document.
	PageCount int `json:"pageCount,omitzero"`
	// First is the page number of Pages[0].
	First int `json:"first,omitzero"`
	// Pages holds the SVG documents of the pages in view.
	Pages []string `json:"pages,omitzero"`
	// Diagnostics lists the errors and warnings.
	Diagnostics []typst.Diagnostic `json:"diagnostics,omitzero"`
	// Truncated is true when Typst reported more diagnostics than are kept.
	Truncated bool `json:"truncated,omitzero"`
	// Failure says why there is no document, if there is none.
	Failure *PreviewFailure `json:"failure,omitzero"`
	// Anchors answers an "anchors" request.
	Anchors []typst.Anchor `json:"anchors,omitzero"`
}

// PreviewFailure explains why a compilation produced no document.
type PreviewFailure struct {
	// Kind is one of the Fail constants.
	Kind string `json:"kind"`
	// Packages names the missing packages for [FailPackage], as
	// "@preview/name:version".
	Packages []string `json:"packages,omitzero"`
}

// failureKinds maps the errors of a compilation, in order, to their failure kind.
var failureKinds = []struct {
	err  error
	kind string
}{
	{typst.ErrMissingPackage, FailPackage},
	{typst.ErrInvalidMain, FailNoMain},
	{typst.ErrTimeout, FailTimeout},
	{typst.ErrMemoryLimit, FailMemory},
	{typst.ErrOutputTooLarge, FailTooLarge},
	{typst.ErrUnsafeProject, FailUnsafe},
	{errUnavailable, FailUnavailable},
	{typst.ErrIntegrity, FailUnavailable},
}

// failureOf maps the error of a compilation to what the webview shows.
func failureOf(err error) *PreviewFailure {
	if err == nil {
		return nil
	}
	if ce, ok := errors.AsType[*typst.CompileError](err); ok {
		if len(ce.MissingPackages) > 0 {
			return &PreviewFailure{Kind: FailPackage, Packages: ce.MissingPackages}
		}
		return &PreviewFailure{Kind: FailErrors}
	}
	for _, f := range failureKinds {
		if errors.Is(err, f.err) {
			return &PreviewFailure{Kind: f.kind}
		}
	}
	return &PreviewFailure{Kind: FailInternal}
}

// mustJSON encodes a message that cannot fail to encode.
func mustJSON(m PreviewMessage) []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err) // a PreviewMessage always encodes
	}
	return b
}

// encodeMessage encodes m; a message too large for a frame loses its pages and
// says so.
func encodeMessage(m *PreviewMessage) []byte {
	b := mustJSON(*m)
	if len(b) <= maxPreviewFrame {
		return b
	}
	m.Pages = nil
	m.Failure = &PreviewFailure{Kind: FailTooLarge}
	return mustJSON(*m)
}

// previewConn is the webview attached to the preview stream.
type previewConn struct {
	c  bridge.Conn
	mu sync.Mutex
}

// send writes one frame; a nil receiver (no webview attached) drops it.
func (c *previewConn) send(log *slog.Logger, b []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.c.Send(bytes.Clone(b)); err != nil {
		log.Warn("preview stream: send to webview", "err", err)
	}
}

// previewRequest is a frame from the webview on the preview stream, as JSON.
type previewRequest struct {
	// Op is "view" (these pages are shown), "edit" (the document changed),
	// "refresh" (compile again) or "anchors" (where the headings and
	// paragraphs are, for the jumps).
	Op string `json:"op"`
	// First and Last are the pages shown, for "view".
	First int `json:"first,omitzero"`
	Last  int `json:"last,omitzero"`
	// Time is when the document last changed, in Unix seconds, for "edit":
	// the date every person sees in the document.
	Time int64 `json:"time,omitzero"`
}

var errBadRequest = errors.New("bad preview request")

// decodePreviewRequest parses and checks one frame of the webview.
func decodePreviewRequest(b []byte) (previewRequest, error) {
	var r previewRequest
	if len(b) > maxPreviewRequest {
		return r, fmt.Errorf("%w: %d bytes", errBadRequest, len(b))
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return previewRequest{}, fmt.Errorf("%w: %w", errBadRequest, err)
	}
	if dec.More() {
		return previewRequest{}, fmt.Errorf("%w: trailing data", errBadRequest)
	}
	switch r.Op {
	case "view":
		if r.First < 1 || r.Last < r.First || r.Last-r.First >= maxPreviewWindow || r.Last > typst.DefaultMaxPages {
			return previewRequest{}, fmt.Errorf("%w: pages %d to %d", errBadRequest, r.First, r.Last)
		}
	case "edit":
		if r.Time < 0 || r.Time > 1<<40 {
			return previewRequest{}, fmt.Errorf("%w: time %d", errBadRequest, r.Time)
		}
	case "refresh", "anchors":
	default:
		return previewRequest{}, fmt.Errorf("%w: op %q", errBadRequest, r.Op)
	}
	return r, nil
}

// Serve attaches c, the preview stream of the webview, replacing any previous
// one (a page reload). It sends the last result, then answers the requests
// until c fails. A malformed request is logged and ignored.
func (p *Preview) Serve(c bridge.Conn) {
	cur := &previewConn{c: c}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.cur = cur
	last := p.last
	if last == nil {
		p.armLocked(0) // nothing shown yet: compile for the first time
	}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		if p.cur == cur {
			p.cur = nil
		}
		p.mu.Unlock()
	}()
	if last != nil {
		cur.send(p.log, last)
	}
	for {
		b, err := c.Receive()
		if err != nil {
			return
		}
		req, err := decodePreviewRequest(b)
		if err != nil {
			p.log.Warn("preview stream: dropped request", "bytes", len(b), "err", err)
			continue
		}
		p.handle(cur, req)
	}
}

// handle runs one request.
func (p *Preview) handle(cur *previewConn, req previewRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	switch req.Op {
	case "view":
		p.win = window{req.First, req.Last}
		if !p.coversLocked(p.win) {
			p.armLocked(0)
		}
	case "edit":
		if req.Time > 0 {
			p.editTime = time.Unix(req.Time, 0).UTC()
		}
	case "refresh":
		p.srcGen++
		p.armLocked(0)
	case "anchors":
		if p.asking {
			return // the answer to the running one reaches the webview too
		}
		p.asking = true
		date, srcGen := p.docTimeLocked(), p.srcGen
		p.wg.Go(func() {
			defer func() {
				p.mu.Lock()
				p.asking = false
				p.mu.Unlock()
			}()
			p.answerAnchors(cur, srcGen, date)
		})
	}
}

// coversLocked says whether the last result already holds every page of w.
func (p *Preview) coversLocked(w window) bool {
	if p.last == nil {
		return false
	}
	have := window{p.lastFirst, p.lastFirst + p.lastPages - 1}
	last := min(w.last, p.lastCount) // pages past the end do not exist
	return last < w.first || (w.first >= have.first && last <= have.last)
}

// answerAnchors sends the anchors of the sources as they are now.
func (p *Preview) answerAnchors(cur *previewConn, srcGen uint64, date time.Time) {
	p.mu.Lock()
	cached := p.anchors
	p.mu.Unlock()
	msg := PreviewMessage{Type: "anchors"}
	if cached.ok && cached.srcGen == srcGen {
		msg.Anchors = cached.list
	} else {
		eng, err := p.engine()
		if err == nil {
			ctx, cancel := context.WithTimeout(p.ctx, anchorsTimeout)
			defer cancel()
			msg.Anchors, err = eng.Anchors(ctx, typst.Project{Root: p.opt.Root}, p.opt.Main, typst.WithTimestamp(date))
		}
		if p.ctx.Err() != nil {
			return
		}
		if err != nil {
			msg.Failure = failureOf(err)
		} else {
			p.mu.Lock()
			p.anchors = anchorCache{srcGen: srcGen, list: msg.Anchors, ok: true}
			p.mu.Unlock()
		}
	}
	cur.send(p.log, encodeMessage(&msg))
}

// ExportResult is the outcome of [PreviewService.ExportPDF].
type ExportResult struct {
	// Path is where the PDF was written; empty if nothing was.
	Path string
	// Cancelled is true when the person closed the dialog.
	Cancelled bool
	// Reason is empty on success, [FailErrors] when the document has errors, or
	// another Fail kind.
	Reason string
}

// exportPDF asks the person where to save, then writes the PDF of the document
// there. The webview never gives a path: the dialog is Go's. It returns an
// error only for what the person cannot fix (no dialog, disk).
func (p *Preview) exportPDF() (ExportResult, error) {
	if p.opt.Choose == nil {
		return ExportResult{}, ErrNoDialog
	}
	path, err := p.opt.Choose(p.exportName())
	if err != nil {
		return ExportResult{}, fmt.Errorf("choosing where to save: %w", err)
	}
	if path == "" {
		return ExportResult{Cancelled: true}, nil
	}
	if !filepath.IsAbs(path) {
		return ExportResult{}, fmt.Errorf("export path %q is not absolute", path)
	}
	path = filepath.Clean(path)
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	pdf, fail, err := p.currentPDF()
	if err != nil {
		return ExportResult{}, err
	}
	if fail != "" {
		return ExportResult{Reason: fail}, nil
	}
	if err := writeFileAtomic(path, pdf); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Path: path}, nil
}

// exportName is the suggested file name of the export.
func (p *Preview) exportName() string {
	title := ""
	if p.opt.Title != nil {
		title = p.opt.Title()
	}
	title = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return -1
		}
		return r
	}, strings.TrimSpace(title))
	if title == "" {
		title = "Document"
	}
	return title + ".pdf"
}

// currentPDF returns the PDF of the sources as they are now: the one of the
// last compilation if nothing changed since, else a new one. fail is a Fail
// kind when the document cannot be exported.
func (p *Preview) currentPDF() (pdf []byte, fail string, err error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, "", context.Canceled
	}
	if p.cache.ok && p.cache.srcGen == p.srcGen && p.cache.pdf != nil {
		pdf = p.cache.pdf
		p.mu.Unlock()
		return pdf, "", nil
	}
	date, srcGen := p.docTimeLocked(), p.srcGen
	p.mu.Unlock()

	eng, err := p.engine()
	if err != nil {
		return nil, FailUnavailable, nil
	}
	res, err := eng.Compile(p.ctx, typst.Project{Root: p.opt.Root}, p.opt.Main, typst.WithFormats(typst.FormatPDF), typst.WithTimestamp(date))
	if err != nil {
		if f := failureOf(err); f.Kind != FailInternal {
			return nil, f.Kind, nil
		}
		return nil, "", fmt.Errorf("compiling for export: %w", err)
	}
	p.mu.Lock()
	if p.srcGen == srcGen && !p.cache.ok { // keep it for the next export
		n, _ := typst.PageCount(res.PDF)
		p.cache = snapshot{srcGen: srcGen, ok: true, pdf: res.PDF, pageCount: n}
	}
	p.mu.Unlock()
	return res.PDF, "", nil
}

// writeFileAtomic writes data to path through a temporary file in the same
// folder, so that a failure leaves nothing half written. The file is the
// person's own: readable like their other documents.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".inkwell-export-*")
	if err != nil {
		return fmt.Errorf("writing the PDF: %w", err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("writing the PDF: %w", err)
	}
	// #nosec G302 -- an exported document is the person's file, not a secret of Inkwell.
	if err := os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("writing the PDF: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("writing the PDF: %w", err)
	}
	return nil
}

// DirChooser returns a [PreviewOptions.Choose] that picks name in dir without
// asking: for headless runs, where no dialog can open.
func DirChooser(dir string) func(string) (string, error) {
	return func(name string) (string, error) {
		// #nosec G703 -- dir comes from the environment of the person running Inkwell.
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", err
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		return filepath.Join(abs, name), nil
	}
}

// PreviewService is the Wails service of the preview: what the webview may ask
// besides the stream. Its exported methods are bound to the frontend.
type PreviewService struct{ p *Preview }

// NewPreviewService returns the service over p.
func NewPreviewService(p *Preview) *PreviewService { return &PreviewService{p: p} }

// NetworkMode says how Typst is kept offline: "namespace" when it runs in a
// private network namespace, "proxy only" when the system forbids that and
// only a refused proxy stands in the way. Empty when Typst cannot start.
func (s *PreviewService) NetworkMode() string { return s.p.networkMode() }

// ExportPDF asks the person where to save the PDF of the document, then writes
// it there. See [ExportResult].
func (s *PreviewService) ExportPDF() (ExportResult, error) { return s.p.exportPDF() }
