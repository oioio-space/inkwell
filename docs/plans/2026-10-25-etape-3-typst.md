# Étape 3 — Typst : sidecars et compilation (C1, partie compilation de C2)

> **Pour les agents :** comme le plan T11, chaque cas de la matrice a **son** test, nommé par son
> identifiant (`t.Run("TYP-07 …")`, `func TestSupervisor/SIDE-22`). Une tâche est terminée quand
> tous ses cas passent et que `mise run ci` est vert. Un cas sans test, ou un test sans assertion
> sur le résultat visible, bloque la revue.

**Objectif :** l'application compile un projet Typst en PDF et en pages SVG, **hors ligne**, avec un
exécutable unique. Les binaires `typst` et `tinymist` sont épinglés, récupérés **à la construction**,
embarqués, extraits une fois dans le cache de l'utilisateur et revérifiés (SHA-256) avant chaque
lancement. Aucun processus n'est laissé orphelin. Aucun téléchargement à l'exécution.

**Périmètre de ce plan :** C1 (sidecar) et la compilation de C2 (PDF, SVG, diagnostics, pièces
jointes). Restent pour la suite : l'aperçu incrémental et le saut source ↔ aperçu (C2), le LSP par le
pont, les présentations (C4), `typst query` côté interface (C3) — voir « Ce qui reste ».

## Recherche (25/10/2026) et décisions

Sources : pages de versions GitHub [typst v0.15.1](https://github.com/typst/typst/releases/tag/v0.15.1)
(17/07/2026) et [tinymist v0.15.10](https://github.com/Myriad-Dreamin/tinymist/releases/tag/v0.15.10)
(16/10/2026), `--help` des deux binaires téléchargés et exécutés, index
<https://packages.typst.org/preview/index.json>, essais de compilation (voir les preuves ci-dessous).

### D1 · Deux binaires, pas un : `typst` compile, `tinymist` sert l'éditeur

**Question :** `tinymist` seul suffit-il (compilation, aperçu, LSP) ? **Non pour la compilation.**

| Besoin | `typst` 0.15.1 | `tinymist` 0.15.10 |
|---|---|---|
| Compiler PDF / SVG / PNG | oui | oui (`tinymist compile`, même moteur : Typst 0.15.1 fourchu) |
| PDF **déterministe** | oui : `--creation-timestamp` / `SOURCE_DATE_EPOCH` | **non** : 2 compilations à 1,2 s d'écart, 2 PDF différents, avec ou sans `SOURCE_DATE_EPOCH` (mesuré) |
| Diagnostics lisibles par machine | `--diagnostic-format short` : `fichier:ligne:col: error: message` | format « humain » à cadres uniquement |
| `typst query` (C3 : clés `.bib`, étiquettes) | oui | non en ligne de commande (commande LSP `tinymist.exportQuery` seulement) |
| Aperçu incrémental, complétion, survol, formatage | non | oui (`tinymist lsp` sur stdio ; `tinymist preview` ouvre un port HTTP/WebSocket **sans jeton**) |

Décision : **`typst` pour compiler et interroger** (déterminisme, diagnostics), **`tinymist` pour le LSP
et l'aperçu incrémental** (C2 suite). Le coût du second binaire est contenu par la compression (D3).
L'aperçu passera par `tinymist lsp` sur **stdio** (commandes `tinymist.exportSvg`, `didChange` avec le
texte Automerge qui n'est pas encore sur disque) : aucun port, donc la règle « serveur local »
(`127.0.0.1`, port éphémère, jeton constant, jamais journalisé) ne s'applique pas. `tinymist
preview` est écarté : port sans jeton.

### D2 · Versions, licences, tailles

| Binaire | Version | Licence | Cible | Archive amont | Binaire | Embarqué (gzip -9) |
|---|---|---|---|---|---|---|
| typst | 0.15.1 | Apache-2.0 | linux/amd64 (musl, statique) | 17,5 Mo | 55,7 Mo | 24,5 Mio |
| typst | 0.15.1 | Apache-2.0 | linux/arm64 (musl, statique) | 16,2 Mo | 48,7 Mo | 23,5 Mio |
| tinymist | 0.15.10 | Apache-2.0 | linux/amd64 (glibc ≥ 2.28) | 33,0 Mo | 77,5 Mo | 31,3 Mio |
| tinymist | 0.15.10 | Apache-2.0 | linux/arm64 (glibc ≥ 2.28) | 30,3 Mo | 64,0 Mo | 28,8 Mio |
| typst / tinymist | idem | idem | darwin/amd64, darwin/arm64 | 14–32 Mo | 45–71 Mo | — |

- Total embarqué (mesuré après `sidecar:fetch`) : **55,8 Mio** (amd64), **52,3 Mio** (arm64) au lieu de
  127 / 107 Mio non compressés. Extraction du premier lancement : ≈ 0,5 s ; vérification avant chaque
  lancement : ≈ 40 ms.
- `tinymist` embarque le même Typst 0.15.1 (`tinymist --version`) : un document se comporte pareil dans les deux.
- Windows : hors de ce plan (pas d'objet de tâche, voir « Ce qui reste »). Le manifeste est extensible.
- Les archives amont sont vérifiées contre le **SHA-256 publié par GitHub** (champ `digest` de l'API) ;
  le binaire extrait est épinglé séparément (celui de `tinymist-linux-x64` publié à part concorde).
- Les licences (`LICENSE`, `NOTICE` de typst, `LICENSE` de tinymist) sont dans le dépôt et embarquées.

### D3 · Récupération à la construction, embarquement par architecture

- `mise run sidecar:fetch` (→ `cmd/sidecar-fetch`) lit `internal/sidecar/manifest.json`, télécharge
  l'archive officielle (HTTPS, hôte épinglé, taille bornée), vérifie son SHA-256, en extrait le seul
  membre voulu avec `tar` (xz et gzip), vérifie le SHA-256 du **binaire**, le compresse en gzip
  (déterministe) dans `internal/sidecar/bin/<os>_<arch>/<nom>.gz`. Un cache de téléchargement
  (`~/.cache/inkwell/sidecar-dl`) évite de retélécharger entre worktrees. Tout est idempotent : les
  fichiers déjà bons ne déclenchent aucun accès réseau.
- Les binaires ne sont **pas** dans git (`.gitignore`) ; la CI les récupère (cache `actions/cache`
  sur le hachage du manifeste) avant tout `go build`.
- `go:embed` par étiquettes `linux && amd64`, etc. Un dossier `bin/<cible>/.gitkeep` suivi garde
  `go build` possible sans récupération ; l'absence d'un binaire embarqué est alors une erreur claire
  (`ErrNotEmbedded`, « lancez `mise run sidecar:fetch` »), et les tâches `test` et `test:ci` en
  dépendent.
- L'exécutable de release **ne contient aucun code de téléchargement** : `internal/sidecar/fetch` n'est
  importé que par `cmd/sidecar-fetch` (vérifié par `mise run arch`).

### D4 · Extraction, intégrité, supervision

- Extraction au premier lancement dans `<cache utilisateur>/inkwell/sidecar/<nom>-<version>-<sha12>/<nom>`,
  dossier `0700`, fichier `0500`, écrit dans un fichier temporaire du même dossier puis `rename`
  (atomique ; deux instances Inkwell, voir `app:duo`, ne se gênent pas).
- **Avant chaque lancement** : on hache le fichier extrait (≈ 30–50 ms pour 55–77 Mo, SHA-NI) et on
  compare au manifeste. Absent, tronqué, modifié, remplacé par un lien symbolique : on supprime et on
  réextrait depuis le binaire embarqué, lui-même vérifié pendant la décompression ; l'événement est
  journalisé (PKG-05). Un binaire embarqué corrompu est une erreur (`ErrIntegrity`), jamais lancé.
- Menace couverte : corruption, extraction partielle, remplacement du fichier du cache par un autre
  utilisateur ou un programme sans les droits du propriétaire. Non couverte : un attaquant avec le même
  UID (il peut modifier Inkwell lui-même).
- **Supervision** (`sidecar.Supervisor`) : contexte, relance avec recul exponentiel (début 250 ms, plafond
  15 s, remis à zéro après 30 s de marche stable), abandon après N échecs consécutifs (`ErrGaveUp`),
  `Command` (qui appelle `Store.Resolve`) rappelé **avant chaque lancement** (donc revérification), événements pour le journal.
- **Pas d'orphelin :** groupe de processus propre (`Setpgid`), arrêt du groupe entier sur annulation
  (SIGTERM puis SIGKILL après un délai), et `Pdeathsig = SIGKILL` sous Linux avec le fil système
  verrouillé (`runtime.LockOSThread`) : le signal est lié au *fil* qui a créé l'enfant
  ([golang/go#27505](https://github.com/golang/go/issues/27505)). macOS n'a pas `Pdeathsig` : un
  `kill -9` du parent peut y laisser un enfant (écart assumé, voir « Ce qui reste »).

### D5 · Hors ligne : trois verrous

Typst n'a **aucun** mode hors ligne : un paquet absent du cache est téléchargé depuis
`packages.typst.org`. Verrous, du plus au moins fort :
1. **Le cache est complet** : paquets fournis extraits et vérifiés, plus `<projet>/.typstdeps/`.
2. **Réseau coupé pour le processus :** environnement construit de zéro (pas d'héritage) avec
   `HTTPS_PROXY`, `HTTP_PROXY`, `ALL_PROXY` vers `127.0.0.1:9` (refusé) — Typst honore ces variables
   (essai : `failed to download package (… Connection refused)`). Même un projet hostile ne sort pas.
3. **Message clair (PRJ-14)** : l'erreur « failed to download package (…/preview/foo-1.2.3.tar.gz …) »
   devient « le paquet `@preview/foo:1.2.3` n'est ni fourni avec Inkwell ni dans `.typstdeps/` ;
   Inkwell ne télécharge jamais », avec `ErrMissingPackage` et le nom du paquet.

Durcissement possible plus tard : Landlock (réseau et système de fichiers, Linux ≥ 6.7) via un
ré-exécutable d'amorçage — non fait ici, noté dans `docs/open-questions.md`.

### D6 · Paquets `@preview` fournis : cmarker, touying, **lilaq**

Archives de `packages.typst.org` **dans git** (0,5 Mo en tout), `go:embed`, SHA-256 épinglés dans
`internal/typst/packages.json`. Dépendances transitives relevées en compilant en ligne dans un cache
vierge :

| Paquet | Licence | Rôle |
|---|---|---|
| cmarker 0.1.10 | MIT | Markdown → Typst (B8) ; exige Typst ≥ 0.15.0 |
| touying 0.7.4 | MIT | présentations (C4) ; version voulue par le plan T11 (0.8.0 existe, exige 0.15 : montée à décider avec C4) |
| lilaq 0.6.0 | MIT | graphiques |
| elembic 1.1.1 | MIT OR Apache-2.0 | dépend de lilaq |
| zero 0.6.1, tiptoe 0.4.0 | MIT | dépendances de lilaq |
| uniwarn 0.1.1 | MIT | dépendance de touying |

**lilaq plutôt que cetz-plot :** cetz et cetz-plot sont sous **LGPL-3.0-or-later**, lilaq et ses
dépendances sous MIT : redistribuables dans un exécutable MPL-2.0 (ADR 0012) sans la question de
l'édition de liens. lilaq est conçu pour les graphiques de données (axes, légendes, ticks, CSV),
cetz-plot pour des figures libres ; notre besoin est le premier. Un projet qui veut cetz le vendra
dans `.typstdeps/`. Seul le thème `simple` de touying est éprouvé ici (les autres thèmes s'ajoutent
avec C4).

Typst n'accepte qu'**un** `--package-cache-path` : on construit une **vue** (liens symboliques)
fusionnant les paquets fournis et `.typstdeps/preview/…` ; en cas de conflit sur `nom:version`, **le
paquet fourni gagne** (un pair ne peut pas substituer un code épinglé).

### D7 · Polices

- `--ignore-system-fonts` : le rendu (pagination !) est **identique chez toutes les personnes**, quelles
  que soient leurs polices installées. Indispensable à l'édition à plusieurs et au PDF déterministe.
- Typst porte déjà Libertinus Serif, New Computer Modern et DejaVu Sans Mono dans son binaire : les
  dupliquer n'apporterait rien. Il manque une famille **sans empattement** pour les diapositives et les
  notes : **Inter 4.1** (SIL OFL 1.1, 4 fichiers TTF statiques : Regular, Italic, Bold, Bold Italic,
  1,7 Mo, dans git avec sa licence, SHA-256 dans `fonts.json`), extraite dans le cache, vérifiée comme
  les paquets, et passée par `--font-path` avec `<projet>/fonts/` s'il existe.

### D8 · Interface Go

```go
eng, _ := typst.New(typst.Config{})                      // cache utilisateur, limites par défaut
res, err := eng.Compile(ctx, typst.Project{Root: dir}, "main.typ")  // options : WithFormats, WithTimestamp
// res.PDF []byte · res.Pages []typst.SVG · res.Diagnostics []typst.Diagnostic
atts, _ := typst.Attachments(res.PDF)                     // pdf.attach vérifiés dans le PDF
eng.Watch(ctx, project, "main.typ", func(*Result, error)) // recompilation à chaque modification
eng.Query(ctx, project, "main.typ", "<label>")            // typst query → JSON (C3)
```

Erreurs (`errors.Is`) : `ErrCompile` (document en erreur, `*CompileError` porte les diagnostics),
`ErrMissingPackage`, `ErrTimeout`, `ErrMemoryLimit`, `ErrOutputTooLarge`, `ErrInvalidMain`,
`ErrUnsafeProject` (lien symbolique qui sort du projet), `ErrUnsafeArchive` (paquet piégé).
La **date du document** (`datetime.today()`, métadonnées du PDF) est un paramètre : par défaut le
1er janvier 2000, jamais l'horloge, sinon deux compilations ne seraient pas identiques ; l'application
passera l'heure de la dernière modification Automerge pour que tout le monde voie la même date.
Colonnes des diagnostics : Typst les imprime à partir de 0, en caractères (mesuré : `😀` compte 1) ;
Inkwell les rend à partir de 1. PDF et SVG sont deux exports : lancés en
parallèle, ils coûtent le plus lent des deux.

Limites (hostile par défaut) : délai 60 s, mémoire virtuelle 4 Gio (`RLIMIT_AS`, Linux), sortie 256 Mio,
2 000 pages, 200 diagnostics, 1 Mio de stderr. Constats qui motivent chaque borne (mesurés) :
`#range(2000000000)` fait échouer l'allocation (`memory allocation of … bytes failed`, SIGABRT) ;
`#for i in range(3000000000)` tourne sans fin ; `#read("../x")` est refusé par Typst (« would escape
the project root »), **mais un lien symbolique dans le projet qui pointe dehors est suivi** (exit 0) :
d'où `ErrUnsafeProject`, par un parcours borné avant compilation.

## Matrice de cas

### Sidecar (`internal/sidecar`)

| Id | Cas | Attendu |
|----|-----|---------|
| SIDE-01 | manifeste embarqué | JSON strict ; pour chaque cible, `typst` et `tinymist` : URL `https` sur l'hôte épinglé, SHA-256 hexadécimaux de 64 caractères, versions = celles du plan |
| SIDE-02 | premier lancement | extrait dans le cache, mode `0500`, SHA-256 = manifeste, `--version` s'exécute et affiche la version épinglée |
| SIDE-03 | second lancement | pas de réécriture (même inode) mais le fichier est haché de nouveau |
| SIDE-04 | octet modifié dans le cache (PKG-05) | détecté, réextrait, journalisé ; binaire final conforme |
| SIDE-05 | fichier absent | réextrait |
| SIDE-06 | fichier tronqué | réextrait |
| SIDE-07 | lien symbolique à la place du binaire | remplacé par un vrai fichier ; la cible du lien n'est ni lue ni exécutée |
| SIDE-08 | archive embarquée corrompue (hachage faux) | `ErrIntegrity`, aucun fichier partiel laissé |
| SIDE-09 | rien d'embarqué | `ErrNotEmbedded` avec la commande à lancer |
| SIDE-10 | 8 `Resolve` concurrents sur un cache vide | un seul fichier, intègre, sans course (`-race`) |
| SIDE-11 | plateforme non gérée | `ErrUnsupported` nommant la cible |
| SIDE-12 | `fetch` : archive au mauvais SHA-256 | refus, rien d'écrit dans `bin/` |
| SIDE-13 | `fetch` : binaire au mauvais SHA-256 (archive correcte) | refus |
| SIDE-14 | `fetch` : second appel hors ligne | rien téléchargé (cache de téléchargement et `bin/` conformes) |
| SIDE-15 | `fetch` : membre absent, nom de membre piégé (`../x`), archive trop grosse, redirection vers un autre hôte | refus avec message |
| SIDE-16 | `Run` : sortie standard et d'erreur | capturées, bornées, code de sortie rapporté |
| SIDE-17 | `Run` : délai | tout le groupe de processus est tué, petits-enfants compris |
| SIDE-18 | `Run` : annulation du contexte | retour rapide, processus disparu |
| SIDE-19 | `Run` : limite mémoire (Linux) | l'allocation échoue dans l'enfant, pas dans Inkwell |
| SIDE-20 | superviseur : démarrage et arrêt | enfant vivant, puis disparu quand le contexte est annulé |
| SIDE-21 | superviseur : plantage | relancé, nouvel enfant |
| SIDE-22 | superviseur : recul | délais croissants, plafonnés, remis à zéro après une marche stable |
| SIDE-23 | superviseur : N échecs consécutifs | `ErrGaveUp`, plus de lancement |
| SIDE-24 | superviseur : `Command` en erreur (intégrité) | rien n'est lancé, compté comme un échec |
| SIDE-25 | superviseur : `Command` avant chaque lancement | revérification à chaque lancement |
| SIDE-26 | parent tué (`kill -9`) | l'enfant disparaît (Linux), pas d'orphelin |
| SIDE-27 | superviseur : petits-enfants à l'arrêt | tout le groupe est tué |
| SIDE-28 | superviseur : événements | `started`, `exited`, `backoff` dans l'ordre, avec la durée de recul |
| SIDE-29 | vrai `tinymist lsp` sous supervision | `initialize` répond sur stdio ; arrêt propre |

### Compilation (`internal/typst`)

| Id | Cas | Attendu |
|----|-----|---------|
| TYP-01 | projet type hors ligne : include, image, bibliographie, tableau CSV, graphique lilaq, diapositives Touying, Markdown cmarker | PDF valide ; `len(Pages)` = nombre de pages du PDF ; aucun diagnostic d'erreur |
| TYP-02 | contenu réellement produit par chaque brique | `Query` retrouve le titre issu de cmarker, les entrées de la bibliographie, les lignes du CSV ; diapositives : une page par étape (`#pause`) |
| TYP-03 | `pdf.attach` d'un fichier du projet et d'octets en ligne | `Attachments` rend nom, description, type MIME et octets identiques aux originaux |
| TYP-04 | deux compilations à 1,2 s d'écart | PDF **et** SVG identiques à l'octet ; identiques aussi depuis un autre dossier et un autre cache |
| TYP-05 | polices | `--ignore-system-fonts` toujours passé ; police inconnue (même installée sur la machine) → avertissement, pas d'erreur ; Inter fournie ; `<projet>/fonts` cherché après les polices fournies |
| TYP-06 | erreur de syntaxe dans un fichier inclus | `Diagnostic{File:"sub/x.typ", Line, Column, Severity:Error, Message}` exacts |
| TYP-07 | avertissements avec succès | `err == nil`, `Result.Diagnostics` contient l'avertissement, PDF présent |
| TYP-08 | colonne après des accents / emoji | colonne en caractères, comme Typst ; pas de panique |
| TYP-09 | paquet absent (PRJ-14) | `ErrMissingPackage`, `@preview/foo:1.2.3` nommé, message qui dit « Inkwell ne télécharge jamais » |
| TYP-10 | le réseau est réellement fermé | le proxy de test reçoit la tentative de connexion ; le projet ne sort jamais |
| TYP-11 | paquet dans `.typstdeps/` | compile hors ligne ; un paquet fourni de même `nom:version` l'emporte |
| TYP-12 | archives de paquets fournis | SHA-256 de chaque archive = `packages.json` ; fichier extrait modifié → réextrait |
| TYP-13 | archive piégée (`../`, lien symbolique, fichier géant, trop de fichiers) | refus à l'extraction |
| TYP-14 | boucle énorme, délai de 1 s | `ErrTimeout` en moins de 5 s, plus aucun processus `typst` |
| TYP-15 | annulation en cours de compilation | retour rapide avec `context.Canceled`, processus disparu |
| TYP-16 | allocation énorme | `ErrMemoryLimit` ; le processus de test n'est pas touché |
| TYP-17 | `#read("../x")`, chemin absolu, `main` hors racine | erreur Typst (« would escape »), contenu jamais dans le résultat ; `main` hors racine → `ErrInvalidMain` |
| TYP-18 | lien symbolique du projet vers l'extérieur | `ErrUnsafeProject`, rien n'est lu |
| TYP-19 | sortie trop grosse / trop de pages | `ErrOutputTooLarge` |
| TYP-20 | 5 000 avertissements | 200 diagnostics gardés, `Truncated` vrai, mémoire bornée |
| TYP-21 | `Watch` : modification | nouveau résultat ; une rafale d'écritures donne une seule compilation ; une compilation en cours est annulée par la suivante |
| TYP-22 | `Watch` : annulation | retourne, aucune goroutine ni processus ne reste |
| TYP-23 | `Attachments` sur des PDF piégés (fuzz) | jamais de panique, taille bornée |
| TYP-24 | analyseur de diagnostics (fuzz) | jamais de panique, nombre et taille bornés |
| TYP-25 | 4 compilations parallèles de projets différents (`-race`) | résultats indépendants et corrects |
| TYP-26 | entrées invalides : racine absente ou fichier, `main` vide, `..`, absolu | `ErrInvalidMain` ou erreur nommée, sans lancer de processus |
| TYP-27 | `Query` | JSON des titres et des étiquettes (prépare C3) |
| TYP-28 | gabarit « Rapport » Markdown (B8, MDL-14) | un projet neuf compile hors ligne avec cmarker fourni |

## Ce qui reste (hors de ce commit)

1. **C2 aperçu (fait sans LSP, voir PROGRESS.md) :** pages visibles par `Compile` + `WithPages`, saut par `typst eval` (`Engine.Anchors`), Stream `preview`. Reste : client LSP sur stdio vers `tinymist lsp` supervisé (`didChange` du texte Automerge,
   `tinymist.exportSvg`), pont vers l'iframe, saut source ↔ aperçu, « Page X sur Y ». `sidecar.Supervisor`
   et `tinymist` embarqué sont prêts ; `Watch` (recompilation complète par `typst`) sert de repli.
2. **C3 :** appeler `Query` pour les clés `.bib` et les étiquettes.
3. **C4 présentations :** thèmes Touying au-delà de `simple`, vue présentateur, `#speaker-note`
   (déjà compilé par TYP-02), décision 0.7.4 → 0.8.0.
4. **Polices :** emoji et écritures non latines (CJK, arabe) ; aujourd'hui Typst, Inter et `fonts/` du
   projet. Une personne qui écrit en japonais doit joindre une police dans `fonts/` (message : avertissement
   « unknown font family »).
5. **Plateformes :** Windows (objet de tâche pour l'arrêt avec le parent) ; `Pdeathsig` n'existe pas sur
   macOS (surveillance d'un tube à ajouter).
6. **Durcissement :** Landlock (FS + réseau) pour les compilations.
7. **Livraison (vague D) :** `goreleaser` doit appeler `sidecar:fetch` par architecture (runners
   natifs `ubuntu-24.04` et `ubuntu-24.04-arm`) ; SBOM des binaires embarqués ; PKG-05 de bout en bout.
