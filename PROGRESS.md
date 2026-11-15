# inkwell — état & feuille de route

Finalité : [docs/conception.pdf](docs/conception.pdf). **Premier jalon mesurable : à la fin
de l'étape 3, écrire le document de conception à deux dans l'application.**
Chaque étape livre quelque chose qui tourne ; l'ordre met les risques techniques en premier
et l'habillage en dernier. Une étape n'est close que lorsque son **critère de sortie** est
démontré (test, démo enregistrée ou tâche utilisateur chronométrée).

## Où en est-on (mis à jour à chaque bloc commité)

Seul état courant du projet. Règles de travail : [CLAUDE.md](CLAUDE.md) ; carte de la documentation :
[docs/README.md](docs/README.md) ; registre lu par les outils : [docs/objectif-en-cours.md](docs/objectif-en-cours.md).

**Objectif en cours : la release R1** — une application utilisable par des collaborateurs de confiance (alpha),
définie le 2026-11-12 dans [docs/autonomous-decisions.md](docs/autonomous-decisions.md) (lignes « release R1 »).

| # | Bloc R1 | État | Détail |
|---|---|---|---|
| 1 | PB — mesure appariée | ✅ `main` | ADR 0038 |
| 2 | C5S — qualité du lien | ✅ `main` (`d051af2`) ; reste l'affichage dans la fenêtre des personnes : bloc C5S-UI, [plan](docs/plans/2026-11-13-c5s-ui-link-level.md) prêt, après PR1 | ADR 0036, `gates/C5S.toml` |
| 3 | OI — instance unique | ✅ `main` (`e3bc987`) ; fenêtre ramenée au premier plan avec le jeton d'activation (purego, sans cgo) ; validé par l'utilisatrice avec le paquet installé le 2026-11-14 (`gates/OI/UX.md`) | `gates/OI.toml`, ADR 0037 |
| 4 | RC1 — reconnexion après redémarrage | ✅ `main` | `gates/RC1.toml` |
| 5 | SI1b — états vrais enregistré / envoi / reçu | ✅ `main` (`0752872`) ; validé par l'utilisatrice en séance le 2026-11-13 (`gates/SI1b/UX.md`) | `gates/SI1b.toml` |
| 5b | SI1c → **HB1a / HB1b** — nouvelle barre du haut B (style GNOME, pastille centrale, aperçu à la taille de la zone d'écriture, version étroite) ; reprend les règles de SI1c ; `gates/SI1c.toml` jamais figé, remplacé | [conception](docs/plans/2026-11-13-report-and-header-design.md) |
| 6 | LF0a3 — coloration incrémentale | ✅ `main` (`427d74a`) ; résidu LF0b après R1 | `gates/LF0a3.toml` |
| 7 | SB1b — bac à sable : mémoire et descendants | ✅ `main` (`ca33ae3`) ; porte L1 imbriquée différée (`gates/deferred.jsonl`), à lancer à part | `gates/SB1b.toml`, ADR 0042 |
| 8 | DU — mises à jour ciblées | ✅ `main` : `uuid` 9.0.1 → 14.0.3 par surcharge `^14.0.3` (GHSA-w5hq-g745-h8pq) ; Go : rien d'appelé (govulncheck) | `gates/DU.toml`, `gates/DU.json` |
| 9 | PR1 — présence correcte (W46 : présence figée, couleurs en double, départ en 45 s) | 🔨 porte figée (`1975027`) ; code dans le worktree `.claude/worktrees/agent-a8b133d4e69426085` (non commité), 6 critères sur 9 verts ; reste la mesure `timers` dans la vraie application, le parcours d'interface, la relecture, le commit | [plan](docs/plans/2026-11-13-pr1-presence.md), `gates/PR1.toml` |
| 10 | Paquet deb/rpm + parcours à deux personnes | 🔨 premier essai fait le 2026-11-13 sur `main` d3d0829 : `pkg:build`, `pkg:lint` et `pkg:test` (PKG-01…11) verts sur Debian 13, Ubuntu 24.04, Fedora 44 ; reste le parcours à deux personnes avec le paquet | `docs/releasing.md` |
| 11 | **W58a** — un document venu d'un autre appareil s'ouvre en lecture seule et ne propose plus d'invitation qui échoue (bloquant R1) ; **PC1** — le projet change par redémarrage, jamais à chaud : reprise d'un document et invitation acceptée (bloquant R1, rigueur complète) | ✅ W58a sur `main` (porte `gates/W58a.toml` verte, parcours accepté par l'utilisatrice avec sa limite : la personne seule lit et copie son texte sans pouvoir le reprendre) ; 📝 **PC1a** (reprise) puis **PC1b** (invitation acceptée), spécifiés et modélisés, à relire par Codex puis à figer, obligatoires avant R1 | [W58a](docs/plans/2026-11-14-w58a-foreign-read-only.md), [PC1](docs/plans/2026-11-14-pc1-project-change-by-restart.md), ADR 0043, `gates/W58/REVIEW.md` |
| 12 | RP1 — outil de signalement (W51) | 📝 conception validée avec l'utilisatrice ; à planifier | [conception](docs/plans/2026-11-13-report-and-header-design.md) |
| 13 | UX1 — gains d'interface immédiats de l'étude W63 : recherche et remplacement branchés (P19), annuler plutôt que confirmer (P24), menu principal et raccourcis standards (P14, P21), typographie française et un mot par chose (P29, P30), règle des cibles (P31) | 📝 retenu par l'utilisatrice le 2026-11-14 ; à planifier après W58 et PR1 ; les autres propositions retenues suivent HB1a/HB1b (P9, P11–P13, P16–P18), UF1 (P1–P6, P15, P22, P28) puis les blocs d'après R1 (P7, P8, P10, P20, P23, P25, P27, P32) | [étude](docs/plans/2026-11-14-ux-ui-study.md) |
| 13a | **EX0 — registre unique des contributions** (premier morceau des « Fondations extensibles », sans greffons) : commandes, menus, raccourcis, barre d'outils, panneaux et types de fichiers s'inscrivent dans un seul registre (`docs/architecture/spec.md` §15.2 X-2, X-3, X-6), le cœur s'y inscrit comme le ferait une extension, la palette et la liste des raccourcis le lisent | 📝 avancé le 2026-11-14 avant les points P14/P21 de UX1 et avant HB1a, qui sinon coderaient en dur menus et raccourcis ; l'hôte des greffons, les capacités et l'API restent au lot « Extensions utilisables » (v1) | spec §15 |
| 14 | W65 — refonte de l'affichage des modifications des autres dans le texte, des différences, des versions (arbre gardé et redessiné), des fichiers, des onglets et des surfaces jamais dessinées ; W66 — les designs décidés sont tenus | 📝 retenu en entier par l'utilisatrice le 2026-11-14 ; ordre : gardes G1–G3 (avec UF1) juste après UX1, puis la zone de texte (avec PR1 et CP1), puis versions et différences (bloc « Relire »), puis fichiers et onglets (après G-0b) | [étude](docs/plans/2026-11-14-ux-diff-versions-files.md), `docs/design/reference/` |
| 15 | SB2 — bac à sable du compilateur : 6 constats de la revue approfondie du 2026-11-14 (limites d'autres processus modifiables dans un repli, tinymist sur arm64, démarrage sous AppArmor d'Ubuntu 24.04, repli cgroup) ; BR1 — pont : 4 constats (files de sortie sans plafond en octets, pairs inconnus, journal, borne recopiée) | 📝 constats à rejouer d'abord (un test rouge chacun) ; SB2 juste après W58, avant PR1. Ordre dans SB2 : (1) Ubuntu 24.04 d'origine (W69, critère de R1) — d'après le code le bac à sable y échoue (sortie 125, tinymist muet) ; option retenue à spécifier : profil AppArmor dans le paquet **et** niveau de repli sans espaces de noms, annoncé à la personne ([diagnostic](docs/plans/2026-11-14-ubuntu-2404-sandbox.md)) ; (2) les deux défauts de sécurité (limites d'autres processus ; `setrlimit` jamais filtré sur arm64, confirmé) ; (3) tinymist sur arm64 ; (4) le reste. BR1 ensuite, avec l'identité du projet dans l'ordre du flux (résiduel R-6 de W58). **CI1c** (analyse de sécurité avec l'outil épinglé, simulation réseau sur GitHub) : porte `gates/CI1c.toml` verte le 2026-11-14, confirmé sur GitHub pour ce qu'il visait (`gates/CI1c/GITHUB.md`) ; il reste rouge sur GitHub, à traiter dans SB2 : les tests du bac à sable hors de la machine de développement (cgroup non délégué, un test qui ne rend pas la main) et arm64 ; la CI GitHub est rouge depuis le 2026-10-25 | [revue](docs/reviews/2026-11-14-ultrareview-sandbox-bridge.md) |
| 16 | **IV1** — invitation : un refus ne change rien (aucun ajout signé avant la dernière vérification), une demande finit avec sa fenêtre ; **AD1** — un seul décodeur des messages des pairs (celui du protocole), quota de documents par auteur signalé, encodeur et pas de conformité alignés sur Go ; **NA1** — adaptateur et qualité du lien | 📝 constats de la seconde revue approfondie (10, dont 6 « complet ») ; IV1 et AD1 bloquants pour R1, NA1 après W58a ; rejouer d'abord les points non confirmés des constats 2 et 3 | [revue](docs/reviews/2026-11-14-ultrareview-admission-invite.md) |

**Intégrateur : la session Claude Code** (seule à commiter ; sans Claude, voir « When Claude is absent » de la méthode v2 et `AGENTS.md`).

**Reprise (passation du 2026-11-15, 00:15, jetons presque épuisés) — à lire en premier :**
1. **Audit du protocole (ultracode)** : workflow `wf_04cd1a24-ce8`, résultats conservés dans `~/.claude/projects/-home-mathieu-GolandProjects-inkwell/61fa9e48-30a6-4e0a-bf70-5c7eb177fa69/subagents/workflows/wf_04cd1a24-ce8/journal.jsonl` ; s'il n'a pas fini, le reprendre avec `Workflow({scriptPath: "~/.claude/projects/-home-mathieu-GolandProjects-inkwell/61fa9e48-30a6-4e0a-bf70-5c7eb177fa69/workflows/scripts/protocol-conformance-audit-wf_04cd1a24-ce8.js", resumeFromRunId: "wf_04cd1a24-ce8"})` (les agents terminés reviennent du cache, sans coût) ; puis écrire `docs/reviews/2026-11-14-protocol-audit.md` et ranger les constats dans TR1, IV1, AD1.
2. **SB2a1** : porte figée (`8d23ecd`) ; worktree `../inkwell-sb2a1` (branche `sb2a1`, non commité, sauvegarde `~/inkwell-archive/2026-11-15/sb2a1-partial.patch`) : faits — descripteur par programme, `decideTier`, classes d'erreurs (EACCES compris), règles du filtre sur les deux architectures (tests rouges d'abord, 7 mutations attrapées), outillage (`gojson_check.py`, `gh-verdict.py`, 56 tests), tâche CI Ubuntu d'origine, interface (ligne « isolation réduite », phrases de refus), chemin de lancement (sonde = lancement réel, un réglage par niveau, poignée de main avec délai, filiation en F, polices exactes, refus du projet protégé, `NetworkMode` « reduced ») ; à faire — les tests de l'inventaire pour le chemin de lancement (T-3…T-6, filiation, S-8, S-9 ; **l'écriture de ce fichier de tests a été arrêtée deux fois par un filtre de sécurité automatique** : ne pas retenter sous une autre forme, à décider avec l'utilisatrice, par exemple l'écrire elle-même ou dans une autre session), la limite mémoire S-11, les témoins hostiles, les mutants MS1–MS19, les tests de proxy de `internal/typst` probablement cassés (le niveau N coupe désormais le réseau) ; trois tests de cgroup échouent aussi sur `main` sur cette machine. **État au 2026-11-15, 01:10** : le paquet compile, les tests existants passent sur le nouveau chemin de lancement (un vrai défaut corrigé : un refus du cgroup au démarrage devenait une sortie vide), `lint` et `fe:check` verts (1 325 tests) ; les 3 échecs connus de cette machine plus `TestCgroupSessionSharesSliceBound`, instable une fois sur deux aussi sur `main` ; reste un commentaire devenu faux dans `internal/typst/engine.go` (`proxyOnly`).
2b. **Audit du protocole fait** (`docs/reviews/2026-11-14-protocol-audit.md`, 53 constats, 6 critiques, 9 majeurs, rangés par bloc en fin de rapport) : décidé le 2026-11-15 : celui qu'un inconnu peut déclencher (validations sans limite pendant une fenêtre d'invitation) passe avant R1 dans IV1 ; les cinq autres demandent un pair admis et suivent R1 (étapes 5, 6 et 8 de l'ADR 0034, AD1), nommés comme limites connues dans les notes de R1 ; chacun rejoué par un test rouge d'abord dans son bloc.
3. **PC1** : modèle formel commité (`5c6fc98`, 12 mutants attrapés) ; plan découpé le 2026-11-15 en partie commune (`docs/plans/2026-11-14-pc1-project-change-by-restart.md`), **PC1a** reprise (`docs/plans/2026-11-15-pc1a-take-over.md`, `gates/PC1a.toml`) puis **PC1b** invitation acceptée (`docs/plans/2026-11-15-pc1b-join.md`, `gates/PC1b.toml`), portes en brouillon non versionnées (des dérogations du garde renvoient au plan commun, que seuls ces brouillons listent : sur une copie neuve `pregate` échoue jusqu'à leur gel, comme pour SB2a1) ; reste la relecture de Codex (sept points dans `gates/PC1/REVIEW.md`), le gel, le code. Puis EX0. La correction de l'outillage (captures) est faite (`b47011c`).

**Reprise (passation du 2026-11-13, 23:45) — à lire en premier par la session suivante, dans cet ordre :**
1. ✅ Fait le 2026-11-14 : correctif `@lezer/common` intégré (`69216d4`), `main` poussé (CI locale verte).
2. ✅ Fait le 2026-11-14 : OI sur `main` (`e3bc987`), validé par l'utilisatrice.
3. ✅ Fait le 2026-11-14 : `AGENTS.md`, `GEMINI.md`, `docs/executors.md`, « When Claude is absent » ; les quatre changements de consignes acceptés par l'utilisatrice ; mentions de gortex retirées du dépôt. Reste, hors dépôt : 8 lignes de `~/.codex/config.toml`.
4. W58 (bloc 11, bloquant R1 ; révision 6 du 2026-11-14, relue deux fois : voir chaque commande rouge, geler la porte, puis le code), puis PR1 (bloc 9 : mesure `timers`, parcours, relecture, commit), puis `pregate` (contrôles rapides avant toute porte : `lint`, `lint:env`, `lint:claude-md`, `fe:check` — ils ont fait échouer 4 pushes et 2 portes le 2026-11-13), puis RP1 et HB1a/HB1b ([conception](docs/plans/2026-11-13-report-and-header-design.md)), puis le bloc « navigation dans le projet » (W59–W62, après G-0b), puis la porte L1 différée.
5. Revues à exploiter : `docs/reviews/2026-11-13-velocity-review.md`, `…-ux-ui-review.md`, `…-ai-handover-audit.md`, `…-skills-hooks-claude-md-review.md` (si présente). Fable n'est plus disponible après le 2026-11-13 : routage normal de `CLAUDE.md`.
**gortex retiré le 2026-11-13 à la demande de l'utilisatrice** (hooks, serveur MCP et consignes globales ôtés, démon arrêté ; sauvegardes `~/.claude/backup-gortex-2026-11-13/`) ; skills `gortex-*`, dossier `.gortex/`, serveur MCP de Codex et outils des agents retirés aussi ; reste à retirer ses mentions dans `CLAUDE.md` (« code via gortex », piège des worktrees), `scripts/opencode-free.sh`, `scripts/claude-or.sh`, `scripts/check-env-reference.py`, les 8 mentions restantes de `~/.codex/config.toml` et le brouillon `AGENTS.md` ; voir aussi `docs/reviews/2026-11-13-skills-hooks-claude-md-review.md` (garde-fous à corriger : `gate-freeze.sh` inactif dans les worktrees, `heavy-guard.sh` trop permissif, `pre-commit-simplify.sh` sur le mauvais dépôt, skills périmés, `CLAUDE.md` réductible d'environ 39 %), puis `lint:claude-md`. Constat qui a motivé le retrait : gortex obligatoire et bloquant a coûté plus qu'il n'a aidé le 2026-11-13 (Codex bloqué par l'approbation de son outil d'édition, deux écritures d'agents dans le dépôt principal au lieu de leur worktree, recherches refusées, diagnostic « CRITICAL » répété à chaque tour ; démon à 14 Go le 2026-11-12) ; proposition : retirer le blocage, couper le diagnostic de fin de tour, interdire l'édition par gortex aux agents, garder la navigation en lecture en option et mesurer une semaine avec et sans. Règles qui ont mordu ce soir : chaque séance avec l'utilisatrice se fait pas à pas, une étape par message ; `app:duo` isole maintenant les documents ; les agents n'écrivent jamais par gortex hors de leur worktree (deux incidents) ; vérifier toute affirmation d'agent avant de la répéter.

**Envoi sur GitHub** : `main` poussé le 2026-11-14 (09:20).

**Contraintes à reprendre dans les spécifications des prochains blocs** (vérification de toutes les demandes du 2026-11-14, [détail](docs/plans/2026-11-14-anticipate-requirements.md)) : SB2a1 — polices (N43), projet protégé (N37), listes par programme pour l'hôte des greffons (N21, N32) ; AD1 — quota et plafond dimensionnés pour un document par fichier, sortie d'un document de l'ensemble admis (W59, G-0b), auteur = clé de membre, IA comptée à l'appareil (N66) ; PR1 — schéma public et extensible de la présence (N14, V9, T3) ; « Relire » — un seul enregistrement de version, signé et à l'échelle du projet, avec l'état de validation (W71, N39, N48) ; PC1 — redémarrage modélisé pour toutes les plates-formes (N35) ; EX0 — il est aussi le registre des types de fichiers (U26, ADR 0027) ; G-0b — stockage derrière une interface (N22).

**Défauts relevés hors bloc, à traiter avant R1** (parcours OI du 2026-11-13, `gates/OI/UX.md`) : la barre d'en-tête se
chevauche à 360 px de large (repris par SI1c) ; un `.md` ouvert depuis le gestionnaire de fichiers s'affiche en lecture seule (« Dans cette
version… ») : limite assumée de G-0a jusqu'à G-0b, gardée pour R1 et annoncée (décision du 2026-11-13, [étude](docs/plans/2026-11-13-md-open-readonly.md)).

**Acceptation de R1** (en plus des portes) : installation et premier lancement sur machine propre ; créer, inviter,
éditer, exporter ; tuer et relancer les deux pairs ; éditions hors ligne puis convergence ; disque plein ;
restauration d'une sauvegarde à côté de l'original ; la seconde personne reçoit images, polices et paquets ;
cas du délimiteur non fermé mesuré. R1 est annoncée « alpha entre personnes de confiance » jusqu'à l'étape 8 de
l'ADR 0034, et peut demander un relais joignable derrière un NAT restrictif.

**En parallèle (outillage, ≤ 20 %) :** **à corriger d'abord** : (1) ✅ corrigé le 2026-11-15 : la porte garde `gates/<lot>/ux/` (les captures du parcours d'interface), tests rouges d'abord ; les 58 captures de W58a restent perdues ; (2) sur une copie neuve du dépôt, `pregate` échoue tant que `gates/SB2a1.toml` n'est pas versionné (des dérogations du garde renvoient au plan SB2) — se règle au gel de SB2a1 ; DL — une seule place par information ([plan](docs/plans/2026-11-13-docs-layout.md), livré le 2026-11-13) ;
audit hebdomadaire `python3 scripts/gate.py --deferred` ; refaire `TestLinkRTTFromProbe` sur un temps contrôlé
(règle 16 de la méthode v2) ; même règle pour `scripts/gate-selftest.py`, dont le cas `heavy.sh --exclusive true` échoue dès qu'une autre tâche lourde tourne (rejoué sur `main` propre le 2026-11-13) ; échec inexpliqué à surveiller : « LF0 Markdown fixed parser equals the original on random edits » a échoué une fois le 2026-11-13 (message non capturé) puis passé deux fois, seul et avec le test de performance (graines fixes, 590 s) — à la prochaine occurrence, garder le journal et trouver la cause ; file de revues Codex ([docs/reviews/codex-coverage.md](docs/reviews/codex-coverage.md), prochaine : frontière
d'admission CRDT).

**Après R1, dans l'ordre :** admission par l'étape 8 de l'ADR 0034 (étapes 4 à 6 d'abord, priorité à la frappe) ;
TR1 (constats de confiance) ; G-0b, G-0c ; le reste de W46 ou LF0b selon ce que le parcours montre ; reportés :
ST1, fenêtre légère OI et menus OS au-delà de `%F`, import Excel, RG.
Audits prévus le 2026-11-09 et toujours à faire : PA (protocole existant aux dix points de
`docs/protocol/README.md`), AA (reste de l'application au regard de `docs/engineering-principles.md` § 0).
Outillage en attente, seulement sur gain démontré (règle 12) : R8+R4, R1, R7 de
[l'étude CI](docs/plans/2026-11-08-ci-cd-study.md).

**Livré récemment :** 2026-11-12 PB, Go 1.27.4, C5S, RC1, LF0a3, GR (`--resume`), plus de portes imbriquées,
méthode v2 ([docs/plans/2026-11-12-method-v2.md](docs/plans/2026-11-12-method-v2.md)) ; 2026-11-09 SP1b, CI1b ; 2026-11-08 L3, P2, P3, RL1.
Historique complet : `git log` et [docs/archive/progress-journal.md](docs/archive/progress-journal.md).

## Étape 0 — Fondations ✅

- [x] Bootstrap go-starter (mise, hooks git, agents, skills, CI) — `mise run ci` vert
- [x] Architecture cible et règles de dépendance — [docs/architecture.md](docs/architecture.md)
- [x] Décisions consignées — [docs/adr/](docs/adr/)
- [x] Glossaire, principes UX, principes d'ingénierie (règle), questions ouvertes
- [x] gopls MCP (`.mcp.json`), skill projet `inkwell-stack`, Dependabot

## Étape 1 — Noyau ✅

> Sortie : **deux personnes tapent dans le même fichier**, curseurs visibles.

- [x] Squelette Wails v3 (beta.34, GTK4) + Svelte 5 / Vite / TS, Taskfile natif, piloté par mise
- [x] `internal/transport` : interface `Transport` + `memnet` (fautes à graine, déterministe sous `synctest`) + `tcp` (supprimé en étape 2 ; bornes mémoire/connexions, audit sécurité appliqué)
- [x] `internal/bridge` : Streams Wails, aucun port local (ADR 0009) ; enveloppe binaire à vecteurs dorés partagés Go/TS, fuzzée
- [x] Frontend : CodeMirror 6 + `automerge-repo` + `WailsNetworkAdapter` + `automerge-codemirror`
- [x] Curseurs et sélections distants nommés (présence automerge-repo, curseurs Automerge)
- [x] Undo local par opérations inverses validé (test unitaire + e2e)
- [x] Simulation de convergence (ADR 0011) : 3 dépôts réels, coupures de lien à graine, 20 graines
- [x] CI : GTK4, `fe:check`, build Linux, e2e Playwright/WebKit (deux instances en mode serveur)
- [x] Démonstration locale à deux instances (`mise run e2e` automatisé, `mise run app:duo` à la main) —
  le projet se teste sur une seule machine ; « deux réseaux » à l'étape 2 sera simulé localement

## Étape 2 — Réseau

Plan : [docs/plans/2026-10-25-etape-2-reseau.md](docs/plans/2026-10-25-etape-2-reseau.md) (12 tâches ; TCP supprimé dès la tâche 4 ; critère vérifié sur un seul PC).

> Sortie : **hors ligne puis retour, sur deux réseaux différents**, sans perte.

- [x] `transport/p2p` : go-libp2p (QUIC, Noise, mDNS, hole punching, relais circuit v2) — `internal/transport/p2p`, `sim_test.go`, suite de contrat `transporttest`
- [x] `identity` : Ed25519 par machine, empreinte en mots, appareils — `internal/identity`, `protocol/sig` (N-1, P2)
- [x] `invite` : code Wormhole + salon LAN — `internal/invite`, `protocol/invite`, ADR 0032
- [x] `trust` : liste de membres signée, rôles, révocation, suspension, lots signés — `protocol/trust`, `internal/trust`, ADR 0017/0018/0021/0030
- [x] `store` : persistance Automerge atomique, sauvegardes horodatées, contrôle d'intégrité
- [x] `project` : index, projection `.typ` sur disque, règle de propriété fsnotify, bac à sable E/S
- [x] Relais libp2p auto-hébergeable en une commande — `cmd/inkwell-relay`, `docs/relay.md`
- [ ] Panneau d'état : qualité du lien par pair mesurée (C5S) ; reste son affichage dans le popover des personnes
- [x] Exit criterion demonstrated and archived (2026-11-08): `mise run e2e:netsim`, 4 scenarios PASS, including 3 (offline 60 s, converged 9 s after coming back) and 4 (network change nat-a → nat-c, converged 25 s after) — [docs/perf/2026-11-08-netsim.md](docs/perf/2026-11-08-netsim.md).

## Étape 3 — Typst

> Sortie : **le premier document utile de bout en bout** — et ce document de conception
> réécrit à deux dans l'app.

- [x] `sidecar` : tinymist et typst embarqués, SHA-256 vérifié, supervision, backoff — [plan](docs/plans/2026-10-25-etape-3-typst.md), [ADR 0024](docs/adr/0024-sidecars-typst-tinymist.md)
- [x] Aperçu (C2) : pages SVG des seules pages visibles, par un Stream sans port ; « Page X sur Y », problèmes dans la barre d'état, saut source ↔ aperçu par paragraphe (`typst eval`), export PDF — [plan](docs/plans/2026-10-25-etape-3-typst.md). Recompilation complète par `typst` (pas d'aperçu incrémental par `tinymist` : reste avec le LSP)
- [x] Lot L3 — LSP via le pont : problèmes cliquables et ancrés, F8 / Maj+F8 (N28) ; complétion, aide sur les arguments et survol, hors ligne (N29). Critère de sortie : `mise run gate -- L3` vert avec la mesure native de la frappe (`INKWELL_NATIVE_PERF_RUN=1`) : problèmes e2e, positions, réponses périmées rejetées, service hors ligne, PERF-LSP p95 ≤ 100 ms, PERF-TYPE natif (chaud p95 ≤ 8 ms, froid p95 ≤ 50 ms), régression L2. Preuves : rapport perdu le 2026-11-08 (non versionné, worktree supprimé); reconstitution : `gates/L3/RECONSTRUCTED.md` ; résultat de la porte : `gates/L3.json`, [plan](docs/plans/2026-11-05-L3-implementation.md)
- [ ] Formatage par le LSP (hors L3)
- [x] Export PDF déterministe, polices embarquées (`typst compile`, `pdf.attach` vérifié dans le PDF)
- [x] `typst` : vente des paquets `@preview` dans `.typstdeps/`, message clair si absent (paquets fournis : cmarker, touying, lilaq)
- [ ] Édition niveau 1 : barre contextuelle, titres, listes, paires, plan, blocs de code, orthographe webview
- [ ] Explorateur de fichiers (états par fichier, fichier principal épinglé, corbeille)
- [ ] Mode présentation plein écran + navigation

## Étape 4 — Collaboration

- [ ] Commentaires ancrés (curseurs Automerge), mentions
- [ ] Panneau Activité (par auteur, par séance), versions nommées (jalons go-git)
- [ ] Diff « depuis ma dernière visite » (`@codemirror/merge`), minibar colorée par auteur
- [ ] Undo par auteur, lecture seule, divergences signalées et réglables en deux clics
- [ ] `blob` : magasin BLAKE3, quotas, échange libp2p (avancé en T11/B4, côté Go fait : magasin + Bao, flux `bulk`, quotas, reprise, ménage, arbre et noms validés ; reste : index et UI côté webview, ADR 0023)
- [ ] Symboles, tableaux, images, encadrés, extraits, panneau Charte ; visionneuse PDF.js

## Étape 5 — Qualité

- [ ] Simulation à graine en CI (200 graines) + scénarios nommés
- [ ] Rejeu d'historique réel, fuzz de tous les décodeurs
- [ ] E2E : deux instances Wails + Playwright
- [ ] Journal circulaire des 1 000 derniers messages réseau
- [ ] Promesse de confidentialité rédigée

## Étape 6 — Produit

- [ ] Style Adwaita complet, i18n fr/en, accessibilité, raccourcis configurables
- [ ] Markdown, export HTML, archive de projet autonome
- [ ] Builds natifs Linux/macOS/Windows en matrice GitHub Actions, signature + notarisation
- [ ] Mises à jour signées (`minio/selfupdate`), rapport de bug en un clic
