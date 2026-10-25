---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Chaîne frontend : Svelte 5, Vite 8, TypeScript, pnpm, Biome, Vitest, Playwright

## Contexte et problème

L'interface compte de nombreux panneaux réactifs (fichiers, activité, personnes,
commentaires) autour d'un éditeur CodeMirror impératif. Il faut une chaîne rapide, sûre
vis-à-vis de la chaîne d'approvisionnement, et lisible par les agents.

## Options envisagées

- Framework : vanilla TS · **Svelte 5** · React 19 · SolidJS
- Paquets : npm · **pnpm** · bun
- Qualité : ESLint + Prettier · **Biome**

## Décision

- **Svelte 5** (runes) : performances proches du JS natif, runtime minimal, template Wails
  officiel ; CodeMirror reste indépendant du framework.
- **Vite 8**, **TypeScript 6.0** en `strict` (TypeScript 7 natif dès que `svelte-check` le
  supporte — il exige aujourd'hui `^5 || ^6`).
- **pnpm 12** épinglé par mise : n'exécute pas les scripts d'installation par défaut,
  `minimum-release-age` de 7 jours (`.npmrc`), lockfile versionné, versions exactes.
- **Biome 2** (lint + format), **Vitest** (unitaires), **Playwright** (e2e).
- **Node 24 LTS** épinglé par mise.

### Conséquences

- Bien : une commande par usage via mise ; installation reproductible et prudente.
- Moins bien : Biome couvre partiellement les fichiers `.svelte` (script) ; `svelte-check`
  complète pour les types.

### Confirmation

`mise run fe:check` (biome + svelte-check + vitest) dans `mise run ci`.
