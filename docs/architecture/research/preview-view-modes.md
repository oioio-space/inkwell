# Recherche : modes de visualisation du volet d'aperçu (N54, N50)

## Faits
- **GNOME Papers et Evince** :
  - Modes de disposition : Défilement continu (*Continuous*, touche `C`), Page par page (*Single page*), Double page (*Dual*, touche `D`).
  - Modes de zoom : Ajuster à la largeur (*Fit Width*, touche `W`), Ajuster à la page (*Fit Page*, touche `F`), Zoom avant (`Ctrl++`), Zoom arrière (`Ctrl+-`), Taille 100 % (`Ctrl+0`).
  - Gestes : `Ctrl+molette` et pincement tactile pour le zoom fluide.
  - Présentation : Diaporama plein écran sur `F5`. Plein écran sur `F11`.
  - IHM : Bouton de zoom affichant le pourcentage dans la HeaderBar. Il ouvre un popover contenant les actions de zoom et de disposition.
  - Sources : [GNOME Papers GitLab](https://gitlab.gnome.org/GNOME/papers), [Documentation Evince](https://help.gnome.org/users/evince/stable/).
- **Okular (KDE)** :
  - Modes de vue : *Single Page*, *Facing Pages*, *Facing Pages (Center First Page)*, *Overview*.
  - Défilement continu : Option commutable indépendamment (*Continuous*).
  - Cadrage et zoom : *Fit Page*, *Fit Width*, `Ctrl++`, `Ctrl+-`, `Ctrl+molette`. Présentation : `Ctrl+Shift+P`.
  - Source : [Manuel Okular](https://docs.kde.org/stable5/en/okular/okular/).
- **pdf.js (Mozilla Firefox)** :
  - Modes de défilement (`scrollMode`) : `VERTICAL` (1, défilement continu par défaut), `PAGE` (4, page par page), `HORIZONTAL` (2), `WRAPPED` (3).
  - Modes de planche (`spreadMode`) : `NONE` (0), `ODD` (1, livre), `EVEN` (2).
  - Cadrage : *Page Fit*, *Page Width*, *Automatic*. Paliers de 50 % à 400 %. Présentation : `Ctrl+Alt+P`.
  - Source : [Dépôt pdf.js](https://github.com/mozilla/pdf.js).
- **macOS Preview (Aperçu)** :
  - Modes de vue : Défilement continu (`Cmd+1`), Page unique (`Cmd+2`), Deux pages (`Cmd+3`).
  - Cadrage et zoom : Ajuster à la fenêtre (`Cmd+9`), Taille réelle (`Cmd+0`), Zoom avant/arrière (`Cmd++` / `Cmd+-`). Diaporama : `Shift+Cmd+F`.
  - Source : [Guide d'utilisation Aperçu macOS](https://support.apple.com/guide/preview/).
- **Motifs GNOME HIG / libadwaita** :
  - Bouton de zoom dans la barre d'en-tête avec libellé en pourcentage ouvrant un popover (`AdwPopover`).
  - Groupes de boutons liés (`.linked`) ou boutons partagés (`AdwSplitButton`).
  - Standard gestuel : `Ctrl+molette` et pincement multitouch.
  - Source : [GNOME Human Interface Guidelines](https://developer.gnome.org/hig/).
- **Approches de rendu webview pour Typst** :
  - SVG par page via `<img src="data:image/svg+xml;base64,...">` : Rendu vectoriel net sur écrans HiDPI sans recalcul matriciel côté Go. La balise `<img>` isole le DOM et bloque l'exécution de scripts malveillants.
  - Canvas (approche pdf.js) : Rendu matriciel lourd en mémoire RAM à haute résolution (`devicePixelRatio`). Typst ne fournit pas d'instructions canvas natives.
  - Défilement continu virtualisé : Calcul de la pile (`tops`, `heights`) hors DOM. Seules les pages visibles dans le viewport sont instanciées.
  - Maintien du point de lecture au zoom : Ancrage de `scrollTop` sur la `currentPage` ou le centre visible pour éviter tout décrochage.

## Existant dans inkwell (file:symbol)
- `frontend/src/preview/Preview.svelte:Preview` : Composant racine du volet d'aperçu.
- `frontend/src/preview/Preview.svelte:mode` : Variable d'état `$state<"fit" | number>("fit")`. Seule la largeur est gérée actuellement (`fitWidth`).
- `frontend/src/preview/Preview.svelte:zoom` : Propriété dérivée calculant le facteur d'échelle à partir de `mode` et `viewW`.
- `frontend/src/preview/Preview.svelte:layout` : Appel à `stack(...)` pour générer la pile verticale des pages.
- `frontend/src/preview/Preview.svelte:rendered` : Virtualisation continue via `visiblePages(...)`.
- `frontend/src/preview/Preview.svelte:tools` : Barre d'outils locale avec boutons `minus`, `plus`, indicateur de pourcentage `{percent} %` et cible curseur `onCursorJump`.
- `frontend/src/preview/Preview.svelte:pageClick` : Clic sur une page pour déplacer le curseur du texte via `onSourceJump`.
- `frontend/src/preview/layout.ts:fitWidth` : Calcul du zoom d'ajustement à la largeur. Aucune fonction `fitPage` ou `fitHeight` n'existe (N50 non couvert).
- `frontend/src/preview/layout.ts:stepZoom` : Paliers discrets de zoom (25 % à 400 %).
- `frontend/src/preview/layout.ts:stack` : Empilement linéaire des pages avec espacement fixe `GAP = 16`.
- `frontend/src/preview/layout.ts:visiblePages` : Calcul géométrique des pages visibles sans mesure DOM.
- `frontend/src/preview/layout.ts:currentPage` : Identification de la page occupant le plus d'espace visible.
- `frontend/src/preview/svg.ts:pageImage` : Conversion sécurisée du SVG Typst en image Base64.
- `frontend/src/preview/preview.svelte.ts:PreviewModel` : Modèle d'état réactif global du document compilé.
- `frontend/src/store/settings.svelte.ts:Settings` : Persistance par projet dans `localStorage` via la clé `inkwell.layout`.
- `frontend/src/App.svelte:present` : Mode présentation déjà implémenté sur `F5` et `Shift+F5`.
- **Points d'insertion pour N54** :
  - Découpler `mode` dans `Preview.svelte` en deux axes : `scrollMode` (`"continuous" | "single"`) et `zoomMode` (`"fit-width" | "fit-page" | number`).
  - Ajouter `fitPage` dans `layout.ts` (ajustement contraint par la largeur et la hauteur du viewport, couvrant N50).
  - Remplacer le bouton de zoom dans `Preview.svelte:tools` par un menu popover Adwaita et insérer un sélecteur de page en mode page unique.

## Options comparées
- **Option 1 : Architecture orthogonale (style GNOME Papers / pdf.js)**
  - *Principe* : Deux dimensions indépendantes : mode de défilement (`continuous` vs `single`) et mode de zoom (`fit-width`, `fit-page`, ou libre).
  - *Avantages* : Conforme aux lecteurs PDF modernes et à libadwaita. Généralise élégamment N50 (`fit-page`). Grande souplesse pour l'utilisatrice.
  - *Inconvénients* : Nécessite un menu popover structuré pour éviter d'encombrer la barre d'outils.
  - *Performance (documents volumineux)* : Optimale. En continu, la virtualisation existante ne garde que 2 à 3 pages en mémoire. En page par page, une seule image SVG est présente dans le DOM.
  - *Accessibilité* : Raccourcis directs mnémotechniques (`W`, `F`, `C`). Navigation clavier fluide via `PageUp`/`PageDown`. Annonce ARIA de la page active.
- **Option 2 : Modes monolithiques fusionnés**
  - *Principe* : Sélecteur unique proposant des modes combinés figés (« Défilement largeur », « Page entière centrée », « Zoom libre »).
  - *Avantages* : Un seul bouton ou menu déroulant simple à implémenter.
  - *Inconvénients* : Rigide. Impossible de zoomer librement en mode page par page ou d'ajuster en largeur une page unique. Éloigné des standards.
  - *Performance (documents volumineux)* : Identique à l'option 1 en charge mémoire.
  - *Accessibilité* : Moins intuitive. Oblige à parcourir une liste plate d'options hétérogènes.

## Recommandation
- **Ensemble des modes retenus** :
  - Défilement : Défilement continu (`continuous`) et Page par page (`single`).
  - Cadrage et zoom : Ajuster à la largeur (`fit-width`), Ajuster à la page entière (`fit-page`, validant N50), Zoom libre par paliers (25 % à 400 %).
- **Mode par défaut** : Défilement continu avec ajustement à la largeur (`continuous` + `fit-width`).
- **Contrôles dans la barre de l'aperçu (`Preview.svelte`)** :
  - Conserver les boutons rapides `-` et `+`.
  - Transformer le bouton central `{percent} %` en bouton menu Adwaita ouvrant un popover :
    - Cadrage : « Ajuster à la largeur » (`W`), « Ajuster à la page » (`F`).
    - Paliers prédéfinis (50 %, 100 %, 150 %, 200 %).
    - Séparateur, puis sélection de disposition : « Défilement continu » (`C`) et « Page par page ».
  - En mode page par page, afficher un sélecteur de page compact `< Page X / Y >` dans la barre.
- **Raccourcis clavier (actifs au focus de l'aperçu)** :
  - `W` : Ajuster à la largeur (*Fit Width*).
  - `F` : Ajuster à la page entière (*Fit Page* / N50).
  - `C` : Basculer entre défilement continu et page par page.
  - `Ctrl++` / `Ctrl+=` : Zoom avant. `Ctrl+-` : Zoom arrière. `Ctrl+0` : Taille réelle 100 %.
  - `Ctrl+molette` et pincement : Zoom continu fluide.
  - `PageUp` / `PageDown` ou `Flèche Gauche` / `Flèche Droite` : Page précédente / suivante en mode page par page.
  - `F5` : Présentation (déjà implémenté).
- **Maintien du point de lecture** :
  - Recalculer `scroller.scrollTop` lors d'un zoom pour maintenir la position relative dans la `currentPage`.
- **Persistance** :
  - Étendre `Layout` dans `frontend/src/store/settings.svelte.ts` avec `previewScrollMode` et `previewZoomMode` par projet.

## Questions ouvertes pour l'utilisatrice
- Quel doit être le mode par défaut à l'ouverture d'un projet : Ajuster à la page entière (N50) ou Ajuster à la largeur (existant) ?
- Souhaitez-vous inclure le mode double page côte à côte (*Facing pages*, touche `D`) dès maintenant pour les grands écrans ?
- Préférez-vous un menu popover unique sur le pourcentage (style Papers), ou deux boutons distincts dans la barre d'outils ?
- Les raccourcis à une touche (`W`, `F`, `C`) doivent-ils être limités au focus sur l'aperçu ou accessibles globalement ?

## Décisions (2026-11-07, choix confiés à Claude par l'utilisatrice : « les meilleurs choix pour l'expérience »)

1. **Mode par défaut : page entière** (`fit-page`) + défilement continu. Raison : N50 est la demande explicite de l'utilisatrice (voir la page entière à côté du texte) ; « largeur » reste à un clic. Le choix fait se mémorise par projet ; une page plus large que haute (paysage, diapositive) reste en page entière.
2. **Double page côte à côte : oui, dans le menu, jamais par défaut.** Utile pour un livre ou un grand écran ; même moteur de mise en page virtualisée, coût faible. Mode impair/pair (couverture seule) comme Papers.
3. **Un seul menu sur le pourcentage (style Papers)**, avec `-` et `+` de part et d'autre. Raison : cohérence GNOME, une seule place à apprendre, barre de l'aperçu compacte (N51 petites fenêtres).
4. **Raccourcis à une touche (`W`, `F`, `C`, `D`) seulement quand l'aperçu a le focus** : dans l'éditeur, ces touches écrivent du texte. Les raccourcis avec `Ctrl` (`Ctrl++`, `Ctrl+-`, `Ctrl+0`, `Ctrl+molette`) agissent aussi sur l'aperçu seulement quand il a le focus ou le pointeur, pour ne pas voler le zoom de l'éditeur. Chaque raccourci est affiché dans le menu, à côté de son entrée.
5. **Le zoom garde le point de lecture** (le point sous le pointeur, ou le haut de la page courante au clavier). Le passage d'un mode à l'autre aussi.
6. Accessibilité : chaque mode est une entrée de menu nommée (lecteur d'écran), état coché visible ; aucune action seulement à la souris.

Ces choix seront vérifiés au parcours utilisateur (skill `ux-walkthrough`) avant la fusion de S13.

## Synchronisation texte ↔ aperçu (N55, 2026-11-07)

Tous les modes ci-dessus restent **synchronisés en permanence avec la zone de texte**, dans les deux sens : déplacer le curseur montre l'endroit correspondant dans l'aperçu (en page par page : aller à la page), et cliquer dans l'aperçu place le curseur. Le zoom ou le changement de mode ne casse jamais ce lien. Un interrupteur visible (entrée cochée « Suivre le curseur » dans le même menu, état mémorisé par projet) le coupe et le rétablit. Les modifications des autres personnes ne déplacent jamais ma vue (N45), sauf quand je suis l'une d'elles (mode « suivre ») : alors le texte et l'aperçu suivent sa position. Base technique à vérifier en S13 : correspondance source ↔ position fournie par tinymist (jump-to-source / scroll-to-preview).
