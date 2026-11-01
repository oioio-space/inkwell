# Changements de la révision 2 (2.0.0-brouillon → 2.0.0-brouillon.2)

Chaque constat des revues [01-codex](reviews/01-codex.md) (« revise ») et
[02-fable](reviews/02-fable.md) (« sound with changes »), et chaque exigence arrivée pendant la
révision (N13, N14, N15), avec le changement qui y répond. « Sans objet » : le mécanisme visé
n'existe plus grâce au flux par classe (F2-1).

## Revue 01-codex

| # | Gravité | Constat | Changement | Où |
|---|---|---|---|---|
| C1 | Critique | Admission sans la garde structurelle de l'ADR 0030 ; ChangeSig et historique « réservés » malgré les prérequis d'activation de l'ADR 0031 ; `core/2` seul n'établit pas les garanties byzantines | Chaîne de réception complète et ordonnée (profil → schéma → admission → couverture → garde G1–G12 → `Repo`) ; garde normative ; formats de ChangeSig (AUTHOR), pages d'historique, accusés, annonces, preuves ; **`core/2` contient tout et rien de sécuritaire ne se négocie** ; prérequis de l'ADR 0031 en porte P-ADR31 avant la bascule | spec §4.5–§4.8, §5.4, §9.4, §9.6, §9.7, §12 ; ADR §Confirmation |
| C2 | Critique | REPO_RESET double d'anciens REPO en file ; échos ; livraisons asynchrones sans génération ; pas de barrière ; épuisement du `u32` | Génération **dans le préambule du flux REPO** ; changer de génération = réinitialiser le flux (le transport jette son contenu) ; règle T (génération vérifiée à chaque passage asynchrone, pont compris) ; barrière = préambule ; règle W (fin de session avant dépassement) ; **modèle TLA+ écrit et vérifié** (il a trouvé deux défauts, corrigés : règles B et A ligne 3) | spec §7.3, §7.4, Annexe A.1 ; `tla/` |
| C3 | Critique | Un écrivain occupe les 65 536 clés d'attestation ; des pairs retiennent tout le budget d'objets offerts | **Règle générale des parts réservées** (`⌊C/n⌋`, reprise seulement au-dessus de sa part) appliquée à tous les plafonds partagés ; attestations par signataire ; objets LARGE par pair des deux côtés ; vrac par pair | spec §10.6, §16 |
| C4 | Critique | Migration : activation avant la vérification de REPO_RESET ; pas de portes ; vecteurs proposés qui cassent les fusions intermédiaires ; décisions non consignées | Modèle et portes **avant** la bascule (étapes 3 et 9) ; neuf portes nommées ; champ `enforce` dans chaque vecteur (exigé à partir d'une étape) ; bascule nette et abandon sans condition du débordement/éviction consignés | ADR §Migration, §Confirmation, §Décisions ; spec §21 |
| C5 | Mineur | File REPO pleine : RESET et TRUST derrière, auto-attente possible | **Sans objet** pour le contrôle : CONTROL est un flux séparé, toujours lu ; preuve d'absence d'auto-blocage (graphe d'attente acyclique) ; vérifiée par le modèle à files bornées | spec §10.2, §10.7 |
| C6 | Mineur | Un message hors fenêtre bloque frappe et présence sur le pont | Deux voies de crédit (en ligne, LARGE) ; présence sans crédit | spec §10.3, Annexe A.1 ; `bridge-internal.json#bridge-credit-lane` |
| C7 | Mineur | Vrac : appariement, dernier groupe partiel, taille Bao avant allocation, portée des erreurs, réponses non sollicitées, BUSY | Pour les objets offerts : **sans objet** (plus d'OFFER). Pour les blobs : une demande par (pair, hash), appariement exact, dernier groupe défini, taille vérifiée avant allocation, non sollicité jeté, BUSY compté, délai au progrès, tailles adaptatives, forme fausse = violation partout | spec §8.1, §13 ; `bulk.json#*final-group*`, `#slice-*` |
| C8 | Mineur | Plage + liste noire ≠ décodage (`02 00…00`) ; rechargement | Décodage RFC 8032 §5.1.3, encodage canonique, **sous-groupe d'ordre premier** ; à chaque entrée et au rechargement/import | spec §3.2, §14 ; `crypto.json#off-curve-public-key`, `#noncanonical-public-keys`, `#speccheck-*` |
| C9 | Mineur | CBOR déterministe non défini | **Profil de validité** (refuse exactement ce que deux décodeurs liraient différemment) + politique d'émission (sérialisation préférée recommandée) ; pas de flottant ; justification : messages non signés (voir F2-3) | spec §9.2 ; `repo.json` |
| C10 | Mineur | `targetId` prose/CDDL ; bornes en commentaires ; champs inconnus imbriqués ; profondeur | `targetId` partout (une règle) ; bornes en contrôles CDDL ; maps ouvertes/fermées à toute profondeur ; profondeur ≤ 8 et tailles dans le profil | `cddl/repo.cddl` ; spec §9.2.3 |
| C11 | Mineur | Pas de vecteurs REPO/PRESENCE ; contexte ; traces de réinitialisation ; queues de vrac | `repo.json` (39 vecteurs, positifs et négatifs, avec `context`) ; `session.json` (flux, générations) ; `bulk.json` (dernier groupe) ; traces : le modèle TLA+ (contre-exemples TLC) | `vectors/`, `tla/` |
| C12 | Mineur | Perte d'ATTEST réparée seulement à la reconnexion | **ATTEST_WANT** : réconciliation tirée par le receveur, répétée toutes les 15 s tant qu'une couverture attend, puis règle L | spec §6.2 |
| C13 | Mineur | `ops` ne tient pas 1024 ops ; lots, ordre causal, suite non définis | **SET** : curseur côté répondeur, lots en ordre `≺`, achèvement prouvé, scellé compris ; un seul mécanisme pour les cinq ensembles signés | spec §6.1 |

## Revue 02-fable

| # | Constat | Changement | Où |
|---|---|---|---|
| F2-1 | SIMPLIFIER : OFFER/WANT/Bao réinvente le transport ; un flux par classe, un flux par message > 64 Kio ; reset = nouveau flux ; Bao pour les blobs seulement ; contrôle jamais derrière les données ; pas de crédit pour la présence | **Adopté** : `control`, `repo`, `presence`, `large` ; génération dans le préambule ; OFFER, tirage, minuteries de 120 s, OFFER_EXPIRED, PULL_FAILED, trame REPO_RESET et son renvoi supprimés ; ATTEST et AUTHOR sur le flux REPO avant le message (même effet que l'attestation dans la trame, sans second format) ; présence sans crédit | spec §5, §7 ; ADR option B |
| F2-2a | Parts réservées partout (attestations par signataire, objets par pair, vrac) | voir C3 | spec §10.6 |
| F2-2b | Toute perte de la classe fiable, dont l'éviction d'une modification non couverte (`coverage.ts:105`), DOIT réinitialiser | **Règle L** ; distinction perte / refus (un refus ne réinitialise pas, sinon boucle) | spec §5.5, §9.4, §13 ; inventaire N-14 |
| F2-2c | Réinitialiser seulement sur **zéro progrès**, jamais sur la lenteur ; tailles adaptatives | Plus aucun délai au débit ; seuls l'arrêt complet d'un transfert réservé (60 s) ou d'une demande de vrac (30 s sans octet) échouent ; WANT adaptatif 16 Kio–1 Mio | spec §7.2, §8.1, §10.2 ; inventaire N-15 |
| F2-2d | Interaction renvoi 10 s / cadence ; débordement du `u32` | Renvoi supprimé ; engagements et réessais partagent **un** compteur de cadence ; règle W | spec §7.3 |
| F2-3a | Pli normatif de la liste | §4.9 (repris de `trust/doc.go`) | spec §4.9 |
| F2-3b | Ensemble admis normatif | §4.10 | spec §4.10 |
| F2-3c | Vecteurs REPO/PRESENCE | voir C11 | `repo.json` |
| F2-3d | Dire que les messages non signés n'ont pas besoin de profil déterministe, et ce qu'émet `cbor-x` | §9.2.1–§9.2.2 (et constat N-12 : `encode` sans réglage émet le tag 64) | spec §9.2 |
| F2-3e | `targetId` | voir C10 | |
| F2-3f | Machine à états de l'émetteur | §7.4 | spec §7.4 |
| F2-3g | `docs/protocol/tla/` absent | créé, vérifié par TLC, mutations détectées | `tla/` |
| F2-3h | Mise en page de la tranche Bao dans le texte | §8.1 | spec §8.1 |
| F2-3i | Suite complète de Chalkias | `crypto.json#speccheck-0` … `#speccheck-11`, verdicts calculés par les vrais codecs | `crypto.json` |
| F2-3j | Capacités de sécurité obligatoires côté receveur | `core/2` contient tout ; anciennes capacités retirées | spec §5.4, §20.2 |
| F2-3k | Résiduel CPU/volume de la webview jusqu'au quota N10 | §17 (Majeur jusqu'à N10) | spec §17 |

## Exigences arrivées pendant la révision

| # | Exigence | Changement | Où |
|---|---|---|---|
| N13 | Paquet du protocole réutilisable | Module Go `protocol` (cœur pur, sans E/S) + sous-module `protocol/libp2p` + `@inkwell/protocol` ; vecteurs embarqués, exécuteur de conformité exporté ; inkwell les utilise ; règles de dépendance ; nouvelle formulation de l'invariant libp2p ; licence ; créé à l'étape 1 | spec §1.6, §1.4.7, §21 ; ADR décision 10, étape 1 |
| N14 | Versionné et évolutif | §1.4 : deux sortes de changement ; table de l'inconnu (TLV impair, trames critiques/ignorables, clés `!`, types ignorés, versions d'objet) ; négociation liée (`VERSIONS`, VERSION_DOWNGRADE) ; objets signés lisibles pour toujours, décision `format` pour les changements de pli ; politique de support (N et N−1 pendant 12 mois, message à la personne via identify) ; GREASE à tous les niveaux ; versions des paquets ; **matrice de compatibilité** | spec §1.4, §5.2, §5.3, §9.2.3, §20 ; `compat.json`, `repo.json#compat-*` |
| N15 | Réutiliser les standards | Audit complet (§22) ; remplacés par un standard : OFFER/WANT/Bao et fragments (flux libp2p), REPO_RESET (réinitialisation de flux), octet de classe (multistream-select), délais au débit (ping, inactivité), compteurs de flux (gestionnaire de ressources), profil déterministe (validité RFC 8949 §5.3.1) ; gardés à nous avec raison : objets signés binaires figés (COSE_Sign1 prêt via l'octet `0x84`), liste signée (MLS, Keyhive, p2panda-auth examinés), trames minimales, pages d'historique, ticket | spec §22 |

## Trouvé pendant la révision (hors revues)

| # | Constat | Changement |
|---|---|---|
| R2-1 | Le modèle TLA+ a montré qu'une réactivation répétée pendant que l'autre pair est Bas fait diverger les générations | règle B |
| R2-2 | Le modèle a montré qu'une génération non confirmée rouverte passait pour un doublon | règle A, ligne 3 |
| R2-3 | `cbor-x` sans réglage émet le tag 64 et accepte les clés en double (vérifié) | profil §9.2.1, constat N-12 |
| R2-4 | Les attestations de pairs pas encore actifs (40 000 × 64 pairs) n'avaient pas de total | supprimé : rien n'est lu avant HELLO (constat N-13) |
| R2-5 | « Pas d'ordre petit » laisse passer les clés d'ordre mixte (cas 2 à 9 de Chalkias) | ordre premier (§3.2) |
| R2-6 | Le passage relais → direct exigeait un ordre entre deux connexions | session = connexion (§5.1) |

## Simplifications

Un seul mécanisme par besoin : flux du transport (au lieu de trames + tirage + fragments), une
règle de génération (au lieu d'une machine avec renvoi et minuteries), une règle de parts
réservées (au lieu de bornes au cas par cas), un SET pour cinq ensembles (au lieu de TRUST +
POLICY + …), une capacité `core/2` (au lieu de cinq), une règle d'identité CBOR (`targetId`
partout), aucun flottant, aucun délai au débit. Bilan : 4 mécanismes nommés au lieu de 5 en
révision 1 et 9 aujourd'hui.

## Reste ouvert

- Exploration TLC de la grande configuration (`RepoGen.cfg`) : non exhaustive à ce jour
  (voir `tla/README.md`) ; la petite est exhaustive.
- Formats `final_ack_v1`, `listing_op_v1`, `evidence_v1` et `author_batch` : proposés, sans
  codec ni vecteur (étape 8, avec l'ADR 0031).
- Valeurs « impl. » du §16 à mesurer (bancs) ; R-Reset résiduel si un pair honnête dépasse
  `max(4096, |admis| + 64)` messages en attente de crédit.

---

# Tour 2 : révision 3 (2.0.0-brouillon.2 → 2.0.0-brouillon.3)

Revues [03-codex](reviews/03-codex.md) et [04-fable](reviews/04-fable.md), toutes deux
« revise ». C1, C2, C6, C8, C10, C11, C12 sont confirmés fermés par Codex.

## Revue 04-fable

| # | Gravité | Constat | Changement | Où |
|---|---|---|---|---|
| F4-1 | Critique | Fenêtre de **connexion** QUIC (go-libp2p v0.51.0 : 10 Mio par flux, 15 Mio par connexion) : REPO + LARGE non lus prennent tout, le contrôle cale | **Un seul flux de données peut rester non lu par connexion** : l'ancien REPO est annulé aussitôt (**corrigé au tour 3** : `CancelRead` ne rend le crédit qu'à l'arrivée du RESET_STREAM ou du FIN du pair, quic-go v0.64.1 `receive_stream.go:137-143`, `:410` ; voir F6-1) ; un flux LARGE n'est ouvert qu'après **LARGE_PULL** (réservation faite, lu aussitôt) ; règle normative « connexion ≥ flux + 4 Mio » (vérifiée au démarrage : go-libp2p v0.51.0 n'expose pas d'option pour régler ces fenêtres) ; **modèle `ConnCredit`** : la règle passe, ses deux mutations échouent | spec §5.1, §5.2 (0x0B), §7.2, §10.2, §10.7 ; `tla/ConnCredit.tla` ; ADR P-QUIC |
| F4-2 | Majeur | Les affirmations dépassent le modèle | §7.3 dit **exactement** ce que vérifient `RepoGen` (exhaustif sur la petite configuration) et `ConnCredit`, et ce qui est seulement argumenté ; `RepoGen` reçoit `InboundLost` (réexhaustif, aucune erreur) ; `ConnCredit` couvre crédit de connexion, LARGE et webview arrêtée | spec §7.3, §10.7 ; `tla/` |
| F4-3 | Majeur | Tête fantôme ou relais en lecture : expiration de la couverture → règle L toutes les 30 s entre pairs honnêtes | **Règle H** : expiration, refus et file pleine côté receveur = refus noté, redemandé par `history-request` au plus 1 fois par (pair, document) et par minute ; jamais de règle L ; le magasin garde les attestations d'un (doc, signataire) jusqu'à 256 têtes (la borne de la couverture), et ATTEST_WANT les renvoie toutes | spec §5.5, §6.2, §9.4, §9.6, §13, §16 |
| F4-4 | Majeur | File sortante locale 16 Mio → règle L et LARGE réinitialisé : boucle au démarrage | **Rythme à la source** : entrée progressive des documents (≤ 64 en synchronisation initiale) ; **au plus un message en attente par (pair, document)**, le nouveau remplace l'ancien avec réinitialisation de l'état de synchronisation de ce seul document ; aucune file locale ne déclenche de règle L | spec §7.4, §16.2 |
| F4-5 | Majeur | Passage relais → direct : générations qui repartent de 0, messages tardifs de l'ancienne session acceptés ; limites du gestionnaire de ressources (Codex : nouvelle Critique) | **Époque `(session, génération)`** côté local (PeerUp, Message, Credit, Reset) ; au plus deux sessions par pair, la nouvelle supplante l'ancienne dès qu'elle est Active ; limites réglées à 2 × une session | spec §5.1, §7.3, §11.4, §16.1, Annexe A ; `bridge-internal.json` |
| F4-6 | Majeur (N12) | Vecteurs du pli, des annonces, de G9, des trames et objets | `fold.json` : 7 scénarios (ancienneté, coupe concurrente, révocation contre octroi, dernier admin, sortie, suspension, admin suspendu qui antidate), **calculés par le vrai `trust`**, indépendance de l'ordre vérifiée ; `objects.json` : `author_batch`, `evidence`, `listing_op` (DocId, documentId), `final_ack` (signatures Ed25519 et BLAKE3 réelles), scénarios L3/L4/L5/gel ; `repo.json#g9-*` : 6 vraies modifications Automerge contre R-root/R-nodes ; `session.json` : SET (have, objects, digest), ATTEST_WANT, GOAWAY, LARGE_PULL ; G et R-root/R-nodes écrits dans la spec | `vectors/` ; spec §4.10, §9.7 |
| F4-7a | Mineur | « Les octets signés commencent par l'octet de version » est faux | Texte exact : domaine `/vN` pour tous, octet de version **dans** les octets signés quand il existe, aucun pour `change` | spec §1.4.4 |
| F4-7b | Mineur | 16 trames ignorables par minute trop peu | 256 trames et 256 Kio par minute et par pair ; règle §20.1 pour une extension bavarde (type critique + capacité) | spec §1.4.2, §16.1, §20.1 |
| F4-7c | Mineur | « Suivre » ouvre un flux non compté | Un flux qui suit n'use pas de jeton ; le receveur compte « ≤ 1 + flux que nous avons ouverts dans les 2 s » | spec §7.3 |
| F4-7d | Mineur | Attestation d'un signataire pas encore connu jetée | Gardée en attente comme un orphelin SET (2 périodes, part du livreur), revérifiée à chaque changement de liste | spec §6.2, §16.1 |
| F4-7e | Mineur | WANT « 1 ≤ length » contre multiples de 16 384 | `length = 16 384 × k, 1 ≤ k ≤ 64` | spec §8.1 |
| F4-7f | Mineur | `doc` ≤ 256 contre `document-id` ≤ 64 | Format figé à 256, valable seulement pour un document admis (≤ 64) ; ATTEST_WANT à 1–64 | spec §4.3, §5.2 |
| F4-7g | Mineur | Un ancien nœud cesse de relayer en silence les versions inconnues | Il ne relaie pas ce qu'il ne peut pas vérifier, mais le **dit** à la personne ; `format` empêche ce cas entre nœuds qui l'acceptent | spec §1.4.4 |

## Revue 03-codex (constats partiels et nouveau)

| # | Constat | Changement | Où |
|---|---|---|---|
| C3 | Objet indivisible plus grand que sa part (63 lents × 4 Mio, honnête de 8 Mio) | Admission : sous la part, admis ; au-delà, place libre et **FIFO** ; si un objet attend plus de 60 s, reprise de l'objet en cours le plus lent **et** sous 16 Kio/s (jamais d'un objet qui avance plus vite, jamais quand personne n'attend) | spec §7.2, §10.6, §16.1 |
| C4 | Le `Reset` sur débordement disparaît avant la règle L | Étapes 6 et 7 fusionnées, **derrière `sync2`** ; le retrait du `Reset` se fait dans la même étape que les règles L et H | ADR §Migration |
| C5 | Crédit de connexion | voir F4-1 | |
| C7 | Appariement du dernier SLICE | Exception écrite : la plage qui contient la fin a `length = taille − offset` | spec §8.1 |
| C9 | Doublons détectés par octets (reproduit : `78 04 'type'`) | Comparaison des **clés décodées** dans le générateur et la spec ; vecteur `repo.json#duplicate-key-nonpreferred` (refusé ; l'ancien générateur l'acceptait) | `generate-repo.cjs.txt`, spec §9.2.1 |
| C13 | Un curseur recommencé tous les 15 s affame la fin | Un `have`/`digest` ne recommence pas un curseur actif : il met à jour le filtre, le curseur reprend où il en était | spec §6.1 |
| Nouveau | Passage relais → direct bloqué par la limite d'un flux CONTROL par pair | voir F4-5 | |

## Simplifications de ce tour

La règle L ne couvre plus que le transport (une seule source de réinitialisation) ; tout ce qui
se passe chez le receveur passe par un mécanisme déjà là, les pages d'historique (règle H).
La fenêtre de connexion est protégée par une règle structurelle (un seul flux non lu) plutôt
que par des réglages fins de fenêtres que go-libp2p n'expose pas.

## Reste ouvert après ce tour

- `RepoGen.cfg` (grande configuration) : toujours non exhaustif ; le passage entre sessions
  et les crédits du pont restent argumentés, non modélisés.
- Les scénarios `listing_admit` donnent un verdict tiré du §4.10, pas d'un codec (aucun
  n'existe avant l'étape 8).

---

# Tour 3 : révision 4 (2.0.0-brouillon.3 → 2.0.0-brouillon.4)

Revues [05-codex](reviews/05-codex.md) et [06-fable](reviews/06-fable.md), toutes deux
« revise », convergentes. Codex confirme fermés : C4, C7, C9, C13.

## Divergence à trancher entre les relecteurs

### D1 — admission d'un objet indivisible plus grand qu'une part (Codex C3, Fable F6-3) — *à confirmer par les deux relecteurs*

Constat commun : avec 64 pairs la part `r` vaut 4 Mio ; des pairs qui renouvellent des objets
sous leur part (Codex) ou qui « gouttent » sans jamais finir peuvent priver d'admission un
objet honnête plus grand, et la reprise « sous 16 Kio/s » de la révision 3 est une
réinitialisation au débit qui affame un pair lent honnête (Fable).

| Critère | A — Codex : réservation progressive | B — Fable : page ≤ part, ou reprise au plus une fois | **C — recommandée, appliquée** |
|---|---|---|---|
| Règle | le premier en file accumule toute la place libérée ; personne ne passe devant lui ; pas de reprise | les pages d'historique ne dépassent jamais la part ; un objet repris une fois est ensuite protégé | (1) un receveur ne tire jamais un objet plus grand que sa part (`LARGE_REPLY` 0) et le redemande par pages ≤ `r` (`history-request.max`) — idée de B ; (2) seule reste une modification **indivisible** plus grande que `r` : voie des gros objets de 16 Mio **hors des parts**, un objet à la fois, FIFO avec réservation du premier en file — idée de A ; (3) dans cette voie, le détenteur la garde tant qu'il progresse, et un **quantum** `10 min + taille / 4 Kio/s` seulement si quelqu'un attend |
| Sûreté N7 | renouvellement fermé ; mais des pairs qui gouttent sans finir gardent la place pour toujours | page ≤ part supprime presque tous les gros objets ; mais un objet « protégé » qui goutte garde sa part pour toujours | aucune part n'est jamais reprise ; un pair qui goutte ne retient la voie qu'un quantum ; admission garantie en au plus (rang dans la file × quantum) |
| Vivacité d'un pair lent honnête (ADR 0028) | bonne (jamais repris) | bonne (au plus une reprise) | les objets ≤ part ne sont jamais repris ; dans la voie, un pair à ≥ 4 Kio/s finit toujours dans son quantum ; en dessous, il ne perd son tour que si quelqu'un attend la voie (il faut une modification indivisible > 4 Mio **et** n ≥ 64) |
| Simplicité | moyenne (comptabilité de la réservation accumulée, une seule réserve partagée) | bonne | bonne : le cas normal réutilise les pages d'historique ; une seule petite voie à part |
| Réutilisation | — | pages d'historique | pages d'historique |
| Évolutivité | neutre | neutre | `max` est un champ de `history-request` ; le quantum et la taille de la voie sont des constantes |

Pourquoi C : elle prend à B ce qui supprime le problème (presque aucun objet ne dépasse une
part si le receveur fixe la taille des pages) et à A ce qui garantit l'admission du reste (une
file avec réservation), et elle remplace le seuil de débit par un quantum de partage du temps
qui ne s'applique qu'à une voie rare et seulement quand quelqu'un attend. Ni A ni B seuls ne
résistent à un pair qui goutte sans jamais finir.

## Revue 06-fable

| # | Gravité | Constat | Changement | Où |
|---|---|---|---|---|
| F6-1 | Critique (avec Codex C5) | `CancelRead` ne rend le crédit qu'au RESET_STREAM du pair | Texte corrigé : crédit rendu à l'arrivée du RESET_STREAM ou du FIN (≥ 1 aller-retour, RFC 9000 §3.5/§4.5) ; **un seul flux annulé en attente** par session ; sans RESET_STREAM en **10 s**, GOAWAY PROTOCOL_VIOLATION et fermeture de cette session seule ; exposition bornée : un aller-retour pour un pair honnête (4,5 Mio de marge aux fenêtres maximales ; au tout début d'une connexion le contrôle peut attendre cet aller-retour) ; `ConnCredit` modélise le retour différé et la fermeture : 4 variantes, dont « pair muet sans fermeture » qui échoue | spec §5.1 (c)(d), §7.3, §10.7, §16.1 ; `tla/ConnCredit.tla` |
| F6-2 | Majeur | Deux sessions sans départage : boucle de reconnexion (DCUtR) | `SESSION_NONCE` (`u64`) dans le HELLO ; clé de session = (nonce du plus petit PeerID, nonce de l'autre) ; on garde le direct avant le relayé, puis la plus grande clé : même choix des deux côtés | spec §5.1, §5.3, §20.2 ; `compat.json#nonce-missing` |
| F6-3 | Majeur | Reprise au débit qui affame un pair lent | voir D1 | spec §7.2 |
| F6-4 | Mineur | Règle I « cesser de lire » contre §5.1 « annuler » | Règle I : annuler (STOP_SENDING) le REPO entrant et tout LARGE entrant de l'ancienne génération | spec §7.3 |
| F6-5 | Mineur | 60 s sans progrès sans « après LARGE_REPLY » | Le délai ne court qu'après `LARGE_REPLY` ; attendre sa réponse n'est jamais un échec | spec §7.2, §7.4 |
| F6-6 | Mineur | Destinataire de `history-request` | D'abord le pair qui avait livré le message, puis un autre membre actif qui détient le document, à tour de rôle | spec §5.5 |
| F6-7 | Mineur | Les 64 places de synchronisation initiale sans délai | Une place se libère à la réponse, à un `doc-unavailable`, au refus du document ou après 60 s | spec §7.4 |
| F6-8 | Mineur | 64 + 64 connexions contre 2 par pair | 128 entrantes, 128 sortantes, 2 par pair | spec §16.1 |
| F6-9 | Mineur | CONTROL « toujours lu » et sa file de 512 Kio | Écrit : la lecture ne s'arrête que le temps du traitement local (CPU, disque), jamais à cause du réseau ou de la webview | spec §16.1 |

## Revue 05-codex

| # | Constat | Changement | Où |
|---|---|---|---|
| C3 | Admission d'un objet plus grand qu'une part | voir D1 | spec §7.2, §10.6, §16.1 |
| C5 | Crédit rendu au RESET seulement ; fenêtres **maximales** 10/15 Mio, initiales 512/768 Kio | voir F6-1 ; valeurs effectives écrites (quic-go garde la connexion ≥ 1,5 × tout flux) ; P-QUIC vérifie les **maximums** et la fenêtre initiale | spec §5.1, §16.1 ; ADR P-QUIC |
| Passage | Credit et PeerDown sans session | PeerDown, Credit, Reset et Message portent l'époque `(session, génération)` ; tout message du pont d'une autre époque est ignoré | spec Annexe A ; `bridge-internal.json#bridge-peerdown-epoch` |
| Nouveau, Critique | La règle H ne répare pas une branche concurrente manquante | La réponse contient **toute modification hors du passé causal de `have`**, branches concurrentes comprises ; réponse terminale vide (`changes = []`, `last = true`) ; `history-request.max` | spec §9.6 ; CDDL ; `repo.json#history-request`, `#history-page-terminal`, `#history-page-empty`, `#history-request-no-max` |
| Nouveau | Remplacement par document : régénération ; LARGE > 16 Mio dans une file de 16 Mio | Après réinitialisation de l'état de synchronisation d'un (pair, document), l'adaptateur redemande un message pour ce document, qui prend la place ; la limite de 16 Mio vaut pour la voie en ligne, un LARGE attend seul dans sa voie ; un LARGE annoncé sans réponse est oublié après 10 min ; `LARGE_REPLY` (ex-`LARGE_PULL`) peut refuser | spec §7.2, §7.4, §16 ; `session.json#large-reply-*` |
| Nouveau | Plusieurs attestations gardées contre « seule la plus récente compte » | Règles écrites : acceptation si elle apporte une tête nouvelle, éviction par plus petit `seq` au-delà de 256 têtes, toutes celles gardées comptent ; §4.3 et §17 alignés | spec §4.3, §6.2, §17 |
| Nouveau | Le parseur des vecteurs accepte des clés autour de `__presence` et ignore les clés critiques internes | Enveloppe fermée (une seule clé) et recherche des clés `!` dans le CBOR interne ; vecteurs `repo.json#presence-extra-key`, `#presence-inner-critical` | `generate-repo.cjs.txt` |
| Portes | Garder toutes les portes ; étendre ConnCredit au délai FIN/RESET | Toutes les portes gardées (P-COMPAT comprise) ; P-TLA et P-QUIC précisées ; ConnCredit étendu | ADR §Confirmation |

## Simplifications de ce tour

Presque aucun objet ne dépasse plus une part : le receveur fixe la taille des pages. Le seuil
de débit disparaît ; il ne reste qu'un quantum de partage du temps, sur une voie rare.

## Reste ouvert après ce tour

- D1 à confirmer par les deux relecteurs.
- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; objets proposés sans codec, admission des annonces déclarative (avant bascule,
  étape 8).

---

# Tour 4 : révision 5 (2.0.0-brouillon.4 → 2.0.0-brouillon.5)

Revues [07-codex](reviews/07-codex.md) et [08-fable](reviews/08-fable.md) (« revise », aucune
Critique) et débat [09-debate-D1](reviews/09-debate-D1.md) : **accord** des deux relecteurs.

## D1 — accord, appliqué

« C (pages ≤ part) + tranches Bao reprenables via §8.1 pour les modifications indivisibles plus
grandes qu'une part, tirées par empreinte depuis l'annonce LARGE, mises en attente sur disque
par auteur sous le quota N10, ordonnancées en DRR, sans quantum, sans voie, sans
réinitialisation sur la lenteur. » Appliqué :

| Élément de l'accord | Où |
|---|---|
| `LARGE{id, size, flags (bit indivisible), root = BLAKE3, change, doc}` ; le bit n'est posé qu'en réponse à `history-request` pour une seule modification plus grande que `max` | spec §5.2, §7.2 ; `session.json#large-indivisible`, `#large-indivisible-no-change`, `#large-flags-reserved` |
| tirage par `root` chez l'annonceur puis chez tout détenteur ; règle de service étendue (« ou une modification indivisible annoncée ou détenue », document admis, demandeur membre actif) ; l'annonceur garde la modification indexée par `root` tant qu'une demande est en cours | spec §8.1 |
| liaison document, auteur, empreinte Automerge, taille, racine Bao ; enveloppe CBOR faite localement ; chaîne de réception normale | spec §8.1 |
| attente **sur disque par auteur**, quota N10 ; d'ici là 1 modification ≤ 16 Mio par auteur et par receveur ; LRU dans la part de l'auteur, jamais sur un délai ; reprise persistée ; DRR ; 3 échecs → autre détenteur, puis règle H | spec §8.1, §16.1 |
| **voie des gros objets supprimée** : plus de quantum, de FIFO, de reprise | spec §7.2, §16.1, §10.6 |

Constats devenus sans objet grâce à l'accord : Fable « pas de signal d'éligibilité à la voie »
(le bit indivisible et `doc` sont dans LARGE) ; Fable « oubli à 10 min contre attente dans la
file de la voie » et « fin de file non définie » (plus de voie ni de file ; avec des parts fixes
un receveur honnête répond aussitôt, l'oubli à 10 min ne vise qu'un pair qui ne répond pas) ;
Codex « une modification de 16 Mio plus son enveloppe CBOR dépasse la voie » (plus de voie ;
le vrac transporte la modification brute, l'enveloppe est faite localement) ; Codex « refuser
LARGE ne dit ni le document ni s'il est indivisible » (`doc` et le bit sont dans l'annonce).

## Revue 07-codex

| # | Constat | Changement | Où |
|---|---|---|---|
| C3 | Les parts rétrécissent quand des pairs arrivent, les réservations ne sont jamais reprises : admission non prouvée | **Parts fixes** pour la mémoire réservée d'avance : `r = 256 Mio / 64 = 4 Mio` (64 = maximum de pairs actifs). Preuve : chaque pair a au plus une réservation ≤ `r`, donc la somme ≤ 64 × `r` = 256 Mio ; tout nouvel objet ≤ `r` est admis aussitôt, quelle que soit l'arrivée de pairs ; rien à renégocier. Les parts dynamiques restent pour ce qui est reprenable | spec §10.6, §16.1 |
| C5 | La règle I annule REPO **et** LARGE alors qu'une seule annulation peut être en attente ; ConnCredit ne modélise que REPO | La règle I n'annule plus que REPO ; un LARGE entrant de l'ancienne génération est lu jusqu'au bout et jeté (déjà dans sa réservation), il ne retient jamais de crédit ; ConnCredit est donc juste par construction (dit dans le modèle) | spec §5.1 (c), §7.3 ; `tla/ConnCredit.tla` |
| Nouveau | `LARGE_REPLY` sans génération | `LARGE_REPLY{gen, id, verdict}` ; une réponse d'une autre génération est ignorée | spec §5.2, §7.2 ; `session.json#large-reply-*` |
| Nouveau | Métadonnées de LARGE pour la règle H | voir D1 | |
| Nouveau | Enveloppe CBOR et voie de 16 Mio | sans objet (voir D1) ; `history-request.max` compte l'enveloppe | spec §9.6 |

## Revue 08-fable

| # | Constat | Changement | Où |
|---|---|---|---|
| F8-1 | Pas de signal d'éligibilité à la voie | sans objet (D1) | |
| F8-2 | Oubli à 10 min contre attente FIFO ; fin de file | sans objet (D1) | |
| F8-3 | LARGE annulé non compté | La règle I ne l'annule plus : lu jusqu'au bout et jeté | spec §5.1 (c), §7.3 |
| F8-4 | La marge de 4,5 Mio ne tient que pour une bascule | §5.1 (d) récrit : la vraie garantie est « le contrôle peut attendre jusqu'au RESET_STREAM : ≤ 1 aller-retour avec un pair honnête, ≤ 10 s avant fermeture sinon » | spec §5.1 (d) |
| F8-5 | Départage à clés égales | Puis la session Active la plus ancienne | spec §5.1 |
| F8-6 | Ligne émetteur du §11.2 | États `Annoncé → Demandé → Écrit → Fini`, `Refusé`, `Oublié` ; plus de « reprise de part » | spec §7.4, §11.2 |
| F8-7 | `HonestNoTeardown` vacante | Supprimée ; variante reproductible `tla/ConnCreditHonest.cfg` (pair honnête sans règle de fermeture : aucune erreur) | `tla/` |

## Simplifications de ce tour

La voie des gros objets, son quantum et sa file disparaissent ; les rares modifications
indivisibles réutilisent le vrac (Bao, reprise, DRR) déjà spécifié pour les blobs. Les parts
fixes de la mémoire réservée suppriment toute renégociation.

## Reste ouvert après ce tour

- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; objets proposés sans codec (étape 8).

---

# Tour 5 : révision 6 (2.0.0-brouillon.5 → 2.0.0-brouillon.6)

Revues [10-codex](reviews/10-codex.md) et [11-fable](reviews/11-fable.md) : « revise », aucune
Critique, aucune divergence. Codex confirme fermés : C3, C5, métadonnées de LARGE, liaison de
`LARGE_REPLY`.

## Revue 11-fable

| # | Gravité | Constat | Changement | Où |
|---|---|---|---|---|
| F11-1 | Majeur | Remplacement LRU à l'arrivée : un auteur honnête avec deux modifications indivisibles fait tourner en rond un receveur lent | Un tirage en cours n'est **jamais** remplacé ; la deuxième attend (règle H) ; une entrée ne sort que **finie** ou **plus voulue** (document plus admis, obtenue autrement, principal retiré) ; jamais sur un délai | spec §8.1, §16.1 |
| F11-2 | Majeur (avec Codex) | Attente rangée dans un cache nettoyé à 24 h ; « §14.6 » inexistant | Nouveau §14 point 7 : sous le dossier de données du projet (`.inkwell/staging/<principal>/<root>`), carte des groupes vérifiés écrite atomiquement, pas un cache, sans limite de durée, prime le point 6 ; reprise après plus de 24 h testée (P-BORNES) | spec §8.1, §14 |
| F11-3 | Mineur | La réinitialisation d'un LARGE d'une époque passée compte comme une perte | La règle T vaut aussi pour les **événements** : fin, réinitialisation ou échec d'un flux d'une époque passée sont ignorés ; seul un échec de l'époque courante est une règle L | spec §7.2 (5), §7.3 |
| F11-4 | Mineur | « Garde indexée tant qu'une demande est en cours » non défini | **Index persistant** `root → (document, empreinte)` de toute modification indivisible annoncée, reçue ou obtenue, servi tant que le document est admis : couvre l'attente du premier WANT et les redémarrages | spec §7.2, §8.1 |
| F11-5 | Mineur | Principal provisoire avant les tuples d'auteur | Principal = clé d'auteur du tuple, et d'ici l'étape 8 le pair qui a fait l'annonce | spec §8.1 |
| F11-6 | Mineur | Annonce indivisible > 16 Mio sans issue | Violation (G1), refus définitif jamais redemandé | spec §7.2, §16.1 ; `session.json#large-indivisible-too-big` |
| F11-7 | Mineur | Bornes 67 108 797 alors que r = 4 Mio | Préambule LARGE et `history-request.max` ≤ 4 194 304 ; une annonce ordinaire plus grande n'est qu'un **avis** (l'émetteur ne garde rien, le receveur répond 0 et passe aux pages) | spec §5.1, §7.2, §16.1 ; CDDL ; `session.json#large-preamble-above-share` |
| F11-8 | Mineur | Règle H pendant un tirage | Suspendue pour le document D tant qu'un tirage pour D est en cours | spec §8.1 |
| F11-9 | Mineur | Vecteurs de la liaison indivisible | `repo.json#indivisible-binding-ok`, `#indivisible-hash-mismatch`, `#indivisible-size-mismatch` (vraie modification Automerge de ~70 ko), `#indivisible-serve-unadmitted` (MISSING) | `vectors/` |
| F11-10 | Mineur (avec Codex) | Reliquat « reprise sous contention » dans l'ADR ; ligne receveur du §11.2 | Conséquence récrite (parts fixes, pages, rien n'est repris) ; ligne receveur avec `Refusé` | ADR §Conséquences ; spec §11.2 |
| F11-11 | Remarque | Coût des pages successives | Écrit (`⌈H / 4 Mio⌉`, 50 pages pour 200 Mio) ; deux pages en vol par (pair, document) permises | spec §9.6 |

## Revue 10-codex

| # | Constat | Changement | Où |
|---|---|---|---|
| Enveloppe | Une modification brute ≤ `max` dont la page encodée dépasse `max` n'a pas de route | L'indivisibilité se décide sur la **page singleton encodée** (enveloppe comprise) ; vecteurs de bord `repo.json#page-route-fits` (max = taille encodée → page) et `#page-route-envelope-overflow` (max − 1 → indivisible, alors que la modification brute tient) | spec §7.2, §9.6 |
| D1 / cache 24 h | voir F11-2 | | |
| Reliquat ADR | voir F11-10 | | |

## Reste ouvert après ce tour

- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; codecs `sync2` et objets proposés sans implémentation (étapes 4 et 8).

---

# Tour 6 : révision 7 (2.0.0-brouillon.6 → 2.0.0-brouillon.7)

Revues [12-codex](reviews/12-codex.md) et [13-fable](reviews/13-fable.md) : « revise », aucune
Critique, aucune divergence ; **le même Majeur trouvé indépendamment par les deux** (un annonceur
peut immobiliser l'emplacement d'attente avec une fausse racine). Codex confirme fermés :
enveloppe, attente hors du cache de 24 h, reliquat de l'ADR.

| # | Source | Constat | Changement | Où |
|---|---|---|---|---|
| T6-1 | Fable F13-2, Codex (Majeur) | Fausse racine et « goutte-à-goutte » : la règle H suspendue prive le receveur du document ; après l'étape 8, un relais occupe l'unique emplacement d'un auteur honnête avec son vrai `change` | Tout ce qui n'est pas vérifié est **imputé au pair annonceur**, toujours ; le quota N10 de l'auteur n'est crédité qu'après vérification de `change` ; **suspension de la règle H supprimée** (pages toujours demandables à d'autres détenteurs) ; **dédoublonnage par `root`** ; plus de principal provisoire ni de dépendance à l'étape 8 ; vecteur de rejeu `repo.json#indivisible-tuple-false-root` ; scénarios ajoutés à P-BYZ | spec §8.1, §16.1 ; ADR P-BYZ |
| T6-2 | Codex (PARTIEL) | Plusieurs relais contournent une limite par auteur réel | Borne écrite honnêtement : 1 tirage ≤ 16 Mio par pair annonceur et par receveur, ≤ 1 Gio au total ; rien ne limite *par auteur réel* avant vérification ; dédoublonnage par `root` | spec §8.1 |
| T6-3 | Codex (Majeur) | Index persistant sans borne, imputation ni suppression | Insertion **après vérification et admission** seulement (ou modification produite localement) ; retrait avec le document (désadmis, purgé) ou la modification ; borne : ≤ 1 entrée par 4 Mio d'historique vérifié (une modification indivisible fait plus de 4 Mio encodée), imputée au quota N10 de l'auteur | spec §8.1, §16.1 |
| T6-4 | Fable F13-3 | Deux pages en vol sûres seulement si une page est un préfixe causalement clos | Règle écrite : chaque dépendance d'une modification d'une page est dans le passé de `have` ou plus tôt dans la page | spec §9.6 |
| T6-5 | Fable F13-4 | Un LARGE d'une époque passée peut goutter et garder la réservation | Le délai de 60 s sans progrès s'applique aussi à lui (réinitialisé, réservation libérée) ; seule la règle L est omise ; il ne retient que la part du pair | spec §7.2 (5) |
| T6-6 | Fable F13-5 | Un avis doit franchir le pont pour calculer `root` | Un avis porte `root` et `change` nuls, s'annonce d'après la taille seule, sans passer les octets par le pont (kind 19 `Advice`) ; une annonce ordinaire ≤ `r` doit avoir un `root` non nul ; vecteurs `session.json#large-advice`, `#large-ordinary-zero-root`, `#large-advice-with-root` | spec §7.1, §7.2, Annexe A |

## Simplification de ce tour

Une seule règle d'imputation (le pair annonceur) remplace « principal = auteur, sauf avant
l'étape 8 » ; la suspension de la règle H disparaît.

## Reste ouvert après ce tour

- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; codecs `sync2` et objets proposés sans implémentation (étapes 4 et 8).

---

# Tour 7 : révision 8 (2.0.0-brouillon.7 → 2.0.0-brouillon.8)

Revues [14-codex](reviews/14-codex.md) et [15-fable](reviews/15-fable.md) : « revise », aucune
Critique, aucune divergence ; le même Majeur trouvé par les deux (borne de l'index fausse si
`max` < `r`). Codex confirme fermés : rejeu tuple/racine, principal provisoire.

| # | Source | Constat | Changement | Où |
|---|---|---|---|---|
| T7-1 | Fable F15-2, Codex (Majeur) | L'indivisibilité dépend du `max` du demandeur, l'index et sa borne supposent `max = r` | **`max` fixé à `r` = 4 194 304** (CDDL, spec ; autre valeur refusée, `repo.json#history-request-wrong-max` ; le champ reste pour N14) ; « annoncée ou détenue » remplacé par « présente dans l'index du serveur » ; borne de l'index juste ; vecteurs de bord **à `max = r`** : `#page-route-fits` (modification de 4 194 098 o, page 4 194 304 o → page) et `#page-route-envelope-overflow` (4 194 099 o, page 4 194 305 o → indivisible), octets non recopiés (empreinte SHA-256 et recette de régénération) | spec §7.2, §8.1, §9.6 ; CDDL ; `repo.json` |
| T7-2 | Codex (nouveau Majeur) | 64 pairs actifs ne bornent pas les attentes laissées par des pairs déconnectés | **Plafond global de 1 Gio, pairs absents compris** ; une entrée est active si un détenteur connecté la sert ; au plafond : d'abord les finies ou plus voulues, puis les inactives en LRU ; jamais une active ; un pair connecté obtient toujours son emplacement (actives ≤ 64 × 16 Mio) ; scénario « 65 membres successifs » dans P-BYZ | spec §8.1, §14 (7), §16.1 ; ADR P-BYZ |
| T7-3 | Fable F15-3 | Un relais pressé goutte une plage, le détenteur honnête n'est jamais interrogé | **Plages doublées** : après 30 s, une plage en cours peut être demandée aussi à un autre détenteur ; la première réponse complète gagne ; scénario dans P-BYZ | spec §8.1 ; ADR P-BYZ |
| T7-4 | Fable F15-4 | État de synchronisation après un avis | Réinitialisé pour ce (pair, document), comme un message remplacé | spec §7.2 |
| T7-5 | Fable F15-5 | Attente liée au premier annonceur, perdue à son retrait | Attente indexée **par `root`** avec ses détenteurs ; l'imputation passe à un autre détenteur annonceur | spec §8.1, §14 (7) |
| T7-6 | Fable F15-5 | kind 19 `doc str8` contre kind 4 `doc str16` | `doc str16 (1–64)` ; vecteur `bridge-internal.json#bridge-advice` (Codex) | spec Annexe A |
| T7-7 | Codex (Mineur) | Le vecteur de rejeu tronquait la modification (« not enough data ») | Rejeu avec une **autre modification valide** : l'empreinte reçue est calculée, différente de `change`, imputation à l'annonceur vérifiée | `repo.json#indivisible-tuple-false-root` |

## Reste ouvert après ce tour

- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; codecs `sync2` et objets proposés sans implémentation (étapes 4 et 8).

---

# Tour 8 : révision 9 (2.0.0-brouillon.8 → 2.0.0-brouillon.9)

Revues [16-codex](reviews/16-codex.md) et [17-fable](reviews/17-fable.md) : « revise », **un seul
Majeur, le même** chez les deux, aucune Critique, aucune divergence. Codex confirme fermés :
index (`max = r`), rejeu, vecteur `bridge-advice`.

| # | Source | Constat | Changement | Où |
|---|---|---|---|---|
| T8-1 | Codex, Fable F17-2 (Majeur) | Deux définitions d'« active » : un pair connecté qui sert plusieurs racines laissées par des absents en rend plusieurs actives hors de tout emplacement (Codex : 63 + 1 × 2 → 65 actives, > 1 Gio ; Fable : X met en attente R1…Rn puis part, Y les annonce toutes et goutte) | **Une définition** : chaque pair connecté possède **un** emplacement de 16 Mio ; une entrée est active **si et seulement si** elle occupe l'emplacement d'un pair connecté qui la détient ; seules les actives sont tirées ; une entrée dont les détenteurs connectés ont tous leur emplacement pris est inactive. Attribution (annonce nouvelle, dédoublonnage, départ du pair attribué, redémarrage) **seulement dans un emplacement libre**, avec **réservation atomique** de la taille ; au redémarrage, réattribution une par emplacement. D'où actives ≤ pairs connectés × 16 Mio ≤ 1 Gio, et une réservation dans l'emplacement d'un pair connecté réussit toujours (en retirant les finies, puis les inactives). Les deux contre-exemples sont des scénarios de P-BYZ | spec §8.1, §16.1 ; ADR P-BYZ |
| T8-2 | Fable F17-3 | « Retiré de la liste » ambigu | « Retiré de la liste des membres », distingué de « se déconnecte » ; les deux rendent l'entrée inactive | spec §8.1 |
| T8-3 | Codex (Mineur) | « PEUT doubler » ne garantit pas l'absence de retard | **DOIT** doubler une plage en cours depuis 30 s s'il existe un autre détenteur | spec §8.1 |
| T8-4 | Fable F17-4 | Borne « 1 par 4 Mio » | « ≈ 4 Mio » (plus que `r` moins l'enveloppe) | spec §8.1 |
| T8-5 | Fable F17-4 | §17 ne cite pas le plafond d'attente | Ajouté à « Épuisement » | spec §17 |
| T8-6 | Fable F17-4 | L'ADR cite encore « brouillon.2 » | « brouillon.9 » | ADR §Décision |

## Reste ouvert après ce tour

- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; codecs `sync2` et objets proposés sans implémentation (étapes 4 et 8).

---

# Tour 9 : révision 10 (2.0.0-brouillon.9 → 2.0.0-brouillon.10)

Revues [18-codex](reviews/18-codex.md) (« revise ») et [19-fable](reviews/19-fable.md)
(« **accept** »). La divergence sur l'équité des emplacements a été tranchée par le débat
[20-debate-D2](reviews/20-debate-D2.md) : **accord**, appliqué ici.

## D2 — accord, appliqué

| Élément de l'accord | Où |
|---|---|
| À chaque libération ou disponibilité d'un emplacement (fin ou abandon du tirage, connexion, redémarrage), réservation **atomique** pour l'entrée inactive éligible **de plus petit rang d'attente** (ordre de première annonce), **avant toute nouvelle annonce** | spec §8.1, §16.1 |
| Rang conservé à travers les réannonces (dédoublonnage) et les redémarrages (persisté) | spec §8.1 |
| Aucun transfert actif préempté ; plafonds et règle H inchangés | spec §8.1 |
| Hypothèse de vivacité écrite : au moins un détenteur honnête, connecté et disponible ; borne : entrées de rang plus petit chez ce détenteur × un transfert de 16 Mio | spec §8.1 |
| Scénario « R face à des annonces renouvelées » dans P-BYZ | ADR P-BYZ |
| **À confirmer par les deux relecteurs** : la variante de Fable « plus récemment avancée d'abord » au seul redémarrage n'est pas retenue ; une seule règle, l'ordre d'attente partout | spec §8.1 |

**Éviction des inactives et ordre d'attente** (vérification demandée) : l'éviction au plafond
pouvait retirer une entrée honnête en file avant son tour. Précédence désormais écrite : finies ou
plus voulues, puis inactives **sans détenteur connecté** (LRU), puis seulement les inactives
éligibles, du plus grand rang au plus petit ; et retirer une entrée retire ses **octets**, jamais
son rang (enregistrement de ~100 octets, ≤ 64 par pair). Une entrée en file peut donc perdre ses
octets si le plafond est plein d'actives, mais jamais son tour : la borne d'attente ne change pas
(elle repart de zéro à son tour). Scénario ajouté à P-BYZ.

## Revue 18-codex

| # | Constat | Changement | Où |
|---|---|---|---|
| Vivacité (Majeur) | Une ancienne entrée peut rester inactive si une nouvelle annonce passe toujours devant | voir D2 | spec §8.1 |
| Règle H sans borne (Majeur) | Redemander ne garantit pas l'admission | Borne d'attente établie sous l'hypothèse d'un détenteur honnête connecté ; sans lui, aucune garantie possible, la règle H reste le recours (écrit) | spec §8.1 |

## Revue 19-fable (accept)

| # | Constat | Changement | Où |
|---|---|---|---|
| F19-3 | Pas de déclencheur à la libération ; ordre non défini | voir D2 (FIFO, pas « plus récemment avancée ») | spec §8.1 |
| F19-4 | Un détenteur retiré de la liste des membres doit sortir de la liste des détenteurs | Écrit ; le doublement d'une plage exige un autre détenteur **connecté et membre** | spec §8.1 |
| F19-5 | « ≤ 1 par 4 Mio » contre « ≈ 4 Mio » | §16.1 aligné (« ≈ 4 Mio ») | spec §16.1 |

## Reste ouvert après ce tour

- Confirmation par les deux relecteurs de l'ordre d'attente unique (y compris au redémarrage).
- `RepoGen.cfg` non exhaustif ; passage entre sessions et crédits du pont argumentés, non
  modélisés ; codecs `sync2` et objets proposés sans implémentation (étapes 4 et 8).

## Tour 10 — confirmation (2026-11-01)

Codex (`reviews/21-codex.md`) et Fable (`reviews/22-fable.md`) **acceptent** la révision 10. Seule remarque commune,
mineure : la cellule « modifications indivisibles en attente » du §16.1 résumait l'éviction en « inactives en LRU » ;
elle reprend désormais l'ordre complet du §8.1 (finies ou plus voulues, puis inactives sans détenteur connecté en LRU,
puis éligibles du plus grand rang au plus petit ; octets retirés, rang gardé). Aucun changement de règle.
