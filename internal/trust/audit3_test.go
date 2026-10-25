package trust

import "testing"

// A leaked key of an admin that already left honestly signs a second, backdated
// exit. It must not erase the career of the honest one (rule 3b, exits).
func TestAudit3LeakedExitErasesHistory(t *testing.T) {
	a, b, y, j, w, m := newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(5), newSigner(6)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	addY := mustIssue(t, l, a, OpAdd, y, RoleAdmin)
	mustIssue(t, l, y, OpAdd, j, RoleAdmin)  // y's junior admin.
	mustIssue(t, l, j, OpAdd, w, RoleWriter) // j's writer.
	mustIssue(t, l, y, OpAdd, m, RoleAdmin)
	mustIssue(t, l, y, OpRemove, m, 0)                  // y revokes m.
	mustIssue(t, l, b, OpAdd, newSigner(7), RoleReader) // others build on top.
	mustIssue(t, l, y, OpRemove, y, 0)                  // honest exit.
	mustApply(t, l, craft(t, l, y, OpSetRole, y, RoleReader, "", h(addY)))
	wantRole(t, l, j, RoleAdmin)
	wantRole(t, l, w, RoleWriter)
	wantRole(t, l, m, 0)
}

// A removed admin's backdated exit is concurrent to the cut that removed it,
// so it does not stand and must not kill the admin's earlier ops, which would
// resurrect the key it had removed.
func TestAudit3RemovedAdminExitResurrects(t *testing.T) {
	a, b, y, m, evil := newSigner(1), newSigner(2), newSigner(3), newSigner(6), newSigner(66)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	addY := mustIssue(t, l, a, OpAdd, y, RoleAdmin)
	addM := mustIssue(t, l, a, OpAdd, m, RoleAdmin) // m is junior to y.
	mustIssue(t, l, y, OpRemove, m, 0)
	mustApply(t, l, craft(t, l, m, OpAdd, evil, RoleAdmin, "", h(addM))) // m backdates: dead.
	wantRole(t, l, m, 0)
	wantRole(t, l, evil, 0)
	mustIssue(t, l, a, OpRemove, y, 0)
	wantRole(t, l, y, 0)
	mustApply(t, l, craft(t, l, y, OpRemove, y, 0, "", h(addY))) // concurrent to a's cut: does not stand.
	wantRole(t, l, m, 0)
	wantRole(t, l, evil, 0)
}

// A current admin that backdates an exit to its own grant does not erase what
// it granted and others built on.
func TestAudit3CurrentAdminBackdatedExit(t *testing.T) {
	a, b, y, j, w := newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(8)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	addY := mustIssue(t, l, a, OpAdd, y, RoleAdmin)
	mustIssue(t, l, y, OpAdd, j, RoleAdmin)
	mustIssue(t, l, j, OpAdd, w, RoleWriter)
	mustApply(t, l, craft(t, l, y, OpRemove, y, 0, "", h(addY)))
	wantRole(t, l, j, RoleAdmin)
	wantRole(t, l, w, RoleWriter)
}

// A leaked ex-creator key signs a backdated exit after the honest hand-over.
func TestAudit3LeakedExCreatorWipes(t *testing.T) {
	a, b, c, w1, w2 := newSigner(1), newSigner(2), newSigner(3), newSigner(4), newSigner(5)
	l := project(t, a)
	addB := mustIssue(t, l, a, OpAdd, b, RoleAdmin)
	mustIssue(t, l, a, OpAdd, c, RoleAdmin)
	mustIssue(t, l, a, OpAdd, w1, RoleWriter)
	mustIssue(t, l, c, OpAdd, w2, RoleWriter)
	mustIssue(t, l, b, OpAdd, newSigner(9), RoleReader) // b builds on top.
	mustIssue(t, l, a, OpRemove, a, 0)                  // honest hand-over.
	n0 := len(l.Members())
	mustApply(t, l, craft(t, l, a, OpRemove, a, 0, "", h(addB))) // leaked key, backdated exit.
	if n := len(l.Members()); n != n0 {
		t.Errorf("members = %d after the leaked exit, want %d", n, n0)
	}
	wantRole(t, l, b, RoleAdmin)
	wantRole(t, l, c, RoleAdmin)
	wantRole(t, l, w1, RoleWriter)
	wantRole(t, l, w2, RoleWriter)
}
