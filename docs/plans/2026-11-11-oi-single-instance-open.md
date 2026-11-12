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
| R8 | Several arguments | classified one by one in argument order; the first file the current project shows is shown (one document at a time until the tabs of G-0c); the toast names the first argument not shown and adds « Et {n} autre(s). » for the rest | D9 |
| R9 | Missing, unreadable, not a regular file or folder (fifo, socket, device), inside `.inkwell/`, path over 4096 bytes, more than 32 arguments | the toast of the first such argument: « « {nom} » est introuvable. », « Inkwell n'a pas le droit de lire « {nom} ». », « « {nom} » n'est pas un fichier ordinaire. », « « {nom} » est un fichier interne d'Inkwell. », « Ce chemin est trop long. », « {n} fichiers non ouverts : 32 au plus à la fois. » | D11 |
| R10 | Second launch while a primary runs on this profile | the second process forwards its arguments, working directory and activation token, waits for the acknowledgement, exits 0; it opens no identity, project, network, window or GTK; the primary applies R3-R9 with the forwarded working directory | D10 |
| R11 | Second launch; the primary holds the lock but gives no answer within 5 s | desktop notification « Inkwell ne répond pas » / « Inkwell est déjà ouvert avec ce profil mais ne répond pas. Fermez-le puis réessayez. »; exit code 3; no second copy | new |
| R12 | Second launch while the primary is closing | the second waits for the lock (at most 15 s), becomes the primary itself and applies R1-R9 to its own arguments | new |
| R13 | The primary crashed | the next launch is primary at once: the kernel released the lock; a leftover socket file is removed by the new primary; no question, no delay | new |
| R14 | Another `INKWELL_HOME` | an independent instance (its own lock and socket) | new |
| R15 | `inkwell --version` | prints the version; takes no lock (`cmd/inkwell/cli.go:8-10`) | D12 |
| R16 | Second launch with no argument | window raised; no toast | new |
| R17 | Second launch while a primary of another Inkwell version runs | the primary answers « unsupported »; notification « Une autre version d'Inkwell est ouverte. Fermez-la puis réessayez. »; exit code 3 | new |
| R18 | The profile folder supports no lock (`ENOLCK`, `EOPNOTSUPP`; NFS without lockd) | Inkwell does not start; notification « Inkwell ne peut pas s'assurer d'être seul à utiliser son profil ; il ne démarre pas. »; exit code 4 | new |

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
`Restore` and `Focus`; a small cgo file `raise_linux.go` (`//go:build linux && !server`), no-op in the server build.
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
| S4 `ExitOkImpliesQueued` | a launch that exits 0 was queued by a primary | invariant |
| S5 `GaveUpJustified` | a launch gives up only on a hung holder | invariant |
| L1 `Settles` | every launch eventually serves, hands off, gives up on a hung holder, or crashed | liveness |
| L2 `AckedApplied` | a queued request is applied unless its primary stopped serving | liveness |

### 6.3 Mutants (each one rule of §4.1 broken; each must be detected)

| Mutant | Override | Broken rule | Detected by |
|---|---|---|---|
| NoLock | `MayLead <- NoLockLead` | primary = « no socket answers » (the socket-only design) | S1 |
| EarlyIdentity | `IdentityAtStart <- EarlyIdentity` | identity opened before the lock (today's order) | S1 |
| AckNoQueue | `Enqueue <- AckNoQueue` | acknowledged before queueing | S4 |
| FireAndForget | `WaitsForReply <- FireAndForget` | exit 0 after sending, no acknowledgement (Wails macOS, W-4) | S4 |
| LockReleasedEarly | `ReleaseOnClose <- ReleaseEarly` | lock released when closing starts | S1 |
| CrashKeepsLock | `ReleaseOnCrash <- KeepLockOnCrash` | pid or `O_EXCL` file left by a crash | L1 |
| GiveUpOnClosing | `GiveUpOnClosing <- GiveUpClosing` | « closing » read as « already running » | S5 |
| SecondUnlinks | `UnlinksStale <- SecondUnlinksStale` | a second launch removes a « stale » socket (it may be the new primary's) | L1 |
| NoRetryLock | `AfterBackoff <- DialAgain` | retries reconnect without retrying the lock | L1 |
| NoDrain | `ToApply <- NoDrain` | requests acknowledged and queued, but the window never reads the queue (rule D-DRAIN) | L2 |

Witnesses (anti-vacuity; each invariant must be **violated** by the sound model): `NoHandoff` (some launch exits 0),
`NoGiveUp` (Hang configuration: some launch gives up), `NoTakeover` (a launch serves after a primary crashed),
`NoHandover` (a launch serves after a primary closed normally).

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
| F3 | the primary crashes between acknowledgement and display | the request is lost with the window; the person sees Inkwell gone and relaunches | model L2 (accepted residual, §10) |
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

Residuals, accepted and graded: F3 (loss with a crash between acknowledgement and display; window: one webview
message), F15 (one upgrade from a pre-OI version), F16 (two profiles on one Docs folder: project lease at OI3), the
Windows AF_UNIX reliability (decided on the N35 runner, named-pipe fallback ready).

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
- Wails v3.0.0 module source (`~/go/pkg/mod/github.com/wailsapp/wails/v3@v3.0.0`), files and lines as cited in §3.
