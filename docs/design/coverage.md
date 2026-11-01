# Matrice de couverture du système de design (N17, révision 2)

Grille de contrôle pour les reviewers. Pour chaque référentiel, chaque ligne dit où
[`spec.md`](spec.md) le traite, où en est le code, et l'écart éventuel, avec son
identifiant dans [`inventory.md`](inventory.md).

**Légende des états :**

| Sigle | Sens |
|---|---|
| **S** | spécifié et conforme dans le code (preuve existante) |
| **S→G** | spécifié ; le code s'en écarte (écart `G-n`, avec sa norme et sa preuve dans l'inventaire) |
| **S→N** | spécifié ; rien n'existe encore dans le code (nouveau) |
| **Q** | dépend d'une décision de l'utilisatrice (spec §11) |
| **H** | hors périmètre, justifié |
| **NA** | non applicable, justifié |

## 1. Les 10 heuristiques de Nielsen [S12]

| # | Heuristique | Où dans la spec | État | Écarts |
|---|---|---|---|---|
| 1 | Visibilité de l'état du système | §4.15 (état de synchronisation + popover), §4.17 (barre d'état), §4.18 (états de l'aperçu), §4.21 (attente), §5.17 (performance perçue), §4.25 (point « non enregistré » dans l'onglet) | S→G | G-2, G-30, G-38, G-56 |
| 2 | Correspondance avec le monde réel | P5, §6.4 (vocabulaire), glossaire, §4.29 (autorisations en phrases) | S→G | G-45, G-48, G-61 |
| 3 | Contrôle et liberté | P3, TO-1 (annulation durable), §5.6 (corbeille), §4.25 (fermer sans question), §5.14 (Laisser revenir) | S→G | G-15, G-43, G-17 |
| 4 | Cohérence et standards | P2, §8 (registre unique, icônes), §4.8 (un seul dialogue), Adwaita | S→G | G-12, G-13, G-16, G-19, G-28, G-62 |
| 5 | Prévention des erreurs | §5.6 (erreurs de nom en place), §4.9 (confirmation de l'irréversible), §4.29 (consentement), PERF-1, K-6 (raccourcis en doublon) | S→N | G-63 |
| 6 | Reconnaître plutôt que se souvenir | §4.24 (palette avec raccourcis), §4.6 (infobulles avec syntaxe), §4.3 (barre par format), §4.26 (fenêtre des raccourcis) | S→G | G-18, G-20, G-49 |
| 7 | Souplesse et efficacité | §3.2 (trois accès à tout), §8.2 (raccourcis), Ctrl+P, F6 | S→N | G-33, G-55 |
| 8 | Esthétique et sobriété | P1, P7, L-8 (un seul bandeau), FT-4 (palette sans arc-en-ciel) | S→G | G-8, G-9, G-13 |
| 9 | Aider à reconnaître, diagnostiquer et réparer les erreurs | P4, E-2, E-3, §5.16, §5.4, §5.12 | S→G | G-14, G-44, G-57 |
| 10 | Aide et documentation | §4.26 (Aide embarquée, F1), écrans vides avec « En savoir plus » | S→N | G-58 |

## 2. WCAG 2.2, critères A et AA [S4]

Il y a 55 critères. Le critère 4.1.1 est retiré de WCAG 2.2.

| Critère | Niv. | Où | État | Écart / justification |
|---|---|---|---|---|
| 1.1.1 Contenu non textuel | A | I-2, FileIcon, pages de l'aperçu « Page n sur N », avatars nommés | S→G | G-34 (nom lu en double), G-26 |
| 1.2.1 à 1.2.5 Médias temporels | A/AA | — | NA | Inkwell n'affiche ni audio ni vidéo. Typst n'en produit pas. |
| 1.3.1 Information et relations | A | A-18, motifs APG §4 | S→G | G-22, G-23, G-51 |
| 1.3.2 Ordre séquentiel logique | A | A-4 | S | — |
| 1.3.3 Caractéristiques sensorielles | A | C-PERSON, C-DIFF, FT-3 | S | aucun libellé du type « le bouton bleu » |
| 1.3.4 Orientation | AA | L-5 | S | aucun verrouillage |
| 1.3.5 Finalité des champs | AA | §4.13 (`autocomplete="nickname"` sur le nom) | S | `ShareDialog.svelte:43` |
| 1.4.1 Utilisation de la couleur | A | A-3, C-PERSON, C-DIFF, FT-1, FT-3 | S→G | G-27 |
| 1.4.2 Contrôle du son | A | — | NA | aucun son |
| 1.4.3 Contraste (minimum) | AA | §2.2, `tokens.json` (paires) | S→G | G-1b, G-50, G-52 |
| 1.4.4 Redimensionnement du texte | AA | T-9, A-11 | S→N | à prouver par des captures (§10) |
| 1.4.5 Texte en image | AA | §4.18 | S | Les pages de l'aperçu montrent le rendu exact du document, ce qui est essentiel. Le texte est disponible dans la source et le PDF exporté. |
| 1.4.10 Redistribution | AA | L-5, §3.4 | S→G | G-37 |
| 1.4.11 Contraste du contenu non textuel | AA | §2.2 (anneaux, marques, traits de personne) | S→G | G-1, G-3, G-4, G-53 |
| 1.4.12 Espacement du texte | AA | T-9 | S→N | preuve §10 |
| 1.4.13 Contenu au survol ou au focus | AA | §4.6 | S→G | G-18 |
| 2.1.1 Clavier | A | A-4, §4 | S→G | G-20, G-22, G-23, G-54 |
| 2.1.2 Pas de piège au clavier | A | §4.8 (Tab reste dans le dialogue, Échap en sort) | S | — |
| 2.1.4 Raccourcis d'une seule touche | A | A-20 | S | B et W n'agissent qu'en présentation |
| 2.2.1 Délai réglable | A | TO-1, A-13 | S→G | G-17, G-15 |
| 2.2.2 Mettre en pause, arrêter, masquer | A | T-11 (pas d'animation infinie sous `reduce`) ; l'indicateur d'attente s'arrête avec la tâche | S | — |
| 2.3.1 Trois flashs | A | T-12 | S | — |
| 2.4.1 Contourner des blocs | A | A-18 (F6) | S→N | G-33 |
| 2.4.2 Titre de page | A | N-2 (titre de fenêtre) | S→G | G-32 |
| 2.4.3 Parcours du focus | A | §4 (retour au déclencheur) | S→G | G-25, G-59 |
| 2.4.4 Fonction du lien | A | liens de l'Aide et des messages : texte explicite | S→N | — |
| 2.4.5 Accès multiples | AA | §3.2 : Fichiers, Ctrl+P, recherche, Sommaire | S→N | G-38, G-55 |
| 2.4.6 En-têtes et étiquettes | AA | §4.13, §6.2 | S | — |
| 2.4.7 Visibilité du focus | AA | `--focus-ring`, `--focus-ring-on-dark` | S→G | G-1, G-53 |
| 2.4.11 Focus non masqué | AA | A-5, TT-1 | S→N | preuve §10 (`e2e/focus.spec.ts`) |
| 2.5.1 Gestes du pointeur | A | le zoom de l'aperçu a des boutons ; aucun geste à plusieurs doigts n'est requis | S | — |
| 2.5.2 Annulation du pointeur | A | les actions partent au relâchement (boutons natifs) ; un glisser se lâche hors de la cible pour annuler | S | — |
| 2.5.3 Étiquette dans le nom | A | H-4 (nom = libellé visible) ; boutons de la barre d'état : le nom commence par le texte visible | S→G | G-31 |
| 2.5.4 Activation par le mouvement | A | — | NA | aucun capteur |
| 2.5.7 Mouvements de glissement | AA | L-4, A-8, §4.25 (déplacer un onglet), §4.16 (« Déplacer vers… ») | S→N | G-36 |
| 2.5.8 Taille des cibles (minimum) | AA | §2.5, A-7 | S→G | G-29 (zones de 28px) |
| 3.1.1 Langue de la page | A | A-14 | S→G | G-47 |
| 3.1.2 Langue d'un passage | AA | le texte du document n'est pas balisé ; l'interface est monolingue | S | — |
| 3.2.1 Au focus | A | aucune action au focus ; l'infobulle n'est pas un changement de contexte | S | — |
| 3.2.2 À la saisie | A | les réglages s'appliquent sans changer de contexte ; le changement de rôle donne un toast « Annuler » | S | — |
| 3.2.3 Navigation cohérente | AA | §4.2 (ordre du menu), L-2 | S | — |
| 3.2.4 Identification cohérente | AA | §6.4, §8.3 | S→G | G-61 |
| 3.2.6 Aide cohérente | A | A-15, §4.26 | S→N | G-58 |
| 3.3.1 Identification des erreurs | A | §4.13, §5.6 | S→N | — |
| 3.3.2 Étiquettes ou instructions | A | §4.13 (libellé toujours visible) | S | — |
| 3.3.3 Suggestion après une erreur | AA | §5.4, §5.6 (le message dit comment corriger) | S→G | G-44 |
| 3.3.4 Prévention des erreurs (données) | AA | P3 (corbeille, annulation durable), §4.9 | S→N | — |
| 3.3.7 Saisie redondante | A | A-16 | S | — |
| 3.3.8 Authentification accessible (minimum) | AA | A-16 (empreinte à comparer, pas à retaper) | S→N | G-41 |
| 4.1.2 Nom, rôle et valeur | A | §4 (motifs APG), A-19, FT-7 | S→G | G-22, G-23, G-24, G-51, G-26 |
| 4.1.3 Messages d'état | AA | A-10, §4.11 (régions persistantes) | S→G | G-60 |

**Au-delà de AA**, retenus sans coût :
- 2.4.13 Apparence du focus (A-6) ;
- 2.3.3 Animation des interactions (T-11).

## 3. Motifs WAI-ARIA APG [S5]

| Motif | Composants | État | Écart |
|---|---|---|---|
| Dialog (Modal) | `Dialog` §4.8 | S→G | G-12, G-59 |
| Alert Dialog | §4.9 | S→G | G-64 |
| Menu Button, Menu | §4.2 | S→G | G-25, G-54 |
| Toolbar | §4.3 (mise en forme, tableur, aperçu, présentateur) | S→G | G-20, G-54 |
| Tabs | §4.5 (sélecteurs), §4.25 (onglets de fichiers) | S→G | G-23 ; onglets de fichiers : S→N |
| Tree View | §4.16 (Fichiers, Sommaire) | S→G | G-22 |
| Listbox | §4.22 (Historique), §4.24 (palette) | S→G | G-51 |
| Grid | §4.19 | S | — |
| Combobox | §4.24 | S→N | — |
| Radio Group | §4.5 (groupes de bascules) | S→G | G-54 |
| Switch | §4.5 | S→G | G-24 |
| Tooltip | §4.6 | S→G | G-18 |
| Window Splitter | §3.3 | S→N | G-36 |
| Button (bascule) | H-4 | S→G | G-31 |

## 4. GNOME HIG et libadwaita 1.8 [S1] [S2]

| Motif ou règle | Où | État | Écart |
|---|---|---|---|
| `AdwHeaderBar` : titre = contenu, peu de contrôles | §4.1 | S→G | G-32 |
| `AdwViewSwitcher` (panneau de gauche, tiroir) | §3.1, §4.5 | S→N | G-55 |
| `AdwTabBar` (onglets de fichiers) | §4.25 | S→N | G-55 |
| `AdwOverlaySplitView` (états étroits) | §3.4 | S→G | G-37 |
| Listes encadrées, `AdwExpanderRow` | §4.10, §4.29 | S | `.card.boxed` |
| `AdwPreferencesDialog` (pages, groupes, portée) | §4.10, §5.13 | S→G | G-42 |
| `AdwAboutDialog` | §5.18 | S→G | G-48 |
| `AdwAlertDialog` | §4.9 | S→G | G-64 |
| `AdwToast` / `AdwToastOverlay` (file, 5s, une action) | §4.12, L-9 | S→G | G-16, G-17 |
| `AdwBanner` (un état, ne disparaît pas seul) | §4.11, L-8 | S→G | G-13, G-14 |
| Popovers (un tiers de la fenêtre au plus, Échap) | §4.7 | S | — |
| `AdwStatusPage` (écrans vides) | §4.20 | S→N | G-57 |
| `AdwSpinner` (délai, un seul) | §4.21 | S→N | — |
| `AdwToggleGroup` | §4.5 | S→G | G-54 |
| Fenêtre des raccourcis, Ctrl+? | §4.26 | S→N | G-58 |
| Clavier HIG : F10, Ctrl+, Ctrl+W, F1, Échap | §8.2 | S→N | G-33 |
| Style d'écriture (verbes, « … », pas de « OK ») | §6.2 | S | — |
| Adaptatif (seuils, taille minimale) | §3.4 | S→G | G-37 |
| Couleurs, rayons, typographie libadwaita | §2 | S→G | G-1 à G-10 |
| Accessibilité HIG (contraste élevé, grand texte) | T-3, A-11 | S→G | G-6 |
| Application *Extensions* de GNOME (lignes, interrupteur, retrait) | §4.29 | S→N | — |

## 5. Domaines de l'expérience

| Domaine | Où | État | Écarts (norme · preuve) |
|---|---|---|---|
| **Architecture de l'information, navigation** | §3.1, §3.2, §4.16, §4.25 | S→G | **G-0** (le clic sur un fichier n'ouvre rien · Nielsen 1, 3 · `App.svelte:197`) ; G-55 (pas de modèle d'onglets · HIG Tabs · `workspace.svelte.ts:120`) ; G-38 |
| **Accueil, premier lancement, écrans vides** | §5.1, §5.2, §4.20 | S→N | G-39 (nom inventé · règle 8 · `workspace.svelte.ts:181`) ; G-40 ; G-57 |
| **États des composants** (survol, focus, actif, désactivé, chargement, erreur, vide, hors ligne) | §4, tableau des huit états, et chaque composant | S→G | G-1, G-10, G-21, G-29 ; hors ligne : S→N |
| **Textes, erreurs, reprise, annulation** | §6, §5.16, E-1 à E-3, TO-1 | S→G | G-44, G-45, G-46, G-61, G-15, G-17 |
| **Notifications : toasts et bandeaux** | §3.5, §4.11, §4.12 | S→G | G-13 à G-17, G-60 |
| **Collaboration** (présence, curseurs, invitations, divergences, rôles, accès suspendu ou retiré, verrous) | §4.14, §4.15, §5.3, §5.4, §5.14, §5.15 | S→G | G-4, G-26, G-27, G-41, G-43 ; retiré et verrous : S→N |
| **Projets à plusieurs fichiers** (panneau, onglets, ouvrir, fermer, créer, renommer, déplacer, supprimer) | §4.16, §4.25, §5.5, §5.6 | S→G | **G-0**, G-22, G-55, G-36 |
| **Aperçu Typst et Markdown, export, impression** | §4.18, §5.8, §5.9, §5.12 | S→G | G-56 (chemin dans le toast · E-2 · `fr.json:450`) ; impression : Q10 |
| **Réglages, raccourcis, palette, aide** | §5.13, §8.2, §4.24, §4.26 | S→G | G-42, G-33, G-49, G-58, G-63 |
| **Langues fr/en** | §6.3, A-14, Q7 | S→G | G-46, G-47 |
| **Clair et sombre, contraste élevé, couleurs forcées** | T-2 à T-4, §2.2 | S→G | G-1, G-6 ; WebKitGTK : preuve T-2 |
| **Tailles de fenêtre, jusqu'à 360px** | §3.3, §3.4 | S→G | G-36, G-37 |
| **Performance perçue** (gros fichiers, WAN) | §5.17, PERF-1 | S | les paliers existent (`store/limits.ts:15-26`) ; preuve §10 |
| **Types de fichiers et code** (N20) : icônes, palette, détection, édition ou lecture, inconnus et binaires | §4.28 (FT-1 à FT-7) | S→N | G-65 à G-69 |
| **Extensions** (N21) : gestion, consentement, points d'affichage, confinement, collaboration | §4.29 | S→N | G-70 ; l'exécution et l'API relèvent de N18 |
| **Commentaires** | §4.27 | H | Ni modèle, ni plan : il faut une ADR (Q8). L'onglet est masqué (G-62). |
| **Présentation** | §4.23, §5.11 | S→G | G-7 (couleurs brutes) |

## 6. Points des revues de la révision 1

Voir [`CHANGES.md`](CHANGES.md). Chaque constat de Codex et de Fable y renvoie à la section qui
le ferme.
