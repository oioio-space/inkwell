# Architecture

> Source : [document de conception](conception.pdf) (24 oct. 2026). Ce fichier en est la
> projection technique ; les décisions et leur raison sont dans [`adr/`](adr/).

## En une phrase

Un éditeur de bureau où plusieurs personnes écrivent en direct un même projet Typst,
Markdown ou texte, **sans serveur**. Le CRDT vit dans la webview ; Go ne transporte que des
octets opaques, gère le réseau, le disque et les processus annexes.

## Les quatre couches

```mermaid
flowchart LR
  subgraph W["Webview (TypeScript)"]
    CM["CodeMirror 6<br/>+ automerge-codemirror"]
    REPO["automerge-repo<br/>NetworkAdapter · StorageAdapter"]
    CM <--> REPO
  end
  subgraph G["Go (un seul binaire)"]
    BR["bridge<br/>Stream Wails « net » · bindings"]
    SES["session<br/>trust · identity · invite"]
    TR["transport<br/>memnet · frame · p2p (libp2p)"]
    ST["store · blob · project<br/>milestone (go-git)"]
    SC["sidecar · typst<br/>tinymist · typst"]
  end
  PEER(("pairs"))
  REPO <-- octets --> BR
  BR <--> SES <--> TR <--> PEER
  BR <--> ST
  BR <--> SC
  SC -- aperçu --> PV["iframe d'aperçu"]
```

| Couche | Rôle | Ce qu'elle ne fait **pas** |
|---|---|---|
| Webview | Édition, fusion Automerge, curseurs, undo par auteur, UI Adwaita | Aucun accès disque ni réseau direct |
| Pont | Transport d'octets opaques dans les deux sens | Ne décode jamais un document |
| Session | Identité, invitations, liste de membres signée, relais des updates | Ne connaît pas le format Automerge au-delà des signatures |
| Stockage / Typst | Persistance, blobs, projection `.typ`, jalons, sidecars | Ne décide rien de la fusion |

## Paquets Go (cible)

Un paquet = un rôle. Chaque paquet est créé **à l'étape qui en a besoin**, pas avant ; la
colonne « Étape » dit quand.

| Paquet | Responsabilité | Étape |
|---|---|---|
| `cmd/inkwell` | Point d'entrée : fenêtre Wails ; plus tard sous-commandes sans fenêtre (`export`, `join`) | 1 |
| `internal/app` | Services Wails liés au frontend (`Session` : inviter, rejoindre une invitation ; démarrage du nœud réseau) ; la composition vit dans `cmd/inkwell` | 1 |
| `internal/bridge` | Routeur Stream Wails `net` ↔ `Transport` (octets opaques, aucun port local — ADR 0009) ; Stream `store` ↔ stockage et projection (ADR 0020) | 1 |
| `internal/transport` | Interface `Transport` (`Send`, `Recv`, `Peers`) ; `memnet` (simulation), `p2p` (libp2p, étape 2) | 1 |
| `internal/identity` | Clé Ed25519 par machine, empreinte prononçable (4 mots), groupes d'appareils | 2 |
| `internal/invite` | Codes Wormhole (PAKE), acceptation d'un pair LAN | 2 |
| `internal/trust` | Liste de membres signée (CRDT à ordre total Lamport → clé), rôles, révocation, vérification des lots signés | 2 |
| `internal/store` | État Automerge sur disque : KV atomique et vérifié sur `os.Root` (`WriteAtomic`, en-tête `IKV1`+SHA-256), quarantaine, sauvegardes par liens durs (ADR 0020) | 2 |
| `internal/project` | Projet = dossier `<Documents>/Inkwell/<Titre>/` : `project.json`, projection `main.typ`, règle de propriété fsnotify, bac à sable `os.Root`, corbeille | 2 |
| `internal/sidecar` | `typst` et `tinymist` épinglés (`manifest.json`), embarqués compressés, extraits et revérifiés (SHA-256) avant chaque lancement ; `Run` (délai, mémoire, sortie bornés) et `Supervisor` (relance avec recul, groupe de processus, `Pdeathsig`) | 3 |
| `internal/sidecar/fetch` | Téléchargement **à la construction** des archives épinglées ; importé par `cmd/sidecar-fetch` seul | 3 |
| `internal/typst` | Compilation hors ligne (PDF, pages SVG, diagnostics, `query`, `Watch`), paquets `@preview` fournis + `.typstdeps/`, polices fournies, pièces jointes `pdf.attach` lues dans le PDF | 3 |
| `cmd/sidecar-fetch` | `mise run sidecar:fetch` : prépare `internal/sidecar/bin/<os>_<arch>/*.gz` | 3 |
| `internal/blob` | Magasin adressé par BLAKE3, quotas, échange sur flux libp2p dédié | 4 |
| `internal/milestone` | Jalons go-git (projection lisible, jamais source de vérité) | 4 |
| `internal/transport/frame` | Trames préfixées par leur longueur, bornées (octets venus d’un pair : fuzzé) | 2 |
| `internal/transport/p2p` | Transport libp2p : QUIC + TCP/Noise, identité Ed25519, un flux sortant par pair | 2 |
| `internal/transport/memnet` | Réseau en mémoire à fautes injectées, déterministe sous `testing/synctest` (ADR 0011) | 1 |
| `frontend/` | Svelte 5 + Vite + TypeScript : CodeMirror, automerge-repo, adaptateur réseau vers le pont, simulation de convergence (Vitest), CSS Adwaita (ADR 0010) | 1 |
| `build/` | Configuration Wails v3 (`config.yml`, Taskfiles par plateforme, icônes) | 1 |

### Règles de dépendance

- `transport` n'importe rien d'interne. `trust` et `identity` n'importent pas `transport`.
- `app` est le seul paquet qui connaît tous les autres ; aucun paquet n'importe `app`.
- libp2p n’est importé que par `internal/transport/p2p`, wormhole-william que par `internal/invite` : `scripts/arch-check.sh` (tâche `arch`, dans `ci`) le vérifie.
- Aucun code Go n'importe Automerge : la convergence se teste en TypeScript avec le vrai
  `automerge-repo` ; Go teste sa colle sur `transport/memnet` (ADR 0011).
- `internal/sidecar` et `internal/typst` n'importent ni `net`, ni `net/http`, ni `crypto/tls` ; seul
  `cmd/sidecar-fetch` importe `internal/sidecar/fetch` : l'exécutable ne contient aucun code de
  téléchargement (`scripts/arch-check.sh`). `typst` importe `sidecar`, jamais l'inverse.
- Tout ce qui lit des octets venus d'un pair est fuzzé (`go test -fuzz`).

## Trois cohérences (du document)

1. **Une seule source de vérité par chose** : le texte dans Automerge, les blobs dans le
   magasin par hachage, les membres dans la liste signée, les versions dans go-git comme
   projection. Aucun panneau n'invente sa propre vérité.
2. **Un seul vocabulaire** : voir [glossaire](glossary.md).
3. **Tout ce qui se voit se désactive** : présence, découverte LAN, relais, vignettes,
   correcteur, résumés ; le réglage est à côté de la fonction.

## Sécurité des frontières

- Pont webview ↔ Go : Streams Wails, **aucun port local** (ADR 0009). Seuls serveurs
  locaux restants (plan de données tinymist) : `127.0.0.1`, port éphémère, jeton aléatoire
  comparé en temps constant, jamais journalisé.
- Sidecars : hachage vérifié avant chaque lancement, `--root` sur le dossier projet,
  `context.WithTimeout`, mémoire (`RLIMIT_AS`) et sortie bornées, relance bornée, mort avec le
  parent (`Pdeathsig` sous Linux ; macOS et Windows : voir le plan de l'étape 3), environnement
  construit de zéro avec des proxys vers un port refusé (Typst n'a pas de mode hors ligne),
  refus d'un projet dont un lien symbolique sort du dossier. Décision : [ADR 0024](adr/0024-sidecars-typst-tinymist.md).
- Réseau : Noise de bout en bout, chaque lot d'updates signé par son auteur, refusé si
  l'auteur n'est pas membre en écriture.

## Plateformes et cgo

Wails v3 exige cgo sous Linux (GTK4 / WebKitGTK 6.0) et macOS. Chaque plateforme est donc construite
**nativement** dans une matrice GitHub Actions, sans cross-compilation — voir
[ADR 0003](adr/0003-wails-v3-cgo-native-builds.md).
