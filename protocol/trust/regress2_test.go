// SPDX-License-Identifier: MPL-2.0

package trust

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// PoC N1: after a hand-over, the ex-creator backdates ops before its exit.
func TestExCreatorCannotBackdateAfterHandOver(t *testing.T) {
	a, b, w, evil := newSigner(1), newSigner(2), newSigner(3), newSigner(66)
	l := project(t, a)
	addB := mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpRemove, a, 0) // hand-over, honoured.
	mustIssue(t, l, b, OpAdd, w, RoleWriter)
	rmB := craft(t, l, a, OpRemove, b, 0, "", h(addB))
	mustApply(t, l, rmB)
	mustApply(t, l, craft(t, l, a, OpAdd, evil, RoleAdmin, "", h(rmB)))
	wantRole(t, l, b, RoleAdmin)
	wantRole(t, l, w, RoleWriter)
	wantRole(t, l, evil, 0)
	if _, err := l.Issue(evil, OpAdd, newSigner(77).Public(), RoleWriter, ""); !errors.Is(err, ErrNotAdmin) {
		t.Errorf("Issue by evil = %v, want ErrNotAdmin", err)
	}
}

// PoC N2: an admin that left cannot backdate either.
func TestSelfLeaverCannotBackdate(t *testing.T) {
	a, s, j, w := newSigner(1), newSigner(2), newSigner(3), newSigner(4)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, s, RoleAdmin)
	addJ := mustIssue(t, l, a, OpAdd, j, RoleAdmin)
	mustIssue(t, l, s, OpRemove, s, 0)
	mustIssue(t, l, j, OpAdd, w, RoleWriter)
	mustApply(t, l, craft(t, l, s, OpRemove, j, 0, "", h(addJ)))
	wantRole(t, l, s, 0)
	wantRole(t, l, j, RoleAdmin)
	wantRole(t, l, w, RoleWriter)
}

// PoC N3: a flood of heads must not make an honest cut forget the subject's ops.
func TestCapPrevDoesNotResurrect(t *testing.T) {
	a, z, m, x := newSigner(1), newSigner(2), newSigner(3), newSigner(9)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, z, RoleAdmin)
	addM := mustIssue(t, l, z, OpAdd, m, RoleAdmin)
	mustIssue(t, l, z, OpRemove, m, 0)
	mustIssue(t, l, z, OpAdd, newSigner(5), RoleWriter)
	prev := addM
	for range 10 {
		o := craft(t, l, m, OpSetRole, m, RoleAdmin, "", h(prev))
		mustApply(t, l, o)
		prev = o
	}
	payload := craft(t, l, m, OpAdd, x, RoleAdmin, "", h(prev))
	mustApply(t, l, payload)
	for i := range MaxPrev + 1 {
		mustApply(t, l, craft(t, l, m, OpSetRole, m, RoleAdmin, fmt.Sprint(i), h(payload)))
	}
	op, err := l.Issue(a, OpSetRole, z.Public(), RoleWriter, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(op.Prev) != MaxPrev {
		t.Errorf("len(Prev) = %d, want %d", len(op.Prev), MaxPrev)
	}
	wantRole(t, l, z, RoleWriter)
	wantRole(t, l, m, 0)
	wantRole(t, l, x, 0)
}

// After a capped Issue, every op of the subject that stood still stands.
func TestCappedIssueKeepsSubjectOpsStanding(t *testing.T) {
	keys := testKeys()
	rapid.Check(t, func(rt *rapid.T) {
		l, _ := genList(rt, keys)
		var admins []*testSigner
		for _, k := range keys {
			if l.Role(k.Public()) == RoleAdmin {
				admins = append(admins, k)
			}
		}
		if len(admins) == 0 {
			rt.Skip("no admin left")
		}
		issuer := rapid.SampledFrom(admins).Draw(rt, "issuer")
		subject := rapid.SampledFrom(keys).Draw(rt, "subject")
		var flooders []*testSigner
		for _, k := range keys {
			if k != subject && k != issuer && slices.ContainsFunc(l.Since(nil), func(o Op) bool { return o.Role == RoleAdmin && slices.Equal(o.Subject, k.Public()) }) {
				flooders = append(flooders, k)
			}
		}
		if len(flooders) == 0 {
			rt.Skip("nobody can flood")
		}
		f := rapid.SampledFrom(flooders).Draw(rt, "flooder")
		anchor := l.Since(nil)[slices.IndexFunc(l.Since(nil), func(o Op) bool { return o.Role == RoleAdmin && slices.Equal(o.Subject, f.Public()) })]
		for i := range MaxPrev + 5 { // concurrent heads, dead or alive, backdated to the grant.
			o := Op{Kind: OpSetRole, Role: RoleAdmin, Subject: f.Public(), Author: f.Public(), Prev: h(anchor), Lamport: anchor.Lamport + 1, Name: fmt.Sprint(i)}
			o.sign(f, l.Project())
			if err := l.Apply(o); err != nil {
				rt.Fatalf("Apply(flood) = %v", err)
			}
		}
		stood := map[Hash]bool{}
		before := l.current()
		for i, ok := range before.stands {
			if ok && string(before.nodes[i].op.Author) == string(subject.Public()) {
				stood[before.nodes[i].hash] = true
			}
		}
		kind := rapid.SampledFrom([]OpKind{OpAdd, OpSetRole, OpRemove}).Draw(rt, "kind")
		role := rapid.SampledFrom([]Role{RoleReader, RoleWriter, RoleAdmin}).Draw(rt, "role")
		if _, err := l.Issue(issuer, kind, subject.Public(), role, ""); err != nil {
			rt.Skip(err.Error())
		}
		after := l.current()
		for i, n := range after.nodes {
			if stood[n.hash] && !after.stands[i] {
				rt.Fatalf("op %d of the subject stood before the capped Issue and is dead after", i)
			}
		}
	})
}

// PoC N4: the last admin cannot leave, by Issue or by a forged op.
func TestLastAdminCannotLeaveAfterHandOver(t *testing.T) {
	a, b, x := newSigner(1), newSigner(2), newSigner(9)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpRemove, a, 0)
	for _, tc := range []struct {
		k OpKind
		r Role
	}{{OpRemove, 0}, {OpSetRole, RoleWriter}} {
		if _, err := l.Issue(b, tc.k, b.Public(), tc.r, ""); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("Issue(kind %d on itself) = %v, want ErrLastAdmin", tc.k, err)
		}
	}
	forge(t, l, b, OpRemove, b, 0)
	wantRole(t, l, b, RoleAdmin)
	mustIssue(t, l, b, OpAdd, x, RoleAdmin)
	mustIssue(t, l, b, OpRemove, b, 0) // x is there now.
	wantRole(t, l, x, RoleAdmin)
}

// PoC N5: a senior that stepped down is removable once it is no longer admin.
func TestStepDownSeniorRemovableAfterHandOver(t *testing.T) {
	a, d, b := newSigner(1), newSigner(2), newSigner(3)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, d, RoleAdmin)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	if _, err := l.Issue(b, OpRemove, d.Public(), 0, ""); !errors.Is(err, ErrOutranked) {
		t.Errorf("Issue(remove a senior admin) = %v, want ErrOutranked", err)
	}
	mustIssue(t, l, d, OpSetRole, d, RoleWriter) // d steps down.
	mustIssue(t, l, a, OpSetRole, a, RoleWriter) // the creator hands over.
	mustIssue(t, l, b, OpRemove, d, 0)
	mustIssue(t, l, b, OpRemove, a, 0)
	wantRole(t, l, d, 0)
	wantRole(t, l, a, 0)
	wantRole(t, l, b, RoleAdmin)
	if len(l.Members()) != 1 {
		t.Errorf("members = %+v, want only b", l.Members())
	}
	// Demoting a senior writer further is a cut as well.
	m := newSigner(4)
	l2 := project(t, a)
	mustIssue(t, l2, a, OpAdd, m, RoleAdmin)
	mustIssue(t, l2, a, OpAdd, d, RoleAdmin)
	mustIssue(t, l2, m, OpSetRole, m, RoleWriter)
	mustIssue(t, l2, d, OpSetRole, m, RoleReader)
	wantRole(t, l2, m, RoleReader)
}

func TestCannotPromoteSenior(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpSetRole, a, RoleWriter) // hand-over: a is a writer, senior to b.
	for _, tc := range []struct {
		k    OpKind
		r    Role
		name string
	}{{OpSetRole, RoleAdmin, ""}, {OpAdd, RoleAdmin, ""}, {OpSetRole, RoleReader, "renamed"}, {OpAdd, RoleReader, "renamed"}} {
		if _, err := l.Issue(b, tc.k, a.Public(), tc.r, tc.name); !errors.Is(err, ErrOutranked) {
			t.Errorf("Issue(b, kind %d role %v name %q on senior) = %v, want ErrOutranked", tc.k, tc.r, tc.name, err)
		}
		forged := Op{Kind: tc.k, Role: tc.r, Name: tc.name, Subject: a.Public(), Author: b.Public(), Prev: l.Frontier()}
		forged.Lamport = l.ops[forged.Prev[0]].op.Lamport + 1
		forged.sign(b, l.Project())
		mustApply(t, l, forged)
	}
	wantRole(t, l, a, RoleWriter)
	if m := l.Members(); m[0].Name != "" {
		t.Errorf("senior renamed to %q", m[0].Name)
	}
	// A senior that is still admin cannot be cut either.
	c := newSigner(3)
	mustIssue(t, l, b, OpAdd, c, RoleAdmin)
	if _, err := l.Issue(c, OpRemove, b.Public(), 0, ""); !errors.Is(err, ErrOutranked) {
		t.Errorf("Issue(remove a senior admin) = %v, want ErrOutranked", err)
	}
}

// A junior may re-add, as a non-admin, a senior that is not admin (as it may
// remove or demote it), but never grant it admin, rename it or regroup it.
func TestJuniorMayReaddSeniorAsNonAdmin(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpRemove, a, 0) // hand-over: a is out, still senior to b.
	wantRole(t, l, a, 0)
	mustIssue(t, l, b, OpAdd, a, RoleWriter)
	wantRole(t, l, a, RoleWriter)
	if _, err := l.Issue(b, OpAdd, a.Public(), RoleAdmin, ""); !errors.Is(err, ErrOutranked) {
		t.Errorf("Issue(b, grant admin to a senior) = %v, want ErrOutranked", err)
	}
	if _, err := l.IssueWith(b, Op{Kind: OpAdd, Subject: a.Public(), Role: RoleReader, Person: Hash{1}}); !errors.Is(err, ErrOutranked) {
		t.Errorf("IssueWith(b, Add of a senior with a Person) = %v, want ErrOutranked", err)
	}
	// A senior that is still admin stays out of reach.
	c := newSigner(3)
	l2 := project(t, a)
	mustIssue(t, l2, a, OpAdd, c, RoleAdmin)
	if _, err := l2.Issue(c, OpAdd, a.Public(), RoleReader, ""); !errors.Is(err, ErrOutranked) {
		t.Errorf("Issue(c, re-add of a senior admin) = %v, want ErrOutranked", err)
	}
}

// A junior's cut of a senior does not beat the senior's concurrent re-grant.
func TestJuniorCutDoesNotBeatConcurrentRegrant(t *testing.T) {
	a, d, b := newSigner(1), newSigner(2), newSigner(3)
	la := project(t, a)
	mustIssue(t, la, a, OpAdd, d, RoleAdmin)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin)
	mustIssue(t, la, a, OpSetRole, d, RoleWriter)
	lb := fork(t, la)
	mustIssue(t, la, a, OpSetRole, d, RoleAdmin) // the senior re-grants d ...
	mustIssue(t, lb, b, OpRemove, d, 0)          // ... while the junior b removes it.
	merge(t, la, lb)
	for _, l := range []*List{la, lb} {
		wantRole(t, l, d, RoleAdmin)
	}
}

// PoC N6: self-cuts of the creator, each judged on its own ancestors, stay cheap enough.
func TestCreatorSelfCutsCostBounded(t *testing.T) {
	l, _, _ := buildCascade(t, 150)
	a := newSigner(1)
	heads := l.Frontier()
	var last []Hash
	for i := 0; i < len(heads); i += MaxPrev - 1 { // merge the heads of the cascade.
		p := slices.Concat(heads[i:min(i+MaxPrev-1, len(heads))], last)
		slices.SortFunc(p, func(x, y Hash) int { return slices.Compare(x[:], y[:]) })
		o := craft(t, l, a, OpAdd, key(50, i), RoleReader, "", p)
		mustApply(t, l, o)
		last = h(o)
	}
	base := time.Now()
	fold(l.ops)
	baseline := time.Since(base)
	for i := 0; !l.Sealed(); i++ {
		mustApply(t, l, craft(t, l, a, OpSetRole, a, RoleWriter, fmt.Sprint(i), last))
	}
	t0 := time.Now()
	fold(l.ops)
	d := time.Since(t0)
	t.Logf("fold of %d ops: %v (without the self-cuts: %v)", l.Len(), d, baseline)
	// Relative to the same fold without the self-cuts, so a loaded machine (-race, parallel
	// packages) does not fail it: a blow-up shows as a ratio, not as wall-clock time.
	if d > 5*time.Second && d > 50*baseline {
		t.Errorf("fold took %v, %.0f× the fold without the self-cuts (%v)", d, float64(d)/float64(baseline), baseline)
	}
}

// PoC N7: strangers cannot fill the buffer of ops waiting for their parents.
func TestStrangerCannotFillPending(t *testing.T) {
	a, b, stranger := newSigner(1), newSigner(2), newSigner(99)
	src := project(t, a)
	o1 := mustIssue(t, src, a, OpAdd, b, RoleAdmin)
	o2 := mustIssue(t, src, a, OpAdd, newSigner(3), RoleWriter)
	l := New(src.Project())
	mustApply(t, l, src.Since(nil)[0])
	for i := range maxPending {
		o := Op{Kind: OpAdd, Role: RoleReader, Subject: key(40, i).Public(), Author: stranger.Public(), Prev: []Hash{{byte(i), byte(i >> 8), 9}}, Lamport: 1}
		o.sign(stranger, l.Project())
		if err := l.Apply(o); !errors.Is(err, ErrUnknownAuthor) {
			t.Fatalf("Apply(stranger's orphan) = %v, want ErrUnknownAuthor", err)
		}
	}
	mustApply(t, l, o2) // arrives before o1: buffered.
	mustApply(t, l, o1)
	if l.Pending() != 0 || l.Len() != 3 {
		t.Errorf("pending=%d len=%d, want 0 and 3", l.Pending(), l.Len())
	}
}
