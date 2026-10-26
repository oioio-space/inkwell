# Décisions d'architecture (ADR)

Format [MADR 4](https://adr.github.io/madr/) avec en-tête YAML. Une décision acceptée n'est
jamais réécrite : une nouvelle ADR la remplace (`status: superseded by NNNN`).
On écrit une ADR pour ce qui serait **coûteux, risqué ou déroutant à défaire** sans en
connaître la raison — pas pour le reste. Modèle : [`template.md`](template.md).

| # | Décision | Statut |
|---|---|---|
| [0001](0001-record-decisions-madr.md) | Consigner les décisions en MADR | acceptée |
| [0002](0002-automerge-in-webview.md) | Automerge dans la webview, Go ne voit que des octets | acceptée, amendée par 0009, 0011 |
| [0003](0003-wails-v3-cgo-native-builds.md) | Wails v3, cgo assumé, builds natifs par plateforme | acceptée |
| [0004](0004-network-stack-libp2p.md) | Pile réseau maison : go-libp2p + wormhole-william derrière `Transport` | acceptée |
| [0005](0005-trust-model-v1.md) | Modèle de confiance v1 : liste de membres signée | acceptée, amendée par 0017 |
| [0006](0006-blobs-by-hash-vendored-packages.md) | Binaires adressés par BLAKE3, paquets Typst vendus | acceptée |
| [0007](0007-testing-strategy.md) | Tests : simulation à graine, rejeu, fuzz, e2e | acceptée, amendée par 0011 |
| [0008](0008-dependency-policy.md) | Dépendances épinglées, mises à jour proposées jamais fusionnées seules | acceptée |
| [0009](0009-bridge-wails-streams-gtk4.md) | Pont webview ↔ Go par Streams Wails ; GTK4 sous Linux | acceptée |
| [0010](0010-frontend-toolchain.md) | Chaîne frontend : Svelte 5, Vite, TS, pnpm, Biome, Vitest | acceptée |
| [0011](0011-simulation-without-automerge-go.md) | Simulation sans automerge-go | acceptée |
| [0012](0012-license-mpl-2.md) | Licence MPL-2.0 | acceptée |
| [0013](0013-model-routing-token-economy.md) | Routage des modèles : économie de tokens sans perte de qualité | acceptée |
| [0017](0017-member-list-seniority.md) | Liste de membres : DAG signé, ancienneté stricte (amende 0005) | acceptée |
| [0018](0018-signed-heads-write-control.md) | Contrôle d'écriture par têtes signées | acceptée |
| [0020](0020-storage-in-go-store-stream.md) | Stockage en Go par un Stream `store` : KV atomique et vérifié, dossier projet | acceptée |
| [0021](0021-suspension.md) | Suspension : déconnecter et laisser revenir (op signées) | proposée |
| [0022](0022-zones-verrouillees.md) | Zones verrouillées : verrous dans le document, admission déterministe | proposée |
| [0023](0023-projet-multi-fichiers-blobs.md) | Projet multi-fichiers : index à nœuds stables, blobs BLAKE3/Bao | proposée |
| [0026](0026-grille-csv.md) | Tableur CSV/TSV : grille Svelte maison virtualisée, texte Automerge pour seule source | proposée |
