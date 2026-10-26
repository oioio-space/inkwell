package trust

import (
	"errors"
	"testing"
)

// ACL-08: demoting a writer to a reader takes effect at once for what the list
// verifies: its attestations, old and new, stop being accepted; promoting it
// back brings them back.
func TestACL08DemotionRevokesAttestations(t *testing.T) {
	a, w := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	att, err := NewAttestation(w, l.Project(), "doc", []Hash{{1}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(att); err != nil {
		t.Fatalf("Verify(writer) = %v", err)
	}
	mustIssue(t, l, a, OpSetRole, w, RoleReader)
	if err := l.Verify(att); !errors.Is(err, ErrNotWriter) {
		t.Errorf("Verify(demoted writer, old attestation) = %v, want ErrNotWriter", err)
	}
	fresh, err := NewAttestation(w, l.Project(), "doc", []Hash{{2}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(fresh); !errors.Is(err, ErrNotWriter) {
		t.Errorf("Verify(demoted writer, new attestation) = %v, want ErrNotWriter", err)
	}
	if got := l.Role(w.Public()); got != RoleReader {
		t.Errorf("Role = %v, want reader", got)
	}
	mustIssue(t, l, a, OpSetRole, w, RoleWriter)
	if err := l.Verify(fresh); err != nil {
		t.Errorf("Verify(promoted back) = %v, want nil", err)
	}
}

// ACL-09: a frozen list takes no more arrivals or departures, and writing goes
// on: the members it holds keep their roles and their attestations verify.
func TestACL09FrozenListKeepsVerifying(t *testing.T) {
	a, w := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, w, RoleWriter)
	if err := chain(t, l, a, maxOps); !errors.Is(err, ErrTooManyOps) {
		t.Fatalf("filling the list: %v", err)
	}
	if !l.Sealed() {
		t.Fatal("the list is not frozen")
	}
	if _, err := l.Issue(a, OpAdd, newSigner(3).Public(), RoleReader, ""); !errors.Is(err, ErrTooManyOps) {
		t.Errorf("Issue(arrival) on a frozen list = %v, want ErrTooManyOps", err)
	}
	if _, err := l.Issue(a, OpRemove, w.Public(), 0, ""); !errors.Is(err, ErrTooManyOps) {
		t.Errorf("Issue(departure) on a frozen list = %v, want ErrTooManyOps", err)
	}
	att, err := NewAttestation(w, l.Project(), "doc", []Hash{{1}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(att); err != nil {
		t.Errorf("Verify(writer) on a frozen list = %v, want nil: writing goes on", err)
	}
	if got := l.Role(w.Public()); got != RoleWriter {
		t.Errorf("Role = %v, want writer", got)
	}
}
