# W58a - /simplify review of the block's diff (integrator, 2026-11-14)

Diff read in full against `main` 726dc47: `internal/app/{session,invites,persist}.go` and their tests,
`frontend/src/workspace/workspace.svelte.ts`, `net/WailsNetworkAdapter.ts`, `ui/{Editor,ShareDialog,
ExternalChangeToast}.svelte`, `ui/people/{AccessBanner.svelte,roster.ts}`, `ui/toolbar/FormatBar.svelte`,
`ui/history/history.svelte.ts`, `ui/sheet/{Sheet.svelte,sheet.svelte.ts}`, the catalogues, `scripts/netsim/
{mutants.py,mutants.toml,mutants_test.py}`, the e2e and walkthrough specs.

| Finding | Decision |
|---|---|
| The take-over, its archive, `JoinResult`, `pause`/`resume`/`projectChanged` and three successive live-reset designs were written and then removed (split into W58a and PC1) | applied: `git diff main` shows none of them; `adopt` is back to main's |
| One lock (`workspace.locked`) for every writer on a user action: editor, format bar, history actions, CSV sheet, the two outside-edit actions | kept: one rule in one place instead of seven `access` checks; the CSV sheet hole (a reader could edit a CSV) predates W58 and is closed by the same getter |
| `refusal()` logs `err = trust.ErrNotAdmin` also for « notMember » | kept as is: the log line names the key, which is what an operator reads; changing the error would add a sentinel used nowhere else |
| `Session.Standing` reads the wiring's list, `refusal()` the invitations' list | kept: Codex accepted it (round 3); the wiring announces roles after it follows a new project, the invitations only after |
| `rightsKnown` reads the role through the adapter without touching `rolesVersion` | checked, no defect: the order « standing first, role later » unlocks the editor (new test « W58a the editor unlocks when the role arrives after the standing », passing) |
| `#shared` (a document outside the admitted scope was counted as shared) | kept, with its red-first test; independent of the removed live-reset designs |
| `refreshSync` on `whenReady` | kept, weaker evidence (mock-based test), recorded in the plan |
| `listPending` and its sentence while a join waits for the member list (walkthrough UX-2-1) | kept: the only signal for a 1-15 s wait; the 15 s cause goes to IV1 (UX-2-2) |
| The walkthrough driver `frontend/e2e/ux/w58a.spec.ts` is skipped unless `W58A_UX=1` | kept as evidence tooling, declared in `.env.reference`, guard override with its reason |

Secrets: none. Artefacts: none staged (captures under `gates/W58a/ux/` stay ignored).
