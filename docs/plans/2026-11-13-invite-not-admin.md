# Invite refused: "trust: signer is not an admin" (2026-11-13)

Status: diagnosis, no code changed. Rigour of the fix: **complete** for the Go part (trust and admission),
standard for the UI part, light for the task.

## 1. Symptom

`mise run app:duo`, both instances: « Partager » → « Inviter » shows
« Impossible d'inviter : RuntimeError: invite: invite: trust: signer is not an admin ». Header « Sans titre /
main.typ ». Same result with an emptied `.task/duo/` (fresh profiles).

## 2. Root cause (measured)

The profiles are fresh; **the project is not**. `app:duo` isolates the identity and the webview data but not
the projects folder, so both instances open the developer's real `~/Documents/Inkwell/Sans titre`, whose
member list was created on 2026-11-05 by a third device key.

| Evidence | Where |
|---|---|
| `app:duo` sets `INKWELL_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`, not `INKWELL_DOCS` | `mise.toml:424` |
| Without `INKWELL_DOCS`, Docs is `Documents/Inkwell`; the latest project there is opened | `internal/app/docs.go:22-40`, `:85-104` |
| Both running instances hold `/home/…/Documents/Inkwell/Sans titre` open | `/proc/<pid>/fd` of pids 343783 (alice), 343785 (bob) |
| That project has one trust op, the genesis, dated 2026-11-05 16:36, author key `088466c1400f8d09…` | `.inkwell/trust/04837d9e….op` (186 bytes) |
| Device keys: archive alice `fc0b71f7…`, bob `6a33022f…`; fresh alice `5f96d879…`, bob `53fb71a9…` — none is in the genesis | `openssl pkey -pubout` on each `device.key`, searched in the op bytes |
| The archive holds no project and no member list at all, only `device.key` (22:18) | `duo-state-not-admin/{alice,bob}/home` |
| A genesis is only written when the folder has none; an existing list is loaded as is, whoever signs | `internal/app/persist.go:166-179` (`opStore.load`) |
| `Start` never checks that the device is a member of the list it loaded | `internal/app/node.go:159-219` |
| The refusal: `List.Role(SelfKey) != RoleAdmin` → `trust.ErrNotAdmin`, wrapped twice with `invite:` | `internal/invite/flow.go:149-151`, `internal/app/invites.go:225-227`, `protocol/trust/op.go:52` |

So a first-launch creator **is** admin of a project it creates (`opStore.load` signs the genesis with its own
key). Here nothing was created: each instance is a stranger in a folder that belongs to another identity.
The original key `088466c1…` is not in `~/.config/inkwell` (absent); which run made it on 2026-11-05 was not
established.

`main` has the same defect: `git diff main` is empty for `mise.toml`, `internal/app/{docs,persist,invites}.go`,
`internal/invite`, `protocol/trust` and `ShareDialog.svelte`. SI1b did not cause it.

Side effect to know: two processes with two identities have been writing the developer's real project folder
(`main.typ`, `.inkwell/repo`, `.inkwell/backups/20261113T2118…–T2154…`). This is residual F16 of
`docs/plans/2026-11-11-oi-single-instance-open.md` (« a different profile with the same Docs folder »).

## 3. Can a real person reach it?

Yes, without any test harness. The state is "the project folder's member list does not name this device":

1. the profile is lost and Documents is kept (system reinstall, new machine with restored documents, deleted
   `~/.config/inkwell`);
2. a project folder is copied or synchronised from another machine or person (USB key, cloud folder, backup);
3. two system accounts or two `INKWELL_HOME` share one Documents folder.

In all three the person sees their own document, « Seul », and an Invite button that fails. A fourth, legitimate
case reaches only the UI defect: a member who is a writer or reader of someone else's project (handled, see §4).

## 4. UI defects

- **Button offered.** `Workspace.mayInvite` returns true when `roleOf(myId)` is `undefined`
  (`frontend/src/workspace/workspace.svelte.ts:572-576`: « true while Go has not said »). It merges two states:
  roles not reported yet, and roles reported but this device absent from them. A known writer or reader is
  correctly disabled with « Seul un admin peut inviter quelqu'un. » (`ShareDialog.svelte:51-52`, UX-S7-4); a
  stranger is not. `ShareDialog.svelte:57` (« renew ») has no guard at all.
- **Raw Go error.** `workspace.svelte.ts:633`: `t("error.host", { reason: String(err) })`, with
  `fr.json:32` « Impossible d'inviter : {reason} ». Already listed as G12 in
  `docs/plans/2026-11-09-ux-research.md:224` (technical reasons inside sentences); it breaks the glossary rule.
- Seen in passing: the default name « Personne 12D3 » uses the first characters of the peer id, which are the
  same for every Ed25519 device, so every unnamed person gets the same name.

## 5. Right behaviour

Only an admin invites (`internal/app/session.go:66-68`, `internal/invite/flow.go:142-144`); a non-admin member
sees the button disabled with the reason (walkthrough 2026-10-26, UX-S7-4). No spec says what a device does
with a project folder whose list does not name it: this is the gap. A device must never silently run as a
stranger in a project (single source of truth for members; every peer byte verified — a stranger's edits are
signed by nobody the list knows).

## 6. Proposed fix

1. **Tooling (light, now).** `app:duo` gives each person `INKWELL_DOCS=$PWD/.task/duo/$who/docs`, like
   `frontend/e2e/instance.ts:14-15` and `scripts/netsim/run.sh:150-151`. Test: a `test:py` or shell check that
   every task or script setting `INKWELL_HOME` also sets `INKWELL_DOCS` (the same class of mistake is possible
   in `scripts/perf/browser.cjs:9-10`, which sets both, by hand).
2. **Go (complete).** `Start` detects "device not in the loaded list" and reports it as a typed state instead
   of running. Product decision to record in `docs/autonomous-decisions.md`; recommended: the folder opens as
   a **foreign project**, read-only for sharing, with one sentence and two actions — « Ce document vient d'un
   autre appareil. » → « En faire une copie à moi » (new project, new genesis signed by this device, text and
   files kept) or « Demander une invitation » (join). Never re-sign or replace an existing genesis in place:
   that would let a copied folder forge authority. Needs an ADR (it touches the member-list invariant).
   Tests: `TestStartForeignProjectIsNotAMember` (list without this key → typed state, no invite host),
   `TestAdoptCopyMakesOwnGenesis` (new project id, this device admin, original folder untouched),
   `TestFirstLaunchCreatorIsAdmin` (fresh home and fresh docs → `Invite("writer")` succeeds; pins what §2 shows).
3. **UI (standard).** `mayInvite` distinguishes "not reported" from "reported, not a member" (`rolesKnown`
   exists next to it); the renew button takes the same guard; `host()` maps known errors to sentences and logs
   the rest, never `String(err)`. Tests: Vitest `mayInvite is false once roles are known and omit this device`,
   `host shows no Go text on failure`; e2e `foreign-project.spec.ts` — second profile on the first one's Docs
   folder: Invite is not offered and the adopt action yields a working invitation.

Coverage gap that let it through: every e2e, netsim and perf launcher isolates `INKWELL_DOCS`, so no test ever
opens a project with an identity that did not create or join it; `app:duo` is the only launcher that does, and
it is run by hand.

## 7. Where it belongs

- Item 1: tooling lot, on its own, before the next manual session.
- Items 2 and 3: one block in the OI family (« Écrire et retrouver », FM-OPEN), the home of residual F16; it
  is an **R1 blocker** because case 1 of §3 (reinstall, documents kept) leaves a person unable to share their
  own document with a message they cannot read. The G12 part of item 3 can ride with the UX review fixes if
  that block lands first.
