// SPDX-License-Identifier: MPL-2.0

// Package protocol defines the shared protocol limits and opaque peer identifiers.
package protocol

// maxPeerIDLen bounds a [PeerID] so it fits a one-byte length prefix.
const maxPeerIDLen = 64

// PeerID identifies a peer. It is opaque to callers and is valid when
// [ValidPeerID] reports true.
type PeerID string

// ValidPeerID reports whether id is 1 to 64 bytes of printable,
// non-space ASCII. Anything a remote peer claims as its identity must pass
// this check before it is used as a map key, logged or displayed.
func ValidPeerID(id PeerID) bool {
	if len(id) == 0 || len(id) > maxPeerIDLen {
		return false
	}
	for i := range len(id) {
		if c := id[i]; c <= ' ' || c > '~' {
			return false
		}
	}
	return true
}

// MaxFrame is the legacy transport payload limit, allowing room for its bridge envelope.
const MaxFrame = 64<<20 - 3 - maxPeerIDLen
