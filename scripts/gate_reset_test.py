#!/usr/bin/env python3
"""Guards for gate.reset_evidence: a gate run clears its evidence but never the block's reports."""
import importlib.util
import pathlib
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("gate", pathlib.Path(__file__).resolve().parent / "gate.py")
gate = importlib.util.module_from_spec(spec)
sys.modules["gate"] = gate
spec.loader.exec_module(gate)


class ResetEvidence(unittest.TestCase):
    def test_reports_are_kept_and_evidence_cleared(self):
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "P3"
            (out / "playwright-results").mkdir(parents=True)
            (out / "playwright-results" / "trace.zip").write_text("x")
            (out / "P-TLA.log").write_text("old evidence")
            (out / "SIMPLIFY.md").write_text("report")
            (out / "RECONSTRUCTED.md").write_text("report")
            outside = pathlib.Path(d) / "outside"
            outside.mkdir()
            (outside / "keep.txt").write_text("not ours")
            (out / "link").symlink_to(outside, target_is_directory=True)
            gate.reset_evidence(out)
            self.assertEqual(sorted(p.name for p in out.iterdir()), ["RECONSTRUCTED.md", "SIMPLIFY.md"])
            self.assertEqual((out / "SIMPLIFY.md").read_text(), "report")
            self.assertTrue((outside / "keep.txt").exists(), "a symlink must not be followed")

    def test_missing_folder_is_created(self):
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "new"
            gate.reset_evidence(out)
            self.assertTrue(out.is_dir())


if __name__ == "__main__":
    unittest.main()
