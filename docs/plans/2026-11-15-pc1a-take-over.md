# PC1a - take over a document by a restart

Status: **draft, step 1**; split from PC1 on 2026-11-15. Rigour: **complete**. Shared part, restart mechanism (N-11..N-14,
built here), model and residuals: `docs/plans/2026-11-14-pc1-project-change-by-restart.md`. Gate: `gates/PC1a.toml`
(draft). It starts from W58a's tree (standing, refusal by key, read-only banner, invite capability) and adds the
banner's second action, « Reprendre ce document ici », and the restart itself.

## Rules (N-5..N-10; N-11..N-14 in the shared part, implemented by this block)

## Rules

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| N-5 | `Session.TakeOver()` is a request: for a non-member it signs and validates the genesis in memory (this device, the title, the recorded root URL when valid), records it as pending, answers `{restarting: true}` and asks the application to quit; it writes nothing. `Runtime.Close` then archives the stored ops by copy to `.inkwell/trust-archive/<UTC time>/` and replaces the list with the genesis (`opStore.replace`), under N-13's ordering. At the next start the loaded list names this device: `Invite("writer")` succeeds | `TestTakeOverRequestWritesNothing` (tree hash equal after the call), `TestTakeOverThenNextStartInvites`, `TestTakeOverArchivesOldList` | MW3 archive skipped -> `N-5 old list archived`; MW4 list written at the request -> `N-5 nothing written at the request` |
| N-6 | Nothing else changes: the tree hash of the folder without `.inkwell/trust*` is equal before the request and after the next start | `TestTakeOverLeavesTextAndFilesIdentical` | MW5 `project.json` rewritten -> `N-6 folder unchanged` |
| N-7 | The archive is a copy (written as `.partial`, renamed when whole); a crash or an error at each step (archive, each hook of `replace`, a failed rollback) leaves a folder that opens, as non-member or as taken over, and the next save works (`ensureOpen` recovers a missing `trust`) | `TestTakeOverCrashAtEachStep`, `TestTakeOverFailureAtEachStep` | MW6 archive by moving the ops -> `N-7 archive is a copy` |
| N-8 | A member's `TakeOver` answers `{restarting: false}` and does nothing; the new `ProjectID` differs from that of a project this device created with the same title | `TestTakeOverNoOpForMember`, `TestTakeOverIdDiffersFromOwnProject` | MW7 empty root in the genesis -> `N-8 distinct project` |
| N-9 | « Reprendre ce document ici » asks one confirmation that says Inkwell restarts; the webview reads the startup id (N-12), asks Go once, shows « Redémarrage… », closes its bridge streams and never reopens them; it polls the startup id and, when the answer is a different id, reloads the page (in the desktop build the webview ends with the process). While it waits the page makes no call but the startup-id poll: every pending continuation and storage write of its heap is dropped with it (model `PageGate`). « Demander une invitation » opens the join part of the share dialog (W58a); the join itself follows J-1..J-5 | Vitest « PC1 take-over asks once and says that Inkwell restarts », « PC1 take-over is asked of Go once and the page waits », « PC1 the page never reopens the bridge of the image it started with », « PC1 the page reloads only on another startup id » | in-test: confirmation skipped -> N-9; reload on the same id -> N-9 |
| N-10 | End to end (assertions on the startup id, never on timing), a second profile on the first one's folder: reads and cannot invite; takes over, Inkwell restarts on the same document, writable, the text has the focus, the notice shows once; a third profile joins and receives the earlier text; the first profile's later launch is a readable non-member state | Playwright `e2e/project-change.spec.ts`: « PC1-E2E the take-over restarts Inkwell on the same document, writable », « PC1-E2E a third profile joins after the take-over and receives the earlier text », « PC1-E2E the first profile then opens the folder as a readable non-member » (the reading half is W58a's) | - |

## Working tree

W58a removes the take-over code written for revision 6; this block writes it again in its restart form: the request
(`Session.TakeOver` → `TakeOverResult{Restarting}`), `finishTakeOver` in `Runtime.Close`, `archive` and `copyOps`
(their tests `TestTakeOverArchivesOldList`, `…LeavesTextAndFilesIdentical`, `…CrashAtEachStep`, `…NoOpForMember`,
`…IdDiffersFromOwnProject` come back from the revision-6 tree, kept under `gates/W58/` history in git), the banner's
second action and its confirmation, `cmd/inkwell/startup.go` with `restart_unix.go` and `restart_other.go`,
`Session.StartupID` and `Session.StartupNotice`, `.env.reference` (`INKWELL_AFTER_TAKEOVER`), the Go mutants below,
the e2e that follows the restart by the startup id, and a new walkthrough.

## Mutants and wording

Go mutants MW3..MW11 (numbers kept from W58 revision 8; MW1 and MW2 belong to W58a) in `scripts/netsim/mutants.toml`,
block `PC1a`, with `expect` tied to the named test's own `go test -json` events (a build failure, a timeout or another
test is no catch). Interface mutants are in-test copies with one edit (`frontend/src/mutant.ts`).

Catalogue (the others unchanged): `foreign.takeOver.confirm.body` « Inkwell va redémarrer, puis vous pourrez modifier
et partager ce document. Les personnes qui le partageaient ne seront plus reliées à lui : il faudra leur donner une
nouvelle invitation. Le texte et les fichiers ne changent pas. » · `foreign.takeOver.busy` « Redémarrage… » ·
`foreign.takeOver.done` « Ce document est maintenant le vôtre sur cet appareil. » · `foreign.takeOver.failed` « Le
document n'a pas pu être repris. Rien n'a changé. » · Go notification (French, English): « Inkwell s'est fermé sans
pouvoir redémarrer. Ouvrez-le de nouveau. »

Durations (estimates, measured at the smoke; cap 45 min): docs 1 s, go 6 min, mutant-tool 1 min, mutants 15 min,
frontend 4 min, e2e 8 min, regression 8 min, restart-barrier 2 min, tla 1 min.
