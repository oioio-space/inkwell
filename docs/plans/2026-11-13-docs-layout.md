# DL — one home per kind of information (light tooling block, 2026-11-13)

**Need (user, 2026-11-12):** no documentation scattered left and right; know where every piece of data lives;
know at any moment where the project stands and what remains. Today the status lives in six places
(`PROGRESS.md`, `docs/objectif-en-cours.md`, `docs/architecture/README.md` "Où en est-on", the coherence §5
phasing, `docs/open-questions.md`, `docs/exigences.md` questions), with stale copies (C5S "gate running" in
the objective file after its delivery; "T11 en cours" in the architecture README), three entry points
(`docs/README.md`, `docs/architecture.md`, `docs/architecture/README.md`) and two review folders (read-only
inventory of 2026-11-12, file:line evidence, summarised here).

**Sources.** Diátaxis separates reference, explanation and how-to, but does not by itself prevent silos or
stale copies; the remedy used by docs-as-code projects is one canonical file per fact plus an index
([GitBook](https://www.gitbook.com/blog/how-to-structure-documentation),
[docs harmonisation plan, live status single source](https://ideony-docs.pages.dev/plans/2026-04-21-docs-harmonization)).
For agents, a short root instruction file that indexes context documents (progressive disclosure) beats a long
one ([agent-context-docs](https://skills.cat/skills/gwenwindflower/charmschool/agent-context-docs),
[lightweight CLAUDE.md](https://teamvincev5.pages.dev/blog/lightweight-claude-md-agents-md)).

**Constraint.** Committed gates are frozen and their overrides require each cited `docs/...` decision path to
exist (`scripts/gate-guard.py`); `scripts/gate.py` parses the `gate-lots` block and `scripts/block_done.py`
requires each block's name in `docs/objectif-en-cours.md`. So no gate-cited path is moved or deleted; a file
that loses its role becomes a short pointer that keeps its machine-read parts.

## Target: one home per kind of information

| Information | Single home | Not anywhere else |
|---|---|---|
| Rules for working | `CLAUDE.md` (index + invariants), method detail in `docs/plans/2026-11-12-method-v2.md` | — |
| Where we are, current block, ordered next list, everything that remains for R1, blockers, last deliveries | `PROGRESS.md` (ordered summary with links; one screen, then the stage checklists) | objective file, architecture README |
| Detailed residual checklist of one block | its plan in `docs/plans/` (PROGRESS links to it) | — |
| Block registry read by tools (`gate-lots`, block names) | `docs/objectif-en-cours.md`, reduced to that registry | — |
| Release scope (which lot belongs to MVP, v1, later) | `docs/architecture/coherence.md` §5 (no status column) | PROGRESS repeats only the order |
| Requirements and their state (one line each, no work lists) | `docs/exigences.md` | — |
| Open questions | `docs/open-questions.md` (exigences' questions moved there, decided rows closed with a link) | exigences, spec |
| Decisions | architecture → `docs/adr/`; doubts and amendments to accepted specs → `docs/architecture/decisions.md`; decisions still in force recorded in the objective file stay there (override paths, e.g. N42, native performance); durable autonomous choices → `docs/autonomous-decisions.md`; the user's product choices → `docs/design/decisions-utilisatrice.md` | journals |
| Plans and history | `docs/plans/` (dated); old journals → `docs/archive/` | PROGRESS, objective file |
| Reviews | `docs/reviews/` (`docs/quality/` merged in, path kept as pointer if cited) | — |
| Evidence | `gates/<lot>/`, `docs/perf/` | — |
| Map of all of this | `docs/README.md` (`docs/architecture.md` stays as the architecture deep-link page) | — |

## Steps (each one commit, `mise run lint:claude-md` and the gate self-tests green)

1. `docs/README.md` becomes the map above, with "to know where we stand, read PROGRESS.md".
2. `PROGRESS.md`: one-screen status at the top (current block, ordered next list, blockers, last five
   deliveries with commits), stage checklists corrected, the journal table moved to `docs/archive/`.
3. `docs/objectif-en-cours.md`: pure history of the journal (≈330 lines) and the stale "active lot" sections
   move to `docs/archive/2026-11-objectif-journal.md`, with an anchor link from each moved heading; what stays is
   the `gate-lots` registry (byte-identical, criterion IDs unchanged), one line per block name, and a section
   "Décisions en vigueur" with every rule still governing (each checked against `gates/guard-overrides.json` and
   the manifests citing the file). Its R1 items move to PROGRESS.
4. Open questions merged; architecture README "Où en est-on" replaced by a pointer; `docs/quality/` into
   `docs/reviews/`.
5. CLAUDE.md: the "Registre des exigences" paragraph points to the homes above; re-read whole file.
6. Lint guard, a separate check in `scripts/check_claude_md.py` (already in `ci`) with its own fixtures and the
   real file name in each finding: every top-level entry of `docs/` is the target of a valid link in
   `docs/README.md`, and every link there resolves. Dated folders (`plans/`, `perf/`, `adr/`) are indexed by
   their naming, not by hand. Tests in `scripts/check_claude_md_test.py` (`test:py`).
7. Not now: path-scoped `.claude/rules/` to slim CLAUDE.md (needs its own extraction plan; `HOT_PATHS:` and hook
   names must stay where the linter reads them).

**Done when:** each row of the table has exactly one live copy (checked by reading, listed in the commit),
`python3 scripts/gate.py --deferred`, `gate-selftest.py`, `lint:claude-md` and `test:py` pass, and one
review (Codex if present) accepts the plan before step 1 (Codex, 2026-11-12: 7 findings, all taken in this version).
