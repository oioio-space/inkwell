# Système de design d'Inkwell — spécification (N17)

- **Statut :** proposition, **révision 2** (2026-11-01).
  - Elle répond aux revues n°1 de Codex et de Fable ([`reviews/`](reviews/)).
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
  - extensions (N21) ;
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

  La règle vaut aussi pour les **thèmes CodeMirror écrits en TypeScript**
  (`EditorView.theme`, `baseTheme` dans `editor/*.ts`).
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
- **T-5.** `docs/design/tokens.json` est la référence lisible par machine ; `tokens.css` est ce
  que le code consomme. Le **test de parité est à écrire** (inventaire G-1b) :
  - chaque jeton de `tokens.json` existe dans `tokens.css` sous son nom `css`, avec ses valeurs
    par mode ;
  - chaque paire de `contrast.pairs` atteint `min` ;
  - chaque paire `forbidden` échoue, et son usage est absent du code.

  Aujourd'hui, seules les neuf couleurs syntaxiques sont vérifiées (`theme/tokens.test.ts`).
  On n'ajoute pas de générateur.
- **T-6.** Un jeton se nomme par son rôle, jamais par sa valeur.

### 2.2 Couleurs

Base : variables de libadwaita 1.8 [S1]. Les valeurs exactes et les 102 paires de contraste
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
| `--tooltip-bg` | infobulles | `rgb(0 0 6 / 80%)` | idem |
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

**Paires interdites** : elles échouent, et `tokens.json` les marque `forbidden`.

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
- **Trait** `--person-N-line` = `light-dark(color-mix(in srgb, var(--person-N) 65%, #000), var(--person-N))`,
  pour le curseur distant, la marge, la bande de modifications et le contour du curseur d'un
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
| **Droite** : le *temps et les autres* | Historique, Activité, Rechercher | Rechercher : recommandation Q5, *à confirmer*. Dans le tiroir, la recherche reste visible pendant qu'on parcourt les fichiers. |

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

- **L-3.** Chaque frontière est une **poignée APG *Window Splitter*** [S5] :
  - rôle `separator`, focalisable, avec `aria-valuenow`, `aria-valuemin`, `aria-valuemax` et
    `aria-controls` ;
  - flèches ±16px, Maj+flèches ±64px, Origine/Fin, Entrée pour replier ou rouvrir ;
  - double-clic pour la taille par défaut ;
  - zone de saisie de 8px.
- **L-4.** Chaque redimensionnement a une alternative sans glisser (2.5.7) : le clavier, et les
  entrées du menu Affichage.

### 3.4 États de la fenêtre

Le plan A6 fait foi (validé, 900px). Les seuils d'Adwaita (400/550/860sp [S1]) sont arrondis
aux siens.

| État | Largeur | Comportement |
|---|---|---|
| **Large** | ≥ 1200px | Trois zones côte à côte ; le tiroir pousse le contenu. |
| **Moyen** | 900–1199px | Le tiroir **recouvre** l'aperçu (niveau 2) au lieu de le pousser. |
| **Étroit** | 560–899px | **Comme A6 :** le panneau de gauche **et l'aperçu** deviennent des panneaux superposés (`AdwOverlaySplitView`). La source occupe toute la largeur ; « Aperçu » ouvre l'aperçu par-dessus. Le tiroir est superposé aussi. Un seul panneau superposé à la fois ; Échap ou un clic extérieur le ferme. |
| **Très étroit** | 360–559px | Comme Étroit. En plus : la barre d'en-tête ne garde que fichiers, titre, Partager et le menu ; les onglets deviennent un menu « fichiers ouverts » ; les dialogues deviennent des feuilles en bas (`AdwDialog` [S1]) ; la barre d'état applique ST-3. |

- La rév. 1 gardait l'aperçu côte à côte jusqu'à 560px, contre A6 (revues Codex et Fable).
- **L-5.** Taille minimale : 360 × 480px. Le contenu se relit à 320px CSS sans défilement
  horizontal de la fenêtre (1.4.10) ; l'éditeur, le tableur et l'aperçu sont exemptés (contenus
  à deux dimensions).
- **L-6.** Pas de barre de défilement horizontale sur la fenêtre. Aucun texte coupé sans
  ellipse et sans moyen de lire le texte entier (infobulle ou nom accessible).
- **L-7.** Les barres débordent dans un menu ⋯, groupe par groupe.

### 3.5 Messages : bandeaux et toasts

**Trois sortes de messages** (HIG [S2]) :

| Sorte | Composant | Exemple |
|---|---|---|
| **Événement** passé | toast | « 3 lignes intégrées » |
| **État** qui dure | bandeau | « Hors ligne » |
| **Question** qui attend une décision | bandeau-question ou dialogue | « Léa demande à rejoindre » |

- **L-8. Au plus un bandeau visible.** Ordre de priorité :

  1. **Les questions**, de la plus ancienne à la plus récente :
     - fichier modifié hors d'Inkwell en mode « Demander » (§5.16) ;
     - demande d'accès, pour un admin (§5.4) ;
     - demande de prise de main, pour le pilote (§5.11) ;
     - demande de suppression d'un fichier, pour un admin (B5).
  2. **Les états qui bloquent l'écriture**, par ordre :
     - fichier disparu ou devenu binaire ;
     - écriture sur le disque impossible ;
     - projet scellé ;
     - accès retiré ;
     - accès suspendu ou en lecture ;
     - fichier trop lourd, en lecture seule.
  3. **Les états d'information** : quelqu'un présente, fichier volumineux, hors ligne
     prolongé.

  Une question passe donc toujours avant un état : un état durable ne peut pas la masquer. Une
  question à laquelle seul le dialogue ouvert permet de répondre s'affiche dans ce dialogue.
- **L-8b.** S'il y a d'autres messages en attente, le bandeau DOIT afficher un bouton
  « +N autres ». Il ouvre le popover de l'état de synchronisation (§4.15), qui DOIT lister tous
  les messages en cours. Rien n'est donc caché sans accès (revue Codex).
- **L-8c.** Les bandeaux **locaux** d'un composant passent par le même `BannerHost` et sa
  priorité, dans la zone du composant :
  - filtre, détection, lecture seule et échec du tableur (`Sheet.svelte:469`, `:541`, `:555`,
    `:567`) ;
  - différences (`HistoryBars.svelte:143`).
- **L-9. Au plus un toast**, dans une file gérée par **un seul `ToastHost`**. Aujourd'hui,
  trois sources s'ignorent :
  - `NoticeToast` ;
  - `ExternalChangeToast` ;
  - le toast du tableur (`Sheet.svelte:605`, minuterie de 9s à `:169`).

  Un toast plus récent remplace le courant ; le remplacé n'est pas perdu (§4.12, annulation
  durable).
- **L-10.** Les exemples d'un toast d'annulation sont des actions **réversibles** :
  « Paul est en lecture · Annuler », « Fichier mis à la corbeille · Annuler ». Retirer une
  personne est irréversible : c'est une confirmation, jamais « Annuler » (correction de la
  rév. 1).

---

## 4. Composants

Une implémentation par composant. Les classes globales vivent dans `theme/adwaita.css` ; les
composants dans `ui/`.

**Les huit états de base**, à décrire pour chaque composant interactif :

| État | Rendu commun |
|---|---|
| repos | — |
| survol | `--hover-bg`, `--duration-fast` |
| focus | `--focus-ring` (`--focus-ring-on-dark` sur fond sombre), `:focus-visible` seulement |
| actif / pressé | `--selected-bg` ; contour sous `forced-colors` |
| désactivé | `--disabled-opacity`, `disabled` ou `aria-disabled`, pas de survol ; **une infobulle dit pourquoi** si la raison n'est pas évidente |
| chargement | `aria-busy`, indicateur après 300ms (§4.21) |
| erreur | `--error` + texte, `aria-invalid` ou bandeau |
| vide | écran vide (§4.20) |
| hors ligne | les actions qui exigent le réseau restent actives et **mettent en attente** (« Sera envoyé au retour du réseau »). Seules restent inactives celles qui ne peuvent pas attendre (« Rejoindre », « Inviter à proximité »), avec leur raison dans l'infobulle. |

Clavier : APG [S5]. **Retour du focus** : quand un conteneur transitoire se ferme (menu,
popover, dialogue, panneau superposé), le focus revient **toujours à l'élément qui l'a ouvert**.
Il n'y a pas d'exception.

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
- **H-2.** Au plus six contrôles à droite. Un espace libre pour déplacer la fenêtre. Un seul
  bouton suggéré, « Partager » : écart assumé avec la HIG, c'est l'action à découvrir.
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
  - une phrase de 60 caractères au plus, au plus une action, et fermer.
- **Durées :** 5s, ou 10s avec une action [S1]. La minuterie **se suspend** au survol et au
  focus.
- **TO-1.** Annulation durable (2.2.1, revues Codex et Fable). Ce qu'un toast propose
  d'annuler DOIT rester annulable **après** sa disparition :
  - une modification de texte : Ctrl+Z, et Historique › « Annuler cette modification » ;
  - une action sur un fichier (corbeille, renommage, déplacement) : Ctrl+Z dans le panneau
    Fichiers, et Fichiers › Corbeille › « Restaurer » ;
  - une action sur une personne (rôle, déconnecter) : la commande inverse **sur la ligne** de
    la personne (« Laisser revenir », menu Rôle) ;
  - un export : rien à annuler.

  Le toast le dit une fois par session : « Vous pouvez aussi annuler avec Ctrl+Z ».
- **TO-2.** Un toast ne pose jamais de question. Écart actuel : `ExternalChangeToast.svelte:52-63`.
- **Lecteur d'écran :** même règle de régions persistantes que §4.11 ; `status`, ou `alert` pour
  un échec. L'action est atteignable par F6, qui passe par la zone des toasts.

### 4.13 Champs

- **Libellé :** toujours visible ; le *placeholder* ne sert que d'exemple.
- **Hauteurs :** 34px (barre) ou 44px (dialogue).
- **Erreur :**
  - message sous le champ, relié par `aria-describedby` ;
  - `aria-invalid` ;
  - la saisie est conservée ;
  - le message dit comment corriger (3.3.1, 3.3.3).
- **Recherche :** loupe, bouton effacer, options en bascules. Échap efface, puis ferme.
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
- **Clic** : ouvre le fichier dans un onglet (§4.25). Écart actuel : le clic ne fait rien
  (G-0).
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
  - B et W ;
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

Le modèle n'existe pas aujourd'hui : un seul fichier courant, `workspace.current`.

- **Modèle :**
  - une liste ordonnée d'onglets **par projet, sur cet ordinateur**, jamais synchronisée ;
  - chaque onglet garde chemin, position, défilement et vue (texte ou tableau) ;
  - un seul onglet actif ;
  - **un fichier = au plus un onglet** : rouvrir un fichier active son onglet.
- **Ouvrir :**
  - clic ou Entrée dans Fichiers, Ctrl+P, un lien, un résultat de recherche, une tâche, un
    problème : l'onglet s'ouvre **à droite de l'onglet actif** et devient actif ;
  - pas d'onglet « aperçu » temporaire : la règle est plus simple pour des personnes non
    techniques.
- **Fermer :**
  - Ctrl+W, bouton × ou clic du milieu ;
  - fermer ne pose **jamais** de question (SAV-03), car tout est gardé ;
  - le focus va à l'onglet voisin de droite, sinon de gauche ;
  - si plus rien n'est ouvert, l'écran vide « Aucun fichier ouvert » s'affiche.
- **Renommé ou déplacé** (ici ou par une autre personne) : l'onglet suit, avec un toast discret
  seulement si c'est quelqu'un d'autre.
- **Supprimé** : l'onglet reste, en italique, avec le bandeau « Ce fichier est dans la
  corbeille » et l'action « Restaurer ».
- **Anatomie :**
  - barre de `--tab-height` sur `--headerbar-bg` ;
  - onglet : `FileIcon`, nom, point si non enregistré (mode Ctrl+S), × au survol **et au
    focus** ;
  - onglet actif : `--selected-bg` + filet `--accent` en bas (et contour sous
    `forced-colors`) ;
  - débordement : défilement horizontal de la barre, plus un menu « Fichiers ouverts » (⌄) qui
    les liste tous ;
  - au-delà de 12 onglets ouverts, rien n'est fermé d'office.
- **Clavier** (APG *Tabs* [S5]) :
  - un arrêt de tabulation ; ← → ; Suppr ferme l'onglet focalisé ;
  - Ctrl+Page↓ / Page↑ et Ctrl+Tab passent au suivant ou au précédent ;
  - Ctrl+Maj+Page↓ / Page↑ déplacent l'onglet (alternative au glisser, 2.5.7) ;
  - Alt+1…9 vont à l'onglet n.
- **Lecteur d'écran :** `tablist` nommé « Fichiers ouverts ». Un onglet se lit « main.typ, Typst,
  non enregistré ».

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
- **FT-3. Colonnes repérées (CSV en texte).**
  - Une colonne sur deux est teintée de `--hover-bg`.
  - Le séparateur est en `--hl-marker`.
  - L'en-tête est en gras.
  - Au survol ou au curseur, une infobulle donne le nom de la colonne (« Colonne C · Prix »).

  On n'utilise pas une couleur par colonne : ce serait illisible au-delà de 8 colonnes, et la
  couleur porterait seule l'information.
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

- Une **liste blanche** d'environ 25 langages, dans `language-data`, limite la taille du paquet.
  Un langage hors liste s'affiche sans coloration.
- La coloration des blocs passe par `markdown({ codeLanguages })`. Pour Typst, il faut vérifier
  que `codemirror-lang-typst` 0.6 permet l'imbrication (`parseMixed`). Sinon, les blocs restent
  en `--hl-raw` : *à vérifier en N18*.

### 4.29 Extensions (N21)

Le mot d'interface est **« extension »**, comme les extensions de GNOME. Jamais « plugin »,
« module », « WASI », « bac à sable » ni « capacité » (P5). L'exécution et l'API relèvent de
l'architecture (N18). N17 fixe ce que la personne voit.

**Gérer les extensions** : Préférences › page **Extensions**. C'est une page de
`AdwPreferencesDialog` [S1], et non une nouvelle fenêtre (K-2). Le modèle est l'application
*Extensions* de GNOME [S15].

- **En tête :** « Installer depuis un fichier… » (sélecteur de fichier). Un catalogue en ligne
  pourra venir plus tard, derrière ce même bouton ; il est hors de N17.
- **Une ligne par extension** (ligne dépliable, comme `AdwExpanderRow`) :
  - icône symbolique (celle de l'extension, sinon la pièce de puzzle générique) ;
  - nom ; auteur ; une phrase ;
  - **interrupteur** Activer ;
  - une fois dépliée :
    - version ;
    - « Ce qu'elle peut faire » (la liste des autorisations, chacune avec son interrupteur) ;
    - « Mettre à jour depuis un fichier… » ;
    - « Retirer ».
- **États d'une ligne :**

  | État | Rendu |
  |---|---|
  | active | interrupteur activé |
  | désactivée | interrupteur désactivé, nom en `--fg-dim` |
  | erreur au chargement | icône `warn` + « Ne démarre pas : fichier abîmé ou version d'Inkwell trop ancienne. » + « Retirer » |
  | arrêtée après des plantages | « Désactivée après 3 arrêts en 5 minutes. » + « Réactiver » |
  | autorisation retirée | la ligne d'autorisation dit ce qui ne marche plus (« Ne peut plus ajouter de bouton ») |

- **Plantage en cours d'usage :** toast « L'extension « Diagramme » s'est arrêtée. Votre texte
  n'est pas touché. · Relancer ». Une extension ne peut jamais bloquer la frappe (PERF-1).
- **Mettre à jour :** depuis un fichier. Si la nouvelle version demande **de nouvelles
  autorisations**, le dialogue de consentement ne montre **que celles-là**.
- **Retirer :** toast « « Diagramme » retirée · Annuler ». L'extension reste récupérable jusqu'à
  la fermeture d'Inkwell. Ses données locales sont effacées après.

**Consentement** (Nielsen n°2, 3, 5 ; HIG *Dialogs* [S2]). À l'installation, un `AlertDialog`
s'ouvre :
- titre « Installer « Diagramme » ? » ;
- l'auteur et la provenance (« depuis un fichier de votre ordinateur ») ;
- **la liste de ce que l'extension pourra faire**, en phrases simples, chacune avec une icône.
  Par exemple :

  | Autorisation (interface) | Sens |
  |---|---|
  | « Lire le texte du fichier ouvert » | lecture du document actif |
  | « Modifier le texte de vos documents » | écriture, **mise en avant** (`--warning`) |
  | « Lire les autres fichiers du projet » | lecture du projet |
  | « Ajouter des boutons et un panneau » | points d'affichage |
  | « Afficher un type de fichier » | visionneuse |
  | « Produire un aperçu ou un export » | aperçu et export |
  | « Garder des données sur cet ordinateur » | stockage local |
  | « Communiquer avec Internet » | réseau, **mise en avant**, avec « Inkwell ne peut pas voir ce qu'elle envoie » |

- les boutons « Annuler » (focus initial) et « Installer ».

Toute autorisation est **révocable** à tout moment depuis la ligne dépliée. L'extension est
alors informée, et la fonction qui en dépend disparaît avec une explication.

**Points d'affichage** : une extension n'apparaît que là où l'interface l'accueille déjà (K-2).

| Point | Où | Règle |
|---|---|---|
| Panneau | un onglet du sélecteur de gauche, après les onglets d'Inkwell | icône symbolique monochrome, sinon la pièce de puzzle |
| Bouton de barre | un groupe « Extensions » à la fin de la barre de mise en forme, avant Insérer, puis dans le débordement ⋯ | infobulle « Nom (extension « X ») » |
| Commande | dans la palette, groupe « Extensions », nom préfixé (« Diagramme : Insérer un diagramme ») | les raccourcis d'une extension ne remplacent jamais ceux d'Inkwell (K-6) |
| Visionneuse de type de fichier | la vue d'un type non géré, ou « Ouvrir avec… » pour un type géré. Les quatre types principaux gardent leur vue par défaut. | — |
| Aperçu, export | un format de plus dans « Exporter… » (« HTML, par Diagramme »), ou un rendu dans l'aperçu | — |

**Confinement visuel** : on doit savoir ce qui vient d'une extension.
- Le contenu d'une extension est dessiné **dans un cadre fourni par Inkwell**.
- Ce cadre a une ligne d'attribution en `--text-caption` : une pièce de puzzle en
  `--mark-extension`, puis « Extension « Diagramme » ».
- Le cadre **reçoit les jetons** de `tokens.css` (couleurs, polices, rayons, clair et sombre).
  Le rendu suit donc le thème d'Inkwell et ne détonne jamais.
- Une extension **ne peut pas** :
  - dessiner hors de son cadre ;
  - ouvrir un dialogue ou un toast à elle : elle passe par `Dialog` et `ToastHost`, qui
    préfixent le nom de l'extension ;
  - poser un bandeau-question ;
  - imiter un écran d'Inkwell (consentement, personnes, partage).

**Collaboration** : quand le document de quelqu'un utilise une extension que je n'ai pas.
- Le **texte reste lisible et modifiable**. Seule la vue enrichie manque.
- À la place de la vue, ou du rendu dans l'aperçu, un encadré neutre s'affiche : « Ce passage
  s'affiche avec l'extension « Diagramme », que vous n'avez pas. Le texte reste modifiable. »
  Il propose « Afficher le texte » et « D'où vient cette extension ? », qui donne le nom,
  l'auteur, et « Demandez le fichier à Marie ».
- **Rien ne s'installe tout seul**, même envoyé par une autre personne. Installer reste un geste
  explicite, avec consentement.
- Avant un export : « Ce document utilise l'extension « Diagramme » : ces passages manqueront
  dans le PDF. · Exporter quand même · Annuler ».

---

## 5. Écrans et parcours

Les cinq tâches de référence (`ux-principles.md`) sont : **créer, partager, relire, présenter,
exporter**. Chaque parcours a son entrée, ses étapes, ses messages et sa sortie.

### 5.1 Premier lancement

- **Accueil** : `StatusPage` plein écran.
  - Titre « Bienvenue dans Inkwell ».
  - « Écrivez seul ou à plusieurs, sans compte et sans serveur. »
  - **Votre nom** (obligatoire, 40 caractères au plus).
  - **Votre couleur** : 8 pastilles en groupe radio, chacune nommée ; le créneau libre est
    proposé (C-PERSON-2).
  - « Commencer ».
- Puis le **projet d'exemple** s'ouvre : aperçu visible, curseur dans le texte, aperçu à jour
  dès la première frappe.
- Pas de visite guidée. Aucune question réseau avant « Partager » (règle 8).
- Le nom et la couleur se changent plus tard dans Préférences › Moi.

### 5.2 Projets

- **Écran Projets** (menu › Projets…, Ctrl+O) :
  - projets récents en liste encadrée (nom, dernière modification relative, personnes) ;
  - « Nouveau projet » (suggéré) ;
  - « Ouvrir un dossier… » ;
  - « Rejoindre avec une invitation ».
- **Nouveau projet** (dialogue `small`) :
  - nom ;
  - modèle (Document, Rapport, Lettre, Diapositives, Notes Markdown) ;
  - le chemin s'affiche en `--text-small` ;
  - « Créer ».
- **Fermer un projet** : rien à confirmer. On revient à l'écran Projets.

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

**Côté admin :** une demande d'accès est un **bandeau-question** de priorité 1 :
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
- **Modifications hors d'Inkwell :** toast « 3 lignes intégrées depuis le disque · Voir ·
  Annuler ». En mode « Demander » : bandeau-question (§5.16).
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
- **Résultat :** toast « PDF enregistré · Ouvrir · Afficher dans le dossier ». Le chemin
  s'affiche dans l'infobulle, **pas dans la phrase**. Écart actuel :
  `fr.json:450` (« PDF enregistré : {path} »).
- **Erreurs :**
  - « Le document contient des erreurs. **Voir les problèmes** » ;
  - « La mise en page ne démarre pas. **Réessayer** ».
- **Imprimer…** passe par le menu et la palette, **sans raccourci**. Ctrl+P sert déjà à
  « Aller au fichier », et Ctrl+Maj+P à la palette.
  - Inkwell exporte un PDF temporaire et l'ouvre dans la boîte d'impression du système (portail
    d'impression XDG sous Linux).
  - *À confirmer techniquement en N18* (Q10).

### 5.13 Préférences (Ctrl+,)

| Groupe | Réglages | Portée |
|---|---|---|
| **Apparence** | Thème : Système, Clair, Sombre ; taille du texte ; ligatures de code | cet ordinateur |
| **Enregistrement** | Automatique, Avec Ctrl+S ; délai ; modifications hors d'Inkwell : Intégrer, Demander | ce projet |
| **Moi** | nom, couleur souhaitée, empreinte ; « Me rendre invisible » | cet ordinateur |
| **Projet** (admin) | suppression des fichiers : sur accord d'un admin, librement | ce projet, signé |
| **Avancé** | Détails techniques ; langue de l'interface : Système, Français, English | cet ordinateur |
| **Extensions** (page à part) | installer, activer, autorisations, mettre à jour, retirer (§4.29) | cet ordinateur |

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
| hors ligne ≥ 30s | bandeau `info` (3) | « Vous êtes hors ligne. Vos modifications sont gardées et partiront au retour du réseau. » | « Réessayer maintenant » |
| écriture disque impossible | bandeau `error` (2) | « Le fichier n'a pas pu être écrit sur le disque. Votre texte reste dans Inkwell ; nouvel essai dans 10 s. » | « Réessayer », « Détails » |
| supprimé dehors | bandeau `warning` (2) | « « ch2.typ » a été supprimé du disque. Son texte reste dans Inkwell. » | « Recréer le fichier », « Fermer » |
| devenu binaire | bandeau `warning` (2) | « « ch2.typ » n'est plus un texte sur le disque. Inkwell garde sa version. » | « Garder la version d'Inkwell » |
| modifié dehors, mode Demander | **question** (1) | « « ch2.typ » a été modifié hors d'Inkwell. » | « Intégrer », « Garder la version d'Inkwell » |
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
| droit d'une extension | **ce qu'elle peut faire**, **autorisation** | capacité, permission système |
| fichier de code | « Python », « JavaScript »… · **lecture** / **Modifier** | IDE, LSP, linter |

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
| A-20 | Raccourcis d'une seule touche (B, W en présentation ; lettres dans un arbre) : actifs seulement quand leur composant a le focus. | 2.1.4 |

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
| Problème suivant, précédent | F8 / Maj+F8 | à faire |
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
contexte. Ctrl+Maj+E est déjà pris par la barre de mise en forme (`ui/toolbar/spec.ts`) : à
départager avant le code.

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

---

## 10. Preuves

Une règle sans assertion n'est pas finie. Traçabilité règle → assertion :

| Règles | Assertion | Où |
|---|---|---|
| T-1 (CSS) | stylelint : pas de couleur, d'ombre, de rayon ni de durée bruts dans `*.svelte` ni dans `theme/adwaita.css` hors `var()` | `mise run lint` (à ajouter) |
| T-1 (TS) | test Vitest qui analyse `editor/*.ts` : aucune couleur `#…`, `rgb(` ni `px` de police dans les objets de thème CodeMirror | `editor/theme.lint.test.ts` (à ajouter) |
| T-5, §2.2, C-SEL, C-GUT | parité `tokens.json` ↔ `tokens.css` ; chaque paire à `min`, composée sur `over` ; chaque paire `forbidden` échoue **et** son motif n'apparaît pas dans le code | `theme/tokens.test.ts` (à étendre) |
| T-2 | e2e **sous WebKitGTK** (application réelle) : `getComputedStyle(body).backgroundColor` vaut `--window-bg` en clair, puis en sombre (`data-theme`) | `frontend/e2e/theme.spec.ts` |
| A-1, A-2, A-17 | axe-core en clair, en sombre et sous `forced-colors`, sur chaque écran de §5, **avec les états concurrents** (bandeau, toast et popover ouverts ensemble) | `e2e/a11y.spec.ts` |
| L-8, L-8b, L-8c, L-9, TO-1 | tests unitaires de la file de `BannerHost` et de `ToastHost` : priorités, une question avant un état, « +N », remplacement, suspension de la minuterie, annulation durable rejouée après expiration | `ui/messages.test.ts` |
| §4.11, §4.12, A-10 | test DOM : les régions `status` et `alert` existent avant le premier message et reçoivent son texte ensuite ; **plus** une passe Orca manuelle par vague | test + `ux-walkthrough` |
| A-4, F-1, SB-1, §4.2, §4.5, §4.22, §4.25 | tests de composants avec `userEvent.keyboard`, par motif APG : touches, focus, `aria-*` | `*.test.ts` |
| A-5, TT-1 | e2e : focaliser chaque bouton de barre et attendre l'infobulle ; vérifier que son rectangle ne coupe ni celui de l'élément ni son anneau ; même vérification pour les toasts et les bandeaux | `e2e/focus.spec.ts` |
| A-7, §2.5 | e2e : chaque élément focalisable (boutons, champs, onglets, lignes, poignées) ≥ 24px ; ceux des barres ≥ 34px de zone active ; ceux des dialogues ≥ 44px | `e2e/targets.spec.ts` |
| A-11, L-5, §3.4 | captures à 360×480, 800×600, 1280×800 à 200 %, et 320px CSS sans défilement horizontal de la fenêtre | e2e |
| T-11, A-12 | e2e en `reducedMotion: "reduce"` : `document.getAnimations()` ne contient que des fondus ≤ 150ms | e2e |
| K-4, §6.3, §6.4 | parité fr/en ; espaces insécables ; « … » et non « ... » ; mots interdits absents de `fr.json` | `i18n.test.ts` (à étendre) |
| K-6 | test du registre : pas de doublon de raccourci par contexte | `commands.test.ts` |
| PERF-1, §5.17 | benchmark de latence de frappe sur un document de 512 Ki et de 2 Mi caractères ; e2e netsim WAN : frappe locale < 50ms pendant une synchronisation lente | `e2e/perf.spec.ts`, netsim |
| §5 parcours | `ux-walkthrough` : cinq tâches chronométrées, personas | `docs/ux/walkthrough-*.md` |

---

## 11. Décisions à confirmer par l'utilisatrice

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
- [RFC 2119] https://www.rfc-editor.org/rfc/rfc2119 · [RFC 8174] https://www.rfc-editor.org/rfc/rfc8174
