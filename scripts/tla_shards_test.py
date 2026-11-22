"""Exercise the TLA+ dispatcher with every shard configured in GitHub CI."""

from collections import Counter
import json
import os
from pathlib import Path
import re
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[1]


class TlaShardsTest(unittest.TestCase):
    def list_runs(self, shard=""):
        env = os.environ.copy()
        env.pop("INKWELL_TLA_ONLY", None)
        env["TLA_SHARD"] = shard
        return subprocess.check_output(
            ["bash", "scripts/tla-check.sh", "--list"], cwd=ROOT, env=env, text=True, stderr=subprocess.PIPE
        ).splitlines()

    def test_matrix_runs_every_case_exactly_once(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        matrix = re.search(r"^        shard: (\[[^\n]+\])$", workflow, re.MULTILINE)
        self.assertIsNotNone(matrix)
        shards = json.loads(matrix[1])
        self.assertEqual(shards.count("alloy"), 1)
        shards.remove("alloy")
        full = self.list_runs()
        self.assertTrue(full)
        self.assertEqual(len(full), len(set(full)), "duplicate TLA+ run names")
        assigned = Counter()
        for shard in shards:
            runs = self.list_runs(shard)
            self.assertTrue(runs, f"empty TLA+ shard {shard}")
            assigned.update(runs)
        self.assertEqual(assigned, Counter(full))

    def test_invalid_shard_fails(self):
        with self.assertRaises(subprocess.CalledProcessError) as error:
            self.list_runs("4/3")
        self.assertEqual(error.exception.returncode, 1)
        self.assertIn("invalid TLA_SHARD", error.exception.stderr)
