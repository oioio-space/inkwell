"""GNOME-only keyboard injection, with one owned D-Bus connection per run.

This is Mutter's private RemoteDesktop API (not a portal). Importing this module
opens no connection, session, window or dialog. No token is read or written.
API: https://gitlab.gnome.org/GNOME/mutter/-/blob/main/data/dbus-interfaces/org.gnome.Mutter.RemoteDesktop.xml
"""
BUS = "org.gnome.Mutter.RemoteDesktop"
ROOT = "/org/gnome/Mutter/RemoteDesktop"
SESSION = BUS + ".Session"


class MutterError(RuntimeError):
    pass


class MutterSession:
    def __init__(self, *, gio=None, glib=None):
        self.connection, self.path, self.subscription = None, None, None
        self.closed = False
        try:
            if gio is None or glib is None:
                from gi.repository import Gio, GLib
                gio, glib = Gio, GLib
            self.gio, self.glib = gio, glib
            address = gio.dbus_address_get_for_bus_sync(gio.BusType.SESSION, None)
            # Own the connection instead of closing a shared bus_get connection.
            # The same connection must create, start, inject and stop the session.
            self.connection = gio.DBusConnection.new_for_address_sync(
                address, gio.DBusConnectionFlags.AUTHENTICATION_CLIENT |
                gio.DBusConnectionFlags.MESSAGE_BUS_CONNECTION, None, None)
            self.connection.set_exit_on_close(False)
            result = self.call("CreateSession", "()", (), path=ROOT, interface=BUS)
            path = result[0]
            if not isinstance(path, str) or not path.startswith(ROOT + "/"):
                raise MutterError("Mutter RemoteDesktop CreateSession returned an invalid session path")
            self.path = path
            self.subscription = self.connection.signal_subscribe(BUS, SESSION, "Closed", self.path,
                None, gio.DBusSignalFlags.NONE, self.mark_closed)
            self.call("Start", "()", ())
            self.check_alive()
        except BaseException as error:
            try:
                self.close()
            except Exception:
                pass
            if isinstance(error, (MutterError, KeyboardInterrupt, SystemExit)):
                raise
            raise MutterError("Mutter RemoteDesktop API unavailable: D-Bus/Gio connection or session setup failed; no input injected") from error

    def call(self, method, signature, args, *, path=None, interface=SESSION):
        try:
            return self.connection.call_sync(BUS, path or self.path, interface, method,
                self.glib.Variant(signature, args), None, self.gio.DBusCallFlags.NONE, 5000, None).unpack()
        except Exception as error:
            raise MutterError(f"Mutter RemoteDesktop {method} failed (API missing, denied or session lost); measurement cannot proceed") from error

    def mark_closed(self, *_):
        self.closed = True

    def check_alive(self):
        if not self.connection or self.closed or self.connection.is_closed():
            raise MutterError("Mutter RemoteDesktop session/connection closed; no input injected")
        context = self.glib.MainContext.default()
        while context.pending():
            context.iteration(False)
        if self.closed or self.connection.is_closed():
            raise MutterError("Mutter RemoteDesktop session/connection closed; no input injected")

    def send_keycode(self, keycode):
        self.check_alive()
        # Always attempt release, even if the reply to the press was lost.
        try:
            self.call("NotifyKeyboardKeycode", "(ub)", (keycode, True))
        finally:
            self.call("NotifyKeyboardKeycode", "(ub)", (keycode, False))
        self.check_alive()

    def send_keysym(self, keysym):
        # Linux input-event-codes.h: KEY_LEFTSHIFT=42, KEY_X=45. These are evdev
        # codes, with no X11 +8 offset. Do not silently inject another test key.
        codes = {0xffe1: 42, ord("x"): 45}
        if keysym not in codes:
            raise MutterError("Unsupported benchmark key; no Mutter input injected")
        self.send_keycode(codes[keysym])

    def close(self):
        connection, self.connection = self.connection, None
        if connection is None:
            return
        try:
            if self.path:
                # Stop is attempted even after Start fails or Closed is received.
                # Closing this owned connection finally destroys any surviving
                # session if Mutter can no longer answer Stop.
                try:
                    connection.call_sync(BUS, self.path, SESSION, "Stop", self.glib.Variant("()", ()),
                        None, self.gio.DBusCallFlags.NONE, 5000, None)
                except Exception as error:
                    raise MutterError("Mutter RemoteDesktop Stop failed; closing its owning D-Bus connection") from error
        finally:
            self.closed = True
            try:
                if self.subscription is not None:
                    connection.signal_unsubscribe(self.subscription)
            finally:
                connection.close_sync(None)
