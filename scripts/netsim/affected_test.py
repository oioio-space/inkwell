"""Tests of scripts/netsim/affected.py, one per rule of gate CI1 (gates/CI1.toml).

They run the selector as a subprocess on a temporary git repository, with a fake `go list`
(INKWELL_AFFECTED_GOLIST), so the decision is checked end to end, exit code included.
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest

SELECTOR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "affected.py")
RUN, SKIP = 0, 10
ZERO = "0" * 40


def git(root, *args):
    env = {**os.environ, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t",
           "GIT_COMMITTER_EMAIL": "t@t", "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_SYSTEM": os.devnull}
    done = subprocess.run(["git", "-C", root, *args], capture_output=True, text=True, env=env, check=True)
    return done.stdout.strip()


def write(root, rel, text="x\n"):
    path = os.path.join(root, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


class Base(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = os.path.realpath(tmp.name)
        git(self.root, "init", "-q", "-b", "main")
        for rel in ("go.mod", "internal/dep/a.go", "internal/dep/a_test.go", "internal/dep/asset.txt",
                    "internal/other/b.go", "docs/x.md", "frontend/src/x.ts", "scripts/netsim/run.sh"):
            write(self.root, rel)
        # the fake `go list -deps -json`: a stream of objects, the standard library first
        self.golist = os.path.join(self.root, "..", os.path.basename(self.root) + "-golist.json")
        self.write_golist([
            {"Dir": "/usr/lib/go/src/fmt", "GoFiles": ["print.go"], "Standard": True},
            {"Dir": os.path.join(self.root, "internal/dep"), "GoFiles": ["a.go"], "EmbedFiles": ["asset.txt"],
             "Module": {"Main": True}},
        ])
        self.addCleanup(lambda: os.path.exists(self.golist) and os.remove(self.golist))
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", "base")
        self.base = git(self.root, "rev-parse", "HEAD")

    def write_golist(self, packages):
        with open(self.golist, "w", encoding="utf-8") as f:
            f.write("\n".join(json.dumps(p, indent=1) for p in packages))

    def select(self, *args, stdin="", golist=None):
        env = {**os.environ, "INKWELL_AFFECTED_GOLIST": golist or self.golist}
        done = subprocess.run([sys.executable, "-I", SELECTOR, "--root", self.root, *args],
                              input=stdin, capture_output=True, text=True, env=env, check=False)
        return done.returncode, done.stdout.strip()

    def paths(self, *paths, golist=None):
        return self.select("--paths", *paths, golist=golist)

    def commit_change(self, edit):
        edit()
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", "change")
        return git(self.root, "rev-parse", "HEAD")


class TestCI1_2_InertList(Base):
    def test_docs_only_diff_skips(self):
        code, line = self.paths("docs/x.md", "docs/plans/p.md", "gates/CI1.toml", ".claude/settings.json",
                                "scripts/perf/p.py", "README.md", "docs/protocol/tla/m.tla")
        self.assertEqual((code, line), (SKIP, "netsim: skip (inert: 7 paths)"))

    def test_test_file_of_a_dependency_skips(self):
        self.assertEqual(self.paths("internal/dep/a_test.go")[0], SKIP)

    def test_go_file_of_a_package_outside_the_dependency_set_skips(self):
        self.assertEqual(self.paths("internal/other/b.go")[0], SKIP)

    def test_frontend_unit_test_skips(self):
        self.assertEqual(self.paths("frontend/src/x.test.ts")[0], SKIP)

    def test_unknown_path_runs(self):
        code, line = self.paths("tools/unheard.bin")
        self.assertEqual((code, line), (RUN, "netsim: run (tools/unheard.bin)"))


class TestCI1_1_InputSet(Base):
    def test_non_test_go_file_of_a_dependency_runs(self):
        self.assertEqual(self.paths("internal/dep/a.go"), (RUN, "netsim: run (internal/dep/a.go)"))

    def test_embedded_asset_runs(self):
        self.assertEqual(self.paths("internal/dep/asset.txt")[0], RUN)

    def test_go_mod_runs(self):
        self.assertEqual(self.paths("go.mod")[0], RUN)

    def test_frontend_source_runs(self):
        self.assertEqual(self.paths("frontend/src/x.ts")[0], RUN)

    def test_frontend_e2e_test_runs(self):
        self.assertEqual(self.paths("frontend/e2e/x.test.ts")[0], RUN)

    def test_netsim_script_runs(self):
        self.assertEqual(self.paths("scripts/netsim/run.sh")[0], RUN)

    def test_tooling_inputs_run(self):
        for p in ("go.sum", "mise.toml", "scripts/heavy.sh", "scripts/memcap.sh"):
            with self.subTest(p):
                self.assertEqual(self.paths(p)[0], RUN)

    def test_first_triggering_path_is_reported(self):
        self.assertEqual(self.paths("docs/x.md", "go.mod", "mise.toml"), (RUN, "netsim: run (go.mod)"))


class TestCI1_3_DiffComputation(Base):
    def test_rename_from_inert_to_input_runs(self):
        head = self.commit_change(lambda: git(self.root, "mv", "docs/x.md", "internal/dep/moved.go"))
        self.assertEqual(self.select("--base", self.base), (RUN, "netsim: run (internal/dep/moved.go)"))
        self.assertEqual(head, git(self.root, "rev-parse", "HEAD"))

    def test_rename_from_input_to_inert_runs(self):
        self.commit_change(lambda: git(self.root, "mv", "internal/dep/a.go", "docs/a.md"))
        self.assertEqual(self.select("--base", self.base), (RUN, "netsim: run (internal/dep/a.go)"))

    def test_deleted_input_file_runs(self):
        self.commit_change(lambda: git(self.root, "rm", "-q", "internal/dep/a.go"))
        self.assertEqual(self.select("--base", self.base), (RUN, "netsim: run (internal/dep/a.go)"))

    def test_inert_diff_skips(self):
        self.commit_change(lambda: write(self.root, "docs/y.md", "y\n"))
        self.assertEqual(self.select("--base", self.base), (SKIP, "netsim: skip (inert: 1 paths)"))

    def test_pre_push_diffs_remote_to_local(self):
        head = self.commit_change(lambda: write(self.root, "go.mod", "changed\n"))
        line = f"refs/heads/b {head} refs/heads/b {self.base}\n"
        self.assertEqual(self.select("--pre-push", stdin=line), (RUN, "netsim: run (go.mod)"))

    def test_pre_push_every_ref_is_examined(self):
        docs = self.commit_change(lambda: write(self.root, "docs/y.md", "y\n"))
        code = self.commit_change(lambda: write(self.root, "go.sum", "z\n"))
        stdin = f"refs/heads/a {docs} refs/heads/a {self.base}\nrefs/heads/b {code} refs/heads/b {docs}\n"
        self.assertEqual(self.select("--pre-push", stdin=stdin), (RUN, "netsim: run (go.sum)"))

    def test_pre_push_new_branch_uses_merge_base_with_origin_main(self):
        git(self.root, "update-ref", "refs/remotes/origin/main", self.base)
        head = self.commit_change(lambda: write(self.root, "docs/y.md", "y\n"))
        head = self.commit_change(lambda: write(self.root, "docs/z.md", "z\n"))
        line = f"refs/heads/new {head} refs/heads/new {ZERO}\n"
        # two inert commits over the merge-base: both files are counted
        self.assertEqual(self.select("--pre-push", stdin=line), (SKIP, "netsim: skip (inert: 2 paths)"))

    def test_pre_push_new_branch_with_an_input_runs(self):
        git(self.root, "update-ref", "refs/remotes/origin/main", self.base)
        self.commit_change(lambda: write(self.root, "docs/y.md", "y\n"))
        head = self.commit_change(lambda: write(self.root, "mise.toml", "m\n"))
        line = f"refs/heads/new {head} refs/heads/new {ZERO}\n"
        self.assertEqual(self.select("--pre-push", stdin=line), (RUN, "netsim: run (mise.toml)"))

    def test_pre_push_deleted_local_ref_is_ignored(self):
        line = f"(delete) {ZERO} refs/heads/gone {self.base}\n"
        self.assertEqual(self.select("--pre-push", stdin=line), (SKIP, "netsim: skip (inert: 0 paths)"))

    def test_unresolvable_base_runs(self):
        self.assertEqual(self.select("--base", "no-such-ref")[0], RUN)

    def test_pre_push_unknown_remote_sha_runs(self):
        line = f"refs/heads/b {self.base} refs/heads/b {'1' * 40}\n"
        self.assertEqual(self.select("--pre-push", stdin=line)[0], RUN)

    def test_new_branch_without_origin_main_runs(self):
        line = f"refs/heads/new {self.base} refs/heads/new {ZERO}\n"
        self.assertEqual(self.select("--pre-push", stdin=line)[0], RUN)


class TestCI1_3_GoListFailure(Base):
    def test_go_list_failing_runs_even_for_an_inert_path(self):
        code, line = self.paths("docs/x.md", golist=os.path.join(self.root, "missing.json"))
        self.assertEqual(code, RUN)
        self.assertIn("go list", line)

    def test_unparsable_go_list_output_runs(self):
        with open(self.golist, "w", encoding="utf-8") as f:
            f.write("{not json")
        self.assertEqual(self.paths("docs/x.md")[0], RUN)

    def test_go_list_without_any_repository_package_runs(self):
        self.write_golist([{"Dir": "/usr/lib/go/src/fmt", "GoFiles": ["print.go"]}])
        self.assertEqual(self.paths("docs/x.md")[0], RUN)


if __name__ == "__main__":
    unittest.main()
