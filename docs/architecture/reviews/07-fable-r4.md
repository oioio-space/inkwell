# Revue Fable n°4 de l'architecture (spec 1.0.0-brouillon.5, révision 4, 2026-11-01) — verdict : **ACCEPT** (0 bloquant, 2 majeurs de texte, 5 mineurs)

Portée : `spec.md` (brouillon.5), `CHANGES.md`, `decisions.md`, `coherence.md`, `traceability.md`,
tels que commis en `87bc622`. Non lu : la revue Codex du tour 4. Référentiels : `docs/exigences.md`
(N1–N41), `docs/engineering-principles.md` §3 (« chaque principe a sa preuve »), ADR 0029/0031/0033,
C2SP age (`https://c2sp.org/age`), documentation Landlock du noyau, documentation wazero
(`CompilationCache`), ISO/IEC 25010:2023, Nygard (ADR « proposée »/« acceptée »), TUF (ancres de
fraîcheur). Gravités : **bloquant** = garantie centrale contredite ou porte qui ne peut ni passer ni
échouer ; **majeur** = texte qui doit changer avant qu'un lot s'en serve ; **mineur** = précision.

**Méthode.** Chaque constat du tour 3 (mes 5 majeurs et 15 mineurs, les 4 bloquants et 6 majeurs de
Codex) relu contre la ligne exacte qui prétend le fermer ; chaque porte du §0.5 relue contre ses
règles E-1…E-5 ; recherche de régressions par `grep` sur les formulations retirées (« à décider,
AD-13 », `%LOCALAPPDATA%`, Spotlight, « format d'en-tête age », « DEK par appareil » seule, UTCP,
ACP « expérimentation », « 16 greffons en concurrence ») ; vérification que `docs/protocol/`,
`docs/design/`, `docs/adr/` n'ont pas bougé (`git show --stat 87bc622`).

---

## 1. Mes constats du tour 3 : tous fermés

| # | Constat | Où c'est fermé | Vérifié |
|---|---|---|---|
| R3-M1 | P-IDX non mesurable | §0.5 : corpus **versionné** (projet d'exemple + deux projets Typst publics, 200 requêtes écrites avant l'essai, jugements à trois niveaux), **rappel@10 ≥ 0,8 et MRR ≥ 0,6** pour R0, R1/R2 seulement à +0,1 ; retard ≤ 5 s ; corruption/reconstruction ; `AD-89` rév. 4 | oui — la porte décide maintenant |
| R3-M2 | Porte ACP indéfinie | §0.5 « Reporté, pas expérimenté » ; ACP-1 « reporté après v1 (§0.5 : report, pas expérimentation) » ; `AD-82` rév. 4 ; `coherence.md` §5 « Après v1 — ACP » | oui, cohérent aux trois endroits |
| R3-M3 | Repli nommé à l'avance | colonne « Repli nommé à l'avance » du §0.5, une entrée par ligne ; E-4 (« jamais un seuil assoupli ») | oui ; **une** entrée est contradictoire (R4-M1 ci-dessous) |
| R3-M4 | Mesure de référence N38 en dernier | `coherence.md` §5 : lot **« Mesure de référence (N38) »** en tête du MVP ; §12.5 PM-5 le dit ; « Distribution » garde N38 pour les preuves WAN et gros projets seulement | oui |
| R3-M5 | Budget du dossier de travail | EN-9 : dépendances réelles (`typst compile --deps` + fichiers ouverts), tmpfs `size=`, **≤ 512 Mio**, refus avec message, « jamais un débordement silencieux » ; rémanence dans EN-7 | oui |
| R3-M6 | Sémantique d'« Intégrer » | HI-6 **différence appliquée** choisie ; « Annuler » = ce groupe exact (opérations inverses) ; intégration répétée depuis le dernier point ; zones verrouillées refusées ; fusion native **écartée** avec sa raison (règle d'admission → amendement d'ADR 0031) ; écart du code au §16 ; `AD-96` rév. 4 | oui — c'est le choix que je tiens pour le plus sûr sous le modèle byzantin |
| m-1 | `clone3` « se rabat » | H-5 (c) « refus préventif » | oui |
| m-2 | Landlock masqué par seccomp | H-0 (3) : Landlock vérifié (`EACCES`) **avant** seccomp (`EPERM`) | oui |
| m-3 | Signatures hors protocole non listées | §9.4 liste **fermée** `plugin-package`, `backup-head`, `export`, `enc-key` ; `enc-key` utilisé par EN-2 | oui |
| m-4 | C14 périmé | `coherence.md` C14 : « déjà dans `ux-principles.md:14`, rien à y corriger » ; §6 point 6 corrigé | oui |
| m-5 | `.inkwell/git/` « à décider » | `[cible, AD-13]` (§6.3 table, `internal/milestone`) ; aucun « à décider, AD-13 » restant | oui |
| m-6 | Bouton puzzle permanent | §15.13 : « affichée seulement quand au moins une extension tierce ou de projet est installée » | oui |
| m-7 | N34 absent du phasage | `coherence.md` §5 v1 « Modèles et écriture » : N34 couche personnelle | oui |
| m-8 | Blob non téléchargé annoncé | IG-5 ligne « blob admis » : « ne l'annonce jamais … vecteur » | oui |
| m-9 | DEK par appareil ou par personne | EN-2 « une par projet **et** par appareil », enveloppe pour l'appareil et le secours seulement ; `AD-91` | oui |
| m-10 | Horloge du voyage | HI-10 : ordre de réception local, heure déclarée marquée | oui |
| m-11 | Budgets manquants | §12.5 : voyage ≤ 200 ms, diff par mots ≤ 1 s, motifs ≤ 20 ms, Argon2id ≤ 500 ms, rechiffrement ≥ 50 Mo/s par cœur | oui |
| m-12 | Rejouer les portes d'enfermement | E-5 | oui |
| m-13 | Verbes par classe | RS-3 + table RS-8 (le cœur : « annuler un travail seulement ») | oui |
| m-14 | BLAKE3 par architecture | §12.5 : ≥ 500 Mo/s amd64 (AVX2), ≥ 300 Mo/s arm64, à confirmer | oui |
| m-15 | Index « chiffré » | RG-3 : dans le dossier de travail (EN-4) pour un projet protégé, « jamais ailleurs » ; EN-5 le répète | oui — plus fort et sans code, comme demandé |
| sur-ing. | Kit régénéré à chaque sauvegarde | ST-8 : **proposé** après 30 jours ou sur geste | oui |
| amélioration | Git dans le coffre | EN-5 : go-git sur `billy.Filesystem` au-dessus du magasin chiffré, aucune matérialisation | oui ; à mesurer par P-GIT-BENCH **dans** le coffre aussi (déjà couvert par « budgets du §12.5 » de P-VAULT) |

---

## 2. Les quatre bloquants de Codex : fermés, et je souscris aux remèdes

**R3-B1 — dossier de travail en clair.** EN-4 (table par cible) : Linux = tmpfs privé **et** absence
de swap en clair, sinon LUKS vérifié, sinon refus ; macOS = disque en mémoire sans pilote (`ram://`,
à prouver), sinon FileVault vérifié (`fdesetup status`), sinon refus ; Windows = BitLocker vérifié,
sinon refus. « Refus » est défini (aperçu, export lisible, LSP des fichiers fermés indisponibles ;
édition, synchronisation, export chiffré par flux age gardés). EN-7 conditionne la garantie à EN-4 ;
P-VAULT ajoute « inspection hors ligne **avant** relance ». Le chiffrement du disque est devenu une
**condition vérifiée**, pas une recommandation — exactement la fermeture que Codex demandait, et la
bonne (ADR 0029 : une garantie se vérifie ou ne se promet pas). Reste un point de mesurabilité
(R4-m1). **Fermé.**

**R3-B2 — DEK et age.** EN-2 : la DEK de 32 octets est la **charge utile** d'un fichier age complet
écrit par `filippo.io/age` (clé de fichier de 16 octets, stanzas X25519 inchangées, C2SP), objet
versionné `keyring/<époque>.age`, clair CBOR `{v, project, peer, epoch, dek}` vérifié au
déchiffrement, deux destinataires (appareil, secours), vecteurs nommés. age n'a pas d'AAD : lier le
projet et l'appareil **dans le clair** et les vérifier est la seule manière correcte, et c'est celle
retenue. **Fermé** — avec une conséquence à écrire sur la récupération (R4-M2).

**R3-B3 — versions et ménage.** §6.5 ajoute aux racines « le manifeste d'une version nommée gardée
ou d'une variante non abandonnée » ; HI-9 pose l'épingle **avant** la publication du manifeste, la
retire seulement à la suppression (attribuée, annulable), déclare qu'un moment quelconque n'épingle
rien, et fait gagner la **purge** (« incomplète (purge du 3 mars) », rien ressuscité). Tests
nommés (V1/logo A→B/31 jours/restauration hors ligne/reconstruction Git/purge). C'est l'ordre
d'écriture correct (racine avant référence, comme un `fsync` de l'inode avant celui du répertoire).
**Fermé**, un cas de bord à écrire (R4-m2).

**R3-B4 — portes.** E-1…E-5 normatives ; chaque ligne du §0.5 a des entrées figées, un oracle, un
seuil et un repli ; « 16 installés / 6 actifs / 10 en attente » remplace la phrase impossible
(annexe alignée) ; sidecars entrés dans P-SANDBOX ; P-VAULT hors ligne ; P-IDX chiffrée ; P-FRAME
« sur chaque moteur » ; ACP sorti. E-2 (« une porte non mesurable échoue ») est la règle que je
demandais au tour 3 (échec fermé). **Fermé**, sauf une entrée de repli inapplicable (R4-M1).

---

## 3. Régression : aucune trouvée

- `docs/protocol/`, `docs/design/`, `docs/adr/` intouchés (stat du commit : cinq fichiers de
  `docs/architecture/` seulement).
- Aucune réapparition des formulations retirées (greps listés en méthode) ; UTCP n'apparaît plus
  qu'en « écarté » (§15.12, §15.18) ; ACP n'apparaît plus qu'en « report ».
- Cohérence interne vérifiée : §2 (hôtes ≤ 40 Mio au repos, somme de mémoire linéaire ≤ 1,5 Gio =
  6 × 256 Mio de H-7, budget global 2,5 Gio en RSS/PSS, VSZ à part) ↔ H-7 ↔ RS-8 (320 Mio = 64 +
  256) ↔ PM-3 ; H-4 (≤ 6 actifs, refus) ↔ §0.5 ↔ annexe (16 installés) ; EN-4 ↔ EN-5 ↔ RG-3 ↔
  EN-7 ↔ P-VAULT ; §6.5 ↔ HI-9 ↔ §15.10 (paquets racines) ; §0.5 ↔ `coherence.md` §5 (lots,
  N34, N38, ACP) ; §9.4 ↔ EN-2/EN-6/ST-5/ABI-7 (quatre préfixes, tous utilisés).
- Le §16 enregistre l'écart « `A.merge` aujourd'hui, différence appliquée demain » : la spec ne
  prétend pas que le code la respecte déjà.

---

## 4. D-10 à D-16 : d'accord tel qu'enregistré

| # | Enregistré (CHANGES.md, decisions.md) | Position |
|---|---|---|
| D-10 | un hôte par greffon actif, paresseux, ≤ 6 ; éviction jamais pendant un mandat ou un appel (`AD-87` rév. 4) | d'accord (je l'avais rejoint au tour 3) ; le repli d'activation est à corriger (R4-M1) |
| D-11 | deux temps (Landlock puis `execve`), seccomp `TSYNC` principal, Landlock vérifié avant seccomp (H-0) | d'accord ; conforme à la doc Landlock (« portée par fil avant ABI 8, héritée par `execve` ») |
| D-12 | Rust et Go en 1.0, TinyGo en 1.1 ; minuterie positive et annulation dans le kit (ABI-9, P-PLUG-PROTO) | d'accord |
| D-13 | UTCP retiré | d'accord |
| D-14 | expérimentations acceptées à leur porte, sans dépendance de production, repli nommé (`AD-95`, E-1…E-5) | d'accord ; c'est la pratique « ADR proposée » de Nygard, rendue exécutable |
| D-15 | `beautiful-mermaid` candidat 1.1 derrière P-MERMAID (seuils chiffrés) ; Mermaid complet derrière P-FRAME par moteur | d'accord (ma position révisée du tour 3) |
| D-16 | préfixes `inkwell-ext:`, PD-2 retiré, liste fermée §9.4 | d'accord |

Le « choix de conception signalé » (différence appliquée plutôt que fusion native) : je le
**soutiens**. Sous ADR 0031 (modèle byzantin complet, choix de l'utilisatrice du 2026-10-31), faire
entrer tels quels des changements admis pour un autre `DocId` ouvrirait une règle d'admission
transitive que personne n'a revue ; l'attribution fine survit dans l'histoire de la variante, liée.
Patchwork fait l'autre choix parce qu'il n'a pas d'adversaire dans le modèle.

---

## 5. Constats nouveaux de la révision 4

### R4-M1 — Majeur (texte) : le repli d'activation de P-PLUG-BENCH contredit H-3

**Preuve.** §0.5, ligne « Hôte des greffons » : « si l'activation dépasse 500 ms : module **compilé
à l'installation** et gardé dans l'hôte tant qu'il vit, … **jamais** de cache disque » ; `AD-87`
rév. 4 répète. Or H-4 crée l'hôte **paresseusement** à la première activation, et H-3 interdit tout
cache de compilation sur disque. Un module compilé « à l'installation » n'a nulle part où vivre
jusqu'à la première activation : wazero ne conserve le code natif qu'en mémoire d'un processus
(`wazero.NewCompilationCache`) ou dans un répertoire (`NewCompilationCacheWithDir`), c'est-à-dire
le cache disque refusé (B-3 du tour 1, H-2/H-3). Le repli est donc inapplicable tel qu'écrit ; sous
E-4, une porte dont le repli n'existe pas revient à « nouvelle revue », ce qui n'est pas ce que la
ligne promet.
**Norme.** E-1/E-4 (repli nommé **et** applicable), `engineering-principles` §3.
**Correction.** Nommer un repli qui tient H-3 et H-4 : (a) **préchauffage** — pour les greffons
que la personne a épinglés ou activés au dernier usage, l'hôte est créé et le module compilé **en
tâche de fond après la première peinture** (§12.5 « 0 ms avant la première peinture »), dans le
budget des 6 actifs ; l'activation « froide » reste possible et affichée (« Extension en
préparation… ») ; ou (b) si le préchauffage ne suffit pas, **revue** de H-3 par ADR (cache disque
scellé, haché par le parent, dans le coffre pour un projet protégé) — jamais implicite. Retirer
« compilé à l'installation ».

### R4-M2 — Majeur (texte) : « refus d'un objet d'un autre appareil » contre « récupération sans l'ancien appareil »

**Preuve.** EN-2 : le clair `{v, project, peer, epoch, dek}` fait refuser « un objet d'un autre
projet **ou d'un autre appareil** ». P-VAULT (§0.5) exige « récupération **sans** l'ancien
appareil » : un appareil neuf (nouveau `PeerID`, trousseau perdu, disque conservé) déchiffre
`keyring/<époque>.age` par le destinataire de secours et trouve `peer` = l'ancien appareil. La règle
telle qu'écrite refuse la récupération légitime ; le vecteur ne peut pas passer.
**Norme.** E-2 (une porte non mesurable ou contradictoire échoue) ; ADR 0029 (non-perte).
**Correction.** Écrire la **voie de récupération** dans EN-2/EN-3 : le refus « autre appareil »
vaut pour le destinataire **appareil** ; par le destinataire de **secours**, le clair est accepté si
`project` correspond, l'ancien `peer` est affiché (« copie chiffrée par *Portable de Paul* ») et la
DEK est aussitôt **ré-enveloppée** pour le nouvel appareil sous une **nouvelle époque** (l'ancienne
enveloppe reste lisible par le secours). Ajouter le vecteur « secours sur appareil neuf, époque + 1,
ancien appareil refusé ensuite ».

### R4-m1 — Mineur : oracles de « vérifié » d'EN-4 non nommés

EN-4 dit « vérifiés au déverrouillage » (swap Linux, LUKS, BitLocker) mais ne nomme l'oracle que
pour FileVault (`fdesetup status`). Pour qu'un faux « vérifié » soit testable (P-VAULT, E-2), nommer
chaque test : Linux — `/proc/swaps` vide ou chaque entrée sur `zram*`/`dm-crypt` (`/sys/block/dm-*/dm/uuid`
commençant par `CRYPT-`), et le périphérique sous le dossier de type `crypt` dans `lsblk` ;
macOS — `sysctl vm.swapusage` contient `(encrypted)` (le disque en mémoire **se pagine** : la ligne
macOS doit aussi exiger le swap chiffré, comme la ligne Linux) ; Windows —
`Win32_EncryptableVolume.ProtectionStatus = 1` sur le volume du dossier. Ajouter à P-VAULT un cas
« support annoncé chiffré mais ne l'est pas → refus ».

### R4-m2 — Mineur : version incomplète **dès la création**

HI-9 ne définit « incomplète » que pour une purge. Un blob déjà parti du ménage local (ou jamais
téléchargé, IG-5) peut manquer **au moment** où la personne « garde ce moment comme version » ; et
l'épingle posée sur l'appareil A n'atteint l'appareil B qu'après synchronisation du document racine,
pendant quoi B peut avoir ménagé. Dire : avant de publier le manifeste, les blobs manquants sont
demandés aux pairs ; ceux qu'aucun pair n'a rendent la version « incomplète (image `x` absente) »
dès sa création, et la vue le dit ; un appareil qui reçoit un manifeste épingle **ce qu'il a** et
va chercher le reste (règle de `blob.Service`). Test à ajouter à la liste d'HI-9.

### R4-m3 — Mineur : `AD-96` garde une recommandation périmée

`decisions.md` `AD-96`, paragraphe « Recommandation », dit encore « fusion Automerge native » ; la
matrice (ligne `AD-96`) et le point « Révision 4 » disent « différence appliquée ». Convention du
journal, mais un lecteur pressé lit la première phrase : marquer la recommandation initiale
« remplacée par la révision 4 ».

### R4-m4 — Mineur : HI-6 « comme une proposition (§15.5) »

§15.5 définit la proposition pour un **auteur non humain** (greffon, IA, MCP, agent). Pour une
personne qui **peut écrire** l'original, dire « prévisualisée avec la même présentation (diff par
morceau), puis appliquée par elle » ; « Proposer d'intégrer » reste la voie de qui ne peut pas
écrire. Évite de faire passer une intégration humaine par la règle des deux (RF-*), qui ne la
concerne pas.

### R4-m5 — Mineur : sidecars dans P-SANDBOX sans chiffre

§0.5 « sortie disque cumulée bornée » pour les sidecars ne cite pas la borne ; E-1 veut un seuil
avant la mesure. Citer la valeur de §9.5 (ou la fixer là) et la reprendre dans la ligne.

---

## 6. Ce qui est bien et qu'il ne faut pas toucher

E-1…E-5 (surtout E-2 « non mesurable = échec » et E-5 « rejouer à chaque majeure ») ; la table par
cible d'EN-4 avec « refus » défini ; la DEK en charge utile age (aucun format maison) ; l'épingle
avant le manifeste ; la purge qui gagne ; l'index dans le dossier de travail ; Git dans le coffre
par `billy.Filesystem` ; la différence appliquée ; RS-8 et sa réserve du cœur ; PM-1…PM-6 ; le lot
« Mesure de référence » en tête du MVP.

---

## Verdict : **ACCEPT**

Mes 5 majeurs et 15 mineurs du tour 3 sont fermés dans le texte ; les 4 bloquants de Codex sont
fermés par des remèdes que je tiens pour corrects ; aucune régression ; D-10 à D-16 convergés et
approuvés. Deux majeurs **de texte** pour la révision 5, au fil de l'eau, sans nouveau tour
bloquant : **R4-M1** (repli d'activation applicable, sans « compilé à l'installation ») et
**R4-M2** (voie de récupération par le secours, ré-enveloppe à une nouvelle époque). Mineurs
R4-m1…m5 au fil de l'eau. Acceptation conditionnée à rien d'autre.
