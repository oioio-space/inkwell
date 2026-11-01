# Revue Fable n°6 du système de design (révision 6, 2026-11-01) — verdict : **ACCEPT**

Revue indépendante de `spec.md` rév. 6 (`673ac2b`), `CHANGES.md`, `tokens.json`, `coverage.md`,
`inventory.md`, contre les revues n°5 (`07-fable-r5.md`, `08-codex-r5.md`). La revue Codex n°6 n'a
pas été lue.

**Méthode.**
- Chaque constat des deux revues n°5 est rouvert dans le texte de la rév. 6 (§1).
- Les **161 paires** de `tokens.json` sont recalculées par script dans les quatre modes
  (**634 valeurs** ; `hcOnly` seulement en HC ; WCAG 2.2, luminance relative, translucides
  composés en sRGB sur `over`, `hcMin` appliqué comme `themeContract.hcRule` le déclare), puis
  comparées aux valeurs déclarées ; le contre-exemple `#bbbbbb` est rejoué **contre tout le contrat**
  (§2).
- Les tables de raccourcis sont relues par système : doublons dans chaque colonne, K-7, et la
  liste d'Apple (*Mac keyboard shortcuts*, support 102650, consultée en ligne) [S30].
- Le nouveau texte est lu comme une personne qui écrit et n'a jamais ouvert Git (§4).

## 1. Fermeture des constats des revues n°5

### Fable n°5

| # | Fermé ? | Preuve |
|---|---|---|
| M1 (raccourcis par plate-forme) | **oui** | K-7 `spec.md:3230-3240` (aucun Ctrl+Alt sur aucun système ; aucun Alt+alphanumérique sous Windows ; ⌥+lettre jamais seule sous macOS) ; **une liaison par système et par commande** `:3242-3297`, plus de traduction Ctrl→Cmd `:3223-3228` ; les sept Ctrl+Alt réattribués ou retirés (`:3271`, `:3273-3276`) ; trois tables réservées `:3301-3304` (⌥⌘H, ⌃Espace, ⌃⌥Espace, ⌥⌘Esc, ⇧⌘3/4/5, ⌃⌘Q, Ctrl+Alt, Alt+lettre, Alt+Espace, Ctrl+Maj+Échap…) ; suggestions ⌥Échap sous macOS `:3266` ; K-6 sur les trois systèmes `:3306-3310`, test `:3438`. Relecture colonne par colonne : **aucun doublon** dans les trois colonnes ; aucune liaison dans la table réservée de son système. Réserve de libellé : m6 ci-dessous. |
| M2 (portée d'une variante) | **oui** | `spec.md:1156-1158` : « une variante est toujours celle du projet » ; repère `--fg-dim` sur les fichiers modifiés, sous-titre « · 2 fichiers modifiés » `:1164` ; alignement HI-6 déclaré à faire `:1172-1174`, `CHANGES.md:323-326`. |
| M3 (phrase par personne, défaut au partage) | **oui** | `spec.md:2587-2592` (une phrase, créée une fois ; ensuite « Protégé par votre phrase de secours (notée le 3 mars) ») ; défaut **au moment où le projet devient partagé** `:2596-2601`, ligne dans §5.3 `:2760-2763` et §5.4 étape 4 `:2778-2780`, « jamais par un dialogue de plus » ; §4.43 référence unique `:2584-2585`. |
| M4 (résidus) | **oui** | Exporter sans Ctrl+Maj+E `:2872` ; §6.4 : « Restaurer cette version » et « Revenir au présent » autorisés `:3148`, deux entrées « Intégrer » `:3151-3152` ; Q15, Q17, Q18 `:3484`, `:3495-3496` ; « Préférences › Avancé » disparu (grep : « Avancé » ne subsiste que comme sous-groupe replié `:2289`, `:2363`) ; « En cours » et « brouillon » absents de l'interface (grep ; `:2564` est un nom de dossier d'exemple). Résidu de `coverage.md:176` : m8. |
| m1 | oui | `:1125-1127` : le focus ne charge rien ; vue après 300ms de calme, ou Entrée. |
| m2 | oui | `:1141-1143` : « l'aperçu est la confirmation », plus de dialogue, même pour tout le projet. |
| m3 | oui | kit réécrit en silence, avertissement seulement en échec `:2623-2626`, `:2638-2639` ; 24 champs, complétion, collage `:2646-2648`. |
| m4 | oui | voie de variante en `--fg-dim` + nom `:1108-1110` ; version nommée = losange `:1105-1106` ; violet réservé à l'IA. Jeton orphelin : m4 ci-dessous. |
| m5 | oui | sélecteur « Variante » dans l'anatomie du centre `:641-644`, zone protégée, hors des six. |
| m6 | oui | L-8 : « proposition d'intégrer une variante » `:565` ; touches inactives dans un champ `:2511-2512` ; « Git » : règle « sans exception » `:1087-1089`. Réserve : m2 ci-dessous. |
| m7 | oui | six pages, portée par ligne `:2901-2912` ; enregistrement et destination de sauvegarde = « cet appareil ». |
| m8 | oui | `:457` : « au plus une quatrième destination, nommée « Extensions », … que si ». |
| m9 | oui | `:2851` : « Adwaita », « Lecture », aucun contenu fourni. |
| m10 | oui | `clock` `:1092-1093`, `:3320` ; G-101. |
| m11 | oui | « une règle d'Inkwell meilleure que son modèle » `:1083-1085` ; [S28] cite les entrées du carnet. |

### Codex n°5

| # | Fermé ? | Preuve |
|---|---|---|
| R5-B1 (texte secondaire sur survol hors contrat) | **oui, dans le contrat** ; **la preuve citée est fausse** | 30 paires d'états ajoutées (161 au total) : `--fg-dim` **interdit** sur `--hover-bg` et `--selected-bg` sur chaque surface (paires 15, 16, 69, 72, 75, 78, 81, 84), `--fg-dim-on-state` vérifié sur 12 fonds (paires 70, 73, 76, 79, 82, 85, 88, 90, 92, 94, 96, 98 : **4,81 à 6,91** en clair/sombre, 7,52 à 15,17 en HC) ; §4.37 en `--fg-dim-on-state` `:2003` ; combinaisons non vérifiables interdites `:392-393`. **Recalcul avec `--fg-dim-on-state`** (§2) : tout tient. Mais le contre-exemple `#bbbbbb` **n'est pas refusé** par ce contrat : M1 ci-dessous. |
| R5-M1 (secours) | **oui** | parcours « Mettre mon secours en lieu sûr », première fois seulement `:2606-2639` ; trois façons de garder la phrase (écrire, imprimer, fichier) `:2611-2616` ; vérification avec complétion et collage, « ce n'est pas un test de mémoire (3.3.8) » `:2617-2618` ; **récupération par la phrase seule** avec recherche sur les sauvegardes et avertissement de fraîcheur `:2641-2645`, §4.44 `:2686-2688` ; « quand les données sont vraiment perdues » dit exactement `:2650-2652` ; copie lisible durable exclue de la promesse `:2600`, `:2657-2658`. |
| R5-M2 (focus et transaction) | **oui** | morceaux focalisables nommés (`role="group"`, « Morceau 2 sur 5 … ») `:2501-2503` ; entrer (clic, Relire › Activité, F6), focus au premier morceau non choisi `:2498-2500` ; touches **seulement sur un morceau, jamais dans un champ** `:2504-2512` ; sortir par Échap, proposition gardée en attente `:2513-2515` ; **choix locaux, puis une seule modification à « Appliquer »** `:2517-2522` ; un seul « Appliquer », désactivé tant qu'une divergence est sans choix `:2523-2524` ; Tout accepter laisse les divergences `:2525-2526` ; un seul morceau = « Accepter et appliquer », même transaction `:2527-2528` ; choix gardés après recalcul `:2529-2531`. |
| R5-M3 (historique au clavier, Git) | **oui** | options en **texte seul**, focus itinérant (pas `aria-activedescendant`) `:1094-1097` ; **barre d'actions** APG *Toolbar* sur l'option sélectionnée, menu contextuel Maj+F10 `:1113-1122` ; comparaison par **deux sélecteurs** `:1146-1149` ; `aria-valuetext` du curseur `:1132-1135` ; résumés en faits mesurés `:1100-1103` ; export en dépôt retiré de l'interface d'écriture `:1177-1180`, §5.12 `:2888`. Réserve : m2 ci-dessous. |
| R5-M4 (macOS) | **oui** | Remplacer ⌥⌘F (⌘H masque) `:3247` ; suivant ⌘G `:3250` ; aller à la ligne ⌘L `:3262` ; aller au fichier ⇧⌘O, ⌘P = Imprimer `:3260-3261` ; code ⌃⌘E, formule ⇧⌘M `:3252`, `:3254` ; table réservée macOS `:3303` ; G-99. |
| R5-m1 (résidus) | **oui** | §4.36 : « sous-vue de Relire … remplace le popover de la rév. 4 » `:1881-1883` ; « brouillons » et « étape 2 » absents (grep) ; vague 5 de l'inventaire `inventory.md:240` ; §4.40 sans « En cours ». |

## 2. Contrastes recalculés

- **161 paires, 634 valeurs** (les trois paires `hcOnly` ne sont évaluées qu'en HC). **0 échec** hors
  des paires `forbidden` ; **0 écart** entre valeur recalculée et valeur déclarée (tolérance 0,01).
- Échantillon demandé, avec `--fg-dim-on-state` (clair · sombre · HC clair · HC sombre) :

| Paire | Clair | Sombre | HC clair | HC sombre |
|---|---:|---:|---:|---:|
| `--fg-dim-on-state` / `--hover-bg` sur `--view-bg` | 5,8639 | 6,9102 | 15,1700 | 11,4435 |
| … sur `--popover-bg` | 5,8639 | 5,2980 | 15,1700 | 8,2246 |
| … sur `--sidebar-bg` | 5,5069 | 5,7903 | 12,9980 | 9,1736 |
| … sur `--dialog-bg` | 5,7784 | 5,2980 | 14,6156 | 8,2246 |
| `--fg-dim-on-state` / `--selected-bg` sur `--sidebar-bg` | 5,3618 | 5,3739 | 12,2140 | 8,3692 |
| … sur `--popover-bg` | 5,7204 | 4,9250 | 14,2518 | 7,5220 |
| … sur `--dialog-bg` | 5,6344 | 4,9250 | 13,7313 | 7,5220 |
| `--fg-dim` / `--hover-bg` sur `--popover-bg` (**interdit**) | 4,5612 | **4,2185** | 15,17 | 8,22 |
| `--fg-dim` / `--hover-bg` sur `--sidebar-bg` (**interdit**) | **4,3492** | 4,5562 | 13,00 | 9,17 |

Les deux défauts annoncés par `CHANGES.md` (4,22 et 4,35) sont confirmés, et le remède (62 % / 66 %)
tient partout avec ≥ 0,42 de marge sur le pire cas sombre (4,92). TH-4 : inchangé depuis la rév. 5.

## 3. Constats

### Bloquants

Aucun.

### Majeurs

**M1 — Le contre-exemple `#bbbbbb` de §2.10 et §10 est accepté par le contrat tel qu'écrit : la
preuve de clôture de R5-B1 est fausse.**
- *Norme.* §10 (« une règle sans assertion n'est pas finie ») ; TH-D2 ; WCAG 1.4.3 ; principe
  « preuve mesurée » (`engineering-principles.md`).
- *Preuve.* `spec.md:388-390` et `:3436` : « `--hover-bg` `#bbbbbb` en clair (texte principal à
  7,91:1, mais **secondaire à 3,92:1**) refusé ». Or 3,92 est le rapport de `--fg-dim`, que la
  rév. 6 rend **interdit** sur `--hover-bg` (paires 69–84, `forbidden: true`), donc **non évalué**
  (TH-D2 : « toutes les paires non interdites »). Rejeu du thème contre tout le contrat
  (36 paires touchant `--hover-bg`, clair et HC clair) : `--fg` 7,91, `--fg-dim-on-state` **4,81**
  (≥ 4,5), `--accent` 3,16 (≥ 3), HC 9,67 : **aucun échec, thème accepté**. Le test `theme/contract.test.ts`
  décrit en `:3436` échouerait tel quel.
- *Correction.* Le contrat est juste (la classe de défaut de Codex est bien couverte par les paires
  `--fg-dim-on-state`) ; seule la preuve est fausse. Remplacer le contre-exemple par un gris qui
  échoue réellement : `--hover-bg` **`#acacac`** en clair donne texte principal **6,94:1** (passe),
  secondaire `--fg-dim-on-state` **4,45:1** (échoue), accent **2,67:1** (échoue) ; ou `#a8a8a8`
  (6,69 / 4,35 / 2,55). Écrire `#bbbbbb` comme **cas accepté** (4,81), ce qui prouve que le seuil
  est au bon endroit. Trois lignes (`:388-390`, `:3436`, `CHANGES.md:281`).

### Mineurs

**m1 — `tokens.json` paire 23 : `fg: "#ffffff"`, une valeur brute dans le contrat.** T-1 (« aucune
valeur brute ») ; TH-D1 (le validateur résout des **jetons**, pas des couleurs). C'est la pastille
« Mode développeur » (`spec.md:1724`). Employer `--on-dark-fg` (blanc, déjà verrouillé).

**m2 — Le mot « Git » reste dans l'interface : Préférences › Application › Mode développeur ›
Outils.** `spec.md:1087-1088` (« sans exception »), `:1177-1180` et `:1720-1721` (« Exporter
l'historique en dépôt Git » dans les outils) ; `CHANGES.md:303` dit « n'apparaît plus du tout » ;
le test `:3446` (« aucun mot de Git dans le DOM ») échoue dès que le mode développeur est actif.
Nielsen 4 ; N39. Correction : « Exporter l'historique (format technique)… », le mot « Git » sous
« Détails techniques » seulement ; et le test précise « hors mode développeur ».

**m3 — §6.4 a deux lignes « retour en arrière »** (`spec.md:3148` et `:3169`), avec deux listes
« Jamais » différentes (rollback/revert/checkout contre commit/checkout/revert/HEAD). Nielsen 4.
Une seule ligne, union des deux listes.

**m4 — `--variant-line` est un jeton orphelin.** `tokens.json:961-967` et paire `:4052`
subsistent alors que §4.22 `:1108-1109` met la voie de variante en `--fg-dim`. Le test de parité
T-5 (`:3418`) le conservera comme CSS mort. Retirer, ou le réserver explicitement.

**m5 — §4.43 point 5 : un bandeau d'information permanent tant que le secours est incomplet, y
compris quand seul le kit (« conseil, pas d'obligation ») manque.** `spec.md:2629-2637`. P1 (calme) ;
L-8 (un seul bandeau : celui-ci occupe l'emplacement en permanence chez qui a dit « Plus tard ») ;
Nielsen 8 ; HIG *Banners* [S2] (« should not be used for information that is permanently
relevant »). Correction : bandeau seulement quand la **phrase** n'est ni notée ni vérifiée ;
fermable, revient au lancement suivant après 7 jours ; l'état du kit ne vit que dans Préférences et
le popover d'état. Ajouter l'état manquant « Phrase **pas encore notée** · Noter » (celui de « Plus
tard » à l'étape 2 : « non vérifiée · Vérifier » suppose qu'on l'a vue).

**m6 — §8.2 : « ⌃⌘S (barre latérale, convention) » n'est pas la convention d'Apple.** [S30], vérifié
en ligne : **⌥⌘S** = « Hide or show the Sidebar in Finder windows » ; ⌃⌘S n'y figure pas. La
rév. 6 donne ⌥⌘S à « Enregistrer une version » (`:3272`). Aucune collision avec le système, mais un
libellé faux (Nielsen 4). Soit échanger (panneau ⌥⌘S, version ⌃⌘S), soit retirer le mot
« convention ». À ajouter aux réservés macOS, sans collision aujourd'hui : **⌃⌘D** (définition du
mot sélectionné) et **⌥⌘T** (barre d'outils des apps de texte).

**m7 — §4.41 / §4.22 : après « Appliquer » d'une intégration, le sort de la variante n'est pas
dit** (reste-t-elle une voie ? est-elle close, comme chez Patchwork ?). Nielsen 1, 6. Une phrase :
« Après l'intégration, la variante est **close** et reste visible dans la frise, filtre « variantes
intégrées » ».

**m8 — `coverage.md:176` : la cellule N28 garde, après la note « rév. 6 », le texte « liste
Problèmes (**popover existant**) … **F8 et Ctrl+.** ».** Deux règles contraires dans une même
cellule (C13 ; `spec.md:1825` interdit Ctrl+.). Nielsen 4. Ne garder que le texte de la rév. 6.

**m9 — §4.22 point 2 : actions « désactivées, avec une infobulle « Choisissez une version » ».**
Un bouton `disabled` n'est pas focalisable : l'infobulle est inaccessible au clavier (WCAG 1.4.13 ;
APG *Toolbar*). `spec.md:606` admet déjà `aria-disabled` : le dire ici.

## 4. Régressions et calme

- **Aucune régression.** §3.1 inchangé (deux onglets, trois destinations, aucun tiroir à
  l'ouverture, aucun compteur dans l'en-tête) ; les nouveautés de la rév. 6 **retirent** des choses
  (trois raccourcis Ctrl+Alt sans remplaçant, un dialogue de restauration, le toast de kit, la
  cérémonie répétée, l'export Git des menus) ou en précisent (barre de relecture, barre d'actions de
  la frise). La barre de relecture n'existe que pendant une proposition, au plus une par document
  (`:2534-2535`).
- **Écriture d'abord, tenu.** Le secours est devenu un parcours en cinq pas, une seule fois,
  avec « Plus tard » partout et « une personne du projet joignable suffit » ; la récupération par
  la phrase seule retire la dernière raison de craindre le chiffrement. La relecture d'une
  proposition se lit comme une correction d'épreuves (morceau, accepter, refuser, appliquer), sans
  notion de transaction visible. L'historique se parcourt aux flèches sans rien charger, et se
  compare avec deux listes « De / À » : pas de Maj+clic à deviner.
- **Sur-ingénierie.** Rien de nouveau ; m5 est le seul point où la rév. 6 en ajoute (un bandeau
  permanent).

## 5. Positions sur `CHANGES.md`

- Barre de mise en forme fixe (T11), Ressources en fenêtre sous 900px dans la fenêtre, Ctrl+Espace
  par système (⌥Échap, Ctrl+J), Ctrl+; abandonné : **d'accord**, identiques à ma position n°5.
- Variante du projet (HI-6) et export Git (HI-5) à aligner dans l'architecture : **d'accord** ;
  m2 précise le libellé à retenir pour HI-5.

## Verdict

**ACCEPT.** Les quatre majeurs et les onze mineurs de ma revue n°5 sont fermés avec preuve ; les
cinq bloquants et majeurs de Codex n°5 sont fermés dans le texte — contraste (634 valeurs exactes,
`--fg-dim-on-state` ≥ 4,81 partout), récupération par la phrase seule, focus et « Appliquer »
unique, historique au clavier sans mot de Git dans l'interface d'écriture, raccourcis macOS sans
collision. Aucune régression ; le produit reste calme et tourné vers l'écriture. Un seul majeur,
documentaire : le contre-exemple `#bbbbbb` est accepté par le contrat qu'il prétend faire échouer
(M1, remplacer par `#acacac`) ; à corriger avec les neuf mineurs dans la rév. 7, avant d'écrire
`theme/contract.test.ts`.

Décompte : 0 bloquant, 1 majeur, 9 mineurs.
