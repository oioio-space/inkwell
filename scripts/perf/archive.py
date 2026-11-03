"""Verify immutable reference evidence; never reseal an existing archive.

SHA256SUMS lists every archived file, and the SHA-256 of SHA256SUMS itself is
pinned below. Resealing an archive (rewriting a file and its SHA256SUMS line
together) therefore changes the pin, which is gate machinery: the base guard
flags that change and only a reviewed override admits it. A new archive needs a
new pin, never a replaced one.
"""
import hashlib
from pathlib import Path

BASELINE = Path("docs/perf/baseline")
# Archive directory -> SHA-256 of its SHA256SUMS.
PINS = {"2026-11-02": "4f62771f7abc7f585cc169b9335f427d7d39e21c545bce2d2b9f1508ba9c0c7e"}


def verify(root, pins=None):
    pins = PINS if pins is None else pins
    base = root / BASELINE
    present = {p.name for p in base.iterdir()} if base.is_dir() else set()
    if present != set(pins):
        raise ValueError(f"baseline archives {sorted(present)} differ from pinned {sorted(pins)}")
    sealed = {}
    for name, pin in sorted(pins.items()):
        archive = base / name
        if archive.is_symlink() or not archive.is_dir():
            raise ValueError("baseline archive is not a directory: " + name)
        sums = archive / "SHA256SUMS"
        digest = hashlib.sha256(sums.read_bytes()).hexdigest()
        if digest != pin:
            raise ValueError(f"{sums} was resealed: sha256 {digest}, pinned {pin}")
        expected = {}
        for line in sums.read_text().splitlines():
            entry_digest, entry = line.split("  ", 1)
            if Path(entry).name != entry or entry in expected:
                raise ValueError("invalid archive entry: " + entry)
            expected[entry] = entry_digest
        actual = {str(p.relative_to(archive)): hashlib.sha256(p.read_bytes()).hexdigest()
                  for p in archive.rglob("*") if p.is_file() and p != sums}
        if not expected or actual != expected:
            raise ValueError("immutable baseline archive differs from SHA256SUMS: " + name)
        sealed.update({f"{name}/{k}": v for k, v in actual.items()})
        sealed[f"{name}/SHA256SUMS"] = digest
    return sealed


if __name__ == "__main__":
    verify(Path.cwd())
    print("Immutable archive verified against SHA256SUMS and its pinned digest")
