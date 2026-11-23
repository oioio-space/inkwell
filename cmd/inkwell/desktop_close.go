package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type closeProbeKey struct{}

// closeProbe provides the native acceptance walk with independent observations
// of decisions and a replaceable storage boundary in its child process.
type closeProbe struct {
	observe func(string)
	wrapKV  func(bridge.KV) bridge.KV
}

func observeClose(ctx context.Context, name string) {
	if probe, ok := ctx.Value(closeProbeKey{}).(closeProbe); ok {
		probe.observe(name)
	}
}

func bindDesktopClose(ctx context.Context, app *application.App, window *application.WebviewWindow, closer *closeCoordinator, open *opener, closePublic func() error, log *slog.Logger) {
	id := window.ID()
	closer.register(id, window.Close)
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !closer.requestWindow(id) {
			observeClose(ctx, "wails-close-vetoed")
			event.Cancel()
		} else {
			observeClose(ctx, "close-permit-consumed")
			// Hooks run before Wails destroys the window, so the web view is still there.
			if closer.abandoned(id) {
				application.InvokeSync(func() { terminateRenderer(window) })
			}
		}
	})
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		if closer.editorClosed(id) {
			if err := closePublic(); err != nil {
				log.Warn("closing the presentation window", "err", err)
			}
		}
	})
	app.HandleStream(openStreamName, func(stream *application.StreamConn) {
		owner := stream.Window()
		if owner == nil || owner.ID() != id {
			_ = stream.Close()
			return
		}
		defer func() { _ = stream.Close() }()
		ctx := stream.Context()
		abort := queueCloseAbort(ctx, func(frame closeFrame) error {
			return stream.SendJSON(frame)
		})
		g := closer.attach(id, func(frame closeFrame) error {
			b, err := json.Marshal(frame)
			if err != nil {
				return err
			}
			return stream.TrySend(b)
		}, abort)
		if g == 0 {
			return
		}
		defer closer.detach(id, g)
		if !closer.ready(id, g) {
			return
		}
		open.serve(stream, func(b []byte) bool {
			const maxCloseReply = 8192
			var frame closeFrame
			if len(b) > maxCloseReply || json.Unmarshal(b, &frame) != nil || frame.Kind == "" {
				return false
			}
			closer.report(id, g, frame)
			return true
		})
	})
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
		defer signal.Stop(signals)
		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				closer.requestAll()
			}
		}
	}()
	watchSessionLogout(ctx, closer, log)
	watchCloseView(ctx, app, window, closer)
}

// Abort must reach a page even when TrySend is full. The writer waits for
// stream capacity outside the coordinator lock and stops when the stream
// closes. Each newer Prepare replaces the old hold, so only the newest
// pending abort is needed. The coordinator serializes calls to enqueue.
func queueCloseAbort(ctx context.Context, send func(closeFrame) error) func(closeFrame) {
	pending := make(chan closeFrame, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case frame := <-pending:
				if err := send(frame); err != nil {
					return
				}
			}
		}
	}()
	return func(frame closeFrame) {
		select {
		case <-pending:
		default:
		}
		pending <- frame
	}
}
