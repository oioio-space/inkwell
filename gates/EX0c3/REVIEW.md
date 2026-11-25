# EX0c3 - blind pre-freeze review: « freeze after fixes »

Reviewed: `docs/plans/2026-11-25-ex0c3-insert-format.md` and `gates/EX0c3.toml` (step 1 draft, 2026-11-25). Every
finding was applied as listed. The changed criteria were rerun once each (`gates/EX0c3/WITNESSES.md`, round 2).

## Major

| # | Finding | Change |
|---|---|---|
| 1 | The ~315 test edits were trusted to be mechanical, on review by eye | `scripts/codemods/ex0c3_type_literals.py` writes them: 17 test files, about 325 lines, from a closed table of exact token pairs (`"typst"`, `"markdown"`, `"plain"` → `TYPST`, `MARKDOWN`, `PLAIN`; the unions → `TestLang`, `AnyLang`, `LangArg`), so its `roundtrip` mode gives each file back. New criterion `mechanical`, with its own reverse frozen in the gate: every test file of the merge-base that changed is a codemod file whose reverse gives the merge-base text back, or a NAMED file with exact edits declared now (L-1's reserved case leaves `insert` for `editor/context`; C-7's one row per new command; `context.test.ts`'s plain `lang` becomes `undefined`). The comparison ignores formatting outside strings and comments only (Biome may reflow a shortened line); each file keeps its set of test titles; no file is deleted; `frontend` passed at least 1513. A new rule test goes in a new file. Probed: green after `apply`, Biome and the named edits; red on one changed assertion, on a doubled space inside a string, on an added test |
| 2 | The language's name sat in `format`, so a grammar without formatting had none | `grammars[].label` (required, VS Code's `aliases`): the status bar's language and the bar's name; `format` holds formatting only (`entry`, `accepts`, `readers`). Lint: the grammars' `label`, and no `label` in `format`; F-1 refuses a grammar without label (vector `*grammar-no-label*`) |
| 3 | EX0c1-E2 (`ex0c1.spec.ts:170-172`) red in the replay set is probably a read race during the Markdown grammar's on-demand load | Plan §4, named step-2 item: record the bar's sequence with a `MutationObserver`, prove which it is; a short loading state → the test polls until both reads agree; items that vanish and come back on a switch from text to text → a product flicker, fixed in the status bar. EX0c1-E2 stays in the replay set |

## Minor

| # | Finding | Change |
|---|---|---|
| 4 | No palette keyword; uniqueness checked in French only and by case | Each insert command has `kw.insert` (« insérer ajouter », « insert add »); the lint checks the keyword and both texts, and no two command labels equal in `fr.json` or `en.json`, compared with `casefold()` |
| 5 | The menu items' accessible names change silently | Plan §2 declares it: an item is named by its command's label; its description and syntax move to `aria-describedby`, still shown. EX0c3-E1 asserts both |
| 6 | `cmd.insert.needsBar` was a wrong reason for a big file; `insert.csvTable.markdown` is a dead duplicate | The context's `readOnly` says a big file is read-only (ADR 0028), so the reason is `cmd.format.unavailable`; a step item run while the person hid the bar shows the bar first (its setting, as `app.toolbar`), so no command greys for the bar and `cmd.insert.needsBar` never exists. `insert.csvTable.markdown` is deleted; Markdown's description goes through `describe` (`insert.csvTable.markdown.desc`). Lint: the exact `when` and reason, the keys absent, the `describe`, `readOnly` reading `workspace.big` |
| 7 | The type-id lint missed inline keys and `"lang." +` | Keys after `{` or `,` (not only at a line's start), and `lang.` built by `+` as by `${…}`. Its one other hit, the grid's clipboard field `tsv` (`sheet.svelte.ts:821,829`), is allowed on those lines only. New mutant MI9 (F-6): the writing help given for every type |
| 8 | `e2e:webkit` ran the tests against no browser when its container did not answer | `mise.toml`: after its 60 s wait the task stops with « e2e:webkit: container not ready … » and the container's last log lines; the timeout is unchanged. Probed with a `podman` that starts nothing: exit 1 after 63 s with that message. No script changed, so no Python test; UI-05, E1 and E2 stay |

## Deadline

30 + 30 + 300 + 120 + 480 + 30 + (20 + 300) + (20 + 240) + (200 + 420) + 150 + 300 + 30 = 2670 s ≤ 2700 s;
`gate.deadline_problem("EX0c3", …)` returns None.
