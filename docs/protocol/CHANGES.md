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
| F4-1 | Critique | Fenêtre de **connexion** QUIC (go-libp2p v0.51.0 : 10 Mio par flux, 15 Mio par connexion) : REPO + LARGE non lus prennent tout, le contrôle cale | **Un seul flux de données peut rester non lu par connexion** : l'ancien REPO est annulé aussitôt (`CancelRead` rend le crédit, quic-go v0.64.1 `receive_stream.go:400-410`) ; un flux LARGE n'est ouvert qu'après **LARGE_PULL** (réservation faite, lu aussitôt) ; règle normative « connexion ≥ flux + 4 Mio » (vérifiée au démarrage : go-libp2p v0.51.0 n'expose pas d'option pour régler ces fenêtres) ; **modèle `ConnCredit`** : la règle passe, ses deux mutations échouent | spec §5.1, §5.2 (0x0B), §7.2, §10.2, §10.7 ; `tla/ConnCredit.tla` ; ADR P-QUIC |
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
