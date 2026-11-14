# PC1 - the project changes by a restart, never live (mandatory before R1)

Status: **draft; step 1 written (join rules, model), to be reviewed after W58a is merged.** Rigour: **complete** (three or more concurrent actors on a
durable state: the page, the Go session with its event buffer, the member list on disk, the peers). It carries the
take-over that revisions 7 and 8 of the W58 specification defined (rules N-5..N-14, numbers kept; reviewed once by
Codex on the specification, `gates/W58/REVIEW.md` « Round 7 ») **and the join** (former JN1, residual R-6 of W58):
both need the same restart. Decision: `docs/adr/0043-foreign-project.md`, part 2. Gate: `gates/PC1.toml` (draft). It
starts from the tree of W58a (`docs/plans/2026-11-14-w58a-foreign-read-only.md`): standing, refusal by key, read-only
banner and invite capability are there; this block adds the second banner action, « Reprendre ce document ici ».
The frozen `gates/W58.toml` stays untouched and superseded.

Out of the block: the copy into a new folder (R-4), genesis freshness at every creation (R-2), the missing-root
fallback (R-1), a lint of the launchers (R-3), the restart on macOS and Windows (R-8), F16 of the OI plan.

## Why a restart

Three designs changed the project inside the live session and each lost to a deeper race: (1) reset the adapter when
the bound call returns — the old coverage admits a head attested only under the old project until the reply arrives;
(2) pause the bridge stream, reset, let Go replay — frames already handed to automerge-repo continue after the pause
(`CollectionSynchronizer.receiveMessage` awaits `shareConfig.access` and `repo.find`; `removePeer` cancels nothing);
(3) the same on the Go side — `AdoptProject` clears the attestation store but not the 64-event buffer `w.out`
(`wiring.go:32`, `:191`, `:326`), `RecvEnvelope` revalidates PeerUp and Message only (`:734-739`), `Router.Run` picks
the connection after dequeuing (`router.go:103-111`), so an old `Attested` reaches the new stream after the replay.
A restart needs no ordering argument: the wiring, the bridge buffers, the adapter, the coverage, automerge-repo and
every pending continuation end with the process, and the next one loads only what is on disk.

Facts read in the code: nothing restarts or relaunches today (no `Exec`, no reload). `startup` (`cmd/inkwell/startup.go:69-112`)
claims the profile, runs the window, and returns after `stopAll` closed the network, the project and the instance
(`:145-151`); `/health` of the server build cannot tell an old image from a new one, hence the startup id; `Runtime.Close` stops the wiring before the invitation store (`internal/app/node.go:234-239`): the list
can be written between the two, when nothing can save an op any more. The Wails services are bound once, to one
`Session`, at `application.New` (`cmd/inkwell/main.go:113-121`): a runtime cannot be swapped inside the process, so the
restart is a new process image. `launch.Resolve` opens the project folder given as first argument when it is in the
Inkwell folder (`main.go:221-229`, rule R2 of OI): the restarted Inkwell reopens the same folder. The profile lock is a
`flock` (`internal/instance/lock_unix.go:13-21`), released by `stopAll` before the restart. A non-member is read-only
(N-3): at the time of a take-over the editor holds no text of its own to lose, and the stores are closed by the normal
shutdown. The e2e harness polls `/health` on the instance's port (`frontend/e2e/instance.ts`) and keeps the child's
pid; an exec in place changes neither the pid nor the process start time, so only the startup id proves a restart.

## Rules

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| N-5 | `Session.TakeOver()` is a request: for a non-member it signs and validates the genesis in memory (this device, the title, the recorded root URL when valid), records it as pending, answers `{restarting: true}` and asks the application to quit; it writes nothing. `Runtime.Close` then archives the stored ops by copy to `.inkwell/trust-archive/<UTC time>/` and replaces the list with the genesis (`opStore.replace`), under N-13's ordering. At the next start the loaded list names this device: `Invite("writer")` succeeds | `TestTakeOverRequestWritesNothing` (tree hash equal after the call), `TestTakeOverThenNextStartInvites`, `TestTakeOverArchivesOldList` | MW3 archive skipped -> `N-5 old list archived`; MW4 list written at the request -> `N-5 nothing written at the request` |
| N-6 | Nothing else changes: the tree hash of the folder without `.inkwell/trust*` is equal before the request and after the next start | `TestTakeOverLeavesTextAndFilesIdentical` | MW5 `project.json` rewritten -> `N-6 folder unchanged` |
| N-7 | The archive is a copy (written as `.partial`, renamed when whole); a crash or an error at each step (archive, each hook of `replace`, a failed rollback) leaves a folder that opens, as non-member or as taken over, and the next save works (`ensureOpen` recovers a missing `trust`) | `TestTakeOverCrashAtEachStep`, `TestTakeOverFailureAtEachStep` | MW6 archive by moving the ops -> `N-7 archive is a copy` |
| N-8 | A member's `TakeOver` answers `{restarting: false}` and does nothing; the new `ProjectID` differs from that of a project this device created with the same title | `TestTakeOverNoOpForMember`, `TestTakeOverIdDiffersFromOwnProject` | MW7 empty root in the genesis -> `N-8 distinct project` |
| N-9 | « Reprendre ce document ici » asks one confirmation that says Inkwell restarts; the webview reads the startup id (N-12), asks Go once, shows « Redémarrage… », closes its bridge streams and never reopens them; it polls the startup id and, when the answer is a different id, reloads the page (in the desktop build the webview ends with the process). While it waits the page makes no call but the startup-id poll: every pending continuation and storage write of its heap is dropped with it (model `PageGate`). « Demander une invitation » opens the join part of the share dialog (W58a); the join itself follows J-1..J-5 | Vitest « PC1 take-over asks once and says that Inkwell restarts », « PC1 take-over is asked of Go once and the page waits », « PC1 the page never reopens the bridge of the image it started with », « PC1 the page reloads only on another startup id » | in-test: confirmation skipped -> N-9; reload on the same id -> N-9 |
| N-10 | End to end (assertions on the startup id, never on timing), a second profile on the first one's folder: reads and cannot invite; takes over, Inkwell restarts on the same document, writable, the text has the focus, the notice shows once; a third profile joins and receives the earlier text; the first profile's later launch is a readable non-member state | Playwright `e2e/project-change.spec.ts`: « PC1-E2E the take-over restarts Inkwell on the same document, writable », « PC1-E2E a third profile joins after the take-over and receives the earlier text », « PC1-E2E the first profile then opens the folder as a readable non-member » (the reading half is W58a's) | - |
| N-11 | Nothing of the old session survives a take-over, and the project of a running session is never changed by it: from the request to the end of the process the wiring adopts nothing and its `ProjectID` stays the old one; the new identity exists only in the next process image; after the restart the page holds no attestation and no pending frame of the old project | `TestTakeOverNeverAdoptsBeforeShutdown` (a sentinel counts every adoption of the wiring and records the order of `Wiring.Close`, the archive and `replace`: zero adoptions, write after `Wiring.Close`), `TestTakeOverIdentityOnlyAtNextStart`; e2e « PC1-E2E nothing of the old session survives the restart » (the startup id differs; a marker set in the page, an attestation and a frame of the old project held back by the test are absent or refused after) ; gate criterion `no-live-change` (text check, an extra) | MW9 the adoption moved into a helper that `takeOver` calls -> `N-11 no adoption before shutdown` |
| N-12 | The restart. Each process image draws a random 128-bit **startup id** (`Session.StartupID()`, also in the e2e observe hook). `startup` restarts only when this image recorded a take-over request, once, after `stopAll` returned and N-14's barrier held. On Linux it executes `/proc/self/exe` — the running image itself, whatever became of its path (replaced or deleted by an upgrade) — with an argument list rebuilt from nothing: a fixed `argv[0]` and the absolute project folder; the environment is the current one without any `INKWELL_AFTER_TAKEOVER`, plus `INKWELL_AFTER_TAKEOVER=1`. The next image reads the variable once, removes it from its environment, and uses it only to choose a message from **the standing it actually loaded**: member -> « Ce document est maintenant le vôtre sur cet appareil. »; not a member -> « Le document n'a pas pu être repris. Rien n'a changé. ». The three outcomes of the write: failure before the install (old list: not a member), replace done, replace done with a finalisation error (`errCommitted`, `persist.go:80-85`: the new list is what loads: member, the error is logged). If the execution fails the runtime is already closed: the process notifies « Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau. » and exits with the error code; the list is on disk and the next launch loads it | `TestRestartOnlyAfterARequestAndOnce`, `TestRestartAfterStopAllReturned`, `TestRestartArgvAndEnvRebuilt` (misleading `argv[0]`, a stale variable in the environment), `TestRestartExecsTheRunningImage` (helper binary whose path is replaced, then deleted: the same image restarts, new startup id), `TestRestartFailureIsReported`, `TestTakeOverOutcomeFollowsTheLoadedList` (the three outcomes: standing and message of the next start), `TestStartupNoticeShownOnce`; Vitest « PC1 the startup notice shows once and the text has the focus », « PC1 a failed take-over says so and stays readable » | MW8 restart before `stopAll` returned -> `N-12 restart after the outcome` |
| N-13 | Shutdown ordering. From the request on, `Join`, `Invite`, `InviteDevice` and `Ask` are refused (the session is leaving). The write starts only when the wiring is closed, the lobby, the invitation host and any pending join have ended, and the blob service and discovery are stopped; it holds the invitation lock; nothing can then save an op to `.inkwell/trust` or sign under the old project; the trust store stays usable between `Wiring.Close` and its own close | `TestNoTrustWriteNorSignatureOnceTheWriteStarts` (a late op from each source — list sync, attestation, lobby, pending join — is refused; counting signer: zero), `TestRequestsRefusedOnceTakeOverRequested`, `TestTrustStoreUsableAfterWiringClose` | MW10 write before the lobby stopped -> `N-13 nothing writes once the write starts` |
| N-14 | Termination barrier before the execution: every child and service the process started has ended (language service and tinymist, typst, the preview and asset servers, the peer-to-peer listeners, the instance socket), and every descriptor the process opened is close-on-exec; the new image starts with no descendant, no listening socket and no descriptor inherited beyond the standard three | `TestRestartBarrier` (native, on the server binary with a preview and the language service started: descendants, listening sockets and open descriptors counted before the request and read by the new image at its start; gate criterion `restart-barrier`); the desktop build is checked in the walkthrough (`UX.md` states « orphans: 0 » from a process listing) | MW11 language service left running -> `N-14 no child survives` |

## The join (rules J-1..J-5, written 2026-11-15)

Today a join replaces the identity of a running session for any adoption of another project (`invites.go:379-419`),
and the three races of « Why a restart » apply to it. From this block an accepted invitation into another project ends
as the take-over does: nothing is written at the request, the new list is written during the shutdown under N-13's
ordering, Inkwell restarts on the same folder (N-12, N-14), and the project of a running session is never changed.
The join stays in the current folder, as on `main` (a new folder per join belongs to the Projects screen, R-4).

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| J-1 | **The pending-join record.** `Join` and `Ask` into another project verify the Welcome as today (`invite.verify`), keep in memory the new list, the inviter (peer id, addresses), the project id and the root (the genesis root when not empty, `flow.go:422`; empty otherwise, which is every genesis Inkwell signs today, `persist.go:172`), answer `{restarting: true}` and ask the application to quit; nothing is written at the request. During the shutdown, once N-13's write may start, the record is written **first**, as the field `pendingJoin` of `.inkwell/project.json` (`{v: 1, id: 128-bit random, project, inviter, addrs, root}`, by the existing atomic write: temporary file, fsync, rename, directory sync, `store.WriteAtomic`), then the old list is archived by copy and replaced as N-5 does. At the next start, when the loaded list's `ProjectID` is the record's, the inviter's addresses go to the address book and the page opens the record's root, or waits for the root announced by a member of the loaded list (the inviter's handshake, `workspace.svelte.ts` `#projectOf`); once `repo.find` has the document and it is flushed to storage, `Session.CompleteJoin(url, peer)` (refused for a non-member peer, an invalid URL, a root other than the record's when it holds one, or no record) rewrites `project.json` once, atomically: `rootUrl` set, record removed. That rewrite is the consumption: it happens once, and before it no root is recorded, so no start can replace the joined root by a fresh document. The text the folder had stays in its storage, no longer shown (as on `main`; R-11) | `TestJoinRequestWritesNothing` (tree hash equal after the call), `TestJoinWritesRecordBeforeTheList` (order of the hooks), `TestJoinCrashAtEachStep` (record, each hook of `replace`, after the install, after the consuming rewrite: the next start opens the previous document unchanged or waits for the joined one, never the old root under the new list), `TestCompleteJoinConsumesOnce`, `TestCompleteJoinRefuses` (non-member, invalid URL, other root, no record) | MJ1 the list installed before the record -> `J-1 record before the list`; MJ2 the record kept by `CompleteJoin` -> `J-1 consumed once` |
| J-2 | **What the person sees.** Under « Rejoindre », one sentence: « Inkwell va redémarrer pour ouvrir ce document. », and when the current document has text a second one: « Le document ouvert maintenant reste dans le dossier mais ne sera plus affiché. »; after « Rejoindre » the dialog says « Redémarrage… » and the page behaves as N-9 (bridge closed, no call but the startup-id poll, reload on another id). After the restart, while the record waits: « En attente de {name} pour ouvrir le document… » in place of the editor, as long as needed (every start waits again; the fresh-document fallback of `workspace.svelte.ts:389-397` never runs while a record exists); then the joined document, writable once standing and role are known (W58a's lock), and « Vous avez rejoint ce document. » once, shown by the image that consumed the record | Vitest « PC1 join says that Inkwell restarts », « PC1 a pending join waits for its root and never creates a document », « PC1 the joined notice shows once »; Playwright « PC1-E2E a join restarts Inkwell on the document of the inviter » (startup id differs, the inviter's text shows, writable), « PC1-E2E a pending join waits while the inviter is away » | in-test: fresh-document fallback while a record exists -> J-2 |
| J-3 | **A first launch that joins restarts too** (one rule, no live case). Measured: a join reads « Synchronisé » 1.1 s to 1.7 s after « Rejoindre » (`gates/SI1/UX.md`, `gates/SI1b/UX.md`, server build); a start of the server build takes 351 ms to 382 ms (`docs/plans/2026-11-11-startup-speed.md` §2): the restart adds about a quarter to the join. A live case would need « nobody else's attestation can exist » checked against the running wiring, the very ordering argument the restart removes, and a second path to test. The native window's start is not measured (same plan, §6): the UX report states the confirmation-to-writable time of a first-launch join | the J-2 Playwright test on a first-launch profile; UX report line « first-launch join: N ms » | - |
| J-4 | **A failed or interrupted join leaves the previous project as it was.** A failure or a crash before the install rename (record, stage, set aside, the rename, a failed rollback) leaves the old list; the next start finds the record with a list of another project, removes it (atomic rewrite), says « L'invitation n'a pas pu être terminée. Rien n'a changé. » and opens the previous document. The inviter's list already names this device (the host admits before the Welcome), so a new invitation from that person works (`admit` returns the Welcome to a member, `flow.go:411-416`). After the install (`errCommitted` included) the joined project loads; a failed execution is N-12's | `TestJoinOutcomeFollowsTheLoadedList` (three outcomes: standing, record and message of the next start), `TestJoinFailedRecordDropped` | MJ3 a failed join keeps its record -> `J-4 nothing changed` |
| J-5 | A join into the project the session already runs answers `{restarting: false}`, writes nothing and does not restart (`adopt`'s early return, `invites.go:381-383`) | `TestJoinSameProjectNoRestart` | MJ4 restart on the same project -> `J-5 no-op` |

## Formal model

`docs/protocol/tla/ProjectChange.tla`, mutants `mutants/ProjectChangeMutants.tla`, registered in the README section
« ProjectChange » and in `scripts/tla-check.sh` (which also checks that TLC names the expected property). Actors: the
Go runtime of each process image (wiring and its event buffer, trust store, invitation host; shutdown, write,
barrier, restart, start), the page (coverage, automerge-repo continuations, root, startup id; the server build's tab,
which outlives the process), the restart by `Platform` (`exec` in place, or `spawn` of a process that claims the
profile lock, N35: the macOS and Windows ports implement the same rules), the environment (a peer, the person, a member
that gives the joined root, children, crashes, failed steps). Durable: the list and its staging siblings, the
record, `rootUrl`; everything else is lost by a crash or a restart. Safety: `NoOldStateAfterChange` (N-11, N-13,
N-14, J-1), `NewStateSurvives` (N-7, J-4), `RestartOnce`, `WriteBeforeRestart` (N-12, N-13), `OpensAfterCrash`
(N-7, J-1, J-4), `JoinOnce` (J-1, J-2), `OneImage` (N-12, N35), `NoticeMatchesStanding` (N-12, J-2); liveness
`ChangeCompletes` under three weak-fairness terms. Four sound configurations (exec and spawn; one or two crashes and
failed steps) hold exhaustively, 14,538 to 124,758 distinct states, 1 to 4 s each; twelve mutants (`LiveAdopt`,
`RestartBeforeDurable`, `RestartTwice`, `OldContinuation`, `JoinTwice`, `WriteBeforeWiringStopped`, `NoBarrier`,
`SpawnNoWait`, `JoinRecordAfterInstall`, `RecoverPrefersAside`, `CarriedOutcome`, `PageNeverReloads`) are each caught
by the property of their row, and four witnesses (take-over, join with an announced root, crash after the install,
failed write) are reached.

## Working tree

W58a removes the take-over code written for revision 6; this block writes it again in its restart form: the request
(`Session.TakeOver` → `TakeOverResult{Restarting}`), `finishTakeOver` in `Runtime.Close`, `archive` and `copyOps`
(their tests `TestTakeOverArchivesOldList`, `…LeavesTextAndFilesIdentical`, `…CrashAtEachStep`, `…NoOpForMember`,
`…IdDiffersFromOwnProject` come back from the revision-6 tree, kept under `gates/W58/` history in git), the banner's
second action and its confirmation, `cmd/inkwell/startup.go` with `restart_unix.go` and `restart_other.go`,
`Session.StartupID` and `Session.StartupNotice`, `.env.reference` (`INKWELL_AFTER_TAKEOVER`), the Go mutants below,
the e2e that follows the restart by the startup id, and a new walkthrough.

## Mutants, wording, residuals

Go mutants MW3..MW11 (numbers kept from revision 8; MW1 and MW2 belong to W58a) and MJ1..MJ4 (the join) in `scripts/netsim/mutants.toml` with `expect` (the runner ties the text to the named test's own
`go test -json` events; a build failure, a timeout or another test is no catch). Interface mutants are in-test copies
with one edit (`frontend/src/mutant.ts`).

Catalogue (the others unchanged): `foreign.takeOver.confirm.body` « Inkwell va redémarrer, puis vous pourrez modifier
et partager ce document. Les personnes qui le partageaient ne seront plus reliées à lui : il faudra leur donner une
nouvelle invitation. Le texte et les fichiers ne changent pas. » · `foreign.takeOver.busy` « Redémarrage… » ·
`foreign.takeOver.done` « Ce document est maintenant le vôtre sur cet appareil. » · `foreign.takeOver.failed` « Le
document n'a pas pu être repris. Rien n'a changé. » · Go notification (French, English): « Inkwell s'est fermé sans
pouvoir redémarrer. Ouvrez-le de nouveau. » · `join.restart.note` « Inkwell va redémarrer pour
ouvrir ce document. » · `join.restart.replaces` « Le document ouvert maintenant reste dans le dossier mais ne sera plus
affiché. » · `join.waitingRoot` « En attente de {name} pour ouvrir le document… » · `join.done` « Vous avez rejoint ce
document. » · `join.failed` « L'invitation n'a pas pu être terminée. Rien n'a changé. »

Residuals: R-6 of W58 (the live join) is closed by this block. **R-7** between the release of the profile lock and its re-acquisition by the restarted image another
launch can become the primary; the restarted one then hands its request over and exits (OI rule R10): the document is
taken over on disk and opens at the next launch. **R-8** the restart is specified for Linux; elsewhere
`restart_other.go` reports « unsupported » and the take-over is not offered (macOS and Windows: with their ports, which
implement the model's `spawn` restart, N35). A join there writes as on Linux, then Inkwell closes and asks to be opened
again (the person is the restart; the model's `Launch`): no platform keeps a live join. **R-9** after a failed execution the next manual launch opens the most recent project, which may
be another folder; the taken-over one opens normally when chosen. **R-10** a joined project whose members all stay away waits
(J-2): leaving it belongs to the Projects screen (R-4). **R-11** joining from a folder that holds text keeps that text in
the folder, no longer shown (as on `main`); the old list is in `trust-archive/`. R-1..R-5 as in the ADR.

## To check at implementation (each has its test above; none is assumed)

That `application.App.Quit` runs `OnShutdown` in the server build as in the desktop one (`TestRestartAfterStopAllReturned`
runs on the real `deps`); the time from the confirmation to a writable editor, stated in the UX report (about one
second expected: the start-up median of PB is 351 ms); the supersession of the frozen gate: `scripts/gate.py` has no
notion of a superseded gate (read: `base_guard`, `run`), so the convention is the header of `gates/PC1.toml` and
`gates/W58/REVIEW.md` « Round 6 »; whether an audit expects a result file for every frozen gate was not settled.
The join's rules J-1..J-5 and the model were written at step 1 (2026-11-15); Codex reviews them before the freeze.

Sources of step 1 (fetched 2026-11-15). systemd « offline updates »
(https://www.freedesktop.org/software/systemd/man/latest/systemd.offline-updates.html): a durable trigger consumed by
the next boot, removed « as early as possible » so that a failed update does not loop. J-1 takes the trigger and
rejects the early removal: removing the record before the root is held would let a crash open the old text under the
new list. A loop cannot happen here anyway, since consuming the record never restarts (`RestartOnce`). Electron's
relauncher (https://github.com/electron/electron/blob/main/shell/browser/relauncher.h): the parent starts a relauncher
that waits for the parent to exit before it launches the new image. The model's `spawn` gets that wait from ADR 0037's
profile lock instead of a pipe, and `SpawnNoWait` is that wait missing. Raft's TLA+ specification
(https://github.com/ongardie/raft.tla, `Restart`: « it loses everything but » stable storage): the idiom used here for
a crash and for a restart.
