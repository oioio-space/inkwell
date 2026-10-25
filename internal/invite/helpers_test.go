package invite_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/invite"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

func testDevice(tb testing.TB) *identity.Device {
	tb.Helper()
	root, err := os.OpenRoot(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = root.Close() })
	dev, err := identity.LoadOrCreate(root)
	if err != nil {
		tb.Fatal(err)
	}
	return dev
}

// peerOf is the test identity of a device: its key in hexadecimal.
func peerOf(dev *identity.Device) transport.PeerID {
	return transport.PeerID(hex.EncodeToString(dev.Public()))
}

func keyOf(id transport.PeerID) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(string(id))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("not a test peer")
	}
	return b, nil
}

// withTimeout returns a context that ends with the test or after 30 s.
func withTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(t.Context(), 30*time.Second)
}

// clock is a controllable time source.
type clock struct {
	now time.Time
	mu  sync.Mutex
}

func newClock() *clock { return &clock{now: time.Unix(1_800_000_000, 0)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// project is a project administered by its creator, with its host.
type project struct {
	admin *identity.Device
	list  *trust.List
	host  *invite.Host
	clock *clock
	// opened records the last window the host announced.
	opened struct {
		until time.Time
		mu    sync.Mutex
	}
}

func (p *project) openUntil() time.Time {
	p.opened.mu.Lock()
	defer p.opened.mu.Unlock()
	return p.opened.until
}

// newProject creates a project, administered by a new device, and its host.
func newProject(t testing.TB) *project {
	t.Helper()
	admin := testDevice(t)
	return projectOf(t, admin, nil)
}

// projectOf makes the host of list for dev; with a nil list, dev creates one.
func projectOf(t testing.TB, dev *identity.Device, list *trust.List) *project {
	t.Helper()
	if list == nil {
		g, id := trust.Genesis(dev, "Mon livre", "automerge:root")
		list = trust.New(id)
		if err := list.Apply(g); err != nil {
			t.Fatal(err)
		}
	}
	p := &project{admin: dev, list: list, clock: newClock()}
	p.host = invite.NewHost(invite.HostConfig{
		List:    list,
		Issue:   func(tmpl trust.Op) (trust.Op, error) { return list.IssueWith(dev, tmpl) },
		KeyOf:   keyOf,
		Addrs:   func() []string { return []string{"/ip4/127.0.0.1/udp/1/quic-v1/p2p/" + string(peerOf(dev))} },
		OnOpen:  func(until time.Time) { p.opened.mu.Lock(); p.opened.until = until; p.opened.mu.Unlock() },
		Now:     p.clock.Now,
		Self:    peerOf(dev),
		SelfKey: dev.Public(),
		Machine: "portable",
		Lang:    "fr",
	})
	return p
}

// direct is a Lobby that calls a host's Serve in-process, as the network would.
type direct struct {
	host *invite.Host
	from transport.PeerID
}

func (d direct) AskLobby(ctx context.Context, _ transport.PeerID, req []byte) ([]byte, error) {
	b, ok := d.host.Serve(ctx, d.from, req)
	if !ok {
		return nil, errors.New("stream reset")
	}
	return b, nil
}

// joiner returns the side of dev that talks to p's host.
func (p *project) joiner(dev *identity.Device, name string) invite.Joiner {
	return invite.Joiner{Lobby: direct{host: p.host, from: peerOf(dev)}, KeyOf: keyOf, Name: name, Self: dev.Public()}
}
