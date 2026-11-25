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

## Amendement du 2026-11-25 (bloc RT1) : une seule page vivante possède le store

Statut : proposé (étape 1 de RT1), accepté à la fusion de RT1. Plan :
[2026-11-25-rt1-root-kept.md](../plans/2026-11-25-rt1-root-kept.md), règles C-1 et C-2.

**Problème.** `Serve` remplace la connexion précédente sans le lui dire : la page remplacée voit
une simple fermeture, et sa requête suivante (le `refresh()` périodique de `App.svelte`) rouvre un
stream qui remplace à son tour la page récente, au milieu de son chargement. Ce va-et-vient a fait
échouer le chargement du document enregistré et remplacer la racine du projet (exécution GitHub
45542633229). Dans l'application de bureau, seule la fenêtre principale ouvre `store` (la fenêtre
publique n'ouvre que `present`, `restart.html` aucun stream), et Wails retire lui-même l'ancienne
génération d'une page rechargée (410) ; deux pages vivantes n'existent que dans la construction
`server` (un contexte de navigateur par session Wails : e2e, netsim, un second onglet).

**Décision.** La page la plus récente gagne, et la perdante le sait et ne revient pas :
- C-1 : avant de fermer une connexion remplacée, `Serve` lui pousse `StatusSuperseded` (id 0,
  statut 9, sans charge ; vecteur `superseded push`) ;
- C-2 : un `GoStorageAdapter` qui le reçoit devient terminal : ses requêtes en attente et à venir
  échouent (`StoreSupersededError`), il ne se reconnecte jamais, et la page le dit une fois
  (`store.openElsewhere`).

**Conséquence assumée.** Les modifications d'une page remplacée qui n'étaient pas encore
enregistrées sont perdues pour cette page : ses écritures en attente échouent et ne sont jamais
renvoyées. Cela n'arrive qu'en construction `server` (deux contextes de navigateur vivants sur un
même profil) ; dans l'application de bureau, Wails a déjà retiré l'ancienne page d'une fenêtre
rechargée.

**Écartées.** « La plus ancienne gagne » (refuser un nouveau stream tant qu'un autre vit) casse le
rechargement en construction `server`, où l'ancienne page peut encore être attachée. Web Locks ou
BroadcastChannel ne relient que des onglets d'un même navigateur, pas deux contextes ni deux
moteurs : Go reste le seul arbitre.

**Confirmation.** `TestRT1C1SupersededToldBeforeClose`, `TestStoreVectors`, Vitest « RT1 C-2 … »,
e2e « RT1-E2E … » de `persistence.spec.ts` (WebKit et Chromium : la page remplacée ne rouvre aucun stream `store`), mutants MR5, MR9 et MR12. Fusion après une exécution GitHub verte de `scripts/candidate.sh RT1`.

## Plus d'informations

[Plan de l'étape 2, tâche 9](../plans/2026-10-25-etape-2-reseau.md) ·
[os.Root](https://go.dev/blog/osroot) · `StorageAdapterInterface` d'automerge-repo.
