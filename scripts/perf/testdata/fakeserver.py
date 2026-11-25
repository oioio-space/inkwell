#!/usr/bin/env python3
"""A fake inkwell server for the driver tests of block PB: `/health` and a page with a `.cm-content`
element on INKWELL_SERVER_PORT, after a start delay, and the readiness line of Wails' server mode on stderr once
it listens. Standard library only.

  fakeserver.py [--delay-ms N] [--noise-ms N] [--editor-ms N] [--silent] [--crash] [--record FILE] [--self PATH]

--editor-ms inserts the `.cm-content` element N ms after the page's script starts instead of serving it in the page
(block PF2: the driver must time it on the page's clock, not on its polling); --silent listens without the line;

--noise-ms adds a uniform random delay of 0..N ms to every start; --crash exits at once with status 3;
--record appends one JSON line per start with the path it was started as and the environment variables
the driver sets, so that a test can compare the two sides.
"""

import argparse
import http.server
import json
import os
import random
import sys
import time

PAGE = b"<!doctype html><title>fake</title><div class='cm-content' contenteditable>fake editor</div>"
LATE = ("<!doctype html><title>fake</title><script>setTimeout(() => {{ const e = document.createElement('div');"
        " e.className = 'cm-content'; e.contentEditable = 'true'; e.textContent = 'fake editor';"
        " document.body.append(e); }}, {ms});</script><body></body>")


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"ok" if self.path == "/health" else self.server.page
        self.send_response(200)
        self.send_header("Content-Type", "text/plain" if self.path == "/health" else "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--delay-ms", type=float, default=0)
    parser.add_argument("--noise-ms", type=float, default=0)
    parser.add_argument("--editor-ms", type=float)
    parser.add_argument("--silent", action="store_true")
    parser.add_argument("--crash", action="store_true")
    parser.add_argument("--record")
    parser.add_argument("--self", dest="me", default=sys.argv[0])
    args = parser.parse_args()
    if args.crash:
        return 3
    if args.record:
        keys = ("INKWELL_SERVER_PORT", "INKWELL_RENDEZVOUS", "INKWELL_HOME", "INKWELL_DOCS")
        with open(args.record, "a") as f:
            f.write(json.dumps(dict(self=args.me, env={k: os.environ.get(k) for k in keys})) + "\n")
    time.sleep((args.delay_ms + random.uniform(0, args.noise_ms)) / 1000)
    address = ("127.0.0.1", int(os.environ["INKWELL_SERVER_PORT"]))
    server = http.server.ThreadingHTTPServer(address, Handler)
    server.page = PAGE if args.editor_ms is None else LATE.format(ms=args.editor_ms).encode()
    if not args.silent:
        print(f"1:04AM INF Server mode starting address={address[0]}:{address[1]}", file=sys.stderr, flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
