# PC1 - the project changes by a restart, never live (mandatory before R1)

Status: **draft, to be reviewed after W58a is merged.** Rigour: **complete** (three or more concurrent actors on a
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
| N-9 | « Reprendre ce document ici » asks one confirmation that says Inkwell restarts; the webview reads the startup id (N-12), asks Go once, shows « Redémarrage… », closes its bridge streams and never reopens them; it polls the startup id and, when the answer is a different id, reloads the page (in the desktop build the webview ends with the process). « Demander une invitation » opens the join part of the share dialog; joining is unchanged and **not made safe by this block** (R-6) | Vitest « W58 take-over asks once and says that Inkwell restarts », « W58 take-over is asked of Go once and the page waits », « W58 the page never reopens the bridge of the image it started with », « W58 the page reloads only on another startup id », « W58 ask for an invitation opens the join dialog » | in-test: confirmation skipped -> N-9; reload on the same id -> N-9 |
| N-10 | End to end (assertions on the startup id, never on timing), a second profile on the first one's folder: reads and cannot invite; takes over, Inkwell restarts on the same document, writable, the text has the focus, the notice shows once; a third profile joins and receives the earlier text; the first profile's later launch is a readable non-member state | Playwright `e2e/foreign-project.spec.ts`: « W58-E2E a second profile reads and cannot invite », « …the take-over restarts Inkwell on the same document, writable », « …a third profile joins after the take-over and receives the earlier text », « …the first profile then opens the folder as a readable non-member » | - |
| N-11 | Nothing of the old session survives a take-over, and the project of a running session is never changed by it: from the request to the end of the process the wiring adopts nothing and its `ProjectID` stays the old one; the new identity exists only in the next process image; after the restart the page holds no attestation and no pending frame of the old project | `TestTakeOverNeverAdoptsBeforeShutdown` (a sentinel counts every adoption of the wiring and records the order of `Wiring.Close`, the archive and `replace`: zero adoptions, write after `Wiring.Close`), `TestTakeOverIdentityOnlyAtNextStart`; e2e « W58-E2E nothing of the old session survives the restart » (the startup id differs; a marker set in the page, an attestation and a frame of the old project held back by the test are absent or refused after) ; gate criterion `no-live-change` (text check, an extra) | MW9 the adoption moved into a helper that `takeOver` calls -> `N-11 no adoption before shutdown` |
| N-12 | The restart. Each process image draws a random 128-bit **startup id** (`Session.StartupID()`, also in the e2e observe hook). `startup` restarts only when this image recorded a take-over request, once, after `stopAll` returned and N-14's barrier held. On Linux it executes `/proc/self/exe` — the running image itself, whatever became of its path (replaced or deleted by an upgrade) — with an argument list rebuilt from nothing: a fixed `argv[0]` and the absolute project folder; the environment is the current one without any `INKWELL_AFTER_TAKEOVER`, plus `INKWELL_AFTER_TAKEOVER=1`. The next image reads the variable once, removes it from its environment, and uses it only to choose a message from **the standing it actually loaded**: member -> « Ce document est maintenant le vôtre sur cet appareil. »; not a member -> « Le document n'a pas pu être repris. Rien n'a changé. ». The three outcomes of the write: failure before the install (old list: not a member), replace done, replace done with a finalisation error (`errCommitted`, `persist.go:80-85`: the new list is what loads: member, the error is logged). If the execution fails the runtime is already closed: the process notifies « Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau. » and exits with the error code; the list is on disk and the next launch loads it | `TestRestartOnlyAfterARequestAndOnce`, `TestRestartAfterStopAllReturned`, `TestRestartArgvAndEnvRebuilt` (misleading `argv[0]`, a stale variable in the environment), `TestRestartExecsTheRunningImage` (helper binary whose path is replaced, then deleted: the same image restarts, new startup id), `TestRestartFailureIsReported`, `TestTakeOverOutcomeFollowsTheLoadedList` (the three outcomes: standing and message of the next start), `TestStartupNoticeShownOnce`; Vitest « W58 the startup notice shows once and the text has the focus », « W58 a failed take-over says so and stays readable » | MW8 restart before `stopAll` returned -> `N-12 restart after the outcome` |
| N-13 | Shutdown ordering. From the request on, `Join`, `Invite`, `InviteDevice` and `Ask` are refused (the session is leaving). The write starts only when the wiring is closed, the lobby, the invitation host and any pending join have ended, and the blob service and discovery are stopped; it holds the invitation lock; nothing can then save an op to `.inkwell/trust` or sign under the old project; the trust store stays usable between `Wiring.Close` and its own close | `TestNoTrustWriteNorSignatureOnceTheWriteStarts` (a late op from each source — list sync, attestation, lobby, pending join — is refused; counting signer: zero), `TestRequestsRefusedOnceTakeOverRequested`, `TestTrustStoreUsableAfterWiringClose` | MW10 write before the lobby stopped -> `N-13 nothing writes once the write starts` |
| N-14 | Termination barrier before the execution: every child and service the process started has ended (language service and tinymist, typst, the preview and asset servers, the peer-to-peer listeners, the instance socket), and every descriptor the process opened is close-on-exec; the new image starts with no descendant, no listening socket and no descriptor inherited beyond the standard three | `TestRestartBarrier` (native, on the server binary with a preview and the language service started: descendants, listening sockets and open descriptors counted before the request and read by the new image at its start; gate criterion `restart-barrier`); the desktop build is checked in the walkthrough (`UX.md` states « orphans: 0 » from a process listing) | MW11 language service left running -> `N-14 no child survives` |

## The join (rules to complete at this block's step 1)

Today a join replaces the identity of a running session for any adoption of another project (`invites.go:379-419`),
and the three races of « Why a restart » apply to it. In this block an accepted invitation into another project
ends as the take-over does: the new list is written during the shutdown, Inkwell restarts on the folder, and the
project of a running session is never changed. To specify, each with its rule, test and mutant: (J-1) the
**pending-join record**, durable before the shutdown, holding what the next start needs to open the inviter's
document — the inviting peer, the project identity, the root document announced in the handshake
(`workspace.svelte.ts` `#projectOf`) or the genesis root when it is not empty (`flow.go:422`), and what becomes of
the text the person had (today it is orphaned, residual of the first-minute lot); (J-2) what the person sees — one
sentence before (« Inkwell va redémarrer pour ouvrir ce document. »), the window on the joined document after, the
notice once; (J-3) a first launch that joins before writing anything: whether it restarts too (uniform rule) or is
the one live case (no other member, no attestation of anyone else can exist) — to decide with a measurement of the
join's duration, the main flow of the application; (J-4) a failed or interrupted join leaves the previous project
as it was; (J-5) the same join into the same project stays a no-op. The e2e `join.spec.ts` and `collab.spec.ts`
follow the restart by the startup id.

## Formal model

`docs/protocol/tla/ProjectChange.tla` (agent `formal-modeler`, at step 1): the page (coverage, bridge open or
closed, startup id seen), the Go session (list in memory, event buffer, wiring open or closed), the list on disk
(old, staged, new, archive), one peer that attests under the old and the new project, and a crash at any step.
Safety: `NoAdmissionAcrossProjects` (nothing attested only under the old project is admitted under the new one),
`NothingOfNewLost`, `WriteAfterWiringStopped`, `RestartAfterOutcome`, `AtMostOneRestartPerRequest`,
`MessageMatchesDisk`; liveness: `RequestEndsInAStart`. Mutants, each caught by its property: `LiveAdopt`,
`ResetAtReply`, `PauseThenReplay` (the three rejected designs), `WriteBeforeWiringStopped`, `RestartBeforeOutcome`,
`ReconnectToOldImage`, `CarriedOutcome`, `RestartLoop`. Witnesses: an honest take-over, a crash after the install,
a join into another project.

## Working tree

W58a removes the take-over code written for revision 6; this block writes it again in its restart form: the request
(`Session.TakeOver` → `TakeOverResult{Restarting}`), `finishTakeOver` in `Runtime.Close`, `archive` and `copyOps`
(their tests `TestTakeOverArchivesOldList`, `…LeavesTextAndFilesIdentical`, `…CrashAtEachStep`, `…NoOpForMember`,
`…IdDiffersFromOwnProject` come back from the revision-6 tree, kept under `gates/W58/` history in git), the banner's
second action and its confirmation, `cmd/inkwell/startup.go` with `restart_unix.go` and `restart_other.go`,
`Session.StartupID` and `Session.StartupNotice`, `.env.reference` (`INKWELL_AFTER_TAKEOVER`), the Go mutants below,
the e2e that follows the restart by the startup id, and a new walkthrough.

## Mutants, wording, residuals

Go mutants MW3..MW11 (numbers kept from revision 8; MW1 and MW2 belong to W58a) in `scripts/netsim/mutants.toml` with `expect` (the runner ties the text to the named test's own
`go test -json` events; a build failure, a timeout or another test is no catch). Interface mutants are in-test copies
with one edit (`frontend/src/mutant.ts`).

Catalogue (the others unchanged): `foreign.takeOver.confirm.body` « Inkwell va redémarrer, puis vous pourrez modifier
et partager ce document. Les personnes qui le partageaient ne seront plus reliées à lui : il faudra leur donner une
nouvelle invitation. Le texte et les fichiers ne changent pas. » · `foreign.takeOver.busy` « Redémarrage… » ·
`foreign.takeOver.done` « Ce document est maintenant le vôtre sur cet appareil. » · `foreign.takeOver.failed` « Le
document n'a pas pu être repris. Rien n'a changé. » · Go notification (French, English): « Inkwell s'est fermé sans
pouvoir redémarrer. Ouvrez-le de nouveau. »

Residuals: R-6 of W58 (the live join) is closed by this block. **R-7** between the release of the profile lock and its re-acquisition by the restarted image another
launch can become the primary; the restarted one then hands its request over and exits (OI rule R10): the document is
taken over on disk and opens at the next launch. **R-8** the restart is specified for Linux; elsewhere
`restart_other.go` reports « unsupported » and the take-over is not offered (macOS and Windows: with their ports). **R-9** after a failed execution the next manual launch opens the most recent project, which may
be another folder; the taken-over one opens normally when chosen. R-1..R-5 as in the ADR.

## To check at implementation (each has its test above; none is assumed)

That `application.App.Quit` runs `OnShutdown` in the server build as in the desktop one (`TestRestartAfterStopAllReturned`
runs on the real `deps`); the time from the confirmation to a writable editor, stated in the UX report (about one
second expected: the start-up median of PB is 351 ms); the supersession of the frozen gate: `scripts/gate.py` has no
notion of a superseded gate (read: `base_guard`, `run`), so the convention is the header of `gates/PC1.toml` and
`gates/W58/REVIEW.md` « Round 6 »; whether an audit expects a result file for every frozen gate was not settled.
The join's rules J-1..J-5 and the model are written at this block's step 1, then reviewed (Codex) before the freeze.
