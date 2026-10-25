---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Pont webview ↔ Go par les Streams Wails v3 ; GTK4 sous Linux

## Contexte et problème

Le document prévoyait des bindings Wails, ou un WebSocket sur `127.0.0.1` avec jeton si
les bindings saturent (zone d'ombre n° 8). Un WebSocket local ouvre un port joignable par
tout processus de la machine, visible des pare-feux, et impose jeton et contrôle d'origine.

## Options envisagées

- Bindings Wails (JSON, `[]byte` en base64) pour tout
- WebSocket local `coder/websocket` + jeton
- **Streams Wails v3** (`app.HandleStream` / `Stream(name)` côté JS), apparus en bêta 2026

## Décision

- **Données** (messages automerge-repo) : un Stream binaire nommé `net`, par fenêtre.
  Aucun port ouvert : le flux passe par le serveur d'assets, déjà lié à l'origine de l'app.
  Débit mesuré par Wails sous Linux : 226 Mo/s Go→JS, 727 Mo/s JS→Go ; latence p99
  1–2 ms ; contre-pression bornée (8 Mo / 256 trames par fenêtre) ; trame ≤ 64 Mo.
- **Contrôle** (requête/réponse : identité locale, héberger, rejoindre) : bindings.
- **Linux : GTK4 / WebKitGTK 6.0**, le défaut de Wails v3 ; le chemin GTK3 (`-tags gtk3`)
  disparaît en v3.1, on ne le prend pas.

### Conséquences

- Bien : la question « bindings ou WebSocket » du document est réglée sans port local, ce
  qui supprime toute une classe d'attaques locales (et le jeton qui allait avec).
- Moins bien : JS→Go est plus lent en petites trames (~6 200 trames/s mesurées sous macOS)
  → le pont regroupe les messages si la frappe rapide le justifie ; Streams sont récents.
  Distributions sans WebKitGTK 6.0 (Ubuntu 22.04, Debian 12) non supportées.

### Confirmation

Aucun `net.Listen` sur `127.0.0.1` pour le pont ; test de débit du pont à l'étape 1.

## Plus d'informations

Wails v3 `docs/guides/streams.md` et `guides/build/linux.md` (tag `v3.0.0-beta.34`).
Amende [0002](0002-automerge-in-webview.md) et [architecture](../architecture.md).
