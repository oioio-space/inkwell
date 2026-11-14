# W58a - red witnesses before the freeze (2026-11-14; worktree on main 5eeb32d + the revision-6 implementation of W58)

Each command that can run before the block's changes was run once; it must fail for the reason its rule names.

| Criterion | Exit | Reason read in the output |
|---|---|---|
| docs | 1 | W58a docs: ['fr.json still has foreign.takeOver', 'fr.json still has foreign.takeOver.confirm.title', 'fr.json still has foreign.takeOver.confirm.body', 'fr.json still has foreign.takeOver.done', 'fr.json still has foreign.takeOver.failed', 'foreign.notNamed does not say the text can be copied', 'ADR 0043 part 1 not accepted'] |
| go | - | not run: it needs the block's renamed tests (titles « W58a … », `TestReplaceFailureAtEachStep`, mutant block `W58a`); its embedded checker parses after TOML decoding |
| mutant-tool | 1 | ........... ---------------------------------------------------------------------- Ran 11 tests in 0.000s  OK W58a mutant entries missing or without test and expect: ['MW1', 'MW2', 'MW3', 'MW4'] |
| mutants | - | not run: it needs the block's renamed tests (titles « W58a … », `TestReplaceFailureAtEachStep`, mutant block `W58a`); its embedded checker parses after TOML decoding |
| frontend | - | not run: it needs the block's renamed tests (titles « W58a … », `TestReplaceFailureAtEachStep`, mutant block `W58a`); its embedded checker parses after TOML decoding |
| e2e | - | not run: it needs the block's renamed tests (titles « W58a … », `TestReplaceFailureAtEachStep`, mutant block `W58a`); its embedded checker parses after TOML decoding |
| regression | - | not run: it needs the block's renamed tests (titles « W58a … », `TestReplaceFailureAtEachStep`, mutant block `W58a`); its embedded checker parses after TOML decoding |
| ux-walkthrough | 1 | (file absent or pattern not found) |
| simplify-report | 1 | (file absent or pattern not found) |
| no-take-over | 1 | workspace/workspace.svelte.ts: .pause(', 'frontend/src/workspace/workspace.svelte.ts: .resume(', 'frontend/src/ui/people/AccessBanner.svelte: takeOver', 'frontend/src/ui/people/AccessBanner.svelte: take-over', 'frontend/src/ui/people/AccessBanner.svelte: <dialog', 'internal/app/session.go: TakeOver', 'internal/app/invites.go: takeOver', 'internal/app/persist.go: trust-archive'] |

The e2e and frontend checkers keep the form of the frozen `gates/W58.toml`: TOML decodes `\n` in a basic string into a newline (measured with `tomllib`; each embedded program compiles).

After Codex's review (« Round 9 »): `mutant-tool` re-run (exit 1, entries MW1..MW4 missing); `e2e` and `regression` now wrap `e2e:build`, `fe:check` and `e2e` in `scripts/heavy.sh` (not run: they need the block's tests); the `frontend` checker gained the lock title and its mutant and still compiles.
