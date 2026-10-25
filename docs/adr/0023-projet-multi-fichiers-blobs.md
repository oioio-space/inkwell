---
status: proposed
date: 2026-10-25
decision-makers: [Mathieu]
---

# Projet multi-fichiers : index Automerge à nœuds stables, blobs BLAKE3 en flux vérifié (Bao)

## Contexte et problème

Seul `main.typ` se synchronise. Un projet réel compte :
- des chapitres ;
- des images et des PDF ;
- des CSV et des `.bib` ;
- des polices ;
- des paquets `@preview`.

L'ADR 0006 a posé les blobs adressés par leur hash. Il reste à décider :
- l'arborescence et ses conflits ;
- le transfert, sa vérification, sa reprise et ses quotas ;
- la validation des chemins ;
- les droits ;
- ce que reçoit un arrivant, et dans quel ordre.

## Facteurs de décision

- **Une source de vérité par chose** : l'arborescence dans l'index, chaque texte dans son propre
  document, chaque binaire dans le magasin BLAKE3.
- Renommer ou supprimer n'interrompt pas une édition en cours.
- Tout octet venu d'un pair est vérifié avant d'être écrit ; un nom hostile est refusé partout.
- Un gros transfert ne retarde pas la frappe.
- Une reprise ne retransmet rien de déjà vérifié.

## Options envisagées

**Index**
- **I1. Table `chemin → entrée`** (le plan). Deux renommages concurrents dupliquent le fichier, et
  renommer un dossier coûte N écritures.
- **I2. Nœuds à identifiant stable**, avec `name` et `parent` : un arbre (voir Kleppmann et al. 2021
  pour les déplacements).

**Transfert**
- **T1. Canal mux 0x04** sur le flux existant : blocage en tête de file.
- **T2. Second flux libp2p** `/inkwell/bulk/1.0.0`.
- **T3. bitswap** : trop lourd (ADR 0006).

**Vérification**
- **V1. Hash du fichier entier à la fin** : un pair menteur fait tout retélécharger.
- **V2. Bao, par groupes de 16 Kio**, comme iroh-blobs.

## Décision

I2, T2 et V2.

### Index

L'index est le document racine du projet, celui du `RootURL` de la genèse :

```
{
  schema: 1,
  main: id,
  nodes: {
    [id]: {
      type: "dir" | "text" | "blob",
      name, parent: id | null,
      url?, hash?: "blake3:<64 hex>", size?,
      trashed?: {by, at} | null,
      by, at
    }
  },
  packages: { "@preview/n:v": {hash, size, by, at} }
}
```

- `id` est aléatoire, sur 128 bits.
- L'`url` d'un nœud texte ne change jamais.
- Une image remplacée garde son nœud et reçoit un nouveau `hash` (PRJ-13).
- **Migration :** le document racine actuel (le texte de `main.typ`) reçoit `nodes`. Son nœud
  `main.typ` pointe vers lui-même.

### Matérialisation

L'arborescence visible est une fonction pure de l'index, identique partout. Elle est écrite en TS,
avec des vecteurs partagés avec Go.

- **Supprimer** pose `trashed` : le fichier va à la corbeille. Le document vit toujours, les frappes
  continuent, et « Restaurer » efface `trashed` (PRJ-06).
- **Renommer ou déplacer** écrit `name` ou `parent`. L'`url` ne change pas (PRJ-05), et un dossier
  est un seul nœud (PRJ-09).
- **Renommages concurrents.** `name` est un registre Automerge : la valeur au plus grand opId gagne
  (compteur de Lamport, puis acteur). L'autre valeur reste dans les conflits et dans l'historique
  (PRJ-08).
- **Cycle** (deux déplacements croisés) : le nœud d'id minimal du cycle est rattaché à la racine.
- **Parent absent :** le nœud est rattaché à la racine, avec une marque.
- **Homonymes.** Deux nœuds d'un même parent sont homonymes si leurs noms sont égaux après
  normalisation NFC et pliage de casse (`Notes.md` et `notes.md`, ou le NFD de macOS).
  - L'id minimal garde le nom.
  - Les autres deviennent `notes (Bob).md`, avec le nom du membre, ou à défaut 6 caractères hex de
    l'id (PRJ-07).
- **Références cassées :** on analyse les littéraux de `#include`, `image(…)` et `read(…)`. L'analyse
  deviendra exacte avec tinymist en vague C (PRJ-05).
- **Plafonds :** 10 000 nœuds, profondeur 32. Au-delà, les nœuds sont ignorés par ordre d'id et
  signalés.

### Suppression sur accord (B5, FM-05…FM-19)

- `settings.deletion` vaut `"approval"` (par défaut) ou `"free"`. Il est modifiable par un admin
  seulement.
- `nodes[id].deletion = {by, at}` est une demande de suppression, qu'un écrivain peut poser.
- Un admin la tranche :
  - accepter pose `trashed` ;
  - refuser écrit `decisions[id] = refused`.
- En concurrence, le refus l'emporte : c'est la règle la plus prudente.
- Avec `"approval"`, poser `trashed` exige la couverture d'un admin (admission, comme R1 de
  l'ADR 0022). Avec `"free"`, un écrivain suffit.
- La purge (« vider la corbeille ») retire le nœud de l'index ; elle est réservée aux admins. Le
  document texte et l'historique restent ; un blob est gardé 30 jours.
- Restaurer efface `trashed`. Un homonyme est suffixé (« (restauré) »), et un parent supprimé est
  restauré avec son enfant.

### Noms portables (PRJ-10)

Un seul validateur, en TS (création et admission) et en Go (projection, `FuzzValidateName`), avec des
vecteurs dorés partagés.

- **Longueur :** UTF-8 valide de 1 à 255 octets par nom ; au plus 1024 octets pour le chemin complet.
- **Caractères interdits :** `/`, `\`, NUL, les contrôles C0, DEL et `<>:"|?*`.
- **Noms interdits :**
  - `.` et `..` ;
  - un nom qui finit par un point ou une espace ;
  - les noms de périphériques Windows, avec ou sans extension et en ignorant la casse : CON, PRN,
    AUX, NUL, CLOCK$, CONIN$, CONOUT$, COM0 à COM9, LPT0 à LPT9, COM¹²³, LPT¹²³.
- **Réservés à la racine :** `.inkwell` et `.typstdeps`.
- À l'admission, toute modification de l'index qui écrit un `name`, `hash`, `url` ou `type` invalide
  est **refusée par tous**. C'est une fonction pure de la valeur, dans la même chaîne d'admission que
  l'ADR 0022.
- Go revalide avant toute écriture, sous `os.Root`.

### Droits

- L'index et chaque texte sont des documents ordinaires. S'y appliquent :
  - T10 : un lecteur ne change rien (PRJ-12) ;
  - la suspension (ADR 0021) ;
  - les verrous, par fichier (ADR 0022).
- Un blob n'a pas d'autorité propre : il compte s'il est référencé par un index admis.
- Go ne lit pas l'index. La webview lui envoie par le Stream `store` :
  - `Tree{path, type, url|hash, size}` : l'arborescence matérialisée ;
  - les hashes référencés par l'index, la corbeille et les paquets.

### Magasin `internal/blob`

- **Emplacement :** `.inkwell/blobs/<2 hex>/<hash>`, en lecture seule, plus un arbre Bao externe
  `.obao` (groupes de 16 Kio, environ 0,4 % de la taille). Tout accès passe par `os.Root`.
- **API :** `Put` refuse au-delà du quota ; `Get` vérifie.
- **Fichiers visibles :** ce sont des copies écrites par `WriteAtomic`, jamais des liens durs : une
  édition externe corromprait le magasin.
- **Ménage :** un blob non référencé depuis 30 jours est supprimé.
- **Bibliothèque :** `lukechampine.com/blake3/bao` (MIT ; `ExtractSlice` et `VerifySlice` avec
  `group`). À valider selon l'ADR 0008.

### Flux bulk

- `p2p.Node.Bulk()` expose un second `transport.Transport` sur un flux QUIC distinct. Ainsi, pas de
  blocage en tête de file, et `p2p` reste le seul paquet qui importe libp2p. memnet en offre un second
  pour les tests.
- **Messages** (`internal/blob/wire.go`, fuzzé) : `Want{hash, offset, length ≤ 1 Mio}`,
  `Slice{hash, offset, length, bao}`, `Missing{hash}`, `Busy{retry}`.
- **Tirage seulement.** Un pair ne sert que les membres actifs non suspendus, et seulement des hashes
  référencés.
- **Réception.**
  1. Chaque tranche passe par `VerifySlice`, puis est écrite à son offset dans
     `.inkwell/blobs/partial/<hash>`.
  2. Une carte des groupes vérifiés est tenue à jour, avec un fsync périodique.
  3. À la fin : vérification BLAKE3 complète, `Rename` dans le magasin, puis projection.
- **Tranche fausse :** elle est jetée et un autre pair est essayé. Après 3 fautes : `drop`, puis
  journal de sécurité (PRJ-03). La racine Bao n'authentifie la longueur qu'avec la tranche qui
  atteint la fin du fichier : un pair peut mentir sur la longueur dans l'en-tête d'une tranche
  intérieure sans être détecté. Un désaccord avec la taille de l'index se règle donc en demandant
  le dernier groupe du fichier tel que le pair le décrit : s'il se vérifie, l'index mentait
  (`ErrSizeMismatch`, pas une faute du pair) ; sinon le pair est fautif.
- **Quotas :**
  - 100 Mo par fichier (amende les 25 Mo de l'ADR 0006) ;
  - 2 Go par projet, au-delà desquels l'arrivant confirme avant de télécharger (PRJ-11) ;
  - 4 transferts simultanés ;
  - l'espace disque est vérifié avant.

### Ce que reçoit l'arrivant (PRJ-01, PRJ-02)

Dans cet ordre :
1. la liste des membres (Welcome) ;
2. l'index ;
3. les textes, `main` d'abord, 4 en parallèle, chacun projeté une fois complet ;
4. les paquets et les polices ;
5. les autres binaires, par taille croissante.

- La progression (« 12 sur 40 fichiers, 3,2 Mo ») compte les documents complets et les blobs
  vérifiés.
- Rien n'apparaît dans le dossier avant vérification.
- Après une coupure, Automerge reprend seul ; les blobs reprennent grâce à la carte des groupes.

### Paquets et polices

- **Paquet :** c'est l'archive officielle `packages.typst.org/preview/<n>-<v>.tar.gz`, stockée telle
  quelle comme blob.
- Elle est décompressée localement dans `.typstdeps/preview/<n>/<v>/`, dossier passé à Typst par
  `--package-cache-path`.
- **Garde-fous de décompression** (`FuzzUntar`) :
  - fichiers et dossiers réguliers seulement ;
  - chemins validés ;
  - au plus 50 Mo et 5 000 entrées une fois décompressée.
- **Ajout d'un paquet :** c'est un acte explicite d'un écrivain, jamais un effet de bord d'une
  compilation (PRJ-14). Les dépendances transitives se résolvent par une compilation d'essai, dans un
  cache temporaire, au moment de l'ajout (vague C).
- **Polices :** ce sont des blobs ordinaires dans `fonts/`, passés à Typst par `--font-path`.

### Conséquences

**Bien**
- Renommer, déplacer et supprimer n'interrompent personne.
- Les conflits se résolvent de façon déterministe, sans écrasement.
- Un pair menteur est détecté à 16 Kio près.
- La frappe n'attend jamais un transfert.

**Moins bien**
- Les binaires existent en deux copies sur disque (magasin et projection). Les copies-réflexion
  (reflink) sont une optimisation possible.
- Il faut une attestation par couple (document, écrivain), toutes renvoyées à l'activation d'un
  lien. Au-delà d'environ 500 textes, il faudra les regrouper.
- `maxPending` (64 documents) est à relever à la taille de l'index.
- Le suffixe d'un homonyme peut changer quand le nœud d'id minimal disparaît.

### Confirmation

- **Cas de la matrice T11 :** PRJ-01 à PRJ-14, X-16, X-19 (index et blobs via relais).
- **Nouveaux cas :**

  | Cas | Ce qui est vérifié |
  |---|---|
  | PRJ-15 | un nom invalide est refusé par tous ; un client honnête n'en produit jamais (fast-check) |
  | PRJ-16 | un cycle est résolu de façon déterministe |
  | PRJ-17 | collisions de casse et NFC/NFD |
  | PRJ-18 | archive hostile : lien symbolique, `../`, bombe de décompression |
  | PRJ-19 | latence d'une frappe pendant un transfert de 100 Mo |

- **Fuzz :** `FuzzBlobWire`, `FuzzValidateName`, `FuzzUntar`, et le décodeur `Tree` du Stream
  `store`.

## Plus d'informations

Amende l'ADR 0006 (quota par fichier).

Corrections de la matrice T11 :
- **PRJ-07 :** « le second » se lit par ordre d'id, faute d'horloge.
- **PRJ-01 :** la compilation hors ligne dépend du sidecar (vague C) ; B4 vérifie l'arborescence et
  les hashes.
- **Modèle de l'index :** des nœuds plutôt que des chemins.

Sources :
- [bao (Go)](https://pkg.go.dev/lukechampine.com/blake3/bao)
- [spécification Bao](https://github.com/oconnor663/bao/blob/master/docs/spec.md)
- [iroh, stockage de blobs](https://iroh.computer/blog/blob-store-design-challenges)
- [opération de déplacement d'arbre](https://martin.kleppmann.com/papers/move-op.pdf)
- [Beelay](https://www.inkandswitch.com/keyhive/notebook/05/)
- [noms Windows](https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file)
