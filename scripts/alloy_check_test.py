"""Guards for scripts/alloy_check.py: a mutant must keep biting as the models evolve.

If a model is edited so that a mutant's anchor text disappears or appears twice, the mutant would no
longer mutate the rule it targets (or would mutate the wrong place); and a mutant naming an assertion
the model no longer checks would prove nothing. Both fail here, before `mise run formal` runs.
"""
import re
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import alloy_check  # noqa: E402


class MutantAnchors(unittest.TestCase):
    def test_each_anchor_appears_exactly_once_in_its_model(self):
        for model, assertion, before, _after in alloy_check.MUTANTS:
            with self.subTest(model=model, assertion=assertion):
                text = (alloy_check.MODELS / model).read_text()
                self.assertEqual(text.count(before), 1, f"anchor for {assertion} in {model}")

    def test_each_mutant_targets_an_assertion_the_model_checks(self):
        for model, assertion, _before, _after in alloy_check.MUTANTS:
            with self.subTest(model=model, assertion=assertion):
                text = (alloy_check.MODELS / model).read_text()
                self.assertRegex(text, rf"(?m)^check {re.escape(assertion)}\b")

    def test_each_mutant_changes_its_model(self):
        for model, assertion, before, after in alloy_check.MUTANTS:
            with self.subTest(model=model, assertion=assertion):
                self.assertNotEqual(before, after)


if __name__ == "__main__":
    unittest.main()
