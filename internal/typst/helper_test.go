package typst

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

// The tests that run the real typst share one cache directory (and so one
// extraction of the binary, the packages and the fonts).
var (
	sharedCache string
	sharedStore *sidecar.Store
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "inkwell-typst-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sharedCache = dir
	sharedStore = &sidecar.Store{Dir: dir}
	code := m.Run()
	_ = os.RemoveAll(dir)
	if code == 0 {
		if err := goleak.Find(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

// newEngine returns an engine over the shared cache; adjust changes its Config.
func newEngine(t *testing.T, adjust func(*Config)) *Engine {
	t.Helper()
	cfg := Config{CacheDir: sharedCache, Store: sharedStore}
	if adjust != nil {
		adjust(&cfg)
	}
	e, err := New(cfg)
	if err != nil {
		t.Fatalf("New() = %v (did you run `mise run sidecar:fetch`?)", err)
	}
	return e
}

// writeProject creates a project folder holding files (path -> content).
func writeProject(t *testing.T, files map[string]string) Project {
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
	return Project{Root: root}
}

// pngBytes is a small real PNG.
func pngBytes(t testing.TB) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

var pageRE = regexp.MustCompile(`/Type\s*/Page\b`)

// pdfPages counts the page objects of a PDF.
func pdfPages(pdf []byte) int { return len(pageRE.FindAll(pdf, -1)) }

func assertPDF(t *testing.T, pdf []byte) {
	t.Helper()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(pdf), []byte("%%EOF")) {
		n := min(len(pdf), 16)
		t.Fatalf("not a complete PDF: %d bytes starting %q", len(pdf), pdf[:n])
	}
}

func assertSVGs(t *testing.T, pages []SVG) {
	t.Helper()
	if len(pages) == 0 {
		t.Fatal("no SVG page")
	}
	for i, p := range pages {
		if !strings.Contains(string(p[:min(len(p), 512)]), "<svg") {
			t.Fatalf("page %d is not an SVG: %.80q", i+1, p)
		}
	}
}

// typstProcesses lists the running processes whose executable is path.
func typstProcesses(t *testing.T, path string) []int {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Skip("no /proc on this system")
	}
	var pids []int
	for _, e := range entries {
		var pid int
		if _, err := fmt.Sscanf(e.Name(), "%d", &pid); err != nil {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if err != nil || exe != path {
			continue
		}
		if stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat")); err == nil {
			if i := bytes.LastIndexByte(stat, ')'); i >= 0 && i+2 < len(stat) && stat[i+2] == 'Z' {
				continue // a zombie is already dead
			}
		}
		pids = append(pids, pid)
	}
	return pids
}

// readDirNames lists the entry names of dir.
func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}
