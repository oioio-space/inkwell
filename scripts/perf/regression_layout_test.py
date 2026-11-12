"""The layout machinery on real builds of internal/transport/mux (no measurement).

Skeleton frozen with gates/RG.toml (criterion `layout-build`): every test fails until block RG step 2
implements it (regression.py, plan sections 3 and 9; docs/plans/2026-11-12-rg-layout-robust-guard.md).
"""
import unittest


class RegressionLayout(unittest.TestCase):
    def test_pad_slots_are_exact(self):
        self.fail("not implemented: RG step 2, test_pad_slots_are_exact")

    def test_pads_precede_package_code(self):
        self.fail("not implemented: RG step 2, test_pads_precede_package_code")

    def test_functions_are_cache_line_aligned(self):
        self.fail("not implemented: RG step 2, test_functions_are_cache_line_aligned")

    def test_hot_offsets_spread_over_page(self):
        self.fail("not implemented: RG step 2, test_hot_offsets_spread_over_page")

    def test_reordered_pads_fail_closed(self):
        self.fail("not implemented: RG step 2, test_reordered_pads_fail_closed")

    def test_pads_keep_compiler_decisions(self):
        self.fail("not implemented: RG step 2, test_pads_keep_compiler_decisions")

    def test_pads_keep_data_and_init(self):
        self.fail("not implemented: RG step 2, test_pads_keep_data_and_init")

    def test_keep_file_never_runs(self):
        self.fail("not implemented: RG step 2, test_keep_file_never_runs")

    def test_dependency_pad_moves_package(self):
        self.fail("not implemented: RG step 2, test_dependency_pad_moves_package")


if __name__ == '__main__':
    unittest.main()
