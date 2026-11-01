package p2p

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/transport"
)

// IDFromKey returns the PeerID of the node that holds the private half of
// pub. It is the canonical libp2p encoding of an Ed25519 identity, 52
// printable characters, and so a valid [transport.PeerID]. It returns "" if
// pub is not an Ed25519 public key.
func IDFromKey(pub ed25519.PublicKey) transport.PeerID {
	if identity.ValidatePublicKey(pub) != nil {
		return ""
	}
	pk, err := crypto.UnmarshalEd25519PublicKey(pub)
	if err != nil {
		return ""
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return ""
	}
	return transport.PeerID(id.String())
}

// KeyFromID returns the Ed25519 public key a PeerID was made from; it is the
// inverse of [IDFromKey]. It fails for anything else: a malformed ID, another
// encoding of the same identity, or an identity of another key type.
func KeyFromID(id transport.PeerID) (ed25519.PublicKey, error) {
	p, ok := parseID(id)
	if !ok {
		return nil, fmt.Errorf("p2p: %q is not a peer id", id)
	}
	pk, err := p.ExtractPublicKey()
	if err != nil {
		return nil, fmt.Errorf("p2p: public key of %q: %w", id, err)
	}
	if pk.Type() != crypto.Ed25519 {
		return nil, errors.New("p2p: peer id is not an Ed25519 identity")
	}
	raw, err := pk.Raw()
	if err != nil {
		return nil, fmt.Errorf("p2p: public key of %q: %w", id, err)
	}
	if err := identity.ValidatePublicKey(raw); err != nil {
		return nil, err
	}
	return ed25519.PublicKey(raw), nil
}

// parseID converts a PeerID to the libp2p form. It accepts only the canonical
// encoding, so that two spellings of one identity cannot become two map keys.
func parseID(id transport.PeerID) (peer.ID, bool) {
	if !transport.ValidPeerID(id) {
		return "", false
	}
	p, err := peer.Decode(string(id))
	if err != nil || p.String() != string(id) {
		return "", false
	}
	return p, true
}
