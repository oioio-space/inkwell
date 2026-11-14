//go:build !linux && !server

package main

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// newRaiser raises w with Wails' own calls: the launch's activation token is a notion of the Linux
// desktops (Windows and macOS are block N35).
func newRaiser(w *application.WebviewWindow) raiser {
	return raiser{invoke: application.InvokeAsync, window: w.NativeWindow, present: func(unsafe.Pointer, string) {
		w.Restore()
		w.Show()
		w.Focus()
	}}
}
