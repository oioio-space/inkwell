# Changements de l'architecture

## 1.0.0-brouillon.3 — révision 2 (2026-11-01)

Réponse au tour 1 de revue : [Fable](reviews/01-fable.md) (4 bloquants, 8 majeurs, 9 mineurs) et
[Codex](reviews/02-codex.md) (5 bloquants, 7 majeurs, 1 mineur), les deux « NOT ACCEPTED ».
Chaque constat est fermé ci-dessous par un changement nommé (section de `spec.md` ou `AD-xx` de
`decisions.md`). S'y ajoutent les exigences N27 à N31 et les études
[editor-features](research/editor-features.md) (avec l'addendum N30) et [ai](research/ai.md)
(avec ACP et UTCP). Aucune modification de `docs/protocol/` : les deltas du protocole sont
**proposés** (§15.19) pour sa propre boucle.

### Revue Fable

| Constat | Gravité | Changement |
|---|---|---|
| B-1 cadre `frame` (Mermaid) : JavaScript tiers dans le processus web, sans capacité, sans geste | bloquant | §15.14 réécrit : hôte `frame` **retiré**, Mermaid **différé** derrière la porte P-FRAME (processus web dédié, tuable, sans liaisons, réseau et navigation refusés par l'hôte) puis capacité `render.frame` hors niveau « projet » ; Graphviz `wasm` est le cas livré ; `ui.canvas` différée pour la même raison ; SC-11 ajouté ; `AD-70` |
| B-2 `plugin/<id>` et `renders` ne sont pas des extensions compatibles du protocole | bloquant | §15.19 table **PD-1, PD-2** (règle de tolérance, registres §20.5 et §20.4, schémas, garde `format`, vecteurs) proposés pour une mineure 2.1 ; N23 bloquée d'ici là (§15.10) ; `renders` **sans delta** (fichiers ordinaires, §15.14 point 4) ; `AD-83` |
| B-3 enfermement de l'hôte non exécutable (cache disque, AppArmor Ubuntu 24.04, mémoire) | bloquant | §9.10 nouveau (H-1 à H-8) : modules par `memfd` scellé, **pas de cache disque en v1**, seccomp et Landlock obligatoires, profil AppArmor `userns` livré dans les `.deb`, budget de 512 Mio par hôte et règle d'éviction, échec fermé visible, sort des extensions intégrées WebAssembly ; `AD-87` |
| B-4 niveau « projet » trop puissant | bloquant | §15.10 : **contribuer seulement** ; ni remplacer ni envelopper ; décorations de marque seulement (aucun `replace`, aucun widget qui masque, vérifié par `EXT-ND2`) ; `transforms` par commande ; vue contribuée = autre vue choisie par la personne ; `AD-69` ; question 3 de l'annexe reposée |
| M-1 banc sans témoin | majeur | annexe P-PLUG-BENCH : témoin en processus, trajet complet, RSS par hôte, arm64, 1 et 4 Mio, réentrance ; budget d'interaction de vue ≤ 100 ms (§15.4) |
| M-2 canal hôte ↔ enfant absent | majeur | §7.3 nouveau (IPC-1 à IPC-6) ; chapitre 18 du plan de `docs/plugin-api/` ; `AD-86` |
| M-3 DX-5 incompatible avec le catalogue | majeur | §15.21 API étagée (1.0 : 13 interfaces) ; DX-5 recalculé par étape ; kits Rust et Go en 1.0 ; GREASE seulement dans `host.features()` ; `AD-71`, `AD-85` |
| M-4 ST-7 surestime la fraîcheur ; ST-3 contredit la clé par appareil | majeur | §6.7 réécrit (avec Codex B3) ; `AD-67` |
| M-5 PV-6 suppose un champ du HELLO | majeur | PV-6 ne promet plus de vérification préalable ; aucun TLV demandé (§15.16, §15.19 « retirés ») |
| M-6 racines du ménage des blobs | majeur | paquet de greffon de projet racine du ménage tant que sa décision est debout (§15.10) ; rendus = fichiers ordinaires, gardés par l'index, commande « Nettoyer les rendus » (§15.14) ; retrait d'un greffon global efface son magasin privé (§15.10, §4.2) |
| M-7 G-0 sans étape | majeur | §6.6 « Chemin minimal pour corriger G-0 » : G-0a, G-0b, G-0c, tests FM-OPEN-1…8 ; `AD-77` |
| M-8 coloration par greffon sans anti-rebond | majeur | §15.4 `tokens` : 150 ms, annulation, dernières plages gardées, coupée au-delà de 2 Mi |
| m-1 fletcher, CeTZ | mineur | §15.14 introduction |
| m-2 polices des rendus | mineur | §15.14 point 1 |
| m-3 CSP par empreinte de scripts externes | mineur | sans objet en v1 (cadre retiré) ; P-FRAME l'exigera |
| m-4 « Bonjour » sans fr et en | mineur | §15.15 exemple bilingue (`strings.toml`) |
| m-5 Tâches hors Linux | mineur | §9.10 « Échec fermé, visible » |
| m-6 ISO 25010 n'a pas d'« extensibilité » | mineur | §2 : ligne rattachée à modifiabilité et adaptabilité, « (prioritaire) » retiré |
| m-7 SC 2.4.13 est AAA | mineur | TH-2 : seuls les seuils AA refusent |
| m-8 dépendances d'un greffon de projet | mineur | PP-6 |
| m-9 « lire vos documents » | mineur | §15.5 « lire les documents de ce projet » |

### Revue Codex

| Constat | Gravité | Changement |
|---|---|---|
| B1 protocole et ABI des greffons à concevoir | bloquant | §15.20 **ABI noyau normatif** (réacteur, exports, imports, WASI toléré, réponse, erreurs, négociation, annulation, octets signés du paquet, vecteurs ABI-8) ; « déterministe » corrigé (émission RFC 8949 §4.2.1, décodage strict) ; PV-7 interprétation unique des intervalles ; débogage pas à pas laissé « à prouver » ; `AD-72` |
| B2 cadre Mermaid : ni fuite ni interruption garanties | bloquant | comme Fable B-1 ; P-FRAME exige navigation refusée par l'hôte, boucle infinie tuée, `MessagePort` par instance, source canari |
| B3 fraîcheur de restauration non vérifiable | bloquant | §6.7 : clés (HKDF, domaines), nonces, AAD, graphe et ordre d'écriture, deux têtes, liste incomplète, ménage, **résiduel écrit** pour un appareil neuf, ancres (phrase datée, membres), autorité jamais rétablie à l'ancienne (pli) |
| B4 confinement natif et autorité IPC incomplets | bloquant | §9.10 compartiments par classe de sortie (H-4), §7.3 IPC-3 (autorité imposée par le parent), tests `PLUG-COMPROMISED-*` ; honnêteté : un crash natif tue tout l'hôte, pas un autre compartiment |
| B5 ajouts au protocole non contractualisés | bloquant | comme Fable B-2 ; `renders` : **validation à l'interprétation**, choix justifié ; état « version trop ancienne » de l'interface (PV-6) |
| M1 geste et révocation : pas de contrat | majeur | §15.10 **mandat** borné, contrôlé à chaque effet, révoqué ; réponses tardives jetées (IPC-5) ; publication des rendus réglée (§15.14 point 5) |
| M2 session multi-fichiers non bornée | majeur | §6.6 MF-19 à MF-24 (plafonds globaux, état par nœud, file de projection, registre des modifications externes, « dernier clic gagne ») ; tests FM-OPEN-4…8 |
| M3 détection CSV insuffisante | majeur | §6.6 MF-25 : conflits structurels au niveau des opérations, réparation attribuée, oracle à identités de lignes ; `AD-52` |
| M4 écarts à la conception ouverts | majeur | decisions.md « Matrice des décisions finales » ; `AD-84` ; AD-01 reformulé ; AD-14 et AD-26 : amendement nécessaire écrit |
| M5 coûts IPC et largeur présumés | majeur | banc avec témoin (annexe), API étagée et ordre de livraison (§15.21) ; activation « compilée » et « compilation comprise » distinguées (§15.4) |
| M6 règle anti-exfiltration partielle | majeur | §15.5 **règle de flux fermée** (I0–I3 × O0–O4, RF-1 à RF-5), secrets injectés par Go ; `AD-58` |
| M7 accessibilité de l'aperçu | majeur | §15.17 « Vue accessible de l'aperçu » (export HTML de Typst assaini, `iframe` sans scripts, descriptions, Orca) ; `AD-44` |
| m1 marques d'état, vue des flux, README | mineur | §0.2 héritage des marques ; §15 entière marquée [cible] ; §3.1 flux complétés ; README « ne décode jamais Automerge (il lit la projection) » |

### Demandes de la coordination

Isolation de `frame` → Fable B-1 ; deltas du protocole sans toucher `docs/protocol/` → §15.19 ;
enfermement exécutable (cache, AppArmor, mémoire par hôte) → §9.10 ; niveau « projet » → §15.10 ;
ABI → §15.20 ; fraîcheur des sauvegardes → §6.7 ; banc avec témoin → annexe ; API 1.0 étagée →
§15.21 ; chemin minimal G-0 → §6.6.

### Exigences nouvelles

| # | Changement |
|---|---|
| N27 modèles | §6.8 (TM-1 à TM-7), `AD-78` (question CSS pour l'utilisatrice) |
| N28 erreurs comme un IDE | §15.17 ER-1 à ER-4, `AD-79` |
| N29 service de langage tinymist | §15.17 LSP-1 à LSP-5 (versions, réponses périmées jetées, p95 ≤ 100 ms, multi-fichiers, docstrings sûrs), `AD-80` (remplace `AD-20`) |
| N30 outils d'écriture | §15.17 « Confort d'écriture », `AD-81` |
| N31 IA | §15.18 (fournisseurs par Go, politique de projet, propositions, règle des deux, discussion déclarative, MCP stdio et socket `0600`, ACP en option, UTCP en export seulement), §15.4 points `ai-*`, §15.5 `ai.complete` et puits O4, §7.1 Stream `ai` et socket MCP, §9.2 trois acteurs ; `AD-82`, PD-1 |

Vérifié le 2026-11-01 pour cette révision : révision MCP 2026-07-28 et requêtes sans état
(modelcontextprotocol.io) ; échantillonnage, racines, journalisation dépréciés et prise en charge par
go-sdk ≥ v1.7.0 (README de go-sdk) ; ACP v1 stable, v2 en brouillon, pas de SDK Go
(agentclientprotocol.com). Restent ouverts : gestionnaire Streamable HTTP de go-sdk, affirmations
« [mémoire] » de l'étude IA hors MCP et ACP.

### Désaccords à débattre (tour 2)

| # | Sujet | Fable | Codex | Recommandation |
|---|---|---|---|---|
| D-1 | Portée de l'ABI dans cette spécification | accepte de renvoyer le contrat à `docs/plugin-api/` (AD-72 « d'accord »), exige seulement le canal hôte ↔ enfant (M-2) | bloquant : le contrat (ABI, manifeste, octets signés, vecteurs) doit être fourni maintenant | **noyau de l'ABI normatif ici** (§15.20), interfaces par point et registres dans `docs/plugin-api/` |
| D-2 | Remède pour Mermaid | capacité nommée + **fenêtre Wails cachée** à processus web propre, ou preuve que l'isolation de site suffit | moteur à DOM réellement isolé et tuable, **ou** limiter ce chemin à un moteur **livré et de confiance** | **différer** tout rendu à DOM (P-FRAME) ; à débattre : un Mermaid **livré** dans un cadre, comme le permet Codex, vaut-il le risque en v1 ? |
| D-3 | Carte `renders` | la traiter par un delta du protocole (même mineure que `plugin/`) | choisir validation à l'interprétation **ou** amendement de garde, et justifier | **fichiers ordinaires**, validation à l'interprétation, aucun delta |
| D-4 | Version annoncée pour les greffons de projet (PV-6) | ajouter un TLV pair `PLUGIN_API` au HELLO, ou retirer la promesse | une version annoncée par un pair byzantin ne prouve rien | **retirer** la promesse ; repli à l'ouverture |
| D-5 | Nombre d'hôtes de greffons | deux sortes d'hôtes dès la v1 : « acceptable si PP-1 est la raison » ; prévient contre la sur-ingénierie | l'isolation des capacités contre une évasion exige des compartiments par confiance | **compartiments par classe de sortie** (H-4) ; coût mesuré par P-PLUG-BENCH (RSS par hôte) |
| D-6 | Tableaux CSV | n'a pas relevé AD-52 (« d'accord » sur AD-46–AD-56) | majeur : la détection par nombre de champs laisse des corruptions silencieuses ; décider maintenant | **détection au niveau des opérations + oracle** maintenant, modèle structuré à l'essai ensuite |
| D-7 | Accessibilité de l'aperçu | n'a relevé que les vues déclaratives (libellé accessible) | majeur : `alt` « Page N sur M » n'est pas un équivalent | **Vue lecture** (HTML de Typst assaini) + descriptions |
| D-8 | Plusieurs projets (AD-01) | ne le relève pas | exige une décision, pas une recommandation | décision de l'utilisatrice, inscrite dans la matrice |
| D-9 | GREASE | inutile pour un hôte unique (RFC 8701 vise des implémentations indépendantes) | demande une négociation définie, sans se prononcer sur GREASE | GREASE seulement dans `host.features()` et injecté par le kit de conformité |

## 1.0.0-brouillon.2 — 2026-11-01

Extensibilité (N20 à N26), modèle multi-fichiers, stockage distant, greffons de projet, diagrammes,
débogage, protocole des greffons. Première version soumise à la revue.

## 1.0.0-brouillon.1 — 2026-11-01

Première rédaction : spécification normative, décisions, traçabilité, README.
