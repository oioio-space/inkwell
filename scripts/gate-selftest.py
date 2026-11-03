"""Verify failure propagation, continuation, log hashes and replacement on rerun."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile

spec = importlib.util.spec_from_file_location("gate", Path(__file__).with_name("gate.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    (root / "gates").mkdir()
    archive = root / "docs/perf/baseline/2026-11-02"
    archive.mkdir(parents=True)
    (archive / "measurements.json").write_bytes(b'{"archived":true}\n')
    digest = hashlib.sha256((archive / "measurements.json").read_bytes()).hexdigest()
    (archive / "SHA256SUMS").write_text(digest + "  measurements.json\n")
    config = root / "gates/T.toml"
    config.write_text('[[criteria]]\nid="bad"\ncommand="echo failure; exit 7"\n[[criteria]]\nid="after"\ncommand="echo continued"\n')
    assert gate.run("T", root) == 1
    m = json.loads((root / "gates/T.json").read_text())
    assert [e["status"] for e in m["criteria"]] == ["fail", "pass"]
    for path, digest in m["sha256"].items():
        assert hashlib.sha256((root / path).read_bytes()).hexdigest() == digest
    config.write_text('[[criteria]]\nid="ok"\ncommand="true"\n')
    assert gate.run("T", root) == 0
    assert len(json.loads((root / "gates/T.json").read_text())["criteria"]) == 1
    for target, reason, expected in [("", "missing target", 1), ("L2", "", 1), ("L2", "opening delivered in L2", 0)]:
        config.write_text(f'[[criteria]]\nid="later"\nstatus="deferred"\ntarget_lot="{target}"\nreason="{reason}"\n')
        assert gate.run("T", root) == expected
        m = json.loads((root / "gates/T.json").read_text())
        assert len(m["deferrals"]) == (expected == 0)
    config.write_text('[[criteria]]\nid="bad"\ncommand="false"\n[[criteria]]\nid="later"\nstatus="deferred"\ntarget_lot="L2"\nreason="later"\n')
    assert gate.run("T", root) == 1
    config.write_text('[[criteria]]\nid="hidden-failure"\nstatus="deferred"\ntarget_lot="L2"\nreason="cannot waive a command"\ncommand="false"\n')
    assert gate.run("T", root) == 1
    config.write_text('[[criteria]]\nid="declared-failure"\nstatus="fail"\ncommand="true"\n')
    assert gate.run("T", root) == 1
    assert json.loads((root / "gates/T.json").read_text())["criteria"][0]["status"] == "fail"
    # A complete gate writes fresh evidence while the sealed archive is byte-identical.
    before = {p.name: p.read_bytes() for p in archive.iterdir()}
    config.write_text('[[criteria]]\nid="fresh"\ncommand="echo fresh > \\"$INKWELL_PERF_OUT/measurements.json\\""\n')
    assert gate.run("T", root) == 0
    assert (root / "gates/T/measurements.json").read_text() == "fresh\n"
    assert {p.name: p.read_bytes() for p in archive.iterdir()} == before
    # A criterion that corrupts the archive fails the post-run check, never reseals.
    config.write_text('[[criteria]]\nid="tamper"\ncommand="echo tampered > docs/perf/baseline/2026-11-02/measurements.json"\n')
    try:
        gate.run("T", root)
        raise AssertionError("accepted archive mutation by criterion")
    except ValueError:
        pass
    assert (archive / "SHA256SUMS").read_bytes() == before["SHA256SUMS"]
    # Corruption must fail verification rather than update SHA256SUMS.
    (archive / "measurements.json").write_text("tampered")
    try:
        gate.run("T", root)
        raise AssertionError("accepted corrupt archive")
    except ValueError:
        pass
    (archive / "measurements.json").write_bytes(before["measurements.json"])
    try:
        gate.run("../escape", root)
        raise AssertionError("accepted invalid lot")
    except ValueError:
        pass
print("gate self-test passed")
