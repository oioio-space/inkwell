package typst

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

func TestFailure(t *testing.T) {
	missing := Diagnostic{Severity: Error, Message: "m", Package: "@preview/ghost:1.0.0"}
	tests := []struct {
		name    string
		run     sidecar.Output
		res     *Result
		wantIs  error
		wantMsg string
	}{
		{
			name:   "Rust ran out of memory",
			run:    sidecar.Output{ExitCode: -1, Stderr: []byte("memory allocation of 2147483664 bytes failed\n")},
			res:    &Result{},
			wantIs: ErrMemoryLimit,
		},
		{
			name:   "errors in the document",
			run:    sidecar.Output{ExitCode: 1},
			res:    &Result{Diagnostics: []Diagnostic{missing, {Severity: Error, Message: "other"}, missing}},
			wantIs: ErrMissingPackage,
		},
		{
			name:    "killed by a signal with nothing to say",
			run:     sidecar.Output{ExitCode: -1, Stderr: []byte("Segmentation fault\nmore")},
			res:     &Result{},
			wantMsg: "stopped by a signal: Segmentation fault",
		},
		{
			name:    "a bare failure status",
			run:     sidecar.Output{ExitCode: 2, Stderr: []byte("  something odd  \n")},
			res:     &Result{Diagnostics: []Diagnostic{{Severity: Warning, Message: "w"}}},
			wantMsg: "exited with status 2: something odd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := failure(tt.res, &tt.run, DefaultMemoryLimit)
			if err == nil {
				t.Fatal("failure() = nil")
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("failure() = %v, want %v", err, tt.wantIs)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("failure() = %q, want it to contain %q", err, tt.wantMsg)
			}
			if ce, ok := errors.AsType[*CompileError](err); ok && tt.name == "errors in the document" {
				if len(ce.MissingPackages) != 1 {
					t.Errorf("MissingPackages = %v, want the package once", ce.MissingPackages)
				}
			}
		})
	}
}

func TestPDFStrings(t *testing.T) {
	t.Run("literal escapes", func(t *testing.T) {
		got := decodeString(`(a\nb\rc\td\be\ff\(g\)\\h\101\0\12\
i)`)
		want := "a\nb\rc\td\be\ff(g)\\hA\x00\ni"
		if got != want {
			t.Errorf("decodeString = %q, want %q", got, want)
		}
		if got := decodeString("(x\\\r\ny)"); got != "xy" {
			t.Errorf("CRLF continuation = %q, want xy", got)
		}
		if got := decodeString(`(trailing\)`); got != `trailing\` {
			t.Errorf("trailing backslash = %q", got)
		}
	})
	t.Run("hex strings", func(t *testing.T) {
		for in, want := range map[string]string{
			"<48 65 6C6c6F>": "Hello",
			"<414>":          "A@",
			"<41zz42>":       "A",
			"<>":             "",
		} {
			if got := decodeString(in); got != want {
				t.Errorf("decodeString(%s) = %q, want %q", in, got, want)
			}
		}
	})
	t.Run("encodings", func(t *testing.T) {
		if got := decodeString("<FEFF00E9>"); got != "é" {
			t.Errorf("UTF-16 = %q", got)
		}
		if got := decodeString("(\xe9t\xe9)"); got != "été" { // Latin-1
			t.Errorf("Latin-1 = %q", got)
		}
		if got := decodeString("(été)"); got != "été" {
			t.Errorf("UTF-8 = %q", got)
		}
		for _, in := range []string{"", "x", "/Name", "(", "<"} {
			if got := decodeString(in); got != "" {
				t.Errorf("decodeString(%q) = %q, want empty", in, got)
			}
		}
	})
	t.Run("names", func(t *testing.T) {
		for in, want := range map[string]string{
			"/text#2Fcsv": "text/csv",
			"/a#zzb":      "a#zzb",
			"/a#2":        "a#2",
			"text":        "",
			"":            "",
		} {
			if got := decodeName(in); got != want {
				t.Errorf("decodeName(%q) = %q, want %q", in, got, want)
			}
		}
	})
}

func TestAttachmentsMalformed(t *testing.T) {
	deep := strings.Repeat("<<", maxNesting+5) + strings.Repeat(">>", maxNesting+5)
	tests := map[string][]byte{
		"key that is not a name":          pdf("<</Type/Filespec 5 6>>"),
		"unterminated dictionary":         []byte("%PDF-1.7\n1 0 obj\n<</Type/Filespec"),
		"unterminated hex string":         pdf("<</Type/Filespec/F<4142>>"),
		"unterminated literal string":     []byte("%PDF-1.7\n1 0 obj\n<</Type/Filespec/F(abc"),
		"unterminated array":              []byte("%PDF-1.7\n1 0 obj\n<</Type/Filespec/X[1 2"),
		"missing value":                   []byte("%PDF-1.7\n1 0 obj\n<</Type"),
		"dictionary nested too deeply":    pdf("<</Type/Filespec/X" + deep + ">>"),
		"array nested too deeply":         pdf("<</Type/Filespec/X" + strings.Repeat("[", maxNesting+5) + strings.Repeat("]", maxNesting+5) + ">>"),
		"no endobj for a plain object":    []byte("%PDF-1.7\n1 0 obj\n[1 2 3]"),
		"stream without endstream":        []byte("%PDF-1.7\n1 0 obj\n<</Type/Filespec/Length 3 0 R>>\nstream\nabc"),
		"stream that ends elsewhere":      []byte("%PDF-1.7\n1 0 obj\n<</Length 2>>\nstream\nabcdef\nendstream\nendobj\n"),
		"EF that is not a dictionary":     pdf("<</Type/Filespec/F(a)/EF 7>>"),
		"EF without a reference":          pdf("<</Type/Filespec/F(a)/EF<</F 7>>>>"),
		"too many objects":                manyObjects(maxPDFObjects + 1),
		"unexpected delimiter as a value": pdf("<</Type/Filespec/F )>>"),
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			if got, err := Attachments(in); err == nil {
				t.Errorf("Attachments = %+v, nil; want an error", got)
			}
		})
	}

	t.Run("a stream with the declared length and a trailing carriage return", func(t *testing.T) {
		file := "%PDF-1.7\n1 0 obj\n<</Type/Filespec/F(a)/EF<</F 2 0 R>>>>\nendobj\n2 0 obj\n<</Type/EmbeddedFile/Length 3>>\r\nstream\r\nabc\r\nendstream\r\nendobj\n"
		got, err := Attachments([]byte(file))
		if err != nil || len(got) != 1 || string(got[0].Data) != "abc" {
			t.Fatalf("Attachments = %+v, %v", got, err)
		}
	})

	t.Run("a repeated object keeps the first dictionary entry and the last object", func(t *testing.T) {
		file := "%PDF-1.7\n1 0 obj\n<</Type/Filespec/F(a)/F(b)/EF<</F 2 0 R>>>>\nendobj\n2 0 obj\n<</Type/EmbeddedFile/Length 1>>\nstream\nx\nendstream\nendobj\n2 0 obj\n<</Type/EmbeddedFile/Length 1>>\nstream\ny\nendstream\nendobj\n"
		got, err := Attachments([]byte(file))
		if err != nil || len(got) != 1 || got[0].Name != "a" || string(got[0].Data) != "y" {
			t.Fatalf("Attachments = %+v, %v", got, err)
		}
	})

	t.Run("an attachment too large to keep as it is", func(t *testing.T) {
		old := maxAttachmentBytes
		maxAttachmentBytes = 4
		t.Cleanup(func() { maxAttachmentBytes = old })
		if _, err := Attachments(pdf(filespec("a", 2, ""), stream("/Type/EmbeddedFile", "0123456789"))); err == nil {
			t.Error("Attachments = nil for a stream past the bound")
		}
	})
}

func manyObjects(n int) []byte {
	var sb strings.Builder
	sb.WriteString("%PDF-1.7\n")
	for i := range n {
		fmt.Fprintf(&sb, "%d 0 obj\n1\nendobj\n", i+1)
	}
	return []byte(sb.String())
}

func TestLoadPins(t *testing.T) {
	t.Run("packages.json", func(t *testing.T) {
		old := packagesJSON
		t.Cleanup(func() { packagesJSON = old })
		for name, doc := range map[string]string{
			"not json":          "{",
			"unknown field":     `{"schema":1,"packages":[],"x":1}`,
			"no packages":       `{"schema":1,"packages":[]}`,
			"wrong schema":      `{"schema":2,"packages":[{"namespace":"preview","name":"a","version":"1.0.0","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
			"foreign namespace": `{"schema":1,"packages":[{"namespace":"evil","name":"a","version":"1.0.0","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
			"path in the name":  `{"schema":1,"packages":[{"namespace":"preview","name":"../a","version":"1.0.0","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
			"bad version":       `{"schema":1,"packages":[{"namespace":"preview","name":"a","version":"1","sha256":"` + strings.Repeat("a", 64) + `"}]}`,
			"short hash":        `{"schema":1,"packages":[{"namespace":"preview","name":"a","version":"1.0.0","sha256":"abc"}]}`,
		} {
			packagesJSON = []byte(doc)
			if _, err := loadBundled(); err == nil {
				t.Errorf("loadBundled(%s) = nil, want an error", name)
			}
			if _, err := New(Config{CacheDir: t.TempDir(), Store: sharedStore}); err == nil {
				t.Errorf("New with %s = nil, want an error", name)
			}
		}
	})
	t.Run("fonts.json", func(t *testing.T) {
		old := fontsJSON
		t.Cleanup(func() { fontsJSON = old })
		for name, doc := range map[string]string{
			"not json":      "{",
			"no files":      `{"schema":1,"files":[]}`,
			"wrong schema":  `{"schema":3,"files":[{"name":"a","sha256":"b"}]}`,
			"unknown field": `{"schema":1,"files":[{"name":"a","sha256":"b"}],"x":1}`,
		} {
			fontsJSON = []byte(doc)
			if _, err := loadFonts(); err == nil {
				t.Errorf("loadFonts(%s) = nil, want an error", name)
			}
		}
		fontsJSON = []byte(`{"schema":1,"files":[{"name":"missing.ttf","sha256":"` + strings.Repeat("a", 64) + `"}]}`)
		if _, err := newFontCache(t.TempDir()).Ensure(); err == nil {
			t.Error("Ensure with a pin for a file that is not embedded = nil, want an error")
		}
	})
}

func TestScanLinksBounds(t *testing.T) {
	t.Run("a project with too many files is refused rather than half checked", func(t *testing.T) {
		old := maxScanFiles
		maxScanFiles = 5
		t.Cleanup(func() { maxScanFiles = old })
		files := map[string]string{}
		for i := range 10 {
			files[fmt.Sprintf("f%d.typ", i)] = "x"
		}
		p := writeProject(t, files)
		if err := scanLinks(p.Root); !errors.Is(err, ErrUnsafeProject) {
			t.Errorf("scanLinks = %v, want ErrUnsafeProject", err)
		}
	})

	t.Run("an unreadable folder does not stop the scan", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can read everything")
		}
		p := writeProject(t, map[string]string{"main.typ": "x", "locked/a.typ": "y"})
		locked := filepath.Join(p.Root, "locked")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) // #nosec G302 -- restoring the test folder so it can be removed
		if err := scanLinks(p.Root); err != nil {
			t.Errorf("scanLinks = %v, want nil", err)
		}
	})

	t.Run("a missing root is an error", func(t *testing.T) {
		if err := scanLinks(filepath.Join(t.TempDir(), "nope")); err == nil {
			t.Error("scanLinks(missing) = nil, want an error")
		}
	})
}

func TestWatchFailure(t *testing.T) {
	if err := failureOr(nil); err != nil {
		t.Errorf("failureOr(nil) = %v", err)
	}
	if err := failureOr(os.ErrClosed); !errors.Is(err, os.ErrClosed) {
		t.Errorf("failureOr(err) = %v, want it wrapped", err)
	}
}
