//go:build server

package p2p

import (
	"context"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	msmux "github.com/multiformats/go-multistream"

	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

// ProtocolFill is the netsim filler's stream. It exists only in the `-tags server` build, the one the
// network-simulation tests run (plan C5S, harness 3): typing gives about 8 packets a second, far
// under the 3000 packets per 30 s window that rule R2 needs, and typing faster trips the bridge's
// « peer does not read » reset. The filler is a fixed-rate source of packets that goes around the
// bridge: it carries nothing and changes nothing the product does outside netsim.
const ProtocolFill = "/inkwell/netsim-fill/1.0.0"

// The filler's rate: one 200-byte message every 1/120 s per direction, three times faster during the
// first fillEntry of a connection (phase P0 of scenario 5).
const (
	fillSize  = 200
	fillEvery = time.Second / 120
	fillEntry = 40 * time.Second
	fillBoost = 3
)

// fillRegister makes the node read and discard the filler of its peers.
func (n *Node) fillRegister() {
	n.h.SetStreamHandler(ProtocolFill, func(s network.Stream) {
		defer func() { _ = s.Reset() }()
		_, _ = io.Copy(io.Discard, s)
	})
}

// fillAttach starts the filler on a raw QUIC connection to a member, the connection that carries the
// session (a TCP connection or a circuit has no packets of its own to count, and carries nothing
// here). The peer may become a member after the connection, so it retries every second.
func (n *Node) fillAttach(c network.Conn) {
	if isCircuit(c.RemoteMultiaddr()) {
		return
	}
	if _, ok := quicStats(c); !ok {
		return
	}
	n.probes.spawn(func() { n.fill(n.ctx, c) })
}

func (n *Node) fill(ctx context.Context, c network.Conn) {
	var s network.Stream
	for s == nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		if c.IsClosed() {
			return
		}
		if n.probeRoles(c)&linkprobe.Member == 0 { // not a member (yet)
			continue
		}
		st, err := c.NewStream(ctx)
		if err != nil {
			continue
		}
		if err := msmux.SelectProtoOrFail(ProtocolFill, st); err != nil || st.SetProtocol(protocol.ID(ProtocolFill)) != nil {
			_ = st.Reset()
			continue
		}
		s = st
	}
	defer func() { _ = s.Reset() }()
	start := time.Now()
	msg := make([]byte, fillSize)
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if _, err := s.Write(msg); err != nil {
			return
		}
		d := fillEvery
		if time.Since(start) < fillEntry {
			d /= fillBoost
		}
		t.Reset(d)
	}
}
