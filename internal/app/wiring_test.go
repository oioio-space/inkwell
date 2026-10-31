package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// Channel prefixes of the wire, as package mux writes them.
const (
	wireRepo   = 0x01
	wireTrust  = 0x02
	wireAttest = 0x03
)

// dropNode is a memnet node that, like a p2p node, can drop a peer.
type dropNode struct {
	*memnet.Node
	net   *memnet.Network
	drops chan transport.PeerID
}

func (d *dropNode) Drop(who transport.PeerID) {
	select {
	case d.drops <- who:
	default:
	}
	d.net.Partition(d.Local(), who)
}

// member is one device of a test project.
type member struct {
	dev  *identity.Device
	id   transport.PeerID
	list *trust.List
	adm  *Admission
	node *dropNode
	w    *Wiring
}

func newDevice(t *testing.T) *identity.Device {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	dev, err := identity.LoadOrCreate(root)
	if err != nil {
		t.Fatal(err)
	}
	return dev
}

// join puts dev on the network and wires it to list.
func join(t *testing.T, net *memnet.Network, dev *identity.Device, list *trust.List, opts ...WiringOptions) *member {
	t.Helper()
	id := p2p.IDFromKey(dev.Public())
	m := &member{dev: dev, id: id, list: list, adm: NewAdmission(list)}
	m.node = &dropNode{Node: net.Join(id), net: net, drops: make(chan transport.PeerID, 8)}
	var o WiringOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	m.w = Wire(t.Context(), m.node, dev, list, m.adm, o)
	t.Cleanup(func() { _ = m.w.Close() })
	return m
}

// newProject returns the list of a project created by admin.
func newProject(t *testing.T, admin trust.Signer) *trust.List {
	t.Helper()
	g, id := trust.Genesis(admin, "book", "")
	l := trust.New(id)
	if err := l.Apply(g); err != nil {
		t.Fatal(err)
	}
	return l
}

// fork returns another replica of l.
func fork(t *testing.T, l *trust.List) *trust.List {
	t.Helper()
	f := trust.New(l.Project())
	if _, err := f.ApplyAll(l.Since(nil)); err != nil {
		t.Fatal(err)
	}
	return f
}

// next returns the next event of the bridge transport.
func next(t *testing.T, tr transport.Transport) transport.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	ev, err := tr.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv(%s): %v", tr.Local(), err)
	}
	return ev
}

// nothing fails if the bridge transport has a queued event.
func nothing(t *testing.T, tr transport.Transport) {
	t.Helper()
	synctest.Wait()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if ev, err := tr.Recv(ctx); err == nil {
		t.Errorf("Recv(%s) = %v from %s %q, want nothing", tr.Local(), ev.Kind, ev.Peer, ev.Payload)
	}
}

func TestNonMemberNeverReachesBridge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		admin, stranger := newDevice(t), newDevice(t)
		a := join(t, net, admin, newProject(t, admin))
		x := net.Join(p2p.IDFromKey(stranger.Public()))
		net.Link(a.id, x.Local())

		for _, frame := range [][]byte{{wireRepo, 'x'}, {wireTrust, 1, 2, 3}, {wireAttest, 1, 2, 3}, {wireRepo, 'y'}} {
			if err := x.Send(t.Context(), a.id, frame); err != nil {
				t.Fatal(err)
			}
		}
		nothing(t, a.w.Repo())
		if got := a.w.Repo().Peers(); len(got) != 0 {
			t.Errorf("Peers() = %v, want none: the stranger is not a member", got)
		}
		if err := a.w.Repo().Send(t.Context(), x.Local(), []byte("secret")); !errors.Is(err, transport.ErrUnknownPeer) {
			t.Errorf("Send(stranger) = %v, want ErrUnknownPeer", err)
		}

		// Once admitted, the same peer gets through.
		if _, err := a.w.Issue(trust.OpAdd, stranger.Public(), trust.RoleWriter, "x"); err != nil {
			t.Fatal(err)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.PeerUp || ev.Peer != x.Local() {
			t.Errorf("after Add: %v from %s, want PeerUp of the new member", ev.Kind, ev.Peer)
		}
		if err := x.Send(t.Context(), a.id, []byte{wireRepo, 'z'}); err != nil {
			t.Fatal(err)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.Message || string(ev.Payload) != "z" {
			t.Errorf("member frame = %v %q, want Message z", ev.Kind, ev.Payload)
		}
	})
}

func TestRevokeDropsAndRefuses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devB := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		a := join(t, net, devA, listA)
		b := join(t, net, devB, fork(t, listA))
		net.Link(a.id, b.id)
		if _, err := a.w.Issue(trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
			t.Fatal(err)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.PeerUp || ev.Peer != b.id {
			t.Fatalf("A got %v from %s, want PeerUp of B", ev.Kind, ev.Peer)
		}
		synctest.Wait() // B has handled the PeerUp of A
		if err := b.w.Repo().Send(t.Context(), a.id, []byte("hello")); err != nil {
			t.Fatal(err)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.Message || string(ev.Payload) != "hello" {
			t.Fatalf("A got %v %q, want hello", ev.Kind, ev.Payload)
		}

		if _, err := a.w.Issue(trust.OpRemove, devB.Public(), 0, ""); err != nil {
			t.Fatal(err)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.PeerDown || ev.Peer != b.id {
			t.Errorf("after Remove: %v from %s, want PeerDown of B", ev.Kind, ev.Peer)
		}
		if got := <-a.node.drops; got != b.id {
			t.Errorf("dropped %s, want B", got)
		}
		if a.adm.Admit(b.id) {
			t.Error("Admit(revoked) = true, want false")
		}
		if err := a.w.Repo().Send(t.Context(), b.id, []byte("late")); !errors.Is(err, transport.ErrUnknownPeer) {
			t.Errorf("Send(revoked) = %v, want ErrUnknownPeer", err)
		}

		// B reconnects anyway (memnet has no gate): its frames still go nowhere.
		net.Link(a.id, b.id)
		if err := b.node.Send(t.Context(), a.id, []byte{wireRepo, 'x'}); err != nil {
			t.Fatal(err)
		}
		nothing(t, a.w.Repo())
	})
}

func TestAttestationsBeforeRepoFrames(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devB := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
			t.Fatal(err)
		}

		// Frame i follows attestation i+1 on the same stream. B checks, as it
		// queues the frame for the bridge, that it already holds the attestation.
		var b *member
		var late, seen atomic.Int32
		observe := func(ev transport.Event) {
			if ev.Kind != transport.Message {
				return
			}
			seen.Add(1)
			if got := b.w.att.all(); len(got) != 1 || got[0].Seq != uint64(ev.Payload[0])+1 {
				late.Add(1)
			}
		}
		a := join(t, net, devA, listA)
		b = join(t, net, devB, fork(t, listA), WiringOptions{onEmit: observe})
		net.Link(a.id, b.id)
		synctest.Wait()
		if next(t, a.w.Repo()).Kind != transport.PeerUp || next(t, b.w.Repo()).Kind != transport.PeerUp {
			t.Fatal("no PeerUp")
		}

		const frames = 50
		for i := range frames {
			head := sha256.Sum256([]byte{byte(i)})
			att, err := trust.NewAttestation(devA, listA.Project(), "doc", []trust.Hash{head}, uint64(i)+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.w.Attest(att); err != nil {
				t.Fatal(err)
			}
			if err := a.w.Repo().Send(t.Context(), b.id, []byte{byte(i)}); err != nil {
				t.Fatal(err)
			}
			if ev := next(t, b.w.Repo()); ev.Kind != transport.Message || ev.Payload[0] != byte(i) {
				t.Fatalf("frame %d = %v %v", i, ev.Kind, ev.Payload)
			}
		}
		if got, n := late.Load(), seen.Load(); got != 0 || n != frames {
			t.Errorf("%d of %d frames reached the bridge before their attestation (%d seen)", got, frames, n)
		}
	})
}

func TestAttestationsGossip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devB := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), trust.RoleReader, "b"); err != nil {
			t.Fatal(err)
		}
		a, b := join(t, net, devA, listA), join(t, net, devB, fork(t, listA))
		head := []trust.Hash{sha256.Sum256([]byte("h"))}
		mk := func(seq uint64) trust.Attestation {
			att, err := trust.NewAttestation(devA, listA.Project(), "doc", head, seq)
			if err != nil {
				t.Fatal(err)
			}
			return att
		}
		for _, seq := range []uint64{5, 3, 5} { // an older or repeated seq is ignored
			if err := a.w.Attest(mk(seq)); err != nil {
				t.Fatal(err)
			}
		}
		if got := a.w.Attestations(); len(got) != 1 || got[0].Seq != 5 {
			t.Errorf("A holds %v, want the single attestation 5", got)
		}

		// A reader's attestation is refused: only writers vouch for changes.
		bad, err := trust.NewAttestation(devB, listA.Project(), "doc", head, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.w.Attest(bad); !errors.Is(err, trust.ErrNotWriter) {
			t.Errorf("Attest(reader) = %v, want ErrNotWriter", err)
		}

		// B gets the latest one when it connects.
		net.Link(a.id, b.id)
		synctest.Wait()
		if got := b.w.Attestations(); len(got) != 1 || got[0].Seq != 5 {
			t.Errorf("B holds %v after connecting, want attestation 5", got)
		}
	})
}

func TestAdmit(t *testing.T) {
	admin, member, stranger := newDevice(t), newDevice(t), newDevice(t)
	list := newProject(t, admin)
	if _, err := list.Issue(admin, trust.OpAdd, member.Public(), trust.RoleReader, "m"); err != nil {
		t.Fatal(err)
	}
	adm := NewAdmission(list)
	id := func(d *identity.Device) transport.PeerID { return p2p.IDFromKey(d.Public()) }

	if !adm.Admit(id(admin)) || !adm.Admit(id(member)) {
		t.Error("Admit(member) = false")
	}
	if adm.Admit(id(stranger)) || adm.Admit("not-a-key") {
		t.Error("Admit(stranger) = true")
	}
	if _, err := list.Issue(admin, trust.OpRemove, member.Public(), 0, ""); err != nil {
		t.Fatal(err)
	}
	if adm.Admit(id(member)) {
		t.Error("Admit(removed member) = true")
	}
}

func TestSwitchProject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		me, old, other := newDevice(t), newDevice(t), newDevice(t)
		oldList := newProject(t, old)
		if _, err := oldList.Issue(old, trust.OpAdd, me.Public(), trust.RoleWriter, "me"); err != nil {
			t.Fatal(err)
		}
		a := join(t, net, me, fork(t, oldList))
		o := join(t, net, old, oldList)
		net.Link(a.id, o.id)
		if ev := next(t, a.w.Repo()); ev.Kind != transport.PeerUp || ev.Peer != o.id {
			t.Fatalf("got %v from %s, want PeerUp of the old project's admin", ev.Kind, ev.Peer)
		}

		// The other project, which me joins, is administered by a device that is
		// already connected but not yet a member for me.
		newList := newProject(t, other)
		if _, err := newList.Issue(other, trust.OpAdd, me.Public(), trust.RoleReader, "me"); err != nil {
			t.Fatal(err)
		}
		x := join(t, net, other, newList)
		net.Link(a.id, x.id)
		nothing(t, a.w.Repo())

		var saved []trust.Op
		if err := a.w.SwitchProject(fork(t, newList), func(op trust.Op) error { saved = append(saved, op); return nil }); err != nil {
			t.Fatal(err)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.PeerDown || ev.Peer != o.id {
			t.Errorf("got %v from %s, want PeerDown of the old project's admin", ev.Kind, ev.Peer)
		}
		if ev := next(t, a.w.Repo()); ev.Kind != transport.PeerUp || ev.Peer != x.id {
			t.Errorf("got %v from %s, want PeerUp of the new project's admin", ev.Kind, ev.Peer)
		}
		if err := a.w.Repo().Send(t.Context(), o.id, []byte("x")); !errors.Is(err, transport.ErrUnknownPeer) {
			t.Errorf("Send to the old project's admin = %v, want ErrUnknownPeer", err)
		}
		if len(saved) != 0 {
			t.Errorf("SwitchProject saved %d ops that were already stored", len(saved))
		}
		if a.adm.Admit(o.id) || !a.adm.Admit(x.id) {
			t.Error("the admission did not follow the project")
		}
	})
}

// TestAdoptProjectNeverWritesTheOldProjectInTheNewStore parks a command of the
// old project, an Issue, while the new project is persisted, and checks that it
// runs after the switch: the new store holds only the new project's ops.
func TestAdoptProjectNeverWritesTheOldProjectInTheNewStore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		me, other := newDevice(t), newDevice(t)
		proj, _ := newProjectFolder(t)
		oldList, st, err := openTrust(proj, me)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.close() })
		a := join(t, net, me, oldList, WiringOptions{Save: st.save})
		newList := newProject(t, other)
		if _, err := newList.Issue(other, trust.OpAdd, me.Public(), trust.RoleAdmin, "me"); err != nil {
			t.Fatal(err)
		}

		own := len(newList.Since(nil)) // the wiring adopts newList and grows it.
		var wg sync.WaitGroup
		persist := func() error {
			// Queued behind this command: it cannot run before the switch.
			wg.Go(func() {
				if _, err := a.w.Issue(trust.OpAdd, newDevice(t).Public(), trust.RoleReader, "x"); err != nil {
					t.Error(err)
				}
			})
			synctest.Wait()
			return st.replace(newList.Since(nil))
		}
		if err := a.w.AdoptProject(newList, persist, st.save); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		ops, err := st.readAll()
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(ops, func(o trust.Op) bool { return o.Hash() == oldList.Since(nil)[0].Hash() }) {
			t.Error("the genesis op of the old project is in the new store")
		}
		list, err := buildList(ops)
		if err != nil {
			t.Fatal(err)
		}
		if list.Project() != newList.Project() || len(ops) != own+1 {
			t.Errorf("new store: project %v with %d ops, want %v with %d (its own plus the issued one)", list.Project(), len(ops), newList.Project(), own+1)
		}

		// A failure to persist keeps the project.
		b := join(t, net, newDevice(t), newProject(t, me))
		before := b.adm.list.Load().Project()
		if err := b.w.AdoptProject(newList, func() error { return errInjected }, nil); !errors.Is(err, errInjected) {
			t.Fatalf("AdoptProject() = %v, want the injected failure", err)
		}
		if got := b.adm.list.Load().Project(); got != before {
			t.Errorf("project = %v after a failed persist, want %v", got, before)
		}
	})
}
