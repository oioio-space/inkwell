# inkwell — entry file for AI agents

Read this first if you are Codex, Gemini CLI, opencode, Cursor or any agent that does not load `CLAUDE.md` by itself.
It is a map and a tool-neutral summary: each rule has **one home**, named here; when this file and that home differ,
the home wins. Claude Code sessions read `CLAUDE.md` directly.

## What the project is

A collaborative desktop editor for Typst and Markdown, **without a server** (Wails v3, CodeMirror 6, Automerge,
go-libp2p). Go module `github.com/oioio-space/inkwell`, nested Go module `protocol/`, frontend in `frontend/`.
All tooling runs through **mise**: `mise run <task>` (`mise tasks` lists them; `mise run setup` installs the tools and
the git hooks).

## Where the truth lives

| Question | Single home |
|---|---|
| Current state, block in progress, what remains, who integrates | `PROGRESS.md` (top) |
| Where any kind of information is | `docs/README.md` (the map of `docs/`) |
| Working rules, invariants, definition of done, known pitfalls | `CLAUDE.md` (French; applies to every AI) |
| Method: tiers of rigour, reviews, absence of Codex or of Claude | `docs/plans/2026-11-12-method-v2.md` |
| The owner's requests and their status | `docs/exigences.md` |
| Choices made without asking her | `docs/autonomous-decisions.md` (append yours, newest last) |
| How each AI executor is launched, quotas, traps | `docs/executors.md` |
| Architecture, decisions, protocol | `docs/architecture.md`, `docs/adr/`, `docs/protocol/` |
| Engineering principles and their proofs | `docs/engineering-principles.md` |

## Cold start (do this before changing anything)

1. Read, in order: the top of `PROGRESS.md`, `docs/README.md`, `CLAUDE.md` (« Architecture », « Processus »,
   « Règles de travail », « Pièges connus »), `docs/plans/2026-11-12-method-v2.md`. The agent table of `CLAUDE.md`
   (« Routage des modèles ») names Claude Code sub-agents; for other executors see `docs/executors.md`.
2. Rebuild the real state; do not trust a status table alone: `git status --short`, `git worktree list`, then
   `git -C <worktree> status --short` for each, `git log origin/main..main --oneline`,
   `python3 scripts/gate.py --deferred`, the last rows of `docs/autonomous-decisions.md`.
3. Check for a running job before anything heavy: `pgrep -af 'gate.py|block_done|gremlins'` (never put the pattern
   of your own command line in a `pkill -f`). Never start a second gate in a tree where one may be running.
4. Know your role for this session: **integrator** (named in `PROGRESS.md`, see « When Claude is absent » in method v2)
   or **delegate**. A delegate never commits, stashes, pushes or removes a worktree.

## Invariants (home: `CLAUDE.md` « Architecture »; changing one needs a new ADR)

- The CRDT lives in the webview; Go carries opaque bytes and never imports Automerge.
- Peer sync goes through the `Transport` interface of `internal/transport`; only `internal/transport/p2p` and
  the `protocol` module's libp2p adapter (ADR 0034, step 4) import libp2p.
- The protocol is public (`docs/protocol/`); the `protocol` module imports nothing from inkwell.
- One source of truth per thing: text in Automerge, binaries in the BLAKE3 store, members in the signed list, versions
  in go-git.
- Every byte from a peer is verified (signature, role) and its decoder is fuzzed.
- Webview ↔ Go bridge is Wails Streams, no local port; any other local server binds `127.0.0.1` with a token.
- Typst never downloads at run time; `typst` and `tinymist` are pinned and re-verified (SHA-256).
- "pair" in code, "personne" in the interface; no jargon in the UI. One package, one role; no empty package.

## Process (home: `CLAUDE.md` « Processus », method v2)

- **Definition of done**, eight ordered steps, checked by `mise run done -- <block>`: executable spec and frozen gate
  first → TDD implementation → `/simplify` review recorded in `gates/<block>/SIMPLIFY.md` → fast checks, formal, tests
  → mutation on touched `protocol/*` packages → full gate → documents → the integrator commits.
- **Frozen gates:** `gates/<block>.toml` is committed before the code and never edited afterwards. Changing a
  criterion is a recorded contract change (method v2 rule 9), not an edit.
- **Never weaken** a test, a threshold or a delay to make something pass. Never use `--no-verify`.
- **Golden rule:** fix the root cause once, simply. When fixes pile up, stop, restate the problem, re-specify (spec,
  model, mutants, new gate), then do it properly.
- **Rigour by risk** (method v2 rule 4): "complete" for the closed list (peer wire and state, trust and admission,
  sandbox boundary, bytes from a peer, three or more concurrent actors on durable state); otherwise standard or light.
- **One block at a time**, in its own worktree, committed before the next starts. Remove a worktree only after
  `git status --porcelain --ignored` shows nothing to keep.
- **Research first:** a non-trivial technical decision cites current external sources and is written in `docs/plans/`
  or `docs/adr/`.
- **Requests:** every request of the owner is added to `docs/exigences.md` and placed in the plan
  (`docs/architecture/coherence.md` §5) when it arrives.
- **Language:** everything newly written in the repository is in English; existing French documents stay French;
  messages to the owner are in French, short, each understandable alone.

## Working without Claude Code's hooks

Claude Code enforces several rules with hooks (`.claude/hooks/`). You have none: apply them yourself.

**Memory (three machine-wide out-of-memory crashes came from uncapped tests).** Run tests only through a capped mise
task or through `scripts/heavy.sh`, always with a `timeout`; never a bare `go test`, `pytest`, `vitest` or `pnpm test`:

```bash
mise run test                                             # Go unit tests (capped)
mise run test:py                                          # Python tooling tests (capped)
mise run fe:check                                         # Biome, svelte-check, Vitest (capped)
timeout 20m scripts/heavy.sh go test ./internal/blob/...  # any other test command
cd protocol && timeout 20m ../scripts/heavy.sh go test ./...   # nested module: run it from protocol/
```

At most two heavy jobs on the machine (`heavy.sh` queues the rest, 8 GB cap each, no swap); at most four agents in
parallel; mutation testing one package at a time (`mise run mutate:go -- ./internal/trust`). A measurement (native
performance, netsim, full gate) has the machine to itself: start nothing heavy while one runs.

**Cheap checks first, after each edit and before any gate** (they replace the post-edit hook):

```bash
gofmt -l . && go vet ./<package>/...   # after editing Go
mise run lint                          # Go style gate
mise run lint:env                      # every variable read is declared in .env.reference
mise run lint:claude-md                # everything the instruction files cite exists; docs/ is mapped
mise run fe:check                      # after editing the frontend
```

Then the inner loop of method v2 rule 5: tests of the touched packages and their consumers, a 20–30 min smoke, and the
frozen gate **once** on the final candidate (`mise run gate -- <block>`; after an incident
`mise run gate -- --resume <block>`). A gate never runs another gate: nested ones are ledgered in
`gates/deferred.jsonl`.

**Long tasks (over 2 minutes, or in the background) are logged and watched, never waited on blindly:**

```bash
setsid nohup bash -c 'mise run gate -- <block> > LOG 2>&1; echo "exit $?" >> LOG' & disown
timeout 10m scripts/watch-log.sh LOG    # repeat until it prints the final "exit N" line
```

`watch-log.sh` reports progress, every failure signature and stalls. A silent log is not proof of death: check with
`pgrep -af` before starting anything again in the same tree. A non-zero exit is always reported, never swallowed.

**Other hook rules, by hand:** never edit a committed `gates/<block>.toml`; a decision is written with its sources
where plans and ADRs live; a claim such as "already broken" or "flaky" is accepted only with a replay on a tree from
before the change.

## Commits and reviews

Home: `CLAUDE.md` « Définition de fini » step 8 and method v2 (« When Codex is absent », « When Claude is absent »).

- Only the session's **integrator** commits, after reading the whole staged diff. With a Claude Code session, that is
  Claude; without one, method v2 says who.
- The pre-commit hook (`.githooks/pre-commit`) runs for everyone: cleanup, secrets, vulnerabilities, Biome, Go style,
  then the `/simplify` attestation: a marker holding the hash of the exact staged diff. Write it only after the review
  it attests (checklists: `.claude/skills/go-style-guide`, `vuln-guard`, `secret-guard`, `helper-ergonomics`; findings
  in `gates/<block>/SIMPLIFY.md`).
- Implementation and its review never come from the same model family; output of small or free models (Sonnet, Haiku,
  open-weight) is always re-read by a strong model before it counts.
- Commit messages: Conventional Commits, in English, no session link (no `Claude-Session:` line, no chat URL).
- After a commit: update `PROGRESS.md` and `docs/exigences.md`, push with `scripts/qpush.sh` (the pre-push hook runs
  `mise run ci`), remove the block's worktree.

## Known traps

Home: `CLAUDE.md` « Pièges connus ». Read it before running a native measurement, mutation testing, or deleting any
directory or worktree.
