package project

import (
	"errors"
	"testing"
)

// TestClaimNeedsTheFileAsItWasRead: Claim records that the file belongs to the
// user, but only if it is still what was read when the hash was taken; a file
// that changed since, or vanished, is refused and nothing is recorded.
func TestClaimNeedsTheFileAsItWasRead(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := base.WriteFile("p/a.bin", []byte("read this"), 0o600); err != nil {
		t.Fatal(err)
	}
	read := hashOf([]byte("read this"))

	if err := p.Claim("a.bin", hashOf([]byte("something else"))); err == nil {
		t.Error("Claim(wrong hash of an existing file) = nil, want an error")
	}
	if err := p.Claim("missing.bin", read); err == nil {
		t.Error("Claim(missing file) = nil, want an error")
	}
	if err := p.Claim("missing.bin", ""); err == nil {
		t.Error("Claim(missing file, empty hash) = nil, want an error")
	}
	if p.claimed["a.bin"] || p.claimed["missing.bin"] {
		t.Error("a refused Claim left a claim")
	}
	if err := p.Claim("../escape", read); !errors.Is(err, ErrNotOwned) {
		t.Errorf("Claim(bad path) = %v, want ErrNotOwned", err)
	}

	if err := p.Claim("a.bin", read); err != nil {
		t.Fatalf("Claim(right hash) = %v", err)
	}
	if !p.claimed["a.bin"] {
		t.Error("Claim did not record the claim")
	}
	// Claimed and recorded: it is now one of the project's files, and a write
	// of the projection over it needs the person's acknowledgement.
	if err := base.WriteFile("p/a.bin", []byte("edited outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProjection("a.bin", []byte("ours")); !errors.Is(err, ErrExternalEdit) {
		t.Errorf("WriteProjection over an outside edit of a claimed file = %v, want ErrExternalEdit", err)
	}
}
