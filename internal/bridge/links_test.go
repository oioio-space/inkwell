package bridge_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// slowNode is a memnet node whose sends wait for release, or fail with err.
type slowNode struct {
	*memnet.Node
	release chan struct{}
	err     error
}

func (s *slowNode) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.err != nil {
		return s.err
	}
	return s.Node.Send(ctx, to, p)
}

// attach runs a router over node with a webview connected, and a peer b linked.
func attach(t *testing.T, node *slowNode, n *memnet.Network) (*bridge.Router, *fakeConn) {
	t.Helper()
	r := newRouter(node, quietLogger())
	c := newFakeConn(t)
	go func() { _ = r.Run(t.Context()) }()
	go r.Serve(c)
	synctest.Wait()
	n.Link("a", "b")
	if got := c.next(t); got.Kind != bridge.PeerUp {
		t.Fatalf("got %+v, want PeerUp", got)
	}
	return r, c
}

func linkTo(t *testing.T, r *bridge.Router, peer transport.PeerID) bridge.PeerLink {
	t.Helper()
	for _, l := range r.Links() {
		if l.Peer == peer {
			return l
		}
	}
	t.Fatalf("Links() has no %s: %+v", peer, r.Links())
	return bridge.PeerLink{}
}

func message(t *testing.T, peer transport.PeerID, size int) []byte {
	return mustEncode(t, bridge.Envelope{Kind: bridge.Message, Peer: peer, Payload: make([]byte, size)})
}

// Plan §7, pendingMsgs/pendingBytes/oldestPendingMs: what waits for a person is
// counted, the message in flight included, and « Lien lent » shows as a grade.
func TestLinkPendingWhileSendBlocked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		node := &slowNode{Node: n.Join("a"), release: make(chan struct{})}
		n.Join("b")
		r, c := attach(t, node, n)

		for range 3 {
			c.push(message(t, "b", 100))
		}
		synctest.Wait()
		time.Sleep(3 * time.Second)
		l := linkTo(t, r, "b")
		if l.PendingMsgs != 3 || l.PendingBytes != 300 {
			t.Errorf("pending = %d msgs %d bytes, want 3 and 300", l.PendingMsgs, l.PendingBytes)
		}
		if l.OldestPending < 3*time.Second || l.OldestPending > 4*time.Second {
			t.Errorf("OldestPending = %v, want about 3s", l.OldestPending)
		}
		// « Lien lent » is the sync indicator's signal (OldestPending), never the link's level.
		if got := l.Quality(); got != transport.QualityUnknown {
			t.Errorf("Quality() with a message waiting 3 s = %v, want the probe's level (unknown here)", got)
		}
		time.Sleep(8 * time.Second)
		if got := linkTo(t, r, "b").Quality(); got != transport.QualityUnknown {
			t.Errorf("Quality() with a message waiting 11 s = %v, want the probe's level (unknown here)", got)
		}

		close(node.release)
		synctest.Wait()
		l = linkTo(t, r, "b")
		if l.PendingMsgs != 0 || l.PendingBytes != 0 || l.OldestPending != 0 {
			t.Errorf("pending after release = %+v, want none", l)
		}
	})
}

// Plan §7, sendFailures: a write given up is counted, once.
func TestLinkSendFailuresCounted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		node := &slowNode{Node: n.Join("a"), err: errors.New("boom")}
		n.Join("b")
		r, c := attach(t, node, n)

		c.push(message(t, "b", 10))
		synctest.Wait()
		l := linkTo(t, r, "b")
		if l.SendFailures != 1 || l.PendingMsgs != 0 {
			t.Errorf("after one failed send: failures %d pending %d, want 1 and 0", l.SendFailures, l.PendingMsgs)
		}
	})
}

// Plan §7, resets: a queue that overflows resets the peer, and it is counted.
func TestLinkResetsCountedOnOverflow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		node := &slowNode{Node: n.Join("a"), release: make(chan struct{})}
		n.Join("b")
		r, c := attach(t, node, n)

		for range 80 { // the depth is 64, plus the message in flight
			c.push(message(t, "b", 1))
		}
		synctest.Wait()
		l := linkTo(t, r, "b")
		if l.Resets < 1 {
			t.Errorf("Resets = %d after an overflow, want at least 1", l.Resets)
		}
	})
}

// Plan §7, ownership 2: the router merges its figures into the transport's
// snapshot, and falls back to the connected peers when the transport reports
// no links.
func TestLinkMergesTransportLinks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := memnet.New(1, memnet.Faults{})
		a := n.Join("a")
		n.Join("b")
		r := newRouter(a, quietLogger())
		n.Link("a", "b")
		got := r.Links()
		if len(got) != 1 || got[0].Peer != "b" || got[0].Kind != transport.LinkDirect {
			t.Errorf("Links() = %+v, want b reported by the transport as direct", got)
		}

		plain := newRouter(struct{ transport.Transport }{a}, quietLogger())
		got = plain.Links()
		if len(got) != 1 || got[0].Peer != "b" || got[0].Kind != 0 {
			t.Errorf("Links() without LinkSource = %+v, want b with no figure", got)
		}
	})
}

// Plan §7, privacy: nothing but the peer's name is a string in a PeerLink, and
// there is no address type, so no address can cross the bridge.
func TestLinkHasNoAddress(t *testing.T) {
	var check func(path string, ty reflect.Type)
	check = func(path string, ty reflect.Type) {
		switch ty.Kind() {
		case reflect.String:
			if ty != reflect.TypeFor[transport.PeerID]() {
				t.Errorf("%s is a %v string: only the peer's name may be text", path, ty)
			}
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Interface, reflect.Chan, reflect.Func:
			t.Errorf("%s is a %v: a PeerLink holds scalars only", path, ty.Kind())
		case reflect.Pointer:
			check(path, ty.Elem())
		case reflect.Struct:
			if ty == reflect.TypeFor[time.Time]() {
				return
			}
			for f := range ty.Fields() {
				check(path+"."+f.Name, f.Type)
			}
		}
	}
	check("PeerLink", reflect.TypeFor[bridge.PeerLink]())
}

// Spec Annex A.1: the level of a relayed link is unknown, always, whatever figure the first hop gave.
func TestPeerLinkQualityIsTheProbeLevelNeverOnARelay(t *testing.T) {
	for _, tc := range []struct {
		name string
		l    bridge.PeerLink
		want transport.Quality
	}{
		{"direct, fair", bridge.PeerLink{Kind: transport.LinkDirect, ProbeLevel: transport.QualityFair}, transport.QualityFair},
		{"direct, unknown, a message waiting", bridge.PeerLink{Kind: transport.LinkDirect, OldestPending: time.Minute}, transport.QualityUnknown},
		{"relayed, poor figure", bridge.PeerLink{Kind: transport.LinkRelayed, ProbeLevel: transport.QualityPoor}, transport.QualityUnknown},
		{"relayed, a message waiting", bridge.PeerLink{Kind: transport.LinkRelayed, OldestPending: time.Minute}, transport.QualityUnknown},
	} {
		if got := tc.l.Quality(); got != tc.want {
			t.Errorf("%s: Quality() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
