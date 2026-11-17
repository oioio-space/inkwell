# PC1a - blind review and /simplify of the block's diff (2026-11-15)

The diff of the block against `HEAD` (59 files, then the fixes below) was read in full by a reviewer that did not
write it, in two passes: correctness against the plan rows (N-5..N-16, G-1) and /simplify (reuse, simpler shapes,
efficiency, altitude). Codex was unavailable; this is the review that counts for a block of this rigour only until
another family reads it (method v2). Each fix has a named test that was red first for the stated reason.

## Correctness findings

| Id | Finding | Decision |
|---|---|---|
| M1 | R-8 says the take-over is not offered off Linux; the page offered it everywhere and Go executed it (list replaced, then « Inkwell s'est fermé sans pouvoir redémarrer ») | applied: `canRestart` const per platform (`cmd/inkwell/restart_linux.go`, `restart_other.go`) → `app.Options.CanRestart`; `Session.TakeOver` refuses before signing (`errNoRestart`); bound `Session.CanTakeOver`, read with the standing; the banner's second button is under `{#if workspace.canTakeOver}`. Tests: `TestTakeOverRefusedWhereNoRestart`, « PC1 no take-over where Inkwell cannot restart » and its in-test mutant |
| M2 | N-13's test checked three flags, not « a late op from each source, counting signer: zero » | applied: `TestNoTrustWriteNorSignatureOnceTheWriteStarts` now wraps the wiring's signer in a counter and, inside the write, sends a peer's list-sync message, an attestation of heads, the host's admission and a join; each must be refused and the key signs nothing. Shown red with the write moved before `Wiring.Close` (five refusals missing, 2 signatures) |
| M3 | The `docs` criterion needs the ADR status token; 13 mutants under a cap sized for eleven | applied: ADR 0043 status « part 2 PC1a accepted 2026-11-15 »; mutants wall time measured below. The gate file is untouched |
| M4 | N-14 on the desktop build (WebKit's helper processes at the barrier) is unproven | left to the interface walkthrough (`gates/PC1a/UX.md`, « orphans: 0 » from a process listing at the barrier) |
| M5 | `quit()` ran inside the `TakeOver` call: in the server build Wails' shutdown waits 5 s for the calls in flight, that is for the call that asked | applied: `go quit()` once the write is recorded (`internal/app/change.go`); `TestTakeOverRepliesBeforeQuit`. Measured on the e2e (annotation « take-over ms », confirmation → application on the new image, four tests): 5951, 5970, 5922, 5918 ms before; 492, 997, 499, 507 ms after |
| new | `mise run pregate` failed: `syscall.Exec` in `cmd/inkwell` is a process launch outside `internal/sidecar` (`tools/archexec`) | applied: the execution moved to `sidecar.ExecSelf` (`internal/sidecar/selfexec_linux.go`); `cmd/inkwell` keeps the barrier and the rebuilt arguments; `TestRestartExecsTheRunningImage` covers it unchanged |
| m1 | The drain may end the stream before its refusal is out: the page then got « store stream closed » and no sentence | applied: `GoStorageAdapter` reports a write left unanswered at an end-of-stream it did not ask for as refused (reads and its own `close` are not). Test « PC1 a write left unanswered when the stream ends says so » |
| m2 | The « refused » sentence waited for `workspace.start()`, which may never end once the bridge is closed for good | applied: `showStartupNotice(focus, ready)` says the sentence at once; only the focus waits for the document. Test « PC1 a refused take-over says so even when the document does not open » |
| m3 | Loading `restart.html` was the request: a reload, or any navigation to it, asked Go again | applied: one-shot mark in the session storage (`frontend/src/restart/mark.ts`), left by `workspace.leave` just before the navigation, taken by the restart page; without it nothing is asked. Tests « PC1 the restart page asks nothing when opened without the confirmation » (and its mutant), « PC1 the leaving page marks the restart page's one request » |
| m4 | A request still draining when the runtime closed recorded its write afterwards: a restart followed no write | applied: `change.finish` closes the record, a later `request` fails with `errClosing`. `TestChangeRequestLosesToClose` |
| m5 | `children()` counted an unreadable list as « no child »: the barrier held vacuously on a kernel without the lists | applied: an error when no thread's list can be read; an ended thread is still skipped. `TestChildrenUnreadableIsAnError` |
| m6 | `workspace.leave` awaited the repo's shutdown without a bound: a hung close left the page locked and silent | applied: the close has the same 5 s; the page then leaves anyway (Go's drain is the proof). Test « PC1 a repo that does not close does not hold the page » |
| m7 | The plan named `restart_unix.go` and eleven mutants | applied in `docs/plans/2026-11-15-pc1a-take-over.md` |
| W58a | Removing « the banner offers nothing else » lost no assertion (exact list of two, no dialog before the click, both kept in `projectChange.test.ts`), only its mutant | applied: « PC1 mutant N-9 an extra action is caught » |

## /simplify findings

| Finding | Decision |
|---|---|
| S1 `Drain.Begin` returned a `sync.OnceFunc` per bridge frame | applied: `Begin() bool` and `End()`, no allocation; each caller ends once |
| S2 `session.stop(&shutdownLoops{}, …)` in `runPrimary` stopped throwaway loops | applied: the loops are a field of `session`; both callers stop the window's own. `TestRestartWaitsForTheWindowLoops`; anchor of MW8 updated |
| S3 The local `finishTakeOver` shadowed the package function in `Session.TakeOver` | applied: renamed `write`; anchors of MW4 and MW5 updated (`mutants.py --check PC1a`: every edit applies) |
| S4 The literal `"leaving"` in three places | applied: `REASON_LEAVING` in `frontend/src/store/protocol.ts`, used by the adapter and the fake; Go keeps `bridge.ReasonLeaving` |
| `TakeOverDialog.svelte` is a new component | kept: no shared confirmation dialog exists; it uses the tokens, `.btn`/`.sug` and the backdrop of `PasteDialog` |
| `rand.Text()` for the startup id | skipped: the rule and its test ask for 128 bits in hexadecimal |
| `attestedSigners`, `Coverage.signers` only serve the e2e hook | kept: same pattern as `coverageStats` |
| `deps.restartRequested` is a seam used by tests only | kept: the startup tests fake the runtime |
| `opStore.archive`'s `if err = errClosedStore; s.root != nil` | kept: it mirrors `opStore.save` |
| `INKWELL_AFTER_TAKEOVER` and the take-over-only `StartupNotice` | kept: PC1b gives the join its own notice |
| The session storage unavailable makes the take-over do nothing (m3 fails closed) | kept: the Wails webview and the browsers of the e2e have it; the application comes back unchanged |

## Mutants

`timeout -k 10 900 ./scripts/heavy.sh python3 -I scripts/netsim/mutants.py PC1a`, on the final tree, another test run
sharing the machine: all 13 caught (MW3..MW15, each by its named test and expect text), wall time 38 s against the
720 s cap of the `mutants` criterion. Nothing to merge or drop.

## Left open

- M4, above: the desktop barrier, in the walkthrough.
- `python3 scripts/gate.py --base-guard` (last step of `mise run pregate`) reports the block's guarded changes
  (`scripts/gate.py` and its tests, `frontend/vite.config.ts`, the `regression` command of `gates/PC1a.toml` and
  `gates/PC1b.toml`, the flush timer of `workspace.svelte.ts`): entries for the integrator, not edited here.

Secrets: none. Artefacts: none staged.

## After the interface walkthrough (2026-11-15, `UX.md`)

| Finding | Outcome |
|---|---|
| UX-PC1a-1 (blocking): the desktop restart failed at the barrier, which waited for the web view's processes | applied: the barrier ends the remaining children (`sidecar.EndChildren`: child subreaper, SIGTERM, SIGKILL after 100 ms, reaped, 5 s deadline naming the survivors); `children` moved to `internal/sidecar` with its test (process control lives there, `tools/archexec`); `TestEndChildren`, `TestRestartBarrier` extended, MW16, MW17; plan row N-14 reworded; measured on the desktop build (`UX.md`, « Fix of UX-PC1a-1 ») |
| MW11 and MW15 would have survived an active barrier (it would end the service the mutant leaves running) | applied: `TestRestartBarrier` reads what the shutdown left to the barrier and requires only the helper the session does not control; both mutants are still caught by their expect text |
| UX-PC1a-2: the restart page waited for good after a failed restart | applied: it gives up after 15 s with the sentence of N-12 (`change.restartFailed`, fr and en), and still loads the application if an image answers later; Vitest with fake timers and an in-test mutant |
| UX-PC1a-8: a failed restart, then a launch by hand, said nothing | applied: a mark in the profile folder (`restart-failed`), taken by the next launch on that folder; `TestRestartFailedThenLaunchedByHandSaysOnce`. Written with `os.Root.WriteFile`, not atomically: a torn mark matches no folder and says nothing, which is the state before the fix |
| UX-PC1a-4, UX-PC1a-5 | applied for the banner's actions and the dialog: a forced-colours rule on the shared `.btn` and `.sug` (the project had none; it applies to every button of these classes), an edge on the take-over dialog, banner actions that stay inside the banner; one e2e test |

Residuals:

- UX-PC1a-3 (severity 2): the banner's actions come about 19 Tab stops after the toolbar although the banner sits right under
  it. The order is the shell's older structure (W58a's « Demander une invitation » has it too), not this block's: left for the
  shell's lot (S13), not changed here. → UX1
- UX-PC1a-5, the rest: at 360 px the banner itself is squeezed because the file list keeps its width (S13 debt, present
  without the banner); only the clipping of the actions is fixed here. → S13
- UX-PC1a-6 and UX-PC1a-7 (severity 1): not changed. → UX1
- `internal/sidecar`: `TestResidentBudgetKillsWithSIGKILL`, `TestCgroupSessionSharesSliceBound` and
  `TestCgroupSessionCopiesLeafBound` fail under `scripts/heavy.sh` on this machine; replayed with the same command on the main
  checkout, without this block: the same three fail there (2026-11-15). Not caused by this change, not looked into here. → SB2a1
