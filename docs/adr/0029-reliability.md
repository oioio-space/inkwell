---
status: proposed
date: 2026-10-26
decision-makers: [Mathieu]
---

# Fiabilité : invariants de non-perte, pannes injectées, endurance, état honnête

## Contexte et problème

Exigence U32 : « que l'on puisse avoir confiance dans son fonctionnement ». Une partie existe déjà :
- écriture atomique vérifiée (`WriteAtomic`, `IKV1` + SHA-256, quarantaine, sauvegardes : ADR 0020) ;
- `TestKillDuringWrite` et `TestKillAtRandomTimes` ;
- blobs vérifiés par Bao, avec reprise ;
- `memnet` à fautes sous `synctest` ; convergence à graine en TypeScript ;
- quatre scénarios netsim et un relais tué en e2e.

Il manque :
- des invariants écrits ;
- la perte de courant (et pas seulement `kill -9`) ;
- le disque plein ;
- un réseau lent ou à pertes ;
- l'endurance ;
- une interface qui dise la vérité sur l'état de synchro.

L'audit de l'ADR 0028 a trouvé un vrai défaut de fiabilité : un envoi sur un lien lent recommence
**sans fin** (constat 9). La confiance se prouve.

## Facteurs de décision

- Aucune perte silencieuse. Une perte possible est bornée, dite et mesurée.
- Chaque invariant a son test qui le casse s'il est violé (`engineering-principles.md`).
- Aucune télémétrie : le diagnostic reste sur la machine (promesse de confidentialité, étape 5).
- Les fautes s'injectent sans code de test dans l'exécutable publié.

## Options envisagées

- Tests d'exemples seulement (l'état actuel) : ils ne trouvent que les pannes imaginées.
- Simulation déterministe de tout le système (comme FoundationDB) : hors de portée, car la webview,
  WebKit et les processus ne sont pas simulables.
- **Couches** :
  - propriétés et simulation à graine là où c'est déterministe (Go `synctest`, TS fast-check) ;
  - injection de pannes réelles là où ça ne l'est pas (netem, LazyFS, `kill -9`) ;
  - endurance et observation locale pour le reste.

  Retenue.

## Décision

### I. Invariants de non-perte (numérotés, cités par les tests)

- **I1 — Arrêt brutal de l'application** (`kill -9`, plantage) : tout ce que Go a acquitté sur le
  Stream `store` survit.
  - Fenêtre au plus : `saveDebounceRate` (100 ms) + un aller-retour du Stream, soit **≤ 200 ms de
    frappe**.
  - Le reste est chez les pairs connectés.
- **I2 — Perte de courant** : la même chose, à la fenêtre de *validation groupée* près, **≤ 500 ms**.
  - Les petits incréments d'automerge-repo sont écrits à l'acquittement, puis synchronisés sur disque
    (`fsync`) par lot : au plus un `fsync` toutes les 500 ms ou tous les 4 Mio par projet.
  - Avant un acquittement, on fait toujours un `fsync` complet pour :
    - les instantanés de compaction ;
    - `project.json` ;
    - la liste des membres ;
    - les clés ;
    - les fichiers possédés.
  - Aujourd'hui chaque incrément coûte deux `fsync` (fichier puis dossier), jusqu'à vingt par
    seconde de frappe : la validation groupée les remplace sans élargir I1.
- **I3 — Jamais de fichier à moitié écrit**, quel que soit le moment de l'arrêt. C'est déjà le cas
  (`WriteAtomic`) ; on l'étend à la projection et aux blobs (`Rename` après vérification).
- **I4 — Rien ne s'efface sans copie durable.** La suppression des incréments après compaction suit
  le `fsync` de l'instantané et de son dossier (le Stream `store` est séquentiel, et un test le fixe).
  La corbeille et la purge suivent l'ADR 0023.
- **I5 — Disque plein (`ENOSPC`) ou erreur d'E/S** : aucune écriture partielle. Le document reste en
  mémoire et chez les pairs. Un bandeau dit « Disque plein : vos modifications sont gardées en
  mémoire et chez les autres personnes. Libérez de la place pour les enregistrer. » Les écritures
  sont retentées avec recul, et la reprise est automatique. Les téléchargements de blobs s'arrêtent
  sous 1 Gio libre ou 5 % du disque.
- **I6 — Un octet abîmé est détecté avant usage** : `IKV1` pour le stockage, Bao pour les blobs,
  attestation pour la synchro. Il est mis en quarantaine, jamais réutilisé.
  - Une **vérification de fond** relit tout le magasin et les blobs une fois par semaine, au rythme
    de 20 Mo/s au plus, pour trouver une corruption avant qu'on en ait besoin.
- **I7 — Aucune boucle sans fin** : tout renvoi est borné en temps (échéance au progrès, ADR 0028 D5)
  et en nombre de reprises par recul. Une boucle détectée est signalée, pas répétée.
- **I8 — Un pair ne peut pas épuiser la mémoire d'un autre** : trames ≤ `MaxFrame`, files bornées
  (déjà fait), réassemblage borné (ADR 0028 D5), document qui dépasse 4 Mi passé en lecture seule
  chez soi (REL-18).

### II. État honnête : ce que l'interface a le droit d'afficher

| Affiché | Preuve exigée |
|---|---|
| « Enregistré » | Go a acquitté l'écriture (I1) ; l'infobulle donne l'heure de la dernière écriture synchronisée sur disque (I2) |
| « À jour avec Paul » | les têtes que Paul déclare (`DocHandle.getSyncInfo`, échange des têtes distantes) **égalent** les nôtres ; jamais déduit d'un message envoyé |
| « Envoi à Paul… (3 s) » | des modifications locales ne sont pas encore couvertes par ses têtes ; on donne leur âge |
| « Paul est hors ligne — vos modifications partiront à son retour » | aucun lien ; nombre de modifications en attente |
| « Lien lent avec Paul » | file d'envoi > 2 s (ADR 0028 D8) |
| « Réception de main.typ : 2,1 sur 6,3 Mo » | fragments reçus (ADR 0028 D5) |

Aucune pastille verte sans preuve. Une incertitude est dite (« état inconnu depuis 30 s »), jamais
arrondie à « tout va bien ».

**Messages de reprise** (exemples, ton du glossaire, zéro jargon) :
- « Inkwell s'est fermé brusquement. Rien n'est perdu : 12 modifications récupérées. »
- « Une copie abîmée de chapitre2.typ a été remplacée par la sauvegarde de 10 h 42 ; les modifications
  plus récentes sont revenues de Paul. »
- « Le disque est de nouveau disponible : tout est enregistré. »

### III. Pannes injectées

| Couche | Pannes | Moyen |
|---|---|---|
| Réseau, déterministe | pertes, doublons, réordonnancement, partitions | `memnet` + `synctest` (existe) ; ajout d'un **débit et d'un délai** par lien, pour tester l'échéance au progrès et les priorités |
| Réseau, réel | délai 50/150/300 ms, gigue, pertes 1/5 %, débit 1/10 Mbit/s, coupure puis retour, relais tué, changement de réseau, veille de la machine | `scripts/netsim` + **`tc netem`** dans les espaces de noms (saute avec la raison si `sch_netem` manque) |
| Processus | `kill -9` à un instant aléatoire pendant la frappe, la compaction, un blob, la projection ; plantage du processus WebKit | e2e ; `TestKillAtRandomTimes` (existe) |
| Disque | perte des écritures non synchronisées (**perte de courant**), écritures déchirées | [LazyFS](https://github.com/dsrhaslab/lazyfs) (FUSE, utilisé par Jepsen), job de nuit Linux |
| Disque | `ENOSPC`, `EIO`, système en lecture seule, `fsync` lent | enveloppe de fautes sur les opérations `os.Root` de `store`, `blob` et `project`, compilée **seulement** avec l'étiquette `faultinject` (jamais dans l'exécutable publié : `scripts/arch-check.sh` le vérifie) |
| Données | bit inversé, fichier tronqué, `.obao` ou carte de reprise abîmés, `.inkwell/` supprimé à chaud | tests Go (existent en partie) + e2e de reprise |
| Horloge | décalage de +2 h (X-20), saut à la sortie de veille | e2e |

### IV. Propriétés de convergence

- **TypeScript (fast-check)** : 3 `Repo` réels, des opérations aléatoires (frappe, suppression, gros
  collage découpé, annulation, mise en forme) et un ordonnancement réseau aléatoire (délai, perte,
  doublon, partition).
  - **Nouveau :** un pair est « tué » puis rechargé depuis son adaptateur de stockage à un instant
    aléatoire.
  - Oracle après guérison et repos :
    - textes et têtes identiques ;
    - toute frappe acquittée avant l'arrêt est présente, sauf si une suppression causalement
      postérieure l'a retirée (chaque frappe porte un marqueur unique, pour le vérifier) ;
    - aucune modification non couverte n'a été admise (ADR 0018).
  - 20 graines en CI, 200 la nuit ; la graine est imprimée à l'échec.
- **Go (rapid)** :
  - fragmentation et réassemblage du mux : pour tout découpage et tout entrelacement de voies,
    chaque voie livre la même suite de messages, et la mémoire reste bornée ;
  - validation groupée : pour tout ordre d'écritures et tout point de coupure LazyFS simulé,
    l'état relu est un préfixe des écritures acquittées.
- **Fuzz** : réassemblage des fragments, décodeur zstd borné (ADR 0028 D6), pages et index de
  `textview`.

### V. Endurance

- **Nuit** (20 min) et **avant chaque version** (8 h) :
  - trois instances et des « dactylos » scriptées (5 caractères/s, rafales, collages, annulations) ;
  - netem variable ;
  - relais redémarré toutes les 30 min ;
  - un pair mis en veille puis réveillé.
- Assertions :
  - convergence à chaque point de contrôle ;
  - pente de la mémoire < 5 Mo/h, par processus Go et WebKit ;
  - goroutines et descripteurs stables ;
  - aucune tâche > 200 ms dans le fil d'affichage ;
  - disque borné (la compaction tourne) ;
  - zéro message « boucle de renvoi » (I7).

### VI. Observation locale seulement

- **Journal** `slog` structuré dans le dossier d'état de l'application : rotation 5 × 5 Mo, `0o600`.
  **Aucun contenu de document, nom de fichier ou jeton.** Un test tape une chaîne témoin et vérifie
  qu'elle n'apparaît dans aucun journal.
- **Journal circulaire des 1 000 derniers événements réseau** (étape 5, déjà prévu) : métadonnées
  seulement (type, taille, voie, pair, latence).
- **Compteurs**, montrés dans « Détails techniques » :
  - histogramme frappe → pair, par personne ;
  - coût d'application ;
  - profondeur des files ;
  - RTT ;
  - type de lien ;
  - mémoire WASM ;
  - dernier `fsync`.
- **« Créer un rapport de diagnostic »** : une archive que la personne **voit avant de l'envoyer**
  elle-même (journaux, compteurs, versions, aucune donnée de document). Aucun envoi automatique.
- **Chiens de garde** :
  - Go attend un battement du pont toutes les 5 s. Après 15 s sans battement, il propose « Inkwell ne
    répond plus — Recharger » : le rechargement relit le stockage, rien n'est perdu (I1).
  - Côté webview, un détecteur d'écart entre images (`requestAnimationFrame`) compte les gels, car
    WebKit n'a pas `longtask`.
- **Mode sans échec** : après trois plantages au démarrage en 2 min, l'application s'ouvre sans
  rouvrir le dernier projet, et propose le rapport de diagnostic.

### VII. Mises à jour

- Le format du stockage est versionné (`IKV1`).
- Chaque version publiée laisse un **corpus de projets** (fixtures) que toute version suivante doit
  ouvrir à l'identique : CI.
- Une version plus ancienne refuse un format plus récent avec un message, sans rien écrire.

### Conséquences

- Bien :
  - la promesse de non-perte devient un texte borné et testé ;
  - l'interface ne ment pas ;
  - on trouve les pertes au courant coupé (LazyFS), que `kill -9` ne montre pas ;
  - on gagne des `fsync` (validation groupée).
- Moins bien :
  - LazyFS et netem ne tournent que sous Linux, la nuit ;
  - l'endurance de 8 h coûte une machine ;
  - l'étiquette `faultinject` crée une variante de build à garder verte.

### Confirmation

Cas REL-01 à REL-20 du plan T11 (« Fiabilité et performance »). Gates :
- `mise run ci` : propriétés à 20 graines, fuzz court, garde `faultinject` dans `arch-check` ;
- job de nuit : 200 graines, LazyFS, netem, endurance de 20 min ;
- avant chaque version : endurance de 8 h.

## Plus d'informations

Complète l'ADR 0020 (politique de `fsync`) et l'ADR 0007 (pannes réelles, endurance), et dépend de
l'ADR 0028 (D5 : échéance au progrès, fragments).

Sources :
- [LazyFS](https://github.com/dsrhaslab/lazyfs) et « When Amnesia Strikes » (Ramos, Kingsbury et
  al., [PVLDB 17, 2024](https://www.vldb.org/pvldb/vol17/p3017-ramos.pdf)) : pertes d'écritures non
  synchronisées, 8 bogues de perte trouvés dans 7 systèmes ;
  [Jepsen local-fs](https://github.com/jepsen-io/local-fs).
- [Pillai et al., « All File Systems Are Not Created Equal », OSDI 2014](https://www.usenix.org/conference/osdi14/technical-sessions/presentation/pillai)
  (`fsync` du dossier après `rename`).
- [fast-check](https://fast-check.dev/) ; [rapid](https://pkg.go.dev/pgregory.net/rapid) ;
  [`tc netem`](https://man7.org/linux/man-pages/man8/tc-netem.8.html).
- automerge-repo `DocHandle.getSyncInfo` (têtes distantes) pour l'état « À jour ».
