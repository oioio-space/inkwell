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

// newWindowHost returns the desktop's windows: an ordinary window for the audience, which the
// person drags to the screen they want and makes full screen there (F11 or the button in it).
// Inkwell does not guess screens.
func newWindowHost() windowHost { return &wailsWindows{} }

func (w *wailsWindows) bind(wails *application.App, _ *application.WebviewWindow) { w.app = wails }

type wailsWindows struct {
	app    *application.App
	mu     sync.Mutex
	public *application.WebviewWindow
}

var _ app.Windows = (*wailsWindows)(nil)

func (w *wailsWindows) OpenPublic() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.public != nil {
		w.public.Focus()
		return nil
	}
	win := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            publicWindowName,
		Title:           "Inkwell",
		URL:             "/?view=public",
		Width:           960,
		Height:          540,
		MinWidth:        320,
		MinHeight:       180,
		InitialPosition: application.WindowCentered,
	})
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.public == win {
			w.public = nil
		}
	})
	w.public = win
	return nil
}

func (w *wailsWindows) ClosePublic() error {
	// Closing runs the closing hook, which takes the lock: take the window out first, close it after.
	w.mu.Lock()
	win := w.public
	w.public = nil
	w.mu.Unlock()
	if win != nil {
		win.Close()
	}
	return nil
}
