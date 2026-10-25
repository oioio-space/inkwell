package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
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
		if len(notices) != 1 || notices[0].Kind != bridge.Attested || notices[0].Peer != a.id {
			t.Errorf("Notices() = %+v, want the one attestation of A", notices)
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
		if n := attester(t, m).Notices(); len(n) != 1 || n[0].Kind != bridge.Sealed {
			t.Errorf("Notices() = %+v, want the Sealed notice", n)
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
