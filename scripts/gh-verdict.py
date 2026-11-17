#!/usr/bin/env python3
"""Re-fetches a candidate's GitHub Actions run and checks it (block SB2a1, decision D6, criterion `github`).

  gh-verdict.py --check gates/SB2a1/github.json --repo oioio-space/inkwell --workflow .github/workflows/ci.yml \\
      --event workflow_dispatch --branch-prefix candidate/sb2a1- --head "$(git rev-parse HEAD)" --clean \\
      --jobs ci,netsim,sandbox-stock,sidecars-arm64 --inventory gates/inventories/SB2a1.toml --columns stock-amd64,arm64

The local file holds only {"run_id": N}; everything else is asked of the API (through `gh api`) now, and refused unless:
repository and head repository are the named one (no fork); the workflow path, event and head branch prefix match; the
run's head SHA is the candidate's HEAD and, with --clean, the tracked tree is clean; the run and each named job are
completed with success; for each column asked, its artefact (ARTIFACTS) is present, not expired, from this run and this
commit, holds `go-test.json`, and satisfies that column of the inventory (scripts/gojson_check.py).

Standard library only. The API is injectable (`verdict(..., api)`) so the tests replay recorded answers."""
import argparse
import importlib.util
import io
import json
import subprocess
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path

# The artefact each machine column is read from, and the structured results file inside it. The workflow
# (.github/workflows/ci.yml, jobs sandbox-stock and sidecars-arm64) uploads exactly these.
ARTIFACTS = {"stock-amd64": "go-test-json-sandbox-stock", "arm64": "go-test-json-sidecars-arm64"}
RESULTS_FILE = "go-test.json"


class ApiError(Exception):
    """The GitHub API (or `gh`) failed."""


@dataclass
class Options:
    repo: str
    workflow: str
    event: str
    branch_prefix: str
    head: str
    clean: bool
    jobs: list
    columns: list


def _load_gojson_check():
    # `python3 -I` leaves the script's directory off sys.path: load the sibling by path.
    path = Path(__file__).resolve().parent / "gojson_check.py"
    spec = importlib.util.spec_from_file_location("gojson_check", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def gh_api(path):
    """The answer body of `gh api PATH` (JSON, or the zip bytes of an artefact)."""
    try:
        done = subprocess.run(["gh", "api", path], capture_output=True, timeout=180, check=False)
    except (OSError, subprocess.TimeoutExpired) as err:
        raise ApiError(f"gh api {path}: {err}") from err
    if done.returncode != 0:
        raise ApiError(f"gh api {path}: {done.stderr.decode(errors='replace').strip()}")
    return done.stdout


def git_tree_clean():
    done = subprocess.run(["git", "status", "--porcelain", "--untracked-files=no"], capture_output=True, text=True, check=False)
    return done.returncode == 0 and not done.stdout.strip()


def _json(api, path):
    try:
        return json.loads(api(path))
    except ValueError as err:
        raise ApiError(f"{path}: the answer is not JSON") from err


def verdict(run_id, o, api, tree_clean, inventory_path):
    """The list of reasons to refuse the run; empty when it is accepted."""
    problems = []
    base = f"repos/{o.repo}/actions"
    run = _json(api, f"{base}/runs/{run_id}")

    def want(label, got, expected):
        if got != expected:
            problems.append(f"{label} is {got!r}, expected {expected!r}")

    want("run id", run.get("id"), run_id)
    want("repository", (run.get("repository") or {}).get("full_name"), o.repo)
    head_repo = run.get("head_repository") or {}
    want("head repository", head_repo.get("full_name"), o.repo)
    if head_repo.get("fork"):
        problems.append("head repository is a fork")
    want("workflow path", str(run.get("path", "")).split("@")[0], o.workflow)
    want("event", run.get("event"), o.event)
    branch = run.get("head_branch") or ""
    if not branch.startswith(o.branch_prefix) or branch == o.branch_prefix:
        problems.append(f"head branch is {branch!r}, expected {o.branch_prefix}<n>")
    want("head sha", run.get("head_sha"), o.head)
    if o.clean and not tree_clean():
        problems.append("the tracked tree of this checkout is not clean")
    want("run status", run.get("status"), "completed")
    want("run conclusion", run.get("conclusion"), "success")

    jobs = {j.get("name"): j for j in _json(api, f"{base}/runs/{run_id}/jobs?per_page=100").get("jobs", [])}
    for name in o.jobs:
        job = jobs.get(name)
        if job is None:
            problems.append(f"job {name} is missing from the run")
        elif job.get("status") != "completed" or job.get("conclusion") != "success":
            problems.append(f"job {name}: status {job.get('status')!r}, conclusion {job.get('conclusion')!r}, expected completed success")

    gojson = _load_gojson_check()
    inventory = gojson.load_inventory(inventory_path)
    artifacts = {a.get("name"): a for a in _json(api, f"{base}/runs/{run_id}/artifacts?per_page=100").get("artifacts", [])}
    for column in o.columns:
        name = ARTIFACTS.get(column)
        if name is None:
            problems.append(f"column {column} has no artefact (known: {sorted(ARTIFACTS)})")
            continue
        art = artifacts.get(name)
        if art is None:
            problems.append(f"artefact {name} is missing from the run")
            continue
        if art.get("expired"):
            problems.append(f"artefact {name} is expired")
            continue
        origin = art.get("workflow_run") or {}
        if origin.get("id") != run_id or origin.get("head_sha") != o.head:
            problems.append(f"artefact {name} comes from another run or commit ({origin.get('id')}, {origin.get('head_sha')})")
            continue
        try:
            with zipfile.ZipFile(io.BytesIO(api(f"{base}/artifacts/{art['id']}/zip"))) as z:
                if RESULTS_FILE not in z.namelist():
                    problems.append(f"artefact {name} holds no {RESULTS_FILE}")
                    continue
                text = z.read(RESULTS_FILE).decode("utf-8", errors="replace")
        except zipfile.BadZipFile:
            problems.append(f"artefact {name} is not a zip")
            continue
        try:
            found = gojson.check_stream(text.splitlines(), inventory, [column])
        except gojson.CheckError as err:
            found = [str(err)]
        problems += [f"[{column}] {p}" for p in found]
    return problems


def main(argv=None, api=None, tree_clean=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--check", required=True, help='file holding {"run_id": N}')
    ap.add_argument("--repo", required=True)
    ap.add_argument("--workflow", required=True)
    ap.add_argument("--event", required=True)
    ap.add_argument("--branch-prefix", required=True)
    ap.add_argument("--head", required=True)
    ap.add_argument("--clean", action="store_true")
    ap.add_argument("--jobs", required=True)
    ap.add_argument("--inventory", required=True)
    ap.add_argument("--columns", required=True)
    a = ap.parse_args(argv)
    try:
        record = json.loads(Path(a.check).read_text())
        run_id = record["run_id"]
        if isinstance(run_id, bool) or not isinstance(run_id, int) or run_id <= 0:
            raise ValueError("run_id must be a positive integer")
    except (OSError, ValueError, KeyError, TypeError) as err:
        print(f"gh-verdict: {a.check}: {err}", file=sys.stderr)
        return 2
    opts = Options(a.repo, a.workflow, a.event, a.branch_prefix, a.head, a.clean, a.jobs.split(","), a.columns.split(","))
    try:
        problems = verdict(run_id, opts, api or gh_api, tree_clean or git_tree_clean, a.inventory)
    except (ApiError, OSError, ValueError) as err:
        print(f"gh-verdict: {err}", file=sys.stderr)
        return 1
    for p in problems:
        print(f"gh-verdict: {p}", file=sys.stderr)
    if problems:
        return 1
    print(f"gh-verdict: run {run_id} of {a.repo} accepted (jobs {a.jobs}; columns {a.columns})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
