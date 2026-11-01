package bridge

import "context"

// Attester is implemented by a repo transport that controls who may write
// (package app's wiring). A [Router] over such a transport turns the Attest
// envelopes of the webview into calls to Attest and gives a newly attached
// webview the current Notices.
type Attester interface {
	// Attest signs the heads of doc with the device key and gossips them to
	// the members, ahead of the repo frames sent afterwards. It fails if this
	// device may not write, or may no longer sign.
	Attest(ctx context.Context, doc string, heads [][HashSize]byte) error
	// Notices returns the Attested, Sealed, Roles and Suspended envelopes
	// describing the current state, for a webview that just attached. It never blocks on
	// the network.
	Notices() []Envelope
}

// Source is implemented by a repo transport that reports more than
// [transport.Event]s: its envelopes (Attested, Revoked, Sealed, Roles,
// Suspended) are ordered
// with the Message envelopes, so an attestation precedes the changes it
// covers. [Router.Run] reads it instead of Recv.
type Source interface {
	// RecvEnvelope blocks for the next envelope, like [transport.Transport.Recv].
	RecvEnvelope(ctx context.Context) (Envelope, error)
}
