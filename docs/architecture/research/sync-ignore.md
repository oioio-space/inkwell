# Étude : exclusion de fichiers de la synchronisation

Date : 2026-11-01. Statut : étude, non engageante. Les points marqués (à vérifier) n'ont pas pu être confirmés dans une source consultée.

## 1. Teamtype (ex-Ethersync)

Identification confirmée : Teamtype est le nouveau nom d'Ethersync, outil pair à pair d'édition collaborative de fichiers locaux, AGPL-3.0 (https://github.com/teamtype/teamtype, consulté 2026-11-01).

- Synchronisation : Automerge (CRDT), connexions chiffrées directes via Iroh, sans serveur. Pas de compte.
- Appairage : `teamtype share` affiche un code court (`5-hamburger-endorse`), `teamtype join <code>` le consomme. Le README cite Magic Wormhole, d'où un code de type Wormhole (détail du mécanisme : à vérifier). inkwell utilise déjà ce principe (ADR 0032).
- Éditeurs : un démon local parle JSON-RPC aux greffons (Neovim, VS Code officiels ; JetBrains, Emacs communautaires). Le modèle est « le fichier sur disque reste la référence locale » : les modifications externes sont fusionnées.
- Fonctions : curseurs et sélections des autres en direct, dossiers entiers, local-first (tout reste sur disque hors ligne).
- Ignorés : les dossiers de VCS (`.git/`, `.jj/`) sont ignorés par défaut ; `teamtype share --sync-vcs` (expérimental) les synchronise (https://man.archlinux.org/man/teamtype-share.1.en, versions 0.9.0/0.9.1). Lecture de `.gitignore` ou `.ignore` : non documentée dans ce que j'ai pu lire (à vérifier dans le code).
- À emprunter : valeur par défaut sûre (le VCS n'est jamais synchronisé sans option explicite), code d'invitation court.

## 2. Art antérieur

| Outil | Mécanisme | Enseignement |
|---|---|---|
| gitignore | glob, `**`, `!` ré-inclusion, `/` ancrage, dernier motif gagnant, un dossier exclu ne peut pas être ré-inclus par `!` | Syntaxe connue de tous ; piège de la ré-inclusion sous dossier exclu |
| Syncthing `.stignore` | Même esprit, mais premier motif gagnant ; `(?i)` insensible à la casse ; `(?d)` : un fichier ignoré qui bloque la suppression d'un dossier parent peut être supprimé ; `#include` ; le `.stignore` n'est jamais synchronisé ; motifs par défaut proposés à l'ajout d'un dossier (https://docs.syncthing.net/users/ignoring.html) | Ignore = décision locale par pair ; `(?d)` pour la saleté OS |
| Dropbox / OneDrive | Pas de fichier de motifs ; synchronisation sélective par dossier (local seulement), attribut d'ignore par fichier (Dropbox) | UX par clic droit, marqueur d'état dans l'arbre |
| Resilio `IgnoreList` | Fichier `.sync/IgnoreList` par dossier, glob simple, synchronisable entre pairs selon réglage | Montre le risque et l'intérêt d'une liste partagée |
| Obsidian Sync | Exclusion par dossier et types de fichiers, réglage par appareil | Réglage personnel, vocabulaire simple |
| rclone filters | `--filter` ordonné `+`/`-`, `--exclude-from`, premier motif gagnant | Règles ordonnées explicites, testables à sec (`--dry-run`) |

Qui décide : si le fichier d'exclusion est synchronisé, un pair malveillant peut le modifier pour masquer des fichiers (ou pour en faire exclure d'autres chez tous). Syncthing règle cela en ne le synchronisant jamais ; Resilio permet les deux. Il faut donc distinguer règles de projet (autorité) et règles personnelles (locales).

Défauts usuels : sorties de compilation (`*.pdf` produits, `build/`, `dist/`), `.typstdeps/` (reconstruisible, ADR 0024), caches, VCS, déchets OS (`.DS_Store`, `Thumbs.db`, `desktop.ini`, `*~`, `*.swp`), fichiers temporaires de verrou.

## 3. Recommandation pour inkwell

**Nom et syntaxe.** `.inkwellignore`, syntaxe gitignore (le plus connu, documentation existante). Bibliothèque Go candidate : `github.com/sabhiram/go-gitignore` (simple, peu maintenue), `github.com/go-git/go-git/v5/plumbing/format/gitignore` (déjà liée au projet via go-git, bien testée, sémantique git exacte : choix préféré, aucune dépendance de plus), `github.com/monochromegane/go-gitignore`. À valider par un jeu de tests de conformité contre `git check-ignore` et par fuzz du parseur (règle : toute entrée venue d'un pair est fuzzée). Extensions facultatives, inspirées de Syncthing : `(?d)` seulement si le besoin apparaît.

**Deux couches.**
1. Projet : `.inkwellignore` à la racine, contenu dans le projet, modifiable par les administratrices uniquement, donc signé comme la liste des membres (source de vérité unique, ADR). Un pair vérifie la signature et le rôle avant de l'appliquer. Un pair non administrateur ne peut ni masquer ni démasquer un fichier pour les autres. Versionné, donc auditable.
2. Personnel : `$CONFIG/projets/<id>/ignore` local, jamais synchronisé. Il ne peut qu'ajouter des exclusions pour soi (ne pas recevoir / ne pas publier), jamais ré-inclure ce que le projet exclut.
Combinaison : défauts intégrés, puis projet, puis personnel ; l'exclusion gagne (pas de `!` qui traverse les couches).

**Règles de sécurité.** Un fichier exclu n'est ni annoncé ni envoyé ni accepté : un pair qui envoie un chemin exclu voit l'octet rejeté (même décodeur vérifié). Un fichier déjà synchronisé puis exclu : il reste en local, est retiré de l'index de synchronisation par une opération signée, avec confirmation. Le motif ne doit jamais sortir de la racine du projet (refus de `..`, chemins absolus, liens symboliques).

**Défauts intégrés** (modifiables par l'admin) : `.typstdeps/`, `*.pdf` produits à côté des sources seulement si générés (à trancher : les PDF fournis comme ressources doivent rester synchronisables), `build/`, `.git/`, `.jj/`, déchets OS, `*.swp`, `*~`.

**Interactions.**
- Modèles et greffons de projet : ils sont des fichiers du projet ; ils ne sont jamais exclus par défaut. Un greffon ne peut pas modifier `.inkwellignore` (il est signé, réservé aux administratrices).
- Index RAG : l'index suit le même filtre (ce qui n'est pas synchronisé n'est pas indexé pour les autres) ; l'index lui-même est local et jamais synchronisé. Un fichier exclu personnellement peut rester indexé localement : option explicite.
- Export chiffré : par défaut il respecte l'exclusion de projet (reproductibilité) ; une case « inclure les fichiers non synchronisés » est proposée à l'exportatrice, jamais cochée d'office. Le manifeste indique ce qui a été omis.

**Interface** (vocabulaire : « synchroniser », jamais « ignore », zéro jargon).
- Menu contextuel de l'arbre de fichiers : « Ne pas synchroniser » / « Resynchroniser », qui écrit la règle dans la couche personnelle ; pour une administratrice, un second choix « Pour tout le projet ».
- Marqueur : icône discrète (nuage barré) et texte atténué sur l'élément ; l'info-bulle dit pourquoi : « Ce fichier reste sur votre ordinateur. Règle du projet » ou « Règle personnelle ».
- Explication en langage simple lors du premier usage : « Les fichiers non synchronisés restent chez vous. Les autres personnes ne les reçoivent pas et ne peuvent pas les voir. »
- Écran d'édition des règles : liste lisible, aperçu « 12 fichiers concernés » avant validation (équivalent du `--dry-run` de rclone), éditeur de texte brut pour les avancées.
- Réutilise bannières, popovers et toasts Adwaita existants ; à passer au skill `ux-walkthrough`.

**Exigences à enregistrer** dans `docs/exigences.md` ; ADR à écrire avant implémentation (frontière signature/rôle).

Sources : https://github.com/teamtype/teamtype ; https://man.archlinux.org/man/teamtype-share.1.en ; https://docs.syncthing.net/users/ignoring.html. Les lignes Dropbox, OneDrive, Resilio, Obsidian et rclone proviennent de mes connaissances générales et n'ont pas été revérifiées ici.
