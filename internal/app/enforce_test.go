package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/trust"
)

// nextEnvelope returns the next envelope for the bridge, notices included.
func nextEnvelope(t *testing.T, tr transport.Transport) bridge.Envelope {
	t.Helper()
	src, ok := tr.(bridge.Source)
	if !ok {
		t.Fatalf("%T is not a bridge.Source", tr)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	e, err := src.RecvEnvelope(ctx)
	if err != nil {
		t.Fatalf("RecvEnvelope(%s): %v", tr.Local(), err)
	}
	return e
}

func attester(t *testing.T, m *member) bridge.Attester {
	t.Helper()
	a, ok := m.w.Repo().(bridge.Attester)
	if !ok {
		t.Fatalf("%T is not a bridge.Attester", m.w.Repo())
	}
	return a
}

func head(s string) [bridge.HashSize]byte { return sha256.Sum256([]byte(s)) }

// pair returns a writer-admin A and a member B with role r, linked and
// connected on both sides.
func pair(t *testing.T, net *memnet.Network, r trust.Role) (a, b *member, devB *identity.Device) {
	t.Helper()
	devA, devB := newDevice(t), newDevice(t)
	listA := newProject(t, devA)
	if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), r, "b"); err != nil {
		t.Fatal(err)
	}
	a, b = join(t, net, devA, listA), join(t, net, devB, fork(t, listA))
	net.Link(a.id, b.id)
	synctest.Wait()
	for _, m := range []*member{a, b} {
		if e := nextEnvelope(t, m.w.Repo()); e.Kind != bridge.PeerUp {
			t.Fatalf("%s got %v, want PeerUp", m.id, e.Kind)
		}
	}
	return a, b, devB
}

func TestAttestedPrecedesRepoFrame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)

		// What the router does for the webview of A: attest, then relay.
		if err := attester(t, a).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h1"), head("h2")}); err != nil {
			t.Fatal(err)
		}
		if err := a.w.Repo().Send(t.Context(), b.id, []byte("sync")); err != nil {
			t.Fatal(err)
		}

		e := nextEnvelope(t, b.w.Repo())
		if e.Kind != bridge.Attested || e.Peer != a.id {
			t.Fatalf("first envelope = %v from %s, want Attested by A", e.Kind, e.Peer)
		}
		h, err := bridge.DecodeHeads(e.Payload)
		if err != nil || h.Doc != "doc" || len(h.Heads) != 2 {
			t.Fatalf("attested heads = %+v, %v; want doc with 2 heads", h, err)
		}
		if e := nextEnvelope(t, b.w.Repo()); e.Kind != bridge.Message || string(e.Payload) != "sync" {
			t.Errorf("second envelope = %v %q, want the sync message", e.Kind, e.Payload)
		}
		// The attestation of A also reaches A's own webview.
		if e := nextEnvelope(t, a.w.Repo()); e.Kind != bridge.Attested || e.Peer != a.id {
			t.Errorf("A's own webview got %v from %s, want Attested by A", e.Kind, e.Peer)
		}
	})
}

func TestAttestSeqKeepsGrowing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, _, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
		var last uint64
		for i := range 5 { // no time passes in the bubble: Seq must still increase
			if err := attester(t, a).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head(fmt.Sprint(i))}); err != nil {
				t.Fatal(err)
			}
			got := a.w.Attestations()
			if len(got) != 1 || got[0].Seq <= last {
				t.Fatalf("attestation %d: %v, want Seq above %d", i, got, last)
			}
			last = got[0].Seq
		}
	})
}

func TestReaderCannotAttest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleReader)

		err := attester(t, b).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("forged")})
		if !errors.Is(err, trust.ErrNotWriter) {
			t.Errorf("Attest(reader) = %v, want ErrNotWriter", err)
		}
		synctest.Wait()
		if got := a.w.Attestations(); len(got) != 0 {
			t.Errorf("A holds %v, want no attestation from a reader", got)
		}
		if got := b.w.Attestations(); len(got) != 0 {
			t.Errorf("the reader holds %v, want none", got)
		}
		nothing(t, a.w.Repo())

		// Not even later, once the reader is made a writer: its forged heads
		// were refused, not kept.
		if _, err := a.w.Issue(trust.OpSetRole, b.dev.Public(), trust.RoleWriter, ""); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := a.w.Attestations(); len(got) != 0 {
			t.Errorf("A holds %v after the promotion, want none", got)
		}
	})
}

func TestRemovedMemberCannotAttest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b, devB := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
		if _, err := a.w.Issue(trust.OpRemove, devB.Public(), 0, ""); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		err := attester(t, b).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("x")})
		if !errors.Is(err, trust.ErrNotWriter) {
			t.Errorf("Attest(removed member) = %v, want ErrNotWriter", err)
		}
	})
}

// A guest signs what its webview vouches for before the member list that
// admits it has arrived: the heads wait.
func TestGuestAttestsOnceAdmitted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devG := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		a := join(t, net, devA, listA)
		g := join(t, net, devG, fork(t, listA)) // does not know itself yet

		if err := attester(t, g).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("g1")}); err != nil {
			t.Fatalf("Attest(unknown device) = %v, want nil: the heads are kept", err)
		}
		if got := g.w.Attestations(); len(got) != 0 {
			t.Fatalf("the guest signed %v before being a member", got)
		}

		if _, err := a.w.Issue(trust.OpAdd, devG.Public(), trust.RoleWriter, "g"); err != nil {
			t.Fatal(err)
		}
		net.Link(a.id, g.id)
		synctest.Wait()
		got := a.w.Attestations()
		if len(got) != 1 || got[0].Doc != "doc" || got[0].Heads[0] != head("g1") {
			t.Errorf("A holds %v, want the guest's attestation of g1", got)
		}
	})
}

func TestRevokedWriterAttestationsAreForgotten(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b, devB := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
		if err := attester(t, b).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("b1")}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if e := nextEnvelope(t, a.w.Repo()); e.Kind != bridge.Attested || e.Peer != b.id {
			t.Fatalf("A got %v from %s, want Attested by B", e.Kind, e.Peer)
		}

		if _, err := a.w.Issue(trust.OpRemove, devB.Public(), 0, ""); err != nil {
			t.Fatal(err)
		}
		var kinds []bridge.Kind
		for len(kinds) < 2 {
			e := nextEnvelope(t, a.w.Repo())
			kinds = append(kinds, e.Kind)
			if e.Kind == bridge.Revoked && e.Peer != b.id {
				t.Errorf("Revoked %s, want B", e.Peer)
			}
		}
		if got := a.w.Attestations(); len(got) != 0 {
			t.Errorf("A holds %v, want B's attestation forgotten", got)
		}
		if n := attestedBy(a, b.id); n != 0 {
			t.Errorf("Notices() still reports %d attestations of B", n)
		}
		if !slices.Contains(kinds, bridge.Revoked) {
			t.Errorf("envelopes after the removal = %v, want a Revoked", kinds)
		}
	})
}

// attestedBy counts the attestations of signer that a webview attaching to m
// would be told about.
func attestedBy(m *member, signer transport.PeerID) int {
	n := 0
	for _, e := range m.w.Repo().(bridge.Attester).Notices() {
		if e.Kind == bridge.Attested && e.Peer == signer {
			n++
		}
	}
	return n
}

// noEnvelope fails if the bridge has a queued envelope, notices included.
func noEnvelope(t *testing.T, tr transport.Transport) {
	t.Helper()
	synctest.Wait()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if e, err := tr.(bridge.Source).RecvEnvelope(ctx); err == nil {
		t.Errorf("RecvEnvelope(%s) = %v from %s, want nothing", tr.Local(), e.Kind, e.Peer)
	}
}

func TestNoticesDescribeTheState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, _, _ := pair(t, memnet.New(1, memnet.Faults{}), trust.RoleWriter)
		if err := attester(t, a).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h")}); err != nil {
			t.Fatal(err)
		}
		notices := attester(t, a).Notices()
		if len(notices) != 2 || notices[0].Kind != bridge.Roles || notices[1].Kind != bridge.Attested || notices[1].Peer != a.id {
			t.Errorf("Notices() = %+v, want the roles, then the one attestation of A", notices)
		}
	})
}

// sealed returns a project whose list holds the maximum number of ops.
func sealed(t *testing.T, admin trust.Signer) *trust.List {
	t.Helper()
	l := newProject(t, admin)
	for i := 0; !l.Sealed(); i++ {
		if _, err := l.Issue(admin, trust.OpSetRole, admin.Public(), trust.RoleAdmin, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	return l
}

func TestSealedStopsSigning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		dev := newDevice(t)
		m := join(t, net, dev, sealed(t, dev))

		// The webview learns it when it attaches...
		if n := attester(t, m).Notices(); len(n) != 2 || n[0].Kind != bridge.Roles || n[1].Kind != bridge.Sealed {
			t.Errorf("Notices() = %+v, want the roles, then the Sealed notice", n)
		}
		// ...and live, once.
		if e := nextEnvelope(t, m.w.Repo()); e.Kind != bridge.Sealed {
			t.Fatalf("first envelope = %v, want Sealed", e.Kind)
		}
		for range 2 {
			err := attester(t, m).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("h")})
			if !errors.Is(err, ErrSealed) {
				t.Errorf("Attest(sealed) = %v, want ErrSealed", err)
			}
		}
		if got := m.w.Attestations(); len(got) != 0 {
			t.Errorf("a sealed list signed %v", got)
		}
		noEnvelope(t, m.w.Repo()) // a single Sealed notice
	})
}

// A guest whose admission is in flight keeps the heads of every document of a
// project the size of the biggest tree (ADR 0023), and signs them all once it
// is a writer.
func TestPendingHeadsHoldAWholeIndex(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devB := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		a := join(t, net, devA, listA)
		b := join(t, net, devB, fork(t, listA))
		for _, m := range []*member{a, b} { // the bridge reads the attestations, as in the application
			go func() {
				for {
					if _, err := m.w.Repo().Recv(t.Context()); err != nil {
						return
					}
				}
			}()
		}
		const docs = 1000 // more than the 64 of the first limit
		for i := range docs {
			if err := b.w.AttestHeads(fmt.Sprintf("doc%04d", i), []trust.Hash{{byte(i), byte(i >> 8)}}); err != nil {
				t.Fatalf("AttestHeads %d: %v", i, err)
			}
		}
		if _, err := a.w.Issue(trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
			t.Fatal(err)
		}
		net.Link(a.id, b.id)
		synctest.Wait()
		if got := len(b.w.Attestations()); got != docs {
			t.Errorf("B signed %d documents once admitted, want all %d", got, docs)
		}
	})
}

// A guest connects to its host before it moves to the host's project (invites.enter): the host
// replays its attestations at once, while the guest is still in another project and listens to
// nobody. The guest keeps them for the moment it switches; without them it drops the host's sync
// messages and joins to an empty document until someone types (N5, JOIN-3).
func TestGuestKeepsAttestationsReplayedBeforeItSwitchesProject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devG := newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		a := join(t, net, devA, listA)
		g := join(t, net, devG, newProject(t, devG)) // alone in a project of its own

		if err := attester(t, a).Attest(t.Context(), "doc", [][bridge.HashSize]byte{head("a1")}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.w.Issue(trust.OpAdd, devG.Public(), trust.RoleWriter, "g"); err != nil {
			t.Fatal(err)
		}
		net.Link(a.id, g.id)
		synctest.Wait() // the host replays; the guest is not in the project yet

		if err := g.w.SwitchProject(fork(t, listA), func(trust.Op) error { return nil }); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		got := g.w.Attestations()
		if len(got) != 1 || got[0].Doc != "doc" || got[0].Heads[0] != head("a1") {
			t.Errorf("the guest holds %v, want the host's attestation of a1", got)
		}
	})
}

// A stranger that floods a guest with attestations fills its own share of what is kept, never the
// host's.
func TestHeldAttestationsAreBoundedPerPeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dev := newDevice(t)
		g := join(t, memnet.New(1, memnet.Faults{}), dev, newProject(t, dev))
		host, stranger := transport.PeerID("host"), transport.PeerID("stranger")
		if err := g.w.do(func() {
			for i := range maxHeld + 10 {
				g.w.hold(stranger, trust.Attestation{Doc: strconv.Itoa(i), Signer: []byte("s")})
			}
			g.w.hold(host, trust.Attestation{Doc: "doc", Signer: []byte("h")})
			if n := len(g.w.held[stranger]); n != maxHeld {
				t.Errorf("held for the stranger = %d, want %d", n, maxHeld)
			}
			if n := len(g.w.held[host]); n != 1 {
				t.Errorf("held for the host = %d, want 1", n)
			}
		}); err != nil {
			t.Fatal(err)
		}
	})
}
