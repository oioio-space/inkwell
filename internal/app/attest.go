package app

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// ErrSealed is returned when this device is asked to sign while the member
// list is sealed: membership is frozen and possibly stale, so the device
// vouches for nothing more. The user must create a new project (the webview is
// told with a [bridge.Sealed] envelope).
var ErrSealed = errors.New("app: member list sealed")

// maxPending bounds the documents whose heads wait for this device to be
// admitted.
const maxPending = 64

// attestKey identifies the attestations that supersede each other: those of
// one writer for one document.
type attestKey struct{ doc, signer string }

// attestStore keeps the latest attestation of each (document, signer). Only
// the goroutine of a [Wiring] writes it; the lock lets the bridge read it.
type attestStore struct {
	latest map[attestKey]trust.Attestation
	mu     sync.Mutex
}

// add stores a unless a newer or equal one (by Seq) is held, and reports
// whether a is new.
func (s *attestStore) add(a trust.Attestation) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := attestKey{a.Doc, string(a.Signer)}
	if cur, ok := s.latest[k]; ok && cur.Seq >= a.Seq {
		return false
	}
	s.latest[k] = a
	return true
}

// all returns the stored attestations, ordered by document then signer.
func (s *attestStore) all() []trust.Attestation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Collect(maps.Values(s.latest))
	slices.SortFunc(out, func(x, y trust.Attestation) int {
		return cmp.Or(cmp.Compare(x.Doc, y.Doc), cmp.Compare(string(x.Signer), string(y.Signer)))
	})
	return out
}

// seq returns the Seq of the latest attestation of doc by signer, or 0.
func (s *attestStore) seq(doc string, signer []byte) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latest[attestKey{doc, string(signer)}].Seq
}

// reset forgets every attestation.
func (s *attestStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.latest)
}

// prune forgets the attestations for which keep is false, and returns the
// signers that lost some, in no particular order and without duplicates.
func (s *attestStore) prune(keep func(trust.Attestation) bool) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	gone := make(map[string]struct{})
	maps.DeleteFunc(s.latest, func(_ attestKey, a trust.Attestation) bool {
		if keep(a) {
			return false
		}
		gone[string(a.Signer)] = struct{}{}
		return true
	})
	return slices.Collect(maps.Keys(gone))
}

// Attest records an attestation made by this device's webview and gossips it
// to the members. It fails if the attestation is not valid for the project or
// its signer is not a writer.
func (w *Wiring) Attest(a trust.Attestation) error {
	var err error
	if derr := w.do(func() { err = w.offerAttest(a, "") }); derr != nil {
		return derr
	}
	return err
}

// AttestHeads signs the heads of doc on behalf of this device, which must be a
// writer, and gossips the attestation to the members ahead of whatever is sent
// to them next. It is how the webview vouches for the changes it holds.
//
// It fails with [trust.ErrNotWriter] for a reader or a removed member and with
// [ErrSealed] once the member list is sealed. A device that the list does not
// know yet, such as a guest whose admission is still in flight, is not refused:
// the heads are kept and signed if it turns out to be a writer.
func (w *Wiring) AttestHeads(doc string, heads []trust.Hash) error {
	var err error
	if derr := w.do(func() { err = w.attestHeads(doc, heads) }); derr != nil {
		return derr
	}
	return err
}

func (w *Wiring) attestHeads(doc string, heads []trust.Hash) error {
	self := w.signer.Public()
	switch role := w.list().Role(self); {
	case w.list().Sealed():
		w.announceSealed()
		return ErrSealed
	case role == 0 && !w.adm.wasMember(self):
		if _, held := w.pending[doc]; held || len(w.pending) < maxPending {
			w.pending[doc] = slices.Clone(heads)
		}
		return nil
	case role < trust.RoleWriter:
		return trust.ErrNotWriter
	}
	delete(w.pending, doc)
	return w.sign(doc, heads)
}

// sign attests heads as this device and offers the attestation.
func (w *Wiring) sign(doc string, heads []trust.Hash) error {
	self := w.signer.Public()
	// Time keeps Seq growing across restarts, since attestations are not
	// persisted and the members keep the highest Seq they saw.
	seq := max(w.att.seq(doc, self)+1, uint64(time.Now().UnixMilli())) // #nosec G115 -- the clock is after 1970.
	a, err := trust.NewAttestation(w.signer, w.list().Project(), doc, heads, seq)
	if err != nil {
		return err
	}
	return w.offerAttest(a, "")
}

// flushPending signs the heads kept for the documents, once this device is
// known to be a writer, and forgets them if it is a reader.
func (w *Wiring) flushPending() {
	if len(w.pending) == 0 {
		return
	}
	role := w.list().Role(w.signer.Public())
	if role == 0 && !w.adm.wasMember(w.signer.Public()) {
		return // still unknown
	}
	pending := w.pending
	w.pending = make(map[string][]trust.Hash)
	if role < trust.RoleWriter || w.list().Sealed() {
		return
	}
	for _, doc := range slices.Sorted(maps.Keys(pending)) {
		if err := w.sign(doc, pending[doc]); err != nil {
			w.log.Debug("signing kept heads", "err", err)
		}
	}
}

// announceSealed tells the webview, once, that the member list is sealed.
func (w *Wiring) announceSealed() {
	if w.sealedSent {
		return
	}
	w.sealedSent = true
	w.notify(w.sealedEnvelope())
}

func (w *Wiring) sealedEnvelope() bridge.Envelope {
	return bridge.Envelope{Kind: bridge.Sealed, Peer: w.raw.Local()}
}

// attestedEnvelope reports a to the webview. It reports false for an
// attestation the bridge cannot carry, which [List.Verify] rules out.
func attestedEnvelope(a trust.Attestation) (bridge.Envelope, bool) {
	h := bridge.Heads{Doc: a.Doc, Heads: make([][bridge.HashSize]byte, len(a.Heads))}
	for i, x := range a.Heads {
		h.Heads[i] = x
	}
	payload, err := bridge.EncodeHeads(h)
	if err != nil {
		return bridge.Envelope{}, false
	}
	return bridge.Envelope{Kind: bridge.Attested, Peer: p2p.IDFromKey(a.Signer), Payload: payload}, true
}

// Attestations returns the latest attestation of each (document, signer),
// ordered by document then signer.
func (w *Wiring) Attestations() []trust.Attestation {
	var out []trust.Attestation
	if err := w.do(func() { out = w.att.all() }); err != nil {
		return nil
	}
	return out
}

// onAttest handles a frame of the attest channel: a gossiped attestation.
func (w *Wiring) onAttest(ev transport.Event) {
	if ev.Kind != transport.Message || !w.isActive(ev.Peer) {
		return
	}
	a, err := trust.DecodeAttestation(ev.Payload)
	if err != nil {
		w.log.Debug("attestation", "peer", ev.Peer, "err", err)
		w.drop(ev.Peer)
		return
	}
	if err := w.offerAttest(a, ev.Peer); err != nil {
		w.log.Debug("attestation", "peer", ev.Peer, "err", err)
	}
}

// offerAttest stores a, if it is valid and new, and relays it to the active
// peers except from. A duplicate (same or older Seq) is not relayed, which
// stops the gossip.
func (w *Wiring) offerAttest(a trust.Attestation, from transport.PeerID) error {
	if err := w.verify(a); err != nil {
		return err
	}
	if !w.att.add(a) {
		return nil
	}
	if e, ok := attestedEnvelope(a); ok {
		w.notify(e)
	}
	data := trust.EncodeAttestation(a)
	for _, p := range w.activePeers() {
		if p != from {
			w.sendAttest(p, data)
		}
	}
	return nil
}

// sendAttestations sends p the latest attestation of each (document, signer).
func (w *Wiring) sendAttestations(p transport.PeerID) {
	for _, a := range w.att.all() {
		w.sendAttest(p, trust.EncodeAttestation(a))
	}
}

func (w *Wiring) sendAttest(p transport.PeerID, data []byte) {
	if err := w.attestCh.Send(w.ctx, p, data); err != nil && w.ctx.Err() == nil {
		w.log.Debug("sending attestation", "peer", p, "err", err)
	}
}
