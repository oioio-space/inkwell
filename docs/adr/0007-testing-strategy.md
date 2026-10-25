---
status: accepted
date: 2026-10-24
decision-makers: [Mathieu]
---

# Tests : simulation à graine, rejeu, fuzz, e2e

## Contexte et problème

On ne teste pas Automerge (c'est leur travail, fuzzé). On teste **notre colle** : réseau,
confiance, persistance, projection.

## Décision

1. Tout le réseau passe par `Transport` ; la version test est un routeur en mémoire avec
   fautes injectées (pertes, réordonnancement, duplication, partitions) et horloge virtuelle.
2. Simulation à graine : N pairs pilotant de vrais documents (`automerge-go`), oracle de
   convergence (textes et Heads identiques, aucune update refusée à tort). 200 graines en
   CI ; graine imprimée à l'échec ; `replay --seed`.
3. Scénarios nommés lisibles (hors-ligne long, révocation concurrente, 50 000 opérations,
   perte de l'hôte, même phrase éditée des deux côtés, frappe au même endroit).
4. Rejeu d'historique réel `.automerge` → projection `.typ` identique à chaque étape.
5. Propriétés (`pgregory.net/rapid`) et fuzz natif sur tout décodeur d'octets venus d'un pair.
6. E2E : deux instances Wails reliées en boucle locale, pilotées par Playwright.

### Conséquences

- Bien : une graine en échec est le meilleur rapport de bug ; journal circulaire des 1 000
  derniers messages réseau conservé en production pour rejouer un incident.
- Moins bien : e2e lents, donc peu nombreux.

### Confirmation

Gates `mise run ci` (unitaires, couverture ≥ 85 %, fuzz court) et job CI dédié simulation.
