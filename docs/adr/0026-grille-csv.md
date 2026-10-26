---
status: proposed
date: 2026-10-25
decision-makers: [Mathieu]
---

# Tableur CSV/TSV : grille Svelte maison virtualisée, texte Automerge pour seule source

## Contexte et problème

Un `.csv`, `.tsv` ou `.psv` s'ouvre en tableur (plan B9). Il faut une grille qui tienne trois
exigences : la CSP stricte (`style-src 'self' 'nonce-…'`, sans `unsafe-inline`), de gros fichiers
(500 000 lignes, défilement fluide, édition d'une cellule en moins de 16 ms) et la sémantique
clavier et ARIA d'une grille. Deux options : RevoGrid (composant web Stencil, MIT) ou une grille
Svelte écrite pour l'occasion.

## Facteurs de décision

- (a) **CSP** : aucun `<style>` sans nonce, aucun attribut `style` écrit dans le HTML.
- (b) **Performance** : 60 images/s, mémoire bornée, cellule éditée en moins de 16 ms.
- (c) **Accessibilité** : `grid`, `row`, `gridcell`, `aria-rowindex`, clavier complet, axe sans erreur.
- Une seule source de vérité : le texte Automerge ; aucune copie du tableau en mémoire.
- Collaboration : curseurs des autres sur les cellules, déplacement de lignes et de colonnes,
  poignée de recopie : tout cela doit se brancher sur le modèle texte.
- Chaîne d'approvisionnement : `minimumReleaseAge` de 7 jours, versions exactes (ADR 0008, 0010).

## Options envisagées

- RevoGrid 4.29.1 (composant web virtualisé).
- Grille Svelte maison, lignes et colonnes virtualisées, rendue en DOM.
- Grille en `canvas` avec miroir DOM accessible (gardée en réserve).

## Décision

Option retenue : « grille Svelte maison virtualisée en DOM », parce qu'elle satisfait les trois
critères avec des mesures, qu'elle lit les cellules à la demande dans le texte, et que
RevoGrid demande un modèle d'objets qui contredit la source unique.

### Mesures (Chromium 1228 sans fenêtre, machine chargée : load 20)

**RevoGrid 4.28.4** (spike jetable, 500 000 lignes × 10 colonnes, tableau d'objets) :

| Critère | Résultat |
|---|---|
| Version | 4.29.1 est parue le 2026-10-23 : **refusée par `minimumReleaseAge`** ; 4.28.4 (2026-10-09) passerait |
| (a) CSP | Aucune violation, **à condition** de garder `<meta name="csp-nonce">` (Stencil le lit) ; positions posées par CSSOM |
| (b) défilement | 16,7 ms par image (60 i/s) ; 25 lignes DOM ; 270 Mo de tas (les 500 000 objets doivent exister) |
| (c) ARIA | `row`, `gridcell`, `aria-rowindex` présents, **aucun `role="grid"`** sur le conteneur (axe : `aria-required-parent`) |
| Taille | 7,5 Mo dépaquetés, runtime Stencil et chargement paresseux de composants |

**Grille maison** (même machine) :

| Critère | Résultat |
|---|---|
| (a) CSP | Aucune violation en e2e ; positions par `style:` (CSSOM), jamais d'attribut `style` dans le HTML |
| (b) ouverture | Index Papa Parse 5.7.0 (worker) de 132 Mo, 500 000 × 20 : 0,43 à 1,0 s ; 5 Mo (40 000 × 12) utilisable en 0,23 s |
| (b) défilement navigateur | 40 000 × 12 : image médiane 16,7 ms (60 i/s), script 5,9 ms et mise en page 2,6 ms par image |
| (b) édition | Planifier + éclater le texte + index incrémental : **0,4 à 0,5 ms** sur 132 Mo ; lecture d'un écran 60 × 20 : 2 ms |
| (b) copie | 100 000 cellules en TSV : 60 à 110 ms |
| (b) vues | Tri d'une colonne de 500 000 lignes : 0,6 à 0,9 s ; somme d'une colonne : 0,8 s |
| (c) | `grid`/`row`/`columnheader`/`gridcell`, `aria-activedescendant`, phrase lue « ligne 12, colonne Prix, 34,50 » ; axe : 0 violation |

La grille `canvas` n'a pas été construite : le DOM virtualisé tient les cibles.

### Limite mesurée de la pile, hors grille

Automerge 3.5.0 : un `splice` de texte coûte 8 ms sur 5 Mo et 14 ms sur 10 Mo ; sur 20 Mo
le test échoue (`PanicError: splice range out of bounds`). **Un fichier de 50 Mo ne passe donc pas
de bout en bout par le document**, quelle que soit la grille. Le modèle et la grille, eux, sont
mesurés sur 132 Mo (tests de performance, `INKWELL_PERF=1`). **Décision :** un fichier de plus
de 10 Mo (10 × 2^20 caractères du texte, `MAX_EDITABLE_CHARS`) s'ouvre en **lecture seule**, avec
le message « Fichier trop grand pour l'édition à plusieurs (X Mo) : ouvert en lecture seule ».
Voir, trier, filtrer, chercher et copier restent possibles (index Papa Parse, aucune écriture) ;
toute modification est refusée au plus bas niveau (`SheetDoc.perform`, annuler et rétablir
compris), si bien qu'aucun chemin n'écrit dans le document. Le seuil est relu à chaque
changement du texte : un fichier qui grossit dans la vue texte passe en lecture seule, un fichier
qui rétrécit redevient modifiable. La vue texte (CodeMirror) n'est pas verrouillée : elle
reste utilisable, mais un collage qui porterait le texte au-delà de 20 Mo reste hors de
portée du document (limite d'Automerge, hors grille). Tests : `sheet.test.ts` (CSV-20),
`e2e/sheet.spec.ts` (SHEET-BIG, fichier de 12 Mo dans une instance à part).

### Conséquences

- Bien : aucune dépendance de grille ; la mémoire reste proportionnelle au texte ; chaque action
  est un seul `handle.change` donc un seul pas d'annulation ; la présence réutilise le canal du
  texte, si bien qu'une personne dans la vue texte voit une plage de cellules comme une sélection.
- Moins bien : le clavier, le glisser-déposer et l'accessibilité sont à entretenir ici.
- **Édition collaborative** : deux personnes qui remplacent la même cellule
  fusionnent leurs textes bout à bout. Un morceau sans guillemets suivi d'un morceau entre
  guillemets (`a","`) casserait la cellule. Tant que d'autres personnes sont dans le fichier,
  chaque cellule écrite est donc entre guillemets (`"a"","` reste une seule cellule) ; seule,
  la personne obtient les guillemets minimaux de la RFC 4180. **Étudié, non retenu : des
  guillemets minimaux sûrs.** Deux écritures simultanées d'une même cellule se concatènent
  dans un ordre fixé par les identifiants d'acteur, que l'écrivain ne contrôle pas. Une
  écriture simple suivie d'une écriture entre guillemets (`x` puis `"y,z"`) donne `x"y,z"`, que
  tout lecteur (Typst compris) coupe en deux cellules ; l'inverse (`"y,z"x`) reste une cellule.
  Il n'existe donc pas de règle « guillemets seulement si nécessaire » sûre : tout écrivain
  doit pouvoir être celui qui précède, d'où le quotage systématique. Seule piste envisagée (non implémentée) : ancrer
  l'insertion d'une valeur simple après l'ancien texte de la cellule pour qu'elle passe
  toujours en second ; elle ne dit rien d'une cellule vide (même ancre, ordre libre) et repose sur
  le départage interne d'Automerge. Le test `collab.test.ts` (CSV-04b) montre le contre-exemple
  dans les deux ordres d'acteurs. Conséquence assumée : des octets en plus (guillemets) dans
  les cellules écrites pendant qu'une autre personne est là ; le fichier reste valide et
  relu à l'identique.
- **Modifications hors d'Inkwell (B1)** : le texte d'un fichier tableau suit la même fusion que
  le texte (`ExternalSync`). Les cellules touchées sont surlignées, « Voir » va à la première,
  Ctrl+Z reprend d'abord l'intégration si rien n'a été écrit depuis. Tant que le projet n'a
  qu'un document (B4), le fichier du disque est son `main.typ` : l'affichage en tableau
  (`?file=data.csv`) n'y change rien. Test : `e2e/sheet.spec.ts` (CSV-14).
- **Séparateur** : l'extension donne le point de départ (`.tsv` et `.tab` : tabulation sans
  guillemets ; `.psv` : barre verticale ; `.csv` : virgule ou point-virgule, ou ce que la détection
  trouve), le contenu le confirme. Un contenu qui contredit l'extension est lu tel qu'il est écrit
  et signalé (« Séparateur détecté : tabulation (l'extension indique virgule) ») avec un choix en
  un clic, retenu par fichier. Un `.txt` qui ressemble à un tableau se voit proposer « Ouvrir comme
  tableau ? », jamais imposé. Tests : `extension.test.ts`, `e2e/sheet.spec.ts` (CSV-DIALECT).
- Le tri appliqué au fichier ne déplace que les lignes qui changent de place (plus longue suite
  croissante) ; au-delà de 2 000 lignes déplacées c'est une seule modification.
- Annuler est par personne et retrouve son texte par ancres sur les caractères voisins ; il refuse
  (au lieu de deviner) si ce texte a changé depuis.

### Confirmation

- Tests : `frontend/src/editor/csv/*.test.ts`, `frontend/src/ui/sheet/*.test.ts`,
  `frontend/e2e/sheet.spec.ts` (deux instances réelles, souris, clavier, presse-papiers).
- `INKWELL_PERF=1` pour faire échouer un budget (journalisé sinon, comme STA-12).
- Le gel `strict CSP` est vérifié en e2e (aucune violation `Content Security Policy`).

## Plus d'informations

- RevoGrid : https://github.com/revolist/revogrid (MIT). À réexaminer si une version expose un
  modèle de données paresseux et un `role="grid"`.
- Papa Parse 5.7.0 : https://www.papaparse.com (MIT), utilisé en flux (`step`, `meta.cursor`) pour
  n'en tirer que les débuts de lignes.
