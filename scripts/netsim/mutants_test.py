"""Tests of the `expect` field of scripts/netsim/mutants.py (W58a). The Go runner is injected: no Go build runs here."""
import json
import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import mutants  # noqa: E402

M = {"id": "MX", "block": "W58a", "kind": "go", "pkg": "./internal/app/", "test": "TestReplaceFailureAtEachStep",
     "expect": "N-16 store usable after a failed replacement"}


def ev(action, test=None, output=None):
    e = {"Action": action, "Package": "p"}
    if test:
        e["Test"] = test
    if output is not None:
        e["Output"] = output
    return json.dumps(e)


def runner(rc, *lines):
    """A runner that records its command and returns rc and the given output lines."""
    calls = []

    def run(cmd, cwd, env=None, log=None, timeout=1800):
        calls.append(cmd)
        return rc, "\n".join(lines) + "\n"

    run.calls = calls
    return run


class ExpectTest(unittest.TestCase):
    def verdict(self, rc, *lines, m=M):
        r = runner(rc, *lines)
        caught, _, _ = mutants.run_go(m, pathlib.Path("."), run=r)
        return caught, r

    def test_caught(self):
        caught, r = self.verdict(
            1, ev("run", M["test"]), ev("output", M["test"], "    x_test.go:9: N-16 store usable after a failed replacement: 0 folders\n"),
            ev("fail", M["test"]), ev("fail"))
        self.assertTrue(caught)
        self.assertIn("-json", r.calls[0])
        self.assertIn("^TestReplaceFailureAtEachStep$", r.calls[0])

    def test_another_test_fails(self):
        caught, _ = self.verdict(
            1, ev("output", "TestOther", "N-16 store usable after a failed replacement\n"), ev("fail", "TestOther"), ev("pass", M["test"]))
        self.assertFalse(caught)

    def test_right_test_other_message(self):
        caught, _ = self.verdict(
            1, ev("output", M["test"], "x_test.go:9: N-6 folder unchanged\n"), ev("fail", M["test"]))
        self.assertFalse(caught)

    def test_collision_name_in_one_test_text_in_another(self):
        # The named test fails with another text, and a different test prints the expected text.
        caught, _ = self.verdict(
            1, ev("output", M["test"], "boom\n"), ev("fail", M["test"]),
            ev("output", "TestOther", "N-16 store usable after a failed replacement\n"), ev("fail", "TestOther"))
        self.assertFalse(caught)
        # The text is in the named test, but only another test fails.
        caught, _ = self.verdict(
            1, ev("output", M["test"], "N-16 store usable after a failed replacement\n"), ev("pass", M["test"]), ev("fail", "TestOther"))
        self.assertFalse(caught)

    def test_name_prefix_is_not_the_name(self):
        longer = M["test"] + "Again"
        caught, _ = self.verdict(
            1, ev("output", longer, "N-16 store usable after a failed replacement\n"), ev("fail", longer))
        self.assertFalse(caught)

    def test_subtest_output_counts_for_the_named_test(self):
        caught, _ = self.verdict(
            1, ev("output", M["test"] + "/a", "N-16 store usable after a failed replacement\n"), ev("fail", M["test"] + "/a"),
            ev("fail", M["test"]))
        self.assertTrue(caught)

    def test_build_failure_is_not_a_catch(self):
        caught, _ = self.verdict(
            1, ev("output", None, "# p\nx.go:1: undefined: y N-16 store usable after a failed replacement\n"), ev("fail"), "FAIL p [build failed]")
        self.assertFalse(caught)

    def test_timeout_is_not_a_catch(self):
        caught, _ = self.verdict(
            124, ev("output", M["test"], "N-16 store usable after a failed replacement\n"), ev("fail", M["test"]))
        self.assertFalse(caught)

    def test_passing_test_is_not_a_catch(self):
        caught, _ = self.verdict(0, ev("output", M["test"], "N-16 store usable after a failed replacement\n"), ev("pass", M["test"]))
        self.assertFalse(caught)

    def test_entry_without_expect_keeps_the_old_rule(self):
        m = {k: v for k, v in M.items() if k != "expect"}
        r = runner(1, "--- FAIL: TestReplaceFailureAtEachStep (0.00s)", "FAIL")
        caught, _, _ = mutants.run_go(m, pathlib.Path("."), run=r)
        self.assertTrue(caught)
        self.assertNotIn("-json", r.calls[0])
        r = runner(1, "--- FAIL: TestX (0.00s)", "FAIL p [build failed]")
        self.assertFalse(mutants.run_go(m, pathlib.Path("."), run=r)[0])

    def test_validate_refuses_an_empty_or_misplaced_expect(self):
        self.assertEqual(mutants.validate([M]), [])
        self.assertTrue(mutants.validate([{**M, "expect": ""}]))
        self.assertTrue(mutants.validate([{**M, "kind": "vitest"}]))


if __name__ == "__main__":
    unittest.main()
