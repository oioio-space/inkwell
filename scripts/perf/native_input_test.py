"""Pure driver tests: no X connection, native process, window or input injection."""
import importlib.util
from pathlib import Path
import unittest
from unittest import mock
import tempfile
import json
import os
import sys
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location("native_input", Path(__file__).with_name("native_input.py"))
driver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(driver)


class GtkMonitorInitializationTests(unittest.TestCase):
    def gtk(self, states):
        gtk = mock.Mock()
        gtk.gtk_is_initialized.side_effect = states
        gtk.gtk_init_check.return_value = 1
        gtk.gdk_display_get_default.return_value = 123
        return gtk

    def test_driver_initializes_gtk_before_getting_its_default_display(self):
        gtk = self.gtk([0, 1])
        self.assertEqual(driver.initialized_monitor_display(gtk), 123)
        self.assertEqual(gtk.mock_calls, [
            mock.call.gtk_is_initialized(), mock.call.gdk_set_allowed_backends(b"x11"),
            mock.call.gtk_init_check(), mock.call.gtk_is_initialized(),
            mock.call.gdk_display_get_default(),
        ])

    def test_failed_init_never_queries_a_display(self):
        gtk = self.gtk([0])
        gtk.gtk_init_check.return_value = 0
        with self.assertRaisesRegex(RuntimeError, "GTK initialization failed"):
            driver.initialized_monitor_display(gtk)
        gtk.gdk_display_get_default.assert_not_called()
        gtk.gdk_display_open.assert_not_called()

    def test_query_before_init_is_explicitly_refused_even_if_init_reports_success(self):
        gtk = self.gtk([0, 0])
        with self.assertRaisesRegex(RuntimeError, r"query refused before gtk_init\(\)"):
            driver.initialized_monitor_display(gtk)
        gtk.gdk_display_get_default.assert_not_called()
        gtk.gdk_display_get_monitors.assert_not_called()

    def test_initialized_display_is_reused_without_changing_backends_or_reinitializing(self):
        gtk = self.gtk([1, 1, 1, 1])
        for _ in range(2):
            self.assertEqual(driver.initialized_monitor_display(gtk), 123)
        gtk.gtk_init_check.assert_not_called()
        gtk.gdk_set_allowed_backends.assert_not_called()
        gtk.gdk_display_open.assert_not_called()
        gtk.gdk_display_close.assert_not_called()

    def test_missing_default_display_is_an_explicit_failure(self):
        gtk = self.gtk([1, 1])
        gtk.gdk_display_get_default.return_value = None
        with self.assertRaisesRegex(RuntimeError, "initialized X11 display unavailable"):
            driver.initialized_monitor_display(gtk)

    def test_worker_thread_is_rejected_before_any_gtk_call(self):
        gtk = self.gtk([0, 1])
        keyboard = object.__new__(driver.X11Keyboard)
        keyboard.window_pid = mock.Mock(return_value=42)
        with mock.patch.object(driver.threading, "current_thread", return_value=object()):
            for query in (lambda: driver.initialized_monitor_display(gtk),
                          lambda: keyboard.monitor_for_window(100)):
                with self.assertRaisesRegex(RuntimeError, "GTK main thread"):
                    query()
        self.assertEqual(gtk.mock_calls, [])
        keyboard.window_pid.assert_not_called()

    def test_target_must_exist_before_loading_or_initializing_gtk(self):
        keyboard = object.__new__(driver.X11Keyboard)
        keyboard.window_pid = mock.Mock(return_value=None)
        with mock.patch.object(driver.c, "CDLL") as load:
            for xid in (0, 100):
                with self.assertRaisesRegex(RuntimeError, "existing managed X11 target window"):
                    keyboard.monitor_for_window(xid)
            load.assert_not_called()

    def test_real_monitor_query_path_initializes_before_gdk_and_borrows_display(self):
        gtk = self.gtk([0, 1])
        def get_monitors(display):
            self.assertEqual(display, 123)
            gtk.gtk_init_check.assert_called_once_with()
            self.assertEqual(gtk.gtk_is_initialized.call_count, 2)
            raise RuntimeError("monitor query reached after init")
        gtk.gdk_display_get_monitors.side_effect = get_monitors
        keyboard = object.__new__(driver.X11Keyboard)
        keyboard.window_pid = mock.Mock(return_value=42)
        libraries = {"libgtk-4.so.1": gtk, "libgio-2.0.so.0": mock.Mock(),
                     "libgobject-2.0.so.0": mock.Mock()}
        with mock.patch.object(driver.c, "CDLL", side_effect=libraries.__getitem__):
            with self.assertRaisesRegex(RuntimeError, "monitor query reached after init"):
                keyboard.monitor_for_window(100)
        keyboard.window_pid.assert_called_once_with(100)
        gtk.gdk_display_open.assert_not_called()
        gtk.gdk_display_close.assert_not_called()
        libraries["libgobject-2.0.so.0"].g_object_unref.assert_not_called()


class DriverProtocolTests(unittest.TestCase):
    def test_default_mutter_driver_is_shared_across_all_runs_and_closed_in_finally(self):
        backend = SimpleNamespace(close=mock.Mock())
        calls = []
        def collect(binary, out, phase, index, keyboard):
            self.assertIs(keyboard.backend, backend)
            calls.append((phase, index))
            return dict(status="collected")
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "MutterKeyboard", return_value=backend) as create, \
             mock.patch.object(driver, "X11Keyboard", side_effect=AssertionError("XTest selected")), \
             mock.patch.object(driver, "collect_run", collect):
            driver.main()
            create.assert_called_once_with()
            self.assertEqual(calls, [("warm", 1)] + [("cold", n) for n in range(1, driver.COLD_REPLICATES + 1)])
            backend.close.assert_called_once_with()
            manifest = json.loads((Path(directory) / "driver.json").read_text())
            self.assertEqual(manifest["driver"], "Mutter RemoteDesktop trusted ASCII x")
            self.assertEqual(manifest["status"], "collected")

    def test_missing_mutter_api_fails_without_retry_xtest_or_app_launch(self):
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "MutterKeyboard", side_effect=RuntimeError("Mutter Start failed")) as create, \
             mock.patch.object(driver, "X11Keyboard", side_effect=AssertionError("XTest selected")), \
             mock.patch.object(driver, "collect_run") as collect:
            with self.assertRaisesRegex(RuntimeError, "Mutter Start failed"):
                driver.main()
            create.assert_called_once_with()
            collect.assert_not_called()
            manifest = json.loads((Path(directory) / "driver.json").read_text())
            self.assertEqual(manifest["status"], "fail")
            self.assertIn("Start failed", manifest["error"])

    def test_mutter_session_closes_on_collection_failure_or_interruption(self):
        for failure in (RuntimeError("input failed"), KeyboardInterrupt()):
            backend = SimpleNamespace(close=mock.Mock())
            with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
                 mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
                 mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
                 mock.patch.object(driver, "MutterKeyboard", return_value=backend), \
                 mock.patch.object(driver, "collect_run", side_effect=failure):
                with self.assertRaises(type(failure)):
                    driver.main()
                backend.close.assert_called_once_with()
                self.assertEqual(json.loads((Path(directory) / "driver.json").read_text())["status"], "fail")

    def test_mutter_requires_trusted_delivery_before_any_cold_replicate(self):
        backend = SimpleNamespace(close=mock.Mock())
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "MutterKeyboard", return_value=backend), \
             mock.patch.object(driver, "collect_run", return_value=dict(status="fail", inputUnavailable=True)) as collect:
            with self.assertRaises(SystemExit) as exit_:
                driver.main()
            self.assertEqual(exit_.exception.code, driver.EX_TEMPFAIL)
            self.assertRegex(json.loads((Path(directory) / "driver.json").read_text())["error"],
                             "Mutter target/permission not validated by trusted keydown")
            collect.assert_called_once()
            self.assertEqual(collect.call_args.args[2:4], ("warm", 1))
            backend.close.assert_called_once_with()

    def test_cleanup_failure_stays_a_failure_and_preserves_driver_evidence(self):
        backend = SimpleNamespace(close=mock.Mock(side_effect=RuntimeError("Stop failed")))
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "MutterKeyboard", return_value=backend), \
             mock.patch.object(driver, "collect_run", return_value=dict(status="collected")):
            with self.assertRaisesRegex(RuntimeError, "Stop failed"):
                driver.main()
            manifest = json.loads((Path(directory) / "driver.json").read_text())
            self.assertEqual(manifest["status"], "fail")
            self.assertEqual(len(manifest["runs"]), driver.COLD_REPLICATES + 1)

    def test_mutter_backend_never_initializes_xtest_and_only_delegates_injection(self):
        session = SimpleNamespace(send_keysym=mock.Mock(), close=mock.Mock())
        module = SimpleNamespace(MutterSession=mock.Mock(return_value=session))
        with mock.patch.object(driver.X11Keyboard, "__init__", return_value=None) as verify, \
             mock.patch.object(driver.X11Keyboard, "close") as close, \
             mock.patch.dict(sys.modules, native_mutter=module):
            backend = driver.MutterKeyboard()
            verify.assert_called_once_with(xtest=False)
            backend.send_key("x")
            backend.send_keysym(0xffe1)
            self.assertEqual(session.send_keysym.call_args_list, [mock.call(ord("x")), mock.call(0xffe1)])
            backend.close()
            session.close.assert_called_once_with()
            close.assert_called_once_with()

    def test_same_pid_but_wrong_active_xid_is_rejected(self):
        backend = object.__new__(driver.X11Keyboard)
        backend.target = dict(pid=42, xid=100)
        backend.window_pid = lambda _: 42
        backend.active_window = lambda: 200
        # Another window of the measured process is active: the product took the focus, a failure, not a fault.
        with self.assertRaisesRegex(RuntimeError, "Active window is not the measured X11 XID") as raised:
            backend.validate_target(42)
        self.assertNotIsInstance(raised.exception, driver.EnvironmentFault)

    def test_mutter_start_failure_releases_target_verifier_without_other_injector(self):
        module = SimpleNamespace(MutterSession=mock.Mock(side_effect=RuntimeError("Start failed")))
        with mock.patch.object(driver.X11Keyboard, "__init__", return_value=None) as verify, \
             mock.patch.object(driver.X11Keyboard, "close") as close, \
             mock.patch.dict(sys.modules, native_mutter=module):
            with self.assertRaisesRegex(RuntimeError, "Start failed"):
                driver.MutterKeyboard()
            verify.assert_called_once_with(xtest=False)
            module.MutterSession.assert_called_once_with()
            close.assert_called_once_with()

    def test_exact_target_is_checked_before_and_after_text_and_probe(self):
        for method, send in (("press_for", "send_key"), ("probe_for", "send_keysym")):
            backend = SimpleNamespace(validate_target=mock.Mock(), focus_pid=lambda: 42,
                                      send_key=mock.Mock(), send_keysym=mock.Mock())
            getattr(driver.FocusedKeyboard(backend), method)(42)
            self.assertEqual(backend.validate_target.call_args_list, [mock.call(42), mock.call(42)])
            getattr(backend, send).assert_called_once()
            backend.validate_target.side_effect = ValueError("wrong active XID")
            with self.assertRaisesRegex(ValueError, "wrong active XID"):
                getattr(driver.FocusedKeyboard(backend), method)(42)
            getattr(backend, send).assert_called_once()

    def test_focus_change_during_probe_or_post_key_target_change_is_fatal(self):
        backend = SimpleNamespace(focus_pid=mock.Mock(side_effect=[42, 43]), send_keysym=mock.Mock())
        with self.assertRaises(driver.EnvironmentFault):
            driver.FocusedKeyboard(backend).probe_for(42)
        backend = SimpleNamespace(validate_target=mock.Mock(side_effect=[None, ValueError("XID changed")]),
                                  focus_pid=lambda: 42, send_key=mock.Mock())
        with self.assertRaisesRegex(ValueError, "XID changed"):
            driver.FocusedKeyboard(backend).press_for(42)
        backend.send_key.assert_called_once_with("x")

    def test_target_validation_precedes_every_real_key(self):
        backend=SimpleNamespace(validate_target=mock.Mock(side_effect=RuntimeError("wrong XID")),
                                focus_pid=lambda:42,send_key=mock.Mock())
        with self.assertRaisesRegex(RuntimeError,"wrong XID"):
            driver.FocusedKeyboard(backend).press_for(42)
        backend.send_key.assert_not_called()

    def test_native_wayland_or_wrong_pid_target_is_rejected_before_injection(self):
        for backend, pid in [("wayland", 42), (None, 42), ("x11", 43), ("x11", True)]:
            with self.assertRaises((ValueError, RuntimeError)):
                driver.validate_activation(dict(runId="a", pid=pid, backend=backend), "a", 42)
        driver.validate_activation(dict(runId="a", pid=42, backend="x11"), "a", 42)

    def test_nominal_refresh_belongs_to_the_target_monitor_not_a_startup_guess(self):
        monitors = [dict(x=0,y=0,width=1920,height=1080,refreshHz=60),
                    dict(x=1920,y=0,width=2560,height=1440,refreshHz=144)]
        self.assertEqual(driver.select_monitor(monitors, 2200, 400)["refreshHz"], 144)
        for source, x in [(monitors, 5000), ([monitors[0],monitors[0]], 200),
                          ([dict(monitors[0],refreshHz=0)], 200)]:
            with self.assertRaisesRegex(RuntimeError, "refresh unavailable or ambiguous"):
                driver.select_monitor(source,x,400)

    def test_input_permission_failure_stops_before_cold_replicates(self):
        calls=[]
        def collect(binary,out,phase,index,keyboard):
            calls.append((phase,index))
            return dict(status="fail",inputUnavailable=True)
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--input-backend", "xtest", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "X11Keyboard", return_value=SimpleNamespace(close=mock.Mock())), \
             mock.patch.object(driver, "collect_run", collect):
            with self.assertRaises(SystemExit) as exit_:
                driver.main()
            self.assertEqual(exit_.exception.code, driver.EX_TEMPFAIL)
            self.assertRegex(json.loads((Path(directory)/"driver.json").read_text())["error"],"XTest target/permission not validated")
            self.assertEqual(calls,[("warm",1)])
            self.assertEqual(json.loads((Path(directory)/"driver.json").read_text())["status"],"fault")

    def test_permission_probe_is_a_modifier_and_checks_measured_focus(self):
        backend=SimpleNamespace(focus_pid=lambda:42,send_keysym=mock.Mock())
        keyboard=driver.FocusedKeyboard(backend)
        keyboard.probe_for(42)
        backend.send_keysym.assert_called_once_with(0xffe1)
        with self.assertRaises(driver.EnvironmentFault): keyboard.probe_for(43)

    def test_failed_completion_preserves_raw_evidence_and_returns_to_collection_loop(self):
        class FailedProcess:
            pid, returncode = 456, None
            def __init__(self, command, cwd, env, **kwargs):
                read_fd, write_fd = os.pipe()
                self.stdout = os.fdopen(read_fd, "rb")
                raw = Path(env["INKWELL_NATIVE_PERF_OUT"])
                raw.write_text(json.dumps({"failures": ["positive control oracle not validated"]}))
                marker = dict(type="complete", runId=env["INKWELL_NATIVE_PERF_RUN_ID"],
                              rawPath=str(raw), status="fail")
                os.write(write_fd, driver.PREFIX + json.dumps(marker).encode() + b'\n')
                os.close(write_fd)
            def poll(self): return self.returncode
            def wait(self, **kwargs): self.returncode = 1; return 1
            def terminate(self): self.returncode = -15
            def kill(self): self.returncode = -9
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.object(driver.subprocess, "Popen", FailedProcess):
            inject = mock.Mock(side_effect=AssertionError("unexpected input"))
            result = driver.collect_run(Path(__file__), Path(directory), "warm", 1,
                                        SimpleNamespace(press_for=inject))
            self.assertEqual(result["status"], "fail")
            self.assertEqual(json.loads((Path(directory) / "warm-01.json").read_text())["failures"],
                             ["positive control oracle not validated"])
            inject.assert_not_called()

    def test_failed_warm_control_still_collects_all_cold_runs_and_stays_failed(self):
        calls = []
        def collect(binary, out, phase, index, keyboard):
            calls.append((phase, index))
            return {"status": "fail" if phase == "warm" else "collected"}
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--input-backend", "xtest", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "X11Keyboard", return_value=SimpleNamespace(close=mock.Mock())), \
             mock.patch.object(driver, "collect_run", collect):
            with self.assertRaises(SystemExit):
                driver.main()
            self.assertEqual(calls, [("warm", 1)] + [("cold", n) for n in range(1, driver.COLD_REPLICATES + 1)])
            self.assertEqual(json.loads((Path(directory) / "driver.json").read_text())["status"], "fail")

    def test_raw_warm_budget_failure_does_not_suppress_thirty_cold_processes(self):
        calls=[]
        def collect(binary, out, phase, index, keyboard):
            calls.append((phase,index))
            return {"status":"collected", "phase":phase, "index":index,
                    "budgetVerdict":"fail" if phase=="warm" else "unknown"}
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--input-backend", "xtest", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "X11Keyboard", return_value=SimpleNamespace(close=mock.Mock())), \
             mock.patch.object(driver, "collect_run", collect):
            driver.main()
            self.assertEqual(calls, [("warm",1)]+[("cold",n) for n in range(1,driver.COLD_REPLICATES + 1)])

    def test_early_failed_completion_preserves_collector_reason(self):
        protocol = driver.TrialProtocol("a" * 32, "warm", driver.WARM_COUNT, lambda: None)
        raw = Path("warm-01.json")
        marker = dict(type="complete", runId="a" * 32, rawPath=str(raw), status="fail")
        driver.validate_completion(marker, protocol, "a" * 32, raw)
        marker["status"] = "collected"
        with self.assertRaisesRegex(ValueError, "Premature"):
            driver.validate_completion(marker, protocol, "a" * 32, raw)
        marker.update(status="fail", runId="wrong")
        with self.assertRaises(ValueError):
            driver.validate_completion(marker, protocol, "a" * 32, raw)

    def marker(self, kind, trial=1, **values):
        return dict(type=kind, runId="a" * 32, id=trial, phase="cold", control="none",
                    engine="WebKitGTK", visible=True, focused=True, **values)

    def test_arm_injects_once_and_waits_for_matching_ack(self):
        keys = []
        protocol = driver.TrialProtocol("a" * 32, "cold", 1, lambda: keys.append("x"))
        protocol.consume(self.marker("armed"))
        self.assertEqual(keys, ["x"])
        with self.assertRaises(ValueError):
            protocol.consume(self.marker("armed"))
        self.assertEqual(keys, ["x"])
        protocol.consume(self.marker("sample"))
        self.assertTrue(protocol.finished)

    def test_invisible_unfocused_untrusted_engine_wrong_run_never_injects(self):
        for field, value, error in [("visible", False, driver.EnvironmentFault), ("focused", False, driver.EnvironmentFault),
                                    ("engine", "Chromium", ValueError), ("runId", "b" * 32, ValueError)]:
            keys = []
            protocol = driver.TrialProtocol("a" * 32, "cold", 1, lambda: keys.append("x"))
            marker = self.marker("armed")
            marker[field] = value
            with self.assertRaises(error):
                protocol.consume(marker)
            self.assertEqual(keys, [])

    def test_sample_without_arm_and_wrong_id_fail(self):
        protocol = driver.TrialProtocol("a" * 32, "cold", 1, lambda: None)
        with self.assertRaises(ValueError):
            protocol.consume(self.marker("sample"))
        with self.assertRaises(ValueError):
            protocol.consume(self.marker("armed", trial=2))

    def test_warm_controls_precede_the_normal_trials(self):
        keys = []
        protocol = driver.TrialProtocol("a" * 32, "warm", driver.WARM_COUNT, lambda: keys.append("x"))
        sequence = driver.CONTROLS + ["none"] * driver.WARM_COUNT + driver.null_schedule()
        for trial, control in enumerate(sequence, 1):
            for kind in ["armed", "sample"]:
                protocol.consume(dict(type=kind, runId="a" * 32, id=trial, phase="warm",
                                      control=control, engine="WebKitGTK", visible=True, focused=True))
        self.assertTrue(protocol.finished)
        self.assertEqual(len(keys), driver.WARM_COUNT + len(driver.CONTROLS) + 2 * driver.NULL_COUNT)
        nulls = sequence[-2 * driver.NULL_COUNT:]
        self.assertEqual(nulls.count("null"), driver.NULL_COUNT)
        self.assertEqual(nulls[:4], ["null", "null-off", "null-off", "null"])

    def test_focus_owner_mismatch_sends_no_key(self):
        class FakeX:
            sent = []
            def focus_pid(self): return 42
            def send_key(self, char): self.sent.append(char)
        backend = FakeX()
        keyboard = driver.FocusedKeyboard(backend)
        with self.assertRaises(driver.EnvironmentFault):
            keyboard.press_for(43)
        self.assertEqual(backend.sent, [])
        keyboard.press_for(42)
        self.assertEqual(backend.sent, ["x"])

    def test_activation_verifies_focus_without_injecting_input(self):
        backend = SimpleNamespace(activate=mock.Mock(), focus_pid=lambda: 42,
                                  send_key=mock.Mock())
        keyboard = driver.FocusedKeyboard(backend)
        keyboard.activate_for(42)
        backend.activate.assert_called_once_with(42)
        backend.send_key.assert_not_called()
        with self.assertRaisesRegex(driver.EnvironmentFault, "activation did not establish focus"):
            keyboard.activate_for(43)

    def test_host_guard_never_opens_display_or_process(self):
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory:
            with mock.patch.dict(driver.os.environ, {"INKWELL_NATIVE_PERF_RUN": "0"}), \
                 mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
                 mock.patch.object(driver, "X11Keyboard", side_effect=AssertionError("display opened")), \
                 mock.patch.object(driver, "MutterKeyboard", side_effect=AssertionError("D-Bus opened")), \
                 mock.patch.object(driver.subprocess, "Popen", side_effect=AssertionError("process launched")):
                with self.assertRaises(SystemExit):
                    driver.main()
                self.assertEqual(json.loads((Path(directory) / "driver.json").read_text())["status"], "blocked")

    def test_undersampled_warm_or_non_single_cold_rejected(self):
        for phase, count in [("warm", driver.WARM_COUNT - 1), ("cold", 2), ("cold", True)]:
            with self.assertRaises(ValueError):
                driver.TrialProtocol("a" * 32, phase, count, lambda: None)

    def test_xtest_key_release_follows_every_successful_press(self):
        events = []
        native = object.__new__(driver.X11Keyboard)
        native.display, native._x_error = 123, False
        native.x = SimpleNamespace(XKeysymToKeycode=lambda *_: 53, XSync=lambda *_: None)
        def key_event(display, keycode, pressed, delay):
            events.append((keycode, pressed))
            return 1
        native.xt = SimpleNamespace(XTestFakeKeyEvent=key_event)
        native.send_key("x")
        self.assertEqual(events, [(53, 1), (53, 0)])

    def test_fake_runner_collects_only_matching_protocol_and_raw_file(self):
        keys = []
        class FakeProcess:
            pid, returncode = 123, None
            def __init__(self, command, cwd, env, **kwargs):
                read_fd, write_fd = os.pipe()
                self.stdout = os.fdopen(read_fd, "rb")
                raw_path = Path(env["INKWELL_NATIVE_PERF_OUT"])
                raw_path.write_text('{}')
                common = dict(runId=env["INKWELL_NATIVE_PERF_RUN_ID"], id=1, phase="cold", control="none")
                markers = [dict(common, type="armed", engine="WebKitGTK", visible=True, focused=True),
                           dict(common, type="sample"),
                           dict(type="complete", runId=common["runId"], rawPath=str(raw_path), status="collected")]
                os.write(write_fd, b''.join(driver.PREFIX + json.dumps(marker).encode() + b'\n' for marker in markers))
                os.close(write_fd)
            def poll(self): return self.returncode
            def wait(self, **kwargs): self.returncode = 0; return 0
            def terminate(self): self.returncode = -15
            def kill(self): self.returncode = -9
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.object(driver.subprocess, "Popen", FakeProcess):
            result = driver.collect_run(Path(__file__), Path(directory), "cold", 1,
                                        SimpleNamespace(press_for=lambda pid: keys.append(pid)))
            self.assertEqual(result["status"], "collected")
            self.assertEqual(keys, [123])


class EnvironmentFaultTests(unittest.TestCase):
    """Plan section 8: the host's faults are named, signalled by exit 75 and never a performance failure."""

    def test_focus_loss_raises_environment_fault(self):
        backend = SimpleNamespace(focus_pid=lambda: 43, send_key=mock.Mock())
        with self.assertRaises(driver.EnvironmentFault) as raised:
            driver.FocusedKeyboard(backend).press_for(42)
        self.assertEqual(raised.exception.kind, "focus-lost")
        backend.send_key.assert_not_called()
        moved = SimpleNamespace(focus_pid=mock.Mock(side_effect=[42, 43]), send_key=mock.Mock())
        with self.assertRaises(driver.EnvironmentFault) as raised:
            driver.FocusedKeyboard(moved).press_for(42)  # focus taken while the key was being sent
        self.assertEqual(raised.exception.kind, "focus-lost")
        protocol = driver.TrialProtocol("a" * 32, "cold", 1, lambda: None)
        marker = dict(type="armed", runId="a" * 32, id=1, phase="cold", control="none", engine="WebKitGTK",
                      visible=True, focused=False)
        with self.assertRaises(driver.EnvironmentFault) as raised:
            protocol.consume(marker)
        self.assertEqual(raised.exception.kind, "focus-lost")

    def test_focus_taken_by_the_product_is_a_failure(self):
        """A focus that moved to a window of the measured process, or of one of its descendants, is the product's
        own doing: a failure (never retried), not the environment fault focus-lost."""
        tree = {43: 42, 42: 1, 99: 1}  # pid -> parent: 43 is a child of the measured 42, 99 is another program
        parent = tree.get
        child = SimpleNamespace(focus_pid=lambda: 43, send_key=mock.Mock())
        with mock.patch.object(driver, "parent_pid", parent):
            with self.assertRaises(RuntimeError) as raised:
                driver.FocusedKeyboard(child).press_for(42)
            self.assertNotIsInstance(raised.exception, driver.EnvironmentFault)
            self.assertIn("product", str(raised.exception))
            other = SimpleNamespace(focus_pid=lambda: 99, send_key=mock.Mock())
            with self.assertRaises(driver.EnvironmentFault):
                driver.FocusedKeyboard(other).press_for(42)
            self.assertTrue(driver.descends(43, 42) and driver.descends(42, 42))
            self.assertFalse(driver.descends(99, 42) or driver.descends(None, 42))
        # X11: the active window is another top-level of the measured process (a dialog it opened).
        x11 = driver.X11Keyboard.__new__(driver.X11Keyboard)
        x11.target = dict(pid=42, xid=7, refreshHz=60.0)
        x11.window_pid = lambda window: {7: 42, 8: 42, 9: 99}[window]
        for active, kind in ((8, RuntimeError), (9, driver.EnvironmentFault)):
            x11.active_window = lambda active=active: active
            with mock.patch.object(driver, "parent_pid", parent), self.assertRaises(kind) as raised:
                x11.validate_target(42)
            if kind is RuntimeError:
                self.assertNotIsInstance(raised.exception, driver.EnvironmentFault)

    def test_delivery_loss_raises_environment_fault(self):
        for backend_name in ("mutter", "xtest"):
            for phase_failing in ("warm", "cold"):
                def collect(binary, out, phase, index, keyboard, failing=phase_failing):
                    return dict(status="fail", inputUnavailable=(phase == failing))
                with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
                     mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
                     mock.patch("sys.argv", ["native_input", "--input-backend", backend_name, "--binary", __file__, "--output", directory]), \
                     mock.patch.object(driver, "MutterKeyboard", return_value=SimpleNamespace(close=mock.Mock())), \
                     mock.patch.object(driver, "X11Keyboard", return_value=SimpleNamespace(close=mock.Mock())), \
                     mock.patch.object(driver, "collect_run", collect):
                    with self.assertRaises(SystemExit) as exit_:
                        driver.main()
                    self.assertEqual(exit_.exception.code, driver.EX_TEMPFAIL)
                    fault = json.loads((Path(directory) / "driver.json").read_text())["fault"]
                    self.assertEqual(fault["kind"], "delivery-lost" if backend_name == "mutter" or phase_failing == "cold" else "permission-dialog")

    def test_activation_failure_raises_environment_fault(self):
        backend = SimpleNamespace(activate=mock.Mock(), focus_pid=lambda: 43)
        with self.assertRaises(driver.EnvironmentFault) as raised:
            driver.FocusedKeyboard(backend).activate_for(42)
        self.assertEqual(raised.exception.kind, "activation-failed")

    def test_engine_mismatch_is_not_environment(self):
        """A wrong engine or run identity is a setup error: a plain ValueError, never retried."""
        for field, value in (("engine", "Chromium"), ("runId", "b" * 32)):
            protocol = driver.TrialProtocol("a" * 32, "cold", 1, lambda: None)
            marker = dict(type="armed", runId="a" * 32, id=1, phase="cold", control="none", engine="WebKitGTK",
                          visible=True, focused=True)
            marker[field] = value
            with self.assertRaises(ValueError) as raised:
                protocol.consume(marker)
            self.assertNotIsInstance(raised.exception, driver.EnvironmentFault)
        with self.assertRaises(RuntimeError) as raised:
            driver.validate_activation(dict(runId="a", pid=42, backend="wayland"), "a", 42)
        self.assertNotIsInstance(raised.exception, driver.EnvironmentFault)

    def test_driver_exit_75_writes_fault(self):
        backend = SimpleNamespace(close=mock.Mock())
        failure = driver.EnvironmentFault("focus-lost", "another window took the focus")
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "MutterKeyboard", return_value=backend), \
             mock.patch.object(driver, "collect_run", side_effect=failure):
            with self.assertRaises(SystemExit) as exit_:
                driver.main()
            self.assertEqual(exit_.exception.code, 75)
            manifest = json.loads((Path(directory) / "driver.json").read_text())
            self.assertEqual(manifest["status"], "fault")
            self.assertEqual(manifest["fault"], dict(kind="focus-lost", message="another window took the focus"))
            backend.close.assert_called_once_with()
        # A genuine failure keeps its own status and exit.
        with tempfile.TemporaryDirectory(dir=driver.ROOT) as directory, \
             mock.patch.dict(os.environ, INKWELL_NATIVE_PERF_RUN="1"), \
             mock.patch("sys.argv", ["native_input", "--binary", __file__, "--output", directory]), \
             mock.patch.object(driver, "MutterKeyboard", return_value=SimpleNamespace(close=mock.Mock())), \
             mock.patch.object(driver, "collect_run", side_effect=RuntimeError("input failed")):
            with self.assertRaises(RuntimeError):
                driver.main()
            self.assertEqual(json.loads((Path(directory) / "driver.json").read_text())["status"], "fail")


if __name__ == "__main__":
    unittest.main()
