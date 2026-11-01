# Revue Fable n°7 du système de design (révision 7, 2026-11-01) — verdict : **ACCEPT**

Revue indépendante de `spec.md` rév. 7 (`bfcd3fc`), `CHANGES.md`, `tokens.json`, `coverage.md`,
`inventory.md`, contre ma revue n°6 (`09-fable-r6.md`) et les deux majeurs de Codex n°6 tels que
`CHANGES.md` les résume. La revue Codex n°7 n'a pas été lue.

**Méthode.**
- Diff complet `673ac2b..bfcd3fc` sur les cinq fichiers (spec : 124 lignes ; tokens : 33 ;
  coverage : 6 ; inventory : 1), puis relecture des sections touchées dans le texte courant
  (§2.10, §3.2, §4.22, §4.35, §4.39, §4.41, §4.43, §6.4, §8.2, §8.4, §10).
- Les **160 paires** de `tokens.json` sont recalculées par script dans les quatre modes
  (**634 valeurs** ; `hcOnly` seulement en HC ; WCAG 2.2, luminance relative, translucides composés
  en sRGB sur `over`, `hcMin` appliqué comme `themeContract.hcRule` le déclare) et comparées aux
  valeurs déclarées ; les contre-exemples `#acacac`, `#bbbbbb` et `#a8a8a8` sont rejoués contre
  **toutes les paires non interdites** qui touchent `--hover-bg`, en clair et en HC clair (§2).
- La colonne macOS de §8.2 est relue ligne par ligne : doublons, présence dans la table réservée,
  K-7 ; la liste d'Apple (*Mac keyboard shortcuts*, support 102650) sert de référence [S30].
- Chaque résidu est cherché par `grep` dans les cinq fichiers.

## 1. Fermeture des constats

### Fable n°6

| # | Fermé ? | Preuve |
|---|---|---|
| **M1** (`#bbbbbb` accepté par le contrat) | **oui** | `spec.md:388-394` : `#acacac` refusé (6,94 ; **4,45** ; **2,67**), `#bbbbbb` accepté (4,81 ; 3,16), et la rév. 6 reconnue fausse ; `:3454` (test TH-D2 rév. 7) ; `CHANGES.md:344`. **Rejeu** (§2) : `#acacac` échoue sur **12 paires** en clair (6 × `--fg-dim-on-state` à 4,45 < 4,5 ; 6 × `--accent` à 2,67 < 3) et sur 18 en HC clair → **refusé** ; `#bbbbbb` : 36 paires, 0 échec → **accepté**. Les quatre rapports cités sont exacts à 0,01. |
| m1 (`#ffffff` brut) | oui | `tokens.json:2973` : `--on-dark-fg` ; recalcul 4,83 / 6,11 / 4,83 / 6,11, identique à `spec.md:1733`. Plus aucune valeur brute dans les 160 paires (script). |
| m2 (« Git » en mode développeur) | oui | `:1092-1094` « sans exception, même en mode développeur » ; `:1184-1186` « Exporter l'historique (format technique)… », nom du format seulement sous « Détails techniques » ; `:1729`. `grep Git` : il ne reste que le titre de §4.22, la règle, §6.4 (colonne « jamais »), G-101 et le test. |
| m3 (deux lignes « retour en arrière ») | oui | `:3159` supprimée ; `:3179` porte l'union (commit, checkout, revert, rollback, HEAD, Git). |
| m4 (`--variant-line` orphelin) | oui | jeton `tokens.json:961-975` et paire `:4052-4062` retirés (script : 90 jetons, aucun `variant-line`) ; `spec.md:1112`. Réserve de place : m3 ci-dessous. |
| m5 (bandeau permanent) | oui | `:2636-2648` : bandeau **seulement** si la phrase n'est pas notée ou pas vérifiée, fermable, retour au lancement suivant après 7 jours, rang 4 « Information » de L-8 (`:573`) ; jamais pour le kit ; état « Phrase pas encore notée · Noter » ajouté `:2639`. |
| m6 (⌃⌘S « convention ») | oui | panneau de gauche **⌥⌘S** `:3283` ; Enregistrer une version **⌃⌘S** `:3286` et `:1153` ; ⌃⌘D, ⌥⌘T, ⌃F1 à ⌃F8 réservés `:3317-3318`. |
| m7 (sort de la variante) | oui | `:2527-2529` : close, visible dans la frise (filtre « variantes intégrées »), « Créer une variante à partir d'ici » pour en repartir. |
| m8 (`coverage.md:176`) | oui | cellule N28 réécrite : Relire › Problèmes, Alt+Entrée, F8 ; le popover ne reste que dans la colonne des preuves. |
| m9 (`disabled` et infobulle) | oui | `aria-disabled="true"` et focalisable : frise `:1128-1130`, « Appliquer » `:2524-2526`. |

### Codex n°6, majeurs

| # | Résolu ? | Preuve |
|---|---|---|
| R6-M1 (deux contrats de proposition) | **oui, genuinement** | §4.39 point 2 `:2247-2262` renvoie **entièrement** à §4.41 : groupes focalisables, choix locaux, **un seul « Appliquer »** qui écrit une modification « via IA » ; les collègues ne voient rien avant « Appliquer » ; ce qui reste propre à l'IA est listé (explication, mises en évidence, garde-fous, « Annuler cette discussion » sur les modifications **appliquées**). §4.39 point 11 `:2385-2387` : groupes, **pas de Listbox**. §4.41 `:2531-2534` : un seul contrôle final, « Accepter et appliquer » retiré ; `grep` : la chaîne ne subsiste que dans cette phrase de retrait. `coverage.md:112` (ligne *Listbox*) aligné. L'ancien clavier propre à l'IA (« Entrée accepte ; Suppr refuse ») n'existe plus qu'en §4.41 `:2509`, sous la règle « jamais dans un champ ». Résidu documentaire : m1 ci-dessous. |
| R6-M2 (raccourcis contradictoires) | **oui, genuinement** | **⇧⌘O** « Aller au fichier » en §3.2 `:484` et §8.2 `:3274`, ⌘P = Imprimer `:3275`, aucun autre « ⌘P » ni « Cmd+P » (grep) ; **Aide = déléguée au système** `:3307`, ⇧⌘? reste dans la table réservée `:3317`, K-6 reconnaît les commandes déléguées `:3322-3324` (Aide, Quitter — cohérent avec §8.4 `:3368` : ⌘Q et Alt+F4 « système ») ; **⌥⌘S** panneau, **⌃⌘S** version, **⌘F6 / ⇧⌘F6** Zones `:3304` ; ⌃F6 réservé. Choix propres dits explicites `:3234-3236`. **Relecture de la colonne macOS** : 46 liaisons, **aucun doublon**, **aucune** dans la table réservée (⌃⇧⌘→/← n'est pas ⌃→/← ; ⌃Tab, ⌘F6, ⌃⌘S, ⌥⌘/ absents de [S30]). |

## 2. Contrastes recalculés

- **160 paires, 634 valeurs**, 0 échec hors des paires `forbidden`, **0 écart** avec les valeurs
  déclarées (tolérance 0,01). 90 jetons résolus, aucune valeur brute, aucun `--variant-line`.
- Échantillon (clair · sombre · HC clair · HC sombre) :

| Paire | Clair | Sombre | HC clair | HC sombre |
|---|---:|---:|---:|---:|
| `--fg` / `--view-bg` | 12,57 | 16,82 | 12,57 | 16,82 |
| `--fg-dim-on-state` / `--hover-bg` sur `--view-bg` | 5,86 | 6,91 | 15,17 | 11,44 |
| `--on-dark-fg` / `--destructive-bg` (paire 23, ex-`#ffffff`) | 4,83 | 6,11 | 4,83 | 6,11 |
| `--fg-dim` / `--hover-bg` sur `--popover-bg` (**interdite**, information) | 4,56 | 4,22 | 15,17 | 8,22 |

- **Contre-exemples rejoués contre tout le contrat** (36 paires non interdites touchant
  `--hover-bg`, clair + HC clair) :

| `--hover-bg` clair | `--fg` | `--fg-dim-on-state` | `--accent` (anneau) | HC clair | Verdict |
|---|---:|---:|---:|---|---|
| `#acacac` | 6,94 | **4,45** | **2,67** | `--fg` 6,94 < 7 ; anneau 2,67 | **refusé** (30 échecs) |
| `#bbbbbb` | 7,91 | 4,81 | 3,16 | 9,67 ; 3,16 | **accepté** (0 échec) |
| `#a8a8a8` | 6,69 | 4,35 | 2,55 | 6,69 ; 2,55 | refusé |

Le texte de §2.10 et la ligne TH-D2 (rév. 7) de §10 sont donc exacts, et le seuil est prouvé des
deux côtés (`#acacac` échoue, `#bbbbbb` passe).

## 3. Constats

### Bloquants

Aucun.

### Majeurs

Aucun.

### Mineurs

**m1 — §10, ligne §4.39 (`spec.md:3458`) : « rien ne part chez les collègues avant « Accepter » ».**
Le contrôle final s'appelle désormais « Appliquer » (§4.39 point 2 `:2250`, §4.41 `:2531`) ; un
morceau n'a que des choix locaux. Un test écrit d'après cette ligne vérifierait le mauvais bouton.
Norme : §10 (« une règle sans assertion n'est pas finie »), Nielsen 4. Sévérité : mineure.
Correction : « avant « Appliquer » ».

**m2 — §10, ligne K-6/K-7 (`:3456`) : « aucun raccourci réservé », sans l'exception des commandes
déléguées.** K-6 `:3322-3324` l'ajoute (« que le test reconnaît »), la ligne de test ne la décrit
pas ; telle quelle, l'assertion refuse ⌘Q et l'Aide macOS. Norme : §10 ; Nielsen 4. Sévérité :
mineure. Correction : « aucun raccourci réservé, **sauf** une commande déléguée au système, décrite
sans être capturée ».

**m3 — K-7 dernier point (`:3254-3255`) contre Zones ⌘F6 (`:3304`).** K-7 : « Chaque commande à
touche de fonction a donc une liaison macOS **à lettre** ». Zones n'a que ⌘F6 / ⇧⌘F6, et F6 est une
touche média (Ne pas déranger ou rétroéclairage) sur les portables Apple sans Fn. Aucune collision
avec [S30], mais la règle et la table se contredisent (Nielsen 4). Sévérité : mineure. Correction :
soit écrire l'exception dans K-7 (« Zones : ⌘F6, Fn requis sur un portable, comme F6 de VS Code et
des navigateurs »), soit ajouter une liaison à lettre.

**m4 — §4.22 point 1 (`:1112`) : la parenthèse sur le retrait de `--variant-line` est dans la
liste « les versions nommées se détachent par… », entre « une forme » et « leur nom ».** Elle parle
des variantes, pas des versions nommées ; un lecteur y lit un troisième critère. Norme : §10
lisibilité, Nielsen 4. Sévérité : mineure (éditoriale). Correction : la déplacer dans le point
« Les variantes » `:1114-1116`, ou dans `CHANGES.md` seulement.

## 4. Régressions et calme

- **Aucune régression.** Le diff **retire** (un bouton « Accepter et appliquer », un second contrat
  de proposition, un jeton mort, un bandeau permanent, une ligne doublon de §6.4, une valeur
  brute) ou **précise** (contre-exemples exacts, registre macOS, `aria-disabled`, sort de la
  variante). §3.1 est inchangé ; aucune nouvelle surface, aucun nouveau dialogue, aucun nouveau
  bandeau.
- Ce que §4.39 point 2 perd en texte propre (clavier, « Proposition périmée · Redemander ») est
  couvert par §4.41 (`:2507-2512` clavier hors champ ; `:2535-2537` « Plus à jour », Recalculer
  garde les choix) : la relecture d'une proposition d'IA et celle d'une variante sont désormais le
  **même geste**, ce que Codex demandait et que la règle de cohérence impose.
- **Écriture d'abord, tenu.** Le secours ne réclame plus rien de permanent ; une variante intégrée
  se referme d'elle-même ; les raccourcis macOS suivent le système là où il a une convention et le
  disent explicitement ailleurs.

## 5. Positions sur `CHANGES.md`

- D'accord avec les treize corrections de la rév. 7 telles que décrites ; chacune est vérifiée
  dans le texte (§1). La formule « 160 paires recalculées : aucun échec hors des paires interdites »
  est confirmée par mon propre calcul.

## Verdict

**ACCEPT.** Mon majeur et mes neuf mineurs de la revue n°6 sont fermés avec preuve ; les deux
majeurs de Codex n°6 sont résolus dans le texte, non dans le seul changelog : l'IA passe par la
relecture commune de §4.41 avec un seul « Appliquer », et la table macOS est cohérente (⇧⌘O,
Aide déléguée au système, ⌥⌘S panneau, ⌃⌘S version, ⌘F6 Zones), sans doublon ni réservé. Les 634
valeurs de contraste sont exactes ; `#acacac` est refusé par 30 paires, `#bbbbbb` accepté par
toutes. Aucune régression. Restent quatre mineurs documentaires (deux lignes de tests à aligner,
une exception K-7 à écrire, une parenthèse mal placée), à prendre dans la rév. 8 ou au moment
d'écrire `commands.test.ts` et `e2e/ai.spec.ts`.

Décompte : 0 bloquant, 0 majeur, 4 mineurs.
