//go:build server

package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// withLinkStepLog makes the node record every step of the link sampler, one line each, so that the
// network simulation can replay the level from the figures it was derived from (plan C5S, rule R1):
// the headless build the end-to-end tests run (`-tags server`) logs them; the desktop build never
// does, and never sets the option (TestLinkProbeOnlyBridgeReads).
func withLinkStepLog(ctx context.Context, o *p2p.Options) {
	o.OnLinkStep, _ = linkStepSink(ctx, os.Stderr)
}

// linkStepQueue is how many steps wait for the logger before the sampler drops one.
const linkStepQueue = 256

// linkStepSink returns the callback of [p2p.Options.OnLinkStep] writing to w, and the count of
// steps it dropped. The callback runs on the sampler's goroutine and must not wait for the log: it
// hands the step to a goroutine through a buffer, and drops it, counted, when the buffer is full.
// A drop is written as its own line (`linkstepdropped count=N`) so the test that replays the steps
// knows a step is missing, instead of reading a gap as a quiet second.
func linkStepSink(ctx context.Context, w io.Writer) (func(p2p.LinkStep), *atomic.Uint64) {
	log := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey) {
				return slog.Attr{} // t is the sampler's own instant
			}
			return a
		},
	}))
	steps := make(chan p2p.LinkStep, linkStepQueue)
	var dropped atomic.Uint64
	go func() {
		var reported uint64
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-steps:
				if d := dropped.Load(); d != reported {
					log.Info("linkstepdropped", "count", d-reported)
					reported = d
				}
				log.Info("linkstep", "peer", string(s.Peer), "t", s.At.UnixMilli(), "kind", s.Kind.String(), "quality", s.Level.String(), "lossPermille", int(s.LossPermille))
			}
		}
	}()
	return func(s p2p.LinkStep) {
		select {
		case steps <- s:
		default:
			dropped.Add(1)
		}
	}, &dropped
}
