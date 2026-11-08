import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import sourcemap  # noqa: E402

CHARS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"


def encode(values):
    out = ""
    for value in values:
        v = (-value << 1) | 1 if value < 0 else value << 1
        while True:
            digit, v = v & 31, v >> 5
            out += CHARS[digit | (32 if v else 0)]
            if not v:
                break
    return out


class SourceMapTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        assets = Path(self.dir.name) / "assets"
        assets.mkdir()
        body = "a();function u(e){return e+1}b.addEventListener(\"input\",f)"
        (assets / "App-x.js").write_text(body)
        # Generated col 0 -> src/perf/native.ts:1; col 4 -> App.svelte:40 (update);
        # col 25 -> node_modules/@codemirror/view index.js:7.
        mappings = ",".join([encode([0, 0, 0, 0]), encode([4, 1, 39, 2, 0]), encode([21, 1, -33, 0])])
        (assets / "App-x.js.map").write_text(json.dumps({
            "version": 3,
            "sources": ["../../src/perf/native.ts", "../../src/App.svelte",
                        "../../node_modules/.pnpm/x/node_modules/@codemirror/view/dist/index.js"],
            "names": ["update"],
            "mappings": mappings,
        }))
        self.dist = Path(self.dir.name)

    def tearDown(self):
        self.dir.cleanup()

    def test_vlq_round_trip(self):
        for values in ([0], [1, -1, 15, -16, 1000, -12345], [0, 0, 39, 2, 0]):
            self.assertEqual(sourcemap.vlq(encode(values)), values)

    def test_frames_skip_the_harness_and_name_the_app_frame(self):
        frames = "\n".join([
            "listenerFor@wails://localhost/assets/App-x.js:1:1",
            "@wails://localhost/assets/App-x.js:1:31",
            "u@wails://localhost/assets/App-x.js:1:12",
        ])
        r = sourcemap.Resolver(self.dist)
        resolved = [r.frame(f) for f in frames.split("\n")]
        self.assertEqual(resolved[0], "src/perf/native.ts:1")
        self.assertEqual(resolved[1], "node_modules/@codemirror/view/dist/index.js:7")
        self.assertEqual(resolved[2], "src/App.svelte:40 (update)")
        self.assertEqual(sourcemap.describe(resolved),
                         "node_modules/@codemirror/view/dist/index.js:7 | app: src/App.svelte:40 (update)")

    def test_annotate_fills_listener_and_plugin_rows_only(self):
        report = {"status": "fail", "attribution_table": {
            "plugin_sources": {"cm-plugin:3:e": "function u(e){return e+1}"},
            "phases": {"warm": {"rows": [
                {"label": "listener view.contentDOM input",
                 "registration": "x@wails://localhost/assets/App-x.js:1:12"},
                {"label": "stage cm-plugin:3:e"},
                {"label": "stage automerge-ui"},
            ]}}}}
        self.assertEqual(sourcemap.annotate(report, self.dist), 2)
        rows = report["attribution_table"]["phases"]["warm"]["rows"]
        self.assertEqual(rows[0]["source"], "src/App.svelte:40 (update)")
        self.assertEqual(rows[1]["source"], "src/App.svelte:40 (update)")
        self.assertNotIn("source", rows[2])
        self.assertEqual(report["status"], "fail")  # Never touches the verdict.

    def test_ambiguous_or_missing_snippet_is_not_guessed(self):
        r = sourcemap.Resolver(self.dist)
        self.assertIsNone(r.snippet("e"))  # Not unique.
        self.assertIsNone(r.snippet("absent()"))


if __name__ == "__main__":
    unittest.main()
