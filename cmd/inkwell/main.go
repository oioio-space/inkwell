// Command inkwell is the desktop application: it opens the editor window and
// wires the webview to the network through the bridge (docs/architecture.md).
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oioio-space/inkwell"
	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())

	home, err := app.OpenHome()
	if err != nil {
		log.Error("opening identity directory", "err", err)
		os.Exit(1)
	}
	node, err := app.StartNode(ctx, home, app.DefaultPort)
	if err != nil {
		log.Error("starting network", "err", err)
		os.Exit(1)
	}
	if err := home.Close(); err != nil {
		log.Warn("closing identity directory", "err", err)
	}
	router := bridge.NewRouter(node, log)

	proj, err := openProject()
	if err != nil {
		log.Error("opening project", "err", err)
		os.Exit(1)
	}
	if err := proj.Backup(time.Now(), backupsKept); err != nil {
		log.Warn("backing up project", "err", err) // not fatal: the store still works
	}
	storeServer := bridge.NewStoreServer(proj, log)

	wails := application.New(application.Options{
		Name:        "Inkwell",
		Description: "Éditeur collaboratif Typst, sans serveur",
		Services: []application.Service{
			application.NewService(app.NewSession(node)),
			application.NewService(app.NewProjects(proj)),
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
			if err := proj.Close(); err != nil {
				log.Warn("closing project", "err", err)
			}
			if err := node.Close(); err != nil {
				log.Warn("closing network", "err", err)
			}
		},
	})
	wails.HandleStream(bridge.StreamName, func(c *application.StreamConn) { router.Serve(c) })
	wails.HandleStream(bridge.StoreStreamName, func(c *application.StreamConn) { storeServer.Serve(c) })

	go func() {
		if err := proj.Watch(ctx, storeServer.NotifyExternal); err != nil {
			log.Error("watching project files", "err", err)
		}
	}()

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

// backupsKept is how many project backups are kept (one per launch).
const backupsKept = 10

// openProject opens the most recent project in the user's Inkwell folder,
// creating an untitled one on first launch.
func openProject() (*project.Project, error) {
	docs, err := app.OpenDocs()
	if err != nil {
		return nil, err
	}
	defer func() { _ = docs.Close() }() // the project keeps its own handle
	return app.OpenProject(docs)
}

// serverPort returns INKWELL_SERVER_PORT, or 8080 when unset or invalid.
func serverPort() int {
	if p, err := strconv.Atoi(os.Getenv("INKWELL_SERVER_PORT")); err == nil && p > 0 && p < 65536 {
		return p
	}
	return 8080
}
