# Block OI: open a file from the OS, and one Inkwell per profile (W41, OI0 partial, OI1, OI2)

Step 1 of the definition of « fini » (spec first), 2026-11-11. Study: `docs/plans/2026-11-09-os-integration.md`
(§1 evidence, §3 decision table D1-D12, §4 per OS, §6 order); requirement W41 (`docs/exigences.md:59`). This plan
narrows the study to what this block ships, fixes every number, and replaces two of the study's choices (§3, §7).
Gate: `gates/OI.toml` (frozen at commit). Formal model: `docs/protocol/tla/OISingleInstance.tla` (§6), with its
mutants and witnesses. ADR: `docs/adr/0037-single-instance.md` (proposed). User decisions of 2026-11-11: §11.

**Rigour: complete** (`docs/engineering-principles.md` §0). Reasons: concurrency between processes (two launches at
once, a crash while holding the lock); the identity key (two processes on one key is the defect fixed here, a
security and correctness fault); bytes from another process decoded by the running one; a new local server, which
touches a `CLAUDE.md` invariant and therefore needs an ADR (§7, D-ADR). Hence: decision table, TLA+ model with
mutants and witnesses, named tests, fuzzed decoder with vectors, Go mutation ≥ 95 %, `ux-walkthrough`, an
independent review by another model family before the gate.

**Scope.** In: OI1 (`Exec=inkwell %F`, `pkg:lint`), OI2 (profile lock, hand-off, raise), and the part of OI0 that
decides what the person sees with what exists today (one project per process, fixed for its lifetime; no tabs before
G-0c). Out, each with its block: the light single-file window for a file outside the open project, decided by the user
on 2026-11-11 (**OI3**, the direct successor of this block, §2.1), switching project in a running window (OI4),
a folder turned into a project (OI5), Windows and macOS packaging (OI6, N35). (Numbering: the study's OI3 « switch
project » and OI4 « loose file » are swapped, so that the successor the user asked for is OI3.) Where this block
cannot yet do what the person asked, it says so in one toast (rows R5-R7); it never drops a request silently.

## 1. Need: what the person sees and does

1. In the file manager, double-click `rapport.typ`, or right-click « Ouvrir avec Inkwell ». Inkwell is closed: it
   opens on the project that holds the file, with the file shown. Inkwell is open on that project: the window comes
   to the front with the file shown, within a second; nothing else starts.
2. Select three files and « Ouvrir avec Inkwell »: one window, never three. Windows Explorer starts one process per
   selected file for a classic verb, up to 15 at once [W1]; all of them must end in the one window.
3. Click the Inkwell icon again while it runs: the window comes to the front. Nothing else.
4. Open a file Inkwell cannot open in this window yet (outside the open project, a plain folder, a missing
   file): the window comes to the front and a toast says why, naming the file. This is the interim of this block:
   a file outside the open project is to open at once in a light single-file window (decided, §2.1, block OI3).
5. Inkwell crashed: the next launch starts normally, without a « profil verrouillé » question and without waiting.
6. Inkwell is closing (saving, stopping the network) when the person double-clicks a file: the file opens in the
   next Inkwell, which starts as soon as the first one is gone.
7. Inkwell is stuck: the second launch does not wait forever and does not start a second copy; a desktop
   notification says « Inkwell ne répond pas » within 5 s.
8. Two profiles (`INKWELL_HOME`, a developer and test setting) are two independent Inkwells.

Never, in any case: two processes on one identity key; a second process touching the identity, a project or the
network; a network or sharing question caused by opening a file (ux rule 8).

## 2. Decision table (what this block ships)

Definitions. *Docs*: the projects folder (`$INKWELL_DOCS` or `Documents/Inkwell`, `internal/app/docs.go:22-40`).
A *project* is a folder holding `.inkwell/project.json`; a *project under Docs* is a direct sub-folder of Docs that is a
project (the only ones `project.Latest` and `project.Open` reach, `internal/project/project.go:241`). The *current
project* is the one the running window has open. An argument is classified by `launch.Resolve` (§4.2); the walk up
from a path to its project stops at `$HOME`, a filesystem root or a mount point (device change), at most 64 levels.
Every row raises the window (R10 included, on the primary's side); « one toast » means at most one toast per launch.

| Row | Situation | What the person sees | Study row |
|---|---|---|---|
| R1 | First launch, no argument | today's behaviour: the most recent project, or the sample at first launch | D1 |
| R2 | First launch; the first argument is a file or folder of a project under Docs | that project opens (not the most recent one); a file is shown in the editor; a project folder shows its main file | D2, D6 |
| R3 | Running; a file of the current project | window raised; the file shown (`Workspace.openFile`); no reload, no second copy | D3 |
| R4 | Running; the current project's folder | window raised; nothing else | D6 |
| R5 | A file or folder of another project (under Docs while running, or outside Docs at any time) | **In block OI (interim):** window raised; toast « « {nom} » fait partie d'un autre projet. Inkwell n'ouvre qu'un projet à la fois pour l'instant. » **Decided (user, §11; block OI3 for a file, OI4 for a project folder):** a file opens at once, with no dialog, in a light single-file window whose header bar offers « Ajouter à un projet… » (its own project proposed first); a project folder opens that project once a window can switch project (OI4, until then the toast); nothing joins a shared project without that action | D4 (OI3, OI4) |
| R6 | A file in no project | **In block OI (interim):** window raised; toast « « {nom} » n'est dans aucun projet. Inkwell ne sait pas encore ouvrir un fichier seul. » **Decided (user, §11; block OI3):** the file opens at once, with no dialog (« le moins de clic »), in the same light single-file window with « Ajouter à un projet… » in its header bar; nothing joins a shared project without that action | D5 (OI3) |
| R7 | A folder that is not a project, including `$HOME`, `/` and a mount root | window raised; toast « « {nom} » n'est pas un projet Inkwell. » | D7, D8 (OI5) |
| R8 | Several arguments | classified one by one in argument order; the first file the current project shows is shown (one document at a time until the tabs of G-0c); every other argument gets its notice; the window names them (amended 2026-11-13 after the UX walkthrough, UX-OI-1/2/3): the requests that end within one burst (800 ms without a request ending) give one toast per kind, naming up to three files then « et {n} autre(s) », the single case keeping its sentence, and every failure (R9, R19) goes in a toast of its own with `role="alert"` naming each file and why (« « absent.md » est introuvable, « b.md » n'a pas pu s'ouvrir. »); several files shown: « {n} fichiers ouverts : … »; toasts follow one another (AdwToastOverlay's queue) | D9 |
| R9 | Missing, unreadable, not a regular file or folder (fifo, socket, device), inside `.inkwell/`, path over 4096 bytes, more than 32 arguments | a failure toast (`role="alert"`, R8); for one such argument its sentence: « « {nom} » est introuvable. », « Inkwell n'a pas le droit de lire « {nom} ». », « « {nom} » n'est pas un fichier ordinaire. », « « {nom} » est un fichier interne d'Inkwell. », « Ce chemin est trop long. », « {n} fichiers non ouverts : 32 au plus à la fois. » | D11 |
| R10 | Second launch while a primary runs on this profile | the second process forwards its arguments, working directory and activation token, waits for the acknowledgement, exits 0; it opens no identity, project, network, window or GTK; the primary applies R3-R9 with the forwarded working directory | D10 |
| R11 | Second launch; the primary holds the lock but gives no answer within 5 s | desktop notification « Inkwell ne répond pas » / « Inkwell est déjà ouvert avec ce profil mais ne répond pas. Fermez-le puis réessayez. »; exit code 3; no second copy | new |
| R12 | Second launch while the primary is closing | the second waits for the lock (at most 15 s), becomes the primary itself and applies R1-R9 to its own arguments | new |
| R13 | The primary crashed | the next launch is primary at once: the kernel released the lock; a leftover socket file is removed by the new primary; no question, no delay | new |
| R14 | Another `INKWELL_HOME` | an independent instance (its own lock and socket) | new |
| R15 | `inkwell --version` | prints the version; takes no lock (`cmd/inkwell/cli.go:8-10`) | D12 |
| R16 | Second launch with no argument | window raised; no toast | new |
| R17 | Second launch while a primary of another Inkwell version runs | the primary answers « unsupported »; notification « Une autre version d'Inkwell est ouverte. Fermez-la puis réessayez. »; exit code 3 | new |
| R18 | The profile folder supports no lock (`ENOLCK`, `EOPNOTSUPP`; NFS without lockd) | Inkwell does not start; notification « Inkwell ne peut pas s'assurer d'être seul à utiliser son profil ; il ne démarre pas. »; exit code 4 | new |
| R19 | Running; the window cannot show the file it was given (not in the project's list, unreadable) | window raised; toast « Inkwell n'a pas pu ouvrir « {nom} ». »; the second launch has exited 0 (it was received and queued) | new (2026-11-12) |

Rules of composition, fixed: one toast per launch request; « {nom} » is the base name of the argument as given; the
toast is built in the webview from a structured notice (`launch.Notice`: kind, name, others), never from a Go string,
so that `frontend/src/i18n/{fr,en}.json` holds every text; the English texts are written with the French ones. A
symlink argument is resolved first (`filepath.EvalSymlinks`) and the target classified: the argument comes from the
same user, who can read the target anyway, and refusing symlinks would break a common setup (the study's « symlink
followed » mutant is replaced by « symlink not resolved », §8). Notifications of R11, R17, R18 are the only Go-side
texts (no webview exists in that process): French when `LC_ALL`, else `LC_MESSAGES`, else `LANG`, starts with `fr`,
English otherwise; also printed on standard error.

### 2.1 Files outside the open project: the decided behaviour, and which block builds it

Decided by the user (§11, Q1, like the JetBrains IDEs): a file outside the open project opens **at once, with no
dialog**, in a **light single-file window** (a window of the same process, the project window untouched). Its
header bar offers « Ajouter à un projet… »; **nothing joins a shared project without that explicit action**. A file
of the open project opens in the project window (R3), as in JetBrains.

**Where it lives: block OI3, the direct successor of this block (nothing between them), not this block.** Reasons:

1. *This block must stay shippable and coherent.* OI closes a correctness and security defect (two processes on one
   identity key, §1) and the OS entry points (`%F`, hand-off, raise). Every row of §2 is complete and honest without
   the light window: R5/R6 give a toast naming the file, never a silent drop.
2. *The light window is a feature of its own, with its own unknowns*, none of which OI needs: a second window in the
   process, and so a process life that ends with the last window, not the project window (when `Primary.Closing` and
   `Close` run, and how a launch during that time behaves: a change to the model of §6, not an extension of it); the
   save and external-change model of a file that belongs to no project (the CRDT, the BLAKE3 store and the signed
   member list exist per project); the « Ajouter à un projet… » flow (choose or create the root, copy or move, what a
   shared project then does with the file); and `ux-walkthrough` of a new window kind and its header bar. Putting
   them here would tie the lock fix to a UI block whose rigour (complete: a new local window lifecycle) and review
   would delay it, and would risk shipping the lock with a half-designed window.
3. *No rework.* `launch.Resolve` already separates `OtherProject` and `Loose` and carries `Project` and `Rel`
   (§4.2); OI3 changes `Plan`'s mapping for these kinds and the delivery (a window instead of a notice). The tests
   of R5/R6 classification stay; only the plan's outcome tests change. The interim toasts are existing texts in the
   catalogue (`open.notice.*`), added by no new component, and OI3 deletes them: not a bolted-on feature.
4. *The interface to OI3 is fixed here:* the request path (instance → queue → `Resolve` → `Plan` → stream) is
   unchanged; `Closing`/`Close` stay the last steps of the **process** shutdown, which OI3 will move from « the
   project window closed » to « the last window closed »; a first launch with only a loose file (no project to open)
   is OI3's to settle (JetBrains opens the light window alone; here R1 would first open the most recent project).

Open for the user (§11, Q3): whether the interim toast is acceptable for the one block it lives, or OI3 must ship
with OI.

## 3. Wails v3.0.0 single instance: not used (evidence)

Read from the module source, `~/go/pkg/mod/github.com/wailsapp/wails/v3@v3.0.0/pkg/application/` (`go.mod:18`).

| # | Fact (file:line) | Consequence for Inkwell |
|---|---|---|
| W-1 | The key is one `UniqueID` per application: D-Bus session name `<UniqueID>.SingleInstance` (`single_instance_linux.go:64,109`), session mutex `wails-app-<UniqueID>-sim` (`single_instance_windows.go:40-45`), `<UniqueID>.lock` in the user's temp dir (`single_instance_darwin.go:49-63`) | not per profile: a second `INKWELL_HOME` is forwarded to the wrong instance unless the id embeds a profile hash; and on Linux the scope is a **session bus**, not the profile: two sessions of the same user (a desktop login and an SSH session with its own bus) both become primary on one key. A per-profile file lock is needed in any case (the study already said so, §3 last rule) |
| W-2 | No session bus: `dbus.ConnectSessionBus` fails (`single_instance_linux.go:91-96`), `New` calls `fatal` (`application.go:214`), which exits 1 (`application.go:575-578`) | a launch without a session bus cannot start at all |
| W-3 | A failed notify is only logged, then `os.Exit(ExitCode)` (`application.go:207-212`), `ExitCode` 0 by default (`single_instance.go:40-41`) | the arguments are lost and the launcher sees success: the silent loss R10 forbids |
| W-4 | macOS: fire-and-forget distributed notification then `os.Exit` (`single_instance_darwin.go:13-22,74-84`); the observer is registered in `run()` (`application_darwin.go:238-243,315-320`), after the lock taken in `New` | a second launch between `New` and `Run` is lost; the payload (file paths) is visible to any process observing the distributed notification centre |
| W-5 | Windows: `SendMessage` with no timeout (`w32/window.go:364-384`) on a window found by `FindWindowW` (`single_instance_windows.go:70`); Microsoft documents message-only windows as « cannot be enumerated », found with `FindWindowEx(HWND_MESSAGE, …)` [W2] | a hung primary hangs every second launch forever; finding the message-only window by `FindWindowW` is not documented to work |
| W-6 | The lock is taken inside `application.New` (`application.go:203-218`), which runs after `OpenHome`, `openProject` and `app.Start` (`cmd/inkwell/main.go:49,54,66,97`) | adopting it means reordering the whole of `run()` and registering every service late |
| W-7 | Server build: `acquire` returns nil, `notify` fails (`application_server.go:434-451`) | the e2e suite (server build, `frontend/e2e/instance.ts:9`) could never exercise it |
| W-8 | Payload: unbounded JSON; optional AES-GCM with a key both processes must know (`single_instance.go:43-45,84-98`) | a key compiled into the binary is no secret; no bounds |

**Verdict: do not use `Options.SingleInstance`.** Own package `internal/instance` (§4.1): a kernel lock in the
profile plus a Unix-domain socket beside it, the same code on Linux, macOS and Windows, testable in `go test` with
real processes and in the server build. GTK stays `G_APPLICATION_NON_UNIQUE` (`linux_cgo.h:16-22` explains why Wails
needs it). Wails is still used for what it does well: `WebviewWindow.Show/Restore/Focus`
(`webview_window.go:518,1331,1692`), `NativeWindow` (`webview_window.go:1664`, the `GtkWindow*` on Linux,
`webview_window_linux.go:474-476`) with `InvokeSync` (`mainthread.go:23`) to apply the activation token, and, at N35,
macOS Apple Events (`application_darwin_delegate.m:14` → `HandleOpenFile`, `application_darwin.go:813-825`), which
deliver files to the running app without a second process. `Options.FileAssociations` stays unset: on Linux and
Windows its event fires only for exactly one argument with a listed extension (`application_linux.go:66-85`) and
would double our own intake.

Prior art, and what it does worse. Chromium's `ProcessSingleton` (per profile, like ours) uses a `SingletonLock`
symlink naming `hostname-pid`, decides staleness by checking that pid, and kills it with `SIGKILL` [W3]: pid reuse and
a profile shared between hosts make that heuristic wrong; it waits 20 s for an answer. It puts the socket in a
temporary directory because « many networked filesystem implementations do not support unix domain sockets » [W3]:
adopted here as the fallback (§4.1). Electron's `requestSingleInstanceLock` reuses it. VS Code keys its IPC socket on
the user-data directory. Our improvement: a kernel lock (`flock`, `LockFileEx`) that the OS releases when the holder
dies, so there is no staleness heuristic at all; an acknowledgement after queueing, so exit 0 means « taken »; a
« closing » answer, so a launch during shutdown is not lost; peer credentials checked on both ends.

## 4. Design

### 4.1 `internal/instance` (new; role: one running process per profile, and the hand-off to it)

Imports: standard library and `golang.org/x/sys` (`go.mod:22`) only; nothing from inkwell. Files: `instance.go`
(API, retry loop), `wire.go` (request codec), `lock_unix.go` / `lock_windows.go`, `peer_linux.go` / `peer_darwin.go`
/ `peer_windows.go`, `path.go` (socket path rule).

```go
type Request struct {
	ID         [16]byte // random per launch; the primary ignores a repeated ID (keeps the last 64)
	Cwd        string   // absolute working directory of the launch
	Activation string   // XDG_ACTIVATION_TOKEN, else DESKTOP_STARTUP_ID; "" when absent or over 1024 bytes
	Dropped    uint16   // arguments beyond the 32 sent (R9)
	Args       []string // at most 32, as given on the command line
}

type Primary struct{ /* lock file, listener, state */ }

// Start claims the profile at dir or hands req to the process that holds it.
// Exactly one of: (p != nil, nil) this process is the primary; (nil, nil) req was acknowledged;
// (nil, err) with err one of ErrNotResponding, ErrOtherVersion, ErrNoLock, or an I/O error.
func Start(ctx context.Context, dir string, req Request, o Options) (*Primary, error)

// Serve accepts requests until Close; handle is called once per new request ID, from one goroutine,
// and must only queue (it runs before the acknowledgement is written).
func (p *Primary) Serve(handle func(Request))
func (p *Primary) Closing()      // from now on every request is answered "closing"
func (p *Primary) Close() error  // last step of a clean exit: unlink the socket, then release the lock (this order)

type Options struct { // zero value = the values of the table below; tests shorten the durations
	Deadline, ClosingWait, ReadTimeout time.Duration
}
```

`Start`, the loop the model checks (§6):

1. `dir` canonical (`filepath.Abs`, `EvalSymlinks`; the caller created it 0700, as `OpenHome` does today,
   `internal/app/node.go:43-61`). Open `instance.lock` through `os.OpenRoot(dir)` (`O_RDWR|O_CREATE`, 0600;
   close-on-exec by Go: `os/file_unix.go:261`, `os/file_windows.go:162` with `syscall_windows.go:397`, so `typst` and
   `tinymist` never inherit it).
2. Try the lock without blocking: `flock(LOCK_EX|LOCK_NB)` (as `internal/app/snapshot_session_unix.go:48` already
   does), Windows `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY)` on one byte
   (`x/sys/windows/zsyscall_windows.go:2923`, flags `syscall_windows.go:67-68`). `ENOLCK`/`EOPNOTSUPP` → `ErrNoLock`
   (R18, fail closed: the profile holds the identity key).
3. Got it → primary: remove both socket candidates (stale by construction: only the lock holder ever binds or
   unlinks, rule D-UNLINK), bind the first candidate that works, `listen`, return. The `*Primary` (hence the lock
   file) is kept reachable by `run` until the process exits (`runtime.KeepAlive`). Rule D-HOLD: the lock is released
   only at the very end, after the identity, the project and the network are closed and the socket file removed:
   explicitly as the last step of a clean exit (Microsoft recommends it, the OS may take time otherwise [W12]), by the
   kernel on a crash. Nothing else ever unlocks it (mutant LockReleasedEarly).
4. Held → connect to the candidates in order; on success check the peer (below), write one frame, read one status
   byte. `accepted` → return `(nil, nil)`. `closing` → the closing budget starts (`ClosingWait`). `busy`, connect
   refused, no socket file, timeout → back off and **go to 2** (rule D-RETRY: a crash of the holder turns the next
   try into a primary). `malformed` → I/O error (a bug, not retried). `unsupported` → `ErrOtherVersion` (R17).
5. Past `Deadline` (or `ClosingWait` once `closing` was seen) with the lock still held → `ErrNotResponding` (R11).

**Contract simplified (2026-11-12, decision in `docs/autonomous-decisions.md`, row « OI »; supersedes the three
amendments below, kept for the record).** After three review rounds the acknowledgement contract is the one of VS Code
and GApplication: a second launch exits 0 once the primary has **received and queued** its request (validated, bounded
queue of 64: « busy » if full; « closing » → retry or takeover as in R12). The queue opens only once the primary can
show a file (identity, project and network up): until then « busy », so a failed start acknowledges nothing. From
then on the **primary owns the outcome**: its window shows the file or says why it cannot in a toast (another project,
no project, a name that is not UTF-8, a file it cannot open: « Inkwell n'a pas pu ouvrir « {nom} ». »). Go keeps each
request, and its slot of the 64, until the window reports it handled (`{"handled":id}`: the file shown or its toast
up; an internal window → Go report, not an acknowledgement to the launch); at most 8 are in flight; a window that
goes away gets the unhandled ones again (rule D-OWNER below: by generation). A step that fails gives the toast and
frees its slot; a step past its bound (readiness 60 s, showing 10 s) is said slow and keeps its slot until it ends
(rule D-OWNER below). Closing the window first stops acknowledging
(« closing ») and waits, at most 2 s, until the window reported handled what was acknowledged; what is still not
handled then is logged: the residual of a window that cannot answer while it closes. A crash of the primary between
receipt and display loses the request: accepted residual. Removed: the webview's
applied/failed answer, the `failed` and `starting` replies, the per-request deadline and `cancel`, the handover mark,
the startup wait for forwarded requests (a forwarded request after a takeover follows the normal rows).

**Rule D-OWNER: one owner at a time, by generation (design decision 2026-11-13, method v2 rule 3, after the
fix-diff check of round 5 found reports with no stream ownership; `gates/OI/REVIEW.md`, round 6).** Go numbers each
connection of the window to the `open` stream (generation g, monotonic). A new connection supersedes the previous one:
Go closes the old stream, and every request delivered to the old generation and not reported handled goes back to the
queue, to be sent on the new one first, in order. A delivery records the generation it went to; a `{"handled":id}` is
accepted only from the stream of the generation that currently holds that id; a report of a superseded stream, or about
an id not held (unknown, duplicate), is ignored and logged at debug level. The end of a stream gives back only what
that stream held, never what a newer one holds. The webview starts each connection from a fresh state: what the old
one held is discarded (Go sends it again), nothing is reported through an old socket, and a new request starts only
once the previous one's steps have really ended, never overlapped. A step past its bound (readiness 60 s, showing
10 s) is not announced as a failure (it may not be one): the window says « L'ouverture de « {nom} » prend plus de
temps que prévu. » and the request keeps its slot until the step really ends; a late success opens the file with no
failure toast, a late rejection gives the « n'a pas pu ouvrir » toast (amendment 2026-11-13, R-trust). Trust: reports arrive only on the Wails stream between the application's own webview and Go
(ADR 0009, no local port), so the sender is our window; the generation guards against our own stale connections, not
against another sender. Model: `OISingleInstance` actions `Connect`, `Reconnect`, `EndStream`, `Send`, `Apply`,
`Deliver`, invariants `SlotsAgree` and `RemovedWasShown`, property `NoStrandedSlot`; mutants `StaleReportAccepted`
and `ResetAllOnDisconnect`; witness `NoStaleReport`. Sources: the Wails stream semantics (`wails/v3@v3.0.0`,
`pkg/application/stream.go`: `Close` notifies the webview, `Context` ends with the connection), the same per-session
ownership `internal/bridge` uses for the network stream (`Router.Serve`: a page reload replaces the attached
connection), and the fencing-token pattern (a monotonic epoch per owner, a message of an older epoch refused;
M. Kleppmann, « How to do distributed locking », 2016, https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html).

**(Superseded) Amended 2026-11-12 after the cross-family review (`gates/OI/SIMPLIFY.md`, « Codex review »).** The
acknowledgement is sent once the **window applied** the request, not once it is queued: `Serve(handle func(Request,
*Ticket) bool)` queues a request with a ticket; the window's delivery acknowledgement (`{"applied":id}` on the
`open` stream, one request in flight) calls `Ticket.Applied`; a launch is held at most `ReadTimeout`, then told
`busy`, and its retry waits on the same ticket; `Closing` drops every pending ticket (`closing`: the launch becomes
the next primary). A primary that answers (`busy` or `closing`) earns `ClosingWait`. The candidates are computed on
every try. The residual F3 (crash between acknowledgement and display) is gone: nothing is acknowledged before it is
shown.

**(Superseded) Second amendment, same day (re-review).** The webview answers each request `{"applied":id}` or `{"failed":id}`
(its application threw, for instance a file the project does not list), and Go fails a request it gets no answer for
within 10 s (`defaultApplyTimeout`), then serves the next: a failed request is answered `failed` (status 5), its
retry too, and the launch shows R19 and exits 5. **Rule D-FORWARD:** a request keeps its intended project across a
takeover: a launch B told `closing` hands its request, unchanged, to whichever launch C takes the lock next; if C is
still starting and was launched with nothing of its own, C opens the project of the first request forwarded while it
starts (as R2 does for its own arguments), so B's file is shown; if C already has a project open, the normal rows
apply (R5's notice for a file of another project). Model: witness `NoForward` (three launches, a takeover by a third
contender, B applied by C); tests `TestHandoffAcrossTakeover`, `TestTakeoverOpensForwardedProject`.

**(Superseded) Third amendment: a design decision (2026-11-12, method v2: a third finding round is redesigned, not patched).**
D1, one owner of the deadline: the webview says `{"ready":true}` once its workspace is ready, and Go sends nothing
before; the 10 s per-request deadline runs from the send, so never before readiness. Until the window is ready, at
most `StartWait` (60 s) after serving, a held launch is told `starting` (status 6) and keeps waiting (its patience is
renewed on each `starting`); past that bound it is told `busy` and ends with R11: a slow first open is never a false
failure. On expiry Go fails the request and sends `{"cancel":id}`; the webview aborts that application
(`AbortController`, checked before and after each step), frees its slot at once even if a step never ends, never
shows the file later, and answers nothing for it; a late answer is ignored on both sides. D2: a primary that closes
with requests it could not apply leaves a handover mark (`instance.handover`, written and removed by lock holders
only); the next primary, when launched with nothing of its own, waits up to 500 ms for the first of those launches to
come back before it chooses its project, from the queue as it is at that moment. D3: the burst end-to-end test checks
each request applied once, in Go's order.

Socket path rule (both ends compute it; pinned by vectors). Candidate 1: `<dir>/instance.sock`. Candidate 2: in the
private runtime directory, `inkwell-<first 32 hex digits of SHA-256(canonical dir)>.sock`; the runtime directory is
`$XDG_RUNTIME_DIR` on Linux when it is owned by the user with mode 0700 (basedir spec [W4]), else `os.TempDir()/inkwell-<uid>`
created 0700 and checked (`Lstat`: a directory, not a symlink, owner = uid, mode 0700; else candidate 2 is skipped:
classic `/tmp` squatting); on macOS `os.TempDir()` (per-user `/var/folders/…/T/`); on Windows `os.TempDir()`
(`%LOCALAPPDATA%\Temp`). Candidate 1 is skipped when its path exceeds 103 bytes: `sun_path` is 108 bytes on Linux
[W5] and Windows (`UNIX_PATH_MAX` 108 in `afunix.h` [W6]), 104 on macOS (`sys/un.h`); 103 leaves the terminating NUL
on all three, one rule. The primary falls back to candidate 2 when binding candidate 1 fails (a network or FUSE
profile folder [W3]).

Authentication. The socket's directory is private (0700 profile, checked runtime dir); in addition each end checks
the other's credentials before reading or writing: Linux `SO_PEERCRED` (`x/sys/unix/syscall_linux.go:1283`), macOS
`LOCAL_PEERCRED` at level `SOL_LOCAL` (`syscall_darwin.go:489`; constants `zerrors_darwin_arm64.go:935,1379`), uid
equal to `os.Getuid()`; Windows `SIO_AF_UNIX_GETPEERPID` [W6][W7] then `OpenProcessToken`/`GetTokenUser`, SID equal to
our own (N35). No token: a same-user process can read any token the profile holds, and another user is refused by
the kernel credentials. Nothing is logged but counts: never a path, never the activation token.

Wire format v1 (big-endian; one request per connection; canonical: exactly one encoding per request):

```
frame      = length:u32 body                         ; 1 ≤ length ≤ 136280
body       = version:u8(=1) id:16B cwd activation dropped:u16 count:u8 arg{count}
cwd        = n:u16 bytes{n}                          ; 1 ≤ n ≤ 4096, no 0x00
activation = n:u16 bytes{n}                          ; 0 ≤ n ≤ 1024, bytes 0x21..0x7E only
arg        = n:u16 bytes{n}                          ; 1 ≤ n ≤ 4096, no 0x00
count      : 0 ≤ count ≤ 32; no byte after the last arg
reply      = status:u8   ; 0 accepted, 1 closing, 2 busy, 3 malformed, 4 unsupported version
```

136280 = 1 + 16 + (2+4096) + (2+1024) + 2 + 1 + 32 × (2+4096). The decoder checks the length against the cap before
allocating, then every field; anything else is `malformed` and the connection is closed. A primary answers
`unsupported` to any version other than 1; a future v2 client falls back to v1 when its request fits.

Numbers, each with its ground:

| Value | Ground |
|---|---|
| 32 arguments, 4096 bytes each, cwd 4096 | the study's bound (§3 security rules); 4096 = Linux `PATH_MAX`; macOS paths are ≤ 1024; longer Windows long paths are refused (R9); bounds a request to 136 KiB |
| activation token ≤ 1024 bytes, printable ASCII | xdg-activation tokens and startup ids are short ASCII strings; an over-long one is dropped by the client, not refused |
| `Deadline` 5 s | the study's proof 4 bound; a healthy hand-off is one connect and one round trip on a local socket (well under 10 ms); 5 s stays under the 10 s limit of attention [W8], so R11's notification comes while the person still waits |
| `ClosingWait` 15 s after a `closing` answer | `shutdown` bounds its loops at 5 s (`cmd/inkwell/main.go`, `shutdownLoopTimeout`), then closes the network and the project; 3 × that bound |
| back-off 10 ms, doubling, cap 200 ms (a pure function, table-tested) | the lock-to-listen window of a starting primary is a few syscalls: the first retry finds it listening; ≤ 30 tries in 5 s, negligible CPU |
| `ReadTimeout` 2 s per connection | a client writes ≤ 136 KiB in one write on a local socket; bounds a client that connects and sends nothing |
| 16 connections served at once | Explorer starts at most 15 processes for one multi-selection [W1] |
| queue 64 requests, then `busy` | 4 × that burst; the client retries |
| 64 recent IDs | a retried request after a lost acknowledgement is shown once |

### 4.2 `internal/launch` (new; role: turn launch arguments into what the window does)

Pure over the filesystem it is given; no network, no UI text.

```go
type Kind uint8   // Show, Current, OtherProject, Loose, Folder, Refused
type Reason uint8 // Missing, Unreadable, Special, Internal, TooLong, TooMany
type Intent struct {
	Kind    Kind
	Name    string // base name of the argument as given (for the toast)
	Project string // absolute project folder, when in a project
	Rel     string // slash path inside Project, for Show
	Reason  Reason
}
type Env struct{ Cwd, Home, Docs, Current string } // absolute; Current "" before a project is open
func Resolve(args []string, dropped int, env Env) []Intent      // total, deterministic, argument order
func Startup(intents []Intent, docs string) (projectDir string) // R2: "" means the most recent (R1)
type Notice struct{ Kind Kind; Reason Reason; Name string; Others int }
type Outcome struct{ Show string; Notice *Notice }               // at most one file, at most one toast
func Plan(intents []Intent) Outcome                              // R3-R9, R16
```

`Resolve` makes a relative argument absolute against `env.Cwd` (the forwarded one for R10, never the primary's own
working directory), `Clean`s it, refuses over 4096 bytes, resolves symlinks, `Lstat`s the target (regular file or
directory only), refuses anything under a `.inkwell` folder, then walks up for `.inkwell/project.json` with the stop
rules of §2. Every later file access goes through the project's `os.Root` (engineering principles §4); `Resolve`
only decides.

### 4.3 `cmd/inkwell`

`run` (`cmd/inkwell/main.go:41`) becomes `startup(args, deps) int` with its effects injected (testable), called by
`run`. Order, fixed:

1. `--version` (R15): print, return; no lock.
2. Profile directory: `app.HomeDir()` (the path `OpenHome` computes today, created 0700; `OpenHome` keeps its
   behaviour and uses it).
3. `instance.Start(ctx, dir, Request{ID: random, Cwd: os.Getwd, Activation, Dropped, Args})`. Second launch:
   acknowledged → exit 0 (R10); `ErrNotResponding`/`ErrOtherVersion` → notification, exit 3 (R11, R17);
   `ErrNoLock` → notification, exit 4 (R18); other error → log, exit 1. **Nothing before this step opens the
   identity, a project, the network or GTK.**
4. Primary: create the open queue (`app.OpenQueue`, bounded 64, holds `instance.Request`s until the webview reads
   them), `go primary.Serve(queue.Push)`, push the primary's own request.
5. Then today's sequence (`OpenHome`, project, `app.Start`, backup, `application.New`, `Run`), with one change: the
   project is chosen by `launch.Startup` over the primary's own intents (R2), else `project.Latest` (R1).
6. `OnShutdown`: `primary.Closing()` first, then today's `shutdown` (`main.go`), then `primary.Close()` as the very
   last step: a launch during shutdown waits and becomes the next primary (R12).

Delivery to the webview: a Wails stream `inkwell/open` (`app.OpenStreamName`; ADR 0009: the bridge is streams), like
the presentation hub (`internal/app/present.go`). For each queued request the Go side runs
`launch.Plan(launch.Resolve(…, Env{Cwd: req.Cwd, Current: project dir}))`, sends `{show, notice}` to the webview and
raises the window on the main thread: `gtk_window_set_startup_id(NativeWindow(), token)` when a token came (GTK 4,
`gtkwindow.h:101`, `GDK_AVAILABLE_IN_ALL`, local GTK 4.22.5; to call « before `gtk_window_present` » [W9]), then
`Restore` and `Focus`; a small cgo file `raise_linux.go` (`//go:build linux && !server`), no-op in the server build
(as built: package `internal/activate`, section 13).
Without the token GNOME may show « Inkwell est prêt » instead of raising (focus-stealing prevention): forwarding the
token, as `emacsclient` does for Emacs [W10], is the fix. Notifications (R11, R17, R18): Linux
`org.freedesktop.Notifications.Notify` over the session bus through `godbus/dbus/v5` (already in the module graph,
`go.mod:42`, becomes direct), falling back to standard error; Windows `MessageBox` (`x/sys/windows`,
`zsyscall_windows.go:4380`, N35); macOS standard error (Finder never starts a second process).

### 4.4 Webview

`frontend/src/workspace/openRequests.ts`: reads `inkwell/open`, calls `Workspace.openFile(show)`
(`frontend/src/workspace/workspace.svelte.ts:132`) when `show` is set, and shows the notice through the existing toast
(`frontend/src/ui/Toast.svelte`, `notice.svelte.ts`) with the texts of `frontend/src/i18n/{fr,en}.json` (keys
`open.notice.*`). A request that arrives before the workspace is ready waits for it. Nothing new in the window's
layout: an existing toast, an existing action.

### 4.5 OI1: desktop entry, MIME, manual page, `pkg:lint`

- `build/linux/io.github.oioio_space.Inkwell.desktop:12`: `Exec=inkwell %F`. `%F`, « a list of files … each file is
  passed as a separate argument »; field codes are removed when no file is opened [W11]. Not `%U`: Inkwell opens only
  local files; GIO hands a FUSE path for a remote location to a `%F` application. Several selected files reach one
  process on Linux (R8 then R10 for a running instance).
- `MimeType=text/x-typst;text/markdown;text/csv;text/tab-separated-values;text/x-psv;` (user decision Q2, §11): Typst,
  Markdown and the separated-value formats the editor reads (`frontend/src/editor/csv/dialect.ts`: `.csv`, `.tsv`,
  `.tab`, `.psv`), and nothing else. `text/x-typst` is upstream (`/usr/share/mime/packages/freedesktop.org.xml:38814`,
  shared-mime-info 2.4-3.fc44), `text/csv` (`:35360`), `text/tab-separated-values` (glob `*.tsv`). Our
  `mime/io.github.oioio_space.Inkwell.xml` keeps `text/x-typst` for older systems and gains the globs shared-mime-info
  lacks: `*.tab` added to `text/tab-separated-values`, and `*.psv` for `text/x-psv`, an unregistered `x-` type
  defined there (sub-class of `text/plain`, comments in English and French). `scripts/pkg/lint.sh` checks the
  `MimeType=` line and, through `update-mime-database` on the package, the globs `*.typ`, `*.tab`, `*.psv`. Not
  added: `inode/directory` (would make Inkwell a folder handler, study §4), `text/plain` (user decision Q2).
- `build/linux/inkwell.1` SYNOPSIS: `inkwell [FILE|FOLDER]...` with one paragraph on R10.
- `scripts/pkg/lint.sh`: the desktop entry must contain the line `Exec=inkwell %F` exactly;
  `scripts/pkg/selftest.sh` gains `refuse "a desktop entry without %F"` on a copy without the field code.
- `Exec`, `MimeType`, `StartupNotify=true` (`:15`, which makes the launcher pass the activation token) are the whole
  Linux integration: same entry for GNOME Files, Dolphin and Thunar.

### 4.6 macOS and Windows (plan; built and tested at N35)

- **Lock and socket**: the same `internal/instance`. Windows: `LockFileEx`, released by the OS when the process ends
  (« the time it takes … depends upon available system resources » [W12]: the retry loop absorbs it); AF_UNIX
  (Windows 10 1803 and later; Go builds `net/unixsock_posix.go` for Windows, line 5), socket in `%APPDATA%\inkwell`
  (user-only ACL) else `%LOCALAPPDATA%\Temp`; Go's own test suite skips one AF_UNIX test as « not reliable on
  windows » (`net/unixsock_test.go:486`), so the N35 Windows runner runs the whole `internal/instance` suite; a named
  pipe with an explicit DACL and `FILE_FLAG_FIRST_PIPE_INSTANCE` is the fallback if it fails there.
- **Windows raise**: Windows refuses `SetForegroundWindow` to a background process; the second launch (which the
  person just started, so it may set the foreground) calls `AllowSetForegroundWindow(peer pid)` before sending.
- **Windows file association** (study §4, OI6): HKCU ProgIDs, `OpenWithProgids`, command `"…\inkwell.exe" "%1"`;
  Explorer then starts one process per selected file (≤ 15 [W1]): exactly the concurrent case R10 and the model cover.
- **macOS**: Finder and the Dock deliver files to the running app by Apple Event, no second process; at cold start
  also by Apple Event, not `argv`. The Wails `ApplicationOpenedWithFile` event (`application_darwin.go:813-825`) is
  routed into the same open queue; the lock covers `open -n` and a launch from a terminal. Document types and the
  imported UTI as in the study §4.

## 5. Data flow

```
launcher ──argv, cwd, XDG_ACTIVATION_TOKEN──► inkwell (2nd)
   instance.Start ── lock held ──► connect instance.sock ── peer uid ✓ ── frame v1 ──► primary
   ◄────────────── status accepted ─────────────────────────── queue.Push (≤ 64) ◄──┘
   exit 0                                                       │
                                     launch.Resolve(args, req.Cwd) → launch.Plan
                                     ├─ raise: set_startup_id(token), Restore, Focus (main thread)
                                     └─ stream inkwell/open {show, notice} → openRequests.ts
                                          → Workspace.openFile(show) / toast (i18n)
```

## 6. Formal model (`OISingleInstance.tla`)

### 6.1 What it models

Each launch on one profile is a process: `start` → lock → (primary) `unlink` → `bind` → `own` → `serve` → `closing`
→ `closed` → `done`, or (second) `dial` → `wait` → `exit0`, with `backoff` back to `start`. Kernel objects: the lock
(released by `Crash` and `Exit`), the socket file (absent, stale, live), the per-connection reply (none, ack,
closing, lost). Faults: `Crash` (any process, any time, bounded by `MaxCrashes`), `Hang` (a primary stops answering,
keeps the lock). Time is abstract: `Timeout` is always allowed while no reply came; `GiveUp` only while the lock
holder is hung (what the 5 s deadline is for). Initial socket: absent or stale (a crash in a previous run).
Fairness: weak on every step, strong on `Accept`, `ReplyClosing` and `GiveUp` (enabled infinitely often across
retries); none on `CloseStart`, `Hang`, `Crash`.

### 6.2 Properties

| Id | Property | Kind |
|---|---|---|
| S1 `MutualExclusion` | at most one process has the identity open | invariant |
| S2 `IdentityNeedsLock` | the identity is open only by the lock holder | invariant |
| S3 `OnlyHolderListens` | a live socket belongs to the lock holder | invariant |
| S4 `ExitOkImpliesQueued` | a launch that exits 0 had its request received and queued by a primary (contract simplified 2026-11-12: acknowledgement means queued, not applied) | invariant |
| S5 `GaveUpJustified` | a launch gives up only on a hung holder | invariant |
| L1 `Settles` | every launch eventually serves, hands off, gives up on a hung holder, or crashed | liveness |
| L2 `AckedApplied` | a request a primary received and queued is eventually shown by its window, or said why not, with three explicit escapes: that primary crashes (residual F3), hangs, or its window stops answering (`Stuck`: the close residual); a primary that closes cleanly drains first | liveness |
| S6 `SlotsAgree` | what the current window holds, Go holds as delivered to that window's generation (rule D-OWNER, 2026-11-13) | invariant |
| S7 `RemovedWasShown` | Go removes a request only once a window showed it or said why not | invariant |
| L3 `NoStrandedSlot` | every request a primary received leaves Go's queue, whatever the reconnections (a report of a superseded stream is ignored, rule `AcceptsReport`), with the same three escapes | liveness |

### 6.3 Mutants (each one rule of §4.1 broken; each must be detected)

| Mutant | Override | Broken rule | Detected by |
|---|---|---|---|
| NoLock | `MayLead <- NoLockLead` | primary = « no socket answers » (the socket-only design) | S1 |
| EarlyIdentity | `IdentityAtStart <- EarlyIdentity` | identity opened before the lock (today's order) | S1 |
| AckNoQueue | `Enqueue <- AckNoQueue` | never queued, so never applied nor acknowledged | L2 |
| FireAndForget | `WaitsForReply <- FireAndForget` | exit 0 after sending, no acknowledgement (Wails macOS, W-4) | S4 |
| LockReleasedEarly | `ReleaseOnClose <- ReleaseEarly` | lock released when closing starts | S1 |
| CrashKeepsLock | `ReleaseOnCrash <- KeepLockOnCrash` | pid or `O_EXCL` file left by a crash | L1 |
| GiveUpOnClosing | `GiveUpOnClosing <- GiveUpClosing` | « closing » read as « already running » | S5 |
| SecondUnlinks | `UnlinksStale <- SecondUnlinksStale` | a second launch removes a « stale » socket (it may be the new primary's) | L1 |
| NoRetryLock | `AfterBackoff <- DialAgain` | retries reconnect without retrying the lock | L1 |
| NoDrain | `ToApply <- NoDrain` | requests queued, but the window never reads the queue (rule D-DRAIN) | L2 |
| AckDropOnClose | `AckAtQueue <- AckQueued` (`MaxCrashes = 0`) | acknowledged when queued: a clean shutdown before the window applies it loses it (added 2026-11-12, review) | L2 |

Witnesses (anti-vacuity; each invariant must be **violated** by the sound model): `NoHandoff` (some launch exits 0),
`NoGiveUp` (Hang configuration: some launch gives up), `NoTakeover` (a launch serves after a primary crashed),
`NoHandover` (a launch serves after a primary closed normally).

Added 2026-11-13 (rule D-OWNER; the current list and results are in `docs/protocol/tla/README.md`): the open stream
is modelled by generation (`Connect`, `Reconnect`, `EndStream`, `Send`, `Apply`, `Deliver`; constant `MaxGen`);
mutants `StaleReportAccepted` (`AcceptsReport <- StaleReport`: a superseded stream's report frees a request) and
`ResetAllOnDisconnect` (`EndGiveBack <- ResetAll`: any stream's end gives back every request), both detected by S6;
witness `NoStaleReport`. Configurations `OISingleInstanceReconnect.cfg` (three launches, no crash, `MaxGen = 2`,
safety) and `OISingleInstanceReconnectLive.cfg` (two launches, `MaxGen = 2`, liveness included).

### 6.4 Configurations (to add at landing; TLC through `scripts/tla-check.sh`)

- `OISingleInstance.cfg`: `Launch = {p1, p2, p3}`, `None = None`, `MaxCrashes = 1`, `Hangs = FALSE`; invariants
  `TypeOK MutualExclusion IdentityNeedsLock OnlyHolderListens ExitOkImpliesQueued GaveUpJustified`; properties
  `Settles AckedApplied`; `CHECK_DEADLOCK FALSE`; sound, exhaustive.
- `OISingleInstanceHang.cfg`: the same with `Hangs = TRUE`; sound.
- `mutants/OISingleInstance<Mutant>.cfg` for the ten mutants: the sound constants (`MaxCrashes = 1`), the one
  override, and only the detecting property of table 6.3.
- `mutants/OISingleInstanceWitness{Handoff,GiveUp,Takeover,Handover}.cfg`: the sound constants (`Hangs = TRUE` for
  GiveUp), `INVARIANT` the witness; listed with the mutants (must violate), like `SP1TypingResidual`.
- Three launches: the smallest number with a primary and two concurrent seconds (two seconds racing on a stale
  socket is the SecondUnlinks counterexample). No symmetry reduction (unsound with liveness in TLC).
- A section « OISingleInstance (W41, single instance) » in `docs/protocol/tla/README.md` with the result table. The
  model lives in `docs/protocol/tla/` although the hand-off is a local interface and not a peer protocol: that is the
  directory `mise run tla`, `formal` and `done` check (`scripts/tla-check.sh`), and `CLAUDE.md` step 1 names it for
  every model.

## 7. Decisions, and alternatives rejected

| Id | Decision | Rejected, and why |
|---|---|---|
| D-OWN | own `internal/instance`, not Wails `SingleInstance` | Wails: §3 (W-1..W-8) |
| D-KERNEL | `flock` / `LockFileEx`, released by the OS at death, explicitly at the end of a clean exit | pid file, `O_EXCL` lock file, Chromium's `hostname-pid` symlink: staleness heuristics, pid reuse, a « profile locked » question after a crash (mutant CrashKeepsLock) |
| D-UDS | Unix-domain socket in the profile folder, runtime-dir fallback | loopback TCP with a token: a port, a token to store and compare, the invariant's heavier path, for nothing a same-user socket lacks; D-Bus name: per session, not per profile, absent without a bus (W-1, W-2); named pipe: Windows-only, global namespace, squatting needs a DACL and `FILE_FLAG_FIRST_PIPE_INSTANCE` (kept as the Windows fallback) |
| D-ADR | ADR 0037 amends the `CLAUDE.md` invariant « autres serveurs locaux » | the invariant as written covers only `127.0.0.1` servers; a Unix-domain socket is a local server, authenticated by directory permissions and peer credentials instead of a token; it needs its ADR before the code |
| D-LOCK | the lock, not the socket, decides who is primary | « first to bind wins »: a stale socket file or a race makes two primaries (mutant NoLock) |
| D-ORDER | lock and listen before `OpenHome` | today's order (`main.go:49` before anything): the second process would have opened the identity (mutant EarlyIdentity) |
| D-ACK | acknowledge after queueing; exit 0 only on the acknowledgement | fire and forget (Wails macOS): silent loss (mutant FireAndForget) |
| D-CLOSING | a closing primary answers « closing »; the launch waits and becomes the next primary | refusing during shutdown loses the launch (mutant GiveUpOnClosing) |
| D-RESOLVE-HERE | the primary resolves the forwarded arguments against the forwarded working directory | resolving in the second process: it would read the filesystem and the project layout the primary owns; resolving against the primary's own working directory: wrong files (test TestResolveRelativeToForwardedCwd) |
| D-SYMLINK | a symlink argument is resolved, its target classified | the study's « refuse symlinks »: same-user input, no privilege gained, common setups broken |
| D-QUEUE-FIRST | the primary's own arguments go through the same queue as forwarded ones | two code paths for the same rows |
| D-DRAIN | the window keeps draining the queue while it serves, requests queued while it was opening included | one-shot events per request: one emitted before the webview listens is lost after its acknowledgement (mutant NoDrain) |
| D-FAILCLOSED | no lock possible → do not start (R18) | starting anyway reopens the two-processes-one-key defect |
| D-NOTICE | Go sends a structured notice; the webview writes the text | Go strings outside the i18n catalogue |

## 8. Failure modes

| # | Failure | Behaviour | Proof |
|---|---|---|---|
| F1 | two launches at once on a free profile | the kernel gives the lock to one; the other connects once it listens | model L1; TestHandoffConcurrentLaunches |
| F2 | the primary crashes holding the lock | the kernel releases it; the next try of a waiting launch becomes primary | model NoTakeover, CrashKeepsLock; TestClaimReleasedByCrash |
| F3 | the primary crashes between acknowledgement and display | accepted residual since the simplified contract (2026-11-12): a request is acknowledged once received and queued, so a crash before the window shows it loses it (the launch has exited 0); the person launches again | model L2 `AckedApplied` (a crashed primary is its escape) |
| F4 | stale socket file after a crash | removed by the next lock holder only | SecondUnlinks; TestHandoffStaleSocket |
| F5 | the primary is hung | the second gives up after 5 s with R11, no second copy | model GiveUpJustified, NoGiveUp; TestHandoffUnresponsive |
| F6 | launch during shutdown | waits ≤ 15 s, becomes primary | model NoHandover; TestHandoffDuringClosing |
| F7 | profile path too long for `sun_path`, or a filesystem without sockets | runtime-dir candidate | TestSocketPathFallback, TestSocketBindFallback |
| F8 | another user, or a non-private runtime dir | refused by peer credentials; an unsafe runtime dir is skipped | TestHandoffPeerCheck, TestSocketPathFallback |
| F9 | malformed, oversized or truncated request; slow client | `malformed` or timeout, connection closed, handler not called, no goroutine left | TestHandoffRejectsMalformed, TestHandoffSlowClient, FuzzDecodeRequest |
| F10 | burst of 15 launches (Explorer) | 16 connections served, queue 64, then `busy` and retry | TestHandoffConcurrentLaunches, TestHandoffBusy |
| F11 | retried request after a lost acknowledgement | shown once (ID) | TestHandoffDuplicateIgnored |
| F12 | the lock fd inherited by `typst`/`tinymist` (would outlive Inkwell) | close-on-exec | TestClaimNotInheritedByChild |
| F13 | the `*Primary` collected by the GC | kept alive by `run` | TestPrimaryHeldUntilExit |
| F14 | old primary, new second (upgrade while running) | `unsupported` → R17 | TestRequestVectors (version byte), TestHandoffOtherVersion |
| F15 | a pre-OI Inkwell (no lock) still running when the new one starts | the new one becomes primary: two processes once, during that upgrade only | residual, §10 |
| F16 | a different profile with the same Docs folder (`INKWELL_HOME` only) | two identities on one project folder: out of this block (project lease, OI3) | residual, §10 |
| F17 | Wayland focus-stealing prevention | the forwarded activation token is applied before present | TestSecondLaunchForwardsArgsAndCwd (token forwarded); ux-walkthrough on GNOME |
| F18 | profile on NFS without lockd | R18, fail closed | TestClaimNoLock (injected `ENOLCK`) |

## 9. Test plan (named tests; the gate requires each to show `--- PASS`)

`internal/launch` (decision table, one test per row family; `t.TempDir()` trees: Docs with projects, a nested
project, a project outside Docs, a loose file, a `$HOME` stand-in, a mount boundary simulated by an injected device
function, a symlink, a fifo, a missing path):
TestResolveNoArgs (R1, R16), TestResolveFileInCurrentProject (R3), TestResolveCurrentProjectFolder (R4),
TestResolveFileInOtherProject (R5), TestResolveProjectOutsideDocs (R5), TestResolveLooseFile (R6),
TestResolvePlainFolder (R7: also `$HOME`, `/`), TestResolveSeveralPaths (R8), TestResolveRefused (R9, every reason,
32 and 33 arguments, 4096 and 4097 bytes), TestResolveRelativeToForwardedCwd, TestResolveSymlinkResolved,
TestResolveWalkStops ($HOME, root, device change, 64 levels), TestResolveStartupPicksProject (R2, R1 fallback),
TestPlanOneToastPerRequest (composition, `Others`), TestPlanOutsideProjectNotice (R5, R6: the interim notice names the file, nothing is shown; OI3 replaces this test), TestResolveProperty (seeds 1-10, 500 random trees and argument
lists each: total, deterministic, every `Show` inside its project's `os.Root`, never a panic).

`internal/instance` (real processes by re-executing the test binary; durations shortened through `Options`):
TestClaimFirstIsPrimary, TestClaimSecondIsNotPrimary, TestClaimSeparateProfiles (R14),
TestClaimSameProfileThroughSymlink, TestClaimReleasedByCrash (R13: child `SIGKILL`ed, next claim primary within 1 s),
TestClaimNotInheritedByChild (F12), TestClaimNoLock (R18), TestHandoffDelivers (R10), TestHandoffBeforeListen,
TestHandoffStaleSocket (F4), TestHandoffDuringClosing (R12), TestHandoffUnresponsive (R11: `ErrNotResponding` after
the deadline, not before, within 500 ms after), TestHandoffConcurrentLaunches (15 child processes at once, 10 rounds:
exactly one primary, 14 requests delivered once each), TestHandoffDuplicateIgnored, TestHandoffBusy,
TestHandoffRejectsMalformed, TestHandoffSlowClient (no goroutine left: `runtime.NumGoroutine` back to its start
value), TestHandoffPeerCheck (injected credential source), TestHandoffOtherVersion (R17), TestHandoffListensUnixOnly
(`Addr().Network() == "unix"`), TestSocketPathFallback, TestSocketBindFallback, TestSocketPathVectors and
TestRequestVectors (`testdata/handoff-v1.json`: frames in hex with the decoded request or the status, and profile
paths with their candidate socket paths; pinned so that two versions keep talking), TestBackoffSchedule; fuzz target
FuzzDecodeRequest (no panic; accepted ⇒ bounds hold and `Encode(Decode(b)) == b`).

`cmd/inkwell`: TestVersionDoesNotClaim (R15), TestSecondLaunchTouchesNothing (R10: a primary child, then a second
child with a file; the second exits 0 within 5 s; the profile and Docs folders hold exactly what they held before;
no identity file created), TestSecondLaunchForwardsArgsAndCwd (arguments verbatim, the second's working directory,
the activation token from `XDG_ACTIVATION_TOKEN` then `DESKTOP_STARTUP_ID`), TestUnresponsiveTellsPerson (R11: the
notifier gets the texts of §2, exit code 3), TestPrimaryClaimsBeforeIdentity (recorded order: claim, serve, home,
project, network), TestPrimaryHeldUntilExit (F13: repeated `runtime.GC`, the lock still held),
TestShutdownStopsHandoffFirst (`Closing` before the network and project close, `Close` after them),
TestOpenQueueBounded.

Webview (Vitest, `frontend/src/workspace/openRequests.test.ts`): « OI open request shows a file of the current
project », « OI open request with no file only raises », « OI open request shows one toast with the catalogue text »,
« OI open request before the workspace is ready waits ».

End to end (Playwright, `frontend/e2e/openWith.spec.ts`; `startInstance` gains `args` and `home`): « OI second launch
shows a file of the current project in the running window » (R3, R10, the second exits 0 within 5 s), « OI second
launch with no file changes nothing » (R16), « OI second launch with a loose file shows the toast » (R6), « OI first
launch with a file opens its project » (R2), « OI another profile runs separately » (R14).

Packaging: `mise run pkg:lint` with the `Exec` and `MimeType` checks, the `*.tab`/`*.psv` globs, and the `selftest` refusals of an entry without `%F` and of one listing `text/plain` (§4.5).

Code mutants that the tests must kill (gremlins finds most; these are checked by hand during step 2): lock attempt
removed (TestClaimSecondIsNotPrimary); unlink before taking the lock (TestHandoffStaleSocket with a racing child);
acknowledgement before `handle` (TestHandoffDelivers counts handler calls before reading the status); relative path
against the primary's cwd (TestResolveRelativeToForwardedCwd); symlink not resolved (TestResolveSymlinkResolved);
walk crossing `$HOME` (TestResolveWalkStops); peer check skipped (TestHandoffPeerCheck); `Closing` after the network
close (TestShutdownStopsHandoffFirst); lock file opened without close-on-exec (TestClaimNotInheritedByChild).
Mutation (gremlins, `mise run mutate:go`): ≥ 95 % on `internal/launch` and on `internal/instance`; every duration and
back-off step is a pure function with a table test, so that no arithmetic mutant is equivalent by timing.

Review before the gate (rigour complete): one independent review of this plan and ADR 0037 by another model family
(`scripts/claude-or.sh` or Codex), recorded in `gates/OI/REVIEW.md`; `ux-walkthrough` with light and dark captures of
R3, R5 and R6 and the SEQ of the short session with the user in `gates/OI/UX.md`.

## 10. Feasibility on the pinned stack (every MUST)

| MUST | Where it is possible |
|---|---|
| exclusive kernel lock, released at death | `syscall.Flock` already used, `internal/app/snapshot_session_unix.go:48`; `LockFileEx`, `x/sys@v0.49.0/windows/zsyscall_windows.go:2923` |
| lock fd not inherited by children | Go 1.27.3 `os/file_unix.go:261`, `os/file_windows.go:162`, `syscall/syscall_windows.go:397` |
| AF_UNIX on Linux, macOS, Windows | `net/unixsock_posix.go:5` (`unix \|\| … \|\| windows`); Windows caveat `net/unixsock_test.go:486` |
| peer credentials | `x/sys@v0.49.0/unix/syscall_linux.go:1283`, `unix/syscall_darwin.go:489`, `zerrors_darwin_arm64.go:935,1379`; Windows `SIO_AF_UNIX_GETPEERPID` [W6][W7] |
| open files through a root | `os.OpenRoot` (used at `internal/app/snapshot_session_unix.go`, `openLease`) |
| raise with the activation token | GTK 4.22.5 `gtkwindow.h:101`; Wails `webview_window.go:1664`, `webview_window_linux.go:474-476`, `mainthread.go:23`, `webview_window.go:1331,1692` |
| no double intake by Wails | `application_linux.go:66-85` fires only with `FileAssociations` set: left unset |
| macOS files from Finder (N35) | `application_darwin_delegate.m:14`, `application_darwin.go:813-825` |
| notifications | `godbus/dbus/v5 v5.2.4` (`go.mod:42`); `windows.MessageBox`, `zsyscall_windows.go:4380` |
| show a file in the webview | `Workspace.openFile`, `frontend/src/workspace/workspace.svelte.ts:132`; toast `frontend/src/ui/Toast.svelte` |
| `%F` and MIME | desktop entry spec [W11]; `freedesktop.org.xml:38814,35360`; `desktop-file-validate` 0.28 accepts `%F` |

Residuals, accepted and graded: F3 (a crash of the primary between acknowledgement and display loses the request:
the simplified contract of 2026-11-12, ADR 0037 point 6), the close residual (a window that cannot report while it
closes: what is left after the 2 s wait is logged; a step that never ends holds its slot and the requests after it
until the window closes, after the « prend plus de temps que prévu » toast), F15 (one upgrade from a pre-OI version), F16 (two profiles on one
Docs folder: project lease at OI3), the Windows AF_UNIX reliability (decided on the N35 runner, named-pipe fallback
ready).

## 11. Decisions of the user (2026-11-11)

- **Q1 → like JetBrains IDEs.** Researched first: JetBrains opens a file that belongs to an open project in that
  project's window; a file outside any open project opens in LightEdit, a light single-file editor, after a dialog
  that asks whether to stay in LightEdit or open it in a project, with « open file in project » available later
  (choose or create the root) ([LightEdit](https://www.jetbrains.com/help/idea/lightedit-mode.html)). VS Code instead
  drops the file into the open window as a stray tab by default, which the user did not choose (it would make a loose
  file look shared). For Inkwell: a file of the open project opens in it (R4); a file outside it opens at once, with
  no dialog (the user: « et le moins de clic »), in a light single-file window whose header bar offers « Ajouter à un
  projet… »; nothing joins a shared project without that explicit action. This replaces the interim toasts
  of R5/R6 and needs the light window. **Settled at step 1 (2026-11-12):** the light window is block OI3, the direct
  successor of this block; rows R5/R6 carry both the interim of OI and the decided behaviour; reasons in §2.1.
- **Q2 → no.** Listed only for the formats Inkwell handles; not `text/plain`. The user added (2026-11-11) « CSV, TSV ou tout autre separated value »: the editor already reads them (`frontend/src/editor/csv/dialect.ts`: `.csv` with the delimiter detected, `.tsv`/`.tab` tab, `.psv` pipe), so the desktop entry lists `text/csv`, `text/tab-separated-values`, and a shared-mime-info package declares the globs `*.tab` and `*.psv` (no registered type), next to Typst and Markdown. Separately the
  user asked that other languages be highlighted correctly when they are opened (N20, block LF0c).
- **Q3 → open (asked 2026-11-12).** The interim toast of R5/R6 lives for the one block between OI and OI3. If the
  user prefers that no release carries the toast, OI3 is built before OI is released (still two blocks, one release);
  the plan and gate of OI do not change.

## 12. Changes to other documents (done at step 7)

ADR 0037 (single instance; amends the local-server invariant, D-ADR) and the matching line of `CLAUDE.md`;
`docs/architecture.md` (two packages, their imports); `docs/protocol/tla/README.md` (§6.4); `.env.reference`
(`XDG_RUNTIME_DIR`, `XDG_ACTIVATION_TOKEN`, `DESKTOP_STARTUP_ID`, `LC_ALL`, `LC_MESSAGES`, `LANG`); `docs/exigences.md`
W41 (state, and OI3 as the successor block); `docs/architecture/coherence.md` §5 and `docs/objectif-en-cours.md` (OI3
right after OI); `PROGRESS.md`; `docs/objectif-en-cours.md`; the study `docs/plans/2026-11-09-os-integration.md` (§3 symlink rule,
§4 « Wails SingleInstance » line, pointing here).

## 13. Amendment (2026-11-13): the activation token reaches GTK

**Defect** (installed rpm, GNOME on Wayland, Fedora 45; W41). A second launch from Files or Activities hands its file
over and exits 0 in 0.07 s, the running window opens the file, but (1) the window is not raised, (2) GNOME shows
« « Inkwell » est prêt », (3) clicking that notification does nothing, (4) the busy cursor of `StartupNotify=true`
spins until GNOME gives up.

**Root cause, measured.** The token was read and forwarded (rows R10, F17; `TestSecondLaunchForwardsArgsAndCwd`,
`TestOpenerServe`) and then dropped at the last step: `raiseWindow(w, _ string)` called Wails' `Show` and `Focus`,
which are `gtk_window_present` (`wails/v3@v3.0.0/pkg/application/linux_cgo.go:1323,1384`). The cgo file this plan
specified in section 4.3 had been refused by `mise run arch` and replaced by a documented limitation
(`gates/OI/SIMPLIFY.md`, finding C4). Trace of the primary (`WAYLAND_DEBUG=1`, headless Mutter 51.1, second launch
with `XDG_ACTIVATION_TOKEN=oi-forwarded-token-42`, exit 0 in 0.05 s), before the fix:

```text
-> xdg_activation_v1#29.get_activation_token(new id xdg_activation_token_v1#89)
-> xdg_activation_token_v1#89.set_serial(0, wl_seat#3)
-> xdg_activation_v1#29.activate("1cd8685a-…_TIME0", wl_surface#10)      (and once more for Focus)
```

GTK, given no token, asks the compositor for one of its own (`gdk_wayland_toplevel_focus` [W14]); it has no input
serial (the person clicked in Files, not in Inkwell), so the compositor's answer cannot activate anything. What
follows is the desktop's rule, not a fault of it:

- A token goes from the launcher to the launched process in `XDG_ACTIVATION_TOKEN` (Wayland [W13]) or
  `DESKTOP_STARTUP_ID` (X11 [W16]), or in `platform_data` (`activation-token`, `desktop-startup-id`) for a D-Bus
  activated application [W17]; « the compositor can ignore unknown activation tokens » [W13].
- Mutter (`maybe_activate` [W15]) activates the window only for a token that can activate or a recent startup
  sequence; otherwise it calls `meta_window_set_demands_attention`, which marks a window that is minimised, on
  another workspace or covered by another window (Files, here), and it completes the startup sequence only for a
  request that names it: hence symptoms (1) and (4).
- GNOME Shell turns « demands attention » into the notification « “%s” is ready » with the window's title
  (`windowAttentionHandler.js` [W18]): symptom (2). Its click handler is `Main.activateWindow`; why it did nothing
  on 2026-11-13 is **not explained** by these sources and was not reproduced (no Shell in the headless check); with
  the fix a launch from the desktop no longer shows it.
- The same defect and the same fix are documented for GTK's own single instance on Plasma: the application « must
  use the token provided … through the XDG_ACTIVATION_TOKEN environment variable or the platform_data argument »,
  not one it requests itself (GTK issue 5483 [W19]).

**Prior art: everybody forwards the launcher's token to the process that owns the window.** GApplication sends it in
`platform_data` and GtkApplication gives it to GDK before the application presents its window
(`gtk_application_impl_wayland_before_emit` [W20]); Chromium carries it to the running browser as an internal
switch, `xdg-activation-token` (`base/nix/xdg_util.h` [W21]), over the same socket as the command line [W3];
Firefox reads `XDG_ACTIVATION_TOKEN` since 108 (bug 1767546 [W22]); emacsclient forwards the startup id [W10].
Applications that do not are reported for exactly this defect (Electron: Obsidian [W23]). Wails' single instance
sends one string over D-Bus and has no token (section 3); Tauri's plugin, D-Bus too, documents only `set_focus()`
[W24].

**Decision.** Keep the hand-off of ADR 0037 (the token already travels in the request, section 4.1) and finish it
as section 4.3 specified: on the GTK main thread the primary calls `gtk_window_set_startup_id(window, token)` and
then `gtk_window_present(window)` [W9]. On a mapped window GTK passes the token at once to
`gdk_toplevel_set_startup_id`, which is `xdg_activation_v1.activate(token, surface)` on Wayland [W14]; on X11 it
takes the launch's time from the id and ends the startup sequence, and `present` raises the window. The call lives
in a new package, `internal/activate` (one role: present a window as the launch that asked for it; one function,
`Present(window, token)`), the only package besides `internal/sidecar` that may import `C`: `tools/archexec` lets
it import `C` and nothing else (no `os/exec`, no process start, no raw syscall, no native source file, no
hand-written `go:linkname`), which is what requirement N42 protects (no program to install; platform libraries
are allowed). `cmd/inkwell` gives it Wails' `NativeWindow` through `raiser` (`raise.go`), whose three effects are
injected. On Linux `Restore`, `Show` and `Focus` are no longer called: the activation un-minimises, and Wails'
`Restore` would also un-maximise a maximised window (`webview_window.go:1331`).

After the fix, same trace: the forwarded token first, then GTK's own request from `present` and the token again
(`gtk_window_notify_startup`). The two extra requests are harmless on Mutter: a completed or unknown sequence does
nothing, and a window in full view is not marked [W15].

```text
-> xdg_activation_v1#29.activate("oi-forwarded-token-42", wl_surface#10)
-> xdg_activation_v1#29.get_activation_token(…) … activate("6b0baae3-…_TIME0", wl_surface#10)
-> xdg_activation_v1#29.activate("oi-forwarded-token-42", wl_surface#10)
```

**Alternatives rejected.**

| Alternative | Why not |
|---|---|
| `DBusActivatable=true` and `org.freedesktop.Application` [W17] | GNOME would pass the token in `platform_data`, but it must still be given to the GtkWindow by the same C call: Wails creates its GtkApplication `G_APPLICATION_NON_UNIQUE` (`linux_cgo.h:22`), so GTK's own path [W20] never runs. It adds a session-bus name as a second single-instance key, which ADR 0037 point 7 refused (two sessions of one user, no start without a bus), and a service file to package |
| Wait for a Wails API (decision of 2026-11-12) | right place in the long run, but the shipped package stays broken for an unknown time; kept as the exit: when Wails wraps the call, `internal/activate` is deleted |
| purego / `dlopen` of libgtk-4 | obeys the letter of the cgo rule and not its intent, adds a dependency, loses the header's type check; Wails already needs cgo and the GTK 4 headers |
| `gdk_wayland_display_set_startup_notification_id` then `present` (what GtkApplication does) | Wayland only, and deprecated (GTK itself calls it between `G_GNUC_BEGIN_IGNORE_DEPRECATIONS` guards [W20]) |
| Skip `present` once the token was given to a mapped window | saves two redundant requests on Wayland, but X11 needs `present` to raise; one sequence for both |
| Let GTK request its own token (the defect) | refused by design [W19] |

**Proof (red first).**

- `TestSecondLaunchTokenReachesPresenter` (`cmd/inkwell/raise_test.go`): a real second launch with
  `XDG_ACTIVATION_TOKEN` → primary → open queue → opener → `raiser` → the presenter gets the native window and that
  token; a launch without a token presents with none; a destroyed window is not presented. Red with the token
  dropped at the presenter (`presentation 0 = {… }, want {… gnome-shell/Files/1234-5-host_TIME678}`).
  `TestRaiseDoesNotWait`: the raise is handed to the main thread and returns.
- `mise run app:activation` (`scripts/activation-check.sh`): the real desktop binary under a headless Mutter, a
  private session bus and a scratch profile; passes only if the primary's first activation request after the second
  launch carries the forwarded token. Red on the binary of 2026-11-13 23:08, green on the fix (traces above).
- `TestToolkitPackageImportsCOnly`, `TestSyntaxAcrossBuildTags` (`tools/archexec`): `internal/activate` may import
  `C`; `os/exec`, `os.StartProcess` and a hand-written `go:linkname` are still reported there.

**Residuals.** The headless Mutter never issued the test's token, so what the compositor does with a real one
(raise, end of the busy cursor) is Mutter's code [W15] and is confirmed on the person's session (2-minute script
in `gates/OI/UX.md`). A launch with no token (from a terminal) still gets « Inkwell est prêt » when another window
covers Inkwell: that is GNOME's focus-stealing prevention. Windows and macOS keep `Restore`, `Show`, `Focus` (N35,
section 4.6), un-maximise included.

## Sources

- [W1] Raymond Chen, The Old New Thing, 2015-07-15 (verb selection model limits: « at most 15 new processes starting at once ») —
  https://devblogs.microsoft.com/oldnewthing/20150715-00/?p=45141 ; verb selection model —
  https://learn.microsoft.com/en-us/windows/win32/shell/how-to-employ-the-verb-selection-model
- [W2] Microsoft, Window features, « Message-Only Windows » —
  https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features
- [W3] Chromium `chrome/browser/process_singleton_posix.cc` —
  https://chromium.googlesource.com/chromium/src/+/main/chrome/browser/process_singleton_posix.cc
- [W4] XDG Base Directory Specification, `XDG_RUNTIME_DIR` — https://specifications.freedesktop.org/basedir-spec/latest/
- [W5] unix(7), `sun_path` — https://man7.org/linux/man-pages/man7/unix.7.html
- [W6] mingw-w64 `afunix.h` (`UNIX_PATH_MAX`, `SIO_AF_UNIX_GETPEERPID`) —
  https://mingw.googlesource.com/mingw-w64/+/refs/heads/v14.x/mingw-w64-headers/include/afunix.h
- [W7] GLib, « gio: Add some AF_UNIX support on Windows » (peer pid as credentials) —
  https://gitlab.gnome.org/GNOME/glib/-/merge_requests/2445 ; zbus `win32.rs` —
  https://docs.rs/crate/zbus/3.9.0/source/src/win32.rs
- [W8] Nielsen Norman Group, « Response Times: The 3 Important Limits » —
  https://www.nngroup.com/articles/response-times-3-important-limits/
- [W9] GTK 4, `gtk_window_set_startup_id` — https://docs.gtk.org/gtk4/method.Window.set_startup_id.html ;
  GLib, XDG activation in `GAppLaunchContext` (2.76) — https://gitlab.gnome.org/GNOME/glib/-/issues/2709
- [W10] GNU bug 49504 (emacsclient forwarding `DESKTOP_STARTUP_ID`) — https://debbugs.gnu.org/db/49/49504.html
- [W11] Desktop Entry Specification, « The Exec key » —
  https://specifications.freedesktop.org/desktop-entry/latest/exec-variables.html
- [W12] Microsoft, `LockFileEx` — https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
- [W13] Wayland, `xdg-activation-v1` (token in `XDG_ACTIVATION_TOKEN`; unknown tokens may be ignored) —
  https://wayland.app/protocols/xdg-activation-v1
- [W14] GTK 4 source: `gtk_window_set_startup_id`, `_gtk_window_present`, `gtk_window_notify_startup` —
  https://gitlab.gnome.org/GNOME/gtk/-/blob/main/gtk/gtkwindow.c ; `gdk_wayland_toplevel_set_startup_id`,
  `gdk_wayland_toplevel_focus` — https://gitlab.gnome.org/GNOME/gtk/-/blob/main/gdk/wayland/gdktoplevel-wayland.c
- [W15] Mutter source: `activation_activate`, `maybe_activate`, `token_can_activate` —
  https://gitlab.gnome.org/GNOME/mutter/-/blob/main/src/wayland/meta-wayland-activation.c ;
  `meta_window_set_demands_attention` — https://gitlab.gnome.org/GNOME/mutter/-/blob/main/src/core/window.c
- [W16] Startup Notification specification (`DESKTOP_STARTUP_ID`, the launchee ends the sequence) —
  https://specifications.freedesktop.org/startup-notification/latest/
- [W17] Desktop Entry Specification, « D-Bus Activation » (`org.freedesktop.Application`, `platform_data` keys
  `desktop-startup-id` and `activation-token`) — https://specifications.freedesktop.org/desktop-entry/latest/dbus.html
- [W18] GNOME Shell, `js/ui/windowAttentionHandler.js` (« “%s” is ready », `Main.activateWindow` on click) —
  https://gitlab.gnome.org/GNOME/gnome-shell/-/blob/main/js/ui/windowAttentionHandler.js
- [W19] GTK issue 5483, « Single-instance window activation doesn't work on Plasma Wayland » —
  https://gitlab.gnome.org/GNOME/gtk/-/issues/5483
- [W20] GTK 4 source, `gtk_application_impl_wayland_before_emit` —
  https://gitlab.gnome.org/GNOME/gtk/-/blob/main/gtk/gtkapplication-wayland.c
- [W21] Chromium `base/nix/xdg_util.h` (switch `xdg-activation-token`) —
  https://chromium.googlesource.com/chromium/src/+/HEAD/base/nix/xdg_util.h
- [W22] Mozilla bug 1767546, « [Wayland] Support XDG_ACTIVATION_TOKEN env var » —
  https://bugzilla.mozilla.org/show_bug.cgi?id=1767546
- [W23] Obsidian forum, « Linux/Wayland: launching Obsidian while it is running does not focus the existing window » —
  https://forum.obsidian.md/t/linux-wayland-launching-obsidian-while-it-is-running-does-not-focus-the-existing-window-an-obsidian-link-does/118880
- [W24] Tauri, Single Instance plugin — https://v2.tauri.app/plugin/single-instance/
- Wails v3.0.0 module source (`~/go/pkg/mod/github.com/wailsapp/wails/v3@v3.0.0`), files and lines as cited in §3.

**Amendment of 2026-11-14 (user decision of 2026-11-13: no cgo exception).** `internal/activate` reaches GTK through
purego (`github.com/ebitengine/purego` v0.11.2, Apache-2.0, the library Ebitengine uses to call C without cgo:
https://github.com/ebitengine/purego) instead of `import "C"`: `Dlopen("libgtk-4.so.1")` with `RTLD_NOLOAD`, so only the
GTK the process already loaded answers and no second copy can enter it (dlopen(3)), then `Dlsym` + `RegisterFunc` for
`gtk_window_set_startup_id` and `gtk_window_present`. `tools/archexec` is back to its rule without exception: no package
but `internal/sidecar` imports `C`. What the header's type check gave is replaced by proofs: `TestGTKCallsResolveOnlyInLoadedGTK`
(the two names exist in GTK 4, a process without GTK is refused), `TestSecondLaunchTokenReachesPresenter`, and
`mise run app:activation` on the real binary (now built first: a stale binary of the day before was once judged), which
shows `xdg_activation_v1.activate("<forwarded token>")`. If GTK cannot be reached `Present` returns an error and the
window is shown with Wails' own calls, as before. Text above that speaks of the `C` import is superseded by this.
