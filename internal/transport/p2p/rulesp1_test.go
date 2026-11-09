package p2p_test

import (
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-yamux/v5"

	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// slowConn is a net.Conn that misbehaves like a relay hop. stall holds a
// Write (the writer blocks, honouring the write deadline yamux armed, as a
// socket does); lag lets the Write return at once and delivers its bytes that
// much later, in order and without limiting the rate. Either may be nil.
type slowConn struct {
	net.Conn
	stall, lag func() time.Duration

	mu       sync.Mutex
	deadline time.Time
	queue    chan delayed
	done     chan struct{}
	once     sync.Once
}

type delayed struct {
	at time.Time
	b  []byte
}

func newSlowConn(c net.Conn, stall, lag func() time.Duration) *slowConn {
	s := &slowConn{Conn: c, stall: stall, lag: lag, done: make(chan struct{})}
	if lag != nil {
		s.queue = make(chan delayed, 1<<16)
		go s.pump()
	}
	return s
}

// pump delivers the lagged writes.
func (s *slowConn) pump() {
	for {
		select {
		case it := <-s.queue:
			select {
			case <-time.After(time.Until(it.at)):
			case <-s.done:
				return
			}
			if _, err := s.Conn.Write(it.b); err != nil {
				return
			}
		case <-s.done:
			return
		}
	}
}

// SetWriteDeadline remembers the deadline. A lagged write is accepted at once,
// so the deadline is not passed on: it must not cut the delivery that follows.
func (s *slowConn) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	s.deadline = t
	s.mu.Unlock()
	if s.lag != nil {
		return nil
	}
	return s.Conn.SetWriteDeadline(t)
}

func (s *slowConn) Write(p []byte) (int, error) {
	if s.stall != nil {
		if d := s.stall(); d > 0 {
			s.mu.Lock()
			dl := s.deadline
			s.mu.Unlock()
			if wake := time.Now().Add(d); !dl.IsZero() && dl.Before(wake) {
				time.Sleep(time.Until(dl))
				return 0, os.ErrDeadlineExceeded
			}
			time.Sleep(d)
		}
	}
	if s.lag == nil {
		return s.Conn.Write(p)
	}
	select {
	case s.queue <- delayed{at: time.Now().Add(s.lag()), b: append([]byte(nil), p...)}:
		return len(p), nil
	case <-s.done:
		return 0, io.ErrClosedPipe
	}
}

func (s *slowConn) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.Conn.Close()
}

func fixed(d time.Duration) func() time.Duration { return func() time.Duration { return d } }

// sessions builds two yamux sessions from the node's own configuration over
// the given connection ends, and closes them with the test.
func sessions(t *testing.T, ca, cb net.Conn) (alice, bob *yamux.Session) {
	t.Helper()
	alice, err := yamux.Client(ca, p2p.MuxConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	bob, err = yamux.Server(cb, p2p.MuxConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alice.Close(); _ = bob.Close(); _ = ca.Close(); _ = cb.Close() })
	return alice, bob
}

// TestRuleSP1MuxConfig proves rules SP1-4 and SP1-5: the muxer the node
// passes to libp2p states the write timeout, the keepalive and the pinned
// stream window.
func TestRuleSP1MuxConfig(t *testing.T) {
	cfg := p2p.MuxConfig()
	// The spec's numbers, written out: this test pins the constants themselves.
	if cfg.ConnectionWriteTimeout != 120*time.Second {
		t.Errorf("ConnectionWriteTimeout = %v, want 120s", cfg.ConnectionWriteTimeout)
	}
	if !cfg.EnableKeepAlive || cfg.KeepAliveInterval != 30*time.Second {
		t.Errorf("keepalive = %v every %v, want on every 30s", cfg.EnableKeepAlive, cfg.KeepAliveInterval)
	}
	if cfg.InitialStreamWindowSize != 256<<10 || cfg.MaxStreamWindowSize != 256<<10 {
		t.Errorf("stream window = %d initial, %d max, want both 256 KiB", cfg.InitialStreamWindowSize, cfg.MaxStreamWindowSize)
	}
}

// TestRuleSP1SessionSurvivesSlowHop proves NoSessionLoss (rule SP1-4, models
// SP1Relay, SP1Hop): a frame write blocked for 55 s, a whole relay hop and
// under the valve's earliest firing of 60 s, closes neither the session nor
// a stream, and the frame is delivered.
func TestRuleSP1SessionSurvivesSlowHop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ca, cb := net.Pipe()
		var armed atomic.Bool
		// One frame write blocks for a whole relay hop: under the valve's
		// earliest firing, half of MuxWriteTimeout.
		hop := p2p.MuxWriteTimeout/2 - 5*time.Second
		alice, bob := sessions(t, newSlowConn(ca, func() time.Duration {
			if armed.CompareAndSwap(true, false) {
				return hop
			}
			return 0
		}, nil), cb)
		sa, err := alice.OpenStream(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sa.Write([]byte("hello")); err != nil {
			t.Fatal(err)
		}
		sb, err := bob.AcceptStream()
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 5)
		if _, err := io.ReadFull(sb, buf); err != nil {
			t.Fatal(err)
		}
		armed.Store(true)
		if _, err := sa.Write([]byte("slow!")); err != nil {
			t.Fatalf("Write across a %v hop = %v, want it delivered", hop, err)
		}
		if _, err := io.ReadFull(sb, buf); err != nil || string(buf) != "slow!" {
			t.Fatalf("frame after the hop = %q, %v, want %q", buf, err, "slow!")
		}
		time.Sleep(10 * time.Second)
		if alice.IsClosed() || bob.IsClosed() {
			t.Errorf("session closed after a %v hop (alice %v, bob %v)", hop, alice.IsClosed(), bob.IsClosed())
		}
		if _, err := sa.Write([]byte("again")); err != nil {
			t.Errorf("stream after the hop: %v, want open", err)
		}
	})
}

// TestRuleSP1KeepaliveAnsweredLate proves NoSessionLoss (rule SP1-4, models
// SP1RelayKeepalive, SP1FloorKeepalive): Bob's direction delivers a frame
// 100 s late after 30 s of silence, a ping behind a window of data in the relay;
// the keepalive ping is answered and the session stays open.
func TestRuleSP1KeepaliveAnsweredLate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ca, cb := net.Pipe()
		// A ping behind a window of data in the relay: the answer arrives
		// late, but inside the answer timer (MuxWriteTimeout).
		late := p2p.MuxWriteTimeout - 20*time.Second
		last := time.Now()
		alice, bob := sessions(t, ca, newSlowConn(cb, nil, func() time.Duration {
			defer func() { last = time.Now() }()
			if time.Since(last) >= p2p.MuxKeepAlive {
				return late
			}
			return 0
		}))
		time.Sleep(3 * (p2p.MuxKeepAlive + late)) // several keepalive rounds
		if alice.IsClosed() || bob.IsClosed() {
			t.Errorf("session closed with a keepalive answered %v late (alice %v, bob %v)", late, alice.IsClosed(), bob.IsClosed())
		}
	})
}

// TestRuleSP1WindowPinned proves SP1-5 (model SP1FloorGrown): over a conn
// with 100 ms RTT, a reader that consumes at once never leaves more than
// 256 KiB accepted by the writer and unread: the window is not auto-tuned.
func TestRuleSP1WindowPinned(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ca, cb := net.Pipe()
		alice, bob := sessions(t, newSlowConn(ca, nil, fixed(50*time.Millisecond)), newSlowConn(cb, nil, fixed(50*time.Millisecond)))
		const total, chunk = 2 << 20, 32 << 10
		var accepted, read, peak atomic.Int64
		observe := func() {
			d := accepted.Load() - read.Load()
			for old := peak.Load(); d > old && !peak.CompareAndSwap(old, d); old = peak.Load() {
			}
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			sb, err := bob.AcceptStream()
			if err != nil {
				return
			}
			buf := make([]byte, 64<<10)
			for read.Load() < total {
				n, err := sb.Read(buf)
				read.Add(int64(n))
				observe()
				if err != nil {
					return
				}
			}
		}()
		sa, err := alice.OpenStream(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for sent := 0; sent < total; sent += chunk {
			if _, err := sa.Write(make([]byte, chunk)); err != nil {
				t.Fatal(err)
			}
			accepted.Add(chunk)
			observe()
		}
		<-done
		if got := peak.Load(); got > p2p.StreamWindow {
			t.Errorf("%d bytes accepted and unread at once, want <= 256 KiB (no auto-tuning)", got)
		}
	})
}
