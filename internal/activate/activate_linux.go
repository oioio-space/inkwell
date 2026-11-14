//go:build linux && !server

package activate

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// gtkLibrary is the GTK 4 the application is linked with (Wails loads it at start).
const gtkLibrary = "libgtk-4.so.1"

// rtldNoLoad is RTLD_NOLOAD (glibc and musl): dlopen answers only for a library the process has
// already loaded, so Present can never bring a second GTK into the process.
const rtldNoLoad = 0x4

// toolkit holds the two GTK calls Present makes.
type toolkit struct {
	setStartupID func(window unsafe.Pointer, id string)
	present      func(window unsafe.Pointer)
}

// loaded is the process's GTK, looked up once.
var loaded = sync.OnceValues(func() (*toolkit, error) { return load(rtldNoLoad) })

// load finds the two calls in GTK. It reports a library that is absent and a call it lacks
// instead of failing later, in the call.
func load(flags int) (*toolkit, error) {
	lib, err := purego.Dlopen(gtkLibrary, purego.RTLD_NOW|purego.RTLD_GLOBAL|flags)
	if err != nil {
		return nil, fmt.Errorf("activate: %s: %w", gtkLibrary, err)
	}
	t := &toolkit{}
	for name, call := range map[string]any{
		"gtk_window_set_startup_id": &t.setStartupID,
		"gtk_window_present":        &t.present,
	} {
		addr, err := purego.Dlsym(lib, name)
		if err != nil {
			return nil, fmt.Errorf("activate: %s: %w", name, err)
		}
		purego.RegisterFunc(call, addr)
	}
	return t, nil
}

// Present brings the GTK window to the front as the launch whose activation token is token; ""
// when the launcher gave none, and the desktop then decides alone (GNOME tells the person that
// the window is ready). window is a GtkWindow* (Wails' WebviewWindow.NativeWindow); nil does
// nothing. It must be called on the GTK main thread. An error means that GTK could not be
// reached and that nothing was done: the caller presents the window its own way.
//
// The token goes to GTK before the window is presented, as gtk_window_set_startup_id asks: GTK
// hands it to the compositor (xdg_activation_v1.activate on Wayland; on X11 it takes the time of
// the launch from it and ends the startup sequence), which raises the window, minimised or not.
func Present(window unsafe.Pointer, token string) error {
	if window == nil {
		return nil
	}
	gtk, err := loaded()
	if err != nil {
		return err
	}
	gtk.presentAs(window, token)
	return nil
}

// presentAs gives the token to GTK, when there is one, and only then presents the window.
func (t *toolkit) presentAs(window unsafe.Pointer, token string) {
	if token != "" {
		t.setStartupID(window, token)
	}
	t.present(window)
}
