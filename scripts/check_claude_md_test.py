#!/usr/bin/env python3
"""Tests for scripts/check_claude_md.py: each kind of stale reference is reported."""
import json
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import check_claude_md  # noqa: E402


class CheckClaudeMdTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = pathlib.Path(tmp.name)
        (self.root / ".claude/agents").mkdir(parents=True)
        (self.root / ".claude/agents/scribe.md").write_text("")
        (self.root / ".claude/skills/go-style-guide").mkdir(parents=True)
        (self.root / ".claude/hooks").mkdir()
        (self.root / ".claude/hooks/guard.sh").write_text("")
        (self.root / "internal/blob").mkdir(parents=True)
        self.hooks({})

    def hooks(self, hooks):
        (self.root / ".claude/settings.json").write_text(json.dumps({"hooks": hooks}))

    def check(self, text, tasks=("lint", "test:py", "e2e:run")):
        return check_claude_md.problems(self.root, text, set(tasks))

    def test_consistent_file_has_no_problem(self):
        text = ("`mise run lint`, `mise run e2e*`, `.claude/hooks/guard.sh`, agent `scribe`,\n"
                "skills `go-style-guide`, `simplify` et `superpowers:brainstorming`\n"
                "HOT_PATHS: internal/blob\n")
        self.assertEqual(self.check(text), [])

    def test_undefined_task_is_reported(self):
        self.assertIn("mise task `gone`", self.check("mise run gone")[0])

    def test_missing_path_is_reported_unless_pending(self):
        self.assertIn("path `docs/gone.md`", self.check("voir `docs/gone.md`")[0])
        self.assertEqual(self.check("`protocol/libp2p` (étape 4)"), [])
        self.assertEqual(self.check("`protocol/libp2p` (étape 4 de l'ADR 0034)"), [])
        self.assertEqual(self.check("`gates/<bloc>.toml`"), [])

    def test_missing_agent_and_skill_are_reported(self):
        self.assertIn("agent `ghost`", self.check("agent `ghost`")[0])
        self.assertIn("agent `ghost`", self.check("| Docs | `ghost` | Haiku |")[0])
        self.assertIn("skill `nope`", self.check("skill `nope`")[0])

    def test_missing_hot_path_is_reported(self):
        self.assertIn("internal/none", self.check("HOT_PATHS: internal/blob internal/none")[0])

    def test_unnamed_registered_hook_is_reported(self):
        self.hooks({"PreToolUse": [{"hooks": [{"command": "$D/.claude/hooks/guard.sh"}]}]})
        self.assertIn("guard.sh", self.check("rien")[0])
        self.assertEqual(self.check("`guard.sh`"), [])


if __name__ == "__main__":
    unittest.main()
