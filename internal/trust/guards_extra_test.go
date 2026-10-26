package trust

import (
	"errors"
	"strings"
	"testing"
)

// TestSizeGuardsComeBeforeParsing: an input over a size limit is refused as
// too large, without being parsed; one at the limit is parsed (and fails on
// its content, not its size).
func TestSizeGuardsComeBeforeParsing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limit  int
		decode func([]byte) error
	}{
		{"op", MaxOpSize, func(b []byte) error { _, err := Decode(b); return err }},
		{"attestation", MaxAttestationSize, func(b []byte) error { _, err := DecodeAttestation(b); return err }},
		{"sync message", MaxMessage, func(b []byte) error { _, _, _, err := decodeMessage(b); return err }},
	} {
		at, over := tc.decode(make([]byte, tc.limit)), tc.decode(make([]byte, tc.limit+1))
		if !errors.Is(at, ErrMalformed) || !errors.Is(over, ErrMalformed) {
			t.Errorf("%s: at the limit %v, over it %v; want ErrMalformed both", tc.name, at, over)
			continue
		}
		if strings.Contains(at.Error(), "too large") {
			t.Errorf("%s: %d bytes are refused as too large, want them parsed: %v", tc.name, tc.limit, at)
		}
		if !strings.Contains(over.Error(), "too large") {
			t.Errorf("%s: %d bytes are refused with %q, want a size refusal", tc.name, tc.limit+1, over)
		}
	}
}

// TestSuspendedMemberRoleChanges: a suspended member cannot be made admin
// (resume it first), but every other change of its role is allowed.
func TestSuspendedMemberRoleChanges(t *testing.T) {
	a, b := newSigner(1), newSigner(2)
	l := project(t, a)
	mustIssue(t, l, a, OpAdd, b, RoleWriter)
	mustIssue(t, l, a, OpSuspend, b, 0)
	if _, err := l.Issue(a, OpSetRole, b.Public(), RoleAdmin, ""); !errors.Is(err, ErrSuspended) {
		t.Errorf("SetRole(suspended, admin) = %v, want ErrSuspended", err)
	}
	for _, role := range []Role{RoleReader, RoleWriter} {
		if _, err := l.Issue(a, OpSetRole, b.Public(), role, ""); err != nil {
			t.Errorf("SetRole(suspended, %v) = %v, want nil", role, err)
		}
	}
	if !l.Suspended(b.Public()) {
		t.Error("a role change lifted the suspension")
	}
	if _, err := l.Issue(a, OpRemove, b.Public(), 0, ""); err != nil {
		t.Errorf("Remove(suspended) = %v, want nil", err)
	}
}
