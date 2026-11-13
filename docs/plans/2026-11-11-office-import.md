# Study: importing Excel and Word documents (W47)

Date: 2026-11-11. Request W47 in `docs/exigences.md` (user, French): "Go has a very good package for Excel,
would it be worth integrating, and for Word too? Maybe import a Word document and turn it into Typst. Study
the packages, say what you think, what the options are, how to integrate them, whether it is relevant."

Status: study only. Nothing was built, run or measured for this document (a long gate was running, so no
download, build or test was started). Every number below comes from a cited source or from the repository;
what I could not verify is marked **[unverified]** and becomes a measurement in the first lot that needs it.
Per the "no empiricism" rule, nothing here is a decision until it has a rubric and a measured result.

## 0. Summary and recommendation

1. **Decision N40 / AD-33 / AD-98 already answers part of this question, in the negative.** On 2026-11-01 the
   user decided "no native Office integration, no pandoc shipped; a plugin (`importers` / `exporters`) if
   someone wants it" (`docs/exigences.md` N40; `docs/architecture/decisions.md` AD-33 revision 3, AD-98).
   W47 reopens it. Any lot below therefore starts with a short ADR that supersedes N40 *for import only*
   (lot IM0). Exporting DOCX stays out of scope.
2. **Excel: yes, worth it, and cheap.** Import a workbook sheet as **CSV text** into the existing CSV editor
   (`frontend/src/editor/csv`), which already generates "Table from a CSV" Typst code
   (`frontend/src/editor/csv/typst.ts`, `csvTableCode`). Text stays in Automerge, collaboration and history
   work unchanged, nothing new in the Typst pipeline. Library: **excelize v2.11.1 (BSD-3-Clause)**, run in a
   confined child process, because its 2026 advisory record shows that its parser cannot be trusted
   in-process on hostile files (section 1.2). A zero-Go-code complement exists: keep the `.xlsx` as a blob
   and read it from Typst with the pinned `spreet` package (section 1.4).
3. **Word to Typst: worth doing, but it is the expensive and contested half.** The best quality per effort
   is **pandoc** (Typst writer since 3.1.2, current 3.13, GPL-2.0) as a third pinned sidecar like `typst` and
   `tinymist`. Its cost is **about +36 to +43 MB of download archive per platform** (section 2.1) and a
   GPL binary to redistribute. The alternative is a small pure-Go OOXML mapper (stdlib only, no licence
   issue, full control of the "what was not converted" report) with a lower fidelity ceiling. This is a real
   product choice (size against function; the user already flagged it as hers in AD-33, "taille de
   l'exécutable contre fonction"). My recommendation: **decide with a measured spike (IM2a)**, not by taste.
4. **Rejected:** LibreOffice headless (too big, not pinnable in a self-contained app), unioffice
   (commercial licence), fumiama/go-docx (AGPL-3.0), GPL/unmaintained `.xls` readers, calling the system's
   pandoc, mammoth.js in the webview.
5. **Fit:** not MVP. Excel import belongs in the v1 row "Modèles et écriture" of
   `docs/architecture/coherence.md` §5 (an on-ramp for existing data). Word import belongs after
   "Six plates-formes" (pandoc must exist on every target) or inside "Extensions utilisables" if the user
   keeps the plugin route.

## 1. Excel: .xlsx, .xls, .ods

### 1.1 Candidate Go libraries

| Library | Licence | Latest | State | Verdict |
|---|---|---|---|---|
| [xuri/excelize](https://github.com/xuri/excelize) | BSD-3-Clause | v2.11.1, 2026-11-02 | very active, 21.1k stars, requires Go 1.26 (we are on 1.27.3) | **candidate** |
| [tealeg/xlsx](https://github.com/tealeg/xlsx) | `NOASSERTION` on GitHub (licence file not detected) **[unverified]** | v3.3.13, 2025-04-18 | **archived** (2025-08-13) | rejected: archived |
| [thedatashed/xlsxreader](https://github.com/thedatashed/xlsxreader) | MIT | v1.2.8, 2024-06-02 | quiet since 2024, 81 stars, low-memory reader only | fallback if excelize is refused |
| [extrame/xls](https://github.com/extrame/xls) (`.xls`) | Apache-2.0 | no release | last push 2023-09 | unmaintained, no hostile-input track record |
| [shakinm/xlsReader](https://github.com/shakinm/xlsReader) (`.xls`) | GPL-3.0 | v0.9.12, 2022 | stale | rejected: GPL, stale |
| [unidoc/unioffice](https://github.com/unidoc/unioffice) | commercial EULA (licence key required) | v2.14.0, 2026-09-21 | active | rejected: commercial |
| ODS: knieriem/odf, multiprocessio/go-openoffice (MIT), mukbeast4/go-ods (created 2026-03, 0 stars) | various | - | old or brand new | none convincing; search was not exhaustive |

Excelize facts (sources: GitHub API, [excelize.go v2.11.1](https://github.com/xuri/excelize/blob/v2.11.1/excelize.go),
[docs](https://xuri.me/excelize/en/), [pkg.go.dev](https://pkg.go.dev/github.com/xuri/excelize/v2)):

- Reads and writes XLAM/XLSM/XLSX/XLTM/XLTX. **Does not read `.xls` (BIFF) nor `.ods`.**
- Row iterator `File.Rows(sheet)` (streaming read, `Next`/`Columns`/`Close`), `GetRows`, a `StreamWriter`
  for writing, formula engine (`CalcCellValue`), password-protected workbooks (`Options.Password`).
- `Options`: `UnzipSizeLimit` (total uncompressed bytes, default 16 GiB: `1000 << 24`), `UnzipXMLSizeLimit`
  (per XML part held in memory, above which parts spill to disk), `MaxCalcIterations`, `RawCellValue`
  (skip number formatting). Defaults are too generous for our purposes; we would set our own.
- Dependencies (v2.11.1 `go.mod`): `richardlehane/mscfb`, `xuri/efp`, `xuri/nfp`, `tiendc/go-deepcopy`,
  `golang.org/x/{crypto,image,net,text}`. Several are already likely in our graph through libp2p; to be
  checked with `go mod why` in IM1.

### 1.2 Security history (the decisive fact)

GitHub's security-advisory API for `xuri/excelize` lists **19 published advisories between 2026-06-03 and
2026-10-03**, all fixed by v2.11.0 or v2.11.1 (7 high, 12 medium by the severity shown). Examples:
unbounded row number makes `GetRows` / `Rows` loop "for days" (GHSA-jw42-f3rr-4cc3, high); `r="0"` rows
cause a negative slice index panic (GHSA-c85p-xxjj-2r75, high); CFB directory entries allocate attacker-sized
buffers, OOM (GHSA-x2q3-8cjh-766f, high); nine panic sites in decryption (GHSA-2j4c-ffch-9f23, high); unbounded
`spinCount` burns CPU (GHSA-jrfj-fhj2-jjvm, high); mutually recursive array formulas overflow the stack
(GHSA-wp2g-vpjj-g53r, high). Most are in features an importer never needs (decryption, pivots, slicers,
formula calculation), but two sit on the exact path we would use (row iteration, coordinate parsing).

Reading of this record, both sides:

- It shows a library that was **never hardened against hostile files**, and that is being fuzzed now
  (the burst of reports looks like systematic fuzzing). More findings are likely.
- It also shows a **responsive maintainer** (fixes within weeks, tagged releases).
- Consequence: excelize may only touch untrusted files **inside a confined child process** (RLIMIT_AS,
  CPU/wall timeout, no network, output cap, killed on overrun), the same hardening pattern as ADR 0024 for
  Typst (`sh` + `ulimit` before `exec`, process group, `Pdeathsig`). A panic, OOM or endless loop then kills
  the child, not the editor, and the whole advisory class (DoS) becomes "import failed, here is why".
  In-process use is rejected.
- The child can be the inkwell binary re-executed with a hidden subcommand (no extra embedded binary, but
  the dispatch must happen before any Wails/GTK initialisation) or a small `cmd/` binary; to decide in IM1.
- Defence in depth before opening: read the ZIP central directory ourselves (stdlib `archive/zip`) and
  refuse by entry count, per-entry and total uncompressed size, compression ratio, absolute or `..` names;
  cap rows, columns, cell length; set excelize's own limits to those values.
- The decoder must be fuzzed (CLAUDE.md invariant "every byte from outside is verified and its decoder fuzzed"
  applies in spirit; an office file is not a peer, but the rigour is the same).

Go's `encoding/xml` does not resolve external entities or DTDs, so XXE is structurally absent for any
stdlib-based reader; the known XML hazards left are size and nesting, covered by the bounds above.

### 1.3 What the user gets: four ways, compared

| Route | Result | Editable in app | Mergeable (CRDT) | New code | Fidelity |
|---|---|---|---|---|---|
| A. Sheet to **CSV** (excelize, child process) | `sheet.csv` text in the project, opens in the existing CSV editor; "Table from a CSV" gives the Typst `#table` | yes | yes (text in Automerge) | importer + dialog | values only: formulas become their cached values; merged cells, styles, charts, images lost (reported) |
| B. Keep `.xlsx` as a **blob**, read it from Typst with `spreet` | `#spreet.decode(read("f.xlsx", encoding: none))` style snippet | no (binary) | no (blob replaced as a whole) | a snippet generator like `csvTableCode` | `.xlsx` and `.ods`; "full" mode returns styles and spans but is "unstable" per the package page |
| C. Sheet to a **Typst `#table`** text directly | table written into the `.typ` | as Typst text | yes | mapper + escaping | static copy, loses the link to the data |
| D. System opens the file | out of scope | - | - | - | - |

Typst side (checked 2026-11-11):

- Native `csv()` reads text (default delimiter `,`, single ASCII char, rows as arrays or dictionaries,
  headers not stripped) ([docs](https://typst.app/docs/reference/data-loading/csv/)). There is no native
  `.xlsx` reader. `read(path, encoding: none)` yields bytes **[verify against 0.15.1 docs]**.
- [`rexllent`](https://typst.app/universe/package/rexllent) 0.4.1 (2026-04-29, MIT, first release 2025-01-10):
  `.xlsx` to a styled `table` through a WASM plugin; "still in development"; TODO list includes in-cell images,
  special characters, hidden rows and columns; **styles and merged cells are lost when it goes through spreet**.
- [`spreet`](https://typst.app/universe/package/spreet) 0.2.0 (2026-06-01, MIT, first release 2024-09-15):
  decodes Excel and OpenDocument spreadsheets into a dictionary of 2-D string arrays (the page names `.xlsx`
  and `.ods` in its examples; `.xls` is not mentioned); no charts.
- Both packages would have to be vendored under the project's offline package rule (ADR 0024: `@preview`
  packages supplied or in `.typstdeps/`, never downloaded). A WASM plugin running inside the already
  sandboxed `typst` process (private namespaces, `RLIMIT_AS`, `RLIMIT_FSIZE`) means **the untrusted file is
  parsed by a process that is already confined**, which is an attractive property for route B.

My view: A is the product feature (editable, collaborative, text); B is a nearly free complement for people
who want the table to follow a spreadsheet they keep editing in Excel (and the only route to `.ods`
without writing an ODS reader). They do not exclude each other. C duplicates A plus
`csvTableCode` and is rejected.

Conversion rules to specify in IM1 (table of decisions, not decided here): one CSV per sheet, named after
the sheet (sanitised); dates and numbers exported as displayed or raw (`RawCellValue`), because Excel's
date serial formats are a classic trap; formulas export cached values (never recalculated, to avoid the
formula engine); hidden sheets/rows skipped and reported; merged cells repeated or left empty and reported;
CSV injection (`=`, `+`, `-`, `@` prefixes) is irrelevant on import but matters on export (section 1.5).
The dialect (delimiter, decimal comma, header) is the project's CSV `Dialect`, already modelled.

`.xls` and `.ods`: the honest answer for v1 is "not supported, save as `.xlsx` or CSV" for `.xls` (the
Go readers are unmaintained or GPL) and route B for `.ods`. A stdlib `.ods` reader is feasible
(`content.xml`, but `table:number-rows-repeated` / `number-columns-repeated` is a decompression-bomb trap
that must be bounded) and could be a later lot if asked.

### 1.4 Export to xlsx (brief)

Excelize's `StreamWriter` makes "Export CSV as .xlsx" easy. Value is modest (people can open a CSV in
Excel directly) and N40 excludes Office export. If ever done: write text cells as strings (never as
formulas) to avoid spreadsheet formula injection, no round-trip promise. Lot IM3, optional, not recommended now.

## 2. Word: .docx, .odt, .rtf to Typst

### 2.1 Pandoc as a pinned sidecar

Facts:

- Pandoc has a **Typst writer since 3.1.2** (2023-03-28, "Add typst writer", per the
  [3.1.2 changelog record](https://zenodo.org/records/7776341); other sources date it 3.1.3, so "3.1.2 or
  3.1.3"). Current release **3.13, 2026-10-26**, GPL-2.0, 46.9k stars, pushed 2026-11-11
  ([GitHub](https://github.com/jgm/pandoc)). Readers: docx, odt, rtf, epub, html, markdown, latex and more.
- Command shape: `pandoc -f docx -t typst --extract-media=<dir> in.docx` (plus `--sandbox`, see below).
  Alternative target `-t gfm` or `markdown` for Markdown projects, from the same sidecar.
- Math: Word equations (OMML) are read into pandoc's math model and written as Typst math by the writer
  (the manual lists Word equations among the supported math sources) **[quality unverified]**.
- Docx reader options worth using: `--track-changes=accept|reject|all` (`all` keeps insertions, deletions and
  comments in spans), `+styles` to keep unmapped styles as `custom-style` classes; `--sandbox` restricts
  readers and writers to the files named on the command line and "is recommended for untrusted input"
  ([manual](https://pandoc.org/MANUAL.html)). `--extract-media` writes images to a directory (hash-derived
  names when the original path is not safe).
- Release assets for 3.13 (archive sizes, from the GitHub API):

  | Platform | Asset | Size |
  |---|---|---|
  | Linux amd64 | `pandoc-3.13-linux-amd64.tar.gz` | 36.1 MB |
  | Linux arm64 | `pandoc-3.13-linux-arm64.tar.gz` | 38.7 MB |
  | macOS arm64 | `pandoc-3.13-arm64-macOS.zip` | 43.1 MB |
  | macOS x86_64 | `pandoc-3.13-x86_64-macOS.zip` | 26.9 MB |
  | Windows x86_64 | `pandoc-3.13-windows-x86_64.zip` | 43.0 MB |
  | Windows arm64 | none seen in the release assets | **gap for the "six platforms" promise** |

  For comparison, `internal/sidecar/manifest.json` pins typst 0.15.1 (archive 14 to 17 MB) and tinymist 0.15.10
  (archive 30 to 33 MB); ADR 0024 reports 55 to 57 MB embedded for the two (gzip). Pandoc would add roughly
  **+36 to +43 MB** compressed per platform, i.e. about +65 to +75 percent over today's embedded sidecars
  **[estimate; the embedded size after gzip is to be measured in IM2a]**.

Fit with ADR 0024 is direct: manifest entry with URL, archive SHA-256, member, binary SHA-256, size;
`mise run sidecar:fetch` at build time only; embedded gzip; SHA-256 rechecked before every launch;
process group, `Pdeathsig`, `RLIMIT_AS` / `RLIMIT_FSIZE`, private network namespace; never downloaded at run
time. The import is a one-shot, so no supervision loop or backoff is needed, just a timeout. Run it with an
empty working directory, `--sandbox`, stdin or one named input path, and an output-size cap watched while
it writes (as ADR 0024 does for Typst).

**GPL-2.0 next to an MPL-2.0 app.** MPL-2.0 is file-level copyleft and the app never links pandoc. The FSF
FAQ says "pipes, sockets and command-line arguments are communication mechanisms normally used between two
separate programs", and that distributing separate programs together is "mere aggregation", while "if the
modules are included in the same executable file, they are definitely combined in one program"
([GPL FAQ](https://www.gnu.org/licenses/gpl-faq.en.html)). We would extract pandoc to the cache and exchange only
argv, a file path and text, exactly as with typst. Obligations that remain: ship the GPL-2.0 text and the
corresponding source offer (GPL-2 section 3) for the pandoc binary we redistribute, and list it in the
NOTICE / About screen and in the SBOM (`scan:sbom`). The FSF also says the line is ultimately a legal
question, so IM0 should get an explicit review. This is not legal advice.

Quality ceiling (what to expect, to be measured not assumed):

- Good: headings, paragraphs, emphasis, links, lists, footnotes, simple tables, images, captions, math.
- Lossy by design: Word styles (default drops unmappable ones; `+styles` keeps names but not formatting),
  page layout (margins, columns, headers/footers, page breaks), text boxes, SmartArt, charts and
  embedded objects, fields (TOC, cross-references become text), complex merged tables, comments (unless
  `--track-changes=all`), fonts and colours.
- Images: Word often embeds EMF/WMF; Typst renders PNG, JPEG, GIF, SVG, WebP **[verify list for 0.15.1]**,
  so vector Office images will be missing and must be reported.
- A pandoc issue (2019, #5617) reports docx produced by Office 365 Online being mishandled
  ([listing](https://www.goodfirstissue.org/it/jgm/pandoc/issues/5617)); nothing newer found. Real-world
  files are the only test that matters, hence the corpus below.
- The result is an editable **starting point**, never a faithful copy. The UX must say so.

### 2.2 Pure-Go options

| Option | Licence | State | Verdict |
|---|---|---|---|
| [fumiama/go-docx](https://github.com/fumiama/go-docx) | **AGPL-3.0** | 308 stars, pushed 2025-05, no release | rejected: AGPL incompatible with MPL distribution and with our model |
| [gomutex/godocx](https://github.com/gomutex/godocx) | MIT | v0.1.4 (2024-07-02), 274 stars, pushed 2025-08 | writer-oriented, pre-1.0, reading is partial **[unverified]**; not a base for a mapper |
| [unidoc/unioffice](https://github.com/unidoc/unioffice) | commercial EULA | v2.14.0 | rejected |
| **Own mapper** on `archive/zip` + `encoding/xml` (`word/document.xml`, `styles.xml`, `numbering.xml`, `footnotes.xml`, `_rels`, `media/`) | MPL-2.0 (ours) | to write | viable for a bounded subset |

An own mapper (est. 2 to 4 k lines of Go **[estimate]**) covers: headings (by style id or outline level),
paragraphs, bold / italic / underline / strike / super- and subscript, hyperlinks, bulleted and numbered
lists (needs `numbering.xml`), simple tables with `gridSpan`/`vMerge`, inline images, footnotes. Its
advantages: no new binary (+~0 MB), no GPL, same Go test and fuzz toolchain, `encoding/xml` has no external
entities, and an exact fidelity report because the mapper itself knows every element it skipped. Its costs:
Word's real-world XML is the long tail (styles inheritance, `w:sdt`, fields, `mc:AlternateContent`, run
splitting by spell-check marks); OMML to Typst math would be a separate sub-project (a bounded subset:
fractions, scripts, radicals, n-ary, delimiters, matrices; the Rust
[tex2typst-rs](https://github.com/Unpredictability/tex2typst-rs) shows the TeX-to-Typst half exists elsewhere
but there is nothing for OMML in Go); `.odt` and `.rtf` are separate formats, each its own reader. Pandoc
gives docx, odt, rtf (and more) through one sidecar.

### 2.3 Other routes

- **docx to Markdown instead of Typst.** Inkwell also edits Markdown. Same pandoc sidecar with `-t gfm`;
  it is lossier for tables and math than Typst, but is the right output for a Markdown project. Cheap to
  offer as a second target in the same dialog.
- **mammoth.js** (BSD-2-Clause, pushed 2026-10-16) converts docx to HTML in JavaScript. Running in the
  webview puts an untrusted-file parser in the renderer, outside the confined-process pattern, and its
  Markdown output is, as far as I know, deprecated **[unverified]**. Rejected.
- **LibreOffice headless** (`soffice --convert-to`). LibreOffice is MPL-2.0, so the licence is not the
  issue. The issues are: a few hundred MB per platform, a profile directory and a slow start, a very large
  attack surface for hostile files, no Typst output (we would still need a second conversion, e.g. to
  docx/html then pandoc or a mapper), and no way to embed it in a "nothing to install" executable. Using an
  installed copy would be an unpinned dependency, contrary to ADR 0024's spirit. **Rejected**, except as a
  user-installed plugin if someone ever wants it.
- **System pandoc if present.** Unpinned, unverified version, behaviour varies; rejected (AD-33 option 2).
- **Plugin route (N40's answer).** `importers` is part of "Extensions utilisables" (N21 to N26, v1 row of
  `coherence.md` §5). Packaging pandoc as a plugin keeps the core binary small and honours N40, at the price
  of "find it, install it" and of waiting for the plugin API 1.0. A fair fallback if the user keeps N40
  unchanged for the core.
- No docx-to-Typst converter in Rust, Go or TypeScript turned up in a search (typ2docx goes the other
  way and needs pandoc; tylax is LaTeX only). Pandoc is the only mature path.

## 3. Constraints, one by one

| Constraint (CLAUDE.md, ADRs) | Consequence for import |
|---|---|
| Nothing downloaded at runtime (ADR 0024) | pandoc, if used, is pinned in `manifest.json`, fetched by `sidecar:fetch`, embedded, SHA-256 rechecked each launch. Excelize is a normal Go dependency. Typst packages for route B are vendored in `.typstdeps/`. |
| Text in Automerge, binaries in the BLAKE3 blob store (ADR 0011: Go carries opaque bytes, no Automerge in Go) | Import runs in Go and returns **text plus asset bytes**; the webview creates the Automerge documents; images and any kept `.xlsx` go to the blob store. Go never builds a CRDT. Convergence of the new documents is already covered by existing tests. |
| One-shot local conversion | No state machine beyond start / progress / done / failed / cancelled. A small decision table plus a timeout is enough; no TLA+ needed (tier below). |
| Untrusted input: zip bombs, XXE, path traversal, huge sheets | Pre-scan of the ZIP directory (entries, sizes, ratio, names), confined child process with `RLIMIT_AS`, timeout, output cap; output paths validated (no `..`, no symlinks, regular files only, names regenerated from content hash); sheet row/column/cell caps; decoder fuzzed (excelize path, mapper, report parser); no network (private namespace as for Typst; macOS lacks it, the known gap of ADR 0024). Initial numeric bounds to calibrate on the gate, not guess here: entries <= 10 000, total uncompressed <= 512 MiB, ratio <= 100:1, rows <= 1 048 576 (the xlsx hard limit), columns <= 16 384, cell <= 32 767 chars (the Excel limit). |
| Binary size, cross-platform | Excelize: unmeasured, expected to be a few MB **[to measure with `go build` size diff in IM1]**. Pandoc: +36 to +43 MB archive per platform, no Windows arm64 asset. Own mapper: ~0. |
| Licences | excelize BSD-3-Clause (compatible, attribution in NOTICE); pandoc GPL-2.0 as a separate process (section 2.1); spreet / rexllent MIT. `scan:sbom` and the licence check must list them. |
| Wording (glossary) | UI says "Importer", "classeur", "feuille", "document Word"; no "CSV dialect", "sidecar", "pair". French and English strings. |
| Interface coherence (CLAUDE.md "Cohérence de l'interface") | No new panel. Entry point inside the existing add/open-file menu ("Importer…") and drag-and-drop of `.xlsx` / `.docx` onto the file list or editor, which opens a small Adwaita dialog (choices: which sheets; "as table data (CSV)" or "keep the workbook"; for Word: "Typst" or "Markdown"). Progress and result in the existing toast / banner patterns. The exact menu to attach to must be read in `docs/ux-principles.md` and the current header-bar spec in IM0; I did not verify it here. |
| Honest report of partial fidelity | Every import ends with a result the person can read: what was converted, what was not, and why, in plain words ("2 images in an unsupported format were left out", "3 merged cells were repeated", "formulas were replaced by their last computed values"). Source of the report: for xlsx, our own counters (hidden sheets, merged ranges, formulas, charts, images found in the ZIP listing); for pandoc, a pre-scan of the docx parts (text boxes, SmartArt, charts, OLE, fields, tracked changes, comments, headers/footers) plus pandoc's stderr warnings. The report is shown in the dialog and kept as a collapsible detail; it is not written into the document. Never claim "done" without it. |
| Large files (W42) | Row iterator in the child, CSV written by chunks, cap and a preview showing the sheet list with row/column counts before conversion. |

## 4. Relevance

Who needs it. The project targets collaborative writing in Typst and Markdown, with people arriving from
Word and Excel (design PDF `docs/conception.pdf` lists RTF/DOCX "import/export only" and pandoc import at
stage 6; AD-33 recorded that). A person with an existing report or a data sheet wants to bring it in once and
continue in Inkwell. That is an **on-ramp**, not a daily feature, and fidelity will never be total.

Against current priorities (`docs/objectif-en-cours.md`: protocol steps 2 to 8, then G-0b / G-0c; the
MVP row of `coherence.md` §5 is about writing, collaboration, compiling, first minute and recovery), import
does not unblock any MVP item. The honest placement is **v1**, after the MVP rows:

- IM1 (xlsx to CSV) next to "Modèles et écriture" (N27, N30, N34) in the v1 table, because the CSV editor,
  CSV-to-table and large-file work (W42) already exist.
- IM2 (docx) after "Six plates-formes" if pandoc ships (needs all targets, including the Windows arm64
  question), or with "Extensions utilisables" if it stays a plugin.
- W47 itself is filed in `docs/exigences.md` with the status "study in progress"; this document is the
  study. Lots below are proposals; I did not edit `coherence.md` or `objectif-en-cours.md`.

## 5. Proposed lots

Rigour tiers follow `docs/engineering-principles.md` §0 (light, standard, complete; "if in doubt, take the
level above"). Each lot follows the project's definition of done (executable gate first, frozen).

### IM0 - Decision and corpus (tier: light)

Scope: ADR superseding N40 / AD-98 for import only (xlsx first, docx conditional); legal read of the GPL
position; build the **test corpus** (about 30 files: 10 xlsx incl. dates, merged cells, huge, hostile; 20 docx
from real people incl. Office 365 Online, Google Docs export, LibreOffice export, scanned-style, math-heavy,
tables-heavy, footnotes, French accents) and the fixed scoring rubric (structure, text, tables, images, math,
footnotes, links, each scored 0 to 2, plus the report's accuracy).
Risks: corpus files may carry personal data (use synthetic or consented files only; never commit real
people's documents). Needs the user's yes on the N40 change.

### IM1 - Excel to CSV (tier: complete, because it parses untrusted files)

Scope: `internal/` importer package (name to choose; one role), ZIP pre-scan, confined child, excelize
>= 2.11.1 pinned, CSV per sheet, report, bridge call returning text + optional blob, dialog and
drag-and-drop, French/English strings, fuzz target for the pre-scan and the sheet reader, hostile-file
tests (zip bomb, `r="0"` rows, absurd dimension, huge shared strings, encrypted workbook gives a clear
"this workbook is password-protected" message), budgets (time and peak memory on a 100 k-row sheet,
fixed before code), `ux-walkthrough` with one person. Optionally IM1b (tier: standard): the spreet snippet
"Table from a workbook" next to "Table from a CSV" with the package vendored in `.typstdeps/`.
Risks: more excelize advisories (mitigated by the child, the pin and `govulncheck` in `scan:code`);
date/number formatting differences (corpus cases); CGO-free build unaffected. Rollback is trivial (feature is additive).

### IM2a - Word spike (tier: standard)

Scope: with the IM0 corpus, run pandoc 3.13 (docx to typst, docx to gfm) and, in parallel, a throw-away
200-line mapper on a sample, score both by the rubric, measure embedded size after gzip and time per
document. Output: a table of scores and a one-page recommendation. No product code merged.
Risk: effort wasted if the user then picks the plugin route; kept small on purpose.

### IM2b - Word to Typst (tier: complete)

One of three shapes, chosen by the user after IM2a:

- **(a) pandoc embedded** as third sidecar: manifest entry, `sidecar:fetch`, confined launch (`--sandbox`,
  empty dir, caps), result validator (the generated Typst must at least compile or be flagged), media to
  blob store, report, fuzz of the report parser and the pre-scan, GPL notice and source offer, SBOM.
  Size +36 to +43 MB archive per platform; Windows arm64 unresolved.
- **(b) pandoc as an optional pinned component** (separate package or plugin): same code, not in the core
  download; honours "nothing downloaded at runtime" only if the user installs it deliberately and it is
  hash-verified; weakens "self-contained".
- **(c) own Go mapper** for the common subset, no pandoc: no size cost, no GPL, `docx` only at first;
  `.odt` and `.rtf` later or never; equations reported as not converted at first.

Risks: fidelity disappointment (mitigated by the report and the "starting point" wording); pandoc
parser memory on hostile docx (cap + rlimit); mapper long tail; Windows arm64; GPL review.

### IM3 - Export CSV to xlsx (tier: standard, optional, not recommended now)

Contradicts the spirit of N40 and has low value; listed only for completeness.

### IM4 - `.ods` / `.odt` / `.rtf` / `.xls` (tier: complete each, later or never)

`.ods`: route B already covers it in Typst; a stdlib reader only if asked. `.odt` and `.rtf`: free with
pandoc (shape a or b), costly with a mapper. `.xls`: no safe Go reader; recommend "save as `.xlsx`".

## 6. Alternatives rejected, with reasons

| Alternative | Reason |
|---|---|
| In-process excelize on user files | 19 advisories in four months, DoS class on our exact read path |
| `tealeg/xlsx` | archived since 2025-08 |
| unioffice | commercial licence key |
| go-docx (fumiama) | AGPL-3.0 |
| godocx | writer-oriented, pre-1.0, not a reading base |
| `.xls` Go readers | GPL-3 (xlsReader) or unmaintained (extrame/xls, 2023) |
| LibreOffice headless | hundreds of MB, not pinnable or embeddable, huge attack surface, no Typst writer |
| System pandoc | unpinned; contradicts ADR 0024 |
| mammoth.js in the webview | parser of hostile files outside the confinement pattern, not Typst |
| Sheet straight to a `#table` text | duplicates CSV import + `csvTableCode`, loses the data link |
| Import by clipboard HTML only | stays: AD-98's rich paste (HTML to Typst/Markdown) remains the zero-cost path for small tables; it does not replace file import |

## 7. Open questions for the user (product choices only)

1. **N40.** Do you accept reopening it for *import only* (no DOCX/XLSX export), starting with Excel? Or
   do you prefer to keep it a plugin?
2. **Size against function for Word.** Is +36 to +43 MB per platform (pandoc embedded, shape a) acceptable
   for a good docx/odt/rtf import? Otherwise: optional component (b) or limited own mapper (c)?
3. **Excel scope.** Are `.xlsx` to CSV (+ keep the workbook as a linked file) enough for v1, with `.xls`
   unsupported and `.ods` through the Typst package only?
4. **Word target.** Typst only, or also Markdown from the same dialog?
5. **Placement.** v1 as proposed (after MVP), or earlier because you have documents to migrate now?
6. **Corpus.** Can you provide a few real (non-confidential) Word and Excel files for the quality rubric?

Technical choices left to the implementation lots (child process shape, numeric bounds, exact menu) are not
asked of the user.

## Sources

- excelize: <https://github.com/xuri/excelize> (v2.11.1, 2026-11-02, BSD-3-Clause), docs <https://xuri.me/excelize/en/>,
  API <https://pkg.go.dev/github.com/xuri/excelize/v2>, advisories via the GitHub API
  (`repos/xuri/excelize/security-advisories`, read 2026-11-11).
- tealeg/xlsx <https://github.com/tealeg/xlsx> (archived); thedatashed/xlsxreader <https://github.com/thedatashed/xlsxreader>;
  extrame/xls <https://github.com/extrame/xls>; shakinm/xlsReader <https://github.com/shakinm/xlsReader>;
  unioffice <https://github.com/unidoc/unioffice> and its EULA; go-docx <https://github.com/fumiama/go-docx>;
  godocx <https://github.com/gomutex/godocx>; mammoth.js <https://github.com/mwilliamson/mammoth.js>.
- pandoc <https://github.com/jgm/pandoc> (3.13, 2026-10-26, GPL-2.0), manual <https://pandoc.org/MANUAL.html>,
  Typst writer introduction <https://zenodo.org/records/7776341>.
- GNU GPL FAQ <https://www.gnu.org/licenses/gpl-faq.en.html>.
- Typst `csv()` <https://typst.app/docs/reference/data-loading/csv/>; Universe packages
  <https://typst.app/universe/package/rexllent> (0.4.1, MIT), <https://typst.app/universe/package/spreet> (0.2.0, MIT).
- tex2typst-rs <https://github.com/Unpredictability/tex2typst-rs>.
- Project: `CLAUDE.md` (Invariants), `docs/adr/0024-sidecars-typst-tinymist.md`, `docs/adr/0011-simulation-without-automerge-go.md`,
  `internal/sidecar/manifest.json`, `docs/architecture/decisions.md` (AD-33, AD-98), `docs/exigences.md` (N40, W47),
  `docs/engineering-principles.md` §0, `docs/architecture/coherence.md` §5, `docs/objectif-en-cours.md`,
  `frontend/src/editor/csv/typst.ts`.

## Decisions of the user (2026-11-11)

- Word: N40 stands; no Word import for now (no pandoc, no mapper).
- Excel: both, the safe one first — IM1a, the workbook kept as a binary asset and rendered as a table by a vendored Typst package (no Go code parses it); then IM1b, editable import into the CSV editor via excelize in a confined, bounded child process after a ZIP pre-scan, fuzzed.
- Placement: after the core blocks (C5S, OI, LF0b/LF0c, W46).
- Later the same day: « des imports simples, pourquoi pas, mais que l'on peut versionner ». An import must produce versionable text whose history shows the differences (CSV, Typst, Markdown). So IM1a (the workbook kept as a binary asset) is dropped; the lot is IM1, `.xlsx` → CSV via excelize in a confined, bounded child process, with the Typst table reading that CSV (`csv()`), and the focus stays on Typst and Markdown for now.
