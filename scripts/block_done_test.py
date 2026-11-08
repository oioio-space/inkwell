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


if __name__ == "__main__":
    unittest.main()
