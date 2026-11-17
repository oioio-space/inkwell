package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Compare the final forced-F launcher with an unrestricted execution of the
// exact same verified binary, input, environment and fixed creation timestamp.
func TestPerimeterF(t *testing.T) {
	t.Run("lsp", testLSPPerimeter)
	for _, operation := range []string{"compile-pdf", "compile-svg", "query", "eval"} {
		t.Run(operation, func(t *testing.T) {
			if !requireLaunchFor(t, "S-5 perimeter") {
				return
			}
			requireEmbedded(t)
			binary, err := (&Store{Dir: sharedCache}).Resolve(Typst)
			if err != nil {
				t.Fatalf("S-5 perimeter: resolve: %v", err)
			}
			dir := t.TempDir()
			input := filepath.Join(dir, "main.typ")
			if err := os.WriteFile(input, []byte("= Sandbox\nHello *world*. <greeting>\n"), 0o600); err != nil {
				t.Fatalf("S-5 perimeter: input: %v", err)
			}
			output := t.TempDir()
			file := "out.pdf"
			args := []string{"query", input, "<greeting>"}
			switch operation {
			case "compile-pdf":
				args = []string{"compile", "--creation-timestamp", "0", input, filepath.Join(output, file)}
			case "compile-svg":
				file = "out.svg"
				args = []string{"compile", "--creation-timestamp", "0", input, filepath.Join(output, file)}
			case "eval":
				args = []string{"eval", "--in", input, "query(<greeting>).len()"}
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Dir = dir
			cmd.Env = []string{}
			baseline, err := cmd.Output()
			if err != nil {
				t.Fatalf("S-5 perimeter: unrestricted %s: %v", operation, err)
			}
			compile := operation == "compile-pdf" || operation == "compile-svg"
			if compile {
				baseline, err = os.ReadFile(filepath.Join(output, file))
				if err != nil {
					t.Fatalf("S-5 perimeter: baseline output: %v", err)
				}
				if err := os.Remove(filepath.Join(output, file)); err != nil {
					t.Fatalf("S-5 perimeter: remove baseline: %v", err)
				}
			}
			s := Spec{Path: binary, Dir: dir, Args: args, tier: tierF}
			if compile {
				s.WatchDir = output
				s.WriteFiles = []string{file}
				s.MaxDirBytes = 16 << 20
			}
			out, err := Run(ctx, s)
			if err != nil || out.ExitCode != 0 {
				t.Fatalf("S-5 perimeter: %s: %+v %v", operation, out, err)
			}
			got := out.Stdout
			if compile {
				got, err = os.ReadFile(filepath.Join(output, file))
				if err != nil {
					t.Fatalf("S-5 perimeter: sandbox output: %v", err)
				}
			}
			if operation == "query" {
				var a, b any
				if json.Unmarshal(baseline, &a) != nil || json.Unmarshal(got, &b) != nil {
					t.Fatalf("S-5 perimeter: invalid JSON: baseline=%q F=%q", baseline, got)
				}
				baseline, _ = json.Marshal(a)
				got, _ = json.Marshal(b)
			}
			if !bytes.Equal(got, baseline) {
				t.Fatalf("S-5 perimeter: %s differs: unrestricted %q F %q", operation, baseline, got)
			}
		})
	}
}
