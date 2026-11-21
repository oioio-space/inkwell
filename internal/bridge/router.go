package bridge

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// Conn is one open stream connection to the webview. It abstracts the Wails
// stream connection so the router is testable without a window.
type Conn interface {
	// Send writes one frame. The implementation takes ownership of b.
	Send(b []byte) error
	// Receive blocks for the next frame and returns an error once the
	// connection is closed.
	Receive() ([]byte, error)
	// Context is done when the connection closes.
	Context() context.Context
}

// Router connects the single attached [Conn] to a [transport.Transport].
// Create one with [NewRouter].
type Router struct {
	t   transport.Transport
	src Source   // t, if it reports envelopes beyond transport events
	att Attester // t, if it controls who may write
	log *slog.Logger
	cur *session // attached connection, nil if none
	lim limiter
	// counts are the per-peer failure counters, which outlive a session.
	counts map[transport.PeerID]*peerCounts
	// epoch is the origin of the stamps.
	epoch time.Time
	// linksPeriod is the period of the Links notifications, 0 for none.
	linksPeriod time.Duration
	drain       *Drain
	presenceOK  func(Conn, uint32)
	mu          sync.Mutex
}

// session is one attached connection; mu serializes its writes, which come
// from both [Router.Run] and [Router.Serve].
type session struct {
	c   Conn
	ctx context.Context // done when the session ends
	wg  sync.WaitGroup  // the senders of out
	out map[transport.PeerID]*outbox
	// omu guards out. It is separate from mu, which a stuck write to the
	// webview may hold.
	omu sync.Mutex
	mu  sync.Mutex
}

// outbox is the queue of the messages for one peer and the stop of the
// goroutine that sends them.
type outbox struct {
	q      chan queuedMsg
	done   chan struct{}
	cancel context.CancelFunc
	// msgs and bytes are what waits to be sent, the message being sent
	// included; head is when that one was enqueued (see [Router.stamp]), or 0.
	msgs, bytes, head atomic.Int64
}

// queuedMsg is a payload with the stamp of the time it was handed to the router.
type queuedMsg struct {
	payload []byte
	at      int64 // [Router.stamp]
	barrier chan<- bool
}

// Resetter is implemented by a transport that can reset a peer: close its
// connections so that the peer goes down and its sync starts afresh, but
// let it reconnect after a short backoff ([p2p.Node.Reset]), unlike a drop,
// which is for a suspended or removed member.
type Resetter interface{ Reset(who transport.PeerID) }

// NewRouter returns a router over t that logs to log. If t implements
// [Source] the router relays its envelopes, and if it implements [Attester]
// it signs the Attest envelopes of the webview; otherwise those are refused.
func NewRouter(t transport.Transport, log *slog.Logger) *Router {
	r := &Router{t: t, log: log, counts: make(map[transport.PeerID]*peerCounts), linksPeriod: LinksPushPeriod, epoch: time.Now()}
	r.src, _ = t.(Source)
	r.att, _ = t.(Attester)
	return r
}

// UseDrain makes d end this router's streams before a project change (see [Drain]). It is called
// before the first [Router.Serve].
func (r *Router) UseDrain(d *Drain) { r.drain = d }

// OnPresenceHandoff reports a successful local presence barrier before the
// stream replies. Install it before serving connections.
func (r *Router) OnPresenceHandoff(fn func(Conn, uint32)) { r.presenceOK = fn }

// stamp returns the nanoseconds since the router's epoch plus one, so that it
// is never zero: a monotonic reading that is cheaper to queue than a time.Time.
func (r *Router) stamp() int64 { return int64(time.Since(r.epoch)) + 1 }

func (r *Router) current() *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cur
}

// Run forwards transport events to the attached connection, dropping them
// while none is attached. It must be the only reader of the transport's Recv
// and returns the error that ended it: ctx's, or the transport's.
func (r *Router) Run(ctx context.Context) error {
	for {
		e, err := r.recv(ctx)
		if err != nil {
			return err
		}
		if s := r.current(); s != nil {
			if e.Kind == PeerDown {
				r.stopPeer(s, e.Peer)
			}
			r.send(s, e)
		}
	}
}

// recv returns the next envelope of the transport.
func (r *Router) recv(ctx context.Context) (Envelope, error) {
	if r.src != nil {
		return r.src.RecvEnvelope(ctx)
	}
	ev, err := r.t.Recv(ctx)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Kind: Kind(ev.Kind), Peer: ev.Peer, Payload: ev.Payload}, nil
}

// warn logs a rate-limited warning: at most one per second per msg, with the
// count of those suppressed in between. args must never carry payload bytes.
func (r *Router) warn(msg string, args ...any) {
	if n, ok := r.lim.allow(msg); ok {
		if n > 0 {
			args = append(args, "suppressed", n)
		}
		r.log.Warn(msg, args...)
	}
}

// send encodes e and writes it to s, logging failures.
func (r *Router) send(s *session, e Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.sendLocked(s, e)
}

// sendLocked is send for a caller that holds s.mu.
func (r *Router) sendLocked(s *session, e Envelope) {
	b, err := Encode(e)
	if err != nil {
		r.log.Error("bridge: encode", "kind", e.Kind, "err", err)
		return
	}
	if err := s.c.Send(b); err != nil {
		r.warn("bridge: send to webview", "kind", e.Kind, "err", err)
	}
}

// Serve attaches c, replacing any previous connection (a page reload), sends
// it a PeerUp for each currently connected peer and the current attestations
// (see [Attester.Notices]), then relays its frames to the transport until c
// fails or is replaced. Malformed frames and sends to
// unknown peers are logged, never their payloads, and skipped. Serve detaches
// c on return unless a newer connection already replaced it. Once its [Drain]
// drained, a frame is dropped and a new stream is closed at once.
func (r *Router) Serve(c Conn) {
	detach, ok := r.drain.attach(c)
	if !ok {
		return // the session is leaving: the stream is closed (rule N-16)
	}
	defer detach()
	var links []LinkEntry // the current links, for the first notification
	if r.linksPeriod > 0 {
		links = linkEntries(r.Links(), time.Now())
	}
	s := &session{c: c, out: make(map[transport.PeerID]*outbox)}
	// Hold the send lock from before s is published until the announce is
	// done, so events Run forwards cannot overtake the initial PeerUps.
	s.mu.Lock()
	r.mu.Lock()
	r.cur = s
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		if r.cur == s {
			r.cur = nil
		}
		r.mu.Unlock()
	}()

	if r.att != nil {
		for _, e := range r.att.Notices() {
			r.sendLocked(s, e)
		}
	}
	for _, p := range r.t.Peers() {
		r.sendLocked(s, Envelope{Kind: PeerUp, Peer: p})
	}
	if r.linksPeriod > 0 { // the current links go out at once; later ones when they change
		if e, ok := r.linksEnvelope(links); ok {
			r.sendLocked(s, e)
		}
	}
	s.mu.Unlock()

	// One sender per peer, so that a slow link delays only its own peer, and
	// FIFO per peer: the mux serves small frames ahead of the fragments of a
	// large one only if the router hands them over while it is being sent.
	var cancel context.CancelFunc
	s.ctx, cancel = context.WithCancel(c.Context())
	defer func() { cancel(); s.wg.Wait() }()
	if r.linksPeriod > 0 {
		last := stableLinks(links)
		s.wg.Go(func() { r.pushLinks(s, last) })
	}
	for {
		b, err := c.Receive()
		if err != nil {
			return
		}
		if r.current() != s {
			return // replaced while blocked in Receive
		}
		// Version zero is reserved here for the five-byte local presence
		// barrier (zero followed by a u32 token). It never goes to a peer.
		if len(b) == 5 && b[0] == 0 {
			if r.drain.Begin() {
				r.presenceBarrier(s, b)
				r.drain.End()
			}
			continue
		}
		e, err := Decode(b)
		if err != nil {
			r.warn("bridge: dropped frame", "bytes", len(b), "err", err)
			continue
		}
		if !r.drain.Begin() { // read once the session is leaving: dropped; a peer's sync sends it again (rule N-16)
			r.warn("bridge: frame dropped, leaving", "kind", e.Kind)
			continue
		}
		switch e.Kind {
		case Message:
			r.enqueue(s, e.Peer, e.Payload)
		case Attest:
			r.attest(c.Context(), e.Payload)
		default:
			r.warn("bridge: dropped frame from webview", "kind", e.Kind)
		}
		r.drain.End()
	}
}

// outboxDepth is how many messages may wait for a peer. A peer that lets more
// pile up is not reading: waiting for it would stall every other peer, so the
// router resets it instead.
const outboxDepth = 64

// enqueue hands payload to the sender of peer, starting it if needed. It never
// waits: if the queue is full, the message is dropped and the peer is reset
// (see [Resetter]), so that the webview sees it go down and syncs again.
func (r *Router) enqueue(s *session, peer transport.PeerID, payload []byte) {
	s.omu.Lock()
	o := s.out[peer]
	if o == nil {
		pctx, cancel := context.WithCancel(s.ctx)
		o = &outbox{q: make(chan queuedMsg, outboxDepth), done: make(chan struct{}), cancel: cancel}
		s.out[peer] = o
		s.wg.Go(func() { r.sendTo(pctx, peer, o) })
	}
	s.omu.Unlock()
	// Count before queueing, so that the sender never sees a message it was
	// not told about.
	o.msgs.Add(1)
	o.bytes.Add(int64(len(payload)))
	select {
	case o.q <- queuedMsg{payload: payload, at: r.stamp()}:
	default:
		o.msgs.Add(-1)
		o.bytes.Add(-int64(len(payload)))
		r.warn("bridge: peer does not read, resetting it", "peer", peer)
		r.countsOf(peer).resets.Add(1)
		r.stopPeer(s, peer)
		if d, ok := r.t.(Resetter); ok {
			d.Reset(peer)
		}
	}
}

// stopPeer ends the sender of peer, and what waits in its queue.
func (r *Router) stopPeer(s *session, peer transport.PeerID) {
	s.omu.Lock()
	o := s.out[peer]
	delete(s.out, peer)
	s.omu.Unlock()
	if o != nil {
		o.cancel()
	}
}

// sendTo sends the payloads of o to peer, in order, until ctx is done.
func (r *Router) sendTo(ctx context.Context, peer transport.PeerID, o *outbox) {
	defer close(o.done)
	failed := false
	for {
		select {
		case m := <-o.q:
			if m.barrier != nil {
				m.barrier <- !failed
				failed = false
				continue
			}
			if ctx.Err() != nil {
				o.msgs.Add(-1)
				o.bytes.Add(-int64(len(m.payload)))
				return
			}
			o.head.Store(m.at)
			err := r.t.Send(ctx, peer, m.payload)
			o.head.Store(0)
			o.msgs.Add(-1)
			o.bytes.Add(-int64(len(m.payload)))
			if err != nil {
				failed = failed || !errors.Is(err, transport.ErrUnknownPeer)
				r.countsOf(peer).sendFailures.Add(1)
				r.warn("bridge: send to peer", "peer", peer, "err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// presenceBarrier confirms that each healthy peer's writer handed off all
// frames preceding the barrier. A disconnected peer uses ordinary link loss.
func (r *Router) presenceBarrier(s *session, token []byte) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	s.omu.Lock()
	boxes := make([]*outbox, 0, len(s.out))
	for _, o := range s.out {
		boxes = append(boxes, o)
	}
	s.omu.Unlock()
	ok := true
	for _, o := range boxes {
		ok = handoff(ctx, o) && ok
	}
	if s.ctx.Err() != nil || r.current() != s {
		return
	}
	ack := slices.Clone(token)
	if !ok {
		ack = append(ack, 1)
	} else if r.presenceOK != nil {
		r.presenceOK(s.c, binary.BigEndian.Uint32(token[1:]))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.c.Send(ack); err != nil && s.ctx.Err() == nil {
		r.warn("bridge: send presence barrier reply", "err", err)
	}
}

func handoff(ctx context.Context, o *outbox) bool {
	reply := make(chan bool, 1)
	select {
	case o.q <- queuedMsg{barrier: reply}:
	case <-o.done:
		return true
	case <-ctx.Done():
		return false
	}
	select {
	case ok := <-reply:
		return ok
	case <-o.done:
		return true
	case <-ctx.Done():
		return false
	}
}

// attest has the transport sign the heads in payload, a valid [Heads]
// encoding. It runs before the next frame is read, so the attestation is
// gossiped ahead of the repo frame that follows it.
func (r *Router) attest(ctx context.Context, payload []byte) {
	h, err := DecodeHeads(payload)
	if err != nil {
		r.warn("bridge: dropped attest", "err", err)
		return
	}
	if r.att == nil {
		r.warn("bridge: attest without enforcement")
		return
	}
	if err := r.att.Attest(ctx, h.Doc, h.Heads); err != nil {
		r.warn("bridge: attest refused", "err", err)
	}
}
