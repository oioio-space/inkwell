# EX0 review, round 2, blind Opus review, 2026-11-24

Verdict: « accept with corrections », 1 blocking + 3 major. Last review round before the freeze: each finding is fixed
at its root. The review text, as the integrator handed it over:

> 2 (blocking) R-10 `chunks` tests only whether an entry's own chunk is in the start closure: a static import of
> Sheet.svelte in App plus a thin `export … from` wrapper as the dynamic entry passes everything. Fix: build with
> sourcemaps and refuse any start-closure chunk whose `sources` include @codemirror/lang-markdown, lang-json, lang-yaml,
> lang-xml, legacy-modes or src/ui/sheet/ (or the exact list the ADR claims leave the start chunk); optionally assert
> the App chunk shrinks below 1,379,259 bytes (measured at 05039bb).
>
> 3 (major) mutants.py run_vitest sets caught = bool(failed) over every assertion of the test FILE, so a mutant can be
> caught by another rule's test while its named test asserts nothing; R-2, R-8, R-9 have no mutant; an R-2 loop over an
> empty testdata/manifests/invalid/ passes. Fix: in scripts/netsim/mutants.py, a Vitest mutant counts as caught only
> when a failing test's title matches the entry's named test (opt-in field per entry is acceptable if making it the
> default would turn frozen gates red — check them and say); add mutants for R-2 and R-8 (and R-9 if it is a rule);
> require ≥ 1 vector in each valid/ and invalid/ folder. Test the mutants.py change in scripts/netsim/mutants_test.py.
>
> 4 (major) `no-lists` misses `["csv","tsv"].includes(type.id)`, `name.split(".").pop() === "csv"`, `endsWith("csv")`;
> the gate exempts every file while the plan allows type-id reads only for the format bar and the Insert menu. Fix:
> list the allowed type-id read sites by file (tagged EX0c), and match bare type ids in comparisons and arrays
> everywhere else.
>
> 5 (major) resolution rules contradict: ADR §6 checks the suffix before the content class and `images` is blob, yet
> witness R-4 expects « a .png that decodes as text keeps a text viewer »; nothing says when a blob type's content class
> is computed; SVG decodes as text. Fix: state when the content class is computed and that a mismatch between it and
> the type's storage sends the file to the essential viewer of the real class; decide SVG's storage explicitly.
>
> 6 (minor) the « looks tabular » hint / « alternative viewer for any text » cannot be written in the manifest (no
> storage wildcard, no point); if it runs on every text open it runs when main.typ opens at start. Name a point or drop
> it from this slice — but keep W79 (text/table choice for CSV, docs/exigences.md) satisfied: CSV's two viewers.
>
> 7 (minor) built-in manifests come through an eager JSON glob (Vite turns them into JS); load them with ?raw +
> JSON.parse so built-ins go through the one parse-and-validate path.
>
> 8 (minor) raising filesVersion also changes OpFileInfo (files.go:11,88) and the tree payloads (tree.go:67,123,129,147):
> name all of them, their vectors and webview decoders in plan step 4.
>
> 9 two caps never measured: `mutants` (280 s, 9 runs) and `perf-start` (1090 s, blocked by PF1). PF1 is committed on
> main (60ea6eb, not yet pushed): fast-forward your worktree onto main (`git merge --ff-only main` after saving your
> uncommitted work; it is untracked/modified files only, check `git status` first) and witness both caps; keep G-1.
>
> 11 (minor) scope: the `data` extension (4 new grammar packages: new supply chain) is new function → EX0d; use
> `markdown` instead of `json` in `chunks` and EX0-E1; moving the image allow-list into internal/project is not needed
> for this slice.

## How each finding was answered

- **2 — fixed at the root.** `chunks` now builds with `--sourcemap` and reads the `sources` of every chunk in the start
  closure. It refuses any source under `@codemirror/lang-markdown/`, `@lezer/markdown/`, `frontend/src/ui/sheet/` or the
  `native/` folder of a built-in not activated at startup. These are exactly the things ADR 0027 rule 9 (rewritten)
  says leave the start. `lang-json`, `lang-yaml`, `lang-xml` and `legacy-modes` are not in the slice any more (11).
  A start JS chunk without a sourcemap must be under 1 KiB: only Vite's generated modulepreload polyfill (670 bytes)
  has none. A static import plus a thin wrapper now fails by source, whatever the chunk names. Witness at 60ea6eb: the
  App chunk holds 15 forbidden sources (`src/ui/sheet/viewMode.svelte.ts`, `@lezer/markdown`,
  `@codemirror/lang-markdown`…). The size assertion was not added: a frozen byte count would turn red on any later
  growth of `main` unrelated to EX0, and the sources rule is the exact property.
- **3 — fixed in the runner.** `run_vitest` takes an opt-in `name` (the exact test title); with it, only that test
  failing is a catch, a title that merely contains the name is not, and `validate` refuses `name` on a non-vitest or
  empty entry. Not the default: block SI1's 14 vitest entries (frozen gate `gates/SI1b.toml`, criterion
  `rule-SI1-mutants`) have no `name` and would no longer be read as their gate was frozen. Six new tests in
  `scripts/netsim/mutants_test.py` (`VitestNameTest`), 29 of 29 pass; `mutants.py --check`: 0 problems. The change of
  `mutants.py` needs its `gates/guard-overrides.json` entry at integration (gate machinery that frozen gates call). New
  mutants: MX10 (R-2), MX11 (R-8), MX12 (R-9, decoder) and MX13 (R-9, Go encoder, `expect`), plus MX14 (R-15, below).
  The gate requires `name` on every vitest entry and ≥ 1 vector in `testdata/manifests/valid/` and `invalid/`.
- **4 — fixed.** `no-lists` matches every string literal that is a type id or format word (`"csv"`, `` `pdf` ``…),
  which catches comparisons, arrays, `includes` and `endsWith("csv")`, and every split of a name on its dot outside
  `frontend/src/extensions/`. The remaining reads are listed by file, with the literals each file may hold and a tag:
  format bar, Insert menu, word count, Typst status, presentation and the CSV diagnostics source → EX0c; the LSP
  language id → K-7. Self-checked on the three examples of the review. Witness: 94 sites, including the earlier
  missed `highlight.ts` switch, the extension table of `format/insert.ts` and `workspace.svelte.ts`'s `includes`.
- **5 — fixed in ADR 0027 rule 6.** The type comes from the name alone. The content class is computed once per open,
  by Go, when the file is read as text, never at listing and never in the webview, and only for text types and unknown
  names. A text type with binary content goes to the file card (the real class wins). A blob type is read as bytes, and
  a viewer that cannot decode them falls back to the card (rule 7). SVG: storage `blob`, decided explicitly (ADR 0023,
  Typst reads it from disk). Witness R-4 is rewritten; the contradictory « `.png` that decodes as text » row is gone.
- **6 — dropped and replaced by a rule.** The « looks tabular » hint leaves this slice (a later « suggest » point, off
  the start path). W79 holds through one rule that runs no code: for every text type whose default viewer is not the
  text editor, the text editor is an alternative. CSV therefore offers grid and text in the existing view bar. New rule
  R-15 with its test and mutant MX14.
- **7 — fixed.** Built-in manifests are read with `?raw` and go through the one `JSON.parse` and validator (ADR rule 4,
  plan step 1).
- **8 — fixed at the root.** One version constant per payload instead of one shared byte. The file list goes to 2; the
  file info and the four tree payloads keep 1 under their own constants. Plan step 4 names each encoder and decoder
  (Go `files.go`, `tree.go:67,123,129,147`; webview `store/files.ts`, `store/tree.ts`) and which vectors change.
- **9 — measured.** The worktree was fast-forwarded onto 60ea6eb (work archived first, `git status` checked; no
  overlap). The caps are measured in `WITNESSES.md`; G-1 holds.
- **11 — scope cut.** `data` stays as a data-only extension (json, yaml, toml, xml as text types, needed to replace
  today's `data` kind) with no grammar; the grammars and their packages are EX0d. `chunks` and EX0-E1 use Markdown.
  `ReadImage`'s name check moves to its only caller, `internal/app/image.go`, which already holds `imageTypes`; no
  allow-list moves.
