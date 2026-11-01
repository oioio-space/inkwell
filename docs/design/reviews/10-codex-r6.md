# Revue Codex — round 6, révision 6 — 2026-11-01

Revue documentaire de `spec.md`, `CHANGES.md`, `tokens.json`, `coverage.md` et `inventory.md`, confrontée aux revues `08-codex-r5.md` et `07-fable-r5.md`. Aucune revue concurrente du round 6 consultée. Seul ce fichier est créé ; aucun changement applicatif, aucun commit. Le verdict porte sur la conception, sans transformer les tests « à écrire » en preuves de livraison.

## Vérifications des corrections

| Demande | Résultat et preuve en rév. 6 |
|---|---|
| Codex R5-B1, contrat de contraste | **Fermé en conception** : §2.10 interdit les combinaisons non vérifiables ; §4.37:2003 utilise `--fg-dim-on-state` ; 161 paires, dont 12 pour ce jeton sur les états. Recalcul ci-dessous. G-100 et G-71 restent ouverts côté code. |
| Codex R5-M1, secours | **Fermé en conception** : §4.43:2587–2647 définit une phrase par personne, une seule création, impression/fichier, vérification assistée, report possible, kit facultatif réécrit en silence et restauration par phrase seule avec avertissement de fraîcheur. §4.44 renvoie à ce parcours. |
| Codex R5-M2, propositions | **Partiel** : §4.41 définit focus, sortie, choix locaux et transaction unique, mais §4.39 conserve le contrat opposé ; R6-M1. |
| Codex R5-M3, historique | **Fermé en conception** : §4.22:1095–1179 impose options sans boutons, barre d'actions externe, sélecteurs De/À, curseur nommé, résumés factuels, export technique hors de l'interface d'écriture. |
| Codex R5-M4, macOS | **Partiel** : §8.2 fournit les trois liaisons explicites, sans Ctrl→Cmd automatique, sans liaison active Ctrl+Alt ni Alt+alphanumérique Windows ; les contradictions de navigation et de réservation subsistent, R6-M2. |
| Codex R5-m1, résidus | Nettoyage réel dans la spec et les vagues de l'inventaire ; incomplet dans la couverture, R6-m1. |
| Fable R5-M1 | Anciennes collisions AltGr, ⌥⌘H et ⌃Espace supprimées ; tables Apple/Microsoft/GNOME ajoutées. Clôture complète refusée pour R6-M2. |
| Fable R5-M2 | **Fermé pour l'UX** : §4.22:1158 définit une variante du projet entier, index compris, avec repères des fichiers modifiés. L'alignement HI-6 reste explicitement à faire dans CHANGES, sans preuve d'implémentation. |
| Fable R5-M3 | **Fermé** : phrase personnelle unique et défaut appliqué au partage et à Rejoindre (§4.43, §5.3–5.4). |
| Fable R5-M4 | **Partiel** : export, glossaire et décisions nettoyés ; la ligne N28 de couverture garde les anciennes prescriptions, R6-m1. |
| Fable R5-m1 | **Fermé** : §4.22, délai de 300 ms ou Entrée, annonce à l'arrêt. |
| Fable R5-m2 | **Fermé** : aperçu = confirmation ; restauration ordinaire annulable, sans dialogue supplémentaire. |
| Fable R5-m3 | **Fermé** : kit silencieux, avertissement sur échec ; collage des 24 mots et complétion à la récupération. |
| Fable R5-m4 | **Fermé pour les usages signalés** : variante en filet neutre nommé, version en losange ; les couleurs proches ne sont plus seules à distinguer ces rôles. |
| Fable R5-m5 | **Fermé** : sélecteur Variante anatomisé dans la zone centrale protégée de §4.1. |
| Fable R5-m6 | **Fermé dans les sections corrigées** : proposition ajoutée aux questions L-8 ; touches neutralisées dans un champ en §4.41 ; Git absent de l'interface d'écriture. Réserve globale sur les propositions : R6-M1. |
| Fable R5-m7 | **Fermé** : §5.13 a six pages et les portées explicites ; enregistrement et destination de sauvegarde locaux. |
| Fable R5-m8 | **Fermé** : §3.1 admet explicitement au plus une quatrième destination Extensions, sous condition. |
| Fable R5-m9 | **Fermé** : §5.9 conserve Adwaita et Lecture, pas Rapport comme style livré. |
| Fable R5-m10 | **Fermé en conception** : horloge en §4.22/8.3 ; dette réelle suivie par G-101. |
| Fable R5-m11 | **Fermé documentairement** : S28 donne les entrées 01/02 ; §4.22 distingue la restauration additive d'Inkwell du remplacement de Time Machine. |

## Contrastes recalculés indépendamment

Python en mémoire, lecture seule du JSON : résolution des modes avec repli HC clair/sombre, composition alpha en sRGB du fond sur `over`, puis du premier plan sur ce fond, linéarisation à 0,04045 et luminance pondérée 0,2126/0,7152/0,0722. Aucun arrondi intermédiaire.

**161 paires × 4 = 644 rapports recalculés**, dont 15 paires interdites par politique et 3 `hcOnly`. Aucun échec aux seuils applicables des paires autorisées ; aucun écart supérieur à 0,011 avec les rapports publiés. Les paires interdites ne sont pas supposées échouer numériquement. Les valeurs ordinaires des bordures `hcOnly` ne sont pas des échecs.

Normes : [WCAG 2.2, 1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html), [1.4.11](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html), seuils HC du contrat TH-D2. Échantillon de **19 paires** ; les noms sont les suffixes des jetons CSS.

| Premier plan / fond / parent | Clair | Sombre | HC clair | HC sombre |
|---|---:|---:|---:|---:|
| fg-dim-on-state / hover-bg / view-bg | 5,8639 | 6,9102 | 15,1700 | 11,4435 |
| fg-dim-on-state / hover-bg / headerbar-bg | 5,8639 | 5,7903 | 15,1700 | 9,1736 |
| fg-dim-on-state / hover-bg / dialog-bg | 5,7784 | 5,2980 | 14,6156 | 8,2246 |
| fg-dim-on-state / hover-bg / popover-bg | 5,8639 | 5,2980 | 15,1700 | 8,2246 |
| fg-dim-on-state / hover-bg / window-bg | 5,7784 | 6,5737 | 14,6156 | 10,7427 |
| fg-dim-on-state / hover-bg / sidebar-bg | 5,5069 | 5,7903 | 12,9980 | 9,1736 |
| fg-dim-on-state / selected-bg / popover-bg | 5,7204 | 4,9250 | 14,2518 | 7,5220 |
| fg-dim-on-state / selected-bg / headerbar-bg | 5,7204 | 5,3739 | 14,2518 | 8,3692 |
| fg-dim-on-state / selected-bg / dialog-bg | 5,6344 | 4,9250 | 13,7313 | 7,5220 |
| fg-dim-on-state / selected-bg / side-bg-2 | 5,5101 | 5,7318 | 13,0160 | 9,0594 |
| fg-dim-on-state / selected-bg / view-bg | 5,7204 | 6,4275 | 14,2518 | 10,4435 |
| fg-dim-on-state / selected-bg / sidebar-bg | 5,3618 | 5,3739 | 12,2140 | 8,3692 |
| accent / hover-bg / view-bg | 5,1948 | 8,1288 | 5,1948 | 8,1288 |
| accent / hover-bg / headerbar-bg | 5,1948 | 6,4230 | 5,1948 | 6,4230 |
| accent / hover-bg / dialog-bg | 4,9828 | 5,7198 | 4,9828 | 5,7198 |
| accent / hover-bg / popover-bg | 5,1948 | 5,7198 | 5,1948 | 5,7198 |
| fg / view-bg / — | 12,5673 | 16,8155 | 12,5673 | 16,8155 |
| fg / window-bg / — | 12,2232 | 15,8513 | 12,2232 | 15,8513 |
| fg / sidebar-bg / — | 11,1820 | 13,5220 | 11,1820 | 13,5220 |

## Constats restant à corriger

### R6-M1 — Majeur — deux contrats incompatibles pour les propositions IA

**Normes :** Nielsen 3 (contrôle), 4 (cohérence), 5 (prévention), [APG Listbox](https://www.w3.org/WAI/ARIA/apg/patterns/listbox/) (les options ne prennent pas en charge des descendants interactifs), WCAG 4.1.2 ; C24 et §4.41.

**Preuve :** `spec.md:2517–2521` rend les choix locaux et publie une seule modification à « Appliquer ». Mais `:2240–2258` dit que les collègues ne voient rien avant « Accepter », puis « Accepter = une modification » et propose de défaire toutes les modifications IA de la séance. `:2386` prescrit encore une Listbox de morceaux avec boutons par Tab, tandis que `:2502–2509` prescrit des groupes focalisables contenant ces boutons. Le comportement, la transaction et la sémantique accessible restent donc doubles. Enfin `:2526–2527` ajoute « Accepter et appliquer » pour un morceau sans expliquer si le bouton Appliquer de la barre disparaît : le chemin demandé avec un seul Appliquer n'est pas garanti.

**Correction :** faire de §4.41 le contrat effectif partout : §4.39 renvoie à la relecture commune, choix sans écriture partagée, groupes avec boutons, publication unique. Définir un unique contrôle final « Appliquer », y compris pour un morceau, et retirer l'ancien comportement par morceau. Harmoniser la ligne APG de `coverage.md:112`.

### R6-M2 — Majeur — le registre par système se contredit encore

**Normes :** N35, K-6/K-7, Nielsen 4 et 5 ; [Apple, Mac keyboard shortcuts](https://support.apple.com/en-us/102650) (⌘P imprime), [GNOME, Standard Keyboard Shortcuts](https://developer.gnome.org/hig/reference/keyboard.html), [Microsoft, Keyboard](https://learn.microsoft.com/windows/win32/uxguide/inter-keyboard).

**Preuve :** `spec.md:479` donne explicitement **Cmd+P** pour Aller au fichier sous macOS ; `:3260–3261` donne **⇧⌘O** pour cette action et **⌘P** pour Imprimer. Ce n'est pas une simple notation Linux abrégée : la première ligne nomme expressément macOS. Le registre ne peut alimenter les deux comportements. Autre contradiction interne : Aide utilise **⇧⌘?** (`:3293`), mais cette liaison figure dans les réservés (`:3303`) que K-6 DOIT refuser sans exception (`:3306–3310`). Une commande Aide du menu système peut être légitime ; son traitement comme liaison système déléguée doit être explicite pour que le test ne la rejette pas.

Les tables et l'interdiction Ctrl+Alt ferment bien les anciennes collisions. Les réservations Apple et les conventions GNOME ont été consultées directement ; l'ouverture de la page Microsoft a échoué dans l'outil web, sa règle est corroborée ici par S31 et les deux revues antérieures, sans prétendre à une nouvelle lecture en ligne.

**Correction :** aligner §3.2 sur ⇧⌘O ; distinguer les commandes déléguées au système des raccourcis capturés par le cœur, notamment Aide. Faire tester les liaisons affichées dans les parcours contre la même table que les menus. Les différences GNOME intentionnelles (Ctrl+P pour Aller au fichier, par exemple) doivent rester des choix explicites, pas être présentées comme des conventions GNOME.

### R6-m1 — Mineur — la couverture conserve les prescriptions retirées

**Normes :** Nielsen 4 ; traçabilité de §10, K-6 ; GNOME/GTK [Text.insert-emoji](https://docs.gtk.org/gtk4/signal.Text.insert-emoji.html).

**Preuve :** `coverage.md:176`, malgré son préfixe « rév. 6 », prescrit encore « liste Problèmes (popover existant) » et « F8 et Ctrl+. ». La même ligne donne pourtant Relire et Alt+Entrée ; §4.36:1825 interdit Ctrl+. pour cause de sélecteur GTK. Ce résidu était expressément visé par Fable R5-M4. La colonne des preuves peut légitimement décrire le popover existant ; la description du comportement cible ne le peut pas.

**Correction :** retirer ces deux prescriptions de la description cible, conserver le popover uniquement comme preuve du code actuel et G-93 comme migration.

## Régression et limites

Pas de nouvelle régression numérique dans les 644 rapports. Le secours est plus simple et accessible, l'historique ne demande ni Git ni sélection multiple, les propositions et les raccourcis ont des règles centrales nettement meilleures. Les deux contradictions majeures empêchent toutefois de conclure que leurs anciens constats sont clos dans tout le document. Les dettes applicatives G-0, G-71, G-85, G-89–101 restent déclarées ; cette revue ne les accepte pas comme fonctionnalités livrées. Les alignements HI-5/HI-6 et de la barre de mise en forme hors du dossier sont explicitement ouverts dans CHANGES ; ils ne constituent pas des preuves de réalisation.

Décompte : 0 nouveau bloquant de contraste établi, 2 majeurs, 1 mineur. Les majeurs suivants bloquent l'accord documentaire.

NOT ACCEPTED — blockers : R6-M1 (transaction et sémantique des propositions contradictoires ; contrôle final unique non garanti), R6-M2 (Cmd+P contradictoire et Aide à la fois liée et interdite).
