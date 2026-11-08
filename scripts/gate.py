"""Small sequential gate runner; commands are trusted repository configuration.

Every run first executes the anti-weakening guard from the merge-base's
version of scripts/gate-guard.py (never the working tree's), clears
gates/<lot>/ so stale files are never hashed as evidence, and validates each
deferral against its target lot.

scripts/gate.sh runs this file from the merge-base (see there), isolated
(python3 -I) from a temporary copy; INKWELL_GATE_ROOT then names the checkout
and INKWELL_GATE_RUNNER says which runner ran, recorded in the manifest.

Usage: gate.py <lot> | gate.py --base-guard
"""
import hashlib
import importlib.util
import json
import os
import pathlib
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import time
import tomllib

# gate.sh runs only a base runner that declares this protocol.
GATE_RUNNER_PROTOCOL = 1
SCRIPTS = pathlib.Path(__file__).resolve().parent
ROOT = pathlib.Path(os.environ.get("INKWELL_GATE_ROOT") or SCRIPTS.parent).resolve()


def reset_evidence(out):
    """Empties gates/<lot>/ for a new run: evidence is only what this run produced. The block's
    reports (*.md: SIMPLIFY, reconstructions, reviews) are versioned documents, not evidence, and
    are kept (2026-11-08: an rmtree of the whole folder erased them). Symlinks are removed, never
    followed."""
    out.mkdir(parents=True, exist_ok=True)
    for entry in out.iterdir():
        if entry.suffix == ".md" and entry.is_file() and not entry.is_symlink():
            continue
        if entry.is_dir() and not entry.is_symlink():
            shutil.rmtree(entry)
        else:
            entry.unlink()


def _load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


# By path, beside this file: isolated mode (-I) puts no script directory on sys.path.
verify = _load("archive", SCRIPTS / "perf/archive.py").verify
NAME = re.compile(r"[A-Za-z0-9_-]+")
OVERRIDES = "gates/guard-overrides.json"
# The known-lot list: a ```gate-lots block of "LOT: CRITERION …" lines.
KNOWN_LOTS = "docs/objectif-en-cours.md"
# Run a guard source as if it lived at argv[1], so its ROOT is the checkout.
BOOTSTRAP = ("import sys; path = sys.argv[1]; source = sys.stdin.read(); sys.argv = [path, *sys.argv[2:]]; "
             "exec(compile(source, path, 'exec'), {'__name__': '__main__', '__file__': path})")


def version(command):
    try:
        p = subprocess.run(command, shell=True, cwd=ROOT, capture_output=True, text=True)
        return (p.stdout + p.stderr).strip() if p.returncode == 0 else "unavailable: " + (p.stdout + p.stderr).strip()
    except OSError as exc:
        return "unavailable: " + str(exc)


def known_lots(root):
    path = root / KNOWN_LOTS
    match = re.search(r"^```gate-lots\n(.*?)^```", path.read_text(), re.M | re.S) if path.exists() else None
    lots = {}
    for line in (match[1] if match else "").splitlines():
        lot, _, criteria = line.partition(":")
        if lot.strip():
            lots[lot.strip()] = set(criteria.split())
    return lots


def deferral_problem(criterion, lot, root):
    """Why a deferral is invalid, or None; the target lot must re-list the criterion."""
    target = criterion.get("target_lot", "")
    if not NAME.fullmatch(target) or target == lot:
        return "invalid target lot"
    if not criterion.get("reason", "").strip():
        return "missing reason"
    if "command" in criterion:
        return "a deferral cannot hide a command"
    config = root / "gates" / (target + ".toml")
    if config.exists():
        listed = {c["id"] for c in tomllib.loads(config.read_text()).get("criteria", [])}
        return None if criterion["id"] in listed else f"gates/{target}.toml does not list {criterion['id']}"
    lots = known_lots(root)
    if target not in lots:
        return f"unknown target lot {target}: no gates/{target}.toml and not in {KNOWN_LOTS}"
    return None if criterion["id"] in lots[target] else f"known lot {target} in {KNOWN_LOTS} does not list {criterion['id']}"


def base_guard(root, stream, ref="main"):
    """Run the merge-base's guard against the working tree; returns its record."""
    record = {"ref": ref}
    try:
        base = subprocess.check_output(["git", "merge-base", "HEAD", ref], cwd=root, text=True).strip()
        source = subprocess.check_output(["git", "show", base + ":scripts/gate-guard.py"], cwd=root)
    except (OSError, subprocess.CalledProcessError) as exc:
        stream.write(f"base guard unavailable: {exc}\n".encode())
        return dict(record, status="fail", reason=str(exc))
    stream.flush()
    # Isolated (-I) and from outside the checkout: nothing in the tree is imported.
    with tempfile.TemporaryDirectory(prefix="gate-guard-") as outside:
        p = subprocess.run([sys.executable, "-I", "-c", BOOTSTRAP, str(root / "scripts/gate-guard.py"), "--base", base],
                           input=source, cwd=outside, stdout=stream, stderr=subprocess.STDOUT)
    return dict(record, base=base, source_sha256=hashlib.sha256(source).hexdigest(),
                exit_code=p.returncode, status="pass" if p.returncode == 0 else "fail")


def exclusive_records(out):
    path = out / "exclusive.jsonl"
    return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []


def run(lot, root=ROOT, pins=None, guard=True):
    if not NAME.fullmatch(lot):
        raise ValueError("invalid lot name")
    sealed = verify(root, pins)
    config = tomllib.loads((root / "gates" / (lot + ".toml")).read_text())
    out = root / "gates" / lot
    if out.is_symlink():
        raise ValueError("gates/" + lot + " must not be a symlink")
    reset_evidence(out)
    entries = []
    ids = [c["id"] for c in config["criteria"]]
    if len(set(ids)) != len(ids) or any(not NAME.fullmatch(i) for i in ids):
        raise ValueError("invalid or duplicate criterion IDs")
    guard_record = None
    if guard:
        log = out / "base-guard.log"
        with log.open("wb") as stream:
            guard_record = dict(base_guard(root, stream), log=str(log.relative_to(root)))
        print("base-guard", guard_record["status"], flush=True)
    for c in config["criteria"]:
        if c.get("status") == "deferred":
            problem = deferral_problem(c, lot, root)
            entries.append(dict(c, status="fail", problem=problem) if problem else dict(c))
            print(c["id"], entries[-1]["status"], problem or "", flush=True)
            continue
        if c.get("status", "pass") not in ("pass", "fail"):
            raise ValueError("invalid criterion status")
        log = out / (c["id"] + ".log")
        start = time.monotonic()
        with log.open("wb") as stream:
            p = subprocess.run(c["command"], shell=True, cwd=root, stdout=stream, stderr=subprocess.STDOUT,
                               env=dict(os.environ, INKWELL_PERF_OUT=str(out), INKWELL_GATE_CRITERION=c["id"]))
        entry = dict(id=c["id"], command=c["command"], status="pass" if p.returncode == 0 and c.get("status") != "fail" else "fail",
                     exit_code=p.returncode, duration_seconds=time.monotonic()-start,
                     log=str(log.relative_to(root)))
        # scripts/heavy.sh --exclusive records each isolated measurement step.
        records = [r for r in exclusive_records(out) if r["criterion"] == c["id"]]
        if records:
            entry["exclusive"] = all(r["exclusive"] for r in records)
            entry["exclusive_steps"] = records
        if c.get("exclusive") and not entry.get("exclusive"):
            entry.update(status="fail", problem="performance measurement ran without the exclusive heavy lock")
        entries.append(entry)
        print(c["id"], entry["status"], entry.get("problem", ""), flush=True)
    if verify(root, pins) != sealed:
        raise ValueError("gate modified immutable archive")
    files = list(out.rglob("*"))
    for pattern in config.get("raw_results", []):
        files.extend(root.glob(pattern))
    hashes = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
              for p in sorted(set(files)) if p.is_file()}
    failed = any(e["status"] == "fail" for e in entries) or (guard_record or {}).get("status") == "fail"
    manifest = dict(lot=lot, criteria=entries, versions={
        "go": version("go version"), "node": version("node --version"),
        "typst": version("typst --version"), "tinymist": version("tinymist --version"),
        "os": platform.platform()}, sha256=hashes,
        status="fail" if failed else "pass",
        deferrals=[e for e in entries if e["status"] == "deferred"],
        known_failures=config.get("known_failures", []))
    if guard_record:
        manifest["base_guard"] = guard_record
    manifest["runner"] = {"source": os.environ.get("INKWELL_GATE_RUNNER", "direct: " + str(pathlib.Path(__file__))),
                          "sha256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    overrides = root / OVERRIDES
    # Reviewed overrides are part of the evidence, shown in full.
    manifest["guard_overrides"] = json.loads(overrides.read_text())["overrides"] if overrides.exists() else []
    pins_file = root / "internal/sidecar/manifest.json"
    if pins_file.exists():
        manifest["versions"]["embedded_sidecars"] = {
            name: data["version"] for name, data in json.loads(pins_file.read_text())["sidecars"].items()}
    for pattern in config.get("measurement_reports", []):
        report = root / pattern
        manifest.setdefault("measurements", []).append(
            json.loads(report.read_text()) if report.exists() else
            {"status": "unavailable", "reason": "measurement command did not produce " + pattern})
    target = root / "gates" / (lot + ".json")
    temporary = target.with_suffix(".json.tmp")
    temporary.write_text(json.dumps(manifest, indent=2) + "\n")
    temporary.replace(target)
    return int(failed)


if __name__ == "__main__":
    if sys.argv[1:] == ["--base-guard"]:
        sys.exit(base_guard(ROOT, sys.stdout.buffer)["status"] != "pass")
    sys.exit(run(sys.argv[1]) if len(sys.argv) == 2 else 2)
