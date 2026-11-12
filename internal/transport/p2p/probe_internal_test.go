package p2p

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	wire "github.com/oioio-space/inkwell/protocol"
)

// TestLinkProbeResourceLimits: the resource manager admits, per peer, 4 inbound
// and 2 outbound probe streams and no more (spec §16.1: 2 connections × (1 + 1
// closing) inbound, 2 × 1 outbound), so the probe never takes the stream slots
// of the session.
func TestLinkProbeResourceLimits(t *testing.T) {
	if wire.LinkProbeInboundStreamsPerPeer != 4 || wire.LinkProbeOutboundStreamsPerPeer != 2 {
		t.Fatalf("protocol limits = %d inbound, %d outbound, want 4 and 2", wire.LinkProbeInboundStreamsPerPeer, wire.LinkProbeOutboundStreamsPerPeer)
	}
	for _, tc := range []struct {
		name            string
		new             func() (network.ResourceManager, error)
		inbound, outbou int
	}{
		{"node", newResourceManager, wire.LinkProbeInboundStreamsPerPeer, wire.LinkProbeOutboundStreamsPerPeer},
		// A relay only answers: it never opens a probe stream (spec §16.1).
		{"relay", newRelayResourceManager, wire.LinkProbeInboundStreamsPerPeer, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rm, err := tc.new()
			if err != nil {
				t.Fatal(err)
			}
			checkProbeLimits(t, rm, tc.inbound, tc.outbou)
		})
	}
}

func checkProbeLimits(t *testing.T, rm network.ResourceManager, inbound, outbound int) {
	t.Helper()
	defer func() { _ = rm.Close() }()
	remote := peer.ID("remote")
	addr := ma.StringCast("/ip4/1.2.3.4/udp/1/quic-v1")
	conn, err := rm.OpenConnection(network.DirInbound, true, addr)
	if err != nil {
		t.Fatalf("OpenConnection: %v", err)
	}
	defer conn.Done()
	if err := conn.SetPeer(remote); err != nil {
		t.Fatalf("SetPeer: %v", err)
	}
	for _, tc := range []struct {
		dir  network.Direction
		name string
		want int
	}{
		{network.DirInbound, "inbound", inbound},
		{network.DirOutbound, "outbound", outbound},
	} {
		var open []network.StreamManagementScope
		defer func() {
			for _, s := range open {
				s.Done()
			}
		}()
		for i := 1; i <= tc.want+1; i++ {
			s, err := rm.OpenStream(remote, tc.dir)
			if err != nil {
				t.Fatalf("%s stream %d: OpenStream: %v", tc.name, i, err)
			}
			open = append(open, s)
			err = s.SetProtocol(ProtocolProbe)
			if i <= tc.want && err != nil {
				t.Errorf("%s stream %d: SetProtocol = %v, want admitted", tc.name, i, err)
			}
			if i > tc.want && err == nil {
				t.Errorf("%s stream %d: SetProtocol admitted it, want the limit of %d", tc.name, i, tc.want)
			}
		}
	}
}
