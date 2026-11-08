#!/usr/bin/env python3
"""Fetch a URL pinned by SHA-256, cache it, and return the cached path.

Used by alloy_check.py and tla-check.sh. Stdlib only.
"""
import hashlib
import os
import pathlib
import shutil
import sys
import urllib.request


def fetch(url: str, sha256: str, dest: pathlib.Path) -> pathlib.Path:
    """Download url to dest.part, verify sha256, atomically rename to dest.
    Returns the dest path (existing valid cache is reused without download)."""
    dest = pathlib.Path(dest)
    if dest.exists() and hashlib.sha256(dest.read_bytes()).hexdigest() == sha256:
        return dest

    dest.parent.mkdir(parents=True, exist_ok=True)
    tmp = dest.with_suffix(".part")
    try:
        with urllib.request.urlopen(url, timeout=120) as r, open(tmp, "wb") as f:
            shutil.copyfileobj(r, f)
    except Exception as e:
        tmp.unlink(missing_ok=True)
        sys.exit(f"fetch_pinned: download failed: {e}")

    digest = hashlib.sha256(tmp.read_bytes()).hexdigest()
    if digest != sha256:
        tmp.unlink()
        sys.exit(f"fetch_pinned: {url} sha256 {digest} differs from pinned {sha256}")

    tmp.replace(dest)
    return dest


def _cli() -> None:
    if len(sys.argv) != 4:
        sys.exit("usage: fetch_pinned.py <url> <sha256> <dest>")
    url, sha256, dest = sys.argv[1], sys.argv[2], sys.argv[3]
    fetch(url, sha256, pathlib.Path(dest))


if __name__ == "__main__":
    _cli()