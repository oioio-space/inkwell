# Étude : les fonctions de Typora, classées pour inkwell (N58, N53)

Date : 2026-11-07. Exigences visées : N58, N53 (et N20, N30, U13, U14, U18, U30).
À lire avec `editor-features.md` (étude des fonctions d'édition), `docs/design/S13-spec.md` et `preview-view-modes.md`.

## Méthode

- Sources primaires d'abord : typora.io, support.typora.io (référence Markdown, raccourcis, images, tableaux, copier-coller, export, diagrammes, focus), notes de version stables, tickets `typora/typora-issues`. Liens en fin de document.
- Avis 2024-2026 : agrégateurs (G2, Toolradar, Fitgap, comparatifs). Ils sont minces. Les tickets GitHub servent de preuve pour les défauts.
- Version de référence : Typora 1.14.15 (canal stable). Prix : 14,99 $, 3 appareils, essai de 15 jours.
- Raccourcis : variante Windows/Linux. Sur macOS, Ctrl devient Cmd.
- État dans inkwell : vérifié dans le graphe gortex (`fichier:symbole`) ou dans le registre (`docs/exigences.md`). Aucun build ni test lancé.
- Classes : **adopter** (tel quel, compatible avec les invariants), **adapter** (autrement, dire comment), **rejeter** (dire pourquoi).
- Ce qui n'a pas pu être vérifié est marqué **[à vérifier]**.

## Fonctions de Typora

**Édition sans mode.** Le Markdown se rend pendant la frappe, sans volet d'aperçu. Le balisage réapparaît quand le curseur entre dans l'élément. Le mode source (`Ctrl+/`) montre le texte brut.
**Paragraphes.** Titres `Ctrl+1` à `Ctrl+6`, paragraphe `Ctrl+0`, niveau `Ctrl+=` / `Ctrl+-`. Tableau `Ctrl+T`, bloc de code `Ctrl+Shift+K`, bloc de maths `Ctrl+Shift+M`, citation `Ctrl+Shift+Q`. Listes `Ctrl+Shift+[` et `]`, retrait `Tab` / `Shift+Tab`. Conversion entre liste ordonnée, à puces et de tâches.
**Format.** Gras `Ctrl+B`, italique `Ctrl+I`, souligné `Ctrl+U`, code `` Ctrl+Shift+` ``, barré `Alt+Shift+5`, lien `Ctrl+K`, image `Ctrl+Shift+I`, effacer le format `Ctrl+\`.
**Sélection.** Mot `Ctrl+D`, ligne ou phrase `Ctrl+L`, portée de style ou cellule `Ctrl+E`. Déplacer une ligne ou un paragraphe : `Alt+↑/↓` (1.11).
**Tableaux.** Taper `| A | B |` puis Entrée crée le tableau. Une barre flottante règle la taille, l'alignement, la suppression. `Ctrl+Entrée` (ou `Tab` en fin) ajoute une ligne. `Ctrl+Shift+Retour arrière` supprime la ligne. Glisser le bord déplace une ligne ou une colonne. Menu contextuel pour les colonnes. L'alignement s'écrit avec les `:` du séparateur.
**Blocs de code.** ` ``` ` + Entrée, langue optionnelle, coloration d'environ 100 langages, indentation automatique.
**Maths.** `$$` + Entrée ouvre un champ TeX rendu par MathJax 4. En ligne `$…$` (option). Numérotation automatique des équations (option). Délimiteurs `\(…\)` et `\[…\]` (1.11).
**Diagrammes.** Blocs `mermaid`, `flow`, `sequence` (option). Clic droit : enregistrer en SVG ou PNG.
**Notes, sommaire, métadonnées.** `[^id]` : le contenu de la note s'affiche au survol. `[toc]` + Entrée : sommaire tenu à jour. `---` + Entrée en tête : bloc YAML (aussi réglages d'images).
**Extensions.** Tâches `- [ ]` cliquables. Émojis `:nom:` avec complétion. Surlignage `==…==`, `~indice~`, `^exposant^` (options). Alertes GitHub `> [!NOTE]` (1.8). HTML en ligne, `<video>`, `<iframe>`.
**Saisie.** Paires automatiques de crochets et guillemets. Option : paires de `*` et `_`. Ponctuation « intelligente » **[à vérifier : aucune page officielle trouvée]**.
**Copier-coller.** Copier met HTML, RTF et texte dans le presse-papiers. Coller convertit le HTML en Markdown. `Ctrl+Shift+V` colle en texte brut. `Ctrl+Shift+C` copie le Markdown. « Copier comme HTML », « Copier sans le thème ».
**Images.** Glisser, coller, choisir un fichier. Option : copier l'image dans un dossier (`typora-copy-images-to` dans le YAML). Chemins relatifs, racine `typora-root-url`. Redimensionnement et zoom. Envoi vers un serveur d'images (iPic, PicGo, PicList). Clic droit « Supprimer l'image » efface aussi le fichier.
**Navigation.** Barre latérale `Ctrl+Shift+L` : plan `Ctrl+Shift+1`, liste `Ctrl+Shift+2`, arborescence `Ctrl+Shift+3` (filtres et clavier en 1.14). Ouverture rapide `Ctrl+P`, rouvrir le fichier fermé `Ctrl+Shift+T`, documents ouverts `Ctrl+Tab`. Liens internes vers un titre.
**Recherche.** `Ctrl+F`, `F3` / `Shift+F3`, remplacer `Ctrl+H`, recherche dans le dossier.
**Affichage.** Focus `F8` : estompe tout sauf la ligne ou le bloc courant. Machine à écrire `F9` : la ligne du curseur reste fixe à l'écran. Plein écran `F11`. Zoom `Ctrl+Shift+=` / `-`, `Ctrl+molette` (1.8). Numéros de ligne en option. Barre d'outils (1.14). Thèmes en CSS.
**Comptage.** Mots, caractères, lignes, temps de lecture, en bas de la fenêtre.
**Export.** PDF (signets, en-têtes et pieds avec `${pageNo}`, taille de page), HTML, image. Avec Pandoc installé à part : DOCX, ODT, LaTeX, EPUB, etc. Commandes shell personnalisées. « Exporter comme la dernière fois ».

**Ce que les personnes aiment** : l'écriture sans volet d'aperçu, le calme visuel, l'export, les thèmes.
**Ce qu'elles reprochent** : aucune collaboration, peu de réglages avancés, passage au payant, gestion des images.
**Défauts prouvés** : voir « Leçons pour N53 ».

## Classement pour inkwell

| Fonction de Typora | État dans inkwell | Classe | Décision |
|---|---|---|---|
| Rendu sur place, balisage caché | N53 (blocs) ; `editor-features.md` §3.6 | adapter | Blocs d'abord (tableaux), puis tâches, maths, images. Pas de masquage du balisage en ligne : voir N53. |
| Mode source `Ctrl+/` | N53 (à préciser) | adapter | Bascule **par personne**, locale, non synchronisée. `Ctrl+/` est déjà le « commenter » de CodeMirror (`defaultKeymap` dans `setup.ts:createEditor`) : choisir un autre raccourci. |
| Titres `Ctrl+1…`, paragraphe | fait : `format/keymap.ts:BINDINGS` (`Mod-1` à `Mod-4`, `Mod-Alt-0`) | — | Garder. |
| Niveau de titre `Ctrl+=` / `-` | nouveau | adopter | Hausser ou baisser le titre. Vérifier le conflit avec le zoom du texte (U27). |
| Gras, italique, souligné, barré, code, lien | fait : `format/keymap.ts:BINDINGS` | — | Raccourcis déjà là (code `Mod-e`, barré `Mod-Shift-x`). |
| Effacer le format `Ctrl+\` | nouveau | adopter | Retire les marques de la sélection, en Typst comme en Markdown. |
| Listes, retrait, conversion de liste | barre U4 ; conversion nouvelle | adopter | Raccourcis visibles dans la palette et le menu. |
| Déplacer ligne `Alt+↑/↓` | fait : `defaultKeymap` de CodeMirror (`setup.ts:createEditor`) | — | — |
| Tâches `- [ ]` cliquables | nouveau | adapter | Case à cocher en mode enhance (Markdown). Un clic écrit un seul caractère (`x`). |
| Tableaux : création, barre, `Ctrl+Entrée`, glisser, alignement | insertion faite : `format/insert.ts:insert` (`table`, `csvTable`) ; édition = N53 | adapter | Grille de N53 (`ui/sheet/Grid.svelte`). Mêmes gestes que le tableur (U21). Typst `#table` en plus du Markdown. |
| Bloc de code, langue, coloration | fait : U5, `editor/highlight.ts:languageExtension` | adopter | Ajouter le choix de langue en liste (enhance). |
| Maths `$$`, `$…$` | insertion faite : `insert.ts` (`formula`, `Mod-m`) | adapter | Rendu sur place par le Typst épinglé, pas MathJax : le rendu égale le PDF. |
| Diagrammes Mermaid | N24 (exemple de greffon) | rejeter du cœur | Greffon (N21). Typst a ses propres paquets fournis. |
| Note de bas de page, contenu au survol | insertion faite : `insert.ts` (`footnote`) | adopter | Infobulle au survol de l'appel. |
| Sommaire `[toc]` | Typst `#outline()` : `insert.ts` (`outline`) | adapter | Markdown : sommaire rendu par l'aperçu, pas de syntaxe propriétaire `[toc]`. |
| Bloc YAML en tête | nouveau | adapter | Markdown seulement. Sert à U18 (mise en page). En enhance : petit formulaire. |
| Émojis `:nom:` | `editor-features.md` §9.5 (sélecteur) | adapter | Dans le sélecteur de caractères unique, pas une 2ᵉ syntaxe. |
| `==surligné==`, `~indice~`, `^exposant^` | — | rejeter | Syntaxes non standard : le fichier ne s'affiche pas ailleurs. Typst a `#highlight`, `#sub`, `#super` (barre). |
| HTML en ligne, `<iframe>`, `<video>` | — | rejeter | Octets de pair : jamais de HTML brut rendu (invariant). |
| Alertes GitHub `> [!NOTE]` | nouveau | adapter | Rendu dans l'aperçu Markdown (U18), syntaxe standard GFM. |
| Paires automatiques | nouveau (pas de `closeBrackets` dans `setup.ts:createEditor`) | adopter | Crochets et guillemets. Paires `*` `_` désactivées par défaut. |
| Ponctuation typographique | `editor-features.md` §9.4 | adapter | Déjà conçue : rendu par Typst, saisie optionnelle en Markdown. |
| Coller du HTML → Markdown ; `Ctrl+Shift+V` brut | nouveau | adapter | Coller riche → Typst ou Markdown selon le fichier, nettoyé (liste blanche). Texte brut à `Ctrl+Shift+V`. |
| Copier comme Markdown | nouveau | adapter | « Copier comme texte mis en forme » (inverse) : le texte source est déjà ce que l'on copie. |
| Copier comme HTML | — | rejeter | L'export suffit ; zéro jargon. |
| Images : glisser, coller, dossier, chemins relatifs | insertion faite : `insert.ts` (`image`, `figure`, `relativePath`) ; coller/glisser nouveau | adapter | Coller ou glisser copie l'image **dans le projet** (magasin BLAKE3, ADR 0023) : tout le groupe la reçoit (N5). |
| Envoi d'images vers un serveur | — | rejeter | Réseau tiers, hors ligne, vie privée. |
| Supprimer l'image et son fichier | U13 (suppression sur accord) | adapter | Retirer le lien ne supprime jamais le fichier. La suppression passe par U13. |
| Plan `Ctrl+Shift+1` | U30 | adopter | Avec présence (I2 de `editor-features.md`). |
| Arborescence, filtres, clavier | U13 | adopter | — |
| Onglets, `Ctrl+Tab`, rouvrir fermé `Ctrl+Shift+T` | U13 (onglets) ; rouvrir nouveau | adopter | `Ctrl+Shift+T` rouvre le dernier onglet fermé. |
| Ouverture rapide `Ctrl+P` | `editor-features.md` §3.3 (palette) | adapter | Dans la palette unique. |
| Chercher, remplacer, dans le dossier | U14 ; `editor/search/` (`project.ts`, `replace.ts`) | — | Déjà plus riche (joker, regex, liste des lignes). |
| Mots, caractères, temps de lecture | fait : `editor/stats/words.ts:countWords`, `ui/popover/StatsPopover.svelte` ; temps de lecture : `editor-features.md` rec. 15 | adopter | Ajouter le temps de lecture. |
| Focus `F8`, machine à écrire `F9` | `editor-features.md` §9.2 (pas au registre) | adapter | Local. `F8` est déjà « problème suivant » (N28) : autre raccourci. |
| Plein écran `F11` | nouveau | adopter | Fenêtre gérée par l'application (N49). |
| Zoom | fait : U27 (zoom du texte) | — | — |
| Numéros de ligne en option | N52 | adopter | Masqués par défaut en mode enhance. |
| Thèmes CSS | N22 (thèmes), U18 (mise en page Markdown) | adapter | Séparer le thème de l'application (Adwaita) de la mise en page du document (projet, partagée). |
| Export PDF, en-têtes, pieds | fait : `preview/actions.ts:exportPdf` ; en-têtes = Typst | — | En-têtes et pieds : dans le document Typst, pas dans un réglage. |
| Export via Pandoc (DOCX, ODT…) | N40, N42 | rejeter | Logiciel à installer ; formats bureautiques en greffon seulement. |
| Commandes d'export en shell | — | rejeter | Exécution de commandes : risque de sécurité. |
| « Exporter comme la dernière fois » | `editor-features.md` §9.6 (préréglages) | adopter | Le dernier préréglage du projet, en un raccourci. |
| Barre d'outils | fait : U4 | — | — |

## Leçons pour N53

**Constats sur Typora (tickets).**
1. Le modèle hybride est incohérent : le balisage réapparaît pour certains éléments et pas pour d'autres. Le ticket #1317 demande un seul interrupteur « Afficher le balisage sous le curseur ».
2. Le balisage caché fausse le curseur. Les `*` masqués changent le retour à la ligne : `Fin` place le curseur au mauvais endroit (#5749). Même chose avec les maths (#5308, #4642), les entités HTML (#546) et les listes numérotées (#6568, mai 2026, v1.13.6).
3. L'annulation perd du texte. Annuler puis rétablir perd une partie du contenu (#190). Le texte coupé sort de l'historique (#517). Une section perdue, revenue seulement par « Rétablir » (#1213). Annuler juste après un enregistrement en mode source (#3125).
4. Réduire un tableau supprime sans prévenir les cellules non vides (#1170).
5. Cause commune : Typora édite un modèle DOM puis le resérialise. Texte affiché et texte source divergent.

**Ce qu'inkwell doit faire.**
- **Le texte reste la seule vérité.** Le mode enhance est une décoration CodeMirror par-dessus le texte Automerge. Aucun modèle parallèle, aucune resérialisation du bloc (S13-spec, N53). Chaque position du curseur est une position dans le texte.
- **Ne cacher que des blocs entiers.** Le widget remplace le bloc quand le curseur est dehors. Pas de masquage de `*` ou `_` en ligne : c'est la source des bugs 2. Au besoin, atténuer le balisage en ligne (couleur discrète), sans changer sa largeur.
- **Entrer et sortir de façon explicite et cohérente.** Une règle pour tous les blocs : flèches ou clic entrent dans le widget, `Échap` revient au texte du bloc. Un interrupteur par personne « Afficher le balisage » (leçon #1317), avec raccourci visible.
- **Annuler = l'historique de CodeMirror.** Chaque geste de la grille est une transaction avec un `userEvent` nommé. L'annulation reste locale à la personne (`editor/localHistory.ts:localHistoryOnly`) : jamais l'édition d'un pair.
- **Jamais de perte silencieuse.** Supprimer une ligne ou une colonne non vide demande un geste explicite et reste annulable. Réduire la taille ne supprime jamais de contenu (leçon #1170).
- **Concurrence.** Une modification de pair dans le bloc ouvert fait reparser le bloc. La cellule active est gardée par un curseur Automerge, pas par un indice. Si la structure devient illisible (balisage cassé), le bloc retombe en texte brut avec un bandeau, sans rien réécrire.
- **Typst n'est pas du Markdown.** Une cellule `#table` peut contenir du code. Une cellule complexe s'affiche en lecture dans la grille et s'ouvre en texte. Pas de WYSIWYG Typst complet (décision de `editor-features.md` §3.6).
- **Tests exigés au lot de code.** Propriété « aller-retour » : tout geste de grille produit le même texte qu'une frappe équivalente. Curseur : `Fin`, `Début` et les flèches donnent la position attendue autour de chaque widget. Fusion concurrente de deux cellules (déjà dans S13-spec).

## Au-delà de Typora

1. **Tableau vivant relié à un CSV.** `#table(..csv("x.csv"))` (inséré par `insert.ts`) s'édite dans la grille ; l'écriture va au fichier CSV, seule vérité. Le document et le tableur montrent la même donnée, à plusieurs.
2. **Présence dans les widgets.** Pastille de la personne sur la cellule, la case ou la formule qu'elle modifie. On voit avant de toucher la même cellule. Base : `editor/remoteCursors.ts`, `editor/presence.ts`.
3. **Chacun son mode sur le même texte.** Une personne écrit en enhance, une autre voit le balisage. Rien de cela n'est synchronisé : seul le texte l'est. Typora ne le permet pas (un réglage global).
4. **Formules et figures rendues par le vrai moteur.** Les widgets de maths utilisent le Typst épinglé (aperçu SVG déjà en place, `preview/svg.ts:pageImage`). Le rendu sur place égale le PDF, sans écart MathJax / export.
5. **Images partagées par contenu.** Coller une image l'ajoute au magasin BLAKE3 du projet : dédoublonnée, vérifiée, envoyée à tous les pairs, historisée. Aucun dossier `assets` à gérer, aucun lien cassé chez une collègue.

## Recommandation : à ajouter au registre

Lignes proposées (non écrites dans `docs/exigences.md`) :

| # | Demande proposée | Où |
|---|---|---|
| N59 | Paires automatiques (crochets, guillemets) ; paires `*` `_` en option, désactivées par défaut | `frontend/src/editor/setup.ts` |
| N60 | Coller du contenu mis en forme le convertit en Typst ou Markdown nettoyé ; `Ctrl+Shift+V` colle en texte brut | `frontend/src/editor/` |
| N61 | Coller ou glisser une image la copie dans le projet (magasin BLAKE3) et insère le lien relatif ; jamais d'envoi en ligne | `frontend/src/editor/format/insert.ts`, `internal/blob` |
| N62 | Mode focus, machine à écrire et plein écran, locaux, avec des raccourcis libres (pas `F8`, pris par N28) | `docs/design/` |
| N63 | Précision de N53 : interrupteur par personne « Afficher le balisage », blocs seulement, `Échap` pour revenir au texte, aucune perte silencieuse, annulation locale ; raccourci autre que `Ctrl+/` | `docs/design/enhance-mode.md` |
| N64 | Petites aides d'écriture : effacer le format (`Ctrl+\`), hausser ou baisser un titre, tâches cliquables, note au survol, rouvrir l'onglet fermé (`Ctrl+Shift+T`), temps de lecture | `frontend/src/editor/format/` |

## Sources

Consultées le 2026-11-07.
- Site : https://typora.io/
- Raccourcis : https://support.typora.io/Shortcut-Keys/
- Référence Markdown : https://support.typora.io/Markdown-Reference/
- Tableaux : https://support.typora.io/Table-Editing/
- Copier-coller : https://support.typora.io/Copy-and-Paste/
- Images : https://support.typora.io/Images/
- Export : https://support.typora.io/Export/
- Diagrammes : https://support.typora.io/Draw-Diagrams-With-Markdown/
- Focus et machine à écrire : https://support.typora.io/Focus-and-Typewriter-Mode
- Notes de version : https://typora.io/releases/stable
- Tickets : [#1317](https://github.com/typora/typora-issues/issues/1317), [#5749](https://github.com/typora/typora-issues/issues/5749), [#5308](https://github.com/typora/typora-issues/issues/5308), [#4642](https://github.com/typora/typora-issues/issues/4642), [#546](https://github.com/typora/typora-issues/issues/546), [#6568](https://github.com/typora/typora-issues/issues/6568), [#190](https://github.com/typora/typora-issues/issues/190), [#517](https://github.com/typora/typora-issues/issues/517), [#1213](https://github.com/typora/typora-issues/issues/1213), [#3125](https://github.com/typora/typora-issues/issues/3125), [#1170](https://github.com/typora/typora-issues/issues/1170)
- Avis : https://www.g2.com/products/typora/reviews, https://toolradar.com/tools/typora, https://us.fitgap.com/products/typora, https://nimbalyst.com/blog/typora-alternatives-2026/

> **Inscrit au registre le 2026-11-07** (accord de l’utilisatrice), après fusion des doublons des deux études : N60 paires, N61 coller propre, N62 images, N63 focus, N64 précision de N53, N65 petites aides, N66 attribution, N67 coller les modifications, N68 colonne et police, N69 relecture du style, N70 relevé de provenance. Les numéros provisoires ci-dessus ne valent plus.
