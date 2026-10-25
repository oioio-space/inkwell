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
	c  Conn
	mu sync.Mutex
}

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
	s := &session{c: c}
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

	for _, p := range r.t.Peers() {
		r.sendLocked(s, Envelope{Kind: PeerUp, Peer: p})
	}
	if r.att != nil {
		for _, e := range r.att.Notices() {
			r.sendLocked(s, e)
		}
	}
	s.mu.Unlock()
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
			if err := r.t.Send(c.Context(), e.Peer, e.Payload); err != nil {
				r.warn("bridge: send to peer", "peer", e.Peer, "err", err)
			}
		case Attest:
			r.attest(c.Context(), e.Payload)
		default:
			r.warn("bridge: dropped frame from webview", "kind", e.Kind)
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
