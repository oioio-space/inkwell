# How editors handle heavy files, and what inkwell should adopt (study, 2026-11-09)

Companion to `docs/plans/2026-11-09-large-files.md` (« LF » below) and ADR 0028 (« ADR »). LF already holds
today's measurements and the amendments D11 (solo mode), the Typst highlighter cap, the search Worker and the
one-transaction replace-all; they are not repeated here. Web research only: no measurement was run, because a
native performance gate was running on the machine. Every claim cites a primary source (source code,
official documentation or an engineering blog). Where the only evidence is a forum post, the text says so.

## 1. Summary

- **Every serious editor does the same three things.** It decides the degradation **at open, from the
  size**. It keeps a **tree buffer whose snapshot is O(1)**. It does the expensive work (tokenizing, parsing,
  searching) **under a time budget or off the UI thread, on a snapshot**. inkwell's plan already does all three
  for rendering and the buffer. The gaps are in **highlighting**, **memory accounting** and **file I/O safety**.
- **New beyond LF and the ADR**, in order of value: (1) a **runtime time-budget kill switch** for highlighting
  (Vim `redrawtime`); (2) **Typst highlighting from `typst-syntax`'s incremental reparser**, as flat token
  arrays, to lift the 512 Ki cap; (3) a **byte budget on undo** (Emacs `undo-outer-limit`); (4) a **size and
  time cap on the external-edit merge** (VS Code diff); (5) **save from a streamed snapshot** with an mtime +
  size conflict check; (6) **no `mmap`** in `internal/textview`; (7) **follow mode** for growing files; (8) a
  **memory pressure message** (Figma's alert and recovery mode); (9) **end-to-end latency certification** and
  the Event Timing API.
- **One contradiction with LF (§8).** VS Code refuses to mirror models over 50 MB to workers, because a mirror
  doubles memory. LF's search Worker keeps a mirror of the text up to 64 Mi, and LF's solo budget
  (≤ 2 B/char + 50 MB) does not count that copy. Two cautions follow it.

## 2. What each editor does (facts, with sources)

**VS Code.**
- **Buffer.** The piece tree replaced a line array that cost about 20x the file size (35 MB → 600 MB). Its
  memory after loading is "very close to the original file size", and line lookup is O(log N) in the number of
  nodes. Files are read in 64 KB chunks that are kept as separate buffers, because concatenating them hit V8's
  maximum string length (a 500 MB file threw)
  ([blog](https://code.visualstudio.com/blogs/2018/03/23/text-buffer-reimplementation)).
- **Size cutoffs**
  ([textModel.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/model/textModel.ts)).
  - Above 20 MB or 300 K lines, a model is never tokenized. The decision is taken at construction: "If a model
    is too large at construction time, it will never get tokenized".
  - Above 50 MB, the model is not synced to workers or extensions (`_MODEL_SYNC_LIMIT`).
  - Above 256 M chars, `getValue()` throws "Operation would exceed heap memory limits".
  - `createSnapshot()` streams the text in chunks of at least 64 KB.
- **What the person sees**
  ([largeFileOptimizations.ts](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/contrib/codeEditor/browser/largeFileOptimizations.ts)).
  - An info notification: "{0}: tokenization, wrapping, folding, codelens, word highlighting and sticky scroll
    have been turned off for this large file in order to reduce memory usage and avoid freezing or crashing".
  - One action, "Forcefully Enable Features", followed by "Please reopen file…".
  - Opening asks for confirmation ("Open Anyway" / "Configure Limit",
    [editor.ts](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/common/editor.ts)) above
    1 GB locally, 10 MB remote and 50 MB on the web
    ([files.ts](https://github.com/microsoft/vscode/blob/main/src/vs/platform/files/common/files.ts)).
  - `files.maxMemoryForLargeFilesMB` no longer exists in the source: a code search of microsoft/vscode returns
    nothing, and users report it as gone
    ([MS Q&A](https://learn.microsoft.com/en-us/answers/questions/2073201/how-to-allocate-more-ram-visual-studio-code)).
- **Tokenization.**
  - TextMate tokenizes line by line and stores an end state per line. It stops once a line's end state is
    unchanged. Tokens are packed in a `Uint32Array`: 96 B against 648 B for the old objects
    ([blog](https://code.visualstudio.com/blogs/2017/02/08/syntax-highlighting-optimizations)).
  - It now runs in a web worker by default (`editor.experimental.asyncTokenization: true`).
  - Lines over `editor.maxTokenizationLineLength` (20 000) are not tokenized. Tree-sitter is opt-in per
    language
    ([editorConfigurationSchema.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/config/editorConfigurationSchema.ts)).
- **Search and diff.**
  - Workspace search spawns ripgrep
    ([ripgrep.ts](https://github.com/microsoft/vscode/blob/main/src/vs/base/node/ripgrep.ts)). In-file find
    is LF §2.
  - The diff editor stops at `maxFileSize: 50` (MB) and `maxComputationTime: 5000` (ms)
    ([diffEditor.ts](https://github.com/microsoft/vscode/blob/main/src/vs/editor/common/config/diffEditor.ts)).
- **Saving.**
  - A save conflict is detected with an etag made of mtime and size, and the person is "asked to compare"
    ([files.ts](https://github.com/microsoft/vscode/blob/main/src/vs/platform/files/common/files.ts),
    [files.contribution.ts](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/contrib/files/browser/files.contribution.ts)).
  - Atomic temp-and-rename writes (`.vsctmp`) are used for VS Code's own state and backups
    ([stateService.ts](https://github.com/microsoft/vscode/blob/main/src/vs/platform/state/node/stateService.ts)).
- **Encoding**
  ([encoding.ts](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/services/textfile/common/encoding.ts)).
  The BOM is checked first. Then the first 512 bytes are scanned for zero bytes (binary or UTF-16). With
  auto-guessing, jschardet gets 4 KiB to 64 KiB.

**Zed.**
- **Rope.** The rope is a `SumTree<Chunk>`, a B+ tree with chunks of at most 128 bytes. Its summaries carry
  bytes, UTF-16 units, line points and the longest row, so seeks are O(log n) in any dimension. A snapshot is
  "bump a reference count", and every edit sends one to a background thread for Tree-sitter
  ([blog](https://zed.dev/blog/zed-decoded-rope-sumtree)).
- **Reparse.** Zed first tries a synchronous reparse with a **1 ms timeout**. Past it, a background task takes
  over, and the foreground keeps an "interpolated tree"
  ([buffer.rs](https://github.com/zed-industries/zed/blob/main/crates/language/src/buffer.rs)). Queries are
  bounded by `MAX_BYTES_TO_QUERY = 16 * 1024`
  ([syntax_map.rs](https://github.com/zed-industries/zed/blob/main/crates/language/src/syntax_map.rs)).
  Syntax trees are copy-on-write too, so they can be updated "on a background thread even while a previous
  snapshot… is being actively used on the main thread"
  ([blog](https://zed.dev/blog/syntax-aware-editing)).
- **CRDT.** The buffer is a CRDT made of immutable insertions, (insertion id, offset) anchors, tombstones and
  Lamport ordering. Undo is a per-operation counter map. The per-character metadata "isn't an issue on
  modern computing hardware" ([blog](https://zed.dev/blog/crdts)).
- **Targets.** Zed aims for 120 fps and measures frames with the Metal HUD and Instruments
  ([blog](https://zed.dev/blog/120fps)).

**Sublime Text.**
- There are few primary sources. ST4 brought GPU rendering (on by default on macOS only), "Improved
  performance when loading files with very long lines", find "graceful degradation" on large files, and
  "Saving files is now asynchronous" ([ST4](https://www.sublimetext.com/blog/articles/sublime-text-4)).
- User reports blame regex scoping, word wrap and long lines, and give about 1 GB of RAM per GB of file. These
  are forum posts only ([forum](https://forum.sublimetext.com/t/sublime-text-performance-with-very-large-files/9832)).

**IntelliJ.**
- No coding assistance above `idea.max.intellisense.filesize` (2 500 KB). Files above
  `idea.max.content.load.filesize` (20 000 KB) are not opened as normal documents
  ([docs](https://www.jetbrains.com/help/idea/file-idea-properties.html)).
- "The file size exceeds 2.5 MB. Such files are considered too large for editing"
  ([docs](https://www.jetbrains.com/help/idea/read-only-files.html)).
- The platform's `LargeFileEditorProvider` serves too-large files in **100 000-byte pages**, with a 1 000-byte
  page-border shift
  ([PropertiesGetter.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/largeFilesEditor/editor/PropertiesGetter.java),
  [LargeFileEditorProvider.java](https://github.com/JetBrains/intellij-community/blob/master/platform/lang-impl/src/com/intellij/largeFilesEditor/editor/LargeFileEditorProvider.java)).
- "Zero-latency typing" paints the typed character before the document update: 2.9 ms on average against
  24.7 ms by default ([Fatin](https://pavelfatin.com/typing-with-pleasure/)).

**Vim and Neovim.**
- **Buffer.** Lines live in a tree of pointer blocks and data blocks, held in a memfile **backed by the swap
  file**. A block not changed since loading can be re-read from the original file
  ([memline.c](https://github.com/vim/vim/blob/master/src/memline.c)).
- **Limits** ([options.txt](https://github.com/vim/vim/blob/master/runtime/doc/options.txt)): `synmaxcol` =
  3000; `redrawtime` = 2000 ms, past which "syntax highlighting is disabled until CTRL-L"; `maxmempattern` =
  1000 KB per regex; `undoreload` = 10 000 lines (a bigger reload is not kept in undo); `backupcopy=auto`
  renames when safe and copies over the original for links or attributes that would be lost.
- **Neovim 0.11** made Tree-sitter highlighting, folding and injections asynchronous ([news](https://github.com/neovim/neovim/blob/master/runtime/doc/news-0.11.txt)).

**Emacs.**
- **Buffer.** A gap buffer
  ([buffers.texi](https://github.com/emacs-mirror/emacs/blob/master/doc/lispref/buffers.texi)).
- **Big files.** Above `large-file-warning-threshold` (10 MB), Emacs asks y / n / **l** ("visit literally":
  expensive features off) ([files.texi](https://github.com/emacs-mirror/emacs/blob/master/doc/emacs/files.texi)).
- **Long lines.** `long-line-threshold` = 50 000 turns on redisplay shortcuts, which narrow to
  `long-line-optimizations-region-size` = 500 000 around point; `large-hscroll-threshold` = 10 000
  ([buffer.c](https://github.com/emacs-mirror/emacs/blob/master/src/buffer.c)). `so-long-mode` triggers above
  10 000 chars within the first 500 lines
  ([so-long.el](https://github.com/emacs-mirror/emacs/blob/master/lisp/so-long.el)).
- **Undo.** `undo-limit` 160 000, `undo-strong-limit` 240 000, `undo-outer-limit` 24 000 000 bytes (one
  command's undo is dropped with a warning) ([undo.c](https://github.com/emacs-mirror/emacs/blob/master/src/undo.c)).

**Kakoune, Helix, Xi and Lapce.**
- Kakoune keeps a vector of reference-counted line strings (`BufferLines`,
  [buffer.hh](https://github.com/mawww/kakoune/blob/master/src/buffer.hh)). Helix uses Ropey: single-digit-µs
  edits on multi-GB texts, about 10 % memory overhead, clones that share data ([ropey](https://github.com/cessen/ropey)).
  Xi's "rope science" covers metrics, incremental wrap, CRDTs, minimal invalidation
  ([index](https://xi-editor.io/docs/rope_science_00.html)).
- Raph Levien's retrospective ([xi](https://raphlinus.github.io/xi/2020/06/27/xi-retrospective.html)): the
  rope's worst case was the best part; "the process separation between front-end and core was not a good
  idea", async made wrap and scroll "considerably" harder; async incremental highlighting worked, and he would
  now pick Lezer.

**Large-file viewers.**
- klogg reads "directly from disk, without loading it into memory", handles 10+ GB, uses multi-threading and
  SIMD, and detects the encoding with uchardet ([README](https://github.com/variar/klogg)). Its line index is
  delta-compressed per block and keeps constant-time lookup
  ([compressedlinestorage.h](https://github.com/variar/klogg/blob/master/src/logdata/include/compressedlinestorage.h)).
- EmEditor claims 16 TB in a "single-process, multithreaded design", with a Large File Controller that opens
  one range of lines ([EmEditor](https://www.emeditor.com/text-editor-features/large-file-support/)). UltraEdit
  skips its temporary copy above a threshold and groups auto-disabled features under "Large Files"
  ([wiki](https://wiki.ultraedit.com/Settings:File_handling:Temporary_files)). 010 Editor advertises "Huge
  file support (50 GB+)" ([site](https://www.sweetscape.com/010editor/)).

**CodeMirror 6.**
- **Document.** The document is a tree of 32-line leaves with 32-way branches (`Tree.BranchShift = 5`,
  [text.ts](https://github.com/codemirror/state/blob/main/src/text.ts)).
- **Very long lines.** The view hides the parts of a long line outside the viewport behind **line gaps**,
  with a margin of 2 000 chars unwrapped and 10 000 wrapped (`LG.Margin`, `LG.MarginWrap`). Heights are scaled
  above 7e6 px ([viewstate.ts](https://github.com/codemirror/view/blob/main/src/viewstate.ts)). Version
  6.34.1 "Improve[d] the way enormously long (non-wrapped) lines are displayed"
  ([CHANGELOG](https://github.com/codemirror/view/blob/main/CHANGELOG.md)).
- **History.** It keeps `minDepth: 100` events, groups changes within 500 ms (`newGroupDelay`), and has no
  byte cap ([history.ts](https://github.com/codemirror/commands/blob/main/src/history.ts)).
- **Parsing.** It is synchronous, sliced and budgeted (LF §2). Marijn Haverbeke calls this "an intentional,
  well-considered decision", and says Lezer's shared-structure trees "fit very poorly" with worker boundaries
  ([forum, 2021](https://discuss.codemirror.net/t/some-thoughts-and-requests-about-how-cm6-schedules-parsing/3045)).

**Typst.**
- `typst-syntax` has an incremental reparser that "refresh[es] the given syntax node with as little parsing as
  possible" ([reparser.rs](https://github.com/typst/typst/blob/main/crates/typst-syntax/src/reparser.rs)).
- tinymist answers `semanticTokens/full/delta`
  ([semantic_tokens_delta.rs](https://github.com/Myriad-Dreamin/tinymist/blob/main/crates/tinymist-query/src/semantic_tokens_delta.rs)).

## 3. Topics and best practices

**3.1 Data structures.**
- The candidates are a piece tree (O(log nodes) lookup, memory ≈ file), a B-tree rope (Zed, Ropey, Xi:
  O(log n) edits and seeks, snapshot = refcount) and CodeMirror's line tree, which is in the same class.
- Best practices: an O(1) snapshot by structural sharing (background work never copies the text); line and
  UTF-16 summaries (O(log n) conversions); a chunked load, never one concatenated string.
- Line arrays (Kakoune, old VS Code) and gap buffers (Emacs) are simple but lose on memory or on edits far
  from the gap.
- *inkwell:* CodeMirror's `Text` is already right (LF §4.1: `line(n)` takes 2 to 3 µs at 50 M). Do not
  introduce any other text structure in the webview.

**3.2 Parsing and cutoffs.**
- Best practices: (a) decide at open, from the size (VS Code: never tokenized above 20 MB / 300 K lines);
  (b) a small synchronous budget, then background work, with the old tree mapped through the edit (Zed: 1 ms,
  then an "interpolated tree"); (c) a cap per line (VS Code 20 000, Vim 3 000, Emacs 10 000 and 50 000);
  (d) **a runtime time-budget kill switch** (Vim `redrawtime`) for pathological grammars that size misses;
  (e) flat token arrays when highlighting crosses a thread (VS Code's `Uint32Array`); Lezer trees do not
  cross well (Marijn).

**3.3 Rendering.**
- Viewport only, everywhere. Long lines need a separate mechanism: CodeMirror line gaps, Emacs narrowing around
  point, VS Code stops rendering after 10 000 chars (LF §2).
- With line gaps, CodeMirror already bounds the DOM of a 10 MB line. What remains is the parse and the wrap
  measurement of that line (PERF-31).

**3.4 Search and replace.**
- **Search.** ripgrep wins by extracting literals, using a rarest-byte `memchr`, Teddy SIMD for alternations,
  UTF-8 inside the DFA, and scanning whole buffers rather than lines. It uses `mmap` for one big file and read
  buffers for many small ones ([ripgrep](https://burntsushi.net/ripgrep/)).
- **Go equivalents.** `bytes.Index` and `bytes.Count` are assembly (POPCNT and SIMD)
  ([count_amd64.s](https://github.com/golang/go/blob/master/src/internal/bytealg/count_amd64.s)). `regexp` is
  "guaranteed to run in time linear" ([regexp.go](https://github.com/golang/go/blob/master/src/regexp/regexp.go)):
  a runaway regex is impossible in Go, but not in JS.
- **Replace-all and counting** (one undo step, full-text replace above a match count, streamed exact count,
  decorations capped at 19 999): covered by LF.

**3.5 Undo memory.**
- Emacs drops one command's undo above 24 MB, with a warning. Vim skips undo on reloads above 10 000 lines.
  CodeMirror bounds the number of events, not their bytes.
- Thanks to structural sharing, an inverted ChangeSet mostly retains the deleted text. That costs nothing until
  the old text would otherwise have been freed: 100 replace-alls on 50 MB can pin gigabytes.
- *Practice:* a byte budget on retained deleted text, with the oldest events dropped and the drop announced.

**3.6 Open and save.**
- **Read.** Stream it, in 64 KB chunks (VS Code). Detection goes BOM, then the first 512 B (binary or
  UTF-16), then a guess on at most 64 KB (VS Code, klogg's uchardet).
- **Atomic save.** Write a temp file on the same filesystem, fsync it, rename it, fsync the directory
  ([renameio](https://github.com/google/renameio): without fsync, a rename "can result in a 0-length file").
  Windows cannot do it reliably (same source). Fall back to copy-in-place for links and special attributes
  (Vim `backupcopy=auto`).
- **mmap.** Touching a page past a truncated end raises **SIGBUS**, and whether a private mapping sees later
  changes is "unspecified" ([mmap(2)](https://man7.org/linux/man-pages/man2/mmap.2.html)). Database
  researchers list transactional safety, I/O stalls, error handling and TLB shootdowns as reasons to avoid it
  ([Crotty et al., CIDR 2022](https://db.cs.cmu.edu/mmap-cidr2022/)). In Go, a fault on mapped memory is
  fatal unless `debug.SetPanicOnFault` is set ([runtime/debug](https://pkg.go.dev/runtime/debug#SetPanicOnFault)).
  *Practice:* `ReadAt` (pread) through the page cache.
- **Saving without freezing.** Stream from a snapshot (VS Code's `createSnapshot`, Sublime's async saves).
- **External change.** Compare an mtime and size etag before writing (VS Code). Watch the parent directory,
  because editors save by rename (fsnotify [FAQ](https://github.com/fsnotify/fsnotify)). For a growing log,
  follow it, as klogg does.

**3.7 Memory budgets and messages.**
- VS Code shows a one-line notification that names what is off, with one action to override and a reopen.
  Emacs asks a question with a « literally » choice. IntelliJ goes read-only and says so.
- Figma shows a **memory meter** in percent. At 90 % a red alert appears that cannot be dismissed; at 100 % the
  file locks and **recovery mode** opens it so memory can be reduced
  ([help](https://help.figma.com/hc/en-us/articles/360040528173-Reduce-memory-usage-in-files)).
- *Practice:* one place, plain words, one reversible action, and an announced fallback before any crash.

**3.8 Measuring fluidity.**
- **End to end.** Typometer synthesizes OS key events and captures the screen, so it includes the compositor
  and V-Sync. Fatin argues for 1 to 5 ms of editor paint time ([Fatin](https://pavelfatin.com/typing-with-pleasure/)).
- **In page.** The Event Timing API measures from the event to the next paint, rounded to 8 ms, and is
  "Baseline 2025" ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/PerformanceEventTiming)). Long
  Animation Frames attribute frames over 50 ms to scripts and forced layout, in Chromium 123+ only
  ([Chrome](https://developer.chrome.com/docs/web-platform/long-animation-frames)).
- Zed reads frame times from the Metal HUD and Instruments (above).

**3.9 Collaboration on big documents.**
- Google Docs caps a document at 1.02 M characters ([help](https://support.google.com/drive/answer/37603)):
  OT cost is bounded by product policy. Figma bounds by memory; Zed accepts per-character metadata (above).
- Eg-walker keeps no CRDT state in steady state: "an order of magnitude less memory", and load "orders of
  magnitude faster" ([paper](https://arxiv.org/abs/2409.14252)).
- Lesson: every product bounds the shared size explicitly. LF's 4 Mi shared cap is about 4x Google's and is
  defensible. The Eg-walker result strengthens ADR 0028's trigger for a Loro trial.

## 4. Ranked practices for inkwell

The status column reads: **LF** = in large-files.md, **ADR** = in ADR 0028, **new** = proposed here. Every
proof is a PERF case under the frozen gate `LF1`, or under a follow-up gate when the case is new (PERF-32 and
up).

| # | Practice | Value | Status |
|---|---|---|---|
| 1 | Highlighter size cap decided at open (Typst 512 Ki), with a notice and a reversible action | removes 50 ms to 1.3 s per key now | LF |
| 2 | **Runtime highlight kill switch**: 3 parse or highlight passes over 50 ms within 10 s → colours off for that document, same notice | catches any grammar pathology that size misses (Vim `redrawtime`) | **new** |
| 3 | **Typst highlighting from `typst-syntax`** (incremental reparser), WASM in a Worker, flat `Uint32Array` tokens applied as decorations; mapped through edits until fresh (Zed's interpolation) | lifts the 512 Ki cap without waiting for the Lezer grammar | **new** (LF only plans an upstream fix) |
| 4 | Search off the main thread, caret first, streamed count, chunk timeout | no 0.3 to 1.4 s freezes | LF, **amended §5** |
| 5 | Replace-all: one ChangeSet, or a full-range edit above 50 000 matches; one undo step | 3 to 39 s → none | LF |
| 6 | Per-line highlight cap (20 000 chars); rendering left to CodeMirror's line gaps | minified files and 10 MB lines | LF (thresholds confirmed by VS Code and Emacs) |
| 7 | **Save from a streamed snapshot** (`doc.iter()`, chunks ≤ 1 MiB) with etag check (mtime + size) before write; conflict → existing B1 merge | no 2× peak; no lost external edit | **partly new** (LF chunks the Stream, but builds `toString()`) |
| 8 | **Atomic save that preserves links and attributes**: temp + fsync + rename + dir fsync; copy-in-place for symlinks, hard links, xattrs and ACLs | no 0-byte file after a crash; no broken link | **new check**: `internal/store/atomic.go` (`writeAtomic`, `SyncDir`) exists; it is unverified whether user files use it and keep links and attributes |
| 9 | **Undo byte budget**: retained deleted text ≤ 64 MiB per document; oldest events dropped, with a toast | bounded memory after big replace-alls | **new** |
| 10 | **Cap on the external-edit merge**: three-way diff of ≤ 50 MB and ≤ 2 s, in a Worker; past it, offer « Garder ma version / Recharger » | no freeze when a 50 MB file changes on disk | **new** (VS Code diff caps) |
| 11 | **Memory pressure message and recovery** | the person is warned before a crash | **new** wording on ADR D9 recycling |
| 12 | Paged view (D4) by `ReadAt`, never `mmap`; index built with `bytes.Count` and `IndexByte` on 1 MiB chunks, in parallel | no SIGBUS crash when a log is truncated | ADR (D4) + **new** constraint |
| 13 | **Follow mode** for files that grow; rebuild after truncation or rewrite | live logs | **new** |
| 14 | Encoding: BOM, then 512 B binary sniff, then a guess on ≤ 64 KiB | the same as VS Code | ADR (A3 reuse) |
| 15 | **End-to-end latency certification** with the Event Timing API | budgets that hold on the real screen | **new** (LF uses rAF gaps) |
| 16 | Shared size capped explicitly (4 Mi), told before inviting | bounded CRDT cost | LF (supported by Google Docs and Figma) |

How to adopt each practice, its cost and risk, and its proof:

| # | Adoption in our stack | Cost / risk | Proof |
|---|---|---|---|
| 1 | `limits.ts` + `languageExtension`; a status bar notice | S / low | PERF-23, PERF-28 |
| 2 | A `ViewPlugin` that times `syntaxTree` work and highlight passes; it reconfigures a `Compartment` to plain text. Webview only, no Go | S / low: false positives on a slow machine, mitigated by the action « Réactiver » | PERF-32: a grammar mutant that sleeps 80 ms per parse; the notice must appear and typing p95 return ≤ 20 ms within 2 s |
| 3 | Build `typst-syntax` to WASM (CSP `'wasm-unsafe-eval'` is already accepted, ADR D2). The Worker holds a `Source` and receives `ChangeSet` JSON; it returns token runs as a transferable buffer. No Go or Wails change. tinymist's `semanticTokens/full/delta` through the Go sidecar is the fallback | M to L / medium: WASM size, token mapping, two highlighters during the transition | PERF-28 with typst-syntax: highlight lag ≤ 1 frame on screen, 0 long tasks at 10 MB |
| 4 | See §5 for the memory side | per LF | PERF-24, plus the §5 memory assertion |
| 5 | per LF | per LF | PERF-25, PERF-26 |
| 6 | per LF; Lezer's Markdown and Typst parse is still whole-line | S / low | PERF-31 |
| 7 | `Text.iter()` → `store` Stream chunks; Go writes to a temp file and renames at the end. The etag comes from the last open or save | S / low | PERF-33: save of 50 MB, no long task, peak heap ≤ 1.1 × the document; a conflict test with an external write between edit and save |
| 8 | Go side only, under `os.Root`: stat, `Lstat` for links, copy the mode, `Lchown` when owned, xattrs on Linux; fall back to an in-place write | S / medium: platform differences; Windows non-atomic | Go table test: hard link, symlink, mode 0600, a kill between write and rename leaves the old file intact |
| 9 | Wrap `history()` with a `transactionExtender` that counts the `ChangeSet` deleted lengths; at the budget, call `isolateHistory` and trim through a reconfigure | M / medium: CodeMirror has no trim API, which may need a small upstream PR | PERF-34: 100 replace-alls on 50 MB, JS heap ≤ budget + 64 MiB, undo of the last 10 intact |
| 10 | In the B1 `ExternalSync` path: size check, Worker, deadline. Webview only | S to M / low | PERF-35: a 50 MB file rewritten externally, no long task, a choice shown ≤ 2.5 s |
| 11 | The Worker reports WASM `memory.buffer.byteLength`; the main thread reads `performance.memory` where it exists. At 75 % of the ADR budget, a banner « Inkwell manque de mémoire : fermez des fichiers ou passez ce fichier en lecture seule. » with actions; at 100 %, recycle (D9) and reopen read-only | M / medium: WebKitGTK has no reliable heap API, so the WASM figure is the reliable one | PERF-36: open documents until the budget, the banner appears before any OOM |
| 12 | `internal/textview`: `ReadAt` pages, a `sync.WaitGroup.Go` per chunk for the index, results streamed on the `view` Stream | S / low | ADR PERF-10, plus a test that truncates the file while paging: an error, no crash |
| 13 | The Go watcher (parent directory) sends « grown » or « replaced » on the `view` Stream; « grown » extends the index from the old size once the last page's bytes are rechecked | S / low | PERF-37: append 10 MB/s to a 1 GB log, tail visible ≤ 500 ms, no rebuild |
| 14 | reuse A3 | — | existing |
| 15 | Feature-detect `PerformanceObserver` `"event"` in WebKitGTK, else keep rAF gaps; one periodic manual Typometer run on the native build, logged in the gate report | S / low; Typometer is a manual tool | PERF-01 extended: in-page p95 and an end-to-end sample within 10 ms of each other |
| 16 | per LF | per LF | per LF |

## 5. Amendment to LF's search Worker (memory)

- **The finding.** VS Code does not sync models over 50 MB to workers (`_MODEL_SYNC_LIMIT`), because a mirror
  doubles the text. LF §6 keeps a full mirror in the search Worker up to 64 Mi, and LF §7's budget
  (≤ 2 B/char + 50 MB) only counts the main thread.
- **The proposal.** Up to 8 Mi, a mirror (cheap; it saves the 50 ms copy per query). Above 8 Mi, no mirror:
  each query sends the snapshot in 1 Mi chunks (transferable copies of `Text` slices), yielding between
  chunks; benchmark it against a time-sliced main-thread scan (`scanSync` per chunk with `scheduler.yield`)
  and keep whichever passes PERF-24 with the lower peak.
- **Proof.** PERF-24 measures the total heap of the main thread plus the Worker, so the mirror cannot hide.

## 6. What not to do (the evidence against)

- **No Lezer parse in a Worker** to cure Typst: its trees do not cross threads (Marijn); move flat tokens (#3).
- **No CodeMirror state off the UI thread.** Xi's split made wrap and scroll harder. ADR D2 is safe because
  CodeMirror stays authoritative on the main thread, as Zed's buffer does; keep it so in the bridge.
- **No `mmap`** for files a person or a program can truncate (§3.6).
- **Not size thresholds alone**: a log saved as `.md` (LF §4.1) needs the time-based switch (#2).

## 7. Measurement plan (no runs today)

- PERF-32 to PERF-37 above join a follow-up gate after `LF1`, frozen before code by the skill `new-gate`.
- Mutants it must catch: kill switch disabled; undo budget removed; merge cap removed; save through
  `toString()`; rename without fsync (crash test); textview through `mmap`; follow mode rebuilding on append.
- Run them at rest, after the running native gate; heavy jobs go through `scripts/heavy.sh`.

## 8. Contradictions and tensions with the current plan

1. **Contradiction.** LF §6, the search Worker mirror up to 64 Mi, against LF §7's solo memory budget. A full
   mirror roughly doubles the text, and VS Code refuses it above 50 MB. Resolved by §5.
2. **Caution.** LF §(b1) bets on fixing `codemirror-lang-typst/lezer` upstream. If fragment reuse cannot be
   fixed, the cap stays forever; #3 gives a path that does not depend on that grammar.
3. **Caution.** ADR D2 and Raph Levien's lesson: keep CodeMirror authoritative on the UI thread (§6). D2 as
   written does this.
4. **No contradiction** on line wrapping. VS Code turns wrapping off above 20 MB, but LF measured it harmless
   up to 50 MB with CodeMirror's line gaps. Keep LF's choice and let PERF-31 settle single long lines.
