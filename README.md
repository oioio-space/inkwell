# inkwell

> Écrire à plusieurs, en direct, des documents Typst — **sans serveur, sans compte, même
> hors ligne** — avec la trace de qui a écrit quoi, un historique qui ne perd rien, et un
> PDF impeccable à la fin.

inkwell est un éditeur de bureau (Linux, macOS, Windows). Les personnes se trouvent sur le
réseau local ou par un code court ; chaque frappe est enregistrée avec son auteur ;
l'aperçu Typst se met à jour pendant qu'on écrit.

**État : étape 0 terminée (fondations).** Rien d'utilisable encore — voir
[PROGRESS.md](PROGRESS.md) pour la feuille de route et le premier jalon : *écrire le
document de conception de l'application à deux, dans l'application.*

## Comment c'est fait

| Couche | Briques |
|---|---|
| Interface | Wails v3, CodeMirror 6, habillage Adwaita |
| Collaboration | Automerge (`automerge-repo`, `automerge-codemirror`) dans la webview |
| Réseau | go-libp2p (QUIC, Noise, mDNS, relais), codes Magic Wormhole |
| Typst | tinymist (aperçu, LSP) et typst (export PDF) embarqués |
| Historique | jalons go-git, binaires adressés par BLAKE3 |

Détails : [docs/architecture.md](docs/architecture.md) · décisions : [docs/adr/](docs/adr/) ·
finalité : [docs/conception.pdf](docs/conception.pdf).

## Développer

Tout passe par [mise](https://mise.jdx.dev) (outils épinglés + tâches) :

```bash
mise run setup   # toolchain + hooks git
mise run test    # tests        | mise run test:watch (TDD)
mise run ci      # le gate complet, identique à la CI GitHub Actions
```

Chaque commit passe par des gates (secrets, vulnérabilités, style, revue). Le guide des
agents IA et les invariants d'architecture sont dans [CLAUDE.md](CLAUDE.md).

## Confidentialité

Aucune télémétrie. La seule connexion sortante prévue est la vérification de mise à jour,
qui n'envoie que le numéro de version (désactivable).
