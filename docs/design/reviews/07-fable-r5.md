# Revue Fable n°5 du système de design (révision 5, 2026-11-01) — verdict : **ACCEPT**

Revue indépendante de `spec.md` rév. 5, `coverage.md`, `inventory.md`, `tokens.json`,
`CHANGES.md`, contre les revues n°3 (`05-fable-r3.md`, `06-codex-r3.md`), `architecture/coherence.md`
§6 (les 20 changements), `architecture/spec.md` rév. 3 (§6.6, §6.9, §6.10, §6.11, §9.11, §10.4,
§11.1, §15.17, §15.18), `docs/exigences.md` N17–N41, `ux-principles.md`, `glossary.md`, et le
code de `frontend/src` (`main`, `a886ad3`). La revue Codex n°5 n'a pas été lue.

**Méthode.**
- Chaque constat des deux revues n°3 est rouvert dans le texte de la rév. 5, puis dans le code (§1).
- Les 20 changements de `coherence.md` §6 sont confrontés un à un à la section qui les porte (§2).
- Les **131 paires** de `tokens.json` sont recalculées par script dans les quatre modes
  (**524 valeurs** ; WCAG 2.2, luminance relative, translucides composés en sRGB sur `over`,
  `hcMin` et `hcOnly` appliqués comme `themeContract.hcRule` le déclare) ; ΔE2000 des huit
  couleurs de personnes **et** des nouveaux repères (frise, variante, IA, extension) (§3).
- Les raccourcis nouveaux sont vérifiés contre la HIG *Keyboard*, les liaisons GTK 4, et, puisque
  N35 entre dans le périmètre, contre les réservations de **macOS** (Apple, « Raccourcis clavier
  du Mac », support 102650) et de **Windows** (Microsoft, *Windows UX Guide › Keyboard*, règle 8).
- Le nouveau périmètre (N39, N41, N30, N31, N32, N34, N37, N35, N40) est lu comme une personne qui
  écrit et n'a jamais ouvert Git (§5).

## 1. Fermeture des constats de la revue n°3

### Fable n°3

| # | Fermé ? | Preuve |
|---|---|---|
| M1 (règle HC) | **oui** | `spec.md:385-391` : planchers, plus de comparaison à Adwaita ; `tokens.json#/$extensions/inkwell/themeContract/hcRule` ; `hcMin: 7` sur les paires `--fg`/`--fg-dim`/`--fg-dim-on-card` (recalcul : 15,56 pour le « papier » est bien ≥ 7) ; refus avec rapport et seuil `spec.md:403-405`. |
| M2 (Ctrl+.) | **oui** | Alt+Entrée `spec.md:1799-1804`, `:3150` ; liste réservée `spec.md:3186-3198` (Ctrl+., Ctrl+;, Ctrl+Maj+U, Ctrl+Maj+I, GNOME Shell, Ctrl+Espace « à risque », repli Alt+/) ; K-6 `spec.md:3182-3184`. Reste incomplète pour macOS et Windows : **M1** ci-dessous. |
| M3 (`--border` HC) | **oui** | paires 75–77, `hcOnly: true`, `min: 3` ; recalcul : 3,96 / 5,14 · 3,83 / 4,61 · 3,93 / 4,31 ; exemption 1.4.11 en clair et en sombre dans §2.2 ; `coverage.md:43`. |
| m1 (doublon, compte) | oui | le doublon 36 = 6 a disparu ; 131 paires ; les cinq couples encore répétés (36/16, 39/17, 40/18, 41/19, 49/46) ont un **rôle et un seuil différents** (texte 4,5 contre anneau de focus 3,0) : ce ne sont plus des doublons. |
| m2 (contrats) | oui | script : **0 jeton sans `contract`** ; `--tooltip-fg` et `--on-dark-fg` verrouillés, « toasts, infobulles et scène restent sombres » `spec.md:362-364` ; délais d'accessibilité verrouillés `spec.md:359`. |
| m3 (4 000, `problems.ts`) | oui | `spec.md:1908-1911` ; `:1827` (`findProblems`, ligne seulement). |
| m4 (paquet incompatible) | oui | `spec.md:1423-1424` : deux lignes PV-1 et PV-4, « Détails techniques ». |
| m5 (texte alternatif hors éditeur) | oui | `spec.md:1666-1690` : description partagée, `image(alt:)`, aperçu, PDF, Vue lecture ; absence = avertissement ; `coverage.md` 1.1.1. |
| m6 (débogage) | oui | `spec.md:1721-1729` : « Enregistrer pour rejouer », pastille « Débogage de X », trace expurgée cochée. |
| m7 (bouton partagé) | oui, **et sans objet** | C13 : un seul bouton `spec.md:1785` (« Problème suivant, 2 erreurs, 1 avertissement »), liste dans Relire. Q17 (`spec.md:3382`) décrit encore le bouton partagé : résidu, **M4**. |
| m8 (§5.19) | oui | `spec.md:2937-2939` (partage = comme un projet, ou export), `:2997` (une seule corbeille), `:3002-3003` (v2, PEUT). |
| m9 (« Désactiver pour moi ») | oui | `spec.md:1594-1595` ; cartes « Démarrer avec » : `spec.md:2941` (galerie dans « Nouveau projet »). |
| m10 (Tab, casse) | oui | `spec.md:2157` (Entrée insère) ; `:2969` (« Diapositives », majuscule de phrase). |
| §5 (crochets N30, N31) | oui | gravité « suggestion » `spec.md:1755` ; Relecture dans Relire › Problèmes `spec.md:2081-2082` ; règle d'exclusion et politique `spec.md:2282-2284` ; MCP en pastille L-8d `spec.md:2348-2349` ; « via IA » `spec.md:2232`. |

### Codex n°3

| # | Fermé ? | Preuve |
|---|---|---|
| R3-B1 | **oui** | paires d'états : `--fg`/`--fg-dim` sur `--btn-bg`, `--hover-bg`, `--selected-bg`, `--card-bg`, `--page-bg` (paires 14–15, 70–74 et suivantes) ; `--fg-dim-on-card` (5,07 en sombre, G-85) et `--page-fg` (7,41) ; contre-exemple « `--hover-bg` noir refusé » dans §10 (`spec.md:3325`). Le recalcul des 524 valeurs ne montre **aucun** échec hors paires interdites. |
| R3-M1 | oui | comme m5. |
| R3-M2 | oui | mode sans extensions = intégrées telles que livrées `spec.md:1640` ; arrêt 1s au total `spec.md:1490` ; rangs L-8 `spec.md:544-557`, §5.4 « rang questions » `:2696`, §5.16 « critique » `:2868-2871`. |
| R3-M3 | oui | popover, deux boutons par ligne, clavier, six épinglées `spec.md:1499-1520`. |
| R3-M4 | oui | galerie = groupe radio de cartes sans contrôle interne `spec.md:2942-2944` ; cartes riches : focus qui entre par Ctrl+Maj+Espace, Tab, Échap `spec.md:1833-1839`. |
| R3-M5 | oui | `spec.md:525` : le code revient à la ligne lui aussi sous 560px. |
| R3-M6 | oui | `contract: "^1.0"`, `required`, bornes typées `spec.md:366-382`, `tokens.json` `compatibility`, `required`, `bounds`. |
| R3-M7 | oui | table d'états §5.19 `spec.md:2979-2986` ; résultats périmés `spec.md:1841-1845` ; service indisponible `:1851-1855`. |
| R3-m1, R3-m2 | oui | comme m3 et m8 ; dialogue unique `spec.md:2959-2977`. |
| Notes §4 | oui | « Donner à X » déplace `spec.md:1630-1632` ; §5.12 « Aller au premier problème » `:2787` ; sans position `:1847-1849` ; lignes de test N27–N32 et N34–N41 dans §10. |

**Tous les constats des deux revues n°3 sont fermés.** G-0 reste ouvert, et dit ouvert (`App.svelte:197`,
`spec.md:972`, `inventory.md:192`).

## 2. Les 20 changements de la passe de cohérence

| # | Changement | Appliqué ? | Preuve, et ce qui manque |
|---|---|---|---|
| 1 | onglets : ouvert ≠ chargé ≠ actif ; fermeture sans question | oui | `spec.md:1209-1213`, `:1220` (SAV-03) |
| 2 | tiroir à trois destinations ; Inspecteur et « En cours » dehors | oui, avec une porte ouverte | `spec.md:448-449` ; mais « ou une quatrième destination « Extensions » » (`:448`) rouvre ce que C2 ferme (**m8**) ; `:2431` parle encore de « la vue En cours » (**M4**) |
| 3 | gauche à deux onglets ; Liens dans les détails ; Tâches en filtre ; nuage barré | oui | `spec.md:928`, `:941-943`, `:957`, `:978-979` |
| 4 | un compteur, barre d'état ; clic = suivant ; liste dans Relire | oui, avec résidus | `spec.md:987-990`, `:1785`, `:1857-1859` ; résidus : `:1786` (« liste Problèmes (popover) »), Q17 et Q18 (`:3375`, `:3382`), `coverage.md:176` (« popover existant », « F8 et Ctrl+. ») (**M4**) |
| 5 | barre d'état : quatre éléments | oui | `spec.md:986-1001` |
| 6 | raccourcis : Ctrl+K, Ctrl+Maj+P, Alt+Entrée ; règle 3 ; export après audit ; plus de Ctrl+. | oui, sauf un | `ux-principles.md:13-14` corrigée ; `spec.md:3169` export sans raccourci ; inventaire G-49, G-82 sans Ctrl+. ; **mais `spec.md:2777` donne encore « Ctrl+Maj+E » à Exporter**, pris par la barre (`:3169`, `:3183`) (**M4**) |
| 7 | historique sans Git | oui | §4.22 ; aucun mot de Git hors « Exporter l'historique en dépôt Git » sous Avancé (`:1157`), à déclarer comme la seule exception (**m6**) |
| 8 | variantes | oui, à clarifier | §4.22 points 6 et 7 ; la **portée** d'une variante est ambiguë (**M2**) |
| 9 | protection | oui, à clarifier | §4.43 ; la phrase est **par personne** (ST-8), le parcours la fait vivre **par projet** (**M3**) |
| 10 | sauvegarde et restauration | oui | §4.44 : dernier succès, à côté, fraîcheur ST-7 |
| 11 | vue Ressources | oui | §4.40 : verbes RS-3, limites RS-4, RS-5, RS-6 ; un nom périmé (`:2431`) |
| 12 | non synchronisés | oui | §4.42 : phrase « ne retire pas les copies déjà reçues » (`:2487`), « Pour tout le projet » (`:2490-2492`), aperçu (`:2498`) |
| 13 | IA | oui | §4.39 : pastille pendant une séance (`:2208-2211`), nouvelle provenance (`:2299-2302`), jetons et estimation (`:2322-2324`), marques (`:2239-2244`), jeton par application (`:2340-2342`), « se présente comme » (`:2343-2345`) ; conforme à l'architecture `:2764-2773` |
| 14 | propositions : une présentation | oui | §4.41 ; `spec.md:1405-1406` |
| 15 | modèles | oui, un résidu | §5.19 ; mais §5.9 (`:2755`) nomme encore « Rapport » parmi les mises en page livrées d'un `.md` (**m9**) |
| 16 | états honnêtes | oui | §4.15 table ; §5.16 `:2863`, `:2869` ; G-94 (`fr.json:7`) |
| 17 | extensions : un gestionnaire | oui | `spec.md:1410-1413` ; §4.30, §4.33 |
| 18 | plates-formes | oui, incomplet | §8.4 ; la liste réservée macOS et Windows est trop courte, et Ctrl+Alt est interdit sous Windows (**M1**) |
| 19 | aucun format bureautique ; collage riche | oui | §4.45 ; §5.12 `:2790-2793` ; `fr.json` et `en.json` : aucune chaîne « Word », « DOCX », « ODT » (vérifié par grep) ; G-97 |
| 20 | glossaire | oui | `glossary.md` : version, variante, proposer d'intégrer, protéger, phrase et kit, sauvegarde, ressources, non synchronisé ; « jalon », « brouillon », « fusionner » dans « ne jamais dire » ; §6.4 complété, **avec une ligne contradictoire** (`spec.md:3056`, **M4**) |

## 3. Recalcul des contrastes

Script : `scratchpad/contrast.py` (lecture de `tokens.json`, résolution `$extensions.inkwell.modes`,
composition alpha sur `over`, WCAG 2.2, ΔE2000 en Lab D65).

- **524 valeurs sur 524 identiques à ±0,02** aux valeurs déclarées ; **0 paire non interdite sous
  son seuil** (`min`, ou `hcMin` en HC ; `hcOnly` ignoré hors HC) ; **0 jeton sans contrat**.
- **Frise et variantes** (paires 79–82, nouvelles) : `--version-marker`/`--side-bg-2` 5,48 · 8,67 ;
  `--variant-line`/`--side-bg-2` 6,15 · 6,10 ; `--ai-mark`/`--view-bg` 6,85 · 8,02 ;
  `--suggestion-line`/`--view-bg` 5,55 · 8,04. Hors paires déclarées, j'ai aussi posé ces quatre
  repères sur `--view-bg`, `--sidebar-bg`, `--selected-bg`, `--hover-bg`, `--window-bg` et
  `--dialog-bg` : minimum **4,43** (`--suggestion-line` sur sélection claire), tous ≥ 3 (1.4.11).
- Échantillon de contrôle : `--fg-dim`/`--view-bg` 4,74 · 5,93 · 17,42 · 13,78 ;
  `--fg-dim-on-card`/`--card-bg` sur dialogue 4,74 · **5,07** · 17,42 · 7,98 ; `--page-fg`/`--page-bg`
  7,41 ; `--border` HC 3,96 / 5,14 ; `--mark-extension`/`--sidebar-bg` 7,52 · 5,62.
- **Couleurs de personnes** : ΔE2000 minimal **21,3** (TH-4 ≥ 15 tenu).
- **Repères sémantiques, entre eux** : `--variant-line` `#813d9c`, `--ai-mark` `#6a4c93` et
  `--mark-extension` `#613583` sont à **ΔE 6,7 à 7,1** les uns des autres ; `--version-marker`
  (= `--accent`) est à **ΔE 7,6** de `--person-5` (bleu d'Okabe-Ito). Voir **m4**.

## 4. Constats

### Bloquants

Aucun. Aucun échec WCAG 2.2 AA n'est introduit ; aucun parcours principal n'est cassé par la rév. 5
(G-0 était déjà ouvert et le reste, exactement décrit).

### Majeurs

**M1 — La liste des raccourcis réservés ignore macOS et Windows ; sept raccourcis en Ctrl+Alt
sont interdits sous Windows, et deux sont pris par macOS.**
- *Norme.* N35 ; K-6 (`spec.md:3182-3184` : « tout raccourci de la liste réservée par la
  plate-forme ») ; ND-3 (aucun raccourci pris en silence) ; Microsoft, *Windows UX Guide ›
  Keyboard › Guidelines*, règle 8 : « **Don't use Ctrl+Alt combinations**, because Windows
  interprets this combination in some language versions as an AltGR key, which generates
  alphanumeric characters » (learn.microsoft.com/windows/win32/uxguide/inter-keyboard) ; Apple,
  *Raccourcis clavier du Mac* (support.apple.com/102650) : **Option-Command-H** = « Hide Others »,
  **Control–Space** = « Select the previous input source », Option-Command-Esc, Shift-Command-3/4/5,
  Control-Command-Q.
- *Preuve.* Le registre (`spec.md:3136-3180`) emploie **Ctrl+Alt+H** (Versions), **Ctrl+Alt+S**
  (Enregistrer une version), **Ctrl+Alt+I** (IA), **Ctrl+Alt+F** (concentration), **Ctrl+Alt+C**
  (caractère spécial), **Ctrl+Alt+J** (source → aperçu), **Ctrl+Alt+0** (paragraphe). §8.4
  (`:3236`) fait de `Mod` la touche Cmd : Ctrl+Alt+H devient **⌥⌘H, « Masquer les autres »** ;
  la table des réservés macOS (`:3244`) ne le liste pas, ni ⌃Espace, ni ⌥⌘Esc, ni ⇧⌘3/4/5.
  Sous Windows, avec un clavier polonais, tchèque, hongrois ou slovaque, **AltGr = Ctrl+Alt** :
  taper « ć » (AltGr+C), « ś » (AltGr+S) ou « í » (AltGr+I) ouvrirait le sélecteur de caractères,
  Enregistrer une version ou l'IA, dans un **éditeur de texte pour des personnes qui écrivent** —
  le pire endroit. **Ctrl+Espace** (`:3155`) est, sur macOS, le changement de source de saisie,
  activé dès qu'il y a deux langues de clavier : précisément le cas d'une personne qui écrit en
  français et en anglais (Q7).
- *Correction* (une règle, pas sept renommages) :
  1. compléter la table de §8.4 : macOS ⌥⌘H, ⌥⌘Esc, ⌃Espace, ⌃⌥Espace, ⇧⌘3/4/5, ⌃⌘Q, ⇧⌘Q ;
     Windows : **toute combinaison Ctrl+Alt**, Ctrl+Alt+Suppr, Ctrl+Maj+Échap, Alt+Espace ;
  2. une règle de registre K-7 : **`Mod+Alt+X` n'existe pas sous Windows** ; le registre l'y
     rend par `Ctrl+Maj+Alt+X` (motif que Microsoft recommande à ses propres compléments) et
     l'affiche ainsi ; sous macOS, `Mod+Alt+X` devient ⌥⌘X **sauf** s'il figure dans la table :
     alors le registre exige une liaison explicite ;
  3. **Versions** : pas ⌥⌘H ; proposer Ctrl+Maj+H (Relire) puis la sous-vue Versions, ou une
     liaison macOS propre ;
  4. **Suggestions** sous macOS : **⌥Échap** (la commande *Complete* du système de texte d'Apple,
     « Option-Esc » dans TextEdit et Xcode), pas ⌃Espace ; ajouter la ligne à §8.4 ;
  5. le test K-6 (`spec.md:3326`) tourne avec la table de **chaque** système, pas seulement celle
     de GNOME.

**M2 — La portée d'une variante n'est pas claire pour une personne qui ne connaît pas Git.**
- *Norme.* N41 (« très claires pour la personne ») ; Nielsen 2 et 6 ; `coherence.md` §6.8.
- *Preuve.* `spec.md:1133` : « une copie **des documents choisis**, qui garde l'histoire commune ;
  on y travaille dans les mêmes onglets » ; `:1134` : « Changer de variante … les onglets
  suivent ». L'architecture HI-6 définit la variante **par document**, et le code aussi
  (`doc.meta.variants`, `history/graph.ts:10`, une voie par document). Si une variante « Ton plus
  court » ne couvre que `ch2.typ`, que montrent le sélecteur du sous-titre, l'aperçu (qui compile
  `main.typ`) et `ch1.typ` quand on « change de variante » ? Rien ne le dit. Une personne qui
  s'attend à « une autre version de mon texte » obtient un mélange d'originaux et de copies, sans
  pouvoir le voir : c'est exactement la confusion « branche » que N41 veut éviter.
- *Correction.* Dans l'interface, **une variante est toujours celle du projet** (tous les documents
  à ce moment, index compris ; un document non modifié y reste identique à l'original). Le choix
  de documents, s'il existe, va dans « Options avancées » et le sous-titre le dit (« variante Ton
  plus court · 2 fichiers »). Dans Fichiers, les fichiers que la variante modifie portent un repère
  `--variant-line` (avec infobulle), les autres aucun. Une phrase dans §4.22 point 6 et une ligne
  de test dans §10.

**M3 — La protection d'un projet fait vivre la phrase de secours par projet, et ne dit pas ce qui
se passe quand on rejoint un projet partagé.**
- *Norme.* N37 (« extrêmement simple pour la personne ») ; architecture ST-8 (`spec.md:1069-1072`
  de l'architecture : « **Un seul secret pour tous les projets de la personne** ») ; Nielsen 6, 8 ;
  `coherence.md` §6.9 (« cochée par défaut pour un projet partagé »).
- *Preuve.* `spec.md:2541-2555` : « à l'activation, avant de terminer », l'écran, les 24 mots, la
  vérification, le kit, « Terminer n'est actif qu'après… ». Rien ne distingue la **première**
  protection (créer la phrase) des suivantes (elle existe déjà). Avec la case cochée par défaut
  pour tout projet partagé, une personne qui rejoint trois projets d'équipe subirait trois fois la
  cérémonie, ou, pire, croirait avoir trois phrases. Et §5.4 (Rejoindre, `:2679-2697`) ne mentionne
  pas la case : où le « défaut pour un projet partagé » s'applique-t-il à qui **rejoint** ? Le projet
  n'est partagé qu'après §5.3, pas « à la création » (`:2528`).
- *Correction.* (1) **Une phrase par personne, créée une fois** : la cérémonie de `:2541-2555` n'a
  lieu qu'à la première protection ; ensuite la case dit « Protégé par votre phrase de secours
  (notée le 3 mars) », et seul le kit est mis à jour. (2) Le défaut s'applique **au moment où le
  projet devient partagé** : dans « Écrire à plusieurs » (§5.3, avant « Créer une invitation ») et
  dans Rejoindre (§5.4, étape 4, une ligne « Ce projet sera protégé sur cet ordinateur · Modifier »),
  jamais par un dialogue de plus. (3) Un seul écran de phrase dans toute l'application : §4.43
  devient la référence, §5.3, §5.4 et §4.44 y renvoient.

**M4 — Résidus de la passe de cohérence qui contredisent la rév. 5 elle-même, et feraient échouer
ses propres tests.**
- *Norme.* K-6 (`commands.test.ts`, `spec.md:3321`) ; §6.4 et le test « mots interdits absents »
  (`spec.md:3320`) ; Nielsen 4 ; C13, C14.
- *Preuve.*
  - `spec.md:2777` : « Exporter… (menu, palette, **Ctrl+Maj+E**) » contre `:3169` (« aucun pour
    l'instant ; Ctrl+Maj+E est pris par la barre ») : deux actions sur un raccourci, ce que K-6
    refuse ;
  - `spec.md:3056` : « retour en arrière → **Revenir à ce moment**, jamais *restaurer* » contre
    `:3077` et §4.22 (« **Restaurer cette version** ») : le vocabulaire figé s'interdit lui-même ;
    `:3059-3060` donnent deux sens à « Intégrer » (texte du disque, variante) sans le dire ;
  - `spec.md:1786` (« liste Problèmes (popover) »), `:3375` (Q18 : « le popover »), `:3382` (Q17 :
    bouton partagé), `:3373` (Q15 : « Inspecteur dans le tiroir »), `coverage.md:176` (« popover
    existant », « F8 et Ctrl+. ») : quatre textes de la rév. 4 restés en place après C13 ;
  - `spec.md:2431` (« ouvre la vue En cours »), `:2166` (« entre deux **brouillons** », mot retiré
    du glossaire), `:1709` (« Préférences › **Avancé** › Mode développeur » contre §5.13, page
    Application).
- *Correction.* Un passage de relecture mécanique (grep des mots « popover », « Ctrl+. »,
  « En cours », « brouillon », « Ctrl+Maj+E », « Avancé ») ; une seule ligne §6.4 pour « restaurer » ;
  deux entrées §6.4 pour « Intégrer » (« Intégrer (fichier modifié dehors) », « Intégrer la variante »).

### Mineurs

**m1 — §4.22, point 2 : chaque flèche dans la frise recharge tout le projet.** « Sélectionner une
entrée met toute la zone d'édition en vue du passé » (`spec.md:1088`), avec ↑ ↓ dans une listbox
dont la sélection suit le focus (`:1071`). Sur un projet de 200 fichiers, descendre de dix entrées
reconstruit dix fois l'index, les documents et l'aperçu. PERF-1, §5.17 ; Nielsen 1. Correction :
le focus se déplace, la vue du passé suit après **300ms de calme** (la règle de l'indicateur
d'attente), ou sur Entrée ; le lecteur d'écran n'annonce que l'arrêt.

**m2 — §4.22, point 3 : trois confirmations pour restaurer tout le projet.** Aperçu, puis
« Restaurer », puis dialogue de confirmation (`:1111`), puis toast « Annuler » (`:1109`), pour une
action qui crée une modification ordinaire annulable. Règle 6 (`ux-principles.md`), P3, §9 (« Êtes-
vous sûr ? » pour une action annulable). L'aperçu **est** la confirmation : retirer le dialogue.

**m3 — §4.43 et §4.44 : un toast après chaque sauvegarde, et une saisie de phrase sans aide.**
(a) « Nouveau kit de secours disponible · Enregistrer » à **chaque sauvegarde réussie**
(`:2556-2557`) : avec une sauvegarde automatique, c'est un toast par séance, pour un fichier qui
n'est pas secret. Calme (P1), TO-3. Réécrire le kit **au même endroit, en silence** ; ne prévenir
que si l'endroit a disparu, et dater le kit imprimé. (b) À la restauration (`:2596-2597`), « on
donne la phrase » : 24 mots recopiés à la main sans coller ni compléter est un test cognitif
(WCAG **3.3.8**, dont l'exception « récupération » n'exempte pas la saisie). Prévoir 24 champs
avec complétion sur la liste BIP-39 française (4 lettres suffisent) et le collage.

**m4 — Quatre repères sémantiques se ressemblent, dans la même vue.** ΔE2000 : variante / IA /
extension 6,7–7,1 ; version nommée / personne 5 7,6 (§3). Dans la frise, la voie d'une variante
(violet), l'entrée « Camille, via IA » (violet) et une proposition « via Diagramme » (violet) se
côtoient, et chaque entrée porte l'avatar d'une personne dont une est du même bleu que le repère de
version. 1.4.1 est tenu (le nom est toujours là), mais la grille du design lui-même (TH-4 : ≥ 15
entre couleurs porteuses de sens) ne l'est pas. Nielsen 8. Correction : voie de variante en
`--fg-dim` avec son nom (c'est ce que fait Patchwork) ; garder le violet à l'IA ; `--version-marker`
en forme (losange plein) plus qu'en couleur.

**m5 — §4.22, point 6 : un sélecteur dans le sous-titre de la barre d'en-tête.** H-1 fait du titre
un `<h1>` et §4.33 une **zone protégée** ; §4.1 n'anatomise aucun contrôle au centre. HIG *Header
bars* ; H-2. Soit l'ajouter à §4.1 (un *Menu Button* « Original ⌄ » sous le nom du fichier, nommé
« Variante »), soit le mettre dans la barre d'onglets. Et le compter dans les six.

**m6 — Petites règles manquantes autour des versions.** (a) L-8 (`:548-553`) liste les questions
par ordre ; « Paul propose d'intégrer » (`:1136`) n'y est pas ; l'ajouter. (b) Les touches 1, 2, 3
de la relecture d'intégration (`:1154`) doivent être **inactives quand le focus est dans le champ
« Écrire une autre version »** (2.1.4 : actives seulement sur le composant focalisé). (c) « Exporter
l'historique en dépôt Git » (`:1157`, `:2795`) est la **seule** apparition autorisée du mot : l'écrire
dans §6.4 et exempter cette chaîne du test de `:3333` (« aucun mot de Git dans le DOM »).

**m7 — §5.13 : neuf lignes pour six pages, et deux portées douteuses.** La table (`:2806-2816`)
mêle groupes et pages (« Apparence et moi » en deux lignes, « Enregistrement » et « Éditeur » à
part) alors que `:2818-2820` et `coherence.md` §4 fixent six pages. La **sauvegarde** (destination
sur un disque externe) est rangée sous « Projet · ce projet, signé » (`:2814`) : une destination est
« cet appareil ». Le **mode d'enregistrement** (Automatique / Ctrl+S) est « ce projet » (`:2809`) :
signé et partagé, ou local ? Dire lequel. Nielsen 4.

**m8 — §3.1 : « ou une quatrième destination « Extensions » » (`:448`).** C2 dit trois. Soit
l'admettre comme règle (« au plus une quatrième, nommée Extensions, seulement si une extension
active fournit un panneau »), soit la retirer. Aujourd'hui c'est une porte entrouverte.

**m9 — §5.9 : « Rapport » reste une mise en page livrée.** `:2755` (« Rapport », « Lecture ») contre
`coherence.md` §6.15 (« pas de Rapport/Lettre proposés comme gabarits ») et Q16 tranchée (`:3348`).
Garder « Adwaita » et « Lecture » (styles neutres), retirer « Rapport », ou l'écrire comme écart
assumé dans §11.

**m10 — §8.3 : l'icône `branch` pour Historique.** Le glyphe de branche est l'imagerie de Git ;
N39 bannit le jargon, l'icône aussi devrait (Nielsen 2). `document-open-recent-symbolic` ou une
horloge (métaphore Time Machine), déjà cohérente avec « voyager dans le temps ».

**m11 — [S28] et une affirmation.** L'adresse citée pour Patchwork est l'index du carnet (vérifié :
aucune page sur les variantes à cette adresse) ; citer l'essai lui-même. Time Machine ne restaure
pas « à côté » par défaut (il remplace, et demande « Conserver l'original / Remplacer / Conserver
les deux ») : la règle d'Inkwell est **meilleure** que son modèle ; l'écrire ainsi, pas comme une
reprise.

## 5. Le nouveau périmètre, lu comme une personne qui écrit

| Exigence | Lecture | Réserves |
|---|---|---|
| **N39** historique | Frise en phrases (« Marie a ajouté 2 phrases »), une entrée par séance (Google Docs), vue du passé en lecture seule avec bandeau et « Revenir au présent », restaurer = nouvelle version sans rien effacer, versions nommées, comparaison par mots : c'est clair, et **aucun** mot de Git n'y apparaît (vérifié : « branche », « commit », « fusion » n'apparaissent dans `spec.md` que dans les interdits). | m1, m2, m6, m10 |
| **N41** variantes | Les cinq verbes (créer, comparer, proposer d'intégrer, intégrer, abandonner), la voie parallèle, la relecture par morceau et les passages des deux côtés **côte à côte** avec trois choix sont compréhensibles sans notion. « Dupliquer le projet » et « intégrer entre projets n'existe pas, et on le dit » sont honnêtes. | **M2** (portée), m5 |
| **N30** outils d'écriture | Orthographe en pointillé jamais rouge ni compté, Relecture à la demande sans score, concentration, objectifs locaux sans série, typographie française au rendu : c'est un outil d'écriture, pas un IDE. Les **extraits** (`;sig`) sont la seule chose qui sent l'IDE ; à garder en PEUT. | aucune bloquante |
| **N31** IA | Rien par défaut, destination visible seulement si activée, dialogue d'envoi du cœur avec le texte d'autrui marqué, propositions par morceau (la **même** présentation que les variantes et les extensions, §4.41 : c'est le point fort de la rév. 5), « marqué IA par Camille », coût et estimation, « se présente comme ». Conforme à l'architecture §15.18 (`:2764-2773`). | M1 (Ctrl+Alt+I) |
| **N32** ressources | Table calme (2s, « Figer »), verbes sans perte, toasts regroupés, jamais de dialogue, aucun nom de fichier (RS-5), « Arrêter ne touche pas à vos documents ». | `:2431` (M4) |
| **N34** non synchronisés | La phrase « ne retire pas les copies déjà reçues » au premier usage, l'aperçu « 12 fichiers concernés », les garde-fous de compilation, « Resynchroniser » qui réimporte (dit). Simple. | — |
| **N37** protection | Un mot (« protéger »), une case, une phrase honnête sous la case, un déverrouillage souvent invisible (trousseau), export en un fichier. La cérémonie de la phrase est la seule lourdeur, et elle est justifiée (ST-8) — **si elle n'a lieu qu'une fois**. | **M3**, m3 |
| **N35** plates-formes | Adwaita partout, gestes du système (Cmd, barre de menus du registre, feux à gauche, trousseaux), fiche « aperçu indisponible sur ce système ». | **M1** |
| **N40** bureautique | Aucune entrée, collage riche converti avec « Coller sans mise en forme », extension possible « par X ». Code : aucune chaîne. | — |

## 6. Sur-ingénierie relevée

- Trois confirmations pour restaurer (m2) ; un toast de kit par sauvegarde (m3a).
- `spec.md:2132-2133`, extraits de texte partagés dans le projet : utile, mais c'est un troisième
  réglage partagé (après dictionnaire et typographie) pour un gain faible ; PEUT.
- `spec.md:2395`, courbe d'activité par ligne dans Ressources : déjà facultative et figée sous
  `reduce` ; acceptable.
- §4.39 est long (onze points), mais N31 demande que **tout** soit visible et réglable : rien n'y est
  gratuit. Rien à retirer.

## 7. Désaccords de `CHANGES.md` : position

1. **Liste des problèmes** (clos) : d'accord avec C13. Ne reste qu'à effacer les résidus (M4).
2. **Barre de mise en forme à l'ouverture.** Position : **le design a raison de garder le choix
   validé** (T11 : barre fixe, Ctrl+Maj+T). `coherence.md` §4 (« se masque ») contredit une
   validation de l'utilisatrice ; c'est l'architecture qui doit se corriger, pas le design. Mais
   `ux-principles.md` règle 2 dit encore « apparaît sur la sélection » (`:12`) : trois textes, trois
   comportements. Une seule question à l'utilisatrice (Q9), puis les trois textes alignés.
3. **Ressources en fenêtre auxiliaire.** Position : **acceptable**, à deux conditions. La HIG
   déconseille les fenêtres secondaires hors dialogues, mais C18 les admet (présentation,
   extensions) et l'argument « voir l'effet d'un arrêt à côté du travail » est réel (Firefox ouvre
   `about:processes` dans un onglet pour la même raison). Conditions : en Étroit et Très étroit
   (§3.4), la vue s'ouvre **dans** la fenêtre principale (page pleine, Échap revient) ; « Voir »
   d'un toast **focalise** la fenêtre si elle est déjà ouverte au lieu d'en ouvrir une autre.
4. **Ctrl+; et Ctrl+Espace.** Ctrl+; → Ctrl+Alt+C est juste sous GNOME, mais tombe sous M1 sous
   Windows (AltGr+C = « ć ») : la règle K-7 de M1 règle le cas sans nouveau raccourci. Ctrl+Espace :
   gardé sous Linux, **à confirmer par l'e2e WebKitGTK** comme prévu ; sous macOS, c'est non
   (changement de source de saisie) : ⌥Échap. Le désaccord devient une table par système, pas un
   choix unique.

## 8. Ce qui est bon, et qu'il ne faut pas rouvrir

- **§4.41** : une seule présentation pour toute proposition (extension, IA, MCP, agent, variante).
  C'est la décision qui rend N31 et N41 cohérentes entre elles, et elle est simple.
- **§4.22** : la vue du passé en lecture seule avec « Revenir au présent », et « restaurer = une
  nouvelle version » : exactement ce que N39 demande, sans une seule notion à apprendre.
- **§4.15** et **§5.16** : des états qui ne promettent que ce qui est acquitté ; G-94 nommé.
- **§4.17** : quatre éléments ; le compteur unique dont le clic va au suivant.
- **§4.42** : la phrase « ne retire pas les copies déjà reçues » dès le premier geste.
- **§4.44** : quatre intentions, quatre mots, jamais tout appelé « Exporter ».
- **`tokens.json`** : 131 paires, états interactifs, HC par planchers, contrat complet ; 524 valeurs
  exactes ; TH-4 tenu avec marge.
- **§10** : chaque section nouvelle a sa ligne de test, en « à écrire », sans fausse preuve.
- L'ensemble est **calme** : rien à l'ouverture que Fichiers, le texte et l'aperçu ; l'IA invisible
  tant qu'elle n'est pas activée ; les Ressources hors du tiroir ; aucun compteur dans l'en-tête.

## Verdict

**ACCEPT.** Les constats des deux revues n°3 sont tous fermés avec preuve ; les 20 changements de
la passe de cohérence sont appliqués ; les 524 contrastes sont exacts et aucun n'échoue ; le
produit reste calme et tourné vers l'écriture. Les quatre majeurs sont des corrections locales à
faire dans la rév. 6 avant toute implémentation de N35, N37 et N41 : une table de raccourcis par
système et une règle de registre (M1), une phrase sur la portée des variantes (M2), « une phrase par
personne, une seule fois » et le défaut au moment du partage (M3), et le nettoyage des résidus qui
feraient échouer K-6 et le test du vocabulaire (M4).

Décompte : 0 bloquant, 4 majeurs, 11 mineurs.
