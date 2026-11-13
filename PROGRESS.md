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
| 3 | OI — instance unique | 🔨 worktree `.claude/worktrees/agent-a78adb31ac7116405` (non commité) : séance avec l'utilisatrice faite le 2026-11-13 avec le paquet installé — « Ouvrir avec » et transmission du fichier OK, **défaut grave** : la fenêtre n'est jamais ramenée au premier plan (« Inkwell est prêt », sablier ; jeton d'activation non transmis) ; correction en cours, puis vérification de 2 min avec le paquet, puis commit | `gates/OI.toml`, [plan](docs/plans/2026-11-11-oi-single-instance-open.md), `gates/OI/UX.md` |
| 4 | RC1 — reconnexion après redémarrage | ✅ `main` | `gates/RC1.toml` |
| 5 | SI1b — états vrais enregistré / envoi / reçu | ✅ `main` (`0752872`) ; validé par l'utilisatrice en séance le 2026-11-13 (`gates/SI1b/UX.md`) | `gates/SI1b.toml` |
| 5b | SI1c → **HB1a / HB1b** — nouvelle barre du haut B (style GNOME, pastille centrale, aperçu à la taille de la zone d'écriture, version étroite) ; reprend les règles de SI1c ; `gates/SI1c.toml` jamais figé, remplacé | [conception](docs/plans/2026-11-13-report-and-header-design.md) |
| 6 | LF0a3 — coloration incrémentale | ✅ `main` (`427d74a`) ; résidu LF0b après R1 | `gates/LF0a3.toml` |
| 7 | SB1b — bac à sable : mémoire et descendants | ✅ `main` (`ca33ae3`) ; porte L1 imbriquée différée (`gates/deferred.jsonl`), à lancer à part | `gates/SB1b.toml`, ADR 0042 |
| 8 | DU — mises à jour ciblées | ✅ `main` : `uuid` 9.0.1 → 14.0.3 par surcharge `^14.0.3` (GHSA-w5hq-g745-h8pq) ; Go : rien d'appelé (govulncheck) | `gates/DU.toml`, `gates/DU.json` |
| 9 | PR1 — présence correcte (W46 : présence figée, couleurs en double, départ en 45 s) | 🔨 porte figée (`1975027`) ; code dans le worktree `.claude/worktrees/agent-a8b133d4e69426085` (non commité), 6 critères sur 9 verts ; reste la mesure `timers` dans la vraie application, le parcours d'interface, la relecture, le commit | [plan](docs/plans/2026-11-13-pr1-presence.md), `gates/PR1.toml` |
| 10 | Paquet deb/rpm + parcours à deux personnes | 🔨 premier essai fait le 2026-11-13 sur `main` d3d0829 : `pkg:build`, `pkg:lint` et `pkg:test` (PKG-01…11) verts sur Debian 13, Ubuntu 24.04, Fedora 44 ; reste le parcours à deux personnes avec le paquet | `docs/releasing.md` |
| 11 | W58 — invitation refusée sur un projet « étranger » (bloquant R1) | 📝 diagnostiqué : un projet chargé dont l'appareil n'est pas membre ; `Start` ne le vérifie pas, « Inviter » est proposé, erreur brute affichée | [diagnostic](docs/plans/2026-11-13-invite-not-admin.md) |
| 12 | RP1 — outil de signalement (W51) | 📝 conception validée avec l'utilisatrice ; à planifier | [conception](docs/plans/2026-11-13-report-and-header-design.md) |

**Reprise (passation du 2026-11-13, 23:45) — à lire en premier par la session suivante, dans cet ordre :**
1. Débloquer le push : `main` a ~10 commits non poussés ; le pre-push échoue sur `frontend/src/editor/highlight.equivalence.test.ts` (« LF0 Markdown fixed parser equals the original on random edits ») ; un diagnostic Fable tournait dans un worktree `.claude/worktrees/agent-*` (chercher le plus récent avec `git worktree list`) : rejouer le contre-exemple, cause racine, jamais affaiblir le test ; puis `scripts/qpush.sh`.
2. OI (bloc 3) : corriger « la fenêtre n'est jamais ramenée au premier plan » (jeton `XDG_ACTIVATION_TOKEN` du second lancement à transmettre à l'instance ouverte, `gtk_window_set_startup_id` + présentation ; recherche GNOME et Firefox/Electron/Tauri/Wails à citer dans le plan OI) dans `.claude/worktrees/agent-a78adb31ac7116405` ; puis vérification de 2 min avec le paquet (`pkexec dnf install dist/*.rpm`, ouvrir depuis Fichiers, `pkexec dnf remove inkwell`) avec l'utilisatrice, pas à pas (oui / non / bof) ; puis commit.
3. Relève des IA : `AGENTS.md`, `GEMINI.md`, `docs/executors.md` et les retouches de `CLAUDE.md` / méthode v2 attendent dans `.claude/worktrees/agent-a394ef8201c46fa82` ; **quatre changements de consignes attendent l'accord de l'utilisatrice** (Codex relit sans implémenter sauf si Claude est absent ; `taskset` cœurs P retiré ; branche `next` retirée ; un worktree par bloc) — les lui présenter en une question, puis relire `CLAUDE.md` en entier et intégrer.
4. W58 (bloc 11, bloquant R1), puis PR1 (bloc 9 : mesure `timers`, parcours, relecture, commit), puis `pregate` (contrôles rapides avant toute porte : `lint`, `lint:env`, `lint:claude-md`, `fe:check` — ils ont fait échouer 4 pushes et 2 portes le 2026-11-13), puis RP1 et HB1a/HB1b ([conception](docs/plans/2026-11-13-report-and-header-design.md)), puis le bloc « navigation dans le projet » (W59–W62, après G-0b), puis la porte L1 différée.
5. Revues à exploiter : `docs/reviews/2026-11-13-velocity-review.md`, `…-ux-ui-review.md`, `…-ai-handover-audit.md`, `…-skills-hooks-claude-md-review.md` (si présente). Fable n'est plus disponible après le 2026-11-13 : routage normal de `CLAUDE.md`.
**gortex retiré le 2026-11-13 à la demande de l'utilisatrice** (hooks, serveur MCP et consignes globales ôtés, démon arrêté ; sauvegardes `~/.claude/backup-gortex-2026-11-13/`) ; reste à retirer ses mentions dans le dépôt (`CLAUDE.md` : « code via gortex », piège des worktrees ; listes d'outils de `.claude/agents/*.md` ; permissions de `.claude/settings.json` ; brouillon `AGENTS.md`), puis `lint:claude-md`. Constat qui a motivé le retrait : gortex obligatoire et bloquant a coûté plus qu'il n'a aidé le 2026-11-13 (Codex bloqué par l'approbation de son outil d'édition, deux écritures d'agents dans le dépôt principal au lieu de leur worktree, recherches refusées, diagnostic « CRITICAL » répété à chaque tour ; démon à 14 Go le 2026-11-12) ; proposition : retirer le blocage, couper le diagnostic de fin de tour, interdire l'édition par gortex aux agents, garder la navigation en lecture en option et mesurer une semaine avec et sans. Règles qui ont mordu ce soir : chaque séance avec l'utilisatrice se fait pas à pas, une étape par message ; `app:duo` isole maintenant les documents ; les agents n'écrivent jamais par gortex hors de leur worktree (deux incidents) ; vérifier toute affirmation d'agent avant de la répéter.

**Envoi sur GitHub** : `main` a des commits non poussés tant que le pre-push échoue (2026-11-13 : test de propriété `highlight.equivalence` LF0 Markdown, en diagnostic).

**Défauts relevés hors bloc, à traiter avant R1** (parcours OI du 2026-11-13, `gates/OI/UX.md`) : la barre d'en-tête se
chevauche à 360 px de large (repris par SI1c) ; un `.md` ouvert depuis le gestionnaire de fichiers s'affiche en lecture seule (« Dans cette
version… ») : limite assumée de G-0a jusqu'à G-0b, gardée pour R1 et annoncée (décision du 2026-11-13, [étude](docs/plans/2026-11-13-md-open-readonly.md)).

**Acceptation de R1** (en plus des portes) : installation et premier lancement sur machine propre ; créer, inviter,
éditer, exporter ; tuer et relancer les deux pairs ; éditions hors ligne puis convergence ; disque plein ;
restauration d'une sauvegarde à côté de l'original ; la seconde personne reçoit images, polices et paquets ;
cas du délimiteur non fermé mesuré. R1 est annoncée « alpha entre personnes de confiance » jusqu'à l'étape 8 de
l'ADR 0034, et peut demander un relais joignable derrière un NAT restrictif.

**En parallèle (outillage, ≤ 20 %) :** DL — une seule place par information ([plan](docs/plans/2026-11-13-docs-layout.md), livré le 2026-11-13) ;
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
