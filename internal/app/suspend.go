package app

import (
	"bytes"
	"cmp"
	"slices"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
	"github.com/oioio-space/inkwell/internal/trust"
)

// rolesPayload returns the [bridge.Roles] payload describing the member list:
// the role and the suspension of each device. It is nil if the list does not
// fit the bridge, which a list of at most 1024 ops cannot do.
func (w *Wiring) rolesPayload() []byte {
	members := w.list().Members()
	rs := make([]bridge.MemberRole, 0, len(members))
	for _, m := range members {
		if id := p2p.IDFromKey(m.Pub); id != "" {
			rs = append(rs, bridge.MemberRole{Peer: id, Role: uint8(m.Role), Suspended: m.Suspended})
		}
	}
	payload, err := bridge.EncodeRoles(rs)
	if err != nil {
		return nil
	}
	return payload
}

// announceRoles tells the webview what changed in the roles of the members,
// and whether this device was suspended or let back.
func (w *Wiring) announceRoles() {
	if payload := w.rolesPayload(); payload != nil && !bytes.Equal(payload, w.rolesSent) {
		w.rolesSent = payload
		w.notify(bridge.Envelope{Kind: bridge.Roles, Peer: w.raw.Local(), Payload: payload})
	}
	by, now := w.selfSuspension()
	if now == w.selfSuspended {
		return
	}
	w.selfSuspended = now
	if !now {
		by = w.lastResumeBy()
	}
	w.notify(suspendedEnvelope(cmp.Or(by, w.raw.Local()), now))
}

// selfSuspension reports whether this device is suspended, and by whom.
func (w *Wiring) selfSuspension() (by transport.PeerID, suspended bool) {
	self := w.signer.Public()
	for _, m := range w.list().Members() {
		if m.Suspended && m.Pub.Equal(self) {
			return p2p.IDFromKey(m.SuspendedBy), true
		}
	}
	return "", false
}

// lastResumeBy returns the author of the latest resumption of this device, or
// "" if there is none.
func (w *Wiring) lastResumeBy() transport.PeerID {
	self := w.signer.Public()
	ops := w.list().Since(nil)
	for _, op := range slices.Backward(ops) {
		if op.Kind == trust.OpResume && op.Subject.Equal(self) {
			return p2p.IDFromKey(op.Author)
		}
	}
	return ""
}

// suspendedEnvelope tells the webview that this device was suspended (on) or
// let back by the admin by.
func suspendedEnvelope(by transport.PeerID, on bool) bridge.Envelope {
	payload := []byte{0}
	if on {
		payload[0] = 1
	}
	return bridge.Envelope{Kind: bridge.Suspended, Peer: by, Payload: payload}
}
