"""Verify failure propagation, continuation, log hashes, replacement on rerun,
deferral targets, stale-evidence removal, the archive pin and the base guard."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


HERE = Path(__file__).resolve().parent
gate = load("gate", HERE / "gate.py")
# Run inside a gate's criterion, the parent's variables would make every gate.run() below a deferral returning 0.
os.environ.pop("INKWELL_GATE_PARENT", None)
os.environ.pop("INKWELL_GATE_PARENT_OUT", None)


def rejects(call, what):
    try:
        call()
    except ValueError:
        return
    raise AssertionError("accepted " + what)


with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    (root / "gates").mkdir()
    archive = root / "docs/perf/baseline/2026-11-02"
    archive.mkdir(parents=True)
    (archive / "measurements.json").write_bytes(b'{"archived":true}\n')
    digest = hashlib.sha256((archive / "measurements.json").read_bytes()).hexdigest()
    (archive / "SHA256SUMS").write_text(digest + "  measurements.json\n")
    pins = {"2026-11-02": hashlib.sha256((archive / "SHA256SUMS").read_bytes()).hexdigest()}

    def run(lot="T"):
        return gate.run(lot, root, pins=pins, guard=False)

    config = root / "gates/T.toml"
    config.write_text('[[criteria]]\nid="bad"\ncommand="echo failure; exit 7"\n[[criteria]]\nid="after"\ncommand="echo continued"\n')
    assert run() == 1
    m = json.loads((root / "gates/T.json").read_text())
    assert [e["status"] for e in m["criteria"]] == ["fail", "pass"]
    for path, digest in m["sha256"].items():
        assert hashlib.sha256((root / path).read_bytes()).hexdigest() == digest
    # Stale files from an earlier run are removed, never hashed as evidence.
    (root / "gates/T/stale.txt").write_text("old evidence")
    config.write_text('[[criteria]]\nid="ok"\ncommand="true"\n')
    assert run() == 0
    m = json.loads((root / "gates/T.json").read_text())
    assert len(m["criteria"]) == 1
    assert not (root / "gates/T/stale.txt").exists() and "gates/T/stale.txt" not in m["sha256"]
    # A failing base guard stops the gate before any criterion runs: it can no longer pass (fail fast).
    real_guard = gate.base_guard
    gate.base_guard = lambda root, stream, ref="main": {"status": "fail"}
    try:
        config.write_text('[[criteria]]\nid="long"\ncommand="touch ran.marker"\n')
        assert gate.run("T", root, pins=pins, guard=True) == 1
        m = json.loads((root / "gates/T.json").read_text())
        assert not (root / "ran.marker").exists(), "a criterion ran after the base guard failed"
        assert [e["status"] for e in m["criteria"]] == ["fail"] and "base guard" in m["criteria"][0]["problem"]
    finally:
        gate.base_guard = real_guard
    assert not any(path.endswith("bad.log") for path in m["sha256"]), m["sha256"]

    # A deferral must target an existing lot that lists the criterion again.
    deferral = '[[criteria]]\nid="later"\nstatus="deferred"\ntarget_lot="%s"\nreason="%s"\n'
    objective = root / "docs/objectif-en-cours.md"
    objective.write_text("# Objectif\n\n```gate-lots\nL2: later\nL3: other\n```\n")
    for target, reason, expected in [("", "missing target", 1), ("L2", "", 1), ("T", "self", 1),
                                     ("L2", "opening delivered in L2", 0),  # known lot lists it
                                     ("L3", "known lot without the criterion", 1),
                                     ("L9", "unknown lot", 1)]:
        config.write_text(deferral % (target, reason))
        assert run() == expected, (target, reason)
        m = json.loads((root / "gates/T.json").read_text())
        assert len(m["deferrals"]) == (expected == 0)
        assert expected == 0 or m["criteria"][0]["problem"], m
    # An existing target toml takes precedence over the known list and must list the id.
    (root / "gates/L2.toml").write_text('[[criteria]]\nid="unrelated"\ncommand="true"\n')
    config.write_text(deferral % ("L2", "opening delivered in L2"))
    assert run() == 1
    assert "does not list later" in json.loads((root / "gates/T.json").read_text())["criteria"][0]["problem"]
    (root / "gates/L2.toml").write_text('[[criteria]]\nid="later"\ncommand="true"\n')
    assert run() == 0
    config.write_text('[[criteria]]\nid="bad"\ncommand="false"\n' + deferral % ("L2", "later"))
    assert run() == 1
    config.write_text('[[criteria]]\nid="hidden-failure"\nstatus="deferred"\ntarget_lot="L2"\nreason="cannot waive a command"\ncommand="false"\n')
    assert run() == 1
    config.write_text('[[criteria]]\nid="declared-failure"\nstatus="fail"\ncommand="true"\n')
    assert run() == 1
    assert json.loads((root / "gates/T.json").read_text())["criteria"][0]["status"] == "fail"

    # Reviewed overrides are shown in full in the manifest.
    override = {"id": "0" * 64, "reason": "reviewed", "decision": "docs/objectif-en-cours.md"}
    (root / "gates/guard-overrides.json").write_text(json.dumps({"overrides": [override]}))
    config.write_text('[[criteria]]\nid="ok"\ncommand="true"\n')
    assert run() == 0
    assert json.loads((root / "gates/T.json").read_text())["guard_overrides"] == [override]

    # A complete gate writes fresh evidence while the sealed archive is byte-identical.
    before = {p.name: p.read_bytes() for p in archive.iterdir()}
    config.write_text('[[criteria]]\nid="fresh"\ncommand="echo fresh > \\"$INKWELL_PERF_OUT/measurements.json\\""\n')
    assert run() == 0
    assert (root / "gates/T/measurements.json").read_text() == "fresh\n"
    assert {p.name: p.read_bytes() for p in archive.iterdir()} == before
    # A criterion that corrupts the archive fails the post-run check, never reseals.
    config.write_text('[[criteria]]\nid="tamper"\ncommand="echo tampered > docs/perf/baseline/2026-11-02/measurements.json"\n')
    rejects(run, "archive mutation by criterion")
    assert (archive / "SHA256SUMS").read_bytes() == before["SHA256SUMS"]
    # Corruption must fail verification rather than update SHA256SUMS.
    (archive / "measurements.json").write_text("tampered")
    rejects(run, "corrupt archive")
    # Resealing (a consistent new SHA256SUMS) is caught by the pinned digest.
    (archive / "SHA256SUMS").write_text(hashlib.sha256(b"tampered").hexdigest() + "  measurements.json\n")
    rejects(run, "resealed archive")
    for name, data in before.items():
        (archive / name).write_bytes(data)
    config.write_text('[[criteria]]\nid="ok"\ncommand="true"\n')
    assert run() == 0
    (root / "docs/perf/baseline/2026-12-01").mkdir()
    rejects(run, "unpinned archive")
    (root / "docs/perf/baseline/2026-12-01").rmdir()
    # The repository pin matches the real archive.
    gate.verify(HERE.parent)
    rejects(lambda: gate.run("../escape", root, pins=pins, guard=False), "invalid lot")

# The base guard is the merge-base's version: rewriting the guard in a lot does not help.
with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    git = lambda *args: subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args],
                                       cwd=root, check=True, capture_output=True)
    git("init", "-q", "-b", "main")
    (root / "scripts").mkdir()
    (root / "gates").mkdir()
    shutil.copy(HERE / "gate-guard.py", root / "scripts/gate-guard.py")
    (root / "x_test.go").write_text("package x\n")
    git("add", ".")
    git("commit", "-q", "-m", "base")
    git("checkout", "-q", "-b", "lot")

    def guard():
        log = root / "guard.log"
        with log.open("wb") as stream:
            record = gate.base_guard(root, stream)
        return record, log.read_text()

    record, log = guard()
    assert record["status"] == "pass", log
    (root / "x_test.go").write_text('package x\nfunc TestX(t *testing.T) { t.Skip("later") }\n')
    (root / "scripts/gate-guard.py").write_text('print("gate guard: pass")\n')
    record, log = guard()
    assert record["status"] == "fail" and "added test skip" in log and "gate machinery changed" in log, log
    assert record["source_sha256"] == hashlib.sha256((HERE / "gate-guard.py").read_bytes()).hexdigest()
# gate.sh runs the merge-base's runner; a branch's own runner never judges it,
# except in the documented bootstrap case, which the manifest records.
def runner_repo(tmp, base_runner):
    root = Path(tmp)
    git = lambda *args: subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args],
                                       cwd=root, check=True, capture_output=True)
    git("init", "-q", "-b", "main")
    for name in ("gate.sh", "gate.py", "gate-guard.py", "perf/archive.py"):
        (root / "scripts" / name).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy(HERE / name, root / "scripts" / name)
    shutil.copytree(HERE.parent / "docs/perf/baseline", root / "docs/perf/baseline")
    (root / "gates").mkdir()
    (root / "gates/T.toml").write_text('[[criteria]]\nid="ok"\ncommand="true"\n')
    real = (HERE / "gate.py").read_text()
    (root / "scripts/gate.py").write_text(base_runner(real))
    git("add", ".")
    git("commit", "-q", "-m", "base")
    git("checkout", "-q", "-b", "lot")
    (root / "scripts/gate.py").write_text(real)
    return root


with tempfile.TemporaryDirectory() as tmp:
    root = runner_repo(tmp, lambda real: real)
    (root / "scripts/gate.py").write_text('import sys\nprint("STUB RUNNER")\nsys.exit(0)\n')
    p = subprocess.run(["bash", str(root / "scripts/gate.sh"), "T"], capture_output=True, text=True)
    m = json.loads((root / "gates/T.json").read_text())
    assert "STUB RUNNER" not in p.stdout and m["runner"]["source"].startswith("base "), (p.stdout, m["runner"])
    assert p.returncode == 1 and m["base_guard"]["status"] == "fail", "the edited runner must be flagged"
with tempfile.TemporaryDirectory() as tmp:
    root = runner_repo(tmp, lambda real: real.replace("GATE_RUNNER_PROTOCOL = 1\n", ""))
    p = subprocess.run(["bash", str(root / "scripts/gate.sh"), "T"], capture_output=True, text=True)
    m = json.loads((root / "gates/T.json").read_text())
    assert m["runner"]["source"].startswith("working-tree (bootstrap"), (p.stdout, p.stderr, m.get("runner"))

# heavy.sh --exclusive holds every slot: a concurrent heavy job starts only after it.
HEAVY = str(HERE / "heavy.sh")
with tempfile.TemporaryDirectory() as tmp:
    base_env = {k: v for k, v in os.environ.items() if k not in ("INKWELL_HEAVY_HELD", "INKWELL_HEAVY_SLOT")}
    env = dict(base_env, XDG_RUNTIME_DIR=tmp, INKWELL_HEAVY_SLOTS="2", INKWELL_PERF_MAX_LOAD="100000",
               INKWELL_PERF_OUT=tmp + "/out", INKWELL_GATE_CRITERION="probe")

    def overlap(mode):
        marks = Path(tmp) / mode
        marks.mkdir()
        holder = subprocess.Popen([HEAVY, *mode.split("+")[1:], "sh", "-c",
                                   'touch "$0/started"; sleep 2; date +%s.%N > "$0/end"', str(marks)], env=env)
        while not (marks / "started").exists():
            assert holder.poll() is None, "holder exited before starting"
            time.sleep(0.05)
        subprocess.run([HEAVY, "sh", "-c", 'date +%s.%N > "$0/other"', str(marks)], env=env, check=True)
        assert holder.wait() == 0
        return float((marks / "other").read_text()) < float((marks / "end").read_text())

    assert overlap("shared"), "control: a shared job must leave a slot free"
    assert not overlap("exclusive+--exclusive"), "a heavy job ran during an exclusive measurement"
    record = json.loads((Path(tmp) / "out/exclusive.jsonl").read_text().splitlines()[-1])
    assert record["exclusive"] and record["criterion"] == "probe" and record["slots"] == 2, record
    assert len(record["load_before"]) == 3 and len(record["load_after"]) == 3, record
    # Above the load bound it waits, then refuses to measure: the command never runs.
    busy = subprocess.run([HEAVY, "--exclusive", "sh", "-c", 'touch "$0/ran"', tmp],
                          env=dict(env, INKWELL_PERF_MAX_LOAD="-1", INKWELL_PERF_LOAD_WAIT="0"))
    assert busy.returncode == 75 and not (Path(tmp) / "ran").exists()
    nested = subprocess.run([HEAVY, "--exclusive", "true"], env=dict(env, INKWELL_HEAVY_HELD="1", INKWELL_HEAVY_SLOT="0"))
    assert nested.returncode == 2, "exclusive inside a shared slot must refuse, not deadlock"

    # The gate fails a measurement criterion that ran without the exclusive lock.
    root = Path(tmp) / "repo"
    (root / "gates").mkdir(parents=True)
    archive = root / "docs/perf/baseline/2026-11-02"
    archive.mkdir(parents=True)
    (archive / "a").write_text("a")
    (archive / "SHA256SUMS").write_text(hashlib.sha256(b"a").hexdigest() + "  a\n")
    pins = {"2026-11-02": hashlib.sha256((archive / "SHA256SUMS").read_bytes()).hexdigest()}
    os.environ.update(XDG_RUNTIME_DIR=tmp, INKWELL_PERF_MAX_LOAD="100000")
    for command, expected in [("true", 1), (f"{HEAVY} --exclusive true", 0)]:
        (root / "gates/T.toml").write_text(f'[[criteria]]\nid="perf"\nexclusive=true\ncommand="{command}"\n')
        assert gate.run("T", root, pins=pins, guard=False) == expected, command
    entry = json.loads((root / "gates/T.json").read_text())["criteria"][0]
    assert entry["exclusive"] is True and entry["exclusive_steps"][0]["load_before"], entry

# baseline.py judges PERF-START by the paired, randomised rule of ADR 0038 (and its self-test, on synthetic pairs, starts no
# process): an unchanged product passes, a x1.2 regression regresses, a wide run is invalid. Never a bare "pass".
subprocess.run([sys.executable, str(HERE / "perf/baseline.py")], check=True,
               env=dict(os.environ, INKWELL_BASELINE_SELF_TEST="1"))
print("gate self-test passed")
