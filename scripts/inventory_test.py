#!/usr/bin/env python3
"""Tests for scripts/inventory.py: each check reports an unfiled open item and accepts a filed one."""
import io
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest
import unittest.mock
from datetime import datetime, timedelta, timezone

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import inventory  # noqa: E402

NOW = datetime(2026, 11, 17, 12, 0, tzinfo=timezone.utc)
PROGRESS = """# inkwell

**Cap à long terme (écrit le 2026-11-15) :**

Ordre :
1. **`main` vert** d'abord.
2. **Finir SB2a1**, puis **EX0** (ADR 0044).
3. **UX1** et HB1 ; W74.

Les décisions...
"""
IN_PROGRESS = """
**Travaux en cours :**
- `old-feature` → UX1
- worktree `wt-filed` → EX0

**Autre**
"""


def write(root, rel, text):
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    return path


class Base(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = pathlib.Path(tmp.name)
        write(self.root, "PROGRESS.md", PROGRESS)
        self.known = inventory.block_ids(PROGRESS)

    def git(self, *args, cwd=None, date=None):
        env = dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@t", GIT_COMMITTER_NAME="t",
                   GIT_COMMITTER_EMAIL="t@t", GIT_CONFIG_GLOBAL="/dev/null", GIT_CONFIG_SYSTEM="/dev/null")
        if date:
            env.update(GIT_AUTHOR_DATE=date, GIT_COMMITTER_DATE=date)
        return subprocess.run(["git", *args], cwd=cwd or self.root, env=env, check=True, capture_output=True, text=True)


class BlockIdsTest(Base):
    def test_ids_come_from_the_ordered_list_only(self):
        self.assertTrue({"SB2a1", "EX0", "UX1", "HB1", "W74"} <= self.known)
        self.assertNotIn("ADR", "".join(self.known))

    def test_a_refinement_of_a_listed_block_is_filed(self):
        self.assertTrue(inventory.filed("HB1a", self.known))
        self.assertTrue(inventory.filed("SB2a1", self.known))
        self.assertFalse(inventory.filed("SB2", self.known))
        self.assertFalse(inventory.filed("W7", self.known))


class RequirementsTest(Base):
    def check(self, rows):
        write(self.root, "docs/exigences.md", "| # | Date | Demande | Où | État |\n|---|---|---|---|---|\n" + rows)
        return inventory.check_requirements(self.root, self.known)

    def test_open_row_without_a_listed_block_is_reported(self):
        found = self.check("| U15 | 2026-10-25 | Inclure | B7 LNK-* | ⏳ |\n")
        self.assertEqual(len(found), 1)
        self.assertIn("docs/exigences.md:3: U15", found[0])

    def test_done_row_and_filed_rows_pass(self):
        self.assertEqual(self.check("| U1 | d | x | A2 | ✅ |\n"
                                    "| U9 | d | x | bloc UX1 | 🔨 |\n"
                                    "| W74 | d | boutons | — | ⏳ |\n"
                                    "| U2 | d | x | — | 🧪 reste : HB1b |\n"), [])


class ResidualsTest(Base):
    def check(self, text, rel="gates/X/SIMPLIFY.md"):
        write(self.root, rel, text)
        return inventory.check_residuals(self.root, self.known)

    def test_untagged_residual_is_reported_with_its_line(self):
        found = self.check("# X\n\nIntro.\n\n- The writer has no deadline: follow-up.\n")
        self.assertEqual(len(found), 1)
        self.assertIn("gates/X/SIMPLIFY.md:5:", found[0])

    def test_tags_accept_a_listed_block(self):
        self.assertEqual(self.check("- residual R-1 → UX1\n\n- deferred [HB1a]\n\n- residual [done]\n"), [])

    def test_obsolete_needs_a_reason(self):
        self.assertEqual(self.check("- residual R-1 [obsolete: removed at the user's request]\n"), [])
        self.assertEqual(len(self.check("- residual R-1 [obsolete]\n")), 1)

    def test_tag_of_an_unlisted_block_is_reported(self):
        self.assertEqual(len(self.check("- residual R-1 → ZZ9\n")), 1)

    def test_each_item_of_a_residuals_section_needs_a_tag(self):
        text = "# X\n\n## Residuals\n\n- first one\n- second one → EX0\n\n## Other\n\n- unrelated\n"
        found = self.check(text)
        self.assertEqual(len(found), 1)
        self.assertIn(":5:", found[0])

    def test_a_label_opens_the_list_that_follows(self):
        found = self.check("Residuals:\n- one\n- two [UX1]\n\nPlain prose.\n")
        self.assertEqual([":2:" in f for f in found], [True])

    def test_accepted_residual_needs_an_existing_adr(self):
        write(self.root, "docs/adr/0037-single-instance.md", "---\nstatus: accepted\n---\n")
        self.assertEqual(self.check("- accepted residual [accepted ADR 0037]\n"), [])
        self.assertEqual(len(self.check("- accepted residual [accepted ADR 0099]\n")), 1)

    def test_title_and_code_fences_are_not_items(self):
        self.assertEqual(self.check("# Plan deferred behind GP\n\n```\nresidual\n```\n\nNothing here.\n",
                                    rel="docs/plans/p.md"), [])

    def test_later_alone_is_not_a_marker(self):
        self.assertEqual(self.check("- a later request fails\n"), [])
        self.assertEqual(len(self.check("- to fix later\n")), 1)


class SkipsTest(Base):
    def check(self, rel, text):
        write(self.root, rel, text)
        self.git("init", "-q", "-b", "main")
        self.git("add", "-A")
        return inventory.check_skips(self.root, self.known)

    def test_fixme_without_block_is_reported(self):
        found = self.check("e2e/a.spec.ts", 'test.fixme("SRC-11 opens the file", async () => {});\n')
        self.assertEqual(len(found), 1)
        self.assertIn("e2e/a.spec.ts:1: test.fixme", found[0])

    def test_fixme_naming_a_block_passes(self):
        self.assertEqual(self.check("e2e/a.spec.ts", '// UX1\ntest.fixme("SRC-11", async () => {});\n'), [])

    def test_todo_and_unconditional_go_skips_are_reported(self):
        found = self.check("x_test.go", 'func TestA(t *testing.T) {\n\tt.Skip("waiting")\n}\n'
                                        'func TestB(t *testing.T) {\n\tif x {\n\t\tt.Skip("TODO ADR 0021")\n\t}\n}\n')
        self.assertEqual(len(found), 2)

    def test_environment_skips_are_allowed(self):
        self.assertEqual(self.check("x_test.go", 'func TestA(t *testing.T) {\n\tif err != nil {\n'
                                                 '\t\tt.Skipf("mDNS is not available here: %v", err)\n\t}\n'
                                                 '\tif testing.Short() {\n\t\tt.Skip("writes 40 MB")\n\t}\n}\n'), [])

    def test_unknown_conditional_skip_is_reported(self):
        self.assertEqual(len(self.check("x_test.go", 'func T(t *testing.T) {\n\tif x {\n\t\tt.Skip("flaky")\n\t}\n}\n')), 1)


class AdrsTest(Base):
    def adr(self, name, status, date):
        write(self.root, f"docs/adr/{name}", f"---\nstatus: {status}\ndate: {date}\n---\n\n# T\n")

    def test_old_proposed_adr_not_cited_is_reported(self):
        self.adr("0021-suspension.md", "proposed", "2026-10-25")
        found = inventory.check_adrs(self.root, self.known, PROGRESS, NOW)
        self.assertEqual(len(found), 1)
        self.assertIn("docs/adr/0021-suspension.md:2: proposed for 23 days", found[0])

    def test_recent_cited_tagged_or_accepted_adrs_pass(self):
        self.adr("0043-recent.md", "proposed", "2026-11-10")
        self.adr("0044-cited.md", "proposed", "2026-10-01")
        self.adr("0029-tagged.md", "proposed → UX1", "2026-10-01")
        self.adr("0001-accepted.md", "accepted", "2026-01-01")
        self.assertEqual(inventory.check_adrs(self.root, self.known, PROGRESS, NOW), [])


class GitTest(Base):
    OLD = "2026-11-01T12:00:00+00:00"

    def setUp(self):
        super().setUp()
        self.git("init", "-q", "-b", "main")
        write(self.root, "f", "1")
        self.git("add", "f")
        self.git("commit", "-qm", "base", date=self.OLD)
        for name in ("old-feature", "stale-feature"):
            self.git("checkout", "-qb", name, "main")
            write(self.root, name, "x")
            self.git("add", name)
            self.git("commit", "-qm", name, date=self.OLD)
        self.git("checkout", "-q", "main")

    def findings(self, progress):
        return inventory.check_git(self.root, inventory.block_ids(progress), progress, NOW)

    def test_old_unmerged_branch_must_be_listed(self):
        found = self.findings(PROGRESS + IN_PROGRESS)
        self.assertEqual([f.split(":")[0] for f in found], ["branch stale-feature"])

    def test_without_the_section_every_old_branch_fails(self):
        self.assertEqual(len(self.findings(PROGRESS)), 2)

    def test_dirty_worktree_older_than_three_days_must_be_listed(self):
        trees = tempfile.TemporaryDirectory()
        self.addCleanup(trees.cleanup)
        for name in ("wt-filed", "wt-stale"):
            tree = pathlib.Path(trees.name) / name
            self.git("worktree", "add", "-q", "--detach", str(tree), "main")
            path = write(tree, "f", "changed")
            stamp = (NOW - timedelta(days=5)).timestamp()
            os.utime(path, (stamp, stamp))
        progress = PROGRESS + IN_PROGRESS.replace("`old-feature`", "`old-feature` `stale-feature`")
        found = self.findings(progress)
        self.assertEqual(len(found), 1)
        self.assertIn("wt-stale", found[0])

    def test_unknown_folder_in_claude_worktrees_is_reported(self):
        (self.root / ".claude/worktrees/ghost").mkdir(parents=True)
        self.assertTrue(any("ghost" in f for f in self.findings(PROGRESS + IN_PROGRESS)))


class FakeGh:
    def __init__(self, runs, jobs, authed=True):
        self.runs, self.jobs, self.authed = runs, jobs, authed

    def __call__(self, root, *args):
        if args[:2] == ("auth", "status"):
            return subprocess.CompletedProcess(args, 0 if self.authed else 1, "", "")
        if args[:2] == ("run", "list"):
            return subprocess.CompletedProcess(args, 0, json.dumps(self.runs), "")
        return subprocess.CompletedProcess(args, 0, json.dumps({"jobs": self.jobs}), "")


class CiTest(unittest.TestCase):
    RUN = [{"databaseId": 7, "createdAt": "2026-11-16T10:00:00Z", "conclusion": "success"}]
    GREEN = [{"name": "ci", "status": "completed", "conclusion": "success"}]

    def test_recent_green_run_passes(self):
        self.assertEqual(inventory.check_ci(None, NOW, FakeGh(self.RUN, self.GREEN)), ([], None))

    def test_red_or_unfinished_job_is_named_with_the_run(self):
        jobs = self.GREEN + [{"name": "netsim", "status": "completed", "conclusion": "failure"},
                             {"name": "arm64", "status": "in_progress", "conclusion": None}]
        found, _ = inventory.check_ci(None, NOW, FakeGh(self.RUN, jobs))
        self.assertEqual(len(found), 1)
        self.assertIn("run 7", found[0])
        self.assertIn("netsim (completed/failure), arm64 (in_progress/-)", found[0])

    def test_red_run_is_filed_only_by_its_run_id_and_a_listed_block(self):
        red = [{"name": "ci", "status": "completed", "conclusion": "failure"}]
        known = inventory.block_ids(PROGRESS)
        filed = ["- `ci.yml` run 7 on main → SB2a1, UX1"]
        found, notice = inventory.check_ci(None, NOW, FakeGh(self.RUN, red), filed, known)
        self.assertEqual(found, [])
        self.assertIn("run 7", notice)
        self.assertIn("filed", notice)
        for section in (["- `ci.yml` run 77 → SB2a1"], ["- `ci.yml` run 7 → ZZ9"]):
            self.assertEqual(len(inventory.check_ci(None, NOW, FakeGh(self.RUN, red), section, known)[0]), 1)

    def test_old_run_fails(self):
        old = [dict(self.RUN[0], createdAt="2026-11-01T10:00:00Z")]
        found, _ = inventory.check_ci(None, NOW, FakeGh(old, self.GREEN))
        self.assertIn("16 days old", found[0])

    def test_no_run_fails_and_unauthenticated_gh_is_a_notice(self):
        self.assertEqual(len(inventory.check_ci(None, NOW, FakeGh([], []))[0]), 1)
        found, notice = inventory.check_ci(None, NOW, FakeGh([], [], authed=False))
        self.assertEqual(found, [])
        self.assertIn("not authenticated", notice)


class DeferredTest(Base):
    def test_overdue_obligation_is_reported_and_recent_one_is_not(self):
        old = (datetime.now(timezone.utc) - timedelta(days=9)).isoformat()
        new = datetime.now(timezone.utc).isoformat()
        rows = [dict(commit="c", criterion="regression", date=old, from_gate="SB1b", lot="L1"),
                dict(commit="c", criterion="regression", date=new, from_gate="SB1b", lot="L2")]
        write(self.root, "gates/deferred.jsonl", "".join(json.dumps(r) + "\n" for r in rows))
        found = inventory.check_deferred(self.root)
        self.assertEqual(len(found), 1)
        self.assertIn("open L1", found[0])


class MainTest(Base):
    def test_report_mode_exits_zero_and_failing_mode_does_not(self):
        write(self.root, "docs/exigences.md", "| # | D | x | Où | État |\n|---|---|---|---|---|\n| U15 | d | x | B7 | ⏳ |\n")
        self.git("init", "-q", "-b", "main")
        out = io.StringIO()
        results, notices = inventory.inventory(self.root, NOW, FakeGh([], [], authed=False))
        total = inventory.render(results, notices, out)
        self.assertEqual(total, 1)
        self.assertIn("== requirements: 1 ==", out.getvalue())
        offline = FakeGh([], [], authed=False)
        original, inventory.gh = inventory.gh, offline
        self.addCleanup(setattr, inventory, "gh", original)
        with unittest.mock.patch("sys.stdout", io.StringIO()):
            codes = (inventory.main(["--root", str(self.root)]), inventory.main(["--report", "--root", str(self.root)]))
        self.assertEqual(codes, (1, 0))


if __name__ == "__main__":
    unittest.main()
