---
status: proposée
date: 2026-11-01
revision: 6 (revues 01 à 11, débat D1 conclu par un accord ; exigences N13, N14, N15)
decision-makers: [Mathieu]
---

# Un protocole unique, public et borné : un flux par classe, générations dans le préambule, parts réservées, paquet réutilisable

## Contexte et problème

Les échanges d'inkwell ont grandi étape par étape : canaux multiplexés dans un flux libp2p,
fragmentation, priorités, réassemblage, débordement sur disque, éviction, boîtes d'envoi à
deux voies, files d'attente dans la webview, crédits inertes. Chaque revue trouve un défaut du
même genre ([inventaire](../protocol/inventaire.md) §4–§5) : une boucle partagée qui attend un
pair (R-Serve, N-2 à N-6), une perte silencieuse dans une classe qui ne la tolère pas (R-Evict,
N-14), un débordement transformé en réinitialisation répétée (R-Reset), une version décodée
avant d'être négociée (R-V2), une écriture non atomique (R-Save), une clé Ed25519 jamais validée
(N-1, Critique). L'utilisatrice demande « un protocole sûr et simple sur tout ce qui existe »,
« clair, défini et bien fait », que d'autres applications puissent parler (N12), et « un
package spécifique pour le protocole, qui peut être utilisé par quelqu'un d'autre » (N13), un
protocole versionné qui reste compatible et peut évoluer (N14), et qui ne réinvente pas ce qui
existe déjà (N15).

La révision 1 de cette ADR (un flux ordonné de petites trames par pair, gros messages tirés
par le receveur avec OFFER/WANT/Bao) a reçu deux revues indépendantes :
[01-codex](../protocol/reviews/01-codex.md) (« revise » : chaîne de réception incomplète,
REPO_RESET doublé par d'anciens messages en file, plafonds partagés sans parts, migration qui
active avant de vérifier) et [02-fable](../protocol/reviews/02-fable.md) (« sound with
changes » : OFFER/WANT/Bao réinvente le transport ; utiliser un flux par classe et un flux par
gros message). Cette révision 2 répond aux deux ; chaque point est relié à son changement dans
[CHANGES.md](../protocol/CHANGES.md).

## Facteurs de décision

- Modèle byzantin complet (N7) : un membre malveillant ne gêne jamais les autres.
- Invariants de `CLAUDE.md` et ADR 0002, 0004, 0011, 0020, 0023, 0028, 0030–0033.
- ADR 0028 : un gros transfert ne retarde pas la frappe ; WAN lent (un pair lent finit, même
  lentement) ; très gros fichiers.
- Simplicité : un mécanisme général plutôt que des cas particuliers ; moins de code à prouver.
- Interopérabilité : spécification autonome, vecteurs, paquets de référence (N12, N13).
- Vérifier avant d'activer : modèle formel et portes de sécurité **avant** la bascule.

## Options envisagées

**A. Garder la forme actuelle et corriger au cas par cas.**

**B. Un flux par classe et par sens, un flux par message de plus de 64 Kio, générations dans
le préambule du flux REPO** ; contrôle de flux du transport par flux ([RFC 9000 §4.1], fenêtre
yamux) ; le changement de génération est la réinitialisation du flux ([RFC 9000 §3.5]).

**C. Un seul flux ordonné de petites trames par pair, gros messages tirés par le receveur
(OFFER, WANT, Bao)** — retenue en révision 1.

**D. Adopter un protocole tiers complet** (synchronisation automerge-repo par serveur,
iroh-docs/iroh-gossip, Hypercore).

| Critère | A | B | C | D |
|---|---|---|---|---|
| Boucle partagée qui attend un pair | à corriger partout | non | non | dépend |
| Contrôle derrière des données | oui (un flux) | **non** (flux CONTROL séparé, toujours lu) | oui : il faut prouver l'absence d'auto-blocage et lire les en-têtes | dépend |
| Perte silencieuse d'un message fiable | éviction, TTL | règle L : toute perte change de génération | idem, plus OFFER_EXPIRED et PULL_FAILED | dépend |
| Anciens messages après une réinitialisation | à étiqueter | **impossibles** : le flux réinitialisé jette son contenu, la génération est dans le préambule | étiquette de génération par trame et annulation atomique des files | — |
| Mémoire face à un pair qui « goutte » | éviction, disque | sa part réservée seulement (§10.6) | idem, plus minuteries de 120 s | dépend |
| Frappe pendant un gros transfert | portes | flux séparés | trames ≤ 96 Kio, gros sur le vrac | — |
| Mécanismes à maintenir | 9 | **4** : flux par classe + préambule, générations, parts réservées, crédits du pont | 5 : trames + HELLO, tirage OFFER/WANT/Bao, générations par trame, crédits, parts | 0 côté fil, adaptation lourde |
| Taille du modèle formel | — | petite (TLC exhaustif en 5 s sur la petite configuration) | plus grande (minuteries, tirage) | — |
| Respect des invariants | oui | oui (libp2p dans `p2p` et `protocol/libp2p`) | oui | **non** : serveur (ADR 0004), Go qui comprend le CRDT (ADR 0011), pas d'admission byzantine (ADR 0031) |

La révision 1 avait écarté B pour trois raisons, chacune levée : « réinitialiser un flux = une
perte à signaler » → c'est exactement le changement de génération, et la règle L le dit
partout ; « barrière de séquence pour l'attestation » → ATTEST et AUTHOR voyagent sur le flux
REPO, avant le message ; « réservation ou éviction » → parts réservées (§10.6). Bao n'apporte
rien à l'intérieur d'une connexion authentifiée avec le pair qui a produit le message : il
reste pour ce qui peut venir de n'importe quel détenteur — les blobs, et les rares
modifications indivisibles plus grandes qu'une part, tirées par leur racine (accord D1).

Encodage, comparé séparément. Les objets signés restent en binaire fixe (le `ProjectID` en
dépend). Les messages CBOR de la couche dépôt ne sont pas signés : un **profil de validité**
qui refuse exactement ce que deux décodeurs liraient différemment (doublons, étiquettes,
flottants, entiers non exacts en JavaScript, profondeur) suffit ; un profil déterministe
exigé à la réception n'ajouterait que des refus d'encodages équivalents (`cbor-x` réglé comme
automerge-repo n'est pas déterministe). La sérialisation préférée est recommandée à
l'émission.

## Décision

Option retenue : **B**, décrite normativement dans
[docs/protocol/spec.md](../protocol/spec.md) (2.0.0-brouillon.2), avec :

1. **Clés** (§3.2) : décodage du point (RFC 8032 §5.1.3), encodage canonique, sous-groupe
   d'ordre premier (`[L]A = O`, `A ≠ O`), à chaque entrée **et au rechargement** ; verdict de
   signature exact (§3.3) et les douze cas de Chalkias en vecteurs.
2. **Spécification autonome** : pli normatif de la liste (§4.9), ensemble admis (§4.10),
   garde structurelle G1–G12 (§9.7), chaîne de réception complète (§9.4, §12), formats des
   objets de l'ADR 0031 (accusé, annonce, tuple d'auteur, preuve).
3. **Une seule capacité, `core/2`, qui contient tout** ce qui porte la sécurité ; rien de
   sécuritaire ne se négocie (§5.4). Les identifiants `member-op/2`, `policy/1`, `history/1`,
   `changesig/1` sont retirés.
4. **Un flux par classe, identifié par son protocole libp2p** (`/inkwell/control/2.0.0`,
   `/inkwell/repo/2.0.0`, `/inkwell/presence/2.0.0`) et un flux `/inkwell/large/2.0.0` par
   message de plus de 64 Kio, ouvert seulement après `LARGE_REPLY` (§5.1, §7.2) ; un seul flux de
   données peut rester non lu par connexion, ce qui protège le contrôle de la fenêtre de
   connexion de QUIC ; nombre de flux borné par le gestionnaire de ressources de libp2p (deux
   fois les limites d'une session, pour le passage relais → direct). Une session = une
   connexion ; côté local, l'époque `(session, génération)`.
5. **Générations dans le préambule du flux REPO** ; règles I, B, A, R, T, W ; cadence partagée
   par engagements et réessais ; vérifiées par le modèle TLA+ ([tla/](../protocol/tla/)).
6. **Règles L et H** (§5.5) : une perte en transport change de génération (L) ; ce que le
   receveur a reçu mais ne garde pas (refus, attente expirée, file pleine) est redemandé par
   pages d'historique (H), sans jamais toucher à la génération ; aucune file **locale** ne
   déclenche de réinitialisation (rythme à la source, §7.4).
7. **SET** : un seul mécanisme d'anti-entropie pour les cinq ensembles signés, par lots en
   ordre causal avec curseur et achèvement prouvé (§6.1) ; **ATTEST_WANT** : réconciliation des
   attestations tirée par le receveur (§6.2).
8. **Parts réservées** sur tout plafond partagé (§10.6), **fixes** (`C / 64`) pour la mémoire
   réservée d'avance des objets LARGE, dynamiques pour ce qui est reprenable ; un objet plus
   grand que sa part est redemandé par pages ≤ part, ou, s'il s'agit d'une modification
   indivisible, tiré par le vrac en tranches Bao reprenables mises en attente sur disque par
   auteur (accord D1 entre les relecteurs) ; **aucun délai sur la lenteur**, seul
   l'arrêt complet d'un transfert réservé échoue (§10.2) ; pas d'auto-blocage (§10.7).
9. **Profil de validité CBOR** (§9.2.1) ; aucun flottant (pointeur en dix-millièmes) ;
   `targetId` partout.
10. **Paquets de référence** (§1.6, N13) : module Go `github.com/oioio-space/inkwell/protocol`
    (sous-module de ce dépôt, `go.mod` propre, `v0.x` jusqu'à l'acceptation), sous-module
    `protocol/libp2p`, paquet TypeScript `@inkwell/protocol` ; vecteurs embarqués et exécuteur
    de conformité exporté ; inkwell les utilise lui-même.
11. **Versionnement et évolution** (§1.4, N14) : extensions compatibles sans changement de
    majeure ; règle définie pour tout inconnu (TLV pair ignoré / impair à comprendre, trames
    critiques `0x01`–`0x3F` / ignorables `0x40`–`0x7F`, clés CBOR `!…` à comprendre, types de
    message inconnus ignorés, versions d'objet inconnues ignorées sans sanction) ;
    négociation liée par `VERSIONS` dans le HELLO (VERSION_DOWNGRADE) ; objets signés lisibles
    pour toujours ; décision `format` pour tout changement de pli ; majeures N et N−1 pendant
    12 mois ; message clair face à un pair trop ancien ou récent ; GREASE à tous les niveaux ;
    matrice de compatibilité en vecteurs.
12. **Réutilisation d'abord** (§22, N15) : chaque mécanisme est comparé à l'existant ; une
    couche à nous seulement avec sa raison écrite.

**Invariants de `CLAUDE.md` à reformuler à l'acceptation** (cette ADR ne modifie pas le
fichier) :
- « Tout le réseau passe par l'interface `internal/transport.Transport` ; seul
  `internal/transport/p2p` importe libp2p » devient : « Tout le réseau passe par
  `internal/transport.Transport` ; seuls `internal/transport/p2p` et le module
  `protocol/libp2p` importent libp2p. »
- Ajouter : « Le module `protocol` (cœur pur, sans E/S) n'importe rien d'inkwell ; inkwell
  importe `protocol` et n'a aucune copie privée de ses codecs. »
- « Aucun code Go n'importe Automerge » reste vrai (le cœur Go valide les enveloppes CBOR, pas
  le contenu Automerge).

### Conséquences

- Bien : disparaissent `internal/transport/mux` (fragmentation, portes, réassemblage,
  débordement, éviction, `Reserver`), le budget commun et la file FIFO commune de `p2p.Node`,
  le budget d'une trame du vrac, les deux voies et la réinitialisation sur débordement du
  routeur, `frontend/src/net/scheduler.ts`, et, par rapport à la révision 1, OFFER, le tirage
  des messages, les minuteries de 120 s, OFFER_EXPIRED, PULL_FAILED, la trame REPO_RESET et son
  renvoi toutes les 10 s.
- Bien : spécification autonome et paquets réutilisables ; vecteurs produits par les vrais
  codecs ; modèle formel vérifié **avant** l'implémentation.
- Moins bien : changement majeur du fil `sync` ; un flux libp2p de plus par message de plus de
  64 Kio (négociation multistream optimiste, coût faible) ; un passage du relais au direct
  relance la synchronisation Automerge avec ce pair (nouvelle session).
- Moins bien : avec la part fixe de 4 Mio, tout message de plus de 4 Mio passe par des pages
  d'historique (`⌈H / 4 Mio⌉` pages, deux en vol), même entre deux pairs ; une modification
  indivisible plus grande est tirée par le vrac. Rien n'est jamais repris pour faire de la
  place.
- Résiduel de volume (Majeur jusqu'au quota N10) : la convergence byzantine ne protège pas du
  volume de modifications valides (spec §17).

### Confirmation (portes avant la bascule)

| Porte | Preuve |
|---|---|
| P-TLA | `mise run tla` : `RepoGen` exhaustif sur `RepoGenSmall.cfg` et `ConnCredit` exhaustif (crédit rendu seulement au RESET_STREAM, fermeture à 10 s), sans erreur, vivacité comprise ; leurs mutations échouent ; `RepoGen.cfg` sans violation sur la nuit ; aucune affirmation de la spec au-delà de ce que dit §7.3 |
| P-QUIC | test au démarrage : maximum de connexion ≥ maximum de flux + fenêtre initiale de flux + 1 Mio pour les versions épinglées de go-libp2p et quic-go ; bout en bout : REPO non lu + LARGE en cours + changement de génération, le contrôle passe ; un pair qui ignore STOP_SENDING voit sa session fermée en 10 s |
| P-VEC | tous les vecteurs dont `enforce` ≤ étape courante passent en Go **et** en TypeScript ; à la bascule, **tous** |
| P-BORNES | un test par ligne du §16 (borne, borne + 1, reprise) |
| P-BYZ | scénarios : pair qui ne lit jamais ; membre qui « goutte » un LARGE ; un écrivain qui remplit 65 536 clés d'attestation (les honnêtes restent couverts) ; inondation de SET ; 3 générations par seconde refusées ; relais à 8 Kio/s qui finit |
| P-ADR31 | prérequis d'activation de l'ADR 0031 : G1–G12 et Worker, `ChangeSig`, pages d'historique, ensemble admis — leurs tests de Confirmation passent |
| P-FUZZ | une cible par décodeur nouveau (préambule, trames, SET, AUTHOR, profil CBOR, schémas), corpus = vecteurs, 1 h chacune sans plantage |
| P-SEC | revue `security-auditor` (Opus) du module `protocol` et de la session |
| P-PERF | benchstat `internal/transport/session` et `protocol/wire` : aucune régression |
| P-ARCH | `scripts/arch-check.sh` : règles de dépendance du §1.6 |
| P-COMPAT | matrice de compatibilité (§1.4, `compat.json`, `repo.json#compat-*`) ; test de rétrogradation (VERSIONS) ; GREASE émis et ignoré dans les tests de bout en bout ; décodage croisé des vecteurs de la version N+1 par le paquet `sync2` de la version N du module, dès la première mineure |

## Stratégie de test (utile, pas gonflée)

Un test prouve une règle de la spec ; s'il n'en prouve aucune, il n'a pas lieu d'être.

1. **Vecteurs** : un exécuteur Go (`protocol/conformance`) et un exécuteur Vitest
   (`@inkwell/protocol`) lisent les mêmes fichiers ; `enforce` étage leur exigence.
2. **Propriétés** (rapid, fast-check) : P1 décodage strict et `encode(decode(b)) = b` pour les
   mises en page binaires ; P2 bornes ; P3 non-blocage ; P4 crédits du pont par voie ; P5 DRR ;
   P6 LARGE : exactement `size` octets, réservés avant lecture ; P7 pli identique dans tout
   ordre (liste, décisions, annonces, ensemble admis) ; P8 parts réservées : un principal sous
   sa part est toujours admis.
3. **Tests à modèle** (rapid `StateMachine`) : pair §11.1, génération §7.3 **contre les mêmes
   règles que le modèle TLA+**, objet LARGE §11.2, crédit A.1, curseur SET.
4. **Fuzz** : cibles existantes plus celles de P-FUZZ.
5. **Modèle formel** : un seul, `docs/protocol/tla/RepoGen.tla`, déjà écrit et vérifié ; il a
   trouvé deux défauts de rédaction (règles B et A ligne 3).
6. **Bout en bout** (peu) : ZZ-1/ZZ-2 ; un pair lent n'arrête pas les autres ; un relais lent
   finit ; 10 000 documents synchronisés sans changement de génération.

Tests redondants à supprimer avec leur code : `internal/transport/mux/*_test.go` ;
`frame/budget_test.go`, `budget_extra_test.go` ; `bridge/outbox_bytes_test.go` et les cas
« deux gros documents » et « débordement » de `router_test.go` ; `bridge/codec_limits_test.go` ;
`frontend/src/net/scheduler.test.ts` ; les cas de budget de `p2p/bulk_test.go` ; les tests
d'encodage qui répètent un vecteur.

## Migration (étapes fusionnables une à une)

Chaque étape garde la CI verte : les vecteurs `proposed` d'une étape future s'exécutent en
mode « signalé », sans échouer, jusqu'à leur `enforce`.

| # | Étape | Code | Clôt | Vecteurs exigés |
|---|---|---|---|---|
| 0 | Accepter cette ADR ; spec, CDDL, vecteurs, modèle versionnés | docs | — | `normative` sans `enforce` |
| 1 | **Module `protocol/`** (`go.mod`, `v0.1.0`, GoDoc, exemples, licence, README pour les tiers) : constantes du §16, codecs existants déplacés depuis `trust`, `blob`, `invite`, `policy` (l'app les réimporte), vecteurs déplacés dans `protocol/vectors/` (`go:embed`), `protocol/conformance`, générateurs en commandes ; **`@inkwell/protocol`** (espace pnpm) avec l'exécuteur Vitest ; `arch-check.sh` reçoit les règles du §1.6 | `protocol/`, `frontend/packages/protocol`, `internal/*` (imports) | — | idem |
| 2 | **N-1** : `protocol/sig` (§3.2–§3.3) appelé au filtre de connexion, `KeyFromID`, ops, attestations, décisions, ticket, rechargement et import (le travail en cours `n1` s'y range, plus la règle d'ordre premier) | `protocol`, `p2p`, `trust`, `policy`, `invite`, `identity` | **N-1** | `step-2` |
| 3 | **Modèle d'abord** : `mise run tla` (TLC épinglé) ; profil CBOR et schémas §9 dans `@inkwell/protocol`, adaptateur branché dessus (`targetId` partout, encodeur conforme, pointeur entier) | outillage, `frontend` | N-12 | `step-3` |
| 4 | `protocol/sync2` (préambules, trames, HELLO avec VERSIONS, règles de l'inconnu, règles I/B/A/R/T/W pures) ; `protocol/libp2p` (un identifiant par classe, gestionnaire de ressources par protocole et par pair, identify pour les versions) ; `internal/transport/session` derrière le drapeau `sync2` (développement seulement) | `protocol`, `internal/transport` | N-4, N-5, N-10, R-V2 | `step-4` |
| 5 | Boucles partagées non bloquantes (boîtes par pair pour `Wiring`, notifications coalescées, `Attest` asynchrone, persistance avant diffusion) ; vrac : trame ≤ 1 065 005 o, parts par pair, appariement, délais au progrès, tailles adaptatives | `app`, `bridge`, `p2p/bulk.go`, `blob` | N-2, N-3, N-6, N-7, N-15 | `step-5` |
| 6 | **Ensemble, derrière le drapeau `sync2`** (Codex C4 : on ne retire pas le `Reset` sur débordement avant que les règles L et H soient en place) : pont (époque dans Message, crédits par voie, présence sans crédit) ; rythme à la source (§7.4) ; flux LARGE avec LARGE_REPLY et parts réservées ; attestations par signataire et ≤ 256 têtes ; règles L et H, dont la fin de l'expiration silencieuse de `coverage.ts` ; ATTEST_WANT ; puis suppression des deux voies et du `Reset` sur débordement du routeur, **dans la même étape** | `bridge`, `session`, `blob`, `app`, `frontend` | **R-Reset**, **R-Evict**, N-11, N-13, N-14, S3 | `step-6` |
| 7 | (fusionnée dans 6) | — | — | — |
| 8 | SET (cinq ensembles, curseurs) ; objets de l'ADR 0031 (accusé, annonce, AUTHOR, preuve) ; pages d'historique — en suivant les étapes 2.x–3.x de l'ADR 0031 | `protocol`, `trust`, `policy`, `listing`, `app`, `frontend` | — | `step-8` |
| 9 | **Portes** P-TLA … P-ARCH vertes ; revue | — | — | tous |
| 10 | **Bascule nette** : `sync2` actif, `/inkwell/sync/1.0.0` retiré (décision 2026-11-01) ; suppression de `mux`, `frame.Budget`, file commune de `Node` ; `HOT_PATHS` : `mux` → `session` ; reformulation des invariants de `CLAUDE.md` | `transport`, `app`, `CLAUDE.md` | — | tous |
| 11 | `store` : version choisie à l'ouverture (indépendant, à tout moment) | `bridge`, `frontend/src/store` | N-8 | — |

**Travail non fusionné des étapes 2.1–2.3** (`feat/stage2-1-3`) :

| Travail | Verdict | Raison |
|---|---|---|
| 2.1 Stream `store` v2 | **garder**, fusionner | interne ; seule la détection de version change (étape 11) |
| 2.2 routeur à deux voies | **garder comme pis-aller**, supprimer à l'étape 6 | corrige R-Serve aujourd'hui |
| 2.2 débordement et éviction du `mux` (`spill.go`, …) | **abandonné, sans condition** (décision 2026-11-01) | garde R-Evict ; `mux` disparaît à l'étape 10 |
| 2.2 `scheduler.ts` | **supprimer** | inerte ; remplacé par A.1 et §10.4 |
| 2.3 `internal/policy`, op v2 | **garder**, fusionner inerte ; codecs déplacés dans `protocol` à l'étape 1 | conforme à §4.2 et §4.4 |
| 2.3 `bridge/policy.go`, kinds 10–11 | **garder inerte, changer** | validation sortie du décodeur générique (N-9) |

## Décisions de l'utilisatrice (2026-11-01)

1. Bascule nette vers `sync/2.0.0`, sans version qui parle les deux.
2. Débordement et éviction du `mux` abandonnés sans condition.
3. N-1 corrigé tout de suite (travail en cours), avec décodage du point.
4. N13 : un paquet du protocole réutilisable par d'autres.
5. N14 : un protocole versionné, compatible et évolutif.
6. N15 : réutiliser les standards et bibliothèques existants, couche maison seulement si rien
   ne convient.

## Décisions demandées à l'utilisatrice

1. Accepter cette ADR, révision 6 (option B).
2. Règle d'ordre premier pour les clés (`[L]A = O`) : plus stricte que « pas d'ordre petit »,
   sans effet sur une clé honnête ; elle change le verdict de deux cas de Chalkias.
3. Session = connexion : le passage du relais au direct relance la synchronisation avec ce
   pair (deux sessions au plus pendant le passage, époques distinctes).
4. Accord D1 des relecteurs : pages ≤ part, et tranches Bao reprenables pour une modification
   indivisible plus grande, attente sur disque par auteur sous le quota N10 (d'ici là, une
   modification ≤ 16 Mio par auteur et par receveur).
5. Règle H : ce qu'un receveur refuse ou laisse expirer est redemandé par pages
   d'historique, au plus une fois par minute et par document, au lieu de réinitialiser la
   génération.
6. Le pointeur de présentation passe en dix-millièmes entiers (le protocole n'a plus aucun
   flottant).
7. Module `protocol` comme sous-module de ce dépôt (recommandé) plutôt qu'un dépôt séparé ;
   licence du module : celle du dépôt (MPL-2.0, copyleft par fichier, compatible avec une
   intégration dans une application propriétaire) est recommandée ; Apache-2.0 ou MIT
   faciliteraient encore l'adoption, au prix de la réciprocité.

## Plus d'informations

- Spécification : [docs/protocol/](../protocol/README.md) ; changements :
  [CHANGES.md](../protocol/CHANGES.md) ; revues : [reviews/](../protocol/reviews/).
- [RFC 9000 §3.5, §4.1](https://www.rfc-editor.org/rfc/rfc9000#section-4.1) ;
  [RFC 9113 §5.2.1](https://www.rfc-editor.org/rfc/rfc9113#section-5.2.1) ;
  [RFC 8949 §4.2.1, §5.3.1](https://www.rfc-editor.org/rfc/rfc8949#section-5.3.1) ;
  [RFC 8032 §5.1.3](https://www.rfc-editor.org/rfc/rfc8032#section-5.1.3) ;
  [RFC 9413](https://www.rfc-editor.org/rfc/rfc9413) ; [RFC 8701](https://www.rfc-editor.org/rfc/rfc8701).
- [DRR] M. Shreedhar, G. Varghese, IEEE/ACM ToN 4(3), 1996.
- K. Chalkias et al., *Taming the many EdDSAs*, 2020, <https://eprint.iacr.org/2020/1244> ;
  vecteurs <https://github.com/novifinancial/ed25519-speccheck>.
- [Newcombe 2015] C. Newcombe et al., *How Amazon Web Services Uses Formal Methods*, CACM 58(4).
- À réexaminer si automerge-repo offre une contre-pression sur `NetworkAdapter.send`.

[RFC 9000 §3.5]: https://www.rfc-editor.org/rfc/rfc9000#section-3.5
[RFC 9000 §4.1]: https://www.rfc-editor.org/rfc/rfc9000#section-4.1
[DRR]: https://doi.org/10.1109/90.502236
[Newcombe 2015]: https://doi.org/10.1145/2699417
