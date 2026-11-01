# Le protocole inkwell, en clair

> Version : **2.0.0-brouillon.6** (2026-11-01). Texte normatif : [spec.md](spec.md).
> Décision : [ADR 0034](../adr/0034-protocole-unifie.md). Changements et revues :
> [CHANGES.md](CHANGES.md), [reviews/](reviews/). État actuel et défauts :
> [inventaire.md](inventaire.md). Vecteurs : [vectors/](vectors/). Schémas CBOR : [cddl/](cddl/).
> Modèle formel : [tla/](tla/). Paquets réutilisables : module Go
> `github.com/oioio-space/inkwell/protocol`, paquet TypeScript `@inkwell/protocol` (spec §1.6).

## En une minute

inkwell n'a pas de serveur. Chaque appareil a une clé Ed25519 ; son identité réseau (le
« PeerID ») en dérive. Les appareils d'un même projet se connectent directement (ou par un
relais qui ne voit que des octets chiffrés) avec libp2p, sur QUIC ou sur TCP.

Entre deux appareils, chaque sorte d'échange a **son propre flux**, et c'est le transport
(QUIC ou yamux) qui freine un flux quand celui qui reçoit n'a plus de place — exactement comme
HTTP/2 et QUIC le font pour chaque flux. Un flux lent ne gêne donc jamais les autres :

| Flux | Ce qu'il porte | Si quelque chose se perd |
|---|---|---|
| `control` | « qui je suis, quel projet, ce que je sais faire, quelles versions » (HELLO) ; la liste des membres, les décisions, les annonces de documents (SET) ; les attestations | c'est réparé : chacun redit toutes les 15 s où il en est, et on redemande une attestation qui manque |
| `repo` | les messages des documents (automerge-repo), précédés de leurs preuves (attestation, signature d'auteur) | jamais en silence : on change de **génération** (on ferme ce flux, on en ouvre un neuf) et Automerge resynchronise |
| `presence` | curseurs, noms affichés, pilotage de présentation | sans importance : le suivant remplace |
| `large` | un message de plus de 64 Kio, sur son propre flux, ouvert seulement quand le receveur a réservé la place et l'a demandé | comme `repo` |
| `bulk` | les fichiers joints, et les rares modifications indivisibles plus grandes qu'une part, par tranches vérifiées contre leur empreinte BLAKE3, reprenables | on redemande ailleurs |
| `lobby` | un inconnu invité se présente (une question, une réponse) | il réessaie |

## Les règles qui évitent les défauts

1. **Rien de partagé n'attend un pair.** Chaque pair a ses files ; une boucle qui sert tout le
   monde dépose et passe au suivant.
2. **Celui qui reçoit fixe le débit.** Sur le réseau, par le contrôle de flux du transport,
   flux par flux ; sur le pont interne, par des crédits. La lenteur n'est jamais une faute ;
   seul un arrêt complet en est une.
3. **Perdre, c'est le dire.** Un message de document perdu en route fait changer de génération
   (règle L). Un message reçu mais non gardé (invalide, preuve manquante, file pleine) est noté,
   puis redemandé par pages d'historique (règle H) : jamais de boucle de réinitialisations.
4. **Chacun sa part.** Tout plafond partagé (attestations, gros messages, service des fichiers)
   garantit une part à chaque pair ou à chaque personne qui écrit ; personne ne peut prendre la
   part d'un autre.
5. **On vérifie dans un seul ordre** : taille, forme, signature, droits, couverture, garde,
   effet. Rien de ce qui échoue n'atteint l'étape suivante.
6. **Ce qu'on ne connaît pas a une règle** : en général on l'ignore ; si c'est marqué « à
   comprendre », on refuse avec une raison. On ne devine jamais. On envoie exprès, de temps en
   temps, des valeurs inconnues (GREASE) pour que cette règle reste vraie.
7. **On ne réinvente rien qui existe** (spec §22) : libp2p, QUIC, automerge-repo, CBOR, BLAKE3,
   Bao, magic-wormhole, TLA+.

## Versions et compatibilité

- Ajouter une fonction (un champ facultatif, un nouveau type de message, une capacité) ne change
  pas la version majeure : les anciens l'ignorent.
- Un changement incompatible crée de nouveaux identifiants (`/inkwell/repo/3.0.0`) ; deux
  appareils proposent toutes leurs versions et prennent la plus haute commune ; on garde la
  précédente au moins 12 mois. Si un appareil est trop ancien, la personne le voit écrit.
- Les objets signés restent lisibles pour toujours.

## Ce qui est public et ce qui ne l'est pas

- **Public** : identités et signatures, protocoles libp2p d'inkwell, trames, objets signés, pli
  de la liste des membres, ensemble des documents admis, messages automerge-repo et leurs
  règles, garde structurelle, ticket. Tout est dans [spec.md](spec.md), avec des vecteurs.
- **Interne** : les Streams Wails entre le Go et la webview d'un même exécutable (Annexe A).

## Comment savoir qu'une implémentation est conforme ?

Elle rend le verdict attendu (`accept`, `reject` ou `ignore`) sur chaque vecteur de
[vectors/](vectors/), reproduit chaque valeur attendue, et passe l'exécuteur de conformité du
module `protocol` (ou de `@inkwell/protocol`). inkwell utilise lui-même ces paquets.
