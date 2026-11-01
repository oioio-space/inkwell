# Matrice de couverture du système de design (N17, révision 3)

C'est la grille de contrôle des reviewers. Chaque ligne dit :
- où [`spec.md`](spec.md) traite le point ;
- l'état du code ;
- l'écart (`G-n` dans [`inventory.md`](inventory.md), avec sa norme et sa preuve) ;
- la **preuve existante**, s'il y en a une.

**Légende :**

| Sigle | Sens | Exigence |
|---|---|---|
| **S** | conforme dans le code | **une preuve existante** est citée (fichier:ligne ou test) |
| **S→G** | spécifié ; le code s'en écarte | l'écart `G-n` est cité |
| **S→N** | spécifié ; rien n'existe, ou rien ne le prouve encore | le test à écrire est cité dans spec §10 |
| **Q** | dépend d'une décision de l'utilisatrice | spec §11 |
| **H** | hors périmètre, justifié | — |
| **NA** | non applicable, justifié | — |

Révision 3 (revues n°2) :
- toute ligne S sans preuve existante est passée en S→N ;
- trois lignes S fausses sont corrigées : 3.2.2, HIG *Popovers*, *Performance perçue* ;
- G-0 est **ouvert** ;
- s'ajoutent N22 à N25.

## 1. Les 10 heuristiques de Nielsen [S12]

| # | Heuristique | Où dans la spec | État | Écarts |
|---|---|---|---|---|
| 1 | Visibilité de l'état du système | §4.15, §4.17, §4.18, §4.21, §5.17, §4.25, §4.29 (états du gestionnaire), §4.33 et §4.35 (pastilles de mode) | S→G | G-0, G-2, G-30, G-38, G-56 |
| 2 | Correspondance avec le monde réel | P5, §6.4, §4.29 (phrases de l'architecture §15.5) | S→G | G-45, G-48, G-61 |
| 3 | Contrôle et liberté | P3, TO-1, §5.6, §4.25, §5.14, §4.29 (retirer sans perdre de données), §4.32 (« Désactiver pour moi »), §4.33 (mode sans extensions) | S→G | G-0, G-15, G-17, G-43 |
| 4 | Cohérence et standards | P2, §8, §4.8, §4.31 (contributions rendues par l'hôte en Adwaita) | S→G | G-12, G-13, G-16, G-19, G-28, G-62 |
| 5 | Prévention des erreurs | §5.6, §4.9, §4.29 (consentement), §2.10 (validation des thèmes), K-6, ND-3 | S→N | G-63, G-71 |
| 6 | Reconnaître plutôt que se souvenir | §4.24, §4.6, §4.3, §4.26, §4.30, §5.19 (galerie : « ce que vous obtenez ») | S→G | G-18, G-20, G-49, G-83 |
| 7 | Souplesse et efficacité | §3.2, §8.2, §4.30 (épinglage) | S→N | G-33, G-55, G-73 |
| 8 | Esthétique et sobriété | P1, P7, L-8, FT-4, §4.31 (densité d'attribution) | S→G | G-8, G-9, G-13 |
| 9 | Reconnaître, diagnostiquer et réparer les erreurs | P4, E-2, E-3, §5.16, **§4.36** (où, comment, un clic jusqu'à l'erreur, corrections rapides), §4.34, §2.10 | S→G | G-14, G-44, G-57, G-80 à G-82 |
| 10 | Aide et documentation | §4.26, §4.35 (Inspecteur) | S→N | G-58, G-76 |

## 2. WCAG 2.2, critères A et AA [S4]

Il y a 55 critères ; le 4.1.1 est retiré de WCAG 2.2.

| Critère | Niv. | Où | État | Preuve existante, ou écart |
|---|---|---|---|---|
| 1.1.1 Contenu non textuel | A | I-2, `FileIcon`, pages de l'aperçu, §4.34 (diagramme) | S→G | G-34, G-26 |
| 1.2.1–1.2.5 Médias temporels | A/AA | — | NA | Inkwell n'affiche ni audio ni vidéo. **À réexaminer** si une vue d'extension en introduit : elle relève alors de A-21. |
| 1.3.1 Information et relations | A | A-18, motifs APG | S→G | G-22, G-23, G-51 |
| 1.3.2 Ordre séquentiel logique | A | A-4 | S→N | `e2e/a11y.spec.ts` (à écrire) |
| 1.3.3 Caractéristiques sensorielles | A | C-PERSON, C-DIFF, FT-1 | S→N | relecture des textes, `ui/i18n.test.ts` (extension, à écrire) |
| 1.3.4 Orientation | AA | L-5 | S→N | aucun verrou d'orientation ; pas de test |
| 1.3.5 Finalité des champs | AA | §4.13 | **S** | `ui/ShareDialog.svelte:43` (`autocomplete="nickname"`) |
| 1.4.1 Utilisation de la couleur | A | A-3, C-PERSON, C-DIFF, FT-1 | S→G | G-27 |
| 1.4.2 Contrôle du son | A | — | NA | aucun son |
| 1.4.3 Contraste (minimum) | AA | §2.2, §2.10, `tokens.json` | S→G | G-1b, G-50, G-52, G-71. Preuve partielle : `theme/tokens.test.ts:44-50` (syntaxe), `e2e/shell.spec.ts:253` (axe, coque) |
| 1.4.4 Redimensionnement du texte | AA | T-9, A-11 | S→N | `e2e/reflow.spec.ts` (à écrire) |
| 1.4.5 Texte en image | AA | §4.18 | **S** (exception) | Les pages de l'aperçu sont le rendu exact, ce qui est essentiel ; le texte est dans la source, à côté. L'accessibilité du PDF exporté n'est pas prouvée (architecture `AD-44`, PDF/UA). |
| 1.4.10 Redistribution | AA | L-5 (texte qui revient à la ligne ; exemption limitée au 2D) | S→G | G-37. Le retour à la ligne de l'éditeur existe : `editor/setup.ts:122` (`EditorView.lineWrapping`). |
| 1.4.11 Contraste non textuel | AA | §2.2, §2.10 | S→G | G-1, G-3, G-4, G-53, G-71 |
| 1.4.12 Espacement du texte | AA | T-9 | S→N | `e2e/reflow.spec.ts` (à écrire) |
| 1.4.13 Contenu au survol ou au focus | AA | §4.6 | S→G | G-18 |
| 2.1.1 Clavier | A | A-4, §4 | S→G | G-20, G-22, G-23, G-54 |
| 2.1.2 Pas de piège au clavier | A | §4.8 | **S** (mécanisme) | `showModal()` natif dans les sept dialogues : `ui/ShareDialog.svelte:12`, `ui/AboutDialog.svelte:21`, `ui/prefs/Preferences.svelte:21`, `ui/people/PeopleDialog.svelte:21`, `ui/history/ApplyDialog.svelte:19`, `ui/sheet/PasteDialog.svelte:13`, `ui/history/DiffView.svelte:47`. Le test de piège reste à écrire. |
| 2.1.4 Raccourcis d'une seule touche | A | A-20 | **S** (présentation) | B, W, L ne sont lus que par le clavier de la vue de présentation : `present/keys.ts:61-69`. Lettres de l'arbre : S→N (G-22). |
| 2.2.1 Délai réglable | A | TO-1, A-13 | S→G | G-17, G-15 |
| 2.2.2 Mettre en pause, arrêter, masquer | A | T-11 | **S** | animations infinies coupées sous `reduce` : `ui/shell/HeaderBar.svelte:293`, `preview/Preview.svelte:306`, `ui/sheet/Grid.svelte:1025` |
| 2.3.1 Trois flashs | A | T-12 | S→N | aucun clignotement dans le code relevé ; pas de test |
| 2.4.1 Contourner des blocs | A | A-18 (F6) | S→N | G-33 |
| 2.4.2 Titre de page | A | N-2 | S→G | G-32 |
| 2.4.3 Parcours du focus | A | §4 (retour du focus et replis) | S→G | G-25, G-59 |
| 2.4.4 Fonction du lien | A | Aide, messages | S→N | — |
| 2.4.5 Accès multiples | AA | §3.2 | S→N | G-0, G-38, G-55 |
| 2.4.6 En-têtes et étiquettes | AA | §4.13, §6.2 | S→N | `e2e/a11y.spec.ts` (à écrire) |
| 2.4.7 Visibilité du focus | AA | `--focus-ring`, `--focus-ring-on-dark` | S→G | G-1, G-53 |
| 2.4.11 Focus non masqué | AA | A-5, TT-1 | S→N | `e2e/focus.spec.ts` (à écrire) |
| 2.5.1 Gestes du pointeur | A | boutons de zoom de l'aperçu | S→N | — |
| 2.5.2 Annulation du pointeur | A | boutons natifs ; glisser annulable | S→N | — |
| 2.5.3 Étiquette dans le nom | A | H-4 | S→G | G-31 |
| 2.5.4 Activation par le mouvement | A | — | NA | aucun capteur |
| 2.5.7 Mouvements de glissement | AA | L-4, A-8, §4.25, §4.30 | S→N | G-36 |
| 2.5.8 Taille des cibles (minimum) | AA | §2.5, A-7 | S→G | G-29 |
| 3.1.1 Langue de la page | A | A-14 | S→G | G-47 |
| 3.1.2 Langue d'un passage | AA | §5.13 (« English » porte `lang="en"`, « Français » `lang="fr"`) ; la langue d'un document déclarée par lui-même (`#set text(lang:)`, `lang` d'un en-tête Markdown) est portée sur l'éditeur | S→N | G-79 |
| 3.2.1 Au focus | A | aucune action au focus | S→N | — |
| 3.2.2 À la saisie | A | les réglages s'appliquent sans changer de contexte ; toast « Annuler » sur le rôle (§5.14) | **S→N** (corrigé) | `PeopleDialog.svelte:57` montre un toast **sans** « Annuler » ; G-43 |
| 3.2.3 Navigation cohérente | AA | §4.2, L-2, §4.31 (groupes du cœur à place fixe) | S→N | — |
| 3.2.4 Identification cohérente | AA | §6.4, §8.3 | S→G | G-61 |
| 3.2.6 Aide cohérente | A | A-15, §4.26 | S→N | G-58 |
| 3.3.1 Identification des erreurs | A | §4.13, §5.1, §5.6, §4.36 (problèmes du document) | S→G | G-80 |
| 3.3.2 Étiquettes ou instructions | A | §4.13 (libellé visible ; exemption explicite du champ de recherche) | **S** (recherche) | `ui/search/SearchField.svelte:55-69` (`aria-label`, loupe, titre de zone). Les autres champs : S→N. |
| 3.3.3 Suggestion après une erreur | AA | §5.4, §5.6, §5.1, §4.36 (conseils, corrections rapides) | S→G | G-44, G-81, G-82 |
| 3.3.4 Prévention des erreurs (données) | AA | P3, §4.9, §4.29 (données d'extension gardées, effacement confirmé) | S→N | — |
| 3.3.7 Saisie redondante | A | A-16 | S→N | — |
| 3.3.8 Authentification accessible | AA | A-16 | S→N | G-41 |
| 4.1.2 Nom, rôle et valeur | A | §4, A-19, FT-7, A-21 | S→G | G-22, G-23, G-24, G-26, G-51 |
| 4.1.3 Messages d'état | AA | A-10, §4.11, §4.36 (compte annoncé 0↔n, au plus toutes les 2 s) | S→G | G-60 |

**Au-delà de AA :** 2.4.13 (A-6) et 2.3.3 (T-11), visés, en S→N.

## 3. Motifs WAI-ARIA APG [S5]

| Motif | Composants | État | Preuve ou écart |
|---|---|---|---|
| Dialog (Modal) | `Dialog` §4.8, Aide et fenêtre des raccourcis §4.26, dialogues d'extension §4.31 | S→G | G-12, G-59 |
| Alert Dialog | §4.9, consentement §4.29, mode sans extensions §4.33 | S→G | G-64 |
| Menu Button, Menu | §4.2, menu Extensions §4.30, contributions de menu §4.31, corrections rapides §4.36, menu d'un modèle §5.19 | S→G | G-25, G-54 |
| Toolbar | §4.3 | S→G | G-20, G-54 |
| Tabs | sélecteurs §4.5 (activation automatique) ; onglets de fichiers §4.25 (activation **manuelle**) | S→G / S→N | G-23 ; G-55 |
| Tree View | §4.16 (Fichiers, Sommaire) | S→G | G-22 |
| Listbox | §4.22, §4.24, galerie des modèles §5.19 | S→G | G-51 ; galerie : S→N (G-83) |
| Grid | §4.19 | **S** | `e2e/sheet.spec.ts:613` (CSV-20 : grille ARIA, clavier, axe sans violation) |
| Combobox | §4.24 (palette), §4.37 (complétion dans l'éditeur) | S→N | G-84 |
| Radio Group | §4.5, thèmes §2.10, couleurs à l'accueil §5.1 | S→G | G-54 |
| Switch | §4.5, interrupteurs d'extension §4.29 | S→G | G-24 |
| Tooltip | §4.6 | S→G | G-18 |
| Window Splitter | §3.3 (nom et orientation inclus) | S→N | G-36 |
| Button (bascule) | H-4, punaise §4.30 | S→G | G-31 |
| **Extensions** : chaque point qui crée un widget reprend le motif de son emplacement (onglet → *Tabs* ; bouton → *Toolbar* ; page de réglages → lignes de préférences ; menu → *Menu*) | §4.31 | S→N | G-77 |

## 4. GNOME HIG et libadwaita 1.8 [S1] [S2]

| Motif ou règle | Où | État | Preuve ou écart |
|---|---|---|---|
| `AdwHeaderBar` : titre = contenu, peu de contrôles | §4.1, H-2 | S→G | G-32 |
| `AdwViewSwitcher` | §3.1, §4.5 | S→N | G-55 |
| `AdwTabBar` | §4.25 | S→N | G-55 |
| `AdwOverlaySplitView` | §3.4 | S→G | G-37 |
| Listes encadrées, `AdwExpanderRow` | §4.10, §4.29 | S→N | `.card.boxed` existe (`theme/adwaita.css`), mais sans ligne dépliable |
| `AdwPreferencesDialog` | §4.10, §5.13 | S→G | G-42 |
| `AdwAboutDialog` | §5.18 | S→G | G-48 |
| `AdwAlertDialog` | §4.9 | S→G | G-64 |
| `AdwToast` / `AdwToastOverlay` | §4.12, L-9 | S→G | G-16, G-17 |
| `AdwBanner` | §4.11, L-8 | S→G | G-13, G-14 |
| Popovers (Échap, retour au déclencheur) | §4.7 | **S→G** (corrigé) | G-25 : `ui/popover/MenuButton.svelte:54` rend le focus à l'éditeur |
| `AdwStatusPage` | §4.20 | S→N | G-57 |
| `AdwSpinner` | §4.21 | S→N | — |
| `AdwToggleGroup` | §4.5 | S→G | G-54 |
| Fenêtre des raccourcis, Ctrl+? | §4.26, §4.33 (conflits) | S→N | G-58 |
| Clavier HIG : F10, Ctrl+, Ctrl+W, F1, Échap | §8.2 | S→N | G-33 |
| Style d'écriture | §6.2 | S→N | `ui/i18n.test.ts` (extension, à écrire) |
| Adaptatif | §3.4 | S→G | G-37 |
| Couleurs, rayons, typographie libadwaita | §2 | S→G | G-1 à G-10 |
| Accessibilité HIG (HC, grand texte) | T-3, A-11, §2.10 (le HC d'un thème ne peut pas faiblir) | S→G | G-6 |
| Application *Extensions* de GNOME | §4.29 | S→N | G-70 |

## 5. Domaines de l'expérience

| Domaine | Où | État | Écarts (norme · preuve) |
|---|---|---|---|
| **Architecture de l'information, navigation** | §3.1, §3.2, §4.16, §4.25 | S→G | **G-0, ouvert** (Nielsen 1 et 3 · `App.svelte:197` ignore le chemin de `ui/shell/Sidebar.svelte:49`) ; G-55 ; G-38 |
| **Accueil, premier lancement, écrans vides** | §5.1 (validation et abandon), §5.2, §4.20 | S→N | G-39, G-40, G-57 |
| **États des composants** | §4 (neuf états, hors ligne revérifié), §4.29 (états du gestionnaire), §2.10 (thème refusé) | S→G | G-1, G-10, G-21, G-29 |
| **Textes, erreurs, reprise, annulation** | §6, §5.16, E-1 à E-3, TO-1 (y compris extensions et thèmes) | S→G | G-15, G-17, G-44, G-45, G-46, G-61 |
| **Notifications** | §3.5 (L-8 révisé : critique, puis questions, puis états), §4.11, §4.12 (une action) | S→G | G-13 à G-17, G-60 |
| **Collaboration** | §4.14, §4.15, §5.3, §5.4, §5.14, §5.15 | S→G | G-4, G-26, G-27, G-41, G-43 |
| **Projets à plusieurs fichiers** | §4.16 (G-0 : comportement attendu), §4.25, §5.5, §5.6 | S→G | **G-0**, G-22, G-36, G-55 |
| **Aperçu, export, impression** | §4.18, §5.8, §5.9, §5.12 | S→G | G-56 ; impression : Q10 |
| **Réglages, raccourcis, palette, aide** | §5.13, §8.2, §4.24, §4.26 | S→G | G-33, G-42, G-49, G-58, G-63 |
| **Langues fr/en** | §6.3, A-14, §5.13 | S→G | G-46, G-47, G-79 |
| **Clair, sombre, contraste élevé, couleurs forcées** | T-2 à T-4, §2.2 | S→G | G-1, G-6. Preuve partielle : `e2e/shell.spec.ts:253` (axe, clair et sombre). |
| **Tailles de fenêtre, jusqu'à 360px** | §3.3, §3.4, L-5 | S→G | G-36, G-37 |
| **Performance perçue** | §5.17, PERF-1 | **S→N** (corrigé) | Les paliers existent (`store/limits.ts:15-26`), mais aucune mesure de latence n'est faite : `e2e/perf.spec.ts` est à écrire. |
| **Types de fichiers et code (N20)** | §4.28 (FT-3 simplifié ; budget de chargement) | S→N | G-65 à G-69 |
| **Extensions : gestion, consentement, données (N21)** | §4.29 | S→N | G-70 |
| **Thèmes (N22)** : contrat, validation au chargement, Apparence, refus, retour au défaut | §2.10, `tokens.json` (`contract`, `themeContract`) | S→N | **G-71** |
| **Stockage distant, miroir, cycle de vie (N22)** | §4.29 | S→N | G-70 |
| **Points d'affichage et attribution** (architecture §15.4, §15.13) | §4.31 | S→N | G-77 |
| **Bouton Extensions et épinglage** | §4.30 | S→N | G-73 |
| **Extensions de projet (N23)** : provenance, consentement au-delà du niveau projet, interrupteur personnel, repli | §4.32 | S→N | G-72 |
| **Ne pas dénaturer (N24)** : zones protégées, dialogues inimitables, conflits de raccourcis, mode sans extensions | §4.33 | S→N | G-74 |
| **Diagrammes (N24)** : bloc, aperçu en ligne et dans l'aperçu, dernier rendu partagé, erreurs en ligne | §4.34 | S→N | G-75 |
| **Outils de développement (N25)** : mode développeur signalé et local, Inspecteur | §4.35 | S→N | G-76 |
| **Modèles de projet (N27)** : aucun modèle fourni ; galerie à « Nouveau projet » (vignette, description, ce que vous obtenez, page d'exemple, étiquettes) ; écran vide ; enregistrer en 3 étapes avec vignette automatique ; modifier, dupliquer, renommer, supprimer, importer, exporter, partager ; charte partagée entre modèles ; copie contre dépendance ; personnel ou d'équipe | §5.2, §5.19 | S→N | G-83 ; Q16 |
| **Erreurs de compilation claires (N28)** : soulignement et marge, liste Problèmes (popover existant), un clic jusqu'à l'erreur depuis chaque signal, fichiers importés et modèles, messages en clair avec le texte d'origine, conseils, corrections rapides, F8 et Ctrl+., aperçu qui garde le dernier bon rendu, gravités, compte annoncé poliment, calme pendant la frappe, même comportement pour Typst, Markdown, CSS, CSV et les extensions | §4.36, §4.18 | S→G | G-80, G-81, G-82. Existant : le compte et la liste (`ui/shell/StatusBar.svelte:84-129`) ; la dernière bonne version de l'aperçu (`preview/Preview.svelte:143`, `preview.stale`). |
| **Aide à l'écriture Typst (N29)** : complétion avec icônes de genre et aperçu de la documentation, aide de signature (paramètre courant, type, défaut), carte de survol rendue et « Aller à la définition » vers un onglet, indices de noms discrets, convention `///` encouragée par l'assistant de modèle, combobox et listbox APG, fr et en, Markdown (chemins, ancres), point d'extension | §4.37 | S→N | G-84 |
| **Commentaires** | §4.27 | H, et S→G sur l'onglet | Hors périmètre (Q8). L'onglet **est encore affiché** (`ui/history/HistoryDrawer.svelte:206-207`) : G-62. |
| **Présentation** | §4.23, §5.11 | S→G | G-7 |

## 6. Revues

[`CHANGES.md`](CHANGES.md) rattache chaque constat des revues n°1 et n°2 à sa correction.
