// Package invite brings a new person into a project. Everything it does
// revolves around one notion, the [Ticket], which travels in three ways:
//
//   - as a short Wormhole code (a PAKE-authenticated, one-way transfer; see
//     [Wormhole]);
//   - as invitation text, "inkwell1:" followed by base32, to copy and paste
//     where there is no Internet (see [Ticket.Text] and [ParseText]);
//   - not at all on the local network, where the host's explicit acceptance
//     takes its place (see [Host.Serve]).
//
// A ticket holds no authority by itself: the newcomer connects to the host,
// presents the ticket's secret on the lobby protocol and the host, an admin,
// signs the newcomer into the member list. The newcomer then checks the op
// chain it receives back to the project's genesis (see [Redeem]), so a host
// cannot hand it a project other than the one it was invited to.
//
// Nothing in this package logs, and every decoder of bytes that come from
// another machine is fuzzed.
package invite

import (
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/trust"
)

const (
	// TicketTTL is how long a ticket can be redeemed.
	TicketTTL = 15 * time.Minute
	// MaxTextLen bounds the invitation text, in bytes.
	MaxTextLen = 1024

	textPrefix    = "inkwell1:"
	ticketVersion = 1
	flagPerson    = 1 << 0
	maxAddrs      = 16
	maxAddrLen    = 255
	peerSegment   = "/p2p/"
	// maxRaw is the largest encoding whose base32 text fits in MaxTextLen.
	maxRaw = (MaxTextLen - len(textPrefix)) * 5 / 8
)

// ErrMalformed marks a ticket, invitation text or lobby message that is not
// valid.
var ErrMalformed = errors.New("invite: malformed")

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// Ticket is the right to join one project once, until Expires.
type Ticket struct {
	// Expires is the end of the ticket's validity, to the second.
	Expires time.Time
	// Person, if not nil, makes the newcomer another device of that person
	// instead of a new person.
	Person *trust.Hash
	// Host is the identity of the device to connect to.
	Host transport.PeerID
	// Addrs are addresses of the host, each ending with /p2p/<Host>.
	Addrs []string
	// Project is the project the ticket is for.
	Project trust.ProjectID
	// Secret proves the holder was given the ticket.
	Secret [16]byte
	// Role is what the newcomer will be allowed to do.
	Role trust.Role
}

// Expired reports whether the ticket is no longer valid at now.
func (t Ticket) Expired(now time.Time) bool { return !now.Before(t.Expires) }

// Text returns the invitation text of the ticket: "inkwell1:" and the base32
// of its encoding, at most [MaxTextLen] bytes. The addresses that do not fit
// (the last ones) or do not end with /p2p/<Host> are left out. Host must be a
// valid [transport.PeerID].
func (t Ticket) Text() string {
	raw := t.encode(nil)
	suffix := peerSegment + string(t.Host)
	var keep []string
	room := maxRaw - len(raw) - 1 // the count byte
	for _, a := range t.Addrs {
		short, ok := strings.CutSuffix(a, suffix)
		if !ok || short == "" || len(short) > maxAddrLen || len(keep) == maxAddrs || room < 1+len(short) {
			continue
		}
		room -= 1 + len(short)
		keep = append(keep, short)
	}
	raw = append(raw, byte(len(keep))) // #nosec G115 -- at most maxAddrs (16).
	for _, a := range keep {
		raw = append(raw, byte(len(a))) // #nosec G115 -- at most maxAddrLen (255), checked above.
		raw = append(raw, a...)
	}
	return textPrefix + b32.EncodeToString(raw)
}

// encode appends the encoding of every field but the addresses.
func (t Ticket) encode(b []byte) []byte {
	var flags byte
	if t.Person != nil {
		flags |= flagPerson
	}
	b = append(b, ticketVersion, flags, byte(t.Role))
	b = append(b, t.Project[:]...)
	if t.Person != nil {
		b = append(b, t.Person[:]...)
	}
	b = append(b, t.Secret[:]...)
	b = binary.BigEndian.AppendUint64(b, uint64(max(t.Expires.Unix(), 0))) // #nosec G115 -- clamped at 0.
	b = append(b, byte(len(t.Host)))                                       // #nosec G115 -- a valid PeerID (Text's contract) is about 52 bytes.
	return append(b, t.Host...)
}

// ParseText decodes an invitation text made by [Ticket.Text]. It tolerates
// what copying does to the text (case, spaces, line breaks) and refuses
// anything else: it does not check the expiry, see [Ticket.Expired].
func ParseText(s string) (Ticket, error) {
	if len(s) > 2*MaxTextLen {
		return Ticket{}, fmt.Errorf("%w: text too long", ErrMalformed)
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	if len(s) > MaxTextLen {
		return Ticket{}, fmt.Errorf("%w: text too long", ErrMalformed)
	}
	if len(s) < len(textPrefix) || !strings.EqualFold(s[:len(textPrefix)], textPrefix) {
		return Ticket{}, fmt.Errorf("%w: not an invitation", ErrMalformed)
	}
	raw, err := b32.DecodeString(strings.ToUpper(s[len(textPrefix):]))
	if err != nil {
		return Ticket{}, fmt.Errorf("%w: %v", ErrMalformed, err) //nolint:errorlint // the cause is not API.
	}
	return decodeTicket(raw)
}

// decodeTicket is the inverse of the encoding of [Ticket.Text].
func decodeTicket(raw []byte) (Ticket, error) {
	r := &reader{b: raw}
	var t Ticket
	if v := r.u8(); v != ticketVersion {
		return Ticket{}, fmt.Errorf("%w: version %d", ErrMalformed, v)
	}
	flags := r.u8()
	t.Role = trust.Role(r.u8())
	copy(t.Project[:], r.take(len(t.Project)))
	if flags&^flagPerson != 0 {
		return Ticket{}, fmt.Errorf("%w: unknown flags", ErrMalformed)
	}
	if flags&flagPerson != 0 {
		var p trust.Hash
		copy(p[:], r.take(len(p)))
		t.Person = &p
	}
	copy(t.Secret[:], r.take(len(t.Secret)))
	exp := r.u64()
	t.Host = transport.PeerID(r.take(int(r.u8())))
	n := int(r.u8())
	if r.err != nil {
		return Ticket{}, r.err
	}
	if exp > 1<<62 {
		return Ticket{}, fmt.Errorf("%w: expiry out of range", ErrMalformed)
	}
	t.Expires = time.Unix(int64(exp), 0).UTC() // #nosec G115 -- bounded above.
	if t.Role < trust.RoleReader || t.Role > trust.RoleAdmin {
		return Ticket{}, fmt.Errorf("%w: role", ErrMalformed)
	}
	if t.Secret == [16]byte{} {
		return Ticket{}, fmt.Errorf("%w: empty secret", ErrMalformed)
	}
	if !transport.ValidPeerID(t.Host) || strings.Contains(string(t.Host), "/") {
		return Ticket{}, fmt.Errorf("%w: host", ErrMalformed)
	}
	if n > maxAddrs {
		return Ticket{}, fmt.Errorf("%w: %d addresses", ErrMalformed, n)
	}
	for range n {
		a := string(r.take(int(r.u8())))
		if r.err != nil {
			return Ticket{}, r.err
		}
		if !validAddr(a) {
			return Ticket{}, fmt.Errorf("%w: address", ErrMalformed)
		}
		t.Addrs = append(t.Addrs, a+peerSegment+string(t.Host))
	}
	if r.err != nil || len(r.b) != 0 {
		return Ticket{}, fmt.Errorf("%w: length", ErrMalformed)
	}
	return t, nil
}

// validAddr reports whether a is printable ASCII starting with "/", the form
// the transport accepts to dial.
func validAddr(a string) bool {
	if a == "" || a[0] != '/' {
		return false
	}
	return !slices.ContainsFunc([]byte(a), func(c byte) bool { return c <= ' ' || c > '~' })
}
