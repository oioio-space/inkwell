# PC1 - reviews

## Round 1 — Codex, 2026-11-15 (before the freeze of PC1a and PC1b): PC1a REJECT, PC1b REJECT

Verbatim:

> - **Majeur — PageGate supposé, sans mécanisme réalisable spécifié** : `docs/plans/2026-11-15-pc1a-take-over.md:18`, `docs/protocol/tla/ProjectChange.tla:95`. Fermer les streams ne supprime pas les continuations déjà confiées à automerge-repo. Définir une barrière couvrant aussi le stockage et les appels en vol, ou détruire le contexte éditeur avant la demande ; ajouter un test à continuation suspendue et son mutant.
> - **Majeur — Redémarrage du join contradictoire** : `docs/plans/2026-11-14-pc1-project-change-by-restart.md:43`. N-12 autorise le redémarrage uniquement après une reprise, alors que PC1b le réutilise pour rejoindre. Étendre explicitement le déclencheur aux deux demandes et prévoir test et mutant « join écrit mais ne redémarre pas ».
> - **Majeur — Couverture de falsification incomplète** : `docs/plans/2026-11-15-pc1b-join.md:17`. MJ1/MJ2 ne vérifient ni les refus de `CompleteJoin`, ni la persistance effective du document avant consommation ; J-3 n'a aucun mutant (`:19`). Ajouter des mutants ciblant chaque refus, la consommation avant stockage et le premier join sans redémarrage.
> - **Majeur — Conservation du texte insuffisamment contractualisée** : `docs/plans/2026-11-14-pc1-project-change-by-restart.md:73`, `docs/plans/2026-11-15-pc1b-join.md:17`. « Reste dans le dossier » ne précise ni récupération ni sauvegarde des modifications en cours ; l'ouverture branche ensuite la projection sur le même fichier (`frontend/src/workspace/workspace.svelte.ts:449`). Spécifier où retrouver l'ancien texte et préserver les dernières modifications ; tester join, sauvegarde du nouveau document puis récupération de l'ancien, avec mutant destructif.
> - **Majeur — Budget non garanti** : `gates/PC1a.toml:16`, `gates/PC1b.toml:16`. Plafonds cumulés ~205 min / 190 min, sans plafond global ; l'estimation PC1a atteint déjà 45 min + 1 s (`docs/plans/2026-11-15-pc1a-take-over.md:44`). Répartir des plafonds compatibles avec 45 min et réserver une marge, puis confirmer au smoke.
> - **Modéré — Modèle limité à une seule demande et sans concurrence de lancement externe** : `ProjectChange.tla:213`, `:251`. Ajouter reprise→join / join→join et le concurrent de R-7, ou nommer leurs tests et mutants d'intégration. L'archive copiée demande une preuve hors modèle.
> - **Modéré — Vérification documentaire permissive** : `gates/PC1a.toml:11`, `gates/PC1b.toml:11`. L'échappement produit une alternance dans la regex ; employer une recherche littérale de `| N-… |` / `| J-… |` ; contrôler le statut de la partie 2.
> - Confirmé : l'e2e Python embarqué se lit ; `WriteBeforeRestart` porte sur l'historique `cut` ; J-1 et MW5 sont distincts ; le point 7 de l'ADR renvoie à PC1b.
>
> Verdict : PC1a REJECT, PC1b REJECT.

### Dispositions (2026-11-15)

| # | Finding | Disposition | Where |
|---|---|---|---|
| 1 | PageGate assumed | **Design changed**: the page discards its JavaScript context before asking (N-15): editor read-only, `repo.flush()` and projection writes awaited (5 s deadline, else nothing sent), `repo.shutdown()`, `location.replace` to a static `restart.html` (no repo, adapter, editor or storage handle) which calls `TakeOver`/`FinishJoin`; Go accepts them only with no editor stream open. Model: `PageGate` → `DiscardContext`, the request empties coverage and continuations; mutant `OldContinuation` → `RequestBeforeNavigation` (caught by `NoOldStateAfterChange`). Tests: « PC1 a continuation suspended at navigation is never applied » and three others, `TestChangeRequestRefusedWhileEditorStreamsOpen`; mutant MW12 | shared plan N-15; PC1a N-9; PC1b J-1, J-2; `ProjectChange.tla`; README; `tla-check.sh` |
| 2 | Join restart contradictory | N-12's trigger is « a change request — take-over or join »; `TestRestartOnlyAfterARequestAndOnce` covers both kinds; mutant MJ5 `N-12 a join restarts`; model mutant `JoinNoRestart` (`RestartWanted <- TakeOverOnly`, joins only), caught by `ChangeCompletes` | shared plan N-12; PC1b mutants |
| 3 | Falsification incomplete | MJ6..MJ9 one per `CompleteJoin` refusal; a fifth refusal (no chunk in the Go storage) with MJ10 `J-1 consumed after storage` and `TestCompleteJoinAfterTheDocumentIsStored`; Vitest + in-test mutant for completion before `repo.flush()`; J-3 gets `TestFirstLaunchJoinRestarts` and MJ11 | PC1b J-1, J-3; gate PC1b |
| 4 | Text preservation | **Decided**: the joined document gets its own folder beside the current one (J-6); a join writes nothing of the current folder but its `.inkwell/join-staging/`; the name comes from the title, confined to a new direct child of the parent. In-flight edits flushed before navigation (N-15). Tests `TestJoinKeepsThePreviousFolder` (tree hash before, after the restart and after the joined document's first save), `TestJoinFolderNameConfined`, e2e « PC1-E2E after a join and a save of the joined document, the previous text is still in its folder »; destructive mutant MJ12, confinement mutant MJ14. R-11 rewritten | PC1b J-1, J-4, J-6; shared plan R-11; decisions log |
| 5 | Budget | Per-criterion caps: PC1a 42 min, PC1b 43 min; new criterion `budget` sums the gate's own `timeout` values and fails above 2 580 s (2 min of the 45 left for the uncapped text checks); the smoke measures each, a cap that does not hold splits the block | gates; plans « Budget » |
| 6 | One request, R-7, archive | N-13: one change per image (take-over→join, join→join, join→take-over refused) with `TestRequestsRefusedOnceTakeOverRequested`, `TestRequestsRefusedOnceJoinHeld`, MW13, MJ13; R-7: `TestRestartLosesLockToAnotherLaunch`; archive: `TestTakeOverArchivesOldList` and MW6; the plan says which three the model does not claim | shared plan N-12, N-13, « Formal model » |
| 7 | Docs check permissive | Literal search of « \n| id | »; the ADR status line must contain « part 2 PC1a accepted » / « part 2 PC1b accepted » (the ADR line now says each block writes it at its merge) | gates `docs`; ADR 0043 line 2 |

TLC after the changes: the four sound configurations hold (exhaustive, liveness), the 13 mutants and 4 witnesses are
each caught by the property of their README row (`INKWELL_TLA_ONLY='^ProjectChange' ./scripts/tla-check.sh`, 21 runs).

## Round 2 — Codex, 2026-11-15: PC1a REJECT, PC1b REJECT

Asked to confirm: (1) N-15 buildable as written, (2) J-6 preserves the text without new attack surface, (3) the mutant
lists falsify every refusal and both restart triggers, (4) the `budget` criterion is a real global cap.

Verbatim:

> 1. **PageGate — NOT RESOLVED** : la navigation remplace utilement la barrière supposée, mais N-15 ne contractualise pas la fin des opérations Go déjà reçues (`docs/plans/2026-11-14-pc1-project-change-by-restart.md:46`, `internal/bridge/storestream.go:443`).
> 2. **Déclencheur du redémarrage — RESOLVED** : reprise et join sont explicites, avec test et mutant (`docs/plans/2026-11-14-pc1-project-change-by-restart.md:43`).
> 3. **Falsification — RESOLVED au niveau spécification** : MJ6–MJ10 couvrent les refus et le stockage ; MJ11 couvre le premier join (`docs/plans/2026-11-15-pc1b-join.md:17`, `docs/plans/2026-11-15-pc1b-join.md:19`).
> 4. **Conservation du texte — RESOLVED au niveau spécification** : dossier distinct, vérification après première sauvegarde et mutant destructif (`docs/plans/2026-11-15-pc1b-join.md:22`), sous réserve de corriger N-15.
> 5. **Budget — NOT RESOLVED comme plafond global** : sommer les délais ne borne ni les commandes hors `timeout`, ni les surcoûts ; le lanceur n’impose aucun délai global (`gates/PC1a.toml:11`, `gates/PC1b.toml:11`, `scripts/gate.py:434`).
> 6. **Limites du modèle — RESOLVED** : demandes successives, lancement externe et archive sont explicitement délégués aux tests/mutants nommés (`docs/plans/2026-11-14-pc1-project-change-by-restart.md:65`).
> 7. **Contrôle documentaire — NOT RESOLVED intégralement** : recherche littérale corrigée, mais les deux mentions recherchées figurent déjà dans la phrase déclarant la partie 2 encore proposée : faux positif (`gates/PC1a.toml:16`, `gates/PC1b.toml:16`, `docs/adr/0043-foreign-project.md:2`).
> 8. **N-15, faisabilité Wails : oui pour la seconde entrée HTML**, embarquée et servie par le même serveur (`cmd/inkwell/main.go:118` ; [documentation Wails](https://v3.wails.io/contributing/asset-server/)).
> 9. **N-15, garantie absolue : non** : `shutdown()` déconnecte les adaptateurs réseau puis appelle `flush()` ; il ne détruit pas le repo (`frontend/node_modules/@automerge/automerge-repo/dist/Repo.js:740`).
> 10. La navigation supprime les continuations de la page détruite, mais ne constitue pas une annulation universelle du stockage : une fermeture IndexedDB attend normalement les transactions ([spécification IndexedDB §5.2](https://www.w3.org/TR/IndexedDB/#closing-connection) ; assertion excessive dans `docs/plans/2026-11-14-pc1-project-change-by-restart.md:46`).
> 11. **Majeur restant : préciser la barrière Go** : compter aussi le stream `store`, attendre la sortie de ses handlers et interdire leur réouverture pendant le changement ; « aucun stream éditeur ouvert » seul ne prouve pas ce drainage (`cmd/inkwell/main.go:138`, `cmd/inkwell/main.go:139`, `internal/bridge/storestream.go:412`, `internal/bridge/storestream.go:443`).
> 12. Le test suspendu doit observer l’écriture effective côté stockage, pas seulement l’absence d’appel au bridge après navigation (`docs/plans/2026-11-14-pc1-project-change-by-restart.md:46`).
> 13. **J-6 : nouvelle entrée non fiable vers le système de fichiers**, mais confinement et collisions sont contractualisés ; aucune traversée démontrée sous l’adversaire pair (`docs/plans/2026-11-15-pc1b-join.md:22`).
> 14. À préciser : installation atomique **sans remplacement**, avec nouvel essai sur collision ; hypothèse de concurrence entre deux créations légitimes, sans supposer de fichiers locaux hostiles (`docs/plans/2026-11-15-pc1b-join.md:17`, `docs/plans/2026-11-15-pc1b-join.md:22`).
> 15. **Autre majeur : commandes Python invalides telles qu’écrites** : dans les arguments shell entre apostrophes, les `\\n` TOML deviennent des caractères littéraux `\n`, pas des sauts de ligne Python ; corriger notamment `budget` et `docs` avant gel (`gates/PC1a.toml:11`, `gates/PC1a.toml:16`, `gates/PC1b.toml:11`, `gates/PC1b.toml:16`).
>
> PC1a REJECT
> PC1b REJECT

### Dispositions (2026-11-15, integrator's decisions D-A..D-E; third round = design decisions, method v2)

| # | Finding | Disposition | Where |
|---|---|---|---|
| 1, 9-12 | PageGate: N-15 does not bound the Go operations already received; `shutdown()` flushes, does not destroy; IndexedDB close waits for transactions; the Go barrier must count `store`, await its handlers, forbid reopening; the suspended-write test must observe storage | **D-A, design**: new rule N-16, Go drains the bridge before it writes (streams closed for good, write not taken refused with a sentence, handlers awaited with a 5 s deadline, join points `StoreServer.Serve`, `Router.Serve`, `storestream.go:443`, `main.go:138-139`); N-15 softened to best effort; `TestStorageWriteBeforeRequestLandsOrIsRefused` observes Go storage after the restart; MW12 redefined « list written before the drain »; model: bridge variables, `Drain`, property `NoLostWrite`, mutant `WriteBeforeDrain`, witness `WitnessWriteRefused` | shared plan N-15, N-16; ADR 0043 point 8; `ProjectChange.tla`; gates `go`, `tla` |
| 2, 3, 4, 6, 8 | resolved or confirmed | none | — |
| 5 | Sum of caps is no global cap | **D-B**: `max_minutes = 45` in each gate file, enforced by `scripts/gate.py` (refused before the run when the caps sum past it, run stopped at the deadline), rule G-1 of PC1a with `scripts/gate_deadline_test.py`; `budget` criterion dropped | PC1a plan « Tooling rule »; gates head, `gate-deadline` |
| 7 | Docs check false positive | **D-C**: the ADR status line is now tokens only (`part 2 PC1a proposed; part 2 PC1b proposed`), format in ADR « Status of the parts »; the check requires `part 2 <block> accepted` and the absence of `part 2 <block> proposed` | ADR 0043 line 2; gates `docs` |
| 13, 14 | J-6: peer-supplied title; install without replacement, retry on collision | **D-D**: normalisation made explicit (allowed characters, 64, no separator, no dot-name), exclusive staging folder in the same parent, rename without replacement per OS, « (2) »… up to 99; tests `TestJoinFolderNameConfined` (traversal-shaped titles), `TestJoinFolderCollisionGetsSuffix`, `TestJoinCrashDuringInstall`; mutant MJ15 `J-6 never replaces` | PC1b plan J-6; ADR 0043 point 9; gate `go`, `mutant-tool` |
| 15 | Embedded Python invalid | **Refuted by measurement** (TOML basic strings decode `\n` into newlines: before the change PC1a `budget` exited 0, `docs` exited 1 on missing catalogue keys, PC1b likewise); **D-E** applied anyway for clarity: every program is a heredoc in a TOML literal string, each run once (witnesses below) | both gates |

TLC after D-A (`INKWELL_TLA_ONLY='^ProjectChange' ./scripts/tla-check.sh`, 23 runs, 132 s): the four sound
configurations hold with `NoLostWrite` (exhaustive, liveness; Faults 998,929 distinct states, 51 s), the 14 mutants
(`WriteBeforeDrain` caught by `NoLostWrite`) and the 5 witnesses (`WitnessWriteRefused` reached) each by their
README property.

Red witnesses of the rewritten commands (2026-11-15, each run once): PC1a `docs` 1 (missing `foreign.takeOver*` and
`change.flushFailed` keys; « part 2 PC1a not accepted »; every plan row found, N-16 and G-1 included), `no-live-change`
1 (« takeOver not found »), `mutant-tool` 1 (MW3..MW13 missing), `gate-deadline` 1 (`scripts/gate_deadline_test.py`
absent), `frontend` and `e2e` Python parts on an empty report 1 (every named test missing); PC1b `docs` 1 (six `join.*`
keys, « part 2 PC1b not accepted », W58 not closed), `mutant-tool` 1 (MJ1..MJ15 missing), `frontend`, `e2e` 1 likewise.
The `go`, `mutants`, `restart-barrier` and `regression` commands are unchanged in form (shell only).

## Round 3 — 2026-11-15 (Codex, decision confirmation): PC1a CONFIRM, PC1b one major, decided

Confirm the design decisions D-A..D-E as dispositions of round 2: (1) N-16, Go drains `store` and `net` before any
write, join points as named, test observes Go storage, mutant MW12 and model `WriteBeforeDrain`; (2) G-1, `max_minutes`
enforced by `gate.py` before and during the run; (3) the ADR status tokens; (4) J-6 naming, staging, rename without
replacement; (5) heredoc commands. A remaining major finding is a design decision for the integrator, not a fourth round.

### Round 3 verdict and disposition

Codex: D-A YES (contract level; implementation to prove at `internal/bridge/storestream.go:443`), D-B YES (G-1 an
obligation before acceptance, not yet in `scripts/gate.py`), D-C YES, D-E YES; rename without replacement available on
the three systems (`renameat2(RENAME_NOREPLACE)`, `renamex_np(RENAME_EXCL)`, `MoveFileExW` without
`MOVEFILE_REPLACE_EXISTING`), failing safe where a filesystem lacks it. D-D NO: J-1 staged in `.inkwell/join-staging/`
while J-6 staged in the parent, and nothing said how the previous folder finds the staging folder after an
interruption. Verdict: **PC1a CONFIRM (freeze)**, **PC1b REJECT** on that one major.

Integrator decision (no fourth round, per method v2): one location, `.inkwell-join-<record id>` in the parent (J-1,
J-6); found again through a marker `.inkwell/join-pending.json` (`{v: 1, id}`) written atomically in the previous
folder **before** the staging folder is created; the next start of the previous folder removes the staging folder it
names, then the marker (J-4); after the install only the marker remains and is removed. New mutant MJ16 (staging before
the marker), crash test extended. PC1b is frozen on this disposition.

## Code review, PC1b — Codex, 2026-11-15 (after the smoke): one major, fixed

Codex read the code of the block (confinement of the joined folder's name, exclusive creation of the staging
folder, rename without replacement, recovery after an interruption, the refusals of `FinishJoin` and
`CompleteJoin`) and found no defect there. One finding:

- **F1 (major, blocking).** When the Welcome carries no root (every genesis today), the page recorded once and for
  good, through `CompleteJoin`, the root announced by the inviter **or** by another member of the joined list that
  announced first and served a document (`#resumeJoin` / `#announced`, `frontend/src/workspace/workspace.svelte.ts`;
  `internal/app/join.go` `CompleteJoin`). A Byzantine member could bind the newcomer's project to a document of its
  choosing, for good, even after that member is removed (this was residual RJ-1 of `gates/PC1b/SIMPLIFY.md`).

### Disposition (integrator's decision, the minimal sound fix)

While the record holds no root, only the inviter's announcement counts:

- Go: `CompleteJoin(url, peer)` refuses any peer other than the record's inviter when the record holds no root
  (`errJoinInviter`); with a root in the record the rule is unchanged (that root, from any member of the list).
  `TestCompleteJoinRefuses/not_the_inviter`, `TestCompleteJoinStatedRootFromAnyMember`, mutant MJ17
  (`J-1 CompleteJoin refuses a root not announced by the inviter`), beyond the MJ1..MJ16 the frozen gate names.
- Page: the wait follows the inviter's announcement alone; another member's is ignored, and the logic « the
  inviter's announcement replaces another member's » is removed. Vitest « PC1 a pending join without a root ignores
  what another member announces », in-test mutant « PC1 mutant J-1 a root announced by another member is caught ».
- Plan rows J-1 and J-2 say it. Residual: without a root in the record the wait lasts until the inviter is
  reachable, even with other members online. A root signed in the Welcome or in the member list lifts both the
  trust in the inviter's unsigned word and that wait: block TR1.
