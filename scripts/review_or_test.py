#!/usr/bin/env python3
"""scripts/review-or.py against a local fake of the chat completions API (no network).

The fake records the request and answers what each test sets. A review is done only when it has content and was not
cut: an empty content or finish_reason = length exits non-zero, which is what the 2026-11-24 runs needed.
"""
import contextlib
import http.server
import importlib.util
import io
import json
import os
import pathlib
import shutil
import tempfile
import threading
import unittest
from unittest import mock

SCRIPTS = pathlib.Path(__file__).resolve().parent


def load():
    spec = importlib.util.spec_from_file_location("review_or_under_test", SCRIPTS / "review-or.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class Fake(http.server.BaseHTTPRequestHandler):
    status = 200
    answer = {}
    seen = []

    def do_POST(self):
        body = self.rfile.read(int(self.headers["Content-Length"]))
        Fake.seen.append(dict(path=self.path, auth=self.headers["Authorization"], body=json.loads(body)))
        data = json.dumps(Fake.answer).encode()
        self.send_response(Fake.status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


def answer(content, finish="stop", cost=0.0512):
    return {"model": "deepseek/deepseek-v4-pro",
            "choices": [{"message": {"role": "assistant", "content": content}, "finish_reason": finish}],
            "usage": {"prompt_tokens": 26000, "completion_tokens": 9000, "cost": cost}}


class ReviewOR(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fake)
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()

    def setUp(self):
        tmp = tempfile.mkdtemp(prefix="review-or-")
        self.addCleanup(shutil.rmtree, tmp)
        self.bundle = pathlib.Path(tmp) / "bundle.md"
        self.bundle.write_text("# Review\nYou have no tools; cite file and line.\n")
        self.out = pathlib.Path(tmp) / "review.md"
        Fake.status, Fake.answer, Fake.seen = 200, answer("Verdict: accept."), []
        env = {k: v for k, v in os.environ.items() if k not in ("OPENROUTER_API_KEY", "LLM_API_ROOT_OPENROUTER")}
        env.update(OPENROUTER_API_KEY="sk-test",
                   LLM_API_ROOT_OPENROUTER=f"http://127.0.0.1:{self.server.server_address[1]}/api/v1")
        patch = mock.patch.dict(os.environ, env, clear=True)
        patch.start()
        self.addCleanup(patch.stop)
        self.review = load()

    def call(self, *extra):
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = self.review.main([str(self.bundle), str(self.out), *extra])
        return code, stdout.getvalue(), stderr.getvalue()

    def test_a_review_is_written_with_its_real_cost(self):
        code, out, _ = self.call()
        self.assertEqual(code, 0)
        self.assertEqual(self.out.read_text(), "Verdict: accept.\n")
        for want in ("prompt 26000 tokens", "completion 9000 tokens", "cost 0.0512 USD", "finish_reason stop"):
            self.assertIn(want, out)
        (sent,) = Fake.seen
        self.assertEqual(sent["path"], "/api/v1/chat/completions")
        self.assertEqual(sent["auth"], "Bearer sk-test")
        body = sent["body"]
        self.assertEqual(body["model"], "deepseek/deepseek-v4-pro")
        self.assertEqual(body["messages"], [{"role": "user", "content": self.bundle.read_text()}])
        # The recipe that worked: a reasoning budget in tokens, never an effort, below max_tokens.
        self.assertEqual(body["reasoning"], {"max_tokens": 14000})
        self.assertEqual(body["max_tokens"], 24000)
        self.assertEqual(body["provider"], {"data_collection": "deny"})

    def test_model_and_reasoning_are_chosen(self):
        code, _, _ = self.call("--model", "x/y", "--reasoning", "2000", "--max-tokens", "8000")
        self.assertEqual(code, 0)
        body = Fake.seen[0]["body"]
        self.assertEqual((body["model"], body["reasoning"], body["max_tokens"]), ("x/y", {"max_tokens": 2000}, 8000))

    def test_empty_content_fails(self):
        for content in ("", "  \n", None):
            Fake.answer = answer(content)
            code, out, err = self.call()
            self.assertEqual(code, 1, content)
            self.assertIn("no content", err)
            self.assertIn("cost 0.0512 USD", out, "the spend is shown even when the review failed")
            self.assertFalse(self.out.exists())

    def test_cut_answer_fails(self):
        Fake.answer = answer("Partial verdict", finish="length")
        code, _, err = self.call()
        self.assertEqual(code, 1)
        self.assertIn("cut at max_tokens", err)

    def test_no_choices_fails(self):
        Fake.answer = {"usage": {"cost": 0}}
        self.assertEqual(self.call()[0], 1)

    def test_http_error_fails(self):
        Fake.status, Fake.answer = 402, {"error": {"message": "insufficient credit"}}
        code, _, err = self.call()
        self.assertEqual(code, 4)
        self.assertIn("HTTP 402", err)

    def test_missing_key_exits_3_without_a_call(self):
        del os.environ["OPENROUTER_API_KEY"]
        self.assertEqual(self.call()[0], 3)
        self.assertEqual(Fake.seen, [])

    def test_reasoning_must_leave_room_for_the_answer(self):
        self.assertEqual(self.call("--reasoning", "24000")[0], 2)
        self.assertEqual(Fake.seen, [])

    def test_empty_bundle_exits_2_without_a_call(self):
        self.bundle.write_text("\n")
        self.assertEqual(self.call()[0], 2)
        self.assertEqual(Fake.seen, [])


if __name__ == "__main__":
    unittest.main()
