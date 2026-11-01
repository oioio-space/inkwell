# Revue indépendante Codex — round 3 — 2026-11-01

Revue statique de la proposition rév. 3, des deux revues du round 2, de `docs/exigences.md` N17–N29, de l'architecture §15 et du frontend. Seul ce fichier a été créé ; aucun commit. Les références `spec.md`, `tokens.json`, `coverage.md`, `inventory.md` et `CHANGES.md` sont relatives à `docs/design/`. Les chemins frontend partent de la racine. Les seuils ont été recalculés indépendamment en Python en mémoire, sans modifier les sources. Pas d'exécution de l'application, de test Orca, ni de certification du rendu WebKitGTK.

**Distinction essentielle :** une correction de spécification n'est pas une correction livrée. Les fonctionnalités absentes et correctement identifiées S→N ne bloquent pas à elles seules l'acceptation de la proposition. G-0 reste en revanche un parcours applicatif cassé ; sa clôture dans le code n'est pas acquise.

Référentiels consultés : [WCAG 2.2](https://www.w3.org/TR/WCAG22/) (critères indiqués par constat), [Nielsen](https://www.nngroup.com/articles/ten-usability-heuristics/), [APG Menu](https://www.w3.org/WAI/ARIA/apg/patterns/menubar/), [APG Listbox](https://www.w3.org/WAI/ARIA/apg/patterns/listbox/), [APG Tooltip](https://www.w3.org/WAI/ARIA/apg/patterns/tooltip/), [GNOME HIG Principles](https://developer.gnome.org/hig/principles.html). APG est un guide de motifs ; une divergence de motif est ici un risque concret à résoudre, pas une certification automatique d'échec WCAG.

## 1. Vérification de chaque constat du round 2

« Fermé en spec » signifie texte correct retrouvé et confronté au code, sans prétendre à une livraison. Les références ci-dessous remplacent les numéros de ligne de la rév. 2.

| Codex round 2 | Conclusion vérifiée | Preuve rév. 3 / réserve |
|---|---|---|
| R2-B1 | **Partiellement fermé** | §2.10, `tokens.json:14` : contrat, quatre modes, priorité, refus et retour au défaut présents. Validation effective encore incomplète : R3-B1. Compatibilité du contrat non définie : R3-M6. |
| R2-B2 | **Affirmation fausse corrigée ; bug ouvert** | `inventory.md:8`, `CHANGES.md` C5 corrigé ; `frontend/src/App.svelte:197` ignore toujours le chemin. Aucun changement de fichier au clic. Nielsen 1/3, N19. |
| R2-M1 | **Largement fermé ; preuve à actualiser** | `coverage.md` distingue S, S→N et S→G ; performance rétrogradée, Commentaires encore affiché reconnu ; §10 sépare existant/à écrire. G-81 est désormais inexact : R3-m1. `showModal()` ne prouve toujours pas à lui seul tous les parcours sans piège, ce que la réserve « mécanisme » reconnaît. |
| R2-M2 | **Partiellement fermé** | `spec.md:478–487` force le reflow sous 560px, couvre les quatre formats ; reste une exception injustifiée pour le code : R3-M5. `frontend/src/editor/setup.ts:122` fournit déjà `lineWrapping`. |
| R2-M3 | **Fermé en spec** | `spec.md:453–462` : séparateurs nommés/orientés ; `:567–578` : replis de focus ; `:1096–1107` : activation manuelle, relations ARIA, fermeture du dernier onglet à `:1090`. G-25/G-55/G-78 restent à réaliser. |
| R2-M4 | **Fermé en spec, durée à préciser** | `spec.md:1296–1306` conserve les données et confirme leur effacement séparé ; réautorisation sans redémarrage à `:1289` ; TO-1 inclut le retrait. « Récemment retirées » disparaît toutefois à la fermeture : ne pas vendre une récupération entre sessions sans conserver le paquet ou définir une réimportation. Nielsen 3 ; mineur, non bloquant pour une annulation disponible sans délai pendant la session. |
| R2-M5 | **Partiellement fermé** | `spec.md:1308–1346` : consentement complet, copie chiffrée, local durable, migration vérifiée, activité ; cohérent avec le stockage §6.7. Budget d'arrêt contradictoire : R3-M2. |
| R2-M6 | **Fermé pour les exemples R2** | Neuf états `spec.md:543`, revalidation à l'envoi `:560`, états du gestionnaire `:1272–1290`, nom/abandon `:1851–1859`. Nouveaux modèles et aide demandent encore leurs états : R3-M7. |
| R2-M7 | **Fermé** | T-5 `spec.md:101–104` distingue politique et échec ; les usages interdits ne sont plus exigés en échec dans chaque mode. Recalcul ci-dessous. |
| R2-M8 | **Partiellement fermé** | Une action par toast `spec.md:789` ; intégration `:1959`, export `:2002`. Priorité critique corrigée en §3.5, mais anciens rangs subsistent dans les parcours : R3-M2. |
| R2-m1 | **Fermé en spec** | T-1 TS limité aux couleurs/ombres `spec.md:79–81`, coloration par priorité et budget §4.28, attribution par zone à 360px `:1426`. Mesures encore à produire (§10). |
| R2-m2 | **Fermé comme déclaration d'état** | `spec.md:2496–2502` dit explicitement que rien n'est ratifié. Arbitrages Q conservés ; aucune adoption implicite du glossaire ou des principes. |

| Fable round 2 | Conclusion vérifiée | Preuve / réserve |
|---|---|---|
| B1 (a–e) | **Partiel** | Classes, HC résoluble, Apparence et thème actif ajoutés (§2.10, §4.31). Aucune pseudo-clé HC dans les paires. R3-B1/R3-M6 empêchent la clôture du contrat complet. |
| B2 | **Couverture fermée, cohérence partielle** | §4.31 : 21 lignes couvrent les 22 lignes de l'architecture §15.4 (stockage et miroir réunis), dont les deux sélecteurs. Attribution différenciée. Reprise de §15.13 incomplète : R3-M2/M3. |
| M1 | **Erreur de statut fermée ; G-0 ouvert** | Même preuve que Codex R2-B2. |
| M2 | **Fermé pour l'existence des preuves** | §10 marque les fichiers futurs à écrire ; preuves syntaxe/e2e existantes explicitement limitées. Pas de preuve générale de conformité. |
| m1 | **Fermé** | Politique forbidden explicite T-5. |
| m2 | **Fermé en spec** | Hexadécimaux canoniques `spec.md:202–205` ; recalcul JSON concordant. Parité CSS future, pas acquise. |
| m3 | **Chemins corrigés** | Inventaire : `workspace/workspace.svelte.ts`, `ui/icons/FileIcon.svelte`, `ui/shell/StatusBar.svelte`, G-61 avec les lignes supplémentaires. Nouvelle obsolescence G-81 : R3-m1. |
| m4 | **Fermé** | B/W/L, A-20 `spec.md:2329`, couverture 2.1.4 ; portée présentation dans `frontend/src/present/keys.ts:61–69`. |
| m5 | **Fermé** | Infobulle sombre explicite en §2.2 ; 20,2392 recomputé sur `--view-bg`. |
| m6 | **Fermé en spec** | `spec.md:2026` exige `lang` pour Français/English, G-79 ouvert pour le code. |
| m7 | **Fermé** | §4.13 autorise l'exception HIG Search ; `frontend/src/ui/search/SearchField.svelte:55–69` reste cohérent avec cette exception. |
| m8 | **Fermé en spec** | FT-3 simplifié ; TO-1 `spec.md:806–807` met le rappel dans l'aide/l'infobulle, plus dans la phrase du toast. |

Les conclusions de clôture R1 reprises par Fable R2 ne deviennent pas des preuves de livraison : notamment régions live, focus, onglets, accueil et jetons restent des écarts explicites de l'inventaire.

## 2. Contrastes : recalcul indépendant

Méthode : résolution `$value`/`modes`, repli HC sur clair/sombre ; alpha composé en sRGB sur `over`, puis premier plan sur le fond effectif ; linéarisation au seuil 0,04045, luminance 0,2126R + 0,7152G + 0,0722B ; rapport (Lmax+0,05)/(Lmin+0,05). Aucune valeur intermédiaire arrondie. Les couleurs littérales telles que `#ffffff` sont résolues comme couleurs, non comme noms de jetons.

**101 paires × 4 modes = 404 rapports recalculés.** Aucun rapport non interdit sous `min` ; aucun écart aux valeurs publiées supérieur à 0,02. Cela valide les paires déclarées, pas l'exhaustivité du contrat. Échantillon de 22 paires distinctes ci-dessous (C/S = clair/sombre, HC-C/HC-S = contraste élevé). Normes : WCAG 1.4.3, 1.4.11.

| Premier plan / fond (parent si translucide) | C | S | HC-C | HC-S | Seuil / politique |
|---|---:|---:|---:|---:|---|
| fg / view | 12,5673 | 16,8155 | 12,5673 | 16,8155 | 4,5 |
| fg / window | 12,2232 | 15,8513 | 12,2232 | 15,8513 | 4,5 |
| fg / sidebar | 11,1820 | 13,5220 | 11,1820 | 13,5220 | 4,5 |
| fg / popover | 12,5673 | 12,0301 | 12,5673 | 12,0301 | 4,5 |
| fg-dim / view | 4,7422 | 5,9314 | 17,4178 | 13,7771 | 4,5 |
| fg-dim / window | 4,6942 | 5,7511 | 16,7816 | 13,0476 | 4,5 |
| fg-dim / sidebar | 4,5390 | 5,2387 | 14,9223 | 11,2530 | 4,5 |
| fg-dim / popover | 4,7422 | 4,8654 | 17,4178 | 10,0847 | 4,5 |
| fg / selected (view) | 10,7928 | 12,4865 | 10,7928 | 12,4865 | 4,5 |
| fg / selected (sidebar) | 9,5602 | 9,8677 | 9,5602 | 9,8677 | 4,5 |
| fg-dim / selected (view) | 4,4767 | 4,9831 | 14,2518 | 10,4435 | interdit |
| accent / view | 6,0719 | 9,9290 | 6,0719 | 9,9290 | 4,5 |
| accent / sidebar | 5,1000 | 7,9843 | 5,1000 | 7,9843 | 4,5 |
| accent-fg / suggested | 4,7695 | 4,7695 | 4,7695 | 4,7695 | 4,5 |
| blanc / destructive-bg | 4,8257 | 6,1081 | 4,8257 | 6,1081 | 4,5 |
| warning / view | 6,0799 | 10,4821 | 6,0799 | 10,4821 | 4,5 |
| error / view | 6,3245 | 7,8506 | 6,3245 | 7,8506 | 4,5 |
| warning / warning-bg (window) | 5,5252 | 7,1361 | 5,5252 | 7,1361 | 4,5 |
| error / error-bg (window) | 5,3783 | 5,6750 | 5,3783 | 5,6750 | 4,5 |
| on-dark-fg / toast (view) | 14,5120 | 12,0301 | 14,5120 | 12,0301 | 4,5 |
| on-dark-fg / tooltip (view) | 12,5673 | 20,2392 | 12,5673 | 20,2392 | 4,5 |
| accent / stage | 3,4586 | 12,3998 | 3,4586 | 12,3998 | interdit par politique |

## 3. Constats restants et nouveaux

### R3-B1 — bloquant — le validateur accepte des thèmes qui rendent des contrôles illisibles

**Normes :** WCAG 1.4.3/1.4.11, Nielsen 5 ; N22. **Preuves :** `tokens.json:355–376` rend `--btn-bg` et `--hover-bg` remplaçables ; aucune paire du tableau `contrast.pairs` n'utilise ces fonds. Même absence pour `--card-bg` et `--page-bg`. TH-D2 (`spec.md:342–349`) ne contrôle que ce tableau.

Contre-exemple calculable : fournir uniquement `--hover-bg = #000000` en mode light laisse **les 404 rapports déclarés inchangés**. Le texte clair `--fg = rgb(0 0 6 / 80%)`, composé sur ce fond noir, devient `rgb(0 0 4.8)` : contraste environ **1,002:1**, au lieu de 4,5. C'est une couleur valide, sans violation du contrat de type. Le même problème existe sur un bouton utilisant `--btn-bg` noir. « Un thème validé ne peut pas être illisible par construction » (`spec.md:366`) est donc faux.

**Correction :** compléter les couples réels par composant/état et leurs parents autorisés : repos, survol, pression, sélection, focus adjacent, cartes, widgets et pages applicatives. Si un fond ne peut pas être validé exhaustivement, le verrouiller. Inclure ce contre-exemple dans les critères de refus et le test prévu. Les paires HC et forbidden sont désormais correctes ; ne pas les refaire inutilement.

### R3-M1 — majeur — diagramme nommé, mais sans alternative au dessin

**Normes :** WCAG 1.1.1, Nielsen 2/6. **Preuve :** `spec.md:1530–1531` donne seulement « Diagramme Mermaid, ligne 12 » comme nom et appelle le code source « texte alternatif complet ». Le code DOT/Mermaid décrit une syntaxe ; il ne restitue pas nécessairement le sens des relations, les groupes ni les conclusions du diagramme. La personne sans connaissance du langage perd l'information visuelle.

**Correction :** prévoir un résumé éditable lié au diagramme et, pour les graphes complexes, une description longue ou une vue textuelle structurée des nœuds/relations. La conserver et la partager avec le rendu, y compris à l'export. Ne pas forcer l'auteur à apprendre le langage pour lire le dessin. Garder le code accessible comme source supplémentaire. Le flux rendu/cache/dernier rendu partagé, Graphviz intégré et Mermaid installable est sinon cohérent avec architecture §15.14.

### R3-M2 — majeur — trois contrats d'extensions/parcours divergent encore de l'architecture

**Normes :** Nielsen 1/4/5 ; N21–N24.

- **Mode sans extensions :** `spec.md:1498–1500` promet « Aucune extension n'est chargée ». Architecture `:1656` conserve les extensions intégrées telles que livrées ; `:1399` inclut l'éditeur et l'aperçu parmi les essentielles. Sans elles, la promesse de récupération d'un éditeur utilisable ne tient pas. Dire explicitement « sans greffons ajoutés », intégrées rétablies par défaut, thème Adwaita ; définir le registre obtenu.
- **Arrêt :** `spec.md:1348` attend 1s **par extension** ; architecture `:1452`, `:1568` exige 1s **au total**, en parallèle. Aligner le délai visible et la reprise ; autrement dix extensions autorisent dix secondes imprévues.
- **Bandeaux :** §3.5 place bien les états critiques avant les questions (`spec.md:502–514`), mais §5.4 `:1916` appelle encore une demande d'accès « priorité 1 » ; §5.16 `:2071–2075` conserve disque (2), question (1), information (3). `inventory.md` vague 2 dit encore « questions d'abord ». Supprimer ces anciens rangs ou les remplacer par les nouveaux. Sans cela R2-M8 n'est pas réellement fermé.

Les 22 points de §15.4 sont bien représentés ; le problème est désormais la cohérence des règles, plus l'absence massive de points.

### R3-M3 — majeur — menu Extensions : deux actions par ligne sans modèle clavier valide

**Normes :** APG Menu, WCAG 2.1.1/4.1.2, Nielsen 4. **Preuve :** `spec.md:1358–1365` impose APG Menu Button, une ligne activable, et une punaise séparée `aria-pressed`. Le contrat global §4.2 prévoit les flèches entre items, pas l'accès à un bouton imbriqué. On ne sait pas comment atteindre la punaise, la basculer sans lancer l'action, puis revenir. Un bouton de bascule n'est pas un `menuitemcheckbox`.

**Correction :** choisir un popover avec lignes et boutons tabulables, ou un vrai menu avec actions distinctes (action principale et « Épingler X » en `menuitemcheckbox`). Définir ouverture, navigation, fermeture et retour du focus. Reprendre aussi la limite **six épinglées** d'architecture `:1701`, absente de §4.30, et le débordement. Le geste personnel et la portée projet sont correctement spécifiés.

### R3-M4 — majeur — cartes interactives dans une listbox et aides riches sans accès au contenu

**Normes :** APG Listbox/Tooltip, WCAG 2.1.1/2.4.3/4.1.2. **Preuves :** galerie « listbox APG à deux dimensions » `spec.md:2172`, réutilisée avec un menu **par carte** `:2213`. Une option de listbox ne doit pas contenir un menu interactif ; le modèle de flèches d'une grille bidimensionnelle n'est pas défini par cette phrase. D'autre part, §4.37 `:1767–1779` et §4.36 `:1590–1597` mettent lien, section repliable, Copier et corrections dans des cartes ; `:1806–1817` maintient le focus dans le texte sans définir comment entrer dans la carte et en sortir. `aria-describedby` ne rend pas ces actions opérables.

**Correction :** galerie en liste/grille appropriée avec actions clairement accessibles, ou menu de la sélection hors options ; définir la navigation à deux dimensions si retenue. Pour les cartes riches, définir un accès au clavier vers leur contenu (popover/dialogue non modal), le retour au texte et Échap. Conserver le focus dans l'éditeur pour la seule liste de complétion. Aucune multiplication de widgets n'est nécessaire.

### R3-M5 — majeur — exception de reflow encore fondée sur une préférence

**Norme :** WCAG 1.4.10. **Preuve :** `spec.md:485–486` exempte « une ligne de code dont le retour à la ligne a été coupé par la personne », alors que `:479` impose le retour sous 560px quel que soit le réglage. Une préférence de non-retour n'établit pas un besoin intrinsèque de deux dimensions.

**Correction :** retirer cette exception générale, conserver les véritables contenus 2D et le retour forcé du texte ordinaire. La majeure partie de R2-M2 est corrigée.

### R3-M6 — majeur — version de contrat affichée, compatibilité non définie

**Normes :** Nielsen 5/9, N22/N26 ; cohérence architecture §15.8 et §15.16 (protocole à venir). **Preuves :** `tokens.json:15` annonce 1.0 ; `spec.md:337–338` ne définit que jetons absents/inconnus. Aucun comportement pour un thème exigeant 2.0, ni distinction champs requis/optionnels ; plusieurs `bounds` sont des phrases (« idem », « 0–12px »), pas une validation structurée. La table TH-D1 ne suffit pas à rendre le contrat calculable de façon univoque.

**Correction :** définir version requise et règle de compatibilité, refus expliqué d'une version majeure incompatible, et contraintes de valeur explicites par type. Distinguer un jeton inconnu facultatif d'une exigence indispensable. L'API complète peut rester dans N26, mais le parcours de refus et les règles du thème doivent être cohérents dès ce design.

### R3-M7 — majeur — nouveaux parcours : erreurs, annulation et résultats périmés non définis

**Normes :** Nielsen 1/3/5/9, WCAG 3.3.1/3.3.3 ; N17/N27–N29. **Preuves :** l'assistant §5.19 (`spec.md:2191–2211`) décrit seulement le succès. Pas d'état disque plein, droits révoqués pendant l'enregistrement d'équipe, annulation en étape 2/3, absence d'aperçu pour la vignette, ni dépendance omise dans la sélection de fichiers. §4.36–4.37 ne dit pas comment une correction rapide ou une complétion reçue après une édition concurrente est invalidée ; le délai seul ne garantit pas une plage toujours correcte.

**Correction :** courte table d'états ciblés : conserver les saisies, montrer le champ/fichier en cause, réessayer, sauvegarder personnellement si les droits d'équipe ont changé ; revenir/annuler sans créer de modèle partiel ; expliquer le repli de vignette. Pour LSP et corrections, lier résultat et version du document, remapper ou abandonner les résultats périmés sans modifier une mauvaise plage. Définir aussi absence/indisponibilité de tinymist : écriture toujours possible, aide manuelle réessayable, pas de spinner permanent.

### R3-m1 — mineur — preuve frontend obsolète dans le nouveau périmètre N28

**Norme :** Nielsen 1 ; principe local de preuve. **Preuves :** `spec.md:1703–1706`, `inventory.md` G-81 disent « pas de colonne » et 400 caractères. Le code actuel `frontend/src/preview/protocol.ts:18` a `column?: number`, `:136` tronque à **4000**, `:141–142` décode la colonne. Plage de fin, conseils, corrections et trace restent absents : G-81 n'est donc pas fermé.

**Correction :** actualiser le relevé et ses références. Le protocole est déjà partiellement amélioré ; ne pas redemander ce qui existe.

### R3-m2 — mineur — sur-ingénierie de l'enregistrement comme modèle

**Normes :** Nielsen 8, GNOME HIG Make it Simple/Reduce User Effort. **Preuves :** `spec.md:2128` demande une description fr **et** en ; `:2193–2210` impose trois étapes avec type, étiquettes, charte séparée, polices et vignette ; `:2227` ajoute une liste de projets consommateurs. Ces moyens dépassent le besoin « simple à créer et enregistrer » de N27 pour une personne voulant conserver son projet comme point de départ.

**Correction :** nom + description dans la langue de la personne + emplacement, génération automatique du reste ; options avancées repliées. Ne pas rendre une traduction obligatoire. Garder l'extraction de charte et la traçabilité en fonctions avancées. Les modèles restent tous créés par les personnes : le projet d'exemple du premier lancement est explicitement distinct (§5.2), donc pas de violation constatée de « aucun modèle intégré ».

## 4. Couverture transversale de l'expérience

Cette table couvre les domaines N17/N19 sans prétendre tester l'interface exécutée. Les 10 heuristiques, les critères A/AA de `coverage.md`, les motifs APG et les domaines HIG ont été considérés ; les états S ne valent que dans la portée de leurs preuves. Les écarts G existants sont maintenus, pas effacés parce qu'une section existe.

| Domaine | Évaluation / preuve et limite |
|---|---|
| Identité GNOME, hiérarchie, typo, espacements, icônes, mouvement | §1–2, §6, §8 : système cohérent, valeurs canoniques clarifiées. Couleurs forcées et mouvement réduit prévus ; rendu et parité toujours à vérifier. R3-B1 sur thèmes. |
| IA, accueil, projets et vides | §3, §4.20, §5.1–5.2 : entrées/sorties et nom invalide décrits. G-39/G-40 ouverts ; aucun accueil livré. |
| Navigation multifichier, arbre, onglets, recherche | §4.16/24/25, §5.5–5.6 : proposition cohérente, focus complété. G-0 reste confirmé dans App. Recherche non montée, G-38. |
| États, clavier, souris, toucher, drag, focus | §4, §3.3, §7 : neuf états, alternatives au glisser, cibles, F6. R3-M3/M4/M7 pour nouveaux widgets ; pas d'essai tactile/Orca. |
| Textes, erreurs, reprise, annulation | §4.11–13, §5.16, TO-1 : reprise/annulation distinguées ; R3-M2 sur rangs, M7 sur nouveaux échecs. |
| Collaboration, présence, invitations, rôles, divergences, accès retiré | §4.14–15, §5.3–4/14–15 : copie conservée, droits visibles, confirmations et annulation prévues. G-26/G-27/G-41/G-43 restent ouverts ; pas de simulation WAN réalisée. |
| Aperçu, export, impression, présentation | §4.18/23, §5.8–12 : dernier rendu correct, accès source, export et vue public décrits. Impression Q10 ; PDF/UA non prouvé. §5.12 dit encore « Voir les problèmes » quand §4.36 exige le saut direct : utiliser explicitement la même commande de saut. Nielsen 4/9, N28 ; clarification mineure. |
| Types txt/csv/md/typst, autres langages | §4.28, §5.8–10 : édition/rendu différenciés, code lecture d'abord, budgets paresseux proportionnés. Pas de demande de transformer toute l'application en IDE. |
| Thèmes, HC et contrastes | §2.10 : 404 calculs concordants mais validation non exhaustive, R3-B1. Version/compatibilité R3-M6. |
| Extensions, points d'affichage, projet, zones protégées, sûreté | §4.29–33 couvre les points de §15.4, consentement/provenance et contrôle personnel ; R3-M2/M3. « Donner à X » pour un raccourci réservé (§4.33) devrait signifier remapper explicitement, pas permettre de supprimer l'accès protégé ; Nielsen 4/5. |
| Diagrammes Mermaid/Graphviz | §4.34 cohérent sur source, rendu isolé, cache, collaboration, panne et complexité ; alternative textuelle insuffisante R3-M1. |
| Développement et protocole | §4.35 : local, visible, désactivable, inspecteur avec limites ; API publique N26 reste à écrire, pas confondue avec une preuve. N25 n'impose pas d'exposer ces détails à tous. |
| Modèles N27 | §5.19 couvre créer/modifier/dupliquer/supprimer/importer/exporter/partager, absence intégrés, personnel/équipe, copie/version. R3-M4/M7/m2. |
| Diagnostics N28 | §4.36 propose un clic, F8/Maj+F8, Ctrl+., corrections annulables, messages clairs et originaux, imports en lecture et lien à l'appel. Bon choix du popover existant ; R3-M4/M7 et relevé obsolète m1. Cas diagnostic sans position (autorisé par `protocol.ts:11–18`) : définir un repli explicite vers fichier/appel/détails sans inventer une ligne ; Nielsen 9, mineur. |
| Aide Typst N29 | §4.37 couvre fonctions importées/personnelles, complétion, signature, hover, `///`, définition et retour. Intelligence déléguée à tinymist ; ce document n'est pas une preuve que chaque champ docstring est effectivement fourni par la version embarquée. Valider par un exemple de charte multifichier lors du branchement G-84 ; R3-M4/M7. |
| fr/en, fenêtres étroites, zoom et espacement | §3.4, §5.13, §6–7 : langue interface/passages, noms de langue, reflow ; R3-M5. Captures 360px/200 %, test 320px et WebKitGTK restent planifiés. |
| Performance perçue et gros fichiers | §5.17, §4.28/31/34 : local indépendant du réseau, seuils et budgets explicites. Aucun chiffre mesuré nouveau ; statut S→N correct. R3-M2 pour délai d'arrêt global. |
| Couverture et décisions | §10–11, coverage/inventory/CHANGES : nette amélioration de l'honnêteté des preuves. Q non ratifiées conservées ; nouveaux tests N27–N29 à ajouter explicitement au tableau §10, au-delà de la ligne générique parcours. Nielsen 1, mineur. |

## Verdict

**NOT ACCEPTED** — blocage R3-B1 : un thème accepté peut encore rendre des états interactifs illisibles. Avant accord sur la proposition, résoudre également R3-M1 à R3-M7 : alternative des diagrammes, cohérence architecture/sûreté/priorités, clavier du menu Extensions, cartes et galerie accessibles, reflow, compatibilité du thème et états de récupération des nouveaux parcours. G-0 demeure un blocker applicatif ouvert ; son statut documentaire est enfin exact. Les corrections documentaires confirmées ci-dessus ne constituent pas une clôture des écarts frontend ni une ratification des décisions Q.
