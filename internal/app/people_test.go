package app

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

func mustKey(t *testing.T, peer string) ed25519.PublicKey {
	t.Helper()
	key, err := p2p.KeyFromID(transport.PeerID(peer))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// person returns the person named name in the people list of r.
func person(t *testing.T, r *Runtime, name string) Person {
	t.Helper()
	for _, p := range r.Session.Members() {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no %q among %+v", name, r.Session.Members())
	return Person{}
}

func TestPeople(t *testing.T) {
	r, _ := startRuntime(t, nil, false)
	devB1, devB2, devC := newDevice(t), newDevice(t), newDevice(t)
	if _, err := r.wiring.Issue(trust.OpAdd, devB1.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	bob := person(t, r, "Bob")
	var bobPerson trust.Hash
	for _, m := range r.inv.currentList().Members() {
		if m.Pub.Equal(devB1.Public()) {
			bobPerson = m.Person
		}
	}
	if _, err := r.wiring.IssueWith(trust.Op{Kind: trust.OpAdd, Subject: devB2.Public(), Role: trust.RoleWriter, Person: bobPerson}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.wiring.Issue(trust.OpAdd, devC.Public(), trust.RoleReader, "Cy"); err != nil {
		t.Fatal(err)
	}

	people := r.Session.Members()
	if len(people) != 3 || !people[0].Self || people[0].Role != "admin" || people[1].Name != "Bob" || people[2].Name != "Cy" {
		t.Fatalf("Members() = %+v, want the creator, Bob and Cy", people)
	}
	if bob = people[1]; len(bob.Devices) != 2 || bob.Suspended || bob.Role != "writer" || bob.Self {
		t.Errorf("Bob = %+v, want two devices, writer, connected", bob)
	}

	// Disconnecting a device of Bob disconnects the person, on every device.
	before := time.Now().Add(-2 * time.Second) // the clock of the file system is coarse
	if err := r.Session.Suspend(bob.Devices[0].Peer); err != nil {
		t.Fatalf("Suspend() = %v", err)
	}
	bob = person(t, r, "Bob")
	if !bob.Suspended || !bob.Devices[0].Suspended || !bob.Devices[1].Suspended || bob.Role != "writer" {
		t.Errorf("Bob after Suspend = %+v, want both devices suspended and the same role", bob)
	}
	if bob.SuspendedByPeer != r.Session.LocalPeer() {
		t.Errorf("SuspendedByPeer = %q, want this device %q", bob.SuspendedByPeer, r.Session.LocalPeer())
	}
	if bob.SuspendedAt.Before(before) || bob.SuspendedAt.After(time.Now().Add(2*time.Second)) {
		t.Errorf("SuspendedAt = %v, want the time it was received (about now)", bob.SuspendedAt)
	}
	if err := r.Session.Suspend(bob.ID); err != nil {
		t.Errorf("Suspend(already suspended) = %v, want nothing to do", err)
	}

	if err := r.Session.Resume(bob.ID); err != nil {
		t.Fatalf("Resume() = %v", err)
	}
	if bob = person(t, r, "Bob"); bob.Suspended || !bob.SuspendedAt.IsZero() || bob.SuspendedByPeer != "" {
		t.Errorf("Bob after Resume = %+v, want connected", bob)
	}

	if err := r.Session.SetRole(bob.ID, "admin"); err != nil {
		t.Fatalf("SetRole() = %v", err)
	}
	if bob = person(t, r, "Bob"); bob.Role != "admin" {
		t.Errorf("Bob's role = %q, want admin", bob.Role)
	}
	for _, d := range bob.Devices {
		if got := r.inv.currentList().Role(mustKey(t, d.Peer)); got != trust.RoleAdmin {
			t.Errorf("device %s has role %v, want admin on every device", d.Peer, got)
		}
	}

	if err := r.Session.Remove(person(t, r, "Cy").ID); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if got := r.Session.Members(); len(got) != 2 {
		t.Errorf("Members() after Remove = %+v, want 2 people", got)
	}

	// What cannot be done.
	if err := r.Session.Suspend(r.Session.LocalPeer()); !errors.Is(err, errSelf) {
		t.Errorf("Suspend(self) = %v, want errSelf", err)
	}
	if err := r.Session.Suspend("nobody"); !errors.Is(err, errUnknownPerson) {
		t.Errorf("Suspend(unknown) = %v, want errUnknownPerson", err)
	}
	if err := r.Session.SetRole(bob.ID, "boss"); err == nil {
		t.Error("SetRole(unknown role) = nil")
	}
}

func TestSiblingCannotResumeItsOwnPerson(t *testing.T) {
	r, _ := startRuntime(t, nil, false)
	sibling := newDevice(t)
	self := r.Session.Members()[0]
	var person trust.Hash
	for _, m := range r.inv.currentList().Members() {
		if m.Pub.Equal(r.inv.dev.Public()) {
			person = m.Person
		}
	}
	if _, err := r.wiring.IssueWith(trust.Op{Kind: trust.OpAdd, Subject: sibling.Public(), Role: trust.RoleAdmin, Person: person}); err != nil {
		t.Fatal(err)
	}
	// The sibling is suspended on its own, so that this device is not.
	if _, err := r.wiring.Issue(trust.OpSuspend, sibling.Public(), 0, ""); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{self.ID, self.Devices[0].Peer, string(p2p.IDFromKey(sibling.Public()))} {
		if err := r.Session.Resume(who); !errors.Is(err, errSelf) {
			t.Errorf("Resume(%q) = %v, want errSelf: a person cannot lift its own suspension", who, err)
		}
	}
	if !r.inv.currentList().Suspended(sibling.Public()) {
		t.Error("the sibling was resumed")
	}
}

func TestSuspendedAtIsNoted(t *testing.T) {
	r, d := startRuntime(t, nil, false)
	b := newDevice(t)
	if _, err := r.wiring.Issue(trust.OpAdd, b.Public(), trust.RoleWriter, "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := r.Session.Suspend(string(p2p.IDFromKey(b.Public()))); err != nil {
		t.Fatal(err)
	}
	at := person(t, r, "Bob").SuspendedAt
	if at.IsZero() {
		t.Fatal("SuspendedAt is zero")
	}

	// What the file system says about the op file is not the time of arrival.
	var op trust.Hash
	for _, m := range r.inv.currentList().Members() {
		if m.Pub.Equal(b.Public()) {
			op = m.SuspendedOp
		}
	}
	if err := r.inv.store.root.Chtimes(op.String()+opExt, time.Unix(1e9, 0), time.Unix(1e9, 0)); err != nil {
		t.Fatal(err)
	}
	if got := person(t, r, "Bob").SuspendedAt; !got.Equal(at) {
		t.Errorf("SuspendedAt = %v after the file was touched, want %v", got, at)
	}

	// It survives a restart.
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Start(t.Context(), d.home, Options{Project: d.proj, RendezvousURL: NoCodes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	if got := person(t, again, "Bob").SuspendedAt; !got.Equal(at) {
		t.Errorf("SuspendedAt = %v after a restart, want %v", got, at)
	}
}

func TestPeopleWithoutProject(t *testing.T) {
	s := NewSession(noNet{})
	if got := s.Members(); len(got) != 0 {
		t.Errorf("Members() = %+v, want none", got)
	}
	for name, err := range map[string]error{
		"Suspend": s.Suspend("x"), "Resume": s.Resume("x"), "SetRole": s.SetRole("x", "reader"), "Remove": s.Remove("x"),
	} {
		if !errors.Is(err, errNoProject) {
			t.Errorf("%s() without project = %v, want errNoProject", name, err)
		}
	}
}
