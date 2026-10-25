package app

import (
	"cmp"
	"maps"
	"slices"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

// attestKey identifies the attestations that supersede each other: those of
// one writer for one document.
type attestKey struct{ doc, signer string }

// attestStore keeps the latest attestation of each (document, signer). It is
// used by the goroutine of a [Wiring] only.
type attestStore struct {
	latest map[attestKey]trust.Attestation
}

// add stores a unless a newer or equal one (by Seq) is held, and reports
// whether a is new.
func (s *attestStore) add(a trust.Attestation) bool {
	k := attestKey{a.Doc, string(a.Signer)}
	if cur, ok := s.latest[k]; ok && cur.Seq >= a.Seq {
		return false
	}
	s.latest[k] = a
	return true
}

// all returns the stored attestations, ordered by document then signer.
func (s *attestStore) all() []trust.Attestation {
	out := slices.Collect(maps.Values(s.latest))
	slices.SortFunc(out, func(x, y trust.Attestation) int {
		return cmp.Or(cmp.Compare(x.Doc, y.Doc), cmp.Compare(string(x.Signer), string(y.Signer)))
	})
	return out
}

// prune forgets the attestations for which keep is false.
func (s *attestStore) prune(keep func(trust.Attestation) bool) {
	maps.DeleteFunc(s.latest, func(_ attestKey, a trust.Attestation) bool { return !keep(a) })
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
	if err := w.list.Verify(a); err != nil {
		return err
	}
	if !w.att.add(a) {
		return nil
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
