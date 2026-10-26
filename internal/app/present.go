package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/oioio-space/inkwell/internal/bridge"
)

// PresentStreamName is the name of the Wails stream that joins the windows of
// one presentation on this machine: the presenter view, which owns the project
// streams, and the public view, which opens no other stream (those serve one
// window at a time).
const PresentStreamName = "present"

const (
	// maxPresentFrame bounds one frame between the windows; a slide larger than
	// this is not forwarded (the public view shows a blank slide).
	maxPresentFrame = 8 << 20
	// maxPresentConns bounds the windows joined to the stream: presenter, public
	// view, and a margin for a window that reloads before the old one is gone.
	maxPresentConns = 6
	// presentQueue is how many frames wait for a slow window before the next is dropped.
	presentQueue = 256
	// maxPresentType bounds the length of a frame type.
	maxPresentType = 16
)

var errBadPresentFrame = errors.New("bad present frame")

// checkPresentFrame verifies that b is a JSON object whose "type" is a short
// lowercase word, and that it is not larger than a frame may be. The hub moves
// the rest of the frame as opaque bytes: only the windows know what it means.
func checkPresentFrame(b []byte) error {
	if len(b) == 0 || len(b) > maxPresentFrame {
		return fmt.Errorf("%w: %d bytes", errBadPresentFrame, len(b))
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return fmt.Errorf("%w: %w", errBadPresentFrame, err)
	}
	if n := len(head.Type); n == 0 || n > maxPresentType {
		return fmt.Errorf("%w: type of %d bytes", errBadPresentFrame, n)
	}
	for _, c := range []byte(head.Type) {
		if c < 'a' || c > 'z' {
			return fmt.Errorf("%w: type %q", errBadPresentFrame, head.Type)
		}
	}
	return nil
}

// Present joins the windows of a presentation on this machine. Whatever one
// window sends goes to all the others, unchanged, so that "next slide" pressed in
// either view moves both, and so that the public view gets its pages from the
// presenter without a second compilation. It also tells every window how many
// windows are joined ({"type":"roster","windows":n}), which is how the presenter
// learns that the public view was closed. It is stateless: a window that joins
// late says "hello" and the others answer.
type Present struct {
	log   *slog.Logger
	mu    sync.Mutex
	conns map[*presentConn]struct{}
}

// presentConn is one joined window. Frames for it queue in out and are written
// by one goroutine, so that a slow window never delays the others.
type presentConn struct {
	c    bridge.Conn
	out  chan []byte
	gone chan struct{}
}

// NewPresent returns a hub that logs to log (nil discards).
func NewPresent(log *slog.Logger) *Present {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Present{log: log, conns: map[*presentConn]struct{}{}}
}

// Serve joins c to the other windows and relays its frames until c fails. A
// malformed frame is logged and dropped; a window beyond the limit is refused.
func (h *Present) Serve(c bridge.Conn) {
	pc := &presentConn{c: c, out: make(chan []byte, presentQueue), gone: make(chan struct{})}
	h.mu.Lock()
	if len(h.conns) >= maxPresentConns {
		h.mu.Unlock()
		h.log.Warn("present stream: too many windows, refused")
		return
	}
	h.conns[pc] = struct{}{}
	h.mu.Unlock()

	var writer sync.WaitGroup
	writer.Go(func() { h.write(pc) })
	h.roster()
	defer func() {
		h.mu.Lock()
		delete(h.conns, pc)
		h.mu.Unlock()
		close(pc.gone)
		writer.Wait()
		h.roster()
	}()

	for {
		b, err := c.Receive()
		if err != nil {
			return
		}
		if err := checkPresentFrame(b); err != nil {
			h.log.Warn("present stream: dropped frame", "bytes", len(b), "err", err)
			continue
		}
		h.relay(pc, b)
	}
}

// write sends the queued frames of pc until pc is gone or a send fails.
func (h *Present) write(pc *presentConn) {
	for {
		select {
		case b := <-pc.out:
			if err := pc.c.Send(b); err != nil {
				h.log.Warn("present stream: send to window", "err", err)
				return
			}
		case <-pc.gone:
			return
		}
	}
}

// relay queues b for every window but from; a window whose queue is full loses it.
func (h *Present) relay(from *presentConn, b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for pc := range h.conns {
		if pc == from {
			continue
		}
		select {
		case pc.out <- bytes.Clone(b):
		default:
			h.log.Warn("present stream: window too slow, frame dropped")
		}
	}
}

// roster tells every window how many are joined.
func (h *Present) roster() {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, err := json.Marshal(struct {
		Type    string `json:"type"`
		Windows int    `json:"windows"`
	}{"roster", len(h.conns)})
	if err != nil {
		return // a fixed struct always encodes
	}
	for pc := range h.conns {
		select {
		case pc.out <- b:
		default:
		}
	}
}
