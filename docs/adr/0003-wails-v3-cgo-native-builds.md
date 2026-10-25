---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Wails v3, cgo assumé, builds natifs par plateforme

## Contexte et problème

L'application cible Linux, macOS et Windows dès le départ, en un seul binaire Go avec une
webview. Wails v3 est encore en bêta et `pkg/application` exige cgo sous Linux et macOS ;
`automerge-go` aussi (cœur Rust).

## Options envisagées

- Wails v3 + cgo, build natif sur chaque OS
- Wails v3 + cross-compilation (zig/osxcross)
- Tauri (Rust) ; GTK natif

## Décision

Wails v3 épinglé à un tag exact, cgo assumé ; chaque plateforme est construite **sur son
propre runner** GitHub Actions (matrice `ubuntu` / `macos` / `windows`) puis signée et
notarisée. Pas de cross-compilation.

### Conséquences

- Bien : chaîne simple et reproductible ; pas de SDK tiers à maintenir.
- Moins bien : releases plus longues (trois runners) ; bêta Wails → montée de version
  traitée comme un changement à risque (rejeu d'historique + e2e).

### Confirmation

Le job `release` est une matrice native ; `CGO_ENABLED=1` explicite dans la CI.

## Plus d'informations

Sorties de secours si Wails cale : MyGo ou GTK (couche shell isolée).
