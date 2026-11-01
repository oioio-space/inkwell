# Étude : fonctions d'édition à reprendre de tinymist et des autres outils

Date : 2026-11-01. Exigences visées : N20, N27, N28, N29 (et N21 pour les greffons).
Public : personnes qui écrivent, pas des développeuses (`docs/ux-principles.md`, `docs/glossary.md`).
Sources consultées le 2026-11-01 ; ce qui n'a pas pu être vérifié est marqué **[à vérifier]**.

## 1. Méthode

Grille fixe, appliquée à chaque fonction :

| Critère | Échelle |
|---|---|
| **Valeur** pour une personne non développeuse | 1 (marginal) à 5 (essentiel) |
| **Coût** d'intégration | S (jours), M (1-3 semaines), L (au-delà) |
| **CRDT / collaboration** | OK (sans effet), + (la collaboration l'enrichit), ! (exige un traitement : le texte vit dans Automerge côté webview, tinymist ne voit que des octets relayés) |
| **Perf** | budget p95 ≤ 100 ms pour tout ce qui est à la frappe (spec §1441) |
| **Hors ligne** | invariant INV-6 : aucun téléchargement à l'exécution |

Classes : **REUSE** (tinymist le fournit, on câble), **ADAPT** (même moteur, présentation pour non-développeuses), **PLUGIN** (N21, optionnel), **REJECT**.

## 2. Faits établis sur l'existant

**tinymist** (dernière version vue : v0.15.10, 2026-10-16 ; v0.15.4 du 2026-07-26 = « Typst Preview NG », aperçu HTML, docs de paquets via LSIF/SCIP) :
[README](https://github.com/Myriad-Dreamin/tinymist), [fonctions de langage](https://myriad-dreamin.github.io/tinymist/feature/language.html), [CHANGELOG extension](https://raw.githubusercontent.com/Myriad-Dreamin/tinymist/main/editors/vscode/CHANGELOG.md).

- Service de langage : complétion, aide de signature, survol (docs au style *tidy*), définition/références, renommage (symboles **et chemins** dans les chaînes), symboles de document (plan) et d'espace de travail, repliement, jetons sémantiques, indications en ligne (désactivées par défaut pour cause de perf), sélecteur de couleur, liens de document (chemins d'images/bibliographies), *code lens*, actions de code (hausser/baisser les titres, équation en ligne ↔ bloc), formatage (typstyle ; v0.15.10 expose `sentence` pour le retour à la ligne de prose), `onEnter` expérimental (continue `///`).
- Aperçu : SVG incrémental, **saut croisé** clic aperçu → source et inverse ([page aperçu](https://myriad-dreamin.github.io/tinymist/feature/preview.html)) ; remplace `typst-preview` (archivé 2024-11-07, [dépôt](https://github.com/Enter-tainer/typst-preview)) et `typst-lsp` (archivé 2024-11-05, [dépôt](https://github.com/nvarner/typst-lsp)).
- Export PDF (au enregistrement ou à la frappe), SVG, PNG, HTML, Markdown, texte ; lint intégré (source `tinymist-lint`, évite les doublons avec les erreurs du compilateur, hints en « Hint: » ; liste des règles non documentée [source](https://docs.rs/crate/tinymist-query/latest/source/src/diagnostics.rs)) ; profilage ; nombre de mots (barre d'état) ; galerie de modèles, vues Symboles / Étiquettes / Paquets ([extension VS Code](https://myriad-dreamin.github.io/tinymist/frontend/vscode.html)).
- Docstrings : `///` en syntaxe **Typst** (pas Markdown), `-> type`, `#example` ([conventions](https://myriad-dreamin.github.io/tinymist/feature/docs.html)). Non officiel.
- Mode `tinymist.syntaxOnly` : pas de compilation, hover/définition/formatage syntaxiques restent ([Zed](https://myriad-dreamin.github.io/tinymist/frontend/zed.html)) : utile pour gros projets.
- Complétion des étiquettes et clés bibliographiques : vue dans les tests du dépôt ([completion.rs](https://github.com/Myriad-Dreamin/tinymist/blob/main/crates/tinymist-query/src/completion.rs)), pas dans la doc de fonctions **[à vérifier au banc d'essai]**.
- **Hors ligne** : `@preview` est téléchargé à la demande puis mis en cache ; paquets locaux dans `{data-dir}/typst/packages/{ns}/{nom}/{version}` ([typst/packages](https://github.com/typst/packages)). La doc tinymist ne traite pas le hors ligne. Les options de chemin de paquets/cache (`--package-path`, `--package-cache-path` côté Typst) existent **[à vérifier côté tinymist]** ; ADR 0024 reste la règle : paquets fournis ou `.typstdeps/`.
- Typst 0.15.0 (2026-06-15) : export HTML + MathML, cible expérimentale *bundle*, bibliographies multiples, meilleurs messages d'erreur ([changelog](https://typst.app/docs/changelog/0.15.0/)).

**typst.app** : collaboration gratuite ; commentaires (liés à une plage de texte, fils, résolution, archivage, pas dans le PDF) réservés à Pro ; liens de relecture seule (2025-09-26) ; modèles privés Pro ([commentaires](https://typst.app/docs/web-app/comments/), [blog](https://typst.app/blog/2025/folders-reviews), [prix](https://typst.app/pricing/)). La communauté trouve les commentaires payants discutable ([forum](https://forum.typst.app/t/why-arent-web-app-comments-a-free-feature/5672)). Typst Universe : catégories Book, Report, Paper, Thesis, Poster, Flyer, Presentation, CV, Office ([Universe](https://typst.app/universe/search/?kind=templates)).

**Zed** : extension Typst 0.2.1 basée sur tinymist, **pas d'aperçu intégré** (RFC [#51633](https://github.com/zed-industries/zed/discussions/51633)). **Neovim** : service de langage + typst-preview.nvim qui **télécharge son propre binaire** (contraire à INV-6) ; export hors PDF non supporté ([README](https://github.com/Myriad-Dreamin/tinymist/blob/main/editors/neovim/README.md)). Leçon : le service de langage seul est portable, l'aperçu et les modèles sont du travail d'application, c'est ce qu'inkwell doit faire.

**Outils d'écriture** : Overleaf (suivi des modifications = premium, commentaires gratuits : [docs](https://docs.overleaf.com/collaborating/track-changes)) ; Google Docs, mode Suggestion avec accepter/rejeter un à un ou tout ([aide](https://support.google.com/docs/answer/6033474)) ; Quarto, éditeur visuel ↔ source avec position et annulation conservées ([doc](https://quarto.org/docs/visual-editor/)) ; Typora, rendu sans mode ni symboles, plan, mode focus/machine à écrire, nombre de mots ([site](https://typora.io/)) ; iA Writer, focus phrase/paragraphe/machine à écrire ([aide](https://ia.net/writer/support/editor/focus-mode)) ; Obsidian, plan cliquable et déplaçable par glisser ([aide](https://obsidian.md/help/plugins/outline)) ; VS Code, aperçu de définition, fil d'Ariane, ampoule de correction, panneau Problèmes (Ctrl+Maj+M), F8/Alt+F8 pour parcourir ([doc](https://code.visualstudio.com/docs/editing/editingevolved)). Non vérifiés faute de page lisible : LyX, Zettlr, Overleaf (journal de compilation), VS Code *sticky scroll*, rétro-liens Obsidian ; classés sur connaissance générale, signalés « (g) ».

## 3. Classification

Note : V valeur, C coût, Co collaboration, P perf, H hors ligne (OK = aucun réseau).

### 3.1 Aide à la saisie Typst (N29)

| Fonction | Source | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Complétion (fonctions, paramètres, chemins) | tinymist | REUSE | 5 | M | ! | p95 | OK | Cœur de N29. Le pont LSP reçoit des deltas du document Automerge de la webview (`didChange` incrémental) ; le texte vient de la webview, pas du disque. Libellés en français simple, extrait de doc dans la ligne. |
| Aide de signature (argument courant en gras) | tinymist | REUSE | 5 | S | OK | OK | OK | Infobulle ancrée ; description en phrase, pas le type brut. |
| Survol : doc, type | tinymist | ADAPT | 5 | M | OK | OK | OK | Masquer les types derrière « Détails techniques » (glossaire). Rendre le tidy en HTML sûr (pas de HTML brut venant d'un pair : un docstring d'un fichier partagé est une entrée d'un pair). |
| Docstrings `///` des fonctions de la personne | tinymist | ADAPT | 5 | M | + | OK | OK | Cible directe de N29 (charte importée). Assistant « Documenter cette fonction » qui insère le squelette `///` ; la doc d'une charte voyage avec le projet (CRDT) et profite à toutes les personnes. |
| Complétion des étiquettes `<l>` / `@l` et clés bibliographiques | tinymist | REUSE | 5 | S | + | OK | OK | Voir idée I1 (étiquettes des collaborateurs). Vérifier au banc d'essai. |
| Complétion d'import de paquet | tinymist | ADAPT | 3 | M | OK | OK | ! | Limiter à l'index **embarqué** + `.typstdeps/` ; un paquet non présent = message « non fourni avec l'application », jamais de téléchargement. |
| Extraits (snippets) de structure (tableau, figure, encadré) | tinymist / Typst | ADAPT | 4 | S | OK | OK | OK | Alimenter aussi la barre de formatage (N20). |
| Sélecteur de couleur | tinymist | REUSE | 3 | S | OK | OK | OK | Utile pour les chartes ; pastille en ligne. |
| Vue Symboles (insérer ∑, flèches, par catégorie, recherche) | tinymist | ADAPT | 4 | M | OK | OK | OK | Excellent pour non-développeuses ; tiroir ou palette. |
| Indications en ligne (noms de paramètres) | tinymist | REJECT (par défaut) | 2 | S | OK | coûteux | OK | Désactivées par défaut dans tinymist pour la perf ; bruit visuel. Réexaminer en option. |
| `onEnter` (continuer `///`) | tinymist | REJECT | 1 | S | OK | OK | OK | Trop spécialisé. |

### 3.2 Erreurs et corrections (N28)

| Fonction | Source | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Diagnostics à la frappe (soulignement) | tinymist | REUSE | 5 | S | ! | p95 | OK | Chaque diagnostic porte un **identifiant de plage ancré** (curseur relatif Automerge) pour survivre aux modifications concurrentes avant l'actualisation. |
| Panneau Problèmes unique (Typst, Markdown, CSS) | VS Code | ADAPT | 5 | M | + | OK | OK | Un panneau « Problèmes à corriger », phrase + action (règle UX 5). Liste groupée par fichier ; clic = saut + sélection de l'erreur. Compteur dans l'en-tête. |
| Prochain / précédent problème (F8 / Maj+F8) | VS Code | REUSE | 5 | S | OK | OK | OK | Exigé par N28. Visible dans la palette avec son raccourci (règle 3). |
| Correction rapide (ampoule) | tinymist / VS Code | ADAPT | 5 | M | OK | OK | OK | Les « Hint: » de Typst deviennent un bouton d'action quand ils sont mécaniques (ex. `@preview/x` manquant → « Ouvrir les paquets fournis »). Sinon : texte d'aide seul. |
| Traduction/reformulation des messages Typst | (nouveau) | ADAPT | 5 | M | OK | OK | OK | Table de motifs (message compilateur → phrase française + action). Pas de modèle de langage : déterministe, testable. Le texte brut reste sous « Détails techniques ». |
| Lint tinymist | tinymist | ADAPT | 3 | S | OK | OK | OK | Afficher comme « Suggestions » (niveau Hint), jamais comme erreur ; les règles ne sont pas documentées **[à vérifier]**. |
| Erreurs dans un fichier importé non ouvert | Overleaf (g) | ADAPT | 5 | S | + | OK | OK | Le clic ouvre l'onglet du fichier au bon endroit (corrige aussi le bug N19). |
| Pile d'appels de l'erreur (trace) | Typst | ADAPT | 3 | S | OK | OK | OK | Repliée par défaut ; utile pour les charte/fonctions imbriquées. |
| Aperçu de définition (peek) | VS Code | ADAPT | 3 | M | OK | OK | OK | Pour « d'où vient ce style ? » : fenêtre flottante en lecture. |

### 3.3 Navigation

| Fonction | Source | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Plan (titres) cliquable | tinymist, Obsidian, Typora | REUSE | 5 | S | + | OK | OK | Voir I2 (présence dans le plan). Glisser un titre déplace la section (Obsidian) : opération CRDT couper/coller, risqué en concurrence → coût M, à faire en v2 avec « Annuler ». |
| Fil d'Ariane (fichier > titre > fonction) | VS Code | ADAPT | 3 | S | OK | OK | OK | Léger ; réutilise les symboles. |
| Aller à la définition / références | tinymist | ADAPT | 4 | S | OK | OK | OK | Libellé « Aller à l'origine » / « Où est-ce utilisé ? » (ex. un style de la charte). |
| Renommer (symboles + chemins) | tinymist | ADAPT | 3 | M | ! | OK | OK | Edits multi-fichiers = transaction Automerge unique par fichier ; **aperçu des changements avant application** et « Annuler » en toast. Divergences possibles si concurrence : message « Une autre personne modifie ce fichier ». |
| Repliement (titres, blocs) | tinymist | REUSE | 3 | S | OK | OK | OK | Local à la personne, jamais synchronisé. |
| Symboles d'espace de travail / palette « Aller à » | tinymist, VS Code | ADAPT | 4 | S | OK | OK | OK | Dans la palette Ctrl/Cmd+K (règle 3). |
| Défilement collant, minicarte | VS Code (g) | REJECT | 1 | S | OK | OK | OK | Surcharge visuelle pour documents. |
| Vue Étiquettes (liste des cibles de renvoi) | tinymist | ADAPT | 3 | S | + | OK | OK | « Renvois possibles » dans le tiroir, avec insertion en un clic. |

### 3.4 Aperçu, export (N20)

| Fonction | Source | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Aperçu incrémental | tinymist | REUSE | 5 | M | OK | clé | OK | Déjà prévu (étape aperçu). Rendre non-bloquant : l'aperçu n'attend pas la frappe. |
| **Saut croisé** clic aperçu → source et retour | tinymist, typst.app | ADAPT | 5 | M | + | OK | OK | Attendu (typst.app référence assumée). Variante collaborative : le saut met en surbrillance 1 s la plage, et montre qui d'autre y est. |
| Export PDF/PNG/SVG | tinymist | REUSE | 5 | S | OK | OK | OK | Un seul bouton « Exporter » avec choix ; PDF par défaut. |
| Export HTML (Typst ≥ 0.15) | tinymist/Typst | PLUGIN | 2 | M | OK | OK | OK | Encore à mûrir ; bundle expérimental. Reporter. |
| Export Markdown / texte | tinymist | REJECT | 1 | S | OK | OK | OK | Hors périmètre. |
| Mode « syntaxe seule » | tinymist | REUSE | 4 | S | OK | gain | OK | Bascule automatique au-delà d'un seuil de taille ou d'un budget temps (perf sur gros fichiers). |
| Aperçu Markdown + CSS | (N27) | ADAPT | 5 | M | OK | OK | OK | Même saut croisé via cartes de source du rendu. |
| Rendu de fichiers txt/csv/code (N20) | Typora (g) | ADAPT | 4 | M | OK | OK | OK | csv en tableau, code colorié ; coloration par grammaire CodeMirror/Lezer, **pas** par LSP (hors scope). Jetons sémantiques tinymist seulement pour Typst. |

### 3.5 Modèles et paquets (N27)

| Fonction | Source | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Galerie de modèles | tinymist, Universe | ADAPT | 5 | M | + | OK | OK | **Aucun modèle par défaut** (N27) : la galerie liste les modèles *de la personne* ; vignette générée par compilation locale (cache), description obligatoire, champs à remplir à la création. |
| `typst init` | Typst | REJECT | — | — | — | — | ! | Télécharge depuis Universe : contraire à INV-6. Remplacé par « Nouveau projet depuis un modèle » (copie locale). |
| Import d'un modèle Universe | Universe | PLUGIN | 3 | M | OK | OK | ! | Greffon *hors ligne* : l'import se fait depuis un fichier/archive que la personne a déjà. Jamais de réseau en tâche de fond. |
| Vue Paquets | tinymist | ADAPT | 3 | M | OK | OK | OK | « Bibliothèques fournies » en lecture seule (+ `.typstdeps/`). |
| Docs de paquets (LSIF/SCIP, v0.15.4) | tinymist | PLUGIN | 2 | L | OK | OK | OK | Peu utile sans développeuse ; reporter. |

### 3.6 Collaboration et relecture

| Fonction | Source | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Commentaires ancrés à une plage | typst.app, Overleaf, Docs | ADAPT | 5 | M | + natif CRDT | OK | OK | Gratuit (contre typst.app). Ancrage par curseur Automerge ; fils, résolution, archivage ; hors du PDF. Panneau Activité. |
| Mode Suggestion (accepter/rejeter) | Google Docs, Overleaf | ADAPT | 4 | L | ! | OK | OK | Reporter en v2 : demande un modèle « brouillon » (`Branch` du glossaire, v2). Cohérent : « Afficher les modifications ». |
| Rôle relecture seule | typst.app | REUSE | 5 | S | OK | OK | OK | Déjà `RoleReader` ; autoriser le commentaire pour ce rôle (décision de protocole à ouvrir). |
| Historique / versions | Overleaf | REUSE | 5 | — | OK | OK | OK | Déjà prévu (go-git, jalons). |
| Éditeur visuel ↔ source | Quarto, Overleaf | ADAPT | 5 | L | ! | OK | OK | Pour Markdown : CodeMirror « live preview » (symboles masqués hors de la ligne active, à la Obsidian/Typora). Pour Typst : barre de mise en forme sur sélection (déjà prévue). Pas de WYSIWYG Typst complet. |
| Mode focus / machine à écrire | iA Writer, Typora | ADAPT | 3 | S | OK | OK | OK | Option d'affichage, local ; respecte `prefers-reduced-motion`. |
| Nombre de mots / pages | Typora, tinymist | REUSE | 4 | S | OK | OK | OK | Barre d'état ; « pages » via l'aperçu. |
| Rétro-liens (g) | Obsidian | REJECT | 1 | M | OK | OK | OK | Modèle de notes, pas de documents. |
| Formatage automatique (typstyle) | tinymist | ADAPT | 3 | S | ! | OK | OK | « Mettre en forme » sur sélection, jamais à l'enregistrement (réécrire le fichier entier produit des diffs concurrents). Sinon plage minimale. |
| Profileur | tinymist | PLUGIN | 1 | M | OK | OK | OK | Greffon « Diagnostic de lenteur » pour personnes avancées. |

## 4. Idées nouvelles que la collaboration permet

- **I1. Complétion des étiquettes et fonctions des collègues.** Le service de langage voit tout le projet, y compris ce qu'une autre personne vient de taper (fusion CRDT avant analyse). Marquer chaque suggestion d'un avatar + « ajouté par Léa il y a 2 min ». Coût S au-dessus de N29 ; valeur élevée.
- **I2. Présence dans le plan.** Pastilles de couleur (Okabe-Ito, nom au survol, jamais la couleur seule) devant les titres où se trouvent les curseurs ; clic = rejoindre. Coût S.
- **I3. Erreur « appartient à qui ».** Un problème indique si c'est la dernière modification de quelqu'un (« après la modification de Marc ») ; bouton « Prévenir » = commentaire ancré automatique. Coût M ; n'accuse pas : formule neutre.
- **I4. Fonctions partagées documentées par tout le groupe.** Le docstring est du texte CRDT : la charte évolue avec sa doc ; assistant « Documenter » (voir §3.1). Une fonction sans doc est signalée en Suggestion.
- **I5. Aperçu personnel vs commun.** Chaque personne règle son zoom, son thème d'aperçu et son repliement localement (non synchronisés) ; seul le contenu l'est.
- **I6. Suivi de l'aperçu d'une collègue (« Suivre »).** À la Figma : l'aperçu suit la position d'une personne choisie, y compris la page courante. Coût M.
- **I7. Renommage collaboratif sûr.** Aperçu des endroits touchés par fichier, avec les personnes présentes dans chacun ; avertit avant d'agir.
- **I8. Les modèles sont des projets partagés** (N27) : « Proposer une modification au modèle » = brouillon (v2), relu par l'administratrice du groupe.

## 5. Contraintes d'architecture à ne pas oublier

1. **Source de vérité du texte** : Automerge côté webview. Le pont LSP envoie `didOpen`/`didChange` depuis la webview ; tinymist ne lit pas le disque pour les fichiers ouverts. Les positions LSP (UTF-16) ↔ indices Automerge : table de conversion testée par propriété.
2. **Concurrence et réponses LSP obsolètes** : numéroter les versions du document ; abandonner toute réponse dont la version est dépassée (déjà évoqué : « sinon abandonnés », spec §1441).
3. **Tout octet venu d'un pair est non fiable** : un docstring, un nom de fichier ou un message de diagnostic dérivé d'un fichier partagé ne doit jamais devenir du HTML brut ni une commande ; décodeurs fuzzés.
4. **Hors ligne strict** : l'index de complétion des paquets vient de l'archive embarquée ; vérifier au banc d'essai que tinymist ne tente aucun réseau (test d'intégration avec réseau coupé, `strace`/espace de noms réseau vide).
5. **Une feature = un point d'entrée existant** : tiroir à onglets, palette, bannières, toasts (règle de cohérence). Pas de nouveau panneau si un onglet suffit (Problèmes dans le tiroir, Plan en onglet voisin).
6. **Mesure** : tâches « corriger une erreur de charte » et « utiliser une fonction de la charte sans la connaître », temps et nombre d'erreurs en test en personne (règle 10).

## 6. Les dix recommandations, par priorité

1. **REUSE** : brancher le LSP tinymist (complétion, signature, survol) avec pont à versions, p95 ≤ 100 ms (N29).
2. **ADAPT** : panneau Problèmes unique + F8/Maj+F8 + clic = saut prêt à corriger, messages traduits par table déterministe (N28).
3. **ADAPT** : docstrings `///` rendus en survol/signature et assistant « Documenter » (fonctions de la charte, N29).
4. **ADAPT** : saut croisé aperçu ↔ source avec surbrillance (déjà la référence typst.app).
5. **ADAPT** : commentaires ancrés (curseurs Automerge), gratuits, rôle relecture autorisé à commenter.
6. **REUSE** : plan cliquable + I2 (présence dans le plan) ; fil d'Ariane léger.
7. **ADAPT** : galerie de modèles locale, sans modèle par défaut, vignettes compilées en local ; remplace `typst init` (N27, INV-6).
8. **ADAPT** : complétion des étiquettes/renvois + I1 (suggestions des collègues) ; vue Symboles.
9. **REUSE** : mode « syntaxe seule » automatique sur gros fichiers + nombre de mots/pages.
10. **ADAPT** : renommage multi-fichiers avec aperçu et « Annuler » ; formatage typstyle sur sélection seulement.

À rejeter : indications en ligne par défaut, défilement collant, minicarte, rétro-liens, `typst init`/Universe en ligne, export Markdown. En greffon (N21) : export HTML/bundle, import de modèles Universe depuis archive locale, profileur, docs de paquets. Reporté v2 : mode Suggestion (dépend des brouillons), déplacement de sections par glisser dans le plan.

## 7. Limites de l'étude

Pages tinymist `completion.html`, `lint.html`, `lsp.html` introuvables (404) ; la complétion des étiquettes/bibliographie et les options hors ligne de tinymist sont donc **[à vérifier]** par un banc d'essai sur le binaire épinglé avant toute décision ferme. Les sources des outils d'écriture non lisibles sont marquées (g). Aucune mesure de performance n'a été faite ici ; les budgets viennent de la spec.

---

# Addendum N30 : outils d'écriture (« se sentir chez soi quand on aime écrire »)

Date : 2026-11-01. Même grille (V, C, Co, P, H) et mêmes classes qu'au §3. Colonne **Déjà là** : ce que Typst ou tinymist fournit. Principe directeur : **le texte d'abord, aucun bruit** ; tout est discret par défaut, local à la personne (jamais synchronisé), et activable dans un seul réglage « Aide à l'écriture ».

## 8. Sources nouvelles

- LanguageTool : cœur sous **LGPL 2.1+**, serveur HTTP local (`/v2/check`, port choisi par l'appelant, inaccessible aux autres machines par défaut, Java 8+) : [dépôt](https://github.com/languagetool-org/languagetool), [serveur HTTP](https://dev.languagetool.org/http-server). Les n-grammes (meilleure détection des confusions) sont **hors du téléchargement**, ~8 Go en anglais, disponibles aussi pour le français, SSD recommandé ([n-grammes](https://dev.languagetool.org/finding-errors-using-n-gram-data)). Application ~156 Mo ([Wikipedia](https://en.wikipedia.org/wiki/LanguageTool), source secondaire) ; mémoire : exemples à 2-4 Go de tas avec n-grammes, aucun minimum officiel trouvé. Par défaut ses réglages utilisent le nuage de LanguageTool : un serveur embarqué devrait le désactiver explicitement.
- Dictionnaires Hunspell via [wooorm/dictionaries](https://github.com/wooorm/dictionaries) : français **MPL-2.0**, anglais **MIT AND BSD** ; tailles non publiées sur la page **[à mesurer]** ; nspell les lit.
- [Harper](https://github.com/hydraxman/harper) (miroirs ; dépôt amont non confirmé **[à vérifier]**) : correcteur de grammaire hors ligne en Rust, WASM, **anglais seulement**, comprend déjà le Typst.
- Typst : [`smartquote`](https://typst.app/docs/reference/text/smartquote/) (guillemets selon `lang`, ex. `fr`), [`text(lang, region, hyphenate)`](https://typst.app/docs/reference/text/text/). Espaces françaises : le [forum](https://forum.typst.app/t/french-unbreakable-spaces-rule/8379) indique qu'il reste du travail pour respecter toutes les règles et cite l'[issue #1920](https://github.com/typst/typst/issues/1920) (insécables dans « »). **Ne pas supposer** que `lang: "fr"` suffit **[à vérifier sur le Typst épinglé]**.
- [Hayagriva](https://github.com/typst/hayagriva) : YAML natif, BibTeX/BibLaTeX en entrée (`.bib` accepté), ~100 styles CSL embarqués (licence CC-BY-SA 3.0 pour ces styles, attribution à prévoir), MIT/Apache-2.0 pour le code.
- Google Docs [mode Suggestion](https://support.google.com/docs/answer/6033474), iA Writer [mode focus](https://ia.net/writer/support/editor/focus-mode), Typora [mode focus / machine à écrire, nombre de mots](https://typora.io/) : déjà cités au §2.
- Non vérifiés faute de page lisible (g) : Antidote (analyse de style, dictionnaires de synonymes), Hemingway, Zettlr, LyX, Zotero (export BibTeX/CSL), formule de lisibilité Kandel-Moles pour le français, CNRTL pour les synonymes.

## 9. Classification

### 9.1 Correction et style

| Fonction | Déjà là | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Orthographe fr/en, soulignement discret | Ni Typst ni tinymist (aucune vue dans la doc **[à vérifier]**) | ADAPT | 5 | M | ! | OK | OK | Hunspell/nspell avec dictionnaires **embarqués** (MPL-2.0 / MIT+BSD : mentions de licence dans « À propos »). Exécuté dans la webview sur le **texte de prose seulement** (pas le code Typst : on le déduit de l'arbre syntaxique, sinon `#let` serait souligné). Langue = `lang` du document. |
| Dictionnaire personnel et du projet | — | ADAPT | 5 | S | + | OK | OK | « Ajouter au dictionnaire » : personnel (local) ou **du projet** (liste partagée, texte CRDT) : les noms propres d'un roman profitent à tout le groupe. |
| Grammaire fr/en | — | PLUGIN | 4 | L | ! | lourd | ! | **LanguageTool** en greffon optionnel (N21) : JVM + règles. Licence LGPL 2.1+ : compatible en processus séparé lancé sur `127.0.0.1`, port éphémère, jeton en temps constant (invariant serveurs locaux), **mode nuage coupé**. Poids : application ~156 Mo, n-grammes en Go : jamais embarqués ; la personne les ajoute elle-même (aucun téléchargement par l'application). Si la JVM n'est pas présente, le greffon l'indique. Alternative légère : Harper pour l'anglais seulement. |
| Affichage des suggestions de grammaire sans bruit | — | ADAPT | 5 | S | OK | OK | OK | Pas de soulignement rouge permanent. Pointillé **gris-bleu** sous l'endroit ; l'infobulle montre une phrase + « Corriger » / « Ignorer ». Pas de compteur dans l'en-tête ; un onglet « Relecture » du tiroir liste les suggestions, **à la demande** (bouton « Relire ») plutôt qu'à la frappe. L'orthographe seule reste à la frappe. Jamais de couleur seule (aussi une icône). |
| Synonymes et dictionnaire des synonymes | — | ADAPT | 4 | M | OK | OK | OK | Jeu libre hors ligne (ex. listes sous licence ouverte, **[à choisir, licence à vérifier]**), consultable via clic droit sur un mot (« Autres mots »). Greffon si la licence impose des mentions. |
| Lisibilité et style (phrases longues, répétitions, adverbes) | — | ADAPT | 3 | M | OK | OK | OK | Surlignage **à la demande** dans l'onglet « Relecture » (à la Hemingway), jamais une note chiffrée comme verdict. Répétitions proches : utile et facile (hors ligne, sans modèle). Formules de lisibilité : indicatif, adapté à la langue **[à valider pour le français]**. |
| Lissage automatique par IA (réécrire) | — | REJECT | — | — | — | — | ! | Hors périmètre (réseau, vie privée). Greffon de tiers possible, jamais livré. |

### 9.2 Concentration et suivi

| Fonction | Déjà là | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Mode focus (phrase/paragraphe) et machine à écrire | — | ADAPT | 4 | S | OK | OK | OK | Local. Masque tiroirs et barres, plein écran sans distraction (Échap pour quitter). Respecte `prefers-reduced-motion` (pas de défilement animé). |
| Mots, caractères, temps de lecture | tinymist (mots, barre d'état) | REUSE/ADAPT | 4 | S | + | OK | OK | Sélection comptée ; temps de lecture ~ 230 mots/min fr, réglable. Le compte porte sur la prose, pas sur le balisage. |
| Objectifs et statistiques de session | — | ADAPT | 3 | S | OK | OK | OK | Objectif de mots (jour/session) en cercle discret ; **jamais de série à maintenir** ni de rappel culpabilisant ; stockage **local** (aucune télémétrie, règle UX 10). |

### 9.3 Chercher, remplacer, abréger

| Fonction | Déjà là | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Chercher dans le fichier, le projet | tinymist : symboles seulement | ADAPT | 5 | M | ! | OK | OK | Ctrl+F simple ; « Dans tout le projet » ; résultats groupés par fichier, avec présence des collègues. **Regex masqué** derrière « Options avancées ». Remplacer tout : aperçu + « Annuler ». |
| Remplacer sur plusieurs fichiers | tinymist : renommage de symbole | ADAPT | 4 | M | ! | OK | OK | Une transaction par fichier ; avertit si une personne édite un fichier touché. |
| Extraits et expansions de texte (ex. `;sig`) | Typst : snippets de tinymist | ADAPT | 3 | M | + | OK | OK | Liste de la personne ; liste **du projet** (partagée). Le texte inséré peut contenir des champs, compatible avec les modèles (N27). |

### 9.4 Typographie française (très important)

| Fonction | Déjà là | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Guillemets « » intelligents, apostrophe ’ | Typst `smartquote` (selon `lang`) | REUSE | 5 | S | OK | OK | OK | Typst le fait **à la compilation** : la personne écrit `"` et obtient « » dans l'aperçu. Dans l'**éditeur** (Markdown/txt), conversion à la frappe optionnelle. Pour le Typst, ne **jamais** réécrire le texte source : l'aperçu suffit. |
| Insécables avant `;` `:` `!` `?`, dans « » | Partiel côté Typst (issue #1920 ouverte à ma connaissance de la source) **[à vérifier]** | ADAPT | 5 | M | OK | OK | OK | Deux niveaux. 1) **Rendu** : règle d'affichage (show rule) fournie par l'application pour les documents `lang: fr` (U+202F avant `;` `!` `?`, U+00A0 avant `:` et dans « », cf. forum) : le texte source reste propre. 2) **Édition** (txt/Markdown) : conversion à la frappe optionnelle, avec point visible (affichage des espaces insécables, le caractère `·` discret, désactivable) et vérification « Typographie » dans l'onglet Relecture. Les modèles de N27 portent la règle. Test : jeux de phrases fr avec attendus Unicode exacts. |
| Points de suspension, tirets (– —), « œ », majuscules accentuées | — | ADAPT | 4 | S | OK | OK | OK | Table de remplacements typographiques (activable) et palette de caractères spéciaux (voir 9.6). |
| Césure et langue du document | Typst `lang` (césure automatique) | REUSE | 4 | S | OK | OK | OK | Sélecteur de langue du projet (pas par fichier d'abord), exposé comme « Langue du document ». |

### 9.5 Références, structure, insertion

| Fonction | Déjà là | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Notes de bas de page | Typst natif (`#footnote`) | REUSE | 5 | S | OK | OK | OK | Bouton « Note » : crée l'appel, place le curseur dans la note. En Markdown : syntaxe standard. |
| Citations et bibliographie | Typst + Hayagriva (YAML, `.bib`) | ADAPT | 5 | M | + | OK | OK | « Sources » : liste des références du projet ; « Citer » ouvre un sélecteur (auteur, titre) qui insère `@clé`. Import d'un `.bib` exporté de Zotero (g) **par fichier** (jamais de connexion à Zotero : réseau et compte hors périmètre ; greffon éventuel). Styles CSL embarqués, **mention CC-BY-SA 3.0**. |
| Renvois croisés, table des matières | Typst natif (`#outline`, `@label`) ; tinymist : plan, étiquettes | REUSE/ADAPT | 5 | S | + | OK | OK | « Insérer un renvoi » ouvre la liste des cibles ; « Table des matières » en un clic (voir §3.3, I1). |
| Insérer figure / tableau | Typst natif ; barre de mise en forme (N20) | ADAPT | 5 | M | OK | OK | OK | Boîte « Image » (glisser, légende, nom) ; tableau par grille de cases, rempli depuis un `.csv` du projet. Génèrent du Typst/Markdown propre, jamais du balisage opaque. |
| Aide à la saisie des maths | Typst : mathématiques natives, tinymist : symboles | ADAPT | 4 | M | OK | OK | OK | Palette de symboles + aperçu de l'équation en direct dans une infobulle ; « Équation » en ligne ↔ bloc (action de code tinymist). |
| Caractères spéciaux et émojis | tinymist : vue Symboles | ADAPT | 4 | S | OK | OK | OK | Sélecteur unique (Ctrl/Cmd+;) : symboles Typst, ponctuation française, émojis ; recherche par nom français. Jeu **embarqué**. |

### 9.6 Versions, relecture, export

| Fonction | Déjà là | Classe | V | C | Co | P | H | Décision |
|---|---|---|---|---|---|---|---|---|
| Comparer deux brouillons / versions | — (go-git, jalons prévus) | ADAPT | 5 | M | + | OK | OK | « Afficher les modifications » (nom figé du glossaire) : comparaison **par mots**, pas par lignes ; ajouts/suppressions soulignés (couleurs + signes). Entre deux jalons ; entre deux brouillons en v2. |
| Annotations et commentaires | — | ADAPT | 5 | M | + | OK | OK | Déjà en rec. 5 : commentaires ancrés, résolution, onglet Activité. Ajouter **surlignage** personnel (local) distinct du commentaire (partagé). |
| Mode Suggestion | — | ADAPT (v2) | 4 | L | ! | OK | OK | Inchangé : dépend des brouillons. |
| Préréglages d'export | tinymist : exports | ADAPT | 4 | S | OK | OK | OK | « Préréglages » du projet (ex. « Relecture : PDF avec numéros de ligne », « Impression », « Écran ») ; stockés dans le projet, donc **partagés**, et rattachés au modèle (N27). Un seul bouton « Exporter ». |

## 10. Idées nouvelles propres à la collaboration (écriture)

- **W1. Dictionnaire du projet partagé** : un nom de personnage ajouté par l'une cesse d'être souligné chez toutes.
- **W2. Règles de typographie partagées par le projet** : langue, guillemets, insécables, appartiennent au projet ; tout le groupe voit le même rendu, aucune « guerre d'apostrophes ».
- **W3. « Relecture » assignable** : le résultat d'une relecture (orthographe/style) devient des commentaires ancrés, adressés à une personne.
- **W4. Objectif de groupe** (optionnel, jamais classement) : compteur de mots du projet, sans nom par personne par défaut.
- **W5. Sources partagées** : la bibliographie est du texte CRDT, « Citer » propose aussi les sources que les collègues viennent d'ajouter, avec leur avatar.

## 11. Recommandations mises à jour (ajouts aux dix du §6)

11. **ADAPT** : orthographe fr/en Hunspell embarquée, soulignement discret, dictionnaire du projet partagé (V5, C M, hors ligne).
12. **ADAPT** : typographie française en deux niveaux, rendu (show rule du modèle) et saisie optionnelle ; ne pas se fier au seul `lang: "fr"` de Typst avant vérification.
13. **PLUGIN** : LanguageTool (LGPL 2.1+) comme greffon hors ligne en processus séparé, nuage coupé, n-grammes jamais embarqués ; suggestions dans l'onglet « Relecture » à la demande.
14. **ADAPT** : onglet « Relecture » unique (style, répétitions, typographie) ; jamais de note chiffrée ni de soulignement rouge permanent.
15. **ADAPT** : focus/machine à écrire, mots + temps de lecture, objectifs **locaux** et sans culpabilisation.
16. **ADAPT** : recherche/remplacement de projet avec regex masqué ; extraits partagés.
17. **REUSE/ADAPT** : notes, citations (import `.bib`), renvois, table des matières, figures/tableaux, maths, caractères : tous via un menu « Insérer » et la palette, générant du Typst propre.
18. **ADAPT** : comparaison par mots entre jalons, préréglages d'export partagés avec le modèle.

À rejeter : réécriture par IA livrée, séries d'écriture et notifications de gamification, connexion directe à Zotero (réseau/compte) dans le cœur. Avant décision ferme, à mesurer : tailles des dictionnaires fr/en embarqués, poids du greffon LanguageTool et mémoire JVM, comportement de `lang: "fr"` sur le Typst épinglé, licence du jeu de synonymes.
