"""Tests of refbuild.py, the reference build of block PB (plan section 7). A temporary git repository and a
stub build vector stand for the checkout and the real build; no network, no browser.
Run from the repository root: ./scripts/heavy.sh python3 -m unittest -v scripts/perf/refbuild_test.py
"""

import gzip
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import refbuild  # noqa: E402

TOOLS = dict(go="go1.27", node="v24", pnpm="11", platform="test")
STUB = (dict(name="stub", argv=["sh", "-c", 'cat VERSION > "$1"', "sh", "{out}"]),)
ENV = {}


def git(repo, *args):
    done = subprocess.run(["git", "-C", str(repo), "-c", "user.name=t", "-c", "user.email=t@t", *args],
                          capture_output=True, text=True, check=True)
    return done.stdout.strip()


def commit(repo, version, message=None):
    (repo / "VERSION").write_text(version)
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "-m", message or version)
    return git(repo, "rev-parse", "HEAD")


def sidecar_payload():
    return b"typst-binary-bytes"


class Fixture(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = Path(self.tmp.name)
        self.repo, self.cache = root / "repo", root / "cache" / "perf-ref"
        self.repo.mkdir()
        git(self.repo, "init", "-q", "-b", "main")
        (self.repo / ".gitignore").write_text("internal/sidecar/bin/*/*\nnode_modules\n")
        (self.repo / "internal/sidecar").mkdir(parents=True)
        pinned = hashlib.sha256(sidecar_payload()).hexdigest()
        self.manifest = {"schema": 1, "sidecars": {"typst": {"targets": {refbuild.target(): {"sha256": pinned}}}}}
        (self.repo / refbuild.MANIFEST).write_text(json.dumps(self.manifest))
        self.base = commit(self.repo, "v1")
        bindir = self.repo / "internal/sidecar/bin" / refbuild.target()
        bindir.mkdir(parents=True)
        with gzip.open(bindir / "typst.gz", "wb") as f:
            f.write(sidecar_payload())
        self.builds = []

    def ensure(self, **kw):
        kw.setdefault("vector", STUB)
        kw.setdefault("tools", TOOLS)
        kw.setdefault("env", ENV)
        return refbuild.ensure_reference(self.repo, self.cache, **kw)

    def counting_run(self, delay=0.0, active=None):
        def run(argv, **kw):
            self.builds.append(argv)
            if active is not None:
                active.append(1)
                self.max_active = max(self.max_active, len(active))
            time.sleep(delay)
            result = subprocess.run(argv, **kw)
            if active is not None:
                active.pop()
            return result

        return run


class Reference(Fixture):
    def test_reference_is_the_merge_base(self):
        """On main the reference is HEAD; on a branch it is the fork point, whatever the branch holds."""
        self.assertEqual(refbuild.merge_base(self.repo), self.base)
        git(self.repo, "checkout", "-q", "-b", "feature")
        commit(self.repo, "v2")
        commit(self.repo, "v3")
        self.assertEqual(refbuild.merge_base(self.repo), self.base)
        record = self.ensure()
        self.assertEqual(record["commit"], self.base)
        self.assertEqual(Path(record["binary"]).read_text(), "v1")  # the reference's tree, not the branch's
        with self.assertRaises(refbuild.ReferenceBuildError):
            refbuild.merge_base(self.repo, "no-such-branch")

    def test_cache_key_covers_commit_and_toolchain(self):
        key = refbuild.cache_key("c1", TOOLS, STUB, ENV, b"m")
        self.assertEqual(key, refbuild.cache_key("c1", dict(TOOLS), STUB, dict(ENV), b"m"))
        variants = [refbuild.cache_key("c2", TOOLS, STUB, ENV, b"m"),
                    refbuild.cache_key("c1", dict(TOOLS, go="go1.28"), STUB, ENV, b"m"),
                    refbuild.cache_key("c1", dict(TOOLS, node="v25"), STUB, ENV, b"m"),
                    refbuild.cache_key("c1", dict(TOOLS, pnpm="12"), STUB, ENV, b"m"),
                    refbuild.cache_key("c1", TOOLS, refbuild.VECTOR, ENV, b"m"),
                    refbuild.cache_key("c1", TOOLS, STUB, {"GOFLAGS": "-race"}, b"m"),
                    refbuild.cache_key("c1", TOOLS, STUB, ENV, b"other manifest")]
        self.assertEqual(len({key, *variants}), 8)

    def test_hit_verifies_digest(self):
        first = self.ensure(run=self.counting_run())
        self.assertFalse(first["cache_hit"])
        self.assertEqual(len(self.builds), 1)
        second = self.ensure(run=self.counting_run())
        self.assertTrue(second["cache_hit"])
        self.assertEqual(len(self.builds), 1)  # not rebuilt
        self.assertEqual((second["commit"], second["sha256"], second["cache_digest"]),
                         (first["commit"], first["sha256"], first["cache_digest"]))
        self.assertEqual(second["sha256"], hashlib.sha256(b"v1").hexdigest())

    def test_tampered_entry_is_rebuilt(self):
        first = self.ensure(run=self.counting_run())
        Path(first["binary"]).write_text("tampered")
        again = self.ensure(run=self.counting_run())
        self.assertFalse(again["cache_hit"])
        self.assertEqual(len(self.builds), 2)
        self.assertEqual(Path(again["binary"]).read_text(), "v1")
        # A missing build record is as bad as a wrong digest.
        (Path(again["binary"]).parent / "build.json").unlink()
        self.assertFalse(self.ensure(run=self.counting_run())["cache_hit"])

    def test_publish_is_atomic(self):
        """While the build runs, the entry does not exist yet and the work is in a sibling temporary
        directory; a failed build publishes nothing; a published entry is complete."""
        seen = []

        def run(argv, **kw):
            seen.append(sorted(p.name for p in self.cache.iterdir()))
            return subprocess.run(argv, **kw)

        record = self.ensure(run=run)
        key = record["cache_digest"]
        self.assertTrue(all(key not in names for names in seen), seen)
        self.assertTrue(any(n.startswith("tmp-") for n in seen[0]), seen)
        entry = self.cache / key
        self.assertEqual(sorted(p.name for p in entry.iterdir()), ["build.json", refbuild.BINARY])
        self.assertEqual(sorted(p.name for p in self.cache.iterdir()), [key])  # work removed
        failing = (dict(name="boom", argv=["false"]),)
        commit(self.repo, "v2")
        with self.assertRaises(refbuild.ReferenceBuildError):
            self.ensure(vector=failing)
        self.assertEqual(sorted(p.name for p in self.cache.iterdir()), [key])

    def test_source_git_is_never_written(self):
        def snapshot():
            files = {}
            for p in sorted((self.repo / ".git").rglob("*")):
                if p.is_file():
                    st = p.stat()
                    files[str(p)] = (st.st_mtime_ns, st.st_size, hashlib.sha256(p.read_bytes()).hexdigest())
            return files

        before = snapshot()
        self.ensure()
        self.ensure()
        self.assertEqual(snapshot(), before)

    def test_both_sides_share_the_build_vector(self):
        """ensure_reference and a build of the checkout under test run the same commands (default vector)
        in the same order, differing only in the tree and the output path."""
        ref, branch = [], []

        def spy(sink):
            def run(argv, **kw):
                sink.append((list(argv), Path(kw["cwd"])))
                if "-o" in argv:
                    Path(argv[argv.index("-o") + 1]).write_bytes(b"bin")
                return subprocess.CompletedProcess(argv, 0)

            return run

        self.ensure(vector=refbuild.VECTOR, run=spy(ref))
        refbuild.build_server(self.repo, self.cache.parent / "branch" / refbuild.BINARY, run=spy(branch))
        norm = lambda steps: [[a if "/" not in a or a.startswith("./") else "<out>" for a in argv] for argv, _ in steps]
        self.assertEqual(norm(ref), norm(branch))
        self.assertIn("-trimpath", ref[-1][0])
        self.assertNotEqual(ref[-1][1], self.repo)  # a private worktree, not the checkout
        self.assertEqual(branch[-1][1], self.repo)

    def test_sidecars_copied_only_when_manifest_matches(self):
        tree = self.cache.parent / "tree"
        (tree / "internal/sidecar").mkdir(parents=True)
        (tree / refbuild.MANIFEST).write_bytes((self.repo / refbuild.MANIFEST).read_bytes())
        fetched = []
        self.assertEqual(refbuild.provision_sidecars(self.repo, tree, fetched.append), "copied")
        copied = tree / "internal/sidecar/bin" / refbuild.target() / "typst.gz"
        self.assertEqual(gzip.open(copied).read(), sidecar_payload())
        self.assertEqual(fetched, [])
        # A different pin: fetched, never copied.
        other = dict(self.manifest, schema=2)
        (tree / refbuild.MANIFEST).write_text(json.dumps(other))
        self.assertEqual(refbuild.provision_sidecars(self.repo, tree, fetched.append), "fetched")
        self.assertEqual(fetched, [tree])
        # The same manifest but a source file that no longer matches its pin: fetched.
        (tree / refbuild.MANIFEST).write_bytes((self.repo / refbuild.MANIFEST).read_bytes())
        src = self.repo / "internal/sidecar/bin" / refbuild.target() / "typst.gz"
        with gzip.open(src, "wb") as f:
            f.write(b"corrupt")
        fetched.clear()
        self.assertEqual(refbuild.provision_sidecars(self.repo, tree, fetched.append), "fetched")
        self.assertEqual(fetched, [tree])

    def test_concurrent_builds_are_serialised(self):
        self.max_active = 0
        active = []
        results = []

        def go():
            results.append(self.ensure(run=self.counting_run(delay=0.3, active=active)))

        threads = [threading.Thread(target=go) for _ in range(3)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        self.assertEqual(self.max_active, 1)
        self.assertEqual(len(self.builds), 1)  # the others waited for the lock, then hit
        self.assertEqual(sorted(r["cache_hit"] for r in results), [False, True, True])

    def test_eviction_keeps_six_recent(self):
        keys = []
        for i in range(8):
            commit(self.repo, f"v{i + 2}")
            keys.append(self.ensure()["cache_digest"])
            time.sleep(0.02)
        present = {p.name for p in self.cache.iterdir()}
        self.assertEqual(present, set(keys[-6:]))
        # A hit refreshes the entry: the oldest kept entry survives the next build.
        self.ensure(env={"GOFLAGS": "-x"})  # a ninth distinct key at the same commit
        self.assertEqual(len(list(self.cache.iterdir())), 6)

    def test_dirtied_reference_worktree_fails_closed(self):
        """A build that modifies a tracked file or leaves an untracked one in the reference worktree is not a
        reference build: ReferenceBuildError, nothing published, the tree kept for inspection (moved, never
        nested into an older copy), and only the two most recent kept trees remain."""
        dirty = (dict(name="dirty", argv=["sh", "-c", 'echo x >> VERSION; touch stray; cat VERSION > "$1"', "sh", "{out}"]),)
        for i in range(4):
            commit(self.repo, f"v{i + 2}")
            with self.assertRaises(refbuild.ReferenceBuildError) as cm:
                self.ensure(vector=dirty)
            self.assertIn("dirtied", str(cm.exception))
            time.sleep(0.02)
        names = sorted(p.name for p in self.cache.iterdir())
        self.assertEqual(len(names), 2, names)
        self.assertTrue(all(n.startswith("kept-") for n in names), names)  # no entry, no work left
        for kept in self.cache.iterdir():
            self.assertTrue((kept / "stray").is_file())
            self.assertFalse((kept / "tree").exists())  # not nested
        with self.assertRaises(refbuild.ReferenceBuildError):
            self.ensure(vector=dirty)
        with self.assertRaises(refbuild.ReferenceBuildError):
            self.ensure(vector=dirty)  # the same key again: replaces its kept tree, never nests into it
        newest = max(self.cache.iterdir(), key=lambda p: p.stat().st_mtime)
        self.assertFalse((newest / "tree").exists())

    def test_vector_regenerates_bindings_and_verifies_dependencies(self):
        """Both sides run every step: the frontend dependencies are installed against the lockfile (frozen), the
        Wails bindings are generated, whatever the tree already holds; the Go build keeps no VCS stamp."""
        steps = {s["name"]: s for s in refbuild.VECTOR}
        self.assertEqual(list(steps), ["install", "bindings", "frontend", "server"])
        self.assertTrue(all("unless_exists" not in s for s in refbuild.VECTOR))
        self.assertIn("--frozen-lockfile", steps["install"]["argv"])
        self.assertEqual(steps["bindings"]["argv"][:3], ["wails3", "generate", "bindings"])
        self.assertIn("-buildvcs=false", steps["server"]["argv"])
        self.assertIn("-trimpath", steps["server"]["argv"])

    def test_cache_key_covers_the_build_environment(self):
        """The toolchain record holds `go env -json` taken where the reference's go.mod selects the toolchain, the
        wails3 and C compiler versions; GOEXPERIMENT, GOTOOLCHAIN, CC and the cgo flags each change the key, the
        location of the tree does not."""
        calls = []

        def run(argv, **kw):
            calls.append((argv, kw.get("cwd")))
            if argv[:3] == ["go", "env", "-json"]:
                env = dict(GOVERSION="go1.27.3", GOEXPERIMENT="", GOTOOLCHAIN="auto", CC="gcc", CGO_CFLAGS="-O2 -g",
                           CGO_LDFLAGS="-O2 -g", GOMOD=f"{kw['cwd']}/go.mod",
                           GOGCCFLAGS=f"-fPIC -m64 -ffile-prefix-map=/tmp/go-build{len(calls)}=/tmp/go-build")
                return subprocess.CompletedProcess(argv, 0, json.dumps(env), "")
            return subprocess.CompletedProcess(argv, 0, " ".join(argv) + " 1.0", "")

        one = refbuild.toolchain(run, Path("/x/tree-1"))
        two = refbuild.toolchain(run, Path("/y/other-tree"))
        self.assertEqual(one, two)  # GOMOD and the random GOGCCFLAGS path do not enter the key
        self.assertIn(("go env -json".split(), Path("/x/tree-1")), [(a, c) for a, c in calls])
        self.assertTrue(any(a[:2] == ["wails3", "version"] for a, _ in calls))
        self.assertTrue(any(a[:2] == ["gcc", "--version"] for a, _ in calls))
        key = lambda tools: refbuild.cache_key("c1", tools, STUB, ENV, b"m")  # noqa: E731
        for name, value in (("GOEXPERIMENT", "jsonv2"), ("GOTOOLCHAIN", "go1.28.0"), ("CC", "clang"),
                            ("CGO_CFLAGS", "-O3"), ("CGO_LDFLAGS", "-s")):
            self.assertNotEqual(key(one), key(dict(one, goenv=dict(one["goenv"], **{name: value}))), name)
        self.assertNotEqual(key(one), key(dict(one, wails3="v3.0.0-alpha.101")))
        self.assertNotEqual(key(one), key(dict(one, cc="gcc 17")))
        # ensure_reference takes it in a directory that holds the reference commit's go.mod.
        (self.repo / "go.mod").write_text("module example.com/x\n\ngo 1.27\n")
        commit(self.repo, "with go.mod")
        seen = []
        real = refbuild.toolchain
        refbuild.toolchain = lambda run=None, tree=None: (seen.append((tree / "go.mod").read_text()), TOOLS)[1]
        try:
            refbuild.ensure_reference(self.repo, self.cache, vector=STUB, env=ENV)
        finally:
            refbuild.toolchain = real
        self.assertEqual(seen, ["module example.com/x\n\ngo 1.27\n"])

    def test_missing_manifest_or_failed_fetch_fails_closed(self):
        """No pinned manifest on either side, or a sidecar fetch that fails: ReferenceBuildError, never a crash
        of another kind and never a reference without its sidecars."""
        tree = self.cache.parent / "tree"
        (tree / "internal/sidecar").mkdir(parents=True)
        with self.assertRaises(refbuild.ReferenceBuildError):
            refbuild.provision_sidecars(self.repo, tree, lambda t: None)  # the tree has no manifest
        (tree / refbuild.MANIFEST).write_text(json.dumps(dict(self.manifest, schema=2)))

        def failing(t):
            raise subprocess.CalledProcessError(1, ["mise", "run", "sidecar:fetch"])

        with self.assertRaises(refbuild.ReferenceBuildError) as cm:
            refbuild.provision_sidecars(self.repo, tree, failing)
        self.assertIn("sidecar", str(cm.exception))
        (self.repo / refbuild.MANIFEST).unlink()
        with self.assertRaises(refbuild.ReferenceBuildError):
            refbuild.provision_sidecars(self.repo, tree, lambda t: None)  # the source has none
        git(self.repo, "rm", "-q", "--cached", str(refbuild.MANIFEST))
        commit(self.repo, "no manifest")
        with self.assertRaises(refbuild.ReferenceBuildError):
            self.ensure()
        self.assertEqual([p.name for p in self.cache.iterdir()], [])

    def test_build_vector_is_reproducible_across_trees(self):
        """The Go step of the vector builds the same source in two checkouts at different commits, one of them
        dirty, into identical binaries (-trimpath, -buildvcs=false): an A/A comparison can be an identity.
        Without -buildvcs=false the VCS stamp makes them differ (the test can fail)."""
        server = next(s for s in refbuild.VECTOR if s["name"] == "server")["argv"]
        trees = []
        for i in range(2):
            tree = Path(self.tmp.name) / f"go-{i}"
            tree.mkdir()
            (tree / "go.mod").write_text("module example.com/repro\n\ngo 1.27\n")
            (tree / "main.go").write_text('package main\n\nimport "fmt"\n\nfunc main() { fmt.Println("repro") }\n')
            git(tree, "init", "-q", "-b", "main")
            commit(tree, f"tree {i}", message=f"commit {i}")
            trees.append(tree)
        (trees[1] / "VERSION").write_text("dirty")

        def build(argv):
            digests = []
            for tree in trees:
                out = tree.parent / f"{tree.name}.bin"
                cmd = [a.replace("{out}", str(out)).replace("./cmd/inkwell", ".") for a in argv]
                subprocess.run(cmd, cwd=tree, check=True, capture_output=True, env=dict(os.environ, GOFLAGS=""))
                digests.append(refbuild.sha256(out))
            return digests

        same = build(server)
        self.assertEqual(same[0], same[1])
        stamped = build([a for a in server if a != "-buildvcs=false"])
        self.assertNotEqual(stamped[0], stamped[1])

    def test_unbuildable_reference_fails_closed(self):
        with self.assertRaises(refbuild.ReferenceBuildError) as cm:
            self.ensure(vector=(dict(name="compile", argv=["sh", "-c", "echo broken >&2; exit 3"]),))
        self.assertIn("reference build failed", str(cm.exception))
        self.assertIn("broken", str(cm.exception))  # the build log is in the error
        self.assertEqual(list(self.cache.iterdir()), [])
        with self.assertRaises(refbuild.ReferenceBuildError):  # a vector that builds nothing is not a build
            self.ensure(vector=(dict(name="noop", argv=["true"]),))


if __name__ == "__main__":
    unittest.main()
