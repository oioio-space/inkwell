package p2p_test

// Block PU1 (spec §8.7, plan docs/plans/2026-11-16-pu1-peerup-after-acceptance.md, gate gates/PU1.toml):
// a node announces a peer only after the peer's ACK on its outbound sync stream. Each arm names its
// rule; a failure reads « PU1 PeerUp without acceptance » for an announcement (or a move) the rules
// forbid, and « PU1 session not up » for a session the rules allow that is missing or late.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p"
	p2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoremem"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

const (
	pu1Forbidden = "PU1 PeerUp without acceptance"
	pu1NotUp     = "PU1 session not up"
	ackByte      = 0x01 // spec §8.7.2, written here from the text, as a remote that is not a Node would
)

// rawHost is a bare libp2p host with the identity of keySeed on the public endpoint of endSeed: a
// remote that does exactly what its test makes it do, and nothing a Node would.
func (nn *natNet) rawHost(t *testing.T, keySeed, endSeed byte) host.Host {
	t.Helper()
	return nn.rawHostWith(t, keySeed, endSeed)
}

// rawHostWith is rawHost with extra libp2p options.
func (nn *natNet) rawHostWith(t *testing.T, keySeed, endSeed byte, extra ...libp2p.Option) host.Host {
	t.Helper()
	priv, err := p2pcrypto.UnmarshalEd25519PrivateKey(key(keySeed))
	if err != nil {
		t.Fatal(err)
	}
	h, err := libp2p.New(append(append(nn.simOptions(nn.newEnd(endSeed, true)), libp2p.Identity(priv), libp2p.DisableRelay()), extra...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func nameOf(h host.Host) transport.PeerID { return transport.PeerID(h.ID().String()) }

func addrStrings(h host.Host) []string {
	out := make([]string, 0, len(h.Addrs()))
	for _, a := range h.Addrs() {
		out = append(out, a.String())
	}
	return out
}

func libp2pID(t *testing.T, n transport.Transport) peer.ID {
	t.Helper()
	id, ok := p2p.LibP2PID(n.Local())
	if !ok {
		t.Fatalf("LibP2PID(%s)", n.Local())
	}
	return id
}

// serve makes h accept sync streams (protocol id) and write the bytes, then read to the end.
func serve(h host.Host, id protocol.ID, write ...byte) {
	h.SetStreamHandler(id, func(s network.Stream) {
		if len(write) > 0 {
			_, _ = s.Write(write)
		}
		_, _ = io.Copy(io.Discard, s)
		_ = s.Reset()
	})
}

// recorded is an event seen by a recorder, with its time.
type recorded struct {
	transport.Event
	at time.Time
}

// recorder consumes every event of a node until the test ends; a test that records a node never
// calls Recv on it (recvWithin, expectEvent) after that.
type recorder struct {
	mu  sync.Mutex
	evs []recorded
}

func record(t *testing.T, n transport.Transport) *recorder {
	r := &recorder{}
	go func() {
		for {
			ev, err := n.Recv(t.Context())
			if err != nil {
				return
			}
			r.mu.Lock()
			r.evs = append(r.evs, recorded{Event: ev, at: time.Now()})
			r.mu.Unlock()
		}
	}()
	return r
}

// of returns the events of kind about peer.
func (r *recorder) of(kind transport.Kind, peer transport.PeerID) []recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recorded
	for _, e := range r.evs {
		if e.Kind == kind && e.Peer == peer {
			out = append(out, e)
		}
	}
	return out
}

// others returns the events that are not Messages.
func (r *recorder) others() []recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(r.evs), func(e recorded) bool { return e.Kind == transport.Message })
}

// noUp fails the test if n announced peer.
func noUp(t *testing.T, r *recorder, n, peer transport.PeerID, why string) {
	t.Helper()
	if ups := r.of(transport.PeerUp, peer); len(ups) > 0 {
		t.Errorf("%s: %s announced %s (%d PeerUp) %s", pu1Forbidden, n, peer, len(ups), why)
	}
}

// connLife records when h first registered a connection to p, and when it last lost one.
type connLife struct {
	mu       sync.Mutex
	up, down time.Time
}

func watchConn(h host.Host, p peer.ID) *connLife {
	l := &connLife{}
	h.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(_ network.Network, c network.Conn) {
			if c.RemotePeer() == p {
				l.mu.Lock()
				if l.up.IsZero() {
					l.up = time.Now()
				}
				l.mu.Unlock()
			}
		},
		DisconnectedF: func(_ network.Network, c network.Conn) {
			if c.RemotePeer() == p {
				l.mu.Lock()
				l.down = time.Now()
				l.mu.Unlock()
			}
		},
	})
	return l
}

func (l *connLife) times() (up, down time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.up, l.down
}

// noConn fails the test if h holds a connection to p.
func noConn(t *testing.T, h host.Host, p peer.ID, why string) {
	t.Helper()
	if cs := h.Network().ConnsToPeer(p); len(cs) > 0 {
		t.Errorf("%s still holds %d connections to %s %s", h.ID(), len(cs), p, why)
	}
}

// closedAtBudget checks that the connection l watched was closed syncack.Budget (15 s, spec §8.7.4)
// after it was registered, ± 10 ms.
func closedAtBudget(t *testing.T, l *connLife, what string) {
	t.Helper()
	const budget, slack = syncack.Budget, 10 * time.Millisecond
	up, down := l.times()
	switch {
	case up.IsZero():
		t.Fatalf("%s: no connection was registered", what)
	case down.IsZero():
		t.Errorf("%s: the connection is still open %v after it was registered, want closed at %v", what, time.Since(up), budget)
	case down.Sub(up) < budget-slack || down.Sub(up) > budget+slack:
		t.Errorf("%s: the connection was closed %v after it was registered, want %v ± %v", what, down.Sub(up), budget, slack)
	}
}

// hideSync is a peerstore that never lists the sync protocol: a host using it negotiates every sync
// stream with multistream-select instead of opening it optimistically.
type hideSync struct {
	peerstore.Peerstore
	peerstore.CertifiedAddrBook // the host requires it
}

func (h hideSync) SupportsProtocols(p peer.ID, ids ...protocol.ID) ([]protocol.ID, error) {
	got, err := h.Peerstore.SupportsProtocols(p, ids...)
	return slices.DeleteFunc(got, func(id protocol.ID) bool { return id == p2p.ProtocolSync }), err
}

func (h hideSync) FirstSupportedProtocol(p peer.ID, ids ...protocol.ID) (protocol.ID, error) {
	got, err := h.SupportsProtocols(p, ids...)
	if err != nil || len(got) == 0 {
		return "", err
	}
	return got[0], nil
}

// negotiating is the libp2p option that makes a node negotiate its sync streams (hideSync).
func negotiating(t *testing.T) libp2p.Option {
	t.Helper()
	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ps.Close() }) // the host does not close it when New fails
	return libp2p.Peerstore(hideSync{Peerstore: ps, CertifiedAddrBook: ps})
}

// mssToken reads one multistream-select token (uvarint length, then the bytes, '\n' included) and
// returns it raw and as text.
func mssToken(r *bufio.Reader) (raw []byte, text string, err error) {
	n, err := binary.ReadUvarint(r)
	if err != nil || n == 0 || n > 1024 {
		return nil, "", fmt.Errorf("token length %d: %w", n, err)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, "", err
	}
	return append(binary.AppendUvarint(nil, n), b...), string(bytes.TrimSuffix(b, []byte("\n"))), nil
}

// replayed is a stream whose first bytes are read again from r.
type replayed struct {
	network.Stream
	r io.Reader
}

func (s *replayed) Read(p []byte) (int, error) { return s.r.Read(p) }

// slowSync makes h answer a sync stream's multistream-select only echo after the stream arrives,
// and write the ACK ack after that; h's own protocols are served as usual. The sync protocol is not
// in h's protocol list, so a peer negotiates it.
func slowSync(h host.Host, echo, ack time.Duration) {
	h.Network().SetStreamHandler(func(s network.Stream) {
		br := bufio.NewReader(s)
		hdr, _, err1 := mssToken(br)
		proto, name, err2 := mssToken(br)
		if err := errors.Join(err1, err2); err != nil {
			_ = s.Reset()
			return
		}
		if name == p2p.ProtocolSync {
			time.Sleep(echo)
			_, _ = s.Write(slices.Concat(hdr, proto))
			time.Sleep(ack)
			_, _ = s.Write([]byte{ackByte})
			_, _ = io.Copy(io.Discard, br)
			_ = s.Reset()
			return
		}
		rs := &replayed{Stream: s, r: io.MultiReader(bytes.NewReader(slices.Concat(hdr, proto)), br)}
		id, handle, err := h.Mux().Negotiate(rs)
		if err != nil {
			_ = s.Reset()
			return
		}
		_ = s.SetProtocol(id)
		_ = handle(id, rs)
	})
}

// openRaw opens a sync stream from the raw host h to p and reads its first byte within d.
func openRaw(t *testing.T, h host.Host, p peer.ID, d time.Duration) (b byte, n int, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), d)
	defer cancel()
	s, err := h.NewStream(ctx, p, p2p.ProtocolSync)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = s.Reset() }()
	_ = s.SetReadDeadline(time.Now().Add(d))
	var buf [1]byte
	n, err = s.Read(buf[:])
	return buf[0], n, err
}

// The node speaks the reference's protocol (spec §8.7.8; the gate wants the literal in p2p).
func TestSyncAckReference(t *testing.T) {
	if p2p.ProtocolSync != syncack.ProtocolID {
		t.Errorf("ProtocolSync %s, the reference's is %s", p2p.ProtocolSync, syncack.ProtocolID)
	}
}

func TestPeerUpAfterAck(t *testing.T) {
	// P-1: HP1r's trace. b holds a's protocol from an earlier session (identify), so its next
	// NewStream to a is optimistic; a closes every new connection as it registers it. On main's code
	// b announced a on NewStream's return, then lost it ~10 ms later.
	t.Run("closed_at_once_by_remote", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(1)
			a := nn.nodeAt(t, 1, true, p2p.Options{})
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			if err := b.Connect(t.Context(), a.Local(), a.Addrs()); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			expectUp(t, a, b)
			aID := libp2pID(t, a)
			_ = a.Close()
			expectEventWithin(t, b, transport.PeerDown, a.Local(), time.Minute)
			if got, _ := b.Host().Peerstore().SupportsProtocols(aID, p2p.ProtocolSync); len(got) == 0 {
				t.Fatal("b no longer lists a's sync protocol: the optimistic path of the trace is not set up")
			}
			raw := nn.rawHost(t, 1, 11) // a's identity at another address
			raw.Network().Notify(&network.NotifyBundle{ConnectedF: func(_ network.Network, c network.Conn) { _ = c.Close() }})
			serve(raw, p2p.ProtocolSync, ackByte)
			rb := record(t, b)
			_ = b.Connect(t.Context(), a.Local(), addrStrings(raw)) // may succeed on b's side
			time.Sleep(time.Second)
			noUp(t, rb, b.Local(), a.Local(), "on a connection a closed as it registered it")
			noConn(t, b.Host(), aID, "1 s after a closed it")
		})
	})

	// P-1 with P-2: a admits b at the gate, then no longer (one-sided admission): a resets b's stream
	// without an ACK. Neither side announces the other, and b closes the connection.
	t.Run("one_sided_admission", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(2)
			// a's gate is the first to ask (InterceptSecured); every later check of a refuses b, its own
			// open included, so a never opens to b and resets b's stream.
			var asked atomic.Int32
			a := nn.nodeAt(t, 1, true, p2p.Options{Admit: func(transport.PeerID) bool { return asked.Add(1) == 1 }})
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			ra, rb := record(t, a), record(t, b)
			_ = b.Connect(t.Context(), a.Local(), a.Addrs())
			time.Sleep(5 * time.Second)
			noUp(t, rb, b.Local(), a.Local(), "whose stream a reset without an ACK")
			noUp(t, ra, a.Local(), b.Local(), "which a does not admit")
			noConn(t, b.Host(), libp2pID(t, a), "5 s after a refused its stream")
		})
	})

	// P-2 step 2: a responder that does not admit the opener resets its stream without writing.
	t.Run("not_admitted_remote", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(3)
			var refuse atomic.Bool
			b := nn.nodeAt(t, 2, true, p2p.Options{Admit: func(transport.PeerID) bool { return !refuse.Load() }})
			b.Host().Network().Notify(&network.NotifyBundle{ConnectedF: func(network.Network, network.Conn) { refuse.Store(true) }})
			raw := nn.rawHost(t, 3, 13)
			if err := raw.Connect(t.Context(), peer.AddrInfo{ID: libp2pID(t, b), Addrs: b.Host().Addrs()}); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			got, n, err := openRaw(t, raw, libp2pID(t, b), 5*time.Second)
			switch {
			case n == 1:
				t.Errorf("%s: b wrote 0x%02x to a peer it does not admit", pu1Forbidden, got)
			case err == nil || isTimeout(err):
				t.Errorf("b neither wrote nor reset the stream of a peer it does not admit: %v", err)
			}
		})
	})

	// P-2 step 3: a responder writes no ACK on a connection it is closing (it closes the opener's
	// connections as it registers them; the opener's stream is optimistic, its protocol known from an
	// earlier session).
	t.Run("ack_needs_live_conn", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(4)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			serve(raw, p2p.ProtocolSync, ackByte)
			bID := libp2pID(t, b)
			if err := raw.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()}); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			expectEventWithin(t, b, transport.PeerUp, nameOf(raw), wait)
			_ = raw.Network().ClosePeer(bID)
			expectEventWithin(t, b, transport.PeerDown, nameOf(raw), wait)
			if got, _ := raw.Peerstore().SupportsProtocols(bID, p2p.ProtocolSync); len(got) == 0 {
				t.Fatal("the raw host no longer lists b's sync protocol: its next stream would not be optimistic")
			}
			b.Host().Network().Notify(&network.NotifyBundle{ConnectedF: func(_ network.Network, c network.Conn) {
				if c.RemotePeer() == raw.ID() {
					_ = c.Close()
				}
			}})
			rb := record(t, b)
			_ = raw.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()})
			if got, n, _ := openRaw(t, raw, bID, 2*time.Second); n == 1 {
				t.Errorf("%s: b wrote 0x%02x on a connection it closed as it registered it", pu1Forbidden, got)
			}
			noUp(t, rb, b.Local(), nameOf(raw), "on a connection b closed as it registered it")
		})
	})

	// P-5: a remote that accepts the stream and never writes: no PeerUp, the connection closed when
	// the 15 s budget runs out.
	t.Run("silent_remote", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(5)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			serve(raw, p2p.ProtocolSync)
			life := watchConn(b.Host(), raw.ID())
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(16 * time.Second)
			noUp(t, rb, b.Local(), nameOf(raw), "which never acknowledged")
			closedAtBudget(t, life, "silent remote")
		})
	})

	// P-5: one budget for NewStream and the ACK. The remote's multistream echo comes 10 s after the
	// stream, its ACK 6 s after that: the connection is closed 15 s after the open, never announced.
	t.Run("slow_negotiation_late_ack", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(6)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			slowSync(raw, 10*time.Second, 6*time.Second)
			life := watchConn(b.Host(), raw.ID())
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(20 * time.Second)
			if slices.Contains(raw.Mux().Protocols(), p2p.ProtocolSync) {
				t.Fatal("the raw host lists the sync protocol: b's stream would not be negotiated")
			}
			noUp(t, rb, b.Local(), nameOf(raw), "whose ACK came 16 s after the open")
			closedAtBudget(t, life, "slow negotiation")
		})
	})

	// P-4: 0x02 is not the ACK.
	t.Run("wrong_ack_byte", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(7)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			serve(raw, p2p.ProtocolSync, 0x02)
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(2 * time.Second)
			noUp(t, rb, b.Local(), nameOf(raw), "which wrote 0x02")
			noConn(t, b.Host(), raw.ID(), "2 s after a wrong ACK")
		})
	})

	// P-4: a byte after the ACK is hostile: PeerUp, then the connection closed within 1 s.
	t.Run("byte_after_ack", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(8)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			serve(raw, p2p.ProtocolSync, ackByte, ackByte)
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(2 * time.Second)
			ups := rb.of(transport.PeerUp, nameOf(raw))
			if len(ups) != 1 {
				t.Fatalf("%s: %d PeerUp of a remote that acknowledged, want 1", pu1NotUp, len(ups))
			}
			downs := rb.of(transport.PeerDown, nameOf(raw))
			if len(downs) != 1 || downs[0].at.Sub(ups[0].at) > time.Second {
				t.Errorf("PeerDown %v after the byte that followed the ACK, want one within 1 s of PeerUp", downs)
			}
			noConn(t, b.Host(), raw.ID(), "after a byte that followed the ACK")
		})
	})

	// P-3: the responder acknowledges before it waits for its own PeerUp of the opener: a remote that
	// never accepts b's stream (so b never announces it) still reads b's ACK on its own.
	t.Run("ack_before_own_up", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(9)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			serve(raw, p2p.ProtocolSync) // accepts b's stream, never acknowledges it
			rb := record(t, b)
			bID := libp2pID(t, b)
			if err := raw.Connect(t.Context(), peer.AddrInfo{ID: bID, Addrs: b.Host().Addrs()}); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			got, n, err := openRaw(t, raw, bID, time.Second)
			if n != 1 || got != ackByte {
				t.Errorf("%s: the remote read %d bytes (0x%02x, %v) within 1 s, want b's ACK before b announces it", pu1NotUp, n, got, err)
			}
			noUp(t, rb, b.Local(), nameOf(raw), "which never acknowledged b's stream")
		})
	})

	// P-3: two nodes, crossed opens, on the optimistic and on the negotiated path: both up, and the
	// session carries frames both ways.
	t.Run("both_sides_up", func(t *testing.T) {
		for _, path := range []string{"cached", "negotiated"} {
			t.Run(path, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					nn := newNatNet(10)
					extra := func() []libp2p.Option {
						if path == "negotiated" {
							return []libp2p.Option{negotiating(t)}
						}
						return nil
					}
					a := nn.nodeOn(t, 1, nn.newEnd(1, true), p2p.Options{}, extra()...)
					b := nn.nodeOn(t, 2, nn.newEnd(2, true), p2p.Options{}, extra()...)
					var wg sync.WaitGroup
					wg.Go(func() { _ = a.Connect(t.Context(), b.Local(), b.Addrs()) })
					wg.Go(func() { _ = b.Connect(t.Context(), a.Local(), a.Addrs()) })
					wg.Wait()
					for _, p := range [][2]*simNode{{a, b}, {b, a}} {
						if ev := recvWithin(t, p[0], wait); ev.Kind != transport.PeerUp || ev.Peer != p[1].Local() {
							t.Fatalf("%s: %s got %v from %s, want PeerUp from %s", pu1NotUp, p[0].Local(), ev.Kind, ev.Peer, p[1].Local())
						}
					}
					for _, p := range [][2]*simNode{{a, b}, {b, a}} {
						if err := p[0].Send(t.Context(), p[1].Local(), []byte("hello")); err != nil {
							t.Fatalf("%s: Send: %v", pu1NotUp, err)
						}
						if ev := recvWithin(t, p[1], wait); ev.Kind != transport.Message || string(ev.Payload) != "hello" {
							t.Fatalf("%s: %s got %v %q, want the Message", pu1NotUp, p[1].Local(), ev.Kind, ev.Payload)
						}
					}
				})
			})
		}
	})

	// P-7: a peer that serves only /inkwell/sync/1.0.0 is refused: no PeerUp, the connection closed.
	t.Run("old_peer_refused", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			nn := newNatNet(11)
			b := nn.nodeAt(t, 2, true, p2p.Options{})
			raw := nn.rawHost(t, 3, 13)
			serve(raw, "/inkwell/sync/1.0.0")
			rb := record(t, b)
			if err := b.Connect(t.Context(), nameOf(raw), addrStrings(raw)); err != nil {
				t.Fatalf("Connect: %v", err)
			}
			time.Sleep(2 * time.Second)
			noUp(t, rb, b.Local(), nameOf(raw), "which serves only /inkwell/sync/1.0.0")
			noConn(t, b.Host(), raw.ID(), "2 s after it found only /inkwell/sync/1.0.0")
		})
	})
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout() || errors.Is(err, context.DeadlineExceeded)
}

func isDirect(c network.Conn) bool {
	_, err := c.RemoteMultiaddr().ValueForProtocol(ma.P_CIRCUIT)
	return err != nil
}

// moveScene is a relayed session between a and b that a hole punch then lets move to a direct
// connection; b's responder runs hook on each inbound sync stream before it acknowledges it.
type moveScene struct {
	a, b   *simNode
	ra, rb *recorder
	sent   uint64
}

func newMoveScene(t *testing.T, seed uint64, configure func(*p2p.Options)) *moveScene {
	t.Helper()
	nn := newNatNet(seed)
	nn.blockDirect(true)
	relays := nn.relay(t, p2p.RelayOptions{})
	a := nn.node(t, 1, p2p.Options{Relays: relays})
	ob := p2p.Options{Relays: relays}
	configure(&ob)
	b := nn.node(t, 2, ob)
	a.Want(map[transport.PeerID][]string{b.Local(): nil})
	expectUp(t, a, b)
	if got := linkTo(a, b.Local()); got != transport.LinkRelayed {
		t.Fatalf("first link a->b = %v, want %v", got, transport.LinkRelayed)
	}
	sc := &moveScene{a: a, b: b, ra: record(t, a), rb: record(t, b)}
	nn.blockDirect(false)
	return sc
}

// send sends the next numbered frame from a to b.
func (sc *moveScene) send(t *testing.T) {
	t.Helper()
	sc.sent++
	if err := sc.a.Send(t.Context(), sc.b.Local(), binary.BigEndian.AppendUint64(nil, sc.sent)); err != nil {
		t.Fatalf("Send frame %d: %v", sc.sent, err)
	}
}

// check waits for b to hold every frame sent, then requires them in order, with no gap, and no
// event but Messages on either side.
func (sc *moveScene) check(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for uint64(len(sc.rb.of(transport.Message, sc.a.Local()))) < sc.sent && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	got := sc.rb.of(transport.Message, sc.a.Local())
	if uint64(len(got)) != sc.sent {
		t.Errorf("b received %d frames, a sent %d", len(got), sc.sent)
	}
	for i, m := range got {
		if len(m.Payload) != 8 || binary.BigEndian.Uint64(m.Payload) != uint64(i+1) {
			t.Fatalf("b's frame %d is % x, want frame %d: the move broke the order", i, m.Payload, i+1)
		}
	}
	for _, r := range []*recorder{sc.ra, sc.rb} {
		if evs := r.others(); len(evs) > 0 {
			t.Errorf("events during the move: %v", evs)
		}
	}
}

func beforeAck(f func(network.Conn) bool) func(*p2p.Options) {
	return func(o *p2p.Options) { p2p.SetBeforeAck(o, f) }
}

func TestPeerUpMoveAfterAck(t *testing.T) {
	const holdAck = 2 * time.Second

	// P-6: the session moves to the direct connection only after the ACK of the new stream (b holds
	// it 2 s), and its frames keep their order across the move.
	t.Run("move_waits_for_ack", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var mu sync.Mutex
			var asked, acked time.Time
			sc := newMoveScene(t, 1, beforeAck(func(c network.Conn) bool {
				if !isDirect(c) {
					return true
				}
				mu.Lock()
				asked = time.Now()
				mu.Unlock()
				time.Sleep(holdAck)
				mu.Lock()
				acked = time.Now()
				mu.Unlock()
				return true
			}))
			var moved time.Time
			for deadline := time.Now().Add(2 * time.Minute); moved.IsZero() || time.Since(moved) < 5*time.Second; {
				if time.Now().After(deadline) {
					t.Fatalf("%s: a's session still relayed after 2 minutes", pu1NotUp)
				}
				sc.send(t)
				if direct, ok := sc.a.OutDirect(sc.b.Local()); ok && direct && moved.IsZero() {
					moved = time.Now()
				}
				time.Sleep(10 * time.Millisecond)
			}
			mu.Lock()
			defer mu.Unlock()
			switch {
			case asked.IsZero():
				t.Fatal("b's responder never saw a's direct stream")
			case acked.IsZero() || moved.Before(acked):
				t.Errorf("%s: a moved its session to the direct stream %v after b was asked, before b's ACK (%v after)",
					pu1Forbidden, moved.Sub(asked), holdAck)
			}
			sc.check(t)
		})
	})

	// P-6: the order across the move does not depend on when the old stream's handler runs. b's handler
	// of a's first (relayed) stream is held right after its ACK, before it reads, while a moves its
	// session to the direct stream and sends on it: b must still deliver the relayed frames first.
	t.Run("move_keeps_order_across_late_handler", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			release := make(chan struct{})
			var held atomic.Bool
			sc := newMoveScene(t, 3, func(o *p2p.Options) {
				p2p.SetAfterAck(o, func(c network.Conn) {
					if !isDirect(c) && held.CompareAndSwap(false, true) {
						<-release
					}
				})
			})
			if !held.Load() {
				t.Fatal("b's handler of the relayed stream was not held after its ACK")
			}
			var moved time.Time
			for deadline := time.Now().Add(2 * time.Minute); moved.IsZero() || time.Since(moved) < 2*time.Second; {
				if time.Now().After(deadline) {
					close(release)
					t.Fatalf("%s: a's session still relayed after 2 minutes", pu1NotUp)
				}
				sc.send(t)
				if direct, ok := sc.a.OutDirect(sc.b.Local()); ok && direct && moved.IsZero() {
					moved = time.Now()
				}
				time.Sleep(10 * time.Millisecond)
			}
			close(release)
			sc.check(t)
		})
	})

	// P-6: b refuses the direct stream: a's session stays on the circuit, with no event either side.
	t.Run("move_refused_keeps_circuit", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var refused atomic.Int32
			sc := newMoveScene(t, 2, beforeAck(func(c network.Conn) bool {
				if isDirect(c) {
					refused.Add(1)
					return false
				}
				return true
			}))
			deadline := time.Now().Add(2 * time.Minute)
			for refused.Load() == 0 {
				if time.Now().After(deadline) {
					t.Fatal("a never opened a direct stream to b")
				}
				sc.send(t)
				time.Sleep(50 * time.Millisecond)
			}
			for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
				sc.send(t)
				if direct, ok := sc.a.OutDirect(sc.b.Local()); !ok || direct {
					t.Fatalf("%s: a's session is on a direct stream b refused (open %v)", pu1Forbidden, ok)
				}
				time.Sleep(50 * time.Millisecond)
			}
			sc.check(t)
		})
	})
}

// TestPeerUpLatency is the paired measurement of plan §3 « Cost »: in one run on controlled time (RTT
// R = 100 ms, no jitter), the delay from a's Connect to its PeerUp of b, with the ACK and without it
// (SetSkipAck). On the optimistic path the ACK costs exactly one more round trip; on the negotiated
// path it rides the multistream echo.
func TestPeerUpLatency(t *testing.T) {
	const oneWay = 50 * time.Millisecond
	const rtt, slack = 2 * oneWay, 2 * time.Millisecond
	measure := func(t *testing.T, nn *natNet, seed byte, skip bool, extra func() []libp2p.Option) time.Duration {
		var oa p2p.Options
		if skip {
			p2p.SetSkipAck(&oa)
		}
		a := nn.nodeOn(t, 1, nn.newEnd(seed, true), oa, extra()...)
		b := nn.nodeOn(t, 2, nn.newEnd(seed+1, true), p2p.Options{}, extra()...)
		defer func() { _ = a.Close(); _ = b.Close() }()
		t0 := time.Now()
		if err := a.Connect(t.Context(), b.Local(), b.Addrs()); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		if ev := recvWithin(t, a, wait); ev.Kind != transport.PeerUp || ev.Peer != b.Local() {
			t.Fatalf("%s: a got %v from %s, want PeerUp from b", pu1NotUp, ev.Kind, ev.Peer)
		}
		return time.Since(t0)
	}
	for _, c := range []struct {
		path     string
		min, max time.Duration
		extra    func(t *testing.T) []libp2p.Option
	}{
		{"cached", rtt, rtt + slack, func(*testing.T) []libp2p.Option { return nil }},
		{"negotiated", 0, rtt + slack, func(t *testing.T) []libp2p.Option { return []libp2p.Option{negotiating(t)} }},
	} {
		t.Run(c.path, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				nn := newNatNet(1)
				nn.setJitter(time.Nanosecond) // Int64N needs > 0: every packet takes exactly oneWay
				nn.setLatency(oneWay - baseDelay)
				extra := func() []libp2p.Option { return c.extra(t) }
				with := measure(t, nn, 1, false, extra)
				without := measure(t, nn, 3, true, extra)
				d := with - without
				t.Logf("PU1 latency, %s path, RTT %v: PeerUp after %v with the ACK, %v without, paired difference %v", c.path, rtt, with, without, d)
				if d < c.min || d > c.max {
					t.Errorf("PU1 latency: %s path, the ACK delays PeerUp by %v, want %v to %v", c.path, d, c.min, c.max)
				}
			})
		})
	}
}
