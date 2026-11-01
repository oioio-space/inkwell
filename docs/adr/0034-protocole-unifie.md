---
status: proposée
date: 2026-11-01
decision-makers: [Mathieu]
---

# Un protocole unique, public et borné : flux ordonné de petites trames, tirage par le receveur, crédits sur le pont

## Contexte et problème

Les échanges d'inkwell ont grandi étape par étape : canaux multiplexés dans un flux libp2p,
fragmentation, priorités, réassemblage, débordement sur disque, éviction, boîtes d'envoi à
deux voies, files d'attente dans la webview, crédits inertes. Chaque revue trouve un nouveau
défaut du même genre ([inventaire](../protocol/inventaire.md) §4–§5) : une boucle partagée
qui attend un pair (R-Serve, N-2 à N-6), une perte silencieuse dans une classe qui ne la
tolère pas (R-Evict), un débordement transformé en réinitialisation qui se répète (R-Reset),
une version décodée avant d'être négociée (R-V2), une écriture non atomique (R-Save). Aucune
spécification ne dit, en un seul endroit, ce qui est permis sur le fil. L'utilisatrice
demande « un protocole sûr et simple sur tout ce qui existe », « clair, défini et bien fait »,
que d'autres applications puissent parler, avec des tests utiles plutôt que nombreux. Une
faille de clé (N-1, Critique) montre en outre qu'un point de base, la validité d'une clé
Ed25519, n'était spécifié nulle part.

## Facteurs de décision

- Modèle byzantin complet (N7) : un membre malveillant ne gêne jamais les autres.
- Invariants de `CLAUDE.md` et ADR 0002, 0004, 0011, 0020, 0023, 0030–0033 : CRDT dans la
  webview, Go opaque, libp2p seulement dans `p2p`, vérification de tout octet venu d'un pair.
- ADR 0028 : un gros transfert ne retarde pas la frappe ; WAN lent ; très gros fichiers.
- Simplicité : un mécanisme général plutôt que des cas particuliers ; moins de code à
  prouver ; un seul endroit pour la version.
- Interopérabilité : spécification publique, vecteurs, conformité vérifiable.
- Migration par étapes fusionnables, objets signés existants inchangés (le `ProjectID` en
  dépend).

## Options envisagées

**A. Garder la forme actuelle et corriger au cas par cas.** Canaux dans un flux, fragments de
32 Kio, portes de priorité, réassemblage avec débordement disque et éviction, plus des crédits
applicatifs sur le réseau pour ne plus bloquer le lecteur commun.

**B. Un flux libp2p par classe et un flux par gros message**, contrôle de flux laissé à
QUIC/yamux, barrière de séquence pour garder « l'attestation avant le message ».

**C. Un flux ordonné de petites trames (≤ 96 Kio) par sens et par pair, et tirage par le
receveur pour tout message de plus de 64 Kio**, sur le protocole de vrac déjà utilisé par les
blobs ; contrôle de flux du transport sur le réseau (lecture suspendue par pair), crédits
explicites sur le pont interne ; un HELLO qui fixe projet et capacités ; des générations de
synchronisation avec `REPO_RESET` explicite.

**D. Adopter un protocole tiers complet** (synchronisation automerge-repo par serveur,
iroh-docs/iroh-gossip, protocole Hypercore).

| Critère | A | B | C | D |
|---|---|---|---|---|
| Boucle partagée qui attend un pair | à corriger partout, crédits réseau à inventer | non | non | dépend |
| Perte silencieuse d'un message fiable | éviction et TTL à rendre explicites | réinit. de flux = perte à signaler | impossible : le receveur tire, l'échec déclenche REPO_RESET | dépend |
| Mémoire face à un pair qui « goutte » | éviction, disque | réservation ou éviction | rien n'arrive sans demande ; ≤ 2 demandes de 1 Mio par pair | dépend |
| Frappe pendant un gros transfert | portes (1 tour sur 4) | flux séparés | trames ≤ 96 Kio ; le gros va sur un autre flux | — |
| Mécanismes à maintenir | 9 (canaux, fragments, portes, réassemblage, débordement, éviction, TTL, budget commun, crédits) | 5 (classes, flux par message, barrière, budgets, crédits du pont) | 4 (trames + HELLO, tirage, générations, crédits du pont) | 0 côté fil, mais adaptation lourde |
| Réutilise l'existant | tout | peu | blobs (Bao, WANT/SLICE, fautes, bannissement), trames, objets signés | rien |
| Changement de fil | aucun | majeur | majeur sur `sync`, aucun sur `bulk`, `lobby`, objets signés | total |
| Respect des invariants | oui | oui | oui | **non** : serveur (ADR 0004), Go qui comprend le CRDT ou pile hors libp2p (ADR 0002, 0011), pas d'admission byzantine (ADR 0031) |

Encodage, comparé séparément : **CBOR partout** (extensible, CDDL) contre **mises en page
binaires fixes** pour les trames et objets d'inkwell et **CBOR seulement là où automerge-repo
l'impose**. Les objets signés ne peuvent pas changer (le `ProjectID` est l'empreinte de la
genèse) ; les messages non signés d'inkwell sont petits et peu nombreux ; le CBOR déterministe
demanderait une nouvelle dépendance Go et un contrôle de forme canonique au décodage. Retenu :
binaire fixe, décodage strict et `encode(decode(b)) = b` ; points d'extension explicites (TLV
du HELLO, capacités, clés CBOR ignorées dans la couche dépôt) ; CDDL pour le CBOR existant.

## Décision

Option retenue : **C**, décrite normativement dans [docs/protocol/spec.md](../protocol/spec.md)
(version 2.0.0), avec :

1. la validation des clés Ed25519 (canonique, pas d'ordre petit) à chaque entrée d'une clé
   (spec §3.2), qui corrige N-1 indépendamment du reste ;
2. le principe unique du §10.1 : toute file a un propriétaire, une borne et un crédit ; une
   boucle partagée n'attend jamais un pair ; un dépassement ne coûte qu'à son auteur ;
3. trois sémantiques de classe (§5.5) : fiable avec générations (dépôt), état réparé par
   anti-entropie (membres, attestations), à perte (présence) ;
4. un seul point de négociation (HELLO, §5.3–§5.4) ; les décodeurs reçoivent l'ensemble
   effectif ;
5. un seul ordre de vérification par saut (§12) et une table unique des bornes (§16) ;
6. les règles de persistance du §14, dont « durable avant diffusé ».

C l'emporte parce qu'elle supprime les causes des défauts au lieu de les corriger : sans
réception non demandée, il n'y a plus de réassemblage à évincer ; sans lecteur commun, plus de
blocage croisé ; avec des crédits, une rafale honnête attend au lieu de déborder ; avec des
générations, une perte se dit au lieu de se taire. Elle réutilise le tirage des blobs, déjà
vérifié et fuzzé, et laisse intacts le salon, le ticket et les objets signés.

### Conséquences

- Bien : `internal/transport/mux` disparaît (fragmentation, portes, réassemblage, débordement,
  éviction, `Reserver`), ainsi que le budget commun et la file FIFO commune de `p2p.Node`,
  le budget d'une trame du vrac, les deux voies et la réinitialisation sur débordement du
  routeur, et `frontend/src/net/scheduler.ts`. Il reste quatre mécanismes nommés, chacun avec
  sa machine à états et ses bornes.
- Bien : une spécification publique, des vecteurs produits par les vrais codecs, un critère de
  conformité.
- Moins bien : changement majeur du fil `sync` (`/inkwell/sync/2.0.0`) ; un aller-retour de
  plus pour les messages de plus de 64 Kio ; l'émetteur garde un objet offert jusqu'au tirage
  (borné : 1 par pair, 256 Mio au total, 120 s).
- Moins bien : un pair dont la file de dépôt est pleine voit aussi ses trames TRUST attendre
  (même flux) ; c'est sans danger, l'anti-entropie passant aussi par les autres pairs.

### Confirmation

- Les vecteurs de `docs/protocol/vectors/` passent en Go et en TypeScript (CI).
- Un test par ligne de la table §16 (borne, borne + 1, reprise).
- Les propriétés et le modèle ci-dessous passent ; `mise run arch` vérifie que
  `internal/transport/session` n'importe ni `trust` ni libp2p.
- Benchstat sur `internal/transport/session` (remplace `mux` dans `HOT_PATHS`).

## Stratégie de test (utile, pas gonflée)

Un test prouve une règle de la spec ; s'il n'en prouve aucune, il n'a pas lieu d'être.

1. **Vecteurs** (un par règle de fil) : un exécuteur Go par paquet (`trust`, `blob`,
   `invite`, `bridge`, `policy`, `session`) et un exécuteur Vitest lisent les mêmes fichiers.
2. **Propriétés** (rapid en Go, fast-check en TS) :
   - P1 décodage strict : `decode(b)` accepté ⇒ `encode(decode(b)) = b` ; jamais de panique —
     pour chaque décodeur du §21 ;
   - P2 bornes : pour toute suite d'évènements (pairs, trames, pannes), chaque file et carte
     reste sous sa borne §16 ;
   - P3 non-blocage : avec un pair dont l'écriture ne progresse jamais et une webview lente,
     les autres pairs livrent et reçoivent avec un retard borné ;
   - P4 crédits du pont : en vol ≤ fenêtre, ou un seul message ; un producteur et un
     consommateur honnêtes finissent toujours ;
   - P5 équité DRR : sur tout schéma d'arrivée, un pair en attente est servi au moins une fois
     par tour, à un quantum près ([DRR], lemme du déficit borné) ;
   - P6 tirage : un objet livré a exactement `size` octets et l'empreinte annoncée ; un
     serveur hostile ne fait livrer que des octets vérifiés ;
   - P7 pli des membres et des décisions identique dans tout ordre (tests existants, gardés).
3. **Tests à modèle** (rapid `StateMachine` contre un modèle de référence) : pair §11.1,
   génération §7.3, objet offert §11.2, crédit du pont A.1.
4. **Fuzz** : les 31 cibles existantes plus trame `sync/2`, HELLO, OFFER, enveloppe Credit ;
   corpus initial = vecteurs.
5. **Modèle formel** : un seul, en **TLA+** (vérifié par TLC, ~80 lignes) pour la
   **génération et REPO_RESET** (§7.3) : deux pairs, files FIFO, réinitialisations
   concurrentes, rupture de connexion. Invariants : aucune trame d'une génération dépassée
   n'est livrée ; génération identique quand les deux sont `Synced`. Vivacité : sans nouvelle
   perte, retour à `Synced`. C'est là que deux initiatives croisées rendent l'intuition peu sûre
   ([Newcombe 2015] sur l'usage de TLA+ chez AWS). Non retenus : un modèle des crédits (P4 suffit,
   machine à un seul sens) ; la causalité des révocations (déjà prouvée par P7 dans tous les
   ordres) ; P comme langage (moins répandu, outillage .NET).
6. **Scénarios de bout en bout** (peu, ciblés) : ZZ-1/ZZ-2 ; un pair lent n'arrête pas les
   autres ; un membre qui « goutte » 64 Mio ne bloque pas un transfert honnête ; la
   synchronisation de 10 000 documents ne déclenche aucune réinitialisation.

**Tests devenus redondants, à supprimer avec le code qu'ils couvrent** : tout
`internal/transport/mux/*_test.go` (≈ 55 tests : fragments, portes, débordement, éviction,
pression) ; `internal/transport/frame/budget_test.go` et `budget_extra_test.go` ;
`internal/bridge/outbox_bytes_test.go` et les cas « deux gros documents » et « débordement »
de `router_test.go` (remplacés par P3/P4) ; `internal/bridge/codec_limits_test.go` (remplacé par
les vecteurs et le test de la table §16) ; `frontend/src/net/scheduler.test.ts` ; les cas de
budget de `p2p/bulk_test.go`. Les tests d'encodage au cas par cas qui répètent un vecteur
disparaissent au profit de l'exécuteur de vecteurs.

## Migration (étapes fusionnables une à une)

| # | Étape | Code | Clôt |
|---|---|---|---|
| 0 | Accepter cette ADR ; spec, CDDL, vecteurs versionnés | docs | — |
| 1 | Exécuteurs de vecteurs Go et Vitest ; `generate.go.txt` → commande de régénération ; table §16 en données de test | tests seulement | — |
| 2 | Validation des clés (§3.2) : filtre de connexion `p2p/gate.go`, `KeyFromID`, ops, attestations, décisions, ticket. Sans dépendance : refuser `y ≥ p`, les 2 encodages non canoniques de x = 0 et les 8 encodages d'ordre petit | `p2p`, `trust`, `policy`, `invite` | **N-1** |
| 3 | Boucles partagées non bloquantes, sans changer le fil : boîtes d'envoi par pair pour `Wiring`, notifications coalescées, `Attest` asynchrone coalescé par document, persistance avant diffusion | `app`, `bridge` | N-2, N-3, N-7 |
| 4 | Vrac : trame ≤ 1 065 005 o, budget par pair au lieu d'une trame commune | `p2p/bulk.go` | N-6 |
| 5 | Crédits du pont (A.1 : kinds 12–14, génération dans PeerUp), file par pair dans l'adaptateur, attente de la webview comptée dans le crédit ; suppression des deux voies et du `Reset` sur débordement du routeur | `bridge`, `frontend/src/net` | **R-Reset** |
| 6 | `internal/transport/session` : trames `sync/2.0.0`, HELLO, capacités, files par pair et par classe, REPO_RESET, derrière le drapeau `sync2` | nouveau paquet, `p2p` (protocole) | N-4, N-5, N-10, R-V2 |
| 7 | OFFER et tirage : objets offerts dans `blob.Service` (mémoire, 1 par pair, 120 s), ordonnanceur DRR des WANT | `blob`, `session` | **R-Evict**, N-11 |
| 8 | Bascule : `sync2` actif, `sync/1.0.0` retiré (ou gardé une version selon la décision 2) ; suppression de `mux`, `frame.Budget`, file commune de `Node` ; `HOT_PATHS` : `mux` → `session` ; benchstat | `transport`, `app` | — |
| 9 | `store` : version choisie à l'ouverture ; retrait de la v1 quand le frontend ne parle plus que v2 | `bridge`, `frontend/src/store` | N-8 |
| 10 | Modèle TLA+ de §7.3 dans `docs/protocol/tla/`, tâche `mise run tla` (outil épinglé, SHA-256) | outillage | — |

**Travail non fusionné des étapes 2.1–2.3** (`feat/stage2-1-3`) :

| Travail | Verdict | Raison |
|---|---|---|
| 2.1 Stream `store` v2 (`storestream.go`, `storev2.go`, `store/kv.go`, `page.go`, `StoreV2.ts`, `protocolV2.ts`, `GoStorageAdapter.ts`, `fakeStore.ts`, tests, `testdata/storev2.json`) | **garder**, fusionner | interne, déjà par voies avec Cancel/Released ; seule la détection de version change (étape 9) |
| 2.2 routeur à deux voies (`router.go`, `outbox_bytes_test.go`) | **garder comme pis-aller**, supprimer à l'étape 5 | corrige R-Serve aujourd'hui ; remplacé par les crédits |
| 2.2 débordement et éviction du `mux` (`spill.go`, `spill_test.go`, changements de `fragment.go` et `mux.go`) | **ne pas fusionner** (sauf version publiée avant l'étape 8) | garde R-Evict ; tout `mux` disparaît à l'étape 8 |
| 2.2 `frontend/src/net/scheduler.ts` et son test | **supprimer** | inerte, sans appelant ; remplacé par A.1 et §10.4 |
| 2.3 `internal/policy`, `trust/policy.go`, op v2 (`trust/op.go`, `list.go`), `seen_test.go` | **garder**, fusionner inerte | conforme à §4.2 et §4.4 ; à l'étape 6, le refus en dur de la v2 devient « refus sauf capacité négociée » |
| 2.3 `bridge/policy.go`, kinds 10–11, `envelope.go` | **garder inerte, changer** | validation de `Decide`/`Policy` sortie du décodeur générique (N-9) |
| Exceptions de `scripts/deadcode-baseline.txt` | garder jusqu'à l'activation | — |

Le plan F.3 de l'admission reste valable ; ses crédits et permis « inertes jusqu'aux reçus
2.4/2.5b » deviennent les crédits A.1, dont la consommation passe de « remis au dépôt » à
« traitement terminé » quand ces reçus existeront (résiduel S2-R1 inchangé jusque-là).

## Décisions demandées à l'utilisatrice

1. Accepter cette ADR (option C, `sync/2.0.0`, tirage par le receveur).
2. Compatibilité : bascule nette vers `sync/2.0.0` (possible tant qu'aucune version publiée
   n'a d'utilisateurs qui se synchronisent) ou une version qui parle les deux.
3. Le débordement et l'éviction du `mux` (2.2) : les abandonner (recommandé) ou les fusionner
   comme pis-aller.
4. N-1 est Critique : autoriser l'étape 2 tout de suite, avant le reste.

## Plus d'informations

- Spécification : [docs/protocol/](../protocol/README.md) ; inventaire et défauts :
  [inventaire.md](../protocol/inventaire.md).
- [RFC 9000 §4](https://www.rfc-editor.org/rfc/rfc9000#section-4) (crédit par flux et par
  connexion, limites absolues), [RFC 9113 §5.2](https://www.rfc-editor.org/rfc/rfc9113#section-5.2)
  (WINDOW_UPDATE), [RFC 9413](https://www.rfc-editor.org/rfc/rfc9413) (tolérance seulement aux
  points d'extension), [RFC 8701](https://www.rfc-editor.org/rfc/rfc8701) (GREASE).
- [DRR] M. Shreedhar, G. Varghese, IEEE/ACM ToN 4(3), 1996, <https://doi.org/10.1109/90.502236>.
- K. Chalkias et al., *Taming the many EdDSAs*, 2020, <https://eprint.iacr.org/2020/1244> ;
  `filippo.io/edwards25519` (« SetBytes accepts all non-canonical encodings »).
- [Newcombe 2015] C. Newcombe et al., *How Amazon Web Services Uses Formal Methods*, CACM 58(4).
- Tirage vérifié par BLAKE3/Bao : ADR 0023 ; iroh-blobs et `bao-tree` (groupes de 16 Kio).
- À réexaminer si automerge-repo offre un jour une contre-pression sur `NetworkAdapter.send`
  ou une pagination native de l'historique (capacité `history/1`).

[DRR]: https://doi.org/10.1109/90.502236
[Newcombe 2015]: https://doi.org/10.1145/2699417
