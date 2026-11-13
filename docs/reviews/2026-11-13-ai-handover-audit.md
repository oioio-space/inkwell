# AI handover audit (2026-11-13)

Question: if the Claude Opus/Fable sessions stop, can another AI (Codex, Claude Sonnet/Haiku, Gemini, free models
through opencode/OpenRouter) continue this project and coordinate correctly, with Codex present or absent?
Method: read-only inspection of `main` at `48ae99b` (instruction files, hooks, scripts, memory outside the repo,
`git worktree list`, `git status --porcelain --ignored`, `du`). Nothing was deleted, committed or run heavy.
`mise run lint:claude-md` passes (exit 0) and every `docs/` entry is mapped in `docs/README.md`.

**Verdict.** The engineering content is transferable: gates, git hooks and `mise` tasks are executable by anyone, and
`PROGRESS.md` + `docs/README.md` + method v2 form a real map. The coordination layer is not: it is written for one
orchestrator (Claude Opus in Claude Code), no other AI has an entry file, the commit rule names Claude only, and
tonight's real state lives in an ignored folder and in uncommitted worktrees. A strong model (Codex, Opus through
`claude-or.sh`) could continue after fixes 1 to 4 below, about half a day of documentation work. A Sonnet, Haiku or
free model alone should not be the integrator: nothing in the repo tells it so, and the owner's rule saying so is
outside the repo (finding 5).

## Findings, ranked by risk to continuity

### 1. No entry file for any AI other than Claude Code (high)

- Evidence: no `AGENTS.md`, `GEMINI.md` or opencode config is tracked (`git ls-files` shows only
  `scripts/opencode-free.sh`). Codex reads `~/.codex/AGENTS.md`, which holds only the gortex block and no project
  rule. `README.md:47` points AI agents to `CLAUDE.md`, which Codex, Gemini and opencode do not load by themselves.
  `scripts/opencode-free.sh:25` injects one sentence (rtk, gortex, never commit or stash).
- Consequence: a non-Claude AI starts with none of the invariants, the definition of done, the memory caps or the
  frozen-gate rule, unless a Claude session writes them into its brief.
- Fix: add a root `AGENTS.md` (English, about 60 lines, read by Codex and opencode; `GEMINI.md` as a one-line pointer
  to it). Content: read order (`PROGRESS.md` top, `docs/README.md`, `CLAUDE.md`, method v2), the ten invariants by
  reference, the tool-neutral rules of finding 3, the cold-start checklist below. Add both files to
  `scripts/check_claude_md.py` so cited paths stay valid.

### 2. The live state cannot be picked up cold (high)

- `PROGRESS.md:23` says SI1b is "🔨 worktree … puis commit"; it is on `main` (`0752872`). `PROGRESS.md:28` says PR1
  is "à figer, puis code"; the gate is frozen (`1975027`) and code is written, uncommitted, in the locked worktree
  `agent-a8b133d4e69426085` (20 modified files). `PROGRESS.md:21` says OI is in "worktree" without naming it: it is
  `agent-a78adb31ac7116405`, detached at `8cacc7a`, 78 uncommitted files, 1.2 GB.
- `main` is 8 commits ahead of `origin/main` (`0b887fa`). The reason (push blocked by a Markdown highlighting test and
  load-sensitive tests) and the OI defect under repair (GNOME activation token not forwarded) are written only in
  `.remember/now.md` and `.remember/today-2026-11-13.md`, which are ignored and written by a Claude plugin.
  `CLAUDE.md:144` asks for a push after each block.
- Three more worktrees hold uncommitted work with no owner recorded: `agent-a4fe300c2db2a007f` (deterministic
  `supervisor_test.go`, `lobby_test.go`), `agent-ac6cc08b70cf28e5a` (three sidecar rule tests, possibly already
  landed by `b1ca1ef`), `agent-abf6df91a052c3d0e` (locked, `zzrepro.test.ts`).
- Working tree of `main`: `gates/SI1c.toml` and `docs/plans/2026-11-13-si1c-header.md` untracked,
  `docs/protocol/tla/README.md` modified.
- Fix: (a) correct rows 3, 5 and 9 of the `PROGRESS.md` table and add a short "Open work trees" list under it:
  path, block, state, what is uncommitted, next action, plus one line "unpushed commits: N, blocked by …".
  (b) Extend `scripts/block_done.py` step 7 or `lint:claude-md` to fail when a worktree in `git worktree list` is not
  named in `PROGRESS.md`. This replaces `.remember/` as the handover channel.

### 3. Coordination rules assume Claude; some have no answer without it (high)

- "Seule la session Claude commite" (`CLAUDE.md:144`, method v2 line 131-132). No file says who integrates when no
  Claude session exists. The git hook does enforce the attestation for everyone (`.githooks/pre-commit:65-80`), but
  any agent can write the marker; "never arm it without the review" is prose (`CLAUDE.md:135`).
- "When Codex is absent" (method v2 lines 84-118) replaces Codex by a blind Opus review. There is no symmetric
  "when Claude is absent" section: who orchestrates, who is the second family for a complete-tier block, who runs the
  user walkthrough.
- Model routing (`CLAUDE.md:250-274`) is expressed as Claude agent names (`go-dev`, `quality-runner`, …) with
  `model:`/`effort:` headers that only Claude Code reads.
- Fix: in method v2, add "When Claude is absent": one named integrator per session (the strongest available model),
  recorded in `PROGRESS.md`; it alone commits; implementation and review never come from the same family; a
  complete-tier block waits for two families; Sonnet, Haiku and free models never integrate or review. Reword
  `CLAUDE.md:144` to "only the integrating session commits (Claude when present)".

### 4. Safety rules enforced only by Claude Code hooks (high, machine-level)

| Rule | Enforced for Claude by | For Codex, Gemini, opencode |
|---|---|---|
| Memory cap on every test | `.claude/hooks/heavy-guard.sh` | prose; `mise` test tasks are capped, a raw `go test` is not (`CLAUDE.md:244` admits it). Three global OOMs came from this. |
| Committed gate is frozen | `.claude/hooks/gate-freeze.sh` | partly: `scripts/gate-guard.py` catches a weakened gate at gate time, not at edit time |
| Long task watched | `long-task-monitor.sh` + `Monitor` tool (`CLAUDE.md:217`) | no `Monitor`; `scripts/watch-log.sh` works in any shell but nothing says "run it in a loop yourself" |
| Check after each edit | `post-edit-check.sh` | found later by `pre-commit` |
| gortex, rtk | global Claude config, hooks | Codex has gortex (`~/.codex/config.toml`), opencode has it in `~/.config/opencode/opencode.json`; both outside the repo; rtk is prose |

- Fix: state the tool-neutral form in `AGENTS.md`: "tests only through `mise run …` or `scripts/heavy.sh`"; "never
  edit a committed `gates/*.toml`"; "long task: log file, then `scripts/watch-log.sh LOG` in the foreground with a
  timeout, repeated until `exit N` appears". For the memory cap, the durable fix is a `go` and `pnpm` shim on the
  agent `PATH` that refuses an uncapped test (same regular expression as `heavy-guard.sh`), since prose already
  failed once for Claude.

### 5. Durable owner rules that exist only in Claude's private memory (medium)

`~/.claude/projects/-home-mathieu-GolandProjects-inkwell/memory/` holds 42 files. Most rules are also in `CLAUDE.md`.
These are not in the repo and another AI would break them:

| Rule (memory file) | Move to |
|---|---|
| No `Claude-Session:` link in commits or PRs (`feedback-no-session-link.md`) | `CLAUDE.md` step 8 / `AGENTS.md` |
| Sonnet output and reviews are checked by Opus or Codex; Sonnet only for mechanical runs (`feedback-no-sonnet-trust.md`). `CLAUDE.md:266` still routes diff review to Sonnet. | routing table, row "Relire un diff Go" |
| Codex is a small subscription: short high-value reviews, use the quota fully, ask Codex before building tooling (`codex-routing.md`, 2026-11-12) | method v2, "Codex, when present" |
| How each executor is launched and its traps: `codex exec -o report.md - < prompt.md`, `env_vars = ['XDG_RUNTIME_DIR']` for gortex, gortex `edit` refused in `codex exec`, `agy` headless flags and quotas, Gemini through opencode, never `pkill -f` on a pattern in its own line (`codex-gortex-env.md`) | a new `docs/executors.md`, mapped in `docs/README.md` |
| Paid OpenRouter credit is about $9, kept for necessary calls, ask before spending (`codex-routing.md`) | same file |
| Every finding cites a standard and a measurement (`feedback-no-empiricism.md`) | `docs/engineering-principles.md` § 0 if absent |

Stale or contradictory memories (to prune by a Claude session, not in the repo): `project-pause-2026-11-11.md` and
`project-l3-state.md` (old states, one cites a worktree that no longer exists); `feedback-hdae-method.md` (says
`docs/objectif-en-cours.md` carries the active lot and "one long Codex task per lot", both reversed since);
`feedback-token-economy-practice.md` (cites `.claude/hooks/openrouter-check.sh`, removed);
`feedback-dual-review-loop.md` against `feedback-use-fable-free-period.md` (temporary override, to delete when the
free period ends); `codex-routing.md` contradicts itself across dates ("Codex = implementer by default" on 11-05,
"no long implementations" on 11-12).

### 6. Contradictions between instruction files (medium)

- Codex for implementation: yes in `CLAUDE.md:259` and `docs/autonomous-decisions.md:10`; "Not for implementation"
  in method v2 line 112. Fix: keep method v2, reword the table row to "review, diagnosis, second opinion".
- `next` → `main` queue (method v2 rule 7, and "the block waits on `next`", line 97): no `next` branch exists and
  `CLAUDE.md` never mentions it. Fix: create it and use it, or strike rule 7 with a dated note.
- "`docs/objectif-en-cours.md` is folded into `PROGRESS.md`" (method v2 line 126); the file exists (237 lines), is
  the tool registry (`docs/README.md:15`) and still has an ordered "Lots suivants" list (lines 33-45) although its
  own header says order lives in `PROGRESS.md`. Fix: correct line 126; move lines 33-45 to the archive.
- P/E core placement: required in `CLAUDE.md` (« Mémoire », `taskset -c 0-11` / `12-19`); "Stopped: … HP's P/E
  placement" in method v2 line 128; `scripts/heavy.sh` has no `taskset`. Fix: drop one of the two.
- Worktrees: "au plus un worktree par bloc actif" (`CLAUDE.md:309`) and "about three checkouts" (method v2 rule 1);
  eight exist. The rule has no check (see fix 2b).
- gopls MCP: `CLAUDE.md:50` asks for `go_diagnostics` after each edit; `.claude/settings.local.json` disables the
  `gopls` server. Fix: drop the sentence or re-enable the server.
- Stale comment: `.claude/hooks/pre-commit-simplify.sh` cites sibling `commit-*-review.sh` hooks that no longer exist.
- `research-grounding.sh`: listed as removed in method v2 line 122, present and cited in `CLAUDE.md:14` (it was
  rewritten to fire only on plans and ADRs). Fix: one clause in method v2.

### 7. Quality of the instructions for a non-Claude reader (medium)

- `CLAUDE.md` is 311 lines, 27 KB, about 9,000 tokens loaded in every Claude session and every sub-agent. Roughly a
  third is Claude-specific (hooks, agent table, Monitor, main-loop rules) and a third restates method v2 and the
  pitfalls with dated anecdotes. Fix: keep in `CLAUDE.md` the Claude-specific part and one-line pointers; move the
  tool-neutral rules to `AGENTS.md` (single home, `CLAUDE.md` imports it with `@AGENTS.md`); move « Pièges connus »
  to `docs/pitfalls.md`. Target: under 150 lines.
- Language: instruction files are French, new documents English (by rule). Strong models cope; small free models
  follow French prose less reliably and the opencode prefix is English. `AGENTS.md` in English solves this without
  translating existing documents.
- Paragraph-long rules with several clauses and dates (for example `CLAUDE.md:85-111`) are hard to apply for a
  small model. In `AGENTS.md`, write one rule per line, imperative, with the command that proves it.

### 8. Scripts depend on machine state outside the repo (low)

`scripts/opencode-free.sh:19` needs `~/.local/opencode` and `~/.config/opencode/opencode.json`; Codex profiles live in
`~/.codex/*.config.toml`; keys in `mise.local.toml` (ignored, correct). A new machine or user account loses all of it.
Fix: list these prerequisites and their install commands in `docs/executors.md`. `scripts/claude-api.sh` is tracked
but referenced by no file (only by Claude's memory): document it there or remove it.

## Cleanup list (nothing was deleted)

| Path | Size | Safe to remove? | Proposed action |
|---|---|---|---|
| `.claude/worktrees/agent-a78adb31ac7116405` (OI) | 1.2 GB | No: 78 uncommitted files, the only copy of OI | commit OI, then remove |
| `.claude/worktrees/agent-a8b133d4e69426085` (PR1, locked) | 232 MB | No: 20 uncommitted files | commit PR1, then unlock and remove |
| `.claude/worktrees/agent-a4fe300c2db2a007f` | 88 MB | No: 3 modified test files | integrate or archive the diff, then remove |
| `.claude/worktrees/agent-ac6cc08b70cf28e5a` | 111 MB | Check: 3 untracked tests, compare with `b1ca1ef` | remove if identical |
| `.claude/worktrees/agent-abf6df91a052c3d0e` (locked) | 289 MB | Check: one scratch `zzrepro.test.ts` | keep the repro if the Markdown failure is still open |
| `.claude/worktrees/agent-a5b89fec0f26c7ef9`, `agent-abaa1fcac45bd62db` | 1.7 GB, 877 MB | Yes after `git status --porcelain --ignored` (26 and 11 ignored entries): clean, HEAD on `main` | remove |
| `../inkwell-push` | 1.0 GB | Clean, HEAD on `main`; it is the push checkout | keep one, or remove and document `qpush.sh` |
| 39 local branches (`worktree-agent-*`, `t11-*`, `fix/*`, `si1b`, …) and 18 remote ones | small | Verify each is merged (`git branch --merged main`) | delete merged ones; decide on the rest |
| `bin/` (`inkwell-server`, `inkwell-relay`) | 140 MB | Yes, rebuilt | `mise run clean` |
| `gates/L3-setup/` (ignored) | 59 MB | Probably; raw runs of a merged block, summaries cited in `.remember` only | archive outside the repo, then remove |
| other ignored gate logs (`gates/*/*.log`, `L0/`, `L2/`, `PB/*.json`) | about 6 MB | Yes for merged blocks; reports `*.md` are tracked | remove per merged block |
| `frontend/test-results/`, `frontend/dist/` | 24 MB, 5 MB | Yes, regenerated | `mise run clean` |
| `.playwright-mcp/` (3 files, header mock-up capture) | 188 KB | Yes once the HB1 design is recorded | remove |
| `.superpowers/` (brainstorm), `.task/` (Taskfile cache) | 64 KB, 92 KB | Yes | remove |
| `.remember/` (13 daily logs) | 6.3 MB | Not before finding 2 is fixed: it is today's only handover record | keep until then |
| `~/.cache/inkwell` (outside the repo) | 7.4 GB | Caches (`gocache`, `gocache-l3`, `codex-gocache`, WebKit) | `go clean -cache` per cache; `gocache-l3` looks orphaned |

Total reclaimable in and next to the repo: about 5.3 GB (4.9 GB of worktrees, 0.4 GB of ignored files), of which
about 2.6 GB is safe today and the rest only after OI and PR1 are committed. No orphan document was found in `docs/`.
`mise` tasks cited nowhere outside their definition: `align:check`, `ci:triage`, `fe:fmt`, `gen:check`, `llm:triage`,
`release:snapshot` (keep if used by hand, else remove). One deferred obligation is open and not overdue (L1, from
SB1b, `gates/deferred.jsonl`).

## Cold-start checklist for an AI without Claude's tools

1. Read, in order: `PROGRESS.md` lines 9-56, `docs/README.md`, `CLAUDE.md` (« Architecture », « Processus »,
   « Règles de travail », « Pièges connus »), `docs/plans/2026-11-12-method-v2.md`. Skip the Claude agent table.
2. Rebuild the real state; do not trust the status table alone: `git status --short`, `git worktree list`, then
   `git -C <worktree> status --short` for each, `git log origin/main..main --oneline`,
   `python3 scripts/gate.py --deferred`, the last rows of `docs/autonomous-decisions.md`.
3. Check for a running job before starting anything heavy: `pgrep -af 'gate.py|block_done|gremlins|go test'`.
   Never start a second gate in the same tree.
4. Run tests only through `mise run <task>` or `scripts/heavy.sh <command>` with a `timeout`; at most two heavy jobs.
5. Long task: `cmd > LOG 2>&1; echo "exit $?" >> LOG` detached, then `scripts/watch-log.sh LOG` until the exit line.
6. Never edit a committed `gates/<block>.toml`; never weaken a test, threshold or delay; never use `--no-verify`.
7. One block at a time, in its own worktree; follow `mise run done -- <block>` (steps 1 to 8 of the definition of
   done); record every decision in `docs/autonomous-decisions.md`.
8. Unless you are the session's named integrator: do not commit, stash, push, or remove a worktree. Report "already
   broken" only with a replay on a tree from before your change.
9. Before the integrator commits: review the staged diff (`/simplify` checklist, `gates/<block>/SIMPLIFY.md`), then
   write the marker; a different model family reviews complete-tier blocks. No session link in the message.
10. After a commit: update `PROGRESS.md` (table and open work trees), `docs/exigences.md`, push with
    `scripts/qpush.sh`, remove the block's worktree after `git status --porcelain --ignored`.
11. New text is in English; messages to the owner are in French, short, each understandable alone.
