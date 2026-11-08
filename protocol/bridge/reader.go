// SPDX-License-Identifier: MPL-2.0

package bridge

import (
	"encoding/binary"
	"fmt"

	transport "github.com/oioio-space/inkwell/protocol"
)

// appendPeer appends a peer as len u8 | peer; the caller has checked the peer.
func appendPeer(b []byte, p transport.PeerID) []byte {
	return append(append(b, byte(len(p))), p...) // #nosec G115 -- ValidPeerID bounds it to 64.
}

// reader reads big-endian fields and remembers the first overrun or invalid
// field; after that it reads zeros.
type reader struct {
	err error
	b   []byte
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: "+format, append([]any{ErrMalformed}, args...)...)
	}
}

func (r *reader) take(n int) []byte {
	if r.err != nil || n > len(r.b) {
		r.fail("truncated")
		return make([]byte, n)
	}
	out := r.b[:n:n]
	r.b = r.b[n:]
	return out
}

func (r *reader) u8() uint8 { return r.take(1)[0] }

func (r *reader) u16() uint16 { return binary.BigEndian.Uint16(r.take(2)) }

func (r *reader) u32() uint32 { return binary.BigEndian.Uint32(r.take(4)) }

// count reads an entry count u16 and refuses one above limit.
func (r *reader) count(limit int) int {
	n := int(r.u16())
	if n > limit {
		r.fail("%d entries", n)
		return 0
	}
	return n
}

// peer reads a peer as len u8 | peer, and refuses an invalid one ([appendPeer]).
func (r *reader) peer() transport.PeerID {
	p := transport.PeerID(r.take(int(r.u8())))
	if r.err == nil && !transport.ValidPeerID(p) {
		r.fail("invalid peer")
	}
	return p
}

// done returns the first error of the reader, or one if bytes are left.
func (r *reader) done() error {
	if r.err == nil && len(r.b) != 0 {
		r.fail("trailing bytes")
	}
	return r.err
}
