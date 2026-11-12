"""Generate PM-1 inputs, attempt a real server build, record unsupported measurements."""
import hashlib
import json
import os
from pathlib import Path
import platform
import math
import shutil
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import paired  # noqa: E402
import refbuild  # noqa: E402

THRESHOLDS = Path("docs/perf/baseline/2026-11-02/thresholds.json")
WORK = Path(".task/perf/paired")  # binaries and homes of the two sides: equal-length paths, outside the evidence folder
PORT = 39128


def start_record(ref, branch_sha, result):
    """The PERF-START entry (ADR 0038): the paired verdict against the merge-base build, then the absolute
    startup budget on the change's judged starts. Both must pass; only invalid attempts were retried."""
    budget = result.get("budget")
    ok = result["outcome"] == "pass" and budget is not None and budget["status"] == "pass"
    status = "pass" if ok else "invalid" if result["outcome"] == "invalid" else "fail"
    return {"status": status, "design": "paired-randomized", "scope": "Today's app cold process, server build to usable editor; "
            "prelaunched Chromium and warm OS caches, not native window timing.", "plugins": 0,
            "reference": {k: ref[k] for k in ("commit", "cache_digest", "sha256", "cache_hit", "build_seconds")},
            "branch": {"sha256": branch_sha}, "identity": ref["sha256"] == branch_sha,
            "outcome": result["outcome"], "reasons": result["reasons"],
            "attempts": result["attempts"], "interference": result.get("interference"),
            "verdict": result["verdict"], "budget": budget,
            "e2e_multiplier": json.loads(THRESHOLDS.read_text())["e2e_multiplier"]}


if os.environ.get("INKWELL_BASELINE_SELF_TEST") == "1":
    print("baseline self-test passed: " + paired.self_test())
    raise SystemExit(0)

out = Path(os.environ.get("INKWELL_PERF_OUT", "gates/L0"))
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
(out / "corpus.json").write_text(json.dumps({
    "version": 1, "generator": "python3 scripts/perf/baseline.py + scripts/perf/automerge.cjs",
    "text_bytes": 2097152, "text_characters": 2097152, "automerge_operations": 50000,
    "memory_blobs": "blobs/00.bin through blobs/19.bin (20 MiB)",
    "git_benchmark_blobs": "all 50 blobs (50 MiB)", "files": files}, indent=2)+"\n")
# The change under test is built by the function that builds the reference (refbuild.build_server): one
# command vector for both sides. bin/inkwell-server stays the artefact of the other L0 steps.
attempts = []
try:
    with (out / "browser-attempt-0.log").open("w") as log:
        refbuild.build_server(Path.cwd(), Path("bin/inkwell-server"), log=log)
    attempts.append({"command": ["refbuild.build_server", "bin/inkwell-server"], "exit_code": 0})
except refbuild.ReferenceBuildError as error:
    raise SystemExit(f"baseline command failed: {error}")
commands = [["node", "scripts/perf/browser.cjs"]]
for i, command in enumerate(commands, 1):
    with (out / f"browser-attempt-{i}.log").open("w") as log:
        result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT)
    attempts.append({"command": command, "exit_code": result.returncode})
    if result.returncode:
        break
report = {
    "corpus": "testdata/perf/corpus.json version 1",
    "attempts": attempts,
    "PERF-TYPE": {"status": "proxy", "reason": "Warm Chromium EventTiming only; censored and rounded entries cannot certify native WebKitGTK keystroke-to-paint p95; native measurement deferred to L2."},
    "PERF-START": {"status": "unmeasured", "plugins": 0, "scope": "Today's app cold process, server build to usable editor; prelaunched Chromium and warm OS caches, not native window timing."},
    "PERF-MEM": {"status": "deferred", "target_lot": "L2", "reason": "Real corpus opening requires G-0 in L2."},
    "PERF-START-plugins": {"status": "deferred", "target_lot": "Fondations-extensibles", "reason": "Pinned plugins do not exist yet; plugin-load measurement belongs to the plugin lot."},
    # This script compares nothing for PM-5: the verdict is the regression-guard criterion.
    "PM-5": {"status": "measured", "verdict": "gate criterion regression-guard (scripts/perf/regression.py --bench)",
             "thresholds": "docs/perf/baseline/2026-11-02/thresholds.json"}}
if len(attempts) != len(commands) + 1 or any(a["exit_code"] for a in attempts):
    raise SystemExit("baseline command failed")
# PERF-START, paired (ADR 0038): the merge-base build (cached by commit and toolchain), then 2 + 80 pairs in a
# random order, then the absolute startup budget. The hot path of every later gate.
try:
    reference = refbuild.ensure_reference(Path.cwd(), refbuild.default_cache())
except refbuild.ReferenceBuildError as error:
    raise SystemExit(f"reference build failed: {error}")
shutil.rmtree(WORK, ignore_errors=True)
for side, source in (("a", reference["binary"]), ("b", "bin/inkwell-server")):
    (WORK / side).mkdir(parents=True)
    shutil.copy2(source, WORK / side / refbuild.BINARY)
branch_sha = refbuild.sha256(WORK / "b" / refbuild.BINARY)
# Equal-length paths and environment values on both sides: directories differ by one letter.
measured = paired.run_criterion(out, str(WORK / "a" / refbuild.BINARY), str(WORK / "b" / refbuild.BINARY), port=PORT, work=WORK)
final_slots = json.loads(Path(measured["attempts"][-1]["raw"]).read_text())["slots"]
report["PERF-START"] = start_record(reference, branch_sha, measured)
samples = [b for _, b in paired.judged_pairs(final_slots)]
if samples:
    report["PERF-START"]["measurement"] = {"samples_ms": samples, "p95_ms": sorted(samples)[math.ceil(len(samples) * 0.95) - 1],
                                           "plugins": 0, "scope": report["PERF-START"]["scope"]}
    (out / "start.json").write_text(json.dumps(report["PERF-START"]["measurement"], indent=2))
diagnostic = out / "type-diagnostic.json"
if diagnostic.exists():
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
assert report["PERF-START"]["status"] == "pass", report["PERF-START"]
raise SystemExit(0)
