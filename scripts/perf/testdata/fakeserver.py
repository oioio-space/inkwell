#!/usr/bin/env python3
"""A fake inkwell server for the driver tests of block PB: `/health` and a page with a `.cm-content`
element on INKWELL_SERVER_PORT, after a start delay. Standard library only.

  fakeserver.py [--delay-ms N] [--noise-ms N] [--crash] [--record FILE] [--self PATH]

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


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"ok" if self.path == "/health" else PAGE
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
    server = http.server.ThreadingHTTPServer(("127.0.0.1", int(os.environ["INKWELL_SERVER_PORT"])), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
