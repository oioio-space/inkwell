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


class E2EBuildTest(unittest.TestCase):
    def test_builds_ignore_parent_repositories(self):
        # The copied tree has no .git; a repository in a parent directory must not reach the build (W77, MN1/MN2).
        run = runner(1)
        orig = mutants.sh
        mutants.sh = run
        try:
            import tempfile
            with tempfile.TemporaryDirectory() as d:
                mutants.run_e2e({"id": "MX", "edit": [{"file": "internal/x.go"}], "scenario": 9}, pathlib.Path(d))
        finally:
            mutants.sh = orig
        builds = [c for c in run.calls if "build" in c and "go" in c]
        self.assertTrue(builds)
        for cmd in builds:
            self.assertIn("-buildvcs=false", cmd)


def entry(i, block, before="a\n", after="b\n", **kw):
    return {"id": i, "block": block, "kind": "go", "pkg": "./x", "test": "TestX",
            "edit": [{"file": "f.go", "before": before, "after": after}], **kw}


class CatalogueCheckTest(unittest.TestCase):
    """The whole-catalogue refusals of `mutants.py --check` (MT1)."""

    def tree(self, files):
        import tempfile
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        root = pathlib.Path(d.name)
        for name, text in files.items():
            (root / name).parent.mkdir(parents=True, exist_ok=True)
            (root / name).write_text(text)
        return root

    def test_duplicate_id_within_a_block_is_refused(self):
        self.assertTrue(mutants.duplicate_ids([entry("M1", "A"), entry("M1", "A")]))

    def test_same_id_in_two_blocks_is_accepted(self):
        # The runner selects by block first (main, gates' `m.get("block") == B`): ids are scoped by their block.
        self.assertEqual(mutants.duplicate_ids([entry("M1", "A"), entry("M1", "B")]), [])

    def test_anchor_matching_nothing_is_refused(self):
        root = self.tree({"f.go": "x\n"})
        self.assertIn("found 0 times", mutants.anchor_problem(entry("M1", "A"), root))
        self.assertIsNone(mutants.anchor_problem(entry("M1", "A", before="x\n"), root))

    def test_retired_entry_is_not_anchored_and_needs_a_reason(self):
        root = self.tree({"f.go": "x\n"})
        m = entry("M1", "A", retired="the code it mutated was removed by B")
        self.assertIsNone(mutants.anchor_problem(m, root))
        self.assertEqual(mutants.validate([m]), [])
        self.assertTrue(mutants.validate([{**m, "retired": ""}]))
        self.assertTrue(mutants.validate([{**m, "retired": True}]))

    def test_selection_leaves_retired_out_and_refuses_one_named(self):
        ms = [entry("M1", "A"), entry("M2", "A", retired="gone"), entry("M2", "B")]
        chosen, refused = mutants.select(ms, "A", set())
        self.assertEqual([m["id"] for m in chosen], ["M1"])
        self.assertEqual(refused, [])
        chosen, refused = mutants.select(ms, "A", {"M2"})
        self.assertEqual(chosen, [])
        self.assertTrue(refused and "gone" in refused[0])

    def gates(self, **gates):
        files = {}
        for lot, command in gates.items():
            files[f"gates/{lot}.toml"] = "[[criteria]]\nid = \"mutants\"\ncommand = " + repr_toml(command) + "\n"
        return self.tree(files)

    def test_entry_named_by_a_gate_and_missing_is_refused(self):
        # The a541db5 loss: MS1..MS22 dropped while gates/SB2a1.toml still names them.
        root = self.gates(SB2a1="python3 -c \"need = [f'MS{i}' for i in range(1, 4)]\" && ./scripts/heavy.sh python3 -I scripts/netsim/mutants.py SB2a1")
        (root / "gates/SB2a1.json").write_text("{}")
        bad, _ = mutants.gate_problems([entry("MS1", "SB2a1"), entry("MS3", "SB2a1")], root)
        self.assertEqual(len(bad), 1)
        self.assertIn("MS2", bad[0])
        self.assertEqual(mutants.gate_problems([entry(f"MS{i}", "SB2a1") for i in (1, 2, 3)], root)[0], [])

    def test_whole_block_lost_is_refused(self):
        root = self.gates(RL1="mise run netsim:mutants -- RL1")
        (root / "gates/RL1.json").write_text("{}")
        self.assertTrue(mutants.gate_problems([entry("M1", "NI0")], root)[0])

    def test_explicit_ids_and_id_lists_are_read(self):
        root = self.gates(
            X="timeout 30 python3 -I scripts/netsim/mutants.py --check X && heavy.sh python3 -I scripts/netsim/mutants.py X MA1 MA2",
            Y='for m in MB1 MB2; do grep -q "id = \\"$m\\"" scripts/netsim/mutants.toml || exit 1; done && mise run netsim:mutants -- Y',
            Z='for i in $(seq -w 1 10); do grep -q "id = \\"Z-$i\\"" scripts/netsim/mutants.toml || exit 1; done && mise run netsim:mutants -- Z')
        for lot in "XYZ":
            (root / f"gates/{lot}.json").write_text("{}")
        ms = [entry("MA1", "X"), entry("MB1", "Y")] + [entry(f"Z-{i:02d}", "Z") for i in range(1, 11) if i != 2]
        bad, _ = mutants.gate_problems(ms, root)
        self.assertEqual(sorted(b.split(":")[0] for b in bad), ["gates/X.toml", "gates/Y.toml", "gates/Z.toml"])
        self.assertTrue(any("MA2" in b for b in bad) and any("MB2" in b for b in bad) and any("Z-02" in b for b in bad))

    def test_id_of_another_block_does_not_count(self):
        root = self.gates(X="mutants.py X MA1")
        (root / "gates/X.json").write_text("{}")
        self.assertTrue(mutants.gate_problems([entry("MA1", "W")], root)[0])

    def test_retired_named_by_a_gate_is_a_note_not_a_refusal(self):
        root = self.gates(X="mutants.py X MA1")
        (root / "gates/X.json").write_text("{}")
        bad, notes = mutants.gate_problems([entry("MA1", "X", retired="removed by B")], root)
        self.assertEqual(bad, [])
        self.assertTrue(any("MA1" in n and "removed by B" in n for n in notes))

    def test_gate_before_its_entries_is_not_checked(self):
        # Step 1 freezes the gate before step 2 writes the entries: no result yet and no entry of its block.
        root = self.gates(NEW="mutants.py NEW MA1 MA2")
        self.assertEqual(mutants.gate_problems([], root)[0], [])
        # Once one entry exists, the others it names are due.
        self.assertTrue(mutants.gate_problems([entry("MA1", "NEW")], root)[0])


def repr_toml(s):
    return "\'\'\'" + s + "\'\'\'"


if __name__ == "__main__":
    unittest.main()
