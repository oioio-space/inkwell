package p2p

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	basic "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch/pb"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/libp2p/go-msgio/pbio"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"go.uber.org/fx"
)

const (
	punchRounds  = 3
	punchMsgSize = 4 << 10
)

// punchService runs the libp2p DCUtR exchange with local ownership of grants.
// The fork's address and event callbacks carry only a PeerID, so they cannot
// associate a refused stream or a cancelled SYNC wait with a grant.
type punchService struct {
	ctx   context.Context
	host  host.Host
	ids   identify.IDService
	gate  *punchGate
	addrs func() []ma.Multiaddr
	ready chan struct{}
	// onRound and onAnswerDial are Options' (tests).
	onRound      func(network.Conn)
	onAnswerDial func(peer.ID)

	mu     sync.Mutex
	active map[peer.ID]struct{}
}

func newPunchService(ctx context.Context, h host.Host, ids identify.IDService, g *punchGate, addrs func() []ma.Multiaddr) *punchService {
	return &punchService{
		ctx: ctx, host: h, ids: ids, gate: g, addrs: addrs,
		ready: make(chan struct{}), active: make(map[peer.ID]struct{}),
	}
}

// punchServiceOption uses the same address sources as libp2p's HolePunchAddrs,
// including singly observed addresses and the host's configured address factory.
func (n *Node) punchServiceOption() libp2p.Option {
	return func(cfg *libp2p.Config) error {
		factory := cfg.AddrsFactory
		return libp2p.WithFxOption(fx.Invoke(func(h *basic.BasicHost, observed basic.ObservedAddrsManager) {
			s := newPunchService(n.ctx, h, h.IDService(), n.pgate, func() []ma.Multiaddr {
				addrs := h.AllAddrs()
				if factory != nil {
					addrs = slices.Clone(factory(addrs))
				}
				if observed != nil {
					addrs = append(addrs, observed.Addrs(1)...)
				}
				return addrs
			})
			s.onRound, s.onAnswerDial = n.opts.onRound, n.opts.onAnswerDial
			n.hps.Store(s)
		}))(cfg)
	}
}

// serve registers DCUtR once the host can offer a public address, on the
// readiness schedule used by libp2p.
func (s *punchService) serve() {
	for delay := 250 * time.Millisecond; s.ctx.Err() == nil; delay = min(delay*2, 5*time.Second) {
		if len(selectAddrs(s.addrs())) != 0 {
			s.host.SetStreamHandler(holepunch.Protocol, s.answer)
			close(s.ready)
			return
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
	}
}

// autoPunch makes the usual one-shot after identify on a new inbound circuit c, its rounds on c (H-20).
func (n *Node) autoPunch(c network.Conn) {
	s := n.hps.Load()
	if s == nil || n.opts.noAutoPunch || c.Stat().Direction != network.DirInbound {
		return
	}
	select {
	case <-s.ready:
	default:
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.closed {
		n.wg.Go(func() {
			select {
			case <-s.ids.IdentifyWait(c):
				if !c.IsClosed() {
					_ = n.punchOn(n.ctx, s, c.RemotePeer(), c)
				}
			case <-n.ctx.Done():
			}
		})
	}
}

// punchOn runs s's call to p bound to the circuit via (rule H-20): its rounds open on via, never on the
// connection libp2p would pick (the one with the most streams, the oldest, dead for 30 s after a crash of the
// peer), and the call ends when via closes; it is then classed on what it did.
func (n *Node) punchOn(ctx context.Context, s *punchService, p peer.ID, via network.Conn) error {
	if via == nil {
		return errNoCircuit
	}
	life, done := n.connContext(via)
	defer done()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(life, cancel)()
	// The call starts once the node has handled identify on via, as libp2p's one-shot waits for identify. Then
	// the peerstore holds the addresses the peer announced on via (after a crash of the peer it held only its
	// dead process's, and the direct dial spent its timeout on them), and so does the record the punch gate
	// admits the dial's destinations from (n.heard, written from identify's event, after libp2p's own wait
	// ends: a dial started in between got a grant that admitted no address, H-16).
	if s.ids != nil {
		select {
		case <-n.identifiedOn(via):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.connect(ctx, p, via)
}

// errNoCircuit is the call of a policy that has no inbound circuit to open its rounds on (H-20).
var errNoCircuit = errors.New("p2p: no inbound circuit for a DCUtR round")

// connect is one DCUtR call to p: a direct dial to the public addresses the peerstore holds, then up to
// punchRounds rounds, each opened on via (H-20).
func (s *punchService) connect(ctx context.Context, p peer.ID, via network.Conn) error {
	select {
	case <-s.ready:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return holepunch.ErrClosed
	}
	s.mu.Lock()
	if _, busy := s.active[p]; busy {
		s.mu.Unlock()
		return holepunch.ErrHolePunchActive
	}
	s.active[p] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.active, p)
		s.mu.Unlock()
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(s.ctx, cancel)()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, c := range s.host.Network().ConnsToPeer(p) {
		if !isCircuit(c.RemoteMultiaddr()) && !c.IsClosed() {
			return nil
		}
	}
	if slices.ContainsFunc(s.host.Peerstore().Addrs(p), func(a ma.Multiaddr) bool {
		return !isCircuit(a) && manet.IsPublicAddr(a)
	}) {
		s.gate.beginDemand(p)
		defer s.gate.endDemand(p)
		err := s.dial(ctx, peer.AddrInfo{ID: p})
		s.gate.endPreliminaryDial(p)
		if err == nil {
			return nil
		}
	}

	streamCtx := network.WithNoDial(network.WithAllowLimitedConn(ctx, "dcutr"), "dcutr")
	for i := range punchRounds {
		str, err := newStreamOn(streamCtx, via, holepunch.Protocol)
		if err != nil {
			return cmp.Or(ctx.Err(), err)
		}
		if s.onRound != nil {
			s.onRound(str.Conn())
		}
		retry, err := s.round(ctx, str, true, i == punchRounds-1)
		if err == nil || !retry || ctx.Err() != nil {
			return cmp.Or(ctx.Err(), err)
		}
	}
	return fmt.Errorf("p2p: all hole punch rounds with %s failed", p)
}

func (s *punchService) answer(str network.Stream) {
	c := str.Conn()
	if c.Stat().Direction == network.DirInbound || !isCircuit(c.RemoteMultiaddr()) {
		_ = str.Reset()
		return
	}
	_, _ = s.round(s.ctx, str, false, true)
}

// round owns its grant through the stream exchange, the optional half-RTT
// wait and the direct dial. Only a failed punch dial allows the initiator's
// next round. Failures during admission or exchange end the call.
func (s *punchService) round(ctx context.Context, str network.Stream, initiator, client bool) (bool, error) {
	var round *punchRound
	var exchanged bool
	p := str.Conn().RemotePeer()
	defer func() {
		if !exchanged {
			_ = str.Reset()
		}
		if round != nil {
			s.gate.release(p, round.until)
		}
	}()
	// Cancellation must also terminate a stream while waiting for admission.
	stop := context.AfterFunc(ctx, func() { _ = str.Reset() })
	defer stop()
	if err := str.Scope().SetService(holepunch.ServiceName); err != nil {
		return false, err
	}
	round = s.gate.beginRound(ctx, p, s.addrs())
	if round == nil {
		return false, cmp.Or(ctx.Err(), errors.New("p2p: DCUtR round refused"))
	}
	ctx, cancel := context.WithDeadline(ctx, round.until)
	defer cancel()
	if len(round.local) == 0 {
		return false, errors.New("p2p: no local DCUtR addresses")
	}
	remote, wait, err := s.exchange(ctx, str, p, round, initiator)
	if err != nil {
		return false, cmp.Or(ctx.Err(), err)
	}
	_ = str.Close()
	exchanged = true
	if initiator {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !initiator && s.onAnswerDial != nil {
		s.onAnswerDial(p)
	}
	ctx = network.WithSimultaneousConnect(ctx, client, "hole-punching")
	return true, s.dial(ctx, peer.AddrInfo{ID: p, Addrs: remote})
}

// exchange uses the pinned libp2p protobuf and message bound for CONNECT and
// SYNC. The initiator waits half the measured round trip before dialing.
func (s *punchService) exchange(ctx context.Context, str network.Stream, p peer.ID, round *punchRound, initiator bool) ([]ma.Multiaddr, time.Duration, error) {
	scope := str.Scope()
	if err := scope.ReserveMemory(punchMsgSize, network.ReservationPriorityAlways); err != nil {
		return nil, 0, err
	}
	defer scope.ReleaseMemory(punchMsgSize)
	deadline := time.Now().Add(holepunch.StreamTimeout)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	if err := str.SetDeadline(deadline); err != nil {
		return nil, 0, err
	}
	rd := pbio.NewDelimitedReader(str, punchMsgSize)
	wr := pbio.NewDelimitedWriter(str)
	addrs := make([][]byte, len(round.local))
	for i, a := range round.local {
		addrs[i] = a.Bytes()
	}
	connect := &pb.HolePunch{Type: pb.HolePunch_CONNECT.Enum(), ObsAddrs: addrs}
	start := time.Now()
	if initiator {
		if err := wr.WriteMsg(connect); err != nil {
			return nil, 0, err
		}
	}
	var msg pb.HolePunch
	if err := rd.ReadMsg(&msg); err != nil {
		return nil, 0, err
	}
	wait := time.Since(start) / 2
	if msg.GetType() != pb.HolePunch_CONNECT {
		return nil, 0, errors.New("p2p: expected DCUtR CONNECT")
	}
	var candidates []ma.Multiaddr
	for _, addr := range msg.GetObsAddrs() {
		if a, err := ma.NewMultiaddrBytes(addr); err == nil {
			candidates = append(candidates, a)
		}
	}
	remote := s.gate.filterRemote(p, round, candidates)
	if len(remote) == 0 {
		return nil, 0, errors.New("p2p: no remote DCUtR addresses")
	}
	if initiator {
		return remote, wait, wr.WriteMsg(&pb.HolePunch{Type: pb.HolePunch_SYNC.Enum()})
	}
	if err := wr.WriteMsg(connect); err != nil {
		return nil, 0, err
	}
	msg.Reset()
	if err := rd.ReadMsg(&msg); err != nil {
		return nil, 0, err
	}
	if msg.GetType() != pb.HolePunch_SYNC {
		return nil, 0, errors.New("p2p: expected DCUtR SYNC")
	}
	return remote, 0, nil
}

func (s *punchService) dial(ctx context.Context, info peer.AddrInfo) error {
	ctx, cancel := context.WithTimeout(network.WithForceDirectDial(ctx, "hole-punching"), knownDialTimeout)
	defer cancel()
	return s.host.Connect(ctx, info)
}

// idWait is closed once the node has handled identify on one connection.
type idWait struct {
	ch   chan struct{}
	done bool
}

// closedWait is the wait of a connection already closed: nothing is left to wait for.
var closedWait = func() chan struct{} { ch := make(chan struct{}); close(ch); return ch }()

// identifiedOn returns a channel closed once the node has handled identify on c: onIdentified recorded what the
// peer announced, or identify failed with c's peer. A closed connection's is closed at once.
func (n *Node) identifiedOn(c network.Conn) <-chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	if w := n.idWaitLocked(c); w != nil {
		return w.ch
	}
	return closedWait
}

// idWaitLocked returns c's wait, made on first use; nil once c is closed (the swarm marks a connection closed before
// it reports its loss, so no wait outlives identifyForget). Under mu.
func (n *Node) idWaitLocked(c network.Conn) *idWait {
	w := n.identified[c]
	if w == nil {
		if c.IsClosed() {
			return nil
		}
		w = &idWait{ch: make(chan struct{})}
		n.identified[c] = w
	}
	return w
}

// identifyDone records that the node has handled identify on c; with c nil, identify failed with p on a connection
// its event does not name, and every connection of p still waiting is released.
func (n *Node) identifyDone(p peer.ID, c network.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()
	release := func(w *idWait) {
		if w != nil && !w.done {
			w.done = true
			close(w.ch)
		}
	}
	if c != nil {
		release(n.idWaitLocked(c))
		return
	}
	for conn, w := range n.identified {
		if conn.RemotePeer() == p {
			release(w)
		}
	}
}

// identifyForget forgets the wait of c, closed.
func (n *Node) identifyForget(c network.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.identified, c)
}
