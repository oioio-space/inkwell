# Passe de cohérence globale — Codex

Audit documentaire et lecture du code, 2026-11-01. Aucune modification du produit, aucun commit. Les recommandations ci-dessous ne valent ni acceptation d'une ADR ni réduction des exigences de l'utilisatrice.

## Verdict et méthode

La finalité tient : écrire ensemble, hors ligne, relire et produire un document partageable. En revanche, la spécification est devenue celle de plusieurs produits simultanés : éditeur, plateforme d'extensions, client d'agents et coffre chiffré. Le premier parcours multi-fichiers reste cassé alors que ces plateformes sont principalement des cibles. **Priorité : rendre le parcours écrire → partager → retrouver → corriger → exporter fiable, puis étendre ses mêmes services.** Preuves : conception p. 1, 6–13 ; `PROGRESS.md` §Étape 3 ; `frontend/src/App.svelte:197` ; architecture §16.

Corpus : conception.pdf lu par `pdftotext -layout` (références par page/section), PROGRESS, registre complet exigences (W, N, U, L et questions), design rév. 4 et inventaire, architecture rév. 2 et décisions, études plugins/editor-features/ai et sync-ignore, protocole brouillon.10, principes UX, glossaire, CLAUDE et principes d'ingénierie. « T11 », A/B/C/D désignent les plans/lots cités dans le registre, pas de nouveaux identifiants d'exigence. Le registre ne possède pas de famille de lignes T autonome. N38, également présent lors de la lecture, est pris en compte pour les budgets.

Lecture de code ciblée : coque App, workspace, préférences, historique, navigation/export d'aperçu, stockage/blobs, sidecars et leurs variantes par OS. Une déclaration *cible* ne constitue pas une fonctionnalité livrée. Aucun test/build exécuté : ce rapport n'est pas une certification fonctionnelle, de sécurité ou WCAG. Les mesures de l'inventaire sont des preuves documentaires, pas des mesures refaites ici.

Le dépôt avait déjà `docs/exigences.md` modifié ; l'étude `research/sync-ignore.md` est apparue non suivie pendant la lecture. Elles sont lues telles qu'observées et laissées intactes. Les numéros de section sont préférés aux lignes pour les gros documents susceptibles d'évoluer.

### Gravités fixes et référentiel

- **blocker** : empêche un parcours essentiel, rend une promesse de sécurité fausse, ou rend deux contrats normatifs impossibles à satisfaire ensemble. Bloque la fonctionnalité ou la release concernée, pas nécessairement toute expérimentation.
- **major** : perte significative de valeur, d'interopérabilité, de compréhension ou de récupérabilité ; une solution de contournement existe.
- **minor** : incohérence de vocabulaire/organisation sans perte immédiate de données ni blocage du parcours.

Références externes consultées pour cette revue ; leur application aux constats est un jugement d'audit, pas une déclaration de conformité :

- **ISO** : [ISO/IEC 25010:2023, modèle de qualité produit](https://committee.iso.org/standard/78176.html). Employer les attributs de qualité comme critères d'arbitrage ; les budgets particuliers restent ceux d'architecture §2, pas des seuils imposés par ISO.
- **H1–H10** : [heuristiques de Nielsen](https://www.nngroup.com/articles/ten-usability-heuristics/) : visibilité, langage utilisateur, contrôle, cohérence, prévention, reconnaissance, efficacité, sobriété, récupération, aide. Heuristiques, pas exigences légales.
- **HIG** : [principes GNOME](https://developer.gnome.org/hig/principles.html) et [fenêtres](https://developer.gnome.org/hig/patterns/containers/windows.html) : compréhension sans expertise et indépendance des fenêtres principales. La hiérarchie des surfaces proposée ici est une application, pas une obligation GNOME.
- **NIST-K** : [SP 800-57 Part 1 Rev. 5](https://csrc.nist.gov/pubs/sp/800/57/pt1/r5/final), gestion des clés ; **NIST-S** : [SP 800-53 Rev. 5](https://csrc.nist.gov/pubs/sp/800/53/r5/upd1/final), notamment SC-39 (isolation), CP-9/CP-10 (sauvegarde/récupération), SI-7 (intégrité). Références de conception, sans prétendre à une validation FIPS du produit.
- **BIP39** : [spécification, Generating the mnemonic](https://github.com/bitcoin/bips/blob/master/bip-0039.mediawiki) : 24 mots représentent 256 bits d'entropie et 8 bits de contrôle.
- Accessibilité : critères et preuves existants dans design §7/§10 et inventaire ; [WCAG 2.2](https://www.w3.org/TR/WCAG22/). Ne pas confondre axe sans erreur et parcours effectivement utilisable au lecteur d'écran.

## 1. Contradictions et décisions nécessaires

Chaque entrée distingue le conflit encore ouvert de ce que la révision 2 a déjà réparé.

### C01 — Projet chiffré contre projet en clair — blocker

**Preuves.** N37 exige un bloc chiffré sur disque et un export monofichier ; architecture §1.3 NG3 exclut le chiffrement au repos, §6.2 matérialise sources, `.typstdeps`, blobs, sauvegardes, quarantaine et Git ; §9.2 autorise le voleur d'appareil à tout lire. Le code écrit `encodeValue(data)` dans des fichiers ordinaires (`internal/store/kv.go:207`), et décrit les blobs comme fichiers nommés par BLAKE3 (`internal/blob/store.go:4`).

**Résolution proposée.** Nouvelle ADR transversale remplaçant NG3 : conteneur local chiffré authentifié, écritures transactionnelles par blocs et manifeste, indépendantes du format de synchronisation. « Rechiffré à chaque écriture » doit signifier que toute nouvelle donnée persistée est chiffrée, pas réécrire plusieurs Go à chaque frappe. Mesurer amplification d'écriture, perte sur crash et coût de compaction (N38, ISO fiabilité/efficacité). Distinguer conteneur de travail et archive d'échange : un ZIP de fichiers projetés ne préserve pas automatiquement historique, autorités et identité du projet.

### C02 — Chiffrement, BLAKE3/Bao et révocation — blocker pour N37

**Preuves.** Protocole §8.1 identifie/vérifie les plages de blobs par hash et Bao ; §14 exige persistance avant diffusion et revérification au chargement. Architecture §9.4 dit qu'il n'existe aucune clé de groupe ; §6.7 ne chiffre que la sauvegarde personnelle distante, avec clé par projet/personne. N37 demande des clés échangées via un annuaire.

**Résolution.** Garder les hashes du contenu logique et Bao à l'intérieur du coffre et des canaux authentifiés existants ; les noms physiques hors coffre doivent être opaques. Ne pas simplement remplacer le hash du clair par celui d'un chiffrement aléatoire : les références communes et reprises Bao changeraient. Le chiffrement local ne nécessite pas à lui seul de modifier le protocole. Si des ciphertexts partageables ou un relais stockant des données sont voulus, ouvrir une évolution de protocole séparée : domaine de hashes, enveloppes de clés, époques, rotation, reprise. Le retrait coupe les échanges futurs, ne retire ni ancien texte ni anciennes clés déjà reçues. Clé de signature d'appareil et clé de chiffrement doivent avoir rôles et cycles distincts (NIST-K).

### C03 — Coffre contre fichiers réels, index et exports — blocker pour N37

**Preuves.** Architecture §6.7 ST-1 impose un dossier matérialisé aux compilateurs/Git/recherche ; §15.17 LSP-2 lit les fichiers fermés sur la projection ; `frontend/src/preview/actions.ts:75` force cette projection avant export. Étude IA §12.4 place l'index dans les données de l'appareil, §12.5 reconnaît qu'il copie le texte et ne tranche pas son chiffrement. Protocole §14.6 prévoit des caches sur disque utilisateur.

**Résolution.** Définir l'état « projet verrouillé » : arrêt LSP/IA/indexeur, invalidation des accès, clés sorties de mémoire autant que possible ; protéger aussi FTS, vecteurs, miniatures, Git, sauvegardes, staging, journaux, temporaires et rapports. Pour compiler, prototype de matérialisation privée temporaire avec même racine confinée ; préciser nettoyage après crash et limites concernant swap/hibernation. Le tmpfs Linux ne constitue pas une solution portable universelle. Si une projection durable en clair est choisie pour l'édition externe U7, ce doit être un export volontaire clairement signalé, incompatible avec la promesse « seul un bloc chiffré reste sur disque ». Tester le coffre sur chaque OS avant de promettre N37 (NIST-K, NIST-S, ISO confidentialité/récupérabilité).

### C04 — Annuaire de clés contre absence de compte/serveur — major

**Preuves.** N37 évoque un annuaire ; conception p. 33 exclut un serveur d'identité ; architecture §1.2 G2 et §1.3 NG2 excluent compte et découverte globale ; §9.4 ne fournit qu'une clé d'appareil et prévoit un nouveau projet si l'unique admin perd son appareil.

**Résolution.** Carnet de contacts local d'abord, vérification d'empreinte via invitation, annuaire d'entreprise optionnel servant uniquement à retrouver une clé. L'annuaire ne doit pas pouvoir substituer silencieusement un destinataire. Une panne d'annuaire ne bloque pas l'écriture ni l'ouverture d'un projet déjà autorisé. Décrire récupération, perte de tous les appareils et vol avant de dessiner le bouton « Chiffrer ». Un service obligatoire serait un changement de finalité à faire accepter (NIST-K ; H5/H9).

### C05 — Linux seulement contre N35 — blocker pour release multiplateforme

**Preuves.** N35 exige Linux/macOS/Windows, amd64/arm64, pour toutes les décisions ; conception p. 1 les annonce dès le départ. Architecture §2 garde Linux en v1, AD-18 recommande explicitement ce report. §9.5 refuse les compilations bornées ailleurs ; `internal/sidecar/dir_other.go:8` retourne une erreur ; `limit_other.go` n'applique pas les limites mémoire/fichier. §7.3 prescrit `SOCK_SEQPACKET`/memfd ; §9.10 prescrit Landlock/seccomp et refuse les greffons hors Linux.

**Résolution.** Conserver l'échec fermé, mais définir des interfaces de confinement/IPC avec contrats testables identiques et implémentations par OS. Les mécanismes macOS/Windows restent **hypothèses à prototyper**, pas « disponibles parce que Wails compile ». CI native sur six cibles, attaques fichiers/réseau, plafonds et mort des enfants. Linux peut être une alpha clairement annoncée ; appeler cette alpha la réalisation finale de N35 serait faux. Parité du produit requise avant v1 finale, sauf arbitrage explicite (ISO adaptabilité ; NIST-S SC-39).

### C06 — « Jamais installer depuis un pair » contre N23 — major, partiellement résolu

**Preuves.** Étude plugins §5.5 dit qu'un projet recommande sans installer et que l'invitation ne transporte aucun code ; design §11 Q12 dit encore « jamais d'installation venue d'une autre personne ». Architecture AD-60 dit désormais « jamais d'installation **globale** depuis un pair ». §15.10 prévoit paquet répliqué, décision admin, chargement automatique restreint et désactivation locale ; design §4.32 reprend l'extension de projet.

**Résolution.** Retenir la distinction : réception d'un paquet ≠ activation ≠ installation globale. Garder l'autoactivation restreinte de N23, le mandat par geste et le repli lisible. Marquer l'ancienne étude comme antécédent dépassé ; bannir « aucun code reçu » dans la promesse publique si des blobs exécutables voyagent effectivement. La signature prouve provenance/intégrité, pas innocuité (NIST-S SC-39/SI-7 ; H1/H3).

### C07 — N23 et politique IA contre registre de protocole fermé — blocker pour ces fonctions

**Preuves.** Protocole §20.5 ne liste ni `plugin/` ni `ai/` ; architecture §15.19 propose PD-1/PD-2 et §15.10 déclare N23 non implémentable avant acceptation. AD-69 conserve une ancienne phrase « extension compatible », rectifiée dans son addendum. Protocole §1.4.4 prévoit `format` et lecture seule des anciens clients.

**Résolution.** Accepter le delta et ses vecteurs avant émission, tester réellement 2.0↔2.1, conserver/ignorer les espaces inconnus uniquement selon une règle normative. Ne pas supposer qu'ajouter cette tolérance aux clients nouveaux corrige les clients anciens déjà stricts. La bascule `format` doit expliquer aux membres ce qui nécessite une mise à jour. N23 et politique IA partagée restent désactivées tant que ces portes manquent (ISO interopérabilité ; H1/H5).

### C08 — Remplacer le stockage N22 contre copie locale obligatoire — major

**Preuves.** N22 demande remplacement du stockage par cloud/autre FS. Architecture §6.7 annonce explicitement sa « réinterprétation » en sauvegarde personnelle ; ST-1 maintient la copie locale autoritative ; K-6 (§15.3) ferme l'intégrité, pas l'ensemble des dorsaux. AD-67 demande confirmation de cette portée.

**Résolution.** Le noyau fermé et un dorsal enfichable sont compatibles si atomicité, intégrité et chiffrement restent imposés par le noyau. Mais une sauvegarde n'est pas un remplacement du stockage : conserver trois concepts — stockage local de travail, destination de sauvegarde, miroir lisible — et planifier séparément un véritable fournisseur de stockage avec cache hors ligne. Ne pas marquer N22 satisfaite par le seul premier dorsal (ISO remplaçabilité/fiabilité ; H2).

### C09 — Récupération ST-8 irréalisable en 24 mots — blocker pour restauration distante

**Preuves.** Architecture §6.7 ST-3 donne une clé aléatoire de 256 bits ; ST-8 prétend encoder dans 24 mots BIP-39 cette clé, ProjectID, compteur, date et contrôle. BIP39, tableau ENT/CS/MS : la capacité utile de 24 mots est déjà entièrement consommée par 256 bits.

**Résolution.** Phrase pour le secret seul ; fichier/carte de récupération versionné séparé pour ProjectID, destination, date/compteur et instructions, authentifié et exportable/imprimable. Ne pas laisser croire que la phrase de sauvegarde restaure aussi l'identité/signature d'appareil : ce sont des objets différents. Tester une restauration sur machine neuve sans configuration préalable et avec dorsal qui rejoue un ancien état (ST-7 ; NIST-K/CP-10).

### C10 — Pas de téléchargement à l'exécution : portée variable — major

**Preuves.** CLAUDE, invariant Typst, vise typst/tinymist/paquets ; architecture NG6 cite sidecars/paquets/polices, §15.18 emploie la formule générale. Étude plugins §5.5 envisage catalogue téléchargé sur geste ; IA §9.1 rejette l'installation automatique des agents, §12.2 accepte modèle fourni ou endpoint ; W4 exclut les modèles locaux pour le travail des agents du dépôt.

**Résolution.** Table unique des flux : moteurs nécessaires embarqués ; dépendances Typst importées par fichier/projet ; greffons globaux importés explicitement, paquets projet répliqués sous politique ; modèles gérés hors application ; appels IA vers hôte autorisé sur geste. Distinguer téléchargement de code, transfert P2P et requête d'inférence. W4 concerne l'outillage de développement, pas automatiquement le produit ; demander un arbitrage produit si l'interdiction doit également s'y appliquer. Lexical RAG sans modèle est compatible avec le hors-ligne (ISO opérabilité/sécurité ; H4).

### C11 — ACP prétendument soumis aux capacités mais processus autonome — blocker pour ACP sécurisé

**Preuves.** Architecture §15.18 ACP-1 qualifie l'agent de processus à pleins droits et ne le confine que « quand le système le permet », puis le dit soumis à la règle des deux. Étude IA §11.2 reconnaît qu'il sort de notre politique ; §11.3 lui retire seulement `fs`/`terminal` annoncés au client.

**Résolution.** Retirer une capacité du protocole ne retire pas les appels système du programme. Pas de lancement intégré d'agent sous promesse de confinement sans barrière OS prouvée. Sinon mode externe explicitement hors garantie, séparé des fonctions sécurisées du projet ; l'admin ne peut pas le rendre sûr par simple consentement. Reporter ACP, garder fournisseur Go + propositions et MCP borné (NIST-S SC-39 ; ISO confidentialité).

### C12 — IA/RAG : autorité de lecture et index sans modèle unique — major

**Preuves.** Architecture §15.18 IA-2 réduit la session contenant autrui à la lecture du fichier courant ; étude IA §12.6 veut recherche agentique de tout le projet et citations multifichiers ; §5.1 propose `.inkwell/ai-ignore`, §11.4 des règles admin signées dans `.inkwell/ai-rules.md`. §6.1 de l'architecture ne décrit pas encore ces vérités/index ; N33 reste principalement une étude.

**Résolution.** Définir une portée de lecture explicitement choisie avant récupération : fichiers autorisés, contenu sélectionné, accès MCP, provenance, exclusions. Le passage à autrui retire réglages et effets sensibles ; toute extension de lecture exige une décision explicite, pas une recherche qui élargit seule son périmètre. Une seule politique IA signée, fichiers éventuels comme vues ; ni sync-ignore ni score RAG ne donnent une autorisation IA. Révisions et suppression de l'index testées (ISO exactitude/confidentialité ; H1/H5).

### C13 — Problèmes : popover ou onglet, compteur où ? — major

**Preuves.** Design §4.36 et §4.17 : compteur en barre d'état, clic = saut, liste en popover ; architecture §15.17 ER-1 : onglet de tiroir et compteur d'en-tête. Étude editor-features §3.2 recommande le second. Code : `App.svelte:309` connecte StatusBar à `jumpToProblem`, qui refuse d'ouvrir un autre fichier (`preview/actions.ts:32`).

**Résolution.** Service unique de diagnostics, commande unique `ouvrir la cible`, F8 global. Pour rester calme : compte en barre d'état, clic = prochain, flèche = liste courte ; commande « Tous les problèmes » ouvre la même liste en vue détaillée dans le tiroir existant, sans deuxième compteur. Garder les suggestions stylistiques hors total des erreurs. Trancher l'emplacement dans les deux specs avant implémentation (H1/H4/H8/H9).

### C14 — Registre de raccourcis encore contradictoire — major

**Preuves.** Principes UX règle 3 : palette Ctrl/Cmd+K ; design §8.2 : Ctrl+Maj+P et Ctrl+K pour lien ; `ui/toolbar/spec.ts:96` et `editor/format/keymap.ts:16` lient Mod-k au lien. Export proposé Ctrl+Maj+E heurte codeblock (`toolbar/spec.ts:116`, `editor/format/keymap.ts:15`). Inventaire G-82 recommande encore Ctrl+. alors que design §4.36 l'interdit pour GTK et retient Alt+Entrée.

**Résolution.** Source exécutable unique avec contexte et réservations OS. Garder Ctrl+K lien, Ctrl+Maj+P commandes, Alt+Entrée corrections ; attribuer un raccourci libre à l'export après audit du registre. Mettre à jour les recommandations périmées, générer menus/aide depuis le registre ; tester sous vrais WebKitGTK/WKWebView/WebView2. Aucun goût personnel ici : collisions observées, prévention H4/H5 et clavier WCAG.

### C15 — Aucun modèle fourni, mais styles « Rapport » et Markdown/CSS — major

**Preuves.** N27 interdit tout modèle par défaut et demande Markdown+CSS ; architecture §6.8 TM-6 exige Typst/cmarker pour PDF, CSS uniquement HTML ; design §5.19 prévoit Markdown+CSS et situe la bibliothèque dans Documents, contre UserDataDir côté architecture. Design §5.9 propose encore « Rapport » ; Q16 (§11) reconnaît le conflit avec les styles B8. Le projet d'exemple est explicitement distingué du modèle par design §5.2.

**Résolution.** Deux objets : projet-modèle créé par la personne ; réglages de rendu neutres, sans contenu métier. Retirer « Rapport/Lettre » des choix fournis si ce sont des gabarits de document. Expliquer que CSS personnalise HTML, pas PDF, ou faire accepter un second moteur avec ses risques de divergence. Choisir un emplacement unique de bibliothèque et un import/export explicite. Le projet d'exemple reste supprimable et n'alimente pas une galerie préremplie (H2/H4 ; ISO interopérabilité).

### C16 — Sync-ignore contre dépendances obligatoires et convergence — blocker pour politique partagée N34

**Preuves.** Étude sync-ignore §3 exclut `.typstdeps/` par défaut, interdit réception des chemins exclus localement, puis retire de l'index les fichiers déjà partagés par opération signée ; §2 dit les dépendances reconstruisibles. N5 et architecture §1.2 G7 exigent tout recevoir ; conception p. 5–6 et architecture §6.8 TM-3 interdisent de dépendre d'un téléchargement pour compiler. Protocole §9.4/§9.7 définit admission par document/changement, pas par filtre local de chemins.

**Résolution.** Garder `.typstdeps/`, chartes, polices et ressources référencées dans la clôture des dépendances partagées. Exclure sorties identifiées, caches et déchets ; jamais tous les PDF indistinctement. « Ignorer » signifie d'abord ne pas importer/publier un nouveau fichier local, pas retirer un document déjà admis du graphe Automerge. Séparer publication, sélection des blobs à recevoir et visibilité locale. Un filtre local différent ne doit pas entraîner refus d'un message mixte ou du passé causal indispensable. Définir sémantique des fichiers déjà partagés, renommages, réinclusion et suppression par ADR/protocole avant promesse ; afficher « ne retire pas les copies déjà reçues ». Même prudence pour « les autres ne peuvent pas voir » dans l'étude (H5/H9 ; ISO exactitude/interopérabilité).

### C17 — Versions : Git vérité ou projection ? — major

**Preuves.** CLAUDE invariant « versions → go-git (projection) » ; conception p. 9 décrit un jalon Git ; architecture §6.1 attribue vérité aux entrées Automerge avec têtes par document ; AD-13 propose de remplacer la bibliothèque go-git par un écrivain Git minimal, en conservant la projection. `frontend/src/history/versions.ts:50` et `HistoryDrawer.svelte:195` portent déjà le parcours de nommage.

**Résolution.** Versions nommées = manifestes de têtes Automerge du projet ; Git = projection/export optionnel, jamais autre autorité. L'instantané doit inclure index, ressources et principal, pas seulement le fichier visible. Restaurer crée une modification, ne remplace pas silencieusement le stockage. Accepter l'écart AD-13 et corriger les textes de référence ensemble (ISO exactitude ; H4/H9).

### C18 — Fenêtres et projets, exceptions mal distinguées — major

**Preuves.** Conception p. 13 : plusieurs projets simultanés ; principes UX règle 1 : une fenêtre ; AD-01 recommande un seul projet par processus/v1, avec absence de synchronisation du projet fermé ; U28 et `App.svelte:316` prévoient déjà la fenêtre public. N24 ouvre aussi des fenêtres d'extension.

**Résolution.** Un espace de travail par projet, avec fenêtres auxiliaires présentation/dialogues explicitement distinctes ; MVP un projet ouvert, avertir que fermer arrête sa synchro. Plus tard, hôte réseau partagé et Runtime par projet comme AD-01 option 2 ; pas plusieurs processus partageant librement la même clé. Les fenêtres d'extension restent auxiliaires et révocables (HIG fenêtres ; ISO fiabilité).

### C19 — Promesse « rien perdu » et ordre qualité/habillage — major

**Preuves.** Design §5.16 affirme « Rien n'est perdu » même pour erreur inattendue ; architecture §2 admet 200 ms de perte kill-9 et 500 ms coupure, §10.3 répète la garantie absolue. PROGRESS introduction place l'habillage en dernier, étapes 5/6 contiennent fiabilité/accessibilité ; inventaire G-0/G-39/G-40 montre des tâches de base non praticables.

**Résolution.** États distincts : conservé en mémoire, durable localement, projeté, envoyé, reçu ; message fondé sur l'état effectivement connu. Ne promettre aucune perte après crash que pour les données acquittées durables. Qualité, clavier et reprise accompagnent chaque tranche dès MVP ; l'étape 5 amplifie les preuves et l'étape 6 industrialise la livraison (ISO fiabilité ; H1/H9).

### C20 — Onglets demandés mais écartés par AD-47 — major

**Preuves.** N19 exige plusieurs fichiers ouverts avec onglets ; design §4.25 décrit leur barre et leurs interactions ; architecture §6.6 MF-2 cite les onglets comme consommateurs de l'identifiant de nœud. Pourtant AD-47 recommande « un document actif et une liste de récents » en v1 et sa matrice de décisions finales (§Synthèse) reprend « sans onglets ». Son argument « un seul fichier visible » ne suffit pas : plusieurs onglets peuvent parfaitement avoir une seule vue active.

**Résolution.** Retenir les onglets demandés, avec chargement paresseux, un seul éditeur visible et limite de ressources indépendante du nombre d'onglets. Réécrire AD-47 autour de ouverts ≠ chargés ≠ actif, sans réinterpréter N19. Le correctif G-0 peut précéder la barre d'onglets, mais ne clôt pas à lui seul cette exigence. H3/H4 et ISO opérabilité/efficacité.

## 2. Manques, classés par valeur pour écrire

« Absent » signifie ici **aucun contrat complet observé dans ce corpus**, pas preuve qu'aucune ligne n'en parle ailleurs. Les hypothèses de besoin sont indiquées. Plusieurs sujets suggérés dans la demande sont déjà couverts : ils ne doivent pas être recréés sous de nouveaux noms.

| Priorité / gravité | Couverture observée et manque précis | Proposition et preuve de réussite |
|---|---|---|
| 1 / blocker | **Récupérer son travail sans expertise.** Architecture §6.2/§10 décrit backups/quarantaine ; `internal/store/kv.go:433` offre des sauvegardes par liens durs ; AD-38 archive et §6.7 restauration distante ne donnent pas un parcours unifié. Les liens durs du même disque ne couvrent pas sa perte. | « Sauvegarder / Restaurer une copie » : destination indépendante, date du dernier succès vérifié, prévisualisation, restauration à côté sans écraser. Exercice sur appareil neuf et archive altérée. CP-9/CP-10, ISO récupérabilité, H9. |
| 2 / blocker pour N37 | **Identité/appareils et récupération de clés.** Groupes prévus (PROGRESS étape 2), perte du seul admin = nouveau projet (architecture §9.4). La phrase ST-8 est celle du stockage distant seulement. | Parcours ajouter/remplacer/retirer un appareil ; distinguer restaurer les documents, l'autorité admin et le droit de déchiffrer ; kit de secours vérifié à sa création. Pas de copie sauvage de device.key. Test perte de tout appareil sans pair joignable et perte d'un seul appareil. NIST-K, H5/H9. |
| 3 / major | **Révision de groupe complète.** Commentaires déjà exigés (PROGRESS étape 4 ; conception p. 11–12), mais design §4.27 laisse hors périmètre et le code n'a qu'un vide (`HistoryDrawer.svelte:206`). AD-14 constate qu'un lecteur ne peut pas commenter. | Une seule spécification fils/résolution, ancrage supprimé, mentions, recherche, conservation, droits ; signaux « non lu » locaux et convergence des résolutions. Choisir explicitement le protocole des commentaires lecteurs avant v1 ; MVP lecteurs sans commentaire annoncé honnêtement. ISO complétude, H1/H4. |
| 4 / major | **Messages différés et notifications fiables.** Conception p. 34 prévoit une mention seulement quand connecté ; design §5.15 couvre présence et divergences, pas livraison de mentions quand deux personnes ne se croisent jamais. | Hypothèse de besoin : boîte « À relire » dérivée des commentaires/activité, accusé local de lecture, déduplication à la reconnexion, silence/notifications OS optionnels. Sans pair commun simultané ou stockage intermédiaire, ne promettre aucune livraison immédiate. Pas de chat distinct nécessaire au MVP. H1/H8, ISO disponibilité. |
| 5 / major | **Entrer depuis les outils d'écriture existants.** AD-33 couvre DOCX/RTF/LaTeX via pandoc, encore à décider ; conception p. 37 prévoit collage HTML/Word. Aucun parcours complet ODT/EPUB/Google Docs observé. | Prioriser DOCX et ODT par fichier, puis export DOCX pour rendre le texte à un relecteur ; Google Docs via téléchargement DOCX/ODT, sans OAuth initial. Rapport de pertes avant validation : images, citations, notes, équations, commentaires ; jamais promettre aller-retour fidèle. EPUB/HTML ensuite. Corpus de documents réels, original gardé intact. ISO interopérabilité, H5/H9. |
| 6 / major | **Mises à jour dignes de confiance sur tout le cycle.** AD-17 spécifie manifeste signé, séparation paquet/archive et choix de vérification encore ouvert ; PROGRESS étape 6. Restent rotation/révocation de clé, refus du rollback, compatibilité des données et échec d'installation. | Une procédure de publication/reprise, ancre de confiance, rejeu d'ancien manifeste, mise à jour hors ligne, canaux de paquets ; restauration des données distincte d'un retour d'exécutable. Test mise à jour interrompue et N−1 après migration. NIST-S SI-7 ; ISO installabilité/récupérabilité. |
| 7 / major | **Accessibilité de la collaboration**, pas seulement des boutons. Design §7, §4.36 et §5.15 couvrent clavier/annonces, architecture AD-44 vue lecture ; inventaire G-4/G-60 révèle contraste/annonces fragiles. Aucun scénario complet de relecture collaborative au lecteur d'écran observé. | Script de bout en bout : identifier la personne autrement que par couleur, atteindre sa modification, lire/répondre/résoudre un commentaire, suivre puis quitter sans perdre le focus ; rafales de changements regroupées, annonce désactivable. Orca puis VoiceOver/NVDA sur plateformes livrées. WCAG, ISO inclusivité, H3. |
| 8 / major | **Migration compréhensible.** Architecture §6.4 et AD-10 couvrent déjà version/layout et reprise technique ; §16 dit que layout manque encore. Manque UX du volume nécessaire, temps, annulation avant bascule et partage avec ancien client. | Avant migration : copie nécessaire/espace estimé et personnes nécessitant mise à jour ; après : ouvrir une copie exportable en lecture si version trop récente. Tests ancien projet réel, disque plein et interruption. ISO compatibilité/récupérabilité, H1/H5. |
| 9 / major | **Désinstaller sans effacer ses écrits ni laisser des secrets oubliés.** Design §4.29 couvre retrait des extensions ; architecture §6.2 distribue données/config/cache/logs ; aucun contrat de désinstallation de l'application observé. | Garder projets et archives par défaut ; écran de nettoyage volontaire séparant caches/index, préférences, clés et documents ; avertir sur pertes de clés, pas d'effacement automatique des projets. Parcours documenté pour les trois OS. NIST-K ; ISO installabilité, H3/H5. |
| 10 / major | **Très grands projets : capacité réelle, pas seulement octets par fichier.** Architecture §2 borne 10 000 nœuds/docs, §12.2 plafonne l'édition ; design §5.17 décrit progression. Manque budget de l'ensemble : index, historique, miniatures, plusieurs hôtes, blobs et coffre cumulés. | Un corpus de livre avec médias et long historique ; budgets cumulés par projet/processus, priorités texte actif puis dépendances puis arrière-plan, annulation, refus compréhensible et import partiel sans casser les références. N38, ISO capacité/efficacité. |
| 11 / minor | **Réglages entre ses appareils.** Design §3.1 interdit explicitement la synchro de disposition ; §5.13 distingue local/projet ; AD-21 prévoit fichier Go, code encore localStorage (`store/settings.svelte.ts:13`). Rien ne décrit transfert personnel des préférences. | Hypothèse de besoin : exporter/importer un profil sans secrets, puis synchro personnelle optionnelle du dictionnaire/extraits/raccourcis ; jamais imposer largeur de fenêtre, chemins, identifiants ou droits d'extensions à l'autre machine. H3/H4, ISO adaptabilité. |
| 12 / minor | **Lire sans installer, mobile/web.** NG1 exclut l'application mobile ; AD-34 prévoit invité navigateur v2. Pas de parcours lecteur mobile complet. | Répondre d'abord par PDF/HTML accessibles et archive, pas par un nouvel éditeur. Lien web vivant ensuite avec expiration/jeton, révocation, hors ligne et ressources ; nouvelle frontière réseau à auditer. Hypothèse de besoin, valeur moindre que restaurer un manuscrit. ISO interopérabilité/confidentialité. |

### Couvert en conception, à fermer dans la livraison

- **major — Accueil/aide** : design §5.1, §4.26 et principes UX règle 8 définissent déjà exemple et guide embarqué ; inventaire G-39/G-58 les dit absents. Ajouter au même guide récupération, limites hors ligne et partage/encryption ; ne pas lancer une seconde « plateforme onboarding ». Valider créer et partager sans vocabulaire technique (H2/H10).
- **major — Impression** : design §5.12 prévoit PDF temporaire et dialogue système, mais marque la faisabilité à confirmer ; ce n'est donc pas une idée oubliée. Fermer le parcours page/plage/annuler/erreur, avec protection du PDF temporaire selon C03, sur chaque OS. MVP : PDF ouvrable dans le lecteur système ; v1 : impression intégrée. H3/H9, ISO opérabilité.
- **minor — Licence/À propos** : design §5.18, ADR 0012 cité par le dépôt et `ui/AboutDialog.svelte:79` montrent que ce n'est pas absent. Compléter accès aux licences des moteurs/polices/paquets/plugins et conditions de leurs ressources exportées ; pas de nouvelle page marketing dans l'éditeur. ISO auto-description, H10.
- **major — Incidents sans télémétrie** : AD-16 et architecture §11 O-6 prévoient archive locale expurgée et envoi volontaire ; manque la validation concrète d'un crash après redémarrage. Prévisualisation des pièces jointes et canaris secrets/contenu, aucune soumission automatique ; tester une panne du moteur, de la webview et du disque. H9, NIST-S, ISO récupérabilité.

## 3. Chevauchements à fusionner

| Gravité | Preuves du chevauchement | Unification proposée |
|---|---|---|
| major | Design §8.2 registre commandes ; architecture §15.4 registre contributions, §15.18 commandes IA/MCP ; raccourcis réels répartis entre App:127, toolbar/spec et editor/format/keymap. | Un registre de contributions possédant commandes et contextes ; menus/palette/aide/IA ne sont que des projections filtrées. Les points WIT ne doivent pas créer un deuxième catalogue de commandes. |
| major | Architecture §15.17 diagnostics et relecture ; design §4.36 sépare suggestions/erreurs ; CSV possède `editor/csv/problems.ts` (inventaire G-81). | Un modèle de diagnostic avec source, gravité, cible ancrée, révision et action ; vues Erreurs et Relecture filtrées. Distinguer erreur du document et panne d'un moteur. |
| major | Étude editor-features §3.6 met les commentaires dans Activité ; design §4.27 en onglet distinct, §4.38 ajoute Relecture ; conception p. 8–9 mélange fil d'activité et historique. | Un tiroir « Relire » : commentaires/propositions à traiter, activité récente, versions comme sous-vues ; même donnée événementielle et navigation. Relecture stylistique reste filtre du document, pas un second système de commentaires. |
| major | Étude IA §12.6 index.chercher/lire/plan ; AD-42 recherche projet ; design §4.16 Sommaire/Liens et B6 Rechercher. | Un service de navigation `ouvrir(cible, révision)` et des index dérivés partagés quand cela est pertinent ; recherche humaine exacte/regex conserve sa sémantique, IA ajoute classement/portée. Ne pas remplacer regex par BM25. |
| major | Architecture §6.7 sauvegarde, miroir ; §6.8 modèles ; AD-38 archive ; conception p. 9 versions. | Quatre intentions explicites : restaurer un état (version), récupérer après perte (sauvegarde), transmettre un projet (archive), commencer un autre projet (modèle sans membres/historique). Un pipeline de snapshot validé, plusieurs politiques d'inclusion ; ne pas appeler toutes ces actions « Exporter ». |
| minor | Glossaire : « version nommée » et « jalon » pour Milestone ; brouillon v2 contre variantes déjà visibles (`HistoryDrawer.svelte:182`) ; inventaire G-61 relève Fusionner. | UI : « version » pour point conservé, « variante » pour travail alternatif si livré, « intégrer » pour merge ; termes techniques derrière Détails. Actualiser le glossaire avant génération d'aide. |
| minor | « extension », « greffon », `.inkplug` (étude plugins §5.5), `.inkwell-plugin` (architecture ABI-7), projet/user/built-in. | UI « extension », code plugin, un format de paquet versionné ; la portée et la provenance sont des attributs, pas trois gestionnaires. Marquer l'étude comme historique quand elle diffère de l'ABI. |
| major | Design §4.40 En cours, §4.35 Inspecteur ; architecture §11 journaux/mesures, IA §5.6 coûts et journaux ACP §11.4. | Une vue Ressources (état/arrêt) et un journal local expurgé ; Inspecteur développeur = détail de ces services. Compteurs IA dans la session, agrégat Ressources, jamais deux facturations calculées différemment. |
| minor | Inventaire G-12/G-13/G-16 : dialogues recopiés, sept bandeaux, trois toasts ; code `App.svelte:234` à `:261` les monte indépendamment. | Dialog/AlertDialog, BannerHost à priorité et ToastHost uniques ; même politique de focus/annulation. Réduction concrète de charge cognitive (H4/H8), pas refonte cosmétique. |
| major | Registre N30 contient aussi un N31 incorporé sur la même ligne ; N31 existe ensuite séparément, U34 est réutilisé pour deux demandes ; inventaire G-39 répété ; N36 pointe un fichier coherence différent de ce rapport. | Conserver les IDs existants comme références historiques, attribuer ultérieurement des suffixes stables aux doublons et des liens précis. Une table exigences → décisions → surface → test → état ; aucune exigence « faite » parce que son texte est écrit. Ne pas renuméroter silencieusement. Preuves : exigences §Réseau et §Interface ; inventaire §2.4. |

## 4. Carte produit cohérente

Carte **proposée**, pas description du code livré. Elle couvre les fonctions du registre en les regroupant par intention ; les composants réels sont ceux d'inventaire §1 et d'App:195–322.

| Surface | Contenu et commandes | Référence / état de surcharge |
|---|---|---|
| **Accueil / Projets** | Nom/couleur au premier usage ; récents ; créer vide Typst/MD/txt/CSV, ouvrir/importer, rejoindre ; sous-vue Mes modèles et modèles partagés ; restaurer une archive. | Design §5.1/5.2/5.19, N20/N27. La création actuelle ne propose que Typst/MD : permettre aussi les deux autres types évite de transformer une simple note en document Typst. **major** : inventaire G-39/G-40, parcours non livré. |
| **En-tête** | Projet et fichier ; ouvrir/fermer gauche, aperçu/lecture ; état local/synchro ; avatars → Personnes ; Partager ; menu Document/Application. Extensions dans le menu, action épinglée seulement sur choix ; accès IA sur demande. | Design §4.1/4.30/4.39, §5.3/5.14 ; architecture §15.13. **major** : compte problèmes + IA + puzzle + actions épinglées s'ajoutent à tous les boutons existants ; éviter une seconde barre d'outils permanente. |
| **Onglets + document** | Fichiers ouverts, type adapté : Typst/MD source avec barre contextuelle ; CSV grille/texte ; txt ; code en lecture colorée ; image/PDF/binaire avec vue sûre. Menu Insérer : image, tableau, note, citation, renvoi, symbole, diagramme. Complétion/signature/erreurs au point d'écriture. | Design §4.25/4.28/4.36–38 ; N20/N28–30 ; U4/U9/U19–26. **blocker actuel** : App:197 et workspace:44–58 n'offrent pas le parcours multi-fichiers réel. |
| **Gauche** | Deux accès principaux Fichiers et Sommaire. Liens dans détails du fichier/menus ; Tâches accessibles comme filtre de navigation, onglet optionnel si utilisé. Corbeille dans Fichiers. | Design §4.16 impose actuellement quatre onglets. **major** : quatre titres + plugins saturent 264 px ; proposition à réconcilier avec U29/U30, sans supprimer la fonction. |
| **Aperçu / Vue lecture** | Pages pour PDF ; vue HTML accessible ; saut source ↔ rendu ; export, impression, zoom/page ; présentation ouvre ses fenêtres dédiées. Layout PDF/HTML explicite pour Markdown. | Design §4.18/5.9/5.11/5.12 ; architecture §15.17/AD-44 ; U10/U28. **major** : aperçu + tiroir + gauche simultanés compriment l'écriture ; fermer une zone secondaire selon largeur. |
| **Tiroir** | Trois destinations visibles à la fois : **Rechercher**, **Relire**, **IA** (si activée). Relire contient versions/historique, activité/commentaires, suggestions ; Problèmes peut s'y ouvrir en liste détaillée. Inspecteur seulement en mode développeur. | Design §3.1 cumule Historique/Activité/Rechercher/Relecture/IA/Inspecteur et futur Commentaires ; ER-1 ajoute Problèmes. **major** : huit vues potentielles, concurrence avec aperçu. Conserver raccourcis directs pour chaque sous-vue. |
| **Barre d'état** | Problèmes/action, non-enregistré, position et mots ; page seulement en aperçu ; zoom/langue/encodage/fin de ligne/layout via menu de détails. Ni deuxième synchro ni deuxième liste de personnes. | Design §4.17 énumère treize rubriques ; inventaire G-30 duplication état. **major** : état non essentiel sous ⋯, essentiels atteignables au clavier. |
| **Préférences — Écriture** | Éditeur, orthographe/langues/dictionnaires, extraits, typographie, objectifs personnels, concentration, raccourcis. | Design §5.13, N29/N30 ; regrouper plutôt qu'ajouter une page par fournisseur. |
| **Préférences — Apparence et Moi** | Thème accessible, variante système/clair/sombre, taille ; nom/couleur ; appareils/identité et carnet de clés. | Design §2.10/5.13 ; N22/N35/N37. Séparer choix visuels et autorité cryptographique même si accessibles depuis la même fenêtre. |
| **Préférences — Projet** | Principal, rôles/politique de suppression/verrous, sauvegarde et restauration, exclusions avec aperçu, chiffrement/récupération, dépendances/charte, règles partagées d'écriture et IA. | Architecture §6/§9 ; N4/N22/N27/N34/N37. **major** : distinguer ce projet / cet appareil / mes appareils sur chaque ligne. Actions admin seulement là où autorisées. |
| **Préférences — IA** | Activer, profils entreprise/externe, tester connexion, coûts/limites, modèles, périmètres MCP et agents ; politique projet liée à Projet. RAG : état/reconstruire/effacer sous contexte documentaire. | Design §4.39/5.13 ; études IA §12 ; N31/N33. Paramètres experts repliés, coût inconnu montré comme inconnu. |
| **Préférences — Extensions / En cours** | Installer depuis fichier, gérer/désactiver/mettre à jour/retirer, portée/provenance/droits ; Ressources pour plugins, LSP, compilateur, index, IA ; arrêter une tâche sans tuer l'édition. | Design §4.29–35/4.40 ; N21–26/N32. Un seul point d'entrée pour gestion et diagnostic. |
| **Préférences — Application** | Langue fr/en, mises à jour/confiance, réseau/confidentialité, sauvegarde profil, nettoyage volontaire. Mode développeur et détails repliés. | AD-16/17/21 ; design §5.13/5.18. Complète les lacunes §2 sans nouvelles barres permanentes. |
| **Dialogues sur geste** | Partager/rejoindre/empreinte ; Personnes ; exporter/imprimer ; importer ; modèle ; autorisations d'extension ; aperçu d'envoi IA ; déverrouiller/récupérer ; confirmation d'effet irréversible ; aide/À propos/rapport local. | Design §4.8/4.9 et §5 ; N37 ajoute le parcours coffre encore à concevoir. Un composant/focus commun, pas dialogue propre à chaque extension. |

**Simplification mesurable (major).** État initial : gauche Fichiers, éditeur, aperçu si largeur suffisante, aucun tiroir ; barre contextuelle masquable. Une seule zone auxiliaire superposée en fenêtre étroite, retour garanti à l'écriture. Cacher les fonctions non installées/non disponibles, jamais afficher un onglet vide pour promettre l'avenir. Préserver trois accès aux actions : menus, clavier disponible, palette. Sources : principes UX règles 1–3, design §3.4 et §4.27, inventaire G-37/G-62 ; H4/H6/H8 et HIG. Vérifier par les cinq tâches chronométrées de principes UX, sans imposer de budget temporel inventé avant mesure de référence.

## 5. Phasage : livrer des usages complets

Les étapes 0–6 restent les repères existants ; les vagues T11 et API 1.x sont des sous-lots, pas deux roadmaps concurrentes. Proposition : MVP = alpha utile, v1 = première release générale conforme au périmètre accepté. Une alpha Linux ne clôt ni N35 ni N37. Aucun délai calendaire sérieux sans mesure des prototypes de confinement/coffre et fermeture des écarts d'admission (PROGRESS §État actuel).

### MVP — rétablir les étapes 1–3, avec qualité 5 et produit 6 minimaux

| Lot / gravité si absent | Exigences existantes à couvrir | Valeur, risque et porte de sortie |
|---|---|---|
| **Écrire/retrouver — blocker** | U1/U3–U8/U13/U14/U24–26, N17/N19/N20 ; T11 A, B1, B4/B5/B6. | Ouvrir réellement plusieurs fichiers, un principal distinct de l'onglet actif, créer/renommer/corbeille/restaurer ; txt/MD/Typst/CSV fonctionnels, pas via `?file=`. Écrire deux fichiers et exporter le bon projet après redémarrage. Ouvrir sans compte ni réseau. |
| **Collaborer sans perdre — blocker** | N1–3/N5–16/N-1, N6 pour CSV, U7/U8/U32–34 ; étapes 1/2. | Admission/quotas et protocole effectivement migré avant exposition non maîtrisée ; inviter avec rôle et empreinte, retirer/suspendre, continuer hors ligne puis converger. Deux appareils/processus réels, coupure/reconnexion, disque plein et crash. Aucun contournement silencieux des gardes. |
| **Comprendre/corriger/produire — blocker** | N28/N29 socle, U4/U9/U15/U17/U18 socle, L1 ; étape 3 C1–C3. | Diagnostic cliquable multifichier + F8 ; LSP complétion/signature/survol de charte ; aperçu et PDF hors ligne avec dépendances reçues. Recherche branchée. Export PDF accessible depuis lecteur système ; impression intégrée peut suivre. |
| **Première minute et secours — blocker** | N17/N19, U31/U32, L3/L5 ; étapes 5/6 avancées en tranche. | Accueil/projets, aide embarquée, identité compréhensible, sauvegarde indépendante/restauration simple, messages justes, clavier/focus/contrastes, fr/en ; archive locale de diagnostic volontaire. Rien de « fait » avant test d'usage, conformément au préambule exigences. |
| **Préserver/revoir — major** | U11/U12/U27 socle ; étape 4 avancée. | Versions nommées de projet, comparer/restaurer sans écraser les autres ; signaler les divergences. Une personne doit pouvoir se tromper et revenir sans Git. Les variantes complexes ne conditionnent pas ce socle. |
| **Architecture extensible minimale — major** | N18/N21/N24 fondations, U26 ; architecture §15.21. | Registre unique commandes/types/navigation, séparation vues et noyau. Livrer le point commande/panneau/proposition en prototype sans faire dépendre l'ouverture ou la saisie de tout le SDK. Prouver le chemin extensible, éviter la réécriture de tout le produit avant usage réel. |

Une alpha réservée à Linux et à des projets en clair doit l'annoncer précisément et ne doit pas être présentée comme « release finale fonctionnelle à 100 % » L5. Si le premier téléchargement public doit déjà respecter N35/N37, les lots ci-dessous deviennent des portes **avant ce téléchargement**, pas des reports automatiques.

### v1 — terminer 3/4 et industrialiser 5/6

| Lot / priorité | Exigences et étapes | Justification / dépendance |
|---|---|---|
| **Portabilité et coffre — blocker pour la promesse finale** | N35/N37, L1/L2/L3/L5 ; étapes 2 (clés/store), 3 (sidecars), 5/6 (preuves/paquets). | ADR d'enveloppe/récupération et contrats OS avant stabilisation des données ; six cibles et restauration réelle, export monofichier clair/chiffré, tests crash/migration. Le coffre ne s'ajoute pas à la fin par une case dans Préférences. |
| **Relire ensemble — major** | Commentaires/mentions étape 4, N4/N6, U11/U12/U16/U27/U29/U30. | Fils ancrés/résolution/notifications différées ; commentaires des lecteurs seulement après extension de couverture AD-14 ; verrous admin explicités, sommaire/tâches/liens. Valeur directe pour révision collective, risque de schéma/protocole à traiter avant UI. |
| **Extensions réellement utilisables — major** | N21–26/N32, N22 par paliers, N23 après PD-1/PD-2 ; API §15.21. | Installation fichier, pause/reprise/retour version, SDK stable réduit, ressources dès le premier plugin tiers, démonstrateur Tâches/Graphviz. Isoler et valider sur les OS publiés. Remplacement complet du stockage peut attendre ; ne pas confondre ce report et une satisfaction de N22. |
| **Modèles et écriture — major** | N27/N30, U9/U15/U17/U18 ; étape 3/4 puis 6. | Modèles personnels/import/export sans modèles fournis, charte versionnée et rendu Markdown tranché ; orthographe/dictionnaire, références/figures/citations usuelles, concentration. Pas nécessaire d'avoir tout LanguageTool ou tous les objectifs pour écrire correctement. |
| **IA premier rang, étroite — major** | N31/N33, étapes 4/6 ; architecture §15.18 et étude IA R0. | Fournisseur entreprise/local existant via Go, aperçu des envois, coût/jetons/latence connus ou déclarés inconnus, corriger/reformuler → diff humain, chat, index lexical local avec citations puis MCP stdio. Politique partagée seulement après PD-1 ; index protégé par coffre. L'IA est livrée dans le parcours d'écriture, pas sous forme d'un terminal. |
| **Distribution et échanges — major** | Étapes 5/6, L1–5, U32/U33, N38. | Signatures/rotation/mises à jour, migrations/retour, licences, rapport incident, nettoyage, preuves WAN/gros projets. Import DOCX/ODT, export DOCX et HTML à prioriser avec corpus utilisateurs ; pandoc épinglé/confiné si retenu. Impression intégrée ; présentations locales déjà amorcées U10/U28 finies sans détection automatique d'écran. |

### Après v1 — plus-value conditionnée par l'usage mesuré

- **major si promis avant preuve ; sinon report raisonnable — ACP, client MCP sortant, embeddings/reranker et voix** : N31/N33, étude IA §11–12. Dépendent du confinement multiplateforme, du consentement et d'un gain de rappel français mesuré. RAG lexical et propositions couvrent d'abord la recherche/réécriture ; ne pas ajouter des modèles de plusieurs Go par défaut.
- **major pour N22 complète ; report explicite — vrai remplacement de stockage, stockage asynchrone de groupe, catalogue d'extensions** : AD-67/68/61. Nouvelles garanties d'autorité, fraîcheur et disponibilité ; la sauvegarde indépendante locale suffit au secours MVP, pas à une équipe jamais connectée simultanément.
- **minor pour premier usage — multi-projets simultanés, lecteur web/mobile, télécommande et présentation réseau avancée** : AD-01/34, U28, conception p. 34–35. Une fenêtre projet et PDF/HTML de lecture sont d'abord suffisants ; ne pas ouvrir des serveurs LAN avant leur propre modèle d'accès.
- **minor pour premier usage — édition riche, suggestion formelle, brouillons privés complexes, OCR, Mermaid en iframe libre, API 1.2 complète** : NG10, étude editor-features §3.6, architecture §15.14/15.21 et IA §12.3. Valeur réelle mais coût élevé en concurrence, rendu et sécurité. Conserver le texte standard et les points d'extension pour leur ajout.

### Porte commune de sortie et gouvernance

**major — Une seule matrice de livraison.** PROGRESS §Étape 2 conserve de nombreuses cases ouvertes malgré du code existant ; exigences distingue fait/test partiel/en cours et architecture §0.2 distingue cible/en place. Aligner ces états sans confondre code présent, UI branchée et critère démontré. W1–W21 restent des règles de travail/qualité (y compris W9 cas croisés, W11 parcours humains, W16 tests utiles), pas des boutons supplémentaires à livrer ; aucune coche ne dispense des portes de sécurité N7–N16.

Pour chaque tranche : scénario solo et à deux, hors ligne/reconnexion, lecteur et écrivain, fichier fermé/ouvert, erreur disque, redémarrage, navigation clavier/lecteur d'écran, plateforme annoncée ; preuve du budget lorsque la tranche touche le chemin de frappe. Références : exigences préambule/W8/W9/W11/W16/W20, PROGRESS introduction et étapes 2/5, architecture §2/§12/§14. Les intégrations réseau explicitement non validées hors sandbox dans PROGRESS doivent rester signalées comme telles.

Ordre des décisions à faire accepter dans le chantier suivant : **coffre + récupération + plateformes**, **limites de stockage et d'exclusion**, **surface de navigation/diagnostics + raccourcis**, **delta protocole pour commentaires/plugins/IA**, puis **périmètre des API et formats de rendu**. Ce sont des dépendances et des arbitrages identifiés, pas des demandes d'autorisation pour terminer le présent audit. La finalité et les cinq tâches d'écriture restent le critère commun à chaque décision.
