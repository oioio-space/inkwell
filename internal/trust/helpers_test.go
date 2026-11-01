package trust

import (
	"crypto/ed25519"
	"testing"
)

type testSigner struct{ priv ed25519.PrivateKey }

func newSigner(seed byte) *testSigner {
	s := make([]byte, ed25519.SeedSize)
	s[0] = seed
	return &testSigner{ed25519.NewKeyFromSeed(s)}
}

func (s *testSigner) Public() ed25519.PublicKey { return s.priv.Public().(ed25519.PublicKey) }

func (s *testSigner) Sign(domain string, msg []byte) []byte {
	return ed25519.Sign(s.priv, append([]byte("inkwell/"+domain+"/v1\x00"), msg...))
}

var _ Signer = (*testSigner)(nil)

// project starts a project created by creator and returns its first replica.
func project(t testing.TB, creator Signer) *List {
	t.Helper()
	g, id := Genesis(creator, "book", "automerge:abc")
	l := New(id)
	if err := l.Apply(g); err != nil {
		t.Fatalf("Apply(genesis) = %v", err)
	}
	return l
}

// fork returns an independent replica holding the same ops as l.
func fork(t testing.TB, l *List) *List {
	t.Helper()
	f := New(l.Project())
	mustApplyAll(t, f, l.Since(nil))
	return f
}

func mustApplyAll(t testing.TB, l *List, ops []Op) {
	t.Helper()
	if _, err := l.ApplyAll(ops); err != nil {
		t.Fatalf("ApplyAll() = %v", err)
	}
}

// merge makes a and b hold the same ops.
func merge(t testing.TB, a, b *List) {
	t.Helper()
	mustApplyAll(t, a, b.Since(nil))
	mustApplyAll(t, b, a.Since(nil))
}

func mustIssue(t testing.TB, l *List, by Signer, k OpKind, subject Signer, r Role) Op {
	t.Helper()
	op, err := l.Issue(by, k, subject.Public(), r, "")
	if err != nil {
		t.Fatalf("Issue(kind %d) = %v", k, err)
	}
	return op
}

func wantRole(t testing.TB, l *List, who Signer, want Role) {
	t.Helper()
	if got := l.Role(who.Public()); got != want {
		t.Errorf("Role(%x) = %v, want %v", who.Public()[:4], got, want)
	}
}
