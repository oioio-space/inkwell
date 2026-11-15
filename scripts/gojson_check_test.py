#!/usr/bin/env python3
"""Tests for scripts/gojson_check.py (block SB2a1, decision D7): a `go test -json` run against the mandatory inventory.

Each test replays a small recorded run and names the rule it proves. The runs are built from events, never from a real
`go test`, so the suite needs no toolchain."""
import io
import json
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import gojson_check  # noqa: E402

PKG = "github.com/oioio-space/inkwell/internal/sidecar"
REPORT_UNAVAILABLE = "tier-report: tier=F n=unavailable errno=EACCES"
REPORT_AVAILABLE = "tier-report: tier=N n=available"
REFUSAL = "preview.fail.sandbox: la mise en page ne peut pas démarrer"


def run(*tests):
    """A go test -json stream: tests is a list of (name, action, output lines)."""
    events = []
    for name, action, lines in tests:
        events.append({"Action": "run", "Package": PKG, "Test": name})
        for line in lines:
            events.append({"Action": "output", "Package": PKG, "Test": name, "Output": line + "\n"})
        events.append({"Action": action, "Package": PKG, "Test": name})
    events.append({"Action": "pass", "Package": PKG})
    return "\n".join(json.dumps(e) for e in events) + "\n"


def inventory(*rows, columns=("local-N", "local-F", "stock-amd64", "arm64")):
    """An inventory text: rows are (name, {column: cell})."""
    out = ["columns = " + json.dumps(list(columns)), ""]
    for name, cells in rows:
        out += ["[[test]]", f'name = "{name}"', 'rule = "X-1"']
        out += [f'{c} = "{cells.get(c, "-")}"' for c in columns]
        out.append("")
    return "\n".join(out)


def check(stream, inv, columns, rc=0):
    with tempfile.TemporaryDirectory() as d:
        (Path(d) / "inv.toml").write_text(inv)
        return gojson_check.check_stream(
            stream.splitlines(), gojson_check.load_inventory(Path(d) / "inv.toml"), columns, rc
        )


class ColumnTest(unittest.TestCase):
    def test_the_true_record_is_accepted(self):
        inv = inventory(("TestA", {"local-N": "pass", "local-F": "-"}), ("TestA/F", {"local-F": "pass"}))
        s = run(("TestA", "pass", []), ("TestA/F", "pass", []))
        self.assertEqual(check(s, inv, ["local-N", "local-F"]), [])

    def test_a_missing_required_test_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}), ("TestB", {"local-N": "pass"}))
        problems = check(run(("TestA", "pass", [])), inv, ["local-N"])
        self.assertEqual(len(problems), 1)
        self.assertIn("TestB", problems[0])
        self.assertIn("missing", problems[0])

    def test_a_missing_subtest_fails_though_its_parent_passed(self):
        inv = inventory(("TestA/x", {"local-N": "pass"}))
        problems = check(run(("TestA", "pass", [])), inv, ["local-N"])
        self.assertTrue(any("TestA/x" in p and "missing" in p for p in problems))

    def test_a_failure_fails_even_outside_the_inventory(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        problems = check(run(("TestA", "pass", []), ("TestOther", "fail", ["boom"])), inv, ["local-N"])
        self.assertTrue(any("TestOther" in p and "fail" in p for p in problems))

    def test_a_failed_subtest_fails_a_required_row(self):
        inv = inventory(("TestA/x", {"local-N": "pass"}))
        problems = check(run(("TestA/x", "fail", [])), inv, ["local-N"])
        self.assertTrue(any("TestA/x" in p for p in problems))

    def test_a_nonzero_exit_status_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        problems = check(run(("TestA", "pass", [])), inv, ["local-N"], rc=2)
        self.assertTrue(any("exit status 2" in p for p in problems))

    def test_a_package_level_failure_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = run(("TestA", "pass", [])) + json.dumps({"Action": "fail", "Package": PKG}) + "\n"
        self.assertTrue(any("package" in p for p in check(s, inv, ["local-N"])))

    def test_a_test_that_never_finished_is_not_a_pass(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = json.dumps({"Action": "run", "Package": PKG, "Test": "TestA"}) + "\n"
        self.assertTrue(any("TestA" in p for p in check(s, inv, ["local-N"])))

    def test_a_skip_where_pass_is_required_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        problems = check(run(("TestA", "skip", [REPORT_UNAVAILABLE])), inv, ["local-N"])
        self.assertTrue(any("TestA" in p and "skip" in p for p in problems))

    def test_no_skip_in_tier_f_even_where_the_cell_would_allow_n_unavailable(self):
        inv = inventory(("TestA/F/x", {"stock-amd64": "pass|n-unavailable"}))
        problems = check(run(("TestA/F/x", "skip", [REPORT_UNAVAILABLE])), inv, ["stock-amd64"])
        self.assertTrue(any("TestA/F/x" in p and "tier F" in p for p in problems))

    def test_a_skipped_f_test_outside_the_inventory_fails_too(self):
        inv = inventory(("TestA", {"local-F": "pass"}))
        problems = check(run(("TestA", "pass", []), ("TestEscape/F-abi3/x", "skip", [])), inv, ["local-F"])
        self.assertTrue(any("TestEscape/F-abi3/x" in p for p in problems))

    def test_n_unavailable_is_accepted_with_its_tier_report(self):
        inv = inventory(("TestA/N/cgroup", {"stock-amd64": "pass|n-unavailable"}))
        s = run(("TestA/N/cgroup", "skip", ["    x_test.go:9: " + REPORT_UNAVAILABLE]))
        self.assertEqual(check(s, inv, ["stock-amd64"]), [])

    def test_n_unavailable_with_the_report_on_the_parent_is_accepted(self):
        inv = inventory(("TestA/N/cgroup", {"stock-amd64": "pass|n-unavailable"}))
        s = run(("TestA", "pass", [REPORT_UNAVAILABLE]), ("TestA/N/cgroup", "skip", []))
        self.assertEqual(check(s, inv, ["stock-amd64"]), [])

    def test_n_unavailable_without_its_tier_report_fails(self):
        inv = inventory(("TestA/N/cgroup", {"stock-amd64": "pass|n-unavailable"}))
        problems = check(run(("TestA/N/cgroup", "skip", ["skipped for no reason"])), inv, ["stock-amd64"])
        self.assertTrue(any("tier report" in p for p in problems))

    def test_n_unavailable_with_a_report_saying_n_is_available_fails(self):
        inv = inventory(("TestA/N/cgroup", {"stock-amd64": "pass|n-unavailable"}))
        problems = check(run(("TestA/N/cgroup", "skip", [REPORT_AVAILABLE])), inv, ["stock-amd64"])
        self.assertTrue(any("tier report" in p for p in problems))

    def test_n_unavailable_in_an_f_column_fails(self):
        inv = inventory(("TestA/x", {"local-F": "pass|n-unavailable"}))
        problems = check(run(("TestA/x", "skip", [REPORT_UNAVAILABLE])), inv, ["local-F"])
        self.assertTrue(any("F column" in p for p in problems))

    def test_pass_is_accepted_where_n_unavailable_is_allowed(self):
        inv = inventory(("TestA/N/cgroup", {"stock-amd64": "pass|n-unavailable"}))
        self.assertEqual(check(run(("TestA/N/cgroup", "pass", [])), inv, ["stock-amd64"]), [])

    def test_refused_needs_the_refusal_sentence(self):
        inv = inventory(("TestA", {"arm64": "refused"}))
        self.assertEqual(check(run(("TestA", "pass", [REFUSAL])), inv, ["arm64"]), [])
        problems = check(run(("TestA", "pass", ["ok"])), inv, ["arm64"])
        self.assertTrue(any("refus" in p for p in problems))

    def test_refused_where_the_test_failed_fails(self):
        inv = inventory(("TestA", {"arm64": "refused"}))
        self.assertTrue(check(run(("TestA", "fail", [REFUSAL])), inv, ["arm64"]))

    def test_a_dash_cell_requires_nothing_but_must_pass_if_it_runs(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        self.assertEqual(check(run(("TestA", "pass", [])), inv, ["local-F"]), [])
        problems = check(run(("TestA", "fail", [])), inv, ["local-F"])
        self.assertTrue(problems)

    def test_an_unknown_status_fails(self):
        inv = inventory(("TestA", {"local-N": "maybe"}))
        self.assertTrue(any("maybe" in p for p in check(run(("TestA", "pass", [])), inv, ["local-N"])))

    def test_an_unknown_column_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        with self.assertRaises(gojson_check.CheckError):
            check(run(("TestA", "pass", [])), inv, ["nowhere"])

    def test_a_malformed_line_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = run(("TestA", "pass", [])) + "not json\n"
        self.assertTrue(any("not JSON" in p for p in check(s, inv, ["local-N"])))

    def test_the_same_test_in_two_packages_fails_if_either_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = run(("TestA", "pass", [])) + json.dumps({"Action": "run", "Package": "other", "Test": "TestA"}) + "\n"
        s += json.dumps({"Action": "fail", "Package": "other", "Test": "TestA"}) + "\n"
        self.assertTrue(check(s, inv, ["local-N"]))

    def test_a_stream_truncated_before_its_package_ends_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = run(("TestA", "pass", []))
        truncated = s[: s.rindex(json.dumps({"Action": "pass", "Package": PKG}))]
        problems = check(truncated, inv, ["local-N"])
        self.assertTrue(any(PKG in p and "did not finish" in p for p in problems), problems)

    def test_a_package_without_test_files_may_end_in_skip(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = run(("TestA", "pass", [])) + json.dumps({"Action": "skip", "Package": PKG + "/bin"}) + "\n"
        self.assertEqual(check(s, inv, ["local-N"]), [])

    def test_a_package_with_tests_that_ends_in_skip_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        s = run(("TestA", "pass", [])).replace(
            json.dumps({"Action": "pass", "Package": PKG}), json.dumps({"Action": "skip", "Package": PKG})
        )
        self.assertTrue(any(PKG in p and "skip" in p for p in check(s, inv, ["local-N"])))

    def test_an_empty_run_fails(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        self.assertTrue(any("no test" in p for p in check("", inv, ["local-N"])))


class MainTest(unittest.TestCase):
    def invoke(self, stream, inv, columns, rc):
        with tempfile.TemporaryDirectory() as d:
            (Path(d) / "run.json").write_text(stream)
            (Path(d) / "inv.toml").write_text(inv)
            out, err = io.StringIO(), io.StringIO()
            with redirect_stdout(out), redirect_stderr(err):
                code = gojson_check.main(
                    [str(Path(d) / "run.json"), "--exit", str(rc), "--inventory", str(Path(d) / "inv.toml"), "--columns", columns]
                )
            return code, out.getvalue() + err.getvalue()

    def test_exit_codes_and_messages(self):
        inv = inventory(("TestA", {"local-N": "pass"}))
        code, _ = self.invoke(run(("TestA", "pass", [])), inv, "local-N", 0)
        self.assertEqual(code, 0)
        code, msg = self.invoke(run(("TestA", "pass", [])), inv, "local-N", 1)
        self.assertEqual(code, 1)
        self.assertIn("exit status 1", msg)

    def test_a_missing_file_is_an_error(self):
        with redirect_stderr(io.StringIO()):
            code = gojson_check.main(
                ["/nonexistent/run.json", "--inventory", "/nonexistent/inv.toml", "--columns", "local-N"]
            )
        self.assertNotEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
