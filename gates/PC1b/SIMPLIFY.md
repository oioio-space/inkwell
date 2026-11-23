# PC1b - /simplify and blind review of the block's diff

Reviewer: an Opus-class session that did not write the block (2026-11-15). Scope: `git diff HEAD` plus the untracked
files of the worktree `inkwell-pc1b` (35 files changed, 5 new). Read against `docs/plans/2026-11-15-pc1b-join.md`
(J-1..J-6, MJ1..MJ16), the shared plan (N-12..N-16, R-1..R-11), ADR 0043 part 2, `gates/PC1/REVIEW.md` and the frozen
gate `gates/PC1b.toml`. Phase 1 below was written before any change; phase 2 says what was applied.

## Phase 1 - findings (blind, read-only)

### A. Correctness, row by row

| Row | Implemented as written | Oracle (test -> mutant) | Remark |
|---|---|---|---|
| J-1 | yes: `invites.hold` writes nothing; `finishJoin` marker -> staging -> record -> list -> rename; `Session.CompleteJoin` five refusals, one atomic rewrite | `TestJoinRequestWritesNothing`; `TestJoinWritesRecordBeforeTheList` -> MJ1; `TestJoinCrashAtEachStep`; `TestCompleteJoinConsumesOnce` -> MJ2; `TestCompleteJoinRefuses` -> MJ6..MJ9; `TestCompleteJoinAfterTheDocumentIsStored` -> MJ10; Vitest order test -> in-test mutant | the peer of `CompleteJoin` is checked against the list this image loaded, which is the joined one (`joinRecord` compares the record's project with it): a device of the previous list cannot pass. See M1 for the page side |
| J-2 | yes, but see M1 (which root the wait keeps) and M2 (the name shown) | Vitest « PC1 join says… », « a pending join waits… » -> in-test mutant, « the joined notice shows once »; two Playwright tests | the wait has no end by design (R-10): see « Walkthrough points » |
| J-3 | yes: one path, no live case | `TestFirstLaunchJoinRestarts` (child process, same pid, other startup id) -> MJ11 | — |
| J-4 | yes: `recoverJoin` at `Start`, `join.failed` once | `TestJoinOutcomeFollowsTheLoadedList` -> MJ5; `TestJoinFailedRecordDropped` -> MJ3 | a start whose `recoverJoin` failed, then a new join in the same image, overwrites the marker: the older staging folder is then named by nothing (m8, residual) → PC1b |
| J-5 | yes | `TestJoinSameProjectNoRestart` -> MJ4 | — |
| J-6 | yes for confinement, exclusive staging, rename that never replaces, marker first; **no** for the length bound on real file systems (m1) and for trailing dots and spaces (m2) | `TestJoinKeepsThePreviousFolder` -> MJ12; `TestJoinFolderNameConfined` -> MJ14; `TestJoinFolderCollisionGetsSuffix` -> MJ15; `TestJoinCrashDuringInstall` -> MJ16 | paths hunted and found sound: staging by `Root.Mkdir` (fails on any existing entry, symlink included); install by `renameat2(RENAME_NOREPLACE)` on two names relative to one open directory (no window between the choice of the name and the rename: a taken name fails and the next is tried); `childName` refuses separators and dot names again below the normaliser; `recoverJoin` removes only `.inkwell-join-<32 lowercase hex>` through `os.Root` (no symlink followed, no parent escape) |
| N-13, N-16 | yes | `TestRequestsRefusedOnceJoinHeld` -> MJ13; `TestStorageWriteBeforeFinishJoinLandsOrIsRefused` | m5: a held Welcome whose page never reaches the restart page keeps the session refusing |

No critical finding: no path was found where a join writes into, replaces or follows an existing entry, where a
peer's title or record field names anything but a new direct child of the parent, or where `CompleteJoin` records a
root from a non-member or before the document is stored.

**Major**

- **M1 - the wait keeps the first root it heard, for the whole start** (`frontend/src/workspace/workspace.svelte.ts`,
  `#resumeJoin`, « kept from one try to the next »). With no root in the record (every invitation today), the first
  member whose announcement arrives fixes the root; when `repo.find` then fails on it for good (that member announced a
  document nobody serves, or went away), the page asks for the same root every second and the announcement of the
  inviter, arriving later, is never looked at. A member other than the inviter can thus hold a newcomer on the waiting
  page. Fix: at each try the inviter's announcement, once there, replaces another member's. What stays (residual
  RJ-1): a member who announces a root **and serves it** before the inviter is heard has it recorded, because the root
  is in no signed op (the genesis root is empty in every list Inkwell signs today, plan J-1); closing that needs the
  root in the signed list, a protocol change outside this block. [done] (superseded by F1); what remains of RJ-1 → TR1
- **M2 - the common invitation shows « En attente de Personne 12D3 pour ouvrir le document… »**
  (`workspace.svelte.ts` `#resumeJoin`: `pending.name || this.nameOf(inviter)`; `internal/app/join.go`
  `JoinPending.Name`: « empty for the creator of the project »). The creator of a project has no name in the list, no
  presence name is heard while the document is not open, and every device id starts with `12D3`. So the person who
  joins the project of its creator, the usual case, reads a meaningless name. Fix: a dedicated sentence when the list
  gives no name, « En attente de la personne qui vous a invité pour ouvrir le document… ».

**Minor**

- **m1 - the 64 limit counts characters, file systems count bytes** (`join.go` `joinFolderName`). 64 letters of a
  supplementary-plane script (4 bytes each in UTF-8) make 256 bytes; ext4, btrfs, xfs, tmpfs and APFS refuse a name
  over 255 bytes, so the install rename fails and the join ends with J-4's sentence for a legitimate title; three-byte
  scripts fit (192 bytes) but the suffix « (99) » is not budgeted either. Fail-safe, never an escape. Fix: also bound
  the name in bytes, cut on a character boundary, with room for the suffix.
- **m2 - a name may end with a dot or a space** (a title « Roman. », or the 64 cut landing after a space). Windows and
  SMB shares strip them silently, so the folder asked for and the folder made differ, and a name that differs only by
  them collides. Fix: trailing dots and spaces removed, as leading ones are.
- **m3 - Windows device names** (`CON`, `NUL`, `COM1`…, with or without extension) pass the normaliser; on Windows the
  rename fails and the join ends with J-4's sentence. Fail-safe; not fixed (residual RJ-2): no Windows build is
  released yet and the list belongs with the Windows packaging block. → PL1
- **m4 - `TestLanAskAndAnswer` lost `expectRepoUp` and never looks at the answer of `Ask`**
  (`internal/app/invites_test.go`): the « ask a nearby device » path is the second entry of `invites.hold` and nothing
  asserts that it answers a restart, nor that the two devices link in the image the join restarts. The other four
  rewritten tests (`TestStandingMemberAfterJoin`, `TestStandingNeverOldAfterRolesEvent`, `TestNotMemberInviteRefusedByKey`,
  `TestJoinByTicketText`, plus `TestOnlyAnAdminInvites` and `TestRestartReconnectsMembers`) keep every assertion that
  still holds, moved to the restarted image; the lock-interleaving half of `TestStandingNeverOldAfterRolesEvent` went
  with the live adoption it interleaved.
- **m5 - a held Welcome can outlive its page** (`change.hold` sets `requested`; only `FinishJoin` or `CancelChange`
  end it). When the restart page finds no mark (session storage unavailable, a reload) it loads the application and
  asks nothing: the Welcome stays held and `Join`, `Ask`, `Invite`, `TakeOver` are refused until Inkwell is quit, with
  no sentence. Fix: the restart page that finds no mark drops whatever is held (`CancelChange`), which is exactly
  « nothing was confirmed ».
- **m6 - the sentence names « Roman » when the folder will be « Roman (2) »** (`JoinResult.Folder` is the base name;
  the suffix is only known at the rename). Not fixed: the free name can only be known at the install; see
  « Walkthrough points ».
- **m7 - `join.folderDefault` is in the catalogue and used by nothing**: the default name is a Go constant
  (`joinFolderDefault`), in French for every locale, and nothing ties the two. Fix: a test that reads the catalogue and
  compares; the constant stays the source (the folder is named by Go, which knows no locale). Residual RJ-3: the default
  folder name is not localised. RJ-3 → PC1b
- **m8 - orphan staging folder after two failures** (J-4 above): residual RJ-4. → PC1b

### B. /simplify

| Id | Finding | Verdict |
|---|---|---|
| S1 | `Wiring.SwitchProject` / `AdoptProject` have no production caller left (only `wiring_test.go`, a fixture of `enforce_test.go`, and the texts of mutants MW9 and MJ11) | see phase 2 |
| S2 | `stageJoin` ends with `if err := stageList(…); err != nil { return err }; return nil` | simplify, MJ1's anchor follows |
| S3 | `hold` and `newHeldJoin` each call `j.List.Since(nil)` | kept: `newHeldJoin(j, title, addrs)` is the seam the folder-name tests drive with arbitrary titles |
| S4 | `JoinWaiting.svelte` repeats the condition `App.svelte` already holds | kept: the component is rendered alone by its test and stays correct by itself |
| S5 | reuse of PC1a: `change.record` is shared by `request` and `finishHeld`; the restart page, the mark and the startup notice are extended, not duplicated; `renameChild` is shared with `renameNative` on Windows and through `noReplace` on Unix | nothing to do |
| S6 | altitude: `internal/project` holds the primitives (staging folder, record, rename that never replaces), `internal/app` the sequence with the member-list store and its hook | right split: the list staging needs `saveIn` of `app` |
| S7 | `CompleteJoin` reads the list through `s.inv.wiring.list()` while `joinRecord` reads `s.inv.currentList()` | kept: one object since nothing adopts a list live; MJ6's anchor |
| S8 | frontend: `.note` and the spacing tokens are reused; `JoinWaiting` reuses the look of `.file-view` by copy | kept (five declarations; a shared class would touch `App.svelte` beyond the block) |

Phase 1 verdict: **FIX-THEN-ACCEPT** - majors M1, M2; minors m1, m2, m4, m5, m7; simplification S2 and the decision on S1.

## Phase 2 - applied (strict TDD: red for the right reason first, then the fix)

| Id | Red first (quoted) | Fix |
|---|---|---|
| m1 | `TestJoinFolderNameConfined/64_four-byte_letters`: « a join titled "𐐀…" failed: join: installing the folder: file name too long » | `joinFolderName` also stops at `maxJoinFolderBytes` = 240 bytes, on a character (`internal/app/join.go`) |
| m2 | `…/trailing_dot`: « joinFolderName("Roman.") = "Roman.", want "Roman" »; `…/trailing_spaces_and_dots`; `…/cut_after_a_space` (a name ending with a space after the 64 cut) | trailing dots and spaces removed; the default name when nothing is left (`cmp.Or`) |
| M1 | Vitest « PC1 a pending join follows the root of the inviter over another member's »: « J-1 the root of the inviter, once announced, is the one followed: expected "vi.fn()" to be called with arguments » (the page completed with the other member's root) | `#resumeJoin`: at each try, what the inviter announced replaces what another member announced (`workspace.svelte.ts`) |
| M2 | Vitest « PC1 a pending join without a root waits for the one a member announces »: « J-2 no made-up name for the inviter: expected 'Personne host' to be '' » | `waitingFor` is "" when no name is known; `JoinWaiting.svelte` then says the new key `join.waitingRootUnnamed` (fr, en); the assertion that expected the neutral name was replaced by the rule, not weakened |
| m4 | no red possible (a restored assertion; passes, mDNS available on this machine) | `TestLanAskAndAnswer` asserts that `Ask` answers a restart with the folder name, restarts the joiner and waits for the link with the host (`expectRepoUp`, as before the block) |
| m7 | no red possible (a drift guard) | `TestJoinFolderDefaultIsTheCatalogueSentence`: the Go constant equals `fr.json` `join.folderDefault` and survives the normaliser |
| S2 | — | `stageJoin` ends with `return stageList(…)`; mutant MJ1's anchor and its swapped form follow |
| S1 | — | **kept, documented**: `Wiring.SwitchProject` / `AdoptProject` carry a comment saying no production code calls them (N-11) and why they stay. Removing them means rewriting `TestSwitchProject`, `TestAdoptProjectNeverWritesTheOldProjectInTheNewStore` and the fixture of `TestGuestKeepsAttestationsReplayedBeforeItSwitchesProject` (the proof of JOIN-3, whose scenario a restart no longer produces), and re-expressing MW9 and MJ11 of two gates: a clean-up block of its own, outside this diff. Guards meanwhile: gate PC1a `no-live-change`, mutants MW9 and MJ11 |

The plan (`docs/plans/2026-11-15-pc1b-join.md`, rows J-1, J-2, J-6 and the catalogue) says the three changed rules; one
row was appended to `docs/autonomous-decisions.md`.

## Not applied, with reasons (residuals)

- **m5** (a held Welcome outliving its page): the fix belongs in the restart page's « no mark » branch, which is the
  anchor of PC1a's in-test mutant « N-9 request without the confirmation » and whose oracle is « the log is exactly
  the load »; changing a frozen block's oracle for a state only a reload or a missing session storage reaches is not
  proportionate here. Quitting Inkwell clears it. **RJ-5**, to take with the next change of the restart page. → PC1b
- **m3** Windows device names: **RJ-2** (fail-safe; Windows packaging block). → PL1
- **m6** the folder sentence cannot know « (2) » before the install: walkthrough point. → PC1b
- **m8** orphan staging folder after a failed recovery followed by a new join: **RJ-4** (needs two failures; the folder
  is inert, `project.Latest` ignores it). → PC1b
- **RJ-1** a member who announces **and serves** another root before the inviter is heard has it recorded: the root is
  in no signed op. **Closed by F1 below** (only the inviter's announcement counts); what remains is the signed root
  (block TR1). → TR1
- **RJ-3** the default folder name is French in every locale. → PC1b
- A file system without a rename that never replaces (NFS, some FUSE mounts) fails every join with J-4's sentence, as
  the plan states; `mkdir` exclusive then a rename over the empty folder would be as safe and is left for a later block. → PC1b

## Results (2026-11-15, this tree, one heavy job at a time)

- `go test -count=1 ./internal/app/... ./internal/project/... ./internal/bridge/... ./cmd/...`: ok, exit 0; with
  `-race`: ok, exit 0 (app 111 s, project 138 s).
- Mutants: PC1b 16 of 16 caught (MJ1..MJ7 in a first run that hit its 900 s cap waiting for a heavy slot taken by
  another worktree's gate, MJ8..MJ16 in a second: « all 9 caught »); PC1a « all 15 caught »; W58a « all 4 caught ».
  Anchors: `mutants.py --check` PC1b 16, PC1a 15, W58a 4, all « edits apply ».
- Vitest `-t 'PC1 '`: 42 passed; the five changed test files and `src/i18n`: 70 passed; Biome: no fix, 13 warnings;
  svelte-check: 0 errors, 0 warnings.
- Gate criteria run verbatim from `gates/PC1b.toml`: `mutant-tool` 0, `go` 0, `frontend` 0, `e2e` 0 (8 expected,
  0 unexpected, 0 flaky, 0 skipped, 74 s).
- `mise run lint`: exit 0. `mise run pregate`: exit 1 on the base guard only, nine ids, all from lines the block
  wrote before this review (none from the fixes above); justifications proposed in the review report, not written
  to `gates/guard-overrides.json`.

## Walkthrough points

1. The waiting page with an unnamed inviter (a project's creator): the new sentence, light and dark.
2. The wait has no end and no way back on the page (R-10): does the person understand that the previous document is
   still in its folder, and how to reopen it (file manager, relaunch on that folder)? Every start reopens the waiting
   folder, the most recent one.
3. The folder sentence under « Rejoindre » when the name is taken: it says « Roman », the folder is « Roman (2) ».
4. First-launch join: the confirmation-to-writable time the plan asks the UX report to state (J-3).
5. A long title (64 characters) and a title in a non-Latin script in the folder sentence.

**Verdict after phase 2: ACCEPT**, with residuals RJ-1..RJ-5 to file and the nine guard ids to justify. Filed: RJ-1 → TR1, RJ-2 → PL1, RJ-3, RJ-4, RJ-5 → PC1b.

## After the code review (Codex, 2026-11-15): F1

Codex's review of the code (`gates/PC1/REVIEW.md`, « Code review, PC1b ») raised RJ-1 to a major, blocking finding
**F1**: with no root in the record, a member of the joined list that announces first and serves a document binds the
newcomer's project to it for good. No other defect (confinement, exclusive create, rename without replacement,
recovery, refusals). Fixed in strict TDD; this supersedes row M1 above (« the inviter's announcement replaces another
member's »), whose logic is removed.

| Id | Red first (quoted) | Fix |
|---|---|---|
| F1 Go | `TestCompleteJoinRefuses/not_the_inviter`: « J-1 CompleteJoin refuses a root not announced by the inviter: CompleteJoin = <nil>; project.json is { … "rootUrl": "automerge:joinedRoot1" }; want refused, the record kept and no root » | `CompleteJoin` refuses, when the record holds no root, any peer other than the record's inviter (`errJoinInviter`, `internal/app/join.go`); with a root the rule is unchanged (`TestCompleteJoinStatedRootFromAnyMember`); mutant MJ17 |
| F1 page | Vitest « PC1 a pending join without a root ignores what another member announces »: « J-1 only the inviter announces the root of a record without one: expected 2 to be +0 » (the other member's document was asked for and opened) | `#resumeJoin` waits for the inviter's announcement alone and always completes with the inviter; `#announced` loses its `also` parameter and resolves with the URL; in-test mutant « PC1 mutant J-1 a root announced by another member is caught »; the test « … follows the root of the inviter over another member's » is replaced by the stricter rule |

Simplification that came with it: `#announced(peer, timeoutMs?)` returns the URL (the announcing peer is always
`peer`), and `#resumeJoin` keeps a URL instead of a `{url, peer}` pair. Residual: without a root in the record the
wait lasts until the inviter is reachable, even with other members online; the signed root belongs to block TR1. → TR1

Two existing mutants lost their observable to the new rule and got it back, tests strengthened, none weakened:
MJ6 (a non-member accepted) was masked because, without a root, the inviter rule refuses a stranger first; its subtest
now runs on a record that states the root, where membership is the one rule on the peer. MJ9 (no record accepted)
became equivalent in effect (an empty record names no inviter, so nobody completes it); the « no record » subtest now
asserts the reason, `errors.Is(err, errNoJoinRecord)`.

Results after F1 (2026-11-15, this tree, one heavy job at a time): `go test -count=1 ./internal/app/...` ok (27 s),
with `-race` ok (79 s); `mutants.py PC1b`: « all 17 caught » (MJ1..MJ17); `mutants_test.py` OK; the `mutant-tool`
check of the frozen gate passes with 17 entries; Vitest `-t 'PC1 '`: 43 passed; Biome on the two touched frontend
files: no fix; `mise run lint`: exit 0; `mise run pregate`: exit 1 on the base guard only, two ids of
`workspace.svelte.ts` (the content-keyed overrides of `#announced` and its timer, to re-enter for the new content).


## After the walkthrough (2026-11-15): decisions D1, D2, D3

The walkthrough (`gates/PC1b/UX.md`) ended « Verdict: fix » on two severity-3 defects. The integrator's decisions were
implemented in strict TDD; the plan's rows J-1, J-2, J-4 and its catalogue carry the new wording.

| Id | Red first (quoted) | Fix |
|---|---|---|
| D1 Go | `TestJoinNamesTheFolderCreated`: « J-2 the folder announced is the first free name: "Mon livre", want "Mon livre (2)" »; `TestJoinRequestWritesNothing`: « J-2 the answer names the document, the folder it gets and the open one: {… Title: Folder:Mon livre Current: Restart:true} » | `hold` answers `title`, `folder` (first free name, `freeJoinName`, same `suffixed` names as the install) and `current`; the record keeps `folder` and `previous`; `PendingJoin` gives the real folder name when it is another one |
| D1 page | « PC1 join says that Inkwell restarts »: « J-2 says before the click that the current document is kept: expected '…' to contain 'Votre document actuel est conservé.' »; « PC1 a join waits for the confirmation… »: « J-2 nothing restarts before the confirmation: expected "leave" to not be called at all, but actually been called 1 times » | `workspace.join` only keeps the offer; `JoinConfirmDialog` (the take-over's dialog pattern) asks; `confirmJoin` leaves, `cancelJoin` calls `CancelChange` |
| D2 Go | `TestReturnToPreviousSurvivesARelaunch`: « J-2 a waiting join offers the return to the previous document: {… Waiting:true CanReturn:false} »; `TestReturnToPreviousRefuses`: « J-2 return refused without a previous document: with no join waiting ReturnToPrevious = <nil> » (against a stub) | `Session.ReturnToPrevious` through `requestChange`; `project.Prefer`, `project.IsProject`; the one-shot word `returned` read at the start |
| D2 page | « PC1 the waiting page says what to do and offers the way back »: the text was the waiting sentence alone; « PC1 the restart page asks Go for the return… »: « expected [ 'takeOver', 'load:./' ] to deeply equal [ 'returnToPrevious', 'load:./' ] » | `JoinWaiting` (sentence, what happens next, « Revenir à mon document »), leave kind `return`, `?return=refused`; no preview pane and « Connexion… » while a join waits |
| D3 page | same test as D1 (« J-2 « Rejoindre » shows progress ») | `aria-busy` on the button, « Connexion à la personne qui vous invite… », no restart sentence beside an error |

Review of this diff (reuse, simplification, efficiency, altitude):

- Applied: one name sequence for the announced folder and the install (`suffixed`); `workspace.joinFolder` removed, replaced
  by the offer; `join.restart.folder` keeps its key (the frozen `docs` criterion names it) and carries the question's first
  sentence, so no dead key; `REFUSED` in `restart.ts` is one table by kind instead of a chain of conditions; `Prefer` is
  bounded to three turns; Escape does not cancel a join whose page is already leaving.
- Kept on purpose: `previous` is a folder name resolved through `os.Root` beside the open folder, not a path (cannot leave
  the parent, survives a moved documents folder); the silence of the start after a return is a word in the previous
  folder, read and removed at its start, rather than a second value of the restart environment (PC1a's tested contract
  `restartCommand` stays as it is); the status while waiting reuses « Connexion… » rather than a new state.
- Residuals: (1) while the question is open the session refuses every other request (N-13, as while a Welcome was held
  before, now for as long as the person reads); a page reloaded at that moment leaves the Welcome held until Inkwell is
  closed; the desktop window has no reload. (2) The application still has no list of documents: after a successful join the
  previous one is reached by its folder, named in the question. (3) The waiting page offers no cancellation of the join
  itself. (4) `Prefer` dates a file: a clock set back by more than the age of the other project is handled (the date is
  put past it), a later manual copy of an older folder is not Inkwell's to order. Filed: (1), (3), (4) → PC1b; (2) → UX1.
- Mutants: MJ18 (a return accepted with no previous document) and MJ19 (the return does not date the previous folder),
  each caught by its test and text; in-test: « PC1 mutant J-2 a restart without the confirmation is caught »,
  « PC1 mutant J-2 « Annuler » that leaves the held invitation is caught ». The frozen gate's `mutants` criterion runs every
  PC1b entry: nineteen now, for a cap written for fifteen (measure below).

The shared e2e helper follows the new step: `acceptJoin` (`frontend/e2e/helpers.ts`) presses « Rejoindre » and, when the
question follows, « Redémarrer et ouvrir » (an invitation into the document already open asks nothing, J-5); `pair`,
`joinAs` and the inline copies of `collab`, `history`, `join` and `people` use it. Their sync-status locator is now
`getByRole("status").first()` as `joinAs` already had it: the notice of a join is a status too, and the first run on fresh
fixture folders failed on the ambiguity (« strict mode violation: getByRole('status') resolved to 2 elements »); no delay
changed. One Go assertion followed the rule (`TestLanAskAndAnswer`: the folder answered is « Mon livre (2) »).

Results after D1..D3 (2026-11-15, this tree, one heavy job at a time, the machine shared with two other worktrees):
`go test -count=1 ./internal/app/ ./internal/project/ ./cmd/inkwell/`: ok (41 s, 118 s, 4 s); with `-race`: ok (99 s,
111 s, 20 s). `mutants.py PC1b`: « all 19 caught » (74 s); anchors `--check`: PC1b 19, PC1a 15, W58a 4, all « edits
apply ». Vitest `-t 'PC1 '`: 50 passed; the whole suite: 119 files, 1371 passed, 1 todo. Biome: no fix (13 warnings, as
before); svelte-check: 0 errors, 0 warnings. Gate criteria verbatim from `gates/PC1b.toml`: `go` 0, `frontend` 0,
`mutant-tool` 0, `e2e` 0 (9 expected, 0 unexpected, 0 flaky, 0 skipped, 95 s; first-launch join 1184 ms and join 1156 ms
from « Redémarrer et ouvrir » to the joined text, writable). e2e `join`, `collab`, `people`, `history`: 8 passed. `mise run
lint`: 0; `lint:env`: 0; `lint:claude-md`: 0. `mise run pregate`: exit 1 on the base guard only, eight ids (below), none
written to `gates/guard-overrides.json`.

| Guard id | File | Proposed justification |
|---|---|---|
| `0177025ca30e8e312c192f576e8b8d82794cc2529e9a4ed7f26a8668b7cf0658` | `frontend/e2e/project-change.spec.ts` | the 60 s poll « another process image answers the application » of the new test « a waiting join returns to the previous document », the same bound as the existing polls of the file; no existing delay changed |
| `5bc890cb36c18b575978d0ce22051e5ee11a173a5474892aefa961be00325f85` | same | `test.setTimeout(180_000)` of that new test, as its three neighbours; no existing delay changed |
| `a92e3e21eddf9e6a8c98dcb06a378fa60fe0bea010a93780af3805c4fe200c87` and `76b24eba7c182942791b7eb76ae200b8a9bcae1c87a43e9cc458c9a7d8ed1230` | `frontend/src/workspace/workspace.svelte.ts` | the content-keyed overrides of `#announced` and its timer (already justified for this block) to re-enter for the new content of the file; those lines are not touched by D1..D3 |
| `befd26d940b9bf07c258ee968ad438f94789f84975538a5bc3f46113f40c2a35`, `09b011342785d13e40a6c8112ac026b2a8421ce6786e7be88bf7d2546cbecbf3` | `frontend/e2e/helpers.ts` | `.first()` added to the status locator; the 60 s bound of the line is unchanged |
| `9dd91ecc49ad077e88f34ca189ad4c091a633c6fafaa1feeebe49da33005d906` | `frontend/e2e/history.spec.ts` | same |
| `8e041cbf4dea6cd8b3732d4c63dc335b3e8c9a120df0d92cb89703e0aae349c1` | `frontend/e2e/join.spec.ts` | same |

## After the re-judgement (2026-11-15): UX RJ-1 fixed, UX RJ-2 residual

(The ids are the re-judgement's, `gates/PC1b/UX.md`; they are not the residuals RJ-1..RJ-5 of this file.) UX RJ-2 → UX1.

**UX RJ-1, cause, with evidence.** A ticket is single-use and the host takes it before it admits (`internal/invite/flow.go`,
`Host.redeem` -> `take`, then `admit`); the Welcome the guest held is dropped by `CancelChange` (`change.cancelHeld`), so
nothing is left to reuse, and the same text is refused by the host with `ReasonInvalid` (`invite: the invitation is not
valid`), which the page shows as its generic `error.join`. Pinned by `TestJoinCancelledInvitationIsUsed` (not a red-first
test: it states the behaviour that exists; its name is in the pattern of the frozen `go` criterion). Making the second
« Rejoindre » succeed would mean a host that answers a used secret, or a guest that keeps a Welcome after « Annuler »: the
first changes the invitation protocol, the second contradicts J-1; neither was done. `admit` does answer a member with the
Welcome again, but only behind a valid ticket or an Ask, hence « a new invitation works » (J-4).

| Red first (quoted) | Fix [done] |
|---|---|
| « PC1 a cancelled invitation leaves the field, and the page asks for a new one »: « J-2 says that a cancelled invitation is used: expected null to be 'Invitation annulée. Pour rejoindre ce…' »; « PC1 a join the page could not save for is dropped »: « J-2 the invitation is used all the same: expected undefined to be true »; the mutant test: « anchor found 0 times » | `workspace.joinCancelled` (set by `cancelJoin`, which is also the path of edits not saved in time; cleared by the next `join` and by `stop`); `ShareDialog` empties the field and shows `join.cancelled` (fr, en) as a status where the restart note was; no error. In-test mutant « PC1 mutant J-2 a cancelled invitation kept in the field is caught » [done] |

Review of this diff: one state in the workspace rather than a callback per button, because the cancellation has two
origins (« Annuler » and N-15's failed flush) and both use the invitation; the sentence takes the place of the restart note
in the existing card (no new element, no toast); nothing changed in Go but the test.

**UX RJ-2, residual for the interface block UX1.** After « Revenir à mon document » nothing says that the join still waits
in its folder, which is reached only from the file manager. Proposed fix: one line in the notice of the start that follows
the return, « L'invitation reste en attente dans le dossier « {folder} ». ». Not a pure page change, so not done here: the
start after a return says nothing by design (the word `returned`, `takeReturned`), `Session.StartupNotice` answers a
catalogue key without a parameter, and the page that returns does not know the name of its own folder
(`JoinPending.Folder` is empty unless the name differs from the one announced). It needs the folder name written with the
word `returned` and a notice with a parameter, with Go tests; UX1 also owns the missing list of documents, which is the
other half of the defect. → UX1


## Re-applied on main 8a23f84 (2026-11-23)

The block's work (made on e637035) was re-applied with `git apply --3way` on main 8a23f84, which gained PU1, HP1,
WC1, CL1 and CI3 meanwhile.

- `internal/app/change.go`: WC1's desktop close approval (`confirm`/`reject`, consumed before the irreversible
  drain) and PC1b's held Welcome both kept. `request` and `finishHeld` consume the approval through one helper
  (`approveLocked`) and hand `reject` to `record`, which releases the desktop hold when the drain or the write fails. A
  join consumes it at `FinishJoin`, not at `Join` (holding drains nothing). Without the approval the held Welcome is
  dropped as `CancelChange` drops it: the restart page then comes back at `?join=refused` (« nothing changed »), so the
  session must accept requests again (N-15).
- `frontend/src/workspace/workspace.svelte.ts`: WC1's `leave` (native approval, `flushBeforeRestart`,
  `closeBeforeRestart`, `closing`) kept, with PC1b's `kind` passed to `markLeave(kind)`; PC1b's join fields added
  beside WC1's.
- Guard: five alerts are PC1b's own justified changes with new fingerprints (their context moved with the merge):
  `frontend/e2e/helpers.ts` (two `.first()` lines of `pair`), `frontend/e2e/history.spec.ts` (`acceptJoin`,
  `.first()`), `frontend/src/workspace/workspace.svelte.ts` (`#projectOf` → `#announced`, its optional timer). Each
  gets a new entry in `gates/guard-overrides.json` (decision `docs/objectif-en-cours.md`), with the same reason as the
  entry it replaces; no bound changed.
- New test `TestFinishJoinConsumesTheDesktopApproval` (`internal/app/join_test.go`) for the merged behaviour: no
  approval consumed by `Join`; without it `FinishJoin` records nothing, drops the Welcome, the session accepts requests
  again; with it the join is recorded. The manual mutant that keeps the Welcome held on refusal is caught.

## Gates of 2026-11-23: PC1b2 (supersedes PC1b), PC1n, PC1n2

Written at step 1 of the rebased block, from `gates/PC1b.toml`, `gates/HP1n3.toml`, `gates/HP1n5.toml` and
`gates/HP1n2.toml`; each change and its reason are in the gate headers, the clause rules in the plan (rule J-7,
« Netsim proof of the join restart »). Findings while writing them, each with its evidence:

- The join restart is not logged: `restarts()` in `netsim.spec.ts` reads `msg=restarting`, which no successful
  restart writes (only the failure line `restarting after a project change`). Scenario 5's C3a would therefore never
  excuse the restart's outage. Rule J-7 (one `msg=restarting change=… folder=…` line before the execution) and
  `TestRestartLogsOneLine`, mutant MJ20 frozen in `gates/PC1b2.toml`. → PC1b
- Once J-7 logs it, scenario 9's frozen C3a (HP1n) and C3c (HP1n5) cannot pass as written (one restart counted, and
  for C3c no outage over the whole run): replaced under new names C3j and C3k by `gates/PC1n2.toml`. → PC1b
- SHEET-MU-01b (WC1's runner, `scripts/wc1-webkit.sh`) failed on WebKit (`#cell-1-3` not found on bob after `pair`).
  Cause, with evidence: the fixture `both()` opens the table view, then pairs; since J-1 the join restarts bob's page
  (trace: « Redémarrer et ouvrir », then bob's new page in the text view, `Mise en forme Typst` toolbar, 2 people), and
  the view mode is page memory (`ui/sheet/viewMode.svelte.ts`, « not kept across a reload »). Not WebKit-specific:
  run alone, Chromium fails the same way; in the full suite it passed only because an earlier test had already
  joined bob into alice's project (J-5, no restart). Fix in the fixture, which relied on a live join the spec no
  longer promises: `tableAfterJoin` (`e2e/sheetHelpers.ts`) shows the table again when the join reloaded the page;
  SHEET-MU-01b alone in Chromium: red, then green. The browser `Page` follows the navigation by itself; the native
  walk does not, which is the capability the native join test adds. → PC1b
- `offline.spec.ts:113` fails in the full Chromium suite (129 passed, 1 failed, 21 skipped, 514 s): strict-mode
  violation, the join notice is a second `status` element; PC1b fixed the same pattern in five specs, not this one.
  The `regression` criterion now runs every spec that pairs or joins (266 s, fourteen specs). → PC1b
- `TestJoinSameProjectNoRestart` failed 4 in 200 runs (8 in 200 with `-race`): `join: p2p: connect to …: failed to dial:
  no addresses`. Cause, with evidence (timestamped instrumentation, removed): the join's `Node.Connect` added the
  ticket's five addresses to the peerstore at 32.519 ms; the dial loop's `dial()` for the same peer ran
  `Peerstore().ClearAddrs` at 32.524 ms (it clears to dial its own set only); the swarm read the emptied set for the
  join's dial and failed it at 32.544 ms. The peerstore is shared by both diallers; nothing kept the loop's clear off a
  `Connect` in flight. Widening the loop's window (20 ms after the clear) did not change the rate, which places the
  race at the clear itself. Fix in the product (`internal/transport/p2p/dial.go`): the loop's attempt is refused
  (`errConnectInFlight`, retried on its schedule) while `connecting[id] > 0`, the check and the clear under `mu`, as
  `Connect` counts itself in before adding its addresses. `TestLoopDialSparesTheAddressesOfAConnectInFlight` red,
  then green; after the fix 200 of 200 and 200 of 200 with `-race`; p2p and app suites green. → PC1b
- Native join: WC1's harness already drives a join between two native processes (`SHEET-MU-01b`, probe command
  `join`); what it lacks is a probe command for « Redémarrer et ouvrir » and following a process that restarts in
  place (same pid, new startup id). About 150 lines, the same that SHEET-MU-01b needs anyway: included in PC1b2 as
  `TestWC1NativeClose/PC1b-join`. → PC1b
- `TestWC1NativeRecovery` fails intermittently at `wc1_native_test.go:813` (« GTK view state = 3, expected mask 12 »:
  after a hung store or renderer, the recovery notice, bits 4|8, is not shown within the walk's 5 s, which equals the
  product's own 5 s deadline). Evidence that PC1b does not cause it: on clean `main` (8a23f84) HungStoreForce failed so;
  on this tree HungRendererForce failed once, then all four recovery cases passed. Cause not established; it belongs
  to WC1's walk, outside this diff. Until it is fixed, PC1b2's `webkit-native` can be red for it too. → WC1
- Regression criterion, three measured runs on this tree: 282, 288, 281 s (cap 400 s), red only for `offline.spec.ts`
  (a named reason, step 2). → PC1b

## Step 2 of PC1b2 (2026-11-23): the reds the frozen gates named

Each item red first for the reason the gate names, then green; the full results of the criteria are below.

- **J-7** (`cmd/inkwell/startup.go`, `internal/app/change.go`): the change records its kind with its write
  (`ChangeTakeOver`, `ChangeJoin`, `ChangeReturn`, `Runtime.RestartChange`); the dep `restartRequested` became
  `restartChange`, and `startup` logs `msg=restarting change=<kind> folder=<folder>` once, after the profile is
  released and just before `restart`. `TestRestartLogsOneLine` drives the real join (an in-process host), the return
  from the folder that then waits, a launch with no change, and the take-over of a folder another device's list
  names. Red: the three changes restarted on the right folders and logged no line; green after the line. → PC1b
- **MJ20** (`scripts/netsim/mutants.toml`): the frozen entry, its edit replaces the log call by `_ = change` (the
  mutant must compile). Anchors moved by this diff and repaired with the mutant unchanged in meaning: MW4 (the
  take-over's `requestChange` now names its kind), MW8 (`code = d.window`), MJ5 (now on `RestartChange`, which the
  product reads; `RestartRequested` is a test convenience over it). Stale on `main` since WC1, in files this diff
  edits, repaired the same way: MW11 and MW15 (the shutdown's closures moved one level up), MW13 (`request` now
  consumes the desktop approval). Still stale on `main` and outside this diff, left to their blocks: NI0 M5c
  (`relay.go`), PU1 MP1 and RL1 RL1a (`stream.go`), SB1b-06, -13, -30, -33 (`internal/sidecar`). → NI0, PU1, RL1, SB1b
- **regression** (`frontend/e2e/offline.spec.ts`, `link.spec.ts`): the connection status is read in the header
  (`getByRole("banner").getByRole("status")`), the element both assertions mean; `link.spec.ts` has the same bare
  locator, which strict mode fails whenever the join notice still shows. Bounds and expected texts unchanged.
- **webkit-native** (`cmd/inkwell/wc1_native_test.go`, `frontend/src/workspace/nativeCloseProbe.ts`): the probe
  answers a held join with `join-held` and presses the real « Redémarrer et ouvrir » (`confirm-join`); each process
  image reports `image` (pid, startup id, standing, members) from Go; `joinRestarting` follows the execution in
  place (same pid, new startup id, the J-7 line in the child's log) and returns the new image's page; the child
  restarted by a change opens the folder its argv names, as the shipped binary does. SHEET-MU-01b's guest goes
  through it; `TestWC1NativeClose/PC1b-join` records join-held and close-authorized (app side), process-restarted
  and project-joined (harness), and checks that the previous folder's text is unchanged (J-6).
- **PC1n, PC1n2** (`frontend/e2e/netsim.spec.ts`): CJ (scenarios 5, 8, 9), CJd (5, 9), C3w (5), C3j (9 graceful) and
  C3k (9 crash) as the plan's table defines them; scenario 9's C3a and C3c removed, scenario 5's C3a kept. An outage
  is placed from one read of the inviter's link (begin = read time - `upMs` - `lastDownMs`); a read that counts two
  new outages, or one on a link that is down, cannot place them and the clause fails as data. Not run (the
  integrator runs netsim alone on the machine): Biome, `playwright --list`, the helpers dry-run on synthetic logs
  and reads (sixteen cases, MN4's 30 s shift caught by C3w), and the gates' clause parsers on dry result files.
- Guard (`python3 scripts/gate.py --base-guard`), four alerts, each a new entry in `gates/guard-overrides.json`
  (decision `docs/objectif-en-cours.md`): `restart_linux_test.go` twice (PC1b's new 10 s and 90 s of
  `TestFirstLaunchJoinRestarts`, new fingerprints because one line of the file changed; the old entries replaced),
  `offline.spec.ts` twice (the status locator rewritten, its 30 s unchanged).
- Results on this tree (2026-11-23), every PC1b2 criterion but ux-walkthrough run once, verbatim: docs 0 s, go 49 s,
  mutant-tool 1 s, mutants 120 s (MJ1..MJ20 caught), frontend 17 s, e2e 110 s, regression 403 s, tla 161 s,
  webkit-native 111 s (SAV-03b and SHEET-MU-01b in both browsers; every native case, `PC1b-join` included, its
  trace join-held, close-authorized, process-restarted, project-joined on one pid), simplify-report: all exit 0.
  The regression's e2e part took 390 s of its 400 s cap (266-288 s measured before) while the main checkout ran a
  full Vitest suite and a TLC check beside it; not attributed to that load without a run alone (CLAUDE.md): the
  integrator's gate run, alone on the machine, measures it. Repaired PC1a mutants MW4, MW8, MW11, MW13, MW15
  caught by their tests; `internal/app`, `cmd/inkwell` and `internal/transport/...` green. → PC1b
- **PC1n2 `netsim-9` red on the gate run (2026-11-23), cause and fix.** On all three seeds C3j read « 2 in the
  join window, 0 in the harness window »: one outage beginning at t_j - 0.4 s (0.4 s long), one at t_j + 2.3 s or
  + 2.8 s (0.1 s). Timeline (the graceful runs' node logs were wiped by the crash variant's run; the crash runs
  follow the same steps up to the restart): run.sh's `netsim: restart bob` comes 2.70, 2.70 and 2.84 s after
  bob's J-7 line, so the windows [t_j - 1 s, t_j + 15 s] and [t_h - 1 s, t_h + 15 s] overlap, and the second
  outage begins at the harness restart. One outage per restart, as PC1n2 allows: not a product finding. The
  clause gave every outage inside the join window to the join. Fixed in `netsim.spec.ts`: each outage counts
  for the restart it follows (`followedRestart`: of the windows holding its begin, the latest restart's), so an
  earlier restart never takes an outage a later one caused (C3k: the crash must cost none). Dry cases added for
  overlapping windows (one per restart passes; two for either restart, an outage at the crash, MN4's 30 s shift
  fail); the three crash runs' measured outages (-0.4 to -0.1 s) stay the join's. → PC1b
