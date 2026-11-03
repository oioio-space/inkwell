"""Verify immutable reference evidence; never reseal an existing archive."""
import hashlib
from pathlib import Path


def verify(root):
    archive = root / "docs/perf/baseline/2026-11-02"
    sums = archive / "SHA256SUMS"
    expected = {}
    for line in sums.read_text().splitlines():
        digest, name = line.split("  ", 1)
        if Path(name).name != name or name in expected:
            raise ValueError("invalid archive entry: " + name)
        expected[name] = digest
    actual = {str(p.relative_to(archive)): hashlib.sha256(p.read_bytes()).hexdigest()
              for p in archive.rglob("*") if p.is_file() and p != sums}
    if not expected or actual != expected:
        raise ValueError("immutable baseline archive differs from SHA256SUMS")
    return {**actual, "SHA256SUMS": hashlib.sha256(sums.read_bytes()).hexdigest()}


if __name__ == "__main__":
    verify(Path.cwd())
    print("Immutable archive verified against SHA256SUMS")
