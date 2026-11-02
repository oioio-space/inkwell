"""Small sequential gate runner; commands are trusted repository configuration."""
import hashlib
import json
import pathlib
import platform
import re
import subprocess
import sys
import time
import tomllib

ROOT = pathlib.Path(__file__).resolve().parent.parent


def version(command):
    try:
        p = subprocess.run(command, shell=True, cwd=ROOT, capture_output=True, text=True)
        return (p.stdout + p.stderr).strip() if p.returncode == 0 else "unavailable: " + (p.stdout + p.stderr).strip()
    except OSError as exc:
        return "unavailable: " + str(exc)


def run(lot, root=ROOT):
    if not re.fullmatch(r"[A-Za-z0-9_-]+", lot):
        raise ValueError("invalid lot name")
    config = tomllib.loads((root / "gates" / (lot + ".toml")).read_text())
    out = root / "gates" / lot
    out.mkdir(parents=True, exist_ok=True)
    entries = []
    ids = [c["id"] for c in config["criteria"]]
    if len(set(ids)) != len(ids) or any(not re.fullmatch(r"[A-Za-z0-9_-]+", i) for i in ids):
        raise ValueError("invalid or duplicate criterion IDs")
    for c in config["criteria"]:
        if c.get("status") == "deferred":
            valid = bool(re.fullmatch(r"[A-Za-z0-9_-]+", c.get("target_lot", ""))) and bool(c.get("reason", "").strip()) and c.get("target_lot") != lot and "command" not in c
            entries.append(dict(c, status="deferred" if valid else "fail"))
            print(c["id"], entries[-1]["status"], flush=True)
            continue
        if c.get("status", "pass") not in ("pass", "fail"):
            raise ValueError("invalid criterion status")
        log = out / (c["id"] + ".log")
        start = time.monotonic()
        with log.open("wb") as stream:
            p = subprocess.run(c["command"], shell=True, cwd=root, stdout=stream, stderr=subprocess.STDOUT)
        entries.append(dict(id=c["id"], command=c["command"], status="pass" if p.returncode == 0 and c.get("status") != "fail" else "fail",
                            exit_code=p.returncode, duration_seconds=time.monotonic()-start,
                            log=str(log.relative_to(root))))
        print(c["id"], entries[-1]["status"], flush=True)
    files = list(out.rglob("*"))
    for pattern in config.get("raw_results", []):
        files.extend(root.glob(pattern))
    hashes = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
              for p in sorted(set(files)) if p.is_file()}
    manifest = dict(lot=lot, criteria=entries, versions={
        "go": version("go version"), "node": version("node --version"),
        "typst": version("typst --version"), "tinymist": version("tinymist --version"),
        "os": platform.platform()}, sha256=hashes,
        status="fail" if any(e["status"] == "fail" for e in entries) else "pass",
        deferrals=[e for e in entries if e["status"] == "deferred"],
        known_failures=config.get("known_failures", []))
    pins = root / "internal/sidecar/manifest.json"
    if pins.exists():
        manifest["versions"]["embedded_sidecars"] = {
            name: data["version"] for name, data in json.loads(pins.read_text())["sidecars"].items()}
    for pattern in config.get("measurement_reports", []):
        report = root / pattern
        manifest.setdefault("measurements", []).append(
            json.loads(report.read_text()) if report.exists() else
            {"status": "unavailable", "reason": "measurement command did not produce " + pattern})
    target = root / "gates" / (lot + ".json")
    temporary = target.with_suffix(".json.tmp")
    temporary.write_text(json.dumps(manifest, indent=2) + "\n")
    temporary.replace(target)
    return int(any(e["status"] == "fail" for e in entries))


if __name__ == "__main__":
    sys.exit(run(sys.argv[1]) if len(sys.argv) == 2 else 2)
