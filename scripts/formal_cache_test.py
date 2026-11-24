"""Tests of the formal verdict cache (block CI3b, scripts/formal_cache.py, docs/plans/2026-11-24-ci3b-formal-cache.md).

What they prove: the key changes when any input changes (a transitively EXTENDed module included); a failure is never
stored; INKWELL_FORMAL_NO_CACHE=1 and scripts/gate.py bypass the cache; a reused run prints the same ✓ line followed by
« reused <key prefix>: <name> »; an expired verdict is rerun. tla-check.sh is exercised end to end on a copy whose
run list and jar pin are swapped for a fixture and whose `java` is a fake that counts its calls (the real run() is
kept byte for byte).
"""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

SCRIPTS = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS))
import alloy_check  # noqa: E402
import formal_cache  # noqa: E402


@contextlib.contextmanager
def cache_env(**extra):
    with tempfile.TemporaryDirectory() as d, mock.patch.dict(os.environ, {"INKWELL_FORMAL_CACHE": d, **extra}):
        if "INKWELL_FORMAL_NO_CACHE" not in extra:
            os.environ.pop("INKWELL_FORMAL_NO_CACHE", None)  # restored by patch.dict on exit
        yield pathlib.Path(d)


def write(root, rel, text):
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)


class Key(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.tmp.name)
        write(self.root, "C.tla", "---- MODULE C ----\nEXTENDS Naturals\nX == 1\n====\n")
        write(self.root, "B.tla", "---- MODULE B ----\nEXTENDS C, FiniteSets\n====\n")
        write(self.root, "A.tla", "---- MODULE A ----\n\\* EXTENDS Nowhere (a comment)\n(* INSTANCE Gone *)\nEXTENDS B\n====\n")
        write(self.root, "mutants/AM.tla", "---- MODULE AM ----\nEXTENDS A\n====\n")
        write(self.root, "A.cfg", "INIT Init\n")

    def tearDown(self):
        self.tmp.cleanup()

    def key(self, module="A", jar="j", java="17", runner="r", args=("kind=sound",)):
        files = formal_cache.tla_closure(self.root, module)
        files["cfg:A.cfg"] = formal_cache.sha256((self.root / "A.cfg").read_bytes())
        return formal_cache.key("tlc", files, jar, java, runner, list(args))

    def test_closure_is_transitive_and_ignores_comments(self):
        self.assertEqual(set(formal_cache.tla_closure(self.root, "mutants/AM")),
                         {"mutants/AM.tla", "A.tla", "B.tla", "C.tla"})

    def test_key_changes_with_every_input(self):
        base = self.key()
        self.assertEqual(base, self.key())
        variants = [self.key(jar="j2"), self.key(java="21"), self.key(runner="r2"), self.key(args=("kind=mutant",))]
        write(self.root, "A.cfg", "INIT Init2\n")
        variants.append(self.key())
        write(self.root, "A.cfg", "INIT Init\n")
        write(self.root, "C.tla", "---- MODULE C ----\nEXTENDS Naturals\nX == 2\n====\n")  # two EXTENDS away
        variants.append(self.key())
        self.assertEqual(len({base, *variants}), 1 + len(variants))

    def test_unknown_module_is_not_cacheable(self):
        write(self.root, "B.tla", "---- MODULE B ----\nEXTENDS Missing\n====\n")
        with self.assertRaises(formal_cache.Unresolved):
            formal_cache.tla_closure(self.root, "A")

    def test_cli_exit_codes(self):
        argv = ["tla-lookup", "--root", str(self.root), "--module", "A", "--cfg", "A.cfg", "--jar-sha", "j",
                "--java", "17", "--runner-sha", "r"]
        with cache_env(), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(formal_cache.main(argv), 1)
            write(self.root, "B.tla", "---- MODULE B ----\nEXTENDS Missing\n====\n")
            with contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(formal_cache.main(argv), 2)


class Store(unittest.TestCase):
    def test_roundtrip_expiry_and_refusals(self):
        k = "a" * 64
        with cache_env(INKWELL_FORMAL_CACHE_TTL_DAYS="1") as d:
            self.assertIsNone(formal_cache.get(k))
            formal_cache.put(k, "X", ["✓ X: no error"])
            self.assertEqual(formal_cache.get(k), ["✓ X: no error"])
            entry = json.loads((d / f"{k}.json").read_text())
            entry["stored"] = time.time() - 2 * 86400
            (d / f"{k}.json").write_text(json.dumps(entry))
            self.assertIsNone(formal_cache.get(k), "an expired verdict must be rerun")
            with self.assertRaises(ValueError):
                formal_cache.put("../x", "X", ["✓"])
            with self.assertRaises(ValueError):
                formal_cache.put(k, "X", [])

    def test_disabled(self):
        for value, want in (("", False), ("0", False), ("1", True)):
            with mock.patch.dict(os.environ, {"INKWELL_FORMAL_NO_CACHE": value}):
                self.assertIs(formal_cache.disabled(), want)


FAKE_JAVA = r"""#!/usr/bin/env bash
if [[ "$1" == -version ]]; then echo 'openjdk version "17.0.0-fake"' >&2; exit 0; fi
echo "$*" >>"$FAKE_CALLS"
module="${@: -1}"
if [[ -n "${FAKE_FAIL:-}" ]]; then echo "Error: something"; exit 1; fi
if [[ "$module" == mutants/* ]]; then echo "Error: Invariant Safe is violated."; exit 12; fi
echo "Model checking completed. No error has been found."
echo "complete state space; 0 states left on queue"
"""


class TlaCheck(unittest.TestCase):
    """tla-check.sh end to end, on a fixture tree with a fake java."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        t = pathlib.Path(self.tmp.name)
        self.root, self.calls = t / "repo", t / "calls"
        tla = self.root / "docs/protocol/tla"
        write(tla, "Base.tla", "---- MODULE Base ----\nEXTENDS Naturals\nX == 1\n====\n")
        write(tla, "M.tla", "---- MODULE M ----\nEXTENDS Base\n====\n")
        write(tla, "M.cfg", "INIT Init\n")
        write(tla, "mutants/MMutants.tla", "---- MODULE MMutants ----\nEXTENDS M\n====\n")
        write(tla, "mutants/MBad.cfg", "INIT Init\n")
        source = (SCRIPTS / "tla-check.sh").read_text()
        jar = b"fake jar"
        source = re.sub(r'^jar_sha="[0-9a-f]{64}"$', f'jar_sha="{hashlib.sha256(jar).hexdigest()}"', source,
                        count=1, flags=re.M)
        self.head, sep, _ = source.partition("\n# Sound models (README tables)")
        self.assertTrue(sep, "tla-check.sh layout changed: no run list marker")
        self.runs = ["run MSound M.cfg M sound", "run MBad mutants/MBad.cfg mutants/MMutants mutant Safe"]
        self.write_script()
        write(self.root, "scripts/heavy.sh", '#!/usr/bin/env bash\nexec "$@"\n')
        write(self.root, "scripts/formal_cache.py", (SCRIPTS / "formal_cache.py").read_text())
        bindir = t / "bin"
        write(bindir, "java", FAKE_JAVA)
        for f in (self.root / "scripts/heavy.sh", bindir / "java"):
            f.chmod(0o755)
        cache = t / "tlacache"
        cache.mkdir()
        (cache / "tla2tools.jar").write_bytes(jar)
        self.env = {k: v for k, v in os.environ.items() if not k.startswith(("INKWELL_", "TLA_"))}
        self.env.update(PATH=f"{bindir}:{os.environ['PATH']}", INKWELL_TLA_CACHE=str(cache),
                        INKWELL_FORMAL_CACHE=str(t / "verdicts"), FAKE_CALLS=str(self.calls))

    def tearDown(self):
        self.tmp.cleanup()

    def write_script(self, head=None):
        write(self.root, "scripts/tla-check.sh", (head or self.head) + "\n" + "\n".join(self.runs) + '\nexit "$fail"\n')
        (self.root / "scripts/tla-check.sh").chmod(0o755)

    def reused(self, out):
        return set(re.findall(r"(?m)^reused [0-9a-f]{12}: (\w+)$", out))

    def test_helper_change_changes_every_key(self):
        self.check()
        helper = "die() { printf 'tla: %s\\n' \"$*\" >&2; exit 1; }"
        self.assertIn(helper, self.head, "the fixture edits die(), a helper run() calls")
        self.write_script(self.head.replace(helper, helper.replace("tla: ", "tla! ")))
        code, out = self.check()
        self.assertEqual((code, self.ncalls(), self.reused(out)), (0, 4, set()), out)

    def test_added_run_keeps_the_other_keys(self):
        self.check()
        write(self.root / "docs/protocol/tla", "M2.cfg", "INIT Init2\n")
        self.runs.append("run MSound2 M2.cfg M sound")
        self.write_script()
        code, out = self.check()
        self.assertEqual((code, self.ncalls(), self.reused(out)), (0, 3, {"MSound", "MBad"}), out)

    def check(self, **extra):
        p = subprocess.run([str(self.root / "scripts/tla-check.sh")], env=dict(self.env, **extra),
                           capture_output=True, text=True, timeout=60)
        return p.returncode, p.stdout

    def ncalls(self):
        return len(self.calls.read_text().splitlines()) if self.calls.exists() else 0

    def test_second_run_reuses_with_the_same_lines(self):
        code, first = self.check()
        self.assertEqual((code, self.ncalls()), (0, 2), first)
        self.assertNotIn("reused", first)
        code, second = self.check()
        self.assertEqual((code, self.ncalls()), (0, 2), "a reused verdict must not run TLC")
        ticks = [line for line in second.splitlines() if line.startswith("✓")]
        self.assertEqual(ticks, [line for line in first.splitlines() if line.startswith("✓")])
        self.assertRegex(second, r"(?m)^reused [0-9a-f]{12}: MSound$")
        self.assertRegex(second, r"(?m)^reused [0-9a-f]{12}: MBad$")

    def test_extended_module_change_reruns_both(self):
        self.check()
        write(self.root / "docs/protocol/tla", "Base.tla", "---- MODULE Base ----\nEXTENDS Naturals\nX == 2\n====\n")
        code, out = self.check()
        self.assertEqual((code, self.ncalls()), (0, 4), out)
        self.assertNotIn("reused", out)

    def test_failure_is_never_stored(self):
        code, _ = self.check(FAKE_FAIL="1")
        self.assertNotEqual(code, 0)
        code, out = self.check()
        self.assertEqual((code, self.ncalls()), (0, 4), "after a failure both runs must run again")
        self.assertNotIn("reused", out)

    def test_bypass_runs_everything(self):
        self.check()
        code, out = self.check(INKWELL_FORMAL_NO_CACHE="1")
        self.assertEqual((code, self.ncalls()), (0, 4))
        self.assertNotIn("reused", out)


def load_gate():
    spec = importlib.util.spec_from_file_location("gate_under_test", SCRIPTS / "gate.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class GateBypass(unittest.TestCase):
    def test_every_gate_criterion_runs_uncached(self):
        env = load_gate().criterion_env(pathlib.Path("/x"), "tla", "RF1")
        self.assertEqual(env["INKWELL_FORMAL_NO_CACHE"], "1")
        with mock.patch.dict(os.environ, env):
            self.assertTrue(formal_cache.disabled())


def fake_alloy_run(model, scratch):
    """Stands in for alloy_check.run: every check holds, and a mutant (a file outside MODELS) is caught."""
    fake_alloy_run.calls += 1
    if "FAIL" in model.read_text():
        return {"A": ("check", True)}
    return {"A": ("check", model.parent != alloy_check.MODELS)}


class AlloyRunner(unittest.TestCase):
    SOURCE = (SCRIPTS / "alloy_check.py").read_text()

    def test_any_function_changes_the_key_and_the_mutant_list_does_not(self):
        base = alloy_check.runner_sha(self.SOURCE)
        helper = "def java_ok():"
        self.assertIn(helper, self.SOURCE)
        self.assertNotEqual(base, alloy_check.runner_sha(self.SOURCE.replace(helper, helper + "\n    pass", 1)))
        grown = self.SOURCE.replace("MUTANTS = [\n", 'MUTANTS = [\n    ("Tree.als", "X", "a", "b"),\n', 1)
        self.assertNotEqual(grown, self.SOURCE)
        self.assertEqual(base, alloy_check.runner_sha(grown))


class Alloy(unittest.TestCase):
    def test_reuse_and_failures(self):
        with tempfile.TemporaryDirectory() as models, cache_env(), \
                mock.patch.object(alloy_check, "MODELS", pathlib.Path(models)), \
                mock.patch.object(alloy_check, "MUTANTS", [("M.als", "A", "rule", "no rule")]), \
                mock.patch.object(alloy_check, "run", fake_alloy_run), \
                mock.patch.object(alloy_check, "java_version", lambda: "17"):
            write(pathlib.Path(models), "M.als", "module M\nopen Lib\nrule\n")
            write(pathlib.Path(models), "Lib.als", "module Lib\n")
            fake_alloy_run.calls = 0
            out = run_quiet(alloy_check.check)
            self.assertEqual(fake_alloy_run.calls, 3)  # Lib, M, the mutant
            self.assertNotIn("reused", out)
            out = run_quiet(alloy_check.check)
            self.assertEqual(fake_alloy_run.calls, 3)
            self.assertIn("mutant M.als -> A: caught", out)
            self.assertRegex(out, r"(?m)^reused [0-9a-f]{12}: mutant M.als -> A$")
            write(pathlib.Path(models), "Lib.als", "module Lib\nsig S {}\n")  # opened by M: M and the mutant rerun
            run_quiet(alloy_check.check)
            self.assertEqual(fake_alloy_run.calls, 6)
            write(pathlib.Path(models), "Lib.als", "module Lib\nFAIL\n")
            with self.assertRaises(SystemExit):
                run_quiet(alloy_check.check)
            calls = fake_alloy_run.calls
            with self.assertRaises(SystemExit):
                run_quiet(alloy_check.check)
            self.assertEqual(fake_alloy_run.calls - calls, 1, "the failing Lib must run again, never be reused")


def run_quiet(fn):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        fn(pathlib.Path(tempfile.gettempdir()))
    return buf.getvalue()


if __name__ == "__main__":
    unittest.main()
