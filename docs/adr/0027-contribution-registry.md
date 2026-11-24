---
status: proposed
date: 2026-11-24
amends: spec §4.4 (the `src/filetypes` target row merges into `src/extensions`), §6.6 MF-14 (the data part is no longer embedded by Go), decisions AD-51 (no table shared by Go and TypeScript; AD-08's « Go ne dépend pas du registre » is kept)
decision-makers: [Mathieu]
---

# One registry of contributions, file types first

## Context and problem

The application learns what a file is in at least nine places, each with its own list: `langOf` (`editor/format/lang.ts`),
`isTabular` (`ui/sheet/viewMode.svelte.ts`), the delimiter by extension (`editor/csv/dialect.ts`), the icon table
(`ui/icons/FileIcon.svelte`), the kinds that open as text (`workspace/workspace.svelte.ts`), search (`editor/search/project.ts`)
and the Insert menu (`ui/toolbar/InsertMenu.svelte`) on the webview side, and `extKinds` with the wire `Kind` byte in Go
(`internal/project/files.go`, `internal/bridge/files.go`). The CodeMirror languages of Typst and Markdown, and the CSV grid,
are imported statically: they weigh on every start. N20 asks for txt, csv, md and typst with a fitting view, and correct
highlighting of other languages; U26 asks that adding a type be adding a module, its editor loaded on demand.

The user's answer to W72 (2026-11-14) is « yes for the structure, no for the exposure »: the spec (§15: closed kernel K-1..K-10,
extension points, capabilities, X-2 « one registry ») never exposes the inside, only narrow points; what must be done now
is the single registry, so that later plugins (N21, WASI; N24, windows and menus without denaturing the app) plug into the
points and never into internals. This ADR fixes that registry; commands, menus, shortcuts, toolbars and panels become
points of the same registry in the next slices of block EX0. There is no plugin host here (lot « Extensions utilisables »).

An earlier draft (branch `t11-b4b-files`, 748bbcf, `docs/adr/0027-types-de-fichiers-modulaires.md`, never merged) proposed
one TypeScript descriptor per type, registered by calling `register({…, open: () => import(…)})`, and a separate Go table.

## Decision drivers

- One source: the app learns of a type, viewer or grammar only from the registry (X-2); no list elsewhere (P-EXT-1).
- Data before code: the registry is built from manifests without running extension code (X-6), so a built-in and a future
  plugin are read the same way, and the plugin host can validate a manifest without executing it (K-9).
- Narrow exposure: a contribution receives what its point's contract gives, never `Repo`, `DocHandle`, a Stream or the
  DOM, except the built-in `ui.native` exception of X-3 (the typing path).
- Invariants: the CRDT lives in the webview and Go carries opaque bytes (ADR 0002, 0011); no download at run time (MF-18,
  ADR 0024); the CSP stays `script-src 'self'` (no `eval`, no `new Function`).
- Speed: opening the main document must not get slower (PERF-START, ADR 0038); a lazily loaded viewer opens a 1 MiB file
  within 200 ms (spec §15.4, « Vue, éditeur d'un type »); the registry builds in ≤ 10 ms (§15.4, « Type de fichier »).
- Nothing becomes unreadable without its extension (ND-7): text always opens as text, anything else as the file card.

## Prior art (read 2026-11-24)

- **VS Code** splits declarative `contributes` (languages, grammars, commands, menus, keybindings, custom editors) in
  `package.json` from code activated by events (`onLanguage`, `onCommand`, `onCustomEditor`); since 1.74 a contributed
  language, command, view or custom editor activates implicitly, and `*` is discouraged
  ([activation events](https://code.visualstudio.com/api/references/activation-events),
  [contribution points](https://code.visualstudio.com/api/references/contribution-points)). Extensions run in a separate
  host and « have no access to the DOM of VS Code UI »
  ([capabilities](https://code.visualstudio.com/api/extension-capabilities/overview)). Lesson taken: manifests as data,
  activation by event, no DOM. Limit: the extension host is one Node process with full API access per window.
- **Zed**: a language is a `config.toml` (name, grammar, `path_suffixes`, `first_line_pattern`) plus Tree-sitter queries,
  all data; the code is Rust compiled to a WebAssembly component through WIT, with no UI panels, no free HTTP and no free
  file system ([languages](https://zed.dev/docs/extensions/languages),
  [Zed decoded: extensions](https://zed.dev/blog/zed-decoded-extensions)). Lesson: highlighting is data plus a grammar;
  code behind a typed WIT boundary (our §15.20). Limit: a regular expression from the manifest (backtracking risk).
- **JetBrains**: extension points are declared in `plugin.xml`, either *bean* (data) or *interface* (code), `dynamic="true"`
  points must re-enumerate on each use or listen to changes
  ([extension points](https://plugins.jetbrains.com/docs/intellij/plugin-extension-points.html)). Lesson: separate data
  points from code points; a dynamic registry is a snapshot replaced as a whole. Limit: plugins share the IDE's classpath,
  so internals are reachable; isolation is by convention (`@ApiStatus.Internal`).
- **Obsidian**: `registerExtensions(["csv"], viewType)` plus `registerView(type, factory)` in `onload`
  ([registerExtensions](https://docs.obsidian.md/Reference/TypeScript+API/Plugin/registerExtensions),
  [views](https://docs.obsidian.md/Plugins/User+interface/Views)). Counter-example: contributions exist only after the
  plugin's code ran, and taking over `.md` goes through the undocumented `app.viewRegistry` internals.
- **Helix**: `languages.toml` maps `file-types` (globs, then extensions) and `shebangs` to a grammar, fetched and built
  ahead of time ([languages](https://docs.helix-editor.com/languages.html)). Lesson: shebangs as words, not regexes.
- **Lapce**: a « volt » is a WASI module with a `volt.toml` (activation by language or workspace file), behind the proxy
  process ([InfoQ, 2024-03](https://www.infoq.com/news/2024/03/lapce-rust-editor),
  [lapce-rpc VoltActivation](https://docs.rs/lapce-rpc/latest/lapce_rpc/plugin/index.html)).
- **CodeMirror 6**: `LanguageDescription.of({name, alias, extensions, filename, load})` loads a language by `import()`
  on first match; `@codemirror/language-data` 6.5.2 lists about a hundred of them; `@codemirror/lang-markdown` takes
  `codeLanguages` to highlight fenced blocks (`@codemirror/language` 6.13.1, `dist/index.d.ts`;
  [language-data](https://github.com/codemirror/language-data)).

Critique: every system above either runs code to learn contributions (Obsidian), or keeps data and code apart but lets
code reach internals (JetBrains, VS Code inside its host). None ties a contribution's power to who loaded it rather than
to what its manifest claims. We keep VS Code's and Zed's « manifest is data », Helix's word-only matching and
CodeMirror's lazy `LanguageDescription`, and add three things: the loader, not the manifest, stamps the origin; code is
named by an entry path, never by a closure, so a manifest is plain JSON; and Go holds no copy of the table.

## Considered options

- **A. Per-consumer tables** (today). Rejected: nine lists, no lazy loading, every new type edits five files.
- **B. Imperative descriptors** (the 748bbcf draft, Obsidian's shape): `register({extensions, open: () => import()})` per
  module. Rejected as the model: closures make a descriptor code, so it cannot be a plugin manifest, cannot be checked
  without running it, and capability-like fields (`binary`, `preview`) mix with code. Its good parts are kept (below).
- **C. One JSON table embedded by Go and imported by TypeScript** (AD-51, MF-14). Rejected: Go would have to merge plugin
  manifests to classify names it never acts on; the wire `Kind` byte makes Go sniff every unknown file at listing; two
  readers of one table drift. Go's real needs are content facts (text or not, encoding, line endings) and its own closed
  media allow-list, both already in Go.
- **D. `@codemirror/language-data` as the language registry.** Rejected as a registry (a second list, about a hundred
  languages beyond AD-54's closed list, no icons or viewers); kept as the pattern, and as the source of the grammar
  packages it names.
- **E. Typed TypeScript manifests** (`defineExtension({…})`). Rejected: a TS object can hold functions and imports; JSON
  forces data and is the format plugins will ship (`inkwell-plugin.json`, spec §15.15 DX-6).
- **F. Manifests as data, code by entry path, one registry in the webview** (chosen).

## Decision

Option F.

1. **Where.** The registry is `frontend/src/extensions/` (closed kernel, K-9): manifest validation, the immutable
   registry snapshot, resolution, and the one loader of entries. It is the only module that knows contributions exist.
   Built-in extensions live in `frontend/src/ext/<name>/`: an `inkwell-plugin.json` manifest, and, only for an extension
   whose contributions declare `ui.native`, a `native/` folder with its code. The spec's `src/filetypes` target is not
   created: file types are a point of this registry.
2. **Manifest.** JSON, the subset of DX-6's `inkwell-plugin.json` that this slice needs: `id`, `version`, `essential`,
   `activation`, and `contributes` keyed by point (`fileTypes`, `viewers`, `grammars` now; `commands`, `menus`,
   `keybindings`, `toolbar`, `panels`, `status`, `settings`, `icons` in the next slices, same shape as VS Code's
   `contributes`). Validation is strict: unknown keys refused, every string and list bounded, ids `[a-z0-9.-]`, no
   regular expression anywhere (matching is by words). The keys `__proto__`, `constructor` and `prototype` are refused at
   every depth, even where a free-form object is allowed (`options`), and the validated manifest is rebuilt from
   null-prototype objects (`Object.create(null)`) and frozen, so no manifest can reach `Object.prototype` (prototype
   pollution, [OWASP](https://cheatsheetseries.owasp.org/cheatsheets/Prototype_Pollution_Prevention_Cheat_Sheet.html)).
   The validator is hand-written TypeScript in `src/extensions` (decided 2026-11-24): ajv compiles with `new Function`,
   which the CSP forbids ([ajv security](https://ajv.js.org/security.html)); generating the validator from a JSON
   Schema ahead of time stays possible later.
3. **Origin and entries.** The loader stamps each manifest `builtin` (found by `import.meta.glob` under `src/ext/*/`),
   and later `plugin` or `project`. A manifest cannot claim an origin. `ui.native` is accepted only for an extension of
   origin `builtin` that is also `essential` (decided 2026-11-24). Every entry is a relative POSIX path inside the
   extension's own folder: normalised, no `..` segment, no leading `/`, no backslash, no drive letter, no URL scheme,
   no empty segment. A `builtin` entry must also be a key of the build-time glob of that same folder, so it can only
   name a file of the extension. For `plugin` and `project` origins, the plugin host (Go, K-9, lot EXT1) resolves the
   entry inside the unpacked package after resolving symbolic links and refuses any path that leaves it (the same rule
   as `os.Root`). The webview never turns a plugin entry into an `import()`.
4. **Build without code.** The registry is built at start from the manifests alone: read, parse, validate, index,
   freeze. Built-in manifests are read as text (`import.meta.glob(…, { query: "?raw", import: "default", eager: true })`)
   and go through the same `JSON.parse` and validator a plugin manifest will, never through Vite's JSON-to-JS module. No
   entry is imported, which a counter proves. A conflict (two file types claiming one extension or whole name, two
   contributions with one id) refuses the later manifest with both names, in a deterministic order; there is no
   numeric priority (X-5).
5. **The `fileTypes` point** (data, verb « contribute »): `id`, `label` (an i18n key for built-ins), `icon` (a name in
   the icon set; the `icons` point later replaces the set, TH-3), `extensions` (with the dot, lower case, several dots
   allowed: `.tar.gz`), `filenames` (whole names, case-insensitive: `Makefile`, `.gitignore`), `shebangs` (interpreter
   words, as Helix), `mime` (IANA, RFC 4180, RFC 7763), `storage` (`text` or `blob`), `grammar` (a grammar id), and
   `options` (a small JSON object handed to the viewer, for example a table's delimiter: `.csv`, `.tsv` and `.psv`
   become three types that the grid views with their own delimiter, and the extension switch of `dialect.ts` goes).
6. **Resolution and content class.** The type comes from the name alone: the person's choice for this file; else the
   whole name; else the longest matching suffix; else a shebang on the first line; else no type (unknown name). The
   *content class* (text or binary) is computed once per open, by Go, when the file is read as text (`Decode`: a NUL
   or control byte means binary, MF-15), never at listing (R-9) and never in the webview. It is computed only for a
   type of storage `text` and for an unknown name. A `blob` type is read as bytes whatever its content, and its viewer
   decides whether it can show them (a « `.png` » that is not a PNG fails to decode and falls back to the file card,
   rule 7). Outcomes: text type and text content → the type's viewer; text type but binary content → the file card,
   with a notice that the file is not text (the real class wins, ND-7); unknown name and text content → the essential
   `text` type; unknown name and binary content → the essential `binary` type. So a document whose bytes are not text
   never reaches the text editor, and nothing is shown as text without Go having decoded it. **SVG** is an image of
   storage `blob`, as every image (ADR 0023, Typst reads it from the projection), although its bytes are XML text:
   storage is declared by the type, not inferred from content. Reading an SVG as text is an « open with » choice for a
   later slice, not a mismatch.
7. **The `viewers` point** (code, verb « replace » per type): `id`, `fileTypes`, `label`, `entry`, `capabilities`. The
   default viewer of a type is the one its own extension contributes. If that extension contributes none (BibTeX, PDF
   today), the default is the essential viewer of the type's storage: the text editor for `text`, the file card for
   `blob`. Alternatives come from one rule with no code run: a viewer of another extension that lists the type in its
   `fileTypes`, and, for every type of storage `text` whose default is not the text editor, the text editor itself
   (anything that is text can be read as text, ND-7). The existing view bar shows them as a toggle, so a CSV offers
   its grid and its text (W79), as today. The « looks tabular » hint that offers a table for a text stays, through the
   registry and off the start path (code review, finding 2): it is offered for a file of the plain `text` type
   other than the main document, once Go has read it as text (`offersTable`, `workspace/viewChoice.svelte.ts`);
   choosing it is the file's type choice. A general « suggest » point may later absorb it. A viewer's code is loaded on the first open of one of its types,
   once, by one shared promise; a failed load falls back to the essential viewer of the content class with a notice
   (ND-7).
8. **The `grammars` point** (code, verb « contribute »): `id`, `names` (fenced-block info strings and aliases), `entry`,
   `capabilities` (`ui.native`: a grammar of this slice is Lezer code in the webview, so only an essential built-in has one).
   In this slice the built-in grammars are the two already in the tree, Typst (`codemirror-lang-typst`, patched) and
   Markdown (`@codemirror/lang-markdown`), moved behind the point. The other languages of AD-54's closed list (JSON,
   YAML, TOML, XML and the rest, new pinned packages, a new supply chain) are block EX0d: there they are data plus an
   entry each, split by Vite and embedded (MF-18). Markdown's `codeLanguages` is derived from this point, so fenced
   blocks and code files highlight through the same registry. Plugin grammars will come through the `tokens` point
   (ranges computed off the display thread, §15.4), never as Lezer code in the webview.
9. **Activation.** `onFileType:<id>` and `onLanguage:<grammar>` load entries; `startup` is allowed only for an essential
   built-in and justified: the text editor and the Typst grammar, since the main document opens at start. Everything
   else leaves the start chunk: Markdown (`@codemirror/lang-markdown`, `@lezer/markdown`), the grid (`src/ui/sheet/`)
   and the image viewer. The gate checks this on the sources of the start chunks, not on chunk names.
10. **Go.** Go keeps no table of names. The store stream's file list drops its `kind` byte, so listing reads no file
    content. The version byte becomes one constant per payload: the file list goes to version 2 (`fileListVersion`),
    and the file info and the four tree payloads keep version 1 under their own constants, since they do not change
    (`internal/bridge/files.go`, `tree.go`; webview `store/files.ts`, `store/tree.ts`; vectors
    `internal/bridge/testdata/storestream.json`). Go keeps what needs the bytes or is security: decoding text when a
    file is read (rule 6), and the closed allow-list of media types it serves as images (`imageTypes`,
    `internal/app/image.go`), part of the closed kernel (K-6, K-8) and never a contribution. `ReadImage` loses its
    own name check, which moves before the read in its only caller, the image handler, which already holds that list.
    When the plugin host comes, Go validates manifests at installation (K-9) with the same rules, but still does not
    act on file types.

### Never exposed

A contribution never receives the `Repo`, a `DocHandle`, a Stream, the bridge, the registry itself, other extensions'
contributions or a way to remove them, nor any K-1..K-10 element. Consumers read a frozen snapshot. A non-native viewer's
context is a facade (name, path, size, read-only reason, ranges read on demand, proposals: `document.read`,
`document.propose`); the TypeScript type is written now so that built-ins cannot widen it unnoticed, and only the
native context of a built-in adds the CodeMirror and Automerge handles (X-3). The FE-3 rule gains one narrow carve-out:
`src/ext/<name>/native/**` may import core modules, only for a built-in `essential` extension whose manifest declares
`ui.native`. In this slice, every built-in that brings code is therefore essential (`text`, `files`, `typst`,
`markdown`, `tables`, `images`): it can be replaced but not disabled (X-7). `bibtex`, `pdf` and `data` (JSON, YAML,
TOML, XML as text types without a grammar until EX0d) are data only and can be disabled. Residual → EXT1: once the capabilities and the `ui.view` path exist, every built-in that does not need
the typing path (images, the file card) moves out of `native/` and stops being essential.

### The file list Go sends is local

The `kind` byte that version 2 drops is never persisted and never leaves the device. Go builds the list on demand,
`StoreServer.listFiles` → `Project.List` → `EncodeFileList` (`internal/bridge/storestream.go`, `listFiles`), and answers
`OpListFiles` on the in-process `store` Stream (ADR 0009). In the webview, `GoStorageAdapter.listFiles` decodes it
(`frontend/src/store/GoStorageAdapter.ts`), and `Workspace.refreshFiles` keeps it in the in-memory `workspace.files`
(`frontend/src/workspace/workspace.svelte.ts`), read only by the sidebar, search, the Insert menu and the problems list.
No module of `src/net` reads it, nothing writes it to Automerge, to IndexedDB or to Go's store (`internal/store` has no
`Kind`), and no peer message carries it. So no data needs migrating; the version byte makes a webview and a Go side of
different builds fail cleanly, and both are built together.

## Consequences

- Good: one place for every type, icon, viewer and grammar; a new type is one folder (manifest plus, if it brings code,
  its entry); the main document's start no longer loads Markdown, the grid or the image viewer; built-ins are read
  exactly as plugins will be; Go stops interpreting names and listing stops reading files.
- Bad: one more indirection on opening a file (resolve, then a dynamic import); the store stream's file list changes
  version (webview and Go of different ages fail cleanly, as designed); an unknown-extension text file shows the generic
  icon in the list until it is opened (today Go sniffs it to choose « text »).
- Neutral: the per-file choice stays per session until the project index (ADR 0023) carries it as a node attribute.

### Decided on review (2026-11-24, integrator, after the cross-family review in `gates/EX0/SPEC-REVIEW.md`)

- Validator: hand-written TypeScript, no ajv (CSP); possibly generated from a schema later.
- The Go wire change stays in EX0: dropping `kind` follows directly from the registry.
- `native/` only for built-in `essential` extensions; residual → EXT1 (above).
- The 200 ms viewer budget stays in the end-to-end run, under `heavy.sh --exclusive`.
- Round 2 (blind Opus review, `gates/EX0/REVIEW-2.md`): rules 4, 6, 7, 8, 9 and 10 above were rewritten at their root
  (raw manifests through one parser, content class and SVG, alternative viewers without code, grammars cut to Typst and
  Markdown with the rest in EX0d, start sources checked by sourcemap, one version byte per payload).

### Open (product, with the user)

- An unknown-extension text file shows the generic icon until it is opened: acceptable? *Default applied in
  slice 1: the generic icon.*
- Should a per-file « open with » choice be remembered on this device only, or shared with the project?
  *Default applied in slice 1, the safer one: this device only, per project (`workspace/viewChoice.svelte.ts`,
  local storage); nothing is shared with the other people. The view chosen for a file (its text rather than its
  table) is remembered the same way (UX-EX0-5, key `inkwell.viewWith`).*

### Confirmation

Block EX0, plan `docs/plans/2026-11-24-ex0-contribution-registry.md`, gate `gates/EX0.toml`: the registry rules R-1 to
R-8 and R-12 to R-15 as named Vitest tests, each guarded by a mutant that this test by its exact title must catch
(block EX0 in `scripts/netsim/mutants.toml`, field `name`). R-9 is checked by named Go tests on the file list, R-10 on Vite's build manifest (lazy
entries outside the start closure), R-11 by PERF-START (paired, ADR 0038) and the viewer budget in the end-to-end run, and a static check proves that no
type id or extension is read outside the manifests, except at the sites listed in the gate (tagged EX0c or K-7).

## More information

What is reused from 748bbcf: the resolution order, « one module per type », refusing a taken extension, the fake-type
test, the mount contract of an editor module (`mount(host, ctx) → {destroy, focus, selection, setType}`). What is not:
closures in descriptors, the Go table, `binary` and `preview` as descriptor flags, `import.meta.glob` towards a module
that may not exist (an entry that does not resolve is now a build error). Related: ADR 0023 (node attribute), ADR 0026
(grid), ADR 0028 (size tiers), the draft ADR 0039 (plugins, spec annex).
