"""Exercise the local mise lock check with isolated TOML fixtures."""

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().with_name("check-mise-lock.py")
GO_CONFIG = '[tools]\ngo = "1.27.4"\n'
GO_LOCK = '''
[[tools.go]]
version = "1.27.4"
backend = "core:go"

[tools.go."platforms.linux-x64"]
url = "https://example.invalid/go-linux-amd64"

[tools.go."platforms.linux-arm64"]
url = "https://example.invalid/go-linux-arm64"
'''


class MiseLockTest(unittest.TestCase):
    def check(self, config, lock):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "mise.toml").write_text(config, encoding="utf-8")
            (root / "mise.lock").write_text(lock, encoding="utf-8")
            return subprocess.run(
                [sys.executable, "-I", str(SCRIPT)],
                cwd=root, capture_output=True, text=True, timeout=5,
            )

    def test_matching_pin(self):
        result = self.check(GO_CONFIG, GO_LOCK)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_stale_pin_even_with_both_platforms(self):
        result = self.check(GO_CONFIG, GO_LOCK.replace('version = "1.27.4"', 'version = "1.27.3"'))
        self.assertEqual(result.returncode, 1)
        self.assertIn("go@1.27.4 is missing (locked: 1.27.3)", result.stderr)

    def test_missing_platform_for_exact_and_floating_requests(self):
        for version in ("1.27.4", "latest"):
            with self.subTest(version=version):
                lock = GO_LOCK.replace("platforms.linux-arm64", "platforms.macos-arm64")
                result = self.check(f'[tools]\ngo = "{version}"\n', lock)
                self.assertEqual(result.returncode, 1)
                self.assertIn("go@1.27.4 is missing linux-arm64", result.stderr)

    def test_unrecorded_latest_is_optional(self):
        result = self.check(GO_CONFIG + 'shellcheck = "latest"\n', GO_LOCK)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_go_backend_locks_source_version_without_binary_entries(self):
        tool = '"go:example.invalid/tool"'
        config = f'[tools]\n{tool} = "v0.0.0-20261009192801-be2c69fb417e"\n'
        lock = f'[[tools.{tool}]]\nversion = "0.0.0-20261009192801-be2c69fb417e"\n'
        result = self.check(config, lock)
        self.assertEqual(result.returncode, 0, result.stderr)
        result = self.check(config, lock.replace("be2c69fb417e", "123456789abc"))
        self.assertEqual(result.returncode, 1)
        self.assertIn("tool@v0.0.0-20261009192801-be2c69fb417e is missing", result.stderr)
