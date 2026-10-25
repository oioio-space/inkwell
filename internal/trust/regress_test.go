package trust

import (
	"crypto/ed25519"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"
)

// craft signs an op by s with explicit parents, bypassing Issue and its
// guards, as a hostile or stale replica would.
func craft(t testing.TB, l *List, s Signer, k OpKind, subj Signer, r Role, name string, prev []Hash) Op {
	t.Helper()
	var lam uint64
	for _, p := range prev {
		lam = max(lam, l.ops[p].op.Lamport+1)
	}
	o := Op{Kind: k, Subject: subj.Public(), Author: s.Public(), Role: r, Name: name, Prev: prev, Lamport: lam}
	o.sign(s, l.Project())
	return o
}

// forge crafts an op on top of the whole frontier of l and applies it.
func forge(t testing.TB, l *List, s Signer, k OpKind, subj Signer, r Role) Op {
	t.Helper()
	o := craft(t, l, s, k, subj, r, "", l.Frontier())
	if err := l.Apply(o); err != nil {
		t.Fatalf("Apply(forged kind %d) = %v", k, err)
	}
	return o
}

func mustApply(t testing.TB, l *List, o Op) {
	t.Helper()
	if err := l.Apply(o); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
}

func h(o Op) []Hash { return []Hash{o.Hash()} }

// key returns a deterministic signer distinct for every (tag, i).
func key(tag byte, i int) *testSigner {
	var seed [32]byte
	seed[0], seed[1], seed[2] = tag, byte(i), byte(i>>8)
	return &testSigner{ed25519.NewKeyFromSeed(seed[:])}
}

func wantAdmins(t testing.TB, l *List, want int) {
	t.Helper()
	n := 0
	for _, m := range l.Members() {
		if m.Role == RoleAdmin {
			n++
		}
	}
	if n != want {
		t.Errorf("admins = %d, want %d (members %+v)", n, want, l.Members())
	}
}

// PoC ExAdminStickyVoid: a removed admin antedates ops to undo others, twice.
func TestExAdminBackdatedOpsKilled(t *testing.T) {
	a, m, w := newSigner(1), newSigner(2), newSigner(3)
	l := project(t, a)
	addM := mustIssue(t, l, a, OpAdd, m, RoleAdmin)
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	mustIssue(t, l, a, OpRemove, m, 0)
	wantRole(t, l, w, RoleWriter)

	// Round 1: m antedates Remove(w) on the state where it was still admin.
	mustApply(t, l, craft(t, l, m, OpRemove, w, 0, "", h(addM)))
	wantRole(t, l, w, RoleWriter)

	// Round 2: a re-adds w having seen that op; m repeats from a longer chain.
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	x := craft(t, l, m, OpSetRole, m, RoleAdmin, "x", h(addM))
	mustApply(t, l, x)
	mustApply(t, l, craft(t, l, m, OpRemove, w, 0, "", h(x)))
	wantRole(t, l, w, RoleWriter)
	wantRole(t, l, m, 0)
}

// PoC ExAdminResurrects and Resurrect2: a removed admin cannot cut the creator.
func TestExAdminCannotResurrect(t *testing.T) {
	for name, zFirst := range map[string]bool{"zBeforeM": true, "zAfterM": false} {
		t.Run(name, func(t *testing.T) {
			a, m, z := newSigner(1), newSigner(2), newSigner(3)
			l := project(t, a)
			var addM Op
			if zFirst {
				mustIssue(t, l, a, OpAdd, z, RoleWriter)
				addM = mustIssue(t, l, a, OpAdd, m, RoleAdmin)
			} else {
				addM = mustIssue(t, l, a, OpAdd, m, RoleAdmin)
				mustIssue(t, l, a, OpAdd, z, RoleWriter)
			}
			mustIssue(t, l, a, OpRemove, z, 0)
			mustIssue(t, l, a, OpRemove, m, 0)
			mustApply(t, l, craft(t, l, m, OpRemove, a, 0, "", h(addM)))
			wantRole(t, l, a, RoleAdmin)
			wantRole(t, l, m, 0)
			wantRole(t, l, z, 0)
			if n := len(l.Members()); n != 1 {
				t.Errorf("members = %d, want exactly the creator", n)
			}
		})
	}
}

// PoC TwoDevicesEscapeRevocation: two revoked devices cannot rescue each other.
func TestRevokedDevicesCannotRescueEachOther(t *testing.T) {
	a, m1, m2, x := newSigner(1), newSigner(2), newSigner(3), newSigner(9)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, m1, RoleAdmin)
	base := mustIssue(t, l, a, OpAdd, m2, RoleAdmin)
	mustIssue(t, l, a, OpRemove, m1, 0)
	mustIssue(t, l, a, OpRemove, m2, 0)
	for _, s := range []Signer{m1, m2} {
		mustApply(t, l, craft(t, l, s, OpRemove, a, 0, "", h(base)))
	}
	wantRole(t, l, a, RoleAdmin)
	wantRole(t, l, m1, 0)
	wantRole(t, l, m2, 0)
	if _, err := l.Issue(m1, OpAdd, x.Public(), RoleAdmin, ""); !errors.Is(err, ErrNotAdmin) {
		t.Errorf("Issue by revoked device = %v, want ErrNotAdmin", err)
	}
}

// PoC Ring: a→b→c→a concurrent removals.
func TestRemovalRing(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	base := mustIssue(t, l, a, OpAdd, c, RoleAdmin)
	mustApply(t, l, craft(t, l, a, OpRemove, b, 0, "", h(base)))
	mustApply(t, l, craft(t, l, b, OpRemove, c, 0, "", h(base)))
	mustApply(t, l, craft(t, l, c, OpRemove, a, 0, "", h(base)))
	wantRole(t, l, a, RoleAdmin)
	wantRole(t, l, b, 0)
	wantRole(t, l, c, RoleAdmin)
}

// PoC DemotedAdminRepromotes: Lamport inflation does not undo a demotion.
func TestDemotedAdminCannotRepromote(t *testing.T) {
	a, m, x := newSigner(1), newSigner(2), newSigner(9)
	l := project(t, a)
	addM := mustIssue(t, l, a, OpAdd, m, RoleAdmin)
	mustIssue(t, l, a, OpSetRole, m, RoleReader)
	prev := h(addM)
	for range 3 {
		o := craft(t, l, m, OpSetRole, m, RoleAdmin, "", prev)
		mustApply(t, l, o)
		prev = h(o)
	}
	mustApply(t, l, craft(t, l, m, OpAdd, x, RoleAdmin, "", prev))
	wantRole(t, l, m, RoleReader)
	wantRole(t, l, x, 0)
	if _, ranked := l.current().rank[string(x.Public())]; ranked {
		t.Error("x got a rank from a dead grant")
	}
}

func TestJuniorCannotCutSenior(t *testing.T) {
	a, b, c := newSigner(1), newSigner(2), newSigner(3)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpAdd, c, RoleAdmin)
	if _, err := l.Issue(c, OpRemove, b.Public(), 0, ""); !errors.Is(err, ErrOutranked) {
		t.Errorf("Issue(junior removes senior) = %v, want ErrOutranked", err)
	}
	forge(t, l, c, OpRemove, b, 0)
	forge(t, l, c, OpSetRole, b, RoleReader)
	wantRole(t, l, b, RoleAdmin)
	wantRole(t, l, c, RoleAdmin)
	// The senior may cut the junior.
	mustIssue(t, l, b, OpRemove, c, 0)
	wantRole(t, l, c, 0)
}

func TestCreatorHandsOverToAnotherAdmin(t *testing.T) {
	for name, leave := range map[string]func(*testing.T, *List, Signer){
		"remove": func(t *testing.T, l *List, a Signer) { mustIssue(t, l, a, OpRemove, a, 0) },
		"demote": func(t *testing.T, l *List, a Signer) { mustIssue(t, l, a, OpSetRole, a, RoleWriter) },
	} {
		t.Run(name, func(t *testing.T) {
			a, b, c, d := newSigner(1), newSigner(2), newSigner(3), newSigner(4)
			l := project(t, a)
			mustIssue(t, l, a, OpAdd, b, RoleAdmin)
			leave(t, l, a)
			wantRole(t, l, b, RoleAdmin)
			if name == "remove" {
				wantRole(t, l, a, 0)
			} else {
				wantRole(t, l, a, RoleWriter)
			}
			mustIssue(t, l, b, OpAdd, c, RoleWriter) // b carries on.

			// The former creator can no longer act, nor be re-granted.
			if _, err := l.Issue(a, OpAdd, d.Public(), RoleReader, ""); !errors.Is(err, ErrNotAdmin) {
				t.Errorf("Issue by former creator = %v, want ErrNotAdmin", err)
			}
			forge(t, l, a, OpAdd, d, RoleReader)
			wantRole(t, l, d, 0)
			if _, err := l.Issue(b, OpSetRole, a.Public(), RoleAdmin, ""); !errors.Is(err, ErrOutranked) {
				t.Errorf("Issue(re-grant creator) = %v, want ErrOutranked", err)
			}
			forge(t, l, b, OpSetRole, a, RoleAdmin)
			forge(t, l, a, OpSetRole, a, RoleAdmin) // dead: a is no longer admin.
			if got := l.Role(a.Public()); got == RoleAdmin {
				t.Errorf("former creator is admin again")
			}
			wantAdmins(t, l, 1)
			if m := l.Members(); name == "remove" && (m[0].Role != RoleAdmin || slices.Equal(m[0].Pub, a.Public())) {
				t.Errorf("Members()[0] = %+v, want b first", m[0])
			}
		})
	}
}

func TestCreatorCannotLeaveAlone(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	for _, tc := range []struct {
		k OpKind
		r Role
	}{{OpRemove, 0}, {OpSetRole, RoleReader}, {OpSetRole, RoleWriter}, {OpAdd, RoleReader}} {
		if _, err := l.Issue(a, tc.k, a.Public(), tc.r, ""); !errors.Is(err, ErrLastAdmin) {
			t.Errorf("Issue(kind %d role %v) = %v, want ErrLastAdmin", tc.k, tc.r, err)
		}
	}
	// A non-admin is not another admin.
	mustIssue(t, l, a, OpAdd, b, RoleWriter)
	if _, err := l.Issue(a, OpRemove, a.Public(), 0, ""); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("Issue(remove self next to a writer) = %v, want ErrLastAdmin", err)
	}
	forge(t, l, a, OpRemove, a, 0)
	forge(t, l, a, OpSetRole, a, RoleReader)
	wantRole(t, l, a, RoleAdmin)

	// The other admin must be in the op's own past: a concurrent grant is not enough.
	la := project(t, a)
	lb := fork(t, la)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin)
	forge(t, lb, a, OpRemove, a, 0)
	merge(t, la, lb)
	for _, r := range []*List{la, lb} {
		wantRole(t, r, a, RoleAdmin)
		wantRole(t, r, b, RoleAdmin)
	}
}

func TestOthersCannotRemoveCreator(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	for _, tc := range []struct {
		k OpKind
		r Role
	}{{OpRemove, 0}, {OpSetRole, RoleReader}, {OpAdd, RoleWriter}, {OpSetRole, RoleAdmin}} {
		if _, err := l.Issue(b, tc.k, a.Public(), tc.r, ""); !errors.Is(err, ErrOutranked) {
			t.Errorf("Issue(b, kind %d on creator) = %v, want ErrOutranked", tc.k, err)
		}
	}
	forge(t, l, b, OpRemove, a, 0)
	forge(t, l, b, OpSetRole, a, RoleReader)
	wantRole(t, l, a, RoleAdmin)
	wantRole(t, l, b, RoleAdmin)
}

// The creator and the last other admin may leave concurrently; the project
// then has no admin: membership is frozen and fails closed.
func TestConcurrentDeparturesMayFreezeMembership(t *testing.T) {
	a, b, x := newSigner(1), newSigner(2), newSigner(9)
	check := func(t *testing.T, l *List) {
		t.Helper()
		wantAdmins(t, l, 0)
		if _, err := l.Issue(a, OpAdd, x.Public(), RoleReader, ""); !errors.Is(err, ErrNotAdmin) {
			t.Errorf("Issue by creator = %v, want ErrNotAdmin", err)
		}
		if _, err := l.Issue(b, OpAdd, x.Public(), RoleReader, ""); !errors.Is(err, ErrNotAdmin) {
			t.Errorf("Issue by b = %v, want ErrNotAdmin", err)
		}
	}
	t.Run("creatorAndAdmin", func(t *testing.T) {
		la := project(t, a)
		mustIssue(t, la, a, OpAdd, b, RoleAdmin)
		lb := fork(t, la)
		mustIssue(t, la, a, OpRemove, a, 0)
		mustIssue(t, lb, b, OpRemove, b, 0)
		merge(t, la, lb)
		check(t, la)
		check(t, lb)
	})
	t.Run("creatorAgainstItself", func(t *testing.T) {
		// The creator's own concurrent Remove(b) is killed by its exit: b stays.
		l := project(t, a)
		base := mustIssue(t, l, a, OpAdd, b, RoleAdmin)
		mustApply(t, l, craft(t, l, a, OpRemove, b, 0, "", h(base)))
		mustApply(t, l, craft(t, l, a, OpRemove, a, 0, "", h(base)))
		wantRole(t, l, b, RoleAdmin)
		wantRole(t, l, a, 0)
	})
}

func TestRepromotionOnlyBySenior(t *testing.T) {
	a, b, c, x := newSigner(1), newSigner(2), newSigner(3), newSigner(9)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpAdd, c, RoleAdmin)
	mustIssue(t, l, a, OpSetRole, b, RoleReader)
	wantRole(t, l, b, RoleReader)

	if _, err := l.Issue(c, OpSetRole, b.Public(), RoleAdmin, ""); !errors.Is(err, ErrOutranked) {
		t.Errorf("Issue(junior re-promotes) = %v, want ErrOutranked", err)
	}
	forge(t, l, c, OpSetRole, b, RoleAdmin)
	wantRole(t, l, b, RoleReader)

	mustIssue(t, l, a, OpSetRole, b, RoleAdmin)
	wantRole(t, l, b, RoleAdmin)
	mustIssue(t, l, b, OpAdd, x, RoleWriter) // b acts again.
	wantRole(t, l, x, RoleWriter)
}

// The p2panda semantics: what a revoked admin wrote without seeing its
// revocation stays dead after it is re-admitted.
func TestConcurrentOpsStayKilledAfterReadd(t *testing.T) {
	a, b, x, y := newSigner(1), newSigner(2), newSigner(8), newSigner(9)
	la := project(t, a)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin)
	lb := fork(t, la)
	mustIssue(t, lb, b, OpAdd, x, RoleWriter) // concurrent to the removal.
	mustIssue(t, la, a, OpRemove, b, 0)
	mustIssue(t, la, a, OpAdd, b, RoleAdmin) // re-admitted.
	merge(t, la, lb)
	for _, l := range []*List{la, lb} {
		wantRole(t, l, x, 0)
		wantRole(t, l, b, RoleAdmin)
	}
	mustIssue(t, la, b, OpAdd, y, RoleWriter) // what b writes now counts.
	wantRole(t, la, y, RoleWriter)
	wantRole(t, la, x, 0)
}

// buildCascade makes k admins S_i that each demote C_i, while the creator
// removes S_0 and each C_i removes S_{i+1}, so every verdict depends on the
// previous one.
func buildCascade(t testing.TB, k int) (l *List, s, c []*testSigner) {
	t.Helper()
	a := newSigner(1)
	l = project(t, a)
	prev := l.Frontier()
	for i := range k {
		s, c = append(s, key(10, i)), append(c, key(11, i))
		for _, x := range []*testSigner{s[i], c[i]} {
			o := craft(t, l, a, OpAdd, x, RoleAdmin, "", prev)
			mustApply(t, l, o)
			prev = h(o)
		}
	}
	base := prev
	d := make([]Op, k)
	for i := range k {
		d[i] = craft(t, l, s[i], OpSetRole, c[i], RoleReader, "", base)
		mustApply(t, l, d[i])
	}
	mustApply(t, l, craft(t, l, a, OpRemove, s[0], 0, "", base))
	for i := 1; i < k; i++ {
		mustApply(t, l, craft(t, l, c[i-1], OpRemove, s[i], 0, "", h(d[i-1])))
	}
	return l, s, c
}

func TestCascadeSingleSweep(t *testing.T) {
	const k = 250
	l, s, c := buildCascade(t, k)
	t0 := time.Now()
	fold(l.ops)
	if d := time.Since(t0); d > 100*time.Millisecond {
		t.Errorf("fold of %d ops took %v, want < 100ms", l.Len(), d)
	}
	for i := range k {
		wantRole(t, l, s[i], 0)
		wantRole(t, l, c[i], RoleAdmin)
	}
}

func TestNonAdminOpsRefused(t *testing.T) {
	a, r, w, x := newSigner(1), newSigner(2), newSigner(3), newSigner(9)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, r, RoleReader)
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	n, heads := l.Len(), l.Frontier()
	for _, s := range []Signer{r, w} {
		for _, o := range []Op{
			craft(t, l, s, OpSetRole, s, RoleAdmin, "", heads),
			craft(t, l, s, OpAdd, x, RoleReader, "", heads),
			craft(t, l, s, OpRemove, a, 0, "", heads),
		} {
			if err := l.Apply(o); !errors.Is(err, ErrUnknownAuthor) {
				t.Errorf("Apply(op by %v) = %v, want ErrUnknownAuthor", s.Public()[:2], err)
			}
		}
	}
	if l.Len() != n || !slices.Equal(l.Frontier(), heads) || l.Pending() != 0 {
		t.Errorf("list changed: len %d→%d, heads %v→%v", n, l.Len(), heads, l.Frontier())
	}
}

func TestIssueCapsPrev(t *testing.T) {
	b, x := newSigner(2), newSigner(9)
	for name, issuer := range map[string]Signer{"admin": b, "creator": newSigner(1)} {
		t.Run(name, func(t *testing.T) {
			a := newSigner(1)
			l := project(t, a)
			g := l.Frontier()
			p1 := craft(t, l, a, OpAdd, key(30, 999), RoleReader, "", g)
			addB := craft(t, l, a, OpAdd, b, RoleAdmin, "", g)
			mustApply(t, l, p1)
			mustApply(t, l, addB)
			for i := range 100 { // 100 concurrent heads, all newer than addB.
				mustApply(t, l, craft(t, l, a, OpAdd, key(30, i), RoleReader, "", h(p1)))
			}
			if n := len(l.Frontier()); n != 101 {
				t.Fatalf("heads = %d, want 101", n)
			}
			op, err := l.Issue(issuer, OpAdd, x.Public(), RoleWriter, "")
			if err != nil {
				t.Fatalf("Issue() with 101 heads = %v", err)
			}
			if len(op.Prev) != MaxPrev {
				t.Errorf("len(Prev) = %d, want %d", len(op.Prev), MaxPrev)
			}
			if name == "admin" && !slices.Contains(op.Prev, addB.Hash()) {
				t.Error("Prev does not cover the issuer's grant")
			}
			wantRole(t, l, x, RoleWriter) // the op stands.
		})
	}
}

// flood returns n chained backdated ops by s, the first following from.
func flood(id ProjectID, s Signer, from Op, n int) []Op {
	ops := make([]Op, 0, n)
	prev := from
	for i := range n {
		o := Op{Kind: OpSetRole, Role: RoleAdmin, Subject: s.Public(), Author: s.Public(), Prev: h(prev), Lamport: prev.Lamport + 1, Name: strconv.Itoa(i)}
		o.sign(s, id)
		ops = append(ops, o)
		prev = o
	}
	return ops
}

func TestSealIsOrderIndependent(t *testing.T) {
	a, m := newSigner(1), newSigner(2)
	src := project(t, a)
	addM := mustIssue(t, src, a, OpAdd, m, RoleAdmin)
	rm := mustIssue(t, src, a, OpRemove, m, 0)
	base := src.Since(nil)[:2] // genesis, addM.
	spam := flood(src.Project(), m, addM, maxOps-1)

	sealed := func(order ...[]Op) *List {
		l := New(src.Project())
		for _, ops := range order {
			_, _ = l.ApplyAll(ops) // the overflow is refused.
		}
		return l
	}
	l1 := sealed(base, spam, []Op{rm}) // the flood first, the revocation last.
	l2 := sealed(base, []Op{rm}, spam) // the revocation first.
	_, _ = l1.ApplyAll(l2.Since(nil))
	_, _ = l2.ApplyAll(l1.Since(nil))

	att, err := NewAttestation(a, src.Project(), "doc", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Sealing freezes membership only: reads and attestations carry on. Under
	// an ex-admin flood the two replicas may differ on what they stored.
	for name, tc := range map[string]struct {
		l     *List
		wantM Role
	}{"floodFirst": {l1, RoleAdmin}, "revocationFirst": {l2, 0}} {
		t.Run(name, func(t *testing.T) {
			l := tc.l
			if !l.Sealed() || l.Len() != maxOps {
				t.Errorf("Sealed() = %v, Len() = %d, want sealed at %d", l.Sealed(), l.Len(), maxOps)
			}
			wantRole(t, l, a, RoleAdmin)
			wantRole(t, l, m, tc.wantM)
			if _, err := l.Issue(a, OpAdd, m.Public(), RoleReader, ""); !errors.Is(err, ErrTooManyOps) {
				t.Errorf("Issue = %v, want ErrTooManyOps", err)
			}
			if err := l.Verify(att); err != nil {
				t.Errorf("Verify = %v, want nil: editing must go on", err)
			}
			if err := l.Apply(craft(t, l, a, OpAdd, m, RoleReader, "", l.Frontier())); !errors.Is(err, ErrTooManyOps) {
				t.Errorf("Apply = %v, want ErrTooManyOps", err)
			}
		})
	}
}

func TestPendingFullRefusesAndExpires(t *testing.T) {
	a := newSigner(1)
	l := project(t, a)
	orphan := func(i int) Op {
		o := Op{Kind: OpAdd, Role: RoleReader, Subject: key(40, i).Public(), Author: a.Public(), Prev: []Hash{{byte(i), byte(i >> 8), 1}}, Lamport: 1}
		o.sign(a, l.Project())
		return o
	}
	for i := range maxPending {
		if err := l.Apply(orphan(i)); err != nil {
			t.Fatalf("Apply(orphan %d) = %v", i, err)
		}
	}
	for range 2 { // refused, not evicting.
		if err := l.Apply(orphan(maxPending)); !errors.Is(err, ErrTooManyOps) {
			t.Errorf("Apply(orphan beyond the buffer) = %v, want ErrTooManyOps", err)
		}
	}
	if err := l.Apply(orphan(0)); err != nil {
		t.Errorf("Apply(buffered duplicate) = %v, want nil", err)
	}
	if l.Pending() != maxPending {
		t.Fatalf("Pending() = %d, want %d", l.Pending(), maxPending)
	}

	s := NewSync(l)
	s.Tick()
	if l.Pending() != maxPending {
		t.Errorf("Pending() after one tick = %d, want %d", l.Pending(), maxPending)
	}
	s.Tick()
	if l.Pending() != 0 {
		t.Errorf("Pending() after two ticks = %d, want 0", l.Pending())
	}
	if err := l.Apply(orphan(maxPending)); err != nil || l.Pending() != 1 {
		t.Errorf("Apply after expiry = %v, pending %d", err, l.Pending())
	}
}
