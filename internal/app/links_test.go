package app

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// scriptedLinks is a dropNode whose Links are scripted, like the figures a
// p2p node measures (kind, RTT, loss), for every peer it is told about,
// connected or not.
type scriptedLinks struct {
	*dropNode
	script []transport.Link
}

func (s *scriptedLinks) Links() []transport.Link { return s.script }

// TestRouterLinksCarryTheTransportFigures proves class C3 of the plan
// (« contract wired but data wrong »): with the app's real wiring between the
// node and the router, the router shows the node's kind, RTT and loss of a
// member, and no link at all for a peer the wiring does not admit.
func TestRouterLinksCarryTheTransportFigures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		net := memnet.New(1, memnet.Faults{})
		devA, devB, stranger := newDevice(t), newDevice(t), newDevice(t)
		listA := newProject(t, devA)
		if _, err := listA.Issue(devA, trust.OpAdd, devB.Public(), trust.RoleWriter, "b"); err != nil {
			t.Fatal(err)
		}
		idA, idB, idX := p2p.IDFromKey(devA.Public()), p2p.IDFromKey(devB.Public()), p2p.IDFromKey(stranger.Public())
		loss := uint16(25)
		since := time.Unix(1_000, 0)
		node := &scriptedLinks{
			dropNode: &dropNode{Node: net.Join(idA), net: net, drops: make(chan transport.PeerID, 8)},
			script: []transport.Link{
				{Peer: idB, Kind: transport.LinkRelayed, PathSince: since, RTT: 120 * time.Millisecond, ProbeLoss: &loss},
				{Peer: idX, Kind: transport.LinkDirect, RTT: time.Millisecond},
			},
		}
		w := Wire(t.Context(), node, devA, listA, NewAdmission(listA), WiringOptions{})
		t.Cleanup(func() { _ = w.Close() })
		join(t, net, devB, fork(t, listA))
		x := net.Join(idX)
		net.Link(idA, idB)
		net.Link(idA, x.Local())
		synctest.Wait()

		r := bridge.NewRouter(w.Repo(), nil)
		got := r.Links()
		if len(got) != 1 || got[0].Peer != idB {
			t.Fatalf("Links() = %+v, want the member B only (the stranger is not admitted)", got)
		}
		l := got[0]
		if l.Kind != transport.LinkRelayed || l.RTT != 120*time.Millisecond || l.ProbeLoss == nil || *l.ProbeLoss != loss || !l.PathSince.Equal(since) {
			t.Errorf("Links()[0] = %+v, want the transport's relayed link, 120ms, 2.5%% loss", l)
		}
	})
}
