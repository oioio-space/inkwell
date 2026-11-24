# EX0 code review (blind Opus review, 2026-11-24)

Verdict: « accept with corrections », 6 major findings. Each finding was fixed at its root, and the test named
under it was run red first: on the code before the fix, or with the fix reverted for that one run.

## Findings and answers

1. **(major, live)** Every file of unknown extension was read through `/project-text`, and `ReadProjectText` read
   up to `store.MaxValue` (32 MiB) before decoding anything. A 50 MB `.zip` failed as too large, which became a 404
   and the card « file.unavailable », where `main` showed the info card. A 30 MB binary file was read in full just
   to learn that it is binary.
   **Answer.** Go now decodes a 4 KiB prefix first (`readText`, `internal/project/project.go`), with a rune that
   the prefix cuts trimmed. Bytes that are not text get `ErrBinary`, answered 415, whatever the size. A text over
   the limit gets the new `project.ErrTooLarge`, answered 413, and its bytes are not read past the prefix. The
   webview reads 413 as `large` and shows the info card.
   **Tests.** `TestProjectTextOfBigFiles` (`internal/app`, sparse 33 MiB files: red, 404 instead of 415 and 413);
   `TestEX0SniffKeepsACutRune` (`internal/project`, red without the trim: « control bytes »); Vitest « what a file
   is, once read » (`workspace.test.ts`).
2. **(major, live)** The « Ouvrir comme tableau ? » offer for a `.txt` that looks tabular was gone.
   **Answer.** Kept in this slice, through the registry. The reason: the fix is small and a shipped feature must
   not vanish. `offersTable` (`workspace/viewChoice.svelte.ts`) offers it for a file of the plain `text` type,
   once Go has read it as text, when it is not the main document and was not opened by choice. The check never
   runs for `main.typ`, so never at start. The button lives in the existing view bar (`data-sheet="open-table"`)
   and makes the file's type choice. `readsAsTable` (in `editor/csv/dialect.ts`) is the former `looksTabular`,
   under a name that the static check allows. The `sheet.openAsTable` messages stay; no block needed filing. ADR
   0027 rule 7 and the plan's scope line are amended.
   **Tests.** Vitest « the « open as a table » offer » (`viewChoice.test.ts`, red: no function); EX0-E1 opens
   `ex0-stock.txt`, takes the offer and sees the grid.
3. **(major, latent)** A plugin claiming the whole name `main.typ` or the extension `.x.typ` took those files from
   Typst, because claims were compared as separate strings and whole names resolve before suffixes.
   **Answer.** `buildRegistry` parses every manifest first and collects the extensions of essential types. A
   whole name or multi-dot suffix of another extension that ends in one of them is refused as a conflict, naming
   both extensions, whatever the order. The R-4 fixture that claimed `requirements.txt` (the essential `.txt`) now
   claims `package.json` (data's `.json`, not essential).
   **Test.** R-3, both cases in both orders (red: nothing refused).
4. **(major, latent)** `ViewerHost` passed the native props (`handle`, `big`, `readImage`) to any viewer.
   **Answer.** `viewerProps` (`extensions/context.ts`) decides, at this one place, from the viewer's
   capabilities. `ui.native` gets the native props; any other viewer gets only a frozen `ViewerContext` (name,
   path, size, read-only reason, `read`, `propose`). `ViewerHost` mounts through it.
   **Test.** Vitest « what a viewer receives » (`context.test.ts`, red: no function).
5. **(major, latent)** `refuseProtoKeys` recursed without a bound, and `buildRegistry` rethrew any error that was
   not a `ManifestError`, at module load, so the app would not start.
   **Answer.** The key walk stops at depth 32 with the reason `too-deep`. Any failure to read or parse a manifest
   refuses that manifest alone, with the reason `invalid`.
   **Tests.** R-12, a 64,002-byte nest and a nest of 40 (red: RangeError); R-1, a source whose text throws
   `RangeError` (red: the build threw).
6. **(major, latent)** `entry()` accepted `%2e%2e/x.js`, `?x` and `#`.
   **Answer.** Each segment must match `[A-Za-z0-9._-]+` and must not be `.` or `..`.
   **Test.** R-13, with `%2e%2e/x.ts`, `native/x.ts?x`, `native/x.ts#y` and `native/a b.ts` (red). Mutant MX6's
   anchor follows the new line.
7. **(minor)** The load cache key lacked the origin.
   **Answer.** The key is now `${origin}:${folder}/${entry}`.
   **Test.** R-6, a project extension and a built-in of the same folder load twice (red with the old key: 1
   load).
8. **(minor)** A failed viewer load was cached forever, and the notice repeated on every open.
   **Answer.** `loadViewer` remembers the stand-in for each viewer: one notice, no new load, as the comment says.
   **Test.** R-6, three opens of a failing grid: one load, one notice (red: three notices).
9. **(minor)** `typeOf` was called without the person's choice.
   **Answer.** The new `workspace.fileTypeOf(path)` applies the choice, then the name, then what reading the file
   showed. `App.svelte` (the LSP opening), `DiffView` and `FileIcon` read it.
   **Test.** Vitest « the type the person chose for a file is the one everything reads ».
10. **(minor)** Keep the handler-level test that only images go through `ReadImage`.
    **Answer.** Confirmed: `TestProjectImagesRefuse` has the case « text as image » (`/project-image`,
    `notes.txt` → 404). The handler checks `imageTypes` before any read.
11. **(minor)** `README`, `LICENSE` and `Makefile` lost the text icon, which Go's sniffing gave them on `main`.
    **Answer.** The workspace remembers the type that Go's reading gave to a name no type claims, so the text icon
    shows once the file is known. The generic icon stays only before any read: the case of the user's open
    question.
    **Test.** Vitest « a text too big shows the info card; an unknown name read as text gets the text type »:
    `LICENSE` has no type before it is opened and the type `text` after.
12. **(simplification)** **Answer.** `workspace.readFile` is now derived from `#readText`. `TestEX0FileListV2` keeps
    only the version-byte assertion; the vector bytes stay checked by `storestream_test.go`.
13. **(minor)** `openHook` was not safe for parallel tests.
    **Answer.** It is a `sync/atomic.Pointer[func(string)]`.
