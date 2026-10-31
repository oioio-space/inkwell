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
- `removeFromCache` lève une erreur sur une poignée non prête et la garde en mémoire (mesuré : environ
  66 Kio de RSS chacune) ;
- le premier correctif, qui ne partageait que les documents tenus, a cassé la réplication des
  variantes créées après l'arrivée (e2e ZZ-2).

Décision de l'utilisatrice N7 : modèle byzantin complet. Même une personne invitée qui écrit, et qui
est malveillante, ne doit jamais gêner les autres. Lecteurs et pairs sans attestation sont strictement
bornés. N8 : l'étape 1 est fusionnée tout de suite, et S1 reste critique jusqu'à la garde de
l'ADR 0030.

## Facteurs de décision

- Aucun état par pair ou par document sans borne numérique et testée (ASVS 5.0 V2.4.1, §4).
- Tout effet d'un écrivain est **attribuable** et **borné par écrivain**. Aucun plafond partagé
  qu'un seul écrivain puisse remplir.
- Déterminisme : le même ensemble admis chez tous, quel que soit l'ordre d'arrivée.
- Go ne lit pas Automerge (ADR 0002, 0011). Une seule source de vérité par chose.
- La réplication immédiate de tout document du projet est conservée (« rejoindre = tout recevoir »).

## Options envisagées

**Où appliquer**
- Politique de partage du `Repo` seule. L'état est créé avant elle.
- Filtrage par Go du format d'automerge-repo. Cela lit un format applicatif, contre les ADR 0002 et
  0011, et suit ses changements (2.6, 3.0).
- Correctif local d'automerge-repo seul : dette à chaque version, et rien sur les formes ni la
  réplication.
- **Frontière dans l'adaptateur, pairs éphémères, et un seul correctif mesuré.**

**Qui décide qu'un document appartient au projet**
- La liste `meta.variants` tronquée : un seul écrivain la remplit ou en efface des lignes, et rien
  n'est attribuable.
- L'arborescence disque : elle exclut les variantes, ce qui recrée R1.
- Des op de la liste de membres : écarté par l'ADR 0030 (O3).
- Des entrées dans le journal de décisions : un écrivain scellerait le journal des admins.
- Des acteurs liés aux clés : demande une ADR à part et change tous les clients.
- Des jetons numérotés désignant un identifiant quelconque : l'équivoque fait croître les
  identifiants sans borne, et l'alias entre écrivains reste possible.
- **Une chaîne d'annonces par écrivain, avec un identifiant de document dérivé de la position.**

**Révocation**
- `EverWriter` : un écrivain retiré émet encore, en antidatant.
- **Rang causal, comme D1 de l'ADR 0030.**

## Décision

### 1. Frontière d'admission

Chaîne de réception : trame Go → décodage strict → **admission** → couverture (ADR 0018) → garde
structurelle (ADR 0030) → `Repo`.

- **Types admis.** Seuls `arrive`, `welcome`, `sync`, `request`, `ephemeral` et `doc-unavailable`
  passent, champ par champ, avec leurs bornes. Le message est reconstruit. Un document hors de
  l'ensemble admis est jeté sans réponse.
- **Éphémères.** Présence seulement : canaux `name`, `anchor`, `head`, au plus 1 Kio, 64 par seconde
  et rafale de 128 par pair. Session et compteur sont réécrits par l'adaptateur. Aucun relais sortant.
- **Pairs éphémères.** Tout pair est déclaré éphémère et son `storageId` est ignoré : aucun état de
  synchro n'est écrit. Ceux d'avant sont effacés par Go avant que la webview s'attache.
- **Contre-pression.** Les messages d'un document pas encore prêt attendent dans une file réservée
  par pair : 4 par document, 8 Mio plus une trame par pair, sans réserve commune.
  - La poignée est amorcée par un seul `find`. La file se vide dès l'état `ready`, `requesting` ou
    `unavailable`.
  - Après chaque attente, l'époque, l'admission et la présence du pair sont revérifiées.
  - Un débordement remet le lien à zéro, avec un délai de 30 s doublé jusqu'à 10 min, gardé par pair
    d'une connexion à l'autre.
  - La livraison se fait à tour de rôle entre pairs, partagée avec la garde (ADR 0030 §5), avec un
    budget de 16 Mio/s par pair.
  - Go ne livre à la webview qu'une trame de plus de 1 Mio par pair, et deux en tout.
- **Résidence.** En mémoire restent les documents ouverts, plus au plus 16 autres et 32 Mio d'octets
  stockés (LRU sans condition). Au plus 32 poignées non prêtes. Un correctif pnpm d'au plus
  15 lignes, couvert par un test de contrat et proposé en amont, permet d'évincer une poignée non
  prête.

### 2. Chaînes d'annonces (`internal/listing`)

- **Annonce.** Une op d'annonce porte : projet, `Prev` (l'annonce précédente de l'auteur),
  `Members` (frontière de la liste vue, au plus 64), position, sorte (`variant` ; `node` réservé à
  l'index de l'ADR 0023), ancre (le hachage d'une modification d'ancrage du nouveau document),
  auteur. Elle est signée Ed25519 sur le tuple entier.
- **Identifiant.** `DocId = BLAKE3("inkwell/doc/v1" ‖ projet ‖ auteur ‖ sorte ‖ position ‖ ancre)[:16]`.
  Il est toujours recalculé, jamais reçu.
- **Admission d'une op.**
  - L1 : canonique et signée pour le projet.
  - L2 : passé connu, sinon attente dans un tampon de 4 par écrivain debout.
  - L3 : position exacte.
  - L4 : l'auteur écrit et n'est pas suspendu sur `past(Members)`.
  - L5 : position ≤ `Q_w`.
- **Équivoque.** Deux op d'un auteur à la même position gèlent sa chaîne à partir de cette position,
  partout. La preuve est gardée et diffusée. Au plus `Q_w + 1` op sont gardées, et au plus `Q_w + 1`
  identifiants admis par écrivain, à vie.
- **Rang.**
  - Les op de membres qui coupent, suspendent ou enregistrent le départ de y portent
    `SeenListing(y)` : le bout de la chaîne de y vu par l'auteur. C'est une entrée typée du `Seen`
    de l'ADR 0030.
  - Une annonce de y tient si y écrivait à `past(Members)`, et si aucune coupe de y ne lui est
    concurrente. Est concurrente une coupe absente de `past(Members)`, quand l'annonce est au-delà du
    `SeenListing` de cette coupe.
  - Une annonce morte n'efface rien. Son document est montré « à confirmer », et une décision
    `doc/<id> = kept` le réadmet.
- **Ancrage (G11, dans le module de la garde).** Un document annoncé n'admet une modification que si
  elle est un ancêtre de l'ancre, ou si l'ancre est dans son passé. Une histoire rivale est refusée
  partout.
- **Ensemble admis.** Il vaut : racine ∪ index G ∪ documents des annonces debout, sous le point de
  gel, moins les documents purgés.
  - Go vérifie, persiste (`.inkwell/listing/`), synchronise sur le canal trust et envoie l'ensemble à
    la webview (`Listed`, avec projet et époque).
  - Le même ensemble sert à l'admission, au partage, au chargement et aux attestations.
  - `meta.variants` ne sert plus qu'à l'affichage. Tout document admis a une ligne dans l'historique,
    même sans entrée.
  - Un document d'un autre projet n'est jamais admis. Importer, c'est copier dans un nouveau document
    annoncé.
- **Quotas.** `Q_w = 32 + 32 × grants(w)` variantes par écrivain, pour toute la vie du projet. Les
  `grants` sont des décisions d'admin `quota/<clé>`. La 33ᵉ variante est refusée avec un message.
- **Purge.**
  - Une décision `doc/<DocId> = purged` est une pierre tombale permanente.
  - Le rejeu est sans effet, et aucune nouvelle op ne redonne le même identifiant.
  - Go efface `repo/<DocId>/` et refuse toute écriture dessous.
  - La position n'est pas rendue.
- **Pont.** `Listed` et `AttestRefused` (Go → webview). L'adaptateur n'atteste que les documents de
  l'époque courante, et réessaie après un refus.

### 3. Ordre

- **Étape 1** : la frontière, avec un ensemble provisoire = racine + `meta.variants`, au plus 256
  documents par session et collants. Elle est fusionnée maintenant ; S1 reste critique (N8).
- **Étape 2** : la garde de l'ADR 0030.
- **Étape 3** : les chaînes, G11, la résidence, l'ordonnanceur et le sémaphore Go. Elle vient après
  l'étape 2.

### Conséquences

**Bien**
- Un écrivain ne peut ni déplacer, ni cacher, ni épuiser les documents d'un autre. Ses effets sont
  signés et bornés à `Q_w + 1` identifiants à vie.
- Lecteurs et pairs sans attestation n'ont plus d'état non borné, ni dans l'adaptateur ni dans le
  `Repo`.
- Plus aucun état de synchro sur disque.
- La réplication immédiate revient, mais seulement pour les documents admis.

**Moins bien, assumé**
- Une reconnexion coûte environ 1,25 octet et 2 µs par modification et par document de plus. C'est
  une mesure indicative, à confirmer par le banc répété.
- Il faut une modification d'ancrage par document créé, et un appel à Go avant `import`.
- Le correctif pnpm P1 est à reprendre à chaque version d'automerge-repo.
- La mémoire des attestations croît comme le carré du nombre d'écrivains : à surveiller.
- 32 variantes par écrivain est une hypothèse (`docs/open-questions.md`). La relever reste compatible.
- Écrire du contenu nuisible dans un document partagé reste possible à un écrivain courant. C'est
  attribuable, réversible par l'historique, et le retrait de la personne l'arrête.

### Confirmation

- **Tests de contrat** sur automerge-repo épinglé :
  - clé de session ;
  - `hasRequested` noté avant l'accès ;
  - `isEphemeral` ;
  - sauvegarde différée ;
  - `removeFromCache` (correctif P1) ;
  - `import` avec `docId`.
- **Étape 1** : décodeur strict (table et fast-check) ; P1, P2 et P3 bornés (comptes par espions et
  tas) ; file et amorçage avec un stockage bloqué ; e2e ZZ-1 et ZZ-2 dans `history.spec.ts`.
- **Étape 3** :
  - inondation d'équivoques sur 3 redémarrages ;
  - alias entre écrivains ;
  - équité entre un écrivain hostile et des créations honnêtes concurrentes ;
  - annonces après coupe (mortes) et annonces vues (debout) ;
  - G11 contre une histoire rivale ;
  - purge, rejeu et ménage disque ;
  - résidence et éviction des poignées non prêtes ;
  - équité multi-pairs (p99 ≤ 2 × base, horloge réelle) ;
  - pic de RSS ≤ base + 256 Mio ;
  - banc de reconnexion répété (≥ 10 exécutions).
- **Go** : `FuzzDecode` de `listing`, vecteurs dorés partagés avec TypeScript, propriété « pli
  identique dans tous les ordres ».

## Plus d'informations

**Amendements :**
- ADR 0018 : les éphémères sont bornés et réduits au schéma de présence ; ils ne sont plus seulement
  « permis aux lecteurs ».
- ADR 0030 : l'ordre de la chaîne de réception ; `SeenListing` ; les entrées `doc/<id>` et
  `quota/<clé>` du journal ; G11.
- ADR 0023 : l'admission des nœuds texte par annonce, avec la sorte `node`, quand l'index arrivera.

**Conception détaillée et revues :**
- `docs/plans/audit-2026-10-31/06-admission-design.md` ;
- revues Codex 07 et 08 ;
- mesures 09.

**Sources :**
- automerge-repo 2.5.6 (`dist/`) ;
- [automerge/automerge-repo#745](https://github.com/automerge/automerge-repo/issues/745) (expéditeur
  non attribuable d'un éphémère) ;
- PR #742 (relais des éphémères en 2.6) ;
- Kleppmann, *Making CRDTs Byzantine Fault Tolerant* (PaPoC 2022), §2.3.1 et §3.4 ;
- spécification Matrix, [vérifications à la réception d'un PDU](https://spec.matrix.org/v1.16/server-server-api/#checks-performed-on-receipt-of-a-pdu) ;
- [gestionnaire de ressources de go-libp2p](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager).
