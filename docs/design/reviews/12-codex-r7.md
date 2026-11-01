# Revue Codex — round 7, révision 7 — 2026-11-01

Revue documentaire de `spec.md` rév. 7, `CHANGES.md` et `tokens.json`, confrontée aux deux revues du round 6 (`10-codex-r6.md`, `09-fable-r6.md`). Vérification ciblée de `coverage.md:112,176` pour les corrections annoncées. Aucune revue concurrente du round 7 consultée. Seul ce fichier est créé ; aucun commit. Le verdict porte sur la conception, pas sur la livraison des tests encore « à écrire ».

## Fermeture des constats

| Constat antérieur | Vérification et preuve | Résultat |
|---|---|---|
| Codex R6-M1 : propositions contradictoires | §4.39 point 2 (`spec.md:2246–2263`) adopte intégralement §4.41 : groupes focalisables, choix locaux, publication unique à « Appliquer ». Le point 11 (`:2381–2391`) abandonne la Listbox. §4.41 (`:2518–2537`) garde un seul contrôle final, même pour un morceau ; « Annuler cette discussion » ne défait que les changements appliqués. | Fermé en conception |
| Codex R6-M2 : raccourcis contradictoires | §3.2:484 et §8.2:3274 donnent ⇧⌘O ; ⌘P imprime. Aide est déléguée au menu système, sans capture (`:3307`), avec exception explicite dans K-6 (`:3321–3326`). | Fermé en conception |
| Codex R6-m1 : couverture N28 | `coverage.md:176` prescrit Relire › Problèmes, Alt+Entrée et F8 ; le popover reste seulement une preuve du code actuel. La ligne Listbox (`:112`) exclut les propositions. | Fermé |
| Fable M1 : faux contre-exemple de thème | §2.10:386–394 et §10:3452 distinguent désormais `#acacac` refusé et `#bbbbbb` accepté. Recalcul indépendant ci-dessous. | Fermé |
| Fable m1 : blanc brut | La paire sur `--destructive-bg` utilise `--on-dark-fg`, verrouillé blanc par TH-D1 ; rapports 4,8257 / 6,1081. | Fermé |
| Fable m2–m4 : résidus | Export nommé « format technique », nom du format sous Détails techniques ; vocabulaire consolidé ; `--variant-line` retiré du JSON et de ses paires. | Fermés |
| Fable m5 : secours trop insistant | §4.43:2636–2649 : état dans Préférences et popover ; bandeau uniquement pour phrase pas notée ou non vérifiée, fermable, rappel au lancement suivant après 7 jours ; aucun bandeau pour kit absent. | Fermé |
| Fable m6 : conventions macOS | §8.2:3283,3286 : panneau ⌥⌘S, version ⌃⌘S ; §4.22:1153 concorde. Réservations étendues à ⌃⌘D, ⌥⌘T et ⌃F1–F8 ; Zones sur ⌘F6 / ⇧⌘F6 (`:3304`). Réserve de rendu du tableau ci-dessous. | Fermé sur le fond |
| Fable m7 : sort de la variante | §4.41:2527–2529 : variante close, visible dans les variantes intégrées, possibilité de repartir de cet état. | Fermé |
| Fable m8–m9 : couverture et boutons inactifs | N28 corrigé ; §4.22:1128–1130 et §4.41:2524–2526 imposent `aria-disabled="true"`, sans `disabled`, et maintien du focus pour atteindre l'explication. | Fermés |

Références de contrôle : [APG Button](https://www.w3.org/WAI/ARIA/apg/patterns/button/), [APG Listbox](https://www.w3.org/WAI/ARIA/apg/patterns/listbox/), [WCAG 2.2](https://www.w3.org/TR/WCAG22/) (2.1.1, 2.4.3, 4.1.2), [raccourcis Apple](https://support.apple.com/en-us/102650). Ces corrections préservent contrôle, navigation clavier et cohérence ; l'implémentation reste à vérifier.

## Contrastes recalculés

Script Python exécuté en mémoire, sans fichier auxiliaire : résolution des jetons et des replis HC clair/sombre, composition alpha en sRGB du fond sur `over`, puis du texte sur ce fond ; linéarisation au seuil 0,04045, luminance 0,2126 R + 0,7152 G + 0,0722 B. Aucun arrondi intermédiaire.

**160 paires ; 634 évaluations applicables** (les 3 paires `hcOnly` uniquement dans les deux modes HC). Aucun échec de paire autorisée au seuil `min` / `hcMin`, aucun écart supérieur à 0,011 avec les valeurs publiées. Les usages `forbidden` restent interdits par politique, indépendamment de leur résultat numérique.

Seuils : texte normal ≥ 4,5 selon [WCAG 1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html), indicateurs nécessaires ≥ 3 selon [WCAG 1.4.11](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html), planchers HC supplémentaires de TH-D2. Échantillon de **16 paires distinctes**, rapports à quatre décimales :

| Premier plan / fond / parent | Clair | Sombre | HC clair | HC sombre |
|---|---:|---:|---:|---:|
| on-dark-fg / destructive-bg / — | 4,8257 | 6,1081 | 4,8257 | 6,1081 |
| on-dark-fg / toast-bg / view-bg | 14,5120 | 12,0301 | 14,5120 | 12,0301 |
| on-dark-fg / tooltip-bg / view-bg | 12,5673 | 20,2392 | 12,5673 | 20,2392 |
| on-dark-fg / stage-bg / — | 21,0000 | 21,0000 | 21,0000 | 21,0000 |
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

Contre-exemples, remplacement de `--hover-bg` en clair :

| Fond | Texte principal | Texte secondaire autorisé | Accent / anneau intérieur | Décision |
|---|---:|---:|---:|---|
| #acacac | 6,9432 | 4,4523 | 2,6748 | Refusé : secondaire < 4,5 et anneau < 3 |
| #bbbbbb | 7,9065 | 4,8141 | 3,1628 | Ces trois contrôles passent |

Le refus de `#acacac` est démontré sans invoquer `--fg-dim`, interdit sur cet état. Le passage des trois contrôles de `#bbbbbb` ne dispense pas un thème complet des autres contraintes TH-D2/TH-4.

## Constats résiduels

### R7-m1 — Mineur — assertion IA restée sur « Accepter »

**Références :** contrat §4.41 ; [WCAG 4.1.2](https://www.w3.org/TR/WCAG22/#name-role-value) pour la cohérence des contrôles à vérifier ; traçabilité de §10. Il s'agit ici d'un défaut documentaire, pas d'un échec WCAG observé dans une interface.

**Preuve :** `spec.md:3458`, ligne de test §4.39 : « rien ne part chez les collègues avant “Accepter” ». Or `:2250–2255` et `:2519–2523` placent la publication à « Appliquer » ; Accepter est un choix local. Cette ancienne assertion ne teste pas la frontière de publication nouvelle.

**Correction :** écrire « rien ne part chez les collègues avant Appliquer ; Accepter/Refuser ne publient rien ». Le contrat opérationnel est désormais univoque, ce résidu dans un test à écrire ne rouvre pas R6-M1.

### R7-m2 — Mineur — tableau des réservations coupé en deux lignes physiques

**Références :** K-6 ; [Apple, Mac keyboard shortcuts](https://support.apple.com/en-us/102650) ; [GFM, tables](https://github.github.com/gfm/#tables-extension-). Défaut de présentation documentaire, sans collision fonctionnelle démontrée.

**Preuve :** `spec.md:3317` finit par « navigation au », puis `:3318` commence par « clavier du système) ». Dans une table GFM, cette seconde ligne forme une nouvelle rangée : la fin des réservations macOS arrive dans la première colonne et les réservations Windows dans la deuxième. Le texte source comporte les bonnes réservations, mais leur attribution visuelle est dégradée.

**Correction :** garder cette rangée sur une seule ligne physique, avec éventuellement des `<br>` dans les cellules. Le registre cible et la liaison Zones restent corrects.

## Régression et verdict

Aucune régression de contraste établie. La relecture reste locale jusqu'au contrôle final unique ; les champs conservent leurs touches de saisie ; Échap garde la proposition en attente ; les choix non touchés survivent au recalcul. La frise conserve les options sans boutons imbriqués, ses actions externes et son délai de 300 ms. Le secours reste reportable, récupérable par phrase seule avec avertissement de fraîcheur, et son rappel ne dépend plus du kit facultatif. Les réservations et les liaisons macOS demandées sont cohérentes sur le fond.

Deux mineurs documentaires, aucun majeur ni bloquant restant dans ce périmètre. Les tests annoncés « à écrire » et les écarts applicatifs restent des travaux à réaliser ; cette acceptation ne vaut pas validation d'une interface exécutée.

ACCEPT
