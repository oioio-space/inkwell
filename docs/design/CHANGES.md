# Révision 2 du système de design (N17) — du constat à la correction

Cette révision répond aux revues n°1 de [Codex](reviews/01-codex.md) et de
[Fable](reviews/02-fable.md), puis aux demandes ajoutées pendant la révision.

## Revue Codex n°1

| # | Constat | Correction |
|---|---|---|
| C1 | §2.2 [M] Valeurs recalculées : avertissement et erreur en sombre (10,48 et 7,85), trait de la personne 4 (3,14) | Tous les rapports sont recalculés par script, en composition sRGB sans arrondi intermédiaire (spec §0). Valeurs corrigées en §2.2 ; 102 paires dans `tokens.json`. |
| C2 | §2.2 [M] Les fonds translucides n'ont pas de fond parent déclaré ; l'infobulle annonce 12,57 dans les deux thèmes | Champ `over` pour chaque paire. Bandeaux mesurés sur `--window-bg`, différences sur `--dialog-bg` (6,24 / 5,94) **et** sur `--view-bg`. Infobulle : 12,57 / 20,24 (§4.6). |
| C3 | §2.2, §7 [B] Paires oubliées : `--fg-dim` sur la sélection (4,48 / 4,26), numéros de ligne (2,51) | Paires interdites C-SEL et C-GUT (§2.2), marquées `forbidden` dans `tokens.json`. Inventaire G-50 (`HistoryRow.svelte:158`) et G-52 (`editor/setup.ts:38`). |
| C4 | §2.1 [M] `tokens.json` ne pilote pas tout le CSS : `--hl-link` manque, variantes en prose, HC en `\|`, typographie sans famille ; la parité est présentée comme existante | `tokens.json` réécrit : `--hl-link`, `--person-N-line` et `--person-N-ink` comme jetons, un nom `css` partout, `modes` (`dark`, `hc-light`, `hc-dark`), typographies avec `fontFamily` et `cssValue`. La parité est déclarée **à écrire** (T-5, G-1b), dans le README et dans le JSON. |
| C5 | §4.16, §5.5 [B] Cliquer un fichier n'ouvre rien (`App.svelte:197`) | Inventaire **G-0** — *rév. 3 : ouvert, non corrigé ; la mention « corrigé » était fausse*. La spec définit le modèle d'onglets de fichiers (§4.25) et l'ouverture (§5.5). |
| C6 | §3.4, §4.19 [M] Troisième source de toasts (`Sheet.svelte:605`, `:169`) et bandeaux locaux du tableur | L-8c, L-9 ; G-13 et G-16 complétés avec les lignes exactes ; `HistoryBars.svelte:143` ajouté. |
| C7 | §4.3–4.5, §4.16, §4.22 [M] Barre du tableur, radios des préférences, DiffView, HistoryBars ; l'historique est une `listbox` dont le focus ne suit pas | F-1 étendu à toutes les barres (§4.3) ; groupes de bascules (§4.5) ; historique en `listbox` APG (§4.22). Inventaire G-51, G-54. |
| C8 | §4.2, §4.13, §4.24 [M] ↑ ouvre sur le premier élément ; l'exception Échap → éditeur ; le menu à la place d'un `<select>` ; l'ARIA de la palette est incomplet | ↑ ouvre sur le dernier élément ; **retour au déclencheur sans exception** (§4) ; `<select>` natif quel que soit le nombre d'options (§4.5) ; palette en *combobox* complet (§4.24). |
| C9 | §4.8–4.9 [M] Retour du focus vers l'éditeur (Préférences, À propos) ; la confirmation en ligne ne gère pas le focus ; focus initial des contenus longs | §4.8 : focus initial sur le titre pour les contenus longs, retour au déclencheur ; §4.9 : variante en ligne. Inventaire G-59, G-64. |
| C10 | §4.11–4.12 [B] Ordre des régions live inversé | Régions **persistantes**, texte inséré ensuite (ARIA22, [S11]) ; correction explicite de la révision 1. Inventaire G-60 ; vérification Orca (§10). |
| C11 | §3.4, §4.12, §7 [B] La pause ne suffit pas pour 2.2.1 ; un bandeau prioritaire peut en masquer d'autres sans recours | TO-1 (annulation durable par type d'action) ; L-8b (« +N autres » et popover d'état, désormais en DOIT). |
| C12 | §1, §3, §5 [M] Arbitrer A6 et B10 ; « variantes » contre « brouillon » ; « Annuler » après un retrait | §3.1 (structure à gauche, temps et personnes à droite) ; §3.4 suit A6 (aperçu superposé sous 900px) ; §4.22 (« variante », amendement du glossaire à confirmer) ; L-10 corrigé. |
| C13 | §10 [M] Des preuves manquent : thèmes TS, cibles 34 et 44px, annonces, traçabilité | §10 réécrit en traçabilité règle → assertion : test des thèmes TS, cibles de tous les éléments focalisables, régions live et Orca, états concurrents, fonds composés. |
| C14 | §11 avis sur Q1–Q7 | Repris en recommandations « à confirmer par l'utilisatrice » (§11). |

## Revue Fable n°1

| # | Constat | Correction |
|---|---|---|
| F1 | [M] Anneau sur les fonds sombres (2,39, 2,07, 3,46) | `--focus-ring-on-dark` et `--on-dark-fg` (§2.2) ; paires dédiées, et paires interdites pour l'anneau normal ; inventaire G-53, en vague 1. |
| F2 | [M] B10 (Fichiers, Sommaire, Tâches à gauche), Liens (B7), RSZ-07/08, aperçu superposé sous 900px | §3.1 (sélecteur de gauche : Fichiers, Sommaire, Tâches, Liens) ; §4.16 ; §3.3 (vue présentateur, recherche, RSZ-07) ; §3.4 suit A6. |
| F3 | [M] Les questions sont ignorées par « un seul bandeau » | L-8 : **les questions d'abord**, puis les états bloquants, puis l'information ; bandeau-question (BN-3) ; §5.4, §5.11, §5.16. |
| F4 | [M] Commentaires sans composant | §4.27 : **hors du périmètre de N17**, onglet masqué (L-2b), Q8. Inventaire G-62. |
| F5 | [m] Cibles : la règle 9 dit 44px | §2.5 : écart explicite, appuyé sur la contrainte T11 ; amendement proposé (Q9) ; barre d'état en zones actives de 34px. |
| F6 | [m] P5 classé dernier | §1 : P5 et P6 sont des invariants, hors de l'ordre d'arbitrage. |
| F7 | [m] `--disabled-opacity` HC à 40 % ; `--duration-fast` en double emploi | T-3 (0,4 en HC) ; `--duration-fade` (150ms, seul conservé sous `reduce`) ; T-11. |
| F8 | [m] « fusion » / « Fusionner » | §6.4 « Intégrer la variante » ; inventaire G-61 (`fr.json:549`, `:618`, `:619`, `:622`, `:656`). |
| F9 | [m] « Annuler » disparaît avec le toast | TO-1 : Ctrl+Z, Historique, Corbeille, commande inverse sur la ligne ; A-13. |
| F10 | Inventaire : G-18 en vague 2 ; le point 1 en vague 1 | G-18 et G-19 en vague 2, avec la raison ; G-53 en vague 1. |
| F11 | [m] Preuves : file des messages, `light-dark()` sous WebKitGTK, 2.4.11 pour l'infobulle | §10 : `ui/messages.test.ts`, `e2e/theme.spec.ts` (application réelle), `e2e/focus.spec.ts`. |
| F12 | Avis sur Q1–Q7 | §11, « à confirmer ». |

## Demandes ajoutées pendant la révision

| Demande | Correction |
|---|---|
| Couvrir **toute** l'expérience, avec une matrice de couverture (Nielsen, WCAG 2.2 A et AA en entier, HIG et libadwaita) | Nouveau [`coverage.md`](coverage.md) : 10 heuristiques, 55 critères WCAG, 14 motifs APG, 21 règles HIG, 17 domaines ; chaque ligne renvoie à la spec et à un écart G-n (norme et preuve). |
| Projets à plusieurs fichiers : le clic ne fait rien ; définir le modèle d'onglets | §4.25 (onglets de fichiers), §5.5, §5.6 ; G-0, G-55. |
| Navigation, accueil, états hors ligne, aide, export et impression, performance perçue, collaboration (accès retiré, verrous) | §3.2, §5.1, le tableau des huit états de §4, §4.26, §5.12, §5.17, §4.14, §5.15. |
| N20 : types de fichiers, code en lecture, icônes, palette, détection, inconnus et binaires ; approche CodeMirror la plus légère | §4.28 (FT-1 à FT-7 et tableau des options) ; `--mark-code` ; Q11 ; G-65 à G-69. |
| N21 : extensions (gestion, consentement, points d'affichage, confinement, collaboration) | §4.29 ; `--mark-extension` ; §5.13 ; §6.4 ; Q12 ; G-70. |
| Inscrire le détail de N17 dans `docs/exigences.md` | Ligne N17 complétée. |

---

# Révision 3 — du constat de la revue n°2 à la correction

Ce qui suit répond aux revues n°2 de [Fable](reviews/03-fable-r2.md) et de
[Codex](reviews/04-codex-r2.md), toutes deux « NOT ACCEPTED », puis aux exigences N22 à N29.

## Revue Fable n°2

| # | Constat | Correction |
|---|---|---|
| B1 | N22, le contrat de thème, est absent | spec **§2.10** :<ul><li>jetons remplaçables, dérivés ou verrouillés ;</li><li>validation par l'hôte dans les quatre modes, avec refus complet ;</li><li>HC jamais affaibli ; ΔE des personnes ; aucun effet sous `forced-colors` ;</li><li>Apparence › Thème : vignettes, refus expliqué, « Revenir au thème par défaut » ;</li><li>ordre de priorité ;</li><li>un projet ne peut pas imposer un thème.</li></ul>`tokens.json` : `contract` sur chaque jeton, `themeContract`, paires HC **calculables par machine** dans les quatre modes. Les pseudo-paires « (contraste élevé) » sont retirées. Couverture : ligne N22. Inventaire : G-71. |
| B2 | Les points d'affichage contredisent l'architecture §15.4 | spec **§4.31** : une ligne par point de §15.4, dans le même ordre (stockage et miroir réunis), avec l'emplacement, l'attribution visible, l'état désactivé ou planté, et ce qui est interdit. Les surfaces de §15.13 sont reprises ; les **deux** sélecteurs, selon L-2. Couverture APG : ligne « Extensions ». |
| M1 | G-0 est présenté comme corrigé | G-0 est **ouvert** partout : inventaire, couverture, CHANGES rév. 2 (voir C5 plus bas). La spec §4.16 décrit le comportement attendu comme critère d'acceptation, avec le test `e2e/files.spec.ts` (à écrire). |
| M2 | Des preuves citées comme existantes n'existent pas ; des lignes S sont sans preuve | §10 réécrit avec une colonne **État** (existant / à écrire). Seuls les fichiers existants sont donnés comme preuve : `tokens.test.ts:44-50`, `e2e/shell.spec.ts:24`, `:151`, `:253`, `e2e/sheet.spec.ts:613`, `ui/i18n.test.ts`. Dans `coverage.md`, chaque S a une preuve ; les autres passent en S→N ; 3.2.2, *Popovers* et *Performance perçue* sont rétrogradés. |
| m1 | « Chaque paire interdite échoue » est faux | T-5 : une paire interdite est un **usage interdit par politique** ; le test vérifie son absence du code, pas son échec. |
| m2 | Les hexadécimaux de `--person-N-line` contredisent `color-mix` | Les hexadécimaux du JSON sont **canoniques** (§2.2, T-5). Les rapports sont recalculés à partir d'eux. |
| m3 | Chemins et lignes de l'inventaire | `workspace/workspace.svelte.ts`, `ui/icons/FileIcon.svelte`, `ui/shell/StatusBar.svelte` ; G-61 : `:549`, `:618`, `:621`, `:622`, `:656`, `:663`. |
| m4 | La touche L est oubliée | §4.23, A-20 et couverture 2.1.4 : B, W, L (`present/keys.ts:61-69`). |
| m5 | Infobulle sombre : la spec et le JSON divergent | La ligne `--tooltip-bg` de §2.2 est explicite (20,24 sur la vue sombre). |
| m6 | 3.1.2 et « English » | §5.13 : chaque nom de langue porte son `lang` ; couverture 3.1.2 ; G-79. |
| m7 | 3.3.2 contre le champ de recherche | §4.13 : exemption explicite (HIG *Search*), preuve `SearchField.svelte:55-69`. |
| m8 | Sur-ingénierie : FT-3, TO-1 | FT-3 ramené au séparateur et à l'en-tête en gras. TO-1 : la phrase « une fois par session » est retirée ; le rappel passe dans l'infobulle « Annuler · Ctrl+Z » et dans l'Aide. |

## Revue Codex n°2

| # | Constat | Correction |
|---|---|---|
| R2-B1 | N22 absent | Comme Fable B1. En plus : version du contrat, jetons inconnus et absents, priorité système, utilisateur, extension, composition des translucides, retour au thème sûr (§2.10). |
| R2-B2 | G-0 toujours ouvert | Comme Fable M1. |
| R2-M1 | La matrice confond exigence et preuve | Légende stricte (S = preuve existante citée). Commentaires : « hors périmètre », mais l'onglet est **encore affiché**, en S→G (G-62). Performance perçue en S→N. |
| R2-M2 | Exemption de reflow trop large | L-5 : le texte revient à la ligne (forcé sous 560px ; preuve `editor/setup.ts:122`). Seul le 2D est exempté : grille, pages, tableaux d'extension. Le test couvre les quatre formats. |
| R2-M3 | Transitions de focus incomplètes | §4 : retour au déclencheur et **quatre replis** ordonnés ; un contenu ouvert reçoit le focus. §4.25 : activation **manuelle**, `aria-selected`, un `tabpanel` unique, focus après le dernier onglet fermé. §3.3 : nom accessible et orientation des séparateurs. G-78. |
| R2-M4 | Retrait d'une extension et perte de données | §4.29 : « Retirer » **garde les données** ; « Effacer ses données… » est un geste à part, confirmé ; « Récemment retirées » ; la réactivation après une autorisation rendue est spécifiée ; TO-1 étendu. |
| R2-M5 | Stockage, cycle de vie, consentement incomplets | §4.29 : le local reste la vérité ; état de la copie ; échec ; arrêt ; migration ; actions de démarrage et d'arrêt visibles dans Activité. Le consentement reprend les phrases de §15.5, montre ce qui est automatique, et « Tout voir » à la mise à jour. |
| R2-M6 | Des états manquent | §4 : neuf états, et les actions hors ligne **revérifiées** à l'envoi. §4.29 : états du gestionnaire (vide, installation, mise à jour, abîmé ou incompatible, échec, pause, conflit). §5.1 : validation et abandon du nom. |
| R2-M7 | Prédicat des paires interdites | Comme Fable m1. |
| R2-M8 | Toasts à deux actions ; priorité absolue des questions | Une seule action par toast (§4.12) : intégration → « Annuler » (lignes teintées) ; export → « Ouvrir » (« Afficher dans le dossier » dans le menu). L-8 révisé : états **critiques** pour l'écriture, puis questions, puis autres états ; « +1 question » toujours visible. |
| R2-m1 | Moyens figés (25 langages, polices en TS) ; densité d'attribution | Couverture par priorité sous **budget** (60 Ko, 100ms, 5ms). T-1 TS limité aux couleurs et ombres. Densité d'attribution à 360px testée (§4.31, §10). |
| R2-m2 | Des arbitrages non ratifiés sont présentés comme clos | §11 : « Rien n'est ratifié » ; C12, F5 et F8 sont fermés en spécification, avec une **décision en attente**. `ux-principles.md` et le glossaire ne sont pas modifiés. |

## Correction de la révision 2

- **C5 (CHANGES rév. 2)** disait « corrigé par un lot séparé ». C'était **faux**. G-0 est
  ouvert, et le restera jusqu'à un commit qui le prouve.

## Exigences ajoutées pendant la révision 3

| Exigence | Correction |
|---|---|
| N22 (thèmes, stockage, cycle de vie) | §2.10, §4.29 ; G-71 |
| N23 (extensions de projet) | §4.32 : provenance, mode restreint et consentement au-delà du niveau projet, « Désactiver pour moi », repli, badge ; G-72 |
| N24 (ne pas dénaturer ; surfaces ; bouton Extensions ; diagrammes) | §4.30 (épinglage par la personne seulement), §4.31 (contributions déclaratives `{commande, emplacement, groupe, ordre, quand}`, rendues par l'hôte en Adwaita), §4.33 (zones protégées, dialogues inimitables, conflits de raccourcis, mode sans extensions), §4.34 (Mermaid, Graphviz : bloc, en ligne et dans l'aperçu, dernier rendu partagé, erreurs en ligne) ; G-73 à G-75, G-77 |
| N25 (outils de développement) | §4.35 : mode développeur local, pastille rouge permanente, Inspecteur dans le tiroir ; G-76 |
| N27 (modèles de projet, créés par les personnes) | §5.2 (« Nouveau projet » vide par défaut), **§5.19** : format `typst.toml` `[template]` + `[tool.inkwell]` ; Touying ; Markdown + CSS ; charte = paquet local partagé ; contenu copié, charte en dépendance versionnée ; galerie ; écran vide ; enregistrer en 3 étapes avec vignette automatique ; gestion dans Projets › Modèles ; personnel ou d'équipe ; G-83 ; Q16 |
| N28 (erreurs de compilation claires, « comme un IDE ») | **§4.36** : soulignement et marge ; liste Problèmes dans le popover existant ; **un clic** jusqu'à l'erreur depuis chaque signal, y compris les fichiers importés et les modèles ; F8 / Maj+F8 (libres) ; corrections rapides (Ctrl+., ampoule) ; messages en clair avec le texte d'origine et les conseils ; aperçu qui garde le dernier bon rendu ; gravités ; annonces polies ; calme pendant la frappe ; un modèle de diagnostic unique pour Typst, Markdown, CSS, CSV et les extensions ; G-80 à G-82 ; Q17 |
| N29 (aide à l'écriture Typst) | **§4.37** : complétion (icônes de genre, aperçu de la documentation), aide de signature, carte de survol avec « Aller à la définition » (F12, onglet), indices discrets désactivés par défaut, convention `///` de tidy encouragée par l'assistant de modèle, combobox et listbox APG, fr et en, Markdown (chemins, ancres), point d'extension, réglages ; G-84 |

---

# Révision 4 — revues n°3, puis N30 à N32

Revues : [Fable n°3](reviews/05-fable-r3.md) (ACCEPT, 3 majeurs et 10 mineurs) ;
[Codex n°3](reviews/06-codex-r3.md) (NOT ACCEPTED : 1 bloquant, 7 majeurs, 2 mineurs). Tous les
contrastes ont été recalculés par script : **128 paires** dans les quatre modes, aucun échec hors
des paires interdites.

## Revue Fable n°3

| # | Constat | Correction |
|---|---|---|
| M1 | La règle HC « ≥ Adwaita » rend le HC inthémable ; le message de refus est faux | §2.10 TH-D2 point 3 : **planchers** et non plus comparaison. `hcMin` = 7:1 pour le texte principal (WCAG 1.4.6), sinon `min` ; `themeContract.hcRule`. Le thème « papier » (15,56:1) est accepté. Le message de refus cite le rapport et le seuil. |
| M2 | Ctrl+. est pris par GTK (`insert-emoji`) | Corrections rapides sur **Alt+Entrée** (§4.36, §8.2). **Liste des raccourcis réservés par la plate-forme** dans K-6 : Ctrl+., Ctrl+;, Ctrl+Maj+U, Ctrl+Maj+I, raccourcis de GNOME Shell ; Ctrl+Espace « à risque », avec le repli Alt+/. Source [S25]. |
| M3 | La paire `--border` HC a disparu | Trois paires `--border` (vue, barre latérale, dialogue), `min: 3`, **`hcOnly: true`** (3,83 à 5,14). §2.2 explique l'exemption 1.4.11 en clair et en sombre. |
| m1 | Doublon et compte des paires | Doublon retiré ; le compte vient du fichier (128). |
| m2 | Cinq jetons sans classe ; `--on-dark-fg` | **Tout jeton a un `contract`** (vérifié par script : 0 manquant). Les délais d'accessibilité, `--text-editor` et la taille minimale sont verrouillés. `--tooltip-fg` est verrouillé comme `--on-dark-fg` ; la règle « toasts, infobulles et scène restent sombres » est écrite dans TH-D1. |
| m3 | Coupure à 4 000 caractères ; `problems.ts` | §4.36 et G-81 : 4 000, colonne déjà présente (`protocol.ts:8-19`, `:134-142`) ; source CSV `editor/csv/problems.ts` (`findProblems`). |
| m4 | Paquet incompatible : détails et cas trop ancien | §4.29 : deux lignes (trop récent PV-1, trop ancien PV-4), avec « Détails techniques ». |
| m5 | Texte alternatif des diagrammes hors de l'éditeur | §4.34 : **description** partagée, dans `image(alt:)`, à l'export HTML et dans la Vue lecture (voir aussi Codex R3-M1). |
| m6 | Mode débogage par extension sans interface | §4.35 : « Enregistrer pour rejouer », pastille « Débogage de X », « Exporter une trace expurgée… », expurgation cochée par défaut. |
| m7 | Bouton partagé sans motif APG | §4.36 : deux boutons voisins, deux arrêts de tabulation, *Menu Button* pour la liste, 34px chacun. |
| m8 | §5.19 sur-ingénieux | Partage d'équipe = rôle Écriture, sans nouvelle sorte de demande ; « Voir les projets créés avec » repoussé en v2 (PEUT) ; une seule corbeille (§5.6). |
| m9 | « Désactiver pour moi » ambigu ; cartes « Démarrer avec » | §4.32 : interrupteur sur chaque ligne, bandeau avec « Voir » seulement ; §4.31 ligne 21 : les cartes vont dans « Nouveau projet ». |
| m10 | Tab dans la complétion ; casse des types | Entrée seule insère, dans la palette comme dans la complétion ; Tab garde l'indentation. Les noms de type de §5.19 suivent la majuscule de phrase (§6.2). |
| §5 (crochets N30, N31) | Gravité « suggestion », onglet Relecture ; règle d'exclusion, MCP visible, propositions | §4.36 (quatrième gravité), §4.38, §4.39 (MCP en pastille L-8d ; « via IA » ; envoi conditionné à la politique et à l'architecture §15.18). |

## Revue Codex n°3

| # | Constat | Correction |
|---|---|---|
| R3-B1 | Le validateur ignore les états interactifs (`--hover-bg` noir accepté) | 21 paires d’**états** ajoutées : texte sur `--btn-bg` et `--hover-bg` sur 6 surfaces, sur `--selected-bg` sur 4 de plus, et sur `--card-bg`. Le contre-exemple est un cas de test (§10). La phrase « illisible par construction » est retirée. Le calcul a révélé **deux défauts réels**, corrigés : `--fg-dim` sur carte de dialogue en sombre à 4,13:1 (→ `--fg-dim-on-card`, G-85), et `--fg` sur papier en sombre à 1,00:1 (→ `--page-fg`). |
| R3-M1 | Diagramme sans alternative au dessin | §4.34 : description en phrases, proposée par le moteur, éditable, écrite dans la source, partagée et exportée ; son absence est un avertissement. Le code n'est qu'un supplément. |
| R3-M2 | Mode sans extensions, arrêt, anciens rangs de bandeaux | §4.33 : extensions **intégrées** chargées telles que livrées, registre d'une installation neuve. Arrêt : **1s au total**, en parallèle. §5.4, §5.16 et la vague 2 de l'inventaire suivent les rangs de L-8 (critique, questions, états, information). |
| R3-M3 | Menu Extensions : deux actions par ligne | §4.30 : un **popover** et non un menu ; action et punaise sont deux boutons ; clavier défini (↑ ↓ entre lignes, ← → dans la ligne, Tab, Échap) ; plafond de **6** épinglées, avec la septième désactivée et expliquée. |
| R3-M4 | Cartes dans une listbox ; cartes riches inaccessibles | Galerie = **groupe radio** de cartes sans contrôle interne ; gestion = liste de cartes avec un *Menu Button* chacune. Cartes de problème et d'aide : popover non modal où le **focus entre** par Ctrl+Maj+Espace ; Tab, Échap, retour au texte. |
| R3-M5 | Exception de reflow fondée sur une préférence | L'exception « ligne de code non repliée » est retirée ; le code revient à la ligne sous 560px. |
| R3-M6 | Compatibilité du contrat | TH-D2 point 1 : `contract: "^1.0"`, majeure refusée avec sa phrase, mineure tolérée, jetons `required` ; **bornes typées** dans `tokens.json` (objets `{min, max, unit}`, `type: color`, `type: font`). |
| R3-M7 | États des nouveaux parcours ; résultats périmés ; tinymist absent | §5.19 : table d'états de l'enregistrement. §4.36 et §4.37 : résultats liés à la version (LSP-3), correction périmée grisée et jamais appliquée, service indisponible sans indicateur permanent, essais bornés. |
| R3-m1 | Relevé obsolète | Comme Fable m3. |
| R3-m2 | Enregistrer comme modèle trop lourd | Un dialogue : nom, une phrase dans la langue de la personne, emplacement ; le reste est automatique, ou replié dans « Options avancées ». |
| Notes de la §4 | « Donner à X » ; §5.12 « Voir les problèmes » ; diagnostic sans position ; tests N27–N29 | « Donner à X » **déplace** l'action d'Inkwell, sans jamais la laisser sans raccourci ni menu. §5.12 emploie « Aller au premier problème », la commande de F8. Le cas « sans position » est défini (§4.36). §10 a des lignes de test pour les modèles, les problèmes, l'aide, l'écriture, l'IA et les ressources. |

## Exigences N30 à N32

| Exigence | Correction |
|---|---|
| N30, outils d'écriture | §4.38 : orthographe Hunspell, pointillé `--suggestion-line` jamais rouge ni compté, dictionnaire du projet partagé ; onglet **Relecture** du tiroir, à la demande, gravité « suggestion », aucun score ; concentration, machine à écrire, F11 ; mots et temps de lecture ; objectifs locaux sans série ; chercher et remplacer, regex masqué, aperçu ; typographie française au rendu et à la frappe (réglage du projet) ; Insérer : figure (texte alternatif obligatoire), tableau, équation, caractère (Ctrl+Alt+C), citation, note, renvoi, table des matières ; comparaison par mots ; préréglages d'export ; G-88 |
| N31, IA | §4.39 : onglet IA du tiroir (modes Corriger, Discuter, Réglages ; puces de contexte) ; « Demander à l'IA » sur la sélection (Ctrl+Alt+I) ; propositions par morceau, une modification « via IA », périmées grisées ; autorat « marqué IA par Camille » ; texte fantôme local, désactivé par défaut ; fournisseurs compatibles OpenAI (adresse, modèle, clé au trousseau, AC, proxy, mTLS, classe, prix, « Tester la connexion ») ; politique du projet (Désactivée, Sur cet ordinateur, Organisation, Libre), désactivée dans les projets partagés avant PD-1 ; dialogue d'envoi du cœur, avec le texte d'autrui marqué ; repère « contexte d'autres personnes », outils retirés ; confirmations ciblées ; coût, durée, arrêt, hors ligne ; MCP et agents désactivés par défaut, consentement par application, pastille permanente ; G-86 |
| N32, ressources | §4.40 : vue « En cours » de Préférences › Extensions ; table APG *Grid* triable (nom, projet, état, mémoire, calcul, délai contre le budget, activité facultative) ; arrêter, relancer, désactiver (aussi pour ce projet), journal, désinstaller ; services relançables mais non désactivables ; limites → toasts « Voir » regroupés, jamais de dialogue ; annonce sur changement d'état ; vocabulaire sans jargon ; G-87 |

## Désaccords à débattre

1. **Liste des problèmes : popover de la barre d'état ou onglet du tiroir ?**
   - L'architecture §15.17 ER-1 prévoit « un seul onglet « Problèmes à corriger » dans le
     tiroir ; compteur dans la **barre d'en-tête** ». Ce design garde le **popover existant** de la
     barre d'état (§4.36).
   - *Preuves :*
     - le popover existe et fonctionne (`ui/shell/StatusBar.svelte:84-129`) ;
     - les deux revues n°3 l'ont approuvé (Fable §6 ; Codex, ligne « Diagnostics N28 » : « Bon choix
       du popover existant ») ;
     - la barre d'en-tête est une **zone protégée** limitée à six contrôles du cœur (H-2, ND-1),
       sans compteur de problèmes ;
     - la règle de cohérence de `CLAUDE.md` demande de réutiliser l'existant avant d'ajouter un
       panneau.
   - *Proposition :* corriger ER-1 en « liste dans le popover de la barre d'état, compteur dans la
     barre d'état ». Q18.
2. **Ctrl+; pour les caractères spéciaux** (étude `editor-features.md` §9.5) : réservé par GTK 4 (même
   signal que Ctrl+., [S25]). Ce design prend **Ctrl+Alt+C**. L'étude est à corriger.
3. **Ctrl+Espace** : Fable le signale « à vérifier » (IBus). Il est **gardé**, parce que c'est la
   convention de CodeMirror et de VS Code, et que GNOME règle IBus sur Super+Espace par défaut
   [à confirmer par un e2e sous WebKitGTK]. Le repli Alt+/ est documenté. Si l'e2e montre une
   interception par défaut, on inverse.

---

# Révision 5 — passe de cohérence (N36), puis N34, N35, N37, N39, N40 et N41

Entrées :
- [`architecture/coherence.md`](../architecture/coherence.md) §6, avec ses 20 changements demandés
  au design ;
- l'architecture rév. 3 (`1.0.0-brouillon.4`) : §6.6, §6.7, §6.9, §6.10, §6.11, §9.11, §10.4,
  §11.1, §15.17, §15.18 ;
- les études `sync-ignore.md` et `encryption.md`.

Les points de la revue n°3 sont fermés depuis la rév. 4. Les contrastes ont été recalculés :
**131 paires**, aucun échec hors des paires interdites.

## Les 20 changements de la passe de cohérence

| # | Changement demandé | Correction |
|---|---|---|
| 1 | Onglets : ouverts ≠ chargés ; fermeture sans question | §4.25 : « ouvert ≠ chargé ≠ actif », rechargement ≤ 200ms, fermeture sans question (SAV-03) |
| 2 | Tiroir à trois destinations ; Inspecteur et « En cours » hors du tiroir | §3.1 : Rechercher, **Relire** (Problèmes, Commentaires, Activité, Versions), IA (si activée). §4.35 : Inspecteur = détail de Ressources. §4.40 : Ressources hors du tiroir. |
| 3 | Gauche à deux onglets ; Liens dans les détails ; Tâches en filtre ; nuage barré | §4.16 réécrit (Fichiers, Sommaire ; « Titres · Tâches · Tout » ; détails d'un fichier avec Liens ; `cloud-off`) |
| 4 | Un seul compteur de problèmes, dans la barre d'état ; clic = suivant ; liste dans Relire | §4.17, §4.36 ; **le désaccord n°1 de la rév. 4 est clos en faveur de C13** : le popover disparaît (G-93) |
| 5 | Barre d'état : quatre éléments, le reste sous ⋯ | §4.17 : problèmes, non enregistré, position et mots, page ; indicateur Ressources seulement en cas de pause |
| 6 | Raccourcis (Ctrl+K, Ctrl+Maj+P, Alt+Entrée) ; corriger la règle 3 ; export après audit ; plus de Ctrl+. | `ux-principles.md` règle 3 **corrigée** ; §8.2 : export sans raccourci jusqu'à l'audit ; inventaire G-82 et G-49 réécrits, sans Ctrl+. |
| 7 | Historique sans Git | §4.22 réécrit : frise, vue du passé, curseur temporel, Revenir au présent, restaurer = nouvelle version, comparer par mots, vocabulaire figé |
| 8 | Variantes | §4.22 points 6 et 7 : créer, comparer, proposer d'intégrer, intégrer, abandonner, divergence côte à côte, Dupliquer le projet |
| 9 | Protection du projet | §4.43 : case cochée par défaut pour un projet partagé ; déverrouillage ; phrase et kit ; « sans eux, perdus » ; export protégé ou lisible |
| 10 | Sauvegarde et restauration | §4.44 : date du dernier succès, restauration à côté, fraîcheur ST-7 |
| 11 | Vue Ressources | §4.40 : vue à part ; annuler, mettre en pause, arrêter, désactiver ; limites à la baisse ; portée sans nom de fichier ni titre (RS-5) ; IA et sauvegarde en lignes |
| 12 | Non synchronisés | §4.42 : « Ne pas synchroniser », « Pour tout le projet », aperçu, « ne retire pas les copies déjà reçues » |
| 13 | IA : indicateur, aperçu à chaque nouvelle provenance, consommation, marques, jeton MCP par client, « se présente comme » | §4.39 : pastille « IA : envoi vers X » pendant une séance ; dialogue d'envoi à chaque nouvelle provenance ; jetons, « estimation », « coût inconnu » ; jeton par application ; « se présente comme » |
| 14 | Propositions : une présentation ; « appliquer sans relire » par extension | §4.41 ; §4.29 (réglage sur la ligne, jamais pour une extension de projet ni l'IA) |
| 15 | Modèles : un emplacement, aucun gabarit, choix des fichiers, CSS = réglage d'export HTML | §5.19 : `Documents/Inkwell/Modèles/` seul ; plus de « modèles de l'équipe » ; fichiers inclus visibles ; CSS = export HTML (`AD-78`) ; Q16 tranchée |
| 16 | États honnêtes | §4.15 (table des états acquittés, H-HON), §5.16 (deux messages corrigés) ; G-94 |
| 17 | Extensions : un gestionnaire, puzzle, épinglage, mode sans extensions, badge | §4.29 : portée et provenance sur chaque ligne, format `.inkwell-plugin` ; le reste était déjà là (§4.30, §4.32, §4.33) |
| 18 | Plates-formes | §8.4 : Cmd sur macOS, barre de menus du système depuis le registre, réservés par système, fiche « aperçu indisponible sur ce système » |
| 19 | Aucun format bureautique ; collage riche oui | §4.45, §5.12 |
| 20 | Glossaire | `glossary.md` : ajout de version, variante, proposer d'intégrer et intégrer, protéger, phrase et kit de secours, sauvegarde, ressources, non synchronisé ; retrait de jalon et de brouillon ; « fusionner » interdit. §6.4 complété. |

Et aussi, depuis la carte produit et les manques :
- Accueil avec « Restaurer une copie » ; « Nouveau » en Typst, Markdown, texte, tableau (§5.2) ;
- Préférences en six pages, chaque ligne avec sa portée (§5.13) ;
- appareils (M02) ; nettoyage (M09) ; profil sans secrets (M11) ;
- impression au MVP par le lecteur de PDF (M21) ;
- script lecteur d'écran de la collaboration (M07, §10).

## Exigences nouvelles

| Exigence | Correction |
|---|---|
| N34, ne pas synchroniser | §4.42, §4.16 (marque), §5.13 (écran des règles) ; G-91 |
| N35, plates-formes | §8.4 ; G-95 |
| N37, protection | §4.43, §4.44, §5.12 (exports) ; G-92 |
| N39, historique sans Git | §4.22 (frise, voyager, restaurer sans détruire, versions nommées, comparer) ; prior art : Google Docs, Figma, Time Machine, Patchwork [S28] ; G-89 |
| N40, aucun format bureautique | §4.45 ; test de chaînes interdites (§10) ; G-97 |
| N41, variantes | §4.22 (variantes, relecture d'intégration, choix côte à côte), §4.41 ; G-90 |

Note de méthode : les références Patchwork, Google Docs, Figma et Time Machine viennent de la
connaissance des produits et de leurs pages d'aide publiques, citées en [S28]. Leur contenu n'a
pas été relu en ligne pour cette révision. L'architecture (HI-6) s'appuie aussi sur Patchwork.

## Désaccords à débattre

1. *(clos)* La liste des problèmes : le popover de la rév. 4 cède à C13 (liste dans Relire,
   compteur unique dans la barre d'état). Le désaccord n'a plus d'objet.
2. **La barre de mise en forme à l'ouverture.** `coherence.md` §4 écrit « la barre de mise en forme
   se masque ».
   - T11 (D, validé) dit que l'utilisatrice a **choisi une barre fixe**, repliable par
     Ctrl+Maj+T, au lieu de la barre sur sélection prévue par la conception.
   - Ce design garde **le choix de la personne** (§3.1). À confirmer par l'utilisatrice avant de
     changer un comportement qu'elle a validé.
3. **Ressources en fenêtre auxiliaire.**
   - L'architecture (RS-6) dit « vue » sans en fixer le contenant, et C18 admet des fenêtres
     auxiliaires.
   - Ce design en fait une fenêtre non modale, pour voir l'effet d'un arrêt à côté du travail.
   - Si l'utilisatrice préfère une page dans la fenêtre principale, la table reste identique.
4. **Ctrl+; et Ctrl+Espace** : inchangés depuis la rév. 4 (désaccords n°2 et n°3), toujours ouverts.

---

# Révision 6 — revues n°5

Revues : [Fable n°5](reviews/07-fable-r5.md) (ACCEPT : 4 majeurs, 11 mineurs) et
[Codex n°5](reviews/08-codex-r5.md) (NOT ACCEPTED : R5-B1, R5-M1 à M4, R5-m1). Les contrastes ont été
recalculés : **161 paires** dans les quatre modes, aucun échec hors des paires interdites.

**Deux défauts réels**, trouvés par les nouvelles paires :
- le texte secondaire sur un survol (`--fg-dim` à **4,22:1** sur un popover sombre, **4,35:1** sur la
  barre latérale claire) : `--fg-dim-on-state` et G-100 ;
- l'anneau de focus intérieur sur une sélection : vérifié à 4,08:1 au moins, donc conforme.

## Codex n°5

| # | Constat | Correction |
|---|---|---|
| R5-B1 | Texte secondaire sur `--hover-bg` hors contrat ; le thème `#bbbbbb` passe | 30 paires d'états ajoutées : `--fg-dim` interdit sur survol et sélection, `--fg-dim-on-state` sur 12 fonds, anneau intérieur sur `--hover-bg` et `--selected-bg` ; combinaisons non vérifiables interdites (TH-D2) ; contre-exemple `#bbbbbb` dans §10 ; §4.37 (indices) en `--fg-dim-on-state` |
| R5-M1 | Secours trop lourd ; phrase seule refusée ; promesses trop absolues | §4.43 réécrit : phrase **une fois par personne** ; « Mettre mon secours en lieu sûr » (écrite, imprimée, ou fichier ; vérification avec complétion et collage) ; kit à un endroit choisi, **réécrit en silence**, avertissement seulement en échec ; **récupération par la phrase seule** (ST-8) avec l'avertissement de fraîcheur ; états précis de ce qui manque ; quand la perte est définitive, dit exactement ; copie lisible durable exclue de la promesse |
| R5-M2 | Focus et transaction des propositions | §4.41 réécrit : entrer, sortir (Échap, proposition gardée en attente), morceaux focalisables nommés, touches **seulement sur un morceau, jamais dans un champ** ; **choix locaux, puis une seule modification** à « Appliquer » ; Tout accepter laisse les divergences ; choix gardés après recalcul ; un seul morceau = « Accepter et appliquer » |
| R5-M3 | Historique au clavier ; jargon Git | §4.22 : options en texte seul, focus itinérant (pas `aria-activedescendant`), **barre d'actions** externe, comparaison par **deux sélecteurs**, `aria-valuetext` du curseur, résumés en faits mesurés ; « Exporter l'historique en dépôt Git » **retiré de l'interface d'écriture** (outils du mode développeur) |
| R5-M4 | Collisions macOS | §8.2 réécrit : **une liaison par système** pour chaque commande (plus de traduction Ctrl→Cmd) ; Remplacer ⌥⌘F, suivant ⌘G, aller à la ligne ⌘L, aller au fichier ⇧⌘O (⌘P = Imprimer), code ⌃⌘E, formule ⇧⌘M, etc. ; table réservée macOS ; G-99 |
| R5-m1 | Résidus | §4.36 (liste dans Relire), §4.38 (« brouillons » retiré), §4.37 (« étape 2 » retirée), inventaire vague 5, §4.40 (« En cours » retiré) |

## Fable n°5

| # | Constat | Correction |
|---|---|---|
| M1 | Raccourcis : Ctrl+Alt (AltGr sous Windows), ⌥⌘H, ⌃Espace | **K-7** : aucune combinaison Ctrl+Alt sur aucun système ; aucun Alt+alphanumérique sous Windows ; ⌥+lettre jamais seul sous macOS. Les sept Ctrl+Alt sont réattribués : Versions par Relire, enregistrer une version Ctrl+Maj+S / ⌥⌘S, IA et caractère spécial sans raccourci par défaut, concentration Ctrl+Maj+D, source → aperçu Ctrl+Maj+J, paragraphe = titre répété. **Trois tables réservées** (Linux, macOS, Windows) tirées de [S2], [S30] et [S31], vérifiées en ligne pour [S30] et [S31] ; test K-6 sur les trois systèmes ; suggestions : ⌥Échap sous macOS. |
| M2 | Portée des variantes | **une variante est toujours celle du projet** ; repère des fichiers modifiés ; sous-titre « 2 fichiers modifiés ». L'alignement avec HI-6 (variantes par document) reste à faire dans l'architecture (voir plus bas). |
| M3 | Phrase par personne contre cérémonie par projet ; défaut à Rejoindre | une phrase par personne, une fois ; défaut au moment où le projet devient partagé (§5.3) et à Rejoindre (§5.4), par une ligne et non par un dialogue ; §4.43 est la référence unique |
| M4 | Résidus de la passe de cohérence | Exporter sans Ctrl+Maj+E (§5.12) ; §6.4 : « Restaurer cette version » et « Revenir au présent » autorisés ; deux entrées « Intégrer », chacune avec son objet ; Q15, Q17 et Q18 mis à jour ; `coverage.md` ligne N28 ; « Préférences › Avancé » devient « Application » |
| m1 | Chaque flèche recharge le passé | 300ms de calme, ou Entrée ; seul l'arrêt est annoncé |
| m2 | Trois confirmations pour restaurer | l'aperçu est la confirmation ; plus de dialogue |
| m3 | Toast de kit ; saisie de la phrase sans aide | kit réécrit en silence ; 24 champs, complétion, collage |
| m4 | Repères violets proches | voie de variante en `--fg-dim` avec son nom ; version nommée en losange (forme) ; violet réservé à l'IA |
| m5 | Sélecteur de variante dans l'en-tête | ajouté à l'anatomie du centre (§4.1), dans la zone protégée |
| m6 | Question « Proposer d'intégrer », touches dans un champ, mot « Git » | ajoutée aux questions de L-8 ; touches inactives dans un champ (§4.41) ; le mot Git n'apparaît plus du tout (choix plus strict que l'exception proposée, pour suivre Codex R5-M3) |
| m7 | §5.13 | six pages exactes, portée de chaque ligne ; enregistrement et destination de sauvegarde = **cet appareil** |
| m8 | Quatrième destination | règle écrite : au plus une, « Extensions », seulement si nécessaire |
| m9 | « Rapport » en §5.9 | retiré ; « Adwaita » et « Lecture » restent |
| m10 | Icône `branch` | `clock` ; G-101 |
| m11 | [S28], Time Machine | [S28] cite les entrées 01 et 02 du carnet, vérifiées en ligne. §4.22 dit que la règle d'Inkwell (ajouter, jamais remplacer) **dépasse** Time Machine. |

## Désaccords : positions communes des deux revues, appliquées

1. **Barre de mise en forme fixe** : gardée, puisqu'elle a été validée par l'utilisatrice (T11).
   Deux textes sont **à corriger, hors de ce dossier**, et ne sont pas modifiés ici :
   - `architecture/coherence.md` §4 : « la barre de mise en forme se masque » devient « la barre
     de mise en forme suit le choix de la personne (fixe, repliable par Ctrl+Maj+T) » ;
   - `ux-principles.md` règle 2 : « La barre de formatage apparaît sur la sélection » devient « La
     barre de mise en forme est fixe et repliable (choix de l'utilisatrice, T11) ».
2. **Ressources** : une fenêtre auxiliaire non modale, ouverte sur geste ; **dans la fenêtre sous
   900px** ; une seule à la fois ; « Voir » la met au premier plan (§4.40).
3. **Ctrl+Espace** : sous Linux seulement, en attendant l'e2e WebKitGTK avec IBus actif (repli
   Alt+/) ; **⌥Échap** sous macOS ; Ctrl+Espace sous Windows, avec le repli Ctrl+J. Ctrl+; n'est pris
   sur aucun système. Le caractère spécial n'a plus de raccourci par défaut, ce qui clôt le
   désaccord n°2 de la rév. 4.

## À aligner dans l'architecture (hors de ce dossier)

- **HI-6** : les variantes sont tenues par document ; l'interface présente une **variante du
  projet**, c'est-à-dire l'ensemble de ces variantes, créé et intégré en une fois.
- **HI-5** : l'export en dépôt Git sort de l'interface d'écriture (outils du mode développeur ou
  extension).

---

# Révision 7 — revues n°6

Revues : [Fable n°6](reviews/09-fable-r6.md) (ACCEPT : 1 majeur, 9 mineurs) et
[Codex n°6](reviews/10-codex-r6.md) (NOT ACCEPTED : 2 majeurs, 1 mineur). **160 paires** recalculées :
aucun échec hors des paires interdites. Les rapports des deux contre-exemples ont été revérifiés par
script.

| # | Constat | Correction |
|---|---|---|
| Codex R6-M1 | Deux contrats de proposition pour l'IA ; *Listbox* contre groupes ; « Accepter et appliquer » | §4.39 point 2 renvoie **entièrement** à §4.41 : morceaux en groupes focalisables, choix locaux, **un seul « Appliquer »** partout, même pour un seul morceau ; « Accepter et appliquer » est retiré. « Annuler cette discussion » ne défait que ce qui a été **appliqué**. §4.39 point 11 n'est plus une *Listbox*. `coverage.md` (ligne *Listbox*) est aligné. |
| Codex R6-M2 | ⌘P contre ⇧⌘O ; ⇧⌘? réservé mais lié | §3.2 : ⇧⌘O sous macOS. Aide sous macOS = **commande déléguée au système**, que le registre décrit sans la capturer ; K-6 reconnaît ces commandes. Les choix propres à Inkwell (Ctrl+P sous GNOME, F9) sont dits **explicites**, et non des conventions ; les liaisons citées dans tout le texte sont testées contre la même table. |
| Codex R6-m1, Fable m8 | `coverage.md:176` garde « popover existant » et « F8 et Ctrl+. » | La cellule est réécrite avec le seul comportement cible (Relire, Alt+Entrée, F8). Le popover ne reste que dans la colonne des preuves du code actuel. |
| Fable M1 | Le contre-exemple `#bbbbbb` est accepté par le contrat | Remplacé par **`#acacac`** : texte principal 6,94:1, `--fg-dim-on-state` **4,45:1**, anneau **2,67:1**, donc **refusé**. `#bbbbbb` est donné comme cas **accepté** (4,81 ; 3,16), ce qui prouve le seuil (§2.10, §10). |
| m1 | `#ffffff` brut dans la paire 23 | `--on-dark-fg` |
| m2 | « Git » dans les outils du mode développeur | « Exporter l'historique (format technique)… » ; le nom du format n'apparaît que sous « Détails techniques ». La règle « aucun mot de Git » vaut **même en mode développeur**. |
| m3 | Deux lignes « retour en arrière » dans §6.4 | une seule ligne, avec l'union des mots interdits |
| m4 | `--variant-line` orphelin | jeton et paire retirés ; la voie d'une variante est en `--fg-dim` |
| m5 | Bandeau de secours permanent | bandeau **seulement** si la phrase n'est pas notée ou pas vérifiée ; fermable ; revient au lancement suivant, 7 jours plus tard ; jamais pour le kit ; état « pas encore notée » ajouté |
| m6 | ⌃⌘S n'est pas la convention de la barre latérale | panneau de gauche **⌥⌘S** (convention du Finder) ; Enregistrer une version **⌃⌘S** ; ⌃⌘D, ⌥⌘T et ⌃F1 à ⌃F8 ajoutés aux réservés macOS ; Zones sous macOS sur **⌘F6** (⌃F6 est réservé) |
| m7 | Sort de la variante après intégration | **close**, visible dans la frise (filtre « variantes intégrées ») ; on peut en repartir (§4.41) |
| m9 | Infobulle d'un bouton `disabled` inatteignable | `aria-disabled="true"` et focalisable : actions de la frise (§4.22), « Appliquer » (§4.41) |
