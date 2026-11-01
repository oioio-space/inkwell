# Revue indépendante Codex — round 2 — 2026-11-01

Périmètre : proposition de design révision 2 et confrontation statique au frontend ; pas une certification de l’application exécutée. Les références `spec.md`, `coverage.md`, `tokens.json`, `inventory.md`, `CHANGES.md` sont relatives à `docs/design/` ; les autres chemins sont depuis la racine. Les écarts de code déclarés dans l’inventaire ne signifient pas à eux seuls que la proposition est inadéquate (`spec.md:3`, `inventory.md:8`). Les constats ci-dessous distinguent ces deux niveaux. Aucun test applicatif ni essai Orca effectué ; les preuves demandées restent à produire (`spec.md:1678`).

Référentiels consultés : [WCAG 2.2](https://www.w3.org/TR/WCAG22/) (critères cités ci-dessous), [APG Tabs](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/), [APG Listbox](https://www.w3.org/WAI/ARIA/apg/patterns/listbox/), [GNOME HIG Toasts](https://developer.gnome.org/hig/patterns/feedback/toasts.html), [libadwaita 1.8 TabBar](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.8/class.TabBar.html), [Nielsen](https://www.nngroup.com/articles/ten-usability-heuristics/). Les normes internes N17/N19–N22 sont dans `docs/exigences.md:62`, `:64`, `:65`, `:66`, `:67`.

## Constats à traiter

### R2-B1 — blocker — N22 absent du contrat de thème

N22 exige les thèmes modifiables (`docs/exigences.md:67`). Le seul contrat actuel fait **recevoir** les jetons au cadre de l’extension (`spec.md:1192`) ; Apparence ne propose que Système/Clair/Sombre (`spec.md:1378`). La portée et la matrice ne couvrent que N21 (`spec.md:15`, `coverage.md:158`). Il manque la liste des jetons remplaçables et protégés, version/compatibilité du contrat, priorité entre thème système/utilisateur/extensions, validation des couleurs **effectives au chargement**, composition des transparences, contrôle des couples de texte/états/focus en clair/sombre/HC, rejet avec explication et retour au thème sûr. Le test de parité statique (`spec.md:91`) ne remplace pas cette validation. Normes : N22, WCAG 1.4.3/1.4.11, Nielsen 3/5/9. Définir aussi l’accès à « Revenir au thème par défaut » quand le thème est illisible. Bloque l’acceptation du design demandé.

### R2-B2 — blocker — G-0 toujours ouvert, formulation de clôture trompeuse

`frontend/src/App.svelte:197` ignore le chemin transmis par `frontend/src/ui/shell/Sidebar.svelte:49` et ne fait que focaliser la session. Cliquer ne change donc pas de fichier. `spec.md:788` et `coverage.md:144` le reconnaissent, mais `inventory.md:46` écrit **« Corrigé par un lot séparé »** et `CHANGES.md:15` emploie également « corrigé ». Remplacer cette affirmation par « à corriger, lot séparé en cours » ; distinguer livraison du correctif et spécification des onglets (`spec.md:945`). Normes : Nielsen 1/3, N19, HIG navigation/Tabs. Le parcours applicatif reste bloqué ; ne pas déclarer ce constat R1 fermé dans le code.

### R2-M1 — major — la matrice confond exigence et preuve de conformité

La légende S signifie « conforme dans le code, preuve existante » (`coverage.md:11`). Pourtant 2.1.2 est S sur la base du futur dialogue (`coverage.md:56`, `spec.md:627`), la performance entière est S sur la seule existence de seuils (`coverage.md:156`, `frontend/src/store/limits.ts:15`), et Commentaires affirme « L’onglet est masqué » (`coverage.md:159`) alors que l’inventaire dit affiché (`inventory.md:110`) et que `frontend/src/ui/history/HistoryDrawer.svelte:206` le rend encore. La matrice ci-après suspend les S sans preuve suffisante. Normes : P7 mesuré (`docs/design/README.md:65`), Nielsen 1, N17. Fournir des assertions réalisées, pas seulement leurs futurs noms (`spec.md:1685`).

### R2-M2 — major — exemption de reflow trop large

`spec.md:407` exempte **l’éditeur** en bloc de 1.4.10 au titre de contenu bidimensionnel. Un paragraphe .txt/.md reste du texte linéaire (`spec.md:1021`) ; l’exception ne couvre que les parties qui nécessitent réellement deux dimensions. Exiger le retour à la ligne et l’accès aux commandes à 320px CSS pour le texte ordinaire ; réserver l’exception aux tableaux, pages ou passages intrinsèquement bidimensionnels. Norme : WCAG 1.4.10. Le test prévu à `spec.md:1692` doit couvrir les quatre formats, pas seulement le chrome.

### R2-M3 — major — contrat clavier/focus incomplet aux transitions

« Toujours au déclencheur, sans exception » (`spec.md:485`) ne peut pas couvrir un déclencheur supprimé, une extension désactivée ou une commande qui ouvre un autre contenu ; APG Dialog autorise un retour logique dans le parcours. Définir un repli explicite. Pour les onglets, `spec.md:975` ne précise ni activation manuelle/automatique, ni relation `tab`/`tabpanel` et `aria-selected`, ni destination du focus après fermeture du dernier onglet (`spec.md:961`). `spec.md:385` omet le nom accessible et l’orientation des séparateurs. Normes : APG Tabs/Dialog/Window Splitter, WCAG 2.4.3/4.1.2. Fournir les transitions et attributs, sans inventer un composant supplémentaire.

### R2-M4 — major — annulation des extensions limitée et perte de données implicite

`spec.md:1152` rend le retrait annulable seulement jusqu’à la fermeture, puis efface les données locales ; TO-1 ne définit que texte/fichiers/personnes/export (`spec.md:694`). L’utilisateur n’est pas averti de l’effacement, et aucune distinction entre désinstaller et effacer les données n’existe dans la ligne décrite (`spec.md:1129`). Normes : Nielsen 3/5/9, WCAG 3.3.4 pour les données contrôlables par l’utilisateur, P3. Conserver les données par défaut, prévoir une suppression explicite et récupérable ou confirmée avec conséquences écrites ; intégrer le retrait à l’annulation durable. Tester aussi la réactivation après retour d’autorisation (`spec.md:1175`).

### R2-M5 — major — extensibilité et consentement incomplets au-delà des vues

Les points d’affichage (`spec.md:1180`) couvrent panneaux, commandes et rendu, mais N22 demande aussi remplacement du stockage et actions au démarrage/arrêt (`docs/exigences.md:67`). Le consentement ne nomme ni le fournisseur qui reçoit les documents, ni la portée des opérations automatiques (`spec.md:1164`). Il manque migration/échec du fournisseur, copie locale conservée, sortie vers le stockage précédent et récupération quand une extension indispensable manque. La promesse « seule la vue enrichie manque » (`spec.md:1202`) ne tient plus pour un fournisseur de stockage. Normes : N21/N22, Nielsen 1/3/5/9. Le report de l’API à N18 (`spec.md:1121`) est raisonnable, celui de ces parcours UX ne l’est pas. À la mise à jour, conserver un accès au consentement complet même si seules les nouvelles autorisations sont mises en avant (`spec.md:1150`).

### R2-M6 — major — tous les états ne sont pas réellement spécifiés

Le tableau commun contient neuf lignes, malgré « huit états » (`spec.md:471`–`:483`), et ne remplace pas les transitions par composant. Exemple concret : la gestion d’extensions n’a ni état vide, ni installation/mise à jour en cours, ni annulation, ni paquet incompatible distinct d’un paquet abîmé, ni reprise après échec de mise à jour (`spec.md:1138`–`:1153`). Le nom obligatoire à l’accueil ne définit pas validation, erreur ou abandon (`spec.md:1224`). Une action réseau « mise en attente » (`spec.md:483`) n’a pas de règle de péremption/revalidation des droits avant envoi. Normes : N17, Nielsen 1/5/9, WCAG 3.3.1/3.3.3. Ajouter ces états ciblés et leur récupération ; ne pas exiger neuf dessins inutiles pour chaque élément décoratif.

### R2-M7 — major — prédicat des paires interdites impossible à satisfaire

T-5 exige que **chaque paire forbidden échoue** (`spec.md:96`, `spec.md:1684`). Le recalcul ci-dessous montre accent/scène = **3,4586 en clair et 12,3998 en sombre**, minimum déclaré 3 : cette paire passe dans les deux thèmes. fg-dim/sélection sur vue échoue en clair mais passe en sombre (4,4767 / 4,9831). Séparer « usage interdit par politique » de « échec de contraste attendu par mode ». Ne pas encoder comme test obligatoire une assertion fausse. Normes : WCAG 1.4.3/1.4.11, principe mesuré (`docs/ux-principles.md:26`). La variante blanche reste une bonne règle de cohérence, sans prétendre que 3,46 échoue à 3.

### R2-M8 — major — notifications : exemples incompatibles et priorité absolue

Un toast admet une seule action (`spec.md:691`) mais les parcours en donnent deux : Voir/Annuler (`spec.md:1318`) et Ouvrir/Afficher dans le dossier (`spec.md:1362`). Choisir l’action principale et placer l’autre dans un accès durable. La priorité « toutes les questions avant tous les blocages » (`spec.md:424`) permet à une suite de demandes d’accès de précéder une erreur disque ou un retrait d’accès ; +N fournit un recours (`spec.md:443`) mais pas la visibilité immédiate du blocage. Conserver les questions accessibles sans masquer le contexte critique d’écriture. Normes : HIG Toasts, Nielsen 1/5/8, WCAG 4.1.3. Le code monte encore toast et texte ensemble (`frontend/src/ui/NoticeToast.svelte:6`) : correction documentaire, pas preuve d’annonce Orca.

### R2-m1 — minor — sur-spécification de l’implémentation et coût visuel

N20 demande une lecture agréable, pas un IDE (`docs/exigences.md:65`). Chargement paresseux et dix couleurs réutilisées sont proportionnés (`spec.md:1057`, `:1105`). En revanche le nombre « environ 25 langages » sans budget mesuré (`spec.md:1111`) et l’interdiction de toute taille de police TS brute (`spec.md:1683`) peuvent figer des moyens plutôt que le résultat ; exiger couverture prioritaire et budget de chargement. L’attribution sur **chaque** cadre (`spec.md:1188`) mérite un test de densité pour plusieurs extensions, hypothèse de surcharge à vérifier à 360px. Norme : Nielsen 8, PERF-1. Ce n’est pas un motif pour supprimer attribution ou consentement.

### R2-m2 — minor — arbitrages encore non ratifiés

La priorité des principes/glossaire est explicite (`README.md:17`) ; les changements de palette, cibles et « variante » restent proposés (`spec.md:1707`, `:1715`), pas adoptés. `docs/ux-principles.md:14` conserve Ctrl+K et `docs/glossary.md:15` conserve brouillon. Ne pas présenter R1 C12/F5/F8 comme totalement clos : conserver le statut décision en attente, puis synchroniser les documents après ratification. Norme : Nielsen 4, hiérarchie documentaire locale. Aucun changement à ces documents effectué par cette revue.

## Vérification ligne par ligne de coverage.md

Chaque ligne ci-dessous correspond à une ligne de données de la matrice, y compris les groupes NA. « Renvoi cohérent » vérifie la présence du traitement dans la proposition, **pas** son implémentation. Les écarts G restent ouverts selon `inventory.md:149`–`:187`. « S non prouvé » retire toute présomption de certification ; les assertions exécutées ne sont pas fournies par la matrice (`coverage.md:11`, `spec.md:1678`).

| Ligne de couverture | Sujet | Renvoi contrôlé / preuve | Conclusion |
|---|---|---|---|
| coverage.md:22 | 1 | spec.md:748, spec.md:802, spec.md:826, spec.md:871, spec.md:1443, spec.md:943 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:23 | 2 | spec.md:1508, spec.md:1117 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:24 | 3 | spec.md:1290, spec.md:943, spec.md:1388, spec.md:694 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:25 | 4 | spec.md:1565, spec.md:616 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:26 | 5 | spec.md:1290, spec.md:649, spec.md:1117, spec.md:1184 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:27 | 6 | spec.md:926, spec.md:594, spec.md:535, spec.md:983 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:28 | 7 | spec.md:354, spec.md:1578 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:29 | 8 | spec.md:35, spec.md:1023 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:30 | 9 | spec.md:1424, spec.md:1260, spec.md:1355 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:31 | 10 | spec.md:983 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:39 | 1.1.1 Contenu non textuel | spec.md:300 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:40 | 1.2.1 à 1.2.5 Médias temporels | spec.md:1535 | Justification de périmètre repérée |
| coverage.md:41 | 1.3.1 Information et relations | spec.md:466, spec.md:1559 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:42 | 1.3.2 Ordre séquentiel logique | spec.md:1545 | S non prouvé ; exigence repérée |
| coverage.md:43 | 1.3.3 Caractéristiques sensorielles | spec.md:1020 | S non prouvé ; exigence repérée |
| coverage.md:44 | 1.3.4 Orientation | spec.md:407 | S non prouvé ; exigence repérée |
| coverage.md:45 | 1.3.5 Finalité des champs | spec.md:708 ; frontend/src/ui/ShareDialog.svelte:43 | S confirmé pour ce champ : autocomplete nickname |
| coverage.md:46 | 1.4.1 Utilisation de la couleur | spec.md:1034, spec.md:1028 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:47 | 1.4.2 Contrôle du son | spec.md:1535 | Justification de périmètre repérée |
| coverage.md:48 | 1.4.3 Contraste (minimum) | spec.md:102 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:49 | 1.4.4 Redimensionnement du texte | spec.md:1676, spec.md:234, spec.md:1552 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:50 | 1.4.5 Texte en image | spec.md:826 | Exception rendu exact plausible ; accessibilité du PDF non prouvée (spec.md:843) |
| coverage.md:51 | 1.4.10 Redistribution | spec.md:394, spec.md:407 | À corriger : exemption éditeur trop large (M2) |
| coverage.md:52 | 1.4.11 Contraste du contenu non textuel | spec.md:102 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:53 | 1.4.12 Espacement du texte | spec.md:1676, spec.md:234 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:54 | 1.4.13 Contenu au survol ou au focus | spec.md:594 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:55 | 2.1.1 Clavier | spec.md:466, spec.md:1545 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:56 | 2.1.2 Pas de piège au clavier | spec.md:616 | S non prouvé ; exigence repérée |
| coverage.md:57 | 2.1.4 Raccourcis d'une seule touche | spec.md:1561 | S non prouvé : portée au focus exigée par spec.md:1561 ; vérifier aussi L et chiffres (present/keys.ts:26) |
| coverage.md:58 | 2.2.1 Délai réglable | spec.md:694, spec.md:1554 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:59 | 2.2.2 Mettre en pause, arrêter, masquer | spec.md:288 | S non prouvé ; exigence repérée |
| coverage.md:60 | 2.3.1 Trois flashs | spec.md:293 | S non prouvé ; exigence repérée |
| coverage.md:61 | 2.4.1 Contourner des blocs | spec.md:1559 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:62 | 2.4.2 Titre de page | spec.md:368 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:63 | 2.4.3 Parcours du focus | spec.md:466 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:64 | 2.4.4 Fonction du lien | spec.md:1535 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:65 | 2.4.5 Accès multiples | spec.md:354 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:66 | 2.4.6 En-têtes et étiquettes | spec.md:708, spec.md:1477 | S non prouvé ; exigence repérée |
| coverage.md:67 | 2.4.7 Visibilité du focus | spec.md:1535 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:68 | 2.4.11 Focus non masqué | spec.md:1676, spec.md:1546 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:69 | 2.5.1 Gestes du pointeur | spec.md:1535 | S non prouvé ; exigence repérée |
| coverage.md:70 | 2.5.2 Annulation du pointeur | spec.md:1535 | S non prouvé ; exigence repérée |
| coverage.md:71 | 2.5.3 Étiquette dans le nom | spec.md:507 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:72 | 2.5.4 Activation par le mouvement | spec.md:1535 | Justification de périmètre repérée |
| coverage.md:73 | 2.5.7 Mouvements de glissement | spec.md:943, spec.md:770, spec.md:391, spec.md:1549 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:74 | 2.5.8 Taille des cibles (minimum) | spec.md:242, spec.md:1548 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:75 | 3.1.1 Langue de la page | spec.md:1555 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:76 | 3.1.2 Langue d'un passage | spec.md:1535 | S non prouvé : document multilingue non traité ; prévoir indication de langue des passages quand déterminable (WCAG 3.1.2) |
| coverage.md:77 | 3.2.1 Au focus | spec.md:1535 | S non prouvé ; exigence repérée |
| coverage.md:78 | 3.2.2 À la saisie | spec.md:1535 | S non prouvé ; exigence repérée |
| coverage.md:79 | 3.2.3 Navigation cohérente | spec.md:512, spec.md:347 | S non prouvé ; exigence repérée |
| coverage.md:80 | 3.2.4 Identification cohérente | spec.md:1508, spec.md:1623 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:81 | 3.2.6 Aide cohérente | spec.md:983, spec.md:1556 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:82 | 3.3.1 Identification des erreurs | spec.md:708, spec.md:1290 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:83 | 3.3.2 Étiquettes ou instructions | spec.md:708 | S non prouvé ; exigence repérée |
| coverage.md:84 | 3.3.3 Suggestion après une erreur | spec.md:1260, spec.md:1290 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:85 | 3.3.4 Prévention des erreurs (données) | spec.md:649 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:86 | 3.3.7 Saisie redondante | spec.md:1557 | S non prouvé ; exigence repérée |
| coverage.md:87 | 3.3.8 Authentification accessible (minimum) | spec.md:1557 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:88 | 4.1.2 Nom, rôle et valeur | spec.md:466, spec.md:1560, spec.md:1097 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:89 | 4.1.3 Messages d'état | spec.md:667, spec.md:1551 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:99 | Dialog (Modal) | spec.md:616 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:100 | Alert Dialog | spec.md:649 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:101 | Menu Button, Menu | spec.md:512 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:102 | Toolbar | spec.md:535 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:103 | Tabs | spec.md:573, spec.md:943 | Sélecteurs spécifiés ; contrat des fichiers incomplet (M3) |
| coverage.md:104 | Tree View | spec.md:770 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:105 | Listbox | spec.md:878, spec.md:926 | Focus corrigé dans la proposition ; actions de ligne à garder hors options (APG Listbox ; spec.md:885) |
| coverage.md:106 | Grid | spec.md:846 | S non prouvé ; exigence repérée |
| coverage.md:107 | Combobox | spec.md:926 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:108 | Radio Group | spec.md:573 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:109 | Switch | spec.md:573 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:110 | Tooltip | spec.md:594 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:111 | Window Splitter | spec.md:374 | Contrat partiel : nom/orientation à ajouter (M3) |
| coverage.md:112 | Button (bascule) | spec.md:507 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:118 | `AdwHeaderBar` : titre = contenu, peu de contrôles | spec.md:489 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:119 | `AdwViewSwitcher` (panneau de gauche, tiroir) | spec.md:315, spec.md:573 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:120 | `AdwTabBar` (onglets de fichiers) | spec.md:943 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:121 | `AdwOverlaySplitView` (états étroits) | spec.md:394 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:122 | Listes encadrées, `AdwExpanderRow` | spec.md:660, spec.md:1117 | S non prouvé ; exigence repérée |
| coverage.md:123 | `AdwPreferencesDialog` (pages, groupes, portée) | spec.md:660, spec.md:1374 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:124 | `AdwAboutDialog` | spec.md:1461 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:125 | `AdwAlertDialog` | spec.md:649 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:126 | `AdwToast` / `AdwToastOverlay` (file, 5s, une action) | spec.md:686, spec.md:451 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:127 | `AdwBanner` (un état, ne disparaît pas seul) | spec.md:667, spec.md:35 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:128 | Popovers (un tiers de la fenêtre au plus, Échap) | spec.md:608 | S non prouvé ; exigence repérée |
| coverage.md:129 | `AdwStatusPage` (écrans vides) | spec.md:854 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:130 | `AdwSpinner` (délai, un seul) | spec.md:871 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:131 | `AdwToggleGroup` | spec.md:573 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:132 | Fenêtre des raccourcis, Ctrl+? | spec.md:983 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:133 | Clavier HIG : F10, Ctrl+, Ctrl+W, F1, Échap | spec.md:1578 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:134 | Style d'écriture (verbes, « … », pas de « OK ») | spec.md:1477 | S non prouvé ; exigence repérée |
| coverage.md:135 | Adaptatif (seuils, taille minimale) | spec.md:394 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:136 | Couleurs, rayons, typographie libadwaita | spec.md:64 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:137 | Accessibilité HIG (contraste élevé, grand texte) | spec.md:35, spec.md:1552 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:138 | Application *Extensions* de GNOME (lignes, interrupteur, retrait) | spec.md:1117 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:144 | **Architecture de l'information, navigation** | spec.md:315, spec.md:354, spec.md:770, spec.md:943 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:145 | **Accueil, premier lancement, écrans vides** | spec.md:1219, spec.md:1233, spec.md:854 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:146 | **États des composants** (survol, focus, actif, désactivé, chargement, erreur, vide, hors ligne) | spec.md:466 | Tableau commun insuffisant ; M6 |
| coverage.md:147 | **Textes, erreurs, reprise, annulation** | spec.md:1468, spec.md:1424, spec.md:694 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:148 | **Notifications : toasts et bandeaux** | spec.md:414, spec.md:667, spec.md:686 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:149 | **Collaboration** (présence, curseurs, invitations, divergences, rôles, accès suspendu ou retiré, verrous) | spec.md:723, spec.md:748, spec.md:1247, spec.md:1260, spec.md:1388, spec.md:1400 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:150 | **Projets à plusieurs fichiers** (panneau, onglets, ouvrir, fermer, créer, renommer, déplacer, supprimer) | spec.md:770, spec.md:943, spec.md:1279, spec.md:1290 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:151 | **Aperçu Typst et Markdown, export, impression** | spec.md:826, spec.md:1325, spec.md:1330, spec.md:1355 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:152 | **Réglages, raccourcis, palette, aide** | spec.md:1374, spec.md:1578, spec.md:926, spec.md:983 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:153 | **Langues fr/en** | spec.md:1490, spec.md:1555 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:154 | **Clair et sombre, contraste élevé, couleurs forcées** | spec.md:102, spec.md:78, spec.md:89 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:155 | **Tailles de fenêtre, jusqu'à 360px** | spec.md:374, spec.md:394 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:156 | **Performance perçue** (gros fichiers, WAN) | spec.md:1443, spec.md:1676 | S excessif : seuils ≠ mesure de latence (M1) |
| coverage.md:157 | **Types de fichiers et code** (N20) : icônes, palette, détection, édition ou lecture, inconnus et binaires | spec.md:1008, spec.md:1028, spec.md:1097 | Renvoi cohérent ; écart/code à réaliser |
| coverage.md:158 | **Extensions** (N21) : gestion, consentement, points d'affichage, confinement, collaboration | spec.md:1117 | N21 partiel ; N22 absent (B1, M4–M6) |
| coverage.md:159 | **Commentaires** | spec.md:994 | Faux état masqué : frontend/src/ui/history/HistoryDrawer.svelte:206 (M1) |
| coverage.md:160 | **Présentation** | spec.md:907, spec.md:1344 | Renvoi cohérent ; écart/code à réaliser |

Le groupe 1.2.1–1.2.5 regroupe cinq critères : le décompte annoncé 55 est cohérent avec le retrait de 4.1.1 (`coverage.md:35`). Son NA doit être réexaminé si une visionneuse d’extension introduit audio/vidéo (hypothèse d’extension de portée ; `spec.md:1185`). Grid est décrit dans `spec.md:848` et possède une cellule active calculée (`frontend/src/ui/sheet/Grid.svelte:188`) ; cela ne certifie pas toutes les interactions. Le switch natif via checkbox n’est pas automatiquement une violation WCAG : distinguer convention visuelle et exigence APG (`inventory.md:84`).

## Fermeture effective des constats R1

Les identifiants C/F suivent `CHANGES.md:6` et `:27`, confrontés aux deux originaux `reviews/01-codex.md:3` et `reviews/02-fable.md:5`. « Corrigé en spec » ne signifie pas corrigé en application (`spec.md:3`).

| Constat R1 | Contrôle R2 / preuve | État réel |
|---|---|---|
| C1 | Recalcul ci-dessous ; spec.md:162 | Corrigé numériquement |
| C2 | Fonds over dans le tableau ; spec.md:39, :604 | Corrigé pour les paires déclarées |
| C3 | spec.md:140, :145 ; frontend/src/editor/setup.ts:39 | Inventorié ; code toujours non conforme |
| C4 | tokens.json:1 ; frontend/src/theme/tokens.test.ts:23 ; spec.md:91 | Contrat enrichi, parité toujours à écrire ; M7 |
| C5 | frontend/src/App.svelte:197 ; inventory.md:46 | Toujours ouvert, B2 |
| C6 | spec.md:446, :451 ; frontend/src/ui/sheet/Sheet.svelte:541 | Sources ajoutées ; migration à faire |
| C7 | spec.md:540, :575, :883 ; frontend/src/ui/history/HistoryDrawer.svelte:143 | Modèles améliorés, code non fermé |
| C8 | spec.md:515, :518, :590, :933 | Corrigé en spec ; retour sans exception à nuancer M3 |
| C9 | spec.md:634, :654 ; frontend/src/ui/prefs/Preferences.svelte:38 | Corrigé en spec ; code non fermé |
| C10 | spec.md:680 ; frontend/src/ui/NoticeToast.svelte:6 | Consigne corrigée ; preuve d’annonce absente |
| C11 | spec.md:443, :694 | Corrigé pour actions listées ; extension manquante M4 |
| C12 | spec.md:337, :403, :901, :459 | Architecture explicitée ; glossaire en attente m2 |
| C13 | spec.md:1682–1697 | Plan de preuves amélioré ; assertions non exécutées et M7 |
| C14 | spec.md:1707–1713 | Recommandations conservées, pas décisions ratifiées |
| F1 | spec.md:151 ; calcul anneau blanc ci-dessous | Variante ajoutée ; assertion d’échec de scène fausse M7 |
| F2 | spec.md:337, :381, :403 | Corrigé en spec |
| F3 | spec.md:424, :443 | Questions reconnues ; priorité à affiner M8 |
| F4 | spec.md:994 ; coverage.md:159 | Hors portée explicité ; affirmation « masqué » fausse M1 |
| F5 | spec.md:251, :1715 | Écart explicité, amendement en attente |
| F6 | spec.md:56 | Corrigé en spec |
| F7 | spec.md:88, :283 | Corrigé en spec |
| F8 | spec.md:1523 ; inventory.md:130 | Vocabulaire prescrit ; application à corriger |
| F9 | spec.md:694 | Corrigé en spec pour actions listées ; M4 |
| F10 | inventory.md:162, :166 | Vague 2 explicitée |
| F11 | spec.md:1685, :1687, :1690 | Preuves planifiées ; résultats non produits |
| F12 | spec.md:1707–1713 | Décisions toujours à confirmer |

## Contrastes recalculés indépendamment

Script Python exécuté en mémoire sur `tokens.json`, **102 paires**, deux modes chacune (les deux dernières HC). Composition sRGB avant linéarisation, sans arrondi intermédiaire. Quatre valeurs claires diffèrent au-delà de l’arrondi à deux décimales : personne-3-line **6,9040** contre 6,93, personne-6-line **7,6415** contre 7,61, personne-7-line **6,2940** contre 6,33, personne-8-line **6,0078** contre 5,97 (indices 88/94/96/98 ci-dessous). Ces écarts ne font échouer aucun minimum ; corriger ces chiffres avec M7. Les autres valeurs déclarées sont retrouvées à 0,011 près ; ce résultat concerne le JSON, pas les valeurs CSS effectivement rendues (`spec.md:91`, `frontend/src/theme/tokens.test.ts:23`). Les minima passent pour les paires autorisées. Les interdictions relèvent parfois de la politique, pas d’un échec numérique : voir M7.

Formule du script :

```python
# canaux dans [0,1] ; fond translucide composé sur le parent over déclaré
compose = lambda fg, bg, alpha: alpha * fg + (1-alpha) * bg
linear = lambda c: c/12.92 if c <= 0.04045 else ((c+0.055)/1.055)**2.4
L = lambda rgb: sum(w*linear(c) for w,c in zip((.2126,.7152,.0722),rgb))
ratio = lambda a,b: (max(L(a),L(b))+.05)/(min(L(a),L(b))+.05)
# foreground translucide composé ensuite sur ce fond final ; modes résolus via
# $value puis $extensions.inkwell.modes ; HC hérite du mode clair/sombre
# pour les jetons sans surcharge HC ; pas de conversion hex intermédiaire.
```

Les numéros identifient l’index zéro de `contrast.pairs` ; pour les deux dernières lignes les colonnes sont HC clair/sombre. Les références pointent vers la première occurrence du même foreground ; l’index et le triplet identifient la paire sans ambiguïté.

| Index | Texte/trait / fond / parent | Clair | Sombre | Seuil | État | Source |
|---|---|---:|---:|---:|---|---|
| 0 | `--fg` / `--view-bg` / `opaque` | 12.5673 | 16.8155 | 4.5 | passe | tokens.json:1872 |
| 1 | `--fg` / `--window-bg` / `opaque` | 12.2232 | 15.8513 | 4.5 | passe | tokens.json:1872 |
| 2 | `--fg` / `--sidebar-bg` / `opaque` | 11.1820 | 13.5220 | 4.5 | passe | tokens.json:1872 |
| 3 | `--fg` / `--popover-bg` / `opaque` | 12.5673 | 12.0301 | 4.5 | passe | tokens.json:1872 |
| 4 | `--fg` / `--dialog-bg` / `opaque` | 12.2232 | 12.0301 | 4.5 | passe | tokens.json:1872 |
| 5 | `--fg` / `--headerbar-bg` / `opaque` | 12.5673 | 13.5220 | 4.5 | passe | tokens.json:1872 |
| 6 | `--fg-dim` / `--view-bg` / `opaque` | 4.7422 | 5.9314 | 4.5 | passe | tokens.json:1920 |
| 7 | `--fg-dim` / `--window-bg` / `opaque` | 4.6942 | 5.7511 | 4.5 | passe | tokens.json:1920 |
| 8 | `--fg-dim` / `--sidebar-bg` / `opaque` | 4.5390 | 5.2387 | 4.5 | passe | tokens.json:1920 |
| 9 | `--fg-dim` / `--popover-bg` / `opaque` | 4.7422 | 4.8654 | 4.5 | passe | tokens.json:1920 |
| 10 | `--fg-dim` / `--dialog-bg` / `opaque` | 4.6942 | 4.8654 | 4.5 | passe | tokens.json:1920 |
| 11 | `--fg-dim` / `--headerbar-bg` / `opaque` | 4.7422 | 5.2387 | 4.5 | passe | tokens.json:1920 |
| 12 | `--fg` / `--selected-bg` / `--view-bg` | 10.7928 | 12.4865 | 4.5 | passe | tokens.json:1872 |
| 13 | `--fg` / `--selected-bg` / `--sidebar-bg` | 9.5602 | 9.8677 | 4.5 | passe | tokens.json:1872 |
| 14 | `--fg-dim` / `--selected-bg` / `--view-bg` | 4.4767 | 4.9831 | 4.5 | interdite par politique | tokens.json:1920 |
| 15 | `--fg-dim` / `--selected-bg` / `--sidebar-bg` | 4.2614 | 4.2709 | 4.5 | interdite par politique | tokens.json:1920 |
| 16 | `--accent` / `--view-bg` / `opaque` | 6.0719 | 9.9290 | 4.5 | passe | tokens.json:2006 |
| 17 | `--accent` / `--sidebar-bg` / `opaque` | 5.1000 | 7.9843 | 4.5 | passe | tokens.json:2006 |
| 18 | `--accent` / `--popover-bg` / `opaque` | 6.0719 | 7.1034 | 4.5 | passe | tokens.json:2006 |
| 19 | `--accent` / `--dialog-bg` / `opaque` | 5.8209 | 7.1034 | 4.5 | passe | tokens.json:2006 |
| 20 | `--accent-fg` / `--suggested-bg` / `opaque` | 4.7695 | 4.7695 | 4.5 | passe | tokens.json:2038 |
| 21 | `--accent-fg` / `--accent-bg` / `opaque` | 3.7687 | 3.7687 | 4.5 | interdite par politique | tokens.json:2038 |
| 22 | `#ffffff` / `--destructive-bg` / `opaque` | 4.8257 | 6.1081 | 4.5 | passe | tokens.json:2056 |
| 23 | `--warning` / `--view-bg` / `opaque` | 6.0799 | 10.4821 | 4.5 | passe | tokens.json:2064 |
| 24 | `--error` / `--view-bg` / `opaque` | 6.3245 | 7.8506 | 4.5 | passe | tokens.json:2072 |
| 25 | `--warning` / `--window-bg` / `opaque` | 5.8286 | 9.8810 | 4.5 | passe | tokens.json:2064 |
| 26 | `--error` / `--window-bg` / `opaque` | 6.0631 | 7.4004 | 4.5 | passe | tokens.json:2072 |
| 27 | `--warning` / `--popover-bg` / `opaque` | 6.0799 | 7.4991 | 4.5 | passe | tokens.json:2064 |
| 28 | `--error` / `--popover-bg` / `opaque` | 6.3245 | 5.6164 | 4.5 | passe | tokens.json:2072 |
| 29 | `--warning` / `--warning-bg` / `--window-bg` | 5.5252 | 7.1361 | 4.5 | passe | tokens.json:2064 |
| 30 | `--error` / `--error-bg` / `--window-bg` | 5.3783 | 5.6750 | 4.5 | passe | tokens.json:2072 |
| 31 | `--fg` / `--info-bg` / `opaque` | 11.2876 | 8.9107 | 4.5 | passe | tokens.json:1872 |
| 32 | `--fg` / `--warning-bg` / `--window-bg` | 11.8016 | 11.4478 | 4.5 | passe | tokens.json:1872 |
| 33 | `--fg` / `--error-bg` / `--window-bg` | 11.2795 | 12.1555 | 4.5 | passe | tokens.json:1872 |
| 34 | `--fg-dim` / `--info-bg` / `opaque` | 4.5550 | 3.9471 | 4.5 | interdite par politique | tokens.json:1920 |
| 35 | `--success` / `--headerbar-bg` / `opaque` | 5.3159 | 9.0419 | 3.0 | passe | tokens.json:2167 |
| 36 | `--accent` / `--view-bg` / `opaque` | 6.0719 | 9.9290 | 3.0 | passe | tokens.json:2006 |
| 37 | `--accent` / `--window-bg` / `opaque` | 5.8209 | 9.3596 | 3.0 | passe | tokens.json:2006 |
| 38 | `--accent` / `--headerbar-bg` / `opaque` | 6.0719 | 7.9843 | 3.0 | passe | tokens.json:2006 |
| 39 | `--accent` / `--sidebar-bg` / `opaque` | 5.1000 | 7.9843 | 3.0 | passe | tokens.json:2006 |
| 40 | `--accent` / `--popover-bg` / `opaque` | 6.0719 | 7.1034 | 3.0 | passe | tokens.json:2006 |
| 41 | `--accent` / `--dialog-bg` / `opaque` | 5.8209 | 7.1034 | 3.0 | passe | tokens.json:2006 |
| 42 | `--accent` / `--side-bg-2` / `opaque` | 5.4790 | 8.6705 | 3.0 | passe | tokens.json:2006 |
| 43 | `--accent` / `--toast-bg` / `--view-bg` | 2.3900 | 7.1034 | 3.0 | interdite par politique | tokens.json:2006 |
| 44 | `--accent` / `--tooltip-bg` / `--view-bg` | 2.0698 | 11.9505 | 3.0 | interdite par politique | tokens.json:2006 |
| 45 | `--accent` / `--stage-bg` / `opaque` | 3.4586 | 12.3998 | 3.0 | interdite par politique | tokens.json:2006 |
| 46 | `--on-dark-fg` / `--toast-bg` / `--view-bg` | 14.5120 | 12.0301 | 3.0 | passe | tokens.json:2269 |
| 47 | `--on-dark-fg` / `--tooltip-bg` / `--view-bg` | 12.5673 | 20.2392 | 3.0 | passe | tokens.json:2269 |
| 48 | `--on-dark-fg` / `--stage-bg` / `opaque` | 21.0000 | 21.0000 | 3.0 | passe | tokens.json:2269 |
| 49 | `--on-dark-fg` / `--toast-bg` / `--view-bg` | 14.5120 | 12.0301 | 4.5 | passe | tokens.json:2269 |
| 50 | `--tooltip-fg` / `--tooltip-bg` / `--view-bg` | 12.5673 | 20.2392 | 4.5 | passe | tokens.json:2305 |
| 51 | `--on-dark-fg` / `--stage-overlay` / `--stage-bg` | 21.0000 | 21.0000 | 4.5 | passe | tokens.json:2269 |
| 52 | `--diff-add-fg` / `--diff-add-bg` / `--dialog-bg` | 8.5483 | 6.2446 | 4.5 | passe | tokens.json:2323 |
| 53 | `--diff-del-fg` / `--diff-del-bg` / `--dialog-bg` | 8.3964 | 5.9422 | 4.5 | passe | tokens.json:2332 |
| 54 | `--diff-add-fg` / `--diff-add-bg` / `--view-bg` | 8.5483 | 8.4551 | 4.5 | passe | tokens.json:2323 |
| 55 | `--diff-del-fg` / `--diff-del-bg` / `--view-bg` | 8.3964 | 7.9629 | 4.5 | passe | tokens.json:2332 |
| 56 | `--hl-heading` / `--view-bg` / `opaque` | 4.7695 | 7.2675 | 4.5 | passe | tokens.json:2359 |
| 57 | `--hl-function` / `--view-bg` / `opaque` | 4.9097 | 8.4754 | 4.5 | passe | tokens.json:2367 |
| 58 | `--hl-string` / `--view-bg` / `opaque` | 5.3407 | 10.2127 | 4.5 | passe | tokens.json:2375 |
| 59 | `--hl-label` / `--view-bg` / `opaque` | 5.9059 | 6.9845 | 4.5 | passe | tokens.json:2383 |
| 60 | `--hl-comment` / `--view-bg` / `opaque` | 5.0499 | 5.9022 | 4.5 | passe | tokens.json:2391 |
| 61 | `--hl-keyword` / `--view-bg` / `opaque` | 6.1081 | 6.6184 | 4.5 | passe | tokens.json:2399 |
| 62 | `--hl-number` / `--view-bg` / `opaque` | 5.8370 | 13.0066 | 4.5 | passe | tokens.json:2407 |
| 63 | `--hl-raw` / `--view-bg` / `opaque` | 6.5814 | 9.1443 | 4.5 | passe | tokens.json:2415 |
| 64 | `--hl-marker` / `--view-bg` / `opaque` | 4.7695 | 7.2675 | 4.5 | passe | tokens.json:2423 |
| 65 | `--hl-link` / `--view-bg` / `opaque` | 6.0719 | 9.9290 | 4.5 | passe | tokens.json:2431 |
| 66 | `--mark-typst` / `--sidebar-bg` / `opaque` | 3.2266 | 7.3606 | 3.0 | passe | tokens.json:2439 |
| 67 | `--mark-typst` / `--view-bg` / `opaque` | 3.8415 | 9.1534 | 3.0 | passe | tokens.json:2439 |
| 68 | `--mark-markdown` / `--sidebar-bg` / `opaque` | 4.0061 | 5.8440 | 3.0 | passe | tokens.json:2455 |
| 69 | `--mark-markdown` / `--view-bg` / `opaque` | 4.7695 | 7.2675 | 3.0 | passe | tokens.json:2455 |
| 70 | `--mark-csv` / `--sidebar-bg` / `opaque` | 3.8967 | 8.2124 | 3.0 | passe | tokens.json:2471 |
| 71 | `--mark-csv` / `--view-bg` / `opaque` | 4.6393 | 10.2127 | 3.0 | passe | tokens.json:2471 |
| 72 | `--mark-bib` / `--sidebar-bg` / `opaque` | 4.9606 | 5.6165 | 3.0 | passe | tokens.json:2487 |
| 73 | `--mark-bib` / `--view-bg` / `opaque` | 5.9059 | 6.9845 | 3.0 | passe | tokens.json:2487 |
| 74 | `--mark-image` / `--sidebar-bg` / `opaque` | 4.1239 | 6.8154 | 3.0 | passe | tokens.json:2503 |
| 75 | `--mark-image` / `--view-bg` / `opaque` | 4.9097 | 8.4754 | 3.0 | passe | tokens.json:2503 |
| 76 | `--mark-pdf` / `--sidebar-bg` / `opaque` | 4.0533 | 4.3446 | 3.0 | passe | tokens.json:2519 |
| 77 | `--mark-pdf` / `--view-bg` / `opaque` | 4.8257 | 5.4028 | 3.0 | passe | tokens.json:2519 |
| 78 | `--mark-data` / `--sidebar-bg` / `opaque` | 3.9009 | 10.4591 | 3.0 | passe | tokens.json:2535 |
| 79 | `--mark-data` / `--view-bg` / `opaque` | 4.6443 | 13.0066 | 3.0 | passe | tokens.json:2535 |
| 80 | `--mark-code` / `--sidebar-bg` / `opaque` | 5.5279 | 7.3533 | 3.0 | passe | tokens.json:2551 |
| 81 | `--mark-code` / `--view-bg` / `opaque` | 6.5814 | 9.1443 | 3.0 | passe | tokens.json:2551 |
| 82 | `--mark-extension` / `--sidebar-bg` / `opaque` | 7.5210 | 5.6165 | 3.0 | passe | tokens.json:2567 |
| 83 | `--mark-extension` / `--view-bg` / `opaque` | 8.9542 | 6.9845 | 3.0 | passe | tokens.json:2567 |
| 84 | `--person-1-line` / `--view-bg` / `opaque` | 4.9564 | 7.4660 | 3.0 | passe | tokens.json:2583 |
| 85 | `--person-1-ink` / `--person-1` / `opaque` | 9.3239 | 9.3239 | 4.5 | passe | tokens.json:2591 |
| 86 | `--person-2-line` / `--view-bg` / `opaque` | 5.0486 | 7.2874 | 3.0 | passe | tokens.json:2599 |
| 87 | `--person-2-ink` / `--person-2` / `opaque` | 9.1008 | 9.1008 | 4.5 | passe | tokens.json:2607 |
| 88 | `--person-3-line` / `--view-bg` / `opaque` | 6.9040 | 4.9152 | 3.0 | passe | tokens.json:2615 |
| 89 | `--person-3-ink` / `--person-3` / `opaque` | 6.1383 | 6.1383 | 4.5 | passe | tokens.json:2623 |
| 90 | `--person-4-line` / `--view-bg` / `opaque` | 3.1416 | 12.7166 | 3.0 | passe | tokens.json:2631 |
| 91 | `--person-4-ink` / `--person-4` / `opaque` | 15.8811 | 15.8811 | 4.5 | passe | tokens.json:2639 |
| 92 | `--person-5-line` / `--view-bg` / `opaque` | 9.4099 | 3.2428 | 3.0 | passe | tokens.json:2647 |
| 93 | `--person-5-ink` / `--person-5` / `opaque` | 5.1854 | 5.1854 | 4.5 | passe | tokens.json:2655 |
| 94 | `--person-6-line` / `--view-bg` / `opaque` | 7.6415 | 4.3483 | 3.0 | passe | tokens.json:2663 |
| 95 | `--person-6-ink` / `--person-6` / `opaque` | 5.4303 | 5.4303 | 4.5 | passe | tokens.json:2671 |
| 96 | `--person-7-line` / `--view-bg` / `opaque` | 6.2940 | 5.4934 | 3.0 | passe | tokens.json:2679 |
| 97 | `--person-7-ink` / `--person-7` / `opaque` | 6.8604 | 6.8604 | 4.5 | passe | tokens.json:2687 |
| 98 | `--person-8-line` / `--view-bg` / `opaque` | 6.0078 | 5.9022 | 3.0 | passe | tokens.json:2695 |
| 99 | `--person-8-ink` / `--person-8` / `opaque` | 7.3709 | 7.3709 | 4.5 | passe | tokens.json:2703 |
| 100 | `--fg-dim (contraste élevé)` / `--view-bg` / `opaque` | 17.4178 | 13.7771 | 4.5 | passe | tokens.json:2711 |
| 101 | `--border (contraste élevé)` / `--view-bg` / `opaque` | 3.9647 | 5.1400 | 3.0 | passe | tokens.json:2718 |

## Verdict

**NOT ACCEPTED — 2 blockers, 8 majors, 2 minors.**

Blockers : **R2-B1**, contrat N22 des thèmes extensibles et validation au chargement absent ; **R2-B2**, G-0 toujours présent et affirmation de correction à rectifier. Le correctif de code peut rester un lot séparé, mais sa clôture ne peut être déclarée avant preuve (`frontend/src/App.svelte:197`). Les majeurs ci-dessus doivent également être résolus pour accepter la spécification.
