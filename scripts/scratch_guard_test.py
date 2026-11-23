"""Tests keep their scratch outside the checkout.

A test that creates and removes a directory in the working tree races every walker of the live
tree: `go test ./...` read a vanished `tmpd0_i7782` and failed with "setup failed" while test:py ran
beside it (CI run 45138932780; the creator was `tempfile.TemporaryDirectory(dir=ROOT)` in
scripts/perf). Scratch belongs under the system temp dir (Python's default) or t.TempDir() (Go).

Static on purpose: a before/after listing of the tree during `ci:checks` would race the sibling
tasks that legitimately write root artefacts (coverage.out, junit.xml, sbom.cdx.json).
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


if __name__ == "__main__":
    unittest.main()
