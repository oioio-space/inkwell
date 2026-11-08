# Lot L3 — rapport reconstitué

Rapport reconstitué le 2026-11-08 : les originaux (`gates/L3-setup/SUMMARY-premerge2.md`, `gates/L3-setup/UX-walkthrough.md`, `gates/L3-setup/SIMPLIFY.md`) ont été perdus (non versionnés, worktree supprimé). Ce document ne contient que ce que prouvent les sources citées ; chaque affirmation renvoie à sa source (fichier, critère du JSON, commit).

Abréviations de sources : `L3.json` = `gates/L3.json` ; `L3.toml` = `gates/L3.toml` ; `83fe637` = commit « feat(language): lot L3, clickable problems and Typst help from tinymist » ; `f66524f` = commit « style(frontend): biome format and import order after the L3 merge » ; `.remember/` = notes de session (`today-2026-11-05.done.md`, `today-2026-11-08.md`, `recent.md`, `remember.md`).

## 1. Verdict de la porte (`gates/L3.json`)

Statut global : `pass`, `deferrals: []`, `known_failures: []` (`L3.json`). `base_guard` : référence `main`, base `5765331c…`, statut `pass` (`L3.json`, champ `base_guard`). Le critère `PERF-TYPE-native` portait dans `L3.toml` la mention « PENDING … this deferral must be replaced by a real criterion » ; le JSON final le montre `pass`, avec mesure réelle et sans report.

| Critère | Verdict | Durée | Source |
|---|---|---|---|
| `ER-1-4` (e2e `problems.spec.ts`) | pass, exit 0 | 7,3 s | `L3.json` critère `ER-1-4` ; journal `gates/L3/ER-1-4.log` |
| `LSP-positions-STALE` (vitest `src/language`) | pass, exit 0 | 1,2 s | `L3.json` critère `LSP-positions-STALE` |
| `LSP-sidecar-offline` (tests Go `internal/language`, `internal/sidecar`, `tools/archexec`, puis `arch-exec-check.sh`) | pass, exit 0 | 2,0 s | `L3.json` critère `LSP-sidecar-offline` |
| `PERF-LSP` (exclusif) | pass, exit 0 | 98,7 s (86 s d'attente de charge) | `L3.json` critère `PERF-LSP`, `exclusive_steps` |
| `PERF-TYPE-native` (exclusif) | pass, exit 0 | 191,8 s | `L3.json` critère `PERF-TYPE-native` |
| `L2-regression` (`mise run gate -- L2`) | pass, exit 0 | 1131,4 s | `L3.json` critère `L2-regression` |

### Mesures consignées dans `L3.json` (`measurements`)

- `PERF-LSP` (seuil 100 ms, 40 cycles d'édition, 4 fichiers, 3019 lignes) : p95 complétion 3,97 ms, survol 11,10 ms, aide sur les arguments 0,53 ms ; statut `pass`.
- `PERF-LSP-browser` (client navigateur via Stream Wails réel, seuil 100 ms) : p95 complétion 20,3 ms, aide sur les arguments 15,6 ms, survol 16,0 ms ; statut `pass`.
- `PERF-TYPE-native` (mesure `native-work-v6-overhead-bound-phases`) : statut `pass`, `reasons: []`. Seuils : gestionnaire p95 8 ms, gestionnaire à froid p95 50 ms, écart d'image 50 ms (70 ms à froid), travail d'image p95 16 ms (50 ms à froid), surcoût d'instrument ≤ 1 ms. p95 mesurés : gestionnaire 8 ms, gestionnaire à froid 8 ms, gestionnaire à chaud 6 ms, travail d'image 14 ms, écart d'image 44 ms. Surcoût d'instrument : borne supérieure à 95 % de 0,137 ms. Images manquées à chaud : éditeur 3/30 contre NULL 1/30, p = 0,306 (test exact de Fisher, échec si p < 0,05). Contrôles positifs : 7 contrôles, tous `validated: true`. `objective_met: False` pour `objective_warm_handler_ms: 4` (objectif non atteint, qui n'est pas un seuil de la porte). `cold_missed_diagnostic: 22` sur 30 (diagnostic, non bloquant d'après le champ `method`). Preuves brutes citées par chemin et SHA-256 dans `raw_evidence` (`gates/L3-setup/native-raw-zzd3bw17/…`).

## 2. Ce qui est livré (commit `83fe637`, message de commit)

- N28 : erreurs Typst ancrées dans le texte et dans une liste Problèmes ; clic pour sauter, F8 / Maj+F8 ; positions UTF-16 <-> Automerge.
- N29 : complétion, aide sur les arguments (aussi sur « ( » et « , »), survol, depuis tinymist embarqué via une session LSP incrémentale avec backoff.
- Chemin de frappe : écriture Automerge après le rendu (`amSync`, rebasée sur les changements concurrents, vidée par sauvegarde/export/compilation et raccourcis), préchauffage de l'éditeur, messages Stream/Worker traités après le rendu, confinement CSS.
- `PERF-TYPE-native` : vraies frappes dans l'application de bureau réelle (Mutter RemoteDesktop), essais froid/chaud/NULL, contrôles positifs ; seuils selon les décisions de l'utilisatrice dans `docs/objectif-en-cours.md` ; « full gate green 3/3 ».
- « UX walkthrough fixes (UX-L3-1..7), simplify pass, compact gate verdict » ; bail de session d'instantané ouvert via `os.Root`.
- Fusion sur `main` : `82a4735` (« Merge branch 'main' into feat/l3-language », `git log --merges`), puis `f66524f` (6 fichiers, `git show --stat`). Détail des corrections de fusion : `.remember/today-2026-11-08.md` (« L3 fusionné `main` `82a4735` poussé (corrections : tests UI-13, compteur problèmes, confinement fichier, faux positifs secrets, format Biome `f66524f`) »).

## 3. Revues et corrections avant fusion

### 3.1 Première revue Opus (2026-11-05)

Source : `.remember/today-2026-11-05.done.md`, `.remember/recent.md`. Constats listés : dérogations de `gates/guard-overrides.json` aux justifications inventées ; `PERF-TYPE-native` différé ; bouton en trop dans la barre d'état (`StatusBar.svelte:106`) ; copie complète du projet à la compilation (`preview.go:324`) ; `PERF-LSP` irréaliste ; `didChange` en texte plein (`client.ts:196`) ; fenêtre native blanche hors bac à sable. Statut alors : corrections confiées à Codex, blocage par une décision de l'utilisatrice.
Suite documentée dans `docs/plans/2026-11-05-L3-implementation.md`, section « Review correction pass » : benchmark de langage de quatre lignes à cache chaud jugé insuffisant, critère de préparation native qui pouvait passer sans mesure, justifications de dérogations affirmant à tort des approbations et des revues (« Those claims are withdrawn ») ; le report de `PERF-TYPE-native` est remplacé par la décision de bloquer L3 jusqu'à la mesure native ; `NATIVE-readiness` supprimé.

### 3.2 Revue Opus avant fusion (`REVIEW-premerge.md`, 2026-11-08)

Source unique : `.remember/today-2026-11-08.md` et `.remember/remember.md`. Verdict BLOQUÉ : la porte échoue à `base-guard` (11 fichiers `scripts/perf` non listés, 5 dérogations périmées) ; `ER-1-4` et `files.spec` de L2 échouent à cause d'un second `.cm-content` introduit par `prewarm.ts:30` ; chaîne de régression L2/L1/L0 ; lint (18 dans `internal/nativeperf`) ; Go, vitest et `PERF-LSP` passent ; restait avant fusion : `.pnpm-store` non ignoré, 7 commits de retard sur `main`, `go mod tidy`, `ux-walkthrough`. La même note indique « 5 blockers + 2 defects fixed » et une nouvelle exécution complète de la porte (`SUMMARY-fixall.md`). Corrections nommées : préchauffage inerte (sans focus, détruit dans l'appel), 11 dérogations `scripts/perf` fidèles (5 mortes retirées), lint `nativeperf` à 0, `.pnpm-store` ignoré, `go mod tidy`, aide sur les arguments via `whenTypedWritten`, backoff de reconnexion LSP de 1 à 30 s avec gigue. Porte natives `INKWELL_NATIVE_PERF_RUN=1 mise run gate -- L3` « PASS 3/3 (cold p95 9 ms) » selon `.remember/remember.md`.

### 3.3 Relecture de qualité

Source : `.remember/today-2026-11-08.md`. Une relecture de qualité par Codex était lancée ; Codex ayant atteint sa limite d'usage, elle a été faite par un assistant Sonnet.

## 4. Passe `/simplify` (`SIMPLIFY.md` perdu)

Ce que les sources prouvent seulement :
- Une passe de simplification a eu lieu : message de `83fe637` (« simplify pass ») et `.remember/today-2026-11-08.md` (« wrote `gates/L3-setup/SIMPLIFY.md` »).
- Ajout d'un assistant de position CodeMirror `Text` avec test de propriété contre `toPosition` dans `frontend/src/language/positions.ts`, message d'erreur mis à jour : cité par `.remember/today-2026-11-08.md` dans la même entrée que l'écriture de `SIMPLIFY.md` ; la note ne dit pas explicitement qu'il s'agit d'un constat de simplification.
- Réduction du résultat de porte (de 9,6 Mo à 97 Ko, `summary.go`, `verdict.go`) : attestée par `.remember/today-2026-11-08.md` et le message de `83fe637` (« compact gate verdict »), non attestée comme constat de simplification.

## 5. Passe `ux-walkthrough` (`UX-walkthrough.md` perdu)

Constats retrouvés par les tests e2e qui en sont issus, `frontend/e2e/ux/l3.spec.ts` (ajouté par `83fe637`), titres exacts :
- UX-L3-1 : cliquer un problème dans une fenêtre étroite montre l'éditeur, pas l'aperçu.
- UX-L3-2 : la barre d'état ne se chevauche jamais elle-même quand le panneau Problèmes est ouvert.
- UX-L3-3 : l'aide sur les arguments se lit en texte simple et tient dans la fenêtre.
- UX-L3-4 : la complétion de fichier cache le dossier interne et n'affiche pas de carte vide.
- UX-L3-5 : une erreur trouvée seulement par le compilateur reste cliquable après sauvegarde.
- UX-L3-6 : les messages courants du compilateur sont en français dans le panneau Problèmes.
- UX-L3-7 : le compteur de problèmes dit comment atteindre le prochain problème et la liste.

Le message de `83fe637` dit que ces constats (« UX-L3-1..7 ») ont été corrigés. Corrections ultérieures attestées par `.remember/today-2026-11-08.md` : deux échecs UI-13 de `e2e/shell.spec.ts` sur la branche L3 (couleur de la première lettre de `@plan`, soulignement Typst L3), corrigés « 12/12 ×3 », correctif dans `ui/shell/StatusBar.svelte`. Exigences : `docs/exigences.md` N28 et N29 notent « passe `ux-walkthrough` faite ».

## 6. Dérogations de garde (`gates/guard-overrides.json`, reprises dans `L3.json`)

`L3.json` embarque la liste `guard_overrides`. Justifications visibles : scripts de porte nouveaux pour L3 (« new gate script for L3 (added file, no existing test/threshold/delay/reference changed) ») ; fichiers nouveaux absents de la base de fusion ; entrée `UX-LUP-1` (test de walkthrough qui expose l'étroitesse connue) ; entrée de portée N42 ; entrée PERF-MEM. Une entrée concerne P2 (voir `gates/P2/RECONSTRUCTED.md`).

## 7. Inconnu

- Le texte, les constats numérotés et la décision appliqué/écarté de `gates/L3-setup/SIMPLIFY.md` : seule l'existence de la passe est attestée (section 4).
- Le détail de `UX-walkthrough.md` : personas, scénarios, captures, gravités, constats éventuellement non retenus. Seuls les sept constats titrés par les tests UX-L3-1..7 sont prouvés ; il pourrait y en avoir d'autres, ou des notes non traduites en test.
- Le contenu de `SUMMARY-premerge2.md` : le nom indique une seconde synthèse avant fusion, mais aucune source ne décrit son contenu. Les notes `.remember/` ne documentent que `REVIEW-premerge.md` et `SUMMARY-fixall.md`.
- Les résultats des exécutions intermédiaires de la porte et des essais natifs (par exemple « 1/3 passe », « 3/6 ») au-delà de ce qu'en disent `.remember/` ; le contenu des preuves brutes citées dans `L3.json` n'a pas été relu pour ce document.
- L'attestation `/simplify` du commit (marqueur de pré-commit) et l'identité des relecteurs de la passe de simplification : non attestées par les sources.
