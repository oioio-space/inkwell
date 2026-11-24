"""The pre-push hook runs the CI for any push that sends code, and nothing for a push that only deletes refs.

The hook is run for real, with a PATH that has no `mise`: where it would start the CI it stops at its
« mise not found » line instead, so the test never launches the 30 min CI.
"""

import os
import shutil
import subprocess
import tempfile
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOK = os.path.join(ROOT, ".githooks", "pre-push")
ZERO = "0" * 40
SHA = "1" * 40
NO_CI = "nothing to check"
WOULD_RUN_CI = "mise not found"


class PrePushHook(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="inkwell-prepush-")
        self.addCleanup(tmp.cleanup)
        # Only the tools the hook needs before its CI step: no mise, so a run stops at « mise not found ».
        self.bin = os.path.join(tmp.name, "bin")
        os.mkdir(self.bin)
        for tool in ("git", "awk", "cat", "python3"):
            os.symlink(shutil.which(tool), os.path.join(self.bin, tool))

    def run_hook(self, stdin):
        env = {"PATH": self.bin, "HOME": os.environ.get("HOME", "/")}
        out = subprocess.run(
            ["/bin/bash", HOOK, "origin", "https://example.invalid/repo.git"],
            input=stdin, text=True, capture_output=True, cwd=ROOT, env=env, timeout=60,
        )
        return out.returncode, out.stdout + out.stderr

    def test_deleting_refs_only_runs_nothing(self):
        lines = f"(delete) {ZERO} refs/heads/candidate/a {SHA}\n(delete) {ZERO} refs/heads/candidate/b {SHA}\n"
        code, out = self.run_hook(lines)
        self.assertEqual(code, 0, out)
        self.assertIn(NO_CI, out)
        self.assertNotIn(WOULD_RUN_CI, out)

    def test_a_push_with_code_reaches_the_ci(self):
        lines = f"refs/heads/main {SHA} refs/heads/main {ZERO}\n"
        _, out = self.run_hook(lines)
        self.assertIn(WOULD_RUN_CI, out)
        self.assertNotIn(NO_CI, out)

    def test_a_deletion_beside_a_code_push_reaches_the_ci(self):
        lines = f"(delete) {ZERO} refs/heads/old {SHA}\nrefs/heads/main {SHA} refs/heads/main {ZERO}\n"
        _, out = self.run_hook(lines)
        self.assertIn(WOULD_RUN_CI, out)

    def test_empty_input_reaches_the_ci(self):
        _, out = self.run_hook("")
        self.assertIn(WOULD_RUN_CI, out)


if __name__ == "__main__":
    unittest.main()
