# Revue Fable n°2 du système de design (révision 2, 2026-11-01) — verdict : **NOT ACCEPTED**

Revue indépendante de `spec.md` rév. 2, `coverage.md`, `inventory.md`, `tokens.json`,
`CHANGES.md`, contre les revues n°1, `docs/exigences.md` (N17, N19–N22), `ux-principles.md`,
le glossaire, `docs/architecture/spec.md` §15 et le code de `frontend/src` (`main`, `f8e906c`).

**Méthode.**
- Fermeture de chaque constat de la revue n°1 vérifiée dans le texte, puis dans le code.
- Les 102 paires de `tokens.json` recalculées par script (WCAG 2.2, luminance relative,
  translucides composés en sRGB sur `over`) : 200 valeurs (clair, sombre, HC).
- `coverage.md` relu ligne par ligne ; les états « S » (conforme, preuve existante)
  contrôlés par échantillon dans le code.
- libadwaita 1.8 revérifié en ligne (`css-variables.html`) : `--disabled-opacity` 50 % / 40 %
  HC, `--dim-opacity` 55 % / 90 %, `--border-opacity` 15 % / 50 %, erreur `#c30000`/`#ff938c`,
  succès `#007c3d`/`#78e9ab`, avertissement `#905400`/`#ffc252`, surfaces identiques à §2.2.

## 1. Fermeture des constats de la revue n°1

| # | Fermé ? | Preuve |
|---|---|---|
| C1, C2, C3 | oui | recalcul : 10,48 / 7,85 / 3,14 exacts ; `over` sur chaque paire ; C-SEL, C-GUT ; `HistoryRow.svelte:158`, `editor/setup.ts:38` vérifiés |
| C4 | **en partie** | `--hl-link`, `--person-N-line`, `--person-N-ink`, `fontFamily`, `modes` présents ; mais deux paires HC ont un `fg` non résoluble (§3, B1-c) |
| C5 | **non** | `App.svelte:197` inchangé ; l'inventaire dit pourtant « Corrigé » (M1) |
| C6, C7, C8, C9, C10, C11, C12, C14 | oui | L-8c, L-9 ; F-1 sur toutes les barres ; ↑ ouvre sur le dernier ; retour au déclencheur sans exception ; `<select>` natif ; régions persistantes ; TO-1, L-8b en DOIT ; §3.1, §3.4, L-10 |
| C13 | **en partie** | §10 est une traçabilité règle → assertion, mais sept fichiers de preuve sont cités comme existants alors qu'ils n'existent pas (M2) |
| F1 | oui | `--focus-ring-on-dark` : 14,51 · 12,57 · 21 / 12,03 · 20,24 · 21 recalculés exacts |
| F2, F3, F4, F5, F6 | oui | §3.1 (Fichiers, Sommaire, Tâches, Liens), §3.3 RSZ-07/08, §3.4 (A6) ; L-8 questions d'abord ; §4.27 hors périmètre, L-2b ; §2.5 écart explicite ; P5/P6 invariants |
| F7 | oui | T-3 à 0,4 (= libadwaita HC 40 %) ; `--duration-fade` seul conservé sous `reduce` |
| F8 | **en partie** | §6.4 et G-61 ; mais la liste des lignes est fausse (m3) |
| F9, F10, F11, F12 | oui | TO-1, A-13 ; G-18 en vague 2, G-53 en vague 1 ; §10 (sous réserve de M2) ; §11 |

## 2. Recalcul des contrastes

Script : `/tmp/…/scratchpad/contrast.py` (résolution de `var()`, `light-dark()`,
`color-mix()`, composition alpha sur `over`).

- **196 valeurs sur 200 identiques à ±0,02** ; aucune paire non interdite sous son `min` ;
  aucune translucide sans fond parent.
- Échantillon confirmé : `--fg`/`--view-bg` 12,57 / 16,82 ; toast 14,51 / 12,03 ; infobulle
  12,57 / 20,24 ; `--success` 5,32 / 9,04 ; `--warning` sombre sur vue 10,48 ; `--error` sombre
  7,85 ; `--mark-code` 5,53 / 7,35 ; `--person-4-line` 3,14.
- Quatre écarts de 0,03–0,04 (`--person-3/6/7/8-line` en clair : 6,90 / 7,64 / 6,29 / 6,01
  contre 6,93 / 7,61 / 6,33 / 5,97) : le JSON stocke un hexadécimal arrondi, la spec définit un
  `color-mix` (m2).
- Six valeurs de paires `forbidden` **passent** dans un mode (`--accent` sur `--stage-bg` clair
  = 3,46 ≥ 3 ; les variantes sombres de toast, infobulle, scène ; `--fg-dim` sur `--selected-bg`
  sombre 4,98 ; sur `--info-bg` clair 4,56) : T-5 « chaque paire `forbidden` échoue » est faux
  tel quel (m1).

## 3. Constats

### Bloquants

**B1 — N22 (thèmes modifiables par une extension) n'existe pas dans le système de design.**
- *Norme.* `docs/exigences.md` N22 ; `docs/architecture/spec.md` §15.4 ligne 1403 (« contrat
  de jetons §15.8 ; validation des contrastes ≤ 20 ms ») et §15.8 TH-1 à TH-4 ; WCAG 1.4.3,
  1.4.11, 2.4.7 ; HIG *Accessibility* (contraste élevé, thème du système).
- *Preuve.* `spec.md` ne contient le mot « thème » qu'aux lignes 18, 52–53, 76–82, 1193 et
  1378 (« Thème : Système, Clair, Sombre ») ; `coverage.md` n'a aucune ligne N22 ;
  `inventory.md` aucun écart ; `tokens.json` ne porte aucune métadonnée de contrat. L'architecture
  renvoie vers `tokens.json` comme contrat (TH-1), et le design ne le définit pas.
- *Ce qui manque, et qui relève du design, pas de l'architecture :*
  - a. le **contrat** dans `tokens.json` : quels jetons sont *remplaçables* (surfaces, texte,
    états, `--hl-*`, marques, rayons, ombres, polices fournies), *dérivés* (`--person-N-line`
    et `--person-N-ink` se calculent de `--person-N`, sinon un thème peut livrer un triplet
    incohérent), *verrouillés* (`--focus-ring` ≥ 3:1 sur chaque surface, `--disabled-opacity`,
    `--target-*`, durées sous `reduce`, les modes HC « jamais affaiblis » de TH-2) ;
  - b. les **règles de validation au chargement**, côté personne : refus complet (TH-2) ; le
    message, en phrases simples (P4, E-2) : « Le thème « Encre » n'a pas été appliqué : 3
    couleurs manquent de contraste. · Détails » ; l'état de la ligne d'extension (§4.29,
    tableau des états) ; ce qui se passe quand un thème valide en clair échoue en sombre ou
    en HC (refus global, ou refus du mode ?) ;
  - c. le contrat doit être **calculable par machine**, et il ne l'est pas : deux paires ont
    `fg = "--fg-dim (contraste élevé)"` et `"--border (contraste élevé)"` (clés `hcLight`,
    `hcDark`), qu'aucun hôte ne peut résoudre. Les modes HC doivent passer par `modes`
    (`hc-light`, `hc-dark`) des jetons eux-mêmes ;
  - d. l'**interface** : Préférences › Apparence liste les thèmes installés (après Système,
    Clair, Sombre), l'aperçu d'un thème avant de l'appliquer, le changement sans scintillement
    (≤ 50 ms, TH budget), l'interaction avec `data-theme`, `prefers-contrast` et
    `forced-colors` (un thème n'a aucun effet sous `forced-colors`, T-4) ;
  - e. les cadres d'extension (§4.29) reçoivent les jetons **du thème actif**, pas ceux
    d'Adwaita ; et un thème ne peut pas toucher `--mark-extension` (l'attribution reste
    reconnaissable).
- *Correction.* Un §2.10 « Thèmes : contrat de jetons » dans `spec.md`, une ligne N22 dans
  `coverage.md` §5, `tokens.json` enrichi (`contract: overridable | derived | locked`, HC par
  `modes`), §5.13 Apparence complété, et un écart G-71 dans l'inventaire.

**B2 — Les points d'affichage des extensions (§4.29) contredisent l'architecture (§15.4) et
ne couvrent pas « hautement extensible ».**
- *Norme.* `docs/exigences.md` N21 (« programme hautement extensible, exigence forte ») ;
  `docs/architecture/spec.md` §15.4 (dix-huit points) et X-1 ; Nielsen 1 et 4 ; §4.29
  lui-même (« on doit savoir ce qui vient d'une extension »).
- *Preuve.* `spec.md:1180–1186` : cinq points (panneau **de gauche seulement**, bouton de
  barre, commande, visionneuse, export). `architecture/spec.md:1380–1408` ajoute : onglet du
  **tiroir**, **barre d'état** (bandeau, toast), **page de réglages**, **décorations et
  survol** dans l'éditeur, vues d'historique, surcouches de l'aperçu, **importateurs**, menus
  et menu contextuel, thèmes et icônes. Sur ces points, le design ne dit ni où, ni comment,
  ni comment on reconnaît l'extension. La règle « dessiné dans un cadre fourni par Inkwell »
  est inapplicable à une décoration en ligne, à un élément de barre d'état, à un survol ou à
  une entrée de menu : il faut une règle d'attribution par point (icône `--mark-extension`,
  infobulle « par « Diagramme » », groupe « Extensions »).
- *Incohérence interne.* L-2 et K-2 : « un panneau entre dans l'un des **deux** sélecteurs
  selon la répartition » ; §4.29 n'offre que le sélecteur de gauche. Une extension
  « Activité étendue » ou « Commentaires » (Q8) irait à droite.
- *Correction.* Reprendre le tableau de §4.29 point par point depuis §15.4, avec pour chacun :
  l'emplacement, l'attribution visible, l'état « désactivée / plantée », et ce qui est interdit.
  Ajouter une ligne « Extensions » dans `coverage.md` §3 (APG) pour les points qui créent des
  widgets (panneau = `tabpanel`, bouton = `toolbar`, réglage = ligne de préférences).

### Majeurs

**M1 — G-0 est présenté comme corrigé ; il ne l'est pas.**
- *Norme.* `inventory.md` §0 (« chaque écart cite sa preuve ») ; Nielsen 1 et 3 ; N19.
- *Preuve.* `inventory.md` G-0 : « **Corrigé par un lot séparé** » ; `coverage.md` §5 renvoie
  à G-0 sans réserve. Le code sur `main` : `App.svelte:197`
  `onOpen={() => session.focus()}` ; `ui/shell/Sidebar.svelte:49` transmet `node.path`.
  Aucun commit ne le corrige (`git log -5`).
- *Correction.* « **Ouvert**, hors du périmètre N17, lot à venir » ; vague 0 « à faire ». Un
  document de design ne déclare jamais un écart fermé sans preuve, par le principe 7.

**M2 — Des preuves déclarées existantes n'existent pas ; des lignes « S » sans preuve.**
- *Norme.* `coverage.md` légende : S = « conforme dans le code (**preuve existante**) » ;
  `spec.md` §10 (« une règle sans assertion n'est pas finie ») ; principe 7 ; même classe
  que Codex n°1 C4.
- *Preuve.* §10 cite `frontend/e2e/theme.spec.ts`, `e2e/focus.spec.ts`, `e2e/a11y.spec.ts`,
  `e2e/targets.spec.ts`, `e2e/perf.spec.ts`, `ui/messages.test.ts`, `commands.test.ts`
  sans « à ajouter » ; `ls frontend/e2e` et `frontend/src` : **aucun n'existe**. Seules les
  lignes T-1 disent « à ajouter ».
  Dans `coverage.md`, 19 lignes sont « S » avec « — » en preuve : 1.3.2, 1.3.4, 2.1.2, 2.2.2,
  2.3.1, 2.4.6, 2.5.1, 2.5.2, 3.1.2, 3.2.1, 3.2.2, 3.2.3, 3.3.2, 3.3.7, APG *Grid*, HIG
  *Popovers*, *Listes encadrées*, *Style d'écriture*, *Performance perçue*. Trois sont
  fausses :
  - 3.2.2 « le changement de rôle donne un toast Annuler » décrit la spec, pas le code
    (`PeopleDialog.svelte` n'a pas ce toast) → S→N ;
  - HIG *Popovers* « Échap … le focus revient au déclencheur » contredit G-25
    (`MenuButton.svelte:54` `session.focus()`) → S→G ;
  - *Performance perçue* « preuve §10 » renvoie à `e2e/perf.spec.ts`, absent → S→N.
  Vérifié exact par ailleurs : 1.3.5 (`ShareDialog.svelte:43`), 2.1.2 (`showModal` dans les
  sept dialogues), 2.2.2 (`HeaderBar.svelte:293`, `Preview.svelte:306` coupent les
  animations infinies sous `reduce`), 2.1.4 (`present/keys.ts:61–66`), APG *Grid*
  (`Grid.svelte:660`).
- *Correction.* Marquer « (à écrire) » dans §10 ; donner une preuve (fichier:ligne ou test) à
  chaque « S », sinon le passer en S→N.

### Mineurs

**m1 — T-5 : « chaque paire `forbidden` échoue ».** Faux dans un mode pour six valeurs (§2).
Écrire « échoue dans au moins un mode ; le test est par mode ». `tokens.json#/contrast`.

**m2 — `--person-N-line` : hexadécimal arrondi contre `color-mix`.** Quatre rapports diffèrent
de 0,03–0,04 selon la source. Déclarer l'hexadécimal du JSON canonique (et le mettre dans
`tokens.css`), ou donner au test de parité une tolérance de ±0,05. `spec.md:194`,
`tokens.json` `--person-3-line` et suivants.

**m3 — Références de l'inventaire.** `workspace.svelte.ts` est
`workspace/workspace.svelte.ts` (:120, :181, :247 exacts) ; `FileIcon.svelte` est
`ui/icons/FileIcon.svelte` (:29–30 exacts) ; `StatusBar.svelte` (G-29) est
`ui/shell/StatusBar.svelte` (:132–150 exacts). G-61 cite `fr.json:619`
(« history.row.merged.sub », sans « fusion ») et oublie `:621` (« pas encore fusionnée ») et
`:663` (« la fusionner »).

**m4 — 2.1.4 et §4.23 oublient la touche `L` (pointeur) :** `present/keys.ts:67–69`. A-20 et la
ligne 2.1.4 de `coverage.md` doivent lister B, W et L, et dire qu'ils n'agissent que dans la
vue de présentation (c'est le cas : clavier de `present/*` seulement).

**m5 — `--tooltip-bg` sombre : la spec dit « idem » (= `#36363a`, `spec.md:119`), le JSON dit
`rgb(0 0 6 / 80%)` sur `--view-bg`.** C'est le JSON qui donne 20,24 ; blanc sur `#36363a`
vaut 11,95. Les deux passent, mais la spec et le contrat divergent : aligner la ligne 119.

**m6 — 3.1.2 « interface monolingue » :** §5.13 affiche « English » dans l'interface
française. L'option porte `lang="en"` (WCAG 3.1.2, exemple 2 de *Understanding*). Même règle
pour `fr.json` ↔ `en.json` sur « Français ».

**m7 — 3.3.2 « libellé toujours visible » (§4.13) contre `SearchField.svelte:55–69`
(placeholder + `aria-label`, pas de libellé visible).** Soit §4.13 exempte les champs de
recherche (loupe = libellé, HIG *Search*), soit la ligne devient S→G. Aujourd'hui, la ligne
« S » et la règle se contredisent.

**m8 — Sur-ingénierie, à simplifier :**
- FT-3 « colonnes repérées » dans la **vue texte** d'un CSV (teinte alternée + infobulle par
  colonne dans CodeMirror) : la vue texte est un repli, le tableur est la vue de lecture.
  Garder le séparateur en `--hl-marker` et l'en-tête en gras ; retirer la teinte et
  l'infobulle (`spec.md:1049–1056`).
- TO-1 « le toast le dit une fois par session : “Vous pouvez aussi annuler avec Ctrl+Z” » :
  une seconde phrase dans un toast limité à 60 caractères (§4.12). La mettre dans l'Aide et
  dans l'infobulle du bouton Annuler (`spec.md:703`).

## 4. Lecture de `coverage.md`, ligne par ligne

- **Nielsen 1–10** : complets, chaque ligne renvoie à une section réelle de la spec et à des
  écarts existants. Rien à redire.
- **WCAG 2.2** : les 55 critères A et AA sont présents (31 A sans 4.1.1, 24 AA), aucun
  oubli ; les justifications NA (1.2.x, 1.4.2, 2.5.4) sont exactes. Corrections : M2 (preuves),
  m4 (2.1.4), m6 (3.1.2), m7 (3.3.2).
- **APG** : 14 motifs, cohérents avec §4. Il manque la ligne « Extensions » (B2) et la ligne
  « Fenêtre de raccourcis / Aide » n'a pas de motif (dialogue : ajouter à *Dialog*).
- **HIG / libadwaita** : valeurs revérifiées exactes (§0 de cette revue) ; *Popovers* à passer
  en S→G (M2).
- **Domaines** : il manque **N22** (B1). « Extensions » doit renvoyer aux points de §15.4 (B2).
  « Performance perçue » en S→N (M2). Les autres domaines (accueil, états, textes,
  notifications, collaboration, multi-fichiers, aperçu/export/impression, réglages, fr/en,
  thèmes clair/sombre/HC, tailles, types de fichiers, commentaires, présentation) sont couverts
  et renvoient à des écarts datés.

## 5. Ce qui est bon, et qu'il ne faut pas rouvrir

- §2.2 est désormais exact et complet au centième ; les paires interdites sont une bonne
  idée.
- L-8 (questions d'abord), L-8b, TO-1, §4.11 (régions persistantes) : fermés proprement.
- §4.25 (onglets), §4.28 (types de fichiers, lecture d'abord, dix jetons réutilisés,
  `language-data` paresseux avec liste blanche) et §4.29 (consentement en phrases, révocable,
  rien ne s'installe seul) sont simples et justes ; §4.29 ne pèche que par ses points
  d'affichage (B2).
- L'ordre des vagues de l'inventaire est le bon.

## Verdict

**NOT ACCEPTED.** Bloquants à lever avant la révision 3 :

1. **B1** — N22 : contrat de jetons (remplaçable / dérivé / verrouillé, HC par `modes`,
   paires résolubles), validation au chargement côté personne, Apparence, ligne de couverture.
2. **B2** — points d'affichage des extensions alignés sur l'architecture §15.4, avec une
   règle d'attribution par point, et les deux sélecteurs.

Puis M1 (G-0 « ouvert »), M2 (preuves « à écrire », « S » prouvés ou rétrogradés), et les
mineurs m1–m8.

Décompte : 2 bloquants, 2 majeurs, 8 mineurs.
