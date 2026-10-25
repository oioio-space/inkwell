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

## Plus d'informations

Dépendances directes validées au titre de cette politique (versions exactes dans `go.mod`) :

| Module | Version | Licence | Pourquoi | Notes de revue |
|---|---|---|---|---|
| `lukechampine.com/blake3` | v1.4.1 | MIT | Hash BLAKE3 et vérification Bao par groupes de 16 Kio (`internal/blob`, [ADR 0023](0023-projet-multi-fichiers-blobs.md)) | Un seul mainteneur ; v1.4.0 est retirée (`retract`), donc l'épinglage compte. Le sous-paquet `bao` n'est pas versionné à part. Seul `cpuid/v2` en dépendance. |
| `golang.org/x/text` | v0.43.0 | BSD-3 | Normalisation NFC et pliage de casse des noms de fichiers (`internal/project/names.go`) | Équipe Go. |
| `golang.org/x/sys` | v0.49.0 | BSD-3 | Espace disque libre avant un transfert (`internal/blob`) | Équipe Go. |
