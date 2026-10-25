package trust

import (
	"errors"
	"fmt"
	"testing"
)

// widen makes the frontier of l wider than MaxPrev: f signs a chain head after
// g and then MaxPrev+5 concurrent children of it.
func widen(t *testing.T, l *List, f Signer, g Op) {
	t.Helper()
	x := craft(t, l, f, OpSetRole, f, RoleAdmin, "x", h(g))
	mustApply(t, l, x)
	for i := range MaxPrev + 5 {
		mustApply(t, l, craft(t, l, f, OpSetRole, f, RoleAdmin, fmt.Sprint(i), h(x)))
	}
	if len(l.Frontier()) <= MaxPrev {
		t.Fatalf("frontier of %d heads, want more than %d", len(l.Frontier()), MaxPrev)
	}
}

// wantStands checks that every op stands in the fold of l.
func wantStands(t interface {
	Helper()
	Errorf(string, ...any)
}, l *List, ops ...Op,
) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.current()
	for _, o := range ops {
		if i, ok := v.index[o.Hash()]; !ok || !v.stands[i] {
			t.Errorf("issued op kind %d on %x does not stand", o.Kind, o.Subject[:4])
		}
	}
}

func TestSuspendAudit(t *testing.T) {
	a, b, c, d, w := newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(5)

	t.Run("F1 a junior's suspension lapses when a senior grants admin again", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleAdmin)
		mustIssue(t, l, b, OpSetRole, b, RoleWriter)
		mustIssue(t, l, c, OpSuspend, b, 0)
		wantSuspended(t, l, b, true)
		if _, err := l.Issue(a, OpSetRole, b.Public(), RoleAdmin, ""); !errors.Is(err, ErrSuspended) {
			t.Errorf("Issue(admin grant to a suspended member) = %v, want ErrSuspended", err)
		}
		forge(t, l, a, OpSetRole, b, RoleAdmin) // a grant made before the news of the suspension
		wantSuspended(t, l, b, false)
		forge(t, l, b, OpAdd, d, RoleWriter)
		wantRole(t, l, d, RoleWriter) // state and effect agree: B is active and its op stands.
	})

	t.Run("F1 a suspension by a senior survives a later grant", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		mustIssue(t, l, a, OpSuspend, b, 0)
		forge(t, l, a, OpSetRole, b, RoleAdmin)
		wantSuspended(t, l, b, true)
		forge(t, l, b, OpAdd, d, RoleWriter)
		wantRole(t, l, d, 0)
	})

	t.Run("F2 the last active admin cannot leave", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleAdmin)
		mustIssue(t, l, a, OpSetRole, a, RoleWriter)
		mustIssue(t, l, b, OpSuspend, c, 0)
		if _, err := l.Issue(b, OpSetRole, b.Public(), RoleWriter, ""); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("Issue(leave with only a suspended admin left) = %v, want ErrLastAdmin", err)
		}
		if _, err := l.Issue(b, OpRemove, b.Public(), 0, ""); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("Issue(remove oneself with only a suspended admin left) = %v, want ErrLastAdmin", err)
		}
		mustIssue(t, l, b, OpResume, c, 0)
		mustIssue(t, l, b, OpSetRole, b, RoleWriter) // C is active again.
	})

	t.Run("F2 only suspended admins left: the last active admin stays", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpSuspend, b, 0)
		if _, err := l.Issue(a, OpSetRole, a.Public(), RoleWriter, ""); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("Issue(creator leaves, the other admin suspended) = %v, want ErrLastAdmin", err)
		}
	})

	t.Run("F3 a suspended admin cannot fill the pending buffer", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleAdmin)
		mustIssue(t, l, a, OpSuspend, b, 0)
		for i := range maxPending + 5 {
			o := Op{Kind: OpAdd, Author: b.Public(), Subject: key(9, i).Public(), Role: RoleWriter, Prev: []Hash{{byte(i), byte(i >> 8), 1}}, Lamport: 5}
			o.sign(b, l.Project())
			if err := l.Apply(o); err == nil {
				t.Fatalf("Apply(op of a suspended admin with unknown parents) buffered it (op %d)", i)
			}
		}
		o := Op{Kind: OpAdd, Author: c.Public(), Subject: key(8, 1).Public(), Role: RoleWriter, Prev: []Hash{{7, 7, 7}}, Lamport: 9}
		o.sign(c, l.Project())
		if err := l.Apply(o); err != nil {
			t.Errorf("Apply(op of an active admin with unknown parents) = %v, want it buffered", err)
		}
	})

	t.Run("F4 a capped Prev covers the issuer's resume", func(t *testing.T) {
		f := newSigner(6)
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, f, RoleAdmin)
		s := mustIssue(t, l, a, OpSuspend, b, 0)
		mustIssue(t, l, a, OpResume, b, 0)
		widen(t, l, f, s)
		wantSuspended(t, l, b, false)
		op, err := l.Issue(b, OpAdd, w.Public(), RoleWriter, "")
		if err != nil {
			t.Fatalf("Issue() = %v", err)
		}
		wantStands(t, l, op)
		wantRole(t, l, w, RoleWriter)
	})

	t.Run("F4 suspend and resume stand with a frontier wider than MaxPrev", func(t *testing.T) {
		f := newSigner(6)
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleWriter)
		g := mustIssue(t, l, a, OpAdd, f, RoleAdmin)
		widen(t, l, f, g)
		s, err := l.Issue(a, OpSuspend, b.Public(), 0, "")
		if err != nil {
			t.Fatal(err)
		}
		wantStands(t, l, s)
		wantSuspended(t, l, b, true)
		r, err := l.Issue(a, OpResume, b.Public(), 0, "")
		if err != nil {
			t.Fatal(err)
		}
		wantStands(t, l, r)
		wantSuspended(t, l, b, false)
	})

	t.Run("F5 a batch with a wide frontier keeps the standing ops of every subject", func(t *testing.T) {
		f := newSigner(6)
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		if _, err := l.IssueWith(a, Op{Kind: OpAdd, Subject: c.Public(), Role: RoleAdmin, Person: personOf(t, l, b)}); err != nil {
			t.Fatal(err)
		}
		g := mustIssue(t, l, a, OpAdd, f, RoleAdmin)
		mustApply(t, l, craft(t, l, c, OpAdd, w, RoleWriter, "", h(g)))
		widen(t, l, f, g)
		wantRole(t, l, w, RoleWriter)
		ops, err := l.IssueBatch(a, []Op{{Kind: OpSuspend, Subject: b.Public()}, {Kind: OpSuspend, Subject: c.Public()}})
		if err != nil {
			t.Fatal(err)
		}
		wantStands(t, l, ops...)
		wantRole(t, l, w, RoleWriter)
		wantSuspended(t, l, b, true)
		wantSuspended(t, l, c, true)
	})

	t.Run("F6 the state and the effect of a suspension agree", func(t *testing.T) {
		l := project(t, a)
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustIssue(t, l, a, OpAdd, c, RoleAdmin)
		mustIssue(t, l, a, OpSuspend, b, 0)
		mustIssue(t, l, a, OpSetRole, b, RoleWriter)
		mustIssue(t, l, c, OpRemove, b, 0) // a junior removes a senior that is not admin.
		mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		op, err := l.Issue(b, OpAdd, d.Public(), RoleWriter, "")
		if l.Suspended(b.Public()) {
			if err == nil {
				t.Error("B is shown suspended and Issue accepted its op")
			}
			return
		}
		if err != nil {
			t.Fatalf("Issue() = %v", err)
		}
		wantStands(t, l, op)
	})

	t.Run("a suspension concurrent with a removal does not outlive the readmission", func(t *testing.T) {
		for _, aRemoves := range []bool{true, false} {
			la := project(t, a)
			mustIssue(t, la, a, OpAdd, b, RoleWriter)
			mustIssue(t, la, a, OpAdd, c, RoleAdmin)
			lc := fork(t, la)
			// The two ops are concurrent; ≺ orders them by Lamport number, then
			// author key, so swapping the authors tries both orders.
			remover, suspender := Signer(a), Signer(c)
			if !aRemoves {
				remover, suspender = c, a
			}
			mustIssue(t, la, remover, OpRemove, b, 0)
			mustIssue(t, lc, suspender, OpSuspend, b, 0)
			merge(t, la, lc)
			mustIssue(t, la, a, OpAdd, b, RoleWriter)
			wantRole(t, la, b, RoleWriter)
			wantSuspended(t, la, b, false)
		}
	})
}
