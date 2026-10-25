package project

import (
	"bytes"
	"errors"
	"io/fs"
	"runtime"
	"slices"
	"testing"
)

// Inkwell takes over only the paths it created: a file the user already had
// there is refused, whatever the tree says.
func TestTreeNeverAdoptsExistingFile(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.WriteFile("p/notes.txt", []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.MkdirAll("p/sub", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := base.WriteFile("p/sub/pic.png", []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := p.ApplyTree([]TreeEntry{text("notes.txt"), blobEntry("sub/pic.png", []byte("png")), text("new.typ")})
	if err != nil {
		t.Fatal(err)
	}
	if got := rejectedPaths(res); !slices.Equal(got, []string{"notes.txt", "sub/pic.png"}) {
		t.Errorf("rejected = %q, want the two files that were there", got)
	}
	for _, rej := range res.Rejected {
		if !errors.Is(rej.Err, ErrTreeConflict) {
			t.Errorf("rejection of %q = %v, want ErrTreeConflict", rej.Path, rej.Err)
		}
	}
	if err := p.WriteProjection("notes.txt", []byte("x")); !errors.Is(err, ErrNotOwned) {
		t.Errorf("WriteProjection of a refused file = %v, want ErrNotOwned", err)
	}
	if err := p.WriteProjection("new.typ", []byte("= ok\n")); err != nil {
		t.Errorf("the free path was refused too: %v", err)
	}
	if len(res.Blobs) != 0 {
		t.Errorf("Blobs = %+v: a refused blob is wanted", res.Blobs)
	}
}

// A move needs a regular file that Inkwell wrote and nobody touched since.
func TestTreeMovesOnlyUntouchedFilesInkwellWrote(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	img := []byte("image")
	e := blobEntry("a.png", img)
	if _, err := p.ApplyTree([]TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, bytes.NewReader(img)); err != nil {
		t.Fatal(err)
	}
	// Edited outside Inkwell: it is not moved, and the new path is not written over it.
	if err := base.WriteFile("p/a.png", []byte("edited by the user"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved := e
	moved.Path = "b.png"
	if _, err := p.ApplyTree([]TreeEntry{moved}); err != nil {
		t.Fatal(err)
	}
	if got, err := base.ReadFile("p/a.png"); err != nil || string(got) != "edited by the user" {
		t.Errorf("edited file = %q, %v, want it left in place", got, err)
	}
	if _, err := base.Stat("p/b.png"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an edited file was moved: %v", err)
	}
}

func TestTreeNeverMovesALink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows")
	}
	base := newBase(t)
	p := create(t, base, "p")
	e := blobEntry("a.png", []byte("x"))
	if _, err := p.ApplyTree([]TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}
	// The user swaps the file for a link to another file of the project.
	if err := base.WriteFile("p/real.txt", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Remove("p/a.png"); err != nil {
		t.Fatal(err)
	}
	if err := base.Symlink("real.txt", "p/a.png"); err != nil {
		t.Skip("no symbolic links:", err)
	}
	moved := e
	moved.Path = "b.png"
	_, _ = p.ApplyTree([]TreeEntry{moved})
	if _, err := base.Lstat("p/b.png"); err == nil {
		t.Error("a symbolic link was moved")
	}
}

// Nothing is read or written through a link, even one that stays in the project.
func TestNoLinksInPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows")
	}
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.MkdirAll("p/real", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := base.Symlink("real", "p/link"); err != nil {
		t.Skip("no symbolic links:", err)
	}
	res, err := p.ApplyTree([]TreeEntry{text("link/ch.typ"), text("real/ok.typ")})
	if err == nil && len(res.Rejected) != 1 {
		t.Errorf("ApplyTree through a link = %+v, %v, want the entry refused", res.Rejected, err)
	}
	if _, err := base.Stat("p/real/ch.typ"); err == nil {
		t.Error("a file was created through the link")
	}
	// A file that is itself a link.
	if err := base.WriteFile("p/real/target.txt", []byte("t"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Symlink("real/target.txt", "p/ok.typ"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ApplyTree([]TreeEntry{text("ok.typ")}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("ok.typ", []byte("= x\n")); err == nil {
		t.Error("WriteProjection followed a link")
	}
	if got, _ := base.ReadFile("p/real/target.txt"); string(got) != "t" {
		t.Errorf("the target of the link was written: %q", got)
	}
	if _, err := p.ReadProjection("ok.typ"); err == nil {
		t.Error("ReadProjection followed a link")
	}
	if f, _, err := p.OpenFile("ok.typ"); err == nil {
		_ = f.Close()
		t.Error("OpenFile followed a link")
	}
}

// A file with another name elsewhere (a hard link) is not read as the project's.
func TestOpenFileRefusesHardLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("link counts are not checked on Windows")
	}
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.WriteFile("p/a.png", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if f, _, err := p.OpenFile("a.png"); err != nil {
		t.Fatalf("OpenFile of a plain file: %v", err)
	} else {
		_ = f.Close()
	}
	if err := base.Link("p/a.png", "p/b.png"); err != nil {
		t.Skip("no hard links:", err)
	}
	for _, name := range []string{"a.png", "b.png"} {
		if f, _, err := p.OpenFile(name); !errors.Is(err, ErrHardLink) {
			if err == nil {
				_ = f.Close()
			}
			t.Errorf("OpenFile(%q) = %v, want ErrHardLink", name, err)
		}
	}
}

func TestRenameNoReplace(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.WriteFile("p/src", []byte("src"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.WriteFile("p/dst", []byte("dst"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.renameNoReplace("src", "dst"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("renameNoReplace over a file = %v, want ErrExist", err)
	}
	if got, _ := base.ReadFile("p/dst"); string(got) != "dst" {
		t.Errorf("dst = %q: it was replaced", got)
	}
	if _, err := base.Stat("p/src"); err != nil {
		t.Errorf("src is gone after a refused move: %v", err)
	}
	if err := base.Remove("p/dst"); err != nil {
		t.Fatal(err)
	}
	if err := p.renameNoReplace("src", "dst"); err != nil {
		t.Fatalf("renameNoReplace: %v", err)
	}
	if got, err := base.ReadFile("p/dst"); err != nil || string(got) != "src" {
		t.Errorf("dst = %q, %v", got, err)
	}
	if _, err := base.Stat("p/src"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("src still there: %v", err)
	}
}

// Paths that left the tree and were never written leave the manifest.
func TestTreePrunesPathsNeverWritten(t *testing.T) {
	p := create(t, newBase(t), "p")
	if _, err := p.ApplyTree([]TreeEntry{text("a.typ"), text("b.typ"), blobEntry("c.png", []byte("c"))}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("a.typ", []byte("= A\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ApplyTree([]TreeEntry{text("a2.typ")}); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	got := make([]string, 0, len(p.m.Files))
	for rel := range p.m.Files {
		got = append(got, rel)
	}
	slices.Sort(got)
	// main.typ always; a.typ was written; a2.typ is new. b.typ and c.png never were.
	if want := []string{"a.typ", "a2.typ", "main.typ"}; !slices.Equal(got, want) {
		t.Errorf("owned files = %q, want %q", got, want)
	}
}

func TestWriteBlobChecksTheContent(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	e := blobEntry("x.bin", []byte("right"))
	if _, err := p.ApplyTree([]TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, bytes.NewReader([]byte("wrong"))); !errors.Is(err, ErrBlobMismatch) {
		t.Errorf("WriteBlob of other content = %v, want ErrBlobMismatch", err)
	}
	if _, err := base.Stat("p/x.bin"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("wrong content was left on disk: %v", err)
	}
	entries, _ := fs.ReadDir(base.FS(), "p")
	for _, e := range entries {
		if e.Name() != ".inkwell" {
			t.Errorf("stray file %q after a refused write", e.Name())
		}
	}
}
