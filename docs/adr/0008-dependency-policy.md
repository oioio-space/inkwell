---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Dépendances épinglées, mises à jour proposées, jamais fusionnées seules

## Contexte et problème

Plusieurs briques reposent sur peu de mainteneurs (tinymist, wormhole-william, Wails bêta).

## Options envisagées

- Dependabot (natif GitHub, simple)
- Renovate (plus configurable, épinglage par digest)

## Décision

Dependabot (le dépôt vit sur GitHub), groupé par écosystème, avec délai de refroidissement,
**jamais d'auto-merge**. `go.mod` et `package-lock.json` en versions exactes ; outils
épinglés dans `mise.toml` ; binaires des sidecars avec hachage dans le dépôt. Toute montée de
tinymist, typst ou Wails déclenche le rejeu d'historique et la comparaison PDF de référence.

### Conséquences

- Bien : configuration minimale, visible dans `.github/dependabot.yml`.
- Moins bien : pas d'épinglage par digest des actions ; passer à Renovate si besoin.

### Confirmation

`govulncheck`, `grype` (SBOM) et `gitleaks` en CI ; revue humaine de chaque PR Dependabot.
