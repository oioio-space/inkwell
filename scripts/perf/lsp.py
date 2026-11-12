"""Measure real service and browser Stream roundtrips; fail closed on missing samples."""
import json
import math
import os
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import refbuild  # noqa: E402  (the pinned Go, never the first `go` on PATH)

ROOT = Path(__file__).resolve().parents[2]
out = Path(os.environ.get("INKWELL_PERF_OUT", ROOT / "gates/L3"))
out.mkdir(parents=True, exist_ok=True)
env = dict(refbuild.tree_go_env(ROOT), INKWELL_PERF_OUT=str(out.resolve()), INKWELL_LSP_PERF="1")
reports = [out / "PERF-LSP.json", out / "PERF-LSP-browser.json"]
for report in reports:
    report.unlink(missing_ok=True)
subprocess.run(["go", "test", "./internal/language", "-run", "^$", "-bench", "^BenchmarkLanguageLatency$", "-benchtime=1x", "-count=1", "-v"],
               cwd=ROOT, env=env, check=True)
subprocess.run(["mise", "run", "--skip-deps", "e2e:build"], cwd=ROOT, env=env, check=True)
subprocess.run(["node", "scripts/perf/lsp-browser.cjs"], cwd=ROOT, env=env, check=True)
for report in reports:
    data = json.loads(report.read_text())
    if data.get("status") != "pass":
        raise ValueError((report, "unsuccessful language measurement"))
    if data.get("fixture_files", 0) < 4 or data.get("fixture_lines", 0) < 3000 or data.get("edit_cycles") != 40:
        raise ValueError((report, "missing realistic four-file/3000-line/40-edit fixture"))
    reference_revisions = None
    for name in ("completion", "signature", "hover"):
        samples = data[name]["samples_ms"]
        if len(samples) != 40:
            raise ValueError((report, name, "expected 40 samples after actual edits"))
        revisions = data[name]["revisions"]
        if len(revisions) != 40 or not all(type(v) is int and v > 1 for v in revisions):
            raise ValueError((report, name, "invalid edit revisions"))
        if not all(a < b for a, b in zip(revisions, revisions[1:])):
            raise ValueError((report, name, "requests did not interleave edits"))
        if reference_revisions is not None and revisions != reference_revisions:
            raise ValueError((report, name, "feature queries used different edit cycles"))
        reference_revisions = revisions
        positions = data[name].get("offsets", data[name].get("positions", []))
        if len(positions) != 40 or len({json.dumps(p, sort_keys=True) for p in positions}) < 2:
            raise ValueError((report, name, "requests did not vary query positions"))
        if not all(type(x) in (int, float) and math.isfinite(x) and x >= 0 for x in samples):
            raise ValueError((report, name, "invalid samples"))
        measured = sorted(samples)[math.ceil(.95 * len(samples)) - 1]
        if data[name]["p95_ms"] != measured:
            raise ValueError((report, name, "p95 mismatch"))
        if measured > 100:
            raise ValueError((report, name, measured, "exceeds 100ms"))
print("PERF-LSP: service and browser roundtrip p95 <= 100ms for all three operations")
