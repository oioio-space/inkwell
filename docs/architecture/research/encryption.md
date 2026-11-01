# Étude — Chiffrement du projet (N37)

*Date : 2026-11-01. Statut : étude, rien n'est décidé (ADR à écrire). Ressources lues : `CLAUDE.md`
(invariants), `docs/architecture/spec.md` §6.7 (dorsal, ST-1 à ST-8), §9.4–9.5 (clés, F5), NG3 ;
`docs/protocol/spec.md` §3–4 (clés Ed25519, ops de membre) ; `docs/exigences.md` N22, N33, N35, N37.*

**Convention de preuve.** `[lu]` = page lue pendant cette étude (URL + date). `[mémoire]` = connaissance
générale non revérifiée : **à confirmer avant ADR**. Rien n'est affirmé sans l'une des deux marques.

## 0. Résumé et recommandation

> **Ne pas monter de système de fichiers.** Chiffrer **chaque objet** du magasin existant (`IKV1` +
> blobs BLAKE3) avec une **clé de projet** (DEK) aléatoire ; envelopper cette clé pour chaque appareil
> membre au **format d'en-tête age** ; l'**annuaire** est la liste de membres signée, enrichie d'une clé
> de chiffrement X25519 **séparée** par appareil. Export = **un fichier** : zip lisible, ou ce zip
> chiffré en **age** (lisible sans inkwell par `age -d | unzip`).

| Brique | Choix | Écarté |
|---|---|---|
| Conteneur au repos | magasin d'objets chiffrés (modèle restic/Kopia, déjà §6.7) | FUSE/macFUSE/WinFsp, bloc monolithique |
| Chiffrement d'objet | XChaCha20-Poly1305, nonce 192 bits aléatoire, AAD liant projet/nom/époque (réutilise ST-4) | AES-GCM nonce 96 bits aléatoire |
| Enveloppe de clé | en-tête age (stanza X25519), un destinataire par appareil + un destinataire « récupération » | MLS, HPKE écrit à la main |
| Clés | X25519 dédiée, liée par signature Ed25519 (`inkwell/enc-key/v1`) | dérivation Ed25519→X25519 |
| Retrait d'un membre | **époque** : nouvelle DEK pour les écritures futures (paresseux) ; ré-chiffrement complet sur demande | rotation obligatoire immédiate |
| Déverrouillage local | trousseau du système, repli mot de passe ; récupération par phrase (ST-8 étendue) | clé en clair à côté des données |
| Exécution | matérialisation minimale dans un dossier de travail privé ; tmpfs sous Linux | pilote noyau |
| Export | zip (+ `age`), signé, AEAD par blocs de 64 Kio (STREAM) | zip « chiffré », conteneur propriétaire |

**Honnêteté sur la portée.** Ce chiffrement protège un **appareil éteint/volé**, une **copie
sauvegardée** (§6.7) et un **export transporté**. Il ne protège pas une session ouverte dont l'appareil
est compromis (le clair vit en mémoire et, pour Typst, dans le dossier de travail). Il ne retire rien à
un membre exclu de ce qu'il a déjà vu (§3.4). NG3 (« pas de chiffrement au repos en v1 ») est **levée**
par N37 : ADR de remplacement à écrire.

## 1. Grille fixe d'évaluation

Six critères, note 0–3 (3 = meilleur). **S** sécurité (norme citée), **Sim** simplicité pour un non-expert,
**X** multiplateforme (N35 : Linux/macOS/Windows × amd64/arm64), **P** performance gros projets,
**I** adéquation aux invariants, **M** maturité. Les notes sont un jugement de l'auteur, appuyé par les faits listés ;
elles ne sont pas mesurées (aucun banc exécuté : à mesurer, `go-benchmark`, avant ADR).

### 1.1 Conteneurs et formats

| Option | S | Sim | X | P | I | M | Faits clés et verdict |
|---|---|---|---|---|---|---|---|
| **gocryptfs** | 3 | 1 | 1 | 3 | 1 | 3 | AES-256-GCM par blocs de 4 Kio, noms EME, HKDF-SHA256, clé maître sous scrypt `[lu]` [nuetzlich.net/gocryptfs/forward_mode_crypto](https://nuetzlich.net/gocryptfs/forward_mode_crypto/), 2026-11-01. FUSE obligatoire ; Windows via un autre projet (cppcryptfs) `[mémoire]`. **Écarté** : noyau + une seule plateforme native. |
| **Cryptomator Vault Format 8** | 3 | 2 | 2 | 2 | 2 | 3 | Clé maître 256+256 bits ; mot de passe → scrypt → KEK → AES Key Wrap (RFC 3394) ; `vault.cryptomator` = JWT signé ; `SIV_GCM` ; frontaux WinFsp, macFUSE, FUSE, repli WebDAV loopback `[lu]` [docs.cryptomator.org/security/architecture](https://docs.cryptomator.org/security/architecture/), 2026-11-01. Hub : JWE ECDH-ES (P-384) par utilisateur `[lu]` — même idée que notre enveloppe par membre. AES-SIV des noms, blocs de 32 Kio : `[mémoire]`. **Inspirant** (une enveloppe par personne, version-MAC anti-rétrogradation), **non adopté** (VFS = pilote ou WebDAV local, contraire à l'invariant « aucun port local », ADR 0009). |
| **restic** | 3 | 2 | 3 | 2 | 3 | 3 | AES-256-CTR + Poly1305-AES, scrypt, **un fichier de clé par mot de passe** (changer le mot de passe ne ré-écrit que ce fichier), blobs 512 Kio–8 Mio, IDs = SHA-256 du clair `[lu]` [restic.readthedocs.io/en/stable/100_references](https://restic.readthedocs.io/en/stable/100_references.html), 2026-11-01. Fuite reconnue : forme/volume ; clé fuitée ⇒ « tout le dépôt est ré-chiffré ». **Modèle structurel retenu** (objets opaques + enveloppes de clé multiples), primitives non reprises (CTR+MAC maison). |
| **Kopia** | 3 | 2 | 3 | 3 | 3 | 3 | Chiffrement enveloppe ; AES-256-GCM-HMAC-SHA256 ou ChaCha20-Poly1305 par défaut ; scrypt pour le bloc de format ; contenu adressé par hash `[lu]` [kopia.io/docs/advanced/encryption](https://kopia.io/docs/advanced/encryption/), 2026-11-01. Détail de l'usage du HMAC contre la fuite de convergence : **non confirmé**. Écrit en Go : bon prior art. |
| **rclone crypt** | 2 | 2 | 3 | 2 | 2 | 3 | NaCl secretbox par blocs de 64 Kio, noms EME `[mémoire]`. Pas de multi-destinataires ni de signature. **Écarté** (mono-clé de mot de passe). |
| **SQLCipher** | 2 | 2 | 1 | 2 | 0 | 3 | Chiffre une base SQLite (pages) `[mémoire]` ; cgo ; inkwell n'a pas de SQLite comme source de vérité. **Écarté.** |
| **Tink** | 3 | 1 | 3 | 3 | 2 | 3 | Bibliothèque de primitives (streaming AEAD, HPKE) `[mémoire]` ; API pour développeurs, aucun format de fichier partageable avec des outils tiers. **Écarté** : age fait mieux pour l'export. |
| **age** (`filippo.io/age`) | 3 | 3 | 3 | 3 | 3 | 3 | Format spécifié (C2SP) : clé de fichier 128 bits enveloppée par destinataire, MAC d'en-tête HMAC-SHA-256, charge utile STREAM en blocs de 64 Kio, ChaCha20-Poly1305, HKDF-SHA-256 `[lu]` [c2sp.org age.md](https://github.com/C2SP/C2SP/blob/main/age.md), 2026-11-01. Go pur ; v1.3.2 (29 août 2026), `DecryptReaderAt` (utile avec `zip.NewReader`), destinataires hybrides post-quantiques depuis v1.3.0, paquet `plugin` `[lu]` [pkg.go.dev/filippo.io/age](https://pkg.go.dev/filippo.io/age), 2026-11-01. **Retenu** pour enveloppes et export. |

### 1.2 Monter (FUSE) ou rester dans le processus

- **macOS** : macFUSE 5.x sait tourner sans extension noyau via FSKit sur macOS 26 ; le fond FSKit est « encore jeune », FSKit n'égale pas FUSE3 (verrous, xattr, ioctl, fallocate absents, avis d'un développeur, juin 2026) `[lu]` résultats de recherche : [github.com/macfuse/macfuse/releases](https://github.com/macfuse/macfuse/releases), [forums Apple](https://origin-devforums.apple.com/forums/thread/831338), 2026-11-01. FSKit exige macOS ≥ 15.4 `[lu]` [developer.apple.com/documentation/fskit](https://developer.apple.com/documentation/fskit.md). macOS < 26 : extension noyau + validation utilisateur `[mémoire]`.
- **Windows** : WinFsp est un pilote à installer (MSI, droits administrateur) `[mémoire]`.
- **Linux** : FUSE3 + `fusermount3` (setuid), absent de certains conteneurs/Flatpak par défaut `[mémoire]`.
- **Verdict** : un pilote tiers à installer contredit « extrêmement simple » et « autonome » (but final : exécutable autonome, deb/rpm). **Aucun montage.** La webview ne lit jamais de fichiers (le CRDT est en mémoire) ; seul Typst en lit (§5). Le « système de fichiers virtuel » demandé par N37 est donc **dans le processus** : l'API du magasin (§6.7 ST-1) ; le pont `io/fs` existant sert Typst par matérialisation.

## 2. Clés

### 2.1 Primitives : standards et choix

| Besoin | Choix | Standard / source |
|---|---|---|
| AEAD d'objet | XChaCha20-Poly1305 (déjà ST-4) | ChaCha20-Poly1305 : RFC 8439 `[mémoire]` ; X- : brouillon IETF `[mémoire]`. Raison : nonce aléatoire 192 bits sans suivi de compteur, alors que NIST SP 800-38D borne les IV aléatoires de 96 bits à 2^32 invocations par clé `[mémoire]` ; un projet de plusieurs millions d'objets sous une même DEK reste loin de la limite mais la marge est gratuite. AES-256-GCM resterait le repli « conformité NIST/FIPS ». |
| Échange de clé | X25519 | RFC 7748 `[mémoire]` ; rejet du secret partagé nul (age le prescrit `[lu]`). |
| Enveloppe | stanza X25519 d'age (HKDF-SHA-256, ChaCha20-Poly1305) | spec age `[lu]`. Équivalent fonctionnel de HPKE mode base (RFC 9180). |
| Dérivation | HKDF-SHA-256, domaines séparés | RFC 5869 `[mémoire]` ; NIST SP 800-57 (séparation des usages de clé) `[mémoire]`. |
| Mot de passe | scrypt (age) ou Argon2id | RFC 7914 / RFC 9106 `[mémoire]` ; age impose un destinataire scrypt **seul** `[lu]`. |

**HPKE, `crypto/hpke` de Go.** Go 1.26 ajoute un paquet `crypto/hpke` (RFC 9180) avec KEM hybrides
ML-KEM-768/X-Wing issus de `draft-ietf-hpke-pq` `[lu]` (résultats : [pkg.go.dev/crypto/hpke@go1.26.7](https://pkg.go.dev/crypto/hpke@go1.26.7), notes de version go1.26 en brouillon, 2026-11-01 ; notes finales non relues). HPKE chiffre **un message vers un destinataire** : bon pour envelopper une clé de 32 octets, pas pour les fichiers. Comme age fournit déjà l'enveloppe, le STREAM, l'outil CLI et les plugins, HPKE direct n'apporte rien sauf si la version Go du projet n'admet pas age (vérifier `go.mod`) : **repli documenté, pas choix**.

**Post-quantique.** age ≥ 1.3 propose un destinataire hybride ML-KEM-768+X25519 (`age1pq…`) ; sa spec déconseille de mêler destinataires PQ et non-PQ dans un même fichier `[lu]`. Le risque « récolter maintenant, déchiffrer plus tard » vise surtout les **copies hors appareil** (sauvegarde N22, exports). Recommandation : champ `suite` (entier) dans toutes les enveloppes dès la v1 pour migrer ; **X25519 par défaut en v1** (clés de 32 octets dans les ops de membre ; les clés hybrides sont bien plus grandes — taille exacte non vérifiée — ce qui touche les bornes du protocole) ; décision PQ renvoyée à l'ADR (question ouverte Q-2).

### 2.2 Dériver X25519 de l'Ed25519 existant, ou clé séparée ?

- Faisable : la clé publique Ed25519 se convertit en X25519 (u = (1+y)/(1−y)), le secret est le scalaire SHA-512 tronqué ; age le fait pour les clés SSH. Filippo Valsorda, 18 mai 2019 : il ne voit pas d'attaque mais tient les **attaques inter-protocoles pour une question ouverte** (un oracle de déchiffrement partiel pourrait aider à forger une signature) `[lu]` [words.filippo.io/using-ed25519-keys-for-encryption](https://words.filippo.io/using-ed25519-keys-for-encryption/), 2026-11-01. libsodium : la conversion existe mais « des clés distinctes restent fortement recommandées » ; ePrint 2021/509 prouve la sécurité conjointe dans le modèle de l'oracle aléatoire pour une construction précise `[lu]` résultats de recherche, 2026-11-01 ([doc.libsodium.org](https://doc.libsodium.org/advanced/ed25519-curve25519), [eprint.iacr.org/2021/509](https://eprint.iacr.org/2021/509)).
- **Spécifique à inkwell** : la spec protocole impose des règles strictes sur les clés Ed25519 (ordre premier, pas de composante de petit ordre, §3.2) ; la conversion ajouterait des cas limites (point d'ordre mixte → u invalide). Et une clé d'identité est **durable et signe tout** ; la clé de chiffrement, elle, doit pouvoir être **remplacée** sans changer d'identité.
- **Recommandation : clé X25519 séparée** par appareil (`enc.key`, 0600 ou protégée, §4.3), publiée dans l'annuaire avec une signature Ed25519 de domaine `inkwell/enc-key/v1` sur `(ProjectID, clé Ed25519, clé X25519, suite, date)`. Coût : un champ de plus dans l'op de membre v2 ou dans l'annonce d'appareil (changement de protocole **additif**, TLV/clé de map ouverte, §1.4 du protocole). Repli si on refuse tout changement de protocole : conversion, à documenter comme risque accepté.

### 2.3 Hiérarchie : DEK enveloppée par KEK

```
phrase de récupération ─┐                               ┌─ HKDF(DEK,"obj-aead/v1")  objets
clé X25519 appareil A ──┼─ stanzas age ─► DEK(époque e) ┼─ HKDF(DEK,"obj-name/v1")  noms BLAKE3 à clé
clé X25519 appareil B ──┘   (trousseau.age)             └─ HKDF(DEK,"index/v1")     index, RAG (§7)
```

- **DEK** : 256 bits, `crypto/rand`, **par projet et par époque**. Le « trousseau » (`keyring/<époque>.age`) est un en-tête age dont chaque stanza enveloppe la DEK pour un destinataire ; il est public (il ne révèle que le nombre de destinataires et leurs clés publiques, déjà dans l'annuaire).
- Ajouter un membre = ajouter une stanza (réécriture d'un petit fichier, pas des données). Changer le mot de passe local = réécrire l'enveloppe locale seulement (modèle restic `[lu]`).
- La clé de stockage de la sauvegarde distante (ST-3) **devient** une sous-clé de la DEK de la personne… *attention* : ST-3 veut une clé « par projet et par personne » qui ne passe pas par les pairs, pour que le dorsal reste un coffre **personnel**. On garde donc deux clés : la DEK de projet (partagée, disque local et pairs) et la clé de sauvegarde (personnelle). Un ancien membre ne peut ainsi pas lire une sauvegarde plus récente que son retrait. *Question Q-3 : fusionner ou non.*

### 2.4 Retrait d'un membre : paresseux ou immédiat ?

Fait structurant : un membre exclu **a déjà vu** tout le clair qu'il a synchronisé (modèle byzantin complet, R7 ; l'architecture dit déjà qu'il n'existe aucune clé symétrique de groupe sur le réseau, §9.4). Le chiffrement au repos ne change donc pas la confidentialité *rétroactive* ; il protège le *futur* et le *support*.

| Stratégie | Effet | Coût | Verdict |
|---|---|---|---|
| **Paresseuse par époque** (défaut) | l'admin émet une op « époque e+1 » ; nouvelle DEK enveloppée pour les membres restants ; les écritures futures utilisent e+1 ; les anciens objets restent sous e (réencapsulés au fil de l'eau ou jamais) | O(membres) | **Retenue**. Un exclu qui ne possède que le disque d'un ancien appareil lit le passé qu'il avait déjà, pas l'avenir. |
| **Immédiate (ré-chiffrement complet)** | tous les objets sous e+1 ; l'ancienne DEK détruite | O(taille du projet), I/O et énergie | bouton « Tout rechiffrer » (cas de compromission d'appareil) ; tâche de fond reprenable. |

- L'époque s'inscrit dans la liste signée ; **concurrence** : deux admins qui changent d'époque ensemble produisent e+1 et e+1' ; règle déterministe : même ordre `≺` que le pli des membres (§4.9 du protocole) ; un appareil garde **toutes** les DEK d'époque reçues. À spécifier avec le pli (non fait ici).
- La révocation du protocole « bat » une octroi concurrent (§4.9) ; l'époque suit la même logique (le retrait gagne : aucune stanza pour la clé retirée).

### 2.5 MLS (RFC 9420) : trop ?

MLS offre secret de confidentialité persistante (FS) et sécurité après compromission (PCS) pour des groupes de 2 à des milliers de membres, en asynchrone, avec un Service d'Authentification **de confiance** et un Service de Livraison peu fiable `[lu]` [rfc-editor.org/rfc/rfc9420](https://www.rfc-editor.org/rfc/rfc9420.html), RFC de juillet 2023, 2026-11-01. Le besoin ici est **différent** : chiffrement de données **au repos**, groupes de quelques personnes, sans serveur. Les Commits MLS ont besoin d'un ordre commun de changements d'état (§14 « Sequencing », texte **non lu** : à vérifier) que notre réseau sans serveur ne fournit pas ; l'AS de confiance n'existe pas ici (la liste signée byzantine en tient lieu). PCS sur des données déjà synchronisées n'a pas de sens (§2.4). **Surdimensionné** ; à rouvrir seulement si inkwell ajoute un canal de messagerie chiffré durable.

## 3. L'annuaire

### 3.1 La liste de membres signée, annuaire de clés

Elle existe (ops Ed25519 à domaine séparé, pli déterministe, §4 du protocole) et distribue déjà l'identité ; on y ajoute la clé X25519 (§2.2). Rien d'autre à créer : **une seule source de vérité** (invariant). Le dossier d'échange demandé par N37 est donc, pour la personne, « les personnes du projet » ; pour un nouvel arrivant, l'invitation Wormhole (ADR 0032) transporte déjà l'authentification hors-bande.

### 3.2 Confiance à la première rencontre et empreintes en langage clair

- L'invitation par code partagé est de l'authentification hors-bande (SPAKE2, §9.4) : la clé est vérifiée **au moment de l'ajout**, sans TOFU aveugle.
- Empreinte : la spec emploie déjà « empreinte en 4 mots » pour les greffons (§15). La réutiliser : « Vérifiez avec Camille que vous voyez : *prune – tigre – orage – sable* ». Même vocabulaire que Signal (numéros de sécurité) `[mémoire]`, sans chiffres hexadécimaux.
- Changement de clé d'une personne = événement visible (« La clé de Camille a changé »), jamais silencieux.

### 3.3 Transparence des clés

La liste signée est déjà un **journal append-only vérifiable** par chaque membre ; la transparence de clés classique (CONIKS, Key Transparency de Google, Sigstore Rekor) `[mémoire]` répond au problème d'un serveur d'annuaire menteur, qui n'existe pas ici. Le risque résiduel est l'**équivoque** (deux vues de la liste) : déjà traité (ADR 0031, auto-équivoque). **Rien de plus à bâtir.**

### 3.4 Annuaires d'entreprise (optionnel, extension)

Pour *trouver* la clé d'un collègue avant l'invitation : extension `directory` (LDAP/AD : attribut de certificat ou de clé publique ; ou recherche de type WKD par courriel `https://domaine/.well-known/…`, conçue pour OpenPGP, statut de la spécification non revérifié `[mémoire]`). Règle : le résultat est une **suggestion** présentée avec son empreinte à confirmer ; elle ne rend jamais un membre de confiance. Hors v1.

## 4. Utilisabilité

### 4.1 Ce que font les autres

| Produit | Modèle | Leçon retenue | Source |
|---|---|---|---|
| Keybase | chaînes de signatures publiques, appareils comme sous-clés | l'idée « appareils d'une même personne » = notre groupe d'appareils (ADR 0017) ; le service s'est arrêté en pratique après le rachat par Zoom (2020) | `[mémoire]` |
| Signal | numéros de sécurité, vérification par QR | comparer hors-bande, **peu** de gestes | `[mémoire]` |
| 1Password | mot de passe + Secret Key de 128 bits (2SKD), « Emergency Kit » imprimable | la personne **imprime** un kit de récupération ; la clé n'est pas sur le serveur | `[mémoire]` : à citer depuis le livre blanc avant ADR |
| Proton Drive | clés OpenPGP par adresse, phrase/fichier de récupération | récupération explicite proposée à l'inscription | `[mémoire]` |
| Tresorit | partage zéro-connaissance par invitation | invitation = ajout de clé | `[mémoire]` |

### 4.2 Parcours cible (zéro jargon, glossaire)

- **Créer** un projet : une case « Protéger ce projet » **cochée par défaut** pour un projet partagé (aucun mot de passe à inventer si le trousseau du système est disponible). UI : « personne » et « appareil », jamais « clé », « DEK », « époque ».
- **Inviter** : inchangé (code) ; l'invitée voit « Ce projet est protégé. Il sera lisible seulement sur vos appareils et ceux des personnes invitées. »
- **Retirer quelqu'un** : « Camille ne recevra plus les prochaines modifications. » + lien « Elle a perdu son appareil ? Protéger davantage » → ré-chiffrement complet (§2.4).
- **Perte d'appareil** : sur un autre appareil de la personne ou d'un collègue, « Ajouter un nouvel appareil » (stanza supplémentaire) ; l'appareil perdu est retiré.
- **Récupération** : la **phrase de 24 mots (ST-8)** existe déjà pour la sauvegarde ; l'étendre à un **destinataire de récupération** de chaque projet de la personne (clé X25519 dérivée de la phrase par HKDF, stanza ordinaire du trousseau). Une seule feuille à imprimer, déjà annoncée par l'application. Sans elle ni autre appareil ni collègue : **les données sont perdues**, et l'écran le dit en clair à l'activation (honnêteté à la 1Password).
- **Déverrouillage local** : trousseau du système par défaut ; bibliothèques Go sans cgo (Secret Service par D-Bus, `security` sur macOS, Credential Manager sur Windows) existent, p. ex. `zalando/go-keyring` `[mémoire]` — **à auditer** ; la spec a écarté le trousseau en v1 « pour dépendance, cgo ou D-Bus » et prévoit de le réexaminer avec ce chiffrement (§9.4). Repli : mot de passe de l'appareil (scrypt/Argon2id). Option avancée : jeton matériel via plugin age (YubiKey, TPM : l'existence et le support Windows/arm64 de ces plugins ne sont **pas vérifiés** ; le protocole de plugin lui-même est spécifié `[lu]` [c2sp.org/age-plugin](https://c2sp.org/age-plugin), 2026-11-01).

## 5. Exécution : Typst et tinymist ont besoin de fichiers

### 5.1 Options

| Option | Linux | macOS | Windows | Verdict |
|---|---|---|---|---|
| tmpfs privé (déjà F5, ADR 0033) | oui, sans pilote | — | — | **garder** sous Linux |
| Disque RAM | — | `hdiutil attach -nomount ram://N` puis formatage : réalisable sans pilote tiers `[mémoire]`, droits non vérifiés | **aucune** méthode sans pilote : l'iSCSI/ImDisk sont des pilotes ; MS signale ImDisk instable sous Windows 11 `[lu]` (recherche : [learn.microsoft.com … ram-disk](https://learn.microsoft.com/cs-cz/archive/blogs/windowsinternals/how-to-create-a-ram-disk-in-windows-server), 2026-11-01) | macOS = piste ; Windows = exclu |
| Dossier de travail privé (ACL utilisateur seul) | oui | oui | oui | **base commune**, §5.2 |
| Surcouche mémoire LSP vers tinymist | oui | oui | oui | tinymist garde les sources modifiées dans une table `memory_changes` alimentée par `didOpen/didChange` `[lu]` ([docs.rs tinymist 0.14.18 `src/input.rs`](https://docs.rs/crate/tinymist/latest/source/src/input.rs), 2026-11-01) : le **texte** (`.typ`, `.md`) peut ne jamais toucher le disque pour l'édition. Les **binaires** (images, polices, `import` non ouverts) sont lus sur disque : couverture exacte **non vérifiée**. |
| Moniteur du monde Typst personnalisé (trait `World`, FS virtuel en mémoire) | oui | oui | oui | exige notre **propre binaire Rust** : brise « typst épinglé SHA-256 » (invariant, ADR 0024). `[mémoire]` sur l'API ; à évaluer en v2. |
| Windows ProjFS (fournisseur utilisateur du système) | — | — | fonctionnalité optionnelle livrée avec Windows `[mémoire]`, fichiers « hydratés » sur disque | non sans fuite ; à étudier v2 |
| FUSE / FSKit / WinFsp | | | | **exclus** (§1.2) |

### 5.2 Recommandation d'exécution

1. **Matérialisation minimale à la demande** : seuls les fichiers que Typst lit (cible de compilation + fichiers référencés) sont déchiffrés dans le dossier de travail, supprimés à la fermeture du projet et **balayés au démarrage** (plantage). Texte vers tinymist par surcouche LSP dès que la couverture est vérifiée (mesure à faire).
2. Emplacement : Linux = tmpfs privé existant ; macOS = `$TMPDIR` utilisateur (hors sauvegarde Time Machine `[mémoire]`) ou disque RAM ; Windows = `%LOCALAPPDATA%\inkwell\work\<id>`, ACL propriétaire seul, attributs `FILE_ATTRIBUTE_TEMPORARY` + `NOT_CONTENT_INDEXED` `[mémoire]`.
3. Sortie bornée (§9.5) déjà là ; sous Windows et macOS « lancement borné refusé » : à reconsidérer car le dossier de travail y devient indispensable (**interaction avec F5 à trancher**, Q-4).

### 5.3 Ce qui fuit, et l'atténuation

| Fuite | Atténuation | Résiduel |
|---|---|---|
| **Swap / hibernation** | mémoire sensible : `mlock`/`VirtualLock` limités (quotas) ; tmpfs *peut* être swappé (`[mémoire]`) ; recommander swap chiffré/zram, chiffrement du disque par le système (déjà recommandé NG3) | hibernation : non maîtrisable par l'application |
| **Vidages sur plantage** | `RLIMIT_CORE=0`, `PR_SET_DUMPABLE=0` (Linux), `MiniDump` désactivé / WER exclu (Windows), `RLIMIT_CORE` (macOS) `[mémoire]` pour tous | un débogueur avec droits de la personne |
| **Vignettes et aperçus** (Explorateur, QuickLook, miniatures GNOME) | dossier hors des chemins indexés, fichiers temporaires sans prévisualisation ; aperçus PDF rendus **dans** l'application | export vers dossier choisi : la personne décide |
| **Indexeur du système** (Windows Search, Spotlight, Tracker/Baloo) | attribut NOT_CONTENT_INDEXED ; macOS : nom de dossier en `.noindex` / fichier `.metadata_never_index` ; Linux : `.trackerignore`/emplacement exclu `[mémoire]` pour tous | à tester (skill `ux-walkthrough`, lot e2e) |
| **Fichiers récents, journaux, rapports de plantage** | aucune donnée de document dans les journaux (déjà §2 sécurité) ; chemin du dossier de travail non journalisé | titres de projet dans la barre de fenêtre |
| **Sauvegardes/synchro du système** (OneDrive, Time Machine, iCloud) | dossier de travail hors dossiers synchronisés | un dossier de projet choisi par la personne dans un nuage reste du **chiffré opaque** : c'est le bénéfice N37 |
| **Mémoire de la webview** | irréductible (CRDT en clair en mémoire) | accepté |

## 6. Export en un seul fichier

### 6.1 Deux contenus, deux enveloppes

| | Clair | Chiffré |
|---|---|---|
| **Copie lisible** (projection : fichiers `.typ`/`.md` + ressources, disposition Typst) | `projet.zip` | `projet.zip.age` |
| **Archive complète** (historique, membres, état Automerge opaque) | `projet.inkwell.zip` (avertissement : contient l'historique) | `projet.inkwell.zip.age` |

- Conteneur : **zip** (zip64 pour > 4 Gio), pas de zip « chiffré » (ZipCrypto cassé ; AES-zip peu portable `[mémoire]`) ; chiffrement **par age autour du zip** : recouvrable sans inkwell : `age -d -i clé.txt projet.zip.age > projet.zip && unzip projet.zip`. Pour un destinataire par mot de passe : `age -d projet.zip.age` demande le mot de passe `[lu]` (scrypt, destinataire unique) [spec age](https://github.com/C2SP/C2SP/blob/main/age.md).
- **Destinataires** : (a) « avec un mot de passe » (le plus simple pour envoyer à quelqu'un hors d'inkwell) ; (b) « pour les personnes du projet » (stanzas X25519 de l'annuaire, §3) ; (c) la phrase de récupération. Pas de mélange mot de passe + clés (limite d'age `[lu]`).
- **Lecture en flux** : `DecryptReaderAt` donne un `io.ReaderAt` + taille, donc `zip.NewReader` lit **sans tout déchiffrer en mémoire** `[lu]` [pkg.go.dev/filippo.io/age](https://pkg.go.dev/filippo.io/age).

### 6.2 STREAM, intégrité, authenticité

- STREAM (Hoang, Reyhanitabar, Rogaway, Vizár, 2015, ePrint 2015/189 `[mémoire]`) : blocs de 64 Kio, nonce = compteur ‖ drapeau « dernier bloc » ; **troncature détectée** (le flux sans dernier bloc valide échoue) `[lu]` spec age. En-tête protégé par HMAC-SHA-256 `[lu]`.
- age n'authentifie **pas l'expéditeur** `[mémoire — conception d'age : chiffrement non signé]`. Remède : le zip contient `MANIFEST.cbor` (liste + BLAKE3 de chaque entrée) **signé Ed25519** (domaine `inkwell/export/v1`) par l'exportateur ; à l'import l'application affiche « Exporté par Camille le 3 mars » (ou « signature absente » pour un zip clair). **Mais** : tout octet importé est de toute façon revérifié (signature, rôle, décodeur fuzzé, invariant) ; la signature sert à l'attribution, pas à la sécurité du moteur.
- Le clair n'est jamais écrit avant d'être déchiffré : écriture vers un `.part`, renommage atomique.
- Même enveloppe pour l'**objet de sauvegarde** du §6.7 ? Non : l'objet reste XChaCha avec nom opaque (ST-4) ; seul l'export est lisible par des outils tiers.

## 7. Composition avec le reste

### 7.1 Magasin BLAKE3 : convergence

Le chiffrement convergent (clé dérivée du contenu) révèle l'égalité de contenus à quiconque tient le chiffré et un candidat (attaque de confirmation de fichier). restic nomme ses blobs par SHA-256 du **clair** `[lu]` — fuite d'existence connue de ce modèle. **Décision : éviter.** Le nom sur disque = `BLAKE3-à-clé(sous-clé obj-name, clair)` (déjà ST-4) : stable **dans** un projet (déduplication préservée), opaque **entre** projets et pour qui n'a pas la DEK. L'identifiant public du protocole (BLAKE3 du clair, échangé entre membres) reste dans l'objet chiffré et dans l'index chiffré ; il n'est jamais le nom de fichier. Résiduel assumé : tailles et nombre d'objets visibles (comme ST-7, pas de rembourrage v1) ; blocs découpés à taille fixe de 1 Mio (ST-4) plutôt que par contenu : évite la fuite du polynôme du découpeur de restic `[lu]`.

### 7.2 Synchronisation pair à pair

Les pairs échangent déjà du **clair** protocolaire sur un transport chiffré (QUIC/TLS 1.3 ou Noise, §9.4 ; aucune clé de groupe). **Ne pas échanger du chiffré-au-repos** : chaque appareil chiffre avec ses propres époques ; deux appareils n'ont pas à partager les mêmes noms d'objets, et un pair malveillant ne reçoit ainsi aucun texte chiffré sous une DEK partagée à attaquer. Conséquence : le chiffrement au repos est **local à l'appareil** (la DEK n'a pas besoin d'être identique d'un membre à l'autre !). *Simplification majeure à retenir* : **DEK par appareil**, enveloppée pour cet appareil + récupération ; pas d'époque partagée entre personnes, pas de rotation au retrait d'un membre pour le disque local. Le retrait d'un membre reste traité par le filtre de connexion (§9.4). Les §2.3–2.4 ne servent alors qu'aux **appareils d'une même personne** et à l'export « pour les personnes du projet » (stanzas de l'annuaire). Reste la question d'un **dossier de projet partagé** (nuage commun) : là seulement une DEK partagée a un sens : Q-1.

### 7.3 Dorsal de stockage distant (N22, §6.7)

Déjà conforme : chiffrement **avant** l'extension, noms opaques, clé jamais transmise (ST-3 à ST-8). N37 ne change que (i) la clé de sauvegarde = sous-clé de la DEK **de l'appareil** (récupération par phrase, ST-8) ; (ii) la possibilité de pointer le dossier de projet lui-même vers un dossier synchronisé : le contenu est déjà chiffré, ce qui rend le « miroir lisible » (non chiffré, §6.7) le seul canal en clair, signalé comme tel.

### 7.4 Index IA et RAG (N33)

L'index (embeddings, texte découpé, métadonnées) **est du clair dérivé** : il doit être chiffré comme un objet (sous-clé `inkwell/index/v1`), ou reconstruit à l'ouverture et gardé en mémoire/dossier de travail privé. Fuites propres aux vecteurs : les embeddings se **réinvertissent** partiellement en texte `[mémoire]` ; ils comptent donc comme du clair. Si une extension d'IA distante reçoit des fragments, c'est une fuite **voulue** et soumise aux règles de flux RF-1 à RF-5 (§15.5), indépendante du chiffrement au repos. Voir aussi `docs/architecture/research/ai.md`.

## 8. Au-delà de l'existant

1. **Pas de VFS, un magasin d'objets** : la contrainte « disque = bloc opaque, rechiffré à l'écriture » est tenue **par objet** : une frappe ne rechiffre que le petit objet touché, pas un conteneur de N Gio (un conteneur monolithique gocryptfs/VeraCrypt réécrit ou re-synchronise des blocs ; un dépôt d'objets se synchronise et se sauvegarde incrémentalement).
2. **L'annuaire est déjà fait** : la liste signée ; on lui ajoute une clé, pas un service.
3. **Récupération = un destinataire de plus** : phrase de 24 mots déjà prévue (ST-8) ; aucun mécanisme de séquestre.
4. **Réutiliser le format age comme format universel d'enveloppe** : outils tiers, plugins matériels, hybride PQ, spécifié (C2SP), et export recouvrable sans inkwell.
5. **DEK par appareil** (§7.2) : supprime l'essentiel de la complexité de groupe (pas de rotation, pas de MLS).

## 9. Risques, questions ouvertes, prochaines étapes

| # | Question | Pourquoi |
|---|---|---|
| Q-1 | Dossier de projet **partagé** par un nuage commun : DEK partagée par époque (§2.4) ou refus en v1 ? | seul cas qui réintroduit une clé de groupe |
| Q-2 | X25519 seule en v1, ou hybride PQ (taille des clés dans les ops de membre) ? | récolte-puis-déchiffrement des copies hors appareil |
| Q-3 | Clé de sauvegarde (ST-3) distincte de la DEK, ou sous-clé ? | un ancien membre vs sauvegardes récentes |
| Q-4 | Dossier de travail sous macOS/Windows et enfermement F5 « lancement borné refusé » | cohérence avec §9.5 |
| Q-5 | Couverture réelle de la surcouche LSP de tinymist (images, polices, imports) | décide si Windows peut éviter le disque pour le texte |
| Q-6 | Trousseau système : bibliothèque pure Go auditée, ou mot de passe seul ? | dépendance, cgo, D-Bus (§9.4) |

**À mesurer avant ADR** (`go-benchmark`, benchstat) : coût de chiffrement par objet pour ouverture d'un gros projet ; latence d'écriture d'une frappe ; temps de ré-chiffrement complet. **À rédiger** : ADR « chiffrement au repos » (remplace NG3), additif protocole (clé X25519 d'appareil, op d'époque), tests : vecteurs d'enveloppe, fuzz des décodeurs d'enveloppe et de manifeste, propriété « un export chiffré est lisible par `age -d` », test canari de fuite (aucun clair dans journaux, noms de fichiers, dossier de travail après fermeture).

**Liens non vérifiés ou à relire** : notes finales Go 1.26 (seul un brouillon vu), §14 de RFC 9420, livre blanc 1Password, états de age-plugin-yubikey/tpm, `go-keyring`, comportements Windows/macOS du §5.3.
