// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"errors"
	"fmt"

	transport "github.com/oioio-space/inkwell/protocol"
	"github.com/oioio-space/inkwell/protocol/syncack"
)

func init() {
	decoderTable["inbound_lifecycle"] = decoderSpec{check: (*referenceCase).inboundLifecycle}
	decoderTable["inbound_probe"] = decoderSpec{check: (*referenceCase).inboundProbe}
	decoderTable["quic_window"] = decoderSpec{check: (*referenceCase).quicWindow}
}

// ilEvent is one observation of an inbound stream's reader (decoder inbound_lifecycle, spec §8.7.3).
type ilEvent struct {
	Op        string `json:"op"`
	AtMs      int64  `json:"at_ms"`
	Expect    string `json:"expect"`
	Placed    bool   `json:"placed"`
	OK        bool   `json:"ok"`
	N         int    `json:"n"`
	Err       string `json:"err"`
	Prefix    *int   `json:"prefix"`
	Leave     bool   `json:"leave"`
	Room      bool   `json:"room"`
	OwnConn   bool   `json:"own_conn"`
	Deliver   int    `json:"deliver"`
	Truncated bool   `json:"truncated"`
}

// inboundLifecycle replays a script of one inbound stream on the reference lifecycle (spec §8.1, §8.7.3, rules
// P-10 to P-14): after each event the stream must be in the state it expects, have left its place exactly at the
// events marked leave, and hold the probed bytes it states. The lane changes nothing: both lanes share the rules.
func (c *referenceCase) inboundLifecycle() error {
	var in struct {
		Lane          string `json:"lane"`
		Places        int    `json:"places"`
		ProbeEveryMs  int64  `json:"probe_every_ms"`
		ProbeMax      int    `json:"probe_max"`
		ReorderWaitMs int64  `json:"reorder_wait_ms"`
		UpWaitMs      int64  `json:"up_wait_ms"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	if (in.Lane != "sync" && in.Lane != "bulk") || in.Places != syncack.Places || ms(in.ProbeEveryMs) != syncack.ProbeEvery ||
		in.ProbeMax != syncack.ProbeMax || ms(in.ReorderWaitMs) != syncack.ReorderWait || ms(in.UpWaitMs) != syncack.UpWait {
		return fmt.Errorf("context %+v differs from the reference's bounds", in)
	}
	var es []ilEvent
	if err := c.vector.Decode("events", &es); err != nil {
		return fmt.Errorf("events: %w", err)
	}
	if len(es) == 0 {
		return errors.New("no events")
	}
	var l syncack.Lifecycle
	for i, e := range es {
		if i > 0 && e.AtMs < es[i-1].AtMs {
			return fmt.Errorf("event %d goes back in time", i)
		}
		at := ms(e.AtMs)
		var leave bool
		switch e.Op {
		case "arrive":
			l.Arrive(e.Placed)
		case "ack":
			leave = l.Ack(e.OK, at)
		case "read":
			end, ok := syncack.ParseReadEnd(e.Err)
			if !ok {
				return fmt.Errorf("event %d: unknown read end %q", i, e.Err)
			}
			leave = l.Read(e.N, end, at)
		case "turn":
			l.Turn()
		case "up":
			l.Up()
		case "budget":
			l.Budget(e.Room)
		case "reset_unseen": // a reset no read observes (the reader is blocked on the budget): nothing changes
		case "conn_lost":
			leave = l.ConnLost(e.OwnConn)
		case "decode":
			if !e.Truncated || e.Deliver < 0 {
				return fmt.Errorf("event %d: a decode reports its whole frames and a truncated tail", i)
			}
			leave = l.Decoded()
		default:
			return fmt.Errorf("event %d: unknown op %q", i, e.Op)
		}
		if got := l.State().String(); got != e.Expect {
			return fmt.Errorf("event %d (%s at %d ms): %s, want %s", i, e.Op, e.AtMs, got, e.Expect)
		}
		if leave != e.Leave {
			return fmt.Errorf("event %d (%s at %d ms): leave %t, want %t", i, e.Op, e.AtMs, leave, e.Leave)
		}
		if e.Prefix != nil && l.Prefix() != *e.Prefix {
			return fmt.Errorf("event %d (%s at %d ms): %d probed bytes held, want %d", i, e.Op, e.AtMs, l.Prefix(), *e.Prefix)
		}
	}
	return c.verdictIs(true)
}

// inboundProbe checks a waiting reader's probe schedule (spec §8.7.3, rule P-11): the attempts' start times and
// the bytes' times, against the reference's bounds.
func (c *referenceCase) inboundProbe() error {
	var in struct {
		ProbeEveryMs int64   `json:"probe_every_ms"`
		ProbeMax     int     `json:"probe_max"`
		AttemptsMs   []int64 `json:"attempts_ms"`
		BytesMs      []int64 `json:"bytes_ms"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	if ms(in.ProbeEveryMs) != syncack.ProbeEvery || in.ProbeMax != syncack.ProbeMax {
		return fmt.Errorf("context: probe every %d ms, %d bytes; the reference's are %v, %d", in.ProbeEveryMs, in.ProbeMax, syncack.ProbeEvery, syncack.ProbeMax)
	}
	var p syncack.ProbeSchedule
	ok := true
	for _, a := range in.AttemptsMs {
		ok = ok && p.Attempt(ms(a)) == nil
	}
	for _, b := range in.BytesMs {
		ok = ok && p.Byte(ms(b)) == nil
	}
	return c.verdictIs(ok)
}

// quicWindow checks rule P-13's inequality (spec §8.7.3, §16.1) on the windows of a vector.
func (c *referenceCase) quicWindow() error {
	var in struct {
		Places             uint64 `json:"places"`
		Lanes              uint64 `json:"lanes"`
		StreamWindow       uint64 `json:"stream_window"`
		UpdateThresholdDen uint64 `json:"update_threshold_den"`
		ConnWindow         uint64 `json:"conn_window"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	if in.Places != syncack.Places || in.Lanes != transport.InboundLanes {
		return fmt.Errorf("context: %d places, %d lanes; the reference's are %d, %d", in.Places, in.Lanes, syncack.Places, transport.InboundLanes)
	}
	return c.verdictIs(syncack.QUICWindowOK(in.Places, in.Lanes, in.StreamWindow, in.ConnWindow, in.UpdateThresholdDen))
}
