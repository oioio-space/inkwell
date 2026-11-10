// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/oioio-space/inkwell/protocol/linkprobe"
)

func init() {
	decoderTable["linkprobe_message"] = decoderSpec{hasInput: true, check: (*referenceCase).linkprobeMessage}
	decoderTable["linkprobe_prober"] = decoderSpec{check: (*referenceCase).linkprobeProber}
	decoderTable["linkprobe_responder"] = decoderSpec{check: (*referenceCase).linkprobeResponder}
	decoderTable["linkprobe_host"] = decoderSpec{check: (*referenceCase).linkprobeHost}
	decoderTable["linkprobe_pto"] = decoderSpec{check: (*referenceCase).linkprobePTO}
	decoderTable["linkprobe_level"] = decoderSpec{check: (*referenceCase).linkprobeLevel}
}

// linkprobeLevel checks one step of the level of spec §8.5.5 step 6: from a level, for a loss
// figure in permille or a round trip in milliseconds, the level after.
func (c *referenceCase) linkprobeLevel() error {
	var in struct {
		Kind  string `json:"kind"`
		From  string `json:"from"`
		Value int64  `json:"value"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	from := linkprobe.LevelUnknown
	for l := linkprobe.LevelUnknown; l <= linkprobe.LevelPoor; l++ {
		if l.String() == in.From {
			from = l
		}
	}
	var got linkprobe.Level
	switch in.Kind {
	case "loss":
		got = linkprobe.StepLoss(from, in.Value)
	case "rtt":
		got = linkprobe.StepRTT(from, time.Duration(in.Value)*time.Millisecond)
	default:
		return fmt.Errorf("context: unknown kind %q", in.Kind)
	}
	c.verdictChecked = true
	return c.matchExpectExact(map[string]any{"level": "\"" + got.String() + "\""})
}

// linkprobePTO checks the late bound of spec §8.5.5 step 3.6 from the RTT estimates (in µs;
// max_ack_delay_us absent or null: the QUIC stack does not expose it).
func (c *referenceCase) linkprobePTO() error {
	var in struct {
		SRTT   int64  `json:"srtt_us"`
		RTTVar int64  `json:"rttvar_us"`
		MAD    *int64 `json:"max_ack_delay_us"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	mad := time.Duration(-1)
	if in.MAD != nil {
		mad = time.Duration(*in.MAD) * time.Microsecond
	}
	got := linkprobe.PTO(time.Duration(in.SRTT)*time.Microsecond, time.Duration(in.RTTVar)*time.Microsecond, mad)
	c.verdictChecked = true
	return c.matchExpectExact(map[string]any{"pto_ms": int64(got / time.Millisecond)})
}

// linkprobeMessage checks one message of spec §8.5.4: the verdict (accept, reject, ignore for
// an ignorable type, incomplete when more bytes are needed), the fields, the unique encoding.
func (c *referenceCase) linkprobeMessage() error {
	m, n, err := linkprobe.Decode(c.input)
	if err == nil && n != len(c.input) {
		err = fmt.Errorf("%w: %d trailing bytes", linkprobe.ErrMalformed, len(c.input)-n)
	}
	got := "accept"
	switch {
	case errors.Is(err, linkprobe.ErrShort):
		got = "incomplete"
	case err != nil:
		got = "reject"
	case m.Ignorable():
		got = "ignore"
	}
	if got != c.vector.Verdict {
		return fmt.Errorf("verdict %s, want %s: %w", got, c.vector.Verdict, err)
	}
	c.verdictChecked = true
	if err != nil {
		return nil
	}
	enc, err := linkprobe.Encode(m)
	if err != nil {
		return fmt.Errorf("re-encode: %w", err)
	}
	if hex.EncodeToString(enc) != hex.EncodeToString(c.input) {
		return errors.New("binary round trip mismatch")
	}
	actual := map[string]any{"type": m.Type}
	switch m.Type {
	case linkprobe.TypeProbe:
		actual["seq"] = m.Seq
	case linkprobe.TypeEcho:
		actual["seq"], actual["received"] = m.Seq, m.Received
	}
	return c.matchExpectExact(actual)
}

// matchExpectExact is matchExpect with integers compared exactly (a u64 does not survive a
// float64), and every expected key required.
func (c *referenceCase) matchExpectExact(actual map[string]any) error {
	for key, raw := range c.expect {
		v, ok := actual[key]
		if !ok {
			return fmt.Errorf("expectation %s: no such field", key)
		}
		if fmt.Sprint(v) != string(raw) {
			return fmt.Errorf("expectation %s: got %v, want %s", key, v, raw)
		}
	}
	return nil
}

// lpEvent is one step of a linkprobe script (spec §21).
type lpEvent struct {
	Op       string          `json:"op"`
	AtMs     int64           `json:"at_ms"`
	Sent     uint64          `json:"sent"`
	SentNow  uint64          `json:"sent_now"`
	Received uint64          `json:"received"`
	PTOMs    int64           `json:"pto_ms"`
	Message  string          `json:"message_hex"`
	Data     string          `json:"data_hex"`
	Writable *bool           `json:"writable"`
	OK       bool            `json:"ok"`
	Fin      bool            `json:"fin"`
	Stream   uint64          `json:"stream"`
	Roles    []string        `json:"roles"`
	Action   string          `json:"action"`
	Expect   json.RawMessage `json:"expect"`
}

// writable is the event's writable flag, true when absent.
func (e lpEvent) writable() bool { return e.Writable == nil || *e.Writable }

func (c *referenceCase) events() ([]lpEvent, error) {
	var es []lpEvent
	if err := c.vector.Decode("events", &es); err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	if len(es) == 0 {
		return nil, errors.New("no events")
	}
	for i := 1; i < len(es); i++ {
		if es[i].AtMs < es[i-1].AtMs {
			return nil, fmt.Errorf("event %d goes back in time", i)
		}
	}
	return es, nil
}

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func expectString(raw json.RawMessage) (string, error) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}

// decodeMessage reads the message of an event; a script only carries well-formed messages.
func decodeMessage(h string) (linkprobe.Message, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return linkprobe.Message{}, err
	}
	m, n, err := linkprobe.Decode(b)
	if err == nil && n != len(b) {
		err = errors.New("trailing bytes")
	}
	return m, err
}

func checkString(i int, e lpEvent, got string) error {
	want, err := expectString(e.Expect)
	if err != nil {
		return fmt.Errorf("event %d: expect: %w", i, err)
	}
	if got != want {
		return fmt.Errorf("event %d (%s at %d ms): %s, want %s", i, e.Op, e.AtMs, got, want)
	}
	return nil
}

func checkLoss(i int, e lpEvent, got uint16) error {
	var want uint16
	if err := json.Unmarshal(e.Expect, &want); err != nil {
		return fmt.Errorf("event %d: expect: %w", i, err)
	}
	if got != want {
		return fmt.Errorf("event %d (loss at %d ms): %d, want %d", i, e.AtMs, got, want)
	}
	return nil
}

// linkprobeProber replays a prober script (spec §8.5.5) on the reference state machine.
func (c *referenceCase) linkprobeProber() error {
	es, err := c.events()
	if err != nil {
		return err
	}
	p := linkprobe.NewProber()
	for i, e := range es {
		now := ms(e.AtMs)
		var got string
		switch e.Op {
		case "open":
			switch err := p.Open(now); {
			case err == nil:
				got = "ok"
			case errors.Is(err, linkprobe.ErrBackoff):
				got = "backoff"
			case errors.Is(err, linkprobe.ErrOpen):
				got = "already_open"
			case errors.Is(err, linkprobe.ErrExhausted):
				got = "exhausted"
			default:
				return fmt.Errorf("event %d: open: %w", i, err)
			}
		case "probe":
			m, err := p.Probe(now, e.Sent, e.writable())
			switch {
			case err == nil:
				b, err := linkprobe.Encode(m)
				if err != nil {
					return fmt.Errorf("event %d: %w", i, err)
				}
				got = hex.EncodeToString(b)
			case errors.Is(err, linkprobe.ErrNoStream):
				got = "no_stream"
			case errors.Is(err, linkprobe.ErrTooSoon):
				got = "too_soon"
			case errors.Is(err, linkprobe.ErrBlocked):
				got = "skipped"
			case errors.Is(err, linkprobe.ErrExhausted):
				got = "exhausted"
			default:
				return fmt.Errorf("event %d: probe: %w", i, err)
			}
		case "receive":
			m, err := decodeMessage(e.Message)
			if err != nil {
				return fmt.Errorf("event %d: %w", i, err)
			}
			got = p.Receive(now, m, e.SentNow, ms(e.PTOMs)).String()
		case "ended":
			p.Ended(now)
			continue
		case "restart":
			p = linkprobe.NewProber()
			continue
		case "loss":
			if err := checkLoss(i, e, p.Loss(now)); err != nil {
				return err
			}
			continue
		case "level":
			got = p.Level(now).String()
		default:
			return fmt.Errorf("event %d: unknown op %q", i, e.Op)
		}
		if err := checkString(i, e, got); err != nil {
			return err
		}
	}
	c.verdictChecked = true
	return nil
}

// linkprobeResponder replays a responder script (spec §8.5.6); the connection is established
// at the first event.
func (c *referenceCase) linkprobeResponder() error {
	es, err := c.events()
	if err != nil {
		return err
	}
	r := linkprobe.NewResponder(ms(es[0].AtMs))
	for i, e := range es {
		now := ms(e.AtMs)
		var got string
		switch e.Op {
		case "stream":
			switch e.Action {
			case "open":
				got = "refused"
				if r.StreamOpened(now) {
					got = "ok"
				}
			case "end":
				r.StreamEnded(now)
				got = "ok"
			default:
				return fmt.Errorf("event %d: unknown stream action %q", i, e.Action)
			}
		case "receive":
			m, err := decodeMessage(e.Message)
			if err != nil {
				return fmt.Errorf("event %d: %w", i, err)
			}
			echo, out := r.Receive(now, m, e.Received, e.writable())
			got = out.String()
			if out == linkprobe.Echoed {
				b, err := linkprobe.Encode(echo)
				if err != nil {
					return fmt.Errorf("event %d: %w", i, err)
				}
				got = hex.EncodeToString(b)
			}
		default:
			return fmt.Errorf("event %d: unknown op %q", i, e.Op)
		}
		if err := checkString(i, e, got); err != nil {
			return err
		}
	}
	c.verdictChecked = true
	return nil
}

var (
	lpTransports = map[string]linkprobe.Transport{
		"quic": linkprobe.QUIC, "tcp": linkprobe.TCP, "circuit": linkprobe.Circuit, "webtransport": linkprobe.WebTransport,
	}
	lpRoles   = map[string]linkprobe.Roles{"member": linkprobe.Member, "relay": linkprobe.Relay, "relay_client": linkprobe.RelayClient}
	lpActions = map[linkprobe.ActionKind]string{
		linkprobe.OpenOut: "open_out", linkprobe.WriteOut: "write_out", linkprobe.ResetOut: "reset_out",
		linkprobe.FinOut: "fin_out", linkprobe.WriteIn: "write_in", linkprobe.ResetIn: "reset_in",
		linkprobe.RefuseIn: "refuse_in", linkprobe.FinIn: "fin_in",
	}
)

func roles(names []string) (linkprobe.Roles, error) {
	var r linkprobe.Roles
	for _, n := range names {
		v, ok := lpRoles[n]
		if !ok {
			return 0, fmt.Errorf("unknown role %q", n)
		}
		r |= v
	}
	return r, nil
}

func actionStrings(as []linkprobe.Action) []string {
	out := []string{}
	for _, a := range as {
		s := lpActions[a.Kind]
		switch a.Kind {
		case linkprobe.WriteOut:
			s += ":" + hex.EncodeToString(a.Data)
		case linkprobe.WriteIn:
			s += fmt.Sprintf(":%d:%s", a.Stream, hex.EncodeToString(a.Data))
		case linkprobe.ResetIn, linkprobe.RefuseIn, linkprobe.FinIn:
			s += fmt.Sprintf(":%d", a.Stream)
		}
		out = append(out, s)
	}
	return out
}

// linkprobeHost replays a host script (spec §8.5.3 to §8.5.6): the events of one connection
// and the actions the host must ask for, in order.
func (c *referenceCase) linkprobeHost() error {
	var ctx struct {
		Transport string
		Roles     []string
	}
	if err := c.vector.Decode("context", &ctx); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	t, ok := lpTransports[ctx.Transport]
	if !ok {
		return fmt.Errorf("context: unknown transport %q", ctx.Transport)
	}
	r, err := roles(ctx.Roles)
	if err != nil {
		return err
	}
	es, err := c.events()
	if err != nil {
		return err
	}
	h := linkprobe.NewHost(t, r, ms(es[0].AtMs))
	for i, e := range es {
		now := ms(e.AtMs)
		data, err := hex.DecodeString(e.Data)
		if err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
		var as []linkprobe.Action
		switch e.Op {
		case "tick":
			as = h.Tick(now, e.Sent, e.writable())
		case "negotiated":
			as = h.Negotiated(now, e.OK, e.Sent, e.writable())
		case "outbound_data":
			as = h.OutboundData(now, data, e.SentNow, ms(e.PTOMs))
		case "outbound_ended":
			as = h.OutboundEnded(now)
		case "close":
			as = h.Close(now)
		case "inbound_opened":
			as = h.InboundOpened(now, e.Stream)
		case "inbound_data":
			as = h.InboundData(now, e.Stream, data, e.Received, e.writable())
		case "inbound_ended":
			as = h.InboundEnded(now, e.Stream, e.Fin)
		case "roles":
			r, err := roles(e.Roles)
			if err != nil {
				return fmt.Errorf("event %d: %w", i, err)
			}
			as = h.SetRoles(now, r)
		case "loss":
			if err := checkLoss(i, e, h.Loss(now)); err != nil {
				return err
			}
			continue
		case "level":
			if err := checkString(i, e, h.Level(now).String()); err != nil {
				return err
			}
			continue
		default:
			return fmt.Errorf("event %d: unknown op %q", i, e.Op)
		}
		var want []string
		if err := json.Unmarshal(e.Expect, &want); err != nil {
			return fmt.Errorf("event %d: expect: %w", i, err)
		}
		if got := actionStrings(as); !slices.Equal(got, want) {
			return fmt.Errorf("event %d (%s at %d ms): %v, want %v", i, e.Op, e.AtMs, got, want)
		}
	}
	c.verdictChecked = true
	return nil
}
