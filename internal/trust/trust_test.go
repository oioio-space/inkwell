package trust

import (
	"crypto/ed25519"
	"errors"
	"testing"
)

// testSigner is a [Signer] backed by crypto/ed25519 with the domain
// separation prefix the real device uses.
type testSigner struct{ priv ed25519.PrivateKey }

func newSigner(seed byte) *testSigner {
	s := make([]byte, ed25519.SeedSize)
	s[0] = seed
	return &testSigner{ed25519.NewKeyFromSeed(s)}
}

func (s *testSigner) Public() ed25519.PublicKey { return s.priv.Public().(ed25519.PublicKey) }

func (s *testSigner) Sign(domain string, msg []byte) []byte {
	return ed25519.Sign(s.priv, signingMessage(domain, msg))
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

func TestConcurrentRevocation(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)
	la := project(t, a)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin)
	lb := fork(t, la)

	mustIssue(t, la, a, OpRemove, b, 0)      // a revokes b ...
	mustIssue(t, lb, b, OpAdd, c, RoleAdmin) // ... while b invites c.

	merge(t, la, lb)
	for name, l := range map[string]*List{"a": la, "b": lb} {
		t.Run(name, func(t *testing.T) {
			wantRole(t, l, a, RoleAdmin)
			wantRole(t, l, b, 0)
			wantRole(t, l, c, 0)
		})
	}
}

func TestTwoAdminsRemoveEachOther(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	for _, tc := range []struct {
		name  string
		extra int // ops b writes first, so its Remove sorts after a's
	}{{"bLowerLamport", 0}, {"bHigherLamport", 3}} {
		t.Run(tc.name, func(t *testing.T) {
			la := project(t, a)
			mustIssue(t, la, a, OpAdd, b, RoleAdmin)
			lb := fork(t, la)
			for range tc.extra {
				mustIssue(t, lb, b, OpSetRole, b, RoleAdmin)
			}
			mustIssue(t, la, a, OpRemove, b, 0)
			forge(t, lb, b, OpRemove, a, 0) // Issue refuses: a is senior.

			merge(t, la, lb)
			for _, l := range []*List{la, lb} {
				wantRole(t, l, a, RoleAdmin) // a is older.
				wantRole(t, l, b, 0)
			}
		})
	}
}

func TestAntedating(t *testing.T) {
	a, b, m := newSigner(1), newSigner(2), newSigner(3)
	la := project(t, a)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin)
	stale := fork(t, la) // b's view before the revocation.

	removal := mustIssue(t, la, a, OpRemove, b, 0)

	// b antedates an Add from its stale view: concurrent to the Remove.
	mustIssue(t, stale, b, OpAdd, m, RoleWriter)
	merge(t, la, stale)
	wantRole(t, la, m, 0)
	wantRole(t, stale, m, 0)

	// Once b has seen the removal it cannot issue at all.
	if err := stale.Apply(removal); err != nil {
		t.Fatalf("Apply(removal) = %v", err)
	}
	if _, err := stale.Issue(b, OpAdd, m.Public(), RoleWriter, ""); !errors.Is(err, ErrNotAdmin) {
		t.Errorf("Issue after removal = %v, want ErrNotAdmin", err)
	}
}

func TestReaddAfterRemove(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleWriter)
	mustIssue(t, l, a, OpRemove, b, 0)
	wantRole(t, l, b, 0)

	mustIssue(t, l, a, OpAdd, b, RoleReader) // sees the Remove: effective.
	wantRole(t, l, b, RoleReader)
}

func TestReaddConcurrentWithRemoveIsVoid(t *testing.T) {
	a, c, b := newSigner(1), newSigner(2), newSigner(3)
	la := project(t, a)
	mustIssue(t, la, a, OpAdd, c, RoleAdmin)
	mustIssue(t, la, a, OpAdd, b, RoleWriter)
	lc := fork(t, la)

	mustIssue(t, la, a, OpRemove, b, 0)
	mustIssue(t, lc, c, OpSetRole, b, RoleAdmin) // concurrent: voided by the Remove.

	merge(t, la, lc)
	wantRole(t, la, b, 0)
	wantRole(t, lc, b, 0)
}

func TestLastAdminCannotLeave(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)

	if _, err := l.Issue(a, OpRemove, a.Public(), 0, ""); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("Issue(remove self) = %v, want ErrLastAdmin", err)
	}
	if _, err := l.Issue(a, OpSetRole, a.Public(), RoleReader, ""); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("Issue(demote self) = %v, want ErrLastAdmin", err)
	}

	// A forged op that bypasses Issue is nullified by the fold.
	forged := Op{Kind: OpRemove, Subject: a.Public(), Author: a.Public(), Prev: l.Frontier(), Lamport: 1}
	forged.sign(a, l.Project())
	if err := l.Apply(forged); err != nil {
		t.Fatalf("Apply(forged) = %v", err)
	}
	wantRole(t, l, a, RoleAdmin)

	// Two admins demoting each other concurrently leave exactly one admin.
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	lb := fork(t, l)
	mustIssue(t, l, a, OpSetRole, b, RoleReader)
	forge(t, lb, b, OpSetRole, a, RoleReader) // Issue refuses: a is senior.
	merge(t, l, lb)
	admins := 0
	for _, m := range l.Members() {
		if m.Role == RoleAdmin {
			admins++
		}
	}
	if admins != 1 {
		t.Errorf("admins after crossed demotions = %d, want 1", admins)
	}
	if ra, rb := l.Role(a.Public()), lb.Role(a.Public()); ra != rb {
		t.Errorf("replicas disagree on a: %v vs %v", ra, rb)
	}
}

func TestWrongProjectRejected(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l1, l2 := project(t, a), project(t, b)
	g2 := l2.Since(nil)[0]
	if err := l1.Apply(g2); !errors.Is(err, ErrWrongProject) {
		t.Errorf("Apply(foreign genesis) = %v, want ErrWrongProject", err)
	}

	op := mustIssue(t, l2, b, OpAdd, a, RoleWriter)
	if err := l1.Apply(op); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Apply(foreign op) = %v, want ErrBadSignature", err)
	}

	att, err := NewAttestation(b, l2.Project(), "doc", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := l1.Verify(att); !errors.Is(err, ErrWrongProject) {
		t.Errorf("Verify(foreign attestation) = %v, want ErrWrongProject", err)
	}
	if l1.Len() != 1 {
		t.Errorf("Len() = %d, want 1", l1.Len())
	}
}

func TestApplyOutOfOrderIsPendingThenIntegrated(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)
	src := project(t, a)
	mustIssue(t, src, a, OpAdd, b, RoleWriter)
	mustIssue(t, src, a, OpAdd, c, RoleReader)
	ops := src.Since(nil) // genesis, add b, add c

	l := New(src.Project())
	if err := l.Apply(ops[1]); !errors.Is(err, ErrUnknownAuthor) {
		t.Errorf("Apply(op before the genesis) = %v, want ErrUnknownAuthor", err)
	}
	mustApply(t, l, ops[0])
	mustApply(t, l, ops[2]) // by a known admin: buffered.
	if l.Pending() != 1 || l.Len() != 1 {
		t.Fatalf("pending=%d len=%d, want 1 and 1", l.Pending(), l.Len())
	}
	mustApply(t, l, ops[1])
	if l.Pending() != 0 || l.Len() != 3 {
		t.Errorf("pending=%d len=%d, want 0 and 3", l.Pending(), l.Len())
	}
	wantRole(t, l, b, RoleWriter)
	wantRole(t, l, c, RoleReader)
	if err := l.Apply(ops[1]); err != nil {
		t.Errorf("Apply(duplicate) = %v, want nil", err)
	}
}

func TestStrangerOpsRefused(t *testing.T) {
	a, x := newSigner(1), newSigner(9)
	l := project(t, a)
	forged := Op{Kind: OpAdd, Subject: x.Public(), Author: x.Public(), Role: RoleAdmin, Prev: l.Frontier(), Lamport: 1}
	forged.sign(x, l.Project())
	if err := l.Apply(forged); !errors.Is(err, ErrUnknownAuthor) {
		t.Errorf("Apply(stranger) = %v, want ErrUnknownAuthor", err)
	}
	bad := forged
	bad.Lamport = 5
	bad.sign(x, l.Project())
	if err := l.Apply(bad); err == nil {
		t.Error("Apply(bad lamport) = nil, want error")
	}
	wantRole(t, l, x, 0)
}

func TestIssueGuards(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	empty := New(ProjectID{1})
	if _, err := empty.Issue(a, OpAdd, b.Public(), RoleReader, ""); !errors.Is(err, ErrNoGenesis) {
		t.Errorf("Issue on empty list = %v, want ErrNoGenesis", err)
	}
	l := project(t, a)
	if _, err := l.Issue(b, OpAdd, b.Public(), RoleReader, ""); !errors.Is(err, ErrNotAdmin) {
		t.Errorf("Issue by stranger = %v, want ErrNotAdmin", err)
	}
	if _, err := l.Issue(a, OpGenesis, a.Public(), RoleAdmin, ""); err == nil {
		t.Error("Issue(genesis) = nil, want error")
	}
}

func TestMembersOrderAndNames(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	if _, err := l.Issue(a, OpAdd, b.Public(), RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Issue(a, OpSetRole, a.Public(), RoleAdmin, "Alice"); err != nil {
		t.Fatal(err)
	}
	ms := l.Members()
	if len(ms) != 2 || ms[0].Name != "Alice" || ms[1].Name != "Bob" || ms[1].Role != RoleWriter {
		t.Errorf("Members() = %+v", ms)
	}
	if ms[0].Person == ms[1].Person {
		t.Error("distinct people share a Person hash")
	}
}

func TestAttestation(t *testing.T) {
	a, w, r := newSigner(1), newSigner(2), newSigner(3)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	mustIssue(t, l, a, OpAdd, r, RoleReader)
	heads := []Hash{{9}, {3}, {9}}

	att, err := NewAttestation(w, l.Project(), "doc1", heads, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(att.Heads) != 2 || att.Heads[0] != (Hash{3}) {
		t.Errorf("Heads = %v, want sorted and deduplicated", att.Heads)
	}
	if err := l.Verify(att); err != nil {
		t.Errorf("Verify(writer) = %v", err)
	}
	back, err := DecodeAttestation(EncodeAttestation(att))
	if err != nil || l.Verify(back) != nil {
		t.Errorf("round trip = %v, verify %v", err, l.Verify(back))
	}

	tampered := att
	tampered.Seq++
	if err := l.Verify(tampered); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Verify(tampered) = %v, want ErrBadSignature", err)
	}
	ratt, err := NewAttestation(r, l.Project(), "doc1", heads, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(ratt); !errors.Is(err, ErrNotWriter) {
		t.Errorf("Verify(reader) = %v, want ErrNotWriter", err)
	}
	mustIssue(t, l, a, OpRemove, w, 0)
	if err := l.Verify(att); !errors.Is(err, ErrNotWriter) {
		t.Errorf("Verify(removed writer) = %v, want ErrNotWriter", err)
	}
	if _, err := NewAttestation(w, l.Project(), "", nil, 1); err == nil {
		t.Error("NewAttestation(empty doc) = nil error")
	}
}

// chain applies n SetRole ops by s, each following the previous one. It
// bypasses Issue, which folds on every call.
func chain(t testing.TB, l *List, s Signer, n int) error {
	t.Helper()
	for range n {
		f := l.Frontier()
		op := Op{
			Kind: OpSetRole, Role: RoleAdmin, Subject: s.Public(), Author: s.Public(), Prev: f,
			Lamport: l.ops[f[0]].op.Lamport + 1, Name: "a name long enough to take some space in each op",
		}
		op.sign(s, l.Project())
		if err := l.Apply(op); err != nil {
			return err
		}
	}
	return nil
}

func TestTooManyOps(t *testing.T) {
	a := newSigner(1)
	l := project(t, a)
	if err := chain(t, l, a, maxOps+1); !errors.Is(err, ErrTooManyOps) {
		t.Errorf("filling the list ended with %v, want ErrTooManyOps", err)
	}
	if l.Len() != maxOps {
		t.Errorf("Len() = %d, want %d", l.Len(), maxOps)
	}
	if !l.Sealed() {
		t.Error("Sealed() = false, want true")
	}
}
