package mux_test

import (
	"bytes"
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/frame"
	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// msgSize is the large message of the SP1 tests: the 879 KiB paste of NI-3.
const msgSize = 879 << 10

// delaySends wraps a transport so that each fragment Send first waits
// delay(n) of virtual time, n counting the fragments sent so far from 1, and
// gives up if its context ends: a link that is slow, or goes silent.
type delaySends struct {
	transport.Transport
	delay func(n int) time.Duration
	n     int
}

func (d *delaySends) Send(ctx context.Context, to transport.PeerID, p []byte) error {
	if len(p) > 0 && p[0] == fragChannel {
		d.n++
		select {
		case <-time.After(d.delay(d.n)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return d.Transport.Send(ctx, to, p)
}

// slowSink is a link to nobody that carries rate bytes per second.
func slowSink(rate int) *delaySends {
	return &delaySends{Transport: &sink{closed: make(chan struct{})}, delay: func(int) time.Duration {
		return time.Duration(fragWire) * time.Second / time.Duration(rate)
	}}
}

// fragWire is a fragment on the wire for slowSink: 32 KiB plus its 14-byte header.
const fragWire = 32<<10 + 14

// TestRuleSP1BlackoutNoSilentDrop proves NoSilentDrop (rule SP1-3, model
// SP1Blackout): a 50 s silence in the middle of an 879 KiB message neither
// fails the send nor makes the reader drop the partial message.
func TestRuleSP1BlackoutNoSilentDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		want := content(msgSize, 7)
		a, b := linked(t, memnet.Faults{}, func(tr transport.Transport) transport.Transport {
			return &delaySends{Transport: tr, delay: func(n int) time.Duration {
				if n == 14 {
					return 50 * time.Second
				}
				return 0
			}}
		})
		if err := a.Send(t.Context(), "b", want); err != nil {
			t.Fatalf("Send across a 50 s silence = %v, want nil", err)
		}
		if ev := recvFor(t, b, 10*time.Minute); !bytes.Equal(ev.Payload, want) {
			t.Errorf("reader delivered %d bytes, want the %d-byte message intact", len(ev.Payload), len(want))
		}
	})
}

// TestRuleSP1PartialKeptPastWriteIdle proves NoSilentDrop (rule SP1-3, models
// SP1Silence, SP1PartialTTLAtIdle): PartialTTL is 120 s, above WriteIdle plus
// two one-way delays, and a fragment that comes that late still completes the
// message.
func TestRuleSP1PartialKeptPastWriteIdle(t *testing.T) {
	const gap = frame.WriteIdle + 2*frame.MaxOneWayDelay
	if mux.PartialTTL != 120*time.Second || mux.PartialTTL <= gap {
		t.Errorf("PartialTTL = %v, want 120s (D-SP1-4), above WriteIdle + 2D (%v)", mux.PartialTTL, gap)
	}
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x")
		want := content(64<<10, 3)
		sendFrag(t, raws["x"], frag(chRepo, 1, 64<<10, 0, want[:32<<10]))
		time.Sleep(gap)
		sendFrag(t, raws["x"], frag(chRepo, 1, 64<<10, 32<<10, want[32<<10:]))
		synctest.Wait()
		if got := messages(rig.repo); len(got) != 1 || !bytes.Equal(got[0], want) {
			t.Errorf("message whose fragment came %v late: delivered %d messages, want the one intact", gap, len(got))
		}
	})
}

// TestRuleSP1PartialFreedAfterTTL proves the other half of SP1-3: a fragment
// that never comes frees the memory PartialTTL after the previous one, however
// much other traffic wakes the reader meanwhile (it is not "up to twice").
func TestRuleSP1PartialFreedAfterTTL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rig, raws := newHostileRig(t, "x", "y")
		start := time.Now()
		sendFrag(t, raws["x"], frag(chRepo, 1, 1<<20, 0, make([]byte, 32<<10)))
		synctest.Wait()
		if rig.res.inUse() == 0 {
			t.Fatal("a partial message holds no reserved memory")
		}
		time.Sleep(frame.WriteIdle) // another peer's message wakes the reader
		sendFrag(t, raws["y"], []byte{chRepo, 'o', 'k'})
		time.Sleep(time.Until(start.Add(mux.PartialTTL - time.Second)))
		if rig.res.inUse() == 0 {
			t.Error("the partial message was freed before PartialTTL")
		}
		time.Sleep(2 * time.Second)
		if got := rig.res.inUse(); got != 0 {
			t.Errorf("%d bytes still reserved just after PartialTTL", got)
		}
	})
}

// TestRuleSP1SlowLinkDeliversWithoutResend proves DeliveredHonest (rule SP1-2,
// model SP1Floor): on a link that progresses at 8 KiB/s or at the 2 KiB/s
// floor, an 879 KiB message crosses at the first attempt, within the total
// budget WriteIdle + S / FloorRate that sendFragmented sets. The rates are
// written out, so that a change of frame.FloorRate cannot move the link along
// with it.
func TestRuleSP1SlowLinkDeliversWithoutResend(t *testing.T) {
	for name, rate := range map[string]int{"8KiB/s": 8 << 10, "FloorRate": 2 << 10} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				sub := mux.Split(slowSink(rate), chRepo)[chRepo]
				defer func() { _ = sub.Close() }()
				start := time.Now()
				if err := sub.Send(t.Context(), "b", make([]byte, msgSize)); err != nil {
					t.Fatalf("Send(%d bytes at %d B/s) = %v, want a first-attempt delivery", msgSize, rate, err)
				}
				if bound, d := frame.WriteIdle+msgSize*time.Second/frame.FloorRate, time.Since(start); d > bound {
					t.Errorf("took %v, want <= WriteIdle + S/FloorRate (%v)", d, bound)
				}
			})
		})
	}
}

// TestRuleSP1DripReaderCutWithinHoldBound proves HoldBound (rule SP1-2, model
// SP1Any): a reader below the floor that never stalls for WriteIdle is cut
// within WriteIdle + S/FloorRate (plus one drip interval), and the
// large-message slot is released so that the next Send proceeds at once.
func TestRuleSP1DripReaderCutWithinHoldBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		drip := frame.WriteIdle - time.Second // one fragment per interval: ~1.7 KiB/s
		link := &delaySends{Transport: &sink{closed: make(chan struct{})}, delay: func(int) time.Duration { return drip }}
		sub := mux.Split(link, chRepo)[chRepo]
		defer func() { _ = sub.Close() }()
		start := time.Now()
		if err := sub.Send(t.Context(), "b", make([]byte, msgSize)); err == nil {
			t.Fatal("Send to a drip reader succeeded, want it cut")
		}
		if bound, d := frame.WriteIdle+msgSize*time.Second/frame.FloorRate+drip, time.Since(start); d > bound {
			t.Errorf("held the sender %v, want <= %v", d, bound)
		}
		if g, _ := mux.GateCount(sub); g != 0 {
			t.Errorf("%d gates after the cut, want the slot released", g)
		}
		before := time.Now()
		if err := sub.Send(t.Context(), "b", []byte("small")); err != nil {
			t.Errorf("Send after the cut = %v", err)
		}
		if d := time.Since(before); d != 0 {
			t.Errorf("small Send after the cut took %v, want none", d)
		}
	})
}
