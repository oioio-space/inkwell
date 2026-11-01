# Revue Codex — round 5, révision 5 — 2026-11-01

Revue documentaire indépendante, après la passe de cohérence ; la rév. 4 n'a pas été revue. Sources examinées : design `spec.md`, `coverage.md`, `inventory.md`, `tokens.json`, `CHANGES.md` ; revues `05-fable-r3.md` et `06-codex-r3.md` ; architecture rév. 3, notamment §6.7–6.11, §9.11, §11.1, §15.17–18 et les vingt changements de `coherence.md` §6 ; exigences N17–N41, principes UX et glossaire ; preuves frontend ciblées. Aucun fichier Fable du round 5 consulté. Aucun changement applicatif, aucun commit.

Le produit devient nettement plus calme : deux destinations à gauche, trois dans le tiroir, compteur unique, suggestions sans score, IA absente tant qu'elle n'est pas activée. L'accord reste impossible : le contrat de thèmes conserve un contre-exemple, le secours refuse un chemin de récupération valide, et plusieurs interactions nouvelles ne définissent pas assez leur portée clavier. La revue accepte des spécifications, pas une livraison : les écarts G restent ouverts.

## 1. Clôture réelle du round 3

| Constat antérieur | Conclusion et preuve actuelle |
|---|---|
| Codex R3-B1 | **Partiel, toujours bloquant** : le contre-exemple noir est refusé ; boutons, cartes et papier ont leurs paires. Les indices sur survol de §4.37 restent sans paire ; voir R5-B1. |
| Codex R3-M1 | Fermé en conception : §4.34 prévoit description éditable en phrases, description longue, source partagée, `image(alt:)`, HTML et Vue lecture. Pas de preuve PDF/UA livrée. |
| Codex R3-M2 | Fermé en conception : §4.33 rétablit les intégrées telles que livrées ; §4.29 impose 1s au total en parallèle ; L-8, §5.4/5.16 et vague 2 suivent critique → questions → états → information. |
| Codex R3-M3 | Fermé : §4.30 remplace le menu par un dialogue popover ; action et punaise tabulables, flèches, Échap, retour ; six épinglées. |
| Codex R3-M4 | Fermé pour les surfaces signalées : §5.19 radio sans boutons imbriqués et liste de gestion distincte ; §4.36–37 accès aux cartes par Ctrl+Maj+Espace, Tab, Échap et retour. La nouvelle frise introduit une ambiguïté distincte (R5-M3). |
| Codex R3-M5 | Fermé : L-5 force le retour sous 560px, sans exemption fondée sur la préférence de code. |
| Codex R3-M6 | Fermé au niveau UX : TH-D2 et JSON définissent compatibilité majeure/mineure, requis et bornes typées. Le validateur est toujours futur, G-71. |
| Codex R3-M7 | Fermé pour les parcours signalés : §5.19 conserve la saisie sur erreur, repli sans vignette ; §4.36–37 versionnent les résultats, refusent les corrections périmées et permettent l'écriture sans service. Les nouveaux parcours de restauration nécessitent R5-M1. |
| Codex R3-m1/m2 | Fermés : G-81 reconnaît colonne et 4 000 caractères ; modèle en un dialogue, description dans une langue, valeurs automatiques et avancées repliées. |
| Fable M1 | Fermé : planchers HC, refus explicite ; plus de comparaison au rapport Adwaita. |
| Fable M2 | Fermé pour Ctrl+. : Alt+Entrée ; réservations GTK explicites, Ctrl+Espace avec repli Alt+/. Réserve plateforme en §5 ci-dessous. |
| Fable M3 | Fermé : trois paires de bordure `hcOnly`, recalculées ci-dessous. |
| Fable m1/m2 | Fermés : 131 paires, classes des jetons et surfaces sombres explicites ; délais verrouillés. |
| Fable m3/m4 | Fermés : protocole 4 000/colonne et CSV `problems.ts` ; incompatibilités anciennes/récentes avec détails en §4.29. |
| Fable m5/m6 | Fermés : alternatives exportées (§4.34), enregistrement de débogage visible et export expurgé (§4.35). |
| Fable m7 | Dépassé proprement par C13 : §4.36 compteur unique, liste dans Relire ; résidu de texte R5-m1. |
| Fable m8/m9/m10 | Fermés pour les demandes d'origine : modèle simplifié, partage ordinaire, une corbeille ; bandeau de projet « Voir » et interrupteurs par ligne ; cartes dans Nouveau projet ; Entrée insère, Tab indente. |

Preuves frontend ouvertes : `frontend/src/App.svelte:197` ignore toujours le fichier du clic (`onOpen={() => session.focus()}`) : G-0a/b sont correctement déclarés ouverts. `frontend/src/ui/prefs/Preferences.svelte:86` contient toujours `.sub` dans une carte de dialogue : G-85 demeure réel. Le jargon de `frontend/src/i18n/fr.json` et l'ancien tiroir `ui/history/HistoryDrawer.svelte` ne sont pas transformés par cette révision documentaire. `preview/protocol.ts` conserve colonne et troncature 4 000 ; cela ne livre ni corrections ni traces. La matrice distingue utilement S, S→G et S→N ; aucun accord sur le design ne clôt ces écarts.

## 2. Les vingt changements de cohérence

« Appliqué » signifie présent dans la conception, pas implémenté.

| # | Vérification | Position |
|---|---|---|
| 1 | §4.25 ouverts/chargés/actif, fermeture sans question | Appliqué. |
| 2 | §3.1 tiroir Rechercher/Relire/IA ; Inspecteur détail de Ressources | Appliqué. |
| 3 | §4.16 Fichiers/Sommaire, filtre Tâches, Liens dans détails, cloud-off | Appliqué ; vague 5 d'inventaire garde l'ancien libellé, R5-m1. |
| 4 | §4.17/36 compteur unique et clic suivant | Appliqué, mais phrase « liste Problèmes (popover) » contradictoire, R5-m1. |
| 5 | §4.17 quatre éléments et débordement | Appliqué ; indicateur Ressources conditionnel acceptable. |
| 6 | §8.2, principes règle 3 : lien/palette/corrections, export en attente | Appliqué ; conventions Mac à préciser, R5-M4. |
| 7 | §4.22 lecture du passé, retour, restauration nouvelle modification, comparaison | Partiel : chemin Git encore exposé ; clavier/frise R5-M3. |
| 8 | §4.22 variantes, demande, intégration et duplication | Appliqué dans le principe ; préciser transaction et focus, R5-M2. |
| 9 | §4.43 protection, défaut partagé, déverrouillage | Partiel : secours trop lourd, promesses trop absolues, R5-M1. |
| 10 | §4.44 sauvegarde datée, restauration à côté, fraîcheur | Partiel : phrase sans kit omise malgré ST-8, R5-M1. |
| 11 | §4.40 Ressources séparée, verbes, limites, unités | Appliqué ; fenêtre non modale raisonnable, sans ouverture automatique. |
| 12 | §4.42 aperçu des règles, portée, copies déjà reçues, dépendances protégées | Appliqué ; couche projet masquée avant PD-1. |
| 13 | §4.39 indicateur de séance, provenance, coût, jeton MCP/client | Appliqué ; la politique partagée reste bloquée avant PD-1, explicitement. |
| 14 | §4.41 proposition commune, exception par extension globale | Partiel : ambiguïtés d'application et raccourcis, R5-M2. |
| 15 | §5.19 bibliothèque unique, aucun gabarit, fichiers visibles, CSS export HTML | Appliqué ; nette réduction de l'assistant. |
| 16 | §4.15/5.16 états acquittés, G-94 conserve la dette du code | Appliqué ; étendre cette honnêteté au secours. |
| 17 | §4.29–33 gestionnaire, puzzle, épinglage, intégrées rétablies | Appliqué. |
| 18 | §8.4 menus et contrôles système, refus explicite des capacités non prouvées | Partiel : remappage Mac incomplet, R5-M4. |
| 19 | §4.45/5.12 aucun format bureautique natif ; collage riche prévu | Appliqué. Les noms cités comme exclusions ou références ne sont pas des fonctionnalités natives. Aucun besoin d'ajouter DOCX/ODT. |
| 20 | Glossaire enrichi et §6.4 | Appliqué, sauf exposition Git de §4.22 et résidu « brouillons » §4.38. |

## 3. Contrastes recalculés indépendamment

Calcul Python exécuté en mémoire depuis `tokens.json` : résolution des quatre modes avec repli HC sur clair/sombre ; alpha CSS en pourcentage converti en fraction ; composition sRGB du fond sur `over`, puis du texte sur ce fond ; linéarisation à 0,04045 et luminance 0,2126R + 0,7152G + 0,0722B ; rapport sans arrondi intermédiaire. **131 × 4 = 524 rapports**, aucun sous `min`/`hcMin` applicable hors `forbidden`, bordures vérifiées uniquement en HC. Cela ne prouve pas l'exhaustivité du contrat.

Références : [WCAG 2.2 1.4.3](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html), [1.4.11](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html). Texte ordinaire ≥4,5 ; indicateurs ≥3 ; plancher HC du produit 7 pour texte principal.

| Paire (parent si nécessaire) | Clair | Sombre | HC clair | HC sombre |
|---|---:|---:|---:|---:|
| fg / view | 12,5673 | 16,8155 | 12,5673 | 16,8155 |
| fg / window | 12,2232 | 15,8513 | 12,2232 | 15,8513 |
| fg / sidebar | 11,1820 | 13,5220 | 11,1820 | 13,5220 |
| fg / popover | 12,5673 | 12,0301 | 12,5673 | 12,0301 |
| fg / dialog | 12,2232 | 12,0301 | 12,2232 | 12,0301 |
| fg / headerbar | 12,5673 | 13,5220 | 12,5673 | 13,5220 |
| fg-dim / view | 4,7422 | 5,9314 | 17,4178 | 13,7771 |
| fg-dim / window | 4,6942 | 5,7511 | 16,7816 | 13,0476 |
| fg-dim / sidebar | 4,5390 | 5,2387 | 14,9223 | 11,2530 |
| fg-dim / popover | 4,7422 | 4,8654 | 17,4178 | 10,0847 |
| fg / selected (view) | 10,7928 | 12,4865 | 10,7928 | 12,4865 |
| fg / selected (sidebar) | 9,5602 | 9,8677 | 9,5602 | 9,8677 |
| fg-dim-on-card / card (dialog) | 4,7422 | 5,0701 | 17,4178 | 7,9838 |
| page-fg / page | 7,4119 | 7,4119 | 7,4119 | 7,4119 |
| border / view — HC seul | 1,4141 | 1,5990 | 3,9647 | 5,1400 |
| border / sidebar — HC seul | 1,4062 | 1,6149 | 3,8264 | 4,6088 |
| border / dialog — HC seul | 1,4122 | 1,5986 | 3,9322 | 4,3094 |
| suggestion-line / view | 5,5477 | 8,0373 | 5,5477 | 8,0373 |
| version-marker / side-bg-2 | 5,4790 | 8,6705 | 5,4790 | 8,6705 |
| variant-line / side-bg-2 | 6,1505 | 6,0993 | 6,1505 | 6,0993 |
| fg-dim / headerbar | 4,7422 | 5,2387 | 17,4178 | 11,2530 |
| ai-mark / view | 6,8450 | 8,0245 | 6,8450 | 8,0245 |

## 4. Constats nécessitant correction

### R5-B1 — Bloquant — R3-B1 n'est pas complètement fermé

**Normes :** WCAG 1.4.3, Nielsen 5 ; TH-D2. **Preuve :** §4.37, « Indices de noms de paramètres », place explicitement `--fg-dim` sur `--hover-bg`. Les nouvelles paires de survol du JSON vérifient `--fg`, pas ce texte secondaire. Un thème remplaçant seulement le survol clair par `#bbbbbb` donne **7,9065:1** pour `--fg`, donc passe les paires de survol déclarées, mais **3,9203:1** pour les indices en `--fg-dim`, sous 4,5. Même si la valeur claire sert de repli HC, le texte principal passe 7 et le secondaire HC atteint 9,6675 : le défaut clair reste invisible au contrat. Le contre-exemple noir précédent est corrigé, la classe de défaut ne l'est pas.

**Correction :** ajouter cette paire pour les parents autorisés, vérifier les textes secondaires et le focus adjacent aux états interactifs ; verrouiller les combinaisons non vérifiables. Ajouter le thème `#bbbbbb` aux cas de refus. Aucun besoin de refaire les 524 rapports déjà corrects.

### R5-M1 — Majeur — le secours est plus difficile et plus restrictif que l'architecture

**Normes :** Nielsen 2, 5, 8, 9 ; GNOME HIG [Reduce User Effort](https://developer.gnome.org/hig/principles.html) ; N37 « extrêmement simple ». **Preuves :** §4.43 impose recopier 24 mots sans Copier, vérifier trois mots, puis enregistrer/imprimer un second objet ; §4.44 propose phrase **et** kit uniquement. Architecture ST-8 autorise explicitement **phrase seule** avec découverte sur le dorsal et avertissement de fraîcheur. Le texte « seuls phrase et kit… sans eux, documents perdus » devient faux lorsqu'un appareil, un pair ou une sauvegarde exploitable existe. « Personne ne peut les lire » ignore la copie lisible durable décrite plus bas. Le bandeau « phrase pas encore notée » ne distingue pas le kit manquant ; le comportement de « Plus tard » face à « Terminer » désactivé est ambigu.

**Correction :** un parcours « Mettre mon secours en lieu sûr » avec enregistrement/impression guidé, et un état précis de ce qui manque ; permettre la phrase seule comme ST-8, puis découverte et avertissement. Dire quelles copies sont protégées et dans quel scénario la perte est définitive. Conserver une assistance accessible à la transcription, notamment lors de la restauration. [WCAG 3.3.8](https://www.w3.org/WAI/WCAG22/Understanding/accessible-authentication-minimum.html) distingue création initiale et authentification : la vérification d'activation n'est pas à elle seule une violation établie, mais « visible à l'étape précédente » ne suffit pas à garantir une récupération accessible. Ne pas exiger une mise à jour manuelle du kit à chaque sauvegarde : destination persistante choisie, mise à jour automatique, avertissement seulement si elle échoue. C'est le principal foyer de sur-ingénierie.

### R5-M2 — Majeur — la proposition commune ne définit pas une interaction sûre dans le texte

**Normes :** Nielsen 3/4/5 ; WCAG 2.1.1, 2.4.3, 4.1.2 ; [2.1.4](https://www.w3.org/WAI/WCAG22/Understanding/character-key-shortcuts.html) pour les chiffres. **Preuves :** §4.41 place le diff dans le texte et réserve ↑/↓, Entrée et Suppr sans définir l'entrée en mode de relecture, la cible focalisée ni le retour à l'édition. §4.22 autorise 1/2/3 « seulement dans cette vue », mais la même vue contient un champ « Écrire une autre version » : une restriction par vue n'exclut pas ce champ. §4.41 dit « Accepter = une modification », alors que §4.22 promet une intégration en **une** modification après « Terminer ». On ne sait pas si accepter un morceau modifie déjà l'original, ni ce que fermer/annuler laisse derrière.

**Correction :** focaliser explicitement un morceau nommé ; raccourcis actifs seulement sur ce composant, jamais dans un champ ; Tab vers les actions, Échap et retour document définis. Choisir et écrire une règle de transaction : choix locaux puis application finale atomique pour l'intégration, ou application immédiate avec annulation exacte clairement annoncée. Définir Tout accepter en présence de divergences et préserver les choix si une modification concurrente périme le résultat. Garder un seul composant de proposition.

### R5-M3 — Majeur — historique au clavier incomplet et exception Git contraire à N39

**Normes :** Nielsen 2/4/6 ; WCAG 2.1.1/4.1.2 ; [APG Listbox](https://www.w3.org/WAI/ARIA/apg/patterns/listbox/). **Preuves :** §4.22 nomme une listbox à sélection unique mais compare deux entrées seulement par Maj+clic ; « Comparer », « Nommer », « Créer une variante » sont « sur une entrée » sans emplacement clavier défini. Une option de listbox ne peut pas héberger ces boutons sans changer de modèle. ↑/↓ déplacent les entrées sans précision focus/`aria-activedescendant`, ce qui ne clôt pas le risque connu G-51 pour la nouvelle frise. Le slider manque de valeur temporelle accessible explicite. Enfin le point 8 affiche « Exporter l'historique en dépôt Git », même sous Avancé, malgré « zéro jargon Git dans l'interface » N39 ; HI-5 contient lui-même cette contradiction.

**Correction :** garder des options simples avec actions pour la sélection dans une barre externe ; définir sélection multiple au clavier ou deux sélecteurs de comparaison, focus et `aria-valuetext` date/auteur. Renommer/déplacer l'export technique derrière une extension ou une surface développeur expressément hors interface d'écriture ; signaler l'alignement nécessaire de HI-5. Montrer les métadonnées utiles plutôt qu'une description automatique trop affirmative du sens des changements.

### R5-M4 — Majeur — la substitution Ctrl → Cmd crée des collisions macOS

**Normes :** Nielsen 4/5 ; Apple HIG [Keyboards](https://developer.apple.com/design/human-interface-guidelines/keyboards), conventions système. **Preuves :** §8.2 dit globalement « Ctrl vaut Cmd sur macOS », tandis que §4.38 prescrit Ctrl+H pour remplacer et §8.4 réserve Cmd+H à Masquer. §8.4 dit que seuls les raccourcis `Mod` changent, mais le registre lisible utilise Ctrl et ne classe pas chaque commande. Une génération littérale produit Cmd+H pour deux actions, ou laisse Remplacer sans accès annoncé. La fiche d'indisponibilité de l'aperçu est honnête ; elle ne remplace pas cet audit des gestes.

**Correction :** matrice concrète des exceptions système : Remplacer sur une combinaison libre et conventionnelle sur Mac, Masquer réservé ; `Mod` et `Ctrl` physique distingués par entrée, conflits testés par OS. Vérifier aussi les raccourcis Ctrl+Alt sur claviers Windows avec AltGr, sans supposer qu'un audit Linux prouve la parité. Aucun besoin de refaire l'apparence Adwaita.

### R5-m1 — Mineur — résidus de la rév. 4 dans la documentation normative

**Normes :** Nielsen 4/8. **Preuves :** §4.36 conserve « liste Problèmes (popover)… le popover se ferme » malgré C13 ; §4.38 termine la comparaison par « Entre deux brouillons : en v2 » malgré N41 ; §4.37 renvoie à l'assistant de modèle « étape 2 » supprimé ; vague 5 de `inventory.md` annonce encore Sommaire/Tâches/Liens comme sélecteur. §4.40 renvoie une action Voir vers « En cours » au lieu de Ressources.

**Correction :** retirer ces anciennes règles, garder les noms figés et aligner le plan de réalisation. Ces résidus ne justifient aucun widget supplémentaire.

## 5. Portée nouvelle et désaccords ouverts

N30 : orthographe discrète, prose seule, relecture volontaire, pas de score ni de série : bons choix pour les personnes qui écrivent. Réglages avancés et symboles restent sur geste. N31 : fournisseur absent par défaut, fantôme local, envoi explicite par provenance, politique partagée bloquée avant PD-1, coûts honnêtes et arrêt sont solides ; R5-M2 doit s'appliquer aussi à l'IA, MCP et agents. Le texte fantôme annoncé mais `aria-hidden` devra être lisible à la demande avant acceptation lors de la réalisation, sans annoncer chaque token.

N32 : Ressources est utile et hors du trajet normal ; sept colonnes et huit verbes sont denses, mais acceptables dans une vue spécialisée si les actions inapplicables sont absentes et les détails techniques repliés. Pas de second tableau de bord « En cours ». N34 : la distinction appareil/projet, l'essai avant application et l'avertissement sur les copies déjà reçues sont clairs ; le nuage barré ne doit pas promettre une confidentialité rétroactive. N37 : le chiffrement ne doit pas devenir une cérémonie technique ; R5-M1 est prioritaire. N39/N41 : vocabulaire d'écriture bien choisi, mais R5-M2/M3 à fermer. N35 : contrôles système et refus des capacités non prouvées bien distingués du dessin ; R5-M4 reste nécessaire. N40 : aucune intégration bureautique native demandée ou recommandée ici.

Positions sur **CHANGES.md**, sans le modifier (consigne de cette revue) :

- Liste des problèmes : désaccord clos ; j'accepte la décision globale C13, malgré mon avis favorable au popover au round 3. Corriger seulement le résidu R5-m1.
- Barre de mise en forme : conserver le choix explicite T11 de la personne, fixe et repliable ; mémoriser ce choix. La phrase générique de cohérence ne justifie pas de l'annuler. Pas de bloqueur.
- Ressources en fenêtre auxiliaire : favorable, non modale, ouverte sur geste ; retour au document et fermeture native à garantir. C18 le permet ; aucune multiplication de fenêtres spontanées.
- Ctrl+; : favorable à Ctrl+Alt+C sur Linux après audit ; le raccourci GTK ne doit pas être repris. Valider par OS.
- Ctrl+Espace : acceptable avec alternative Alt+/ toujours accessible et affichée ; tester avec IBus actif, pas seulement la configuration GNOME par défaut. Ne pas présenter une commande inaccessible à certaines personnes comme universelle.

Le calme général est acquis dans l'organisation proposée, pas encore dans tous les parcours. Les simplifications demandées portent surtout sur la récupération et sur des contrats d'interaction explicites ; elles ne nécessitent ni nouvelle destination ni nouvel assistant.

**NOT ACCEPTED** — bloqueurs avant accord : R5-B1 (contrat de contraste incomplet), R5-M1 (secours simple et récupération sans kit), R5-M2 (focus et transaction des propositions), R5-M3 (historique clavier et zéro jargon Git), R5-M4 (raccourcis macOS sans collision). G-0 et les autres écarts frontend restent des blocages de livraison distincts.
