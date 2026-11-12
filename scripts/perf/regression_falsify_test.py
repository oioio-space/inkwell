"""End-to-end falsification of the guard harness: each injected fault fails closed (F1-F7).

Skeleton frozen with gates/RG.toml (criterion `falsify`): every test fails until block RG step 2
implements it (regression.py, plan section 6; docs/plans/2026-11-12-rg-layout-robust-guard.md).
"""
import unittest


class RegressionFalsify(unittest.TestCase):
    def test_falsify_reused_binary(self):
        self.fail("not implemented: RG step 2, test_falsify_reused_binary")

    def test_falsify_ignored_overlay(self):
        self.fail("not implemented: RG step 2, test_falsify_ignored_overlay")

    def test_falsify_altered_production_flags(self):
        self.fail("not implemented: RG step 2, test_falsify_altered_production_flags")

    def test_falsify_dropped_production_verdict(self):
        self.fail("not implemented: RG step 2, test_falsify_dropped_production_verdict")

    def test_falsify_inconclusive_as_pass(self):
        self.fail("not implemented: RG step 2, test_falsify_inconclusive_as_pass")

    def test_falsify_missing_funcalign(self):
        self.fail("not implemented: RG step 2, test_falsify_missing_funcalign")

    def test_falsify_shared_pad_vectors(self):
        self.fail("not implemented: RG step 2, test_falsify_shared_pad_vectors")

    def test_session_records_identities_and_offsets(self):
        self.fail("not implemented: RG step 2, test_session_records_identities_and_offsets")


if __name__ == '__main__':
    unittest.main()
