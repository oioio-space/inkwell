package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oioio-space/inkwell/internal/transport"
)

const (
	// maxInvitationLen bounds the text Join accepts before parsing it.
	maxInvitationLen = 16 << 10
	// maxInvitationAddrs bounds how many addresses an invitation carries.
	maxInvitationAddrs = 64
	// maxAddrLen bounds one address of an invitation.
	maxAddrLen = 256
	// peerSegment introduces the peer identity at the end of an address.
	peerSegment = "/p2p/"
)

var errBadInvitation = errors.New("invalid invitation")

// invitation renders addrs, each ending with /p2p/<id>, as the single opaque
// text a person copies to invite another: addresses separated by spaces. At
// most maxInvitationAddrs are kept so the result always parses back.
func invitation(addrs []string) string {
	return strings.Join(addrs[:min(len(addrs), maxInvitationAddrs)], " ")
}

// parseInvitation splits an invitation made by [invitation] into the identity
// of the inviting peer and its addresses. The text comes from outside, so it
// is checked here: every address must be printable ASCII, start with "/" and
// end with /p2p/<id> for one and the same valid id. The addresses themselves
// are validated by the transport when dialing.
func parseInvitation(text string) (transport.PeerID, []string, error) {
	if len(text) > maxInvitationLen {
		return "", nil, fmt.Errorf("%w: too long", errBadInvitation)
	}
	addrs := strings.Fields(text)
	if len(addrs) == 0 || len(addrs) > maxInvitationAddrs {
		return "", nil, fmt.Errorf("%w: %d addresses", errBadInvitation, len(addrs))
	}
	var peer transport.PeerID
	for _, a := range addrs {
		id, err := addrPeer(a)
		if err != nil {
			return "", nil, err
		}
		if peer != "" && id != peer {
			return "", nil, fmt.Errorf("%w: several identities", errBadInvitation)
		}
		peer = id
	}
	return peer, addrs, nil
}

// addrPeer returns the peer identity an address ends with.
func addrPeer(a string) (transport.PeerID, error) {
	if len(a) > maxAddrLen || a[0] != '/' {
		return "", fmt.Errorf("%w: bad address", errBadInvitation)
	}
	for i := range len(a) {
		if a[i] <= ' ' || a[i] > '~' {
			return "", fmt.Errorf("%w: bad character", errBadInvitation)
		}
	}
	i := strings.LastIndex(a, peerSegment)
	if i < 0 {
		return "", fmt.Errorf("%w: no identity", errBadInvitation)
	}
	id := transport.PeerID(a[i+len(peerSegment):])
	if !transport.ValidPeerID(id) || strings.Contains(string(id), "/") {
		return "", fmt.Errorf("%w: bad identity", errBadInvitation)
	}
	return id, nil
}
