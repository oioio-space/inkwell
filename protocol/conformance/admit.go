// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/oioio-space/inkwell/protocol/admit"
)

func init() {
	decoderTable["admit_ledger"] = decoderSpec{check: (*referenceCase).admitLedger}
}

// adEvent is one step of an admission script (spec §16.1, block RF1).
type adEvent struct {
	Op     string          `json:"op"`
	AtMs   int64           `json:"at_ms"`
	Conn   string          `json:"conn"`
	Dir    string          `json:"dir"`
	Origin string          `json:"origin"`
	Peer   string          `json:"peer"`
	Class  string          `json:"class"`
	Expect json.RawMessage `json:"expect"`
}

// admitLedger replays an admission script on the reference ledger: each opening's and handshake's outcome
// ("ok" or the refusal's reason) and each check's counts.
func (c *referenceCase) admitLedger() error {
	var in struct {
		Bounds struct {
			PerRelay      int   `json:"per_relay"`
			PerIP         int   `json:"per_ip"`
			PerWide       int   `json:"per_ipv6_48"`
			Pending       int   `json:"pending"`
			DeadlineMs    int64 `json:"deadline_ms"`
			Burst         int   `json:"burst"`
			RefillMs      int64 `json:"refill_ms"`
			Members       int   `json:"members"`
			Share         int   `json:"share"`
			PerMember     int   `json:"per_member"`
			Relays        int   `json:"relays"`
			PerRelayConns int   `json:"per_relay_conns"`
		} `json:"bounds"`
		Roster []string `json:"roster"`
	}
	if err := c.vector.Decode("context", &in); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	var events []adEvent
	if err := c.vector.Decode("events", &events); err != nil {
		return fmt.Errorf("events: %w", err)
	}
	if len(events) == 0 {
		return errors.New("events: none")
	}
	b := in.Bounds
	r := &adRun{l: admit.New(admit.Config{
		PerRelay: b.PerRelay, PerIP: b.PerIP, PerWide: b.PerWide, Pending: b.Pending, Deadline: admit.Nanos(ms(b.DeadlineMs)),
		Burst: b.Burst, Refill: admit.Nanos(ms(b.RefillMs)), Members: b.Members, Share: b.Share, PerMember: b.PerMember,
		Relays: b.Relays, PerRelayConns: b.PerRelayConns,
	}), conns: map[string]admit.ID{}}
	r.l.SetRoster(in.Roster)
	for i, e := range events {
		if err := r.step(e); err != nil {
			return fmt.Errorf("event %d (%s %s at %d ms): %w", i, e.Op, e.Conn, e.AtMs, err)
		}
	}
	return c.verdictIs(true) // a script is always accepted
}

// adRun replays one script: the ledger and its connections by the script's names.
type adRun struct {
	l     *admit.Ledger
	conns map[string]admit.ID
}

func (r *adRun) step(e adEvent) error {
	now := admit.Nanos(ms(e.AtMs))
	switch e.Op {
	case "open":
		return r.open(now, e)
	case "identify":
		class, ok := map[string]admit.Class{"member": admit.Member, "relay": admit.Relay, "stranger": admit.Stranger}[e.Class]
		if !ok {
			return fmt.Errorf("class %q", e.Class)
		}
		id, ok := r.conns[e.Conn]
		if !ok {
			return errors.New("connection not open")
		}
		return adMatch(e, adOutcome(r.l.Identify(now, id, e.Peer, class)))
	case "close":
		id, ok := r.conns[e.Conn]
		if !ok {
			return errors.New("connection not open")
		}
		r.l.Close(now, id)
		delete(r.conns, e.Conn)
		return nil
	case "check":
		return r.check(now, e)
	}
	return fmt.Errorf("op %q", e.Op)
}

func (r *adRun) open(now admit.Nanos, e adEvent) error {
	dir, o := admit.Inbound, admit.Origin{}
	switch e.Dir {
	case "out":
		dir = admit.Outbound
	case "in":
		var err error
		if o, err = adOrigin(e.Origin); err != nil {
			return err
		}
	default:
		return fmt.Errorf("dir %q", e.Dir)
	}
	id, err := r.l.Open(now, dir, o)
	if err == nil {
		r.conns[e.Conn] = id
	}
	return adMatch(e, adOutcome(err))
}

// check compares the counts at now with the expected ones.
func (r *adRun) check(now admit.Nanos, e adEvent) error {
	o, err := adOrigin(e.Origin)
	if err != nil {
		return err
	}
	var want map[string]int
	if err := json.Unmarshal(e.Expect, &want); err != nil {
		return fmt.Errorf("expect: %w", err)
	}
	s := r.l.Stats(now)
	have := map[string]int{"pending": s.Pending, "origin_pending": s.PerOrigin[o], "tokens": r.l.Tokens(now, o), "members": s.Members}
	for k, v := range want {
		h, ok := have[k]
		if !ok {
			return fmt.Errorf("expectation %s: no such count", k)
		}
		if h != v {
			return fmt.Errorf("%s = %d, want %d", k, h, v)
		}
	}
	return nil
}

// adMatch compares an outcome with the event's expected one.
func adMatch(e adEvent, got string) error {
	want, err := expectString(e.Expect)
	if err != nil {
		return fmt.Errorf("expect: %w", err)
	}
	if got != want {
		return fmt.Errorf("%s, want %s", got, want)
	}
	return nil
}

// adOrigin parses "relay:<PeerID>" or "ip:<address>".
func adOrigin(s string) (admit.Origin, error) {
	kind, v, _ := strings.Cut(s, ":")
	switch kind {
	case "relay":
		if v != "" {
			return admit.RelayOrigin(v), nil
		}
	case "ip":
		if a, err := netip.ParseAddr(v); err == nil {
			return admit.IPOrigin(a), nil
		}
	}
	return admit.Origin{}, fmt.Errorf("origin %q", s)
}

func adOutcome(err error) string {
	if err == nil {
		return "ok"
	}
	if r := admit.ReasonOf(err); r != 0 {
		return r.String()
	}
	return "error: " + err.Error()
}
