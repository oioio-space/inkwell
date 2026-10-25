---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Stockage en Go par un Stream `store` : KV atomique et vérifié dans un dossier projet

## Contexte et problème

Jusqu'ici automerge-repo écrivait dans IndexedDB : invisible de l'utilisateur, sans contrôle
d'intégrité, perdu avec le profil du navigateur, et inutilisable par Typst ou un outil externe.
L'étape 2 exige « hors ligne puis retour, sans perte » : un arrêt brutal ou un bit inversé ne
doit jamais faire perdre ni corrompre un document.

## Facteurs de décision

- Arrêt brutal entre l'écriture et le `Rename` : ancienne ou nouvelle valeur, jamais tronquée
- Un bit inversé est détecté, mis en quarantaine, remplacé par une sauvegarde
- E/S disque uniquement via `os.Root` (anti-traversée), fichiers `0o600`, dossiers `0o700`
- Pas de gonflement base64 (+33 %) ni de blocage en tête de file avec le Stream `net`

## Options envisagées

- Les bindings Wails (`[]byte` en base64 JSON)
- Partager le Stream `net`
- **Un second Stream `store`**, protocole binaire séquentiel

## Décision

Option retenue : un Stream `store` (`id u32 | op u8 | clé | data`, réponses `id u32 | statut u8 |
charge`), traité **séquentiellement**, ce qui rend la cohérence lecture-après-écriture triviale.
Go peut aussi pousser `ExternalChange{nom}` (id 0).

- `internal/store` : `WriteAtomic` (temporaire `O_EXCL`, écriture, `Sync`, `Close`, `Rename`,
  `Sync` du dossier) ; en-tête `"IKV1" | SHA-256(données)` ; clés de 1 à 4 composants
  `^[A-Za-z0-9._-]{1,128}$` ; pour les clés `incremental`, le dernier composant est le SHA-256 des
  données (convention `keyHash` d'automerge-repo) ; `ErrCorrupt` : fichier en `quarantine/`, puis
  repli sur la dernière sauvegarde saine ; sauvegardes par liens durs dans `backups/<horodatage>/`
  (RFC 3339 sans `:` pour Windows), 10 conservées.
- `internal/project` : dossier `<Documents>/Inkwell/<Titre>/` créé sans poser de question,
  `main.typ` (projection) et `.inkwell/` (`project.json`, `repo/`, `backups/`, `quarantine/`).
  **Règle de propriété** : Inkwell possède les fichiers de `project.json` et note le SHA-256 du
  dernier écrit. Un hachage différent est une modification externe : notifiée par fsnotify
  (« Importer » applique `updateText`, « Ignorer » réécrit le fichier depuis le document), et
  Inkwell **refuse d'écraser** tant qu'elle n'est pas acquittée. Les autres fichiers ne sont
  jamais touchés.
- Webview : `GoStorageAdapter` remplace `IndexedDBStorageAdapter` ; `projection.ts` envoie le
  texte 300 ms après la dernière frappe.

### Conséquences

- Bien : documents lisibles et sauvegardables, intégrité vérifiée, aucun état dans le navigateur.
- Moins bien : une charge `LoadRange` est limitée à une trame de 64 Mo ; une valeur est limitée à
  32 Mo. Un document plus gros demandera une lecture paginée.
- Moins bien : les valeurs corrompues sans sauvegarde sont perdues pour ce pair ; ses pairs les
  renvoient à la synchronisation.

### Confirmation

`TestKillDuringWrite` (sous-processus tué), `TestBitFlipQuarantined`, `TestSymlinkEscape`,
`FuzzStoreRequest`, vecteurs partagés Go/TypeScript (`testdata/storestream.json`), e2e
`persistence.spec.ts`.

## Plus d'informations

[Plan de l'étape 2, tâche 9](../plans/2026-10-25-etape-2-reseau.md) ·
[os.Root](https://go.dev/blog/osroot) · `StorageAdapterInterface` d'automerge-repo.
