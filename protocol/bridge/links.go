// SPDX-License-Identifier: MPL-2.0

package bridge

import (
	"encoding/binary"
	"fmt"

	transport "github.com/oioio-space/inkwell/protocol"
)

// MaxLinks bounds the entries of a Links payload.
const MaxLinks = 1024

// LossUnknown is the LossPermille of a link with fewer than 50 packets sent.
const LossUnknown = 0xFFFF

// The values of [Link.Path], [Link.Quality] and [Link.LossScope].
const (
	PathUnknown = 0
	PathLAN     = 1
	PathDirect  = 2
	PathRelayed = 3

	QualityUnknown = 0
	QualityGood    = 1
	QualityFair    = 2
	QualityPoor    = 3

	LossScopeNone     = 0
	LossScopePath     = 1
	LossScopeFirstHop = 2
)

// Link is one peer of a Links payload (spec Annex A.1). Instants are ages in
// milliseconds, and no network address is part of it. It is comparable.
type Link struct {
	Peer                                       transport.PeerID
	PathAgeMs                                  uint32
	Upgrades, Fallbacks                        uint16
	RTTMs, RTTVarMs, RTTAgeMs                  uint32 // RTTMs 0: not measured
	LossPermille                               uint16 // [LossUnknown]: not known
	UpMs                                       uint32 // 0: disconnected
	Downs10m                                   uint16
	DownsTotal, LastDownMs                     uint32
	Resyncs10m                                 uint16
	Resyncs                                    uint32
	PendingMsgs, PendingBytes, OldestPendingMs uint32
	SendFailures, Resets                       uint32
	Path, Quality, LossScope                   uint8
}

// Stable returns l with its ages reduced to whether they are set: the form in
// which two states are compared to tell whether the content changed (spec
// Annex A.1, coalescence of Links). PathAgeMs, which is always set, drops out.
func (l Link) Stable() Link {
	l.PathAgeMs = 0
	l.RTTAgeMs = min(l.RTTAgeMs, 1)
	l.UpMs = min(l.UpMs, 1)
	l.OldestPendingMs = min(l.OldestPendingMs, 1)
	return l
}

// figure points at one field of a Link after its peer: the pointer of its
// width is set.
type figure struct {
	u8  *uint8
	u16 *uint16
	u32 *uint32
}

// figures lists the fields of l after its peer, in wire order. It is the one
// description of the layout: encoding, decoding and [linkFixed] follow it.
func (l *Link) figures() [21]figure {
	return [...]figure{
		{u8: &l.Path},
		{u8: &l.Quality},
		{u32: &l.PathAgeMs},
		{u16: &l.Upgrades},
		{u16: &l.Fallbacks},
		{u32: &l.RTTMs},
		{u32: &l.RTTVarMs},
		{u32: &l.RTTAgeMs},
		{u16: &l.LossPermille},
		{u8: &l.LossScope},
		{u32: &l.UpMs},
		{u16: &l.Downs10m},
		{u32: &l.DownsTotal},
		{u32: &l.LastDownMs},
		{u16: &l.Resyncs10m},
		{u32: &l.Resyncs},
		{u32: &l.PendingMsgs},
		{u32: &l.PendingBytes},
		{u32: &l.OldestPendingMs},
		{u32: &l.SendFailures},
		{u32: &l.Resets},
	}
}

// LinkFixed is the size of a Link entry without its peer, derived from the
// field list.
var LinkFixed = func() (n int) {
	var l Link
	for _, f := range l.figures() {
		switch {
		case f.u8 != nil:
			n++
		case f.u16 != nil:
			n += 2
		default:
			n += 4
		}
	}
	return n
}()

// checkEntry checks l, which follows prev (nil for the first): a valid peer
// sorted after prev's, and values in their domain.
func checkEntry(prev *Link, l *Link) error {
	if !transport.ValidPeerID(l.Peer) || l.Path > PathRelayed || l.Quality > QualityPoor ||
		l.LossScope > LossScopeFirstHop || (prev != nil && prev.Peer >= l.Peer) {
		return fmt.Errorf("%w: link entry", ErrMalformed)
	}
	return nil
}

// EncodeLinks serializes ls, the payload of a Links envelope: count u16, then
// per entry len(peer) u8 | peer | the figures, big endian. ls must be sorted
// by peer, without duplicate. It returns an error wrapping [ErrMalformed]
// otherwise, or for a value out of its domain.
func EncodeLinks(ls []Link) ([]byte, error) {
	if len(ls) > MaxLinks {
		return nil, fmt.Errorf("%w: %d links", ErrMalformed, len(ls))
	}
	b := binary.BigEndian.AppendUint16(nil, uint16(len(ls))) // #nosec G115 -- bounded by MaxLinks.
	for i := range ls {
		l := ls[i]
		var prev *Link
		if i > 0 {
			prev = &ls[i-1]
		}
		if err := checkEntry(prev, &l); err != nil {
			return nil, err
		}
		b = appendPeer(b, l.Peer)
		for _, f := range l.figures() {
			switch {
			case f.u8 != nil:
				b = append(b, *f.u8)
			case f.u16 != nil:
				b = binary.BigEndian.AppendUint16(b, *f.u16)
			default:
				b = binary.BigEndian.AppendUint32(b, *f.u32)
			}
		}
	}
	return b, nil
}

// DecodeLinks parses an encoding of [EncodeLinks], strictly. It returns an
// error wrapping [ErrMalformed] for a truncated or oversized list, an invalid
// peer, an unsorted or duplicate peer, a value out of its domain, or trailing
// bytes.
func DecodeLinks(b []byte) ([]Link, error) {
	r := reader{b: b}
	n := r.count(MaxLinks)
	var out []Link
	for range n {
		l := Link{Peer: r.peer()}
		for _, f := range l.figures() {
			switch {
			case f.u8 != nil:
				*f.u8 = r.u8()
			case f.u16 != nil:
				*f.u16 = r.u16()
			default:
				*f.u32 = r.u32()
			}
		}
		if r.err != nil {
			return nil, r.err
		}
		var prev *Link
		if len(out) > 0 {
			prev = &out[len(out)-1]
		}
		if err := checkEntry(prev, &l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := r.done(); err != nil {
		return nil, err
	}
	return out, nil
}
