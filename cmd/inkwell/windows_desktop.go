//go:build !server

package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/oioio-space/inkwell/internal/app"
)

// publicWindowName names the window of the public view.
const publicWindowName = "inkwell-public"

// newWindowHost returns the desktop's windows, for the presenter view in the
// main window; [windowHost.bind] gives it the application and that window once
// they exist. Placement is the window manager's to honour: Wayland lets no
// program position its windows, so there the person drags the public window
// themselves (and "Échanger les écrans" does not change where it lands).
func newWindowHost() windowHost { return &wailsWindows{} }

func (w *wailsWindows) bind(wails *application.App, main *application.WebviewWindow) {
	w.app, w.main = wails, main
}

type wailsWindows struct {
	app  *application.App
	main *application.WebviewWindow

	mu         sync.Mutex
	public     *application.WebviewWindow
	fullscreen bool // how the public window was created
}

var _ app.Windows = (*wailsWindows)(nil)

func (w *wailsWindows) Screens() []app.Screen {
	var out []app.Screen
	for _, s := range w.app.Screen.GetAll() {
		out = append(out, app.Screen{
			ID: s.ID, Name: s.Name, Primary: s.IsPrimary,
			X: s.Bounds.X, Y: s.Bounds.Y, Width: s.Bounds.Width, Height: s.Bounds.Height,
		})
	}
	return out
}

func (w *wailsWindows) OpenPublic(p app.Placement) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	screen := w.app.Screen.GetByID(p.Screen)
	if w.public != nil && w.fullscreen != p.Fullscreen {
		w.public.Close() // frame and fullscreen are set at creation: start over
		w.public = nil
	}
	if w.public != nil {
		w.public.UnFullscreen()
		if screen != nil {
			w.public.SetScreen(screen)
		}
		if p.Fullscreen {
			w.public.Fullscreen()
		}
		return nil
	}
	opts := application.WebviewWindowOptions{
		Name:            publicWindowName,
		Title:           "Inkwell",
		URL:             "/?view=public",
		Width:           p.Width,
		Height:          p.Height,
		MinWidth:        320,
		MinHeight:       180,
		Screen:          screen,
		InitialPosition: application.WindowCentered,
	}
	if p.Fullscreen {
		opts.Frameless = true
		opts.StartState = application.WindowStateFullscreen
		opts.Mac.DisableEscapeExitsFullscreen = true // Esc is a key of the presentation (macOS)
	}
	win := w.app.Window.NewWithOptions(opts)
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.public == win {
			w.public = nil
		}
	})
	w.public, w.fullscreen = win, p.Fullscreen
	return nil
}

func (w *wailsWindows) PlacePresenter(p app.Placement) error {
	if screen := w.app.Screen.GetByID(p.Screen); screen != nil {
		w.main.SetScreen(screen)
		w.main.Maximise()
	}
	return nil
}

func (w *wailsWindows) ClosePublic() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.public != nil {
		w.public.Close()
		w.public = nil
	}
	return nil
}
