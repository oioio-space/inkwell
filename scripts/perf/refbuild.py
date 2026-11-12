"""The reference build of the paired performance criteria (block PB, plan section 7, ADR 0038).

The reference is the merge-base of HEAD with main, built in a private worktree by the same function
(`build_server`) and the same command vector (`VECTOR`) that builds the change under test, and cached
by commit and toolchain (`go env -json` where the reference's go.mod selects the toolchain, the wails3
and C compiler versions). Only the binary is cached. The source checkout's `.git` is never written
(a private bare clone with shared objects carries the worktree), and a reference that does not build,
or whose build dirties its worktree, fails the criterion (`ReferenceBuildError`), it is never skipped.
Nothing is downloaded at run time: the sidecars are copied from the source checkout when the pinned
manifest matches, else fetched at build time by `mise run sidecar:fetch` (ADR 0024); a missing manifest
or a failed fetch fails closed too. Go is the toolchain the tree pins (`[tools] go` of its mise.toml, installed by
mise), never the first `go` on PATH, and runs with GOTOOLCHAIN=local (`go_env`); an absent pin fails closed.

Standard library only. Command line: `python3 scripts/perf/refbuild.py` prints the reference record.
"""

from __future__ import annotations

import fcntl
import gzip
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import tempfile
import time
import tomllib
from pathlib import Path

BINARY = "inkwell-server"
KEEP = 6  # most recently used cache entries kept
MANIFEST = Path("internal/sidecar/manifest.json")
PNPM = ["pnpm", "--config.verifyDepsBeforeRun=false", "--dir", "frontend"]
KEEP_DIRTY = 2  # most recent dirtied reference worktrees kept for inspection (kept-*)
# One build vector for both sides, every step on both. "{out}" is the binary path. The frontend dependencies are
# installed against the tree's lockfile (frozen: a checkout whose node_modules drifted from its lockfile is
# reinstalled, never built as is), the Wails bindings are regenerated from the tree's Go code, and the Go build
# keeps neither checkout paths (-trimpath) nor a VCS stamp (-buildvcs=false), so that the same source gives the
# same binary in the reference worktree and in the checkout under test (A/A identity).
VECTOR = (
    dict(name="install", argv=[*PNPM, "install", "--frozen-lockfile", "--prefer-offline"]),
    dict(name="bindings", argv=["wails3", "generate", "bindings", "-ts", "-i", "./cmd/inkwell"]),
    dict(name="frontend", argv=[*PNPM, "exec", "vite", "build", "--configLoader", "runner", "--mode",
                                "production", "--outDir", "dist"]),
    dict(name="server", argv=["go", "build", "-tags", "server", "-trimpath", "-buildvcs=false", "-o", "{out}",
                              "./cmd/inkwell"]),
)
ENV_KEYS = ("GOFLAGS", "CGO_ENABLED", "GOAMD64", "GOARCH", "GOOS", "GOEXPERIMENT", "GOTOOLCHAIN", "CC", "CXX",
            "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS")
# Keys of `go env -json` that name a location or a download policy: they cannot change the binary (go.sum pins
# the modules' content) but differ between trees and sessions, so they stay out of the key.
GOENV_VOLATILE = ("GOMOD", "GOWORK", "GOTMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH", "GOBIN", "GOENV", "GOTELEMETRY",
                  "GOTELEMETRYDIR", "GOPROXY", "GONOPROXY", "GONOSUMDB", "GOPRIVATE", "GOSUMDB", "GOINSECURE", "GOVCS",
                  "GOAUTH")


class ReferenceBuildError(RuntimeError):
    """The reference could not be built: the criterion fails (never invalid, never skipped)."""


def go_env(mise_toml: str, base=None, run=subprocess.run) -> dict:
    """The environment that runs the Go toolchain a tree pins (`[tools] go` of its mise.toml, as mise installed it),
    whatever PATH holds: its bin directory first on PATH, GOROOT set to it, and GOTOOLCHAIN=local so that go.mod
    can never switch silently to another toolchain (a go.mod newer than the pin then fails the build)."""
    try:
        pin = tomllib.loads(mise_toml)["tools"]["go"]
        pin = pin["version"] if isinstance(pin, dict) else pin
    except (tomllib.TOMLDecodeError, KeyError, TypeError) as error:
        raise ReferenceBuildError(f"no Go pin in mise.toml ([tools] go): {error!r}") from None
    try:
        done = run(["mise", "where", f"go@{pin}"], capture_output=True, text=True)
    except OSError as error:
        raise ReferenceBuildError(f"mise unavailable, cannot resolve the pinned Go {pin}: {error}") from None
    root = Path(done.stdout.strip())
    if done.returncode or not done.stdout.strip() or not (root / "bin" / "go").is_file():
        raise ReferenceBuildError(f"Go {pin} pinned by mise.toml is not installed (mise install go@{pin}): "
                                  f"{done.stderr.strip()}")
    base = dict(os.environ if base is None else base)
    return dict(base, GOROOT=str(root), GOTOOLCHAIN="local",
                PATH=f"{root / 'bin'}{os.pathsep}{base.get('PATH', '')}")


def tree_go_env(tree: Path, base=None) -> dict:
    """`go_env` of the mise.toml at the root of `tree`."""
    try:
        text = (tree / "mise.toml").read_text()
    except OSError as error:
        raise ReferenceBuildError(f"no mise.toml in {tree}, so no pinned Go: {error}") from None
    return go_env(text, base)


def git(repo, *args, check=True) -> str:
    done = subprocess.run(["git", "-C", str(repo), *args], capture_output=True, text=True)
    if check and done.returncode:
        raise ReferenceBuildError(f"git {' '.join(args)} failed: {done.stderr.strip()}")
    return done.stdout.strip()


def merge_base(repo: Path, main: str = "main") -> str:
    """The reference commit: the merge-base of HEAD with main (HEAD itself on main)."""
    return git(repo, "merge-base", "HEAD", main)


def toolchain(run=subprocess.run, tree: Path | None = None, env=None) -> dict:
    """The versions and settings the build depends on: go, node, pnpm, wails3, the C compiler, and `go env -json`
    taken in `tree` (a directory holding the reference's go.mod, which selects the toolchain), without its
    location and download keys; the random build directory of GOGCCFLAGS is removed. `env` (`go_env`) selects the
    pinned Go; its GOROOT and GOVERSION are then in the record, hence in the cache key."""
    def version(*cmd, cwd=None):
        try:
            done = run(list(cmd), capture_output=True, text=True, cwd=cwd, env=env)
        except OSError as error:
            raise ReferenceBuildError(f"{cmd[0]} unavailable: {error}") from None
        return (done.stdout + done.stderr).strip()

    try:
        goenv = json.loads(version("go", "env", "-json", cwd=tree))
    except ValueError as error:
        raise ReferenceBuildError(f"go env -json failed: {error}") from None
    goenv = {k: v for k, v in goenv.items() if k not in GOENV_VOLATILE}
    if "GOGCCFLAGS" in goenv:
        goenv["GOGCCFLAGS"] = re.sub(r"/go-build\d+", "/go-build", goenv["GOGCCFLAGS"])
    cc = version(goenv.get("CC") or "cc", "--version").splitlines()
    return dict(go=version("go", "version", cwd=tree), node=version("node", "--version"),
                pnpm=version("pnpm", "--version"), wails3=version("wails3", "version"), cc=cc[0] if cc else "",
                goenv=goenv, platform=f"{platform.system()}/{platform.machine()}")


def reference_toolchain(repo: Path, commit: str) -> dict:
    """`toolchain` where the reference selects it: a scratch directory holding the commit's go.mod (and go.work
    when it has one), with the Go its mise.toml pins (`go_env`); no checkout is needed before a cache hit."""
    with tempfile.TemporaryDirectory(prefix="refbuild-env-") as scratch:
        for name in ("go.mod", "go.work", "mise.toml"):
            done = subprocess.run(["git", "-C", str(repo), "show", f"{commit}:{name}"], capture_output=True)
            if done.returncode == 0:
                (Path(scratch) / name).write_bytes(done.stdout)
        return toolchain(tree=Path(scratch), env=tree_go_env(Path(scratch)))


def cache_key(commit: str, tools: dict, vector=VECTOR, env=None, manifest: bytes = b"") -> str:
    """SHA-256 of everything that determines the binary."""
    env = os.environ if env is None else env
    inputs = dict(commit=commit, toolchain=tools, vector=list(vector),
                  env={k: env.get(k, "") for k in ENV_KEYS}, manifest=hashlib.sha256(manifest).hexdigest())
    return hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def build_server(tree: Path, out: Path, vector=VECTOR, run=subprocess.run, log=None) -> None:
    """Build the server of `tree` into `out` with the shared command vector, every step with the Go the tree
    pins (`tree_go_env`). `log`, a file object, receives the output of the commands."""
    env = tree_go_env(tree)
    out.parent.mkdir(parents=True, exist_ok=True)
    for step in vector:
        if step.get("unless_exists") and (tree / step["unless_exists"]).exists():
            continue
        argv = [a.replace("{out}", str(out)) for a in step["argv"]]
        done = run(argv, cwd=tree, env=env, stdout=log, stderr=subprocess.STDOUT if log else None)
        if done.returncode:
            raise ReferenceBuildError(f"reference build failed at {step['name']}: exit {done.returncode}")
    if not out.is_file():
        raise ReferenceBuildError(f"reference build produced no {out.name}")


def target() -> str:
    """The sidecar directory of this platform, as embedded by internal/sidecar (linux_amd64, ...)."""
    goos = {"Linux": "linux", "Darwin": "darwin"}.get(platform.system(), platform.system().lower())
    goarch = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine(), platform.machine())
    return f"{goos}_{goarch}"


def provision_sidecars(source: Path, tree: Path, fetch=None, triple: str | None = None) -> str:
    """Make the embedded sidecars of `tree`: copied from `source` when its pinned manifest equals the
    tree's and every file matches its pinned SHA-256 (of the decompressed binary), else fetched by
    `mise run sidecar:fetch`. Returns "copied" or "fetched"."""
    triple = triple or target()
    mine, theirs = source / MANIFEST, tree / MANIFEST
    for side, path in (("source", mine), ("reference", theirs)):
        if not path.is_file():
            raise ReferenceBuildError(f"no sidecar manifest in the {side} tree ({MANIFEST})")
    try:
        manifest = json.loads(theirs.read_text())
    except ValueError as error:
        raise ReferenceBuildError(f"unreadable sidecar manifest: {error}") from None
    if mine.read_bytes() == theirs.read_bytes():
        files = {}
        for name, sidecar in manifest["sidecars"].items():
            src = source / "internal/sidecar/bin" / triple / f"{name}.gz"
            pinned = sidecar["targets"].get(triple, {}).get("sha256")
            if not (pinned and src.is_file()):
                break
            with gzip.open(src) as f:
                if hashlib.sha256(f.read()).hexdigest() != pinned:
                    break
            files[name] = src
        else:
            dest = tree / "internal/sidecar/bin" / triple
            dest.mkdir(parents=True, exist_ok=True)
            for src in files.values():
                shutil.copyfile(src, dest / src.name)
            return "copied"
    try:
        (fetch or (lambda t: subprocess.run(["mise", "run", "sidecar:fetch"], cwd=t, check=True)))(tree)
    except (subprocess.CalledProcessError, OSError) as error:
        raise ReferenceBuildError(f"sidecar fetch failed: {error}") from None
    return "fetched"


def clean_worktree(tree: Path, kept: Path) -> None:
    """Remove the reference worktree after the build. Ignored files (node_modules, dist, bindings, the embedded
    sidecars) are build output; anything else (a modified or untracked file) means the build changed the
    reference, which is then no reference: the worktree is moved to `kept` (replacing an older copy of the same
    key, never nested into it; the KEEP_DIRTY most recent are kept) and ReferenceBuildError is raised, so
    nothing is published."""
    state = git(tree, "status", "--porcelain", "--ignored").splitlines()
    unexpected = [line for line in state if not line.startswith("!!")]
    if not unexpected:
        shutil.rmtree(tree)
        return
    shutil.rmtree(kept, ignore_errors=True)
    os.rename(tree, kept)  # inside the cache: one file system, no copy, no nesting
    os.utime(kept)
    for old in sorted(kept.parent.glob("kept-*"), key=lambda p: p.stat().st_mtime, reverse=True)[KEEP_DIRTY:]:
        shutil.rmtree(old, ignore_errors=True)
    raise ReferenceBuildError(f"the build dirtied the reference worktree (kept at {kept}): {unexpected[:5]}")


def ensure_reference(repo: Path, cache: Path, *, vector=VECTOR, run=subprocess.run, tools: dict | None = None,
                     fetch=None, keep: int = KEEP, main: str = "main", env=None) -> dict:
    """The reference binary at the merge-base: from the cache after re-hashing it, else built once (serialised
    by a lock) and published atomically. Returns commit, cache_digest, sha256, cache_hit, binary, build_seconds."""
    repo = repo.resolve()
    commit = merge_base(repo, main)
    tools = tools if tools is not None else reference_toolchain(repo, commit)
    manifest = (repo / MANIFEST).read_bytes() if (repo / MANIFEST).exists() else b""
    key = cache_key(commit, tools, vector, env, manifest)
    cache.mkdir(parents=True, exist_ok=True)
    with cache.with_name(cache.name + ".lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        entry = cache / key
        record = _hit(entry)
        if record:
            os.utime(entry / "build.json")
            return dict(record, cache_hit=True, binary=str(entry / BINARY))
        for stale in cache.glob("tmp-*"):  # a crashed build's leftovers: the lock makes us the only builder
            shutil.rmtree(stale, ignore_errors=True)
        shutil.rmtree(entry, ignore_errors=True)  # a tampered or partial entry
        work = Path(tempfile.mkdtemp(prefix="tmp-", dir=cache))
        try:
            started = time.monotonic()
            repository, tree = work / "repository.git", work / "tree"
            for cmd in (["git", "clone", "--bare", "--shared", "--quiet", str(repo), str(repository)],
                        ["git", f"--git-dir={repository}", "worktree", "add", "--quiet", "--detach", str(tree), commit]):
                done = subprocess.run(cmd, capture_output=True, text=True)
                if done.returncode:
                    raise ReferenceBuildError(f"reference checkout failed: {done.stderr.strip()}")
            provision_sidecars(repo, tree, fetch)
            staged = work / "out"
            with (work / "build.log").open("w") as log:
                try:
                    build_server(tree, staged / BINARY, vector, run, log)
                except ReferenceBuildError as error:
                    log.flush()
                    tail = (work / "build.log").read_text()[-2000:]
                    raise ReferenceBuildError(f"{error}\n{tail}") from None
            record = dict(commit=commit, cache_digest=key, sha256=sha256(staged / BINARY), toolchain=tools,
                          build_seconds=round(time.monotonic() - started, 1))
            (staged / "build.json").write_text(json.dumps(record, indent=2) + "\n")
            clean_worktree(tree, cache / f"kept-{key[:12]}")
            os.rename(staged, entry)  # atomic publication: an entry is either absent or complete
        finally:
            shutil.rmtree(work, ignore_errors=True)
        _evict(cache, keep)
        return dict(record, cache_hit=False, binary=str(entry / BINARY))


def _hit(entry: Path) -> dict | None:
    """The build record of a cache entry that names this entry and whose binary still has its recorded digest."""
    try:
        record = json.loads((entry / "build.json").read_text())
        ok = record["cache_digest"] == entry.name and sha256(entry / BINARY) == record["sha256"]
        return record if ok else None
    except (OSError, ValueError, KeyError):
        return None


def _evict(cache: Path, keep: int) -> None:
    entries = sorted((p for p in cache.iterdir() if (p / "build.json").exists()),
                     key=lambda p: (p / "build.json").stat().st_mtime, reverse=True)
    for old in entries[keep:]:
        shutil.rmtree(old, ignore_errors=True)


def default_cache() -> Path:
    return Path(os.environ.get("XDG_CACHE_HOME", Path.home() / ".cache")) / "inkwell" / "perf-ref"


if __name__ == "__main__":
    print(json.dumps(ensure_reference(Path.cwd(), default_cache()), indent=2))
