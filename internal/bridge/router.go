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
	log *slog.Logger
	lim limiter
	cur *session // attached connection, nil if none
	mu  sync.Mutex
}

// session is one attached connection; mu serializes its writes, which come
// from both [Router.Run] and [Router.Serve].
type session struct {
	c  Conn
	mu sync.Mutex
}

// NewRouter returns a router over t that logs to log.
func NewRouter(t transport.Transport, log *slog.Logger) *Router {
	return &Router{t: t, log: log}
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
		ev, err := r.t.Recv(ctx)
		if err != nil {
			return err
		}
		if s := r.current(); s != nil {
			r.send(s, Envelope{Kind: ev.Kind, Peer: ev.Peer, Payload: ev.Payload})
		}
	}
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
// it a PeerUp for each currently connected peer, then relays its frames to
// the transport until c fails or is replaced. Malformed frames and sends to
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
		r.sendLocked(s, Envelope{Kind: transport.PeerUp, Peer: p})
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
		if e.Kind != transport.Message {
			r.warn("bridge: dropped frame from webview", "kind", e.Kind)
			continue
		}
		if err := r.t.Send(c.Context(), e.Peer, e.Payload); err != nil {
			r.warn("bridge: send to peer", "peer", e.Peer, "err", err)
		}
	}
}
