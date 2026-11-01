# Revue Fable n°6 de l'architecture (spec 1.0.0-brouillon.7, révision 6, 2026-11-01) — verdict : **ACCEPT** (0 bloquant, 2 majeurs de texte, 5 mineurs)

Portée : `spec.md` (brouillon.7), `CHANGES.md`, `decisions.md`, `traceability.md`, tels que commis en
`3b146d2` (diff complet `9249a47..3b146d2` relu ligne à ligne ; `docs/protocol/`, `docs/design/`,
`docs/adr/` intouchés). Non lu : la revue Codex du tour 6. Lu : `10-codex-r5.md`, pour rejouer ses
deux bloquants sur le texte nouveau. Référentiels : `docs/exigences.md` (N37, N38), règles E-1 à E-5
du §0.5, ADR 0033 (échec fermé), modèle « Voleur » (§9.2), `core(5)` (man7, « Piping core dumps to a
program »), `systemd-coredump(8)`, `proc_pid_mountinfo(5)`, noyau `admin-guide/blockdev/zram`
(*writeback*), Microsoft Learn (WebView2 `FailureReportFolderPath`, `Win32_EncryptableVolume`),
code `internal/sidecar/{mount,dir,limit}_linux.go` et `internal/typst/engine.go`. Mesures faites
sur cet hôte (Fedora 44, btrfs sur LUKS, zram, `core_pattern` systemd) pour les oracles Linux.
Gravités : **bloquant** = garantie centrale contredite ou porte qui ne peut ni passer ni échouer ;
**majeur** = texte qui doit changer avant qu'un lot s'en serve ; **mineur** = précision.

---

## 1. Mes constats du tour 5 : tous fermés

| # | Constat | Où c'est fermé (`spec.md`) | Vérifié |
|---|---|---|---|
| R5-M1 | §9.4 : le trousseau garde le secret de secours | `:1648–1656` : le trousseau garde la **clé privée X25519** (VK-1), « le secret de secours n'y est jamais (SR-1) », l'enveloppe de DEK est `keyring/<époque>.age` ; lignes **Clé X25519**, **DEK**, **Secret de secours** dans la table des clés (`:1641–1643`) | oui |
| R5-M2 | H-4 « jamais au démarrage » contre le préchauffage | H-4 (`:1727`) : « jamais **avant la première peinture** », préchauffage des seuls épinglés ou actifs au dernier usage, si P-PLUG-BENCH a engagé le repli (a), priorité basse, après « dernier projet modifiable » ; §12.5 (`:1968`) : `PERF-START` mesuré **avec six greffons épinglés** ; `AD-57` rév. 6 ; budget 6 × 40 Mio inchangé (`:165`) | oui |
| R5-m1 | `zram*` sans `backing_dev` ; `lsblk` | OV-1 (`:1336–1341`) : sysfs seulement, `backing_dev` = `none` ou `dm-*` `CRYPT-` ; `lsblk` disparu du texte (grep) | oui — mais l'oracle sysfs ne résout pas btrfs (R6-M1) |
| R5-m2 | la phrase traverse la webview sans oracle swap | SR-5 (`:1091–1119`) : oracle de pagination (swap **et** hibernation) avant toute cérémonie, refus avec marche à suivre, presse-papiers bloqué, champ détruit, P-VAULT cherche les **mots** | oui — un canal de persistance manque (R6-M2) |
| R5-m3 | « ancien appareil refusé ensuite » ne révoque rien | EN-2 (`:1294–1299`) : « cela **ne révoque rien** », « Tout rechiffrer » proposé, vecteur « récupération puis Tout rechiffrer » | oui |
| R5-m4 | §9.5 surclame l'objet de tâche | §9.5 (`:1669`) : macOS `RLIMIT_FSIZE`, Windows volume à quota ou refus ; §16 (`:3196`) | oui — ma formule « a posteriori » était trop forte, à corriger (R6-m4) |
| R5-m5 | clé X25519 absente des tables | §6.1 (`:771`) et §9.4 (`:1641`) | oui |

---

## 2. Les bloquants Codex du tour 5, rejoués sur le texte

**R5-B1 (phrase paginée en clair).** Contre-exemple rejoué : Linux, swap en clair, création ou
restauration, pages du processus de rendu paginées pendant le dialogue. Révision 6 : SR-5 dit que les
mots **sont** le secret, que Go ne peut ni verrouiller ni effacer la mémoire du processus de rendu
(WebKitGTK, WKWebView, WebView2 ; chaînes JavaScript immuables), que retirer les mots du DOM n'efface
rien ; la **garantie minimale** (« aucune page contenant la phrase n'est écrite en clair sur un support
persistant ») vient de l'**environnement** : oracle OV-1…OV-3 (swap **et** hibernation) joué avant
toute cérémonie, sinon **refus** avec la marche à suivre ; P-VAULT : pression mémoire, échec du
verrouillage, fermeture, plantage, puis recherche des **mots** (deux listes) sur le swap, l'image
d'hibernation et le disque. Le contre-exemple ne tient plus : avec swap en clair, la cérémonie n'a
pas lieu. **Fermé**, sous une réserve de texte : la pagination et l'hibernation ne sont pas les seuls
chemins par lesquels une page du processus de rendu atteint le disque — le **vidage sur plantage**
en est un troisième, que SR-5 nomme pour Go seulement (R6-M2).

**R5-B2 (FileVault non lié au volume).** Contre-exemple rejoué : FileVault actif sur le volume de
démarrage, dossier de travail ou clé X25519 sur un volume externe en clair. Révision 6 : OV-2 résout
`statfs(chemin).f_mntfromname` → volume APFS et exige le chiffrement **de ce volume** (`diskutil info
-plist`), « le statut global `fdesetup status` ne suffit pas » ; OV-4 force le placement du dossier de
travail et de la clé sous le dossier de la personne sur un volume vérifié, montage changé → refus,
état inconnu → refus ; témoin « démarrage chiffré, destination externe en clair » dans P-VAULT ; la
table EN-4 (`:1326`) renvoie à OV-2. Le contre-exemple ne tient plus. **Fermé.**

**Honnêteté par système.** Linux : lecture de sysfs, jamais d'un programme externe (bien) ; macOS :
`diskutil`, `pmset`, `sysctl` — des programmes externes, sans que OV-2 le dise alors qu'OV-1 l'interdit ;
il n'existe pas d'équivalent public de sysfs, donc l'asymétrie est légitime, mais elle doit être
écrite (R6-m2). Windows : WMI du volume, `ProtectionStatus = 1` (refuse bien un volume chiffré à
protection suspendue) ; les oracles d'hibernation et de chiffrement du fichier d'échange ne sont pas
nommés (R6-m2). Rien n'est présenté comme prouvé : « à auditer », « à prouver » partout. Honnête.

---

## 3. Régression : aucune

Greps sur les formulations retirées : `lsblk`, `fdesetup` (hors la négation d'OV-2), « jamais au
démarrage », « compilé à l'installation », « reçoit que les mots », « secret de secours et
l'enveloppe » : **aucune** occurrence. `0600` ne reste que pour `device.key`, les sockets et les
journaux (`:467`, `:770`, `:1632`, `:1640`), et dans l'interdiction de VK-1 (`:1309`). Cohérence
vérifiée : SR-5 ↔ OV ↔ EN-4 « Refus » (deux refus distincts, chacun défini) ; H-4 ↔ §0.5 ↔ §12.5 ↔
`AD-57` ; §9.4 ↔ SR-1 ↔ VK-1 ↔ §6.1 ; EN-2 ↔ §9.4 « Tout rechiffrer » ; EN-7 inchangé et toujours
vrai. `AD-91` rév. 6 résume fidèlement. Versions : `spec`, `decisions`, `traceability`, `CHANGES`
alignés sur brouillon.7.

---

## 4. Constats nouveaux de la révision 6

### R6-M1 — Majeur (texte) : l'oracle OV-1 ne peut pas passer sur btrfs ni sur un fichier d'échange

**Preuve (mesurée sur cet hôte).** OV-1 (`:1336–1338`) : « volume = `stat(chemin).st_dev` →
`/sys/dev/block/<maj>:<min>` ». Sur btrfs, `st_dev` est un numéro de périphérique **anonyme** par
sous-volume : ici `/home` → `0:51`, `/` → `0:38`, **absents de `/sys/dev/block/`**, alors que les
deux sont sur LUKS (`/dev/mapper/luks-…`). Même défaut pour overlayfs, NFS et tout système de
fichiers à périphérique anonyme. Par la règle « oracle illisible → refus », chaque installation
Fedora par défaut (btrfs sur LUKS, la cible de développement) **refuse** la matérialisation, la voie
« disque chiffré » de VK-1 et la cérémonie SR-5, chiffrement ou non. De même, `/proc/swaps` liste un
**fichier** d'échange par son chemin (`/swapfile`, `Type: file`) — ni `dm-*` ni `zram*` — ce qui
refuse l'installation Ubuntu chiffrée par défaut (swapfile sur la racine LUKS). Enfin, pour une
partition (`8:1`), `slaves/` n'existe que sur le disque parent.
**Norme.** E-2/E-3 (une porte doit pouvoir **passer** sur les six cibles) ; ADR 0033 (échec fermé,
mais pas échec permanent) ; N37. Ce n'est pas une faille (le refus est sûr), c'est une porte qui ne
peut que fermer.
**Correction.** Résoudre le volume par `/proc/self/mountinfo` (`proc_pid_mountinfo(5)` : le point de
montage le plus long qui préfixe le chemin, champ « source du montage ») → `stat` de la source →
`/sys/dev/block/<maj>:<min>` ; si c'est une partition, remonter au parent avant `slaves/` ; une
entrée de `/proc/swaps` de type `file` est résolue comme un chemin. Témoins à ajouter à P-VAULT :
« btrfs sur LUKS → accepté », « swapfile sur LUKS → accepté », « btrfs en clair → refus ».

### R6-M2 — Majeur (texte) : le vidage sur plantage du processus de rendu échappe à l'oracle de SR-5

**Preuve.** SR-5 désactive les vidages **de Go** « pendant la cérémonie » (`RLIMIT_CORE = 0`,
`PR_SET_DUMPABLE = 0`, exclusion WER) et reconnaît que les mots vivent dans le **processus de rendu**.
Trois faits rendent la « garantie minimale » (`:1103–1105`) fausse sur ce chemin :
(1) `core(5)` : « *RLIMIT_CORE will be ignored if the system is configured to pipe core dumps to a
program* » — ici `kernel.core_pattern = |/usr/lib/systemd/systemd-coredump … %c` ; c'est le
**gestionnaire** qui décide d'après `%c`, la limite **du processus qui plante** ; or `setrlimit` sur
le parent ne change pas un enfant déjà lancé, et le WebProcess de WebKitGTK est lancé bien avant la
cérémonie, avec la limite héritée au démarrage de l'application. Le vidage va dans
`/var/lib/systemd/coredump/` (`systemd-coredump(8)`), sur le volume racine, que OV-4 **n'exige pas**
chiffré (seuls le dossier de la personne, le swap et l'hibernation le sont) ; Apport (Ubuntu) a ses
propres règles. (2) Windows : WebView2 écrit un **minidump de chaque processus qui plante** dans
`FailureReportFolderPath`, sous le dossier de données utilisateur (Microsoft Learn, *Handling
process-related events*) ; `IsCustomCrashReportingEnabled` n'empêche que l'**envoi**, pas
l'écriture ; l'exclusion WER ne couvre pas Crashpad. (3) macOS : ReportCrash écrit un rapport `.ips`
(piles et registres, pas le tas entier) dans `~/Library/Logs/DiagnosticReports` — moindre, à prouver.
P-VAULT demande déjà « plantage, puis inspection hors ligne du disque, recherche des mots » : la porte
**saurait échouer** ; c'est le contrat qui ne nomme pas le canal.
**Norme.** N37 ; `core(5)` ; SR-5 elle-même (« toute représentation compte »).
**Correction.** (a) Poser `RLIMIT_CORE = 0` **au lancement de l'application, avant la création de la
webview**, pour qu'il soit hérité par les processus de rendu, et le dire (la mention « pendant la
cérémonie » ne vaut que pour `PR_SET_DUMPABLE`). (b) Étendre l'oracle de pagination en oracle de
**pagination et de vidage** : Linux, si `core_pattern` est un tube, le volume de sa destination
(`/var/lib/systemd/coredump`, `/var/crash`) doit être vérifié, sinon refus de la cérémonie ; Windows,
le dossier de données de WebView2 entre dans la liste d'OV-4 (volume vérifié) et
`FailureReportFolderPath` est purgé à la fin de la cérémonie ; macOS, `DiagnosticReports` sur le
volume vérifié. (c) P-VAULT : tuer le processus de rendu **pendant** la cérémonie (`SIGSEGV`,
`TerminateProcess`) et chercher les mots dans ces trois magasins. Majeur et non bloquant : le texte
change d'un paragraphe, la porte teste déjà le cas, aucun lot ne s'en sert encore.

### R6-m1 — Mineur : VK-1 ne dit pas que « Protéger ce projet » dépend aussi de SR-5

**Preuve.** VK-1 (`:1310–1311`) : « si aucune des trois voies n'existe, Protéger ce projet demande un
mot de passe ou reste indisponible ». Or EN-2 exige le destinataire de **secours** dans chaque
enveloppe, donc la création du secret, donc la cérémonie SR-5, donc l'oracle de pagination : sur une
machine à swap en clair sans chiffrement du disque, un mot de passe **ne suffit pas** — « Protéger ce
projet » y est indisponible. De même, OV est « rejoué au déverrouillage » avec l'hibernation, mais la
seule conséquence écrite est le « Refus » de matérialisation ; la DEK en mémoire de Go peut encore
être hibernée en clair, ce que le résiduel « hibernation non chiffrée » du Voleur (`:1622`) couvre
sans renvoyer à OV. **Norme.** N37 (honnêteté), §0.3. **Correction.** Une phrase dans VK-1 et au §0.5
(repli de P-VAULT) ; au §9.2, « hibernation non chiffrée : oracle OV joué et dit à l'écran au
déverrouillage, non bloquant ».

### R6-m2 — Mineur : oracles macOS et Windows incomplets ou non nommés

OV-2 s'appuie sur la sortie de `diskutil`, `pmset`, `sysctl` alors qu'OV-1 interdit « la sortie d'un
programme externe » : écrire « pas d'équivalent de sysfs ; plist analysée structurellement, jamais
du texte » ; les clés `FileVault` / `Encrypted` du plist sont à confirmer par P-VAULT-MAC. OV-3 ne
nomme ni l'oracle d'hibernation (`HKLM\SYSTEM\CurrentControlSet\Control\Power\HibernateEnabled`, ou
`powercfg /a`) ni celui du fichier d'échange chiffré (`fsutil behavior query encryptpagingfile`,
`NtfsEncryptPagingFile`). OV-1 : `/sys/power/disk` = `[disabled]` (verrouillage du noyau, mesuré ici)
vaut aussi « pas d'hibernation ». **Norme.** E-2 (un oracle nommé par porte).

### R6-m3 — Mineur : OV-4 liste les chemins sensibles sans les données de la webview

OV-4 nomme le dossier de travail et la clé X25519. Le dossier de données de la webview (cache HTTP,
`localStorage`, et, sur Windows, les minidumps de R6-M2) est sous le dossier de la personne mais
n'est pas dans la liste ; l'y mettre évite qu'un déplacement de `UserDataDir` le sorte du volume
vérifié. **Norme.** VK-3 (aucune autre copie locale), OV-4.

### R6-m4 — Mineur (correction de mon R5-m4) : §16 sous-estime `MaxOutput` sur Linux

**Preuve.** §16 (`:3196`) et §9.5 (`:1669`) disent que `MaxOutput` « n'est aujourd'hui qu'une
vérification *a posteriori* ». C'est vrai de `engine.go:658` et `:706` seulement. Sur Linux, la même
valeur est le **budget de la borne noyau** : `engine.go:484` la passe en `MaxDirBytes` et
`FileSizeLimit` ; `mount_linux.go:95` en fait la taille du tmpfs (`size=`), `dir_linux.go:121–123`
les parts fixes par fichier (Landlock, `RLIMIT_FSIZE`), `limit_linux.go:32` le `ulimit -f`. La ligne
doit dire : « Linux : `MaxOutput` dimensionne la borne noyau (tmpfs, `RLIMIT_FSIZE`, parts Landlock)
et la vérification finale est une seconde admission ; macOS et Windows : vérification *a posteriori*
seulement, donc lancement borné refusé (`dir_other.go`) ». Mon tour 5 a propagé l'imprécision.
**Norme.** §16 (écarts code ↔ spec exacts).

### R6-m5 — Mineur : `coherence.md` reste daté « révision 4, brouillon.5 »

En-tête (`coherence.md:5`) deux révisions en retard alors que `spec.md` y renvoie comme pièce de
version. Hors du commit revu, mais contraire à §0.3 (une source par chose). Une ligne.

---

## 5. Ce qui est bien et qu'il ne faut pas toucher

SR-5 : la séparation « ce que Go garantit / ce qu'il ne peut pas », la garantie minimale tirée de
l'environnement et le refus de la cérémonie plutôt qu'une promesse de `mlock` — c'est la bonne
forme, il faut seulement y ajouter le canal des vidages. OV-1…OV-4 : résolution par chemin, pas par
statut global ; `zram` avec `backing_dev` ; témoins de montage changé et d'état inconnu. §9.4 enfin
exact sur le trousseau. EN-2 qui dit ce que la récupération **ne** révoque **pas**. H-4 et
`PERF-START` avec six greffons épinglés : le préchauffage ne pourra pas manger la marge.

---

## Verdict : **ACCEPT**

Mes 2 majeurs et 5 mineurs du tour 5 sont fermés dans le texte ; les bloquants Codex R5-B1 et
R5-B2 sont fermés par SR-5 et OV-1…OV-4 (contre-exemples rejoués : ils ne tiennent plus) ; les
garanties par système sont dites honnêtement, « à auditer » et « à prouver » ; aucune régression.
Deux majeurs **de texte** pour la révision 7, au fil de l'eau, sans nouveau tour bloquant :
**R6-M1** (OV-1 doit résoudre btrfs et les fichiers d'échange par `mountinfo`, sinon la porte ne peut
que refuser sur la cible de développement) et **R6-M2** (le vidage sur plantage du processus de rendu
entre dans l'oracle de SR-5 et `RLIMIT_CORE = 0` est posé avant la webview). Mineurs R6-m1…m5 au fil
de l'eau, dont la correction de ma propre imprécision R5-m4. Acceptation conditionnée à rien d'autre.
