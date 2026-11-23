"""Tests keep their scratch outside the checkout.

A test that creates and removes a directory in the working tree races every walker of the live
tree: `go test ./...` read a vanished `tmpd0_i7782` and failed with "setup failed" while test:py ran
beside it (CI run 45138932780; the creator was `tempfile.TemporaryDirectory(dir=ROOT)` in
scripts/perf). Scratch belongs under the system temp dir (Python's default) or t.TempDir() (Go).

Static on purpose: a before/after listing of the tree during `ci:checks` would race the sibling
tasks that legitimately write root artefacts (coverage.out, junit.xml, sbom.cdx.json).

Scratch must also go away, killed run or not (/tmp is a tmpfs: what stays there holds memory until reboot;
10 GB of it on 2026-11-23). heavy.sh gives every job its own TMPDIR, removed even after a kill; the classes
it cannot reach have a rule here, each cheap and each seen in /tmp:
- Java ignores TMPDIR: a launch names java.io.tmpdir (TLC left an empty tlc-<n> folder per run, Alloy an
  alloy-plingeling<n>.cnf per command);
- a Python test that calls mkdtemp() also removes it (shutil.rmtree), else one folder stays per run;
- a shell script that calls mktemp has a trap to remove it, so that a signal does not leave it behind.
A runtime sweep (a /tmp listing before and after) cannot be a test: other sessions write there meanwhile.
"""
import ast
import re
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TEMPFILE_CALLS = {"TemporaryDirectory", "mkdtemp", "mkstemp", "NamedTemporaryFile", "TemporaryFile",
                  "SpooledTemporaryFile"}
# os.MkdirTemp / os.CreateTemp with a non-empty literal directory: relative to the package folder.
GO_LITERAL_DIR = re.compile(r'\bos\.(?:MkdirTemp|CreateTemp)\(\s*"[^"]+"')
# A JVM launch: `java` and its options up to -cp or -jar, in a shell command (continuation lines included)
# or a Python argument list. JVM options precede -cp/-jar, so java.io.tmpdir must be among them.
SHELL_JAVA = re.compile(r'\bjava((?:[ \t]+(?:\\\n[ \t]*)?-\S+)*?)[ \t]+(?:\\\n[ \t]*)?-(?:cp|jar)\b')
PY_JAVA = re.compile(r'["\']java["\']((?:\s*,\s*f?["\'][^"\']*["\'])*?)\s*,\s*["\']-(?:cp|jar)["\']')


def tracked(*patterns):
    out = subprocess.run(["git", "ls-files", "-z", "--", *patterns], cwd=ROOT, check=True,
                         capture_output=True).stdout
    return [ROOT / name for name in out.decode().split("\0") if name]


def python_offences(source, name="<test>"):
    """Lines where a tempfile call chooses its own directory (keyword dir= or the third positional)."""
    offences = []
    for node in ast.walk(ast.parse(source, name)):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        called = func.attr if isinstance(func, ast.Attribute) else func.id if isinstance(func, ast.Name) else ""
        if called in TEMPFILE_CALLS and (any(k.arg == "dir" for k in node.keywords) or len(node.args) >= 3):
            offences.append(node.lineno)
    return offences


def java_offences(text):
    """Offsets of the JVM launches in text that leave java.io.tmpdir at its default (/tmp)."""
    return [m.start() for pattern in (SHELL_JAVA, PY_JAVA) for m in pattern.finditer(text)
            if "java.io.tmpdir" not in m.group(1)]


def mkdtemp_without_rmtree(source, name="<test>"):
    """True when a Python source calls mkdtemp() and never names shutil.rmtree."""
    names = {node.attr if isinstance(node, ast.Attribute) else node.id
             for node in ast.walk(ast.parse(source, name)) if isinstance(node, (ast.Attribute, ast.Name))}
    return "mkdtemp" in names and "rmtree" not in names


class ScratchOutsideCheckoutTests(unittest.TestCase):
    def test_detector_flags_a_chosen_directory_and_accepts_the_default(self):
        self.assertEqual(python_offences("import tempfile\ntempfile.TemporaryDirectory(dir=ROOT)\n"), [2])
        self.assertEqual(python_offences("from tempfile import mkdtemp\nmkdtemp('', 'tmp', ROOT)\n"), [2])
        self.assertEqual(python_offences("import tempfile\ntempfile.TemporaryDirectory(prefix='x-')\n"), [])
        self.assertIsNotNone(GO_LITERAL_DIR.search('os.MkdirTemp(".", "x-")'))
        self.assertIsNone(GO_LITERAL_DIR.search('os.MkdirTemp("", "x-")'))
        self.assertIsNone(GO_LITERAL_DIR.search('os.MkdirTemp(t.TempDir(), "x-")'))

    def test_python_tests_use_the_system_temp_dir(self):
        found = [f"{path.relative_to(ROOT)}:{line}" for path in tracked("*_test.py")
                 for line in python_offences(path.read_text(), str(path))]
        self.assertEqual(found, [], "tempfile scratch must not choose its directory in a test")

    def test_go_tests_never_create_temp_entries_beside_the_package(self):
        found = [f"{path.relative_to(ROOT)}:{n}" for path in tracked("*_test.go")
                 for n, text in enumerate(path.read_text().splitlines(), 1) if GO_LITERAL_DIR.search(text)]
        self.assertEqual(found, [], "use t.TempDir() or os.MkdirTemp(\"\", …) in Go tests")



class ScratchRemovedTests(unittest.TestCase):
    def test_detectors_flag_the_leaking_forms_and_accept_the_fixed_ones(self):
        self.assertEqual(len(java_offences('java -Xmx2g -cp "$jar" tlc2.TLC')), 1)
        self.assertEqual(len(java_offences('java -Xmx2g \\\n  -jar x.jar')), 1)
        self.assertEqual(java_offences('java -Xmx2g -Djava.io.tmpdir="$d" -cp "$jar" tlc2.TLC'), [])
        self.assertEqual(java_offences('java -version 2>&1 | head -1'), [])
        self.assertEqual(len(java_offences('["java", "-Xmx2g",\n "-jar", jar]')), 1)
        self.assertEqual(java_offences('["java", f"-Djava.io.tmpdir={d}", "-jar", jar]'), [])
        self.assertEqual(java_offences('["java", "-version"], capture_output=True'), [])
        self.assertTrue(mkdtemp_without_rmtree("import tempfile\nroot = tempfile.mkdtemp()\n"))
        self.assertFalse(mkdtemp_without_rmtree("d = tempfile.mkdtemp()\nself.addCleanup(shutil.rmtree, d)\n"))

    def test_java_launches_set_their_temporary_directory(self):
        found = [f"{path.relative_to(ROOT)}:{text.count(chr(10), 0, at) + 1}"
                 for path in tracked("*.sh", "*.py", "*.toml", "*.yml", ".githooks/*")
                 if path.name != Path(__file__).name and path.is_file()
                 for text in [path.read_text()] for at in java_offences(text)]
        self.assertEqual(found, [], "pass -Djava.io.tmpdir=<a folder removed on exit>: Java ignores TMPDIR")

    def test_python_tests_remove_their_mkdtemp_folders(self):
        found = [str(path.relative_to(ROOT)) for path in tracked("*_test.py")
                 if mkdtemp_without_rmtree(path.read_text(), str(path))]
        self.assertEqual(found, [], "use tempfile.TemporaryDirectory() with addCleanup, or rmtree what mkdtemp made")

    def test_shell_scripts_trap_their_mktemp(self):
        found = [str(path.relative_to(ROOT)) for path in tracked("*.sh", ".githooks/*") if path.is_file()
                 for text in [path.read_text()] if re.search(r"\bmktemp\b", text) and not re.search(r"^\s*trap\b", text, re.M)]
        self.assertEqual(found, [], "a script that calls mktemp removes it in a trap (EXIT; INT TERM HUP if needed)")


if __name__ == "__main__":
    unittest.main()
