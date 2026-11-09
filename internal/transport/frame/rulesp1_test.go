package frame_test

import (
	"errors"
	"os"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/frame"
)

const (
	// sp1Window is the yamux stream window pinned by SP1-5 (256 KiB). The p2p
	// package states it too, in its muxer; this is the one copy of the number
	// in this package, as the frame package cannot import p2p.
	sp1Window = 256 << 10
	// msgSize is the large message of NI-3: 879 KiB.
	msgSize = 879 << 10
)

// sendFrames writes size bytes as 32 KiB frames through frame.WriteProgress
// with the package's WriteIdle and FloorRate, as the mux does, and returns the
// first error.
func sendFrames(t *testing.T, c *windowConn, size int) error {
	t.Helper()
	for rest := size; rest > 0; rest -= 32 << 10 {
		if err := frame.WriteProgress(t.Context(), c, frame.WriteIdle, frame.FloorRate, make([]byte, min(rest, 32<<10))); err != nil {
			return err
		}
	}
	return nil
}

// TestRuleSP1NoFalseCut proves NoFalseCut and DeliveredHonest (rules SP1-1,
// SP1-2, models SP1Relay, SP1Floor): a reader that drains at 8 KiB/s behind 3 s
// of delay, or at the floor behind 5 s, is never cut by WriteIdle. The first
// stall is half a window of reading plus 2D at most (E1 measured 21.8 s at
// 8 KiB/s), no Write blocks for WriteIdle, and the message ends within
// WriteIdle + S / FloorRate.
func TestRuleSP1NoFalseCut(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rate  int
		delay time.Duration
	}{
		{"At8KiB", 8 << 10, 3 * time.Second},
		{"AtFloor", frame.FloorRate, frame.MaxOneWayDelay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := newWindowConn(sp1Window, tc.rate, tc.delay)
				start := time.Now()
				if err := sendFrames(t, c, msgSize); err != nil {
					t.Fatalf("frames at %d B/s: %v, want every frame to succeed at the first attempt", tc.rate, err)
				}
				if len(c.blocked) == 0 {
					t.Fatal("no Write blocked: the window never closed, the test proves nothing")
				}
				lo := sp1Window / 2 * time.Second / time.Duration(tc.rate)
				hi := lo + 2*tc.delay + time.Second
				if first := c.blocked[0]; first < lo || first > hi {
					t.Errorf("first stall %v, want in [W/2/R, W/2/R + 2d + 1 s] = [%v, %v]", first, lo, hi)
				}
				if worst := slices.Max(c.blocked); worst > hi || worst >= frame.WriteIdle {
					t.Errorf("a Write blocked for %v, want <= %v and < WriteIdle (%v)", worst, hi, frame.WriteIdle)
				}
				if total := time.Since(start); total > frame.WriteIdle+msgSize*time.Second/frame.FloorRate {
					t.Errorf("message took %v, want within WriteIdle + S/FloorRate", total)
				}
			})
		})
	}
}

// TestRuleSP1StoppedReaderCutWithinWriteIdle proves StopCut (rule SP1-1, model
// SP1Any): a reader that stops cuts the next blocked frame with a timeout
// between WriteIdle and WriteIdle + 1 s after the last Write returned.
func TestRuleSP1StoppedReaderCutWithinWriteIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newWindowConn(sp1Window, 8<<10, time.Second)
		c.stopAfter = 300 << 10
		var last time.Time // when the Write that failed began, as the previous one returned
		var err error
		for err == nil {
			last = time.Now()
			err = frame.WriteProgress(t.Context(), c, frame.WriteIdle, frame.FloorRate, make([]byte, 32<<10))
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("WriteProgress(stopped reader) = %v, want a deadline error", err)
		}
		if d := time.Since(last); d < frame.WriteIdle || d > frame.WriteIdle+time.Second {
			t.Errorf("cut %v after the last Write returned, want in [WriteIdle, WriteIdle+1s]", d)
		}
	})
}
