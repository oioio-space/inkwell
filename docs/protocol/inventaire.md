# Inventaire des échanges et défauts (état au 2026-11-01)

> Informatif. Photographie du code avant la mise en œuvre de [spec.md](spec.md) (révision 2). Chemins
> relatifs à la racine du dépôt ; ceux marqués † n'existent que dans le travail non fusionné
> des étapes 2.1–2.3 (`.claude/worktrees/s2-1`, branche `feat/stage2-1-3`).

## 1. Échanges entre machines

| Échange | Où | Format | Bornes aujourd'hui | Propriétaire |
|---|---|---|---|---|
| Connexion libp2p (QUIC, TCP/Noise, relais v2, DCUtR, mDNS `_inkwell._udp`) | `internal/transport/p2p/node.go:80-120`, `gate.go:15-48`, `mdns.go:16`, `relay.go` | libp2p | 64 pairs, 256 Mio de gestionnaire de ressources (`node.go:88-91`, `:419-460`) | `p2p.Node` |
| Flux `/inkwell/sync/1.0.0`, un sortant par pair | `p2p/node.go:80`, `p2p/stream.go:83-117`, `:192-252` | `u32` longueur + charge (`frame/frame.go:29-104`) | trame ≤ `MaxFrame` = 64 Mio − 67 (`transport.go:36`) ; budget global 256 Mio (`node.go:116`, `stream.go:232-239`) ; écriture 20 s sans progrès + 16 Kio/s (`frame/progress.go:18-95`) | `p2p.Node` |
| Canaux multiplexés dans sync : `0x01` dépôt, `0x02` membres, `0x03` attestations, `0xFF` fragment | `internal/app/wiring.go:22-25`, `transport/mux/mux.go:97-196` | 1 octet de canal + charge | file par canal 256 évènements / 32 Mio (`mux.go:58-67`) | `mux` (un seul lecteur pour tous les pairs) |
| Fragmentation des messages > 64 Kio | `mux/fragment.go:16-37`, `:178-213`, `:237-311` ; † `mux/spill.go` | `0xFF ch id u32 total u32 offset u32` + 32 Kio | 1 message partiel par pair, 4 Mio en RAM puis disque, 128 Mio au total disque compris, 30 s sans progrès (†) | `mux` |
| Priorité petits / gros messages | `mux/fragment.go:53-131` | — | 1 tour sur 4 aux fragments | `mux` (portes par pair) |
| Liste des membres (Have / Ops) | `internal/trust/sync.go:12-216` | §6.1 de la spec | message ≤ 80 Kio, Have ≤ 256, 2 réponses par pair et Tick (15 s, `wiring.go:29`) | `Wiring` |
| Ops de membres | `internal/trust/op.go:254-290` (encodage), `:373-420` (décodage) ; † v2 Seen `op.go:196-198`, `:381-405` | §4.1 | ≤ 4 Kio, 64 parents, 1024 ops (`list.go:19-20`) | `trust.List` |
| Attestations | `internal/trust/attest.go:53-120`, commérage `internal/app/attest.go:227-300` | §4.3 | ≤ 4 Kio, 64 têtes ; gardées sans plafond global (S3) ; 40 000 par pair non lié (`attest.go:290`) | `Wiring` |
| Décisions d'admin † | `internal/policy/op.go:95-197`, `journal.go`, `disk.go:18-120` | §4.4 | ≤ 8 Kio, 8192 ops, 1024 par auteur | `policy.Journal` (inerte) |
| Vrac `/inkwell/bulk/1.0.0` | `p2p/bulk.go:21-295`, `internal/blob/wire.go:11-200`, `blob/service.go:22-48` | §8.1 | lecture jusqu'à `MaxFrame` ; budget d'**une** trame pour tous les pairs (`bulk.go:27-30`, `:164-168`) ; 4 transferts, 8 servis, 2 par pair | `blob.Service`, `p2p.Bulk` |
| Salon `/inkwell/lobby/1.0.0` | `p2p/lobby.go:17-128`, `internal/invite/lobby.go:13-200`, `invite/flow.go` | §8.2 | 4 Kio / 256 Kio, 10 s, 3 min | `invite`, `app/invites.go` |
| Ticket, texte `inkwell1:` | `internal/invite/ticket.go:31-200` | §8.3 | 1024 caractères, 15 min | `invite` |
| Rendez-vous Wormhole | `internal/invite/wormhole.go:14-80` | magic-wormhole, `AppID inkwell.app/invite/v1` | code ≤ 64 | `invite` |
| Relais | `cmd/inkwell-relay`, `docs/relay.md` | libp2p circuit v2 | — | `p2p` |

## 2. Couche dépôt (webview)

| Échange | Où | Bornes |
|---|---|---|
| Messages automerge-repo (arrive, welcome, sync, request, doc-unavailable, ephemeral) | `frontend/src/net/admission.ts:51-101` | poignée de main 1 Kio, ids ≤ 64, `projectUrl` ≤ 80, éphémère ≤ 1 Kio |
| Budgets et attente | `admission.ts:120-235` | éphémère 128 / 64 s⁻¹ ; arrive 3 / 20 s ; attente 4 par doc, 8 Mio, ≤ 64 Mio ; refroidissement 15 s → 10 min |
| Couverture par attestations | `frontend/src/net/coverage.ts:11-23` | 256 têtes, 10 000 docs, 1024 par signataire, 4096 réécrits |
| Attestation demandée avant chaque envoi | `WailsNetworkAdapter.ts:176-180`, `:299-345` | Heads ≤ 64 |
| Présence et présentation (éphémères) | `admission.ts:23-48`, `frontend/src/present/wire.ts:1-117` | voir CDDL |
| Ordonnanceur et crédits † | `frontend/src/net/scheduler.ts` | inerte, sans appelant |

## 3. Pont interne (Streams Wails)

| Stream | Où | Format | Bornes |
|---|---|---|---|
| `net` | `cmd/inkwell/main.go:119`, `internal/bridge/router.go`, `envelope.go:29-141`, `kinds.go:19-261` ; webview `WailsNetworkAdapter.ts:215-260` | enveloppe v1, types 1–9 (+ † 10–11) | 64 Mio ; boîte d'envoi par pair : 64 messages / 2 Mio + 2 gros (†, `router.go:202-319`) ; dépassement → `Reset` du pair |
| `store` | `main.go:120`, `bridge/storestream.go:17-663`, † `storev2.go` ; webview `store/GoStorageAdapter.ts`, † `StoreV2.ts` | v1 `id op nkey clé data` ; † v2 `ISV2` + voies | 1026 voies, page 1 Mio, valeur 32 Mio |
| `preview` | `main.go:121`, `internal/app/preview.go:25-38` | JSON | requête 1 Kio, 40 pages, 48 Mio |
| `present` | `main.go:122`, `internal/app/present.go:19-60` | JSON `type` + opaque | 8 Mio, 6 fenêtres, 256 / 32 Mio |

## 4. Défauts déjà trouvés (audit et revues)

| ID | Défaut | Preuve | État | Règle de la spec qui l'empêche |
|---|---|---|---|---|
| S1, S1b | état par document et par pair créé avant toute règle, éphémères non filtrés | audit `2026-10-31-architecture-audit.md`, `audit-2026-10-31/04`, `05` | étape 1 et 2.0 fusionnées ; reste Critique jusqu'à la garde ADR 0030 | §12 (un seul ordre), §9.4 |
| S2 | liste des membres réécrite de façon non atomique | `app/persist.go:133-137` (avant correctif) | corrigé (D6) | §14.1 |
| S3 | attestations gardées sans plafond ; avec un plafond commun, un écrivain le remplirait seul | `app/attest.go:49` | ouvert | §10.6 (parts par signataire), §16.1 |
| S4 | CBOR nul → TypeError, trame perdue | `WailsNetworkAdapter.ts:302` (avant) | corrigé | §9.2 |
| R-Serve | la boucle `Serve` du routeur attendait l'envoi d'un gros document | revue Opus du 2026-11-01 (`17-stage2-progress.md:471-480`) | corrigé par deux voies par pair † | §10.1 |
| R-Reset | boucle de réinitialisations : une rafale honnête (> 64 messages ou > 2 Mio) remplit la boîte d'envoi, `Reset` du pair, resynchronisation, nouvelle rafale | `bridge/router.go:210-241` † ; même schéma pour l'attente de la webview `admission.ts:212-231` | **ouvert** | §10.3 et Annexe A.1 (crédits par voie : la rafale attend au lieu de déborder) ; §7.3 (cadence) |
| R-Evict | perte silencieuse d'un message de la classe fiable : un message partiel abandonné (TTL, trou, éviction) n'est jamais signalé à l'émetteur, dont l'état de synchronisation croit l'avoir livré | `mux/fragment.go:248-299`, `:313-330` † | **ouvert** | §5.5 (règle L), §7.2 (flux LARGE), §7.3 |
| R-V2 | op v2 décodée avant toute négociation | `trust/op.go:196-198`, `:381` † (corrigé par un refus en dur) | contourné | §5.4 (décodeurs paramétrés par l'ensemble effectif) |
| R-Save | journal de décisions écrit sans remplacement atomique | `policy/disk.go:40` † (corrigé : `store.WriteAtomic`) | corrigé | §14.1 |

## 5. Défauts nouveaux (cette analyse)

| ID | Gravité (barème de l'audit) | Défaut | Preuve | Règle |
|---|---|---|---|---|
| N-1 | **Critique** (invariant « tout octet venu d'un pair est vérifié » ; N7) | Une clé Ed25519 d'ordre petit est acceptée partout : `crypto/ed25519.Verify` vaut vrai pour la clé neutre et `R = O, S = 0` quel que soit le message, et go-libp2p v0.51.0 l'accepte dans la poignée de main. Un invité malveillant qui rachète un ticket sous le PeerID `12D3KooW9tGaPdJo5jmCpadQ971nfiq4kLcQjeBPYTfutBTtckPH` devient un membre au nom duquel **n'importe qui** se connecte, écrit et signe des attestations. | `p2p/keys.go:33-50` (pas de contrôle), `trust/op.go:197-200` (longueur seule), `trust/attest.go:42-44` ; démonstration : `vectors/crypto.json#ed25519-small-order-key`, `#peer-id-small-order` (go 1.27, go-libp2p v0.51.0) | §3.2 |
| N-2 | Majeur | La goroutine de `Wiring` envoie elle-même sur le réseau (`trustCh.Send`, `attestCh.Send`) : un pair lent l'arrête jusqu'à 20 s par message, donc la liste, les attestations et les messages de **tous** les pairs ; `Router.Serve` l'attend aussi via `Attest` → `w.do`. | `app/wiring.go:633-639`, `app/attest.go:282-285`, `bridge/router.go:194-195` → `app/wiring.go:715-721` → `app/attest.go:116-160` | §10.1, §15 |
| N-3 | Majeur | `Wiring.notify` attend la webview (canal de 64) : une webview lente arrête le traitement des membres. | `app/wiring.go:623-631`, `:31` | §10.1, Annexe A.1 (état coalescé) |
| N-4 | Majeur | Le lecteur unique du `mux` sert tous les pairs : une file de canal pleine l'arrête, donc tous les canaux de tous les pairs (« a full queue stops the reader, and with it every other channel »). | `mux/mux.go:14-17`, `:162-195`, `:292-314` | §10.2 (files par pair, lecture suspendue par pair) |
| N-5 | Majeur | `p2p.Node` a une file de réception FIFO et un budget de 256 Mio communs à tous les pairs, pris en bloquant : un pair qui inonde fait attendre la lecture des autres. | `p2p/stream.go:230-251`, `p2p/node.go:668-690` | §10.2 |
| N-6 | Majeur | Le vrac lit des trames jusqu'à 64 Mio alors que la plus grande valide fait 1 065 005 octets, et partage un budget d'une seule trame entre tous les pairs, pris en bloquant : un membre qui envoie lentement une trame de 64 Mio arrête tous les transferts de fichiers. | `p2p/bulk.go:27-30`, `:164-168` | §16.1 (trame vrac), §10.2 |
| N-7 | Mineur | Une op locale est diffusée avant d'être persistée. | `app/wiring.go:266`, `:286` puis `afterChange` `:436-453` | §14.2 |
| N-8 | Mineur | Version du Stream `store` devinée à chaque trame par le préfixe `ISV2` : une requête v1 d'identifiant `0x49535632` est mal aiguillée. | `bridge/storestream.go:441` † | Annexe A.2 |
| N-9 | Mineur | Le décodeur générique d'enveloppe décode et ré-encode le JSON de `Decide` / `Policy` (jusqu'à 64 Mio) avant tout aiguillage. | `bridge/envelope.go:77-84` †, `bridge/policy.go:62-75` † | Annexe A.1 (validation tardive) |
| N-10 | Mineur | Aucune liaison explicite au projet en début de session : un membre d'un autre projet n'est écarté qu'au cas par cas (signature, projet de l'attestation, admission des documents). | absence de HELLO ; `app/wiring.go:409-430` | §5.3 |
| N-11 | Mineur | Le débordement disque ne borne pas le pic de mémoire : la fin d'un réassemblage alloue le message entier. | `mux/spill.go:109-116` † | §7.2 (taille réservée avant lecture) ; débordement abandonné |
| N-12 | Majeur (désaccord d'analyse entre implémentations) | L'adaptateur encode avec `encode` de `cbor-x` sans réglage : chaque `Uint8Array` est étiqueté (tag 64) et les maps ont un en-tête de 2 octets ; le décodeur `cbor-x` accepte les clés en double en gardant la dernière. Vérifié : `encode({data:Uint8Array[1]})` = `b9 0001 64 'data' d8 40 41 01` ; `decode(a3 61'a' 01 61'a' 02 61'a' 03)` = `{a:3}`. | `frontend/src/net/WailsNetworkAdapter.ts:9`, `:469` ; `admission.ts:2`, `:56` | §9.2.1 (profil de validité), §9.2.2 (émission) |
| N-13 | Mineur | Attestations gardées pour un pair pas encore actif : 40 000 par pair, sans total (64 pairs × 40 000 × 4 Kio). | `app/attest.go:290` | §5.3 (rien n'est lu avant HELLO) : supprimé |
| N-14 | Majeur (revue 02-fable) | Une modification non couverte évincée ou oubliée est une perte silencieuse : l'émetteur l'a notée envoyée (`sentHashes`) et ne la renverra pas. | `frontend/src/net/coverage.ts:99-105` | §5.5 (règle L), §9.4 (attente bornée, ATTEST_WANT, puis changement de génération) |
| N-15 | Majeur (revue 02-fable ; ADR 0028) | Les délais au débit (20 s + taille / 16 Kio/s, 30 s par WANT de 1 Mio) font échouer en boucle un pair honnête derrière un relais lent. | `frame/progress.go:18-95`, `blob/service.go` | §10.2 (aucun délai sur la lenteur), §8.1 (délai au progrès, tailles adaptatives) |

Hors protocole, déjà suivis par l'audit : F3 (plantage libp2p à la fermeture), F6 (test de
simulation intermittent), F1 (aperçu).
