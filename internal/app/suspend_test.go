package app

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// queued returns the envelopes waiting for the bridge of m, once its wiring
// is idle. Reading the queue directly, unlike Recv with a canceled context,
// cannot miss an envelope.
func queued(m *member) []bridge.Envelope {
	synctest.Wait()
	var out []bridge.Envelope
	for {
		select {
		case e := <-m.w.out:
			out = append(out, e)
		default:
			return out
		}
	}
}

// find returns the envelopes of kind k about peer p.
func find(es []bridge.Envelope, k bridge.Kind, p transport.PeerID) (out []bridge.Envelope) {
	for _, e := range es {
		if e.Kind == k && (p == "" || e.Peer == p) {
			out = append(out, e)
		}
	}
	return out
}

// lastLink reports whether the last PeerUp or PeerDown about p in es is a
// PeerUp, and whether there is one.
func lastLink(es []bridge.Envelope, p transport.PeerID) (up, seen bool) {
	for _, e := range es {
		if e.Peer == p && (e.Kind == bridge.PeerUp || e.Kind == bridge.PeerDown) {
			up, seen = e.Kind == bridge.PeerUp, true
		}
	}
	return up, seen
}

// trio returns an admin A and two writers, B and C, all connected to A.
func trio(t *testing.T, net *memnet.Network) (a, b, c *member, devB, devC *identity.Device) {
	t.Helper()
	devA, devB, devC := newDevice(t), newDevice(t), newDevice(t)
	listA := newProject(t, devA)
	for _, d := range []*identity.Device{devB, devC} {
		if _, err := listA.Issue(devA, trust.OpAdd, d.Public(), trust.RoleWriter, "w"); err != nil {
			t.Fatal(err)
		}
	}
	a, b, c = join(t, net, devA, listA), join(t, net, devB, fork(t, listA)), join(t, net, devC, fork(t, listA))
	net.Link(a.id, b.id)
	net.Link(a.id, c.id)
	for _, m := range []*member{a, b, c} {
		queued(m)
	}
	return a, b, c, devB, devC
}

func TestSuspend(t *testing.T) {
	t.Run("ACL-13 the restricted link carries the member list only", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			net := memnet.New(1, memnet.Faults{})
			devA, devB, devD := newDevice(t), newDevice(t), newDevice(t)
			listA := newProject(t, devA)
			if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), trust.RoleAdmin, "b"); err != nil {
				t.Fatal(err)
			}
			listB := fork(t, listA)
			opts := WiringOptions{TickEvery: time.Second}
			a, b := join(t, net, devA, listA, opts), join(t, net, devB, listB, opts)

			// A suspends B while B, which does not know it yet, signs an op of
			// its own: the op that a suspended admin backdates (ACL-12).
			dOp, err := listB.Issue(devB, trust.OpAdd, devD.Public(), trust.RoleWriter, "d")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := listA.Issue(devA, trust.OpSuspend, devB.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			net.Link(a.id, b.id)
			time.Sleep(5 * time.Second)

			// The member list went both ways over the restricted link.
			if !listA.Has(dOp.Hash()) {
				t.Error("A never received the op B signed: the restricted link must carry the member list from B")
			}
			if !listB.Suspended(devB.Public()) {
				t.Error("B never learned that it is suspended: the restricted link must carry the member list to B")
			}
			if listA.Role(devD.Public()) != 0 || listB.Role(devD.Public()) != 0 {
				t.Error("the op B signed without having seen its suspension stands")
			}

			// A never announced B to its bridge. B saw A, then lost it.
			aEnvs := queued(a)
			if up, seen := lastLink(aEnvs, b.id); seen {
				t.Errorf("A's bridge got a link event for the suspended B (up = %v)", up)
			}
			bEnvs := queued(b)
			if up, seen := lastLink(bEnvs, a.id); !seen || up {
				t.Errorf("B's bridge: last link event about A = (up %v, seen %v), want PeerDown", up, seen)
			}
			if got := find(bEnvs, bridge.Suspended, a.id); len(got) != 1 || string(got[0].Payload) != "\x01" {
				t.Errorf("B got %+v, want one Suspended envelope by A", got)
			}
			if got := b.w.Repo().Peers(); len(got) != 0 {
				t.Errorf("B's Peers() = %v, want none", got)
			}
			if err := b.w.Repo().Send(t.Context(), a.id, []byte("x")); !errors.Is(err, transport.ErrUnknownPeer) {
				t.Errorf("Send by the suspended B = %v, want ErrUnknownPeer", err)
			}

			// Neither the repo channel nor the attestation channel passes, from B or to B.
			att, err := trust.NewAttestation(devB, listA.Project(), "doc", []trust.Hash{{1}}, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, frame := range [][]byte{{wireRepo, 'x'}, append([]byte{wireAttest}, trust.EncodeAttestation(att)...)} {
				if err := b.node.Send(t.Context(), a.id, frame); err != nil {
					t.Fatal(err)
				}
			}
			if got := queued(a); len(got) != 0 {
				t.Errorf("A's bridge got %+v from the suspended B, want nothing", got)
			}
			if got := a.w.Attestations(); len(got) != 0 {
				t.Errorf("A holds %v from the suspended B, want none", got)
			}
			if err := a.w.Repo().Send(t.Context(), b.id, []byte("secret")); !errors.Is(err, transport.ErrUnknownPeer) {
				t.Errorf("Send to the suspended B = %v, want ErrUnknownPeer", err)
			}
			if err := attester(t, a).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h")}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Second)
			if got := b.w.Attestations(); len(got) != 0 {
				t.Errorf("the suspended B holds %v, want none", got)
			}
			if got := find(queued(b), bridge.Attested, ""); len(got) != 0 {
				t.Errorf("B's bridge got %+v, want no attestation", got)
			}

			// A resume brings it all back, and the backdated op stays dead.
			if _, err := a.w.Issue(trust.OpResume, devB.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			time.Sleep(2 * time.Second)
			if up, seen := lastLink(queued(a), b.id); !seen || !up {
				t.Errorf("A's bridge: last link event about B = (up %v, seen %v), want PeerUp", up, seen)
			}
			if err := b.w.Repo().Send(t.Context(), a.id, []byte("back")); err != nil {
				t.Fatalf("Send after the resume = %v", err)
			}
			if ev := next(t, a.w.Repo()); ev.Kind != transport.Message || string(ev.Payload) != "back" {
				t.Errorf("A got %v %q, want the message of the resumed B", ev.Kind, ev.Payload)
			}
			if listA.Role(devD.Public()) != 0 || listA.Suspended(devB.Public()) {
				t.Error("after the resume the backdated op stands or B is still suspended")
			}
		})
	})

	t.Run("Revoked is emitted at the suspension", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a, b, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
			if err := attester(t, b).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h")}); err != nil {
				t.Fatal(err)
			}
			queued(a)
			if len(a.w.Attestations()) != 1 {
				t.Fatalf("A holds %v, want the attestation of B", a.w.Attestations())
			}
			if _, err := a.w.Issue(trust.OpSuspend, b.dev.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			es := queued(a)
			if got := find(es, bridge.Revoked, b.id); len(got) != 1 {
				t.Errorf("A's bridge got %d Revoked about B, want 1 (envelopes %v)", len(got), es)
			}
			if got := a.w.Attestations(); len(got) != 0 {
				t.Errorf("A still holds %v, want the attestations of B pruned", got)
			}
			if up, seen := lastLink(es, b.id); !seen || up {
				t.Errorf("A's bridge: last link event about B = (up %v, seen %v), want PeerDown", up, seen)
			}
		})
	})

	t.Run("ACL-02 the heads of a suspended device wait for the resume", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a, b, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
			if _, err := a.w.Issue(trust.OpSuspend, b.dev.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			if !b.list.Suspended(b.dev.Public()) {
				t.Fatal("B did not learn its suspension")
			}
			heads := [][bridge.HashSize]byte{head("offline edit")}
			if err := attester(t, b).Attest(t.Context(), "doc", heads); !errors.Is(err, trust.ErrSuspended) {
				t.Fatalf("Attest by a suspended device = %v, want ErrSuspended", err)
			}
			synctest.Wait()
			if got := a.w.Attestations(); len(got) != 0 {
				t.Fatalf("A holds %v, want nothing from the suspended B", got)
			}
			// Another change of the list, while B is still suspended, must not use the heads up.
			if _, err := a.w.Issue(trust.OpAdd, newDevice(t).Public(), trust.RoleReader, "r"); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()

			if _, err := a.w.Issue(trust.OpResume, b.dev.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			got := a.w.Attestations()
			if len(got) != 1 || got[0].Doc != "doc" || string(got[0].Signer) != string(b.dev.Public()) ||
				len(got[0].Heads) != 1 || got[0].Heads[0] != trust.Hash(heads[0]) {
				t.Fatalf("A holds %v, want the attestation of B for the heads it kept", got)
			}
			es := queued(b)
			if s := find(es, bridge.Suspended, a.id); len(s) != 2 || string(s[0].Payload) != "\x01" || string(s[1].Payload) != "\x00" {
				t.Errorf("B got %+v, want Suspended on then off, by A", s)
			}
			// A device that is not suspended keeps nothing: the heads are signed at once.
			if err := attester(t, b).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("next")}); err != nil {
				t.Errorf("Attest after the resume = %v", err)
			}
		})
	})

	t.Run("ACL-11 a person is suspended on all of its devices", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			net := memnet.New(1, memnet.Faults{})
			devA, devB1, devB2 := newDevice(t), newDevice(t), newDevice(t)
			listA := newProject(t, devA)
			if _, err := listA.Issue(devA, trust.OpAdd, devB1.Public(), trust.RoleWriter, "bob"); err != nil {
				t.Fatal(err)
			}
			var person trust.Hash
			for _, m := range listA.Members() {
				if m.Pub.Equal(devB1.Public()) {
					person = m.Person
				}
			}
			if _, err := listA.IssueWith(devA, trust.Op{Kind: trust.OpAdd, Subject: devB2.Public(), Role: trust.RoleWriter, Person: person}); err != nil {
				t.Fatal(err)
			}
			a := join(t, net, devA, listA)
			b1, b2 := join(t, net, devB1, fork(t, listA)), join(t, net, devB2, fork(t, listA))
			net.Link(a.id, b1.id)
			net.Link(a.id, b2.id)
			queued(a)

			ops, err := a.w.IssueBatch([]trust.Op{
				{Kind: trust.OpSuspend, Subject: devB1.Public()},
				{Kind: trust.OpSuspend, Subject: devB2.Public()},
			})
			if err != nil || len(ops) != 2 {
				t.Fatalf("IssueBatch() = %d ops, %v; want 2", len(ops), err)
			}
			es := queued(a)
			for _, m := range []*member{b1, b2} {
				if up, seen := lastLink(es, m.id); !seen || up {
					t.Errorf("A's bridge: last link event about %s = (up %v, seen %v), want PeerDown", m.id, up, seen)
				}
				if !m.list.Suspended(m.dev.Public()) {
					t.Errorf("%s did not learn that it is suspended", m.id)
				}
			}
			third := newDevice(t)
			_, err = a.w.IssueWith(trust.Op{Kind: trust.OpAdd, Subject: third.Public(), Role: trust.RoleWriter, Person: person})
			if !errors.Is(err, trust.ErrSuspended) {
				t.Errorf("adding a device to a suspended person = %v, want ErrSuspended", err)
			}
		})
	})

	t.Run("ACL-14 a batch that would seal the list is refused whole", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			net := memnet.New(1, memnet.Faults{})
			devA, devB, devC := newDevice(t), newDevice(t), newDevice(t)
			listA := newProject(t, devA)
			for _, d := range []*identity.Device{devB, devC} {
				if _, err := listA.Issue(devA, trust.OpAdd, d.Public(), trust.RoleWriter, "w"); err != nil {
					t.Fatal(err)
				}
			}
			const sealAt = 1024 // ops of a sealed list.
			for listA.Len() < sealAt-2 {
				if _, err := listA.Issue(devA, trust.OpSetRole, devA.Public(), trust.RoleAdmin, "a name that takes some room"); err != nil {
					t.Fatal(err)
				}
			}
			a := join(t, net, devA, listA)
			queued(a)
			_, err := a.w.IssueBatch([]trust.Op{
				{Kind: trust.OpSuspend, Subject: devB.Public()},
				{Kind: trust.OpSuspend, Subject: devC.Public()},
			})
			if !errors.Is(err, trust.ErrTooManyOps) {
				t.Fatalf("IssueBatch(sealing) = %v, want ErrTooManyOps", err)
			}
			if listA.Len() != sealAt-2 || listA.Suspended(devB.Public()) || listA.Suspended(devC.Public()) {
				t.Errorf("a refused batch left Len() = %d, B suspended = %v, C suspended = %v", listA.Len(), listA.Suspended(devB.Public()), listA.Suspended(devC.Public()))
			}
		})
	})

	t.Run("the member roles reach the webview", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a, b, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
			queued(a)
			queued(b)
			devC := newDevice(t)
			if _, err := a.w.Issue(trust.OpAdd, devC.Public(), trust.RoleReader, "c"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.w.Issue(trust.OpSuspend, b.dev.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			var last []bridge.MemberRole
			rolesEnvs := find(queued(a), bridge.Roles, "")
			if len(rolesEnvs) < 2 {
				t.Fatalf("A's bridge got %d Roles envelopes, want one per change of roles", len(rolesEnvs))
			}
			for _, e := range rolesEnvs {
				if e.Peer != a.id {
					t.Errorf("Roles about %s, want the local device", e.Peer)
				}
				rs, err := bridge.DecodeRoles(e.Payload)
				if err != nil {
					t.Fatalf("DecodeRoles() = %v", err)
				}
				last = rs
			}
			want := []bridge.MemberRole{
				{Peer: a.id, Role: uint8(trust.RoleAdmin)},
				{Peer: b.id, Role: uint8(trust.RoleWriter), Suspended: true},
				{Peer: p2p.IDFromKey(devC.Public()), Role: uint8(trust.RoleReader)},
			}
			if len(last) != len(want) {
				t.Fatalf("Roles = %+v, want %+v", last, want)
			}
			for i := range want {
				if last[i] != want[i] {
					t.Errorf("Roles[%d] = %+v, want %+v", i, last[i], want[i])
				}
			}

			// A webview that attaches later is told the same, and B's is told it is suspended.
			var notices []bridge.Envelope
			for _, m := range []*member{a, b} {
				notices = append(notices, attester(t, m).Notices()...)
			}
			if len(find(notices, bridge.Roles, "")) != 2 {
				t.Errorf("Notices() = %+v, want the Roles of both devices", notices)
			}
			if got := find(attester(t, b).Notices(), bridge.Suspended, a.id); len(got) != 1 || string(got[0].Payload) != "\x01" {
				t.Errorf("B's Notices() = %+v, want Suspended by A", attester(t, b).Notices())
			}
			if got := find(attester(t, a).Notices(), bridge.Suspended, ""); len(got) != 0 {
				t.Errorf("A's Notices() = %+v, want no Suspended", got)
			}
		})
	})

	t.Run("replaying the attestation of a suspended writer is refused", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a, b, c, devB, _ := trio(t, memnet.New(1, memnet.Faults{}))
			if err := attester(t, b).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h")}); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			old := a.w.Attestations()
			if len(old) != 1 {
				t.Fatalf("A holds %v, want the attestation of B", old)
			}
			if _, err := a.w.Issue(trust.OpSuspend, devB.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			queued(a)
			// C, who is not suspended, replays it to A.
			if err := c.node.Send(t.Context(), a.id, append([]byte{wireAttest}, trust.EncodeAttestation(old[0])...)); err != nil {
				t.Fatal(err)
			}
			if got := find(queued(a), bridge.Attested, ""); len(got) != 0 {
				t.Errorf("A's bridge got %+v, want the replayed attestation refused", got)
			}
			if got := a.w.Attestations(); len(got) != 0 {
				t.Errorf("A holds %v, want the replay refused", got)
			}
		})
	})

	t.Run("a flood over the restricted link is bounded by the budget of Sync", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			net := memnet.New(1, memnet.Faults{})
			devA, devB := newDevice(t), newDevice(t)
			listA := newProject(t, devA)
			if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
				t.Fatal(err)
			}
			if _, err := listA.Issue(devA, trust.OpSuspend, devB.Public(), 0, ""); err != nil {
				t.Fatal(err)
			}
			a := join(t, net, devA, listA)
			raw := net.Join(p2p.IDFromKey(devB.Public())) // a peer that answers to nobody
			net.Link(a.id, raw.Local())
			synctest.Wait()
			for range 50 {
				if err := raw.Send(t.Context(), a.id, []byte{wireTrust, 1, 0, 0}); err != nil { // a Have of nothing
					t.Fatal(err)
				}
				if err := raw.Send(t.Context(), a.id, []byte{wireRepo, 'x'}); err != nil {
					t.Fatal(err)
				}
			}
			synctest.Wait()
			var ops int
			for {
				ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
				ev, err := raw.Recv(ctx)
				cancel()
				if err != nil {
					break
				}
				if ev.Kind == transport.Message && len(ev.Payload) > 1 && ev.Payload[0] == wireTrust && ev.Payload[1] == 2 {
					ops++
				}
			}
			if ops < 1 || ops > 2 {
				t.Errorf("A answered the flood with %d Ops messages, want 1 or 2 (the budget of a tick)", ops)
			}
			if got := queued(a); len(get(got, bridge.Message)) != 0 {
				t.Errorf("A's bridge got %+v from the flood, want nothing", got)
			}
		})
	})
}

// get returns the envelopes of kind k.
func get(es []bridge.Envelope, k bridge.Kind) []bridge.Envelope { return find(es, k, "") }
