# Registre des exigences

Chaque demande de l'utilisatrice a une ligne ici, avec la date, l'endroit où elle est traitée et
son état. La source de vérité du détail reste le plan
[T11](plans/2026-10-25-t11-interface.md) et les ADR. Ce registre est mis à jour à chaque fusion.
Rien n'est « fait » sans tests verts et sans une passe `ux-walkthrough` réussie.

États : ⏳ à faire · 🔨 en cours · ✅ fait et testé · 🧪 fait, tests partiels · ❓ question ouverte

## Façon de travailler

| # | Date | Demande | Où | État |
|---|------|---------|----|------|
| W1 | 2026-10-25 | Démarrer avec les meilleures pratiques Go + IA (kit go-starter, encre, Internet) | CLAUDE.md, ADR 0001–0013 | ✅ |
| W2 | 2026-10-25 | Rechercher les bonnes pratiques sur Internet pour chaque élément (règle) | skill research-grounding | ✅ en continu |
| W3 | 2026-10-25 | Conception moderne, modulaire, performante et sécurisée (règle) | docs/engineering-principles.md | ✅ en continu |
| W4 | 2026-10-25 | Économie de tokens sans perte de qualité ; routage skills/agents ; modèles ouverts hébergés (OpenRouter), **pas de modèle local** | ADR 0013, scripts/llm.sh | ✅ |
| W5 | 2026-10-25 | GitHub Actions, mise, rien au hasard ; pratique, performant, beau | mise.toml, .github | ✅ |
| W6 | 2026-10-25 | Faire les meilleurs choix modernes soi-même (autonomie) | — | ✅ en continu |
| W7 | 2026-10-25 | Un seul PC : tous les tests en local | netsim, e2e locaux, e2e:webkit (podman) | ✅ |
| W8 | 2026-10-25 | Penser à tous les cas de chaque fonction (qui, état, entrée, résultat) | plan T11, matrices de cas | ✅ méthode |
| W9 | 2026-10-25 | Penser aux cas non évidents (simultanéité, retours en arrière, verrous) | plan T11 « Cas croisés » X-01…X-20 | ⏳ tests à écrire |
| W10 | 2026-10-25 | Tout implémenter, tout tester, puis l'interface de bout en bout | plan T11 vagues A–D | 🔨 |
| W11 | 2026-10-25 | Tester l'interface comme un humain (interactions naturelles, défauts, manques) | skill `ux-walkthrough`, agent `ux-tester` | 🔨 skill créé |
| W12 | 2026-10-25 | `/simplify` réel sur tout le code depuis le début | passes de rattrapage go-core, go-net, frontend ; règle mémoire | 🔨 |
| W14 | 2026-10-25 | Toujours peser le modèle à utiliser (règle) : le moins cher à qualité égale, `model:` fixé à chaque agent | CLAUDE.md, mémoire | ✅ règle active |
| W15 | 2026-10-25 | Router modèle **et** effort pour chaque tâche, sous-tâche ou skill (règle) | CLAUDE.md, mémoire | ✅ règle active |
| W16 | 2026-10-26 | Revue des tests unitaires : des tests utiles, qui couvrent tous les cas pertinents (pas « pour faire un test ») | mutation testing (gremlins, StrykerJS) + matrices de cas ; docs/quality/ | 🔨 |
| W17 | 2026-10-26 | Vérifier en début de session ce qui peut être délégué à OpenRouter | hook SessionStart `openrouter-check.sh`, modes `summarize`/`prose`/`check` de llm.sh | ✅ (crédit OpenRouter épuisé au 2026-10-26 → repli Haiku) |
| W18 | 2026-10-26 | Corriger les plantages mémoire (2 OOM globaux : mutant gremlins 8,7 Go dans `trust.test`, Chrome headless 3,4 Go, 7 agents en parallèle) | `scripts/heavy.sh` (cgroup 8 Go, 2 tâches lourdes max), tâches mise, règle CLAUDE.md | ✅ |
| W19 | 2026-10-31 | Se placer en architecte : vérifier et réorganiser pour un code performant, conforme aux normes de l'industrie et aux design patterns ; planifier, corriger, continuer | [audit](plans/2026-10-31-architecture-audit.md) | 🔨 |
| W20 | 2026-10-31 | Rien au hasard ni à l'intuition : chaque constat et chaque décision a une preuve, une règle et une gravité | référentiel de l'audit | ✅ en continu |
| W21 | 2026-10-31 | Toujours répartir le travail entre les modèles, Claude et Codex, avec modèle et effort explicites | CLAUDE.md (routage) | ✅ en continu |
| W13 | 2026-10-25 | Noter toutes les demandes pour les reprendre | ce registre | ✅ |

## Réseau, droits, sécurité

| # | Date | Demande | Où | État |
|---|------|---------|----|------|
| N1 | 2026-10-25 | Abandonner TCP brut au profit de libp2p | étape 2 | ✅ |
| N2 | 2026-10-25 | Un admin absent ne bloque jamais l'édition ; la créatrice peut transmettre | ADR 0017 | ✅ |
| N3 | 2026-10-25 | Déconnecter quelqu'un et le laisser revenir | ADR 0021, B3 | 🧪 Go fusionné (audité) ; panneau « Personnes » (B3b) : e2e S7 + walkthrough `docs/ux/walkthrough-2026-10-26-people.md` ; en attente de fusion |
| N4 | 2026-10-25 | Zones verrouillées par les admins, que les autres ne peuvent pas modifier | ADR 0022, B3 LCK-* | ⏳ |
| N5 | 2026-10-25 | Rejoindre un projet = recevoir tous les fichiers et dépendances | ADR 0023, B4 PRJ-*, e2e JOIN-3 (le 3e arrivant reçoit le texte : attestations rejouées avant l'adoption du projet, désormais gardées) | 🔨 Go en correction d'audit ; TS à faire |
| N6 | 2026-10-25 | Gestion multi-utilisateur partout (y compris le tableur) | B9 SHEET-MU-*, toutes sections | 🔨 |
| N7 | 2026-10-31 | Modèle de menace byzantin complet : même une personne qui écrit, malveillante, ne doit jamais gêner les autres (lecteurs et pairs non attestés strictement bornés) | audit S1/S1b, ADR 0031 à rédiger | 🔨 |
| N8 | 2026-10-31 | Fusionner tout de suite l'étape 1 (bornes dures à l'entrée de la webview, validation des messages) ; S1 reste Critique jusqu'à la garde ADR 0030 | audit S1, étape 1 | 🔨 |
| N9 | 2026-10-31 | Admission (ADR 0031), décisions D-1 à D-6 : 32 variantes de départ par personne qui écrit ; données purgées récupérables 30 jours puis effacées après accusé de réception de tous les appareils admin ; croissance anormale de l'historique détectée et signalée aux admins, sans blocage automatique ; purge irréversible une fois effacée ; adoption automatique par les co-éditeurs d'un document rendu orphelin ; historique antérieur intégré par une décision admin signée par document | `audit-2026-10-31/06-admission-design.md` §11, ADR 0031 | 🔨 |
| N10 | 2026-10-31 | ADR 0030, 0031 (rév. 6, avec l'arbitrage d'équivocation par un admin), 0032 et 0033 acceptées ; résiduel R1 accepté ; R2 fermé par un quota disque cumulé par personne qui écrit (amende D-3) | `docs/adr/`, `audit-2026-10-31/06-…` | 🔨 |
| N11 | 2026-10-31 | Étape 2.0 : seuls les appareils membres atteignent le Repo ; retrait et suspension déconnectent et drainent, TDD strict, sans écriture de refs git | `audit-2026-10-31/06-admission-design.md` §5.6, A14 ; `internal/app`, `internal/bridge`, `frontend/src/net` | 🧪 implémenté ; Vitest, svelte-check, lint, architecture et tests ciblés race verts ; intégration réseau bloquée par le sandbox |
| N12 | 2026-11-01 | Un protocole clair, explicable, défini et bien fait, ouvert à d'autres applications : spécification publique interopérable (langage normatif RFC 2119/8174, CDDL RFC 8610, encodage déterministe, cryptographie exacte, vecteurs de test, suite de conformité, versionnage) ; plus de correctifs au fil de l'eau ; tests utiles (propriétés, modèle, fuzzing) | `docs/protocol/`, ADR 0034 | 🔨 |

## Interface et édition

| # | Date | Demande | Où | État |
|---|------|---------|----|------|
| U1 | 2026-10-25 | UTF-8, fins de ligne par système, encodage, nombre de mots… et un très beau design | A2, A3, A4 | ✅ |
| U2 | 2026-10-25 | Utiliser les skills de design pour la maquette | maquette Artifact | ✅ |
| U3 | 2026-10-25 | Style reconnaissable Adwaita (GNOME) | A4, jetons libadwaita 1.8 | ✅ |
| U4 | 2026-10-25 | Barre de mise en forme selon le type de fichier (titres, gras, souligné…) | A1, A4 | ✅ |
| U5 | 2026-10-25 | Bloc de code **et** code en ligne | A1 FMT-14/16 | ✅ |
| U6 | 2026-10-25 | Icônes par type de fichier dans la liste | A4 UI-08 | ✅ |
| U7 | 2026-10-25 | Prendre en compte les modifications faites hors de l'interface | B1 EXT-* | 🧪 fait et testé pour `main.typ` (fusion à partir des têtes du dernier accord avec le disque, lignes marquées, Voir/Annuler) ; EXT-09 (renommage) et les autres fichiers texte attendent B4b/B5 ; zones verrouillées : crochet `skip` prêt (ADR 0022) |
| U8 | 2026-10-25 | Enregistrement automatique **ou** manuel, au choix | B1 SAV-* | ✅ |
| U9 | 2026-10-25 | Menu Insérer : pièces jointes PDF (`pdf.attach`), inclusions, références, toutes les possibilités, avec des libellés clairs | A1 FMT-22…26, A4 | ✅ insertion ; données réelles en C3 |
| U10 | 2026-10-25 | Présentations : diapositives, vue présentateur (chronomètre, suivante, notes, commentaires) | C4 PRS-* ; [plan](plans/2026-10-26-c4-present.md) : PRS-04 à 08, 10 à 13 faits ; PRS-01 à 03 et 09 non faits | ◐ |
| U11 | 2026-10-25 | Bande de modifications dans la marge (style Git) | B2 HIS-* | 🧪 fait et testé : bande ancrée par curseurs Automerge, minicarte, par type ou par personne |
| U12 | 2026-10-25 | Visualiser l'historique et les branches, revenir en arrière | B2 HIS-*, B2+ DIF-*, ZOOM-* | 🧪 tiroir Historique, différences lisibles, « Qui a écrit quoi », variantes, zoom du texte ; reste : onglet Commentaires (vide) et différences des autres fichiers (B5) |
| U13 | 2026-10-25 | Gestionnaire de fichiers : ajouter, supprimer (sur accord), répliqué, historisé, restaurable | B5 FM-* | ⏳ |
| U14 | 2026-10-25 | Recherche normale, joker, regex ; liste des lignes à la Notepad++ ; atteindre chaque résultat | B6 SRC-* | 🧪 moteur et panneau fusionnés ; branchement dans la coque à faire |
| U15 | 2026-10-25 | Inclure des documents dans d'autres (Typst), liens internes, référencer un document | B7 LNK-* | ⏳ |
| U16 | 2026-10-25 | Voir dans la liste des fichiers qui édite quoi | B7 PRS-21…26 | ⏳ |
| U17 | 2026-10-25 | Liens vers images et fichiers marqués présents ou absents ; liens externes jamais testés (hors ligne) | B7 LNK-* | ⏳ |
| U18 | 2026-10-25 | Choisir la mise en page des Markdown, prise en compte par l'aperçu | B8 MDL-* | ⏳ |
| U19 | 2026-10-25 | Tableur CSV/TSV : édition complète, icônes, copier-coller facile, plusieurs Mo, graphiques dans Typst | B9 CSV-* | 🧪 fusionné (B9), passe ux-walkthrough à faire |
| U20 | 2026-10-25 | Tableur extrêmement pratique et performant, UX type Microsoft/Apple | B9 SHEET-* | 🧪 fusionné (B9), passe ux-walkthrough à faire |
| U21 | 2026-10-25 | Priorité : copier-coller, tirer une ligne (poignée), ajout/suppression de lignes | B9 | 🧪 fusionné (B9), passe ux-walkthrough à faire |
| U22 | 2026-10-25 | Tous les problèmes des CSV : séparateur, cellules qui le contiennent, recherche… | B9 CSV-EDGE-* | 🧪 fusionné (B9), passe ux-walkthrough à faire |
| U24 | 2026-10-25 | L'application s'affiche bien quel que soit le redimensionnement ; panneaux internes ajustables | A6 RSZ-* | ⏳ après la passe /simplify frontend |
| U25 | 2026-10-26 | Tableur : séparateur selon l'extension (CSV/TSV/PSV) puis confirmé par détection ; lire tout fichier texte ordinaire ; icônes adaptées pour tous ces types | B9, B4b/B5 | 🧪 fusionné (B9), passe ux-walkthrough à faire |
| U26 | 2026-10-26 | Types de fichiers modulaires : ajouter un type = ajouter un module (registre, éditeur chargé à la demande) | ADR 0027, B4b | 🔨 |
| U27 | 2026-10-26 | Zoom du texte ; différences « comme Git » très claires : qui, quoi exactement, quand — simple et beau | B2+ DIF-*, ZOOM-* | 🔨 |
| U28 | 2026-10-26 | Présentation : vue présentateur (suivante, notes, commentaires) et vue public (diapo seule) en deux fenêtres simultanées pour deux écrans ; public sur une machine, présentateur sur une autre en réseau, synchronisés | C4+ PRS-30…40 ; [plan](plans/2026-10-26-c4-present.md) : fait ; fenêtre public ordinaire, placée et mise en plein écran par la personne | ◐ |
| U29 | 2026-10-26 | Tâches (TODO, FIXME…) dans les commentaires, retrouvées dans un panneau du projet, bien présentées | B10 TODO-* | ⏳ |
| U30 | 2026-10-26 | Panneau de gauche : gestionnaire de fichiers **ou** sommaire cliquable des titres | B10 OUT-* | ⏳ |
| U31 | 2026-10-26 | Toute fonctionnalité intégrée et cohérente (jamais une « verrue ») : très bon design UX/UI, beau et harmonieux partout | règle de conception ; revue de cohérence `ux-walkthrough` à chaque fusion d'interface | 🔨 règle active |
| U32 | 2026-10-26 | Programme globalement très fiable : on doit pouvoir lui faire confiance | Q-REL (plan T11 « Fiabilité et performance », REL-01…20), [ADR 0029](adr/0029-reliability.md) | 🔨 conception proposée |
| U33 | 2026-10-26 | Très performant, y compris par Internet (relais, NAT) et sur des fichiers texte énormes à afficher | Q-PERF (plan T11 « Fiabilité et performance », PERF-01…20), [ADR 0028](adr/0028-performance-huge-files-and-wan.md) | 🔨 conception proposée |
| U34 | 2026-10-26 | Les fichiers de 0,5 à 2 Mi caractères restent modifiables, avec un bandeau discret de lenteur, jusqu'à l'arrivée du Worker Automerge (pas de lecture seule) | ADR 0028 D1, `store/limits.ts`, PERF-02..04 | 🔨 palier et bandeau faits |
| U34 | 2026-10-26 | Présentation locale simple : une 2ᵉ fenêtre pour le public, que l'on agrandit en plein écran soi-même (pas de placement automatique) | C4, PRS-30/32 révisés | 🔨 |
| U23 | 2026-10-25 | Coloration syntaxique (constat de revue) | A5 | ✅ |

## Livraison

| # | Date | Demande | Où | État |
|---|------|---------|----|------|
| L1 | 2026-10-25 | Application autonome : tout dans un exécutable | Objectif final, C1 (sidecars embarqués) | 🔨 |
| L2 | 2026-10-25 | `.deb` et `.rpm` pour amd64 et arm64 | Objectif final PKG-*, D1 | 🔨 |
| L3 | 2026-10-25 | Versionner en SemVer | Objectif final, D1, ADR 0025 (à venir) | 🔨 |
| L4 | 2026-10-25 | Icône moderne, fluide, agréable, représentative | `build/icons/` | ✅ « les traits qui se rejoignent » choisie |
| L5 | 2026-10-25 | **But :** une release GitHub testée et fonctionnelle à 100 % (exécutable seul, deb, rpm) qui s'installe proprement | Objectif final PKG-01…12, D1 | 🔨 |

## Questions ouvertes

| # | Question | Décision |
|---|----------|----------|
| Q1 | « peut-être » compte pour 1 ou 2 mots ? | 1 mot (comme Word et LibreOffice), à confirmer |
| Q2 | Garder « Page X sur Y » dans la barre d'état ? | sans réponse ; gardé (C2 : « Page X sur Y » dans la barre d'état, quand l'aperçu est affiché) |
