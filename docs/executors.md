# AI executors — how each one is launched

Which executor does which task is decided elsewhere: the routing table of `CLAUDE.md` (« Routage des modèles ») and
`docs/plans/2026-11-12-method-v2.md` (« When Codex is absent », « When Claude is absent », Codex quota policy, paid
budget per review). This file is only the operating manual: launch command, prerequisites outside the repository,
known traps. Entry file read by non-Claude agents: `AGENTS.md`.

Rules common to all:

- Keys live in `mise.local.toml` (ignored by git), never in `mise.toml` (public repository) and never printed. Run a
  script that needs one under `mise x -- …`.
- Never send secrets, security work or a review to a free endpoint: it may log prompts, and a free model never counts
  as a review.
- A delegated executor never commits, stashes or pushes; the brief says so, and the integrator re-reads its diff.
- `mise run quotas` prints the remaining quota and reset time of each provider (`scripts/quotas.sh`).
- Paid OpenRouter credit is small (about $9 on 2026-11-08) and is kept for calls that are really necessary (a critical
  security review, a complete-tier review point): ask the owner before any spend beyond the per-review budget of
  method v2, and measure real spend on `https://openrouter.ai/api/v1/credits` before and after.
- Never run `pkill -f` or `pgrep -f` with a pattern that also appears in the command's own line: it matches the calling
  shell and kills it (exit 144).

## Claude Code (interactive session)

Reads `CLAUDE.md`, the hooks of `.claude/settings.json`, the agents of `.claude/agents/` (each header fixes its model
and effort) and the skills of `.claude/skills/`. rtk (output filter) comes from the user's global
configuration, outside the repository. gortex (code navigation) was removed on 2026-11-13: use gopls and plain search.

## Codex

- From a Claude session: the `codex` plugin (`/codex:rescue`, `/codex:review`, `/codex:adversarial-review`, agent
  `codex:codex-rescue`), always with an explicit `--model` and `--effort` (`CLAUDE.md`, « Codex, même règle »).
- Standalone, or when Claude's quota is low: `codex exec -o report.md - < prompt.md`, detached and watched like any
  long task. Check that the job really started right after launching it (a quota error ends it at once).
- Reads `AGENTS.md` at the repository root. Login: ChatGPT subscription (default). Profiles for other providers are
  separate files in `~/.codex/` (`chatgpt.config.toml`, `openrouter.config.toml`, `openrouter-cheap.config.toml`),
  selected with `--profile` on the `codex` CLI only.
- Trap: Codex is not subject to the Claude hooks. The brief repeats: tests only through `mise run …` or
  `scripts/heavy.sh` with a `timeout`.

## Claude through OpenRouter or the API key (no subscription quota)

- `mise x -- bash scripts/claude-or.sh -t pro|flash|haiku|sonnet|opus [-e effort] [-b max_usd] [-w dir] < prompt.md`:
  headless Claude Code billed to `OPENROUTER_API_KEY`; the project's hooks and rtk apply. Tiers and models are
  in the script's header. `-b` is only a runaway guard (it overprices non-Anthropic models). `:free` models fail tool
  calling through this route.
- `mise x -- bash scripts/claude-api.sh [-m model] [-t max_tokens] [-e effort] < prompt.md > answer.md`: one-shot
  call without tools, billed to `CLAUDE_API_KEY`; for a review of a curated bundle. Always pass an effort (the default
  is medium): without one, Opus can spend the whole token budget thinking.

## Free models through opencode

- `mise x -- bash scripts/opencode-free.sh [-m model] [-w dir] < prompt.md`. Default model and the ones known to
  work with tools are listed in the script's header; Gemini Flash runs through it with
  `GOOGLE_GENERATIVE_AI_API_KEY=$GEMINI_API_KEY … -m google/<model>`.
- Prerequisites outside the repository: opencode installed in `~/.local/opencode`
  (`npm i --prefix ~/.local/opencode opencode-ai`).
- opencode has no rtk hook and no Claude hook: the script prefixes the prompt with the rule (rtk, never commit
  or stash). Reads `AGENTS.md`.
- Use: implementation guided by a frozen gate, mechanical work. Never security; its output is always re-read by a
  strong model. Free quotas are small (a few tasks a day on some providers) and often saturated in the evening.

## Mechanical text: `scripts/llm.sh`

One-shot text (commit message drafts, log triage, translation drafts) through hosted open-weight models, in the
provider order written in the script and in the routing table (ADR 0013). Exit code 3 means no provider answered:
fall back to the cheapest Claude tier. Never secrets, security, design or code review.

## Gemini (Antigravity CLI `agy`, Gemini CLI)

- `agy -p "<prompt>" --mode accept-edits --sandbox --effort high --model <model> --output-format text` (headless).
  Permissions for headless runs live in `~/.gemini/antigravity-cli/settings.json` (`permissions.allow`).
- The free individual tier is small (a handful of tasks, reset about weekly) and `--effort` is rejected by some
  models. Used as the third family for a complete-tier review when Codex is absent (method v2).
- Gemini CLI reads `GEMINI.md`, which imports `AGENTS.md`.

## A new machine

Nothing above that lives under `~` is in the repository. After `mise run setup`: install the CLIs you will use
(`codex`, `claude`, opencode, `agy`), log in, add the keys to `mise.local.toml` (names in `.env.reference` and in each
script's header), then run `mise run quotas`.
