//go:build linux && !server

package main

import (
	"log/slog"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oioio-space/inkwell/internal/activate"
)

// newRaiser raises w through GTK with the launch's activation token (internal/activate): Wails'
// Show and Focus present the window without it, and GNOME then only says that it is ready. They
// remain the fallback when GTK cannot be reached.
func newRaiser(w *application.WebviewWindow) raiser {
	return raiser{invoke: application.InvokeAsync, window: w.NativeWindow, present: func(window unsafe.Pointer, token string) {
		if err := activate.Present(window, token); err != nil {
			slog.Warn("window presented without the launch's activation token", "err", err)
			w.Show()
			w.Focus()
		}
	}}
}
