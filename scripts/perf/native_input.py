"""External Mutter keyboard driver with X11 target verification for native perf.

This driver collects raw evidence. It does not certify paint, infer readiness from
screenshots, or declare a performance gate passed. Run only on the native host,
under heavy.sh --exclusive. The libraries are platform libraries, not subprocesses.
"""
import argparse
import ctypes as c
import hashlib
import json
import os
from pathlib import Path
import secrets
import selectors
import subprocess
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[2]
PREFIX = b"INKWELL_NATIVE_PERF "
CONTROLS = ["handler", "layout", "early-raf", "measure", "document", "task", "deferred"]
NULL_COUNT = 30  # per arm: recording hooks on ("null") and off ("null-off")


def null_schedule():
    """Interleaved NULL arms in ABBA blocks (on, off, off, on), so a linear host drift weighs on
    both arms equally. Mirrors nullTrialKind in internal/nativeperf."""
    return ["null" if k % 4 in (0, 3) else "null-off" for k in range(2 * NULL_COUNT)]
FOCUS_INTERRUPTED = ("Focus interrompu : autre fenêtre ou dialogue GNOME d’autorisation "
                     "d’interaction distante/XTest. Autorisez l’interaction si nécessaire, puis relancez.")


def initialized_monitor_display(gtk):
    """Initialize the driver's GTK, independently of GTK in the Wails child.

    Called on the driver's main thread only after a managed target XID exists.
    The returned default display is borrowed: never close or unref it per query.
    """
    if threading.current_thread() is not threading.main_thread():
        raise RuntimeError("GdkMonitor query requires the GTK main thread")
    gtk.gtk_is_initialized.argtypes = []
    gtk.gtk_is_initialized.restype = c.c_int
    gtk.gtk_init_check.argtypes = []
    gtk.gtk_init_check.restype = c.c_int
    gtk.gdk_set_allowed_backends.argtypes = [c.c_char_p]
    gtk.gdk_display_get_default.argtypes = []
    gtk.gdk_display_get_default.restype = c.c_void_p
    if not gtk.gtk_is_initialized():
        gtk.gdk_set_allowed_backends(b"x11")
        if not gtk.gtk_init_check():
            raise RuntimeError("GdkMonitor GTK initialization failed: X11 display unavailable")
    # Check the postcondition before any GDK display/monitor query. A failed
    # initialization must produce evidence, not a fatal GDK process abort.
    if not gtk.gtk_is_initialized():
        raise RuntimeError("GdkMonitor query refused before gtk_init()")
    display = gtk.gdk_display_get_default()
    if not display:
        raise RuntimeError("GdkMonitor initialized X11 display unavailable")
    return display


def validate_completion(marker, protocol, run_id, raw_path):
    if marker.get("runId") != run_id or marker.get("rawPath") != str(raw_path):
        raise ValueError("Mismatched native completion identity")
    if marker.get("status") not in ("collected", "fail"):
        raise ValueError("Unknown native completion status")
    if marker["status"] == "collected" and not protocol.finished:
        raise ValueError("Premature native success before all expected inputs")


def validate_activation(marker, run_id, pid):
    if marker.get("runId") != run_id or type(marker.get("pid")) is not int or marker["pid"] != pid:
        raise ValueError("Mismatched native activation identity")
    if marker.get("backend") != "x11":
        raise RuntimeError("Injection target is not an X11 client; refusing XTest")


class TrialProtocol:
    """One outstanding, attested trial at a time; malformed markers fail closed."""
    def __init__(self, run_id, phase, count, inject):
        if phase not in ("cold", "warm") or type(count) is not int or (phase == "cold" and count != 1) or (phase == "warm" and count < 30):
            raise ValueError("invalid native input phase/count")
        self.run_id, self.phase, self.inject = run_id, phase, inject
        self.controls = ["none"] if phase == "cold" else CONTROLS + ["none"] * count + null_schedule()
        self.index, self.pending = 0, False

    @property
    def finished(self):
        return self.index == len(self.controls) and not self.pending

    def consume(self, marker):
        if self.finished:
            raise ValueError("native trial after expected final sample")
        expected = dict(runId=self.run_id, id=self.index + 1, phase=self.phase,
                        control=self.controls[self.index])
        if type(marker.get("id")) is not int or any(marker.get(key) != value for key, value in expected.items()):
            raise ValueError("native marker does not match the outstanding run/trial")
        if marker.get("type") == "armed":
            if self.pending:
                raise ValueError("duplicate arm before sample acknowledgement")
            if marker.get("visible") is not True or marker.get("focused") is not True or marker.get("engine") != "WebKitGTK":
                raise ValueError("native input requires a focused visible WebKitGTK window")
            self.inject()
            self.pending = True
        elif marker.get("type") == "sample":
            if not self.pending:
                raise ValueError("native sample without armed input")
            self.pending = False
            self.index += 1
        else:
            raise ValueError("unknown native trial marker")


class FocusedKeyboard:
    def __init__(self, backend):
        self.backend = backend

    def press_for(self, pid):
        if hasattr(self.backend, "validate_target"):
            self.backend.validate_target(pid)
        # Check the actual X11 focus independently of the application attestation.
        if self.backend.focus_pid() != pid:
            raise ValueError(FOCUS_INTERRUPTED + " Aucune touche injectée.")
        self.backend.send_key("x")
        if self.backend.focus_pid() != pid:
            raise ValueError(FOCUS_INTERRUPTED + " Focus modifié pendant l’injection.")
        if hasattr(self.backend, "validate_target"):
            self.backend.validate_target(pid)

    def activate_for(self, pid):
        target = self.backend.activate(pid)
        if self.backend.focus_pid() != pid:
            raise ValueError("native X11 window activation did not establish focus")
        if hasattr(self.backend, "validate_target"):
            self.backend.validate_target(pid)
        return target

    def probe_for(self, pid):
        if hasattr(self.backend, "validate_target"):
            self.backend.validate_target(pid)
        if self.backend.focus_pid() != pid:
            raise ValueError(FOCUS_INTERRUPTED + " XTest permission probe not injected.")
        self.backend.send_keysym(0xffe1)  # Shift_L: no text edit, cold first input preserved.
        if self.backend.focus_pid() != pid:
            raise ValueError(FOCUS_INTERRUPTED + " Focus modifié pendant la preuve de saisie.")
        if hasattr(self.backend, "validate_target"):
            self.backend.validate_target(pid)


def select_monitor(monitors, x, y):
    matches = [m for m in monitors if m["x"] <= x < m["x"] + m["width"] and m["y"] <= y < m["y"] + m["height"]]
    if len(matches) != 1 or matches[0]["refreshHz"] <= 0:
        raise RuntimeError("Target monitor refresh unavailable or ambiguous; refusing cadence inference")
    return matches[0]


class X11Keyboard:
    def __init__(self, *, xtest=True):
        self.permission_verified = False
        try:
            self.x = c.CDLL("libX11.so.6")
            if xtest:
                self.xt = c.CDLL("libXtst.so.6")
        except OSError as error:
            raise RuntimeError("Native X11 input library unavailable; measurement failed") from error
        self.x.XOpenDisplay.argtypes = [c.c_char_p]
        self.x.XOpenDisplay.restype = c.c_void_p
        self.x.XCloseDisplay.argtypes = [c.c_void_p]
        self.x.XGetInputFocus.argtypes = [c.c_void_p, c.POINTER(c.c_ulong), c.POINTER(c.c_int)]
        self.x.XSetInputFocus.argtypes = [c.c_void_p, c.c_ulong, c.c_int, c.c_ulong]
        self.x.XQueryTree.argtypes = [c.c_void_p, c.c_ulong, c.POINTER(c.c_ulong), c.POINTER(c.c_ulong), c.POINTER(c.POINTER(c.c_ulong)), c.POINTER(c.c_uint)]
        self.x.XInternAtom.argtypes = [c.c_void_p, c.c_char_p, c.c_int]
        self.x.XInternAtom.restype = c.c_ulong
        self.x.XGetWindowProperty.argtypes = [c.c_void_p, c.c_ulong, c.c_ulong, c.c_long, c.c_long, c.c_int, c.c_ulong, c.POINTER(c.c_ulong), c.POINTER(c.c_int), c.POINTER(c.c_ulong), c.POINTER(c.c_ulong), c.POINTER(c.POINTER(c.c_ubyte))]
        self.x.XKeysymToKeycode.argtypes = [c.c_void_p, c.c_ulong]
        self.x.XKeysymToKeycode.restype = c.c_ubyte
        self.x.XFlush.argtypes = [c.c_void_p]
        self.x.XSync.argtypes = [c.c_void_p, c.c_int]
        self.x.XFree.argtypes = [c.c_void_p]
        if xtest:
            self.xt.XTestQueryExtension.argtypes = [c.c_void_p, c.POINTER(c.c_int), c.POINTER(c.c_int), c.POINTER(c.c_int), c.POINTER(c.c_int)]
            self.xt.XTestFakeKeyEvent.argtypes = [c.c_void_p, c.c_uint, c.c_int, c.c_ulong]
        self.display = self.x.XOpenDisplay(None)
        if not self.display:
            raise RuntimeError("No X11/XWayland display; native measurement failed")
        event, error, major, minor = c.c_int(), c.c_int(), c.c_int(), c.c_int()
        if xtest and not self.xt.XTestQueryExtension(self.display, c.byref(event), c.byref(error), c.byref(major), c.byref(minor)):
            self.x.XCloseDisplay(self.display)
            self.display = None
            raise RuntimeError("XTest extension unavailable; native measurement failed")
        self._x_error = False
        callback_type = c.CFUNCTYPE(c.c_int, c.c_void_p, c.c_void_p)
        def on_x_error(*_):
            self._x_error = True
            return 0
        self._error_handler = callback_type(on_x_error)
        self.x.XSetErrorHandler.argtypes = [c.c_void_p]
        self.x.XSetErrorHandler.restype = c.c_void_p
        self._previous_error_handler = self.x.XSetErrorHandler(c.cast(self._error_handler, c.c_void_p))
        self.pid_atom = self.x.XInternAtom(self.display, b"_NET_WM_PID", 1)
        self.original_focus = self.focus_window()

    def activate(self, pid):
        """User-authorized activation, with a fresh timestamp from the X server."""
        x = self.x
        x.XDefaultRootWindow.argtypes = [c.c_void_p]
        x.XDefaultRootWindow.restype = c.c_ulong
        root = x.XDefaultRootWindow(self.display)
        atom = x.XInternAtom(self.display, b"_NET_CLIENT_LIST", 1)
        deadline = time.monotonic() + 5
        window = None
        while time.monotonic() < deadline and window is None:
            actual, format_, count, remaining = c.c_ulong(), c.c_int(), c.c_ulong(), c.c_ulong()
            data = c.POINTER(c.c_ubyte)()
            status = x.XGetWindowProperty(self.display, root, atom, 0, 4096, 0, 33,
                                          c.byref(actual), c.byref(format_), c.byref(count),
                                          c.byref(remaining), c.byref(data))
            try:
                if status == 0 and actual.value == 33 and format_.value == 32 and data:
                    clients = c.cast(data, c.POINTER(c.c_ulong))
                    matches = [clients[i] for i in range(count.value) if self.window_pid(clients[i]) == pid]
                    if len(matches) > 1:
                        raise RuntimeError("Ambiguous X11 target: more than one managed window belongs to measured PID")
                    window = matches[0] if matches else None
            finally:
                if data:
                    x.XFree(data)
            if window is None:
                time.sleep(.02)
        if window is None:
            raise RuntimeError(f"Measured PID {pid} has no managed X11 window: native Wayland targets cannot receive this XTest driver")
        # XPropertyEvent/XClientMessageEvent are smaller than XEvent's 24 longs.
        class PropertyEvent(c.Structure):
            _fields_ = [("type", c.c_int), ("serial", c.c_ulong), ("send_event", c.c_int),
                        ("display", c.c_void_p), ("window", c.c_ulong), ("atom", c.c_ulong),
                        ("time", c.c_ulong), ("state", c.c_int)]
        class ClientEvent(c.Structure):
            _fields_ = [("type", c.c_int), ("serial", c.c_ulong), ("send_event", c.c_int),
                        ("display", c.c_void_p), ("window", c.c_ulong),
                        ("message_type", c.c_ulong), ("format", c.c_int), ("data", c.c_long * 5)]
        class Event(c.Union):
            _fields_ = [("property", PropertyEvent), ("client", ClientEvent), ("pad", c.c_long * 24)]
        x.XCreateSimpleWindow.argtypes = [c.c_void_p, c.c_ulong, c.c_int, c.c_int,
                                         c.c_uint, c.c_uint, c.c_uint, c.c_ulong, c.c_ulong]
        x.XCreateSimpleWindow.restype = c.c_ulong
        x.XSelectInput.argtypes = [c.c_void_p, c.c_ulong, c.c_long]
        x.XChangeProperty.argtypes = [c.c_void_p, c.c_ulong, c.c_ulong, c.c_ulong,
                                     c.c_int, c.c_int, c.c_void_p, c.c_int]
        x.XPending.argtypes = [c.c_void_p]
        x.XNextEvent.argtypes = [c.c_void_p, c.POINTER(Event)]
        x.XSendEvent.argtypes = [c.c_void_p, c.c_ulong, c.c_int, c.c_long, c.POINTER(Event)]
        x.XDestroyWindow.argtypes = [c.c_void_p, c.c_ulong]
        temporary = x.XCreateSimpleWindow(self.display, root, 0, 0, 1, 1, 0, 0, 0)
        try:
            x.XSelectInput(self.display, temporary, 1 << 22)
            stamp_atom = x.XInternAtom(self.display, b"_INKWELL_PERF_TIMESTAMP", 0)
            x.XChangeProperty(self.display, temporary, stamp_atom, 31, 8, 0, b"x", 1)
            event = Event()
            x.XFlush(self.display)
            deadline = time.monotonic() + 2
            while True:
                if time.monotonic() >= deadline:
                    raise RuntimeError("X11 did not return an activation timestamp")
                if x.XPending(self.display):
                    x.XNextEvent(self.display, c.byref(event))
                    if event.property.type == 28 and event.property.window == temporary:
                        break
                else:
                    time.sleep(.005)
            timestamp = event.property.time
            active = Event()
            active.client.type, active.client.window = 33, window
            active.client.message_type = x.XInternAtom(self.display, b"_NET_ACTIVE_WINDOW", 0)
            active.client.format = 32
            active.client.data[0], active.client.data[1] = 2, timestamp
            x.XSendEvent(self.display, root, 0, (1 << 20) | (1 << 19), c.byref(active))
            x.XSync(self.display, 0)
            # XWayland permits this fallback when GNOME declines a background present.
            x.XSetInputFocus(self.display, window, 2, timestamp)
            x.XSync(self.display, 0)
            deadline = time.monotonic() + 2
            while (self.focus_pid() != pid or self.active_window() != window) and time.monotonic() < deadline:
                time.sleep(.01)
        finally:
            x.XDestroyWindow(self.display, temporary)
        # Prove the target's real XID/PID and report its GdkMonitor, not session
        # type or an assumed 60 Hz. This creates no GTK window.
        monitor = self.monitor_for_window(window)
        self.target = dict(pid=pid, xid=window, backend="x11", refreshHz=monitor["refreshHz"], monitor=monitor,
                           permissionVerified=self.permission_verified)
        return self.target

    def validate_target(self, pid):
        target = getattr(self, "target", None)
        if not target or target["pid"] != pid or self.window_pid(target["xid"]) != pid:
            raise RuntimeError("Measured X11 target XID/PID changed; no key injected")
        if self.active_window() != target["xid"]:
            raise ValueError(FOCUS_INTERRUPTED + " Active window is not the measured X11 XID; no key injected.")
        window = self.focus_window()
        for _ in range(32):
            if window == target["xid"]:
                break
            root, parent, children, count = c.c_ulong(), c.c_ulong(), c.POINTER(c.c_ulong)(), c.c_uint()
            status = self.x.XQueryTree(self.display, window, c.byref(root), c.byref(parent), c.byref(children), c.byref(count))
            if children: self.x.XFree(children)
            if not status or window in (0,1) or window == root.value or window == parent.value:
                raise ValueError(FOCUS_INTERRUPTED + " Focus is outside the measured X11 XID; no key injected.")
            window = parent.value
        else:
            raise RuntimeError("X11 target focus ancestry unverified; no key injected")
        monitor = self.monitor_for_window(target["xid"])
        if abs(monitor["refreshHz"] - target["refreshHz"]) > .000001:
            raise RuntimeError("Target monitor refresh changed during measurement; rerun cadence calibration")

    def monitor_for_window(self, window):
        if threading.current_thread() is not threading.main_thread():
            raise RuntimeError("GdkMonitor query requires the GTK main thread")
        if not window or not self.window_pid(window):
            raise RuntimeError("GdkMonitor query requires an existing managed X11 target window")
        gtk = c.CDLL("libgtk-4.so.1")
        display = initialized_monitor_display(gtk)
        gio = c.CDLL("libgio-2.0.so.0")
        obj = c.CDLL("libgobject-2.0.so.0")
        gtk.gdk_display_get_monitors.argtypes = [c.c_void_p]
        gtk.gdk_display_get_monitors.restype = c.c_void_p
        gio.g_list_model_get_n_items.argtypes = [c.c_void_p]
        gio.g_list_model_get_item.argtypes = [c.c_void_p, c.c_uint]
        gio.g_list_model_get_item.restype = c.c_void_p
        obj.g_object_unref.argtypes = [c.c_void_p]
        class Rect(c.Structure):
            _fields_ = [(name, c.c_int) for name in ("x", "y", "width", "height")]
        gtk.gdk_monitor_get_geometry.argtypes = [c.c_void_p, c.POINTER(Rect)]
        gtk.gdk_monitor_get_refresh_rate.argtypes = [c.c_void_p]
        gtk.gdk_monitor_get_scale_factor.argtypes = [c.c_void_p]
        model = gtk.gdk_display_get_monitors(display)
        monitors = []
        for i in range(gio.g_list_model_get_n_items(model)):
            monitor = gio.g_list_model_get_item(model, i)
            try:
                rect = Rect()
                gtk.gdk_monitor_get_geometry(monitor, c.byref(rect))
                scale = gtk.gdk_monitor_get_scale_factor(monitor)
                monitors.append(dict(x=rect.x*scale, y=rect.y*scale, width=rect.width*scale,
                                     height=rect.height*scale, refreshHz=gtk.gdk_monitor_get_refresh_rate(monitor)/1000))
            finally:
                obj.g_object_unref(monitor)
        root, child = c.c_ulong(), c.c_ulong()
        wx, wy, width, height, border, depth = c.c_int(), c.c_int(), c.c_uint(), c.c_uint(), c.c_uint(), c.c_uint()
        self.x.XGetGeometry.argtypes = [c.c_void_p, c.c_ulong, c.POINTER(c.c_ulong), c.POINTER(c.c_int), c.POINTER(c.c_int), c.POINTER(c.c_uint), c.POINTER(c.c_uint), c.POINTER(c.c_uint), c.POINTER(c.c_uint)]
        if not self.x.XGetGeometry(self.display, window, c.byref(root), c.byref(wx), c.byref(wy), c.byref(width), c.byref(height), c.byref(border), c.byref(depth)):
            raise RuntimeError("X11 target geometry unavailable")
        self.x.XTranslateCoordinates.argtypes = [c.c_void_p, c.c_ulong, c.c_ulong, c.c_int, c.c_int, c.POINTER(c.c_int), c.POINTER(c.c_int), c.POINTER(c.c_ulong)]
        if not self.x.XTranslateCoordinates(self.display, window, root.value, 0, 0, c.byref(wx), c.byref(wy), c.byref(child)):
            raise RuntimeError("X11 target monitor position unavailable")
        return select_monitor(monitors, wx.value + width.value/2, wy.value + height.value/2)

    def focus_window(self):
        window, revert = c.c_ulong(), c.c_int()
        self.x.XGetInputFocus(self.display, c.byref(window), c.byref(revert))
        return window.value

    def active_window(self):
        """Check the window manager's active top-level XID as well as X focus."""
        self.x.XDefaultRootWindow.argtypes = [c.c_void_p]
        self.x.XDefaultRootWindow.restype = c.c_ulong
        root = self.x.XDefaultRootWindow(self.display)
        atom = self.x.XInternAtom(self.display, b"_NET_ACTIVE_WINDOW", 1)
        if not atom:
            return None
        actual, format_, count, remaining = c.c_ulong(), c.c_int(), c.c_ulong(), c.c_ulong()
        data = c.POINTER(c.c_ubyte)()
        status = self.x.XGetWindowProperty(self.display, root, atom, 0, 1, 0, 33,
            c.byref(actual), c.byref(format_), c.byref(count), c.byref(remaining), c.byref(data))
        try:
            if status == 0 and actual.value == 33 and format_.value == 32 and count.value == 1 and data:
                return c.cast(data, c.POINTER(c.c_ulong))[0]
        finally:
            if data:
                self.x.XFree(data)
        return None

    def window_pid(self, window):
        actual, format_, count, remaining = c.c_ulong(), c.c_int(), c.c_ulong(), c.c_ulong()
        data = c.POINTER(c.c_ubyte)()
        status = self.x.XGetWindowProperty(self.display, window, self.pid_atom, 0, 1, 0, 6,
                                           c.byref(actual), c.byref(format_), c.byref(count),
                                           c.byref(remaining), c.byref(data))
        try:
            if status == 0 and actual.value == 6 and format_.value == 32 and count.value == 1 and data:
                return c.cast(data, c.POINTER(c.c_ulong))[0]
        finally:
            if data:
                self.x.XFree(data)
        return None

    def focus_pid(self):
        if self._x_error:
            raise RuntimeError("X11 rejected a window query; native measurement failed")
        window = self.focus_window()
        for _ in range(32):
            if window in (0, 1):
                return None
            pid = self.window_pid(window) if self.pid_atom else None
            if pid is not None:
                return pid
            root, parent, children, count = c.c_ulong(), c.c_ulong(), c.POINTER(c.c_ulong)(), c.c_uint()
            status = self.x.XQueryTree(self.display, window, c.byref(root), c.byref(parent), c.byref(children), c.byref(count))
            if children:
                self.x.XFree(children)
            if not status or window == root.value or window == parent.value:
                return None
            window = parent.value
        return None

    def send_key(self, char):
        self.send_keysym(ord(char))

    def send_keysym(self, symbol):
        keycode = self.x.XKeysymToKeycode(self.display, symbol)
        if not keycode:
            raise RuntimeError("ASCII test key has no X11 keycode")
        pressed = False
        try:
            if not self.xt.XTestFakeKeyEvent(self.display, keycode, 1, 0):
                raise RuntimeError("XTest key press failed")
            pressed = True
        finally:
            if pressed:
                released = self.xt.XTestFakeKeyEvent(self.display, keycode, 0, 0)
                self.x.XSync(self.display, 0)
                if self._x_error:
                    raise RuntimeError("X11 rejected native input injection")
                if not released:
                    raise RuntimeError("XTest key release failed")

    def close(self):
        if not self.display:
            return
        # A window may have disappeared during the run: restoration is best effort,
        # and must never leave Xlib's default fatal error handler installed here.
        try:
            if self.original_focus not in (0, 1):
                self.x.XSetInputFocus(self.display, self.original_focus, 2, 0)
                self.x.XSync(self.display, 0)
        finally:
            self.x.XSetErrorHandler(self._previous_error_handler)
            self.x.XCloseDisplay(self.display)
            self.display = None


class MutterKeyboard(X11Keyboard):
    """X11 verifies the exact target; only Mutter sends keyboard events."""
    def __init__(self):
        super().__init__(xtest=False)
        try:
            from native_mutter import MutterSession
            self.session = MutterSession()
        except BaseException:
            super().close()
            raise

    def send_keysym(self, keysym):
        self.session.send_keysym(keysym)

    def close(self):
        try:
            self.session.close()
        finally:
            super().close()



def collect_run(binary, out, phase, index, keyboard, timeout=120):
    run_id = secrets.token_hex(16)
    raw_path = out / f"{phase}-{index:02d}.json"
    # A failed/repeated invocation must not inherit an old process's raw report.
    raw_path.unlink(missing_ok=True)
    state_root = ROOT / ".task/perf/native-input"
    state_root.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=f"{phase}-{index:02d}-", dir=state_root) as state:
        profile = Path(state)
        env = dict(os.environ, GDK_BACKEND="x11", INKWELL_RENDEZVOUS="off", INKWELL_LAN="0",
                   INKWELL_HOME=str(profile / "home"), INKWELL_DOCS=str(profile / "docs"),
                   XDG_CACHE_HOME=str(profile / "cache"), XDG_DATA_HOME=str(profile / "data"),
                   INKWELL_NATIVE_PERF_PHASE=phase, INKWELL_NATIVE_PERF_INDEX=str(index),
                   INKWELL_NATIVE_PERF_REPLICATES="30", INKWELL_NATIVE_PERF_COUNT="1" if phase == "cold" else "30",
                   INKWELL_NATIVE_PERF_RUN_ID=run_id, INKWELL_NATIVE_PERF_OUT=str(raw_path))
        target_path = profile / "native-target.json"
        env["INKWELL_NATIVE_PERF_TARGET"] = str(target_path)
        child = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        protocol = TrialProtocol(run_id, phase, 1 if phase == "cold" else 30,
                                 lambda: keyboard.press_for(child.pid))
        # Added mandatory NULL trials retain the same 3s per-trial watchdog.
        deadline, pending, complete = time.monotonic() + timeout + (2 * NULL_COUNT * 3 if phase == "warm" else 0), b"", None
        probe_pending, probe_due = False, 0.0
        with selectors.DefaultSelector() as selector, (out / f"{phase}-{index:02d}.log").open("wb") as log:
            selector.register(child.stdout, selectors.EVENT_READ)
            try:
                while complete is None:
                    if time.monotonic() >= deadline:
                        raise TimeoutError("Native input run did not complete within its deadline")
                    if probe_pending and time.monotonic() >= probe_due:
                        if keyboard.backend.focus_pid() == child.pid:
                            keyboard.probe_for(child.pid)
                        probe_due = time.monotonic() + .2
                    for key, _ in selector.select(timeout=min(.1 if probe_pending else 1, max(0, deadline - time.monotonic()))):
                        chunk = os.read(key.fileobj.fileno(), 65536)
                        if not chunk:
                            raise RuntimeError(f"Native app exited before complete marker: {child.poll()}")
                        log.write(chunk)
                        log.flush()
                        pending += chunk
                        if len(pending) > 1024 * 1024:
                            raise ValueError("Unbounded native stdout line")
                        while b"\n" in pending:
                            line, pending = pending.split(b"\n", 1)
                            if not line.startswith(PREFIX):
                                continue
                            if complete is not None:
                                raise ValueError("Native protocol marker after completion")
                            marker = json.loads(line[len(PREFIX):])
                            if not isinstance(marker, dict):
                                raise ValueError("Invalid native marker")
                            if marker.get("type") == "complete":
                                validate_completion(marker, protocol, run_id, raw_path)
                                complete = marker
                            elif marker.get("type") == "activate":
                                validate_activation(marker, run_id, child.pid)
                                target = keyboard.activate_for(child.pid)
                                target_path.write_text(json.dumps(target))
                                log.write(("NATIVE_TARGET " + json.dumps(target) + "\n").encode())
                                log.flush()
                            elif marker.get("type") == "probe":
                                if phase != "warm":
                                    raise ValueError("Cold process must not receive a warm-up key")
                                if marker.get("runId") != run_id or marker.get("pid") != child.pid:
                                    raise ValueError("Mismatched XTest permission probe identity")
                                keyboard.probe_for(child.pid)
                                probe_pending, probe_due = True, time.monotonic() + .2
                            elif marker.get("type") == "probe-ok":
                                if marker.get("runId") != run_id or marker.get("pid") != child.pid:
                                    raise ValueError("Mismatched trusted native input proof")
                                probe_pending = False
                                keyboard.backend.permission_verified = True
                            else:
                                protocol.consume(marker)
                child.wait(timeout=max(1, deadline - time.monotonic()))
                # Preserve failed raw runs for merge, then collect the other modes.
                # A control/budget rejection is not a reason to omit cold evidence.
                if child.returncode != 0 and complete["status"] != "fail":
                    raise RuntimeError(f"Native collector exited {child.returncode}")
                if not raw_path.is_file():
                    raise ValueError("Native collector did not write its promised raw evidence")
                json.loads(raw_path.read_text())
                return dict(phase=phase, index=index, runId=run_id, status=complete["status"],
                            inputUnavailable=complete.get("inputUnavailable", False),
                            rawPath=str(raw_path.relative_to(ROOT)), sha256=hashlib.sha256(raw_path.read_bytes()).hexdigest())
            finally:
                if child.poll() is None:
                    child.terminate()
                    try:
                        child.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        child.kill()
                        child.wait()
                child.stdout.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=ROOT / "bin/inkwell-native-perf")
    parser.add_argument("--output", type=Path, default=ROOT / "gates/L3-native-raw")
    parser.add_argument("--input-backend", choices=("mutter", "xtest"), default="mutter",
                        help="prompt-free Mutter keyboard (default); xtest explicitly opts into per-run GNOME approval")
    args = parser.parse_args()
    binary, out = args.binary.resolve(), args.output.resolve()
    if not binary.is_relative_to(ROOT) or not out.is_relative_to(ROOT):
        parser.error("native binary and output must be inside this worktree")
    if not binary.is_file():
        parser.error("dedicated native-perf binary missing")
    out.mkdir(parents=True, exist_ok=True, mode=0o700)
    if os.environ.get("INKWELL_NATIVE_PERF_RUN") != "1":
        (out / "driver.json").write_text(json.dumps({"status": "blocked", "paint_certified": False,
            "reason": "Native host execution requires explicit INKWELL_NATIVE_PERF_RUN=1; no display or window opened."}, indent=2) + "\n")
        raise SystemExit(1)
    manifest = dict(status="collecting", paint_certified=False,
                    driver="Mutter RemoteDesktop trusted ASCII x" if args.input_backend == "mutter" else "X11/XWayland XTest trusted ASCII x",
                    cache_scope="fresh process/application cache per cold run; OS page cache not evicted",
                    binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(), runs=[])
    target = out / "driver.json"
    native = None
    try:
        native = MutterKeyboard() if args.input_backend == "mutter" else X11Keyboard()
        keyboard = FocusedKeyboard(native)
        # Collect all modes even when budgets fail. Only the Go merge certifies
        # performance; a structurally complete warm run must not suppress cold.
        warm = collect_run(binary, out, "warm", 1, keyboard)
        manifest["runs"].append(warm)
        if warm.get("inputUnavailable"):
            if args.input_backend == "mutter":
                raise RuntimeError("Mutter target/permission not validated by trusted keydown; stopping before cold replicates")
            raise RuntimeError("XTest target/permission not validated; stopping before cold replicates. Grant GNOME remote-interaction permission and rerun.")
        for index in range(1, 31):
            run = collect_run(binary, out, "cold", index, keyboard)
            manifest["runs"].append(run)
            if run.get("inputUnavailable"):
                raise RuntimeError("Native X11 input delivery lost; refusing further injection")
        manifest["status"] = "collected" if len(manifest["runs"]) == 31 and all(run["status"] == "collected" for run in manifest["runs"]) else "fail"
    except BaseException as error:
        manifest.update(status="fail", error=str(error))
        raise
    finally:
        try:
            if native is not None:
                native.close()
        except Exception as error:
            manifest.update(status="fail", error=str(error))
            raise
        finally:
            target.write_text(json.dumps(manifest, indent=2) + "\n")
    if manifest["status"] != "collected":
        raise SystemExit("Native evidence contains a failed run; no performance pass claimed")
    print("Native raw evidence collected; validate with the native report verifier. No paint budget certified by this driver.")


if __name__ == "__main__":
    main()
