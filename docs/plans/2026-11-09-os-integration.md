# OS desktop integration: open a file or folder, « Ouvrir avec », second launch

Study, 2026-11-09. Scope: how Inkwell meets the file managers (GNOME Files/Nautilus, KDE Dolphin,
XFCE Thunar, Windows Explorer, macOS Finder); what exists today; the semantics to adopt; what to
ship per OS; the executable proofs; order and cost. Nothing here is implemented yet.

## 1. What exists today (evidence)

**Command line and startup.**
- The only argument read is the version: `cmd/inkwell/cli.go:8-10` (`--version`, `-version`,
  `version`, alone). Any other argument (a file, a folder) is **ignored**.
- The project is always the most recently changed one in `Documents/Inkwell`, or a new untitled
  one: `cmd/inkwell/main.go:54` → `openProject` (`main.go:183-190`) → `app.OpenProject`
  (`internal/app/docs.go:86-104`) → `project.Latest` (`internal/project/project.go:241`), which
  only looks at direct sub-folders holding `.inkwell/project.json`. A project can only live in
  `$INKWELL_DOCS` or `Documents/Inkwell` (`docs.go:22-40`).
- Order of startup: identity directory (`main.go:49`), project (`:54`), network `app.Start`
  (`:66`), backup (`:83`), then only `application.New` (`:97`) and `wails.Run` (`:164`).
- One project per process, fixed for the process lifetime: `app.NewProjects(proj, …)` is bound
  once (`main.go:102`, `internal/app/projects.go:35`); no project switch exists. The spec's
  Projects screen with « Ouvrir un dossier… » (`docs/design/spec.md:3204-3212`, §5.2) is not built
  (no Projects UI in `frontend/src`; the webview only calls `Projects.Current`/`SetRootURL`,
  `frontend/src/workspace/workspace.svelte.ts:379,417`). Several open projects is after v1
  (`docs/architecture/coherence.md` §5, « Après v1 », `AD-01` option 2).

**Single instance: none.**
- No `SingleInstance` option in `application.Options` (`main.go:97-124`); Wails' GTK4 backend
  creates the `GtkApplication` with `G_APPLICATION_NON_UNIQUE` (Wails v3.0.0
  `pkg/application/linux_cgo.h:22`, `linux_cgo.go:151`), so GTK does not dedupe either.
- No lock on the identity directory or the project: the only `flock` in the tree is the preview
  snapshot lease (`internal/app/snapshot_session_unix.go:48`). **A second launch today starts a
  second full process on the same identity key and the same project store**, both on the network
  (`app.DefaultPort` is only "offered first", `internal/app/node.go:20-22`). This is the case
  `docs/architecture/reviews/coherence-codex.md:140` excludes (« pas plusieurs processus partageant
  librement la même clé »). The e2e suite does not see it: each instance gets its own
  `INKWELL_HOME`/`INKWELL_DOCS` (`frontend/e2e/instance.ts`).

**Linux packaging** (the only released OS: `.goreleaser.yaml:27`, `targets: [linux_amd64]`, one
architecture per runner).
- Packaged desktop entry `build/linux/io.github.oioio_space.Inkwell.desktop`: `MimeType=text/x-typst;
  text/markdown;text/csv;` (`:18`) but **`Exec=inkwell` with no field code** (`:12`). The file
  manager therefore lists Inkwell in « Ouvrir avec » and then launches it **without the file**;
  `desktop-file-validate` accepts this silently (only a Categories hint, checked 2026-11-09).
  The git-ignored `build/linux/inkwell.desktop` (`.gitignore:40`, Wails-generated) has `%F` but
  is not packaged.
- MIME definition `build/linux/mime/io.github.oioio_space.Inkwell.xml:5-10`: `text/x-typst`,
  glob `*.typ`, sub-class of `text/plain`; installed to `/usr/share/mime/packages`
  (`.goreleaser.yaml:99-100`); caches refreshed by `build/linux/nfpm/scripts/postinstall.sh`
  (`update-desktop-database`, `update-mime-database`).
- AppStream `build/linux/io.github.oioio_space.Inkwell.metainfo.xml.in`: `launchable` (`:25`),
  `provides/mediatype` for the three types (`:26-31`).
- `mise run pkg:lint` (`scripts/pkg/lint.sh`) already runs `desktop-file-validate` (`:27`), checks
  required keys including `MimeType` (`:28-31`), `appstreamcli validate --no-net --pedantic`
  (`:54`), `update-mime-database` and the `*.typ` glob (`:69-73`). It does **not** check `Exec`.
- `scripts/pkgtest/inside.sh:88-90` (`mime_registered`) checks `*.typ` in `globs` and our desktop
  id in `mimeinfo.cache`; nobody launches the app with a file.

**Windows and macOS** (not released yet; native builds are lot N35 « Six plates-formes »).
- `build/config.yml:58-69`: `fileAssociations` all commented out, so the NSIS macros
  `wails.associateFiles` / `unassociateFiles` are empty (`build/windows/nsis/wails_tools.nsh:229-237`)
  although `project.nsi:99,115` calls them. No ProgID, no verb.
- `build/darwin/Info.plist:1-29`: no `CFBundleDocumentTypes`, no UTI declaration; template
  leftovers (`CFBundleGetInfoString` "This is a comment" `:15`, "© 2026, My Company" `:27`,
  version 0.1.0 hard-coded `:13,17`).

**Wails v3.0.0** (`go.mod:17`; read from the module source, the v3.wails.io pages answer 403).
- `Options.FileAssociations []string` (`application_options.go:106-109`) only feeds the
  `common:ApplicationOpenedWithFile` event. On Linux and Windows it fires **only when there is
  exactly one argument whose extension is listed** (`application_linux.go:66-85`,
  `application_windows.go:165-188`): no folders, no several files. On macOS it comes from the
  `application:openFile:` delegate (`application_darwin_delegate.m:14`, `application_darwin.go:813-825`),
  also when the app already runs (Apple Events; no second process).
- `Options.SingleInstance` (`single_instance.go:28-46`): `UniqueID`, `OnSecondInstanceLaunch(
  SecondInstanceData{Args, WorkingDir, AdditionalData})`, optional AES-GCM `EncryptionKey`.
  Linux: D-Bus session name `<UniqueID>.SingleInstance` (`single_instance_linux.go:64-125`);
  Windows: named mutex + hidden message window (`single_instance_windows.go:40-80`). The lock is
  taken **inside `application.New`**, which calls `os.Exit` in the second process
  (`application.go:203-218`). `App.RegisterService` exists (`application.go:553`), so services
  can be bound after `New`.

## 2. Research findings

- **Desktop Entry**: `%F` = list of local files, `%U` = list of URLs; an app with `MimeType` must
  accept files through a field code or D-Bus; `DBusActivatable=true` requires implementing
  `org.freedesktop.Application` (`Open`, `Activate`) [1]. GLib does that only with
  `G_APPLICATION_HANDLES_OPEN` [5]; Wails creates a non-unique application, so D-Bus activation
  is not reachable without patching Wails.
- **shared-mime-info**: `text/x-typst` (glob `*.typ`) is upstream since 2.3 (2023-10) [3]; Fedora
  44 ships it (2.4, checked locally). A merge request proposes `text/vnd.typst` at the Typst
  owners' request [4] (status unconfirmed: page behind a bot wall). `text/markdown` and `text/csv`
  are upstream. Our own XML stays useful for older systems; an `<alias>` covers a rename.
- **AppStream**: `provides/mediatype` is what software centres use to suggest the app for a type
  [2]; already present.
- **GNOME Files**: the extension API (nautilus-python, API 4.0 since Nautilus 43) lost all GTK
  access and its Python binding has had long maintenance gaps [6][7]; GNOME's own apps add
  nothing to the Files menu and rely on « Ouvrir avec ». Scripts under
  `~/.local/share/nautilus/scripts` are per-user and cannot be installed by a package.
- **Dolphin**: service menus are declarative `.desktop` files in `/usr/share/kio/servicemenus`
  (Plasma 6; KF6 requires the executable bit for user files) [8]. **Thunar**: custom actions live
  in the per-user `~/.config/Thunar/uca.xml`, not installable by a package [9].
- **Windows**: register a ProgID under `HKCU\Software\Classes` (per user, no admin), list it in
  `.typ\OpenWithProgids` and `Applications\inkwell.exe\SupportedTypes`, then
  `SHChangeNotify(SHCNE_ASSOCCHANGED)`; an app may not make itself the default (the user chooses)
  [10][11]. A classic `shell\<verb>` appears in Windows 11 only under « Afficher plus d'options »;
  the top-level menu needs an `IExplorerCommand` COM server plus a signed sparse package for
  identity [12][13].
- **macOS**: `CFBundleDocumentTypes` with `LSItemContentTypes` and `LSHandlerRank` (`Alternate`
  for types the app does not own); a type the app does not own goes in
  `UTImportedTypeDeclarations`, not `UTExportedTypeDeclarations` [14][15]. `open -a Inkwell f.typ`
  and Finder « Ouvrir avec » deliver `application:openFile:` to the running app.
- **Comparable apps**: VS Code opens a file alone or a folder as a workspace, reuses the running
  instance, and opens unknown folders in Restricted Mode (no tasks, no workspace extensions)
  until trusted [16]. Obsidian refused loose files for years and added « open file from outside
  the vault » in 1.14.2 (2026-09-15), in the current window [17]. GNOME Text Editor and Zed open a
  single file without any project. typst.app is web-only; no Typst desktop client to copy.

## 3. Proposed semantics (decision table)

Definitions. A **project** is a folder holding `.inkwell/project.json` (what `project.Latest`
already tests). A file is **in a project** when the nearest ancestor folder that is a project
contains it (walk up from the file, stop at `$HOME`, a filesystem root or a mount point). An
**intent** is the result of a pure function `Resolve(args, cwd, fs) → []Intent`; it never
carries a network action, by construction of its type.

| # | Situation | Behaviour | Why |
|---|---|---|---|
| D1 | No argument, nothing running | today's behaviour: last project, or the sample project at first launch | spec §5.1, ux rule 8 |
| D2 | File in a project, nothing running | open that project, open the file in a tab, focus it | VS Code, Obsidian: the file's context wins |
| D3 | File in the **current** project of the running instance | raise the window, focus its tab (open the tab if closed); no reload | « already open » is a focus, never a second copy |
| D4 | File in **another** project, app running | switch the window to that project, file focused; toast « Projet « A » fermé · Revenir » | one project per process (AD-01); « Fermer un projet : rien à confirmer » (§5.2); ux rule 6 |
| D5 | Loose file (in no project) | **single-file mode**: edit the file in place, preview on; header shows the file name and « Pas dans un projet »; menu action « Faire un projet de ce dossier… » | VS Code, Zed, GNOME Text Editor, Obsidian 1.14; no hidden write beside a downloaded file |
| D6 | Folder that is a project | open it as in D2 (D4 when running) | — |
| D7 | Folder that is not a project | open it as a project **after one explicit gesture** in a sheet: « Ouvrir « X » comme projet ? Inkwell y ajoute un dossier caché .inkwell. · Ouvrir · Annuler »; after, toast « Annuler » removes `.inkwell/` | writes into a user folder: the gesture is the consent; undo covers the rest (rule 6) |
| D8 | Folder refused: `$HOME`, `/`, a system folder, removable root | message « Choisissez un dossier plus précis » + « Choisir… » | prevents indexing a whole disk (ADR 0028 huge trees) |
| D9 | Several paths | first project-capable path decides the window; the other files open as tabs if in the same project, else are listed in a toast « 2 fichiers d'un autre projet non ouverts » | one window, one project |
| D10 | Second launch, any of the above | forwarded to the running instance (args + working directory), which applies D2-D9; the second process exits 0 **before** opening the identity, a project or the network | one identity key per machine session |
| D11 | Path missing, unreadable, special file, > size bound | toast with the reason and « Choisir un fichier… »; nothing else changes | ux rule 5 |
| D12 | `inkwell --version` | unchanged (`cli.go:8`) | — |

**Security rules, whatever the row.**
- Arguments, including forwarded ones, are untrusted input: absolute after `filepath.Join(cwd,…)`
  and `Clean`, `Lstat`ed, regular file or directory only, at most 32 paths and 4 KiB per path;
  every later access goes through `os.Root` (engineering principles §4).
- Opening never shares, never invites, never asks a network question; sharing starts only from
  « Partager » (ux rule 8). A loose file has no Automerge sync at all (no peer can reach it);
  « Partager » on it first proposes « Faire un projet de ce dossier » (D7).
- Typst always runs in its sandbox, rooted at the project, or for a loose file at the file's
  folder (siblings, images) — except when that folder is refused by D8, where the root is a
  private copy holding the file alone (ADR 0024, ADR 0033).
- A folder opened through D7 is **untrusted**: project-provided extensions and anything that
  executes stay off until the person enables them (VS Code Restricted Mode [16]; spec §15.5
  consent); text editing and the sandboxed preview work at once.
- Two processes never share an identity: a lock on the identity directory backs up the
  single-instance check (D-Bus may be missing, a sandbox may refuse the name: then the second
  process says « Inkwell est déjà ouvert » and exits, it does not fall back to running).

**Not proposed**: auto-converting a loose file into a project (silent write in `~/Téléchargements`);
opening a loose file's folder as a project (surprising, D8 cases); several windows (after v1).

## 4. What to ship per OS

| OS | Item | State | Action |
|---|---|---|---|
| Linux | `.desktop` `MimeType` | present (`…Inkwell.desktop:18`) | keep; do **not** add `inode/directory` (it can make Inkwell a folder handler in some file managers) |
| Linux | `.desktop` `Exec` field code | **missing** (`:12`) | `Exec=inkwell %F` |
| Linux | `[Desktop Action]` « Nouveau projet » | missing | optional, low value; skip for v1 |
| Linux | `DBusActivatable` | missing | not advisable: Wails is `NON_UNIQUE`, no `org.freedesktop.Application` |
| Linux | MIME XML `text/x-typst` | present (`mime/…Inkwell.xml:5-10`) | add `<alias type="text/vnd.typst"/>` once upstream confirms [4] |
| Linux | `text/markdown`, `text/csv` | upstream | nothing |
| Linux | AppStream `provides/mediatype` | present (`metainfo.xml.in:26-31`) | nothing |
| Linux | Nautilus extension / script | absent | **not advisable**: unstable API, not installable for scripts |
| Linux | Dolphin service menu « Ouvrir dans Inkwell » (folders) | absent | optional after v1: declarative, packaged in `/usr/share/kio/servicemenus` |
| Linux | Thunar custom action | absent | not advisable: per-user file only; document it in the help |
| Windows | ProgIDs `Inkwell.Typst`, `.md`, `.csv` in `OpenWithProgids`, `SupportedTypes`, per user (HKCU) | **missing** (`wails_tools.nsh:229-237`) | write in NSIS (or the MSIX manifest); unregister on uninstall; never force the default |
| Windows | Folder verb « Ouvrir dans Inkwell » (`HKCU\…\Directory\shell`) | missing | optional install checkbox, off by default (Windows 11: under « Afficher plus d'options ») |
| Windows | Windows 11 top-level menu (`IExplorerCommand` + sparse package) | missing | not advisable for v1: COM DLL + signed package for one menu line |
| macOS | `CFBundleDocumentTypes` (typ, md, csv, `public.folder`), rank `Alternate` | **missing** (`Info.plist`) | add; folders allow drop on the Dock icon |
| macOS | `UTImportedTypeDeclarations` for `.typ` (conforms to `public.plain-text`) | **missing** | add, imported (Inkwell does not own Typst) [14][15] |
| macOS | Finder Quick Action | absent | not advisable: « Ouvrir avec » and Dock drop suffice |
| macOS | Info.plist placeholders | wrong (`:13-17,27`) | fill from the version and licence at build |
| All | Single instance + forwarding | **missing** | Wails `SingleInstance{UniqueID: appID}` + identity lock, taken before `OpenHome` |
| All | Argument parsing | **missing** | own `Resolve` (Wails' event covers one file with a listed extension only) |
| All | Drag and drop of a file or folder on the window | missing | same `Resolve` path; cheap once D2-D9 exist |

Context-menu recommendation: « Ouvrir avec » is the only entry Inkwell installs by default on every
OS. An extra entry is only worth it for folders (D6-D7), and only where it is declarative and
removed with the package (Dolphin service menu, Windows HKCU verb, opt-in).

## 5. Executable proofs (engineering principles)

1. **Decision-table test** (Go, `Resolve`): one table row per D1-D12, on `t.TempDir()` trees
   (project, nested project, loose file, `$HOME` stand-in, symlink escaping, fifo, missing path,
   33 paths). Each test names its row. Mutants that must be caught: folder without `.inkwell`
   treated as a project; walk-up crossing `$HOME`; relative path resolved against the process cwd
   instead of the forwarded `WorkingDir`; symlink followed. Mutation ≥ 95 % on the package.
   Property test: for random trees and argument lists, `Resolve` is total, deterministic, and
   every returned path is inside an opened `os.Root`.
2. **`pkg:lint` additions** (`scripts/pkg/lint.sh`): `Exec` must match `^Exec=inkwell %F$`; every
   `MimeType` entry must be known to `update-mime-database` run over the system
   `freedesktop.org.xml` plus ours; `desktop-file-validate` and `appstreamcli validate` stay
   (`:27`, `:54`). macOS: `plutil -lint` and a check that every extension in the decision table has
   a `CFBundleDocumentTypes` entry (on the macOS runner, N35). Windows: an NSIS dry-run test that
   reads back the HKCU keys (N35).
3. **Launch with a path, e2e**: `startInstance(port, docs, args)` in `frontend/e2e/instance.ts`
   passes paths to `inkwell-server`; specs for D2, D5, D7 (sheet then undo), D11 assert the open
   tab, the header and the absence of any network question (axe fixture included, UF-1). On the
   real GTK build, `scripts/pkgtest/inside.sh` adds `gtk-launch io.github.oioio_space.Inkwell
   /tmp/a.typ` under Xvfb and checks the window title.
4. **Second launch forwards**: under `dbus-run-session`, start the real binary, start a second one
   with a file; assert the second exits 0 within 5 s, opened no listening socket and did not touch
   `INKWELL_HOME` (mtime and lock), and the first shows the file (D10). Second case without a
   session bus: the identity lock refuses the second process (non-zero exit, message), never two
   processes on one key. Go unit test of the `OnSecondInstanceLaunch` handler: forwarded data goes
   through `Resolve` (no path accepted that `Resolve` refuses).
5. **UI**: D4, D5, D7 are new states of the window statechart (UF-3, XState); their Playwright
   paths are generated, each with its ARIA snapshot (UF-10); `ux-walkthrough` before merge.

## 6. Order and cost

Each block follows the definition of « fini » (gate first, then code). Placement in
`docs/architecture/coherence.md` §5: OI1-OI3 in « Écrire et retrouver » (FM-OPEN), OI4-OI5 with the
Projects screen of « Première minute et secours » (W40 R4), OI6 in « Six plates-formes » (N35).

| Block | Content | Depends on | Cost |
|---|---|---|---|
| OI0 | Requirement in `docs/exigences.md`; decision table frozen in `gates/OI0.toml`; `Resolve` + its tests and mutants | — | S (1 day) |
| OI1 | `Exec=inkwell %F`; `pkg:lint` and `pkgtest` checks (proof 2, 3 GTK part) | — | XS (hours) |
| OI2 | Identity lock; reorder `run()` so `application.New` with `SingleInstance` comes before `OpenHome`, services added by `RegisterService`; forwarding (proof 4) | OI0 | M (2-3 days) |
| OI3 | Apply D2, D3, D6, D11 at startup and on forward; D4 first as a clean restart of the process on the new project (shutdown then re-exec), later as an in-process switch | OI0, OI2; tabs of G-0c | M |
| OI4 | Single-file mode D5 (store outside the folder, Typst root rule, header state) | OI3, G-0c | L (a week) |
| OI5 | D7/D8: folder to project with sheet and undo; untrusted-folder state; same code serves « Ouvrir un dossier… » of §5.2 and drag and drop | OI3, Projects screen | M |
| OI6 | Windows HKCU ProgIDs (NSIS), macOS document types and imported UTI, Info.plist cleanup | N35 native builds | M per OS |
| OI7 | Optional: Dolphin service menu, Windows folder verb (opt-in) | OI5 | S |

OI1 is a standalone bug fix worth shipping now: today « Ouvrir avec Inkwell » opens Inkwell on the
last project and drops the file. OI2 is the most urgent behaviour fix: two processes on one key.

## 7. Open questions

- In-process project switch versus restart (D4): restart is simple and safe (the CRDT is
  persisted), but costs a cold start (PERF-START); the Projects screen will need the switch anyway.
- Where a loose file's history lives (D5): a private state directory keyed by the file's path and
  identity, or no history beyond the editor's undo. To decide in OI4's spec.
- Whether a project may live outside `Documents/Inkwell` (D2, D6, D7). Today it cannot
  (`docs.go:22-40`); the recent-projects list must then store absolute paths.
- `text/vnd.typst`: follow the shared-mime-info merge request [4] before adding the alias.

## Sources

1. Desktop Entry Specification (Exec field codes, MimeType, Actions, D-Bus activation) —
   https://specifications.freedesktop.org/desktop-entry-spec/latest/
2. AppStream metadata, `provides` / `mediatype` — https://www.freedesktop.org/software/appstream/docs/
3. shared-mime-info 2.3 NEWS, « Add mime type for Typst files » —
   https://gitlab.freedesktop.org/xdg/shared-mime-info/-/blob/master/NEWS ;
   shared-mime-info spec — https://specifications.freedesktop.org/shared-mime-info-spec/latest/
4. shared-mime-info MR 334, « Update mimetype for Typst source files » (`text/vnd.typst`) —
   https://gitlab.freedesktop.org/xdg/shared-mime-info/-/merge_requests/334
5. GApplication, `G_APPLICATION_HANDLES_OPEN`, uniqueness — https://docs.gtk.org/gio/class.Application.html
6. Migrating to Nautilus API 4.0 (no GTK access) —
   https://gnome.pages.gitlab.gnome.org/nautilus-python/nautilus-python-migrating-to-4.html
7. Ubuntu archive, extensions incompatible with Nautilus 43 —
   https://lists.ubuntu.com/archives/ubuntu-archive/2022-August/080613.html
8. KDE, Creating Dolphin service menus — https://develop.kde.org/docs/apps/dolphin/service-menus/ ;
   KF6 executable bit — https://medo64.com/posts/meld-context-menu-in-kde-plasma-6
9. Thunar custom actions — https://docs.xfce.org/xfce/thunar/custom-actions
10. Programmatic identifiers (ProgIDs) — https://learn.microsoft.com/en-us/windows/win32/shell/fa-progids
11. Include an application in the Open With dialog —
    https://learn.microsoft.com/en-us/windows/win32/shell/how-to-include-an-application-on-the-open-with-dialog-box
12. Integrate a packaged or sparse-packaged app with File Explorer —
    https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/integrate-packaged-app-with-file-explorer
13. Extending the context menu in Windows 11 —
    https://blogs.windows.com/windowsdeveloper/2021/07/19/extending-the-context-menu-and-share-dialog-in-windows-11/
14. Apple, Defining file and data types for your app —
    https://developer.apple.com/documentation/uniformtypeidentifiers/defining-file-and-data-types-for-your-app
15. Apple, `CFBundleDocumentTypes` —
    https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundledocumenttypes ;
    Technical Q&A QA1959 (`LSHandlerRank`) — https://developer.apple.com/library/archive/qa/qa1959/_index.html
16. VS Code, Workspace Trust / Restricted Mode — https://code.visualstudio.com/docs/editing/workspaces/workspace-trust ;
    workspaces — https://code.visualstudio.com/docs/editing/workspaces/workspaces
17. Obsidian desktop 1.14.2 changelog (open files outside the vault) —
    https://obsidian.md/changelog/2026-09-15-desktop-v1.14.2/
18. Wails v3 guides (single instance, file associations; pages answered 403, facts above read from
    the v3.0.0 module source) — https://v3.wails.io/llms.txt , https://wails.io/guides/single-instance/ ,
    https://wails.io/guides/file-associations/
