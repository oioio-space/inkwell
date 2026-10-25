// Package mux splits one [transport.Transport] into independent
// sub-transports, one per channel.
//
// Every payload crosses the underlying transport prefixed by the one byte of
// its channel. A single internal reader routes what arrives to a bounded queue
// per channel, so the channels share one ordered stream: frames sent to one peer
// are queued in the order they were sent, whatever their channel. A consumer
// that must act on a message of one channel only after those of another that
// were sent before it can use [Wake] to do so.
//
//	subs := mux.Split(t, 0x01, 0x02)
//	repo, trust := subs[0x01], subs[0x02]
//
// Peer events (PeerUp, PeerDown) are delivered on every channel, in order with
// its messages. A frame for a channel nobody asked for, or without a channel
// byte, is dropped. Every channel must be drained: a full queue stops the
// reader, and with it every other channel, so that no frame is lost.
package mux

import (
	"context"
	"sync"

	"github.com/oioio-space/inkwell/internal/transport"
)

// Overhead is how many bytes the channel prefix adds to a payload: the
// sub-transports accept payloads up to transport.MaxFrame - Overhead.
const Overhead = 1

const (
	// maxQueued bounds the events queued per channel.
	maxQueued = 256
	// maxQueuedBytes bounds the payload bytes queued per channel. A single
	// larger frame is still accepted when the queue is empty.
	maxQueuedBytes = 32 << 20
)

// muxer is the state shared by the sub-transports of one Split.
type muxer struct {
	t      transport.Transport
	subs   map[byte]*sub
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // closed when the reader has stopped
	wake   chan struct{}
	err    error
	order  []*sub // subs in the order they were asked for
	once   sync.Once
}

// Split returns a sub-transport for each of channels, over t. Sub-transports
// share t: closing any of them closes t and so all of them. Split starts one
// goroutine that reads t, which stops when t or a sub-transport is closed.
// The caller must not use t's Recv afterwards; Send on t stays possible but
// bypasses the channels. Duplicate channels are returned once.
func Split(t transport.Transport, channels ...byte) map[byte]transport.Transport {
	ctx, cancel := context.WithCancel(context.Background())
	m := &muxer{
		t:      t,
		subs:   make(map[byte]*sub, len(channels)),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		wake:   make(chan struct{}, 1),
	}
	out := make(map[byte]transport.Transport, len(channels))
	for _, c := range channels {
		if _, dup := m.subs[c]; dup {
			continue
		}
		s := &sub{m: m, ch: c, q: newQueue(m.notify)}
		m.subs[c] = s
		m.order = append(m.order, s)
		out[c] = s
	}
	go m.run()
	return out
}

// Wake returns a channel that receives whenever any sub-transport returned by
// the same [Split] as t may have a new event, and when they are closed.
// Wakeups coalesce and may be spurious, so a consumer polls its channels (with
// an already canceled context, which still returns a queued event) until they
// are empty, then waits on Wake. It returns nil, which blocks forever, if t is
// not a sub-transport of this package.
func Wake(t transport.Transport) <-chan struct{} {
	if s, ok := t.(*sub); ok {
		return s.m.wake
	}
	return nil
}

func (m *muxer) notify() { signal(m.wake) }

// run routes what t receives until t or the muxer is closed.
func (m *muxer) run() {
	defer close(m.done)
	defer func() {
		for _, s := range m.order {
			s.q.close()
		}
	}()
	for {
		ev, err := m.t.Recv(m.ctx)
		if err != nil {
			return
		}
		if ev.Kind != transport.Message {
			for _, s := range m.order {
				if !s.q.push(m.ctx, ev) {
					return
				}
			}
			continue
		}
		if len(ev.Payload) == 0 {
			continue
		}
		s := m.subs[ev.Payload[0]]
		if s == nil {
			continue
		}
		ev.Payload = ev.Payload[Overhead:]
		if !s.q.push(m.ctx, ev) {
			return
		}
	}
}

func (m *muxer) close() error {
	m.once.Do(func() {
		m.cancel()
		m.err = m.t.Close()
		<-m.done
	})
	return m.err
}

// sub is one channel of a muxer.
type sub struct {
	m  *muxer
	q  *queue
	ch byte
}

var _ transport.Transport = (*sub)(nil)

// Local returns the identity of the underlying transport.
func (s *sub) Local() transport.PeerID { return s.m.t.Local() }

// Peers returns the peers of the underlying transport.
func (s *sub) Peers() []transport.PeerID { return s.m.t.Peers() }

// Send sends payload to peer to on this channel.
func (s *sub) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	if len(payload) > transport.MaxFrame-Overhead {
		return transport.ErrFrameTooLarge
	}
	framed := make([]byte, Overhead+len(payload))
	framed[0] = s.ch
	copy(framed[Overhead:], payload)
	return s.m.t.Send(ctx, to, framed)
}

// Recv returns the next event of this channel.
func (s *sub) Recv(ctx context.Context) (transport.Event, error) { return s.q.pop(ctx) }

// Close closes the underlying transport, and so every channel.
func (s *sub) Close() error { return s.m.close() }

// queue is a bounded FIFO of events with one producer and one consumer.
type queue struct {
	notify   func()
	notEmpty chan struct{}
	notFull  chan struct{}
	events   []transport.Event
	bytes    int
	mu       sync.Mutex
	closed   bool
}

func newQueue(notify func()) *queue {
	return &queue{notify: notify, notEmpty: make(chan struct{}, 1), notFull: make(chan struct{}, 1)}
}

func signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

// push appends ev, waiting for room. It reports false if the queue was closed
// or ctx done first.
func (q *queue) push(ctx context.Context, ev transport.Event) bool {
	for {
		q.mu.Lock()
		switch {
		case q.closed:
			q.mu.Unlock()
			return false
		case len(q.events) == 0 || (len(q.events) < maxQueued && q.bytes+len(ev.Payload) <= maxQueuedBytes):
			q.events = append(q.events, ev)
			q.bytes += len(ev.Payload)
			q.mu.Unlock()
			signal(q.notEmpty)
			q.notify()
			return true
		}
		q.mu.Unlock()
		select {
		case <-q.notFull:
		case <-ctx.Done():
			return false
		}
	}
}

// pop returns the oldest event, waiting for one. A queued event is returned
// even if ctx is done; a closed queue returns [transport.ErrClosed].
func (q *queue) pop(ctx context.Context) (transport.Event, error) {
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return transport.Event{}, transport.ErrClosed
		}
		if len(q.events) > 0 {
			ev := q.events[0]
			q.events[0] = transport.Event{}
			q.events = q.events[1:]
			q.bytes -= len(ev.Payload)
			q.mu.Unlock()
			signal(q.notFull)
			return ev, nil
		}
		q.mu.Unlock()
		select {
		case <-q.notEmpty:
		case <-ctx.Done():
			return transport.Event{}, ctx.Err()
		}
	}
}

// close discards the queued events and makes pop fail.
func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.events = nil
	q.bytes = 0
	q.mu.Unlock()
	signal(q.notEmpty)
	signal(q.notFull)
	q.notify()
}
