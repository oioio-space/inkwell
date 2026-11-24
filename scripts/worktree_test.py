#!/usr/bin/env python3
"""scripts/worktree.sh makes a block's worktree usable at once, on a temporary repository with a fake `mise`.

The fake records each call (arguments and folder) and fails `run sidecar:fetch` when told to, so the test never
fetches anything. The repository is named `inkwell`, so the worktree lands at its sibling `inkwell-<block>`.
"""
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest

SCRIPTS = pathlib.Path(__file__).resolve().parent
SCRIPT = SCRIPTS / "worktree.sh"
SECRET = '[env]\nOPENROUTER_API_KEY = "sk-secret"\n'
FAKE_MISE = """#!/bin/sh
printf '%s|%s\\n' "$PWD" "$*" >> "$FAKE_MISE_LOG"
[ "$*" = "run sidecar:fetch" ] && [ -n "$FAKE_MISE_FAIL" ] && exit 7
exit 0
"""


def git(root, *args):
    return subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@t", *args], cwd=root, check=True,
                          capture_output=True, text=True).stdout


class Worktree(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="worktree-test-"))
        self.addCleanup(shutil.rmtree, self.tmp)
        self.main = self.tmp / "inkwell"
        self.main.mkdir()
        (self.main / "mise.toml").write_text("[tasks]\n")
        (self.main / ".gitignore").write_text("mise.local.toml\n")
        git(self.main, "init", "-q", "-b", "main")
        git(self.main, "add", "-A")
        git(self.main, "commit", "-q", "-m", "base")
        (self.main / "mise.local.toml").write_text(SECRET)
        self.bin = self.tmp / "bin"
        self.bin.mkdir()
        (self.bin / "mise").write_text(FAKE_MISE)
        (self.bin / "mise").chmod(0o755)
        self.log = self.tmp / "mise.log"
        self.env = {"PATH": f"{self.bin}:{os.environ['PATH']}", "HOME": str(self.tmp), "FAKE_MISE_LOG": str(self.log),
                    "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1"}
        self.path = self.tmp / "inkwell-B1"

    def run_script(self, *args, cwd=None, **env):
        return subprocess.run(["bash", str(SCRIPT), *args], cwd=cwd or self.main, env={**self.env, **env},
                              capture_output=True, text=True, timeout=60)

    def calls(self):
        return self.log.read_text().splitlines() if self.log.exists() else []

    def branches(self):
        return git(self.main, "branch", "--format=%(refname:short)").split()

    def test_worktree_is_ready(self):
        p = self.run_script("B1")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertEqual(p.stdout.splitlines()[-1], str(self.path))
        self.assertEqual(git(self.path, "branch", "--show-current").strip(), "B1")
        link = self.path / "mise.local.toml"
        self.assertTrue(link.is_symlink(), "the secrets are linked, never copied")
        self.assertEqual(link.resolve(), (self.main / "mise.local.toml").resolve())
        self.assertEqual(git(self.path, "status", "--porcelain", "--untracked-files=all"), "", "git sees nothing new")
        self.assertEqual(self.calls(), [f"{self.main}|trust --quiet {self.path}/mise.toml",
                                        f"{self.main}|trust --quiet {self.path}/mise.local.toml",
                                        f"{self.path}|run sidecar:fetch"])

    def test_from_another_worktree_lands_beside_the_main_tree(self):
        self.assertEqual(self.run_script("B1").returncode, 0)
        p = self.run_script("B2", cwd=self.path)
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertTrue((self.tmp / "inkwell-B2/mise.local.toml").is_symlink())

    def test_base_is_chosen(self):
        first = git(self.main, "rev-parse", "HEAD").strip()
        (self.main / "f").write_text("x")
        git(self.main, "add", "f")
        git(self.main, "commit", "-q", "-m", "second")
        self.assertEqual(self.run_script("B1", first).returncode, 0)
        self.assertEqual(git(self.path, "rev-parse", "HEAD").strip(), first)

    def test_existing_path_refused(self):
        self.path.mkdir()
        p = self.run_script("B1")
        self.assertEqual(p.returncode, 1)
        self.assertIn("already exists", p.stderr)
        self.assertEqual(self.branches(), ["main"])
        self.assertEqual(self.calls(), [])

    def test_existing_branch_refused(self):
        git(self.main, "branch", "B1")
        p = self.run_script("B1")
        self.assertEqual(p.returncode, 1)
        self.assertIn("branch B1 already exists", p.stderr)
        self.assertFalse(self.path.exists())

    def test_bad_arguments_refused(self):
        for args in ([], ["../x"], ["-x"], ["B1", "main", "extra"]):
            self.assertEqual(self.run_script(*args).returncode, 2, args)
        self.assertEqual(self.run_script("B1", "nosuchbase").returncode, 1)
        self.assertEqual(self.branches(), ["main"])

    def test_secrets_not_ignored_refused_and_undone(self):
        (self.main / ".gitignore").write_text("")
        git(self.main, "commit", "-q", "-am", "unignore")
        p = self.run_script("B1")
        self.assertEqual(p.returncode, 1)
        self.assertIn("not ignored", p.stderr)
        self.assertFalse(self.path.exists(), "a failed run leaves no worktree")
        self.assertEqual(self.branches(), ["main"])

    def test_failed_fetch_undone(self):
        p = self.run_script("B1", FAKE_MISE_FAIL="1")
        self.assertNotEqual(p.returncode, 0)
        self.assertFalse(self.path.exists())
        self.assertEqual(self.branches(), ["main"])
        self.assertIn("main", git(self.main, "worktree", "list"))
        self.assertEqual(len(git(self.main, "worktree", "list").splitlines()), 1)

    def test_without_secrets_still_ready(self):
        (self.main / "mise.local.toml").unlink()
        p = self.run_script("B1")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("no API keys", p.stderr)
        self.assertFalse((self.path / "mise.local.toml").exists())


if __name__ == "__main__":
    unittest.main()
