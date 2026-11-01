# Changements de l'architecture

## 1.0.0-brouillon.6 — révision 5 (2026-11-01)

Réponse au tour 4 : [Fable](reviews/07-fable-r4.md) **ACCEPT** (2 majeurs de texte, 5 mineurs) ;
[Codex](reviews/08-codex-r4.md) **NOT ACCEPTED** (1 bloquant, 1 majeur, 1 mineur). Aucune
modification de `docs/protocol/` ni de `docs/design/`.

| Constat | Changement |
|---|---|
| Codex R4-B1 le secret de secours peut rester lisible sur le disque volé | §6.7 **SR-1 à SR-4** : le secret n'est **jamais** sur l'appareil ; en clair seulement en mémoire, à sa création et à une restauration, puis effacé (tampon écrasé, jamais une `string`) ; page verrouillée (`mlock`, `VirtualLock`) quand le système l'accorde ; jamais journalisé ; fenêtre de la phrase exclue des captures quand c'est possible (Windows, macOS ; dit sous Linux) ; l'appareil garde la clé **publique** de secours et des sous-clés de sauvegarde **enveloppées** ; §6.10 **EN-3 / VK-1 à VK-3** : toute voie locale vers la DEK dépend d'un secret absent du disque lisible hors ligne (trousseau, mot de passe Argon2id aux paramètres fixés, ou disque chiffré vérifié), **aucun repli** vers un fichier nu pour un projet protégé ; §6.1 aligné ; P-VAULT : vol hors ligne, configuration et trousseau absents, recherche des octets de clés sur l'image |
| Codex R4-M1 PM-4 arrête le chronomètre avant la peinture | §12.5 PM-4 : trois grandeurs séparées (gestionnaire ≤ 4 ms, attente du rappel, latence jusqu'à l'affichage) ; `requestAnimationFrame` ne prouve pas la peinture ; Event Timing ou capture horodatée externe ; oracle validé par un cas témoin de mise en page lente ; sans oracle, échec |
| Codex R4-m1 résumés anciens | §3.2 : un greffon par hôte ; annexe, Décision 6 : message `inkwell-ext:plugin-package:1\0` ; `decisions.md` : les recommandations initiales remplacées (AD-13, 18, 33, 47, 57, 70, 83, 91, 95, 96) portent la mention « remplacée par la dernière révision, qui seule fait foi » |
| Fable R4-M1 repli « compilé à l'installation » inapplicable sous H-3 | §0.5 : repli = **préchauffage** en tâche de fond après la première peinture (épinglés ou actifs au dernier usage, ≤ 6) avec « Extension en préparation… » ; sinon **revue de H-3 par ADR** (cache disque scellé, haché par le parent, dans le coffre si protégé) |
| Fable R4-M2 « refus d'un autre appareil » contre récupération | §6.10 EN-2 **voie de récupération** : par le destinataire de secours, ancien `peer` accepté et affiché, DEK ré-enveloppée pour le nouvel appareil sous une nouvelle époque ; vecteur « secours sur appareil neuf, époque + 1, ancien appareil refusé ensuite » |
| Fable R4-m1 oracles de « vérifié » | EN-4 : `/proc/swaps`, `dm/uuid` en `CRYPT-`, `lsblk`, `sysctl vm.swapusage` « (encrypted) » (exigé aussi pour le disque en mémoire de macOS), `fdesetup status`, `Win32_EncryptableVolume.ProtectionStatus` ; support faussement annoncé → refus |
| Fable R4-m2 version incomplète dès la création | HI-9 : blobs manquants demandés aux pairs avant publication, sinon « incomplète » dès la création ; épingle de ce qu'on a, le reste cherché ; tests |
| Fable R4-m3 `AD-96` périmé | mention « remplacée » en tête de l'entrée |
| Fable R4-m4 HI-6 et § 15.5 | la personne qui peut écrire l'original **prévisualise** l'intégration comme une proposition et l'applique elle-même ; les règles des auteurs non humains ne la concernent pas |
| Fable R4-m5 borne des sidecars sans chiffre | 256 Mio par compilation, 512 Mio cumulés (admission par le parent), au §0.5 et au §9.5 |

### Désaccords à débattre

Aucun. D-10 à D-16 restent convergés (Codex tour 4 : accord sur l'issue enregistrée).


## 1.0.0-brouillon.5 — révision 4 (2026-11-01)

Réponse au tour 3 : [Fable](reviews/05-fable-r3.md) **ACCEPT** (0 bloquant, 5 majeurs de texte,
15 mineurs) ; [Codex](reviews/06-codex-r3.md) **NOT ACCEPTED** (4 bloquants, 6 majeurs, 1 mineur).
Aucune modification de `docs/protocol/` ni de `docs/design/`.

### Revue Codex, tour 3

| Constat | Changement |
|---|---|
| R3-B1 clair persistant du dossier de travail (macOS, Windows ; swap) | §6.10 EN-4 : dossier de travail **seulement** en mémoire (tmpfs sans swap en clair vérifié ; disque en mémoire sous macOS, à prouver) ou sur un disque chiffré par le système **vérifié** (LUKS, FileVault, BitLocker) ; sinon, pour un projet protégé, aperçu, export lisible et service de langage des fichiers fermés **refusés** sur la cible ; EN-7 réécrit (garantie conditionnée) ; P-VAULT : coupure à chaque étape et **inspection hors ligne avant relance** |
| R3-B2 une DEK de 256 bits n'est pas une clé d'en-tête age | EN-2 : la DEK est la **charge utile d'un fichier age complet** (`filippo.io/age`, stanzas inchangées), clair `{v, project, peer, epoch, dek}`, refus d'un objet d'un autre projet ; vecteurs |
| R3-B3 l'historique promet des blobs que le ménage efface | §6.5 : versions nommées et variantes **racines** du ménage ; §6.11 HI-9 : épingle avant publication du manifeste, moment quelconque sans épingle et dit, purge prioritaire (« version incomplète ») ; tests V1 / remplacement / ménage / restauration |
| R3-B4 portes non mesurables ou contradictoires | §0.5 réécrit : règles E-1 à E-5, oracle et seuil par porte, **repli nommé à l'avance**, 16 installés / 6 actifs / 10 en attente, sidecars dans P-SANDBOX, P-VAULT hors ligne, P-IDX chiffrée, P-FRAME sur chaque moteur, ACP **reporté** (plus une expérimentation) |
| R3-M1 récupération (découverte, espaces de noms, enveloppe des têtes) | §6.7 : `p/<loc>/` dérivé du secret seul, descripteur chiffré (phrase seule + adresse du dorsal), même enveloppe AEAD que les objets, vérification de la signature des têtes, test deux projets / deux appareils |
| R3-M2 « intégrer = une modification » | §6.11 HI-6 : **différence appliquée** (choisie), admission ordinaire, « Annuler » sur le groupe, intégration répétée, zones verrouillées refusées ; fusion native écartée (amendement d'ADR 0031 qu'elle exigerait) ; écart du code au §16 |
| R3-M3 transitions des fichiers non synchronisés | §6.9 IG-5 : table des transitions (jamais admis, texte admis, blob, dépendance du principal) |
| R3-M4 protocole de mesure | §12.5 PM-1 à PM-6 ; §2 RSS/PSS plafonnée, VSZ à part ; Long Tasks ≥ 50 ms seulement |
| R3-M5 autorité et fraîcheur de l'index | §15.18 RG-6 (intersection, outils réduits si contaminé, pas de provenance pas de résultat), RG-7 (file coalescée bornée, 5 s au plus, mise à jour forcée par une requête, invalidation) |
| R3-M6 plafond de nuisance | §11.1 RS-8 : quatre niveaux, chien de garde permanent (500 ms, 2 relevés), limites par classe, réserve du cœur ; RS-3 verbes par classe ; RS-7 corrigé |
| R3-m1 restes | §15.5 `time.read` (jamais 0) ; FM-OPEN-7 ; synthèses `AD-69`, `AD-83`, `AD-87` ; traçabilité U29/U30 ; liste « sous réserve » sans `AD-13`, `AD-18`, `AD-33` |
| Tests demandés en passant | JSON Lines : deux ajouts concurrents donnent deux lignes ; minuterie positive et annulation (ABI-9) ; Vue lecture : tableaux, mathématiques, figures, renvois, sans prétendre à une équivalence générale |

### Revue Fable, tour 3

| Constat | Changement |
|---|---|
| R3-M1 P-IDX non mesurable | §0.5 : corpus, rappel@10 ≥ 0,8, MRR ≥ 0,6, +0,1 pour R1/R2 |
| R3-M2 porte ACP indéfinie | ACP **reporté après v1** (§0.5, ACP-1) |
| R3-M3 repli nommé | colonne « repli » du §0.5 |
| R3-M4 N38 placé en dernier | mesure de référence au **premier lot du MVP** (`coherence.md` §5, §12.5 PM-5) |
| R3-M5 budget du dossier de travail | §6.10 EN-9 : dépendances réelles, ≤ 512 Mio, refus au-delà ; rémanence dans EN-7 |
| R3-M6 sémantique d'« Intégrer » | §6.11 HI-6 (voir Codex R3-M2) |
| m-1 `clone3` « se rabat » | « refus préventif » (H-5) |
| m-2 vérification Landlock masquée par seccomp | H-0 : Landlock vérifié (`EACCES`) **avant** seccomp (`EPERM`) |
| m-3 liste des signatures hors protocole | §9.4 : liste fermée `plugin-package`, `backup-head`, `export`, `enc-key` |
| m-4 C14 périmé | `coherence.md` C14 et §6 point 6 corrigés (`ux-principles.md` dit déjà Ctrl+Maj+P) |
| m-5 `.inkwell/git/` « à décider » | `[cible, AD-13]`, dans le coffre si protégé |
| m-6 bouton puzzle permanent | affiché seulement si une extension tierce ou de projet existe |
| m-7 N34 absent du phasage | v1, lot « Modèles et écriture » |
| m-8 blob non téléchargé annoncé | IG-5 : jamais annoncé |
| m-9 DEK « par appareil » ou « par personne » | **par projet et par appareil** ; enveloppe pour l'appareil et le secours seulement |
| m-10 horloge du voyage dans le temps | HI-10 |
| m-11 budgets manquants | §12.5 : voyage dans le temps, comparaison par mots, motifs, déverrouillage, rechiffrement |
| m-12 rejouer les portes d'enfermement | E-5 |
| m-13 verbes par classe | RS-3, RS-8 |
| m-14 BLAKE3 par architecture | §12.5 |
| m-15 index « chiffré » | RG-3 : dans le dossier de travail pour un projet protégé |
| sur-ingénierie : kit régénéré à chaque sauvegarde | kit **proposé** après 30 jours ou sur geste |
| amélioration : Git dans le coffre | EN-5 : go-git sur `billy.Filesystem` au-dessus du magasin chiffré |

### Désaccords D-10 à D-16 : tous convergés

| # | Décision commune |
|---|---|
| D-10 | un hôte par greffon actif, paresseux, ≤ 6 actifs ; éviction jamais pendant un mandat ou un appel ; repli nommé si la compilation dépasse 500 ms |
| D-11 | démarrage en deux temps (Landlock puis `execve`), seccomp avec `TSYNC` barrière principale ; vérification Landlock avant seccomp |
| D-12 | Rust et Go en 1.0, TinyGo en 1.1 ; minuterie positive et annulation dans le kit ; taille et compilation du module Go consignées |
| D-13 | UTCP retiré |
| D-14 | expérimentations acceptées à leur porte, sans dépendance de production, avec repli nommé |
| D-15 | `beautiful-mermaid` **candidat** de l'API 1.1 derrière P-MERMAID ; Mermaid complet derrière P-FRAME sur chaque moteur |
| D-16 | préfixes externes `inkwell-ext:`, PD-2 retiré, liste fermée au §9.4 |

### Désaccords à débattre

Aucun désaccord ouvert entre les deux revues. Un **choix de conception** est signalé pour la
revue suivante plutôt que débattu : « Intégrer » par différence appliquée (attribution fine
gardée dans la variante) plutôt que par fusion native des opérations (attribution d'origine, mais
amendement d'ADR 0031) ; Fable a noté que Patchwork fait la seconde.


## 1.0.0-brouillon.4 — révision 3 (2026-11-01)

Réponse au tour 2 ([Fable](reviews/03-fable-r2.md) : 4 bloquants, 3 majeurs, 9 mineurs ;
[Codex](reviews/04-codex-r2.md) : 7 bloquants, 5 majeurs, 1 mineur ; les deux « NOT ACCEPTED »),
à l'[audit de cohérence de Codex](reviews/coherence-codex.md) et aux exigences N32 à N41. Nouveau
document : [coherence.md](coherence.md) (contradictions C01–C25, manques M01–M22, doublons, carte
produit, phasage, changements du design). Aucune modification de `docs/protocol/` ni de
`docs/design/`.

### Revue Fable, tour 2

| Constat | Changement |
|---|---|
| R2-B1 (1) Landlock ne couvre pas les fils déjà créés ; poser avant `execve` interdit l'`execve` | §9.10 **H-0 démarrage en deux temps** : binaire hôte **statique** distinct, Landlock posé sur un fil fixé puis `execve` de lui-même (tous les autres fils disparaissent, le domaine est hérité), `TSYNC` en plus quand l'ABI ≥ 8 ; une seule règle (`execute`, `read_file` sur le binaire) |
| R2-B1 (2) liste seccomp qui tue le runtime Go | §9.10 H-5 : familles et refus normatifs, liste blanche **dérivée** par `strace -f` en CI dans `seccomp_linux.go`, `clone` filtré par drapeaux, `clone3` refusé (`ENOSYS`), contrôle d'architecture ; seccomp devient la barrière principale |
| R2-B1 (3) `SOCK_SEQPACKET` contre 4 Mio | §7.3 IPC-1 : **flux d'octets** et cadrage par longueur |
| R2-B2 ABI-3 refuse le bonjour Go ; horloge à 0 | §15.20 ABI-4 : WASI dérivé des importations réelles (Go 1.27, Rust), neutralisé par fonction (`poll_oneoff`, `sched_yield`, `fd_prestat_*`…) ; horloge quantifiée, jamais 0 ; test du kit « `fmt.Println`, `time.Sleep(0)` » ; annexe alignée (fonctions de l'hôte) |
| R2-B3 règle de pli à la réception absente ; `index.json` non sûr sous fusion ; polices hors de la clé | §15.19 PD-1 point 4 (entrée sans effet tant que `format ≥ 1` n'est pas debout dans son passé causal) et vecteur `#plugin-entry-before-format` ; §15.14 `.renders/index.jsonl` en **JSON Lines** ; empreinte des polices dans la clé |
| R2-B4 ST-8 impossible en 24 mots | §6.7 ST-8 : phrase = secret seul ; **kit de secours** versionné ; ST-7 (a) réécrit sur le kit |
| R2-M1 « Ailleurs : — » contre N35 | §9.11 nouveau, port `internal/sandbox`, INV-13, `AD-18` révisé, `AD-94` |
| R2-M2 budget système des hôtes | §2 : ≤ 40 Mio RSS au repos, ≤ 6 actifs, somme ≤ 1,5 Gio, budget global ≤ 2,5 Gio, RSS **et** VSZ |
| R2-M3 consentement IA au premier envoi seulement | §15.18 « Consentement à l'envoi, continu » + indicateur permanent |
| R2-m1 restes de la révision 1 | §4.2 cache disque retiré ; §6.1 secret de secours ; §15.15 « ≤ 45 types » ; traçabilité N22 « règle de flux » |
| R2-m2 deux sens de « compartiment » | un hôte = un greffon : « compartiment » = le greffon et ceux qu'il appelle ; autorité **par connexion** (§7.3 IPC-3) |
| R2-m3 hôtes paresseux | §9.10 H-4 |
| R2-m4 Vue lecture : repli | §15.17 : export HTML expérimental épinglé, repli plan + texte source |
| R2-m5 Tâches en WebAssembly | Tâches redevient **TypeScript** ; Graphviz porte le chemin WebAssembly |
| R2-m6 `compile.run` / `compile.read` | §15.5 : deux lignes |
| R2-m7 deux UX de proposition ; `document.create` invisible | §15.5 : une règle (diff par morceau) ; « appliquer sans relire » réglable par greffon ; créations dans l'Activité |
| R2-m8 AppArmor et sidecars | §9.5 renvoie au profil `inkwell` (H-6) |
| R2-m9 tailles d'ABI | §15.20 ABI-3 : entrée ≤ 1 Mio, réponse ≤ 1 Mio sauf `document.read` ≤ 4 Mio ; vérification contre `memory.size()` |
| §3.4 sur-ingénierie : UTCP, TinyGo dans le banc | UTCP retiré (§15.12) ; banc sur Rust et Go, TinyGo avec le kit 1.1 |
| §4 crochets N32, N33, N34, N35 | §11.1, §15.18 RG-*, §6.9, §9.11 |

### Revue Codex, tour 2

| Constat | Changement |
|---|---|
| R2-1 confinement non exécutable (ABI Landlock, `clone`, threads, ordre) | §9.10 H-0, H-1 (masque par ABI sondée), H-5 ; matrice Ubuntu 22.04, Debian 12, Ubuntu 24.04, Debian 13, Fedora ; P-SANDBOX-LINUX ; enfant natif hostile |
| R2-2 autorité des compartiments, transport | **un hôte par greffon actif** (autorité exacte, attachée à la connexion) ; O1, O4 et greffons de projet chacun dans leur hôte par construction ; §7.3 codec complet (gros-boutiste, types, bornes, crédits en octets, annulation prioritaire, descripteurs ancillaires refusés) ; test 4 Mio avec annulation sans privilège |
| R2-3 ABI Go et TinyGo, octets signés | §15.20 ABI-4 (importations réelles), ABI-3 (sans réentrance, `host_read`), ABI-1 (maximum imposé par l'hôte), ABI-7 (contenu signé canonique, préfixe externe, chemins), ABI-9 (chaînes épinglées ; TinyGo en 1.1) ; vecteurs et modules produits par P-PLUG-PROTO avant l'acceptation de l'hôte |
| R2-4 PD-1 et PD-2 incompatibles avec les codecs et domaines acceptés | PD-1 : base64url d'un CBOR dans le `str16`, schémas complets, émission séparée, aveu de l'intolérance des 2.0 et transition par lecture seule (§1.4.4), vecteurs ; **PD-2 retiré** (préfixes externes `inkwell-ext:`, hors §20.4) |
| R2-5 récupération, nommage, formats, ménage | §6.7 réécrit : secret et kit, nommage et vérification identiques, formats CBOR avec AAD, têtes signées, compteurs par `(ProjectID, PeerID)`, **aucun ménage distant en v1** ; §9.2 ne promet plus la détection universelle |
| R2-6 baseline contraire à N35, N37, N39, N40 | §1.2 (G4, G9, G10, G13), §1.3 (NG1, NG3, NG10), INV-12, INV-13, §6.10, §6.11, §9.11 ; `AD-13`, `AD-18`, `AD-33`, `AD-47` tranchés |
| R2-7 session manuelle saturée, G-0a, versions de diagnostics | MF-12 et MF-21 réécrits (état « non enregistré » par nœud, poignée déchargeable, mémoire de l'historique comptée) ; bandeau G-0a honnête ; LSP-3 (instantané par requête, diagnostics périmés inactifs, `didOpen` des documents non enregistrés, actions de code en proposition atomique) |
| R2-8 MF-25 n'est pas indépendant de l'observateur | MF-25 : toutes les paires concurrentes, recalcul déterministe, réparation seulement par proposition choisie, cas à trois appareils |
| R2-9 IA : consommation, consentement, identité MCP | §15.18 : limites et compteurs, estimation marquée, coût inconnu, consentement continu, contamination monotone, jeton de lancement MCP, MCP désactivé en projet partagé avant PD-1, ACP sans garantie |
| R2-10 P-PLUG-BENCH non passée | §0.5 : l'hôte, l'ABI et l'enfermement deviennent des **expérimentations** acceptées seulement à leur porte (`AD-95`) ; aucun code de production n'en dépend d'ici là |
| R2-11 crochets N32, N33, N34, N37, N39 | §11.1, §15.18, §6.9, §6.10, §6.11 ; traçabilité N1 à N41 |
| R2-12 modèles et outils d'écriture | §6.8 : valeurs typées et échappées, fichiers choisis, caches et clés exclus |

### Audit de cohérence (Codex) et exigences N32 à N41

Tous les constats C01 à C20 et les manques 1 à 12 sont traités dans [coherence.md](coherence.md)
(tables §1 et §2), sauf la proposition DOCX/ODT, **écartée par N40**. Exigences : N32 → §11.1,
`AD-88` ; N33 → §15.18, `AD-89` ; N34 → §6.9, `AD-90` ; N35 → §9.11, `AD-94` ; N36 → coherence.md ;
N37 → §6.10, `AD-91` ; N38 → §12.5, `AD-93` ; N39 → §6.11, `AD-92`, `AD-13` ; N40 → §1.3,
`AD-33`, `AD-98` ; N41 → §6.11, `AD-96`.

Vérifié le 2026-11-01 pour cette révision : `sandbox-exec` déprécié mais fonctionnel, Seatbelt
hérité par les enfants (Apple containerization #737, essais 2026) ; AppContainer sans capacité =
aucun réseau, objet de tâche (mémoire, CPU, nombre de processus) (Chromium `sandbox/win`) ; go-git v6
en alpha, CVE-2026-45022 corrigée en 5.19.0 (traqueur Debian) ; Patchwork (Ink & Switch). Restent
**à prouver** : P-SANDBOX-MAC, P-SANDBOX-WIN, P-VAULT, P-IDX, P-GIT-BENCH.

### Désaccords à débattre (tour 3)

| # | Sujet | Fable | Codex | Recommandation |
|---|---|---|---|---|
| D-10 | Combien d'hôtes de greffons | un hôte par **classe de sortie**, paresseux, ≤ 6 (D-5 du tour 2) | **autorité commune exacte** : un colocataire natif compromis ne peut pas être distingué (R2-2) | **un hôte par greffon actif**, paresseux, ≤ 6 actifs : satisfait l'exigence d'autorité de Codex et la borne de coût de Fable ; P-PLUG-BENCH mesure le RSS par hôte |
| D-11 | Comment poser Landlock | binaire statique lancé par le chemin `setpriv` des sidecars, une règle, `TSYNC` si ABI ≥ 8 | composition exacte écrite, sans réutiliser tel quel le lanceur des sidecars | **démarrage en deux temps** (Landlock sur un fil puis `execve` de soi) : ne dépend pas d'un `setpriv` récent (Ubuntu 24.04 a util-linux 2.39) ; seccomp avec `TSYNC` (disponible depuis Linux 3.17) comme barrière principale |
| D-12 | TinyGo | peut attendre le kit 1.1 | profil Rust, Go **et** TinyGo épinglé maintenant | Rust et Go en 1.0 ; TinyGo en 1.1, sa table d'importations établie par mesure |
| D-13 | UTCP | retirer de la spec (sur-ingénierie) | export seul, choix accepté | **retirer** (§15.12 « écarté, réexaminer si ») |
| D-14 | Preuves avant acceptation | bloquants « de texte » ; le banc reste une porte | le banc est expressément préalable : le passer ou rétrograder la décision | **rétrograder en expérimentation** (§0.5) : la spec s'accepte, l'hôte attend ses preuves |
| D-15 | Mermaid | `beautiful-mermaid` (sans DOM) compilé par le kit JavaScript, `renderer` `wasm` en API 1.1 | différé, aucun résultat P-FRAME ne justifie un retour | **sous-ensemble sans DOM en 1.1** (pas de P-FRAME, pas de DOM), Mermaid complet derrière P-FRAME ; à confirmer par Codex |
| D-16 | Domaines de signature des paquets et têtes | les inscrire au registre (tour 1, B-2) | la grammaire du §20.4 interdit `/` ; signatures externes | **préfixes externes `inkwell-ext:`**, PD-2 retiré |

Convergences constatées (aucun débat) : D-1 (noyau d'ABI ici, interfaces ailleurs), D-3 (rendus
fichiers ordinaires), D-4 (pas de TLV), D-6 (MF-25), D-7 (Vue lecture), D-8 (`Runtime` par projet,
un projet ouvert en v1, décision de l'utilisatrice), D-9 (GREASE dans `host.features()`).

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
