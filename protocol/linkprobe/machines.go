// SPDX-License-Identifier: MPL-2.0

package linkprobe

import (
	"errors"
	"math"
	"math/bits"
	"slices"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// Bounds and timers of spec §8.5.7, the constants of protocol's §16 table.
const (
	Interval       = transport.LinkProbeInterval       // the prober sends one PROBE per Interval
	MinSpacing     = transport.LinkProbeMinSpacing     // two PROBEs of a connection are never closer than this
	SeqSlack       = transport.LinkProbeSeqSlack       // the responder's margin on the PROBE rate
	SeqDropMax     = transport.LinkProbeSeqDropMax     // above the rate bound by at most this: dropped, not a fault
	SeqSkew        = transport.LinkProbeSeqSkew        // the rate bound counts (1 + 1/SeqSkew) × the elapsed time
	Window         = transport.LinkProbeWindow         // the span of the loss figure
	MinSpan        = transport.LinkProbeMinSpan        // the two samples of a figure are at least this far apart
	MaxSamples     = transport.LinkProbeMaxSamples     // accepted samples kept per connection
	MaxOutstanding = transport.LinkProbeMaxOutstanding // PROBEs waiting for their ECHO, per stream
	MinPackets     = transport.LinkProbeMinPackets     // fewer sent packets in the window: figure unknown
	Backoff        = transport.LinkProbeBackoff        // no new stream for this long after one ended
	ReopenGap      = transport.LinkProbeReopenGap      // the responder refuses a new stream this soon after one ended
	Credit         = transport.LinkProbeCredit         // the responder's bucket, per connection
	CreditPerEcho  = transport.LinkProbeCreditPerEcho  // 3 echoes at once, then 2 per second
	Unknown        = 0xFFFF                            // lossPermille of an unknown figure (Annex A.1)
)

var (
	// ErrNoStream is returned while no stream is open: Open one first.
	ErrNoStream = errors.New("linkprobe: no stream open")
	// ErrTooSoon is returned when a PROBE of the connection was sent less than MinSpacing ago.
	ErrTooSoon = errors.New("linkprobe: probe too soon")
	// ErrBlocked is returned when the stream's writer still holds a message: the PROBE is skipped.
	ErrBlocked = errors.New("linkprobe: writer busy, probe skipped")
	// ErrExhausted is returned once seq 2^32 - 1 was used on the connection: the prober is done with it.
	ErrExhausted = errors.New("linkprobe: sequence exhausted")
	// ErrBackoff is returned when a stream ended less than Backoff ago.
	ErrBackoff = errors.New("linkprobe: backoff")
	// ErrOpen is returned when a stream is already open.
	ErrOpen = errors.New("linkprobe: stream already open")
)

// PTO is the late bound of spec §8.5.5 step 3.6: smoothed_rtt + max(4 rttvar, 1 ms) +
// max_ack_delay (RFC 9002 §6.2.1, no backoff), rounded up to a whole millisecond. maxAckDelay < 0
// means the QUIC stack does not expose the peer's: 25 ms is used.
func PTO(srtt, rttvar, maxAckDelay time.Duration) time.Duration {
	if maxAckDelay < 0 {
		maxAckDelay = transport.LinkProbeDefaultMaxAckDelay
	}
	p := srtt + max(4*rttvar, time.Millisecond) + maxAckDelay
	if r := p % time.Millisecond; r != 0 {
		p += time.Millisecond - r
	}
	return p
}

// Outcome is what a machine did with a received message.
type Outcome uint8

// The outcomes of Prober.Receive and Responder.Receive.
const (
	Accepted  Outcome = iota + 1 // prober: the ECHO became a sample
	Late                         // prober: the ECHO came more than one PTO after its PROBE: dropped
	Stale                        // prober: the PROBE was no longer outstanding: dropped
	Ignored                      // an ignorable message, skipped
	Violation                    // reset the stream (§8.5.8)
	Echoed                       // responder: write the returned ECHO
	Dropped                      // responder: no credit or the writer is busy: no answer, never a fault
	Discarded                    // the stream has ended: the message has no effect
)

// String returns the outcome's name, as the corpus writes it.
func (o Outcome) String() string {
	switch o {
	case Accepted:
		return "accepted"
	case Late:
		return "late"
	case Stale:
		return "stale"
	case Ignored:
		return "ignored"
	case Violation:
		return "violation"
	case Echoed:
		return "echoed"
	case Dropped:
		return "dropped"
	case Discarded:
		return "discarded"
	default:
		return "unknown"
	}
}

type outstanding struct {
	seq  uint32
	at   time.Duration
	sent uint64
}

type sample struct {
	at             time.Duration // when its PROBE was sent
	rtt            time.Duration // from the PROBE to its ECHO
	sent, received uint64
}

// Prober is the state of one connection seen by the endpoint that measures it (§8.5.5). A
// new connection is a new Prober: nothing passes from one connection to the next.
type Prober struct {
	open         bool
	exhausted    bool   // seq 2^32 - 1 used on this connection: never again
	nextSeq      uint64 // next seq to send on the open stream
	highest      uint32 // highest seq sent on the open stream
	lastEchoed   uint32 // highest seq echoed on the open stream
	echoes       uint64 // ECHOes read on the open stream
	ignorables   uint64 // ignorable messages read on the open stream
	lastReceived uint64 // largest received of any ECHO on this connection
	lastProbe    time.Duration
	probed       bool          // lastProbe is set (per connection, across streams)
	resumeAt     time.Duration // no stream before this instant
	pending      []outstanding
	samples      []sample
	lossLevel    Level // the levels of §8.5.5 step 6, each with its hysteresis
	rttLevel     Level
	knownAt      time.Duration // when the levels last stepped
}

// NewProber returns the state of a new connection: no stream, no sample.
func NewProber() *Prober { return &Prober{} }

// Open records a new stream whose negotiation succeeded (seq starts at 1). It refuses while
// one is open, during the backoff that follows the end of the previous one, and for good once
// the connection's sequence is exhausted.
func (p *Prober) Open(now time.Duration) error {
	switch {
	case p.exhausted:
		return ErrExhausted
	case p.open:
		return ErrOpen
	case now < p.resumeAt:
		return ErrBackoff
	}
	p.open, p.nextSeq, p.highest, p.lastEchoed, p.echoes, p.ignorables, p.pending = true, 1, 0, 0, 0, 0, nil
	return nil
}

// CanOpen reports whether Open would succeed at now.
func (p *Prober) CanOpen(now time.Duration) bool { return !p.exhausted && !p.open && now >= p.resumeAt }

// Ended records the end of the stream (reset or closed by either side, or failed): the PROBEs
// in flight are forgotten, the samples stay, and no stream opens for Backoff.
func (p *Prober) Ended(now time.Duration) {
	p.open, p.pending, p.resumeAt = false, nil, now+Backoff
}

// violation ends the stream and forgets every sample of the connection (§8.5.8).
func (p *Prober) violation(now time.Duration) Outcome {
	p.Ended(now)
	p.samples, p.lastReceived = nil, 0
	return Violation
}

// Probe returns the PROBE to write now; sent is the connection's sent-packet counter at the
// moment the PROBE is handed to the transport. writable is false while the stream's writer still
// holds a previous message: the PROBE is then skipped and nothing changes.
func (p *Prober) Probe(now time.Duration, sent uint64, writable bool) (Message, error) {
	switch {
	case !p.open:
		return Message{}, ErrNoStream
	case p.nextSeq > math.MaxUint32:
		p.exhausted = true
		return Message{}, ErrExhausted
	case p.probed && now-p.lastProbe < MinSpacing:
		return Message{}, ErrTooSoon
	case !writable:
		return Message{}, ErrBlocked
	}
	seq := uint32(p.nextSeq) // #nosec G115 -- bounded just above.
	if len(p.pending) == MaxOutstanding {
		p.pending = p.pending[1:]
	}
	p.pending = append(p.pending, outstanding{seq: seq, at: now, sent: sent})
	p.nextSeq++
	p.highest, p.lastProbe, p.probed = seq, now, true
	return Message{Type: TypeProbe, Seq: seq}, nil
}

// Receive applies a message read from the stream at now. sentNow is the connection's
// sent-packet counter at now; pto is the connection's PTO at now (see [PTO]).
func (p *Prober) Receive(now time.Duration, m Message, sentNow uint64, pto time.Duration) Outcome {
	switch {
	case !p.open:
		return Discarded
	case m.Ignorable():
		p.ignorables++
		if p.ignorables > p.echoes+1 { // one per ECHO, plus one (§8.5.4)
			return p.violation(now)
		}
		return Ignored
	case m.Type != TypeEcho || m.Seq > p.highest || m.Seq <= p.lastEchoed:
		return p.violation(now)
	}
	p.lastEchoed = m.Seq
	p.echoes++
	if m.Received < p.lastReceived || m.Received > sentNow {
		return p.violation(now)
	}
	p.lastReceived = m.Received
	i := 0
	for i < len(p.pending) && p.pending[i].seq < m.Seq {
		i++
	}
	if i == len(p.pending) || p.pending[i].seq != m.Seq {
		p.pending = p.pending[i:]
		return Stale
	}
	o := p.pending[i]
	p.pending = p.pending[i+1:]
	if now-o.at > pto {
		return Late
	}
	if len(p.samples) == MaxSamples {
		p.samples = p.samples[1:]
	}
	p.samples = append(p.samples, sample{at: o.at, rtt: now - o.at, sent: o.sent, received: m.Received})
	p.advance(now)
	return Accepted
}

// Samples returns the number of samples kept.
func (p *Prober) Samples() int { return len(p.samples) }

// window returns the samples of the figure at now (§8.5.5 step 5), oldest to newest; ok false
// means the figure is unknown.
func (p *Prober) window(now time.Duration) (ws []sample, ok bool) {
	n := len(p.samples)
	if n < 2 {
		return nil, false
	}
	s1 := p.samples[n-1]
	if now-s1.at > Window {
		return nil, false
	}
	// The oldest sample at most Window older than s1 (s1 itself at least); with s1 alone, the span
	// is 0, under MinSpan.
	i0 := slices.IndexFunc(p.samples, func(s sample) bool { return s1.at-s.at <= Window })
	s0 := p.samples[i0]
	if s1.at-s0.at < MinSpan || s1.sent < s0.sent || s1.sent-s0.sent < MinPackets {
		return nil, false
	}
	return p.samples[i0:], true
}

// Loss returns the loss figure at now in permille (0..1000), or Unknown (§8.5.5, step 5).
func (p *Prober) Loss(now time.Duration) uint16 {
	ws, ok := p.window(now)
	if !ok {
		return Unknown
	}
	s0, s1 := ws[0], ws[len(ws)-1]
	dS, dR := s1.sent-s0.sent, s1.received-s0.received
	lost := dS - min(dR, dS)
	hi, lo := bits.Mul64(lost, 1000)
	lo, carry := bits.Add64(lo, dS/2, 0)
	q, _ := bits.Div64(hi+carry, lo, dS) // the quotient is at most 1000, so hi+carry < dS
	return uint16(q)                     // #nosec G115 -- at most 1000.
}

// Responder is the answering side of one connection (§8.5.6). Its credit, its refusals and
// its gap between streams are per connection; the stream's own state is reset at each open.
type Responder struct {
	stream     bool // an inbound probe stream is open
	opened     time.Duration
	lastSeq    uint32
	probes     uint64 // PROBEs read on the open stream
	ignorables uint64 // ignorable messages read on the open stream
	credit     time.Duration
	at         time.Duration
	refuseTil  time.Duration // no new stream before this instant
}

// NewResponder returns the state of a connection established at now, with a full credit.
func NewResponder(now time.Duration) *Responder { return &Responder{credit: Credit, at: now} }

func (r *Responder) refill(now time.Duration) {
	r.credit = min(Credit, r.credit+min(max(now-r.at, 0), Credit)) // the elapsed time is capped first: no overflow
	r.at = max(r.at, now)
}

// StreamOpened records an inbound probe stream; false means reset it unread: one is already
// open, the previous one ended less than ReopenGap ago, or a violation of this connection
// happened less than Backoff ago.
func (r *Responder) StreamOpened(now time.Duration) bool {
	if r.stream || now < r.refuseTil {
		return false
	}
	r.stream, r.opened, r.lastSeq, r.probes, r.ignorables = true, now, 0, 0, 0
	return true
}

// StreamEnded records the end of the inbound stream (FIN read, or reset by either side).
func (r *Responder) StreamEnded(now time.Duration) {
	if r.stream {
		r.stream, r.refuseTil = false, max(r.refuseTil, now+ReopenGap)
	}
}

// Receive applies a message read at now on the inbound stream; received is the connection's
// received-packet counter, read when the PROBE was delivered; writable is false while the
// stream's writer still holds a previous ECHO. With Echoed, write the returned ECHO at once.
// Dropping a PROBE (no credit, writer busy) is never a fault and spends no credit.
func (r *Responder) Receive(now time.Duration, m Message, received uint64, writable bool) (Message, Outcome) {
	if !r.stream {
		return Message{}, Discarded
	}
	r.refill(now)
	if m.Ignorable() {
		r.ignorables++
		if r.ignorables > r.probes+1 { // one per PROBE, plus one (§8.5.4)
			r.Violation(now)
			return Message{}, Violation
		}
		return Message{}, Ignored
	}
	if m.Type != TypeProbe || m.Seq <= r.lastSeq {
		r.Violation(now)
		return Message{}, Violation
	}
	// A prober sends at most one PROBE per MinSpacing; it may have sent some before this side
	// accepted the stream (lazy negotiation, a slow accept), at most one per MinSpacing of the
	// negotiation timeout: those are dropped, a larger seq is a flood.
	// The elapsed time gets a margin of 1/SeqSkew against the two clocks' rates.
	d := max(now-r.opened, 0)
	bound := uint64((d+d/SeqSkew)/MinSpacing) + SeqSlack // #nosec G115 -- clamped at 0.
	if uint64(m.Seq) > bound+SeqDropMax {
		r.Violation(now)
		return Message{}, Violation
	}
	r.lastSeq = m.Seq
	r.probes++
	if uint64(m.Seq) > bound || !writable || r.credit < CreditPerEcho {
		return Message{}, Dropped
	}
	r.credit -= CreditPerEcho
	return Message{Type: TypeEcho, Seq: m.Seq, Received: received}, Echoed
}

// Violation records a violation raised by the responder (also a malformed message): the stream
// is reset and every new probe stream of the connection is refused for Backoff.
func (r *Responder) Violation(now time.Duration) {
	r.stream, r.refuseTil = false, max(r.refuseTil, now+Backoff)
}
