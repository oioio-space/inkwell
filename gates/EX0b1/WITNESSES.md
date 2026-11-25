# EX0b1 - red witnesses

Recorded 2026-11-25 at step 1, before any production change, on the worktree `inkwell-ex0b` at `main` cb356b5 plus the
plan, this draft gate and the step-1 amendments that are decisions, not code: rule 48 of the UX1 plan, spec §8.2 (the
capture listener), the PROGRESS lines EX0b1, EX0b2 and UX-EX0b-1. Each command was run once as written in
`gates/EX0b1.toml`, from the worktree root, through `sh -c` as `scripts/gate.py` does. Logs are in the session
scratchpad (`witness-EX0b1-<id>.log`). Setup first: `pnpm install --frozen-lockfile` and `mise run fe:bindings` (the
worktree had neither). Before each heavy criterion both `heavy.sh` slots were checked free (`flock -n`). Round 1 was the
unsplit draft `gates/EX0b.toml`; round 2 is this gate, after the blind review and the split.

| Criterion | Round | Exit | Time | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| docs | 2 | 1 | < 1 s | ADR 0027 lacks its EX0b1 section; 12 rows of spec §8.2 marked « fait » name no command id; G-63, G-98, G-99 do not name EX0b1. Found: the 12 plan rows, the EX0b2 section, rule 48, the capture listener in §8.2, the EX0b1, EX0b2 and UX-EX0b-1 lines | yes (written with the code). Two of its checks were first wrong (rule 48 wraps onto a second line; PROGRESS names EX0b1 in a summary line before its own): fixed to read the rule's continuation and the block's own `- **…**` line, then run again |
| no-hard-keys | 2 | 1 | < 1 s | 64 problems: the same 46 sites as round 1 plus `ui/sheet/keys.ts:64` (`"F3"` is not one of its exact literals F2, F10); 10 grid `case` literals outside the widget keys; the header bar's own menu items; the five named commands absent; « only 0 commands have chords » | yes: plan §4's sites, now by exact literal per file |
| mutants | 2 | 1 | < 1 s | MK1..MK12 missing; no `*command*` vector in `valid/`; no `*foreign-command*`, `*other-manifest-binding*`, `*plugin-command-entry*` in `invalid/`; runner not reached | yes. `mutants.py --check`: 255 entries, 0 problems with this gate present |
| simplify-report | 2 | 1 | < 1 s | `gates/EX0b1/SIMPLIFY.md` absent | yes |
| frontend | 2 | 1 | 308 s | Biome clean (485 files), svelte-check `0 ERRORS 0 WARNINGS` (800 files), Vitest 1435 passed, 0 failed, 1 todo; the 12 rule names C-1..C-12 `absent` | yes (308 s of 480 s, load 1.38) |
| e2e | 2 | 1 | 31 s | `e2e:build` done; Chromium: SAV-02, SAV-03b, UI-02, UI-03, UI-04, ER-1–4, PRV-105, « B2 text zoom », SHEET-KEY (2), SHEET-ROWS, PRS-06: 12 tests passed first time (26 s); EX0b1-E1, E2 `absent` | yes (load 1.17 → 1.02) |
| perf-start | 3 | 0 | 123 s | after PF2 (ae99621, the worktree fast-forwarded onto it): PERF-START `pass` in its first attempt, paired against the merge-base ae99621 (A/A identity true), ratio 0.9996, bounds 0.986–1.013, half-width 1.37 % (cap 4 %), contaminated 0; the absolute budget `pass` (median bound 244 ms ≤ 350, p95 bound 274 ms ≤ 400); no other exclusive run, both slots free, load 0.75 → 1.27 | yes: a regression guard, green on the unchanged tree. Rounds 1 and 2 (before PF2) were invalid in all six attempts, « imprecise », half-widths 4.1–4.7 % |

Red for the right reason: 6 of 7; `perf-start`, a regression guard, is green on the unchanged tree since PF2 fixed
its measuring driver (below).

## Why PERF-START was imprecise on an unchanged tree (finding 9; fixed by PF2, ae99621)

- The start times are not continuous: in each of the six runs they fall in three modes, about 245, 265 and 315 ms
  (run 2, attempt 1: 46 starts at 240–259, 75 at 260–279, 38 at 290–329), on both sides alike and in both orders, with
  no drift between the first and last 40 starts.
- The modes are 245 + {0, 20, 70} ms. `scripts/perf/start.cjs:86` ends each start with
  `page.locator('.cm-content').waitFor()`. In Playwright 1.64.1 that is `waitForSelector` → `retryWithProgressAndBackoff`,
  which checks the selector after waits of `[0, 20, 50, 100, 100, 500]` ms (`playwright-core/lib/coreBundle.js`, the
  `backoffScale` of `retryWithProgressAndBackoff`): the measured time is the first check after the editor appears, so
  it is quantised to +0, +20 or +70 ms. Readiness is also polled every 20 ms (`start.cjs:76`).
- An A/A pair therefore differs by 0 or about 8 %, 19 % or 29 % either way, at random, and the Hodges-Lehmann interval of 80 such pairs at
  α = 0.001 is about 4–4.7 % wide: on the cap by construction. EX0's passing run was already borderline (its attempt 1
  invalid at 4.09 %, attempt 2 pass at 3.54 %; `inkwell-ex0/.task/ex0/perf`, the same modes).
- The fix belonged to the measure (ADR 0038's driver), not to this block nor to the cap. PF2 (ae99621) times the start
  from the program's own events instead of the driver's polling; round 3 above passes at a half-width of 1.37 %.

## Caps (rule G-1)

- Measured: `frontend` 308 s (cap 480); `e2e` 31 s with a warm `e2e:build` and 26 s for 12 tests (caps 200 + 360: the
  two EX0b1 scenarios add a CSV grid, a held key and a menu census); `perf-start` 123 s for one valid attempt after PF2, 360–367 s for three invalid ones before (cap 600).
- `mutants`: no EX0b1 entry before step 2; EX0's 14 entries took about 80 s on the same runner; 12 single-file Vitest
  entries; cap 300 s.
- The caps add up to 2050 s, within `max_minutes = 45`; `deadline_problem` returns `None`.

## Checked to work with the real tools

- `no-hard-keys` matches plan §4's sites and only them, now by exact literal per file (`editor/setup.ts` may hold only
  `Mod-z`; `ui/sheet/keys.ts` only F2 and F10 and its widget `case` literals; `PublicWindow.svelte` only F11); in
  `frontend/src/extensions/` only `keytables.ts` is exempt.
- `e2e`: the twelve regression tests pass on today's code, so their red after the change can only be a regression;
  `PRV-105` presses `ControlOrMeta+Alt+j` today and is updated to Ctrl+Maj+J with the code (spec chord, K-7).
- Round 1 also ran `chunks` (a real Vite build: lazy modules are keyed by source path with `isDynamicEntry`) and
  `e2e-webkit` (`PLAYWRIGHT_JSON_OUTPUT_NAME` passes through `mise run e2e:webkit`); both move to EX0b2's gate.

## What each named test must assert (checked at the code review and by the mutants)

A rule test that only runs code passes nothing here: each must make these assertions, and the mutant in brackets must
turn exactly this test red, by its title (criterion `mutants`, block EX0b1, field `name`; entries written at step 2).

- **C-1** [MK1]: each `invalid/` vector whose name holds `command` is refused with the reason its file name states:
  unknown key in a command, a menu item or a keybinding; an id without its extension's prefix; `Mod` in a chord; a
  non-canonical chord (`Shift+Ctrl+x`, `Ctrl+Shift+?`); more than 3 chords on a system; a menu `location` or `group`
  outside the closed lists; `when` without `unavailable`; an `entry` leaving the folder; and the three the gate names:
  `*foreign-command*` (a viewer's `commands` holding another extension's non-delegable id), `*other-manifest-binding*`
  (a keybinding of a command this manifest does not declare), `*plugin-command-entry*` (origin `plugin`, a command with
  an `entry`: refused until EXT1). Each `valid/` command vector is accepted unchanged.
- **C-2** [MK2]: a table of at least 15 rows (expression, context → value or refusal): `!`, `&&`, `||` with and without
  parentheses (`a || b && c` is `a || (b && c)`), `==`, `!=`, `in` on `formats`, an unknown key, a value outside an enum
  (`focus == sidebar`), `=~` refused, 257 characters, depth 9; `fileType == csv` with the `tables` extension disabled is
  valid and false; and every key of the context table is used by at least one built-in `when` or `toggled`.
- **C-3** [MK3]: per system: Ctrl+Alt+x refused everywhere; macOS: Alt+e, Alt+Shift+e, Alt+1 refused, Alt+F5 and
  Alt+ArrowDown accepted (not printable); a command with F5 on Linux and only F5 on macOS refused (K-7 letter rule), with
  ⌥⌘P accepted; Windows: Alt+a, Alt+Shift+a, Alt+1 refused; the reserved chords ⌘H, ⌥⌘H, ⌘M, ⌘Q, ⌃Space, ⇧⌘? (macOS),
  Ctrl+., Ctrl+Shift+u (Linux), Ctrl+Escape (Windows) refused, ⇧⌘? accepted with `delegated: system` and never captured;
  in `window` scope Ctrl+a, Ctrl+z, Ctrl+ArrowLeft, Home refused (field-editing set); a bare `b` and a bare `Enter`
  refused in any scope.
- **C-4** [MK4]: two built-in manifests binding Ctrl+b in `editor`: (A, B) refuses B's binding naming A and B, (B, A)
  the reverse, the rest identical; `window` against `grid` on one chord conflicts unless the `grid` binding `masks` the
  window command; a `masks` in a `plugin`-origin manifest refused; a plugin binding on Ctrl+s inactive and listed, its
  commands kept; a built-in viewer declaring `format.bold` from `text` (own) and `app.zoom-in` (delegable) accepted,
  one declaring `typst.present` refused.
- **C-5** [MK5]: her Ctrl+Shift+b for `format.bold` replaces Ctrl+b; her Ctrl+s for `format.strike` switches
  `app.save`'s default off on this device and the snapshot reports it replaced; `null` removes; her ⌘H on macOS, her
  Ctrl+Alt+x, her Alt+e on macOS and a second command on a chord of hers refused; a stored list that is not JSON or has
  unknown keys leaves the defaults in force.
- **C-6** [MK6]: `keytables.ts` holds, per system and scope, CodeMirror's `defaultKeymap`, `historyKeymap` and
  `completionKeymap` chords (computed from the installed packages' `key`, `mac`, `win`, `linux` fields and compared to
  the table), the grid's APG keys, the unwired search keys and the field-editing set; every registry chord meeting an
  entry in an overlapping scope names it in `masks` (Ctrl+i `selectParentSyntax`, Ctrl+m `toggleTabFocusMode`, Ctrl+u
  `undoSelection` on Linux and Windows), a `masks` hiding nothing is refused. Census: the keymap facet of an editor
  created by `editor/setup.ts` holds exactly CodeMirror's keymaps plus the delegated undo; the modifier cases of
  `ui/sheet/keys.ts`, exercised by synthetic events, are exactly the grid's table.
- **C-7** [MK7]: the registry built from the built-ins has no refusal and no inactive binding on linux, mac and win;
  every label, tooltip and keyword key exists in `fr.json` and `en.json`, labels ≤ 40 and tooltips ≤ 80 characters
  rendered (ND-4); every row of design spec §8.2 marked « fait » names an id whose three rendered chords equal the row;
  a `delegated: widget` chord equals the one its widget binds. One table, one row per built-in command: on a mounted
  fixture (shell spy, editor with a known text and selection, grid, diff view), `run(id)` produces that row's effect and
  no other row's (bold wraps the selection, `app.sidebar` flips `sidebarOpen` only, …); MK7 swaps two handlers in
  `ext/app/native/commands.ts`.
- **C-8** [MK8]: events → chord or none, per system: US Ctrl+b; AZERTY Ctrl+& (keyCode 49) → Ctrl+1; Cyrillic Ctrl+и
  (keyCode 66) → Ctrl+b; Windows Ctrl+Alt+e (AltGr) → none; macOS Option+e and Option+Shift+e → none; Windows
  Alt+Shift+a → none; `isComposing` and keyCode 229 → none. Dispatch: Ctrl+b in the editor runs `format.bold` once and
  prevents the default; in a field it neither runs nor prevents; a false `when` lets the event through; `e.repeat`
  runs `app.next-change` again but not `app.save`; with `workspace.closing` nothing runs. The system: `darwin` → mac,
  `windows` → win, `linux` and absent → linux, read from `window._wails.environment.OS`.
- **C-9** [MK9]: with an editor and a grid mounted, `app.zoom-in` runs the grid's handler when the key is pressed in the
  grid and the text zoom otherwise; a key on a lazy command pressed in the grid, focus moved to the editor before the
  entry resolves, acts on the grid; `run()` calls the flush hook before the handler (a recorder shows the order).
- **C-10** [MK10]: the main menu from the built-ins has groups `view`, `project`, `document`, `app` in that order, each
  sorted by `order` then extension id; a plugin item in `app` or `project` refused; with `canPresent` false the three
  presentation items are greyed (`aria-disabled`) with their reason, and activating one announces the reason and runs
  nothing; with `canReturn` false « Revenir à mon document » is absent; a toggle item is a `menuitemcheckbox`.
- **C-11** [MK11]: with a fake registry, every control of `HeaderBar` (six buttons, the menu open), `FormatBar`,
  `HeadingMenu` and `ZoomMenu` that runs something has `data-cmd` naming a registry command, an accessible name that
  starts with the registry label, a `title` with the label and this system's chord, a matching `aria-keyshortcuts`, and
  a click reaches `run()` with that id (a spy); a toggle keeps its label in both states and sets `aria-pressed`.
- **C-12** [MK12]: building the registry loads no entry except those of `startup` extensions; each `when` is parsed once
  (a counter); 1 000 key events without Ctrl, Alt or Meta and not a function key cause 0 chord lookups and 0 `when`
  evaluations.
- **EX0b1-E1**: a letter typed then Ctrl+S at once is in the saved file, and Ctrl+S held for 1 s saves once; Ctrl+B in
  the text bolds, in the file-name field changes nothing; F5 opens the presenter view; Ctrl+F in a CSV grid opens its
  search bar; F7 held in a difference steps more than once.
- **EX0b1-E2**: in the shell with the main menu open, every `[data-cmd]` names a command of the built-in manifests (read
  from disk) and shows its French label from `fr.json`; the six header buttons and the main-menu items all carry one.
