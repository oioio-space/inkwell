"""Guards for scripts/block_done.py, which enforces the definition of done of every block."""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import block_done  # noqa: E402


class CodeFilter(unittest.TestCase):
    def test_docs_gates_and_markdown_are_not_code(self):
        files = ["docs/a.md", "gates/P3.toml", "gates/P3/SIMPLIFY.md", "README.md",
                 "protocol/repo/cbor.go", "frontend/src/net/admission.ts", "scripts/gate.py"]
        self.assertEqual(block_done.code(files),
                         ["protocol/repo/cbor.go", "frontend/src/net/admission.ts", "scripts/gate.py"])


class Efficacy(unittest.TestCase):
    def test_reads_gremlins_efficacy(self):
        self.assertEqual(block_done.parse_efficacy("Killed: 79\nTest efficacy: 74.53%\n"), 74.53)

    def test_missing_report_counts_as_zero_so_the_block_fails(self):
        self.assertEqual(block_done.parse_efficacy("panic: build failed"), 0.0)
        self.assertLess(block_done.parse_efficacy(""), block_done.MIN_EFFICACY)

    def test_all_timed_out_is_a_void_measurement(self):
        # 2026-11-08: a cached coverage run made every mutant time out, read as 0 % "justified".
        out = "Killed: 0, Lived: 0, Not covered: 6\nTimed out: 278\nTest efficacy: 0.00%\n"
        self.assertIn("no mutant", block_done.void_measurement(out))
        self.assertIn("no mutant", block_done.void_measurement("panic: build failed"))
        self.assertEqual(block_done.void_measurement("Killed: 95, Lived: 2, Not covered: 0\nTimed out: 0"), "")

    def test_many_timeouts_void_the_measurement(self):
        self.assertIn("20 of 100", block_done.void_measurement("Killed: 75, Lived: 5\nTimed out: 20"))
        self.assertEqual(block_done.void_measurement("Killed: 85, Lived: 5\nTimed out: 10"), "")

    def test_mutation_step_runs_without_protocol_packages(self):
        # 2026-11-08: a local named `code` shadowed code() and crashed step 6 at its first line.
        block_done.step6_mutation("P3", ["docs/a.md", "frontend/src/x.ts", "protocol/repo/x_test.go"])

    def test_threshold_is_the_rule_of_claude_md(self):
        self.assertEqual(block_done.MIN_EFFICACY, 95.0)


class PreflightTest(unittest.TestCase):
    def test_a_failing_base_guard_stops_done_before_the_long_steps(self):
        import types
        calls = []
        def fake_sh(cmd, **kw):
            calls.append(cmd)
            return types.SimpleNamespace(returncode=1 if "--base-guard" in cmd else 0)
        orig = block_done.sh
        block_done.sh = fake_sh
        try:
            with self.assertRaises(SystemExit) as e:
                block_done.preflight("ANY")
            self.assertIn("base guard failed", str(e.exception))
            self.assertEqual(len(calls), 1)
        finally:
            block_done.sh = orig

    def test_a_mutant_whose_anchor_moved_stops_done_before_the_long_steps(self):
        import types
        calls = []
        def fake_sh(cmd, **kw):
            calls.append(cmd)
            return types.SimpleNamespace(returncode=1 if "mutants.py --check" in cmd else 0)
        orig = block_done.sh
        block_done.sh = fake_sh
        try:
            with self.assertRaises(SystemExit) as e:
                block_done.preflight("NI0")  # a block with netsim mutants on main
            self.assertIn("no longer applies", str(e.exception))
            self.assertTrue(any("mutants.py --check NI0" in c for c in calls))
        finally:
            block_done.sh = orig

    def test_preflight_runs_right_after_step_2(self):
        src = (block_done.ROOT / "scripts/block_done.py").read_text()
        main = src[src.index("def main():"):]
        self.assertLess(main.index("preflight(a.block)"), main.index('must("mise run lint'))


if __name__ == "__main__":
    unittest.main()
