# Revue indépendante d’architecture — Codex — tour 2

Date : 2026-11-01. Architecture examinée : `1.0.0-brouillon.3`, protocole accepté :
`2.0.0-brouillon.10`. HEAD observé : `b5decfc` ; la révision de référence annoncée par
la spec (§0.2) reste `362cb4d`. Revue des contrats, avec vérifications statiques dans le
code ; aucune implémentation ni mesure de performance revendiquée. La revue
`03-fable-r2.md` n’a pas été ouverte. Les modifications préexistantes de `docs/exigences.md`
et le fichier de revue round 2 de l’autre reviewer restent intacts.

Corpus : spec, décisions, traçabilité, CHANGES, README et études du dossier architecture ;
les deux revues du tour 1 ; exigences N18–N40 ; invariants de CLAUDE.md et principes
d’ingénierie ; ADR, notamment 0002–0009, 0011, 0017–0026, 0028–0034 ; contrat public du
protocole ; chemins de code cités ci-dessous. Les études sont des propositions, non une
preuve d’implémentation. Les sources externes utilisées sont liées près des constats.

**Verdict : NOT ACCEPTED.** Les remèdes vont majoritairement dans le bon sens, mais
CHANGES.md ne peut pas déclarer tous les constats fermés. L’ABI, le confinement,
l’autorité des compartiments, la restauration et la compatibilité protocolaire ont encore
des défauts de contrat. N32–N40 demandent aussi une consolidation normative.

Gravités : **bloquant** = empêche d’accepter une garantie centrale ou un contrat annoncé
exécutable ; **majeur** = comportement important incomplet ou contradictoire ; **mineur** =
correction locale de cohérence. « Fermé au contrat » n’affirme pas que le code est livré.

## 1. Vérification de chacun des constats du tour 1

Les identifiants C et F désignent respectivement `02-codex.md` et `01-fable.md`.
Le référentiel et la gravité initiaux restent ceux de ces revues ; les constats encore
ouverts sont détaillés, avec leurs preuves et règles, en §2 et §4 de cette revue.

| Constat | Vérification dans la révision 2 | Conclusion |
|---|---|---|
| C-B1 ABI et protocole des greffons | §15.20 fixe pointeurs, exports, réacteur et réponse ; imports et paquet restent insuffisants, vecteurs seulement promis | **Ouvert : R2-3** |
| C-B2 Mermaid | §15.14 retire `frame`, diffère `ui.canvas`, impose P-FRAME avant réouverture | **Fermé au contrat**, par retrait de la voie dangereuse |
| C-B3 restauration | ST-7 reconnaît l’absence d’ancre sur appareil neuf ; ST-8 et ST-4/6 ne sont pas exécutables ensemble | **Partiel : R2-5** |
| C-B4 confinement et autorité IPC | §7.3 et §9.10 ajoutés, mais profil Go et séparation des droits incomplets | **Ouvert : R2-1, R2-2** |
| C-B5 modifications du protocole | §15.19 les désigne comme propositions et bloque N23 ; PD-1/2 contiennent encore des incompatibilités | **Partiel : R2-4** |
| C-M1 geste et révocation | §15.10 mandat 30 s, documents, nombre d’effets, révocation ; IPC-5 jette les réponses tardives | **Fermé au contrat dans le compartiment courant** ; usurpation entre greffons : R2-2 |
| C-M2 session multi-fichiers | MF-19–24 : plafonds, état par nœud, projection indépendante, dernier clic ; contradiction MF-12/20 et saturation manuelle | **Partiel : R2-7** |
| C-M3 CSV | MF-25 et addendum AD-52 imposent détection par opérations et oracle à identités | **Partiel : R2-8**, cas observateur tiers non couvert |
| C-M4 décisions ouvertes | matrice explicite les actes requis et retire la prétention de fidélité complète | **Transparence fermée**, décisions finales toujours ouvertes ; N35/N39/N40 : R2-6 |
| C-M5 coûts et surface | témoin, trajet complet et RSS ajoutés ; deux SDK, livraison étagée | **Partiel : R2-10** ; le banc reste expressément préalable à l’acceptation |
| C-M6 anti-exfiltration | RF-1–5 ferme les sources oubliées (métadonnées, compilation, import) et ajoute O4 | **Partiel : R2-2, R2-9** ; classes de processus et consentements non alignés |
| C-M7 aperçu accessible | §15.17 ajoute vue HTML interne assainie, descriptions, parcours Orca | **Fermé au contrat** ; preuve de fidélité et parcours reste une porte de livraison |
| C-m1 marques/flux/README | §0.2 héritage, §15 cible, sorties §3.1 et README Automerge corrigés | **Fermé sur le fond** ; référence HEAD périmée à actualiser |
| F-B-1 frame | même remède que C-B2 ; SC-11 ajouté | **Fermé au contrat** |
| F-B-2 deltas protocole | table PD, `renders` supprimé, N23 explicitement suspendue | **Partiel : R2-4** |
| F-B-3 hôte exécutable | memfd, abandon cache disque, AppArmor, mémoire et message d’échec précisés | **Partiel : R2-1** |
| F-B-4 niveau projet | §15.10 contribuer seulement, autre vue choisie, marques sans masquage, collage non enveloppé, EXT-ND2 | **Fermé au contrat fonctionnel** ; frontière native R2-2 |
| F-M-1 banc sans témoin | annexe P-PLUG-BENCH ajoute témoin, chemin complet, arm64 et interaction ≤100 ms | **Définition fermée ; preuve absente : R2-10** |
| F-M-2 canal absent | IPC-1–6 décrit transport et concurrence ; cadrage pas encore interopérable ni dimensionné | **Partiel : R2-2** |
| F-M-3 surface DX | §15.21 étagé, Rust/Go d’abord ; la liste API 1.0 n’est pas celle de « 13 interfaces » annoncée dans CHANGES | **Direction fermée**, compter sur schéma réel à P-EXT ; dépend R2-3/R2-10 |
| F-M-4 fraîcheur/clés | clé par personne et projet, transfert explicitement prévu, résiduel appareil neuf écrit | **Partiel : R2-5** |
| F-M-5 HELLO | PV-6 et §15.10 retirent la prétendue connaissance préalable | **Fermé** |
| F-M-6 ménage | §6.5/15.10 racine du paquet ; §15.14 rendus ordinaires référencés ; §4.2/15.10 suppression magasin privé ; cache mémoire | **Fermé au contrat local** ; ménage sauvegarde distinct : R2-5 |
| F-M-7 G-0 | §6.6/AD-77 : G-0a avant Worker, G-0b après annonces, tests FM-OPEN nommés | **Chemin fermé**, limites de l’état affiché : R2-7 |
| F-M-8 tokens | §15.4 : 150 ms, annulation, dernières plages, désactivation >2 Mi | **Fermé au contrat** |
| F-m-1 CeTZ/fletcher | introduction §15.14 | **Fermé** |
| F-m-2 polices | §15.14.1 : polices projet/fournies | **Fermé au contrat**, frame retiré |
| F-m-3 CSP scripts externes | frame retiré, P-FRAME avant retour | **Fermé par retrait**, pas une validation de CSP future |
| F-m-4 exemple bilingue | §15.15 `strings.toml` fr/en | **Fermé** |
| F-m-5 Tâches hors Linux | §9.10 dit explicitement « indisponible » | **Réponse explicite**, insuffisante pour N35 : R2-6 |
| F-m-6 ISO | §2 rattache extensibilité à modifiabilité/adaptabilité | **Fermé** |
| F-m-7 contraste AA/AAA | TH-2 distingue refus AA et cible AAA | **Fermé** |
| F-m-8 dépendances | PP-6 : décision porte dépendances, graphe incomplet non chargé | **Fermé au contrat**, schéma encore soumis à PD-1 |
| F-m-9 phrase document.read | §15.5 : « documents de ce projet » | **Fermé** |

## 2. Défauts encore bloquants

### R2-1 — Bloquant — H-1/H-5 ne constituent pas encore un profil de confinement exécutable

**Règles :** N7/N21/N32, INV-4 ; principes §4 ; ADR 0033 ; sémantique
[Landlock](https://docs.kernel.org/userspace-api/landlock.html) et
[seccomp](https://docs.kernel.org/userspace-api/seccomp_filter.html).

**Preuves :** spec §9.10 H-1 exige tous les droits Landlock avec ABI ≥1 ; H-5 énumère une
liste blanche de syscalls ; l’enfant pose lui-même ces restrictions avant le premier module.
`internal/sidecar/dir_linux.go:33` exige déjà ABI ≥3 pour son périmètre, plus restreint.
Le runtime Go installé (`go1.27.3`, cohérent avec `go.mod`) crée des threads par `clone`
(`runtime/os_linux.go:156–185`), absent de H-5. Les signaux « du runtime Go » ne sont pas une
liste vérifiable ; epoll/eventfd et les opérations de gestion des FD ne sont pas spécifiés.
Refuser aveuglément `clone` casse la croissance du runtime ; l’autoriser sans masque ouvre
un périmètre de création de processus non décidé.

Landlock ne fournit pas « tous les droits » dès ABI 1. `REFER` apparaît en ABI 2,
`TRUNCATE` en ABI 3, TCP en ABI 4, ioctl périphériques en ABI 5. Ajouter les bits inconnus
fait échouer la création ; les retirer demande une compensation seccomp explicite pour
chaque garantie. Les FD déjà ouverts ne deviennent pas inaccessibles par magie. Ces faits
ne prouvent pas qu’ABI 1 est inutilisable : ils prouvent que la composition doit être écrite.

| Famille de système à tester | Noyau de référence, non une détection | ABI amont attendue | Conséquence pour le profil |
|---|---|---|---|
| Ubuntu 22.04 GA | 5.15 | 1 | masquer droits récents ; interdire séparément troncature et autres opérations non médiées |
| Debian 12 initial | 6.1 | 2 | pas de droit TRUNCATE ; noyaux de remplacement possibles |
| Ubuntu 24.04 GA | 6.8 | 4 | profil AppArmor pour userns ; syscall Landlock à sonder même si userns indisponible |
| Debian 13 initial | 6.12 | 6 | droits et portée plus riches ; garder le même profil de sécurité minimal |

Références de versions : [Ubuntu, cycle noyaux](https://ubuntu.com/kernel/lifecycle),
[Debian 12](https://www.debian.org/releases/bookworm/amd64/release-notes/ch-whats-new.en.html),
[Debian 13](https://www.debian.org/releases/trixie/release-notes/whats-new.en.html),
[table ABI amont](https://www.kernel.org/doc/html/v6.13/userspace-api/landlock.html).
Ce tableau est une dérivation des versions initiales, pas une mesure sur ces distributions.
HWE, rétroportages, LSM désactivé et conteneurs imposent de sonder l’ABI réelle ; aucun
résultat de matrice distro × architecture n’est fourni dans le dépôt.

**Autre défaut central :** Landlock s’applique au thread et à ses futurs descendants ;
seccomp exige une politique de synchronisation des threads. Poser ces filtres depuis une
goroutine d’un Go déjà multithreadé ne suffit pas. H-6 décrit le réseau, pas la vérification
de H-1/H-5 par le parent à laquelle l’introduction renvoie. Il manque bootstrap, ordre
`no_new_privs`/Landlock/seccomp, TSYNC ou mécanisme équivalent, traitement des threads déjà
créés et résultat vérifiable avant chargement. Refuser `execve` **après** lancement est
correct : le parent exécute d’abord l’enfant, puis l’enfant se confine. Pré-appliquer un
Landlock sans droit EXECUTE empêcherait en revanche ce lancement : ne pas réutiliser tel
quel le wrapper sidecar.

**Fermeture :** profil exact amd64/arm64 (architecture BPF incluse), restrictions d’arguments
de `clone`, signaux et FD, `close`/réception memfd, refus des écritures/troncatures par chemin,
bootstrap de tous les threads et matrice Ubuntu/Debian/Fedora en paquet et archive. Tester
un enfant natif hostile, puis compilation WASM, GC, croissance des threads, délais et arrêt.
L’absence de réseau via `socket` refusé peut suffire sans userns, à condition de ne transmettre
aucun autre socket utilisable. Le profil AppArmor et l’abandon du cache disque sont de bons remèdes.

### R2-2 — Bloquant — IPC et compartiments ne portent pas encore la même autorité

**Règles :** moindre privilège (principes §4), N7/N23, IPC-3, RF-1–5 ; CWE-441
(adjoint complaisant) ;
[sockets Unix Linux](https://man7.org/linux/man-pages/man7/unix.7.html).

**Preuves :** H-4 regroupe les greffons globaux O0 et tous les greffons d’un projet ;
IPC-3 attribue un compartiment « à chaque activation » puis le mappe vers un greffon et
ses droits. Or H-4 dit **un hôte = un compartiment**, qui peut contenir plusieurs greffons.
Après évasion, le parent ne peut authentifier lequel a produit l’appel. Une génération ou
un ID transmis à cet enfant est connu de l’enfant compromis. Les revendications d’un
autre processus peuvent être refusées ; celles d’un autre module cohébergé ne le peuvent
pas sur cette seule base. PP-2 interdit pourtant le mandat d’une autre commande.

H-4 ne dit pas où logent les greffons O1 et **O4**. `ai.complete` est interdit aux greffons
de projet mais pas aux globaux. Le ranger parmi « sans réseau direct » donnerait, après
évasion, son envoi IA à des modules O0 cohébergés. Le ranger ailleurs exige une règle écrite.
Même sans sortie réseau, des droits de lecture de projets différents ou des mandats distincts
ne sont pas automatiquement une seule classe de confiance.

IPC-2 autorise un paquet de 4 Mio sans réglage ni contrôle de capacité `SO_SNDBUF`, sans
fragmentation et sans solution si le noyau refuse un message atomique trop grand. Un
`SOCK_SEQPACKET` ne transforme pas spontanément 4 Mio en petits enregistrements ; il faut
traiter `EMSGSIZE`, `MSG_TRUNC`, les crédits en **octets**, le drainage et la priorité de
`cancel`. Seize appels ×4 Mio plus callbacks ne sont pas bornés par la seule file de huit
appels par greffon. L’ordre des octets des entiers et les valeurs de `type` manquent aussi.
La tentative locale de `socketpair` a reçu `EPERM` du sandbox de revue : **aucune taille
effective de buffer n’est prétendue mesurée ici**.

**Fermeture :** lier l’autorité à la connexion possédée par le parent ; un processus par
ensemble exact de droits/projet/consentement, ou déclarer explicitement une autorité
commune et ses conséquences. Une sortie O1/O4 exige son propre compartiment et un geste
créé par le parent, pas une déclaration de l’enfant. Rédiger un mini-codec complet avec
identité des appels et callbacks, endian, crédits d’octets, quotas cumulés, refus des FD
ancillaires inattendus, fragmentation bornée (ou stream cadré), annulation prioritaire et
test 4 Mio sur systèmes non privilégiés. Conserver le contrôle de génération côté parent.

### R2-3 — Bloquant — Le noyau ABI ne démontre pas la compatibilité Go/TinyGo promise

**Règles :** N25/N26, ABI-1–8, ADR 0034 (contrat reproductible), RFC 8949 §4.2.1.

**Preuves :** §15.20 ABI-3 refuse toute importation WASI hors liste. Celle-ci omet
`poll_oneoff`, `sched_yield`, `fd_prestat_get` et `fd_prestat_dir_name`.
Le [runtime Go WASI](https://go.dev/src/runtime/os_wasip1.go) importe `poll_oneoff` ; les
sources installées `runtime/lock_wasip1.go:114` importent `sched_yield` et
`syscall/fs_wasip1.go:339–378` parcourent les preopens à l’initialisation via `fd_prestat_get`.
Un bonjour usuel avec `fmt` entraîne `os`/`syscall` : la liste actuelle n’est donc pas un
profil compatible avec cette chaîne standard. Le réacteur Go est bien réalisable avec
`-buildmode=c-shared` et `//go:wasmexport`, depuis
[Go 1.24](https://go.dev/doc/go1.24) ; il faut aussi fixer son maximum mémoire à la construction.

TinyGo installé : 0.41.1 ; `targets/wasip1.json` choisit `scheduler: asyncify` ;
`src/runtime/runtime_wasip1.go:83–104` utilise et importe `poll_oneoff`. Le comportement
exact d’un bonjour minimal après élimination des fonctions inutilisées dépend des options
du linker : **non exécuté et non établi ici**. Il est incorrect d’affirmer que le bonjour
Go **et** TinyGo fonctionne sans fournir binaire, options et import table. ABI-8 ne fournit
que les noms de vecteurs futurs. Aucun kit/plugin host n’existe dans les chemins de code
examinés ; cette revue ne les invente pas pour faire passer la porte.

L’horloge constante 0 sans `time.read` peut aussi empêcher les attentes du runtime ; il
faut distinguer temps logique exposé au plugin et fonctions nécessaires à son ordonnanceur.
`host_call` alloue via `inkwell_alloc` alors que le module est déjà en cours d’appel :
la réentrance du runtime/allocateur doit être contractualisée ou évitée par un buffer fourni.
La taille maximale, le propriétaire et la libération des entrées, les chevauchements et les
erreurs d’allocation doivent être explicites, pas déduits de « même convention ».

Enfin ABI-7 ne définit ni la canonicalisation du manifeste, ni l’encodage de la liste
nom→hash, ni le tri des noms, doublons, liens et exclusions (`signature.cbor` doit être
exclu de sa propre liste). Un « niveau DEFLATE fixé » n’est pas une valeur ni un algorithme
canonique. Le hash de contenu peut éviter de signer le ZIP brut ; il faut le dire exactement.

**Fermeture :** publier maintenant un profil SDK **épinglé** Rust/Go/TinyGo, commandes et
modules bonjour, table des imports effectivement liés, stubs WASI sûrs (aucun preopen,
`EBADF` défini), horloge/attentes, réentrance et vecteurs d’octets du paquet signé. Les
interfaces riches peuvent attendre `docs/plugin-api/` ; ce minimum détermine la faisabilité
du choix et ne peut être repoussé après son acceptation.

### R2-4 — Bloquant — PD-1/PD-2 sont correctement isolées, mais leur texte ne définit pas une mineure compatible

**Règles :** protocole §1.4.4, §4.4, §20.4/20.5 ; ADR 0030/0034 ; INV-4/10.

**Preuves :** PD-1 (§15.19) prescrit « valeurs CBOR à clés fixes ». Le codec accepté
`policy_op_v1.entries` est `(key str16, value str16)` ; `str16` est du **UTF-8** (§0.1).
Le CBOR brut n’est pas généralement de l’UTF-8. Choisir une représentation textuelle
canonique (par exemple base64url d’un CBOR précisément défini) ou une version d’objet
distincte, avec capacité et stratégie de relais. Il manque aussi le schéma concret
plugin (hash/version/droits/dépendances/retrait) et IA, pas seulement les bornes 4096/160.

La phrase « le pli reste identique entre 2.0 et 2.1 puisqu’une entrée ignorée n’a aucun
effet » est fausse : le client **2.0 existant** ne connaît pas la nouvelle tolérance et
refuse l’op ; le 2.1 applique les espaces connus. `format ≥1` offre une transition honnête
en lecture seule aux anciens (§1.4.4), pas un patch rétroactif de leur décodeur. Vectoriser
notamment une op mêlant `main` et `plugin/`, l’arrivée de `format` dans différents ordres,
les entrées inconnues futures et les générations de reconnexion.

PD-2 propose des domaines `inkwell/plugin/v1` et `inkwell/backup-head/v1` au §20.4 ; ce
registre dit qu’un domaine ne contient **ni slash ni NUL**. ABI-7 signe pourtant un préfixe
contenant les deux. C’est possible comme **protocole de signature externe distinct**, pas
comme inscription littérale dans le registre existant sans amendement de sa grammaire et
du framing §3.3. Ne pas modifier les domaines des objets acceptés pour y arriver.

**Fermeture :** conserver le blocage N23 et de la politique IA partagée ; corriger les
propositions avec codecs/vecteurs et les faire accepter dans la boucle protocole. Une
réservation de domaine de sauvegarde externe ne doit pas bloquer inutilement une sauvegarde
locale personnelle qui n’émet aucun objet réseau. Les fichiers de rendus ordinaires,
eux, ne demandent plus de delta : ce choix est accepté (D-3).

### R2-5 — Bloquant — Le contrat de récupération ST-8 est impossible, ST-4/6 divergent

**Règles :** ADR 0029 I3/I4/I6, ST-3–8 ;
[BIP-39, génération du mnémonique](https://raw.githubusercontent.com/bitcoin/bips/master/bip-0039.mediawiki).

**Preuves :** ST-8 promet 24 mots BIP-39 encodant une clé aléatoire de **256 bits**, le
ProjectID, le compteur et sa date avec checksum. Ces 24 mots contiennent exactement 256
bits d’entropie et 8 bits de contrôle : la clé occupe déjà toute la charge utile. Aucun
encodage sans perte n’ajoute ces champs indépendants dans ces mêmes mots. Une dérivation
ne récupère pas un ProjectID existant indépendant à partir de la clé.

ST-4 nomme l’objet par **BLAKE3 à clé tronqué à 32 caractères hex**, ST-6 demande
« BLAKE3 du clair = nom attendu » : un hash non précisé comme keyed, complet, ne correspond
pas. Le manifeste a une « AAD de manifeste » non définie ; les têtes n’ont pas de format
binaire signé ni de nonce/AAD définis. Ce sont des trous dans le contrat cryptographique,
pas une accusation contre XChaCha20-Poly1305.

ST-7 est désormais honnête sur la fraîcheur d’un appareil neuf : **cette partie du constat
initial est fermée**. Il reste à indexer les compteurs par `(ProjectID, PeerID)` et à
persister leur maximum ; un compteur global ne compare pas des têtes de plusieurs appareils.
§9.2 ne doit plus promettre la détection universelle du rejeu par le dorsal.

Le ménage se fonde sur « les deux dernières têtes de chaque appareil » alors que `list`
peut être incomplet. Il ne dit pas qui décide parmi plusieurs appareils de la même personne,
ni comment protéger les objets téléversés par un autre appareil avant sa tête. **Risque
déduit :** un balayage peut supprimer un objet d’une sauvegarde en cours ou cachée par une
liste incomplète. Une lecture de vérification n’empêche pas une suppression ultérieure.

**Fermeture :** 24 mots = secret seul, plus un **kit de récupération versionné** portant
ProjectID, localisateur du dépôt, ancres par appareil et dates, ou format plus long
explicitement distinct de BIP-39. Définir noms keyed et vérification identiques, CBOR/champs,
AAD et signature des manifestes/têtes. Désigner un coordinateur de GC ou différer le GC
distant en v1 ; aucun effacement basé uniquement sur une liste non fiable. Tester phrase
seule/kit, deux appareils, écriture interrompue, liste tronquée et restauration hors ligne.

### R2-6 — Bloquant — La baseline normative n’intègre pas N35/N37/N39/N40

**Règles :** exigences explicites N18/N35/N37/N39/N40 ; INV-3/7/8 ; ADR 0003,
0008, 0020 et 0034 ; principe « une exception exige une ADR ».

**Preuves :** AD-18 et §9.10 décident Linux seulement et désactivent les tiers ailleurs ;
NG3/§9.2 maintiennent projet en clair, tandis que `research/encryption.md` annonce NG3 levée
par N37 mais reste une étude sans décision. AD-13 recommande toujours un écrivain Git
maison et rejette go-git ; N39 et CLAUDE.md demandent go-git. NG1, AD-33 et traceability §1
conservent import/export DOCX/pandoc comme arbitrage natif ; N40 l’a déjà tranché en faveur
des plugins. La matrice des actes futurs améliore la transparence, pas cette contradiction.

**Position :** le refus fermé sur un OS sans sandbox est une bonne mesure provisoire.
Il n’est **pas** une réponse finale acceptable à Linux/macOS/Windows × amd64/arm64. Ne
réduire ni la sécurité pour obtenir la parité, ni N35 à une compilation d’UI sans fonctions.

**Fermeture :** matrice finale six cibles, backend d’isolation par OS et contrat commun
avec sonde de capacités/échec fermé (voir §4). Remplacer NG3 par un contrat N37 et borner
les fichiers temporaires en clair. Retenir go-git pur Go derrière un port de projection,
sans exécutable Git ni refs faisant autorité. Retirer l’intégration bureautique native et
la dépendance pandoc qu’elle justifiait ; un plugin peut l’offrir avec capacités locales.
Une étude ou un arbitrage déjà remplacé ne doit pas servir de norme concurrente.

## 3. Position indépendante sur D-1 à D-9

Ces positions proposent un compromis technique ; elles ne prétendent pas être un accord
déjà obtenu avec l’autre reviewer. Gravité indiquée = celle de l’enjeu restant, pas une
nouvelle série de constats en double.

| Désaccord | Position et preuve | Règle / gravité |
|---|---|---|
| **D-1 ABI ici ou plus tard** | Accepter le partage : noyau minimal **exécutable et vectorisé maintenant**, interfaces par point ensuite. §15.20 est la bonne place mais ABI-3/7/8 échouent encore à ce critère (R2-3). Ne pas demander tous les SDK avant le premier prototype. | N25/N26, ADR 0034 ; **bloquant** |
| **D-2 Mermaid** | Accepter **différé** et Graphviz livré. Un Mermaid livré/de confiance retire la menace « code plugin arbitraire », mais doit encore supporter la source byzantine, le DOM et l’interruption. Aucun résultat P-FRAME ne justifie son retour v1. Une fenêtre Wails cachée n’est pas une preuve de processus indépendant sur chaque OS. | N7, SC-11, §15.14 ; **fermé par retrait** |
| **D-3 renders** | Accepter fichiers ordinaires + validation à l’interprétation (§15.14.4). Supprimer la carte spéciale évite une nouvelle garde. Réassainir SVG et borner JSON, chemins, nombre d’entrées et profondeur avant lecture. Rendu partagé trompeur = contenu attribué, jamais preuve de la source. | INV-3/4, protocole G9, X-8 ; **fermé au contrat** |
| **D-4 version HELLO** | Accepter retrait. PV-6/PP-4 vérifie localement l’intervalle, affiche le repli ; un TLV serait un indice d’UX, aucune attestation d’une capacité réelle d’un pair byzantin. Il ne mérite pas un delta maintenant. | protocole §5.3, N14 ; **fermé** |
| **D-5 nombre d’hôtes** | Accepter processus séparé, corriger « classe de sortie » en **autorité commune exacte** ; O4/O1, projets et mandats comptent. H-4/IPC-3 incompatibles aujourd’hui (R2-2). Mesurer les coûts avant de choisir les regroupements, sans prétendre protéger un module de son colocataire natif compromis. | moindre privilège, N7, P-PLUG-BENCH ; **bloquant** |
| **D-6 CSV** | Accepter maintien texte + détection sémantique obligatoire maintenant ; modèle structuré seulement après essai. Étendre MF-25 au cas où le receveur n’est aucun des deux auteurs (R2-8). | ADR 0026, N6 ; **majeur** |
| **D-7 accessibilité** | Accepter la vue lecture interne, texte/structure/math et descriptions (§15.17), avec validation sur mêmes documents. Vérifier tableaux, renvois, ordre et navigation ; ne pas annoncer une équivalence générale sur la seule présence d’un export HTML. | WCAG 2.2 SC 1.1.1/1.3.1/2.1.1 ; **fermé au contrat**, preuve de livraison |
| **D-8 projets simultanés** | Préférer un `ProjectRuntime` propriétaire par projet dès les frontières internes, puis v1 avec une seule fenêtre active si l’utilisatrice l’autorise. Cela limite le coût immédiat sans mélanger clés/quotas/sessions. La matrice AD-01 reconnaît enfin l’écart ; aucune décision utilisateur acquise n’est démontrée. | N18, conception p.13, AD-01 ; **majeur, décision explicite attendue** |
| **D-9 GREASE** | Accepter uniquement dans la découverte négociée `host.features()` et le kit de conformité. Pas dans les opérations à effets ni pour prétendre qu’un unique hôte est un écosystème réseau. Tester champ inconnu et « à comprendre » suffit pour les messages fermés. | RFC 8701, PV-1–7 ; **fermé au contrat** |

## 4. Autres constats et hooks des nouvelles exigences

### R2-7 — Majeur — Session manuelle saturée, G-0a et versions de diagnostics

**Règles :** N19/N20/N28/N29, MF-19–23, ADR 0020/0029 (état honnête), LSP 3.17
([synchronisation](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#textDocument_synchronization)).

**Preuves :** MF-12 détruit projection/synchronisation externe au déchargement ; MF-20 dit
l’inverse. MF-21 interdit d’évincer un document sale en mode manuel ; MF-19 ne dit pas quoi
faire quand les 8 Mi sont tous sales et qu’une ouverture ou réception demande une poignée.
Il faut un refus d’ouverture intelligible ou un état sale persistant indépendant du handle,
sans bloquer la réception admise. Définir quel état est évictable et quelle borne couvre
la mémoire des historiques Automerge, pas seulement le nombre de caractères présents.

G-0a est un bon correctif immédiat mais affirme « pas encore partagé » pour tout autre
fichier lu depuis la projection : des blobs/projections peuvent déjà être partagés. Dire
« affiché en lecture seule dans cette version », avec l’état réseau réel. Ne pas promettre
la présence par nœud avant MF-7. Code confirmé : `App.svelte:197` ignore le chemin ;
`workspace.svelte.ts:42,275–293` reste centré sur `MAIN_FILE`. G-0a n’est pas livré.

ER-2 et LSP-3 veulent des diagnostics ancrés et des réponses versionnées ; les réponses LSP
n’ont pas toutes un champ version. Le client doit conserver `(requestId, nodeId, revision,
generation)` et un instantané de conversion UTF-16. Un diagnostic du compilateur sur une
projection ancienne n’est pas ancré correctement en le convertissant dans le texte courant.
Les fichiers fermés en mode manuel peuvent différer de leur projection : LSP-2 doit fournir
un overlay des documents sales ou signaler explicitement les diagnostics périmés.

**Fermeture :** une règle unique de durée de vie ; cas plafond entièrement sale ; versions
par requête et par compilation avec mapping de l’instantané, diagnostics périmés inactifs,
actions LSP appliquées comme propositions atomiques multi-fichiers sous garde et rôle.

### R2-8 — Majeur — MF-25 protège l’auteur local, pas tous les observateurs d’une fusion CSV

**Règles :** N6/N7, ADR 0026, oracle MF-25 (« chaque cellule garde sa ligne d’origine ou est signalée »).

**Preuve :** MF-25 compare les lignes « que la personne a touchées » aux suppressions
distantes. Si A modifie une cellule et B supprime sa ligne concurremment, C n’a touché
aucune ligne : l’intersection locale est vide chez C, même si la valeur est déplacée.
`frontend/src/editor/csv/collab.test.ts:217–220` ne prouve toujours que convergence et
longueur non nulle des records. Le nouvel oracle est annoncé, pas implémenté.

**Fermeture :** calculer sur toutes les opérations concurrentes admises, local/local comme
distant/distant, indépendamment de l’observateur et de l’ordre d’arrivée. Inclure C en
ligne, C hors ligne puis rattrapage, réouverture, colonnes et cellules multilignes. Conserver
le conflit attribué jusqu’à réparation ; ne pas modifier automatiquement le texte pour le
masquer. Cette précision rend l’option texte acceptable sans imposer un CRDT structuré.

### R2-9 — Majeur — IA : consommation visible, consentement continu et identité MCP manquent au contrat

**Règles :** N31, N7, RF-3, IA-2 ;
[MCP Security Best Practices](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices)
(local server compromise, proxy stdio, scope minimization).

**Preuves :** §15.18 fournit une bonne base : Go seul détient réseau/secrets, profils internes,
propositions, classe `jamais`, réduction hors prompt, HTTP optionnel, UTCP export seulement.
Mais l’exigence N31 complétée demande **tokens, coût, latence et paramètres visibles et
réglables** : ni les budgets par session/projet ni un schéma d’usage/estimation ne sont
normatifs. L’aperçu au premier envoi d’une session n’autorise pas automatiquement les plages
ajoutées plus tard par un outil, ni un changement de profil ou de politique en cours de SSE.

MCP-1 donne une socket 0600 au relais, MCP-3 promet un consentement **par client** : le nom
annoncé par le protocole ou l’UID Unix ne prouve pas quel client de la même session se
connecte. La menace « autre programme de la même session hors périmètre » (§9.2) peut être
assumée, mais alors l’écran ne doit pas présenter cette attribution comme authentifiée.
Prévoir jeton de lancement par client/session et projet, ACL socket/canal nommé selon OS,
expiration et révocation. Avant PD-1, choisir explicitement si MCP est désactivé pour le
projet partagé : une politique par appareil ne matérialise pas la politique admin promise.

ACP-1 reconnaît correctement qu’un agent est **à pleins droits** : le dossier vide et
l’absence de méthodes `fs` ne l’empêchent pas d’utiliser ses propres outils. La « règle des
deux » ne s’applique qu’aux services d’inkwell si ce processus n’est pas réellement enfermé.
Conserver l’option différée et le résiduel explicite ; ne pas lui attribuer les garanties
des plugins WASM. UTCP reste un export sans canal d’exécution autonome : choix accepté.

**Fermeture :** schéma de profil, limites de flux/tours/outils/jetons, annulation, compteurs
réels vs estimations, prix inconnus et confidentialité des traces ; revalidation à chaque
envoi/outil avec consentement lié à portée et destination. Contamination `autrui/externe`
monotone sur toute la session, y compris résultats d’outils et extraits RAG ; pas de retour
à privilèges élargis après résumé. Tester changement de projet/politique/profil en vol.

### R2-10 — Majeur, porte explicite d’acceptation — N38 : protocole de mesure amélioré, aucun résultat

**Règles :** N38, principes §3, CLAUDE.md « prove perf changes », annexe P-PLUG-BENCH
**« Prérequis de l’acceptation »**, ADR 0028.

**Preuves :** la porte compare désormais témoin/cible, p50/p95/p99, 64 Kio/1/4 Mio, trajet
complet, réentrance, 16 plugins et amd64/arm64. Aucun résultat de ce banc ni prototype
P-PLUG-PROTO n’accompagne la révision. H-7 ne remplace pas un budget **global** des multiples
hôtes : `SetMemoryLimit` est une cible de GC, non un plafond de RSS. 4 Gio d’espace virtuel
peut aussi être insuffisant selon les réservations du runtime/JIT : **hypothèse à mesurer**,
pas preuve d’un crash existant. Les délais muraux ne bornent pas le débit CPU soutenu d’un
plugin réactivé sans fin.

**Fermeture :** fournir le petit prototype et les mesures prescrites, matériel/OS/versions,
résultats bruts, profils et budgets globaux Go+webview+sidecars+hôtes. Sinon réécrire
explicitement le statut de la décision en expérimentation non acceptée ; ne pas prétendre
fermer C-M5 en ajoutant seulement une meilleure liste de mesures.

### R2-11 — Majeur — Hooks N32/N33/N34/N37/N39 à intégrer dans spec et traçabilité

**Règles :** N18/N32–N39, INV-1/3/4/8/9, principes §2–4, ADR 0020/0023/0029/0034.
**Preuves :** traceability §2 s’arrête à N31 ; spec §6/9/15 n’intègre pas les contrats des
études `sync-ignore.md`, `encryption.md` et de `ai.md` §12. Ces études sont utiles, mais
plusieurs interactions changent les frontières d’autorité. Les hooks suivants sont requis
avant de transformer ces recommandations en implémentation ; les paquets ne doivent être
créés qu’au moment utile.

| Exigence | Hook architectural requis et preuve attendue |
|---|---|
| **N32 gestionnaire de tâches** | Un superviseur cœur possède processus, instances, tâches, générations, files et réservations. Le panneau ne peut pas être fourni uniquement par un plugin qu’il doit arrêter. Identifiants stables, état/raison de pause, CPU mesuré, mémoire distinguant linéaire/compilation/RSS, I/O et quotas ; arrêter/réactiver/révoquer sans perdre les données. H-7 est par hôte : l’UI ne doit pas attribuer arbitrairement le tas partagé à un module. Plafond global du nombre d’hôtes, CPU soutenu et relances ; limites OS par processus (cgroups si disponibles, alternatives mesurées sur les autres OS), budget WASM par module, file parent bornée même si l’enfant ment. Tests boucle infinie native/WASM, allocations, avalanche IPC et « Arrêter » sous saturation. |
| **N33 RAG natif** | Adopter le service dérivé proposé dans `research/ai.md` §12, avec port instantané `(nodeId, revision, texte, provenance)` venant de la webview, jamais Automerge décodé en Go ; index local jetable, schéma versionné, purge, quotas et annulation via N32. Outils recherche/lecture bornés communs à IA/MCP, citations par **nodeId** et révision, vérification d’actualité. L’étude propose encore des citations par chemin : renommer ne doit pas les casser. Filtrer `ai: never` et portée **à l’indexation et à chaque requête** ; embeddings/reranking sont des envois O4, pas de simples lectures locales. Le choix FTS5/pilote et les modèles doivent passer compatibilité six cibles et mesures rappel/latence/coût ; ne pas accepter les performances estimées de l’étude comme mesures. |
| **N34 sync-ignore** | Une politique d’import/publication locale distincte de l’index partagé et de l’admission. L’étude affirme « ne ni envoyer ni accepter un chemin exclu » et propose de retirer un fichier de l’index : les échanges Automerge portent des documents/opérations, pas un filtre sûr par chemin ; un document peut rester référencé sous un autre nom, et l’index entier contient des nœuds ignorés. Ne pas supprimer chez tous pour un choix personnel. Définir fichiers déjà reçus, renommages, ré-inclusion, dépendances et absence attendue ; l’ignore n’est pas un contrôle d’accès et ne retire rien aux anciens destinataires. Une règle admin partagée nécessite un contrat signé/proposition protocole, pas un fichier éditable ordinaire qui devient une seconde autorité. Rejeter `.typstdeps/` comme défaut global de l’étude : contraire à N5/ADR 0024 hors ligne. Tester motifs, plateformes/casse, chemins hostiles, fichiers exclus référencés par Typst, G-0 et modèles. |
| **N35 six plateformes** | Port sandbox avec garanties communes (FS/réseau/IPC/processus/mémoire/CPU/arrêt), sondes et échec fermé, adaptateurs Linux/macOS/Windows. Étudier les mécanismes macOS et Windows natifs puis prouver les garanties ; leur nom seul n’est pas une preuve. Transports internes : Unix socket ou canal nommé protégé, handles hérités limités ; budgets et builds natifs amd64/arm64. ADR 0033 refuse déjà les lancements bornés sans backend approprié : porter aussi typst/tinymist, pas seulement les plugins. Linux seul est un jalon, pas l’architecture finale. |
| **N37 chiffrement projet** | Port magasin chiffré sous les codecs existants, AEAD liant projet/type/objet/époque ; enveloppes par appareils, clé X25519 séparée authentifiée et récupération. Le changement d’annuaire et les époques exigent proposition protocole, pas un TLV supposé suffisant. Le blob chiffré doit être vérifié avant déchiffrement et son clair validé ensuite. Un objet par écriture évite de réécrire un bloc monolithique ; ce choix reste à faire approuver comme interprétation du besoin. Définir verrouillage et matérialisation privée minimale pour Typst, LSP, Git, recherche, export, vignettes et journaux ; index RAG, Git et fichiers temporaires sont des copies sensibles. Crash/reprise/nettoyage et refus de promesse d’effacement physique sur SSD. Un simple chiffrement du store laissant la projection claire permanente ne satisfait pas N37. Les sauvegardes ST-3 et la DEK du projet sont deux usages distincts, à relier explicitement dans le kit de récupération. |
| **N38 mesures** | Harness reproductible et seuils CI (régression, taille, CPU/mémoire) sur les frontières ci-dessus, profils réels et bancs de concurrence, plus budgets UX de §12. Aucune optimisation « la plus rapide » ne se déduit du langage Go ; comparer les options sur matériel déclaré. P-PLUG-BENCH demeure la porte initiale R2-10. |
| **N39 historique autonome** | `go-git` pur Go en projection reconstructible, jamais invocation de `git`, `ssh` ou outils de compression externes ; annuler la recommandation d’écrivain maison AD-13. Le texte courant et les versions restent dans Automerge ; la restauration produit une **nouvelle modification** soumise aux droits et à la garde, jamais un remplacement de magasin/ref qui ressuscite autorité ou purge. Navigation arrière/avant sans perte : état consulté distinct de l’actif, brouillon courant préservé, restauration prévisualisée et annulable, fichiers supprimés/blobs/dépendances présents dans le jalon. Tests sans Git dans PATH et après reconstruction de la projection, changement de rôle pendant restauration, reprise après crash ; vocabulaire UI « version », « comparer », « restaurer », aucun commit/branche/checkout requis. |

### R2-12 — Mineur — Modèles et outils d’écriture : conserver la direction, préciser les entrées actives

**Règles :** N27/N30/N40, INV-4, TM-1–7, principes §4, WCAG 2.2.
**Preuves :** §6.8 couvre bien créer/modifier/dupliquer/supprimer/importer/exporter/partager,
sans modèles imposés ni téléchargement implicite. TM-6 indique honnêtement que CSS ne
pilote pas le PDF ; cet écart à N27 demande la décision explicite annoncée, pas une promesse
de second moteur implicite. Les substitutions de titre/auteur doivent être typées ou échappées
selon le fichier : une valeur ne doit pas devenir du code Typst ou un chemin arbitraire.
Les archives de modèles restent soumises aux limites/anti-traversée d’AD-38, et les vignettes
à la même compilation hostile que tout projet. Enregistrer un projet comme modèle doit
choisir les fichiers inclus et exclure clés, sauvegardes et état personnel, y compris les
nouveaux caches N33/N37.

Le catalogue N30 est adapté à l’écriture : orthographe locale embarquée, relecture sans note,
extraits, renvois, comparaison par mots et objectifs locaux. Pour dictionnaires/extraits
partagés, réutiliser le magasin et les garde-fous existants, borner expansions et regex,
aucun script ni accès réseau implicite. Les commentaires de lecteurs restent derrière la
mineure explicitement requise par AD-14 ; le panneau N30 ne l’autorise pas à lui seul.

## 5. Conditions de convergence

Je maintiens le choix du host enfant, le retrait de `frame`, le niveau projet restreint,
les rendus comme fichiers ordinaires, l’absence de TLV HELLO plugin et la livraison étagée.
Il n’est pas nécessaire d’implémenter tout le produit pour accepter l’architecture : il faut
que le petit chemin choisi soit réalisable, que les décisions normatives ne contredisent
pas les exigences, et que les portes expressément préalables soient effectivement passées.

Le prochain tour doit apporter un contrat corrigé et quelques preuves concentrées :
bonjour Rust/Go/TinyGo et paquet signé, bootstrap sandbox sur les deux architectures,
message 4 Mio avec annulation, enfant compromis et compartiments O4, récupération complète
et GC sûr, propositions PD précises. Ajouter les hooks N32–N40 à la matrice normative et
à la traçabilité ; ne pas demander à cette revue d’approuver implicitement le protocole 2.1.

NOT ACCEPTED — Bloquants : R2-1 confinement/runtime/threads non spécifiés de façon exécutable ; R2-2 autorité des compartiments et transport IPC incomplets ; R2-3 ABI Go/TinyGo et octets signés non démontrés ; R2-4 propositions PD-1/PD-2 incompatibles avec les codecs/domaines acceptés ; R2-5 récupération impossible dans 24 mots et format de sauvegarde incohérent ; R2-6 baseline contraire à N35/N37/N39/N40 ; R2-10 porte explicite P-PLUG-BENCH avant acceptation non passée.
