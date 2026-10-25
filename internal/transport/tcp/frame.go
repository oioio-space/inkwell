package tcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/oioio-space/inkwell/internal/transport"
)

const (
	headerLen = 4
	// smallFrame is the largest payload written together with its header in a
	// single Write; bigger ones avoid the copy with two writes.
	smallFrame = 64 << 10
	// readChunk is the first allocation for a payload; the buffer then grows
	// as bytes really arrive, so a peer that only claims a large length costs
	// little memory.
	readChunk = 64 << 10
)

// WriteFrame writes payload to w as one frame: a 4-byte big-endian length
// followed by the payload. It returns [transport.ErrFrameTooLarge] if payload
// exceeds [transport.MaxFrame]. Callers must serialise concurrent writes to
// the same w.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > transport.MaxFrame {
		return transport.ErrFrameTooLarge
	}
	if len(payload) <= smallFrame {
		buf := make([]byte, headerLen+len(payload))
		binary.BigEndian.PutUint32(buf, uint32(len(payload))) // #nosec G115 -- bounded by MaxFrame above.
		copy(buf[headerLen:], payload)
		if _, err := w.Write(buf); err != nil {
			return fmt.Errorf("tcp: write frame: %w", err)
		}
		return nil
	}
	var hdr [headerLen]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload))) // #nosec G115 -- bounded by MaxFrame above.
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("tcp: write frame header: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("tcp: write frame payload: %w", err)
	}
	return nil
}

// ReadFrame reads one frame from r. It reads the announced length first and
// returns [transport.ErrFrameTooLarge] if it exceeds limit, before allocating
// anything: no buffer larger than limit is ever allocated. A stream that ends
// before the first byte gives [io.EOF]; one that ends inside a frame gives
// [io.ErrUnexpectedEOF].
func ReadFrame(r io.Reader, limit int) ([]byte, error) {
	return readFrame(r, limit, nil)
}

// readFrame is ReadFrame with a hook: reserve, if not nil, is called with the
// payload size once it passed the limit check and before anything is
// allocated; its error aborts the read.
func readFrame(r io.Reader, limit int, reserve func(size int) error) ([]byte, error) {
	var hdr [headerLen]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("tcp: read frame header: %w", err)
	}
	size := uint64(binary.BigEndian.Uint32(hdr[:]))
	if limit < 0 || size > uint64(limit) {
		return nil, transport.ErrFrameTooLarge
	}
	n := int(size)
	if reserve != nil {
		if err := reserve(n); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, 0, min(n, readChunk))
	for len(buf) < n {
		if len(buf) == cap(buf) {
			grown := make([]byte, len(buf), min(n, 2*cap(buf)))
			copy(grown, buf)
			buf = grown
		}
		m, err := io.ReadFull(r, buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+m]
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return nil, fmt.Errorf("tcp: read frame payload: %w", err)
		}
	}
	return buf, nil
}
