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

    def test_walkthrough_captures_are_kept(self):
        # The interface walkthrough (step 4, before the gate) stores its captures in gates/<lot>/ux/; the
        # gate did not produce them and must not erase them (58 captures of W58a lost on 2026-11-14).
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "W58a"
            (out / "ux" / "sub").mkdir(parents=True)
            (out / "ux" / "banner-light.png").write_bytes(b"png")
            (out / "ux" / "sub" / "dialog-dark.png").write_bytes(b"png")
            (out / "e2e.log").write_text("old evidence")
            gate.reset_evidence(out)
            self.assertEqual(sorted(p.name for p in out.iterdir()), ["ux"])
            self.assertTrue((out / "ux" / "sub" / "dialog-dark.png").exists())

    def test_github_run_input_is_kept(self):
        # gates/<lot>/github.json names the GitHub run a `github` criterion checks (scripts/gh-verdict.py,
        # gates/SB2a1.toml): it is the gate's input, written before the run, not its evidence (2026-11-17: the
        # reset erased it and the criterion could never pass).
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "SB2a1"
            out.mkdir()
            (out / "github.json").write_text('{"run_id": 1}')
            (out / "github.log").write_text("old evidence")
            gate.reset_evidence(out)
            self.assertEqual(sorted(p.name for p in out.iterdir()), ["github.json"])
            self.assertEqual((out / "github.json").read_text(), '{"run_id": 1}')

    def test_a_ux_symlink_is_still_removed(self):
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "L"
            out.mkdir()
            elsewhere = pathlib.Path(d) / "elsewhere"
            elsewhere.mkdir()
            (elsewhere / "keep.png").write_bytes(b"png")
            (out / "ux").symlink_to(elsewhere, target_is_directory=True)
            gate.reset_evidence(out)
            self.assertEqual(list(out.iterdir()), [])
            self.assertTrue((elsewhere / "keep.png").exists(), "a symlink must not be followed")

    def test_missing_folder_is_created(self):
        with tempfile.TemporaryDirectory() as d:
            out = pathlib.Path(d) / "new"
            gate.reset_evidence(out)
            self.assertTrue(out.is_dir())


if __name__ == "__main__":
    unittest.main()
