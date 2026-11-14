package instance

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// Version is the version of the hand-off wire format this build speaks.
const Version = 1

// Bounds of a request (plan section 4.1): checked by the encoder before any byte is written and
// by the decoder before anything is allocated.
const (
	// MaxArgs is the number of arguments a request carries; the rest are counted in Dropped.
	MaxArgs = 32
	// MaxArg is the length in bytes of one argument (Linux PATH_MAX).
	MaxArg = 4096
	// MaxCwd is the length in bytes of the working directory.
	MaxCwd = 4096
	// MaxActivation is the length in bytes of the activation token.
	MaxActivation = 1024
	// MaxFrame is the largest body of a frame: version, ID, the three strings at their bounds,
	// Dropped, the count and 32 arguments of 4096 bytes, each string with its 2-byte length.
	MaxFrame = 1 + 16 + (2 + MaxCwd) + (2 + MaxActivation) + 2 + 1 + MaxArgs*(2+MaxArg)
)

// Status is the one-byte reply of the primary to a request.
type Status uint8

// The replies, in their wire values.
const (
	// StatusAccepted: the request is queued (or was already, for a repeated ID).
	StatusAccepted Status = 0
	// StatusClosing: the primary is shutting down; the launch waits for the lock.
	StatusClosing Status = 1
	// StatusBusy: the primary cannot queue now (still starting, or its queue is full); the launch
	// backs off and tries again.
	StatusBusy Status = 2
	// StatusMalformed: the frame broke the format; a bug, not retried.
	StatusMalformed Status = 3
	// StatusUnsupported: the frame is of another version.
	StatusUnsupported Status = 4
)

func (s Status) String() string {
	switch s {
	case StatusAccepted:
		return "accepted"
	case StatusClosing:
		return "closing"
	case StatusBusy:
		return "busy"
	case StatusMalformed:
		return "malformed"
	case StatusUnsupported:
		return "unsupported"
	}
	return fmt.Sprintf("status(%d)", byte(s))
}

var (
	// errInvalid is a request outside the bounds, refused by Encode.
	errInvalid = errors.New("instance: request out of bounds")
	// errMalformed is a frame that breaks the format.
	errMalformed = errors.New("instance: malformed request")
	// errVersion is a frame of another version.
	errVersion = errors.New("instance: unsupported request version")
)

// Request is what a launch hands to the primary: its arguments as given on the command line, its
// working directory (the arguments are resolved against it, never against the primary's), and
// the activation token that lets the window come to the front.
type Request struct {
	// ID is random per launch; the primary ignores a repeated ID, so a request retried after a
	// lost acknowledgement is applied once.
	ID [16]byte
	// Cwd is the absolute working directory of the launch, 1 to MaxCwd bytes, no NUL.
	Cwd string
	// Activation is XDG_ACTIVATION_TOKEN, else DESKTOP_STARTUP_ID: at most MaxActivation bytes of
	// printable ASCII (0x21 to 0x7E); "" when there is none.
	Activation string
	// Dropped counts the arguments beyond the MaxArgs carried.
	Dropped uint16
	// Args are at most MaxArgs arguments of 1 to MaxArg bytes, no NUL.
	Args []string
}

// NewRequest builds the request of a launch, within the bounds whatever it was given: a random ID;
// cwd as given, or "." when it is out of bounds (relative arguments are then not found); the
// activation token, or "" when it is out of bounds; the arguments as given, empty ones left out, a
// longer one cut to MaxArg bytes (still refused as too long by the primary), those beyond MaxArgs
// counted in Dropped.
func NewRequest(args []string, cwd, activation string) Request {
	r := Request{Cwd: cwd, Activation: activation}
	_, _ = rand.Read(r.ID[:]) // never fails (crypto/rand)
	if !validPath(r.Cwd, MaxCwd) {
		r.Cwd = "."
	}
	if len(r.Activation) > MaxActivation || !printable(r.Activation) {
		r.Activation = ""
	}
	dropped := 0
	for _, a := range args {
		a, _, _ = strings.Cut(a, "\x00") // none in a command line
		switch {
		case a == "":
		case len(r.Args) == MaxArgs:
			dropped++
		default:
			r.Args = append(r.Args, a[:min(len(a), MaxArg)])
		}
	}
	r.Dropped = uint16(min(dropped, math.MaxUint16)) // #nosec G115 -- bounded just above
	return r
}

// check reports whether r is within the bounds of the wire format.
func (r Request) check() error {
	if !validPath(r.Cwd, MaxCwd) {
		return fmt.Errorf("%w: working directory of %d bytes", errInvalid, len(r.Cwd))
	}
	if len(r.Activation) > MaxActivation || !printable(r.Activation) {
		return fmt.Errorf("%w: activation token", errInvalid)
	}
	if len(r.Args) > MaxArgs {
		return fmt.Errorf("%w: %d arguments", errInvalid, len(r.Args))
	}
	for _, a := range r.Args {
		if !validPath(a, MaxArg) {
			return fmt.Errorf("%w: argument of %d bytes", errInvalid, len(a))
		}
	}
	return nil
}

// validPath reports whether s has 1 to limit bytes and no NUL.
func validPath(s string, limit int) bool {
	return len(s) >= 1 && len(s) <= limit && strings.IndexByte(s, 0) < 0
}

// printable reports whether every byte of s is printable ASCII other than the space.
func printable(s string) bool {
	for i := range len(s) {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// Encode returns the frame of r: a big-endian u32 length, then the body
//
//	version:u8 id:16B cwd activation dropped:u16 count:u8 arg{count}
//
// each string as a u16 length and its bytes. It fails, writing nothing, when r is out of bounds.
func (r Request) Encode() ([]byte, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	// One frame per launch: no size to precompute, append grows it.
	b := []byte{0, 0, 0, 0, Version}
	b = append(b, r.ID[:]...)
	b = appendString(b, r.Cwd)
	b = appendString(b, r.Activation)
	b = binary.BigEndian.AppendUint16(b, r.Dropped)
	b = append(b, byte(len(r.Args)))
	for _, a := range r.Args {
		b = appendString(b, a)
	}
	binary.BigEndian.PutUint32(b, uint32(len(b)-4)) // #nosec G115 -- at most MaxFrame, checked above
	return b, nil
}

func appendString(b []byte, s string) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(len(s))) // #nosec G115 -- at most 4096, checked by check
	return append(b, s...)
}

// decodeRequest decodes one whole frame, refusing any byte after it. Its error wraps errVersion
// for a frame of another version, errMalformed for anything else that is not a valid frame.
func decodeRequest(frame []byte) (Request, error) {
	rd := bytes.NewReader(frame)
	r, err := readRequest(rd)
	if err == nil && rd.Len() != 0 {
		return Request{}, fmt.Errorf("%w: %d bytes after the frame", errMalformed, rd.Len())
	}
	return r, err
}

// readRequest reads one frame from rd. The length is checked against MaxFrame before the body is
// allocated; the version byte is read first, so that any other version gets « unsupported »,
// whatever its length.
func readRequest(rd io.Reader) (Request, error) {
	var head [5]byte
	if _, err := io.ReadFull(rd, head[:4]); err != nil {
		return Request{}, fmt.Errorf("%w: length: %w", errMalformed, err)
	}
	n := binary.BigEndian.Uint32(head[:4])
	if n == 0 {
		return Request{}, fmt.Errorf("%w: empty frame", errMalformed)
	}
	if _, err := io.ReadFull(rd, head[4:]); err != nil {
		return Request{}, fmt.Errorf("%w: version: %w", errMalformed, err)
	}
	if head[4] != Version {
		return Request{}, fmt.Errorf("%w: %d", errVersion, head[4])
	}
	if n > MaxFrame {
		return Request{}, fmt.Errorf("%w: frame of %d bytes", errMalformed, n)
	}
	body := make([]byte, n-1)
	if _, err := io.ReadFull(rd, body); err != nil {
		return Request{}, fmt.Errorf("%w: body: %w", errMalformed, err)
	}
	return decodeBody(body)
}

// decoder reads the fields of a body; the first short read sets bad and every later read
// returns zero values.
type decoder struct {
	b   []byte
	bad bool
}

func (d *decoder) take(n int) []byte {
	if d.bad || n > len(d.b) {
		d.bad = true
		return nil
	}
	v := d.b[:n]
	d.b = d.b[n:]
	return v
}

func (d *decoder) u16() int {
	v := d.take(2)
	if v == nil {
		return 0
	}
	return int(binary.BigEndian.Uint16(v))
}

func (d *decoder) str() string { return string(d.take(d.u16())) }

// decodeBody decodes the body after the version byte, checking every field against its bound.
func decodeBody(body []byte) (Request, error) {
	d := decoder{b: body}
	var r Request
	copy(r.ID[:], d.take(len(r.ID)))
	r.Cwd = d.str()
	r.Activation = d.str()
	r.Dropped = uint16(d.u16()) // #nosec G115 -- read from two bytes
	count := d.take(1)
	if d.bad {
		return Request{}, fmt.Errorf("%w: truncated", errMalformed)
	}
	if int(count[0]) > MaxArgs {
		return Request{}, fmt.Errorf("%w: %d arguments", errMalformed, count[0])
	}
	r.Args = make([]string, 0, count[0])
	for range count[0] {
		r.Args = append(r.Args, d.str())
	}
	switch {
	case d.bad:
		return Request{}, fmt.Errorf("%w: truncated", errMalformed)
	case len(d.b) != 0:
		return Request{}, fmt.Errorf("%w: %d bytes after the arguments", errMalformed, len(d.b))
	}
	if err := r.check(); err != nil {
		return Request{}, fmt.Errorf("%w: %w", errMalformed, err)
	}
	return r, nil
}

// replyTo is the status the primary answers to a request that decoded with err.
func replyTo(err error) Status {
	switch {
	case err == nil:
		return StatusAccepted
	case errors.Is(err, errVersion):
		return StatusUnsupported
	}
	return StatusMalformed
}
