# Requirements to anticipate now in the soon blocks (2026-11-14)

Method: every non-done row of docs/exigences.md, against the blocks built next (SB2a1/a2/p, PC1, IV1, AD1, NA1, PR1,
EX0, UX1, UF1 guards, HB1a/b, RP1, R1 acceptance; then G-0b/G-0c, TR1, ADR 0034 steps 4-8, Relire, files and tabs,
Git link). Kept only where building the soon block without the requirement forces a rewrite, a data- or wire-format
change, a migration or a security hole.

| # | Requirement | Soon block | Decide or add now | Evidence |
|---|---|---|---|---|
| 1 | N43 fonts | SB2a1 (gate about to freeze) | the read-only paths of both tiers are the font list of FT-1/FT-4, computed by Go (XDG data `fonts`, `~/.fonts`, `fonts.conf` dirs, « mes polices », project `fonts/`), with a witness; today the launcher hard-codes two folders and ADR 0044 names none | `internal/sidecar/launcher_linux.go:435`; spec §6.12 FT-1, FT-2, FT-4 (« ouverts en lecture seule aux sidecars (Landlock…) »); `docs/adr/0044-sandbox-tiers.md:70` |
| 2 | W59/W59b (large folders) + G-0b | AD1 | the admitted set is capped at 257 documents for everyone and only grows (every variant, whatever its state); with one document per file (G-0b) a real project exceeds it: size the per-author quota and the cap for file documents, and say when merged/abandoned variants and deleted files leave the set | `frontend/src/net/scope.ts:36-49`; exigences W59, W59b; coherence §5 « Écrire et retrouver » (G-0b) |
| 3 | N14 + W67/V9, T3, CP1 | PR1 | the presence payload (`name`, `file`, `anchor`, `head`) is not specified publicly and presence maps are closed (unknown key ⇒ message dropped): define its schema now with the fields the next blocks need (document id for variants, « is writing », visible range) or an extension rule, else each addition silences older peers | `docs/protocol/spec.md:71`, §9.5 (`:1760`); `frontend/src/editor/presence.ts:30-47`; PR1 plan out-of-scope list |
| 4 | W71, N39, N48 | Relire block (and G0 manifests) | a version record carries `by` = Automerge actor (an unchecked claim) and `at` = the writer's clock, for `main.typ` only; the Git link needs a signed member author and project-wide heads, N48 a validation state: define the record once, as the project-level manifest, before V6 (note) changes it | `frontend/src/history/types.ts:5-15`; `docs/plans/2026-11-14-git-link-design.md` (prerequisite G0) |
| 5 | N35 (macOS, Windows) | PC1 | the restart is specified for Linux only (R-8, take-over not offered elsewhere); PC1's model is being written now: model the restart abstractly (exec in place vs spawn-then-hand-over of the single-instance lock) so v1's platforms are a new implementation, not a new design | `docs/plans/2026-11-14-pc1-project-change-by-restart.md:13`, `:108-109`; ADR 0037 |
| 6 | U26 (file types modular, ADR 0027 missing) | EX0 | EX0's registry is the file-type registry of spec §15.4 (« Type de fichier : reconnaissance ») and absorbs ADR 0027, otherwise two registries exist | exigences U26 (« rédiger ADR 0027 ») ; spec §15.2 X-2, §15.4 |
| 7 | N37 (protected project) | SB2a1 / SB2p | a protected project compiles from a plaintext working folder in memory (tmpfs without swap, C03); tier F has no mount namespace: ADR 0044 must state what tier F does for a protected project (refuse, or a memory-backed folder), so the vault lot does not reopen the tier table | coherence C03 (§6.10 EN-3–EN-5); `docs/adr/0044-sandbox-tiers.md:70` |
| 8 | N21, N23, N32 (plugins, AD-57) | SB2a1 | plugins run in an `inkwell-sandbox` process: the tier decision and the allow-lists should be per program (each program's measured perimeter), not hard-wired to typst and tinymist, so the plugin host is a third program, not a redesign | `docs/architecture/decisions.md` AD-57; ADR 0044 D2 perimeter |
| 9 | N22 (storage backend as extension point) | G-0b | spec K-6 makes the storage *backend* an extension point; `internal/store` has no backend interface; G-0b defines the new layout: put it behind that interface then | spec §15.3 K-6; `internal/store/*.go` (no interface) |
| 10 | N66 (AI as its own signed author) | AD1 | AD1's quota is « per author »: define the author as the member key, with an AI author counted to the device that runs it, before ADR 0031 step 8 adds AUTHOR signatures | exigences N66; ADR 0031 (AUTHOR) |

Already settled by the design: N37 storage layout (coherence C01–C03), X-8 extensions never enter compilation,
N4/W43 locked zones (ADR 0022, proposed: deterministic admission C1 — AD1 must keep its place in the check order),
N44/N45/N55 (lot S13, after R1, no soon block depends on it), W34 (SW1 keeps Typst behind an interface).

Missing from the register: none found — the owner's requests of 2026-11-14 are W63–W72 (UX studies, trust boundary,
diff/versions/files/tabs/text changes, designs kept, variants with several people, cloud reviews, Ubuntu 24.04,
existing sandboxes and Windows, Git link, plugins exposure); the two mock-up remarks (switcher on one line, close
button) are in `docs/plans/2026-11-14-ux-ui-study.md` §7.
