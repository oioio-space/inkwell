# W58a - a foreign project is read-only and never offers a broken invitation (R1 blocker)

Status: step 1, **reviewed by Codex (ACCEPT WITH CHANGES, applied: `gates/W58/REVIEW.md` « Round 9 »), ready to freeze**; gate `gates/W58a.toml`. Rigour: **standard**. First half of W58, split on 2026-11-14
(`gates/W58/REVIEW.md` « Round 8 »): exactly what three code-review rounds accepted and what the working tree already
implements, **without any take-over and without any change of project identity** (the join stays as on `main`).
Decision: `docs/adr/0043-foreign-project.md`, part 1. Diagnosis: `docs/plans/2026-11-13-invite-not-admin.md`. The
frozen `gates/W58.toml` stays untouched and superseded. Own worktree (`inkwell-w58`).

## Rules

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| N-1 | `Session.Standing()` is `member`, `removed` (the list holds an op about this device, no role) or `notNamed`, read from the wiring's list, never an older one once the roles event of a join has been emitted; a first-launch creator is admin; the webview asks again on the roles event and after a join | `TestStandingFromList` (table), `TestStandingMemberAfterJoin`, `TestStandingNeverOldAfterRolesEvent`, `TestFirstLaunchCreatorIsAdmin`; Vitest « W58a a join asks for the standing again » | MW1 always `member` -> `N-1 standing` |
| N-2 | Every refusal — a non-member, a non-admin member (a writer), a member demoted between the check and the emission — on **both** `Invite` and `InviteDevice` returns `Invitation{Refused}` (`notMember`, `notAdmin`), no error, no text, and leaves `.inkwell/trust` byte-identical; the webview shows the catalogue sentence and logs nothing from Go; the key crosses the real bridge | `TestNotMemberInviteRefusedByKey` and `TestInviteDemotedBetweenChecksRefusedByKey`, each a table over {`Invite`, `InviteDevice`} × {non-member, writer, demoted} that compares the store's bytes before and after (to complete: today the writer case checks `Invite` only and the demotion does not compare the store); Vitest « W58a a refusal shows its sentence, never Go text »; e2e 1 (a real `Invite` from the page) | MW2 refusal as an error -> `N-2 refused by key`; MW4 `InviteDevice` skips the refusal for a writer -> `N-2 device invitation refused by key`; in-test `String(err)` -> N-2 |
| N-3 | The editor is locked until the rights are known — the standing **and** this device's role reported — for every project: before that nothing can be typed and nothing is saved or sent, while the text shows, can be selected and copied, and no banner flashes (an empty document shows its placeholder). It then unlocks for a member who may write, and stays read-only with the `AccessBanner` sentence for a non-member (`notNamed`, `removed`) | Vitest « W58a the editor is locked until standing and role are known » (`Standing` delayed: typing refused, selection and copy work, no banner; then unlocked for a member, locked with the banner for a non-member), « W58a not a member: read-only, one sentence, one action », « W58a removed: its own sentence », « W58a read-only text can be selected and copied » | in-test: `access` ignores standing -> N-3; lock before the rights removed -> N-3 |
| N-4 | « Inviter » and « Créer une nouvelle invitation » are disabled until an admin role is reported (« Vérification… »), with « Ce document n'est pas à vous sur cet appareil : vous ne pouvez pas inviter. » for a non-member | Vitest « W58a invite capability: loading / admin / non-admin / not a member » on the mounted `ShareDialog` | in-test: renew guard removed -> N-4 |
| N-15 | The banner has **one** action, « Demander une invitation », which opens the join part of the share dialog with `share.join.foreignHelp`; the block offers no take-over and changes no project identity: no `TakeOver` binding, no confirmation dialog, no project-change entry point in the adapter; a join behaves as on `main` | Vitest « W58a ask for an invitation opens the join dialog », « W58a the banner offers nothing else »; gate criterion `no-take-over` (text, an extra) | in-test: a second action rendered -> N-15 |
| N-16 | The trust store survives a failed replacement: when `opStore.replace` (reached by every join into another project, `invites.go` `adopt`) fails **and its rollback fails too**, the same store, before any reopening, recovers the set-aside list at its next use (`ensureOpen` runs `recoverSwap` when `trust` is missing) and a save then works; an error at each step of `replace` leaves a store that opens with the old or the new list | `TestReplaceFailureAtEachStep` (the former `TestTakeOverFailureAtEachStep`, driven through `replace` directly: an error at each hook; the double failure, replace then rollback, followed by a `save` on the same store that must succeed, then a reopen that loads the old list) | MW3 recovery removed from `ensureOpen` -> `N-16 store usable after a failed replacement` |
| N-10 | End to end, a second profile on the first one's folder: it reads the text, the banner shows, « Inviter » is not offered and a real `Invite` answers `refused: "notMember"`; it asks for an invitation, joins with the first profile's invitation and can then write | Playwright `e2e/foreign-project.spec.ts`: « W58a-E2E a second profile reads and cannot invite », « W58a-E2E a non-member asks for an invitation, joins and can write » | - |

## The lock at start (N-3)

Today `refreshStanding()` is launched without waiting (`workspace.svelte.ts:333`) and `access` allows editing while
the standing and the role are unknown (`:608`, `Editor.svelte:81`): a non-member can type before Go answers, and the
save goes out. The lock closes that window. Cost for a member: the walkthrough measured « Inviter » enabled 323 to
359 ms after the page load (6 loads of 6); the same data unlocks the editor, so a member sees their text at once and
can type about a third of a second later, with nothing moving on screen. Risk named: an end-to-end test that types
right after the page load without waiting would lose its keys silently (`keyboard.type` does not wait for an
editable target, `fill` does); `frontend/e2e/helpers.ts` `editor()` gains a wait for the editable state where a
test types, and the gate's `regression` criterion (join, collab, people) and the whole `e2e` run in step 4 show
any miss.

What « not a member » still writes locally, and why it is accepted: nothing is invited and no attestation is
signed while this device is not a member; the automatic projection still writes its local maintenance files
(`workspace.svelte.ts:444`, `store/projection.ts:187`) in a folder that is the person's (W64). The banner speaks
of the right to edit, not of an immutable folder.

## The stated limit: a person alone

With no take-over, a person whose profile is lost and whose documents are kept, with nobody to ask for an invitation,
**cannot edit or share that document from Inkwell** until PC1. What is true today, and what the banner says without
promising more: `foreign.notNamed` « Ce document vient d'un autre appareil. Vous pouvez le lire et en copier le
texte, mais pas le modifier ni le partager. » The text is readable and can be copied (N-3); the files stay in the
folder. Inkwell has no « new document » command yet, so the copy cannot be pasted into a new Inkwell document
either: this case is the block's stated limit, closed by PC1 (« Reprendre ce document ici »). `foreign.removed`
« Vous ne faites plus partie de ce document. Vous pouvez relire ce que vous aviez reçu. » is unchanged.

Catalogue keys of the block: `foreign.notNamed`, `foreign.removed`, `foreign.ask` « Demander une invitation »,
`share.host.notMember`, `share.host.loading` « Vérification… », `error.host` « Impossible d'inviter pour l'instant.
Réessayez dans un moment. » (no `{reason}`), `share.join.foreignHelp` « Demandez à la personne qui partage ce
document de vous inviter, puis collez son invitation ici. ».

## Working tree: what is removed, what stays

**Removed** (it comes back, in its restart form, with PC1):
- `internal/app/session.go`: `TakeOver`; `invites.go`: `takeOver`, `adoptList`'s `before` hook (back to `main`'s
  `adopt`), the `proj` field, `JoinResult` (`Join` returns the peer as on `main`); `persist.go`: `archive`, `copyOps`,
  the archive steps and constants; `node.go`: `inv.proj`.
- `internal/app/foreign_test.go`: every `TestTakeOver…`, `TestJoinAndTakeOverReportTheProjectChange`,
  `TestNoticesAfterTakeOverHoldOnlyTheCurrentProject` (`TestTakeOverFailureAtEachStep` becomes
  `TestReplaceFailureAtEachStep`).
- `frontend/src/net/WailsNetworkAdapter.ts`: `projectChanged`, `pause`, `resume`, `#redial`, the stream-identity
  guard and their fields; `net/projectChange.test.ts`; `workspace.svelte.ts`: `takeOver`, `#joinQuiet`, the
  `changed` handling; `ui/people/AccessBanner.svelte`: the take-over button, the confirmation dialog and its styles,
  the focus after take-over; `ui/foreign.test.ts`: the take-over tests and the N-9, N-11 in-test mutants;
  `e2e/ux/UX-W58-1.spec.ts`; the take-over tests of `e2e/foreign-project.spec.ts`.
- `i18n/*.json`: `foreign.takeOver`, `foreign.takeOver.confirm.title`, `foreign.takeOver.confirm.body`,
  `foreign.takeOver.done`, `foreign.takeOver.failed`; `scripts/netsim/mutants.toml`: MW3..MW7 of block `W58`.
- `gates/W58/UX.md` and its captures are redone (one action, no dialog).

**Stays**: `Standing` (from the wiring's list) and `refusal`, `Refused` with the demotion mapping; `ensureOpen`'s
recovery; `roster.ts` (`accessOf`, `Standing`), `inviteState`, `mayInvite`, `refreshStanding` (also after a join),
`host()` by key; the banner sentence and « Demander une invitation » with `onAsk`; `ShareDialog.svelte`; the
`__inkwellInvite` e2e hook; `frontend/src/mutant.ts`; the `expect` field of `scripts/netsim/mutants.py` with
`mutants_test.py`; `TestOnlyAnAdminInvites` on the new contract; MW1, MW2 and the new MW3 and MW4 under block `W58a`.

**The two fixes found while the stream was being reopened.** Their causes do not depend on `pause`/`resume`:
`#shared` counted a document outside the admitted scope as shared (the previous root, still synced right after a
join that changes the root, is never answered, so the indicator stays on « checking »); `refreshSync` was not run
when a document handle turned ready. But both were only *observed* after a reopened stream, and `join.spec` and
`collab.spec` pass on `main` without them. Rule: each is kept in this block only if its red test fails on the tree
once `pause`/`resume` are removed (the test and the fix then stay, under N-1's neighbour rule of the sync
indicator, named in `SIMPLIFY.md`); if the test is green without the fix, test and fix are left out and the
observation goes to PR1's notes. Not settled statically.

Interface change: `ux-walkthrough` before the merge (light and dark captures of the banner at full and minimum
width, the share dialog for a non-member, the join from there). Step 7: `PROGRESS.md`, `docs/exigences.md` (W58
stays open until PC1; W64 closed), ADR 0043 part 1 accepted.
