# Revue indépendante d’architecture — Codex — tour 1

Date : 2026-11-01. Documents : architecture `1.0.0-brouillon.2`, protocole accepté `2.0.0-brouillon.10`. Code observé : HEAD `d0e4d59` (les documents annoncent `362cb4d`). Revue statique, sans modification du code, sans exécution de tests ni de benchmarks, sans commit. La conception a été extraite avec `pdftotext -layout`. Les constats ci-dessous distinguent défaut de contrat, risque déduit et comportement directement observé ; ils ne prétendent pas démontrer une exploitation dans le produit livré.

**Verdict : NOT ACCEPTED — 5 blocker, 7 major, 1 minor.**

## Appréciation générale

Le découpage CRDT/webview, autorité signée/Go, stockage local autoritatif, projection lisible et transport abstrait respecte la direction de la conception. Un document par fichier, un index distinct, des identifiants stables, un registre de contributions, des vues déclaratives et un repli sans greffon sont de bons choix. Les fenêtres, menus contextuels, bouton Extensions, thèmes et événements sont effectivement traités ; ils ne sont pas oubliés.

L’état proposé n’est cependant pas « entièrement spécifié, sans doute restant ». Les contrats les plus sensibles sont encore des intentions ou des portes futures. L’acceptation de l’architecture ne doit pas valoir acceptation implicite d’une modification du protocole déjà accepté, d’un affaiblissement des invariants, ni d’une réduction de la conception.

## Constats bloquants

### B1 — blocker — Le protocole des greffons et son ABI restent à concevoir

**Référentiel :** N25–N26 (`docs/exigences.md:70`), ADR 0034 (contrat public et vecteurs), ISO/IEC 25010:2023 — interopérabilité et analysabilité ; RFC 8949 §4.2 pour l’encodage déterministe.

**Preuve :** `docs/architecture/spec.md:1899`–1960 renvoie expressément la spécification normative à plus tard ; `docs/architecture/decisions.md:988` (AD-72) recommande de l’écrire ensuite. L’annexe décrit seulement `alloc` et `call`, des messages CBOR et des imports WASI « dont les chaînes d’outils ont besoin » (`spec.md:2059`). Aucun WIT, schéma exhaustif de manifeste, registre d’opérations, convention de pointeurs/longueurs/libération, profil précis d’imports, ni protocole d’IPC n’est fourni.

**Conséquence :** deux implémentations indépendantes ne peuvent pas produire aujourd’hui un greffon compatible. WIT ne définit pas à lui seul le transport CBOR d’une ABI personnalisée. Le débogage pas à pas dépend encore d’un prototype de rejeu (`spec.md:1854`). De plus, §15.16 appelle « déterministe » le profil réseau dont `docs/protocol/spec.md:1168`–1181 dit explicitement qu’il ne l’est pas à la réception. La règle majeure/mineure de `spec.md:1625` et la plage SemVer de `:1918` doivent avoir une seule interprétation, y compris pour N−1.

**Pour fermer :** fournir les contrats normatifs annoncés, les octets canoniques signés du paquet, les imports WASI autorisés, l’ABI mémoire, la négociation, les erreurs, l’annulation et quelques vecteurs complets. Montrer le même petit greffon et une proposition annulable en Rust et Go standard `wasip1` ; préciser initialisation/réacteur et compatibilité des autres langages. Définir le contrat de debug réalisable sans promettre le prototype comme acquis. L’implémentation complète peut rester future ; le contrat demandé ne le peut pas.

### B2 — blocker — Le cadre Mermaid ne garantit ni absence de fuite ni interruption

**Référentiel :** principes de sécurité §4, N7/N24, ISO/IEC 25010 — confidentialité, résistance et disponibilité ; [HTML, sandbox des iframe](https://html.spec.whatwg.org/multipage/iframe-embed-object.html#attr-iframe-sandbox), [CSP3, default-src](https://www.w3.org/TR/CSP3/#directive-default-src), [HTML, agents et boucles d’événements](https://html.spec.whatwg.org/multipage/webappapis.html#agents-and-agent-clusters).

**Preuve :** `docs/architecture/spec.md:1729`–1736 transmet la source à du JavaScript tiers dans un iframe `sandbox="allow-scripts"`, protégé par CSP ; `:1766`–1769 promet travail hors du fil d’affichage et plafond de 5 s. Aucun mécanisme de refus des navigations, processus de rendu dédié, interruption indépendante, ni authentification des messages du cadre n’est spécifié.

**Risque déduit :** l’origine opaque protège l’accès au DOM parent, pas toute sortie réseau. La CSP propre au cadre ne suffit pas à démontrer le refus d’une navigation du cadre lui-même avec la source dans l’URL : préciser aussi la politique `frame-src` du parent et son application aux navigations, puis la vérifier sur le moteur cible. L’off-screen ne garantit pas davantage un ordonnanceur indépendant : une boucle synchrone hostile peut empêcher le timer censé supprimer le cadre. Le sandbox de WebKit protège le système, pas nécessairement la réactivité de l’éditeur.

**Pour fermer :** décrire un moteur DOM réellement isolé et tuable indépendamment, avec blocage réseau et navigation au niveau hôte, ou limiter explicitement ce chemin à un moteur livré/de confiance et revoir la portée des greffons tiers. Définir canal par instance, `event.source`/MessagePort, identifiants, tailles, messages tardifs et nettoyage. Vérifier sur WebKitGTK la boucle infinie et la tentative de navigation avec source canari. Le besoin de DOM est légitime ([discussion Mermaid #3886](https://github.com/mermaid-js/mermaid/issues/3886)) ; il ne se résout pas en faisant passer Mermaid pour du WASI pur.

### B3 — blocker — La restauration distante promet une fraîcheur qu’elle ne peut établir

**Référentiel :** ADR 0029 I4/I6 et état honnête ; ISO/IEC 25010 — intégrité et récupérabilité ; précédent [format de dépôt restic](https://restic.readthedocs.io/en/stable/100_references.html#repository-format), qui distingue objets authentifiés et organisation du dépôt.

**Preuve :** `docs/architecture/spec.md:917` autorise une liste mensongère ; `:936`–942 prétend détecter tout rejeu grâce à un compteur supérieur à la dernière tête vue ; `:948`–951 restaure sur un nouvel appareil avec une phrase. Aucune ancre de fraîcheur indépendante n’accompagne cette phrase. La clé est dite par projet **et appareil** (`:929`), mais le nouvel appareil doit retrouver et déchiffrer les têtes des autres. L’écriture de tête est réputée sans besoin d’atomicité parce qu’elle a un seul auteur (`:920`).

**Contre-exemple :** le dorsal rend un ancien ensemble de têtes correctement signées et chiffrées à un appareil neuf. Celui-ci n’a jamais vu de compteur supérieur : il ne peut distinguer cette sauvegarde d’une sauvegarde récente. Le CRDT ne recrée pas des modifications cachées par le dorsal. Même un seul écrivain peut subir une écriture déchirée ou deux téléversements réordonnés.

**Pour fermer :** spécifier une ancre locale/exportée de fraîcheur ou annoncer honnêtement une fraîcheur non vérifiable à la restauration neuve ; définir les clés, dérivation/domaines séparés, nonces AEAD, AAD et format de récupération. Donner le graphe signé d’objets, l’ordre objets→manifestes→tête, la conservation d’une tête précédente, le traitement d’une liste incomplète et les règles de GC. Une restauration ne doit pas réintroduire silencieusement un ancien état d’autorité/purge. Retirer la garantie universelle « ne peut […] rejouer sans être vu » si aucune ancre externe n’est prévue.

### B4 — blocker — Le confinement natif et l’autorité IPC du processus enfant sont incomplets

**Référentiel :** principes §4 (moindre privilège et entrées hostiles), ISO/IEC 25010 — résistance et isolation ; [documentation Landlock : droits, portée et limitations](https://docs.kernel.org/userspace-api/landlock.html).

**Preuve :** l’annexe (`docs/architecture/spec.md:2048`–2057) justifie le processus séparé par une évasion de wazero et affirme alors « aucun fichier, aucun réseau et aucun secret ». Un seul hôte global contient plusieurs greffons aux capacités différentes. Le contrat ne dit pas comment le parent rattache un appel à une autorité non usurpable par un enfant compromis. Le réemploi du confinement sidecar n’est pas suffisant : `internal/sidecar/dir_linux.go:33`–43 protège aujourd’hui les écritures ; `proc_linux.go:17`–18 configure groupe et signal de mort ; `netns_linux.go:13`–21 isole le réseau. La lecture restreinte est encore cible, avec repli permissif dans AD-28 (`decisions.md:490`). Aucun profil complet des descripteurs hérités, signaux, sockets Unix/IPC et cache compilé n’est donné pour l’hôte hostile.

**Risque déduit :** une évasion peut utiliser l’IPC légitime pour demander les droits d’un autre module cohébergé, même sans ouvrir `device.key`. Le parent peut devenir adjoint complaisant. Le cache disque et le rechargement des modules exigent également un chemin de confiance compatible avec « aucun fichier ». Un crash natif tue tous les modules de l’hôte, contrairement à l’isolation individuelle promise par `spec.md:1572`.

**Pour fermer :** définir le profil noyau minimal, les descripteurs, le démarrage avant tout code tiers, le chargement/cache et l’échec fermé. Définir la frontière d’autorité par processus ou compartiment de confiance ; ne pas prétendre isoler des capacités contre une évasion si elles partagent le même enfant. Le parent doit imposer tailles, crédits, identité de session/projet, générations et autorisations indépendamment du contenu de l’IPC. Tester un enfant simulé compromis, pas seulement un module WASM malveillant.

### B5 — blocker — Les ajouts au protocole accepté ne sont pas spécifiés comme tels

**Référentiel :** invariant protocole public (CLAUDE.md), ADR 0034 ; `docs/protocol/spec.md:101`–106 (changement de pli et décision `format`), `:334`–350 et `:1702`–1708 (registre des décisions).

**Preuve :** `docs/architecture/spec.md:1588`–1593 appelle `plugin/<id>` une extension compatible à inscrire au registre, mais le registre accepté ne la contient pas. Son encodage de valeur, la désactivation, les décisions retirées et le pli ne sont pas définis ; PP-6 emprunte `contested` sans définir le registre nouveau. PV-6 (`:1942`) suppose que HELLO annonce la version de l’application/API plugins ; HELLO ne contient que PROJECT, CAPS, VERSIONS des majeures réseau et SESSION_NONCE (`docs/protocol/spec.md:635`). `renders` annonce aussi une nouvelle règle de garde (`architecture/spec.md:1748`), alors que G9 accepté ne la définit pas (`protocol/spec.md:1292`).

**Conséquence :** clients anciens et nouveaux ne disposent pas d’une règle normative commune pour décider quoi charger/interpréter. La simple mention « additif » ne démontre ni compatibilité ni sécurité d’un nouveau pli. Une version d’application annoncée par un pair byzantin ne prouve d’ailleurs pas ses fonctions disponibles.

**Pour fermer :** livrer les propositions normatives exactes, leurs vecteurs et la stratégie de négociation/`format`, puis les faire accepter dans la boucle protocole ; ou borner cette architecture à ce que permet le protocole actuel. Pour `renders`, choisir explicitement validation à l’interprétation ou amendement de garde et justifier le choix. Définir l’état « capacités inconnues/hors ligne » de l’interface plutôt qu’une compatibilité supposée.

## Constats majeurs

### M1 — major — Le geste utilisateur et la révocation des greffons de projet ne sont pas un contrat d’autorisation

**Référentiel :** N23/N24, principes §4, ISO/IEC 25010 — authenticité et protection contre l’erreur ; ADR 0030 (autorité signée).

**Preuve :** `docs/architecture/spec.md:1595`–1611 accorde lecture/propositions automatiquement et autorise l’écriture seulement à la suite d’une commande humaine. Il ne définit pas de mandat à usage borné lié au projet, document, instance, commande et durée, ni la révocation des appels en cours lors d’une mise à jour, désactivation, décision concurrente ou fermeture. Le rendu automatique publié après une frappe (`:1757`) doit aussi être concilié avec PP-2 (« jamais sur un événement »).

**Risque :** une commande anodine peut être utilisée comme autorisation durable de modifier ultérieurement plusieurs documents ; une réponse tardive peut produire un effet après révocation ou changement de projet. Ce sont des ambiguïtés d’implémentation, pas des attaques déjà démontrées.

**Pour fermer :** mandat hôte borné, contrôlé à chaque effet ; annulation et génération sur chaque appel ; purge des contributions, événements et résultats en vol ; procédure de mise à jour avec différence de droits. Définir précisément quels rendus dérivés une frappe autorise à publier et leur attribution. L’auto-chargement borné reste possible sans exiger une confirmation à chaque ouverture.

### M2 — major — Le modèle multi-fichiers corrige G-0 mais ne borne pas la session complète

**Référentiel :** N20, ADR 0028 D2/D9, ADR 0029, ISO/IEC 25010 — capacité, exactitude et récupérabilité.

**Preuve actuelle :** `frontend/src/ui/shell/Sidebar.svelte:17` transmet un chemin ; `frontend/src/App.svelte:197` le jette et appelle seulement `session.focus()`. `frontend/src/workspace/workspace.svelte.ts:42`, `:247` et `:275`–293 n’ont qu’un `MAIN_FILE`, un handle actif et une projection/synchronisation externe correspondante. Le paramètre `?file=` (`:44`–58) change le nom présenté, pas l’identité du document. Le diagnostic architectural est donc fondé ; changer seulement le callback ne suffirait pas.

**Lacune cible :** MF-6 (`docs/architecture/spec.md:806`) borne les « autres » handles à 64, pas le nombre d’ouverts visibles ni les octets cumulés de ces documents. MF-12 (`:829`) promet la projection de tous les documents, tout en détruisant les synchronisations externes au déchargement. L’ordonnancement des réceptions/projections de documents fermés, les écritures manuelles encore sales à l’éviction, l’undo/sélection par document et les ouvertures asynchrones concurrentes ne sont pas contractualisés.

**Pour fermer :** plafonds globaux mémoire/handles/travaux, propriétaire de l’état par nœud, file de projection indépendante de la visibilité et registre des modifications externes des fichiers déchargés. Définir « dernier clic gagne » et réponse obsolète ignorée. Preuves : ouvrir A/B/A, renommer A pendant le chargement, recevoir une modification de B fermé, évincer en mode manuel, redémarrer ; utiliser de vrais documents différents pour txt/csv/md/typ, sans `?file=`.

### M3 — major — La détection proposée pour les fusions CSV laisse des corruptions sémantiques silencieuses

**Référentiel :** ADR 0026 (une seule vérité texte), conception p. 9–10 (divergences visibles), ISO/IEC 25010 — exactitude et protection contre l’erreur.

**Preuve :** AD-52 reconnaît la suppression de ligne concurrente à une saisie et recommande un signalement par différence de nombre de champs (`docs/architecture/decisions.md:807`–826). Cette heuristique ne détecte pas un déplacement de valeur vers une ligne voisine qui garde le même nombre de cellules. Le test intitulé « well-shaped CSV » ne vérifie finalement que `record(r).length > 0` et l’égalité des textes (`frontend/src/editor/csv/collab.test.ts:217`–220) ; ce n’est pas une preuve de préservation des associations ligne/cellule.

**Pour fermer :** conserver le texte si souhaité, mais détecter les conflits structurels à partir des opérations concurrentes et proposer une réparation attribuée, y compris lorsque le CSV est syntaxiquement valide. Sinon choisir des identités stables de lignes/colonnes par ADR. Fixer maintenant le comportement et l’oracle ; reporter le choix après un essai ne clôt pas le risque actuel.

### M4 — major — Les écarts à la conception et les arbitrages restent ouverts

**Référentiel :** conception p. 1 et 13, N18 et demande « aucun doute restant » ; principes, introduction (exception par ADR) ; ADR 0003.

**Preuve :** `docs/architecture/decisions.md:113`–134 réduit plusieurs projets simultanés à un seul projet actif, avec approbation encore demandée. La synthèse dit pourtant « un projet par fenêtre et par processus » (`:22`), option que le corps écarte (`:125`). AD-18 recommande Linux seulement, AD-33 garde deux issues pour pandoc, AD-26 garde deux moteurs à mesurer et assouplit les 300 ms des principes ; `traceability.md:28`–32 garde ces choix ouverts. AD-14 recommande des commentaires de lecteurs alors que la couverture acceptée exige un écrivain (`protocol/spec.md:1222`).

**Pour fermer :** une seule matrice des décisions finales, distinguant recommandation et décision autorisée, et les amendements nécessaires. Ne pas affirmer la fidélité complète tant que ces écarts sont non arbitrés. Les récents sans onglets (AD-47) sont raisonnables et compatibles avec une session multi-fichiers ; cela ne tranche pas la pluralité de projets. L’absence d’implémentation n’est pas en soi un refus d’architecture ; l’absence de décision l’est ici.

### M5 — major — Les coûts IPC et la largeur du système d’extensions sont présumés acceptables

**Référentiel :** principes §3 (`docs/engineering-principles.md:29`), INV-9 ; ISO/IEC 25010 — efficacité et maintenabilité ; [API wazero](https://pkg.go.dev/github.com/tetratelabs/wazero#RuntimeConfig).

**Preuve :** `docs/architecture/spec.md:2112` exige P-PLUG-BENCH **avant acceptation**, sans résultat fourni. Les budgets par point (`:1433`–1454) ne mesurent pas l’ensemble webview→Go→enfant→WASM et retour, avec callbacks, sérialisation et concurrence. Le budget 64 Kio ne caractérise pas les propositions 4 Mio, SVG 4 Mio et sauvegardes par plages. L’architecture exige simultanément tous les points utilisés par le cœur, quatre SDK, génération WIT→CBOR personnalisée, debug/rejeu et extensions d’extensions en 1.1 (`:1665`), avant de prouver le chemin minimal.

**Jugement :** le processus séparé est préférable à wazero dans le processus principal si l’évasion native fait partie des menaces ; le surcoût ne justifie pas de remettre les clés dans le même espace mémoire. Wazero possède déjà une isolation WASM : la défense supplémentaire doit avoir un objectif précis et un coût mesuré. Le risque de sur-ingénierie concerne la surface initiale et les multiples générateurs, pas le principe d’extensibilité.

**Pour fermer :** fournir le benchmark comparatif prescrit (in-process comme témoin, enfant comme cible) sur amd64/arm64, à froid/cache chaud, avec appels réentrants et plusieurs greffons. Définir équité, limites d’appels en vol, compilation bornée et coût RSS total. Garder les points d’extension prévus, mais livrer progressivement registre→commande/panneau/proposition→Graphviz→dorsal, sans dépendre des extensions d’extensions. Un cache chaud ne doit pas être appelé démarrage « à froid » (`:1439`).

### M6 — major — La règle anti-exfiltration est une liste partielle, pas une propriété fermée

**Référentiel :** principes §4 (moindre privilège), ISO/IEC 25010 — confidentialité ; invariant de vérification avant usage.

**Preuve :** `docs/architecture/spec.md:1482` interdit `net.hosts` avec une liste de lectures du clair, mais omet notamment `import.read` (fichier choisi), `self.read` et `presence.read`. Choisir un fichier à lire n’équivaut pas à autoriser son téléversement. `compile.read` figure aussi au niveau projet (`:1598`) sans contenu clairement distinct de `compile.run`. Le contrat pur `renderers` reçoit lui-même du texte sans capacité (`:1720`) : son articulation avec un module disposant par ailleurs de `net.hosts` doit être explicite. L’exclusion par graphe d’appel d’AD-65 ne suffit pas à couvrir toutes ces entrées.

**Pour fermer :** classifier toutes les sources d’information et tous les canaux de sortie, puis calculer l’exclusion sur leurs effets transitifs, y compris événements et rendu. Ou autoriser explicitement certains transferts après accord local et reformuler la garantie. Traiter le besoin réel d’identifiants OAuth/secrets du dorsal sans transmettre de secret de document au module. La phrase absolue « rien ne sort […] en clair » n’est pas établie par la table actuelle.

### M7 — major — L’accessibilité de l’aperçu est déclarée sans équivalent de son information

**Référentiel :** objectif WCAG 2.2 AA du §2 de la spécification, notamment [SC 1.1.1, alternative de même finalité](https://www.w3.org/WAI/WCAG22/Understanding/non-text-content) ; ISO/IEC 25010 — inclusivité.

**Preuve :** AD-44 (`docs/architecture/decisions.md:676`–683) remplace le contenu des pages par `alt="Page N sur M"` et renvoie à l’éditeur source ou au PDF exporté. Cela ne transmet ni le texte rendu, ni le diagramme, ni les diagnostics visuels ; le PDF externe n’est pas une alternative équivalente à toutes les opérations de l’aperçu. Le choix `<img>` est pertinent pour rendre un SVG hostile inerte, mais ne démontre pas l’accessibilité.

**Pour fermer :** une représentation accessible interne du contenu utile (texte/structure ou vue équivalente), des descriptions de diagrammes dans le contrat de rendu et un parcours clavier/lecteur d’écran vérifié. Conserver le SVG inerte. Ne pas confondre absence d’alertes axe avec conformité complète.

## Constat mineur

### m1 — minor — Les marques d’état et la vue des flux peuvent faire passer une cible pour un acquis

**Référentiel :** règle des preuves des principes, ISO/IEC 25010 — analysabilité ; règle locale `docs/architecture/spec.md:24`–30 (« sans marque = en place »).

**Preuve :** §15 décrit majoritairement des fonctions futures sans marque locale ; l’annexe reste proposée et §16 dit qu’aucune extension n’existe (`spec.md:1986`). La vue de contexte (§3.1) limite les sorties à quatre flux, mais `net.hosts` et les sauvegardes en ajoutent. Le schéma de `docs/architecture/README.md` affirme que Go ne lit jamais le texte alors que la recherche sur projection d’AD-42 le prévoit légitimement : l’invariant concerne Automerge, pas le texte projeté.

**Pour fermer :** marquer la section extensions entière comme cible, préciser l’héritage des marques, corriger la vue des flux et distinguer « ne décode pas Automerge » de « ne lit pas le texte ». Conserver une révision de code vérifiable pour les affirmations actuelles.

## Conditions de la prochaine acceptation

Les remèdes ci-dessus demandent des contrats et preuves ciblés, pas une implémentation intégrale anticipée. Priorité : fermer B1–B5, puis les ambiguïtés majeures, réconcilier décisions/spécification/traçabilité et fournir les mesures explicitement requises avant acceptation. Aucun changement au protocole accepté ne doit être implicitement approuvé par cette revue. L’autre reviewer reste indépendant ; aucun verdict commun n’est présumé.

**Verdict final : NOT ACCEPTED. Bloquants : B1 protocole/ABI des greffons non défini ; B2 cadre Mermaid insuffisamment isolé ; B3 fraîcheur de restauration non vérifiable ; B4 confinement et autorité IPC incomplets ; B5 extensions du protocole accepté non contractualisées.**
