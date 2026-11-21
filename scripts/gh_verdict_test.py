#!/usr/bin/env python3
"""Tests for scripts/gh-verdict.py (block SB2a1, decision D6): the candidate's GitHub run, re-fetched and checked.

The GitHub API is replayed from fixtures (a dict of path -> bytes): no network, no `gh`. Each test starts from the true
record, changes one fact, and expects exactly that refusal."""
import copy
import importlib.util
import io
import json
import sys
import tempfile
import unittest
import zipfile
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("gh_verdict", HERE / "gh-verdict.py")
gh_verdict = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gh_verdict)

REPO = "oioio-space/inkwell"
SHA = "0123456789abcdef0123456789abcdef01234567"
RUN = 987654
PKG = "github.com/oioio-space/inkwell/internal/sidecar"
UNAVAILABLE = "tier-report: tier=F n=unavailable errno=EACCES"
REFUSAL = "preview.fail.sandbox: la mise en page ne peut pas démarrer"

INVENTORY = """
columns = ["local-N", "local-F", "stock-amd64", "arm64"]

[[test]]
name = "TestDecide"
rule = "T-1"
local-N = "pass"
local-F = "-"
stock-amd64 = "pass"
arm64 = "pass"

[[test]]
name = "TestEscape/N/exec-slot"
rule = "S-1"
local-N = "pass"
local-F = "-"
stock-amd64 = "pass|n-unavailable"
arm64 = "refused"

[[test]]
name = "TestEscape/F/exec-slot"
rule = "S-1"
local-N = "-"
local-F = "pass"
stock-amd64 = "pass"
arm64 = "refused"
"""


def go_json(*tests):
    ev = []
    for name, action, lines in tests:
        ev.append({"Action": "run", "Package": PKG, "Test": name})
        ev += [{"Action": "output", "Package": PKG, "Test": name, "Output": l + "\n"} for l in lines]
        ev.append({"Action": action, "Package": PKG, "Test": name})
    ev.append({"Action": "pass", "Package": PKG})
    return "\n".join(json.dumps(e) for e in ev) + "\n"


def zipped(text, member="go-test.json"):
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        z.writestr(member, text)
    return buf.getvalue()


STOCK = go_json(
    ("TestDecide", "pass", []),
    ("TestEscape/N/exec-slot", "skip", [UNAVAILABLE]),
    ("TestEscape/F/exec-slot", "pass", []),
)
ARM = go_json(
    ("TestDecide", "pass", []),
    ("TestEscape/N/exec-slot", "pass", [REFUSAL]),
    ("TestEscape/F/exec-slot", "pass", [REFUSAL]),
)

RUN_RECORD = {
    "id": RUN,
    "repository": {"full_name": REPO},
    "head_repository": {"full_name": REPO, "fork": False},
    "path": ".github/workflows/ci.yml",
    "event": "workflow_dispatch",
    "head_branch": "candidate/sb2a1-1",
    "head_sha": SHA,
    "status": "completed",
    "conclusion": "success",
}
JOBS = ["ci", "netsim", "sandbox-stock", "sidecars-arm64"]


class Fake:
    """A replayed API: every path the checker asks for must be here."""

    def __init__(self, run=None, jobs=None, artifacts=None, stock=None, arm=None):
        self.asked = []
        self.run = copy.deepcopy(run or RUN_RECORD)
        self.jobs = [{"name": n, "status": "completed", "conclusion": "success"} for n in JOBS] if jobs is None else jobs
        self.arts = (
            [
                {"id": 11, "name": "go-test-json-sandbox-stock", "expired": False, "workflow_run": {"id": RUN, "head_sha": SHA}},
                {"id": 12, "name": "go-test-json-sidecars-arm64", "expired": False, "workflow_run": {"id": RUN, "head_sha": SHA}},
            ]
            if artifacts is None
            else artifacts
        )
        self.zips = {11: zipped(STOCK if stock is None else stock), 12: zipped(ARM if arm is None else arm)}

    def __call__(self, path):
        self.asked.append(path)
        base = f"repos/{REPO}/actions"
        if path == f"{base}/runs/{RUN}":
            return json.dumps(self.run).encode()
        if path.startswith(f"{base}/runs/{RUN}/jobs"):
            return json.dumps({"jobs": self.jobs}).encode()
        if path.startswith(f"{base}/runs/{RUN}/artifacts"):
            return json.dumps({"artifacts": self.arts}).encode()
        for i, z in self.zips.items():
            if path == f"{base}/artifacts/{i}/zip":
                return z
        raise AssertionError(f"unexpected API path {path}")


def options(**over):
    o = dict(
        repo=REPO,
        workflow=".github/workflows/ci.yml",
        event="workflow_dispatch",
        branch_prefix="candidate/sb2a1-",
        head=SHA,
        clean=True,
        jobs=JOBS,
        columns=["stock-amd64", "arm64"],
    )
    o.update(over)
    return gh_verdict.Options(**o)


def verdict(api, opts=None, tree_clean=True):
    with tempfile.TemporaryDirectory() as d:
        inv = Path(d) / "inv.toml"
        inv.write_text(INVENTORY)
        return gh_verdict.verdict(RUN, opts or options(), api, lambda: tree_clean, inv)


class VerdictTest(unittest.TestCase):
    def refused(self, problems, *words):
        self.assertTrue(problems, "a bad record was accepted")
        text = " | ".join(problems)
        for w in words:
            self.assertIn(w, text)

    def test_the_true_record_is_accepted(self):
        self.assertEqual(verdict(Fake()), [])

    def test_another_repository_is_refused(self):
        run = copy.deepcopy(RUN_RECORD)
        run["repository"]["full_name"] = "someone/inkwell"
        self.refused(verdict(Fake(run=run)), "repository")

    def test_a_fork_is_refused(self):
        run = copy.deepcopy(RUN_RECORD)
        run["head_repository"] = {"full_name": "mallory/inkwell", "fork": True}
        self.refused(verdict(Fake(run=run)), "head repository")

    def test_another_workflow_path_is_refused(self):
        run = dict(RUN_RECORD, path=".github/workflows/evil.yml")
        self.refused(verdict(Fake(run=run)), "workflow")

    def test_a_workflow_path_with_its_ref_suffix_is_the_same_workflow(self):
        run = dict(RUN_RECORD, path=".github/workflows/ci.yml@refs/heads/candidate/sb2a1-1")
        self.assertEqual(verdict(Fake(run=run)), [])

    def test_another_event_is_refused(self):
        self.refused(verdict(Fake(run=dict(RUN_RECORD, event="push"))), "event")

    def test_another_branch_is_refused(self):
        self.refused(verdict(Fake(run=dict(RUN_RECORD, head_branch="main"))), "branch")

    def test_another_sha_is_refused(self):
        self.refused(verdict(Fake(run=dict(RUN_RECORD, head_sha="f" * 40))), "head sha")

    def test_a_dirty_tree_is_refused(self):
        self.refused(verdict(Fake(), tree_clean=False), "tracked tree")

    def test_a_dirty_tree_is_accepted_without_the_clean_option(self):
        self.assertEqual(verdict(Fake(), options(clean=False), tree_clean=False), [])

    def test_an_in_progress_run_is_refused(self):
        run = dict(RUN_RECORD, status="in_progress", conclusion=None)
        self.refused(verdict(Fake(run=run)), "status")

    def test_a_failed_run_is_refused(self):
        self.refused(verdict(Fake(run=dict(RUN_RECORD, conclusion="failure"))), "conclusion")

    def test_a_failed_job_is_refused(self):
        jobs = [{"name": n, "status": "completed", "conclusion": "failure" if n == "netsim" else "success"} for n in JOBS]
        self.refused(verdict(Fake(jobs=jobs)), "netsim", "conclusion")

    def test_a_skipped_job_is_not_a_success(self):
        jobs = [{"name": n, "status": "completed", "conclusion": "skipped" if n == "sandbox-stock" else "success"} for n in JOBS]
        self.refused(verdict(Fake(jobs=jobs)), "sandbox-stock")

    def test_a_missing_job_is_refused(self):
        jobs = [{"name": n, "status": "completed", "conclusion": "success"} for n in JOBS if n != "sidecars-arm64"]
        self.refused(verdict(Fake(jobs=jobs)), "sidecars-arm64", "missing")

    def test_a_missing_artefact_is_refused(self):
        fake = Fake(artifacts=[{"id": 12, "name": "go-test-json-sidecars-arm64", "expired": False, "workflow_run": {"id": RUN, "head_sha": SHA}}])
        self.refused(verdict(fake), "go-test-json-sandbox-stock", "missing")

    def test_an_expired_artefact_is_refused(self):
        fake = Fake()
        fake.arts[0]["expired"] = True
        self.refused(verdict(fake), "expired")

    def test_an_artefact_of_another_commit_is_refused(self):
        fake = Fake()
        fake.arts[1]["workflow_run"]["head_sha"] = "e" * 40
        self.refused(verdict(fake), "go-test-json-sidecars-arm64", "commit")

    def test_an_artefact_without_its_results_file_is_refused(self):
        fake = Fake()
        fake.zips[11] = zipped(STOCK, member="other.txt")
        self.refused(verdict(fake), "go-test.json")

    def test_a_missing_cell_is_refused(self):
        stock = go_json(("TestDecide", "pass", []), ("TestEscape/N/exec-slot", "skip", [UNAVAILABLE]))
        self.refused(verdict(Fake(stock=stock)), "TestEscape/F/exec-slot", "missing")

    def test_an_extra_skip_in_f_is_refused(self):
        stock = go_json(
            ("TestDecide", "pass", []),
            ("TestEscape/N/exec-slot", "skip", [UNAVAILABLE]),
            ("TestEscape/F/exec-slot", "skip", [UNAVAILABLE]),
        )
        self.refused(verdict(Fake(stock=stock)), "TestEscape/F/exec-slot", "tier F")

    def test_n_unavailable_without_its_tier_report_is_refused(self):
        stock = go_json(
            ("TestDecide", "pass", []),
            ("TestEscape/N/exec-slot", "skip", ["no reason given"]),
            ("TestEscape/F/exec-slot", "pass", []),
        )
        self.refused(verdict(Fake(stock=stock)), "tier report")

    def test_a_column_read_from_no_artefact_is_refused(self):
        # local columns come from the local run (gojson_check, which also refuses n-unavailable in an F column).
        self.refused(verdict(Fake(), options(columns=["local-F"])), "local-F", "no artefact")

    def test_arm64_must_show_the_refusal_sentence(self):
        arm = go_json(
            ("TestDecide", "pass", []),
            ("TestEscape/N/exec-slot", "pass", []),
            ("TestEscape/F/exec-slot", "pass", []),
        )
        self.refused(verdict(Fake(arm=arm)), "arm64", "refus")

    def test_a_failure_in_an_artefact_is_refused(self):
        stock = STOCK + json.dumps({"Action": "fail", "Package": PKG, "Test": "TestOther"}) + "\n"
        self.refused(verdict(Fake(stock=stock)), "TestOther")

    def test_only_the_named_columns_need_their_artefact(self):
        fake = Fake(artifacts=[{"id": 11, "name": "go-test-json-sandbox-stock", "expired": False, "workflow_run": {"id": RUN, "head_sha": SHA}}])
        self.assertEqual(verdict(fake, options(columns=["stock-amd64"])), [])


class MainTest(unittest.TestCase):
    def call(self, run_file_text, api, **over):
        with tempfile.TemporaryDirectory() as d:
            f = Path(d) / "github.json"
            f.write_text(run_file_text)
            inv = Path(d) / "inv.toml"
            inv.write_text(INVENTORY)
            argv = [
                "--check", str(f), "--repo", REPO, "--workflow", ".github/workflows/ci.yml", "--event", "workflow_dispatch",
                "--branch-prefix", "candidate/sb2a1-", "--head", SHA, "--jobs", ",".join(JOBS),
                "--inventory", str(inv), "--columns", "stock-amd64,arm64",
            ]
            out, err = io.StringIO(), io.StringIO()
            with redirect_stdout(out), redirect_stderr(err):
                code = gh_verdict.main(argv, api=api, tree_clean=lambda: True)
            return code, out.getvalue() + err.getvalue()

    def test_the_run_file_holds_the_run_id(self):
        code, _ = self.call(json.dumps({"run_id": RUN}), Fake())
        self.assertEqual(code, 0)

    def test_a_run_file_without_a_run_id_is_refused(self):
        for text in ("{}", '{"run_id": "abc"}', '{"run_id": 0}', "[1]", "not json"):
            code, msg = self.call(text, Fake())
            self.assertNotEqual(code, 0, text)

    def test_an_api_failure_is_a_refusal_not_a_crash(self):
        def broken(path):
            raise gh_verdict.ApiError("HTTP 404")

        code, msg = self.call(json.dumps({"run_id": RUN}), broken)
        self.assertEqual(code, 1)
        self.assertIn("HTTP 404", msg)


if __name__ == "__main__":
    unittest.main()
