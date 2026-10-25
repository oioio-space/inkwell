---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Simulation sans automerge-go : Go teste sa colle, TypeScript teste la convergence

## Contexte et problème

Les ADR 0002 et 0007 prévoyaient `automerge-go` côté Go pour la simulation et le rejeu.
Recherche du 25/10/2026 : dépôt sans commit depuis octobre 2024, aucun tag, lié au cœur
Automerge 2 alors que la webview utilise Automerge 3, et cgo + Rust à la compilation.

## Options envisagées

- automerge-go malgré tout
- Automerge WASM exécuté dans Go (wazero) — la colle wasm-bindgen est propre à JS
- **Séparer** : Go simule le transport sur octets opaques ; TypeScript simule la
  convergence avec le vrai `automerge-repo`

## Décision

- **Go** (`internal/transport/memnet`, `testing/synctest`) : pertes, doublons,
  réordonnancement, partitions, sur charges opaques ; oracle = propriétés de livraison et
  de routage (rien n'est livré à un mauvais pair, rien n'est inventé, tout survit aux
  reconnexions). Horloge virtuelle fournie par `testing/synctest`, donc déterministe.
- **TypeScript** (Vitest) : N `Repo` réels reliés par un réseau simulé à graine ; oracle
  de convergence (textes et heads identiques). Rejeu d'historique `.automerge` en TS.
- La projection `.typ` sur disque est envoyée par la webview (qui a le document) ; aucun
  code Go ne décode Automerge. Un mode sans fenêtre (v2) réévaluera la question.

### Conséquences

- Bien : aucun code mort de 2024, Go reste pur (cgo uniquement pour Wails), chaque
  couche est testée dans son langage avec les vraies bibliothèques.
- Moins bien : deux harnais au lieu d'un.

### Confirmation

`go list -deps ./...` ne contient pas `automerge`.

## Plus d'informations

Amende [0002](0002-automerge-in-webview.md) et [0007](0007-testing-strategy.md).
