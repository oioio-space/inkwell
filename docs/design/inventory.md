# Inventaire de l'interface et écarts avec la spécification (N17, révision 7)

- **Relevé du 2026-11-01** sur `main` (`3841ff2`), dans `frontend/src` ; les chemins sont
  relatifs à ce dossier, sauf mention contraire.
- La révision 2 ajoute les constats des revues n°1, l'audit de couverture
  ([`coverage.md`](coverage.md)), N20 et N21.
- La révision 3 ajoute les revues n°2 :
  - **G-0 est ouvert** ;
  - les chemins sont corrigés ;
  - s'ajoutent G-71 à G-83 (N22 à N25, N27, N28).
- Les identifiants G-1 à G-49 de la révision 1 sont conservés.
- **Gravité :**
  - **B** : bloquant (échec WCAG 2.2 AA ou parcours principal cassé) ;
  - **M** : majeur ;
  - **m** : mineur.
- Chaque écart cite la **norme** et la **preuve** (fichier:ligne, ou valeur mesurée).

## 1. Ce qui existe

| Zone | Fichier | Conformité |
|---|---|---|
| Jetons (3 niveaux, `light-dark()`, HC, mouvement réduit) | `theme/tokens.css:10-151` | bonne base ; valeurs sous le seuil (§2.1) ; test de parité absent |
| Classes Adwaita | `theme/adwaita.css` | pas de `.destructive` ; opacités mélangées |
| Fenêtre | `App.svelte:195-315` | pas d'onglets de fichiers, pas de poignées, états étroits incomplets |
| Barre d'en-tête | `ui/shell/HeaderBar.svelte` | titre = application ; `title=` |
| Barre de mise en forme | `ui/toolbar/*` | conforme à T11 ; clavier non APG |
| Menus | `ui/popover/MenuButton.svelte` | ↑ et Échap non APG |
| Barre d'état | `ui/shell/StatusBar.svelte` | complète ; boutons et texte confondus |
| Fichiers | `ui/shell/Sidebar.svelte` | **le clic n'ouvre rien** (G-0, ouvert) ; pas d'arbre ARIA ; pas de Sommaire, Tâches ni Liens |
| Aperçu | `preview/Preview.svelte` | bon ; couleurs brutes |
| Tableur | `ui/sheet/*` | grille APG ; toast et bandeaux propres |
| Historique | `ui/history/*` | `listbox` sans focus d'option ; onglet Commentaires vide |
| Personnes | `ui/people/PeopleDialog.svelte` | confirmation en ligne sans gestion du focus |
| Partage | `ui/ShareDialog.svelte` | incomplet |
| Préférences | `ui/prefs/Preferences.svelte` | enregistrement seulement |
| Messages | `ui/Banner.svelte`, `ui/Toast.svelte` et 9 usages | pas de file ; régions montées avec leur texte |
| Présentation | `present/*` | couleurs brutes |
| Icônes | `ui/icons/*` | conforme ; pas de type « code » |
| i18n | `i18n/*` | typographie, accords, `lang`, « fusion » |
| Recherche | `ui/search/*`, `editor/search/*` | codée, **non montée** |
| Coloration | `editor/highlight.ts:47-58` | Typst et Markdown seulement, sans blocs de code |
| Absents | — | accueil, Projets, onglets, palette, poignées, `StatusPage`, `Dialog`, `ToastHost`, `BannerHost`, infobulle, aide, raccourcis, extensions |

## 2. Écarts

### 2.0 Parcours principal cassé

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| **G-0** | **B** | Cliquer un fichier dans Fichiers ne l'ouvre pas : `onOpen` ignore le chemin transmis. | Nielsen 1 et 3 ; spec §4.16 | `App.svelte:197` (`onOpen={() => session.focus()}`) ; `ui/shell/Sidebar.svelte:49` transmet `node.path` | **Ouvert.** Le correctif est un lot séparé, **à faire** ; aucun commit ne le corrige (`App.svelte:197` est inchangé). Le comportement attendu, qui sert de critère d'acceptation, est en spec §4.16. Le test e2e `G-0` (`e2e/files.spec.ts`) est à écrire. Le modèle d'onglets est G-55. |

### 2.1 Jetons et contraste

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-1 | **B** | Anneau de focus translucide | 1.4.11 | 2,62 (vue claire) · 2,34 (barre latérale) · 2,63 (barre sombre) ; `theme/tokens.css:72` | `--focus-ring: 2px solid var(--accent)` |
| G-1b | M | Pas de test de parité `tokens.json` ↔ `tokens.css` ; seules les 9 couleurs syntaxiques sont vérifiées | spec T-5 | `theme/tokens.test.ts:24-58` | étendre le test (§10) |
| G-2 | **B** | `--success` `#2ec27e` | 1.4.11 | 2,30 sur blanc ; `tokens.css:65` | `#007c3d` / `#78e9ab` |
| G-3 | **B** | Marques CSV et image | 1.4.11 | 2,73 et 2,90 sur la barre latérale ; `tokens.css:94`, `:96` | `#1b8553`, `#c64600` |
| G-4 | **B** | Curseur distant jaune | 1.4.11 | 1,32 sur blanc ; `editor/remoteCursors.ts:97`, `editor/changeGutter.ts:76`, `:197` | `--person-N-line` (3,14 au minimum) |
| G-5 | M | Étiquette de curseur en 11px | T-7 | `editor/remoteCursors.ts:107` | 12px |
| G-6 | M | Valeurs HC à 75 % et 45 % | T-3, libadwaita | `tokens.css:141-143` | 90 % et 50 % ; `--disabled-opacity` à 0,4 |
| G-7 | M | Couleurs brutes dans 17 composants | T-1 | `ui/Toast.svelte:49-63`, `preview/Preview.svelte:263`, `:292`, `:297`, `present/SlideStage.svelte:64-103`, `present/PublicWindow.svelte:94-127`, `present/PublicOverlay.svelte:36`, `:45`, `ui/search/SearchPanel.svelte:696`, `ui/search/SearchField.svelte:149`, `ui/people/PeopleDialog.svelte:338`, `:341`, `ui/sheet/Grid.svelte:956`, `ui/history/HistoryDrawer.svelte:364`, dialogues (G-12) | jetons, stylelint |
| G-8 | M | 13 tailles de police | §2.3 | relevé de `font-size` | échelle de 6 tailles |
| G-9 | m | Rayons bruts (2, 3, 4, 5, 8, 9, 11, 99, 999px) | §2.6 | relevé de `border-radius` | 5 jetons |
| G-10 | m | Opacité de désactivation à 0,45 ou 0,5 | §2.2 | `theme/adwaita.css:38`, `:154`, `ui/toolbar/FormatBar.svelte:127` | `--disabled-opacity` |
| G-11 | m | Repli `#ff7800` | T-1 | `editor/search/highlight.ts:28` | le retirer |
| G-50 | **B** | `--fg-dim` sur une ligne sélectionnée | 1.4.3 | 4,48 sur vue, 4,26 sur barre latérale ; `ui/history/HistoryRow.svelte:158` (`.s` en `--fg-dim`) | C-SEL : `--fg` sur la sélection |
| G-52 | **B** | Numéros de ligne à 45 % de `--fg` | 1.4.3 | 2,51 sur blanc ; `editor/setup.ts:38` | C-GUT : `--fg-dim` (4,74) |
| G-53 | **B** | Anneau normal sur les fonds sombres du thème clair | 1.4.11 | 2,39 (toast), 2,07 (infobulle), 3,46 (scène) | `--focus-ring-on-dark` |

### 2.2 Composants

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-12 | M | Sept dialogues recopiés (rayons, voiles et ombres différents) | P2 | `ui/ShareDialog.svelte:99-112`, `ui/AboutDialog.svelte:88-101`, `ui/prefs/Preferences.svelte:103-117`, `ui/people/PeopleDialog.svelte:188-201`, `ui/history/ApplyDialog.svelte:46-63`, `ui/sheet/PasteDialog.svelte:36-46`, `ui/history/DiffView.svelte:236-255` | `Dialog.svelte` |
| G-13 | M | Jusqu'à 7 bandeaux globaux empilés, plus des bandeaux locaux | L-8, `AdwBanner` | `App.svelte:234-261` ; `ui/sheet/Sheet.svelte:469`, `:541`, `:555`, `:567` ; `ui/history/HistoryBars.svelte:143` | `BannerHost` (L-8, L-8b, L-8c) |
| G-14 | M | Bandeaux sans action | BN-1, Nielsen 9 | `ui/ProjectionBanner.svelte:8-12`, `ui/SealedBanner.svelte:8`, `App.svelte:256-258` | ajouter les actions |
| G-15 | M | Un toast qui pose une question | TO-2, HIG *Toasts* | `ui/ExternalChangeToast.svelte:52-63` | bandeau-question |
| G-16 | M | **Trois** sources de toasts, sans file | L-9 | `ui/NoticeToast.svelte:6-8`, `ui/ExternalChangeToast.svelte:40-64`, `ui/sheet/Sheet.svelte:605` (minuterie à `:169`) | `ToastHost` |
| G-17 | M | La minuterie ne se suspend pas, et aucune annulation durable n'est annoncée | 2.2.1 | `ui/notice.svelte.ts:12`, `ui/ExternalChangeToast.svelte:35`, `ui/sheet/Sheet.svelte:169` | TO-1 |
| G-18 | **B** | Infobulles sans délai, non fermables par Échap | 1.4.13 | `theme/adwaita.css:222-224` | `Tooltip` (vague 2) |
| G-19 | M | 48 `title=` | TT-2 | `HeaderBar.svelte:77`, `:97`, `:107`, `:122`, `present/PresenterView.svelte` (9), `HistoryDrawer.svelte` (6)… | `Tooltip` |
| G-20 | M | Barre de mise en forme sans tabindex itinérant | APG *Toolbar* | `ui/toolbar/FormatBar.svelte:61-111` | F-1 |
| G-21 | M | Barre verrouillée grisée et inerte | Nielsen 1 | `FormatBar.svelte:66`, `:126-128` | F-2 |
| G-22 | M | Fichiers sans arbre ARIA | APG *Tree View*, 4.1.2 | `Sidebar.svelte:31-59` | §4.16 |
| G-23 | M | Onglets du tiroir sans flèches ni `aria-controls` | APG *Tabs* | `HistoryDrawer.svelte:82-88` | §4.5 |
| G-24 | m | Interrupteurs en cases à cocher | APG *Switch* | `HistoryDrawer.svelte:121-122` | `role="switch"` |
| G-25 | M | Échap rend le focus à l'éditeur | APG *Menu Button*, 2.4.3 | `MenuButton.svelte:50-55` | retour au déclencheur, sans exception |
| G-26 | M | `<ul>` dans un `<button>` ; pas de « +N » | HTML, 4.1.2 | `ui/PeopleStack.svelte:14-36` | AV-1 |
| G-27 | M | Couleur par hachage modulo 8 | C-PERSON-2, 1.4.1 | `ui/people.ts:25-31` | créneau du projet (Q4) |
| G-28 | M | `.danger` local | P2 | `PeopleDialog.svelte:166`, `:336-342` | `.destructive` |
| G-29 | M | `.fb` sur du texte ; `opacity` sur le texte ; zones actives de 28px | ST-1, ST-2, §2.5 | `ui/shell/StatusBar.svelte:132`, `:137`, `:139`, `:150`, `:169-175` | zones actives de 34px |
| G-30 | m | Pastille d'état en double | Nielsen 8 | `HeaderBar.svelte:82-84`, `StatusBar.svelte:78-81` | §4.15 |
| G-31 | m | Nom accessible variable en plus d'`aria-pressed` | APG *Button*, 2.5.3 | `HeaderBar.svelte:76`, `:96`, `:121` | H-4 |
| G-32 | m | Titre de barre = « Inkwell » | HIG *Header bars*, 2.4.2 | `App.svelte:110`, `:201` | Q2 |
| G-33 | m | Pas de F10, F6, Ctrl+, ni Ctrl+? | HIG *Keyboard*, 2.4.1 | `App.svelte:127-148` | §8.2 |
| G-34 | m | Type de fichier lu en double | 1.1.1 | `ui/icons/FileIcon.svelte:29-30`, `ui/shell/Sidebar.svelte:52` | I-5 |
| G-35 | m | Liseré d'avatar en `--headerbar-bg` fixe | T-1 | `PeopleStack.svelte:67` | surface parente |
| G-51 | M | Historique en `listbox` : les flèches changent la sélection sans déplacer le focus ni renseigner `aria-activedescendant` | APG *Listbox*, 4.1.2 | `HistoryDrawer.svelte:143-149` | §4.22 |
| G-54 | M | Groupes radio sans flèches ; barre du tableur sans tabindex itinérant ; ↑ ouvre un menu sur le premier élément | APG *Radio Group*, *Toolbar*, *Menu Button* | `Preferences.svelte:52`, `DiffView.svelte:181`, `HistoryBars.svelte:162`, `Sheet.svelte:479`, `MenuButton.svelte:115` | §4.2, §4.3, §4.5 |
| G-59 | M | Les dialogues rendent le focus à l'éditeur | APG *Dialog*, 2.4.3 | `Preferences.svelte:38`, `AboutDialog.svelte:62` | retour au déclencheur |
| G-60 | **B** | Région live montée **en même temps** que son texte : annonce non garantie | 4.1.3, ARIA22 | `NoticeToast.svelte:6-8`, `Toast.svelte:24`, `Banner.svelte:21` | régions persistantes (§4.11) ; à vérifier avec Orca |
| G-64 | M | Confirmation en ligne : ni focus, ni Échap | APG *Alert Dialog* | `PeopleDialog.svelte:158-172` | §4.9 |

### 2.3 Architecture de l'information et mise en page

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-36 | M | Aucune poignée de redimensionnement (panneaux, tiroir, vue présentateur) | T11 A6 (RSZ-04 à 08), 2.5.7 | `Sidebar.svelte:68`, `HistoryDrawer.svelte:230`, `present/PresenterView.svelte` | §3.3 |
| G-37 | M | Sous 900px, l'aperçu reste côte à côte (seuil à 560px) | T11 A6, 1.4.10 | `App.svelte:357-364` | §3.4 |
| G-38 | M | Recherche dans le projet non montée | T11 B6 | `ui/search/SearchPanel.svelte` (aucun import hors test) ; `editor/search/keymap.ts:22-40` jamais branché | onglet Rechercher (Q5) |
| G-55 | **B** | Pas de modèle d'onglets de fichiers ni de sélecteur à gauche (Sommaire, Tâches, Liens absents) | HIG *Tabs*, `AdwViewSwitcher` ; T11 B7, B10 | `workspace/workspace.svelte.ts:120` (un seul `current`) ; `Sidebar.svelte:29-61` | §4.25, §4.16 |
| G-62 | M | Onglet Commentaires affiché, sans fonction | Nielsen 8, L-2b | `HistoryDrawer.svelte:46`, `:206-207` | le masquer (Q8) |

### 2.4 Parcours, contenu, aide

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-39 | **B** | Pas d'accueil : nom inventé | règle 8 | `workspace/workspace.svelte.ts:181` | §5.1 |
| G-40 | **B** | Pas d'écran Projets | tâche « créer » | `internal/app/docs.go:86` | §5.2 |
| G-41 | M | Le partage n'expose ni le rôle, ni la proximité, ni les demandes, ni l'empreinte | §5.3–5.4, 3.3.8 | `ShareDialog.svelte:37-95` ; `internal/app/invites.go` | §5.3, §5.4 |
| G-42 | M | Préférences réduites ; `data-theme` réglé nulle part | §5.13, T-2 | `Preferences.svelte:48-99` ; aucun `data-theme` dans `src` | §5.13 |
| G-43 | M | « Déconnecter » confirmé | P3 | `PeopleDialog.svelte:143`, `:158-172` | Q3 |
| G-44 | M | Raison technique dans la phrase | E-2, 3.3.3 | `fr.json:44` | « Détails » |
| G-45 | M | Accords genrés | §6.3 | `fr.json:3`, `:544` | formulations neutres |
| G-46 | M | 79 espaces ordinaires avant `? ! : ;` et dans « » | §6.3 | `fr.json` | U+202F, U+00A0, et un test |
| G-47 | m | `<html lang="fr">` fixe | 3.1.1 | `frontend/index.html:2`, `i18n/index.ts:8` | `lang` dynamique |
| G-48 | m | Jargon (« Révision {commit} », `.typstdeps`, UTF-8) | P5 | `fr.json:85`, `:428`, `:167-177` | « Détails techniques » |
| G-49 | m | Pas de palette ; Ctrl+K = lien (**juste**, C14) | règle 3 corrigée | `ui/toolbar/spec.ts:96` | palette sur Ctrl+Maj+P (Cmd+Maj+P sur macOS) |
| G-56 | M | Chemin du fichier dans le toast d'export ; pas d'action « Ouvrir » | E-2, Nielsen 3 | `fr.json:450`, `preview/actions.ts:82` | §5.12 |
| G-57 | M | Écrans vides en texte seul, sans action | HIG *Placeholders*, Nielsen 9 | `HistoryDrawer.svelte:207`, `:213`, `PeopleDialog.svelte:101`, `Preview.svelte:193-201` | `StatusPage` |
| G-58 | M | Pas d'aide ni de fenêtre des raccourcis | Nielsen 10, 3.2.6 | aucun composant | §4.26 |
| G-61 | M | « fusion », « Fusionner », « fusionnée » | §6.4, glossaire (« merge ») | `i18n/fr.json:549`, `:618`, `:621`, `:622`, `:656`, `:663` | « Intégrer la variante » |
| G-63 | m | Pas de registre unique des commandes ; doublons de raccourcis non testés | K-6, Nielsen 5 | raccourcis répartis entre `App.svelte:127-148`, `ui/toolbar/spec.ts`, `editor/format/keymap.ts`, `editor/search/keymap.ts` | §8.2 |

### 2.5 Types de fichiers et code (N20)

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-65 | M | Pas de type « code » dans `FileKind` | FT-1 | `store/files.ts:20-30` (le format est versionné : `FILES_VERSION`, ligne 12) | ajouter `code`, à décider avec N18 et le protocole |
| G-66 | M | Pas de coloration du code : ni fichiers, ni blocs Markdown (`codeLanguages` absent) | FT-2, FT-4 | `editor/highlight.ts:47-58` ; `@codemirror/language-data` absent de `package.json` | `language-data` paresseux, avec une liste blanche |
| G-67 | M | Pas de mode lecture pour le code ou les fichiers inconnus | FT-5 | `editor/format/lang.ts:5-15` (tout ce qui n'est pas Typst ou Markdown devient « plain » et s'édite) | barre « lecture · Modifier » |
| G-68 | M | Pas de fiche pour les fichiers binaires ; ce qui s'affiche à leur ouverture reste à vérifier une fois G-0 corrigé | FT-6 | `workspace/workspace.svelte.ts:247` (`open(url)` pour des documents texte seulement) | fiche (§4.28) |
| G-69 | m | Le CSV en texte n'a pas de colonnes repérées | FT-3 | `App.svelte:248-253` (vue texte = éditeur nu) | FT-3 |

### 2.6 Extensions (N21)

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-70 | — | Rien n'existe encore : c'est attendu, l'exécution relève de N18 | §4.29 | — | page Préférences › Extensions, consentement, cadre attribué |

### 2.7 Thèmes, extensions de projet, intégrité, diagrammes, développement (N22 à N25)

Ces fonctions n'existent pas encore. Chaque ligne nomme ce que le code devra montrer.

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-71 | M | Pas de contrat de thème ; `data-theme` réglé nulle part ; aucune validation au chargement | N22, TH-1 à TH-4 (architecture §15.8), 1.4.3, 1.4.11 | `theme/tokens.css` n'a ni modes HC complets ni chargement de thème ; aucun `data-theme` dans `src` | spec §2.10, `tokens.json` (`contract`, `themeContract`), `theme/contract.test.ts` |
| G-72 | M | Pas d'interface pour les extensions de projet (provenance, mode restreint, « Désactiver pour moi », « Installer pour tous mes projets ») | N23, PP-3 à PP-5 | aucun composant | spec §4.32 |
| G-73 | m | Pas de bouton Extensions ni d'épinglage | N24, architecture §15.13 | aucun composant | spec §4.30 |
| G-74 | M | Pas de mode sans extensions, ni de zones protégées, ni d'affichage des conflits de raccourcis | N24, ND-1 à ND-6 | aucun composant | spec §4.33 |
| G-75 | m | Pas de rendu de diagramme | N24, architecture §15.14 | aucun composant | spec §4.34 |
| G-76 | m | Pas de mode développeur ni d'Inspecteur | N25, architecture §15.15 | aucun composant | spec §4.35 |
| G-77 | M | Aucun registre de contributions : menus, barres et panneaux sont codés en dur | architecture P-EXT-1, spec §4.31 | `ui/shell/HeaderBar.svelte:128-208`, `ui/toolbar/spec.ts` | registre déclaratif |
| G-78 | m | Pas de repli quand le déclencheur du focus a disparu | APG *Dialog*, 2.4.3 | `ui/popover/MenuButton.svelte:50-55` (un seul chemin : l'éditeur) | spec §4, « Retour du focus » |
| G-79 | m | La langue d'un passage n'est pas portée (option « English », langue déclarée d'un document) | 3.1.2 | `ui/prefs/Preferences.svelte` (aucun `lang`) ; `frontend/index.html:2` | spec §5.13, coverage 3.1.2 |

### 2.8 Modèles et problèmes de compilation (N27, N28)

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-80 | M | Aucun soulignement ni marqueur de marge pour les problèmes dans l'éditeur | N28, Nielsen 9, 3.3.1 | `@codemirror/lint` absent (`frontend/package.json`) ; aucun `setDiagnostics` dans `src` | spec §4.36 |
| G-81 | M | Le diagnostic a la ligne et la **colonne** de début (message coupé à **4 000** caractères), mais ni fin de plage, ni conseil, ni correction, ni trace ; les problèmes CSV n'ont que la ligne | N28, 3.3.3 | `preview/protocol.ts:8-19`, `:134-142` ; `editor/csv/problems.ts:17` (`findProblems`) | modèle de diagnostic unique (§4.36) ; à porter au protocole de l'aperçu (N18) |
| G-82 | M | Messages du compilateur bruts, en anglais ; pas de phrase en clair ni de conseil ; le compte de la barre d'état n'amène pas à l'erreur (il ouvre une liste) | N28, P5, Nielsen 9 | `ui/shell/StatusBar.svelte:120` (`d.message.split("\n")[0]`), `:84-101` | table de traduction ; compteur unique dont le clic va au suivant ; F8 ; corrections sur **Alt+Entrée** ; liste dans Relire (C13) |
| G-83 | M | Aucun modèle de projet : ni galerie, ni enregistrement, ni gestion. « Nouveau projet » n'existe pas (G-40). | N27 | aucun composant | spec §5.2, §5.19 |
| G-84 | M | tinymist est embarqué, mais son LSP n'est pas relié à l'éditeur : ni complétion, ni aide de signature, ni survol, ni définition | N29 | `@codemirror/autocomplete` absent de `frontend/package.json` ; aucun appel LSP dans `frontend/src/editor` | spec §4.37 ; branchement (architecture N18) |

### 2.9 Révision 4 : contrastes nouveaux, écriture, IA, ressources (N30 à N32)

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-85 | **B** | Texte secondaire d'une ligne de préférences (`.sub`, `--fg-dim`) sur une carte dans un dialogue : **4,13:1** en sombre | 1.4.3 | `ui/prefs/Preferences.svelte:86` (`.sub`, dans `.card.boxed`, dans `<dialog>`) ; paire de `tokens.json` | `--fg-dim-on-card` (5,07:1) |
| G-85b | M | Texte posé sur le papier de l'aperçu : la couleur brute `#55555a` est juste, mais n'est pas un jeton ; `--fg` y donnerait 1,00:1 en sombre | T-1, 1.4.3 | `preview/Preview.svelte:292` (déjà dans G-7) | `--page-fg` |
| G-86 | M | Pas d'IA : ni onglet, ni proposition, ni fournisseur, ni politique, ni dialogue d'envoi | N31 | aucun code ; architecture §15.18 | spec §4.39 |
| G-87 | M | Pas de vue des ressources des extensions | N32, ND-5 | aucun code | spec §4.40 |
| G-88 | M | Pas d'orthographe, de Relecture, de mode concentration, d'objectifs, de typographie française à la frappe, de dialogues d'insertion de figure ni de citation | N30 | aucun correcteur dans `frontend/package.json` ; le compte de mots existe (`ui/popover/StatsPopover.svelte`) | spec §4.38 |

### 2.10 Révision 5 : cohérence (N36), versions, variantes, protection, synchronisation, plates-formes

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-0a / G-0b | **B** | G-0 se corrige en deux temps (architecture `AD-77`). **G-0a** : le clic ouvre vraiment (éditeur pour `main.typ`, lecture seule ailleurs, avec un bandeau). **G-0b** : édition en onglet. **Toujours ouvert.** | Nielsen 1 et 3 | `App.svelte:197` | spec §4.16 ; `e2e/files.spec.ts` |
| G-89 | M | L'historique est un tiroir à trois onglets (Historique, Commentaires, Activité) : pas de vue du passé en lecture seule, pas de « Revenir au présent », pas de curseur temporel, pas de restauration de tout le projet avec aperçu | N39, `coherence.md` §6.7 | `ui/history/HistoryDrawer.svelte:46`, `:82-88` | Relire › Versions (spec §4.22) |
| G-90 | M | Les variantes existent (`doc.meta.variants`), mais l'interface dit « fusion » et n'a ni « Proposer d'intégrer », ni relecture d'intégration par morceau, ni choix côte à côte | N41, glossaire | `ui/history/HistoryDrawer.svelte:61-64` ; `i18n/fr.json:549`, `:618`, `:621`, `:622`, `:656`, `:663` (G-61) | spec §4.22, §4.41 |
| G-91 | M | Pas de fichiers non synchronisés : ni geste, ni marque, ni aperçu, ni règles | N34 | aucun code ; architecture §6.9 | spec §4.42 |
| G-92 | M | Pas de protection du projet (au choix, non cochée par défaut : décision de l'utilisatrice), de phrase ni de kit de secours, de sauvegarde ni de restauration à côté | N37, M01 | aucun code ; architecture §6.7, §6.10 | spec §4.43, §4.44 |
| G-93 | M | Barre d'état surchargée (personnes, problèmes, page, non enregistré, position, mots, zoom, encodage, langage) ; liste des problèmes en popover | `coherence.md` §6.4, §6.5 | `ui/shell/StatusBar.svelte:78-150` | quatre éléments, le reste sous ⋯ ; liste dans Relire (spec §4.17, §4.36) |
| G-94 | M | Un texte promet plus que l'état connu | C19, H-HON | `i18n/fr.json:7` (« Hors ligne — rien n'est perdu ») | spec §4.15 |
| G-95 | M | Pas de barre de menus macOS générée du registre, pas d'affichage « ⌘ » des raccourcis, pas de fiche « aperçu indisponible sur ce système » | N35, `coherence.md` §6.18 | `ui/toolbar/spec.ts` (`shortcutLabel`), `App.svelte:127-148` | spec §8.4 |
| G-96 | m | Panneau de gauche : la spec rév. 4 prévoyait quatre onglets ; il en faut deux (Fichiers, Sommaire), avec Liens dans les détails et Tâches en filtre | `coherence.md` §6.3 | spec rév. 4 §4.16 (corrigé) ; code : un seul panneau Fichiers (`ui/shell/Sidebar.svelte`) | spec §4.16 |
| G-97 | m | Aucune entrée bureautique dans le code ; le collage riche n'est pas converti | N40, M05 | aucun gestionnaire de collage HTML dans `frontend/src/editor` | spec §4.45 |

### 2.11 Révision 6 : raccourcis par système, contraste des états, résidus

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-98 | M | `Mod-Alt-0` (paragraphe) est une combinaison Ctrl+Alt : sous Windows, elle tape un caractère (AltGr) | Microsoft UX Guide « Don't use Ctrl+Alt » [S31] | `editor/format/keymap.ts:22` | le raccourci de titre, une seconde fois, rend paragraphe (§8.2) |
| G-99 | M | Sous macOS, `Mod-m` (formule) donnerait ⌘M (réduire la fenêtre) et `Mod-e` (code) ⌘E (utiliser la sélection pour chercher) ; le registre traduit `Mod` en Cmd sans table par système | Apple, raccourcis du Mac [S30] ; K-7 | `editor/format/keymap.ts:14`, `:17` ; `ui/toolbar/spec.ts:106`, `:126` | liaisons macOS explicites ⌃⌘E et ⇧⌘M (§8.2) |
| G-100 | M | Texte secondaire en `--fg-dim` sur survol : raccourcis des menus à 4,22:1 sur un popover sombre | 1.4.3 ; Codex R5-B1 | `theme/adwaita.css` (`.mi .k` en `--fg-dim`, `.mi:hover` en `--hover-bg`) | `--fg-dim-on-state` |
| G-101 | m | L'icône de l'historique est une branche | N39 (zéro imagerie de Git) | `ui/shell/HeaderBar.svelte:126` (`<Icon name="branch" />`) | `clock` (§8.3) |

### 2.12 Addendum rév. 7.1 : polices (N43)

| # | Grav. | Écart | Norme | Preuve | Correction |
|---|---|---|---|---|---|
| G-102 | M | Pas de gestionnaire de polices ni de dossier `polices/` du projet ; pas d'installation « pour tous mes projets » ; une police venue de cet ordinateur n'est pas signalée, et une police absente n'a que le message brut du compilateur, sans correction | N43, N28, Nielsen 9 | aucun composant ; `preview/protocol.ts` (message seul) ; `ui/shell/StatusBar.svelte:120` | spec §4.46 (signalement **local**, sans donnée des autres appareils) |

## 3. Corrections, dans l'ordre

L'ordre suit la valeur (parcours cassé et accessibilité d'abord), puis le coût, puis les
dépendances. Chaque vague passe par `ux-walkthrough`, par des captures en clair et en sombre, et
par une passe Orca.

- **Vague 0, lot séparé, à faire.** G-0 : ouvrir le fichier cliqué, selon spec §4.16 (non corrigé au 2026-11-01).
- **Vague 1, contraste et jetons :**
  - G-1, G-53 (anneaux), G-2, G-3, G-4, G-5, G-6, G-10, G-50, G-52 ;
  - G-1b (test de parité et paires) ;
  - les jetons nouveaux ;
  - G-7, G-11 et stylelint ;
  - le test des thèmes TS.
- **Vague 2, messages et infobulles :**
  - G-60 (régions persistantes) ;
  - G-16, G-17, G-15 (`ToastHost`, annulation durable) ;
  - G-13, G-14, G-44 (`BannerHost` : états critiques, puis questions, puis états, puis information, L-8) ;
  - **G-18, G-19** (`Tooltip`) : remonté depuis la vague 3 à la demande de Fable, car c'est un
    échec AA ;
  - G-45, G-46, G-47, G-61 (textes).
- **Vague 3, composants partagés :**
  - G-12, G-59, G-64 (`Dialog`, `AlertDialog`) ;
  - G-28, G-29, G-30, G-31, G-35 ;
  - G-57 (`StatusPage`) ;
  - G-8, G-9.
- **Vague 4, clavier :** G-20, G-54, G-25, G-23, G-24, G-51, G-22, G-34, G-33, G-63 (registre).
- **Vague 5, architecture de l'information et mise en page :**
  - **G-55** (onglets de fichiers ; panneau de gauche à **deux** onglets, Fichiers et Sommaire, avec Tâches en filtre et Liens dans les détails, G-96) ;
  - G-62 ;
  - G-36, G-37 ;
  - G-38 (Rechercher, après Q5).
- **Vague 6, types de fichiers :** G-65 (avec N18), G-66, G-67, G-68, G-69.
- **Vague 7, parcours** (après Q1 à Q4) :
  - G-39, G-40, G-41, G-42, G-43, G-26, G-27 ;
  - G-49 (palette) ;
  - G-56 ;
  - G-58 (aide) ;
  - G-21, G-32, G-48.
- **Vague 8, extensions** (après la conception de l'exécution, N18) :
  - G-77 (registre de contributions) ;
  - G-70, G-73, G-74 (gestion, bouton, sûreté) ;
  - G-72 (extensions de projet) ;
  - G-75 (diagrammes) ;
  - G-76 (outils).
- **Vague 9, thèmes :** G-71 (contrat, validation, Apparence), après G-1b.
- **Vague 10, problèmes et modèles :**
  - G-80, G-81, G-82 (problèmes, avec N18 pour le protocole) ;
  - G-83 (modèles, après G-40) ;
  - G-84 (aide à l'écriture, après le branchement du LSP).
- **Vague 11, écriture, IA, ressources :** G-85 et G-85b avec la vague 1 (contraste) ; G-88
  (écriture) ; G-87 (ressources, avec la vague 8) ; G-86 (IA, après PD-1 et les ADR 0040 et 0041).
- **Vague 14, polices** : G-102, avec G-80 et G-82 (problèmes) et G-84 (complétion).
- **Vague 13, rév. 6** : G-100 avec la vague 1 (contraste) ; G-98, G-99 avec le registre (G-63) ;
  G-101 avec G-89.
- **Vague 12, cohérence** (MVP de `coherence.md` §5) :
  - G-0a, puis G-0b ;
  - G-93, G-94 ;
  - G-89 et G-90 (« Revenir en arrière ») ;
  - G-91 (couche personnelle) ;
  - G-92 (sauvegarde locale et restauration à côté au MVP ; protection en v1) ;
  - G-95 (v1, six plates-formes) ;
  - G-96, G-97.
- **Transverses**, à mener avec la vague 4 : G-78 (replis du focus), G-79 (langue des
  passages).
