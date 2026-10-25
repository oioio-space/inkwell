package project

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func text(path string) TreeEntry {
	return TreeEntry{Path: path, Type: NodeText, URL: "automerge:" + strings.ReplaceAll(path, "/", "")}
}

func dir(path string) TreeEntry { return TreeEntry{Path: path, Type: NodeDir} }

func blobEntry(path string, data []byte) TreeEntry {
	return TreeEntry{Path: path, Type: NodeBlob, Hash: sha256.Sum256(data), Size: int64(len(data))}
}

func rejectedPaths(r TreeResult) []string {
	var out []string
	for _, rej := range r.Rejected {
		out = append(out, rej.Path)
	}
	return out
}

// PRJ-10: hostile paths from a peer are refused, and nothing reaches the disk
// for them, while the honest entries of the same tree are applied.
func TestApplyTreeRejectsHostilePaths(t *testing.T) {
	t.Run("PRJ-10 hostile paths from a peer", runApplyTreeRejectsHostilePaths)
}

func runApplyTreeRejectsHostilePaths(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	long := strings.Repeat("a", 300)
	deep := strings.Repeat("d/", MaxDepth) + "f.typ" // MaxDepth folders and a file: one level too many
	hostile := []string{
		"../escape.typ", "a/../../escape.typ", "/abs.typ", "C:/windows.typ", "back\\slash.typ", "a\\b.typ",
		"NUL.typ", "dir/com1.txt", "dir/COM\u00b2", "nul\x00byte.typ", long + ".typ", "tab\there.typ",
		".inkwell/project.json", ".typstdeps/x", "trailing.", "trailing ", "", "a//b.typ", "dir/", "./x.typ",
		deep,
	}
	entries := []TreeEntry{text("good.typ"), dir("sub"), text("sub/also good.typ")}
	for _, h := range hostile {
		entries = append(entries, text(h))
	}
	res, err := p.ApplyTree(entries)
	if err != nil {
		t.Fatalf("ApplyTree: %v", err)
	}
	got := rejectedPaths(res)
	slices.Sort(got)
	want := slices.Clone(hostile)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("rejected = %q\nwant       %q", got, want)
	}
	for _, rej := range res.Rejected {
		if !errors.Is(rej.Err, ErrInvalidName) && !errors.Is(rej.Err, ErrTreeConflict) {
			t.Errorf("rejection of %q = %v, want an invalid name or a conflict", rej.Path, rej.Err)
		}
		if rej.Err != nil && strings.Contains(rej.Err.Error(), "\x00") {
			t.Errorf("the error repeats a hostile name: %q", rej.Err)
		}
	}
	// The honest entries are there; nothing was owned or created for the others.
	for _, ok := range []string{"good.typ", "sub/also good.typ"} {
		if err := p.WriteProjection(ok, []byte("= ok\n")); err != nil {
			t.Errorf("WriteProjection(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"../escape.typ", "NUL.typ", "dir/com1.txt", ".inkwell/project.json"} {
		if err := p.WriteProjection(bad, []byte("x")); !errors.Is(err, ErrNotOwned) {
			t.Errorf("WriteProjection(%q) = %v, want ErrNotOwned", bad, err)
		}
	}
	entriesOnDisk, _ := fs.ReadDir(base.FS(), ".")
	if len(entriesOnDisk) != 1 {
		t.Errorf("the base folder holds %d entries, want only the project: something escaped", len(entriesOnDisk))
	}
	if _, err := base.Stat("p/dir"); err == nil {
		t.Error("a folder was created for a refused entry")
	}
}

// A symbolic link that leaves the project cannot be used to write outside it.
func TestApplyTreeCannotFollowSymlinkOut(t *testing.T) {
	t.Run("PRJ-10 no way out through a link", runApplyTreeCannotFollowSymlinkOut)
}

func runApplyTreeCannotFollowSymlinkOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symbolic links needs a privilege on Windows")
	}
	outside := t.TempDir()
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.Symlink(outside, "p/link"); err != nil {
		t.Skip("no symbolic links:", err)
	}
	if _, err := p.ApplyTree([]TreeEntry{text("link/evil.typ")}); err == nil {
		t.Error("ApplyTree through a link out of the project succeeded")
	}
	if err := p.WriteProjection("link/evil.typ", []byte("x")); err == nil {
		t.Error("WriteProjection through a link out of the project succeeded")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("%d files written outside the project", len(entries))
	}
}

// PRJ-17: names that differ only by case or by normalization collide; the
// first path in byte order keeps its name, and a file never stands on a folder.
func TestApplyTreeCollisions(t *testing.T) {
	tests := []struct {
		name    string
		entries []TreeEntry
		want    []string // rejected paths
	}{
		{"case", []TreeEntry{text("Notes.md"), text("notes.md")}, []string{"notes.md"}},
		{"case in a folder", []TreeEntry{text("a/Notes.md"), text("a/NOTES.MD"), text("b/notes.md")}, []string{"a/Notes.md"}},
		{"NFC and NFD", []TreeEntry{text("caf\u00e9.md"), text("cafe\u0301.md")}, []string{"caf\u00e9.md"}},
		{"sharp s", []TreeEntry{text("STRASSE.txt"), text("Stra\u00dfe.txt")}, []string{"Stra\u00dfe.txt"}},
		{"folders that differ by case", []TreeEntry{dir("Docs"), text("docs/x.md")}, []string{"docs/x.md"}},
		{"a file where a folder is needed", []TreeEntry{text("a"), text("a/b.typ")}, []string{"a/b.typ"}},
		{"a folder where a file is", []TreeEntry{dir("a"), text("a")}, []string{"a"}},
		{"same folder twice", []TreeEntry{dir("a"), dir("a"), text("a/b.typ")}, nil},
		{"two files at one path", []TreeEntry{text("a.typ"), text("a.typ")}, []string{"a.typ"}},
		{"different names", []TreeEntry{text("a.md"), text("b.md"), text("a/b.md")}, nil},
	}
	for _, tc := range tests {
		t.Run("PRJ-17 "+tc.name, func(t *testing.T) {
			p := create(t, newBase(t), "p")
			res, err := p.ApplyTree(tc.entries)
			if err != nil {
				t.Fatal(err)
			}
			if got := rejectedPaths(res); !slices.Equal(got, tc.want) {
				t.Errorf("rejected = %q, want %q", got, tc.want)
			}
			for _, rej := range res.Rejected {
				if !errors.Is(rej.Err, ErrTreeConflict) {
					t.Errorf("rejection of %q = %v, want ErrTreeConflict", rej.Path, rej.Err)
				}
			}
		})
	}
}

func TestApplyTreeLimits(t *testing.T) {
	p := create(t, newBase(t), "p")
	many := make([]TreeEntry, MaxNodes+1)
	for i := range many {
		many[i] = dir("d")
	}
	if _, err := p.ApplyTree(many); err == nil {
		t.Errorf("a tree of %d entries was applied, want it refused", len(many))
	}
	// The limits themselves are fine: MaxDepth levels, 255-byte names.
	okDeep := strings.Repeat("d/", MaxDepth-1) + "f.typ"
	okLong := strings.Repeat("a", 251) + ".typ"
	res, err := p.ApplyTree([]TreeEntry{text(okDeep), text(okLong)})
	if err != nil || len(res.Rejected) != 0 {
		t.Errorf("ApplyTree at the limits = %+v, %v; want everything accepted", res.Rejected, err)
	}
	// Entries of a type that does not exist, or without a document, are refused.
	res, err = p.ApplyTree([]TreeEntry{{Path: "x", Type: 9}, {Path: "y.typ", Type: NodeText}, {Path: "z.png", Type: NodeBlob, Size: -1}})
	if err != nil || len(res.Rejected) != 3 {
		t.Errorf("ApplyTree of malformed entries = %+v, %v; want 3 rejections", res.Rejected, err)
	}
}

// PRJ-01 (Go side): every text file of the tree is projected, in folders, not
// only main.typ; a joiner starts from nothing.
func TestApplyTreeProjectsEveryTextFile(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	tree := []TreeEntry{
		{Path: MainFile, Type: NodeText, URL: "automerge:main", Main: true},
		dir("chapters"), text("chapters/ch1.typ"), text("chapters/ch2.md"), text("notes/deep/todo.txt"),
		dir("empty"), blobEntry("img/logo.png", []byte("png")),
	}
	res, err := p.ApplyTree(tree)
	if err != nil || len(res.Rejected) != 0 {
		t.Fatalf("ApplyTree = %+v, %v", res.Rejected, err)
	}
	for _, d := range []string{"chapters", "empty", "notes/deep", "img"} {
		if info, err := base.Stat("p/" + d); err != nil || !info.IsDir() {
			t.Errorf("folder %s: %v", d, err)
		}
	}
	for _, f := range []string{"chapters/ch1.typ", "chapters/ch2.md", "notes/deep/todo.txt"} {
		if err := p.WriteProjection(f, []byte("= "+f+"\n")); err != nil {
			t.Errorf("WriteProjection(%q): %v", f, err)
			continue
		}
		got, err := p.ReadProjection(f)
		if err != nil || string(got) != "= "+f+"\n" {
			t.Errorf("ReadProjection(%q) = %q, %v", f, got, err)
		}
	}
	if len(res.Blobs) != 1 || res.Blobs[0].Path != "img/logo.png" {
		t.Errorf("Blobs = %+v, want img/logo.png", res.Blobs)
	}
	files, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Path)
	}
	for _, want := range []string{"chapters/ch1.typ", "chapters/ch2.md", "notes/deep/todo.txt"} {
		if !slices.Contains(names, want) {
			t.Errorf("List() = %q, missing %s", names, want)
		}
	}
	// Applying the same tree again changes nothing.
	if res2, err := p.ApplyTree(tree); err != nil || len(res2.Rejected) != 0 {
		t.Errorf("second ApplyTree = %+v, %v", res2.Rejected, err)
	}
}

// The visible copy of a blob is a plain file, never a link to the store.
func TestWriteBlob(t *testing.T) { t.Run("PRJ-13 visible copy of a blob", runWriteBlob) }

func runWriteBlob(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	data := bytes.Repeat([]byte("PNG"), 1000)
	e := blobEntry("img/a.png", data)
	if _, err := p.ApplyTree([]TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, data); err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	got, err := base.ReadFile("p/img/a.png")
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("visible copy = %d bytes, %v", len(got), err)
	}
	info, err := base.Stat("p/img/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("visible copy mode = %v, want 0600: the user may edit it", info.Mode().Perm())
	}
	// A copy that is a link to another file would show here: it has one name.
	if nlink := linkCount(t, base, "p/img/a.png"); nlink != 1 {
		t.Errorf("visible copy has %d links, want 1 (never a hard link)", nlink)
	}
	// Written again with the same content: nothing to do.
	if err := p.WriteBlob(e.Path, e.Hash, data); err != nil {
		t.Errorf("second WriteBlob: %v", err)
	}
	// Written for a path or a hash the tree does not list.
	if err := p.WriteBlob("img/b.png", e.Hash, data); !errors.Is(err, ErrNotOwned) {
		t.Errorf("WriteBlob at an unlisted path = %v, want ErrNotOwned", err)
	}
	other := sha256.Sum256([]byte("other"))
	if err := p.WriteBlob(e.Path, other, data); !errors.Is(err, ErrBlobMoved) {
		t.Errorf("WriteBlob of another hash = %v, want ErrBlobMoved", err)
	}
	if err := p.WriteBlob("../x.png", e.Hash, data); !errors.Is(err, ErrNotOwned) {
		t.Errorf("WriteBlob outside the project = %v, want ErrNotOwned", err)
	}
	// PRJ-13: the image is replaced on disk; Inkwell does not overwrite it.
	replaced := bytes.Repeat([]byte("JPG"), 900)
	if err := base.WriteFile("p/img/a.png", replaced, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, data); !errors.Is(err, ErrExternalEdit) {
		t.Errorf("WriteBlob over an external edit = %v, want ErrExternalEdit", err)
	}
	if got, _ := base.ReadFile("p/img/a.png"); !bytes.Equal(got, replaced) {
		t.Error("the replaced image was overwritten")
	}
	// ...and the new content is a new blob: when the tree lists it, the file is already right.
	e2 := blobEntry("img/a.png", replaced)
	if e2.Hash == e.Hash {
		t.Fatal("a replaced image has the same hash")
	}
	if _, err := p.ApplyTree([]TreeEntry{e2}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e2.Path, e2.Hash, replaced); err != nil {
		t.Errorf("WriteBlob of the replacement = %v, want it accepted: the file is already that", err)
	}
	if paths := p.BlobPaths(e2.Hash); !slices.Equal(paths, []string{"img/a.png"}) {
		t.Errorf("BlobPaths = %q", paths)
	}
}

// A blob of 40 MB is written and hashed without being limited to the size of a text.
func TestWriteBlobLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 40 MB")
	}
	base := newBase(t)
	p := create(t, base, "p")
	data := bytes.Repeat([]byte{7}, 40<<20)
	e := blobEntry("big.bin", data)
	if _, err := p.ApplyTree([]TreeEntry{e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, data); err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, data); err != nil {
		t.Errorf("second WriteBlob: %v", err)
	}
}

// PRJ-05 (Go side): a file renamed in the tree is moved on disk, with its
// ownership, not written anew; a file that left the tree is not deleted.
func TestApplyTreeMovesRenamedFiles(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	img := []byte("image")
	e := blobEntry("img/a.png", img)
	if _, err := p.ApplyTree([]TreeEntry{text("ch1.typ"), text("keep.typ"), e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("ch1.typ", []byte("= One\n")); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, img); err != nil {
		t.Fatal(err)
	}

	renamed := TreeEntry{Path: "chapters/one.typ", Type: NodeText, URL: text("ch1.typ").URL}
	moved := e
	moved.Path = "pictures/logo.png"
	if _, err := p.ApplyTree([]TreeEntry{renamed, text("keep.typ"), moved}); err != nil {
		t.Fatal(err)
	}
	if got, err := base.ReadFile("p/chapters/one.typ"); err != nil || string(got) != "= One\n" {
		t.Errorf("moved text = %q, %v", got, err)
	}
	if got, err := base.ReadFile("p/pictures/logo.png"); err != nil || !bytes.Equal(got, img) {
		t.Errorf("moved image = %q, %v", got, err)
	}
	for _, gone := range []string{"p/ch1.typ", "p/img/a.png"} {
		if _, err := base.Stat(gone); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is still there after the move: %v", gone, err)
		}
	}
	// The moved file is still owned: the editor goes on writing to it (PRJ-05).
	if err := p.WriteProjection("chapters/one.typ", []byte("= One, edited\n")); err != nil {
		t.Errorf("WriteProjection after the move: %v", err)
	}
	if err := p.WriteProjection("ch1.typ", []byte("x")); err != nil {
		t.Logf("old path: %v", err) // still owned in the manifest; harmless
	}

	// A file that leaves the tree stays on disk.
	if _, err := p.ApplyTree([]TreeEntry{text("keep.typ")}); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Stat("p/chapters/one.typ"); err != nil {
		t.Errorf("a file that left the tree was deleted: %v", err)
	}
	// Nothing is moved over a file that exists.
	if err := base.WriteFile("p/taken.typ", []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ApplyTree([]TreeEntry{{Path: "taken.typ", Type: NodeText, URL: text("keep.typ").URL}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := base.ReadFile("p/taken.typ"); string(got) != "mine" {
		t.Errorf("taken.typ = %q: a file was overwritten by a move", got)
	}
}

// The tree survives a restart: a rename made while closed is still a move.
func TestTreeIsPersisted(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", "automerge:abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ApplyTree([]TreeEntry{text("a.typ")}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("a.typ", []byte("= A\n")); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(base, "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	if _, err := again.ApplyTree([]TreeEntry{{Path: "b.typ", Type: NodeText, URL: text("a.typ").URL}}); err != nil {
		t.Fatal(err)
	}
	if got, err := base.ReadFile("p/b.typ"); err != nil || string(got) != "= A\n" {
		t.Errorf("b.typ = %q, %v; want the moved file", got, err)
	}
}

// PRJ-13: a text file in a subfolder, listed after the watcher started, is
// watched too.
func TestWatchFollowsTheTree(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	got := startWatch(t, p)
	if _, err := p.ApplyTree([]TreeEntry{text("sub/ch1.typ")}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("sub/ch1.typ", []byte("= One\n")); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, got, 2*watchDebounce) // Inkwell's own write is not news
	if err := base.WriteFile("p/sub/ch1.typ", []byte("= One, edited elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, got, "sub/ch1.typ")
}

func TestOpenFile(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.WriteFile("p/pic.png", []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Mkdir("p/folder", 0o700); err != nil {
		t.Fatal(err)
	}
	f, info, err := p.OpenFile("pic.png")
	if err != nil || info.Size() != 3 {
		t.Fatalf("OpenFile = %v, %v", info, err)
	}
	_ = f.Close()
	for _, bad := range []string{"../pic.png", ".inkwell/project.json", "missing.png", "folder", "NUL"} {
		if f, _, err := p.OpenFile(bad); err == nil {
			_ = f.Close()
			t.Errorf("OpenFile(%q) succeeded, want an error", bad)
		}
	}
}
