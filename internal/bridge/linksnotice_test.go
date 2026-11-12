package bridge_test

import (
	"bytes"
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// The conversion rules (ages, saturation, permille) are the protocol's and are
// tested there; this is the mapping of a PeerLink onto them.
func TestLinksNotificationMapsPeerLink(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	loss := uint16(43)
	got := bridge.LinkEntryOf(bridge.PeerLink{
		Peer: "peer-a", Kind: transport.LinkRelayed, PathSince: now.Add(-61 * time.Second), Upgrades: 2,
		RTT: 500 * time.Millisecond, ProbeLevel: transport.QualityPoor, ProbeLoss: &loss, LossScope: transport.LossFirstHop,
		UpSince: now.Add(-2 * time.Minute), Downs10m: 1,
		PendingMsgs: 2, PendingBytes: 4096, OldestPending: 3 * time.Second, SendFailures: 5, Resets: 1,
	}, now)
	// A relayed link's level is unknown whatever the first hop's figure (Annex A.1); the figure stays.
	if got.Path != 3 || got.Quality != 0 || got.LossScope != 2 || got.LossPermille != 43 ||
		got.PathAgeMs != 61000 || got.UpMs != 120000 || got.PendingBytes != 4096 || got.OldestPendingMs != 3000 || got.Resets != 1 {
		t.Errorf("entry = %+v", got)
	}
	none := bridge.LinkEntryOf(bridge.PeerLink{Peer: "p", Kind: transport.LinkLAN}, now)
	if none.Quality != 0 || none.Path != 1 || none.LossPermille != bridge.LossUnknown || none.LossScope != 0 {
		t.Errorf("unmeasured entry = %+v", none)
	}
}

// Spec Annex A.1, Links: no network address (IP, multiaddress) is part of the
// payload, only the kind of path (§18).
func TestLinksNotificationHasNoAddress(t *testing.T) {
	now := time.Now()
	ls := []bridge.PeerLink{
		{Peer: "peer-a", Kind: transport.LinkDirect, PathSince: now},
		{Peer: "peer-b", Kind: transport.LinkRelayed, PathSince: now},
	}
	b, err := bridge.EncodeLinks(bridge.LinkEntries(ls, now))
	if err != nil {
		t.Fatal(err)
	}
	// The payload is made of the count, and per peer its length, its ID and the
	// fixed figures: there is no room for anything else.
	if want := 2 + 2*(1+len("peer-a")+bridge.LinkFixed); len(b) != want {
		t.Errorf("payload of %d bytes, want %d", len(b), want)
	}
	for _, s := range []string{"/ip4", "/ip6", "/tcp", "/udp", "/quic", "/p2p", "/dns", "/"} {
		if bytes.Contains(b, []byte(s)) {
			t.Errorf("payload contains %q", s)
		}
	}
}

// newLinksRouter returns a router with the Links notifications on.
func newLinksRouter(t transport.Transport) *bridge.Router {
	r := bridge.NewRouter(t, quietLogger())
	r.SetLinksPeriod(bridge.LinksPushPeriod)
	return r
}

// drain reads the envelopes waiting for the webview: the Links ones decoded, the others apart.
func drain(t *testing.T, c *fakeConn) (links [][]bridge.LinkEntry, others []bridge.Envelope) {
	t.Helper()
	for len(c.sent) > 0 {
		e := c.next(t)
		if e.Kind != bridge.Links {
			others = append(others, e)
			continue
		}
		ls, err := bridge.DecodeLinks(e.Payload)
		if err != nil {
			t.Fatal(err)
		}
		links = append(links, ls)
	}
	return links, others
}

// Spec Annex A.1, coalescence of Links: the current state at once to a webview
// that connects, then at most one per second and only if the content changed.
func TestLinksNotificationPush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("b")
		n.Join("c")
		r := newLinksRouter(a)
		ctx, cancel := context.WithCancel(t.Context())
		runDone := wait(func() { _ = r.Run(ctx) })
		c := newFakeConn(t)
		serveDone := wait(func() { r.Serve(c) })
		synctest.Wait()

		first := c.next(t)
		if first.Kind != bridge.Links || first.Peer != "a" {
			t.Fatalf("first frame = %+v, want Links from the local peer", first)
		}
		if ls, err := bridge.DecodeLinks(first.Payload); err != nil || len(ls) != 0 {
			t.Fatalf("first Links = %+v, %v; want empty", ls, err)
		}

		// Nothing changed: nothing is sent, however long we wait (the ages grow).
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if links, others := drain(t, c); len(links)+len(others) != 0 {
			t.Fatalf("idle frames: %d Links, %+v", len(links), others)
		}

		// Two changes within one second give one notification, carrying both.
		n.Link("a", "b")
		time.Sleep(300 * time.Millisecond)
		n.Link("a", "c")
		time.Sleep(3 * time.Second)
		synctest.Wait()
		links, _ := drain(t, c)
		if len(links) != 1 || len(links[0]) != 2 || links[0][0].Peer != "b" || links[0][1].Peer != "c" {
			t.Fatalf("after two changes got %+v, want one Links of b and c", links)
		}

		time.Sleep(10 * time.Second)
		synctest.Wait()
		if links, _ := drain(t, c); len(links) != 0 {
			t.Errorf("unchanged links sent again: %+v", links)
		}

		// The pusher stops with the session: synctest fails the test if it
		// outlives the bubble.
		c.close()
		<-serveDone
		cancel()
		<-runDone
	})
}

// A webview that attaches later receives the current state at once.
func TestLinksNotificationOnAttach(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("b")
		n.Link("a", "b")
		r := newLinksRouter(a)
		c := newFakeConn(t)
		serveDone := wait(func() { r.Serve(c) })
		synctest.Wait()
		links, _ := drain(t, c)
		if len(links) != 1 || len(links[0]) != 1 || links[0][0].Peer != "b" {
			t.Errorf("Links on attach = %+v, want b", links)
		}
		c.close()
		<-serveDone
	})
}

// A router with the notifications off sends no Links at all.
func TestLinksNotificationOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		r := newRouter(n.Join("a"), quietLogger())
		c := newFakeConn(t)
		serveDone := wait(func() { r.Serve(c) })
		time.Sleep(5 * time.Second)
		synctest.Wait()
		if links, others := drain(t, c); len(links)+len(others) != 0 {
			t.Errorf("frames with Links off: %d Links, %+v", len(links), others)
		}
		c.close()
		<-serveDone
	})
}
