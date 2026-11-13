# A `.md` opened from the file manager is read-only: limit or defect?

2026-11-13. Investigation only (no code, no test run: a frozen gate held the machine). Rigour if a block follows:
standard. Origin: `PROGRESS.md:31-33` (« Défauts relevés hors bloc ») and `gates/OI/UX.md`, « Out of scope but seen ».

## Verdict

**Assumed limit, not a defect.** The read-only view is the specified behaviour of lot G-0a, delivered by L2 on
2026-11-03, and OI did not cause it. One thing is **not** recorded anywhere: R1 registers Inkwell as a Markdown
handler while Markdown cannot be edited before G-0b, which is scheduled after R1. That is a product choice to put to
the user, not a bug to patch.

## Need

A person double-clicks `notes.md` in the file manager. Inkwell opens it (OI row R3 when the file belongs to the open
project) and shows « Dans cette version, ce fichier s'affiche en lecture seule ». She cannot type. The question from the
walkthrough: intended, or broken?

## Cause (evidence)

Only one file of a project is an Automerge document today: `main.typ`. Every other file is a projection read from
disk through Go, with no write authority.

| Where | What it shows |
|---|---|
| `frontend/src/workspace/workspace.svelte.ts:44` | `const MAIN_FILE = "main.typ"`: the single editable document |
| `frontend/src/workspace/workspace.svelte.ts:132-150` | `openFile`: `MAIN_FILE` returns to the live handle; any other path is loaded into `opened` (a text, an image or a card) and polled for changes |
| `frontend/src/App.svelte:81` | `mainFile = workspace.current === "main.typ"` |
| `frontend/src/App.svelte:466-473` | `{#if !mainFile}`: a table goes to `Sheet … readOnly readOnlyReason={t("file.readOnly")}`, any other text to `ReadOnlyText … message={t("file.readOnly")}` |
| `frontend/src/ui/ReadOnlyText.svelte:16,31-33` | `createReadOnlyEditor` and the banner (`role="status"`) |
| `frontend/src/i18n/fr.json:2`, `en.json:2` | the sentence itself |
| `frontend/e2e/files.spec.ts:43` | the read-only state of a non-main text is asserted (`aria-readonly="true"`): it is a tested requirement |

The decision is by file identity, not by file kind: a second `.typ`, a `.md`, a `.csv`, a `.bib` and a `.txt` are all
read-only; `main.typ` alone is editable. The OI open path adds nothing: R3 calls the same `Workspace.openFile` as a
sidebar click (`docs/plans/2026-11-11-oi-single-instance-open.md`, row R3).

## Where the limit is recorded

- `docs/architecture/spec.md:1055` (§6.6, lot G-0a): « tout autre fichier texte → **lecture seule** depuis la
  projection … avec le bandeau « Dans cette version, ce fichier s'affiche en lecture seule » »; `:1056` G-0b brings
  « un document par fichier », depending on ADR 0031 and step 8 of ADR 0034.
- `docs/design/spec.md:984-989` (§4.16): same two steps, G-0a read-only, then « G-0b, ensuite : ouverture en édition ».
- `docs/architecture/reviews/04-codex-r2.md:320-322`: the cross-family review that chose this exact wording.
- `docs/plans/2026-11-03-L2-implementation.md:3-5,13-14` and commit `d42a73e`: « Only `main.typ` edits its existing
  Automerge handle … its banner quotes design §4.16 exactly ».
- `docs/adr/0023-projet-multi-fichiers-blobs.md:11` (status: proposed): « Seul `main.typ` se synchronise ».
- `docs/exigences.md:156` (U7) and `:162` (U13): the other text files wait for B4b/B5.
- Order: `PROGRESS.md:46-47` puts G-0b and G-0c **after R1**.

Not recorded: `docs/autonomous-decisions.md` and `docs/exigences.md` have no line saying that R1 ships with Markdown
read-only. The limit predates the autonomy rule, so its source is the two specs above.

## The unrecorded tension

`build/linux/io.github.oioio_space.Inkwell.desktop:18` lists `text/markdown` (and `text/x-typst`, `text/csv`; OI adds
the other separated-value types by user decision Q2). The Desktop Entry Specification defines `MimeType` as « The MIME
type(s) supported by this application »
(<https://specifications.freedesktop.org/desktop-entry/latest/recognized-keys.html>); desktops use it to build
« Ouvrir avec » and the default handler. A person may therefore make Inkwell her default for `.md` and then meet a
viewer. The same holds for any `.typ` that is not `main.typ`, and for every `.csv`.

The banner itself follows the GNOME HIG: a banner is for an ongoing state and states a short, precise fact
(<https://developer.gnome.org/hig/patterns/feedback/banners.html>). It says the state and that it is temporary
(« Dans cette version »). It offers no way forward because none exists in the application yet; adding a button that
leads nowhere would be worse (Nielsen, heuristics 1 and 9: <https://www.nngroup.com/articles/ten-usability-heuristics/>).

## Recommendation: keep for R1, record it, ask one question

1. **Keep the behaviour.** No code change in this block. A local « make the `.md` editable » shortcut would write a
   projection outside Automerge and break two invariants of `CLAUDE.md` (one source of truth per thing: text →
   Automerge; every byte from a peer verified): it is the patch the golden rule forbids. The root fix is G-0b.
2. **Record it where it belongs** (coordinator, not this file): a line in `docs/exigences.md` under W41 or U13
   (« R1 : seul `main.typ` se modifie ; les autres fichiers s'ouvrent en lecture seule jusqu'à G-0b »), the R1
   announcement text, and the R1 acceptance walkthrough (block 10) as an expected screen, not a finding. Close the
   `PROGRESS.md:32-33` item as « limite assumée (G-0a, `docs/architecture/spec.md:1055`) ».
3. **Two wording corrections in existing documents** (when OI is next touched): OI row R2 says « a file is shown in
   the editor », true only for `main.typ`; `gates/OI/UX.md` should point to G-0a instead of « pre-existing ».
4. **One question for the user**, to log in `docs/autonomous-decisions.md` with the default below: does R1 keep
   Inkwell in « Ouvrir avec » for Markdown while it only displays it? Default: **yes** (Q2 was her decision; reading
   with highlighting is real support; the banner is honest; R1 is an alpha between trusted people).

Alternatives considered:

| Option | Size | Why not the default |
|---|---|---|
| A. Keep and record (recommended) | docs only | — |
| B. Remove `text/markdown` and `text/csv` from `MimeType` until G-0b | one line + `pkg:lint` expectation | reverses user decision Q2; the file still opens read-only from the sidebar, so the limit stays, only less visible |
| C. Pull G-0b before R1 | large, « complet » rigour (peer state, admission) | depends on ADR 0031 and step 8 of ADR 0034, which `PROGRESS.md:46` orders after R1 on purpose (typing first) |
| D. Edit the projection file directly, outside Automerge | medium | violates the invariants above; unshared edits would silently diverge between people |
| E. Reword the banner or add a button | small | the sentence is quoted from design §4.16 and matches the HIG; no real action exists to offer |

## If the user picks a fix (option B), test plan and draft gate

Named tests, each at the cheapest layer that keeps its defect class:

- `pkg:lint` selftest « refuses a desktop entry listing a type the editor cannot edit » (Python, pure): the
  `MimeType` line equals the editable set, today `text/x-typst` only.
- `TestDesktopEntryMimeTypesAreEditable` (Go, table): each listed type maps to a file kind the window edits.
- e2e `files.spec.ts` « FM-OPEN-3 » unchanged: the sidebar still opens a `.md` read-only with the banner.

Draft acceptance gate `gates/MR.toml` (commands only, to freeze before the change):

```
mise run pkg:lint
mise run test:py
mise run test
mise run fe:check
mise run e2e:build && cd frontend && ../scripts/heavy.sh timeout 900 mise x -- pnpm exec playwright test e2e/files.spec.ts
mise run lint:claude-md
test -s gates/MR/SIMPLIFY.md
```

For option C the tests are already named in `docs/architecture/spec.md:1056` (FM-OPEN-4 to FM-OPEN-8); its plan and
gate belong to block G-0b, not here.
