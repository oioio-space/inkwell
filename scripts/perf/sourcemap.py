"""Resolve the native attribution table's minified frames to source (diagnostic only).

The perf_native build alone emits sourcemaps (`vite build --sourcemap`); the
production asset set is unchanged. Rows keep their raw registration frames; this
fills each row's `source` from the first frame outside the harness, plus the first
application frame (frontend/src, not node_modules). `cm-plugin:*` stage rows are
located by their recorded minified source prefix. Nothing here touches a verdict.
"""
import json
from pathlib import Path
import re
import sys

B64 = {c: i for i, c in enumerate("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")}
# JSC: "fn@url:line:col"; V8 (tests): "    at fn (url:line:col)".
FRAME = re.compile(r"^\s*(?:at\s+)?(?:(?P<fn>[^@(\s]*)[@\s(]+)?(?P<url>\S+?):(?P<line>\d+):(?P<col>\d+)\)?$")


def vlq(segment):
    values, shift, value = [], 0, 0
    for char in segment:
        digit = B64[char]
        value += (digit & 31) << shift
        if digit & 32:
            shift += 5
            continue
        values.append(-(value >> 1) if value & 1 else value >> 1)
        shift, value = 0, 0
    return values


class SourceMap:
    """Version-3 map: generated (line, column), both 0-based, to source/line/name."""

    def __init__(self, data):
        self.sources = [short(s) for s in data["sources"]]
        self.names = data.get("names", [])
        self.lines = []
        src = sline = scol = name = 0
        for text in data["mappings"].split(";"):
            col, segments = 0, []
            for seg in filter(None, text.split(",")):
                v = vlq(seg)
                col += v[0]
                if len(v) >= 4:
                    src, sline, scol = src + v[1], sline + v[2], scol + v[3]
                    if len(v) >= 5:
                        name += v[4]
                    segments.append((col, src, sline, scol, name if len(v) >= 5 else None))
            self.lines.append(segments)

    def lookup(self, line, col):
        best = None
        for seg in self.lines[line] if 0 <= line < len(self.lines) else ():
            if seg[0] > col:
                break
            best = seg
        if best is None:
            return None
        _, src, sline, _, name = best
        where = f"{self.sources[src]}:{sline + 1}"
        return where + (f" ({self.names[name]})" if name is not None else "")


def short(path):
    path = path.replace("\\", "/")
    if "node_modules/" in path:
        return "node_modules/" + path.rsplit("node_modules/", 1)[1]
    return path[path.index("src/"):] if "src/" in path else path.lstrip("./")


class Resolver:
    def __init__(self, dist):
        self.assets = Path(dist) / "assets"
        self.maps, self.texts = {}, {}

    def _map(self, name):
        if name not in self.maps:
            path = self.assets / (name + ".map")
            self.maps[name] = SourceMap(json.loads(path.read_text())) if path.is_file() else None
        return self.maps[name]

    def frame(self, line):
        match = FRAME.match(line)
        if not match or "/assets/" not in match["url"]:
            return None
        sm = self._map(match["url"].rsplit("/assets/", 1)[1].split("?")[0])
        return sm.lookup(int(match["line"]) - 1, int(match["col"]) - 1) if sm else None

    def snippet(self, text):
        """Locate a unique minified source prefix in the JS assets."""
        hits = []
        for js in sorted(self.assets.glob("*.js")):
            body = self.texts.setdefault(js.name, js.read_text(errors="replace"))
            at = body.find(text)
            if at >= 0 and body.find(text, at + 1) < 0:
                line = body.count("\n", 0, at)
                hits.append((js.name, line, at - (body.rfind("\n", 0, at) + 1)))
        if len(hits) != 1:
            return None
        sm = self._map(hits[0][0])
        return sm.lookup(hits[0][1], hits[0][2]) if sm else None


def describe(resolved):
    outside = [r for r in resolved if r and not r.startswith("src/perf/")]
    app = next((r for r in outside if r.startswith("src/")), None)
    if not outside:
        return None
    return outside[0] + (f" | app: {app}" if app and app != outside[0] else "")


def annotate(report, dist):
    table = report.get("attribution_table")
    if not table:
        return 0
    resolver, plugins, count = Resolver(dist), table.get("plugin_sources") or {}, 0
    for phase in (table.get("phases") or {}).values():
        for row in phase.get("rows") or []:
            source = None
            if row.get("registration"):
                source = describe([resolver.frame(f) for f in row["registration"].split("\n")])
            label = row.get("label", "")
            if source is None and label.startswith("stage cm-plugin:"):
                text = plugins.get(label.removeprefix("stage "))
                source = resolver.snippet(text) if text else None
            if source:
                row["source"] = source
                count += 1
    return count


def main(argv):
    path, dist = Path(argv[1]), Path(argv[2])
    report = json.loads(path.read_text())
    count = annotate(report, dist)
    path.write_text(json.dumps(report, indent=2) + "\n")
    print(f"attribution: resolved {count} row sources from {dist}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
