// Command inkwell is the desktop application: it opens the editor window and
// wires the webview to the network through the bridge (docs/architecture.md).
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oioio-space/inkwell"
	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/transport"
	"github.com/oioio-space/inkwell/internal/transport/tcp"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())

	// Stage 1: plain TCP between trusted LAN peers; libp2p replaces it at stage 2.
	node := tcp.New(tcp.NewPeerID(), tcp.Options{})
	router := bridge.NewRouter(node, log)

	wails := application.New(application.Options{
		Name:        "Inkwell",
		Description: "Éditeur collaboratif Typst, sans serveur",
		Services: []application.Service{
			application.NewService(app.NewSession(node)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(inkwell.Assets),
		},
		// Used only by `-tags server` builds (headless end-to-end tests): bound to
		// loopback, port from INKWELL_SERVER_PORT.
		Server: application.ServerOptions{Host: "127.0.0.1", Port: serverPort()},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			cancel()
			if err := node.Close(); err != nil {
				log.Warn("closing network", "err", err)
			}
		},
	})
	wails.HandleStream(bridge.StreamName, func(c *application.StreamConn) { router.Serve(c) })

	go func() {
		// Run only returns once ctx is cancelled or the network is closed; anything
		// else means the bridge died while the app is still running.
		if err := router.Run(ctx); ctx.Err() == nil && !errors.Is(err, transport.ErrClosed) {
			log.Error("bridge stopped", "err", err)
		}
	}()

	wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Inkwell",
		Width:     1280,
		Height:    800,
		MinWidth:  720,
		MinHeight: 480,
		URL:       "/",
	})

	if err := wails.Run(); err != nil {
		log.Error("inkwell stopped", "err", err)
		os.Exit(1)
	}
}

// serverPort returns INKWELL_SERVER_PORT, or 8080 when unset or invalid.
func serverPort() int {
	if p, err := strconv.Atoi(os.Getenv("INKWELL_SERVER_PORT")); err == nil && p > 0 && p < 65536 {
		return p
	}
	return 8080
}
