package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAddFile(t *testing.T) {
	t.Run("FM-02 writes a new file with its folders, and leaves no temporary file", func(t *testing.T) {
		base := newBase(t)
		p := create(t, base, "p")
		data := []byte{0x89, 'P', 'N', 'G', 0, 1, 2}
		if err := p.AddFile("figs/deep/a.png", data); err != nil {
			t.Fatalf("AddFile: %v", err)
		}
		got, err := base.ReadFile("p/figs/deep/a.png")
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("file = %q, %v; want %q", got, err, data)
		}
		entries, _ := os.ReadDir(filepath.Join(base.Name(), "p", "figs", "deep"))
		if len(entries) != 1 {
			t.Errorf("folder holds %d entries, want only the file", len(entries))
		}
	})

	t.Run("FM-02 never replaces a file or a folder that is there, nor follows a link", func(t *testing.T) {
		base := newBase(t)
		p := create(t, base, "p")
		put(t, p, "mine.png", "the user's own")
		put(t, p, "dir/x.txt", "x")
		if err := os.Symlink("mine.png", filepath.Join(base.Name(), "p", "link.png")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		for _, rel := range []string{"mine.png", "dir"} {
			if err := p.AddFile(rel, []byte("dropped")); !errors.Is(err, ErrExists) {
				t.Errorf("AddFile(%q) = %v, want ErrExists", rel, err)
			}
		}
		if err := p.AddFile("link.png", []byte("dropped")); !errors.Is(err, ErrLink) {
			t.Errorf("AddFile over a link = %v, want ErrLink", err)
		}
		if got, _ := base.ReadFile("p/mine.png"); string(got) != "the user's own" {
			t.Errorf("the user's file was replaced: %q", got)
		}
	})

	t.Run("PRJ-10 refuses a hostile path and a path through a link", func(t *testing.T) {
		base := newBase(t)
		p := create(t, base, "p")
		for _, rel := range []string{"../x", "/etc/x", "a\\b", "CON.txt", ".inkwell/x", "a/../b"} {
			if err := p.AddFile(rel, []byte("x")); !errors.Is(err, ErrInvalidName) {
				t.Errorf("AddFile(%q) = %v, want ErrInvalidName", rel, err)
			}
		}
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(base.Name(), "p", "out")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if err := p.AddFile("out/x.png", []byte("x")); !errors.Is(err, ErrLink) {
			t.Errorf("AddFile through a link = %v, want ErrLink", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "x.png")); err == nil {
			t.Error("a file was written outside the project")
		}
	})
}
