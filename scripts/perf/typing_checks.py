"""Automated typing checks in the real perf-build window, CodeMirror-owned typing ON then OFF.

Each mode builds the perf collector (`VITE_PERF_CM_TYPING` = 1 or 0), launches it in its
`typing-check` phase (no measurement), types with real keys through the Mutter RemoteDesktop
session (NotifyKeyboardKeysym / NotifyKeyboardKeycode, pointer for Ctrl+click) and reads the result
twice: from CodeMirror over the perf stream (Go prints it on stdout) and from the accessibility bus
(AT-SPI text and events, what Orca reads). Checks: dead keys, IME composition through ibus, undo /
redo, AT-SPI typing events, autocompletion, multiple cursors, read-only, paste. Both modes must
pass and give the same results. Requires a lit, unlocked GNOME session (DISPLAY, GDK_BACKEND=x11)
and INKWELL_NATIVE_PERF_RUN=1. Run through `mise run perf:typing-checks` (exclusive heavy slot).
"""
import argparse
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(Path(__file__).resolve().parent))
import refbuild  # noqa: E402  (the pinned Go, never the first `go` on PATH)

PREFIX = b"INKWELL_TYPING_CHECK "
# linux/input-event-codes.h: evdev codes, no X11 +8 offset.
KEY_LEFTCTRL, KEY_LEFTSHIFT, BTN_LEFT = 29, 42, 0x110
CTRL, SHIFT = (KEY_LEFTCTRL,), (KEY_LEFTSHIFT,)
XK = dict(dead_acute=0xFE51, dead_circumflex=0xFE52, dead_diaeresis=0xFE57, space=0x20,
          Return=0xFF0D, Home=0xFF50, End=0xFF57, Escape=0xFF1B)
# (dead key, next key, character the request expects). The pass criterion is what the host's XKB
# compose table gives for the sequence (compose_lookup): on fr_FR.UTF-8 (en_US.UTF-8/Compose)
# <dead_diaeresis> <space> is '"', not '¨'.
DEAD_KEYS = (("dead_circumflex", "e", "ê"), ("dead_acute", "e", "é"), ("dead_diaeresis", "space", "¨"))


class CheckError(RuntimeError):
    pass


def keysym(char):
    """X11 keysym of one character: Latin-1 maps to itself, the rest to 0x1000000 + code point."""
    code = ord(char)
    return code if code < 0x100 else 0x1000000 + code


def compose_lookup(sequence, locale=None):
    """Result of a key sequence in the locale's X11 compose table (the table ibus/GTK compose from)."""
    import re
    base = Path("/usr/share/X11/locale")
    locale = locale or os.environ.get("LC_ALL") or os.environ.get("LC_CTYPE") or os.environ.get("LANG", "C")
    table = None
    for line in (base / "compose.dir").read_text().splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1].rstrip(":") == locale:
            table = base / parts[0].rstrip(":")
            break
    if table is None:
        return None
    wanted = " ".join(f"<{key}>" for key in sequence)
    for line in table.read_text(errors="replace").splitlines():
        match = re.match(r'^(<[^:]+>)\s*:\s*"((?:\\.|[^"])*)"', line)
        if match and " ".join(match.group(1).split()) == wanted:
            return match.group(2).encode().decode("unicode_escape").encode("latin-1").decode("utf-8")
    return None


def build(mode, runner=subprocess.run):
    binary = ROOT / f"bin/inkwell-typing-check-{mode}"
    env = dict(os.environ, VITE_PERF_NATIVE="1", VITE_E2E_OBSERVE="0",
               VITE_PERF_CM_TYPING="1" if mode == "on" else "0")
    runner(["pnpm", "--config.verifyDepsBeforeRun=false", "exec", "vite", "build", "--configLoader",
            "runner", "--mode", "production", "--outDir", "perf-dist"], cwd=ROOT / "frontend", env=env, check=True)
    runner(["go", "build", "-tags", "perf_native", "-buildvcs=false", "-o", str(binary), "./cmd/inkwell"],
           cwd=ROOT, env=refbuild.tree_go_env(ROOT), check=True)
    return binary


def pump(seconds):
    """Runs the default GLib context (AT-SPI events, Mutter's D-Bus signals) for a while."""
    from gi.repository import GLib
    context, deadline = GLib.MainContext.default(), time.monotonic() + seconds
    while True:
        while context.pending():
            context.iteration(False)
        if time.monotonic() >= deadline:
            return
        time.sleep(.005)


class App:
    """The perf binary in its typing-check phase, with a fresh private profile."""

    def __init__(self, binary, profile):
        env = dict(os.environ, GDK_BACKEND="x11", INKWELL_RENDEZVOUS="off", INKWELL_LAN="0",
                   INKWELL_HOME=str(profile / "home"), INKWELL_DOCS=str(profile / "docs"),
                   XDG_CACHE_HOME=str(profile / "cache"), XDG_DATA_HOME=str(profile / "data"),
                   INKWELL_NATIVE_PERF_RUN="1", INKWELL_NATIVE_PERF_PHASE="typing-check")
        self.child = subprocess.Popen([str(binary)], cwd=ROOT, env=env, stdin=subprocess.PIPE,
                                      stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        self.messages, self.tail = queue.Queue(), []
        threading.Thread(target=self._read, daemon=True).start()

    def _read(self):
        for line in self.child.stdout:
            self.tail = (self.tail + [line.decode(errors="replace").rstrip()])[-40:]
            if line.startswith(PREFIX):
                self.messages.put(json.loads(line[len(PREFIX):]))
        self.messages.put(None)

    def wait(self, kinds, timeout):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            pump(0)
            try:
                message = self.messages.get(timeout=.02)
            except queue.Empty:
                continue
            if message is None:
                raise CheckError(f"app exited ({self.child.poll()}): " + " | ".join(self.tail[-8:]))
            if message.get("type") in kinds:
                return message
        raise CheckError(f"no {kinds} message from the app within {timeout}s")

    def command(self, cmd, timeout=5, **args):
        while not self.messages.empty():
            self.messages.get_nowait()
        self.child.stdin.write((json.dumps(dict(cmd=cmd, **args)) + "\n").encode())
        self.child.stdin.flush()
        return self.wait({"coords"} if cmd == "coords" else {"state"}, timeout)

    def until(self, predicate, timeout=3):
        deadline = time.monotonic() + timeout
        state = self.command("state")
        while not predicate(state) and time.monotonic() < deadline:
            pump(.05)
            state = self.command("state")
        return state

    def close(self):
        if self.child.poll() is None:
            self.child.terminate()
            try:
                self.child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.child.kill()
                self.child.wait()


class Keys:
    """Real input through the Mutter session; the X11 target and focus are verified before each."""

    def __init__(self, native, pid):
        self.native, self.pid, self.session = native, pid, native.session

    def guard(self):
        self.native.validate_target(self.pid)
        if self.native.focus_pid() != self.pid:
            raise CheckError("focus left the checked window; no key injected")
        self.session.check_alive()

    def call(self, method, signature, args):
        self.session.call(method, signature, args)

    def tap(self, sym, mods=()):
        self.guard()
        held = []
        try:
            for code in mods:
                self.call("NotifyKeyboardKeycode", "(ub)", (code, True))
                held.append(code)
            self.call("NotifyKeyboardKeysym", "(ub)", (sym, True))
            try:
                pump(.02)
            finally:
                self.call("NotifyKeyboardKeysym", "(ub)", (sym, False))
        finally:
            for code in reversed(held):
                self.call("NotifyKeyboardKeycode", "(ub)", (code, False))
        pump(.05)

    def type(self, text):
        for char in text:
            self.tap(keysym(char))


class Pointer:
    """A second Mutter RemoteDesktop session linked to a ScreenCast of the primary monitor: on this
    host Mutter ignores pointer motion from a session without a stream. The stream is never read."""

    RD, SC = "org.gnome.Mutter.RemoteDesktop", "org.gnome.Mutter.ScreenCast"

    def __init__(self):
        from gi.repository import Gio, GLib
        self.Gio, self.GLib, self.path = Gio, GLib, None
        address = Gio.dbus_address_get_for_bus_sync(Gio.BusType.SESSION, None)
        self.bus = Gio.DBusConnection.new_for_address_sync(address, Gio.DBusConnectionFlags.AUTHENTICATION_CLIENT |
                                                           Gio.DBusConnectionFlags.MESSAGE_BUS_CONNECTION, None, None)
        self.bus.set_exit_on_close(False)
        try:
            self.path = self._call(self.RD, "/org/gnome/Mutter/RemoteDesktop", self.RD, "CreateSession", None, None)[0]
            session_id = self._call(self.RD, self.path, "org.freedesktop.DBus.Properties", "Get", "(ss)",
                                    (self.RD + ".Session", "SessionId"))[0]
            cast = self._call(self.SC, "/org/gnome/Mutter/ScreenCast", self.SC, "CreateSession", "(a{sv})",
                              ({"remote-desktop-session-id": GLib.Variant("s", session_id)},))[0]
            self.stream = self._call(self.SC, cast, self.SC + ".Session", "RecordMonitor", "(sa{sv})", ("", {}))[0]
            self.call("Start", "()", ())
        except BaseException:
            self.close()
            raise

    def _call(self, dest, path, iface, method, signature, args):
        value = self.GLib.Variant(signature, args) if signature else None
        return self.bus.call_sync(dest, path, iface, method, value, None, self.Gio.DBusCallFlags.NONE, 5000, None).unpack()

    def call(self, method, signature, args):
        return self._call(self.RD, self.path, self.RD + ".Session", method, signature, args)

    def move_to(self, x, y):
        self.call("NotifyPointerMotionAbsolute", "(sdd)", (self.stream, float(x), float(y)))
        pump(.08)

    def click(self, mods=()):
        """Left click, modifiers held on this session's own keyboard so they precede the button."""
        held = []
        try:
            for code in mods:
                self.call("NotifyKeyboardKeycode", "(ub)", (code, True))
                held.append(code)
            pump(.1)
            self.call("NotifyPointerButton", "(ib)", (BTN_LEFT, True))
            try:
                pump(.03)
            finally:
                self.call("NotifyPointerButton", "(ib)", (BTN_LEFT, False))
        finally:
            for code in reversed(held):
                self.call("NotifyKeyboardKeycode", "(ub)", (code, False))
        pump(.1)

    def close(self):
        try:
            if self.path:
                self.call("Stop", "()", ())
        finally:
            self.bus.close_sync(None)


def window_center(native):
    """Centre of the target X11 window in root coordinates (Xwayland = compositor logical)."""
    import ctypes as c
    x, xid = native.x, native.target["xid"]
    root, child = c.c_ulong(), c.c_ulong()
    wx, wy, w, h, border, depth = c.c_int(), c.c_int(), c.c_uint(), c.c_uint(), c.c_uint(), c.c_uint()
    x.XGetGeometry.argtypes = [c.c_void_p, c.c_ulong] + [c.c_void_p] * 7
    x.XTranslateCoordinates.argtypes = [c.c_void_p, c.c_ulong, c.c_ulong, c.c_int, c.c_int, c.c_void_p, c.c_void_p, c.c_void_p]
    if not x.XGetGeometry(native.display, xid, c.byref(root), c.byref(wx), c.byref(wy), c.byref(w), c.byref(h), c.byref(border), c.byref(depth)):
        raise CheckError("target geometry unavailable")
    if not x.XTranslateCoordinates(native.display, xid, root.value, 0, 0, c.byref(wx), c.byref(wy), c.byref(child)):
        raise CheckError("target position unavailable")
    return wx.value + w.value // 2, wy.value + h.value // 2


def pointer_to(app, pointer, native, target):
    """Closed loop on the webview's own mousemove report: no assumed window or frame offsets."""
    monitor = native.target["monitor"]
    x, y = window_center(native)
    x, y = x - monitor["x"], y - monitor["y"]  # stream coordinates are the monitor's
    for _ in range(6):
        pointer.move_to(x, y)
        seen = app.command("state").get("pointer")
        if not seen:
            raise CheckError("the pointer never reached the editor webview (no mousemove)")
        dx, dy = target[0] - seen["x"], target[1] - seen["y"]
        if abs(dx) < 1.5 and abs(dy) < 1.5:
            return seen
        x, y = x + dx, y + dy
    raise CheckError(f"pointer did not settle on {target}: {seen}")


class Accessibility:
    """AT-SPI: the editor's text as assistive technologies read it, and its text/caret events."""

    KINDS = ("object:text-changed:insert", "object:text-changed:delete", "object:text-caret-moved")

    def __init__(self, pid):
        import gi
        gi.require_version("Atspi", "2.0")
        from gi.repository import Atspi
        self.Atspi, self.pid, self.events, self.editor = Atspi, pid, [], None
        self.listener = Atspi.EventListener.new(self._on)
        for kind in self.KINDS:
            self.listener.register(kind)

    def _on(self, event, *_):
        try:
            source = event.source
            role = source.get_role_name() if source else None
        except Exception:  # an accessible that disappeared meanwhile
            role = None
        data = event.any_data if isinstance(event.any_data, str) else None
        self.events.append(dict(type=event.type, detail1=event.detail1, detail2=event.detail2, text=data, role=role))

    def close(self):
        for kind in self.KINDS:
            self.listener.deregister(kind)

    def find_editor(self, timeout=10):
        """The focused, editable, multi-line accessible inside the app's tree."""
        S = self.Atspi.StateType
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            desktop = self.Atspi.get_desktop(0)
            apps = [desktop.get_child_at_index(i) for i in range(desktop.get_child_count())]
            app = next((a for a in apps if a and a.get_process_id() == self.pid), None)
            stack = [(app, 0)] if app else []
            while stack:
                node, depth = stack.pop()
                try:
                    states = node.get_state_set()
                    if states.contains(S.EDITABLE) and states.contains(S.MULTI_LINE) and states.contains(S.FOCUSED):
                        self.editor = node
                        return node
                    if depth < 40:
                        stack.extend((node.get_child_at_index(i), depth + 1) for i in range(node.get_child_count()))
                except Exception:
                    continue
            pump(.25)
        raise CheckError("no focused editable multi-line accessible in the app (AT-SPI tree not exposed?)")

    def text(self):
        editor = self.editor or self.find_editor()
        Text = self.Atspi.Text
        return Text.get_text(editor, 0, Text.get_character_count(editor)), Text.get_caret_offset(editor)


def check(name, passed, **evidence):
    return dict(name=name, passed=bool(passed), **evidence)


def run_checks(app, keys, native, a11y):
    reset = lambda text="", **kw: app.command("reset", text=text, **kw)

    def a_dead_keys():
        # a) dead keys, through the session's XKB layout and the IM's compose path.
        layout = {name: bool(native.x.XKeysymToKeycode(native.display, XK[name])) for name, _, _ in DEAD_KEYS}
        if not all(layout.values()):
            raise CheckError(f"the session layout lacks dead keys {layout}; set a dead-key layout first")
        cases = []
        for dead, base, requested in DEAD_KEYS:
            want = compose_lookup((dead, base)) or requested
            reset()
            keys.tap(XK[dead])
            keys.tap(XK.get(base) or keysym(base))
            state = app.until(lambda s: s["doc"] == want and not s["composing"])
            cases.append(dict(keys=[dead, base], doc=state["doc"], want=want, requested=requested, edits=state["edits"],
                              composition=[i for i in state["inputs"] if i["type"].startswith("composition")]))
        once = lambda c: c["doc"] == c["want"] and sum(e["inserted"] == [c["want"]] for e in c["edits"]) == 1
        return check("dead keys", all(once(c) for c in cases), layout=layout, cases=cases)


    def b_ime():
        # b) one composed input through ibus: Ctrl+Shift+U hex entry, committed with space.
        with open(f"/proc/{app.child.pid}/maps") as maps:
            ibus_module = any("im-ibus" in line for line in maps)
        engine = subprocess.run(["ibus", "engine"], capture_output=True, text=True).stdout.strip()
        reset()
        keys.tap(keysym("u"), CTRL + SHIFT)
        keys.type("2603")
        preedit = app.command("state")
        keys.tap(XK["space"])
        state = app.until(lambda s: s["doc"] == "☃" and not s["composing"])
        composition = [i for i in state["inputs"] if i["type"].startswith("composition")]
        return check("IME (ibus)", state["doc"] == "☃" and ibus_module and
                                 any(i["type"] == "compositionend" and i["data"] == "☃" for i in composition),
                                 real_ime=ibus_module, ibus_engine=engine, doc=state["doc"],
                                 preedit_doc=preedit["doc"], composition=composition, edits=state["edits"])


    def c_undo():
        # c) undo: a typed word goes in one Ctrl+Z and comes back with Ctrl+Shift+Z.
        reset("seed ")
        keys.type("word")
        typed = app.until(lambda s: s["doc"] == "seed word")
        keys.tap(keysym("z"), CTRL)
        undone = app.until(lambda s: s["doc"] == "seed ")
        keys.tap(keysym("z"), CTRL + SHIFT)
        redone = app.until(lambda s: s["doc"] == "seed word")
        return check("undo", (typed["doc"], undone["doc"], redone["doc"]) == ("seed word", "seed ", "seed word"),
                                  typed=typed["doc"], after_undo=undone["doc"], after_redo=redone["doc"],
                                  undo_depth=[typed["undo"], undone["undo"], redone["undo"]],
                                  typed_events=[i for i in typed["inputs"] if i["type"] == "beforeinput"])


    def d_accessibility():
        # d) accessibility: what Orca reads while typing "abc".
        reset("x ")
        a11y.find_editor()
        pump(.5)
        a11y.events.clear()
        keys.type("abc")
        app.until(lambda s: s["doc"] == "x abc")
        pump(1)
        events = [e for e in a11y.events if e["role"] != "frame"]
        text, caret = a11y.text()
        role = a11y.editor.get_role_name()
        # Orca follows the focused editor accessible: each typed character must come with an
        # insert event and a caret event from it, at its own offsets ("x " + abc: 2..4, caret 3..5).
        mine = [e for e in events if e["role"] == role]
        inserted = [(e["detail1"], e["text"]) for e in mine if e["type"] == "object:text-changed:insert"]
        carets = [e["detail1"] for e in mine if e["type"] == "object:text-caret-moved"]
        return check("accessibility", inserted == [(2, "a"), (3, "b"), (4, "c")] and carets == [3, 4, 5]
                     and "x abc" in text and caret == 5, editor_role=role,
                     text=text, caret=caret, inserted=inserted, carets=carets, events=events)


    def e_editing():
        # e) autocompletion, multiple cursors, read-only, paste.
        reset("#")
        keys.type("le")
        offered = app.until(lambda s: s["completion"] == "active" and s["completions"], timeout=8)
        pump(.3)  # past CodeMirror's 75 ms interactionDelay, as a person reading the list would be
        keys.tap(XK["Return"])
        accepted = app.until(lambda s: s["doc"] != "#le", timeout=3)
        completion_ok = (offered["completion"] == "active" and accepted["doc"].startswith("#")
                         and accepted["doc"] not in ("#le", "#le\n") and accepted["completion"] is None)

        reset("aa\nbb", anchor=0)
        rect = app.command("coords", pos=3)["rect"]
        mouse = Pointer()
        try:
            pointer = pointer_to(app, mouse, native, (rect["left"] + 1, (rect["top"] + rect["bottom"]) / 2))
            keys.guard()
            mouse.click(CTRL)
        finally:
            mouse.close()
        cursors = app.command("state")
        keys.type("X")
        multi = app.until(lambda s: s["doc"] in ("Xaa\nXbb", "aa\nXbb"))
        clicks = [i for i in cursors["inputs"] if i["type"] == "mousedown"]
        # The product editor decides: with allowMultipleSelections off (inkwell today), Ctrl+click
        # only moves the caret, so one range and one X; with it on, two ranges and two X.
        many = cursors["multipleSelections"]
        multi_ok = ([c.get("ctrl") for c in clicks] == [True] and len(cursors["ranges"]) == (2 if many else 1)
                    and multi["doc"] == ("Xaa\nXbb" if many else "aa\nXbb"))

        reset("ro")
        app.command("readonly", on=True)
        keys.type("q")
        pump(.5)
        readonly = app.command("state")
        app.command("readonly", on=False)
        readonly_ok = readonly["readOnly"] and readonly["doc"] == "ro" and not readonly["edits"]

        reset("abc")
        keys.tap(XK["Home"], SHIFT)
        keys.tap(keysym("c"), CTRL)
        keys.tap(XK["End"])
        keys.tap(keysym("v"), CTRL)
        pasted = app.until(lambda s: s["doc"] == "abcabc")
        paste_ok = pasted["doc"] == "abcabc"
        return check("completion, cursors, read-only, paste",
                                     completion_ok and multi_ok and readonly_ok and paste_ok,
                                     completion=dict(passed=completion_ok, status=offered["completion"],
                                                     offered=offered["completions"][:8], doc=accepted["doc"]),
                                     multi_cursor=dict(passed=multi_ok, multipleSelections=many, pointer=pointer, clicks=clicks, ranges=cursors["ranges"], doc=multi["doc"]),
                                     read_only=dict(passed=readonly_ok, readOnly=readonly["readOnly"], doc=readonly["doc"],
                                                    inputs=readonly["inputs"]),
                                     paste=dict(passed=paste_ok, doc=pasted["doc"],
                                                inputs=[i for i in pasted["inputs"] if i["type"] == "beforeinput"]))
        return results

    results = {}
    for section in (a_dead_keys, b_ime, c_undo, d_accessibility, e_editing):
        try:
            results[section.__name__] = section()
        except Exception as error:  # one broken check must not hide the others
            results[section.__name__] = check(section.__name__, False, error=f"{type(error).__name__}: {error}")
    return results


def run_mode(mode, binary, native_factory):
    with tempfile.TemporaryDirectory(prefix=f"typing-check-{mode}-", dir=ROOT / ".task") as state:
        app = App(binary, Path(state))
        native = a11y = None
        try:
            ready = app.wait({"ready"}, 60)
            native = native_factory()
            native.activate(app.child.pid)
            if native.focus_pid() != app.child.pid:
                raise CheckError("window activation did not give it the focus")
            first = app.until(lambda s: s.get("ready"), timeout=20)
            if not first.get("ready"):
                raise CheckError("the editor never appeared")
            if first["ownsTyping"] != (mode == "on"):
                raise CheckError(f"binary built for {mode} reports ownsTyping={first['ownsTyping']}")
            a11y = Accessibility(app.child.pid)
            keys = Keys(native, app.child.pid)
            results = run_checks(app, keys, native, a11y)
            return dict(mode=mode, ownsTyping=first["ownsTyping"], ready=ready.get("ready"), checks=results)
        finally:
            if a11y:
                a11y.close()
            if native:
                native.close()
            app.close()


def same(on, off):
    """ON must give OFF's results; only these fields are compared (timing-free)."""
    def key(results):
        a, b, c, d, e = (results[k] for k in ("a_dead_keys", "b_ime", "c_undo", "d_accessibility", "e_editing"))
        return dict(dead=[x["doc"] for x in a["cases"]], ime=b["doc"],
                    undo=[c["typed"], c["after_undo"], c["after_redo"]],
                    a11y=[(x["type"], x["detail1"], x["detail2"], x["text"], x["role"]) for x in d["events"]],
                    a11y_text=d["text"],
                    editing=[e["completion"]["doc"], e["multi_cursor"]["doc"], e["read_only"]["doc"], e["paste"]["doc"]])
    left, right = key(on), key(off)
    return {k: dict(on=left[k], off=right[k]) for k in left if left[k] != right[k]}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--modes", default="on,off", help="comma-separated: on, off")
    parser.add_argument("--no-build", action="store_true", help="reuse bin/inkwell-typing-check-<mode>")
    parser.add_argument("--output", type=Path, default=ROOT / "gates/L3-setup/typing-checks.json")
    args = parser.parse_args(argv)
    if os.environ.get("INKWELL_NATIVE_PERF_RUN") != "1":
        parser.error("opens a real window and types into it: set INKWELL_NATIVE_PERF_RUN=1 on the graphical host")
    modes = args.modes.split(",")
    (ROOT / ".task").mkdir(exist_ok=True)
    if not args.no_build:
        subprocess.run(["mise", "run", "fe:bindings"], cwd=ROOT, check=True)
    from native_input import MutterKeyboard
    report = dict(status="fail", modes={})
    try:
        for mode in modes:
            binary = ROOT / f"bin/inkwell-typing-check-{mode}" if args.no_build else build(mode)
            try:
                report["modes"][mode] = run_mode(mode, binary, MutterKeyboard)
            except Exception as error:
                report["modes"][mode] = dict(mode=mode, error=f"{type(error).__name__}: {error}")
        passed = all("checks" in m and all(c["passed"] for c in m["checks"].values()) for m in report["modes"].values())
        if passed and {"on", "off"} <= set(report["modes"]):
            report["differences"] = same(report["modes"]["on"]["checks"], report["modes"]["off"]["checks"])
            passed = not report["differences"]
        report["status"] = "pass" if passed else "fail"
    finally:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
    for mode, result in report["modes"].items():
        summary = result.get("error") or ", ".join(f"{k}={'pass' if v['passed'] else 'FAIL'}" for k, v in result["checks"].items())
        print(f"{mode}: {summary}")
    if report.get("differences"):
        print("ON/OFF differences:", ", ".join(report["differences"]))
    print("typing checks:", report["status"], "->", args.output)
    return 0 if report["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
