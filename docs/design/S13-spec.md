# Spécification de conception — Lot S13

Ce document spécifie les exigences N49 à N57 et U13.
Il reprend les décisions de `docs/architecture/research/preview-view-modes.md` et les constats de `docs/design/icon-audit.md`.
Le registre `docs/exigences.md` prime sur toute recommandation de ce document.
Les références au code sont de la forme `fichier:symbole` et ont été vérifiées dans le graphe gortex le 2026-11-07.

## 1. Exigences détaillées

### N49 — Fenêtre sans barre système (CSD)
- **État actuel.** `cmd/inkwell/main.go:main` crée la fenêtre sans `Frameless` : les décorations système restent. `frontend/src/ui/shell/HeaderBar.svelte:HeaderBar` n'a ni contrôle de fenêtre ni zone de glisser : aucun `--wails-draggable` dans `frontend/src`. `cmd/inkwell/windows_desktop.go:wailsWindows` gère la fenêtre publique de présentation, pas les contrôles de la fenêtre principale.
- **Comportement cible.**
  - Décorations côté client, à la manière d'Adwaita. La barre d'en-tête sert de poignée : `--wails-draggable: drag`.
  - Chaque bouton, champ et menu de l'en-tête porte `--wails-draggable: no-drag`.
  - Des zones de redimensionnement de 6 px bordent la fenêtre (quatre bords, quatre coins).
  - À droite : réduire, agrandir/restaurer, fermer, avec `window-minimize-symbolic`, `window-maximize-symbolic` et `window-close-symbolic` (N56).
  - Un double-clic sur la barre agrandit ou restaure la fenêtre.
  - Cible de 44 px minimum. Les boutons restent visibles en fenêtre étroite.
- **Points d'insertion.** `cmd/inkwell/main.go:main` (`Frameless: true`) ; `HeaderBar.svelte` ; nouveau composant `WindowControls.svelte`.
- **API.** Runtime Wails v3 (`@wailsio/runtime`, déjà importé par `frontend/src/App.svelte:App`) : `Window.Minimise()`, `Window.ToggleMaximise()`, `Window.Close()`. Le mécanisme de redimensionnement d'une fenêtre `Frameless` sous Linux (GTK4) est à confirmer dans `https://v3.wails.io/llms.txt` avant S13.2. Si Wails ne le fournit pas, les zones de bord appellent une méthode de redimensionnement exposée par un service Go.
- **Critère d'acceptation.** Test E2E Playwright avec le module `@wailsio/runtime` simulé :
  1. Un clic sur `[data-testid="window-minimize"]`, `"window-maximize"` et `"window-close"` appelle respectivement une fois `Window.Minimise`, `Window.ToggleMaximise` et `Window.Close` du runtime simulé.
  2. Un double-clic sur la barre appelle `Window.ToggleMaximise`.
  3. Les huit zones de bord existent, et chaque contrôle de l'en-tête a la valeur calculée `no-drag`.
  4. Un glisser sur la zone de bord droite appelle le redimensionnement simulé.
- **Parcours (ux-walkthrough).** Déplacer la fenêtre, la redimensionner par chaque bord, l'agrandir au double-clic, la restaurer, la réduire, la fermer.

### N50 — Aperçu ajusté à la page entière
- **État actuel.** `frontend/src/preview/Preview.svelte:Preview` calcule `zoom` avec `frontend/src/preview/layout.ts:fitWidth` seulement. La hauteur n'est jamais prise en compte.
- **Comportement cible.** La page entière tient dans l'aperçu.
  - Avec la marge `m = 16` px : `zoom = min((vw − 2m) / pw, (vh − 2m) / ph)`.
  - Une page paysage (diapositive) reste aussi en page entière (décision 1).
  - Le zoom se recalcule au redimensionnement.
- **Point d'insertion.** Nouvelle fonction pure `layout.ts:fitPage(vw, vh, pw, ph)`, utilisée par `Preview.svelte`.
- **Critère d'acceptation.** Test unitaire de `fitPage`, en portrait A4 et en paysage 16:9 :
  - `zoom·pw ≤ vw − 32` et `zoom·ph ≤ vh − 32` ;
  - égalité sur l'un des deux axes (la page n'est pas réduite plus que nécessaire).
- **Parcours.** Ouvrir un document A4 puis des diapositives : chaque page est entière, sans défilement initial.

### N51 — Menus et popovers en petite fenêtre
- **État actuel.**
  - `frontend/src/ui/popover/MenuButton.svelte:MenuButton` fixe `maxHeight = Math.max(160, …)` : ce plancher de 160 px fait déborder le panneau en petite hauteur.
  - `frontend/src/ui/toolbar/InsertMenu.svelte:InsertMenu` passe `width={520}` à `MenuButton` : le panneau déborde dans une fenêtre étroite.
- **Comportement cible.**
  - Le popover s'ouvre du côté qui a le plus de place.
  - Sa hauteur suit la place disponible, sans plancher, avec défilement interne.
  - Sa largeur vaut `min(largeur demandée, 100vw − 16px)`.
  - Rien n'est tronqué en 720×480. Le clavier existant reste (`↑`, `↓`, `Début`, `Fin`, `Échap`, `Tab`).
- **Points d'insertion.** `MenuButton.svelte` (place et hauteur), `InsertMenu.svelte` (largeur).
- **Critère d'acceptation.** Test E2E Playwright en 720×480 qui boucle sur :
  - chaque `MenuButton` de l'interface ;
  - le menu contextuel de U13 ;
  - le popover de zoom de N54.
  
  Pour chacun, il vérifie les quatre bords (`top ≥ 0`, `left ≥ 0`, `bottom ≤ innerHeight`, `right ≤ innerWidth`). Il atteint aussi le dernier élément avec `Fin` et vérifie qu'il est visible.
- **Parcours.** Fenêtre en 720×480 : ouvrir « Insérer », aller au dernier élément au clavier, fermer avec `Échap`.

### N52 — Gouttière des numéros de ligne ajustée
- **État actuel.** `frontend/src/editor/setup.ts:theme` impose `minWidth: "56px"` aux éléments de `.cm-lineNumbers`. Les chiffres flottent à droite d'une colonne trop large.
- **Comportement cible.**
  - La largeur suit le nombre de chiffres de la dernière ligne.
  - Une marge droite de 8 px sépare les numéros du texte.
  - La couleur est discrète, en clair comme en sombre.
- **Point d'insertion.** `setup.ts:theme`.
- **Critère d'acceptation.** Test navigateur Playwright, qui fournit une vraie mise en page. Un test Vitest/jsdom mesurerait 0 px et passerait toujours.
  - Il mesure la largeur de `.cm-lineNumbers` avec 5, 50 et 500 lignes.
  - Avec 5 lignes, la largeur est inférieure à 32 px.
  - La largeur croît strictement de 5 à 50 lignes, puis de 50 à 500.
- **Parcours.** Document court, puis franchir 10 et 100 lignes en tapant : la gouttière s'élargit sans saut du texte.

### N53 — Mode « enhance » : éditions facilitées sur place
- **Livrable de S13.7.** Comme le demande le registre (« design + architecture »), ce lot livre seulement deux documents :
  - `docs/design/enhance-mode.md` ;
  - `docs/architecture/enhance-mode.md` (document à écrire).

  Le code viendra dans un lot ultérieur.
- **État actuel.**
  - L'éditeur (`frontend/src/editor/setup.ts:createEditor`) montre le balisage brut.
  - `frontend/src/ui/sheet/Sheet.svelte:Sheet` ne gère que les fichiers tabulaires autonomes.
- **Orientation que les documents doivent détailler.**
  - Détecter `#table(…)` en Typst et les tableaux Markdown. Une grille simple, tirée de `frontend/src/ui/sheet/Grid.svelte`, remplace alors le balisage.
  - Réutiliser le contrat des widgets en ligne du design, `docs/design/spec.md` ligne 1770 (ligne 10 du tableau : « widgets sous une ligne », filet gauche, nom accessible).
  - Le texte reste la source unique (Automerge).
  - Une modification de cellule n'écrit qu'un remplacement minimal, limité à la plage du contenu de la cellule. Le bloc n'est jamais réécrit en entier, car cela écraserait les éditions concurrentes.
- **Critère d'acceptation (documentaire, S13.7).** Les deux documents existent et définissent :
  - la grammaire reconnue ;
  - le calcul des bornes de chaque cellule ;
  - le remplacement minimal ;
  - le comportement quand un pair modifie le même bloc ;
  - les maquettes en clair et en sombre.

  Ils nomment le test de fusion concurrente exigé du lot de code : deux pairs modifient deux cellules du même tableau, et les deux modifications survivent. Les documents passent la double revue Codex + Sonnet.

### N54 — Modes de visualisation de l'aperçu
- **État actuel.**
  - `frontend/src/preview/Preview.svelte:Preview` déclare `mode = $state<"fit" | number>("fit")` : un seul cadrage (la largeur), toujours en défilement continu.
  - La barre de l'aperçu a déjà les boutons `minus` et `plus` et le pourcentage.
  - `frontend/src/store/settings.svelte.ts:Settings` persiste la mise en page par projet (`LAYOUT_KEY = "inkwell.layout"`).
- **Comportement cible (décisions 1 à 6 de preview-view-modes.md).**
  1. **Défaut.** Page entière (`fit-page`, N50) en défilement continu. La largeur reste à un clic. Le choix se mémorise par projet. Une page paysage reste en page entière.
  2. **Double page.** Disponible dans le menu, jamais par défaut. Mode impair/pair : la couverture est seule, comme dans Papers.
  3. **Menu unique sur le pourcentage**, dans le style de Papers, avec `-` et `+` de part et d'autre.
     - Le menu contient : « Ajuster à la largeur », « Ajuster à la page », les paliers 50, 100, 150 et 200 %, puis la disposition (« Défilement continu », « Page par page », « Double page »).
     - En page par page, un sélecteur `< Page X / Y >` apparaît.
  4. **Raccourcis.**
     - `W`, `F`, `C` et `D` agissent seulement quand l'aperçu a le focus.
     - `Ctrl++`, `Ctrl+-`, `Ctrl+0`, `Ctrl+molette` et le pincement agissent quand l'aperçu a le focus ou le pointeur. Ils ne volent jamais le zoom de l'éditeur.
     - Chaque raccourci est affiché dans le menu, à côté de son entrée.
  5. **Point de lecture.**
     - À la souris, le zoom garde fixe le point sous le pointeur.
     - Au clavier, il garde fixe le haut de la page courante.
     - Le changement de mode garde aussi la position de lecture.
  6. **Accessibilité.** Chaque mode est une entrée de menu nommée, avec un état coché visible (`menuitemradio`, `aria-checked`). Aucune action n'est réservée à la souris.
- **Points d'insertion.**
  - `Preview.svelte` : `mode` se sépare en deux axes, `scrollMode` et `zoomMode`.
  - `layout.ts` : `fitPage` et une pile de paires pour la double page.
  - `settings.svelte.ts:Settings` : `previewScrollMode` et `previewZoomMode`.
- **Critère d'acceptation.** Un parcours E2E Playwright par décision :
  1. **Défaut.** À la première ouverture, `fit-page` et « Défilement continu » sont cochés, et la page 1 est entière.
  2. **Persistance.** Après un choix et un rechargement, le même projet garde ce choix ; un autre projet garde le défaut.
  3. **Double page.** La page 1 est seule, puis viennent les paires 2-3 et 4-5.
  4. **Point de lecture.** Le test note la page courante, puis zoome au clavier, au pointeur, et change de mode. Après chaque action, la page courante n'a pas changé et l'élément pointé reste sous le pointeur, à 8 px près.
  5. **Menu.** Chaque mode choisi est seul `aria-checked="true"`, et chaque entrée montre son raccourci.
  6. **Focus.** `W` tapé dans l'éditeur écrit « W ». `W` avec le focus dans l'aperçu passe en largeur. `Ctrl+molette` au-dessus de l'éditeur ne change pas le zoom de l'aperçu.
  7. **Page par page.** `PageDown` et `PageUp` changent de page.
- **Parcours.** Ouvrir un projet, puis essayer chaque mode avec la souris et avec le clavier seul.

### N55 — Synchronisation continue texte ↔ aperçu
- **État actuel.** Les sauts sont ponctuels :
  - aperçu → texte avec `Preview.svelte:Preview` (`pageClick`, `onSourceJump`) ;
  - texte → aperçu avec la cible `onCursorJump`.

  Il n'y a pas de suivi continu.
- **Comportement cible.**
  1. **Texte → aperçu.** Quand le curseur bouge, l'élément d'aperçu de sa ligne reste dans le viewport. En page par page, l'aperçu va à la page du curseur ; en double page, à la paire qui la contient.
  2. **Aperçu → texte.** Un clic dans l'aperçu place le curseur à la source correspondante et la rend visible.
  3. **Interrupteur.** L'entrée cochée « Suivre le curseur » se trouve dans le menu de N54 et se mémorise par projet. Décochée, l'aperçu défile librement. Recochée, l'aperçu se réaligne aussitôt.
  4. **Autres personnes (N45).** Leurs modifications ne déplacent jamais la vue. En mode « suivre une personne », le texte et l'aperçu suivent sa position.
- **Base technique.** La correspondance source ↔ position est fournie par tinymist (jump-to-source, scroll-to-preview). Elle est à vérifier au début de S13.5.
- **Critère d'acceptation.** Tests E2E Playwright, chacun en `continuous`, `single` et `dual`.

  « Aligné » veut dire : le rectangle de l'élément d'aperçu de la ligne du curseur coupe le viewport de l'aperçu, au plus 300 ms après le dernier mouvement.
  1. **Texte → aperçu.** Le test place le curseur sur une ligne lointaine : l'aperçu est aligné.
  2. **Aperçu → texte.** Le test clique un élément de l'aperçu : la ligne du curseur est la ligne source attendue et elle est visible.
  3. **Réactivation.** Le test décoche le suivi et déplace le curseur : l'aperçu ne bouge pas. Il recoche : l'aperçu est aligné.
  4. **Pair simulé.** Un pair simulé modifie et insère du texte ailleurs : la position de défilement locale ne change pas. En mode « suivre » ce pair, la vue va à sa position.
- **Parcours.** Taper et regarder l'aperçu suivre ; cliquer une formule ; couper puis rétablir le suivi.

### N56 — Unification des icônes
- **État actuel.**
  - `frontend/src/ui/icons/paths.ts:ICONS` compte 56 icônes (l'audit en recensait 53). Leurs noms sont courts (`close`, `plus`…).
  - `frontend/src/ui/icons/Icon.svelte:Icon` fixe le trait à 1,6. Certaines entrées le forcent à 1,3, 1,8 ou 2,2.
  - L'audit relève des glyphes Unicode (`×`, `✓`, `›`, `●`, `+`, `−`, `∑`).
  - `frontend/src/editor/changeGutter.ts:theme` dessine un triangle en CSS (`borderLeft` et bordures transparentes) pour marquer une suppression.
- **Comportement cible.**
  - Source unique : GNOME Icon Development Kit (CC0, `docs/design/icon-source.md`). Grille de 16, trait de 2, `currentColor`.
  - Les clés prennent les noms GNOME (`window-close-symbolic`, `list-add-symbolic`, `go-next-symbolic`…).
  - Aucun glyphe Unicode, aucun SVG ad hoc, aucune forme CSS qui imite une icône. Tout passe par `<Icon name="…">`.
- **Points d'insertion.**
  - `paths.ts` et `Icon.svelte` ;
  - les composants relevés par l'audit (`SearchPanel`, `SheetStatus`, `FormatBar`, `Sidebar`) ;
  - `changeGutter.ts:theme`.
- **Critère d'acceptation.** Test Vitest `icons.test.ts` :
  1. Aucun `<svg` brut dans les `.svelte` et `.ts`, hors de `Icon.svelte` et du logo de N57.
  2. Aucun des glyphes listés comme contenu de bouton ou d'étiquette.
  3. Aucune règle de style qui combine `border-*: … transparent` avec une bordure colorée, c'est-à-dire un triangle CSS.
  4. Pour chaque entrée de `paths.ts` : les coordonnées restent dans [0, 16], et aucun `stroke-width` n'est forcé. Le trait vient de `Icon.svelte`, qui le fixe à 2.
- **Parcours.** Captures en clair et en sombre de chaque panneau, comparées aux écrans voisins.

### N57 — Icône de l'application dans l'interface
- **État actuel.**
  - `build/icons/inkwell.svg` (viewBox 128) et `build/icons/inkwell-symbolic.svg` (viewBox 16) existent. Le second a un trait de 1,5 et une couleur fixe `#2e3436`.
  - Aucun des deux n'apparaît dans l'interface.
  - `frontend/src/ui/AboutDialog.svelte:AboutDialog` existe mais n'affiche pas le logo.
  - Il n'existe aucun composant de page d'état (`StatusPage`) dans `frontend/src`.
- **Comportement cible.**
  1. **Petite icône.** `inkwell-symbolic` est redessinée selon N56 (trait de 2, `currentColor`) et ajoutée à `paths.ts`. Elle s'affiche avec `<Icon name="inkwell-symbolic">` à gauche du titre de `HeaderBar.svelte`.
  2. **Grande icône.** Elle s'affiche en **128×128**, sa taille native, par un seul composant `AppLogo.svelte` qui importe `build/icons/inkwell.svg` comme ressource du bundle. Elle apparaît dans `AboutDialog.svelte` et dans une nouvelle page d'état vide, `StatusPage.svelte` (patron Adwaita « status page », pour un projet vide ou l'accueil).
- **Critère d'acceptation.**
  - Test composant : `AppLogo` rend une image de 128×128, et `AboutDialog` et `StatusPage` l'utilisent.
  - Test Playwright de captures comparées (`toHaveScreenshot`), en clair et en sombre, pour l'en-tête, « À propos » et la page d'état vide.
- **Parcours.** Projet vide (logo de 128), puis l'en-tête, puis « À propos », dans les deux thèmes.

### U13 — Gestionnaire de fichiers et onglets
- **Registre.** « Ajouter, supprimer (sur accord), répliqué, historisé, restaurable. » Le rappel du 2026-11-07 ajoute : créer un fichier ou un dossier, renommer, déplacer, un menu contextuel et des onglets. Le registre a tranché ; ce n'est pas une question ouverte.
- **État actuel.**
  - `frontend/src/ui/shell/Sidebar.svelte:Sidebar` liste les fichiers sans action.
  - `frontend/src/workspace/workspace.svelte.ts:Workspace` n'a qu'un fichier courant (`current`) et pas d'onglets.
- **Comportement cible.**
  - **Gestion.** Le bouton `<Icon name="list-add-symbolic">` de l'en-tête de Fichiers ouvre « Nouveau fichier » et « Nouveau dossier ». Sur un fichier, un clic droit, `Shift+F10` ou la touche Menu ouvre un menu contextuel : « Renommer », « Déplacer », « Supprimer ».
  - **Suppression sur accord.**
    - « Sur accord » est défini par l'ADR 0023, § « Suppression sur accord » (B5, FM-05…FM-19), et ses décisions signées (ADR 0030) : l'interface l'applique, elle ne le redéfinit pas.
    - Avec `settings.deletion = "approval"` (par défaut) : une personne qui écrit **demande** la suppression (`nodes[id].deletion`) ; le fichier est marqué « suppression demandée » chez tout le monde ; une personne admin **accepte** (le fichier va à la corbeille) ou **refuse**. En concurrence, le refus l'emporte.
    - Avec `"free"` (réglable par un admin seulement) : une personne qui écrit met directement le fichier à la corbeille.
    - Chaque geste nomme le fichier et dit qu'il vaut pour tout le monde (dialogue Adwaita pour demander ; bandeau ou ligne dans le panneau pour accepter ou refuser).
    - Elle est répliquée à tous les pairs et inscrite dans l'Historique avec son auteur.
    - Le fichier reste restaurable depuis la corbeille (« Restaurer » efface `trashed`), sans limite de temps ; seule la purge, réservée aux admins, le retire de l'index (le texte et l'historique restent).
    - Il n'y a pas de suppression immédiate avec un toast d'annulation de 8 s.
  - **Onglets.** `TabBar.svelte` se place entre l'en-tête et l'éditeur.
    - Chaque onglet a un bouton fermer `<Icon name="window-close-symbolic">`. Le clic milieu ferme aussi.
    - `Ctrl+PageDown` et `Ctrl+PageUp` passent d'un onglet à l'autre.
    - La barre défile horizontalement si elle déborde.
    - Puce de modification : U8 (✅) laisse choisir entre enregistrement automatique et manuel. En mode manuel, la puce existante, `[data-testid="unsaved-dot"]` (`HeaderBar.svelte:HeaderBar`, et `StatusBar.svelte:StatusBar`), se décline par onglet. En mode automatique, il n'y a pas de puce. Ce n'est donc pas incohérent avec Automerge : la puce signale le fichier disque, pas le CRDT.
- **Points d'insertion.**
  - `Sidebar.svelte` : actions, menu contextuel, clavier ;
  - `TabBar.svelte` (nouveau) ;
  - `Workspace` : liste des fichiers ouverts ;
  - restauration : panneau Historique (`frontend/src/ui/history/`).
- **Critère d'acceptation.** Tests E2E Playwright, avec deux instances pour la réplication :
  1. Créer un fichier et un dossier, renommer, déplacer : les deux instances le voient.
  2. Ouvrir trois onglets, passer de l'un à l'autre avec `Ctrl+PageDown`, en fermer un au clic milieu.
  3. Ouvrir le menu contextuel au clic droit, avec `Shift+F10` et avec la touche Menu.
  4. Demander une suppression puis refuser dans le dialogue : rien ne change chez aucun pair.
  5. Accepter : le fichier disparaît chez les deux pairs, et l'Historique montre l'entrée avec son auteur.
  6. Après d'autres modifications, et après redémarrage des deux instances : restaurer depuis l'Historique, puis vérifier que le contenu revient à l'identique chez les deux pairs.
- **Parcours.** Créer deux fichiers, travailler en onglets au clavier, supprimer sur accord, restaurer plus tard.

## 2. Sous-lots et dépendances

Les sous-lots ne sont pas indépendants. L'ordre suit le graphe ci-dessous.

| Lot | Contenu | Dépend de |
|---|---|---|
| S13.1 | Icônes et logo (N56, N57) | — |
| S13.2 | CSD et gouttière (N49, N52) | S13.1 |
| S13.3 | Popovers en petite fenêtre (N51) | S13.1 |
| S13.4 | Page entière et modes d'aperçu (N50, N54) | S13.1, S13.3 (popover de zoom) |
| S13.5 | Synchronisation texte ↔ aperçu (N55) | S13.4 (modes) |
| S13.6 | Fichiers et onglets (U13) | S13.1, S13.3 (menu contextuel) |
| S13.7 | Documents du mode enhance (N53) | — (documentaire) |

Le critère N51 couvre le popover de zoom et le menu contextuel : il est rejoué à la fin de S13.4 et de S13.6.

## 3. Risques

1. **Redimensionnement CSD sous Linux (N49).** Il est incertain avec Wails v3 et GTK4. *Parade :* vérifier dans la doc avant de coder. Sinon, utiliser un service Go de redimensionnement (voir N49, « API »).
2. **Charge de N55 pendant la frappe.** *Parade :* au plus une requête de correspondance par image (≈ 60 Hz), et seulement pour la position du curseur.
3. **Suppression d'un fichier ouvert chez un pair (U13).** *Parade :* l'accord est demandé avant. Chez le pair, l'onglet se ferme avec un bandeau (`BannerHost`) qui propose la restauration.

## 4. Arbitrages

- **N54/N50.** Page entière et défilement continu par défaut (décision 1).
- **N49.** Boutons de fenêtre à droite (GNOME, Windows). Leur ordre selon l'OS est fourni par `WindowControls.svelte`.
- **U13.** Suppression sur accord, répliquée, historisée, restaurable, comme le registre l'exige.
- **N57.** Grande icône en 128×128, la taille native de `inkwell.svg`.

## 5. Changements suite à la relecture

1. **U13, suppression.** Le toast de 8 s est supprimé. La suppression se fait sur accord, répliquée, historisée et restaurable. Les tests couvrent la demande, le refus, l'acceptation par un admin, le refus qui l'emporte en concurrence, le mode `"free"`, et la restauration après redémarrage (FM-05…FM-19 de l'ADR 0023).
2. **N54, décisions.** Les décisions 1 à 6 sont reprises : double page impair/pair, `-`/`+` et paliers 50 à 200 %, `Ctrl` sous le pointeur avec molette et pincement, raccourcis affichés, point de lecture, entrées cochées, mémoire par projet.
3. **N54, critère.** Un parcours E2E par décision, avec l'état coché du menu et la page courante avant et après zoom ou changement de mode.
4. **N55, critère.** Les deux sens, les trois modes, la réactivation et un pair simulé sont testés. « Aligné » est défini : l'élément coupe le viewport en 300 ms au plus.
5. **N53, écriture.** Un remplacement minimal limité à la plage de la cellule. Le test de fusion concurrente est exigé.
6. **N53, livrable.** S13.7 livre deux documents (design et architecture), avec un critère documentaire. Ils citent §1770 et `Sheet`/`Grid`. Le critère fonctionnel passe au lot de code.
7. **N56, cohérence.** `window-close-symbolic` et `list-add-symbolic` remplacent `×` et `+`. Le logo passe par `Icon` ou `AppLogo`, sans SVG ad hoc. Le test couvre les formes CSS, la grille de 16 et le trait de 2.
8. **N52, mesure.** La largeur est mesurée par Playwright, non par jsdom, avec 5, 50 et 500 lignes.
9. **N50, formule.** La formule intègre les marges et la largeur. Elle est testée en portrait et en paysage.
10. **N49, fenêtre.** Les zones de bord, `no-drag` et le redimensionnement sont dans le parcours. Les appels attendus sont nommés : `Window.Minimise`, `Window.ToggleMaximise` et `Window.Close` du runtime v3 simulé. Les noms v2 (`WindowMinimise`…) sont corrigés.
11. **N51 et N57.** N51 boucle sur tous les menus, vérifie les quatre bords et le dernier élément. N57 fixe la taille à 128, ajoute la page d'état vide et les captures clair/sombre comparées.
12. **Vérifications dans le code (gortex) et sous-lots.**
    - Vérifiés : `AboutDialog.svelte:AboutDialog` (sans logo) ; `MenuButton.svelte:MenuButton` avec `Math.max(160, …)` ; `InsertMenu.svelte:InsertMenu` avec `width={520}` ; `Preview.svelte:Preview` avec le mode unique `"fit"` ; le triangle CSS de `changeGutter.ts:theme` ; `minWidth: "56px"` dans `setup.ts:theme` ; les deux SVG de `build/icons/`.
    - Corrigés :
      - il y a 56 icônes, et non 53 ;
      - `AdwStatusPage` n'existe pas et sera créé (`StatusPage.svelte`) ;
      - `inkwell-symbolic.svg` n'est pas conforme à N56 (trait 1,5, couleur fixe) ;
      - les numéros de ligne faux sont remplacés par des `fichier:symbole`.
    - Puce non enregistrée : elle est justifiée par U8 (mode manuel).
    - Menu contextuel : il s'ouvre aussi au clavier (`Shift+F10`, touche Menu).
    - Sous-lots : les dépendances sont déclarées (§2).
