"""scripts/port-worktree.sh: a worktree is ported by its diff against its own HEAD, so main commits made after the
worktree started are never reverted (2026-11-24: W80 and W81 were lost by a diff taken from a later main commit)."""

import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "port-worktree.sh"


def git(cwd, *args):
    return subprocess.run(["git", "-C", str(cwd), "-c", "user.name=t", "-c", "user.email=t@example.invalid", *args],
                          check=True, capture_output=True, text=True).stdout


class PortWorktree(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="inkwell-port-")
        self.addCleanup(tmp.cleanup)
        t = pathlib.Path(tmp.name)
        self.main, self.wt = t / "main", t / "wt"
        subprocess.run(["git", "init", "-q", "-b", "main", str(self.main)], check=True)
        (self.main / "scripts").mkdir()
        shutil.copy(SCRIPT, self.main / "scripts/port-worktree.sh")
        (self.main / "reg.md").write_text("| A |\n| B |\n")
        git(self.main, "add", "-A")
        git(self.main, "commit", "-q", "-m", "base")
        git(self.main, "worktree", "add", "-q", "-b", "blk", str(self.wt))
        # main moves on after the worktree started: a row added on main only.
        (self.main / "reg.md").write_text("| A |\n| B |\n| C main |\n")
        git(self.main, "commit", "-q", "-am", "main row")

    def port(self):
        return subprocess.run(["bash", "scripts/port-worktree.sh", str(self.wt), str(self.main.parent / "p.patch")], cwd=self.main,
                              capture_output=True, text=True, timeout=60)

    def test_main_commits_made_meanwhile_are_kept(self):
        (self.wt / "new.txt").write_text("block work\n")
        out = self.port()
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertIn("| C main |", (self.main / "reg.md").read_text())
        self.assertEqual((self.main / "new.txt").read_text(), "block work\n")

    def test_the_old_way_would_have_reverted_them(self):
        # The defect this script replaces: a diff from main's HEAD to the worktree's tree drops « C main ».
        (self.wt / "new.txt").write_text("block work\n")
        git(self.wt, "add", "-A")
        tree = git(self.wt, "write-tree").strip()
        old = git(self.main, "diff", "HEAD", tree)
        self.assertIn("-| C main |", old)

    def test_a_true_conflict_is_reported(self):
        (self.wt / "reg.md").write_text("| A |\n| B |\n| C block |\n")
        out = self.port()
        self.assertEqual(out.returncode, 1)
        self.assertIn("reg.md", out.stderr)

    def test_bad_arguments_are_refused(self):
        for args in ([], ["/nonexistent"], [str(self.main)]):
            r = subprocess.run(["bash", "scripts/port-worktree.sh", *args], cwd=self.main, capture_output=True,
                               text=True, timeout=60)
            self.assertEqual(r.returncode, 2, args)


if __name__ == "__main__":
    unittest.main()
