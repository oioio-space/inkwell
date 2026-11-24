#!/usr/bin/env python3
"""Every open item anywhere carries the ID of a block present in PROGRESS.md's ordered list (block INV1).

Work was being set aside: open requirements, residuals, skipped tests, proposed ADRs, branches and worktrees
lived for weeks with no place in « Cap à long terme », and main's GitHub CI had no green run for three weeks.
This inventory lists each such item with its file:line and the fix; it fails when any is unfiled.

Checks (docs/plans/2026-11-17-inv1-inventory.md gives the reason of each threshold):
  1 requirements  docs/exigences.md rows not « ✅ » name a block of the ordered list (row ID, « Où » or « État »);
  2 residuals     a residual, deferral or follow-up in gates/*/SIMPLIFY.md, gates/*/WITNESSES.md or
                  docs/plans/*.md carries a tag « → ID » or « [ID] » of a listed block, « [accepted ADR NNNN] »
                  (consciously accepted, the ADR exists), « [done] » or « [obsolete: reason] »;
  3 skips         test.fixme, an unconditional skip or a skip saying TODO names a listed block; environment
                  skips (mDNS, root, links, OS, kernel, sockets, short mode...) are allowed by ENV_SKIP;
  4 adrs          an ADR « proposed » for more than 14 days is cited by the ordered list (« ADR NNNN ») or its
                  status line carries a tag;
  5 git           a branch not merged into main whose tip is older than 7 days, a worktree whose uncommitted
                  changes are older than 3 days, and a folder of .claude/worktrees/ git does not know, are listed
                  in PROGRESS.md « Travaux en cours » on a line naming a listed block;
  6 ci            (only when `gh` is authenticated) the latest completed ci.yml run on main is at most 7 days old
                  and every job completed with success; a red run is filed only by a line of « Travaux en cours »
                  naming its run ID and a listed block (a newer red run opens the finding again), and is then
                  printed as a notice;
  7 deferred      no obligation of gates/deferred.jsonl is overdue (scripts/gate.py --deferred).

A block ID nests: HB1a is filed when HB1 is listed.

Usage: inventory.py [--report] [--root DIR]   --report prints the list and exits 0 (session start).
"""
import argparse
import importlib.util
import io
import json
import pathlib
import re
import subprocess
import sys
from datetime import datetime, timezone

SCRIPTS = pathlib.Path(__file__).resolve().parent

ID = r"[A-Z]{1,5}-?[0-9][A-Za-z0-9]*(?:-[A-Z]{2,})?"
ID_RE = re.compile(rf"(?<![\w-])({ID})(?![\w-])")
# Tokens shaped like IDs that never name a block.
NOT_BLOCKS = re.compile(r"(?:UTF|SHA|MPL|ADR|CVE|GHSA|RFC|ISO|BLAKE|TLS|HTTP|IPV|ED|X)-?\d")
TAG_RE = re.compile(rf"(?:→|->)\s*\**`?({ID})|\[({ID})\]")
ACCEPTED_RE = re.compile(r"\[accepted ADR (\d{4})\]")
DONE_TAG = "[done]"
OBSOLETE_RE = re.compile(r"\[obsolete: [^\]]+\]")

MARKER = re.compile(
    r"résiduel|residual|\bdeferr(?:ed|al)\b|\bdiffér[ée]e?s?\b|\breport[ée]e?s?\b|follow[- ]ups?\b"
    r"|\b(?:for|until) later\b|\blater (?:block|lot|step|stage|pass)\b|\bto (?:do|fix|handle|address|revisit) later\b"
    r"|plus tard|not done|pas fait", re.I)

SKIP_RE = re.compile(r"\b(?:test|it|describe)\.(fixme|skip)\b|\bt\.Skip(?:f|Now)?\(")
TODO_RE = re.compile(r"TODO|FIXME|XXX|not merged|not yet|\bneeds? (?:B\d|wave|ADR)|known defect|blocked", re.I)
ENV_SKIP = re.compile(
    r"mDNS|multicast|\broot\b|Getuid|symlink|symbolic link|hard link|link count|Windows|GOOS|platform|this OS"
    r"|Linux-only|kernel|Unix modes|socket|taken meanwhile|AppArmor|namespace|testing\.Short|INKWELL_PERF"
    r"|process\.env|browserName|skipIf|covered by Fuzz|privilege|parent-death|refuses removals|/proc\b"
    r"|unavailable on this machine|refused on this machine|not delegated|block device|io controller|race detector", re.I)
TEST_FILE = re.compile(r"(?:_test\.go|\.(?:spec|test)\.[cm]?[jt]sx?)$")

PROPOSED_DAYS = 14
BRANCH_DAYS = 7
WORKTREE_DAYS = 3
CI_DAYS = 7
CI_WORKFLOW = "ci.yml"

CHECKS = ("requirements", "residuals", "skips", "adrs", "git", "ci", "deferred")


def git(root, *args, timeout=60):
    return subprocess.run(["git", "-C", str(root), *args], capture_output=True, text=True, timeout=timeout)


# ── PROGRESS.md: the ordered list and « Travaux en cours » ───────────────────────────────────────────────────

def ordered_list(progress):
    """Lines (number, text) of the numbered list that follows « Cap à long terme »."""
    lines = progress.splitlines()
    start = next((i for i, l in enumerate(lines) if "Cap à long terme" in l), None)
    if start is None:
        return []
    out, inside = [], False
    for i in range(start + 1, len(lines)):
        line = lines[i]
        if re.match(r"\s*\d+\.\s", line):
            inside = True
            out.append((i + 1, line))
        elif inside and line.startswith("   ") and line.strip():
            out.append((i + 1, line))
        elif inside:
            break
    return out


def block_ids(progress):
    ids = set()
    for _, line in ordered_list(progress):
        ids.update(m for m in ID_RE.findall(line) if not NOT_BLOCKS.match(m))
    return ids


def filed(ident, known):
    """An ID is filed when it, or the block it refines (HB1a → HB1, SB2a1 → SB2), is listed."""
    while True:
        if ident in known:
            return True
        shorter = re.sub(r"(?<=\d)[a-z]\d*$", "", ident)
        if shorter == ident:
            return False
        ident = shorter


def any_filed(text, known):
    return any(filed(m, known) for m in ID_RE.findall(text))


def in_progress_section(progress):
    """Lines of the « Travaux en cours » section (heading or bold label), up to the next heading."""
    lines = progress.splitlines()
    out, inside = [], False
    for line in lines:
        if re.match(r"\s*(?:#+\s*|\*\*)Travaux en cours", line):
            inside = True
            continue
        if inside and (line.startswith("#") or re.match(r"\*\*[^*]+\*\*\s*:?\s*$", line)):
            break
        if inside:
            out.append(line)
    return out


def listed_in_progress(name, section, known):
    pat = re.compile(rf"(?<![\w./-]){re.escape(name)}(?![\w-])")
    return any(pat.search(line) and any_filed(line, known) for line in section)


# ── 1 requirements ────────────────────────────────────────────────────────────────────────────────────────────

def check_requirements(root, known):
    path = root / "docs/exigences.md"
    found = []
    if not path.exists():
        return found
    for n, line in enumerate(path.read_text().splitlines(), 1):
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if not line.lstrip().startswith("|") or len(cells) < 5 or cells[0] in ("#", "") or set(cells[0]) <= set("-:"):
            continue
        state = cells[-1]
        if state.startswith("✅"):
            continue
        if not (filed(cells[0], known) or any_filed(cells[-2] + " " + state, known)):
            found.append(f"docs/exigences.md:{n}: {cells[0]} ({state[:30]}) names no block of the ordered list — "
                         f"name its block in « Où » (e.g. « UX1 ») or add a block for it to « Cap à long terme »")
    return found


# ── 2 residuals ───────────────────────────────────────────────────────────────────────────────────────────────

def _items(lines):
    """Splits markdown into items (start line, text, heading level of the enclosing section, heading text).
    An item is a bullet with its continuation lines, a table data row or a paragraph; code fences are skipped."""
    items, fence, current, heading = [], False, None, (0, "")
    for n, line in enumerate(lines, 1):
        if line.lstrip().startswith("```"):
            fence = not fence
            current = None
            continue
        if fence:
            continue
        h = re.match(r"(#+)\s+(.*)", line)
        bullet = re.match(r"\s*(?:[-*+]|\d+[.)])\s", line)
        row = line.lstrip().startswith("|")
        if h:
            heading, current = (len(h.group(1)), h.group(2)), None
            items.append([n, line, heading, True])
        elif not line.strip():
            current = None
        elif row:
            current = None
            if not re.fullmatch(r"[\s|:-]+", line):
                items.append([n, line, heading, False])
        elif bullet or current is None:
            current = [n, line, heading, False]
            items.append(current)
        else:
            current[1] += "\n" + line
    return items


def check_residuals(root, known):
    files = sorted([*root.glob("gates/*/SIMPLIFY.md"), *root.glob("gates/*/WITNESSES.md"), *root.glob("docs/plans/*.md")])
    found = []
    for path in files:
        rel = path.relative_to(root)
        items = _items(path.read_text(errors="replace").splitlines())
        opener_level, label_open = None, False
        for i, (n, text, heading, is_heading) in enumerate(items):
            if is_heading:
                level = heading[0]
                if opener_level is not None and level <= opener_level:
                    opener_level = None
                # A section titled by a marker (« ## Residuals ») makes each of its list items open; a document
                # title (« # ... deferred behind GP ») does not.
                if level >= 2 and MARKER.search(text) and opener_level is None:
                    opener_level = level
                label_open = False
                continue
            first = text.splitlines()[0]
            is_list_item = bool(re.match(r"\s*(?:[-*+]|\d+[.)])\s|\s*\|", first))
            if not is_list_item:
                label_open = False
            # A label such as « Residuals: » opens the list that follows it, and is not an item itself.
            if MARKER.search(text) and text.rstrip().endswith(":") and "\n" not in text and len(text) <= 80 \
                    and not is_list_item:
                label_open = True
                continue
            under = opener_level is not None and is_list_item or label_open and is_list_item
            if not (under or MARKER.search(text)):
                continue
            if DONE_TAG in text or OBSOLETE_RE.search(text) or any(filed(a or b, known) for a, b in TAG_RE.findall(text)):
                continue
            acc = ACCEPTED_RE.search(text)
            if acc and list((root / "docs/adr").glob(f"{acc.group(1)}-*.md")):
                continue
            found.append(f"{rel}:{n}: open item without a block tag: « {first.strip()[:90]} » — "
                         f"add « → ID » or « [ID] » of a block of the ordered list, « [accepted ADR NNNN] », « [done] » "
                         f"or « [obsolete: reason] »")
    return found


# ── 3 skips ───────────────────────────────────────────────────────────────────────────────────────────────────

def call_span(lines, i, limit=8):
    """The lines of the call that starts on line i, until its parentheses close (at most limit lines)."""
    depth, out = 0, []
    for line in lines[i:i + limit]:
        out.append(line)
        depth += line.count("(") - line.count(")")
        if depth <= 0:
            break
    return out


def check_skips(root, known):
    out = git(root, "ls-files")
    files = [f for f in out.stdout.splitlines() if TEST_FILE.search(f) and "node_modules/" not in f]
    found = []
    for rel in files:
        path = root / rel
        try:
            lines = path.read_text(errors="replace").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines):
            m = SKIP_RE.search(line)
            if not m:
                continue
            # The whole call, which a formatter may cut over several lines (the reason, or its block, on the
            # next one), and the line above it.
            call = call_span(lines, i)
            context = "\n".join(lines[max(0, i - 2):i] + call)
            if any_filed("\n".join(lines[max(0, i - 1):i] + call), known):
                continue
            line = " ".join(x.strip() for x in call)
            kind = m.group(1)
            if kind == "fixme":
                reason = "test.fixme is an open defect"
            elif TODO_RE.search(line):
                reason = "the skip says it waits for work"
            elif rel.endswith(".go") and re.match(r"\t\S", line):
                reason = "unconditional t.Skip"
            elif kind == "skip" and re.search(r"\b(?:test|it|describe)\.skip\(\s*[\"'`]", line):
                reason = "unconditional skip"
            elif ENV_SKIP.search(context):
                continue
            else:
                reason = "the skip matches no environment reason of ENV_SKIP"
            found.append(f"{rel}:{i + 1}: {reason}: « {line.strip()[:90]} » — name a block of the ordered list "
                         f"in the skip or the comment above it, or give the environment reason")
    return found


# ── 4 adrs ────────────────────────────────────────────────────────────────────────────────────────────────────

def check_adrs(root, known, progress, now):
    cited = {m.zfill(4) for _, l in ordered_list(progress) for m in re.findall(r"ADR\s+(\d{1,4})", l)}
    found = []
    for path in sorted((root / "docs/adr").glob("[0-9][0-9][0-9][0-9]-*.md")):
        head = path.read_text(errors="replace").split("\n---", 1)[0]
        status = re.search(r"^status:\s*(.*)$", head, re.M)
        date = re.search(r"^date:\s*(\d{4}-\d{2}-\d{2})", head, re.M)
        if not status or not status.group(1).lower().startswith("proposed") or not date:
            continue
        age = (now - datetime.fromisoformat(date.group(1)).replace(tzinfo=timezone.utc)).days
        number = path.name[:4]
        if age <= PROPOSED_DAYS or number in cited or any(filed(a or b, known) for a, b in TAG_RE.findall(status.group(1))):
            continue
        line = head[:status.start()].count("\n") + 1
        found.append(f"docs/adr/{path.name}:{line}: proposed for {age} days (> {PROPOSED_DAYS}) and cited by no "
                     f"block — cite « ADR {number} » in the block of « Cap à long terme » that decides it, "
                     f"or accept or reject it")
    return found


# ── 5 git ─────────────────────────────────────────────────────────────────────────────────────────────────────

def _porcelain_paths(text):
    for line in text.splitlines():
        p = line[3:]
        if " -> " in p:
            p = p.split(" -> ", 1)[1]
        yield p.strip().strip('"')


def check_git(root, known, progress, now):
    section = in_progress_section(progress)
    found = []
    hint = "list it in PROGRESS.md « Travaux en cours » (`name` → BLOCK-ID), or merge or delete it"
    refs = git(root, "for-each-ref", "--format=%(refname:short)\t%(committerdate:unix)", "refs/heads", "refs/remotes")
    merged = set(git(root, "branch", "-a", "--merged", "main", "--format=%(refname:short)").stdout.split())
    for line in refs.stdout.splitlines():
        name, _, stamp = line.partition("\t")
        if name in merged or name in ("main", "origin/main", "origin/HEAD", "origin") or not stamp.isdigit():
            continue
        age = (now.timestamp() - int(stamp)) / 86400
        short = name.split("/", 1)[1] if name.startswith("origin/") else name
        if age > BRANCH_DAYS and not listed_in_progress(short, section, known):
            found.append(f"branch {name}: not merged into main, last commit {age:.0f} days ago (> {BRANCH_DAYS}) — {hint}")
    listing = git(root, "worktree", "list", "--porcelain").stdout
    trees = [pathlib.Path(l[9:]) for l in listing.splitlines() if l.startswith("worktree ")]
    for tree in trees:
        status = git(tree, "status", "--porcelain")
        if status.returncode != 0 or not status.stdout.strip():
            continue
        stamps = []
        for p in _porcelain_paths(status.stdout):
            try:
                stamps.append((tree / p).lstat().st_mtime)
            except OSError:
                pass
        if not stamps:
            continue
        age = (now.timestamp() - min(stamps)) / 86400
        if age > WORKTREE_DAYS and not (listed_in_progress(tree.name, section, known)
                                        or listed_in_progress(str(tree), section, known)):
            found.append(f"worktree {tree}: uncommitted changes {age:.0f} days old (> {WORKTREE_DAYS}) — {hint}")
    main_tree = trees[0] if trees else root
    known_trees = {t.resolve() for t in trees}
    for folder in sorted((main_tree / ".claude/worktrees").glob("*")):
        if folder.is_dir() and folder.resolve() not in known_trees and not listed_in_progress(folder.name, section, known):
            found.append(f"worktree folder {folder}: unknown to `git worktree list` — {hint}")
    return found


# ── 6 ci ──────────────────────────────────────────────────────────────────────────────────────────────────────

def gh(root, *args):
    return subprocess.run(["gh", *args], cwd=root, capture_output=True, text=True, timeout=60)


def check_ci(root, now, run=gh, section=(), known=frozenset()):
    """Returns (findings, notice). A red run is filed by a « Travaux en cours » line naming its ID and a block."""
    try:
        if run(root, "auth", "status").returncode != 0:
            return [], "ci: skipped, `gh` is not authenticated"
        runs = run(root, "run", "list", "--workflow", CI_WORKFLOW, "--branch", "main", "--status", "completed",
                   "--limit", "1", "--json", "databaseId,createdAt,conclusion")
    except (OSError, subprocess.TimeoutExpired) as exc:
        return [], f"ci: skipped, `gh` unavailable ({exc})"
    if runs.returncode != 0:
        return [f"ci: `gh run list` failed: {runs.stderr.strip()[:200]}"], None
    rows = json.loads(runs.stdout or "[]")
    if not rows:
        return [f"ci: no completed {CI_WORKFLOW} run on main — run it (`gh workflow run {CI_WORKFLOW} --ref main`)"], None
    last = rows[0]
    rid = last["databaseId"]
    found = []
    age = (now - datetime.fromisoformat(last["createdAt"].replace("Z", "+00:00"))).total_seconds() / 86400
    if age > CI_DAYS:
        found.append(f"ci: latest completed {CI_WORKFLOW} run on main is {rid}, {age:.0f} days old (> {CI_DAYS}) — "
                     f"push main or run the workflow")
    view = run(root, "run", "view", str(rid), "--json", "jobs")
    if view.returncode != 0:
        return found + [f"ci: `gh run view {rid}` failed: {view.stderr.strip()[:200]}"], None
    bad = [f"{j['name']} ({j.get('status')}/{j.get('conclusion') or '-'})" for j in json.loads(view.stdout)["jobs"]
           if j.get("status") != "completed" or j.get("conclusion") != "success"]
    if bad:
        red = (f"ci: run {rid} on main ({last.get('conclusion')}): jobs not green or not completed: "
               f"{', '.join(bad)} — fix main first (« Cap à long terme », point 1)")
        rid_re = re.compile(rf"(?<!\d){rid}(?!\d)")
        filing = next((l.strip() for l in section if rid_re.search(l) and any_filed(l, known)), None)
        if filing:
            return found, f"{red}; filed in « Travaux en cours »: « {filing[:120]} »"
        found.append(red)
    return found, None


# ── 7 deferred ────────────────────────────────────────────────────────────────────────────────────────────────

def check_deferred(root):
    spec = importlib.util.spec_from_file_location("inventory_gate", SCRIPTS / "gate.py")
    gate = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gate)
    stream = io.StringIO()
    code = gate.deferred_report(root, stream)
    lines = stream.getvalue().splitlines()
    found = [f"gates/deferred.jsonl: {l} — run `mise run gate -- <lot>` alone to close it"
             for l in lines if l.endswith("overdue") or l.startswith("unreadable")]
    if code and not found:
        found = [f"gates/deferred.jsonl: {lines[-1] if lines else 'gate.py --deferred failed'}"]
    return found


# ── driver ────────────────────────────────────────────────────────────────────────────────────────────────────

def inventory(root, now=None, gh_run=None):
    """Returns ({check: [finding]}, [notice])."""
    now = now or datetime.now(timezone.utc)
    progress_path = root / "PROGRESS.md"
    progress = progress_path.read_text() if progress_path.exists() else ""
    known = block_ids(progress)
    notices = []
    if not known:
        notices.append("PROGRESS.md: no numbered list under « Cap à long terme »: no block is known")
    ci, notice = check_ci(root, now, gh_run or gh, in_progress_section(progress), known)
    if notice:
        notices.append(notice)
    results = {
        "requirements": check_requirements(root, known),
        "residuals": check_residuals(root, known),
        "skips": check_skips(root, known),
        "adrs": check_adrs(root, known, progress, now),
        "git": check_git(root, known, progress, now),
        "ci": ci,
        "deferred": check_deferred(root),
    }
    return results, notices


def render(results, notices, stream):
    total = sum(len(v) for v in results.values())
    for name in CHECKS:
        items = results[name]
        stream.write(f"== {name}: {len(items)} ==\n")
        for item in items:
            stream.write(f"  {item}\n")
    for n in notices:
        stream.write(f"notice: {n}\n")
    stream.write(f"inventory: {total} unfiled item(s) — "
                 + ", ".join(f"{k} {len(results[k])}" for k in CHECKS) + "\n")
    return total


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--report", action="store_true", help="print the list and exit 0")
    parser.add_argument("--root", default=str(SCRIPTS.parent))
    args = parser.parse_args(argv)
    results, notices = inventory(pathlib.Path(args.root).resolve())
    total = render(results, notices, sys.stdout)
    return 0 if args.report or total == 0 else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
