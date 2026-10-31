package blob

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"lukechampine.com/blake3/bao"

	"github.com/oioio-space/inkwell/internal/transport"
)

// Limits of the pull protocol.
const (
	// MaxTransfers is how many blobs download at once.
	MaxTransfers = 4
	// MaxFaults is how many forged slices or malformed messages a peer may
	// send before it is dropped.
	MaxFaults = 3

	defaultTimeout    = 30 * time.Second
	defaultMaxServing = 8
	// banFor is how long a peer that reached MaxFaults is not asked again.
	banFor = 10 * time.Minute
	// maxAttempts bounds the requests for one range of a blob, across peers.
	maxAttempts = 8
	// maxBusy bounds the Busy answers waited out for one range.
	maxBusy = 5
	// maxBusyWait bounds the wait a Busy answer asks for.
	maxBusyWait = 5 * time.Second
	// maxServingPerPeer bounds the requests served at once for one peer, so
	// that a member cannot hold every serving slot.
	maxServingPerPeer = 2
	// maxBusyReplies bounds the answers to a peer that asks more than we serve.
	maxBusyReplies = 32
)

// ErrNoPeer is returned by [Service.Fetch] when no connected peer that may be
// asked has the blob: none is connected, none has it, or the ones that have it
// were dropped. Ask again when a peer connects (see [Service.PeersChanged]).
var ErrNoPeer = errors.New("blob: no peer to fetch from")

// ServiceOptions tunes a [Service]. Referenced and Allowed are required.
type ServiceOptions struct {
	// Referenced says whether the project refers to a blob. Only referenced
	// blobs are served: a blob that was removed from the project, or that a
	// peer guesses the hash of, is answered as missing.
	Referenced func(Hash) bool
	// Allowed says whether a peer may be served and asked: an active member
	// of the project that is not suspended.
	Allowed func(transport.PeerID) bool
	// Drop, if not nil, is called when a peer reaches [MaxFaults]; it should
	// close the connections to it.
	Drop func(transport.PeerID)
	// Log receives the diagnostics and the security events (attribute
	// event=security). Nil discards them.
	Log *slog.Logger
	// Timeout bounds the wait for the answer to one request; zero means 30 s.
	Timeout time.Duration
	// MaxServing is how many requests are served at once; the others are
	// answered Busy. Zero means 8.
	MaxServing int
}

// Service pulls blobs from the peers of a transport and serves them. Create it
// with [NewService] and start it with [Service.Run].
type Service struct {
	t     transport.Transport
	store *Store
	opt   ServiceOptions
	log   *slog.Logger
	// slots limits the downloads: a download holds one while it runs.
	slots chan struct{}

	// The rest is under mu.
	mu      sync.Mutex
	up      map[transport.PeerID]struct{}
	waiting map[reqKey]*waiter
	calls   map[Hash]*call
	faults  map[transport.PeerID]int
	banned  map[transport.PeerID]time.Time
	// strikes counts the requests a peer left unanswered since it last
	// answered, so that a silent peer is asked after the others.
	strikes map[transport.PeerID]int
	// changed is closed and replaced when the set of peers changes.
	changed chan struct{}
	serving int
	busy    int
	// servingBy counts the requests being served for each peer.
	servingBy map[transport.PeerID]int
}

// reqKey identifies the one request a peer may have outstanding for a blob.
type reqKey struct {
	peer transport.PeerID
	hash Hash
}

// waiter receives the answer to a request.
type waiter struct{ ch chan result }

type result struct {
	msg Message
	err error
}

// call is a download in progress, which other callers wait for.
type call struct {
	done chan struct{}
	err  error
}

// errPeerGone ends a request whose peer disconnected.
var errPeerGone = errors.New("blob: peer disconnected")

// NewService returns a service that speaks the pull protocol over t, normally
// the bulk lane of the network node, and keeps its blobs in store. It consumes
// the events of t: t must have no other reader.
func NewService(t transport.Transport, store *Store, o ServiceOptions) (*Service, error) {
	if o.Referenced == nil || o.Allowed == nil {
		return nil, errors.New("blob: Referenced and Allowed are required")
	}
	o.Timeout = cmp.Or(o.Timeout, defaultTimeout)
	o.MaxServing = cmp.Or(o.MaxServing, defaultMaxServing)
	v := &Service{
		t: t, store: store, opt: o, log: o.Log,
		slots:     make(chan struct{}, MaxTransfers),
		up:        make(map[transport.PeerID]struct{}),
		waiting:   make(map[reqKey]*waiter),
		calls:     make(map[Hash]*call),
		faults:    make(map[transport.PeerID]int),
		banned:    make(map[transport.PeerID]time.Time),
		strikes:   make(map[transport.PeerID]int),
		servingBy: make(map[transport.PeerID]int),
		changed:   make(chan struct{}),
	}
	if v.log == nil {
		v.log = slog.New(slog.DiscardHandler)
	}
	for _, p := range t.Peers() { // those connected before Run is called
		v.up[p] = struct{}{}
	}
	return v, nil
}

// Store returns the store the service fills.
func (v *Service) Store() *Store { return v.store }

// PeersChanged returns a channel that is closed the next time a peer connects
// or disconnects: it is when to ask again for a blob that no peer had.
func (v *Service) PeersChanged() <-chan struct{} {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.changed
}

func (v *Service) changedLocked() {
	close(v.changed)
	v.changed = make(chan struct{})
}

// Run handles the events of the transport until ctx is done or the transport
// closes: it serves the requests of the peers and routes the answers to the
// requests of [Service.Fetch]. It returns the error that ended it, ctx.Err()
// when ctx did.
func (v *Service) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		ev, err := v.t.Recv(ctx)
		if err != nil {
			return err
		}
		switch ev.Kind {
		case transport.PeerUp:
			v.mu.Lock()
			v.up[ev.Peer] = struct{}{}
			v.changedLocked()
			v.mu.Unlock()
		case transport.PeerDown:
			v.peerDown(ev.Peer)
		case transport.Message:
			v.handle(ctx, &wg, ev.Peer, ev.Payload)
		}
	}
}

// peerDown forgets peer and ends the requests waiting on it.
func (v *Service) peerDown(peer transport.PeerID) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.up, peer)
	for k, w := range v.waiting {
		if k.peer == peer {
			select {
			case w.ch <- result{err: errPeerGone}:
			default:
			}
		}
	}
	v.changedLocked()
}

// handle processes one message of peer.
func (v *Service) handle(ctx context.Context, wg *sync.WaitGroup, peer transport.PeerID, payload []byte) {
	m, err := Decode(payload)
	if err != nil {
		v.misbehaved(peer, Hash{}, err.Error())
		return
	}
	switch m := m.(type) {
	case Want:
		if !v.opt.Allowed(peer) {
			v.log.Debug("blob: request from a peer that is not allowed", "peer", peer)
			return
		}
		v.mu.Lock()
		var reply Message
		switch {
		case v.serving < v.opt.MaxServing && v.servingBy[peer] < maxServingPerPeer:
			v.serving++
			v.servingBy[peer]++
		case v.busy < maxBusyReplies:
			v.busy++
			reply = Busy{Hash: m.Hash, Retry: time.Second}
		default:
			v.mu.Unlock()
			return // asked far more than we serve: say nothing
		}
		v.mu.Unlock()
		if reply != nil {
			wg.Go(func() {
				v.send(ctx, peer, reply)
				v.mu.Lock()
				v.busy--
				v.mu.Unlock()
			})
			return
		}
		wg.Go(func() {
			v.serve(ctx, peer, m)
			v.mu.Lock()
			if v.servingBy[peer]--; v.servingBy[peer] <= 0 {
				delete(v.servingBy, peer)
			}
			v.serving--
			v.mu.Unlock()
		})
	case Slice:
		v.deliver(peer, m.Hash, m)
	case Missing:
		v.deliver(peer, m.Hash, m)
	case Busy:
		v.deliver(peer, m.Hash, m)
	}
}

// deliver gives the answer m about h to the request waiting for it.
func (v *Service) deliver(peer transport.PeerID, h Hash, m Message) {
	v.mu.Lock()
	w := v.waiting[reqKey{peer, h}]
	v.mu.Unlock()
	if w == nil {
		v.log.Debug("blob: unsolicited answer", "peer", peer, "hash", h.short())
		return
	}
	select {
	case w.ch <- result{msg: m}:
	default: // an answer was delivered already
	}
}

// serve answers one request of a peer that is allowed.
func (v *Service) serve(ctx context.Context, peer transport.PeerID, w Want) {
	if !v.opt.Referenced(w.Hash) {
		v.send(ctx, peer, Missing{Hash: w.Hash})
		return
	}
	s, err := v.store.ReadSlice(w.Hash, w.Offset, w.Length)
	switch {
	case err == nil:
		v.send(ctx, peer, s)
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrCorrupt):
		v.send(ctx, peer, Missing{Hash: w.Hash})
	default:
		v.log.Warn("blob: serving a slice", "hash", w.Hash.short(), "err", err)
		v.send(ctx, peer, Missing{Hash: w.Hash})
	}
}

// send encodes and sends m, logging a failure.
func (v *Service) send(ctx context.Context, peer transport.PeerID, m Message) {
	b, err := Encode(m)
	if err == nil {
		err = v.t.Send(ctx, peer, b)
	}
	if err != nil && ctx.Err() == nil {
		v.log.Debug("blob: sending", "peer", peer, "err", err)
	}
}

// misbehaved counts a fault of peer: a forged slice or a malformed message.
// The peer is dropped at the [MaxFaults]th, and a security event logged.
func (v *Service) misbehaved(peer transport.PeerID, h Hash, why string) {
	v.mu.Lock()
	if until, ok := v.banned[peer]; ok && time.Now().Before(until) {
		v.mu.Unlock()
		return
	}
	v.faults[peer]++
	n := v.faults[peer]
	drop := n >= MaxFaults
	if drop {
		v.banned[peer] = time.Now().Add(banFor)
		delete(v.faults, peer)
	}
	v.mu.Unlock()
	v.log.Warn("blob: peer misbehaved", "event", "security", "peer", peer, "hash", h.short(), "why", why, "faults", n)
	if drop {
		v.log.Warn("blob: peer dropped", "event", "security", "peer", peer, "faults", n)
		if v.opt.Drop != nil {
			v.opt.Drop(peer)
		}
	}
}

// pending is the conversation with one peer about one blob: the requests sent
// to it and the answers still to come. Answers to an earlier request, which was
// given up on, may arrive in the middle of a later one, so a caller reads
// answers until the one it waits for. Create it with [Service.open].
type pending struct {
	v        *Service
	wt       *waiter
	key      reqKey
	deadline time.Time
}

// open registers the right to a conversation with peer about h. At most one
// is open at a time for a given peer and blob.
func (v *Service) open(peer transport.PeerID, h Hash) (*pending, error) {
	key := reqKey{peer, h}
	p := &pending{v: v, key: key, wt: &waiter{ch: make(chan result, 4)}}
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, taken := v.waiting[key]; taken {
		return nil, fmt.Errorf("blob: a request for %s to %s is outstanding", h.short(), peer)
	}
	v.waiting[key] = p.wt
	return p, nil
}

// close ends the conversation: later answers are dropped.
func (p *pending) close() {
	p.v.mu.Lock()
	delete(p.v.waiting, p.key)
	p.v.mu.Unlock()
}

// send sends w to the peer and restarts the timeout.
func (p *pending) send(ctx context.Context, w Want) error {
	b, err := Encode(w)
	if err != nil {
		return err
	}
	p.deadline = time.Now().Add(p.v.opt.Timeout)
	return p.v.t.Send(ctx, p.key.peer, b)
}

// next waits for the next answer of the peer about the blob: a [Slice],
// [Missing] or [Busy]. It fails when the peer disconnects, the timeout of the
// last send passes or ctx ends.
func (p *pending) next(ctx context.Context) (Message, error) {
	timer := time.NewTimer(time.Until(p.deadline))
	defer timer.Stop()
	select {
	case r := <-p.wt.ch:
		return r.msg, r.err
	case <-timer.C:
		return nil, fmt.Errorf("blob: %s did not answer in %v", p.key.peer, p.v.opt.Timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// pick returns the peers to ask for h, best first: those that are up, allowed,
// not banned and not in skip, with the fewest faults, then the fewest
// unanswered requests, then the fewest failures on this request (tried), then
// by name.
func (v *Service) pick(skip map[transport.PeerID]bool, tried map[transport.PeerID]int) []transport.PeerID {
	v.mu.Lock()
	peers := slices.Collect(maps.Keys(v.up))
	faults := maps.Clone(v.faults)
	strikes := maps.Clone(v.strikes)
	banned := maps.Clone(v.banned)
	v.mu.Unlock()
	now := time.Now()
	peers = slices.DeleteFunc(peers, func(p transport.PeerID) bool {
		until, isBanned := banned[p]
		return skip[p] || (isBanned && now.Before(until)) || !v.opt.Allowed(p)
	})
	slices.SortFunc(peers, func(a, b transport.PeerID) int {
		return cmp.Or(cmp.Compare(faults[a], faults[b]), cmp.Compare(strikes[a], strikes[b]),
			cmp.Compare(tried[a], tried[b]), cmp.Compare(a, b))
	})
	return peers
}

// Fetch brings the blob ref into the store: it asks the connected peers for
// it, checks each slice that arrives against the hash, and resumes from the
// groups already verified if an earlier attempt stopped. It returns nil at
// once if the blob is stored.
//
// It returns [ErrTooLarge], [*QuotaError] (see [Store.Confirm]) or
// [ErrNoSpace] before asking anyone, [ErrNoPeer] when nobody can be asked,
// [ErrSizeMismatch] when the size of ref is not the file's, and ctx's error.
// A failed Fetch keeps what it verified. At most [MaxTransfers] blobs download
// at once; the others wait. Concurrent calls for one blob share one download.
func (v *Service) Fetch(ctx context.Context, ref Ref) error {
	for {
		if v.store.Has(ref.Hash) {
			return nil
		}
		v.mu.Lock()
		c, running := v.calls[ref.Hash]
		if !running {
			c = &call{done: make(chan struct{})}
			v.calls[ref.Hash] = c
		}
		v.mu.Unlock()
		if running {
			select {
			case <-c.done:
			case <-ctx.Done():
				return ctx.Err()
			}
			if c.err == nil || ctx.Err() != nil || !isContextErr(c.err) {
				return c.err
			}
			continue // the leader was canceled, not us: take over
		}
		c.err = v.fetch(ctx, ref)
		v.mu.Lock()
		delete(v.calls, ref.Hash)
		v.mu.Unlock()
		close(c.done)
		return c.err
	}
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// fetch downloads ref.
func (v *Service) fetch(ctx context.Context, ref Ref) (err error) {
	if (ref.Size == 0) != (ref.Hash == emptyHash) {
		return fmt.Errorf("%w: %d bytes announced for %v", ErrSizeMismatch, ref.Size, ref.Hash)
	}
	if err := v.store.hold(ref.Hash.hex(), ref.Size); err != nil {
		return err
	}
	defer v.store.settle(ref.Hash) // the hold stays while a partial download is on disk
	select {
	case v.slots <- struct{}{}:
		defer func() { <-v.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	d, err := v.store.openDownload(ref.Hash, ref.Size)
	if err != nil {
		return err
	}
	finished := false
	defer func() {
		if !finished {
			if serr := d.suspend(); serr != nil && err == nil {
				err = serr
			}
		}
	}()
	st := &fetchState{missing: map[transport.PeerID]bool{}}
	for !d.complete() {
		offset, length, _ := d.next()
		if err := v.pull(ctx, d, st, offset, length); err != nil {
			if errors.Is(err, ErrSizeMismatch) { // the progress is worth nothing
				d.discard()
				finished = true
			}
			return err
		}
	}
	finished = true
	return d.finish()
}

// fetchState is what a download learns about the peers.
type fetchState struct {
	missing map[transport.PeerID]bool // peers that said they do not have the blob
}

// errBadSlice marks a slice that does not verify, which is the peer's fault.
var errBadSlice = errors.New("blob: bad slice")

// verdict is what [Service.accept] makes of a slice.
type verdict int

const (
	// rejected: the slice is bad, and counted as a fault of the peer.
	rejected verdict = iota
	// stored: the slice is good, but not the one waited for (a late answer to
	// an earlier request): keep waiting.
	stored
	// complete: the slice is good and the range asked for is stored.
	complete
	// inconclusive: the slice's claim about the size could not be settled
	// (the peer was busy, silent or did not have the file): nobody is blamed.
	inconclusive
)

// pull receives the range [offset, offset+length) of d from the peers, trying
// others when one fails, and stores it.
func (v *Service) pull(ctx context.Context, d *download, st *fetchState, offset uint64, length uint32) error {
	tried := map[transport.PeerID]int{}
	busy := 0
	for attempt := 0; attempt < maxAttempts; attempt++ {
		peers := v.pick(st.missing, tried)
		if len(peers) == 0 {
			return ErrNoPeer
		}
		peer := peers[0]
		done, retry, err := v.exchange(ctx, d, st, peer, offset, length, &busy)
		switch {
		case err != nil:
			return err
		case done:
			return nil
		}
		tried[peer]++
		if !retry {
			attempt-- // the peer asked us to wait, which is not an attempt
		}
	}
	return ErrNoPeer
}

// exchange asks peer for the range and handles its answers until one settles
// the request. It reports done when the range is stored. retry is false after
// a Busy that was waited out, so that the caller asks again at no cost. A
// non-nil error ends the download.
func (v *Service) exchange(ctx context.Context, d *download, st *fetchState, peer transport.PeerID,
	offset uint64, length uint32, busy *int,
) (done, retry bool, err error) {
	p, err := v.open(peer, d.h)
	if err != nil {
		return false, true, nil // another download of the blob is talking to the peer
	}
	defer p.close()
	if err := p.send(ctx, Want{Hash: d.h, Offset: offset, Length: length}); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return false, true, cerr
		}
		v.log.Debug("blob: request failed", "peer", peer, "hash", d.h.short(), "err", err)
		v.strike(peer)
		return false, true, nil
	}
	for {
		msg, err := p.next(ctx)
		if cerr := ctx.Err(); cerr != nil {
			return false, true, cerr
		}
		if err != nil {
			v.log.Debug("blob: request failed", "peer", peer, "hash", d.h.short(), "err", err)
			v.strike(peer)
			return false, true, nil
		}
		v.answered(peer)
		switch m := msg.(type) {
		case Missing:
			st.missing[peer] = true
			return false, true, nil
		case Busy:
			if *busy++; *busy > maxBusy {
				return false, true, nil
			}
			if err := sleep(ctx, min(m.Retry, maxBusyWait)); err != nil {
				return false, true, err
			}
			return false, false, nil
		case Slice:
			switch verdict, err := v.accept(ctx, d, peer, p, m, offset, length); {
			case err != nil:
				return false, true, err
			case verdict == complete:
				return true, true, nil
			case verdict == rejected, verdict == inconclusive:
				return false, true, nil
			}
			// stored: a late answer; the one we asked for is still to come.
		default:
			v.misbehaved(peer, d.h, "answer of a kind that is not an answer")
			return false, true, nil
		}
	}
}

// accept checks the slice m that peer sent and stores it. The slice answers a
// request for [offset, offset+length) of d, but may be a late answer to an
// earlier request: it is judged on what it says it is, by the hash alone, and
// kept if it verifies, so that a slow peer is not blamed for being slow. A
// slice that does not verify counts as a fault of the peer. It returns an
// error that ends the download: [ErrSizeMismatch], or the store's.
//
// The size the index announced is checked against the length of the file that
// the slice states. The length is authenticated only when the slice reaches
// the end of the file; for an earlier one a peer can lie about it undetected.
// So a disagreement is settled by asking for the last group of the file as the
// peer describes it: if that verifies, the index was wrong.
func (v *Service) accept(ctx context.Context, d *download, peer transport.PeerID, p *pending, m Slice, offset uint64, length uint32) (verdict, error) {
	data, size, final, err := checkSlice(m, d.h)
	if err != nil {
		v.misbehaved(peer, d.h, err.Error())
		return rejected, nil
	}
	if size == uint64(d.size) { // #nosec G115 -- the size of a download is not negative.
		if err := d.put(m.Offset, data); err != nil {
			return rejected, err
		}
		if d.covers(offset, length) {
			return complete, nil
		}
		return stored, nil
	}
	if final {
		return rejected, fmt.Errorf("%w: the file has %d bytes, the index said %d", ErrSizeMismatch, size, d.size)
	}
	switch v.confirmSize(ctx, p, size) {
	case proven:
		return rejected, fmt.Errorf("%w: the file has %d bytes, the index said %d", ErrSizeMismatch, size, d.size)
	case refuted:
		v.misbehaved(peer, d.h, "length of the file not authentic")
		return rejected, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return rejected, cerr
	}
	v.strike(peer) // not blamed, but asked after the others from now on
	return inconclusive, nil
}

// proof is what [Service.confirmSize] found out about a claimed file size.
type proof int

const (
	// unproven: the peer did not settle it, by being busy, silent or
	// without the file, or because a late answer came instead. An honest peer
	// does all of these, so nobody is blamed for it.
	unproven proof = iota
	// proven: the last group verifies at the claimed size.
	proven
	// refuted: the peer's own answer contradicts its claim.
	refuted
)

// confirmSize asks the peer of p for the last group of the file as it
// describes it, of size bytes. The answer is [proven] if that group verifies
// and ends the file at size, [refuted] if the peer sends a slice that does not,
// and [unproven] if it sends nothing usable, which proves nothing about it.
func (v *Service) confirmSize(ctx context.Context, p *pending, size uint64) proof {
	if size == 0 || size > maxOffset {
		return refuted // no file of ours is that big
	}
	offset := (size - 1) / GroupSize * GroupSize
	if err := p.send(ctx, Want{Hash: p.key.hash, Offset: offset, Length: GroupSize}); err != nil {
		return unproven
	}
	for {
		msg, err := p.next(ctx)
		if err != nil {
			return unproven
		}
		m, ok := msg.(Slice)
		if !ok {
			return unproven // Busy or Missing
		}
		if m.Offset != offset { // a late answer to the earlier request
			continue
		}
		_, got, final, err := checkSlice(m, p.key.hash)
		if err != nil || !final || got != size {
			return refuted
		}
		return proven
	}
}

// strike counts a request that peer left unanswered; answered forgives them.
func (v *Service) strike(peer transport.PeerID) {
	v.mu.Lock()
	v.strikes[peer]++
	v.mu.Unlock()
}

func (v *Service) answered(peer transport.PeerID) {
	v.mu.Lock()
	delete(v.strikes, peer)
	v.mu.Unlock()
}

// sleep waits for d or until ctx ends.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// emptyHash is the hash of the empty file.
var emptyHash = HashOf(nil)

// checkSlice verifies m, a slice of the blob h, against h. It returns the
// data, the length of the file that the slice states, and whether the slice
// reaches the end of the file, which is what authenticates that length. The
// range the slice claims must be whole groups, or end at the end of the file.
// Anything wrong wraps errBadSlice.
func checkSlice(m Slice, h Hash) (data []byte, size uint64, final bool, err error) {
	bad := func(why string) ([]byte, uint64, bool, error) {
		return nil, 0, false, fmt.Errorf("%w: %s", errBadSlice, why)
	}
	if m.Hash != h {
		return bad("not the blob asked for")
	}
	if len(m.Bao) < 8 {
		return bad("no length header")
	}
	size = binary.LittleEndian.Uint64(m.Bao)
	n := uint64(m.Length)
	end := m.Offset + n
	switch {
	case n == 0 || n > MaxSlice:
		return bad("invalid slice length")
	case m.Offset >= size || end > size || end < m.Offset:
		return bad("range outside the file")
	case n%GroupSize != 0 && end != size:
		return bad("range ends inside a group")
	}
	encoded := bytes.NewReader(m.Bao)
	buf := bytes.NewBuffer(make([]byte, 0, int(n)))
	ok, decodeErr := bao.DecodeSlice(buf, encoded, groupLog, m.Offset, n, h)
	if decodeErr != nil || !ok || encoded.Len() != 0 || buf.Len() != int(n) {
		return bad("does not match the hash")
	}
	return buf.Bytes(), size, end == size, nil
}
