---
status: accepted
date: 2026-10-24
decision-makers: [Mathieu]
---

# Binaires adressés par BLAKE3, paquets Typst vendus dans le projet

## Contexte et problème

Automerge compresse mal les gros binaires et garderait chaque version d'une image pour
toujours. Typst télécharge ses paquets à la compilation, ce qui casse le hors-ligne.

## Options envisagées

- Blobs dans Automerge
- Magasin par hachage + protocole libp2p maison (~200 lignes)
- `boxo/bitswap` (IPFS)

## Décision

Le document ne contient que des références `{path, hash: "blake3:…", size}`. Magasin par
projet adressé par contenu, transfert à la demande sur un flux libp2p dédié, quota par
fichier (25 Mo par défaut) et par projet ; bitswap seulement si l'essai le justifie.
Les paquets `@preview/…` sont téléchargés à l'import dans `<projet>/.typstdeps/` et passés
via `--package-cache-path` ; ce dossier se synchronise comme des blobs.

### Conséquences

- Bien : compilation jamais dépendante du réseau ; une image remplacée = un nouveau hachage.
- Moins bien : protocole d'échange et quotas à écrire.

### Confirmation

Test : un pair qui rejoint hors ligne compile le projet sans accès réseau.
