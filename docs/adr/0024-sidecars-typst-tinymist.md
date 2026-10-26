---
status: accepted
date: 2026-10-25
decision-makers: [Mathieu]
---

# Sidecars Typst : `typst` compile, `tinymist` sert l'éditeur ; récupérés à la construction, embarqués compressés, vérifiés à chaque lancement

## Contexte et problème

L'application doit compiler du Typst hors ligne, avec un exécutable unique (`.deb`, `.rpm`,
amd64 et arm64). Il faut décider quels binaires embarquer, comment les livrer et les vérifier,
comment interdire tout téléchargement à Typst, et comment ne laisser aucun processus orphelin.

## Facteurs de décision

- PDF **déterministe** (même projet, mêmes octets) et diagnostics lisibles par machine.
- Hors ligne **garanti**, pas seulement probable : Typst n'a pas de mode hors ligne.
- Taille de l'exécutable, chaîne d'approvisionnement (aucun binaire dans git), licences.
- Règle d'architecture : pas de port local si un tube suffit (ADR 0009).

## Options envisagées

- **Un seul binaire, `tinymist`** (compilation, aperçu, LSP).
- **Deux binaires** : `typst` pour compiler et interroger, `tinymist` pour le LSP et l'aperçu.
- Binaires dans git (ADR 0008 d'origine), ou récupérés au premier lancement.

## Décision

**Deux binaires.** Mesuré sur typst 0.15.1 et tinymist 0.15.10 : `tinymist compile` n'est pas
déterministe (deux PDF différents à 1,2 s d'écart, avec ou sans `SOURCE_DATE_EPOCH`), n'a pas de
diagnostics en une ligne, ni `query` en ligne de commande. `typst compile --creation-timestamp`
donne des PDF identiques à l'octet. L'aperçu incrémental et le LSP passeront par `tinymist lsp` sur
**stdio** (pas de port ; `tinymist preview` ouvre un port HTTP/WebSocket sans jeton, écarté).

**Récupération à la construction**, jamais à l'exécution :
- `internal/sidecar/manifest.json` (dans git) épingle, par plate-forme, l'URL officielle, le SHA-256 et
  la taille de l'archive, le membre à extraire et le SHA-256 du binaire ;
- `mise run sidecar:fetch` (`cmd/sidecar-fetch`) télécharge, vérifie les deux hachages, compresse en
  gzip déterministe dans `internal/sidecar/bin/<os>_<arch>/<nom>.gz` (hors git) ;
- `go:embed` par étiquettes de plate-forme ; 55–57 Mo embarqués au lieu de 112–133 Mo ;
- le code de téléchargement n'est importé que par `cmd/sidecar-fetch` (`mise run arch`).

**Extraction et vérification :** extrait une fois dans le cache de l'utilisateur ; SHA-256 recalculé
**avant chaque lancement** ; absent, tronqué, modifié ou remplacé par un lien : réextrait et journalisé.

**Supervision :** groupe de processus propre, arrêt SIGTERM puis SIGKILL, relance avec recul
exponentiel, abandon borné, `Pdeathsig` sous Linux (fil verrouillé).

**Hors ligne par trois verrous :** cache de paquets complet (paquets fournis + `.typstdeps/`),
environnement construit de zéro avec des proxys vers un port refusé, erreur claire (PRJ-14).

**Durcissement (audit) :** Typst tourne dans un espace de noms réseau et utilisateur privé quand le
système l'autorise (même `127.0.0.1` est injoignable ; repli sur les proxys refusés si les espaces de
noms utilisateur sont interdits, mode journalisé et visible par `Engine.NetworkMode`). Les limites
(`RLIMIT_AS`, `RLIMIT_FSIZE`) sont posées avant la première instruction du programme (un `sh` fait
`ulimit` puis `exec`). Le dossier de sortie est surveillé pendant l'écriture et le groupe de processus
est tué au-delà de `MaxOutput` ; les pages SVG demandées sont bornées (`--pages`, `WithPages`). Le
nombre de compilations simultanées est borné (CPU). Parseur de diagnostics et pièces jointes bornés.
macOS : pas d'espace de noms ni de limites mémoire/fichier (un profil `sandbox-exec` serait à ajouter).

**Paquets fournis :** cmarker 0.1.10, touying 0.7.4, lilaq 0.6.0 (MIT) et leurs dépendances, dans git
(0,5 Mo), SHA-256 épinglés. **lilaq plutôt que cetz-plot** : cetz/cetz-plot sont sous LGPL-3.0, lilaq et
ses dépendances sous MIT. **Polices :** `--ignore-system-fonts` (pagination identique chez tous) ; Inter
4.1 (OFL) fournie en plus de celles de Typst.

### Conséquences

- Bien : PDF reproductibles, aucune surprise réseau, exécutable unique, binaires absents de git,
  amd64 et arm64 testés nativement en CI.
- Moins bien : deux binaires à tenir à jour ensemble (le `Typst Version` de `tinymist --version` doit
  rester celui de `typst`) ; `go build` sans récupération produit un exécutable sans sidecars
  (`ErrNotEmbedded`, test qui échoue) ; le `kill -9` du parent peut laisser un enfant sous macOS ;
  Windows n'est pas pris en charge ici.

### Confirmation

Tests `SIDE-xx` et `TYP-xx` du plan [étape 3](../plans/2026-10-25-etape-3-typst.md) : intégrité
(PKG-05), supervision et absence d'orphelin, compilation hors ligne d'un projet complet, `pdf.attach`
vérifié dans le PDF, déterminisme, projet hostile (délai, mémoire, chemins, liens).

## Plus d'informations

Amende [0008](0008-dependency-policy.md) (le hachage est dans le dépôt, le binaire non) et précise
[0006](0006-blobs-by-hash-vendored-packages.md) (vue de paquets `.typstdeps` + paquets fournis).
Sources : versions GitHub typst v0.15.1 et tinymist v0.15.10, index `packages.typst.org`, `--help` des
binaires, golang/go#27505 (`Pdeathsig` et fils).
