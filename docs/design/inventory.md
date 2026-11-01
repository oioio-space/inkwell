# Inventaire de l'interface et écarts avec la spécification (N17)

- **Relevé du 2026-11-01**, sur `main` (`362cb4d`), dans `frontend/src`. Les chemins sont
  relatifs à `frontend/src/`, sauf mention contraire.
- **Méthode :**
  - lecture des composants ;
  - recherches ciblées : couleurs brutes, `title=`, rôles ARIA, tailles de police, rayons,
    animations ;
  - calcul des contrastes des jetons, composés sur leur fond réel (script repris dans
    `spec.md` §2.2).
- **Gravité :**
  - **B** (bloquant) : échec WCAG 2.2 AA, ou parcours principal absent ;
  - **M** (majeur) : incohérence visible, ou motif APG non respecté ;
  - **m** (mineur) : finition.

## 1. Ce qui existe

| Zone | Composant | Fichier | Conformité à la spec |
|---|---|---|---|
| Jetons | 3 niveaux, `light-dark()`, `prefers-contrast`, `prefers-reduced-motion` | `theme/tokens.css:10-151` | bonne base ; 4 valeurs sous le seuil (§2) |
| Classes Adwaita | `.fb .btn .sug .pop .mi .tip .card .boxed .tg` | `theme/adwaita.css` | bonne base ; pas de `.destructive`, opacités mélangées |
| Fenêtre | barre latérale, colonne, tiroir ; requête de conteneur à 560px | `App.svelte:195-315`, `:357-364` | pas de poignées, pas d'états moyen et étroit (§3.3) |
| Barre d'en-tête | bascules, état, titre, pile, Partager, Historique, menu | `ui/shell/HeaderBar.svelte` | proche ; titre, `title=`, F10 manquant |
| Barre de mise en forme | par format, débordement ⋯, Insérer, Titres | `ui/toolbar/FormatBar.svelte`, `BarButton.svelte`, `InsertMenu.svelte`, `HeadingMenu.svelte` | conforme à T11 ; clavier non APG |
| Menus et popovers | `MenuButton` (flèches, Échap, Tab) | `ui/popover/MenuButton.svelte` | bon ; le focus revient à l'éditeur |
| Barre d'état | personnes, problèmes, page, non enregistré, position, mots, zoom, encodage, langage | `ui/shell/StatusBar.svelte` | complète ; rendu ambigu bouton / texte |
| Fichiers | arbre repliable, icônes, « principal » | `ui/shell/Sidebar.svelte` | pas d'arbre ARIA ; actions B5 absentes de l'UI |
| Aperçu | pages, squelette, zoom, problèmes, échecs | `preview/Preview.svelte` | bon ; couleurs brutes |
| Tableur | grille ARIA, barre, filtres, collage | `ui/sheet/*` | bon (grille APG) |
| Historique | tiroir à onglets, filtres, lignes, différences, application | `ui/history/*` | riche ; onglets sans flèches ; interrupteurs en cases |
| Personnes | dialogue, rôles, déconnecter, passer la main | `ui/people/PeopleDialog.svelte` | bon ; confirmation sur action réversible |
| Partage | nom, inviter, copier, rejoindre | `ui/ShareDialog.svelte` | incomplet (§5.3–5.4) |
| Préférences | enregistrement, modifications extérieures | `ui/prefs/Preferences.svelte` | incomplet (§5.10) |
| À propos | dialogue | `ui/AboutDialog.svelte` | révision (« commit ») visible |
| Bandeaux | `Banner` + 6 usages | `ui/Banner.svelte`, `ExternalBanner`, `ProjectionBanner`, `SealedBanner`, `people/AccessBanner`, `present/PresentBanner`, `App.svelte:256` | empilables ; certains sans action |
| Toasts | `Toast` + `NoticeToast` + `ExternalChangeToast` | `ui/Toast.svelte`, `ui/notice.svelte.ts`, `ui/ExternalChangeToast.svelte` | deux sources sans file ; un toast qui pose une question |
| Présentation | présentateur, public, bandeau, superposition | `present/*.svelte` | fonctionnel ; couleurs brutes |
| Icônes | une soixantaine d'icônes symboliques, `FileIcon` à marque colorée | `ui/icons/paths.ts`, `Icon.svelte`, `FileIcon.svelte` | conforme (I-1) |
| i18n | fr/en à parité, `Intl.PluralRules` | `i18n/index.ts`, `i18n/*.json` | typographie française, accords, `lang` |
| Recherche | panneau, champ, arbre de résultats, raccourcis | `ui/search/*`, `editor/search/*` | **codé mais non monté** |
| Absents | accueil, écran Projets, palette, poignées, `StatusPage`, `Dialog`, `ToastHost`, infobulle universelle, fenêtre de raccourcis | — | §4.8, §4.12, §4.20, §4.24, §5.1, §5.2 |

## 2. Écarts

### 2.1 Jetons et contraste

| # | Grav. | Écart | Où | Règle | Correction |
|---|---|---|---|---|---|
| G-1 | **B** | Anneau de focus à 2,62:1 (vue claire), 2,34:1 (barre latérale), 2,63:1 (barre sombre) | `theme/tokens.css:72` | A-2, 1.4.11 | `--focus-ring: 2px solid var(--accent)` |
| G-2 | **B** | `--success` `#2ec27e` à 2,30:1 sur blanc (pastille « Synchronisé ») | `theme/tokens.css:65` | A-2 | `light-dark(#007c3d, #78e9ab)` + `--success-bg` |
| G-3 | **B** | Marques de fichiers CSV 2,73:1 et image 2,90:1 sur la barre latérale claire | `theme/tokens.css:94`, `:96` | A-2 | `#1b8553`, `#c64600` ; Markdown `#1c71d8` |
| G-4 | **B** | Curseur distant jaune (`--person-4`) à 1,32:1 sur blanc ; orange 2,25, bleu ciel 2,31 | `editor/remoteCursors.ts:97` | A-2, C-PERSON | `--person-N-line` (65 % + noir en clair) ; idem marge `editor/changeGutter.ts:76`, `:197` |
| G-5 | M | Étiquette du nom d'un curseur en 11px | `editor/remoteCursors.ts:107` | T-7 | `--text-caption` (12px) |
| G-6 | M | `--fg-dim` et `--border` en contraste élevé à 75 % et 45 %, contre 90 % et 50 % dans libadwaita | `theme/tokens.css:141-143` | T-3 | valeurs de libadwaita |
| G-7 | M | Couleurs brutes dans 17 composants : toasts, fonds de dialogue, aperçu, présentation, avatars, recherche | `ui/Toast.svelte:49-63`, `preview/Preview.svelte:263`, `:292`, `:297`, `present/SlideStage.svelte:64-103`, `present/PublicWindow.svelte:94-127`, `present/PublicOverlay.svelte:36`, `:45`, `ui/search/SearchPanel.svelte:696`, `ui/search/SearchField.svelte:149`, `ui/people/PeopleDialog.svelte:338`, `:341`, `ui/sheet/Grid.svelte:956`, `ui/history/HistoryDrawer.svelte:364`, dialogues (G-12) | T-1 | jetons `--toast-bg`, `--page-bg`, `--stage-*`, `--shadow-*`, `--backdrop` ; règle stylelint |
| G-8 | M | 13 tailles de police : 11, 11.5, 12, 12.5, 13, 13.5, 14, 14.5, 15, 16, 17, 18 et 26px | `rg "font-size"` dans `*.svelte` | §2.3 | échelle de 6 tailles ; les glyphes de la barre (`BarButton.svelte:58-75`) restent des tailles d'icône |
| G-9 | m | Rayons bruts 2, 3, 4, 5, 8, 9, 11, 99 et 999px dans les composants | `rg "border-radius: [0-9]"` | §2.6 | 5 jetons, dont `--radius-pill` |
| G-10 | m | Opacité de désactivation à 0,45 ou 0,5 selon l'endroit | `theme/adwaita.css:38`, `:154`, `ui/toolbar/FormatBar.svelte:127` | §2.2 | `--disabled-opacity: 0.5` |
| G-11 | m | `--warning` en repli `#ff7800` dans le surlignage de recherche | `editor/search/highlight.ts:28` | T-1 | retirer le repli |

### 2.2 Composants

| # | Grav. | Écart | Où | Règle | Correction |
|---|---|---|---|---|---|
| G-12 | M | **Sept dialogues** recopient chacun leur CSS : rayon `--radius` (4) ou `--radius-window` (3), ombre et voile de 25 à 35 %, en-tête et bouton fermer propres | `ui/ShareDialog.svelte:99-112`, `ui/AboutDialog.svelte:88-101`, `ui/prefs/Preferences.svelte:103-117`, `ui/people/PeopleDialog.svelte:188-201`, `ui/history/ApplyDialog.svelte:46-63`, `ui/sheet/PasteDialog.svelte:36-46`, `ui/history/DiffView.svelte:236-255` | P2, §4.8 | `ui/Dialog.svelte` (tailles, en-tête, pied, retour du focus) |
| G-13 | M | Jusqu'à **sept bandeaux** empilés sous les barres | `App.svelte:234-261` | L-8 | `BannerHost` avec priorité ; un seul visible |
| G-14 | M | Bandeaux sans action : écriture impossible, projet scellé, fichier lourd | `ui/ProjectionBanner.svelte:8-12`, `ui/SealedBanner.svelte:8`, `App.svelte:256-258` | BN-1, règle 5 | « Réessayer », « Voir les personnes », « Ouvrir quand même en écriture » |
| G-15 | M | Un **toast qui pose une question** et ne part pas seul (« Intégrer · Garder ») | `ui/ExternalChangeToast.svelte:52-63` | TO-2 | bandeau `info` (§5.12) |
| G-16 | M | Deux sources de toasts sans file, à la même place fixe : elles se superposent | `ui/NoticeToast.svelte:6-8`, `ui/ExternalChangeToast.svelte:40-64` | L-9 | `ToastHost` unique, une file, pause au survol et au focus |
| G-17 | M | La minuterie des toasts ne se suspend pas au survol ni au focus | `ui/notice.svelte.ts:12`, `ui/ExternalChangeToast.svelte:35` | TO-1, 2.2.1 | pause et reprise |
| G-18 | **B** | Infobulles : affichées sans délai, **non fermables par Échap** | `theme/adwaita.css:222-224` | TT-1, 1.4.13 | composant `Tooltip` (délai 500ms, Échap, survolable) |
| G-19 | M | 48 attributs `title=` servent d'infobulle (en-tête, présentation, historique, recherche…) | `ui/shell/HeaderBar.svelte:77`, `:97`, `:107`, `:122` ; `present/PresenterView.svelte` (9) ; `ui/history/HistoryDrawer.svelte` (6) ; `ui/PeopleStack.svelte:18`, `:25`, `:31`… | TT-2, I-2 | composant `Tooltip` |
| G-20 | M | Barre de mise en forme : chaque bouton est un arrêt de tabulation, sans flèches | `ui/toolbar/FormatBar.svelte:61-111` | F-1, APG *Toolbar* | tabindex itinérant ; Alt+F10 |
| G-21 | M | Barre verrouillée grisée (0,45) et inerte, au lieu d'être masquée | `ui/toolbar/FormatBar.svelte:66`, `:126-128` | §4.3, §9 | masquer ; le bandeau d'accès explique |
| G-22 | M | Fichiers : des `<button>` dans une `<nav>`, sans `role="tree"` ni flèches | `ui/shell/Sidebar.svelte:31-59` | SB-1, APG *Tree View* | arbre APG ; F2, Suppr, Maj+F10 |
| G-23 | M | Onglets du tiroir sans ← → ni `aria-controls` | `ui/history/HistoryDrawer.svelte:82-88` | §4.5, APG *Tabs* | flèches, Origine et Fin, `tabpanel` |
| G-24 | m | Interrupteurs du tiroir en cases à cocher | `ui/history/HistoryDrawer.svelte:121-122` | §4.5 | `role="switch"` |
| G-25 | M | Échap dans tout menu rend le focus à l'**éditeur**, même pour le menu principal ou la barre d'état | `ui/popover/MenuButton.svelte:50-55` | M-1, APG *Menu Button* | focus au déclencheur, sauf pour les menus de la barre de mise en forme |
| G-26 | M | `<ul>` à l'intérieur d'un `<button>` (contenu non phrasé) ; pas de « +N » au-delà de 4 avatars | `ui/PeopleStack.svelte:14-36` | AV-1 | `<span>`, et pile `aria-hidden` ; débordement |
| G-27 | M | Couleur de personne = hachage modulo 8 : deux personnes présentes peuvent avoir la même | `ui/people.ts:25-31` | C-PERSON-2 | créneau libre, stable dans le projet |
| G-28 | M | Bouton destructeur local `.danger`, avec encre brute | `ui/people/PeopleDialog.svelte:166`, `:336-342` | §4.4 | `.destructive` global |
| G-29 | M | La barre d'état met la classe `.fb` (bouton) sur du texte non cliquable (page, non enregistré, position, langage) et une `opacity` de 0,8 sur le texte | `ui/shell/StatusBar.svelte:132`, `:137`, `:139`, `:150`, `:169-175` | ST-1, ST-2 | texte `.item` ; seuls les vrais boutons sont `.fb` |
| G-30 | m | L'état de synchronisation apparaît deux fois, avec pastille dans l'en-tête **et** dans la barre d'état | `ui/shell/HeaderBar.svelte:82-84`, `ui/shell/StatusBar.svelte:78-81` | §4.15 | une pastille (en-tête), un compte de personnes (barre d'état) |
| G-31 | m | Nom accessible des bascules variable (« Afficher / Masquer l'aperçu ») en plus d'`aria-pressed` | `ui/shell/HeaderBar.svelte:76`, `:96`, `:121` | H-4 | nom fixe |
| G-32 | m | Titre de la barre = « Inkwell » plutôt que le projet | `App.svelte:201`, `:110` | H-1 (Q2) | nom du projet |
| G-33 | m | Pas de F10 (menu principal), F6 (zones), Ctrl+, ni Ctrl+? | `App.svelte:127-148` | §8.2 | registre de raccourcis |
| G-34 | m | `FileIcon` lue en plus du nom dans chaque ligne (« Typst main.typ ») | `ui/icons/FileIcon.svelte:29-30`, `ui/shell/Sidebar.svelte:52` | I-5 | `aria-hidden` dans les lignes ; type en description |
| G-35 | m | Liseré de l'avatar en `--headerbar-bg` en dur : faux hors de la barre d'en-tête | `ui/PeopleStack.svelte:67` | §4.14 | `--avatar-ring: currentColor` du contexte, ou jeton de surface |

### 2.3 Mise en page

| # | Grav. | Écart | Où | Règle | Correction |
|---|---|---|---|---|---|
| G-36 | M | Aucune poignée de redimensionnement (A6) : barre latérale à 264px fixe, tiroir `min(380px, 42vw)` | `ui/shell/Sidebar.svelte:68`, `ui/history/HistoryDrawer.svelte:230` | L-3, L-4 | `Splitter` APG, tailles mémorisées par projet |
| G-37 | M | Pas d'états de fenêtre moyen et étroit : la barre latérale et le tiroir poussent toujours le contenu ; seul l'aperçu passe par-dessus sous 560px | `App.svelte:325-364` | §3.3 | superposition sous 900px ; une zone à la fois sous 560px |
| G-38 | M | La recherche dans le projet n'a pas de place : le panneau existe mais n'est monté nulle part | `ui/search/SearchPanel.svelte` (769 lignes, testé), `editor/search/keymap.ts:22-40` jamais branché | L-2 (Q5) | onglet « Rechercher » du tiroir ; brancher le keymap dans `editor/setup.ts` |

### 2.4 Parcours et contenu

| # | Grav. | Écart | Où | Règle | Correction |
|---|---|---|---|---|---|
| G-39 | **B** | Pas d'accueil : le nom est inventé en silence (« Personne 1a2b ») ; pas de choix de couleur ; pas de projet d'exemple | `workspace/workspace.svelte.ts:181` | §5.1, règle 8 | écran d'accueil |
| G-40 | **B** | Pas d'écran Projets ni de création : un seul projet, implicite, dans Documents | `internal/app/docs.go:86`, `App.svelte` | §5.2 | écran Projets, « Nouveau projet » |
| G-41 | M | Le dialogue Partager mêle inviter et rejoindre. Il n'expose ni le rôle, ni les personnes à proximité, ni les demandes, ni l'empreinte, ni la validité — que le Go fournit déjà | `ui/ShareDialog.svelte:37-95` ; `internal/app/invites.go` (`Invite(role)`, `Nearby`, `Requests`, `Answer`, `SetVisible`) | §5.3, §5.4 | séparer ; exposer |
| G-42 | M | Préférences limitées à l'enregistrement : pas de thème, de zoom par défaut, de nom et couleur, de « Détails techniques », de langue. `data-theme` n'est réglé nulle part | `ui/prefs/Preferences.svelte:48-99` | §5.10, T-2 | groupes de §5.10 |
| G-43 | M | « Déconnecter » (réversible) demande une confirmation | `ui/people/PeopleDialog.svelte:143`, `:158-172` | P3 (Q3) | toast « Annuler » |
| G-44 | M | Raison technique insérée dans la phrase du bandeau | `i18n/fr.json:44` (`projection.failed.reason`), `ui/ProjectionBanner.svelte:9-11` | E-2 | « Détails » derrière « Détails techniques » |
| G-45 | M | Accords genrés : « Seul », « vous a déconnecté{time} » | `i18n/fr.json:3`, `:544` | §6.3 | « Sur cet ordinateur » ; « {name} a suspendu votre accès » |
| G-46 | M | Espace ordinaire avant `? ! : ;` et dans « » dans 79 messages | `i18n/fr.json` | §6.3 | U+202F, U+00A0 ; test dans `i18n.test.ts` |
| G-47 | m | `<html lang="fr">` fixe, alors que l'interface passe en anglais selon le système | `frontend/index.html:2`, `i18n/index.ts:8` | A-14 | `document.documentElement.lang = lang` |
| G-48 | m | Jargon visible : « Révision {commit} » (À propos), « dossier .typstdeps », « UTF-8 », « LF » | `i18n/fr.json:85`, `:428`, `:167-177` | P5 | révision et chemin sous « Détails techniques » ; le menu d'encodage n'apparaît qu'en cas d'écart (déjà) : ses termes restent tolérés, à revoir en test utilisateur |
| G-49 | m | Ctrl+K = lien, alors que la règle 3 le réserve à la palette, qui n'existe pas | `ui/toolbar/spec.ts:96`, `editor/format/keymap.ts:16` | §4.24 (Q1) | décision de l'utilisatrice |

## 3. Corrections, dans l'ordre

L'ordre suit la valeur (accessibilité d'abord), puis le coût, puis les dépendances. Chaque
vague passe par `ux-walkthrough` et des captures clair et sombre avant fusion (`CLAUDE.md`).

### Vague 1 — Contraste et jetons (petit, sans risque, débloque les tests)

1. **G-1, G-2, G-3, G-6, G-10** : corriger `tokens.css` et étendre `tokens.test.ts` (parité avec
   `tokens.json`, paires de contraste).
2. **G-4, G-5** : ajouter `--person-N-line`, puis l'utiliser pour le curseur et la marge ;
   étiquette en 12px.
3. Ajouter les jetons nouveaux de `tokens.json` (`status: "new"`) : surfaces, ombres, `z`,
   mouvement, échelle de texte, espacements.
4. **G-7, G-11** : remplacer les couleurs brutes par ces jetons, puis activer la règle
   stylelint (§10).

### Vague 2 — Messages (cohérence visible, risque faible)

5. **G-16, G-17, G-15** : `ToastHost` (file, pause), puis migrer `NoticeToast` et
   `ExternalChangeToast` ; la question « Intégrer ? » devient un bandeau.
6. **G-13, G-14, G-44** : `BannerHost` (priorité L-8), une action par bandeau, la raison dans
   « Détails ».
7. **G-45, G-46, G-47** : textes neutres, espaces insécables, `lang` dynamique, et leur test.

### Vague 3 — Composants partagés (consolidation)

8. **G-12** : `Dialog.svelte`, puis migrer les sept dialogues, un par commit.
9. **G-18, G-19** : composant `Tooltip`, puis remplacer les 48 `title=`.
10. **G-28, G-29, G-30, G-31, G-35** : `.destructive`, rendu de la barre d'état, pastille unique,
    noms fixes.
11. **G-8, G-9** : échelle de texte et rayons, avec captures de référence avant et après.

### Vague 4 — Clavier (motifs APG)

12. **G-20** : barre de mise en forme à tabindex itinérant.
13. **G-25** : retour du focus des menus.
14. **G-23, G-24** : onglets et interrupteurs du tiroir.
15. **G-22, G-34** : arbre des fichiers APG (prépare B5).
16. **G-33** : registre de raccourcis, F10, F6, Ctrl+,, Ctrl+?, fenêtre des raccourcis.

### Vague 5 — Mise en page (A6)

17. **G-36** : `Splitter` accessible, tailles par projet.
18. **G-37** : états moyen, étroit et très étroit ; dialogues en feuille en bas.
19. **G-38** : monter la recherche dans le tiroir et brancher son keymap (après Q5).

### Vague 6 — Parcours manquants (après réponses Q1–Q4)

20. **G-39** : accueil (nom, couleur, projet d'exemple).
21. **G-40** : écran Projets et nouveau projet.
22. **G-41** : Partager et Rejoindre séparés ; rôle, proximité, demandes, empreinte.
23. **G-42** : préférences complètes, `data-theme`.
24. **G-43, G-26, G-27** : Personnes (toast « Annuler », pile « +N », couleurs sans collision).
25. **G-49, §4.24** : palette de commandes (après Q1).
26. **G-21, G-32, G-48** : finitions.
