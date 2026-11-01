# Traçabilité : finalité et exigences → architecture → preuves

| | |
|---|---|
| Version | 1.0.0-brouillon.4 (2026-11-01, révision 3 ; N1 à N41) |
| Lire avec | [spec.md](spec.md) (règles), [decisions.md](decisions.md) (`AD-xx`), [exigences](../exigences.md) |

Chaque ligne relie une demande à l'élément d'architecture qui la sert et à la preuve qui la
vérifie (test, porte, revue). Une ligne sans preuve est un trou : colonne « État ».
États : ✅ prouvé · 🧪 en partie · ⏳ à faire · ❓ décision attendue (`AD-xx`).

## 1. Finalité → objectifs → architecture

| Conception (page) | Objectif | Éléments (spec) | Preuves | État |
|---|---|---|---|---|
| éditer à plusieurs en direct, Typst, Markdown, texte (p. 1) | G1 | §3.5, §6.1, §6.6, Worker (§5.5), `src/net`, session | e2e `collab.spec.ts` ; convergence fast-check (3 `Repo`, 20/200 graines) | 🧪 (un seul fichier, `AD-46`) |
| sans serveur ; LAN ou code court (p. 1, 15, 19) | G2 | §8, `transport/p2p`, `invite`, relais (`AD-43`) | `TestJoinByTicketText` ; `invites_test.go` ; e2e `join.spec.ts` | 🧪 ❓ `AD-43` |
| hors ligne, fusion sans perte (p. 1, 14) | G3 | §6, §10.2 (I1–I8), §6.6 MF-11–12 | `TestKillDuringWrite`, `TestKillAtRandomTimes`, e2e `offline.spec.ts`, `persistence.spec.ts` | 🧪 |
| qui a écrit quoi, historique, versions (p. 6–11) | G4 | §6.1, ADR 0031 (signature par modification), `AD-13` | e2e `history.spec.ts` ; vecteurs `step-8` | 🧪 ❓ `AD-13` |
| aperçu et export PDF hors ligne (p. 5, 13) | G5 | §3.5, `typst`, `sidecar`, §9.5, X-8 | `TestRunNoNetwork`, `TestRunDirectoryBound`, e2e `preview.spec.ts` | 🧪 (`AD-26`) |
| confiance explicite (p. 3–4, 20) | G6 | §9, ledger (`AD-02`), protocole §4, §12 | vecteurs `trust.json`, Chalkias ; P-BYZ ; `suspend_test.go` | 🧪 |
| projet multi-fichiers (p. 5, 12, 31) | G7 | §6.6, ADR 0023, `blob` | `blob` (bancs, fuzz) ; e2e à écrire (`AD-46`–`AD-50`) | ⏳ |
| Adwaita, zéro jargon, accessible (p. 13, 37) | G8 | §2, docs/design, ND-4 | axe e2e ; garde i18n (à venir) ; `ux-walkthrough` | 🧪 |
| fiable et rapide (U32, U33) | G9 | §10, §12, ADR 0028, 0029 | `PERF-*`, `bench:compare`, LazyFS | 🧪 |
| un exécutable, `.deb`, `.rpm` (L1–L5) | G10 | §14, ADR 0024, 0025 | `pkg:test` (PKG-01…12) ; AC-14 | 🧪 (six cibles : N35, §9.11) |
| protocole public (N12–N16) | G11 | §4.3, §8, `docs/protocol/` | P-VEC, P-COMPAT, P-TLA, P-FUZZ | 🧪 (migration) |
| hautement extensible (N20–N26) | G12 | §15, annexe (ADR 0039), §6.6, §6.7, §7.3, §9.10 | P-EXT-1…7, `EXT-ND*`, `CAP-*`, P-PLUG-BENCH (avec témoin), `PLUG-COMPROMISED-*` | ⏳ ❓ `AD-57` |
| projet chiffré, export en un fichier (N37) | G13 | §6.10 | P-VAULT | ⏳ expérimentation |
| historique « à la Git » sans Git, variantes (N39, N41) | G4 | §6.11 | P-GIT-BENCH, e2e Historique | ⏳ |
| six plates-formes (N35) | G10 | §9.11 | P-SANDBOX-* | ⏳ |
| commentaires, lecture seule (p. 11–12) | G4, G6 | `AD-14` | — | ❓ `AD-14` |
| plusieurs projets ouverts (p. 13) | — | N-1, `AD-01` | — | ❓ `AD-01` |
| mises à jour signées, rapport de bug (p. 15–16, 38) | G10 | §11, `AD-16`, `AD-17` | test canari O-7 | ⏳ ❓ |
| archive de projet, visionneuse PDF (p. 6, 37, 39) | G7 | `AD-38`, `AD-39` | — | ⏳ |
| import DOCX/Markdown/LaTeX par pandoc (p. 13, 37) | NG1 | **écarté par N40** : aucun format bureautique natif, aucun pandoc ; greffons possibles | revue | ✅ décidé (`AD-33`, `AD-98`) |
| v2 : aperçu pour un invité, `join --headless`, brouillons (p. 7, 38–39) | NG10 | `AD-34`, `AD-35` | — | hors v1 |

## 2. Exigences de l'utilisatrice → architecture → preuves

| # | Demande (résumé) | Éléments | Preuves | État |
|---|---|---|---|---|
| N-1 | clés canoniques, ordre premier | `protocol/sig`, §9.4 | vecteurs Chalkias, `small_order_test.go` | 🧪 |
| N1 | libp2p | `transport/p2p` (AC-4) | `arch-check.sh` | ✅ |
| N2 | admin absent ne bloque pas | ADR 0017, §10.4 | `trust` (rapid) | ✅ |
| N3 | suspendre, laisser revenir | ADR 0021, ledger | `suspend_test.go`, e2e `people.spec.ts` ; `AD-36` | 🧪 |
| N4 | zones verrouillées | ADR 0022, garde | — | ⏳ |
| N5 | rejoindre = tout recevoir | ADR 0023, §6.6, vrac | e2e JOIN-3 | 🧪 |
| N6 | multi-utilisateur partout | §6.1 (une vérité), ADR 0026 | SHEET-MU-* | 🧪 |
| N7 | modèle byzantin complet | §9.2, protocole §12, §16 | P-BYZ, P-BORNES, fuzz | 🧪 |
| N8–N11 | admission, décisions, quota, membres seulement | ADR 0030, 0031, §6.5, `AD-24` | vecteurs `step-*`, `admission*.test.ts` | 🧪 ❓ `AD-24` |
| N12–N16 | protocole public, module, versions, standards | §4.3, §8, ADR 0034 | P-* d'ADR 0034 ; AC-9, AC-10 | 🧪 |
| N17, N19 | design spécifié et revu | docs/design | `ux-walkthrough`, axe | 🔨 |
| N18 | architecture spécifiée | ce dossier | revue Fable + Codex | 🔨 |
| N20 | types txt, csv, md, typ ; coloration des autres langages | §6.6 MF-14–18, `AD-51`–`AD-55` | vecteurs de reniflage Go/TS ; `PERF-EXT-*` (coloration) ; e2e d'ouverture par type | ⏳ |
| N21 | greffons WASI, gestion, API | §15, annexe, `AD-57`–`AD-64` | P-EXT-*, P-PLUG-BENCH, P-PLUG-PROTO | ⏳ ❓ |
| N22 | stockage remplaçable, thèmes, cycle de vie | §6.7, §15.8, §15.9, `AD-67`, `AD-75`, `AD-76` | fuzz du dorsal et de l'assainisseur ; `PERF-EXT-START`, `PERF-EXT-STOP` ; test de contraste | ⏳ ❓ `AD-67` |
| N23 | greffons de projet | §15.10, `AD-69` | `EXT-PP-*` (PP-1…7) ; vecteurs de décision `plugin/` | ⏳ ❓ |
| N24 | ne pas dénaturer ; aller au plus loin | §15.11, §15.12, §15.13 | `EXT-ND1`…`EXT-ND8` | ⏳ |
| N25 | API claire, débogage | §15.15, `AD-71`, `AD-74` | TTFP chronométré, `api:surface`, `api:doc`, rejeu | ⏳ |
| N26 | protocole des greffons, versions compatibles | §15.16, `AD-72` | vecteurs `docs/plugin-api/`, matrice de compatibilité | ⏳ (spécification à écrire) |
| N27 | modèles créés par les personnes | §6.8, `AD-78` | e2e `templates.spec.ts` (créer, dupliquer, importer, nouveau projet sans réseau) ; test « aucun `typst init` » | ⏳ ❓ `AD-78` |
| N28 | erreurs comme un IDE | §15.17 ER-1–4, `AD-79` | e2e `problems.spec.ts` (F8, clic → fichier et plage) ; table de messages (unitaire) | ⏳ |
| N29 | aide à la saisie Typst (tinymist) | §15.17 LSP-1–5, `AD-80` | propriété UTF-16 ↔ Automerge ; réponses périmées jetées (`LSP-STALE`) ; `PERF-LSP` p95 ≤ 100 ms ; test réseau coupé | ⏳ |
| N30 | outils d'écriture | §15.17, `AD-81` | `ux-walkthrough` ; tests en personne (étude §5 point 6) | ⏳ |
| N31 | IA de premier rang, MCP, serveurs de l'entreprise | §15.18, `AD-82` | propriété IA-5 ; corpus d'injections fuzzé ; fuzz SSE et arguments MCP ; « une proposition acceptée = une modification annulable » ; MCP : `Origin`, jeton, portées | ⏳ ❓ `AD-82`, PD-1 |
| N32 | ressources des greffons, gestionnaire de tâches | §11.1, §9.10 H-7, `AD-88` | test « Arrêter sous saturation » ; boucle infinie native et WebAssembly ; canari O-7 sur la vue | ⏳ |
| N33 | RAG natif | §15.18 RG-1–RG-5, `AD-89` | P-IDX (banc de rappel en français, `PERF-IDX-*`) ; propriété « citation existe à `rev` ou dite périmée » ; injection « extrait `autrui` en tête ⇒ aucun outil hors classe » | ⏳ expérimentation |
| N34 | fichiers non synchronisés | §6.9 IG-1–IG-6, `AD-90` | conformité contre `git check-ignore` ; fuzz du parseur ; « fichier exclu référencé par Typst signalé » | ⏳ (projet : PD-1) |
| N35 | Linux, macOS, Windows, amd64 et arm64 | §9.11, `internal/sandbox`, INV-13, `AD-18`, `AD-94` | P-SANDBOX-LINUX, -MAC, -WIN ; CI native sur six cibles | ⏳ expérimentation (macOS, Windows) |
| N36 | passe de cohérence | [coherence.md](coherence.md) | revue Fable + Codex | 🔨 |
| N37 | projet chiffré, export en un fichier | §6.10 EN-1–EN-8, `AD-91` | P-VAULT ; « export chiffré lisible par `age -d` » ; canari « aucun clair sur disque après fermeture » | ⏳ expérimentation |
| N38 | performance mesurée | §12.5, PF-5–PF-10, `AD-93` | `bench:compare` en CI ; `PERF-*` la nuit sur amd64 et arm64 ; `goleak` ; PGO prouvé | 🔨 |
| N39 | historique sans Git, autoporteur | §6.11 HI-1–HI-5, INV-12, `AD-13`, `AD-92` | P-GIT-BENCH ; test sans `git` dans `PATH` ; e2e « voyager, restaurer, annuler » | ⏳ |
| N40 | aucun format bureautique natif | §1.3 NG1, `AD-33`, `AD-98` | revue : aucune dépendance pandoc ni entrée de menu Word/ODT | ✅ décidé |
| N41 | variantes et intégrations claires | §6.11 HI-6, HI-7, `AD-96` | e2e « créer, comparer, proposer d'intégrer, intégrer, annuler » ; aucun mot de Git (garde du glossaire) | ⏳ |
| G-0 | clic dans la liste des fichiers | §6.6 (G-0a, b, c), `AD-77` | e2e `files.spec.ts` FM-OPEN-1…8 | ⏳ |
| U1 | encodage, fins de ligne | `project` (`FileInfo`), MF-15 | `text_test.go`, `externalSync.test.ts` | ✅ |
| U4–U6, U9 | barres par type, insertion, icônes | points `toolbar`, `menus`, `file-types` (§15.4) | e2e `format.spec.ts`, `shell.spec.ts` | ✅ (à porter sur le registre) |
| U7 | modifications externes | ADR 0020, F-2, MF-12 | `externalSync.test.ts`, EXT-* | 🧪 (main.typ seulement) |
| U8 | enregistrement auto ou manuel | MF-13 | e2e `save.spec.ts` | 🧪 (par fichier : `AD-49`) |
| U10, U28 | présentations, deux fenêtres | Stream `present`, `windows` | e2e `present.spec.ts` | ◐ |
| U11, U12, U27 | bande des modifications, historique, différences | `history-views`, `decorations` | e2e `history.spec.ts` | 🧪 |
| U13 | gestionnaire de fichiers | §6.6, ADR 0023, 0030 | FM-* | ⏳ |
| U14 | recherche dans le projet | `AD-42` | e2e `search.spec.ts` | 🧪 |
| U15–U17 | inclusions, liens, qui édite quoi | §6.6 MF-7, MF-10 | LNK-*, PRS-21…26 | ⏳ |
| U18 | mise en page des Markdown | Typst + `cmarker` (§6.6) | MDL-* | ⏳ |
| U19–U22, U25 | tableur CSV | ADR 0026, `AD-52`, `AD-53` | `sheet.spec.ts`, `collab.test.ts` (CSV-04b) | 🧪 ❓ `AD-52` |
| U26 | types modulaires | ADR 0027, `AD-08`, `AD-51` | P-EXT-1 | ⏳ |
| U29, U30 | tâches, sommaire | panneaux (§15.4) ; Tâches en greffon WebAssembly | contrat de l'API | ⏳ |
| U31 | aucune « verrue » | §15.1, §15.11 (ND-4) | `ux-walkthrough` | règle |
| U32 | fiable | §10, ADR 0029 | I1–I8 | 🧪 |
| U33, U34 | performant, gros fichiers | §12, ADR 0028 | `PERF-*`, `limits.test.ts` | 🧪 |
| L1–L5 | exécutable, paquets, SemVer, icône, publication testée | §14, ADR 0024, 0025 | `pkg:test`, SBOM, cosign, AC-14 | 🧪 |
| W3, W20 | conception moderne ; rien au hasard | engineering-principles, §2 | portes du §13 | règle |
| W7, W18 | un seul PC ; mémoire | `scripts/heavy.sh` | — | ✅ |
| W16 | tests utiles | §13 (« un test prouve une règle nommée ») | mutation (gremlins, Stryker) | 🔨 |

## 3. Invariants → portes

| Invariant | Porte | État |
|---|---|---|
| INV-1 (pas d'Automerge en Go) | AC-8 (`arch-check.sh`) | ✅ |
| INV-2 (réseau par `transport`) | AC-4, AC-5 ; AC-10 | ✅ ; AC-10 ⏳ |
| INV-3 (une vérité) | §6.1, revue ; §15.1 point 3 | revue |
| INV-4 (octets de pair vérifiés, fuzzés) | cibles `Fuzz*` (≈ 30), P-FUZZ | 🧪 |
| INV-5 (aucun port) | AC-14 (`AD-22`) | ⏳ |
| INV-6 (sidecars épinglés) | AC-6, AC-7, tests `SIDE-*` | ✅ |
| INV-7 (cgo, natif) | CI | ✅ (Linux) |
| INV-8 (pair/personne) | glossaire, ND-4, garde i18n | 🧪 |
| INV-9 (un paquet, un rôle) | revue ; AC-11 à AC-13 | ⏳ |
| INV-10 (module `protocol` autonome) | AC-9 ; `AD-31`, `AD-40` | ⏳ |
| INV-11 (noyau fermé, proposé) | AC-16, FE-3, P-EXT-*, `EXT-ND*` | ⏳ |

## 4. Points d'extension → API → capacité → extension intégrée → budget → preuve

Détail des budgets : spec §15.4, §15.13, §15.14.

| Point | API (WIT) | Capacité | Extension intégrée ou d'exemple | Budget | Preuve |
|---|---|---|---|---|---|
| commandes, palette, raccourcis | `commands`, `menus` | — ou selon la commande | toutes les commandes du cœur | palette ≤ 50 ms (1 000 commandes) | contrat + `PERF-EXT-PALETTE` ; `EXT-ND3` |
| menus contextuels, menu de l'application | `menus` (emplacements §15.13) | — | liste des fichiers, éditeur, aperçu | 0 appel pour afficher | `EXT-MENU-*` |
| bouton d'extension (zone dédiée) | `actions` | — | Inspecteur d'extensions (mode développeur) | badge ≤ 1 changement/s | `EXT-ACT-*` ; ND-1 |
| barre d'outils | `toolbar` | — | barres Typst, Markdown, tableur | 0 appel pour afficher | `format.spec.ts` porté |
| panneaux, onglets du tiroir | `panels` | `ui.view` | Historique, Commentaires, Rechercher, Sommaire, Tâches (TypeScript) | 1er rendu ≤ 100 ms | contrat ; P-PLUG-PROTO |
| barre d'état, toasts, bandeaux | `status` | `ui.view` | mots, pages, encodage | ≤ 4 maj/s ; 1 toast/10 s | `EXT-ND5` |
| fenêtres, dialogues | `windows` | `ui.view` | fenêtre public de présentation | ouverture ≤ 300 ms | `EXT-WIN-*` ; ND-2 |
| accueil, préférences | `welcome`, `settings` | — | modèles de projet ; pages Enregistrement, Éditeur | rendu ≤ 50 ms | contrat |
| types de fichiers, vues, éditeurs | `file-types`, `viewers` | `document.*`, `blob.read`, `ui.view` / `ui.native` | éditeur, tableur, images, PDF | ouverture ≤ 200 ms | vecteurs de registre ; e2e par type |
| coloration | `grammars`, `tokens` | `document.read` | Typst, Markdown, liste fermée | ≤ 5 ms (intégrée), ≤ 50 ms (greffon) | `PERF-EXT-HL` |
| complétion, survol, langages | `completion`, `hover`, `language-server` | `document.read` | LSP tinymist | p95 ≤ 100 ms | contrat LSP |
| diagnostics, décorations, gouttières, widgets | `diagnostics`, `decorations`, `gutters`, `inline-widgets` | `document.read` | problèmes Typst, bande des modifications, aperçu en ligne des diagrammes | 1 maj/300 ms ; ≤ 4 ms | `PERF-EXT-DECO` |
| transformations, collage | `transforms` | `document.read`, `document.propose` | tri, collage du tableur, remplacer tout | ≤ 50 ms appliqué | `EXT-ND8` (Annuler) |
| rendu de diagramme | `renderers` | entrée I2 seulement | **Diagrammes** : Graphviz (`wasm`) ; exemple D2 ; Mermaid différé (P-FRAME) | p95 ≤ 300 ms | fuzz assainisseur SVG ; e2e jusqu'au PDF |
| affichage de l'aperçu | `preview-overlays` | `compile.read` | saut source ↔ aperçu | ≤ 16 ms/page | contrat ; X-8 |
| exportateurs, importateurs | `exporters`, `importers` | `compile.run`, `export.write`, `import.read`, `document.create` | PDF, PDF/UA, archive ; CSV | ≤ 120 s, annulable | e2e export |
| thèmes, icônes | `themes`, `icons` | — | Adwaita clair, sombre, contraste élevé | validation ≤ 20 ms ; bascule ≤ 50 ms | test de contraste (TH-2) |
| stockage distant, miroir | `storage-backend`, `mirrors` | `net.hosts` + `storage.backend` ; `mirror.dir` | « Dossier de sauvegarde » | hors chemin d'édition | fuzz ; ST-1…8 ; règle de flux `CAP-RF` |
| cycle de vie, événements | `events` | celle de l'événement | Activité, « Pendant votre absence… » | ≤ 1 µs sans abonné ; arrêt ≤ 1 s | `PERF-EXT-START`, `PERF-EXT-STOP` |
| fournisseurs et outils d'IA | `ai-providers`, `ai-tools`, `ai-surfaces` | `ai.complete` (O4, par Go), classes d'outils | « Reformuler », « Corriger », discussion | premier jeton ≤ 1 s (local) | IA-5, fuzz SSE |
| erreurs, service de langage, outils d'écriture | `diagnostics`, `completion`, `hover`, `panels` | `document.read` | Problèmes, tinymist, Relecture, orthographe | p95 ≤ 100 ms | `PERF-LSP`, `problems.spec.ts` |
| greffons de projet | décisions `plugin/<id>` (PD-1) | niveau « projet » = contribuer, mandats | exemple « greffon de projet » (§15.15) | 16 greffons, 64 Mio | `EXT-PP-1`…`7`, `EXT-MANDATE-*` |
| débogage | Inspecteur, rejeu | mode développeur | — | sans effet hors mode | `EXT-DBG-*` (jamais pour un greffon de projet) |

## 5. Trous connus (à fermer)

Tous listés en [spec §16](spec.md#16-écarts-avec-le-code-au-2026-11-01) avec leur `AD-xx`. Les
décisions qui attendent l'utilisatrice sont marquées « Utilisatrice : oui » dans
[decisions.md](decisions.md).
