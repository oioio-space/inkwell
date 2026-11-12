"""The formal model of block RG (plan section 6): error rates of the layout-robust rule and its mutants.

Skeleton frozen with gates/RG.toml (criterion `model`): every test fails until block RG step 2
implements it (rgmodel.py, plan sections 4-6; docs/plans/2026-11-12-rg-layout-robust-guard.md).
"""
import unittest


class Rgmodel(unittest.TestCase):
    def test_noise_refits_from_archive(self):
        self.fail("not implemented: RG step 2, test_noise_refits_from_archive")

    def test_incumbent_flags_pure_layout(self):
        self.fail("not implemented: RG step 2, test_incumbent_flags_pure_layout")

    def test_no_false_regression_under_layout(self):
        self.fail("not implemented: RG step 2, test_no_false_regression_under_layout")

    def test_no_false_regression_under_drift(self):
        self.fail("not implemented: RG step 2, test_no_false_regression_under_drift")

    def test_flag_rate_at_margin_bounded(self):
        self.fail("not implemented: RG step 2, test_flag_rate_at_margin_bounded")

    def test_power_at_five_percent(self):
        self.fail("not implemented: RG step 2, test_power_at_five_percent")

    def test_true_slowdown_never_certified(self):
        self.fail("not implemented: RG step 2, test_true_slowdown_never_certified")

    def test_loaded_host_is_inconclusive_not_certified(self):
        self.fail("not implemented: RG step 2, test_loaded_host_is_inconclusive_not_certified")

    def test_production_arm_gives_investigate(self):
        self.fail("not implemented: RG step 2, test_production_arm_gives_investigate")

    def test_unequal_shapes_never_flag_at_no_change(self):
        self.fail("not implemented: RG step 2, test_unequal_shapes_never_flag_at_no_change")

    def test_margin_level_under_unequal_shapes(self):
        self.fail("not implemented: RG step 2, test_margin_level_under_unequal_shapes")

    def test_certification_spends_alpha_over_looks(self):
        self.fail("not implemented: RG step 2, test_certification_spends_alpha_over_looks")

    def test_power_sized_at_max_comparisons(self):
        self.fail("not implemented: RG step 2, test_power_sized_at_max_comparisons")

    def test_rates_asserted_on_clopper_pearson_bounds(self):
        self.fail("not implemented: RG step 2, test_rates_asserted_on_clopper_pearson_bounds")

    def test_power_not_below_incumbent(self):
        self.fail("not implemented: RG step 2, test_power_not_below_incumbent")

    def test_every_mutant_is_rejected(self):
        self.fail("not implemented: RG step 2, test_every_mutant_is_rejected")

    def test_constants_match_code(self):
        self.fail("not implemented: RG step 2, test_constants_match_code")


if __name__ == '__main__':
    unittest.main()
