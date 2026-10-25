# inkwell — état & feuille de route

Finalité : [docs/conception.pdf](docs/conception.pdf). **Premier jalon mesurable : à la fin
de l'étape 3, écrire le document de conception à deux dans l'application.**
Chaque étape livre quelque chose qui tourne ; l'ordre met les risques techniques en premier
et l'habillage en dernier. Une étape n'est close que lorsque son **critère de sortie** est
démontré (test, démo enregistrée ou tâche utilisateur chronométrée).

## État actuel

- **Étape 0 — Fondations** : terminée. Squelette go-starter, gates verts, architecture, ADR,
  feuille de route, outillage IA.
- **Prochaine : étape 1 — Noyau.** Plan à écrire (`docs/plans/`) avant tout code.

## Étape 0 — Fondations ✅

- [x] Bootstrap go-starter (mise, hooks git, agents, skills, CI) — `mise run ci` vert
- [x] Architecture cible et règles de dépendance — [docs/architecture.md](docs/architecture.md)
- [x] Décisions consignées — [docs/adr/](docs/adr/)
- [x] Glossaire, principes UX, questions ouvertes
- [x] gopls MCP (`.mcp.json`), skill projet `inkwell-stack`, Dependabot

## Étape 1 — Noyau

> Sortie : **deux personnes tapent dans le même fichier**, curseurs visibles.

- [ ] Squelette Wails v3 épinglé (`build/`, `frontend/` Vite + TS, Taskfile) piloté par mise
- [ ] `internal/transport` : interface `Transport` + `memnet` (fautes injectées, horloge virtuelle) + `tcp`
- [ ] `internal/bridge` : octets opaques webview ↔ Go ; **mesure du débit** bindings vs WS local à jeton → ADR
- [ ] Frontend : CodeMirror 6 + `automerge-repo` + `NetworkAdapter` vers le pont + `automerge-codemirror`
- [ ] Curseurs et sélections distants (messages éphémères)
- [ ] Undo local par opérations inverses (zone d'ombre n° 1) validé
- [ ] Amorce `internal/sim` : 2 pairs, oracle de convergence, graine rejouable
- [ ] CI : job frontend (lint, typecheck, tests) + build Wails Linux

## Étape 2 — Réseau

> Sortie : **hors ligne puis retour, sur deux réseaux différents**, sans perte.

- [ ] `transport/p2p` : go-libp2p (QUIC, Noise, mDNS, hole punching, relais circuit v2)
- [ ] `identity` : Ed25519 par machine, empreinte en 4 mots, groupes d'appareils
- [ ] `invite` : code Wormhole + acceptation LAN (un inconnu ne voit que nom de machine et titre)
- [ ] `trust` : liste de membres signée, rôles, révocation, lots signés
- [ ] `store` : persistance Automerge atomique, sauvegardes horodatées, contrôle d'intégrité
- [ ] `project` : index, projection `.typ` sur disque, règle de propriété fsnotify, bac à sable E/S
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
