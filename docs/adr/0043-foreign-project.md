---
status: accepted # part 1 (W58a) accepted and merged on 2026-11-14; part 2 (PC1) stays a draft, proposed; # split on 2026-11-14 into part 1 (block W58a, gate gates/W58a.toml: read-only and no broken invitation) and part 2 (block PC1, gate gates/PC1.toml: the project changes by a restart, never live); history in gates/W58/REVIEW.md rounds 1-8; the frozen gates/W58.toml is superseded; each part is accepted when its block is merged
date: 2026-11-14
decision-makers: [Mathieu]
---

# A project whose member list does not name this device opens read-only, and is taken over in place or joined; inkwell does not defend the local folder

Diagnosis: `docs/plans/2026-11-13-invite-not-admin.md`. Plans: `docs/plans/2026-11-14-w58a-foreign-read-only.md` (block W58a) and
`docs/plans/2026-11-14-pc1-project-change-by-restart.md` (block PC1). Gates: `gates/W58a.toml` and `gates/PC1.toml` (drafts; they supersede `gates/W58.toml`, frozen on 2026-11-14 for revision 6 and no longer run). Review history: `gates/W58/REVIEW.md`. Requirements:
W58 (R1 blocker), W64.

## Context and problem

`Start` loads the member list stored in the project folder (`.inkwell/trust`) and runs without checking that this
device is in it (`internal/app/node.go:170-176`, `persist.go:166-179`). A person whose profile was lost but whose
Documents were kept, who copied or synchronised a project folder, or who shares one Documents folder between two
profiles, runs as a stranger: the editor accepts text nobody will ever receive, « Inviter » is offered
(`workspace.svelte.ts:614-618`, true while the role is unknown, and unknown forever for a stranger) and fails with the
raw Go error « signer is not an admin » (`workspace.svelte.ts:730`).

**Trust boundary (owner's rule, 2026-11-14, W64; `docs/engineering-principles.md` §4 « Frontière de confiance »).**
inkwell is a text editor. The adversary is a *peer*; the files already on the machine are the person's, and who may
read, write, copy or replace them is her business and her operating system's. Revisions 1 to 3 of this ADR defended
against a crafted, substituted or replayed local folder (ownership registry, handle binding, bounded hostile reads,
migration exception E-1, fresh-folder transactions): all of that is withdrawn. What stays is correctness and clarity:
no crash, no silent loss, a sentence the person can act on.

A related correctness fact: `trust.Genesis(signer, title, root)` signs a fixed message with Ed25519, which is
deterministic (RFC 8032 §5.1.6), so one device, one title and one root give one `ProjectID`
(`protocol/trust/op.go:404-409`); the application passes an empty root today (`persist.go:172`).

## Decision

### Part 1 — block W58a: a foreign project is read-only and never offers a broken invitation

Reviewed by Codex (ACCEPT WITH CHANGES, applied: `gates/W58/REVIEW.md` « Round 9 »), ready to freeze.

No take-over and no change of project identity in this part. A person alone (profile lost, documents kept, nobody
to ask) can read the document and copy its text, and cannot edit or share it from Inkwell until part 2: the banner
says exactly that and promises nothing more. This is part 1's stated limit.

1. **Standing.** Go reports whether this device is in the loaded member list: `member`, `removed` (the list holds an
   op whose subject is this device, `List.Since(nil)` and `Op.Subject`, but gives it no role) or `notNamed`. It is
   computed from the wiring's list: a join turns it into `member` in the running session, a take-over at the next
   start (point 5).
2. **Not a member: read-only; no invitation and no signed attestation.** The editor is locked for every project
   until the standing and the role are known, so nothing is typed, saved or sent before the rights are established.
   A non-member's session issues no invitation and signs no attestation; the automatic projection still writes its
   local maintenance files in the folder, which is the person's (W64): the banner speaks of the right to edit, not
   of an immutable folder. The webview shows the document through the existing reader path
   (`access`, `Editor.setReadOnly`, `AccessBanner`) with one sentence and, in part 1, one action. The node keeps running exactly
   as today, because the join flow needs it (`invites.go:350-352`); a stranger is admitted by nobody, since every peer
   checks its own list.
3. **« Demander une invitation ».** Opens the existing join dialog; joining keeps today's behaviour and is **not made safe by part 1**: it still replaces the identity of a running session (part 2) (the member list
   is replaced in place, `invites.go:357-385`).
4. **Invite capability and refusals.** « Inviter » and « Créer une nouvelle invitation » are enabled only once Go has
   reported an admin role; before that they are disabled (« Vérification… »). A refusal crosses the bridge as a field
   of the result (`Invitation.Refused`, a stable key, like the existing `CodeError`), never as an error text: a Go
   error reaches the webview only as a string (`cmd/inkwell/main.go:113`, no serialiser). The Go text goes to the log.

### Part 2 — block PC1: the project changes by a restart, never live (mandatory before R1)

Part 2 adds « Reprendre ce document ici » and puts the join under the same rule. Draft, reviewed after W58a is
merged; its formal model is `ProjectChange.tla` (plan of PC1).

5. **« Reprendre ce document ici » (take over in place, by a restart).** After one confirmation that says Inkwell
   restarts and states the consequence, Go signs a new genesis in memory (this device, the title, the folder's recorded
   root document URL when valid) and asks the application to quit. While it shuts down, once the wiring is stopped
   and nothing can save an op any more, the existing member list is archived by copy in
   `.inkwell/trust-archive/<time>/` (never deleted, never rotated; not in `backups/`, whose rotation takes every name,
   `kv.go:307-318`, `:464-470`) and replaced by the genesis (`opStore.replace`: stage, swap, recovery). Then the same
   executable is executed again in place on the same folder. Text, files, binary store and root document stay where
   they are: no new folder, no seeding. **The project of a running session is never changed**: the next process loads
   the new list from disk, so this device is a member from its first instant, and nothing of the old session exists
   any more (point 6). A member's take-over is a no-op.
6. **Nothing of the old session survives.** Admission in the webview rests on the coverage built from the
   attestations of the project's writers; a head attested only under the old project must not be admitted under the
   new one, and nothing attested under the new one may be lost. Inside a live session that needs an ordering between
   the adoption in Go, the bridge's buffered events, the adapter and automerge-repo's own pending continuations, which
   three successive designs failed to give (« Considered options »). A restart gives it by construction: `execve`
   replaces the process image and ends every other thread, and the webview goes with it; in the headless server
   build, where the page lives in a separate browser, the page reloads once the application answers again. The
   Only a random **startup id**, drawn by each process image, proves the restart (an execution in place changes
   neither the pid nor the process start time, and `/health` answers for an old image as for a new one): the page
   closes its bridge before asking, never reopens it to the image it started with, and reloads only on another id.
   On Linux the executed file is `/proc/self/exe`, the running image itself whatever became of its path, with an
   argument list and an environment rebuilt explicitly. Across the restart goes one fact, « a take-over was asked »;
   the next image chooses the message from the standing it actually loaded, so a write that failed before the
   install (old list), succeeded, or succeeded with a finalisation error (`errCommitted`: the new list loads) is
   each told truthfully. Before the execution every child and service has ended and no descriptor is inherited; if
   the execution fails the runtime is already closed, so the process says so in a notification and exits: the list
   is on disk for the next launch.

7. **The join follows the same rule.** An accepted invitation into another project is written during the shutdown
   and followed by the same restart, with a pending-join record so that the next start opens the inviter's document
   (rules J-1..J-5, written on 2026-11-15 in `docs/plans/2026-11-15-pc1b-join.md`, block PC1b, after PC1a's take-over
   and restart; model `docs/protocol/tla/ProjectChange.tla`). Until PC1b is merged the join behaves as on `main`.

## Considered options

| Option | Verdict |
|---|---|
| Change the project inside the running session, reset the adapter when the bound call returns (revision 6, first implementation) | **rejected by review**: until the reply arrives the old coverage admits a head attested only under the old project (`WailsNetworkAdapter.ts`, admission reads coverage), and an attestation of the new project received before the reply is wiped |
| …pause the bridge stream before the call, reset, reopen and let Go replay its state | **rejected by review**: frames already handed to automerge-repo continue after the pause (`CollectionSynchronizer.receiveMessage` awaits `shareConfig.access` and `repo.find`; `removePeer` cancels nothing; `DocSynchronizer` load callbacks) and are applied under the old authorization |
| …and a generation boundary in Go (not built) | the same class again on the Go side: `AdoptProject` clears the attestation store but not the 64-event buffer (`wiring.go:32`, `:191`, `:326`), `RecvEnvelope` revalidates PeerUp and Message only (`:734-739`), `Router.Run` picks the connection after dequeuing (`router.go:103-111`); every fix needs one more ordering argument across four asynchronous layers |
| Carry the project identity in an ordered bridge envelope | changes the public `docs/protocol/bridge` format and still leaves automerge-repo's pending continuations; not needed once nothing changes live |
| Restart the runtime inside the process and reload the webview | the Wails services are bound once to one `Session` (`cmd/inkwell/main.go:113-121`); a second runtime in the same process keeps the old goroutines' closing as a correctness condition |
| **Take over in place, by a restart** (chosen for R1) | the smallest sound change that gives the person their document back: it moves no file, needs no project switch and no ordering argument (the list is written while nothing runs, then a new process loads it); the old list is archived, and the consequence is said before the action |
| Copy into a new folder (revision 4) | **deferred to the Projects screen, after R1.** Codex's review of revision 4 found five majors that all come from the copy: opening the original already writes to it (`docs.go:103`, `projection.ts:92`); `project.Create` has no complete failure contract (`project.go:127-143`); a staging folder can be picked by `Latest` (`project.go:255-264`); seeding the copy's text is not idempotent (`Repo.import` returns before persisting, `workspace.svelte.ts:381-389`); and the application has no way to switch project while running (`main.go:221-229`, `openRequests.ts:51`). Each is real work that belongs with the screen that lists and opens projects |
| Keep running as a stranger, fix the button only | text typed there reaches nobody and is not attributable to a member; the person is not told |
| Do not start the network for a non-member | refused: joining needs the node; no gain once the local folder is trusted |
| Ownership registry and restricted standings (revision 3) | withdrawn: defends against prior local write access, out of scope (W64) |
| Genesis freshness for every creation (minted root, revisions 3 and 4 « W58b ») | residual after R1: not needed by the take-over, whose genesis carries the document's own root |

## Consequences

- A stranger can no longer type into a document that is not theirs; they read, then take it over or join.
- After a take-over, the people of the old document are no longer connected to this folder: each needs a new
  invitation. The confirmation says so.
- After the restart this device, now a writer of the new project, attests the heads of the text as any session does
  at start: that makes the earlier text reach a person invited after the take-over. This is valid for
  the admission boundary **as executed today**: it accepts an attested head and its ancestors without checking their
  authors (`frontend/src/net/coverage.ts:42-46`, `:88-102`; `internal/app/attest.go:124-140`, `:152-161`). It does not
  implement the stricter rule ADR 0031 plans (lines 257-258, 285-300); residual R-5, to revisit with step 8 of
  ADR 0034.
- After a take-over the old list stays in `trust-archive/`.
- Two profiles that share one folder, or a folder synchronised between two machines, can take the document over in
  turn, each time disconnecting the other. Accepted (W64: the folder is the person's); the right tool there is an
  invitation, which the banner offers beside it (residual F16 of the OI plan, unchanged).
- Residual R-1: a missing root document is still replaced by an empty one (`workspace.svelte.ts:383-389`).
- Residual R-2: two projects created by one device with one title and an empty root share a `ProjectID`; closed by
  genesis freshness at creation, after R1 (`Repo.import(binary, { docId })` accepts a chosen id; an empty root stays
  wire-compatible — both checked by Codex on revision 4).
- Residual R-3: a launcher that isolates the profile but not the Documents folder still produces this state, now a
  readable one; lint the launchers.
- Residual R-4: the copy into a new folder, with the five findings above, at the Projects screen.

### Revision 7: the restart, the join, what remains

- The take-over costs the person a restart (the window closes and reopens on the same document; about one second
  expected, to measure in the walkthrough). It is a rare act, done once per recovered folder, and the confirmation
  says it.
- A non-member is read-only, so no text of its own is pending at that moment; the restart goes through the normal
  shutdown, which closes the stores as closing the window does.
- **Joining is unchanged in this block and keeps replacing the identity of a running session, as on `main`**, for
  any adoption of another project (`invites.go:379-419`): every accepted invitation into a project other than the
  current one, whoever was in the previous one. The races found on the take-over apply to it: residual **R-6, a
  known defect that blocks R1**. It is closed by block **PC1, mandatory before R1**, with the same restart, which
  first needs a pending-join record so that the next start opens the inviter's document. W58 adds no live change
  of its own and does not make « Demander une invitation » safe: it stops offering an invitation that cannot work
  and leads to the existing dialog.
- R-7: between the release of the profile lock and its re-acquisition another launch can become the primary; the
  restarted image then hands over and exits (ADR 0037, R10); the take-over is already on disk.
- R-8: the restart is specified for Linux; elsewhere the take-over is not offered until the port is written.
- R-9: after a failed execution the next manual launch opens the most recent project, which may be another folder.

## Sources

- Owner's rule of 2026-11-14 (W64); `docs/engineering-principles.md` §4.
- Codex review of revision 4 (2026-11-14), `gates/W58/REVIEW.md` « Round 4 ».
- rename(2) and Dan Luu, « Files are hard » (fetched 2026-11-14, https://man7.org/linux/man-pages/man2/rename.2.html,
  https://danluu.com/file-consistency/): the stage, sync, rename, recover discipline that `opStore.replace` already
  follows, and why a crash is tested at each of its steps rather than assumed atomic.
- RFC 8032 §5.1.6 (Ed25519 signing is deterministic) — https://www.rfc-editor.org/rfc/rfc8032#section-5.1.6.
- automerge-repo 2.5.6, `Repo.import(binary, { docId })` (`frontend/node_modules/@automerge/automerge-repo/dist/Repo.d.ts:127`):
  a document can be created under a chosen id; storage carries no owner
  (https://automerge.org/docs/reference/repositories/storage/).
- Code facts cited above, read on `main` 47b15ee.
- `execve(2)`, Linux man-pages (the calling process's image is replaced, the PID does not change, all other threads
  are destroyed, descriptors without close-on-exec stay open): <https://man7.org/linux/man-pages/man2/execve.2.html>;
  `flock(2)` (the lock belongs to the open file description and goes when it is closed):
  <https://man7.org/linux/man-pages/man2/flock.2.html>. Cited from the pages as known; not fetched again for this
  revision.
- automerge-repo's synchronizers, read in `frontend/node_modules/@automerge/automerge-repo/dist/synchronizer/`
  (`CollectionSynchronizer.js:96-111`, `:156-164`; `DocSynchronizer.js:85-108`, `:279-295`).
