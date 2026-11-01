# Passe de cohérence globale (N36)

| | |
|---|---|
| Date | 2026-11-01 (révision 4 de l’architecture, `spec.md` 1.0.0-brouillon.5) |
| But | relire ensemble tout ce qui a été demandé à la volée (N17–N41, U*, L*), résoudre les contradictions, combler ou différer explicitement les manques, fusionner les doublons, et ramener le tout à la finalité : **des personnes qui aiment écrire, ensemble, sans serveur** |
| Entrées | [audit de cohérence de Codex](reviews/coherence-codex.md) ; revues du tour 2 ([Fable](reviews/03-fable-r2.md) §4, [Codex](reviews/04-codex-r2.md) §4) ; études [encryption](research/encryption.md), [sync-ignore](research/sync-ignore.md), [ai §12](research/ai.md), [editor-features](research/editor-features.md), [plugins](research/plugins.md) ; [design rév. 4](../design/spec.md) ; [exigences](../exigences.md) |
| Règle de tri | à valeur égale, ce qui sert d'abord le parcours **écrire → partager → retrouver → corriger → exporter** passe devant ; une plate-forme (extensions, IA, coffre) se construit sur ce parcours, jamais à sa place |

Une remarque d'audit est **écartée** : Codex propose l'échange DOCX et ODT (gap 5 et phasage v1).
L'utilisatrice l'a **refusé** (N40) : aucune intégration native des formats bureautiques ; un
greffon (`importers`, `exporters`) peut l'offrir. Le reste de cet écart est noté en C21.

## 1. Contradictions et leur résolution

| # | Contradiction | Résolution | Où |
|---|---|---|---|
| C01 | Projet chiffré (N37) contre projet en clair (NG3, §6.2) | NG3 retiré ; coffre par objet, sans montage ; G13 | spec §1.3, §6.10 ; `AD-91` |
| C02 | Chiffrement contre BLAKE3/Bao et révocation | identifiants du protocole **dans** les objets chiffrés, noms physiques opaques ; DEK par appareil, aucune clé de groupe ; un membre retiré garde ce qu'il a reçu | §6.10 EN-1, EN-2, EN-7 |
| C03 | Coffre contre fichiers réels (Typst, LSP, Git, recherche, export) | dossier de travail **en mémoire** (tmpfs sans swap en clair ; disque en mémoire sous macOS) ou sur un disque **chiffré par le système, vérifié** ; sinon compilation refusée pour un projet protégé ; borne de 512 Mio et matérialisation par dépendances ; Git dans le coffre (billy) ; index dans le dossier de travail ; projection lisible durable = choix explicite | §6.10 EN-3–EN-5 |
| C04 | « Annuaire » contre aucun serveur | l'annuaire **est** la liste signée des membres ; annuaire d'entreprise = suggestion, jamais autorité ; clé X25519 dans les ops de membres reportée (PD-3) | §6.10 EN-8, §15.19 |
| C05 | Linux seulement (AD-18) contre N35 | port `internal/sandbox` à trois implémentations ; capacité **refusée** sur une cible non prouvée (INV-13) ; Linux seul = alpha annoncée | §9.11 ; `AD-18`, `AD-94` |
| C06 | « Jamais installé depuis un pair » contre N23 | réception d'un paquet ≠ activation ≠ installation globale ; jamais d'installation globale ; greffon de projet borné ; « aucun code reçu » banni des promesses | §15.10 |
| C07 | N23 et politique IA contre registre de décisions fermé | PD-1 réécrit (encodage, schémas, émission, règle de pli, transition `format`) ; bloqué d'ici l'acceptation | §15.19 |
| C08 | « Remplacer le stockage » (N22) contre copie locale obligatoire | trois intentions : stockage de travail, sauvegarde, miroir ; N22 servie en v1 par la sauvegarde, **non déclarée satisfaite** ; vrai fournisseur reporté | §6.7 |
| C09 | 24 mots contre clé + projet + compteur | phrase = secret seul ; **kit de secours** versionné à part | §6.7 ST-8 |
| C10 | « Pas de téléchargement » de portée variable | table des flux unique (§3.1) : moteurs embarqués ; paquets Typst fournis ; greffons importés sur geste ; modèles d'IA installés hors application ; inférence = requête vers un hôte autorisé, sur geste ; W4 (outillage du dépôt) ne vaut pas pour le produit | spec §3.1, NG6 |
| C11 | ACP « soumis aux capacités » contre processus à pleins droits | aucune garantie prêtée à l'agent ; option indisponible sans bac à sable prouvé | §15.18 ACP-1 |
| C12 | IA et RAG : portée de lecture | portée de session choisie avant récupération, appliquée à chaque requête ; provenance par extrait ; contamination monotone ; `ai: never` et fichiers non synchronisés exclus | §15.18 RG-1–RG-5 |
| C13 | Problèmes : popover, onglet, compteur d'en-tête ou de barre d'état | un service ; **un** compteur, dans la barre d'état ; clic = suivant ; liste dans « Relire » | §15.17 ER-1 |
| C14 | Raccourcis répartis (registre du design, `toolbar/spec.ts`, `editor/format/keymap.ts`), collision de l'export avec le bloc de code, recommandation périmée Ctrl+. dans l'inventaire du design | un registre exécutable unique ; Ctrl+K = lien (code, `keymap.ts:16`), Ctrl+Maj+P = palette (**déjà** dans `ux-principles.md:14`, rien à y corriger), Alt+Entrée = corrections ; export : raccourci libre après audit du registre ; inventaire du design à mettre à jour | §15.13 ; design rév. 5 |
| C15 | « Aucun modèle fourni » contre styles « Rapport », Markdown + CSS, deux emplacements | aucun gabarit livré ; un emplacement (`Documents/Inkwell/Modèles/`) ; CSS pour l'export HTML seulement, décision de l'utilisatrice (`AD-78`) ; le projet d'exemple n'est pas un modèle | §6.8 |
| C16 | Ignorer contre « tout recevoir » (N5) | ignorer = ne pas importer un fichier local, ne pas télécharger un blob ; jamais `.typstdeps/`, polices, charte, ressources référencées ; jamais un retrait chez les autres | §6.9 |
| C17 | Versions : Git vérité ou projection ? | vérité = manifestes de têtes Automerge ; Git = projection go-git reconstructible ; restaurer = nouvelle modification | §6.11 |
| C18 | Plusieurs projets (conception p. 13) contre un projet à la fois | un `Runtime` par projet dès maintenant ; v1 : un projet ouvert, fenêtres auxiliaires (présentation, extensions) distinctes ; fermer un projet arrête sa synchronisation, et c'est dit | `AD-01` (utilisatrice) |
| C19 | « Rien n'est perdu » contre 200 ms au pire | états distincts : en mémoire, durable, projeté, envoyé, reçu ; le message dit l'état connu ; la qualité accompagne chaque tranche (pas en dernier) | design §5.16 ; §10 |
| C20 | Onglets exigés (N19) contre AD-47 | **onglets retenus** (N19) : ouverts ≠ chargés ≠ actif ; G-0 d'abord, barre d'onglets ensuite | §6.6 ; `AD-47` |
| C21 | Pandoc (AD-33), DOCX dans la carte et le phasage de Codex, contre N40 | aucune intégration bureautique native, aucun pandoc livré ; greffons possibles | §1.3 NG1 ; `AD-33` |
| C22 | Écrivain Git maison (AD-13) contre N39 (go-git) | go-git (Go pur) retenu, mesuré ; jamais de binaire `git` | §6.11 HI-5 ; INV-12 |
| C23 | Tâches en WebAssembly contre parité des plates-formes | Tâches redevient TypeScript ; Graphviz porte le chemin WebAssembly | §9.10 |
| C24 | Deux comportements de proposition (greffon : appliquer puis annuler ; IA : relire puis accepter) | une seule règle : relire par morceau ; « appliquer sans relire » réglable par greffon, jamais pour un greffon de projet ni l'IA | §15.5 |
| C25 | UTCP « option exportée » contre MCP interface unique | UTCP écarté de la spec | §15.12 |

## 2. Manques : comblés ou différés, avec raison

| # | Manque | Décision | Où |
|---|---|---|---|
| M01 | Récupérer son travail sans expertise | « Sauvegarder / Restaurer une copie » : sauvegarde locale (liens durs) + destination indépendante (§6.7), date du dernier succès vérifié, restauration **à côté** sans écraser ; kit de secours | §6.7, §10.3 ; MVP (local), v1 (destination) |
| M02 | Appareils, identité, perte de tous les appareils | parcours ajouter / remplacer / retirer un appareil (groupe d'appareils, ADR 0017) ; distinguer restaurer les documents (sauvegarde), l'autorité d'admin (second admin conseillé, ADR 0017) et le droit de déchiffrer (destinataire de secours) ; perte de tout sans second admin ni pair : nouveau projet, dit à l'avance | §6.7, §6.10, §9.4 |
| M03 | Relecture de groupe complète | commentaires ancrés, fils, résolution, mentions (étape 4) ; commentaires des **lecteurs** derrière la mineure du protocole (`AD-14`) ; MVP : lecteurs sans commentaire, annoncé | design ; `AD-14` |
| M04 | Messages différés | « À relire » dérivé des commentaires et de l'Activité, lu localement, dédupliqué à la reconnexion ; aucune livraison immédiate promise sans pair joignable | v1 |
| M05 | Entrer depuis d'autres outils | **N40** : pas de DOCX/ODT natifs ; collage riche (HTML → Typst ou Markdown) dans le cœur ; formats bureautiques par greffon | NG1 ; v1 (collage) |
| M06 | Mises à jour sur tout le cycle | `AD-17` + rotation de la clé de signature, refus du retour arrière, mise à jour hors ligne, données distinctes de l'exécutable | v1 ; ADR 0038 |
| M07 | Accessibilité de la collaboration | script de bout en bout au lecteur d'écran (identifier une personne sans la couleur, atteindre sa modification, commenter, suivre et quitter), annonces groupées et désactivables ; Orca, puis VoiceOver et NVDA | design rév. 5 ; v1 |
| M08 | Migration compréhensible | avant : espace nécessaire, personnes à mettre à jour ; après : ouvrir en lecture seule une copie si trop récent | §6.4 ; v1 |
| M09 | Désinstaller | projets et archives gardés ; écran de nettoyage volontaire (caches, index, préférences, clés) avec avertissement sur les clés | v1 |
| M10 | Très grands projets | budgets cumulés (§2 budget global, §12.5) ; priorité : texte actif, puis dépendances, puis arrière-plan ; import partiel sans casser les références | §12.5 |
| M11 | Réglages entre appareils | export et import d'un profil sans secrets ; synchronisation personnelle **différée** | après v1 |
| M12 | Lire sans installer | PDF et HTML accessibles ; lien web vivant **différé** (AD-34) | après v1 |
| M13 | Gestionnaire de tâches (N32) | vue « Ressources », superviseur du cœur | §11.1 ; `AD-88` |
| M14 | RAG (N33) | index lexical local d'abord, expérimentation | §15.18 ; `AD-89` |
| M15 | Fichiers non synchronisés (N34) | couche personnelle d'abord, règle de projet après PD-1 | §6.9 ; `AD-90` |
| M16 | Six plates-formes (N35) | port d'enfermement, portes par OS | §9.11 |
| M17 | Projet chiffré (N37) | coffre par objet, expérimentation | §6.10 |
| M18 | Performance mesurée (N38) | table des budgets, règles de concurrence, PGO, CI | §12.5 |
| M19 | Historique sans Git (N39) | voyager, restaurer, comparer, versions, projection go-git | §6.11 |
| M20 | Variantes et intégrations (N41) | vocabulaire d'écriture, sur les variantes existantes | §6.11 HI-6 |
| M21 | Impression | MVP : PDF ouvert dans le lecteur du système ; v1 : impression intégrée | design §5.12 |
| M22 | Accueil et aide | guide embarqué unique (créer, partager, récupérer, hors ligne, protéger) ; pas de seconde « plateforme d'accueil » | design |

## 3. Doublons fusionnés

| Doublon | Fusion |
|---|---|
| Registre de commandes du design, registre des contributions, outils d'IA et de MCP, raccourcis répartis dans le code | **un** registre de contributions ; palette, menus, aide, IA, MCP en sont des projections filtrées ; les interfaces WIT ne créent pas un second catalogue |
| Diagnostics, relecture, problèmes du tableur | **un** modèle de diagnostic (source, gravité, cible ancrée, révision, action) ; vues « Erreurs » et « Relecture » filtrées |
| Commentaires, Activité, Historique, Relecture, Problèmes | **un** tiroir « Relire » à sous-vues : Problèmes, Commentaires, Activité, Versions ; même donnée d'événements, même navigation |
| Recherche humaine (AD-42), outils de l'IA (`index.*`), Sommaire, liens | **un** service de navigation `ouvrir(cible, révision)` ; la recherche exacte et les expressions régulières gardent leur sens ; l'IA ajoute le classement |
| Sauvegarde, miroir, modèle, archive, version | quatre intentions nommées : **restaurer un état** (version), **récupérer après une perte** (sauvegarde), **transmettre** (archive), **commencer autre chose** (modèle) ; un seul mécanisme d'instantané ; jamais tout appelé « Exporter » |
| « Version nommée » et « jalon » ; « brouillon » et « variante » ; « fusionner » | interface : **version**, **variante**, **intégrer** ; glossaire mis à jour |
| « Extension », « greffon », `.inkplug`, `.inkwell-plugin` | interface : **extension** ; code : plugin ; un format, `.inkwell-plugin` ; portée et provenance sont des attributs |
| « En cours » (design §4.40), Inspecteur, journaux, coûts d'IA | **une** vue Ressources (§11.1) ; l'Inspecteur est son détail en mode développeur ; les compteurs d'IA y sont agrégés |
| Dialogues recopiés, sept bandeaux, trois toasts | un composant de dialogue, un hôte de bandeaux à priorité, un hôte de toasts |
| Doublons du registre des exigences (N30/N31 sur une ligne, U34 deux fois) | identifiants gardés, suffixes stables ajoutés par la personne qui tient le registre ; traçabilité par identifiant |

## 4. Carte produit : chaque fonction à sa place

Principe : **calme, l'écriture d'abord**. À l'ouverture : Fichiers à gauche, le texte, l'aperçu
si la fenêtre est assez large ; **aucun tiroir ouvert** ; la barre de mise en forme se masque.
Trois accès à toute action : menu, raccourci, palette. Rien d'affiché pour une fonction absente
ou non installée.

| Surface | Contenu | Surcharge constatée → simplification |
|---|---|---|
| **Accueil** | nom et couleur au premier usage ; projets récents ; Nouveau (Typst, Markdown, texte, tableau ; depuis un modèle de la personne) ; Ouvrir ; Rejoindre ; Restaurer une copie | — ; aucun modèle préinstallé |
| **Barre d'en-tête** | projet et fichier ; état de synchronisation et d'enregistrement ; personnes ; Partager ; menu de l'application ; bouton « Extensions » (pièce de puzzle) **seulement** si une extension tierce ou de projet existe ; actions **épinglées par la personne** ; indicateur « IA : envoi vers X » seulement pendant une session d'IA | compteur de problèmes, IA, puzzle, actions s'ajoutaient → le compteur va à la barre d'état ; l'IA n'apparaît qu'activée ; ≤ 6 actions épinglées |
| **Onglets et document** | fichiers ouverts en onglets (N19) ; éditeur du type (Typst, Markdown, tableau, texte, code coloré, image, PDF, carte) ; barre contextuelle ; menu Insérer (image, tableau, note, citation, renvoi, symbole, diagramme) ; complétion, signature, erreurs au point d'écriture | — |
| **Gauche** | deux onglets : **Fichiers** (avec corbeille et marque « non synchronisé ») et **Sommaire** ; Tâches = filtre du Sommaire | quatre onglets (Fichiers, Sommaire, Liens, Tâches) + extensions dans 264 px → deux onglets ; Liens dans les détails d'un fichier |
| **Aperçu** | pages ; « Vue lecture » accessible ; saut source ↔ aperçu ; zoom ; Exporter ; Présenter | aperçu + tiroir + gauche compriment le texte → une seule zone secondaire à la fois sous une largeur donnée, retour garanti au texte |
| **Tiroir (droite)** | trois destinations : **Rechercher**, **Relire** (Problèmes, Commentaires, Activité, Versions et « voyager dans le temps »), **IA** (si activée) ; panneaux d'extensions comme sous-vues ou quatrième destination « Extensions » | jusqu'à huit vues → trois destinations ; Ressources et Inspecteur sortent du tiroir |
| **Barre d'état** | problèmes (compteur, clic = suivant) ; non enregistré ; position et mots ; page (avec l'aperçu) ; indicateur de pause des ressources ; le reste sous « ⋯ » (zoom, langue, encodage, fin de ligne, mise en page) | treize rubriques → quatre visibles |
| **Préférences** | Écriture (éditeur, orthographe et dictionnaires, extraits, typographie, objectifs, concentration, raccourcis) ; Apparence et moi (thème, taille, nom, couleur, appareils) ; Projet (principal, rôles, suppression, verrous, sauvegarde, non synchronisés, protection, règles partagées) ; IA (profils, limites, MCP, index) ; Extensions ; Application (langue, mises à jour, réseau, confidentialité, nettoyage, mode développeur) | chaque ligne dit « ce projet », « cet appareil » ou « mes appareils » |
| **Vue Ressources** | §11.1 ; ouverte par le menu, la palette, le menu Extensions | — |
| **Dialogues sur geste** | Partager, Rejoindre, empreinte ; Personnes ; Exporter (copie lisible, archive complète, chiffrée ou non) ; Imprimer ; Importer ; Enregistrer comme modèle ; droits d'une extension ; aperçu d'envoi à l'IA ; Déverrouiller ; Restaurer ; confirmation d'un effet irréversible ; Aide, À propos, rapport local | un seul composant de dialogue |

## 5. Phasage, sur les étapes de `PROGRESS.md`

Chaque lot livre un usage complet ; qualité, clavier et reprise accompagnent chaque lot (pas en
dernier). Une alpha Linux ne clôt ni N35, ni N37, ni L5.

### MVP (alpha utile) — terminer les étapes 1 à 3, avec le minimum des étapes 5 et 6

| Lot | Exigences | Valeur | Risque traité | Porte |
|---|---|---|---|---|
| Mesure de référence (N38) | `PERF-TYPE`, `PERF-SYNC-*`, `PERF-START`, `PERF-MEM`, bancs Go de `HOT_PATHS:`, corpus de §12.5 PM-1 | sans elle, aucun budget n'est un seuil | budgets restés des souhaits | résultats bruts archivés, seuils fixés (PM-5) |
| Écrire et retrouver | G-0a puis G-0b ; **onglets (N19) au lot G-0c, qui dépend du Worker** (ADR 0028 D2) : G-0a en lecture seule ne satisfait ni N19 ni N20 ; N20, U1, U3–U8, U13, U14, U24–U26 | ouvrir vraiment plusieurs fichiers, créer, renommer, corbeille, txt, md, typ, csv | parcours principal cassé aujourd'hui | FM-OPEN-1…8, export du bon projet après redémarrage |
| Collaborer sans perdre | N1–N16, N-1, U7, U32–U34 ; migration ADR 0034 et admission ADR 0031 | inviter, retirer, hors ligne puis converger | sécurité byzantine avant exposition | P-* d'ADR 0034, deux appareils réels, disque plein, plantage |
| Comprendre, corriger, produire | N28, N29 (socle), U4, U9, U15, U17, U18, L1 | erreurs cliquables, F8, complétion et survol de la charte, aperçu et PDF hors ligne | tinymist non branché | `problems.spec.ts`, `PERF-LSP` |
| Première minute et secours | N17, N19, U31, U32 ; sauvegarde locale et restauration à côté ; aide embarquée ; fr et en | ne jamais avoir peur de perdre | confiance | tests en personne, Orca |
| Revenir en arrière | N39 (voyager, restaurer, comparer, versions nommées), **après** les épingles des versions dans le ménage (HI-9) et la sémantique d'« Intégrer » (HI-6) | se tromper et revenir sans Git | perte perçue | e2e Historique |
| Fondations extensibles | registre unique, extensions intégrées TypeScript (N21, N24 socle) | ajouter sans « verrue » | réécriture plus tard | P-EXT-1, P-EXT-2 |

### v1 (première version générale) — finir les étapes 3 et 4, industrialiser 5 et 6

| Lot | Exigences | Pourquoi maintenant |
|---|---|---|
| Six plates-formes | N35, L1–L5 | promesse finale ; portes P-SANDBOX-* ; aperçu sur chaque cible |
| Projet protégé | N37 | données stabilisées avant la 1.0 (format figé, ADR 0025) ; P-VAULT |
| Relire ensemble | étape 4 : commentaires, mentions, Activité ; N4, N6 ; U11, U12, U16, U27, U29, U30 | valeur directe pour la relecture de groupe ; schéma avant interface |
| Variantes | N41 | sur les variantes déjà présentes |
| Extensions utilisables | N21–N26, N32 ; API 1.0 ; Graphviz ; N23 après PD-1 | après P-PLUG-PROTO et P-PLUG-BENCH ; vue Ressources dès le premier greffon tiers |
| Modèles et écriture | N27, N30 ; N34 (couche personnelle des fichiers non synchronisés ; couche du projet après PD-1) | modèles de la personne, orthographe, références, concentration |
| IA, étroite | N31, N33 (lexical) | fournisseur de l'entreprise par Go, aperçu d'envoi, propositions par morceau, discussion, index lexical, MCP stdio ; politique partagée après PD-1 |
| Distribution | étapes 5 et 6, N38, M06, M08, M09 | signatures, mises à jour, migrations, nettoyage, preuves WAN et gros projets |

### Après v1 — selon l'usage mesuré

ACP, client MCP sortant, vecteurs et reclassement (N33 R1, R2), voix ; vrai fournisseur de
stockage et stockage de groupe (N22 complète, `AD-68`) ; catalogue d'extensions (`AD-61`) ;
plusieurs projets ouverts (`AD-01` option 2), lecteur web, télécommande ; mode Suggestion,
brouillons privés, intégration entre projets ; Mermaid complet (P-FRAME) ; API 1.2.

## 6. Ce que le design doit changer (révision 5)

1. **Onglets** de documents (N19) : barre d'onglets, ouverts ≠ chargés, un éditeur visible ;
   comportement à la fermeture d'un onglet non enregistré (aucune question, SAV-03).
2. **Tiroir à trois destinations** : Rechercher, **Relire** (Problèmes, Commentaires, Activité,
   Versions), IA ; retirer Inspecteur et « En cours » du tiroir.
3. **Panneau de gauche à deux onglets** (Fichiers, Sommaire) ; Liens dans les détails d'un
   fichier ; Tâches en filtre du Sommaire ; marque « non synchronisé » (nuage barré, info-bulle).
4. **Problèmes** : un seul compteur, dans la barre d'état ; clic = suivant ; liste dans Relire ;
   aucun compteur dans l'en-tête (C13).
5. **Barre d'état** : quatre éléments visibles, le reste sous « ⋯ ».
6. **Raccourcis** : Ctrl+K lien, Ctrl+Maj+P palette (déjà dans `ux-principles.md`), Alt+Entrée
   corrections ; raccourci d'export attribué après audit du registre ; retirer la recommandation
   périmée Ctrl+. de l'inventaire ; menus et aide générés depuis le registre.
7. **Historique sans Git** : vue « voyager dans le temps » (curseur temporel, Revenir au présent),
   « Restaurer cette version » avec aperçu et Annuler, « Comparer » par mots ; vocabulaire
   **version**, **variante**, **proposer d'intégrer**, **intégrer**, **abandonner la variante** ;
   jamais branche, fusion, commit.
8. **Variantes** (N41) : créer, comparer avec l'original, proposer d'intégrer, intégrer, divergence
   affichée ; « Dupliquer le projet ».
9. **Protection du projet** (N37) : case « Protéger ce projet » (cochée par défaut pour un projet
   partagé), message « Pour compiler un projet protégé ici, activez le chiffrement du disque » quand
   la cible l'exige (§6.10 EN-4), message de la borne de 512 Mio, déverrouillage, phrase et **kit de secours** (impression, QR), « les données sont
   perdues sans eux » dit à l'activation, export chiffré ou lisible.
10. **Sauvegarde et restauration** : « Sauvegarder / Restaurer une copie », date du dernier succès,
    restauration à côté, avertissement de fraîcheur d'un appareil neuf (§6.7 ST-7).
11. **Vue Ressources** (N32) : tableau façon `about:processes`, verbes annuler, mettre en pause,
    arrêter, désactiver ; limites réglables à la baisse ; noms sans jargon.
12. **Fichiers non synchronisés** (N34) : « Ne pas synchroniser » et « Pour tout le projet » (admin),
    aperçu « 12 fichiers concernés », phrase « ne retire pas les copies déjà reçues ».
13. **IA** : indicateur « IA : envoi vers X » ; aperçu d'envoi à chaque nouvelle provenance ;
    consommation (jetons, estimation marquée, coût inconnu) ; diff par morceau ; marques « marqué
    IA par Camille » ; jeton de lancement par client MCP, nom « se présente comme ».
14. **Propositions** : une seule présentation (diff par morceau) pour extensions, IA, MCP ;
    « appliquer sans relire » réglable par extension.
15. **Modèles** : bibliothèque visible dans `Documents/Inkwell/Modèles/`, aucun modèle fourni, pas
    de « Rapport »/« Lettre » proposés comme gabarits ; « Enregistrer comme modèle » avec choix des
    fichiers ; CSS présenté comme réglage de l'export HTML.
16. **États honnêtes** (C19) : messages fondés sur l'état connu (durable, envoyé, reçu) ; aucun
    « rien n'est perdu » au-delà de ce qui est acquitté.
17. **Extensions** : un seul gestionnaire (portée, provenance, droits) ; bouton puzzle ; actions
    épinglées par la personne ; « Redémarrer sans extensions » ; badge « Extension du projet ».
18. **Plates-formes** : contrôles natifs de chaque OS (menus, raccourcis Cmd sur macOS, tube nommé
    invisible), et l'écran « aperçu indisponible sur ce système » tant qu'une porte manque.
19. **Formats bureautiques** : aucune entrée Word/ODT/Google Docs dans les menus (N40) ; collage
    riche oui.
20. **Glossaire** : ajouter version, variante, intégrer, protéger, kit de secours, ressources,
    non synchronisé ; retirer jalon, brouillon (v1), fusionner.
