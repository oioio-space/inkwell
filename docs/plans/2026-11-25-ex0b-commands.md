# EX0b: commands, menus and keybindings through the registry (EX0b1), then the palette (EX0b2)

Rigour: standard. Split on review (integrator, 2026-11-25, W84): **EX0b1** (this plan, gate `gates/EX0b1.toml`, witnesses
`gates/EX0b1/WITNESSES.md`) builds the `commands`, `menus` and `keybindings` points, `run()`, the one key listener and
the migration of every hard-coded command, shortcut and main-menu item; **EX0b2** (its own gate, below) the palette, its
ranker and the shortcuts window. Builds on ADR 0027, which gains a section « Commands, menus and keybindings (EX0b1) ».
Closes P21, G-63, G-98, G-99; P20 with EX0b2; applies spec §4.2, §8.2 (K-5…K-7), architecture §15.4, §15.13, ND-3, ND-4.
Out: other menu locations, toolbar layout, panels, status, Insert (EX0c); find in the text and the project (UX1-2:
`editor/search/keymap.ts` is unwired, and `@codemirror/search` is not installed); editing one's shortcuts (UX1-5); the
macOS menu bar (UX1-1); commands of other origins at run time (EXT1); arguments and macros (§15.12, API 1.1).

## Prior art (read 2026-11-25) and what is better here

VS Code: `commands`, `menus` (`group@order`), `keybindings` (`key`, `mac`, `when`); user rules « appended at runtime to the
bottom » win; conflicts only listed; `when` has `=~` regex and unknown keys are silently undefined
([points](https://code.visualstudio.com/api/references/contribution-points), [when](https://code.visualstudio.com/api/references/when-clause-contexts),
[keys](https://code.visualstudio.com/docs/configure/keybindings)). Zed: actions are the identity, the deeper then the later
binding wins, `null` unbinds ([keys](https://zed.dev/docs/key-bindings)). Obsidian runs `checkCallback` to know
availability ([commands](https://docs.obsidian.md/Plugins/User+interface/Commands)). GNOME: Ctrl+letter, no Alt, Super is
the system's ([keyboard](https://developer.gnome.org/hig/guidelines/keyboard.html)); 3–12 menu items, « Keyboard
Shortcuts », « Preferences », « About » last ([menus](https://developer.gnome.org/hig/patterns/controls/menus.html)). Apple:
⌘ first, shown ⌃⌥⇧⌘, no standard shortcut repurposed ([keyboards](https://developer.apple.com/design/human-interface-guidelines/keyboards)).
`aria-keyshortcuts` names keys ([MDN](https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Reference/Attributes/aria-keyshortcuts)).
CodeMirror 6.43 `runHandlers`: `keyName`, then `base[keyCode]` under a modifier (AZERTY digits), never for AltGr.
**Better:** a conflict is refused at build, never settled by order or depth, which hide it; a scope shadows another
contribution only by naming it (`masks`, built-ins only); `when` is typed against a closed table, no regex; availability
is data, so menus draw with no extension call (§15.4); each system's chord is written, never derived from `Mod` (⌘H, ⌘M).

## 1. The three points (same validator, bounds and null-prototype rules as ADR 0027)

| Point | Fields |
|---|---|
| `commands` | `id` (`<extension>.<name>`), `label`, `tooltip`?, `keywords`? (i18n keys, used by EX0b2), `icon`? (§8.3), `category` (closed), `when`?, `unavailable` (reason, required with `when`), `toggled`? (boolean key), `repeat`? (held key repeats: zoom, next-change, next-problem), `entry`? + `capabilities`? (confined as viewers' entries; refused for any origin but `builtin` until EXT1; `ui.native` for essential built-ins only) |
| `menus` | `command`, `location` (`main` here), `group` (`view`, `project`, `document`, `app`; the last two protected, §15.13), `order` 0–999, `when`? (shown) |
| `keybindings` | `command` (one of the manifest's own commands), `linux`, `mac`, `win` (≤ 3 canonical chords each, first shown; absent = none), `scope` (`window`, `editor`, `grid`, `diff`), `when`?, `masks`? (built-in manifests only: a command or a widget-table entry it hides in its scope), `delegated`? (`system` or `widget`: shown, never captured) |

`viewers[]` gains `commands`: its own extension's ids, plus the closed list of surface-delegable core ids (`app.undo`,
`app.redo`, `app.zoom-in`, `-out`, `-reset`, `app.find`, `app.replace`, `app.find-next`, `-previous`, `app.next-change`,
`-previous`). Activation `onCommand:<id>`. A command takes no argument.

## 2. Context, keys, conflicts

- **`when`** (`extensions/when.ts`): exactly the keys a built-in uses, a test refuses an unused one: `focus` (innermost
  surface: `editor`, `grid`, `diff`, `other`), `field` (a text field has focus), `fileType`, `formats` (a set), `readOnly`,
  `hasToolbar` (enables `app.toolbar`), `toolbarVisible`, `previewOpen`, `sidebarOpen`, `reviewOpen` (the toggled states),
  `canPresent`, `canReturn`; `!` > `&&` > `||`, parentheses, `==`, `!=`, `<word> in <set>`; ≤ 256 characters, depth ≤ 8;
  parsed at validation (unknown key, value outside an enum: refused), evaluated as an AST (CSP). A `fileType` value that no
  loaded extension contributes is valid and simply never equals the current type: the clause is false until that
  extension is there, whatever the build order, since `when` is evaluated against the live context, not resolved at build.
- **Chord** (`extensions/keys.ts`): `Ctrl+Alt+Shift+Meta+<key>` in that order; a shifted character carries no `Shift`
  (`Ctrl+?`); `Mod` refused. **Rendering per system:** Linux shows the character as typed (« Ctrl+? », GNOME); Windows shows
  the Shift the character needs (« Ctrl+Maj+? »); macOS ⌃⌥⇧⌘ then the key, ⇧ shown likewise (« ⇧⌘? »); French « Maj »,
  English « Shift ». The system is the OS Wails reports (`darwin` → mac, `windows` → win, else linux).
- **Refused on its system:** Ctrl+Alt; on macOS any chord whose modifiers are within {Alt, Shift}, include Alt and end in
  a printable key (it types a character), and a command whose Linux or Windows chord is a function key without a macOS
  chord on a letter (K-7; Zones is the spec's one exception, not in this slice); on Windows the same with Alt±Shift (access
  keys); the reserved table of §8.2 unless `delegated: system`; in `window` scope a chord of the field-editing set (Ctrl+A,
  C, V, X, Z, Y, Ctrl+arrows, Ctrl+Backspace, Ctrl+Delete, Home, End and their ⌘/⌥ forms); a chord without Ctrl, Alt or
  Meta unless a function key.
- **One table of keys owned by others** (`extensions/keytables.ts`, data): CodeMirror's `defaultKeymap`, `historyKeymap`
  and `completionKeymap` (computed from the installed packages), the grid's APG keys (Ctrl+A, Ctrl+Space, Shift+Space,
  Alt+↓, F2, Shift+F10, ContextMenu, Alt+Enter in a cell, navigation), the unwired search keys of UX1-2, the field-editing
  set, the reserved tables. A registry chord meeting an entry in an overlapping scope names it in `masks`; a `masks` entry
  hiding nothing is refused.
- **Conflict** (same chord and system, overlapping scopes; `window` overlaps all): between built-ins the later is refused
  naming both, in either order, unless the narrower `masks` the other; any other origin stays inactive and is reported
  (ND-3). **Her bindings win** (`inkwell.keybindings`, this device): hers replaces the default, a default it collides with
  is switched off and shown replaced, `null` removes; reserved, forbidden or already hers: refused on entry.

## 3. Running and identity

- `run(id, target?)` (`extensions/commands.ts`) first flushes pending edits (`beforeCommand`: `editor/amSync.ts:138`
  relies today on listener order), then calls the target surface's handler (a mounted viewer declaring the command, or
  the diff view), else the command's entry with the native context (`shell`: App actions; `view`: the target editor).
- One capture-phase `keydown` listener: nothing while `workspace.closing` (today's guard, `App.svelte:272`) or a modal is
  open; skips composition (`isComposing`, 229) and `e.repeat` unless the command has `repeat`; leaves at once without
  Ctrl, Alt or Meta unless a function key; resolves as CodeMirror does; consumes the key only if a binding is active, its
  `when` holds and a handler exists; captures the target surface at keydown, so a lazy entry that loads later still acts
  where the key was pressed.
- **Identity:** `cmdAttrs(id)` gives `data-cmd`, label or `aria-label`, `title` « Label (Ctrl+S) », `aria-keyshortcuts`;
  `CommandButton` and `CommandMenuItem` draw it and call `run()` (no other click path), with `aria-disabled` and the reason
  when unavailable (« UX1 question 3 »): activating a greyed item announces its reason and runs nothing. A toggle keeps one
  label with `aria-pressed` (APG): « Panneau de gauche », not « Afficher / Masquer ».

## 4. Migration (each site deleted once on the points)

New built-ins `ext/app` (essential, startup, `native/commands.ts`) and `ext/format` (data only, handled by `text.editor`,
can be disabled). About 50 commands, 55 bindings per system, 9 main-menu items.

| Site | Becomes |
|---|---|
| `App.svelte:271-297`, `:563` (F8, Ctrl+S, F5, Ctrl+Alt+J, Ctrl+Maj+T) | `app.problem-next`/`-previous`, `app.save`, `typst.present`/`-here`, `typst.jump-to-preview` = Ctrl+Maj+J (K-7), `app.toolbar` |
| `editor/format/keymap.ts:9-23` (installed `editor/setup.ts:78`); twin `key` fields `ui/toolbar/spec.ts:57-127` | `format.*` (18); `Mod-Alt-0` gone (G-98); macOS ⌃⌘E, ⇧⌘M (G-99) |
| `shortcutLabel`, `isMac` `ui/toolbar/spec.ts:193-206`; `HeaderBar.svelte:120,156,182,186`; `HeadingMenu.svelte:51,67`; `FormatBar.svelte:79,103` | registry labels per system |
| `HeaderBar.svelte:85-138` (six buttons), `:141-227` (main menu, 9 items) | `CommandButton`; `menus` `main` |
| `language/extension.ts:131-137` Ctrl+Maj+Espace | `typst.signature-help` (`editor`, `fileType == typst`) |
| `ui/history/HistoryBars.svelte:110-121,140` zoom | `app.zoom-in`/`-out`/`-reset`, the grid's when it has focus |
| `ui/history/DiffView.svelte:148-153,163` F7 | `app.next-change`/`-previous` (diff surface) |
| `ui/sheet/keys.ts:63-65,97-135`; `ui/sheet/FindBar.svelte:87` F3 | `app.find`, `app.replace`, `app.find-next`, `tables.fill-down`/`-fill-right`/`-insert-today`/`-insert-cells`/`-delete-cells` (`!field`; masks `app.zoom-out` in `grid`); Ctrl+Alt+- gone (K-7) |
| `editor/setup.ts:177` Ctrl+Z; `ui/sheet/keys.ts` Ctrl+Z, Y | `app.undo`, `app.redo`, `delegated: widget` |
| `ZoomMenu.svelte:18-24`, `StatusBar.svelte:97`, `FormatBar.svelte:81,96`, `HeadingMenu.svelte:44,60`, `ContextMenu.svelte:86` | `data-cmd`, identity and `run()`; placement in EX0c |

Widget keys stay where they are, each listed in `keytables.ts` and by exact literal in the gate: the grid's APG keys,
menus, lists and fields, presentation mode (`present/*`, F11 in `PublicWindow.svelte`), Escape closers, clipboard.

## 5. Rules (test titles in the gate; each Vitest rule has a mutant caught by that title only)

| Rule | What | Mutant |
|---|---|---|
| C-1 | the three points validate strictly; vectors: a viewer declaring a foreign id, a binding of another manifest's command, a `plugin` command with an `entry` | MK1 `Mod` accepted |
| C-2 | `when`: closed keys all used, grammar, precedence, `in`, a type not loaded | MK2 `\|\|` binds before `&&` |
| C-3 | K-7, Alt±Shift on macOS and Windows, function keys on macOS, reserved chords, field-editing set, on each system | MK3 macOS Option rule skipped |
| C-4 | conflicts in either order; `masks` only between built-ins; a viewer handling a foreign non-delegable id refused; other origins inactive | MK4 the later binding overwrites |
| C-5 | her bindings win and are checked | MK5 defaults applied after hers |
| C-6 | the table of keys owned by others: collisions named, none stale; census: the created editor's keymap facet = CodeMirror's keymaps + delegated undo + registry; `keys.ts`'s modifier cases = the grid's table | MK6 a grid key missing from the table |
| C-7 | built-ins on three systems: no refusal, fr and en ≤ 40 and ≤ 80 (ND-4), §8.2 « fait » rows equal; every built-in command run through `run()` on a mounted fixture shows its own effect (one table) | MK7 two handlers swapped |
| C-8 | event to chord (US, AZERTY, Cyrillic, AltGr, Option, Alt+Shift, IME), repeat, closing guard, the system from Wails | MK8 `base[keyCode]` removed |
| C-9 | target surface captured at keydown, lazy entry acts there, edits flushed first | MK9 `beforeCommand` skipped |
| C-10 | main menu from `menus`, fixed and protected groups, greyed item announces and runs nothing | MK10 group order ignored |
| C-11 | `data-cmd`, registry identity and `run()` on HeaderBar, main menu, FormatBar, HeadingMenu, ZoomMenu | MK11 a `data-cmd` dropped |
| C-12 | no start cost; typing keys reach no lookup | MK12 typing keys looked up |

End to end: `EX0b1-E1` keys (Ctrl+S keeps the last letter, Ctrl+B in text not in a field, F5, Ctrl+F in the grid, a held
F7 repeats, a held Ctrl+S saves once), `EX0b1-E2` every `[data-cmd]` names a command with its label; replayed: SAV-02,
SAV-03b, UI-02, UI-03, UI-04, ER-1–4, PRV-105 (its chord becomes Ctrl+Maj+J), « B2 text zoom », SHEET-KEY, SHEET-ROWS, PRS-06.

## 6. Performance and gate

Start: PERF-START paired (ADR 0038), a regression guard; commands parse with the manifests (counted, C-12). Typing: no
lookup without a modifier (C-12). Gate (max 45 min): `docs`, `no-hard-keys`, `frontend`, `mutants`, `e2e`, `perf-start`,
`simplify-report`. Risks: the capture listener on the typing path, AZERTY, IME (C-8, UI-02, SAV-03b); a save missing
the last keystroke (C-9); macOS and Windows checked as data only (C-3, C-6, C-7) until CI.

## 7. EX0b2: palette and shortcuts window (own gate, after EX0b1, before UX1-0)

Rewritten into its own plan: [2026-11-25-ex0b2-palette.md](2026-11-25-ex0b2-palette.md) (gate `gates/EX0b2.toml`,
witnesses `gates/EX0b2/WITNESSES.md`): the palette, its ranker (ordered match classes, rule 48), the shortcuts window,
PERF-PALETTE in Chromium and WebKit, a counted bound on 1 000 commands, both components lazy (criterion `chunks`).
