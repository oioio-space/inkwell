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

## Round 2 — pending (Codex, short)

Confirm: (1) N-15 is buildable as written (Wails asset server serves `restart.html` as its own entry; `repo.shutdown()`
then `location.replace` ends every automerge-repo continuation; Go's « no editor stream open » check), (2) J-6 (new
folder beside, name confinement, staging under `.inkwell/`) preserves the text and adds no new attack surface, (3) the
mutant lists now falsify every refusal and both restart triggers, (4) the budget criterion is a real global cap.
At most 20 lines, one verdict per block.
