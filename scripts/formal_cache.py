#!/usr/bin/env python3
"""Content-keyed verdict cache of the formal models (block CI3b, plan docs/plans/2026-11-24-ci3b-formal-cache.md).

A verdict depends only on its inputs: the model and every module it EXTENDS or INSTANCEs (resolved transitively in
the model's folder and the TLC working folder, never guessed), the configuration, the code that judges the run, this
file, the pinned jar's SHA-256, the Java version and the run's arguments. The key is the SHA-256 of all of them, so a
change of any input is a miss. Only a fully green verdict is stored (sound: no error; mutant: caught by its expected
property); a failure, a timeout or an interrupted run never is. Entries expire after INKWELL_FORMAL_CACHE_TTL_DAYS
(default 7), so every verdict is re-earned by a full run at least weekly. INKWELL_FORMAL_NO_CACHE=1 bypasses the cache
entirely; scripts/gate.py sets it for every criterion, so a gate keeps its own full run.

  formal_cache.py tla-lookup --root DIR --module M --cfg C --jar-sha H --java V --runner-sha R [--arg A]...
      prints the key on its first line; exit 0 and the stored lines after it on a hit, 1 on a miss,
      2 when the inputs cannot be resolved (the run is then not cacheable)
  formal_cache.py put KEY NAME < lines
"""
import argparse
import hashlib
import json
import os
import pathlib
import re
import sys
import tempfile
import time

SCHEMA = "inkwell-formal-verdict/1"
SELF = pathlib.Path(__file__).resolve()
# Modules shipped inside tla2tools.jar: their content is covered by the jar's SHA-256.
TLA_STANDARD = frozenset({"Naturals", "Integers", "Reals", "Sequences", "FiniteSets", "Bags", "TLC", "TLCExt",
                          "RealTime", "Randomization", "Json", "Toolbox", "TLAPS", "_TLCTrace"})
IDENT = r"[A-Za-z0-9_]+"


class Unresolved(Exception):
    """An input the key cannot name exactly: the run is not cacheable."""


def disabled():
    return os.environ.get("INKWELL_FORMAL_NO_CACHE", "") not in ("", "0")


def cache_dir():
    explicit = os.environ.get("INKWELL_FORMAL_CACHE")
    if explicit:
        return pathlib.Path(explicit).expanduser()  # GitHub's env passes ~ unexpanded
    base = os.environ.get("XDG_CACHE_HOME") or pathlib.Path.home() / ".cache"
    return pathlib.Path(base) / "inkwell/formal-verdicts"


def ttl_seconds():
    return float(os.environ.get("INKWELL_FORMAL_CACHE_TTL_DAYS", "7")) * 86400


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def strip_tla_comments(text: str) -> str:
    """Removes \\* line comments and nested (* *) block comments, so a word in a comment is never read as a module."""
    out, i, depth = [], 0, 0
    while i < len(text):
        if text.startswith("(*", i):
            depth, i = depth + 1, i + 2
        elif depth and text.startswith("*)", i):
            depth, i = depth - 1, i + 2
        elif depth:
            i += 1
        elif text.startswith("\\*", i):
            end = text.find("\n", i)
            i = len(text) if end < 0 else end
        else:
            out.append(text[i])
            i += 1
    return "".join(out)


def tla_references(text: str) -> list[str]:
    """Module names a TLA+ file EXTENDS or INSTANCEs."""
    text = strip_tla_comments(text)
    names = []
    for m in re.finditer(rf"\bEXTENDS\s+({IDENT}(?:\s*,\s*{IDENT})*)", text):
        names += [n.strip() for n in m.group(1).split(",")]
    names += re.findall(rf"\bINSTANCE\s+({IDENT})", text)
    return names


def tla_closure(root: pathlib.Path, module: str) -> dict[str, str]:
    """{path relative to root: sha256} of the module and every module it reaches. A name is looked up in the
    referring file's folder and in root (TLC's working folder); every local file found is included, so no lookup
    order is assumed. A name found nowhere must be a standard module of the jar, else Unresolved."""
    files, todo = {}, [root / f"{module}.tla"]
    if not todo[0].is_file():
        raise Unresolved(f"no module file {todo[0]}")
    while todo:
        path = todo.pop()
        rel = path.resolve().relative_to(root.resolve()).as_posix()
        if rel in files:
            continue
        data = path.read_bytes()
        files[rel] = sha256(data)
        for name in tla_references(data.decode("utf-8", "replace")):
            found = [p for p in {path.parent / f"{name}.tla", root / f"{name}.tla"} if p.is_file()]
            if not found and name not in TLA_STANDARD:
                raise Unresolved(f"{rel}: module {name} not found")
            todo += found
    return files


def alloy_closure(models: pathlib.Path, text: str) -> dict[str, str]:
    """{path: sha256} of the local modules an Alloy text opens (transitively); util/* ships in the jar."""
    files, todo = {}, [text]
    while todo:
        for name in re.findall(r"^\s*open\s+([\w/]+)", strip_alloy_comments(todo.pop()), re.M):
            path = models / f"{name}.als"
            if path.is_file():
                if name not in files:
                    data = path.read_bytes()
                    files[name] = sha256(data)
                    todo.append(data.decode("utf-8", "replace"))
            elif not name.startswith("util/"):
                raise Unresolved(f"Alloy module {name} not found")
    return files


def strip_alloy_comments(text: str) -> str:
    text = re.sub(r"/\*.*?\*/", "", text, flags=re.S)
    return re.sub(r"(--|//).*", "", text)


def key(tool: str, files: dict[str, str], jar_sha: str, java: str, runner_sha: str, args: list[str]) -> str:
    material = {"schema": SCHEMA, "tool": tool, "cache": sha256(SELF.read_bytes()), "jar": jar_sha,
                "java": java.strip(), "runner": runner_sha, "files": dict(sorted(files.items())), "args": args}
    return sha256(json.dumps(material, sort_keys=True, separators=(",", ":")).encode())


def get(k: str):
    """The stored lines of a fresh verdict, or None."""
    path = cache_dir() / f"{k}.json"
    try:
        entry = json.loads(path.read_text())
    except (OSError, ValueError):
        return None
    if entry.get("key") != k or not isinstance(entry.get("lines"), list):
        return None
    if time.time() - float(entry.get("stored", 0)) > ttl_seconds():
        path.unlink(missing_ok=True)
        return None
    return entry["lines"]


def put(k: str, name: str, lines: list[str]):
    """Stores a green verdict atomically (a reader never sees half a file)."""
    if not re.fullmatch(r"[0-9a-f]{64}", k) or not lines:
        raise ValueError("refusing to store: bad key or empty verdict")
    d = cache_dir()
    d.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=d, prefix=".tmp-")
    with os.fdopen(fd, "w") as f:
        json.dump({"key": k, "name": name, "lines": lines, "stored": time.time()}, f)
    os.replace(tmp, d / f"{k}.json")


def reused_line(k: str, name: str) -> str:
    return f"reused {k[:12]}: {name}"


def main(argv=None):
    p = argparse.ArgumentParser()
    sub = p.add_subparsers(dest="cmd", required=True)
    look = sub.add_parser("tla-lookup")
    for opt in ("--root", "--module", "--cfg", "--jar-sha", "--java", "--runner-sha"):
        look.add_argument(opt, required=True)
    look.add_argument("--arg", action="append", default=[])
    store = sub.add_parser("put")
    store.add_argument("key")
    store.add_argument("name")
    a = p.parse_args(argv)
    if a.cmd == "put":
        put(a.key, a.name, [line for line in sys.stdin.read().splitlines() if line])
        return 0
    root = pathlib.Path(a.root)
    try:
        files = tla_closure(root, a.module)
        cfg = root / a.cfg
        files["cfg:" + a.cfg] = sha256(cfg.read_bytes())
    except (Unresolved, OSError) as exc:
        print(f"formal-cache: not cacheable: {exc}", file=sys.stderr)
        return 2
    k = key("tlc", files, a.jar_sha, a.java, a.runner_sha, a.arg)
    print(k)
    lines = get(k)
    if lines is None:
        return 1
    print("\n".join(lines))
    return 0


if __name__ == "__main__":
    sys.exit(main())
