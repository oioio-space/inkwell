package typst

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseDiagnostics(t *testing.T) {
	const root = "/home/u/Doc/Projet"
	const pkgs = "/cache/inkwell/typst/packages"
	tests := []struct {
		name string
		in   string
		want []Diagnostic
	}{
		{
			name: "error with a position, columns become 1-based",
			in:   "main.typ:1:18: error: unknown variable: unknownfn\n",
			want: []Diagnostic{{Severity: Error, File: "main.typ", Line: 1, Column: 19, Message: "unknown variable: unknownfn"}},
		},
		{
			name: "warning and error in a sub folder",
			in:   "sub/x.typ:1:12: error: expected expression\nmain.typ:1:16: warning: unknown font family: nonexistent font\n",
			want: []Diagnostic{
				{Severity: Error, File: "sub/x.typ", Line: 1, Column: 13, Message: "expected expression"},
				{Severity: Warning, File: "main.typ", Line: 1, Column: 17, Message: "unknown font family: nonexistent font"},
			},
		},
		{
			name: "no position",
			in:   "error: input file not found\n",
			want: []Diagnostic{{Severity: Error, Message: "input file not found"}},
		},
		{
			name: "a message on several lines stays one diagnostic",
			in:   "main.typ:1:1: error: panicked with: line1\nline2\n",
			want: []Diagnostic{{Severity: Error, File: "main.typ", Line: 1, Column: 2, Message: "panicked with: line1\nline2"}},
		},
		{
			name: "CRLF",
			in:   "main.typ:3:0: error: the character `#` is not valid in code\r\n",
			want: []Diagnostic{{Severity: Error, File: "main.typ", Line: 3, Column: 1, Message: "the character `#` is not valid in code"}},
		},
		{
			name: "an absolute path inside the project becomes relative",
			in:   root + "/chap/a.typ:2:3: error: boom\n",
			want: []Diagnostic{{Severity: Error, File: "chap/a.typ", Line: 2, Column: 4, Message: "boom"}},
		},
		{
			name: "the project folder is not repeated in messages",
			in:   "main.typ:1:7: error: file not found (searched at " + root + "/nope.png)\n",
			want: []Diagnostic{{Severity: Error, File: "main.typ", Line: 1, Column: 8, Message: "file not found (searched at nope.png)"}},
		},
		{
			name: "a file of a bundled package is named like its import",
			in:   pkgs + "/preview/cmarker/0.1.10/lib.typ:4:5: error: boom\n",
			want: []Diagnostic{{Severity: Error, File: "@preview/cmarker:0.1.10/lib.typ", Line: 4, Column: 6, Message: "boom"}},
		},
		{
			name: "text that is not a diagnostic is ignored before the first one",
			in:   "downloading something\n\nmain.typ:1:1: warning: w\n",
			want: []Diagnostic{{Severity: Warning, File: "main.typ", Line: 1, Column: 2, Message: "w"}},
		},
		{name: "empty", in: "", want: nil},
		{name: "garbage", in: "\x00\xff not a diagnostic", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := parseDiagnostics([]byte(tt.in), root, pkgs, limits{maxDiagnostics: 200, maxMessage: 2000})
			if truncated {
				t.Error("truncated = true")
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("parseDiagnostics(%q) mismatch (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}

func TestParseDiagnosticsMissingPackage(t *testing.T) {
	in := "main.typ:1:8: error: failed to download package (https://packages.typst.org/preview/ghost-1.2.3.tar.gz: Connection Failed: Connect error)\n" +
		"main.typ:2:8: error: package not found (searched for /cache/local/mine/0.1.0)\n"
	got, _ := parseDiagnostics([]byte(in), "/p", "/c", limits{maxDiagnostics: 10, maxMessage: 2000})
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %+v", len(got), got)
	}
	if got[0].Package != "@preview/ghost:1.2.3" || got[1].Package != "@local/mine:0.1.0" {
		t.Errorf("packages = %q, %q; want @preview/ghost:1.2.3 and @local/mine:0.1.0", got[0].Package, got[1].Package)
	}
	for _, d := range got {
		if !strings.Contains(d.Message, "never downloads") || !strings.Contains(d.Message, d.Package) {
			t.Errorf("message %q must name %s and say that Inkwell never downloads", d.Message, d.Package)
		}
	}
}

func TestParseDiagnosticsBounds(t *testing.T) {
	var sb strings.Builder
	for range 5000 {
		sb.WriteString("main.typ:1:1: warning: w\n")
	}
	got, truncated := parseDiagnostics([]byte(sb.String()), "/p", "/c", limits{maxDiagnostics: 200, maxMessage: 2000})
	if len(got) != 200 || !truncated {
		t.Errorf("kept %d diagnostics, truncated=%v; want 200 and true", len(got), truncated)
	}

	long := "main.typ:1:1: error: " + strings.Repeat("x", 100000) + "\n" + strings.Repeat("more\n", 1000)
	got, _ = parseDiagnostics([]byte(long), "/p", "/c", limits{maxDiagnostics: 200, maxMessage: 100})
	if len(got) != 1 || len(got[0].Message) > 100+len("…") {
		t.Errorf("message length = %d, want at most 100 plus an ellipsis", len(got[0].Message))
	}
}

func FuzzParseDiagnostics(f *testing.F) {
	f.Add("main.typ:1:18: error: unknown variable\n")
	f.Add("error: x\nwarning: y\n")
	f.Add(":::: error: \n\n\r\n")
	f.Add("a:99999999999999999999:1: error: overflow\n")
	f.Fuzz(func(t *testing.T, in string) {
		got, _ := parseDiagnostics([]byte(in), "/p", "/c", limits{maxDiagnostics: 20, maxMessage: 64})
		if len(got) > 20 {
			t.Fatalf("%d diagnostics exceed the bound", len(got))
		}
		for _, d := range got {
			if d.Severity != Error && d.Severity != Warning {
				t.Fatalf("severity %q", d.Severity)
			}
			if d.Line < 0 || d.Column < 0 || len(d.Message) > 64+len("…") {
				t.Fatalf("out of bounds: %+v", d)
			}
		}
	})
}
