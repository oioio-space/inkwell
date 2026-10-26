package typst

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"image"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

// noisePNG is an incompressible image of about size bytes.
func noisePNG(t testing.TB, size int) string {
	t.Helper()
	side := 1
	for side*side*3 < size {
		side++
	}
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	if _, err := rand.Read(img.Pix); err != nil {
		t.Fatal(err)
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// FINDING 2, scaled down: a 1 MiB picture drawn 100 times is some 130 MB of
// SVG. The bound is hit while Typst writes, not after.
func TestCompileOutputBoundedWhileWriting(t *testing.T) {
	e := newEngine(t, func(c *Config) { c.MaxOutput = 20 << 20 })
	p := writeProject(t, map[string]string{
		"noise.png": noisePNG(t, 1<<20),
		"main.typ":  "#for i in range(100) { image(\"noise.png\", width: 1cm) }\n",
	})
	begin := time.Now()
	_, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG))
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("Compile = %v, want ErrOutputTooLarge", err)
	}
	if took := time.Since(begin); took > 15*time.Second {
		t.Errorf("took %v", took)
	}
	if left := readDirNames(t, e.tmp); len(left) != 0 {
		t.Errorf("temporary folder holds %v", left)
	}
}

func TestCompilePageWindow(t *testing.T) {
	e := newEngine(t, nil)
	p := writeProject(t, map[string]string{"main.typ": "#for i in range(6) [Page #i #pagebreak()]\n"})

	res, err := e.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG), WithPages(2, 3))
	if err != nil || len(res.Pages) != 2 {
		t.Fatalf("WithPages(2,3) = %v pages, %v; want 2", len(res.Pages), err)
	}
	res, err = e.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG), WithPages(5, 100))
	if err != nil || len(res.Pages) < 1 {
		t.Fatalf("WithPages(5,100) = %v, %v; want the pages that exist", res, err)
	}
	small := newEngine(t, func(c *Config) { c.MaxPages = 3 })
	if _, err := small.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG)); !errors.Is(err, ErrOutputTooLarge) {
		t.Errorf("6 pages with MaxPages 3 = %v, want ErrOutputTooLarge", err)
	}
	res, err = small.Compile(t.Context(), p, "main.typ", WithFormats(FormatSVG), WithPages(1, 50))
	if err != nil || len(res.Pages) != 3 {
		t.Errorf("an explicit window is capped to MaxPages: %v pages, %v", len(res.Pages), err)
	}
}

// FINDING 3: Typst runs without network when the system allows it.
func TestCompileNetworkMode(t *testing.T) {
	e := newEngine(t, nil)
	if want := map[bool]string{true: "namespace", false: "proxy only"}[sidecar.NetworkIsolation()]; e.NetworkMode() != want {
		t.Errorf("NetworkMode() = %q, want %q", e.NetworkMode(), want)
	}
	proxyOnly := newEngine(t, func(c *Config) { c.proxyOnly = true })
	if proxyOnly.NetworkMode() != "proxy only" {
		t.Errorf("NetworkMode() = %q with proxyOnly", proxyOnly.NetworkMode())
	}
	// Either way a missing package is reported, never fetched.
	p := writeProject(t, map[string]string{"main.typ": "#import \"@preview/ghost:1.2.3\": *\n"})
	for _, eng := range []*Engine{e, proxyOnly} {
		if _, err := eng.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)); !errors.Is(err, ErrMissingPackage) {
			t.Errorf("%s: Compile = %v, want ErrMissingPackage", eng.NetworkMode(), err)
		}
	}
}

// FINDING 4: at most MaxConcurrent compilations at once, the others wait.
func TestCompileConcurrencyCap(t *testing.T) {
	const slots = 2
	e := newEngine(t, func(c *Config) { c.MaxConcurrent = slots })
	bin, err := sharedStore.Resolve(sidecar.Typst)
	if err != nil {
		t.Fatal(err)
	}
	p := writeProject(t, map[string]string{"main.typ": "#{ let n = 0; for i in range(2000) { for j in range(2000) { n += 1 } } }\nDone\n"})

	var peak atomic.Int32
	stop := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Go(func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
				if n := int32(len(typstProcesses(t, bin))); n > peak.Load() {
					peak.Store(n)
				}
			}
		}
	})
	var wg sync.WaitGroup
	errs := make([]error, 6)
	for i := range errs {
		wg.Go(func() { _, errs[i] = e.Compile(t.Context(), p, "main.typ", WithFormats(FormatPDF)) })
	}
	wg.Wait()
	close(stop)
	watcher.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("compilation %d: %v", i, err)
		}
	}
	if got := peak.Load(); got > slots {
		t.Errorf("%d Typst processes at once, want at most %d", got, slots)
	}
	if peak.Load() < 1 {
		t.Error("never saw a Typst process: the test measured nothing")
	}
}

func TestCompileWaitsForASlotUntilCancelled(t *testing.T) {
	e := newEngine(t, func(c *Config) { c.MaxConcurrent = 1 })
	e.sem <- struct{}{} // the only slot is taken
	defer func() { <-e.sem }()
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	p := writeProject(t, map[string]string{"main.typ": "Hi\n"})
	if _, err := e.Compile(ctx, p, "main.typ"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Compile = %v, want the context's error while waiting", err)
	}
	if _, err := e.Query(ctx, p, "main.typ", "heading"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Query = %v, want the context's error while waiting", err)
	}
}
