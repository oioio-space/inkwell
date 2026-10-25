---
status: accepted
date: 2026-10-24
decision-makers: [Mathieu]
---

# Automerge dans la webview, Go ne voit que des octets

## Contexte et problème

Il faut un CRDT pour l'édition à plusieurs, hors ligne, avec historique par auteur.

## Facteurs de décision

- Historique complet et attribution par auteur (panneau Activité, diff par auteur)
- Liaison éditeur prête à l'emploi (CodeMirror 6)
- Format binaire ouvert, relisible hors de la webview
- Changer de CRDT ou de framework ne doit toucher qu'une couche

## Options envisagées

- Automerge (`@automerge/automerge`, `automerge-repo`, `automerge-codemirror`)
- Yjs
- Loro

## Décision

Automerge dans la webview, avec `automerge-repo` et un `NetworkAdapter` / `StorageAdapter`
maison qui parlent au pont Go. Go transporte des octets opaques ; `automerge-go` n'est
utilisé côté Go que pour la simulation, le rejeu et la relecture hors webview.

### Conséquences

- Bien : historique, diff exact, clone/fusion (branches) et attribution « presque gratuits » ;
  modèle éprouvé par Patchwork (Ink & Switch).
- Moins bien : pas d'UndoManager natif → undo local par opérations inverses (zone
  d'ombre n° 1 du document) ; débit du pont Wails à mesurer à l'étape 1.

### Confirmation

Aucun paquet Go de production n'importe `automerge-go` (vérifiable par `go list -deps`).

## Plus d'informations

Document de conception, sections « Architecture » et « Zones d'ombre » 1, 2, 7, 8.
