"""Small sequential gate runner; commands are trusted repository configuration.

Every run first executes the anti-weakening guard from the merge-base's
version of scripts/gate-guard.py (never the working tree's), clears
gates/<lot>/ so stale files are never hashed as evidence, and validates each
deferral against its target lot.

scripts/gate.sh runs this file from the merge-base (see there), isolated
(python3 -I) from a temporary copy; INKWELL_GATE_ROOT then names the checkout
and INKWELL_GATE_RUNNER says which runner ran, recorded in the manifest.

Every run records each finished criterion (gates/<lot>/resume/<id>.json). With
--resume, a criterion whose record still matches the same candidate is re-used
instead of run (docs/plans/2026-11-12-gr-gate-resume.md).

No gate runs another gate (docs/plans/2026-11-12-method-v2.md, rule 6): a gate invoked from inside a criterion
(INKWELL_GATE_PARENT set) runs nothing; the parent records it as a deferred obligation (manifest, and the ledger
gates/deferred.jsonl), closed by the next passing top-level run of that lot. --deferred lists open obligations.

Usage: gate.py [--resume] <lot> | gate.py --base-guard | gate.py --deferred
"""
import hashlib
import importlib.util
import json
import os
import pathlib
import platform
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
import tomllib

# gate.sh runs only a base runner that declares this protocol.
GATE_RUNNER_PROTOCOL = 1
SCRIPTS = pathlib.Path(__file__).resolve().parent
ROOT = pathlib.Path(os.environ.get("INKWELL_GATE_ROOT") or SCRIPTS.parent).resolve()


# The interface walkthrough's captures (skill ux-walkthrough), kept across gate runs.
WALKTHROUGH_DIR = "ux"
# The gate's inputs written before a run: github.json names the GitHub run a `github` criterion checks
# (scripts/gh-verdict.py). They are not evidence, so a reset keeps them (2026-11-17: erasing it made the
# criterion fail on a run that was all green).
GATE_INPUTS = frozenset({"github.json"})


def reset_evidence(out):
    """Empties gates/<lot>/ for a new run: evidence is only what this run produced. The block's
    reports (*.md: SIMPLIFY, reconstructions, reviews) are versioned documents, not evidence, and
    are kept (2026-11-08: an rmtree of the whole folder erased them), and so is ux/, the captures of
    the interface walkthrough, which runs before the gate and is not its evidence (2026-11-14: the 58
    captures of W58a were erased). Symlinks are removed, never followed."""
    out.mkdir(parents=True, exist_ok=True)
    for entry in out.iterdir():
        if entry.suffix == ".md" and entry.is_file() and not entry.is_symlink():
            continue
        if entry.name == WALKTHROUGH_DIR and entry.is_dir() and not entry.is_symlink():
            continue
        if entry.name in GATE_INPUTS and entry.is_file() and not entry.is_symlink():
            continue
        if entry.is_dir() and not entry.is_symlink():
            shutil.rmtree(entry)
        else:
            entry.unlink()


def _load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


# By path, beside this file: isolated mode (-I) puts no script directory on sys.path.
verify = _load("archive", SCRIPTS / "perf/archive.py").verify
NAME = re.compile(r"[A-Za-z0-9_-]+")
OVERRIDES = "gates/guard-overrides.json"
# The known-lot list: a ```gate-lots block of "LOT: CRITERION …" lines.
KNOWN_LOTS = "docs/objectif-en-cours.md"
# Run a guard source as if it lived at argv[1], so its ROOT is the checkout.
BOOTSTRAP = ("import sys; path = sys.argv[1]; source = sys.stdin.read(); sys.argv = [path, *sys.argv[2:]]; "
             "exec(compile(source, path, 'exec'), {'__name__': '__main__', '__file__': path})")


def version(command):
    try:
        p = subprocess.run(command, shell=True, cwd=ROOT, capture_output=True, text=True)
        return (p.stdout + p.stderr).strip() if p.returncode == 0 else "unavailable: " + (p.stdout + p.stderr).strip()
    except OSError as exc:
        return "unavailable: " + str(exc)


def known_lots(root):
    path = root / KNOWN_LOTS
    match = re.search(r"^```gate-lots\n(.*?)^```", path.read_text(), re.M | re.S) if path.exists() else None
    lots = {}
    for line in (match[1] if match else "").splitlines():
        lot, _, criteria = line.partition(":")
        if lot.strip():
            lots[lot.strip()] = set(criteria.split())
    return lots


def deferral_problem(criterion, lot, root):
    """Why a deferral is invalid, or None; the target lot must re-list the criterion."""
    target = criterion.get("target_lot", "")
    if not NAME.fullmatch(target) or target == lot:
        return "invalid target lot"
    if not criterion.get("reason", "").strip():
        return "missing reason"
    if "command" in criterion:
        return "a deferral cannot hide a command"
    config = root / "gates" / (target + ".toml")
    if config.exists():
        listed = {c["id"] for c in tomllib.loads(config.read_text()).get("criteria", [])}
        return None if criterion["id"] in listed else f"gates/{target}.toml does not list {criterion['id']}"
    lots = known_lots(root)
    if target not in lots:
        return f"unknown target lot {target}: no gates/{target}.toml and not in {KNOWN_LOTS}"
    return None if criterion["id"] in lots[target] else f"known lot {target} in {KNOWN_LOTS} does not list {criterion['id']}"


def base_guard(root, stream, ref="main"):
    """Run the merge-base's guard against the working tree; returns its record."""
    record = {"ref": ref}
    try:
        base = subprocess.check_output(["git", "merge-base", "HEAD", ref], cwd=root, text=True).strip()
        source = subprocess.check_output(["git", "show", base + ":scripts/gate-guard.py"], cwd=root)
    except (OSError, subprocess.CalledProcessError) as exc:
        stream.write(f"base guard unavailable: {exc}\n".encode())
        return dict(record, status="fail", reason=str(exc))
    stream.flush()
    # Isolated (-I) and from outside the checkout: nothing in the tree is imported.
    with tempfile.TemporaryDirectory(prefix="gate-guard-") as outside:
        p = subprocess.run([sys.executable, "-I", "-c", BOOTSTRAP, str(root / "scripts/gate-guard.py"), "--base", base],
                           input=source, cwd=outside, stdout=stream, stderr=subprocess.STDOUT)
    return dict(record, base=base, source_sha256=hashlib.sha256(source).hexdigest(),
                exit_code=p.returncode, status="pass" if p.returncode == 0 else "fail")


def exclusive_records(out):
    path = out / "exclusive.jsonl"
    return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []


# Per-criterion records, in gates/<lot>/resume/<id>.json, and the fields of their body.
RECORDS = "resume"
RECORD_FIELDS = {"lot", "id", "criterion", "entry", "evidence", "digest", "runner", "tools", "environment"}


def is_report(relative):
    """A block report kept by reset_evidence: a *.md file directly in gates/<lot>/."""
    return "/" not in relative and relative.endswith(".md")


def candidate_digest(root, lot):
    """SHA-256 of the candidate: path, executable bit and content of every tracked or untracked non-ignored file,
    except the gate's own outputs (gates/<lot>.json, the evidence of gates/<lot>/ but not its *.md reports).
    None outside a git work tree."""
    try:
        listed = subprocess.run(["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
                                cwd=root, capture_output=True, check=True).stdout
    except (OSError, subprocess.CalledProcessError):
        return None
    own = f"gates/{lot}/"
    digest = hashlib.sha256()
    for path in sorted({os.fsdecode(p) for p in listed.split(b"\0") if p}):
        if path in (f"gates/{lot}.json", LEDGER) or (path.startswith(own) and not is_report(path[len(own):])):
            continue
        file = root / path
        if file.is_symlink():
            kind, data = "link", os.fsencode(os.readlink(file))
        elif file.is_file():
            kind, data = "x" if file.stat().st_mode & 0o100 else "-", file.read_bytes()
        else:
            kind, data = "missing" if not file.exists() else "other", b""
        digest.update(f"{path}\0{kind}\0{hashlib.sha256(data).hexdigest()}\n".encode("utf-8", "surrogateescape"))
    return digest.hexdigest()


def tool_versions(root):
    versions = {"go": version("go version"), "node": version("node --version"),
                "typst": version("typst --version"), "tinymist": version("tinymist --version"),
                "os": platform.platform()}
    pins_file = root / "internal/sidecar/manifest.json"
    if pins_file.exists():
        versions["embedded_sidecars"] = {
            name: data["version"] for name, data in json.loads(pins_file.read_text())["sidecars"].items()}
    return versions


def resume_context(root):
    """What a record must still match besides the candidate: the runner's content, the tools, the environment
    (INKWELL_GATE_RUNNER names the merge-base, not the runner's content)."""
    runner = {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
              for p in (pathlib.Path(__file__), SCRIPTS / "perf/archive.py")}
    environment = {k: v for k, v in os.environ.items() if k.startswith("INKWELL_") and k != "INKWELL_GATE_RUNNER"}
    return {"runner": runner, "tools": tool_versions(root), "environment": environment}


def _canonical(body):
    return json.dumps(body, sort_keys=True, separators=(",", ":")).encode()


def snapshot(root, out):
    """Evidence files of gates/<lot>/ and their SHA-256 (records, reports and the shared exclusive.jsonl apart)."""
    files = {}
    for p in out.rglob("*"):
        relative = str(p.relative_to(out))
        if p.is_file() and not p.is_symlink() and not is_report(relative) and relative != "exclusive.jsonl" \
                and not relative.startswith(RECORDS + "/"):
            files[str(p.relative_to(root))] = hashlib.sha256(p.read_bytes()).hexdigest()
    return files


def write_record(out, body):
    """Atomically, after the criterion's command exited: an interrupted criterion leaves no record."""
    path = out / RECORDS / (body["id"] + ".json")
    path.parent.mkdir(exist_ok=True)
    temporary = path.with_suffix(".json.tmp")
    temporary.write_text(json.dumps({"body": body, "sha256": hashlib.sha256(_canonical(body)).hexdigest()}, indent=2))
    temporary.replace(path)


def read_record(out, lot, cid):
    """The record's body if it is intact and belongs to this criterion, else None."""
    try:
        record = json.loads((out / RECORDS / (cid + ".json")).read_text())
        body = record["body"]
        intact = record["sha256"] == hashlib.sha256(_canonical(body)).hexdigest() and set(body) == RECORD_FIELDS
        return body if intact and body["lot"] == lot and body["id"] == cid else None
    except (OSError, ValueError, KeyError, TypeError):
        return None


def resume_problem(root, body, criterion, digest, context):
    """Why a record cannot stand for this criterion now, or None."""
    if body is None:
        return "no intact record (missing, interrupted or corrupted)"
    if criterion.get("exclusive") or body["entry"].get("exclusive_steps"):
        return "native performance measurement (exclusive): always re-run"
    if body["criterion"] != criterion:
        return "criterion (command) changed"
    if digest is None or body["digest"] != digest:
        return "candidate digest changed"
    for key, what in (("runner", "runner changed"), ("tools", "tool versions changed"),
                      ("environment", "INKWELL_* environment changed")):
        if body[key] != context[key]:
            return what
    for path, sha in body["evidence"].items():
        file = root / path
        if file.is_symlink() or not file.is_file() or hashlib.sha256(file.read_bytes()).hexdigest() != sha:
            return "evidence missing or changed: " + path
    return None


def keep_for_resume(root, out, keep):
    """Empties gates/<lot>/ but for the reports and the given files (intact records and their evidence)."""
    out.mkdir(parents=True, exist_ok=True)
    for p in sorted(out.rglob("*"), reverse=True):
        relative = str(p.relative_to(out))
        if p.is_dir() and not p.is_symlink():
            continue
        if not is_report(relative) and str(p.relative_to(root)) not in keep:
            p.unlink()


def discard(root, out, cid, body):
    """Removes a re-run criterion's record and its evidence (only ever inside gates/<lot>/)."""
    for path in (body or {}).get("evidence", {}):
        file = root / path
        if out in file.parents and ".." not in pathlib.Path(path).parts:
            file.unlink(missing_ok=True)
    (out / RECORDS / (cid + ".json")).unlink(missing_ok=True)


# A nested gate hands its lot name to the parent through this file of gates/<parent>/, read after each criterion.
NESTED = "nested-deferred.txt"
NESTED_REASON = "no gate runs another gate (docs/plans/2026-11-12-method-v2.md rule 6, user decision 2026-11-12)"
# Deferred obligations, one JSON object per line, tracked; open until a passing top-level run of the lot closes them.
LEDGER = "gates/deferred.jsonl"
OVERDUE_DAYS = 7


def defer_nested(lot, root, parent, parent_out):
    """Hands `lot` to the parent gate as a deferred obligation instead of running it. Refuses when the parent is not
    a gate directory of this checkout (the deferral could not be recorded) or the lot has no gate; the parent
    re-validates every handed line."""
    expected = (root / "gates" / parent).resolve() if NAME.fullmatch(parent) else None
    if expected is None or not parent_out or pathlib.Path(parent_out).resolve() != expected or not expected.is_dir():
        print(f"nested gate {lot} refused: INKWELL_GATE_PARENT_OUT is not gates/{parent} of {root}", flush=True)
        return 1
    with (expected / NESTED).open("a") as handoff:
        handoff.write(lot + "\n")
    if not (root / "gates" / (lot + ".toml")).is_file():
        print(f"nested gate {lot} refused: no gates/{lot}.toml", flush=True)
        return 1
    print(f"nested gate {lot} deferred: no nested gates (method v2, user decision 2026-11-12)", flush=True)
    return 0


def take_nested(root, lot, out):
    """(obligations, problem) handed by nested gates during the last criterion; clears the hand-off. Each valid lot
    once, in order; any invalid line is a problem that fails the criterion, never dropped."""
    path = out / NESTED
    if not path.is_file():
        return [], None
    lines = path.read_text().splitlines()
    path.unlink()
    bad = [line for line in lines
           if not NAME.fullmatch(line) or line == lot or not (root / "gates" / (line + ".toml")).is_file()]
    obligations = [dict(lot=name, reason=NESTED_REASON) for name in dict.fromkeys(lines) if name not in bad]
    return obligations, ("invalid nested deferral: " + ", ".join(map(repr, bad))) if bad else None


# Rule G-1 (docs/plans/2026-11-15-pc1a-take-over.md): a gate has a deadline, max_minutes, that its criteria's
# `timeout` caps fit in; the gates written before the rule are listed, and the list is closed.
LEGACY_NO_DEADLINE = "gates/legacy-no-deadline.txt"
CAP = re.compile(r"\btimeout\s+(?:-k\s+\S+\s+)?(\d+(?:\.\d+)?)([smhd]?)(?=\s)")
UNIT_SECONDS = {"": 1, "s": 1, "m": 60, "h": 3600, "d": 86400}
KILL_GRACE_SECONDS = 10
# The deadline's clock, replaced by the tests.
clock = time.monotonic


def criterion_caps(command):
    """Seconds of every `timeout [-k K] N` cap in a criterion's command, summed."""
    return sum(float(n) * UNIT_SECONDS[unit] for n, unit in CAP.findall(command))


def legacy_gates(root):
    path = root / LEGACY_NO_DEADLINE
    lines = path.read_text().splitlines() if path.exists() else []
    return {line.strip() for line in lines if line.strip() and not line.startswith("#")}


def deadline_problem(lot, config, root):
    """Why the gate may not start (no max_minutes and not legacy, or caps past it), or None."""
    if "max_minutes" not in config:
        return None if lot in legacy_gates(root) else \
            f"gates/{lot}.toml has no max_minutes and is not listed in {LEGACY_NO_DEADLINE}"
    limit = config["max_minutes"] * 60
    caps = sum(criterion_caps(c.get("command", "")) for c in config["criteria"])
    return None if caps <= limit else \
        f"the timeout caps of gates/{lot}.toml sum to {caps:g} s, past max_minutes = {config['max_minutes']} ({limit:g} s)"


def process_groups(leader):
    """The process groups of `leader`'s tree: its own and those its descendants made (`timeout` makes one)."""
    table = subprocess.run(["ps", "-A", "-o", "pid=,ppid=,pgid="], capture_output=True, text=True).stdout
    rows = [tuple(map(int, line.split())) for line in table.splitlines() if len(line.split()) == 3]
    tree, groups = {leader}, {leader}
    while grown := {pid for pid, ppid, _ in rows if ppid in tree} - tree:
        tree |= grown
    groups.update(pgid for pid, _, pgid in rows if pid in tree)
    return groups


def signal_groups(groups, sig):
    """Signals each group; returns those still present."""
    alive = set()
    for group in groups:
        try:
            os.killpg(group, sig)
            alive.add(group)
        except ProcessLookupError:
            pass
    return alive


def run_until(command, deadline, **kwargs):
    """Runs a criterion's shell command; returns its exit code, or None when the deadline (a clock() value) stopped
    it: SIGTERM to the groups of its tree, SIGKILL KILL_GRACE_SECONDS later. Without a deadline (a legacy gate) it
    runs as before, in the runner's group."""
    p = subprocess.Popen(command, shell=True, start_new_session=deadline is not None, **kwargs)
    try:
        return p.wait(None if deadline is None else max(0, deadline - clock()))
    except subprocess.TimeoutExpired:
        pass
    except BaseException:
        # Ctrl-C or any error of the runner: in its own session the criterion would outlive the runner.
        if deadline is not None:
            stop_tree(p)
        raise
    stop_tree(p)
    return None


def stop_tree(p):
    """SIGTERM to the groups of p's tree, SIGKILL KILL_GRACE_SECONDS later; reaps p."""
    # Taken before the signal: a dying parent's children are re-parented out of reach.
    groups = signal_groups(process_groups(p.pid), signal.SIGTERM)
    grace = time.monotonic() + KILL_GRACE_SECONDS
    while groups and time.monotonic() < grace:
        p.poll()  # reaps the leader, so a zombie does not hold its group
        time.sleep(0.1)
        groups = signal_groups(groups, 0)
    signal_groups(groups, signal.SIGKILL)
    p.wait()


DEADLINE = "deadline"


def stop_at_deadline(entries, c, **fields):
    entries.append(dict(id=c["id"], command=c.get("command", ""), status=DEADLINE,
                        problem="stopped or not run: the gate reached its max_minutes", **fields))
    print(c["id"], DEADLINE, flush=True)


def _now():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def _head(root):
    try:
        return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True,
                                       stderr=subprocess.DEVNULL).strip()
    except (OSError, subprocess.CalledProcessError):
        return None


def read_ledger(root):
    path = root / LEDGER
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()] if path.exists() else []


def ledger_open(root, gate, cid, obligations):
    """Appends each obligation not already open for the same (lot, gate, criterion): the oldest date is kept."""
    open_keys = {(r["lot"], r["from_gate"], r["criterion"]) for r in read_ledger(root) if "closed_commit" not in r}
    rows = [dict(lot=o["lot"], from_gate=gate, criterion=cid, commit=_head(root), date=_now())
            for o in obligations if (o["lot"], gate, cid) not in open_keys]
    if rows:
        with (root / LEDGER).open("a") as ledger:
            ledger.writelines(json.dumps(r, sort_keys=True) + "\n" for r in rows)


def ledger_close(root, lot):
    """A passing top-level run of `lot` closes its open obligations, recording the closing commit."""
    rows = read_ledger(root)
    closing = [r for r in rows if r["lot"] == lot and "closed_commit" not in r]
    if not closing:
        return
    for r in closing:
        r.update(closed_commit=_head(root), closed_date=_now())
    temporary = (root / LEDGER).with_suffix(".jsonl.tmp")
    temporary.write_text("".join(json.dumps(r, sort_keys=True) + "\n" for r in rows))
    temporary.replace(root / LEDGER)


def deferred_report(root, stream):
    """Lists open obligations; 1 if any is older than OVERDUE_DAYS or the ledger is unreadable (fail closed)."""
    try:
        rows = [r for r in read_ledger(root) if "closed_commit" not in r]
        now = datetime.now(timezone.utc)
        overdue = 0
        for r in rows:
            age = (now - datetime.fromisoformat(r["date"])).total_seconds() / 86400
            overdue += age > OVERDUE_DAYS
            stream.write(f"open {r['lot']}: deferred by {r['from_gate']}/{r['criterion']} at {r['commit']}, "
                         f"{r['date']} ({age:.1f} days){' overdue' if age > OVERDUE_DAYS else ''}\n")
    except (OSError, ValueError, KeyError, TypeError) as exc:
        stream.write(f"unreadable ledger {LEDGER}: {exc}\n")
        return 1
    stream.write(f"{len(rows)} open deferred obligation(s), {overdue} overdue (> {OVERDUE_DAYS} days)\n")
    return int(overdue > 0)


def run(lot, root=ROOT, pins=None, guard=True, resume=False):
    if not NAME.fullmatch(lot):
        raise ValueError("invalid lot name")
    if "INKWELL_GATE_PARENT" in os.environ:
        return defer_nested(lot, root, os.environ["INKWELL_GATE_PARENT"], os.environ.get("INKWELL_GATE_PARENT_OUT", ""))
    sealed = verify(root, pins)
    config = tomllib.loads((root / "gates" / (lot + ".toml")).read_text())
    out = root / "gates" / lot
    if out.is_symlink():
        raise ValueError("gates/" + lot + " must not be a symlink")
    ids = [c["id"] for c in config["criteria"]]
    if len(set(ids)) != len(ids) or any(not NAME.fullmatch(i) for i in ids):
        raise ValueError("invalid or duplicate criterion IDs")
    problem = deadline_problem(lot, config, root)
    if problem:
        raise ValueError(problem)
    deadline = clock() + config["max_minutes"] * 60 if "max_minutes" in config else None
    context = resume_context(root)
    if resume:
        # Only intact records and their evidence survive; everything else is stale (never hashed as evidence).
        bodies = {c["id"]: read_record(out, lot, c["id"]) for c in config["criteria"] if "command" in c}
        keep = {str(out.relative_to(root) / RECORDS / (cid + ".json")) for cid, b in bodies.items() if b}
        keep.update(path for b in bodies.values() if b for path in b["evidence"])
        keep_for_resume(root, out, keep)
        reused, rerun = [], {}
    else:
        reset_evidence(out)
    entries = []
    guard_record = None
    if guard:
        log = out / "base-guard.log"
        with log.open("wb") as stream:
            guard_record = dict(base_guard(root, stream), log=str(log.relative_to(root)))
        print("base-guard", guard_record["status"], flush=True)
    # A failing base guard means the gate can no longer pass: stop before the criteria (some run for hours)
    # instead of failing at the end. Each criterion is recorded as not run.
    stop = guard_record is not None and guard_record["status"] == "fail"
    for c in config["criteria"]:
        if stop:
            entries.append(dict(id=c["id"], command=c.get("command", ""), status="fail",
                                problem="not run: the base guard failed (see base-guard.log)"))
            print(c["id"], "fail", "not run: base guard failed", flush=True)
            continue
        if deadline is not None and clock() >= deadline:
            stop_at_deadline(entries, c)
            continue
        if c.get("status") == "deferred":
            problem = deferral_problem(c, lot, root)
            entries.append(dict(c, status="fail", problem=problem) if problem else dict(c))
            print(c["id"], entries[-1]["status"], problem or "", flush=True)
            continue
        if c.get("status", "pass") not in ("pass", "fail"):
            raise ValueError("invalid criterion status")
        # Judged when reached: an earlier re-run may have changed this criterion's inputs or evidence.
        digest = candidate_digest(root, lot)
        if resume:
            body = bodies.get(c["id"])
            problem = resume_problem(root, body, c, digest, context)
            if problem is None:
                entries.append(dict(body["entry"], resumed=True))
                reused.append(c["id"])
                print(c["id"], entries[-1]["status"], "resumed", flush=True)
                continue
            rerun[c["id"]] = problem
            discard(root, out, c["id"], body)
        log = out / (c["id"] + ".log")
        before = snapshot(root, out)
        start = time.monotonic()
        with log.open("wb") as stream:
            code = run_until(c["command"], deadline, cwd=root, stdout=stream, stderr=subprocess.STDOUT,
                             env=dict(os.environ, INKWELL_PERF_OUT=str(out), INKWELL_GATE_CRITERION=c["id"],
                                      INKWELL_GATE_PARENT=lot, INKWELL_GATE_PARENT_OUT=str(out)))
        if code is None:
            # Interrupted: no record, as for any interrupted criterion; the rest is not run.
            stop_at_deadline(entries, c, log=str(log.relative_to(root)), duration_seconds=time.monotonic()-start)
            continue
        entry = dict(id=c["id"], command=c["command"], status="pass" if code == 0 and c.get("status") != "fail" else "fail",
                     exit_code=code, duration_seconds=time.monotonic()-start,
                     log=str(log.relative_to(root)))
        # scripts/heavy.sh --exclusive records each isolated measurement step.
        records = [r for r in exclusive_records(out) if r["criterion"] == c["id"]]
        if records:
            entry["exclusive"] = all(r["exclusive"] for r in records)
            entry["exclusive_steps"] = records
        if c.get("exclusive") and not entry.get("exclusive"):
            entry.update(status="fail", problem="performance measurement ran without the exclusive heavy lock")
        # The criterion keeps its own commands' status; a chained gate is an obligation, never a pass for that lot.
        obligations, problem = take_nested(root, lot, out)
        if obligations:
            entry["deferred_obligations"] = obligations
            ledger_open(root, lot, c["id"], obligations)
        if problem:
            entry.update(status="fail", problem="; ".join(filter(None, (entry.get("problem"), problem))))
        entries.append(entry)
        if digest is not None:
            after = snapshot(root, out)
            evidence = {path: sha for path, sha in after.items() if before.get(path) != sha}
            evidence[str(log.relative_to(root))] = after[str(log.relative_to(root))]
            write_record(out, dict(lot=lot, id=c["id"], criterion=c, entry=entry, evidence=evidence, digest=digest,
                                   **context))
        print(c["id"], entry["status"], entry.get("problem", ""), flush=True)
    if verify(root, pins) != sealed:
        raise ValueError("gate modified immutable archive")
    files = list(out.rglob("*"))
    for pattern in config.get("raw_results", []):
        files.extend(root.glob(pattern))
    hashes = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
              for p in sorted(set(files)) if p.is_file()}
    failed = any(e["status"] in ("fail", DEADLINE) for e in entries) or (guard_record or {}).get("status") == "fail"
    manifest = dict(lot=lot, criteria=entries, versions=context["tools"], sha256=hashes,
        status="fail" if failed else "pass",
        deferrals=[e for e in entries if e["status"] == "deferred"],
        known_failures=config.get("known_failures", []),
        # Chained gates that were not run (rule 6): each runs on its own when needed.
        deferred_obligations=[dict(o, criterion=e["id"]) for e in entries for o in e.get("deferred_obligations", [])])
    if guard_record:
        manifest["base_guard"] = guard_record
    if resume:
        manifest["resume"] = {"reused": reused, "rerun": rerun}
    manifest["runner"] = {"source": os.environ.get("INKWELL_GATE_RUNNER", "direct: " + str(pathlib.Path(__file__))),
                          "sha256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    overrides = root / OVERRIDES
    # Reviewed overrides are part of the evidence, shown in full.
    manifest["guard_overrides"] = json.loads(overrides.read_text())["overrides"] if overrides.exists() else []
    for pattern in config.get("measurement_reports", []):
        report = root / pattern
        manifest.setdefault("measurements", []).append(
            json.loads(report.read_text()) if report.exists() else
            {"status": "unavailable", "reason": "measurement command did not produce " + pattern})
    target = root / "gates" / (lot + ".json")
    temporary = target.with_suffix(".json.tmp")
    temporary.write_text(json.dumps(manifest, indent=2) + "\n")
    temporary.replace(target)
    if not failed:
        ledger_close(root, lot)
    return int(failed)


def main(argv):
    if argv == ["--base-guard"]:
        return base_guard(ROOT, sys.stdout.buffer)["status"] != "pass"
    if argv == ["--deferred"]:
        return deferred_report(ROOT, sys.stdout)
    lots = [a for a in argv if a != "--resume"]
    if len(lots) != 1 or len(argv) - len(lots) > 1 or lots[0].startswith("-"):
        return 2
    return run(lots[0], resume=len(argv) == 2)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
