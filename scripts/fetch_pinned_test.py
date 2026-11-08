#!/usr/bin/env python3
"""Tests for scripts/fetch_pinned.py"""
import hashlib
import io
import os
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import fetch_pinned


# The response is a real io.BytesIO: a MagicMock whose read() never returns b""
# makes shutil.copyfileobj loop forever (global OOM on 2026-11-08).
class FetchPinnedTest(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__('shutil').rmtree(self.tmpdir, ignore_errors=True))

    def test_matching_hash_returns_file(self):
        url = "https://example.com/file.jar"
        content = b"hello world"
        sha256 = hashlib.sha256(content).hexdigest()
        dest = pathlib.Path(self.tmpdir) / "file.jar"

        with patch('urllib.request.urlopen', return_value=io.BytesIO(content)):

            result = fetch_pinned.fetch(url, sha256, dest)

        self.assertEqual(result, dest)
        self.assertTrue(dest.exists())
        self.assertEqual(dest.read_bytes(), content)

    def test_mismatched_hash_fails_and_leaves_no_file(self):
        url = "https://example.com/file.jar"
        content = b"hello world"
        wrong_sha256 = "0" * 64
        dest = pathlib.Path(self.tmpdir) / "file.jar"

        with patch('urllib.request.urlopen', return_value=io.BytesIO(content)):

            with self.assertRaises(SystemExit) as cm:
                fetch_pinned.fetch(url, wrong_sha256, dest)

        self.assertIn("differs from pinned", str(cm.exception))
        self.assertFalse(dest.exists())
        self.assertFalse(dest.with_suffix(".part").exists())

    def test_existing_valid_cache_reused_without_download(self):
        url = "https://example.com/file.jar"
        content = b"hello world"
        sha256 = hashlib.sha256(content).hexdigest()
        dest = pathlib.Path(self.tmpdir) / "file.jar"
        dest.write_bytes(content)

        with patch('urllib.request.urlopen') as mock_urlopen:
            result = fetch_pinned.fetch(url, sha256, dest)
            mock_urlopen.assert_not_called()

        self.assertEqual(result, dest)
        self.assertEqual(dest.read_bytes(), content)

    def test_existing_invalid_cache_is_replaced(self):
        url = "https://example.com/file.jar"
        old_content = b"old content"
        new_content = b"new content"
        sha256 = hashlib.sha256(new_content).hexdigest()
        dest = pathlib.Path(self.tmpdir) / "file.jar"
        dest.write_bytes(old_content)

        with patch('urllib.request.urlopen', return_value=io.BytesIO(new_content)):

            result = fetch_pinned.fetch(url, sha256, dest)

        self.assertEqual(result, dest)
        self.assertEqual(dest.read_bytes(), new_content)


if __name__ == "__main__":
    unittest.main()