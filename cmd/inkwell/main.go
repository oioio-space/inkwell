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
)

const (
	// lanEnv set to "1" turns on the local network discovery, until the
	// settings screen has the setting "Découverte sur le réseau local".
	lanEnv = "INKWELL_LAN"
	// rendezvousEnv is the URL of the Wormhole rendezvous server that carries
	// the short codes: unset for the public server, "off" for no codes.
	rendezvousEnv = "INKWELL_RENDEZVOUS"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())

	home, err := app.OpenHome()
	if err != nil {
		log.Error("opening identity directory", "err", err)
		os.Exit(1)
	}
	rt, err := app.Start(ctx, home, app.Options{
		Log:           log,
		Title:         "Inkwell",
		Port:          app.DefaultPort,
		LAN:           os.Getenv(lanEnv) == "1",
		RendezvousURL: os.Getenv(rendezvousEnv),
	})
	if err != nil {
		log.Error("starting network", "err", err)
		os.Exit(1)
	}
	if err := home.Close(); err != nil {
		log.Warn("closing identity directory", "err", err)
	}
	router := bridge.NewRouter(rt.Repo, log)

	wails := application.New(application.Options{
		Name:        "Inkwell",
		Description: "Éditeur collaboratif Typst, sans serveur",
		Services: []application.Service{
			application.NewService(rt.Session),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(inkwell.Assets),
			Middleware: app.CSPNonce(),
		},
		// Used only by `-tags server` builds (headless end-to-end tests): bound to
		// loopback, port from INKWELL_SERVER_PORT.
		Server: application.ServerOptions{Host: "127.0.0.1", Port: serverPort()},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			cancel()
			if err := rt.Close(); err != nil {
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
