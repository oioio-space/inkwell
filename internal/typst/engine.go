// Package typst compiles Typst projects to PDF and SVG pages, offline, with
// the typst binary embedded in Inkwell ([internal/sidecar]).
//
//	eng, err := typst.New(typst.Config{})
//	res, err := eng.Compile(ctx, typst.Project{Root: dir}, "main.typ")
//	// res.PDF, res.Pages (one SVG per page), res.Diagnostics
//
// Compilation is deterministic and offline by construction:
//
//   - the output depends on the project and on the document date only: system
//     fonts are ignored (the pagination is the same on every machine), the
//     creation date is a parameter ([WithTimestamp]) and the environment of the
//     process is never inherited;
//   - Typst cannot download anything: the package cache holds the packages
//     bundled with Inkwell plus the project's own .typstdeps/ folder, and the
//     process can only reach a refused local port, so a missing package is
//     reported (see [ErrMissingPackage]) instead of fetched.
//
// A project is not trusted. A compilation has a deadline, an address-space
// limit (Linux), bounded output, and refuses a project whose symbolic links
// lead out of its folder. Typst itself refuses paths that leave the root.
package typst

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

// Defaults for [Config].
const (
	DefaultTimeout        = 60 * time.Second
	DefaultMemoryLimit    = 4 << 30
	DefaultMaxOutput      = 256 << 20
	DefaultMaxPages       = 2000
	DefaultMaxDiagnostics = 200

	maxMessage = 2000
	maxStderr  = 1 << 20

	// deadProxy refuses connections: Typst honours the proxy variables, so any
	// download it attempts ends here.
	deadProxy = "http://127.0.0.1:9"
)

// maxScanFiles bounds the entries checked for links; a variable for the tests.
var maxScanFiles = 100_000

// DefaultTimestamp is the document date used when none is given: dates in a
// document (datetime.today(), the PDF metadata) come from the document, not
// from the clock, so two compilations of the same project are identical.
var DefaultTimestamp = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Config configures an [Engine]. The zero value is ready for use.
type Config struct {
	// Store extracts and verifies the typst binary. The default is a
	// [sidecar.Store] in CacheDir.
	Store *sidecar.Store
	// Logger receives warnings (a repaired cache). The default is slog.Default.
	Logger *slog.Logger
	// CacheDir is the Inkwell cache directory; the engine uses CacheDir/typst.
	// The default is the user cache directory ([sidecar.DefaultDir]).
	CacheDir string
	// egressProxy replaces the refused proxy, for the test that proves that a
	// download attempt goes nowhere else.
	egressProxy string
	// Timeout stops a compilation that runs too long; default [DefaultTimeout].
	Timeout time.Duration
	// MemoryLimit caps the address space of Typst in bytes (Linux only);
	// default [DefaultMemoryLimit].
	MemoryLimit uint64
	// MaxOutput bounds the bytes of a PDF, of all the SVG pages together and of
	// a query result; default [DefaultMaxOutput].
	MaxOutput int64
	// MaxPages bounds the number of pages; default [DefaultMaxPages].
	MaxPages int
	// MaxConcurrent bounds the compilations running at once (each is one or two
	// Typst processes); the others wait. Default: half the number of CPUs, at least 1.
	MaxConcurrent int
	// proxyOnly turns the network namespace off, for the test that watches the
	// refused proxy.
	proxyOnly bool
	// MaxDiagnostics bounds the diagnostics kept; default [DefaultMaxDiagnostics].
	MaxDiagnostics int
}

// Project is a folder of Typst sources and assets.
type Project struct {
	// Root is the project folder: Typst's --root. Files outside it are out of
	// reach. A package vendored in Root/.typstdeps/preview/<name>/<version> is
	// available to the document; so are fonts in Root/fonts.
	Root string
}

// Format selects the outputs of [Engine.Compile].
type Format uint8

// The formats; combine with |.
const (
	// FormatPDF produces [Result.PDF].
	FormatPDF Format = 1 << iota
	// FormatSVG produces [Result.Pages].
	FormatSVG
)

// SVG is one page rendered as an SVG document.
type SVG []byte

// Result is what a compilation produced.
type Result struct {
	// PDF is the document, with its pdf.attach files ([Attachments]).
	PDF []byte
	// Pages holds one SVG document per page, in order.
	Pages []SVG
	// Diagnostics holds the errors and warnings Typst reported. A warning does
	// not make the compilation fail.
	Diagnostics []Diagnostic
	// Elapsed is how long the compilation took.
	Elapsed time.Duration
	// DiagnosticsTruncated is true when Typst reported more than MaxDiagnostics.
	DiagnosticsTruncated bool
}

// Option adjusts one call.
type Option func(*options)

type options struct {
	timestamp time.Time
	formats   Format
	first     int // first page of SVG output, 0 = 1
	last      int // last page of SVG output, 0 = as many as MaxPages
}

// WithPages renders only pages first to last (1-based, inclusive) as SVG, the
// ones a viewer shows, instead of the whole document. The PDF is always whole.
func WithPages(first, last int) Option {
	return func(o *options) { o.first, o.last = first, last }
}

// WithFormats chooses the outputs; the default is FormatPDF | FormatSVG.
func WithFormats(f Format) Option { return func(o *options) { o.formats = f } }

// WithTimestamp sets the document date (creation date and datetime.today()).
// The default is [DefaultTimestamp]; the application passes the time of the
// last change so that every person sees the same date.
func WithTimestamp(t time.Time) Option { return func(o *options) { o.timestamp = t } }

func applyOptions(opts []Option) options {
	o := options{formats: FormatPDF | FormatSVG, timestamp: DefaultTimestamp}
	for _, f := range opts {
		f(&o)
	}
	return o
}

// Engine compiles projects. It is safe for concurrent use.
type Engine struct {
	store *sidecar.Store
	log   *slog.Logger
	pkgs  *packageCache
	fonts *fontCache
	tmp   string
	home  string
	local string
	proxy string
	cfg   Config
	sem   chan struct{}
	noNet bool
}

// New returns an engine whose files live in cfg.CacheDir/typst, extracting the
// bundled packages and fonts. The typst binary is verified (and extracted) at
// the first use, and again before every launch.
func New(cfg Config) (*Engine, error) {
	if cfg.CacheDir == "" {
		dir, err := sidecar.DefaultDir()
		if err != nil {
			return nil, err
		}
		cfg.CacheDir = dir
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MemoryLimit == 0 {
		cfg.MemoryLimit = DefaultMemoryLimit
	}
	if cfg.MaxOutput <= 0 {
		cfg.MaxOutput = DefaultMaxOutput
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = max(1, runtime.NumCPU()/2)
	}
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = DefaultMaxPages
	}
	if cfg.MaxDiagnostics <= 0 {
		cfg.MaxDiagnostics = DefaultMaxDiagnostics
	}
	e := &Engine{
		cfg: cfg, store: cfg.Store, log: cfg.Logger, proxy: cmp.Or(cfg.egressProxy, deadProxy),
		sem: make(chan struct{}, cfg.MaxConcurrent),
	}
	if e.log == nil {
		e.log = slog.Default()
	}
	if e.store == nil {
		e.store = &sidecar.Store{Dir: cfg.CacheDir, Logger: e.log}
	}
	base := filepath.Join(cfg.CacheDir, "typst")
	e.pkgs = newPackageCache(filepath.Join(base, "packages"))
	e.fonts = newFontCache(filepath.Join(base, "fonts"))
	e.tmp, e.home, e.local = filepath.Join(base, "tmp"), filepath.Join(base, "home"), filepath.Join(base, "local")
	for _, d := range []string{e.tmp, e.home, e.local} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("typst: %w", err)
		}
	}
	if err := e.ensureCaches(); err != nil {
		return nil, err
	}
	e.noNet = !cfg.proxyOnly && sidecar.NetworkIsolation()
	e.log.Info("typst: offline", "network", e.NetworkMode())
	return e, nil
}

// NetworkMode says how Typst is kept offline: "namespace" when it runs in a
// private network namespace (not even the loopback is reachable), "proxy only"
// when the system forbids that and only the refused proxy stands in the way.
func (e *Engine) NetworkMode() string {
	if e.noNet {
		return "namespace"
	}
	return "proxy only"
}

// acquire waits for a free compilation slot.
func (e *Engine) acquire(ctx context.Context) (release func(), err error) {
	select {
	case e.sem <- struct{}{}:
		return func() { <-e.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ensureCaches verifies the bundled packages and fonts, repairing them.
func (e *Engine) ensureCaches() error {
	fixed, err := e.pkgs.Ensure()
	if err != nil {
		return err
	}
	if len(fixed) > 0 {
		e.log.Info("typst: bundled packages extracted", "packages", fixed)
	}
	if changed, err := e.fonts.Ensure(); err != nil {
		return err
	} else if changed {
		e.log.Info("typst: bundled fonts extracted")
	}
	return nil
}

// job is a validated request, ready to launch.
type job struct {
	cleanup func()
	root    string
	main    string
	typst   string
	pkgPath string
	fonts   []string
}

// prepare validates the project and the main file and gathers what a launch needs.
func (e *Engine) prepare(p Project, main string) (*job, error) {
	root, err := realRoot(p.Root)
	if err != nil {
		return nil, err
	}
	rel := filepath.Clean(filepath.FromSlash(main))
	if main == "" || !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidMain, main)
	}
	if err := statInRoot(root, rel); err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidMain, main, err)
	}
	if err := scanLinks(root); err != nil {
		return nil, err
	}
	if err := e.ensureCaches(); err != nil {
		return nil, err
	}
	bin, err := e.store.Resolve(sidecar.Typst)
	if err != nil {
		return nil, err
	}
	j := &job{root: root, main: rel, typst: bin, fonts: []string{e.fonts.root}, cleanup: func() {}}
	if info, err := os.Stat(filepath.Join(root, "fonts")); err == nil && info.IsDir() {
		j.fonts = append(j.fonts, filepath.Join(root, "fonts"))
	}
	if j.pkgPath, j.cleanup, err = e.packageView(root); err != nil {
		return nil, err
	}
	return j, nil
}

// realRoot returns the absolute, symlink-free path of the project folder.
func realRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("typst: no project folder")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("typst: project folder: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("typst: project folder: %w", err)
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return "", fmt.Errorf("typst: project folder %q is not a directory", root)
	}
	return resolved, nil
}

// statInRoot checks that rel is a regular file reachable without leaving root.
func statInRoot(root, rel string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	info, err := r.Stat(rel)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("not a regular file")
	}
	return nil
}

// scanLinks refuses a project with a symbolic link whose target is outside the
// project. Typst checks that paths stay inside the root but follows a link
// that is inside to a target that is not, so a link would read any file the
// user can. A broken link is harmless: there is nothing to read.
func scanLinks(root string) error {
	count := 0
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil // an unreadable subfolder cannot be followed either
		}
		if count++; count > maxScanFiles {
			return fmt.Errorf("%w: more than %d files to check", ErrUnsafeProject, maxScanFiles)
		}
		if d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil
		}
		if _, inside := within(root, target); !inside && target != root {
			rel, _ := filepath.Rel(root, path)
			return fmt.Errorf("%w: %s", ErrUnsafeProject, filepath.ToSlash(rel))
		}
		return nil
	})
}

// packageView returns the directory to pass as --package-cache-path. Typst takes
// one, so when the project vendors packages in .typstdeps a view of both is
// built from links; a bundled package wins over a vendored one of the same
// name and version, so a project cannot replace code Inkwell pins.
func (e *Engine) packageView(root string) (path string, cleanup func(), err error) {
	deps := filepath.Join(root, ".typstdeps", "preview")
	names, err := os.ReadDir(deps)
	if err != nil {
		return e.pkgs.root, func() {}, nil // no vendored packages
	}
	view, err := os.MkdirTemp(e.tmp, "pkgs-")
	if err != nil {
		return "", nil, fmt.Errorf("typst: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(view) }
	link := func(name, version, target string) error {
		dir := filepath.Join(view, "preview", name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		return os.Symlink(target, filepath.Join(dir, version))
	}
	bundledSet := map[string]bool{}
	pkgs, err := loadBundled()
	if err == nil {
		for _, p := range pkgs {
			bundledSet[p.Name+"/"+p.Version] = true
			err = errors.Join(err, link(p.Name, p.Version, e.pkgs.dir(p)))
		}
	}
	for _, n := range names {
		if !n.IsDir() || !pkgNameRE.MatchString(n.Name()) {
			continue
		}
		versions, readErr := os.ReadDir(filepath.Join(deps, n.Name()))
		if readErr != nil {
			continue
		}
		for _, v := range versions {
			if !v.IsDir() || !versionRE.MatchString(v.Name()) || bundledSet[n.Name()+"/"+v.Name()] {
				continue
			}
			err = errors.Join(err, link(n.Name(), v.Name(), filepath.Join(deps, n.Name(), v.Name())))
		}
	}
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("typst: package view: %w", err)
	}
	return view, cleanup, nil
}

// env is the whole environment of Typst: nothing of the caller's, and every
// proxy variable pointing at a refused port.
func (e *Engine) env() []string {
	env := []string{"HOME=" + e.home, "XDG_CACHE_HOME=" + e.home, "XDG_DATA_HOME=" + e.home, "LANG=C.UTF-8", "NO_PROXY=", "no_proxy="}
	for _, k := range []string{"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "https_proxy", "http_proxy", "all_proxy"} {
		env = append(env, k+"="+e.proxy)
	}
	return env
}

// args builds the command line shared by compile and query.
func (e *Engine) args(sub string, j *job, o options, extra ...string) []string {
	args := []string{
		sub, "--diagnostic-format", "short",
		"--root", j.root,
		"--ignore-system-fonts",
		"--font-path", strings.Join(j.fonts, string(os.PathListSeparator)),
		"--package-path", e.local,
		"--package-cache-path", j.pkgPath,
		"--creation-timestamp", strconv.FormatInt(o.timestamp.Unix(), 10),
	}
	return append(args, extra...)
}

// run launches typst with the engine's limits.
//
// The limits are in force from the first instruction of Typst. watch, if not
// empty, is the folder Typst writes its output to: it is bounded to MaxOutput
// as it grows (no single file may exceed it either), so that a document that
// renders gigabytes is stopped long before the compilation ends.
func (e *Engine) run(ctx context.Context, j *job, args []string, maxOut int, watch string) (sidecar.Output, error) {
	spec := sidecar.Spec{
		Path: j.typst, Args: args, Dir: j.root, Env: e.env(), NoNetwork: e.noNet,
		MemoryLimit: e.cfg.MemoryLimit, MaxOutput: maxOut,
	}
	if watch != "" {
		spec.WatchDir, spec.MaxDirBytes, spec.FileSizeLimit = watch, e.cfg.MaxOutput, uint64(e.cfg.MaxOutput) // #nosec G115 -- MaxOutput is positive
	}
	return sidecar.Run(ctx, spec)
}

// deadline derives the context of one compilation. When the timeout is what
// ended it, the cause is [ErrTimeout]; a cancellation by the caller stays one.
func (e *Engine) deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeoutCause(ctx, e.cfg.Timeout, ErrTimeout)
}

// ended turns the end of a context into the error to return.
func ended(ctx context.Context, timeout time.Duration) error {
	if errors.Is(context.Cause(ctx), ErrTimeout) {
		return fmt.Errorf("%w (limit %v)", ErrTimeout, timeout)
	}
	return ctx.Err()
}

// Compile compiles main (a path relative to the project folder) and returns
// the requested outputs. When the document has errors the returned error is a
// [*CompileError] (errors.Is ErrCompile) and the Result, still non-nil, holds
// the diagnostics only. Other errors: [ErrInvalidMain], [ErrUnsafeProject],
// [ErrTimeout], [ErrMemoryLimit], [ErrOutputTooLarge]; or the context's error
// when ctx ends first.
func (e *Engine) Compile(ctx context.Context, p Project, main string, opts ...Option) (*Result, error) {
	o := applyOptions(opts)
	if o.formats&(FormatPDF|FormatSVG) == 0 {
		return nil, errors.New("typst: no output format requested")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	out, err := os.MkdirTemp(e.tmp, "out-")
	if err != nil {
		return nil, fmt.Errorf("typst: %w", err)
	}
	defer func() { _ = os.RemoveAll(out) }()
	pdfDir, svgDir := filepath.Join(out, "pdf"), filepath.Join(out, "svg")
	for _, d := range []string{pdfDir, svgDir} {
		if err := os.Mkdir(d, 0o700); err != nil {
			return nil, fmt.Errorf("typst: %w", err)
		}
	}

	begin := time.Now()
	runCtx, cancel := e.deadline(ctx)
	defer cancel()
	pdfPath, svgTemplate := filepath.Join(pdfDir, "out.pdf"), filepath.Join(svgDir, "page-{0p}.svg")
	// One page more than allowed, so that a document over the limit is noticed.
	first := max(o.first, 1)
	last := first + e.cfg.MaxPages
	if o.last > 0 {
		last = min(o.last, last-1)
	}
	pages := strconv.Itoa(first) + "-" + strconv.Itoa(last)
	var (
		wg           sync.WaitGroup
		pdfOut, svgO sidecar.Output
		pdfErr, svgE error
	)
	if o.formats&FormatPDF != 0 {
		wg.Go(func() {
			pdfOut, pdfErr = e.run(runCtx, j, e.args("compile", j, o, "--format", "pdf", "--", filepath.ToSlash(j.main), pdfPath), maxStderr, pdfDir)
		})
	}
	if o.formats&FormatSVG != 0 {
		wg.Go(func() {
			svgO, svgE = e.run(runCtx, j, e.args("compile", j, o, "--format", "svg", "--pages", pages, "--", filepath.ToSlash(j.main), svgTemplate), maxStderr, svgDir)
		})
	}
	wg.Wait()
	if err := errors.Join(pdfErr, svgE); err != nil {
		if runCtx.Err() != nil {
			return nil, ended(runCtx, e.cfg.Timeout)
		}
		return nil, err
	}

	res := &Result{Elapsed: time.Since(begin)}
	var ran []sidecar.Output
	if o.formats&FormatPDF != 0 {
		ran = append(ran, pdfOut)
	}
	if o.formats&FormatSVG != 0 {
		ran = append(ran, svgO)
	}
	if err := e.settle(res, j, ran); err != nil {
		return res, err
	}
	if o.formats&FormatPDF != 0 {
		if res.PDF, err = e.readPDF(pdfPath); err != nil {
			return res, err
		}
	}
	if o.formats&FormatSVG != 0 {
		if res.Pages, err = e.readPages(svgDir); err != nil {
			return res, err
		}
	}
	return res, nil
}

// settle merges the diagnostics of the runs into res and turns a failed run
// into the error to return.
func (e *Engine) settle(res *Result, j *job, runs []sidecar.Output) error {
	lim := limits{maxDiagnostics: e.cfg.MaxDiagnostics, maxMessage: maxMessage, maxInput: maxStderr}
	var failed *sidecar.Output
	for _, run := range runs {
		diags, truncated := parseDiagnostics(run.Stderr, j.root, j.pkgPath, lim)
		res.DiagnosticsTruncated = res.DiagnosticsTruncated || truncated
		for _, d := range diags {
			if !slices.Contains(res.Diagnostics, d) && len(res.Diagnostics) < lim.maxDiagnostics {
				res.Diagnostics = append(res.Diagnostics, d)
			}
		}
		if run.ExitCode != 0 && failed == nil {
			failed = &run
		}
	}
	if failed == nil {
		return nil
	}
	return failure(res, failed, e.cfg.MemoryLimit)
}

// failure explains why typst exited abnormally.
func failure(res *Result, run *sidecar.Output, memoryLimit uint64) error {
	// SIGXFSZ (25 on Linux and macOS): a file grew past the size limit.
	if run.OutputLimit || run.Signal == 25 {
		return fmt.Errorf("%w: the output grew past the limit while it was being written", ErrOutputTooLarge)
	}
	stderr := string(run.Stderr)
	if strings.Contains(stderr, "memory allocation of") && strings.Contains(stderr, "failed") {
		return fmt.Errorf("%w (limit %d MiB)", ErrMemoryLimit, memoryLimit>>20)
	}
	ce := &CompileError{Diagnostics: res.Diagnostics}
	for _, d := range res.Diagnostics {
		if d.Package != "" && !slices.Contains(ce.MissingPackages, d.Package) {
			ce.MissingPackages = append(ce.MissingPackages, d.Package)
		}
	}
	if slices.ContainsFunc(res.Diagnostics, func(d Diagnostic) bool { return d.Severity == Error }) {
		return ce
	}
	first, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
	if run.ExitCode < 0 {
		return fmt.Errorf("typst: the compiler was stopped by a signal: %s", truncate(first, 200))
	}
	return fmt.Errorf("typst: the compiler exited with status %d: %s", run.ExitCode, truncate(first, 200))
}

func (e *Engine) readPDF(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("typst: no PDF was produced: %w", err)
	}
	if info.Size() > e.cfg.MaxOutput {
		return nil, fmt.Errorf("%w: the PDF is %d bytes, the limit is %d", ErrOutputTooLarge, info.Size(), e.cfg.MaxOutput)
	}
	return os.ReadFile(path) // #nosec G304 -- inside the engine's private temporary folder
}

func (e *Engine) readPages(dir string) ([]SVG, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("typst: %w", err)
	}
	var names []string
	for _, ent := range entries {
		if strings.HasPrefix(ent.Name(), "page-") && strings.HasSuffix(ent.Name(), ".svg") {
			names = append(names, ent.Name())
		}
	}
	slices.Sort(names) // the numbers are zero-padded
	if len(names) == 0 {
		return nil, errors.New("typst: no page was produced")
	}
	if len(names) > e.cfg.MaxPages {
		return nil, fmt.Errorf("%w: %d pages, the limit is %d", ErrOutputTooLarge, len(names), e.cfg.MaxPages)
	}
	pages := make([]SVG, 0, len(names))
	var total int64
	for _, n := range names {
		path := filepath.Join(dir, n)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("typst: %w", err)
		}
		if total += info.Size(); total > e.cfg.MaxOutput {
			return nil, fmt.Errorf("%w: the SVG pages exceed %d bytes", ErrOutputTooLarge, e.cfg.MaxOutput)
		}
		data, err := os.ReadFile(path) // #nosec G304 -- inside the engine's private temporary folder
		if err != nil {
			return nil, fmt.Errorf("typst: %w", err)
		}
		pages = append(pages, SVG(data))
	}
	return pages, nil
}
