// Command inkwell is the desktop application: it opens the editor window and
// wires the webview to the network through the bridge (docs/architecture.md).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/oioio-space/inkwell"
	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/instance"
	"github.com/oioio-space/inkwell/internal/language"
	"github.com/oioio-space/inkwell/internal/launch"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/transport"
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

// run is the launch of Inkwell; configure, when set, adjusts the application before it runs (the
// native performance build). It exits with the code of [startup] when that is not 0.
func run(configure func(*application.App, *application.WebviewWindow)) {
	if code := startup(os.Args[1:], liveDeps(configure)); code != exitOK {
		os.Exit(code)
	}
}

// liveDeps are the real effects of a launch.
func liveDeps(configure func(*application.App, *application.WebviewWindow)) deps {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	var wails atomic.Pointer[application.App] // the export dialog needs the application, created last
	return deps{
		stdout:   os.Stdout,
		log:      log,
		getenv:   startEnv(), // the activation token survives GTK there
		unsetenv: os.Unsetenv,
		getwd:    os.Getwd,
		homeDir:  app.HomeDir,
		start: func(ctx context.Context, dir string, req instance.Request) (*instance.Primary, error) {
			return instance.Start(ctx, dir, req, instance.Options{})
		},
		notify:      tell,
		openHome:    app.OpenHome,
		openProject: openProject,
		startNetwork: func(ctx context.Context, home *os.Root, proj *project.Project, afterChange bool) (*app.Runtime, error) {
			choose := func(name string) (string, error) {
				return wails.Load().Dialog.SaveFile().SetFilename(name).AddFilter("PDF", "*.pdf").PromptForSingleSelection()
			}
			if dir := os.Getenv(exportDirEnv); dir != "" {
				choose = app.DirChooser(dir)
			}
			return app.Start(ctx, home, app.Options{
				Log:           log,
				Project:       proj,
				Port:          app.DefaultPort,
				LAN:           os.Getenv(lanEnv) == "1",
				RendezvousURL: os.Getenv(rendezvousEnv),
				ExportChoose:  choose,
				AfterChange:   afterChange,
				CanRestart:    canRestart,
			})
		},
		window: func(ctx context.Context, s *session) int {
			return runWindow(ctx, s, log, &wails, configure)
		},
		restartRequested: (*app.Runtime).RestartRequested,
		restart:          restartImage,
	}
}

// runWindow serves the webview of the open project until the window closes.
func runWindow(ctx context.Context, s *session, log *slog.Logger, wailsApp *atomic.Pointer[application.App], configure func(*application.App, *application.WebviewWindow)) int {
	proj, rt := s.proj, s.rt
	router := bridge.NewRouter(rt.Repo, log)
	router.UseDrain(rt.Bridge)

	if err := proj.Backup(time.Now(), backupsKept); err != nil {
		log.Warn("backing up project", "err", err) // not fatal: the store still works
	}
	storeServer := bridge.NewStoreServer(rt.Files, rt.Files, rt.Files, log)
	storeServer.UseDrain(rt.Bridge)
	windows := newWindowHost() // nil without native windows (the server build)
	var winService app.Windows
	if windows != nil {
		winService = windows
	}
	presentHub := app.NewPresent(log)
	languageService := s.language

	loops := &s.loops
	assetToken := app.NewAssetToken()
	wails := application.New(application.Options{
		Name:        "Inkwell",
		Description: "Éditeur collaboratif Typst, sans serveur",
		Services: []application.Service{
			application.NewService(rt.Session),
			application.NewService(app.NewProjects(proj, rt.Files)),
			application.NewService(app.NewPreviewService(rt.Preview)),
			application.NewService(app.NewPresentService(winService)),
		},
		Assets: application.AssetOptions{
			Handler: app.ProjectImages(proj, assetToken, application.AssetFileServerFS(inkwell.Assets)),
			Middleware: func(next http.Handler) http.Handler {
				return app.CSPNonce()(app.PageToken(assetToken)(next))
			},
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
		OnShutdown: func() { s.stop(log) },
	})
	wailsApp.Store(wails)
	rt.SetQuit(wails.Quit) // a recorded project change quits, then restarts (rule N-12)
	wails.HandleStream(bridge.StreamName, func(c *application.StreamConn) { router.Serve(c) })
	wails.HandleStream(bridge.StoreStreamName, func(c *application.StreamConn) { storeServer.Serve(c) })
	wails.HandleStream(app.PreviewStreamName, func(c *application.StreamConn) { rt.Preview.Serve(c) })
	wails.HandleStream(language.StreamName, func(c *application.StreamConn) { languageService.Serve(c) })
	wails.HandleStream(app.PresentStreamName, func(c *application.StreamConn) { presentHub.Serve(c) })

	loops.Go("project watch", func() {
		if err := proj.Watch(ctx, func(rel string) {
			log.Info("project file edited outside Inkwell", "file", rel)
			storeServer.NotifyExternal(rel)
			rt.Preview.Changed(rel)
		}); err != nil {
			log.Error("watching project files", "err", err)
		}
	})

	loops.Go("bridge", func() {
		// Run only returns once ctx is cancelled or the network is closed; anything
		// else means the bridge died while the app is still running.
		if err := router.Run(ctx); ctx.Err() == nil && !errors.Is(err, transport.ErrClosed) {
			log.Error("bridge stopped", "err", err)
		}
	})

	main := wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Inkwell",
		Width:     1280,
		Height:    800,
		MinWidth:  720,
		MinHeight: 480,
		URL:       "/",
	})
	if windows != nil {
		windows.bind(wails, main)
	}
	var closingWindow atomic.Bool // a closing window is not raised again
	mainRaiser := newRaiser(main)
	raise := func(token string) {
		if !closingWindow.Load() {
			mainRaiser.raise(token)
		}
	}
	open := &opener{queue: s.queue, env: openEnv(proj.Path()), raise: raise, log: log}
	wails.HandleStream(openStreamName, func(c *application.StreamConn) { open.Serve(c) })
	// Closing the window: stop acknowledging, and wait (bounded) until the webview reported handled
	// what was acknowledged (rule D-DRAIN at close). The raise is asynchronous: it never waits on
	// this hook.
	main.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		closingWindow.Store(true)
		drainOnClose(s.queue, s.closing, drainTimeout, log)
	})

	if configure != nil {
		configure(wails, main)
	}
	if err := wails.Run(); err != nil {
		log.Error("inkwell stopped", "err", err)
		return exitError
	}
	return exitOK
}

// openEnv is where the open requests are resolved: the open project dir, its Docs folder (every
// project opened is a folder of Docs) and the person's home.
func openEnv(dir string) launch.Env {
	home, _ := os.UserHomeDir() // "": the walk to a project then stops only at / or a mount point
	return launch.Env{Home: home, Docs: filepath.Dir(dir), Current: dir}
}

// windowHost is the desktop's windows for a presentation, which can only be
// bound to the application once it and its main window exist. The server build
// has none.
type windowHost interface {
	app.Windows
	bind(*application.App, *application.WebviewWindow)
}

// backupsKept is how many project backups are kept (one per launch).
const backupsKept = 10

// openProject opens the project of the first argument of this launch when it is a project of the
// user's Inkwell folder (R2), else the most recent one, creating an untitled one on first launch
// (R1).
func openProject(own instance.Request) (*project.Project, error) {
	docs, err := app.OpenDocs()
	if err != nil {
		return nil, err
	}
	defer func() { _ = docs.Close() }() // the project keeps its own handle
	home, _ := os.UserHomeDir()
	env := launch.Env{Cwd: own.Cwd, Home: home, Docs: docs.Name()}
	return app.OpenProjectIn(docs, launch.Startup(launch.Resolve(own.Args, int(own.Dropped), env), docs.Name()))
}

// serverPort returns INKWELL_SERVER_PORT, or 8080 when unset or invalid.
func serverPort() int {
	if p, err := strconv.Atoi(os.Getenv("INKWELL_SERVER_PORT")); err == nil && p > 0 && p < 65536 {
		return p
	}
	return 8080
}

// shutdownLoopTimeout bounds shutdown when a loop is stuck sending to a dead webview.
const shutdownLoopTimeout = 5 * time.Second

type shutdownLoop struct {
	name string
	done <-chan struct{}
}

type shutdownLoops []shutdownLoop

// Go registers loops before shutdown starts, retaining their names for diagnostics.
func (loops *shutdownLoops) Go(name string, run func()) {
	done := make(chan struct{})
	*loops = append(*loops, shutdownLoop{name: name, done: done})
	go func() {
		defer close(done)
		run()
	}()
}

// shutdown gives loops a bounded time to finish before closing their resources.
func shutdown(cancel context.CancelFunc, loops *shutdownLoops, closeNetwork, closeProject func() error, log *slog.Logger) {
	cancel()
	timer := time.NewTimer(shutdownLoopTimeout)
	defer timer.Stop()
wait:
	for _, loop := range *loops {
		select {
		case <-loop.done:
		case <-timer.C:
			for _, pending := range *loops {
				select {
				case <-pending.done:
				default:
					log.Warn("shutdown loop did not stop", "loop", pending.name)
				}
			}
			break wait
		}
	}
	if err := closeNetwork(); err != nil {
		log.Warn("closing network", "err", err)
	}
	if err := closeProject(); err != nil {
		log.Warn("closing project", "err", err)
	}
}
