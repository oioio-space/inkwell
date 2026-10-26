package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// A squatter pre-lists P with its own hash Ha; the user creates P and
// claims it; the squatter then lists Ha at Q: the user's claimed file is moved
// to Q because moveRenamed trusts the tree's ref, not the disk content.
func TestMoveRenamedSparesClaimedFile(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	sq := blobEntry("figs/plot.png", []byte("attacker"))
	if _, err := p.ApplyTree([]TreeEntry{sq}); err != nil {
		t.Fatal(err)
	}
	mine := []byte("user's real plot")
	if err := base.WriteFile("p/figs/plot.png", mine, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(mine)
	if err := p.Claim("figs/plot.png", hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := p.WriteBlob(sq.Path, sq.Hash, bytes.NewReader([]byte("attacker"))); err == nil {
		t.Fatal("claimed file overwritten")
	}
	mv := sq
	mv.Path = "elsewhere/x.png"
	if _, err := p.ApplyTree([]TreeEntry{mv}); err != nil {
		t.Fatal(err)
	}
	if got, err := base.ReadFile("p/elsewhere/x.png"); err == nil && bytes.Equal(got, mine) {
		t.Error("the user's claimed file was moved by a tree whose ref never matched its content")
	}
	if got, _ := base.ReadFile("p/elsewhere/x.png"); bytes.Equal(got, []byte("attacker")) {
		t.Error("claimed file overwritten after move")
	}
}
