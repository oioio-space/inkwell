# Le protocole inkwell, en clair

> Version du protocole : **2.0.0-brouillon** (2026-11-01). Texte normatif : [spec.md](spec.md).
> Décision : [ADR 0034](../adr/0034-protocole-unifie.md). État actuel et défauts :
> [inventaire.md](inventaire.md). Vecteurs de test : [vectors/](vectors/). Schémas CBOR :
> [cddl/](cddl/).

## En une minute

inkwell n'a pas de serveur. Chaque appareil a une clé Ed25519 ; son identité réseau
(le « PeerID ») en dérive. Les appareils d'un même projet se connectent directement
(ou par un relais qui ne voit que des octets chiffrés) avec libp2p, sur QUIC ou sur TCP.

Entre deux appareils il n'y a que **trois sortes de conversations** :

1. **Le salon** (`/inkwell/lobby/1.0.0`) : une question, une réponse. C'est là qu'un
   inconnu qui a reçu une invitation se présente ; personne d'autre n'y parle.
2. **La synchronisation** (`/inkwell/sync/2.0.0`) : un fil ordonné de petites trames
   (au plus 96 Kio) dans chaque sens. On y échange la liste des membres, les
   attestations (« j'écris, et voici où j'en suis »), les messages des documents et la
   présence (curseurs). La première trame dit **qui on est, sur quel projet, et ce qu'on
   sait faire** (HELLO).
3. **Le vrac** (`/inkwell/bulk/1.0.0`) : tout ce qui est gros (fichiers joints, très gros
   messages de document) est **demandé par celui qui reçoit**, par tranches d'au plus
   1 Mio, et chaque tranche est vérifiée contre son empreinte BLAKE3.

```
 appareil A                                                     appareil B
 ┌────────────┐   libp2p : QUIC (TLS 1.3) ou TCP (Noise + yamux)  ┌────────────┐
 │  webview   │                                                  │  webview   │
 │ automerge  │                                                  │ automerge  │
 │   -repo    │                                                  │   -repo    │
 └─────┬──────┘                                                  └─────┬──────┘
   pont│(interne, crédits)                                       pont  │
 ┌─────┴──────┐  sync/2 ──── HELLO, TRUST, ATTEST, REPO, … ────► ┌─────┴──────┐
 │     Go     │ ◄──── HELLO, TRUST, ATTEST, REPO, … ──── sync/2  │     Go     │
 │  (session) │  bulk/1 ──── WANT ────►      ◄──── SLICE ─ bulk/1 │  (session) │
 └────────────┘  lobby/1 (inconnus seulement)                    └────────────┘
```

## Les quatre règles qui évitent les défauts

1. **Rien de partagé n'attend un pair.** Chaque pair a ses propres files. Une boucle qui
   sert tout le monde (le routeur vers la webview, le câblage des membres) ne fait jamais
   d'envoi bloquant vers un pair : elle dépose dans la file du pair et passe au suivant.
2. **Celui qui reçoit fixe le débit.** Sur le réseau, c'est le contrôle de flux de QUIC ou
   de yamux qui freine l'émetteur quand une file est pleine (comme HTTP/2 et QUIC le font
   pour chaque flux) ; pour le gros, c'est le receveur qui demande les tranches. Sur le
   pont interne, des **crédits** explicites jouent le même rôle. Un émetteur sans crédit
   attend ; il ne gêne que lui.
3. **Perdre, c'est le dire.** Un message de document ne se perd jamais en silence : s'il
   ne peut pas arriver, les deux côtés recommencent la synchronisation de ce pair
   (trame `REPO_RESET`), ce qu'Automerge sait faire. Les messages « d'état » (liste des
   membres, attestations, présence) peuvent se perdre : ils sont répétés ou remplacés.
4. **On vérifie dans un seul ordre, à un seul endroit** : taille, puis décodage strict,
   puis signature et rôle, puis admission, puis effet. Un octet qui échoue n'atteint
   jamais l'étape suivante.

## Que contient chaque trame de synchronisation ?

| Trame | Rôle | Si elle se perd |
|---|---|---|
| HELLO | projet, capacités | la session ne démarre pas (délai 10 s) |
| TRUST | liste signée des membres : « voici ma frontière » / « voici des ops » | réparée par l'annonce suivante (15 s) |
| ATTEST | « moi, qui écris, j'atteste ces têtes de ce document » | remplacée par la suivante ; renvoyée à la connexion |
| REPO | message automerge-repo (≤ 64 Kio) | interdit : `REPO_RESET` |
| OFFER | « un message de plus de 64 Kio t'attend, empreinte H, taille N » | interdit : `REPO_RESET` |
| PRESENCE | curseur, nom affiché, pilotage de présentation | sans importance, la suivante la remplace |
| REPO_RESET | « recommençons la synchronisation, génération g » | renvoyée tant qu'elle n'est pas confirmée |
| GOAWAY | « je ferme, voici pourquoi » | la connexion se ferme de toute façon |

## Ce qui est public et ce qui ne l'est pas

- **Public** (une autre application peut le parler) : identités et signatures, libp2p et
  ses trois protocoles, les trames ci-dessus, les objets signés (ops de membres,
  attestations, décisions), les messages automerge-repo et leurs règles d'admission, le
  ticket d'invitation. Tout est dans [spec.md](spec.md), avec des vecteurs de test.
- **Interne** (entre le Go et la webview d'un même exécutable, jamais sur le réseau) : les
  Streams Wails `net`, `store`, `preview` et `present`. Ils sont décrits en annexe pour que
  nos deux implémentations restent d'accord, mais ne font pas partie du contrat public et
  peuvent changer à chaque version de l'application.

## Comment savoir qu'une implémentation est conforme ?

Elle décode chaque vecteur de [vectors/](vectors/) avec le verdict attendu (accepter ou
refuser) et, pour ceux qui le donnent, reproduit exactement les octets, l'empreinte ou la
signature. Nos implémentations Go et TypeScript lisent ces mêmes fichiers dans leurs tests.
