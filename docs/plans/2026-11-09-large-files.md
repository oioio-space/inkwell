# Large files: open several MB, stay fluid (study, 2026-11-09)

User request (2026-11-09): « ouvrir des fichiers texte et autres de plusieurs Mo sans problème et rester
fluide en permanence, comme plusieurs IDE », extended the same day: « la recherche et le remplacement et les
autres opérations d'édition doivent marcher extrêmement bien ; l'appli doit être légère, réactive, extrêmement
rapide ». This study measures the pinned toolchain, compares it with ADR 0028 (proposed) and proposes
amendments. Nothing is decided here; the amendments need an ADR update and a frozen gate (§7).

## 1. Summary

- **CodeMirror is never the limit.** A 50 M-character state is created in 75 ms. A keystroke costs 15 to 55 µs
  at any size. In headless Chromium, with highlighting off, a 50 MB file opens in 45 to 56 ms, scrolls with no
  long task and types at p95 ≤ 18 ms.
- **The editor already freezes on files it allows today, because of the Typst highlighter.** With
  `codemirror-lang-typst/lezer` 0.6.0, the cost of a keystroke grows with its distance from the start of the
  file. In the middle of the file it takes 49 ms at 2 MB, 122 ms at 5 MB, 190 ms at 10 MB and 1.3 s at 50 MB.
  The Markdown parser, line wrapping and our search highlight stay flat. This is the cheapest and largest win.
- **Automerge 3.5.0 has not changed since ADR 0028.** It is still the latest JS release.
  - Applying one remote keystroke costs O(n): 0.24 s at 1 M characters, 1.3 s at 5 M, 2.3 s at 10 M, 4.6 s at
    20 M and 12 s at 50 M.
  - Loading costs about 135 B of RSS per character, and a 50 M document runs out of WASM memory.
- **Replace-all in a shared document is the worst operation measured.** The app's `replaceAll` makes one
  `A.splice` per occurrence inside one change, and each splice costs O(n). 5 000 occurrences take 3.1 s at
  1 MB, 13 to 19 s at 5 MB and 32 to 39 s at 10 MB, all on the main thread. `A.updateText` is 3 to 7 times
  worse. The same replace-all as one CodeMirror ChangeSet takes 89 ms at 10 MB.
- **Recommendation:**
  - a solo editing mode outside Automerge for any file that nobody else shares (§5a). It is IDE-like and fluid
    up to tens of MB;
  - the Typst highlighter fixed, and off above 512 Ki until then;
  - search and replace moved off the main thread and chunked;
  - collaborative tiers raised only modestly once D2 is in: Fluid to 512 Ki, Grand to 4 Mi;
  - a frozen gate `LF1` built from new PERF-21 to PERF-31 cases.

## 2. State of the art (sources)

| Editor | Buffer | Above a threshold |
|---|---|---|
| VS Code | piece tree: piece table, red-black tree, per-node line-break cache; memory close to file size, against 20x for the old line array ([blog](https://code.visualstudio.com/blogs/2018/03/23/text-buffer-reimplementation)) | **20 MB or 300 000 lines** (`LARGE_FILE_SIZE_THRESHOLD`, `LARGE_FILE_LINE_COUNT_THRESHOLD`): never tokenized; folding, sticky scroll and wrapping off. **50 MB** (`_MODEL_SYNC_LIMIT`): not synced to extensions. **256 M chars**: `getValue()` and replace-all refused ([textModel.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/model/textModel.ts), [folding.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/contrib/folding/browser/folding.ts), [viewModelImpl.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/viewModel/viewModelImpl.ts)). Lines over 20 000 chars are not tokenized; rendering stops at 10 000 chars ([editorOptions.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/config/editorOptions.ts)) |
| Zed | rope on a SumTree (B+ tree, 128-byte chunks, copy-on-write `Arc` root, O(log n) seeks) ([blog](https://zed.dev/blog/zed-decoded-rope-sumtree)) | no documented threshold. Opens 1.2 GB, but a 1.5 M-char single line hangs it ([#16120](https://github.com/zed-industries/zed/issues/16120), closed as stale). The proposal there is an Emacs-like "large file → plain text" |
| Sublime Text | not documented | ST4 skips syntax detection on large files and opens them as plain text (moderator, [forum](https://forum.sublimetext.com/t/sublime-text-4-removes-color-when-opening-big-files/59224)). Search highlight is off above about 30 000 lines (user report, [forum](https://forum.sublimetext.com/t/enable-search-highlighting-for-large-files/59265)) |
| IntelliJ | — | `idea.max.intellisense.filesize` = 2 500 KB (no coding assistance); `idea.max.content.load.filesize` = 20 000 KB; above that, a read-only large-file viewer ([docs](https://www.jetbrains.com/help/idea/file-idea-properties.html)) |
| CodeMirror 6 | balanced tree of lines; draws the viewport plus a 1 000 px margin; height scaling above 7e6 px ([viewstate.ts](https://github.com/codemirror/view/blob/main/src/viewstate.ts)) | "a few million lines" ([million](https://codemirror.net/examples/million/)). Parsing is lazy with budgets: 20 ms per change, 25 to 100 ms slices, at most 1e5 chars ahead of the viewport, background parsing paused after 3 s per 30 s ([language.ts](https://github.com/codemirror/language/blob/main/src/language.ts)). Search highlights only `visibleRanges`; `replaceAll` is one transaction; `selectSelectionMatches` refuses more than 1 000 matches ([search.ts](https://github.com/codemirror/search/blob/main/src/search.ts)) |

What the editors converge on:
1. A tree buffer, which CodeMirror already has.
2. Tokenization and highlighting off above about 20 MB, or for lines longer than about 10 to 20 K chars.
3. Folding, sticky headers and wrapping off with them.
4. Hard refusal of whole-text operations only around 256 M chars.

Find and replace:
- The VS Code find widget decorates at most 19 999 matches ([findModel.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/contrib/find/browser/findModel.ts)).
- From 19 999 matches on, VS Code replaces all with one regex replace on the whole text and a single full-range edit (`_largeReplaceAll`).
- VS Code delays re-searching above 50 MB.

**Automerge.**
- 3.0 keeps history compressed at runtime: 10x less memory or more; Moby Dick pasted costs 700 MB in v2 against 1.3 MB in v3 ([blog](https://automerge.org/blog/automerge-3/)).
- 3.5.0 (16 Sep) is still the latest JS release. It fixed a large-change regression ([releases](https://github.com/automerge/automerge/releases)).
- The "Hexane" storage engine is announced as 2 to 9x faster on load ([July 2026](https://automerge.org/blog/2026-july/)). It is not measurable in 3.5.0: our numbers are ADR 0028's.
- [automerge-repo#339](https://github.com/automerge/automerge-repo/issues/339) (a 10 MB document blocks the main thread) is still open.
- On the dmonad B4 editing trace, Automerge takes 14.3 s against 3.1 s for Loro and 5.7 s for Yjs. Parsing the result takes 1.8 s against 13 and 39 ms. B4×100 (about 10 M chars) is skipped for Automerge ([crdt-benchmarks](https://github.com/dmonad/crdt-benchmarks)).

**"Feels light" references.**
- Fatin's Typometer, average and maximum per key: GVim 0.9 / 1.2 ms, Sublime 8.2 / 35 ms, IDEA 24.7 / 84 ms ([Typing with pleasure](https://pavelfatin.com/typing-with-pleasure/)).
- Dan Luu: 10 ms is noticeable and 100 ms is terrible for direct input ([term-latency](https://danluu.com/term-latency/)).
- Zed targets 120 fps, with frames measured under 4 ms ([120fps](https://zed.dev/blog/120fps)).
- A long task is anything over 50 ms ([web.dev](https://web.dev/articles/optimize-long-tasks)).
- RAIL: respond within 100 ms, frames of 10 to 16 ms, idle work in chunks of 50 ms or less ([RAIL](https://web.dev/articles/rail)).
- INP is "good" at 200 ms or less at p75 ([INP](https://web.dev/articles/inp)). That is too lax for an editor and is used here only as an outer bound.

## 3. Method

- **Scripts.** Scratch, outside the repo: `$SCRATCH/bigfile/` (gen.mjs, cmbench.ts, ambench.cjs, repbench.ts, amrep.cjs, web/page.ts, web/drive.cjs).
- **Corpus.** Generated deterministically, with mostly Latin-1 characters:
  - Typst prose (headings, figures, maths, lists, accented French) at 1, 2, 5, 10, 20 and 50 M chars;
  - CSV and log files at 1, 10 and 50 M chars.
- **Versions.** The pinned `frontend/package.json`: @codemirror/state 6.7.6, view 6.43.15, language 6.13.1, search 6.7.2, codemirror-lang-typst 0.6.0, @automerge/automerge 3.5.0. Node 22.23.1.
- **Heavy jobs.** Every run went through `scripts/heavy.sh timeout …`.
- **Reproducibility.** One process per size and mode, so memory peaks (`VmHWM`) are clean. Our `src/editor/search` code and highlighter are bundled with rolldown 1.2.15.
- **Browser.** Headless Chromium 1248 through Playwright 1.64.1, on a standalone page using the app's editor extensions: line numbers, history, drawSelection, active line, `lineWrapping`, `languageExtension`, `searchHighlight`, and the app's theme `contain: strict`.
  - Not measured: Automerge sync, presence, the Wails Streams.
  - Typing is measured from keydown to the first task after the next frame. At 60 Hz it therefore includes up to one frame of waiting.
- **Not done:**
  - Playwright WebKit cannot launch on this Fedora host (missing system dependencies). Native WebKitGTK certification stays with `frontend/src/perf/native.ts` (PERF-01).
  - The runs were not `--exclusive`, and other heavy jobs ran at the same time. A 1 MB build varied from 0.5 to 2.6 s between runs, so treat ±30 % as noise on Automerge wall times.
  - Startup, idle CPU, disk save through the Stream, and Typst preview compile time were not measured. §7 adds them as cases.

## 4. Measurements

### 4.1 CodeMirror EditorState, headless Node (median of runs, 300 random one-char edits)

| Text | create | keystroke p50 / p99 | `doc.line(n)` | sliceDoc all (copy) | 1 Mi paste / undo | Typst full parse | Markdown full parse | RSS |
|---|---|---|---|---|---|---|---|---|
| 1 M | 3 ms | 55 / 867 µs | 1.9 µs | 1.1 ms | 2.1 / 0.4 ms | 86 ms | 103 ms | 139 MB |
| 2 M | 3.7 ms | 23 / 310 µs | 2.0 µs | 1.9 ms | 2.3 / 0.7 ms | 155 ms | 212 ms | 187 MB |
| 5 M | 9.4 ms | 21 / 487 µs | 2.3 µs | 4.7 ms | 2.7 / 0.5 ms | 333 ms | 446 ms | 337 MB |
| 10 M | 14 ms | 18 / 317 µs | 2.5 µs | 5.8 ms | 2.4 / 0.6 ms | 674 ms | 652 ms | 363 MB |
| 20 M | 25 ms | 15 / 294 µs | 2.0 µs | 5.9 ms | 1.9 / 0.3 ms | 1.32 s | 1.11 s | 446 MB |
| 50 M | 75 ms | 21 / 357 µs | 3.0 µs | 18 ms | 2.2 / 0.4 ms | 3.14 s | 3.56 s | 895 MB |
| CSV 50 M | 186 ms | 18 / 362 µs | 3.2 µs | 54 ms | 6.3 / 0.4 ms | 2.65 s | 2.60 s | 1.7 GB |

**First screen.** Highlighting the first screen costs 1 to 3 ms in Typst at every size.
- In Markdown it is O(paragraph): a log or CSV without blank lines costs 0.5 s at 10 MB and 2.7 s at 50 MB.
- A per-line `foldable` scan of a log parsed as Markdown took 83 s at 50 MB. This is pathological, and the app has no fold gutter today.

### 4.2 Headless Chromium, the app's editor configuration (Typst highlighter unless noted)

| File | open (build + 2 frames) | smooth scroll p95 | jump scroll max (long tasks) | typing in the middle p50 / p95 | JS heap |
|---|---|---|---|---|---|
| 1 MB, wrap + hl (app) | 25 ms | 18.6 ms | 32 ms (0) | 27.5 / 35 ms | 25 MB |
| 1 MB, no wrap, no hl | 22 ms | 17.0 ms | 19 ms (0) | 11.4 / 12.9 ms | 11 MB |
| 2 MB, wrap + hl (app) | 29 ms | 18.9 ms | 39 ms (0) | 35 / 45 ms | 14 MB |
| 5 MB, wrap + hl (app) | 30 ms | 18.7 ms | 126 ms (2) | 121 / 129 ms | 42 MB |
| 10 MB, wrap + hl (app) | 30 ms | 18.0 ms | 262 ms (2) | **251 / 296 ms** | 77 MB |
| 10 MB, wrap only | 39 ms | 17.1 ms | 20 ms (0) | 8.6 / 17.3 ms | 23 MB |
| 10 MB, hl only | 32 ms | 18.9 ms | 191 ms (2) | 191 / 239 ms | 50 MB |
| 10 MB, Markdown hl, no wrap | 30 ms | 18.9 ms | 105 ms (5 × ~100 ms) | 10.9 / 21 ms | — |
| 50 MB, wrap + hl (app) | 55 ms | 18.0 ms | 938 ms (2) | **923 / 1 247 ms** | 364 MB |
| 50 MB, wrap only | 50 ms | 18.1 ms | 19 ms (0) | 8.0 / 16.8 ms | 64 MB |
| 50 MB, no wrap, no hl | 45 ms | 17.1 ms | 19 ms (0) | 7.5 / 18.1 ms | 72 MB |
| log 50 MB, wrap + hl | 60 ms | 18.7 ms | 2 001 ms (2) | 1 917 / 2 925 ms | 269 MB |
| log 50 MB, wrap only | 56 ms | 17.0 ms | 19.5 ms (0) | 8.5 / 17.6 ms | 72 MB |

**The Typst highlighter.** Its per-keystroke cost is about 25 to 35 ms per million characters before the
caret. CodeMirror's incremental parse should reuse the tree, which suggests the grammar's node structure
defeats fragment reuse; this needs confirming upstream.

**Other operations, all sizes.** None makes a long task:
- select-all, one frame (17 to 21 ms);
- go to line, two frames (about 31 ms);
- dispatching a search highlight, one frame (2 to 17 ms).

### 4.3 Search (app engine `scanSync`; CodeMirror cursors for comparison), Node

| Text | literal: 1st / 20 000 / all | regex `pair\w*\|\d{4,}`: 1st / 20 000 / all | accent-folded literal: all | CM `SearchCursor` (case-fold) | CM `RegExpCursor` | `structuredClone(text)` |
|---|---|---|---|---|---|---|
| 1 M | 0.3 / 3.1 / 4.4 ms (4 559) | 0.1 / 2.7 / 2.0 ms | 37 ms | 110 ms | 7 ms | 0.8 ms |
| 10 M | 0.3 / 7.2 / 21 ms (45 719) | 0.2 / 7.4 / 21 ms | **276 ms** | 729 ms | 30 ms | 11 ms |
| 50 M | 0.8 / 18.5 / 77 ms (228 499) | 0.2 / 7.2 / 108 ms | **1.45 s** | 3.99 s | 166 ms | 50 ms |
| CSV 50 M | 0.3 / 18.5 / 60 ms | 0.2 / 8.7 / 796 ms (2.8 M hits) | 1.12 s | 3.62 s | 848 ms | 49 ms |

What these numbers mean for the app:
- The app's engine is fast: a first match in under 1 ms, and 20 000 matches in under 20 ms even at 50 MB.
- Unguarded queries run on the main thread, among them every literal with "ignore accents". They become long
  tasks from about 2 MB (accent folding) and from about 30 MB (plain literal "all").
- CodeMirror's case-folding `SearchCursor` is 50x slower than our regex. It must not be used for counting.
- The regex worker receives the whole text on every query: a 50 ms copy at 50 MB, on the main thread at
  `postMessage` time.

### 4.4 Replace-all

**CodeMirror, one transaction, Node.** Every occurrence of « réseau » is replaced by « lien ».

| Text (occurrences) | one ChangeSet: apply / undo | `String.replace` + one full-range edit / undo | slices of 2 000 changes: worst slice |
|---|---|---|---|
| 1 M (4 559) | 26 / 13 ms | 1.9 / 0.1 ms | 13 ms |
| 10 M (45 719) | 89 / 63 ms | 19 / 0.2 ms | 15 ms |
| 50 M (228 499) | 454 / 476 ms | 186 / 0.2 ms | 13 ms |

The full-range edit is VS Code's large-replace strategy. It is cheap, but it moves every caret, mark and remote
cursor to the edges, so it suits the solo mode only.

**Automerge, as `replaceAll` does today.** One `A.change` and one splice per occurrence, capped at 5 000.

| Text | 5 000 splices | `A.updateText` (same result) | peak RSS |
|---|---|---|---|
| 1 M | 3.0 to 3.5 s | 17 s | 0.4 GB |
| 5 M | 13.5 to 19 s | 62 s | 2.0 GB |
| 10 M | 32 to 39 s | — | 1.7 to 2.4 GB |
| 20 M | 76 s | — | 2.5 GB |
| 50 M | 186 s | — | 3.3 GB |

### 4.5 Automerge 3.5.0 (Node; "built" means 64 Ki changes as D3, "from" means one change)

| Text | built (D3): time / peak | one change (`from`): time / peak | load: time / RSS | local 1-char change | **remote 1-char change applied** | save: size |
|---|---|---|---|---|---|---|
| 1 M | 0.5 to 2.6 s / 97 MB | 0.9 s / 327 MB | 0.49 s / 214 MB | 0.4 to 2.9 ms | **0.24 to 0.37 s** | 171 KB (0.17 B/char) |
| 2 M | 1.1 s / 99 MB | 1.8 s / 580 MB | 0.91 s / 347 MB | 0.8 to 1.2 ms | **0.48 s** | 336 KB |
| 5 M | 2.4 s / 116 MB | 4.8 s / 1.69 GB | 2.0 s / 743 MB | 1.8 ms | **1.25 to 1.32 s** | 833 KB |
| 10 M | 4.3 s / 135 MB | 10.9 s / 3.30 GB | 4.0 s / 1.39 GB | 3.2 to 5.6 ms | **2.24 to 2.82 s** | 1.65 MB |
| 10 M CSV / log | 5.0 / 4.7 s | — | 4.2 / 5.0 s, 1.39 GB | 4.6 / 3.6 ms | **2.5 / 2.4 s** | 2.8 / 2.0 MB |
| 20 M | 8.7 s / 195 MB | — | 8.8 s / 2.71 GB | 5.8 ms | **4.6 s** | 3.3 MB |
| 50 M | 29 s / 301 MB | — | **out of memory** (`error inflating document chunk ops`) | 19.6 ms | **12 s** | 8.2 MB |

- The remote apply cost is the same with and without a `patchCallback`.
- **A built document is much lighter than a loaded one.** It stays at about 6 B/char until it is saved and
  reloaded; after loading it costs about 135 B/char. Every peer that opens or receives the document pays the
  "load" figure.

### 4.6 What changed against ADR 0028 (2026-10-26)

| Item | ADR 0028 | Now | Verdict |
|---|---|---|---|
| CodeMirror: create, keystroke | 100 Mio in 219 ms, 11 µs | 50 M in 75 ms, 15 to 55 µs | same |
| Load 1 / 5 / 10 / 20 Mi | 0.48 / 2.6 / 4.3 / 9.4 s; 145 MB / 660 MB / 1.3 GB / 2.6 GB of WASM | 0.49 / 2.0 / 4.0 / 8.8 s; 214 MB / 743 MB / 1.39 GB / 2.71 GB of RSS | same: same 3.5.0 binary, RSS includes the JS heap |
| Remote apply, 1 Mi | 240 to 420 ms | 240 to 370 ms | same |
| Remote apply, 5 Mi | 3.3 s | 1.3 s | the ADR's figure included the CodeMirror diff; the core cost is the same |
| One-change paste | about 300 B/char | 330 B/char (10 M: 3.3 GB) | same; D3 stays essential |
| 50 Mi document | not tried | load runs out of memory; remote apply 12 s | new: a hard ceiling |
| Replace-all, typing deep in a Typst file | not measured | §4.4, §4.2 | new: both break the 16 ms budget at sizes the app allows today (≤ 2 Mi) |

## 5. Proposed amendments to ADR 0028

### (a) D11 — solo editing outside Automerge (recommended)

**When.** A text file is in solo mode when no one else can see it:
- a loose file outside any project;
- or a file of a project whose member list (the signed list) has only its owner and no pending invitation.

**How it works.**
- CodeMirror holds the text. The disk file is the single source of truth: no Automerge document exists, so the
  invariant "one source of truth per thing" still holds.
- Saving and autosave follow the user's autosave choice (T11). Saving writes `doc.toString()` (19 ms at 50 MB)
  through the `store` Stream in chunks of 1 MiB at most, so no single message is large.
- External edits are merged by the existing B1 three-way machinery (ExternalSync), with the last saved text as
  the base.
- Undo is CodeMirror's history.
- Versions are go-git snapshots at save, as today's projection.

**Limits.**
- Fluid without degradation up to 50 MB once the Typst fix (b1) is in. Measured: open ≤ 60 ms, typing p95
  ≤ 18 ms, no long task while scrolling (§4.2).
- Memory is about 1.4 B/char of JS heap in Chromium without the highlighter. The budget is ≤ 2 B/char + 50 MB.
- Above 64 Mi the file opens in the paged read-only view (D4), as in the ADR.

**Becoming shared.** When the file becomes shared (an invitation, or a first person joining):
- up to the collaborative limit, the document is built in the Worker in 64 Ki changes (D3). This takes 2.4 s and
  peaks at 116 MB for 5 M, with visible progress. The disk text becomes the projection again;
- above the limit, the file is shared as a read-only blob (ADR 0023). The owner keeps editing it in solo, and
  each save publishes a new version, as for an image (PRJ-13);
- the person is told before sending the invitation, never after (see (d)).

**What it needs.**
- An ADR that amends ADR 0026 and ADR 0028 D1, and reads ADR 0011 as "text that is shared lives in Automerge".
- A single `EditorBackend` seam in `createEditor`: `amSync` or `diskSync`. The size gate (`limits.ts`) then
  chooses among three outcomes instead of two.

**Risks.**
- Two editing paths. Mitigation: one CodeMirror setup, with only the sync extension swapped, and the same e2e
  run in both modes.
- A project that turns shared while a big file is open. Mitigation: this is the explicit conversion above.

### (b) Tiers and degradations for collaborative documents

- **(b1) Typst highlighter, now.**
  - Turn it off above 512 Ki characters. Measured: about 30 ms per M chars before the caret, so 512 Ki caps
    the extra cost near 16 ms at the end of the file; 256 Ki would keep it under 8 ms.
  - Find why fragment reuse fails in `codemirror-lang-typst/lezer` 0.6.0, report it upstream with the script,
    and lift the threshold once the PERF-28 gate passes.
  - Markdown highlighting stays on to 20 MB. It is plain text when a single block is over 1 Mi, which covers
    logs saved as .md.
- **(b2) Tiers once the Worker (D2) is in.**
  - Remote apply then becomes latency, not a freeze.
  - **Fluid ≤ 512 Ki**: 104 to 120 ms of apply plus the 100 ms debounce, inside the 250 ms LAN budget.
  - **Grand ≤ 4 Mi**: about 1 s of apply plus D7 pacing (100 ms + 0.25 ms × Ki, so 1.1 s at 4 Mi), which keeps
    the "slight delay" banner honest. Load memory at 4 Mi is about 560 MB of RSS, under the 1 GB Worker budget.
  - `MAX_GROWN_CHARS` rises from 4 Mi to 6 Mi.
  - **Do not go higher.** At 10 M, a remote keystroke takes 2.3 to 2.8 s, a local one 3 to 6 ms of WASM, and a
    load 1.39 GB. At 20 M: 4.6 s and 2.7 GB. At 50 M the document cannot be loaded at all.
- **(b3) Before D2.**
  - The code already allows shared editing from 512 Ki to 2 Mi (`SLOW_CHARS`, `MAX_EDITABLE_CHARS`), which
    means 0.1 to 0.5 s of freeze per received batch. That is against ADR 0028's own pre-D2 rule (Grand stops
    at 512 Ki).
  - Keep it only because the solo mode (a) removes the freeze for the common case of a file nobody else has.
    Otherwise lower `MAX_EDITABLE_CHARS` to 512 Ki until D2.
- **(b4) Reexamination trigger unchanged.** The trial of Loro is now motivated by three numbers: remote apply,
  replace-all and the 50 M load OOM.

### (c) What degrades above thresholds, and what the person reads

| Feature | Threshold (measured) | Instead | Words (UI, French) |
|---|---|---|---|
| Typst colours | > 512 Ki (until b1 fix) | plain text | « Couleurs désactivées : fichier très long. » in the status bar, with a « Réactiver » action |
| Markdown or plain colours | > 20 MB, or a block > 1 Mi | plain text | same message |
| Line wrapping | none: measured harmless up to 50 MB | keep | — |
| Lines > 20 000 chars | not measured; Zed hangs at 1.5 M (PERF-31) | colours off for that line, then like VS Code past 10 000 chars | « Ligne très longue : affichée sans couleurs. » |
| Accent-insensitive or "all" search | > 1 Mi | Worker (§6) | progress in the panel, « 12 340 résultats… » |
| Typst preview | not measured (PERF-29) | pause when a compile exceeds 2 s; compile on demand | « Aperçu en pause : document très long. Actualiser » |
| Replace-all, shared document | > 500 occurrences or > 1 Mi | Worker, in changes of ≤ 200 splices, with progress and cancel | « Remplacement : 1 200 sur 5 000… Annuler » |
| Multi-cursor on every match | > 1 000 matches (CodeMirror refuses) | keep CodeMirror's guard; say it | « Trop d'occurrences pour les sélectionner toutes (plus de 1 000). » |

Every degradation:
- appears in one place, the status bar notice, which follows the banner and toast patterns;
- is reversible by the person;
- is never silent.

### (d) Honest limits, and the words at sharing time

- **Hard ceilings.**
  - A shared editable text holds 4 Mi (Grand, after D2); a single document 6 Mi at most.
  - Automerge cannot load about 30 to 50 M characters, because of the 4 GiB WASM limit shared by all open
    documents.
  - In solo, there is no edit limit up to 64 Mi; the paged view takes over above.
  - A blob can be shared up to the 100 MB quota.
- **Before inviting someone** to a project with a solo file over 4 Mi, the invitation dialog says:
  « main.log (38 Mo) est trop long pour être modifié à plusieurs. Les autres pourront le lire ; vous seul
  pourrez le modifier. » with « Inviter quand même » and « Annuler ».
  - The same sentence appears in the file's info popover.
  - No number other than the size, and no word like CRDT, document or Automerge (glossary).
- **When a peer's growth pushes a shared document past 6 Mi**, the existing D1 rule stays: it switches to
  read-only locally, with the message « Ce fichier est devenu trop long pour être modifié à plusieurs. Vous
  pouvez le lire ; pour le modifier, faites-en une copie personnelle. » with the action « Faire une copie »,
  which creates a solo copy.

### (e) Executable proofs (see §7)

The budgets become PERF-21 to PERF-31. They are written into a frozen gate `gates/LF1.toml` before any code, and
checked at two levels:
1. a component harness in headless Chromium: this study's page, committed under `frontend/e2e/perf/`, fast and
   run in CI;
2. the real app, server mode and the native WebKitGTK harness (`frontend/src/perf/native.ts`), for
   certification.

## 6. Search and replace without blocking

- **The text stays in the Worker.** The search Worker keeps a mirror of the open document, and each
  transaction sends its `ChangeSet.toJSON()`, a few bytes per key. A query then sends only the matcher, never
  the 50 ms copy of the text. In collaborative mode, after D2, the Automerge Worker already holds the text and
  can do the same.
- **First match first.** The scan starts at the caret, in 1 Mi chunks: 0.3 to 1.5 ms per chunk for a literal.
  It posts the first match at once, which costs under 5 ms at any size. The count then streams up
  (« 12 340… ») until done, with cancellation when the query changes.
- **Time limit.** It becomes "no progress for 200 ms per chunk" instead of 200 ms for the whole job. That keeps
  catastrophic-regex protection without failing a legitimate 0.8 s count on 50 MB. A match-list cap of 20 000
  stays for the panel, and the count is exact.
- **Highlight.** It stays viewport-only, as today (`MAX_SPAN` 200 000): one frame at 50 MB. Literal queries
  stay inline for the viewport.
- **Replace-all, solo mode.**
  - Up to 50 000 occurrences: one CodeMirror ChangeSet, which keeps carets and marks (89 ms at 10 MB). The
    ChangeSet is built in the Worker and applied in one dispatch.
  - Above 50 000 occurrences or 10 MB: VS Code's strategy. The new text is built in the Worker with
    `String.replace`, followed by one minimal-diff edit covering the span from the first match to the last
    (186 ms at 50 MB), announced by the progress line. A single undo step either way.
- **Replace-all, shared document.**
  - The Worker applies changes of ≤ 200 splices, about 0.6 s each at 5 Mi, with progress and cancel.
  - It is one undo group in `localHistory` and gets one attestation at the end, as D3 does for pastes.
  - Above 4 Mi a document is never shared, so the worst case is bounded at about 40 s for 5 000 occurrences in
    4 Mi. It is announced, and the editor is never blocked.
  - Upstream: report the O(n)-per-splice cost with `amrep.cjs`.
- **Project search.** It reads Go-side projections (D9, PERF-16), streamed by file as planned.

## 7. Budgets and gates

**Budgets for "feels light".**

| Measure | Budget | Basis |
|---|---|---|
| Key → paint, every tier and mode | p50 ≤ 8 ms, p95 ≤ 16 ms (native WebKit); ≤ 20 ms in the headless harness at 60 Hz | Zed 120 fps; Sublime 8.2 ms on average (Fatin); Luu's 10 ms |
| Any interaction → response | ≤ 100 ms; progress shown beyond that | RAIL |
| Main-thread task | none over 50 ms, while opening, scrolling, typing, searching, replacing and saving | long-task definition, RAIL idle chunks |
| Scroll, smooth and jump | frame p95 ≤ 20 ms (harness); no frame over 50 ms | — |
| Open a local file | first paint ≤ 300 ms up to 64 Mi; editable at the same time in solo | ADR 0028; 55 ms measured at 50 MB |
| Startup | window painted ≤ 1 s; last project editable ≤ 1.5 s | ADR 0028 |
| Idle, 60 s with a 50 MB file open | WebKit CPU ≤ 1 %; no timer under 1 s; memory flat (±5 MB) | own rule; no editor publishes one |
| Memory, solo | ≤ 2 B/char + 50 MB of JS heap | 1.4 B/char measured |

**New PERF cases** (Q-PERF in the T11 plan). Each runs on 10 MB and 50 MB Typst and on a 50 MB log, and each
records long tasks: `PerformanceObserver('longtask')` in Chromium, and the rAF-gap detector in WebKit, which
has no long-task API.

| Id | Case | Pass when |
|---|---|---|
| PERF-21 | open from the disk (solo) | first paint ≤ 300 ms; no long task |
| PERF-22 | smooth scroll of 150 frames, then 60 jumps | p95 ≤ 20 ms; max ≤ 50 ms |
| PERF-23 | 40 keys at 10 %, 50 % and 90 % of the file | p95 ≤ 20 ms (harness), 16 ms (native) at every position |
| PERF-24 | find a literal, an accent-folded literal and a regex | first match ≤ 50 ms; count streamed; no long task; a regex runaway still stopped |
| PERF-25 | replace-all in solo, all occurrences | no long task; one undo step that restores the text exactly; ≤ 0.5 s at 10 MB |
| PERF-26 | replace-all in shared mode, 5 000 occurrences at 2 Mi | no long task; progress ≤ 1 s; cancel works; one undo group; one attestation |
| PERF-27 | select-all + copy, go to line, undo of a 1 Mi paste, multi-cursor guard | no long task; the guard message is shown |
| PERF-28 | Typst parse after an edit at 90 % of a 5 Mi doc (Vitest) | `ensureSyntaxTree` work ≤ 5 ms and independent of the position (ratio 10 % vs 90 % ≤ 1.5) |
| PERF-29 | Typst preview on a 2 Mi and a 10 MB document | typing budget holds during compile; pause message beyond 2 s |
| PERF-30 | idle 60 s with 50 MB open | CPU ≤ 1 %; RSS flat |
| PERF-31 | a single 10 MB line, and a 50 MB minified JSON | open ≤ 300 ms; typing p95 ≤ 20 ms; no long task |

**Gate `LF1`.**
- It is frozen before the code (skill `new-gate`) and lists PERF-21 to PERF-28 and PERF-30 to PERF-31 as
  failing criteria with `INKWELL_PERF=1`.
- PERF-29 is measured first and turns into a criterion once its threshold is known.
- Mutants the gate must catch:
  - the Typst threshold removed;
  - the accent-fold search moved back inline;
  - replace-all as one Automerge change;
  - the search time limit set back to a whole-job 200 ms.
- The component harness reruns at every `@codemirror/*`, `codemirror-lang-typst` or `@automerge/*` update, next
  to PERF-18.

## 8. Ranked work

| # | Work | Gain | Cost | Risk |
|---|---|---|---|---|
| 1 | Typst highlighter off above 512 Ki (status-bar notice) + PERF-23/28 | removes 50 ms to 1.3 s per key on files the app allows today | S (1 day) | low |
| 2 | Search Worker with a document mirror, caret-first chunked scan, streamed count, per-chunk time limit; accent-fold off the main thread | first match ≤ 5 ms at any size; no 0.3 to 1.4 s freezes | S to M (2 to 3 days) | low |
| 3 | Replace-all: one ChangeSet in solo; chunked Worker changes with progress in shared mode | 3 to 39 s freezes → none | M (3 days) | medium: undo grouping and the single attestation (ADR 0018) |
| 4 | Component perf harness + gate LF1 (PERF-21 to PERF-31) | makes "fluid" a regression test | M (3 days) | low |
| 5 | D11 solo mode (ADR, `EditorBackend` seam, disk sync, sharing conversion and the invitation message) | "several MB is never a problem": 50 MB edited at 18 ms p95 | L (1 to 2 weeks) | medium to high: a new ADR touching ADR 0011/0026, two sync paths, external-merge base |
| 6 | Upstream: Typst grammar fragment reuse; Automerge O(n) remote apply and per-splice cost (with scripts) | may lift #1 and the tiers | S | none |
| 7 | D2 Worker, then tiers raised to 512 Ki / 4 Mi (b2) | shared Grand files without freezes | XL (as ADR) | high |
| 8 | Typst preview pause and measurement (PERF-29); long-line guard (PERF-31) | covers the remaining unmeasured freezes | M | medium (tinymist behaviour unknown) |
| 9 | D4 paged view above 64 Mi | unchanged from ADR | L | medium |

Items 1 to 4 give most of the felt speed without new architecture; item 5 makes « plusieurs Mo » true for unshared files.
