// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/oioio-space/inkwell/protocol/syncack"
)

func init() {
	decoderTable["sync_supersede"] = decoderSpec{check: (*referenceCase).syncSupersede}
}

// syncSupersede checks rule P-15 (spec §8.7.4): at now_ms, over the open connections to one peer, the proofs to
// start, the connections to close, the proofs to drop, the next deadline and each connection's proof state.
func (c *referenceCase) syncSupersede() error {
	var in struct {
		NowMs         int64 `json:"now_ms"`
		ProofWindowMs int64 `json:"proof_window_ms"`
		Conns         []struct {
			ID       uint64 `json:"id"`
			OpenedMs int64  `json:"opened_ms"`
			Acked    bool   `json:"acked"`
			Proof    *struct {
				StartedMs int64 `json:"started_ms"`
				ForMs     int64 `json:"for_ms"`
				Heard     bool  `json:"heard"`
			} `json:"proof"`
		} `json:"conns"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	if ms(in.ProofWindowMs) != syncack.ProofWindow {
		return fmt.Errorf("context: proof window %d ms, the reference's is %v", in.ProofWindowMs, syncack.ProofWindow)
	}
	if in.Conns == nil {
		return errors.New("context: conns is required")
	}
	conns := make([]syncack.Conn, 0, len(in.Conns))
	seen := map[uint64]bool{}
	for _, x := range in.Conns {
		if seen[x.ID] {
			return fmt.Errorf("context: connection %d twice", x.ID)
		}
		seen[x.ID] = true
		k := syncack.Conn{ID: x.ID, Opened: ms(x.OpenedMs), Acked: x.Acked}
		if p := x.Proof; p != nil {
			k.Proving, k.Started, k.For, k.Heard = true, ms(p.StartedMs), ms(p.ForMs), p.Heard
		}
		conns = append(conns, k)
	}
	now := ms(in.NowMs)
	d := syncack.Supersede(now, conns)
	states := map[string]string{}
	for _, k := range conns {
		states[strconv.FormatUint(k.ID, 10)] = k.State(now).String()
	}
	if err := c.verdictIs(true); err != nil { // a decision is always accepted
		return err
	}
	return c.matchExpect(map[string]any{
		"prove": d.Prove, "close": d.Close, "drop": d.Drop, "wake_ms": d.Wake.Milliseconds(), "states": states,
	})
}
