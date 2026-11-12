"""The rule of the layout-robust Go guard as regression.py runs it, no process started.

Skeleton frozen with gates/RG.toml (criterion `rule`): every test fails until block RG step 2
implements it (regression.py, plan sections 4 and 9; docs/plans/2026-11-12-rg-layout-robust-guard.md).
"""
import unittest


class Regression(unittest.TestCase):
    def test_bound_is_shared_with_paired(self):
        self.fail("not implemented: RG step 2, test_bound_is_shared_with_paired")

    def test_regression_needs_lower_bound_above_margin(self):
        self.fail("not implemented: RG step 2, test_regression_needs_lower_bound_above_margin")

    def test_certified_needs_upper_bound_below_budget(self):
        self.fail("not implemented: RG step 2, test_certified_needs_upper_bound_below_budget")

    def test_inconclusive_fails_never_passes(self):
        self.fail("not implemented: RG step 2, test_inconclusive_fails_never_passes")

    def test_alpha_from_comparison_count(self):
        self.fail("not implemented: RG step 2, test_alpha_from_comparison_count")

    def test_stage_two_runs_only_uncertified(self):
        self.fail("not implemented: RG step 2, test_stage_two_runs_only_uncertified")

    def test_regression_judged_once_at_final_look(self):
        self.fail("not implemented: RG step 2, test_regression_judged_once_at_final_look")

    def test_production_arm_envelope(self):
        self.fail("not implemented: RG step 2, test_production_arm_envelope")

    def test_informational_packages_skipped(self):
        self.fail("not implemented: RG step 2, test_informational_packages_skipped")

    def test_budgets_follow_predeclared_rule(self):
        self.fail("not implemented: RG step 2, test_budgets_follow_predeclared_rule")

    def test_throughput_is_inverse_time(self):
        self.fail("not implemented: RG step 2, test_throughput_is_inverse_time")

    def test_allocs_rule_unchanged(self):
        self.fail("not implemented: RG step 2, test_allocs_rule_unchanged")

    def test_missing_benchmark_fails_closed(self):
        self.fail("not implemented: RG step 2, test_missing_benchmark_fails_closed")

    def test_new_benchmark_listed(self):
        self.fail("not implemented: RG step 2, test_new_benchmark_listed")

    def test_parse_bench_lines(self):
        self.fail("not implemented: RG step 2, test_parse_bench_lines")

    def test_layout_plan_replays_from_seed(self):
        self.fail("not implemented: RG step 2, test_layout_plan_replays_from_seed")

    def test_sides_draw_independent_layouts(self):
        self.fail("not implemented: RG step 2, test_sides_draw_independent_layouts")

    def test_orders_are_fair_coins_per_pair(self):
        self.fail("not implemented: RG step 2, test_orders_are_fair_coins_per_pair")

    def test_session_report_records_seed_layouts_and_bounds(self):
        self.fail("not implemented: RG step 2, test_session_report_records_seed_layouts_and_bounds")

    def test_certification_is_sign_test(self):
        self.fail("not implemented: RG step 2, test_certification_is_sign_test")

    def test_comparison_budget_exceeded_fails_closed(self):
        self.fail("not implemented: RG step 2, test_comparison_budget_exceeded_fails_closed")

    def test_production_vector_is_release_vector(self):
        self.fail("not implemented: RG step 2, test_production_vector_is_release_vector")

    def test_robust_vector_differs_only_by_funcalign_and_overlay(self):
        self.fail("not implemented: RG step 2, test_robust_vector_differs_only_by_funcalign_and_overlay")

    def test_pinned_toolchain_environment(self):
        self.fail("not implemented: RG step 2, test_pinned_toolchain_environment")

    def test_investigate_fails_and_needs_waiver(self):
        self.fail("not implemented: RG step 2, test_investigate_fails_and_needs_waiver")


if __name__ == '__main__':
    unittest.main()
