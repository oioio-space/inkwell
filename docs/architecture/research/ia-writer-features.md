# Étude : les fonctions d'iA Writer, classées pour inkwell (N59, N53)

Date : 2026-11-07. Exigences visées : N59, N53 (et N31, N44, N55, U12, U13, U15, U18, U30, N27).
À lire avec `typora-features.md`, `editor-features.md` (§9), `docs/design/S13-spec.md` et `preview-view-modes.md`.

## Méthode

- Sources primaires d'abord : ia.net/writer, pages d'aide (support), billets (topics), historique des versions. Liens en fin de document.
- Version de référence : iA Writer 8 (juin 2026). Prix : achat unique, environ 50 $ sur Mac, 30 $ sur Windows, 20 $ sur iOS (sources secondaires).
- Raccourcis : variante macOS (`Cmd`). La page Windows ne donne souvent aucun raccourci.
- Avis 2024-2026 : minces (sspai, rywalker, codeless, appgefahren). Aucun fil HN ou Reddit récent trouvé.
- Équivalent Typora : repris de `typora-features.md` (raccourcis Windows/Linux).
- État dans inkwell : vérifié dans le graphe gortex (`fichier:symbole`) ou dans le registre. Aucun build ni test lancé.
- Classes : **adopter**, **adapter** (dire comment), **rejeter** (dire pourquoi).
- Ce qui n'a pas pu être vérifié est marqué **[à vérifier]**. Plusieurs pages d'aide renvoyaient une erreur 404 (plan, blocs de contenu, navigation).

## Fonctions d'iA Writer

**Mode focus.** `Cmd+D`. Seule la phrase (ou le paragraphe) en cours est en pleine couleur, le reste est estompé. Le mode machine à écrire garde la ligne au centre, sans estomper. iA conseille de couper le focus pendant la relecture.
**Coloration des natures de mots.** `Shift+Cmd+D`. Adjectifs, noms, adverbes, verbes et conjonctions, chacun dans sa couleur. Langues : en, de, fr, es, it, ru. Pour relire, pas pour écrire.
**Vérification du style.** `Opt+Shift+Cmd+D`. Mots de remplissage, clichés et redondances barrés à la frappe, visibles par soi seul. Calcul sur l'appareil, sans IA. Langues : en, fr, es, de. On peut ajouter ses propres motifs (texte, exception avec `-`, `/regex/`).
**Blocs de contenu (inclusion).** Un chemin seul sur une ligne (`/chapitres/intro.md`), avec une légende optionnelle. Le fichier est inclus : texte, code, image, ou CSV rendu en tableau. Visible dans l'aperçu et dans les exports.
**Modèles d'aperçu et d'export.** Modèles en HTML, CSS et JS. Le choix se fait dans la barre de l'aperçu. Un bloc `<style>` dans le document est permis. Export HTML, PDF, Word, Markdown.
**Bibliothèque.** Dossiers, emplacements externes, favoris, étiquettes `#mot`, dossiers intelligents (règles sur la recherche, le chemin, la date ; « Récents » par défaut). Liens `[[wiki]]` avec complétion (v6). Plan fusionné dans la recherche rapide et palette de commandes (v8).
**Typographie.** Polices maison Mono, Duospace et Quattro, dérivées d'IBM Plex Mono. Duospace donne plus de place aux lettres larges (m, w). Lignes d'environ 64, 72 ou 80 signes selon la police. L'interligne grandit avec la colonne.
**Mode nuit.** Thème sombre. Pas de page dédiée **[à vérifier : raccourci]**.
**Attribution (« Authorship », v7, 2023).** Le texte venu d'une IA est grisé (dégradé de couleur en v7.3). Chaque personne a sa teinte. Le marquage est **manuel** : « Coller comme », « Marquer comme », « Coller les modifications de » (calcule la différence entre le texte collé et la sélection). Une conversation ChatGPT collée est reconnue. Stockage : un bloc en fin de fichier, caché dans iA Writer, visible dans les autres éditeurs (format ouvert « Markdown Annotations »). Les exports le retirent. Une modification hors d'iA Writer peut décaler les marques : l'application prévient.
**Publication.** WordPress, Ghost, Micro.blog, Micropub (Mac, iOS). Medium seulement pour les anciens comptes. Absente sous Windows **[à vérifier]**.
**Aperçu.** Volet à côté ou en plein écran (Mac : menu Affichage > Aperçu). Défilement synchronisé, désactivable.
**Balisage.** Visible mais discret (gris). Aucun masquage, aucun rendu sur place **[à vérifier : confirmé par des demandes du forum, sans page officielle]**.
**Ce que les personnes aiment** : le calme, le focus, la typographie, l'achat unique, la publication.
**Ce qu'elles reprochent** : aucune collaboration, pas de rendu sur place, Windows et Android en retard, marquage de l'IA manuel, intégration Medium fermée.

## Classement pour inkwell

| Fonction d'iA Writer | Typora (comportement, raccourci) | État dans inkwell | Classe | Décision |
|---|---|---|---|---|
| Focus phrase / paragraphe | Focus `F8` : ligne ou bloc | `editor-features.md` §9.2 ; proposé par Typora (non écrit) | adapter | Local, jamais synchronisé. Phrase ou paragraphe. Pas `F8` (pris par N28). Coupé automatiquement à l'ouverture de l'historique. |
| Machine à écrire | `F9` | idem | adopter | Ligne fixe au centre ; respecte `prefers-reduced-motion`. |
| Natures de mots en couleur | aucun | nouveau | rejeter du cœur | Analyseur par langue, lourd. Couleurs en conflit avec la coloration Typst et les couleurs des personnes. Greffon possible (N21). |
| Vérification du style | aucun | `editor-features.md` §9.1, rec. 14 (onglet « Relecture ») | adapter | À la demande dans « Relecture », jamais à la frappe. Prose seule (pas le code Typst). Motifs personnels **et du projet** (partagés). Résultat visible par soi seul. |
| Motifs personnalisés (`/regex/`) | aucun | nouveau | adapter | Liste de mots simple. Regex derrière « Options avancées » (comme la recherche, U14). |
| Blocs de contenu | aucun (liens, HTML `<img>`) | U15 ; `format/insert.ts:renderTypst` (`csvTable`) | adapter | Pas de syntaxe privée. Typst : `#include`, `#image`, `csv()`. Markdown : lien standard ; l'aperçu peut inclure un `.md` du projet. Le fichier inclus vient du projet (N5). |
| CSV rendu en tableau | aucun | fait : `insert.ts` (`csvTable`) | — | Déjà là. Édition en grille : U19, N53. |
| Modèles HTML/CSS/JS | thèmes CSS | N27 (modèles), U18 (mise en page Markdown) | adapter | Modèle = projet partagé (N27). **Pas de JS** : octets de pair, jamais exécutés (invariant). CSS seul en Markdown. |
| Export HTML, PDF | PDF, HTML | fait : `preview/actions.ts:exportPdf` | — | — |
| Export Word | via Pandoc | N40 | rejeter | Décidé : greffon seulement. |
| Dossiers, emplacements | arborescence `Ctrl+Shift+3` | U13 | adopter | — |
| Favoris | aucun | nouveau | rejeter (pour l'instant) | Les onglets (U13) et la recherche rapide suffisent. |
| Étiquettes `#mot` | aucun | nouveau | rejeter | `#` est la syntaxe de code Typst. Ambigu et source d'erreurs. |
| Dossiers intelligents | aucun | nouveau | rejeter | Hors du rôle d'un projet de document. La recherche de projet (U14) couvre le besoin. |
| Liens `[[wiki]]` | liens vers un titre | U15 | adapter | `[[` ouvre la complétion, insère un lien **standard** (`@label` Typst, lien relatif Markdown). Jamais de syntaxe `[[ ]]` dans le fichier. |
| Recherche rapide + plan + palette | `Ctrl+P` ; plan `Ctrl+Shift+1` | U30 ; `editor-features.md` §3.3 | adopter | Une seule palette : fichiers, titres, commandes. |
| Polices Duospace / Quattro | thèmes | nouveau | adapter | Police de l'éditeur réglable, locale. Embarquer une police à chasse duale si sa licence le permet (iA-Fonts, OFL **[à vérifier]**). N'affecte jamais le document (Typst a ses polices, N43). |
| Longueur de ligne (~64-80 signes) | largeur de thème | nouveau | adopter | Colonne de prose limitée (~70 signes) en Markdown et txt, centrée. Réglage local. Code et tableaux gardent la pleine largeur. |
| Mode nuit | thème « Night » | fait : `theme/tokens.css` (`prefers-color-scheme`, `data-theme`) | — | — |
| Attribution des personnes | aucun | fait : `editor/blameMarks.ts:blameMarks`, `history/track.ts:blameOf` (U12) | adapter | Déjà dérivée de l'historique, jamais saisie. Voir les leçons. |
| Marquage du texte d'IA | aucun | N31 (IA « first class ») | adapter | Automatique : l'IA (MCP, N31) écrit sous **son propre acteur**, signé. Pas de marquage manuel. |
| « Coller comme » (référence, IA) | aucun | nouveau | adapter | Un seul geste : « Coller comme citation » (bloc de citation). Pas de catégorie déclarée à la main. |
| « Coller les modifications de » | aucun | nouveau | adapter | Diff mot à mot du texte collé contre la sélection, appliqué en **modification minimale** : l'attribution des mots inchangés est gardée. Utile pour une relecture faite hors d'inkwell. |
| Stockage en fin de fichier | — | — | rejeter | Casserait la compilation Typst et doublerait la vérité. L'attribution reste dans l'historique Automerge. |
| Publication de blog | aucun | nouveau | rejeter | Réseau, comptes, jetons. Greffon possible (N21, N22). |
| Aperçu à côté, synchronisé | aucun (rendu sur place ; source `Ctrl+/`) | fait : `preview/Preview.svelte`, `preview/sync.ts:sourceBlocks` ; N44, N55 | — | Déjà plus riche (modes N54, « Suivre le curseur »). |
| Balisage gris, jamais caché | balisage caché hors du curseur | N53 (à préciser) | adopter | Atténuer le balisage en ligne (couleur seule, largeur inchangée). Voir les leçons. |
| Compte de mots | barre d'état | fait : `editor/stats/words.ts:countWords` | — | Temps de lecture : rec. 15. |

## Leçons pour N53 et l'attribution

**Deux écoles.**
- Typora cache le balisage et rend sur place. Prix payé : curseur faux, annulation qui perd du texte (tickets cités dans `typora-features.md`).
- iA Writer garde tout le balisage, en gris, et montre le rendu à part. Prix payé : pas d'édition facile des tableaux, ce que les avis reprochent.
- Leçon : le texte affiché doit rester le texte source, caractère pour caractère. iA n'a aucun bug de curseur parce qu'il ne cache rien.

**Pour N53 (enhance).**
- **En ligne : l'école iA.** Le balisage en ligne (`*`, `_`, `` ` ``, `#`, `=`) reste visible, atténué par la couleur. Aucun changement de largeur. Le curseur ne saute jamais.
- **Blocs : l'école Typora, en mieux.** Seuls les blocs entiers (tableau, puis tâches, maths, images) deviennent des widgets quand le curseur est dehors. Grille de N53, `Échap` revient au texte.
- **L'aperçu reste à côté** (comme iA) : enhance ne remplace pas l'aperçu Typst, il facilite la saisie.
- **Focus et enhance sont des décorations CodeMirror locales.** Ils ne touchent pas au texte Automerge. Chaque personne règle les siens.
- Le focus estompe ; il ne cache rien. Les curseurs des autres restent visibles dans la partie estompée.

**Pour l'attribution.**
- iA prouve le besoin : savoir qui (ou quelle IA) a écrit quoi. Il le fait mal pour inkwell : marquage manuel, bloc dans le fichier, marques décalées par une édition externe.
- inkwell a déjà la bonne base : chaque modification Automerge porte un acteur, et « Qui a écrit quoi » en est une projection (`blameOf`). Aucune donnée en plus dans le texte.
- L'IA (N31) doit écrire sous un acteur distinct, rattaché à la personne qui l'a lancée. Le texte d'IA est alors marqué sans geste, et on ne peut pas le maquiller : les octets sont signés et vérifiés (invariant).
- Une modification externe (U7) est attribuée à « modifié hors d'inkwell », pas à une personne.
- Calme par défaut : l'attribution est un affichage à la demande (déjà le cas, U12), pas des couleurs permanentes. iA grise l'IA plutôt que de la colorer : reprendre l'idée.
- Vie privée : le groupe entier voit l'attribution. Le dire dans l'interface, sans jargon (« Tout le groupe voit qui a écrit chaque passage »).
- Copier-coller entre personnes : coller garde l'auteur du collage. « Coller les modifications de » garde l'auteur des mots inchangés.

## Au-delà d'iA Writer

1. **Attribution certifiée, sans geste.** Dérivée des modifications signées : personnes, IA, éditions externes. Rien à marquer, rien à falsifier. Export d'un **relevé de provenance** (PDF Typst : part de chaque personne et de l'IA, par section). Utile pour une déclaration d'usage d'IA (études, édition).
2. **Focus partagé, jamais imposé.** En mode « suivre » (N45), mon focus suit la phrase de la personne suivie. Sinon, une pastille discrète dans la marge montre où chacune travaille, sans casser mon calme.
3. **Relecture de groupe.** Les motifs de style et le dictionnaire appartiennent au projet. Un résultat de relecture peut devenir un commentaire adressé à une personne (W3 d'`editor-features.md`), visible par elle seule jusqu'à son envoi.
4. **Inclusion vivante et vérifiée.** Un bloc inclus (texte, CSV, image) vient du projet et du magasin BLAKE3. Tout le groupe voit le même contenu. Lien absent signalé (U17). L'aperçu le rend avec le vrai Typst, identique au PDF.
5. **Longueur de ligne tirée du document.** En Typst, la colonne de l'éditeur peut suivre la mesure de la page (largeur du texte en signes). On écrit avec le même rythme que le lecteur lira.

## Recommandation : à ajouter au registre

Lignes proposées (non écrites dans `docs/exigences.md`). N59 est déjà pris par cette étude ; les lignes proposées par `typora-features.md` (N59 à N64) sont à renuméroter avant d'être écrites. Numéros provisoires ci-dessous :

| # | Demande proposée | Où |
|---|---|---|
| N65 | Mode focus (phrase ou paragraphe) et machine à écrire : locaux, estompent sans cacher, curseurs des autres visibles, raccourcis libres (pas `F8`) ; fusion avec la ligne N62 de Typora | `docs/design/` |
| N66 | Précision de N53 : en ligne, balisage visible et atténué (couleur seule, largeur inchangée) ; widgets seulement pour des blocs entiers | `docs/design/enhance-mode.md` |
| N67 | Attribution sans geste : l'IA (N31) écrit sous son propre acteur signé ; éditions externes attribuées à « hors d'inkwell » ; affichage à la demande, IA grisée ; mention visible que le groupe voit l'attribution | `frontend/src/history/`, `docs/architecture/` |
| N68 | « Coller les modifications de » : diff mot à mot appliqué en modification minimale, attribution des mots inchangés gardée | `frontend/src/editor/` |
| N69 | Colonne de prose limitée (~70 signes, réglable, locale) et police de l'éditeur réglable, à chasse duale embarquée si la licence le permet | `frontend/src/editor/setup.ts`, `theme/` |
| N70 | Relecture du style à la demande (remplissage, clichés, redondances) avec motifs personnels et du projet ; prose seule | `docs/design/` (onglet « Relecture ») |
| N71 | Relevé de provenance exportable (part de chaque personne et de l'IA, par section) | `docs/design/`, `preview/` |

## Sources

Consultées le 2026-11-07.
- Site : https://ia.net/writer
- Focus : https://ia.net/writer/support/editor/focus-mode ; Windows : https://ia.net/writer/support/editor/focus-mode/focus-mode-windows
- Natures de mots : https://ia.net/writer/support/editor/syntax-highlight
- Style : https://ia.net/writer/support/editor/style-check
- Blocs de contenu : https://ia.net/?p=19258 ; https://ia.net/topics/ia-writer-4
- Modèles : https://ia.net/writer/support/preview/templates
- Bibliothèque : https://ia.net/writer/support/library ; https://ia.net/writer/support/library/organize
- Typographie : https://ia.net/topics/in-search-of-the-perfect-writing-font ; https://ia.net/?p=4339 (extraits de recherche seulement)
- Attribution : https://ia.net/topics/ia-writer-7 ; https://ia.net/writer/support/editor/authorship?tab=iphone ; https://ia.net/writer/how-to/track-authors-and-ai
- Publication : https://ia.net/writer/support/preview/blog ; https://ia.net/writer/support/preview/blog/publish-to-blogging-platforms-mac
- Aperçu et réglages : https://ia.net/writer/support/basics/settings/settings-ipad
- Versions : https://ia.net/writer/support/help/version-history ; https://alternativeto.net/news/2026/6/ia-writer-8-merges-search-and-outline-with-new-liquid-glass-design/
- Avis : https://sspai.com/post/112787 ; https://sspai.com/post/59460 (ancien) ; https://rywalker.com/research/ia-writer ; https://codeless.io/best-writing-apps/ia-writer/ ; https://www.appgefahren.de/ia-writer-8-liquid-glass-design-und-anpassungen-fuer-apples-neueste-systeme-401141.html ; https://9to5mac.com/guides/ia-writer/

> **Inscrit au registre le 2026-11-07** (accord de l’utilisatrice), après fusion des doublons des deux études : N60 paires, N61 coller propre, N62 images, N63 focus, N64 précision de N53, N65 petites aides, N66 attribution, N67 coller les modifications, N68 colonne et police, N69 relecture du style, N70 relevé de provenance. Les numéros provisoires ci-dessus ne valent plus.
