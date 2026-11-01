# Étude — système de plugins pour inkwell

> Statut : étude de recherche, **pas une décision** (une ADR sera nécessaire : elle touche
> l'invariant « Go n'importe pas Automerge », le pont, la CSP et le protocole).
> Date : 2026-11-01. Toutes les sources ont été consultées ce jour sauf mention contraire.
> Convention : **[vérifié]** = lu dans la source citée ; **[mémoire]** = connaissance générale
> non revérifiée dans cette session (à confirmer avant ADR) ; **[avis]** = jugement de l'auteur.

## 1. Résumé et recommandation

**Recommandation [avis]** : plugins = modules WebAssembly *cœur* (core Wasm) hébergés **côté Go
par wazero** (Go pur, sans cgo), derrière une ABI « octets entrants / octets sortants » décrite
par un schéma **WIT** (WIT sert d'IDL, pas encore de format binaire). Les plugins ne font **jamais
d'E/S** : ils appellent des *fonctions hôte* à capacités déclarées dans un manifeste. Leur
interface graphique est **déclarative** (arbre de composants Adwaita rendu par Svelte, façon
remote-dom), jamais d'iframe par défaut. Ils éditent un document en renvoyant des
**propositions d'édition** (ops ancrées) que la webview applique comme une transaction
locale signée par l'utilisateur : Go ne voit jamais Automerge, et les collègues sans le plugin
reçoivent de simples changements Automerge ordinaires. Les points d'extension suivent un
modèle à trois verbes — *contribuer* (défaut), *envelopper* (pipeline ordonné par
l'utilisateur), *remplacer* (un fournisseur actif par créneau) — et tout ce qui touche à la
confiance, aux signatures, à la synchronisation et au stockage reste **fermé**. Les
fonctionnalités du cœur sont des **plugins fournis** sur la même API (§6bis) ; seul un noyau
fermé minimal (identité, confiance, signatures, synchronisation, intégrité du CRDT) n'est pas
extensible.

Pourquoi pas le Component Model aujourd'hui : wazero ne le supporte pas et ne le prévoit pas
(§3), wasmtime-go l'expose partiellement et ne fournit des binaires précompilés que pour x86-64
(contradiction avec l'objectif deb/rpm arm64), les navigateurs n'exécutent pas les composants
sans transpilation jco (expérimentale en navigateur). WASI 0.3 est stable depuis juin 2026 mais
les chaînes d'outils invitées sont « en cours ». On garde la porte ouverte (§5.6).

## 2. Contraintes inkwell qui orientent le choix

Sources internes : `CLAUDE.md`, `docs/architecture.md`, `docs/engineering-principles.md`.

- CRDT dans la webview ; Go ne transporte que des octets opaques ; aucun import Automerge en Go.
- Menace : pairs byzantins. Tout octet de pair est hostile. Un plugin ne doit **jamais** recevoir
  d'octets de pair bruts ni pouvoir élargir son pouvoir par le contenu d'un document.
- CSP stricte (`default-src 'self'`, nonce de style, seul `'wasm-unsafe-eval'`), aucun
  contenu distant, aucun `eval` ; E/S fichier via `os.Root` ; sidecars avec mémoire, délai et
  sortie bornés ; supply chain avec versions exactes et SHA-256.
- Livrables : exécutable autonome, deb/rpm **amd64 + arm64**, build natif par OS (cgo déjà requis
  par Wails). Budgets : démarrage < 1 s ; frappe → affichage distant < 100 ms.
- Sous-commandes sans fenêtre prévues (`export`, `join`) : un plugin hébergé *dans la webview*
  ne tournerait pas dans ce mode.
- UX : « zéro jargon », Adwaita, « tout ce qui se voit se désactive », pas de fonctionnalité
  « verrue ».

## 3. État de l'art des technologies (sources)

### 3.1 WASI et Component Model

- **WASI 0.3.0 ratifié le 11 juin 2026**, « stable » ; `stream<T>`, `future<T>`, `async func`
  intégrés au Component Model ; plus de `wasi:io` en 0.3. Wasmtime 45 exécute le RC, « Wasmtime 46
  livrera 0.3.0 avec l'async activé par défaut ». Chaînes d'outils invitées : « en cours »
  pour Rust (wit-bindgen `async fn`), Go (`componentize-go`), C, JS (`componentize-js`), Python
  (`componentize-py`) ; jco supporte tout 0.3 [vérifié]
  <https://bytecodealliance.org/articles/WASI-0.3>, <https://wasi.dev/releases/wasi-p3>.
- 0.3.1 ajoute `map<K,V>` (adopté le 6 août 2026) ; wasi.dev : cible Rust `wasm32-wasip2` en
  Tier 2 stable, `wasm32-wasip3` Tier 3 nightly ; tous les outils WIT doivent viser le même
  instantané (0.3.0-rc-2026-03-15 dans la page lue) [vérifié via résumé de recherche]
  <https://wasi.dev/releases>, <https://wasi.dev/languages>.
- Langages aujourd'hui (composants 0.2) : Rust (cargo-component / wasip2), JS
  (`jco componentize`, commande « expérimentale »), Python (`componentize-py`), Go : sources
  contradictoires — une page dit que les outils composants TinyGo ne sont plus maintenus
  (utiliser Go « officiel »), une autre documente `-target=wasip2` depuis TinyGo 0.34 [vérifié]
  <https://component-model.bytecodealliance.org/language-support/go.html>,
  <https://component-model.bytecodealliance.org/language-support/building-a-simple-component/tinygo.html>,
  <https://github.com/bytecodealliance/jco>.
- **Lecture [avis]** : la couverture multi-langage *des composants* est réelle en 0.2 mais mouvante
  en 0.3 ; imposer le format composant à des auteurs de plugins en 2026 ferait de l'API publique
  d'inkwell un otage du calendrier des chaînes d'outils.

### 3.2 Hôtes d'exécution

| Hôte | Faits | Source |
|---|---|---|
| **wazero** (Go pur) | Dernière version vue : v1.12.0 (29 mai ; exceptions, références typées, constantes étendues, « vers Wasm 3.0 ») ; tail-calls expérimental en 1.10.1. **Component Model : fermé « not planned » le 2 mai 2024** (mathetake : « définitivement non » avant standardisation W3C, ~5 ans) ; en 2025 un tiers cite *gravity* (Arcjet) qui transpile des composants vers du core Wasm ; le 8 mai 2026 un commentaire cite **wacogo**, implémentation précoce du Component Model sur wazero (lien non vérifiable : 404 sur le dépôt supposé). | <https://github.com/tetratelabs/wazero/issues/2200>, <https://github.com/tetratelabs/wazero/releases> |
| **wasmtime-go** (cgo) | v50.0.x ; API composants présente (`Component`, `ComponentLinker`, `Config.SetWasmComponentModel`) mais derrière un fichier/tag `feat_component_model` (déduit des noms de fichiers), types composites « volontairement commentés » ; README : binaires Wasmtime précompilés **pour x86-64 seulement** (Linux/macOS/Windows). | <https://pkg.go.dev/github.com/bytecodealliance/wasmtime-go/v50>, <https://github.com/bytecodealliance/wasmtime-go> |
| **Extism** | Hôte Go = wazero ; hôtes listés : Browser JS, C, C++, Go, Java, .NET, Python, Rust, Zig… ; PDK (types hôte) : Rust, Go, Haskell, AssemblyScript, C, Zig, JS ; *host functions* (le plugin voit des imports Wasm) ; `Manifest.AllowedPaths` (WASI requis) ; `CallWithContext` pour le délai ; cache de compilation `wazero.NewCompilationCache` + `NewCompiledPlugin`/`Instance()` pour compiler une fois. Limites mémoire/réseau : non documentées dans le README lu. | <https://extism.org/docs/concepts/host-functions>, <https://github.com/extism/go-sdk> |
| **jco / navigateur** | `jco transpile` produit core Wasm + JS + `.d.ts` ; support navigateur « expérimental » (article jco 1.0, ancien) ; exemple : module cœur de 1,83 Mio + 40 Kio de JS (inclut le runtime WASI émulé). | <https://bytecodealliance.org/articles/jco-1.0> |

### 3.3 Performance — chiffres sourcés (et leurs limites)

- Journal CI interne de wazero (non daté, auto-mesuré) : appel `factorial` ≈ **1 559 ns/op**
  (wazero compilateur) contre ≈ **3 577 ns/op** (Wasmtime) ; appel avec allocation ≈ 14,8 µs contre
  15,6 µs. Wasmtime : compilation ≈ 11 µs, instanciation ≈ 0,70 ms (petit module) et ≈ 77,6 ms
  (gros module) [vérifié via résumé de recherche]
  <https://gist.github.com/codefromthecrypt/48877abc4b3ba5ceca12b312926b1e0a>.
- Wasmtime : l'instanciation passe de la milliseconde à la microseconde grâce au CoW mémoire et à
  la réutilisation des mappages ; l'allocateur à pool réutilise les emplacements « chauds » ; coût :
  grosse réservation virtuelle ; faille passée CVE-2022-39393 (fuite de tas entre instances
  réutilisées) [vérifié] <https://bytecodealliance.org/articles/wasmtime-10-performance>,
  <https://docs.rs/wasmtime/latest/wasmtime/struct.PoolingAllocationConfig.html>.
- Figma : sérialiser un gros document vers une iframe a coûté ≈ **14 s** avant qu'un plugin tourne
  → **ne jamais copier tout le document vers un plugin** ; passer des plages [vérifié]
  <https://www.figma.com/blog/how-we-built-the-figma-plugin-system/>.
- Aucune comparaison indépendante récente d'instanciation wazero/Wasmtime n'a été trouvée. **Règle
  du projet : aucune décision « au ressenti »** → un banc d'essai `benchstat` (compilation,
  instanciation, appel, séparés ; `-count ≥ 10`) sur 3 plugins types est un prérequis de l'ADR
  (§9).

### 3.4 Antécédents et leçons de sécurité

| Système | Fait [vérifié] | Leçon pour inkwell |
|---|---|---|
| **Zed** | Extensions Rust → `extension.wasm`, WIT (`since_v0.2.0/extension.wit`), Wasmtime, `bindgen!` côté hôte, ressources WIT (`worktree`), le manifeste porte `wasm_api_version`, l'hôte charge les liaisons correspondantes ; API publique volontairement limitée (langages, thèmes, snippets, slash-commands). Isolation/permissions : non documentées dans l'article. <https://zed.dev/blog/zed-decoded-extensions> | Versionner *par plugin*, garder les anciennes liaisons ; commencer avec peu de points d'extension. |
| **Typst** | `plugin()` : modules Wasm 32 bits, fonctions **pures**, octets en/octets sortants, isolés (pas de fichiers ni d'affichage), `plugin.transition` pour l'état ; pureté non imposée. <https://typst.app/docs/reference/foundations/plugin/> | ABI octets minimale = portable ; la pureté permet cache et parallélisme — à imposer par conception (§5.4), pas par convention. Même écosystème que nos documents. |
| **Figma** | iframe null-origin rejetée (async forcé, 14 s) ; Duktape-Wasm (lent, ES5) ; Realms (`with`+Proxy) adopté puis **remplacé par une VM JS compilée en Wasm après une faille privée** ; fuites par `({}).constructor` ; « membrane » de ~500 lignes ; interpréteur interchangeable. | Ne pas fonder la sécurité sur du masquage de globales JS ; Wasm = pas d'autorité ambiante ; garder une petite membrane auditée. |
| **VS Code** | Hôte d'extensions en processus séparé ; pas d'accès DOM *volontairement* pour pouvoir refaire l'UI sans casser les extensions ; **pas de sandbox** de sécurité. <https://code.visualstudio.com/api/extension-capabilities/overview> | Le choix « pas de DOM » est la bonne décision d'évolutivité ; le « pas de sandbox » est la mauvaise pour nous. |
| **Obsidian** | Plugins JS dans le processus, accès complet au coffre ; la communauté admet qu'un bac à sable « casserait l'API ou serait contournable » ; un plugin tiers (Safe JS) isole en Web Worker. <https://forum.obsidian.md/t/security-of-the-plugins/7544> | Modèle à éviter (aucune capacité) ; le monkey-patching d'API privées n'a pas été retrouvé dans les sources lues → **[mémoire]** seulement. |
| **Lapce** | Plugins WASI (Rust, C, AssemblyScript), registre ; runtime Wasmtime non confirmé. (sources secondaires) | Peu exploitable. |
| **Envoy proxy-wasm** | ABI hôte/invité versionnée (v0.2.1 largement implémentée), SDK multi-langage, hôtes Envoy/NGINX/ATS ; rien lu sur le Component Model. <https://github.com/proxy-wasm/spec> | ABI bas niveau stable pendant des années = adoption ; choix d'inkwell cohérent. |
| **Shopify Functions** | Wasm, « tout langage compilant vers Wasm », entrée = JSON issu d'une requête GraphQL déclarée, sortie = JSON d'opérations demandées ; Rust conseillé pour la perf ; limites d'instructions : page non lue → **[mémoire]** (budget d'instructions strict). <https://shopify.dev/docs/apps/build/functions> | **Modèle « le plugin décrit ce qu'il veut, l'hôte l'exécute »** : c'est exactement nos propositions d'édition. L'entrée déclarée (le plugin demande les données dont il a besoin) = moindre privilège. |

## 4. Grille d'évaluation des options d'exécution

Grille fixe : **S** isolation/capacités ; **L** langages aujourd'hui ; **P** performance ;
**C** adéquation Go/cgo ; **T** taille binaire ; **V** versionnage de l'API. Notes : ++ très bon,
+ bon, ~ moyen, − faible, ?? non mesuré.

| Option | S | L | P | C | T | V |
|---|---|---|---|---|---|---|
| A. Composants WIT + wasmtime-go | ++ (WASI par capacités, fuel/époque) [mémoire] | ++ en théorie (0.2) ; 0.3 en cours | ++ (JIT, CoW) | − cgo ; binaires précompilés x86-64 seul ; API composants partielle | − (runtime Wasmtime, ordre de dizaines de Mio, ?? à mesurer) | ++ (WIT, mondes) |
| B. Composants dans la webview (jco) | + (Wasm sans autorité, mais dans le processus de rendu) | + | + (JIT du navigateur) mais transpilation + 1,8 Mio/plugin | ++ aucun cgo | ~ (0 côté Go, JS généré) | + mais jco navigateur « expérimental » |
| C. wazero, core Wasm, ABI maison | ++ (Go sûr en mémoire, aucune autorité par défaut) | ++ (tout ce qui compile en core Wasm : Rust, Go/TinyGo, C, Zig, AssemblyScript, JS via QuickJS-Wasm…) | + (compilateur AOT ; 1,6 µs/appel mesuré par wazero) ; pas de *fuel* natif | ++ Go pur, arm64 ok | ++ (quelques Mio, ?? à mesurer) | ~ à construire (c'est l'objet de §5.6) |
| D. Extism (wazero + PDK) | + (hôte wazero ; politiques à vérifier) | ++ (7 PDK listés) | + | ++ | + | + ABI et PDK déjà versionnés, mais dépendance externe |
| E. Style Typst (fonctions pures, octets) | ++ | ++ | ++ | ++ | ++ | ~ |
| F. Processus externe (type sidecar) | ~ (OS seulement) | ++ | ~ | ++ | − | ~ |

**Choix [avis] : C, avec l'ergonomie de D (PDK/hôtes) et la discipline de E.** Le PDK Extism est
réutilisable *si* l'audit confirme ses garanties ; sinon une ABI maison de ~6 fonctions hôte
reste plus petite à auditer (la « membrane » de Figma).

## 5. Conception proposée

### 5.1 Où tournent les plugins

**Dans Go, un module wazero par plugin et par projet ouvert**, jamais dans la webview.
Raisons [avis] :
1. Frontière plus forte : un bug de moteur JS/JIT dans la webview, qui héberge déjà le CRDT et du
   contenu de pairs, ne devient pas une évasion de plugin.
2. Les capacités vivent là où sont les secrets et `os.Root`.
3. Même comportement sur tous les OS (versions de WebKitGTK variables selon la distribution).
4. Fonctionne en mode sans fenêtre (`export`, `join`).
5. Délai/mémoire bornés par `context` et pages mémoire, comme déjà pour les sidecars.

Coût : un aller-retour par le pont pour lire/éditer ; acceptable car les plugins sont appelés par
commandes, événements anti-rebondis ou rendus, **jamais dans la boucle de frappe synchrone**.
Les lectures se font sur la projection disque (`main.typ`, `os.Root`) ou sur une plage poussée
par la webview, jamais sur un document entier copié (leçon Figma).

### 5.2 Édition de documents sans casser l'invariant

Le plugin ne tient aucune poignée de document. Il lit des **plages** via `doc.read(range)` et
renvoie une **proposition** :

```
Proposal { id, plugin, base: <empreinte de version lue>,
           ops: [ {path, anchor: <curseur stable>, delete: n, insert: "…"} ... ],
           label: "Aligner les tableaux" }
```

- Go transporte la proposition comme **octets opaques** vers la webview (aucun import Automerge,
  invariant intact ; ADR 0011 inchangée).
- La webview la valide (bornes de taille, nombre d'ops, chemins du projet), vérifie `base`,
  traduit les ancres en positions (curseurs Automerge), l'applique comme **une seule
  transaction** annulable (« Annuler : Aligner les tableaux »), attribuée à l'utilisateur local
  avec l'étiquette du plugin. Si `base` est périmée (un collègue a écrit), les ancres stables
  fusionnent comme du texte concurrent ordinaire, ou la proposition est rejetée avec un message.
- Pour les pairs, c'est un changement Automerge signé par l'utilisateur : **aucun plugin requis
  chez les collègues**, aucun nouveau format sur le réseau.
- Le même motif que Shopify Functions : le plugin déclare, l'hôte décide d'exécuter.

### 5.3 Contenu hostile

Le texte d'un document vient potentiellement d'un pair byzantin : c'est une **entrée hostile**
pour le plugin (et le plugin l'est pour nous). Conséquences : un plugin n'obtient jamais le
pouvoir par le contenu (aucune commande, installation ou permission déclenchée par du texte
synchronisé) ; ses sorties sont revalidées comme n'importe quelle entrée ; le fuzz couvre le
décodeur des propositions (règle du projet).

### 5.4 Capacités et limites de ressources

Manifeste (`inkwell-plugin.json`), permissions **déclaratives, affichées avant installation, par
projet par défaut** :

| Capacité | Portée | Contrôle |
|---|---|---|
| `doc.read` | plages du fichier actif / projet | Go, via la projection |
| `doc.propose` | propositions §5.2 | webview valide |
| `ui.contribute` | points §6 | registre |
| `storage` | dossier `plugins/<id>/` (quota) | `os.Root` |
| `blob.read` | blobs par hachage du projet | magasin BLAKE3 |
| `net` | **absent en v1** ; plus tard : hôtes nommés, via Go, avec consentement | jamais direct |
| `process`, `fs` hors projet, `peers`, `identity`, `trust` | **jamais** | — |

Limites par appel (Go, au crochet d'instanciation) : mémoire max (pages Wasm plafonnées),
**délai** (`context` avec fermeture à l'expiration — dans wazero, option de runtime, **[mémoire]**
à confirmer), taille de sortie, nombre d'appels hôte par seconde, nombre d'ops par proposition.
wazero n'a pas de *fuel* ; le délai le remplace (Wasmtime a carburant/interruption par époque
**[mémoire]**). Un plugin qui dépasse trois fois est suspendu et l'utilisateur est prévenu
(« X est trop lent et a été mis en pause »). **Pureté par conception** : pas de WASI par défaut
(seulement les stubs `wasi_snapshot_preview1` nécessaires aux chaînes d'outils), pas d'horloge ni
d'aléa non déterministes sauf capacité nommée — pour permettre mise en cache et parallélisme
comme Typst.

### 5.5 Distribution, signature, confiance

- **Paquet** `.inkplug` : zip contenant manifeste, `plugin.wasm`, ressources ; identifié par son
  hachage SHA-256 (BLAKE3 acceptable, déjà dans le magasin).
- **Signature Ed25519** de l'éditeur (infrastructure `identity` réutilisable, empreinte en 4 mots) ;
  confiance à la première utilisation de la clé de l'éditeur, affichée en langage simple.
- **Catalogue sans serveur** : index statique signé (dépôt Git ou fichier), plus *sideload* depuis
  un fichier. Pas de téléchargement par le moteur de plugin : le gestionnaire passe par une action
  explicite (compatible avec « jamais de téléchargement à l'exécution » ; seul un module hors
  `sidecar`/`typst` le ferait, soumis à `arch-check`).
- **Mises à jour** : jamais automatiques silencieuses ; la mise à jour affiche le **diff de
  permissions** ; ancienne version conservée pour retour arrière ; révocation par liste signée avec
  âge maximal toléré. TUF/Sigstore comme référence à étudier **[mémoire, non vérifié]**.
- **Cycle de vie** : installer / activer (global ou par projet) / désactiver / mettre à jour /
  supprimer. Suppression = suppression du registre de contributions par identifiant de
  propriétaire + dossier `plugins/<id>/` ; le contenu des documents reste du texte ordinaire.
- **Activation paresseuse** (événements `onCommand`, `onView`, `onFileType:typ`…, jamais
  « au démarrage » par défaut) pour tenir le budget < 1 s ; modules compilés une fois puis
  cache disque (cache de compilation wazero).
- **Collègues sans le plugin** : un projet peut *recommander* un plugin (id + hachage) dans
  `project.json`, jamais l'installer ; l'invitation ne transporte aucun code. Le contenu produit
  doit être un contenu standard (texte Typst/Markdown, blobs). Données propres à un plugin dans
  le document : espace nommé `ext/<id>` **borné et validé comme toute donnée de pair**
  (étape « bornes de fusion »), conservé opaquement et ignoré si le plugin est absent ; décorations,
  panneaux, diagnostics restent **locaux**, jamais dans le CRDT. (Cette dernière règle touche au
  protocole : ADR.)

### 5.6 Versionnage et évolution

- Une **version d'API entière** par plugin dans le manifeste (`api: 1`) + `engines.inkwell`
  semver ; schéma des messages en **WIT** (`inkwell:plugin@1.x`), généré vers les structures Go et
  les types TypeScript : une seule source de vérité.
- Évolutions *additives* seulement dans une version majeure ; N et N−1 pris en charge en parallèle
  (modèle Zed : l'hôte garde les anciennes liaisons) ; fenêtre de dépréciation annoncée.
- **Trajectoire Component Model** : le jour où wazero (ou *wacogo*/*gravity*, à évaluer) exécute
  des composants 0.3 ou que les chaînes d'outils Go/Python 0.3 sont stables, on publie
  `api: 2` = même monde WIT en composant ; l'ABI octets de la v1 reste supportée. Critères de bascule
  mesurables : chaînes d'outils 0.3 « stables » pour ≥ 4 langages ; hôte Go pur validé par notre banc.
- Leçon Firefox : un modèle permissif lié aux entrailles empêche de moderniser le cœur ; Mozilla a
  dû imposer signature, permissions et API stables, et a payé la réécriture des extensions
  (<https://blog.mozilla.org/addons/2015/08/21/the-future-of-developing-firefox-add-ons/>).
  On évite la dette en partant avec l'API étroite.

## 6. Modifier ou ajouter UI et fonctions : bonnes pratiques

Critères = grille §4 plus : **K** gestion des conflits ; **U** propreté à la désinstallation ;
**A** stabilité d'API ; **F** performance.

### 6.1 Patrons examinés

| Patron | Fonctionnement et sources | K | U | A | F | Leçon |
|---|---|---|---|---|---|---|
| **Contributions déclaratives + événements d'activation** (VS Code `contributes`, `onCommand:`, `onView:`) | JSON dans le manifeste ; le code ne s'exécute qu'à l'événement ; contributions limitées aux points listés, par exemple les emplacements de menu documentés [vérifié] <https://code.visualstudio.com/api/references/contribution-points> | + (le cœur fusionne des listes) | ++ (retirer le manifeste = tout retirer) | ++ | ++ (aucun code au démarrage) | **Base du modèle.** Eclipse (registre d'extensions) et JetBrains (points d'extension XML) suivent la même idée **[mémoire]**. |
| **Crochets et filtres** (WordPress actions/filters ; Emacs `advice-add`) | Action = effet, filtre = transforme et renvoie [vérifié] <https://developer.wordpress.org/plugins/hooks/> ; Emacs : combinateurs `:around`, `:before-until`, `:after-until`… [vérifié] <https://www.gnu.org/software/emacs/manual/html_node/elisp/Advice-Combinators.html> | − priorité entière libre : « guerre des priorités » **[mémoire]** ; combinateurs Emacs : plus clairs | − (retrait manuel, fuites) | − (tout est crochet) | + | Prendre des **combinateurs nommés** (envelopper/opposer un veto/court-circuiter), jamais des entiers de priorité auto-déclarés ; ordre choisi par l'utilisateur. |
| **Extensions composables** (CodeMirror 6 : facettes, `Prec`, `Compartment`) | Facette = valeur combinée depuis plusieurs entrées via `combine` ; ordre = précision `highest…lowest` puis position ; valeurs uniques : la plus haute gagne (`tabSize`, `readOnly`) ; `Compartment.reconfigure` remplace à chaud [vérifié] <https://codemirror.net/docs/ref/#state.Facet> | ++ (règle par facette) | ++ (reconfigurer un compartiment) | ++ | ++ | **Modèle direct pour la couche éditeur** : chaque plugin = une `Extension` dans son `Compartment`, détruite en le vidant. ProseMirror (plugins, `appendTransaction`) est analogue **[mémoire]**. |
| **Intercepteurs/middleware** (décorateurs de fonctions) | Remplacer *ou* envelopper *ou* opposer un veto | dépend de la règle d'ordre | + si enregistrés | ~ | + | Voir §6.2 : trois verbes distincts, chacun avec sa règle de conflit. |
| **UI distante déclarative** (Shopify remote-dom ; Atlassian Forge UI Kit ; Slack Block Kit/Adaptive Cards **[mémoire]**) | remote-dom : arbre DOM construit dans un bac à sable (iframe ou Web Worker), rendu côté hôte via `postMessage`, **liste blanche** d'éléments/attributs/événements [vérifié] <https://github.com/Shopify/remote-dom> ; Forge UI Kit : composants natifs seuls, **aucun accès DOM**, React limité aux composants exportés [vérifié] <https://developer.atlassian.com/platform/forge/ui-kit/> | ++ (l'hôte arbitre la mise en page) | ++ | + (jeu de composants versionné) | + | **Meilleur compromis cohérence/sécurité.** Atlassian propose aussi *Custom UI* (iframe) pour l'exceptionnel : même ordre de préférence que nous. |
| **iframe / micro-frontend** | Isolation d'origine, UI libre ; Figma : iframe null-origin pour l'UI de plugin, messages asynchrones [vérifié] | − (aucune cohérence) | + | + | − (copie) | Réservé aux cas à fort besoin graphique, derrière une capacité nommée. |

### 6.2 Remplacer, envelopper, opposer un veto : règles de résolution

| Verbe | Usage | Règle de conflit | Exemples |
|---|---|---|---|
| **Contribuer** (défaut) | ajouter commande, panneau, décoration, diagnostic, snippet, thème | Collecte de toutes les entrées ; ordre : cœur → utilisateur → plugins, puis ordre d'installation ; conflits de raccourcis résolus **par l'utilisateur** | tout |
| **Envelopper** (pipeline) | transformer une valeur en transit : avant export, après collage, post-formatage | Chaîne **ordonnée par l'utilisateur** (réglage visible, glisser-déposer) ; le cœur occupe des positions fixes ; chaque étape reçoit la sortie de la précédente | collage, export, formatage |
| **Veto** | uniquement `avant-*` d'actions **locales** et non critiques | un refus doit porter une **raison affichée** ; l'utilisateur peut passer outre ; jamais sur sync, trust, signature | « exporter sans titre ? » |
| **Remplacer** | fournisseur d'un **créneau** : correcteur, formateur, compilateur de format d'export, fournisseur de complétion | **un seul fournisseur actif par créneau**, choisi par l'utilisateur (comme le formateur par défaut de VS Code) ; le défaut du cœur reste disponible | correcteur, thème, export |

### 6.3 Anti-patrons à écarter

- **Accès à l'interne par monkey-patching** (Obsidian, Atom **[mémoire]**) : casse à chaque
  refonte et supprime la possibilité de sandbox ; VS Code a refusé volontairement le DOM pour
  garder la liberté de refaire son UI [vérifié, voir §3.4].
- **Extensibilité non bornée** : le modèle XUL/XPCOM de Firefox a dû être abandonné ; le coût a été
  supporté par l'écosystème [vérifié, §5.6].
- **Priorités entières auto-déclarées** (WordPress) : course aux armements **[mémoire]**.
- **Tout est crochet** : on ne peut plus raisonner sur le comportement ni garantir la sécurité.

## 6bis. Extensibilité maximale : « tout est extension, sauf le noyau de sécurité »

**Parti pris retenu [avis, sur demande explicite de l'utilisatrice]** : la quasi-totalité des
fonctions sont des *points d'extension* ; les fonctionnalités du cœur (correcteur, barre d'outils par
format, aperçu, export, jalons, liste de problèmes, thèmes, recherche…) sont livrées comme
**plugins fournis** (*bundled*), écrits contre la **même API et le même manifeste**, sans
raccourci privé. Seul un **noyau fermé minimal** n'est pas extensible : identité, liste de membres
et rôles (`trust`), signatures et vérification des lots, synchronisation (`transport`, `p2p`),
intégrité du CRDT (le seul code qui applique des changements Automerge reste dans la webview,
validé par bornes), magasin et quarantaine, vérification des sidecars, CSP et pont, et le
gestionnaire de plugins. Les plugins fournis sont signés par la clé de build, vivent dans le
binaire, ne peuvent pas être mis à jour hors d'une mise à jour d'inkwell, mais **sont
désactivables et remplaçables** (sauf quelques-uns marqués « essentiels », par exemple l'éditeur
de texte de base).

### Preuves : qui a réussi extensibilité + sécurité + stabilité, et à quel prix

| Système | Ce qui a marché (source) | Prix payé |
|---|---|---|
| **CodeMirror 6** | Tout est `Extension` : facettes avec règle de combinaison *par facette*, `Prec` pour l'ordre, `Compartment` pour reconfigurer à chaud ; le noyau n'a presque aucune fonction « en dur » [vérifié] <https://codemirror.net/docs/ref/#state.Facet> | Aucune sécurité : les extensions sont du JS de confiance dans la page ; la courbe d'apprentissage des facettes est réelle ; conflits résolus par précédence, pas par arbitrage utilisateur **[avis]**. D'où : CM6 pour la couche éditeur *interne*, jamais exposé tel quel aux plugins tiers. |
| **VS Code** | Contributions déclaratives (`contributes`), activation par événements, hôte d'extensions séparé, DOM volontairement non exposé pour pouvoir refaire l'UI sans casser les extensions ; la quasi-totalité des fonctions du produit (langages, git, thèmes) sont des extensions fournies [vérifié pour contributions/DOM : <https://code.visualstudio.com/api/references/contribution-points>, <https://code.visualstudio.com/api/extension-capabilities/overview>] | Pas de sandbox de sécurité : une extension a les droits de l'utilisateur ; la limite des points d'extension (menus listés seulement, UI figée) frustre les auteurs et pousse aux contournements (CSS/JS injecté, « Custom UI » non supporté) **[mémoire]** ; hôte d'extensions = coût mémoire. |
| **Eclipse** | « Tout est plug-in » ; registre d'extensions lu au démarrage depuis les manifestes **sans activer** les plug-ins (activation paresseuse, passe à l'échelle) ; couches de points d'extension sur points d'extension [vérifié] <https://eclipse.googlesource.com/platform/eclipse.platform.common/+/e2ac111e372bddcc14c2d8d3272a00e00f4bbf4c/bundles/org.eclipse.platform.doc.isv/guide/int_eclipse.htm>, <https://eclipse.dev/equinox/documents/eclipsecon2006/Best%20Practices%20for%20Programming%20Eclipse%20and%20OSGi.pdf> | Le registre n'avait **aucun contrôle de sécurité** sur qui déclare ou contribue [vérifié, même document] ; complexité OSGi (trois mécanismes inter-bundles : registre, services, Declarative Services) ; coût de démarrage et de résolution de dépendances **[mémoire]**. Leçon : manifeste lu sans exécuter du code, mais ajouter *capacités* et *signature*. |
| **Zed** | Extensions Wasm/WIT/Wasmtime avec API versionnée par extension ; API publique volontairement petite (langages, thèmes, snippets, slash-commands) [vérifié] <https://zed.dev/blog/zed-decoded-extensions> | Surface encore étroite (on ne peut pas tout faire) ; permissions non documentées dans l'article lu ; maintenance de plusieurs liaisons d'API en parallèle. |
| **Typst** | Plugins Wasm purs, isolés [vérifié] <https://typst.app/docs/reference/foundations/plugin/> | Pouvoir minimal (octets entrants/sortants), pas d'UI. |
| **Firefox** | A abandonné XUL/XPCOM, trop permissif, pour WebExtensions avec permissions + signature, afin de pouvoir moderniser le cœur [vérifié] <https://blog.mozilla.org/addons/2015/08/21/the-future-of-developing-firefox-add-ons/> | Réécriture de l'écosystème, perte d'extensions, communauté mécontente. |

**Synthèse [avis]** : aucun système lu ne cumule les trois à la fois. Les **plus extensibles**
(CM6, Eclipse, VS Code) le sont avec du code de confiance dans le processus, donc sans sécurité
contre un extenseur hostile. Les **plus sûrs** (Typst, Zed, Figma) ont un pouvoir étroit. Notre
pari, non démontré par la littérature lue et à valider par prototype : **la forme de CM6/VS Code
(contributions déclaratives + combinaison par point + activation paresseuse + cœur = plugins
fournis) avec l'isolation de Typst/Zed (Wasm sans autorité + capacités déclarées)**. Les
compromis assumés : la mesure du coût d'un aller-retour hôte (§5.1) et la largeur de l'API
(commencer étroit, élargir par versions additives, comme Zed, pour ne pas payer le prix Firefox).

### Règles qui rendent l'extensibilité maximale *sans* perdre la sécurité

1. **Un seul chemin** : une fonction du cœur n'a aucun droit qu'un plugin ne puisse demander par
   capacité ; sinon c'est un point d'extension manquant (à traiter comme un bogue d'API).
2. **Le noyau fermé est une liste courte, versionnée et testée** (`scripts/arch-check.sh` :
   aucun plugin ne peut importer ni appeler `trust`, `identity`, `transport`, `store`).
3. **Tout ce qui est une *politique* est extensible ; tout ce qui est une *vérification* est
   fermé** : un plugin peut proposer, afficher, transformer, ne peut pas valider ni signer.
4. **Plugins fournis testés comme des plugins** : la suite de tests du cœur exerce l'API
   publique, ce qui protège la stabilité (si le cœur casse, l'API casse, et inversement).
5. **Coût de l'extensibilité mesuré** : démarrage < 1 s avec tous les plugins fournis
   (registre lu sans exécuter ; activation paresseuse, §5.5) ; banc `benchstat` avant l'ADR.

## 7. Catalogue des points d'extension (v1)

Chaque point a : un schéma WIT, sa **règle de combinaison** (§6.2), ses capacités requises, un
**budget** (délai/mémoire) et sa **cohérence UI**.

| Point | Verbe | Capacités | Rendu |
|---|---|---|---|
| Commande (palette, raccourci) | contribuer | selon l'usage | palette existante |
| Entrée de menu / bouton de barre d'outils (par format) | contribuer | `ui.contribute` | barre d'en-tête et barre par format existantes |
| Panneau latéral à onglets | contribuer | `ui.contribute` | panneaux à onglets existants |
| Élément de barre d'état, bannière, toast | contribuer | `ui.contribute` | composants existants |
| Diagnostics (linter) | contribuer | `doc.read` | soulignements et liste de problèmes du cœur |
| Complétion, actions rapides, slash-commands | contribuer | `doc.read`, `doc.propose` | popovers existants |
| Transformation de sélection / « avant collage » | envelopper | `doc.read`, `doc.propose` | — |
| Importeur (blob → texte), exporteur (projet → octets) | remplacer (par format) | `blob.read`, `storage` | boîte de dialogue côté Go |
| Correcteur, formateur | remplacer | `doc.read` | le défaut du cœur |
| Thèmes, snippets, grammaire | **données seulement**, sans code | — | jetons de design (`docs/design/tokens.json`) |

**Fermé aux plugins** (aucun point d'extension, aucune fonction hôte) : `trust` (membres, rôles,
révocation), `identity`, signatures et vérification des lots, `transport`/`p2p`/`mux`, `store` et
quarantaine, `invite`, `blob` (vérification Bao), lancement et vérification des sidecars, CSP,
pont (`bridge`), et **le gestionnaire de plugins lui-même**. Un plugin ne peut ni lire des octets de
pair non vérifiés ni émettre un changement signé : seule la webview, sur ordre de l'utilisateur,
signe (§5.2).

**Dogfooding** : le correcteur, la barre d'outils par format, l'export PDF, la liste de problèmes
s'enregistrent par **le même registre** et le même manifeste que les plugins, sans API privée
(seule différence : origine « cœur », pas de limite de délai). Cela garantit que l'API suffit,
que l'ordre de pipeline est testé en continu, et que désactiver une fonction du cœur et la
remplacer par un plugin est possible (principe « tout ce qui se voit se désactive »).

## 8. UI de plugin : cohérence Adwaita et accessibilité

- **UI déclarative par défaut** : le plugin renvoie un arbre `{type, props, children, on…}` d'un
  vocabulaire fermé (`row`, `entry`, `switch`, `button`, `list`, `banner`, `icon`, `markdown`,
  `progress`…) validé par liste blanche (modèle remote-dom/UI Kit). Svelte rend avec les composants
  et jetons Adwaita existants : thème clair/sombre, forte densité, mouvement réduit hérités sans
  effort.
- **Accessibilité (WCAG 2.2 AA)** : rôles ARIA, ordre de focus, contraste et taille de cible
  viennent du composant hôte ; le manifeste est refusé à l'installation si un bouton-icône n'a pas
  de libellé accessible (lint). Pas de CSS ni de HTML libre : le plugin ne peut pas dégrader
  contraste ou focus.
- **Vue libre (iframe `sandbox` null-origin, CSP propre, sans réseau)** : seulement pour un besoin
  graphique réel (canvas, diagramme), derrière la capacité nommée `ui.canvas`, dans un cadre
  Adwaita avec titre et bouton fermer ; jamais pour des formulaires ordinaires.
- **Revue de cohérence** : tout plugin du dépôt passe `ux-walkthrough` (captures clair/sombre).

## 9. Prochaines étapes (avant ADR)

1. **Banc d'essai** `benchstat` : compiler/instancier/appeler 3 plugins types (Rust, Go/TinyGo, JS
   QuickJS-Wasm) sous wazero, mesurer taille binaire du moteur ajouté et mémoire résidente.
2. **Prototype** d'un point d'extension complet (diagnostic) + une proposition d'édition de bout en bout
   (plugin → Go → webview → transaction Automerge → annuler).
3. Confirmer **[mémoire]** : fermeture sur délai de wazero, garanties de limite mémoire de
   Extism, support de `wasi_snapshot_preview1` minimal ; état réel de *wacogo*/*gravity*.
4. Réponse aux questions ouvertes : espace `ext/<id>` dans le document (protocole), format du
   catalogue signé, politique de révocation hors ligne, plugins par projet vs globaux.
5. ADR « Plugins » (frontières, fermé/ouvert, CSP, manifeste), puis ajout à `docs/exigences.md`.

## Sources principales

- WASI 0.3 : <https://bytecodealliance.org/articles/WASI-0.3> · <https://wasi.dev/releases> ·
  <https://wasi.dev/languages>
- Toolchains : <https://component-model.bytecodealliance.org/language-support/go.html> ·
  <https://github.com/bytecodealliance/jco> · <https://bytecodealliance.org/articles/jco-1.0>
- wazero : <https://github.com/tetratelabs/wazero/issues/2200> ·
  <https://github.com/tetratelabs/wazero/releases>
- wasmtime-go : <https://pkg.go.dev/github.com/bytecodealliance/wasmtime-go/v50> ·
  <https://github.com/bytecodealliance/wasmtime-go>
- Wasmtime perf : <https://bytecodealliance.org/articles/wasmtime-10-performance> ·
  <https://docs.rs/wasmtime/latest/wasmtime/struct.PoolingAllocationConfig.html>
- Extism : <https://extism.org/docs/concepts/host-functions> · <https://github.com/extism/go-sdk>
- Zed : <https://zed.dev/blog/zed-decoded-extensions> · Typst :
  <https://typst.app/docs/reference/foundations/plugin/>
- Figma : <https://www.figma.com/blog/how-we-built-the-figma-plugin-system/>
- VS Code : <https://code.visualstudio.com/api/extension-capabilities/overview> ·
  <https://code.visualstudio.com/api/references/contribution-points>
- Obsidian : <https://forum.obsidian.md/t/security-of-the-plugins/7544>
- proxy-wasm : <https://github.com/proxy-wasm/spec> · Shopify :
  <https://shopify.dev/docs/apps/build/functions>
- CodeMirror : <https://codemirror.net/docs/ref/#state.Facet> · WordPress :
  <https://developer.wordpress.org/plugins/hooks/> · Emacs :
  <https://www.gnu.org/software/emacs/manual/html_node/elisp/Advice-Combinators.html>
- remote-dom : <https://github.com/Shopify/remote-dom> · Forge UI Kit :
  <https://developer.atlassian.com/platform/forge/ui-kit/>
- Firefox : <https://blog.mozilla.org/addons/2015/08/21/the-future-of-developing-firefox-add-ons/>
- Benchmarks wazero/Wasmtime (journal CI, non daté) :
  <https://gist.github.com/codefromthecrypt/48877abc4b3ba5ceca12b312926b1e0a>
