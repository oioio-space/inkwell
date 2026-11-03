"""Attempt the real Wails desktop; capture readiness evidence, never proxy paint latency.

Requires an X11 display, libX11, and Pillow. No host-window titles are retained.
This probe cannot certify PERF-TYPE; a blank native WebView is a blocking finding.
"""
import ctypes as c
import os
from pathlib import Path
import subprocess
import tempfile
import time
from PIL import Image

out = Path(os.environ.get("INKWELL_PERF_OUT", "gates/L2"))
out.mkdir(parents=True, exist_ok=True)
x = c.CDLL("libX11.so.6")
x.XOpenDisplay.restype = c.c_void_p
x.XDefaultRootWindow.argtypes = [c.c_void_p]
x.XDefaultRootWindow.restype = c.c_ulong
x.XQueryTree.argtypes = [c.c_void_p, c.c_ulong, c.POINTER(c.c_ulong), c.POINTER(c.c_ulong), c.POINTER(c.POINTER(c.c_ulong)), c.POINTER(c.c_uint)]
x.XFetchName.argtypes = [c.c_void_p, c.c_ulong, c.POINTER(c.c_char_p)]
x.XGetGeometry.argtypes = [c.c_void_p, c.c_ulong, c.POINTER(c.c_ulong), c.POINTER(c.c_int), c.POINTER(c.c_int), c.POINTER(c.c_uint), c.POINTER(c.c_uint), c.POINTER(c.c_uint), c.POINTER(c.c_uint)]

class XImage(c.Structure):
    _fields_ = [("width", c.c_int), ("height", c.c_int), ("xoffset", c.c_int), ("format", c.c_int), ("data", c.c_void_p), ("byte_order", c.c_int), ("bitmap_unit", c.c_int), ("bitmap_bit_order", c.c_int), ("bitmap_pad", c.c_int), ("depth", c.c_int), ("bytes_per_line", c.c_int), ("bits_per_pixel", c.c_int)]

x.XGetImage.argtypes = [c.c_void_p, c.c_ulong, c.c_int, c.c_int, c.c_uint, c.c_uint, c.c_ulong, c.c_int]
x.XGetImage.restype = c.POINTER(XImage)
x.XDestroyImage.argtypes = [c.POINTER(XImage)]
x.XFree.argtypes = [c.c_void_p]
x.XCloseDisplay.argtypes = [c.c_void_p]
display = x.XOpenDisplay(None)
if not display:
    raise SystemExit("No X11 display; no native measurements")
with tempfile.TemporaryDirectory(prefix="inkwell-native-") as directory:
    runtime = Path(directory, "runtime")
    runtime.mkdir(mode=0o700)
    env = dict(os.environ, XDG_RUNTIME_DIR=str(runtime), GDK_BACKEND="x11", GSK_RENDERER="cairo", INKWELL_RENDEZVOUS="off", INKWELL_HOME=str(Path(directory, "home")), INKWELL_DOCS=str(Path(directory, "docs")))
    with (out / "native-probe.log").open("w") as log:
        child = subprocess.Popen(["./bin/inkwell"], env=env, stdout=log, stderr=log)
        try:
            time.sleep(15)
            root = x.XDefaultRootWindow(display)
            parent, returned = c.c_ulong(), c.c_ulong()
            children, count = c.POINTER(c.c_ulong)(), c.c_uint()
            x.XQueryTree(display, root, c.byref(returned), c.byref(parent), c.byref(children), c.byref(count))
            for window in children[:count.value]:
                name = c.c_char_p()
                x.XFetchName(display, window, c.byref(name))
                owned = name.value == b"Inkwell"
                if name:
                    x.XFree(name)
                if not owned:
                    continue
                left, top = c.c_int(), c.c_int()
                width, height, border, depth = c.c_uint(), c.c_uint(), c.c_uint(), c.c_uint()
                x.XGetGeometry(display, window, c.byref(returned), c.byref(left), c.byref(top), c.byref(width), c.byref(height), c.byref(border), c.byref(depth))
                image = x.XGetImage(display, window, 0, 0, width.value, height.value, 0xffffffff, 2)
                if not image:
                    raise RuntimeError("Native window cannot be captured")
                try:
                    if image.contents.bits_per_pixel != 32 or image.contents.byte_order != 0:
                        raise RuntimeError("Unsupported native pixel layout")
                    raw = c.string_at(image.contents.data, image.contents.bytes_per_line * image.contents.height)
                    Image.frombytes("RGB", (width.value, height.value), raw, "raw", "BGRX", image.contents.bytes_per_line).save(out / "native-window.png")
                    (out / "native-window-probe.log").write_text(f"Native window after 15 seconds: {width.value}x{height.value}; inspect native-window.png for editor readiness. No input samples collected.\n")
                finally:
                    x.XDestroyImage(image)
                break
            else:
                raise RuntimeError("Native window not found after 15 seconds")
            x.XFree(children)
        finally:
            child.terminate()
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()
x.XCloseDisplay(display)
