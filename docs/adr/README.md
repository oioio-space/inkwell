# Décisions d'architecture (ADR)

Format [MADR 4](https://adr.github.io/madr/) avec en-tête YAML. Une décision acceptée n'est
jamais réécrite : une nouvelle ADR la remplace (`status: superseded by NNNN`).
On écrit une ADR pour ce qui serait **coûteux, risqué ou déroutant à défaire** sans en
connaître la raison — pas pour le reste. Modèle : [`template.md`](template.md).

| # | Décision | Statut |
|---|---|---|
| [0001](0001-record-decisions-madr.md) | Consigner les décisions en MADR | acceptée |
| [0002](0002-automerge-in-webview.md) | Automerge dans la webview, Go ne voit que des octets | acceptée |
| [0003](0003-wails-v3-cgo-native-builds.md) | Wails v3, cgo assumé, builds natifs par plateforme | acceptée |
| [0004](0004-network-stack-libp2p.md) | Pile réseau maison : go-libp2p + wormhole-william derrière `Transport` | acceptée |
| [0005](0005-trust-model-v1.md) | Modèle de confiance v1 : liste de membres signée | acceptée |
| [0006](0006-blobs-by-hash-vendored-packages.md) | Binaires adressés par BLAKE3, paquets Typst vendus | acceptée |
| [0007](0007-testing-strategy.md) | Tests : simulation à graine, rejeu, fuzz, e2e | acceptée |
| [0008](0008-dependency-policy.md) | Dépendances épinglées, mises à jour proposées jamais fusionnées seules | acceptée |
