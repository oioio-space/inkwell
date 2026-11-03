"""G-0a: file selection must come from the sidebar, never a URL override."""
from pathlib import Path

for path in Path("frontend/src").rglob("*"):
    if path.suffix not in {".ts", ".svelte"}:
        continue
    source = path.read_text()
    if "?file=" in source or '.get("file")' in source or ".get('file')" in source:
        raise SystemExit(f"file URL override remains: {path}")
print("File URL override absent")
