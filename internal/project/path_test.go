package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePath(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	put(t, p, "docs/a.pdf", "%PDF")
	got, err := p.FilePath("docs/a.pdf")
	if err != nil || got != filepath.Join(base.Name(), "p", "docs", "a.pdf") {
		t.Fatalf("Path = %q, %v", got, err)
	}
	for _, rel := range []string{"../x", "docs", "missing.pdf", ".inkwell/project.json"} {
		if _, err := p.FilePath(rel); err == nil {
			t.Errorf("FilePath(%q) was accepted", rel)
		}
	}
	if err := os.Symlink("a.pdf", filepath.Join(base.Name(), "p", "docs", "l.pdf")); err == nil {
		if _, err := p.FilePath("docs/l.pdf"); err == nil {
			t.Error("Path followed a symbolic link")
		}
	}
}
