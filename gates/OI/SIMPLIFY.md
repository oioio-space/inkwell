# Block OI — /simplify of the diff (reuse, simplification, efficiency, altitude)

Scope: the whole diff of block OI against `d342c94` (the frozen gate): `internal/instance` (new), `internal/launch`
(new), `cmd/inkwell` (`startup.go`, `open.go`, `notify.go`, `env_*.go`, `raise_*.go`, `main.go`), `internal/app`
(`HomeDir`, `OpenProjectIn`), `frontend/src/workspace/openRequests.ts` and its wiring in `App.svelte`, the
catalogue texts `open.notice.*`, `frontend/e2e/openWith.spec.ts` and `instance.ts`, the desktop entry, the MIME
package, the manual page, `scripts/pkg/lint.sh` and `selftest.sh`, `.env.reference`, `gates/guard-overrides.json`.
Checklists: `go-style-guide`, `vuln-guard`, `secret-guard`, `simplify`, `helper-ergonomics`.

## Reuse

| Finding | Decision |
|---|---|
| The webview needed a reconnecting reader of a Wails stream; `present/hub.ts` (`HubClient`) has one, typed to the hub's frames | **Kept separate**: `OpenRequests` copies its 20-line connect/close loop (same shape, same `SocketLike`) but chains each request after the workspace's `ready` promise and in order; a shared generic client would touch the presentation code, outside this block. Noted for a later extraction if a third stream client appears. |
| The toast: reuse the existing `notice` (`ui/notice.svelte.ts`, `NoticeToast`) rather than a new component | **Applied**: `App.svelte` passes `notice.show`; no new component, no new layout (plan §4.4). |
| Opening a file in the editor: `Workspace.openFile`, as the sidebar does (with `sheetView.toGrid(undefined)` first) | **Applied**, plus `refreshFiles()` first: a file made outside Inkwell a moment ago is not yet in the list, and `openFile` silently ignores an unlisted path. |
| The profile folder: `OpenHome` computed it inline | **Applied**: `app.HomeDir()` holds the rule (the lock must be taken before the folder is opened); `OpenHome` calls it. |
| Opening a chosen project: `OpenProject(docs)` only picked the most recent | **Applied**: `OpenProjectIn(docs, dir)`, with `OpenProject` = `OpenProjectIn(docs, "")`; the sync-cache cleanup stays in one place. |
| Bounds of a request (32 arguments, 4096 bytes): needed by the client (`NewRequest`), the codec and `launch.Resolve` | **Applied**: one source, `instance.MaxArgs`/`MaxArg`; `launch` imports them instead of repeating 32 and 4096. |
| The flock call: `internal/app/snapshot_session_unix.go` already flocks a lease | **Not shared**: that lease is a per-session cache lock in `internal/app`; `internal/instance` imports nothing from inkwell (plan §4.1) and maps the errno to its own errors (`ErrNoLock`). |
| Notifications: Wails has no notification API usable before `application.New` | **Applied**: `godbus/dbus/v5`, already in the module graph (now direct), one `Notify` call, bounded by 2 s; standard error always. |

## Simplification

| Finding | Decision |
|---|---|
| Plan sketch `Serve(handle func(Request))` with `go primary.Serve(...)`: the busy answer needs the handler to say when its queue is full, and a blocking `Serve` left the goroutine's ownership to the caller | **Applied**: `Serve(handle func(Request, *Ticket) bool)` starts the accept loop itself and `Close` waits for it; the ticket carries the window's delivery acknowledgement (see « Codex review », C1). |
| `run()` was one 150-line function opening everything | **Applied**: `startup(args, deps)` (version → claim → serve → `runPrimary`) with its effects injected; `runWindow` keeps the Wails wiring unchanged; `run` only maps the exit code. |
| Shutdown order spread over `OnShutdown` | **Applied**: `stopAll` = `Closing` → today's `shutdown` → `Close`, one function, tested. |
| The open queue: plan put it in `internal/app` (`app.OpenQueue`) | **Changed**: the gate tests it in `cmd/inkwell` (`TestOpenQueueBounded`), and it only joins `instance` to the window there: `openQueue` and `opener` live in `cmd/inkwell/open.go`; no new exported API in `internal/app`. |
| Stream name `inkwell/open` (plan §4.3) | **Changed** to `open`, like every other stream (`present`, `preview`, `lsp`, `net`, `store`). |
| A one-shot event per request (would be lost before the webview listens) | **Rejected** (rule D-DRAIN): a bounded queue drained by the stream for the window's whole life; a request taken when the send fails is put back (`unpop`). |
| `launch.Intent` lacked the count of « 32 au plus » (R9) | **Applied**: `Count` on `Intent` and `Notice` (TooMany only); `Others` keeps its meaning (« Et {n} autre(s) »). |
| A second file of the current project in one request (one document at a time) had no text | **Applied**: notice kind `show` → `open.notice.notShown`, so R8's « names the first argument not shown » is total. |

## Efficiency

| Finding | Decision |
|---|---|
| The decoder allocates the body only after checking the length against `MaxFrame`, and reads the version byte first (any other version answers « unsupported » without reading its body) | **Kept** (plan §4.1); fuzzed. |
| Per-launch cost: one connect + one 136 KiB-at-most write + one byte read on a local socket; back-off 10 ms doubling to 200 ms, ≤ 30 tries in 5 s | **Kept**; `TestBackoffSchedule` pins it. |
| `launch.Resolve` opens each argument once to test read access (only regular files and folders: a fifo is refused before, so the open cannot block) | **Kept**: the alternative (`access(2)`) is not portable and answers for the real, not effective, user. |
| `Primary.Serve`: 16 connections at once, each bounded by `ReadTimeout`; `Close` waits for them (≤ 2 s) | **Kept**: bounds the goroutines (`TestHandoffSlowClient` counts them). |

## Altitude

| Finding | Decision |
|---|---|
| Where the arguments are resolved | Primary side, against the forwarded working directory (rule D-RESOLVE-HERE); the second process reads nothing but its own command line and environment (`TestSecondLaunchTouchesNothing`). |
| Texts for people | Webview catalogue only (`open.notice.*`, French and English), from a structured notice; the three Go texts are the notifications of a process with no window (R11, R17, R18). |
| A 4096-byte-or-longer argument cannot cross the wire (arguments are at most 4096 bytes) | **Decided**: `NewRequest` cuts it to 4096 bytes and `launch` refuses any argument of 4096 bytes or more (Linux `PATH_MAX` counts the NUL), so the cut one still gets « Ce chemin est trop long » and can never name another file after `Clean` (test case in `TestResolveRefused`). |
| The activation token never reached Go: GTK 4 unsets `XDG_ACTIVATION_TOKEN` and `DESKTOP_STARTUP_ID` in a library constructor (`gdk.c`, `stash_and_unset_environment`) before the Go runtime copies the environment (measured: a C program linked with `gtk4` sees the variable unset, the same without `gtk4` sees it; the cmd test binary's `os.Getenv` returned "") | **Fixed at the root**: `startEnv` reads the environment the process started with from `/proc/self/environ` (unsetenv moves pointers, not the strings), `os.Getenv` elsewhere; found by `TestSecondLaunchForwardsArgsAndCwd`, pinned by `TestStartEnvKeepsActivationToken`. Without it every raise would have lacked the token (GNOME's « Inkwell est prêt » instead of the window). |
| A launch whose working directory is unknown | `NewRequest` sends `.`; `launch` then reports relative arguments as missing and still opens absolute ones. |
| Windows and macOS | Lock (`LockFileEx`) and macOS peer credentials (`LOCAL_PEERCRED`) are written and cross-compiled (`mise run arch` vets windows and darwin); Windows peer credentials fail closed (no request sent or read) until N35, as the plan schedules. |
| Applying the activation token to the window (plan §4.3: `gtk_window_set_startup_id` from a small cgo file) | **Fixed 2026-11-13 (plan §13, ADR 0037 amendment)** after the limitation first kept here failed with the installed package (window not raised, « Inkwell est prêt », busy cursor): `internal/activate.Present` calls `gtk_window_set_startup_id` then `gtk_window_present`; it is the one package besides `internal/sidecar` allowed to import `C`, and only `C` (`tools/archexec`, `TestToolkitPackageImportsCOnly`). `cmd/inkwell/raise.go` (`raiser`, effects injected) replaces `raiseWindow`. Proofs: `TestSecondLaunchTokenReachesPresenter`, `mise run app:activation` (red on the binary before, green after). |

## Codex review (adversarial, cross-family, 2026-11-12): « fix first »

| # | Finding | Root cause | Fix | Proof (red first) |
|---|---|---|---|---|
| C1 (High) | `startup.go`: requests acknowledged before the identity, project and network opened; a failed start lost them after the launches exited 0 | the acknowledgement meant « queued », a promise the process could not keep | **Acknowledged only once the window applied the request**: `Serve(func(Request, *Ticket) bool)`; the webview acks each frame (`{"applied":id}`), which calls `Ticket.Applied`; until then the launch is held `ReadTimeout`, told `busy`, and retries on the same ticket; a failed start closes the primary, which answers `closing` (`Closing` drops the pending tickets): the launch becomes the next primary | `TestStartupFailureAcksNothing` (cmd), `TestHandoffAckAfterApply` |
| C2 (High) | `claim` computed the socket candidates once: a launch that started before the primary made `/tmp/inkwell-<uid>` (long profile path, no `XDG_RUNTIME_DIR`) kept zero candidates and timed out | candidates are filesystem state, cached as a constant | `socketPaths(dir, runtimeDir(false))` on every try | `TestHandoffFallbackCreatedLate`: failed with « the running Inkwell does not answer » after 5 s before the fix, passes after |
| C3 (High) | a clean shutdown marked « closing » but dropped requests already acknowledged and queued | same as C1 | same as C1: nothing pending was acknowledged; `Closing` answers `closing` to it and the launch opens its file as the next primary. Model: S4 is now `ExitOkImpliesApplied`, L2 `AckedApplied` has no escape for a closing or crashed primary, new mutant `AckDropOnClose` (acknowledged at queueing, `MaxCrashes = 0`) is caught by `AckedApplied`; `AckNoQueue` now caught by `AckedApplied` | `TestHandoffDroppedAtClosing`; `mise run tla`: both sound OI configurations pass (19,701 and 27,312 distinct states), the 11 OI mutants and 4 witnesses violate; the model stays cheap because a launch reads « ack » or « closing » in the step that sends it (a first draft took 4 min per configuration, over the gate's 30-minute `tla` budget on a loaded machine) |
| C4 (Med) | the activation token is discarded | the cgo file of plan §4.3 was refused by `mise run arch` and no Wails API exists | **fixed 2026-11-13**: `internal/activate` (ADR 0037 amendment), the token goes to GTK before `present` | `TestSecondLaunchTokenReachesPresenter`; `mise run app:activation` |
| C5 (Med) | `json.Marshal` turns invalid UTF-8 into U+FFFD: a different path would reach the webview | paths are bytes, the stream is JSON text | `launch` refuses a file of the open project whose path is not UTF-8 (`BadName`, toast « Inkwell ne sait pas lire le nom de « {nom} ». ») before anything is shown | `TestResolveNonUTF8Name`; e2e « OI second launch with a file whose name is not UTF-8 shows the toast » (argument passed as raw bytes through `/bin/sh`) |
| C6 (Med) | unbounded promise chain in the webview; Go drained its queue on send | no back-pressure from the webview | Go sends one request at a time and waits for its `{"applied":id}` (an unacknowledged one goes back to the queue on reconnection); the webview holds at most `MAX_PENDING` (8) and drops a frame beyond, unacknowledged | `TestOpenerOneInFlight` (20 launches, never more than one in flight, each acknowledged once applied); Vitest « keeps at most MAX_PENDING requests outstanding… » |

## Codex re-review (native read, 2026-11-12): 4 of 6 resolved, token = documented R1 limit; new findings

| # | Finding | Fix | Proof (red first) |
|---|---|---|---|
| R1 (High) | the webview logged an application error, then sent `applied`: the launch exited 0 with nothing shown | the webview sends `{"failed":id}`; Go answers `failed` (new status 5, `ErrNotApplied`), also to a retry of the same ID; the launch shows « Inkwell n'a pas pu ouvrir le fichier » and exits 5 (R19). The window's `show` throws when the file is not the one shown (a file the project does not list, which `Workspace.openFile` used to skip silently) | `TestHandoffFailedReported`; Vitest « reports a request the window could not show as failed… »; e2e « OI second launch whose file the window cannot show fails visibly » (a hidden file: exit 5 and the French text on standard error) |
| R2 (High) | `awaitApplied` had no deadline: a webview that never answers held the stream for ever | one reader goroutine per stream; each request has `defaultApplyTimeout` (10 s), then fails (no success acknowledgement), the slot is free and the next request is served; a late answer for the old request is ignored | `TestOpenerApplyDeadline` (never-answering webview: `ErrNotApplied` after the deadline, not before; a refusal; then a request applied); Vitest « answers nothing while the workspace never becomes ready… » |
| R3 (Med) | after `closing`, a third launch C can win the lock and B's file became « another project » | rule D-FORWARD: B hands its request unchanged to C; a C still starting with nothing of its own opens B's project (`openProject(own, queued)`), else the normal rows apply. Plan and ADR amended; model witness `NoForward` | `TestHandoffAcrossTakeover` (deterministic: B's lock held back until C holds it; C gets B's request unchanged, B exits 0 only once C applied it); `TestTakeoverOpensForwardedProject` (C opens B's project, not the most recent one; the window gets `{"id":2,"show":"b.md"}`); `mise run tla` (`WitnessForward` violated) |
| R4 (Med) | same-user queue flooding | **accepted residual**, recorded in ADR 0037 « Consequences »: the user is the trust boundary; a same-user process can already read the identity key or kill Inkwell; bounds stay (64 queued, 16 connections), the victim sees R11. FIFO order of busy retries: **not done**: it needs a waiting list beyond the 64-slot queue, which is either unbounded or bounded with a « busy » of its own, i.e. the same behaviour | — [accepted ADR 0037] |
| R5 (Med) | weak oracles | the shutdown/takeover tests prove application by the third contender and the displayed frame; the in-flight property is proven through the real webview (e2e « OI a burst of launches is applied one by one through the window »: 8 launches, each exits 0 only once the real webview acknowledged it); rejection and never-answer cases assert what is displayed (launcher text, exit code, no toast) | the tests named above |

## Round 3: design decision (D1-D3)

See `gates/OI/REVIEW.md`, « Round 3 », for the table. Simplifications kept: the deadline lives in one place (Go,
`defaultApplyTimeout`, from the send); readiness has one bound (`instance.Options.StartWait`) on the primary's side,
and the launch only renews its patience while told « starting »; the webview's cancellation is one `AbortController`
per request, raced against the application so that a step that never ends holds no slot. The handover mark is one
empty file, written while the lock is held and removed by the next holder, like the socket (rule D-UNLINK); the 500 ms
wait applies only to a primary that took over and was launched with nothing of its own, so a plain launch never waits.

## Round 4: contract simplified (the golden rule, after three review rounds)

The applied/failed round trip, the `failed` and `starting` replies, the per-request deadline and `cancel`, the
handover mark, the startup wait and the forwarded-project choice are **removed** (round-3 snapshot in the scratchpad,
`oi-backup/round3`). The contract is now « received and queued » (VS Code, GApplication); the window owns the outcome
and says in a toast what it cannot do. Lines, round 3 → round 4: `instance.go` 573 → 411, its tests 1405 → 1170, `open.go` 280 → 152, `startup.go` 218 → 200, its tests 951 → 666, `openRequests.ts` 289 → 245, its tests 264 → 196, `OISingleInstance.tla` 373 → 360 (953 lines fewer), and one witness configuration removed. The
rows of « Codex review » C1, C3 and C6 and « Codex re-review » R1-R3 above describe machinery that no longer exists;
their findings are answered by the simpler contract (`gates/OI/REVIEW.md`, « Round 4 »).

## Round 6: one owner at a time, by generation (design decision, 2026-11-13)

Reviewed on the round-6 diff (`cmd/inkwell/open.go`, `openRequests.ts`, the model):

| Finding | Decision |
|---|---|
| Reuse: `internal/bridge.Router.Serve` already replaces its attached connection on a page reload | Same idea, kept local: the open queue's generation is the owner; no shared helper (the router holds per-peer outboxes, the queue holds acknowledged requests: nothing else in common). |
| Simplification: `resend()` (every entry back on any disconnect) replaced by `connect()` and `release(g)` around one `giveBackLocked(g)` | Applied: one rule, « what g held goes back », used at supersede and at the end of g. |
| Simplification: the webview's global `#pending`, `#dropped` and `socket` check | Replaced by one `Session` per connection: retiring it discards its state, so nothing can cross from one stream to the next. |
| Efficiency: `take` and `handled` scan the queue (at most 64) | Kept: linear in a bounded 64, under one mutex, once per request; no index to keep in sync. |
| Altitude: the generation could live in the wire frame (`{"id","gen"}`) | Rejected: the generation is the stream's identity on the Go side; the webview never needs to name it, a report is attributed to the stream it arrives on. |

## Tooling found on the way

| Finding | Decision |
|---|---|
| `mise run mutate:go -- ./internal/launch` aborted before the first mutant: under memcap.sh's 1.8 GB address-space cap, the cgo test binary (it links `net`) failed `pthread_create` after about ten threads, each holding a 64 MB glibc malloc arena of address space | **Fixed at the root** in `scripts/memcap.sh`: `MALLOC_ARENA_MAX=2` (unless set) before the cap; same run then passes with 96.7 % coverage. The cap and its value are unchanged (override entry in `gates/guard-overrides.json`). |
| gremlins v0.6.0 on `./cmd/inkwell` (restricted to `open.go` with `-E`, 2026-11-13) reports every covered mutant « lived » in 1.3 s, i.e. without running the tests: the boundary mutant `open.go:97` (`>=` → `>`) it calls lived is killed by `TestOpenQueueBounded` and `TestOpenQueueOwnerHistories` when applied by hand | Not usable on this `main` package; `open.go` is mutation-tested by hand instead (six mutants, each red, `gates/OI/REVIEW.md` rows G2 and G6). The gate's mutation criterion (`internal/launch`, `internal/instance`) is unaffected. |
| `mise run arch` refused the planned `raise_linux.go` (cgo) | First escalated and kept as a limitation; **decided 2026-11-13, redone 2026-11-14 on the user's decision**: no cgo exception, `internal/activate` uses purego with `RTLD_NOLOAD` and `tools/archexec` is unchanged (ADR 0037, amendment of 2026-11-14). |

## Mutation

gremlins v0.6.0 (`mise run mutate:go`, 4 workers, memory-capped), 2026-11-12:

| Package | Killed | Lived | Not covered | Timed out | Efficacy |
|---|---|---|---|---|---|
| `internal/launch` | 35 | 0 | 13 | 0 | 100.00 % |
| `internal/instance` | 83 | 0 | 32 | 0 | 100.00 % |

(Final code, after the round-4 contract simplification; `internal/launch` 35/0. Round 3: instance 93/0 after killing 3 survivors (the run before: 96.84 %), 2026-11-12. Round 3's first run left 3 survivors on `instance` (96.84 %): the `StartWait` default untested, and the boundary of `time.Since(serving) < StartWait`, equivalent by timing; killed by checking the default in `TestBackoffSchedule` and by comparing against a precomputed `startBy` with `time.Before`. Earlier runs: 34/0 and 83/0, 35/0 and 87/0, 35/0 and 88/0.)

Survivors of the first runs and what killed them (no survivor is accepted as equivalent):
`launch.go` `len(path) >= maxPath` (a relative argument of exactly 4096 bytes once joined, with short folder
names so that only the length can refuse it); `wire.go` `IndexByte(s, 0) < 0` (an argument and a working
directory whose first byte is NUL); `wire.go` the capacity precomputation of `Encode` (six arithmetic mutants
with no observable effect: the precomputation was removed, `append` grows the one frame of a launch).
« Not covered » are mostly `case` conditions, which gremlins does not credit (CLAUDE.md « Pièges connus »), and
the per-OS files not built on Linux (`device_other.go`, `lock_windows.go`).
