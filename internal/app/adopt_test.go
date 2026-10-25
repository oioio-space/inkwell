package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/trust"
)

func TestCutProject(t *testing.T) {
	id := trust.ProjectID{1, 2, 3}
	const addr = "/ip4/10.0.0.2/tcp/1/p2p/ID"
	tests := []struct {
		name     string
		text     string
		wantRest string
		wantID   *trust.ProjectID
		wantErr  bool
	}{
		{name: "none", text: addr, wantRest: addr},
		{name: "with project", text: withProject(addr, id), wantRest: addr, wantID: &id},
		{name: "project first", text: projectPrefix + id.String() + " " + addr, wantRest: addr, wantID: &id},
		{name: "twice", text: withProject(withProject(addr, id), id), wantErr: true},
		{name: "not hex", text: addr + " " + projectPrefix + strings.Repeat("zz", 32), wantErr: true},
		{name: "short", text: addr + " " + projectPrefix + "abcd", wantErr: true},
		{name: "too long", text: strings.Repeat("a", maxInvitationLen+1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest, got, err := cutProject(tt.text)
			if (err != nil) != tt.wantErr {
				t.Fatalf("cutProject(%q) error = %v, wantErr %v", tt.text, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if rest != tt.wantRest || (got == nil) != (tt.wantID == nil) || (got != nil && *got != *tt.wantID) {
				t.Errorf("cutProject(%q) = %q, %v; want %q, %v", tt.text, rest, got, tt.wantRest, tt.wantID)
			}
		})
	}
}

func FuzzCutProject(f *testing.F) {
	f.Add("/ip4/1.2.3.4/tcp/1/p2p/X project:" + strings.Repeat("ab", 32))
	f.Add("project:")
	f.Fuzz(func(t *testing.T, text string) {
		rest, _, err := cutProject(text)
		if err != nil {
			if !errors.Is(err, errBadInvitation) {
				t.Fatalf("cutProject(%q) error %v is not errBadInvitation", text, err)
			}
			return
		}
		for f := range strings.FieldsSeq(rest) {
			if strings.HasPrefix(f, projectPrefix) {
				t.Fatalf("cutProject(%q) left a project in %q", text, rest)
			}
		}
	})
}

func TestHostedInvitationNamesTheProject(t *testing.T) {
	dev := newDevice(t)
	list := newProject(t, dev)
	adm := NewAdmission(list)
	s := NewSession(fakeNet{addrs: []string{"/ip4/10.0.0.1/tcp/1/p2p/ID"}}, WithAdmission(adm))
	text, err := s.Host()
	if err != nil {
		t.Fatal(err)
	}
	rest, id, err := cutProject(text)
	if err != nil || id == nil || *id != list.Project() {
		t.Fatalf("cutProject(Host()) = %q, %v, %v; want the project %v", rest, id, err, list.Project())
	}
	if _, _, err := parseInvitation(rest); err != nil {
		t.Errorf("addresses of the invitation do not parse: %v", err)
	}
}

type fakeNet struct{ addrs []string }

func (fakeNet) Local() transport.PeerID                                   { return "me" }
func (n fakeNet) Addrs() []string                                         { return n.addrs }
func (fakeNet) Connect(context.Context, transport.PeerID, []string) error { return nil }
func (fakeNet) Peers() []transport.PeerID                                 { return nil }

// A guest joins the host's project: it takes the host's member list, becomes
// a writer in it, and both sides accept each other's attestations.
func TestAdoptJoinsHostsProject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devH, devG := newDevice(t), newDevice(t)
		h := join(t, net, devH, newProject(t, devH))
		h.adm.OpenInvitation()
		guestList := newProject(t, devG)
		ownGenesis := guestList.Since(nil)[0].Hash()
		kept, forgotten := []trust.Hash{ownGenesis}, []trust.Hash(nil) // what the store holds
		reset := 0
		g := join(t, net, devG, guestList, WiringOptions{
			Save:  func(op trust.Op) error { kept = append(kept, op.Hash()); return nil },
			Reset: func() error { reset++; forgotten, kept = kept, nil; return nil },
		})

		if err := g.w.Adopt(h.list.Project()); err != nil {
			t.Fatalf("Adopt() = %v", err)
		}
		if reset != 1 {
			t.Errorf("Reset called %d times, want 1", reset)
		}
		// The guest's webview vouches for its heads before it is a member: kept.
		if err := attester(t, g).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("g1")}); err != nil {
			t.Fatal(err)
		}
		g.adm.Invite(h.id, nil)
		net.Link(h.id, g.id)
		synctest.Wait()

		got := g.w.list()
		if got.Project() != h.list.Project() {
			t.Fatalf("guest project = %v, want the host's %v", got.Project(), h.list.Project())
		}
		if r := got.Role(devG.Public()); r != trust.RoleWriter {
			t.Errorf("guest role = %v, want writer", r)
		}
		if r := got.Role(devH.Public()); r != trust.RoleAdmin {
			t.Errorf("host role = %v, want admin", r)
		}
		if slices.Contains(kept, ownGenesis) || !slices.Contains(forgotten, ownGenesis) {
			t.Error("the guest kept the genesis of its previous project")
		}
		if len(kept) != got.Len() {
			t.Errorf("saved %d ops, want the %d of the adopted list", len(kept), got.Len())
		}

		// The kept heads were signed once the guest became a writer, and the host
		// accepted them.
		if atts := h.w.Attestations(); len(atts) != 1 || atts[0].Doc != "doc" || string(atts[0].Signer) != string(devG.Public()) {
			t.Errorf("host holds %v, want the guest's attestation", atts)
		}
		// And the guest takes the host's.
		if err := attester(t, h).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h1")}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if n := attestedBy(g, h.id); n != 1 {
			t.Errorf("guest holds %d attestations of the host, want 1", n)
		}
	})
}

// Until the first op of the project arrives, the host stands for its admin, so
// its attestations are not lost on the way.
func TestAdoptTakesTheHostAttestationsBeforeTheList(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devH, devG := newDevice(t), newDevice(t)
		h := join(t, net, devH, newProject(t, devH))
		g := join(t, net, devG, newProject(t, devG))
		if err := g.w.Adopt(h.list.Project()); err != nil {
			t.Fatal(err)
		}
		if err := attester(t, h).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h1")}); err != nil {
			t.Fatal(err)
		}
		att := h.w.Attestations()[0]

		g.adm.Invite(h.id, nil)
		if err := g.w.offerAttest(att, h.id); err != nil {
			t.Errorf("offerAttest(host, empty list) = %v, want nil", err)
		}
		other := newDevice(t)
		forged, err := trust.NewAttestation(other, h.list.Project(), "doc", att.Heads, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := g.w.offerAttest(forged, h.id); !errors.Is(err, trust.ErrNotWriter) {
			t.Errorf("offerAttest(stranger, empty list) = %v, want ErrNotWriter", err)
		}
	})
}

func TestAdoptRefusesWhenNotAlone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devB, devC := newDevice(t), newDevice(t), newDevice(t)
		a := join(t, net, devA, newProject(t, devA))
		if _, err := a.w.Issue(trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
			t.Fatal(err)
		}
		other := newProject(t, devC)
		if err := a.w.Adopt(other.Project()); !errors.Is(err, ErrNotAlone) {
			t.Errorf("Adopt(another project) = %v, want ErrNotAlone", err)
		}
		if err := a.w.Adopt(a.list.Project()); err != nil {
			t.Errorf("Adopt(own project) = %v, want nil", err)
		}
	})
}

type fakeAdopter struct {
	got []trust.ProjectID
	err error
}

func (f *fakeAdopter) Adopt(id trust.ProjectID) error {
	f.got = append(f.got, id)
	return f.err
}

func TestJoinAdoptsTheInvitedProject(t *testing.T) {
	id := trust.ProjectID{9}
	host := NewAdmission(newProject(t, newDevice(t)))
	guestAdm := NewAdmission(newProject(t, newDevice(t)))
	text := withProject("/ip4/10.0.0.2/tcp/1/p2p/HOST", id)

	ad := &fakeAdopter{}
	s := NewSession(fakeNet{}, WithAdmission(guestAdm), WithAdopter(ad))
	if peer, err := s.Join(t.Context(), text); err != nil || peer != "HOST" {
		t.Fatalf("Join() = %q, %v; want HOST", peer, err)
	}
	if len(ad.got) != 1 || ad.got[0] != id {
		t.Errorf("adopted %v, want [%v]", ad.got, id)
	}
	if !guestAdm.Admit("HOST") {
		t.Error("the host is not admitted after Join")
	}

	// An invitation without a project leaves the guest in its own.
	ad = &fakeAdopter{}
	s = NewSession(fakeNet{}, WithAdmission(host), WithAdopter(ad))
	if _, err := s.Join(t.Context(), "/ip4/10.0.0.2/tcp/1/p2p/OLD"); err != nil || len(ad.got) != 0 {
		t.Errorf("Join(no project) = %v, adopted %v; want nil, nothing", err, ad.got)
	}

	// A refusal stops the join before the host is admitted.
	refused := NewAdmission(newProject(t, newDevice(t)))
	s = NewSession(fakeNet{}, WithAdmission(refused), WithAdopter(&fakeAdopter{err: ErrNotAlone}))
	if _, err := s.Join(t.Context(), text); !errors.Is(err, ErrNotAlone) {
		t.Errorf("Join() = %v, want ErrNotAlone", err)
	}
	if refused.Admit("HOST") {
		t.Error("the host is admitted although the project was refused")
	}
}
