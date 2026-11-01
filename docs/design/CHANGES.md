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
| C5 | §4.16, §5.5 [B] Cliquer un fichier n'ouvre rien (`App.svelte:197`) | Inventaire **G-0**, corrigé par un lot séparé. La spec définit le modèle d'onglets de fichiers (§4.25) et l'ouverture (§5.5). |
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
