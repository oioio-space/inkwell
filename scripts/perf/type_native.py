"""Build the tagged native collector; run only with an explicit native-host opt-in.

No display is opened by build-only mode or by the default sandbox refusal. The
Go collector/verifier writes every PERF-TYPE verdict; Python never invents timing
samples or turns rAF opportunities into paint evidence. The dedicated asset set
also supplies the native monotonic handler clock (docs/perf/native-typing.md).
"""
import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
BINARY = ROOT / "bin/inkwell-native-perf"


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
            "./cmd/inkwell"], cwd=ROOT, check=True)


def execute(binary, output, raw, enabled, runner=subprocess.run, input_backend="mutter"):
    """The input driver is unreachable without the external-native opt-in."""
    def unavailable(reason):
        runner([str(binary), "--native-perf-unavailable=" + reason,
                "--native-perf-out=" + str(output)], cwd=ROOT, check=False)
        return 1

    if not enabled:
        return unavailable("BLOCKED: run on the native graphical host with INKWELL_NATIVE_PERF_RUN=1; no window was opened here")
    driver = runner([sys.executable, str(ROOT / "scripts/perf/native_input.py"),
                     "--binary", str(binary), "--output", str(raw),
                     "--input-backend", input_backend], cwd=ROOT, check=False)
    verified = runner([str(binary), "--native-perf-merge=" + str(raw),
                       "--native-perf-out=" + str(output)], cwd=ROOT, check=False)
    if driver.returncode != 0 and verified.returncode == 0:
        return unavailable("Native input driver failed; partial evidence cannot pass")
    return verified.returncode if verified.returncode else driver.returncode


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
