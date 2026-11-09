#!/usr/bin/env python3
"""Decide whether the impaired netsim must run for a diff (gate CI1, rules CI1-1..CI1-4).

Exit 0 = run, 10 = skip. Prints one line: `netsim: run (<first triggering path>)` or
`netsim: skip (inert: N paths)`. Every failure (unresolvable base, `go list` failing, crash)
decides "run": the selector fails towards running, never towards skipping.

  affected.py --paths P...      test the given paths as if they were the diff
  affected.py --pre-push        read git's pre-push stdin: <local ref> <local sha> <remote ref> <remote sha>
  affected.py --base B          diff B..HEAD

The Go input set is computed (CI1-1): the non-test files of every package, under the repository,
in `go list -deps` of the binaries the netsim launches (see GO_TARGETS). Tests may inject the
`go list` output (a stream of JSON objects) with INKWELL_AFFECTED_GOLIST=<file>.
"""

import argparse
import fnmatch
import json
import os
import subprocess
import sys

RUN, SKIP = 0, 10
ZERO = "0" * 40

# What `mise run e2e:build` builds and scripts/netsim/run.sh launches: bin/inkwell-server and bin/inkwell-relay.
GO_TARGETS = (
    ({}, ["-tags", "server", "./cmd/inkwell"]),
    ({"CGO_ENABLED": "0"}, ["./cmd/inkwell-relay"]),
)

# Exact files whose change always runs the netsim; scripts/netsim/** and frontend/** (but its unit tests) too.
INPUT_FILES = {"go.mod", "go.sum", "mise.toml", "scripts/heavy.sh", "scripts/memcap.sh"}
# Explicitly inert (CI1-2); anything else that is not an input is unknown and runs.
INERT_PREFIXES = ("docs/", "gates/", ".claude/", ".remember/", "scripts/perf/")


class SelectorError(Exception):
    """A failure that must make the netsim run."""


def git(root, *args):
    done = subprocess.run(["git", "-C", root, *args], capture_output=True, check=False)
    if done.returncode != 0:
        raise SelectorError(f"git {' '.join(args)}: {done.stderr.decode(errors='replace').strip()}")
    return done.stdout


def changed_paths(root, old, new):
    """Paths of `git diff old new`: a rename counts both paths, a deletion the old one."""
    fields = git(root, "diff", "--name-status", "-M", "-z", old, new).decode().split("\0")
    paths, i = [], 0
    while i < len(fields) and fields[i]:
        status = fields[i]
        count = 2 if status[0] in "RC" else 1
        paths += fields[i + 1 : i + 1 + count]
        i += 1 + count
    return paths


def prepush_paths(root, lines):
    paths = []
    for line in lines:
        parts = line.split()
        if not parts:
            continue
        if len(parts) != 4:
            raise SelectorError(f"malformed pre-push line: {line!r}")
        _, local, _, remote = parts
        if local == ZERO:  # deleting a remote ref pushes no code
            continue
        if remote == ZERO:  # new branch: what it adds over origin/main
            remote = git(root, "merge-base", "origin/main", local).decode().strip()
        paths += changed_paths(root, remote, local)
    return paths


def parse_stream(text):
    decoder, i, objs = json.JSONDecoder(), 0, []
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            return objs
        obj, i = decoder.raw_decode(text, i)
        objs.append(obj)


def go_packages(root):
    """Directories (relative to root) of the packages in the netsim binaries, and their file set."""
    injected = os.environ.get("INKWELL_AFFECTED_GOLIST")
    try:
        if injected:
            with open(injected, encoding="utf-8") as f:
                packages = parse_stream(f.read())
        else:
            packages = []
            for env, args in GO_TARGETS:
                done = subprocess.run(
                    ["go", "list", "-deps", "-json", *args],
                    cwd=root, env={**os.environ, **env}, capture_output=True, check=False,
                )
                if done.returncode != 0:
                    raise SelectorError(f"go list: {done.stderr.decode(errors='replace').strip()[:200]}")
                packages += parse_stream(done.stdout.decode())
    except (OSError, ValueError) as err:
        raise SelectorError(f"go list: {err}") from err
    real_root = os.path.realpath(root)
    dirs, files = set(), set()
    for pkg in packages:
        pkg_dir = os.path.realpath(pkg.get("Dir", ""))
        if pkg_dir != real_root and not pkg_dir.startswith(real_root + os.sep):
            continue  # standard library, module cache
        rel = os.path.relpath(pkg_dir, real_root)
        dirs.add("" if rel == "." else rel)
        for key in ("GoFiles", "CgoFiles", "EmbedFiles"):
            for name in pkg.get(key) or []:
                files.add(os.path.normpath(os.path.join(rel, name)))
    if not dirs:
        raise SelectorError("go list: no package under the repository")
    return dirs, files


def is_input(path, dirs, files):
    """True if a change to path can alter the netsim; False if inert; unknown paths are inputs."""
    base = os.path.basename(path)
    if path in INPUT_FILES or path.startswith("scripts/netsim/"):
        return True
    if path.startswith("frontend/"):
        # unit tests are inert, the Playwright specs under frontend/e2e are not
        return not (base.endswith(".test.ts") and not path.startswith("frontend/e2e/"))
    if path.startswith(INERT_PREFIXES) or fnmatch.fnmatch(base, "*.md"):
        return False
    if base.endswith(".go"):
        if base.endswith("_test.go"):
            return False
        return path in files or os.path.dirname(path) in dirs  # a tag-excluded file of a dependency too
    return True


def decide(paths, root):
    dirs, files = go_packages(root)
    seen = set()
    for path in paths:
        if path in seen:
            continue
        seen.add(path)
        if is_input(path, dirs, files):
            return RUN, f"netsim: run ({path})"
    return SKIP, f"netsim: skip (inert: {len(seen)} paths)"


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    mode = ap.add_mutually_exclusive_group(required=True)
    mode.add_argument("--paths", nargs="+", metavar="P")
    mode.add_argument("--pre-push", action="store_true")
    mode.add_argument("--base", metavar="B")
    ap.add_argument("--root", help="repository root (default: the git top level of the current directory)")
    args = ap.parse_args(argv)
    try:
        root = args.root or git(".", "rev-parse", "--show-toplevel").decode().strip()
        if args.paths:
            paths = args.paths
        elif args.base:
            paths = changed_paths(root, args.base, "HEAD")
        else:
            paths = prepush_paths(root, sys.stdin.read().splitlines())
        code, line = decide(paths, root)
    except Exception as err:  # fail towards running, whatever went wrong
        code, line = RUN, f"netsim: run (selector failure: {str(err)[:160]})"
    print(line)
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
