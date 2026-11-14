# W58 - a project that does not name this device (R1 blocker): one block

**Superseded on 2026-11-14** by `2026-11-14-w58a-foreign-read-only.md` (block W58a) and
`2026-11-14-pc1-project-change-by-restart.md` (block PC1), after three review rounds showed the live project change of
rules N-9 and N-11 below unsound (`gates/W58/REVIEW.md`, rounds 6 to 9). Kept unchanged below because the frozen
`gates/W58.toml` cites it; that gate is never run for acceptance.

Status: step 1 done, revision 6, reviewed twice by Codex, gate frozen on 2026-11-14 (owner's rule W64: `gates/W58/REVIEW.md`
« Round 3 »; Codex on revisions 4 and 5, accepted with changes: « Round 4 », « Round 5 »). Decision: `docs/adr/0043-foreign-project.md`. Diagnosis:
`docs/plans/2026-11-13-invite-not-admin.md`. Rigour: **standard**. Gate: `gates/W58.toml` (frozen;
red witnesses in `gates/W58/WITNESSES.md`). Own worktree.

Out of the block: the copy into a new folder (residual R-4, Projects screen), genesis freshness at every creation
(R-2), the missing-root fallback (R-1), a lint of the launchers (R-3), F16 of the OI plan.

## Rules

| Id | Rule | Test | Mutant -> expected assertion text |
|---|---|---|---|
| N-1 | `Session.Standing()` is `member`, `removed` (the list holds an op whose subject is this device, no role) or `notNamed`, from the loaded list; a first-launch creator is admin; it becomes `member` after a join or a take-over, and the webview is told (roles event) | `TestStandingFromList` (table), `TestStandingMemberAfterJoin`, `TestFirstLaunchCreatorIsAdmin` | MW1 always `member` -> `N-1 standing` |
| N-2 | A non-member's `Invite` returns `Invitation{Refused: "notMember"}` (a non-admin member's: `"notAdmin"`), no error, and `.inkwell/trust` is byte-identical afterwards; the webview shows the catalogue sentence of the key and logs nothing from Go; the key crosses the real Wails bridge | `TestNotMemberInviteRefusedByKey`; Vitest « W58 a refusal shows its sentence, never Go text »; e2e « W58-E2E a second profile reads and cannot invite » (reads the sentence in the real application) | MW2 refusal returned as an error -> `N-2 refused by key`; in-test: `String(err)` restored -> N-2 |
| N-3 | The webview treats a non-member like a reader: `access` is `notNamed` or `removed`, the editor is read-only, `AccessBanner` shows the sentence and the two actions | Vitest « W58 not a member: read-only, one sentence, two actions », « W58 removed: its own sentence » | in-test: `access` ignores standing -> N-3 |
| N-4 | « Inviter » and « Créer une nouvelle invitation » are disabled while the role is not reported (« Vérification… »), enabled for an admin only | Vitest « W58 invite capability: loading / admin / non-admin / not a member » on the mounted `ShareDialog` | in-test: renew guard removed -> N-4 |
| N-5 | Take-over: `Session.TakeOver()` copies every stored op to `.inkwell/trust-archive/<UTC time>/` (a folder nothing rotates or lists), synced, **then** signs a genesis (this device, the project's title, the recorded root URL when valid, else empty) and passes the one-op list to `Wiring.AdoptProject` with `opStore.replace` as its persist step, then renews the invitation host as `adopt` does; afterwards `Invite("writer")` succeeds | `TestTakeOverThenInviteSucceeds`, `TestTakeOverWiringFollows`, `TestTakeOverArchivesOldList` (archived bytes equal the old ops) | MW3 archive skipped -> `N-5 old list archived`; MW4 list replaced without `AdoptProject` -> `N-5 wiring follows the new project` (the test also asserts the wiring's `ProjectID` and that a third device, added by the new list, is admitted) |
| N-6 | A take-over changes nothing else: the tree hash of the folder without `.inkwell/trust`, `trust.next`, `trust.prev` and `trust-archive/` is equal before and after (text, files, binary store, root document, `project.json`) | `TestTakeOverLeavesTextAndFilesIdentical` | MW5 `project.json` rewritten -> `N-6 folder unchanged` |
| N-7 | The archive is a **copy**: the active ops are never moved or deleted before `opStore.replace`, which is what keeps its stage, swap and recovery safe (`persist.go:299-320`, `:382-407`, `:86-94`, `:143-151`). A crash at each step (after the archive, and at each hook of `replace`) leaves a folder that opens, as non-member or as taken over, never broken; the archive is whole in both cases | `TestTakeOverCrashAtEachStep` (the store's existing `step` hook) | MW6 archive made by moving the ops -> `N-7 archive is a copy` |
| N-8 | A member's take-over is a no-op (nothing written, same `ProjectID`); the take-over's `ProjectID` differs from that of a project this device created with the same title | `TestTakeOverNoOpForMember`, `TestTakeOverIdDiffersFromOwnProject` | MW7 empty root in the genesis -> `N-8 distinct project` |
| N-11 | When the `ProjectID` changes (take-over, or a join that keeps the same root document) the adapter is told, replaces its `Coverage`, clears its attestation cache and asks again for an attestation of every local head, under the new project | Vitest « W58 project change renews coverage and re-attests the local heads », « W58 heads attested before the change are asked again » | in-test: `#attested` kept -> N-11 |
| N-9 | « Reprendre ce document ici » asks one confirmation that states the consequence; on success the banner goes and the editor becomes writable without a restart; « Demander une invitation » opens the join part of the share dialog, joining is unchanged | Vitest « W58 take-over asks once and says the consequence », « W58 after take-over the editor is writable », « W58 ask for an invitation opens the join dialog » | in-test: confirmation skipped -> N-9 |
| N-10 | End to end, a second profile on the first one's folder: reads and cannot invite; takes over, invites, a third profile joins after the take-over and receives the earlier text; the first profile's later launch on that folder is a readable non-member state | Playwright `e2e/foreign-project.spec.ts`, three « W58-E2E … » tests | - |

What follows the `ProjectID`, and what the take-over does about each (read on `main` 47b15ee, checked by Codex):
wiring state (admission, list synchronisation, attestations in memory, pending ops, role flags) is reset by
`AdoptProject` (`wiring.go:320-332`); there are no attestations on disk (`attest.go:154-155`); sync caches are cleaned
at each open (`docs.go:103`); the binary service serves the same folder under `w.isActive` (`node.go:197-200`); the
network scope is read dynamically (`node.go:188-191`); the lobby keeps its handler (`invites.go:130`) and the
invitation host is renewed as in `invites.go:377-380`; mDNS is not keyed by it (`mdns.go:44`). The one gap is in the
webview: the adapter keeps `#attested`, `#lastHeads` and `Coverage` (`WailsNetworkAdapter.ts:83-102`), so a head
asked before the change is never asked again (`:384`; `attest.go:130-132`) — rule N-11. A join shares the gap when
the joined project has the same root document (`scopeChanged`, `workspace.svelte.ts:368`, fires on a root change
only), so one mechanism serves both; the adapter's existing `#reattest` (`:389-391`) is the re-asking half.

## Mutants

Go mutants MW1..MW7: entries in `scripts/netsim/mutants.toml` (block `W58`, kind `go`), written with the tests so
that each compiles. `scripts/netsim/mutants.py` (`run_go`, lines 74-80) counts any `--- FAIL` as a catch: this block
adds an optional `expect` field. With it the runner uses `go test -json` and a mutant is caught only if the entry's
named test has a `fail` event **and** an `output` event of that same test holds the `expect` text (each assertion
message starts with its rule tag, such as `N-5 old list archived`); a timeout or a build failure is never a catch;
entries without the field behave as before. A new `scripts/netsim/mutants_test.py`, added to `test:py`, covers:
caught; another test fails; the right test with another message; the collision (name in one test, text in another:
not caught); build failure; timeout. Interface mutants are in-test copies with one edit, anchor asserted once, as in
PR1.

## Wording (catalogue keys; « personne », no jargon)

`foreign.notNamed` « Ce document vient d'un autre appareil. Vous pouvez le lire, mais pas le modifier ni le
partager. » · `foreign.removed` « Vous ne faites plus partie de ce document. Vous pouvez relire ce que vous aviez
reçu. » · `foreign.takeOver` « Reprendre ce document ici » · `foreign.ask` « Demander une invitation » ·
`foreign.takeOver.confirm.title` « Reprendre ce document sur cet appareil ? » · `foreign.takeOver.confirm.body`
« Vous pourrez le modifier et le partager. Les personnes qui partageaient ce document ne seront plus reliées à
lui : il faudra leur donner une nouvelle invitation. Le texte et les fichiers ne changent pas. » ·
`foreign.takeOver.done` « Ce document est maintenant le vôtre sur cet appareil. » · `foreign.takeOver.failed`
« Le document n'a pas pu être repris. Rien n'a changé. » · `share.host.notMember` « Ce document n'est pas à vous sur
cet appareil : vous ne pouvez pas inviter. » · `share.host.loading` « Vérification… » · `error.host` « Impossible
d'inviter pour l'instant. Réessayez dans un moment. » · `share.join.foreignHelp` « Demandez à la personne qui partage
ce document de vous inviter, puis collez son invitation ici. »

## Files

`internal/app/session.go` (`Standing`, `TakeOver`), `invites.go` (`Refused`, take-over beside `adopt`), `persist.go`
(archive of the stored ops); `frontend/src/net/WailsNetworkAdapter.ts` (project change); `frontend/src/ui/people/roster.ts` (`accessOf`), `ui/people/AccessBanner.svelte`
(actions, as `ExternalBanner` does; confirmation with the existing dialog), `ui/ShareDialog.svelte`,
`workspace/workspace.svelte.ts`, `i18n/*.json`; `e2e/foreign-project.spec.ts`; `scripts/netsim/mutants.py`, its test
and `mutants.toml`. Interface change: `ux-walkthrough` (light and dark captures of the banner, the confirmation, the
disabled invite row) before the merge. Step 7: `PROGRESS.md`, `docs/exigences.md` (W58, W64), ADR 0043 accepted.

## Settled, and still to check before the freeze

Settled with Codex's evidence: a joiner goes through the handshake and the announced root, not the genesis root
(`invites.go:300-306`, `workspace.svelte.ts:739-772`, `:416`, `WailsNetworkAdapter.ts:551`); the roles event reaches
the webview after an adoption (`wiring.go:330-332`, `:463-473`, `suspend.go:32-35`, `WailsNetworkAdapter.ts:317-332`,
`workspace.svelte.ts:337-340`); the archive leaves `.inkwell/backups/`, whose rotation takes every name
(`kv.go:307-318`, `:464-470`). Nothing lists the children of `.inkwell/` generically: the store uses the exact names
`trust`, `trust.next`, `trust.prev` (`persist.go:26-49`).

Still measured, not assumed: N-10's third profile receives the earlier text once N-11 is in (the admission boundary
as executed today accepts an attested head and its ancestors, ADR 0043 « Consequences »); how the adapter learns the
`ProjectID` change (the roles event carrying it, or an event of its own) is chosen at implementation, N-11 tests
either.
