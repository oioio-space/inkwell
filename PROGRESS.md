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
| 2 | C5S — qualité du lien | ✅ `main` (`d051af2`) ; reste l'affichage dans le popover des personnes | ADR 0036, `gates/C5S.toml` |
| 3 | OI — instance unique | 🔨 worktree : Codex accepte, parcours d'interface fait (UX-OI-1 à 3 corrigés) ; reste la séance avec l'utilisatrice, puis commit | `gates/OI.toml`, [plan](docs/plans/2026-11-11-oi-single-instance-open.md) |
| 4 | RC1 — reconnexion après redémarrage | ✅ `main` | `gates/RC1.toml` |
| 5 | SI1b — états vrais enregistré / envoi / reçu | 🔨 worktree : critères verts sur `main`, parcours « à livrer » ; reste la séance avec l'utilisatrice, puis commit | `gates/SI1b.toml`, `gates/SI1b/UX.md` |
| 5b | SI1c — en-tête stable et lisible : une seule formulation tant que deux personnes tapent (amende DS-3), mise en page de l'en-tête de 360 à 700 px (absorbe le chevauchement relevé par OI), « Seul » remplacé quand mon texte n'est pas parti | 📝 plan et porte prêts, relus à l'aveugle (2 tours, sources vérifiées) ; à figer après SI1b | [plan](docs/plans/2026-11-13-si1c-header.md), `gates/SI1c.toml` |
| 6 | LF0a3 — coloration incrémentale | ✅ `main` (`427d74a`) ; résidu LF0b après R1 | `gates/LF0a3.toml` |
| 7 | SB1b — bac à sable : mémoire et descendants | ⏸ première moitié écrite (non commitée, worktree), refusée par Codex (2 défauts graves) ; arrêtée par les garde-fous d'Opus (`[cyber]`), en attente d'un exécutant | `gates/SB1b.toml`, ADR 0042 |
| 8 | DU — mises à jour ciblées | ⏳ à refaire depuis `main` : l'ancien worktree (base du 2026-11-09, antérieure à Go 1.27.4) est périmé, son diff archivé (`~/inkwell-archive/2026-11-13-DU-stale-diff.patch`) ; périmètre mesuré le 2026-11-13 sur `main` : govulncheck ne trouve aucune vulnérabilité appelée (GO-2026-5932, `x/crypto/openpgp`, n'est pas appelée) ; `pnpm audit --prod` signale `uuid` < 11.1.1 (GHSA-w5hq-g745-h8pq, modérée) via automerge-repo, qui n'appelle que `v4` (non concerné) → DU pour R1 = une surcharge `uuid` ≥ 11.1.1 et rien d'autre | — |
| 9 | PR1 — présence correcte (W46 : présence figée, couleurs en double, départ en 45 s) | 📝 plan et porte prêts, relus à l'aveugle (2 tours) ; à figer, puis code | [plan](docs/plans/2026-11-13-pr1-presence.md), `gates/PR1.toml` |
| 10 | Paquet deb/rpm + parcours à deux personnes | 🔨 premier essai fait le 2026-11-13 sur `main` d3d0829 : `pkg:build`, `pkg:lint` et `pkg:test` (PKG-01…11) verts sur Debian 13, Ubuntu 24.04, Fedora 44 ; reste le parcours à deux personnes avec le paquet | `docs/releasing.md` |

**Défauts relevés hors bloc, à traiter avant R1** (parcours OI du 2026-11-13, `gates/OI/UX.md`) : la barre d'en-tête se
chevauche à 360 px de large (repris par SI1c) ; un `.md` ouvert depuis le gestionnaire de fichiers s'affiche en lecture seule (« Dans cette
version… ») — à confirmer comme limite assumée ou à corriger, au parcours empaqueté (bloc 10).

**Acceptation de R1** (en plus des portes) : installation et premier lancement sur machine propre ; créer, inviter,
éditer, exporter ; tuer et relancer les deux pairs ; éditions hors ligne puis convergence ; disque plein ;
restauration d'une sauvegarde à côté de l'original ; la seconde personne reçoit images, polices et paquets ;
cas du délimiteur non fermé mesuré. R1 est annoncée « alpha entre personnes de confiance » jusqu'à l'étape 8 de
l'ADR 0034, et peut demander un relais joignable derrière un NAT restrictif.

**En parallèle (outillage, ≤ 20 %) :** DL — une seule place par information ([plan](docs/plans/2026-11-13-docs-layout.md), livré le 2026-11-13) ;
audit hebdomadaire `python3 scripts/gate.py --deferred` ; refaire `TestLinkRTTFromProbe` sur un temps contrôlé
(règle 16 de la méthode v2) ; même règle pour `scripts/gate-selftest.py`, dont le cas `heavy.sh --exclusive true` échoue dès qu'une autre tâche lourde tourne (rejoué sur `main` propre le 2026-11-13) ; file de revues Codex ([docs/reviews/codex-coverage.md](docs/reviews/codex-coverage.md), prochaine : frontière
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
