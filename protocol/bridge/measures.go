// SPDX-License-Identifier: MPL-2.0

package bridge

import (
	"math"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// Measures are the figures of one link as an implementation holds them, in its
// own units; [LinkOf] turns them into a wire [Link] by the rules of spec Annex
// A.1 (Links). Path, Quality and LossScope are already the wire values.
type Measures struct {
	Peer                     transport.PeerID
	Path, Quality, LossScope uint8
	// PathSince and UpSince are instants; the zero time means none.
	PathSince, UpSince time.Time
	// RTT is zero while none was measured.
	RTT, RTTVar, RTTAge time.Duration
	// LossPct is nil while the loss is not known.
	LossPct                                        *float64
	LastDown, OldestPending                        time.Duration
	Upgrades, Fallbacks, Downs10m, DownsTotal      int
	Resyncs10m, Resyncs, PendingMsgs, PendingBytes int
	SendFailures, Resets                           uint64
}

// sat32 converts v to a u32, saturating at the maximum of the type (spec
// Annex A.1, Links: never truncated); a negative value is zero.
func sat32[T ~int | ~int64 | ~uint64](v T) uint32 {
	switch {
	case v <= 0:
		return 0
	case uint64(v) > math.MaxUint32:
		return math.MaxUint32
	}
	return uint32(v) // #nosec G115 -- bounded just above.
}

// sat16 is sat32 for a u16.
func sat16(v int) uint16 {
	switch {
	case v <= 0:
		return 0
	case v > math.MaxUint16:
		return math.MaxUint16
	}
	return uint16(v) // #nosec G115 -- bounded just above.
}

// ms converts d to whole milliseconds in a u32, rounding a positive d up so a
// real measure never reads as « none » (zero).
func ms(d time.Duration) uint32 {
	n := int64(d / time.Millisecond)
	if d%time.Millisecond > 0 {
		n++ // d + 1ms would overflow near the maximum
	}
	return sat32(n) // a negative d gives zero
}

// age converts an instant to the milliseconds elapsed at now; a zero instant
// is zero.
func age(t, now time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return ms(now.Sub(t))
}

// LinkOf converts m to a wire [Link], instants as ages at now: every value
// saturates at the maximum of its type, a positive duration is at least 1 ms,
// loss is in permille (saturating one below [LossUnknown], which is nil), and
// the loss scope is none while the loss is unknown.
func LinkOf(m Measures, now time.Time) Link {
	l := Link{
		Peer: m.Peer, Path: m.Path, Quality: m.Quality,
		PathAgeMs: age(m.PathSince, now),
		Upgrades:  sat16(m.Upgrades), Fallbacks: sat16(m.Fallbacks),
		RTTMs: ms(m.RTT), RTTVarMs: ms(m.RTTVar), RTTAgeMs: ms(m.RTTAge),
		LossPermille: LossUnknown,
		UpMs:         age(m.UpSince, now),
		Downs10m:     sat16(m.Downs10m), DownsTotal: sat32(m.DownsTotal), LastDownMs: ms(m.LastDown),
		Resyncs10m: sat16(m.Resyncs10m), Resyncs: sat32(m.Resyncs),
		PendingMsgs: sat32(m.PendingMsgs), PendingBytes: sat32(m.PendingBytes), OldestPendingMs: ms(m.OldestPending),
		SendFailures: sat32(m.SendFailures), Resets: sat32(m.Resets),
	}
	if m.LossPct != nil && !math.IsNaN(*m.LossPct) {
		l.LossPermille = min(sat16(int(math.Round(max(*m.LossPct, 0)*10))), LossUnknown-1)
		l.LossScope = m.LossScope
	}
	return l
}
