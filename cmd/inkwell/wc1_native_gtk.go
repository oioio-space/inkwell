//go:build linux && !server && !gtk3 && wc1native

package main

import (
	"runtime/cgo"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// These calls let the acceptance test observe GTK independently of Wails'
// window event processing and inspect the widgets the person uses.
type witnessGTK struct {
	*closeGTK
	windowType      func() uintptr
	windowActive    func(gtkObject) int32
	windowFocus     func(gtkObject) gtkObject
	windowDefault   func(gtkObject) gtkObject
	windowTransient func(gtkObject) gtkObject
	windowList      func() gtkObject
	widgetMapped    func(gtkObject) int32
	widgetHasClass  func(gtkObject, string) int32
	labelType       func() uintptr
	labelText       func(gtkObject) string
	buttonType      func() uintptr
	buttonLabel     func(gtkObject) string
	spinnerType     func() uintptr
	spinnerActive   func(gtkObject) int32
	accessibleRole  func(gtkObject) int32
	listSize        func(gtkObject) uint32
	listItem        func(gtkObject, uint32) gtkObject
	unref           func(gtkObject)
	valueObject     func(gtkObject) gtkObject
	signalID        func(string, uintptr) uint32
	emissionHook    func(uint32, uint32, uintptr, uintptr, uintptr) uintptr
	removeHook      func(uint32, uintptr)
	terminate       func(gtkObject)
	closeRequest    uintptr
	unrealize       uintptr
	itemsChanged    uintptr
}

type windowWitness struct {
	window              gtkObject
	signal              uint32
	emission            uintptr
	removed, unrealized bool
	event               func(string)
}

var nativeWitnessGTK = sync.OnceValues(loadWitnessGTK)

func loadWitnessGTK() (*witnessGTK, error) {
	base, err := nativeCloseGTK()
	if err != nil {
		return nil, err
	}
	g := &witnessGTK{closeGTK: base}
	if err := bindGTK(gtkLibrary, map[string]any{
		"gtk_window_get_type": &g.windowType, "gtk_window_is_active": &g.windowActive,
		"gtk_window_get_focus": &g.windowFocus, "gtk_window_get_default_widget": &g.windowDefault,
		"gtk_window_get_transient_for": &g.windowTransient, "gtk_window_get_toplevels": &g.windowList,
		"gtk_widget_get_mapped": &g.widgetMapped, "gtk_widget_has_css_class": &g.widgetHasClass,
		"gtk_label_get_type": &g.labelType, "gtk_label_get_text": &g.labelText,
		"gtk_button_get_type": &g.buttonType, "gtk_button_get_label": &g.buttonLabel,
		"gtk_spinner_get_type": &g.spinnerType, "gtk_spinner_get_spinning": &g.spinnerActive,
		"gtk_accessible_get_accessible_role": &g.accessibleRole,
		"g_list_model_get_n_items":           &g.listSize, "g_list_model_get_item": &g.listItem,
		"g_object_unref": &g.unref, "g_value_get_object": &g.valueObject,
		"g_signal_lookup": &g.signalID, "g_signal_add_emission_hook": &g.emissionHook,
		"g_signal_remove_emission_hook": &g.removeHook,
	}); err != nil {
		return nil, err
	}
	if err := bindGTK(webkitLibrary, map[string]any{"webkit_web_view_terminate_web_process": &g.terminate}); err != nil {
		return nil, err
	}
	g.closeRequest = purego.NewCallback(func(_ gtkObject, count uint32, values gtkObject, data uintptr) int32 {
		w := cgo.Handle(data).Value().(*windowWitness)
		if count > 0 && g.valueObject(values) == w.window {
			w.event("gtk-close-request")
		}
		return gTrue
	})
	g.unrealize = purego.NewCallback(func(_ gtkObject, data uintptr) {
		w := cgo.Handle(data).Value().(*windowWitness)
		w.unrealized = true
		if w.removed {
			w.event("gtk-window-destroyed")
		}
	})
	g.itemsChanged = purego.NewCallback(func(model gtkObject, _, _, _ uint32, data uintptr) {
		w := cgo.Handle(data).Value().(*windowWitness)
		if w.removed || g.isToplevel(model, w.window) {
			return
		}
		w.removed = true
		g.removeHook(w.signal, w.emission)
		if w.unrealized {
			w.event("gtk-window-destroyed")
		}
	})
	return g, nil
}

func (g *witnessGTK) isToplevel(model, window gtkObject) bool {
	for i := range g.listSize(model) {
		item := g.listItem(model, i)
		found := item == window
		g.unref(item)
		if found {
			return true
		}
	}
	return false
}

func nativeWitness(window *application.WebviewWindow, event func(string)) bool {
	g, err := nativeWitnessGTK()
	if err != nil {
		return false
	}
	w := &windowWitness{window: window.NativeWindow(), event: event}
	if w.window == nil || g.isA(w.window, g.windowType()) == 0 {
		return false
	}
	w.signal = g.signalID("close-request", g.windowType())
	handle := cgo.NewHandle(w)
	w.emission = g.emissionHook(w.signal, 0, g.closeRequest, uintptr(handle), 0)
	if w.emission == 0 {
		handle.Delete()
		return false
	}
	g.connect(w.window, "unrealize", g.unrealize, uintptr(handle), 0, 0)
	g.connect(g.windowList(), "items-changed", g.itemsChanged, uintptr(handle), 0, 0)
	return true
}

func (g *witnessGTK) findButton(widget gtkObject, name string) gtkObject {
	if g.isA(widget, g.buttonType()) != 0 && g.buttonLabel(widget) == name {
		return widget
	}
	for child := g.firstChild(widget); child != nil; child = g.nextSibling(child) {
		if button := g.findButton(child, name); button != nil {
			return button
		}
	}
	return nil
}

func (g *witnessGTK) saving(parent gtkObject) bool {
	banner := g.firstChild(g.windowChild(parent))
	if banner == nil || g.isA(banner, g.boxType()) == 0 {
		return false
	}
	spinner := g.firstChild(banner)
	if spinner == nil || g.isA(spinner, g.spinnerType()) == 0 || g.spinnerActive(spinner) == 0 || g.widgetMapped(spinner) == 0 {
		return false
	}
	label := g.nextSibling(spinner)
	return label != nil && g.isA(label, g.labelType()) != 0 && g.labelText(label) == savingLabel
}

func (g *witnessGTK) noticeStatus(dialog gtkObject) int {
	retry, force := g.findButton(dialog, retryLabel), g.findButton(dialog, forceLabel)
	if retry == nil || force == nil {
		return 0
	}
	var status int
	const gtkAccessibleRoleButton = 3
	if g.windowDefault(dialog) == retry && g.widgetHasClass(force, "destructive-action") != 0 &&
		g.accessibleRole(force) == gtkAccessibleRoleButton {
		status |= 4
	}
	switch g.windowFocus(dialog) {
	case retry:
		status |= 8
	case force:
		status |= 16
	}
	return status
}

func nativeViewStatus(window *application.WebviewWindow) int {
	g, err := nativeWitnessGTK()
	if err != nil || window.NativeWindow() == nil {
		return 0
	}
	parent := window.NativeWindow()
	var status int
	if g.windowActive(parent) != 0 {
		status |= 1
	}
	if g.saving(parent) {
		status |= 2
	}
	windows := g.windowList()
	for i := range g.listSize(windows) {
		dialog := g.listItem(windows, i)
		if g.windowTransient(dialog) == parent && g.widgetMapped(dialog) != 0 {
			status |= g.noticeStatus(dialog)
		}
		g.unref(dialog)
	}
	return status
}

func nativeTerminateRenderer(window *application.WebviewWindow) bool {
	g, err := nativeWitnessGTK()
	if err != nil || window.NativeWindow() == nil {
		return false
	}
	view := g.webview(window.NativeWindow())
	if view == nil {
		return false
	}
	g.terminate(view)
	return true
}
