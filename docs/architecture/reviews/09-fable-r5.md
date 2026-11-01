# Revue Fable n°5 de l'architecture (spec 1.0.0-brouillon.6, révision 5, 2026-11-01) — verdict : **ACCEPT** (0 bloquant, 2 majeurs de texte, 5 mineurs)

Portée : `spec.md` (brouillon.6), `CHANGES.md`, `decisions.md`, `traceability.md`, tels que commis en
`23c4246` (quatre fichiers de `docs/architecture/` seulement ; `docs/protocol/`, `docs/design/`,
`docs/adr/` intouchés par ce commit — les modifications non commises de `docs/design/spec.md` et
`tokens.json` dans l'arbre de travail ne font pas partie de la révision 5 et ne sont pas revues ici).
Non lu : la revue Codex du tour 5. Lu : la revue Codex du tour 4 (`08-codex-r4.md`), pour vérifier
son bloquant R4-B1 sur le texte. Référentiels : `docs/exigences.md` (N37, N38, N39, N41),
`docs/engineering-principles.md` §3 et §4, ADR 0029/0031/0033, C2SP age (`https://c2sp.org/age`),
RFC 9106 (Argon2), NIST SP 800-57 Part 1 rév. 5 (§5.3.6, §8.3.5 : compromission ⇒ rotation), noyau
Linux `admin-guide/blockdev/zram` (section *writeback*), documentation wazero (`CompilationCache`),
HTML Standard « update the rendering », Event Timing (W3C), Microsoft Learn `JOBOBJECT_*_LIMIT_INFORMATION`.
Gravités : **bloquant** = garantie centrale contredite ou porte qui ne peut ni passer ni échouer ;
**majeur** = texte qui doit changer avant qu'un lot s'en serve ; **mineur** = précision.

**Méthode.** Diff complet `5e2ab5f..23c4246` relu ligne à ligne ; chacun de mes constats du tour 4
relu contre la ligne qui prétend le fermer ; le bloquant Codex rejoué comme contre-exemple sur le
texte nouveau ; recherche de régressions par `grep` sur les formulations retirées (`0600`,
`UserConfigDir`, « trousseau », « secret de secours », « compilé à l'installation », « fusion
Automerge native », `inkwell/plugin/v1`) ; cohérence interne H-3/H-4 ↔ §0.5, SR ↔ VK ↔ §9.4,
EN-9 ↔ §9.5, §12.5 ↔ PM-4 ; constantes citées vérifiées dans le code (`internal/typst/engine.go`).

---

## 1. Mes constats du tour 4 : tous fermés

| # | Constat | Où c'est fermé (ligne de `spec.md`) | Vérifié |
|---|---|---|---|
| R4-M1 | repli « compilé à l'installation » inapplicable sous H-3 | §0.5 ligne « Hôte des greffons » (`:70`) : (a) **préchauffage** après la première peinture pour les greffons épinglés ou actifs au dernier usage, dans la limite des 6 actifs, activation froide affichée « Extension en préparation… », raison wazero écrite ; (b) sinon **revue de H-3 par ADR** (cache scellé, haché par le parent, dans le coffre) ; `AD-57` rév. 5 | oui — le repli existe maintenant sans cache disque ; reste une phrase de H-4 à aligner (R5-M2) |
| R4-M2 | refus « autre appareil » contre récupération | EN-2 « Voie de récupération » (`:1262–1269`) : refus par le destinataire **appareil** seulement ; par le secours, `project` vérifié, ancien `peer` **affiché** (« copie chiffrée par *Portable de Paul* »), DEK **ré-enveloppée** sous `keyring/<époque + 1>.age` pour le nouvel appareil et le secours, secret effacé ; vecteur « secours sur un appareil neuf, époque + 1, ancien appareil refusé ensuite » ; `AD-91` rév. 5 | oui ; une précision sur ce que « refusé ensuite » révoque (R5-m3) |
| R4-m1 | oracles de « vérifié » | EN-4 (`:1296–1304`) : `/proc/swaps` vide ou entrées sur `zram*` / `dm-*` dont `dm/uuid` commence par `CRYPT-` ; `lsblk -o TYPE` sous un nœud `crypt` ; `sysctl vm.swapusage` « (encrypted) », **exigé aussi pour le disque en mémoire macOS** (ligne de la table, `:1296`) ; `fdesetup status` « FileVault is On » ; `Win32_EncryptableVolume.ProtectionStatus = 1` ; support faussement annoncé → refus ; oracle illisible → refus (échec fermé) | oui ; un trou zram à boucher (R5-m1) |
| R4-m2 | version incomplète dès la création | HI-9 (`:1413–1419`) : blobs absents demandés aux pairs **avant** publication du manifeste ; sinon « incomplète (image `x` absente) » dès la création, dite par la vue ; un appareil qui reçoit un manifeste épingle ce qu'il a et cherche le reste (`blob.Service`) ; deux tests ajoutés | oui |
| R4-m3 | `AD-96` périmé | `decisions.md` : bandeau « *Historique : la recommandation initiale ci-dessous est remplacée par le dernier paragraphe « Révision n »…* » en tête d'`AD-96` (et des neuf autres entrées remplacées, Codex R4-m1) | oui |
| R4-m4 | HI-6 « comme une proposition (§15.5) » | HI-6 (`:1377–1381`) : la personne **qui peut écrire** prévisualise avec la présentation d'une proposition et l'applique **elle-même** ; « ce ne sont pas les règles des auteurs non humains (§15.5, RF-\*) » ; qui ne peut pas écrire passe par « Proposer d'intégrer » ; `AD-92` rév. 5 | oui |
| R4-m5 | borne des sidecars sans chiffre | §0.5 (`:71`) et §9.5 (`:1617`) : **256 Mio par compilation** (`typst.DefaultMaxOutput` — vérifié : `internal/typst/engine.go:47`, `256 << 20`), **512 Mio cumulés**, admission par le parent ; `AD-95` rév. 5 | oui ; la colonne macOS/Windows surclame (R5-m4) |

---

## 2. Le bloquant Codex R4-B1 : genuinement fermé sur le texte

**Contre-exemple de Codex rejoué** (Linux, aucun trousseau, aucun swap, projet protégé, dossier de
configuration sur disque ordinaire, appareil éteint volé, image lue hors ligne) contre la révision 5 :

1. **Le secret de secours n'est plus sur le disque.** §6.1 (`:777`) : « **jamais sur l'appareil** » ;
   ST-3/SR-1 (`:1072–1078`) : ni fichier, ni trousseau, ni réglage, ni journal, ni rapport ; en clair
   **en mémoire seulement**, deux fois (création, restauration), tampon `[]byte` écrasé de zéros,
   jamais `string` ; VK-2 (`:1279`) le répète côté coffre. L'attaquant n'a donc plus de quoi dériver
   `inkwell-recovery/x25519/1`. La voie « secours » exige la **phrase**, gardée hors de l'appareil (SR-4).
2. **Ce que l'appareil garde ne rend pas la DEK sans secret.** SR-2 (`:1079–1082`) : clé **publique**
   de secours (non secrète) et sous-clés de sauvegarde **enveloppées** comme la DEK sous la clé de
   l'appareil ; VK-3 : aucune autre copie locale (ni cache, ni temporaire).
3. **La clé privée X25519 de l'appareil est elle-même protégée.** VK-1 (`:1272–1278`) : trousseau du
   système, **ou** enveloppe sous mot de passe Argon2id à paramètres fixés (256 Mio, 3 passes, 4 fils,
   sel 16 octets — au-dessus des deux profils de la RFC 9106 §4 et du minimum OWASP), **ou** disque
   chiffré **vérifié** par les oracles d'EN-4 ; « **aucun repli** vers un fichier de clé nu (`0600`) pour
   un projet protégé » ; sans aucune des trois voies, « Protéger ce projet » demande un mot de passe ou
   reste indisponible. Dans le contre-exemple (pas de trousseau, pas de LUKS), seule la voie mot de passe
   reste : l'image ne contient que `keyring/<époque>.age` (deux stanzas), une clé X25519 enveloppée sous
   Argon2id, et une clé publique. **L'attaque ne tient plus.**
4. **Page verrouillée et zéroïsation** : SR-3 (`:1083–1089`) `mlock`/`VirtualLock` quand le système
   l'accorde, jamais journalisé (O-2), fenêtre exclue des captures (Windows, macOS), honnêteté sur
   Wayland/X11 et sur la mémoire de la webview (EN-7) ; EN-3 : projet verrouillé ⇒ tampons effacés.
5. **Preuve demandée à la porte** : EN-3 ajoute à P-VAULT « configuration et trousseau absents ou
   indisponibles ; verrouillage puis **vol hors ligne** (image du disque) : aucune clé ni DEK retrouvable
   sans mot de passe ni phrase (recherche des octets connus des clés sur l'image) ; récupération avec la
   phrase ; le mot de passe ne peut être contourné par aucune autre copie locale » (`:1285–1288`) —
   exactement la fermeture que Codex exigeait, et testable (E-2).

Conclusion : **fermé**, sur le texte. Une contradiction résiduelle dans §9.4 (R5-M1) doit disparaître
pour que la fermeture soit sans ambiguïté, et deux réductions de risque méritent une ligne (R5-m2).

---

## 3. Régression : aucune de fond ; deux contradictions de texte

- Aucune réapparition de « compilé à l'installation », « fusion Automerge native » (hors historique
  marqué), `inkwell/plugin/v1` ; §3.2 (`:267`) dit maintenant « **un** greffon par processus, ≤ 6,
  paresseux » ; annexe Décision 6 (`:3238`) porte le message exact `inkwell-ext:plugin-package:1\0`.
- PM-4 (`:1943–1951`, Codex R4-M1) : trois grandeurs séparées, « un rappel `requestAnimationFrame`,
  simple ou double, précède … la peinture : il ne prouve pas (3) » est conforme au HTML Standard ;
  Event Timing mesure bien jusqu'à la prochaine peinture après les gestionnaires ; cas témoin de mise en
  page lente exigé ; sans oracle validé, échec (E-2). Correct, et il referme mon R3-M4.
- Cohérence vérifiée : SR-1…SR-4 ↔ VK-1…VK-3 ↔ §6.1 ↔ P-VAULT ; EN-9 (dossier de travail ≤ 512 Mio) ↔
  §9.5 (sortie 256/512 Mio : la sortie est lue par le parent, pas comptée dans le dossier de travail —
  pas de double comptage) ; §0.5 ↔ `AD-57`, `AD-67`, `AD-91`, `AD-92`, `AD-93`, `AD-95` rév. 5.
- Deux phrases anciennes contredisent le texte nouveau : §9.4 (R5-M1) et H-4 (R5-M2).

---

## 4. Constats nouveaux de la révision 5

### R5-M1 — Majeur (texte) : §9.4 dit encore que le trousseau garde le secret de secours

**Preuve.** `spec.md:1599–1601` : « Le trousseau du système (…) garde, quand il est disponible, le
**secret de secours** et l'enveloppe locale de la DEK (§6.10 EN-3) ». SR-1 (`:1074`) : « ni fichier,
**ni trousseau** » ; VK-2 : « jamais sur l'appareil ». L'« enveloppe locale de la DEK » est en outre
`keyring/<époque>.age` dans le projet, pas dans le trousseau ; ce que VK-1 met au trousseau est la **clé
privée X25519 de l'appareil**. La phrase est l'exact reliquat du contrat que Codex a cassé.
**Norme.** §0.3 (unicité du contrat) ; N37 ; `engineering-principles` §4.
**Correction.** Remplacer par : « Le trousseau du système garde, quand il est disponible, la clé privée
X25519 de l'appareil (VK-1) ; le secret de secours n'y est **jamais** (SR-1) ». Ajouter à la table de
§9.4 une ligne **Clé X25519 de l'appareil** (création `crypto/rand`, liée par `inkwell-ext:enc-key:1` ;
stockage : VK-1 ; rotation : « Tout rechiffrer »), aujourd'hui absente alors que la table se veut la
liste des clés.

### R5-M2 — Majeur (texte) : H-4 « jamais au démarrage » interdit le repli de préchauffage

**Preuve.** H-4 (`:1674`) : hôte « créé **paresseusement** à la première activation, **jamais au
démarrage** ». §0.5 (`:70`) : repli (a) « l'hôte est créé et le module compilé en tâche de fond après la
première peinture » pour les greffons épinglés ou actifs au dernier usage. Sous E-4, un repli que la
règle H-4 interdit n'est pas applicable tel quel ; c'est la forme faible du défaut R4-M1.
**Norme.** E-1/E-4 ; §0.3.
**Correction.** H-4 : « jamais **avant la première peinture** ; après elle, seulement les greffons
épinglés ou actifs au dernier usage, et seulement si P-PLUG-BENCH a engagé le repli (a) du §0.5 ». Dire
dans §12.5 que le préchauffage commence après « dernier projet modifiable ≤ 1,5 s », à priorité basse, et
que `PERF-START` se mesure **avec** six greffons épinglés (le préchauffage ne doit pas manger la marge
entre 1 s et 1,5 s) ; la mémoire au repos reste ≤ 6 × 40 Mio (H-7), déjà dans le budget du §2.

### R5-m1 — Mineur : `zram*` accepté sans regarder son `backing_dev`

**Preuve.** EN-4 (`:1297`) accepte toute entrée de `/proc/swaps` sur `zram*` comme « en mémoire ». Le
noyau permet à zram d'**écrire les pages inactives sur un périphérique de repli**
(`/sys/block/zramX/backing_dev`, `admin-guide/blockdev/zram`, section *writeback*) : un `backing_dev`
sur une partition en clair renvoie le contre-exemple de R3-B1 par la petite porte.
**Correction.** `zram*` n'est accepté que si `/sys/block/zramX/backing_dev` vaut `none` ou désigne un
`dm-*` dont `dm/uuid` commence par `CRYPT-`. Même remarque pour `lsblk -o TYPE` : préférer lire
`/sys/block/<dev>/slaves/` et `dm/uuid` (sysfs) plutôt que d'analyser la sortie d'un programme externe —
l'oracle d'EN-4 est une garantie de sécurité, pas un diagnostic.

### R5-m2 — Mineur : la phrase traverse la webview sans vérifier le swap

**Preuve.** SR-1 : « la webview ne reçoit que les mots à afficher » ; or les 24 mots **sont** le secret
(BIP-39 est réversible), et à la restauration ils sont **saisis** dans la webview (chaînes JavaScript
immuables, non effaçables, mémoire WebKit hors contrôle — SR-3 le reconnaît). EN-4 n'oblige à vérifier le
swap qu'au déverrouillage d'un projet protégé ; la création du secret peut avoir lieu sur une machine
jamais vérifiée, dont le swap en clair peut recevoir une page de la webview.
**Norme.** N37 ; modèle « Voleur » (§9.2, résiduel « hibernation non chiffrée »).
**Correction.** Avant d'afficher ou de demander la phrase : jouer l'oracle « swap » d'EN-4 ; s'il échoue,
le dire (« Ce poste a une mémoire d'échange non chiffrée : évitez de saisir la phrase ici, ou activez le
chiffrement du disque ») ; effacer le champ à la fermeture ; interdire le presse-papiers sur ce champ.
Ajouter le cas à P-VAULT (recherche des mots de la phrase sur l'image de swap après la cérémonie).

### R5-m3 — Mineur : « ancien appareil refusé ensuite » ne révoque rien

**Preuve.** EN-2 (`:1262–1269`) : la **même** DEK est ré-enveloppée à l'époque + 1 ; « désormais une
enveloppe dont le destinataire appareil est l'ancien `peer` est refusée » n'agit que sur le **nouvel**
appareil ; si l'ancien appareil et son trousseau existent encore (disque cloné, restauration d'image), il
ouvre toujours `keyring/<époque>.age` avec sa propre clé. Ce n'est pas une faille (même personne, DEK par
copie), mais la phrase promet une révocation qu'elle n'opère pas.
**Norme.** NIST SP 800-57 Part 1 §5.3.6/§8.3.5 (clé possiblement compromise ⇒ rotation) ; §9.4
« rotation : Tout rechiffrer (appareil compromis) ».
**Correction.** Écrire : « l'ancienne enveloppe reste lisible par l'ancien appareil s'il existe encore ;
la récupération propose « Tout rechiffrer » (§9.4) quand l'ancien appareil est perdu et non retiré ».
Vecteur : récupération, puis « Tout rechiffrer », ancienne enveloppe inutilisable.

### R5-m4 — Mineur : §9.5 surclame « objet de tâche, rlimit » pour la sortie sur Windows ; le cap du code est *a posteriori*

**Preuve.** Ligne nouvelle (`:1617`), colonne macOS/Windows : « oui (objet de tâche, rlimit) ». Un objet
de tâche Windows borne mémoire, temps, nombre de processus, affinité, priorité, ensemble de travail et
débit d'E/S (`JOBOBJECT_BASIC_LIMIT_INFORMATION`, `JOBOBJECT_EXTENDED_LIMIT_INFORMATION`,
`JOBOBJECT_IO_RATE_CONTROL_INFORMATION`) — **pas** la taille cumulée des fichiers écrits. `RLIMIT_FSIZE`
existe sur macOS seulement. Dans le code, `MaxOutput` est vérifié **après** la compilation sur la taille
du fichier produit (`internal/typst/engine.go:658`, `:706`) : c'est une admission du résultat, pas une
borne ; la borne réelle est la ligne suivante (tmpfs de taille fixe, `RLIMIT_FSIZE`, ADR 0033), qui dit
déjà « lancement borné refusé » hors Linux.
**Norme.** ADR 0033 (échec fermé) ; §16 (écarts code ↔ spec).
**Correction.** Colonne macOS/Windows : « macOS : `RLIMIT_FSIZE` ; Windows : dossier de travail sur un
volume à quota ou refus (§9.11, à prouver) » ; noter au §16 que `MaxOutput` est aujourd'hui une
vérification *a posteriori* et que la borne noyau reste la seule preuve P-SANDBOX.

### R5-m5 — Mineur : la table de §9.4 et §6.1 ne nomment pas le stockage de la clé X25519

Voir R5-M1 : `device.key` (Ed25519, `0600`) reste légitimement nu (identité ; vol = retrait par un admin,
`:1591`), mais la clé X25519 qui ouvre le coffre n'a de règle de stockage que dans VK-1, loin de la
table des clés (`:1589–1596`) et de §6.1 (`:770`). Une ligne dans chaque table évite qu'un lecteur du
§9.4 range les deux clés dans le même fichier.

---

## 5. Ce qui est bien et qu'il ne faut pas toucher

SR-1…SR-4 et VK-1…VK-3 (le secret n'existe qu'en mémoire ; trois voies locales, aucune nue ; porte avec
image du disque) ; la voie de récupération à une nouvelle époque avec l'ancien appareil **affiché** ; les
oracles nommés d'EN-4 avec refus en échec fermé ; le préchauffage à la place d'un cache disque ; HI-9
« incomplète dès la création » ; HI-6 sans la règle des deux pour une personne qui peut écrire ; PM-4 à
trois grandeurs avec cas témoin ; les bandeaux « Historique » de `decisions.md`.

---

## Verdict : **ACCEPT**

Mes 2 majeurs et 5 mineurs du tour 4 sont fermés dans le texte ; le bloquant Codex R4-B1 est fermé
par SR-1…SR-4 et VK-1…VK-3 (contre-exemple rejoué : il ne tient plus) ; aucune régression de fond.
Deux majeurs **de texte** pour la révision 6, au fil de l'eau, sans nouveau tour bloquant : **R5-M1**
(§9.4 ne doit plus dire que le trousseau garde le secret de secours ; ligne « clé X25519 » dans la
table des clés) et **R5-M2** (H-4 « jamais au démarrage » aligné sur le repli de préchauffage).
Mineurs R5-m1…m5 au fil de l'eau. Acceptation conditionnée à rien d'autre.
