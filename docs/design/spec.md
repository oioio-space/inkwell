# Système de design d'Inkwell — spécification (N17)

- **Statut :** proposition, révision 1 (2026-11-01). Doit être relue par l'utilisatrice avant
  d'être appliquée (questions ouvertes en §11).
- **Portée :** toute l'interface du frontend (`frontend/src`) : fenêtre principale, vues de
  présentation, dialogues, menus et messages.
- **Sources internes :** [`ux-principles.md`](../ux-principles.md),
  [`glossary.md`](../glossary.md), [plan T11](../plans/2026-10-25-t11-interface.md) (maquette v3
  validée et exigences), [`conception.pdf`](../conception.pdf), `CLAUDE.md` (« Cohérence »),
  [`engineering-principles.md`](../engineering-principles.md).
- **Sources externes :** listées en §12 ; chaque règle renvoie à sa source par un repère
  `[S…]`.

## 0. Conventions

Les mots **DOIT**, **NE DOIT PAS**, **DEVRAIT**, **NE DEVRAIT PAS** et **PEUT**, en capitales,
s'entendent au sens de BCP 14 ([RFC 2119], [RFC 8174]) : MUST, MUST NOT, SHOULD, SHOULD NOT,
MAY. En minuscules, ils ont leur sens ordinaire.

- Une règle numérotée (`T-3`, `C-12`…) est vérifiable. La colonne « Preuve » de §10 dit comment.
- « Personne » désigne l'utilisatrice ou l'utilisateur. « Pair » n'apparaît que pour parler du
  code.
- Les valeurs de couleur sont données en clair / sombre. Les rapports de contraste sont
  calculés selon WCAG 2.2 (luminance relative), après composition des couleurs translucides
  sur leur fond réel. Le script de calcul est reproduit dans le test de §2.1.

---

## 1. Principes

Chaque principe découle des dix règles de `ux-principles.md` et se prouve (§10).

| # | Principe | Ce qu'il impose | Source |
|---|---|---|---|
| P1 | **Le texte d'abord** | Une barre d'en-tête, une barre de mise en forme repliable, une barre d'état. Tout le reste se replie ou vit dans un popover. | règles 1–2 ; HIG *Header bars* [S2] |
| P2 | **Un seul langage** | Un besoin = un composant, un nom, un raccourci. Aucun composant recopié avec ses propres styles. | `CLAUDE.md` « Cohérence » |
| P3 | **Réversible plutôt que confirmé** | Une action destructrice s'annule par un toast. Une confirmation n'est permise que si l'action est irréversible *pour les autres* (vider la corbeille, retirer une personne, passer la main). | règle 6 ; HIG *Dialogs* [S2] |
| P4 | **Chaque message dit quoi faire** | Une phrase qui dit ce qui se passe et ce qui est préservé, plus une action. Toute attente de plus de 3 s montre une progression. | règle 5 ; HIG *Spinners* [S2] |
| P5 | **Zéro jargon** | Le vocabulaire du glossaire, et lui seul. Le technique vit derrière « Détails techniques ». | règle 4 ; glossaire |
| P6 | **Pour tout le monde** | WCAG 2.2 AA au minimum, clavier complet, lecteur d'écran, thème et contraste du système, mouvement réduit. | règle 9 ; [S4] [S5] |
| P7 | **Natif et sobre** | Apparence libadwaita 1.8, thème du système. Pas de couleur sans rôle ni d'animation sans fonction. | « Système visuel » ; [S1] |

En cas de conflit, l'ordre est **P6 > P4 > P3 > P2 > P1 > P7 > P5**. L'accessibilité passe
avant tout. La fidélité à Adwaita cède devant un contraste insuffisant (exemple : l'anneau de
focus, §2.2).

---

## 2. Jetons de design

### 2.1 Architecture

- **T-1.** Les jetons ont trois niveaux, en propriétés CSS dans `frontend/src/theme/tokens.css` :
  - les **primitifs** `--p-*` sont des valeurs brutes ;
  - les **sémantiques** `--*` portent un sens ;
  - les **composants** n'emploient que des jetons sémantiques.

  Un composant NE DOIT PAS employer de primitif ni de valeur brute (couleur, ombre, rayon,
  durée, taille de texte). Exceptions permises : `0`, `1px` pour un filet, `50%` pour un
  cercle, et les tailles intrinsèques d'une icône.
- **T-2.** Le clair et le sombre DOIVENT passer par `light-dark()` sur `:root`, avec
  `color-scheme: light dark`. Le système décide par défaut ; `data-theme="light|dark"` sur la
  racine force le choix (réglage « Apparence », §5.10).
  - `light-dark()` est Baseline 2024 [S6].
  - WebKitGTK DOIT être vérifié par un test e2e qui lit une couleur calculée dans les deux
    thèmes. Repli si absent : un bloc `@media (prefers-color-scheme: dark)`.
- **T-3.** `@media (prefers-contrast: more)` DOIT renforcer les traits et le texte atténué
  (valeurs « HC » de §2.2, reprises de libadwaita [S1]).
- **T-4.** `@media (forced-colors: active)` : les composants NE DOIVENT PAS masquer les
  couleurs système. Tout état porté par un fond DOIT aussi avoir un contour ou une forme :
  sélection, bouton enfoncé, focus [S6].
- **T-5.** `docs/design/tokens.json` est la **référence lisible par machine**. `tokens.css` est
  ce que le code consomme. Un test Vitest (`theme/tokens.test.ts`, à étendre) DOIT vérifier :
  - que chaque jeton de `tokens.json` existe dans `tokens.css` avec la même valeur ;
  - que chaque paire déclarée dans `contrast` atteint son seuil, en clair et en sombre.

  On n'ajoute pas de générateur (Style Dictionary, etc.) : le test de parité suffit et ne coûte
  aucune dépendance.
- **T-6.** Un jeton se nomme par son **rôle**, jamais par sa valeur : `--fg-dim`, pas `--grey-55`.

### 2.2 Couleurs

Base : variables libadwaita 1.8 [S1]. Les rapports sont calculés ; ✗ marque un échec, corrigé
dans la colonne « Spécifié ».

#### Surfaces

| Jeton | Rôle | Clair | Sombre |
|---|---|---|---|
| `--window-bg` | fond de fenêtre | `#fafafb` | `#222226` |
| `--view-bg` | éditeur, listes, grille | `#ffffff` | `#1d1d20` |
| `--headerbar-bg` | barre d'en-tête, barre de mise en forme, barre d'état | `#ffffff` | `#2e2e32` |
| `--sidebar-bg` | panneau Fichiers, fond de l'aperçu | `#ebebed` | `#2e2e32` |
| `--side-bg-2` | tiroir de droite (Historique…) | `#f3f3f5` | `#28282c` |
| `--card-bg` | cartes, listes encadrées | `#ffffff` | `rgb(255 255 255 / 8%)` |
| `--popover-bg` | popovers, menus | `#ffffff` | `#36363a` |
| `--dialog-bg` *(nouveau)* | dialogues | `#fafafb` | `#36363a` |
| `--stage-bg` *(nouveau)* | fond des vues de présentation | `#000000` | `#000000` |
| `--page-bg` *(nouveau)* | page de l'aperçu (papier) | `#ffffff` | `#ffffff` |

`--dialog-bg` reprend `--dialog-bg-color` de libadwaita [S1]. Aujourd'hui, les dialogues
prennent un fond au hasard.

#### Texte, traits, interaction

| Jeton | Clair | Sombre | Contraste (clair / sombre) |
|---|---|---|---|
| `--fg` | `rgb(0 0 6 / 80%)` | `#ffffff` | 12,57 sur vue · 11,18 sur barre latérale / 16,82 · 13,52 |
| `--fg-dim` | `rgb(0 0 6 / 55%)` | `rgb(255 255 255 / 55%)` | 4,74 sur vue · **4,54** sur barre latérale / 5,93 · 4,87 sur popover |
| `--fg-dim` HC | `rgb(0 0 6 / 90%)` | `rgb(255 255 255 / 90%)` | ≥ 9 |
| `--border` | `rgb(0 0 6 / 15%)` | `rgb(255 255 255 / 15%)` | décoratif |
| `--border` HC | `rgb(0 0 6 / 50%)` | `rgb(255 255 255 / 50%)` | ≥ 3 |
| `--shade` | `rgb(0 0 6 / 12%)` | `rgb(0 0 6 / 36%)` | décoratif |
| `--btn-bg` | `rgb(0 0 6 / 10%)` | `rgb(255 255 255 / 10%)` | le libellé porte le sens |
| `--hover-bg` | `rgb(0 0 6 / 7%)` | `rgb(255 255 255 / 7%)` | — |
| `--selected-bg` | `rgb(0 0 6 / 10%)` | `rgb(255 255 255 / 10%)` | doublé d'une graisse ou d'une coche (T-4) |
| `--accent` (texte, liens) | `#0461be` | `#81d0ff` | 6,07 sur vue · 5,10 sur barre latérale / 9,93 · 7,98 |
| `--accent-bg` | `#3584e4` | `#3584e4` | remplissage seulement ; texte blanc dessus : **3,77 ✗** |
| `--suggested-bg` | `#1c71d8` | `#1c71d8` | texte blanc : 4,77 |
| `--accent-fg` | `#ffffff` | `#ffffff` | — |
| `--disabled-opacity` *(nouveau)* | `0.5` | `0.5` | exempté (WCAG 1.4.3) |

- `--fg-dim` est la couleur la plus faible permise pour du texte. Le texte atténué NE DOIT PAS
  recevoir d'`opacity` supplémentaire : 55 % × 0,8 donne 3,23:1, sous le seuil.
- `--fg-dim` NE DOIT PAS servir de texte sur `--info-bg` en sombre : 3,95:1 ✗.
- `--disabled-opacity` reprend libadwaita (50 %) et remplace 0,45 et 0,5 mélangés.

#### Anneau de focus — écart assumé avec libadwaita

| Jeton | Actuel | Contraste actuel | Spécifié | Contraste spécifié |
|---|---|---|---|---|
| `--focus-ring` | `2px solid` `--accent-bg` à 75 % | **2,62 ✗** vue claire · **2,34 ✗** barre latérale · **2,63 ✗** barre sombre | `2px solid var(--accent)`, décalage 2px | 6,07 · 5,10 / 7,98 · 9,93 |

- WCAG 1.4.11 exige 3:1 pour un indicateur de focus [S4].
- libadwaita emploie un anneau translucide, trop pâle ici. **A-2** (§7) l'emporte sur P7.

#### États

| Jeton | Clair | Sombre | Contraste |
|---|---|---|---|
| `--success` (texte, pastille) | ~~`#2ec27e`~~ → **`#007c3d`** | `#78e9ab` | actuel **2,30 ✗** → 5,32 / 9,04 |
| `--success-bg` *(nouveau)* | `#2ec27e` | `#26a269` | remplissage |
| `--warning` | `#905400` | `#ffc252` | 6,08 · 5,53 sur `--warning-bg` / 9,88 |
| `--warning-bg` | `#fdf3e1` | `rgb(255 194 82 / 14%)` | `--fg` dessus : 11,80 / 11,45 |
| `--error` | `#c01c28` → **`#c30000`** | `#ff7b63` → **`#ff938c`** | 6,32 · 5,38 sur `--error-bg` / 7,40 · 5,67 |
| `--error-bg` | `#fce8e9` | `rgb(255 147 140 / 14%)` | — |
| `--destructive` (texte) | = `--error` | = `--error` | idem |
| `--destructive-bg` *(nouveau)* | `#e01b24` | `#c01c28` | texte blanc : 4,83 / 6,11 |
| `--info-bg` | `#e3eefb` | `#2b4b73` | `--fg` dessus : 11,29 / 8,91 |

- Les valeurs en gras alignent `--success` et `--error` sur les couleurs nommées de
  libadwaita 1.8 [S1]. Les anciennes passaient pour l'erreur, mais pas pour le succès.
- `--success-bg`, `--destructive-bg` et `--warning-bg` sont des **fonds**. On n'écrit jamais du
  texte de leur couleur.

#### Différences (historique)

| Jeton | Clair | Sombre | Contraste |
|---|---|---|---|
| `--diff-add-fg` sur `--diff-add-bg` | `#0b4a2a` / `#d4f0df` | `#c6f7d9` / `rgb(87 227 137 / 22%)` | 8,55 / 8,46 |
| `--diff-del-fg` sur `--diff-del-bg` | `#7a0f18` / `#fbd9d9` | `#ffd0c7` / `rgb(255 123 99 / 24%)` | 8,40 / 7,96 |

- **C-DIFF.** Un ajout DOIT aussi se signaler sans la couleur : souligné, préfixe « + » pour le
  lecteur d'écran.
- Une suppression DOIT être barrée. La couleur NE DOIT PAS être le seul indice (WCAG 1.4.1).

#### Syntaxe

Les jetons `--hl-*` de `tokens.css` restent tels quels. Ils sont déjà vérifiés à 4,5:1 par
`tokens.test.ts`.

#### Marques de type de fichier

Ce sont des objets graphiques : WCAG 1.4.11 demande 3:1 contre le fond, ici
`--sidebar-bg` / `--view-bg`. Le type est aussi donné par l'`aria-label` et l'extension.

| Jeton | Clair actuel → spécifié | Contraste clair (barre latérale) | Sombre | Contraste sombre |
|---|---|---|---|---|
| `--mark-typst` | `#1a8f9e` | 3,23 | `#4fd0e0` | 7,36 |
| `--mark-markdown` | `#3584e4` → **`#1c71d8`** | 3,17 → 4,01 | `#78aeed` | 5,84 |
| `--mark-csv` | `#26a269` → **`#1b8553`** | **2,73 ✗** → 3,90 | `#57e389` | 8,21 |
| `--mark-bib` | `#9141ac` | 4,96 | `#dc8add` | 5,62 |
| `--mark-image` | `#e66100` → **`#c64600`** | **2,90 ✗** → 4,12 | `#ffa348` | 6,82 |
| `--mark-pdf` | `#e01b24` | 4,05 | `#f66151` | 4,34 |
| `--mark-data` | `#a2680c` | 3,90 | `#f8e45c` | 10,46 |

#### Couleurs des personnes

Les huit couleurs d'Okabe-Ito, distinguables par les daltoniens [S8] : `--person-1` à
`--person-8` = `#e69f00 #56b4e9 #009e73 #f0e442 #0072b2 #d55e00 #cc79a7 #999999`.

| Usage | Règle | Calcul |
|---|---|---|
| Pastille (avatar) | Initiale en `#000` ; en `#fff` sur `--person-5` seulement. | min 5,43 (noir sur `#d55e00`) ; 5,19 (blanc sur `#0072b2`) |
| Curseur, trait de marge, coche de bande de modifications | **`--person-N-line`** *(nouveau)* : `light-dark(color-mix(in srgb, var(--person-N) 65%, #000), var(--person-N))` | clair : toutes ≥ 3,0 (jaune pur 1,32 ✗ → 3,05). Sombre : min 3,24 (`#0072b2`). |
| Fond de sélection ou de « qui a écrit quoi » | `color-mix(var(--person-N) 25 %, transparent)` | décoratif : le nom est donné au survol et en légende |

- **C-PERSON.** La couleur NE DOIT JAMAIS être le seul moyen d'identifier une personne. Le nom
  DOIT être visible :
  - à côté du curseur distant ;
  - au survol d'une pastille ;
  - dans toute légende.
- **C-PERSON-2.** Deux personnes présentes en même temps NE DEVRAIENT PAS partager une couleur.
  - L'attribution par hachage modulo 8 (`ui/people.ts`) DEVRAIT céder la place à une
    attribution stable dans le projet : premier créneau libre, dans l'ordre d'entrée dans la
    liste des membres.
  - Au-delà de 8 personnes, l'initiale et le nom départagent.

### 2.3 Typographie

- **Polices** : `--font-ui` (Adwaita Sans), `--font-mono` (Adwaita Mono), `--font-serif`
  (Source Serif 4, glyphes de la barre et style « Lecture »). Aucune police n'est téléchargée
  (CSP).
- **Échelle**, calquée sur les classes de libadwaita [S1]. Les tailles en `%` de la base
  viennent du source de libadwaita (non revérifiées en ligne, voir §12). La base reste à
  **14px**, la valeur actuelle.

| Jeton | Taille / interligne / graisse | Équivalent Adwaita | Usage |
|---|---|---|---|
| `--text-caption` | 12px / 1.33 / 400 | `.caption` (82 %) | légendes, raccourcis dans les menus, nom sur un curseur distant |
| `--text-small` | 13px / 1.4 / 400 | — | barre d'état, sous-titres, infobulles, lignes secondaires |
| `--text-body` | 14px / 1.45 / 400 | `.body` | texte d'interface par défaut |
| `--text-heading` | 14px / 1.25 / 700 | `.heading` | titre de la barre d'en-tête, titres de cartes et de panneaux, boutons |
| `--text-title-4` | 16.5px / 1.3 / 700 | `.title-4` (118 %) | titre de dialogue |
| `--text-title-2` | 19px / 1.25 / 800 | `.title-2` (136 %) | titre d'écran vide, d'accueil |
| `--text-title-1` | 25px / 1.2 / 800 | `.title-1` (180 %) | nom de l'application à l'accueil, « À propos » |
| `--text-document` | 16px / 1.6 / 400 | `.document` (12pt) | lecture longue : différences, aperçu HTML, notes d'orateur |
| `--text-editor` | 14px × zoom / 1.75, mono | `--monospace-font-family` | éditeur, tableur |

- **T-7.** Aucune taille de texte en dessous de 12px, sauf le texte dessiné dans l'aperçu par
  Typst.
- **T-8.** Les chiffres qui changent (position, compteurs, minuteur, zoom) DOIVENT utiliser
  `font-variant-numeric: tabular-nums` (`.numeric` d'Adwaita).
- **T-9.** Le texte DOIT tenir sans coupure ni chevauchement :
  - au zoom de 200 % (WCAG 1.4.4) ;
  - avec les espacements de WCAG 1.4.12 : interligne 1,5, espacement des paragraphes 2×, des
    lettres 0,12em, des mots 0,16em.

### 2.4 Espacement

Une grille de 2px, avec les multiples d'Adwaita, 6, 12 et 18 [S1]. Les jetons existants gardent
leur valeur, pour ne rien casser.

| Jeton | Valeur | Usage type |
|---|---|---|
| `--space-0` *(nouveau)* | 2px | entre boutons d'une barre |
| `--space-1` | 4px | icône ↔ compteur |
| `--space-1b` *(nouveau)* | 6px | marge des barres, espacement entre contrôles d'en-tête |
| `--space-2` | 8px | icône ↔ libellé, marge de popover |
| `--space-3` | 12px | marge intérieure de ligne, groupes de la barre |
| `--space-4` | 18px | marge de dialogue, écart entre sections |
| `--space-5` *(nouveau)* | 24px | entre groupes de préférences |
| `--space-6` *(nouveau)* | 36px | page d'accueil, écran vide |

### 2.5 Cibles et hauteurs

| Jeton | Valeur | Règle |
|---|---|---|
| `--target-bar` | 34px | boutons des barres (en-tête, mise en forme), éléments de menu |
| `--target-min` | 44px | boutons et lignes de dialogue, lignes de liste encadrée, cibles tactiles |
| `--bar-header` *(nouveau)* | 47px | barre d'en-tête (Adwaita) |
| `--bar-tool` *(nouveau)* | 46px | barre de mise en forme |
| `--bar-sub` *(nouveau)* | 40px | barres secondaires : outils de l'aperçu, bascule tableau ↔ texte |
| `--bar-status` *(nouveau)* | 36px | barre d'état ; éléments de 28px de haut |

- **T-10.** Toute cible DOIT faire au moins **24 × 24px** (WCAG 2.5.8 AA) [S4]. Les cibles des
  barres DEVRAIENT faire 34px et celles des dialogues 44px (règle 9 de `ux-principles.md`).
- Les éléments de 28px de la barre d'état passent 2.5.8 ; ils ne descendent pas plus bas.

### 2.6 Rayons

| Jeton | Valeur | Usage |
|---|---|---|
| `--radius-small` | 6px | éléments de menu, lignes de liste, puces |
| `--radius-button` | 9px | boutons, champs, infobulles, groupes de bascules |
| `--radius` | 12px | cartes, popovers, listes encadrées |
| `--radius-window` | 15px | **dialogues** et fenêtre (`--window-radius` d'Adwaita) |
| `--radius-pill` *(nouveau)* | 999px | toasts, pastilles de comptage, avatars |

Les rayons bruts 2, 3, 4, 5, 8, 11px relevés dans le code DOIVENT revenir à l'un de ces cinq
jetons.

### 2.7 Élévation et couches

| Niveau | Jeton | Valeur (clair / sombre) | Usage |
|---|---|---|---|
| 0 | — | aucune ombre, filet `--shade` | barres, panneaux |
| 1 | `--shadow-card` | `0 0 0 1px --card-shade, 0 1px 3px 1px --card-shade` | cartes |
| 2 | `--shadow-popover` | contour 1px + `0 4px 16px` 18 % / 45 % | popovers, menus |
| 3 | `--shadow-dialog` *(nouveau)* | `0 0 0 1px --border, 0 12px 32px` `rgb(0 0 6 / 25%)` / `rgb(0 0 0 / 45%)` | dialogues |
| 3 | `--shadow-toast` *(nouveau)* | `0 4px 16px` `rgb(0 0 6 / 25%)` / `rgb(0 0 0 / 45%)` | toasts |
| — | `--backdrop` *(nouveau)* | `rgb(0 0 6 / 30%)` / `rgb(0 0 0 / 45%)` | fond derrière un dialogue modal |

Ordre d'empilement (`--z-*`, nouveaux) :

| Couche | Valeur |
|---|---|
| barres | 2 |
| panneau superposé (fenêtre étroite) | 5 |
| popover | 20 |
| infobulle | 30 |
| toast | 40 |

Les dialogues et la présentation passent par la *top layer* (`showModal()`) et n'ont pas de
`z-index`.

### 2.8 Mouvement

| Jeton | Valeur | Usage |
|---|---|---|
| `--duration-fast` *(nouveau)* | 100ms | survol, pression |
| `--duration` | 200ms | apparition d'un popover, d'un toast ou d'un bandeau ; rotation d'un chevron |
| `--duration-slow` *(nouveau)* | 300ms | panneau qui s'ouvre ou se replie, tiroir |
| `--ease-out` *(nouveau)* | `cubic-bezier(0.33, 1, 0.68, 1)` | ce qui entre |
| `--ease-in-out` *(nouveau)* | `cubic-bezier(0.65, 0, 0.35, 1)` | ce qui se déplace |

- **T-11.** Sous `prefers-reduced-motion: reduce` [S6] :
  - toutes les durées valent 0, sauf les fondus d'opacité, permis à `--duration-fast` ;
  - aucune translation, mise à l'échelle ni rotation ;
  - aucune animation infinie : pulsation « Connexion… », miroitement des pages en attente ;
  - pas de défilement doux (`behavior: "auto"`).
- **T-12.** Une animation NE DOIT PAS durer plus de 300ms, sauf un indicateur d'attente.
  - Rien ne clignote plus de 3 fois par seconde (WCAG 2.3.1).
  - Le surlignage d'une cellule reçue (`fresh`, 4s) est un fondu de couleur, pas un mouvement.
    Il est permis s'il disparaît sans mouvement.

### 2.9 Iconographie

- **I-1.** Les icônes sont **symboliques**, dans le style Adwaita :
  - grille 16 × 16, trait de 1.6, bouts arrondis, `currentColor` ;
  - définies dans `ui/icons/paths.ts`, rendues par `Icon.svelte`.

  Aucune icône en couleur, sauf la marque intérieure d'une icône de fichier (`FileIcon`).
- **I-2.** Une icône seule dans un bouton DOIT avoir :
  - un nom accessible (`aria-label`) ;
  - une infobulle (§4.6) qui reprend ce nom et le raccourci.

  Une icône à côté d'un libellé DOIT être `aria-hidden`.
- **I-3.** Tailles : 16px dans les barres et les menus ; 12–14px dans un bouton de fermeture ou
  une coche ; 64px (ou une illustration) seulement sur un écran vide de premier niveau (HIG
  *Placeholders* [S2]).
- **I-4.** Une même idée = une même icône partout : `branch` = Historique, `eye` = Aperçu,
  `side` = Fichiers, `present` = Présenter, `menu` = menu principal. Une nouvelle icône DOIT
  être ajoutée à `paths.ts` et au tableau §8.3.
- **I-5.** Une `FileIcon` placée dans une ligne qui affiche déjà le nom du fichier DEVRAIT être
  `aria-hidden`. Le type se lit alors dans la description de la ligne, pour éviter
  « Typst main.typ » lu deux fois.

---

## 3. Mise en page

### 3.1 Les zones

```
┌─────────────┬───────────────────────────────────────────────┬──────────────┐
│ Fichiers    │ Barre d'en-tête (47)                          │              │
│ (barre      ├───────────────────────────────────────────────┤ Tiroir       │
│  latérale)  │ Barre de mise en forme (46) — repliable        │ Historique · │
│  264        ├───────────────────────────────────────────────┤ Commentaires │
│             │ Bandeau (au plus un, §3.4)                    │ · Activité   │
│             ├────────────────────────┬──────────────────────┤ 380          │
│             │ Source (éditeur,       │ Aperçu               │              │
│             │ tableur)               │                      │              │
│             ├────────────────────────┴──────────────────────┤              │
│             │ Barre d'état (36)                             │              │
└─────────────┴───────────────────────────────────────────────┴──────────────┘
                     Toasts : centrés, au-dessus de la barre d'état
```

- **L-1.** Une seule fenêtre (règle 1). La barre latérale, l'aperçu et le tiroir se replient
  chacun d'un geste. Leur état et leur taille sont mémorisés **par projet**.
- **L-2.** Le tiroir de droite est **le seul** lieu des panneaux secondaires : Historique,
  Commentaires, Activité, et à terme Tâches et Rechercher (règle 2). Un nouveau panneau DOIT
  devenir un onglet de ce tiroir, ou justifier une exception dans une ADR.
  - La recherche dans le projet (B6) DEVRAIT devenir un onglet « Rechercher » du tiroir.
  - La recherche dans le fichier reste une barre au-dessus de l'éditeur.

### 3.2 Panneaux ajustables

| Panneau | Défaut | Min | Max | Sous le min |
|---|---|---|---|---|
| Fichiers | 264px | 180px | 400px | se replie |
| Aperçu | 50 % de la zone | 260px | 70 % | se replie |
| Tiroir | 380px | 300px | 50 % | se replie |
| Source | — | 280px | — | — |

Les bornes de la barre latérale reprennent `AdwOverlaySplitView` (180–280sp), élargies à 400
pour les noms longs [S1].

- **L-3.** Chaque frontière ajustable DOIT être une poignée accessible (APG *Window Splitter*
  [S5]) :
  - `role="separator"`, `aria-orientation="vertical"`, `aria-valuenow`, `aria-valuemin`,
    `aria-valuemax` et `aria-controls` ;
  - focalisable ;
  - flèches = ±16px, Maj+flèches = ±64px, Origine/Fin = min/max, Entrée = replier ou rouvrir ;
  - double-clic = taille par défaut ;
  - zone de saisie de 8px pour un trait visible de 1px.
- **L-4.** Le redimensionnement DOIT avoir une alternative sans glisser (WCAG 2.5.7) : le
  clavier ci-dessus, et les entrées du menu principal (« Fichiers », « Aperçu », « Historique »).

### 3.3 États de la fenêtre

Seuils en largeur de fenêtre, alignés sur les points de rupture d'Adwaita (400 / 550 / 860sp
[S1]) et sur le plan A6 (900px) :

| État | Largeur | Comportement |
|---|---|---|
| **Large** | ≥ 1200px | Les trois zones côte à côte ; le tiroir pousse le contenu. |
| **Moyen** | 900–1199px | Le tiroir **recouvre** l'aperçu (superposé, ombre de niveau 2), sans le pousser. |
| **Étroit** | 560–899px | La barre latérale et le tiroir sont superposés (`AdwOverlaySplitView`) et se ferment au clic extérieur et à Échap. L'aperçu reste à côté de la source tant que la zone centrale fait ≥ 560px. |
| **Très étroit** | 360–559px | Une zone à la fois : la source, ou l'aperçu par-dessus. Dans la barre d'en-tête, l'état de synchronisation devient une pastille seule (nom accessible conservé), et l'aperçu, la présentation et l'historique passent dans le menu principal. Les dialogues deviennent des feuilles en bas (`AdwDialog` en *bottom sheet* [S1]). |

- **L-5.** Taille minimale : **360 × 480px**. Au zoom de 400 % d'une fenêtre de 1280px
  (= 320px CSS), le contenu DOIT se lire sans défilement horizontal de la fenêtre (WCAG 1.4.10
  *Reflow* [S4]). L'éditeur, le tableur et l'aperçu PEUVENT défiler en deux dimensions : ce sont
  des contenus à deux dimensions, exemptés par 1.4.10.
- **L-6.** Aucune barre de défilement horizontale sur la fenêtre. Aucun texte d'interface coupé
  sans ellipse ni infobulle.
- **L-7.** La barre de mise en forme déborde dans un menu « Plus » (⋯), par groupes entiers
  (déjà fait : `toolbar/layout.ts`).

### 3.4 Empilement des messages

Sources : HIG *Banners*, *Toasts* [S2] ; `AdwBanner`, `AdwToastOverlay` [S1].

- **L-8.** **Au plus un bandeau visible à la fois**, sous les barres. S'il y a plusieurs états
  à la fois, le plus prioritaire s'affiche. Les autres attendent et restent consultables
  (popover de l'état de synchronisation, §4.15).

  Priorité, de la plus haute à la plus basse :
  1. Le fichier a disparu ou n'est plus un texte (`ExternalBanner`).
  2. L'écriture sur le disque échoue (`ProjectionBanner`).
  3. Le projet est scellé (`SealedBanner`).
  4. Accès en lecture seule ou suspendu (`AccessBanner`).
  5. Quelqu'un présente (`PresentBanner`).
  6. Fichier lourd, lecture lente (`limit.slow`).
  7. Hors ligne prolongé (nouveau, §5.12).
- **L-9.** **Au plus un toast à la fois**, dans une file : un nouveau toast remplace l'ancien
  s'il est plus prioritaire, sinon il attend. Un seul `ToastHost` DOIT gérer la file ;
  aujourd'hui, `NoticeToast` et `ExternalChangeToast` se superposent.
- **L-10.** Un **toast** annonce un *événement* passé : « 3 lignes intégrées », « Paul a été
  retiré · Annuler ». Un **bandeau** décrit un *état* qui dure : « Hors ligne ». Une
  **question** qui attend une réponse est un bandeau ou un dialogue, jamais un toast (§9).

---

## 4. Composants

Chaque composant a une seule implémentation. Les classes globales (`.fb`, `.btn`, `.sug`,
`.pop`, `.mi`, `.card`, `.boxed`, `.tg`, `.tip`) vivent dans `theme/adwaita.css`. Les
composants Svelte vivent dans `ui/`.

**États communs**, sauf mention contraire :

| État | Rendu |
|---|---|
| repos | — |
| survol | `--hover-bg`, `--duration-fast` |
| focus clavier | `--focus-ring` (`:focus-visible` seulement) |
| actif (pressé) | `--selected-bg` |
| désactivé | `--disabled-opacity`, `aria-disabled` ou `disabled`, aucun survol |
| erreur | `--error`, plus un texte (jamais la couleur seule) |
| chargement | indicateur après 300ms ; texte « … en cours » pour le lecteur d'écran |
| vide | §4.20 |

Le clavier suit l'APG [S5]. Les noms de touches sont ceux de l'interface française.

### 4.1 Barre d'en-tête (`ui/shell/HeaderBar.svelte`)

- **Anatomie :**
  - à gauche : bascule Fichiers, état de synchronisation ;
  - au centre : titre, puis sous-titre (fichier, avec un point « non enregistré ») ;
  - à droite : Aperçu, Présenter, pile de personnes, **Partager** (bouton suggéré), Historique,
    menu principal.
- **H-1.** Le titre DEVRAIT être le **nom du projet** et le sous-titre le fichier ouvert.
  L'application ne se nomme pas dans sa propre barre : HIG *Header bars*, le titre décrit le
  contenu [S2]. Le titre de la fenêtre système reste « ● fichier — projet — Inkwell ».
- **H-2.** Au plus **six** contrôles à droite. Un espace libre DOIT rester pour déplacer la
  fenêtre [S2].
  - Un seul bouton suggéré : « Partager ». La HIG le déconseille dans une barre d'en-tête, mais
    c'est l'action à découvrir du produit. L'écart est assumé, et c'est le seul permis.
  - Aucun bouton destructeur dans la barre.
- **H-3.** Chaque bouton à icône a une infobulle avec son raccourci (I-2), via le composant
  Infobulle (§4.6), pas l'attribut `title`.
- **H-4.** Les bascules (Fichiers, Aperçu, Historique) ont `aria-pressed`. Leur nom accessible
  est fixe (« Fichiers », « Aperçu », « Historique ») : c'est `aria-pressed` qui dit l'état. Un
  nom qui change (« Afficher… / Masquer… ») et un état annoncé feraient double emploi (APG
  *Button* [S5]).
- **Clavier :** F10 ouvre le menu principal (HIG *Keyboard* [S2]). Ctrl+, ouvre les
  préférences. Ctrl+? ouvre les raccourcis (§8.2).
- **Lecteur d'écran :** `<header>` contient un `<h1>` (titre). L'état de synchronisation est un
  `role="status"` qui n'annonce que les **changements**, pas chaque rendu.

### 4.2 Menu principal et menus (`ui/popover/MenuButton.svelte`, `.pop`, `.mi`)

- **Anatomie :**
  - un bouton déclencheur (`aria-haspopup="menu"`, `aria-expanded`) ;
  - un popover avec une flèche ;
  - des éléments `.mi`, avec la position du raccourci à droite en `--text-caption` et une coche
    pour les `menuitemcheckbox` ;
  - des séparateurs ;
  - des titres de section `.cap`.
- **Clavier** (APG *Menu Button*, *Menu* [S5]) :
  - Entrée, Espace ou ↓ ouvre et focalise le premier élément ; ↑ ouvre et focalise le dernier ;
  - ↑ et ↓ se déplacent en boucle ; Origine et Fin vont au premier et au dernier ;
  - Échap ferme et rend le focus au **déclencheur** ;
  - Tab ferme et poursuit.
  - **M-1.** Échap rend aujourd'hui le focus à l'éditeur (`MenuButton.svelte:54`). C'est permis
    pour les menus de la barre de mise en forme seulement. Partout ailleurs, le focus DOIT
    revenir au déclencheur.
  - **M-2.** Une lettre tapée DEVRAIT aller au prochain élément qui commence par elle.
- **Contenu :**
  - un élément = un verbe à l'infinitif ou un nom d'affichage (« Exporter en PDF »,
    « Barre de mise en forme ») ;
  - « … » seulement si l'action demande une saisie de plus (HIG *Writing style* [S2]) ;
  - au plus 12 éléments, en groupes de 2 à 5 séparés.
- Le menu principal DOIT contenir, dans cet ordre :
  1. Affichage : Barre de mise en forme, Fichiers, Aperçu, Historique ;
  2. Présenter ;
  3. Exporter en PDF ;
  4. Préférences, Raccourcis clavier, À propos d'Inkwell.

### 4.3 Barre de mise en forme (`ui/toolbar/FormatBar.svelte`, `BarButton.svelte`)

- **Anatomie :**
  - menu des titres ;
  - groupes de boutons (marques, code en ligne et bloc, listes, liens, etc.), séparés par un
    espace de `--space-3` ;
  - menu ⋯ « Plus » ;
  - menu Insérer.

  Le contenu dépend du format : Typst, Markdown. Le CSV n'a pas cette barre ; le tableur a la
  sienne (§4.19).
- **États :**
  - bouton bascule `aria-pressed` selon le curseur ;
  - barre « verrouillée » pour une personne en lecture ou suspendue : la barre DEVRAIT être
    **masquée**, pas grisée. Le bandeau d'accès explique pourquoi. Une barre grisée et inerte
    n'apprend rien (§9).
- **Clavier** (APG *Toolbar* [S5]) :
  - **F-1.** La barre DOIT être **un seul arrêt de tabulation**, avec un tabindex itinérant :
    ← → se déplacent, Origine/Fin vont aux extrêmes, ↓ ouvre un menu.
  - Le dernier bouton focalisé est mémorisé.
  - Alt+F10 (convention des éditeurs de texte riche) DEVRAIT amener le focus de l'éditeur à la
    barre ; Échap le rend.
- **Infobulle :** libellé, syntaxe, raccourci (« Gras `*texte*` Ctrl+B »), déjà fait.
- **Souris :** `mousedown` ne prend pas le focus à l'éditeur (déjà fait).

### 4.4 Boutons (`.fb`, `.btn`, `.sug`, `.destructive`)

| Variante | Classe | Usage | Rendu |
|---|---|---|---|
| plat | `.fb` | barres, icônes | transparent ; survol `--hover-bg` |
| plat + libellé | `.fb.lbl` | actions de toast, barre secondaire | idem, marge 10px |
| normal | `.btn` | dialogues, bandeaux | `--btn-bg`, `--text-heading` |
| suggéré | `.sug` | **une** action principale par vue | `--suggested-bg`, texte `--accent-fg` |
| destructeur | `.destructive` *(à créer, global)* | retirer, vider, passer la main | `--destructive-bg`, texte blanc |
| bascule | `.fb[aria-pressed]` | état on/off | `--selected-bg` quand pressé |

- **B-1.** Au plus un bouton suggéré par dialogue ou par vue, et au plus un destructeur.
- **B-2.** Un libellé de bouton est un **verbe précis** : « Inviter », « Rejoindre »,
  « Garder ma version ». Jamais « OK » ni « Oui » (HIG *Dialogs* [S2]).
- **B-3.** Un bouton occupé (action en cours) reste à sa place, garde son libellé, prend
  `aria-busy="true"` et `disabled`. Au-delà de 300ms, il montre un indicateur d'attente
  (§4.21).
- `.danger` local à `PeopleDialog` DOIT être remplacé par `.destructive`.

### 4.5 Bascules : groupe et interrupteur (`.tg`, `.sw`)

- **Groupe de bascules** (`AdwToggleGroup` [S1]) : choix exclusif de 2 à 4 options courtes
  (« Automatique | Ctrl+S »).
  - `role="radiogroup"` et `role="radio"` ; ← → déplacent et choisissent.
  - Option choisie : fond `--popover-bg` et ombre. En couleurs forcées, un contour (T-4).
- **Onglets du tiroir :** même rendu, mais sémantique `tablist` / `tab` / `tabpanel` :
  - ← → déplacent et activent (APG *Tabs*, activation automatique [S5]) ;
  - Origine et Fin vont au premier et au dernier ;
  - `aria-controls` relie l'onglet à son panneau.
- **Interrupteur :** une préférence on/off qui s'applique tout de suite. `role="switch"`, Espace
  bascule. Une case à cocher n'est employée que dans une liste de choix multiples.

### 4.6 Infobulle (`.tipwrap` / `.tip`)

- **Anatomie :** libellé en gras, puis la syntaxe en `--font-mono`, puis le raccourci en `.kbd`.
  Fond `--tooltip-bg`, texte blanc, contraste 12,57:1 en clair comme en sombre.
- **Comportement** (APG *Tooltip* [S5], WCAG 1.4.13 [S4]) :
  - apparaît au survol après **500ms** et au focus clavier tout de suite ;
  - reste tant que le pointeur est sur le bouton ou sur elle (*hoverable*) ;
  - **Échap la ferme** sans déplacer le focus (*dismissible*) ;
  - disparaît au départ du pointeur, du focus, ou au clic.
- **TT-1.** Une seule infobulle visible à la fois. Elle NE DOIT PAS couvrir l'élément focalisé
  (WCAG 2.4.11) : elle se place en dessous dans les barres du haut, au-dessus dans la barre
  d'état.
- **TT-2.** L'attribut `title` NE DOIT PAS servir d'infobulle : rendu système, sans délai
  maîtrisé, invisible au clavier, ignoré par certains lecteurs d'écran. On en relève 48 dans
  le code (inventaire).
- **Lecteur d'écran :** `role="tooltip"`, relié par `aria-describedby`. Le nom reste dans
  `aria-label`.

### 4.7 Popover (`.pop`)

- **Anatomie :** une flèche vers le déclencheur ; `--radius` ; `--shadow-popover` ; une marge de
  6px ; hauteur au plus celle de la fenêtre moins 16px, avec défilement intérieur.
- **P-1.** Un popover DEVRAIT occuper au plus un tiers de la fenêtre [S2], hors liste de
  problèmes et menu Insérer.
- **P-2.** Pas de bouton « Fermer » ni « Terminé » dans un popover [S2]. Échap et le clic
  extérieur ferment.
- **Rôles :**
  - `role="menu"` s'il ne contient que des actions ;
  - `role="dialog"` avec `aria-label` s'il contient des champs ou du texte (statistiques,
    filtres).

### 4.8 Dialogue (`Dialog.svelte`, *à créer*)

Un seul composant remplace les sept dialogues recopiés : partage, personnes, préférences, à
propos, application d'une version, collage, différences.

- **Anatomie :**
  - `<dialog>` ouvert par `showModal()` (top layer, focus piégé par le navigateur) ;
  - fond `--dialog-bg`, `--radius-window`, `--shadow-dialog`, `::backdrop` `--backdrop` ;
  - en-tête : titre `<h2>` en `--text-title-4` et bouton fermer rond de 44px (« Fermer ») ;
  - corps qui défile ;
  - pied facultatif : Annuler à gauche, action à droite.
- **Tailles :** `small` 380px, `medium` 520px, `large` 760px, `wide` 1100px. Chacune DOIT être
  plafonnée à `100vw − 32px`. Sous 560px, le dialogue DEVRAIT devenir une feuille en bas
  (`AdwDialog` [S1]).
- **Clavier** (APG *Dialog (Modal)* [S5]) :
  - à l'ouverture, le focus va au premier champ, sinon au bouton par défaut, sinon au titre
    (`tabindex="-1"`) ;
  - Tab reste dans le dialogue ;
  - Échap = Annuler ;
  - à la fermeture, le focus revient au **déclencheur** ;
  - Entrée active l'action par défaut, sauf si elle est destructrice [S2].
- **Lecteur d'écran :** `aria-labelledby` (titre), `aria-describedby` (première phrase, si elle
  explique).
- **D-1.** Un dialogue ne s'ouvre qu'à la demande de la personne, jamais tout seul [S2].
  Exception : une demande d'accès reçue (§5.4), et seulement si la personne est admin et active.

### 4.9 Dialogue de confirmation (`AlertDialog`)

- Réservé aux actions irréversibles pour les autres (P3) : retirer une personne, vider la
  corbeille, passer la main, revenir à une version pour **tout** le document.
- **Anatomie :**
  - un titre-question (« Retirer Paul du projet ? ») ;
  - une phrase qui dit la conséquence ;
  - **deux** boutons : Annuler, puis l'action en verbe (`.destructive` si elle détruit) [S2].
- **Focus initial :** sur **Annuler** (APG *Alert Dialog* [S5]). Entrée NE DOIT PAS valider une
  action destructrice.
- **Variante en ligne :** dans une liste, la confirmation PEUT se déplier sous la ligne
  (`PeopleDialog`, « ask »).
  - Le focus DOIT y aller.
  - Échap DOIT la refermer et rendre le focus au bouton qui l'a ouverte.

### 4.10 Dialogue de préférences

Calqué sur `AdwPreferencesDialog` [S1] : groupes titrés (`<h3>`), listes encadrées `.card.boxed`,
lignes de 52px minimum (titre, sous-titre `--text-small`, contrôle à droite), note `--fg-dim`
sous le groupe. Le contenu est en §5.10.

### 4.11 Bandeau (`ui/Banner.svelte`)

- **Anatomie :** une bande pleine largeur sous les barres, une phrase centrée, **au plus deux**
  boutons `.btn` à droite. Ton `info` (`--info-bg`), `warning` (`--warning-bg`) ou `error`
  (`--error-bg`, nouveau).
- **BN-1.** Un bandeau DOIT proposer une action, sauf s'il décrit un état sur lequel la personne
  ne peut rien. Il dit alors ce qui se passe tout seul (« nouvel essai en cours ») et quand.
- **BN-2.** Un bandeau ne disparaît pas seul (HIG [S2]). Il disparaît quand l'état cesse.
- **BN-3.** Au plus un à la fois (L-8).
- **Lecteur d'écran :**
  - `role="status"` pour un état d'information ;
  - `role="alert"` seulement si une action est attendue *maintenant* (fichier supprimé,
    écriture impossible) ;
  - le texte DOIT être présent dans le DOM avant l'apparition de la région (WCAG 4.1.3 [S4]).
- **Mouvement :** révélation en hauteur, `--duration` `--ease-out`. Aucune sous `reduce`.

### 4.12 Toast (`ui/Toast.svelte` + `ToastHost`, *à créer*)

- **Anatomie :** une pilule (`--radius-pill`) centrée au-dessus de la barre d'état. Fond
  `--toast-bg` (nouveau : `rgb(0 0 6 / 84%)` en clair, `#36363a` en sombre), texte blanc,
  14,51:1. Une phrase courte, **au plus une** action plus le bouton fermer.
- **Durée** : 5s par défaut, comme `AdwToast` [S1]. 10s si le toast porte une action
  (« Annuler », « Voir »).
  - **TO-1.** La minuterie DOIT se suspendre tant que le pointeur ou le focus est sur le toast
    (WCAG 2.2.1).
  - Le toast reste joignable par F6 (§8.2).
- **TO-2.** Un toast NE DOIT PAS poser de question ni attendre une réponse (HIG [S2]).
  « Modifié hors d'Inkwell : intégrer ou garder ? » devient un bandeau (§5.12).
- **Lecteur d'écran :** `role="status"`. `role="alert"` seulement pour un échec. L'action est
  annoncée (« Annuler, bouton »). La phrase NE DOIT PAS dépasser 60 caractères.
- **Exemples :**
  - « 3 lignes intégrées depuis le disque · Voir · Annuler » ;
  - « Paul est en lecture · Annuler » ;
  - « Version « Relu » enregistrée ».

### 4.13 Champs (entrée, sélection, recherche)

- **Entrée :**
  - hauteur 34px (barre) ou 44px (dialogue), `--btn-bg`, `--radius-button` ;
  - focus : anneau ;
  - un libellé visible (`<label>`), jamais un *placeholder* seul ;
  - le *placeholder* donne un exemple (« Collez l'invitation ici »).
- **Erreur :**
  - message sous le champ, `--error`, relié par `aria-describedby` ;
  - `aria-invalid="true"` ;
  - le message dit comment corriger (WCAG 3.3.1, 3.3.3) ;
  - la saisie est conservée.
- **Sélection :** `<select>` natif habillé en `.btn`, pour garder le clavier et le lecteur
  d'écran du système. Au-delà de 7 options, ou avec une description par option, utiliser un
  menu (§4.2).
- **Recherche** (`SearchField`) : icône loupe, bouton effacer, options en bascules (casse, mot
  entier, expression). Échap efface, puis ferme.
- **Redondance** (WCAG 3.3.7) : le nom saisi une fois (accueil) n'est plus jamais redemandé.
  L'invitation collée reste dans le champ après un échec.

### 4.14 Avatar, pile de personnes, curseur distant

- **Avatar :**
  - cercle de 28px (barre) ou 32px (liste), fond `--person-N`, initiale selon §2.2 ;
  - liseré de 2px de la couleur de la surface ;
  - un point vert (`--success`) en bas à droite = présent, mais l'état est aussi écrit dans la
    liste.
- **Pile** (`PeopleStack.svelte`) :
  - au plus **4** avatars, puis « +N » ;
  - c'est un bouton qui ouvre le dialogue Personnes, nom accessible « Personnes : vous et
    3 autres » ;
  - **AV-1.** Le contenu d'un `<button>` DOIT être du contenu phrasé : pas de `<ul>` dedans
    (HTML). On utilise des `<span>`, ou le bouton reçoit un nom et la pile devient
    `aria-hidden`.
  - au survol, une infobulle liste les noms.
- **Curseur distant** (`editor/remoteCursors.ts`) :
  - trait de 2px `--person-N-line` ;
  - étiquette du nom en `--text-caption` (12px), au-dessus de la ligne, fond `--person-N`,
    encre selon §2.2 ;
  - l'étiquette s'estompe après 3s d'inactivité et revient au survol ;
  - `aria-hidden` (déjà fait).

### 4.15 Indicateur de synchronisation

- **Anatomie :** une pastille de 8px et un mot, dans la barre d'en-tête. **Un seul**
  indicateur : la barre d'état n'affiche que le nombre de personnes, sans pastille, pour ne pas
  doubler l'information.

| État (glossaire) | Mot | Pastille | Mouvement |
|---|---|---|---|
| seul | « Sur cet ordinateur » | `--fg-dim` | — |
| connexion | « Connexion… » | `--accent-bg` | pulsation, coupée sous `reduce` |
| synchronisé | « Synchronisé » | `--success` | — |
| hors ligne | « Hors ligne — rien n'est perdu » | `--warning` | — |
| suspendu | « Accès suspendu » | `--warning` | — |

- **S-1.** L'indicateur DEVRAIT être un bouton qui ouvre un popover d'état. Le popover dit :
  - avec qui on est relié, en clair (« directement », « par un relais ») ;
  - depuis quand ;
  - ce qui attend d'être envoyé ;
  - les états mis en attente par L-8.

  « Détails techniques » s'y trouve, replié.
- « Seul » (actuel) s'accorde au genre : on le remplace par une formule neutre (§6.3).

### 4.16 Liste des fichiers (`ui/shell/Sidebar.svelte`)

- **Anatomie :**
  - titre « Fichiers » dans une tête de 47px, alignée sur la barre d'en-tête ;
  - arbre de lignes de 36px, indentation de 20px par niveau ;
  - chevron, `FileIcon`, nom tronqué avec ellipse, mention « principal ».
- **États :**
  - sélection : `--selected-bg` + graisse 600 ;
  - en réception : barre fine et « 12 sur 40 » ;
  - demande de suppression : nom barré, plus « Suppression demandée » ;
  - dans la corbeille : section repliée en bas ;
  - vide : §4.20.
- **Clavier** (APG *Tree View* [S5]) :
  - **SB-1.** L'arbre DOIT être un `role="tree"` à un seul arrêt de tabulation ;
  - ↑ ↓ se déplacent ; → ouvre ou descend, ← ferme ou remonte ; Origine et Fin ;
  - Entrée ouvre le fichier ; F2 renomme ; Suppr supprime (avec toast « Annuler ») ;
  - Maj+F10 ou la touche Menu ouvrent le menu contextuel ;
  - une lettre tapée va au fichier suivant qui commence par elle.

  Aujourd'hui ce sont des boutons dans une `<nav>`, sans flèches.
- **Lecteur d'écran :** `aria-expanded` sur les dossiers, `aria-selected` et `aria-current`
  sur le fichier ouvert, `aria-level`. Le type est lu dans la description.
- **Glisser-déposer** (B5) : il DOIT avoir son équivalent sans glisser : menu contextuel
  « Déplacer vers… » (WCAG 2.5.7).

### 4.17 Barre d'état (`ui/shell/StatusBar.svelte`)

- **Anatomie, de gauche à droite :**
  - personnes présentes ;
  - espace ;
  - problèmes (✓ ou ⊗ n ⚠ n) ;
  - page n sur N ;
  - « Non enregistré » ;
  - position « Ligne, colonne » ;
  - mots (popover Statistiques) ;
  - zoom ;
  - encodage, seulement s'il diffère d'UTF-8 et LF ;
  - langage, puis « Mise en page » (B8).
- **ST-1.** Les éléments cliquables sont des boutons de 28px avec une infobulle ; les autres
  sont du texte. Un même rendu NE DOIT PAS servir aux deux. Aujourd'hui, la position et le
  langage portent la classe `.fb` sans être des boutons.
- **ST-2.** Le texte est en `--text-small`, couleur `--fg`. Pas d'`opacity` sur du texte
  (§2.2).
- **ST-3.** En fenêtre très étroite, on garde problèmes, « Non enregistré » et position ; le
  reste passe dans un menu ⋯.
- **Lecteur d'écran :**
  - `<footer aria-label="Barre d'état">` ;
  - les changements de position ne sont **pas** annoncés ;
  - les problèmes sont annoncés une fois, quand leur nombre passe de 0 à n ou de n à 0.

### 4.18 Aperçu (`preview/Preview.svelte`)

- **Anatomie :**
  - barre d'outils de 40px : zoom, ajuster, page n/N, « Exporter en PDF » ;
  - pages sur `--sidebar-bg`, papier `--page-bg`, ombre de niveau 1 ;
  - repère de saut source ↔ aperçu.
- **États :**

| État | Rendu | Texte |
|---|---|---|
| démarrage | squelette de page (`--hover-bg`, miroitement coupé sous `reduce`) | « Préparation de l'aperçu… » |
| à jour | pages | — |
| en cours | pages actuelles + indicateur discret dans la barre après 300ms | « Mise à jour… » |
| erreurs, aperçu ancien | pages d'avant + bandeau interne `warning` | « Le document contient des erreurs : l'aperçu montre la dernière version correcte. **Voir les problèmes** » |
| échec | écran vide (§4.20) avec action | selon le cas (`preview.fail.*`), toujours avec une action : « Voir les problèmes », « Réessayer » |
| page manquante | case de page + texte | « Page indisponible » |

- **PV-1.** Les couleurs brutes (`#fff`, `#55555a`, dégradé `#f4f4f6`) DOIVENT devenir des
  jetons (`--page-bg`, `--fg-dim`, `--hover-bg`).
- **Clavier :**
  - la zone des pages est focalisable (`role="group"`, « Pages de l'aperçu ») ;
  - ↑ ↓ et Page↑ Page↓ défilent ; Ctrl+Alt+J saute de la source à l'aperçu ;
  - Entrée sur une page saute à la source.
- **Lecteur d'écran :**
  - chaque page est une image « Page n sur N » ;
  - le PDF exporté reste la version accessible du rendu ; l'aperçu n'est pas une alternative
    textuelle.

### 4.19 Tableur (`ui/sheet/*`)

Grille APG *Grid* [S5] (`role="grid"`, déjà en place) :
- flèches, Ctrl+flèches, Origine et Fin, Page↑ et Page↓ ;
- F2 ou Entrée pour éditer, Échap pour annuler ;
- Maj+F10 pour le menu contextuel.

Barre propre, fond `--headerbar-bg`, hauteur `--bar-sub`. Mêmes jetons, menus, popovers et
toasts que le reste : le tableur n'a pas de style propre. Chargement : `role="progressbar"`
(déjà fait).

### 4.20 Écran vide (`StatusPage`, *à créer*)

Calqué sur `AdwStatusPage` et HIG *Placeholders* [S1] [S2] :
- une icône symbolique de 64px, ou une illustration seulement au premier lancement ;
- un titre `--text-title-2` qui dit l'état ;
- une phrase qui dit quoi faire ;
- au plus un bouton suggéré.

Exemples :
- historique vide ;
- aucun commentaire ;
- aucun résultat de recherche ;
- personne d'autre dans le projet ;
- échec de l'aperçu.

Un écran vide DOIT toujours dire comment en sortir.

### 4.21 Attente : indicateur et progression

- **Indicateur d'attente** (`AdwSpinner` [S1]) : affiché seulement si l'attente dépasse
  **300ms**, pour éviter les éclairs. Un seul à la fois dans une zone [S2].
- **Barre de progression** : obligatoire dès qu'une durée **ou** une quantité est connue
  (réception de fichiers, ouverture d'un gros fichier) ; et au-delà de 30s [S2].
  `role="progressbar"`, `aria-valuenow`, texte « 12 sur 40 fichiers, 3,2 Mo ».
- **W-1.** Au-delà de 3s, l'attente DOIT dire ce qui se passe et ce qu'on peut faire en
  attendant (règle 5).

### 4.22 Historique et différences (`ui/history/*`)

- **Tiroir :**
  - onglets Historique, Commentaires, Activité (§4.5) ;
  - filtres en `<select>` habillés ;
  - interrupteurs « Qui a écrit quoi » et « Afficher les variantes abandonnées » (en
    `role="switch"`) ;
  - liste de séances (`HistoryRow`) en `role="list"`, flèches pour naviguer (déjà là).
- **Ligne :**
  - avatar ;
  - phrase en langage naturel (« Marie a ajouté 2 phrases dans « Introduction » ») ;
  - heure relative (« aujourd'hui 14:10 »).

  Actions au survol **et** au focus : Voir, Revenir à ce moment, Enregistrer une version.
- **Vue des différences** (`DiffView`) :
  - un dialogue `wide` ;
  - mots ajoutés soulignés sur `--diff-add-bg`, supprimés barrés sur `--diff-del-bg`
    (C-DIFF) ;
  - F7 / Maj+F7 vont de changement en changement ;
  - en tête, la phrase résumé.
- **Revenir** : un aperçu, puis un bouton « Revenir à ce moment ». Annulable par un toast,
  puisque cela ajoute une modification (P3). La confirmation n'est requise que pour « tout le
  document ».

### 4.23 Vues de présentation (`present/*`)

- **Vue public** (`PublicWindow`) : plein écran, `--stage-bg`, la diapositive seule, aucun
  chrome. Le message discret « Présentateur déconnecté » apparaît en bas, `--text-small`, sur un
  fond `rgb(0 0 0 / 55%)` (jeton `--stage-overlay`, nouveau).
- **Vue présentateur** (`PresenterView`) :
  - diapositive courante (≥ 60 % de la largeur) ;
  - diapositive ou étape suivante ;
  - notes en `--text-document`, agrandissables ;
  - minuteur et heure en `tabular-nums` ;
  - outils : écran noir ou blanc, pointeur, échanger les écrans, quitter.
- **Clavier :**
  - → / Espace / Page↓ = suivant ; ← / Page↑ = précédent ;
  - Origine / Fin ;
  - B = écran noir, W = écran blanc ;
  - un numéro puis Entrée = aller à la diapositive ;
  - Échap = quitter ;
  - F5 = présenter, Maj+F5 = depuis ici.

  Ce sont les conventions de LibreOffice Impress et PowerPoint.
- **Lecteur d'écran :** annonce « Diapositive n sur N » et le titre de la diapositive à chaque
  changement (`role="status"`).
- **Pointeur laser :** `--destructive-bg` avec halo. Le halo est coupé sous `reduce`.

### 4.24 Palette de commandes (*à créer*)

Règle 3 de `ux-principles.md` : « trois accès à tout ».
- **Anatomie :** un dialogue `medium` en haut de la fenêtre, un champ, une liste de commandes
  (nom, raccourci, groupe).
- **Clavier :** APG *Combobox* avec liste [S5]. Le focus reste dans le champ ; ↑ ↓ et
  `aria-activedescendant` ; Entrée exécute ; Échap ferme.
- La liste vient du **même registre** que les menus, la barre et les raccourcis : un seul
  registre (P2).
- **Raccourci :** voir la question ouverte Q1 (§11).

---

## 5. Écrans et parcours

Les cinq tâches de référence sont : créer, partager, relire, présenter, exporter
(`ux-principles.md`). Chaque parcours ci-dessous dit l'état d'entrée, les étapes, les messages
et la sortie.

### 5.1 Premier lancement (accueil)

Règle 8 : « première minute soignée ; aucune question réseau avant le premier partage ».

1. **Écran d'accueil** : `StatusPage` plein écran, centré, largeur ≤ 480px.
   - Titre `--text-title-1` « Bienvenue dans Inkwell ».
   - Une phrase : « Écrivez seul ou à plusieurs, sans compte et sans serveur. »
   - Champ « Votre nom » (obligatoire, 40 caractères au plus), puis choix de **couleur** parmi
     les 8, en groupe radio de pastilles, chaque pastille nommée (« Orange », « Bleu ciel »…).
   - Bouton suggéré « Commencer ».
2. **Projet d'exemple** ouvert, avec l'aperçu visible et le curseur dans le texte. L'aperçu se
   met à jour dès la première frappe.
3. **Rien d'autre** : pas de visite guidée en 6 étapes. Le projet d'exemple montre ce qu'on
   peut faire (titres, image, tableau, une diapositive).

Écart actuel : le nom est inventé en silence (« Personne 1a2b »), et il n'y a pas d'écran
d'accueil (inventaire).

### 5.2 Ouvrir, créer un projet

- **Écran Projets**, atteint par le menu principal › « Projets… » ou Ctrl+O :
  - une liste encadrée des projets récents : nom, dernière modification relative, personnes ;
  - « Nouveau projet » (suggéré) ;
  - « Ouvrir un dossier… » ;
  - « Rejoindre avec une invitation ».
- **Nouveau projet** : un dialogue `small`.
  - Nom.
  - Modèle, en liste radio avec description : Document, Rapport, Lettre, Diapositives, Notes
    Markdown.
  - « Créer ».

  Le dossier est créé dans `Documents/Inkwell/` et le chemin se montre sous le nom, en
  `--text-small`.
- Un projet ouvert remplace le précédent dans la même fenêtre. « Ouvrir dans une nouvelle
  fenêtre » est en v2.
- Vocabulaire : « projet », jamais « dossier » ni « dépôt », sauf l'action « Ouvrir un
  dossier… » qui désigne un dossier du disque.

### 5.3 Partager (inviter)

Point d'entrée : bouton « Partager » de la barre d'en-tête → dialogue `medium`
« Écrire à plusieurs ».

1. **Rôle** : groupe de bascules « Écriture | Lecture », avec une phrase sous le choix
   (« Peut modifier le texte » / « Peut lire et commenter »). « Admin » ne s'offre pas ici ; il
   se donne depuis Personnes.
2. **« Créer une invitation »** (suggéré). Ensuite :
   - l'invitation apparaît en texte sélectionnable (`--font-mono`) ;
   - « Copier l'invitation », avec un toast « Invitation copiée » ;
   - une durée de validité en clair (« Valable 24 heures, pour une personne »).
3. **Personnes à proximité** (même réseau) : une liste encadrée, nom et empreinte en 4 mots.
   « Inviter » sur la ligne. L'interrupteur « Me rendre invisible » est en bas.
4. **Demandes reçues** : « Léa demande à rejoindre · Accepter · Refuser ». Elles s'affichent
   aussi en bandeau si le dialogue est fermé (priorité 4 bis, admin seulement).
5. Note de confiance en bas : « Toute personne qui a cette invitation peut rejoindre le
   projet. Ne la partagez qu'avec elle. »

Aujourd'hui, le dialogue mêle inviter et rejoindre, et n'expose ni le rôle, ni la proximité,
ni les demandes. Le Go les fournit déjà (`internal/app/invites.go`).

### 5.4 Rejoindre

- **Entrées :**
  - Écran Projets › « Rejoindre avec une invitation » ;
  - ou le collage d'une invitation dans la fenêtre (détection) ;
  - ou une personne à proximité qui invite.
- **Étapes :**
  1. Champ « Invitation » et « Rejoindre ».
  2. « Connexion à Marie… » : indicateur d'attente, et au-delà de 3s, « Marie doit avoir
     Inkwell ouvert ».
  3. **Vérifier l'empreinte** : « Vérifiez avec Marie que vous voyez les mêmes mots :
     *pomme · rivière · lampe · nord* ». Boutons « Ce sont les mêmes » et « Ils sont
     différents ».
  4. Réception avec progression (« Réception : 12 sur 40 fichiers, 3,2 Mo ») ; le fichier
     principal s'ouvre dès qu'il est là.
  5. Toast « Vous avez rejoint « Rapport annuel » ».
- **Erreurs** (une phrase + une action) :

| Cas | Texte |
|---|---|
| invitation invalide | « Cette invitation n'est pas complète. Copiez-la de nouveau en entier. » |
| déjà utilisée | « Cette invitation a déjà servi. Demandez-en une nouvelle à Marie. » |
| hôte absent | « Marie n'est pas joignable. Elle doit avoir Inkwell ouvert. **Réessayer** » |
| empreintes différentes | « Par prudence, Inkwell n'a pas rejoint ce projet. Vérifiez l'invitation avec Marie. » |

### 5.5 Éditeur — commun à tous les formats

- Zones de §3. Le curseur est dans le texte à l'ouverture.
- **Enregistrement** (exigence T11) :
  - mode automatique : rien à voir ;
  - mode Ctrl+S : un point dans le titre, « Non enregistré » dans la barre d'état ;
  - fermer ne pose **aucune** question bloquante (SAV-03).
- **Modifications hors d'Inkwell** (exigence T11) : toast « 3 lignes intégrées depuis le
  disque · Voir · Annuler ». Les lignes sont teintées 12s. Le mode « Demander » utilise un
  bandeau (TO-2).
- **Zoom du texte** : Ctrl+= / Ctrl+- / Ctrl+0, de 50 à 300 %, et un indicateur dans la barre
  d'état.
- **Rechercher** : Ctrl+F (fichier), Ctrl+H (remplacer), Ctrl+Maj+F (projet, onglet du tiroir).
  Le module existe (`editor/search`, `ui/search`) mais n'est pas branché (inventaire).

### 5.6 Éditeur Typst

- Barre de mise en forme Typst, avec souligné et formule.
- Aperçu à droite (§4.18).
- Problèmes dans la barre d'état : un clic sur un problème amène à la ligne.
- Saut source ↔ aperçu : Ctrl+Alt+J, et clic dans une page.
- Menu Insérer : image, tableau, figure, citation, inclusion, note de bas de page, saut de
  page, pièce jointe, tâche.
- **Message de paquet manquant** : il garde l'action et cache le chemin technique derrière
  « Détails techniques ».

### 5.7 Éditeur Markdown

- Barre Markdown : sans souligné, avec cases à cocher.
- **Mise en page** (B8) : menu dans la barre d'état à côté du langage, en deux groupes :
  - « Pour imprimer (PDF) » : modèles Typst, aperçu en pages ;
  - « Pour l'écran (HTML) » : styles CSS, aperçu en colonne `--text-document` dans une iframe
    isolée.
- Le nom du choix est affiché (« Rapport », « Lecture »), jamais « cmarker » ni « CSS ».

### 5.8 CSV et texte

- **CSV / TSV** : le tableur (§4.19), pas de barre de mise en forme, pas d'aperçu. Une barre
  secondaire propose « Afficher le texte ».
- Un **`.txt` tabulaire** propose « Ouvrir comme tableau » dans une barre secondaire, jamais
  d'office.
- **Fichier très lourd** : lecture seule, et un bandeau qui dit pourquoi et quoi faire
  (« Ce fichier est très lourd : il s'ouvre en lecture. **Ouvrir quand même en écriture** »).

### 5.9 Présenter

1. Entrée : bouton « Présenter » (F5), menu › « Présenter depuis cette diapositive » (Maj+F5),
   « Afficher la vue public sur cet écran ».
2. Deux écrans détectés : le public va sur l'écran externe, le présentateur sur l'écran
   principal, et « Échanger les écrans » est dans la vue présentateur. Un seul écran : la vue
   public est une fenêtre déplaçable.
3. **Les autres personnes** voient le bandeau « Marie présente · **Suivre** ». En suivant, elles
   peuvent « Parcourir librement » puis « Revenir au présentateur ».
4. « Prendre la main » se demande. Le pilote voit un bandeau dans sa vue : « Paul demande à
   prendre la main · Accepter · Refuser ».
5. Quitter (Échap) : retour à l'éditeur, le curseur sur la diapositive qui était affichée.

### 5.10 Préférences (Ctrl+,)

Dialogue de préférences (§4.10), groupes dans cet ordre :

| Groupe | Réglages | Portée |
|---|---|---|
| **Apparence** | Thème : Système · Clair · Sombre ; Taille du texte (zoom par défaut) ; Ligatures de code (Adwaita Mono / JetBrains Mono) | cet ordinateur |
| **Enregistrement** | Automatique · Avec Ctrl+S ; Délai ; Modifications hors d'Inkwell : Intégrer · Demander | ce projet |
| **Moi** | Nom, couleur, empreinte (4 mots) ; « Me rendre invisible » | cet ordinateur |
| **Projet** (admin) | Supprimer un fichier : sur accord d'un admin · librement | ce projet, signé |
| **Avancé** | Détails techniques (interrupteur, glossaire) ; Langue de l'interface | cet ordinateur |

- Chaque réglage s'applique tout de suite, sans bouton « Appliquer ».
- La portée est écrite sous le groupe (« Pour ce projet seulement »).

### 5.11 Personnes

Dialogue `medium`, ouvert par la pile d'avatars.
- **Une ligne par personne :**
  - avatar, nom, puis « (vous) » ;
  - état : « Présente maintenant », « Vue il y a 2 heures », « Accès suspendu par Marie à
    14:10 » ;
  - rôle dans un menu ;
  - actions : Déconnecter / Laisser revenir, Passer la main, Retirer (`.destructive`,
    confirmation en ligne).
- **Ordre :** vous, puis les admins, puis les personnes présentes, puis les autres, par nom.
- **Changer de rôle** : immédiat, avec un toast « Paul est en lecture · Annuler ».
- **Déconnecter** : réversible (« Laisser revenir »), donc pas de confirmation. On montre un
  toast avec « Annuler ».

  Aujourd'hui, une confirmation en ligne est demandée : écart avec P3 (inventaire).
- **Passer la main, retirer** : confirmation (§4.9).
- **Une personne non admin** voit la liste sans contrôles, avec la note « Seul un admin change
  les rôles ».
- **Seul dans le projet** : écran vide « Personne d'autre n'a encore rejoint ce projet », avec
  le bouton « Partager ». La formule est neutre (§6.3) ; on n'écrit pas « Vous êtes seul ».

### 5.12 Erreurs, hors ligne, divergences

| Situation | Composant | Texte (fr) | Action |
|---|---|---|---|
| hors ligne < 30s | indicateur seul | « Hors ligne — rien n'est perdu » | — |
| hors ligne ≥ 30s | bandeau `info` | « Vous êtes hors ligne. Vos modifications sont gardées et partiront au retour du réseau. » | « Réessayer maintenant » |
| écriture disque impossible | bandeau `error`, `alert` | « Le fichier n'a pas pu être écrit sur le disque. Votre texte reste dans Inkwell ; nouvel essai dans 10 s. » | « Réessayer », « Détails » |
| fichier supprimé dehors | bandeau `warning` | « « ch2.typ » a été supprimé du disque. Son texte reste dans Inkwell. » | « Recréer le fichier », « Fermer » |
| fichier devenu binaire | bandeau `warning` | « « ch2.typ » n'est plus un texte sur le disque. Inkwell garde sa version. » | « Garder la version d'Inkwell » |
| modifié dehors, mode Demander | bandeau `info` | « « ch2.typ » a été modifié hors d'Inkwell. » | « Intégrer », « Garder la version d'Inkwell » |
| **divergence** (modifications au même endroit) | marque dans la marge + toast | « Paul et vous avez modifié le même passage. Les deux versions sont gardées. » | « Voir » (ouvre la comparaison) |
| projet scellé | bandeau `warning` | une phrase qui dit pourquoi et qui peut l'ouvrir | « Voir les personnes » |
| lecture seule | bandeau `info` | « Vous lisez ce projet sans pouvoir le modifier. » | « Demander l'accès en écriture » |
| suspendu | bandeau `info` | « Marie a suspendu votre accès à 14:10. Vous pouvez lire, pas modifier. » | « Demander à revenir » |
| erreur inattendue | toast `alert` | « Quelque chose s'est mal passé. Rien n'est perdu. » | « Détails » (copie un rapport, sans données personnelles) |

- **E-1.** Le mot « conflit » NE DOIT PAS apparaître (glossaire) : on dit « divergence » ou
  « même passage ».
- **E-2.** Un message d'erreur NE DOIT PAS afficher une raison technique brute (`{reason}`) dans
  la phrase. Elle va dans « Détails », derrière l'interrupteur « Détails techniques ».
  Aujourd'hui, `projection.failed.reason` l'insère dans la phrase.

### 5.13 À propos

`AdwAboutDialog` [S1] : icône, nom, version SemVer, une phrase, licence, crédits. La révision
(« commit ») n'apparaît que sous « Détails techniques ».

---

## 6. Contenu et ton

### 6.1 Voix

- On **vouvoie**. On écrit à la voix active, au présent, en phrases courtes (≤ 20 mots).
- Ton calme, sans point d'exclamation, sans « Oups » ni « Désolé ».
- On dit **ce qui est préservé** chaque fois que la personne pourrait craindre une perte
  (« rien n'est perdu », « votre texte reste dans Inkwell »).
- Les personnes sont nommées par leur nom (« Paul a… »), jamais « un pair », « un
  utilisateur » ni « quelqu'un » quand le nom est connu.

### 6.2 Forme

| Élément | Règle | Exemple |
|---|---|---|
| bouton, élément de menu | verbe à l'infinitif, majuscule initiale seulement, sans point | « Enregistrer une version » |
| titre de dialogue | nom ou question, sans point final (sauf « ? ») | « Écrire à plusieurs », « Retirer Paul ? » |
| bandeau, toast | une phrase, avec point final si elle est complète | « Vous êtes hors ligne. » |
| infobulle | nom de l'action, puis syntaxe et raccourci, sans point | « Gras `*texte*` Ctrl+B » |
| points de suspension | caractère « … » (U+2026), seulement si l'action demande une saisie de plus | « Ouvrir un dossier… » |

- Le français emploie la majuscule de phrase. La « capitalisation de titre » de la HIG ne vaut
  que pour l'anglais : `en.json` la suit [S2].

### 6.3 Typographie française et inclusivité

- **Espaces insécables** [S9] :
  - U+202F (espace fine) avant `; ! ?` et entre un nombre et `%` ;
  - U+00A0 avant `:` et à l'intérieur de « » ;
  - U+00A0 entre un nombre et son unité (« 3,2 Mo »).

  Aujourd'hui, 79 messages de `fr.json` ont une espace ordinaire à ces endroits : une ligne
  peut commencer par « ? ». Un test DOIT le vérifier.
- **Guillemets** « » pour les noms de fichiers et de versions. Les apostrophes sont typographiques
  (’) dans le texte affiché ; `fr.json` PEUT garder `'`, mais le test l'accepte alors des deux
  façons.
- **Nombres et dates** par `Intl` (`NumberFormat`, `DateTimeFormat`, `RelativeTimeFormat`,
  `PluralRules`) [S10]. On n'écrit jamais de règle de pluriel à la main. « aujourd'hui 14:10 »,
  « hier 17:42 », sinon « 3 nov. 14:10 ».
- **Accord neutre** : la personne n'a pas de genre connu. On évite les participes et adjectifs
  qui s'accordent avec « vous » ou avec le nom d'une personne : on reformule à la voix active.

| À éviter | À écrire |
|---|---|
| « Seul » (statut) | « Sur cet ordinateur » |
| « {name} vous a déconnecté » | « {name} a suspendu votre accès » |
| « Vous êtes connecté » | « Connexion établie » |
| « Bienvenue, prêt ? » | « Bienvenue dans Inkwell » |

- **Longueur** : prévoir **+30 %** pour l'anglais et l'allemand, à terme. Aucun libellé ne
  dépend d'une largeur fixe.

### 6.4 Vocabulaire figé

Il complète le glossaire. Un même mot pour une même chose, partout : menus, infobulles, toasts,
aide, tests.

| Concept | Terme d'interface | Jamais |
|---|---|---|
| inviter quelqu'un | **Partager** (bouton), **Inviter** (action) | « héberger », « host » |
| entrer dans un projet | **Rejoindre** | « se connecter à », « join » |
| couper une personne, réversible | **Déconnecter** / **Laisser revenir** | « bannir », « suspendre » (dans un bouton) |
| retirer définitivement | **Retirer** | « supprimer » une personne |
| version gardée | **Enregistrer une version**, **version nommée** | « commit », « tag », « snapshot » |
| retour en arrière | **Revenir à ce moment** | « restaurer », « rollback » |
| annuler un effet | **Annuler** | « défaire » |
| garder le texte local | **Garder la version d'Inkwell** | « ignorer », « écraser » |
| incorporer le texte du disque | **Intégrer** | « importer », « fusionner » |
| rendu | **Aperçu** | « preview », « rendu » |
| moteur | **mise en page** | « compilation », « typst », « tinymist » |
| erreur du document | **problème** | « diagnostic », « erreur de compilation » |

---

## 7. Accessibilité

Le niveau visé est WCAG 2.2 **AA**, plus quelques critères AAA désignés [S4]. Chaque exigence a
sa preuve en §10.

| # | Exigence | Critère |
|---|---|---|
| A-1 | Texte ≥ 4,5:1, grand texte (≥ 18,66px gras ou 24px) ≥ 3:1, en clair, en sombre et en contraste élevé. | 1.4.3 |
| A-2 | Composants, états et icônes porteuses de sens ≥ 3:1 contre leur voisinage, **anneau de focus compris**. | 1.4.11 |
| A-3 | Rien n'est porté par la seule couleur : personnes, différences, problèmes, état de synchronisation. | 1.4.1 |
| A-4 | Tout se fait au clavier, sans piège ; ordre de tabulation = ordre visuel. | 2.1.1, 2.1.2, 2.4.3 |
| A-5 | Focus toujours visible (`:focus-visible`), jamais masqué par un toast, un bandeau collant ou un popover. | 2.4.7, 2.4.11 |
| A-6 | L'anneau fait 2px et contraste à 3:1 : cela vise aussi 2.4.13 (AAA), sans coût. | 2.4.13 |
| A-7 | Cibles ≥ 24×24px ; 34px dans les barres, 44px dans les dialogues. | 2.5.8 |
| A-8 | Toute action par glisser a une alternative (redimensionner, déplacer un fichier, réordonner). | 2.5.7 |
| A-9 | Infobulles et popovers au survol : fermables par Échap, survolables, persistants. | 1.4.13 |
| A-10 | Messages d'état annoncés sans déplacer le focus (`status`, `alert`, `log`), sans flot : pas plus d'une annonce par seconde pour un même sujet. | 4.1.3 |
| A-11 | Reflow à 320px CSS, zoom 200 %, espacement du texte. | 1.4.10, 1.4.4, 1.4.12 |
| A-12 | Mouvement réduit respecté (T-11) ; rien ne clignote. | 2.3.3 (AAA), 2.3.1 |
| A-13 | Durées réglables : les toasts se suspendent au survol et au focus ; aucune action à faire dans un temps limité, sauf une invitation, qui expire, à l'écrit. | 2.2.1 |
| A-14 | Langue déclarée : `<html lang="fr">` ou `en`. Les noms de fichiers en langue étrangère ne sont pas balisés. | 3.1.1 |
| A-15 | Aide cohérente : « Raccourcis clavier » et « À propos » sont toujours au même endroit du menu principal. | 3.2.6 |
| A-16 | Pas de saisie redondante ; aucune épreuve cognitive pour rejoindre. L'empreinte se **compare**, elle ne se retape pas. | 3.3.7, 3.3.8 |
| A-17 | Couleurs forcées : tout état reste visible (T-4). | — (Windows HC, `forced-colors`) |
| A-18 | Repères : un `<header>` (bannière), un `<main>`, une `<nav>` ou un arbre (Fichiers), un `<aside>` par panneau, un `<footer>` (barre d'état). F6 passe de zone en zone : Fichiers → barre de mise en forme → texte → aperçu → tiroir → barre d'état. | 1.3.1, 2.4.1 |
| A-19 | L'éditeur est un `textbox` multiligne CodeMirror nommé « Texte de main.typ ». Les curseurs et étiquettes distants sont cachés au lecteur d'écran ; l'arrivée d'une personne s'annonce une fois (« Paul a rejoint »). | 4.1.2 |

---

## 8. Cohérence

### 8.1 Règles

- **K-1.** Avant de créer un composant, chercher dans §4 :
  - réutiliser s'il existe ;
  - sinon, l'ajouter ici d'abord, puis le coder dans `theme/adwaita.css` (classe) ou `ui/`
    (composant).
- **K-2.** Un nouveau panneau devient un onglet du tiroir (L-2). Un nouveau réglage entre dans
  un groupe existant de §5.10. Une nouvelle action entre dans le registre des commandes (menu,
  palette, raccourci, infobulle).
- **K-3.** Chaque écran DOIT être revu en **clair et en sombre**, à 100 % et 200 %, en
  contraste élevé, puis comparé à ses voisins sur des captures (`CLAUDE.md`). Les captures de
  référence (`toHaveScreenshot`) couvrent chaque composant de §4 dans ses états.
- **K-4.** Les textes sont dans `i18n/fr.json` et `en.json`, à parité. Aucun texte en dur, même
  un « × » : il passe par un `aria-label` traduit.
- **K-5.** Un même verbe ne nomme qu'une seule action (§6.4). Un même raccourci ne fait qu'une
  seule chose dans un même contexte (§8.2).

### 8.2 Raccourcis

Un seul registre, affiché dans les menus, les infobulles, la palette et la fenêtre
« Raccourcis clavier » (Ctrl+?).

| Action | Raccourci | État |
|---|---|---|
| Enregistrer (mode Ctrl+S) | Ctrl+S | fait |
| Rechercher / Remplacer / Dans le projet | Ctrl+F / Ctrl+H / Ctrl+Maj+F | codé, non branché |
| Suivant / précédent (recherche) | F3 / Maj+F3 | codé |
| Gras, italique, souligné | Ctrl+B, Ctrl+I, Ctrl+U | fait |
| Lien | Ctrl+K | fait — **conflit avec la palette** (Q1) |
| Titres 1–4 | Ctrl+1…4 | fait |
| Barre de mise en forme | Ctrl+Maj+T | fait |
| Fichiers / Historique | F9 / Ctrl+Maj+H | à faire (F9 = barre latérale, convention GNOME) |
| Aperçu | à définir avec Q1 (Ctrl+Maj+P est proposé pour la palette) | à faire |
| Source → aperçu | Ctrl+Alt+J | fait (la HIG déconseille Alt [S2] ; gardé, convention de tinymist) |
| Présenter / depuis ici | F5 / Maj+F5 | fait |
| Différence suivante / précédente | F7 / Maj+F7 | fait |
| Renommer un fichier | F2 | à faire |
| Zoom du texte | Ctrl+= / Ctrl+- / Ctrl+0 | fait |
| Menu principal | F10 | à faire |
| Menu contextuel | Maj+F10, touche Menu | tableur seulement |
| Zone suivante / précédente | F6 / Maj+F6 | à faire |
| Préférences | Ctrl+, | à faire |
| Raccourcis clavier | Ctrl+? | à faire |
| Projets | Ctrl+O | à faire |

`Ctrl` = `Cmd` sur macOS (`Mod` dans le code).

### 8.3 Icônes par idée

`side` Fichiers · `eye` Aperçu · `branch` Historique · `present` Présenter · `menu` menu
principal · `more` Plus · `close` Fermer · `check` coche ou « aucun problème » · `fail`
erreur · `warn` avertissement · `undo` Annuler · `copy` Copier · `gear` réglage local · `link`
lien · `at` personne citée.

---

## 9. Anti-motifs

Ce qu'on NE DOIT PAS faire, avec la règle qui l'interdit.

| Anti-motif | Pourquoi | À la place |
|---|---|---|
| « Êtes-vous sûr ? » pour une action annulable | P3, HIG [S2] | toast « Annuler » |
| Toast qui pose une question ou ne part pas seul | TO-2 | bandeau ou dialogue |
| Plusieurs bandeaux empilés | L-8, `AdwBanner` montre un état | un bandeau, par priorité |
| Bandeau sans action alors qu'on peut agir | BN-1, règle 5 | ajouter l'action |
| Raison technique dans la phrase (`ENOSPC`, chemin, hash) | P5, E-2 | « Détails » |
| `title=""` comme infobulle | TT-2, WCAG 1.4.13 | composant Infobulle |
| Couleur, ombre ou rayon bruts dans un composant | T-1 | un jeton |
| `opacity` sur du texte | §2.2, A-1 | `--fg-dim` |
| Contrôle grisé sans explication | P4 | le masquer et expliquer (bandeau), ou une infobulle « pourquoi » |
| Chaque dialogue avec son propre CSS | P2 | `Dialog.svelte` |
| Couleur seule pour une personne ou un état | A-3, C-PERSON | nom, icône, texte |
| Animation infinie ou décorative | T-11, P7 | rien, ou un indicateur d'attente |
| Barre d'outils où chaque bouton est un arrêt de tabulation | F-1, APG [S5] | tabindex itinérant |
| Liste en `<button>` sans flèches (arbre, onglets) | SB-1, §4.5 | motifs APG |
| Le focus part dans l'éditeur à la fermeture d'un menu non lié | M-1 | le rendre au déclencheur |
| Un même mot pour deux actions, ou deux mots pour une | K-5 | §6.4 |
| Jargon : pair, commit, conflit, merge, CRDT, hash, UTF-8 en clair | P5, glossaire | glossaire ; « Détails techniques » |
| Un panneau flottant de plus | L-2 | un onglet du tiroir |
| Une question réseau avant le premier partage | règle 8 | rien jusqu'à « Partager » |

---

## 10. Preuves

Comment chaque famille de règles se vérifie. Une règle sans preuve n'est pas finie.

| Règles | Preuve | Où |
|---|---|---|
| T-1, T-6 | règle stylelint : interdire les couleurs, ombres et rayons bruts dans `*.svelte` (`color-no-hex` et `declaration-property-value-disallowed-list` hors `theme/`) | `mise run lint` (à ajouter) |
| T-5, §2.2 | parité `tokens.json` ↔ `tokens.css` et contrastes calculés | `theme/tokens.test.ts` (à étendre) |
| A-1, A-2, A-17 | axe-core dans Playwright, clair, sombre et `forced-colors`, sur chaque écran de §5 | `frontend/e2e/a11y.spec.ts` |
| A-4, F-1, SB-1, M-1, §4 clavier | tests de composants (Testing Library + `userEvent.keyboard`) par motif APG | `*.test.ts` |
| A-7 | test qui mesure `getBoundingClientRect` de chaque `button, [role=button]` ≥ 24px | e2e |
| A-11, L-5 | captures à 360×480, 1280×800 à 200 %, et 320px CSS sans défilement horizontal | e2e `toHaveScreenshot` |
| T-11, A-12 | e2e avec `reducedMotion: "reduce"` : aucune animation en cours (`document.getAnimations()` vide après 1s) | e2e |
| K-3 | captures de référence clair et sombre par composant et par état | e2e (UI-11) |
| K-4, §6.3 | parité fr/en ; espaces insécables ; pas de « ... » ; mots interdits du glossaire absents de `fr.json` | `i18n.test.ts` (à étendre) |
| P4, parcours §5 | skill `ux-walkthrough` (agent `ux-tester`), cinq tâches chronométrées | `docs/ux/walkthrough-*.md` |

---

## 11. Questions ouvertes pour l'utilisatrice

| # | Question | Proposition |
|---|---|---|
| Q1 | **Ctrl+K** : palette de commandes (`ux-principles.md`, règle 3) ou lien (actuel, convention des traitements de texte) ? | Palette sur **Ctrl+Maj+P** (convention VS Code) ; lien garde Ctrl+K. Autre choix : palette sur Ctrl+K, lien sur Ctrl+Maj+K. |
| Q2 | Le titre de la barre d'en-tête : « Inkwell » (actuel) ou le **nom du projet** (H-1) ? | Nom du projet. |
| Q3 | « Déconnecter » une personne : confirmation (actuelle) ou toast « Annuler » (P3) ? | Toast : l'action est réversible. |
| Q4 | Couleur de personne : la choisir à l'accueil (règle 8) ou la dériver de l'identifiant (actuel) ? Avec C-PERSON-2, deux personnes présentes n'ont jamais la même. | La choisir, avec un créneau libre proposé par défaut. |
| Q5 | Recherche dans le projet : onglet du tiroir (L-2) ou panneau à gauche, à la place des Fichiers, comme VS Code ? | Onglet du tiroir. |
| Q6 | Corriger les contrastes (§2.2) en s'écartant de libadwaita pour l'anneau de focus ? | Oui (P6 > P7). |
| Q7 | Langue : seulement fr et en, ou préparer d'autres langues (largeurs +30 %, RTL) dès maintenant ? | fr/en, avec des règles de mise en page déjà neutres (`inset-inline`, pas de largeur fixe). |

---

## 12. Sources

Consultées le 2026-11-01. Les valeurs marquées « non revérifiées » viennent du code source de
libadwaita, pas de sa documentation en ligne.

- [S1] libadwaita (documentation `main`, ≥ 1.8) :
  - variables CSS : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/css-variables.html ;
  - classes de style : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/style-classes.html ;
  - mises en page adaptatives : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/adaptive-layouts.html ;
  - `AdwToast` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.Toast.html ;
  - `AdwBanner` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.Banner.html ;
  - `AdwDialog` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.Dialog.html ;
  - `AdwAlertDialog` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.AlertDialog.html ;
  - `AdwOverlaySplitView` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.OverlaySplitView.html ;
  - `AdwSpinner` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.Spinner.html ;
  - `AdwToggleGroup` : https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/class.ToggleGroup.html.
- [S2] GNOME Human Interface Guidelines : https://developer.gnome.org/hig/
  - *Header bars* `patterns/containers/header-bars.html` ;
  - *Banners* `patterns/feedback/banners.html` ;
  - *Toasts* `patterns/feedback/toasts.html` ;
  - *Popovers* `patterns/containers/popovers.html` ;
  - *Dialogs* `patterns/feedback/dialogs.html` ;
  - *Adaptive* `guidelines/adaptive.html` ;
  - *Keyboard* `guidelines/keyboard.html` ;
  - *Writing style* `guidelines/writing-style.html` ;
  - *Accessibility* `guidelines/accessibility.html` ;
  - *Placeholders* `patterns/feedback/placeholders.html` ;
  - *Spinners* `patterns/feedback/spinners.html`.
- [S4] WCAG 2.2 : https://www.w3.org/TR/WCAG22/ ; Understanding :
  https://www.w3.org/WAI/WCAG22/Understanding/ (`contrast-minimum`, `non-text-contrast`,
  `reflow`, `text-spacing`, `content-on-hover-or-focus`, `focus-visible`,
  `focus-not-obscured-minimum`, `focus-appearance`, `target-size-minimum`,
  `dragging-movements`, `redundant-entry`, `accessible-authentication-minimum`,
  `consistent-help`, `status-messages`).
- [S5] WAI-ARIA Authoring Practices Guide : https://www.w3.org/WAI/ARIA/apg/patterns/
  (`dialog-modal`, `alertdialog`, `menu-button`, `menubar`, `toolbar`, `tabs`, `treeview`,
  `grid`, `combobox`, `switch`, `tooltip`, `windowsplitter`, `button`).
- [S6] MDN :
  - `light-dark()` : https://developer.mozilla.org/en-US/docs/Web/CSS/color_value/light-dark ;
  - `prefers-reduced-motion` : https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-reduced-motion ;
  - `prefers-contrast` : https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-contrast ;
  - `forced-colors` : https://developer.mozilla.org/en-US/docs/Web/CSS/@media/forced-colors ;
  - `color-scheme` : https://developer.mozilla.org/en-US/docs/Web/CSS/color-scheme.
- [S7] Design Tokens Community Group (format `$value` / `$type`) : https://www.designtokens.org/
- [S8] Okabe & Ito, *Color Universal Design* : https://jfly.uni-koeln.de/color/
- [S9] Espace insécable en typographie française :
  https://fr.wikipedia.org/wiki/Espace_ins%C3%A9cable ; équipe française de Fedora :
  https://fedoraproject.org/wiki/L10N_French_Team
- [S10] MDN `Intl` :
  - `DateTimeFormat` : https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat ;
  - `RelativeTimeFormat` : https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/RelativeTimeFormat ;
  - `PluralRules` : https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/PluralRules.
- [RFC 2119] https://www.rfc-editor.org/rfc/rfc2119 · [RFC 8174] https://www.rfc-editor.org/rfc/rfc8174
