# Architecture d'inkwell — doutes, options, décisions

| | |
|---|---|
| Version | 1.0.0-brouillon.7 (2026-11-01, révision 6 : tour 5 ; [CHANGES.md](CHANGES.md), [coherence.md](coherence.md)) |
| Rôle | lever chaque doute trouvé en écrivant [spec.md](spec.md) : une question, ses options, une recommandation, sa raison |
| Sources des doutes | [conception](../conception.pdf), [questions ouvertes](../open-questions.md), [audit](../plans/2026-10-31-architecture-audit.md), ADR 0001–0034, [protocole](../protocol/spec.md), [design](../design/spec.md), code (`main` à `362cb4d`, chantier `proto1`), `TODO` et `t.Skip` |

Règles :

- Une `AD-xx` ne change aucune ADR acceptée. Quand la recommandation touche une frontière,
  un invariant ou une décision acceptée, elle dit quelle **nouvelle ADR** l'actera.
- **Utilisatrice** = la décision lui revient (portée produit, vie privée, écart à la
  conception, acceptation d'ADR). Les autres sont techniques : la recommandation s'applique
  dès que la spécification est acceptée, sauf objection en revue.
- Une décision prise sort de cette page vers une ADR ou vers `spec.md` ; son identifiant
  n'est jamais réutilisé.

## Synthèse

| # | Sujet | Recommandation | Utilisatrice | Se ferme par |
|---|---|---|---|---|
| AD-01 | Plusieurs projets ouverts | v1 : **un seul projet ouvert à la fois**, dans un seul processus ; architecture prête pour plusieurs | **oui** (écart à la conception p. 13) | spec N-1 ; ADR si plusieurs |
| AD-02 | Où vit l'autorité (`Wiring`) | un acteur `internal/ledger`, plis purs dans `protocol` | non | ADR 0036 |
| AD-03 | Alias `internal/trust` → `protocol/trust` | toléré à l'étape 1 seulement, garde AC-15 | non | ADR 0034 étape 2 |
| AD-04 | Composition processus / projet | `cmd/inkwell` = processus ; `app.Host` tient le `Runtime` courant | non | ADR 0036 |
| AD-05 | Aperçu dans `internal/app` | extraire `internal/preview` | non | ADR 0036 |
| AD-06 | Port réseau pour la session v2 | `transport.Conn`/`Stream`, implémenté par `p2p` et `memnet` | non | ADR 0034 étapes 4 et 10 |
| AD-07 | Statuts d'ADR incohérents | corriger l'index ; faire accepter 0021–0023, 0026, 0028, 0029 | **oui** | index des ADR |
| AD-08 | ADR 0027 citée mais absente | la rédiger (registre des types de fichiers) | non | ADR 0027 |
| AD-09 | « Rotation de la clé de session » (ADR 0005, point 5) | sans objet ; le noter | non | ADR 0036 (renvoi) |
| AD-10 | Version des données stockées | entier `layout`, lecture seule si plus récent, migration atomique | non | ADR 0035 |
| AD-11 | Rangement des ensembles signés | `.inkwell/sets/<ensemble>/` ; `trust/` migré à la disposition 2 | non | ADR 0035 |
| AD-12 | Instance unique, verrous | instance unique Wails + `device.lock` + `.inkwell/lock` | non | ADR 0037 |
| AD-13 | Versions nommées → Git | **go-git** (Go pur, N39) en projection reconstructible, jamais de binaire `git` ; mesuré (P-GIT-BENCH) | non (N39 a tranché) | spec §6.11 |
| AD-14 | Commentaires des lecteurs | admission bornée au sous-arbre des commentaires | **oui** | ADR à rédiger (avant l'étape 4) |
| AD-15 | Journal circulaire réseau | métadonnées seulement | non | spec O-3 |
| AD-16 | Journaux, rapport de bug | fichier tournant local ; rapport = archive locale + page de ticket préremplie | **oui** (envoi) | ADR 0037 |
| AD-17 | Mises à jour | manifeste signé ; `.deb`/`.rpm` : avertir seulement ; activée par défaut | **oui** | ADR 0038 |
| AD-18 | Plates-formes | **six cibles** (N35) ; port d'enfermement par OS, capacité refusée tant qu'une porte manque ; Linux seul = alpha | non (N35 a tranché) | spec §9.11 |
| AD-19 | Wails beta.34 | passer à v3.0.x stable par la procédure de l'ADR 0008 | non | lot dédié |
| AD-20 | LSP tinymist | remplacée par `AD-80` | non | spec §15.17 |
| AD-21 | Réglages de l'appareil | `settings.json` en Go, versionné ; `INKWELL_*` pour les tests | non | ADR 0037 |
| AD-22 | Étiquettes de test dans l'exécutable publié | porte AC-14 (`go version -m`) | non | job de publication |
| AD-23 | Liste de membres scellée à 1024 ops | garder ; alerte à 80 % et « recréer avec les mêmes personnes » | non | spec §2 |
| AD-24 | Valeur du quota N10 | 1 Gio d'historique par personne qui écrit et par projet, extensible par décision | **oui** (valeur) | spec §6.5 |
| AD-25 | Durable avant diffusé | un travailleur de persistance par magasin ; émission après acquittement | non | ADR 0034 étape 5 |
| AD-26 | Aperçu sous 300 ms | mesurer `typst watch` et tinymist par stdio ; aucun port ; budget révisé | non | essai chronométré |
| AD-27 | Mesure « frappe → pair » | 5 car./s ; rafale = + cadence de synchronisation | non | spec §12.3 |
| AD-28 | Lecture des sidecars bornée | Landlock en lecture, mode dégradé journalisé | non | spec §9.5 |
| AD-29 | `bridge` importe `project` et `store` | ports à lui ; hub `present` dans `bridge` | non | ADR 0036 |
| AD-30 | Couches du frontend non vérifiées | Biome `noRestrictedImports` dans `fe:check` | non | `fe:check` |
| AD-31 | `protocol/bridge` dans le module public | rendre l'enveloppe du pont à `internal/bridge` | non | revue de l'étape 1 |
| AD-32 | Couleur et nom d'une personne | couleur choisie portée par la présence ; nom fixé à l'admission en v1 | **oui** (design Q4) | spec §6.1 |
| AD-33 | Import et export DOCX, RTF, ODT | **aucune intégration native, aucun pandoc livré** (N40) ; greffons possibles | non (N40 a tranché) | spec §1.3 NG1 |
| AD-34 | Aperçu partagé pour un invité sans l'application | v2 ; exigera sa propre ADR (serveur hors boucle locale) | non | v2 |
| AD-35 | Compagnon en ligne de commande | `export` en v1.x (projection) ; `join --headless` en v2 avec ADR | non | v2 |
| AD-36 | Test ignoré `files_test.go:713` | l'écrire maintenant que la suspension est fusionnée | non | test actif |
| AD-37 | Questions ouvertes de 2026-10-25 | clore celles que les ADR ont tranchées | non | `open-questions.md` |
| AD-38 | Archive de projet autonome | zip déterministe, sans `.inkwell/` | non | spec §6 (étape 6) |
| AD-39 | Visionneuse PDF | PDF.js épinglé, `isEvalSupported: false`, dans un Worker | non | étape 4 |
| AD-40 | Sous-modules et `HOT_PATHS` | tâches mise sur les trois modules ; erratum `protocol/wire` | non | `mise.toml` |
| AD-41 | Projet sur clé USB, disque réseau, dossier synchronisé | détecter, avertir, lecture seule si les garanties manquent | non | spec §10.3 |
| AD-42 | Recherche dans tout le projet | Go cherche la projection (RE2) ; la webview cherche les documents ouverts | non | spec §4 |
| AD-43 | Relais par défaut | aucun relais tenu par le projet en v1 ; relais du groupe | **oui** | ADR (précise 0004) |
| AD-44 | Accessibilité de l'aperçu et du PDF | **équivalent accessible** : « Vue lecture » (export HTML de Typst assaini, sans script) + descriptions de diagrammes ; PDF balisé, option PDF/UA-1 | non | spec §2 |
| AD-45 | Tests intermittents F3, F6, F8, F9, PRS-37 | mesurer et prouver la cause avant la bascule ADR 0034 | non | porte P-* |
| AD-46 | Un document par fichier ou par projet | un document Automerge par fichier texte + index `G` séparé (ADR 0030 §3) ; tout désigne un nœud, jamais un chemin | non | spec §6.6 MF-1–4 |
| AD-47 | Documents ouverts, onglets, document actif | **onglets** (N19) : ouverts ≠ chargés ≠ actif, un éditeur visible | non (N19 a tranché) | spec §6.6 |
| AD-48 | Fichier principal ou fichier édité | l'aperçu compile toujours le principal (décision d'admin) ; « Aperçu de ce fichier » à part | non | spec §6.6 MF-8–10 |
| AD-49 | Enregistrement par fichier | projection de tous les textes admis ; « non enregistré » par fichier ; Ctrl+S = tout | non | spec §6.6 MF-11–13 |
| AD-50 | Renommer ou supprimer un fichier ouvert ailleurs | l'éditeur suit le nœud ; corbeille = bandeau, frappe continue | non | spec §6.6 |
| AD-51 | Registre des types de fichiers | une table de données partagée Go/TS + modules TS à la demande ; noyau inscrit comme les greffons | non | ADR 0027 |
| AD-52 | Tableaux CSV : texte ou modèle structuré | texte (ADR 0026) + **détection des conflits structurels au niveau des opérations** et réparation attribuée (MF-25), oracle à identités de lignes ; essai du modèle structuré à l'étape 4 | **oui** (avec l'acceptation d'ADR 0026) | ADR 0026 |
| AD-53 | Bornes de taille par type | paliers ADR 0028 par type ; grille sur index de lignes ; images ≤ 100 Mpx | non | spec §12.2 |
| AD-54 | Grammaires de coloration | liste fermée de langages Lezer, découpés par Vite, embarqués, chargés à la demande | non | spec §6.6 MF-18 |
| AD-55 | Fichier binaire ou inconnu | reniflage WHATWG partagé Go/TS ; texte brut ou carte de blob | non | spec §6.6 MF-15 |
| AD-56 | Un seul point d'entrée « ouvrir » pour tous les types | `open(nodeId)` → registre → vue ; fin de `?file=` | non | spec §6.6 MF-5 |
| AD-57 | Où tournent les greffons | **un processus `inkwell-plugin-host` par greffon actif** (binaire statique Go pur, démarrage en deux temps), ≤ 6 actifs ; **expérimentation** jusqu'à P-PLUG-PROTO et P-PLUG-BENCH | **oui** (ADR 0039) | spec §0.5, §9.10 |
| AD-58 | Droits des greffons | capacités nommées ; **règle de flux fermée** (sources I0–I3 × puits O0–O4, RF-1 à RF-5) ; secrets injectés par Go | **oui** (ADR 0039) | ADR 0039 |
| AD-59 | Surface et versions de l'API | WIT `inkwell:plugin@1.x.y`, SemVer, règles d'évolution du protocole (N14) | non | ADR 0039 |
| AD-60 | Greffons et collaboration | jamais d'installation **globale** depuis un pair ; greffons de projet bornés (`AD-69`) ; rendu dégradé sans greffon | **oui** | ADR 0039 |
| AD-61 | Distribution et signature | fichier `.inkwell-plugin` signé, continuité de clé ; dépôt optionnel plus tard, jamais sans geste | **oui** (dépôt ou non) | ADR 0039 |
| AD-62 | Budgets des greffons | démarrage inchangé ; appel p95 ≤ 2 ms (≤ 64 Kio) ; interface jamais bloquée | non | spec §2 |
| AD-63 | Le noyau sur les mêmes points d'extension | registre unique des contributions ; une fonction livrée comme greffon (U29) | non | ADR 0027, 0039 |
| AD-64 | Ce qui change le PDF | seulement des paquets Typst fournis (greffons Typst inclus) ; un greffon inkwell ne change jamais la compilation | non | ADR 0039 |
| AD-65 | Points d'extension déclarés par un greffon | retenu en API 1.1 : graphe de dépendances SemVer, cycles et conflits refusés, règle d'exclusion par composante | non | spec §15.12 |
| AD-66 | Données d'extension dans le document partagé | différé ; v1 : magasin privé et contenu standard | non | ADR future |
| AD-67 | Dorsal de stockage par extension | sauvegarde personnelle (v1) ; clé par projet et par personne, AEAD avec AAD, graphe objets → manifeste → tête ; **fraîcheur non vérifiable sur un appareil neuf** (résiduel écrit, ancres : phrase datée, membres) | **oui** (portée v1) | spec §6.7, ADR 0039 |
| AD-68 | Nuage partagé entre membres | v2 : clé de stockage de projet distribuée par le ledger, ADR à part | **oui** | v2 |
| AD-69 | Greffons de projet | **contribuer seulement**, mandats bornés et révocables, un hôte par greffon (jamais partagé avec un autre greffon) ; **bloqué par PD-1** (mineure du protocole) | **oui** (N23 à confirmer) | spec §15.10 |
| AD-70 | Diagrammes : contrat de rendu et rendus partagés | contrat pur en `wasm` (Graphviz livré) ; rendus = **fichiers ordinaires** du projet (aucun delta du protocole) ; **Mermaid différé** (P-FRAME) | non | spec §15.14 |
| AD-71 | Expérience des développeuses | API étagée 1.0, 1.1, 1.2 (plafonds de surface par étape) ; Rust et Go en 1.0 ; bonjour bilingue en 10 lignes | non | spec §15.15 |
| AD-72 | Spécification du protocole des greffons | `docs/plugin-api/` normative, deux axes de version, détection de fonctions, N/N−1 avec adaptateur ; boucle de revue Fable + Codex | **oui** (acceptation) | `docs/plugin-api/` |
| AD-73 | Surfaces d'interface et bouton d'extension | contribution `{commande, emplacement, groupe, ordre, quand}` ; une action par extension, épinglée par la personne seule | non | spec §15.13 |
| AD-74 | Mode débogage | piles DWARF, Inspecteur, enregistrer et rejouer ; pas à pas par rejeu sous Wasmtime ou en natif | non | spec §15.15 |
| AD-75 | Thèmes | données typées sur `tokens.json`, contrastes WCAG vérifiés au chargement | non | spec §15.8 |
| AD-76 | Cycle de vie et événements | bus versionné, asynchrone, coalescé, borné ; arrêt : données d'abord, 1 s pour les greffons | non | spec §15.9 |
| AD-77 | Corriger G-0 (clic sans effet) | chemin minimal G-0a tout de suite (lecture seule des autres fichiers), puis G-0b, G-0c | non | spec §6.6 |
| AD-78 | Modèles de projet (N27) | aucun livré ; bibliothèque locale ; `typst.toml [template]` ; jamais `typst init` ; Markdown stylé par Typst, CSS pour l'export HTML seulement | **oui** (CSS) | spec §6.8 |
| AD-79 | Erreurs comme un IDE (N28) | onglet « Problèmes » unique, F8, diagnostics ancrés, messages par table déterministe | non | spec §15.17 |
| AD-80 | Service de langage tinymist (N29) | stdio enfermé, `didChange` depuis la webview, versions et réponses périmées jetées, p95 ≤ 100 ms, docstrings sûrs | non | spec §15.17 |
| AD-81 | Outils d'écriture (N30) | recommandations 11–18 de l'étude, en extensions intégrées | non | spec §15.17 |
| AD-82 | IA de premier rang (N31) | `ext/ai` ; fournisseurs compatibles OpenAI appelés par Go ; politique de projet ; propositions ; règle des deux ; MCP stdio par socket `0600` | **oui** (défauts, ADR 0040, 0041) | spec §15.18 |
| AD-83 | Deltas du protocole demandés | PD-1 (espaces `plugin/`, `ai/`, `ignore/`, encodage, règle de pli) ; PD-2 **retiré** (préfixes externes) ; PD-3 reporté ; mineure 2.1 à faire accepter par la boucle du protocole | **oui** (acceptation) | `docs/protocol/` |
| AD-84 | Fidélité à la conception | aucune fidélité complète affirmée tant que les écarts de la matrice ci-dessous sont ouverts | **oui** | matrice |
| AD-85 | Ordre de livraison de l'API | API 1.0 réduite, puis 1.1, 1.2 ; registre → commande, panneau, proposition → Graphviz → dorsal | non | spec §15.21 |
| AD-86 | Canal Go ↔ hôte des greffons | flux d’octets (`SOCK_STREAM`, tubes sous Windows), trames bornées, autorité imposée par le parent, un appel à la fois par instance | non | spec §7.3 |
| AD-87 | Enfermement de l'hôte (Linux) | démarrage en deux temps (Landlock puis `execve`), seccomp avec `TSYNC` barrière principale, un hôte par greffon, aucun fichier (memfd), profil AppArmor livré, chien de garde du parent, échec fermé visible ; **expérimentation** | non | spec §9.10 |
| AD-88 | Gestionnaire de tâches (N32) | vue « Ressources » du cœur (`internal/tasks`), mesures par le parent, limites réglables à la baisse | non | spec §11.1 |
| AD-89 | Index de recherche pour l'IA (N33) | lexical local d'abord, moteur choisi par la mesure, provenance par extrait, citations par nœud ; **expérimentation** (P-IDX) | non | spec §15.18 |
| AD-90 | Fichiers non synchronisés (N34) | politique d'import et de réception locale ; règle de projet par décision signée (PD-1) ; jamais `.typstdeps/` | non | spec §6.9 |
| AD-91 | Projet chiffré (N37) | coffre par objet sans montage ; DEK par projet et par appareil, charge utile d'un fichier age ; dossier de travail en mémoire ou sur disque chiffré vérifié, sinon compilation refusée ; ≤ 512 Mio ; **expérimentation** (P-VAULT) | **oui** (ADR, remplace NG3) | spec §6.10 |
| AD-92 | Historique sans Git (N39) | voyager, restaurer par nouvelle modification, comparer par mots, versions = manifestes de têtes | non | spec §6.11 |
| AD-93 | Performance mesurée (N38) | budgets par chemin, règles de concurrence bornée, PGO, portes CI, choix de bibliothèque par banc | non | spec §12.5 |
| AD-94 | Port d'enfermement six cibles (N35) | `internal/sandbox` : Landlock et seccomp, Seatbelt, AppContainer et objets de tâche ; INV-13 ; **expérimentation** par OS | non | spec §9.11 |
| AD-95 | Ce que vaut l'acceptation | les expérimentations ne sont acceptées qu'à leur porte (§0.5) | **oui** | spec §0.5 |
| AD-96 | Variantes et intégrations (N41) | sur les variantes existantes (ADR 0031) ; « Intégrer » = **différence appliquée** en modifications nouvelles, annulable en un groupe ; fusion native écartée en v1 ; entre projets différé | non | spec §6.11 HI-6, HI-7 |
| AD-97 | Carte produit et surfaces | tiroir à trois destinations, gauche à deux onglets, un compteur de problèmes, barre d'état à quatre éléments | non (design rév. 5) | `coherence.md` §4 |
| AD-98 | Formats bureautiques (N40) | aucun natif ; collage riche dans le cœur ; greffons | non | spec §1.3 |

**Nouvelles ADR proposées** (numéros réservés) : 0027 registre unique des contributions
(types de fichiers, commandes, panneaux, exportateurs ; `AD-08`, `AD-51`, `AD-63`) ; 0035 disposition et migration des données (`AD-10`, `AD-11`) ; 0036 découpage de
`internal/app` et acteur `internal/ledger` (`AD-02`, `AD-04`, `AD-05`, `AD-09`, `AD-29`) ;
0037 état local de l'appareil : verrous, réglages, journaux (`AD-12`, `AD-16`, `AD-21`) ;
0038 mises à jour (`AD-17`) ; ADR à rédiger « chiffrement au repos » qui remplace NG3 (`AD-91`) ; 0039 greffons WebAssembly (`AD-57` à `AD-76`, brouillon dans
[spec.md, annexe](spec.md#annexe--brouillon-dadr-0039--greffons-webassembly)). Sous réserve du choix de l'utilisatrice : `AD-01` (si
plusieurs projets), `AD-14`, `AD-43`, `AD-68` (`AD-13`, `AD-18`, `AD-33` sont tranchés par N39, N35, N40). **Spécification à écrire ensuite** : `docs/plugin-api/` (`AD-72`), avec sa propre boucle de revue Fable + Codex.

### Matrice des décisions finales (ce que l'acceptation de cette spécification ne vaut pas)

Accepter `spec.md` vaut décision pour toutes les recommandations **techniques** (colonne
« Utilisatrice : non »), **sauf** les expérimentations du §0.5 de la spec, acceptées seulement à
leur porte (`AD-95`). Elle **ne vaut pas** les décisions ci-dessous, qui exigent un acte à part ;
tant qu'elles sont ouvertes, la spécification n'affirme pas sa fidélité complète à la conception
(`AD-84`).

**Tranché par l'utilisatrice depuis la révision 2** (plus de décision à demander) : plates-formes
(N35 → `AD-18`), historique par go-git (N39 → `AD-13`), aucun format bureautique natif (N40 →
`AD-33`, `AD-98`), onglets (N19 → `AD-47`).

| AD | Recommandation | Qui décide | Amendement nécessaire | Écart à la conception |
|---|---|---|---|---|
| AD-01 | un seul projet ouvert à la fois en v1, `Runtime` par projet | utilisatrice | aucun (design §5.2) | **oui**, p. 13 |
| AD-07 | accepter 0021–0023, 0026 (seuil remplacé), 0028, 0029 | utilisatrice | statuts d'ADR | — |
| AD-14 | commentaires des lecteurs par admission bornée | utilisatrice | **mineure du protocole** (couverture, protocole §9.4) + ADR | p. 11–12 sinon |
| AD-17 | mises à jour : manifeste signé, avertir pour `.deb`/`.rpm` | utilisatrice (défaut actif ou non) | ADR 0038 | — |
| AD-24 | quota 1 Gio par personne qui écrit | utilisatrice | — | — |
| AD-26 | aperçu ≤ 1 s complet, ≤ 300 ms incrémental | technique, après mesure | **amendement de `engineering-principles.md` §3 par ADR** si 300 ms est inatteignable en complet | — |
| AD-43 | aucun relais tenu par le projet | utilisatrice | ADR (précise 0004) | — |
| AD-52 | texte + conflits structurels pour tous les observateurs | utilisatrice (avec AD-07) | ADR 0026 acceptée | — |
| AD-57, AD-58, AD-61 | greffons : un hôte par greffon, règle de flux, fichier seulement | utilisatrice | **ADR 0039**, après P-PLUG-PROTO, P-PLUG-BENCH, P-SANDBOX-*, revue de sécurité | — |
| AD-67 | N22 servie en v1 par la sauvegarde personnelle, non déclarée satisfaite | utilisatrice | ADR 0039 | N22 partielle |
| AD-69 | greffons de projet bornés | utilisatrice | **PD-1** + ADR 0039 | N23 bornée |
| AD-70 | Mermaid complet différé ; sous-ensemble sans DOM en API 1.1 | utilisatrice | — | Mermaid partiel |
| AD-72 | `docs/plugin-api/` | utilisatrice | sa propre boucle de revue | — |
| AD-78 | pas de CSS pour le PDF | utilisatrice | — | N27 « Markdown + CSS » |
| AD-82 | IA : défauts de politique, MCP | utilisatrice | **PD-1**, ADR 0040, 0041 | — |
| AD-83 | PD-1 (PD-2 retiré, PD-3 reporté) | utilisatrice | boucle du protocole (2.1) | — |
| AD-91 | projet chiffré par objet, DEK par appareil | utilisatrice | **ADR** (remplace NG3), après P-VAULT | interprétation de « bloc chiffré » par objet |
| AD-95 | expérimentations acceptées à leur porte | utilisatrice | — | — |

---

## Structure du code

### AD-01 — Plusieurs projets ouverts en même temps

- **Doute.** La conception (p. 13) dit : « plusieurs projets ouverts = plusieurs fenêtres
  Wails, chacune avec son `Repo`, ses connexions et ses sidecars ; un même pair peut être
  connecté à plusieurs projets ». Le design (§5.2) dit : « un projet ouvert remplace le
  précédent ; ouvrir dans une nouvelle fenêtre est en v2 ». Le code ouvre un projet à la fois.
- **Options.**
  1. *v1 : un projet par processus.* Changer de projet ferme le `Runtime` (arrêt §5.4,
     étapes 3 à 6) et en ouvre un autre. Simple ; ce que fait le code.
  2. *Plusieurs projets dans un processus.* Un seul hôte libp2p (une clé = un PeerID), des
     sessions indexées par (PeerID, `ProjectID`) — le HELLO porte déjà le `ProjectID`, le
     protocole ne change pas ; un `Runtime` et un `Repo` par fenêtre ; filtre de connexion =
     union des listes.
  3. *Un processus par projet.* Deux hôtes avec la même clé, donc le même PeerID : réservations
     de relais et annonces mDNS en conflit. Écarté (ou une clé par projet, ce qui casse « la clé
     est l'appareil »).
- **Recommandation.** Option 1 en v1, en gardant l'option 2 ouverte : `Runtime` par projet
  (déjà le cas), aucune variable de processus qui suppose un seul projet hors de `app.Host`
  (`AD-04`), sessions indexées par (PeerID, `ProjectID`) dès `internal/transport/session`.
- **Raison.** Le design l'a déjà choisi ; l'option 2 double la surface de test (deux
  `Runtime`, filtre d'union) pour un gain que personne n'a demandé. Résiduel : un projet fermé
  ne se synchronise pas (comme Teamtype) ; l'édition hors ligne le couvre (G3).
- **Utilisatrice : oui** — écart à la conception p. 13.

- **Révision 2.** La synthèse disait « un projet par fenêtre et par processus » : formulation ambiguë, corrigée en « un seul projet ouvert à la fois, dans un seul processus » (option 1 du corps). Écart à la conception (p. 13) inscrit dans la matrice.

### AD-02 — L'autorité du projet : un acteur `internal/ledger`

- **Doute.** L'audit (M5) demande de sortir l'autorité d'`internal/app` vers
  « `internal/session` ». L'ADR 0030 nomme `internal/policy`, l'ADR 0031 `internal/listing` et
  `internal/session` ; l'ADR 0034 crée `internal/transport/session` (acteur d'un pair) ; le
  §15 du protocole parle de « l'acteur `Wiring` ». Deux `session` différentes, trois paquets
  pour un seul état.
- **Options.**
  1. Trois paquets (`policy`, `listing`, `session`), un acteur chacun.
  2. Un acteur `internal/ledger` qui possède tous les ensembles signés, leurs vues et leur
     persistance ; les plis purs (liste, décisions, annonces, ensemble admis) dans
     `protocol/{trust,policy,listing}`.
  3. Laisser dans `internal/app`.
- **Recommandation.** Option 2.
- **Raison.** L'ensemble admis (protocole §4.10) est **un** pli de trois ensembles à la fois
  (« recalculé à chaque changement des ops de membres, de décisions ou d'annonces ») ; L4 et L5
  croisent membres et décisions. Trois acteurs publieraient trois instantanés décalés : le
  verdict d'admission dépendrait de l'ordre d'arrivée, ce que le protocole interdit
  (« chargement = pli »). Un acteur publie une seule `View` immuable cohérente, a un seul
  chemin « durable avant diffusé » (`AD-25`) et un seul ordre d'arrêt. Le nom `ledger` évite la
  collision avec `transport/session`. Les ADR 0030 et 0031 ne fixent que des règles ; leurs noms
  de paquets sont indicatifs (l'étape 1 d'ADR 0034 a déjà déplacé leurs codecs).
- **Ferme par** ADR 0036 (acte les noms, ne change aucune règle des ADR 0030, 0031, 0034).

### AD-03 — Alias `internal/trust` vers `protocol/trust`

- **Doute.** Le chantier `proto1` garde `internal/trust/protocol.go`, des alias de types vers
  `protocol/trust`. `arch-check.sh` vérifie encore les racines `internal/trust/...`.
- **Recommandation.** Alias tolérés pendant l'étape 1 seulement ; l'étape 2 réécrit les
  imports, supprime `internal/trust` et ajoute AC-9, AC-10, AC-15 à `arch-check.sh` (les
  racines du cœur deviennent `protocol/...`).
- **Raison.** Deux chemins vers le même type cachent les imports réels au contrôle
  d'architecture et contredisent « aucune copie privée » (INV-10).

### AD-04 — Qui compose : le processus ou le projet

- **Doute.** Les services Wails sont enregistrés une fois par processus, mais le `Runtime`
  change avec le projet ouvert.
- **Recommandation.** `cmd/inkwell` = processus : instance unique (`AD-12`), Wails, Streams,
  arrêt. `internal/app.Host` (un par processus) tient le `Runtime` courant derrière un
  `atomic.Pointer` ; les services appellent `Host`, jamais un `Runtime` gardé de côté ; changer
  de projet = refuser les commandes, fermer l'ancien `Runtime`, publier le nouveau.
- **Raison.** Pas de service qui écrit dans un projet fermé ; prépare `AD-01` option 2 sans la
  coûter.

### AD-05 — Extraire `internal/preview`

- **Doute.** `internal/app/preview.go` (25 Kio) porte l'anti-rebond, l'annulation, les pages
  visibles et l'export ; ses tests PRV-10 et PRV-32 ont échoué par intermittence (audit F1).
- **Recommandation.** Paquet `internal/preview`, port `Compiler` défini chez lui, temps
  injecté, tests sous `testing/synctest` ; règle AC-12.
- **Raison.** Un paquet = un rôle (M5) ; un paquet dont les tests ne tirent pas 610
  dépendances ; l'intermittence se diagnostique mieux sur une horloge simulée.

### AD-06 — Le port réseau de la session v2

- **Doute.** L'invariant dit « tout le réseau passe par `internal/transport.Transport` » ; la
  session v2 a besoin de flux par classe, pas de messages. Elle ne peut pas importer libp2p
  (AC-4).
- **Options.** (1) la session importe les types libp2p — interdit ; (2) `internal/transport`
  définit `Conn` (`OpenStream(classe)`, `AcceptStream`, `RemotePeer`, `Close`) et `Stream`
  (`Read`, `Write`, `CloseWrite`, `Reset`, délais), que `p2p` adapte et que `memnet` implémente
  avec une fenêtre de crédit par flux ; (3) la session dépend de `protocol/libp2p`.
- **Recommandation.** Option 2.
- **Raison.** La frontière reste `internal/transport` ; la simulation déterministe reste
  possible et peut prouver l'absence de blocage en tête de file (exigé par le §10 du
  protocole). L'ancien `Transport` par messages disparaît à l'étape 10, qui reformule
  l'invariant de `CLAUDE.md` comme l'ADR 0034 le prévoit.

### AD-29 — `internal/bridge` sans `project` ni `store`

- **Doute.** `bridge` importe `project` et `store` ; `bridge.Storage` a 17 méthodes (audit
  m-x) ; le hub `present`, du routage de Streams, est dans `app`.
- **Recommandation.** Ports définis dans `bridge` (`KV`, `Projection`, `Tree`, `Blobs`,
  chacun ≤ 5 méthodes), types de transfert à lui, adaptateurs dans `app` ; hub `present`
  déplacé dans `bridge`.
- **Raison.** Interfaces petites chez le consommateur (P-2) ; `bridge` redevient testable
  sans disque.

### AD-30 — Couches du frontend

- **Doute.** Aucune règle de dépendance n'est vérifiée dans `frontend/src` ; `src/ui` pourrait
  toucher automerge-repo.
- **Options.** Biome `noRestrictedImports` (option `patterns`, Biome ≥ 2.2, déjà l'outil de
  l'ADR 0010) ; dependency-cruiser (plus riche, dépendance de plus) ;
  eslint-plugin-boundaries (ESLint n'est pas utilisé).
- **Recommandation.** Biome, règles FE-1 et FE-2 de spec §4.5, dans `fe:check`.

### AD-31 — `protocol/bridge` dans le module public

- **Doute.** Le chantier `proto1` a mis l'enveloppe du pont (`protocol/bridge`) dans le module
  public, alors que le protocole classe le pont **interne, hors contrat** (§1.5, annexe A).
- **Options.** (1) la garder publique, déclarée instable ; (2) la rendre à `internal/bridge`,
  vecteurs dans `internal/bridge/testdata` lus par Go et Vitest comme aujourd'hui ; (3) un
  paquet `internal/` du module `protocol` — impossible, le module d'inkwell ne pourrait pas
  l'importer.
- **Recommandation.** Option 2 ; côté TypeScript, les crédits du pont restent dans `src/net`.
- **Raison.** Une API publique se maintient selon SemVer (protocole §1.4.7) ; geler à la 1.0 un
  format qui ne sert qu'à inkwell n'apporte rien à un tiers. Le §1.6 du protocole est
  informatif (§1.5) : ce déplacement ne le contredit pas.

---

## Données et état

### AD-10 — Version de la disposition `.inkwell/` et migrations

- **Doute.** `project.json` n'a pas de version de disposition ; une version plus ancienne
  d'inkwell ouvrant un projet plus récent pourrait l'abîmer ; aucune règle de migration.
- **Recommandation.** Règles V-1 à V-6 de spec §6.4 : entier `layout` (absent = 1) ; plus
  récent = lecture seule avec un message ; migration = sauvegarde par liens durs, écriture dans
  `.inkwell.next/`, installation par renommage, reprise si interrompue, test d'injection de
  plantage à chaque étape ; format figé à la 1.0.0.
- **Raison.** Même mécanisme que la bascule D6 déjà prouvée ; sépare clairement la version du
  disque (ici) de la version du fil (protocole) et de celle du Stream `store` (ADR 0034
  étape 11).
- **Ferme par** ADR 0035.

### AD-11 — Où ranger les ensembles signés

- **Doute.** Les ops de membres sont dans `.inkwell/trust/` ; décisions, annonces, accusés et
  preuves n'ont pas encore de place.
- **Options.** (1) un dossier par nouvel ensemble à côté de `trust/` ; (2) un magasin générique
  `sets/<ensemble>/<hash>` avec bascule D6 par ensemble, `trust/` déplacé en
  `sets/members/`.
- **Recommandation.** Option 2, à la disposition 2 (première migration réelle de `AD-10`).
- **Raison.** Un seul code de magasin pour cinq ensembles (`SetStore[T]`), une seule
  vérification au rechargement (D-4) ; la migration est un renommage de dossier, testé par
  injection de plantage.

### AD-12 — Instance unique et verrous

- **Doute.** Deux processus inkwell peuvent ouvrir la même configuration (deux écrivains de
  `device.key`) ou le même projet (deux écrivains du magasin) ; aucun marqueur ne permet de
  détecter un arrêt brutal (conception p. 38 : « récupération après plantage »).
- **Recommandation.**
  1. Instance unique de Wails v3 (`SingleInstance`, identifiant unique, rappel
     `OnSecondInstanceLaunch`) : la seconde instance passe ses arguments à la première et sort.
     Les arguments reçus sont **non fiables** : validés comme toute entrée (chemin résolu par
     `os.Root`, taille bornée).
  2. `device.lock` : verrou consultatif (`flock` ; `LockFileEx` sous Windows) dans
     `<UserConfigDir>/inkwell/`, pris avant de lire `device.key`.
  3. `.inkwell/lock` : verrou du projet, plus un marqueur d'exécution (PeerID, nom de la
     machine, pid, heure) ; présent sans verrou tenu au démarrage = arrêt brutal → message de
     reprise (§10.3) ; tenu par une autre machine (dossier partagé) → `AD-41`.
- **Raison.** Le verrou du système protège des données ; l'instance unique ne fait que le
  confort. Le canal de l'instance unique est un socket Unix, pas un port réseau (INV-5
  intact) ; à vérifier sur la v3.0.x stable (`AD-19`).
- **Ferme par** ADR 0037.

### AD-13 — Versions nommées et projection Git

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Doute.** La conception (p. 1, 6, 9) et `CLAUDE.md` veulent des jalons go-git, projection
  jamais source de vérité ; rien n'est écrit ; l'étape 2.0 (N11) a écarté toute écriture de
  refs Git en attendant.
- **Options.**
  1. *go-git*, un commit à chaque version nommée dans `.inkwell/git/` (dépôt nu).
  2. *Écrivain Git minimal maison* : objets non compressés en paquet (`blob`, `tree`,
     `commit` : zlib + SHA-1 de la bibliothèque standard, ≈ 200 lignes), même emplacement,
     même déclencheur.
  3. *À la demande seulement* : « Exporter l'historique en Git » reconstruit le dépôt depuis
     les versions nommées.
- **Recommandation.** Déclencheur de 1 (à chaque version nommée, jamais par frappe) avec
  l'écrivain de 2, plus l'export de 3. Les commits sont **déterministes** (arbre = projection
  aux têtes de la version ; auteur = nom de la personne ; date = date de la version) : deux
  appareils produisent les mêmes identifiants, et le dossier peut être effacé puis reconstruit.
- **Raison.** On n'écrit que des objets Git, au format stable et documenté ; go-git apporte
  lecture, fusion et transports réseau dont on n'a pas l'usage, et une grosse surface de
  dépendances (ADR 0008). Si un jour il faut lire ou fusionner du Git, go-git redevient le bon
  choix.
- **Utilisatrice : oui** — la conception nomme go-git ; et : le dépôt doit-il être visible
  dans le dossier du projet (`.git/`, utile aux informaticiens, mais un éditeur externe le
  verra) ou caché (`.inkwell/git/`, recommandé) ?

- **Révision 3 (N39, revue Codex R2-6).** L'utilisatrice exige go-git et une application autoporteuse : **go-git** (Go pur, v5 ≥ 5.19 ; v6 en alpha) dans `internal/milestone`, projection reconstructible, jamais de binaire `git` (INV-12, AC-17, AC-18). L'écrivain minimal reste le **témoin** du banc P-GIT-BENCH (projection de 1 000 fichiers, reconstruction de 500 versions, taille) ; go-git est retenu sauf écart au-delà des budgets du §12.5. Dépôt caché (`.inkwell/git/`), « Exporter l'historique en dépôt Git » pour qui veut un `.git` visible. Plus de décision de l'utilisatrice à demander.

### AD-21 — Réglages de l'appareil

- **Doute.** Les préférences vivent dans le `localStorage` de la webview (`safeStorage.ts`,
  zoom, historique de recherche, choix d'enregistrement) et dans des variables `INKWELL_*`
  (`HOME`, `DOCS`, `NET`, `LAN`, `RELAYS`, `RENDEZVOUS`, `EXPORT_DIR`, `SERVER_PORT`…). Go a
  besoin des réglages réseau **avant** que la webview existe ; le `localStorage` se perd avec
  les données de WebKit et n'est pas versionné.
- **Recommandation.** `<UserConfigDir>/inkwell/settings.json`, possédé par Go (paquet
  `internal/settings`, créé à l'étape qui en a besoin), versionné (`schema`), écrit par
  `store.WriteAtomic`, lu et modifié par un service validé. Il porte aussi l'état de reprise
  (fenêtres, document ouvert, panneaux). Les `INKWELL_*` restent pour les tests et le
  développement, documentées.
- **Raison.** Une vérité par chose (INV-3) ; « tout ce qui se voit se désactive » exige un
  réglage lisible avant le démarrage du réseau.
- **Ferme par** ADR 0037 (nouvelle ligne du tableau §6.1).

### AD-23 — Liste de membres scellée à 1024 ops

- **Doute.** Une liste de 1024 ops est scellée (gestion gelée, édition continue, ADR 0017).
  Une équipe de vingt personnes avec plusieurs appareils et des suspensions peut l'atteindre
  en quelques années.
- **Recommandation.** Garder la borne (elle borne le coût du pli) ; alerte aux admins à 80 %
  (820 ops) ; action « Recréer le projet avec les mêmes personnes » (nouvelle genèse, mêmes
  textes, invitations rejouées) proposée dès l'alerte.
- **Raison.** Relever la borne repousse le problème et augmente le coût de chaque pli ; une
  recréation guidée est un geste rare et compréhensible.

### AD-24 — Valeur du quota disque N10

- **Doute.** L'utilisatrice a fermé le résiduel R2 par un quota disque cumulé par personne qui
  écrit (N10) ; aucune valeur n'est fixée. D'ici là, seule l'alerte de 256 Mio par jour (D-3)
  s'applique.
- **Recommandation.** Par personne qui écrit et par projet : **1 Gio** d'historique stocké
  (modifications attribuées, attentes indivisibles, entrées d'index), plus 1 Gio par décision
  `quota/<clé>` d'un admin ; les blobs comptent pour la personne qui les a ajoutés (attribuée
  par la modification signée de l'index). À 80 % : carte aux admins ; au-delà : modifications
  de cette personne refusées par la règle H, sans gêner les autres.
- **Raison.** Un projet visé fait moins de 2 Go (borne de confirmation, ADR 0023) ; un texte
  de 2 Mi caractères avec un historique de plusieurs années pèse quelques dizaines de Mio ;
  1 Gio laisse une marge de deux ordres de grandeur à une personne honnête.
- **Utilisatrice : oui** (valeur).

### AD-32 — Couleur et nom d'une personne

- **Doute.** Le design (Q4) propose que chacune choisisse sa couleur ; la conception (p. 14)
  veut 8 teintes Okabe-Ito et jamais deux présentes identiques. Le nom est fixé dans l'op
  `Add` par l'admin (protocole §4.1) ; aucune op ne le change.
- **Options pour la couleur.** (1) dérivée du PeerID de la personne, conflits résolus
  localement ; (2) choisie et portée par la **présence** (éphémère), gardée par chaque appareil
  comme vue (cache dans `settings.json`) ; (3) choisie et stockée dans une op signée ou dans le
  document racine — nouvelle vérité, et les lecteurs n'écrivent pas.
- **Recommandation.** Option 2, repli sur l'option 1 tant qu'aucune présence n'a été vue ;
  conflit entre deux présentes : la plus ancienne dans la liste garde sa couleur. Nom : fixé à
  l'admission en v1 ; un renommage par soi-même exige une extension du protocole (v2).
- **Raison.** Aucune nouvelle vérité (§6.1) ; la couleur sert à reconnaître une personne
  présente, la présence est le bon canal.
- **Utilisatrice : oui** (design Q4).

### AD-38 — Archive de projet autonome

- **Doute.** La conception (p. 6, 37) veut « Exporter le projet » en archive autonome
  (sources, blobs, paquets, polices), sans format fixé.
- **Recommandation.** Zip déterministe (`archive/zip`, entrées triées, date fixe) de la
  projection, de `.typstdeps/` et des polices utilisées, compilable par `typst compile` seul ;
  **sans** `.inkwell/` (l'historique peut contenir du texte supprimé exprès). Une sauvegarde
  complète, historique compris, est un autre geste, explicite.
- **Raison.** L'archive sert à envoyer à quelqu'un sans l'application : elle ne doit rien
  divulguer au-delà de ce qui est visible.

### AD-41 — Projet sur une clé USB, un disque réseau ou un dossier synchronisé

- **Doute.** Sur exFAT, SMB, NFS ou dans un dossier Dropbox, Syncthing ou OneDrive : pas de
  liens durs (sauvegardes), `flock` sans effet entre machines, renommage parfois non atomique,
  fsnotify peu fiable ; un service de synchronisation peut réécrire `.inkwell/` sous inkwell.
- **Recommandation.** À l'ouverture, sonder le système de fichiers (type, liens durs,
  verrou, renommage) ; si une garantie manque : bandeau « Ce dossier ne permet pas de garantir
  vos données. Déplacer le projet ? » et, pour un dossier synchronisé détecté (verrou tenu par
  un autre appareil), lecture seule. Sauvegardes : copie complète quand les liens durs
  manquent.
- **Raison.** I1–I5 (ADR 0029) supposent un disque local ; mieux vaut le dire que perdre en
  silence. Inkwell synchronise lui-même : un second synchroniseur ne peut que nuire.

---

## Réseau

### AD-43 — Relais par défaut

- **Doute.** Sans relais, deux personnes derrière des NAT ne se voient que si le perçage
  réussit ; or DCUtR a besoin d'un relais pour se coordonner. Mesure à grande échelle :
  70 % ± 7,1 % de réussite du perçage **une fois** la réservation de relais obtenue
  (Trautwein et al., 2025). Le code lit des relais configurés (`INKWELL_RELAYS`).
- **Options.**
  1. Aucun relais tenu par le projet : LAN, direct, et relais tenu par une personne du groupe
     (`inkwell-relay`, une commande).
  2. Un relais communautaire tenu par le projet, pour la coordination et en dernier recours.
  3. Relais publics de libp2p (IPFS) : limités par défaut à des circuits courts et petits,
     bons pour coordonner le perçage, pas pour synchroniser ; politique d'usage incertaine.
- **Recommandation.** Option 1 en v1, avec un parcours « Par Internet » qui explique comment
  ajouter le relais d'une personne du groupe ; l'option 2 seulement si l'utilisatrice accepte
  de tenir un service (coût, disponibilité, métadonnées visibles, protocole §18).
- **Raison.** G2 (« aucun serveur ») ; le relais ne voit que du chiffré mais voit qui parle à
  qui. Le serveur Wormhole public est déjà un tiers par défaut : un relais du projet serait un
  second tiers, à assumer explicitement.
- **Utilisatrice : oui.**

### AD-09 — « Rotation de la clé de session » (ADR 0005, point 5)

- **Doute.** L'ADR 0005 prévoit une rotation de clé de session au retrait d'un membre. Il
  n'existe aucune clé de groupe : chaque connexion a ses clés éphémères (TLS 1.3, Noise).
- **Recommandation.** Noter « sans objet » : un retrait prend effet par le filtre de
  connexion, la fin de la couverture et des attestations (spec §9.4). Renvoi dans l'ADR 0036
  (une ADR acceptée ne se réécrit pas).

### AD-15 — Journal circulaire réseau

- **Doute.** La conception (p. 15, 38) veut un journal circulaire des 1 000 derniers
  messages et « la trace d'une graine en échec » comme rapport de bug ; un message contient du
  texte.
- **Recommandation.** Métadonnées seulement (heure, pseudonyme du pair, classe, type, taille,
  génération, verdict) ; le rejeu en simulation utilise tailles et horaires, pas le contenu.
- **Raison.** Règle « jamais de contenu de document » (conception p. 38) ; un journal qui ne
  contient rien de sensible peut être joint à un rapport sans relecture.

---

## Sécurité et confidentialité

### AD-14 — Commentaires des lecteurs

- **Doute.** La conception (p. 11–12) : les commentaires entrent en v1, dans le document
  Automerge ; « les pairs ignorent les updates de texte d'un membre en lecture, mais acceptent
  ses commentaires ». Les ADR 0018 et 0031 refusent toute modification non signée par une
  personne qui écrit : un lecteur ne peut pas commenter.
- **Options.**
  1. v1 sans commentaire des lecteurs (donner le rôle écriture à qui doit commenter).
  2. Admission bornée : la modification d'un lecteur est admise si elle ne touche que
     `meta.comments` du document, signée par l'appareil du lecteur (domaine de signature
     distinct), vérifiée par la garde structurelle — comme les zones verrouillées de
     l'ADR 0022, qui admettent déjà selon le chemin touché ; volume borné par le quota N10.
  3. Un document de commentaires par lecteur, dont il serait l'unique auteur : il faut des
     annonces de lecteurs (ADR 0031 les réserve aux écrivains), plus de documents, et la
     fusion de fils de discussion entre documents.
  4. Un rôle « commentaire » entre lecture et écriture : même mécanisme que 2, un rôle de
     plus dans la liste.
- **Recommandation.** Option 2, en ADR avant l'étape 4.
- **Raison.** Fidèle à la conception (relecture par des personnes qui ne doivent pas
  modifier le texte) ; réutilise la garde et le chemin d'admission existants ; pas de nouvelle
  vérité. Le coût : une extension du protocole (domaine `inkwell/comment/v1`, règle de garde),
  avec vecteurs et fuzz.
- **Utilisatrice : oui** (l'option 1 est un écart à la conception ; l'option 2 étend le
  protocole).

- **Révision 2.** L'option 2 n'est **pas** compatible avec le protocole accepté : la couverture (protocole §9.4) exige qu'une modification admise soit couverte par un écrivain. Elle exige une **mineure du protocole** (nouveau domaine de signature, règle de couverture des commentaires, vecteurs) et une ADR ; sans elles, seule l'option 1 est possible. Inscrit dans la matrice.

### AD-20 — LSP tinymist

- **Doute.** `tinymist` (32,8 Mo) est embarqué mais jamais lancé (audit m-x) ; la conception
  veut complétion, survol et diagnostics à la frappe. tinymist sait aussi exécuter des
  commandes (export de fichiers, serveur d'aperçu sur un port).
- **Recommandation.** `tinymist lsp` sur stdio, relayé par un Stream `lsp` ; cadrage JSON-RPC
  borné en Go (en-tête ≤ 1 Kio, corps ≤ 8 Mio) ; **liste blanche** des méthodes que la webview
  peut appeler (complétion, survol, aide de signature, symboles, définition, diagnostics,
  formatage) ; `workspace/executeCommand` refusé sauf commandes listées ; le fichier ouvert est
  envoyé par `didChange` depuis la webview, les autres lus sur la projection. Enfermement
  §9.5 (lecture seule, pas de réseau, mémoire bornée), relance bornée.
- **Raison.** Aucun port (ADR 0009) ; un défaut de la webview ne donne pas à un pair le
  pouvoir de faire écrire tinymist (B-2).

- **Révision 2.** Remplacée par `AD-80` (N29, spec §15.17), qui en garde l'enfermement et la liste blanche et ajoute versions, espace de travail multi-fichiers et budget.

### AD-22 — Étiquettes de test dans l'exécutable publié

- **Doute.** `-tags server` (e2e sans fenêtre) écoute sur `127.0.0.1` ; `faultinject` injecte
  des pannes. Rien n'empêche aujourd'hui de les publier.
- **Recommandation.** Porte AC-14 dans le job de publication : `go version -m` sur chaque
  exécutable, échec si `-tags` contient `server` ou `faultinject`.
- **Raison.** INV-5 : en production, aucun serveur local. Après `AD-20` (stdio), l'exécutable
  publié n'en a aucun.

### AD-28 — Lecture des sidecars bornée

- **Doute.** Typst refuse de lire hors de `--root` ; c'est sa propre règle. Rien, côté noyau,
  n'empêche `typst` ou `tinymist` de lire `~/.ssh` si un défaut de Typst le permettait.
- **Recommandation.** Landlock (ABI ≥ 1, noyau ≥ 5.13) : lecture limitée au projet, au
  binaire, aux paquets et polices fournis ; écriture limitée à la sortie (ADR 0033).
  Landlock absent : mode dégradé journalisé et visible dans « Détails techniques », pas
  d'échec fermé (contrairement à la borne de sortie de l'ADR 0033, ici `--root` protège déjà).
- **Raison.** Défense en profondeur peu coûteuse ; Debian 13, Ubuntu 24.04 et Fedora 44 ont
  Landlock actif.

### AD-39 — Visionneuse PDF

- **Doute.** La conception (p. 39, étape 4) veut PDF.js dans la webview. Un PDF du projet peut
  venir d'un pair (blob, pièce jointe) : c'est une entrée hostile. PDF.js a eu une exécution de
  code par police forgée (CVE-2024-4367, corrigée en 4.2.67).
- **Recommandation.** PDF.js épinglé (≥ la dernière version corrigée), `isEvalSupported:
  false`, rendu dans un Worker (`worker-src 'self'`), polices et cartes CMap fournies, aucune
  ressource distante ; taille bornée par la borne des blobs.
- **Raison.** Même règle que tout octet d'un pair : borné, isolé, sans exécution.

---

## Opérations

### AD-16 — Journaux et rapport de bug

- **Doute.** Les journaux vont sur la sortie d'erreur seulement (`cmd/inkwell/main.go:44`).
  La conception (p. 38) veut un rapport de bug « préparé localement, envoyé seulement si
  l'utilisatrice confirme, jamais de contenu de document » — mais il n'y a pas de serveur.
- **Recommandation.** Journal JSON tournant (5 × 4 Mio, 0600) écrit par un petit gestionnaire
  `slog` maison (pas de dépendance) dans le dossier d'état de la plate-forme ; règles O-1 à
  O-7. Rapport : archive **locale** (journaux expurgés, journal circulaire, version,
  plate-forme, trace éventuelle) ; « Envoyer » ouvre la page de nouveau ticket du dépôt avec un
  résumé prérempli, la personne y joint l'archive elle-même.
- **Raison.** Aucun serveur à tenir (G2), aucun envoi automatique (NG8), et la personne voit
  ce qu'elle envoie.
- **Utilisatrice : oui** (le canal d'envoi : ticket GitHub prérempli, courriel, ou rien).

### AD-17 — Mises à jour

- **Doute.** La conception (p. 15–16, 38) : vérification de mise à jour qui n'envoie que le
  numéro de version, désactivable, signatures vérifiées (elle cite `minio/selfupdate`).
  Wails v3.0.0 fournit un module de mise à jour à clé Ed25519 épinglée, qui échoue fermé ; son
  fournisseur GitHub ne lit pas de signature séparée, il faut un manifeste. Remplacer
  soi-même un fichier installé par `.deb`/`.rpm` désaccorde le gestionnaire de paquets.
- **Recommandation.**
  1. Vérification = lecture d'un manifeste statique signé (Ed25519, clé épinglée à la
     construction) publié avec chaque version ; aucun identifiant envoyé ; une fois par
     semaine ; la première 24 h après le premier lancement.
  2. `.deb`/`.rpm` : avertir seulement (« Une nouvelle version est disponible »), lien vers
     la page de la version.
  3. Archive autonome : module de mise à jour de Wails v3 avec la même clé.
  4. Réglage « Vérifier les mises à jour » dans les préférences et dans la promesse de
     confidentialité.
- **Raison.** Signature vérifiée sans dépendance de plus ; aucune corruption du système de
  paquets.
- **Utilisatrice : oui** — activée par défaut (conception : « désactivable ») ou désactivée
  par défaut (vie privée maximale) ?
- **Ferme par** ADR 0038.

### AD-18 — Plates-formes de la v1

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Doute.** L'objectif final (L2) vise `.deb`/`.rpm` amd64 et arm64 ; `PROGRESS.md`
  (étape 6) vise aussi macOS et Windows signés et notarisés. L'enfermement (espace de noms
  réseau, Landlock, tmpfs borné, `Pdeathsig`) est Linux seulement ; ADR 0033 **refuse** de
  lancer une compilation bornée ailleurs : sous macOS, pas d'aperçu. Les sidecars sont
  embarqués pour Linux et macOS, pas Windows ; la CI ne construit que Linux.
- **Options.** (1) v1 Linux seulement, macOS et Windows en v1.x une fois l'enfermement à
  parité (macOS : processus enfant sous bac à sable ; Windows : objet de tâche,
  AppContainer) ; (2) publier macOS et Windows avec un enfermement plus faible — contraire à
  l'échec fermé de l'ADR 0033 ; (3) tout en v1.
- **Recommandation.** Option 1 ; garder le code macOS compilé (`GOOS=darwin go vet` sur les
  paquets sans cgo) pour qu'il ne pourrisse pas.
- **Utilisatrice : oui.**

- **Révision 3 (N35, revues Fable R2-M1, Codex R2-6).** N35 tranche : Linux, macOS, Windows, amd64 et arm64. Port `internal/sandbox` (spec §9.11) ; tant qu'une porte P-SANDBOX-* manque sur une cible, les capacités qu'elle protège y sont **refusées** (greffons tiers, ACP, aperçu), jamais affaiblies (INV-13) ; une version Linux seule est une alpha annoncée.

### AD-19 — Wails beta.34 vers v3.0.x

- **Doute.** `go.mod` épingle `v3.0.0-beta.34` ; la v3.0.0 stable est publiée
  (2026-10-26).
- **Recommandation.** Lot dédié avant l'étape 6, procédure de l'ADR 0008 : lire le journal des
  changements (Streams, instance unique, mise à jour), e2e complets, `pkg:test`, puis épingler
  v3.0.x.

### AD-33 — Import et export DOCX, RTF, LaTeX

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Doute.** La conception veut RTF et DOCX « en import/export seulement » (p. 1) et, à
  l'étape 6, l'import Markdown, DOCX et LaTeX par pandoc (p. 37) ; « un jour pandoc » parmi les
  sidecars (p. 13). Pandoc lit Typst depuis 3.1 (3.10.1 en juillet 2026).
- **Options.** (1) pandoc en troisième sidecar, épinglé, embarqué, vérifié et enfermé comme
  typst (ADR 0024), au prix de plusieurs dizaines de Mo de plus dans l'exécutable ; (2) utiliser
  le pandoc du système s'il existe — version non épinglée, contraire à l'esprit d'INV-6 ;
  (3) report en v2 ; la v1 exporte PDF (et HTML par Typst, encore expérimental).
- **Recommandation.** Option 1 à l'étape 6 si la taille est acceptée ; sinon option 3.
- **Utilisatrice : oui** (taille de l'exécutable contre fonction).

- **Révision 3 (N40).** Tranché : **aucune** intégration native des formats bureautiques, ni import ni export, **aucun pandoc** livré ; un greffon (`importers`, `exporters`) peut l'offrir. Le collage riche (HTML → Typst ou Markdown) reste dans le cœur.

### AD-34 — Aperçu partagé pour un invité sans l'application (v2)

- **Constat.** Prévu en v2 (conception p. 39) : tinymist servirait l'aperçu sur le LAN. Ce
  serait le premier serveur hors de la boucle locale : il exigera sa propre ADR (jeton dans le
  lien, lecture seule, expiration à la fermeture, LAN ou relais seulement). Rien en v1.

### AD-35 — Compagnon en ligne de commande (v2)

- **Constat.** `inkwell export <projet> --pdf` n'a besoin que de la projection et de typst :
  possible en v1.x sans toucher aux invariants. `inkwell join <code> --headless` exige
  Automerge hors de la webview (INV-1) : moteur JavaScript sans fenêtre ou WebKit sans
  affichage ; décision par ADR en v2. Préparation dès maintenant : services Wails minces
  (B-6), cas d'usage dans `app` indépendants de Wails.

### AD-40 — Sous-modules, `HOT_PATHS` et portes

- **Doute.** Le module `protocol` (et bientôt `protocol/libp2p`) a son propre `go.mod` ;
  `go test ./...` depuis la racine **n'entre pas** dans un sous-module : tests, fuzz,
  couverture, lint et bancs du protocole ne tourneraient pas dans `mise run ci`. La porte P-PERF
  de l'ADR 0034 nomme `protocol/wire`, qui n'existe pas (le paquet est `protocol/sync2`).
- **Recommandation.** Les tâches `test`, `cover`, `lint`, `bench*`, `mutate:go` et `fuzz`
  parcourent les trois modules ; `HOT_PATHS:` accepte des chemins de sous-module ; la
  publication vérifie que la directive `replace` pointe vers `./protocol` et que l'étiquette
  `protocol/vX.Y.Z` correspond à la version requise. Erratum : lire `protocol/sync2` dans
  P-PERF (note dans `docs/protocol/CHANGES.md`).

### AD-45 — Tests intermittents

- **Doute.** F3 (plantage de libp2p à la fermeture, 1 fois sur 6), F6 (`sim_test.go:455`),
  F8 (`TestOpenWindow`), F9 (`TestPackageCache`, course possible dans le cache de paquets),
  PRS-37 : tous ouverts.
- **Recommandation.** D14 : taux mesuré (500 exécutions sous `scripts/heavy.sh`), cause
  prouvée, correctif, puis 500 succès de suite ; F3 par l'ordre de fermeture (D13). Tous clos
  avant la porte P-* de l'étape 9 d'ADR 0034 : la bascule touche exactement `p2p` et la
  session.

---

## Performance et qualité

### AD-25 — « Durable avant diffusé » sans bloquer l'acteur

- **Doute.** Aujourd'hui l'acteur `Wiring` écrit sur le disque dans sa propre goroutine ; un
  `fsync` lent (disque réseau, disque plein) fige toute l'autorité.
- **Recommandation.** Un travailleur de persistance par magasin, file bornée ; l'acteur dépose
  l'objet signé, reçoit l'acquittement durable **par sa boîte** (jamais par rappel réentrant,
  C-5), puis seulement l'émet ; disque plein : l'objet n'est pas émis, bandeau, reprise avec
  recul (I5).
- **Raison.** Protocole §14.2 et C-3/C-6 de la spec ; c'est l'étape 5 d'ADR 0034.

### AD-26 — Aperçu sous 300 ms

- **Doute.** `engineering-principles.md` §3 et la conception (p. 37 : « tinymist résident,
  aperçu incrémental ») visent 300 ms après la dernière frappe ; le code recompile tout par
  `typst compile` (environ 1 s). La spec propose ≤ 1 s aujourd'hui et ≤ 300 ms avec
  l'incrémental.
- **Options.**
  1. *tinymist preview* : serveur WebSocket sur un port local, moteur de rendu à lui (« au
     mieux » identique à typst) — un port (ADR 0009) et un aperçu qui peut différer de
     l'export.
  2. *tinymist par stdio* (`AD-20`) : compilateur incrémental en mémoire, texte du fichier
     ouvert reçu par `didChange`, pages SVG demandées par commande.
  3. *`typst watch` résident* : compilateur officiel, mémoïsation entre compilations, pages
     SVG écrites dans la sortie bornée ; lit la projection, donc ajoute son anti-rebond.
  4. Garder la recompilation complète.
- **Recommandation.** Essai chronométré de 2 et 3 (cas `PERF-*`, documents de 1, 20 et
  100 pages) avant de choisir ; préférence à 3 si elle tient le budget (rendu identique à
  l'export, enfermement déjà prouvé), avec un anti-rebond de projection réduit pour le fichier
  ouvert ; sinon 2. Option 1 écartée. Corriger `engineering-principles.md` : ≤ 300 ms p95 en
  incrémental pour ≤ 20 pages, ≤ 1 s en compilation complète.
- **Raison.** Pas de port, pas de second moteur de rendu ; décider sur mesure, pas au jugé.

- **Révision 2.** Le budget de 300 ms est écrit dans `engineering-principles.md` §3 : la spécification ne peut pas l'assouplir seule. Tant que la mesure n'a pas tranché, le principe reste la cible ; si l'essai montre qu'il est inatteignable en compilation complète, une **ADR amende le principe** (matrice).

### AD-27 — Condition de mesure « frappe → pair »

- **Doute.** Le budget LAN (p95 ≤ 100 ms) se heurte à la cadence de synchronisation
  d'automerge-repo (un message toutes les 100 ms au plus en rafale, ADR 0028 D7).
- **Recommandation.** Mesure normative avec une frappe régulière de 5 caractères par seconde
  (ADR 0029 §V) ; une rafale continue peut ajouter la cadence (spec §12.3).

### AD-42 — Recherche dans tout le projet

- **Doute.** U14 : recherche normale, joker, expression régulière, dans tout le projet. Le
  moteur actuel est en TypeScript sur les documents chargés ; avec 10 000 documents et le
  déchargement des documents invisibles (PF-3), la webview ne les a pas tous.
- **Recommandation.** Deux sources : la webview cherche les documents ouverts (texte à jour à
  la frappe près) ; Go cherche la projection sur le disque pour les autres (flux de résultats
  bornés, `regexp` RE2 en temps linéaire, annulable). Fusion par chemin, le document ouvert
  l'emporte.
- **Raison.** Lire la projection n'est pas lire Automerge (INV-1 intact) ; RE2 ne peut pas
  boucler sur une expression pathologique ; aucune mémoire de 10 000 documents dans la
  webview.

### AD-44 — Accessibilité de l'aperçu et du PDF

- **Doute.** L'aperçu est un SVG affiché en `<img>` (B-7, sécurité) : un lecteur d'écran n'en
  lit rien.
- **Recommandation.** `alt` « Page N sur M » ; le chemin accessible est l'éditeur de source
  (CodeMirror est accessible au clavier et au lecteur d'écran) et le PDF exporté, balisé par
  défaut depuis Typst 0.14 ; option d'export PDF/UA-1 (`--pdf-standard ua-1`) dans la boîte
  d'export.
- **Raison.** Garder l'aperçu inerte (aucun balisage venu d'un pair dans le DOM) sans
  sacrifier WCAG 2.2 AA.

- **Révision 2 (revue Codex M7).** `alt="Page N sur M"` n'est pas un équivalent (WCAG 2.2 SC 1.1.1). Recommandation remplacée : « Vue lecture » de l'aperçu, produite par l'export HTML de Typst (≥ 0.15, MathML), **assainie** par Go, affichée dans un `iframe` `sandbox` sans scripts, navigable au clavier et au lecteur d'écran, synchronisée avec la page ; descriptions des diagrammes (contrat `renderers`) ; parcours vérifié avec Orca, pas seulement axe (spec §15.17).

### AD-36 — Test ignoré `files_test.go:713`

- **Doute.** Un `t.Skip("TODO ADR 0021 … Wiring.isActive must say false for a suspended
  member")` : la règle existe (N3, Go fusionné), son test non.
- **Recommandation.** Écrire le test maintenant ; un `t.Skip` sans condition d'environnement
  est interdit (un test prouve une règle, §13).

---

## Gouvernance des documents

### AD-07 — Statuts d'ADR incohérents

- **Doute.** L'index `docs/adr/README.md` marque 0030, 0033 et 0034 « proposée » alors que
  leurs fichiers sont acceptés ; 0031 et 0032 n'y ont pas de statut. 0021, 0022, 0023, 0026,
  0028 et 0029 sont « proposed » mais en partie livrées (suspension en Go, paliers de taille,
  tests de pannes) ; l'ADR 0031, acceptée, exige le Worker de l'ADR 0028, proposée. L'ADR 0026
  garde un seuil de 10 Mo que l'ADR 0028 D1 remplace par des paliers.
- **Recommandation.** Corriger l'index sur les fichiers ; soumettre à l'utilisatrice
  l'acceptation de 0021, 0022, 0023, 0028, 0029, et de 0026 avec la mention « seuil remplacé
  par ADR 0028 D1 ».
- **Utilisatrice : oui** (acceptation).

### AD-08 — ADR 0027 absente

- **Doute.** Le registre des types de fichiers (U26) est cité comme ADR 0027 par
  `exigences.md`, le plan T11 et la spec ; le fichier n'existe pas (0014–0016 et 0019 manquent
  aussi, sans renvoi).
- **Recommandation.** Rédiger l'ADR 0027 comme **registre unique des contributions** (types de fichiers, commandes, menus, panneaux, exportateurs ; spec §15.2 X-2), dont les types de fichiers sont un point : un module par type (extensions,
  type MIME, icône, éditeur chargé à la demande, syntaxe des commentaires, palier, barre
  d'outils, sorte d'aperçu) ; aucun `switch` sur l'extension ailleurs ; Go ne dépend pas du
  registre (l'index dit déjà texte ou blob). Noter dans l'index que 0014–0016 et 0019 n'ont
  jamais été attribués.

### AD-37 — Questions ouvertes du 2026-10-25

| Question (`open-questions.md`) | État | Où |
|---|---|---|
| Nom du dossier `.inkwell/` | tranchée | ADR 0020, code |
| Admin distinct de l'hôte initial | tranchée (la créatrice transmet, l'interface pousse un second admin) | ADR 0017 |
| Résumés automatiques ; Tiptap | v2 | NG10 |
| Taille d'un blob et d'un projet | tranchée : 100 Mo, 2 Go puis confirmation | ADR 0023 |
| Cloisonner les compilations | sortie : ADR 0033 ; lecture : `AD-28` | — |
| Arrêt des sidecars sous macOS et Windows | hors v1 | `AD-18` |
| Thèmes Touying | produit, pas architecture | reste ouverte (C4) |

Recommandation : réduire `open-questions.md` à la dernière ligne et à un renvoi ici.

---

## Projet multi-fichiers et types de fichiers (spec §6.6)

Constat commun (preuve) : `App.svelte:197` passe `onOpen={() => session.focus()}` à la liste
des fichiers, le chemin est jeté ; `workspace.svelte.ts:30` (`Doc = {text, meta}`), `:42`
(`MAIN_FILE`), `:247` (`open(url)` ouvre un projet), `:270–310` (projection et
synchronisation externe sur `main.typ`) ; `store/tree.ts` et `bridge/tree.go` (opération
`Tree`) existent mais rien ne matérialise l'index en TypeScript ; `preview.go:85,179` fixe le
principal au démarrage. Codex a refusé un correctif étroit : le modèle manque.

### AD-46 — Un document par fichier, un index séparé

- **Options.** (1) un document Automerge par projet (tout le texte dans une carte) ; (2) un par
  fichier texte plus un index (ADR 0023) ; (3) un par fichier sans index (arbre déduit du
  disque).
- **Recommandation.** Option 2, avec l'index `G` dérivé (ADR 0030 §3), distinct de la racine ;
  tout désigne un **nœud**, jamais un chemin.
- **Norme et preuve.** automerge-repo : le document est l'unité de synchronisation, de partage
  et de chargement (une URL par document) ; ADR 0023 (nœuds stables, renommage sans
  interruption, PRJ-05) ; ADR 0030 §3 et protocole §4.10 (index déterministe, acceptés). L'option
  1 charge tout le projet pour une frappe et rend la porte de taille globale ; l'option 3 perd les
  renommages concurrents (ADR 0023, option I1 écartée). Écart à corriger : l'ADR 0023 (proposée)
  disait « index = racine », l'ADR 0030 (acceptée) l'a remplacé.

### AD-47 — Documents ouverts, onglets, document actif

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Options.** (1) un document actif et une liste de récents, retour arrière (Alt+←), comme
  Overleaf et typst.app ; (2) onglets (barre d'onglets d'Adwaita, comme l'Éditeur de texte de
  GNOME, VS Code) ; (3) vues divisées.
- **Recommandation.** Option 1 en v1 ; le modèle (ouverts ≠ chargés ≠ actif, MF-5 à MF-7) permet
  les onglets sans refonte. Le design (H-1 : « le sous-titre est le fichier ouvert ») suppose déjà
  un seul fichier visible.
- **Norme et preuve.** GNOME HIG : les onglets servent les applications à documents multiples ;
  ux-principles (« calme », peu de chrome) ; présence par nœud (U16).
- **Utilisatrice : oui** (onglets ou non).

- **Révision 3 (N19, audit de cohérence C20).** N19 demande les onglets : **retenus**. Ouverts ≠ chargés ≠ actif (MF-5 à MF-7, MF-19) ; un seul éditeur visible ; barre d'onglets au lot G-0c, après le correctif G-0a.

### AD-48 — Fichier principal et fichier édité

- **Recommandation.** MF-8 à MF-10 : l'aperçu et l'export compilent toujours le principal
  (décision d'admin, ADR 0030 ; repli : nœud de la genèse) ; « Aperçu de ce fichier » à part ;
  saut source ↔ aperçu entre fichiers.
- **Norme et preuve.** Modèle de projet de Typst : une racine (`--root`) et **un** point
  d'entrée ; typst.app et tinymist (« épingler le principal ») font de même ; ADR 0030 §1 range le
  fichier principal parmi les décisions d'admin ; `tree.ts` porte déjà le drapeau « principal ».

### AD-49 — Enregistrement par fichier

- **Recommandation.** MF-11 à MF-13 : persistance Automerge de tout document ; projection de
  **tous** les textes admis (pas seulement l'actif) ; mode d'enregistrement par appareil, état
  « non enregistré » par fichier ; Ctrl+S enregistre tout ; aucune question bloquante à la
  fermeture.
- **Norme et preuve.** ADR 0020 (règle de propriété), ADR 0029 I1 (≤ 200 ms perdues, quel que soit
  le mode) ; SAV-03 ; aujourd'hui, un fichier modifié par un pair et non ouvert ne serait jamais
  écrit sur le disque (projection limitée à `main.typ`).

### AD-50 — Renommer ou supprimer un fichier ouvert ailleurs

- **Recommandation.** Tableau du §6.6 : l'éditeur suit le nœud ; corbeille = bandeau et
  « Restaurer », la frappe continue ; purge = lecture seule et « Enregistrer une copie ».
- **Norme et preuve.** ADR 0023 (URL d'un nœud texte stable, « supprimer » pose `trashed`, le
  document vit) ; registre Automerge pour les renommages concurrents ; FM-18 (le principal ne va
  jamais à la corbeille).

### AD-51 — Registre des types de fichiers

- **Recommandation.** Une table de **données** (`filetypes.json`) embarquée par Go et importée
  par TypeScript, plus des modules d'éditeur chargés à la demande ; vecteurs partagés ; aucun
  `switch` ailleurs ; les types du cœur s'y inscrivent comme ceux d'un greffon (`AD-63`).
- **Norme et preuve.** Types IANA (`text/csv` RFC 4180, `text/markdown` RFC 7763,
  `text/tab-separated-values`), `shared-mime-info` de freedesktop pour l'intégration au bureau
  (`text/x-typst`, ADR 0025) ; U25, U26, N20. Go doit classer les fichiers ajoutés hors de
  l'application (surveillance) : une table partagée évite deux listes.

### AD-52 — Tableaux CSV : texte ou modèle structuré

- **Constat.** ADR 0026 (proposée, livrée en B9) : texte Automerge, grille virtualisée, cellules
  entre guillemets quand d'autres sont présents. Qualité de fusion : deux cellules différentes
  d'une même ligne → bon ; même cellule → texte concaténé (`a"b` après guillemets
  systématiques) ; **ligne supprimée pendant qu'une autre personne y écrit** → les caractères
  insérés survivent et se collent à la ligne voisine (ligne abîmée) ; insertion de colonne
  concurrente à un ajout de ligne → ligne courte.
- **Options.** (1) garder le texte, détecter et signaler les lignes abîmées après une fusion
  distante (nombre de champs différent de l'en-tête, carte « Ligne fusionnée de façon
  inattendue » avec les deux contributions de l'historique) ; (2) modèle structuré à la
  `SharedMatrix` de Fluid Framework : listes d'identifiants de lignes et de colonnes, cellules en
  registre (dernier écrivain gagne), projection en CSV — c'est la forme de l'industrie pour les
  tableurs collaboratifs ; (3) hybride.
- **Recommandation.** Option 1 en v1 (déjà livrée, mesurée, une seule chaîne de format, Typst lit
  les mêmes octets, synchronisation externe identique au texte) ; **essai mesuré de l'option 2 à
  l'étape 4** avec critères : taux de lignes abîmées dans des tests de concurrence à graine
  (CSV-MU), mémoire par cellule, ouverture d'un tableau de 2 Mi caractères ; bascule par ADR si
  l'option 2 gagne.
- **Utilisatrice : oui**, avec l'acceptation de l'ADR 0026 (`AD-07`).

- **Révision 2 (revue Codex M3).** Compter les champs ne voit pas une valeur déplacée dans une ligne voisine de même largeur. Décidé maintenant : détection **au niveau des opérations** (lignes touchées localement × plages supprimées ou déplacées par des modifications concurrentes), carte de réparation attribuée, texte orphelin retiré dans la même proposition (spec MF-25) ; **oracle** : propriété à graine contre un modèle à identités de lignes, qui remplace l'assertion `record(r).length > 0` de `collab.test.ts:217–220`. L'essai du modèle structuré reste à l'étape 4 ; il ne conditionne plus la sûreté actuelle.

### AD-53 — Bornes de taille par type

- **Recommandation.** Paliers de l'ADR 0028 (comptés en caractères) pour texte, code, Markdown,
  Typst et tableaux ; grille en lecture seule sur un index des débuts de ligne (ADR 0026), jamais
  un objet par cellule ; coloration désactivée au-delà de 2 Mi ; image décodée seulement si l'en-
  tête annonce ≤ 100 mégapixels (bombe de décompression) ; binaires ≤ 100 Mo (ADR 0023).
- **Correction.** spec §12.2 disait « Grand … 512 Ki tant que le Worker n'est pas là » ; le code
  et U34 gardent 0,5 à 2 Mi modifiables avec bandeau : la spec est corrigée.

### AD-54 — Grammaires de coloration

- **Options.** (1) CodeMirror et Lezer, `LanguageDescription` à chargement par `import()` ;
  (2) tree-sitter en WebAssembly ; (3) Shiki (grammaires TextMate et moteur d'expressions
  régulières en WebAssembly).
- **Recommandation.** Option 1, liste fermée (Go, Python, JavaScript, TypeScript, JSON, YAML,
  TOML, HTML, CSS, SQL, Rust, C, C++, Java, shell, Dockerfile, XML, LaTeX, diff, INI, Makefile),
  modules découpés par Vite, **embarqués** dans `frontend/dist` (aucun téléchargement, comme
  NG6), chargés au premier fichier du langage ; langages de greffons par le point `tokens`
  (plages calculées hors du fil d'affichage, §15.4) ; blocs de code imbriqués de Markdown et Typst
  colorés par le même registre.
- **Norme et preuve.** Analyse incrémentale et budget de travail de Lezer (pensés pour
  l'édition) ; tree-sitter et Shiki n'ont pas d'intégration incrémentale standard à CodeMirror 6 ;
  aujourd'hui `package.json` n'a que `lang-markdown` et `codemirror-lang-typst`.

### AD-55 — Binaire ou inconnu

- **Recommandation.** Extension connue → registre ; sinon reniflage des 8 premiers Kio : octet
  binaire au sens du *MIME Sniffing Standard* du WHATWG → blob (carte : nom, taille, qui, quand,
  « Afficher dans le dossier », jamais lancé) ; sinon texte brut (encodage et fin de ligne
  gardés, U1). Même règle en Go et en TypeScript, vecteurs partagés.
- **Norme et preuve.** WHATWG MIME Sniffing (§ « binary data byte ») ; heuristique de Git (octet
  nul dans les 8 000 premiers octets).

### AD-56 — Un seul point d'entrée « ouvrir »

- **Recommandation.** `open(nodeId)` → registre → vue du type ; suppression du paramètre
  `?file=` de développement (`workspace.svelte.ts:44–58`), remplacé par l'ouverture d'un nœud
  dans les tests e2e.

---

## Extensibilité et greffons (spec §15, annexe)

Étude : [research/plugins.md](research/plugins.md). Les affirmations « [mémoire] » de l'étude ne
servent pas de preuve ; celles dont dépendent les décisions ont été vérifiées (annexe de la spec,
« Vérifié le 2026-11-01 »).

### AD-57 — Où tournent les greffons

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Options.** (1) composants par wasmtime-go ; (2) composants par jco dans la webview ;
  (3) wazero dans le processus d'inkwell (recommandation de l'étude) ; (4) wazero dans un
  **processus enfant enfermé** ; (5) Extism.
- **Recommandation.** Option 4, ABI minimale, WIT comme schéma, composants en API 2.
- **Raison.** Comme l'étude : Go pur, arm64, aucun cgo de plus, mode sans fenêtre possible. En
  plus de l'étude : une évasion du moteur dans le processus principal donnerait `device.key` ;
  l'enfant enfermé (même enfermement que les sidecars) ne tient ni fichier, ni réseau, ni secret.
  L'option 2 mettrait du code tiers dans l'origine qui tient le pont et la demande de signature.
  wasmtime-go n'a pas publié son API de composants (tickets #280, #290, #291). Extism : dernière
  version du SDK Go le 2025-03-19.
- **Preuve exigée.** P-PLUG-BENCH (`benchstat`, `-count ≥ 10`) avant l'acceptation.
- **Utilisatrice : oui** (ADR 0039).

- **Révision 2 (revues Fable B-3, M-1 ; Codex B4, M5).** Les deux revues approuvent l'option 4. Précisé : enfermement exécutable (spec §9.10 : modules par `memfd` scellé, **pas de cache disque en v1**, seccomp et Landlock obligatoires, profil AppArmor livré pour Ubuntu 24.04, budget mémoire par hôte, échec fermé visible) ; **compartiments** (un hôte par projet, un pour les greffons globaux sans sortie, un par greffon qui a une sortie) : une évasion n'obtient que les droits de son compartiment ; canal et autorité imposés par le parent (§7.3) ; banc P-PLUG-BENCH avec **témoin en processus**, trajet complet, RSS par hôte.

- **Révision 3 (revues Fable R2-B1, R2-M2 ; Codex R2-1, R2-2, R2-10).** (1) **Un hôte par greffon actif** : seule autorité qu'un parent peut attacher à une connexion après une évasion (Codex) ; paresseux et **≤ 6 actifs** (borne de Fable) ; (2) hôte = binaire **statique** Go pur `inkwell-plugin-host`, démarrage en deux temps (Landlock posé sur un fil puis `execve`, seccomp avec `TSYNC`, vérification, `ready`) ; seccomp est la barrière principale, Landlock la défense en profondeur ; liste seccomp dérivée par `strace` en CI, `clone` filtré, `clone3` refusé ; (3) canal en **flux d'octets**, autorité attachée à la connexion ; (4) **statut d'expérimentation** (spec §0.5, `AD-95`) : l'acceptation de la spec ne vaut pas celle de l'hôte tant que P-PLUG-PROTO, P-PLUG-BENCH, `PLUG-COMPROMISED-*` et P-SANDBOX-* ne sont pas passées, résultats bruts à l'appui.

- **Révision 4 (D-10 convergé ; Fable R3).** Les deux revues retiennent **un hôte par greffon actif**. Précisions : l'éviction ne touche jamais un hôte qui a un mandat ou un appel en vol ; **repli nommé** si l'activation dépasse 500 ms : module compilé à l'installation et gardé dans l'hôte tant qu'il vit, aucune éviction automatique sous 6 actifs, jamais de cache disque ; P-PLUG-BENCH éprouve 16 greffons installés, 6 actifs, 10 en attente.

- **Révision 5 (Fable R4-M1).** Le repli « compilé à l'installation » n'existait pas sous H-3 (wazero ne garde le code natif qu'en mémoire d'un processus ou dans un dossier de cache). Repli réel : (a) **préchauffage** en tâche de fond après la première peinture pour les greffons épinglés ou actifs au dernier usage, dans la limite des 6 actifs, activation froide affichée « Extension en préparation… » ; (b) sinon **revue de H-3 par ADR** (cache disque scellé, haché par le parent, dans le coffre pour un projet protégé).

- **Révision 6 (Fable R5-M2).** H-4 dit maintenant « jamais avant la première peinture » : le préchauffage est permis après « dernier projet modifiable », à priorité basse, pour les greffons épinglés ou actifs au dernier usage, seulement si le repli (a) est engagé ; `PERF-START` se mesure avec six greffons épinglés.

### AD-58 — Droits

- **Recommandation.** §15.5 : aucune capacité ambiante, capacités nommées et montrées, règle
  d'exclusion (`net.hosts` ou lecture du clair, jamais les deux), bornes de mémoire et de temps
  (pas de *fuel* dans wazero : interruption par contexte).
- **Utilisatrice : oui** (liste des capacités, phrases montrées).

- **Révision 2 (revue Codex M6).** La liste d'exclusion était partielle (`import.read`, `self.read`, `presence.read`, entrée des `renderers`). Remplacée par une **règle de flux fermée** : sources I0–I3, puits O0–O4, règles RF-1 à RF-5 calculées par compartiment (spec §15.5) ; secrets d'un dorsal injectés par Go (`secret:<nom>`) ; `ai.complete` (O4) seul chemin de sortie du contenu, par Go, sous politique et aperçu.

### AD-59 — Surface et versions de l'API

- **Recommandation.** WIT `inkwell:plugin@1.x`, SemVer, règles PV-1 à PV-6 (§15.16).

### AD-60 — Greffons et collaboration

- **Recommandation.** Jamais d'installation globale depuis un pair ; un projet peut porter ses
  greffons (`AD-69`) ; un contenu produit par greffon est un contenu standard ; repli ND-7.

### AD-61 — Distribution

- **Options.** Fichier seulement ; catalogue signé statique (index signé, consulté sur geste) ;
  catalogue avec TUF ou Sigstore (non vérifié dans cette étude).
- **Recommandation.** Fichier en v1 ; catalogue signé ensuite, par ADR, exigeant le kit de
  conformité.
- **Utilisatrice : oui.**

### AD-62 — Budgets

- **Recommandation.** §15.4 et §15.9 : 0 code d'extension avant la peinture, ≤ 10 ms pour 50
  manifestes, appel p95 ≤ 2 ms (64 Kio), arrêt des greffons ≤ 1 s ; preuves `BenchmarkRegistryBuild`,
  `PERF-EXT-*`, P-PLUG-BENCH.

### AD-63 — Le cœur sur les mêmes points

- **Recommandation.** Registre unique (X-2), extensions intégrées (`src/ext`, FE-3), chaque point
  avec un client intégré (P-EXT-3) ; Tâches (U29) livrée en greffon WebAssembly pour éprouver le
  chemin complet ; `ui.native` seule exception (chemin de frappe).
- **Norme et preuve.** VS Code (fonctions du produit livrées en extensions, contributions
  déclaratives), Eclipse (« tout est plug-in », registre lu sans activer), CodeMirror 6 (tout est
  extension) ; leçon : aucun ne cumule extensibilité et sécurité, d'où le noyau fermé et les
  capacités (étude §6bis).

### AD-64 — Ce qui change le PDF

- **Recommandation.** X-8 : jamais un greffon dans la compilation ; paquets Typst fournis
  (greffons Typst compris) ; résultats de greffons = fichiers du projet (rendus, §15.14).
- **Raison.** G5 (PDF identique chez tous) ; un appareil sans le greffon compilerait autre chose.

### AD-65 — Points déclarés par un greffon

- **Recommandation.** §15.12 : API 1.1, graphe SemVer, refus des cycles et conflits, appels par
  l'hôte, règle d'exclusion par composante du graphe d'appel (pas d'adjoint complaisant).
- **Norme.** Points d'extension d'Eclipse, `extensionDependencies` de VS Code.

### AD-66 — Données d'extension dans le document

- **Recommandation.** Différé ; exige une règle de garde (ADR 0030) et des bornes dans le
  protocole. v1 : magasin privé, contenu standard.

### AD-67 — Dorsal de stockage

- **Recommandation.** §6.7 : copie locale autoritative ; dorsal byzantin, chiffré avant
  l'extension, adressé par contenu (BLAKE3 à clé), revérifié ; têtes par appareil (pas d'échange
  atomique nécessaire) ; v1 = sauvegarde et restauration pour une même personne.
- **Norme et preuve.** Dorsal de restic (opérations sur noms opaques, dépôt non fiable chiffré),
  services d'OpenDAL (opérations uniformes, drapeaux de capacité), `io/fs` ; Syncthing (appareils
  « non fiables » chiffrés).
- **Utilisatrice : oui** (portée v1 : sauvegarde personnelle).

- **Révision 2 (revues Fable M-4, Codex B3).** ST-7 promettait de voir tout rejeu : faux pour un appareil neuf. Spec §6.7 réécrite : clé **par projet et par personne** (HKDF, domaines séparés), AEAD XChaCha20-Poly1305 avec AAD (projet, nom), graphe objets → manifeste → tête, deux têtes gardées, têtes jamais réécrites ; **fraîcheur** : détectée sur un appareil déjà synchronisé, **non vérifiable** sur un appareil neuf (résiduel écrit à l'écran), ancres : phrase de récupération datée et synchronisation avec les membres ; restaurer ne réimpose jamais une autorité ancienne (pli) ; tailles visibles (résiduel).

- **Révision 3 (revues Fable R2-B4, Codex R2-5).** Phrase de 24 mots = **secret seul** (BIP-39) ; **kit de secours** versionné à part (projets, dorsal, compteurs par appareil, dates), authentifié ; un secret par personne, clés par HKDF (sauvegarde et destinataire de secours du coffre) ; nommage BLAKE3 à clé complet et vérification identique ; manifestes et têtes en CBOR avec AAD définis ; têtes signées sous un préfixe externe ; compteurs par `(ProjectID, PeerID)` ; **aucun ménage distant en v1** (liste non fiable, appareils multiples) ; N22 **non déclarée satisfaite** par la sauvegarde (audit de cohérence C08).

- **Révision 4 (Codex R3-M1).** Espace de noms par projet `p/<loc>/` avec `loc` dérivé du **secret seul** (le dorsal ne voit pas le `ProjectID`) ; descripteur de projet chiffré : la **phrase seule** (plus l'adresse du dorsal) retrouve les projets ; manifestes et têtes : même enveloppe que les objets (version, nonce, AEAD), têtes signées et vérifiées ; kit **proposé** à la régénération, pas exigé (Fable).

- **Révision 5 (Codex R4-B1).** Le secret de secours n'est **jamais** sur l'appareil (SR-1 à SR-4, spec §6.7) : en mémoire seulement à sa création et à une restauration, effacé ensuite, page verrouillée quand le système l'accorde, fenêtre exclue des captures quand c'est possible ; l'appareil garde la clé **publique** de secours et des sous-clés de sauvegarde **enveloppées** ; la phrase et le kit sont gardés hors de l'appareil.

### AD-68 — Nuage partagé entre membres (v2)

- **Constat.** Utile quand les membres ne sont jamais en ligne en même temps ; exige une clé de
  stockage de projet partagée, distribuée par le ledger (K-1, K-2) : nouvelle ADR.

### AD-69 — Greffons de projet

- **Recommandation.** §15.10 : décision d'admin `plugin/<id>` (nouvel espace de noms, extension
  compatible du protocole), blob vérifié, niveau « projet » chargé sans question, accord local au-
  delà, hôte par projet, aucune écriture sans geste, provenance visible, interrupteur, bornes.
- **Norme et preuve.** Macros d'Office bloquées par la marque du Web (2022) ; *Workspace Trust* de
  VS Code (qui ne peut pas empêcher une extension d'ignorer le mode restreint) ; Jupyter
  (sorties non fiables jamais exécutées) ; `.obsidian/plugins` (contre-exemple) ; paquets Typst
  (modèle) ; greffons d'organisation de Figma.
- **Utilisatrice : oui** (confirmer le niveau « projet »).

- **Révision 2 (revues Fable B-2, B-4 ; Codex B5, M1).** (1) `plugin/<id>` n'est **pas** une extension compatible du protocole accepté : il exige la mineure PD-1 (`AD-83`) ; N23 est bloquée d'ici là. (2) Niveau « projet » réduit à **contribuer** : ni remplacer (vue par défaut, thème…), ni envelopper (collage), décorations de marque seulement (aucun `replace`, aucun widget qui masque), `transforms` par commande seulement. (3) **Mandat** borné (commande, documents, 30 s, propositions comptées), révoqué à la désactivation, la mise à jour, une nouvelle décision, la fermeture ; réponses tardives jetées. (4) Rendus : locaux seulement pour un greffon de projet.

### AD-70 — Diagrammes

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Recommandation.** §15.14 : contrat pur `renderers` ; hôte `wasm` par défaut, cadre isolé pour
  Mermaid ; assainisseur SVG en Go, affichage en `<img>` ; rendu partagé en blob et entrée
  `renders` de l'index (règle de garde à ajouter) ; paquet Typst `inkwell-diagrams`.
- **Norme et preuve.** mermaid #3886 (mise en page par le navigateur) ; CVE-2026-41148 et 41149
  (injections dans les SVG de Mermaid) ; `diagraph` (Graphviz en greffon Typst) ; D2.js.

- **Révision 2 (revues Fable B-1, m-2 ; Codex B2, B5).** (1) Le cadre `frame` faisait tourner du JavaScript tiers dans le processus web qui tient le pont : **retiré**. Mermaid est **différé** jusqu'à la porte P-FRAME (processus web dédié, tuable, sans liaisons, réseau et navigation refusés par l'hôte, prouvé sur WebKitGTK) ; alors capacité `render.frame`, hors niveau « projet ». Graphviz en `wasm` est le cas livré. (2) La carte `renders` dans l'index exigeait une règle de garde : remplacée par des **fichiers ordinaires** (`.renders/<clé>.svg`, `.renders/index.json`), validation à l'interprétation ; **aucun delta du protocole**. (3) Polices du rendu = polices fournies à Typst ; `description` exigée (accessibilité).
- **Utilisatrice : oui** (Mermaid différé).

- **Révision 3 (Fable D-2).** Voie retenue pour l'API 1.1 : `beautiful-mermaid` (TypeScript, MIT, rendu SVG sans DOM d'un sous-ensemble de Mermaid) compilé en WebAssembly par le kit JavaScript, comme `renderer` `wasm` ordinaire ; Mermaid complet reste derrière P-FRAME. `.renders/index.jsonl` en **JSON Lines** (tolérant à la fusion de texte) ; la clé de rendu inclut l'empreinte des polices du projet (revue Fable R2-B3).

- **Révision 4 (D-15 convergé).** `beautiful-mermaid` est un **candidat** de l'API 1.1 derrière la porte **P-MERMAID** (§0.5 : module épinglé, importations permises, ≤ 8 Mio, p95 ≤ 300 ms pour 200 nœuds, polices, corpus pris en charge et refusé, SVG assaini, annulation, six cibles) ; repli : Mermaid absent, et dit ; rien en 1.0 ; Mermaid complet derrière P-FRAME, qui doit valoir sur chaque moteur de cible.

### AD-71 — Expérience des développeuses

- **Recommandation.** §15.15 : DX-1 à DX-7, bonjour en 10 lignes, quatre langages, mesures.
- **Norme.** Google AIP (193, 151), guides de l'API des extensions de VS Code, *Rust API
  Guidelines*.

- **Révision 2 (revues Fable M-3, m-4 ; Codex M5).** 29 interfaces pour 60 fonctions étaient incompatibles. API **étagée** (spec §15.21) : 1.0 (13 interfaces, ≤ 60 fonctions, ≤ 45 types), 1.1, 1.2, avec plafonds par étape ; kits Rust et Go en 1.0, TypeScript et AssemblyScript en 1.1 ; GREASE seulement dans `host.features()` ; l'exemple « bonjour » est bilingue (ND-4).

### AD-72 — Spécification du protocole des greffons

- **Recommandation.** `docs/plugin-api/`, normative (BCP 14), plan du §15.16, vecteurs et kit de
  conformité ; **rédigée ensuite**, avec sa propre boucle de revue Fable + Codex, comme
  `docs/protocol/`.
- **Utilisatrice : oui** (acceptation, comme l'ADR 0034).

- **Révision 2 (revue Codex B1).** Le noyau de l'ABI est maintenant **normatif dans la spec** (§15.20 : module réacteur, exports, imports, WASI toléré, réponse et erreurs, négociation, annulation, octets signés du paquet, vecteurs minimaux) ; `docs/plugin-api/` reste à écrire pour les interfaces de chaque point et les registres, avec sa propre boucle.

- **Révision 3 (revues Fable R2-B2, Codex R2-3).** ABI corrigée : WASI toléré **dérivé des importations réelles** de Go et de Rust et neutralisé fonction par fonction (`poll_oneoff`, `sched_yield`, `fd_prestat_*` compris), horloge quantifiée jamais nulle, **aucune réentrance** (`host_call` puis `host_read` dans une zone allouée par le greffon), maximum mémoire imposé par l'hôte, paquet signé par une liste canonique d'empreintes sous un préfixe externe. Rust et Go en 1.0 avec chaînes d'outils épinglées (ABI-9) ; TinyGo en 1.1, sa table d'importations établie par mesure. Les vecteurs et modules sont produits par P-PLUG-PROTO avant l'acceptation de l'hôte (`AD-95`).

### AD-73 — Surfaces d'interface et bouton d'extension

- **Recommandation.** §15.13 : contribution `{commande, emplacement, groupe, ordre, quand}` ;
  une action par extension dans le menu « Extensions », épinglée par la personne seule (globale,
  ou par projet pour un greffon de projet).
- **Norme.** `menus` de VS Code, `menuContributions` d'Eclipse, groupes d'actions de JetBrains,
  GMenu/GAction de GNOME, bouton « Extensions » unifié de Firefox (109).

### AD-74 — Mode débogage

- **Recommandation.** §15.15 : piles DWARF de wazero, Inspecteur, enregistrement et rejeu ; pas à
  pas par rejeu sous Wasmtime (`gdbstub`, LLDB, DAP) ou en natif ; jamais pour un greffon de
  projet reçu.
- **Constat honnête.** wazero n'offre pas de débogueur ; le pas à pas direct dans inkwell n'est
  pas possible.

### AD-75 — Thèmes

- **Recommandation.** §15.8 : données typées sur `tokens.json`, paires de contraste de
  `tokens.json#/contrast/pairs` recalculées (WCAG 2.2), refus complet en cas d'échec.

### AD-76 — Cycle de vie et événements

- **Recommandation.** §15.9 : bus versionné, asynchrone, coalescé, borné, gardé par capacités ;
  aucun crochet avant la peinture ; arrêt : données durables d'abord, puis 1 s pour les greffons.

---

## Révision 2 : décisions nouvelles

### AD-77 — Corriger G-0 sans attendre le modèle complet

- **Doute.** Les deux revues confirment le défaut (`App.svelte:197`, `Sidebar.svelte:17`,
  `workspace.svelte.ts:42,247,275–293`) et qu'un correctif du seul rappel ne suffit pas ; le modèle
  complet dépend de l'index `G` et des annonces (ADR 0031, étape 8 d'ADR 0034).
- **Recommandation.** Trois lots (spec §6.6) : **G-0a** immédiat (ouvrir pour de vrai : `main.typ`
  modifiable, les autres en lecture seule depuis la projection avec un bandeau honnête ; fin de
  `?file=`) ; **G-0b** après les annonces (MF-1, MF-2, MF-5, MF-12) ; **G-0c** avec le Worker
  (MF-6, MF-7, MF-19–MF-24). Tests nommés FM-OPEN-1 à 8.

### AD-78 — Modèles de projet (N27)

- **Recommandation.** Spec §6.8 : aucun modèle livré ; un modèle est un projet de la bibliothèque
  locale, décrit par `typst.toml [template]` (format des modèles de Typst Universe) ; créer depuis
  un projet (sans historique ni membres), modifier, dupliquer, supprimer, importer et exporter
  (archive `AD-38`), partager ; nouveau projet = copie locale, **jamais** `typst init` (il
  télécharge) ; vignettes compilées localement ; plusieurs variantes (document, présentation) par
  modèle.
- **Doute pour l'utilisatrice.** N27 cite « Markdown + CSS ». Le PDF d'inkwell vient de Typst
  (déterministe, G5) ; un style **CSS** pour le PDF exigerait un moteur de navigateur comme second
  moteur de mise en page (rendu variable d'une machine à l'autre). Recommandé : Markdown mis en page
  par un gabarit Typst (`cmarker`), CSS réservé à l'export HTML.
- **Utilisatrice : oui.**

### AD-79 — Erreurs comme un IDE (N28)

- **Recommandation.** Spec §15.17 ER-1 à ER-4 : onglet « Problèmes à corriger » unique (Typst,
  service de langage, CSV, Markdown, CSS des modèles), diagnostics ancrés par curseur Automerge,
  F8/Maj+F8, clic → bon fichier, plage sélectionnée ; messages par table déterministe, brut sous
  « Détails techniques ».
- **Norme et preuve.** VS Code (panneau Problèmes, F8), étude editor-features §3.2 ; un message
  d'un pair n'est jamais du HTML.

### AD-80 — Service de langage tinymist (N29)

- **Recommandation.** Spec §15.17 LSP-1 à LSP-5 : enfermé, liste blanche, `didChange` depuis la
  webview (texte Automerge), multi-fichiers (autres fichiers sur la projection), versions et
  réponses périmées jetées, p95 ≤ 100 ms, docstrings `///` rendus sûrs, `syntaxOnly` au-delà du
  palier Grand, hors ligne prouvé par un test en espace de noms réseau vide.
- **À vérifier au banc** (étude §7) : complétion des étiquettes et clés bibliographiques ; options
  de chemin des paquets de tinymist.

### AD-81 — Outils d'écriture (N30)

- **Recommandation.** Recommandations 11 à 18 de l'étude (addendum §8–§11), en extensions
  intégrées : orthographe embarquée, dictionnaire du projet, onglet « Relecture », typographie
  française, focus, comptes et objectifs locaux, recherche et extraits du projet, insertions,
  comparaison par mots, préréglages d'export. LanguageTool en greffon optionnel ; gamification
  écartée. **À mesurer** : taille des dictionnaires, `lang: "fr"` sur le Typst épinglé, licence des
  synonymes.

### AD-82 — IA de premier rang (N31)

- **Recommandation.** Spec §15.18, d'après l'étude `research/ai.md` : extension intégrée
  `ext/ai` ; une API (registre de commandes) pour palette, greffons, agent et MCP ; fournisseurs
  compatibles OpenAI appelés **par Go** (AC interne, mTLS, OIDC, proxy) ; politique IA du projet
  (`off`, `local`, `org`, `libre`), défaut « rien ne part sans décision d'admin » dans un projet
  partagé ; aperçu de ce qui part ; propositions par morceau, couche fantôme hors CRDT ; règle des
  deux et CaMeL allégé contre l'injection ; discussion = artefacts déclaratifs ; MCP par shim stdio
  et socket `0600`, HTTP local en option.
- **Résolu** : le conflit de l'étude avec la règle d'exclusion (§3.3 de l'étude) est réglé par le
  puits O4 et `ai.complete` (RF-3) : le contenu ne sort que par Go, sous politique et aperçu.
- **Vérifié le 2026-11-01** : révision MCP 2026-07-28, requêtes sans état
  (modelcontextprotocol.io/specification/latest) ; échantillonnage, racines et journalisation
  dépréciés (README de go-sdk) ; go-sdk ≥ v1.7.0 prend en charge 2026-07-28. **Ouvert** :
  gestionnaire Streamable HTTP du SDK (avant MCP-2) ; affirmations « [mémoire] » de l'étude sur
  les autres fournisseurs et éditeurs, à sourcer avant les ADR 0040 et 0041.
- **Utilisatrice : oui** (défauts de la politique, exposition MCP).

- **Extension de l'étude (§9–§11, ACP, UTCP).** Intégrée en spec §15.18 (ACP-1, UTCP, règles de
  projet signées). Vérifié le 2026-11-01 : ACP v1 stable, v2 en brouillon, registre stabilisé, SDK
  officiels Kotlin, Java, Python, Rust, TypeScript, **pas de Go** (agentclientprotocol.com/llms.txt).
  Questions ouvertes de l'étude (§11.5, ses numéros 8 à 16), suivies ici comme **IA-Q8 à IA-Q16** :
  SDK ACP en Go ou client maison ; attendre ACP v2 stable ; bac à sable de sous-processus
  multiplateforme ; empreinte et mise à jour d'un agent installé par la personne ; conversion
  `oldText/newText` en opérations ancrées sous concurrence ; licences (ACP, poids Zeta2) ; statut de
  MCP Apps ; Open Responses contre Responses ; UTCP (attendre un hôte majeur, licence MPL-2.0 du
  SDK Go, modèle d'appel `mcp` sur stdio). Aucune ne bloque les premières étapes (fournisseurs,
  propositions, MCP stdio).

- **Révision 3 (revues Fable R2-M3, Codex R2-9).** Aperçu d'envoi à chaque nouvelle provenance, fichier ou destination dans la session ; indicateur permanent ; consommation visible et bornée (jetons comptés ou estimation marquée, coût inconnu dit) ; contamination monotone sur toute la session (outils et extraits de l'index compris) ; identité MCP par **jeton de lancement** par (client, projet), nom annoncé non pris pour une preuve ; MCP désactivé dans les projets partagés avant PD-1 ; ACP sans garantie prêtée ; UTCP écarté de la spec (Fable) ; RAG en §15.18 RG-1 à RG-5 (`AD-89`).

- **Révision 4 (Fable R3-M2, Codex).** ACP est un **report après v1**, pas une expérimentation : un agent tiers a besoin de fichiers et de réseau ; sa réouverture exigera son propre profil (destination et proxy, fichiers, processus, tests d'évasion).

### AD-83 — Deltas du protocole

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Constat (revues Fable B-2, M-5 ; Codex B5).** La révision 1 appelait « extension compatible »
  des ajouts que le protocole accepté ne tolère pas (registre §20.5 fermé, aucune règle pour un
  espace inconnu, HELLO sans version d'application).
- **Recommandation.** Table PD de la spec (§15.19) : PD-1 (espaces `plugin/` et `ai/`, règle de
  tolérance « entrée conservée et ignorée, op acceptée, sans sanction », émission après `format`) et
  PD-2 (domaines de signature) ; proposés pour une **mineure 2.1** dans la boucle du protocole ;
  carte `renders` et TLV `PLUGIN_API` **retirés** (plus nécessaires).
- **Utilisatrice : oui** (acceptation de la mineure).

- **Révision 3 (revues Fable R2-B3, Codex R2-4).** PD-1 réécrit : valeur = base64url d'un CBOR déterministe (le champ est un `str16` UTF-8) ; schémas `plugin/`, `ai/`, `ignore/` ; jamais émis avant `format ≥ 1` ni mêlé à un espace de la 2.0 ; **règle de pli à la réception** (entrée conservée et ignorée tant que `format ≥ 1` n'est pas debout dans son passé causal) ; aveu que la 2.1 ne rend pas tolérants les appareils 2.0 (transition par la lecture seule du protocole §1.4.4) ; vecteurs nommés. **PD-2 retiré** : signatures de paquets, de têtes et d'exports sous des préfixes externes `inkwell-ext:`, hors registre §20.4 (dont la grammaire interdit `/`). PD-3 (clé X25519 dans les ops de membres) reporté.

### AD-84 — Fidélité à la conception

- **Recommandation.** Ne pas affirmer la fidélité complète : la matrice des décisions finales liste
  chaque écart (AD-01, AD-13, AD-14, AD-33, AD-67, AD-69, AD-70, AD-78) et ce qu'il faut pour le
  lever.

### AD-85 — Ordre de livraison de l'API

- **Recommandation.** Spec §15.21 : 1.0 réduite (13 interfaces), 1.1, 1.2 ; ordre registre →
  commande, panneau, proposition → Graphviz jusqu'au PDF → reste ; les points déclarés par un
  greffon, le dorsal et le service de langage des greffons en 1.2.

### AD-86 — Canal Go ↔ hôte des greffons

- **Recommandation.** Spec §7.3 IPC-1 à IPC-6 : `socketpair` `SOCK_SEQPACKET` seul descripteur
  avec des `memfd` scellés ; trames bornées (≤ 4 Mio) ; compartiment et génération attribués par le
  parent ; un appel à la fois par instance, file ≤ 8 par greffon, ≤ 16 en vol par hôte ; annulation ;
  vecteurs et fuzz (règle B-1).

- **Révision 3 (revues Fable R2-B1 point 3, Codex R2-2).** `SOCK_STREAM` (Linux, macOS) et tubes anonymes (Windows) au lieu de `SOCK_SEQPACKET` (borné par `SO_SNDBUF`) ; cadrage complet (gros-boutiste, types, drapeaux, bornes par type), contre-pression **en octets**, annulation prioritaire, descripteurs ancillaires refusés sauf le `memfd` d'activation ; autorité attachée à la connexion (un greffon par hôte).

### AD-87 — Enfermement de l'hôte et compartiments

- **Recommandation.** Spec §9.10 H-1 à H-8.
- **Norme et preuve.** Landlock (documentation du noyau) ; restriction AppArmor des espaces de noms
  utilisateur d'Ubuntu 24.04 (`kernel.apparmor_restrict_unprivileged_userns=1`, citée par la revue
  Fable) et `internal/sidecar/netns_linux.go:22–34` (repli actuel) ; défense en profondeur (NIST SP
  800-53 SC-39). Test d'un enfant simulé compromis (`PLUG-COMPROMISED-*`).
- **Ouvert, mesuré par P-PLUG-BENCH** : coût de la compilation sans cache disque ; un cache géré par
  le parent est une décision ultérieure.

- **Révision 3.** Remplacée par la spec §9.10 révisée (H-0 démarrage en deux temps ; matrice des noyaux Ubuntu 22.04, Debian 12, Ubuntu 24.04, Debian 13, Fedora ; Landlock masqué par ABI sondée ; seccomp barrière principale) et §9.11 (autres plates-formes, `AD-94`).

---

## Révision 3 : passe de cohérence et exigences N32 à N41

Le détail des contradictions, manques, doublons, carte produit et phasage est dans
[coherence.md](coherence.md) ; ici, les décisions et leur raison.

### AD-88 — Gestionnaire de tâches (N32)

- **Recommandation.** Spec §11.1 : `internal/tasks`, table unique de tout ce qui consomme ; mesures
  par le **parent** (ce qu'un hôte dit de lui-même est indicatif) ; verbes annuler, mettre en pause,
  arrêter, désactiver ; limites par classe réglables à la baisse ; vue « Ressources » hors du tiroir.
- **Norme et preuve.** Firefox `about:processes` ; revue Fable §4.1 et Codex R2-11 ; un greffon ne
  peut pas fournir la vue qui sert à l'arrêter (Codex).

- **Révision 4 (Codex R3-M6 ; Fable m-13).** Quatre niveaux (information, plafond WebAssembly, plafond du noyau, chien de garde du parent qui tourne toujours, relevé toutes les 500 ms, arrêt après 2 relevés au-delà) ; limites chiffrées par classe ; réserve du cœur (25 % d'un cœur, 512 Mio) ; verbes par classe (le cœur n'offre qu'« annuler un travail ») ; « Arrêter » n'interrompt jamais une écriture non durable ; parts estimées des tâches internes dites comme telles.

### AD-89 — Index de recherche pour l'IA (N33)

- **Recommandation.** Spec §15.18 RG-1 à RG-5, d'après `research/ai.md` §12 : instantané venu de la
  webview, lexical d'abord, moteur choisi par la mesure (SQLite FTS5 en Go pur, bleve, index
  interne), local et jetable, chiffré comme le projet, provenance par extrait, citations par nœud et
  révision (pas par chemin : un renommage ne les casse pas, Codex R2-11), embeddings = envoi O4.
- **Ouvert, mesuré par P-IDX** : moteur, rappel en français, coût sur les six cibles.

- **Révision 4 (Fable R3-M1, Codex R3-B4, R3-M5).** Porte **P-IDX** mesurable : corpus français versionné (projet d'exemple et deux projets Typst publics, 200 requêtes écrites avant l'essai, jugements à trois niveaux), **rappel@10 ≥ 0,8 et MRR ≥ 0,6**, aucune requête sans résultat, retard ≤ 5 s ; R1 ou R2 seulement à +0,1 de rappel@10 ; repli : recherche d'IA sans index. Autorité effective = intersection (RG-6) ; file coalescée bornée, mise à jour forcée par une requête, invalidation immédiate (RG-7) ; pour un projet protégé, l'index vit dans le dossier de travail.

### AD-90 — Fichiers non synchronisés (N34)

- **Recommandation.** Spec §6.9 : ignorer = ne pas importer un fichier local, ne pas télécharger
  un blob ; jamais un retrait chez les autres ; deux couches (personnelle maintenant, projet par
  décision signée après PD-1) ; syntaxe gitignore par le paquet de go-git ; **jamais** `.typstdeps/`
  ni ressources référencées (contre l'étude, qui l'excluait par défaut : contraire à N5 et à
  l'ADR 0024).
- **Norme et preuve.** Teamtype (VCS non synchronisé par défaut), Syncthing (`.stignore` jamais
  synchronisé), revue Codex R2-11 (un filtre par chemin n'est pas une frontière d'admission).

- **Révision 4 (Codex R3-M3, Fable m-8).** Table des transitions (§6.9 IG-5) : seul un fichier jamais admis crée un nœud ; un texte déjà admis ne peut pas être « désynchronisé » (l'écran dit que ses nouvelles modifications continuent) ; un blob non téléchargé n'est jamais annoncé comme disponible ; une dépendance du principal ne peut pas être exclue.

### AD-91 — Projet chiffré (N37)

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Recommandation.** Spec §6.10, d'après `research/encryption.md` : pas de montage ; chiffrement
  par objet ; DEK **par appareil** (les pairs échangent du clair sur un canal chiffré, donc aucune
  clé de groupe) enveloppée au format age pour la clé X25519 de l'appareil, le destinataire de
  secours et les autres appareils de la personne ; trousseau du système ou mot de passe ; dossier de
  travail privé ; export zip ou zip chiffré par age.
- **Interprétation à faire accepter** : « un bloc chiffré, rechiffré à chaque écriture » est tenu
  **par objet** (une frappe ne rechiffre que l'objet touché), pas par un conteneur monolithique.
- **Expérimentation** : P-VAULT. **Utilisatrice : oui** (ADR qui remplace NG3).

- **Révision 4 (Codex R3-B1, R3-B2 ; Fable R3-M5, m-9).** (1) La DEK (32 octets) est la **charge utile d'un fichier age complet** (bibliothèque `filippo.io/age`, stanzas inchangées), liée au projet, à l'appareil et à l'époque ; destinataires : la clé de l'appareil et le destinataire de secours ; DEK **par projet et par appareil** (pas d'enveloppe pour les autres appareils). (2) Le dossier de travail n'existe **que** sur un support qui ne garde pas de clair après coupure ou vol : mémoire (tmpfs sans swap en clair ; disque en mémoire sous macOS, à prouver) ou disque chiffré par le système **vérifié** ; sinon, pour un projet protégé, aperçu, export lisible et service de langage des fichiers fermés sont refusés sur la cible. (3) Budget : matérialisation par dépendances réelles, ≤ 512 Mio, refus au-delà. (4) Git dans le coffre (go-git sur `billy.Filesystem`), index dans le dossier de travail. (5) Repli : NG3 reste et G13 passe en v1.x si P-VAULT échoue.

- **Révision 5 (Codex R4-B1, Fable R4-M2, m-1).** Règle VK (EN-3) : toute voie locale vers la DEK dépend d'un secret absent du disque lisible hors ligne : clé privée X25519 dans le trousseau, ou enveloppée sous mot de passe (Argon2id 256 Mio, 3 passes, 4 fils), ou sur disque chiffré vérifié ; **aucun repli** vers un fichier nu pour un projet protégé ; P-VAULT : vol hors ligne, aucun trousseau, recherche des octets de clés sur l'image. **Voie de récupération** : par le destinataire de secours, un `peer` différent est accepté, l'ancien appareil affiché, la DEK ré-enveloppée pour le nouvel appareil sous une **nouvelle époque**. **Oracles** de « vérifié » nommés (`/proc/swaps`, `dm/uuid` en `CRYPT-`, `lsblk`, `sysctl vm.swapusage`, `fdesetup status`, `Win32_EncryptableVolume`) ; le disque en mémoire de macOS exige aussi le swap chiffré.

- **Révision 6 (Codex R5-B1, R5-B2, R5-M1 ; Fable R5-M1, m-1 à m-5).** (1) **Cérémonie de la phrase** (SR-5) : Go ne peut verrouiller que ses tampons ; la webview garde les mots dans un processus de rendu non verrouillable ; la garantie minimale (aucune page de la phrase écrite en clair sur un support persistant) vient de l'oracle de pagination (swap et hibernation) joué avant toute création ou restauration ; s'il échoue, la cérémonie est **refusée** avec la marche à suivre. (2) **Oracles liés au volume** (OV-1 à OV-4) : chaque chemin sensible (dossier de travail, clé X25519 en mode disque, swap, hibernation) est résolu jusqu'à son volume réel et vérifié sur ce volume (sysfs sous Linux, `zram` sans périphérique de repli en clair ; volume APFS sous macOS, le statut global de FileVault ne suffit pas ; volume BitLocker sous Windows) ; placement forcé sous le dossier de la personne. (3) §9.4 corrigé : le trousseau garde la clé X25519 de l'appareil, **jamais** le secret de secours ; lignes de la clé X25519, de la DEK et du secret dans les tables des §9.4 et §6.1. (4) La récupération ne révoque pas l'ancien appareil : elle propose « Tout rechiffrer ».

### AD-92 — Historique sans Git (N39)

- **Recommandation.** Spec §6.11 HI-1 à HI-5 : voyager dans le temps (vue en lecture seule),
  restaurer par une **nouvelle modification** prévisualisée et annulable (jamais un remplacement qui
  ressuscite une autorité ou une purge, Codex R2-11), versions nommées = manifestes de têtes de tout
  le projet, comparer par mots, projection go-git (`AD-13`). Zéro mot de Git dans l'interface.

- **Révision 4 (Codex R3-B3 ; Fable m-10).** Les versions nommées **épinglent** leurs blobs, rendus et paquets dans le ménage (§6.5, HI-9), épingle posée avant la publication du manifeste ; un moment quelconque n'épingle rien et la vue le dit ; une **purge** l'emporte (version « incomplète », rien ressuscité) ; horloge du voyage = ordre de réception sur l'appareil, heure déclarée marquée (HI-10).

- **Révision 5 (Fable R4-m2, R4-m4).** Version **incomplète dès la création** quand un blob manque partout (demandé aux pairs avant la publication) ; un appareil qui reçoit un manifeste épingle ce qu'il a et va chercher le reste. Une personne qui peut écrire l'original **prévisualise** l'intégration comme une proposition et l'applique elle-même (pas les règles des auteurs non humains).

### AD-93 — Performance mesurée (N38)

- **Recommandation.** Spec §12.5 : budgets par chemin (valeurs proposées, fixées par une première
  mesure de référence puis tenues), règles PF-5 à PF-10 (parallélisme borné par `errgroup.SetLimit`
  ou pools, `goleak`, `-race`, `synctest`, PGO gardé seulement s'il gagne, budgets du fil
  d'affichage, `bench:compare` en CI, choix de bibliothèque par banc).
- **Norme.** `engineering-principles.md` §3 ; consigne du dépôt « prouver par benchstat ».

- **Révision 4 (Codex R3-M4 ; Fable R3-M4, m-11, m-14).** Protocole de mesure normatif (§12.5 PM-1 à PM-6) : corpus versionnés, conditions, échantillons, chronomètres, bruit ; RSS/PSS plafonnée, VSZ observée à part ; Long Tasks ne prouve que ≥ 50 ms, frappe mesurée à la peinture suivante ; seuils fixés avant comparaison ; **mesure de référence dans le premier lot du MVP** ; budgets ajoutés (voyage dans le temps, comparaison par mots, motifs, déverrouillage, rechiffrement) ; BLAKE3 par architecture.

- **Révision 5 (Codex R4-M1).** PM-4 sépare gestionnaire (≤ 4 ms), attente du rappel d'image et latence jusqu’à l’affichage ; `requestAnimationFrame` ne prouve pas la peinture ; mesure par Event Timing là où le moteur l'offre ou par capture horodatée externe ; chaque oracle validé par un cas témoin de mise en page lente ; sans oracle validé, le cas échoue.

### AD-94 — Port d'enfermement sur six cibles (N35)

- **Recommandation.** Spec §9.11 : `internal/sandbox` avec garanties communes (aucun fichier,
  aucun réseau, aucun processus, mémoire, CPU, mort avec le parent, canal) ; Linux : Landlock et
  seccomp ; macOS : profil Seatbelt appliqué par l'hôte à lui-même (`sandbox-exec` déprécié mais
  Seatbelt reste le seul mécanisme hors App Sandbox ; enfants inclus) ; Windows : AppContainer sans
  capacité, objet de tâche (mémoire, CPU, un processus, tuer à la fermeture), politique « aucun
  enfant », poignées héritées par liste. Les sidecars passent par le même port et lisent le dossier
  de travail privé.
- **Expérimentation** : P-SANDBOX-MAC et P-SANDBOX-WIN (« le nom d'un mécanisme n'est pas une
  preuve », Codex).

### AD-95 — Ce que vaut l'acceptation de la spécification

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Constat.** Codex R2-10 : une porte déclarée « préalable à l'acceptation » ne peut pas être
  remplacée par une meilleure liste de mesures. Un agent de conception ne produit ni binaire ni
  banc.
- **Recommandation.** Spec §0.5 : la spécification est acceptable sans ces preuves, **mais** les
  parties qui en dépendent (hôte des greffons et ABI, enfermement macOS et Windows, coffre, index,
  rendu à DOM, ACP, deltas du protocole) sont des **expérimentations**, acceptées seulement à leur
  porte, résultats bruts dans `docs/plans/`. Aucun code de production n'en dépend d'ici là.
- **Utilisatrice : oui.**

- **Révision 4 (Codex R3-B4 ; Fable R3-M3, D-14 convergé).** Règles E-1 à E-5 de toutes les portes (entrées et seuils figés avant, résultats bruts, matrice, **repli nommé à l'avance**, rejeu à chaque majeure de macOS et de Windows) ; une colonne de repli par expérimentation ; portes complétées (P-PLUG-PROTO, P-PLUG-BENCH, P-SANDBOX-* avec sidecars, P-VAULT avec inspection hors ligne avant relance, P-IDX, P-GIT-BENCH, P-MERMAID, P-FRAME) ; ACP sorti des expérimentations (report).

- **Révision 5 (Fable R4-m5).** Sortie des sidecars : 256 Mio par compilation, 512 Mio cumulés pour les compilations simultanées, inscrits dans P-SANDBOX (§0.5) et au §9.5.

### AD-96 — Variantes et intégrations (N41)

> *Historique : la recommandation initiale ci-dessous est **remplacée** par le dernier paragraphe « Révision n » de cette entrée, qui seul fait foi.*

- **Recommandation.** Spec §6.11 HI-6, HI-7 : sur les variantes déjà présentes (ADR 0031,
  `history/graph.ts`) ; « Créer une variante », « Comparer avec l'original », « Proposer
  d'intégrer », « Intégrer » (fusion Automerge native puisque l'histoire est commune ; une
  modification annulable ; les passages modifiés des deux côtés deviennent une divergence ordinaire),
  « Abandonner la variante » ; « Dupliquer le projet » pour un projet ; intégrer entre projets
  différents hors v1.
- **Norme et preuve.** Patchwork d'Ink & Switch (branches légères, différences visibles, groupes
  de modifications, histoire comme une conversation) ; Upwelling (brouillons comme branches
  légères).

- **Révision 4 (Codex R3-M2, Fable R3-M6).** Choisi : **différence appliquée**. « Intégrer » applique à l'original actuel la différence entre le point de départ (ou la dernière intégration) et l'état de la variante, comme une proposition de modifications **nouvelles** écrites par qui intègre, admises par les règles ordinaires ; aucune règle d'admission nouvelle ; « Annuler » annule ce groupe ; intégration répétée depuis le dernier point ; zones verrouillées refusées ; divergence ordinaire si les deux côtés ont changé. Écartée pour la v1 : la fusion native (Patchwork, `A.merge` du code actuel), qui exigerait d'amender l'ADR 0031 ; écart du code au §16 de la spec.

### AD-97 — Carte produit et surfaces

- **Recommandation.** `coherence.md` §4 : tiroir à trois destinations (Rechercher, Relire, IA),
  gauche à deux onglets (Fichiers, Sommaire), un seul compteur de problèmes (barre d'état), barre
  d'état à quatre éléments visibles, Ressources hors du tiroir, rien d'affiché pour une fonction
  absente. Liste des changements du design au §6 de `coherence.md`.

### AD-98 — Formats bureautiques (N40)

- **Recommandation.** Aucun natif ; collage riche (HTML → Typst ou Markdown) dans le cœur ;
  greffons `importers` et `exporters` pour qui le veut. La suggestion DOCX/ODT de l'audit de
  cohérence de Codex est écartée par N40.

## Sources consultées pour ces décisions

- Wails v3 : [v3.0.0](https://github.com/wailsapp/wails/releases/tag/v3.0.0) (publiée le
  2026-10-26, vérifié par l'API GitHub) ; [guide de mise à jour](https://v3.wails.io/guides/updater) ;
  [protocole du manifeste](https://v3.wails.io/reference/update-manifest) ;
  [instance unique (DeepWiki, à vérifier dans `application_options.go`)](https://deepwiki.com/wailsapp/wails/13.3-single-instance) ;
  [instance unique v2, données non fiables](https://wails.io/docs/guides/single-instance-lock).
- Perçage de NAT : D. Trautwein, C. Ihle, *Challenging Tribal Knowledge – Large Scale
  Measurement Campaign on Decentralized NAT Traversal*, 2025, <https://arxiv.org/abs/2510.27500> ;
  [ProbeLab](https://probelab.io/blog/can-libp2p-punch-through-nats/).
- Aperçu : [tinymist-preview](https://docs.rs/crate/tinymist-preview/latest/source/README.md),
  [typst-preview](https://github.com/Enter-tainer/typst-preview).
- Typst 0.14 : [billet](https://typst.app/blog/2025/typst-0.14),
  [journal 0.14.0](https://typst.app/docs/changelog/0.14.0) (PDF balisé par défaut, PDF/UA-1).
- Pandoc : [3.10.1](https://github.com/jgm/pandoc/releases/tag/3.10.1),
  [historique](https://pandoc.org/releases.html).
- PDF.js : CVE-2024-4367 (corrigée en 4.2.67).
- Landlock : <https://docs.kernel.org/userspace-api/landlock.html>.
- Greffons : [research/plugins.md](research/plugins.md) ;
  [wazero](https://pkg.go.dev/github.com/tetratelabs/wazero) (v1.12.0 ; ticket #2200) ;
  [wasmtime-go](https://github.com/bytecodealliance/wasmtime-go) (tickets #280, #290, #291) ;
  [Extism go-sdk](https://github.com/extism/go-sdk) ; [WASI 0.3](https://wasi.dev/releases/wasi-p3) ;
  [jco](https://bytecodealliance.github.io/jco/transpiling.html) ;
  [WebKitGTK 6.0, bac à sable](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/gtk/migrating-to-webkitgtk-6.0.md) ;
  [Figma](https://www.figma.com/blog/how-we-built-the-figma-plugin-system/) ;
  [Obsidian](https://obsidian.md/help/Extending%2BObsidian/Plugin%2Bsecurity) ;
  [Zed](https://zed.dev/blog/zed-decoded-extensions).
- Greffons de projet : [macros d'Office](https://learn.microsoft.com/en-us/deployoffice/security/internet-macros-blocked) ;
  [VS Code Workspace Trust](https://code.visualstudio.com/api/extension-guides/workspace-trust) ;
  [Jupyter](https://github.com/jupyter/notebook/blob/4.4.1/docs/source/security.rst).
- Débogage : [Wasmtime, invité](https://docs.wasmtime.dev/examples-debugging-guest.html),
  [Wasmtime, natif](https://docs.wasmtime.dev/examples-debugging-native-debugger.html).
- Stockage : restic `internal/backend/backend.go` ; [OpenDAL](https://opendal.apache.org/docs/concepts).
- Diagrammes : [mermaid #3886](https://github.com/mermaid-js/mermaid/issues/3886) ;
  [CVE-2026-41149](https://nvd.nist.gov/vuln/detail/CVE-2026-41149) ;
  [diagraph](https://typst.app/universe/package/diagraph) ; D2.js (`@terrastruct/d2`).
- Tableaux : [Fluid Framework SharedMatrix](https://fluidframework.com/docs/v1/api/matrix/sharedmatrix-class).
- API : [AIP-193](https://google.aip.dev/193), [AIP-151](https://google.aip.dev/151),
  [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/checklist.html),
  [VS Code, points de contribution](https://code.visualstudio.com/api/references/contribution-points).
- Révision 2 : [MCP 2026-07-28](https://modelcontextprotocol.io/specification/latest) ;
  [go-sdk](https://github.com/modelcontextprotocol/go-sdk) ;
  [Landlock](https://docs.kernel.org/userspace-api/landlock.html) ;
  [Typst, modèles (`typst.toml [template]`)](https://github.com/typst/packages) ;
  [research/editor-features.md](research/editor-features.md) ; [research/ai.md](research/ai.md).
