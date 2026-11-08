#!/usr/bin/env python3
"""Provider fall-through of scripts/llm.sh, with a fake `curl` on PATH (no network, no real keys)."""
import json
import os
import pathlib
import subprocess
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).resolve().parent / "llm.sh"

# Fake curl: picks the canned (status, body) whose key is a substring of the URL, logs the URL.
FAKE_CURL = r'''#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
out = args[args.index("-o") + 1]
url = next(a for a in args if a.startswith("http"))
with open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "log"), "a") as f:
    f.write(url + "\n")
for key, (status, body) in json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "routes.json"))).items():
    if key in url:
        open(out, "w").write(json.dumps(body))
        sys.stdout.write(str(status))
        break
else:
    sys.stdout.write("000")
'''

OK = lambda text: (200, {"choices": [{"message": {"content": text}}]})  # noqa: E731
CF_OK = lambda text: (200, {"result": {"response": text}})  # noqa: E731


class FallThrough(unittest.TestCase):
    def run_llm(self, routes, keys, stdin="log", args=("summarize", "3")):
        with tempfile.TemporaryDirectory() as d:
            bindir = pathlib.Path(d)
            curl = bindir / "curl"
            curl.write_text(FAKE_CURL)
            curl.chmod(0o755)
            routes_file, log = bindir / "routes.json", bindir / "log"
            routes_file.write_text(json.dumps(routes))
            env = {
                "PATH": f"{bindir}:{os.environ['PATH']}",
                "HOME": os.environ["HOME"],
                **keys,
            }
            p = subprocess.run(
                ["bash", str(SCRIPT), *args], input=stdin, capture_output=True, text=True, env=env, timeout=60
            )
            calls = log.read_text().splitlines() if log.exists() else []
            return p, [c.split("/")[2] for c in calls]

    ALL = {"GROQ_API_KEY": "k1", "CLOUDFLARE_API_TOKEN": "k2", "CLOUDFLARE_ACCOUNT_ID": "acc", "OPENROUTER_API_KEY": "k3"}

    def test_groq_first(self):
        p, hosts = self.run_llm({"groq.com": OK("from groq")}, self.ALL)
        self.assertEqual((p.returncode, p.stdout.strip(), hosts), (0, "from groq", ["api.groq.com"]))

    def test_429_falls_to_cloudflare(self):
        routes = {"groq.com": (429, {}), "cloudflare.com": CF_OK("from cf")}
        p, hosts = self.run_llm(routes, self.ALL)
        self.assertEqual((p.returncode, p.stdout.strip()), (0, "from cf"))
        self.assertEqual(hosts, ["api.groq.com", "api.cloudflare.com"])

    def test_5xx_then_openrouter_free_then_paid(self):
        routes = {"groq.com": (503, {}), "cloudflare.com": (500, {}), "openrouter.ai": OK("from or")}
        p, hosts = self.run_llm(routes, self.ALL)
        self.assertEqual((p.returncode, p.stdout.strip()), (0, "from or"))
        self.assertEqual(hosts, ["api.groq.com", "api.cloudflare.com", "openrouter.ai"])

    def test_missing_keys_skip_without_calling(self):
        p, hosts = self.run_llm({"openrouter.ai": OK("only or")}, {"OPENROUTER_API_KEY": "k3"})
        self.assertEqual((p.returncode, p.stdout.strip(), hosts), (0, "only or", ["openrouter.ai"]))

    def test_nothing_available_exits_3(self):
        p, hosts = self.run_llm({}, {})
        self.assertEqual((p.returncode, hosts), (3, []))

    def test_all_fail_exits_3(self):
        routes = {k: (429, {}) for k in ("groq.com", "cloudflare.com", "openrouter.ai")}
        p, _ = self.run_llm(routes, self.ALL)
        self.assertEqual(p.returncode, 3)

    def test_empty_content_falls_through(self):
        routes = {"groq.com": OK(""), "cloudflare.com": CF_OK("cf")}
        p, _ = self.run_llm(routes, self.ALL)
        self.assertEqual(p.stdout.strip(), "cf")

    def test_large_input_skips_groq(self):
        routes = {"groq.com": OK("groq"), "cloudflare.com": CF_OK("cf")}
        p, hosts = self.run_llm(routes, self.ALL, stdin="x" * 20000)
        self.assertEqual((p.stdout.strip(), hosts), ("cf", ["api.cloudflare.com"]))

    def test_writing_tasks_use_openrouter_only(self):
        routes = {"groq.com": OK("groq"), "openrouter.ai": OK("or")}
        p, hosts = self.run_llm(routes, self.ALL, args=("prose",))
        self.assertEqual((p.stdout.strip(), hosts), ("or", ["openrouter.ai"]))

    def test_no_key_in_output(self):
        p, _ = self.run_llm({"groq.com": (429, {})}, self.ALL)
        for secret in ("k1", "k2", "k3"):
            self.assertNotIn(f"Bearer {secret}", p.stdout + p.stderr)


if __name__ == "__main__":
    unittest.main()
