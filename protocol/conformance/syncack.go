// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/oioio-space/inkwell/protocol/syncack"
)

func init() {
	decoderTable["sync_ack"] = decoderSpec{check: (*referenceCase).syncAckRead}
	decoderTable["sync_ack_responder"] = decoderSpec{check: (*referenceCase).syncAckResponder}
	decoderTable["sync_ack_opener"] = decoderSpec{check: (*referenceCase).syncAckOpener}
	decoderTable["sync_order"] = decoderSpec{check: (*referenceCase).syncOrder}
	decoderTable["sync_refusal"] = decoderSpec{check: (*referenceCase).syncRefusal}
}

// syncAckRead checks an opener's reading of its stream (spec §8.7.2): result is the verdict on the
// first byte (accepted, refused, violation), after the verdict on the bytes left once accepted
// (none, violation). A vector with no input_hex is a stream that ends at once.
func (c *referenceCase) syncAckRead() error {
	var in []byte
	if _, ok := c.vector.Fields["input_hex"]; ok {
		in = c.unhex(c.inputHex())
	}
	r := bytes.NewReader(in)
	err := syncack.ReadAck(r)
	result, after := "accepted", "none"
	switch {
	case errors.Is(err, syncack.ErrRefused):
		result = "refused"
	case errors.Is(err, syncack.ErrViolation):
		result = "violation"
	case err != nil:
		return fmt.Errorf("unclassified error %w", err)
	case r.Len() > 0:
		after = "violation"
	}
	if err := c.verdictIs(result == "accepted" && after == "none"); err != nil {
		return err
	}
	return c.matchExpect(map[string]any{"result": result, "after": after})
}

// syncAckResponder checks the responder's decision (spec §8.7.3) for one state.
func (c *referenceCase) syncAckResponder() error {
	var in struct {
		NodeClosed *bool `json:"node_closed"`
		Admitted   *bool `json:"admitted"`
		ConnClosed *bool `json:"conn_closed"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	if in.NodeClosed == nil || in.Admitted == nil || in.ConnClosed == nil {
		return errors.New("context: node_closed, admitted and conn_closed are required")
	}
	ack := (syncack.Responder{NodeClosed: *in.NodeClosed, Admitted: *in.Admitted, ConnClosed: *in.ConnClosed}).Accept()
	if err := c.verdictIs(ack); err != nil {
		return err
	}
	action := "reset"
	if ack {
		action = "ack"
	}
	return c.matchExpect(map[string]any{"action": action})
}

// verdictIs checks the vector's verdict against accept and marks it checked.
func (c *referenceCase) verdictIs(accept bool) error {
	got := pick(accept, "accept", "reject")
	if got != c.vector.Verdict {
		return fmt.Errorf("verdict %s, want %s", got, c.vector.Verdict)
	}
	c.verdictChecked = true
	return nil
}

// syncAckOpener runs an opener script (spec §8.7.4, §8.7.5): every event's outcome must be the
// one it expects.
func (c *referenceCase) syncAckOpener() error {
	var in struct {
		Role       string `json:"role"`
		Announced  *bool  `json:"peer_announced"`
		DeadlineMs int64  `json:"deadline_ms"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	role, ok := syncack.ParseRole(in.Role)
	if !ok || in.Announced == nil {
		return fmt.Errorf("context: role %q, peer_announced %v", in.Role, in.Announced)
	}
	if time.Duration(in.DeadlineMs)*time.Millisecond != syncack.Budget {
		return fmt.Errorf("context: deadline %d ms, the reference's is %v", in.DeadlineMs, syncack.Budget)
	}
	es, err := c.events()
	if err != nil {
		return err
	}
	o := syncack.NewOpener(role, *in.Announced)
	for i, e := range es {
		op, ok := syncack.ParseOp(e.Op)
		if !ok {
			return fmt.Errorf("event %d: unknown op %q", i, e.Op)
		}
		ev := syncack.Event{Op: op, At: time.Duration(e.AtMs) * time.Millisecond}
		if op == syncack.OpByte {
			b, err := hex.DecodeString(e.Byte)
			if err != nil || len(b) != 1 {
				return fmt.Errorf("event %d: byte_hex %q", i, e.Byte)
			}
			ev.Byte = b[0]
		}
		got, err := o.Step(ev)
		if err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
		if err := checkString(i, e, got.String()); err != nil {
			return err
		}
	}
	return c.verdictIs(true) // an opener script is always accepted
}

// syncOrder runs a script of one peer's inbound order (spec §8.7.3, rules P-8, P-9): every event's answer must be
// the one it expects, and an end must wake exactly the streams it names.
func (c *referenceCase) syncOrder() error {
	var in struct {
		Places        int   `json:"places"`
		ReorderWaitMs int64 `json:"reorder_wait_ms"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	if in.Places != syncack.Places || time.Duration(in.ReorderWaitMs)*time.Millisecond != syncack.ReorderWait {
		return fmt.Errorf("context: places %d, reorder wait %d ms; the reference's are %d, %v", in.Places, in.ReorderWaitMs, syncack.Places, syncack.ReorderWait)
	}
	es, err := c.events()
	if err != nil {
		return err
	}
	var o syncack.Order
	for i, e := range es {
		at := ms(e.AtMs)
		var got string
		switch e.Op {
		case "arrive":
			got = pick(o.Arrive(e.Stream, at), "place", "busy")
		case "ack":
			got = "acked"
			if !o.Holds(e.Stream) {
				return fmt.Errorf("event %d: ack of stream %d, which holds no place", i, e.Stream)
			}
		case "ready":
			got = pick(o.Ready(e.Stream, at), "turn", "wait")
		case "end":
			got = "leave"
			if woken := o.Leave(e.Stream); !slices.Equal(woken, e.Wake) && (len(woken) > 0 || len(e.Wake) > 0) {
				return fmt.Errorf("event %d: end of stream %d woke %v, want %v", i, e.Stream, woken, e.Wake)
			}
		default:
			return fmt.Errorf("event %d: unknown op %q", i, e.Op)
		}
		if err := checkString(i, e, got); err != nil {
			return err
		}
	}
	return c.verdictIs(true)
}

// syncRefusal checks the opener's decision on a reset before the ACK (spec §8.7.4, rule P-8).
func (c *referenceCase) syncRefusal() error {
	var in struct {
		Role      string  `json:"role"`
		ResetCode *uint32 `json:"reset_code"`
		ElapsedMs int64   `json:"elapsed_ms"`
		Attempt   int     `json:"attempt"`
		BudgetMs  int64   `json:"budget_ms"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	role, ok := syncack.ParseRole(in.Role)
	if !ok || time.Duration(in.BudgetMs)*time.Millisecond != syncack.Budget {
		return fmt.Errorf("context: role %q, budget %d ms", in.Role, in.BudgetMs)
	}
	capacity := in.ResetCode != nil && *in.ResetCode == syncack.BusyCode
	action, wait := syncack.OnRefusal(role, capacity, ms(in.ElapsedMs), in.Attempt)
	if err := c.verdictIs(true); err != nil {
		return err
	}
	actual := map[string]any{"action": action.String()}
	if action == syncack.Retry {
		actual["wait_ms"] = wait.Milliseconds()
	}
	return c.matchExpect(actual)
}

// pick is yes if b, else no.
func pick(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}
