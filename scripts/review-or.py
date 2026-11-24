#!/usr/bin/env python3
"""One-shot cross-family review on OpenRouter: a curated bundle in, the review out (block CI3a, 2026-11-24).

Usage: mise x -- python3 scripts/review-or.py BUNDLE OUT [--model M] [--reasoning N] [--max-tokens N]

The bundle (plan, spec diff, gate, code excerpts, attack questions, "you have no tools, cite file and line") is sent
as one user message to the chat completions API, with a reasoning budget in tokens (`reasoning.max_tokens`) below the
completion budget so the answer always has room, and `provider.data_collection = deny`. The content is written to
OUT; prompt and completion tokens, the real cost (`usage.cost`) and finish_reason are printed. Exit 1 when the review
is empty or cut (finish_reason = length): an empty review is never reported as done. Exit 2 on usage, 3 when
OPENROUTER_API_KEY is unset (run through `mise x --` from a tree that has mise.local.toml), 4 on an HTTP error.

Why not scripts/claude-or.sh: with DeepSeek V4 Pro its tool loop returned an empty result after 22 turns, and
one-shot calls with `reasoning.effort` high or medium spent all of max_tokens on reasoning and returned no content
(2026-11-24, about $0.12 each). LLM_API_ROOT_OPENROUTER overrides the API root (the tests point it at a local server).
"""
import argparse
import json
import os
import pathlib
import sys
import urllib.error
import urllib.request

DEFAULT_MODEL = "deepseek/deepseek-v4-pro"
DEFAULT_REASONING = 14000
DEFAULT_MAX_TOKENS = 24000
DEFAULT_ROOT = "https://openrouter.ai/api/v1"
TIMEOUT_SECONDS = 900


def parse(argv):
    p = argparse.ArgumentParser(prog="review-or.py", description=__doc__.splitlines()[0])
    p.add_argument("bundle", type=pathlib.Path)
    p.add_argument("out", type=pathlib.Path)
    p.add_argument("--model", default=DEFAULT_MODEL)
    p.add_argument("--reasoning", type=int, default=DEFAULT_REASONING, help="reasoning budget in tokens")
    p.add_argument("--max-tokens", type=int, default=DEFAULT_MAX_TOKENS, help="completion budget, reasoning included")
    args = p.parse_args(argv)
    if not 0 < args.reasoning < args.max_tokens:
        p.error("--reasoning must be positive and below --max-tokens, or the answer has no room")
    return args


def request_body(bundle, args):
    return {
        "model": args.model,
        "messages": [{"role": "user", "content": bundle}],
        "reasoning": {"max_tokens": args.reasoning},
        "max_tokens": args.max_tokens,
        "provider": {"data_collection": "deny"},
        "usage": {"include": True},
    }


def main(argv):
    try:
        args = parse(argv)
    except SystemExit as exc:
        return 0 if exc.code == 0 else 2
    key = os.environ.get("OPENROUTER_API_KEY", "")
    if not key:
        print("review-or: OPENROUTER_API_KEY unset (mise.local.toml; run through `mise x --`)", file=sys.stderr)
        return 3
    bundle = args.bundle.read_text()
    if not bundle.strip():
        print(f"review-or: {args.bundle} is empty", file=sys.stderr)
        return 2
    root = os.environ.get("LLM_API_ROOT_OPENROUTER") or DEFAULT_ROOT
    req = urllib.request.Request(
        root.rstrip("/") + "/chat/completions", data=json.dumps(request_body(bundle, args)).encode(),
        headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT_SECONDS) as resp:  # noqa: S310 (fixed https root or a test)
            answer = json.load(resp)
    except urllib.error.HTTPError as exc:
        print(f"review-or: HTTP {exc.code}: {exc.read()[:500].decode(errors='replace')}", file=sys.stderr)
        return 4
    except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
        print(f"review-or: no answer: {exc}", file=sys.stderr)
        return 4
    choice = (answer.get("choices") or [{}])[0]
    content = ((choice.get("message") or {}).get("content") or "").strip()
    finish = choice.get("finish_reason")
    usage = answer.get("usage") or {}
    print(f"model {answer.get('model', args.model)}; prompt {usage.get('prompt_tokens')} tokens, completion "
          f"{usage.get('completion_tokens')} tokens; cost {usage.get('cost')} USD; finish_reason {finish}")
    if content:
        args.out.write_text(content + "\n")
    if not content:
        print("review-or: FAILED: the answer has no content (reasoning may have used the whole budget)", file=sys.stderr)
        return 1
    if finish == "length":
        print(f"review-or: FAILED: the answer was cut at max_tokens (partial text in {args.out})", file=sys.stderr)
        return 1
    print(f"review written to {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
