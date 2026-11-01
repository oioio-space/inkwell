// SPDX-License-Identifier: MPL-2.0

package blob

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Limits of the wire format. Every byte of a message comes from a peer.
const (
	// GroupSize is the unit of verification: a slice is checked against the
	// hash as a whole, in groups of 16 KiB (2^4 chunks of the Bao tree). A
	// forged byte costs at most one group to detect.
	GroupSize = 16 << 10

	// MaxSlice is the most data one [Want] asks for and one [Slice] carries.
	MaxSlice = 1 << 20
	// maxBao bounds the encoding of a slice: its data, the length header and
	// the parent hashes on the paths to it, which never reach 16 KiB for the
	// files this package accepts.
	maxBao = MaxSlice + 16<<10
	// maxOffset bounds the offset of a message, far above any accepted file.
	maxOffset = 1 << 40
	// maxRetry bounds the delay of a [Busy].
	maxRetry = time.Minute
)

// Message kinds, the first byte of a frame.
const (
	kindWant    = 1
	kindSlice   = 2
	kindMissing = 3
	kindBusy    = 4
)

const (
	hashEnd    = 1 + HashSize
	wantLen    = hashEnd + 8 + 4
	sliceHead  = wantLen
	missingLen = hashEnd
	busyLen    = hashEnd + 4
)

// ErrMalformed is wrapped by every error of [Decode] and [Encode].
var ErrMalformed = errors.New("blob: malformed message")

// Message is one frame of the pull protocol: [Want], [Slice], [Missing] or
// [Busy]. The protocol is request and response, one request at a time for a
// given peer and hash.
type Message interface{ message() }

// Want asks for the bytes [Offset, Offset+Length) of the blob Hash. Offset
// and Length are multiples of [GroupSize], and Length is at most [MaxSlice].
// The answer is a [Slice], [Missing] or [Busy]. A request that reaches past
// the end of the file is clamped to it.
type Want struct {
	// Hash names the blob.
	Hash Hash
	// Offset is where the range starts.
	Offset uint64
	// Length is the size of the range.
	Length uint32
}

// Slice answers a [Want] with the Bao slice encoding of the range: the length
// of the file, the hashes along the path to the range, and the data. The
// receiver checks it against the Hash alone, so the sender needs no trust.
type Slice struct {
	// Hash names the blob.
	Hash Hash
	// Bao is the slice encoding (see lukechampine.com/blake3/bao).
	Bao []byte
	// Offset is where the data starts, as asked.
	Offset uint64
	// Length is the number of data bytes, less than asked at the end of the file.
	Length uint32
}

// Missing answers a [Want] for a blob that the peer does not have or does not
// serve. Both look the same on purpose.
type Missing struct {
	// Hash names the blob.
	Hash Hash
}

// Busy answers a [Want] when the peer is serving as much as it will: ask
// again after Retry.
type Busy struct {
	// Hash names the blob that was asked for.
	Hash Hash
	// Retry is how long to wait, at most one minute.
	Retry time.Duration
}

func (Want) message()    {}
func (Slice) message()   {}
func (Missing) message() {}
func (Busy) message()    {}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformed, fmt.Sprintf(format, args...))
}

// checkRange validates the range of a Want or a Slice. exact says that
// length must be a multiple of GroupSize, as in a Want.
func checkRange(offset uint64, length uint32, exact bool) error {
	switch {
	case length == 0 || length > MaxSlice:
		return malformed("length %d", length)
	case exact && length%GroupSize != 0:
		return malformed("length %d is not a multiple of the group size", length)
	case offset%GroupSize != 0 || offset > maxOffset:
		return malformed("offset %d", offset)
	}
	return nil
}

// Encode serializes m into a frame:
//
//	Want:    1 | hash | offset u64 | length u32
//	Slice:   2 | hash | offset u64 | length u32 | bao
//	Missing: 3 | hash
//	Busy:    4 | hash | retry u32, in milliseconds
//
// Integers are big-endian. It returns an error wrapping [ErrMalformed] for a
// message that [Decode] would refuse.
func Encode(m Message) ([]byte, error) {
	switch m := m.(type) {
	case Want:
		if err := checkRange(m.Offset, m.Length, true); err != nil {
			return nil, err
		}
		b := append(make([]byte, 0, wantLen), kindWant)
		b = append(b, m.Hash[:]...)
		b = binary.BigEndian.AppendUint64(b, m.Offset)
		return binary.BigEndian.AppendUint32(b, m.Length), nil
	case Slice:
		if err := checkRange(m.Offset, m.Length, false); err != nil {
			return nil, err
		}
		if len(m.Bao) < 8+int(m.Length) || len(m.Bao) > maxBao {
			return nil, malformed("slice encoding of %d bytes for %d of data", len(m.Bao), m.Length)
		}
		b := append(make([]byte, 0, sliceHead+len(m.Bao)), kindSlice)
		b = append(b, m.Hash[:]...)
		b = binary.BigEndian.AppendUint64(b, m.Offset)
		b = binary.BigEndian.AppendUint32(b, m.Length)
		return append(b, m.Bao...), nil
	case Missing:
		return append(append(make([]byte, 0, missingLen), kindMissing), m.Hash[:]...), nil
	case Busy:
		if m.Retry < 0 || m.Retry > maxRetry || m.Retry%time.Millisecond != 0 {
			return nil, malformed("retry %v", m.Retry)
		}
		b := append(append(make([]byte, 0, busyLen), kindBusy), m.Hash[:]...)
		return binary.BigEndian.AppendUint32(b, uint32(m.Retry/time.Millisecond)), nil // #nosec G115 -- ≤ maxRetry.
	}
	return nil, malformed("message %T", m)
}

// Decode parses a frame built by [Encode]. The Bao of a returned [Slice]
// aliases b. It returns an error wrapping [ErrMalformed] for an unknown kind,
// a bad length, a misaligned or oversized range, or trailing bytes: what it
// accepts, [Encode] gives back byte for byte.
func Decode(b []byte) (Message, error) {
	if len(b) < 1 {
		return nil, malformed("empty frame")
	}
	var h Hash
	if len(b) >= hashEnd {
		copy(h[:], b[1:hashEnd])
	}
	switch b[0] {
	case kindWant:
		if len(b) != wantLen {
			return nil, malformed("want of %d bytes", len(b))
		}
		w := Want{Hash: h, Offset: binary.BigEndian.Uint64(b[hashEnd:]), Length: binary.BigEndian.Uint32(b[hashEnd+8:])}
		if err := checkRange(w.Offset, w.Length, true); err != nil {
			return nil, err
		}
		return w, nil
	case kindSlice:
		if len(b) < sliceHead {
			return nil, malformed("slice of %d bytes", len(b))
		}
		s := Slice{Hash: h, Offset: binary.BigEndian.Uint64(b[hashEnd:]), Length: binary.BigEndian.Uint32(b[hashEnd+8:]), Bao: b[sliceHead:]}
		if err := checkRange(s.Offset, s.Length, false); err != nil {
			return nil, err
		}
		if len(s.Bao) < 8+int(s.Length) || len(s.Bao) > maxBao {
			return nil, malformed("slice encoding of %d bytes for %d of data", len(s.Bao), s.Length)
		}
		return s, nil
	case kindMissing:
		if len(b) != missingLen {
			return nil, malformed("missing of %d bytes", len(b))
		}
		return Missing{Hash: h}, nil
	case kindBusy:
		if len(b) != busyLen {
			return nil, malformed("busy of %d bytes", len(b))
		}
		retry := time.Duration(binary.BigEndian.Uint32(b[hashEnd:])) * time.Millisecond
		if retry > maxRetry {
			return nil, malformed("retry %v", retry)
		}
		return Busy{Hash: h, Retry: retry}, nil
	}
	return nil, malformed("kind %d", b[0])
}
