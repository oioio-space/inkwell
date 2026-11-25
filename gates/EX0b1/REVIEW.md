# EX0b review, step 1 (blind Opus review, 2026-11-25; standard tier, no OpenRouter credit)

Verdict: accept with corrections. Nothing blocking; six majors, all applied before the freeze (answers in WITNESSES.md):
1. a viewer could take over a core command (palette, save) by declaring it → a viewer declares only its own ids plus a closed list of surface-delegable core ids;
2. keybindings and masks could name another extension's command → own commands only, masks between built-ins;
3. macOS Option and Windows Alt chords stealing printable characters → refused when modifiers ⊆ {Alt, Shift} and the key is printable;
4. widget keys (grid, APG) and searchKeymap outside the conflict check → one data table checked like CodeMirror's keymaps;
5. no-hard-keys allowlist per file, not per key → exact literals per file plus a Vitest census;
6. commands could be vacuous → a table-driven test runs every built-in command through run() and asserts its own effect, with a handler-swap mutant.
Minor: run() surface captured at keydown, closing guard, e.repeat, CommandButton/CommandMenuItem through run(); per-system chord rendering; spec §8.2 amended; perf-start witness green at rest (PF2 fixed the driver's polling).
Integrator's decisions: split into EX0b1 (points, run(), listener, migration, menus) and EX0b2 (palette, ranker, shortcuts window); palette order in UX1 rule 48 (no query: recents first; with a query: match quality, recency as tie-break); cuts (English-label matching, unused `when` keys, non-builtin command entries until EXT1).
