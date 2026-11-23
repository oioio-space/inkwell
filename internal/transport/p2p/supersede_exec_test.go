package p2p

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/protocol/syncack"
)

// The executor of rule P-15 (supersede.go) on fake connections, on controlled time: what it does when a proof
// fails here, when its window ends, when the newer connection is lost before a close runs, and under churn.

const proofPeer peer.ID = "peer"

// proofConn is a connection to proofPeer whose proof streams open as open says.
type proofConn struct {
	network.Conn
	open   func(ctx context.Context) (network.Stream, error)
	opens  atomic.Int32
	closes atomic.Int32
	closed atomic.Bool
}

func (c *proofConn) RemotePeer() peer.ID { return proofPeer }
func (c *proofConn) RemoteMultiaddr() ma.Multiaddr {
	return ma.StringCast("/ip4/1.2.3.4/udp/1/quic-v1")
}
func (c *proofConn) IsClosed() bool { return c.closed.Load() }
func (c *proofConn) Close() error {
	c.closes.Add(1)
	c.closed.Store(true)
	return nil
}

func (c *proofConn) NewStream(ctx context.Context) (network.Stream, error) {
	c.opens.Add(1)
	return c.open(ctx)
}

// silentConn's proof never gets a byte: its stream does not open before the window ends.
func silentConn() *proofConn {
	return &proofConn{open: func(ctx context.Context) (network.Stream, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
}

// refusedConn's proof cannot start here: the node's own resource manager refuses the stream.
func refusedConn() *proofConn {
	return &proofConn{open: func(context.Context) (network.Stream, error) {
		return nil, errors.New("cannot reserve stream: resource limit exceeded")
	}}
}

// proofNode is the part of a Node the executor uses; it stops with the test.
func proofNode(t *testing.T) *Node {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	n := &Node{ctx: ctx, born: time.Now(), proofs: newProofSet()}
	t.Cleanup(func() {
		cancel()
		n.proofs.close()
		n.wg.Wait()
	})
	return n
}

// P-15: a proof that fails here (a local refusal) gives no verdict: the connection is not judged by it, so it is
// neither closed at T_vie nor proved again until a newer connection registers.
func TestSupersedeLocalFailureNoVerdict(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := proofNode(t)
		older, newer := refusedConn(), silentConn()
		n.supersedeRegister(older)
		n.supersedeRegister(newer)
		n.supersedeAcked(newer)
		time.Sleep(syncack.ProofWindow + time.Second)
		synctest.Wait()
		if older.closes.Load() > 0 {
			t.Fatalf("P-15 live connection closed on a local failure: the proof could not start here, yet the connection was closed at T_vie")
		}
		if k := older.opens.Load(); k != 1 {
			t.Fatalf("P-15 proof retried after a local failure: %d attempts, want 1 (no verdict until a newer registration)", k)
		}
		if n.proving(proofPeer) {
			t.Fatalf("H-15 waits for a proof that ended without a verdict")
		}
	})
}

// P-15 and H-15: a proof ends at T_vie; a silent connection is no longer « proving », so H-15 does not wait for
// ever when the newer connection is never acknowledged; the silent one stays open as the fallback.
func TestSupersedeSilentProofEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := proofNode(t)
		older, newer := silentConn(), silentConn()
		n.supersedeRegister(older)
		n.supersedeRegister(newer)
		synctest.Wait()
		if !n.proving(proofPeer) {
			t.Fatalf("H-15 does not wait for a pending proof")
		}
		time.Sleep(syncack.ProofWindow + time.Second)
		synctest.Wait()
		if n.proving(proofPeer) {
			t.Fatalf("H-15 waits for a finished proof: the silent connection's proof ended at T_vie")
		}
		if !n.silent(proofPeer, older) {
			t.Fatalf("H-15: the silent connection is not reported dead (a dead circuit is no duplicate)")
		}
		if older.closes.Load() > 0 {
			t.Fatalf("P-15 fallback closed: no newer connection is acknowledged")
		}
	})
}

// P-15: a close is decided against the connections as they are when it runs. The newer connection was lost
// (libp2p closed it; its notification has not reached the node yet), so the silent older one is now the sole
// connection and must not be closed.
func TestSupersedeCloseRechecksNewer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := proofNode(t)
		older, newer := silentConn(), silentConn()
		n.supersedeRegister(older)
		n.supersedeRegister(newer)
		n.supersedeAcked(newer)
		newer.closed.Store(true)
		time.Sleep(syncack.ProofWindow + time.Second)
		synctest.Wait()
		if older.closes.Load() > 0 {
			t.Fatalf("P-15 sole connection closed: the newer connection was already lost when the close ran")
		}
	})
}

// P-15 under churn: a peer that registers connection after connection makes at most one proof in flight per
// older connection; a pending proof is never restarted by a newer registration.
func TestSupersedeChurnOneProofPerConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := proofNode(t)
		conns := []*proofConn{silentConn()}
		n.supersedeRegister(conns[0])
		for range 6 {
			time.Sleep(100 * time.Millisecond)
			c := silentConn()
			conns = append(conns, c)
			n.supersedeRegister(c)
		}
		synctest.Wait()
		for i, c := range conns[:len(conns)-1] {
			if k := c.opens.Load(); k != 1 {
				t.Errorf("P-15 proof restarted while one runs: connection %d proved %d times, want 1", i, k)
			}
		}
		if k := conns[len(conns)-1].opens.Load(); k != 0 {
			t.Errorf("P-15 newest connection proved %d times, want 0", k)
		}
	})
}
