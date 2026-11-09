# Diagnostics study: logs, crashes, health signals, local report (2026-11-09)

Status: study only, nothing implemented. Written in English per CLAUDE.md. Scope: when someone's Inkwell crashes, degrades or misbehaves (network, sync, Typst, storage, UI), we must be able to see why, with no telemetry and no document data. Requirement to file: W44 (diagnostics), attached to
the lot "Premiere minute et secours" of `docs/architecture/coherence.md` section 5, with the network-health part feeding W33 and the failure-class proofs feeding W36.

Method note: the audit (section 1) is from the code at HEAD 05919a1 (file:line). The research (section 2) cites URLs from the author's knowledge of these projects; they were not re-fetched in this pass, so each is marked "verify" and must be re-read in the plan step (Research-first) before
code is written.

## 1. Audit of what exists

### 1.1 Go logging

- One logger, built once: `cmd/inkwell/main.go:46` `slog.New(slog.NewTextHandler(os.Stderr, nil))`. Default level (Info), text format, **stderr only**. No file, no rotation, no level switch, no `ReplaceAttr` (so no redaction), no
  `slog.SetDefault`. A desktop launch from the menu discards stderr, so the logs are lost.
- `cmd/inkwell-relay/main.go:88` has its own stderr text handler (a server: stderr is right there).
- The logger is injected (`Options.Log`, `main.go:68`) into app, bridge, blob, typst, preview. Eight constructors fall back to `slog.DiscardHandler` when nil (`internal/app/present.go:129`, `files.go:119`, `wiring.go:200`, `node.go:174`, `preview.go:175`, `internal/blob/service.go:146`,
  `store.go:157`): good for tests, but it means a forgotten wiring is silent.
- Four call sites bypass the injected logger and use the global `slog`: `internal/app/docs.go:102`, `internal/app/persist.go:262`, `internal/language/language.go:95,108,119,157`. They go to the default handler (stderr, `log` package format), not to ours.
- About 70 log calls, only free-text messages with ad-hoc keys ("store stream: ..." prefixes), no event names, no categories, no session or peer correlation id.
- Fatal startup paths log then `os.Exit(1)` (`main.go:51,56,75,165`): the only trace of "the app did not start" is on stderr.
- Silent degradation: `internal/app/node.go:88-92` retries `p2p.New` on port 0 when 7419 is taken and logs **nothing** about it. Two instances (or a stale process) therefore run on a random port with no word to the person or the log; if the retry also fails, `main.go:75` exits with a stderr
  line. The coordinator reported the bind error as "stderr only": confirmed, and worse, the first failure is not even printed.

### 1.2 Panics and crashes

- **No `recover()` anywhere** in `cmd/` or `internal/` (search over non-test Go): not in the `shutdownLoops.Go` goroutines (`main.go:213-221`), not in the stream handlers (`main.go:123-127`), not in the sidecar goroutines (`internal/language/language.go:112-121`).
  A panic in any goroutine kills the process; the Go runtime prints the trace to stderr, which is lost on a menu launch.
- No `debug.SetCrashOutput`, no crash file, no "closed unexpectedly" flow. Go is 1.27.3 (`go.mod:3`), so `SetCrashOutput` (1.23+) is available; fatal errors that bypass recover (out of memory, concurrent map write, deadlock, SIGSEGV in cgo/WebKit) can only be caught that way.
- Wails `application.Options` in `main.go:95-120` sets no panic or error handler (to verify in the Wails v3 `llms.txt`, skill `inkwell-stack`: v3 exposes `PanicHandler`/`ErrorHandler` options in recent alphas; check the pinned version before relying on it).

### 1.3 Webview

- No `window.onerror`, no `unhandledrejection` listener, no Svelte `onerror`/error boundary: `frontend/src/main.ts` (22 lines) just mounts `App.svelte`/`PublicWindow.svelte`.
- Errors surface only as `console.warn/error` (`frontend/src/editor/amSync.ts:62`, `workspace/workspace.svelte.ts:290,407,510`, `store/GoStorageAdapter.ts:82,212`, `store/externalSync.ts:175`, `preview/PreviewClient.ts:96`). The webview console is not
  persisted and there is no channel from the webview to the Go logger. Worker errors (`editor/search/runner.ts:79,105`, `csv/open.ts:80`, `history/analysis.ts:218`) are swallowed or turned into a UI state, never recorded.
- No long-task / freeze detection (no `PerformanceObserver` for `longtask`, no heartbeat).
- A perf-native collector exists (`internal/nativeperf`, `frontend/src/perf/nativeEarly`) but is a measurement harness behind a build flag, writing to stderr (`nativeperf/service.go:190-210`), not a diagnostic channel.

### 1.4 Sidecars

- `internal/sidecar/run.go:117,142`: stdout and stderr are captured into capped buffers (`DefaultMaxOutput`, `run.go:19`) and returned in `Output{Stderr, ExitCode, Truncated}` (`run.go:189-191`). Typst: stderr is parsed into diagnostics (`internal/typst/engine.go:618-657`)
  and the first line is shown on failure; it is never logged. Tinymist (`language.go:112`): the result is logged only as "language sidecar stopped" with `exit` and `truncated`; **stderr is dropped**, so a tinymist crash has no reason attached. The `launcher_linux.go:55` sandbox error
  goes to the child's own stderr.

### 1.5 What the person can see or copy

- About dialog (`frontend/src/ui/AboutDialog.svelte`): name, tagline, version, short commit and date, licence, offline mode. Nothing is copyable; no "Copier les informations" exists in code (the only match for that string is `docs/design/spec.md`, a design intention). No Help screen with
  a report entry (the embedded help is still a plan item). No "open the logs folder".
- Health: `StatusBar.svelte` has a sync state (`statusOf`, W33 says it is dishonest today) and `internal/bridge/linksnotice.go` sends a `Links` notification (peer link list capped at `MaxLinks`, ages reduced by `stableLinks`). There is **no** subsystem-degraded signal: a node that
  failed to bind, a dead tinymist, a stopped blob service (`node.go:206`) or a bridge that died (`main.go:145`) only log to stderr.

### 1.6 Log lines that can carry sensitive data

Rules: `docs/engineering-principles.md:45` (no document data in logs or bug reports), `:42` (tokens never logged). Findings, ranked:

| Where | Field | Risk |
|---|---|---|
| `internal/bridge/storestream.go:626,642` | `"key", req.Key` | store keys are document/file paths and ids: path leak. `err` can embed paths too |
| `internal/app/files.go:241` | `"path", rej.Path`, `"why", rej.Err` | a **hostile peer-supplied** file name goes verbatim into the log (log injection and path/name leak) |
| `internal/app/files.go:466`, `cmd/inkwell/main.go:133` | `"path"/"file", rel` | project-relative file name = user content |
| `internal/app/attest.go:244,249,284,315`, `wiring.go:425,637`, `blob/service.go:222,272,306,326,328,570,580` | `"peer", peer` | full peer id: a stable pseudonymous identifier, correlates people across logs and reports |
| `internal/blob/*`, `files.go:448,451` | `hash` (short in blob, full in files) | content hash identifies a document's bytes (confirms known content) |
| `internal/app/invites.go:257,368` | `err` from the invite flow | Wormhole code / rendezvous errors may embed the short code or URL (to check in `internal/invite`) |
| every `"err", err` | wrapped errors | `*PathError` embeds absolute paths with the user name; Typst/Go parse errors may quote document text (`typst/engine.go:657` shows the first stderr line, which can be a source excerpt) |
| `internal/language/language.go:157` | `"err"` of a refused frame | LSP frames carry document text if an error quotes the frame |

No log line prints a key or a token today (positive finding: `image.go:20` keeps the asset token out of URLs). The risk is structural: every value is an untyped `any`, so the next call site can leak.

## 2. Research (verify each URL at plan time)

### 2.1 Where logs go

- Linux: XDG Base Directory spec: `$XDG_STATE_HOME`, default `~/.local/state`, is for "state data that should persist between restarts but is not important enough for data": logs, history. So `~/.local/state/inkwell/log/`. https://specifications.freedesktop.org/basedir-spec/latest/
- Windows: `%LOCALAPPDATA%\Inkwell\Logs` (local, non-roaming). macOS: `~/Library/Logs/Inkwell/` (Apple File System Programming Guide; Console.app lists it). Go has no stdlib call for either, so a tiny `logdir` per OS with build tags; `os.UserConfigDir` is wrong here (config is roamed).
  https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/FileSystemProgrammingGuide/MacOSXDirectories/MacOSXDirectories.html
- The sandbox in `internal/sidecar/workdir.go:65` already treats `~/.local/state` as secret to the sidecars: logs there stay unreadable by Typst. Good, keep it.
- journald: apps can write to the journal via stderr when started by systemd, or via `go-systemd/journal`. For a GUI app launched from a desktop entry, GNOME starts it as a transient systemd scope/service and captures stderr into the journal on most distributions. It
  is a useful free extra on Linux, **not** the primary store: it is absent on macOS and Windows, rotation and access depend on the distro (a normal user may lack the `systemd-journal` group), and people cannot send it easily. Recommendation: file first; additionally keep stderr output at
  Warn+ so `journalctl --user` shows it; no journald dependency. https://www.freedesktop.org/software/systemd/man/latest/systemd-journald.service.html
- How the others do it: VS Code and Zed write per-session log files (VS Code: a dated `logs/` folder per launch with one file per process and an "Open Logs Folder" command; Zed: `Zed.log` in `~/.local/share/zed/logs` / `~/Library/Logs/Zed`, `zed: open log`, previous run in `Zed.log.old`,
  a crash minidump uploaded only if telemetry is on). Obsidian: developer console plus a debug log; no persisted file by default. GNOME apps: GLib structured logging (`g_log_structured`, `G_MESSAGES_DEBUG`, `GLIB_LOG_WRITER`) to the journal, level by env var. Firefox: crash reports
  saved locally in `Crash Reports/pending`, shown at the next launch by the "Firefox has crashed" page; sending is opt-in per report. https://code.visualstudio.com/docs/supporting/troubleshoot , https://zed.dev/docs/troubleshooting , https://docs.gtk.org/glib/logging.html ,
  https://support.mozilla.org/kb/mozilla-crash-reporter-tell-mozilla-about-crashes

### 2.2 Rotation and retention

- `gopkg.in/natefinch/lumberjack.v2` is the de-facto slog sink (`io.Writer`), but is unmaintained for years and rotates by size with backups/age/compress. The need here is smaller: a ring of K files of S bytes. A 100-line home-made `rotor` (write; if size+len > S rename `n -> n+1`, drop
  the oldest, `os.Root`-confined, `0o600`/`0o700`) removes a dependency (principle "stdlib before dependency", `engineering-principles.md:11`) and is exactly testable (size cap proof). Chosen: home-made. https://github.com/natefinch/lumberjack
- Retention: 5 files x 2 MiB = 10 MiB for the app log, 1 MiB x 3 for the previous-session copy; a session file name carries the start time; at start, delete anything older than 14 days. Crash files kept 10, 1 MiB each. Disk-full: the writer degrades to the in-memory ring and emits one
  `store.log_unwritable` event; logging never blocks or fails the caller.

### 2.3 Structured fields and redaction by design

- `log/slog` supports `LogValuer` (a type controls how it is logged), `ReplaceAttr`, groups and `Handler` wrapping; a custom `Handler` can enforce a policy for every record. The security literature (OWASP Logging Cheat Sheet) says: never log what you do not need; sanitise at the
  sink, but make the type system the first line. https://pkg.go.dev/log/slog , https://go.dev/blog/slog , https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html
- Beyond the usual `LogValuer` redactor: close the `any` hole. Allow only attributes built by constructors of a package `internal/diag`, which returns an opaque `Field` (wrapping `slog.Attr`); the logger API takes `...Field`, not `...any`. Then a raw `"path", p` does not compile.

### 2.4 Crash capture

- Fatal errors are not recoverable and skip `defer`: `runtime/debug.SetCrashOutput(f, opts)` (Go
  1.23) duplicates the fatal-error/panic trace to a file in addition to stderr, fixing exactly the "stderr is lost" case; it must be a file the process keeps open. With `CrashOptions{}`; the
  runtime also honours `GOTRACEBACK`. https://go.dev/doc/go1.23#runtime/debug , https://pkg.go.dev/runtime/debug#SetCrashOutput Go 1.27 notes to verify in `use-modern-go` before use.
- Plan: at start, open `crash/current.log` and `SetCrashOutput`; a clean exit deletes (or empties) it and writes `clean-exit` marker; the next start finds a non-empty `current.log`/missing marker = previous run crashed, moves it to `crash/<time>.txt` and offers the report (no upload).
  Recoverable panics in our goroutines go through `diag.Go(name, fn)` (recover, log with stack, write crash file, re-panic or mark the subsystem degraded depending on policy).
- SIGKILL/OOM kill and power loss leave no Go trace: the **absent clean-exit marker plus the breadcrumb ring flushed to disk every few seconds** are the only evidence; say so in the report ("the app stopped without a trace: probably killed by the system, low memory").
- Webview: `window.addEventListener("error")` and `"unhandledrejection"`, Svelte 5 `<svelte:boundary onerror>`, `PerformanceObserver({type:"longtask"})` and the `securitypolicyviolation` event, forwarded through one Wails stream message (the existing `bridge` stream) to the Go
  `diag` sink, rate-limited (token bucket, dedupe by hash of message+top frame). https://developer.mozilla.org/docs/Web/API/Window/unhandledrejection_event , https://svelte.dev/docs/svelte/svelte-boundary , https://developer.mozilla.org/docs/Web/API/PerformanceLongTaskTiming
- WebKitGTK web-process crash: the `WebKitWebView::web-process-terminated` signal carries a reason (`CRASHED`, `EXCEEDED_MEMORY_LIMIT`, `TERMINATED_BY_API`); Wails v3 may not expose it (verify; if not, the Go side detects a webview that stops answering the heartbeat, below). On macOS WKWebView:
  `webViewWebContentProcessDidTerminate`. https://webkitgtk.org/reference/webkit2gtk/stable/signal.WebView.web-process-terminated.html
- Next-launch flow (GNOME/Firefox style): a dialog "Inkwell s'est arrete de maniere inattendue" with the actions "Copier le rapport", "Ouvrir le dossier des journaux", "Ignorer"; nothing is sent.

### 2.5 Local report bundle

Content (all local, assembled on demand, redacted by construction): app version/commit/date, OS and desktop (`XDG_CURRENT_DESKTOP`, session type X11/Wayland), WebKitGTK/WebView2/WKWebView version, Go version, sidecar versions (typst, tinymist from the manifest), CPU count and RAM, free disk on
the data volume, locale; settings **allowlist** (never the whole settings file); last 500 log lines; latest 3 crash files; breadcrumb ring; subsystem health table; network snapshot (section 6). Format: one UTF-8 text/markdown block, at most 256 KiB, suitable for pasting into an issue.

### 2.6 Level control without restart

`slog.LevelVar` is atomically settable. Sources, by priority: `INKWELL_LOG` env (`debug`,
`net=debug,sync=info`), Preferences switch "Journal detaille" (writes the setting, flips the
`LevelVar` live), default Info. A debug session auto-reverts to Info after 24 h and is recorded in the report ("detailed log on since ..."). Per-category levels: one `LevelVar` per category.

## 3. Proposal

### 3.1 Package and API

New package `internal/diag` (one role: record, rotate, report), created at the lot that needs it (CLAUDE.md "no empty package"). Modules: `field.go` (typed fields), `event.go` (taxonomy),
`handler.go` (slog handler + redaction), `rotor.go`, `crash.go`, `breadcrumb.go`, `report.go`,
`health.go`, `watchdog.go`. `cmd/inkwell` builds it first, before `OpenHome`, so even startup failures reach the file. The existing `*slog.Logger` injection stays: `diag.Logger` embeds one, and the 4 global `slog.*` calls are moved to the injected logger (`slog.SetDefault` set as a safety net).

### 3.2 Levels and categories

Levels: `Error` (a thing the person would notice or that loses data), `Warn` (degraded, retried or refused input), `Info` (lifecycle and state transitions, once each), `Debug` (per-message detail, off by default). Categories (a `cat` attribute, one `LevelVar` each): `app`, `net`, `sync`,
`typst`, `store`, `ui`, `action`, `sec` (security events, always at least Warn, matches the existing `"event","security"` convention at `files.go:241`, `blob/service.go:326`).

### 3.3 Event taxonomy (stable names, never reworded; a golden file lists them)

Format `cat.noun.verb`, snake case. Fields are only the typed kinds in 3.4. Level in brackets.

| Event | Level | Fields |
|---|---|---|
| `app.start` / `app.stop` | I | version, commit, os, session id, `clean` |
| `app.start_failed` | E | stage (`home`,`project`,`net`,`wails`), reason class |
| `app.panic` | E | goroutine name, stack (Go frames only), subsystem |
| `net.listen.ok` | I | addr kinds (count of v4/v6/relay), port, `fallback`(bool) |
| `net.listen.fallback` | W | wanted port, got port, reason class (`addr_in_use`) |
| `net.listen.failed` | E | reason class; **also sets health `net=down`** |
| `net.relay.reserve` / `net.relay.lost` | I/W | relay id (hashed), reason class |
| `net.path.change` | I | peer (hashed), from/to (`direct`,`relay`), rtt bucket |
| `net.nat.status` | I | reachability (`public`,`private`,`unknown`), changed |
| `net.conn.open` / `net.conn.close` | I | peer (hashed), conn id, dir, transport, duration, close class |
| `net.conn.fail` | W | peer (hashed), stage (`dial`,`handshake`,`auth`), class, attempt n |
| `net.send.cut` | W | peer, conn id, bytes queued, reason (send-path design) |
| `net.frame.refused` | W | peer, kind, why class (bounds, signature, role) |
| `sync.resync.start` / `.done` | I | peer, rule-L cause, ops count, duration |
| `sync.diverged` | E | peer, doc id (hashed), heads count |
| `sync.apply.fail` | E | doc id (hashed), class (`invalid_change`,`bounds`) |
| `sync.state` | I | state transition (`pending n`, `behind`, `synced`), same source as W33 |
| `typst.compile.fail` | W | class (`syntax`,`oom`,`timeout`,`sandbox`), ms, diagnostics count |
| `typst.sidecar.exit` | W/E | which (`typst`,`tinymist`), exit code, signal, truncated, **stderr tail sanitised** |
| `typst.sandbox.unavailable` | E | capability probed, reason class |
| `store.write.fail` / `store.read.fail` | E | op, class (`enospc`,`eacces`,`corrupt`,`io`) |
| `store.corrupt` | E | collection (not key), action (`quarantined`,`rebuilt`) |
| `store.disk.low` | W | free bytes bucket |
| `store.backup` | I | ok/fail, count |
| `ui.error` | E | source (`error`,`rejection`,`boundary`), message class, top frame (file:line of **our bundle**) |
| `ui.freeze` | W | stall ms, last action id, last breadcrumb ids, task kind |
| `ui.render.fail` | W | surface (`preview`,`editor`,`present`), class |
| `ui.webview.crashed` | E | reason (`crashed`,`oom`,`unresponsive`) |
| `action.run` | breadcrumb only | command id, ms-since-start (never logged to the file at Info) |
| `sec.refused` | W | where, why class (replaces the raw path log at `files.go:241`) |
| `diag.log_unwritable` | W | class; written to stderr |

Correlation ids (random 64-bit, hex, from `crypto/rand`, never derived from user data):
`sid` per process session, `cid` per peer connection (alongside the hashed peer), `op` per operation (a compile, a resync, a save). They are attached by `logger.With(...)` or carried in
`context` (`diag.WithOp(ctx)`); the same `op` appears in the breadcrumb, the log and the crash file.

### 3.4 Redaction as a type-level design

1. `package diag` exposes `Field` constructors only for safe kinds; there is no `diag.String(string)`:
   - `Class(string)` : a value from a closed, compile-time enum (`type Reason uint8` with a `String()` from a table) so the text is ours by construction;
   - `Count(n)`, `Bytes(n)`, `Dur(d)`, `Bool`, `Bucket(n)` (log2 buckets for latency and sizes);
   - `Peer(id)` : hashes `HMAC-SHA-256(sessionSalt, id)` truncated to 6 bytes (hex 12): stable inside one session for correlation, **different across sessions and across reports** (no cross-report tracking); the salt lives only in memory;
   - `Doc(id)` / `Blob(hash)`: same keyed hash, 4 bytes;
   - `Err(err)` : logs `ErrClass(err)` (`errors.Is/As` mapping to the closed enum: `enospc`, `eacces`, `eaddrinuse`, `timeout`, `refused`, `eof`, `unknown`) plus, in Debug only, the type name via `%T`; **never `err.Error()`**, which can embed paths and quoted text;
   - `Sensitive[T]`: a wrapper whose `LogValue()` returns `slog.StringValue("[redacted]")` and whose `String()`, `GoString()`, `Format` and `MarshalJSON/Text` all redact, so even `fmt.Sprintf("%v")` or a stray `slog.Any` cannot print it. Document text, file names, paths, invitation codes,
     keys and tokens are carried as `Sensitive` at the point they enter the program (decoders and bridge handlers), so a later log call cannot leak them without an explicit `Reveal()` that a lint forbids outside `internal/diag`;
   - `Stack(skip)` : frames as `package.func file.go:line`, **file base name only**, no arguments.
2. The logging API takes `...diag.Field`, not `...any`: a raw string or `error` value does not compile (`log.Warn(ev, "path", p)` is a type error).
3. Backstop in the sink: the handler runs a `ReplaceAttr`-style pass on every record that (a) redacts any attribute of kind `String`/`Any` not produced by a `Field` (should be unreachable; counted as `diag.policy_violation` and shown in tests), (b) truncates any value to 200 bytes,
   (c) strips control characters (log injection from hostile peer names), (d) rewrites `$HOME`, the project root and `INKWELL_HOME` prefixes to `~`, `<project>`, `<home>`.
4. A lint (`go vet`-style analyzer in `scripts/lint-diag`, run by `mise run lint`, or a `forbidigo`/`depguard` rule in `.golangci.yml` as the cheap first step): forbid `log/slog` calls (`slog.Logger.Info|Warn|Error|Debug`, `slog.Info...`) and `fmt.Fprint*(os.Stderr`
   outside `internal/diag` and tests; forbid `err.Error()` and `%w`-formatted strings as log values; forbid `Reveal` outside diag. Migration of the ~70 existing calls is mechanical per package.
5. Webview side: the same rule in TypeScript: `ui/diag.ts` exports `report(event, fields)` with fields limited to `number | boolean | enum`; Biome rule `noConsole` outside `diag.ts`; messages from `Error` are reduced to a class, the stack to our bundle's `file:line` (Vite sourcemap
   names), never source text.
6. Sidecar stderr is attacker/document influenced (Typst quotes source lines): never logged raw. Stored only as `Class` + line count + 256-byte tail passed through the same sanitiser and shown only in the report when the person has ticked "Inclure les messages d'erreur de Typst" (default
   off), because the tail can quote the document.

### 3.5 Files, rotation, level

```
Linux   ~/.local/state/inkwell/log/        Windows %LOCALAPPDATA%\Inkwell\Logs\     macOS ~/Library/Logs/Inkwell/
  inkwell.log, inkwell.1.log ... .4.log     (2 MiB each, 5 files)    JSON lines (slog JSONHandler) for tools,
  last-session.log                          copy of the previous session's tail (so a restart does not push it out)
  crash/current.log   crash/<UTC>.txt (max 10, 1 MiB)   breadcrumbs.bin (ring, flushed every 5 s)
  clean-exit          marker, removed at start, written at orderly shutdown
```
Mode `0o700` dir, `0o600` files, opened through `os.Root` (principle in `engineering-principles.md`). File is JSON lines, human reading through the report view (text). Stderr keeps Warn+ in text form (journald and `go run`). Level: default Info; `INKWELL_LOG` env or Preferences "Journal detaille"
flips the `LevelVar` live (no restart); auto-revert after 24 h. `INKWELL_LOG` joins `.env.reference` (`mise run lint:env`).

### 3.6 Breadcrumbs (anonymous user actions)

- Ring of the last 256 entries in memory (`[]entry`, fixed array, lock-free index), entry = `{t: ms since session start, kind: action|state|event, id: uint16 enum, a: uint32}`; **no strings**. `action` ids are command ids from the planned command registry (W40 R3) or the existing menu and
  shortcut ids: a closed table, so the type cannot hold a name or path. `state` entries are transitions (`sync pending -> synced`, `net up -> degraded`, window focus, project opened as a count only). `a` is a small number (a count, a bucket), never content.
- Recorded by default only for system events/state transitions (diagnosing needs them); **user action** breadcrumbs are off until the person turns on "Garder une trace locale de mes actions" in Preferences ("Reste sur cet appareil. Aucun contenu, aucun nom, aucun chemin ; seulement le
  nom des commandes et l'heure."), which is the answer to ux rule 10 (no telemetry) and the anonymity requirement. Off: action entries are not even stored.
- Flushed to `breadcrumbs.bin` every 5 s and on panic, so it survives a crash; included in the crash file and in the report as decoded names plus relative timestamps.

### 3.7 Crash flow

1. Start: `diag.Start` opens the rotor, sets `SetCrashOutput(crash/current.log)`, checks the marker. Previous run unclean => `crash/<UTC>.txt` assembled from `current.log` + the flushed breadcrumb ring + `last-session.log` tail + health snapshot; set `pendingCrash`.
2. Running: `diag.Go(name, fn)` replaces bare `go func()` for long-lived goroutines (`shutdownLoops.Go` at `main.go:213`, `language.go:112`, stream handlers, blob/wiring loops): recover, log `app.panic` with stack, write the crash file synchronously, mark the subsystem degraded; for the
   main goroutine or unknown state, exit with a non-zero code so the next start shows the dialog.
3. Clean exit (`OnShutdown`, `main.go:120`): write the marker, empty `current.log`.
4. Next start, after the window is up: dialog "Inkwell s'est arrete de maniere inattendue" (3.9).
5. Webview crash (`ui.webview.crashed` or heartbeat lost 10 s): Go keeps running, writes `ui.webview.crashed`, and when the window can be reloaded does so, then shows a toast "L'affichage s'est relance; votre travail est sauvegarde" (the store is Go-side, so this is true; test it).

### 3.8 Health signal (detection, not only logging)

`diag.Health` is a small registry: for each subsystem (`net`, `sync`, `typst`, `language`, `store`,
`preview`, `webview`) a state `ok | degraded | down` + a reason class + since-when. Setters are the same call that emits the event (`h.Set("net", Down, ReasonAddrInUse)` emits `net.listen.failed` and flips the state; one code path means the event and the signal cannot diverge).
Exposed to the webview over the existing bridge stream (a `Health` notification next to the `Links` notification, `linksnotice.go`), consumed by:
- **StatusBar** (`StatusBar.svelte`): the existing sync indicator gains one more state "Hors reseau" when `net` is down/degraded (ties to W33: same item, same popover, which lists the person's links; the popover shows "Pourquoi ?" with the plain reason and a "Copier le rapport" link);
- the existing single banner (UF-rule: one banner, one toast) for a persistent failure that needs an action, e.g. "Inkwell n'a pas pu se connecter au reseau. La collaboration est coupee; vos documents restent enregistres sur cet appareil." with "Details" => the report view;
- `net.listen.fallback` is **not** an error for the person (random port works) but is logged at Warn; `net.listen.failed` is `down` and visible. This closes today's silent `node.go:88-92`. Wording follows the glossary: no "pair", "relais", "port", "bind" in the UI; "personnes",
"connexion", "reseau local".

**UI freeze watchdog.** Webview posts a heartbeat every 1 s from a `requestAnimationFrame`-driven tick and records `longtask` entries (>= 100 ms) with the last breadcrumb ids; Go compares: a gap > 3 s emits `ui.freeze` (`stall ms`, last action, what Go was doing: active compile, resync, store op,
read from `diag` op registry), and a gap > 10 s emits `ui.webview.crashed(unresponsive)`. Thresholds are constants in one table, pinned by tests. Heartbeat cost is measured with benchstat (HOT_PATHS includes `internal/app`) before merging.

Ties: `mise run diagnose` (W36, plan `docs/plans/2026-11-08-prevent-recurrence.md`) is the CI-side twin: it prints, for a failing gate, the same event names and fields from the proof directory. One taxonomy file (`internal/diag/events.go`) is read by both; netsim failures (W32) assert on events
instead of grepping strings.

### 3.9 Report bundle and wording

Placement (no new screen, UX coherence rule): About gets a "Informations de diagnostic" section (version, build, offline mode already there; adds system, webview, sidecar versions) with two buttons: **Copier le rapport** and **Ouvrir le dossier des journaux**; the future embedded Help
gets an entry "Un probleme ?" linking to the same dialog; the crash dialog and the degraded banner open the same view. Preferences > Confidentialite gets: "Journal detaille" (switch, auto-off 24 h), "Garder une trace locale de mes actions" (switch), "Inclure les messages d'erreur de Typst" (switch).
Report view: read-only text with a plain header "Ce rapport reste sur votre appareil. Il ne contient ni vos documents, ni les noms de vos fichiers, ni vos mots de passe. Vous choisissez quoi en faire." Copy = clipboard, toast "Rapport copie". Open folder = `xdg-open`/`open`/`explorer` on the log dir
(via the Wails shell API, fixed path, no argument from the webview).

Report sections: header (time, session id, app/commit/date, clean/unclean last exit), system, settings allowlist, **health table**, **network snapshot**, last 500 log lines (Warn+ and the state-transition Infos), breadcrumbs, last 3 crash summaries. Size cap 256 KiB; truncation is
marked, oldest first.

## 4. Network incident diagnosis from a report

Reproducing a network incident needs the **sequence** and the **conditions**, without identities. Included in the network snapshot (all via `Peer()` hashes, buckets and classes):
- listen result: wanted/actual port, address kinds, reachability and NAT status, relay reservation state and age, `INKWELL_NET` mode (relay-only), LAN discovery on/off, rendezvous mode;
- per-connection table: hashed peer, `cid`, direct or relay, transport, age, RTT bucket, loss and stability class (the same numbers W33 shows, from the `Links` notification source, `linksnotice.go`), bytes queued, last error class;
- the last 100 `net.*` and `sync.*` events with `cid`/`op` and relative times (path changes, resync causes under rule L, send-path cuts, refused frames), so the incident timeline is readable;
- sync state per document: hashed doc id, pending/behind counts, last resync, heads count (not heads);
- the library versions (libp2p, protocol step `conformance.CurrentStep`, `INKWELL_PROTOCOL_STEP`). That is enough to replay in netsim (`scripts/netsim`): a report maps to a scenario (topology direct/relay, NAT class, loss bucket, event order). A `netsim --from-report` importer is a later
residual (graded, not part of the first lots). What stays out: addresses (IP is personal data; only kinds and counts), peer ids, keys, document ids in clear, payload sizes beyond buckets.

## 5. Executable proofs

Each row is a test or gate; a seeded defect (mutant) must turn it red (W36 method). Gate file
`gates/DG1.toml` (commit it before the code).

| # | Proof | How |
|---|---|---|
| P1 | Panic writes a crash file; next start offers the report | test re-executes the test binary (`os.Args[0] -test.run=TestHelper` with env) that panics in a `diag.Go` goroutine, and a second helper that triggers a fatal error (`concurrent map writes`) to prove `SetCrashOutput`; assert `crash/<UTC>.txt` exists, contains the stack, breadcrumbs and no `Sensitive` marker value; then start `diag.Start` on the same dir and assert `PendingCrash()==true`, and false after dismissal. Memory: through `scripts/heavy.sh`. |
| P2 | Redaction | table test with document text, a file name, a path, a full peer id, a 32-byte key, a Wormhole code, an asset token as `Sensitive`/raw: render through every handler and assert none of the canaries appears in the file, stderr, report or crash output |
| P3 | Redactor property/fuzz | `FuzzSink` (random Unicode, control bytes, huge, `%` verbs, newlines): invariants: output is valid JSON line, <= cap, no canary substring from the seeded secret set, no raw newline, no `$HOME`; plus `rapid`/`testing/quick` property that `Sensitive[T]` never prints under `%v %+v %#v %s %q %x`, `json.Marshal`, `slog.Any`. Mutation (gremlins) on `internal/diag` >= 95 % |
| P4 | Type-level guard | compile-fail test (`go vet` analyzer test data, `analysistest`) for `log.Warn(ev, "path", p)`; lint run in `mise run lint` fails on a seeded raw call |
| P5 | Rotation size cap | write 50 MiB of lines in a loop: total bytes in dir <= files x size + one line; file count <= cap; oldest dropped; concurrent writers; disk-full (`ENOSPC` through a fake `fs`) never blocks the caller and emits `diag.log_unwritable` once |
| P6 | Level switch | `LevelVar` flip at runtime changes output without restart; env syntax `net=debug,sync=info`; auto-revert via a fake clock (`testing/synctest`) |
| P7 | Network bind failure (first case) | occupy 7419 with a listener, start `app.Start`: assert event `net.listen.fallback` (W) with wanted/actual port; then make the retry fail too (injected failing listener) and assert `net.listen.failed` (E), `Health[net]==down` and the bridge sends the `Health` notification with reason `addr_in_use`; e2e (Playwright, two instances, `ux-tester` scenario): second instance shows the plain banner/status text, and "Copier le rapport" contains `net.listen`; WCAG axe check on the banner (UF-1) |
| P8 | One test per failure class: event and signal | `sync.diverged`/`sync.apply.fail` (bad change from a fixture peer => event + sync indicator not "Synchronise"); `typst.sidecar.exit` (kill the sidecar => event with exit/signal + preview error state; tinymist stderr no longer dropped); `store.write.fail` (read-only dir / `ENOSPC` fake => event + persistent banner "Impossible d'enregistrer"); `ui.error` (throw in a Playwright page, reject a promise => event arrives in Go log, rate-limit holds under 10 000 errors); `ui.freeze` (busy-loop 4 s in page => event with last action); `ui.webview.crashed` (heartbeat stops); `net.send.cut`, `sync.resync.*` driven by the netsim impaired scenarios (W32) |
| P9 | Health/event single source | property: every `Health.Set` call emits exactly its event (spy handler), every event in the table appears in `events.go` and the golden file (name stability), no event name appears in code that is not in the table |
| P10 | Report | unit: sections present, size <= 256 KiB, truncation marker, settings allowlist (a seeded secret setting is absent); e2e: click "Copier le rapport" in About, read clipboard, assert version, health table, no canary; "Ouvrir le dossier" calls the shell API with the fixed path only |
| P11 | Breadcrumbs | ring of 256 wraps correctly; entry has no string field (reflection test); action entries absent when the preference is off; present in crash file when on |
| P12 | Perf | benchstat for the logger hot path (`Debug` disabled must be zero-allocation), heartbeat, and ring push (HOT_PATHS) |

## 6. Order and cost

Cost in focused sessions (one session = a Sonnet go-dev block with Opus review; model per CLAUDE.md routing). Each lot follows the "fini" definition, with `gates/DGn.toml` first.

| Lot | Content | Cost | Model |
|---|---|---|---|
| DG1 | `internal/diag` core: fields, `Sensitive`, handler, rotor, level, `diag.Go`, crash file + next-start detection, migrate the ~70 calls and 4 global `slog`, lint rule, P1-P6, P9 | 3-4 sessions | architect for the redaction design (Opus/high), go-dev for the migration |
| DG2 | Health registry, `net.*` events, bind failure signal (P7), banner + status text, `Health` notification | 2 sessions | go-dev + UI via ux-walkthrough |
| DG3 | Webview capture: `window.onerror`, rejections, `svelte:boundary`, long tasks, heartbeat, watchdog, rate limit; P8 UI cases | 2 sessions | go-dev |
| DG4 | Report bundle, About/Help/crash dialog, Preferences switches, breadcrumbs, e2e P10-P11 | 2-3 sessions | go-dev + ux-tester |
| DG5 | Sidecar stderr capture and class mapping (typst, tinymist), `sync.*`, `store.*` events and the remaining P8 cases | 2 sessions | go-dev |
| DG6 (residual) | `netsim --from-report`; WebKit `web-process-terminated` if Wails exposes it; crash minidump for cgo faults | open | graded residual |

Recommended first: **DG1 then DG2** (value on the very first crash and the silent bind failure). Estimated 13-15 sessions in total. Dependencies: command registry (W40 R3) for readable action ids (until then, a closed local table); sync indicator (W33) for the health item; embedded Help for the
"Un probleme ?" entry (About works alone meanwhile).

## 7. Risks and open questions

- Wails v3 hooks (panic handler, WebKit process-terminated) depend on the pinned alpha: verify in `inkwell-stack` before DG2/DG3; fallback is the heartbeat.
- `SetCrashOutput` writes on a fatal path: the file must be on a local volume and open at crash time; on `ENOSPC` nothing is written (documented, covered by the "no trace" report line).
- Hashing with a per-session salt makes cross-session correlation of one peer impossible in reports. Accepted: privacy over convenience; a person in a live incident can read the table in one report. Decision for the user if they want a stable per-device pseudonym instead.
- User-action breadcrumbs are opt-in; if off, the report simply lacks them (stated in the report).
- Requirement to add to `docs/exigences.md` (W44) and an ADR if the `...Field` API is judged a departure from "slog standard" (it still builds on slog; propose ADR 0036 only if reviewers object).
