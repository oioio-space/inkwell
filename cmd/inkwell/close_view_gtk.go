//go:build linux && !server && !gtk3

package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/cgo"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type gtkObject = unsafe.Pointer

const (
	gtkLibrary    = "libgtk-4.so.1"
	webkitLibrary = "libwebkitgtk-6.0.so.4"
	gFalse        = int32(0)
	gTrue         = int32(1)
	horizontal    = int32(0)
	vertical      = int32(1)
	alignFill     = int32(0)
	alignEnd      = int32(2)
)

type closeGTK struct {
	windowNew         func() gtkObject
	windowChild       func(gtkObject) gtkObject
	windowSetChild    func(gtkObject, gtkObject)
	windowTitle       func(gtkObject, string)
	windowSize        func(gtkObject, int32, int32)
	windowTransient   func(gtkObject, gtkObject)
	windowWithParent  func(gtkObject, int32)
	windowModal       func(gtkObject, int32)
	windowResizable   func(gtkObject, int32)
	windowFocus       func(gtkObject, gtkObject)
	windowDefault     func(gtkObject, gtkObject)
	windowPresent     func(gtkObject)
	windowDestroy     func(gtkObject)
	boxType           func() uintptr
	boxNew            func(int32, int32) gtkObject
	boxAppend         func(gtkObject, gtkObject)
	boxPrepend        func(gtkObject, gtkObject)
	boxRemove         func(gtkObject, gtkObject)
	labelNew          func(string) gtkObject
	labelWrap         func(gtkObject, int32)
	labelAlign        func(gtkObject, float32)
	spinnerNew        func() gtkObject
	spinnerStart      func(gtkObject)
	accessibleLabel   func(gtkObject, int32, string, int32)
	buttonNew         func(string) gtkObject
	firstChild        func(gtkObject) gtkObject
	nextSibling       func(gtkObject) gtkObject
	widgetClass       func(gtkObject, string)
	widgetAlign       func(gtkObject, int32)
	widgetSize        func(gtkObject, int32, int32)
	marginStart       func(gtkObject, int32)
	marginEnd         func(gtkObject, int32)
	marginTop         func(gtkObject, int32)
	marginBottom      func(gtkObject, int32)
	keyController     func() gtkObject
	addController     func(gtkObject, gtkObject)
	isA               func(gtkObject, uintptr) int32
	connect           func(gtkObject, string, uintptr, uintptr, uintptr, int32) uintptr
	webviewType       func() uintptr
	onDismiss         uintptr
	onRetry           uintptr
	onForce           uintptr
	onKey             uintptr
	onRelease         uintptr
	onRendererStopped uintptr
}

// GTK and WebKitGTK are already linked by Wails. As in internal/activate, use
// NOLOAD to resolve their public APIs without adding a C importer or a library.
func bindGTK(library string, calls map[string]any) error {
	const rtldNoLoad = 4
	handle, err := purego.Dlopen(library, purego.RTLD_NOW|purego.RTLD_LOCAL|rtldNoLoad)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", library, err)
	}
	defer func() { _ = purego.Dlclose(handle) }()
	for name, call := range calls {
		address, err := purego.Dlsym(handle, name)
		if err != nil {
			return fmt.Errorf("resolving GTK symbol %s: %w", name, err)
		}
		purego.RegisterFunc(call, address)
	}
	return nil
}

var nativeCloseGTK = sync.OnceValues(loadCloseGTK)

func loadCloseGTK() (*closeGTK, error) {
	g := &closeGTK{}
	if err := bindGTK(gtkLibrary, map[string]any{
		"gtk_window_new": &g.windowNew, "gtk_window_get_child": &g.windowChild,
		"gtk_window_set_child": &g.windowSetChild, "gtk_window_set_title": &g.windowTitle,
		"gtk_window_set_default_size": &g.windowSize, "gtk_window_set_transient_for": &g.windowTransient,
		"gtk_window_set_destroy_with_parent": &g.windowWithParent, "gtk_window_set_modal": &g.windowModal,
		"gtk_window_set_resizable": &g.windowResizable, "gtk_window_set_focus": &g.windowFocus,
		"gtk_window_set_default_widget": &g.windowDefault, "gtk_window_present": &g.windowPresent,
		"gtk_window_destroy": &g.windowDestroy,
		"gtk_box_get_type":   &g.boxType, "gtk_box_new": &g.boxNew,
		"gtk_box_append": &g.boxAppend, "gtk_box_prepend": &g.boxPrepend, "gtk_box_remove": &g.boxRemove,
		"gtk_label_new": &g.labelNew, "gtk_label_set_wrap": &g.labelWrap, "gtk_label_set_xalign": &g.labelAlign,
		"gtk_spinner_new": &g.spinnerNew, "gtk_spinner_start": &g.spinnerStart,
		"gtk_accessible_update_property": &g.accessibleLabel, "gtk_button_new_with_label": &g.buttonNew,
		"gtk_widget_get_first_child": &g.firstChild, "gtk_widget_get_next_sibling": &g.nextSibling,
		"gtk_widget_add_css_class": &g.widgetClass, "gtk_widget_set_halign": &g.widgetAlign,
		"gtk_widget_set_size_request": &g.widgetSize,
		"gtk_widget_set_margin_start": &g.marginStart, "gtk_widget_set_margin_end": &g.marginEnd,
		"gtk_widget_set_margin_top": &g.marginTop, "gtk_widget_set_margin_bottom": &g.marginBottom,
		"gtk_event_controller_key_new": &g.keyController, "gtk_widget_add_controller": &g.addController,
		"g_type_check_instance_is_a": &g.isA, "g_signal_connect_data": &g.connect,
	}); err != nil {
		return nil, err
	}
	if err := bindGTK(webkitLibrary, map[string]any{"webkit_web_view_get_type": &g.webviewType}); err != nil {
		return nil, err
	}

	g.onRelease = purego.NewCallback(func(handle uintptr, _ gtkObject) {
		cgo.Handle(handle).Delete()
	})
	g.onRendererStopped = purego.NewCallback(func(_ gtkObject, _ int32, handle uintptr) {
		cgo.Handle(handle).Value().(func())()
	})
	g.onDismiss = purego.NewCallback(func(_ gtkObject, handle uintptr) int32 {
		cgo.Handle(handle).Value().(func(int))(0)
		return gTrue
	})
	g.onRetry = purego.NewCallback(func(_ gtkObject, handle uintptr) {
		cgo.Handle(handle).Value().(func(int))(1)
	})
	g.onForce = purego.NewCallback(func(_ gtkObject, handle uintptr) {
		cgo.Handle(handle).Value().(func(int))(2)
	})
	g.onKey = purego.NewCallback(func(_ gtkObject, key, _, _ uint32, handle uintptr) int32 {
		const escape = 0xff1b
		if key != escape {
			return gFalse
		}
		cgo.Handle(handle).Value().(func(int))(0)
		return gTrue
	})
	return g, nil
}

func (g *closeGTK) layout(window gtkObject) gtkObject {
	child := g.windowChild(window)
	if child != nil && g.isA(child, g.boxType()) != 0 {
		return child
	}
	return nil
}

func (g *closeGTK) webview(window gtkObject) gtkObject {
	layout := g.layout(window)
	if layout == nil {
		return nil
	}
	for child := g.firstChild(layout); child != nil; child = g.nextSibling(child) {
		if g.isA(child, g.webviewType()) != 0 {
			return child
		}
	}
	return nil
}

func (g *closeGTK) showSaving(window gtkObject) gtkObject {
	layout := g.layout(window)
	if layout == nil {
		return nil
	}
	banner := g.boxNew(horizontal, 8)
	g.widgetClass(banner, "toolbar")
	g.widgetAlign(banner, alignFill)
	spinner := g.spinnerNew()
	const gtkAccessiblePropertyLabel = 4
	g.accessibleLabel(spinner, gtkAccessiblePropertyLabel, savingLabel, -1)
	g.spinnerStart(spinner)
	g.boxAppend(banner, spinner)
	g.boxAppend(banner, g.labelNew(savingLabel))
	g.boxPrepend(layout, banner)
	return banner
}

func (g *closeGTK) showNotice(window gtkObject, view closeView, click func(int)) gtkObject {
	const title = "Enregistrement interrompu"
	notice := g.windowNew()
	g.windowTitle(notice, title)
	g.windowTransient(notice, window)
	g.windowWithParent(notice, gTrue)
	g.windowModal(notice, gTrue)
	g.windowResizable(notice, gFalse)
	g.windowSize(notice, 420, -1)

	body := g.boxNew(vertical, 16)
	g.marginStart(body, 24)
	g.marginEnd(body, 24)
	g.marginTop(body, 24)
	g.marginBottom(body, 24)
	g.windowSetChild(notice, body)
	heading := g.labelNew(title)
	g.widgetClass(heading, "title-2")
	g.boxAppend(body, heading)
	text := g.labelNew(view.message())
	g.labelWrap(text, gTrue)
	g.labelAlign(text, 0)
	g.boxAppend(body, text)
	buttons := g.boxNew(horizontal, 8)
	g.widgetAlign(buttons, alignEnd)
	g.boxAppend(body, buttons)
	force := g.buttonNew(forceLabel)
	g.widgetClass(force, "destructive-action")
	g.widgetSize(force, -1, 44)
	g.boxAppend(buttons, force)
	retry := g.buttonNew(retryLabel)
	g.widgetClass(retry, "suggested-action")
	g.widgetSize(retry, -1, 44)
	g.boxAppend(buttons, retry)

	handle := uintptr(cgo.NewHandle(click))
	g.connect(notice, "close-request", g.onDismiss, handle, g.onRelease, 0)
	g.connect(retry, "clicked", g.onRetry, handle, 0, 0)
	g.connect(force, "clicked", g.onForce, handle, 0, 0)
	keys := g.keyController()
	g.connect(keys, "key-pressed", g.onKey, handle, 0, 0)
	g.addController(notice, keys)
	g.windowDefault(notice, retry)
	g.windowFocus(notice, retry)
	g.windowPresent(notice)
	return notice
}

func watchCloseView(ctx context.Context, app *application.App, window *application.WebviewWindow, closer *closeCoordinator) {
	g, err := nativeCloseGTK()
	if err != nil {
		slog.Warn("loading the native close view", "err", err)
		watchWailsCloseDialog(ctx, app, window, closer)
		return
	}
	var progress, notice gtkObject
	var watching bool
	observe := func() {
		if watching || window.NativeWindow() == nil {
			return
		}
		if webview := g.webview(window.NativeWindow()); webview != nil {
			handle := cgo.NewHandle(func() { closer.rendererCrashed(window.ID()) })
			watching = g.connect(webview, "web-process-terminated", g.onRendererStopped, uintptr(handle), g.onRelease, 0) != 0
			if !watching {
				handle.Delete()
			}
		}
	}
	window.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		application.InvokeAsync(observe)
	})
	go closer.watchView(ctx, window.ID(), func(view closeView) {
		application.InvokeAsync(func() {
			parent := window.NativeWindow()
			if parent == nil {
				return
			}
			observe()
			if notice != nil {
				g.windowDestroy(notice)
				notice = nil
			}
			if layout := g.layout(parent); layout != nil && progress != nil {
				g.boxRemove(layout, progress)
				progress = nil
			}
			if view.attempt == 0 {
				return
			}
			if view.failure.Kind == "" {
				progress = g.showSaving(parent)
				return
			}
			notice = g.showNotice(parent, view, func(response int) {
				switch response {
				case 1:
					closer.retry(view.attempt)
				case 2:
					closer.force(view.attempt)
				default:
					closer.dismiss(view.attempt)
				}
			})
		})
	})
}
