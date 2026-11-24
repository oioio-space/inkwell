"""scripts/candidate.sh: one candidate branch per GitHub run, all pushed in one push (2026-11-24: five runs dispatched on
one branch cancelled each other through ci.yml's per-ref concurrency)."""

import os
import pathlib
import shutil
import stat
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "candidate.sh"

FAKE_GH = """#!/usr/bin/env bash
# Records dispatches; `run list` answers the number of dispatches made so far on the branch (0 = none).
log="$FAKE_GH_LOG"
if [[ $1 == workflow && $2 == run ]]; then echo "dispatch ${@: -1}" >> "$log"; exit 0; fi
if [[ $1 == run && $2 == list ]]; then
  b=""; while [[ $# -gt 0 ]]; do [[ $1 == --branch ]] && b=$2; shift; done
  n=$(grep -c "dispatch $b\\$" "$log" 2>/dev/null || true)
  [[ $n -gt 0 ]] && echo $((1000 + n)) || echo 0
  exit 0
fi
echo "unexpected gh $*" >&2; exit 9
"""

FAKE_PUSH = """#!/usr/bin/env bash
echo "push $*" >> "$FAKE_GH_LOG"
exec git push -q "$@"
"""


class Candidate(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="inkwell-candidate-")
        self.addCleanup(tmp.cleanup)
        t = pathlib.Path(tmp.name)
        self.repo, origin, bin_ = t / "repo", t / "origin.git", t / "bin"
        bin_.mkdir()
        subprocess.run(["git", "init", "-q", "--bare", str(origin)], check=True)
        subprocess.run(["git", "init", "-q", "-b", "main", str(self.repo)], check=True)
        (self.repo / ".github/workflows").mkdir(parents=True)
        (self.repo / "scripts").mkdir()
        shutil.copy(SCRIPT, self.repo / "scripts/candidate.sh")
        (self.repo / ".github/workflows/ci.yml").write_text(
            "concurrency:\n  group: ${{ github.workflow }}-${{ github.ref }}\n  cancel-in-progress: true\n")
        git = ["git", "-C", str(self.repo), "-c", "user.name=t", "-c", "user.email=t@example.invalid"]
        subprocess.run(git + ["add", "-A"], check=True)
        subprocess.run(git + ["commit", "-q", "-m", "c"], check=True)
        subprocess.run(git + ["remote", "add", "origin", str(origin)], check=True)
        for name, body in (("gh", FAKE_GH), ("push", FAKE_PUSH)):
            p = bin_ / name
            p.write_text(body)
            p.chmod(p.stat().st_mode | stat.S_IEXEC)
        self.log = t / "gh.log"
        self.env = dict(os.environ, PATH=f"{bin_}:{os.environ['PATH']}", FAKE_GH_LOG=str(self.log),
                        CANDIDATE_PUSH=str(bin_ / "push"))

    def run_script(self, *args):
        return subprocess.run(["bash", "scripts/candidate.sh", *args], cwd=self.repo, env=self.env,
                              capture_output=True, text=True, timeout=60)

    def test_several_runs_get_one_branch_each_and_one_push(self):
        out = self.run_script("x", "HEAD", "3")
        self.assertEqual(out.returncode, 0, out.stderr)
        log = self.log.read_text().splitlines()
        self.assertEqual(sum(line.startswith("push ") for line in log), 1, log)
        self.assertEqual([line for line in log if line.startswith("dispatch")],
                         [f"dispatch candidate/x-{i}" for i in (1, 2, 3)])
        self.assertEqual(out.stdout.splitlines(), ["run: 1001", "run: 1001", "run: 1001"])
        heads = subprocess.run(["git", "-C", str(self.repo), "ls-remote", "--heads", "origin"], capture_output=True,
                               text=True, check=True).stdout
        self.assertEqual(sorted(line.split("refs/heads/")[1] for line in heads.splitlines()),
                         ["candidate/x-1", "candidate/x-2", "candidate/x-3"])

    def test_one_run_uses_the_bare_name(self):
        out = self.run_script("y")
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertIn("dispatch candidate/y", self.log.read_text())

    def test_an_existing_branch_is_refused_before_any_push(self):
        self.assertEqual(self.run_script("z").returncode, 0)
        self.log.write_text("")
        out = self.run_script("z")
        self.assertEqual(out.returncode, 1)
        self.assertIn("already exists", out.stderr)
        self.assertNotIn("push", self.log.read_text())

    def test_bad_arguments_are_refused(self):
        for args in ((), ("a b",), ("x", "HEAD", "0"), ("x", "nope")):
            self.assertNotEqual(self.run_script(*args).returncode, 0, args)


if __name__ == "__main__":
    unittest.main()
