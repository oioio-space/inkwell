package blob_test

import (
	"bytes"
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"sync"
	"testing"

	"github.com/oioio-space/inkwell/internal/blob"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
)

// testData returns n pseudo-random bytes derived from seed.
func testData(n int, seed uint64) []byte {
	r := rand.New(rand.NewPCG(seed, seed)) // #nosec G404 -- test data, nothing secret.
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Uint32())
	}
	return b
}

// openStore opens a store in dir, or in a new temporary directory if dir is "".
func openStore(t testing.TB, dir string, o blob.Options) (*blob.Store, string) {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	s, err := blob.Open(root, o)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, dir
}

// logRecorder is a slog.Handler that keeps the records.
type logRecorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *logRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (l *logRecorder) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	l.records = append(l.records, r.Clone())
	l.mu.Unlock()
	return nil
}
func (l *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logRecorder) WithGroup(string) slog.Handler      { return l }

// security counts the records marked event=security.
func (l *logRecorder) security() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, r := range l.records {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "event" && a.Value.String() == "security" {
				n++
				return false
			}
			return true
		})
	}
	return n
}

// peer is one participant of a test network: a store and a service on a
// bulk lane, with the set of hashes it considers referenced.
type peer struct {
	id    transport.PeerID
	lane  *memnet.Node
	store *blob.Store
	dir   string
	svc   *blob.Service
	log   *logRecorder

	mu sync.Mutex
	// gate, if set, is called when a request is being served, to hold it.
	gate    func(blob.Hash)
	refs    map[blob.Hash]bool
	denied  map[transport.PeerID]bool
	dropped []transport.PeerID
}

func (p *peer) reference(hs ...blob.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, h := range hs {
		p.refs[h] = true
	}
}

// netOf is a memnet network of peers that trust each other.
type netOf struct {
	t   *testing.T
	net *memnet.Network
}

func newNet(t *testing.T) *netOf { return &netOf{t: t, net: memnet.New(1, memnet.Faults{})} }

// join adds a peer with its own store, running a service until the test ends.
func (n *netOf) join(id transport.PeerID, so blob.Options, o blob.ServiceOptions) *peer {
	n.t.Helper()
	p := &peer{id: id, refs: map[blob.Hash]bool{}, denied: map[transport.PeerID]bool{}, log: new(logRecorder)}
	p.lane = n.net.JoinBulk(id)
	p.store, p.dir = openStore(n.t, "", so)
	p.start(n.t, p.store, p.lane, o)
	return p
}

// start runs a service over lane with store, until the test ends.
func (p *peer) start(t *testing.T, store *blob.Store, lane transport.Transport, o blob.ServiceOptions) {
	t.Helper()
	o.Referenced = func(h blob.Hash) bool {
		p.mu.Lock()
		gate := p.gate
		ref := p.refs[h]
		p.mu.Unlock()
		if gate != nil {
			gate(h)
		}
		return ref
	}
	o.Allowed = func(id transport.PeerID) bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return !p.denied[id]
	}
	o.Drop = func(id transport.PeerID) {
		p.mu.Lock()
		p.dropped = append(p.dropped, id)
		p.mu.Unlock()
	}
	o.Log = slog.New(p.log)
	svc, err := blob.NewService(lane, store, o)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	p.svc = svc
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() { _ = svc.Run(ctx) })
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
}

// deny makes the peer refuse to serve or ask id.
func (p *peer) deny(id transport.PeerID) {
	p.mu.Lock()
	p.denied[id] = true
	p.mu.Unlock()
}

func (p *peer) droppedPeers() []transport.PeerID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]transport.PeerID(nil), p.dropped...)
}

// put stores data in the peer's store and marks it referenced there.
func (p *peer) put(t testing.TB, data []byte) blob.Hash {
	t.Helper()
	h, err := p.store.PutBytes(data)
	if err != nil {
		t.Fatalf("%s: PutBytes: %v", p.id, err)
	}
	p.reference(h)
	return h
}

// mustGet fails the test unless the peer's store holds data under h.
func mustGet(t testing.TB, s *blob.Store, h blob.Hash, data []byte) {
	t.Helper()
	got, err := s.Get(h)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("Get(%v) = %d bytes, %v; want the %d bytes put", h, len(got), err, len(data))
	}
}
