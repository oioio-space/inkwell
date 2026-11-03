package sidecar

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Exercise the actual pinned compiler under the production allowlist, using a
// PM-1 project member. The supervised Tinymist LSP test covers the server.
func TestAllowlistRealTypstWorkloads(t *testing.T) {
	requireEmbedded(t)
	store := &Store{Dir: sharedCache}
	binary, err := store.Resolve(Typst)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the first versioned member without requiring the large corpus
	// generator as a prerequisite for ordinary unit tests.
	corpus := filepath.Join(t.TempDir(), "00000.typ")
	data := append([]byte("// 00000\n"), bytes.Repeat([]byte("x"), 2098)...)[:2098]
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "21812b9a42119136d7af34920edf2fb311d148c80cc9dbed1eb65eecd759a4d6" {
		t.Fatal("PM-1 corpus member differs from manifest")
	}
	if err := os.WriteFile(corpus, data, 0o600); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	for _, args := range [][]string{
		{"compile", corpus, filepath.Join(output, "out.pdf")},
		{"query", corpus, "heading"},
		{"eval", "--in", corpus, "query(heading)"},
	} {
		t.Run(args[0], func(t *testing.T) {
			spec := Spec{Path: binary, Dir: filepath.Dir(corpus), Args: args}
			if args[0] == "compile" {
				spec.WatchDir, spec.MaxDirBytes, spec.WriteFiles = output, 16<<20, []string{"out.pdf"}
			}
			out, err := Run(t.Context(), spec)
			if err != nil || out.ExitCode != 0 {
				t.Fatalf("%s: %v exit=%d %s", args[0], err, out.ExitCode, out.Stderr)
			}
		})
	}
}
