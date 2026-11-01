# Revue Fable n°3 du système de design (révision 3, 2026-11-01) — verdict : **ACCEPT**

Revue indépendante de `spec.md` rév. 3, `coverage.md`, `inventory.md`, `tokens.json`,
`CHANGES.md`, contre les revues n°2 (`03-fable-r2.md`, `04-codex-r2.md`), `docs/exigences.md`
N17–N29, `docs/architecture/spec.md` §15 (§15.4 à §15.16), l'étude
`docs/architecture/research/editor-features.md`, et le code de `frontend/src` (`feat/protocol-module`,
`3cc926c`). N30 et N31 sont hors périmètre ; §5 note les crochets qu'elles exigeront.

**Méthode.**
- Chaque constat des deux revues n°2 est vérifié dans le texte, puis dans le code (§1).
- Les 101 paires de `tokens.json` sont recalculées par script dans les **quatre** modes
  (404 valeurs ; WCAG 2.2, luminance relative, translucides composés en sRGB sur `over`, HC =
  `modes.hc-*` sinon le mode clair ou sombre, comme `contrastMethod` le déclare) ; la distance
  ΔE2000 des huit couleurs de personnes est calculée (§2).
- §4.31 est confronté ligne à ligne à l'architecture §15.4 et §15.13 ; §2.10 à §15.8 ; §4.32 à
  §15.10 ; §4.33 à §15.11 ; §4.34 à §15.14 ; §4.35 à §15.15.
- Les raccourcis nouveaux (F8, Ctrl+., Ctrl+Espace, Ctrl+Maj+Espace, F12) sont vérifiés contre
  la HIG *Keyboard* et les liaisons par défaut de GTK, la plate-forme cible (ADR 0003).
- Chaque preuve « existant » de §10 et chaque ligne **S** de `coverage.md` est ouverte.

## 1. Fermeture des constats de la revue n°2

### Fable n°2

| # | Fermé ? | Preuve |
|---|---|---|
| B1 (N22) | **oui** | `spec.md:323-378` (§2.10 : TH-D1 trois classes, TH-D2 validation, TH-D3 Apparence, priorité, projet sans thème imposé) ; `tokens.json` `$extensions.inkwell.themeContract` (version 1.0, `missingToken`, `unknownToken`, `validation`, `hcRule`, `personRule`, `forcedColors`) ; `contract` sur 142 jetons sur 147 (80 `overridable`, 43 `locked`, 19 `derived` ; les 5 sans contrat : m2) ; les pseudo-paires « (contraste élevé) » sont retirées et `--fg-dim` HC passe par `modes.hc-light` / `hc-dark` (paire 6 : 17,42 / 13,78, recalculées exactes) ; `coverage.md:166` ; `inventory.md:159` (G-71) ; `spec.md:2020` (§5.13). Reste un trou : la paire `--border` HC a disparu au lieu de devenir calculable (M3), et la règle HC est trop stricte (M1). |
| B2 (§15.4) | **oui** | `spec.md:1394-1416` : 21 lignes, dans l'ordre de §15.4 (22 points, stockage et miroir réunis) ; les **deux** sélecteurs (ligne 4, L-2) ; attribution, état désactivé / planté et interdits par point ; surfaces de §15.13 dans les lignes 2, 4, 5, 10, 21 ; `coverage.md:119` (ligne APG « Extensions »). Vérifié point par point : plafonds identiques (≤ 8 entrées par menu, ≤ 3 éléments de barre d'état, ≤ 2 gouttières, ≤ 4 cartes, ≤ 6 épinglés en H-2 `spec.md:599`). |
| M1 (G-0) | **oui** | `inventory.md:50` « **Ouvert** » ; `coverage.md:151`, `:157` ; `CHANGES.md:14`, `:94-97` (la mention « corrigé » est déclarée fausse) ; `spec.md:898-914` critère d'acceptation ; `App.svelte:197` toujours `onOpen={() => session.focus()}` et `Sidebar.svelte:49` transmet `node.path` : l'état « ouvert » est exact. |
| M2 (preuves) | **oui** | `spec.md:2457-2494` colonne **État** ; chaque « existant » ouvert : `tokens.test.ts:44-50` ✓, `e2e/shell.spec.ts:24`, `:151`, `:253` ✓ (UI-03, UI-07, UI-06), `e2e/sheet.spec.ts:613` ✓ (CSV-20), `src/ui/i18n.test.ts` et `src/i18n/i18n.test.ts` ✓, `showModal()` aux sept lignes citées ✓, `prefers-reduced-motion` à `HeaderBar.svelte:293`, `Preview.svelte:306`, `Grid.svelte:1025` ✓. `coverage.md` : chaque **S** porte une preuve existante (1.3.5 `ShareDialog.svelte:43` ✓ ; 2.1.4 `present/keys.ts:61-69` ✓ ; 3.3.2 `ui/search/SearchField.svelte:55-69` ✓ `role="searchbox"` + `aria-label`) ; 3.2.2 → S→N (`PeopleDialog.svelte:57` `notice.show(done)` sans « Annuler » ✓), *Popovers* → S→G (`MenuButton.svelte:54` ✓), *Performance perçue* → S→N ✓. |
| m1 | oui | `spec.md:103-105` ; `tokens.json#/contrast/$description` : usage interdit par politique, test d'absence dans le code. |
| m2 | oui | `spec.md:99-100`, `:202-204` : hexadécimaux canoniques ; recalcul depuis le JSON : 0 écart (§2). |
| m3 | oui | `inventory.md:113` (`workspace/workspace.svelte.ts:120`), `:98` (`ui/icons/FileIcon.svelte:29-30`), `:93` (`ui/shell/StatusBar.svelte`), `:134` (G-61 : `fr.json:549`, `:618`, `:621`, `:622`, `:656`, `:663`, tous vérifiés : « fusion », « fusionnée », « Fusionner », « la fusionner »). |
| m4 | oui | `spec.md:1045-1046`, `:2329` (A-20), `coverage.md:65` : B, W, L, vue de présentation seulement. |
| m5 | oui | `spec.md:127` : infobulle sombre sur vue sombre, 20,24. |
| m6 | oui | `spec.md:2025`, `coverage.md:84`, `inventory.md:167` (G-79). |
| m7 | oui | `spec.md:823-827` : exemption explicite (HIG *Search*), preuve `SearchField.svelte:55-69`. |
| m8 | oui | FT-3 `spec.md:1176-1179` (séparateur et en-tête, rien de plus) ; TO-1 `spec.md:805-806` (« Aucune phrase de plus dans le toast »). |

### Codex n°2

| # | Fermé ? | Preuve |
|---|---|---|
| R2-B1 | **oui** | comme B1 ; en plus `themeContract.version`, `unknownToken`, priorité `spec.md:370-374`, retour au thème sûr `spec.md:365-369`, composition des translucides `contrastMethod`. |
| R2-B2 | **oui** | comme M1. |
| R2-M1 | oui | légende stricte `coverage.md:13` ; Commentaires « H, et S→G sur l'onglet » `coverage.md:177`, `HistoryDrawer.svelte:206-207` ✓ ; performance S→N `coverage.md:163` (`store/limits.ts:15-26` ✓ : paliers, pas de mesure). |
| R2-M2 | oui | L-5 `spec.md:477-487` : retour à la ligne forcé sous 560px, exemption limitée au 2D, test sur les quatre formats ; `editor/setup.ts:122` `EditorView.lineWrapping` ✓. |
| R2-M3 | oui | replis ordonnés `spec.md:570-579` ; onglets : activation manuelle, `aria-selected`, `tabpanel` unique, focus après le dernier onglet `spec.md:1088-1107` ; séparateurs : nom, orientation, `aria-valuenow` `spec.md:452-461` ; G-78. |
| R2-M4 | oui | « Retirer » garde les données, « Effacer ses données… » confirmé, « Récemment retirées » `spec.md:1294-1302` ; TO-1 étendu `spec.md:802-803`. |
| R2-M5 | oui | stockage, miroir, migration, arrêt, Activité `spec.md:1331-1349` ; consentement avec les 13 phrases de §15.5 (comparées une à une : identiques), automatique mis en avant, « Tout voir » `spec.md:1307-1329`. |
| R2-M6 | oui | neuf états et revérification à l'envoi `spec.md:543-566` ; états du gestionnaire `spec.md:1275-1290` ; nom à l'accueil `spec.md:1851-1857`. |
| R2-M7 | oui | comme m1. |
| R2-M8 | oui | une action par toast `spec.md:789-792` ; export « Ouvrir », « Afficher dans le dossier » dans le menu `spec.md:2003-2006` ; intégration : lignes teintées 12s au lieu de « Voir » `spec.md:1958-1960` ; L-8 : critique, questions, états, information ; « +1 question » `spec.md:502-522`. |
| R2-m1 | oui | budget 60 Ko / 100ms / 5ms `spec.md:1234-1241` ; T-1 TS limité aux couleurs et ombres `spec.md:78-81` ; densité `spec.md:1426-1427`. |
| R2-m2 | oui | `spec.md:2498-2502` « Rien n'est ratifié » ; `ux-principles.md` et `glossary.md` inchangés (vérifié : règle 3 dit toujours Ctrl+K, le glossaire dit toujours « brouillon (v2) »). |

**Tous les constats des deux revues n°2 sont fermés.** Les deux bloquants le sont avec preuve.

## 2. Recalcul des contrastes et du contrat

Script : `scratchpad/contrast.py` (lecture de `tokens.json`, résolution par mode, composition
alpha sur `over`, WCAG 2.2 ; ΔE2000 en Lab D65).

- **404 valeurs sur 404 identiques à ±0,02** aux valeurs déclarées ; aucune paire non interdite
  sous son `min` dans aucun des quatre modes ; aucune translucide sans `over`.
- Échantillon : `--fg`/`--view-bg` 12,57 · 16,82 · 12,57 · 16,82 ; `--fg-dim`/`--view-bg` 4,74 ·
  5,93 · **17,42** · **13,78** ; `--fg-dim`/`--sidebar-bg` 4,54 · 5,24 · 14,92 · 11,25 ;
  `--fg-dim`/`--info-bg` (interdite) 4,56 · 3,95 · 15,11 · 7,58 ; `--accent`/`--stage-bg`
  (interdite) 3,46 · 12,40 ; `--person-4-line` 3,14 ; `--person-3/6/7/8-line` 6,90 · 7,64 · 6,29 ·
  6,01 depuis les hexadécimaux canoniques (m2 de la revue n°2 fermé).
- **Couleurs de personnes** : ΔE2000 minimal entre deux `--person-N` d'Adwaita = **21,3**
  (personnes 2 et 8), puis 21,7 (1 et 4), 22,2 (1 et 6). Le seuil TH-D2 ≥ 15 est donc tenu par
  le thème par défaut lui-même : la règle est satisfiable.
- **Anomalies :**
  - la paire **36 est un doublon** de la paire 6 (`--fg-dim`/`--view-bg`, `over` nul, min 4,5 ;
    seule la note « HC inclus par modes » diffère) : 101 paires déclarées, 100 distinctes ;
    `spec.md:112` dit encore « 102 paires » (m1) ;
  - **aucune paire ne porte `--border`** : la revue n°2 (B1-c) demandait que la paire HC
    `--border`/`--view-bg` devienne calculable par `modes` ; elle a été supprimée. Recalcul :
    `--border` sur vue = 1,41 · 1,60 · **3,96** · **5,14**, sur barre latérale 1,41 · 1,61 ·
    3,83 · 4,61. La valeur HC de T-3 (`spec.md:91`) n'est plus gardée par le contrat (M3) ;
  - `hcRule` (« ≥ celui d'Adwaita pour la même paire ») : un thème « papier » qui met
    `--view-bg` à `#f5f0e6` et ne fournit pas de `hc-light` obtient, pour `--fg-dim`/`--view-bg`
    en HC clair, **15,56 < 17,42** → thème refusé en entier (M1).
- Budget de validation (≤ 20ms, §15.4) : 404 rapports et 28 distances ΔE, trivialement tenu.

## 3. Constats

### Bloquants

Aucun. Les deux bloquants de la revue n°2 sont fermés ; aucun échec WCAG 2.2 AA ni parcours
principal cassé n'est introduit par la révision 3.

### Majeurs

**M1 — La règle « HC jamais affaibli » rend les modes HC pratiquement inthémables, et le refus
est trompeur.**
- *Norme.* N22 (« thèmes modifiables ») ; architecture TH-2 (`architecture/spec.md:1529` : « ne
  peuvent pas être affaiblis ») ; WCAG 1.4.6 (AAA : 7:1 texte, 4,5:1 grand texte) ; HIG
  *Accessibility* (contraste élevé) ; Nielsen 9 (message d'erreur exact).
- *Preuve.* `spec.md:339-340` et `tokens.json` `themeContract.hcRule` comparent **paire par
  paire au rapport d'Adwaita**. Les rapports HC d'Adwaita sont proches du maximum (17,42, 16,78,
  14,92… sur un plafond de 21) : toute surface qui n'est ni blanche ni noire échoue. Calcul :
  `--view-bg` `#f5f0e6` → 15,56 contre 17,42 ; le message « Pas assez lisible : 1 couleur manque
  de contraste » (`spec.md:360-361`) est faux pour un rapport de 15,56.
- *Correction.* Lire TH-2 comme un **plancher plus haut**, pas comme une comparaison : en
  `hc-light` et `hc-dark`, texte ≥ 7:1, composants et anneau ≥ 4,5:1 (1.4.6 et la pratique de
  libadwaita HC : `--fg-dim` 90 %, `--border` 50 %). Garder la comparaison à Adwaita pour les
  seuls jetons `locked` (déjà impossibles à changer). Une phrase dans TH-D1, une valeur
  `hcMin` par paire dans `tokens.json`, un cas de `theme/contract.test.ts` (« thème papier
  accepté ; thème qui met `--fg-dim` HC à 55 % refusé »).

**M2 — Ctrl+. (corrections rapides) est pris par GTK sur la plate-forme cible.**
- *Norme.* K-6 (`spec.md:2393`), ND-3 (aucun raccourci pris en silence), HIG *Keyboard* ; ADR
  0003 (Linux GTK4 / WebKitGTK 6.0).
- *Preuve.* `spec.md:1628-1629` et `:2367` déclarent Ctrl+. « libre dans le registre ». Le
  registre ne voit que l'application : `GtkText::insert-emoji` est un signal de liaison de
  touches dont « les liaisons par défaut sont **Ctrl+.** et Ctrl+; »
  (https://docs.gtk.org/gtk4/signal.Text.insert-emoji.html). Dans WebKitGTK sous GTK4, c'est
  GTK qui intercepte ce raccourci avant WebKit (bug 256080, ouvert :
  https://bugs.webkit.org/show_bug.cgi?id=256080 ; Epiphany #2083) : le sélecteur d'émojis
  s'ouvre, mal placé, et la page ne reçoit rien. `preventDefault` ne peut pas l'empêcher.
- *Correction.* Choisir un raccourci hors des liaisons GTK : **Alt+Entrée** (convention JetBrains
  pour « intentions », cohérent avec l'écart Alt déjà assumé pour Alt+← et Ctrl+Alt+J), ou
  Ctrl+Maj+. ; l'ampoule et le menu restent. Ajouter au registre (K-6) une **liste de
  raccourcis réservés par la plate-forme** (Ctrl+., Ctrl+;, Super+*, les raccourcis de GNOME
  Shell, et Ctrl+Espace « à vérifier » : c'est l'ancien déclencheur d'IBus, encore configuré
  chez beaucoup de personnes qui écrivent en CJK) ; le test K-6 refuse tout doublon avec elle.

**M3 — La paire `--border` en contraste élevé a été supprimée au lieu d'être rendue
calculable : le contrat ne protège plus les filets en HC.**
- *Norme.* WCAG 1.4.11 (bordures de champs et de composants ≥ 3:1) ; T-3 (`spec.md:91` :
  `--border` à 50 % en HC) ; TH-2 ; revue n°2 B1-c.
- *Preuve.* `tokens.json#/contrast/pairs` : aucune paire dont `fg` est `--border` (la révision 2
  en avait une, index 101 : 3,96 / 5,14). `--border` est `overridable` : un thème peut donc
  livrer un `hc-light` à 15 % sans qu'aucune paire ne le refuse, alors que les champs (§4.13),
  les lignes de liste et les séparateurs (§3.3, filet de 1px) reposent sur lui en HC.
- *Correction.* Rétablir trois paires `--border` sur `--view-bg`, `--sidebar-bg`, `--dialog-bg`,
  `min: 3`, avec un champ `hcOnly: true` (en clair et en sombre, le filet d'Adwaita est
  décoratif et les champs sont identifiés par leur fond : exemption 1.4.11, à écrire dans §2.2).

### Mineurs

**m1 — `tokens.json` : doublon et compte.** Paire 36 = paire 6 ; `spec.md:112` dit 102 paires,
le fichier en a 101, 100 distinctes. Supprimer la 36, corriger le nombre.

**m2 — Contrat : cinq jetons sans classe, et une contrainte implicite.**
- Sans `contract` : `--text-editor`, `size/window-min`, `--tooltip-delay`, `--spinner-delay`,
  `motion/toast-timeout`. TH-D1 dit « durées normales » remplaçables : les trois délais doivent
  être classés (`locked` : ce sont des seuils d'accessibilité, 1.4.13 et §4.21).
- `--on-dark-fg` est `locked` (blanc) alors que `--toast-bg`, `--tooltip-bg` et `--stage-bg`
  sont `overridable` : les paires 46 à 51 refusent donc tout thème à toasts, infobulles ou
  scène clairs. C'est défendable (HIG : toasts sombres), mais ce n'est écrit nulle part. Soit
  l'écrire dans TH-D1 (« toasts, infobulles et scène restent sombres »), soit dériver
  `--on-dark-fg` comme `--person-N-ink` (noir ou blanc, le plus contrasté). `--tooltip-fg`
  (blanc, `overridable`) doit suivre la même classe que `--on-dark-fg`.

**m3 — Références de code à corriger.**
- `spec.md:1705` et `inventory.md:174` (G-81) : le message est tronqué à **4 000** caractères,
  pas 400 (`preview/protocol.ts:135` : `v.message.slice(0, 4000)`).
- `spec.md:1652` cite `editor/csv/dialect.ts` pour les problèmes CSV ; la source existante est
  `editor/csv/problems.ts` (`findProblems` : guillemet non fermé, guillemet isolé, `\"`), avec la
  ligne seulement, sans colonne : à citer, et à ranger sous G-81.

**m4 — §4.29, paquet incompatible.** `spec.md:1283` ne donne ni « Détails » (l'architecture PV-1
nomme les deux versions : `architecture/spec.md:1919-1921`), ni le cas inverse d'une extension
**trop ancienne** (majeure N−2, PV-4). Ajouter « Détails techniques » avec les versions, et la
phrase « Cette extension est faite pour une version plus ancienne d'Inkwell ; demandez une mise à
jour à son auteur. »

**m5 — §4.34, texte de remplacement du diagramme hors de l'éditeur.** `spec.md:1530-1531` nomme
l'image pour le lecteur d'écran dans l'éditeur ; rien n'est dit pour l'**aperçu** et le **PDF**
(1.1.1 ; PDF/UA `AD-44`). Le paquet `inkwell-diagrams` doit appeler `image(..., alt:)` avec la
première ligne du bloc, ou la légende s'il y en a une ; `coverage.md:47` renvoie à §4.34 pour
1.1.1 et ne couvre donc que l'éditeur.

**m6 — §4.35, le mode débogage par extension n'a pas d'interface.** L'architecture
(`architecture/spec.md:1864-1875`) enregistre **les données de documents** dans un anneau ≤ 64 Mio,
impose un bandeau « Débogage de X » et prévoit `--redact`. §4.35 n'en montre rien : ni
l'activation par extension, ni l'indicateur d'enregistrement, ni « Expurger avant de partager ».
Nielsen 1 ; O-2. Une ligne dans l'Inspecteur suffit : « Débogage : enregistrement en cours
(12 Mo) · Arrêter · Exporter une trace expurgée… ».

**m7 — §4.36, le « bouton partagé » du compte de problèmes.** `spec.md:1614` : APG n'a pas de
motif *split button*. Préciser **deux arrêts de tabulation**, deux noms (« Aller au premier
problème » ; « Problèmes, liste ») et le motif *Menu Button* pour la flèche ; ST-1 (zone active
de 34px pour chacun). Q17 reste à confirmer.

**m8 — §5.19, sur-ingénierie, à simplifier.**
- « Partager avec l'équipe… : si l'on n'y a pas le droit d'écrire, une **demande** part aux
  admins » (`spec.md:2226`) invente une troisième sorte de demande (après l'accès §5.4 et la
  suppression §5.6) sans décision de protocole. À la place : le bouton n'apparaît qu'avec le rôle
  Écriture dans le projet de modèles ; sinon une phrase : « Demandez le rôle Écriture à un admin
  de « Équipe ACME » ».
- « Voir les projets créés avec » (`spec.md:2227`) exige de tracer la provenance dans chaque
  projet. PEUT, v2.
- « la corbeille garde le modèle 30 jours » (`spec.md:2223`) : la corbeille des fichiers (§5.6)
  n'a pas de durée. Une seule règle de corbeille, dans §5.6.

**m9 — §4.32 et §4.31, deux ambiguïtés.**
- Bandeau « Ce projet utilise 2 extensions ajoutées par Marie. · Voir · Désactiver pour moi »
  (`spec.md:1434-1435`, `:1445`) : « Désactiver pour moi » porte sur lesquelles ? Dire « les
  deux », ou ne garder que « Voir » dans le bandeau et l'interrupteur par ligne.
- §4.31 ligne 21 place « au plus 4 cartes « Démarrer avec » » sur l'**accueil**, alors que
  l'accueil de §5.1 est le premier lancement (aucune extension encore) et que les modèles vivent
  dans « Nouveau projet » (§5.2). Préciser : dans le dialogue « Nouveau projet », sous la galerie.

**m10 — Petits écarts de cohérence.**
- §4.37 « Entrée **ou Tab** insèrent » (`spec.md:1804`) : Tab insère aussi dans la palette ?
  §4.24 ne le dit pas. Un seul comportement pour les deux *combobox* (K-5).
- §6.4 interdit « thème » pour la charte (`spec.md:2295`) mais `spec.md:2128` liste « le type »
  d'un modèle avec « rapport », « lettre », « notes » en minuscules, et §5.19 « Document
  officiel » en majuscule : fixer la casse (§6.2).

## 4. Lecture de `coverage.md`

- **Nielsen 1–10** : les renvois ajoutés (§4.29, §4.31, §4.36, §5.19, §4.35) sont réels.
- **WCAG 2.2** : 55 critères ; chaque **S** a sa preuve existante, ouverte et exacte (§1). Les
  corrections demandées : 1.1.1 (m5), 1.4.11 (M3 : ajouter la paire `--border` HC à la preuve).
- **APG** : ligne « Extensions » présente ; *Combobox* couvre §4.24 et §4.37 ; m7 pour le compte.
- **HIG** : exactes ; *Popovers* en S→G, correct.
- **Domaines** : N22 à N29 ont chacune leur ligne, renvoyant à une section réelle et à un écart
  G-71 à G-84 daté. Rien ne manque pour le périmètre de la rév. 3.

## 5. Crochets à prévoir pour N30 et N31 (rév. 4), sans rien spécifier ici

- **N30.** Le modèle de diagnostic unique (§4.36) a trois gravités ; l'étude (§9.1) veut des
  **suggestions** d'orthographe et de style qui ne comptent **pas** dans la barre d'état et ne
  sont jamais rouges. Réserver dès maintenant une quatrième gravité « suggestion » (pointillé
  neutre, jamais comptée), et un onglet « Relecture » dans le **tiroir** (L-2 : « le temps et les
  autres à droite »). Le dictionnaire du projet (W1) et les règles typographiques (W2) sont des
  données de projet : ADR 0030 (décisions) ou texte partagé, à trancher en N18.
- **N31.** Trois points heurtent l'existant et demandent une ADR avant tout design : (1) la
  **règle d'exclusion** de §15.5 interdit à une extension avec `net.hosts` de lire du clair, ce
  qui interdit *par construction* une IA qui envoie le texte à un serveur (DGX interne compris) ;
  (2) un serveur MCP est un serveur local (invariant : `127.0.0.1`, port éphémère, jeton) : sa
  présence doit être visible comme le mode développeur (pastille, L-8d) ; (3) une IA qui modifie
  « directement » passe par `document.propose` et « via X » (ND-8), et le mode *Suggestion* de
  l'étude dépend des brouillons (v2). Le consentement (§4.29) aura besoin d'une phrase mise en
  avant : « envoyer votre texte à dgx.example ».

## 6. Ce qui est bon, et qu'il ne faut pas rouvrir

- §2.10 et `tokens.json` forment un vrai contrat calculable : classes, bornes, dérivés, modes HC
  par jeton, refus complet, retour au défaut ; 404 valeurs exactes.
- §4.31 est aligné ligne pour ligne sur §15.4 et §15.13, avec une règle d'attribution par point
  et les deux sélecteurs : c'est ce que la revue n°2 demandait, et c'est simple.
- §4.33 (zones protégées, dialogues inimitables, conflits montrés, mode sans extensions avec
  pastille) et §4.30 (la personne épingle) tiennent N24 sans verrue.
- §4.36 réutilise le popover existant de la barre d'état et `@codemirror/lint` ; §4.37 réutilise
  tinymist et `@codemirror/autocomplete` ; les deux restent calmes à la frappe (1s, 300ms, jamais
  sur une lettre isolée). C'est un outil d'écriture, pas un IDE : le ton de N29 est juste.
- §5.19 : aucun modèle fourni, format `typst.toml` de Typst, charte en paquet local versionné,
  copie à la création : simple et prévisible (hors m8).
- §10 distingue enfin « existant » et « à écrire », et `coverage.md` ne ment plus.

## Verdict

**ACCEPT.** Les deux bloquants de la revue n°2 sont fermés avec preuve ; les 404 contrastes
sont exacts ; l'alignement avec l'architecture §15 est vérifié point par point. Les trois majeurs
(M1 règle HC, M2 Ctrl+., M3 paire `--border` HC) sont des corrections locales d'une phrase ou
d'une paire, sans incidence sur l'architecture : à faire dans la révision 4, avec les dix
mineurs.

Décompte : 0 bloquant, 3 majeurs, 10 mineurs.
