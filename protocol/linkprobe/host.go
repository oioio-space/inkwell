// SPDX-License-Identifier: MPL-2.0

package linkprobe

import (
	"errors"
	"time"

	transport "github.com/oioio-space/inkwell/protocol"
)

// Transport is the kind of connection a Host runs on (§8.5.1).
type Transport uint8

// The transports. Only raw QUIC (version 1 or 2) carries the probe.
const (
	QUIC         Transport = iota + 1 // a QUIC v1 or v2 connection, direct or to a relay
	TCP                               // TCP + Noise + yamux
	Circuit                           // a relay circuit (not a QUIC connection end to end)
	WebTransport                      // a WebTransport session over HTTP/3: not raw QUIC
)

// Roles is what the peer is to this endpoint (§8.5.1), a set: a peer may hold several.
type Roles uint8

// The roles. The empty set is a guest of the lobby, or any other peer.
const (
	Member      Roles = 1 << iota // a member of a project both share: probed and answered
	Relay                         // a relay this endpoint uses: probed (first hop), not answered
	RelayClient                   // a peer with a reservation or an active circuit on this relay: answered, not probed
)

func (r Roles) probes() bool  { return r&(Member|Relay) != 0 }
func (r Roles) answers() bool { return r&(Member|RelayClient) != 0 }

// ActionKind is what a Host asks its I/O layer to do.
type ActionKind uint8

// The actions. "Out" is the stream this endpoint opened (it probes), "in" the stream the peer
// opened (it answers). There is no session action: nothing here can end a session.
const (
	OpenOut  ActionKind = iota + 1 // open a probe stream on this connection and negotiate it
	WriteOut                       // write Data on the outbound stream
	ResetOut                       // reset the outbound stream (application error code 0)
	FinOut                         // close the outbound stream for writing
	WriteIn                        // write Data on the inbound stream
	ResetIn                        // reset the inbound stream
	RefuseIn                       // reset the inbound stream just opened (Stream), unread
	FinIn                          // close the inbound stream for writing
)

// Action is one thing to do: Data for WriteOut and WriteIn, Stream for the inbound actions.
type Action struct {
	Kind   ActionKind
	Data   []byte
	Stream uint64
}

// reader keeps the incomplete head of a stream in a fixed buffer (§8.5.4: at most 65 bytes).
type reader struct {
	buf [transport.LinkProbeMaxBuffered]byte
	n   int
}

// next returns the next message of the bytes read so far (chunk being the bytes just read, of
// which used have been consumed); ok false means wait for more bytes.
func (r *reader) next(chunk []byte, used *int) (Message, bool, error) {
	for {
		if r.n > 0 {
			m, n, err := Decode(r.buf[:r.n])
			if err == nil {
				m.Body = append([]byte(nil), m.Body...) // the buffer is reused
				r.n = copy(r.buf[:], r.buf[n:r.n])
				return m, true, nil
			}
			if !errors.Is(err, ErrShort) {
				return Message{}, false, err
			}
			if *used == len(chunk) {
				return Message{}, false, nil
			}
			c := copy(r.buf[r.n:], chunk[*used:])
			r.n += c
			*used += c
			continue
		}
		m, n, err := Decode(chunk[*used:])
		switch {
		case err == nil:
			*used += n
			return m, true, nil
		case errors.Is(err, ErrShort):
			r.n = copy(r.buf[:], chunk[*used:]) // at most MaxLen bytes: an ErrShort head is shorter than a message
			*used = len(chunk)
			return Message{}, false, nil
		default:
			return Message{}, false, err
		}
	}
}

// Buffered returns the bytes kept of an incomplete message, out and in.
func (h *Host) Buffered() (int, int) { return h.out.n, h.in.n }

// Host is the probe on one connection, both roles, without I/O (§8.5.3 to §8.5.6): events in,
// actions out. Bytes read from a stream may arrive in any chunks.
type Host struct {
	quic         bool
	roles        Roles
	prober       *Prober
	responder    *Responder
	negotiating  bool // OpenOut asked, negotiation not finished
	negoSince    time.Duration
	closeAsked   bool // Close during the negotiation: FIN as soon as it succeeds
	unsupported  bool // the peer refused the protocol: never again on this connection
	closing      bool // FinOut written: read and discard until the peer ends its side
	closingSince time.Duration
	closingBytes int  // bytes discarded since the FIN
	stopped      bool // Close was called: the prober never opens a stream again on this connection
	out, in      reader
	inID         uint64
	inOpen       bool
}

// NewHost returns the probe state of a connection established at now.
func NewHost(t Transport, r Roles, now time.Duration) *Host {
	return &Host{quic: t == QUIC, roles: r, prober: NewProber(), responder: NewResponder(now)}
}

func (h *Host) mayProbe() bool  { return h.quic && h.roles.probes() }
func (h *Host) mayAnswer() bool { return h.quic && h.roles.answers() }

// SetRoles records that the peer's roles changed (§8.5.1): a stream the new roles exclude is reset.
func (h *Host) SetRoles(now time.Duration, r Roles) []Action {
	h.roles = r
	var out []Action
	if !h.mayProbe() && (h.prober.open || h.negotiating || h.closing) {
		h.endOut(now)
		out = append(out, Action{Kind: ResetOut})
	}
	if !h.mayAnswer() && h.inOpen {
		h.inOpen, h.in.n = false, 0
		h.responder.StreamEnded(now)
		out = append(out, Action{Kind: ResetIn, Stream: h.inID})
	}
	return out
}

// endOut forgets the outbound stream as ended at now.
func (h *Host) endOut(now time.Duration) {
	h.negotiating, h.closing, h.closeAsked, h.out.n = false, false, false, 0
	if h.prober.open {
		h.prober.Ended(now)
	} else if h.prober.resumeAt < now+Backoff {
		h.prober.resumeAt = now + Backoff
	}
}

// Tick is the prober's timer: once per Interval, with the endpoint's jitter. sent is the
// connection's sent-packet counter; writable says whether the outbound writer is free.
func (h *Host) Tick(now time.Duration, sent uint64, writable bool) []Action {
	switch {
	case h.negotiating && now-h.negoSince >= transport.LinkProbeNegotiationTimeout:
		h.endOut(now)
		return []Action{{Kind: ResetOut}}
	case h.closing && now-h.closingSince >= Backoff:
		h.closing, h.out.n = false, 0
		return []Action{{Kind: ResetOut}}
	case !h.mayProbe() || h.unsupported || h.stopped || h.negotiating || h.closing:
		return nil
	}
	if !h.prober.open {
		if !h.prober.CanOpen(now) {
			return nil
		}
		h.negotiating, h.negoSince = true, now
		return []Action{{Kind: OpenOut}}
	}
	return h.probeNow(now, sent, writable)
}

func (h *Host) startClosing(now time.Duration) []Action {
	h.prober.Ended(now)
	h.closing, h.closingSince, h.closingBytes, h.out.n = true, now, 0, 0
	return []Action{{Kind: FinOut}}
}

func (h *Host) probeNow(now time.Duration, sent uint64, writable bool) []Action {
	m, err := h.prober.Probe(now, sent, writable)
	switch {
	case err == nil:
		b, _ := Encode(m) // a PROBE of seq >= 1 always encodes
		return []Action{{Kind: WriteOut, Data: b}}
	case errors.Is(err, ErrExhausted):
		return h.startClosing(now)
	default: // too soon, writer busy: nothing this time
		return nil
	}
}

// Negotiated reports the end of the outbound negotiation. ok false means the peer does not
// speak the protocol (multistream-select na): the probe never runs again on this connection.
// On success the first PROBE goes out at once (or the FIN, if Close came during negotiation).
func (h *Host) Negotiated(now time.Duration, ok bool, sent uint64, writable bool) []Action {
	if !h.negotiating {
		return nil
	}
	h.negotiating = false
	if !ok {
		h.unsupported = true
		return nil
	}
	if err := h.prober.Open(now); err != nil {
		h.endOut(now)
		return []Action{{Kind: ResetOut}} // a negotiated stream the prober cannot use is not left open
	}
	if h.closeAsked {
		h.closeAsked = false
		return h.startClosing(now)
	}
	return h.probeNow(now, sent, writable)
}

// OutboundEnded reports that the outbound stream ended: FIN or reset from the peer (whatever
// its code), a transport error, or a failed negotiation other than na.
func (h *Host) OutboundEnded(now time.Duration) []Action {
	if h.closing {
		h.closing, h.out.n = false, 0
		return nil
	}
	if h.negotiating || h.prober.open {
		h.endOut(now)
	}
	return nil
}

// Close stops measuring on this connection for good: FIN on the outbound stream, what is read
// afterwards is discarded, and no stream opens again. During the negotiation the FIN goes out
// when it succeeds. Answering the peer's probes is not affected.
func (h *Host) Close(now time.Duration) []Action {
	h.stopped = true
	switch {
	case h.negotiating:
		h.closeAsked = true
		return nil
	case h.prober.open:
		return h.startClosing(now)
	default:
		return nil
	}
}

// OutboundData is bytes read from the outbound stream at now.
func (h *Host) OutboundData(now time.Duration, b []byte, sentNow uint64, pto time.Duration) []Action {
	if h.closing {
		// Read and discarded, up to ClosingMaxBytes: an honest responder sends at most the ECHOes
		// of the outstanding PROBEs and their ignorables; more is a junk pipe.
		h.closingBytes += len(b)
		if h.closingBytes > transport.LinkProbeClosingMaxBytes {
			h.closing = false
			return []Action{{Kind: ResetOut}}
		}
		return nil
	}
	if !h.prober.open {
		return nil // ended or never opened: discarded
	}
	used := 0
	for {
		m, ok, err := h.out.next(b, &used)
		if err != nil {
			h.prober.violation(now)
			h.out.n = 0
			return []Action{{Kind: ResetOut}}
		}
		if !ok {
			return nil
		}
		if h.prober.Receive(now, m, sentNow, pto) == Violation {
			h.out.n = 0
			return []Action{{Kind: ResetOut}}
		}
	}
}

// InboundOpened reports a probe stream opened by the peer, with its identity. A refused stream
// is reset at once: its later events are ignored.
func (h *Host) InboundOpened(now time.Duration, id uint64) []Action {
	if !h.mayAnswer() || !h.responder.StreamOpened(now) {
		return []Action{{Kind: RefuseIn, Stream: id}}
	}
	h.inOpen, h.inID, h.in.n = true, id, 0
	return nil
}

// InboundData is bytes read from inbound stream id at now; received is the connection's
// received-packet counter when they were delivered; writable says whether its writer is free.
// At most one ECHO goes out per read: the writer holds one message.
func (h *Host) InboundData(now time.Duration, id uint64, b []byte, received uint64, writable bool) []Action {
	if !h.inOpen || id != h.inID {
		return nil
	}
	var out []Action
	used := 0
	for {
		m, ok, err := h.in.next(b, &used)
		if err != nil {
			h.responder.Violation(now)
			h.inOpen, h.in.n = false, 0
			return append(out, Action{Kind: ResetIn, Stream: id})
		}
		if !ok {
			return out
		}
		echo, o := h.responder.Receive(now, m, received, writable)
		switch o {
		case Violation:
			h.inOpen, h.in.n = false, 0
			return append(out, Action{Kind: ResetIn, Stream: id})
		case Echoed:
			e, _ := Encode(echo) // an ECHO of seq >= 1 always encodes
			out = append(out, Action{Kind: WriteIn, Data: e, Stream: id})
			writable = false // one message at a time in the writer
		}
	}
}

// InboundEnded reports that inbound stream id ended: fin true for a FIN (answered by a FIN),
// false for a reset (whatever its code) or a transport error.
func (h *Host) InboundEnded(now time.Duration, id uint64, fin bool) []Action {
	if !h.inOpen || id != h.inID {
		return nil
	}
	h.inOpen, h.in.n = false, 0
	h.responder.StreamEnded(now)
	if fin {
		return []Action{{Kind: FinIn, Stream: id}}
	}
	return nil
}

// Loss returns the figure of the connection at now (Annex A.1 lossPermille, a diagnostic).
func (h *Host) Loss(now time.Duration) uint16 { return h.prober.Loss(now) }

// Level returns the level of the connection at now (Annex A.1 quality); call it once per sampler
// step (see [Prober.Level]).
func (h *Host) Level(now time.Duration) Level { return h.prober.Level(now) }
