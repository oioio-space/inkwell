package app

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/mux"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// The channels of the network transport (see package mux).
const (
	chanRepo   = 0x01 // opaque frames between the webview's repos, for the bridge
	chanTrust  = 0x02 // member list synchronization (trust.Sync)
	chanAttest = 0x03 // attestation gossip
)

const (
	// defaultTick is how often the member list frontier is announced again.
	defaultTick = 15 * time.Second
	// repoBuffer bounds the events waiting for the bridge.
	repoBuffer = 64
)

// Admission decides which peers may connect: the members of the project. A
// stranger who must be let in to be admitted (a ticket, a request to join) is
// the business of the node's open window (see p2p.Node.SetOpen), not of the
// admission, so a stranger never counts as a peer of the project. It is safe
// for concurrent use.
type Admission struct {
	list atomic.Pointer[trust.List]
}

// NewAdmission returns the admission policy of the project whose member list
// is list.
func NewAdmission(list *trust.List) *Admission {
	a := &Admission{}
	a.list.Store(list)
	return a
}

// Admit reports whether peer id may connect: it is a member of the project. It
// is meant for p2p.Options.Admit.
func (a *Admission) Admit(id transport.PeerID) bool {
	key, err := p2p.KeyFromID(id)
	return err == nil && a.list.Load().Role(key) > 0
}

// reset makes list the project whose members are admitted.
func (a *Admission) reset(list *trust.List) { a.list.Store(list) }

// wasMember reports whether key is, or ever was, the subject of an op of the
// member list.
func (a *Admission) wasMember(key ed25519.PublicKey) bool {
	return slices.ContainsFunc(a.list.Load().Since(nil), func(op trust.Op) bool {
		return op.Subject.Equal(key)
	})
}

// dialTargets returns the peers worth keeping connected: the other members.
func (a *Admission) dialTargets(self transport.PeerID) map[transport.PeerID][]string {
	out := make(map[transport.PeerID][]string)
	for _, m := range a.list.Load().Members() {
		if id := p2p.IDFromKey(m.Pub); id != "" && id != self {
			out[id] = nil
		}
	}
	return out
}

// Dropper is implemented by transports that can close the connections of a
// peer ([p2p.Node.Drop]).
type Dropper interface{ Drop(who transport.PeerID) }

// Wanter is implemented by transports that can keep peers connected
// ([p2p.Node.Want]).
type Wanter interface {
	Want(peers map[transport.PeerID][]string)
}

// WiringOptions tunes [Wire]. The zero value is valid.
type WiringOptions struct {
	// Log receives the diagnostics, never payload bytes. Nil discards them.
	Log *slog.Logger
	// Save, if not nil, persists each new op of the member list.
	Save func(trust.Op) error
	// TickEvery is how often the member list is announced again; zero means 15 s.
	TickEvery time.Duration

	// onEmit, if not nil, is called by the goroutine of the wiring just before
	// an event is queued for the bridge; tests use it to observe the order of
	// the handling.
	onEmit func(transport.Event)
}

// Wiring connects a network transport to the signed member list. It splits the
// transport into channels (see package mux) and, from a single goroutine:
//
//   - synchronizes the member list with the members ([trust.Sync]);
//   - gossips the attestations of the writers and keeps the latest of each;
//   - gives the bridge the frames of the repo channel, from members only;
//   - drops a member as soon as it is revoked, and refuses it afterwards;
//   - keeps a suspended member on a restricted link: only the member list
//     passes, in both directions, so that it learns about its suspension and
//     its resumption (ADR 0021). A device that is itself suspended restricts
//     all its links.
//
// A frame sent on a channel before another one, to the same peer, is handled
// before it: an attestation is known when the changes it covers arrive.
// Create one with [Wire].
type Wiring struct {
	raw      transport.Transport
	signer   trust.Signer
	adm      *Admission // holds the member list, replaced by SwitchProject
	sync     *trust.Sync[transport.PeerID]
	att      attestStore
	log      *slog.Logger
	save     func(trust.Op) error
	onEmit   func(transport.Event)
	ctx      context.Context
	pollCtx  context.Context // already canceled: Recv drains queued events without waiting
	cancel   context.CancelFunc
	repoCh   transport.Transport
	trustCh  transport.Transport
	attestCh transport.Transport
	out      chan bridge.Envelope // events for the bridge
	cmds     chan func()          // run by the goroutine
	stopped  chan struct{}        // closed when the goroutine has ended
	up       map[transport.PeerID]struct{}
	active   map[transport.PeerID]struct{} // under mu; written by the goroutine only: the linked peers, whose member list is synchronized
	full     map[transport.PeerID]struct{} // under mu; written by the goroutine only: the active peers that are not restricted
	saved    map[trust.Hash]struct{}
	pending  map[string][]trust.Hash                              // heads to sign once this device is known to be a writer
	held     map[transport.PeerID]map[attestKey]trust.Attestation // replayed by connected peers not linked yet, verified when they are
	closeErr error

	saveWarnedAt time.Time // zero while persistence is healthy

	// fullSnapshot is immutable and published before membership notifications.
	fullSnapshot atomic.Pointer[map[transport.PeerID]struct{}]

	// rolesSent is the last Roles payload sent to the bridge.
	rolesSent []byte
	tick      time.Duration
	nOps      int
	mu        sync.Mutex
	// sealedSent is set once the webview was told the list is sealed.
	sealedSent bool
	// selfSuspended is whether this device is suspended, as the webview was told.
	selfSuspended bool
}

// Wire starts a Wiring over t, whose events it consumes: t must have no other
// reader. adm then admits the members of list, and follows the project of
// the wiring. Canceling ctx or calling [Wiring.Close] stops it and closes t.
func Wire(ctx context.Context, t transport.Transport, s trust.Signer, list *trust.List, adm *Admission, o WiringOptions) *Wiring {
	ctx, cancel := context.WithCancel(ctx)
	pollCtx, cancelPoll := context.WithCancel(context.Background())
	cancelPoll()
	subs := mux.Split(t, chanRepo, chanTrust, chanAttest)
	w := &Wiring{
		raw:      t,
		signer:   s,
		adm:      adm,
		sync:     trust.NewSync[transport.PeerID](list),
		att:      attestStore{latest: make(map[attestKey]trust.Attestation)},
		log:      o.Log,
		save:     o.Save,
		onEmit:   o.onEmit,
		ctx:      ctx,
		pollCtx:  pollCtx,
		cancel:   cancel,
		repoCh:   subs[chanRepo],
		trustCh:  subs[chanTrust],
		attestCh: subs[chanAttest],
		out:      make(chan bridge.Envelope, repoBuffer),
		cmds:     make(chan func()),
		stopped:  make(chan struct{}),
		up:       make(map[transport.PeerID]struct{}),
		active:   make(map[transport.PeerID]struct{}),
		full:     make(map[transport.PeerID]struct{}),
		saved:    savedOps(list),
		pending:  make(map[string][]trust.Hash),
		held:     make(map[transport.PeerID]map[attestKey]trust.Attestation),
		tick:     o.TickEvery,
	}
	adm.reset(list)
	w.rolesSent, w.selfSuspended = w.rolesPayload(), list.Suspended(s.Public()) // the webview learns them from Notices
	if w.log == nil {
		w.log = slog.New(slog.DiscardHandler)
	}
	if w.tick <= 0 {
		w.tick = defaultTick
	}
	go w.run()
	return w
}

// savedOps returns the hashes of the ops list holds, which are persisted
// already: they came from the store.
func savedOps(list *trust.List) map[trust.Hash]struct{} {
	saved := make(map[trust.Hash]struct{})
	for _, op := range list.Since(nil) {
		saved[op.Hash()] = struct{}{}
	}
	return saved
}

// list returns the member list of the project.
func (w *Wiring) list() *trust.List { return w.adm.list.Load() }

// Repo returns the transport for the bridge: the repo channel, restricted to
// the members that are not suspended. A peer is announced with a PeerUp once it
// is a member, is not suspended and is connected, and with a PeerDown when it
// stops being any of them (this device being suspended counts: it restricts
// all its links); frames of other peers never come out, and Send to them fails
// with [transport.ErrUnknownPeer].
func (w *Wiring) Repo() transport.Transport { return (*repoPort)(w) }

// Close stops the wiring and closes the underlying transport. It is idempotent.
func (w *Wiring) Close() error {
	w.cancel()
	<-w.stopped
	return w.closeErr
}

// do runs fn on the goroutine of the wiring and waits for it.
func (w *Wiring) do(fn func()) error {
	done := make(chan struct{})
	select {
	case w.cmds <- func() { fn(); close(done) }:
		<-done
		return nil
	case <-w.ctx.Done():
		return transport.ErrClosed
	}
}

// Issue creates and signs a member list op on behalf of this device (see
// [trust.List.Issue]), and sends it to the members.
func (w *Wiring) Issue(k trust.OpKind, subject ed25519.PublicKey, r trust.Role, name string) (trust.Op, error) {
	return w.IssueWith(trust.Op{Kind: k, Subject: subject, Role: r, Name: name})
}

// IssueWith is like [Wiring.Issue] with the fields of the op taken from tmpl
// (see [trust.List.IssueWith]); it can admit another device of a person.
func (w *Wiring) IssueWith(tmpl trust.Op) (trust.Op, error) {
	var (
		op  trust.Op
		err error
	)
	if derr := w.do(func() {
		if op, err = w.list().IssueWith(w.signer, tmpl); err != nil {
			return
		}
		w.sendTrust(w.sync.Broadcast(op))
		w.afterChange()
	}); derr != nil {
		return trust.Op{}, derr
	}
	return op, err
}

// IssueBatch issues the ops of tmpls on behalf of this device, all or nothing
// (see [trust.List.IssueBatch]), and sends them to the members.
func (w *Wiring) IssueBatch(tmpls []trust.Op) ([]trust.Op, error) {
	var (
		ops []trust.Op
		err error
	)
	if derr := w.do(func() {
		if ops, err = w.list().IssueBatch(w.signer, tmpls); err != nil {
			return
		}
		for _, op := range ops {
			w.sendTrust(w.sync.Broadcast(op))
		}
		w.afterChange()
	}); derr != nil {
		return nil, derr
	}
	return ops, err
}

// SwitchProject makes list, a replica of another project, the project of the
// wiring: the peers of the old project are announced down, and the connected
// peers that are members of the new one are announced up. save persists the
// new ops of list; the ops list holds now are taken as saved already.
func (w *Wiring) SwitchProject(list *trust.List, save func(trust.Op) error) error {
	return w.AdoptProject(list, nil, save)
}

// AdoptProject is [Wiring.SwitchProject] preceded by persist, which stores
// list. Both run on the goroutine of the wiring, the one that calls the save
// of the old project: none of its ops can be written once persist started. If
// persist fails, the project is kept and its error is returned.
func (w *Wiring) AdoptProject(list *trust.List, persist func() error, save func(trust.Op) error) error {
	var perr error
	if err := w.do(func() {
		if persist != nil {
			if perr = persist(); perr != nil {
				return
			}
		}
		for _, p := range w.linkedPeers() {
			w.deactivate(p)
		}
		w.adm.reset(list)
		w.save = save
		w.sync = trust.NewSync[transport.PeerID](list)
		w.att.reset()
		w.saved = savedOps(list)
		w.saveWarnedAt = time.Time{}
		clear(w.pending)
		w.sealedSent, w.selfSuspended, w.rolesSent = false, false, nil
		w.nOps = 0
		w.afterChange()
	}); err != nil {
		return err
	}
	return perr
}

// run is the goroutine of the wiring.
func (w *Wiring) run() {
	defer close(w.stopped)
	defer func() {
		w.closeErr = w.repoCh.Close() // closes the underlying transport
	}()
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	wake := mux.Wake(w.repoCh)
	w.afterChange()
	for w.ctx.Err() == nil {
		select {
		case fn := <-w.cmds:
			fn()
			continue
		default:
		}
		if w.step() {
			continue
		}
		select {
		case fn := <-w.cmds:
			fn()
		case <-wake:
		case <-ticker.C:
			w.sendTrust(w.sync.Tick())
			w.afterChange()
		case <-w.ctx.Done():
		}
	}
}

// poll returns the event queued on c, if any. It cancels the wiring once the
// transport is closed.
func (w *Wiring) poll(c transport.Transport) (transport.Event, bool) {
	ev, err := c.Recv(w.pollCtx)
	if errors.Is(err, transport.ErrClosed) {
		w.cancel()
	}
	return ev, err == nil
}

// step handles one queued event and reports whether there was one. The member
// list and the attestations go first, and are fully handled before a repo
// frame: everything that was sent to us before that frame is queued already,
// since the channels share one stream.
func (w *Wiring) step() bool {
	if w.control() {
		return true
	}
	ev, ok := w.poll(w.repoCh)
	if !ok {
		return false
	}
	for w.control() { // whatever was sent before ev is queued by now
	}
	if ev.Kind == transport.Message && w.isActive(ev.Peer) {
		w.emit(ev)
	}
	return true
}

// control handles one queued event of the trust or attest channel.
func (w *Wiring) control() bool {
	if ev, ok := w.poll(w.trustCh); ok {
		w.onTrust(ev)
		return true
	}
	if ev, ok := w.poll(w.attestCh); ok {
		w.onAttest(ev)
		return true
	}
	return false
}

func (w *Wiring) onTrust(ev transport.Event) {
	switch ev.Kind {
	case transport.PeerUp:
		w.up[ev.Peer] = struct{}{}
		w.reconcile()
	case transport.PeerDown:
		delete(w.up, ev.Peer)
		delete(w.held, ev.Peer)
		w.reconcile()
	case transport.Message:
		if !w.isLinked(ev.Peer) {
			return
		}
		out, err := w.sync.OnMessage(ev.Peer, ev.Payload)
		w.sendTrust(out)
		if err != nil {
			w.log.Debug("trust message", "peer", ev.Peer, "err", err)
			if errors.Is(err, trust.ErrMalformed) {
				w.drop(ev.Peer)
			}
		}
		w.afterChange()
	}
}

// afterChange saves the new ops of the member list, forgets what became
// invalid and keeps the members connected.
func (w *Wiring) afterChange() {
	// One ordered pass per change/tick; a failed save remains pending.
	if w.save != nil && len(w.saved) < w.list().Len() {
		for _, op := range w.list().Since(nil) {
			h := op.Hash()
			if _, done := w.saved[h]; done {
				continue
			}
			if err := w.save(op); err != nil {
				if now := time.Now(); w.saveWarnedAt.IsZero() || now.Sub(w.saveWarnedAt) >= time.Minute {
					w.log.Warn("saving trust op", "err", err)
					w.saveWarnedAt = now
				}
				break
			}
			w.saved[h] = struct{}{}
		}
	}
	if !w.saveWarnedAt.IsZero() && len(w.saved) == w.list().Len() {
		w.log.Info("saving trust ops recovered")
		w.saveWarnedAt = time.Time{}
	}
	if n := w.list().Len(); n != w.nOps {
		w.nOps = n
		for _, key := range w.att.prune(func(a trust.Attestation) bool { return w.list().Verify(a) == nil }) {
			if id := p2p.IDFromKey(ed25519.PublicKey(key)); id != "" {
				w.notify(bridge.Envelope{Kind: bridge.Revoked, Peer: id})
			}
		}
		if wt, ok := w.raw.(Wanter); ok {
			wt.Want(w.adm.dialTargets(w.raw.Local()))
		}
		w.announceRoles()
	}
	if w.list().Sealed() {
		w.announceSealed()
	}
	w.flushPending()
	w.reconcile()
}

// reconcile makes the linked peers those that are connected and allowed: it
// links the new ones and drops the ones that are not allowed anymore. It then
// opens the repo and attestation channels to the linked peers that may have
// them, and closes them to the others: a suspended peer, or every peer if this
// device is suspended.
func (w *Wiring) reconcile() {
	// Scan members rather than revalidating every connected stranger on each event.
	for _, p := range slices.Sorted(maps.Keys(w.adm.dialTargets(w.raw.Local()))) {
		if _, up := w.up[p]; up && !w.isLinked(p) && w.adm.Admit(p) {
			w.activate(p)
		}
	}
	for _, p := range w.linkedPeers() {
		if _, up := w.up[p]; !up {
			w.deactivate(p)
		} else if !w.adm.Admit(p) {
			w.deactivate(p)
			w.drop(p)
		}
	}
	for _, p := range w.linkedPeers() {
		w.refresh(p)
	}
}

// restricted reports whether the link to p carries the member list only.
func (w *Wiring) restricted(p transport.PeerID) bool {
	key, err := p2p.KeyFromID(p)
	return err != nil || w.list().Suspended(key) || w.list().Suspended(w.signer.Public())
}

// activate links the connected member p. Its repo and attestation channels
// open too unless the link is restricted.
func (w *Wiring) activate(p transport.PeerID) {
	open := !w.restricted(p)
	w.mu.Lock()
	w.active[p] = struct{}{}
	if open {
		w.full[p] = struct{}{}
		w.publishFull()
	}
	w.mu.Unlock()
	if open {
		w.emit(transport.Event{Kind: transport.PeerUp, Peer: p})
	}
	w.sendTrust(w.sync.OnPeerUp(p))
	if open {
		w.sendAttestations(p)
		w.releaseHeld(p)
	}
}

// refresh opens or closes the repo and attestation channels to the linked
// peer p, after a suspension or a resumption.
func (w *Wiring) refresh(p transport.PeerID) {
	open := !w.restricted(p)
	w.mu.Lock()
	_, was := w.full[p]
	switch {
	case open && !was:
		w.full[p] = struct{}{}
	case !open && was:
		delete(w.full, p)
	}
	if open != was {
		w.publishFull()
	}
	w.mu.Unlock()
	switch {
	case open && !was:
		w.emit(transport.Event{Kind: transport.PeerUp, Peer: p})
		w.sendAttestations(p)
		w.releaseHeld(p)
	case !open && was:
		w.emit(transport.Event{Kind: transport.PeerDown, Peer: p})
	}
}

// deactivate unlinks p.
func (w *Wiring) deactivate(p transport.PeerID) {
	w.mu.Lock()
	_, was := w.full[p]
	delete(w.active, p)
	delete(w.full, p)
	if was {
		w.publishFull()
	}
	w.mu.Unlock()
	w.sync.OnPeerDown(p)
	if was {
		w.emit(transport.Event{Kind: transport.PeerDown, Peer: p})
	}
}

// publishFull publishes an immutable view of the live gate. The caller holds mu.
func (w *Wiring) publishFull() {
	snapshot := maps.Clone(w.full)
	w.fullSnapshot.Store(&snapshot)
}

// isActive reports whether the repo and attestation channels are open to p.
func (w *Wiring) isActive(p transport.PeerID) bool {
	snapshot := w.fullSnapshot.Load()
	if snapshot == nil {
		return false
	}
	_, ok := (*snapshot)[p]
	return ok
}

// isLinked reports whether the member list is synchronized with p.
func (w *Wiring) isLinked(p transport.PeerID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.active[p]
	return ok
}

// activePeers returns the peers that the repo and attestation channels are
// open to, sorted.
func (w *Wiring) activePeers() []transport.PeerID {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Sorted(maps.Keys(w.full))
}

// linkedPeers returns the peers the member list is synchronized with, sorted.
func (w *Wiring) linkedPeers() []transport.PeerID {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Sorted(maps.Keys(w.active))
}

func (w *Wiring) drop(p transport.PeerID) {
	if d, ok := w.raw.(Dropper); ok {
		d.Drop(p)
	}
}

// emit queues an event for the bridge, unless the wiring stops first.
func (w *Wiring) emit(ev transport.Event) {
	if w.onEmit != nil {
		w.onEmit(ev)
	}
	w.notify(bridge.Envelope{Kind: bridge.Kind(ev.Kind), Peer: ev.Peer, Payload: ev.Payload})
}

// notify queues an envelope for the bridge, unless the wiring stops first.
// Envelopes come out in the order they went in, so an attestation reaches the
// webview before the frames that follow it on the wire.
func (w *Wiring) notify(e bridge.Envelope) {
	select {
	case w.out <- e:
	case <-w.ctx.Done():
	}
}

func (w *Wiring) sendTrust(out []trust.Outbound[transport.PeerID]) {
	for _, o := range out {
		if err := w.trustCh.Send(w.ctx, o.To, o.Data); err != nil && w.ctx.Err() == nil {
			w.log.Debug("sending trust message", "peer", o.To, "err", err)
		}
	}
}

// repoPort is the [transport.Transport] that [Wiring.Repo] returns.
type repoPort Wiring

var (
	_ transport.Transport = (*repoPort)(nil)
	_ bridge.Source       = (*repoPort)(nil)
	_ bridge.Attester     = (*repoPort)(nil)
	_ bridge.Resetter     = (*repoPort)(nil)
)

func (p *repoPort) Local() transport.PeerID { return p.raw.Local() }

func (p *repoPort) Peers() []transport.PeerID { return (*Wiring)(p).activePeers() }

func (p *repoPort) Send(ctx context.Context, to transport.PeerID, payload []byte) error {
	if !(*Wiring)(p).isActive(to) {
		return transport.ErrUnknownPeer
	}
	return p.repoCh.Send(ctx, to, payload)
}

// Reset resets peer without banning it, see [bridge.Resetter].
func (p *repoPort) Reset(who transport.PeerID) {
	if r, ok := p.raw.(bridge.Resetter); ok {
		r.Reset(who)
	}
}

func (p *repoPort) Recv(ctx context.Context) (transport.Event, error) {
	for {
		e, err := p.RecvEnvelope(ctx)
		if err != nil {
			return transport.Event{}, err
		}
		if e.Kind <= bridge.Message { // the attestation envelopes are for the bridge only
			return transport.Event{Kind: transport.Kind(e.Kind), Peer: e.Peer, Payload: e.Payload}, nil
		}
	}
}

// RecvEnvelope is [Wiring.Repo]'s Recv with the bridge's own envelopes:
// Attested, Revoked, Sealed, Roles and Suspended come out in order with the
// messages (see [bridge.Source]).
func (p *repoPort) RecvEnvelope(ctx context.Context) (bridge.Envelope, error) {
	select {
	case <-p.stopped:
		return bridge.Envelope{}, transport.ErrClosed
	default:
	}
	for {
		var e bridge.Envelope
		// Queued frames can be checked immediately; only wait when the queue is empty.
		select {
		case e = <-p.out:
		default:
			select {
			case e = <-p.out:
			case <-p.stopped:
				return bridge.Envelope{}, transport.ErrClosed
			case <-ctx.Done():
				return bridge.Envelope{}, ctx.Err()
			}
		}
		// Recheck queued work against the live gate. Keep PeerDown even after
		// departure: the router needs it to cancel the peer's sender.
		if (e.Kind == bridge.PeerUp || e.Kind == bridge.Message) && !(*Wiring)(p).isActive(e.Peer) {
			continue
		}
		return e, nil
	}
}

// Attest signs the heads of doc (see [Wiring.AttestHeads]); it makes the repo
// transport a [bridge.Attester].
func (p *repoPort) Attest(_ context.Context, doc string, heads [][bridge.HashSize]byte) error {
	hs := make([]trust.Hash, len(heads))
	for i, h := range heads {
		hs[i] = h
	}
	return (*Wiring)(p).AttestHeads(doc, hs)
}

// Notices returns the member roles, the suspension of this device if it is
// suspended, the attestations held and, if the list is sealed, the Sealed
// envelope, for a webview that just attached.
func (p *repoPort) Notices() []bridge.Envelope {
	w := (*Wiring)(p)
	var out []bridge.Envelope
	if payload := w.rolesPayload(); payload != nil {
		out = append(out, bridge.Envelope{Kind: bridge.Roles, Peer: w.raw.Local(), Payload: payload})
	}
	if by, ok := w.selfSuspension(); ok {
		out = append(out, suspendedEnvelope(by, true))
	}
	for _, a := range w.att.all() {
		if e, ok := attestedEnvelope(a); ok {
			out = append(out, e)
		}
	}
	if w.list().Sealed() {
		out = append(out, w.sealedEnvelope())
	}
	return out
}

func (p *repoPort) Close() error { return (*Wiring)(p).Close() }
