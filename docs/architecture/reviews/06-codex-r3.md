# Revue d’architecture — Codex, tour 3

2026-11-01 — `spec.md` **1.0.0-brouillon.4**, protocole accepté **2.0.0-brouillon.10**.
Revue statique des contrats avec vérifications dans le code ; aucun résultat de prototype,
benchmark ou confinement n’est revendiqué. Seul ce rapport est créé ; aucun commit.
**`05-fable-r3.md` n’a pas été lu.**

Référentiel : exigences N1–N41, CLAUDE.md, principes d’ingénierie §§2–4, protocole accepté,
revues `04-codex-r2.md`, `03-fable-r2.md` et `coherence-codex.md`. Corpus d’architecture :
spec, coherence, décisions, traçabilité, CHANGES, README et études plugins, editor-features,
ai, encryption, sync-ignore. Les études ne sont pas des normes concurrentes. Les références
par section désignent la révision examinée. Sources primaires externes revérifiées :
age/C2SP, noyau Linux, Microsoft, Automerge, beautiful-mermaid et W3C, liées ci-dessous.

Gravités : **bloquant** = contradiction d’une garantie centrale ou porte d’acceptation
inapte à la vérifier ; **majeur** = contrat important incomplet ; **mineur** = correction
locale. « Fermé au contrat » ne signifie jamais livré.

## 1. Vérification des constats du tour 2

La demande compte sept blocages Codex : R2-1 à R2-6 étaient explicitement bloquants ; R2-10
était « majeur, porte explicite d’acceptation ». Je vérifie aussi ce septième sans réécrire
sa gravité historique. Les quatre blocages Fable sont vérifiés séparément. Les règles et
gravités d’origine sont celles des deux rapports du tour 2.

| Constat | Preuve dans la révision 3 | État |
|---|---|---|
| Codex R2-1 : confinement Linux | H-0 : fil fixé, Landlock, ré-exécution statique puis seccomp TSYNC ; H-5 : architecture, x32, clone filtré, clone3 ENOSYS ; matrice des noyaux et enfant natif hostile | **Fermé comme contrat expérimental**, pas comme confinement prouvé. Liste seccomp encore à produire ; §0.5 interdit la dépendance de production. |
| Codex R2-2 : autorité et IPC | IPC-1–6 : SOCK_STREAM, endian, types, crédits d’octets, annulation ; H-4 : un plugin par connexion/processus | **Fermé au contrat** : l’autorité d’un autre colocataire n’est plus disponible après évasion native. |
| Codex R2-3 : ABI | ABI-3 host_read sans réentrance ; ABI-4 imports/horloge ; ABI-7 octets signés, tri, exclusions et chemins ; ABI-8/9 vecteurs et chaînes à produire | **Fermé comme expérimentation** Rust/Go ; report TinyGo accepté. Reste local §15.5 : R3-m1. |
| Codex R2-4 : protocole | PD-1 : CBOR base64url dans str16, schémas, opérations séparées, transition des anciens en lecture seule ; PD-2 retiré | **Fermé quant à la séparation des contrats**. Aucune approbation de 2.1 donnée ici ; boucle protocole obligatoire. |
| Codex R2-5 : récupération | ST-3/4/6 même hash keyed complet ; ST-8 phrase = secret, kit distinct ; ST-7 compteurs par projet/appareil ; aucun ménage distant | **Partiel** : arithmétique et GC fermés ; découverte phrase seule et framing cryptographique : R3-M1. |
| Codex R2-6 : baseline et nouvelles exigences | NG1/N40 ; G10/INV-13/§9.11 ; G13/§6.10 ; INV-12/§6.11 | DOCX/ODT et go-git **fermés au choix** ; parité non réputée livrée ; coffre contradictoire : B1/B2. |
| Codex R2-10 : preuve avant acceptation | §0.5 retire l’acceptation de l’hôte et interdit les dépendances de production ; annexe conserve témoin/trajet complet | **Fermé par rétrogradation explicite**, option offerte dans R2-10 ; portes encore à corriger : B4. Aucun banc réputé passé. |
| Fable R2-B1 : threads, syscalls, SEQPACKET | H-0/H-5 et IPC-1 ; SCM_RIGHTS autorisé une fois pour les modules, autres descripteurs refusés | **Fermé comme contrat expérimental** : ré-exécution et stream répondent aux défauts précis. |
| Fable R2-B2 : imports et horloge | ABI-4 ajoute poll_oneoff, sched_yield, preopens refusés, horloge non figée | **Fermé dans ABI-4**, sous P-PLUG-PROTO ; contradiction locale §15.5 : m1. |
| Fable R2-B3 : pli/rendus | PD-1 §4 : passé causal/format ; JSONL borné, empreinte des polices (§15.14) | **Fermé au contrat de proposition**. JSONL tolère une ligne corrompue, sans prouver que toute insertion concurrente conserve deux lignes : tester cette dégradation. |
| Fable R2-B4 : phrase impossible | ST-8 sépare phrase et kit | **Fermé pour les 24 mots** ; parcours phrase seule : M1. |

### Majeurs du tour 2

| Constat | Preuve et conclusion |
|---|---|
| Codex R2-7 : session/manuels/diagnostics | MF-12/20/21 séparent état sale et poignée évictable, comptent l’historique ; G-0a dit lecture seule ; LSP-3 garde instantané/requête et neutralise diagnostics périmés. **Fermé au contrat** ; FM-OPEN-7 doit suivre la nouvelle règle. |
| Codex R2-8 : CSV observateur tiers | MF-25 exige toutes les paires concurrentes, C observateur, rattrapage/réouverture, calcul déterministe et réparation choisie. **Fermé au contrat** ; oracle à implémenter dans `frontend/src/editor/csv/collab.test.ts`. |
| Codex R2-9 : IA/MCP | §15.18 : limites, compteurs/estimations, coût inconnu, consentement renouvelé, contamination monotone, jeton client/projet, MCP partagé désactivé avant PD-1, ACP sans garantie implicite. **Fermé pour ces défauts** ; intersection des portées RAG : M5. |
| Codex R2-11 : hooks N32–N39 | §11.1, §6.9–6.11, §9.11, RG-1–5 et traçabilité ajoutés. **Fermé pour l’intégration**, pas pour leur suffisance : constats ci-dessous. |
| Codex R2-12 : modèles | TM-2 exclut secrets/caches/état personnel ; TM-3 valeurs typées/échappées. **Fermé au contrat**. |
| Fable R2-M1 : six plateformes | Port, matrice, refus explicite §9.11 ; alpha distinguée de v1. **Fermé pour le cadrage**, pas preuve de parité ; B4. |
| Fable R2-M2 : budget système | §2 : six hôtes, RSS au repos, somme linéaire, budget global. **Fermé pour l’existence du budget**, interprétation/mesure : M4. |
| Fable R2-M3 : consentement continu | Premier envoi puis nouvelle provenance/fichier/destination, indicateur permanent, interruption sur changement de politique/profil. **Fermé au contrat**. |

Les majeurs hérités du premier tour (mandat/révocation, règle de flux, API étagée,
accessibilité de l’aperçu) conservent leurs remèdes. La Vue lecture dispose d’un repli
plan + source : bon secours, pas automatiquement un équivalent sémantique pour tableaux,
mathématiques et figures. Référentiel : N17, WCAG 2.2 SC 1.1.1/1.3.1 ; gravité **majeure
à la livraison** si ces parcours échouent, pas nouveau blocage sur la seule absence de code.

## 2. Débat D-10 à D-16

| Débat | Position et justification |
|---|---|
| **D-10 : nombre d’hôtes** | **Un hôte par plugin actif**, paresseux, plafond six. Une classe O0/O4 ne représente ni les mêmes lectures, ni les mêmes mandats/consentements. Après compromission native, le parent authentifie une connexion, pas son auteur interne (IPC-3 ; moindre privilège, principes §4). Mesurer six hôtes puis 16 plugins installés avec admission/éviction explicites ; pas 16 actifs (B4). **Enjeu bloquant fermé au contrat** ; coût expérimental. |
| **D-11 : Landlock** | **Démarrage en deux temps accepté**, binaire statique séparé et seccomp TSYNC principal. La [documentation du noyau](https://docs.kernel.org/userspace-api/landlock.html#previous-limitations) confirme portée par fil avant ABI 8 et héritage aux enfants ; exec depuis le fil restreint règle les threads préexistants. Un wrapper setpriv reste possible seulement s’il est présent/compatible sur les distributions publiées. Tester croissance du runtime et ABI sondée. **Enjeu bloquant passé en expérimentation**, pas preuve acquise. |
| **D-12 : TinyGo** | **Je rejoins API 1.1.** Ma demande précédente de trois profils immédiats était trop large une fois TinyGo retiré de la promesse 1.0. Rust/Go éprouvent déjà deux chaînes ; ABI-9 conditionne correctement TinyGo à ses imports réels. Sleep(0) ne prouve aucune attente : ajouter timer positif et annulation. N25/N26, ABI-4/9 ; **majeur fermé par réduction explicite de périmètre**. |
| **D-13 : UTCP** | **Retirer**, comme proposé : §15.12/15.18 gardent une condition de réexamen sans second catalogue. Registre commun et MCP couvrent l’usage défini ; N31 n’impose pas UTCP. Principe §2 ; **mineur, fermé**. |
| **D-14 : preuves** | **Oui à une architecture contenant des expériences non acceptées**, sans dépendance de production et avec issue d’échec écrite. C’est l’alternative offerte dans R2-10. Ce statut ne ferme pas N21/N35/N37 et ne répare pas une porte contradictoire (B4). N18, principes « chaque principe a sa preuve » ; **enjeu bloquant**, dispositif sain sous correction des portes. |
| **D-15 : Mermaid** | **Oui au sous-ensemble beautiful-mermaid sans DOM en 1.1**, derrière la porte du renderer WASM, **sans P-FRAME**. Le [README primaire](https://github.com/lukilabs/beautiful-mermaid#features) confirme SVG synchrone et absence de DOM ; il ne prouve pas compilation du bundle/ELK en Javy, imports ou coûts. Exiger module épinglé, imports permis, polices reproductibles, corpus supporté/refusé, SVG assaini, annulation et budget §15.14.6 sur six cibles. La taille seule ne suffit pas. Mermaid complet à DOM garde P-FRAME. N7/N24/N38 ; **majeur conditionnel**. |
| **D-16 : signatures** | **Préfixes externes acceptés**, PD-2 retiré. Protocole §3.3 : `inkwell/<domaine>/v1\0` ; `inkwell-ext:` est disjoint. Ne pas modifier §20.4 pour paquets/exports locaux. ABI-7 définit le contenu signé ; ST-5 reste à cadrer (M1). N12/N14, séparation des domaines ; **ancien blocage fermé pour le choix**. |

## 3. Portes expérimentales (§0.5)

Le principe est **sain**, particulièrement l’interdiction de dépendance de production.
Chaque porte doit toutefois avoir entrées figées, oracle de réussite, seuil, matrice,
résultats reproductibles et issue d’échec. Un nom de test ne remplace pas ces éléments.
Référentiel : N18/N38 et principes d’ingénierie introduction/§3 ; insuffisance **bloquante B4**.

| Porte | Acquis | Complément nécessaire |
|---|---|---|
| P-PLUG-PROTO | Deux SDK, paquet signé, proposition/Annuler, vecteurs ABI-8 | Imports réellement liés ; allocation/overflow/erreurs ; timer positif et annulation pendant attente ; révocation ; artefacts et commandes exactes. |
| P-PLUG-BENCH | Témoin, trajet complet, percentiles, tailles, deux architectures | Résoudre 16 concurrents contre six actifs ; tailles des modules, froid/chaud, cumul mémoire ; mesure du coût complet. |
| PLUG-COMPROMISED / P-SANDBOX-* | Enfant **natif** hostile, six cibles, paquet/archive | Corpus commun, chemins canaris normalement accessibles, poignées héritées, deux hôtes, quotas/arrêt sous saturation. Ajouter sidecars : lecture bornée, sortie disque cumulée, réseau et mort ; un test de plugin sans fichiers ne suffit pas. |
| P-VAULT | Crash à chaque étape, six cibles, budgets relatifs | B1/B2 ; inspection hors ligne avant relance après crash ; verrouillage, export, migration depuis clair, récupération sans ancien appareil, disque plein, troncature/tampering, index/WAL/temporaires ; oracle de non-perte. |
| P-IDX | Budget 50 ms/100 Ko, initialisation, RAM/CPU, citations | Corpus français versionné, questions/passages attendus, **seuil rappel@k fixé avant essai** ; zéro résultat interdit ; retard borné ; corruption/reconstruction ; six cibles. « Mesurer le rappel » n’est pas un seuil. |
| P-FRAME | Processus indépendant, canari réseau/navigation, boucle infinie | Le texte parle encore d’un prototype WebKitGTK : moteurs macOS/Windows à prouver avant parité, ou refus explicite sur ces cibles. |
| ACP | Refus sans sandbox prouvé | Profil propre : un plugin sans réseau n’est pas un agent devant joindre son fournisseur. Destination/proxy, fichiers, processus/sous-outils autorisés et tests d’évasion. |
| PD-* | Boucle séparée | Conserver : aucune acceptation architecture ne vaut approbation des octets/du pli 2.1. |

### R3-B4 — Bloquant : portes incohérentes ou non discriminantes

**Preuves :** H-4 refuse un septième hôte actif, mais l’annexe exige « 16 greffons en
concurrence » sans modèle de multiplexage ; P-IDX/AD-89 ne fixent aucun seuil de rappel ;
P-SANDBOX-* ne spécifie pas de preuve du budget cumulé des sorties sidecars ; P-VAULT
cherche le clair seulement après fermeture ; ACP renvoie à une preuve sans profil propre.
Ces portes peuvent être impossibles ou réussir sans vérifier leur garantie.

**Fermeture :** rendre normatifs les compléments du tableau. On peut éprouver 16 plugins
**installés**, six actifs et dix en attente, avec refus/éviction et équité ; ne pas relever
le plafond pour satisfaire une phrase périmée. En cas d’échec : maintien expérimental/refus,
autre option ou nouvelle revue, jamais seuil assoupli après mesure. Je n’exige pas de passer
ces portes pour accepter les seules parties stables de l’architecture.

## 4. Défauts des nouveaux contrats

### R3-B1 — Bloquant : le dossier privé en clair ne tient pas la garantie du coffre

**Règles :** N37, §9.2 « Voleur », EN-5/7, principes §4 et ADR 0029.
**Preuves :** EN-4 écrit sur disque ordinaire sous macOS et dans `%LOCALAPPDATA%` sous
Windows. Exclusions Spotlight/Time Machine et ACL ne chiffrent pas ces fichiers. Une
coupure, un crash ou un vol pendant l’usage laisse ce clair avant le balayage au prochain
lancement. EN-7 retire session compromise, hibernation et effacement physique SSD, mais
promet encore protéger l’appareil éteint/volé. Même sous Linux, vérifier le swap du tmpfs.
C’est une contradiction du contrat, pas une faille observée dans un coffre déjà implémenté.

P-VAULT peut réussir « aucun clair après fermeture » tout en ratant ce scénario. L’absence
reconnue d’effacement SSD garanti ne justifie pas une copie **encore directement lisible**
dans le dossier de travail après panne.

**Fermeture :** matérialiser sur un support dont la persistance est chiffrée avec une clé
indisponible au voleur, ou une voie mémoire prouvée ; sinon refuser ces fonctions pour un
projet protégé sur la cible. Si le chiffrement intégral du système devient une dépendance,
en faire une condition de garantie, pas « toujours recommandé ». Tester arrêt brutal et
inspection hors ligne **avant relance**, sur six cibles. Coherence C03 reste partielle.

### R3-B2 — Bloquant : une DEK de 256 bits n’est pas une clé d’en-tête age

**Règles :** N15/N37, réutiliser les standards sans format cryptographique ambigu.
**Preuves :** EN-2/AD-91 annoncent une DEK de 256 bits enveloppée au « format d’en-tête age ».
Le [standard C2SP, File key et Header](https://c2sp.org/age#file-key) fixe une clé de fichier
aléatoire de **16 octets**, enveloppée par les stanzas ; il ne définit pas une enveloppe
arbitraire de DEK de 32 octets. L’étude encryption §1.1 connaît cette taille. Citer le
standard ne définit donc pas ici un format interopérable suffisant.

**Fermeture recommandée :** chiffrer la DEK de 32 octets comme **payload d’un fichier age
complet**, par la bibliothèque age, dans un objet versionné lié au projet/appareil ; ne pas
modifier les stanzas X25519. Vecteurs récupération, plusieurs destinataires, remplacement
d’enveloppe et refus d’un objet d’un autre projet à P-VAULT. Conserver la DEK par appareil
et l’absence de clé de groupe, qui simplifient correctement le modèle.

### R3-B3 — Bloquant : l’historique promet de restaurer des objets que le GC efface

**Règles :** N39 « sans rien perdre », HI-1/2/3/5, INV-3, ADR 0029.
**Preuves :** HI-2 promet que blobs/dépendances reviennent avec la version ; HI-3 les inclut
au manifeste. §6.5 ne donne comme racines du ménage que l’index admis et les décisions plugin
debout, sans pin des versions conservées. `internal/blob/gc.go:34–38,53–67` supprime après
TTL ce que le prédicat ne référence plus. Exemple : V1 avec logo A ; remplacer A par B ;
attendre 31 jours sans autre référence à A. Le contrat autorise sa suppression puis promet
sa restauration avec V1. Le Git reconstructible ne peut devenir sa sauvegarde autoritative.

**Fermeture :** racines des versions/variantes conservées, dépendances et rendus compris ;
pins acquis avant publication du manifeste, libération par rétention compatible avec
purge/finalité. Définir comment annoncer les versions volontairement incomplètes après une
purge irréversible. Tester V1/remplacement, GC après TTL, restauration hors ligne après
redémarrage et reconstruction Git. Un pin d’historique ne doit pas ressusciter une purge.

### R3-M1 — Majeur, résiduel R2-5 : récupération personnelle encore sous-spécifiée

**Règles :** ST-3–8, N18, ADR 0029 ; AEAD explicitement cadré en ST-4.
**Preuves :** phrase seule doit « lister les projets trouvés sur le dorsal », mais le kit
seul porte ProjectID/adresse. K_meta/K_nom dépendent de ProjectID. ST-5 nomme
`heads/<PeerID>/<counter>` et `manifests/<PeerID>/<counter>` sans namespace projet explicite :
deux projets d’un appareil peuvent revendiquer le même nom si le compteur est par projet.
ST-5 donne les AAD mais pas l’enveloppe nonce/algorithme de son AEAD comme ST-4 le fait.

**Fermeture :** namespace projet et localisateur récupérable sans déchiffrement, ou kit
obligatoire avec repli honnête. Pour phrase seule, demander l’adresse et spécifier découverte
des identifiants non secrets. Définir framing/version/nonce, validation de signature de tête,
et test deux projets/deux appareils. Aucun nouveau protocole réseau nécessaire.

### R3-M2 — Majeur : « intégrer = une modification Automerge » ne décrit pas le mécanisme

**Règles :** N7/N39/N41, protocole §§4.6/4.7/9.4/9.7 (ancre, auteur, couverture), HI-6/8.
**Preuves :** HI-6/AD-96 qualifient la fusion native de « une modification, annulable,
attribuée ». Une [merge Automerge](https://automerge.org/automerge/api-docs/js/functions/merge.html)
importe les modifications existantes, sans les réécrire en une modification du signataire
qui accepte. `frontend/src/history/versions.ts:234–271` écrit une version, appelle
`main.merge`, puis change l’état ; avec `skip`, il applique plutôt le texte net par
`integrate`. Le fork actuel (`:170–183`) importe un document sans montrer son raccord à
l’annonce signée/ancre. Ce code n’est pas la preuve du nouveau contrat.

**Fermeture :** choisir soit une proposition de différence appliquée en nouvelles
modifications autorisées par document, liée à la variante, soit un groupe d’intégration
native conservant auteurs/tuples, avec couverture, ancre, annulation et réintégration
explicites. Cas : variante modifiée pendant revue, original modifié, zone verrouillée,
auteur retiré, intégration répétée, annulation après frappe et divergence signalée.
« Une action annulable » est une bonne promesse UX ; « une modification » n’en est pas
le synonyme technique. HI-7 expose honnêtement le report interprojets ; N41 reste partielle.

### R3-M3 — Majeur : sync-ignore confond transitions de fichiers nouveaux et déjà admis

**Règles :** N5/N34, IG-1/4/5, Nielsen visibilité/prévention des erreurs (N17/N36).
**Preuves :** IG-1 interdit correctement de retirer un document admis ; IG-5 dit pourtant
« réinclure un fichier l’importe comme un nouveau fichier ». Pour un texte déjà admis,
cela pourrait doubler le nœud tandis que l’ancien reçoit toujours les changements. La carte
produit propose « Ne pas synchroniser » sans distinguer cette impossibilité. IG-4 interdit
l’exclusion des ressources référencées ; IG-5 se limite à la signaler. L’étude proposait
rejet par chemin/retrait d’index : son rejet dans la spec est justifié, mais les transitions
restent incomplètes.

**Fermeture :** table absent/local/admis/blob non téléchargé, effets sur import, surveillance,
projection, réception et réinclusion. Réinclure un nœud admis retrouve son identité ; seul
un fichier jamais admis crée un nœud. Dire pour un texte déjà partagé que ses **nouvelles**
modifications continuent à se synchroniser ; « ne retire pas les copies déjà reçues » ne le
dit pas. Définir refus/résolution explicite pour une dépendance du document principal.
Aucun filtre d’admission par chemin ni retrait rétroactif du graphe n’est demandé.

### R3-M4 — Majeur : budgets utiles, protocole de mesure incomplet (N38)

**Règles :** N38, §12.3/12.5, principes §3, W20.
**Preuves :** recherche 10 000 fichiers/indexation 1 000 fichiers sans octets cumulés ;
500 versions sans taux de changement ; débit blob sans taille/cache/implémentation ;
budgets « à fixer par la première mesure », alors que l’annexe interdit de les déplacer
après essai. Les résultats ne seraient pas comparables. §2 cite RSS **et** VSZ sans
séparer les plafonds : réservation virtuelle et mémoire résidente diffèrent. PF-8 invoque
les Long Tasks pour 16 ms ; la [spécification W3C](https://w3c.github.io/longtasks/) rapporte
les tâches d’au moins 50 ms : zéro entrée ne prouve ni 16 ms ni 4 ms.

Le code n’apporte pas ces preuves : `.github/workflows/ci.yml` ne lance pas explicitement
la comparaison à main/matrice complète six cibles ; `scripts/bench.sh` utilise un baseline
local. Cela reste une cible, pas une régression d’une promesse déjà livrée.

**Fermeture :** corpus versionnés (octets, opérations, blobs, churn), machines/OS/moteurs,
froid/chaud, échantillons, frontières des chronos et bruit CI ; plafond RSS/PSS agrégé
séparé de VSZ. Mesurer entrée→frame et gestionnaires, détecter API indisponible, jamais
réussir sans mesure. Fixer les seuils avant comparaison ; leur révision passe en revue/ADR.
Conserver PF-5, annulation et priorité au texte ; parallélisme sur groupes de 16 Kio retenu
seulement après mesure des copies/ordonnancement. N38 ne demande pas le maximum de goroutines.

### R3-M5 — Majeur : autorité et fraîcheur de l’index non totalement composées

**Règles :** IA-2, RG-1/4/5, C-4, N7/N33, principes §4.
**Preuves :** IA-2 réduit la lecture au « fichier courant » après contamination ; RG-4/5
expose plan/recherche/références/fichiers dans les fichiers choisis du projet. Le texte ne
dit pas si ces outils sont retirés ou filtrés à l’intersection. RG-1 attend 800 ms
d’inactivité : la saisie continue peut empêcher indéfiniment la mise à jour. Le quota
disque ne borne pas file d’instantanés et extraits intermédiaires. Un avertissement de
péremption ne remplace pas l’exclusion ajoutée après indexation.

**Fermeture :** autorité effective = intersection politique/projet/session/capacité/règle
des deux, vérifiée avant recherche **et lecture du texte retourné** ; pas de provenance,
pas de résultat. File coalescée bornée par document/octets ; âge maximal ou flush sur
recherche ; résultat incomplet déclaré. Exclusion/retrait/verrouillage/changement de projet
invalident réponses en vol/caches. Tester ces cas, fichier fermé et saturation distante.

### R3-M6 — Majeur : Ressources ne définit pas encore le plafond de nuisance

**Règles :** N7/N32, H-7, RS-2/3/4/7, C-4.
**Preuves :** H-7 promet mesure et arrêt ; RS-4 ralentissement puis pause. Mettre en pause
la **file** ne suspend pas une boucle native hostile déjà en cours. Sans cgroup délégué,
Linux/macOS reposent en partie sur une surveillance sans période, dépassement toléré ou
réserve du parent fixés. RS-7 « 0 » CPU vue fermée doit viser le rendu, jamais le superviseur.
Une goroutine/un Worker partagé n’a pas une RSS individuellement attribuable comme un
processus distinct.

**Fermeture :** distinguer information, plafond WASM, plafond noyau et watchdog ; limites
numériques par classe, intervalle/délai d’arrêt, cumul et réserves du cœur ; arrêter une
exécution hostile même vue fermée. Pour les tâches internes, afficher partage/estimation
explicitement et définir Arrêter sans interrompre une écriture non durable. Tester boucle
native et pression mémoire sur six hôtes, vue fermée puis ouverte sous saturation.

### R3-m1 — Mineur : restes contradictoires

**Règles :** N18/N36, §0.3 unicité du contrat. **Preuves :** §15.5 time.read dit encore
« sans elle, 0 » contre ABI-4 ; synthèse AD-87 « classe de sortie », AD-69 « hôte par
projet », AD-83 PD-2 comme delta ; traceability U29/U30 dit Tâches WebAssembly contre
§9.10 ; FM-OPEN-7 « éviction refusée » contre MF-21. Un passage précédant la matrice
rappelle AD-13/18/33 parmi les arbitrages alors que N35/N39/N40 les ont tranchés.
**Fermeture :** aligner ces renvois sans redemander les décisions utilisateur acquises.

## 5. Cohérence et phasage

**Résolutions acceptées au contrat** : C02 (DEK locale), C06/C07 (paquet reçu distinct de
l’activation, protocole séparé), C08 (sauvegarde distincte du stockage de travail), C09
(phrase/kit), C10 (flux), C13/C14 (diagnostics/raccourcis), C17/C20/C21/C22/C24/C25
(projection, onglets, formats, go-git, propositions, retrait UTCP). Référentiel N18/N36,
INV-3 ; enjeux initialement **majeurs**, remèdes acceptés comme cibles.

**Partielles** : C01/C03 restent B1/B2 ; C05 vaut feuille de route, pas parité prouvée ;
C11 nécessite le profil ACP ; C12 est affectée par M5 ; C15 dépend encore de la décision
CSS/PDF ; C16 de M3 ; C18 reste une recommandation utilisateur dans decisions, pas une
nouvelle autorisation obtenue en revue. C19 se vérifiera au branchement des accusés réels.
C23 est correctement décidée, mais sa traçabilité reste périmée (m1).

**Reports justifiés** : commentaires des lecteurs après extension de couverture, vrai
stockage remplaçable après sauvegarde personnelle, embeddings après gain mesuré, ACP/client
MCP sortant après confinement, multi-projets/intégration interprojets après v1. Référentiel :
priorité sécurité §2 et valeur/coût de coherence §5 ; gravité **majeure de portée**, correctement
exposée si N22/N41 restent partielles. **DOCX/ODT sont rejetés par N40** : je retire cette
suggestion de l’ancien audit ; aucune réouverture ni intégration native demandée.

**Carte produit : favorable, calme et orientée écriture.** Texte central, tiroir fermé,
deux vues à gauche, revue commune et Ressources hors du tiroir réduisent la concurrence
visuelle. Référentiel N30/N36/U31 et visibilité des états N17 ; enjeu **majeur**, solution
retenue. La réduction à quatre éléments d’état ne doit pas escamoter une écriture non
enregistrée/échouée ; indicateur IA et arrêts restent visibles quand nécessaires. Tester
clavier, fenêtre étroite et VoiceOver/NVDA/Orca. Le rangement des menus ne vaut pas
acceptation du design révision 5 encore à écrire.

**MVP : ordre réaliste, ampleur à surveiller.** G-0a → G-0b → collaboration sûre → LSP/PDF
et secours locaux → historique → registre TS évite de faire dépendre l’écriture du moteur
tiers expérimental. Référentiel premier jalon §1.1 et coherence §5 ; enjeu **majeur de
planification**. Les onglets du lot « Écrire et retrouver » dépendent toutefois de G-0c/
Worker : rendre cette dépendance explicite ; G-0a en lecture seule ne satisfait pas
N19/N20. B3/M2 précèdent la tranche historique. Prototyper coffre/format et profils OS
avant stabilisation v1, même si l’alpha Linux les diffère. Aucun calendrier fiable n’est
déductible du seul tableau. Une alpha ne clôt ni N35/N37 ni L5.

## Verdict

La révision permet un accord technique sur D-10 à D-16 et ferme beaucoup de défauts du
tour 2. Les expériences restent non acceptées jusqu’aux preuves ; leurs portes doivent
empêcher une réussite qui ne vérifie pas la garantie annoncée. Les majeurs M1–M6 demandent
une réponse explicite au prochain tour, sans exiger la livraison immédiate de tout le produit.

**NOT ACCEPTED — bloqueurs : R3-B1 (clair persistant du coffre), R3-B2 (enveloppe age), R3-B3 (historique contre GC), R3-B4 (portes insuffisantes ou contradictoires).**
