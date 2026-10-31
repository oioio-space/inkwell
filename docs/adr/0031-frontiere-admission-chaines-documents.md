---
status: acceptée
decided: 2026-10-31 (utilisatrice)
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

D-1 à D-6 sont **ratifiées par N9** (voir ci-dessous). Le statut de cette ADR reste
**proposée** jusqu’à son acceptation explicite ; ratifier ces paramètres ne vaut pas accepter
la conception ni son résiduel Critique. Révision n°6 (« périmètre final », architecte, 31/10/2026),
après la revue Codex n°4, une relecture indépendante de la v5 et la recherche byzantine
(`docs/plans/audit-2026-10-31/14-research-byzantine-solutions.md`). Le détail est dans la section
« Périmètre final » en tête de `06-admission-design.md`, qui prévaut sur la v5.

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
- **Frontière dans l’adaptateur, pairs éphémères, correctif de cycle de vie et de chargement** :
  suspension des entrées et drainage des sauvegardes, annulation confirmée des poignées non prêtes.
  Le choix v4 sans correctif est rejeté : activité continue et chargements abandonnés le contredisent.

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
- Purge logique, puis barrière avec coordinateur, verrous, votes, abandon et certificat (v5) : même
  garantie, avec beaucoup plus de pièces et un coordinateur à récupérer. Écartée.
- **Purge logique restaurable pendant au moins 30 jours. Ensuite, chaque appareil admin enregistre puis
  publie un accusé définitif (`FinalAck`) et ne peut plus restaurer. La finalité est un prédicat pur
  et monotone ; l'effacement n'a lieu qu'une fois la finalité atteinte.**

**Lier un acteur Automerge à une clé**
- Correspondance acteur → clé au premier vu : elle accepte deux genèses rivales.
- Champ `author` d'Automerge, descripteur dans le message, double clonage (v5) : `load` ignore
  `author`, et il faut trois règles.
- **Tuple signé `ChangeSig` (clé, epoch, seq, hachage) pour chaque modification, avec un acteur
  dérivé de (projet, clé, epoch).** Le hachage Automerge engage l'acteur et le seq. Une sonde du
  31/10 confirme qu'un chargement avec le seul acteur suffit. La preuve d'équivoque fait ≈ 300 octets
  et Go la vérifie sans lire Automerge.

**Auto-équivoque d'un écrivain**
- Laisser la division en l'état, avec preuve et retrait (v5) : les répliques divergent sans fin.
- Règles sans arbitre :
  - tout exclure à partir de seq k : chasse tout le travail honnête fait depuis ;
  - le plus petit hachage gagne : W choisit le gagnant ;
  - le plus de témoins gagne : des complices votent, et le résultat n'est pas monotone.
- Identifiants d'op par hachage (Kleppmann 2022 §3.3) : un autre encodage CRDT, à rechercher.
- Diffusion fiable byzantine ou comité (> 2/3, Kleppmann §2.3.2) : incompatible avec l'édition hors
  ligne.
- **Arbitrage par une décision admin `equiv/`, reconstruction déterministe depuis l'ensemble filtré,
  et branche perdante conservée dans un document de récupération dérivé.** C'est le modèle d'arbitre
  d'ERA, avec les admins de confiance de l'ADR 0017.

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
- **Amorçage et stockage.** Une voie par propriétaire, au plus une requête de stockage sous-jacente
  par voie (en file ou exécutée). L’échéance de 10 s détache l’attente, mais garde le bail jusqu’à
  `Released`. Retrait d’admission/époque identique. Pas de seconde poignée pour remplacer une attente
  bloquée. Le flux séquentiel actuel (`GoStorageAdapter.ts:57`) doit devenir un répartiteur réel,
  un worker indépendant par voie (≤ écrivains + 2), réponses démultiplexées et pages ≤1 Mio ; ordre
  des sauvegardes/lectures conservé par id. Une E/S bloquée ne bloque que sa voie. Libérer seulement
  la Promise accumule les requêtes ; garder le bail sans changer Go bloque encore H : deux options
  rejetées. La pagination et les workers sont des prérequis, pas une escalade éventuelle.
- **Équité.** Réassemblage réservé 4 Mio/pair, débordement disque, files bornées, tranches ≤1 Mio,
  crédit 2 Mio/pair ; DRR sur coût mesuré, quantum 8 ms, fond au moins un tour sur quatre.
  Deux permis bornent les **allocations vivantes**, un par pair. À 30 s, déconnexion ou retrait,
  Cancel arrête les entrées ; le permis reste occupé jusqu’au `Released` après fin du garde/apply,
  libération des tampons ou terminaison/join du Worker. Un ancien Done ne libère jamais un nouveau
  permis. Époques et générations identifient chaque travail. Le Worker précède l’activation.
- **Historique sans croissance textuelle.** Le plafond de texte ne borne pas les opérations :
  parser borné avant WASM, ≤16 Mio décompressés par job et ≤65 536 opérations **par modification**,
  mémoire Worker ≤256 Mio. La fabrique locale découpe toute modification, suppressions comprises
  (`store/chunked.ts` ne les découpe pas aujourd'hui) ; l'historique `legacy` est exempté du plafond
  d'op, sous la borne Worker. Une synchro trop grosse passe par des pages d'historique.
  Vérifier insertions/suppressions et réécritures à croissance nulle. Une synchro trop grosse exige
  un protocole émetteur paginé ; ne pas prétendre découper le paquet actuel. Sans ce protocole,
  disponibilité des gros historiques reste Critique et bloque l’activation. D-3 ne change pas :
  aucune suspension ni limite cumulée automatique, détection/alerte à 256 Mio/jour par auteur.
- **Résidence.** Suspendre toutes les entrées de l’id choisi, même sous écritures continues ; annuler
  le throttle, attendre sauvegardes/compactages, flush des têtes figées puis barrière du stockage,
  enfin évincer. Les échecs de sauvegarde restent visibles et ne perdent pas les modifications.
  Correctif requis : génération/identité après chaque await de chargement (dont Repo.js:499),
  suppression des poignées non prêtes sans flush, désinscription du synchroniseur et des rappels,
  flush limité aux prêtes. Au plus une poignée non prête active/abandonnée par bail (≤W+2), même
  après retrait ; aucun remplacement avant confirmation de drainage. W compte les propriétaires
  historiques, y compris retirés, bornés par le journal des membres. Une poignée unavailable est
  détruite avant de rendre son bail ; sinon les chargements achevés s’accumuleraient. 60 s déclenche l’annulation,
  pas une promesse de déchargement d’E/S bloquée. Les 16 documents non ouverts et le budget numOps
  sont complétés par la borne mémoire Worker et son recyclage après persistance.
- **Un seul adaptateur par `Repo`**, vérifié par identité d'objet.

### 2. Chaînes d'annonces (`internal/listing`)

- **Annonce.** Une op d'annonce porte : projet, `Prev` (l'annonce précédente de l'auteur),
  `Members` (frontière de la liste vue, au plus 64), `Decisions` (frontière du journal de décisions
  vue, au plus 64), position, sorte (`variant`, `adopt` ; `node` réservé à l'index de l'ADR 0023),
  Slot (ordinal des variantes, distinct de la position de chaîne qui compte aussi les adoptions),
  ancre (hachage d’ancrage) ou op adoptée, auteur. Elle est
  signée Ed25519 sur le tuple entier.
- **Identifiant.** `DocId = BLAKE3("inkwell/doc/v1" ‖ projet ‖ auteur ‖ sorte ‖ Slot ‖ ancre)[:16]`.
  Il est toujours recalculé, jamais reçu. Une seule ancre par identifiant.
- **Admission d'une op.**
  - L1 : canonique et signée pour le projet.
  - L2 : passé connu, sinon attente dans un tampon de 4 par écrivain debout, qui n'évince que les
    siennes.
  - L3 : position exacte. `1 ≤ Slot ≤ Q`, et aucun Slot répété sur la chaîne. Pas d'arithmétique
    de préfixe (révision 6).
  - L4 : l'auteur écrit et n'est pas suspendu sur `past(Members)`.
  - L5 : nombre de variantes ≤ `Q = 32 + 32 × grants`, où `grants` compte les décisions
    `quota/<clé>` **du passé de `Decisions`** qui tiennent dans le pli. Une adoption vérifie L1, L4 et
    **L5 à l’émission** sur l’op adoptée : pli de ses frontières signées, sans les coupes ultérieures.
    L’annonce ordinaire exige encore les octrois debout dans le pli courant. Une annonce avant
    octroi reste invalide ; une annonce après octroi peut être adoptée si cet octroi meurt ensuite.
    L'op adoptée est vérifiée par L1, L4 et L5 à l'émission seulement : aucun préfixe à récupérer ni
    à conserver (révision 6). La borne reste ≤ Q* identifiants propres.
    Une seule adoption par (adoptant, auteur étranger, Slot), aucune adoption imbriquée.
    Dériver l’id de la position de chaîne (v4) déplacerait les slots par des adoptions intercalées :
    rejeté ; le Slot signé conserve la borne de variantes sous équivoque.
- **Équivoque.** Deux op d'un auteur à la même position gèlent sa chaîne à partir de cette position,
  partout. La preuve est gardée et diffusée.
- **Adoption.**
  - Elle est persistée automatiquement au premier **véritable coédit local**. Le déclencheur est la
    première `ChangeSig` de cet appareil sur un document annoncé par un autre auteur.
  - Elle est écrite avant la publication de la modification, et reprise de façon idempotente après un
    plantage.
  - Un relais ne produit jamais de `ChangeSig`, donc jamais d'adoption. Le reçu de contribution de la
    v5 est supprimé : aucun pair ne le vérifiait.
  - Elle ne consomme pas le quota de H et ne contourne pas une purge.
  - Bornes : une adoption par (adoptant, auteur étranger, Slot) ; Q* = 32 + 32 × octrois causalement
    valides émis. La chaîne retient variantes **et** adoptions, plus deux op de preuve.
  - À densité complète, l'adoption différée et l'immédiate font toutes deux `32 W(W−1)` op. Celle au
    premier coédit supprime le détecteur de disparition et sa fenêtre.
- **Signature d'auteur `ChangeSig`.**
  - Forme : `Ed25519(clé, "inkwell/change/v1" ‖ projet ‖ clé ‖ epoch ‖ seq ‖ hachage)`.
  - L'acteur vaut `BLAKE3("inkwell/actor/v2" ‖ projet ‖ clé ‖ epoch)[:16]`. L'epoch est aléatoire, émis
    et persisté par Go, un par génération de poignée.
  - Elle est signée seulement sur le chemin sortant de la fabrique locale. Go la vérifie avant de
    transmettre ; la garde vérifie que le tuple correspond à la modification et que l'acteur est bien
    dérivé.
  - L'autorisation reste T10 (couverture par les écrivains courants). Les deux sont exigées.
  - Exceptions : G exacte et `past(L_d)`.
  - Stockage : environ 104 octets par modification, compris dans le résiduel disque.
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
  ∪ documents adoptés ∪ documents `kept` issus d'une annonce détenue ∪ documents `legacy/<d>`
  (identifiants d'avant la mise à niveau) ∪ documents de récupération des décisions `equiv/`, moins
  les documents purgés.
  - Go vérifie, persiste (`.inkwell/listing/`), synchronise sur le canal trust et envoie l'ensemble à
    la webview (`Listed`, avec projet et époque).
  - Le même ensemble sert à l'admission, au partage, au chargement et aux attestations.
  - `meta.variants` ne sert plus qu'à l'affichage. Tout document admis a une ligne dans l'historique,
    même sans entrée.
  - Un document d'un autre projet n'est jamais admis. Importer, c'est copier dans un nouveau document
    annoncé.
- **Purge et restauration.**
  - La décision logique retire l'admission mais garde les octets. Un `kept`/`trashed` ultérieur
    restaure.
  - Fenêtre : **30 jours minimum** par génération, durable ; un recul d'horloge retarde, n'accélère pas.
  - Après la fenêtre, chaque appareil admin dont l'utilisatrice n'a pas restauré **enregistre, puis
    publie** un `FinalAck` (document, génération, purge, frontières). Il refuse ensuite de signer une
    restauration de cette génération.
  - Ensemble requis R(p) : les appareils admin debout à `past(Members(p))`, fixé par la purge.
  - **Finalité** : tous les accusés de R(p) sont détenus, et la purge gagne dans le pli de l'union de
    leurs frontières. Prédicat pur et monotone.
  - Une fois final :
    - une restauration hors du passé des accusés est ignorée partout ; seule celle d'un admin promu
      après la purge peut l'être, résiduel Mineur ;
    - rejeu et adoption de l'id sont refusés ;
    - chaque détenteur écrit une intention, clôt les écritures, draine, efface, avec reprise après
      plantage.
  - Un appareil absent bloque. On le retire, puis on réémet la purge : nouvelle génération, nouvelle
    fenêtre.
  - Ni coordinateur, ni préparation, ni verrou, ni vote.
  - Les blobs suivent la même finalité. Les positions ne sont jamais rendues.
- **Pont.** `Listed`, `AttestRefused`, `Done`, `Cancel` et `Released` ; tout rappel différé porte l'époque.

### 3. Ordre et contrat G12

- **Étape 1** fusionnée : périmètre provisoire collant ; S1 reste Critique (N8).
- **Étape 2** : membres et déconnexion explicite au retrait (même si PeerDown est filtré) ;
  `internal/session` puis journal/Seen/passé joint et décisions **legacy/<d> avant G12** ;
  garde G1-G10, Worker, limites opérations/historique, Cancel/Released, ordonnanceur et stockage
  paginé ; enfin G12 avec fabrique de chargement intégrée à Repo.
- **G12 = `ChangeSig`** (révision 6), voir l'étape 3.
  - Chaque modification est vérifiée isolément : la signature de l'auteur, et l'acteur égal à la
    dérivation de (projet, clé, epoch).
  - Réencoder W sous l'acteur de H échoue, tout comme deux genèses rivales et l'étiquetage par un
    relais.
  - `changeAt` natif est désactivé : view, puis clone avec un nouvel epoch.
- **Fabrique** : `load`/`clone`/`create`/`import` et l'hydratation interne de Repo reçoivent l'acteur
  dérivé, via le correctif de cycle de vie.
  - Sonde 3.5.0 : `load(bytes, {actor})` donne cet acteur à seq 1.
  - Le champ `author` n'est plus utilisé.
  - Tester le delta suivant dans le vrai Repo avant activation.
- **Exception de genèse ADR 0030 §3 (:182)** : seulement G de l’index, recalculée localement,
  octets et hash exacts, acteur project-derived a0, seq/startOp=1, time=0, deps=[], schéma et RootURL
  exacts. Exemption auteur/couverture/dérivation uniquement pour G ; aucun descendant sans auteur
  ni genèse concurrente. L’exception legacy exige, séparément, les têtes signées par admin.
- **Étape 3** : chaînes et SeenListing ; pont et ensemble admis en mode fantôme ; migration `legacy/<d>`
  et négociation de version ; **activation commune** (création à id dérivé, admission, G11, adoption au
  premier coédit) ; arbitrage `equiv/` ; retrait de l'autorité de `meta.variants`.
- Plan ordonné, étapes 2.0 à 3.6, chacune fusionnable seule avec ses tests : design, « Périmètre final »
  F.3. Compatibilité et **ZZ-2 à chaque fusion**, même inactive.

### 4. Périmètre final (révision 6)

**Faisable maintenant** (sous les hypothèses : Ed25519/BLAKE3 sûrs, échange final entre pairs
honnêtes, admins debout de confiance) :
- `ChangeSig` et preuves d'équivoque compactes ;
- contribution = modification signée acceptée, adoption au premier coédit ;
- chaînes d'annonces, ids dérivés, G11, révocation causale, quotas sur frontière ;
- garde G1-G10 avec bornes par modification et par job ; la fabrique locale découpe à 65 536 op ;
- Worker, jetons Cancel/Released, DRR (coût décodé imputé à l'auteur) ;
- pages d'historique pour les gros documents ;
- correctif de cycle de vie, voies de stockage paginées ;
- finalité de purge par `FinalAck` ;
- arbitrage `equiv/` avec reconstruction déterministe et document de récupération ;
- compteur par auteur, alerte et plancher disque de l'hôte (dans D-3) ;
- table des pairs limitée aux membres.

**Recherche ouverte, en résiduels gradés :**
- R1, auto-équivoque : divergence jusqu'à l'arbitrage, travail honnête de la branche perdante
  déplacé, sans perte. **Critique**, à accepter.
- R2, disque cumulé : **Critique**, sous D-3. La fermeture connue est un quota cumulé par écrivain,
  qui demande d'amender D-3.
- R3, effacement chez un pair hostile : hors barème.
- R4, effacement bloqué par un appareil absent : Mineur.
- R5, duel d'admins sans finalité neutre hors ligne : Mineur.
- R6, restauration écrasée d'un admin promu après la purge : Mineur.
- R8, coût quadratique des adoptions et des `ChangeSig` : Mineur tant que non mesuré.
- R9, S_max : Mineur si B11 passe.

### Conséquences

**Bien**
- Sous réserve des tests d’activation, les quotas et annonces d’un écrivain ne consomment pas
  ceux d’un autre ; l’adoption préserve la visibilité des coédits. Cela ne clôt pas l’autoéquivoque
  ni le disque cumulé, résiduels Critique ci-dessous.
- Ses effets sont signés et bornés : au plus `Q_w` identifiants à lui, et une adoption par position et
  par co-auteur.
- Lecteurs, pairs sans attestation et inconnus n'ont plus d'état non borné, ni dans l'adaptateur ni
  dans le `Repo`.
- Plus aucun état de synchro sur disque ; correctif Repo de cycle de vie/fabrique maintenu et testé.
- Rien n'est effacé du disque tant qu'une annulation honnête reste possible.
- Le transfert réseau ne détient pas les permis de traitement ; ordonnanceurs et baux bornent
  le travail sous-jacent, sous réserve des mesures B10–B14. Le matériel de stockage reste partagé.

**Moins bien, assumé**
- **Reste critique (R1)** : un écrivain qui publie deux contenus pour **son propre** `(acteur, seq)` divise
  les documents où il écrit, jusqu'à l'arbitrage `equiv/` par un admin.
  - Ensuite, toutes les répliques convergent par reconstruction déterministe, et le travail honnête
    de la branche perdante reste dans un document de récupération.
  - Preuve signée par W, plus de relais, nommage dans l'interface, retrait.
  - Il n'existe pas de règle sans arbitre pour des identifiants `(compteur, acteur)` (Kleppmann 2022
    §2.4) ; la finalité avant application exige plus des 2/3 de pairs corrects et un quorum (§2.3.2).
- Les documents sans coéditeur restent « à confirmer » ; les coédits durables ont déjà leur adoption.
- Un appareil admin absent bloque l'effacement. Il faut le retirer puis réémettre la purge, ce qui
  ouvre une nouvelle fenêtre (R4, Mineur).
- Application indivisible : les limites d’opérations, de décompression et de mémoire Worker,
  avec terminaison confirmée, complètent le plafond textuel D1. **Critique tant que** le protocole
  de gros historiques et l’intégration de la fabrique ne passent pas leurs tests.
- **Critique ASVS V2.4.1 (R2)** : la croissance cumulée du disque reste non bornée par auteur. N9 D-3
  autorise détection et alerte, pas un blocage automatique.
  - Le plancher disque de l'hôte met en pause la réception distante pour tous, sans perte ni
    corruption. Il ne ferme pas ce risque.
  - Seul un quota cumulé par écrivain, qui suppose d'amender D-3, le fermerait.
- Une reconnexion coûte environ 1,25 octet et 2 µs par modification et par document de plus. C'est
  une mesure indicative, à confirmer par le banc répété.
- Il faut une modification d'ancrage par document créé, et un appel à Go avant `import`.
- La mémoire des attestations/adoptions croît comme le carré du nombre d’écrivains : Mineur
  tant que non mesuré, Majeur seulement si un budget mesuré est dépassé.
- 32 variantes par écrivain est une hypothèse (`docs/open-questions.md`). La relever reste compatible.
- Écrire du contenu nuisible dans un document partagé reste possible à un écrivain courant. C'est
  attribuable, réversible par l'historique, et le retrait de la personne l'arrête.

**D-1 à D-6 ratifiées (N9)**
- D-1 : 32 variantes initiales/personne qui écrit (octrois de 32).
- D-2 : récupération pendant 30 jours minimum, effacement après tous les accusés/barrière.
- D-3 : détecter et alerter les admins, sans blocage cumulatif automatique.
- D-4 : irréversibilité **après effacement**, restauration avant.
- D-5 : adoption automatique des coédits (persistée au premier coédit en v5).
- D-6 : une décision admin signée legacy/<doc> par document antérieur, pas de remise à zéro.

Reste à l'utilisatrice, sans rouvrir D-1..D-6, et sans qu'aucune acceptation se déduise de N8/N9 :
1. accepter l'ADR 0030 (proposée), base de l'étape 2, avec ses amendements ci-dessous ;
2. accepter cette ADR, révision 6 (elle reste **proposée**), y compris le rôle d'arbitre `equiv/` des
   admins ;
3. accepter explicitement le résiduel Critique R1 ;
4. pour R2, soit l'accepter sous D-3, soit amender D-3 vers un quota cumulé par écrivain.

Barème inchangé ; aucune fermeture d'implémentation n'est revendiquée ici.

### Confirmation

- **Tests de contrat** sur automerge-repo épinglé :
  - clé de session ; `hasRequested` noté avant l'accès ; `isEphemeral` ; sauvegarde différée ;
  - `removeFromCache` et `flush` sur une poignée non prête (documentent pourquoi on ne le fait pas) ;
  - éviction d'une poignée prête, rechargement, modification, puis rechargement depuis le disque :
    aucune modification perdue (compactage) ;
  - `import` avec `docId` ; message de synchro en un seul paquet ; fabrique à acteur dérivé
    intégrée à Repo (delta suivant), `ChangeSig`, deux genèses concurrentes et G exact.
- **Étape 1** : décodeur strict (table et fast-check) ; P1, P2 et P3 bornés (comptes par espions et
  tas) ; file et amorçage avec un stockage bloqué ; e2e ZZ-1 et ZZ-2 dans `history.spec.ts`.
- **Étapes 2 et 3** : un test par borne, au seuil, juste au-delà, et pour le retour à la normale
  (tableau 8.2 de `06`), plus :
  - équivoque après une contribution honnête (le document reste, adopté) ;
  - inondation d'équivoques sur 3 redémarrages ; alias entre écrivains ;
  - quotas : annonce avant ou après l'octroi, octroi annulé par une coupe concurrente ;
  - G11 : livraison fragmentée, parent rival, descendant qui usurpe un acteur ;
  - purge : coupe tardive, `kept` concurrent, redémarrage entre les phases, écriture déjà engagée,
    appareil absent ; restauration au jour 29, `FinalAck` refusé avant le jour 30, restauration
    refusée après son propre `FinalAck`, effacement seulement une fois la finalité atteinte ;
  - amorçage avec 40 voies hostiles bloquées, 100 échéances répétées, H puis retrait et reprise tardive ;
  - équité multi-pairs sur horloge réelle (p99 interactif ≤ 2 × base + une application) ;
  - jetons : garde suspendue au-delà de 30 s, retrait/déconnexion, reprise tardive ; pas de
    réutilisation avant Released ; opérations à texte constant aux seuils/+1 et pic RSS mesuré ;
  - éviction sous écritures locales/distantes continues, compactage bloqué, retrait et reprise ;
  - identifiants inconnus pendant une invitation (S1-R6) ;
  - `ChangeSig` (A15 révisé, B18) ; suppression honnête de 2 Mi caractères en ≥ 32 modifications ;
    modification distante de 65 537 op refusée avant WASM ;
  - pages d'historique (200 Mio à croissance nulle, reprise) ;
  - `FinalAck` (A9 révisé, B15') ;
  - arbitrage `equiv/` sur 3 répliques, dans tous les ordres (A16) ;
  - plancher disque et alerte par auteur (B19) ;
  - banc de reconnexion répété (≥ 10 exécutions).
- **Go** : `FuzzDecode` de `listing`, vecteurs dorés partagés avec TypeScript, propriété « pli
  identique dans tous les ordres » ; `mux` sous benchstat (chemin critique).

## Plus d'informations

**Amendements :**
- ADR 0017 et 0030 :
  - `SeenListing` ; passé causal joint pour D1 ;
  - `FinalAck` et finalité de purge (remplacent `Ack`, la barrière et le certificat) ;
  - entrées `doc/<id>`, `quota/<clé>`, `legacy/<doc>` et `equiv/<acteur>/<seq>` du journal ;
  - G11, `ChangeSig` (G12) et preuves d'équivoque dans la garde ;
  - bornes par modification et par job ;
  - ordre de la chaîne de réception ; ordonnanceur commun.
- ADR 0018 : les éphémères sont bornés et réduits au schéma de présence ; la couverture d'une
  modification vient de la clé de son auteur.
- ADR 0023 : admission des nœuds texte par annonce (sorte `node`) quand l'index arrivera ; ménage des
  blobs soumis à la stabilité de la purge.
- ADR 0028 : réassemblage réservé par pair avec débordement sur disque (D5) ; jetons de grosse
  trame ; le budget mémoire de D9 sert de borne de résidence.

**Conception détaillée et revues :**
- `docs/plans/audit-2026-10-31/06-admission-design.md` (v5) ;
- revues Codex 07, 08, 10 et 13 ; correspondance complète « Changes after review 4 » dans le design ;
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
- [gestionnaire de ressources de go-libp2p](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager) ;
- recherche byzantine du 31/10/2026 (`14-research-byzantine-solutions.md`) : SSB, Nostr NIP-01, RFC 9162
  §11.3, p2panda-auth, [ERA v2](https://arxiv.org/html/2601.22963v2) (PaPoC 2026), Autobase, MLS (RFC 9420) ;
- preuves de fourche de Hypercore ([hypercore-next#76](https://github.com/hypercore-protocol/hypercore-next/issues/76)).


## Décision de l'utilisatrice (2026-10-31)

ADR acceptée (rév. 6), y compris le rôle d'admin qui arbitre une équivocation. Résiduel R1
(équivocation d'un auteur sur ses propres modifications, jusqu'à l'arbitrage) **accepté** : le
supprimer exigerait un quorum, donc la fin de l'édition hors ligne. Résiduel R2 (croissance cumulée
du disque) **fermé** : D-3 est amendée par un **quota disque cumulé par personne qui écrit**, avec
alerte avant le seuil ; au-delà, ses nouvelles écritures sont refusées ; un admin peut l'augmenter.
Registre : `docs/exigences.md` N10.
