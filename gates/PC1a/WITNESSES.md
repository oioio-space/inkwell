# PC1a - red witnesses

Red witnesses recorded 2026-11-15 after the freeze commit 0355f4d; the file's first line still reads DRAFT because a frozen gate is never edited — the freeze is that commit.

Each command was run once, exactly as written in `gates/PC1a.toml`, from the repository root, one at a time. Logs: scratchpad `witness-PC1a-<id>.log`.

| Criterion | Exit | Reason read in the output | Right reason? |
|---|---|---|---|
| docs | 1 | `PC1a docs: ['fr.json foreign.takeOver', …, 'ADR 0043 status line: part 2 PC1a not accepted']` — catalogue keys absent, ADR 0043 not accepted; the plan rows pass | yes |
| go | 1 | `missing or not passing: TestTakeOverRequestWritesNothing` (the go test run itself exits 0: the named test does not exist yet) | yes |
| gate-deadline | 1 | `ModuleNotFoundError: No module named 'scripts/gate_deadline_test'` (`scripts/gate_deadline_test.py` absent) | yes |
| mutant-tool | 1 | `PC1a mutant entries missing or without test and expect: ['MW3', 'MW4', …, 'MW13']` (the mutants_test.py run itself is OK) | yes |
| mutants | 1 | `netsim mutants: no mutant for block PC1a` | yes |
| frontend | 1 | `PC1a frontend: exit 0, failed 0, missing ['PC1 not a member: one sentence, two actions', …]` (Vitest passes; the named PC1 tests are absent) | yes |
| e2e | 1 | `PC1a e2e: exit 1, stats {… 'expected': 0 …}, missing ['PC1-E2E the take-over restarts Inkwell on the same document, writable', …]` (`e2e/project-change.spec.ts` absent, 0 tests ran) | yes |
| regression | 124 | `[fe:check] ERROR sh exited with non-zero status: no exit status` after `Finished in 180.04s`: the 180 s cap of `fe:check` expired; no PC1 reason shown | NO — timeout of the check, not a PC1a reason |
| restart-barrier | 1 | `missing or not passing: TestRestartBarrier` | yes |
| no-live-change | 1 | `PC1a live change: ['takeOver not found']` (`internal/app/invites.go` has no `takeOver`) | yes |
| tla | 0 | exit 0: the four sound configurations and the 20 mutants/witnesses already pass on the current tree (`scripts/tla-check.sh`, `INKWELL_TLA_ONLY='^ProjectChange'`) | NO — passes, criterion is not red |
| ux-walkthrough | 1 | `test -s gates/PC1a/UX.md` fails (file absent) | yes |
| simplify-report | 1 | `test -s gates/PC1a/SIMPLIFY.md` fails (file absent) | yes |

Count red for the right reason: 11 of 13. NO: `regression` (timeout at the 180 s cap of `fe:check`, not a PC1a reason) and `tla` (exit 0, passes).

## Integrator notes (2026-11-15)

- `tla` passes before the code by design: the TLA+ model is the specification written first (definition of done,
  step 1); its mutants are what must be red, and they are caught (`docs/protocol/tla/README.md`). Right reason: yes.
- `regression` is **not passable as frozen**: its `fe:check` cap is 180 s, while `fe:check` alone on an idle machine
  takes 306.9 s (measured 04:53, `mise run fe:check`, exit 0); the other gates give it 1500 s. A spec error of the
  integrator, not a weakening. A frozen gate changes only on the user's word (`.claude/hooks/gate-freeze.sh`):
  decision requested — raise the cap to 1500 s as in `gates/SB2a1.toml`.

## Gate changed on the user's word (2026-11-15, 09:03)

The user accepted the integrator's recommendation: in `regression`, the frontend part is now Biome, svelte-check and
`vitest run --changed <merge-base with main>` (the tests related to the files the block changed), capped at 180 s,
plus 30 s for the bindings; the e2e part is unchanged. Caps now sum to 2690 s of the 2700 s allowed. The whole
suite still runs in `mise run ci` at every push. No other criterion changed.
