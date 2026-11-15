# PC1a - take over a document by a restart

Status: **draft, step 1**; split from PC1 on 2026-11-15. Rigour: **complete**. Shared part, restart mechanism (N-11..N-15,
built here), model and residuals: `docs/plans/2026-11-14-pc1-project-change-by-restart.md`. Gate: `gates/PC1a.toml`
(draft). It starts from W58a's tree (standing, refusal by key, read-only banner, invite capability) and adds the
banner's second action, « Reprendre ce document ici », and the restart itself.

## Rules (N-5..N-10; N-11..N-16 in the shared part, implemented by this block)

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| N-5 | `Session.TakeOver()` is a request: for a non-member it signs and validates the genesis in memory (this device, the title, the recorded root URL when valid), records it as pending, answers `{restarting: true}` and asks the application to quit; it writes nothing. `Runtime.Close` then archives the stored ops by copy to `.inkwell/trust-archive/<UTC time>/` and replaces the list with the genesis (`opStore.replace`), under N-13's ordering. At the next start the loaded list names this device: `Invite("writer")` succeeds | `TestTakeOverRequestWritesNothing` (tree hash equal after the call), `TestTakeOverThenNextStartInvites`, `TestTakeOverArchivesOldList` | MW3 archive skipped -> `N-5 old list archived`; MW4 list written at the request -> `N-5 nothing written at the request` |
| N-6 | Nothing else changes: the tree hash of the folder without `.inkwell/trust*` is equal before the request and after the next start | `TestTakeOverLeavesTextAndFilesIdentical` | MW5 `project.json` rewritten -> `N-6 folder unchanged` |
| N-7 | The archive is a copy (written as `.partial`, renamed when whole); a crash or an error at each step (archive, each hook of `replace`, a failed rollback) leaves a folder that opens, as non-member or as taken over, and the next save works (`ensureOpen` recovers a missing `trust`) | `TestTakeOverCrashAtEachStep`, `TestTakeOverFailureAtEachStep` | MW6 archive by moving the ops -> `N-7 archive is a copy` |
| N-8 | A member's `TakeOver` answers `{restarting: false}` and does nothing; the new `ProjectID` differs from that of a project this device created with the same title | `TestTakeOverNoOpForMember`, `TestTakeOverIdDiffersFromOwnProject` | MW7 empty root in the genesis -> `N-8 distinct project` |
| N-9 | « Reprendre ce document ici » asks one confirmation that says Inkwell restarts; once confirmed, the page follows N-15 (edits flushed, repo closed, context discarded by navigation to the restart page) and the restart page calls `Session.TakeOver()` once; on `{restarting: true}` it shows « Inkwell redémarre… », polls the startup id and loads the application only on another id (in the desktop build the webview ends with the process); on `{restarting: false}` or an error it loads the application again and the notice says « Le document n'a pas pu être repris. Rien n'a changé. ». « Demander une invitation » opens the join part of the share dialog (W58a); the join itself follows J-1..J-6 | Vitest « PC1 take-over asks once and says that Inkwell restarts », « PC1 the restart page asks Go once and waits », « PC1 the restart page reloads only on another startup id », « PC1 a refused take-over reloads the application unchanged » | in-test: confirmation skipped -> N-9; reload on the same id -> N-9 |
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

Go mutants MW3..MW13 (MW3..MW11 numbered in W58 revision 8, MW1 and MW2 belong to W58a; MW12 and MW13 added after Codex's round 1, MW12 redefined after round 2 as « the member list written before the drain », N-16) in `scripts/netsim/mutants.toml`,
block `PC1a`, with `expect` tied to the named test's own `go test -json` events (a build failure, a timeout or another
test is no catch). Interface mutants are in-test copies with one edit (`frontend/src/mutant.ts`).

Catalogue (the others unchanged): `foreign.takeOver.confirm.body` « Inkwell va redémarrer, puis vous pourrez modifier
et partager ce document. Les personnes qui le partageaient ne seront plus reliées à lui : il faudra leur donner une
nouvelle invitation. Le texte et les fichiers ne changent pas. » · `foreign.takeOver.busy` « Inkwell redémarre… » · `change.flushFailed` « Le document n'a pas pu être enregistré. Rien n'a changé. » ·
`foreign.takeOver.done` « Ce document est maintenant le vôtre sur cet appareil. » · `foreign.takeOver.failed` « Le
document n'a pas pu être repris. Rien n'a changé. » · Go notification (French, English): « Inkwell s'est fermé sans
pouvoir redémarrer. Ouvrez-le de nouveau. »

Budget (Codex round 2, decision D-B): the gate file carries `max_minutes = 45`; `scripts/gate.py` refuses to start a
gate whose `timeout` caps sum past it and stops the whole run at that deadline (rule G-1 below). The caps: go 5,
gate-deadline 1, mutant-tool 1, mutants 12 (eleven mutants, about one minute each with the named test only), frontend
3, e2e 4 + 5 (build, run), regression 3 + 5, restart-barrier 2, tla 3 min 20 s (the 23 model runs measured at 132 s); 44 min 20 s in all,
40 s left for the uncapped text checks. The smoke measures each; a cap that does not hold splits the block, it is never raised past the total.

## Tooling rule (built by this block)

| Id | Rule | Test (oracle) |
|---|---|---|
| G-1 | **A gate has a deadline.** A gate file carries a top-level `max_minutes` (at most 45 for a « standard » or « complete » block of method v2). `scripts/gate.py` reads every criterion's `timeout -k K N` caps; it refuses to start when their sum exceeds `max_minutes` × 60 (message naming the sum and the limit), and at run time it stops the criterion running at `start + max_minutes` (its process group gets SIGTERM, then SIGKILL after 10 s), records `deadline` for it and every criterion not run, and fails the gate. A gate file without `max_minutes` is refused, except those listed in `gates/legacy-no-deadline.txt` (the gates committed before this rule; the list is closed: a name added later fails the test) | `scripts/gate_deadline_test.py`: `test_caps_over_max_minutes_refused_before_any_run`, `test_run_stopped_at_the_deadline` (a criterion sleeping past a 1-minute deadline, time controlled: the process group is gone, the rest is `deadline`), `test_gate_without_max_minutes_refused_unless_legacy`, `test_legacy_list_is_closed` |

## Gate change on the user's word (2026-11-15)

The `regression` criterion of this block's frozen gate was changed with the user's explicit agreement: its frontend
part is Biome, svelte-check and `vitest run --changed <merge-base with main>` (the tests related to the files the
block changed), capped at 180 s, plus 30 s for the bindings; the e2e part is unchanged. Reason: the whole Vitest suite
takes 307 s alone on an idle machine (measured 2026-11-15), past the cap and the gate's 45 minutes; method v2 accepts
on the touched packages and the whole suite runs in `mise run ci` at every push. Caps now sum to 2690 s of 2700 s.
