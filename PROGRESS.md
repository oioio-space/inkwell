# inkwell — état & feuille de route

Finalité : [docs/conception.pdf](docs/conception.pdf). **Premier jalon mesurable : à la fin
de l'étape 3, écrire le document de conception à deux dans l'application.**
Chaque étape livre quelque chose qui tourne ; l'ordre met les risques techniques en premier
et l'habillage en dernier. Une étape n'est close que lorsque son **critère de sortie** est
démontré (test, démo enregistrée ou tâche utilisateur chronométrée).

## État actuel

- **Audit d'architecture (2026-10-31)** : référentiel, constats, décisions D1–D14, état des lots et
  marche à suivre pour reprendre : [docs/plans/2026-10-31-architecture-audit.md](docs/plans/2026-10-31-architecture-audit.md).
- **Audit admission — étape 2.0** : pairs du Repo limités aux appareils membres non suspendus ; retrait explicite, annulation des files et suppression des cooldowns (A14). TDD et tests ciblés race, Vitest, svelte-check, lint et architecture verts ; invitations/relay à valider hors sandbox (sockets interdits).
- **Étape 0 — Fondations** : terminée.
- **Étape 1 — Noyau** : terminée — critère de sortie démontré en local (deux instances réelles, Playwright ; `mise run app:duo`).
  Plan : [docs/plans/2026-10-25-etape-1-noyau.md](docs/plans/2026-10-25-etape-1-noyau.md).
- **Étape 2 — Réseau** : code livré (p2p, identité, invitation, confiance, relais, store, projet) ; exit criterion **demonstrated and archived** on 2026-11-08 (`docs/perf/2026-11-08-netsim.md`); remaining: the status panel (link type).
- **Lots HDAE (méthode `docs/plans/2026-11-02-methode-hdae.md`, portes figées `gates/<lot>.toml`)** :
  L0 (mesure de référence, garde anti-affaiblissement), L1 (autoportance : bac à sable des sidecars sans programme externe, N42), L2 (G-0a : un clic dans la barre latérale ouvre le fichier), L3 (LSP : problèmes cliquables, F8, aide Typst, N28/N29) — tous `pass` dans `gates/L{0,1,2,3}.json` ; reports : PERF-MEM → G-0b, native darwin/windows → N35, PERF-START-plugins → Fondations extensibles.
- **Protocole ADR 0034** : étape 1 (module `protocol/`), étape 2 (P2 : `protocol/sig`, Ed25519 strict, N-1) et étape 3 (P3 : TLC épinglé `mise run tla`, profil CBOR et schémas §9 dans `@inkwell/protocol`) faites, portes `gates/P2.json` et `gates/P3.json` vertes ; **prochaine : étape 4** (`protocol/sync2`, `protocol/libp2p`), à spécifier d'abord en exécutable (`docs/plans/2026-11-08-spec-executable.md`). Ensuite : G-0b, G-0c, S13. Suivi : [docs/objectif-en-cours.md](docs/objectif-en-cours.md).

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
- [ ] Panneau d'état : reste l'indicateur global et le type de lien par pair (la barre d'état ne montre que le nombre de personnes)
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

## Journal

| Date | Changement |
|------|------------|
| 2026-11-08 | ADR 0034 étape 2 (P2) : `protocol/sig`, Ed25519 strict, porte `gates/P2.json` verte. |
| 2026-11-08 | ADR 0034 étape 3 (P3) : TLC épinglé, profil CBOR, schémas §9 ; porte `gates/P3.json` verte. |
| 2026-11-08 | Bloc RL1 : règle L de la spec sur sync/1 — une écriture échouée relance la synchronisation (plus de perte silencieuse après une coupure, NI-2 vert sur 3 graines) ; mesures de lien v1 et notification Links jusqu'à la webview ; outillage réseau perturbé (NI0, en cours). Porte `gates/RL1.json` verte, régressions et mesure native comprises. |
| 2026-11-09 | Bloc SP1b (chemin d'envoi provisoire, remplace WB et SP1) : plancher 2 Kio/s, garde sans progrès 90 s, yamux configuré (120 s, fenêtre fixe 256 Kio), partiels 120 s ; un gros collage sur relais lent arrive du premier coup, sans coupure ni resynchronisation (NI-3 C1 strict). Frappe sur relais lent encore lente (NI-3 C2, panne connue jusqu'aux étapes 4 à 6). Correctif go-libp2p (fork épinglé, libp2p/go-libp2p#3575, #3576). Porte `gates/SP1b.json` verte, régressions et mesure native comprises. |
| 2026-11-09 | Bloc CI1b (outillage, W35) : au pré-push, le netsim dégradé ne tourne que si le diff touche ses entrées, calculées depuis `go list -deps` des binaires lancés (`scripts/netsim/affected.py`, tout échec ⇒ il tourne) ; sur GitHub il ne tourne plus qu'une fois (job `netsim`), le job `ci` lance `ci:core` ; `done` inchangé. |
| 2026-11-08 | Triage du registre (exigences 🔨) et mise à jour de PROGRESS. |
| 2026-11-03 | Lots L1, L2 (N42 précisée : rien à installer à part), L-maj (dépendances). |
| 2026-11-02 | Lot L0 : portes HDAE et mesure de référence ; spécifications closes, méthode HDAE. |
| 2026-10-25 | Bootstrap go-starter ; fondations : architecture, 8 ADR, glossaire, UX, roadmap, gopls MCP, Dependabot. |
| 2026-10-25 | Étape 3 (C1, compilation de C2) : sidecars `typst`/`tinymist` épinglés, embarqués et supervisés ; `internal/typst` compile hors ligne (PDF, SVG, diagnostics, `query`, `Watch`). Reste : aperçu incrémental et LSP par `tinymist` (stdio), présentations. |
| 2026-10-26 | Étape 3 (C2) : aperçu, problèmes, saut source ↔ aperçu, « Exporter en PDF » et mode réseau (À propos) ; `internal/app.Preview`, `typst.PageCount`/`Anchors`. Reste : LSP `tinymist` (diagnostics à la frappe, aperçu incrémental), ouverture des autres fichiers depuis un problème (B4 côté webview). |
| 2026-11-08 | Lot L3 : tinymist branché sur l'éditeur (N28, N29). Porte L3 verte, mesure native de la frappe comprise. Le manifeste `gates/L3.json` reste compact : les mesures brutes sont dans `gates/L3-setup/`, citées par chemin et SHA-256. |
| 2026-10-25 | Étape 1 : transport (memnet, tcp), pont Streams, éditeur collaboratif Svelte/CodeMirror/Automerge, présence, undo local, e2e deux instances ; ADR 0009–0013. |
