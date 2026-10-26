package app

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

// memCarrier carries tickets in memory, as a code server would.
type memCarrier struct {
	tickets map[string]invite.Ticket
	mu      sync.Mutex
	n       int
}

func (m *memCarrier) Offer(ctx context.Context, t invite.Ticket) (string, <-chan error, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n++
	code := "7-" + strings.Repeat("a", m.n) + "-code"
	if m.tickets == nil {
		m.tickets = make(map[string]invite.Ticket)
	}
	m.tickets[code] = t
	done := make(chan error, 1)
	go func() { <-ctx.Done(); done <- ctx.Err(); close(done) }()
	return code, done, nil
}

func (m *memCarrier) Fetch(_ context.Context, code string) (invite.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tickets[code]
	if !ok {
		return invite.Ticket{}, errors.New("no such code")
	}
	return t, nil
}

// testLANService is an mDNS service name of this test run alone: the tests do
// not see (or answer) the inkwell instances of the real network.
var testLANService = "_ink" + rand.Text()[:12] + "._udp"

// dirs are what a runtime keeps on disk: the identity directory and the
// project folder.
type dirs struct {
	home *os.Root
	proj *project.Project
}

// startRuntime starts a runtime on a new identity directory and a new project.
// Codes go through c.
func startRuntime(t *testing.T, c carrier, lan bool) (*Runtime, dirs) {
	t.Helper()
	home, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	proj, _ := newProjectFolder(t)
	rt, err := Start(t.Context(), home, Options{Project: proj, RendezvousURL: NoCodes, LAN: lan, LANService: testLANService})
	if err != nil {
		t.Fatal(err)
	}
	rt.inv.carrier = c
	t.Cleanup(func() { _ = rt.Close() })
	return rt, dirs{home, proj}
}

func (r *Runtime) role(of *Runtime) trust.Role {
	dev := of.inv.dev
	return r.inv.currentList().Role(dev.Public())
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// expectRepoUp waits for the PeerUp of peer on the bridge transport of r.
func expectRepoUp(t *testing.T, r *Runtime, peer transport.PeerID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	for {
		ev, err := r.Repo.Recv(ctx)
		if err != nil {
			t.Fatalf("waiting for PeerUp of %s: %v", peer, err)
		}
		if ev.Kind == transport.PeerUp && ev.Peer == peer {
			return
		}
	}
}

// noNet is a Network with nobody on it.
type noNet struct{}

func (noNet) Local() transport.PeerID   { return "me" }
func (noNet) Addrs() []string           { return nil }
func (noNet) Peers() []transport.PeerID { return nil }
func (noNet) Connect(context.Context, transport.PeerID, []string) error {
	return transport.ErrUnknownPeer
}

func TestSessionWithoutProject(t *testing.T) {
	s := NewSession(noNet{})
	if _, err := s.Invite("writer"); err == nil {
		t.Error("Invite() on a session without project = nil")
	}
	if _, err := s.Join(t.Context(), "inkwell1:AA"); err == nil {
		t.Error("Join() on a session without project = nil")
	}
	if s.Visible() || len(s.Requests()) != 0 || len(s.Nearby(t.Context())) != 0 {
		t.Error("a session without project is visible or has requests")
	}
}

func TestJoinByTicketText(t *testing.T) {
	a, _ := startRuntime(t, nil, false)
	b, bDirs := startRuntime(t, nil, false)

	inv, err := a.Session.Invite("writer")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Code != "" || !strings.HasPrefix(inv.Text, "inkwell1:") || len(inv.Text) > invite.MaxTextLen {
		t.Fatalf("Invite() = %+v, want text only", inv)
	}

	// A stranger with the right text but another identity cannot be mistaken
	// for a member before it redeems, and garbage is refused.
	for _, bad := range []string{"", "hello", "inkwell1:AAAA", "/ip4/127.0.0.1/tcp/1/p2p/12D3KooWAbc"} {
		if _, err := b.Session.Join(t.Context(), bad); err == nil {
			t.Errorf("Join(%q) = nil, want an error", bad)
		}
	}
	if _, err := a.Session.Join(t.Context(), inv.Text); err == nil {
		t.Error("Join(own invitation) = nil, want an error")
	}

	peer, err := b.Session.Join(t.Context(), inv.Text)
	if err != nil {
		t.Fatalf("Join() = %v", err)
	}
	if peer != a.Session.LocalPeer() {
		t.Errorf("Join() = %q, want the host %q", peer, a.Session.LocalPeer())
	}
	if got := a.role(b); got != trust.RoleWriter {
		t.Errorf("host sees the joiner as %v, want writer", got)
	}
	if got, want := b.inv.currentList().Project(), a.inv.currentList().Project(); got != want {
		t.Errorf("joiner works on project %v, want the host's %v", got, want)
	}
	if got := b.role(a); got != trust.RoleAdmin {
		t.Errorf("joiner sees the host as %v, want admin", got)
	}

	// Both bridges see each other, and the ticket is spent.
	expectRepoUp(t, a, b.Node.Local())
	expectRepoUp(t, b, a.Node.Local())
	c, _ := startRuntime(t, nil, false)
	if _, err := c.Session.Join(t.Context(), inv.Text); err == nil {
		t.Error("Join(replayed ticket) = nil, want an error")
	}
	if a.role(c) != 0 {
		t.Error("a replayed ticket made a member")
	}
	if got := a.Node.Peers(); len(got) != 1 {
		t.Errorf("host peers = %v, want only the joiner", got)
	}

	// The joined project is the one found again after a restart.
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Start(t.Context(), bDirs.home, Options{Project: bDirs.proj, RendezvousURL: NoCodes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	if got, want := again.inv.currentList().Project(), a.inv.currentList().Project(); got != want {
		t.Errorf("after a restart the joiner works on %v, want %v", got, want)
	}
}

func TestJoinByCode(t *testing.T) {
	codes := &memCarrier{}
	a, _ := startRuntime(t, codes, false)
	b, _ := startRuntime(t, codes, false)
	inv, err := a.Session.Invite("reader")
	if err != nil {
		t.Fatal(err)
	}
	if !invite.LooksLikeCode(inv.Code) {
		t.Fatalf("Invite() code = %q, want a code", inv.Code)
	}
	if _, err := b.Session.Join(t.Context(), inv.Code); err != nil {
		t.Fatalf("Join(code) = %v", err)
	}
	if got := a.role(b); got != trust.RoleReader {
		t.Errorf("host sees the joiner as %v, want reader", got)
	}
	if _, err := b.Session.Join(t.Context(), "9-unknown-code"); err == nil {
		t.Error("Join(unknown code) = nil")
	}
}

func TestOnlyAnAdminInvites(t *testing.T) {
	a, _ := startRuntime(t, nil, false)
	b, _ := startRuntime(t, nil, false)
	inv, err := a.Session.Invite("writer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Session.Join(t.Context(), inv.Text); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Session.Invite("reader"); !errors.Is(err, trust.ErrNotAdmin) {
		t.Errorf("Invite() by a writer = %v, want ErrNotAdmin", err)
	}
	if _, err := b.Session.InviteDevice(); !errors.Is(err, trust.ErrNotAdmin) {
		t.Errorf("InviteDevice() by a writer = %v, want ErrNotAdmin", err)
	}
	if _, err := a.Session.Invite("owner"); err == nil {
		t.Error("Invite(unknown role) = nil")
	}
}

func TestInviteDeviceKeepsPerson(t *testing.T) {
	a, _ := startRuntime(t, nil, false)
	b, _ := startRuntime(t, nil, false)
	inv, err := a.Session.InviteDevice()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Session.Join(t.Context(), inv.Text); err != nil {
		t.Fatal(err)
	}
	var mine, theirs trust.Hash
	for _, m := range a.inv.currentList().Members() {
		switch {
		case m.Pub.Equal(a.inv.dev.Public()):
			mine = m.Person
		case m.Pub.Equal(b.inv.dev.Public()):
			theirs = m.Person
			if m.Role != trust.RoleAdmin {
				t.Errorf("the second device is %v, want admin like the first", m.Role)
			}
		}
	}
	if mine == (trust.Hash{}) || mine != theirs {
		t.Errorf("persons %x and %x, want the same person on both devices", mine[:4], theirs[:4])
	}
}

func TestLanAskAndAnswer(t *testing.T) {
	a, _ := startRuntime(t, nil, true)
	b, _ := startRuntime(t, nil, true)
	if err := errors.Join(a.Node.LANError(), b.Node.LANError()); err != nil {
		t.Skipf("mDNS is not available here: %v", err)
	}
	if err := a.Session.SetVisible(true); err != nil {
		t.Fatal(err)
	}
	if !a.Session.Visible() || a.Session.VisibleUntil().IsZero() {
		t.Fatal("not visible after SetVisible(true)")
	}

	var near []NearbyPerson
	deadline := time.Now().Add(20 * time.Second)
	for len(near) == 0 {
		if near = b.Session.Nearby(t.Context()); len(near) == 0 {
			if time.Now().After(deadline) {
				t.Skip("mDNS multicast does not work on this machine")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	if near[0].Peer != a.Session.LocalPeer() || near[0].Title != "Mon livre" || near[0].Machine == "" {
		t.Fatalf("Nearby() = %+v, want the host with its title", near)
	}

	// The host declines first.
	errc := make(chan error, 1)
	go func() {
		_, err := b.Session.Ask(context.Background(), near[0].Peer, "Bob")
		errc <- err
	}()
	var reqs []JoinRequest
	waitUntil(t, "the request", func() bool { reqs = a.Session.Requests(); return len(reqs) == 1 })
	if reqs[0].Name != "Bob" || reqs[0].Peer != b.Session.LocalPeer() || reqs[0].Fingerprint == "" {
		t.Errorf("Requests() = %+v, want Bob with a fingerprint", reqs)
	}
	if err := a.Session.Answer(reqs[0].ID, "refuse"); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err == nil {
		t.Error("Ask() refused = nil")
	}
	if a.role(b) != 0 {
		t.Error("a refused request made a member")
	}

	// Then accepts as a reader.
	go func() {
		_, err := b.Session.Ask(context.Background(), near[0].Peer, "Bob")
		errc <- err
	}()
	waitUntil(t, "the second request", func() bool { reqs = a.Session.Requests(); return len(reqs) == 1 })
	if err := a.Session.Answer(reqs[0].ID, "reader"); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("Ask() accepted = %v", err)
	}
	if got := a.role(b); got != trust.RoleReader {
		t.Errorf("host sees the joiner as %v, want reader", got)
	}
	expectRepoUp(t, b, a.Node.Local())
	if err := a.Session.Answer(99, "reader"); err == nil {
		t.Error("Answer(unknown request) = nil")
	}
	if err := a.Session.Answer(1, "owner"); err == nil {
		t.Error("Answer(unknown role) = nil")
	}
}

func TestNotVisibleIsInvisible(t *testing.T) {
	a, _ := startRuntime(t, nil, true)
	b, _ := startRuntime(t, nil, true)
	if err := errors.Join(a.Node.LANError(), b.Node.LANError()); err != nil {
		t.Skipf("mDNS is not available here: %v", err)
	}
	// Whatever mDNS shows, a host that is not visible answers no greeting.
	time.Sleep(2 * time.Second)
	// Other inkwell instances of the machine (a parallel test run, a running app) are on the same
	// mDNS segment and may answer: only this host must be missing.
	for _, n := range b.Session.Nearby(t.Context()) {
		if n.Peer == a.Session.LocalPeer() {
			t.Errorf("Nearby() = %+v lists a host that is not visible", n)
		}
	}
	if _, err := b.Session.Ask(t.Context(), a.Session.LocalPeer(), "Bob"); err == nil {
		t.Error("Ask() to a host that is not visible = nil")
	}
}
