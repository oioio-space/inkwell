// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oioio-space/inkwell/protocol/directup"
)

func init() {
	decoderTable["directup_classify"] = decoderSpec{check: (*referenceCase).directupClassify}
	decoderTable["directup_role"] = decoderSpec{check: (*referenceCase).directupRole}
	decoderTable["directup_ladder"] = decoderSpec{check: (*referenceCase).directupLadder}
	decoderTable["directup_bucket"] = decoderSpec{check: (*referenceCase).directupBucket}
	decoderTable["directup_select"] = decoderSpec{check: (*referenceCase).directupSelect}
}

// matchJSON compares each expected value with the JSON encoding of the actual one.
func (c *referenceCase) matchJSON(actual map[string]any) error {
	c.verdictChecked = true
	for key, raw := range c.expect {
		v, ok := actual[key]
		if !ok {
			return fmt.Errorf("expectation %s: no such field", key)
		}
		got, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var want bytes.Buffer
		if err := json.Compact(&want, raw); err != nil {
			return err
		}
		if !bytes.Equal(got, want.Bytes()) {
			return fmt.Errorf("expectation %s: got %s, want %s", key, got, want.Bytes())
		}
	}
	return nil
}

// directupClassify checks the class of a call of spec §8.6 from what it observed.
func (c *referenceCase) directupClassify() error {
	var in struct {
		Rounds  int    `json:"rounds"`
		Dialled bool   `json:"dialled"`
		Direct  bool   `json:"direct"`
		Outcome string `json:"outcome"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	out, ok := map[string]directup.Outcome{"ok": directup.OK, "busy": directup.ErrBusy, "closed": directup.ErrClosed, "other": directup.ErrOther}[in.Outcome]
	if !ok {
		return fmt.Errorf("context: unknown outcome %q", in.Outcome)
	}
	return c.matchJSON(map[string]any{"class": directup.Classify(in.Rounds, in.Dialled, in.Direct, out).String()})
}

// directupRole checks who initiates (spec §8.6, rôles).
func (c *referenceCase) directupRole() error {
	var in struct {
		In     bool `json:"in"`
		Out    bool `json:"out"`
		Public bool `json:"remote_public"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	return c.matchJSON(map[string]any{"action": directup.Role(directup.Obs{In: in.In, Out: in.Out, RemotePublic: in.Public}).String()})
}

// directupLadder checks the wait after the n-th call of a class in a row, without jitter: the calls
// are an hour apart, so the attempt budget never delays them.
func (c *referenceCase) directupLadder() error {
	var in struct {
		Class string `json:"class"`
		Calls int    `json:"calls"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	class, ok := map[string]directup.Class{"unspent": directup.Unspent, "spent": directup.Spent}[in.Class]
	if !ok || in.Calls < 1 {
		return fmt.Errorf("context: class %q, calls %d", in.Class, in.Calls)
	}
	now := time.Unix(0, 0)
	p := directup.NewPolicy(now, func(d time.Duration) time.Duration { return d })
	o := directup.Obs{Enabled: true, Wanted: true, Relayed: true, In: true, LocalReady: true}
	var wait time.Duration
	for i := range in.Calls {
		if d := p.Step(o, now); d.Action == directup.None {
			return fmt.Errorf("no call at the %d-th step", i+1)
		}
		p.Result(p.Start(now), class, now)
		wait = p.Due().Sub(now)
		now = p.Due().Add(time.Hour)
	}
	return c.matchJSON(map[string]any{"wait_ms": wait.Milliseconds()})
}

// directupBucket checks one of the four buckets of spec §8.6 and §16: takes every period_ms during
// window_s seconds (both ends included), or a hundred at once when period_ms is 0.
func (c *referenceCase) directupBucket() error {
	var in struct {
		Bucket   string `json:"bucket"`
		PeriodMs int64  `json:"period_ms"`
		WindowS  int64  `json:"window_s"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	now := time.Unix(0, 0)
	var take func(time.Time) bool
	switch in.Bucket {
	case "attempts":
		b := directup.NewBucket(directup.AttemptBurst, directup.AttemptRefill, now)
		take = b.Take
	case "rounds":
		b := directup.NewBudgets(now)
		take = b.Rounds.Take
	case "dests":
		b := directup.NewBudgets(now)
		take = b.Dests.Take
	case "node": // one admission per grant: a peer that takes the slot and releases it at once
		s := directup.NewSlots(now)
		take = func(t time.Time) bool {
			if !s.Ask("p", t) {
				return false
			}
			s.Release("p", t)
			return true
		}
	default:
		return fmt.Errorf("context: unknown bucket %q", in.Bucket)
	}
	granted := 0
	if in.PeriodMs <= 0 {
		for range 100 {
			if take(now) {
				granted++
			}
		}
	} else {
		period := time.Duration(in.PeriodMs) * time.Millisecond
		end := now.Add(time.Duration(in.WindowS) * time.Second) // inclusive: « in any hour » counts both ends
		for t := now; !t.After(end); t = t.Add(period) {
			if take(t) {
				granted++
			}
		}
	}
	return c.matchJSON(map[string]any{"granted": granted})
}

// directupSelect checks the address selection of spec §8.6. An address is named by its transport and
// IP family (quic4, quic6, tcp4, tcp6), with -private or -circuit when it is not a public direct one, or
// -dns when it names its host instead of an IP address (/dns4/…).
func (c *referenceCase) directupSelect() error {
	var in struct {
		Addrs []string `json:"addrs"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	addrs := make([]directup.Addr, len(in.Addrs))
	for i, s := range in.Addrs {
		kind, attr, _ := strings.Cut(s, "-")
		a := directup.Addr{Public: attr != "private", Circuit: attr == "circuit", Name: attr == "dns"}
		switch kind {
		case "quic4":
			a.QUIC = true
		case "quic6":
			a.QUIC, a.IPv6 = true, true
		case "tcp4":
			a.TCP = true
		case "tcp6":
			a.TCP, a.IPv6 = true, true
		default:
			return fmt.Errorf("context: unknown address %q", s)
		}
		addrs[i] = a
	}
	sel := directup.Select(addrs)
	if sel == nil {
		sel = []int{}
	}
	return c.matchJSON(map[string]any{"selected": sel})
}
