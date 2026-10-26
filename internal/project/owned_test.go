package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Writing an owned file costs one small record, not a rewrite of the project:
// at the biggest tree a write stays within a budget a keystroke can afford.
func TestManifestRewriteCost(t *testing.T) {
	if testing.Short() {
		t.Skip("applies a tree of 10000 long paths")
	}
	base := newBase(t)
	p := create(t, base, "p")
	seg := strings.Repeat("a", 240)
	var es []TreeEntry
	for i := range MaxNodes {
		es = append(es, text(fmt.Sprintf("%s/%s/%s/%s%05d.typ", seg, seg, seg, strings.Repeat("b", 200), i)))
	}
	start := time.Now()
	if _, err := p.ApplyTree(es); err != nil {
		t.Fatal(err)
	}
	t.Logf("ApplyTree of %d paths: %v", MaxNodes, time.Since(start))
	info, err := base.Stat("p/.inkwell/project.json")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 4<<10 {
		t.Errorf("project.json is %d bytes: it still holds the owned files", info.Size())
	}
	const n = 20
	start = time.Now()
	for i := range n {
		if err := p.WriteProjection(es[i].Path, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if per := time.Since(start) / n; per > 25*time.Millisecond {
		t.Errorf("WriteProjection takes %v at %d owned files, want a small constant", per, MaxNodes)
	} else {
		t.Logf("WriteProjection: %v each", per)
	}
}

// BenchmarkRecord is the cost of recording one write at the biggest tree.
func BenchmarkRecord(b *testing.B) {
	base := newBase(b)
	p, err := Create(base, "p", "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	p.mu.Lock()
	for i := range MaxNodes {
		p.m.Files[fmt.Sprintf("d/f%05d.typ", i)] = "x"
	}
	p.mu.Unlock()
	b.ReportAllocs()
	for b.Loop() {
		p.mu.Lock()
		err := p.record("d/f00001.typ", "abc", nil)
		p.mu.Unlock()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// A project whose project.json still holds the owned files, as the first
// version wrote it, is migrated when it opens.
func TestMigratesOwnedFilesOutOfTheManifest(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", "automerge:abc")
	if err != nil {
		t.Fatal(err)
	}
	img := []byte("img")
	e := blobEntry("img/a.png", img)
	if _, err := p.ApplyTree([]TreeEntry{text("ch1.typ"), e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("ch1.typ", []byte("= One\n")); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, bytes.NewReader(img)); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	legacy := map[string]any{
		"id": p.m.ID, "title": p.m.Title, "rootUrl": p.m.RootURL,
		"files": p.m.Files, "tree": p.m.Tree,
	}
	files := map[string]string{}
	for k, v := range p.m.Files {
		files[k] = v
	}
	p.mu.Unlock()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	// Put the manifest back as the old version wrote it, and lose the records.
	raw, _ := json.Marshal(legacy)
	if err := base.WriteFile("p/.inkwell/project.json", raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.RemoveAll("p/.inkwell/owned"); err != nil {
		t.Fatal(err)
	}

	again, err := Open(base, "p")
	if err != nil {
		t.Fatalf("Open of an old project: %v", err)
	}
	t.Cleanup(func() { _ = again.Close() })
	again.mu.Lock()
	for rel, hash := range files {
		if got, ok := again.m.Files[rel]; !ok || got != hash {
			t.Errorf("owned %q = %q, %v after migration, want %q", rel, got, ok, hash)
		}
	}
	if got := again.m.Tree["img/a.png"]; got == "" {
		t.Error("the tree was not migrated")
	}
	again.mu.Unlock()
	after, _ := base.ReadFile("p/.inkwell/project.json")
	if strings.Contains(string(after), `"files"`) || strings.Contains(string(after), "img/a.png") {
		t.Errorf("project.json still holds the owned files: %s", after)
	}
	// The migrated project works as before, and keeps working after another restart.
	if err := again.WriteProjection("ch1.typ", []byte("= One again\n")); err != nil {
		t.Errorf("WriteProjection after the migration: %v", err)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := Open(base, "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = third.Close() })
	if got, err := third.ReadProjection("ch1.typ"); err != nil || string(got) != "= One again\n" {
		t.Errorf("after the second open: %q, %v", got, err)
	}
	if err := third.WriteBlob(e.Path, e.Hash, bytes.NewReader(img)); err != nil {
		t.Errorf("WriteBlob after the migration: %v", err)
	}
}

// Ownership and the claim survive a restart through the records.
func TestOwnedRecordsSurviveRestart(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := base.WriteFile("p/mine.png", []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.Claim("mine.png", hashOf([]byte("mine"))); err != nil {
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
	again.mu.Lock()
	defer again.mu.Unlock()
	if !again.claimed["mine.png"] || again.m.Files["mine.png"] != hashOf([]byte("mine")) {
		t.Errorf("claimed = %v, hash %q after a restart", again.claimed["mine.png"], again.m.Files["mine.png"])
	}
}

// A writer lists a path before the user has a file there. When the user creates
// the file, the writer's content never replaces it.
func TestPreClaimedPathNeverOverwritesTheUsersFile(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		base := newBase(t)
		p := create(t, base, "p")
		if _, err := p.ApplyTree([]TreeEntry{text("notes.typ")}); err != nil { // the peer's claim, nothing on disk
			t.Fatal(err)
		}
		if err := base.WriteFile("p/notes.typ", []byte("my own notes"), 0o600); err != nil { // the user
			t.Fatal(err)
		}
		if err := p.WriteProjection("notes.typ", []byte("the writer's text")); !errors.Is(err, ErrExternalEdit) {
			t.Errorf("WriteProjection over the user's file = %v, want ErrExternalEdit", err)
		}
		if got, _ := base.ReadFile("p/notes.typ"); string(got) != "my own notes" {
			t.Errorf("notes.typ = %q: the user's file was replaced", got)
		}
	})
	t.Run("blob", func(t *testing.T) {
		base := newBase(t)
		p := create(t, base, "p")
		evil := []byte("the writer's picture")
		e := blobEntry("pic.png", evil)
		if _, err := p.ApplyTree([]TreeEntry{e}); err != nil { // the peer's claim, nothing on disk
			t.Fatal(err)
		}
		mine := []byte("my own picture")
		if err := base.WriteFile("p/pic.png", mine, 0o600); err != nil { // the user creates it...
			t.Fatal(err)
		}
		if err := p.Claim("pic.png", hashOf(mine)); err != nil { // ...and adds it to the project
			t.Fatal(err)
		}
		if err := p.WriteBlob(e.Path, e.Hash, bytes.NewReader(evil)); !errors.Is(err, ErrExternalEdit) {
			t.Errorf("WriteBlob over the user's file = %v, want ErrExternalEdit", err)
		}
		if got, _ := base.ReadFile("p/pic.png"); !bytes.Equal(got, mine) {
			t.Errorf("pic.png = %q: the user's file was replaced", got)
		}
		// The user's file becoming the version of the tree, the path follows the tree again.
		mineEntry := blobEntry("pic.png", mine)
		if _, err := p.ApplyTree([]TreeEntry{mineEntry}); err != nil {
			t.Fatal(err)
		}
		if err := p.WriteBlob(mineEntry.Path, mineEntry.Hash, bytes.NewReader(mine)); err != nil {
			t.Fatal(err)
		}
		newer := blobEntry("pic.png", []byte("a newer version"))
		if _, err := p.ApplyTree([]TreeEntry{newer}); err != nil {
			t.Fatal(err)
		}
		if err := p.WriteBlob(newer.Path, newer.Hash, bytes.NewReader([]byte("a newer version"))); err != nil {
			t.Errorf("WriteBlob of a newer version over the tree's own version: %v", err)
		}
	})
}
