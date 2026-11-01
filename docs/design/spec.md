# Système de design d'Inkwell — spécification (N17)

- **Statut :** proposition, **révision 4** (2026-11-01).
  - Elle répond aux revues n°1 à n°3 de Codex et de Fable ([`reviews/`](reviews/)), et intègre
    N22 à N25 et N27 à N32.
  - Le détail des changements est dans [`CHANGES.md`](CHANGES.md).
  - Les choix marqués « *à confirmer par l'utilisatrice* » restent des recommandations (§11).
- **Portée :** toute l'expérience d'interface du frontend (`frontend/src`) :
  - architecture de l'information et navigation ;
  - premier lancement ;
  - composants et leurs états ;
  - textes, erreurs et annulation ;
  - collaboration ;
  - projets à plusieurs fichiers ;
  - types de fichiers et code (N20) ;
  - extensions (N21), thèmes et cycle de vie (N22), extensions de projet (N23), ce qu'une
    extension ne peut pas changer et les diagrammes (N24), outils de développement (N25) ;
  - modèles de projet (N27), problèmes de compilation (N28), aide à l'écriture Typst (N29) ;
  - outils d'écriture (N30), IA (N31), ressources des extensions (N32) ;
  - aperçu, export et impression ;
  - réglages, raccourcis, palette et aide ;
  - langues, thèmes et tailles de fenêtre ;
  - performance perçue.
- **Couverture :** la grille de contrôle (heuristiques de Nielsen, WCAG 2.2 A et AA, GNOME HIG
  et libadwaita 1.8) est dans [`coverage.md`](coverage.md). Les reviewers s'y réfèrent.
- **Sources internes :**
  - [`ux-principles.md`](../ux-principles.md) ;
  - [`glossary.md`](../glossary.md) ;
  - le [plan T11](../plans/2026-10-25-t11-interface.md) : maquette v3 et exigences A1 à C4+,
    validées ;
  - [`conception.pdf`](../conception.pdf) ;
  - `CLAUDE.md` (« Cohérence », invariants).
- **Sources externes :** §12 ; repères `[S…]`.

## 0. Conventions

- **DOIT**, **NE DOIT PAS**, **DEVRAIT**, **NE DEVRAIT PAS** et **PEUT**, en capitales,
  s'entendent au sens de BCP 14 ([RFC 2119], [RFC 8174]).
- Une règle numérotée (`T-3`, `L-8`…) est vérifiable : §10 dit par quelle assertion.
- « Personne » désigne l'utilisatrice ou l'utilisateur ; « pair » n'apparaît que pour le code.
- **Contrastes :**
  - ils sont calculés selon WCAG 2.2 (luminance relative) ;
  - une couleur translucide est composée en sRGB, sans arrondi intermédiaire, sur le **fond
    parent déclaré** (`over` dans `tokens.json`) ;
  - les valeurs sont données en clair / sombre.

## 1. Principes

| # | Principe | Ce qu'il impose | Source |
|---|---|---|---|
| P1 | **Le texte d'abord** | Une barre d'en-tête, une barre d'onglets de fichiers, une barre de mise en forme repliable, une barre d'état. Le reste se replie ou vit dans un panneau, un popover ou un menu. | règles 1–2 ; HIG *Header bars* [S2] |
| P2 | **Un seul langage** | Un besoin = un composant, un nom, un raccourci. | `CLAUDE.md` « Cohérence » |
| P3 | **Réversible plutôt que confirmé** | Une action annulable s'annule par un toast **et** reste annulable ensuite (§4.12). On ne confirme que l'irréversible pour les autres : retirer une personne, vider la corbeille, passer la main, revenir à une version pour tout le document. | règle 6 ; HIG *Dialogs* [S2] |
| P4 | **Chaque message dit quoi faire** | Une phrase (ce qui se passe, ce qui est préservé) et une action. Toute attente de plus de 3 s montre une progression. | règle 5 ; HIG *Spinners* [S2] |
| P5 | **Zéro jargon** | Le glossaire, et lui seul ; le technique derrière « Détails techniques ». | règle 4 ; glossaire ; invariant `CLAUDE.md` |
| P6 | **Pour tout le monde** | WCAG 2.2 AA, clavier complet, lecteur d'écran, thème et contraste du système, mouvement réduit. | règle 9 ; [S4] [S5] |
| P7 | **Natif et sobre** | Apparence libadwaita 1.8, thème du système ; ni couleur sans rôle, ni animation sans fonction. | « Système visuel » ; [S1] |

**Arbitrage.**
- **P5 et P6 sont des invariants** : P5 en est un de `CLAUDE.md`, P6 est une obligation
  légale et morale. Ils ne cèdent jamais.
- Entre les autres, l'ordre est **P4 > P3 > P2 > P1 > P7**.
- La fidélité à Adwaita (P7) cède donc devant un contraste insuffisant (anneau de focus, §2.2).
- En rév. 1, P5 était classé dernier sans justification (revue Fable, point 6).

---

## 2. Jetons de design

### 2.1 Architecture

- **T-1.** Trois niveaux de jetons, dans `frontend/src/theme/tokens.css` :
  - les primitifs `--p-*` ;
  - les sémantiques `--*` ;
  - les composants, qui n'emploient que des sémantiques.

  Un composant NE DOIT PAS employer de valeur brute : couleur, ombre, rayon, durée, taille de
  texte. Exceptions : `0`, un filet de `1px`, `50%` pour un cercle, et les tailles d'icône.

  La règle vaut aussi pour les **couleurs et les ombres** des thèmes CodeMirror écrits en
  TypeScript (`EditorView.theme`, `baseTheme` dans `editor/*.ts`), parce qu'un thème (§2.10) doit
  pouvoir les changer. Les tailles et marges internes de CodeMirror en sont exemptées : on vise
  le résultat (T-7, T-9), pas le moyen (Codex R2-m1).
- **T-2.** Le clair et le sombre DOIVENT passer par `light-dark()` avec
  `color-scheme: light dark`. Le système décide par défaut ; `data-theme="light|dark"` force le
  choix (réglage Apparence, §5.13).
  - `light-dark()` est Baseline 2024 [S6], mais son support par WebKitGTK n'est pas confirmé.
  - Un e2e DOIT lire une couleur calculée sous WebKitGTK, dans les deux thèmes (§10).
  - Repli : un bloc `@media (prefers-color-scheme: dark)` produit à partir des mêmes valeurs.
- **T-3.** `prefers-contrast: more` applique les valeurs `hc-light` et `hc-dark` de
  `tokens.json` (libadwaita [S1]) :
  - `--fg-dim` à 90 % ;
  - `--border` à 50 % ;
  - `--disabled-opacity` à 0,4.
- **T-4.** Sous `forced-colors: active`, tout état porté par un fond (sélection, pression,
  focus, bascule) a aussi un contour ou une forme [S6].
- **T-5.** `docs/design/tokens.json` est la référence lisible par machine **et le contrat de
  thème** (§2.10). `tokens.css` est ce que le code consomme. Le **test de parité est à écrire**
  (inventaire G-1b) :
  - chaque jeton de `tokens.json` existe dans `tokens.css` sous son nom `css`, avec ses valeurs
    par mode, **à l'hexadécimal près** ; les hexadécimaux du JSON sont canoniques, y compris pour
    les jetons dérivés ;
  - chaque paire **non interdite** de `contrast.pairs` atteint `min` dans les quatre modes
    (`light`, `dark`, `hcLight`, `hcDark`) ;
  - une paire `forbidden` est un **usage interdit par politique**, pas un échec attendu : elle
    peut passer dans un mode (par exemple `--accent` sur `--stage-bg`, 3,46 / 12,40). Le test
    vérifie que ce couple **n'apparaît pas dans le code** (analyse des styles), pas qu'il échoue.

  Aujourd'hui, seules les neuf couleurs syntaxiques sont vérifiées (`theme/tokens.test.ts:24-58`).
- **T-6.** Un jeton se nomme par son rôle, jamais par sa valeur.

### 2.2 Couleurs

Base : variables de libadwaita 1.8 [S1]. Les valeurs exactes et les 128 paires de contraste
calculées sont dans `tokens.json`. Ci-dessous : les rôles et les décisions.

#### Surfaces

| Jeton | Rôle | Clair | Sombre |
|---|---|---|---|
| `--window-bg` | fenêtre, parent des bandeaux | `#fafafb` | `#222226` |
| `--view-bg` | éditeur, listes, grille | `#ffffff` | `#1d1d20` |
| `--headerbar-bg` | barres : en-tête, onglets, mise en forme, état | `#ffffff` | `#2e2e32` |
| `--sidebar-bg` | panneau de gauche, fond de l'aperçu | `#ebebed` | `#2e2e32` |
| `--side-bg-2` | tiroir de droite | `#f3f3f5` | `#28282c` |
| `--card-bg`, `--popover-bg` | cartes, popovers | `#ffffff` | 8 % blanc / `#36363a` |
| `--dialog-bg` *(nouveau)* | dialogues | `#fafafb` | `#36363a` |
| `--toast-bg` *(nouveau)* | toasts | `rgb(0 0 6 / 84%)` | `#36363a` |
| `--tooltip-bg` | infobulles | `rgb(0 0 6 / 80%)` (texte blanc 12,57 sur vue) | `rgb(0 0 6 / 80%)` sur vue sombre (20,24) |
| `--stage-bg`, `--stage-overlay` *(nouveaux)* | scène de présentation | `#000`, `rgb(0 0 0 / 55%)` | idem |
| `--page-bg` *(nouveau)* | papier de l'aperçu | `#fff` | `#fff` |
| `--backdrop` *(nouveau)* | voile derrière un dialogue | `rgb(0 0 6 / 30%)` | `rgb(0 0 0 / 45%)` |

#### Texte et interaction

| Jeton | Clair / sombre | Contraste minimal mesuré |
|---|---|---|
| `--fg` | `rgb(0 0 6 / 80%)` / `#fff` | 9,56 (sur sélection, barre latérale) |
| `--fg-dim` | 55 % / 55 % (HC 90 %) | 4,54 sur barre latérale claire ; 4,87 sur popover sombre |
| `--accent` (texte, liens, focus) | `#0461be` / `#81d0ff` | 5,10 (barre latérale claire) |
| `--accent-bg` | `#3584e4` | remplissage seulement |
| `--suggested-bg` + `--accent-fg` | `#1c71d8` + blanc | 4,77 |
| `--on-dark-fg` *(nouveau)* | blanc | 12,03 (toast sombre) |
| `--disabled-opacity` *(nouveau)* | 0,5 (HC 0,4) | exempté (1.4.3) |

**Filets et bordures** (`--border`) : en clair et en sombre, le filet d'Adwaita (1,41 à 1,61:1)
est **décoratif**. Champs, lignes et boutons s'identifient par leur fond, leur libellé et leur
position, ce que permet WCAG 1.4.11 (« pas besoin d'une bordure si le composant est identifiable
autrement »). En **contraste élevé**, il porte l'information : trois paires `hcOnly` le vérifient
à 3:1 (3,83 à 5,14 ; Fable R3-M3).

**Texte secondaire dans une carte** : `--fg-dim-on-card`. En sombre, 55 % ne tient pas sur une
carte de dialogue (4,13:1, nouvelle paire de la rév. 4) ; il passe à 65 % (5,07:1). **Texte
d'interface sur le papier de l'aperçu** : `--page-fg` (`#55555a`, 7,41:1), car `--fg` devient
blanc en sombre, sur un papier qui reste blanc (1,00:1).

**Paires interdites** : usages interdits par politique, marqués `forbidden` dans `tokens.json`.

| Paire | Rapport | Règle |
|---|---|---|
| `--fg-dim` sur `--selected-bg` | 4,48 (vue) · 4,26 (barre latérale) | **C-SEL** : sur une ligne sélectionnée, le texte secondaire prend `--fg`. Écart actuel : `HistoryRow.svelte:158`. |
| `--fg-dim` sur `--info-bg` sombre | 3,95 | Texte de bandeau en `--fg`. |
| blanc sur `--accent-bg` | 3,77 | Texte sur `--suggested-bg` seulement. |
| `--accent` sur `--toast-bg` / `--tooltip-bg` / `--stage-bg`, en clair | 2,39 / 2,07 / 3,46 | Anneau `--focus-ring-on-dark` (ci-dessous). |
| `opacity` sur du texte atténué (`--fg-dim` × 0,8) | 3,23 | Interdit (§9). |
| numéros de ligne `--fg` à 45 % | 2,51 | **C-GUT** : les numéros de ligne sont du texte et prennent `--fg-dim` (4,74). Écart actuel : `editor/setup.ts:38`. |

#### Anneau de focus — écart assumé avec libadwaita

| Jeton | Valeur | Où | Contraste |
|---|---|---|---|
| `--focus-ring` | `2px solid var(--accent)`, décalage 2px | partout sauf ci-dessous | min 5,10 / 7,10 sur les sept surfaces |
| `--focus-ring-on-dark` *(nouveau)* | `2px solid var(--on-dark-fg)` | toasts, infobulles, vues de présentation | 14,51 · 12,57 · 21 / 12,03 · 20,24 · 21 |

- libadwaita emploie un anneau translucide, qui échoue à 1.4.11 [S4]. P6 l'emporte sur P7.

#### États

| Jeton | Clair | Sombre | Note |
|---|---|---|---|
| `--success` | `#007c3d` (était `#2ec27e`, 2,30 ✗) | `#78e9ab` | texte et pastille : 5,32 / 9,04 |
| `--success-bg` *(nouveau)* | `#2ec27e` | `#26a269` | remplissage |
| `--warning` / `--warning-bg` | `#905400` / `#fdf3e1` | `#ffc252` / 14 % | sur vue : 6,08 / **10,48** |
| `--error` = `--destructive` | `#c30000` | `#ff938c` | sur vue : 6,32 / **7,85** ; sur `--error-bg` (fenêtre) : 5,38 / 5,67 |
| `--destructive-bg` *(nouveau)* | `#e01b24` | `#c01c28` | texte blanc : 4,83 / 6,11 |
| `--info-bg` | `#e3eefb` | `#2b4b73` | `--fg` : 11,29 / 8,91 |

- Les chiffres en gras corrigent la rév. 1, qui mesurait le sombre sur `--window-bg`
  (revue Codex).

#### Différences

`--diff-add-*` et `--diff-del-*` sont inchangés.

| Contexte | Rapport | Fond de calcul |
|---|---|---|
| dans `DiffView` | ajout 8,55 / **6,24** · suppression 8,40 / **5,94** | dialogue sombre |
| dans l'éditeur | 8,55 / 8,46 · 8,40 / 7,96 | vue |

- **C-DIFF.** L'ajout est aussi **souligné**, la suppression **barrée**. Le lecteur d'écran
  entend « ajouté : » et « supprimé : » (texte masqué visuellement).

#### Syntaxe, marques de fichiers

- Syntaxe : les dix jetons `--hl-*`, dont `--hl-link` = `--accent`, atteignent tous ≥ 4,77 sur
  la vue.
- Marques de fichiers (1.4.11, graphiques porteurs de sens) : on corrige Markdown `#1c71d8`,
  CSV `#1b8553` et image `#c64600`. Le minimum devient **3,23** (Typst, sur la barre latérale
  claire).

#### Couleurs des personnes

- Les huit couleurs d'Okabe-Ito [S8] : `--person-1` à `--person-8`.
- **Encre** `--person-N-ink` : noir, sauf blanc sur `--person-5`. Contraste minimal 5,19.
- **Trait** `--person-N-line` : en clair, `--person-N` × 0,65 (canaux sRGB, arrondis). Les
  **hexadécimaux de `tokens.json` sont canoniques** et s'écrivent tels quels dans `tokens.css`
  (`#9c942b` pour la personne 4, par exemple). En sombre, c'est `--person-N` lui-même. Il sert au
  curseur distant, à la marge, à la bande de modifications et au contour du curseur d'un
  suiveur. Contraste minimal **3,14** en clair (jaune), 3,24 en sombre (bleu).
- **Fond** de sélection ou de « qui a écrit quoi » : 25 % de la couleur. Il est décoratif, car
  le nom est donné ailleurs.
- **C-PERSON.** La couleur n'est jamais le seul indice : le nom est visible près du curseur, au
  survol d'une pastille et dans chaque légende.
- **C-PERSON-2** *(recommandation Q4, à confirmer par l'utilisatrice)*. La couleur se choisit à
  l'accueil ; le créneau libre est proposé par défaut. **Le créneau inscrit dans le projet fait
  foi.** La préférence locale n'est qu'un souhait :
  - en entrant dans un projet où le créneau voulu est pris, la personne reçoit le premier
    créneau libre ;
  - un toast l'annonce : « Votre couleur ici : bleu ».
  - si deux personnes reçoivent le même créneau en même temps (ajouts concurrents), l'ordre de
    la liste signée des membres départage, et la seconde passe au créneau suivant.
  - au-delà de 8 personnes, l'initiale et le nom départagent.

  Écart actuel : couleur par hachage modulo 8 (`ui/people.ts:25-31`).

### 2.3 Typographie

Polices : `--font-ui` (Adwaita Sans), `--font-mono` (Adwaita Mono), `--font-serif` (Source
Serif 4). Aucune n'est téléchargée (CSP).

| Jeton | Valeur CSS (`font`) | Adwaita | Usage |
|---|---|---|---|
| `--text-caption` | `400 12px/1.33 var(--font-ui)` | `.caption` | raccourcis dans les menus, étiquette de curseur, mention « principal » |
| `--text-small` | `400 13px/1.4` | — | barre d'état, sous-titres, infobulles, lignes secondaires |
| `--text-body` | `400 14px/1.45` | `.body` | défaut |
| `--text-heading` | `700 14px/1.25` | `.heading` | titres de barre, de carte, de panneau ; boutons |
| `--text-title-4` | `700 16.5px/1.3` | `.title-4` | titre de dialogue |
| `--text-title-2` | `800 19px/1.25` | `.title-2` | écran vide |
| `--text-title-1` | `800 25px/1.2` | `.title-1` | accueil, À propos |
| `--text-document` | `400 16px/1.6` | `.document` | lecture longue : différences, aperçu HTML, notes, aide |
| `--text-editor` | `400 calc(14px × zoom)/1.75 var(--font-mono)` | monospace | éditeur, tableur |

- Les tailles en pourcentage viennent du source de libadwaita ; elles ne sont pas revérifiées
  en ligne (§12).
- **T-7.** Pas de texte sous 12px, sauf dans le rendu Typst de l'aperçu.
- **T-8.** Les chiffres qui changent sont en `tabular-nums`.
- **T-9.** Le texte tient au zoom de 200 % (1.4.4) et avec les espacements de 1.4.12.

### 2.4 Espacement

Grille de 2px, avec les multiples d'Adwaita : `--space-0` à `--space-6` = 2, 4, 6, 8, 12, 18,
24 et 36px. Les jetons existants (`-1` à `-4`) gardent leur valeur ; `-0`, `-1b`, `-5` et `-6`
sont nouveaux.

### 2.5 Cibles et hauteurs

| Jeton | Valeur | Où |
|---|---|---|
| `--target-floor` | 24px | plancher absolu, WCAG 2.5.8 [S4] |
| `--target-bar` | 34px | **toute barre** : en-tête, onglets, mise en forme, barre secondaire, barre d'état. Dans la barre d'état, la zone active fait 34px de haut, même si le dessin fait 28px. |
| `--target-min` | 44px | dialogues, listes, lignes de préférences, accueil |
| `--bar-header` · `--bar-tool` · `--bar-sub` · `--bar-status` · `--tab-height` | 47 · 46 · 40 · 36 · 34px | hauteurs des barres |

**Écart explicite avec `ux-principles.md` (règle 9 : « cibles de 44px »).**
- La spec suit la contrainte globale du plan T11, validée et postérieure aux principes :
  « Cibles ≥ 34px dans les barres et ≥ 44px dans les dialogues ».
- 34px est la taille des boutons de barre d'Adwaita.
- Le plancher légal reste 24px.

*Amendement de la règle 9, à confirmer par l'utilisatrice :* « cibles de 44px dans les
dialogues et les listes, 34px dans les barres, jamais moins de 24px ».

### 2.6 Rayons

`--radius-small` 6 · `--radius-button` 9 · `--radius` 12 (cartes, popovers) ·
`--radius-window` 15 (**dialogues**, fenêtre) · `--radius-pill` 999 (toasts, pastilles,
onglets en pilule). Aucun autre rayon.

### 2.7 Élévation et couches

| Niveau | Jeton | Usage |
|---|---|---|
| 0 | filet `--shade` | barres, panneaux |
| 1 | `--shadow-card` | cartes, pages de l'aperçu |
| 2 | `--shadow-popover` | popovers, menus, panneau superposé |
| 3 | `--shadow-dialog`, `--shadow-toast` | dialogues, toasts |

- `--z-bar` 2 · `--z-overlay` 5 · `--z-popover` 20 · `--z-tooltip` 30 · `--z-toast` 40.
- Les dialogues et la présentation passent par la *top layer*.

### 2.8 Mouvement

| Jeton | Valeur | Usage | Sous `prefers-reduced-motion: reduce` |
|---|---|---|---|
| `--duration-fast` | 100ms | survol, pression | 0 |
| `--duration-fade` *(nouveau)* | 150ms | fondus d'opacité **seulement** | **conservé** |
| `--duration` | 200ms | popover, toast, bandeau, chevron | 0 |
| `--duration-slow` | 300ms | panneau, tiroir | 0 |
| `--ease-out` · `--ease-in-out` | `cubic-bezier(.33,1,.68,1)` · `(.65,0,.35,1)` | entrée · déplacement | — |

- **T-11.** Sous `reduce` :
  - aucune translation, mise à l'échelle ni rotation ;
  - aucune animation infinie : pulsation « Connexion… », miroitement, halo du pointeur ;
  - défilement instantané ;
  - seuls les fondus en `--duration-fade` restent.
- **T-12.** Aucune animation au-delà de 300ms, sauf un indicateur d'attente. Rien ne clignote
  plus de 3 fois par seconde (2.3.1).

### 2.9 Iconographie

- **I-1.** Les icônes sont symboliques, en 16 × 16, avec un trait de 1.6 et `currentColor`
  (`ui/icons/paths.ts`). Seule la marque d'une icône de fichier est en couleur.
- **I-2.** Une icône seule dans un bouton a un nom accessible et une infobulle (§4.6). Une icône
  à côté d'un libellé est `aria-hidden`.
- **I-3.** Tailles :
  - 16px dans les barres et les menus ;
  - 12–14px pour fermer ou cocher ;
  - 64px sur un écran vide ;
  - une illustration seulement à l'accueil [S2].
- **I-4.** Une idée = une icône (§8.3).
- **I-5.** Dans une ligne qui affiche déjà le nom, `FileIcon` est `aria-hidden` ; le type est lu
  dans la description.

---

### 2.10 Thèmes : le contrat de jetons (N22)

Un thème est une **extension de données**, sans code ni CSS (architecture §15.8, TH-1 à TH-4).
Il fournit des valeurs de jetons pour les modes `light`, `dark`, `hc-light` et `hc-dark`. N17
fixe ce qu'un thème peut changer, comment l'hôte le valide, et ce que la personne voit.

**TH-D1. Trois classes de jetons**, déclarées dans `tokens.json` (`contract`). **Tout jeton en a
une** ; un jeton sans classe est une erreur du fichier, refusée par le test.

| Classe | Sens | Jetons |
|---|---|---|
| **remplaçable** (`overridable`) | un thème peut fournir une valeur, dans les **bornes typées** (`bounds`) | surfaces (y compris `--btn-bg`, `--hover-bg`, `--selected-bg`, `--card-bg`, `--page-bg`, toasts, infobulles, scène), `--fg`, `--fg-dim`, `--accent`, `--accent-bg`, `--suggested-bg`, états, différences, `--hl-*` (sauf lien), marques (sauf extension), `--person-1` à `-8`, rayons (sauf pilule), ombres, polices de la liste fournie, courbes, durées normales |
| **dérivé** (`derived`) | l'hôte le **recalcule** ; une valeur fournie est ignorée | `--person-N-line`, `--person-N-ink`, `--hl-link` = `--accent`, `--destructive` = `--error`, couleur de `--focus-ring` = `--accent` |
| **verrouillé** (`locked`) | jamais changé | `--on-dark-fg` et `--tooltip-fg` (blanc), `--focus-ring-on-dark`, épaisseurs et décalages, `--disabled-opacity`, `--mark-extension`, cibles, hauteurs, espacements, échelle de texte et `--text-editor`, couches, taille minimale de fenêtre, `--duration-fade`, **délais d'accessibilité** (`--tooltip-delay`, `--spinner-delay`, durée des toasts), durées sous `reduce` |

- **Toasts, infobulles et scène restent sombres.** Leur texte est verrouillé en blanc. Un thème
  peut changer leur fond, mais la validation exige ≥ 4,5:1 contre ce blanc (choix de la HIG :
  ces surfaces sont sombres dans Adwaita).
- **Bornes typées** (`bounds`, structurées dans `tokens.json`) :
  - couleur : sRGB, opacité de 0 à 100 % ;
  - longueur : `{min, max, unit}` ;
  - durée : `{min, max}` en ms ;
  - police : un nom de la liste fournie, ou un fichier `woff2` du paquet.

  Une valeur hors bornes refuse le thème, et la raison est donnée.

**TH-D2. Validation au chargement**, côté hôte, en ≤ 20ms (architecture §15.4) :
1. **Compatibilité du contrat.** Le thème déclare `contract: "^1.0"`.
   - Une majeure inconnue est refusée : « Ce thème est fait pour une version plus récente
     d'Inkwell. »
   - Une mineure plus récente est acceptée : ses jetons inconnus sont ignorés et notés dans
     l'Inspecteur.
   - Un jeton **requis** par une version annoncée (champ `required` du contrat) qui manque
     refuse le thème. Un jeton **facultatif** absent garde la valeur d'Adwaita.
2. L'hôte compose le thème sur `tokens.json`, recalcule les dérivés, puis évalue **toutes les
   paires non interdites** dans les quatre modes.
   - Les paires couvrent **chaque état interactif** : texte sur `--btn-bg`, `--hover-bg` et
     `--selected-bg`, composés sur chacune des surfaces où ils apparaissent ; texte sur
     `--card-bg` et `--page-bg` ; anneau de focus contre chaque surface (Codex R3-B1).
   - Contre-exemple de test : `--hover-bg` noir en clair donne `--fg` sur survol à 1,00:1, et le
     thème est **refusé**.
3. **Planchers** : chaque paire a `min`, et `hcMin` pour les modes HC (Fable R3-M1).
   - En `hc-light` et `hc-dark`, le texte principal (`--fg`, `--fg-dim`) doit tenir **7:1**
     (WCAG 1.4.6, AAA). Les autres paires gardent leur seuil, et `--border` y doit atteindre 3:1
     (`hcOnly`).
   - **On ne compare plus au rapport d'Adwaita.** Un thème « papier » (`--view-bg` `#f5f0e6`) donne
     15,56:1 en HC et il est **accepté**. Un thème qui met `--fg-dim` HC à 55 % est refusé.
4. Les couleurs de personnes restent distinctes : ΔE2000 ≥ 15 (TH-4 ; Adwaita : 21,3 au minimum).
5. **Un seul échec, et le thème entier est refusé.** Rien n'est appliqué à moitié, pas même un
   mode sans l'autre.

**TH-D3. Ce que la personne voit** (Préférences › Apparence, §5.13) :
- **Thème** : une liste radio, « Adwaita (par défaut) » puis les thèmes installés, chacun avec
  sa **vignette** (fond, texte, accent, en clair et en sombre). Le choix s'applique tout de suite
  (≤ 50ms, une écriture des variables sur `:root`).
- **Variante** : Système, Clair, Sombre. Elle est indépendante du thème.
- Après un changement : toast « Thème « Encre » appliqué · Annuler ».
- **Thème refusé** :
  - la ligne reste, désactivée, avec `warn` et une phrase **exacte**. Par exemple : « Pas assez
    lisible en contraste élevé : le texte secondaire atteint 5,1:1, il faut 7:1. » ou « Fait pour
    une version plus récente d'Inkwell. » ;
  - « Détails » liste chaque paire refusée, avec son rôle en clair, son mode, le rapport mesuré
    et le seuil ;
  - le thème en place n'est pas touché.
- **Revenir au thème par défaut** : en tête du groupe Thème, toujours ; le mode sans extensions
  ignore tout thème (§4.33).
- **Ordre de priorité :**
  - `forced-colors` passe avant tout ;
  - puis `prefers-contrast: more`, qui sélectionne les modes HC du thème ;
  - puis la variante ;
  - puis le thème.

  Un projet ne peut pas imposer de thème (§4.32). Les cadres d'extension reçoivent les jetons du
  thème actif (§4.31).

## 3. Architecture de l'information et mise en page

### 3.1 Les zones

```
┌──────────────────┬──────────────────────────────────────────────┬────────────────┐
│ [Fichiers|Somm.| │ Barre d'en-tête (47) : projet / fichier, état│ Tiroir (380)   │
│  Tâches|Liens]   ├──────────────────────────────────────────────┤ [Historique |  │
│                  │ Onglets de fichiers (34)                     │  Activité |    │
│ Panneau de       ├──────────────────────────────────────────────┤  Rechercher]   │
│ gauche (264)     │ Barre de mise en forme (46), repliable       │                │
│                  ├──────────────────────────────────────────────┤                │
│                  │ Bandeau (au plus un, §3.4)                    │                │
│                  ├───────────────────────┬──────────────────────┤                │
│                  │ Source                │ Aperçu               │                │
│                  ├───────────────────────┴──────────────────────┤                │
│                  │ Barre d'état (36)                             │                │
└──────────────────┴──────────────────────────────────────────────┴────────────────┘
```

**Répartition**, arbitrée entre T11 B6, B7, B10 et la revue :

| Panneau | Ce qu'il contient | Pourquoi |
|---|---|---|
| **Gauche** : la *structure* | Fichiers, Sommaire, Tâches, Liens | B10, validé le 2026-10-26, place Fichiers, Sommaire et Tâches à gauche, en `AdwViewSwitcher`. B7 place Liens « dans la barre latérale ». |
| **Droite** : le *temps et les autres* | Historique, Activité, Rechercher, **Relecture** (§4.38), **IA** (§4.39) ; Inspecteur en mode développeur (§4.35) | Rechercher : recommandation Q5, *à confirmer*. Dans le tiroir, la recherche reste visible pendant qu'on parcourt les fichiers. |

- La rév. 1 mettait Tâches dans le tiroir et oubliait Sommaire et Liens (revue Fable, point 2).
- **L-1.** Le panneau de gauche, l'aperçu et le tiroir :
  - se replient chacun d'un geste ;
  - ont un onglet actif, une taille et un état replié mémorisés **par projet, sur cet
    ordinateur**.

  Ces réglages ne sont jamais synchronisés.
- **L-2.** Un nouveau panneau DOIT entrer dans l'un des deux sélecteurs, selon cette
  répartition : la structure à gauche, le temps et les autres à droite. Sinon, une ADR.
- **L-2b.** **Commentaires** : hors du périmètre de N17 (§4.27). L'onglet DOIT être **masqué**
  tant que la fonction n'existe pas.

  Écart actuel : l'onglet est affiché, avec un texte vide (`HistoryDrawer.svelte:207`).

### 3.2 Navigation

| Je veux… | Clic | Clavier | Palette |
|---|---|---|---|
| ouvrir un fichier | Fichiers › ligne | Ctrl+P (« Aller au fichier ») | oui |
| changer de fichier ouvert | onglet | Ctrl+Page↓ / Page↑, Ctrl+Tab | oui |
| aller à un titre | Sommaire › titre | Ctrl+Maj+O | oui |
| aller à une ligne, à un problème | barre d'état | Ctrl+G ; F8 / Maj+F8 (problèmes) | oui |
| passer d'une zone à l'autre | — | F6 / Maj+F6 | — |
| revenir où j'étais | — | Alt+← / Alt+→ (historique de navigation) | oui |
| trouver une commande | menu › « Rechercher une commande… » | Ctrl+Maj+P | — |

- **N-1.** La position (fichier, ligne, défilement) de chaque onglet est mémorisée et rendue au
  retour.
- **N-2.** Le titre de la fenêtre et le sous-titre de la barre d'en-tête disent toujours où l'on
  est : fichier, puis projet.

Alt+← et Alt+→ s'écartent de la HIG, qui déconseille Alt [S2]. C'est la convention de
navigation des navigateurs, connue de tous.

### 3.3 Panneaux ajustables

| Frontière | Défaut | Min | Max | Sous le min |
|---|---|---|---|---|
| panneau de gauche | 264px | 180px | 400px | se replie |
| aperçu | 50 % | 260px | 70 % | se replie |
| tiroir, y compris Rechercher | 380px | 300px | 50 % | se replie |
| vue présentateur : diapositive ↔ notes | notes 30 % de la hauteur | 15 % | 60 % | — |
| vue présentateur : courante ↔ suivante | 65 % / 35 % | 50 % | 80 % | — |
| tableur ↔ aperçu, panneau de recherche ouvert (RSZ-07) | trois zones ; les règles ci-dessus s'additionnent | | | |

- **L-3.** Chaque frontière est une poignée APG *Window Splitter* [S5] :
  - `role="separator"`, focalisable (`tabindex="0"`) ;
  - **nom accessible** « Redimensionner le panneau Fichiers », « … l'aperçu », « … les notes » ;
  - `aria-orientation="vertical"` entre deux panneaux côte à côte, `"horizontal"` entre
    diapositive et notes ;
  - `aria-valuenow`, `aria-valuemin` et `aria-valuemax`, en pourcentage de la zone ;
    `aria-controls` vers le panneau qu'elle redimensionne ;
  - clavier : flèches ±16px, Maj+flèches ±64px, Origine/Fin, Entrée pour replier ou rouvrir ;
  - double-clic pour la taille par défaut ;
  - zone de saisie de 8px pour un filet visible de 1px.
- **L-4.** Chaque redimensionnement a une alternative sans glisser (2.5.7) : le clavier, et les
  entrées du menu Affichage.

### 3.4 États de la fenêtre

Le plan A6 fait foi (validé, 900px). Les seuils d'Adwaita (400/550/860sp [S1]) sont arrondis
aux siens.

| État | Largeur | Comportement |
|---|---|---|
| **Large** | ≥ 1200px | Trois zones côte à côte ; le tiroir pousse le contenu. |
| **Moyen** | 900–1199px | Le tiroir **recouvre** l'aperçu (niveau 2). |
| **Étroit** | 560–899px | Comme A6 : le panneau de gauche **et l'aperçu** deviennent des panneaux superposés (`AdwOverlaySplitView`). La source prend toute la largeur ; « Aperçu » ouvre l'aperçu par-dessus. Le tiroir est superposé lui aussi. Un seul panneau superposé à la fois ; Échap ou un clic extérieur le ferme. |
| **Très étroit** | 360–559px | Comme Étroit. La barre d'en-tête ne garde que fichiers, titre, Partager et le menu. Les boutons épinglés passent dans le menu Extensions (§4.30). Les onglets deviennent un menu « Fichiers ouverts ». Les dialogues s'ouvrent en feuille en bas. La barre d'état applique ST-3. |

- **L-5.** Taille minimale : 360 × 480px. À 320px CSS (zoom 400 %), tout se relit **sans
  défilement horizontal** (1.4.10).
  - **Le texte des documents** (`.txt`, `.md`, `.typ`, code) **revient à la ligne** : sous 560px,
    le retour à la ligne est forcé, quel que soit le réglage, et les commandes restent
    accessibles par le menu.
  - **Seul ce qui est intrinsèquement à deux dimensions** (le code revient à la ligne lui aussi : un réglage personnel « sans retour à la ligne » ne vaut qu'au-delà de 560px) est exempté, par la note de 1.4.10 :
    - la grille du tableur ;
    - les pages de l'aperçu ;
    - les tableaux d'une vue d'extension ;
  - Le test (§10) couvre les quatre formats, en plus du chrome.
- **L-6.** Pas de barre de défilement horizontale sur la fenêtre. Aucun texte coupé sans ellipse,
  ni sans moyen de lire le texte entier.
- **L-7.** Les barres débordent dans un menu ⋯, groupe par groupe.

### 3.5 Messages : bandeaux et toasts

**Trois sortes de messages** (HIG [S2]) :

| Sorte | Composant | Exemple |
|---|---|---|
| **Événement** passé | toast | « 3 lignes intégrées » |
| **État** qui dure | bandeau | « Hors ligne » |
| **Question** qui attend une décision | bandeau-question, ou dialogue | « Léa demande à rejoindre » |

- **L-8. Au plus un bandeau visible.** Ordre de priorité :
  1. **États critiques pour l'écriture en cours** : écriture sur le disque impossible, fichier
     actif disparu ou devenu binaire, accès retiré. Ils sont rares et brefs, et la personne ne
     peut rien faire de sûr avant de les connaître.
  2. **Questions**, de la plus ancienne à la plus récente :
     - fichier modifié hors d'Inkwell en mode « Demander » (§5.16) ;
     - demande d'accès (admin, §5.4) ;
     - prise de main (pilote, §5.11) ;
     - demande de suppression d'un fichier (admin, B5) ;
     - mise à jour des liens après un renommage (§5.6).
  3. **Autres états qui bloquent l'écriture** : projet scellé, accès suspendu ou en lecture,
     fichier trop lourd.
  4. **Information** : quelqu'un présente, fichier volumineux, hors ligne prolongé, mode sans
     extensions, mode développeur (§4.35, qui reste toujours visible : voir L-8d).
- **L-8b.** S'il reste d'autres messages, le bandeau affiche un bouton :
  - « **+1 question** » en style suggéré s'il y a au moins une question en attente ;
  - sinon « +N autres ».

  Le bouton ouvre le popover de l'état de synchronisation (§4.15), qui DOIT lister tous les
  messages en cours. Une question n'est donc **jamais invisible** : au pire, elle est signalée
  par ce bouton sous un état critique. Un état critique n'est jamais caché par une question.
- **L-8c.** Les bandeaux **locaux** d'un composant passent par le même `BannerHost`, dans la
  zone de ce composant :
  - le tableur (`ui/sheet/Sheet.svelte:469`, `:541`, `:555`, `:567`) ;
  - les différences (`ui/history/HistoryBars.svelte:143`).
- **L-8d.** Le **mode développeur** et le **mode sans extensions** ont un repère permanent, qui
  n'est pas un bandeau : une pastille dans la barre d'en-tête, §4.33 et §4.35. Ils ne prennent
  donc jamais la place d'une question.
- **L-9. Au plus un toast**, dans une file gérée par un seul `ToastHost`. Aujourd'hui, trois
  sources s'ignorent : `NoticeToast`, `ExternalChangeToast` et le toast du tableur
  (`ui/sheet/Sheet.svelte:605`, minuterie à `:169`). Un toast plus récent remplace le courant ;
  le remplacé reste rattrapable (TO-1).
- **L-10.** Un toast « Annuler » ne suit qu'une action **réversible** : « Paul est en lecture »,
  « Fichier mis à la corbeille ». Retirer une personne est irréversible : c'est une confirmation,
  sans « Annuler ».

## 4. Composants

Une implémentation par composant. Les classes globales vivent dans `theme/adwaita.css`, les
composants dans `ui/`.

**Les neuf états de base.** Chaque composant **interactif** les décrit ; un élément décoratif,
seulement ceux qui le concernent.

| État | Rendu commun |
|---|---|
| repos | — |
| survol | `--hover-bg`, `--duration-fast` |
| focus | `--focus-ring` (ou `--focus-ring-on-dark` sur un fond sombre), en `:focus-visible` seulement |
| actif / pressé | `--selected-bg` ; contour sous `forced-colors` |
| désactivé | `--disabled-opacity`, `disabled` ou `aria-disabled`, sans survol. **Une infobulle dit pourquoi**, si ce n'est pas évident. |
| chargement | `aria-busy`, indicateur après 300ms (§4.21) |
| erreur | `--error` + texte, avec `aria-invalid` ou un bandeau |
| vide | écran vide (§4.20) |
| hors ligne | voir ci-dessous |

**Hors ligne.**
- Les actions qui exigent le réseau restent actives et **mettent en attente** : « Sera envoyé au
  retour du réseau ».
- Seules restent inactives celles qui ne peuvent pas attendre (« Rejoindre », « Inviter à
  proximité ») ; leur infobulle dit pourquoi.
- **À l'envoi**, chaque action en attente est **revérifiée** avec les droits de ce moment-là. Une
  action refusée (rôle changé, accès suspendu) n'est pas perdue : elle reste sur cet ordinateur
  (ADR 0021). Un toast le dit : « 2 modifications n'ont pas été envoyées : votre accès a changé.
  · Voir ». « Voir » ouvre l'Historique, filtré sur ces modifications.

**Clavier :** APG [S5].

**Retour du focus.** Quand un conteneur transitoire se ferme (menu, popover, dialogue, panneau
superposé, palette), le focus revient à l'élément qui l'a ouvert. **Replis**, si cet élément
n'existe plus ou n'est plus focalisable (supprimé, désactivé, extension retirée) :
1. l'élément qui avait le focus juste avant l'ouverture (APG *Dialog*, « retour logique ») ;
2. sinon, le conteneur parent du déclencheur, sur son élément courant (barre d'outils, liste) ;
3. sinon, l'éditeur de l'onglet actif ;
4. sinon, le titre de la zone principale.

Une commande qui **ouvre un autre contenu** (un fichier, un panneau) donne le focus à ce
contenu, pas au déclencheur.

### 4.1 Barre d'en-tête (`ui/shell/HeaderBar.svelte`)

- **Anatomie, à gauche :**
  - bascule du panneau de gauche ;
  - état de synchronisation (§4.15).
- **Anatomie, au centre :**
  - **nom du projet** (H-1, recommandation Q2, *à confirmer*) ;
  - en dessous, le fichier actif, précédé d'un point s'il n'est pas enregistré.
- **Anatomie, à droite :**
  - Aperçu ;
  - Présenter (Typst) ;
  - pile de personnes ;
  - **Partager** (bouton suggéré) ;
  - Historique ;
  - menu principal.
- **H-2.** À droite, au plus **six contrôles du cœur**, plus le bouton **Extensions** (§4.30) et
  le groupe de ses boutons **épinglés**, placé juste avant lui.
  - Un espace libre reste pour déplacer la fenêtre.
  - Épinglés visibles : 6 au plus en Large, 2 en Moyen, aucun en Étroit ; les autres restent
    dans le menu Extensions.
  - Un seul bouton suggéré, « Partager » : écart assumé avec la HIG, c'est l'action à
    découvrir.
  - Le titre, la pile des personnes, l'état et « Partager » sont des **zones protégées** (§4.33).
- **H-3.** Infobulles par le composant §4.6, jamais par `title=`.
- **H-4.** Les bascules ont un **nom fixe** (« Fichiers », « Aperçu », « Historique ») et
  `aria-pressed`.
- **Clavier :** F10 = menu principal [S2].
- **Lecteur d'écran :** `<h1>` = projet.

### 4.2 Menus (`ui/popover/MenuButton.svelte`, `.mi`)

- **Clavier** (APG *Menu Button*, *Menu* [S5]) :
  - Entrée, Espace ou ↓ ouvrent sur le **premier** élément ; ↑ ouvre sur le **dernier** ;
  - ↑ ↓ en boucle ; Origine et Fin ;
  - une lettre tapée va au prochain élément qui commence par elle ;
  - Échap ferme et rend le focus au **déclencheur** ; Tab ferme et poursuit.

  Écarts actuels :
  - ↑ ouvre sur le premier élément (`MenuButton.svelte:115`) ;
  - Échap rend le focus à l'éditeur (`:54`).
- **Contenu :**
  - verbes à l'infinitif ;
  - « … » si l'action demande une saisie de plus ;
  - raccourci à droite en `--text-caption` ;
  - groupes de 2 à 5, au plus 12 éléments.
- **Menu principal**, dans cet ordre :
  1. **Affichage** : Barre de mise en forme, Panneau de gauche, Aperçu, Historique.
  2. **Projet** : Projets…, Nouveau fichier, Rechercher une commande…
  3. **Document** : Présenter, Exporter…, Imprimer…
  4. **Préférences**, **Raccourcis clavier**, **Aide**, **À propos d'Inkwell**. Ces entrées
     sont toujours à la même place (3.2.6).

### 4.3 Barres d'outils

Cela vaut pour la barre de mise en forme (`FormatBar.svelte`), la barre du tableur
(`Sheet.svelte:479`), la barre de l'aperçu et la barre de la vue présentateur.

- **F-1.** APG *Toolbar* [S5] :
  - un seul arrêt de tabulation, avec tabindex itinérant ;
  - ← → se déplacent ; Origine et Fin vont aux extrêmes ;
  - ↓ ou Entrée ouvrent un menu ;
  - le dernier bouton focalisé est mémorisé.
- **Clavier propre à la barre de mise en forme :** Alt+F10 amène le focus de l'éditeur dans la
  barre ; Échap le rend à l'éditeur.
- **F-2.** Pour une personne en lecture ou suspendue, la barre de mise en forme est
  **masquée**. Le bandeau d'accès explique pourquoi.
- **F-3.** Le contenu dépend du format (T11 A1, UI-01) :
  - Typst : souligné, formule ;
  - Markdown : cases à cocher ;
  - code en ligne **et** bloc de code ;
  - Insérer.

  Infobulle : « Gras `*texte*` Ctrl+B ».

### 4.4 Boutons

| Variante | Classe | Usage |
|---|---|---|
| plat | `.fb` | barres, icônes |
| plat + libellé | `.fb.lbl` | actions de toast, barre secondaire |
| normal | `.btn` | dialogues, bandeaux |
| suggéré | `.sug` | une action principale par vue |
| destructeur | `.destructive` *(nouveau, global, remplace `.danger` local)* | retirer, vider, passer la main |
| bascule | `.fb[aria-pressed]` | on/off |

- **B-1.** Au plus un bouton suggéré et un destructeur par vue.
- **B-2.** Des verbes précis, jamais « OK » [S2].
- **B-3.** Un bouton occupé garde son libellé et prend `aria-busy` + `disabled`. Au-delà de
  300ms, un indicateur d'attente s'affiche.

### 4.5 Choix : groupes de bascules, onglets, interrupteurs, sélecteurs

- **Groupe de bascules** (`AdwToggleGroup` [S1]) :
  - `radiogroup` / `radio`, à un arrêt de tabulation ;
  - ← → déplacent **et** choisissent.

  Écarts actuels, sans flèches :
  - `Preferences.svelte:52` ;
  - `DiffView.svelte:181` ;
  - `HistoryBars.svelte:162`.
- **Sélecteurs de vue** (panneau de gauche, tiroir) : même rendu, sémantique APG *Tabs* [S5] :
  - `tablist`, `tab`, `tabpanel`, `aria-controls` ;
  - ← → avec activation automatique ; Origine et Fin.
  - Sous 300px de large, les onglets montrent l'icône seule, avec une infobulle et le nom
    accessible.
- **Interrupteur** : `role="switch"`, Espace. Écart actuel : cases à cocher dans
  `HistoryDrawer.svelte:121-122`.
- **Sélecteur** : `<select>` natif habillé en `.btn`, **quel que soit le nombre d'options**. Le
  modèle de sélection reste celui du système ; un menu est réservé aux **actions** (correction
  de la rév. 1).

### 4.6 Infobulle (`Tooltip`, *à créer*)

- **Anatomie :** libellé en gras, syntaxe en mono, raccourci en `.kbd` ; fond `--tooltip-bg`,
  texte `--tooltip-fg` (12,57 / 20,24).
- **Comportement** (APG *Tooltip* [S5], 1.4.13 [S4]) :
  - au survol après `--tooltip-delay` (500ms), au focus immédiatement ;
  - reste tant que le pointeur est sur l'élément ou sur l'infobulle ;
  - **Échap la ferme** sans déplacer le focus.
- **TT-1.** Une seule infobulle à la fois. Elle **ne couvre jamais l'élément focalisé ni son
  anneau** (2.4.11) : sous les barres du haut, au-dessus de la barre d'état ; elle se retourne si
  elle sort de la fenêtre.
- **TT-2.** L'attribut `title=` est interdit comme infobulle (48 occurrences, inventaire G-19).
- **Lecteur d'écran :** `role="tooltip"`, relié par `aria-describedby`.

### 4.7 Popover (`.pop`)

- Une flèche, `--radius`, `--shadow-popover`. Au plus un tiers de la fenêtre [S2], sauf les
  problèmes et Insérer.
- Ni « Fermer » ni « Terminé » [S2] ; Échap et le clic extérieur ferment, et le focus revient
  au déclencheur.
- Rôle `menu` (actions seulement) ou `dialog` avec `aria-label` (champs, texte).

### 4.8 Dialogue (`Dialog.svelte`, *à créer*)

- **Remplace sept copies :**
  - Partage ;
  - Personnes ;
  - Préférences ;
  - À propos ;
  - Appliquer une version ;
  - Collage ;
  - Différences.
- **Anatomie :**
  - `<dialog>` + `showModal()` ;
  - `--dialog-bg`, `--radius-window`, `--shadow-dialog`, `::backdrop` `--backdrop` ;
  - en-tête : `<h2>` `--text-title-4`, bouton fermer de 44px ;
  - corps qui défile ;
  - pied facultatif : Annuler à gauche, action à droite.
- **Tailles :** `small` 380, `medium` 520, `large` 760, `wide` 1100px, plafonnées à
  `100vw − 32px`. Sous 560px, feuille en bas.
- **Focus initial** (APG *Dialog (Modal)* [S5]) :
  - le premier champ ;
  - sinon le bouton par défaut ;
  - pour un **contenu long à lire d'abord** (Différences, Aide, À propos), le **titre**
    (`tabindex="-1"`), pour ne pas faire défiler au-delà du début.
- **Fermeture :**
  - Échap = Annuler ;
  - Tab reste dans le dialogue ;
  - Entrée = action par défaut, sauf si elle est destructrice ;
  - à la fermeture, le focus revient au **déclencheur**.

  Écarts actuels, focus forcé vers l'éditeur : `Preferences.svelte:38`, `AboutDialog.svelte:62`.
- **D-1.** Un dialogue ne s'ouvre qu'à la demande de la personne, sauf une question pour un
  admin actif quand aucun bandeau n'est possible (présentation en cours).

### 4.9 Confirmation (`AlertDialog`)

- Réservée à l'irréversible pour les autres (P3).
- Titre-question ; une phrase de conséquence ; Annuler, puis le verbe (`.destructive`). Le focus
  initial est sur **Annuler** [S5].
- **Variante en ligne**, sous une ligne de liste :
  - le focus DOIT aller sur sa phrase (`tabindex="-1"`, `role="group"` nommé) ;
  - Échap la referme et rend le focus au bouton d'origine.

  Écart actuel : `PeopleDialog.svelte:160` ne gère ni le focus ni Échap.

### 4.10 Dialogue de préférences

- Calqué sur `AdwPreferencesDialog` [S1].
- Groupes titrés, listes encadrées, lignes de 52px minimum.
- La **portée** est écrite sous chaque groupe (« Pour ce projet, sur cet ordinateur »).
- Contenu : §5.13.

### 4.11 Bandeau (`Banner.svelte` + `BannerHost`, *à créer*)

- **Anatomie :**
  - une bande sous les barres, fond `--info-bg`, `--warning-bg` ou `--error-bg` (parent
    `--window-bg`) ;
  - une phrase ;
  - au plus deux boutons `.btn` ;
  - « +N autres » (L-8b).
- **BN-1.** Une action si la personne peut agir. Sinon, la phrase dit ce qui se passe tout seul
  et quand.
- **BN-2.** Le bandeau disparaît quand l'état cesse, jamais au bout d'un délai.
- **BN-3.** Sorte « question » : bouton suggéré, pour la réponse attendue.
- **Lecteur d'écran** ([ARIA22] [S11]) :
  - `BannerHost` garde **en permanence** dans le DOM une région `role="status"` et une
    région `role="alert"`, vides au départ ;
  - le texte y est **inséré ensuite**. Monter la région et son texte ensemble n'est pas annoncé
    de façon fiable (correction de la rév. 1, qui disait l'inverse) ;
  - `alert` seulement pour une question ou un blocage.

### 4.12 Toast (`Toast.svelte` + `ToastHost`, *à créer*)

- **Anatomie :**
  - une pilule `--toast-bg`, texte `--on-dark-fg` (14,51 / 12,03) ;
  - focus `--focus-ring-on-dark` ;
  - une phrase de 60 caractères au plus, **une seule action**, et fermer.

  Une seconde action éventuelle vit ailleurs, sur un accès durable : un menu ou la ligne
  concernée.
- **Durées :** 5s, ou 10s avec une action [S1]. La minuterie se suspend au survol et au focus.
- **TO-1. Annulation durable** (2.2.1). Ce qu'un toast propose d'annuler reste annulable **après**
  sa disparition :

  | Action | Annulation durable |
  |---|---|
  | modification de texte (dont une intégration depuis le disque) | Ctrl+Z ; Historique › « Annuler cette modification » |
  | fichier mis à la corbeille, renommé, déplacé | Ctrl+Z dans Fichiers ; Corbeille › « Restaurer » |
  | rôle changé, personne déconnectée | la commande inverse sur la ligne de la personne |
  | extension retirée | Préférences › Extensions › « Récemment retirées » › « Réinstaller » ; ses données ont été gardées (§4.29) |
  | thème changé | Préférences › Apparence |

  L'infobulle du bouton « Annuler » rappelle le raccourci (« Annuler · Ctrl+Z »), et l'Aide le
  dit. **Aucune phrase de plus dans le toast.**
- **TO-2.** Un toast ne pose jamais de question. Écart actuel : `ui/ExternalChangeToast.svelte:52-63`.
- **TO-3.** Le toast d'une extension est préfixé par son nom (« Diagramme : … »). Une extension
  émet au plus un toast toutes les 10s (architecture §15.13).
- **Lecteur d'écran :** régions persistantes (§4.11) ; `status`, ou `alert` pour un échec.
  L'action est atteignable par F6, qui passe par la zone des toasts.

### 4.13 Champs

- **Libellé :** toujours visible ; le *placeholder* ne sert que d'exemple.
- **Hauteurs :** 34px (barre) ou 44px (dialogue).
- **Erreur :**
  - message sous le champ, relié par `aria-describedby` ;
  - `aria-invalid` ;
  - la saisie est conservée ;
  - le message dit comment corriger (3.3.1, 3.3.3).
- **Recherche :** loupe, bouton effacer, options en bascules ; Échap efface, puis ferme.
  - **Exemption explicite du libellé visible**, comme le champ de recherche d'Adwaita
    (`GtkSearchEntry`, HIG *Search*) : la loupe, le texte indicatif « Rechercher… » et le titre
    de la zone (« Rechercher », barre « Rechercher dans le fichier ») tiennent lieu de libellé.
  - Le nom accessible est donné (`aria-label`, `ui/search/SearchField.svelte:55-69`). WCAG 3.3.2
    est satisfait par ces instructions persistantes, car la zone garde son titre.
- **Saisie redondante** (3.3.7) : le nom n'est demandé qu'une fois ; une invitation reste dans
  le champ après un échec.
- **Renommer en place** (F2, §5.6) : un champ dans la ligne. Entrée valide, Échap annule ; une
  erreur s'affiche dans une bulle sous le champ, sans perdre la saisie.

### 4.14 Avatar, pile, curseur distant, verrou

- **Avatar :**
  - 28 ou 32px, `--person-N` + `--person-N-ink` ;
  - liseré de la couleur de la surface **parente** ;
  - point `--success` = présente, l'état étant aussi écrit dans la liste.
- **Pile :**
  - au plus 4 avatars, puis « +N » ;
  - un bouton à contenu phrasé (`<span>`, pas de `<ul>`) ;
  - nom « Personnes : vous et 3 autres » ;
  - une infobulle liste les noms.
- **Curseur distant :**
  - trait `--person-N-line` ;
  - étiquette du nom en `--text-caption`, qui s'estompe après 3s et revient au survol ;
  - `aria-hidden`.
  - **Annonce :** « Paul a rejoint », « Paul est parti », une seule fois, en région `log`.
- **Zone verrouillée** (B3, ADR 0022) :
  - fond hachuré, `--hover-bg` à 45° ;
  - cadenas dans la marge ;
  - infobulle « Verrouillé par Marie » ;
  - taper dedans ne fait rien, et un toast propose « Proposer un changement ».

  Lecteur d'écran : la zone est annoncée à l'entrée du curseur (« Passage verrouillé par
  Marie »).

### 4.15 État de synchronisation

C'est **un bouton** dans la barre d'en-tête : pastille + mot. Il ouvre un popover d'état
(DOIT, L-8b).

| État | Mot | Pastille |
|---|---|---|
| seul | « Sur cet ordinateur » | `--fg-dim` |
| connexion | « Connexion… » | `--accent-bg`, pulsation coupée sous `reduce` |
| synchronisé | « Synchronisé » | `--success` |
| envoi en attente | « Envoi en cours (3) » | `--accent-bg` |
| hors ligne | « Hors ligne — rien n'est perdu » | `--warning` |
| accès suspendu | « Accès suspendu » | `--warning` |

- **Popover :**
  - avec qui l'on est relié, en clair (« directement », « par un relais ») ;
  - depuis quand ;
  - ce qui attend d'être envoyé ;
  - **tous** les messages en attente (L-8b) ;
  - « Détails techniques » replié.
- La barre d'état n'affiche que le **nombre de personnes**, sans pastille.

### 4.16 Panneau de gauche

- **Sélecteur :** Fichiers | Sommaire | Tâches | Liens (§4.5). Le dernier onglet est mémorisé.
- **Fichiers** (`Sidebar.svelte`) :
  - arbre APG *Tree View* [S5] à un arrêt de tabulation :
    - ↑ ↓ se déplacent ; → ouvre ou descend ; ← ferme ou remonte ; Origine et Fin ;
    - lettre tapée ;
    - Entrée ouvre le fichier ; F2 renomme ; Suppr met à la corbeille ;
    - Maj+F10 ou la touche Menu ouvrent le menu contextuel ;
  - `aria-level`, `aria-expanded`, `aria-selected` ; `aria-current` sur le fichier actif ;
  - lignes de 36px ; mention « principal » ;
  - fichier ouvert dans un onglet : graisse 600 ;
  - fichier actif : `--selected-bg`.
- **États d'une ligne :**
  - en réception : barre fine et « 12 sur 40 » ;
  - suppression demandée : nom barré et mention ;
  - corbeille : section repliée en bas ;
  - modifié dehors : point `--accent`.
- **Ouvrir un fichier depuis l'arbre (G-0, ouvert : le correctif est un lot séparé, à faire).**
  C'est le comportement attendu, qui servira de critère d'acceptation au correctif :
  - un **clic** ou **Entrée** sur un fichier appelle `open(nodeId)` (architecture MF-5) ;
  - si le fichier a déjà un onglet, cet onglet est activé ; sinon, un onglet s'ouvre à droite de
    l'onglet actif (§4.25) ;
  - le **focus va à l'éditeur** du fichier ouvert (contenu ouvert, §4 « Retour du focus ») ;
  - **Espace** ouvre le fichier mais **garde le focus dans l'arbre**, pour parcourir sans
    quitter la liste ;
  - un clic sur un dossier le replie ou le déplie ; il n'ouvre rien ;
  - un type sans éditeur ouvre sa vue (image, fiche d'un binaire, §4.28) ;
  - un fichier qui vient de disparaître : toast « « ch2.typ » n'existe plus », et la liste se met
    à jour ;
  - la ligne du fichier actif prend `aria-current="page"` et `--selected-bg`.

  Aujourd'hui, `App.svelte:197` ignore le chemin transmis par `ui/shell/Sidebar.svelte:49`. Le
  test e2e `G-0` (à écrire, `e2e/files.spec.ts`) clique deux fichiers, puis vérifie le texte de
  l'éditeur et l'onglet actif.
- **Glisser-déposer** : avec l'équivalent « Déplacer vers… » (2.5.7).
- **Sommaire** (B10) : arbre des titres du fichier actif. Le titre courant est surligné et
  suivi ; filtre par Ctrl+Maj+O ; repli et dépli ; même modèle APG.
- **Tâches** (B10) :
  - liste groupée par fichier, puis par type ;
  - filtres par type et par personne ;
  - compteur sur l'onglet (« Tâches, 3 ») et dans la barre d'état ;
  - case « fait », annulable.
- **Liens** (B7) : « Utilise » et « Utilisé par » pour le fichier actif ; un lien cassé a l'icône
  `warn` **et** le texte « introuvable ».
- **Vide :** §4.20.

### 4.17 Barre d'état (`StatusBar.svelte`)

- **Ordre :**
  - personnes ;
  - espace ;
  - problèmes ;
  - liens cassés ;
  - tâches ;
  - page ;
  - « Non enregistré » ;
  - position (bouton : Aller à la ligne) ;
  - mots ;
  - zoom ;
  - encodage (s'il y a un écart) ;
  - langage ;
  - Mise en page (Markdown).
- **ST-1.** Un élément cliquable est un bouton (zone active de 34px, infobulle) ; un élément
  non cliquable est du texte. Écart actuel : `.fb` sur du texte, `StatusBar.svelte:132-150`.
- **ST-2.** Texte en `--text-small`, couleur `--fg`, sans `opacity`.
- **ST-3.** En fenêtre très étroite, on garde problèmes, « Non enregistré » et position ; le
  reste passe dans ⋯.
- **Lecteur d'écran :** les problèmes sont annoncés quand ils passent de 0 à n ou de n à 0, pas
  à chaque frappe.

### 4.18 Aperçu (`preview/Preview.svelte`)

| État | Rendu | Texte et action |
|---|---|---|
| démarrage | squelette | « Préparation de l'aperçu… » |
| à jour | pages, papier `--page-bg` | — |
| mise à jour > 300ms | pages actuelles + indicateur dans la barre | « Mise à jour… » |
| erreurs | dernière version correcte + bandeau local | « … l'aperçu montre la dernière version correcte. **Voir les problèmes** » |
| échec | écran vide | selon le cas, **toujours une action** (« Voir les problèmes », « Réessayer ») |
| page indisponible | case | « Page indisponible » |
| Markdown, mise en page CSS | colonne `--text-document` dans une iframe isolée (B8) | — |

- **Clavier :**
  - la zone est focalisable ;
  - ↑ ↓ et Page↑ Page↓ défilent ;
  - Ctrl+Alt+J saute de la source à l'aperçu ;
  - Entrée sur une page saute à la source.
- **Lecteur d'écran :** chaque page est une image « Page n sur N ». Le PDF exporté est la
  version accessible.

### 4.19 Tableur (`ui/sheet/*`)

- Grille APG *Grid* (en place) : flèches, Ctrl+flèches, Page, F2 ou Entrée pour éditer, Échap,
  Maj+F10.
- Barre : F-1.
- Ses toasts et ses bandeaux passent par `ToastHost` et `BannerHost` (L-8c, L-9).
- Mêmes jetons ; aucun style propre.

### 4.20 Écran vide (`StatusPage`, *à créer*)

`AdwStatusPage` [S1] : icône de 64px, titre `--text-title-2`, une phrase qui dit quoi faire, au
plus un bouton suggéré.

| Où | Titre | Action |
|---|---|---|
| Historique | « Pas encore de modification » | — |
| Activité | « Rien de neuf » | — |
| Rechercher, sans résultat | « Aucun résultat pour « mot » » | « Chercher dans tout le projet » |
| Sommaire | « Pas de titre dans ce fichier » | « Insérer un titre » |
| Tâches | « Aucune tâche » | « Comment écrire une tâche » (Aide) |
| Liens | « Ce fichier n'en utilise aucun autre » | — |
| Personnes, seule | « Personne d'autre n'a encore rejoint ce projet » | « Partager » |
| Fichiers, projet vide | « Ce projet est vide » | « Nouveau fichier » |
| Aucun onglet ouvert | « Aucun fichier ouvert » | « Ouvrir le fichier principal » |

### 4.21 Attente

- **Indicateur d'attente** (`AdwSpinner`) : seulement après 300ms ; un seul par zone.
- **Barre de progression** : dès qu'une quantité est connue, et toujours au-delà de 30s.
  `role="progressbar"`, avec un texte (« 12 sur 40 fichiers, 3,2 Mo »).
- **W-1.** Au-delà de 3s, l'attente dit ce qui se passe et ce qu'on peut faire en attendant.

### 4.22 Historique et différences

- **Tiroir › Historique :**
  - filtres (sélecteurs) ;
  - interrupteurs (« Qui a écrit quoi », « Variantes abandonnées ») ;
  - **liste en `listbox`** (APG *Listbox*), à un arrêt de tabulation : ↑ ↓ **déplacent le focus
    sur l'option**, ou `aria-activedescendant` si le focus reste sur la liste ; Entrée = Voir ;
  - actions de ligne au focus et au survol.

  Écart actuel : les flèches changent la sélection sans déplacer le focus ni renseigner
  `aria-activedescendant` (`HistoryDrawer.svelte:143`).
- **Ligne :** avatar, phrase en langage naturel, heure relative. Sur sélection, le texte
  secondaire passe en `--fg` (C-SEL).
- **Différences** (`DiffView`, dialogue `wide`) :
  - focus initial sur le titre ;
  - « En ligne | Côte à côte » en groupe de bascules (§4.5) ;
  - F7 / Maj+F7 ;
  - ajout et suppression selon C-DIFF ;
  - résumé en tête.
- **Bande de modifications** (`HistoryBars`) : bandeau par `BannerHost` ; « Colorer par » en
  groupe de bascules.
- **Revenir à ce moment :** aperçu, puis toast « Annuler ». Une confirmation seulement pour
  tout le document (§4.9).
- **Variantes :** le mot « variante » est celui de la demande T11 (2026-10-25). Le glossaire dit
  « brouillon (v2) » : *amender le glossaire, « variante » remplace « brouillon », à confirmer
  par l'utilisatrice*.
  - Réunir une variante au texte se dit **« Intégrer la variante »**, jamais « fusionner ».
  - Écart actuel : `fr.json:549`, `:618`, `:619`, `:622`, `:656`.

### 4.23 Vues de présentation

- **Vue public :** `--stage-bg`, rien d'autre. « Présentateur déconnecté » en `--text-small`
  sur `--stage-overlay` (21:1).
- **Vue présentateur :**
  - courante et suivante, notes en `--text-document` ;
  - minuteur en `tabular-nums` ;
  - outils ;
  - **frontières ajustables** (§3.3, RSZ-08) ;
  - focus `--focus-ring-on-dark` sur la scène ;
  - barre d'outils : F-1.
- **Clavier :**
  - → / Espace / Page↓ ; ← / Page↑ ; Origine et Fin ;
  - B (écran noir), W (écran blanc), L (pointeur laser), uniquement dans la vue de présentation
    (`present/keys.ts:61-69`) ;
  - un numéro + Entrée ;
  - Échap.
- **Lecteur d'écran :** « Diapositive n sur N : titre », en `status`.
- **Questions** (prise de main) : bandeau-question dans la vue présentateur (L-8).

### 4.24 Palette de commandes (*à créer*)

- **Raccourci :** **Ctrl+Maj+P**, et le menu principal › « Rechercher une commande… »
  (recommandation Q1, *à confirmer*). Ctrl+K reste le lien.
- **Amendement de la règle 3 de `ux-principles.md`**, *à confirmer* : « Palette (Ctrl/Cmd+Maj+P,
  ou menu › Rechercher une commande) qui affiche le raccourci ».
- **Anatomie :** dialogue `medium`, en haut ; un champ ; une liste (nom, raccourci, groupe).
- **ARIA** (APG *Combobox* avec `listbox` [S5]) :
  - le champ : `role="combobox"`, `aria-expanded`, `aria-controls` vers la liste,
    `aria-autocomplete="list"`, `aria-activedescendant` ;
  - la liste : `role="listbox"` ; les éléments : `role="option"` et `aria-selected`.
- **Clavier :** ↑ ↓, Entrée exécute, Échap ferme.
- **Lecteur d'écran :** le nombre de résultats est annoncé en `status`.
- La liste vient du **registre unique des commandes** (§8.2), avec ses raccourcis.
- « Aller au fichier » (Ctrl+P) en est un mode : le préfixe est implicite, la liste contient
  les fichiers.

### 4.25 Onglets de fichiers (`FileTabs`, *à créer*)

Le modèle n'existe pas aujourd'hui : un seul fichier courant (`workspace/workspace.svelte.ts:120`).
Il prend la place des « documents ouverts » de l'architecture (MF-5 ; `AD-47` décide s'il y a
des onglets, et cette section est la proposition de design pour cette décision).

- **Modèle :**
  - une liste ordonnée d'onglets **par projet, sur cet ordinateur**, jamais synchronisée ;
  - chaque onglet garde chemin, position, défilement et vue ;
  - un seul onglet actif ;
  - un fichier = au plus un onglet.
- **Ouvrir :**
  - clic ou Entrée dans Fichiers (§4.16), Ctrl+P, un lien, un résultat, une tâche, un
    problème ;
  - l'onglet s'ouvre à droite de l'onglet actif et devient actif.
- **Fermer :**
  - Ctrl+W, bouton ×, clic du milieu, ou Suppr sur l'onglet focalisé ;
  - aucune question (SAV-03) ;
  - le focus va à l'onglet voisin de droite, sinon de gauche ;
  - **dernier onglet fermé** : l'écran vide « Aucun fichier ouvert » s'affiche, et le focus va
    sur son bouton « Ouvrir le fichier principal ».
- **Renommé ou déplacé** : l'onglet suit le fichier. **Supprimé** : l'onglet reste, en italique,
  avec un bandeau « Ce fichier est dans la corbeille · Restaurer ».
- **Anatomie :**
  - barre de `--tab-height` sur `--headerbar-bg` ;
  - onglet : `FileIcon`, nom, point « non enregistré », × visible au survol **et au focus** ;
  - onglet actif : `--selected-bg` et un filet `--accent` en bas ;
  - débordement : défilement de la barre, plus un menu « Fichiers ouverts ».
- **ARIA et clavier** (APG *Tabs*, **activation manuelle**, car ouvrir un fichier charge un
  document [S5]) :
  - `role="tablist"` (« Fichiers ouverts ») ;
  - chaque onglet a `role="tab"`, `aria-selected="true"` s'il est actif, et `aria-controls` vers
    **l'unique** `role="tabpanel"` de la zone d'édition. Ce panneau porte
    `aria-labelledby` = l'onglet actif ;
  - un seul arrêt de tabulation (tabindex itinérant) ; ← → Origine Fin **déplacent le focus**
    sans activer ; **Entrée ou Espace activent** ;
  - Ctrl+Page↓ / Page↑ et Ctrl+Tab **activent** le suivant ou le précédent depuis n'importe où ;
  - Ctrl+Maj+Page↓ / Page↑ déplacent l'onglet, en alternative au glisser (2.5.7) ;
  - Alt+1…9 activent l'onglet n.
- **Lecteur d'écran :** « main.typ, Typst, non enregistré, onglet 2 sur 4 ».

### 4.26 Aide (`Help`, *à créer*)

- Menu › **Aide** (F1) ouvre un dialogue `large` en `--text-document`.
- Le guide est **embarqué**, hors ligne. Une page par tâche de référence : créer, partager,
  relire, présenter, exporter. Plus « Écrire en Typst » et « Écrire en Markdown » (syntaxe en
  regard du bouton de la barre), et le glossaire de l'interface.
- Menu › **Raccourcis clavier** (Ctrl+?) : une fenêtre de raccourcis, comme
  `GtkShortcutsWindow`, générée depuis le registre (§8.2), avec une recherche.
- Chaque écran vide et chaque message d'erreur PEUT pointer vers sa page d'aide (« En savoir
  plus »).

### 4.27 Commentaires — hors du périmètre de N17

Il n'y a ni modèle de données, ni plan, ni code au-delà d'un texte vide. Ancres, fils,
résolution et @mentions demandent une décision d'architecture : N18 ou un plan dédié, puis une
ADR.

N17 fixe seulement :
- l'onglet est masqué tant que la fonction n'existe pas (L-2b) ;
- le jour venu, les commentaires seront un onglet du tiroir, réutiliseront l'avatar, le champ,
  la liste APG et le toast, et le texte commenté sera surligné selon C-PERSON.

La tâche « relire » s'appuie en attendant sur l'Historique, les versions et les différences.
Question Q8.

### 4.28 Vues par type de fichier, et code (N20)

Inkwell **n'est pas un éditeur pour développeurs**. Les quatre types principaux ont chacun leur
vue. Le code des autres langages se **lit** agréablement, sans fonctions d'atelier de
programmation : pas de complétion, de diagnostic, de pliage intelligent ni de terminal.

**Les types et leurs vues**

| Type | Extensions | Vue par défaut | Autres vues | Édition | Icône (marque) |
|---|---|---|---|---|---|
| Typst | `.typ` | éditeur + barre Typst + aperçu en pages | Présenter | oui | `--mark-typst` |
| Markdown | `.md`, `.markdown` | éditeur + barre Markdown + aperçu (mise en page Typst ou CSS, B8) | — | oui | `--mark-markdown` |
| Tableau | `.csv`, `.tsv` | tableur (§4.19) | « Afficher le texte » : texte avec **colonnes repérées** (FT-3) | oui | `--mark-csv` |
| Texte | `.txt`, sans extension mais texte | éditeur nu, sans barre ni aperçu ; un `.txt` tabulaire se voit proposer « Ouvrir comme tableau » | — | oui | neutre (`text`) |
| Bibliographie | `.bib` | éditeur nu, coloration | — | oui | `--mark-bib` |
| **Code** | `.py .js .mjs .ts .json .go .sh .bash .yaml .yml .toml .css .html .xml .sql .rs .c .h .java …` et *shebang* | **lecture** colorée (FT-4) | « Modifier » | sur demande | `--mark-code` |
| Image | `.png .jpg .jpeg .gif .webp .svg` | visionneuse, ajustée à la fenêtre ; zoom | — | non | `--mark-image` |
| PDF | `.pdf` | fiche du fichier (FT-6) ; « Ouvrir avec l'application du système » | — | non | `--mark-pdf` |
| Données, polices, autres binaires | `.ttf .otf .woff2 .zip …` | fiche du fichier (FT-6) | — | non | `--mark-data` ou neutre |

- **FT-1. Icônes** (I-1, T11 UI-08, cohérentes avec les icônes symboliques de fichiers
  d'Adwaita).
  - Un contour de document neutre, en `currentColor`, et **une seule marque intérieure
    colorée** par famille.
  - Le code a **une marque unique `</>`** pour tous les langages, sur `--mark-code` (neutre,
    5,53:1 / 7,35:1) : pas d'arc-en-ciel de langages, et la couleur n'est jamais le seul indice
    (A-3).
  - Le langage est écrit dans la description de la ligne et dans la barre d'état (« Python »).
  - Contraste minimal de toutes les marques : 3,23:1 (`tokens.json`).
- **FT-2. Détection du langage.**
  1. **Par l'extension**, sans tenir compte de la casse, avec la table
     `LanguageDescription.matchFilename` de `@codemirror/language-data` [S13].
  2. **Sans extension** : par la ligne *shebang* (`#!/bin/sh`, `#!/usr/bin/env python3`).
  3. **Dans un bloc de code** Markdown (```` ```python ````) : par la chaîne d'info, avec
     `matchLanguageName(…, fuzzy)` [S13] (option `codeLanguages` de `@codemirror/lang-markdown`).
     Dans un bloc brut Typst (```` ```python ````) : de même, si la grammaire Typst expose la
     langue du bloc ; sinon, le bloc reste en `--hl-raw`.
  4. **Inconnu** : texte sans coloration, jamais d'erreur.

  On ne devine jamais un langage par le contenu (statistiques) : c'est imprévisible, et deux
  personnes pourraient voir deux colorations différentes.
- **FT-3. CSV en vue texte.** La vue texte est un **repli** : la lecture se fait dans le tableur.
  Dans le texte, le séparateur est en `--hl-marker` et la ligne d'en-tête en gras, rien de plus.
  Simplification de la rév. 2, à la demande de Fable : plus de teinte par colonne ni
  d'infobulle.
- **FT-4. Palette de coloration du code.** On **réutilise les dix jetons `--hl-*`** existants.
  Ils sont tous ≥ 4,77:1 sur la vue, en clair comme en sombre, et déjà testés. On n'ajoute
  aucune couleur.

  | Catégorie (étiquettes Lezer [S14]) | Jeton |
  |---|---|
  | mot-clé, opérateur de contrôle, balise | `--hl-keyword` |
  | chaîne, expression régulière | `--hl-string` |
  | nombre, booléen, `null`, constante | `--hl-number` |
  | commentaire | `--hl-comment` (+ italique) |
  | fonction, méthode | `--hl-function` |
  | nom de propriété, clé JSON ou YAML, attribut | `--hl-heading` |
  | type, classe, espace de noms | `--hl-label` |
  | méta, annotation, décorateur, directive | `--hl-raw` |
  | lien, URL | `--hl-link` + soulignement |
  | invalide | `--hl-keyword` + soulignement ondulé |
  | variable, ponctuation, reste | `--fg` |

  Un même `HighlightStyle` (`editor/highlight.ts`) sert aux fichiers de code, aux blocs des
  fichiers Markdown et Typst, à la vue des différences et à l'aperçu HTML.
- **FT-5. Lecture d'abord.** Un fichier de code s'ouvre **en lecture** :
  - une barre secondaire affiche « Python · lecture » et le bouton « Modifier » ;
  - en lecture : numéros de ligne, retour à la ligne au choix, recherche, sélection et copie ;
  - « Modifier » passe en édition **texte simple**, sans barre de mise en forme. Le choix est
    mémorisé pour ce fichier, sur cet ordinateur. Les modifications sont collaboratives et
    suivies par l'historique comme les autres ;
  - aucune complétion, aucun diagnostic, aucune indentation intelligente autre que garder
    l'indentation de la ligne précédente.
- **FT-6. Inconnu ou binaire.**
  - Un fichier est **texte** s'il se décode (UTF-8, ou l'encodage détecté par A3) sans octet
    nul. Il s'ouvre alors comme du texte simple **en lecture**, avec « Modifier ».
  - Un fichier **binaire** n'est jamais affiché comme du texte. Il s'ouvre sur une **fiche** :
    - un écran vide (§4.20) avec la grande icône, le nom, le type en clair (« Police »,
      « Archive »), la taille et la date ;
    - les actions « Ouvrir avec l'application du système », « Afficher dans le dossier » et
      « Renommer » ;
    - les binaires voyagent par le magasin de blocs (ADR 0023) ;
    - au-delà de 25 Mo, la fiche dit : « Ce fichier est trop lourd pour être partagé (25 Mo au
      plus). Il reste sur cet ordinateur. »
  - Un fichier qui devient binaire sur le disque : bandeau de §5.16.
- **FT-7. Lecteur d'écran.**
  - La barre secondaire annonce « Python, lecture seule » ; l'éditeur porte `aria-readonly`.
  - La fiche d'un binaire est un `region` nommé par le nom du fichier.

**Approche technique recommandée, la plus légère.**

| Option | Ce qu'elle apporte | Coût | Verdict |
|---|---|---|---|
| `@codemirror/language-data` [S13] | Une table de `LanguageDescription` : noms, alias, extensions, et un `load()` **paresseux** par langage, en `import()` dynamique. Vite en fait des morceaux locaux du paquet : rien ne se télécharge à l'exécution, et la CSP est respectée. | Le code d'un langage n'est chargé qu'à sa première ouverture. | **Retenu** |
| Grammaires Lezer (`@codemirror/lang-*`) | Analyse incrémentale ; les étiquettes standard de `@lezer/highlight` alimentent FT-4. | Déjà apportées par `language-data`. | retenu, à travers `language-data` |
| `@codemirror/legacy-modes` (`StreamLanguage`) | Shell, TOML, et d'autres langages sans grammaire Lezer. | Petit, chargé de la même façon. | retenu pour ces langages seulement |
| Jetons sémantiques de tinymist (LSP) | Coloration Typst plus fine (types et symboles résolus). | Un aller-retour vers le *sidecar* à chaque modification, de la latence, et un second système de coloration. | **non retenu en v1** ; à réévaluer si la coloration Lezer de Typst se révèle insuffisante en test |
| `codemirror-lang-typst` (grammaire Lezer, déjà employée, sans WASM) | Coloration Typst actuelle. | Déjà là. | conservé |

- **Couverture par priorité, sous budget mesuré**, plutôt qu'un nombre fixe de langages :
  - **priorité 1**, toujours dans la liste fermée de l'architecture (`AD-54`) : JSON, YAML,
    Python, JavaScript/TypeScript, Shell, HTML/CSS, Go, SQL, TOML, XML ;
  - **priorité 2** : Rust, C/C++, Java ; le reste n'est ajouté que sous le budget.
  - **Budget** : un langage coûte au plus 60 Ko compressés, chargés à la première ouverture.
    La première coloration de la zone visible arrive ≤ 100ms après l'ouverture. Une mise à jour
    coûte ≤ 5ms (architecture §15.4, point Coloration).
  - Un langage hors de la liste s'affiche sans coloration.
- La coloration des blocs passe par `markdown({ codeLanguages })`. Pour Typst, il faut vérifier
  que `codemirror-lang-typst` 0.6 permet l'imbrication (`parseMixed`). Sinon, les blocs restent
  en `--hl-raw` : *à vérifier en N18*.

### 4.29 Extensions : gérer et consentir (N21)

- **Le mot de l'interface est « extension ».** Jamais « plugin », « greffon », « module »,
  « WASI », « bac à sable » ni « capacité » (P5).
- Le code parle de « greffon » pour un module WebAssembly, et d'« extension intégrée » pour une
  extension du cœur (architecture §15.6). L'exécution, l'API et les capacités relèvent de N18
  (architecture §15) ; N17 fixe ce que la personne voit, **sans rien y contredire**.

**Gérer.** Préférences › page **Extensions**, une page d'`AdwPreferencesDialog` (K-2), sur le
modèle de l'application *Extensions* de GNOME [S15].

- **En tête :**
  - « Installer depuis un fichier… » ;
  - « Redémarrer sans extensions » (§4.33).
- **Trois groupes :**
  - « Extensions de ce projet » (§4.32) ;
  - « Mes extensions » ;
  - « Intégrées » : extensions du cœur, désactivables sauf les essentielles, qui ne peuvent pas
    être retirées.
- **Une ligne par extension** (`AdwExpanderRow`) :
  - icône symbolique, nom, auteur, une phrase, **interrupteur** Activer ;
  - une fois dépliée :
    - version et provenance ;
    - « Ce qu'elle peut faire », chaque autorisation avec son interrupteur ;
    - raccourcis et conflits (§4.33) ;
    - « Mettre à jour depuis un fichier… » ;
    - « Retirer » ;
    - « Effacer ses données… ».

**États du gestionnaire** (Codex R2-M6) :

| État | Rendu | Sortie |
|---|---|---|
| aucune extension installée | écran vide « Aucune extension pour l'instant » + « Installer depuis un fichier… » | — |
| installation en cours | ligne provisoire, barre de progression, « Annuler » | annuler : rien n'est gardé |
| mise à jour en cours | la ligne garde l'ancienne version active, avec un indicateur d'attente | annulable |
| **paquet abîmé** ou signature invalide | dialogue « Ce fichier n'est pas une extension valide, ou il a été modifié. » | rien n'est installé |
| **paquet incompatible, trop récent** (majeure d'API au-delà, PV-1) | « Faite pour une version plus récente d'Inkwell. Mettez Inkwell à jour pour l'utiliser. » + « Détails techniques » : API demandée, API d'Inkwell | rien n'est installé |
| **paquet incompatible, trop ancien** (majeure hors de la fenêtre N et N−1, PV-4) | « Cette extension est faite pour une version plus ancienne d'Inkwell ; demandez une mise à jour à son auteur. » + « Détails techniques » | rien n'est installé ; une extension déjà installée passe en « désactivée », avec la même phrase |
| échec d'une mise à jour | « La mise à jour a échoué ; la version 1.2 reste installée. · Détails » | ancienne version intacte |
| active | interrupteur activé | — |
| désactivée | interrupteur désactivé, nom en `--fg-dim` | réactiver |
| **en pause pour lenteur** (ND-5) | « Mise en pause : trop lente. » + « Reprendre » | — |
| arrêtée après des plantages | « Désactivée après 3 arrêts en 5 minutes. » + « Réactiver » | — |
| autorisation retirée | la ligne dit ce qui ne marche plus. Rendre l'autorisation rétablit la fonction **sans redémarrer**, et un toast le confirme. | — |
| conflit de raccourci | badge « Conflit » ; le raccourci de l'extension reste **inactif** jusqu'au choix (§4.33) | — |

- **Plantage en usage :** toast « Diagramme s'est arrêtée. Votre texte n'est pas touché. ·
  Relancer ». Une extension ne bloque jamais la frappe (PERF-1, ND-5).
- **Retirer** (Codex R2-M4) :
  - retire le code ;
  - **garde ses données** (réglages, magasin privé), qu'une réinstallation retrouve ;
  - toast « Diagramme retirée · Annuler » ;
  - « Récemment retirées », en bas de la page, garde « Réinstaller » **pendant la session** :
    le paquet est gardé jusqu'à la fermeture d'Inkwell. Après, on réinstalle depuis le fichier
    d'origine, et les données gardées sont retrouvées (Codex R3).
- **Effacer ses données…** est un geste à part : une confirmation (§4.9), « Effacer les données
  de Diagramme ? Ses réglages et ce qu'elle a gardé sur cet ordinateur seront supprimés. Vos
  documents ne sont pas touchés. » Elle n'efface jamais un document du projet.

**Consentement** (Nielsen 2, 3, 5 ; HIG *Dialogs*). C'est un `AlertDialog` **du cœur** : zone
protégée, inimitable (§4.33).
- Titre « Installer « Diagramme » ? » ; l'auteur ; la provenance.
- **La liste de ce qu'elle pourra faire**, avec les **phrases de l'architecture** (§15.5) :
  - « lire vos documents » ;
  - « proposer des modifications » ;
  - « créer des fichiers » ;
  - « lire les fichiers du projet » ;
  - « compiler le projet » ;
  - « enregistrer un fichier que vous choisissez » ;
  - « ouvrir un fichier que vous choisissez » ;
  - « voir l'historique » ;
  - « voir les personnes du projet » ;
  - « afficher un dessin libre » ;
  - « se connecter à : nuage.example » ;
  - « garder une copie chiffrée de vos projets chez nuage.example » ;
  - « copier vos fichiers lisibles dans … ».

  Les autorisations qui sortent de l'ordinateur (réseau, stockage, miroir) sont **mises en
  avant** (icône `warn`, `--warning`), avec ce qu'elles font **automatiquement** : « à chaque
  enregistrement, en arrière-plan ».
- La mémoire demandée, si elle dépasse 64 Mio : « peut utiliser jusqu'à 256 Mo de mémoire ».
- Boutons : « Annuler » (focus initial) et « Installer ».
- **Mise à jour** : les **nouvelles** autorisations sont en tête, et « Tout voir » déplie la liste
  complète (Codex R2-M5).
- Toute autorisation est **révocable** depuis la ligne dépliée.

**Stockage distant, miroir, cycle de vie** (N22, Codex R2-M5 ; architecture §6.7, §15.4,
§15.9) :
- **Le dossier local reste la vérité.** Un stockage distant ne garde qu'une **copie chiffrée** ;
  un miroir, une copie lisible dans un dossier choisi.
- Sans l'extension, ou si elle échoue, **le projet s'ouvre et s'édite normalement** depuis cet
  ordinateur. La promesse ND-7 tient.
- L'état de la copie est dans le popover d'état (§4.15) : « Copie chez nuage.example : il y a
  3 min ».
- En cas d'échec, bandeau d'information (niveau 4) : « La copie chez nuage.example a échoué. Vos
  projets restent sur cet ordinateur. · Réessayer ».
- « **Arrêter la copie** » est sur la ligne de l'extension. La copie distante n'est pas effacée
  sans un second geste confirmé.
- **Migration** vers un autre stockage : « Copier vers… » montre une progression. L'ancien
  stockage reste en place jusqu'à la fin, vérifiée, de la copie.
- **Actions au démarrage et à l'arrêt** : elles ne montrent rien, sauf un effet visible. Elles
  sont alors listées dans l'onglet Activité, préfixées du nom (« Au démarrage, Sauvegarde a
  copié 3 projets »).
  - L'arrêt ne pose jamais de question. Il attend **au plus 1s au total**, pour toutes les extensions en parallèle (architecture §15.9).
  - Ce qui n'a pas fini reprend au lancement suivant.

### 4.30 Bouton Extensions (menu à pièce de puzzle)

Inspiré du bouton unifié de Firefox (version 109) [S16] ; architecture §15.13.

- **Place :** barre d'en-tête, à droite, avant le menu principal. Icône `puzzle`, nom accessible
  « Extensions », infobulle. Il n'apparaît que si une extension active offre une action.
- **Ce qu'il ouvre :** un **popover** (`role="dialog"`, « Extensions »), et non un menu. Chaque
  ligne porte **deux** commandes, et un menu APG n'en admet qu'une par élément (Codex R3-M3).
  - Une **liste** (`role="list"`) ; chaque ligne a :
    - un **bouton d'action** : icône, nom, badge éventuel de 4 caractères au plus. Il lance
      l'action, ou ouvre son popover déclaratif ;
    - un **bouton bascule** « Épingler Diagramme » (`aria-pressed`, icône punaise). Il ne lance
      pas l'action.
  - En bas : « Gérer les extensions… ».
- **Clavier :**
  - Entrée, Espace ou ↓ sur le bouton ouvrent le popover, et le focus va au premier bouton
    d'action ;
  - ↑ ↓ passent d'une ligne à l'autre, sur le même genre de bouton ; ← → passent de l'action à
    la punaise de la même ligne ;
  - Tab et Maj+Tab parcourent aussi tous les boutons, dans l'ordre ;
  - Échap ferme, et le focus revient au bouton Extensions ;
  - lancer une action ferme le popover ; basculer une punaise le laisse ouvert, et annonce
    « Épinglé » ou « Désépinglé ».
- **C'est la personne qui épingle**, jamais l'extension (ND-1) ; par ordinateur, ou par projet
  pour une extension de projet ; jamais imposé par un admin.
- **Six épinglées au plus** (architecture §15.13) : la septième punaise est désactivée, avec
  l'infobulle « 6 extensions au plus dans la barre ; désépinglez-en une ». Selon la largeur
  (H-2), 6, 2 ou 0 sont visibles ; les autres restent dans le popover.
- **Bouton épinglé** :
  - `.fb`, icône symbolique de l'extension, infobulle « Nom (extension « X ») », badge ;
  - menu contextuel : « Désépingler », « Déplacer à gauche », « Déplacer à droite » (2.5.7),
    « Gérer… ».
- **États :** une extension désactivée ou en pause disparaît du popover et de la barre. Une
  extension plantée garde sa ligne, avec `warn` et « Arrêtée · Relancer ».

### 4.31 Contributions : où une extension s'affiche, et comment on la reconnaît

- **Le modèle est déclaratif** (architecture §15.13). Une contribution =
  `{commande, emplacement, groupe, ordre, quand}`.
  - L'hôte la **dessine lui-même, en composants Adwaita** (`ui.view`), sans appeler
    l'extension pour afficher.
  - Les groupes du cœur gardent leur place. Les éléments d'une extension vont dans son groupe,
    après ceux du cœur.
  - La personne peut réordonner ou masquer un élément d'extension, jamais un élément protégé.
- **Attribution :** chaque point d'affichage a sa règle, pour que l'on sache toujours ce qui
  vient d'une extension (Fable R2-B2). La pièce `puzzle` en `--mark-extension` est le repère
  commun. On ne l'ajoute que là où c'est lisible ; ailleurs, le **nom** suffit.
- **Points d'extension**, alignés **ligne pour ligne** sur l'architecture §15.4 (et §15.13 pour
  les surfaces) :

| # | Point (§15.4) | Où cela s'affiche | Attribution visible | Désactivée, en pause, plantée | Interdit |
|---|---|---|---|---|---|
| 1 | Commande | palette, menus, raccourcis | palette : colonne « groupe » = nom de l'extension | commande retirée de la palette | prendre un raccourci du cœur (ND-3) |
| 2 | Menu, palette, raccourci | menu principal (groupes `extensions`, `insert`, `export`, `view`) ; menus contextuels `editor/`, `files/`, `tabs/`, `preview/`, `people/`, `link/context` | groupe séparé par un filet ; au-delà de 8 entrées par menu, un sous-menu « Extensions » ; infobulle « par X » | entrées retirées | groupes `project`, `people`, `about` ; entrées de droits et de confiance du menu d'une personne ; « Supprimer », « Restaurer » |
| 3 | Barre d'outils par type | groupe « Extensions » en fin de barre, avant Insérer, puis dans le débordement ⋯ | infobulle « Nom (extension « X ») » | boutons retirés | Annuler, Rétablir, état d'enregistrement |
| 4 | Panneau latéral, onglet du tiroir | **les deux sélecteurs**, selon L-2 (la structure à gauche, le temps et les autres à droite), après les onglets du cœur | icône de l'onglet + nom ; en tête du panneau, une ligne `--text-caption` : pièce + « Extension « X » » | le panneau affiche « X est désactivée · Réactiver » | onglet Personnes ; Fichiers comme onglet par défaut |
| 5 | Barre d'état, bandeau, toast | barre d'état : groupe `extensions` à droite, au plus 3 éléments visibles, puis ⋯ ; bandeau et toast par `BannerHost` / `ToastHost` | élément : infobulle « X : … » ; toast et bandeau préfixés « X : » | éléments retirés | état de synchronisation, d'enregistrement, présence ; bandeau-question de confiance ; une question dans un toast (TO-2) ; plus d'un bandeau d'extension à la fois |
| 6 | Type de fichier : reconnaissance | icône et nom de type dans Fichiers | description de la ligne : « Type fourni par X » | repli sur « fichier inconnu » (FT-6) | changer l'icône des quatre types principaux |
| 7 | Vue, éditeur d'un type | la zone d'édition, pour ce type ; « Ouvrir avec… » pour un type déjà géré | barre secondaire « Affiché par X · Ouvrir avec… » | repli MF-14 : texte, ou fiche du fichier (FT-6) | remplacer la vue d'un des quatre types principaux sans le geste « Ouvrir avec… » |
| 8 | Coloration | dans l'éditeur, avec les jetons `--hl-*` seulement | aucune (coloration standard) | texte sans coloration | couleurs hors jetons |
| 9 | Complétion, survol, aide de signature | liste de complétion ; carte de survol | section titrée par le nom de l'extension ; complétion : colonne « source » | rien | la carte d'une personne, la carte de divergence |
| 10 | Diagnostics, décorations, gouttières, widgets en ligne | soulignements, colonne de gouttière (2 au plus, à droite de celles du cœur), widgets sous une ligne | gouttière et soulignement : l'infobulle commence par « X : » ; widget en ligne : filet gauche `--mark-extension`, et nom accessible suffixé « (extension X) » | décorations retirées | marques des autres personnes, zones verrouillées, divergences, bande des modifications |
| 11 | Transformation de document | commande, ou proposition avant collage | la modification porte « via X » dans l'Historique (ND-8) | commande retirée | appliquer sans le geste de la personne (PP-2) |
| 12 | Affichage de l'aperçu | surcouche des pages | survol de la surcouche : « X » | surcouche retirée | modifier le rendu Typst (X-8) |
| 13 | Exportateur | Exporter… : un format de plus | « HTML · par X » dans la liste des formats | format retiré | remplacer le PDF par défaut sans choix explicite |
| 14 | Importateur | « Importer… » : un format de plus | « par X » | format retiré | — |
| 15 | Thème, jeu d'icônes | Préférences › Apparence (§2.10) | nom du thème, « par X » | retour à Adwaita | jetons verrouillés ; icônes des zones protégées |
| 16 | Page de réglages | une page par extension dans Préférences, rangée par ordre alphabétique après les pages du cœur | titre de la page = nom ; sous-titre « Extension » ; icône de l'extension | « X est désactivée · Réactiver » | pages Réseau, Confidentialité, Extensions |
| 17 | Diff et historique | une vue de plus dans Différences (groupe de bascules) | nom de la vue + « par X » | vue retirée | — |
| 18 | Stockage distant, miroir | ligne de l'extension ; popover d'état | « Copie chez … par X » | §4.29 : le local reste la vérité | voir du clair (règle d'exclusion, §15.5) |
| 19 | Cycle de vie, événements | rien, sauf l'Activité | Activité : « Au démarrage, X a … » | — | une question à l'arrêt |
| 20 | Rendu de diagramme | §4.34 | pied du rendu : « Dessiné par X » | dernier rendu partagé | exécuter du code hors du cadre isolé |
| 21 | Fenêtres, dialogues, menus contextuels, popovers, écran d'accueil, glisser-déposer | fenêtre : titre « X — Inkwell » et une bande d'attribution ; dialogue : **dans** la fenêtre, avec une bande « Extension « X » » sous le titre ; popover : attaché à un bouton de l'extension ; dialogue « Nouveau projet » : au plus 4 cartes « Démarrer avec », sous la galerie de modèles (§5.19), badge « par X » ; dépôt : si plusieurs extensions acceptent, un popover « Déposer avec… » | toujours la bande ou le badge | fenêtres fermées, cartes retirées | une fenêtre qui prend le focus sans geste ; les dialogues d'accord, de droits et d'invitation ; « Nouveau projet », « Ouvrir », « Rejoindre » |

- **Correspondance :** chaque ligne de §15.4 a sa ligne ici, dans le même ordre. Seuls
  « Stockage distant » et « Miroir lisible » sont réunis (ligne 18). Les surfaces de §15.13 sont
  dans les lignes 2, 4, 5, 10 et 21. Une contradiction avec l'architecture se règle en faveur
  de §15, puis cette table est corrigée.
- **Cadre d'une vue libre** (`ui.canvas`, iframe isolée) :
  - un cadre fourni par Inkwell, avec la ligne d'attribution ;
  - il reçoit les jetons **du thème actif** ;
  - aucun dessin hors du cadre.
- **Densité :** à 360px, une attribution par zone, jamais une par élément (test de §10, Codex
  R2-m1).

### 4.32 Extensions de projet (N23)

Une extension peut **appartenir au projet** : un admin l'ajoute, et elle se charge chez chaque
personne qui ouvre le projet (architecture §15.10, PP-1 à PP-7).

- **Provenance, à la première ouverture** (PP-3) : un bandeau d'information non bloquant, « Ce
  projet utilise 2 extensions ajoutées par Marie. · Voir ».
  - « Voir » ouvre Préférences › Extensions › « Extensions de ce projet ». Chaque ligne y dit
    qui l'a ajoutée et quand, son auteur, et l'empreinte du paquet en 4 mots.
  - Le bandeau ne revient plus pour ces versions.
- **Ce qui se charge sans question** : le niveau « projet » (affichage déclaratif, types de
  fichiers, coloration, lecture du projet, propositions sur commande).
- **Ce qui dépasse ce niveau** reste **en mode restreint**, sans le pouvoir demandé : réseau,
  stockage, fichiers choisis, dessin libre, personnes. La ligne dit alors : « Demande davantage :
  se connecter à nuage.example · Autoriser pour moi ». Autoriser ouvre le consentement (§4.29),
  qui vaut pour cette version et cet ordinateur.
- **Interrupteur personnel** (PP-4) : « Désactiver pour moi », **sur chaque ligne**. Le bandeau
  de provenance n'a que « Voir », sans ambiguïté sur l'extension visée.
  Les autres personnes ne sont pas touchées, et le projet reste lisible et modifiable.
- **Repli** : un contenu dont l'extension manque, ou est désactivée, s'affiche en texte, ou par
  son dernier rendu partagé (§4.34). Un encadré neutre le dit : « Ce passage s'affiche avec
  « Diagramme », désactivée pour vous. · Afficher le texte ».
- **Version incompatible** : « Mettez Inkwell à jour pour utiliser Diagramme », puis le repli.
- **Décision contestée** (PP-6) : les admins voient une carte « Deux versions de Diagramme ont
  été ajoutées en même temps : aucune n'est chargée. · Choisir ».
- **Badge « Extension du projet »** sur chaque cadre, chaque bouton épinglé et chaque page de
  réglages d'une extension de projet (PP-5).
- Une modification proposée par une extension de projet porte « via Diagramme (ajoutée par
  Marie) » dans l'Historique (PP-2).

### 4.33 Ne pas dénaturer Inkwell (N24)

Une extension ajoute, décore ou enveloppe. Elle n'enlève ni ne cache rien (architecture §15.11,
ND-1 à ND-8).

- **Zones protégées**, qu'aucune contribution ne peut atteindre :
  - dans la barre d'en-tête : nom du projet, pile des personnes, état de synchronisation et
    d'enregistrement, « Partager » ;
  - qui modifie quel fichier (curseurs, bande des modifications) ;
  - les bandeaux de confiance et de droits ;
  - les dialogues d'accord ;
  - Annuler et Rétablir ;
  - le panneau Personnes ;
  - le gestionnaire d'extensions ;
  - « Redémarrer sans extensions ».
- **Dialogues de confiance inimitables** (ND-2) :
  - un dialogue d'accord, de droits ou d'invitation est **toujours rendu par le cœur** ; il porte
    l'en-tête « Inkwell » et l'icône de l'application ;
  - un dialogue d'extension porte **toujours** la bande « Extension « X » », ne peut afficher ni
    cet en-tête, ni cette icône, ni un champ de mot de passe, et ne s'ouvre jamais par-dessus un
    dialogue du cœur.
- **Raccourcis** (ND-3) :
  - les raccourcis du cœur sont réservés ;
  - un conflit rend le raccourci de l'extension **inactif** ;
  - il est signalé par le badge « Conflit » sur la ligne de l'extension et dans la fenêtre des
    raccourcis (§4.26) ; la personne tranche (« Garder celui d'Inkwell » ; « Donner à X », qui **déplace** l'action d'Inkwell sur un autre
    raccourci choisi dans la même fenêtre, sans jamais la laisser sans raccourci ni sans menu ;
    « Choisir un autre raccourci… ») ;
  - jamais en silence.
- **Textes d'extension** (ND-4) : fr et en présents ; longueurs bornées (titre de commande ≤ 40,
  infobulle ≤ 80, toast ≤ 120 caractères) ; aucun terme interdit du glossaire. Sinon, l'élément
  est refusé au chargement, avec la raison dans l'Inspecteur (§4.35).
- **Mode sans extensions** (ND-6) : il charge le registre d'une installation neuve : les extensions **intégrées** (éditeur, aperçu, Tâches…), sans aucune extension ajoutée ni de projet, avec le thème Adwaita (architecture ND-6).
  - **Entrées :**
    - menu principal › « Redémarrer sans extensions » ;
    - Préférences › Extensions ;
    - `inkwell --safe-mode` ;
    - proposé d'office après deux démarrages interrompus, par un dialogue du cœur : « Inkwell
      s'est arrêté deux fois au démarrage. Redémarrer sans extensions ? · Redémarrer sans
      extensions · Démarrer normalement » ;
  - **Repère** : une pastille permanente « Sans extensions » dans la barre d'en-tête, à côté de
    l'état (L-8d). Elle ouvre un popover : « Seules les extensions **intégrées** sont chargées, telles que livrées ; vos extensions et
    celles du projet ne le sont pas. Thème Adwaita. · Redémarrer normalement » ;
  - le thème est Adwaita.

### 4.34 Diagrammes : le parcours de référence (N24)

Architecture §15.14. **Graphviz** (DOT) est le cas livré, par l'extension intégrée
« Diagrammes » en WebAssembly ; D2 sert d'exemple. **Mermaid est différé** (P-FRAME) : il exige un
moteur de navigateur. Le parcours vaut pour tout langage de rendu.

- **Écrire :**
  - Markdown : bloc ```` ```dot ```` ; Typst : bloc brut ```` ```dot ````, même règle ;
  - Insérer › Diagramme propose les langages présents, coloration FT-2.
- **Voir :** dans l'aperçu, à sa place, et **en ligne** sous le bloc : un widget repliable, avec
  le filet `--mark-extension` et le pied « Dessiné par Graphviz ».
  - Mise à jour 500ms après la dernière frappe.
  - L'ancien dessin reste, avec « Mise à jour… » après 300ms.
- **Décrire le dessin** (1.1.1 ; Codex R3-M1, Fable m5). Chaque diagramme a une **description**
  en phrases, éditable, partagée avec le rendu :
  - le moteur de rendu en propose une (`render → {svg, description}`, architecture §15.14) : « Graphe
    de 6 étapes : Demande → Validation → … ; 2 retours vers Demande. » ;
  - la personne la **corrige ou la remplace** dans le widget : champ « Description », une ligne,
    avec « Plus de détails » pour un texte long ;
  - elle est **écrite dans la source**, de façon standard et lisible sans Inkwell : légende de
    `#figure` en Typst, texte alternatif en Markdown ;
  - elle **voyage** avec le rendu : `image(alt: …)` dans l'aperçu et le PDF (PDF/UA, `AD-44`),
    attribut `alt` à l'export HTML, et « Vue lecture » de l'aperçu (architecture §15.17) ;
  - **sans description**, le widget le signale (`warn` : « Ajoutez une description, pour les
    personnes qui ne voient pas le dessin »), et les Problèmes le comptent en avertissement ;
  - le code source reste une information **en plus**, jamais le seul équivalent.
- **Erreur, en ligne**, jamais en toast : le dernier dessin réussi reste, estompé, avec « Ligne 4 :
  flèche incomplète ». Un clic amène à la ligne (§4.36).
- **Trop complexe** (plafond de 5s) : « Diagramme trop complexe pour être dessiné. », avec le
  dernier dessin réussi.
- **Collaboration :**
  - sans l'extension, on voit le **dernier rendu partagé** et sa description : « Dernier dessin
    partagé · Dessiné par Graphviz chez Marie » ;
  - dessin périmé : « Le dessin partagé ne correspond pas à sa source · Redessiner » (chez qui a
    l'extension) ;
  - jamais dessiné : « Ce diagramme s'affiche avec Graphviz, que vous n'avez pas. · Afficher le
    texte ».
- **Lecteur d'écran :** l'image se lit avec sa **description** ; « Diagramme, ligne 12 » n'est
  que son titre.

### 4.35 Outils de développement des extensions (N25)

Pour qui écrit une extension (architecture §15.12, §15.15). **Invisibles par défaut.**

- **Mode développeur** : Préférences › Avancé › « Mode développeur ». Il est **local**, jamais
  synchronisé, et jamais appliqué à une extension de projet.
- **Repère permanent et clair** :
  - une pastille « Mode développeur » en `--destructive-bg`, avec texte blanc (4,83 / 6,11),
    dans la barre d'en-tête, à côté de l'état ;
  - un filet de 2px `--destructive-bg` en haut de la fenêtre ;
  - un clic sur la pastille propose « Quitter le mode développeur ».

  C'est le « bandeau rouge permanent » de l'architecture, rendu comme une pastille pour ne pas
  prendre l'emplacement du bandeau (L-8d).
- **Extension non empaquetée** : « Charger depuis un dossier… ». Elle est rechargée à chaque
  modification, avec le toast « Diagramme rechargée », et porte le badge « Non signée · en
  développement ».
- **Inspecteur d'extensions** : un onglet du **tiroir**, présent seulement en mode développeur.
  Pour l'extension choisie (sélecteur) :
  - **Journal** (`role="log"`, horodaté, filtre par niveau, copier) ;
  - **Appels** (nom, durée, résultat) ;
  - **Temps et mémoire** par rapport à ses budgets (§15.4), en barres avec valeur écrite ;
  - **Refus d'autorisation** (« lire vos documents : refusé ») ;
  - **Événements reçus** ;
  - **Contributions refusées** au chargement (ND-4, jetons de thème inconnus).

  Rien ne quitte l'ordinateur ; « Copier le rapport » met le texte dans le presse-papiers.
- **Débogage par extension** (architecture §15.15) : dans l'Inspecteur, un interrupteur
  « Enregistrer pour rejouer », par extension, seulement pour une copie de développement, jamais
  pour une extension de projet ni pour un paquet signé.
  - Pendant l'enregistrement : la pastille « Débogage de Diagramme » s'ajoute à celle du mode
    développeur.
  - L'Inspecteur affiche « Enregistrement en cours (12 Mo sur 64) · Arrêter · Exporter une trace
    expurgée… ».
  - L'enregistrement contient du texte de documents : il reste local, et l'export propose
    l'expurgation, cochée par défaut (`--redact`).
- **Messages destinés aux développeuses** : ils sont techniques **seulement dans l'Inspecteur**.
  Le reste de l'interface garde le glossaire (P5).

### 4.36 Problèmes de compilation : où, et comment corriger (N28)

**But :** voir **où** est le problème et **comment** le corriger, tout de suite. Typst, Markdown
(mis en page par Typst ou en CSS), CSS, extensions de rendu.

**Sources, réutilisées et non réinventées :**
- **Typst** : les diagnostics de **tinymist** (LSP), avec leur position (plage), leurs
  **conseils** (`hint`) et leur trace (« appelé depuis ») [S19]. Les diagnostics de la
  compilation de l'aperçu servent de secours.
- **Markdown** : les erreurs de la mise en page Typst (`cmarker`), ramenées à la ligne du `.md` ;
  les liens et images introuvables (avertissement).
- **CSS** : les refus de l'assainisseur (B8), par exemple « `@import` retiré : un style ne peut
  pas charger un autre fichier » (avertissement).
- **Extensions de rendu** : l'erreur d'un diagramme (§4.34).

**Gravités :**

| Gravité | Sens | Soulignement | Marge | Compté dans la barre d'état |
|---|---|---|---|---|
| **Erreur** | l'aperçu et l'export ne peuvent pas se mettre à jour | ondulé `--error` | icône `fail` | oui (⊗ n) |
| **Avertissement** | le document se compile, mais quelque chose cloche (police absente, lien cassé) | pointillé `--warning` | icône `warn` | oui (⚠ n) |
| **Conseil** | suggestion du compilateur (`hint`) | aucun ; seulement dans la carte | — | non |
| **Suggestion** (N30) | orthographe, style, typographie (§4.38) | pointillé `--suggestion-line` (gris-bleu, jamais rouge) | aucun | **non** ; listée dans l'onglet Relecture |

Les formes et les icônes diffèrent d'une gravité à l'autre : la couleur n'est jamais seule
(A-3). Les couleurs viennent des jetons, et les contrastes sont dans `tokens.json`.

**Dans l'éditeur :**
- **Soulignement** sous la plage exacte, et **marqueur dans la marge**, à droite des numéros de
  ligne (`@codemirror/lint` : `setDiagnostics` et `lintGutter` ; son panneau n'est pas employé).
- **Carte de survol, ou de focus**, quand le curseur entre dans la plage et après F8 :
  - la phrase en clair, en fr ou en en (« « titre » n'est défini nulle part. ») ;
  - **Conseil :** la suggestion (« Vérifiez l'orthographe, ou définissez-le avec `#let`. ») ;
  - « Texte d'origine », une section repliable avec le message du compilateur tel quel, en mono
    et en anglais, et « Copier » ;
  - pour une trace : « Appelé depuis main.typ, ligne 3 », en lien.
- **Calme pendant la frappe :** le soulignement d'une plage **qu'on est en train de taper**
  n'apparaît qu'après **1s sans frappe**, ou quand le curseur quitte la ligne. Les autres
  restent à jour. Les diagnostics suivent la compilation de l'aperçu, qui attend 300ms de
  calme. Rien ne déplace le curseur ni ne vole le focus.

**Un clic, et le curseur est sur l'erreur** (« comme un IDE »). De **chaque** endroit qui
signale un problème, **un seul clic** (ou Entrée) suffit :
- ouvrir le fichier, dans son onglet (§4.25), même s'il était fermé ;
- **placer le curseur au début de la plage fautive**, la ligne centrée à l'écran, le focus dans
  l'éditeur ;
- ouvrir la carte du problème (lue par le lecteur d'écran).

On peut alors corriger tout de suite.

| Où le problème est signalé | Geste | Ce qui se passe |
|---|---|---|
| **compte de la barre d'état** (⊗ 2 ⚠ 1) | clic sur le compte | **va au premier problème** (le plus grave, dans le fichier actif d'abord) ; chaque nouveau clic va au suivant. Ce sont **deux boutons voisins**, deux arrêts de tabulation : « Aller au premier problème » (le compte), puis « Problèmes, liste » (⌄, *Menu Button* APG), chacun avec une zone de 34px (ST-1 ; Fable m7). |
| **liste Problèmes** (popover) | clic ou Entrée sur un élément | va à ce problème ; le popover se ferme |
| **bandeau d'erreur de l'aperçu** | « Voir le premier problème » | va au premier problème qui bloque l'aperçu |
| **marqueur de la marge** | clic | curseur au début de la plage de cette ligne, carte ouverte |
| **erreur dans un fichier importé, un modèle ou une charte** | clic dans la liste | ouvre **ce fichier** dans un onglet, curseur sur l'erreur ; modèle ou charte : onglet **en lecture**, avec « Aller à l'appel dans main.typ » en tête de la carte |
| **erreur de l'export** (toast d'échec, §5.12) | « Voir le premier problème » | idem |
| **carte d'un diagramme en échec** (§4.34) | clic sur « Ligne 4 : … » | curseur dans le bloc, à cette ligne |

**Clavier :**
- **F8 / Maj+F8** : problème suivant ou précédent, dans tout le projet. Ce sont les touches de
  VS Code, la convention de fait [S22].
  - Pas de conflit : F2, F3/F4 (recherche), F5 (présenter), F6 (zones), F7 (différences), F9,
    F10 et F1 ont d'autres usages, F8 était libre (§8.2).
  - Le registre (K-6) le vérifie.
- **Alt+Entrée** : ouvre les corrections rapides à la position du curseur (convention de
  JetBrains).
  - **Pas Ctrl+.** : sous GTK 4, Ctrl+. et Ctrl+; ouvrent le sélecteur d'émojis
    (`GtkText::insert-emoji`). WebKitGTK ne reçoit jamais ces touches (bug WebKit 256080) [S25].
  - Ils sont désormais dans la liste des raccourcis réservés par la plate-forme (K-6 ; Fable
    R3-M2).

**Corrections rapides.**
- Quand tinymist propose une **action de code** (`textDocument/codeAction` du LSP, par exemple
  « Importer `x` depuis… » ou « Corriger en « titre » ») [S19] :
  - une **ampoule** apparaît dans la marge, sur la ligne du curseur ;
  - la carte du problème liste les corrections en boutons (« Corriger : remplacer par
    « titre » »).
- Alt+Entrée, ou un clic sur l'ampoule, ouvre le même menu (APG *Menu*).
- **Appliquer une correction = une modification**, annulable par Ctrl+Z (ND-8), attribuée à la
  personne.
- Une extension qui fournit des diagnostics peut fournir des corrections par le même point
  (`diagnostics`, §4.31 ligne 10). Elles portent « via X ».

**Le même comportement pour tous les langages.** Un **modèle de diagnostic unique** :
`{fichier, plage, gravité, message en clair, message d'origine, conseils, corrections, trace,
source}`. Il est alimenté par :

| Source | Langage | Exemples |
|---|---|---|
| tinymist (LSP) et compilation Typst | Typst ; Markdown mis en page par Typst, ramené à la ligne du `.md` | variable inconnue, fichier manquant |
| analyseur Markdown | Markdown | lien ou image introuvable (avertissement) |
| assainisseur CSS (B8) | CSS | `@import` ou `url()` externe retirés |
| analyseur CSV (`editor/csv/problems.ts`, `findProblems` : guillemet non fermé ou isolé ; ligne seulement, sans colonne) | CSV, TSV | guillemet non fermé à la ligne n, nombre de colonnes irrégulier (avertissement) ; dans le tableur, la cellule est marquée et le clic y amène |
| point `diagnostics` des extensions (architecture §15.4) | langages fournis par une extension | préfixe « X : » dans la carte et la liste |

Soulignement, marge, carte, liste, F8, Alt+Entrée et annonces sont **identiques** quelle que soit la
source.

**Cartes au clavier** (Codex R3-M4) :
- **Ctrl+Maj+Espace** ouvre la carte du problème, ou l'aide (§4.37), à la position du curseur.
  C'est un **popover non modal** (`role="dialog"`, nommé « Problème ligne 12 »), et **le focus
  y entre**.
- Tab parcourt : corrections, « Texte d'origine », « Copier », « Aller à l'appel ».
- Échap ferme, et rend le focus à l'éditeur, au même endroit.
- La carte au survol de la souris ne prend pas le focus.

**Résultats périmés** (Codex R3-M7 ; architecture LSP-3) :
- chaque diagnostic et chaque correction sont liés à la **version** du document ;
- si le texte a changé depuis (soi ou une autre personne), la plage suit le curseur Automerge ;
- une correction dont la plage n'existe plus est **grisée**, avec « Plus à jour · Recalculer ».
  Elle n'est jamais appliquée à une mauvaise plage.

**Sans position** : un diagnostic sans ligne (autorisé par le protocole) se range sous son
fichier, ou sous « Projet ». Il porte « Sans position précise » et « Détails » ; un clic ouvre le
fichier au début. On n'invente jamais une ligne.

**Service indisponible** (tinymist arrêté, ou trop lent) :
- l'écriture continue, et la compilation de l'aperçu fournit encore ses diagnostics ;
- la liste affiche « L'aide à l'écriture est indisponible · Réessayer » ;
- pas d'indicateur d'attente permanent ;
- après 3 échecs, plus de nouvel essai automatique.

**La liste « Problèmes »** n'ajoute pas de panneau (cohérence). C'est le **popover de la barre
d'état** qui existe déjà (`ui/shell/StatusBar.svelte:84-129`), étendu :
- groupée **par fichier**, le fichier actif d'abord, puis l'ordre des lignes ; chaque élément a
  son icône, sa phrase en clair et « fichier, ligne n » ;
- un clic ou Entrée **ouvre le fichier** (onglet, §4.25) et place le curseur sur la plage, avec la
  carte ouverte ;
- **erreurs dans un fichier importé** (`#import`, `#include`) : groupées sous ce fichier, avec
  « appelé depuis main.typ, ligne 3 » ;
- **erreurs dans un modèle ou une charte** (paquet dans `.typstdeps/` ou `@local`, §5.19) : sous
  « Modèle « Rapport ACME » » ou « Charte « ACME » ». Le fichier s'ouvre **en lecture**, et la
  ligne de **votre** fichier qui l'appelle est donnée en premier : c'est souvent là qu'on corrige ;
- F8 / Maj+F8 vont au problème suivant ou précédent, d'un fichier à l'autre ;
- si la liste dépasse 200 éléments : « Et 37 autres : corrigez les premiers, la liste se mettra
  à jour » (le code tronque déjà : `preview.truncated`).

**Messages en clair (fr et en).**
- Une **table de traduction**, versionnée avec tinymist, associe les messages fréquents de Typst
  à une phrase et à un conseil :

  | Message d'origine | Phrase | Conseil |
  |---|---|---|
  | `unknown variable: x` | « x » n'est défini nulle part. | Vérifiez l'orthographe, ou définissez-le avec `#let`. |
  | `file not found` | Le fichier « logo.png » est introuvable dans le projet. | Ajoutez-le, ou corrigez le chemin. |
  | `unclosed delimiter` | Une parenthèse ou un crochet n'est pas fermé. | Cherchez son partenaire avant la fin de la ligne. |
  | `expected expression` | Il manque une valeur ici. | — |
  | `unknown font family` | La police « X » n'est pas disponible : une autre est utilisée. | Ajoutez-la au projet, ou choisissez-en une autre. |
  | paquet introuvable | Le paquet « X » n'est ni fourni ni dans le projet. | Inkwell ne télécharge jamais de paquet ; ajoutez-le dans le dossier du projet. |

- **Un message absent de la table** s'affiche tel quel, précédé de « Message du compilateur : ».
  Il n'est jamais caché, et le conseil de tinymist s'ajoute s'il existe.
- Les nouvelles traductions s'écrivent à partir des messages relevés en test utilisateur.

**Aperçu** (§4.18) : en cas d'erreur, **la dernière version correcte reste affichée**, sous un
bandeau **local à l'aperçu**, discret (`--info-bg`) : « 2 erreurs : l'aperçu montre la dernière
version correcte. · Voir le premier problème ». Pas de bandeau global, pas de toast.
L'**export** refusé dit la même chose, avec la même action (§5.12).

**Lecteur d'écran :**
- la barre d'état annonce le compte, en région **polie**, **seulement** quand il passe de 0 à n,
  ou de n à 0, et au plus une fois toutes les 2s ;
- jamais à chaque frappe ;
- entrer dans une plage fautive lie la carte à l'éditeur par `aria-describedby` : le message se
  lit à la demande ;
- F8 annonce « Erreur, ligne 12 : … ».

**Données nécessaires** (inventaire G-81) : chaque diagnostic porte la plage (ligne et colonne
de début et de fin), la gravité, le message d'origine, le conseil, le fichier et la trace.
Aujourd'hui, le protocole de l'aperçu porte la gravité, le message (tronqué à **4 000**
caractères), le fichier, le paquet manquant, la **ligne et la colonne** de début
(`preview/protocol.ts:8-19`, `:134-142`). Manquent la fin de plage, le conseil, les corrections
et la trace (G-81 ; Fable m3, Codex R3-m1).

### 4.37 Aide à l'écriture : complétion, signatures, aide au survol (N29)

**But :** quand on importe une charte faite de nombreux fichiers Typst, l'éditeur propose ses
fonctions et dit **exactement quoi leur passer** : noms, types, valeurs par défaut,
documentation. Cela vaut aussi pour les fonctions qu'on écrit soi-même.

**Ton :** c'est un outil d'**écriture**, pas un IDE. L'aide vient quand on la demande, ou quand
on tape là où elle sert. Elle ne clignote jamais, et ne recouvre jamais la ligne en cours.

**Sources, réutilisées.**
- Toute l'intelligence vient de **tinymist**, déjà embarqué, par le LSP [S19] :
  - complétion ;
  - aide de signature ;
  - survol, avec la documentation rendue ;
  - aller à la définition ;
  - indices de noms de paramètres (*inlay hints*).

  Son branchement à l'éditeur relève de l'architecture (point « Complétion, survol, aide de
  signature », §15.4, budget p95 ≤ 100ms, sinon résultats abandonnés).
- L'affichage passe par **`@codemirror/autocomplete`** (fenêtre de complétion, `info`,
  Ctrl+Espace) et par les **infobulles de survol** de `@codemirror/view` (`hoverTooltip`) [S23].
  Seul le style change : jetons Adwaita, popover (§4.7).

**1. Fenêtre de complétion.**
- **Quand elle s'ouvre :**
  - après `#` et un début de nom ;
  - après `.` (champ d'un module ou d'un dictionnaire) ;
  - dans les arguments d'un appel ;
  - après `@` (étiquettes et références, T11 B7) ;
  - dans une chaîne de chemin (`#image("`, `#include "`) ;
  - ou sur demande, avec **Ctrl+Espace**.

  Jamais sur une lettre isolée en plein texte.
- **Ligne** :
  - une **icône de genre** (fonction, variable, module, paramètre, étiquette, fichier,
    symbole), symbolique en 16px, **doublée d'un nom accessible** (« fonction ») ;
  - le nom, avec les lettres tapées en gras ;
  - à droite, en `--fg-dim`, la **provenance** (« charte ACME », « ce fichier », « Typst »).
- **Aperçu de la documentation** : à côté de la liste, si la place le permet, sinon dessous.
  Il montre la signature (`titre-section(texte, niveau: 1)`) et la **première phrase** de la
  documentation, en `--text-small`, puis « Plus d'aide : Ctrl+Maj+Espace ».
- **Ordre** : ce qui vient du projet et des chartes importées d'abord, puis Typst. Au plus 50
  éléments affichés, filtrés au fil de la frappe.

**2. Aide de signature**, pendant qu'on tape les arguments :
- une bulle **au-dessus** de la ligne (jamais dessus), qui montre la signature complète ;
- le **paramètre courant en gras et souligné** (pas en couleur seule) ;
- sous la signature, ce paramètre : **type**, **valeur par défaut** (« par défaut : 1 ») et sa
  phrase de documentation ;
- elle suit la virgule. Échap la ferme ; Ctrl+Maj+Espace la rouvre.

**3. Carte de survol** (souris), ou au clavier par **Ctrl+Maj+Espace**. Ce raccourci ouvre l'aide
de signature dans des arguments, et la carte ailleurs :
- la **documentation rendue** : titre et signature en mono, description en `--text-body`,
  **paramètres en tableau** (nom, type, défaut, description), valeur rendue, exemple s'il y en a ;
- le lien « **Aller à la définition** » (aussi **F12**) **ouvre le fichier source dans un
  onglet** (§4.25), curseur sur la définition. Une charte ou un modèle dans `.typstdeps/`
  s'ouvre **en lecture** ;
- Alt+← revient où l'on était (§3.2) ;
- une fonction **sans documentation** montre quand même sa signature, puis la phrase « Pas de
  description. Ajoutez un commentaire `///` au-dessus de sa définition pour en écrire une. » ;
- la carte apparaît après 500ms de survol, reste tant que le pointeur est dessus, et Échap la
  ferme (1.4.13). Elle ne s'affiche jamais pendant la frappe.

**4. Indices de noms de paramètres** (*inlay hints*) :
- **désactivés par défaut** ; Préférences › Éditeur › « Afficher le nom des arguments » ;
- discrets : `--text-caption`, `--fg-dim`, sur fond `--hover-bg`, rayon `--radius-small` ;
- seulement pour les arguments positionnels de plus de deux caractères ;
- `aria-hidden` : ils ne sont pas du texte.

**5. Documenter une charte ou un modèle : la convention `///`.**
- On reprend la convention de **tidy**, l'outil de documentation de Typst, que tinymist lit
  [S24] : des lignes `///` au-dessus d'un `#let`.
  - La première phrase sert de résumé.
  - Chaque paramètre s'écrit `- nom (type): description`, et le retour `-> type`.
  - Exemple :

    ```typ
    /// Titre de section aux couleurs de la charte.
    /// - texte (content): le titre lui-même.
    /// - niveau (int): 1 pour une partie, 2 pour une sous-partie.
    /// -> content
    #let titre-section(texte, niveau: 1) = { … }
    ```
- **Insérer › Documentation de fonction** écrit ce squelette, rempli à partir de la signature
  sous le curseur.
- L'**assistant « Enregistrer comme modèle »** (§5.19, étape 2) **compte les fonctions
  exportées sans documentation**. Il le dit sans bloquer : « 4 fonctions de la charte n'ont pas
  de description : les personnes qui utiliseront ce modèle ne sauront pas quoi leur passer. ·
  Les voir ». « Les voir » ouvre la première dans un onglet.

**6. Clavier et lecteur d'écran** (APG *Combobox* avec `listbox` [S5], déjà suivi par
`@codemirror/autocomplete`) :
- l'éditeur porte `aria-autocomplete="list"`, `aria-expanded` et `aria-controls`. La liste est
  un `listbox` ; l'option active est désignée par `aria-activedescendant` ; le focus ne quitte
  jamais le texte ;
- **↑ ↓** se déplacent ; Page↑ Page↓ ; **Entrée** insère, comme dans la palette (§4.24, K-5) ;
  **Tab** garde son rôle d'indentation (Fable m10) ; **Échap** ferme sans rien changer ;
- l'insertion d'une fonction place le curseur **entre les parenthèses**, et ouvre l'aide de
  signature ;
- annonces, en région polie : « 12 suggestions » à l'ouverture, puis le nom et le genre de
  l'option active (« titre-section, fonction, charte ACME »). L'aide de signature et la carte se
  lisent à la demande (`aria-describedby`) ;
- tout se fait au clavier : Ctrl+Espace, Ctrl+Maj+Espace, F12, Alt+← ;
- la **carte d'aide** suit le modèle des cartes de §4.36 : popover non modal, le focus y entre
  par Ctrl+Maj+Espace, Tab parcourt « Aller à la définition » et les détails, Échap rend le focus
  au texte.

**Résultats périmés et service absent** (Codex R3-M7) :
- une complétion, une signature ou une carte reçue **après** une modification (LSP-3 : la version
  ne correspond plus) n'est pas affichée ;
- si tinymist ne répond pas en 100ms, rien ne s'affiche, et on écrit normalement ;
- si le service est arrêté, Ctrl+Espace affiche une seule ligne : « Suggestions indisponibles ·
  Réessayer » ;
- jamais d'indicateur d'attente dans le texte.

**7. Langues.**
- Les libellés de l'interface sont en fr et en en : genres, « par défaut », « Pas de
  description », « Aller à la définition ».
- La **documentation** s'affiche dans la langue où l'auteur l'a écrite.
- La documentation de **Typst lui-même** vient de tinymist, en anglais ; elle est signalée
  « (en anglais) » dans l'interface française.

**8. Markdown, et les autres langages.**
- **Markdown**, là où cela a du sens :
  - chemins après `](` et `![` (fichiers du projet, avec icône de type) ;
  - ancres de titres après `#` dans un lien ;
  - même fenêtre et même clavier.

  Pas d'aide de signature.
- **Fichiers de code** (§4.28) : **aucune** complétion. Ils restent en lecture d'abord, sans
  outils d'atelier (FT-5).
- **Langages fournis par une extension** : par le même point (`completion`, `hover`, et
  `language-server` en API 1.1, architecture §15.12), avec le même rendu. La provenance affiche
  le nom de l'extension.

**Réglages** (Préférences › Éditeur) :

| Réglage | Défaut |
|---|---|
| « Proposer des suggestions en tapant » | activé ; désactivé, Ctrl+Espace reste |
| « Aide sur les arguments » | activée |
| « Afficher le nom des arguments » | désactivé |

### 4.38 Outils d'écriture (N30)

Étude : `architecture/research/editor-features.md` §8 à §11 ; architecture §15.17. **Pour qui aime
écrire** : tout est calme, rien n'est rouge, rien n'est noté. Tout est livré en extensions
intégrées, sur les points de §4.31.

**1. Orthographe, à la frappe, discrète.**
- Hunspell, avec des dictionnaires fr et en **embarqués** (licences dans « À propos »). La langue
  est celle du document (« Langue du document », réglage du projet).
- Seule la **prose** est vérifiée, jamais le code Typst ni le balisage : la prose est délimitée
  par l'arbre syntaxique.
- **Pointillé `--suggestion-line`** (gris-bleu, 5,55 / 8,04:1), gravité « suggestion » (§4.36) :
  **jamais rouge, jamais compté** dans la barre d'état. Apparition après 1s de calme, jamais sous
  le mot qu'on tape.
- Clic droit sur le mot, ou Ctrl+Maj+Espace : une carte avec jusqu'à 5 corrections, puis :
  - « Ajouter au dictionnaire du projet » ;
  - « Ajouter à mon dictionnaire » ;
  - « Ignorer ici » ;
  - « Autres mots… » (synonymes, si un jeu libre est retenu).
- **Dictionnaire du projet** (W1) : une liste de mots **partagée** (texte du projet). Un nom de
  personnage ajouté par l'une cesse d'être souligné chez toutes. On le gère dans Préférences ›
  Écriture : liste, retirer, qui a ajouté.
- Interrupteur : Préférences › Écriture › « Signaler les fautes d'orthographe » (activé).

**2. Relecture, à la demande** : un onglet **« Relecture »** du tiroir (L-2 : le temps et les
autres).
- **Rien n'est analysé tant qu'on n'appuie pas sur « Relire ».** Portée : le fichier, la
  sélection, ou le projet.
- Résultats **groupés par sorte** :
  - **Orthographe** ;
  - **Grammaire** (si l'extension LanguageTool est installée : greffon optionnel, hors ligne, en
    processus séparé, jamais livré par défaut) ;
  - **Style** : phrases longues, répétitions proches, adverbes ;
  - **Typographie** : espaces insécables, guillemets, points de suspension.
- **Gravité « suggestion »** pour tout (le quatrième niveau, §4.36). Chaque élément :
  - une phrase (« Phrase de 48 mots : la couper peut aider ») ;
  - « Corriger » quand c'est mécanique ;
  - « Ignorer » ;
  - un clic va au texte.
- **Jamais de note**, de score ni de « niveau de lecture » affiché comme un verdict. Une formule
  de lisibilité PEUT être donnée en information, repliée.
- Les soulignements de style n'apparaissent **que tant que l'onglet est ouvert**. Le fermer les
  retire.
- **Relecture adressée** (W3, v2) : « Envoyer comme commentaires à… » ; dépend des commentaires
  (Q8).

**3. Concentration.**
- **Mode concentration** (menu Affichage, palette, Ctrl+Alt+F) :
  - masque panneaux et barres ; la ligne ou le paragraphe courant est à pleine encre, le reste
    en `--fg-dim` ;
  - option « Machine à écrire » : la ligne courante reste au milieu de l'écran, **sans
    défilement animé** sous `reduce`.

  Échap ou le même raccourci en sortent. Un repère discret en haut, « Concentration · Échap pour
  quitter », s'efface après 3s.
- **Plein écran** : F11 (convention GNOME), aussi dans le menu.
- **Mots, caractères, temps de lecture** : dans la barre d'état (popover Statistiques existant,
  `ui/popover/StatsPopover.svelte`).
  - Le compte porte sur la **prose**, et sur la sélection s'il y en a une.
  - Temps de lecture : 230 mots par minute en français, réglable.
- **Objectifs, doux et locaux** : Préférences › Écriture › « Objectif de mots » (par séance ou par
  jour).
  - Un **cercle discret** autour du compte de mots se remplit ; atteint, il est plein, sans
    animation ni son ni message.
  - **Pas de série** à maintenir, pas de rappel, pas de comparaison entre personnes. Stocké sur
    cet ordinateur (règle 10 : aucune collecte).
  - Objectif de groupe (W4) : PEUT, sans nom par personne.

**4. Chercher et remplacer** (B6, G-38) :
- Ctrl+F dans le fichier, Ctrl+H pour remplacer, Ctrl+Maj+F dans le projet (onglet Rechercher du
  tiroir) ;
- les **expressions régulières** sont **masquées** derrière « Options avancées » ;
- « Remplacer tout » montre d'abord un aperçu, par fichier, puis s'applique en une modification
  par fichier, avec le toast « 12 remplacements dans 3 fichiers · Annuler » ;
- quelqu'un écrit dans un fichier touché : avertissement dans l'aperçu (« Paul modifie ch2.typ »).
- **Extraits** (`;sig` → signature) : personnels, ou **du projet** (partagés). Dans Préférences ›
  Écriture ; ils s'insèrent par la complétion (§4.37).

**5. Typographie française** (W2) :
- **Au rendu**, la règle d'affichage du modèle (`lang: "fr"`) pose les espaces fines insécables
  avant `; ! ?`, insécables avant `:` et dans « », sans toucher la source Typst.
- **À la saisie**, en Markdown et en texte, et en option : `"` devient « », `'` devient ’, `...`
  devient …, et l'insécable est ajoutée avant `; : ! ?`.
  - Préférences › Écriture › « Typographie française à la frappe » ; réglage **du projet**,
    partagé, pour éviter la « guerre d'apostrophes ».
  - Ctrl+Z annule la conversion seule.
- **Voir les insécables** : un point médian discret (`--fg-dim`), désactivable.
- La Relecture signale ce qui manque (§2 ci-dessus).

**6. Insérer** (menu Insérer, palette ; génère du Typst ou du Markdown propre, jamais opaque) :
- **Figure** : un dialogue `small`, avec image (glisser ou choisir), légende, **texte
  alternatif** (obligatoire) et étiquette ;
- **Tableau** : une grille de cases pour choisir la taille, ou « Depuis un fichier CSV du
  projet » ;
- **Équation** : un champ, avec **aperçu en direct** dans la carte et une palette de symboles ;
  « En ligne ↔ En bloc » ;
- **Caractère spécial** : un sélecteur unique (symboles Typst, ponctuation française, émojis),
  avec recherche par nom en français.
  - Raccourci **Ctrl+Alt+C**. Pas Ctrl+; (pris par GTK, K-6) ; l'étude proposait Ctrl+;, et ce
    point est en désaccord dans `CHANGES.md`.
  - APG *Listbox* en grille, Entrée insère.
- **Citer** : un sélecteur (auteur, titre, année) sur les **sources du projet** (`.bib`, Hayagriva),
  qui insère `@clé`. Les sources récentes des collègues portent leur avatar (W5).
  - « Importer un fichier .bib… » (export de Zotero, **par fichier**, sans connexion).
  - Les styles CSL embarqués sont mentionnés dans « À propos ».
- **Note de bas de page**, **Renvoi** (liste des cibles), **Table des matières**.

**7. Comparer** : « Afficher les modifications » entre deux versions nommées, **par mots**. C'est
la vue des différences de §4.22 (C-DIFF : souligné et barré, avec les signes). Entre deux
brouillons : en v2.

**8. Préréglages d'export** : « Exporter… » (§5.12) propose les **préréglages du projet**, par
exemple :
- « Relecture » : PDF avec numéros de ligne ;
- « Impression » ;
- « Écran ».

Ils sont stockés dans le projet, donc partagés, et rattachés au modèle (N27). On les crée et
on les modifie dans Exporter… › « Gérer les préréglages ».

### 4.39 IA (N31)

Étude : `architecture/research/ai.md` ; architecture §15.18 (ADR 0040 et 0041 à rédiger). L'IA est
une **extension intégrée** (`ext/ai`). Elle n'a aucun droit qu'une extension ne puisse demander,
passe par les points de §4.31, ne touche aucune zone protégée (§4.33, N24), et **propose sans
jamais écrire**.

**Principes de l'interface :**
- **Une personne dispose** : tout changement de texte est une **proposition** relue, morceau par
  morceau.
- **On voit ce qui part, et vers où**, avant le premier envoi.
- **On voit qui a quoi** : « via IA » dans l'Historique et pour les collègues.
- **Rien par défaut** : pas de fournisseur, rien ne part ; dans un projet partagé, rien sans
  décision d'admin.

**1. Où est l'IA (cohérence).**
- **Onglet « IA » du tiroir** (L-2 : les autres, et l'IA en est un). Il contient :
  - en tête, la **ligne d'état** : « IA : serveur de votre entreprise (DGX interne) · politique
    du projet : organisation » ;
  - la **discussion** : messages, propositions, champ de saisie ;
  - les **puces de contexte** au-dessus du champ (« Sélection · main.typ », « Plan »,
    « Problèmes ») : on en ajoute avec `@` (sélection, plan, problèmes, commentaires) ;
  - le **mode** en groupe de bascules :
    - **Corriger** : sans outil, texte → proposition ;
    - **Discuter** : lire et proposer ;
    - **Réglages** : produit des artefacts déclaratifs (réglages, jetons de thème, modèles),
      toujours confirmés.
- **« Demander à l'IA » sur une sélection** : menu contextuel de l'éditeur, palette, **Ctrl+Alt+I**.
  - Un **popover** ancré à la sélection : un champ, et des actions rapides (« Reformuler »,
    « Raccourcir », « Corriger », « Traduire en… », « Résumer »).
  - Le résultat s'affiche **en ligne**, en proposition (point 2).
- Aucune autre surface : pas de bouton dans la barre d'en-tête (zone protégée). Un bouton
  « IA » PEUT être **épinglé** par la personne (§4.30), comme toute extension.

**2. Propositions : relire, accepter, annuler.**
- Le texte en flux s'écrit dans une **couche fantôme locale** :
  - souligné pointillé `--ai-mark`, avec le libellé « Proposition de l'IA » ;
  - **jamais dans le document partagé** : les collègues ne voient rien avant « Accepter », pas de
    demi-phrases chez les autres.
- **Diff par morceau** (C-DIFF : ajouts soulignés, suppressions barrées, avec les signes). Chaque
  morceau a « Accepter » et « Refuser » ; en tête, « Tout accepter », « Tout refuser », et une
  ligne d'explication du modèle.
  - Clavier : ↑ ↓ d'un morceau à l'autre ; Entrée accepte ; Suppr refuse.
- **Mise en évidence obligatoire**, dans le diff (architecture §15.18) :
  - liens et chemins **nouveaux** ;
  - `#read`, `#include` hors projet ;
  - caractères invisibles (zéro largeur, balises Unicode, bidi).

  Chacun porte `warn` et une phrase (« Lien nouveau vers example.org »).
- Garde-fous :
  - au-delà de 30 % du fichier, un avertissement en tête (« Cette proposition change 42 % du
    fichier ») ;
  - un morceau qui touche le texte d'**une autre personne** porte son nom (« texte de Paul »).
- **Accepter = une modification**, signée par la personne qui accepte, étiquetée « via IA ». Un
  Ctrl+Z l'annule (ND-8). « Annuler cette discussion » défait toutes les modifications « via IA »
  de la séance, en une modification.
- **Proposition périmée** (le texte a changé entre-temps) : le morceau est grisé, « Plus à jour ·
  Redemander ». Il n'est jamais appliqué ailleurs.

**3. Autorat visible pour les collègues.**
- L'Historique dit « Camille, via IA (DGX interne) ». C'est une **déclaration signée par
  Camille, pas une preuve** : on écrit « marqué IA par Camille », jamais « écrit par une IA ».
- « Qui a écrit quoi » (§4.22) a une option « Marquer les passages IA » :
  - soulignement pointillé `--ai-mark` + pastille « IA » au survol (pas la couleur seule) ;
  - taper dans le passage le rend humain ;
  - rien n'est écrit dans le fichier ; « retirer les marques » à l'export.

**4. Texte fantôme en continu** (complétion « à la Zed ») : **désactivé par défaut**
(Préférences › IA).
- Le texte grisé `--fg-dim`, en italique, suit le curseur, et **reste local**.
- **Tab** l'accepte (si aucune fenêtre de complétion n'est ouverte) ; Échap ou la frappe
  l'effacent.
- `aria-hidden`, avec une annonce polie « Suggestion disponible, Tab pour accepter », au plus une
  fois par pause.
- Jamais sur un fichier marqué `ai: never`.

**5. Fournisseurs : Préférences › page « IA ».**
- **Interrupteur général** « Utiliser l'IA » (désactivé tant qu'aucun fournisseur n'est réglé).
- **Profils** (liste encadrée), « Ajouter un fournisseur… » :
  - **Nom** (« DGX interne ») ;
  - **Adresse du serveur**, compatible OpenAI (`https://dgx.example/v1`), pour NVIDIA NIM, vLLM,
    Ollama, llama.cpp ;
  - **Modèle** (liste lue sur le serveur, ou saisie) ;
  - **Clé** : un champ masqué, rangée dans le trousseau du système, jamais affichée en clair
    ensuite ;
  - **Certificat de l'entreprise** (AC interne : choisir un fichier ; empreinte affichée) ;
  - **Proxy** et certificat client (mTLS), sous « Avancé » ;
  - **Classe** : « Sur cet ordinateur », « Serveur de l'organisation », « Service en ligne » ;
  - **Prix** (facultatif) : coût par million de jetons, pour l'estimation.
- **« Tester la connexion »** répond par une phrase :
  - « Connexion réussie : 3 modèles, premier mot en 0,4 s, appels d'outils pris en charge » ;
  - ou une erreur précise : « Certificat inconnu : ajoutez le certificat de votre entreprise »,
    « Adresse injoignable », « Clé refusée » ;
  - les « Détails techniques » repliés.
- **États** : non configuré ; en test ; prêt ; injoignable ; clé refusée ; certificat inconnu.

**6. Politique du projet (admin) :** Préférences › Projet › « IA dans ce projet ».
- Groupe radio, chaque option avec sa phrase :
  - **Désactivée** ;
  - **Sur cet ordinateur seulement** : modèles locaux ;
  - **Serveurs de l'organisation** : profils nommés par l'admin (adresse et empreinte de
    certificat), les seuls permis ;
  - **Libre** : chacun choisit.
- **Défaut d'un projet partagé : rien ne part vers un serveur distant sans décision d'admin.**
  Tant que le protocole n'a pas l'espace de décision `ai/` (PD-1), l'IA est **désactivée dans tout
  projet partagé** (architecture §15.18).
- La décision est signée, prend effet pour l'avenir, et s'affiche dans la ligne d'état de l'IA et
  dans le popover d'état (§4.15).
- **Limite écrite à l'écran**, en clair : « Cette règle guide les applications Inkwell de
  l'équipe. Elle n'empêche pas une personne qui lit le document de le copier ailleurs. »
- **Fichier exclu** : menu contextuel du fichier › « Ne jamais envoyer à l'IA » (`ai: never`) ;
  icône discrète dans Fichiers.

**7. Avant le premier envoi : ce qui part** (dialogue d'accord **du cœur**, inimitable, §4.33).
Titre : « Envoyer à « DGX interne » ? ».
- **Vers où** : nom, adresse, classe (« serveur de votre entreprise »).
- **Quoi, exactement** : le texte qui part, en lecture, avec sa taille (« 3 pages, 1 240 mots »).
  Les **passages d'autres personnes** sont **marqués** (bord `--person-N-line` et nom : « texte de
  Paul ») ; une phrase les compte en tête.
- « Ne plus demander pour cette séance » ; « Annuler » (focus initial) ; « Envoyer ».

**8. Sécurité face aux consignes cachées** (injection de prompt ; architecture IA-1 à IA-5).
- **Contexte marqué** : dès que le contexte contient du texte d'une autre personne ou d'une source
  externe (import, collage, MCP), la ligne d'état affiche un **repère** (icône bouclier, et le
  mot) : « Le contexte contient le texte d'autres personnes : l'IA peut seulement **lire ce
  fichier et proposer** ».
  - Les autres outils sont retirés, et la liste le dit (« Réglages : indisponibles dans ce
    contexte »).
  - Le mode **Corriger** (sans outil) reste entier.
- **Confirmations au bon endroit**, pas partout :
  - une proposition de texte se confirme par sa relecture (le diff) ;
  - un **réglage** (artefact « Réglages ») ouvre un dialogue du cœur qui montre **l'effet exact**
    (« Taille du texte : 14 → 16 px ; Thème : Encre ») ;
  - supprimer, inviter, partager, changer les droits, les clés ou la politique **ne sont jamais
    proposés** à l'IA.
- Dans la discussion : **aucune image ni ressource distante** affichée ; un lien s'affiche comme
  texte, avec son adresse complète.

**9. Coût, durée, annulation, hors ligne.**
- Sous chaque réponse, en `--text-caption` : « 1,2 s · 830 mots · gratuit (serveur de votre
  entreprise) », ou « ≈ 0,02 € » si un prix est réglé.
- Jauge de contexte avant l'envoi : « l'IA verra 3 pages ».
- **Arrêter** : le bouton remplace « Envoyer » pendant la réponse ; Échap fait de même. Ce qui
  est déjà arrivé reste en proposition, marqué « interrompue ».
- **Hors ligne, ou serveur injoignable** : la fonction est grisée, avec la raison (« Le serveur de
  votre entreprise ne répond pas · Réessayer »). La Relecture déterministe (§4.38) reste
  disponible. Jamais d'erreur technique seule.

**10. Autres applications et agents** (désactivés par défaut).
- **Préférences › IA › « Laisser d'autres applications utiliser Inkwell »** (serveur MCP) :
  - interrupteur ;
  - portée en groupe radio : « Lire » ou « Lire et proposer ».

  Par défaut, la connexion se fait **sans port** (`inkwell mcp`, socket locale). « Accès par le
  réseau local (127.0.0.1) » est sous « Avancé », avec un jeton affiché une fois, puis « Copier ».
- **Première connexion d'une application** : dialogue du cœur, « « Claude Desktop » veut lire ce
  projet et proposer des modifications. · Refuser · Autoriser », **par application**. Les
  propositions arrivent dans la même file, « via Claude Desktop ».
- **Repère permanent** pendant une connexion : une pastille « Connecté à Claude Desktop » dans la
  barre d'en-tête, comme le mode développeur (L-8d). Un clic : « Déconnecter ».
- **Agents externes** (ACP) : Préférences › IA › « Agents ».
  - « Ajouter un agent… » montre la **commande exacte** et son empreinte.
  - Dans un projet partagé : décision d'admin.
  - L'agent n'a ni fichiers ni terminal ; il lit et **propose** par Inkwell (même file, « via
    Codex »).
  - Pendant une séance : la pastille « Agent : Codex au travail · Arrêter » ; ses actions
    refusées (exécuter, télécharger, supprimer) sont listées dans l'onglet IA.
  - Quand le système ne permet pas d'isoler l'agent, une phrase le dit à l'ajout.

**11. Lecteur d'écran et clavier.**
- La discussion est un `role="log"` : les réponses sont annoncées poliment, **une fois
  terminées**, jamais mot à mot.
- Le diff de proposition suit APG *Listbox* (morceaux), avec ses boutons par Tab.
- Le popover de sélection est un `role="dialog"` nommé ; Échap le ferme et rend le focus au
  texte.
- Textes en fr et en en ; les réponses du modèle sont dans la langue de la personne, demandée au
  modèle.

### 4.40 Ressources : voir ce qui tourne, et l'arrêter (N32)

Chaque extension est bornée (mémoire, calcul, temps ; architecture §15.5, ND-5). La personne voit
ce qui tourne, ce que cela consomme, et peut l'arrêter. C'est le modèle de `about:processes` de
Firefox et du Gestionnaire des tâches [S26].

**Où.** Pas de nouvelle fenêtre.
- **Préférences › page Extensions**, avec un sélecteur de vue **« Installées · En cours »** (§4.5) :
  les ressources sont l'autre face de la même gestion.
- Accès : menu principal › « Extensions en cours… », la palette, et l'action « Voir » des
  messages ci-dessous.

**Ce qui est montré : une table** (APG *Grid*, en lecture : une cellule par information).

| Colonne | Contenu |
|---|---|
| **Nom** | icône, nom, sorte : « Extension », « Extension du projet », « Séance d'IA », « Agent », « Service d'Inkwell » (mise en page Typst, aide à l'écriture Typst) |
| **Projet** | le projet auquel elle est rattachée, ou « Tous » |
| **État** | Active · Au repos · **Ralentie** · **En pause** · Arrêtée · **Plantée**, avec une icône et le mot |
| **Mémoire** | « 38 Mo sur 64 » et une jauge horizontale fine (valeur écrite, jamais la jauge seule) |
| **Calcul** | « 4 % » sur la dernière minute |
| **Délai** | délai récent par rapport au **budget** du point (§15.4) : « 120 ms (budget 300) » ; en `--warning` + `warn` au-delà |
| **Activité** (facultatif) | courbe de la minute écoulée en `--fg-dim`, `aria-hidden`, figée sous `reduce` ; désactivable |

- Lignes regroupées : **Services d'Inkwell**, puis **Ce projet**, puis **Mes extensions**, puis
  **IA et agents**. Un processus hôte qui en porte plusieurs se déplie en ses extensions.
- **Calme** : mise à jour toutes les 2s, sans animation ; « Figer » arrête la mise à jour.

**Actions**, dans une barre au-dessus de la table, sur la ligne sélectionnée, et dans son menu
contextuel :

| Action | Pour | Effet |
|---|---|---|
| Arrêter | tout, sauf les services | arrêt immédiat ; une ligne « Arrêtée · Relancer » |
| Relancer | tout | services compris |
| Désactiver | extensions | comme l'interrupteur de §4.29 |
| Désactiver pour ce projet | extensions de projet, et globales dans ce projet | PP-4 |
| Ouvrir le journal | tout | l'Inspecteur (§4.35) en mode développeur ; sinon un journal en lecture, sans données de documents |
| Désinstaller | extensions installées | comme « Retirer » (§4.29), données gardées |

- **Les services d'Inkwell** se relancent, mais ne se désactivent pas : l'action n'apparaît pas.
- Une séance d'IA ou d'agent s'**arrête** (comme « Arrêter » dans l'onglet IA).

**Limites appliquées automatiquement** : ce que la personne voit. **Jamais de dialogue en
cascade.**

| Événement (ND-5) | Message | Action |
|---|---|---|
| ralentie (cadence réduite) | rien de plus que l'état « Ralentie » dans la table | — |
| mise en pause (3 dépassements) | **toast** « Diagramme est trop lente et a été mise en pause. » | « Voir » (ouvre la vue En cours, ligne sélectionnée) |
| arrêtée au plafond de mémoire | toast « Diagramme a dépassé sa mémoire et a été arrêtée. Votre texte n'est pas touché. » | « Voir » |
| plantages répétés | bandeau d'information (§4.29 : désactivée après 3 arrêts) | « Voir » |
| plusieurs en même temps | **un seul** toast regroupé : « 3 extensions ont été arrêtées ou mises en pause. » | « Voir » |

Les toasts d'extension suivent TO-3 (au plus un toast toutes les 10s). Un service d'Inkwell qui
redémarre seul le dit une fois, par un toast : « La mise en page a redémarré. »

**Accessibilité :**
- table en APG *Grid*, avec en-têtes de colonne ;
- **tri** par clic, ou par Entrée sur un en-tête (`aria-sort`) ;
- flèches de cellule en cellule ; Origine et Fin ; Maj+F10 pour le menu de la ligne ;
- **pas d'annonce à chaque rafraîchissement** : on n'annonce poliment qu'un changement d'**état**
  (« Diagramme : en pause ») ;
- les nombres sont lus avec leur unité (« 38 mégaoctets sur 64 »).
- Textes en fr et en en ; jamais « processus », « CPU » ni « PID » : « calcul », « mémoire »,
  « délai ». Les termes techniques sont sous « Détails techniques ».

## 5. Écrans et parcours

Les cinq tâches de référence (`ux-principles.md`) sont : **créer, partager, relire, présenter,
exporter**. Chaque parcours a son entrée, ses étapes, ses messages et sa sortie.

### 5.1 Premier lancement

- **Accueil** : `StatusPage` plein écran.
  - Titre « Bienvenue dans Inkwell ».
  - « Écrivez seul ou à plusieurs, sans compte et sans serveur. »
  - **Votre nom** (obligatoire, 1 à 40 caractères ; espaces de bord retirés).
    - Vide : « Commencer » reste actif. Le clic affiche sous le champ « Donnez un nom : les
      autres personnes le verront. », avec `aria-invalid` (3.3.1), et le focus revient au
      champ.
    - Trop long : la saisie s'arrête à 40 caractères, et un compteur « 40 / 40 » s'affiche.
    - **Abandon** (fenêtre fermée) : rien n'est gardé ; l'accueil revient au lancement
      suivant.
  - **Votre couleur** : 8 pastilles en groupe radio, chacune nommée ; le créneau libre est
    proposé (C-PERSON-2).
  - « Commencer ».
- Puis le **projet d'exemple** s'ouvre : aperçu visible, curseur dans le texte, aperçu à jour
  dès la première frappe.
- Pas de visite guidée. Aucune question réseau avant « Partager » (règle 8).
- Le nom et la couleur se changent plus tard dans Préférences › Moi.

### 5.2 Projets

- **Écran Projets** (menu › Projets…, Ctrl+O). En tête, un sélecteur de vue **Projets ·
  Modèles** (§4.5) : la gestion des modèles vit ici (§5.19).
  - Vue **Projets** :
    - projets récents en liste encadrée (nom, dernière modification relative, personnes) ;
    - « Nouveau projet » (suggéré) ;
    - « Ouvrir un dossier… » ;
    - « Rejoindre avec une invitation ».
- **Nouveau projet** (dialogue `medium`) :
  - **Vide par défaut.** Inkwell ne fournit **aucun** modèle (N27). Les choix sont :
    - **Nom** ;
    - **Format du fichier principal** : « Typst » (documents, diapositives) ou « Markdown »
      (notes) ;
    - une section **« Partir d'un de mes modèles »**, qui n'apparaît que si la personne en a, ou
      si son équipe en partage. C'est la galerie, §5.19.
  - Le chemin du dossier s'affiche en `--text-small`. « Créer ».
- **Fermer un projet** : rien à confirmer ; retour à l'écran Projets.
- Le **projet d'exemple** du premier lancement (§5.1) n'est pas un modèle : c'est un projet
  ordinaire, qu'on peut supprimer.

### 5.3 Partager (inviter)

Dialogue « Écrire à plusieurs » :
1. Rôle « Écriture | Lecture ».
2. « Créer une invitation » : le texte de l'invitation, « Copier » (toast), et sa validité en
   clair.
3. Personnes à proximité, avec leur empreinte en 4 mots et « Inviter » ; l'interrupteur « Me
   rendre invisible ».
4. Demandes reçues : « Accepter · Refuser ».
5. La note de confiance.

Le Go fournit déjà tout cela (`internal/app/invites.go`).

### 5.4 Rejoindre

1. Coller l'invitation, puis « Rejoindre ».
2. « Connexion à Marie… ». Au-delà de 3s : « Marie doit avoir Inkwell ouvert ».
3. **Comparer** l'empreinte : 4 mots, « Ce sont les mêmes » ou « Ils sont différents ». On
   compare, on ne retape pas (3.3.8).
4. Réception, avec sa progression. Le fichier principal s'ouvre dès qu'il est là.
5. Toast « Vous avez rejoint « Rapport » ».

| Erreur | Texte | Action |
|---|---|---|
| invitation incomplète | « Cette invitation n'est pas complète. Copiez-la de nouveau en entier. » | — |
| déjà utilisée | « Cette invitation a déjà servi. Demandez-en une nouvelle à Marie. » | — |
| hôte absent | « Marie n'est pas joignable. Elle doit avoir Inkwell ouvert. » | « Réessayer » |
| empreintes différentes | « Par prudence, Inkwell n'a pas rejoint ce projet. Vérifiez l'invitation avec Marie. » | — |

**Côté admin :** une demande d'accès est un **bandeau-question** (rang « questions », L-8) :
« Léa demande à rejoindre · Accepter · Refuser ».

### 5.5 Fichiers : ouvrir, fermer, créer

- **Ouvrir** et **fermer** : §4.25.
- **Créer :**
  - « Nouveau fichier » (Fichiers › +, menu, palette ; Ctrl+N) ;
  - modèle Typst, Markdown ou texte ;
  - le nom se saisit **en place** dans l'arbre, avec l'extension présélectionnée hors du nom ;
  - le fichier s'ouvre dans un onglet.
- **Importer :** glisser depuis le bureau, ou « Importer… ». La progression s'affiche dans la
  ligne.

### 5.6 Fichiers : renommer, déplacer, supprimer

- **Renommer** (F2, menu contextuel) : champ en place (§4.13).

  | Erreur | Texte |
  |---|---|
  | nom vide | « Donnez un nom au fichier. » |
  | déjà pris | « « ch2.typ » existe déjà dans ce dossier. » |
  | caractère interdit | « Un nom ne peut pas contenir « / ». » |

  Si d'autres fichiers y renvoient : « Mettre à jour les 3 liens vers ce fichier ? · Mettre à
  jour · Laisser » (bandeau-question, B7).
- **Déplacer** : glisser, ou « Déplacer vers… ».
- **Supprimer :**
  - admin, ou réglage « librement » : corbeille, avec toast « « ch2.typ » mis à la corbeille ·
    Annuler » ;
  - écrivain, avec le réglage par défaut : **demande de suppression** (B5). La ligne est barrée,
    la mention « Suppression demandée » s'affiche, et les admins reçoivent un bandeau-question.
- **Corbeille :** section repliée de Fichiers, « Restaurer ». « Vider la corbeille » est réservé
  aux admins et confirmé (§4.9).

### 5.7 Éditeur commun

- **Enregistrement :**
  - automatique : rien à voir ;
  - Ctrl+S : un point dans l'onglet et dans le titre, « Non enregistré » dans la barre d'état.

  Fermer ne pose jamais de question.
- **Modifications hors d'Inkwell :** toast « 3 lignes intégrées depuis le disque · Annuler ».
  Les lignes intégrées restent teintées 12s, ce qui remplace « Voir ». L'annulation durable est
  Ctrl+Z (TO-1). En mode « Demander » : bandeau-question (§5.16).
- **Zoom du texte** (50–300 %) : Ctrl+= / - / 0, et l'indicateur dans la barre d'état.
- **Rechercher :** Ctrl+F et Ctrl+H dans le fichier (barre au-dessus de l'éditeur) ;
  Ctrl+Maj+F dans le projet (onglet Rechercher du tiroir). Le module existe mais n'est pas
  branché (G-38).

### 5.8 Typst

- Barre Typst ; aperçu (§4.18) ; problèmes (F8) ; saut source ↔ aperçu ; menu Insérer.
- Un paquet manquant : l'action reste, le chemin passe dans « Détails techniques ».

### 5.9 Markdown

- Barre Markdown.
- **Mise en page** (barre d'état) :
  - « Pour imprimer (PDF) » : modèles Typst, aperçu en pages ;
  - « Pour l'écran (HTML) » : styles, aperçu en colonne isolée.
- Les noms sont simples (« Rapport », « Lecture »), jamais « cmarker » ni « CSS ».

### 5.10 CSV, texte, gros fichiers

- **CSV et TSV :** tableur, sans barre de mise en forme ni aperçu ; « Afficher le texte ».
- **`.txt` tabulaire :** « Ouvrir comme tableau » est proposé, jamais imposé.
- **Gros fichiers :** voir les seuils de §5.17.

### 5.11 Présenter

1. F5, Maj+F5, ou « Afficher la vue public sur cet écran ».
2. Avec deux écrans : public sur l'écran externe ; « Échanger les écrans ».
3. Les autres voient le bandeau « Marie présente · **Suivre** » ; puis « Parcourir librement »
   et « Revenir au présentateur ».
4. **Prendre la main** : le pilote reçoit un **bandeau-question** dans sa vue : « Paul demande à
   prendre la main · Accepter · Refuser ». Sans réponse au bout de 30s, la demande expire, et
   Paul voit « Marie n'a pas répondu ».
5. Échap : retour à l'éditeur, sur la diapositive qui était affichée.

### 5.12 Exporter et imprimer

- **Exporter…** (menu, palette, Ctrl+Maj+E ; bouton de la barre de l'aperçu) ouvre le
  sélecteur de fichier du système :
  - Typst, ou Markdown en mise en page Typst : PDF ;
  - Markdown en mise en page CSS : HTML autonome (B8) ;
  - toujours possible : PDF de toutes les pages, ou de la page courante.
- **Résultat :** toast « PDF enregistré · Ouvrir ». « Afficher le dernier export dans le dossier » est dans le menu
  Document. Le chemin
  s'affiche dans l'infobulle, **pas dans la phrase**. Écart actuel :
  `fr.json:450` (« PDF enregistré : {path} »).
- **Erreurs :**
  - « Le document contient des erreurs. **Aller au premier problème** » (la même commande que
    F8, §4.36) ;
  - « La mise en page ne démarre pas. **Réessayer** ».
- **Imprimer…** passe par le menu et la palette, **sans raccourci**. Ctrl+P sert déjà à
  « Aller au fichier », et Ctrl+Maj+P à la palette.
  - Inkwell exporte un PDF temporaire et l'ouvre dans la boîte d'impression du système (portail
    d'impression XDG sous Linux).
  - *À confirmer techniquement en N18* (Q10).

### 5.13 Préférences (Ctrl+,)

| Groupe | Réglages | Portée |
|---|---|---|
| **Apparence** | **Thème** : Adwaita, puis les thèmes installés, avec vignette, refus expliqué et « Revenir au thème par défaut » (§2.10) ; **Variante** : Système, Clair, Sombre ; taille du texte ; ligatures de code | cet ordinateur |
| **Enregistrement** | Automatique, Avec Ctrl+S ; délai ; modifications hors d'Inkwell : Intégrer, Demander | ce projet |
| **Éditeur** | « Proposer des suggestions en tapant », « Aide sur les arguments », « Afficher le nom des arguments » (§4.37) ; retour à la ligne | cet ordinateur |
| **Écriture** (N30) | orthographe (interrupteur) ; dictionnaire du projet et personnel ; typographie française à la frappe (réglage du projet) ; voir les insécables ; vitesse de lecture ; objectif de mots ; extraits | cet ordinateur ; dictionnaire, typographie et extraits du projet : partagés |
| **IA** (page à part, N31) | utiliser l'IA ; fournisseurs et « Tester la connexion » ; texte fantôme ; autres applications (MCP) ; agents (§4.39) | cet ordinateur |
| **Moi** | nom, couleur souhaitée, empreinte ; « Me rendre invisible » | cet ordinateur |
| **Projet** (admin) | suppression des fichiers : sur accord d'un admin, librement  ; **IA dans ce projet** : Désactivée, Sur cet ordinateur seulement, Serveurs de l'organisation, Libre (§4.39) ; agents et applications permis | ce projet, signé |
| **Avancé** | Détails techniques ; langue de l'interface : Système, Français, English (chaque nom de langue porte son propre `lang`, WCAG 3.1.2) ; Mode développeur (§4.35) | cet ordinateur |
| **Extensions** (page à part) | vues **Installées · En cours** (§4.40) ; extensions de ce projet, les miennes, les intégrées ; installer, activer, autorisations, raccourcis et conflits, mettre à jour, retirer, effacer les données ; redémarrer sans extensions (§4.29 à §4.33) | cet ordinateur ; extensions de projet : par projet |

Chaque réglage s'applique tout de suite. Aujourd'hui, seuls les réglages d'enregistrement
existent (G-42).

### 5.14 Personnes

- **Ligne :** avatar, nom, « (vous) », état, rôle (sélecteur), actions.
- **Ordre :** vous, les admins, les personnes présentes, les autres.
- **Changer de rôle :** immédiat, avec le toast « Paul est en lecture · Annuler ». L'annulation
  durable passe par le sélecteur de la ligne.
- **Déconnecter** (recommandation Q3, *à confirmer*) :
  - sans confirmation, avec le toast « Paul est déconnecté · Annuler » ;
  - « **Laisser revenir** » reste sur la ligne : c'est l'annulation durable.
- **Passer la main, Retirer :** confirmation (§4.9).
- Une personne non admin voit la liste, sans contrôles.

### 5.15 Collaboration au quotidien

- **Présence :**
  - pile d'avatars, curseurs nommés ;
  - « Paul a rejoint » annoncé une fois ;
  - Suivre une personne (menu de son avatar) : la vue suit son curseur ; une touche ou un
    défilement arrête de suivre, avec un toast.
- **Divergence** (modifications concurrentes au même endroit) :
  - les deux textes sont gardés (CRDT) ;
  - marque `warn` dans la marge ;
  - toast « Paul et vous avez modifié le même passage. Les deux versions sont gardées. · Voir ».
    « Voir » ouvre la comparaison du passage.

  Jamais le mot « conflit ».
- **Rôle changé par un admin :** bandeau d'accès (lecture), ou retour de la barre (écriture),
  plus le toast « Marie vous a donné le rôle Écriture ».
- **Suspendu :** bandeau « Marie a suspendu votre accès à 14:10. Vous pouvez lire, pas
  modifier. · Demander à revenir ». Les frappes hors ligne restent chez soi, refusées tant que
  l'accès est suspendu, et intégrées au retour (ADR 0021). Le bandeau le dit : « Vos
  modifications sont gardées ».
- **Retiré** (accès révoqué, définitif) : bandeau `error`, sans fermeture : « Vous ne faites
  plus partie de ce projet. Votre copie reste lisible sur cet ordinateur. · Exporter une copie ·
  Fermer le projet ». Plus rien n'est envoyé.

### 5.16 Erreurs, hors ligne, reprise

| Situation | Composant (L-8) | Texte | Action |
|---|---|---|---|
| hors ligne < 30s | état seul | « Hors ligne — rien n'est perdu » | — |
| hors ligne ≥ 30s | bandeau `info` (information) | « Vous êtes hors ligne. Vos modifications sont gardées et partiront au retour du réseau. » | « Réessayer maintenant » |
| écriture disque impossible | bandeau `error` (critique) | « Le fichier n'a pas pu être écrit sur le disque. Votre texte reste dans Inkwell ; nouvel essai dans 10 s. » | « Réessayer », « Détails » |
| supprimé dehors | bandeau `warning` (critique) | « « ch2.typ » a été supprimé du disque. Son texte reste dans Inkwell. » | « Recréer le fichier », « Fermer » |
| devenu binaire | bandeau `warning` (critique) | « « ch2.typ » n'est plus un texte sur le disque. Inkwell garde sa version. » | « Garder la version d'Inkwell » |
| modifié dehors, mode Demander | **question** | « « ch2.typ » a été modifié hors d'Inkwell. » | « Intégrer », « Garder la version d'Inkwell » |
| erreur inattendue | toast `alert` | « Quelque chose s'est mal passé. Rien n'est perdu. » | « Détails » (rapport copiable, sans données personnelles) |

- **E-1.** Jamais le mot « conflit ».
- **E-2.** Jamais de raison technique brute dans la phrase : elle va dans « Détails ». Écart
  actuel : `fr.json:44`.
- **E-3. Reprise** (Nielsen n°9) : chaque message d'erreur dit **ce qui est gardé** et offre
  **l'action suivante**. Chaque action annulable a une annulation immédiate (toast, Ctrl+Z) et
  une annulation durable (TO-1).

### 5.17 Performance perçue

Gros fichiers et réseau lent. Source : exigence U33 et la règle « fiable et rapide ».

| Situation | Promesse | Rendu |
|---|---|---|
| frappe locale | l'écho à l'écran ne dépend **jamais** du réseau ni de l'aperçu | — |
| ouverture d'un fichier > 512 Ki caractères (`SLOW_CHARS`) | onglet ouvert tout de suite ; texte progressif | barre de progression ; bandeau `info` (3) « Fichier volumineux : les modifications des autres peuvent arriver avec un peu de retard. » |
| fichier > 2 Mi caractères (`MAX_EDITABLE_CHARS`) | lecture seule | bandeau (2) qui dit pourquoi, avec l'action « Exporter une copie » |
| aperçu lent | l'aperçu précédent reste | « Mise à jour… » après 300ms |
| WAN lent ou relais | les modifications distantes arrivent sans bloquer | état « Envoi en cours (n) » ; popover : « par un relais » |
| réception d'un projet | le fichier principal s'ouvre d'abord | progression par fichier dans Fichiers |
| recherche dans le projet | résultats au fil de l'eau | compteur qui monte, « Recherche… » |

**PERF-1.** Aucune action d'interface NE DOIT bloquer la frappe. Toute opération de plus de
100ms quitte le fil de l'interface, ou se découpe (`whenIdle`, déjà employé par la barre
d'état).

### 5.18 À propos

`AdwAboutDialog` [S1] : icône, nom, version, licence, crédits. La révision n'apparaît que sous
« Détails techniques ».

---

### 5.19 Modèles de projet (N27)

**Principe :**
- **Ce sont les personnes qui créent les modèles.** Inkwell n'en fournit aucun.
- Un modèle est **un projet**, gardé comme point de départ.
- On reprend les conventions existantes.

**Format, celui de Typst** [S20]. Un modèle est un dossier de projet avec un `typst.toml` :
- `[package]` : nom, version, description ;
- `[template]` : `path`, `entrypoint`, `thumbnail` (PNG ou WebP, plus grand côté ≥ 1080px). C'est
  le format de Typst Universe, que `typst init` sait employer ;
- `[tool.inkwell]`, la section réservée aux outils : type, étiquettes, polices requises, charte,
  format (`typst`, ou `markdown` + style CSS assaini, B8).

Repères : les diapositives importent **Touying**, fourni (Polylux possible si le projet le
fournit) ; un modèle Markdown + CSS ressemble aux formats personnalisés de Quarto [S21], sans
adopter Quarto.

**Charte partagée** : une charte est un **paquet Typst local** (`@local/charte-acme:1.0.0`, copié
dans `.typstdeps/`). « Diapositives ACME » et « Document officiel ACME » l'importent tous deux.
- **Le contenu de départ est copié** à la création, comme `typst init` : modifier un modèle ne
  touche aucun projet existant.
- **La charte est une dépendance versionnée.** Quand une version plus récente est disponible, le
  bandeau d'information dit « La charte ACME a une nouvelle version (1.1). · Mettre à jour ». Le
  changement est **une** modification annulable ; rien n'est automatique.
- Paquets et charte sont **dans le projet** : il se compile chez tous, même hors ligne.

**Où vivent les modèles :**
- **Mes modèles** : `Documents/Inkwell/Modèles/`, sur cet ordinateur.
- **Modèles de l'équipe** : un projet de modèles partagé comme tout projet. **Ajouter un modèle y
  demande le rôle Écriture dans ce projet**, rien de plus (Fable m8).

**Galerie, dans « Nouveau projet »** (§5.2), section « Partir d'un de mes modèles » :
- **Choisir un modèle = un groupe radio de cartes** (APG *Radio Group* : les flèches passent d'une
  carte à l'autre dans l'ordre de lecture, Espace choisit). Une carte ne contient **aucun autre
  contrôle** : on n'y met pas de menu (Codex R3-M4).
- Chaque carte montre :
  - la vignette (« Aperçu de la première page de Rapport ACME ») ;
  - le titre ;
  - une phrase de description ;
  - **ce que vous obtenez** : type, charte (pastilles et nom), polices (« fournie » ou
    « **manquante** », avec `warn` et le mot) ;
  - les étiquettes ;
  - la provenance.
- **Filtres** (type, recherche) au-dessus. **Détail** de la carte choisie à côté : grande page
  d'exemple (jusqu'à 3 pages), puis « Utiliser ce modèle ».
- **Écran vide** : « Vous n'avez pas encore de modèle. Préparez un projet comme vous l'aimez
  (charte, page de garde, styles), puis choisissez **Projet › Enregistrer comme modèle…**. Il
  apparaîtra ici. · Comment faire ».

**Enregistrer comme modèle** (menu Projet) : **un seul dialogue**, `medium` (Codex R3-m2,
Fable m8).
- Visibles :
  - **nom** (obligatoire) ;
  - **une phrase de description**, dans la langue de la personne (aucune traduction exigée) ;
  - **emplacement** : « Mes modèles », ou un projet d'équipe où l'on a le rôle Écriture.
- **Automatiques** :
  - vignette tirée de la page 1 de l'aperçu ;
  - type deviné (Touying → « Diapositives ») ;
  - polices et charte détectées ;
  - tous les fichiers inclus.
- **« Options avancées »**, repliées :
  - type et étiquettes ;
  - fichiers inclus ;
  - « Remplacer le texte par un texte d'exemple » ;
  - autre page pour la vignette ;
  - « Enregistrer aussi la charte séparément » ;
  - fonctions de la charte sans documentation (§4.37), à titre d'information.
- « Enregistrer » ; toast « Modèle « Rapport ACME » enregistré · Voir ».

**États de l'enregistrement** (Codex R3-M7) :

| Cas | Ce qu'on voit | Saisie |
|---|---|---|
| Annuler, ou fermer le dialogue | rien n'est créé, aucun modèle partiel | perdue (rien à garder) |
| Aucun aperçu (erreur de compilation) | « Pas de vignette : le document a des erreurs. Une vignette neutre (nom et type) est utilisée. · Aller au premier problème » ; l'enregistrement reste possible | gardée |
| Disque plein, droits refusés | « Le modèle n'a pas pu être écrit dans Documents/Inkwell/Modèles : disque plein. · Réessayer » | gardée |
| Rôle Écriture perdu pendant l'enregistrement d'équipe | « Vous ne pouvez plus écrire dans « Équipe ACME ». · Enregistrer dans Mes modèles » | gardée |
| Fichier importé exclu dans les options avancées | avertissement sur la ligne : « main.typ importe chapitre.typ : le modèle ne se compilera pas sans lui » | — |

**Gérer : écran Projets › vue Modèles.** Une **liste** (`role="list"`) de cartes. Chaque carte a
**un** bouton « Plus d'actions » (*Menu Button* APG), atteignable par Tab ; Maj+F10 l'ouvre
aussi.

| Action | Comportement |
|---|---|
| Modifier | ouvre le modèle comme un projet ; « Enregistrer la nouvelle version » incrémente la version ; les projets existants ne changent pas |
| Renommer | champ en place (§4.13) |
| Dupliquer | « Copie de … », ouverte en mode Modifier |
| Supprimer | toast « Modèle supprimé · Annuler » ; **même corbeille que les fichiers** (§5.6), sans durée propre |
| Exporter… | un `.zip` du dossier avec son `typst.toml` (réutilisable par `typst init`) |
| Importer… | depuis un `.zip` ou un dossier ; un manifeste invalide est refusé avec la raison |
| Partager avec l'équipe… | n'apparaît qu'avec le rôle Écriture dans un projet de modèles ; sinon, la phrase « Demandez le rôle Écriture à un admin de « Équipe ACME » » |

« Voir les projets créés avec ce modèle » PEUT venir en v2 : il faut tracer la provenance dans
chaque projet (Fable m8).

## 6. Contenu et ton

### 6.1 Voix

- Vouvoiement, voix active, présent, phrases de 20 mots au plus.
- Pas de « ! », ni « Oups », ni « Désolé ».
- Dire ce qui est préservé.
- Nommer les personnes par leur nom.

### 6.2 Forme

| Élément | Règle | Exemple |
|---|---|---|
| bouton, élément de menu | verbe à l'infinitif, majuscule initiale, sans point | « Enregistrer une version » |
| titre de dialogue | nom ou question | « Retirer Paul ? » |
| bandeau, toast | une phrase | « Vous êtes hors ligne. » |
| infobulle | nom, puis syntaxe, puis raccourci | « Gras `*texte*` Ctrl+B » |
| « … » (U+2026) | seulement si l'action demande une saisie de plus | « Exporter… » |

En français, la majuscule ne se met qu'en début de phrase ; `en.json` suit la *header
capitalization* de la HIG [S2].

### 6.3 Typographie française et inclusivité

- **Espaces** [S9] :
  - U+202F avant `; ! ?` ;
  - U+00A0 avant `:`, dans « », et entre un nombre et son unité.

  Écart : 79 messages (G-46). Un test vérifie.
- **Nombres et dates :** `Intl` [S10], jamais de pluriel écrit à la main : « aujourd'hui
  14:10 », « hier 17:42 », « 3 nov. 14:10 ».
- **Accord neutre :** reformuler plutôt qu'accorder.

  | Éviter | Écrire |
  |---|---|
  | « Seul » | « Sur cet ordinateur » |
  | « {name} vous a déconnecté » | « {name} a suspendu votre accès » |
  | « Vous êtes connecté » | « Connexion établie » |
- **Longueur :** +30 % en anglais. Aucun libellé dans une largeur fixe.

### 6.4 Vocabulaire figé

Il complète le glossaire.

| Concept | Interface | Jamais |
|---|---|---|
| inviter | **Partager** (bouton), **Inviter** (action) | héberger, host |
| entrer dans un projet | **Rejoindre** | se connecter, join |
| couper une personne, réversible | **Déconnecter** / **Laisser revenir** | bannir |
| retirer définitivement | **Retirer** | supprimer une personne |
| version gardée | **Enregistrer une version**, **version nommée** | commit, tag |
| retour en arrière | **Revenir à ce moment** | restaurer, rollback |
| annuler un effet | **Annuler** | défaire |
| garder le texte local | **Garder la version d'Inkwell** | ignorer, écraser |
| incorporer le texte du disque | **Intégrer** | importer, fusionner |
| réunir une variante | **Intégrer la variante** | fusionner, merge |
| rendu | **Aperçu** | preview |
| moteur | **mise en page** | compilation, typst, tinymist |
| erreur du document | **problème** | diagnostic |
| fichiers ouverts | **onglets** / « Fichiers ouverts » | buffers |
| demande en attente | **demande** | requête |
| greffon | **extension** | plugin, module, WASI, bac à sable |
| template | **modèle** | template, gabarit, squelette |
| identité visuelle partagée | **charte** | thème (réservé à l'apparence d'Inkwell), brand kit |
| diagnostic | **problème** : erreur, avertissement, conseil | diagnostic, lint, warning |
| quick fix | **correction rapide** | code action |
| droit d'une extension | **ce qu'elle peut faire**, **autorisation** | capacité, permission système |
| fichier de code | « Python », « JavaScript »… · **lecture** / **Modifier** | IDE, LSP, linter |
| proposition d'IA | **proposition**, « via IA », « marqué IA par Camille » | « généré », « écrit par l'IA » |
| fournisseur d'IA | **fournisseur**, « serveur de votre entreprise » | endpoint, LLM, API |
| consommation d'une extension | **mémoire**, **calcul**, **délai** | CPU, RAM, PID, processus |
| suggestion d'écriture | **suggestion**, **Relecture** | lint, score |

---

## 7. Accessibilité

WCAG 2.2 AA. Le relevé critère par critère des 55 critères A et AA est dans
[`coverage.md`](coverage.md) §2. Exigences transverses :

| # | Exigence | Critères |
|---|---|---|
| A-1 | Texte ≥ 4,5:1, grand texte ≥ 3:1, en clair, en sombre et en HC, sur le fond parent déclaré. | 1.4.3 |
| A-2 | Composants, états, icônes porteuses de sens, **anneaux de focus sur toutes les surfaces** (y compris sombres) ≥ 3:1. | 1.4.11 |
| A-3 | Rien n'est porté par la couleur seule. | 1.4.1 |
| A-4 | Tout au clavier, sans piège ; ordre de tabulation = ordre visuel ; motifs APG. | 2.1.1, 2.1.2, 2.4.3 |
| A-5 | Focus visible, jamais couvert : ni par un toast, ni par un bandeau, ni par un popover, ni par une infobulle, ni par un panneau superposé. | 2.4.7, 2.4.11 |
| A-6 | Anneau de 2px à 3:1, ce qui vise aussi 2.4.13 (AAA). | 2.4.13 |
| A-7 | Cibles : 24px au minimum ; 34px dans les barres, 44px dans les dialogues. | 2.5.8 |
| A-8 | Une alternative à chaque glisser. | 2.5.7 |
| A-9 | Infobulles et popovers : fermables, survolables, persistants. | 1.4.13 |
| A-10 | Messages d'état par des régions persistantes, sans voler le focus, au plus une annonce par seconde et par sujet. | 4.1.3 |
| A-11 | Reflow à 320px, zoom 200 %, espacement du texte. | 1.4.10, 1.4.4, 1.4.12 |
| A-12 | Mouvement réduit ; rien qui clignote. | 2.3.1, 2.3.3 (AAA) |
| A-13 | Les toasts se suspendent au survol et au focus, **et l'annulation reste disponible après** (TO-1). | 2.2.1 |
| A-14 | `<html lang>` suit la langue de l'interface. | 3.1.1 |
| A-15 | Aide et raccourcis toujours au même endroit du menu. | 3.2.6 |
| A-16 | Pas de saisie redondante ; l'empreinte se compare, elle ne se retape pas. | 3.3.7, 3.3.8 |
| A-17 | `forced-colors` : tous les états restent visibles. | — |
| A-18 | Repères et F6 : panneau de gauche → onglets → barre de mise en forme → texte → aperçu → tiroir → toasts → barre d'état. | 1.3.1, 2.4.1 |
| A-19 | L'éditeur est nommé (« Texte de main.typ ») ; la présence est annoncée en `log`, sans les curseurs. | 4.1.2 |
| A-20 | Raccourcis d'une seule touche (B, W, L en présentation, `present/keys.ts:61-69` ; lettres dans un arbre ou une liste) : actifs seulement quand leur composant a le focus. | 2.1.4 |
| A-21 | Tout ce qu'une extension affiche est rendu par l'hôte en composants Adwaita (§4.31) et suit ces exigences ; une vue libre (`ui.canvas`) est un cadre nommé, et son contenu interne est de la responsabilité de l'extension, signalé comme tel. | 4.1.2, 1.4.3 |
| A-22 | Un thème ne peut pas faire baisser un contraste sous les seuils (TH-D2). | 1.4.3, 1.4.11 |

---

## 8. Cohérence

### 8.1 Règles

- **K-1.** Avant de créer un composant, chercher en §4 ; si rien ne convient, compléter la spec
  d'abord.
- **K-2.** Un panneau entre dans l'un des deux sélecteurs (L-2). Un réglage entre dans un
  groupe de §5.13. Une action entre dans le registre des commandes.
- **K-3.** Captures en clair et en sombre, à 100 % et 200 %, en HC, à 360px ; comparées aux
  écrans voisins.
- **K-4.** Textes dans `fr.json` et `en.json`, à parité, sans texte en dur.
- **K-5.** Un verbe = une action ; un raccourci = une action dans un même contexte.

### 8.2 Registre des commandes et raccourcis

Une seule source alimente les menus, les infobulles, la palette, l'aide et la fenêtre des
raccourcis. `Ctrl` vaut `Cmd` sur macOS.

| Action | Raccourci | État |
|---|---|---|
| Enregistrer (mode Ctrl+S) | Ctrl+S | fait |
| Rechercher / Remplacer / Projet | Ctrl+F / Ctrl+H / Ctrl+Maj+F | codé, non branché |
| Suivant, précédent | F3 / Maj+F3 | codé |
| Gras, italique, souligné | Ctrl+B, Ctrl+I, Ctrl+U | fait |
| Lien | Ctrl+K | fait (Q1 : reste) |
| Titres 1–4, paragraphe | Ctrl+1…4, Ctrl+Alt+0 | fait |
| Barre de mise en forme | Ctrl+Maj+T | fait |
| Palette | Ctrl+Maj+P | à faire |
| Aller au fichier | Ctrl+P | à faire |
| Aller à la ligne | Ctrl+G | à faire |
| Sommaire, filtrer | Ctrl+Maj+O | à faire (B10) |
| Problème suivant, précédent | F8 / Maj+F8 (convention VS Code ; F8 est libre) | à faire |
| Corrections rapides | **Alt+Entrée** (pas Ctrl+., réservé par GTK) | à faire |
| Demander à l'IA sur la sélection | Ctrl+Alt+I | à faire (N31) |
| Mode concentration | Ctrl+Alt+F | à faire (N30) |
| Plein écran | F11 | à faire (convention GNOME) |
| Caractère spécial | Ctrl+Alt+C | à faire (pas Ctrl+;) |
| Suggestions | Ctrl+Espace | à faire (N29) |
| Aide sur la fonction (signature ou carte) | Ctrl+Maj+Espace | à faire |
| Aller à la définition | F12 | à faire (libre dans le registre) |
| Panneau de gauche | F9 | à faire |
| Historique | Ctrl+Maj+H | à faire |
| Aperçu | Ctrl+Maj+Entrée | à faire (*à confirmer*) |
| Source → aperçu | Ctrl+Alt+J | fait (Alt : convention de tinymist, écart HIG assumé) |
| Nouveau fichier | Ctrl+N | à faire |
| Fermer l'onglet | Ctrl+W | à faire |
| Onglet suivant, précédent | Ctrl+Page↓ / Page↑, Ctrl+Tab | à faire |
| Déplacer l'onglet | Ctrl+Maj+Page↓ / Page↑ | à faire |
| Renommer | F2 | à faire |
| Exporter | Ctrl+Maj+E | à faire (vérifier qu'il ne heurte pas `Mod-Shift-e` de la barre) |
| Présenter, depuis ici | F5 / Maj+F5 | fait |
| Différence suivante | F7 / Maj+F7 | fait |
| Zoom du texte | Ctrl+= / Ctrl+- / Ctrl+0 | fait |
| Navigation arrière, avant | Alt+← / Alt+→ | à faire |
| Menu principal | F10 | à faire |
| Menu contextuel | Maj+F10, touche Menu | tableur seulement |
| Zones | F6 / Maj+F6 | à faire |
| Projets | Ctrl+O | à faire |
| Préférences | Ctrl+, | à faire |
| Aide | F1 | à faire |
| Raccourcis | Ctrl+? | à faire |

**K-6.** Le registre DOIT refuser, par un test, deux actions sur un même raccourci dans un même
contexte, **et tout raccourci de la liste réservée par la plate-forme**. Ctrl+Maj+E est déjà pris
par la barre de mise en forme (`ui/toolbar/spec.ts`) : à départager avant le code.

**Raccourcis réservés par la plate-forme** (Fable R3-M2). Inkwell ne les prend jamais :

| Raccourci | Pris par | Source |
|---|---|---|
| Ctrl+., Ctrl+; | sélecteur d'émojis de GTK 4 (`GtkText::insert-emoji`) ; WebKitGTK ne les reçoit pas | [S25] |
| Ctrl+Maj+U | saisie Unicode (IBus, GTK) | IBus |
| Ctrl+Maj+I | inspecteur web de WebKitGTK (s'il est activé) | WebKitGTK |
| Super+…, Alt+Tab, Alt+F2, Alt+F4, Ctrl+Alt+flèches | GNOME Shell, gestionnaire de fenêtres | HIG *Keyboard* [S2] |
| Ctrl+Espace | **à risque** : ancien déclencheur d'IBus, encore réglé chez des personnes qui écrivent en chinois, japonais ou coréen | IBus |

Ctrl+Espace reste la complétion (convention de CodeMirror et VS Code). Un e2e sous WebKitGTK
vérifie qu'il arrive à la page. S'il est pris, la complétion reste accessible par **Alt+/**, qui
figure aussi dans la fenêtre des raccourcis.

### 8.3 Icônes par idée

| Icône | Idée |
|---|---|
| `side` | panneau de gauche |
| `eye` | Aperçu |
| `branch` | Historique |
| `present` | Présenter |
| `menu` | menu principal |
| `more` | Plus |
| `close` | Fermer, fermer un onglet |
| `check` | coche |
| `fail` | erreur |
| `warn` | avertissement, lien cassé, divergence |
| `undo` | Annuler |
| `copy` | Copier |
| `link` | lien, Liens |
| `outline` | Sommaire |
| `task` | Tâches |
| `at` | personne citée |
| `folder` | Fichiers |
| `puzzle` | Extensions, attribution d'une extension |
| `shield` | contexte qui contient le texte d'autres personnes (IA) |
| `pin` | épingler à la barre |

Toute nouvelle icône est ajoutée ici.

---

## 9. Anti-motifs

| Anti-motif | Règle | À la place |
|---|---|---|
| « Êtes-vous sûr ? » pour une action annulable | P3 | un toast « Annuler » et une annulation durable |
| un toast qui pose une question | TO-2 | un bandeau-question |
| une question cachée derrière un état | L-8 | les questions d'abord |
| des bandeaux empilés, ou un message masqué sans accès | L-8, L-8b | un bandeau et « +N autres » |
| une région live montée en même temps que son texte | §4.11 | des régions persistantes |
| une raison technique dans la phrase | E-2 | « Détails » |
| `title=` comme infobulle | TT-2 | le composant Infobulle |
| une valeur brute, en CSS ou dans un thème CodeMirror en TS | T-1 | un jeton |
| du texte secondaire sur une sélection, ou de l'`opacity` sur du texte | C-SEL, §2.2 | `--fg` |
| un anneau normal sur un fond sombre | §2.2 | `--focus-ring-on-dark` |
| un contrôle grisé sans raison | §4 « désactivé » | le masquer, ou une infobulle « pourquoi » |
| un dialogue avec son propre CSS | P2 | `Dialog.svelte` |
| un focus rendu ailleurs qu'au déclencheur | §4 | le déclencheur |
| une barre d'outils, une liste ou des onglets sans flèches | APG | tabindex itinérant |
| un menu pour choisir une valeur | §4.5 | un `<select>` |
| un onglet de fonction absente | L-2b | le masquer |
| un même fichier dans deux onglets | §4.25 | activer l'onglet existant |
| une question à la fermeture | SAV-03 | rien : tout est gardé |
| le réseau qui ralentit la frappe | PERF-1 | l'écho local d'abord |
| du jargon (pair, commit, conflit, fusion, CRDT, hash) | P5 | le glossaire |
| une extension qui s'épingle, se met dans une zone protégée ou imite un dialogue d'accord | §4.30, §4.33 | la personne épingle ; zones protégées ; bande d'attribution |
| un raccourci d'extension qui en prend un du cœur en silence | ND-3 | conflit montré, la personne tranche |
| un thème appliqué à moitié, ou qui affaiblit le contraste élevé | TH-D2 | refus complet, avec la raison |
| une extension de projet qui agit sans le geste de la personne | PP-2 | sur commande seulement |
| une erreur de diagramme dans un toast | §4.34 | en ligne, sous le bloc |
| un mode développeur discret | §4.35 | une pastille rouge permanente |

---

## 10. Preuves

Une règle sans assertion n'est pas finie.
- **Existant** : le fichier existe aujourd'hui, et la ligne citée le prouve.
- **À écrire** : c'est un test planifié. Son nom est réservé ; il n'est pas une preuve tant
  qu'il n'existe pas.

`coverage.md` ne met l'état S que sur une preuve **existante**.

| Règles | Assertion | Fichier | État |
|---|---|---|---|
| syntaxe ≥ 4,5:1 (§2.2) | 9 couleurs `--hl-*` en clair et en sombre | `frontend/src/theme/tokens.test.ts:44-50` | existant |
| A-1, A-2 (coque) | axe-core en clair et en sombre sur la fenêtre | `frontend/e2e/shell.spec.ts:253` (UI-06) | existant (coque seulement) |
| M-1, §4.2 (menu principal) | flèches et Échap dans le menu | `frontend/e2e/shell.spec.ts:24` (UI-03) | existant ; le retour du focus à l'éditeur y est **attendu**, à changer avec G-25 |
| UI-07 | encodage masqué en UTF-8 + LF | `frontend/e2e/shell.spec.ts:151` | existant |
| parité fr/en | les clés des deux langues | `frontend/src/ui/i18n.test.ts`, `frontend/src/i18n/i18n.test.ts` | existant (parité seulement) |
| 2.1.2 | `showModal()` dans les sept dialogues | `ui/ShareDialog.svelte:12`, `ui/AboutDialog.svelte:21`, `ui/prefs/Preferences.svelte:21`, `ui/people/PeopleDialog.svelte:21`, `ui/history/ApplyDialog.svelte:19`, `ui/sheet/PasteDialog.svelte:13`, `ui/history/DiffView.svelte:47` | existant (mécanisme natif ; sans test de piège) |
| 2.2.2, T-11 (en partie) | animations infinies coupées sous `reduce` | `ui/shell/HeaderBar.svelte:293`, `preview/Preview.svelte:306`, `ui/sheet/Grid.svelte:1025` | existant (CSS) ; test à écrire |
| T-1 (CSS) | stylelint : pas de valeur brute dans `*.svelte` | `mise run lint` | **à écrire** |
| T-1 (TS) | pas de couleur ni d'ombre brute dans les thèmes CodeMirror (`editor/*.ts`) | `editor/theme.lint.test.ts` | **à écrire** |
| T-5, §2.2, C-SEL, C-GUT | parité `tokens.json` ↔ `tokens.css` ; paires non interdites ≥ `min` dans les quatre modes ; paires interdites absentes du code | `theme/tokens.test.ts` (extension) | **à écrire** |
| TH-D1 à TH-D3 | validation d'un thème : refus complet sur une paire en échec (cas : bon en clair, mauvais en sombre ; HC affaibli ; ΔE trop faible) ; ≤ 20ms | `theme/contract.test.ts` | **à écrire** |
| T-2 | sous WebKitGTK, couleur calculée en clair puis en sombre | `frontend/e2e/theme.spec.ts` | **à écrire** |
| A-1, A-2, A-17 | axe-core sur chaque écran de §5, en sombre et sous `forced-colors`, avec bandeau, toast et popover ouverts ensemble | `frontend/e2e/a11y.spec.ts` | **à écrire** (étend UI-06) |
| L-8 à L-9, TO-1 | file de `BannerHost` et `ToastHost` : état critique avant question, question avant état, « +1 question », remplacement, pause, annulation durable après expiration | `ui/messages.test.ts` | **à écrire** |
| §4.11, A-10 | régions présentes avant le premier message ; passe Orca à chaque vague | `ui/messages.test.ts` + `ux-walkthrough` | **à écrire** |
| A-4, F-1, SB-1, §4.2, §4.5, §4.22, §4.25 | motifs APG au clavier (`userEvent.keyboard`) | `*.test.ts` par composant | **à écrire** |
| §4 « Retour du focus » | retour au déclencheur, puis les quatre replis | `ui/focus.test.ts` | **à écrire** |
| A-5, TT-1 | l'infobulle, le toast ou le bandeau ne recouvrent ni l'élément focalisé ni son anneau | `frontend/e2e/focus.spec.ts` | **à écrire** |
| A-7, §2.5 | tout élément focalisable ≥ 24px ; zones des barres ≥ 34px ; dialogues ≥ 44px | `frontend/e2e/targets.spec.ts` | **à écrire** |
| A-11, L-5 | 320px CSS sans défilement horizontal, dans les quatre formats ; captures 360×480 et 1280×800 à 200 % | `frontend/e2e/reflow.spec.ts` | **à écrire** |
| T-11, A-12 | `reducedMotion: "reduce"` : seuls des fondus ≤ 150ms en cours | `frontend/e2e/motion.spec.ts` | **à écrire** |
| §6.3, §6.4, K-4 | espaces insécables ; « … » ; mots interdits absents | `ui/i18n.test.ts` (extension) | **à écrire** |
| K-6, ND-3 | aucun doublon de raccourci ; conflit d'extension signalé, raccourci inactif | `commands.test.ts` | **à écrire** |
| G-0, §4.16 | clic et Entrée ouvrent le fichier, onglet actif, focus dans l'éditeur | `frontend/e2e/files.spec.ts` | **à écrire** |
| PERF-1, §5.17 | latence de frappe à 512 Ki et 2 Mi caractères ; frappe locale < 50ms pendant une synchronisation lente (netsim) | `frontend/e2e/perf.spec.ts` | **à écrire** (`e2e/netsim.spec.ts` existe, sans mesure de frappe) |
| §4.29 à §4.35 | consentement, états du gestionnaire, épinglage par la personne seulement, attribution par point, mode sans extensions, repère du mode développeur, repli d'une extension de projet, erreur de diagramme en ligne ; densité d'attribution à 360px | `frontend/e2e/extensions.spec.ts` + tests d'API de l'architecture (`EXT-ND1` à `ND8`) | **à écrire** |
| TH-D2 (rév. 4) | états interactifs (`--hover-bg` noir refusé), planchers `hcMin` (thème « papier » accepté, `--fg-dim` HC à 55 % refusé), `hcOnly` (`--border` HC < 3:1 refusé), contrat `^2.0` refusé, valeur hors bornes refusée | `theme/contract.test.ts` | **à écrire** |
| K-6 (liste réservée) | aucun raccourci du registre dans la liste réservée ; sous WebKitGTK, Ctrl+Espace atteint la page, sinon Alt+/ | `commands.test.ts`, `frontend/e2e/keys.spec.ts` | **à écrire** |
| §4.34 | chaque diagramme a une description, présente dans `image(alt:)`, dans l'export HTML et dans la Vue lecture ; l'absence est comptée en avertissement | `e2e/diagrams.spec.ts` | **à écrire** |
| §4.36, §4.37 (rév. 4) | carte au clavier (entrer, Tab, Échap) ; correction périmée grisée et jamais appliquée ; diagnostic sans position ; service indisponible sans indicateur permanent | `editor/diagnostics.test.ts`, `editor/assist.test.ts` | **à écrire** |
| §4.38 | orthographe en pointillé, jamais comptée ; Relecture seulement sur « Relire » ; dictionnaire du projet partagé entre deux instances ; typographie à la frappe annulable en un pas ; aucun score affiché | `e2e/writing.spec.ts` (deux instances) | **à écrire** |
| §4.39 | rien n'est envoyé sans profil ; dialogue d'envoi avec les passages d'autrui marqués ; contexte « autrui » → outils retirés (test de propriété IA-5) ; proposition par morceau, une modification « via IA », Ctrl+Z ; rien ne part chez les collègues avant « Accepter » ; repère des connexions MCP et agents | `e2e/ai.spec.ts` (fournisseur factice compatible OpenAI), `ext/ai/*.test.ts` | **à écrire** |
| §4.40 | table APG *Grid* triable ; annonce seulement sur changement d'état ; extension lente de test → toast « Voir » unique, aucun dialogue ; service relançable, jamais désactivable | `e2e/resources.spec.ts` | **à écrire** |
| §5.19 (rév. 4) | dialogue unique ; annuler ne crée rien ; vignette de repli ; perte du rôle Écriture → « Enregistrer dans Mes modèles » | `e2e/templates.spec.ts` | **à écrire** |
| §5 parcours | cinq tâches chronométrées, personas | `docs/ux/walkthrough-*.md` | existant pour les personnes et la présentation (2026-10-26) ; à refaire |

## 11. Décisions à confirmer par l'utilisatrice

**Rien de ce tableau n'est ratifié.** Les amendements de `ux-principles.md` (règle 3 : palette ;
règle 9 : cibles) et du glossaire (« variante » au lieu de « brouillon ») restent **en attente**.
Ces documents ne sont pas modifiés tant que l'utilisatrice n'a pas tranché (Codex R2-m2). Les
constats C12, F5 et F8 de la revue n°1 sont donc fermés **en spécification**, avec une décision
en attente.

Les deux revues convergent sur chacune de ces recommandations.

| # | Question | Recommandation (*à confirmer par l'utilisatrice*) |
|---|---|---|
| Q1 | Ctrl+K : palette ou lien ? | Ctrl+K reste le **lien**. Palette sur **Ctrl+Maj+P**, plus « Rechercher une commande… » dans le menu. Amender la règle 3 de `ux-principles.md` (§4.24). |
| Q2 | Le titre de la barre d'en-tête | Le **nom du projet** ; le fichier en sous-titre (HIG). |
| Q3 | Déconnecter | Un toast « Annuler », **et** « Laisser revenir » sur la ligne comme annulation durable. |
| Q4 | Couleur de personne | Choisie à l'accueil ; créneau libre par défaut. **Le créneau du projet fait foi** ; la préférence locale est un souhait ; les collisions sont réconciliées par l'ordre de la liste signée (C-PERSON-2). |
| Q5 | Recherche dans le projet | Un onglet du **tiroir**, en cohérence avec B10 : la structure à gauche, le temps et les autres à droite (§3.1). |
| Q6 | S'écarter de libadwaita pour l'anneau | Oui, 1.4.11 l'exige, avec la variante pour fond sombre. |
| Q7 | Langues | fr et en, propriétés logiques dès maintenant, pas de RTL pour l'instant. |
| Q8 | Commentaires | Hors de N17 ; onglet masqué ; un plan dédié et une ADR avant toute interface (§4.27). |
| Q9 | Amendements de `ux-principles.md` et du glossaire | règle 9 (cibles, §2.5) ; règle 3 (palette, Q1) ; « variante » au lieu de « brouillon » (§4.22). |
| Q10 | Imprimer | Par l'export PDF et la boîte d'impression du système, sans raccourci (§5.12). |
| Q11 | Fichiers de code | Ouverts **en lecture**, « Modifier » sur demande, mémorisé par fichier ; coloration avec les 10 jetons existants ; `language-data` paresseux, avec une liste blanche d'environ 25 langages (§4.28). |
| Q13 | Thèmes (N22) | Thème et variante choisis séparément dans Apparence ; refus complet d'un thème qui échoue dans un mode ; aucun thème imposé par un projet (§2.10). |
| Q14 | Extensions de projet (N23) | Bandeau de provenance non bloquant ; mode restreint pour ce qui dépasse le niveau « projet » ; « Désactiver pour moi » (§4.32). |
| Q15 | Développement (N25) | Mode développeur local, signalé par une pastille rouge permanente ; Inspecteur dans le tiroir (§4.35). |
| Q16 | Mises en page Markdown fournies (T11 B8 : « Note », « Rapport »…) contre N27 (« aucun modèle fourni ») | Les mises en page B8 restent des **mises en page** d'un fichier `.md` (§5.9), pas des modèles de projet. Garder les styles « Adwaita » et « Lecture » comme valeurs par défaut lisibles ; retirer « Rapport » et « Lettre » de B8 si l'utilisatrice veut aucun contenu fourni. |
| Q18 | Liste des problèmes : popover de la barre d'état (ce design, §4.36) ou onglet « Problèmes à corriger » du tiroir (architecture ER-1) ? | Le popover : il existe, et les deux revues l'ont approuvé ; voir `CHANGES.md`, « Désaccords à débattre ». |
| Q19 | IA (N31) | Onglet IA du tiroir ; propositions par morceau ; dialogue d'envoi du cœur ; désactivée dans les projets partagés jusqu'à PD-1 ; MCP et agents désactivés par défaut (§4.39). |
| Q20 | Outils d'écriture (N30) | Orthographe discrète à la frappe ; tout le reste dans Relecture, à la demande ; aucun score ; objectifs locaux sans série (§4.38). |
| Q21 | Ressources (N32) | Vue « En cours » de Préférences › Extensions ; toasts « Voir », jamais de dialogue (§4.40). |
| Q17 | Compte des problèmes dans la barre d'état | Bouton partagé : un clic va au premier problème, ⌄ ouvre la liste (§4.36). |
| Q12 | Extensions | Mot « extension » ; gestion dans Préférences › Extensions ; consentement en phrases simples, révocable ; jamais d'installation venue d'une autre personne (§4.29). |

---

## 12. Sources

Consultées le 2026-11-01. « Non revérifié » : la valeur vient du source de libadwaita, pas de sa
documentation en ligne.

- [S1] libadwaita, documentation `main` (≥ 1.8) :
  - https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/css-variables.html ;
  - `style-classes.html`, `adaptive-layouts.html` ;
  - `class.Toast.html`, `class.Banner.html`, `class.Dialog.html`, `class.AlertDialog.html` ;
  - `class.OverlaySplitView.html`, `class.Spinner.html`, `class.ToggleGroup.html` ;
  - `class.ViewSwitcher.html`, `class.TabBar.html`, `class.StatusPage.html` ;
  - `class.PreferencesDialog.html`, `class.AboutDialog.html`.
- [S2] GNOME HIG : https://developer.gnome.org/hig/
  - `patterns/containers/header-bars.html`, `popovers.html` ;
  - `patterns/feedback/banners.html`, `toasts.html`, `dialogs.html`, `placeholders.html`,
    `spinners.html` ;
  - `patterns/nav/tabs.html`, `patterns/nav/view-switchers.html` ;
  - `guidelines/adaptive.html`, `keyboard.html`, `writing-style.html`, `accessibility.html`.
- [S4] WCAG 2.2 : https://www.w3.org/TR/WCAG22/ ; Understanding :
  https://www.w3.org/WAI/WCAG22/Understanding/
- [S5] WAI-ARIA APG : https://www.w3.org/WAI/ARIA/apg/patterns/ (dialog-modal, alertdialog,
  menu-button, menubar, toolbar, tabs, treeview, listbox, grid, combobox, switch, tooltip,
  windowsplitter, button).
- [S6] MDN : `light-dark()`, `prefers-reduced-motion`, `prefers-contrast`, `forced-colors`,
  `color-scheme` (https://developer.mozilla.org/en-US/docs/Web/CSS/…).
- [S7] Design Tokens Community Group : https://www.designtokens.org/
- [S8] Okabe & Ito, *Color Universal Design* : https://jfly.uni-koeln.de/color/
- [S9] https://fr.wikipedia.org/wiki/Espace_ins%C3%A9cable ;
  https://fedoraproject.org/wiki/L10N_French_Team
- [S10] MDN `Intl` : `DateTimeFormat`, `RelativeTimeFormat`, `PluralRules`.
- [S11] Technique WCAG ARIA22, « Using role=status to present status messages » :
  https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA22
- [S12] Nielsen, *10 Usability Heuristics for User Interface Design* :
  https://www.nngroup.com/articles/ten-usability-heuristics/
- [S13] CodeMirror :
  - `@codemirror/language-data` : https://github.com/codemirror/language-data ;
  - `LanguageDescription` (`matchFilename`, `matchLanguageName`, `load`) :
    https://codemirror.net/docs/ref/#language.LanguageDescription ;
  - option `codeLanguages` : https://github.com/codemirror/lang-markdown ;
  - `@codemirror/legacy-modes` : https://github.com/codemirror/legacy-modes.
- [S14] Lezer, étiquettes de coloration : https://lezer.codemirror.net/docs/ref/#highlight.tags ;
  `codemirror-lang-typst` : https://www.npmjs.com/package/codemirror-lang-typst ; jetons
  sémantiques de tinymist : https://github.com/Myriad-Dreamin/tinymist
- [S15] GNOME Extensions (application et site) : https://apps.gnome.org/Extensions/ ;
  https://extensions.gnome.org/
- [S16] Firefox, bouton Extensions unifié (version 109) :
  https://support.mozilla.org/kb/extensions-button
- [S17] VS Code, *Workspace Trust* (mode restreint) :
  https://code.visualstudio.com/docs/editing/workspaces/workspace-trust
- [S19] tinymist (diagnostics, conseils, actions de code par le LSP) :
  https://github.com/Myriad-Dreamin/tinymist ; LSP 3.17, `textDocument/codeAction`,
  `publishDiagnostics` :
  https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/
- [S20] Manifeste des paquets et modèles Typst (`[package]`, `[template]` : `path`,
  `entrypoint`, `thumbnail` ; `[tool]`) : https://github.com/typst/packages/blob/main/docs/manifest.md ;
  `typst init` : https://typst.app/docs/reference/ (CLI) ; Touying : https://touying-typ.github.io/ ;
  Polylux : https://github.com/polylux-typ/polylux
- [S21] Quarto, formats personnalisés et modèles de démarrage :
  https://quarto.org/docs/extensions/formats.html ;
  https://quarto.org/docs/extensions/starter-templates.html ; modèles de Pandoc :
  https://pandoc.org/MANUAL.html#templates
- [S22] VS Code, raccourcis par défaut (F8 « Go to Next Problem », Ctrl+. « Quick Fix ») :
  https://code.visualstudio.com/docs/getstarted/keybindings
- [S23] CodeMirror 6 : `@codemirror/autocomplete` (complétion, `info`, Ctrl+Espace) :
  https://codemirror.net/docs/ref/#autocomplete ; `hoverTooltip` :
  https://codemirror.net/docs/ref/#view.hoverTooltip
- [S24] tidy (documentation Typst, commentaires `///`) : https://github.com/Mc-Zen/tidy ;
  documentation de tinymist : https://myriad-dreamin.github.io/tinymist/
- [S25] GTK 4, `GtkText::insert-emoji` (liaisons Ctrl+. et Ctrl+;) :
  https://docs.gtk.org/gtk4/signal.Text.insert-emoji.html ; WebKit, bug 256080 :
  https://bugs.webkit.org/show_bug.cgi?id=256080
- [S26] Firefox, gestionnaire de processus `about:processes` et Gestionnaire des tâches :
  https://support.mozilla.org/kb/task-manager-tabs-or-extensions-are-slowing-firefox
- [S27] Études d'Inkwell : [`../architecture/research/editor-features.md`](../architecture/research/editor-features.md)
  §8–11 (N30) ; [`../architecture/research/ai.md`](../architecture/research/ai.md) (N31)
- [S18] Architecture d'Inkwell, §15 (points d'extension, capacités, thèmes, greffons de projet,
  noyau fermé, surfaces, diagrammes, outils) : [`../architecture/spec.md`](../architecture/spec.md)
- [RFC 2119] https://www.rfc-editor.org/rfc/rfc2119 · [RFC 8174] https://www.rfc-editor.org/rfc/rfc8174
