# Lot P2 — rapport reconstitué

Rapport reconstitué le 2026-11-08 : les originaux (`gates/P2/SUMMARY-impl.md`) ont été perdus (non versionnés, worktree supprimé). Ce document ne contient que ce que prouvent les sources citées ; chaque affirmation renvoie à sa source (fichier, critère du JSON, commit).

Abréviations : `P2.json` = `gates/P2.json` ; `P2.toml` = `gates/P2.toml` ; `229420d` = « gate(P2): frozen acceptance for ADR 0034 step 2 (N-1, protocol/sig) and its plan » ; `df05d21` = « feat(protocol): ADR 0034 step 2 (N-1), strict Ed25519 in protocol/sig » ; `.remember/` = `.remember/today-2026-11-08.md`.

## 1. But (plan `docs/plans/2026-11-08-P2-implementation.md`)

ADR 0034, étape 2 : un seul point de vérité pour accepter une clé et une signature Ed25519, appelé partout où un octet venu d'un pair est vérifié ; clôt N-1. Cinq travaux annoncés : vérification stricte `Verify` dans `protocol/sig`, conformité Go pilotée par `INKWELL_PROTOCOL_STEP`, conformité TypeScript, appelants N-1 (filtre de connexion, `KeyFromID`, ops, attestations, décisions, ticket, rechargement, import), fuzz `FuzzVerify`. Le plan prévoit « Revue indépendante (Codex ou Opus) avant fusion ».

## 2. Verdict de la porte (`gates/P2.json`)

Statut global : `pass`, `deferrals: []`, `known_failures: []`. `base_guard` : `main`, base `5765331c…`, `pass` (`P2.json`). `docs/objectif-en-cours.md` écrit « 7/7, régression L3 comprise » alors que `P2.json` liste 6 critères ; l'écart n'est pas expliqué par les sources (voir Inconnu).

| Critère | Verdict | Durée | Source |
|---|---|---|---|
| `P-VEC-2-go` (`TestCorpus`, étape 2) | pass, exit 0 | 0,29 s | `P2.json` ; `gates/P2/P-VEC-2-go.log` |
| `P-VEC-2-ts` (vitest `packages/protocol`, étape 2) | pass, exit 0 | 0,67 s | `P2.json` |
| `N-1-callers` (`./sig ./trust` dans `protocol`, puis `p2p`, `trust`, `invite`, `identity`) | pass, exit 0 | 15,3 s | `P2.json` |
| `P-FUZZ-sig` (`FuzzVerify`, 120 s) | pass, exit 0 | 120,9 s ; journal : 12 576 913 exécutions, 22 entrées intéressantes au total, `PASS` | `P2.json` ; `gates/P2/P-FUZZ-sig.log` |
| `P-ARCH` (`scripts/arch-check.sh`) | pass, exit 0 | 2,9 s | `P2.json` |
| `L3-regression` (`mise run gate -- L3`) | pass, exit 0 | 1432,9 s | `P2.json` |

## 3. Ce qui est livré (`df05d21`, message de commit)

- `sig.Verify` : clé valide (§3.2), `S < L`, encodage canonique de `[S]B - [k]A` octet à octet égal à `R` (sans cofacteur, spec §3.3) ; `FuzzVerify`.
- Conformité : `INKWELL_PROTOCOL_STEP` pilote le corpus ; `Strict` route les décodeurs de clé et de signature vers `protocol/sig` ; le lanceur TypeScript fait de même.
- `protocol/trust` : ops et attestations vérifient via `sig.Verify` (fichiers `protocol/trust/op.go` et `attest.go`, `git show --stat df05d21`).
- `gates/P2.toml` : commande `N-1-callers` corrigée (module imbriqué, pas d'`internal/policy`), dérogation justifiée ; « full gate green incl. L3 ».
- Provenance indiquée par le commit : « Implemented by a free model (DeepSeek V4.1 Flash via opencode), reviewed against the spec and gated here. »
- Autres traces (`.remember/`) : 14 vecteurs et 181 sous-tests branchés côté Go et TS, appelants N-1 via `sig.Verify`/`KeyFromID` ; poussé avec `30ee378` (« lanceurs gratuits `scripts/opencode-free.sh`, porte 7/7 OK »).

## 4. Correction de la porte (aucun critère affaibli)

`N-1-callers` ne pouvait pas s'exécuter tel qu'écrit à l'origine : `protocol/sig` et `protocol/trust` sont dans le module imbriqué `protocol`, et `internal/policy` n'existe pas. La commande corrigée teste les mêmes paquets, plus `protocol/trust` ; « Aucun critère ni seuil n'est affaibli » (plan P2, section « Correction de la porte » ; commentaire « Corrected 2026-11-08 » dans `P2.toml`). Entrée de dérogation correspondante dans `gates/guard-overrides.json` : « P2 gate written the same day with a command that could not run… » (`P2.json`, `guard_overrides`, dernière entrée). Une panne `base-guard` a été corrigée avant l'exécution finale : « garde `base-guard` fix (`gates/guard-overrides.json`) », puis « `mise run gate -- P2` relancé post-fix, exit 0, verdict propre » (`.remember/`).

## 5. Revue et simplification

- Revue : le commit dit « reviewed against the spec and gated here » ; le plan exige une revue indépendante (Codex ou Opus). Aucune source ne nomme le relecteur ni ne liste de constats.
- Simplification : `gates/P3/SIMPLIFY.md` s'intitule « /simplify — protocole étapes 2 et 3, outillage formel (2026-11-08) » et couvre donc aussi l'étape 2 ; il est versionné et fait foi. Le rapport propre à P2 est perdu : `.remember/` écrit « L3/P2 SIMPLIFY reports lost to worktree delete, unrecoverable ».
- Aucune source ne mentionne de passe `ux-walkthrough` pour P2.

## 6. Inconnu

- Le contenu de `SUMMARY-impl.md` : aucun extrait ne survit.
- Les constats de la relecture indépendante de l'étape 2 (relecteur, constats, acceptés ou écartés).
- Le décompte « 7/7 » (`docs/objectif-en-cours.md`, `.remember/`) face aux 6 critères de `P2.json`.
- Le rapport `SIMPLIFY.md` propre à P2 : seule sa perte est attestée ; la part de `gates/P3/SIMPLIFY.md` relative à l'étape 2 n'a pas été attribuée ligne à ligne ici.
- Le détail des essais de modèles gratuits précédant l'implémentation retenue (la note cite plusieurs échecs de modèles OpenRouter gratuits et un essai DeepSeek V4 Pro) : non repris, hors périmètre des critères.

> Écart « 7/7 » de `docs/objectif-en-cours.md` résolu le 2026-11-08 : la porte P2 compte 6 critères (`gates/P2.json`) ; le texte dit désormais 6/6.
