#!/usr/bin/env python3
"""Fail when code reads an environment variable that .env.reference does not declare.

.env.reference is the inventory of inkwell's configuration surface (and the provider side of
gortex env contracts). This check keeps it exhaustive: `mise run lint:env`, part of `mise run ci`.
"""
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
READS = [
    r'os\.(?:Getenv|LookupEnv)\(\s*"([A-Z][A-Z0-9_]+)"',
    r'process\.env\.([A-Z][A-Z0-9_]+)',
    r'process\.env\[\s*"([A-Z][A-Z0-9_]+)"',
    r'os\.environ(?:\.get)?[\[(]\s*"([A-Z][A-Z0-9_]+)"',
    r'os\.getenv\(\s*"([A-Z][A-Z0-9_]+)"',
    r'import\.meta\.env\.([A-Z][A-Z0-9_]+)',
]
SUFFIXES = (".go", ".ts", ".js", ".cjs", ".mjs", ".py", ".svelte")
SKIP = {"node_modules", ".claude", ".git", "dist", "perf-dist", "bindings", "build", "bin"}


def declared():
    """Names declared in .env.reference; a declared value is an error (never a secret there)."""
    names, valued = set(), []
    for line in (ROOT / ".env.reference").read_text().splitlines():
        m = re.match(r"^([A-Z][A-Z0-9_]*)=(.*)$", line)
        if m:
            names.add(m.group(1))
            if m.group(2).strip():
                valued.append(m.group(1))
    return names, valued


def reads():
    found = {}
    for path in ROOT.rglob("*"):
        if path.suffix not in SUFFIXES or SKIP.intersection(path.relative_to(ROOT).parts):
            continue
        text = path.read_text(errors="ignore")
        for pattern in READS:
            for m in re.finditer(pattern, text):
                found.setdefault(m.group(1), str(path.relative_to(ROOT)))
    return found


def main():
    names, valued = declared()
    missing = {k: v for k, v in reads().items() if k not in names}
    for name, where in sorted(missing.items()):
        print(f"env var {name} read in {where} is missing from .env.reference", file=sys.stderr)
    for name in valued:
        print(f".env.reference: {name} has a value; this file lists names only", file=sys.stderr)
    return 1 if missing or valued else 0


if __name__ == "__main__":
    sys.exit(main())
