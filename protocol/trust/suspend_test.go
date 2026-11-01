// SPDX-License-Identifier: MPL-2.0

package trust

import (
	"bytes"
	"errors"
	"testing"
)

// susp reports whether who is suspended in l.
func susp(l *List, who Signer) bool { return l.Suspended(who.Public()) }

func wantSuspended(t testing.TB, l *List, who Signer, want bool) {
	t.Helper()
	if got := susp(l, who); got != want {
		t.Errorf("Suspended(%x) = %v, want %v", who.Public()[:4], got, want)
	}
}

// personOf returns the Person of the member who.
func personOf(t testing.TB, l *List, who Signer) Hash {
	t.Helper()
	for _, m := range l.Members() {
		if m.Pub.Equal(who.Public()) {
			return m.Person
		}
	}
	t.Fatalf("%x is not a member", who.Public()[:4])
	return Hash{}
}

func TestSuspend(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)

	t.Run("ACL-03 resume keeps the role and the rank", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		s := mustIssue(t, l, a, OpSuspend, b, 0)
		wantSuspended(t, l, b, true)
		wantRole(t, l, b, RoleAdmin)
		var by []byte
		for _, m := range l.Members() {
			if m.Pub.Equal(b.Public()) {
				by = m.SuspendedBy
			}
		}
		if !bytes.Equal(by, a.Public()) {
			t.Errorf("SuspendedBy = %x, want the creator", by)
		}
		if s.Kind != OpSuspend || s.Role != 0 || s.Name != "" {
			t.Errorf("Issue(OpSuspend) = %+v, want no role and no name", s)
		}
		mustIssue(t, l, a, OpResume, b, 0)
		wantSuspended(t, l, b, false)
		wantRole(t, l, b, RoleAdmin)
		// B is still an admin of the same rank: its op stands.
		mustIssue(t, l, b, OpAdd, c, RoleWriter)
		wantRole(t, l, c, RoleWriter)
	})

	t.Run("ACL-04 a writer cannot forge", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		mustIssue(t, l, a, OpAdd, c, RoleReader)
		forged := craft(t, l, b, OpSuspend, c, 0, "", l.Frontier())
		if err := l.Apply(forged); !errors.Is(err, ErrUnknownAuthor) {
			t.Errorf("Apply(suspension by a writer) = %v, want ErrUnknownAuthor", err)
		}
		if _, err := l.Issue(b, OpSuspend, c.Public(), 0, ""); !errors.Is(err, ErrNotAdmin) {
			t.Errorf("Issue(suspension by a writer) = %v, want ErrNotAdmin", err)
		}
		wantSuspended(t, l, c, false)
	})

	t.Run("ACL-05 a senior or the creator is out of reach", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleAdmin)
		if _, err := l.Issue(c, OpSuspend, b.Public(), 0, ""); !errors.Is(err, ErrOutranked) {
			t.Errorf("Issue(suspend a senior admin) = %v, want ErrOutranked", err)
		}
		if _, err := l.Issue(b, OpSuspend, a.Public(), 0, ""); !errors.Is(err, ErrOutranked) {
			t.Errorf("Issue(suspend the creator) = %v, want ErrOutranked", err)
		}
		// A forged op is stored without effect, and what the suspender signs afterwards stands.
		for _, tc := range []struct{ by, target *testSigner }{{b, a}, {c, b}, {c, a}} {
			forge(t, l, tc.by, OpSuspend, tc.target, 0)
			wantSuspended(t, l, tc.target, false)
		}
		mustIssue(t, l, a, OpAdd, newSigner(9), RoleWriter)
		mustIssue(t, l, b, OpAdd, newSigner(10), RoleWriter)
		wantAdmins(t, l, 3)
	})

	t.Run("ACL-05 nobody suspends or resumes itself", func(t *testing.T) {
		l := project(t, a)
		for _, k := range []OpKind{OpSuspend, OpResume} {
			if _, err := l.Issue(a, k, a.Public(), 0, ""); !errors.Is(err, ErrMalformed) {
				t.Errorf("Issue(kind %d on itself) = %v, want ErrMalformed", k, err)
			}
			self := Op{Kind: k, Author: a.Public(), Subject: a.Public(), Prev: l.Frontier(), Lamport: 1}
			self.sign(a, l.Project())
			if _, err := Decode(Encode(self)); !errors.Is(err, ErrMalformed) {
				t.Errorf("Decode(kind %d on itself) = %v, want ErrMalformed", k, err)
			}
			if err := l.Apply(self); !errors.Is(err, ErrMalformed) {
				t.Errorf("Apply(kind %d on itself) = %v, want ErrMalformed", k, err)
			}
		}
	})

	t.Run("S1 a senior that stepped down can be suspended by a junior", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleAdmin)
		mustIssue(t, l, b, OpSetRole, b, RoleWriter) // B steps down; C is junior to B.
		if _, err := l.Issue(c, OpSuspend, b.Public(), 0, ""); err != nil {
			t.Fatalf("Issue(suspend a senior that is not admin) = %v", err)
		}
		wantSuspended(t, l, b, true)
		wantRole(t, l, b, RoleWriter)
	})

	t.Run("ACL-06 a concurrent resume loses against a suspension", func(t *testing.T) {
		la := project(t, a)
		mustIssue(t, la, a, OpAdd, b, RoleAdmin)
		mustIssue(t, la, a, OpAdd, c, RoleWriter)
		lb := fork(t, la)
		mustIssue(t, la, a, OpSuspend, c, 0)
		mustIssue(t, lb, b, OpResume, c, 0)
		lc := New(la.Project())
		merge(t, la, lb)
		mustApplyAll(t, lc, lb.Since(nil)) // the other arrival order
		mustApplyAll(t, lc, la.Since(nil))
		for name, l := range map[string]*List{"A": la, "B": lb, "C": lc} {
			if !susp(l, c) {
				t.Errorf("replica %s: Suspended(C) = false, want true: the suspension beats the concurrent resume", name)
			}
		}
		// A resume that sees the suspension wins.
		mustIssue(t, lb, b, OpResume, c, 0)
		merge(t, la, lb)
		wantSuspended(t, la, c, false)
		wantSuspended(t, lb, c, false)
	})

	t.Run("ACL-07 a readmission after a removal is not suspended", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		mustIssue(t, l, a, OpSuspend, b, 0)
		mustIssue(t, l, a, OpRemove, b, 0)
		wantSuspended(t, l, b, false)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		wantRole(t, l, b, RoleWriter)
		wantSuspended(t, l, b, false)
		mustIssue(t, l, a, OpSuspend, b, 0) // a new suspension counts again
		wantSuspended(t, l, b, true)
	})

	t.Run("ACL-12 a backdated op of a suspended admin stays dead after the resume", func(t *testing.T) {
		la := project(t, a)
		mustIssue(t, la, a, OpAdd, b, RoleAdmin)
		lb := fork(t, la)
		mustIssue(t, la, a, OpSuspend, b, 0)
		// B, who does not know yet, signs concurrently to the suspension.
		d, e := newSigner(7), newSigner(8)
		mustApply(t, lb, craft(t, lb, b, OpAdd, d, RoleWriter, "", lb.Frontier()))
		wantRole(t, lb, d, RoleWriter)
		merge(t, la, lb)
		wantRole(t, la, d, 0)
		wantRole(t, lb, d, 0)
		mustIssue(t, la, a, OpResume, b, 0)
		merge(t, la, lb)
		wantSuspended(t, la, b, false)
		wantRole(t, la, d, 0)
		wantRole(t, lb, d, 0)
		// What B signs once it has seen the suspension and the resume stands.
		mustIssue(t, lb, b, OpAdd, e, RoleWriter)
		wantRole(t, lb, e, RoleWriter)
	})

	t.Run("ACL-12 a suspended admin signs nothing that stands", func(t *testing.T) {
		la := project(t, a)
		mustIssue(t, la, a, OpAdd, b, RoleAdmin)
		mustIssue(t, la, a, OpSuspend, b, 0)
		if _, err := la.Issue(b, OpAdd, c.Public(), RoleWriter, ""); !errors.Is(err, ErrSuspended) {
			t.Errorf("Issue(by a suspended admin) = %v, want ErrSuspended", err)
		}
		// Forged anyway, with the suspension in its past.
		mustApply(t, la, craft(t, la, b, OpAdd, c, RoleWriter, "", la.Frontier()))
		wantRole(t, la, c, 0)
		// It cannot resume itself, nor remove the suspender's work.
		mustApply(t, la, craft(t, la, b, OpRemove, newSigner(5), 0, "", la.Frontier()))
		wantSuspended(t, la, b, true)
	})

	t.Run("ACL-11 two devices of a person are suspended together", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		p := personOf(t, l, b)
		if _, err := l.IssueWith(a, Op{Kind: OpAdd, Subject: c.Public(), Role: RoleWriter, Person: p}); err != nil {
			t.Fatal(err)
		}
		ops, err := l.IssueBatch(a, []Op{{Kind: OpSuspend, Subject: b.Public()}, {Kind: OpSuspend, Subject: c.Public()}})
		if err != nil || len(ops) != 2 {
			t.Fatalf("IssueBatch() = %d ops, %v; want 2, nil", len(ops), err)
		}
		wantSuspended(t, l, b, true)
		wantSuspended(t, l, c, true)
		third := newSigner(6)
		if _, err := l.IssueWith(a, Op{Kind: OpAdd, Subject: third.Public(), Role: RoleWriter, Person: p}); !errors.Is(err, ErrSuspended) {
			t.Errorf("IssueWith(device of a suspended person) = %v, want ErrSuspended", err)
		}
		if _, err := l.IssueBatch(a, []Op{{Kind: OpResume, Subject: b.Public()}, {Kind: OpResume, Subject: c.Public()}}); err != nil {
			t.Fatal(err)
		}
		wantSuspended(t, l, b, false)
		wantSuspended(t, l, c, false)
		if _, err := l.IssueWith(a, Op{Kind: OpAdd, Subject: third.Public(), Role: RoleWriter, Person: p}); err != nil {
			t.Errorf("IssueWith(device of a resumed person) = %v", err)
		}
	})

	t.Run("ACL-14 a batch that would seal the list is refused whole", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		mustIssue(t, l, a, OpAdd, c, RoleWriter)
		if err := chain(t, l, a, maxOps-2-l.Len()); err != nil {
			t.Fatal(err)
		}
		if l.Len() != maxOps-2 {
			t.Fatalf("Len() = %d, want %d", l.Len(), maxOps-2)
		}
		before := l.Len()
		batch := []Op{{Kind: OpSuspend, Subject: b.Public()}, {Kind: OpSuspend, Subject: c.Public()}}
		if _, err := l.IssueBatch(a, batch); !errors.Is(err, ErrTooManyOps) {
			t.Fatalf("IssueBatch(sealing) = %v, want ErrTooManyOps", err)
		}
		if l.Len() != before || l.Sealed() {
			t.Errorf("Len() = %d, Sealed() = %v after a refused batch, want %d and false", l.Len(), l.Sealed(), before)
		}
		wantSuspended(t, l, b, false)
		wantSuspended(t, l, c, false)
		// A smaller batch that does not seal goes through.
		if _, err := l.IssueBatch(a, batch[:1]); err != nil {
			t.Errorf("IssueBatch(1 op) = %v", err)
		}
		wantSuspended(t, l, b, true)
	})

	t.Run("a batch is refused whole if one op is", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleWriter)
		before := l.Len()
		// B cannot suspend the creator, so nothing of the batch is issued.
		_, err := l.IssueBatch(b, []Op{{Kind: OpSuspend, Subject: c.Public()}, {Kind: OpSuspend, Subject: a.Public()}})
		if !errors.Is(err, ErrOutranked) || l.Len() != before {
			t.Errorf("IssueBatch() = %v, Len() = %d; want ErrOutranked and %d", err, l.Len(), before)
		}
		if ops, err := l.IssueBatch(b, nil); err != nil || len(ops) != 0 {
			t.Errorf("IssueBatch(nil) = %v, %v; want nothing", ops, err)
		}
	})

	t.Run("a device suspended with its person cannot resume its sibling", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		p := personOf(t, l, b)
		if _, err := l.IssueWith(a, Op{Kind: OpAdd, Subject: c.Public(), Role: RoleAdmin, Person: p}); err != nil {
			t.Fatal(err)
		}
		if _, err := l.IssueBatch(a, []Op{{Kind: OpSuspend, Subject: b.Public()}, {Kind: OpSuspend, Subject: c.Public()}}); err != nil {
			t.Fatal(err)
		}
		mustApply(t, l, craft(t, l, b, OpResume, c, 0, "", l.Frontier()))
		mustApply(t, l, craft(t, l, c, OpResume, b, 0, "", l.Frontier()))
		wantSuspended(t, l, b, true)
		wantSuspended(t, l, c, true)
	})

	t.Run("replaying an old resume does not undo a later suspension", func(t *testing.T) {
		la := project(t, a)
		mustIssue(t, la, a, OpAdd, b, RoleWriter)
		old := mustIssue(t, la, a, OpResume, b, 0)
		mustIssue(t, la, a, OpSuspend, b, 0)
		lb := New(la.Project())
		mustApplyAll(t, lb, la.Since(nil))
		mustApply(t, lb, old) // the replay is a known op
		mustApply(t, la, old)
		wantSuspended(t, la, b, true)
		wantSuspended(t, lb, b, true)
	})

	t.Run("an attestation from before the suspension is refused", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		att, err := NewAttestation(b, l.Project(), "doc", []Hash{{1}}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Verify(att); err != nil {
			t.Fatalf("Verify(writer) = %v", err)
		}
		mustIssue(t, l, a, OpSuspend, b, 0)
		if err := l.Verify(att); !errors.Is(err, ErrSuspended) {
			t.Errorf("Verify(suspended writer) = %v, want ErrSuspended", err)
		}
		mustIssue(t, l, a, OpResume, b, 0)
		if err := l.Verify(att); err != nil {
			t.Errorf("Verify(resumed writer) = %v", err)
		}
	})

	t.Run("a suspended reader is not a writer", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleReader)
		mustIssue(t, l, a, OpSuspend, b, 0)
		att, err := NewAttestation(b, l.Project(), "doc", nil, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Verify(att); !errors.Is(err, ErrNotWriter) {
			t.Errorf("Verify(suspended reader) = %v, want ErrNotWriter", err)
		}
	})

	t.Run("a suspension of a non-member has no effect", func(t *testing.T) {
		l := project(t, a)
		forge(t, l, a, OpSuspend, b, 0)
		if susp(l, b) || l.Role(b.Public()) != 0 {
			t.Error("a non-member is suspended or has a role")
		}
	})
}
