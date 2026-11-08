"""Mutter D-Bus tests: no real bus, session, display, key or dialog."""
from types import SimpleNamespace
import unittest
from unittest import mock

import native_mutter as mutter


class Variant:
    def __init__(self, signature, value):
        self.signature, self.value = signature, value

    def unpack(self):
        return self.value


class MutterSessionTests(unittest.TestCase):
    def setUp(self):
        self.path = mutter.ROOT + "/Session/u1"
        self.connection = mock.Mock()
        self.connection.is_closed.return_value = False
        self.connection.signal_subscribe.return_value = 7
        self.connection.call_sync.side_effect = self.reply
        self.context = mock.Mock()
        self.context.pending.return_value = False
        self.gio = SimpleNamespace(
            BusType=SimpleNamespace(SESSION=2),
            DBusConnectionFlags=SimpleNamespace(AUTHENTICATION_CLIENT=1, MESSAGE_BUS_CONNECTION=8),
            DBusCallFlags=SimpleNamespace(NONE=0), DBusSignalFlags=SimpleNamespace(NONE=0),
            dbus_address_get_for_bus_sync=mock.Mock(return_value="mock-session-bus"),
            DBusConnection=SimpleNamespace(new_for_address_sync=mock.Mock(return_value=self.connection)))
        self.glib = SimpleNamespace(Variant=Variant, MainContext=SimpleNamespace(default=lambda: self.context))
        self.session = None

    def reply(self, bus, path, interface, method, parameters, *_):
        return Variant("(o)", (self.path,)) if method == "CreateSession" else Variant("()", ())

    def open(self):
        self.session = mutter.MutterSession(gio=self.gio, glib=self.glib)
        return self.session

    def methods(self):
        return [call.args[3] for call in self.connection.call_sync.call_args_list]

    def tearDown(self):
        if self.session is not None:
            self.connection.call_sync.side_effect = self.reply
            self.session.close()

    def test_one_connection_owns_create_start_every_evdev_key_and_stop(self):
        session = self.open()
        session.send_keysym(0xffe1)
        for _ in range(31):
            session.send_keysym(ord("x"))
        session.close()
        self.gio.DBusConnection.new_for_address_sync.assert_called_once_with("mock-session-bus", 9, None, None)
        calls = self.connection.call_sync.call_args_list
        self.assertEqual(self.methods(), ["CreateSession", "Start"] + ["NotifyKeyboardKeycode"] * 64 + ["Stop"])
        self.assertTrue(all(call.args[0] == mutter.BUS for call in calls))
        self.assertEqual(calls[0].args[1:4], (mutter.ROOT, mutter.BUS, "CreateSession"))
        for call in calls[1:]:
            self.assertEqual(call.args[1:3], (self.path, mutter.SESSION))
        keys = [call.args[4] for call in calls if call.args[3] == "NotifyKeyboardKeycode"]
        self.assertTrue(all(v.signature == "(ub)" for v in keys))
        self.assertEqual([v.value for v in keys[:4]], [(42, True), (42, False), (45, True), (45, False)])
        self.connection.close_sync.assert_called_once_with(None)
        self.connection.signal_unsubscribe.assert_called_once_with(7)

    def test_missing_api_fails_explicitly_without_start_or_input(self):
        self.connection.call_sync.side_effect = RuntimeError("UnknownMethod")
        with self.assertRaisesRegex(mutter.MutterError, "CreateSession failed.*API missing"):
            self.open()
        self.assertEqual(self.methods(), ["CreateSession"])
        self.connection.close_sync.assert_called_once_with(None)

    def test_start_failure_still_stops_the_created_session_and_closes_connection(self):
        def reply(*args):
            if args[3] == "Start":
                raise RuntimeError("Start denied")
            return self.reply(*args)
        self.connection.call_sync.side_effect = reply
        with self.assertRaisesRegex(mutter.MutterError, "Start failed"):
            self.open()
        self.assertEqual(self.methods(), ["CreateSession", "Start", "Stop"])
        self.connection.close_sync.assert_called_once_with(None)

    def test_connection_failure_is_explicit_and_has_no_other_backend(self):
        self.gio.DBusConnection.new_for_address_sync.side_effect = RuntimeError("no bus")
        with self.assertRaisesRegex(mutter.MutterError, "API unavailable"):
            self.open()
        self.connection.call_sync.assert_not_called()

    def test_lost_press_reply_still_attempts_release_then_caller_finally_stops(self):
        session = self.open()
        def reply(*args):
            if args[3] == "NotifyKeyboardKeycode" and args[4].value[1]:
                raise RuntimeError("lost reply")
            return self.reply(*args)
        self.connection.call_sync.side_effect = reply
        with self.assertRaisesRegex(mutter.MutterError, "NotifyKeyboardKeycode failed"):
            try:
                session.send_keysym(ord("x"))
            finally:
                session.close()
        self.assertEqual(self.methods(), ["CreateSession", "Start", "NotifyKeyboardKeycode", "NotifyKeyboardKeycode", "Stop"])
        self.assertEqual(self.connection.call_sync.call_args_list[-2].args[4].value, (45, False))
        self.connection.close_sync.assert_called_once_with(None)

    def test_stop_failure_is_explicit_but_owned_connection_always_closes(self):
        session = self.open()
        self.connection.call_sync.side_effect = RuntimeError("Stop unavailable")
        with self.assertRaisesRegex(mutter.MutterError, "Stop failed"):
            session.close()
        self.connection.close_sync.assert_called_once_with(None)
        session.close()
        self.connection.close_sync.assert_called_once_with(None)

    def test_closed_signal_blocks_all_further_input(self):
        session = self.open()
        signal = self.connection.signal_subscribe.call_args.args
        self.assertEqual(signal[:4], (mutter.BUS, mutter.SESSION, "Closed", self.path))
        signal[-1]()
        with self.assertRaisesRegex(mutter.MutterError, "session/connection closed"):
            session.send_keysym(ord("x"))
        self.assertEqual(self.methods(), ["CreateSession", "Start"])
        session.close()
        self.assertEqual(self.methods()[-1], "Stop")

    def test_connection_loss_blocks_all_further_input(self):
        session = self.open()
        self.connection.is_closed.return_value = True
        with self.assertRaisesRegex(mutter.MutterError, "connection closed"):
            session.send_keysym(ord("x"))
        self.assertEqual(self.methods(), ["CreateSession", "Start"])

    def test_closed_signal_during_context_pump_blocks_input(self):
        session = self.open()
        self.context.pending.side_effect = [True, False]
        self.context.iteration.side_effect = lambda *_: session.mark_closed()
        with self.assertRaisesRegex(mutter.MutterError, "session/connection closed"):
            session.send_keysym(ord("x"))
        self.assertEqual(self.methods(), ["CreateSession", "Start"])

    def test_unsupported_key_is_never_injected(self):
        session = self.open()
        with self.assertRaisesRegex(mutter.MutterError, "Unsupported benchmark key"):
            session.send_keysym(ord("a"))
        self.assertEqual(self.methods(), ["CreateSession", "Start"])

    def test_invalid_session_path_fails_before_start(self):
        self.connection.call_sync.return_value = Variant("(o)", ("/wrong/session",))
        self.connection.call_sync.side_effect = None
        with self.assertRaisesRegex(mutter.MutterError, "invalid session path"):
            self.open()
        self.assertEqual(self.methods(), ["CreateSession"])
        self.connection.close_sync.assert_called_once_with(None)


if __name__ == "__main__":
    unittest.main()
