//go:build server

package app

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// The network simulation parses these lines (frontend/e2e/netsim.spec.ts): the format is a contract.
func TestLinkStepLogLine(t *testing.T) {
	var buf lockedBuffer
	sink, _ := linkStepSink(t.Context(), &buf)
	sink(p2p.LinkStep{Peer: "peerA", At: time.UnixMilli(1_700_000_000_123), Kind: transport.LinkDirect, Level: transport.QualityFair, LossPermille: 37})
	sink(p2p.LinkStep{Peer: "peerA", At: time.UnixMilli(1_700_000_001_123), Kind: transport.LinkRelayed, Level: transport.QualityUnknown, LossPermille: p2p.LossUnknown})
	want := "msg=linkstep peer=peerA t=1700000000123 kind=LinkDirect quality=fair lossPermille=37\n" +
		"msg=linkstep peer=peerA t=1700000001123 kind=LinkRelayed quality=unknown lossPermille=65535\n"
	for end := time.Now().Add(5 * time.Second); buf.String() != want && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
	}
	if got := buf.String(); got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}

// The sampler's goroutine never waits for the log: with the writer stuck, the callback returns at once,
// drops what does not fit, counts it, and says so in the log when the writer is free again.
func TestLinkStepSinkNeverBlocks(t *testing.T) {
	w := &stuckWriter{release: make(chan struct{})}
	sink, dropped := linkStepSink(t.Context(), w)
	start := time.Now()
	for range 5 * linkStepQueue {
		sink(p2p.LinkStep{Peer: "p", At: time.UnixMilli(1), Kind: transport.LinkDirect, LossPermille: p2p.LossUnknown})
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("the callback took %v with the log stuck", d)
	}
	if dropped.Load() == 0 {
		t.Fatal("nothing was dropped with the log stuck")
	}
	close(w.release)
	sink(p2p.LinkStep{Peer: "p", At: time.UnixMilli(2), Kind: transport.LinkDirect, LossPermille: p2p.LossUnknown})
	for end := time.Now().Add(5 * time.Second); !strings.Contains(w.String(), "linkstepdropped count=") && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
	}
	if !strings.Contains(w.String(), "msg=linkstepdropped count=") {
		t.Errorf("the drops were not reported in the log")
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// stuckWriter blocks every write until release is closed.
type stuckWriter struct {
	lockedBuffer
	release chan struct{}
}

func (s *stuckWriter) Write(p []byte) (int, error) {
	<-s.release
	return s.lockedBuffer.Write(p)
}
