package trust

import (
	"bytes"
	"testing"
)

func TestSmallOrderOpsBeforeMutation(t *testing.T) {
	admin := newSigner(42)
	l := project(t, admin)
	neutral := make([]byte, 32)
	neutral[0] = 1
	before := Encode(l.ops[l.frontier()[0]].op)
	if _, err := l.Issue(admin, OpAdd, neutral, RoleWriter, "bad"); err == nil {
		t.Fatal("admitted small-order subject")
	}
	add := Op{Kind: OpAdd, Author: admin.Public(), Subject: neutral, Role: RoleWriter, Prev: l.frontier(), Lamport: 1}
	add.sign(admin, l.Project())
	if err := l.Apply(add); err == nil {
		t.Fatal("accepted signed admission of small-order subject")
	}
	if _, err := Decode(Encode(add)); err == nil {
		t.Fatal("decoded small-order member op")
	}
	g, _ := Genesis(admin, "bad", "")
	g.Author, g.Subject = neutral, neutral
	g.Sig = [64]byte{1}
	empty := New(ProjectID(g.Hash()))
	if err := empty.Apply(g); err == nil || len(empty.ops) != 0 {
		t.Fatal("accepted forged genesis")
	}
	op := Op{Kind: OpAdd, Author: neutral, Subject: admin.Public(), Role: RoleAdmin, Prev: []Hash{l.frontier()[0]}, Lamport: 1, Sig: [64]byte{1}}
	if err := l.Apply(op); err == nil {
		t.Fatal("accepted forged member op")
	}
	if len(l.ops) != 1 || l.Pending() != 0 || !bytes.Equal(before, Encode(l.ops[l.frontier()[0]].op)) {
		t.Fatal("mutated list")
	}
	a := Attestation{Project: l.Project(), Doc: "doc", Signer: neutral, Sig: [64]byte{1}}
	if err := l.Verify(a); err == nil {
		t.Fatal("accepted forged attestation")
	}
	if _, err := DecodeAttestation(EncodeAttestation(a)); err == nil {
		t.Fatal("decoded small-order attestation")
	}
}
