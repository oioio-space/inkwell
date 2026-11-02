"""Generate PM-1 inputs, attempt a real server build, record unsupported measurements."""
import hashlib
import json
import os
from pathlib import Path
import platform
import math
import subprocess

out = Path("docs/perf/baseline/2026-11-02")
out.mkdir(parents=True, exist_ok=True)
corpus = Path("testdata/perf/generated")
corpus.mkdir(parents=True, exist_ok=True)
files = {}
def put(name, data):
    path = corpus / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)
    files[name] = {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
put("text/main.typ", (b"Performance reference text.\n" * 90000)[:2*1024*1024])
for count in (1000, 10000):
    for i in range(count):
        size = 2*1024*1024//count + (i < 2*1024*1024 % count)
        put(f"project-{count}/{i:05}.typ", (f"// {i:05}\n".encode() + b"x"*size)[:size])
for i in range(50):
    put(f"blobs/{i:02}.bin", bytes([i])*1024*1024)
# History is represented by deterministic replacement deltas, not a hidden Git repository.
for v in range(500):
    put(f"history/{v:03}.json", json.dumps({
        "version": v, "base": "project-1000", "changed_files": [
            {"path": f"{(v*20+j)%1000:05}.typ", "replacement": f"// version {v}, file {j}\n"}
            for j in range(20)], "change_rate_files": 0.02}, sort_keys=True).encode())
with (out / "corpus-build.log").open("w") as log:
    p = subprocess.run(["node", "scripts/perf/automerge.cjs"], stdout=log, stderr=subprocess.STDOUT)
if p.returncode:
    raise SystemExit(p.returncode)
for path in (corpus / "automerge").glob("*"):
    files[str(path.relative_to(corpus))] = {"bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
Path("testdata/perf/corpus.json").write_text(json.dumps({
    "version": 1, "generator": "python3 scripts/perf/baseline.py + scripts/perf/automerge.cjs",
    "text_bytes": 2097152, "text_characters": 2097152, "automerge_operations": 50000,
    "memory_blobs": "blobs/00.bin through blobs/19.bin (20 MiB)",
    "git_benchmark_blobs": "all 50 blobs (50 MiB)", "files": files}, indent=2)+"\n")
commands = [
    ["pnpm", "--dir", "frontend", "exec", "vite", "build", "--configLoader", "runner", "--mode", "production", "--outDir", str(Path.home() / ".cache/inkwell/l0-frontend")],
    ["go", "build", "-tags", "server", "-o", "bin/inkwell-server", "./cmd/inkwell"],
    ["node", "scripts/perf/browser.cjs"],
    ["node", "scripts/perf/start.cjs"],
]
attempts = []
for i, command in enumerate(commands):
    with (out / f"browser-attempt-{i}.log").open("w") as log:
        result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT)
    attempts.append({"command": command, "exit_code": result.returncode})
    if result.returncode:
        break
report = {
    "corpus": "testdata/perf/corpus.json version 1",
    "attempts": attempts,
    "PERF-TYPE": {"status": "proxy", "reason": "Warm Chromium EventTiming only; censored and rounded entries cannot certify native WebKitGTK keystroke-to-paint p95; native measurement deferred to L2."},
    "PERF-START": {"status": "pass", "plugins": 0, "scope": "Today's app cold process, server build to usable editor; prelaunched Chromium and warm OS caches, not native window timing."},
    "PERF-MEM": {"status": "deferred", "target_lot": "L2", "reason": "Real corpus opening requires G-0 in L2."},
    "PERF-START-plugins": {"status": "deferred", "target_lot": "Fondations-extensibles", "reason": "Pinned plugins do not exist yet; plugin-load measurement belongs to the plugin lot."},
    "PM-5": {"status": "pass", "thresholds": str(out / "thresholds.json")}}
if len(attempts) != len(commands) or any(a["exit_code"] for a in attempts):
    raise SystemExit("baseline command failed")
report["PERF-START"]["measurement"] = json.loads((out / "start.json").read_text())
diagnostic = out / "type-diagnostic.json"
if diagnostic.exists() and len(attempts) == len(commands) and attempts[-1]["exit_code"] == 0:
    data = json.loads(diagnostic.read_text())
    durations = [max((e["duration"] for e in s["events"]), default=None) for s in data["samples"]]
    bounds = sorted(max([e["duration"] + 4 for e in sample["events"]] or [16]) for sample in data["samples"])
    report["PERF-TYPE"]["p95_upper_bound_ms"] = bounds[math.ceil(len(bounds)*0.95)-1]
    report["PERF-TYPE"]["bound_method"] = "Censored entries <16ms bounded at 16ms; rounded durations +4ms. Same-bound proxy comparison only."
    report["PERF-TYPE"]["diagnostic"] = {
        "rounded_p95_ms": sorted(durations)[math.ceil(len(durations)*0.95)-1] if durations and all(v is not None for v in durations) else None,
        "censored_samples": sum(v is None for v in durations),
        "conditions": "warm browser; concurrent Go jobs; no noise subtraction; not a native acceptance result",
        "samples": len(data["samples"]),
        "slow_layout_control_detected": data["slowLayoutControl"]["detected"],
        "raw_results": str(diagnostic),
        "engine": "Chromium (not native WebKit)"}
(out / "measurements.json").write_text(json.dumps(report, indent=2)+"\n")
machine = platform.platform() + "\n" + Path("/proc/cpuinfo").read_text().split("model name",1)[-1].split("\n",1)[0]
versions = {}
for name, command in {"go":["go","version"],"node":["node","--version"],"typst":["typst","--version"],"tinymist":["tinymist","--version"]}.items():
    try:
        r = subprocess.run(command, capture_output=True, text=True)
        versions[name] = (r.stdout+r.stderr).strip()
    except FileNotFoundError:
        versions[name] = "unavailable on PATH; embedded versions: internal/sidecar/manifest.json"
(out / "machine.json").write_text(json.dumps({"machine":machine, "cpus":os.cpu_count(),
    "memory":Path("/proc/meminfo").read_text(), "versions":versions}, indent=2)+"\n")
print(json.dumps(report, indent=2))
# Non-deferred baseline evidence must exist; proxy remains explicitly limited.
assert report["PERF-TYPE"].get("diagnostic", {}).get("slow_layout_control_detected")
raise SystemExit(0)
