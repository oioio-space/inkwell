// Command inkwell is the desktop application: it opens the editor window and
// wires the webview to the network through the bridge (docs/architecture.md).
package main

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/oioio-space/inkwell/internal/version"
)

const (
	// appID is the reverse-DNS identifier of the application on Linux: the name of the
	// .desktop file, the AppStream component and the icons (ADR 0025).
	appID = "io.github.oioio_space.Inkwell"
	// lanEnv set to "1" turns on the local network discovery, until the
	// settings screen has the setting "Découverte sur le réseau local".
	lanEnv = "INKWELL_LAN"
	// rendezvousEnv is the URL of the Wormhole rendezvous server that carries
	// the short codes: unset for the public server, "off" for no codes.
	rendezvousEnv = "INKWELL_RENDEZVOUS"
	// exportDirEnv, when set, is where "Exporter en PDF" saves without asking:
	// for headless runs (end-to-end tests), where no dialog can open.
	exportDirEnv = "INKWELL_EXPORT_DIR"
)

func main() {
	if versionRequested(os.Args[1:]) {
		fmt.Println(version.Current())
		return
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())

	home, err := app.OpenHome()
	if err != nil {
		log.Error("opening identity directory", "err", err)
		os.Exit(1)
	}
	proj, err := openProject()
	if err != nil {
		log.Error("opening project", "err", err)
		os.Exit(1)
	}
	var wails *application.App // the export dialog needs the application, created below
	choose := func(name string) (string, error) {
		return wails.Dialog.SaveFile().SetFilename(name).AddFilter("PDF", "*.pdf").PromptForSingleSelection()
	}
	if dir := os.Getenv(exportDirEnv); dir != "" {
		choose = app.DirChooser(dir)
	}
	rt, err := app.Start(ctx, home, app.Options{
		Log:           log,
		Project:       proj,
		Port:          app.DefaultPort,
		LAN:           os.Getenv(lanEnv) == "1",
		RendezvousURL: os.Getenv(rendezvousEnv),
		ExportChoose:  choose,
	})
	if err != nil {
		log.Error("starting network", "err", err)
		os.Exit(1)
	}
	if err := home.Close(); err != nil {
		log.Warn("closing identity directory", "err", err)
	}
	router := bridge.NewRouter(rt.Repo, log)

	if err := proj.Backup(time.Now(), backupsKept); err != nil {
		log.Warn("backing up project", "err", err) // not fatal: the store still works
	}
	storeServer := bridge.NewStoreServer(rt.Files, log)

	projects := app.NewProjects(proj, rt.Files)

	wails = application.New(application.Options{
		Name:        "Inkwell",
		Description: "Éditeur collaboratif Typst, sans serveur",
		Services: []application.Service{
			application.NewService(rt.Session),
			application.NewService(projects),
			application.NewService(app.NewPreviewService(rt.Preview)),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(inkwell.Assets),
			Middleware: app.CSPNonce(),
		},
		// Used only by `-tags server` builds (headless end-to-end tests): bound to
		// loopback, port from INKWELL_SERVER_PORT.
		Server: application.ServerOptions{Host: "127.0.0.1", Port: serverPort()},
		// The id of the desktop entry and the icons (build/linux): the window manager matches
		// the window with them through this id (Wayland app_id, X11 WM_CLASS).
		Linux: application.LinuxOptions{ApplicationID: appID},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			cancel()
			if err := rt.Close(); err != nil { // before the project: it keeps files there
				log.Warn("closing network", "err", err)
			}
			if err := proj.Close(); err != nil {
				log.Warn("closing project", "err", err)
			}
		},
	})
	projects.SetOpener(wails.Browser.OpenFile, app.RevealFile(func(path string) error {
		return wails.Env.OpenFileManager(path, true)
	}))
	wails.HandleStream(bridge.StreamName, func(c *application.StreamConn) { router.Serve(c) })
	wails.HandleStream(bridge.StoreStreamName, func(c *application.StreamConn) { storeServer.Serve(c) })
	wails.HandleStream(app.PreviewStreamName, func(c *application.StreamConn) { rt.Preview.Serve(c) })

	go func() {
		if err := proj.Watch(ctx, func(rel string) {
			log.Info("project file edited outside Inkwell", "file", rel)
			storeServer.NotifyExternal(rel)
			rt.Preview.Changed(rel)
		}); err != nil {
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
