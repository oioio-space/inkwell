"""Build the tagged native collector; run only with an explicit native-host opt-in.

No display is opened by build-only mode or by the default sandbox refusal. The
Go collector/verifier writes every PERF-TYPE verdict; Python never invents timing
samples or turns rAF opportunities into paint evidence. The dedicated asset set
also supplies the native monotonic handler clock (docs/perf/native-typing.md).
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
import paired  # noqa: E402
import refbuild  # noqa: E402  (the pinned Go, never the first `go` on PATH)

ROOT = Path(__file__).resolve().parents[2]
BINARY = ROOT / "bin/inkwell-native-perf"
EX_TEMPFAIL = 75  # native_input.py: an environment fault (focus lost, delivery lost, activation failed)
ATTEMPTS = paired.ATTEMPTS  # runs per criterion; only an invalid run (an environment fault) is retried


def build(runner=subprocess.run):
    runner(["mise", "run", "fe:bindings"], cwd=ROOT, check=True)
    env = dict(os.environ, VITE_PERF_NATIVE="1", VITE_E2E_OBSERVE="0")
    runner(["pnpm", "--config.verifyDepsBeforeRun=false", "exec", "vite", "build",
            "--configLoader", "runner", "--mode", "production", "--outDir", "perf-dist",
            # perf_native assets only: maps resolve the attribution table's frames offline.
            "--sourcemap"],
           cwd=ROOT / "frontend", env=env, check=True)
    BINARY.parent.mkdir(parents=True, exist_ok=True)
    runner(["go", "build", "-tags", "perf_native", "-buildvcs=false", "-o", str(BINARY),
            "./cmd/inkwell"], cwd=ROOT, env=refbuild.tree_go_env(ROOT), check=True)


def report_status(output):
    """The status the Go verifier wrote for this run, or None when it wrote none."""
    try:
        return json.loads((ROOT / output).read_text()).get("status")
    except (OSError, ValueError):
        return None


def fault_reason(raw):
    """Why the driver stopped with exit 75, from the fault it recorded in driver.json."""
    try:
        fault = json.loads((ROOT / raw / "driver.json").read_text())["fault"]
        return f"{fault['kind']}: {fault['message']}".replace("|", "/")
    except (OSError, ValueError, KeyError, TypeError):
        return "environment fault (exit 75)"


def execute(binary, output, raw, enabled, runner=subprocess.run, input_backend="mutter", cool=paired.cool):
    """The input driver is unreachable without the external-native opt-in. An environment fault (driver exit 75,
    or a coded fault of the webview that the verifier reports as invalid) is retried with a fresh raw
    directory after cooling, at most ATTEMPTS times, then the verdict is `invalid`; a valid failure is never
    retried (plan section 8)."""
    def unavailable(reason):
        runner([str(binary), "--native-perf-unavailable=" + reason,
                "--native-perf-out=" + str(output)], cwd=ROOT, check=False)
        return 1

    if not enabled:
        return unavailable("BLOCKED: run on the native graphical host with INKWELL_NATIVE_PERF_RUN=1; no window was opened here")
    faults = []
    for attempt in range(1, ATTEMPTS + 1):
        if attempt > 1:
            cool()
        attempt_raw = raw if attempt == 1 else raw.with_name(f"{raw.name}-attempt{attempt}")
        driver = runner([sys.executable, str(ROOT / "scripts/perf/native_input.py"),
                         "--binary", str(binary), "--output", str(attempt_raw),
                         "--input-backend", input_backend], cwd=ROOT, check=False)
        if driver.returncode == EX_TEMPFAIL:
            faults.append(f"attempt {attempt}: {fault_reason(attempt_raw)}")
            continue
        (ROOT / output).unlink(missing_ok=True)  # never read a verdict of an earlier attempt or run
        verified = runner([str(binary), "--native-perf-merge=" + str(attempt_raw),
                           "--native-perf-out=" + str(output)], cwd=ROOT, check=False)
        if report_status(output) == "invalid":
            faults.append(f"attempt {attempt}: coded environment fault in the verdict")
            continue
        if driver.returncode != 0 and verified.returncode == 0:
            return unavailable("Native input driver failed; partial evidence cannot pass")
        return verified.returncode if verified.returncode else driver.returncode
    runner([str(binary), "--native-perf-invalid=" + " | ".join(faults),
            "--native-perf-out=" + str(output)], cwd=ROOT, check=False)
    return 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build-only", action="store_true", help="compile only; never open a window")
    parser.add_argument("--input-backend", choices=("mutter", "xtest"), default="mutter",
                        help="prompt-free Mutter keyboard (default), or explicit XTest fallback")
    args = parser.parse_args()
    build()
    if args.build_only:
        print("Built dedicated perf_native collector (no window opened):", BINARY)
        return 0
    out = Path(os.environ.get("INKWELL_PERF_OUT", ROOT / "gates/L3")).resolve()
    if not out.is_relative_to(ROOT):
        parser.error("native evidence must stay inside this worktree")
    out.mkdir(parents=True, exist_ok=True)
    enabled = os.environ.get("INKWELL_NATIVE_PERF_RUN") == "1"
    # Fresh evidence only; preserve prior raw runs separately, never silently reuse them.
    if enabled:
        retained = ROOT / "gates/L3-setup"
        retained.mkdir(parents=True, exist_ok=True)
        raw = Path(tempfile.mkdtemp(prefix="native-raw-", dir=retained))
    else:
        raw = out
    # The collector runs from ROOT: repository-relative paths keep host paths out of the verdict.
    status = execute(BINARY, (out / "PERF-TYPE.json").relative_to(ROOT), raw.relative_to(ROOT), enabled,
                     input_backend=args.input_backend)
    if enabled:
        # Go writes the attribution table to the detail file (nativeperf.DetailPath).
        annotate_sources(out / "PERF-TYPE-detail.json")
    return status


def annotate_sources(report_path):
    """Best-effort diagnostic: a resolver failure never changes the verdict or exit code."""
    import sourcemap
    try:
        sourcemap.main(["sourcemap", str(report_path), str(ROOT / "frontend/perf-dist")])
    except (OSError, ValueError, KeyError, IndexError) as error:
        print("attribution source resolution skipped:", error, file=sys.stderr)


if __name__ == "__main__":
    raise SystemExit(main())
