package bridge

import (
	"context"
	"log/slog"
	"sync"

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
	mu  sync.Mutex
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
	q      chan []byte
	cancel context.CancelFunc
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
	r := &Router{t: t, log: log}
	r.src, _ = t.(Source)
	r.att, _ = t.(Attester)
	return r
}

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
// c on return unless a newer connection already replaced it.
func (r *Router) Serve(c Conn) {
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
	s.mu.Unlock()

	// One sender per peer, so that a slow link delays only its own peer, and
	// FIFO per peer: the mux serves small frames ahead of the fragments of a
	// large one only if the router hands them over while it is being sent.
	var cancel context.CancelFunc
	s.ctx, cancel = context.WithCancel(c.Context())
	defer func() { cancel(); s.wg.Wait() }()
	for {
		b, err := c.Receive()
		if err != nil {
			return
		}
		if r.current() != s {
			return // replaced while blocked in Receive
		}
		e, err := Decode(b)
		if err != nil {
			r.warn("bridge: dropped frame", "bytes", len(b), "err", err)
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
		o = &outbox{q: make(chan []byte, outboxDepth), cancel: cancel}
		s.out[peer] = o
		s.wg.Go(func() { r.sendTo(pctx, peer, o.q) })
	}
	s.omu.Unlock()
	select {
	case o.q <- payload:
	default:
		r.warn("bridge: peer does not read, resetting it", "peer", peer)
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

// sendTo sends the payloads of q to peer, in order, until ctx is done.
func (r *Router) sendTo(ctx context.Context, peer transport.PeerID, q <-chan []byte) {
	for {
		select {
		case p := <-q:
			if ctx.Err() != nil {
				return
			}
			if err := r.t.Send(ctx, peer, p); err != nil {
				r.warn("bridge: send to peer", "peer", peer, "err", err)
			}
		case <-ctx.Done():
			return
		}
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
