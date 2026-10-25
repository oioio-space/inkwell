# inkwell — état & feuille de route

Finalité : [docs/conception.pdf](docs/conception.pdf). **Premier jalon mesurable : à la fin
de l'étape 3, écrire le document de conception à deux dans l'application.**
Chaque étape livre quelque chose qui tourne ; l'ordre met les risques techniques en premier
et l'habillage en dernier. Une étape n'est close que lorsque son **critère de sortie** est
démontré (test, démo enregistrée ou tâche utilisateur chronométrée).

## État actuel

- **Étape 0 — Fondations** : terminée. Squelette go-starter, gates verts, architecture, ADR,
  feuille de route, outillage IA.
- **Étape 1 — Noyau** : terminée — critère de sortie démontré en local par un test de bout
  en bout (deux instances réelles, Playwright) et `mise run app:duo` (deux fenêtres).
  Plan : [docs/plans/2026-10-25-etape-1-noyau.md](docs/plans/2026-10-25-etape-1-noyau.md).

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

- [ ] `transport/p2p` : go-libp2p (QUIC, Noise, mDNS, hole punching, relais circuit v2)
- [ ] `identity` : Ed25519 par machine, empreinte en 4 mots, groupes d'appareils
- [ ] `invite` : code Wormhole + acceptation LAN (un inconnu ne voit que nom de machine et titre)
- [ ] `trust` : liste de membres signée, rôles, révocation, lots signés
- [x] `store` : persistance Automerge atomique, sauvegardes horodatées, contrôle d'intégrité
- [x] `project` : index, projection `.typ` sur disque, règle de propriété fsnotify, bac à sable E/S
- [ ] Relais libp2p auto-hébergeable en une commande
- [ ] Panneau d'état (indicateur global, liste des pairs, type de lien)

## Étape 3 — Typst

> Sortie : **le premier document utile de bout en bout** — et ce document de conception
> réécrit à deux dans l'app.

- [ ] `sidecar` : tinymist et typst embarqués, SHA-256 vérifié, supervision, backoff
- [ ] Aperçu incrémental (iframe), saut source ↔ aperçu dans les deux sens
- [ ] LSP via le pont : complétion, survol, diagnostics, formatage
- [ ] Export PDF déterministe, polices embarquées
- [ ] `typst` : vente des paquets `@preview` dans `.typstdeps/`, message clair si absent
- [ ] Édition niveau 1 : barre contextuelle, titres, listes, paires, plan, blocs de code, orthographe webview
- [ ] Explorateur de fichiers (états par fichier, fichier principal épinglé, corbeille)
- [ ] Mode présentation plein écran + navigation

## Étape 4 — Collaboration

- [ ] Commentaires ancrés (curseurs Automerge), mentions
- [ ] Panneau Activité (par auteur, par séance), versions nommées (jalons go-git)
- [ ] Diff « depuis ma dernière visite » (`@codemirror/merge`), minibar colorée par auteur
- [ ] Undo par auteur, lecture seule, divergences signalées et réglables en deux clics
- [ ] `blob` : magasin BLAKE3, quotas, échange libp2p
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
| 2026-10-25 | Bootstrap go-starter ; fondations : architecture, 8 ADR, glossaire, UX, roadmap, gopls MCP, Dependabot. |
| 2026-10-25 | Étape 1 : transport (memnet, tcp), pont Streams, éditeur collaboratif Svelte/CodeMirror/Automerge, présence, undo local, e2e deux instances ; ADR 0009–0013. |
