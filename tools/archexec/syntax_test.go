package archexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntaxAcrossBuildTags(t *testing.T) {
	for _, imp := range []string{"C", "os/exec"} {
		t.Run(imp, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "hidden")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			source := "//go:build never_enabled\n\npackage hidden\nimport alias \"" + imp + "\"\n"
			if err := os.WriteFile(filepath.Join(dir, "hidden.go"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ScanSources(root); err == nil || !strings.Contains(err.Error(), "forbidden import "+imp) {
				t.Fatalf("expected import rejection, got %v", err)
			}
			allowed := filepath.Join(root, "internal", "sidecar")
			if err := os.MkdirAll(allowed, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(dir, "hidden.go"), filepath.Join(allowed, "hidden.go")); err != nil {
				t.Fatal(err)
			}
			if err := ScanSources(root); err != nil {
				t.Fatal(err)
			}
		})
	}
}
