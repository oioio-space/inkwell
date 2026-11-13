# Skills, hooks and CLAUDE.md review (2026-11-13)

Complements `2026-11-13-ai-handover-audit.md` (not repeated here: gopls disabled, P/E placement, Codex row,
`next` branch, stale comment in `pre-commit-simplify.sh`, the 150-line target). Read-only review of `main` at
`2a80a62`. Every verdict below marked "ran" comes from feeding the hook a sample JSON payload on stdin and
reading its exit code or JSON; nothing was committed and no test suite was run. `bash -n` passes on all nine
hook scripts, `shellcheck -x` reports only SC1091 (info), `mise run lint:claude-md` exits 0.

## Findings, ranked by impact

### 1. The edit path the global rules mandate bypasses every edit hook (high)

- `~/.gortex/instructions/active.md` says "Mutate only with `edit` or `refactor`" (gortex). The three project
  edit hooks are registered on `Write|Edit|MultiEdit` only (`.claude/settings.json:31,46`), and they read
  `.tool_input.file_path`, which `mcp__gortex__edit` does not carry (`target.file`, `changes[].path`, or a symbol
  id). Ran: a gortex-shaped payload naming a committed gate goes through `gate-freeze.sh` with exit 0.
- So a gortex edit gets no gate freeze, no gofmt/vet/Biome check, no research reminder. It is also the tool that
  wrote into the main checkout instead of the agent's worktree twice today: nothing compares the edit target with
  the caller's `cwd`. Five agents carry `mcp__gortex__edit` (`go-dev`, `formal-modeler`, `quality-runner`, `scribe`,
  `ux-tester`), Haiku ones included.
- Fix: one new PreToolUse hook on `mcp__gortex__(edit|refactor)` that (a) resolves the target path (`target.file`,
  `changes[].path|source|destination`) and applies `frozen()`; (b) denies when the payload `cwd` is a linked
  worktree and the call has neither a `view` nor an absolute path under that `cwd`. Add the same matcher to
  `post-edit-check.sh` (PostToolUse), reading the same fields. Remove `mcp__gortex__edit` from `quality-runner`
  and `scribe` unless they need it.

### 2. `gate-freeze.sh` does not protect a gate inside a worktree (high)

- `gate-freeze.sh:8` takes the root from `CLAUDE_PROJECT_DIR` (main); `:13` requires the path relative to that
  root to match `^gates/[^/]+\.toml$`. Ran: Edit of `<main>/gates/C5S.toml` exits 2; Edit of
  `.claude/worktrees/agent-a8b1…/gates/C5S.toml` (same committed gate) exits 0. Blocks are implemented in
  worktrees by rule, so the freeze is off exactly where code is written.
- Bash side, ran: blocked `sed -i`, `echo x > gate`, `git rm`; allowed `echo x >gate` and `>>gate` (the pattern
  needs a space after `>`), `git checkout HEAD~3 -- gate`, `git restore --source`, `truncate`, `perl -pi`,
  `python3 -c "open(...,'w')"`. False positive: `cp gates/C5S.toml /tmp/x` (the gate as source) is refused.
- Fix: derive the root from the file (`git -C "$(dirname "$path")" rev-parse --show-toplevel`, as
  `post-edit-check.sh:12` already does). Replace the Bash regular expression by a state check: a small PostToolUse
  hook on Bash that runs `git status --porcelain -- 'gates/*.toml'` in the payload `cwd` and fails when a tracked
  gate is modified or deleted. It catches every write method and has no false positive on reads.

### 3. `heavy-guard.sh` is bypassed by ordinary command shapes (high: three OOMs came from this class)

Ran 43 commands through it. The runner must be in "command position" behind a fixed prefix list
(`heavy-guard.sh:12`), and any occurrence of `heavy.sh` anywhere in the text whitelists the whole command (`:19`).

| Allowed although uncapped | Refused although light |
|---|---|
| `taskset -c 12-19 go test ./...` (the form CLAUDE.md « Mémoire » asks agents to use) | `go test -c …`, `go test -list …` |
| `for p in a b; do go test ./$p; done`, `if …; then go test …; fi` | `pnpm exec biome check src/a.ts` |
| `time`, `nice`, `xargs`, `command`, `exec`, `sudo`, `rtk proxy` before `go test` | |
| `bash -c "go test ./..."` (quoted text is stripped) | |
| `timeout -k 5 60 go test …` (option with a value) | |
| `ls scripts/heavy.sh; go test ./...` and `go test ./... # heavy.sh` | |
| `node_modules/.bin/vitest run`, `make test`, `task test` | |

- Fix (root cause, also covers Codex and opencode, finding 4 of the handover audit): a `go` wrapper first on the
  agent `PATH` that re-executes `go test` under `scripts/heavy.sh` unless `INKWELL_CAPPED=1` is already set by
  `heavy.sh`/`memcap.sh`; same for `vitest` and `pytest`. The hook then becomes a courtesy message. Until then:
  match the runner anywhere after stripping quotes (drop the command-position prefix list), test the allow-list
  per pipeline segment instead of on the whole string, and exempt `go test -c|-list` and `biome check`.

### 4. Five skills describe hooks that no longer exist; the linter cannot see it (medium)

- `go-style-guide/SKILL.md:13-16`, `secret-guard/SKILL.md:13-15`, `vuln-guard/SKILL.md:15-16`,
  `repo-janitor/SKILL.md:13-14` all say "the PreToolUse hook injects this checklist"; `helper-ergonomics/SKILL.md:87`
  cites `commit-ergonomics-review.sh`; `scribe.md:14` says "hook injects it". No such hook is registered: the only
  commit hook prints one line naming the checks (`pre-commit-simplify.sh:27-34`). A model that trusts the skill
  waits for a checklist that never comes.
- `scripts/check_claude_md.py` validates `CLAUDE.md` only; `post-edit-check.sh:31` runs it when a skill or agent
  is edited, where it checks nothing about that file. `secret-guard:36-37` cites `.gitleaks.toml` /
  `.gitleaksignore` (neither exists).
- Fix: replace each sentence by "run this checklist on `git diff --cached` at step 3 (`/simplify`)". Extend
  `check_claude_md.py` to apply its path, task, hook and agent checks to `.claude/skills/*/SKILL.md` and
  `.claude/agents/*.md` (same functions, more input files).

### 5. `inkwell-stack` is partly stale and contradicts itself (medium: it is the "never code from memory" skill)

| Line | Says | Repo |
|---|---|---|
| 9 | "Wails v3 is beta and APIs move" | `go.mod:19` and `mise.toml:32` pin `v3.0.0`, `@wailsio/runtime` 3.0.0 |
| 18 | sidecars pinned in a "hash file next to the embed (étape 3)" | `internal/sidecar/manifest.json` (typst 0.15.1, tinymist) exists |
| 24 | Go docs through the gopls MCP | server disabled in `.claude/settings.local.json` |
| 44 vs 58 | "a bare `ip:port` cannot be dialed" vs "always offer `ip:port`" | contradictory |
| 44 | "Only `internal/transport/p2p` imports libp2p" | CLAUDE.md adds `protocol/libp2p`; `go.mod:128` replaces go-libp2p by a fork, not mentioned |
| 50 and 54 | remote changes out of CodeMirror history | same trap twice |

Fix: correct the six rows; point the version table at `manifest.json`; add the fork to the traps. `go-dev` does
not preload this skill (`go-dev.md` preloads `go-style-guide`, `use-modern-go` only), so the rule depends on the
description firing; add it to `go-dev` or state in the brief template that stack code needs it.

### 6. `pre-commit-simplify.sh` checks the wrong repository for worktree commits (medium)

- It uses the payload `cwd` (`hooklib.sh:17`), not the directory the command acts on. Ran, with `cwd` = main:
  `cd <worktree> && git add -A && git commit` is denied with an instruction to write main's marker
  (`.git/claude-simplify-ok`) from main's staged diff, which is empty (hash `da39a3ee…`). Following it satisfies
  the hook, then `.githooks/pre-commit:70-80` refuses in the worktree with the other marker path: two rejected
  attempts per commit. `git -C <dir> commit` is not recognised as a commit at all (`hooklib.sh:35` accepts only
  dash options between `git` and `commit`), so that form skips the Claude hook.
- "Marker in a separate call" is structural: a PreToolUse hook runs before the command, so `marker && commit` in
  one call is always denied (ran). This is intended friction but only prose explains it.
- Fix: delete the hook's own marker check. Keep the hook as a reminder only (emit `additionalContext` with the
  checks, never deny) and let `.githooks/pre-commit` be the single enforcement point: it runs in the right
  repository by construction, for every tool. One rule, one place, no double rejection.

### 7. Skill token cost and triggers (medium)

- `use-modern-go` is 11.2 KB (about 3,400 tokens) and is preloaded into every `go-dev` and `go-reviewer` launch,
  on top of `go-style-guide` (3.5 KB) and CLAUDE.md. Lines 33-147 (Go 1.0 to 1.23, a third of the file) list idioms
  the `modernize` linter already refuses. Lines 14-22 order the model to print a sentence about the Go version and
  to ask the user which version to target, against the autonomy rule. Its version probe (`:10`) greps the whole
  tree, worktrees included (18 `go.mod` found, 142 ms). Fix: keep 1.24 to 1.27 only, drop the "Say/Ask" block,
  read the version from `go.mod` at the root. Saves about 1,300 tokens per Go agent launch.
- `helper-ergonomics` (4.9 KB) is copied from a library project: `myproj.RunFile`, `RecoverFile`, a `defaults`
  side-effect import, "used as a Go library and as a CLI" (`:9-30`). inkwell is a desktop application. Its trigger
  ("adding or changing exported Go API… and before committing") fires on most Go work. Its routing section
  (`:76-85`) restates the CLAUDE.md table. Fix: cut to the eight precepts (about 25 lines), restrict the trigger to
  "exported API of `protocol/*` or a `cmd/` flag".
- `research-grounding` skill duplicates the CLAUDE.md « Research-first » paragraph and names the
  `microsoft-docs` MCP, irrelevant here; trigger "ANY non-trivial question" is too broad. Keep one home (the
  skill), leave one line in CLAUDE.md.
- `go-benchmark:23-34` mixes `b.Loop()` with `b.ResetTimer()` and a `sink` (both unneeded with `b.Loop`), and omits
  what CLAUDE.md requires: `-count` ≥ 10, interleaved paired runs, `HOT_PATHS`, a quiet machine.
- `goroutine-leak-check:95` says the project is "pure Go / no-CGO (see CLAUDE.md)"; CLAUDE.md says cgo is
  required. Its run commands should name `scripts/heavy.sh`.
- `new-gate` has `disable-model-invocation: true`, so the model cannot invoke the skill CLAUDE.md step 1 sends it
  to, while the autonomy rule says not to wait for the user. Its template still lists
  `docs/objectif-en-cours.md` in `raw_results` and a nested `mise run gate -- <previous block>` criterion (method
  v2: no gate launches another; 15 gates still carry one), and step 4 says "show the gate to the user". Fix:
  allow model invocation; template uses the deferred-obligation form; step 4 becomes "other-family review, then
  commit alone".
- Unused by CLAUDE.md: `readme-author`, `goroutine-leak-check`, `repo-janitor` (cost: description only, about 80
  tokens each per session). `readme-author` (5.8 KB) is preloaded into every `scribe` launch for PROGRESS and
  commit-message work; remove the preload.
- Thirteen descriptions total 3.9 KB, about 1,100 tokens per session; trimming each to one sentence saves 500.

### 8. Smaller hook defects (low to medium)

- `post-edit-check.sh:27-28`: Biome is skipped silently when `frontend/node_modules` is absent, which is the
  case in 4 of 8 worktrees (ran: exit 0 in 41 ms, no check). Fix: fall back to the main checkout's
  `node_modules/.bin/biome`, or print one line "frontend check skipped".
- `long-task-monitor.sh:10` fires on every `run_in_background` call, however short (150 tokens each), and stays
  silent for the detached form CLAUDE.md prescribes for gates (`setsid nohup … & disown`; ran: no output) and for
  a foreground `mise run gate|done|ci`. Fix: trigger on the command (`gate`, `done`, `ci`, `mutate`, `-fuzz`,
  `nohup`, `setsid`) instead of on the flag.
- Missing tools: `heavy-guard.sh:8`, `gate-freeze.sh:9` and `post-edit-check.sh:9` call `jq` unguarded under
  `set -e`; without `jq` they exit 127, which Claude Code treats as a non-blocking error: all three guards fail
  open. Fix: `command -v jq >/dev/null || { echo "jq missing" >&2; exit 2; }` in the two guards. `.githooks/pre-push:33`
  skips CI with a warning when `mise` is absent: make it exit 1.
- No hook has a `timeout` in `settings.json`; `post-edit-check.sh:23` allows `go vet` 120 s. Set explicit values.
- `.githooks/pre-commit:6-17`: the header lists a "Pure-Go (no CGO) gate" that does not exist and omits Biome.
- The global gortex hook answered two read-only shell calls of this review (a `find` over `.claude/skills`, a
  `git ls-files '*.go' | awk`) with a 200-token "Do not Grep indexed source" block. Not a project hook; report
  upstream or narrow its pattern to commands that read file content.
- Performance is not a problem: per call, measured over 10 runs, `heavy-guard` 21 ms, `gate-freeze` 42 ms,
  `pre-commit-simplify` 39 ms, `long-task-monitor` 39 ms, `research-grounding` 19 ms; `post-edit-check` 83 ms on
  `internal/trust`, 236 ms on `internal/app` (warm cache), 91 ms for the CLAUDE.md lint. Half of each is the `jq`
  mise shim; calling `jq` once per hook instead of two or three times would halve it.

## CLAUDE.md

Measured: 311 lines, 4,160 words, 27,335 bytes, about 9,000 tokens, loaded by the main session and by every
sub-agent (a Haiku `explorer` pays it too). Always-loaded companions: `MEMORY.md` 6.0 KB, gortex profile 4.3 KB,
`RTK.md` 1.0 KB, skill descriptions 3.9 KB: about 5,000 more.

- **Order.** The invariants, the part a model must never get wrong, start at line 57, after 15 lines of hook
  inventory and 25 of commands. « Processus » (7.4 KB) and « Règles de travail » (7.2 KB) are 53 % of the file.
  Proposed order: invariants; hard safety rules (memory cap, frozen gate, who commits); definition of done;
  commands; pointers.
- **Redundancy.** Lines 84-113 restate `docs/plans/2026-11-12-method-v2.md` while calling that file the single
  source. Lines 5-15 restate what each hook prints when it fires. « Research-first » is also a skill and a hook.
  The routing table restates every agent's own header.
- **Never enforced.** "≤ 4 agents in parallel", "one worktree per active block" (8 exist), "tooling ≤ 20 % of the week's commits", "progress point every
  5 minutes", "never write the commit keyword in another command". The last one is stale: ran, `grep "git commit"
  docs/` and `echo "… git commit"` are not matched by `hooklib.sh:35`; only an unquoted trailing occurrence is.
- **Contradiction with the global profile.** Line 279 recommends targeted `grep`, `head`, `tail`; the gortex
  profile loaded just before forbids "Read/Grep/Glob or shell equivalents" on indexed source. State once which
  files are outside the rule (Markdown, TOML, shell, logs).
- **Unclear for a model.** Fifteen dated anecdotes sit inside rules; rules of 60 to 90 words with nested
  parentheses (lines 166, 115-120, 258); "(comme simplify)" at line 267 has no referent.
- **Language.** CLAUDE.md is French; hooks, skills, agents and method v2 are English. A rewrite is new text, so
  by the file's own rule it should be English. Accented French costs more tokens per word than English: expect a
  further 10 to 15 % (estimate, not measured).

### Five rewrites (before is the current line range; after is complete)

**A. Hook inventory, lines 5-15, 1,120 bytes → 190.**
After: `Guards: git hooks in .githooks/ (clean, secrets, vulns, Biome, Go style, /simplify marker) and Claude
hooks in .claude/hooks/ (each prints its rule when it fires). --no-verify: emergencies only.`

**B. Method v2 summary, lines 84-113, 2,968 bytes → 620.**
After:
```
Method v2 (docs/plans/2026-11-12-method-v2.md is the rule; read it before starting a block):
- One block in implementation, one in verification; one worktree per block; commit before the next starts.
- Pick the rigour first: complete (peer bytes, trust, sandbox boundary, ≥3 concurrent actors), else standard or light.
- Other-family review of spec and gate before the freeze, and of the code after the smoke; two rounds at most.
- Loop: minutes on touched packages → 20-30 min smoke → the frozen gate once. A gate never launches another gate.
- Never drop a proof to fit a budget: split the block.
```

**C. Long tasks, lines 213-231, 1,771 bytes → 520.**
After:
```
Long task (> 2 min or background): always watched.
1. `cmd > LOG 2>&1; echo "exit $?" >> LOG` (LOG in the scratchpad); detached with `setsid nohup` if it can outlive the session.
2. Arm Monitor on `scripts/watch-log.sh LOG`; re-arm at each expiry until the exit line.
3. Report each step and any failure at once; never poll with sleep.
4. Before a second run in the same tree, prove the first is dead: `rtk proxy pgrep -af <name>`.
```

**D. Routing and main loop, lines 250-286, 3,632 bytes → 900.**
After: keep the rule sentence; replace the 12-row table by `Agents and their model: .claude/agents/ (each header
fixes model and effort). External executors (llm.sh, Codex, opencode, claude-or.sh): docs/executors.md.` and keep
four main-loop lines: orchestrate, do not debug or read logs; never re-read a file in context; briefs short,
reports ≤ 10 lines; user messages in French, each understandable alone, push with `scripts/qpush.sh`.

**E. Known pitfalls, lines 288-311, 2,398 bytes → 330.**
After: move the section to `docs/pitfalls.md` and keep:
```
Pitfalls (details and history: docs/pitfalls.md): native measure needs an idle machine; mutate `protocol` from
`protocol/`; before removing a worktree run `git status --porcelain --ignored`; one worktree per block (each costs
gortex a full graph); fake I/O with io.BytesIO, never a MagicMock.
```

Also: drop the dates and origin stories from « Mémoire » (1,427 → about 800 bytes) and the parentheticals
from « Définition de fini » (3,231 → about 2,300).

**Saving.** A to E: 11,889 → 2,560 bytes, 9,330 bytes saved; with the two smaller trims about 10,900 bytes, that
is about 3,500 tokens of 9,000 (39 %) per session and per sub-agent launch. With finding 7 (skills) a `go-dev`
launch drops by about 4,800 tokens. Nothing is lost: each removed passage already has a home that is cited.

## Suggested order of work

1. Findings 1 and 2 (one new hook, one corrected root): they close the two incidents of today.
2. Finding 3 (`go` wrapper on `PATH`): closes the memory rule for every executor.
3. Finding 6 (single enforcement point for the marker), finding 8 (`jq` fail-closed, long-task trigger).
4. Findings 4, 5, 7 (skills) with the linter extension, then the CLAUDE.md rewrites A to E.
