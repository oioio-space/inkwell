---
status: proposed # revision 6: scope reset « Round 3 », reviewed twice by Codex (« Round 4 », « Round 5 », gates/W58/REVIEW.md), gate frozen 2026-11-14; accepted when W58 is merged
date: 2026-11-14
decision-makers: [Mathieu]
---

# A project whose member list does not name this device opens read-only, and is taken over in place or joined; inkwell does not defend the local folder

Diagnosis: `docs/plans/2026-11-13-invite-not-admin.md`. Plan: `docs/plans/2026-11-14-w58-foreign-project.md` (one block,
W58). Gate: `gates/W58.toml` (frozen 2026-11-14). Review history: `gates/W58/REVIEW.md`. Requirements:
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

1. **Standing.** Go reports whether this device is in the loaded member list: `member`, `removed` (the list holds an
   op whose subject is this device, `List.Since(nil)` and `Op.Subject`, but gives it no role) or `notNamed`. It is
   computed from the list, so a take-over or a join turns it into `member` without a restart.
2. **Not a member: read-only, nothing issued.** The webview shows the document through the existing reader path
   (`access`, `Editor.setReadOnly`, `AccessBanner`) with one sentence and two actions. The node keeps running exactly
   as today, because the join flow needs it (`invites.go:350-352`); a stranger is admitted by nobody, since every peer
   checks its own list.
3. **« Reprendre ce document ici » (take over in place).** After one confirmation that states the consequence, the
   existing member list is archived by copy in `.inkwell/trust-archive/<time>/` (never deleted, never rotated; not in
   `backups/`, whose rotation takes every name and would trade text backups for it, `kv.go:307-318`, `:464-470`) and this device signs a new genesis in the
   same folder. Text, files, binary store and root document stay where they are: no new folder, no project switch, no
   seeding. In code it is the join path with a list of one op: `Wiring.AdoptProject` (`wiring.go:312-337`) already
   resets the wiring's state for the new `ProjectID` (inventory in the plan), and `opStore.replace` (`persist.go:280-290`) already replaces the stored list by stage, swap and recovery.
   The genesis carries the folder's recorded root document URL when there is one, so the new `ProjectID` is tied to
   this document and cannot equal that of another project this device created with the same title. A member's
   take-over is a no-op.
4. **« Demander une invitation ».** Opens the existing join dialog; joining keeps today's behaviour (the member list
   is replaced in place, `invites.go:357-385`).
5. **Invite capability and refusals.** « Inviter » and « Créer une nouvelle invitation » are enabled only once Go has
   reported an admin role; before that they are disabled (« Vérification… »). A refusal crosses the bridge as a field
   of the result (`Invitation.Refused`, a stable key, like the existing `CodeError`), never as an error text: a Go
   error reaches the webview only as a string (`cmd/inkwell/main.go:113`, no serialiser). The Go text goes to the log.
6. **The webview follows a project change.** When the `ProjectID` changes (take-over, or a join that keeps the same
   root document) the network adapter replaces its coverage, clears its attestation cache and asks again for an
   attestation of every local head under the new project (`WailsNetworkAdapter.ts:83-102`, `:384`). Without it a head
   attested before the change is never attested again and a person invited afterwards receives nothing.

## Considered options

| Option | Verdict |
|---|---|
| **Take over in place** (chosen for R1) | the smallest change that gives the person their document back: it reuses the join path, moves no file, needs no project switch; the old list is archived, and the consequence is said before the action |
| Copy into a new folder (revision 4) | **deferred to the Projects screen, after R1.** Codex's review of revision 4 found five majors that all come from the copy: opening the original already writes to it (`docs.go:103`, `projection.ts:92`); `project.Create` has no complete failure contract (`project.go:127-143`); a staging folder can be picked by `Latest` (`project.go:255-264`); seeding the copy's text is not idempotent (`Repo.import` returns before persisting, `workspace.svelte.ts:381-389`); and the application has no way to switch project while running (`main.go:221-229`, `openRequests.ts:51`). Each is real work that belongs with the screen that lists and opens projects |
| Keep running as a stranger, fix the button only | text typed there reaches nobody and is not attributable to a member; the person is not told |
| Do not start the network for a non-member | refused: joining needs the node; no gain once the local folder is trusted |
| Ownership registry and restricted standings (revision 3) | withdrawn: defends against prior local write access, out of scope (W64) |
| Genesis freshness for every creation (minted root, revisions 3 and 4 « W58b ») | residual after R1: not needed by the take-over, whose genesis carries the document's own root |

## Consequences

- A stranger can no longer type into a document that is not theirs; they read, then take it over or join.
- After a take-over, the people of the old document are no longer connected to this folder: each needs a new
  invitation. The confirmation says so.
- The re-attestation of rule 6 makes the earlier text reach a person invited after the take-over. This is valid for
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
