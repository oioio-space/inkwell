# EX0c3 - red witnesses

Recorded 2026-11-25 at step 1, before any production change. Tree: the worktree `inkwell-ex0c3` at `main` af69c6f,
plus the plan `docs/plans/2026-11-25-ex0c3-insert-format.md`, this draft gate and the PROGRESS line EX0c3, which now
points at the plan. Each command was run once, exactly as `gates/EX0c3.toml` writes it, from the worktree root and
through `sh -c`, which is how `scripts/gate.py` runs it. The logs are in the session scratchpad
(`witness-EX0c3-<id>.log`). No other heavy job ran; the load average stayed between 0.8 and 2.7.

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| docs | 1 | < 1 s | ADR 0027 has no EX0c3 section; spec §4.31 has no EX0c3 line naming `insert` or `format`; W83 does not name the plan. It found the plan's rows F-1..F-7, its site list and the PROGRESS line | yes (written with the code) |
| no-hard-locations | 1 | 0.2 s | 119 problems, all at plan sites. Tables: `syntaxOf`, `formatLanguage`, `EMPTY_PAIRS`, `READERS`, `PICTURES`, `MD_SYNTAX`, `MD_WORDING` (App, `editor/format/*`, the Insert menu); the Insert menu's `MenuButton` outside a host. Old drawings: `problems.csv(` in Sheet and problems, `lang !== "plain"` in App, `lang.${` in FormatBar and Language, `lang ===` in setup, FormatBar importing `../toolbar/InsertMenu`. Files: `ui/toolbar/InsertMenu.svelte`, `editor/format/{typst,markdown,lang}.ts` still exist; `ui/command/InsertMenu.svelte`, `ext/{typst,markdown}/native/format.ts`, `extensions/formatEntries.ts` absent. `insert` still reserved, its groups `blocks, files, data, extensions` without captions; no `inserts` key; the 13 commands and items absent; the grammars have no `format`. Type ids: the 23 lines of plan §3, among them `App.svelte:396` (leaves with `LANGUAGE_TYPE`) and the object keys `cleanup.ts:12,13` (the new key rule). No palette label is shared today. No false positive: the first draft's `ENTRIES` also matched `extensions/loader.ts` (4 lines, a loader table), so the rule now looks for `const ENTRIES` in the new host only; `ui/icons/paths.ts` (icon names) and `<style>` blocks are skipped | yes |
| lint | 0 | 4 s (9 s in a first probe) | `mise run lint`: gofmt, vet, golangci-lint, build clean | yes: a guard, green on the unchanged tree |
| test-py | 0 | 21 s | `mise run test:py`: 126 + 278 tests OK, gate self-test and guard self-test pass | yes: a guard, green on the unchanged tree |
| frontend | 1 | 312 s | Biome clean, svelte-check 0 errors 0 warnings, Vitest 1513 passed, 0 failed, 1 todo (tool chain exit 0); the 7 rule tests F-1..F-7 `absent` | yes (312 s of the 480 s cap) |
| mutants | 1 | < 1 s | MI1..MI7 missing; no `*format*` vector in `valid/`, none of `*plugin-format-entry*`, `*format-no-label*`, `*format-bad-accepts*`, `*format-bad-reader*`; the runner was not reached. `mutants.py --check`: 335 entries, 0 problems | yes (entries come at step 2) |
| e2e-mutant | 1 | < 1 s | MI8 absent from `scripts/netsim/mutants.toml` (kind `playwright`, `spec` ex0c3.spec.ts, `grep` EX0c3-E1, `error` stray, one stray `menuitem` in `ui/command/InsertMenu.svelte`); `mutants.py EX0c3 MI8` not reached. The kind exists since EX0c2 | yes |
| e2e | 1 | 71 s, then 65 s | `e2e:build` done; Chromium: 21 of 22 passed at their first attempt, 0 skipped. EX0c3-E1 and E2 `absent`. **EX0c1-E2 failed both runs** at `ex0c1.spec.ts:172` (the note: `items()` lacks `app.position`, `app.words`, `app.language` that `resolved(page, "status")` lists). Alone, at rest, after a fresh `e2e:build`: 8 of 8 passed (`--repeat-each 8`, 41 s) | E1, E2 yes. EX0c1-E2 is a red that is not EX0c3's: see open question 1 |
| e2e-webkit | 1 | 65 s, then 9 s | First run: the WebKit container never answered (`browserType.connect: ECONNREFUSED 127.0.0.1:37266`; `e2e:webkit` waits 60 s for its port, then goes on), so UI-05 failed without running. Second run: UI-05 passed at its first attempt; EX0c3-E1 and E2 `absent` | yes on the second run; the first is open question 2 |
| perf-start | 0 | 116 s | PERF-START `pass` at its first attempt, paired against the merge-base af69c6f: ratio 1.0006, bounds 0.989–1.012, half-width 1.17 %; budget `pass` (median bound 262 ms ≤ 350, p95 bound 274 ms ≤ 400) | yes: a regression guard, green on the unchanged tree |
| simplify-report | 1 | < 1 s | `gates/EX0c3/SIMPLIFY.md` absent | yes |

8 of 11 criteria are red for the right reason; `lint`, `test-py` and `perf-start` are guards, green on the unchanged
tree. `perf-start` rebuilds `frontend/dist` without `VITE_E2E_OBSERVE`: an e2e run after it needs `e2e:build` again
(the gate's `e2e` builds first, and runs before it).

**Environment-gated suites:** none of the replayed specs skips on an environment variable for the replayed titles
(`grep process.env` on the 12 specs: only `sheet.spec.ts:769`, SHEET-PERF behind `INKWELL_PERF`, not replayed).

## Caps (rule G-1)

30 + 30 + 300 + 120 + 480 + (20 + 300) + (20 + 240) + (200 + 420) + 150 + 300 + 30 = 2640 s ≤ 2700 s, and
`gate.deadline_problem("EX0c3", …)` returns None. The measured total is about 600 s (`frontend` 312 s, `perf-start`
116 s, `e2e` 71 s). Against EX0c2: `e2e` 420 s (measured 71 s for 22 tests; EX0c2's 480 s held W58a-UX-1e's 200 s,
not replayed here), `e2e-webkit` 150 s (covers the 60 s container wait seen once), `perf-start` 300 s (measured 116 s),
`mutants` 300 s for 7 mutants (EX0c2: 12 in 400 s); `lint` 300 s for a cold golangci-lint cache, `test-py` 120 s
(measured 21 s).

## Open questions (before the freeze)

1. **EX0c1-E2 is red in the replay set (2 of 2) and green alone (8 of 8).** The test reads the status bar's items and
   the resolved list one after the other, after a poll on another condition (`not.toContain("typst.problems")`). The
   cause is not proven: a rendering of the bar in two steps that the read falls between, or state left by an earlier
   spec of the set (`workers: 1`, one project per run). Step 2 names the cause with a trace before any edit; a
   polled read is a correct test fix only if the cause is that race; never drop EX0c1-E2 from the set (→ EX0c3).
2. **The WebKit container did not answer once in 60 s.** `e2e:webkit` carries on after its wait loop, so a slow start
   looks like a test failure. Measured once; to reproduce at rest before calling it the machine (→ EX0c3).
3. **Palette entries (plan §2, declared change):** the thirteen insert commands enter the palette, and a step item
   greys while the bar is hidden. The reviewer decides whether that is in scope or whether the palette waits (UX1-D).
4. **About 280 test call sites** pass `"typst"` or `"markdown"`; they move to `TYPST` and `MARKDOWN` from the testkit
   (mechanical, no assertion change), a large diff for the code review.

## Round 2 (after the blind review, `gates/EX0c3/REVIEW.md`): changed criteria only

Same tree, plus the review's fixes to the plan, the gate, `mise.toml` (`e2e:webkit`) and `scripts/codemods/ex0c3_type_literals.py`.
Each changed criterion was run once; logs `r2/witness-EX0c3-<id>.log` in the scratchpad. Load average 0.5–1.0.

| Criterion | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|
| no-hard-locations | 1 | 0.2 s | 136 problems: round 1's, less the three the review removed (`lang.ts` may stay: `Lang` keeps its name; the two grammar lines, reworded), plus 20 new: the 13 commands lack `kw.insert`, `kw.insert` is absent, the grammars have no `label` and no `format`, `insert.csvTable.markdown` still in `fr.json` and `en.json`, Markdown's `describe` absent, `readOnly` does not read `workspace.big`. The widened key rule found one other place, `sheet.svelte.ts:821,829` (the clipboard's `tsv` field), allowed on those lines only; no false positive left | yes |
| mechanical | 1 | 0.3 s | « no test file carries the codemod yet » (0 test files changed); 17 codemod files listed; vitest passed 1513 (round 1's report) | yes |
| mutants | 1 | < 1 s | MI1..MI7 and MI9 missing; the vector `*grammar-no-label*` replaces `*format-no-label*`; the runner was not reached | yes |
| test-py | 0 | 21 s | 126 + 278 tests OK, self-tests pass (rerun: `mise.toml` changed) | yes (guard) |
| e2e-webkit | 1 | 9 s | Container ready; UI-05 passed at its first attempt; EX0c3-E1 and E2 `absent` | yes |

`docs`, `lint`, `frontend`, `e2e`, `perf-start` and `simplify-report` did not change and were not rerun.

**Probes of `mechanical`** (on the worktree, restored with `git checkout -- frontend/src` after each):
`codemod.py roundtrip`: 17 files, 0 problems. After `codemod.py apply`, Biome's formatter and the three named edits:
exit 0. With one assertion flipped (`active.test.ts`, `toBe(true)` → `toBe(false)`): « not mechanical ». With a
doubled space inside a string (`commands.test.ts`): « not mechanical » (a first, whitespace-blind version passed it,
so the comparison now keeps strings and comments). With a test added to `palette.test.ts`: « its test titles
changed ». With the codemod alone, without the named edits of `locations.test.ts`: « not mechanical ».

**Probe of `e2e:webkit`'s wait**: with a `podman` on `PATH` that starts nothing, `mise run e2e:webkit` exits 1 after
63 s with « e2e:webkit: container not ready: nothing listens on 127.0.0.1:37266 after 60 s ».

Open questions 2 to 4 of round 1 are closed by the review (REVIEW minor 8, minor 5 and §2, major 1). Question 1 is
the plan's named step-2 item (REVIEW major 3).

Caps after the review: 2670 s ≤ 2700 s (`mechanical` adds 30 s); `deadline_problem` returns None.

## Before the freeze

- A blind review (Opus, plus Gemini if its quota allows) of the plan and of this gate (method v2), then the freeze.
- The rule tests, mutants MI1..MI8, vectors and `frontend/e2e/ex0c3.spec.ts` are written at step 2; their titles are
  fixed here.
