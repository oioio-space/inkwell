---
status: proposed
date: 2026-10-31
decision-makers: [Mathieu]
---

# Frontière d'admission avant le `Repo`, et chaînes de documents signées par écrivain

## Contexte et problème

L'audit du 31/10/2026 (S1, S1b, R1) montre que des octets d'un pair créent de l'état dans
automerge-repo 2.5.6 **avant** toute règle de partage. Constats, avec preuves dans
`docs/plans/audit-2026-10-31/` (04, 05, 09) :

- un message éphémère par identifiant de session laisse une entrée à vie (`NetworkSubsystem.js:53-57`).
  Un lecteur a ainsi fait retenir environ 114 Mio ;
- chaque document demandé est noté avant le contrôle d'accès, sans jamais être oublié
  (`CollectionSynchronizer.js:87-96`) ;
- chaque `storageId` annoncé écrit un état de synchro sur disque. Ces sauvegardes sont différées et
  annulées d'un document à l'autre (`throttle.js:80-104`) ;
- les messages d'un document en chargement s'accumulent sans borne (`DocSynchronizer.js:259-266`) ;
- `removeFromCache` attend puis échoue sur une poignée non prête et la garde en mémoire (mesuré :
  environ 66 Kio de RSS chacune) ; `flush` lève sur une telle poignée (`Repo.js:706`), et un
  chargement tardif réinscrit le synchroniseur (`Repo.js:499`) ;
- un message de synchro est un seul paquet compressé, appliqué d'un bloc (sonde `split.mjs`, 09) ;
- le premier correctif, qui ne partageait que les documents tenus, a cassé la réplication des
  variantes créées après l'arrivée (e2e ZZ-2).

Décision de l'utilisatrice N7 : modèle byzantin complet. Même une personne invitée qui écrit, et qui
est malveillante, ne doit jamais gêner les autres. Lecteurs et pairs sans attestation sont strictement
bornés. N8 : l'étape 1 est fusionnée tout de suite (fait : `ad2cf86`, `f2f1460`), et S1 reste critique
jusqu'à la garde de l'ADR 0030.

## Facteurs de décision

- Aucun état par pair ou par document sans borne numérique et testée (ASVS 5.0 V2.4.1, §4).
- Tout effet d'un écrivain est **attribuable** et **borné par écrivain**. Aucun plafond partagé
  qu'un seul écrivain, ou quelques écrivains de connivence, puissent remplir.
- Déterminisme : le même ensemble admis chez tous, quel que soit l'ordre d'arrivée.
- Aucune perte de données : rien n'est effacé du disque tant qu'une annulation reste possible.
- Go ne lit pas Automerge (ADR 0002, 0011). Une seule source de vérité par chose.
- La réplication immédiate de tout document du projet est conservée (« rejoindre = tout recevoir »).
- Préférer la conception la plus simple qui tient la même règle.

## Options envisagées

**Où appliquer**
- Politique de partage du `Repo` seule. L'état est créé avant elle.
- Filtrage par Go du format d'automerge-repo. Cela lit un format applicatif, contre les ADR 0002 et
  0011, et suit ses changements (2.6, 3.0).
- Correctif local d'automerge-repo. Un correctif limité à `removeFromCache` ne suffit pas (revue 3) ;
  un correctif exact touche quatre endroits du chargement, à reprendre à chaque version.
- **Frontière dans l'adaptateur, pairs éphémères, aucun correctif** : on n'évince que des poignées
  prêtes, et les poignées non prêtes sont bornées par l'ensemble admis.

**Qui décide qu'un document appartient au projet**
- La liste `meta.variants` tronquée : un seul écrivain la remplit ou en efface des lignes, et rien
  n'est attribuable.
- L'arborescence disque : elle exclut les variantes, ce qui recrée R1.
- Des op de la liste de membres : écarté par l'ADR 0030 (O3).
- Des entrées dans le journal de décisions : un écrivain scellerait le journal des admins.
- Des jetons numérotés désignant un identifiant quelconque : l'équivoque fait croître les
  identifiants sans borne, et l'alias entre écrivains reste possible.
- Un identifiant dérivé de la position sans l'ancre (proposition de la revue 3) : en cas
  d'équivoque, un même identifiant aurait deux ancres, et l'ancre imposée dépendrait de l'ordre
  d'arrivée.
- **Une chaîne d'annonces par écrivain, un identifiant dérivé de la position et de l'ancre, et
  l'adoption par les co-auteurs.**

**Révocation et quotas**
- `EverWriter` : un écrivain retiré émet encore, en antidatant.
- Des quotas lus sur les décisions « debout au moment de la réception » : le verdict dépend du moment.
- **Rang causal, comme D1 de l'ADR 0030, et frontière de décisions dans chaque annonce.**

**Purge**
- Effacer dès que la purge tient localement : une annulation tardive trouve le disque vide.
- Ne jamais effacer : contraire à l'intention d'une purge.
- Effacer après un délai fixe : une coupe gardée hors ligne plus longtemps annule encore.
- **Deux phases : pierre tombale logique, puis effacement quand la purge est stable.**

**Équité du traitement**
- Seau de 16 Mio/s par pair : ne rend pas la main pendant une application indivisible.
- File équitable pondérée : coût en O(log n), sans gain à quelques pairs.
- Découper les messages de synchro : impossible, un message est un seul paquet.
- Deux `Repo` (documents ouverts, le reste) : isolation forte, mais coûteuse ; gardée en recours.
- **Tourniquet à déficit (DRR) sur le coût mesuré, files par pair bornées, et deux jetons de
  grosse trame servis à tour de rôle.**

## Décision

### 1. Frontière d'admission

Chaîne de réception : trame Go → décodage strict → **admission** → couverture (ADR 0018) → garde
structurelle (ADR 0030) → `Repo`.

- **Types admis.** Seuls `arrive`, `welcome`, `sync`, `request`, `ephemeral` et `doc-unavailable`
  passent, champ par champ, avec leurs bornes. Le message est reconstruit. Un document hors de
  l'ensemble admis est jeté sans réponse.
- **Pairs.** Go ne transmet à la webview que les messages des appareils membres ; la table des pairs
  de la webview ne contient que des appareils membres (fermeture de S1-R6 : des identifiants inconnus
  ne peuvent plus occuper la table pendant une invitation).
- **Éphémères.** Présence seulement : canaux `name`, `anchor`, `head`, au plus 1 Kio, 64 par seconde
  et rafale de 128 par pair. Session et compteur sont réécrits par l'adaptateur. Aucun relais sortant.
- **Pairs éphémères.** Tout pair est déclaré éphémère et son `storageId` est ignoré : aucun état de
  synchro n'est écrit. Ceux d'avant sont effacés par Go avant que la webview s'attache.
- **Contre-pression.** Les messages d'un document pas encore prêt attendent dans une file réservée
  par pair : 4 par document, 8 Mio plus une trame par pair, sans réserve commune. Un débordement
  remet le lien à zéro, avec un délai de 30 s doublé jusqu'à 10 min, gardé par appareil d'une
  connexion à l'autre.
- **Amorçage.** Une voie par propriétaire (le projet, les documents ouverts, chaque auteur ou
  adoptant), au plus un chargement en cours par voie, sans plafond global ; l'attente d'un
  chargement est abandonnée au bout de 10 s ; les chargements abandonnés restent dédupliqués par
  identifiant.
- **Équité.**
  - Go : réassemblage réservé de 4 Mio par pair, au-delà dans un fichier temporaire par pair (plus de
    réserve commune où un pair rapide évince un pair lent) ; files par pair bornées ; livraison à la
    webview par tranches de 1 Mio, à tour de rôle ; crédit de 2 Mio par pair accordé par la webview.
  - Grosses trames : deux jetons pour tout le processus, servis dans l'ordre d'arrivée, un seul par
    pair, tenus seulement le temps du traitement local et rendus par `Done`, la déconnexion ou au bout
    de 30 s.
  - Webview : tourniquet à déficit sur le temps mesuré (quantum 8 ms), une voie par document dans la
    file d'un pair, l'interactif d'abord avec au moins un tour sur quatre pour le fond (ADR 0028 D5).
    Un même mécanisme sert la garde (ADR 0030 §5).
  - L'application d'un message est indivisible ; sa taille est bornée par le plafond de croissance de
    l'ADR 0028 D1 (512 Ki avant le Worker, 4 Mi après).
- **Résidence.** En mémoire restent les documents ouverts, plus au plus 16 autres documents prêts et
  un budget d'op (`A.stats`) étalonné, puis, avec le Worker de l'ADR 0028, la mémoire WASM mesurée et
  le recyclage. On n'évince qu'une poignée prête, inactive depuis 1 s : `flush`, nouvelle
  vérification, puis `removeFromCache`. Les poignées non prêtes ne sont jamais évincées ; il y en a au
  plus une par document admis.
- **Un seul adaptateur par `Repo`**, vérifié par identité d'objet.

### 2. Chaînes d'annonces (`internal/listing`)

- **Annonce.** Une op d'annonce porte : projet, `Prev` (l'annonce précédente de l'auteur),
  `Members` (frontière de la liste vue, au plus 64), `Decisions` (frontière du journal de décisions
  vue, au plus 64), position, sorte (`variant`, `adopt` ; `node` réservé à l'index de l'ADR 0023),
  ancre (le hachage d'une modification d'ancrage du nouveau document) ou op adoptée, auteur. Elle est
  signée Ed25519 sur le tuple entier.
- **Identifiant.** `DocId = BLAKE3("inkwell/doc/v1" ‖ projet ‖ auteur ‖ sorte ‖ position ‖ ancre)[:16]`.
  Il est toujours recalculé, jamais reçu. Une seule ancre par identifiant.
- **Admission d'une op.**
  - L1 : canonique et signée pour le projet.
  - L2 : passé connu, sinon attente dans un tampon de 4 par écrivain debout, qui n'évince que les
    siennes.
  - L3 : position exacte.
  - L4 : l'auteur écrit et n'est pas suspendu sur `past(Members)`.
  - L5 : nombre de variantes ≤ `Q = 32 + 32 × grants`, où `grants` compte les décisions
    `quota/<clé>` **du passé de `Decisions`** qui tiennent dans le pli. Une adoption vérifie L1, L4 et
    L5 sur l'op adoptée, et n'adopte qu'une fois une même position d'un même auteur.
- **Équivoque.** Deux op d'un auteur à la même position gèlent sa chaîne à partir de cette position,
  partout. La preuve est gardée et diffusée.
- **Adoption.** Un document gelé, mort ou privé de son quota reste admis tant qu'un écrivain debout
  l'adopte. Go adopte automatiquement les documents que cet appareil a modifiés (il les connaît par
  ses propres attestations). L'adoption ne coûte rien au quota de l'adoptant, et ne ressuscite jamais
  un document purgé. Ainsi l'équivoque d'un écrivain ne fait plus disparaître ce que d'autres ont
  écrit.
- **Rang.**
  - Les op de membres qui coupent, suspendent ou enregistrent le départ de y portent
    `SeenListing(y)` : le bout de la chaîne de y vu par l'auteur. C'est une entrée typée du `Seen`
    de l'ADR 0030.
  - Une annonce de y tient si y écrivait à `past(Members)`, si aucune coupe de y ne lui est
    concurrente, si L5 tient avec les quotas debout, et si elle est sous le point de gel.
  - Le pli est pur, recalculé à chaque changement des op de membres, de décisions ou d'annonces.
  - Une annonce morte n'efface rien. Son document est montré « à confirmer », puis adopté par un
    co-auteur, ou réadmis par une décision `doc/<id> = kept`.
- **Ancrage (G11, dans le module de la garde).**
  - Rien d'un document n'est accepté avant son ancre. Le premier message est traité d'un bloc : on
    remonte depuis l'ancre, par les dépendances, à l'intérieur du message ; tout le reste doit en
    descendre ; sinon le message entier est refusé.
  - L'ancre est acceptée si son hachage est celui que l'auteur a signé et si elle passe G1 à G10.
  - Ensuite, une modification n'est admise que si l'ancre est dans son passé.
  - Une histoire rivale, ou un descendant d'une rivale, est refusée partout.
- **Ensemble admis.** Il vaut : racine ∪ index G ∪ documents des annonces debout sous le point de gel
  ∪ documents adoptés ∪ documents `kept` issus d'une annonce détenue, moins les documents purgés.
  - Go vérifie, persiste (`.inkwell/listing/`), synchronise sur le canal trust et envoie l'ensemble à
    la webview (`Listed`, avec projet et époque).
  - Le même ensemble sert à l'admission, au partage, au chargement et aux attestations.
  - `meta.variants` ne sert plus qu'à l'affichage. Tout document admis a une ligne dans l'historique,
    même sans entrée.
  - Un document d'un autre projet n'est jamais admis. Importer, c'est copier dans un nouveau document
    annoncé.
- **Purge en deux phases.**
  - Une décision `doc/<DocId> = purged` (ou `node/<id>`) est **absorbante** : une décision qui la voit
    ne peut plus la changer. Seule une op **concurrente** peut l'annuler.
  - Phase 1, pierre tombale logique : le document sort de l'ensemble admis, ses octets restent.
  - Phase 2, effacement quand la purge est **stable** : chaque appareil admin debout a publié un
    accusé (`Ack`, le dernier par appareil) dont le passé causal joint contient la purge ; plus un délai
    de grâce optionnel. L'identifiant est d'abord inscrit dans la liste des effacés (le magasin refuse
    alors toute écriture dessous), puis le dossier est supprimé.
  - Le même verrou s'applique au ménage des blobs de l'ADR 0023.
  - La position n'est pas rendue. Le rejeu est sans effet.
- **Pont.** `Listed`, `AttestRefused` et `Done` ; tout rappel différé porte l'époque.

### 3. Ordre

- **Étape 1** (fusionnée) : la frontière, avec un ensemble provisoire = racine + `meta.variants`, au
  plus 256 documents par session et collants. S1 reste critique (N8).
- **Étape 2** : l'ADR 0030, avec ce que la présente décision exige d'elle :
  1. table des pairs limitée aux appareils membres (S1-R6) ;
  2. plafond de croissance de l'ADR 0028 D1, s'il manque ;
  3. garde G1 à G10 ;
  4. **G12, liaison acteur ↔ auteur** : chaque modification porte l'empreinte de la clé de son auteur
     (champ `author` d'Automerge 3.5, compris dans le hachage), tous les changements d'un acteur
     portent le même auteur, et la couverture n'accepte que l'attestation de cette clé. L'histoire
     antérieure passe par une décision `legacy/<doc>` unique ;
  5. journal de décisions, `Seen`, passé causal joint, purge absorbante, accusés, effacement en deux
     phases ;
  6. ordonnanceur équitable (webview et Go).
- **Étape 3** : les chaînes (annonces, adoption, `Decisions`), `SeenListing`, le pont, la création de
  variante avec ancre, G11, l'adoption automatique, les voies d'amorçage, la résidence, puis la
  pagination de `LoadRange` si l'hypothèse H-1 se confirme.

### Conséquences

**Bien**
- Un écrivain ne peut ni déplacer, ni cacher, ni épuiser les documents d'un autre, ni faire
  disparaître ce qu'un autre a écrit dans les siens.
- Ses effets sont signés et bornés : au plus `Q_w` identifiants à lui, et une adoption par position et
  par co-auteur.
- Lecteurs, pairs sans attestation et inconnus n'ont plus d'état non borné, ni dans l'adaptateur ni
  dans le `Repo`.
- Plus aucun état de synchro sur disque, et aucun correctif d'automerge-repo.
- Rien n'est effacé du disque tant qu'une annulation honnête reste possible.
- Aucune ressource commune n'est tenue pendant un transfert réseau ; les deux seules ressources
  communes sont servies à tour de rôle.

**Moins bien, assumé**
- **Reste critique** : un écrivain qui publie deux contenus pour **son propre** `(acteur, seq)` divise
  les documents où il écrit, y compris ceux d'autrui. Sans consensus, aucune règle déterministe ne
  tranche (Kleppmann 2022, §2.3.1). Avec G12, la preuve est signée deux fois par lui, ses
  modifications ne sont plus relayées, l'interface le nomme, et son retrait l'arrête. On récupère le
  document en le copiant à une version antérieure dans une nouvelle variante. Une ADR devra décider
  s'il faut aller plus loin.
- Un document peut rester « à confirmer » entre un gel et l'adoption par un co-auteur.
- Un appareil admin absent bloque l'effacement (de la place disque, rien de perdu) jusqu'à son
  retrait.
- L'application d'un message reste indivisible : un pair peut retarder les autres d'au plus une
  application, bornée par l'ADR 0028 D1.
- Une reconnexion coûte environ 1,25 octet et 2 µs par modification et par document de plus. C'est
  une mesure indicative, à confirmer par le banc répété.
- Il faut une modification d'ancrage par document créé, et un appel à Go avant `import`.
- La mémoire des attestations croît comme le carré du nombre d'écrivains : à surveiller.
- 32 variantes par écrivain est une hypothèse (`docs/open-questions.md`). La relever reste compatible.
- Écrire du contenu nuisible dans un document partagé reste possible à un écrivain courant. C'est
  attribuable, réversible par l'historique, et le retrait de la personne l'arrête.

**À trancher par l'utilisatrice** (détail : `06-admission-design.md` §11)
- quota de base et taille d'un octroi (32) ;
- délai de grâce avant effacement (proposé : 30 jours ; 0 est sûr) ;
- croissance d'historique par écrivain : détecter et signaler (recommandé) ou bloquer ;
- purge irréversible pour les variantes et les nœuds ;
- adoption automatique sans demander (recommandé) ;
- histoire antérieure sans auteur : décision `legacy/<doc>` ou remise à zéro.

### Confirmation

- **Tests de contrat** sur automerge-repo épinglé :
  - clé de session ; `hasRequested` noté avant l'accès ; `isEphemeral` ; sauvegarde différée ;
  - `removeFromCache` et `flush` sur une poignée non prête (documentent pourquoi on ne le fait pas) ;
  - éviction d'une poignée prête, rechargement, modification, puis rechargement depuis le disque :
    aucune modification perdue (compactage) ;
  - `import` avec `docId` ; message de synchro en un seul paquet ; champ `author` compris dans le
    hachage.
- **Étape 1** : décodeur strict (table et fast-check) ; P1, P2 et P3 bornés (comptes par espions et
  tas) ; file et amorçage avec un stockage bloqué ; e2e ZZ-1 et ZZ-2 dans `history.spec.ts`.
- **Étapes 2 et 3** : un test par borne, au seuil, juste au-delà, et pour le retour à la normale
  (tableau 8.2 de `06`), plus :
  - équivoque après une contribution honnête (le document reste, adopté) ;
  - inondation d'équivoques sur 3 redémarrages ; alias entre écrivains ;
  - quotas : annonce avant ou après l'octroi, octroi annulé par une coupe concurrente ;
  - G11 : livraison fragmentée, parent rival, descendant qui usurpe un acteur ;
  - purge : coupe tardive, `kept` concurrent, redémarrage entre les phases, écriture déjà engagée,
    appareil absent ;
  - amorçage avec 40 voies hostiles bloquées ;
  - équité multi-pairs sur horloge réelle (p99 interactif ≤ 2 × base + une application) ;
  - jetons de grosse trame : ordre, un par pair, pair bloqué ; pic de RSS mesuré ;
  - identifiants inconnus pendant une invitation (S1-R6) ;
  - banc de reconnexion répété (≥ 10 exécutions).
- **Go** : `FuzzDecode` de `listing`, vecteurs dorés partagés avec TypeScript, propriété « pli
  identique dans tous les ordres » ; `mux` sous benchstat (chemin critique).

## Plus d'informations

**Amendements :**
- ADR 0017 et 0030 : `SeenListing` ; passé causal joint pour D1 ; purge absorbante pour D2 ; accusés
  `Ack` ; effacement en deux phases ; entrées `doc/<id>`, `quota/<clé>` et `legacy/<doc>` du journal ;
  G11 et G12 dans la garde ; ordre de la chaîne de réception ; ordonnanceur commun.
- ADR 0018 : les éphémères sont bornés et réduits au schéma de présence ; la couverture d'une
  modification vient de la clé de son auteur.
- ADR 0023 : admission des nœuds texte par annonce (sorte `node`) quand l'index arrivera ; ménage des
  blobs soumis à la stabilité de la purge.
- ADR 0028 : réassemblage réservé par pair avec débordement sur disque (D5) ; jetons de grosse
  trame ; le budget mémoire de D9 sert de borne de résidence.

**Conception détaillée et revues :**
- `docs/plans/audit-2026-10-31/06-admission-design.md` (v4) ;
- revues Codex 07, 08 et 10 ;
- mesures et sondes 09.

**Sources :**
- automerge-repo 2.5.6 (`dist/`) et automerge 3.5.0 (`decodeChange`, `InitOptions.author`) ;
- [automerge/automerge-repo#745](https://github.com/automerge/automerge-repo/issues/745) (expéditeur
  non attribuable d'un éphémère) ; PR #742 (relais des éphémères en 2.6) ;
- Kleppmann, [Making CRDTs Byzantine Fault Tolerant](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf)
  (PaPoC 2022), §2.3.1 et §3.4 ;
- Baquero, Almeida, Shoker, [Pure Operation-Based Replicated Data Types](https://arxiv.org/abs/1710.04469)
  (2017) : stabilité causale, ménage des pierres tombales ;
- Shreedhar, Varghese, [Efficient Fair Queuing Using Deficit Round Robin](https://iqua.ece.toronto.edu/baochun/ece1771f/papers/deficitrr.pdf)
  (IEEE/ACM ToN, 1996) ;
- [RFC 9000 §4](https://www.rfc-editor.org/rfc/rfc9000#section-4) (contrôle de flux par crédit du
  receveur) ;
- spécification Matrix, [vérifications à la réception d'un PDU](https://spec.matrix.org/v1.16/server-server-api/#checks-performed-on-receipt-of-a-pdu) ;
- [gestionnaire de ressources de go-libp2p](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager).
