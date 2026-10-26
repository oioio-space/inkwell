---
status: accepted
date: 2026-10-26
decision-makers: [Mathieu]
---

# Types de fichiers modulaires : un descripteur par type, chargé à la demande

## Contexte et problème

Le type d'un fichier décidait en trois endroits, par des `if` sur « typst » ou « markdown » :
l'éditeur, la barre de mise en forme et le décompte de mots. Les icônes avaient un quatrième
tableau. Le gestionnaire de fichiers (B5) ouvre maintenant n'importe quel fichier : texte
ordinaire, code, table, image, PDF, polices. Ajouter un type ne doit pas obliger à retoucher
cinq fichiers, ni alourdir le paquet de ceux qui n'en ouvrent aucun.

## Facteurs de décision

- Ajouter un type = un seul module, sans édition ailleurs.
- Le paquet ne grossit pas par type : l'éditeur et la langue se chargent à l'ouverture.
- Un fichier texte s'ouvre toujours en texte ; un fichier binaire n'est jamais affiché en
  caractères.
- Le choix peut être corrigé fichier par fichier, et il se réplique (ADR 0023).

## Options envisagées

- **T1. Tables dans chaque consommateur** (l'état d'avant).
- **T2. Un registre de descripteurs** dans `frontend/src/filetypes/`, un module par type.
- **T3. Greffons chargés à l'exécution.** Écarté : la CSP interdit le code dynamique.

## Décision

T2.

### Le descripteur

`FileType` (`filetypes/types.ts`) :

- `id` (minuscules, stable), `label` (clé i18n), `icon` (clé de `FILE_MARKS`), `extensions[]`,
  `filenames[]` (sans extension : `readme`, `.gitignore`), `mime[]`, `sniff(octets) → confiance`
  optionnel ;
- `open: () => import(…)` : un `EditorModule { mount(hôte, contexte) → EditorInstance }` avec
  `destroy`, `focus`, `selection`, `rename`, `setType`, `statusItems` ;
- capacités optionnelles : `language` (langue CodeMirror, chargée à la demande),
  `formatProfile` (barre A1), `wordCount` (A2, `"none"` la masque), `preview` (C2, B8),
  `insertItems`, `dialect` (B9), `binary`.

### Ordre de résolution

`resolve({name, override, bytes, isText})` :

1. le choix explicite gardé dans l'index (attribut `ftype` du nœud, ADR 0023) ;
2. le nom entier, puis l'extension ;
3. l'analyse des premiers octets (`sniff`, confiance > 0,5) ;
4. le texte brut, si c'est du texte ;
5. la visionneuse binaire.

Un document texte ne reçoit jamais de visionneuse, même si son nom dit `.png`.

### Ajouter un type

Créer `filetypes/<type>.ts` qui appelle `register({…})`, et l'importer dans `filetypes/index.ts`.
Aucun autre fichier ne change : `register` refuse une extension déjà prise. Le test de
`filetypes.test.ts` le prouve avec un type factice.

Modules : `typst`, `markdown`, `csv` (CSV et TSV), `plaintext` (texte, journal, configuration,
BibTeX), `code` (JSON, YAML, TOML, XML, HTML, CSS, JavaScript, TypeScript, Python, Go, shell,
SQL, autre code), `image`, `pdf`, `binary` (polices, archives, binaire).

### Chargement à la demande

`open` et `language` sont des `import()` : Vite en fait des fragments séparés. Les langues
viennent des paquets officiels `@codemirror/lang-*` (versions exactes, plus de 7 jours) et du
mode shell de `@codemirror/legacy-modes`. La table CSV pointe vers `ui/sheet/module.ts` (B9) par
`import.meta.glob` : tant que le module n'existe pas, elle s'ouvre en texte.

### Côté Go

`internal/project` classe un fichier avec une table de données (extension → `Kind`, nom entier →
`Kind`), puis une analyse du contenu. Les valeurs de `Kind` et le format du fil sont inchangés.
Une extension ajoutée à la table ne change aucun comportement hors de la liste des fichiers.

## Conséquences

- Bien : un type s'ajoute en un module ; les langues ne pèsent que sur ceux qui les ouvrent ; les
  icônes et les libellés viennent d'un seul endroit.
- Moins bien : deux tables d'extensions (Go pour la liste du disque, TS pour l'interface), à garder
  voisines ; `langOf` devient une lecture du registre.

### Confirmation

`filetypes.test.ts` : résolution, ordre, chargement à la demande, type factice, légende
(chaque type a une icône nommée en français et en anglais). `kinds_test.go` : la table Go.

## Plus d'informations

Voir ADR 0023 (attribut de nœud), ADR 0026 (grille CSV).
