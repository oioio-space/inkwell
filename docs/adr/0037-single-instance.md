---
status: accepted
date: 2026-11-12
amends: CLAUDE.md « Invariants » (local servers); supersedes the « Wails SingleInstance » line of docs/plans/2026-11-09-os-integration.md section 4
decision-makers: [Mathieu]
---

# One Inkwell per profile: a kernel lock and a private local socket, not Wails' single instance

Plan: `docs/plans/2026-11-11-oi-single-instance-open.md` (decision table R1-R18, sections 3, 4.1, 6, 7).
Formal model: `docs/protocol/tla/OISingleInstance.tla`. Gate: `gates/OI.toml`. Requirement: W41.

## Context and problem

Nothing stops two Inkwell processes from running on one profile folder (`INKWELL_HOME`, default under the user's
config directory). Both open the same identity key, the same projects and the same network identity: the defect is
a correctness and security fault, not a convenience. It also blocks W41: « Ouvrir avec Inkwell » on a second file
starts a second process, so the open request has no running window to go to.

Inkwell needs, per profile: (1) at most one process holds the identity, whatever the timing of launches and crashes;
(2) a second launch hands its arguments, working directory and activation token to the first, and learns that they
were taken; (3) a stuck or closing first process never loses the request silently and never leaves a second copy.

Two decisions cannot be left to the code: Wails v3.0.0 ships a single-instance option that does not meet this
(evidence in the plan, section 3), and a Unix-domain socket is a local server, which the `CLAUDE.md` invariant on
local servers does not cover as written.

## Decision drivers

- One key per **profile**, not per desktop session or per application name.
- A crash must never leave a lock to clean up by hand, and no staleness heuristic (pid, hostname) may decide who is
  primary.
- The second launch must be acknowledged after the request is queued, and a failure must be reported, never
  swallowed as success.
- The second process must touch nothing: no identity, project, network, window or GTK.
- Same code on Linux, macOS and Windows; testable with real processes in `go test`, in the server build too.
- No new port, no stored secret (invariant: bridge = Wails streams, local servers are authenticated and private).

## Considered options

- A: Wails `Options.SingleInstance`.
- B: a pid file or an `O_EXCL` lock file, plus a socket (the Chromium and Electron approach).
- C: first to bind a socket wins (no lock).
- D: a kernel lock on a file in the profile folder, plus a Unix-domain socket beside it (Windows: AF_UNIX, a named
  pipe as the fallback).
- E: loopback TCP with a token, or a D-Bus name.

## Decision

Option retained: **D**, because it is the only one that decides « who is primary » without a heuristic, per profile,
with an acknowledged hand-off, on all three systems.

1. **Lock.** `<profile>/instance.lock`, opened through `os.OpenRoot`, mode 0600, close-on-exec (so `typst` and
   `tinymist` never inherit it). The primary is whoever takes the exclusive non-blocking lock: `flock(LOCK_EX|LOCK_NB)`
   on Unix, `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY)` on Windows. The kernel releases it when
   the process dies, so a crash is recovered by the next launch with no question and no delay. The lock is released
   only as the very last step of a clean exit, after the identity, the project and the network are closed and the
   socket file removed. If the filesystem cannot lock (`ENOLCK`, `EOPNOTSUPP`), Inkwell **does not start** (exit 4,
   a notification): starting anyway reopens the two-processes-one-key defect.
2. **Order.** The claim comes first in `main`, before the identity, a project, the network or GTK. A launch that
   does not get the lock opens none of them.
3. **Hand-off socket.** The primary listens on a Unix-domain socket `<profile>/instance.sock`. If that path exceeds
   103 bytes (`sun_path`), or binding fails (network or FUSE profile folder), it falls back to a socket in a private
   runtime folder named from the hash of the canonical profile path: `$XDG_RUNTIME_DIR` when owned by the user with
   mode 0700, else `os.TempDir()/inkwell-<uid>` created 0700 and checked with `Lstat` (a directory, not a symlink,
   owned by the user); an unsafe folder is skipped. Both ends compute the candidates by the same rule, pinned by
   vectors. Only the lock holder ever binds or unlinks a socket file. On Windows the same package uses AF_UNIX; a
   named pipe with an explicit DACL and `FILE_FLAG_FIRST_PIPE_INSTANCE` is the fallback if the N35 runner finds
   AF_UNIX unreliable.
4. **Same-user check, no token.** Each end checks the other's credentials before reading or writing: `SO_PEERCRED`
   (Linux), `LOCAL_PEERCRED` (macOS), `SIO_AF_UNIX_GETPEERPID` then the process token's SID (Windows); the uid or SID
   must equal our own. Together with the private folder this refuses another user. A token would add nothing: any
   process of the same user can read the profile, including the identity key, and so is already trusted at that
   level. Nothing is logged but counts, never a path, never the activation token.
5. **Wire format v1.** One canonical request per connection, big-endian, bounded before allocation (at most 32
   arguments, 4096 bytes each, working directory 4096, activation token 1024 printable ASCII, frame at most 136280
   bytes), a one-byte reply (accepted, closing, busy, malformed, unsupported version). The decoder is fuzzed; vectors
   pin the bytes (`protocol/` rule: every byte from another process is verified).
6. **Hand-off protocol.** **Amended 2026-11-12 (contract simplified, supersedes the amendments in parentheses
   below):** the second launch exits 0 on `accepted`, sent once the primary has received and queued the request (a
   bounded queue that opens once the primary can show a file: `busy` before, or when full); the primary owns the
   outcome (its window shows the file or says why not, then reports it handled to Go, which keeps it and its slot
   until then); closing the window stops acknowledging and waits at most 2 s for those reports (what remains is
   logged: a window that cannot answer while closing); a crash in between loses the request (accepted residual).
   **Amended 2026-11-13 (one owner at a time, plan rule D-OWNER):** each connection of the window to the `open`
   stream is a generation; a new one supersedes the old one, which Go closes, and what the old one had not reported
   handled is sent again on the new one; a report is accepted only from the generation holding the request (others
   ignored, logged at debug level); the webview starts each connection afresh and never runs two requests at once.
   A step past its bound is said slow (« prend plus de temps que prévu »), never a failure, and keeps its slot until
   it ends; one that never ends holds the requests after it until the window closes (the close residual).
   The reports need no further authentication: they arrive only on the Wails stream between the application's own
   webview and Go, with no local port (ADR 0009), so their sender is our window; the generation guards against our
   own stale connections. (Earlier wording,
   superseded:) The second launch forwards the request and exits 0 **only** on `accepted`, sent once the
   window applied the request (amended 2026-11-12 after the cross-family review: « after it is queued » lost a
   request on a clean shutdown or a failed start; a pending request is now answered `closing` and its launch becomes
   the next primary; while the window has not applied it, the launch is held, then told `busy`, and waits up to 15 s
   for an answering primary). A request the window cannot apply, or does not answer within 10 s, is answered
   `failed`: the launch shows a notification and exits 5 (R19). A request handed over to a third launch after a
   takeover keeps its intended project (plan, rule D-FORWARD). Third amendment (design decision after a third
   review round, plan section 4.1): the deadline has one owner, Go, and starts once the webview reported ready;
   until then a launch is told `starting` (at most 60 s); an expired request is cancelled in the webview, never
   shown later; a primary that took over waits up to 500 ms for the requests its predecessor dropped before it
   chooses its project. `closing` (the primary is shutting down) makes it wait for the lock up to 15 s and become the
   next primary; `busy`, a refused connection or a missing socket make it back off and retry the lock; no answer
   within 5 s while the lock is held ends with the notification « Inkwell ne répond pas » and exit 3; an answer of
   another version ends with exit 3 and a notification.
7. **Wails' single instance is not used** (plan section 3, facts W-1 to W-8, read from `wails/v3@v3.0.0`):
   the key is one application id, not a profile, and on Linux its scope is a D-Bus **session** bus (two sessions of
   one user both become primary); without a session bus the application cannot start; a failed notification is
   logged and the process exits **0**, so the arguments are lost and the launcher sees success; on macOS the
   notification is fire-and-forget and visible to any process observing the distributed notification centre; on
   Windows the message is sent without a timeout, so a hung primary hangs every launch; the lock is taken inside
   `application.New`, after the identity and the project are already open (reordering `run` and registering every
   service late would be needed); the server build (used by the e2e suite) has no implementation; the encrypted
   payload option needs a key compiled into the binary. GTK stays `G_APPLICATION_NON_UNIQUE`. Wails still provides
   `Show`, `Restore`, `Focus`, `NativeWindow` and `InvokeSync` for raising the window, and, at N35, the macOS
   Apple Events. `Options.FileAssociations` stays unset (its event fires only for one argument with a listed
   extension and would double the intake).

### Amendment of the `CLAUDE.md` local-server invariant

The invariant as written reads (« Invariants »):

```
- Pont webview ↔ Go = Streams Wails (aucun port local, ADR 0009). Autres serveurs locaux :
  `127.0.0.1`, port éphémère, jeton comparé en temps constant, jamais logué.
```

It is replaced, line for line, by this wording (the French of the file; to be applied at step 7 of the block, once
`internal/instance` exists, with `mise run lint:claude-md`):

```
- Pont webview ↔ Go = Streams Wails (aucun port local, ADR 0009). Autres serveurs locaux : soit
  `127.0.0.1`, port éphémère, jeton comparé en temps constant, jamais logué ; soit, pour la remise
  d'une seconde exécution à l'instance du profil (`internal/instance`, ADR 0037), une prise Unix (tube
  nommé sous Windows) dans un dossier privé (0700), sans jeton, protégée par les droits du dossier et
  par l'utilisateur du pair, vérifié des deux côtés, jamais journalisée ; aucun autre canal local.
```

Nothing else in the invariants changes: the single source of truth for membership and the identity key is unchanged,
and the socket carries launch arguments only, never document content, keys or peer bytes.

### Amendment of 2026-11-13: one C call gives the activation token to GTK

The token forwarded by a second launch (point 2 of the context) must be given to the toolkit that owns the window,
or GNOME refuses the focus and only says « « Inkwell » est prêt » (defect of 2026-11-13, measured: plan section
13). Wails wraps no such call and creates its GtkApplication `G_APPLICATION_NON_UNIQUE`, so the call is ours:
`gtk_window_set_startup_id` then `gtk_window_present`, on the GTK main thread, in the new package
`internal/activate`. It is the only package besides `internal/sidecar` that may import `C`, and `C` is all it
gains: `tools/archexec` still reports there `os/exec`, a process start, a raw syscall, a native source file and a
hand-written `go:linkname` (requirement N42 is about programs to install, not platform libraries). Considered and
refused, with sources (plan section 13): `DBusActivatable` with `org.freedesktop.Application` (the same C call
would still be needed, and a session-bus name would become a second single-instance key, refused in point 7),
waiting for a Wails API (the exit: `internal/activate` is deleted the day Wails wraps the call), purego.

### Consequences

- Good: the defect is closed by construction (model properties S1, S2: no two processes on one identity) and crash
  recovery needs no cleanup; the same package serves Linux now, macOS and Windows at N35; real-process tests run in
  `go test`; the request is applied against the forwarded working directory by the primary, which owns the project
  layout.
- Good: failure is visible: stuck, other version, no lock support are three distinct notifications and exit codes,
  none of them silent.
- Bad: one more local server to maintain and review (new package `internal/instance`, cgo-free, standard library and
  `golang.org/x/sys` only), and an amendment of a global invariant.
- Bad: residual (accepted, 2026-11-12): a process of the same user can fill the queue (64) and the 16 connection
  slots, so that other launches get `busy` until their 15 s run out. The trust boundary is the user (decision 4): such
  a process can already read the identity key, kill Inkwell or edit its files, so a quota per process would protect
  nothing; the queue and the slots stay bounded, the person sees « Inkwell ne répond pas », nothing is lost silently.
- Bad: residual (graded, plan section 10): a pre-OI Inkwell still running when the new one starts is not locked, once;
  two profiles on one projects folder are not excluded here (project lease, a later block); Windows AF_UNIX
  reliability is decided on the N35 runner.
- Out of scope: what the window does with a file outside the open project (the light single-file window, block OI3)
  and switching the project of a running window; this ADR fixes only the process and the channel.

### Confirmation

The model `OISingleInstance.tla` (7 properties, 9 mutants, 4 witnesses, run by `mise run tla`); the named tests of
plan section 9 under `-race` with real processes (`TestClaim*`, `TestHandoff*`, `TestSocket*`, vectors,
`FuzzDecodeRequest`); gremlins at 95 % or more on `internal/instance` and `internal/launch`; the gate
`gates/OI.toml`; `lint:claude-md` for the amended invariant; an independent review by another model family recorded
in `gates/OI/REVIEW.md`. For the amendment of 2026-11-13: `TestSecondLaunchTokenReachesPresenter`,
`TestToolkitPackageImportsCOnly`, and the native check `mise run app:activation`.

## More information

Prior art and sources are listed in the plan (Chromium `ProcessSingleton`, Electron, VS Code, emacsclient for the
activation token; Raymond Chen on the 15-process limit of the verb selection model; unix(7) `sun_path`; the XDG
base directory specification for `XDG_RUNTIME_DIR`). Revisit if a supported platform cannot give a kernel lock with
fail-closed behaviour, if Windows AF_UNIX proves unreliable (named-pipe fallback), or if Wails ships a per-profile
single instance with an acknowledged hand-off and a bounded, authenticated payload.

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
